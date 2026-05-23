package vm

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
	"github.com/gscript/gscript/internal/runtime"
)

const (
	maxStack                 = 256    // max registers per call frame
	maxCallDepth             = 100000 // max call stack depth
	initialCallFrameCapacity = 64
	maxMetaDepth             = 50 // max __index chain depth
)

// MethodJITEngine is the interface for the Method JIT compiler.
// It compiles hot functions to native code and executes them.
type MethodJITEngine interface {
	TryCompile(proto *FuncProto) interface{} // returns *CompiledFunction or nil
	Execute(compiled interface{}, regs []runtime.Value, base int, proto *FuncProto) ([]runtime.Value, error)
	SetCallVM(v *VM) // sets the VM for call-exit/global-exit
}

type methodJITEngineWithResultBuffer interface {
	ExecuteWithResultBuffer(compiled interface{}, regs []runtime.Value, base int, proto *FuncProto, retBuf []runtime.Value) ([]runtime.Value, error)
}

type methodJITEngineWithCompiledSpecializationCall interface {
	TryExecuteCompiledSpecializationCall(fn runtime.Value, regs []runtime.Value, absSlot, nArgs, nRets int) (bool, error)
}

// MethodJITContinuation describes a suspended method-JIT frame. The concrete
// compiled object remains owned by the engine; the VM only stores enough state
// for a coroutine to re-enter the same function after coroutine.yield.
type MethodJITContinuation struct {
	Compiled interface{}
	Base     int
	Proto    *FuncProto
	PC       int
	Offset   int
}

type methodJITEngineWithContinuation interface {
	ExecuteContinuation(cont MethodJITContinuation, regs []runtime.Value, retBuf []runtime.Value) ([]runtime.Value, error)
}

type methodJITEngineWithCoroutineChild interface {
	NewCoroutineChildEngine(child *VM) MethodJITEngine
}

// VM is the bytecode virtual machine.
type VM struct {
	regs                 []runtime.Value          // register file (shared across frames via base offset)
	frames               []CallFrame              // call stack
	frameCount           int                      // current number of active frames
	globals              map[string]runtime.Value // legacy map (kept for interop)
	globalArray          []runtime.Value          // indexed globals (fast path)
	globalIndex          map[string]int           // name → index in globalArray
	globalVer            uint32                   // bumped on structural changes (new globals added)
	globalValueVer       uint64                   // bumped whenever indexed global values may have changed
	globalOverrides      map[string]runtime.Value // per-VM global overrides for coroutine-local builtins
	globalOverrideIdx    map[int]runtime.Value    // indexed mirror of globalOverrides for GETGLOBAL cache hits
	globalOverrideFast   int                      // single indexed override fast path (-1 = disabled)
	globalOverrideVal    runtime.Value
	readOnlyGlobals      map[string]bool
	globalsMu            *sync.RWMutex  // protects globals for goroutine safety (shared across VMs)
	noGlobalLock         bool           // skip globals mutex (single-threaded mode)
	openUpvals           []*Upvalue     // list of open upvalues (sorted by regIdx descending)
	top                  int            // top of used registers (for variable returns)
	stringMeta           *runtime.Table // string metatable
	methodJIT            MethodJITEngine
	argBuf               [16]runtime.Value // pre-allocated arg buffer for OP_CALL
	retBuf               [8]runtime.Value  // pre-allocated return buffer for OP_RETURN
	coroutineResultBuf   [8]runtime.Value  // pre-allocated coroutine.resume result buffer
	callSiteFloatBuf     []float64         // reusable non-pointer scratch for guarded call-site runtime specializations
	callSiteIntBuf       []int64           // reusable non-pointer scratch for guarded call-site runtime specializations
	callSiteValueBuf     []runtime.Value   // reusable Value scratch; scanned as GC roots below
	spectralCoefficients spectralCoefficientCache
	currentCoroutine     *VMCoroutine // coroutine currently running on this VM, if any
	coroutineYielded     bool         // current coroutine VM paused through coroutine.yield
	coroutineStats       *coroutineStats
	coroutineCreateFn    *runtime.GoFunction
	coroutineResumeFn    *runtime.GoFunction
	coroutineYieldFn     *runtime.GoFunction
	ipairsIteratorFn     *runtime.GoFunction
	debugHook            runtime.Value
	debugOpts            runtime.DebugHookOptions
	debugSink            runtime.Value
	debugBusy            bool
	scriptDir            string
}

// SetMethodJIT sets the Method JIT engine for this VM.
// When set, hot functions are automatically compiled and executed natively.
// Also sets the VM reference on the engine for call-exit support.
func (vm *VM) SetMethodJIT(engine MethodJITEngine) {
	vm.methodJIT = engine
	if engine != nil {
		engine.SetCallVM(vm)
	}
}

// Regs returns the register file. Used by the JIT executor.
func (vm *VM) Regs() []runtime.Value {
	return vm.regs
}

// SetTop sets the top-of-stack pointer. Used by the Method JIT to reserve
// register space for temp slots before executing calls via call-exit.
func (vm *VM) SetTop(top int) {
	vm.top = top
}

// Top returns the current top-of-stack pointer. Used by the baseline JIT
// to implement B=0 (variable args) in OP_CALL.
func (vm *VM) Top() int {
	return vm.top
}

// TopPtr returns a pointer to vm.top. Used by the JIT to read/write Top
// from native code for variable-arg (B=0) and variable-return (C=0) calls.
func (vm *VM) TopPtr() *int {
	return &vm.top
}

// CurrentClosure returns the closure for the current (topmost) call frame.
// Used by the baseline JIT to access upvalues. Returns nil if no frame is active.
func (vm *VM) CurrentClosure() *Closure {
	if vm.frameCount > 0 {
		return vm.frames[vm.frameCount-1].closure
	}
	return nil
}

// CurrentVarargs returns the varargs for the current (topmost) call frame.
// Used by the Tier 2 JIT to support OP_VARARG via exit-resume.
func (vm *VM) CurrentVarargs() []runtime.Value {
	if vm.frameCount > 0 {
		return vm.frames[vm.frameCount-1].varargs
	}
	return nil
}

// PushFrame pushes a minimal call frame for the given closure and base.
// Used by the baseline JIT's fast call path so that CurrentClosure() and
// CloseUpvalues() work correctly for the callee.
// Returns false if the call stack would overflow.
func (vm *VM) PushFrame(cl *Closure, base int) bool {
	if !vm.ensureFrameSlot() {
		return false
	}
	frame := &vm.frames[vm.frameCount]
	frame.closure = cl
	frame.pc = 0
	frame.base = base
	frame.numResults = -1
	frame.varargs = nil
	frame.callSitePC = -1
	frame.defers = nil
	vm.frameCount++
	return true
}

func (vm *VM) ensureFrameSlot() bool {
	if vm.frameCount < len(vm.frames) {
		return true
	}
	if vm.frameCount >= maxCallDepth {
		return false
	}
	newLen := len(vm.frames) * 2
	if newLen == 0 {
		newLen = initialCallFrameCapacity
	}
	if newLen <= vm.frameCount {
		newLen = vm.frameCount + 1
	}
	if newLen > maxCallDepth {
		newLen = maxCallDepth
	}
	newFrames := make([]CallFrame, newLen)
	copy(newFrames, vm.frames)
	for i := 0; i < vm.frameCount; i++ {
		if n := len(newFrames[i].varargs); n > 0 && n <= len(newFrames[i].inlineVarargs) {
			copy(newFrames[i].inlineVarargs[:], newFrames[i].varargs)
			newFrames[i].varargs = newFrames[i].inlineVarargs[:n]
		}
	}
	vm.frames = newFrames
	return true
}

func setFrameVarargs(frame *CallFrame, args []runtime.Value) {
	if len(args) == 0 {
		frame.varargs = nil
		return
	}
	if len(args) <= len(frame.inlineVarargs) {
		copy(frame.inlineVarargs[:], args)
		frame.varargs = frame.inlineVarargs[:len(args)]
		return
	}
	frame.varargs = make([]runtime.Value, len(args))
	copy(frame.varargs, args)
}

func observeCallResultFixed(proto *FuncProto, pc int, regs []runtime.Value, resultBase int, resultCount int) {
	if proto == nil || proto.CallSiteFeedback == nil || pc < 0 || pc >= len(proto.CallSiteFeedback) || resultCount <= 1 {
		return
	}
	if resultBase < 0 || resultBase >= len(regs) {
		proto.CallSiteFeedback[pc].ObserveResult(runtime.NilValue())
		return
	}
	proto.CallSiteFeedback[pc].ObserveResult(regs[resultBase])
}

func observeCallResultSlice(proto *FuncProto, pc int, results []runtime.Value, resultCount int) {
	if proto == nil || proto.CallSiteFeedback == nil || pc < 0 || pc >= len(proto.CallSiteFeedback) || resultCount == 1 {
		return
	}
	if len(results) == 0 {
		proto.CallSiteFeedback[pc].ObserveResult(runtime.NilValue())
		return
	}
	proto.CallSiteFeedback[pc].ObserveResult(results[0])
}

// PopFrame removes the topmost call frame.
// Used by the baseline JIT's fast call path after callee execution.
func (vm *VM) PopFrame() {
	if vm.frameCount > 0 {
		vm.frameCount--
	}
}

// FrameCount returns the current call stack depth.
func (vm *VM) FrameCount() int {
	return vm.frameCount
}

// EnsureRegs ensures the register file has at least `needed` slots.
// If the register file is grown, returns the new slice.
func (vm *VM) EnsureRegs(needed int) []runtime.Value {
	if needed > len(vm.regs) {
		newRegs := runtime.MakeNilSlice(needed * 2)
		copy(newRegs, vm.regs)
		vm.regs = newRegs
	}
	return vm.regs
}

// SetCurrentFramePC updates the top frame's program counter. JIT coroutine
// suspension uses this so interpreter fallback can resume at the same point.
func (vm *VM) SetCurrentFramePC(pc int) error {
	if vm.frameCount == 0 {
		return fmt.Errorf("SetCurrentFramePC: no active call frame")
	}
	vm.frames[vm.frameCount-1].pc = pc
	return nil
}

// Globals returns the globals map.
func (vm *VM) Globals() map[string]runtime.Value {
	return vm.globals
}

// GetGlobal reads a global variable with proper locking.
func (vm *VM) GetGlobal(name string) runtime.Value {
	if vm.globalOverrides != nil {
		if v, ok := vm.globalOverrides[name]; ok {
			return v
		}
	}
	if vm.noGlobalLock {
		if idx, ok := vm.globalIndex[name]; ok {
			return vm.globalArray[idx]
		}
		return runtime.NilValue()
	}
	vm.globalsMu.RLock()
	if idx, ok := vm.globalIndex[name]; ok {
		v := vm.globalArray[idx]
		vm.globalsMu.RUnlock()
		return v
	}
	vm.globalsMu.RUnlock()
	return runtime.NilValue()
}

func (vm *VM) GlobalIndex(name string) (int, bool) {
	if vm == nil || !vm.noGlobalLock || vm.globalOverrides != nil {
		return 0, false
	}
	idx, ok := vm.globalIndex[name]
	if !ok || idx < 0 || idx >= len(vm.globalArray) {
		return 0, false
	}
	return idx, true
}

func (vm *VM) GetGlobalByIndex(idx int) (runtime.Value, bool) {
	if vm == nil || !vm.noGlobalLock || vm.globalOverrides != nil || idx < 0 || idx >= len(vm.globalArray) {
		return runtime.NilValue(), false
	}
	return vm.globalArray[idx], true
}

func (vm *VM) GlobalValueVersionPtr() (*uint64, uint64, bool) {
	if vm == nil || !vm.noGlobalLock || vm.globalOverrides != nil {
		return nil, 0, false
	}
	return &vm.globalValueVer, vm.globalValueVer, true
}

// SetGlobal writes a global variable with proper locking.
func (vm *VM) SetGlobal(name string, val runtime.Value) {
	if vm.noGlobalLock {
		if idx, ok := vm.globalIndex[name]; ok {
			vm.globalArray[idx] = val
			vm.globals[name] = val
			vm.globalValueVer++
		} else {
			idx = len(vm.globalArray)
			vm.globalArray = append(vm.globalArray, val)
			vm.globalIndex[name] = idx
			vm.globals[name] = val
			vm.globalVer++
			vm.globalValueVer++
		}
		return
	}
	vm.globalsMu.Lock()
	if idx, ok := vm.globalIndex[name]; ok {
		vm.globalArray[idx] = val
		vm.globals[name] = val
		vm.globalValueVer++
	} else {
		idx = len(vm.globalArray)
		vm.globalArray = append(vm.globalArray, val)
		vm.globalIndex[name] = idx
		vm.globals[name] = val
		vm.globalVer++
		vm.globalValueVer++
	}
	vm.globalsMu.Unlock()
}

func (vm *VM) DeleteGlobal(name string) {
	if vm.noGlobalLock {
		if idx, ok := vm.globalIndex[name]; ok {
			vm.globalArray[idx] = runtime.NilValue()
			delete(vm.globalIndex, name)
			delete(vm.globals, name)
			vm.globalVer++
			vm.globalValueVer++
		}
		return
	}
	vm.globalsMu.Lock()
	if idx, ok := vm.globalIndex[name]; ok {
		vm.globalArray[idx] = runtime.NilValue()
		delete(vm.globalIndex, name)
		delete(vm.globals, name)
		vm.globalVer++
		vm.globalValueVer++
	}
	vm.globalsMu.Unlock()
}

func (vm *VM) RestrictStdlib(allowed map[string]bool) {
	for _, name := range runtime.StdlibModuleNames() {
		if allowed[name] {
			continue
		}
		vm.DeleteGlobal(name)
		vm.setPackageLoaded(name, runtime.NilValue())
		if name == "string" {
			vm.stringMeta = nil
		}
	}
}

func (vm *VM) setPackageLoaded(name string, val runtime.Value) {
	pkgVal, ok := vm.globals["package"]
	if !ok || !pkgVal.IsTable() {
		return
	}
	loaded := pkgVal.Table().RawGet(runtime.StringValue("loaded"))
	if loaded.IsTable() {
		loaded.Table().RawSetString(name, val)
	}
}

// RegisterProtectedCallLib installs VM-aware pcall/xpcall builtins so protected
// calls can invoke ordinary VM closures.
func (vm *VM) RegisterProtectedCallLib() {
	vm.SetGlobal("pcall", runtime.FunctionValue(vm.newPCallFunction()))
	vm.SetGlobal("xpcall", runtime.FunctionValue(vm.newXPCallFunction()))
}

// RegisterTestkitLib installs VM-aware testkit helpers for APIs that need to
// call or introspect ordinary VM closures. Pure runtime diagnostics stay in
// the runtime-provided testkit table.
func (vm *VM) RegisterTestkitLib() {
	val, ok := vm.globals["testkit"]
	if !ok || !val.IsTable() {
		return
	}
	lib := val.Table()
	lib.RawSetString("protect", runtime.FunctionValue(&runtime.GoFunction{
		Name: "testkit.protect",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'testkit.protect' (function expected)")
			}
			if !args[0].IsFunction() {
				return nil, fmt.Errorf("bad argument #1 to 'testkit.protect' (function expected)")
			}
			results, err := vm.callValue(args[0], args[1:])
			out := runtime.NewTable()
			if err != nil {
				out.RawSetString("ok", runtime.BoolValue(false))
				out.RawSetString("error", protectedErrorValue(err))
				return []runtime.Value{runtime.TableValue(out)}, nil
			}
			values := runtime.NewTable()
			for i, result := range results {
				values.RawSet(runtime.IntValue(int64(i+1)), result)
			}
			out.RawSetString("ok", runtime.BoolValue(true))
			out.RawSetString("values", runtime.TableValue(values))
			out.RawSetString("n", runtime.IntValue(int64(len(results))))
			return []runtime.Value{runtime.TableValue(out)}, nil
		},
	}))
	lib.RawSetString("functionInfo", runtime.FunctionValue(&runtime.GoFunction{
		Name: "testkit.functionInfo",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsFunction() {
				return nil, fmt.Errorf("bad argument #1 to 'testkit.functionInfo' (function expected)")
			}
			out := runtime.NewTable()
			out.RawSetString("type", runtime.StringValue("function"))
			out.RawSetString("raw", runtime.StringValue(fmt.Sprintf("0x%x", args[0].Raw())))
			out.RawSetString("identity", runtime.StringValue(fmt.Sprintf("function:%x", args[0].Raw())))
			if gf := args[0].GoFunction(); gf != nil {
				out.RawSetString("name", runtime.StringValue(gf.Name))
				out.RawSetString("kind", runtime.StringValue("native"))
				return []runtime.Value{runtime.TableValue(out)}, nil
			}
			if cl, ok := closureFromValue(args[0]); ok && cl != nil && cl.Proto != nil {
				name := cl.Proto.Name
				if name == "" {
					name = "<anonymous>"
				}
				out.RawSetString("name", runtime.StringValue(name))
				out.RawSetString("kind", runtime.StringValue("script"))
				if cl.Proto.Source != "" {
					out.RawSetString("sourceName", runtime.StringValue(cl.Proto.Source))
				}
				if cl.Proto.LineDefined > 0 {
					out.RawSetString("line", runtime.IntValue(int64(cl.Proto.LineDefined)))
					out.RawSetString("column", runtime.IntValue(1))
				}
				out.RawSetString("params", runtime.IntValue(int64(cl.Proto.NumParams)))
				out.RawSetString("vararg", runtime.BoolValue(cl.Proto.IsVarArg))
				out.RawSetString("upvalues", runtime.IntValue(int64(len(cl.Upvalues))))
				return []runtime.Value{runtime.TableValue(out)}, nil
			}
			out.RawSetString("name", runtime.StringValue("<unknown>"))
			out.RawSetString("kind", runtime.StringValue("unknown"))
			return []runtime.Value{runtime.TableValue(out)}, nil
		},
	}))
}

// RegisterToStringLib installs a VM-aware tostring builtin so __tostring
// metamethods implemented as VM closures can be invoked correctly.
func (vm *VM) RegisterToStringLib() {
	vm.SetGlobal("tostring", runtime.FunctionValue(&runtime.GoFunction{
		Name: "tostring",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'tostring' (value expected)")
			}
			s, err := vm.luaToString(args[0])
			if err != nil {
				return nil, err
			}
			return []runtime.Value{runtime.StringValue(s)}, nil
		},
		FastArg1: func(arg runtime.Value) (runtime.Value, error) {
			s, err := vm.luaToString(arg)
			if err != nil {
				return runtime.NilValue(), err
			}
			return runtime.StringValue(s), nil
		},
		Fast1: func(args []runtime.Value) (runtime.Value, error) {
			if len(args) == 0 {
				return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'tostring' (value expected)")
			}
			s, err := vm.luaToString(args[0])
			if err != nil {
				return runtime.NilValue(), err
			}
			return runtime.StringValue(s), nil
		},
	}))
}

func (vm *VM) executeStdSelectCall(absSlot, nArgs, rawC int, gf *runtime.GoFunction) error {
	if nArgs == 0 || absSlot+1 >= len(vm.regs) {
		return fmt.Errorf("bad argument #1 to 'select'")
	}
	start, countOnly, err := runtime.SelectReturnRange(vm.regs[absSlot+1], nArgs)
	if err != nil {
		return err
	}
	runtime.RecordRuntimePathNativeCallFastFor(gf)
	if countOnly {
		vm.storeStdSelectOne(absSlot, rawC, runtime.IntValue(int64(start)))
		return nil
	}
	valueStart := absSlot + 1 + start
	valueEnd := absSlot + 1 + nArgs
	if start >= nArgs || valueStart >= len(vm.regs) {
		vm.storeStdSelectResults(absSlot, rawC, nil)
		return nil
	}
	if valueEnd > len(vm.regs) {
		valueEnd = len(vm.regs)
	}
	vm.storeStdSelectRange(absSlot, rawC, valueStart, valueEnd)
	return nil
}

func (vm *VM) executeStdSelectVarargCall(absSlot, rawC int, selector runtime.Value, varargs []runtime.Value, gf *runtime.GoFunction) error {
	if rawC == 2 {
		if selector.IsString() && selector.Str() == "#" {
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			vm.regs[absSlot] = runtime.IntValue(int64(len(varargs)))
			return nil
		}
		if selector.RawType() == runtime.TypeInt {
			idx := int(selector.RawInt())
			argCount := len(varargs) + 1
			if idx < 0 {
				idx = argCount + idx
			}
			if idx < 1 {
				return fmt.Errorf("bad argument #1 to 'select' (index out of range)")
			}
			runtime.RecordRuntimePathNativeCallFastFor(gf)
			if idx > len(varargs) {
				vm.regs[absSlot] = runtime.NilValue()
			} else {
				vm.regs[absSlot] = varargs[idx-1]
			}
			return nil
		}
	}
	start, countOnly, err := runtime.SelectReturnRange(selector, len(varargs)+1)
	if err != nil {
		return err
	}
	runtime.RecordRuntimePathNativeCallFastFor(gf)
	if countOnly {
		vm.storeStdSelectOne(absSlot, rawC, runtime.IntValue(int64(start)))
		return nil
	}
	if start > len(varargs) {
		vm.storeStdSelectResults(absSlot, rawC, nil)
		return nil
	}
	vm.storeStdSelectResults(absSlot, rawC, varargs[start-1:])
	return nil
}

func (vm *VM) tryExecuteStdSelectVarargPeephole(frame *CallFrame, base, varargA int, varargs []runtime.Value) (bool, error) {
	if frame.pc >= len(frame.closure.Proto.Code) || varargA < 2 {
		return false, nil
	}
	callInst := frame.closure.Proto.Code[frame.pc]
	if DecodeOp(callInst) != OP_CALL || DecodeB(callInst) != 0 {
		return false, nil
	}
	callA := DecodeA(callInst)
	if callA+2 != varargA {
		return false, nil
	}
	absSlot := base + callA
	if absSlot+1 >= len(vm.regs) {
		return false, nil
	}
	gf := vm.regs[absSlot].GoFunction()
	if gf == nil || gf.NativeKind != runtime.NativeKindStdSelect || gf.NativeData != runtime.StdSelectIdentityPtr() {
		return false, nil
	}
	if err := vm.executeStdSelectVarargCall(absSlot, DecodeC(callInst), vm.regs[absSlot+1], varargs, gf); err != nil {
		return true, err
	}
	frame.pc++
	return true, nil
}

func (vm *VM) storeStdSelectOne(absSlot, rawC int, value runtime.Value) {
	if rawC == 0 {
		if absSlot < len(vm.regs) {
			vm.regs[absSlot] = value
		}
		vm.top = absSlot + 1
		return
	}
	nr := rawC - 1
	if nr <= 0 {
		return
	}
	if absSlot < len(vm.regs) {
		vm.regs[absSlot] = value
	}
	for i := 1; i < nr; i++ {
		if idx := absSlot + i; idx < len(vm.regs) {
			vm.regs[idx] = runtime.NilValue()
		}
	}
}

func (vm *VM) storeStdSelectRange(absSlot, rawC, valueStart, valueEnd int) {
	if valueEnd < valueStart {
		valueEnd = valueStart
	}
	if rawC == 0 {
		n := valueEnd - valueStart
		for i := 0; i < n; i++ {
			dst := absSlot + i
			src := valueStart + i
			if dst < len(vm.regs) && src < len(vm.regs) {
				vm.regs[dst] = vm.regs[src]
			}
		}
		vm.top = absSlot + n
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		dst := absSlot + i
		if dst >= len(vm.regs) {
			continue
		}
		src := valueStart + i
		if src < valueEnd && src < len(vm.regs) {
			vm.regs[dst] = vm.regs[src]
		} else {
			vm.regs[dst] = runtime.NilValue()
		}
	}
}

func (vm *VM) storeStdSelectResults(absSlot, rawC int, results []runtime.Value) {
	if rawC == 0 {
		for i, r := range results {
			if idx := absSlot + i; idx < len(vm.regs) {
				vm.regs[idx] = r
			}
		}
		vm.top = absSlot + len(results)
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		idx := absSlot + i
		if idx >= len(vm.regs) {
			continue
		}
		if i < len(results) {
			vm.regs[idx] = results[i]
		} else {
			vm.regs[idx] = runtime.NilValue()
		}
	}
}

// ExecuteStdIPairsCall handles the standard multi-return ipairs setup without
// routing through the generic GoFunction adapter.
func (vm *VM) ExecuteStdIPairsCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs == 0 || absSlot+1 >= len(vm.regs) || !vm.regs[absSlot+1].IsTable() {
		return true, fmt.Errorf("bad argument #1 to 'ipairs' (table expected)")
	}
	runtime.RecordRuntimePathNativeCallFastFor(vm.regs[absSlot].GoFunction())
	vm.storeStdSelectResults(absSlot, rawC, []runtime.Value{
		runtime.FunctionValue(vm.ipairsIteratorFn),
		vm.regs[absSlot+1],
		runtime.IntValue(0),
	})
	return true, nil
}

// ExecuteStdPairsCall handles the ordinary-table standard pairs setup. Tables
// with a __pairs metamethod deliberately fall back to the GoFunction body so
// the existing callback/yield diagnostics stay centralized.
func (vm *VM) ExecuteStdPairsCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs == 0 || absSlot+1 >= len(vm.regs) || !vm.regs[absSlot+1].IsTable() {
		return true, fmt.Errorf("bad argument #1 to 'pairs' (table expected)")
	}
	tbl := vm.regs[absSlot+1].Table()
	if mt := tbl.GetMetatable(); mt != nil && !mt.RawGetString("__pairs").IsNil() {
		return false, nil
	}
	runtime.RecordRuntimePathNativeCallFastFor(vm.regs[absSlot].GoFunction())
	vm.storeStdSelectResults(absSlot, rawC, []runtime.Value{
		runtime.FunctionValue(vm.newPairsIteratorFunction(tbl)),
		vm.regs[absSlot+1],
		runtime.NilValue(),
	})
	return true, nil
}

func (vm *VM) luaToString(v runtime.Value) (string, error) {
	if v.IsTable() {
		if mt := v.Table().GetMetatable(); mt != nil {
			if mm := mt.RawGetString("__tostring"); !mm.IsNil() {
				results, err := vm.callValue(mm, []runtime.Value{v})
				if err != nil {
					return "", err
				}
				if len(results) == 0 || !results[0].IsString() {
					return "", fmt.Errorf("'__tostring' must return a string")
				}
				return results[0].Str(), nil
			}
			if name := mt.RawGetString("__name"); name.IsString() {
				return name.Str() + ": " + strings.TrimPrefix(v.String(), "table: "), nil
			}
		}
	}
	return v.String(), nil
}

func protectedErrorValue(err error) runtime.Value {
	var luaErr *runtime.LuaError
	if errors.As(err, &luaErr) {
		return luaErr.Value
	}
	return runtime.StringValue(err.Error())
}

func (vm *VM) newPCallFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "pcall",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("bad argument #1 to 'pcall' (value expected)")
			}
			results, err := vm.callValue(args[0], args[1:])
			if err != nil {
				return []runtime.Value{runtime.BoolValue(false), protectedErrorValue(err)}, nil
			}
			return append([]runtime.Value{runtime.BoolValue(true)}, results...), nil
		},
	}
}

func (vm *VM) newXPCallFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "xpcall",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument #%d to 'xpcall' (value expected)", len(args)+1)
			}
			results, err := vm.callValue(args[0], args[2:])
			if err == nil {
				return append([]runtime.Value{runtime.BoolValue(true)}, results...), nil
			}
			handlerResults, handlerErr := vm.callValue(args[1], []runtime.Value{protectedErrorValue(err)})
			if handlerErr != nil {
				return []runtime.Value{runtime.BoolValue(false), protectedErrorValue(handlerErr)}, nil
			}
			msg := runtime.NilValue()
			if len(handlerResults) > 0 {
				msg = handlerResults[0]
			}
			return []runtime.Value{runtime.BoolValue(false), msg}, nil
		},
	}
}

// RegisterPairsLib installs a VM-aware pairs builtin so __pairs metamethods
// can be ordinary VM closures.
func (vm *VM) RegisterPairsLib() {
	vm.SetGlobal("pairs", runtime.FunctionValue(vm.newPairsFunction()))
}

// RegisterTableSortLib installs a VM-aware table.sort so file-loaded VM
// closures can be used as comparators.
func (vm *VM) RegisterTableSortLib() {
	tblVal, ok := vm.globals["table"]
	if !ok || !tblVal.IsTable() {
		return
	}
	tblVal.Table().RawSet(runtime.StringValue("sort"), runtime.FunctionValue(vm.newTableSortFunction()))
}

func (vm *VM) newTableSortFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.sort",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.sort' (table expected)")
			}
			t := args[0]
			length, err := vm.tableLenInt(t)
			if err != nil {
				return nil, err
			}
			if length < 0 {
				length = 0
			}
			elems := make([]runtime.Value, int(length))
			for i := 0; i < len(elems); i++ {
				v, err := vm.tableGet(t, runtime.IntValue(int64(i+1)))
				if err != nil {
					return nil, err
				}
				elems[i] = v
			}

			var sortErr error
			if len(args) >= 2 && args[1].IsFunction() {
				comp := args[1]
				sort.SliceStable(elems, func(a, b int) bool {
					if sortErr != nil {
						return false
					}
					results, err := vm.callValue(comp, []runtime.Value{elems[a], elems[b]})
					if err != nil {
						sortErr = err
						return false
					}
					if len(results) > 0 && results[0].Truthy() {
						reverse, err := vm.callValue(comp, []runtime.Value{elems[b], elems[a]})
						if err != nil {
							sortErr = err
							return false
						}
						if len(reverse) > 0 && reverse[0].Truthy() {
							sortErr = fmt.Errorf("invalid order function for sorting")
							return false
						}
						return true
					}
					return false
				})
			} else {
				sort.SliceStable(elems, func(a, b int) bool {
					if sortErr != nil {
						return false
					}
					less, err := vm.valueLessThan(elems[a], elems[b])
					if err != nil {
						sortErr = err
						return false
					}
					return less
				})
			}
			if sortErr != nil {
				return nil, sortErr
			}
			for i, val := range elems {
				if err := vm.tableSet(t, runtime.IntValue(int64(i+1)), val); err != nil {
					return nil, err
				}
			}
			return nil, nil
		},
	}
}

// RegisterTableHigherOrderLib installs VM-aware table higher-order helpers so
// file-loaded VM closures can be used as callbacks.
func (vm *VM) RegisterTableHigherOrderLib() {
	tblVal, ok := vm.globals["table"]
	if !ok || !tblVal.IsTable() {
		return
	}
	tbl := tblVal.Table()
	tbl.RawSet(runtime.StringValue("filter"), runtime.FunctionValue(vm.newTableFilterFunction()))
	tbl.RawSet(runtime.StringValue("map"), runtime.FunctionValue(vm.newTableMapFunction()))
	tbl.RawSet(runtime.StringValue("reduce"), runtime.FunctionValue(vm.newTableReduceFunction()))
	tbl.RawSet(runtime.StringValue("fromArray"), runtime.FunctionValue(vm.newTableFromArrayFunction()))
}

func (vm *VM) newTableFilterFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.filter",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.filter'")
			}
			src := args[0].Table()
			fn := args[1]
			result := runtime.NewTable()
			out := int64(1)
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				results, err := vm.callValue(fn, []runtime.Value{v, runtime.IntValue(i)})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 && results[0].Truthy() {
					result.RawSet(runtime.IntValue(out), v)
					out++
				}
			}
			return []runtime.Value{runtime.TableValue(result)}, nil
		},
	}
}

func (vm *VM) newTableMapFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.map",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.map'")
			}
			src := args[0].Table()
			fn := args[1]
			result := runtime.NewTable()
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				results, err := vm.callValue(fn, []runtime.Value{v, runtime.IntValue(i)})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 {
					result.RawSet(runtime.IntValue(i), results[0])
				} else {
					result.RawSet(runtime.IntValue(i), runtime.NilValue())
				}
			}
			return []runtime.Value{runtime.TableValue(result)}, nil
		},
	}
}

func (vm *VM) newTableReduceFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.reduce",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 3 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.reduce'")
			}
			src := args[0].Table()
			fn := args[1]
			acc := args[2]
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				results, err := vm.callValue(fn, []runtime.Value{acc, v})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 {
					acc = results[0]
				}
			}
			return []runtime.Value{acc}, nil
		},
	}
}

func (vm *VM) newTableFromArrayFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "table.fromArray",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.fromArray'")
			}
			src := args[0].Table()
			fn := args[1]
			result := runtime.NewTable()
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				keys, err := vm.callValue(fn, []runtime.Value{v})
				if err != nil {
					return nil, err
				}
				if len(keys) > 0 {
					result.RawSet(keys[0], v)
				}
			}
			return []runtime.Value{runtime.TableValue(result)}, nil
		},
	}
}

// RegisterSortCallbackLib installs VM-aware sort namespace helpers whose
// callbacks may be file-loaded VM closures.
func (vm *VM) RegisterSortCallbackLib() {
	sortVal, ok := vm.globals["sort"]
	if !ok || !sortVal.IsTable() {
		return
	}
	tbl := sortVal.Table()
	tbl.RawSet(runtime.StringValue("by"), runtime.FunctionValue(vm.newSortByFunction()))
	tbl.RawSet(runtime.StringValue("byKey"), runtime.FunctionValue(vm.newSortByKeyFunction()))
	tbl.RawSet(runtime.StringValue("partition"), runtime.FunctionValue(vm.newSortPartitionFunction()))
	tbl.RawSet(runtime.StringValue("min"), runtime.FunctionValue(vm.newSortMinMaxFunction(false)))
	tbl.RawSet(runtime.StringValue("max"), runtime.FunctionValue(vm.newSortMinMaxFunction(true)))
}

func sortArrayValues(tbl *runtime.Table) []runtime.Value {
	length := tbl.Length()
	elems := make([]runtime.Value, length)
	for i := 0; i < length; i++ {
		elems[i] = tbl.RawGet(runtime.IntValue(int64(i + 1)))
	}
	return elems
}

func writeSortArrayValues(tbl *runtime.Table, elems []runtime.Value) {
	for i, v := range elems {
		tbl.RawSet(runtime.IntValue(int64(i+1)), v)
	}
}

func (vm *VM) newSortByFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "sort.by",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad arguments to 'sort.by' (table and function expected)")
			}
			tbl := args[0].Table()
			fn := args[1]
			elems := sortArrayValues(tbl)
			var sortErr error
			sort.SliceStable(elems, func(a, b int) bool {
				if sortErr != nil {
					return false
				}
				results, err := vm.callValue(fn, []runtime.Value{elems[a], elems[b]})
				if err != nil {
					sortErr = err
					return false
				}
				return len(results) > 0 && results[0].Truthy()
			})
			if sortErr != nil {
				return nil, sortErr
			}
			writeSortArrayValues(tbl, elems)
			return []runtime.Value{args[0]}, nil
		},
	}
}

func (vm *VM) newSortByKeyFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "sort.byKey",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad arguments to 'sort.byKey' (table and function expected)")
			}
			tbl := args[0].Table()
			fn := args[1]
			elems := sortArrayValues(tbl)
			type keyedValue struct {
				value runtime.Value
				key   runtime.Value
			}
			pairs := make([]keyedValue, len(elems))
			for i, elem := range elems {
				results, err := vm.callValue(fn, []runtime.Value{elem})
				if err != nil {
					return nil, err
				}
				pairs[i].value = elem
				if len(results) > 0 {
					pairs[i].key = results[0]
				} else {
					pairs[i].key = runtime.NilValue()
				}
			}
			var sortErr error
			sort.SliceStable(pairs, func(a, b int) bool {
				if sortErr != nil {
					return false
				}
				less, err := vm.valueLessThan(pairs[a].key, pairs[b].key)
				if err != nil {
					sortErr = err
					return false
				}
				return less
			})
			if sortErr != nil {
				return nil, sortErr
			}
			for i, pair := range pairs {
				elems[i] = pair.value
			}
			writeSortArrayValues(tbl, elems)
			return []runtime.Value{args[0]}, nil
		},
	}
}

func (vm *VM) newSortPartitionFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "sort.partition",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad arguments to 'sort.partition' (table and function expected)")
			}
			src := args[0].Table()
			fn := args[1]
			truthy := runtime.NewTable()
			falsy := runtime.NewTable()
			trueIdx := int64(1)
			falseIdx := int64(1)
			for i := int64(1); i <= int64(src.Length()); i++ {
				v := src.RawGet(runtime.IntValue(i))
				results, err := vm.callValue(fn, []runtime.Value{v})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 && results[0].Truthy() {
					truthy.RawSet(runtime.IntValue(trueIdx), v)
					trueIdx++
				} else {
					falsy.RawSet(runtime.IntValue(falseIdx), v)
					falseIdx++
				}
			}
			return []runtime.Value{runtime.TableValue(truthy), runtime.TableValue(falsy)}, nil
		},
	}
}

func (vm *VM) newSortMinMaxFunction(max bool) *runtime.GoFunction {
	name := "sort.min"
	if max {
		name = "sort.max"
	}
	return &runtime.GoFunction{
		Name: name,
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to '%s' (table expected)", name)
			}
			src := args[0].Table()
			length := src.Length()
			if length == 0 {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			hasKeyFn := len(args) >= 2 && args[1].IsFunction()
			best := src.RawGet(runtime.IntValue(1))
			bestKey := best
			if hasKeyFn {
				results, err := vm.callValue(args[1], []runtime.Value{best})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 {
					bestKey = results[0]
				}
			}
			for i := int64(2); i <= int64(length); i++ {
				candidate := src.RawGet(runtime.IntValue(i))
				candidateKey := candidate
				if hasKeyFn {
					results, err := vm.callValue(args[1], []runtime.Value{candidate})
					if err != nil {
						return nil, err
					}
					if len(results) > 0 {
						candidateKey = results[0]
					}
				}
				less, err := vm.valueLessThan(candidateKey, bestKey)
				if err != nil {
					return nil, err
				}
				if !max && less {
					best = candidate
					bestKey = candidateKey
					continue
				}
				if max {
					reverseLess, err := vm.valueLessThan(bestKey, candidateKey)
					if err != nil {
						return nil, err
					}
					if reverseLess {
						best = candidate
						bestKey = candidateKey
					}
				}
			}
			return []runtime.Value{best}, nil
		},
	}
}

// RegisterTableProxyLib installs VM-aware table functions that honor __index,
// __newindex, and __len for proxy tables.
func (vm *VM) RegisterTableProxyLib() {
	tblVal, ok := vm.globals["table"]
	if !ok || !tblVal.IsTable() {
		return
	}
	tbl := tblVal.Table()
	insert := func(t, posValue, value runtime.Value, hasPos bool) error {
		if !t.IsTable() {
			return fmt.Errorf("bad argument #1 to 'table.insert' (table expected)")
		}
		length, err := vm.tableLenInt(t)
		if err != nil {
			return err
		}
		if !hasPos {
			if t.Table().TryPlainArrayInsert(int64(length+1), value) {
				return nil
			}
			return vm.tableSet(t, runtime.IntValue(length+1), value)
		}
		pos := vmToInt(posValue)
		if pos < 1 || pos > length+1 {
			return fmt.Errorf("bad argument #2 to 'table.insert' (position out of bounds)")
		}
		if t.Table().TryPlainArrayInsert(pos, value) {
			return nil
		}
		for i := length; i >= pos; i-- {
			v, err := vm.tableGet(t, runtime.IntValue(i))
			if err != nil {
				return err
			}
			if err := vm.tableSet(t, runtime.IntValue(i+1), v); err != nil {
				return err
			}
		}
		return vm.tableSet(t, runtime.IntValue(pos), value)
	}
	remove := func(t, posValue runtime.Value, hasPos bool) (runtime.Value, error) {
		if !t.IsTable() {
			return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'table.remove' (table expected)")
		}
		length, err := vm.tableLenInt(t)
		if err != nil {
			return runtime.NilValue(), err
		}
		pos := length
		if hasPos {
			pos = vmToInt(posValue)
		}
		if pos < 0 || pos > length+1 || (pos == 0 && length > 0) {
			return runtime.NilValue(), fmt.Errorf("bad argument #2 to 'table.remove' (position out of bounds)")
		}
		if pos == length+1 {
			return runtime.NilValue(), nil
		}
		if removed, ok := t.Table().TryPlainArrayRemove(pos); ok {
			return removed, nil
		}
		removed, err := vm.tableGet(t, runtime.IntValue(pos))
		if err != nil {
			return runtime.NilValue(), err
		}
		for i := pos; i < length; i++ {
			v, err := vm.tableGet(t, runtime.IntValue(i+1))
			if err != nil {
				return runtime.NilValue(), err
			}
			if err := vm.tableSet(t, runtime.IntValue(i), v); err != nil {
				return runtime.NilValue(), err
			}
		}
		if err := vm.tableSet(t, runtime.IntValue(length), runtime.NilValue()); err != nil {
			return runtime.NilValue(), err
		}
		return removed, nil
	}
	tbl.RawSet(runtime.StringValue("insert"), runtime.FunctionValue(&runtime.GoFunction{
		Name: "table.insert",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.insert' (table expected)")
			}
			if len(args) != 2 && len(args) != 3 {
				return nil, fmt.Errorf("wrong number of arguments to 'table.insert'")
			}
			if len(args) == 2 {
				return nil, insert(args[0], runtime.NilValue(), args[1], false)
			}
			return nil, insert(args[0], args[1], args[2], true)
		},
		FastArg2: func(t, value runtime.Value) (runtime.Value, error) {
			if err := insert(t, runtime.NilValue(), value, false); err != nil {
				return runtime.NilValue(), err
			}
			return runtime.NilValue(), nil
		},
		FastArg3: func(t, pos, value runtime.Value) (runtime.Value, error) {
			if err := insert(t, pos, value, true); err != nil {
				return runtime.NilValue(), err
			}
			return runtime.NilValue(), nil
		},
	}))
	tbl.RawSet(runtime.StringValue("remove"), runtime.FunctionValue(&runtime.GoFunction{
		Name: "table.remove",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'table.remove' (table expected)")
			}
			pos := runtime.NilValue()
			if len(args) >= 2 {
				pos = args[1]
			}
			removed, err := remove(args[0], pos, len(args) >= 2)
			if err != nil {
				return nil, err
			}
			return []runtime.Value{removed}, nil
		},
		FastArg1: func(t runtime.Value) (runtime.Value, error) {
			return remove(t, runtime.NilValue(), false)
		},
		FastArg2: func(t, pos runtime.Value) (runtime.Value, error) {
			return remove(t, pos, true)
		},
	}))
	tbl.RawSet(runtime.StringValue("concat"), runtime.FunctionValue(&runtime.GoFunction{
		Name: "table.concat",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.concat' (table expected)")
			}
			t := args[0]
			sep := ""
			if len(args) >= 2 && args[1].IsString() {
				sep = args[1].Str()
			}
			i := int64(1)
			j, err := vm.tableLenInt(t)
			if err != nil {
				return nil, err
			}
			if len(args) >= 3 {
				i = vmToInt(args[2])
			}
			if len(args) >= 4 {
				j = vmToInt(args[3])
			}
			parts := make([]string, 0)
			if j >= i {
				parts = make([]string, 0, j-i+1)
			}
			for k := i; k <= j; k++ {
				v, err := vm.tableGet(t, runtime.IntValue(k))
				if err != nil {
					return nil, err
				}
				s, ok := runtime.ConcatOperandString(v)
				if !ok {
					return nil, fmt.Errorf("invalid value at index %d in table for 'concat'", k)
				}
				parts = append(parts, s)
			}
			return []runtime.Value{runtime.StringValue(strings.Join(parts, sep))}, nil
		},
	}))
	tableUnpack := func(name string, args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.%s' (table expected)", name)
		}
		t := args[0]
		i := int64(1)
		j, err := vm.tableLenInt(t)
		if err != nil {
			return nil, err
		}
		if len(args) >= 2 {
			i = vmToInt(args[1])
		}
		if len(args) >= 3 {
			j = vmToInt(args[2])
		}
		count, err := runtime.CheckTableUnpackRange(name, i, j)
		if err != nil {
			return nil, err
		}
		result := make([]runtime.Value, 0, count)
		for k := i; k <= j; k++ {
			v, err := vm.tableGet(t, runtime.IntValue(k))
			if err != nil {
				return nil, err
			}
			result = append(result, v)
		}
		return result, nil
	}
	tbl.RawSet(runtime.StringValue("unpack"), runtime.FunctionValue(&runtime.GoFunction{Name: "table.unpack", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		return tableUnpack("unpack", args)
	}}))
	tbl.RawSet(runtime.StringValue("spread"), runtime.FunctionValue(&runtime.GoFunction{Name: "table.spread", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		return tableUnpack("spread", args)
	}}))
	tbl.RawSet(runtime.StringValue("move"), runtime.FunctionValue(&runtime.GoFunction{
		Name: "table.move",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 4 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument to 'table.move'")
			}
			src := args[0]
			f := vmToInt(args[1])
			e := vmToInt(args[2])
			tPos := vmToInt(args[3])
			dst := src
			if len(args) >= 5 {
				if !args[4].IsTable() {
					return nil, fmt.Errorf("bad argument to 'table.move'")
				}
				dst = args[4]
			}
			if e >= f {
				if dst.Table().TryPlainArrayMove(src.Table(), f, e, tPos) {
					return []runtime.Value{dst}, nil
				}
				count := e - f + 1
				if tPos <= f || src.Table() != dst.Table() {
					for i := int64(0); i < count; i++ {
						v, err := vm.tableGet(src, runtime.IntValue(f+i))
						if err != nil {
							return nil, err
						}
						if err := vm.tableSet(dst, runtime.IntValue(tPos+i), v); err != nil {
							return nil, err
						}
					}
				} else {
					for i := count - 1; i >= 0; i-- {
						v, err := vm.tableGet(src, runtime.IntValue(f+i))
						if err != nil {
							return nil, err
						}
						if err := vm.tableSet(dst, runtime.IntValue(tPos+i), v); err != nil {
							return nil, err
						}
					}
				}
			}
			return []runtime.Value{dst}, nil
		},
	}))
}

// RegisterIPairsLib installs a VM-aware ipairs builtin so ordinary indexing
// during iteration can invoke VM __index closures.
func (vm *VM) RegisterIPairsLib() {
	if vm.ipairsIteratorFn == nil {
		vm.ipairsIteratorFn = vm.newIPairsIteratorFunction()
	}
	vm.SetGlobal("ipairs", runtime.FunctionValue(vm.newIPairsFunction()))
}

func (vm *VM) newIPairsFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "ipairs",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'ipairs' (table expected)")
			}
			return []runtime.Value{runtime.FunctionValue(vm.ipairsIteratorFn), args[0], runtime.IntValue(0)}, nil
		},
		NativeKind: runtime.NativeKindStdIPairs,
		NativeData: runtime.StdIPairsIdentityPtr(),
	}
}

func (vm *VM) newIPairsIteratorFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "ipairs_iterator",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'for iterator' (table expected)")
			}
			i := int64(0)
			if len(args) >= 2 && !args[1].IsNil() {
				if args[1].IsInt() {
					i = args[1].Int()
				} else if args[1].IsFloat() {
					i = int64(args[1].Float())
				} else {
					return nil, fmt.Errorf("bad argument #2 to 'for iterator' (number expected)")
				}
			}
			i++
			key := runtime.IntValue(i)
			v, err := vm.tableGet(args[0], key)
			if err != nil {
				return nil, err
			}
			if v.IsNil() {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			return []runtime.Value{key, v}, nil
		},
		FastArg2Ret2: func(table, keyValue runtime.Value) (runtime.Value, runtime.Value, int, error) {
			if !table.IsTable() {
				return runtime.NilValue(), runtime.NilValue(), 0, fmt.Errorf("bad argument #1 to 'for iterator' (table expected)")
			}
			i := int64(0)
			if !keyValue.IsNil() {
				if keyValue.IsInt() {
					i = keyValue.Int()
				} else if keyValue.IsFloat() {
					i = int64(keyValue.Float())
				} else {
					return runtime.NilValue(), runtime.NilValue(), 0, fmt.Errorf("bad argument #2 to 'for iterator' (number expected)")
				}
			}
			i++
			key := runtime.IntValue(i)
			v, err := vm.tableGet(table, key)
			if err != nil {
				return runtime.NilValue(), runtime.NilValue(), 0, err
			}
			if v.IsNil() {
				return runtime.NilValue(), runtime.NilValue(), 1, nil
			}
			return key, v, 2, nil
		},
	}
}

func (vm *VM) newPairsFunction() *runtime.GoFunction {
	return &runtime.GoFunction{
		Name: "pairs",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'pairs' (table expected)")
			}
			tbl := args[0].Table()
			if mt := tbl.GetMetatable(); mt != nil {
				mm := mt.RawGetString("__pairs")
				if !mm.IsNil() {
					if cl, ok := closureFromValue(mm); ok && vm.activeCoroutine() != nil && protoContainsOp(cl.Proto, OP_YIELD) {
						return nil, fmt.Errorf("__pairs cannot yield through the host pairs() setup; return a coroutine-backed iterator instead")
					}
					if cl, ok := closureFromValue(mm); ok {
						newBase := vm.top
						if vm.frameCount > 0 {
							curFrame := &vm.frames[vm.frameCount-1]
							minBase := curFrame.base + curFrame.closure.Proto.MaxStack
							if newBase < minBase {
								newBase = minBase
							}
						}
						return vm.call(cl, []runtime.Value{args[0]}, newBase, -1)
					}
					return vm.callValue(mm, []runtime.Value{args[0]})
				}
			}
			return []runtime.Value{runtime.FunctionValue(vm.newPairsIteratorFunction(tbl)), args[0], runtime.NilValue()}, nil
		},
		NativeKind: runtime.NativeKindStdPairs,
		NativeData: runtime.StdPairsIdentityPtr(),
	}
}

func (vm *VM) newPairsIteratorFunction(tbl *runtime.Table) *runtime.GoFunction {
	keys := tbl.PairsKeysSnapshot()
	idx := 0
	return &runtime.GoFunction{
		Name: "pairs_iterator",
		Fn: func(_ []runtime.Value) ([]runtime.Value, error) {
			if idx >= len(keys) {
				return []runtime.Value{runtime.NilValue()}, nil
			}
			k := keys[idx]
			idx++
			return []runtime.Value{k, tbl.RawGet(k)}, nil
		},
		FastArg2Ret2: func(_, _ runtime.Value) (runtime.Value, runtime.Value, int, error) {
			if idx >= len(keys) {
				return runtime.NilValue(), runtime.NilValue(), 1, nil
			}
			k := keys[idx]
			idx++
			return k, tbl.RawGet(k), 2, nil
		},
	}
}

// PrepareTier2GlobalArray resolves the requested string constants as indexed
// globals and returns the data needed by the Tier 2 indexed-global fast path.
// The native path is enabled only for single-threaded VMs without per-VM
// overrides; other VM shapes fall back to the existing exit-resume protocol.
func (vm *VM) PrepareTier2GlobalArray(constants []runtime.Value, usedConsts map[int]bool) ([]int32, uintptr, *uint32, uint32, bool) {
	if vm == nil || !vm.noGlobalLock || vm.globalOverrides != nil {
		return nil, 0, nil, 0, false
	}
	indices := make([]int32, len(constants))
	for i := range indices {
		indices[i] = -1
	}
	for constIdx := range usedConsts {
		if constIdx < 0 || constIdx >= len(constants) {
			return nil, 0, nil, 0, false
		}
		c := constants[constIdx]
		if !c.IsString() {
			return nil, 0, nil, 0, false
		}
		idx := vm.resolveGlobalIndex(c.Str())
		indices[constIdx] = int32(idx)
	}
	if len(vm.globalArray) == 0 {
		return indices, 0, &vm.globalVer, vm.globalVer, true
	}
	return indices, uintptr(unsafe.Pointer(&vm.globalArray[0])), &vm.globalVer, vm.globalVer, true
}

// Tier2GlobalArrayState returns the current indexed-global backing pointer and
// version state for a previously prepared Tier 2 global index map.
func (vm *VM) Tier2GlobalArrayState() (uintptr, *uint32, uint32, bool) {
	if vm == nil || !vm.noGlobalLock || vm.globalOverrides != nil {
		return 0, nil, 0, false
	}
	if len(vm.globalArray) == 0 {
		return 0, &vm.globalVer, vm.globalVer, true
	}
	return uintptr(unsafe.Pointer(&vm.globalArray[0])), &vm.globalVer, vm.globalVer, true
}

// SyncTier2GlobalMap mirrors indexed global values back into the legacy globals
// map for names written natively by Tier 2. VM.GetGlobal reads globalArray, but
// the map is still part of the VM's public interop surface.
func (vm *VM) SyncTier2GlobalMap(constants []runtime.Value, indices []int32, constSet map[int]bool) {
	if vm == nil || len(indices) == 0 || len(constSet) == 0 {
		return
	}
	if vm.noGlobalLock {
		for constIdx := range constSet {
			if constIdx < 0 || constIdx >= len(constants) || constIdx >= len(indices) {
				continue
			}
			idx := int(indices[constIdx])
			if idx < 0 || idx >= len(vm.globalArray) || !constants[constIdx].IsString() {
				continue
			}
			vm.globals[constants[constIdx].Str()] = vm.globalArray[idx]
		}
		return
	}
	vm.globalsMu.Lock()
	for constIdx := range constSet {
		if constIdx < 0 || constIdx >= len(constants) || constIdx >= len(indices) {
			continue
		}
		idx := int(indices[constIdx])
		if idx < 0 || idx >= len(vm.globalArray) || !constants[constIdx].IsString() {
			continue
		}
		vm.globals[constants[constIdx].Str()] = vm.globalArray[idx]
	}
	vm.globalsMu.Unlock()
}

func (vm *VM) setGlobalOverride(name string, val runtime.Value) {
	if vm.globalOverrides == nil {
		vm.globalOverrides = make(map[string]runtime.Value, 1)
	}
	vm.globalOverrides[name] = val
	vm.globalOverrideFast = -1
	if vm.globalOverrideIdx == nil {
		vm.globalOverrideIdx = make(map[int]runtime.Value, 1)
	}
	if vm.noGlobalLock {
		if idx, ok := vm.globalIndex[name]; ok {
			vm.globalOverrideIdx[idx] = val
			if len(vm.globalOverrides) == 1 {
				vm.globalOverrideFast = idx
				vm.globalOverrideVal = val
			}
		}
		return
	}
	vm.globalsMu.RLock()
	idx, ok := vm.globalIndex[name]
	vm.globalsMu.RUnlock()
	if ok {
		vm.globalOverrideIdx[idx] = val
		if len(vm.globalOverrides) == 1 {
			vm.globalOverrideFast = idx
			vm.globalOverrideVal = val
		}
	}
}

// resolveGlobalIndex returns the globalArray index for a global name,
// creating a new entry if it doesn't exist.
func (vm *VM) resolveGlobalIndex(name string) int {
	if idx, ok := vm.globalIndex[name]; ok {
		return idx
	}
	// New global — add to array
	idx := len(vm.globalArray)
	val, ok := vm.globals[name]
	if !ok {
		val = runtime.NilValue()
	}
	vm.globalArray = append(vm.globalArray, val)
	vm.globalIndex[name] = idx
	vm.globalVer++
	return idx
}

// New creates a new VM with the given globals.
func New(globals map[string]runtime.Value) *VM {
	// Build indexed global array from the initial map
	ga := make([]runtime.Value, 0, len(globals))
	gi := make(map[string]int, len(globals))
	for name, val := range globals {
		gi[name] = len(ga)
		ga = append(ga, val)
	}

	v := &VM{
		regs:               runtime.MakeNilSlice(1024),
		frames:             make([]CallFrame, initialCallFrameCapacity),
		globals:            globals,
		globalArray:        ga,
		globalIndex:        gi,
		globalOverrideFast: -1,
		globalsMu:          &sync.RWMutex{},
		noGlobalLock:       true, // single-threaded by default
	}
	v.RegisterCoroutineLib()
	v.RegisterProtectedCallLib()
	v.RegisterTestkitLib()
	v.RegisterToStringLib()
	v.RegisterIPairsLib()
	v.RegisterPairsLib()
	v.RegisterTableProxyLib()
	v.RegisterTableSortLib()
	v.RegisterSortCallbackLib()
	v.RegisterTableHigherOrderLib()
	v.RegisterStringLib()
	v.RegisterHTTPLib()
	v.RegisterDebugLib()
	v.RegisterScriptLib()
	v.RegisterLoaderLib()
	v.registerChannelBuiltins()
	runtime.RegisterVM(v)
	return v
}

// Close unregisters this VM from the GC root scanner.
// Should be called when the VM is no longer needed.
func (vm *VM) Close() {
	runtime.UnregisterVM(vm)
}

// ScanGCRoots implements runtime.GCRootScanner. It visits all live GC root
// pointers reachable from this VM: registers, globals, open upvalues, call
// frame closures, proto constants, and recursively all table contents.
func (vm *VM) ScanGCRoots(visitor func(unsafe.Pointer)) {
	seen := make(map[uintptr]struct{}, 256)
	seenProtos := make(map[*FuncProto]struct{}, 32)

	// Scan the entire register file conservatively.
	// The previous optimization (capping at frames[-1].base + maxStack) missed
	// registers used by JIT self-calls: the JIT advances mRegRegs by calleeBaseOff
	// per recursive level without pushing vm.frames entries.  A table referenced
	// only from a deep self-call register would be invisible to gcCompact, causing
	// premature eviction from gcLog and subsequent use-after-free crashes.
	// Scanning all registers is safe: nil/float/int values return immediately in
	// ScanValueRoots, and old stale pointers in unused slots keep their referents
	// alive until those slots are overwritten — a minor delay in GC, not a leak.
	for i := 0; i < len(vm.regs); i++ {
		runtime.ScanValueRoots(vm.regs[i], visitor, seen)
	}

	// Scan globals array.
	for _, v := range vm.globalArray {
		runtime.ScanValueRoots(v, visitor, seen)
	}

	// Scan open upvalues (closed upvalues point into registers already scanned).
	for _, uv := range vm.openUpvals {
		if uv != nil {
			runtime.ScanValueRoots(uv.Get(), visitor, seen)
		}
	}

	// Scan call frame closures, their upvalues, and their proto constants.
	for i := 0; i < vm.frameCount; i++ {
		f := &vm.frames[i]
		if f.closure != nil {
			for _, uv := range f.closure.Upvalues {
				if uv != nil && !uv.open {
					runtime.ScanValueRoots(uv.Get(), visitor, seen)
				}
			}
			// Scan the proto's constants and nested protos recursively.
			scanProtoRoots(f.closure.Proto, visitor, seen, seenProtos)
		}
		// Scan varargs.
		for _, v := range f.varargs {
			runtime.ScanValueRoots(v, visitor, seen)
		}
	}

	// Scan call/result scratch buffers that may contain live values from recent calls.
	for _, v := range vm.argBuf {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	for _, v := range vm.retBuf {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	for _, v := range vm.coroutineResultBuf {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	for _, v := range vm.callSiteValueBuf {
		runtime.ScanValueRoots(v, visitor, seen)
	}

	// Scan string metatable.
	if vm.stringMeta != nil {
		mp := unsafe.Pointer(vm.stringMeta)
		if _, already := seen[uintptr(mp)]; !already {
			seen[uintptr(mp)] = struct{}{}
			visitor(mp)
			runtime.ScanTableRootsExported(vm.stringMeta, visitor, seen)
		}
	}
}

// scanProtoRoots scans a FuncProto's constants and recursively its children.
func scanProtoRoots(proto *FuncProto, visitor func(unsafe.Pointer), seen map[uintptr]struct{}, seenProtos map[*FuncProto]struct{}) {
	if proto == nil {
		return
	}
	if _, already := seenProtos[proto]; already {
		return
	}
	seenProtos[proto] = struct{}{}

	// Scan constants (contains string literals, function values, etc.)
	for _, v := range proto.Constants {
		runtime.ScanValueRoots(v, visitor, seen)
	}

	// Recursively scan nested function prototypes.
	for _, child := range proto.Protos {
		scanProtoRoots(child, visitor, seen, seenProtos)
	}
}

// newChildVM creates a child VM that shares globals with the parent.
// Used by coroutines which need to see the caller's global state.
func newChildVM(parent *VM, co *VMCoroutine) *VM {
	child := &VM{
		regs:               runtime.MakeNilSlice(1024),
		frames:             make([]CallFrame, initialCallFrameCapacity),
		globals:            parent.globals,
		globalArray:        parent.globalArray,
		globalIndex:        parent.globalIndex,
		globalVer:          parent.globalVer,
		globalValueVer:     parent.globalValueVer,
		globalOverrideFast: -1,
		globalsMu:          parent.globalsMu,
		noGlobalLock:       false, // shared globals, must lock
		stringMeta:         parent.stringMeta,
		currentCoroutine:   co,
		coroutineStats:     parent.coroutineStats,
	}
	child.setGlobalOverride("coroutine", runtime.TableValue(child.newCoroutineLib()))
	child.setGlobalOverride("pcall", runtime.FunctionValue(child.newPCallFunction()))
	child.setGlobalOverride("xpcall", runtime.FunctionValue(child.newXPCallFunction()))
	child.setGlobalOverride("ipairs", runtime.FunctionValue(child.newIPairsFunction()))
	child.setGlobalOverride("pairs", runtime.FunctionValue(child.newPairsFunction()))
	child.setGlobalOverride("debug", runtime.TableValue(child.newDebugLib()))
	runtime.RegisterVM(child)
	return child
}

// newIsolatedChildVM creates a child VM with a snapshot of the parent's globals.
// Used by OP_GO goroutines for lock-free reads. Shared heap objects (tables,
// channels) remain shared via pointers; globals array and index are copied.
func newIsolatedChildVM(parent *VM) *VM {
	// Copy both globalArray and globalIndex for full isolation
	ga := make([]runtime.Value, len(parent.globalArray))
	copy(ga, parent.globalArray)

	gi := make(map[string]int, len(parent.globalIndex))
	for k, v := range parent.globalIndex {
		gi[k] = v
	}

	childGlobals := make(map[string]runtime.Value, len(gi))
	for name, idx := range gi {
		childGlobals[name] = ga[idx]
	}

	child := &VM{
		regs:               runtime.MakeNilSlice(1024),
		frames:             make([]CallFrame, initialCallFrameCapacity),
		globals:            childGlobals,
		globalArray:        ga,
		globalIndex:        gi,
		globalVer:          parent.globalVer,
		globalValueVer:     parent.globalValueVer,
		globalOverrideFast: -1,
		globalsMu:          &sync.RWMutex{},
		noGlobalLock:       true, // own copy, fully lock-free
		stringMeta:         parent.stringMeta,
		coroutineStats:     parent.coroutineStats,
	}
	child.RegisterCoroutineLib()
	child.RegisterProtectedCallLib()
	child.RegisterTestkitLib()
	child.RegisterToStringLib()
	child.RegisterIPairsLib()
	child.RegisterPairsLib()
	child.RegisterTableProxyLib()
	child.RegisterTableSortLib()
	child.RegisterSortCallbackLib()
	child.RegisterTableHigherOrderLib()
	child.RegisterStringLib()
	child.RegisterHTTPLib()
	child.RegisterDebugLib()
	child.RegisterScriptLib()
	child.RegisterLoaderLib()
	child.scriptDir = parent.scriptDir
	runtime.RegisterVM(child)
	return child
}

// registerChannelBuiltins adds channel-related builtins to globals.
func (vm *VM) registerChannelBuiltins() {
	vm.SetGlobal("close", runtime.FunctionValue(&runtime.GoFunction{
		Name: "close",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 || !args[0].IsChannel() {
				return nil, fmt.Errorf("close expects a channel")
			}
			ch := args[0].Channel()
			if err := ch.Close(); err != nil {
				return nil, err
			}
			return nil, nil
		},
	}))
}

// SetStringMeta sets the string metatable.
func (vm *VM) SetStringMeta(meta *runtime.Table) {
	vm.stringMeta = meta
}

// RegisterStringLib installs VM-aware string callbacks such as function-valued
// string.gsub replacements.
func (vm *VM) RegisterStringLib() {
	var strLib *runtime.Table
	if existing, ok := vm.globals["string"]; ok && existing.IsTable() {
		strLib = runtime.RefreshStringLibWithCaller(existing.Table(), vm.callValue)
	} else {
		strLib = runtime.BuildStringLibWithCaller(vm.callValue)
		vm.SetGlobal("string", runtime.TableValue(strLib))
	}
	meta := runtime.NewTable()
	meta.RawSet(runtime.StringValue("__index"), runtime.TableValue(strLib))
	vm.stringMeta = meta
	vm.setPackageLoaded("string", runtime.TableValue(strLib))
}

func (vm *VM) RegisterHTTPLib() {
	httpLib := runtime.TableValue(runtime.BuildHTTPLibWithCaller(vm.callValue))
	vm.SetGlobal("http", httpLib)
	vm.setPackageLoaded("http", httpLib)
}

func (vm *VM) RegisterScriptLib() {
	t := runtime.NewTable()
	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSetString(name, runtime.FunctionValue(&runtime.GoFunction{Name: "script." + name, Fn: fn}))
	}
	set("env", func(args []runtime.Value) ([]runtime.Value, error) {
		seed := runtime.NewTable()
		if len(args) >= 1 && !args[0].IsNil() {
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'script.env' (table expected)")
			}
			seed = args[0].Table()
		}
		return []runtime.Value{runtime.TableValue(vmScriptEnvOptions(seed, false))}, nil
	})
	set("sandbox", func(args []runtime.Value) ([]runtime.Value, error) {
		seed := runtime.NewTable()
		if len(args) >= 1 && !args[0].IsNil() {
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'script.sandbox' (table expected)")
			}
			seed = args[0].Table()
		}
		return []runtime.Value{runtime.TableValue(vmScriptEnvOptions(seed, true))}, nil
	})
	set("compile", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.compile' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		return vm.compileScriptChunk(args[0].Str(), opt, "<script.compile>")
	})
	set("eval", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.eval' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := vm.compileScriptChunk(args[0].Str(), opt, "<script.eval>")
		if err != nil {
			return nil, err
		}
		return vm.callValue(fn[0], nil)
	})
	set("loadFile", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.loadFile' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		return vm.loadScriptFile(args[0].Str(), opt)
	})
	set("runFile", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.runFile' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := vm.loadScriptFile(args[0].Str(), opt)
		if err != nil {
			return nil, err
		}
		return vm.callValue(fn[0], nil)
	})
	set("dir", func(args []runtime.Value) ([]runtime.Value, error) {
		return []runtime.Value{runtime.StringValue(vm.scriptDir)}, nil
	})
	set("setDir", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'script.setDir' (string expected)")
		}
		old := vm.scriptDir
		vm.scriptDir = args[0].Str()
		return []runtime.Value{runtime.StringValue(old)}, nil
	})
	val := runtime.TableValue(t)
	vm.SetGlobal("script", val)
	vm.setPackageLoaded("script", val)
}

func (vm *VM) RegisterLoaderLib() {
	vm.SetGlobal("load", runtime.FunctionValue(&runtime.GoFunction{Name: "load", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'load' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := vm.compileScriptChunk(args[0].Str(), opt, "<load>")
		if err != nil {
			return []runtime.Value{runtime.NilValue(), runtime.StringValue(err.Error())}, nil
		}
		return fn, nil
	}}))
	vm.SetGlobal("loadstring", vm.GetGlobal("load"))
	vm.SetGlobal("loadfile", runtime.FunctionValue(&runtime.GoFunction{Name: "loadfile", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'loadfile' (string expected)")
		}
		opt := runtime.NilValue()
		if len(args) >= 2 {
			opt = args[1]
		}
		fn, err := vm.loadScriptFile(args[0].Str(), opt)
		if err != nil {
			return []runtime.Value{runtime.NilValue(), runtime.StringValue(err.Error())}, nil
		}
		return fn, nil
	}}))
	vm.SetGlobal("dofile", runtime.FunctionValue(&runtime.GoFunction{Name: "dofile", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'dofile' (string expected)")
		}
		fn, err := vm.loadScriptFile(args[0].Str(), runtime.NilValue())
		if err != nil {
			return nil, err
		}
		return vm.callValue(fn[0], nil)
	}}))
	vm.SetGlobal("require", runtime.FunctionValue(&runtime.GoFunction{Name: "require", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'require' (string expected)")
		}
		name := args[0].Str()
		if loaded := vm.packageLoaded(name); !loaded.IsNil() {
			return []runtime.Value{loaded}, nil
		}
		if module := vm.GetGlobal(name); module.IsTable() || module.IsFunction() {
			vm.setPackageLoaded(name, module)
			return []runtime.Value{module}, nil
		}
		filename := vm.resolveScriptPath(strings.ReplaceAll(name, ".", "/") + ".gs")
		if _, err := os.Stat(filename); err != nil {
			return nil, fmt.Errorf("module '%s' not found", name)
		}
		fn, err := vm.loadScriptFile(filename, runtime.NilValue())
		if err != nil {
			return nil, err
		}
		results, err := vm.callValue(fn[0], nil)
		if err != nil {
			return nil, err
		}
		module := runtime.BoolValue(true)
		if len(results) > 0 {
			module = results[0]
		}
		vm.setPackageLoaded(name, module)
		return []runtime.Value{module}, nil
	}}))
}

func (vm *VM) packageLoaded(name string) runtime.Value {
	pkg := vm.GetGlobal("package")
	if !pkg.IsTable() {
		return runtime.NilValue()
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		return runtime.NilValue()
	}
	return loaded.Table().RawGetString(name)
}

type vmScriptConfig struct {
	sourceName string
	scriptDir  string
	env        *runtime.Table
	sandbox    bool
}

func (vm *VM) compileScriptChunk(src string, opt runtime.Value, defaultSource string) ([]runtime.Value, error) {
	cfg, err := vm.scriptConfigFromValue(opt, defaultSource)
	if err != nil {
		return nil, err
	}
	proto, err := compileScriptSource(src, cfg.sourceName)
	if err != nil {
		return nil, err
	}
	if cfg.sourceName != "" {
		setProtoSource(proto, cfg.sourceName)
	}
	if cfg.scriptDir != "" && cfg.env == nil {
		// Preserve the chunk directory for later relative loads.
		cl := NewClosure(proto)
		return []runtime.Value{runtime.FunctionValue(&runtime.GoFunction{Name: "script.chunk", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			prev := vm.scriptDir
			vm.scriptDir = cfg.scriptDir
			defer func() { vm.scriptDir = prev }()
			return vm.callValue(runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl), args)
		}})}, nil
	}
	if cfg.env != nil {
		return []runtime.Value{runtime.FunctionValue(&runtime.GoFunction{Name: "script.chunk", Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			return vm.executeScriptInChild(proto, cfg, args)
		}})}, nil
	}
	cl := NewClosure(proto)
	return []runtime.Value{runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl)}, nil
}

func (vm *VM) loadScriptFile(filename string, opt runtime.Value) ([]runtime.Value, error) {
	cfg, err := vm.scriptConfigFromValue(opt, filename)
	if err != nil {
		return nil, err
	}
	resolveDir := cfg.scriptDir
	if resolveDir == "" {
		resolveDir = vm.scriptDir
	}
	resolved := vm.resolveScriptPathWithDir(filename, resolveDir)
	src, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %s", resolved, err)
	}
	if cfg.sourceName == "" {
		cfg.sourceName = resolved
	}
	if cfg.scriptDir == "" {
		if abs, err := filepath.Abs(resolved); err == nil {
			cfg.scriptDir = filepath.Dir(abs)
		}
	}
	return vm.compileScriptChunk(string(src), vmScriptConfigValue(cfg), cfg.sourceName)
}

func compileScriptSource(src string, sourceName string) (*FuncProto, error) {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil, wrapScriptCompileSourceError(err, sourceName)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, wrapScriptCompileSourceError(err, sourceName)
	}
	proto, err := Compile(prog)
	if err != nil {
		return nil, wrapScriptCompileSourceError(err, sourceName)
	}
	setProtoSource(proto, sourceName)
	return proto, nil
}

func wrapScriptCompileSourceError(err error, sourceName string) error {
	if err == nil || sourceName == "" || strings.Contains(err.Error(), sourceName) {
		return err
	}
	return fmt.Errorf("%s: %w", sourceName, err)
}

func setProtoSource(proto *FuncProto, sourceName string) {
	if proto == nil {
		return
	}
	if proto.Source == "" {
		proto.Source = sourceName
	}
	for _, child := range proto.Protos {
		setProtoSource(child, sourceName)
	}
}

func (vm *VM) scriptConfigFromValue(opt runtime.Value, defaultSource string) (vmScriptConfig, error) {
	cfg := vmScriptConfig{sourceName: defaultSource}
	if vmScriptOptionIsNil(opt) {
		return cfg, nil
	}
	if opt.IsString() {
		cfg.sourceName = opt.Str()
		return cfg, nil
	}
	if !opt.IsTable() {
		return cfg, fmt.Errorf("script environment options must be a table, string, or nil")
	}
	tbl := opt.Table()
	if v := tbl.RawGetString("sourceName"); !v.IsNil() {
		if !v.IsString() {
			return cfg, fmt.Errorf("script environment option 'sourceName' must be a string")
		}
		cfg.sourceName = v.Str()
	}
	if v := tbl.RawGetString("source"); !v.IsNil() {
		if !v.IsString() {
			return cfg, fmt.Errorf("script environment option 'source' must be a string")
		}
		cfg.sourceName = v.Str()
	}
	if v := tbl.RawGetString("scriptDir"); !v.IsNil() {
		if !v.IsString() {
			return cfg, fmt.Errorf("script environment option 'scriptDir' must be a string")
		}
		cfg.scriptDir = v.Str()
	}
	envVal := tbl.RawGetString("env")
	if envVal.IsNil() {
		if !vmScriptOptionsTableHasConfigKeys(tbl) {
			envVal = opt
		}
	} else if !envVal.IsTable() {
		return cfg, fmt.Errorf("script environment option 'env' must be a table")
	}
	cfg.sandbox = tbl.RawGetString("sandbox").Truthy()
	if envVal.IsTable() {
		cfg.env = envVal.Table()
	}
	return cfg, nil
}

func vmScriptOptionIsNil(opt runtime.Value) bool {
	return opt.IsNil() || uint64(opt) == 0
}

func vmScriptConfigValue(cfg vmScriptConfig) runtime.Value {
	t := runtime.NewTable()
	if cfg.sourceName != "" {
		t.RawSetString("sourceName", runtime.StringValue(cfg.sourceName))
	}
	if cfg.scriptDir != "" {
		t.RawSetString("scriptDir", runtime.StringValue(cfg.scriptDir))
	}
	if cfg.env != nil {
		t.RawSetString("env", runtime.TableValue(cfg.env))
	}
	if cfg.sandbox {
		t.RawSetString("sandbox", runtime.BoolValue(true))
	}
	return runtime.TableValue(t)
}

func vmScriptEnvOptions(seed *runtime.Table, sandbox bool) *runtime.Table {
	opts := runtime.NewTable()
	opts.RawSetString("env", runtime.TableValue(seed))
	opts.RawSetString("sandbox", runtime.BoolValue(sandbox))
	return opts
}

func vmScriptOptionsTableHasConfigKeys(tbl *runtime.Table) bool {
	for _, key := range []string{"env", "sandbox", "sourceName", "source", "scriptDir"} {
		if !tbl.RawGetString(key).IsNil() {
			return true
		}
	}
	return false
}

func (vm *VM) executeScriptInChild(proto *FuncProto, cfg vmScriptConfig, args []runtime.Value) ([]runtime.Value, error) {
	base := make(map[string]runtime.Value)
	original := make(map[string]runtime.Value)
	originalSet := make(map[string]bool)
	if !cfg.sandbox {
		for name, val := range vm.globals {
			base[name] = val
			original[name] = val
			originalSet[name] = true
		}
	}
	envKeys := make(map[string]bool)
	if cfg.env != nil {
		k, v, ok := cfg.env.Next(runtime.NilValue())
		for ok {
			if k.IsString() {
				name := k.Str()
				envKeys[name] = true
				base[name] = v
				original[name] = v
				originalSet[name] = true
			}
			k, v, ok = cfg.env.Next(k)
		}
	}
	child := New(base)
	child.SetStringMeta(vm.stringMeta)
	child.scriptDir = cfg.scriptDir
	if child.scriptDir == "" {
		child.scriptDir = vm.scriptDir
	}
	cl := NewClosure(proto)
	var results []runtime.Value
	var err error
	if len(args) == 0 {
		results, err = child.Execute(proto)
	} else {
		results, err = child.callValue(runtime.VMClosureFunctionValue(unsafe.Pointer(cl), cl), args)
	}
	if cfg.env != nil {
		for name, idx := range child.globalIndex {
			if idx < 0 || idx >= len(child.globalArray) {
				continue
			}
			val := child.globalArray[idx]
			if cfg.sandbox || envKeys[name] || !originalSet[name] || original[name] != val {
				cfg.env.RawSetString(name, val)
			}
		}
	}
	return results, err
}

func (vm *VM) resolveScriptPath(filename string) string {
	return vm.resolveScriptPathWithDir(filename, vm.scriptDir)
}

func (vm *VM) resolveScriptPathWithDir(filename string, dir string) string {
	if filename == "" || filepath.IsAbs(filename) || dir == "" {
		return filename
	}
	candidate := filepath.Join(dir, filename)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return filename
}

func (vm *VM) SetScriptDir(dir string) {
	vm.scriptDir = dir
}

func (vm *VM) ScriptDir() string {
	return vm.scriptDir
}

// Execute runs a top-level function prototype.
func (vm *VM) Execute(proto *FuncProto) ([]runtime.Value, error) {
	cl := &Closure{Proto: proto}
	vm.frameCount = 0
	vm.top = 0
	return vm.call(cl, nil, 0, 0)
}

// CallValue calls a function value with the given arguments (exported for gscript wrapper).
func (vm *VM) CallValue(fn runtime.Value, args []runtime.Value) ([]runtime.Value, error) {
	return vm.callValue(fn, args)
}

func (vm *VM) executeMethodJIT(compiled interface{}, regs []runtime.Value, base int, proto *FuncProto) ([]runtime.Value, error) {
	if exec, ok := vm.methodJIT.(methodJITEngineWithResultBuffer); ok {
		return exec.ExecuteWithResultBuffer(compiled, regs, base, proto, vm.retBuf[:0])
	}
	return vm.methodJIT.Execute(compiled, regs, base, proto)
}

func (vm *VM) markGlobalReadOnly(name string) {
	if vm.readOnlyGlobals == nil {
		vm.readOnlyGlobals = make(map[string]bool)
	}
	vm.readOnlyGlobals[name] = true
}

func (vm *VM) isGlobalReadOnly(name string) bool {
	return vm.readOnlyGlobals != nil && vm.readOnlyGlobals[name]
}

func (vm *VM) drainFrameDefers(frame *CallFrame) error {
	if frame == nil || len(frame.defers) == 0 {
		return nil
	}
	calls := frame.defers
	frame.defers = nil
	var firstErr error
	for i := len(calls) - 1; i >= 0; i-- {
		if _, err := vm.callValue(calls[i].fn, calls[i].args); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (vm *VM) drainActiveDefers(fromFrame int) error {
	var firstErr error
	for i := vm.frameCount - 1; i >= fromFrame && i >= 0; i-- {
		if err := vm.drainFrameDefers(&vm.frames[i]); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// call pushes a new call frame and executes.
func (vm *VM) call(cl *Closure, args []runtime.Value, base int, numResults int) ([]runtime.Value, error) {
	// GC safe point at function entry: all caller's register writes are complete.
	runtime.CheckGC()

	proto := cl.Proto

	// Ensure register space
	needed := base + proto.MaxStack + 1
	if needed > len(vm.regs) {
		newRegs := runtime.MakeNilSlice(needed * 2)
		copy(newRegs, vm.regs)
		vm.regs = newRegs
	}

	// Place args in registers
	nParams := proto.NumParams
	for i := 0; i < nParams && i < len(args); i++ {
		vm.regs[base+i] = args[i]
	}
	for i := len(args); i < nParams; i++ {
		vm.regs[base+i] = runtime.NilValue()
	}

	// Push frame
	if !vm.ensureFrameSlot() {
		return nil, fmt.Errorf("stack overflow (max call depth %d)", maxCallDepth)
	}
	frame := &vm.frames[vm.frameCount]
	frame.closure = cl
	frame.pc = 0
	frame.base = base
	frame.numResults = numResults
	if proto.IsVarArg && len(args) > nParams {
		setFrameVarargs(frame, args[nParams:])
	} else {
		frame.varargs = nil
	}
	frame.callSitePC = -1
	frame.defers = nil
	vm.frameCount++
	if err := vm.emitDebugHook("call", "script", debugProtoName(proto), runtime.NilValue()); err != nil {
		vm.frameCount--
		return nil, err
	}

	// Method JIT: check for compiled function.
	if vm.methodJIT != nil && proto.MethodJITTier1Callable() && !proto.JITDisabled {
		proto.CallCount++
		if proto.CallCount <= 64 {
			proto.ObserveArgShapes(args)
			proto.ObserveArgArrayElementShapes(args)
		}
		if compiled := vm.methodJIT.TryCompile(proto); compiled != nil {
			results, err := vm.executeMethodJIT(compiled, vm.regs, base, proto)
			if err == errCoroutineYield {
				return results, err
			}
			if err == nil {
				vm.closeUpvalues(base)
				if err := vm.emitDebugHook("return", "script", debugProtoName(proto), runtime.NilValue()); err != nil {
					vm.frameCount--
					return nil, err
				}
				vm.frameCount--
				return results, nil
			}
			// Method JIT execution failed; fall through to interpreter.
		}
	}

	result, err := vm.run()
	if err == errCoroutineYield {
		return result, err
	}
	if vm.coroutineYielded {
		return result, nil
	}
	if err != nil {
		_ = vm.emitDebugHook("error", "script", debugProtoName(proto), runtime.StringValue(err.Error()))
	} else if err := vm.emitDebugHook("return", "script", debugProtoName(proto), runtime.NilValue()); err != nil {
		vm.frameCount--
		return nil, err
	}
	vm.frameCount--
	return result, err
}

// wrapLineErr wraps an error with source location info from the current frame.
func wrapLineErr(frame *CallFrame, err error) error {
	if err == nil {
		return nil
	}
	pc := frame.pc - 1
	line := 0
	if pc >= 0 && pc < len(frame.closure.Proto.LineInfo) {
		line = frame.closure.Proto.LineInfo[pc]
	}
	name := frame.closure.Proto.Source
	if name == "" {
		name = frame.closure.Proto.Name
	}
	if line > 0 {
		return fmt.Errorf("%s:%d: %w", name, line, err)
	}
	return err
}

// run is the main execution loop. Handles inline call/return to avoid
// Go stack growth for GScript function calls.
func (vm *VM) run() (retVals []runtime.Value, retErr error) {
	initialFC := vm.frameCount
	coroutineChild := vm.currentCoroutine != nil
	if coroutineChild && initialFC > 1 {
		initialFC = 1
	}

	// On error, reset frame count to clean up any inline sub-frames.
	if !coroutineChild {
		defer func() {
			if retErr != nil && retErr != errCoroutineYield {
				if deferErr := vm.drainActiveDefers(initialFC - 1); deferErr != nil && retErr == nil {
					retErr = deferErr
				}
				vm.frameCount = initialFC
			}
		}()
	}

	frame := &vm.frames[vm.frameCount-1]
	code := frame.closure.Proto.Code
	constants := frame.closure.Proto.Constants
	base := frame.base

	for {
		if frame.pc >= len(code) {
			// End of function - implicit return nil
			if err := vm.drainFrameDefers(frame); err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.closeUpvalues(base)
			if vm.frameCount <= initialFC {
				return nil, nil
			}
			// Inline return with no values
			childCallSitePC := frame.callSitePC
			vm.frameCount--
			rc := frame.resultCount
			rb := frame.resultBase
			if rc != 0 {
				nr := rc - 1
				for i := 0; i < nr; i++ {
					vm.regs[rb+i] = runtime.NilValue()
				}
			} else {
				vm.top = rb
			}
			if vm.frameCount > 0 {
				observeCallResultFixed(vm.frames[vm.frameCount-1].closure.Proto, childCallSitePC, vm.regs, rb, rc)
			}
			if err := vm.emitDebugHook("return", "script", debugProtoName(frame.closure.Proto), runtime.NilValue()); err != nil {
				return nil, err
			}
			frame = &vm.frames[vm.frameCount-1]
			code = frame.closure.Proto.Code
			constants = frame.closure.Proto.Constants
			base = frame.base
			continue
		}
		inst := code[frame.pc]
		frame.pc++

		op := DecodeOp(inst)

		switch op {
		case OP_LOADNIL:
			a := DecodeA(inst)
			b := DecodeB(inst)
			for i := a; i <= a+b; i++ {
				vm.regs[base+i] = runtime.NilValue()
			}

		case OP_LOADBOOL:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)
			vm.regs[base+a] = runtime.BoolValue(b != 0)
			if c != 0 {
				frame.pc++
			}

		case OP_LOADINT:
			a := DecodeA(inst)
			sbx := DecodesBx(inst)
			vm.regs[base+a] = runtime.IntValue(int64(sbx))

		case OP_LOADK:
			a := DecodeA(inst)
			bx := DecodeBx(inst)
			vm.regs[base+a] = constants[bx]

		case OP_MOVE:
			a := DecodeA(inst)
			b := DecodeB(inst)
			vm.regs[base+a] = vm.regs[base+b]

		case OP_GETGLOBAL:
			a := DecodeA(inst)
			bx := DecodeBx(inst)
			// Lazy-init GlobalCache
			proto := frame.closure.Proto
			if proto.GlobalCache == nil {
				proto.GlobalCache = make([]globalCacheEntry, len(proto.Constants))
				for i := range proto.GlobalCache {
					proto.GlobalCache[i].index = -1
				}
			}
			cache := &proto.GlobalCache[bx]
			if cache.index >= 0 && cache.version == vm.globalVer {
				if int(cache.index) == vm.globalOverrideFast {
					vm.regs[base+a] = vm.globalOverrideVal
					break
				}
				if vm.globalOverrideIdx != nil {
					if v, ok := vm.globalOverrideIdx[int(cache.index)]; ok {
						vm.regs[base+a] = v
						break
					}
				}
				if vm.noGlobalLock {
					// Single-threaded: no lock needed
					vm.regs[base+a] = vm.globalArray[cache.index]
				} else {
					// Multi-threaded: use indexed array but lock for memory barrier
					vm.globalsMu.RLock()
					vm.regs[base+a] = vm.globalArray[cache.index]
					vm.globalsMu.RUnlock()
				}
			} else if vm.noGlobalLock {
				// Single-threaded cache miss: resolve + cache without lock
				name := constants[bx].Str()
				idx := vm.resolveGlobalIndex(name)
				cache.index = int32(idx)
				cache.version = vm.globalVer
				if vm.globalOverrides != nil {
					if v, ok := vm.globalOverrides[name]; ok {
						vm.globalOverrideIdx[idx] = v
						vm.regs[base+a] = v
						break
					}
				}
				vm.regs[base+a] = vm.globalArray[idx]
			} else {
				// Multi-threaded cache miss: locked map fallback
				name := constants[bx].Str()
				if vm.globalOverrides != nil {
					if ov, ok := vm.globalOverrides[name]; ok {
						vm.globalsMu.RLock()
						idx, hasIdx := vm.globalIndex[name]
						ver := vm.globalVer
						vm.globalsMu.RUnlock()
						if hasIdx {
							cache.index = int32(idx)
							cache.version = ver
							vm.globalOverrideIdx[idx] = ov
						}
						vm.regs[base+a] = ov
						break
					}
				}
				vm.globalsMu.RLock()
				v := vm.globals[name]
				vm.globalsMu.RUnlock()
				vm.regs[base+a] = v
			}

		case OP_SETGLOBAL:
			a := DecodeA(inst)
			bx := DecodeBx(inst)
			val := vm.regs[base+a]
			name := constants[bx].Str()
			if vm.isGlobalReadOnly(name) {
				return nil, wrapLineErr(frame, fmt.Errorf("cannot assign to readonly variable %q", name))
			}
			if vm.noGlobalLock {
				// Single-threaded fast path
				proto := frame.closure.Proto
				if proto.GlobalCache == nil {
					proto.GlobalCache = make([]globalCacheEntry, len(proto.Constants))
					for i := range proto.GlobalCache {
						proto.GlobalCache[i].index = -1
					}
				}
				cache := &proto.GlobalCache[bx]
				if cache.index >= 0 && cache.version == vm.globalVer {
					vm.globalArray[cache.index] = val
				} else {
					idx := vm.resolveGlobalIndex(name)
					cache.index = int32(idx)
					cache.version = vm.globalVer
					vm.globalArray[idx] = val
				}
				vm.globals[name] = val
				vm.globalValueVer++
			} else {
				// Multi-threaded: locked access, update both map and array
				vm.globalsMu.Lock()
				vm.globals[name] = val
				if idx, ok := vm.globalIndex[name]; ok {
					vm.globalArray[idx] = val
					vm.globalValueVer++
				}
				vm.globalsMu.Unlock()
			}

		case OP_SETGLOBALRO:
			a := DecodeA(inst)
			bx := DecodeBx(inst)
			val := vm.regs[base+a]
			name := constants[bx].Str()
			if vm.isGlobalReadOnly(name) {
				return nil, wrapLineErr(frame, fmt.Errorf("cannot redeclare readonly variable %q", name))
			}
			if vm.noGlobalLock {
				proto := frame.closure.Proto
				if proto.GlobalCache == nil {
					proto.GlobalCache = make([]globalCacheEntry, len(proto.Constants))
					for i := range proto.GlobalCache {
						proto.GlobalCache[i].index = -1
					}
				}
				cache := &proto.GlobalCache[bx]
				idx := vm.resolveGlobalIndex(name)
				cache.index = int32(idx)
				cache.version = vm.globalVer
				vm.globalArray[idx] = val
				vm.globals[name] = val
				vm.globalValueVer++
			} else {
				vm.globalsMu.Lock()
				vm.globals[name] = val
				if idx, ok := vm.globalIndex[name]; ok {
					vm.globalArray[idx] = val
				} else {
					idx = len(vm.globalArray)
					vm.globalIndex[name] = idx
					vm.globalArray = append(vm.globalArray, val)
					vm.globalVer++
				}
				vm.globalValueVer++
				vm.globalsMu.Unlock()
			}
			vm.markGlobalReadOnly(name)

		case OP_GETUPVAL:
			a := DecodeA(inst)
			b := DecodeB(inst)
			vm.regs[base+a] = frame.closure.Upvalues[b].Get()

		case OP_SETUPVAL:
			a := DecodeA(inst)
			b := DecodeB(inst)
			if b >= 0 && b < len(frame.closure.Proto.Upvalues) && frame.closure.Proto.Upvalues[b].ReadOnly {
				return nil, wrapLineErr(frame, fmt.Errorf("cannot assign to readonly variable %q", frame.closure.Proto.Upvalues[b].Name))
			}
			frame.closure.Upvalues[b].Set(vm.regs[base+a])

		case OP_CHECKCONST:
			a := DecodeA(inst)
			bx := DecodeBx(inst)
			if name, ok := frame.closure.Proto.ReadOnlyLocals[a]; ok {
				if bx >= 0 && bx < len(constants) && constants[bx].IsString() {
					name = constants[bx].Str()
				}
				return nil, wrapLineErr(frame, fmt.Errorf("cannot assign to readonly variable %q", name))
			}

		case OP_NEWTABLE:
			a := DecodeA(inst)
			b := DecodeB(inst) // array hint
			c := DecodeC(inst) // hash hint
			vm.regs[base+a] = runtime.FreshTableValue(runtime.NewTableSized(b, c))

		case OP_NEWOBJECT2:
			a := DecodeA(inst)
			b := DecodeB(inst) // table ctor index
			c := DecodeC(inst) // first value register
			if b >= 0 && b < len(frame.closure.Proto.TableCtors2) {
				ctor := &frame.closure.Proto.TableCtors2[b].Runtime
				val1 := vm.regs[base+c]
				val2 := vm.regs[base+c+1]
				vm.regs[base+a] = runtime.FreshTableValue(runtime.NewTableFromCtor2(ctor, val1, val2))
			} else {
				vm.regs[base+a] = runtime.FreshTableValue(runtime.NewTableSized(0, 2))
			}

		case OP_NEWOBJECTN:
			a := DecodeA(inst)
			b := DecodeB(inst) // table ctor index
			c := DecodeC(inst) // first value register
			if b >= 0 && b < len(frame.closure.Proto.TableCtorsN) {
				ctor := &frame.closure.Proto.TableCtorsN[b].Runtime
				n := len(ctor.Keys)
				start := base + c
				if start >= 0 && start+n <= len(vm.regs) {
					if vm.currentCoroutine != nil {
						if vm.currentCoroutine.stackYieldEnabled {
							co := vm.currentCoroutine
							if co.pooledFixedRecord == nil {
								if runtime.DefaultHeap != nil {
									co.pooledFixedRecord = runtime.DefaultHeap.AllocFixedRecord()
								} else {
									co.pooledFixedRecord = &runtime.FixedRecord{}
								}
							}
							if v, ok := runtime.FillFixedRecordKnownCtor(co.pooledFixedRecord, ctor, vm.regs[start:start+n]); ok {
								vm.regs[base+a] = v
								break
							}
						}
						if n == 5 {
							if v, ok := runtime.NewFixedRecordValue5KnownCtor(ctor, vm.regs[start], vm.regs[start+1], vm.regs[start+2], vm.regs[start+3], vm.regs[start+4]); ok {
								vm.regs[base+a] = v
								break
							}
						} else {
							if v, ok := runtime.NewFixedRecordValue(ctor, vm.regs[start:start+n]); ok {
								vm.regs[base+a] = v
								break
							}
						}
					}
					vm.regs[base+a] = runtime.FreshTableValue(runtime.NewTableFromCtorN(ctor, vm.regs[start:start+n]))
				} else {
					vm.regs[base+a] = runtime.FreshTableValue(runtime.NewTableSized(0, n))
				}
			} else {
				vm.regs[base+a] = runtime.FreshTableValue(runtime.NewEmptyTable())
			}

		case OP_GETTABLE:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cidx := DecodeC(inst)
			tableVal := vm.regs[base+b]
			var key runtime.Value
			if cidx >= RKBit {
				key = constants[cidx-RKBit]
			} else {
				key = vm.regs[base+cidx]
			}
			if key.IsString() {
				if v, ok := tableVal.FixedRecordRawGetString(key.Str()); ok {
					vm.regs[base+a] = v
					if frame.closure.Proto.Feedback != nil {
						fb := &frame.closure.Proto.Feedback[frame.pc-1]
						fb.Left.Observe(tableVal.Type())
						fb.Right.Observe(key.Type())
						fb.Result.Observe(v.Type())
					}
					break
				}
			}
			// Fast path: plain table (no metatable)
			if tableVal.IsTable() {
				if tbl := tableVal.Table(); tbl.GetMetatable() == nil {
					pc := frame.pc - 1
					if key.IsString() {
						proto := frame.closure.Proto
						if proto.TableStringKeyCache == nil {
							proto.TableStringKeyCache = make([]runtime.TableStringKeyCacheEntry, len(proto.Code)*runtime.TableStringKeyCacheWays)
						}
						vm.regs[base+a] = tbl.RawGetStringDynamicCached(
							key.Str(),
							runtime.TableStringKeyCacheSlot(proto.TableStringKeyCache, pc),
						)
					} else {
						vm.regs[base+a] = tbl.RawGet(key)
					}
					if frame.closure.Proto.Feedback != nil {
						fb := &frame.closure.Proto.Feedback[pc]
						fb.Left.Observe(tableVal.Type())
						fb.Right.Observe(key.Type())
						fb.Result.Observe(vm.regs[base+a].Type())
						if frame.closure.Proto.TableKeyFeedback != nil {
							frame.closure.Proto.TableKeyFeedback[pc].ObserveTableAccess(tbl, key, vm.regs[base+a], TableAccessKindGet, -1, -1)
						}
					}
					break
				}
			}
			val, err := vm.tableGet(tableVal, key)
			if err != nil {
				return nil, err
			}
			vm.regs[base+a] = val
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(tableVal.Type())
				fb.Right.Observe(key.Type())
				fb.Result.Observe(val.Type())
				if frame.closure.Proto.TableKeyFeedback != nil {
					tkf := &frame.closure.Proto.TableKeyFeedback[frame.pc-1]
					if tableVal.IsTable() {
						tkf.ObserveTableAccess(tableVal.Table(), key, val, TableAccessKindGet, -1, -1)
					}
				}
			}

		case OP_SETTABLE:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			tableVal := vm.regs[base+a]
			var key, val runtime.Value
			if bidx >= RKBit {
				key = constants[bidx-RKBit]
			} else {
				key = vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				val = constants[cidx-RKBit]
			} else {
				val = vm.regs[base+cidx]
			}
			// Fast path: plain table
			if tableVal.IsTable() {
				if tbl := tableVal.Table(); tbl.GetMetatable() == nil {
					pc := frame.pc - 1
					beforeLen, beforeFieldIdx := -1, -1
					if key.IsInt() {
						beforeLen = tbl.Len()
					} else if key.IsString() {
						beforeFieldIdx = tbl.FieldIndex(key.Str())
					}
					if key.IsString() {
						proto := frame.closure.Proto
						if proto.TableStringKeyCache == nil {
							proto.TableStringKeyCache = make([]runtime.TableStringKeyCacheEntry, len(proto.Code)*runtime.TableStringKeyCacheWays)
						}
						tbl.RawSetStringDynamicCached(
							key.Str(),
							val,
							runtime.TableStringKeyCacheSlot(proto.TableStringKeyCache, pc),
						)
					} else {
						tbl.RawSet(key, val)
					}
					if frame.closure.Proto.Feedback != nil {
						fb := &frame.closure.Proto.Feedback[pc]
						fb.Left.Observe(tableVal.Type())
						fb.Right.Observe(key.Type())
						fb.Result.Observe(val.Type())
						if frame.closure.Proto.TableKeyFeedback != nil {
							frame.closure.Proto.TableKeyFeedback[pc].ObserveTableAccess(tbl, key, val, TableAccessKindSet, beforeLen, beforeFieldIdx)
						}
					}
					break
				}
			}
			if err := vm.tableSet(tableVal, key, val); err != nil {
				return nil, err
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(tableVal.Type())
				fb.Right.Observe(key.Type())
				fb.Result.Observe(val.Type())
				if frame.closure.Proto.TableKeyFeedback != nil {
					if tableVal.IsTable() {
						frame.closure.Proto.TableKeyFeedback[frame.pc-1].ObserveTableAccess(tableVal.Table(), key, val, TableAccessKindSet, -1, -1)
					}
				}
			}

		case OP_GETFIELD:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)
			tableVal := vm.regs[base+b]
			if v, ok := tableVal.FixedRecordRawGetString(constants[c].Str()); ok {
				vm.regs[base+a] = v
				if frame.closure.Proto.Feedback != nil {
					fb := &frame.closure.Proto.Feedback[frame.pc-1]
					fb.Result.Observe(v.Type())
				}
				break
			}
			// Fast path: plain table → direct string field lookup with inline cache
			if tableVal.IsTable() {
				if tbl := tableVal.Table(); tbl.GetMetatable() == nil {
					proto := frame.closure.Proto
					if proto.FieldCache == nil {
						proto.FieldCache = make([]runtime.FieldCacheEntry, len(proto.Code))
					}
					pc := frame.pc - 1
					vm.regs[base+a] = tbl.RawGetStringCached(constants[c].Str(), &proto.FieldCache[pc])
					if proto.Feedback != nil {
						fb := &proto.Feedback[pc]
						fb.Result.Observe(vm.regs[base+a].Type())
					}
					if proto.FieldAccessFeedback != nil {
						proto.FieldAccessFeedback[pc].ObserveFieldCache(proto.FieldCache[pc], vm.regs[base+a], 1)
					}
					break
				}
			}
			key := constants[c]
			val, err := vm.tableGet(tableVal, key)
			if err != nil {
				return nil, err
			}
			vm.regs[base+a] = val
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Result.Observe(val.Type())
			}

		case OP_SETFIELD:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cidx := DecodeC(inst)
			tableVal := vm.regs[base+a]
			var val runtime.Value
			if cidx >= RKBit {
				val = constants[cidx-RKBit]
			} else {
				val = vm.regs[base+cidx]
			}
			// Fast path: plain table → direct string field set with inline cache
			if tableVal.IsTable() {
				if tbl := tableVal.Table(); tbl.GetMetatable() == nil {
					proto := frame.closure.Proto
					if proto.FieldCache == nil {
						proto.FieldCache = make([]runtime.FieldCacheEntry, len(proto.Code))
					}
					pc := frame.pc - 1
					tbl.RawSetStringCached(constants[b].Str(), val, &proto.FieldCache[pc])
					if proto.Feedback != nil {
						fb := &proto.Feedback[pc]
						fb.Result.Observe(val.Type())
					}
					if proto.FieldAccessFeedback != nil {
						proto.FieldAccessFeedback[pc].ObserveFieldCache(proto.FieldCache[pc], val, 2)
					}
					break
				}
			}
			key := constants[b]
			if err := vm.tableSet(tableVal, key, val); err != nil {
				return nil, err
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Result.Observe(val.Type())
			}

		case OP_SETLIST:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)
			t := vm.regs[base+a].Table()
			if t == nil {
				return nil, fmt.Errorf("SETLIST on non-table")
			}
			offset := (c - 1) * 50
			if b == 0 {
				valueStart := base + a + 1
				count := vm.top - valueStart
				if count < 0 {
					count = 0
				}
				for i := 1; i <= count; i++ {
					t.RawSetInt(int64(offset+i), vm.regs[valueStart+i-1])
				}
				break
			}
			for i := 1; i <= b; i++ {
				t.RawSetInt(int64(offset+i), vm.regs[base+a+i])
			}

		case OP_SETLISTDYN:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)
			t := vm.regs[base+a].Table()
			if t == nil {
				return nil, fmt.Errorf("SETLISTDYN on non-table")
			}
			idxVal := vm.regs[base+b]
			if !idxVal.IsInt() {
				return nil, fmt.Errorf("SETLISTDYN index is %s", idxVal.TypeName())
			}
			start := idxVal.Int()
			valueStart := base + c
			count := vm.top - valueStart
			if count < 0 {
				count = 0
			}
			for i := 0; i < count; i++ {
				t.RawSetInt(start+int64(i), vm.regs[valueStart+i])
			}
			vm.regs[base+b] = runtime.IntValue(start + int64(count))

		case OP_APPEND:
			a := DecodeA(inst)
			b := DecodeB(inst)
			t := vm.regs[base+a].Table()
			if t == nil {
				return nil, fmt.Errorf("APPEND on non-table")
			}
			t.Append(vm.regs[base+b])

		// ---- Arithmetic ----
		case OP_ADD:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bp, cp *runtime.Value
			if bidx >= RKBit {
				bp = &constants[bidx-RKBit]
			} else {
				bp = &vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cp = &constants[cidx-RKBit]
			} else {
				cp = &vm.regs[base+cidx]
			}
			dst := &vm.regs[base+a]
			if !runtime.AddNums(dst, bp, cp) {
				r, err := vm.arith(*bp, *cp, "__add", func(x, y float64) float64 { return x + y })
				if err != nil {
					return nil, wrapLineErr(frame, err)
				}
				*dst = r
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(bp.Type())
				fb.Right.Observe(cp.Type())
				fb.Result.Observe(dst.Type())
			}

		case OP_SUB:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bp, cp *runtime.Value
			if bidx >= RKBit {
				bp = &constants[bidx-RKBit]
			} else {
				bp = &vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cp = &constants[cidx-RKBit]
			} else {
				cp = &vm.regs[base+cidx]
			}
			dst := &vm.regs[base+a]
			if !runtime.SubNums(dst, bp, cp) {
				r, err := vm.arith(*bp, *cp, "__sub", func(x, y float64) float64 { return x - y })
				if err != nil {
					return nil, wrapLineErr(frame, err)
				}
				*dst = r
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(bp.Type())
				fb.Right.Observe(cp.Type())
				fb.Result.Observe(dst.Type())
			}

		case OP_MUL:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bp, cp *runtime.Value
			if bidx >= RKBit {
				bp = &constants[bidx-RKBit]
			} else {
				bp = &vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cp = &constants[cidx-RKBit]
			} else {
				cp = &vm.regs[base+cidx]
			}
			dst := &vm.regs[base+a]
			if !runtime.MulNums(dst, bp, cp) {
				r, err := vm.arith(*bp, *cp, "__mul", func(x, y float64) float64 { return x * y })
				if err != nil {
					return nil, wrapLineErr(frame, err)
				}
				*dst = r
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(bp.Type())
				fb.Right.Observe(cp.Type())
				fb.Result.Observe(dst.Type())
			}

		case OP_DIV:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bp, cp *runtime.Value
			if bidx >= RKBit {
				bp = &constants[bidx-RKBit]
			} else {
				bp = &vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cp = &constants[cidx-RKBit]
			} else {
				cp = &vm.regs[base+cidx]
			}
			dst := &vm.regs[base+a]
			if !runtime.DivNums(dst, bp, cp) {
				r, err := vm.arith(*bp, *cp, "__div", func(x, y float64) float64 { return x / y })
				if err != nil {
					return nil, wrapLineErr(frame, err)
				}
				*dst = r
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(bp.Type())
				fb.Right.Observe(cp.Type())
				fb.Result.Observe(dst.Type())
			}

		case OP_MOD:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bv, cv runtime.Value
			if bidx >= RKBit {
				bv = constants[bidx-RKBit]
			} else {
				bv = vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cv = constants[cidx-RKBit]
			} else {
				cv = vm.regs[base+cidx]
			}
			r, err := vm.arithMod(bv, cv)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.regs[base+a] = r
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(bv.Type())
				fb.Right.Observe(cv.Type())
				fb.Result.Observe(r.Type())
			}

		case OP_POW:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bv, cv runtime.Value
			if bidx >= RKBit {
				bv = constants[bidx-RKBit]
			} else {
				bv = vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cv = constants[cidx-RKBit]
			} else {
				cv = vm.regs[base+cidx]
			}
			r, err := vm.arith(bv, cv, "__pow", func(x, y float64) float64 { return math.Pow(x, y) })
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.regs[base+a] = r

		case OP_BAND, OP_BOR, OP_BXOR, OP_BANDN, OP_SHL, OP_SHR:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bv, cv runtime.Value
			if bidx >= RKBit {
				bv = constants[bidx-RKBit]
			} else {
				bv = vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cv = constants[cidx-RKBit]
			} else {
				cv = vm.regs[base+cidx]
			}
			r, err := bitwiseBinary(op, bv, cv)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.regs[base+a] = r

		case OP_UNM:
			a := DecodeA(inst)
			bv := vm.regs[base+DecodeB(inst)]
			r, err := vm.unaryMinus(bv)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.regs[base+a] = r

		case OP_BNOT:
			a := DecodeA(inst)
			bv := vm.regs[base+DecodeB(inst)]
			n, err := bitwiseInt(bv)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.regs[base+a] = runtime.IntValue(^n)

		case OP_NOT:
			a := DecodeA(inst)
			bv := vm.regs[base+DecodeB(inst)]
			vm.regs[base+a] = runtime.BoolValue(!bv.Truthy())

		case OP_LEN:
			a := DecodeA(inst)
			bv := vm.regs[base+DecodeB(inst)]
			r, err := vm.length(bv)
			if err != nil {
				return nil, err
			}
			vm.regs[base+a] = r
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Result.Observe(r.Type())
			}

		case OP_CONCAT:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)
			r, err := vm.ConcatValues(vm.regs[base+b : base+c+1])
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.regs[base+a] = r

		// ---- Comparison ----
		case OP_EQ:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bp, cp *runtime.Value
			if bidx >= RKBit {
				bp = &constants[bidx-RKBit]
			} else {
				bp = &vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cp = &constants[cidx-RKBit]
			} else {
				cp = &vm.regs[base+cidx]
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(bp.Type())
				fb.Right.Observe(cp.Type())
			}
			if bp.RawType() == runtime.TypeInt && cp.RawType() == runtime.TypeInt {
				if (bp.RawInt() == cp.RawInt()) != (a != 0) {
					frame.pc++
				}
			} else if eq, ok := runtime.EQStrings(bp, cp); ok {
				if eq != (a != 0) {
					frame.pc++
				}
			} else {
				eq, err := vm.valueEqual(*bp, *cp)
				if err != nil {
					return nil, err
				}
				if eq != (a != 0) {
					frame.pc++
				}
			}

		case OP_LT:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bp, cp *runtime.Value
			if bidx >= RKBit {
				bp = &constants[bidx-RKBit]
			} else {
				bp = &vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cp = &constants[cidx-RKBit]
			} else {
				cp = &vm.regs[base+cidx]
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(bp.Type())
				fb.Right.Observe(cp.Type())
			}
			if lt, ok := runtime.LTInts(bp, cp); ok {
				if lt != (a != 0) {
					frame.pc++
				}
			} else if lt, ok := runtime.LTStrings(bp, cp); ok {
				if lt != (a != 0) {
					frame.pc++
				}
			} else {
				lt, err := vm.valueLessThan(*bp, *cp)
				if err != nil {
					return nil, fmt.Errorf("%w at pc=%d B=%d C=%d bp=0x%x cp=0x%x", err, frame.pc-1, bidx, cidx, uint64(*bp), uint64(*cp))
				}
				if lt != (a != 0) {
					frame.pc++
				}
			}

		case OP_LE:
			a := DecodeA(inst)
			bidx := DecodeB(inst)
			cidx := DecodeC(inst)
			var bp, cp *runtime.Value
			if bidx >= RKBit {
				bp = &constants[bidx-RKBit]
			} else {
				bp = &vm.regs[base+bidx]
			}
			if cidx >= RKBit {
				cp = &constants[cidx-RKBit]
			} else {
				cp = &vm.regs[base+cidx]
			}
			if frame.closure.Proto.Feedback != nil {
				fb := &frame.closure.Proto.Feedback[frame.pc-1]
				fb.Left.Observe(bp.Type())
				fb.Right.Observe(cp.Type())
			}
			if le, ok := runtime.LEInts(bp, cp); ok {
				if le != (a != 0) {
					frame.pc++
				}
			} else if le, ok := runtime.LEStrings(bp, cp); ok {
				if le != (a != 0) {
					frame.pc++
				}
			} else {
				le, err := vm.valueLessEqual(*bp, *cp)
				if err != nil {
					return nil, err
				}
				if le != (a != 0) {
					frame.pc++
				}
			}

		// ---- Logical ----
		case OP_TEST:
			a := DecodeA(inst)
			c := DecodeC(inst)
			if vm.regs[base+a].Truthy() != (c != 0) {
				frame.pc++
			}

		case OP_TESTSET:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)
			bv := vm.regs[base+b]
			if bv.Truthy() != (c != 0) {
				frame.pc++
			} else {
				vm.regs[base+a] = bv
			}

		// ---- Jump ----
		case OP_JMP:
			sbx := DecodesBx(inst)
			frame.pc += sbx

		// ---- Call / Return (INLINE) ----
		case OP_YIELD:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)
			nArgs := b - 1
			if b == 0 {
				nArgs = vm.top - (base + a + 1)
			}
			_, err, _ := vm.handleCoroutineYieldFromSlots(base+a, nArgs, c)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			if vm.coroutineYielded {
				return nil, nil
			}

		case OP_RESUME:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)
			nArgs := b - 1
			if b == 0 {
				nArgs = vm.top - (base + a + 1)
			}
			co, args, err := vm.coroutineResumeBoundaryFromSlots(base+a, nArgs)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			co.stackYieldEnabled = vm.ResumePayloadIsFieldOnly(frame.closure.Proto, frame.pc, a, c)
			okResult, values, err := vm.resumeCoroutineRaw(co, args)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.finishCoroutineResumeToSlots(base+a, c, okResult, values)

		case OP_CALLTABLE:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)

			fnVal := vm.regs[base+a]
			argsVal := vm.regs[base+b]
			if !argsVal.IsTable() {
				return nil, fmt.Errorf("CALLTABLE args on non-table")
			}
			argsTable := argsVal.Table()
			nVal := argsTable.RawGet(runtime.StringValue("n"))
			nArgs := int64(argsTable.Length())
			if nVal.IsInt() {
				nArgs = nVal.Int()
			}
			if nArgs < 0 {
				nArgs = 0
			}
			args := make([]runtime.Value, int(nArgs))
			for i := int64(1); i <= nArgs; i++ {
				args[i-1] = argsTable.RawGetInt(i)
			}
			results, err := vm.callValue(fnVal, args)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.writeCallResults(base+a, c, results)

		case OP_CALL:
			a := DecodeA(inst)
			b := DecodeB(inst)
			c := DecodeC(inst)

			fnVal := vm.regs[base+a]
			nArgs := b - 1
			if b == 0 {
				nArgs = vm.top - (base + a + 1)
			}
			callPC := frame.pc - 1
			callerProto := frame.closure.Proto
			if frame.closure.Proto.Feedback != nil {
				proto := frame.closure.Proto
				fb := &proto.Feedback[callPC]
				fb.Left.Observe(fnVal.Type())
				if proto.CallSiteFeedback != nil && callPC >= 0 && callPC < len(proto.CallSiteFeedback) {
					argStart := base + a + 1
					argEnd := argStart + nArgs
					if argStart >= 0 && argEnd >= argStart && argEnd <= len(vm.regs) {
						proto.CallSiteFeedback[callPC].ObserveCall(fnVal, vm.regs[argStart:argEnd], nArgs, c)
					} else {
						proto.CallSiteFeedback[callPC].ObserveCall(fnVal, nil, nArgs, c)
					}
				}
			}

			if fnVal.IsFunction() {
				if gf := fnVal.GoFunction(); gf != nil {
					if handled, err := vm.tryFastCoroutineCall(gf, base, a, nArgs, c); handled {
						if err != nil {
							if err == errCoroutineYield {
								return nil, err
							}
							return nil, wrapLineErr(frame, err)
						}
						if vm.coroutineYielded {
							return nil, nil
						}
						observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
						break
					}
				}
			}

			// GC safe point at ordinary function call boundaries. VM-native
			// coroutine intrinsics above only rearrange already-rooted VM state.
			runtime.CheckGC()

			// Fixed-arity self recursion is common in Tier0-only table walkers
			// such as tree constructors/traversals. When the global lookup still
			// resolved to the active closure, enter the next VM frame directly
			// without the generic closure dispatch, vararg setup, or JIT probe.
			if b != 0 {
				proto := frame.closure.Proto
				if !proto.IsVarArg && nArgs == proto.NumParams && (vm.methodJIT == nil || proto.JITDisabled) {
					if p := fnVal.VMClosurePointer(); p == unsafe.Pointer(frame.closure) {
						newBase := base + proto.MaxStack
						if vm.top > newBase {
							newBase = vm.top
						}
						needed := newBase + proto.MaxStack + 1
						if needed > len(vm.regs) {
							newRegs := runtime.MakeNilSlice(needed * 2)
							copy(newRegs, vm.regs)
							vm.regs = newRegs
						}

						srcStart := base + a + 1
						switch nArgs {
						case 0:
						case 1:
							vm.regs[newBase] = vm.regs[srcStart]
						case 2:
							vm.regs[newBase] = vm.regs[srcStart]
							vm.regs[newBase+1] = vm.regs[srcStart+1]
						default:
							for i := 0; i < nArgs; i++ {
								vm.regs[newBase+i] = vm.regs[srcStart+i]
							}
						}

						if !vm.ensureFrameSlot() {
							return nil, fmt.Errorf("stack overflow (max call depth %d)", maxCallDepth)
						}
						newFrame := &vm.frames[vm.frameCount]
						newFrame.closure = frame.closure
						newFrame.pc = 0
						newFrame.base = newBase
						newFrame.numResults = 0
						newFrame.varargs = nil
						newFrame.resultBase = base + a
						newFrame.resultCount = c
						newFrame.callSitePC = callPC
						newFrame.defers = nil
						vm.frameCount++
						if err := vm.emitDebugHook("call", "script", debugProtoName(proto), runtime.NilValue()); err != nil {
							vm.frameCount--
							return nil, err
						}

						frame = newFrame
						code = proto.Code
						constants = proto.Constants
						base = newBase
						continue
					}
				}
			}

			// ---- Fast path: VM Closure (inline call) ----
			if cl, ok := closureFromValue(fnVal); ok {
				if b != 0 && nArgs == 1 {
					handled, err := vm.tryRecursiveTableBuildFoldRegion(frame, base, cl, a, nArgs, c)
					if handled {
						if err != nil {
							return nil, wrapLineErr(frame, err)
						}
						observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
						break
					}
				}
				if b != 0 && callSiteRuntimeSpecializationArity(nArgs) {
					args := vm.regs[base+a+1 : base+a+1+nArgs]
					handled, err := vm.tryValueRuntimeSpecialization(cl, args, c, base+a)
					if handled {
						if err != nil {
							return nil, wrapLineErr(frame, err)
						}
						observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
						break
					}
					handled, err = vm.tryNoResultRuntimeSpecialization(cl, args, c, base+a)
					if handled {
						if err != nil {
							return nil, wrapLineErr(frame, err)
						}
						observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
						break
					}
				}
				if b != 0 && c > 1 && vm.methodJIT != nil {
					if exec, ok := vm.methodJIT.(methodJITEngineWithCompiledSpecializationCall); ok {
						handled, err := exec.TryExecuteCompiledSpecializationCall(fnVal, vm.regs, base+a, nArgs, c-1)
						if handled {
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
							break
						}
					}
				}

				proto := cl.Proto

				// Compute new base: after current frame's registers
				newBase := base + frame.closure.Proto.MaxStack
				if vm.top > newBase {
					newBase = vm.top
				}

				// Ensure register space
				needed := newBase + proto.MaxStack + 1
				if needed > len(vm.regs) {
					newRegs := runtime.MakeNilSlice(needed * 2)
					copy(newRegs, vm.regs)
					vm.regs = newRegs
				}

				// Copy args directly to new frame's registers
				nParams := proto.NumParams
				srcStart := base + a + 1
				for i := 0; i < nParams && i < nArgs; i++ {
					vm.regs[newBase+i] = vm.regs[srcStart+i]
				}
				for i := nArgs; i < nParams; i++ {
					vm.regs[newBase+i] = runtime.NilValue()
				}

				// Push new frame
				if !vm.ensureFrameSlot() {
					return nil, fmt.Errorf("stack overflow (max call depth %d)", maxCallDepth)
				}
				newFrame := &vm.frames[vm.frameCount]
				newFrame.closure = cl
				newFrame.pc = 0
				newFrame.base = newBase
				if proto.IsVarArg && nArgs > nParams {
					setFrameVarargs(newFrame, vm.regs[srcStart+nParams:srcStart+nArgs])
				} else {
					newFrame.varargs = nil
				}
				newFrame.resultBase = base + a
				newFrame.resultCount = c
				newFrame.callSitePC = callPC
				newFrame.defers = nil
				vm.frameCount++
				if err := vm.emitDebugHook("call", "script", debugProtoName(proto), runtime.NilValue()); err != nil {
					vm.frameCount--
					return nil, err
				}

				// Method JIT: check for compiled function
				if vm.methodJIT != nil && proto.MethodJITTier1Callable() && !proto.JITDisabled {
					proto.CallCount++
					if proto.CallCount <= 64 {
						argEnd := newBase + nArgs
						if newBase >= 0 && argEnd >= newBase && argEnd <= len(vm.regs) {
							proto.ObserveArgShapes(vm.regs[newBase:argEnd])
							proto.ObserveArgArrayElementShapes(vm.regs[newBase:argEnd])
						}
					}
					if compiled := vm.methodJIT.TryCompile(proto); compiled != nil {
						results, err := vm.executeMethodJIT(compiled, vm.regs, newBase, proto)
						if err == errCoroutineYield {
							return results, err
						}
						if err == nil {
							vm.closeUpvalues(newBase)
							if err := vm.emitDebugHook("return", "script", debugProtoName(proto), runtime.NilValue()); err != nil {
								vm.frameCount--
								return nil, err
							}
							vm.frameCount--
							if c == 0 {
								for i, r := range results {
									vm.regs[base+a+i] = r
								}
								vm.top = base + a + len(results)
							} else {
								nr := c - 1
								for i := 0; i < nr; i++ {
									if i < len(results) {
										vm.regs[base+a+i] = results[i]
									} else {
										vm.regs[base+a+i] = runtime.NilValue()
									}
								}
							}
							observeCallResultSlice(callerProto, callPC, results, c)
							break
						}
						// Compilation or execution failed; fall through to interpreter.
					}
				}

				// Switch to new frame (inline).
				frame = newFrame
				code = proto.Code
				constants = proto.Constants
				base = newBase
				continue
			}

			// ---- Fast path: GoFunction (direct call, skip callValue) ----
			if fnVal.IsFunction() {
				if gf := fnVal.GoFunction(); gf != nil {
					handledSpecial := false
					switch gf.NativeKind {
					case runtime.NativeKindStdSelect:
						if gf.NativeData == runtime.StdSelectIdentityPtr() {
							if err := vm.executeStdSelectCall(base+a, nArgs, c, gf); err != nil {
								return nil, wrapLineErr(frame, err)
							}
							observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
							handledSpecial = true
						}
					case runtime.NativeKindStdIPairs:
						if gf.NativeData == runtime.StdIPairsIdentityPtr() {
							handled, err := vm.ExecuteStdIPairsCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					case runtime.NativeKindStdPairs:
						if gf.NativeData == runtime.StdPairsIdentityPtr() {
							handled, err := vm.ExecuteStdPairsCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					}
					if handledSpecial {
						break
					}
					var args []runtime.Value
					if nArgs <= len(vm.argBuf) {
						args = vm.argBuf[:nArgs]
					} else {
						args = make([]runtime.Value, nArgs)
					}
					for i := 0; i < nArgs; i++ {
						args[i] = vm.regs[base+a+1+i]
					}
					results, err := vm.callGoFunction(gf, args)
					if err != nil {
						return nil, wrapLineErr(frame, err)
					}
					if gf.Fast1 != nil {
						result := runtime.NilValue()
						if len(results) > 0 {
							result = results[0]
						}
						if c == 0 {
							vm.regs[base+a] = result
							vm.top = base + a + 1
						} else {
							nr := c - 1
							if nr > 0 {
								vm.regs[base+a] = result
								for i := 1; i < nr; i++ {
									vm.regs[base+a+i] = runtime.NilValue()
								}
							}
						}
						observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
						break
					}
					if c == 0 {
						for i, r := range results {
							vm.regs[base+a+i] = r
						}
						vm.top = base + a + len(results)
					} else {
						nr := c - 1
						for i := 0; i < nr; i++ {
							if i < len(results) {
								vm.regs[base+a+i] = results[i]
							} else {
								vm.regs[base+a+i] = runtime.NilValue()
							}
						}
					}
					observeCallResultSlice(callerProto, callPC, results, c)
					break
				}
			}

			// ---- Slow path: __call metamethod, tree-walker closures, etc. ----
			var args []runtime.Value
			if nArgs <= len(vm.argBuf) {
				args = vm.argBuf[:nArgs]
			} else {
				args = make([]runtime.Value, nArgs)
			}
			for i := 0; i < nArgs; i++ {
				args[i] = vm.regs[base+a+1+i]
			}
			results, err := vm.callValue(fnVal, args)
			if err != nil {
				return nil, wrapLineErr(frame, err)
			}
			if c == 0 {
				for i, r := range results {
					vm.regs[base+a+i] = r
				}
				vm.top = base + a + len(results)
			} else {
				nr := c - 1
				for i := 0; i < nr; i++ {
					if i < len(results) {
						vm.regs[base+a+i] = results[i]
					} else {
						vm.regs[base+a+i] = runtime.NilValue()
					}
				}
			}
			observeCallResultSlice(callerProto, callPC, results, c)

		case OP_DEFER:
			a := DecodeA(inst)
			b := DecodeB(inst)
			nArgs := b - 1
			if b == 0 {
				nArgs = vm.top - (base + a + 1)
			}
			if nArgs < 0 {
				nArgs = 0
			}
			args := make([]runtime.Value, nArgs)
			for i := 0; i < nArgs; i++ {
				args[i] = vm.regs[base+a+1+i]
			}
			frame.defers = append(frame.defers, deferredVMCall{fn: vm.regs[base+a], args: args})

		case OP_RETURN:
			a := DecodeA(inst)
			b := DecodeB(inst)

			if err := vm.drainFrameDefers(frame); err != nil {
				return nil, wrapLineErr(frame, err)
			}
			vm.closeUpvalues(base)

			// Initial frame return → back to Go caller (call() will pop)
			if vm.frameCount <= initialFC {
				if b == 0 {
					nret := vm.top - (base + a)
					var ret []runtime.Value
					if nret <= len(vm.retBuf) {
						ret = vm.retBuf[:nret]
					} else {
						ret = make([]runtime.Value, nret)
					}
					for i := 0; i < nret; i++ {
						ret[i] = vm.regs[base+a+i]
					}
					return ret, nil
				}
				if b == 1 {
					return nil, nil
				}
				nret := b - 1
				var ret []runtime.Value
				if nret <= len(vm.retBuf) {
					ret = vm.retBuf[:nret]
				} else {
					ret = make([]runtime.Value, nret)
				}
				for i := 0; i < nret; i++ {
					ret[i] = vm.regs[base+a+i]
				}
				return ret, nil
			}

			// Inline sub-frame return
			childCallSitePC := frame.callSitePC
			vm.frameCount--

			resultBase := frame.resultBase
			resultCount := frame.resultCount

			var nret int
			if b == 0 {
				nret = vm.top - (base + a)
			} else if b == 1 {
				nret = 0
			} else {
				nret = b - 1
			}

			if resultCount == 0 {
				// Return all results
				for i := 0; i < nret; i++ {
					vm.regs[resultBase+i] = vm.regs[base+a+i]
				}
				vm.top = resultBase + nret
			} else {
				nr := resultCount - 1
				for i := 0; i < nr; i++ {
					if i < nret {
						vm.regs[resultBase+i] = vm.regs[base+a+i]
					} else {
						vm.regs[resultBase+i] = runtime.NilValue()
					}
				}
			}
			if vm.frameCount > 0 {
				observeCallResultFixed(vm.frames[vm.frameCount-1].closure.Proto, childCallSitePC, vm.regs, resultBase, resultCount)
			}
			if err := vm.emitDebugHook("return", "script", debugProtoName(frame.closure.Proto), runtime.NilValue()); err != nil {
				return nil, err
			}

			// Restore parent frame
			frame = &vm.frames[vm.frameCount-1]
			code = frame.closure.Proto.Code
			constants = frame.closure.Proto.Constants
			base = frame.base
			continue

		case OP_CLOSURE:
			a := DecodeA(inst)
			bx := DecodeBx(inst)
			subProto := frame.closure.Proto.Protos[bx]
			cl := NewClosure(subProto)
			switch len(subProto.Upvalues) {
			case 0:
			case 1:
				desc := subProto.Upvalues[0]
				if desc.InStack {
					cl.Upvalues[0] = vm.findOrCreateUpvalue(base + desc.Index)
				} else {
					cl.Upvalues[0] = frame.closure.Upvalues[desc.Index]
				}
			default:
				for i, desc := range subProto.Upvalues {
					if desc.InStack {
						cl.Upvalues[i] = vm.findOrCreateUpvalue(base + desc.Index)
					} else {
						cl.Upvalues[i] = frame.closure.Upvalues[desc.Index]
					}
				}
			}
			vm.regs[base+a] = runtime.VMClosureFastValue(unsafe.Pointer(cl))

		case OP_CLOSE:
			a := DecodeA(inst)
			vm.closeUpvalues(base + a)

		// ---- Numeric For Loop ----
		case OP_FORPREP:
			a := DecodeA(inst)
			sbx := DecodesBx(inst)
			if handled, err := vm.tryRunDriverLoopRuntimeSpecialization(frame, base, code, constants, a, sbx); err != nil {
				return nil, err
			} else if handled {
				continue
			}
			initV := vm.regs[base+a]
			stepV := vm.regs[base+a+2]
			if initV.IsInt() && stepV.IsInt() {
				vm.regs[base+a] = runtime.IntValue(initV.Int() - stepV.Int())
			} else {
				vm.regs[base+a] = runtime.FloatValue(initV.Number() - stepV.Number())
			}
			frame.pc += sbx

		case OP_FORLOOP:
			a := DecodeA(inst)
			sbx := DecodesBx(inst)
			idxP := &vm.regs[base+a]
			if idxP.RawType() == runtime.TypeInt {
				stepP := &vm.regs[base+a+2]
				limitP := &vm.regs[base+a+1]
				if stepP.RawType() == runtime.TypeInt && limitP.RawType() == runtime.TypeInt {
					step := stepP.RawInt()
					idx := idxP.RawInt() + step
					limit := limitP.RawInt()
					var cont bool
					if step > 0 {
						cont = idx <= limit
					} else {
						cont = idx >= limit
					}
					if cont {
						idxP.SetIntUnchecked(idx)
						vm.regs[base+a+3].SetIntUnchecked(idx)
						frame.pc += sbx
					}
					break
				}
			}
			step := vm.regs[base+a+2].Number()
			limit := vm.regs[base+a+1].Number()
			idx := vm.regs[base+a].Number() + step
			cont := false
			if step > 0 {
				cont = idx <= limit
			} else {
				cont = idx >= limit
			}
			if cont {
				if floatIsExactInt(idx) {
					vm.regs[base+a] = runtime.IntValue(int64(idx))
					vm.regs[base+a+3] = runtime.IntValue(int64(idx))
				} else {
					vm.regs[base+a] = runtime.FloatValue(idx)
					vm.regs[base+a+3] = runtime.FloatValue(idx)
				}
				frame.pc += sbx
			} else {
				vm.regs[base+a] = runtime.FloatValue(idx)
			}

		case OP_VARARG:
			a := DecodeA(inst)
			b := DecodeB(inst)
			va := frame.varargs
			if b == 0 {
				handled, err := vm.tryExecuteStdSelectVarargPeephole(frame, base, a, va)
				if err != nil {
					return nil, wrapLineErr(frame, err)
				}
				if handled {
					continue
				}
				for i, v := range va {
					vm.regs[base+a+i] = v
				}
				vm.top = base + a + len(va)
			} else {
				n := b - 1
				for i := 0; i < n; i++ {
					if i < len(va) {
						vm.regs[base+a+i] = va[i]
					} else {
						vm.regs[base+a+i] = runtime.NilValue()
					}
				}
			}

		case OP_SETTOP:
			a := DecodeA(inst)
			vm.top = base + a

		case OP_SELF:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cidx := DecodeC(inst)
			obj := vm.regs[base+b]
			vm.regs[base+a+1] = obj
			var key runtime.Value
			if cidx >= RKBit {
				key = constants[cidx-RKBit]
			} else {
				key = vm.regs[base+cidx]
			}
			val, err := vm.tableGet(obj, key)
			if err != nil {
				return nil, err
			}
			vm.regs[base+a] = val

		case OP_TFORCALL:
			a := DecodeA(inst)
			c := DecodeC(inst)
			fnVal := vm.regs[base+a]

			if fnVal.IsChannel() {
				ch := fnVal.Channel()
				val, ok := ch.Recv()
				if ok {
					vm.regs[base+a+3] = val
					for i := 1; i < c; i++ {
						vm.regs[base+a+3+i] = runtime.NilValue()
					}
				} else {
					for i := 0; i < c; i++ {
						vm.regs[base+a+3+i] = runtime.NilValue()
					}
				}
			} else {
				if gf := fnVal.GoFunction(); gf != nil && gf.FastArg1Ret2 != nil {
					runtime.RecordRuntimePathNativeCallFastFor(gf)
					r0, r1, n, err := gf.FastArg1Ret2(vm.regs[base+a+1])
					if err != nil {
						return nil, err
					}
					for i := 0; i < c; i++ {
						switch {
						case i == 0 && n > 0:
							vm.regs[base+a+3+i] = r0
						case i == 1 && n > 1:
							vm.regs[base+a+3+i] = r1
						default:
							vm.regs[base+a+3+i] = runtime.NilValue()
						}
					}
				} else if gf := fnVal.GoFunction(); gf != nil && gf.FastArg2Ret2 != nil {
					runtime.RecordRuntimePathNativeCallFastFor(gf)
					r0, r1, n, err := gf.FastArg2Ret2(vm.regs[base+a+1], vm.regs[base+a+2])
					if err != nil {
						return nil, err
					}
					for i := 0; i < c; i++ {
						switch {
						case i == 0 && n > 0:
							vm.regs[base+a+3+i] = r0
						case i == 1 && n > 1:
							vm.regs[base+a+3+i] = r1
						default:
							vm.regs[base+a+3+i] = runtime.NilValue()
						}
					}
				} else {
					args := []runtime.Value{vm.regs[base+a+1], vm.regs[base+a+2]}
					results, err := vm.callValue(fnVal, args)
					if err != nil {
						return nil, err
					}
					for i := 0; i < c; i++ {
						if i < len(results) {
							vm.regs[base+a+3+i] = results[i]
						} else {
							vm.regs[base+a+3+i] = runtime.NilValue()
						}
					}
				}
			}

		case OP_TFORLOOP:
			a := DecodeA(inst)
			sbx := DecodesBx(inst)
			if !vm.regs[base+a+1].IsNil() {
				vm.regs[base+a] = vm.regs[base+a+1]
				frame.pc += sbx
			}

		case OP_GO:
			// Mark shared table objects as concurrent and switch parent
			// to locked mode (prevents concurrent writes to globalIndex).
			if vm.noGlobalLock {
				vm.markGlobalTablesConcurrent()
				vm.noGlobalLock = false
				vm.globalVer++
			}

			a := DecodeA(inst)
			b := DecodeB(inst)
			fnVal := vm.regs[base+a]
			nArgs := b - 1
			if b == 0 {
				nArgs = vm.top - (base + a + 1)
			}
			args := make([]runtime.Value, nArgs)
			for i := 0; i < nArgs; i++ {
				args[i] = vm.regs[base+a+1+i]
			}
			go func(fn runtime.Value, goArgs []runtime.Value) {
				goVM := newIsolatedChildVM(vm)
				if cl, ok := closureFromValue(fn); ok {
					goVM.call(cl, goArgs, 0, 0)
				} else if gf := fn.GoFunction(); gf != nil {
					gf.Fn(goArgs)
				}
			}(fnVal, args)

		case OP_MAKECHAN:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cc := DecodeC(inst)
			capacity := 0
			if cc == 1 {
				sizeVal := vm.regs[base+b]
				var err error
				capacity, err = runtime.ChannelCapacityFromValue(sizeVal, "make(chan)")
				if err != nil {
					return nil, err
				}
			}
			ch := runtime.NewChannel(capacity)
			vm.regs[base+a] = runtime.ChannelValue(ch)

		case OP_SEND:
			a := DecodeA(inst)
			b := DecodeB(inst)
			chVal := vm.regs[base+a]
			if !chVal.IsChannel() {
				return nil, fmt.Errorf("send on non-channel value (got %s)", chVal.TypeName())
			}
			ch := chVal.Channel()
			val := vm.regs[base+b]
			if err := ch.Send(val); err != nil {
				return nil, err
			}

		case OP_RECV:
			a := DecodeA(inst)
			b := DecodeB(inst)
			chVal := vm.regs[base+b]
			if !chVal.IsChannel() {
				return nil, fmt.Errorf("receive from non-channel value (got %s)", chVal.TypeName())
			}
			ch := chVal.Channel()
			val, ok := ch.Recv()
			if ok {
				vm.regs[base+a] = val
			} else {
				vm.regs[base+a] = runtime.NilValue()
			}

		default:
			return nil, fmt.Errorf("unhandled opcode %d (%s)", op, OpName(op))
		}
	}
}

func (vm *VM) tryFastCoroutineCall(gf *runtime.GoFunction, base, a, nArgs, c int) (bool, error) {
	switch gf.NativeKind {
	case goFunctionKindCoroutineWrapper:
		co, ok := vmCoroutineFromNativeData(gf.NativeData)
		if !ok {
			return true, fmt.Errorf("invalid wrapped coroutine")
		}
		if handled, err := vm.tryFastWrappedGeneratorCall(co, base, a, nArgs, c); handled {
			return true, err
		}
		if co.status == VMCoroutineDead {
			return true, fmt.Errorf("cannot resume dead coroutine")
		}
		var args []runtime.Value
		if nArgs > 0 {
			start := base + a + 1
			args = vm.regs[start : start+nArgs]
		}
		okResult, values, err := vm.resumeCoroutineRaw(co, args)
		if err != nil {
			return true, err
		}
		if !okResult {
			if len(values) > 0 {
				return true, fmt.Errorf("%s", values[0].String())
			}
			return true, fmt.Errorf("cannot resume dead coroutine")
		}
		if len(values) == 0 {
			vm.writeSingleCallResult(base+a, c, runtime.NilValue())
			return true, nil
		}
		vm.writeCallResults(base+a, c, values)
		return true, nil

	case goFunctionKindCoroutineCreate:
		if nArgs < 1 || !vm.regs[base+a+1].IsFunction() {
			return true, fmt.Errorf("coroutine.create expects a function")
		}
		cl, ok := closureFromValue(vm.regs[base+a+1])
		if !ok {
			gf := vm.regs[base+a+1].GoFunction()
			if gf == nil {
				return true, fmt.Errorf("coroutine.create expects a GScript function")
			}
			co := NewVMGoCoroutine(gf)
			vm.recordCoroutineCreated(false)
			vm.writeSingleCallResult(base+a, c, runtime.VMCoroutineValue(unsafe.Pointer(co), co))
			return true, nil
		}
		co := NewVMCoroutine(cl)
		vm.recordCoroutineCreated(false)
		vm.writeSingleCallResult(base+a, c, runtime.VMCoroutineValue(unsafe.Pointer(co), co))
		return true, nil

	case goFunctionKindCoroutineResume:
		co, args, err := vm.coroutineResumeBoundaryFromSlots(base+a, nArgs)
		if err != nil {
			return true, err
		}
		okResult, values, err := vm.resumeCoroutineRaw(co, args)
		if err != nil {
			return true, err
		}
		vm.finishCoroutineResumeToSlots(base+a, c, okResult, values)
		return true, nil

	case goFunctionKindCoroutineYield:
		results, err, suspended := vm.handleCoroutineYieldFromSlots(base+a, nArgs, c)
		if err != nil {
			return true, err
		}
		if suspended {
			return true, nil
		}
		vm.writeCallResults(base+a, c, results)
		return true, nil

	case goFunctionKindCoroutineIsYield:
		vm.writeSingleCallResult(base+a, c, runtime.BoolValue(vm.activeCoroutine() != nil))
		return true, nil
	}

	if gf == vm.coroutineCreateFn || gf.Name == coroutineCreateName {
		if nArgs < 1 || !vm.regs[base+a+1].IsFunction() {
			return true, fmt.Errorf("coroutine.create expects a function")
		}
		cl, ok := closureFromValue(vm.regs[base+a+1])
		if !ok {
			gf := vm.regs[base+a+1].GoFunction()
			if gf == nil {
				return true, fmt.Errorf("coroutine.create expects a GScript function")
			}
			co := NewVMGoCoroutine(gf)
			vm.recordCoroutineCreated(false)
			vm.writeSingleCallResult(base+a, c, runtime.VMCoroutineValue(unsafe.Pointer(co), co))
			return true, nil
		}
		co := NewVMCoroutine(cl)
		vm.recordCoroutineCreated(false)
		vm.writeSingleCallResult(base+a, c, runtime.VMCoroutineValue(unsafe.Pointer(co), co))
		return true, nil
	}

	if gf == vm.coroutineResumeFn || gf.Name == coroutineResumeName {
		co, args, err := vm.coroutineResumeBoundaryFromSlots(base+a, nArgs)
		if err != nil {
			return true, err
		}
		okResult, values, err := vm.resumeCoroutineRaw(co, args)
		if err != nil {
			return true, err
		}
		vm.finishCoroutineResumeToSlots(base+a, c, okResult, values)
		return true, nil
	}

	if gf == vm.coroutineYieldFn || gf.Name == coroutineYieldName {
		results, err, suspended := vm.handleCoroutineYieldFromSlots(base+a, nArgs, c)
		if err != nil {
			return true, err
		}
		if suspended {
			return true, nil
		}
		vm.writeCallResults(base+a, c, results)
		return true, nil
	}

	if gf.Name == coroutineIsYieldableName {
		vm.writeSingleCallResult(base+a, c, runtime.BoolValue(vm.activeCoroutine() != nil))
		return true, nil
	}

	return false, nil
}

func (vm *VM) writeSingleCallResult(dst, c int, result runtime.Value) {
	if c == 0 {
		vm.regs[dst] = result
		vm.top = dst + 1
		return
	}
	if c == 1 {
		return
	}
	vm.regs[dst] = result
	for i := 1; i < c-1; i++ {
		vm.regs[dst+i] = runtime.NilValue()
	}
}

func (vm *VM) writeCoroutineResumeResults(dst, c int, ok bool, values []runtime.Value) {
	if c == 0 {
		vm.regs[dst] = runtime.BoolValue(ok)
		for i, r := range values {
			vm.regs[dst+1+i] = r
		}
		vm.top = dst + 1 + len(values)
		return
	}
	if c == 3 && len(values) == 1 {
		vm.regs[dst] = runtime.BoolValue(ok)
		vm.regs[dst+1] = values[0]
		return
	}
	if c == 2 && len(values) == 0 {
		vm.regs[dst] = runtime.BoolValue(ok)
		return
	}
	nr := c - 1
	for i := 0; i < nr; i++ {
		switch {
		case i == 0:
			vm.regs[dst] = runtime.BoolValue(ok)
		case i-1 < len(values):
			vm.regs[dst+i] = values[i-1]
		default:
			vm.regs[dst+i] = runtime.NilValue()
		}
	}
}

func (vm *VM) ResumePayloadIsFieldOnly(proto *FuncProto, nextPC, resumeA, c int) bool {
	if proto == nil || c != 3 {
		return false
	}
	if nextPC >= 0 && nextPC < len(proto.Code) {
		if proto.ResumePayloadCache == nil {
			proto.ResumePayloadCache = make([]int8, len(proto.Code))
		}
		switch proto.ResumePayloadCache[nextPC] {
		case 1:
			return false
		case 2:
			return true
		}
		result := vm.resumePayloadIsFieldOnlyUncached(proto, nextPC, resumeA, c)
		if result {
			proto.ResumePayloadCache[nextPC] = 2
		} else {
			proto.ResumePayloadCache[nextPC] = 1
		}
		return result
	}
	return vm.resumePayloadIsFieldOnlyUncached(proto, nextPC, resumeA, c)
}

func (vm *VM) resumePayloadIsFieldOnlyUncached(proto *FuncProto, nextPC, resumeA, c int) bool {
	payloadReg := resumeA + 1
	for pc := nextPC; pc < len(proto.Code); pc++ {
		inst := proto.Code[pc]
		op := DecodeOp(inst)
		a := DecodeA(inst)
		b := DecodeB(inst)
		cc := DecodeC(inst)

		switch op {
		case OP_GETFIELD:
			if b == payloadReg {
				continue
			}
			if a == payloadReg {
				return true
			}
		case OP_RESUME, OP_FORLOOP:
			return true
		case OP_RETURN:
			return !registerRangeMayRead(payloadReg, a, b)
		case OP_JMP:
			if DecodesBx(inst) < 0 {
				return true
			}
		case OP_MOVE, OP_UNM, OP_BNOT, OP_NOT, OP_LEN:
			if b == payloadReg {
				return false
			}
			if a == payloadReg {
				return true
			}
		case OP_GETTABLE, OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_POW, OP_BAND, OP_BOR, OP_BXOR, OP_BANDN, OP_SHL, OP_SHR, OP_EQ, OP_LT, OP_LE:
			if b == payloadReg || (cc < RKBit && cc == payloadReg) {
				return false
			}
			if op != OP_EQ && op != OP_LT && op != OP_LE && a == payloadReg {
				return true
			}
		case OP_SETTABLE:
			if a == payloadReg || b == payloadReg || (cc < RKBit && cc == payloadReg) {
				return false
			}
		case OP_SETFIELD:
			if a == payloadReg || cc == payloadReg {
				return false
			}
		case OP_TEST:
			if a == payloadReg {
				return false
			}
		case OP_TESTSET:
			if b == payloadReg {
				return false
			}
			if a == payloadReg {
				return true
			}
		case OP_CALL, OP_YIELD, OP_GO:
			if callRegisterRangeMayRead(payloadReg, a, b) {
				return false
			}
			if callRegisterRangeMayWrite(payloadReg, a, cc) {
				return true
			}
		case OP_TFORCALL:
			if payloadReg >= a && payloadReg <= a+2 {
				return false
			}
			if payloadReg >= a+3 && payloadReg <= a+2+cc {
				return true
			}
		case OP_TFORLOOP:
			if payloadReg == a || payloadReg == a+1 {
				return false
			}
		case OP_SETGLOBAL, OP_SETGLOBALRO, OP_SETUPVAL, OP_CLOSE, OP_APPEND, OP_SEND, OP_DEFER, OP_CHECKCONST:
			if a == payloadReg || b == payloadReg {
				return false
			}
		case OP_SELF:
			if b == payloadReg || (cc < RKBit && cc == payloadReg) {
				return false
			}
			if a == payloadReg || a+1 == payloadReg {
				return true
			}
		case OP_CONCAT:
			if payloadReg >= b && payloadReg <= cc {
				return false
			}
			if a == payloadReg {
				return true
			}
		case OP_SETLIST:
			if a == payloadReg || (payloadReg > a && payloadReg <= a+b) {
				return false
			}
		case OP_FORPREP:
			if payloadReg >= a && payloadReg <= a+3 {
				return false
			}
		case OP_RECV:
			if a == payloadReg {
				return true
			}
			if b == payloadReg {
				return false
			}
		default:
			if a == payloadReg {
				return true
			}
		}
	}
	return true
}

func registerRangeMayRead(reg, start, b int) bool {
	if b == 0 {
		return reg >= start
	}
	return reg >= start && reg < start+b-1
}

func callRegisterRangeMayRead(reg, a, b int) bool {
	if b == 0 {
		return reg >= a
	}
	return reg >= a && reg < a+b
}

func callRegisterRangeMayWrite(reg, a, c int) bool {
	if c == 0 {
		return reg >= a
	}
	return reg >= a && reg < a+c-1
}

func (vm *VM) writeCallResults(dst, c int, results []runtime.Value) {
	if c == 0 {
		for i, r := range results {
			vm.regs[dst+i] = r
		}
		vm.top = dst + len(results)
		return
	}
	if c == 1 {
		return
	}
	if c == 2 {
		if len(results) > 0 {
			vm.regs[dst] = results[0]
		} else {
			vm.regs[dst] = runtime.NilValue()
		}
		return
	}
	nr := c - 1
	for i := 0; i < nr; i++ {
		if i < len(results) {
			vm.regs[dst+i] = results[i]
		} else {
			vm.regs[dst+i] = runtime.NilValue()
		}
	}
}

// callValue dispatches a function call (supports Closure, GoFunction, and __call metamethod).
func (vm *VM) callValue(fnVal runtime.Value, args []runtime.Value) ([]runtime.Value, error) {
	if fnVal.IsFunction() {
		if cl, ok := closureFromValue(fnVal); ok {
			if callSiteRuntimeSpecializationArity(len(args)) {
				if handled, results, err := vm.tryRunNonRecursiveTableValueRuntimeSpecialization(cl, args); handled {
					return results, err
				}
				if handled, err := vm.tryRunNoResultRuntimeSpecialization(cl, args); handled {
					return nil, err
				}
			}
			newBase := vm.top
			if vm.frameCount > 0 {
				curFrame := &vm.frames[vm.frameCount-1]
				minBase := curFrame.base + curFrame.closure.Proto.MaxStack
				if newBase < minBase {
					newBase = minBase
				}
			}
			return vm.call(cl, args, newBase, -1)
		}
		if gf := fnVal.GoFunction(); gf != nil {
			return vm.callGoFunction(gf, args)
		}
		if c := fnVal.Closure(); c != nil {
			return nil, fmt.Errorf("cannot call tree-walker closure from VM")
		}
	}
	if fnVal.IsTable() {
		mt := fnVal.Table().GetMetatable()
		if mt != nil {
			callMM := mt.RawGet(runtime.StringValue("__call"))
			if !callMM.IsNil() {
				newArgs := make([]runtime.Value, len(args)+1)
				newArgs[0] = fnVal
				copy(newArgs[1:], args)
				return vm.callValue(callMM, newArgs)
			}
		}
	}
	return nil, fmt.Errorf("attempt to call a %s value", fnVal.TypeName())
}

// tableGet performs table access with __index metamethod support.
func (vm *VM) tableGet(t runtime.Value, key runtime.Value) (runtime.Value, error) {
	return vm.tableGetDepth(t, key, 0)
}

func (vm *VM) tableGetDepth(t runtime.Value, key runtime.Value, depth int) (runtime.Value, error) {
	if depth > maxMetaDepth {
		return runtime.NilValue(), fmt.Errorf("__index chain too deep")
	}

	if t.IsString() {
		if vm.stringMeta != nil {
			v := vm.stringMeta.RawGet(key)
			if !v.IsNil() {
				return v, nil
			}
		}
		return runtime.NilValue(), nil
	}

	if key.IsString() {
		if v, ok := t.FixedRecordRawGetString(key.Str()); ok {
			return v, nil
		}
	}

	if !t.IsTable() {
		if t.IsNil() && vm.frameCount > 0 {
			frame := &vm.frames[vm.frameCount-1]
			fmt.Printf("[DEBUG] attempt to index nil in %s pc=%d key=%v\n",
				frame.closure.Proto.Name, frame.pc, key)
		}
		return runtime.NilValue(), fmt.Errorf("attempt to index a %s value", t.TypeName())
	}

	tbl := t.Table()
	v := tbl.RawGet(key)
	if !v.IsNil() {
		return v, nil
	}

	mt := tbl.GetMetatable()
	if mt == nil {
		return runtime.NilValue(), nil
	}
	idx := mt.RawGet(runtime.StringValue("__index"))
	if idx.IsNil() {
		return runtime.NilValue(), nil
	}
	if idx.IsTable() {
		return vm.tableGetDepth(runtime.TableValue(idx.Table()), key, depth+1)
	}
	if idx.IsFunction() {
		args := [2]runtime.Value{t, key}
		results, err := vm.callValue(idx, args[:])
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return runtime.NilValue(), nil
	}
	return runtime.NilValue(), nil
}

// tableSet performs table assignment with __newindex metamethod support.
func (vm *VM) tableSet(t runtime.Value, key runtime.Value, val runtime.Value) error {
	if !t.IsTable() {
		return fmt.Errorf("attempt to index a %s value", t.TypeName())
	}
	tbl := t.Table()

	existing := tbl.RawGet(key)
	if existing.IsNil() {
		mt := tbl.GetMetatable()
		if mt != nil {
			ni := mt.RawGet(runtime.StringValue("__newindex"))
			if !ni.IsNil() {
				if ni.IsFunction() {
					args := [3]runtime.Value{t, key, val}
					_, err := vm.callValue(ni, args[:])
					return err
				}
				if ni.IsTable() {
					return vm.tableSet(runtime.TableValue(ni.Table()), key, val)
				}
			}
		}
	}

	tbl.RawSet(key, val)
	return nil
}

func (vm *VM) tableLenInt(t runtime.Value) (int64, error) {
	if !t.IsTable() {
		return 0, fmt.Errorf("attempt to get length of a %s value", t.TypeName())
	}
	l, err := vm.length(t)
	if err != nil {
		return 0, err
	}
	return vmToInt(l), nil
}

func vmToInt(v runtime.Value) int64 {
	switch v.Type() {
	case runtime.TypeInt:
		return v.Int()
	case runtime.TypeFloat:
		return int64(v.Float())
	case runtime.TypeString:
		n, ok := v.ToNumber()
		if ok {
			return vmToInt(n)
		}
		return 0
	default:
		return 0
	}
}

// ---- Arithmetic helpers ----

func (vm *VM) arith(a, b runtime.Value, metamethod string, op func(float64, float64) float64) (runtime.Value, error) {
	if a.IsInt() && b.IsInt() {
		switch metamethod {
		case "__add":
			return runtime.IntValue(a.Int() + b.Int()), nil
		case "__sub":
			return runtime.IntValue(a.Int() - b.Int()), nil
		case "__mul":
			return runtime.IntValue(a.Int() * b.Int()), nil
		case "__pow":
			return runtime.FloatValue(math.Pow(float64(a.Int()), float64(b.Int()))), nil
		}
	}
	if a.IsNumber() && b.IsNumber() {
		result := op(a.Number(), b.Number())
		if a.IsInt() && b.IsInt() && metamethod != "__div" && metamethod != "__pow" {
			if floatIsExactInt(result) {
				return runtime.IntValue(int64(result)), nil
			}
		}
		return runtime.FloatValue(result), nil
	}
	ac, aok := a.ToNumber()
	bc, bok := b.ToNumber()
	if aok && bok {
		return vm.arith(ac, bc, metamethod, op)
	}
	mm, err := vm.getMetamethod(a, b, metamethod)
	if err == nil && !mm.IsNil() {
		args := [2]runtime.Value{a, b}
		results, err := vm.callValue(mm, args[:])
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return runtime.NilValue(), nil
	}
	return runtime.NilValue(), fmt.Errorf("attempt to perform arithmetic on %s and %s", a.TypeName(), b.TypeName())
}

func (vm *VM) arithMod(a, b runtime.Value) (runtime.Value, error) {
	if a.IsInt() && b.IsInt() {
		bi := b.Int()
		if bi == 0 {
			return runtime.NilValue(), fmt.Errorf("attempt to perform 'n%%0'")
		}
		r := a.Int() % bi
		if r != 0 && (r^bi) < 0 {
			r += bi
		}
		return runtime.IntValue(r), nil
	}
	if a.IsNumber() && b.IsNumber() {
		bf := b.Number()
		if bf == 0 {
			return runtime.NilValue(), fmt.Errorf("attempt to perform 'n%%0'")
		}
		r := math.Mod(a.Number(), bf)
		if r != 0 && (r < 0) != (bf < 0) {
			r += bf
		}
		return runtime.FloatValue(r), nil
	}
	return vm.arith(a, b, "__mod", func(x, y float64) float64 { return math.Mod(x, y) })
}

func bitwiseInt(v runtime.Value) (int64, error) {
	n, ok := v.ToNumber()
	if !ok {
		return 0, fmt.Errorf("attempt to perform bitwise operation on %s", v.TypeName())
	}
	if n.IsInt() {
		return n.Int(), nil
	}
	return int64(n.Float()), nil
}

func bitwiseShift(v runtime.Value) (uint, error) {
	n, err := bitwiseInt(v)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("negative shift count")
	}
	return uint(n), nil
}

func bitwiseBinary(op Opcode, a, b runtime.Value) (runtime.Value, error) {
	x, err := bitwiseInt(a)
	if err != nil {
		return runtime.NilValue(), err
	}
	y, err := bitwiseInt(b)
	if err != nil {
		return runtime.NilValue(), err
	}
	switch op {
	case OP_BAND:
		return runtime.IntValue(x & y), nil
	case OP_BOR:
		return runtime.IntValue(x | y), nil
	case OP_BXOR:
		return runtime.IntValue(x ^ y), nil
	case OP_BANDN:
		return runtime.IntValue(x &^ y), nil
	case OP_SHL:
		shift, err := bitwiseShift(b)
		if err != nil {
			return runtime.NilValue(), err
		}
		if shift >= 64 {
			return runtime.IntValue(0), nil
		}
		return runtime.IntValue(int64(uint64(x) << shift)), nil
	case OP_SHR:
		shift, err := bitwiseShift(b)
		if err != nil {
			return runtime.NilValue(), err
		}
		if shift >= 64 {
			return runtime.IntValue(0), nil
		}
		return runtime.IntValue(int64(uint64(x) >> shift)), nil
	default:
		return runtime.NilValue(), fmt.Errorf("unsupported bitwise opcode %s", OpName(op))
	}
}

func (vm *VM) unaryMinus(v runtime.Value) (runtime.Value, error) {
	if v.IsInt() {
		return runtime.IntValue(-v.Int()), nil
	}
	if v.IsFloat() {
		return runtime.FloatValue(-v.Float()), nil
	}
	if nv, ok := v.ToNumber(); ok {
		return vm.unaryMinus(nv)
	}
	mm, err := vm.getMetamethod(v, v, "__unm")
	if err == nil && !mm.IsNil() {
		args := [1]runtime.Value{v}
		results, err := vm.callValue(mm, args[:])
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
	}
	return runtime.NilValue(), fmt.Errorf("attempt to negate a %s value", v.TypeName())
}

func (vm *VM) length(v runtime.Value) (runtime.Value, error) {
	if v.IsString() {
		return runtime.IntValue(int64(runtime.StringLen(v))), nil
	}
	if v.IsTable() {
		mt := v.Table().GetMetatable()
		if mt != nil {
			mm := mt.RawGet(runtime.StringValue("__len"))
			if !mm.IsNil() {
				args := [1]runtime.Value{v}
				results, err := vm.callValue(mm, args[:])
				if err != nil {
					return runtime.NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return runtime.IntValue(0), nil
			}
		}
		return runtime.IntValue(int64(v.Table().Len())), nil
	}
	return runtime.NilValue(), fmt.Errorf("attempt to get length of a %s value", v.TypeName())
}

func (vm *VM) ConcatValues(values []runtime.Value) (runtime.Value, error) {
	if len(values) == 0 {
		return runtime.StringValue(""), nil
	}
	allNative := true
	for _, v := range values {
		if !(v.IsString() || v.IsNumber()) {
			allNative = false
			break
		}
	}
	if allNative {
		result := values[0]
		if len(values) == 1 {
			s, _ := runtime.ConcatOperandString(result)
			return runtime.StringValue(s), nil
		}
		for i := 1; i < len(values); i++ {
			result = runtime.LazyStringValue(result, values[i])
		}
		return result, nil
	}

	result := values[len(values)-1]
	for i := len(values) - 2; i >= 0; i-- {
		var err error
		result, err = vm.concatPair(values[i], result)
		if err != nil {
			return runtime.NilValue(), err
		}
	}
	return result, nil
}

func (vm *VM) concatPair(a, b runtime.Value) (runtime.Value, error) {
	if (a.IsString() || a.IsNumber()) && (b.IsString() || b.IsNumber()) {
		return runtime.LazyStringValue(a, b), nil
	}
	mm, err := vm.getMetamethod(a, b, "__concat")
	if err == nil && !mm.IsNil() {
		args := [2]runtime.Value{a, b}
		results, err := vm.callValue(mm, args[:])
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return runtime.NilValue(), nil
	}
	if !(a.IsString() || a.IsNumber()) {
		return runtime.NilValue(), fmt.Errorf("attempt to concatenate a %s value", a.TypeName())
	}
	return runtime.NilValue(), fmt.Errorf("attempt to concatenate a %s value", b.TypeName())
}

func (vm *VM) valueEqual(a, b runtime.Value) (bool, error) {
	if a.IsTable() && b.IsTable() {
		if a.Table() == b.Table() {
			return true, nil
		}
		mm, err := vm.getMetamethod(a, b, "__eq")
		if err == nil && !mm.IsNil() {
			args := [2]runtime.Value{a, b}
			results, err := vm.callValue(mm, args[:])
			if err != nil {
				return false, err
			}
			if len(results) > 0 {
				return results[0].Truthy(), nil
			}
			return false, nil
		}
		return false, nil
	}
	return a.Equal(b), nil
}

func (vm *VM) valueLessThan(a, b runtime.Value) (bool, error) {
	if lt, ok := a.LessThan(b); ok {
		return lt, nil
	}
	mm, err := vm.getMetamethod(a, b, "__lt")
	if err == nil && !mm.IsNil() {
		args := [2]runtime.Value{a, b}
		results, err := vm.callValue(mm, args[:])
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	return false, fmt.Errorf("attempt to compare %s with %s", a.TypeName(), b.TypeName())
}

func (vm *VM) valueLessEqual(a, b runtime.Value) (bool, error) {
	if less, ok := a.LessThan(b); ok {
		return less || a.Equal(b), nil
	}
	mm, err := vm.getMetamethod(a, b, "__le")
	if err == nil && !mm.IsNil() {
		args := [2]runtime.Value{a, b}
		results, err := vm.callValue(mm, args[:])
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	return false, fmt.Errorf("attempt to compare %s with %s", a.TypeName(), b.TypeName())
}

func (vm *VM) getMetamethod(a, b runtime.Value, name string) (runtime.Value, error) {
	key := runtime.StringValue(name)
	if a.IsTable() {
		mt := a.Table().GetMetatable()
		if mt != nil {
			mm := mt.RawGet(key)
			if !mm.IsNil() {
				return mm, nil
			}
		}
	}
	if b.IsTable() {
		mt := b.Table().GetMetatable()
		if mt != nil {
			mm := mt.RawGet(key)
			if !mm.IsNil() {
				return mm, nil
			}
		}
	}
	return runtime.NilValue(), fmt.Errorf("no metamethod %s", name)
}

// markGlobalTablesConcurrent enables mutex on all top-level global tables.
// Called once when the first OP_GO goroutine is spawned.
func (vm *VM) markGlobalTablesConcurrent() {
	vm.globalsMu.Lock()
	for _, v := range vm.globals {
		if v.IsTable() {
			v.Table().SetConcurrent(true)
		}
	}
	vm.globalsMu.Unlock()
}

// ---- Upvalue management ----

// RegisterOpenUpvalue adds an existing open upvalue to the tracked list so that
// closeUpvalues will close it when the enclosing function returns.
// Used by the baseline JIT's CLOSURE handler.
func (vm *VM) RegisterOpenUpvalue(uv *Upvalue) {
	// Don't add duplicates.
	for _, existing := range vm.openUpvals {
		if existing == uv {
			return
		}
	}
	vm.openUpvals = append(vm.openUpvals, uv)
}

// FindOrCreateUpvalue returns the VM-tracked open upvalue for regIdx.
// JIT op-exit closure creation uses this to mirror interpreter OP_CLOSURE
// semantics and avoid accumulating duplicate open upvalues for loop locals.
func (vm *VM) FindOrCreateUpvalue(regIdx int) *Upvalue {
	return vm.findOrCreateUpvalue(regIdx)
}

// CloseUpvalues closes all open upvalues at or above fromReg.
// Used by the baseline JIT for OP_CLOSE handling.
func (vm *VM) CloseUpvalues(fromReg int) {
	vm.closeUpvalues(fromReg)
}

func (vm *VM) findOrCreateUpvalue(regIdx int) *Upvalue {
	for _, uv := range vm.openUpvals {
		if uv.regIdx == regIdx {
			return uv
		}
	}
	uv := NewOpenUpvalue(&vm.regs[regIdx], regIdx)
	vm.openUpvals = append(vm.openUpvals, uv)
	return uv
}

func (vm *VM) closeUpvalues(fromReg int) {
	if len(vm.openUpvals) == 0 {
		return
	}
	kept := vm.openUpvals[:0]
	for _, uv := range vm.openUpvals {
		if uv.regIdx >= fromReg {
			uv.Close()
		} else {
			kept = append(kept, uv)
		}
	}
	vm.openUpvals = kept
}

// ---- Helpers ----

func floatIsExactInt(f float64) bool {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return false
	}
	return f == math.Trunc(f) && f >= math.MinInt64 && f <= math.MaxInt64
}

func init() {
}
