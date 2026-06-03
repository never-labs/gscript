package vm

// GC root scanning, split verbatim from vm.go.

import (
	"github.com/never-labs/leia/internal/runtime"
	"unsafe"
)

func init() {
	runtime.RegisterVMClosureRootScanner(scanVMClosureValueRoots)
	runtime.RegisterVMCoroutineRootScanner(scanVMCoroutineValueRoots)
}

func (vm *VM) ScanGCRoots(visitor func(unsafe.Pointer)) {
	seen := make(map[uintptr]struct{}, 256)
	seenProtos := make(map[*FuncProto]struct{}, 32)
	vm.scanGCRoots(visitor, seen, seenProtos)
}

func (vm *VM) scanGCRoots(visitor func(unsafe.Pointer), seen map[uintptr]struct{}, seenProtos map[*FuncProto]struct{}) {
	if vm == nil {
		return
	}
	// Scan active register windows. Scanning the whole backing slice keeps stale
	// temporaries alive for long-running scripts; vm.top is normalized by every
	// fixed-result call path, and frame windows cover active bytecode frames.
	for i, limit := 0, vm.gcRegisterScanLimit(); i < limit; i++ {
		runtime.ScanValueRoots(vm.regs[i], visitor, seen)
	}

	// Scan globals array.
	for _, v := range vm.globalArray {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	for _, v := range vm.globals {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	for _, v := range vm.globalOverrides {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	for _, v := range vm.globalOverrideIdx {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	runtime.ScanValueRoots(vm.globalOverrideVal, visitor, seen)
	runtime.ScanValueRoots(vm.debugHook, visitor, seen)
	runtime.ScanValueRoots(vm.debugSink, visitor, seen)

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
		for _, d := range f.defers {
			runtime.ScanValueRoots(d.fn, visitor, seen)
			for _, arg := range d.args {
				runtime.ScanValueRoots(arg, visitor, seen)
			}
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
	for _, v := range vm.typeNameValues {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	runtime.ScanValueRoots(vm.unknownTypeName, visitor, seen)
	scanVMCoroutineRoots(vm.currentCoroutine, visitor, seen)

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

func (vm *VM) gcRegisterScanLimit() int {
	limit := vm.top
	for i := 0; i < vm.frameCount; i++ {
		f := &vm.frames[i]
		frameLimit := f.base
		if f.closure != nil && f.closure.Proto != nil {
			slots := f.closure.Proto.MaxStack
			if f.closure.Proto.NumParams > slots {
				slots = f.closure.Proto.NumParams
			}
			frameLimit += slots + 1
		}
		if frameLimit > limit {
			limit = frameLimit
		}
	}
	if limit < 0 {
		return 0
	}
	if limit > len(vm.regs) {
		return len(vm.regs)
	}
	return limit
}

func scanVMClosureValueRoots(p unsafe.Pointer, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if p == nil {
		return
	}
	addr := uintptr(p)
	if _, already := seen[addr]; already {
		return
	}
	seen[addr] = struct{}{}
	cl := (*Closure)(p)
	if cl == nil {
		return
	}
	for _, uv := range cl.Upvalues {
		if uv != nil {
			runtime.ScanValueRoots(uv.Get(), visitor, seen)
		}
	}
	scanProtoRoots(cl.Proto, visitor, seen, make(map[*FuncProto]struct{}, 8))
}

func scanVMCoroutineValueRoots(p unsafe.Pointer, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if p == nil {
		return
	}
	scanVMCoroutineRoots((*VMCoroutine)(p), visitor, seen)
}

func scanVMCoroutineRoots(co *VMCoroutine, visitor func(unsafe.Pointer), seen map[uintptr]struct{}) {
	if co == nil {
		return
	}
	addr := uintptr(unsafe.Pointer(co))
	if _, already := seen[addr]; already {
		return
	}
	seen[addr] = struct{}{}
	visitor(unsafe.Pointer(co))
	if co.closure != nil {
		scanVMClosureValueRoots(unsafe.Pointer(co.closure), visitor, seen)
	}
	if co.goFunction != nil {
		visitor(unsafe.Pointer(co.goFunction))
	}
	if co.pooledFixedRecord != nil {
		runtime.ScanFixedRecordRootsExported(co.pooledFixedRecord, visitor, seen)
	}
	for _, v := range co.yieldResult.values {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	if co.hasJITContinuation {
		scanProtoRoots(co.jitContinuation.Proto, visitor, seen, make(map[*FuncProto]struct{}, 4))
	}
	if co.vm != nil {
		co.vm.scanGCRoots(visitor, seen, make(map[*FuncProto]struct{}, 8))
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
