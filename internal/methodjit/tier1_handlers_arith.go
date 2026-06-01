//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func (e *BaselineJITEngine) handleArithmetic(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for arithmetic op-exit")
	}

	op := vm.Opcode(ctx.BaselineOp)
	a := int(ctx.BaselineA)
	bidx := int(ctx.BaselineB)
	cidx := int(ctx.BaselineC)
	absA := base + a
	if absA < 0 || absA >= len(regs) {
		return fmt.Errorf("%s op-exit destination out of range", vm.OpName(op))
	}

	left := resolveCmpRK(regs, base, proto, bidx)
	right := resolveCmpRK(regs, base, proto, cidx)
	result, err := e.callVM.ArithmeticForJIT(op, left, right)
	if err != nil {
		return err
	}
	regs[absA] = result
	if proto != nil && proto.Feedback != nil {
		pc := int(ctx.BaselinePC) - 1
		if pc >= 0 && pc < len(proto.Feedback) {
			fb := &proto.Feedback[pc]
			fb.Left.Observe(left.Type())
			fb.Right.Observe(right.Type())
			fb.Result.Observe(result.Type())
		}
	}
	return nil
}
