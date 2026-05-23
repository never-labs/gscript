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

type boolTableMarkCountFillShape struct {
	boolTableMarkCountShape
	loopPC int
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
	fill, ok := matchBoolTableInitFill(p)
	if !ok ||
		!matchBoolTableMarkMultiples(p, fill) ||
		!matchBoolTableCountTruthy(p, fill.boolTableMarkCountShape) {
		return nil, false
	}
	return &boolTableMarkCountSpecializationSpec{minValue: 2}, true
}

func matchBoolTableInitFill(p bytecodePattern) (boolTableMarkCountFillShape, bool) {
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
		return boolTableMarkCountFillShape{
			boolTableMarkCountShape: boolTableMarkCountShape{nReg: 0, flagsReg: flagsReg},
			loopPC:                  loopPC,
		}, true
	}
	return boolTableMarkCountFillShape{}, false
}

func matchBoolTableMarkMultiples(p bytecodePattern, fill boolTableMarkCountFillShape) bool {
	for markStartPC := fill.loopPC + 2; markStartPC < len(p.code); markStartPC++ {
		init, ok := p.op(markStartPC-1, OP_LOADINT)
		if !ok || DecodesBx(init) != 2 {
			continue
		}
		iReg := DecodeA(init)
		firstMul, ok := p.op(markStartPC, OP_MUL)
		if !ok || DecodeB(firstMul) != iReg || DecodeC(firstMul) != iReg {
			continue
		}
		tmpReg := DecodeA(firstMul)
		if !p.abc(markStartPC+1, OP_LE, 0, tmpReg, fill.nReg) {
			continue
		}
		countStart, ok := p.jumpTarget(markStartPC + 2)
		if !ok || countStart <= markStartPC+18 || countStart >= len(p.code) {
			continue
		}
		moveProbe, ok := p.op(markStartPC+3, OP_MOVE)
		if !ok || DecodeB(moveProbe) != iReg {
			continue
		}
		auxReg := DecodeA(moveProbe)
		if !p.abc(markStartPC+4, OP_GETTABLE, tmpReg, fill.flagsReg, auxReg) ||
			!p.abc(markStartPC+5, OP_TEST, tmpReg, 0, 0) {
			continue
		}
		afterInner, ok := p.jumpTarget(markStartPC + 6)
		if !ok || afterInner < markStartPC+16 || afterInner >= countStart {
			continue
		}
		innerStart := markStartPC + 8
		if !p.abc(markStartPC+7, OP_MUL, tmpReg, iReg, iReg) ||
			!p.abc(innerStart, OP_LE, 0, tmpReg, fill.nReg) ||
			!p.jumpTo(markStartPC+9, afterInner) ||
			!p.loadBool(markStartPC+10, auxReg, false) {
			continue
		}
		moveKey, ok := p.op(markStartPC+11, OP_MOVE)
		if !ok || DecodeB(moveKey) != tmpReg {
			continue
		}
		keyReg := DecodeA(moveKey)
		if !p.abc(markStartPC+12, OP_SETTABLE, fill.flagsReg, keyReg, auxReg) ||
			!p.abc(markStartPC+13, OP_ADD, auxReg, tmpReg, iReg) ||
			!p.move(markStartPC+14, tmpReg, auxReg) ||
			!p.jumpTo(markStartPC+15, innerStart) ||
			!p.loadInt(afterInner, auxReg, 1) ||
			!p.abc(afterInner+1, OP_ADD, tmpReg, iReg, auxReg) ||
			!p.move(afterInner+2, iReg, tmpReg) ||
			!p.jumpTo(afterInner+3, markStartPC) {
			continue
		}
		return true
	}
	return false
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
