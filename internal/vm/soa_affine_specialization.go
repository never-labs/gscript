package vm

import "github.com/gscript/gscript/internal/runtime"

type soaColumnAffineUpdateSpec struct {
	srcName string
	dstName string
	guard   runtime.SoAShapeSnapshot
}

type soaColumnAffineUpdateCache struct {
	recognized bool
	spec       soaColumnAffineUpdateSpec
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
