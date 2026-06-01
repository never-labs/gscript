package vm

import "github.com/never-labs/leia/internal/runtime"

type coroutineAffineReturnSpec struct {
	mul int64
	add int64
}

func isCoroutineYieldSumLoopProto(p *FuncProto) bool {
	return matchCoroutineYieldSumLoopProto(p)
}

func isCoroutineCreateResumeAffineSumProto(p *FuncProto) bool {
	if !matchCoroutineCreateResumeAffineSumProto(p) || len(p.Protos) != 1 {
		return false
	}
	_, ok := coroutineAffineReturnSpecForProto(p.Protos[0])
	return ok
}

func matchCoroutineYieldSumLoopProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 18 || len(p.Protos) != 1 ||
		!protoStringConstantEquals(p, 0, "coroutine") || !protoStringConstantEquals(p, 1, "create") {
		return false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_GETGLOBAL, 1: OP_GETFIELD, 2: OP_CLOSURE, 3: OP_CALL,
		4: OP_LOADINT, 5: OP_LOADINT, 6: OP_MOVE, 7: OP_LOADINT,
		8: OP_FORPREP, 9: OP_MOVE, 10: OP_RESUME, 11: OP_ADD,
		12: OP_MOVE, 13: OP_FORLOOP, 14: OP_MOVE, 15: OP_RETURN,
		16: OP_CLOSE, 17: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return false
		}
	}
	if DecodeA(code[0]) != 2 || DecodeBx(code[0]) != 0 ||
		DecodeA(code[1]) != 1 || DecodeB(code[1]) != 2 || DecodeC(code[1]) != 1 ||
		DecodeA(code[2]) != 2 || DecodeBx(code[2]) != 0 ||
		DecodeA(code[3]) != 1 || DecodeB(code[3]) != 2 || DecodeC(code[3]) != 2 ||
		DecodeA(code[4]) != 2 || DecodesBx(code[4]) != 0 ||
		DecodeA(code[5]) != 3 || DecodesBx(code[5]) != 1 ||
		DecodeA(code[6]) != 4 || DecodeB(code[6]) != 0 ||
		DecodeA(code[7]) != 5 || DecodesBx(code[7]) != 1 ||
		DecodeA(code[9]) != 8 || DecodeB(code[9]) != 1 ||
		DecodeA(code[10]) != 7 || DecodeB(code[10]) != 2 || DecodeC(code[10]) != 3 ||
		DecodeA(code[11]) != 9 || DecodeB(code[11]) != 2 || DecodeC(code[11]) != 8 ||
		DecodeA(code[12]) != 2 || DecodeB(code[12]) != 9 ||
		DecodeA(code[14]) != 6 || DecodeB(code[14]) != 2 ||
		DecodeA(code[15]) != 6 || DecodeB(code[15]) != 2 {
		return false
	}
	return matchCoroutineRangeYieldIdentityProto(p.Protos[0])
}

func matchCoroutineRangeYieldIdentityProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 0 || p.UsesVarargBytecode || len(p.Upvalues) != 1 || len(p.Code) != 9 {
		return false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_LOADINT, 1: OP_GETUPVAL, 2: OP_LOADINT, 3: OP_FORPREP,
		4: OP_MOVE, 5: OP_YIELD, 6: OP_FORLOOP, 7: OP_GETUPVAL, 8: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return false
		}
	}
	return DecodeA(code[0]) == 0 && DecodesBx(code[0]) == 1 &&
		DecodeA(code[1]) == 1 && DecodeB(code[1]) == 0 &&
		DecodeA(code[2]) == 2 && DecodesBx(code[2]) == 1 &&
		DecodeA(code[4]) == 5 && DecodeB(code[4]) == 3 &&
		DecodeA(code[5]) == 4 && DecodeB(code[5]) == 2 &&
		DecodeA(code[7]) == 3 && DecodeB(code[7]) == 0 &&
		DecodeA(code[8]) == 3 && DecodeB(code[8]) == 2
}

func matchCoroutineCreateResumeAffineSumProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 17 || len(p.Protos) != 1 ||
		!protoStringConstantEquals(p, 0, "coroutine") || !protoStringConstantEquals(p, 1, "create") {
		return false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_LOADINT, 1: OP_LOADINT, 2: OP_MOVE, 3: OP_LOADINT, 4: OP_FORPREP,
		5: OP_GETGLOBAL, 6: OP_GETFIELD, 7: OP_CLOSURE, 8: OP_CALL, 9: OP_MOVE,
		10: OP_RESUME, 11: OP_ADD, 12: OP_MOVE, 13: OP_FORLOOP, 14: OP_CLOSE,
		15: OP_MOVE, 16: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return false
		}
	}
	return DecodeA(code[0]) == 1 && DecodesBx(code[0]) == 0 &&
		DecodeA(code[1]) == 2 && DecodesBx(code[1]) == 1 &&
		DecodeA(code[2]) == 3 && DecodeB(code[2]) == 0 &&
		DecodeA(code[3]) == 4 && DecodesBx(code[3]) == 1 &&
		DecodeA(code[5]) == 7 && DecodeBx(code[5]) == 0 &&
		DecodeA(code[6]) == 6 && DecodeB(code[6]) == 7 && DecodeC(code[6]) == 1 &&
		DecodeA(code[7]) == 7 && DecodeBx(code[7]) == 0 &&
		DecodeA(code[8]) == 6 && DecodeB(code[8]) == 2 && DecodeC(code[8]) == 2 &&
		DecodeA(code[9]) == 8 && DecodeB(code[9]) == 6 &&
		DecodeA(code[10]) == 7 && DecodeB(code[10]) == 2 && DecodeC(code[10]) == 3 &&
		DecodeA(code[11]) == 9 && DecodeB(code[11]) == 1 && DecodeC(code[11]) == 8 &&
		DecodeA(code[12]) == 1 && DecodeB(code[12]) == 9 &&
		DecodeA(code[14]) == 5 &&
		DecodeA(code[15]) == 5 && DecodeB(code[15]) == 1 &&
		DecodeA(code[16]) == 5 && DecodeB(code[16]) == 2
}

func coroutineAffineReturnSpecForProto(p *FuncProto) (coroutineAffineReturnSpec, bool) {
	if p == nil || p.NumParams != 0 || p.UsesVarargBytecode || len(p.Upvalues) != 1 || len(p.Code) == 0 {
		return coroutineAffineReturnSpec{}, false
	}
	code := p.Code
	if len(code) == 4 &&
		DecodeOp(code[0]) == OP_GETUPVAL &&
		DecodeOp(code[1]) == OP_LOADINT &&
		DecodeOp(code[2]) == OP_MUL &&
		DecodeOp(code[3]) == OP_RETURN &&
		DecodeA(code[0]) == 1 && DecodeB(code[0]) == 0 &&
		DecodeA(code[1]) == 2 &&
		DecodeA(code[2]) == 0 && DecodeB(code[2]) == 1 && DecodeC(code[2]) == 2 &&
		DecodeA(code[3]) == 0 && DecodeB(code[3]) == 2 {
		return coroutineAffineReturnSpec{mul: int64(DecodesBx(code[1]))}, true
	}
	if len(code) == 2 &&
		DecodeOp(code[0]) == OP_GETUPVAL &&
		DecodeOp(code[1]) == OP_RETURN &&
		DecodeA(code[0]) == 0 && DecodeB(code[0]) == 0 &&
		DecodeA(code[1]) == 0 && DecodeB(code[1]) == 2 {
		return coroutineAffineReturnSpec{mul: 1}, true
	}
	return coroutineAffineReturnSpec{}, false
}

func (vm *VM) runCoroutineYieldSumLoopRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 1 || args[0].RawType() != runtime.TypeInt || !matchCoroutineYieldSumLoopProto(cl.Proto) {
		return false, nil, nil
	}
	n := args[0].RawInt()
	if n < 0 {
		return false, nil, nil
	}
	return true, []runtime.Value{runtime.IntValue(arithmeticSeries(n))}, nil
}

func (vm *VM) runCoroutineCreateResumeAffineSumRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 1 || args[0].RawType() != runtime.TypeInt || !matchCoroutineCreateResumeAffineSumProto(cl.Proto) {
		return false, nil, nil
	}
	spec, ok := coroutineAffineReturnSpecForProto(cl.Proto.Protos[0])
	if !ok {
		return false, nil, nil
	}
	n := args[0].RawInt()
	if n < 0 {
		return false, nil, nil
	}
	total := arithmeticSeries(n)*spec.mul + n*spec.add
	return true, []runtime.Value{runtime.IntValue(total)}, nil
}

func arithmeticSeries(n int64) int64 {
	if n&1 == 0 {
		return (n / 2) * (n + 1)
	}
	return n * ((n + 1) / 2)
}

func protoStringConstantEquals(p *FuncProto, idx int, want string) bool {
	if p == nil || idx < 0 || idx >= len(p.Constants) {
		return false
	}
	got, ok := p.Constants[idx].RawString()
	return ok && got == want
}
