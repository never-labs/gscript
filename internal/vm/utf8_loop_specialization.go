package vm

import (
	"unicode/utf8"

	"github.com/never-labs/leia/internal/runtime"
)

func (vm *VM) tryUTF8CodepointSumLoopRuntimeSpecialization(frame *CallFrame, base int, a int) (bool, error) {
	proto := frame.closure.Proto
	if proto == nil || !isUTF8CodepointSumLoopProto(proto, frame.pc-1, a) || !vm.standardUTF8CodesActive() {
		return false, nil
	}
	textNameConst := DecodeBx(proto.Code[frame.pc-3])
	if textNameConst < 0 || textNameConst >= len(proto.Constants) || !proto.Constants[textNameConst].IsString() {
		return false, nil
	}
	textValue := vm.GetGlobal(proto.Constants[textNameConst].Str())
	if !textValue.IsString() {
		return false, nil
	}
	sumSlot := base + a - 1
	if sumSlot < 0 || sumSlot >= len(vm.regs) || !vm.regs[sumSlot].IsInt() {
		return false, nil
	}
	sum := vm.regs[sumSlot].Int()
	for pos := 0; pos < len(textValue.Str()); {
		r, size := utf8.DecodeRuneInString(textValue.Str()[pos:])
		if r == utf8.RuneError && size == 1 {
			return false, nil
		}
		sum += int64(r) + int64(pos+1)
		pos += size
	}
	vm.regs[sumSlot].SetInt(sum)
	frame.pc += 6
	runtime.RecordRuntimePathRuntimeSpecializationHit(string(RuntimeSpecializationRouteDriverLoop), "utf8_codepoint_sum_loop")
	return true, nil
}

func isUTF8CodepointSumLoopProto(proto *FuncProto, pc int, a int) bool {
	if proto == nil {
		return false
	}
	proto.RuntimeSpecs.mu.Lock()
	defer proto.RuntimeSpecs.mu.Unlock()
	switch proto.RuntimeSpecs.UTF8CodepointSumLoopShape {
	case 1:
		return true
	case -1:
		return false
	}
	if matchUTF8CodepointSumLoopProto(proto, pc, a) {
		proto.RuntimeSpecs.UTF8CodepointSumLoopShape = 1
		return true
	}
	proto.RuntimeSpecs.UTF8CodepointSumLoopShape = -1
	return false
}

func matchUTF8CodepointSumLoopProto(proto *FuncProto, pc int, a int) bool {
	if pc < 4 || pc+6 >= len(proto.Code) {
		return false
	}
	insts := proto.Code
	setup0 := insts[pc-4]
	setup1 := insts[pc-3]
	setup2 := insts[pc-2]
	setup3 := insts[pc-1]
	if DecodeOp(setup0) != OP_GETGLOBAL || DecodeA(setup0) != a+1 || !constString(proto, DecodeBx(setup0), "utf8") {
		return false
	}
	if DecodeOp(setup1) != OP_GETFIELD || DecodeA(setup1) != a || DecodeB(setup1) != a+1 || !constString(proto, DecodeC(setup1), "codes") {
		return false
	}
	if DecodeOp(setup2) != OP_GETGLOBAL || DecodeA(setup2) != a+1 {
		return false
	}
	if DecodeOp(setup3) != OP_CALL || DecodeA(setup3) != a || DecodeB(setup3) != 2 || DecodeC(setup3) != 4 {
		return false
	}
	sum := a - 1
	pos := a + 3
	cp := a + 4
	tmp0 := a + 6
	tmp1 := a + 5
	return insts[pc] == EncodeABC(OP_TFORCALL, a, 0, 2) &&
		insts[pc+1] == EncodeAsBx(OP_TFORLOOP, a+2, 1) &&
		insts[pc+2] == EncodesBx(OP_JMP, 4) &&
		insts[pc+3] == EncodeABC(OP_ADD, tmp0, sum, cp) &&
		insts[pc+4] == EncodeABC(OP_ADD, tmp1, tmp0, pos) &&
		insts[pc+5] == EncodeABC(OP_MOVE, sum, tmp1, 0) &&
		insts[pc+6] == EncodesBx(OP_JMP, -7)
}

func (vm *VM) standardUTF8CodesActive() bool {
	utf8Value := vm.GetGlobal("utf8")
	if !utf8Value.IsTable() {
		return false
	}
	codes := utf8Value.Table().RawGetString("codes")
	gf := codes.GoFunction()
	return gf != nil && gf.Name == "utf8.codes" && gf.FastArg1 != nil
}
