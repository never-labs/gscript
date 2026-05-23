package vm

import "github.com/gscript/gscript/internal/runtime"

type boolTableMarkCountSpecializationCache struct {
	fingerprint runtimeSpecializationFingerprint
	spec        *boolTableMarkCountSpecializationSpec
}

type boolTableMarkCountSpecializationSpec struct {
	minValue int
}

type boolTableMarkCountShape struct {
	nReg     int
	flagsReg int
}

func (vm *VM) tryRunBoolTableMarkCountRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if cl == nil || cl.Proto == nil || !cachedRuntimeSpecializationRecognized(cl.Proto, runtimeSpecializationBoolTableMarkCount) {
		return false, nil, nil
	}
	return vm.runBoolTableMarkCountRuntimeSpecialization(cl, args)
}

func (vm *VM) runBoolTableMarkCountRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if cl == nil || cl.Proto == nil || len(args) != 1 || !vm.noGlobalLock {
		return false, nil, nil
	}
	spec, ok := boolTableMarkCountSpecializationSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	if !args[0].IsNumber() {
		return false, nil, nil
	}
	nn := args[0].Number()
	n64 := int64(nn)
	if float64(n64) != nn || n64 < 0 || int64(int(n64)) != n64 {
		return false, nil, nil
	}
	return true, []runtime.Value{runtime.IntValue(spec.run(int(n64)))}, nil
}

func (spec *boolTableMarkCountSpecializationSpec) run(n int) int64 {
	if spec == nil || n < spec.minValue {
		return 0
	}
	// Track only odd table positions. Index i represents 2*i+1, so index 1 is 3.
	// A zero byte means "not marked composite", avoiding a full initialization
	// pass over the source table shape.
	maxOddIndex := n / 2
	composite := make([]byte, maxOddIndex+1)
	for i := 1; ; i++ {
		p := 2*i + 1
		if p > n/p {
			break
		}
		if composite[i] != 0 {
			continue
		}
		step := 2 * p
		for j := p * p; j <= n; j += step {
			composite[j/2] = 1
		}
	}
	count := int64(1) // the even seed value remains unmarked by the odd-position table.
	for i := 1; i <= maxOddIndex; i++ {
		if 2*i+1 > n {
			continue
		}
		if composite[i] == 0 {
			count++
		}
	}
	return count
}

func IsBoolTableMarkCountSpecializationProto(p *FuncProto) bool {
	return cachedRuntimeSpecializationRecognized(p, runtimeSpecializationBoolTableMarkCount)
}

func isBoolTableMarkCountProto(p *FuncProto) bool {
	_, ok := boolTableMarkCountSpecializationSpecForProto(p)
	return ok
}

func boolTableMarkCountSpecializationSpecForProto(p *FuncProto) (*boolTableMarkCountSpecializationSpec, bool) {
	if p == nil || p.NumParams != 1 || p.IsVarArg || p.MaxStack < 13 ||
		len(p.Constants) != 0 || len(p.Protos) != 0 {
		return nil, false
	}
	fp := runtimeSpecializationFingerprintForProto(p)
	cache := p.BoolTableMarkCountSpecialization
	if cache != nil && cache.fingerprint == fp {
		return cache.spec, cache.spec != nil
	}
	spec, ok := analyzeBoolTableMarkCountSpecializationSpec(p.Code)
	if !ok {
		p.BoolTableMarkCountSpecialization = &boolTableMarkCountSpecializationCache{fingerprint: fp}
		return nil, false
	}
	p.BoolTableMarkCountSpecialization = &boolTableMarkCountSpecializationCache{fingerprint: fp, spec: spec}
	return spec, true
}

func analyzeBoolTableMarkCountSpecializationSpec(code []uint32) (*boolTableMarkCountSpecializationSpec, bool) {
	if len(code) < 40 {
		return nil, false
	}
	p := newBytecodePattern(code)
	shape, ok := matchBoolTableInitFill(p)
	if !ok ||
		!matchBoolTableMarkMultiples(p, shape) ||
		!matchBoolTableCountTruthy(p, shape) {
		return nil, false
	}
	return &boolTableMarkCountSpecializationSpec{minValue: 2}, true
}

func matchBoolTableInitFill(p bytecodePattern) (boolTableMarkCountShape, bool) {
	for fillForPrep := 4; fillForPrep < len(p.code); fillForPrep++ {
		prep, ok := p.op(fillForPrep, OP_FORPREP)
		if !ok {
			continue
		}
		fillBase := DecodeA(prep)
		bodyPC, loopPC, ok := p.numericForLoop(fillForPrep, fillBase)
		if !ok || loopPC != bodyPC+3 {
			continue
		}
		newTable, ok := p.op(fillForPrep-4, OP_NEWTABLE)
		if !ok || DecodeB(newTable) != 0 || DecodeC(newTable) != 0 {
			continue
		}
		flagsReg := DecodeA(newTable)
		if !p.loadInt(fillForPrep-3, fillBase, 2) ||
			!p.move(fillForPrep-2, fillBase+1, 0) ||
			!p.loadInt(fillForPrep-1, fillBase+2, 1) {
			continue
		}
		loadTrue, ok := p.op(bodyPC, OP_LOADBOOL)
		if !ok || DecodeB(loadTrue) != 1 {
			continue
		}
		trueReg := DecodeA(loadTrue)
		moveKey, ok := p.op(bodyPC+1, OP_MOVE)
		if !ok || DecodeB(moveKey) != fillBase+3 {
			continue
		}
		keyReg := DecodeA(moveKey)
		if !p.abc(bodyPC+2, OP_SETTABLE, flagsReg, keyReg, trueReg) {
			continue
		}
		return boolTableMarkCountShape{nReg: 0, flagsReg: flagsReg}, true
	}
	return boolTableMarkCountShape{}, false
}

func matchBoolTableMarkMultiples(p bytecodePattern, shape boolTableMarkCountShape) bool {
	const (
		iReg   = 5
		tmpReg = 6
		auxReg = 7
		keyReg = 8

		markStartPC = 10
		countStart  = 30
		innerStart  = 18
		afterInner  = 26
	)
	return p.loadInt(9, iReg, 2) &&
		p.abc(10, OP_MUL, tmpReg, iReg, iReg) &&
		p.abc(11, OP_LE, 0, tmpReg, shape.nReg) &&
		p.jumpTo(12, countStart) &&
		p.move(13, auxReg, iReg) &&
		p.abc(14, OP_GETTABLE, tmpReg, shape.flagsReg, auxReg) &&
		p.abc(15, OP_TEST, tmpReg, 0, 0) &&
		p.jumpTo(16, afterInner) &&
		p.abc(17, OP_MUL, tmpReg, iReg, iReg) &&
		p.abc(18, OP_LE, 0, tmpReg, shape.nReg) &&
		p.jumpTo(19, afterInner) &&
		p.loadBool(20, auxReg, false) &&
		p.move(21, keyReg, tmpReg) &&
		p.abc(22, OP_SETTABLE, shape.flagsReg, keyReg, auxReg) &&
		p.abc(23, OP_ADD, auxReg, tmpReg, iReg) &&
		p.move(24, tmpReg, auxReg) &&
		p.jumpTo(25, innerStart) &&
		p.loadInt(26, auxReg, 1) &&
		p.abc(27, OP_ADD, tmpReg, iReg, auxReg) &&
		p.move(28, iReg, tmpReg) &&
		p.jumpTo(29, markStartPC)
}

func matchBoolTableCountTruthy(p bytecodePattern, shape boolTableMarkCountShape) bool {
	for countPrep := 4; countPrep < len(p.code); countPrep++ {
		prep, ok := p.op(countPrep, OP_FORPREP)
		if !ok {
			continue
		}
		countBase := DecodeA(prep)
		bodyPC, loopPC, ok := p.numericForLoop(countPrep, countBase)
		if !ok || loopPC != bodyPC+7 {
			continue
		}
		countInit, ok := p.op(countPrep-4, OP_LOADINT)
		if !ok || DecodesBx(countInit) != 0 {
			continue
		}
		countReg := DecodeA(countInit)
		if !p.loadInt(countPrep-3, countBase, 2) ||
			!p.move(countPrep-2, countBase+1, shape.nReg) ||
			!p.loadInt(countPrep-1, countBase+2, 1) {
			continue
		}
		moveKey, ok := p.op(bodyPC, OP_MOVE)
		if !ok || DecodeB(moveKey) != countBase+3 {
			continue
		}
		keyReg := DecodeA(moveKey)
		getFlag, ok := p.op(bodyPC+1, OP_GETTABLE)
		if !ok || DecodeB(getFlag) != shape.flagsReg || DecodeC(getFlag) != keyReg {
			continue
		}
		flagReg := DecodeA(getFlag)
		if !p.abc(bodyPC+2, OP_TEST, flagReg, 0, 0) ||
			!p.jumpTo(bodyPC+3, loopPC) {
			continue
		}
		loadOne, ok := p.op(bodyPC+4, OP_LOADINT)
		if !ok || DecodesBx(loadOne) != 1 {
			continue
		}
		oneReg := DecodeA(loadOne)
		if !p.abc(bodyPC+5, OP_ADD, flagReg, countReg, oneReg) ||
			!p.move(bodyPC+6, countReg, flagReg) {
			continue
		}
		moveRet, ok := p.op(loopPC+1, OP_MOVE)
		if !ok || DecodeB(moveRet) != countReg {
			continue
		}
		returnReg := DecodeA(moveRet)
		if p.returnFixed(loopPC+2, returnReg, 2) {
			return true
		}
	}
	return false
}
