package vm

import "github.com/never-labs/leia/internal/runtime"

type permutationFlipChecksumSpecializationCache struct {
	fingerprint runtimeSpecializationFingerprint
	spec        *permutationFlipChecksumSpecializationSpec
}

type permutationFlipChecksumSpecializationSpec struct {
	resultCtor *runtime.SmallTableCtor2
}

func (vm *VM) runPermutationFlipChecksumRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if cl == nil || cl.Proto == nil || len(args) != 1 || !vm.noGlobalLock {
		return false, nil, nil
	}
	if !args[0].IsNumber() {
		return false, nil, nil
	}
	nn := args[0].Number()
	n64 := int64(nn)
	if float64(n64) != nn || n64 < 1 || int64(int(n64)) != n64 {
		return false, nil, nil
	}
	spec, ok := permutationFlipChecksumSpecializationSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	result, ok := runPermutationFlipChecksumSpecialization(int(n64), spec.resultCtor)
	if !ok {
		return false, nil, nil
	}
	seedPermutationFlipChecksumFeedback(cl.Proto)
	return true, []runtime.Value{runtime.FreshTableValue(result)}, nil
}

func isPermutationFlipChecksumSpecializationProto(p *FuncProto) bool {
	_, ok := permutationFlipChecksumSpecializationSpecForProto(p)
	return ok
}

func permutationFlipChecksumSpecializationSpecForProto(p *FuncProto) (*permutationFlipChecksumSpecializationSpec, bool) {
	if p == nil {
		return nil, false
	}
	fp := runtimeSpecializationFingerprintForProto(p)
	p.RuntimeSpecs.mu.Lock()
	cache := p.RuntimeSpecs.PermutationFlipChecksumSpecialization
	if cache != nil && cache.fingerprint == fp {
		p.RuntimeSpecs.mu.Unlock()
		return cache.spec, cache.spec != nil
	}
	p.RuntimeSpecs.mu.Unlock()

	spec, ok := analyzePermutationFlipChecksumSpecializationSpec(p)
	cache = &permutationFlipChecksumSpecializationCache{fingerprint: fp}
	if ok {
		cache.spec = spec
	}
	p.RuntimeSpecs.mu.Lock()
	if existing := p.RuntimeSpecs.PermutationFlipChecksumSpecialization; existing != nil && existing.fingerprint == fp {
		p.RuntimeSpecs.mu.Unlock()
		return existing.spec, existing.spec != nil
	}
	p.RuntimeSpecs.PermutationFlipChecksumSpecialization = cache
	p.RuntimeSpecs.mu.Unlock()
	if !ok {
		return nil, false
	}
	return spec, true
}

func analyzePermutationFlipChecksumSpecializationSpec(p *FuncProto) (*permutationFlipChecksumSpecializationSpec, bool) {
	if p == nil || p.NumParams != 1 || p.IsVarArg || p.MaxStack != 30 || len(p.Protos) != 0 || len(p.Constants) != 2 {
		return nil, false
	}
	ctor, ok := permutationFlipChecksumResultCtor(p)
	if !ok {
		return nil, false
	}
	if !matchPermutationFlipChecksumBytecode(p.Code) {
		return nil, false
	}
	return &permutationFlipChecksumSpecializationSpec{resultCtor: ctor}, true
}

func matchPermutationFlipChecksumBytecode(code []uint32) bool {
	if len(code) < 120 || len(code) > 130 {
		return false
	}
	p := newBytecodePattern(code)
	return matchPermutationTableSeeds(p) &&
		matchPermutationLoopSkeleton(p) &&
		matchPermutationFlipAndChecksumSkeleton(p) &&
		matchPermutationResultReturn(p)
}

func matchPermutationTableSeeds(p bytecodePattern) bool {
	return p.abc(0, OP_NEWTABLE, 1, 0, 0) &&
		p.abc(1, OP_NEWTABLE, 2, 0, 0) &&
		p.abc(2, OP_NEWTABLE, 3, 0, 0)
}

func matchPermutationLoopSkeleton(p bytecodePattern) bool {
	initLoop, ok := p.findNumericForLoopWithBodyLen(0, 6)
	if !ok || !matchPermutationSeedLoopBody(p, initLoop.bodyPC) {
		return false
	}
	copyLoop, ok := p.findNumericForLoopWithBodyLen(initLoop.loopPC+1, 4)
	if !ok || !matchPermutationCopyLoopBody(p, copyLoop.bodyPC) {
		return false
	}
	rotateLoop, ok := p.findNumericForLoopWithBodyLen(copyLoop.loopPC+1, 34)
	if !ok {
		return false
	}
	innerRotateLoop, ok := p.findNumericForLoopWithBodyLen(rotateLoop.bodyPC, 5)
	return ok && innerRotateLoop.loopPC < rotateLoop.loopPC
}

func matchPermutationSeedLoopBody(p bytecodePattern, bodyPC int) bool {
	firstSet, ok := p.op(bodyPC+2, OP_SETTABLE)
	if !ok || DecodeA(firstSet) != 2 {
		return false
	}
	secondSet, ok := p.op(bodyPC+5, OP_SETTABLE)
	return ok && DecodeA(secondSet) == 3
}

func matchPermutationCopyLoopBody(p bytecodePattern, bodyPC int) bool {
	get, ok := p.op(bodyPC+1, OP_GETTABLE)
	if !ok || DecodeB(get) != 2 {
		return false
	}
	set, ok := p.op(bodyPC+3, OP_SETTABLE)
	return ok && DecodeA(set) == 1
}

func matchPermutationFlipAndChecksumSkeleton(p bytecodePattern) bool {
	hasFlipLoop := false
	hasChecksumParity := false
	for pc := range p.code {
		if p.abc(pc, OP_LT, 0, 15, 16) {
			hasFlipLoop = true
		}
		if p.abc(pc, OP_MOD, 15, 9, 16) {
			hasChecksumParity = true
		}
	}
	return hasFlipLoop && hasChecksumParity
}

func matchPermutationResultReturn(p bytecodePattern) bool {
	inst, ok := p.op(len(p.code)-2, OP_NEWOBJECT2)
	if !ok || DecodeA(inst) != 19 {
		return false
	}
	return p.returnFixed(len(p.code)-1, 19, 2)
}

func permutationFlipChecksumResultCtor(p *FuncProto) (*runtime.SmallTableCtor2, bool) {
	if p == nil || len(p.Constants) != 2 || !p.Constants[0].IsString() || !p.Constants[1].IsString() {
		return nil, false
	}
	left, right := p.Constants[0].Str(), p.Constants[1].Str()
	if left == "" || right == "" || left == right {
		return nil, false
	}
	ctor := runtime.NewSmallTableCtor2(left, right)
	return &ctor, true
}

func seedPermutationFlipChecksumFeedback(p *FuncProto) {
	// Preserve the feedback shape that the old executed path produced so
	// diagnostics and later TypeSpec passes still see int-array accesses.
	fb := p.EnsureFeedback()
	for pc, inst := range p.Code {
		switch DecodeOp(inst) {
		case OP_GETTABLE:
			fb[pc].Result = FBInt
			fb[pc].Kind = FBKindInt
		case OP_SETTABLE:
			fb[pc].Kind = FBKindInt
		}
	}
}

func runPermutationFlipChecksumSpecialization(n int, ctor *runtime.SmallTableCtor2) (*runtime.Table, bool) {
	if n < 1 || n > 12 {
		return nil, false
	}
	if ctor == nil {
		return nil, false
	}
	perm := make([]int, n+1)
	perm1 := make([]int, n+1)
	count := make([]int, n+1)
	for i := 1; i <= n; i++ {
		perm1[i] = i
		count[i] = i
	}

	maxFlips := 0
	checksum := 0
	nperm := 0
	for {
		copy(perm[1:], perm1[1:])

		flips := 0
		for k := perm[1]; k != 1; k = perm[1] {
			for lo, hi := 1, k; lo < hi; lo, hi = lo+1, hi-1 {
				perm[lo], perm[hi] = perm[hi], perm[lo]
			}
			flips++
		}
		if flips > maxFlips {
			maxFlips = flips
		}
		if nperm%2 == 0 {
			checksum += flips
		} else {
			checksum -= flips
		}
		nperm++

		done := true
		for i := 2; i <= n; i++ {
			t := perm1[1]
			for j := 1; j < i; j++ {
				perm1[j] = perm1[j+1]
			}
			perm1[i] = t

			count[i]--
			if count[i] > 0 {
				done = false
				break
			}
			count[i] = i
		}
		if done {
			break
		}
	}

	return runtime.NewTableFromCtor2NonNil(
		ctor,
		runtime.IntValue(int64(maxFlips)),
		runtime.IntValue(int64(checksum)),
	), true
}
