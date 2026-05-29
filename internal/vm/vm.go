package vm

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"unsafe"

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

type methodJITEngineWithIsolatedChild interface {
	NewIsolatedChildEngine(child *VM) MethodJITEngine
}

// VM is the bytecode virtual machine.
type VM struct {
	regs                 []runtime.Value          // register file (shared across frames via base offset)
	frames               []CallFrame              // call stack
	frameCount           int                      // current number of active frames
	globals              map[string]runtime.Value // legacy map (kept for interop)
	globalArray          []runtime.Value          // indexed globals (fast path)
	globalIndex          map[string]int           // name → index in globalArray
	globalValueEpoch     []uint64                 // per-index value epoch for named global dependencies
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
	typeNameValues       [runtime.TypeChannel + 1]runtime.Value
	unknownTypeName      runtime.Value
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
	maxSteps             int64 // <=0 means unlimited
	steps                int64
	maxNativeCalls       int64 // <=0 means unlimited
	nativeCalls          int64
	maxCallDepth         int64 // <=0 means the VM default
	maxGoroutines        int64 // <=0 means unlimited
	activeGoroutines     *atomic.Int64
	maxChannelCap        int64 // <=0 means unlimited
	maxHostResult        int64 // <=0 means unlimited
	ctx                  context.Context
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

// SetMaxSteps sets the maximum number of bytecode instruction checkpoints.
// A non-positive value disables the limit. The counter resets for each Execute.
func (vm *VM) SetMaxSteps(max int64) {
	vm.maxSteps = max
	vm.steps = 0
}

// SetMaxNativeCalls sets the maximum number of native Go calls made by one
// Execute or host CallValue. A non-positive value disables the limit.
func (vm *VM) SetMaxNativeCalls(max int64) {
	vm.maxNativeCalls = max
	vm.nativeCalls = 0
}

// SetMaxCallDepth sets the maximum number of active bytecode call frames. A
// non-positive value restores the VM default.
func (vm *VM) SetMaxCallDepth(max int64) {
	vm.maxCallDepth = max
}

// SetMaxGoroutines sets the maximum number of active script-created
// goroutines. A non-positive value disables the limit.
func (vm *VM) SetMaxGoroutines(max int64) {
	vm.maxGoroutines = max
}

// SetMaxChannelCapacity sets the maximum buffer capacity for script-created
// channels. A non-positive value disables the limit.
func (vm *VM) SetMaxChannelCapacity(max int64) {
	vm.maxChannelCap = max
}

// SetMaxHostResultBytes sets the maximum byte size of strings returned from a
// single native Go call. A non-positive value disables the limit.
func (vm *VM) SetMaxHostResultBytes(max int64) {
	vm.maxHostResult = max
}

// SetContext installs a host cancellation context checked at bytecode
// instruction checkpoints. A nil context disables cancellation polling.
func (vm *VM) SetContext(ctx context.Context) {
	vm.ctx = ctx
}

func (vm *VM) resetExecutionBudgets() {
	vm.steps = 0
	vm.nativeCalls = 0
}

func (vm *VM) checkStepBudget() error {
	if vm.ctx != nil {
		select {
		case <-vm.ctx.Done():
			return vm.ctx.Err()
		default:
		}
	}
	if vm.maxSteps > 0 {
		vm.steps++
		if vm.steps > vm.maxSteps {
			return fmt.Errorf("execution step limit exceeded (%d)", vm.maxSteps)
		}
	}
	return nil
}

func (vm *VM) checkNativeCallBudget() error {
	if vm.maxNativeCalls <= 0 {
		return nil
	}
	vm.nativeCalls++
	if vm.nativeCalls > vm.maxNativeCalls {
		return fmt.Errorf("native call limit exceeded (%d)", vm.maxNativeCalls)
	}
	return nil
}

func (vm *VM) recordFastNativeCall(gf *runtime.GoFunction) error {
	if err := vm.checkNativeCallBudget(); err != nil {
		return err
	}
	runtime.RecordRuntimePathNativeCallFastFor(gf)
	return nil
}

func (vm *VM) callDepthLimit() int {
	if vm.maxCallDepth > 0 && vm.maxCallDepth < int64(maxCallDepth) {
		return int(vm.maxCallDepth)
	}
	return maxCallDepth
}

func (vm *VM) reserveGoroutineBudget() error {
	if vm.maxGoroutines <= 0 {
		return nil
	}
	if vm.activeGoroutines == nil {
		vm.activeGoroutines = &atomic.Int64{}
	}
	for {
		current := vm.activeGoroutines.Load()
		if current >= vm.maxGoroutines {
			return fmt.Errorf("goroutine limit exceeded (%d)", vm.maxGoroutines)
		}
		if vm.activeGoroutines.CompareAndSwap(current, current+1) {
			return nil
		}
	}
}

func (vm *VM) releaseGoroutineBudget() {
	if vm.maxGoroutines <= 0 || vm.activeGoroutines == nil {
		return
	}
	vm.activeGoroutines.Add(-1)
}

func (vm *VM) checkChannelCapacityBudget(capacity int) error {
	if vm.maxChannelCap <= 0 || int64(capacity) <= vm.maxChannelCap {
		return nil
	}
	return fmt.Errorf("channel capacity limit exceeded (%d)", vm.maxChannelCap)
}

func (vm *VM) checkHostResultBudget(values ...runtime.Value) error {
	return runtime.CheckHostResultBytes(vm.maxHostResult, values...)
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
	return vm.PushFrameWithVarargs(cl, base, nil)
}

// PushFrameWithVarargs pushes a minimal call frame and attaches the callee's
// vararg tail. Used by JIT call paths that still need VM-owned vararg state.
func (vm *VM) PushFrameWithVarargs(cl *Closure, base int, varargs []runtime.Value) bool {
	if !vm.ensureFrameSlot() {
		return false
	}
	frame := &vm.frames[vm.frameCount]
	frame.closure = cl
	frame.pc = 0
	frame.base = base
	frame.numResults = -1
	setFrameVarargs(frame, varargs)
	frame.callSitePC = -1
	frame.defers = nil
	vm.frameCount++
	return true
}

// PushFrameWithBorrowedVarargs pushes a frame whose varargs borrow a caller
// register window. It is intended for synchronous JIT fast calls where the
// caller frame remains alive until this frame is popped. Generic interpreter
// calls still copy varargs so vararg state is independent of register reuse.
func (vm *VM) PushFrameWithBorrowedVarargs(cl *Closure, base int, varargs []runtime.Value) bool {
	if !vm.ensureFrameSlot() {
		return false
	}
	frame := &vm.frames[vm.frameCount]
	frame.closure = cl
	frame.pc = 0
	frame.base = base
	frame.numResults = -1
	frame.varargs = varargs
	frame.callSitePC = -1
	frame.defers = nil
	vm.frameCount++
	return true
}

func (vm *VM) ensureFrameSlot() bool {
	limit := vm.callDepthLimit()
	if vm.frameCount >= limit {
		return false
	}
	if vm.frameCount < len(vm.frames) {
		return true
	}
	newLen := len(vm.frames) * 2
	if newLen == 0 {
		newLen = initialCallFrameCapacity
	}
	if newLen <= vm.frameCount {
		newLen = vm.frameCount + 1
	}
	if newLen > limit {
		newLen = limit
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

func (vm *VM) GlobalValueVersion(name string) (uint64, bool) {
	if vm == nil || vm.globalOverrides != nil {
		return 0, false
	}
	if vm.noGlobalLock {
		idx, ok := vm.globalIndex[name]
		if !ok || idx < 0 || idx >= len(vm.globalValueEpoch) {
			return 0, true
		}
		return vm.globalValueEpoch[idx], true
	}
	vm.globalsMu.RLock()
	idx, ok := vm.globalIndex[name]
	if !ok || idx < 0 || idx >= len(vm.globalValueEpoch) {
		vm.globalsMu.RUnlock()
		return 0, true
	}
	epoch := vm.globalValueEpoch[idx]
	vm.globalsMu.RUnlock()
	return epoch, true
}

func (vm *VM) GlobalValueVersionByIndex(idx int) (uint64, bool) {
	if vm == nil || vm.globalOverrides != nil || idx < 0 {
		return 0, false
	}
	if vm.noGlobalLock {
		if idx >= len(vm.globalValueEpoch) {
			return 0, true
		}
		return vm.globalValueEpoch[idx], true
	}
	vm.globalsMu.RLock()
	if idx >= len(vm.globalValueEpoch) {
		vm.globalsMu.RUnlock()
		return 0, true
	}
	epoch := vm.globalValueEpoch[idx]
	vm.globalsMu.RUnlock()
	return epoch, true
}

func (vm *VM) ensureGlobalValueEpochLen() {
	for len(vm.globalValueEpoch) < len(vm.globalArray) {
		vm.globalValueEpoch = append(vm.globalValueEpoch, 0)
	}
}

func (vm *VM) bumpGlobalValueEpoch(idx int) {
	vm.globalValueVer++
	if idx < 0 {
		return
	}
	vm.ensureGlobalValueEpochLen()
	if idx < len(vm.globalValueEpoch) {
		vm.globalValueEpoch[idx]++
	}
}

func (vm *VM) initTypeNameValues() {
	vm.typeNameValues[runtime.TypeNil] = runtime.StringValue("nil")
	vm.typeNameValues[runtime.TypeBool] = runtime.StringValue("boolean")
	vm.typeNameValues[runtime.TypeInt] = runtime.StringValue("number")
	vm.typeNameValues[runtime.TypeFloat] = runtime.StringValue("number")
	vm.typeNameValues[runtime.TypeString] = runtime.StringValue("string")
	vm.typeNameValues[runtime.TypeTable] = runtime.StringValue("table")
	vm.typeNameValues[runtime.TypeFunction] = runtime.StringValue("function")
	vm.typeNameValues[runtime.TypeCoroutine] = runtime.StringValue("coroutine")
	vm.typeNameValues[runtime.TypeChannel] = runtime.StringValue("channel")
	vm.unknownTypeName = runtime.StringValue("unknown")
}

func (vm *VM) typeNameValue(v runtime.Value) runtime.Value {
	if vm == nil {
		return runtime.StringValue(v.TypeName())
	}
	t := v.Type()
	if int(t) < len(vm.typeNameValues) {
		if tv := vm.typeNameValues[t]; !tv.IsNil() {
			return tv
		}
	}
	if !vm.unknownTypeName.IsNil() {
		return vm.unknownTypeName
	}
	return runtime.StringValue("unknown")
}

// SetGlobal writes a global variable with proper locking.
func (vm *VM) SetGlobal(name string, val runtime.Value) {
	if vm.noGlobalLock {
		if idx, ok := vm.globalIndex[name]; ok {
			vm.globalArray[idx] = val
			vm.globals[name] = val
			vm.bumpGlobalValueEpoch(idx)
		} else {
			idx = len(vm.globalArray)
			vm.globalArray = append(vm.globalArray, val)
			vm.globalValueEpoch = append(vm.globalValueEpoch, 1)
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
		vm.bumpGlobalValueEpoch(idx)
	} else {
		idx = len(vm.globalArray)
		vm.globalArray = append(vm.globalArray, val)
		vm.globalValueEpoch = append(vm.globalValueEpoch, 1)
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
			vm.bumpGlobalValueEpoch(idx)
		}
		return
	}
	vm.globalsMu.Lock()
	if idx, ok := vm.globalIndex[name]; ok {
		vm.globalArray[idx] = runtime.NilValue()
		delete(vm.globalIndex, name)
		delete(vm.globals, name)
		vm.globalVer++
		vm.bumpGlobalValueEpoch(idx)
	}
	vm.globalsMu.Unlock()
}

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
			vm.bumpGlobalValueEpoch(idx)
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
		vm.bumpGlobalValueEpoch(idx)
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
	vm.globalValueEpoch = append(vm.globalValueEpoch, 0)
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
	ge := make([]uint64, len(ga))

	v := &VM{
		regs:               runtime.MakeNilSlice(1024),
		frames:             make([]CallFrame, initialCallFrameCapacity),
		globals:            globals,
		globalArray:        ga,
		globalIndex:        gi,
		globalValueEpoch:   ge,
		globalOverrideFast: -1,
		globalsMu:          &sync.RWMutex{},
		noGlobalLock:       true, // single-threaded by default
		activeGoroutines:   &atomic.Int64{},
	}
	v.initTypeNameValues()
	v.RegisterCoroutineLib()
	v.RegisterProtectedCallLib()
	v.RegisterTestkitLib()
	v.RegisterTypeLib()
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
	v.RegisterSyncLib()
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
func newChildVM(parent *VM, co *VMCoroutine) *VM {
	child := &VM{
		regs:               runtime.MakeNilSlice(1024),
		frames:             make([]CallFrame, initialCallFrameCapacity),
		globals:            parent.globals,
		globalArray:        parent.globalArray,
		globalIndex:        parent.globalIndex,
		globalValueEpoch:   parent.globalValueEpoch,
		globalVer:          parent.globalVer,
		globalValueVer:     parent.globalValueVer,
		globalOverrideFast: -1,
		globalsMu:          parent.globalsMu,
		noGlobalLock:       false, // shared globals, must lock
		stringMeta:         parent.stringMeta,
		currentCoroutine:   co,
		coroutineStats:     parent.coroutineStats,
		maxSteps:           parent.maxSteps,
		maxNativeCalls:     parent.maxNativeCalls,
		maxCallDepth:       parent.maxCallDepth,
		maxGoroutines:      parent.maxGoroutines,
		activeGoroutines:   parent.activeGoroutines,
		maxChannelCap:      parent.maxChannelCap,
		maxHostResult:      parent.maxHostResult,
	}
	child.initTypeNameValues()
	child.setGlobalOverride("coroutine", runtime.TableValue(child.newCoroutineLib()))
	child.setGlobalOverride("pcall", runtime.FunctionValue(child.newPCallFunction()))
	child.setGlobalOverride("xpcall", runtime.FunctionValue(child.newXPCallFunction()))
	child.setGlobalOverride("type", runtime.FunctionValue(child.newTypeFunction()))
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
	if parent.globalsMu != nil && !parent.noGlobalLock {
		parent.globalsMu.RLock()
		defer parent.globalsMu.RUnlock()
	}

	// Copy both globalArray and globalIndex for full isolation.
	ga := make([]runtime.Value, len(parent.globalArray))
	copy(ga, parent.globalArray)
	ge := make([]uint64, len(parent.globalValueEpoch))
	copy(ge, parent.globalValueEpoch)

	gi := make(map[string]int, len(parent.globalIndex))
	for k, v := range parent.globalIndex {
		gi[k] = v
	}

	childGlobals := make(map[string]runtime.Value, len(gi))
	for name, idx := range gi {
		if idx < 0 || idx >= len(ga) {
			continue
		}
		childGlobals[name] = ga[idx]
	}
	if pkgVal, ok := childGlobals["package"]; ok && pkgVal.IsTable() {
		pkgCopy := clonePackageTableForChild(pkgVal.Table())
		pkgCopyVal := runtime.TableValue(pkgCopy)
		childGlobals["package"] = pkgCopyVal
		if idx, ok := gi["package"]; ok && idx >= 0 && idx < len(ga) {
			ga[idx] = pkgCopyVal
		}
	}
	for _, name := range []string{"table", "sort", "string", "testkit"} {
		if val, ok := childGlobals[name]; ok && val.IsTable() {
			copyVal := runtime.TableValue(cloneTableForChild(val.Table()))
			childGlobals[name] = copyVal
			if idx, ok := gi[name]; ok && idx >= 0 && idx < len(ga) {
				ga[idx] = copyVal
			}
		}
	}

	child := &VM{
		regs:               runtime.MakeNilSlice(1024),
		frames:             make([]CallFrame, initialCallFrameCapacity),
		globals:            childGlobals,
		globalArray:        ga,
		globalIndex:        gi,
		globalValueEpoch:   ge,
		globalVer:          parent.globalVer,
		globalValueVer:     parent.globalValueVer,
		globalOverrideFast: -1,
		globalsMu:          &sync.RWMutex{},
		noGlobalLock:       true, // own copy, fully lock-free
		stringMeta:         parent.stringMeta,
		coroutineStats:     parent.coroutineStats,
		debugHook:          parent.debugHook,
		debugOpts:          parent.debugOpts,
		debugSink:          parent.debugSink,
		maxSteps:           parent.maxSteps,
		maxNativeCalls:     parent.maxNativeCalls,
		maxCallDepth:       parent.maxCallDepth,
		maxGoroutines:      parent.maxGoroutines,
		activeGoroutines:   parent.activeGoroutines,
		maxChannelCap:      parent.maxChannelCap,
		maxHostResult:      parent.maxHostResult,
	}
	child.initTypeNameValues()
	child.RegisterCoroutineLib()
	child.RegisterProtectedCallLib()
	child.RegisterTestkitLib()
	child.RegisterTypeLib()
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
	child.RegisterSyncLib()
	child.RegisterScriptLib()
	child.RegisterLoaderLib()
	child.scriptDir = parent.scriptDir
	runtime.RegisterVM(child)
	child.attachIsolatedChildJIT(parent)
	return child
}

func (vm *VM) attachIsolatedChildJIT(parent *VM) {
	if vm == nil || parent == nil || parent.methodJIT == nil {
		return
	}
	factory, ok := parent.methodJIT.(methodJITEngineWithIsolatedChild)
	if !ok {
		return
	}
	childEngine := factory.NewIsolatedChildEngine(vm)
	if childEngine != nil {
		vm.SetMethodJIT(childEngine)
	}
}

func (vm *VM) launchSyncTask(fn runtime.Value, args []runtime.Value, done func(error)) {
	taskArgs := append([]runtime.Value(nil), args...)
	go func() {
		goVM := newIsolatedChildVM(vm)
		defer goVM.Close()
		var err error
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
			if done != nil {
				done(err)
			}
		}()
		if cl, ok := closureFromValue(fn); ok {
			_, err = goVM.call(cl, taskArgs, 0, 0)
			return
		}
		if gf := fn.GoFunction(); gf != nil {
			if budgetErr := goVM.checkNativeCallBudget(); budgetErr != nil {
				err = budgetErr
				return
			}
			_, err = gf.Fn(taskArgs)
			return
		}
		err = fmt.Errorf("attempt to call a %s value", fn.TypeName())
	}()
}

func clonePackageTableForChild(parent *runtime.Table) *runtime.Table {
	pkg := runtime.NewTable()
	loaded := runtime.NewTable()
	if parent != nil {
		parentLoaded := parent.RawGetString("loaded")
		if parentLoaded.IsTable() {
			src := parentLoaded.Table()
			for _, name := range runtime.StdlibModuleNames() {
				if v := src.RawGetString(name); !v.IsNil() {
					loaded.RawSetString(name, v)
				}
			}
		}
		if path := parent.RawGetString("path"); !path.IsNil() {
			pkg.RawSetString("path", path)
		}
	}
	pkg.RawSetString("loaded", runtime.TableValue(loaded))
	return pkg
}

func cloneTableForChild(parent *runtime.Table) *runtime.Table {
	out := runtime.NewTable()
	if parent == nil {
		return out
	}
	for _, key := range parent.PairsKeysSnapshot() {
		out.RawSet(key, parent.RawGet(key))
	}
	return out
}

// registerChannelBuiltins adds channel-related builtins to globals.
func (vm *VM) Execute(proto *FuncProto) ([]runtime.Value, error) {
	cl := &Closure{Proto: proto}
	vm.frameCount = 0
	vm.top = 0
	vm.resetExecutionBudgets()
	return vm.call(cl, nil, 0, 0)
}

// CallValue calls a function value with the given arguments (exported for gscript wrapper).
func (vm *VM) CallValue(fn runtime.Value, args []runtime.Value) ([]runtime.Value, error) {
	vm.resetExecutionBudgets()
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
		return nil, fmt.Errorf("call depth limit exceeded (%d)", vm.callDepthLimit())
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
		if err := vm.checkStepBudget(); err != nil {
			return nil, wrapLineErr(frame, err)
		}
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
				idx := -1
				if cache.index >= 0 && cache.version == vm.globalVer {
					idx = int(cache.index)
					vm.globalArray[idx] = val
				} else {
					idx = vm.resolveGlobalIndex(name)
					cache.index = int32(idx)
					cache.version = vm.globalVer
					vm.globalArray[idx] = val
				}
				vm.globals[name] = val
				vm.bumpGlobalValueEpoch(idx)
			} else {
				// Multi-threaded: locked access, update both map and array
				vm.globalsMu.Lock()
				vm.globals[name] = val
				if idx, ok := vm.globalIndex[name]; ok {
					vm.globalArray[idx] = val
					vm.bumpGlobalValueEpoch(idx)
				} else {
					idx := len(vm.globalArray)
					vm.globalIndex[name] = idx
					vm.globalArray = append(vm.globalArray, val)
					vm.globalValueEpoch = append(vm.globalValueEpoch, 1)
					vm.globalVer++
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
				vm.bumpGlobalValueEpoch(idx)
			} else {
				vm.globalsMu.Lock()
				vm.globals[name] = val
				idx := -1
				if existingIdx, ok := vm.globalIndex[name]; ok {
					idx = existingIdx
					vm.globalArray[idx] = val
				} else {
					idx = len(vm.globalArray)
					vm.globalIndex[name] = idx
					vm.globalArray = append(vm.globalArray, val)
					vm.globalValueEpoch = append(vm.globalValueEpoch, 0)
					vm.globalVer++
				}
				vm.bumpGlobalValueEpoch(idx)
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
			if handled, err := vm.trySoAAffineManyLiteralRuntimeSpecialization(frame, base, frame.pc-1); handled {
				if err != nil {
					return nil, wrapLineErr(frame, err)
				}
				break
			}
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
			if tableVal.IsDenseArray() {
				if idx, ok, err := runtime.DenseArrayIndexFromValue(key, tableVal.DenseArray().Len()); ok || err != nil {
					if err != nil {
						return nil, err
					}
					val, err := tableVal.DenseArray().At(idx)
					if err != nil {
						return nil, err
					}
					vm.regs[base+a] = val
					if frame.closure.Proto.Feedback != nil {
						fb := &frame.closure.Proto.Feedback[frame.pc-1]
						fb.Left.Observe(tableVal.Type())
						fb.Right.Observe(key.Type())
						fb.Result.Observe(val.Type())
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
			if tableVal.IsDenseArray() {
				if idx, ok, err := runtime.DenseArrayIndexFromValue(key, tableVal.DenseArray().Len()); ok || err != nil {
					if err != nil {
						return nil, err
					}
					if err := tableVal.DenseArray().Set(idx, val); err != nil {
						return nil, err
					}
					if frame.closure.Proto.Feedback != nil {
						fb := &frame.closure.Proto.Feedback[frame.pc-1]
						fb.Left.Observe(tableVal.Type())
						fb.Right.Observe(key.Type())
						fb.Result.Observe(val.Type())
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

		case OP_ISNUMBER:
			a := DecodeA(inst)
			bv := vm.regs[base+DecodeB(inst)]
			vm.regs[base+a] = runtime.BoolValue(bv.IsNumber())

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
					if handled, err := vm.tryFuseToStringNumericToIntegerWrapper(frame, base, a, nArgs, c, gf); err != nil {
						return nil, wrapLineErr(frame, err)
					} else if handled {
						observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
						break
					}
					if handled, err := vm.tryFuseStringSubToNumber(frame, base, a, nArgs, c, gf); err != nil {
						return nil, wrapLineErr(frame, err)
					} else if handled {
						observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
						break
					}
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
							return nil, fmt.Errorf("call depth limit exceeded (%d)", vm.callDepthLimit())
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
				if handled, err := vm.tryExecuteNumericToIntegerWrapperCall(cl, base+a, nArgs, c); err != nil {
					return nil, wrapLineErr(frame, err)
				} else if handled {
					observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
					break
				}

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
					return nil, fmt.Errorf("call depth limit exceeded (%d)", vm.callDepthLimit())
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

			// ---- Fast path: callable table with VM __call metamethod ----
			if b != 0 && fnVal.IsTable() {
				if mt := fnVal.Table().GetMetatable(); mt != nil {
					callMM := mt.RawGetString("__call")
					if cl, ok := closureFromValue(callMM); ok {
						proto := cl.Proto
						mmArgs := nArgs + 1
						if callSiteRuntimeSpecializationArity(mmArgs) {
							var local [4]runtime.Value
							var specArgs []runtime.Value
							if mmArgs <= len(local) {
								specArgs = local[:mmArgs]
							} else {
								specArgs = make([]runtime.Value, mmArgs)
							}
							specArgs[0] = fnVal
							srcStart := base + a + 1
							for i := 0; i < nArgs; i++ {
								specArgs[i+1] = vm.regs[srcStart+i]
							}
							handled, results, err := vm.tryRunNonRecursiveTableValueRuntimeSpecialization(cl, specArgs)
							if handled {
								if err != nil {
									return nil, wrapLineErr(frame, err)
								}
								vm.writeCallResults(base+a, c, results)
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								break
							}
						}
						if !proto.IsVarArg || mmArgs <= proto.NumParams {
							newBase := base + frame.closure.Proto.MaxStack
							if vm.top > newBase {
								newBase = vm.top
							}

							needed := newBase + proto.MaxStack + 1
							if needed > len(vm.regs) {
								newRegs := runtime.MakeNilSlice(needed * 2)
								copy(newRegs, vm.regs)
								vm.regs = newRegs
							}

							nParams := proto.NumParams
							if nParams > 0 {
								vm.regs[newBase] = fnVal
							}
							srcStart := base + a + 1
							for i := 1; i < nParams && i <= nArgs; i++ {
								vm.regs[newBase+i] = vm.regs[srcStart+i-1]
							}
							for i := mmArgs; i < nParams; i++ {
								vm.regs[newBase+i] = runtime.NilValue()
							}

							if !vm.ensureFrameSlot() {
								return nil, fmt.Errorf("call depth limit exceeded (%d)", vm.callDepthLimit())
							}
							newFrame := &vm.frames[vm.frameCount]
							newFrame.closure = cl
							newFrame.pc = 0
							newFrame.base = newBase
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

							if vm.methodJIT != nil && proto.MethodJITTier1Callable() && !proto.JITDisabled {
								proto.CallCount++
								if proto.CallCount <= 64 {
									argEnd := newBase + mmArgs
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
								}
							}

							frame = newFrame
							code = proto.Code
							constants = proto.Constants
							base = newBase
							continue
						}
					}
				}
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
					case runtime.NativeKindStdStringFind:
						if gf.NativeData == runtime.StdStringFindIdentityPtr() {
							handled, err := vm.ExecuteStdStringFindCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					case runtime.NativeKindStdStringMatch:
						if gf.NativeData == runtime.StdStringMatchIdentityPtr() {
							handled, err := vm.ExecuteStdStringMatchCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					case runtime.NativeKindStdStringGSub:
						if gf.NativeData == runtime.StdStringGSubIdentityPtr() {
							handled, err := vm.ExecuteStdStringGSubCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					case runtime.NativeKindStdRawGet:
						if gf.NativeData == runtime.StdRawGetIdentityPtr() {
							handled, err := vm.ExecuteStdRawGetCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					case runtime.NativeKindStdNext:
						if gf.NativeData == runtime.StdNextIdentityPtr() {
							handled, err := vm.ExecuteStdNextCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					case runtime.NativeKindStdRawSet:
						if gf.NativeData == runtime.StdRawSetIdentityPtr() {
							handled, err := vm.ExecuteStdRawSetCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					case runtime.NativeKindStdRawLen:
						if gf.NativeData == runtime.StdRawLenIdentityPtr() {
							handled, err := vm.ExecuteStdRawLenCall(base+a, nArgs, c)
							if err != nil {
								return nil, wrapLineErr(frame, err)
							}
							if handled {
								observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
								handledSpecial = true
							}
						}
					case runtime.NativeKindStdType:
						if gf.NativeData == runtime.StdTypeIdentityPtr() {
							handled, err := vm.ExecuteStdTypeCall(base+a, nArgs, c)
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
					if handled, err := vm.executeDirectGoFunctionFastCall(gf, base+a, nArgs, c); handled {
						if err != nil {
							return nil, wrapLineErr(frame, err)
						}
						observeCallResultFixed(callerProto, callPC, vm.regs, base+a, c)
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
			if handled, err := vm.tryUTF8CodepointSumLoopRuntimeSpecialization(frame, base, a); err != nil {
				return nil, err
			} else if handled {
				break
			}
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
					if err := vm.recordFastNativeCall(gf); err != nil {
						return nil, err
					}
					r0, r1, n, err := gf.FastArg1Ret2(vm.regs[base+a+1])
					if err != nil {
						return nil, err
					}
					if err := vm.checkHostResultBudget(r0, r1); err != nil {
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
					if err := vm.recordFastNativeCall(gf); err != nil {
						return nil, err
					}
					r0, r1, n, err := gf.FastArg2Ret2(vm.regs[base+a+1], vm.regs[base+a+2])
					if err != nil {
						return nil, err
					}
					if err := vm.checkHostResultBudget(r0, r1); err != nil {
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
			if err := vm.reserveGoroutineBudget(); err != nil {
				return nil, err
			}
			go func(fn runtime.Value, goArgs []runtime.Value) {
				defer vm.releaseGoroutineBudget()
				goVM := newIsolatedChildVM(vm)
				defer goVM.Close()
				defer func() {
					if r := recover(); r != nil {
						_ = goVM.emitGoroutineError(fmt.Errorf("panic: %v", r), fn)
					}
				}()
				if cl, ok := closureFromValue(fn); ok {
					if _, err := goVM.call(cl, goArgs, 0, 0); err != nil {
						_ = goVM.emitGoroutineError(err, fn)
					}
				} else if gf := fn.GoFunction(); gf != nil {
					if err := goVM.checkNativeCallBudget(); err != nil {
						_ = goVM.emitGoroutineError(err, fn)
						return
					}
					if _, err := gf.Fn(goArgs); err != nil {
						_ = goVM.emitGoroutineError(err, fn)
					}
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
			if err := vm.checkChannelCapacityBudget(capacity); err != nil {
				return nil, err
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

		case OP_RECVOK:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cc := DecodeC(inst)
			chVal := vm.regs[base+b]
			if !chVal.IsChannel() {
				return nil, fmt.Errorf("receive from non-channel value (got %s)", chVal.TypeName())
			}
			val, ok := chVal.Channel().Recv()
			vm.regs[base+a] = val
			vm.regs[base+cc] = runtime.BoolValue(ok)

		case OP_TRYSEND:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cc := DecodeC(inst)
			chVal := vm.regs[base+a]
			if !chVal.IsChannel() {
				return nil, fmt.Errorf("send on non-channel value (got %s)", chVal.TypeName())
			}
			ok, err := chVal.Channel().TrySend(vm.regs[base+b])
			if err != nil {
				return nil, err
			}
			vm.regs[base+cc] = runtime.BoolValue(ok)

		case OP_TRYRECV:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cc := DecodeC(inst)
			chVal := vm.regs[base+b]
			if !chVal.IsChannel() {
				return nil, fmt.Errorf("receive from non-channel value (got %s)", chVal.TypeName())
			}
			val, ok := chVal.Channel().TryRecv()
			vm.regs[base+a] = val
			vm.regs[base+cc] = runtime.BoolValue(ok)

		case OP_TRYRECVOK:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cc := DecodeC(inst)
			chVal := vm.regs[base+b]
			if !chVal.IsChannel() {
				return nil, fmt.Errorf("receive from non-channel value (got %s)", chVal.TypeName())
			}
			val, ready, recvOK := chVal.Channel().TryRecvOK()
			vm.regs[base+a] = val
			vm.regs[base+a+1] = runtime.BoolValue(ready)
			vm.regs[base+cc] = runtime.BoolValue(recvOK)

		case OP_SELECT:
			a := DecodeA(inst)
			b := DecodeB(inst)
			cc := DecodeC(inst)
			if cc <= 0 {
				return nil, fmt.Errorf("select requires at least one case")
			}
			cases := make([]runtime.ChannelSelectCase, cc)
			for i := 0; i < cc; i++ {
				modeVal := vm.regs[base+b+i*3]
				chVal := vm.regs[base+b+i*3+1]
				val := vm.regs[base+b+i*3+2]
				if !modeVal.IsInt() {
					return nil, fmt.Errorf("select case mode is not integer")
				}
				if !chVal.IsChannel() {
					return nil, fmt.Errorf("select case uses non-channel value (got %s)", chVal.TypeName())
				}
				cases[i] = runtime.ChannelSelectCase{
					Kind:    runtime.ChannelSelectKind(modeVal.Int()),
					Channel: chVal.Channel(),
					Value:   val,
				}
			}
			chosen, val, recvOK, err := runtime.ChannelSelect(cases)
			if err != nil {
				return nil, err
			}
			vm.regs[base+a] = runtime.IntValue(int64(chosen + 1))
			vm.regs[base+a+1] = val
			vm.regs[base+a+2] = runtime.BoolValue(recvOK)

		default:
			return nil, fmt.Errorf("unhandled opcode %d (%s)", op, OpName(op))
		}
	}
}

func init() {
}
