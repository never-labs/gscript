package vm

import "github.com/Never-Labs/gscript/internal/runtime"

type soaColumnAffineUpdateSpec struct {
	srcName string
	dstName string
	guard   runtime.SoAShapeSnapshot
}

type soaColumnAffineUpdateCache struct {
	recognized bool
	spec       soaColumnAffineUpdateSpec
}

type soaAffineManyScalarKind uint8

const (
	soaAffineManyScalarConst soaAffineManyScalarKind = iota + 1
	soaAffineManyScalarReg
)

type soaAffineManyScalarSpec struct {
	kind  soaAffineManyScalarKind
	reg   int
	value float64
}

type soaAffineManyLiteralTermSpec struct {
	dst   string
	src   string
	scale soaAffineManyScalarSpec
	bias  soaAffineManyScalarSpec
}

type soaAffineManyLiteralSpec struct {
	callA   int
	callC   int
	colsReg int
	nextPC  int
	terms   []soaAffineManyLiteralTermSpec
}

type soaAffineManyLiteralCallSiteCache struct {
	recognized bool
	probed     bool
	spec       soaAffineManyLiteralSpec
}

func isSoAColumnAffineUpdateProto(p *FuncProto) bool {
	_, ok := soaColumnAffineUpdateSpecForProto(p)
	return ok
}

func soaColumnAffineUpdateSpecForProto(p *FuncProto) (soaColumnAffineUpdateSpec, bool) {
	var spec soaColumnAffineUpdateSpec
	if p != nil && p.SoAColumnAffineUpdateSpecialization != nil {
		c := p.SoAColumnAffineUpdateSpecialization
		return c.spec, c.recognized
	}
	spec, ok := recognizeSoAColumnAffineUpdateSpec(p)
	if p != nil {
		p.SoAColumnAffineUpdateSpecialization = &soaColumnAffineUpdateCache{
			recognized: ok,
			spec:       spec,
		}
	}
	return spec, ok
}

func recognizeSoAColumnAffineUpdateSpec(p *FuncProto) (soaColumnAffineUpdateSpec, bool) {
	var spec soaColumnAffineUpdateSpec
	if p == nil || p.NumParams != 3 || p.IsVarArg || len(p.Code) != 26 || len(p.Constants) < 5 {
		return spec, false
	}
	pat := newBytecodePattern(p.Code)
	if !pat.hasOps(
		opcodeAt{pc: 0, op: OP_GETGLOBAL},
		opcodeAt{pc: 5, op: OP_GETGLOBAL},
		opcodeAt{pc: 10, op: OP_GETGLOBAL},
		opcodeAt{pc: 17, op: OP_FORPREP},
		opcodeAt{pc: 24, op: OP_FORLOOP},
	) ||
		!pat.hasABs(
			abAt{pc: 1, op: OP_GETFIELD, a: 3, b: 4},
			abAt{pc: 6, op: OP_GETFIELD, a: 4, b: 5},
			abAt{pc: 11, op: OP_GETFIELD, a: 5, b: 6},
			abAt{pc: 25, op: OP_RETURN, a: 0, b: 1},
		) ||
		!pat.hasABCs(
			abcAt{pc: 2, op: OP_MOVE, a: 4, b: 0, c: 0},
			abcAt{pc: 4, op: OP_CALL, a: 3, b: 3, c: 2},
			abcAt{pc: 7, op: OP_MOVE, a: 5, b: 0, c: 0},
			abcAt{pc: 9, op: OP_CALL, a: 4, b: 3, c: 2},
			abcAt{pc: 12, op: OP_MOVE, a: 6, b: 0, c: 0},
			abcAt{pc: 13, op: OP_CALL, a: 5, b: 2, c: 2},
			abcAt{pc: 15, op: OP_MOVE, a: 7, b: 5, c: 0},
			abcAt{pc: 18, op: OP_MOVE, a: 13, b: 9, c: 0},
			abcAt{pc: 19, op: OP_GETTABLE, a: 12, b: 3, c: 13},
			abcAt{pc: 20, op: OP_MUL, a: 11, b: 12, c: 1},
			abcAt{pc: 21, op: OP_ADD, a: 10, b: 11, c: 2},
			abcAt{pc: 22, op: OP_MOVE, a: 11, b: 9, c: 0},
			abcAt{pc: 23, op: OP_SETTABLE, a: 4, b: 11, c: 10},
		) ||
		!pat.hasASBxs(
			asbxAt{pc: 14, op: OP_LOADINT, a: 6, sbx: 1},
			asbxAt{pc: 16, op: OP_LOADINT, a: 8, sbx: 1},
		) {
		return spec, false
	}
	if !constantStringEquals(p, DecodeBx(p.Code[0]), "soa") ||
		!constantStringEquals(p, DecodeBx(p.Code[5]), "soa") ||
		!constantStringEquals(p, DecodeBx(p.Code[10]), "soa") ||
		!constantStringEquals(p, DecodeC(p.Code[1]), "column") ||
		!constantStringEquals(p, DecodeC(p.Code[6]), "column") ||
		!constantStringEquals(p, DecodeC(p.Code[11]), "len") {
		return spec, false
	}
	srcIdx := DecodeBx(p.Code[3])
	dstIdx := DecodeBx(p.Code[8])
	if !stringConst(p.Constants, srcIdx) || !stringConst(p.Constants, dstIdx) {
		return spec, false
	}
	spec.srcName = p.Constants[srcIdx].Str()
	spec.dstName = p.Constants[dstIdx].Str()
	return spec, true
}

func constantStringEquals(p *FuncProto, idx int, want string) bool {
	return idx >= 0 && idx < len(p.Constants) && p.Constants[idx].IsString() && p.Constants[idx].Str() == want
}

func soaAffineManyLiteralSpecForProto(p *FuncProto, startPC int) (soaAffineManyLiteralSpec, bool) {
	var spec soaAffineManyLiteralSpec
	if p == nil || startPC < 0 || startPC >= len(p.Code) {
		return spec, false
	}
	if len(p.SoAAffineManyLiteralSpecialization) != len(p.Code) {
		p.SoAAffineManyLiteralSpecialization = make([]soaAffineManyLiteralCallSiteCache, len(p.Code))
	}
	cache := &p.SoAAffineManyLiteralSpecialization[startPC]
	if cache.probed {
		return cache.spec, cache.recognized
	}
	spec, ok := recognizeSoAAffineManyLiteralSpec(p, startPC)
	cache.probed = true
	cache.recognized = ok
	cache.spec = spec
	return spec, ok
}

func recognizeSoAAffineManyLiteralSpec(p *FuncProto, startPC int) (soaAffineManyLiteralSpec, bool) {
	var spec soaAffineManyLiteralSpec
	if p == nil || startPC < 3 || startPC >= len(p.Code) {
		return spec, false
	}
	newTable := p.Code[startPC]
	if DecodeOp(newTable) != OP_NEWTABLE || DecodeC(newTable) != 0 {
		return spec, false
	}
	termsReg := DecodeA(newTable)
	termCount := DecodeB(newTable)
	if termCount <= 0 || termCount > 8 {
		return spec, false
	}
	getGlobal := p.Code[startPC-3]
	getField := p.Code[startPC-2]
	moveCols := p.Code[startPC-1]
	if DecodeOp(getGlobal) != OP_GETGLOBAL ||
		DecodeOp(getField) != OP_GETFIELD ||
		DecodeOp(moveCols) != OP_MOVE ||
		!constantStringEquals(p, DecodeBx(getGlobal), "soa") ||
		!constantStringEquals(p, DecodeC(getField), "affineMany") {
		return spec, false
	}
	globalReg := DecodeA(getGlobal)
	callA := DecodeA(getField)
	if DecodeB(getField) != globalReg || DecodeA(moveCols) != callA+1 || termsReg != callA+2 {
		return spec, false
	}
	spec.callA = callA
	spec.colsReg = DecodeB(moveCols)
	spec.terms = make([]soaAffineManyLiteralTermSpec, 0, termCount)
	pc := startPC + 1
	for i := 0; i < termCount; i++ {
		if pc+4 >= len(p.Code) {
			return spec, false
		}
		dstLoad := p.Code[pc]
		srcLoad := p.Code[pc+1]
		scaleLoad := p.Code[pc+2]
		biasLoad := p.Code[pc+3]
		newObject := p.Code[pc+4]
		if DecodeOp(dstLoad) != OP_LOADK || DecodeOp(srcLoad) != OP_LOADK || DecodeOp(newObject) != OP_NEWOBJECTN {
			return spec, false
		}
		termReg := termsReg + i + 1
		valueBase := DecodeC(newObject)
		if DecodeA(newObject) != termReg ||
			DecodeA(dstLoad) != valueBase ||
			DecodeA(srcLoad) != valueBase+1 ||
			DecodeA(scaleLoad) != valueBase+2 ||
			DecodeA(biasLoad) != valueBase+3 {
			return spec, false
		}
		ctorIdx := DecodeB(newObject)
		if !soaAffineManyTermCtorKeys(p, ctorIdx) {
			return spec, false
		}
		dstIdx := DecodeBx(dstLoad)
		srcIdx := DecodeBx(srcLoad)
		if !stringConst(p.Constants, dstIdx) || !stringConst(p.Constants, srcIdx) {
			return spec, false
		}
		scale, ok := soaAffineManyScalarFromProducer(p, scaleLoad)
		if !ok {
			return spec, false
		}
		bias, ok := soaAffineManyScalarFromProducer(p, biasLoad)
		if !ok {
			return spec, false
		}
		spec.terms = append(spec.terms, soaAffineManyLiteralTermSpec{
			dst:   p.Constants[dstIdx].Str(),
			src:   p.Constants[srcIdx].Str(),
			scale: scale,
			bias:  bias,
		})
		pc += 5
	}
	if pc+1 >= len(p.Code) {
		return spec, false
	}
	setList := p.Code[pc]
	call := p.Code[pc+1]
	if DecodeOp(setList) != OP_SETLIST || DecodeA(setList) != termsReg || DecodeB(setList) != termCount || DecodeC(setList) != 1 ||
		DecodeOp(call) != OP_CALL || DecodeA(call) != callA || DecodeB(call) != 3 {
		return spec, false
	}
	spec.callC = DecodeC(call)
	spec.nextPC = pc + 2
	return spec, true
}

func soaAffineManyTermCtorKeys(p *FuncProto, ctorIdx int) bool {
	if ctorIdx < 0 || ctorIdx >= len(p.TableCtorsN) {
		return false
	}
	keys := p.TableCtorsN[ctorIdx].KeyConsts
	return len(keys) == 4 &&
		constantStringEquals(p, keys[0], "dst") &&
		constantStringEquals(p, keys[1], "src") &&
		constantStringEquals(p, keys[2], "scale") &&
		constantStringEquals(p, keys[3], "bias")
}

func soaAffineManyScalarFromProducer(p *FuncProto, inst uint32) (soaAffineManyScalarSpec, bool) {
	switch DecodeOp(inst) {
	case OP_MOVE:
		return soaAffineManyScalarSpec{kind: soaAffineManyScalarReg, reg: DecodeB(inst)}, true
	case OP_LOADINT:
		return soaAffineManyScalarSpec{kind: soaAffineManyScalarConst, value: float64(DecodesBx(inst))}, true
	case OP_LOADK:
		idx := DecodeBx(inst)
		if idx < 0 || idx >= len(p.Constants) || !p.Constants[idx].IsNumber() {
			return soaAffineManyScalarSpec{}, false
		}
		return soaAffineManyScalarSpec{kind: soaAffineManyScalarConst, value: p.Constants[idx].Number()}, true
	default:
		return soaAffineManyScalarSpec{}, false
	}
}

func (vm *VM) runSoAColumnAffineUpdateRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, error) {
	if len(args) != 3 || !args[0].IsSoA() || !args[1].IsNumber() || !args[2].IsNumber() {
		return false, nil
	}
	spec, ok := soaColumnAffineUpdateSpecForProto(cl.Proto)
	if !ok {
		return false, nil
	}
	s := args[0].SoA()
	if len(spec.guard.Columns) > 0 && !s.ValidateSnapshotForWrites(spec.guard, spec.dstName) {
		spec.guard = runtime.SoAShapeSnapshot{}
	}
	if len(spec.guard.Columns) == 0 {
		guard, err := s.Snapshot(spec.dstName, spec.srcName)
		if err != nil {
			return false, nil
		}
		spec.guard = guard
		if cl != nil && cl.Proto != nil && cl.Proto.SoAColumnAffineUpdateSpecialization != nil {
			cl.Proto.SoAColumnAffineUpdateSpecialization.spec = spec
		}
	}
	if err := s.Affine(spec.dstName, spec.srcName, args[1].Number(), args[2].Number()); err != nil {
		return false, nil
	}
	return true, nil
}

func (vm *VM) trySoAAffineManyLiteralRuntimeSpecialization(frame *CallFrame, base int, startPC int) (bool, error) {
	if frame == nil || frame.closure == nil || frame.closure.Proto == nil {
		return false, nil
	}
	spec, ok := soaAffineManyLiteralSpecForProto(frame.closure.Proto, startPC)
	if !ok {
		return false, nil
	}
	fnSlot := base + spec.callA
	colsSlot := base + spec.colsReg
	if fnSlot < 0 || fnSlot >= len(vm.regs) || colsSlot < 0 || colsSlot >= len(vm.regs) {
		return false, nil
	}
	if !runtime.IsStdSoAAffineManyFunction(vm.regs[fnSlot]) || !vm.regs[colsSlot].IsSoA() {
		return false, nil
	}
	var stackTerms [8]runtime.SoAAffineTerm
	terms := stackTerms[:len(spec.terms)]
	for i, term := range spec.terms {
		scale, ok := vm.soaAffineManyScalarValue(base, term.scale)
		if !ok {
			return false, nil
		}
		bias, ok := vm.soaAffineManyScalarValue(base, term.bias)
		if !ok {
			return false, nil
		}
		terms[i] = runtime.SoAAffineTerm{Dst: term.dst, Src: term.src, Scale: scale, Bias: bias}
	}
	if err := vm.emitDebugHook("call", "native", "soa.affineMany", runtime.NilValue()); err != nil {
		return true, err
	}
	if err := vm.regs[colsSlot].SoA().AffineMany(terms); err != nil {
		_ = vm.emitDebugHook("error", "native", "soa.affineMany", runtime.StringValue(err.Error()))
		return true, err
	}
	if err := vm.emitDebugHook("return", "native", "soa.affineMany", runtime.NilValue()); err != nil {
		return true, err
	}
	vm.writeSoAAffineManyLiteralResults(base+spec.callA, spec.callC)
	frame.pc = spec.nextPC
	runtime.RecordRuntimePathRuntimeSpecializationHit(string(RuntimeSpecializationRouteCallSiteNoResult), "soa_affine_many_literal")
	return true, nil
}

func (vm *VM) soaAffineManyScalarValue(base int, spec soaAffineManyScalarSpec) (float64, bool) {
	switch spec.kind {
	case soaAffineManyScalarConst:
		return spec.value, true
	case soaAffineManyScalarReg:
		slot := base + spec.reg
		if slot < 0 || slot >= len(vm.regs) || !vm.regs[slot].IsNumber() {
			return 0, false
		}
		return vm.regs[slot].Number(), true
	default:
		return 0, false
	}
}

func (vm *VM) writeSoAAffineManyLiteralResults(dst, c int) {
	switch c {
	case 0:
		vm.regs[dst] = runtime.BoolValue(true)
		vm.top = dst + 1
	case 1:
		vm.writeNoResults(dst, c)
	default:
		vm.regs[dst] = runtime.BoolValue(true)
		for i := 1; i < c-1; i++ {
			vm.regs[dst+i] = runtime.NilValue()
		}
	}
}
