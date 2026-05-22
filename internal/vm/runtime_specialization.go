package vm

import "github.com/gscript/gscript/internal/runtime"

const (
	runtimeSpecializationRawIntNested = iota
	runtimeSpecializationCount
)

const (
	driverLoopRuntimeSpecializationGenericRecordArrayLoop = iota
	driverLoopRuntimeSpecializationCount
)

type runtimeSpecializationProtoCache struct {
	fingerprint wholeCallKernelFingerprint
	recognized  uint64
}

// RuntimeSpecialization is the shared recognizer metadata for guarded runtime
// specializations. A specialization may still perform runtime dependency
// checks before handling a call.
type RuntimeSpecialization struct {
	Info      KernelInfo
	Recognize func(*FuncProto) bool
}

// WholeCallValueSpecialization handles OP_CALL sites that return values.
type WholeCallValueSpecialization struct {
	RuntimeSpecialization
	Run wholeCallValueKernelRunner
}

type driverLoopRuntimeSpecializationRunner func(*VM, *CallFrame, int, []uint32, []runtime.Value, int, int) (bool, error)

// DriverLoopRuntimeSpecialization handles OP_FORPREP sites whose admission
// depends on runtime values in addition to structural bytecode shape.
type DriverLoopRuntimeSpecialization struct {
	Info      KernelInfo
	Recognize func(*FuncProto, map[string]*FuncProto) bool
	Run       driverLoopRuntimeSpecializationRunner
}

var wholeCallValueRuntimeSpecializationRegistry = [runtimeSpecializationCount]WholeCallValueSpecialization{
	{
		RuntimeSpecialization: RuntimeSpecialization{
			Info: KernelInfo{
				Name:          "nested_int_recurrence",
				Route:         KernelRouteWholeCallValue,
				Arity:         2,
				Results:       kernelWholeCallSingleResultCount,
				TieringPolicy: kernelTieringStructural,
			},
			Recognize: IsRawIntNestedKernelProto,
		},
		Run: (*VM).runRawIntNestedValueKernel,
	},
}

var driverLoopRuntimeSpecializationRegistry = [driverLoopRuntimeSpecializationCount]DriverLoopRuntimeSpecialization{
	{
		Info: KernelInfo{
			Name:          "generic_record_array_loop",
			Route:         KernelRouteDriverLoop,
			Arity:         kernelUnknownDriverLoopArity,
			Results:       kernelUnknownDriverLoopResultCount,
			TieringPolicy: kernelTieringStructural,
		},
		Recognize: HasGenericRecordArrayDriverLoopKernel,
		Run:       (*VM).tryGenericRecordArrayForLoopKernel,
	},
}

func (vm *VM) tryRunWholeCallValueRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if cl == nil || cl.Proto == nil {
		return false, nil, nil
	}
	if !mayHaveWholeCallValueRuntimeSpecializationCandidate(cl.Proto, len(args)) {
		return false, nil, nil
	}
	recognized := cachedRuntimeSpecializationBits(cl.Proto)
	if recognized == 0 {
		return false, nil, nil
	}
	for i, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if recognized&(uint64(1)<<uint(i)) == 0 || entry.Run == nil {
			continue
		}
		handled, results, err := entry.Run(vm, cl, args)
		if handled || err != nil {
			if handled {
				runtime.RecordRuntimePathStructuralKernelHit(string(entry.Info.Route), entry.Info.Name)
			}
			return handled, results, err
		}
	}
	return false, nil, nil
}

func (vm *VM) tryRunDriverLoopRuntimeSpecialization(frame *CallFrame, base int, code []uint32, constants []runtime.Value, a int, sbx int) (bool, error) {
	if frame == nil || frame.closure == nil || frame.closure.Proto == nil {
		return false, nil
	}
	for _, entry := range driverLoopRuntimeSpecializationRegistry {
		if entry.Info.Name == "" || entry.Run == nil {
			continue
		}
		handled, err := entry.Run(vm, frame, base, code, constants, a, sbx)
		if handled || err != nil {
			if handled {
				runtime.RecordRuntimePathStructuralKernelHit(string(entry.Info.Route), entry.Info.Name)
			}
			return handled, err
		}
	}
	return false, nil
}

func mayHaveWholeCallValueRuntimeSpecializationCandidate(proto *FuncProto, argc int) bool {
	if proto == nil || proto.IsVarArg {
		return false
	}
	for _, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if entry.Info.Arity == argc && proto.NumParams == argc {
			return true
		}
	}
	return false
}

func recognizedRuntimeSpecializationBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	return runtimeSpecializationCacheForProto(proto).recognized
}

func cachedRuntimeSpecializationBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	if cache := proto.RuntimeSpecialization; cache != nil {
		return cache.recognized
	}
	return runtimeSpecializationCacheForProto(proto).recognized
}

func cachedRuntimeSpecializationRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(wholeCallValueRuntimeSpecializationRegistry) {
		return false
	}
	return cachedRuntimeSpecializationBits(proto)&(uint64(1)<<uint(id)) != 0
}

func runtimeSpecializationCacheForProto(proto *FuncProto) *runtimeSpecializationProtoCache {
	fp := wholeCallKernelFingerprintForProto(proto)
	cache := proto.RuntimeSpecialization
	if cache != nil && cache.fingerprint == fp {
		return cache
	}
	cache = &runtimeSpecializationProtoCache{fingerprint: fp}
	for i, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if entry.Info.Name == "" || entry.Recognize == nil {
			continue
		}
		if entry.Recognize(proto) {
			cache.recognized |= uint64(1) << uint(i)
		}
	}
	proto.RuntimeSpecialization = cache
	return cache
}
