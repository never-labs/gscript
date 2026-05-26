package vm

// GC root scanning, split verbatim from vm.go.

import (
	"github.com/gscript/gscript/internal/runtime"
	"unsafe"
)

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
	for _, v := range vm.typeNameValues {
		runtime.ScanValueRoots(v, visitor, seen)
	}
	runtime.ScanValueRoots(vm.unknownTypeName, visitor, seen)

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
