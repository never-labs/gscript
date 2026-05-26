//go:build darwin && arm64

// tier1_handlers_compare.go contains the Tier 1 baseline JIT exit handlers for
// comparison opcodes (OP_LT / OP_LE) that the native path cannot compare
// directly, including operand resolution and comparison-metamethod dispatch.
// Pure code movement from tier1_handlers.go; no behavior change.

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// resolveCmpRK resolves a comparison operand (RK). For registers, base+idx;
// for constants, proto.Constants[idx - RKBit].
func resolveCmpRK(regs []runtime.Value, base int, proto *vm.FuncProto, idx int) runtime.Value {
	if idx >= vm.RKBit {
		k := idx - vm.RKBit
		if k >= 0 && k < len(proto.Constants) {
			return proto.Constants[k]
		}
		return runtime.NilValue()
	}
	abs := base + idx
	if abs >= 0 && abs < len(regs) {
		return regs[abs]
	}
	return runtime.NilValue()
}

// handleLT handles OP_LT exit for operands that the native path can't compare
// (typically strings). Computes (bval < cval) via Value.LessThan and overrides
// BaselinePC based on the VM semantics: if (result) != bool(A), then PC++.
// The exit emitter stored BaselinePC = pc+1 (instruction after LT); we adjust
// to pc+2 when the skip fires.
func (e *BaselineJITEngine) handleLT(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	bidx := int(ctx.BaselineB)
	cidx := int(ctx.BaselineC)

	bval := resolveCmpRK(regs, base, proto, bidx)
	cval := resolveCmpRK(regs, base, proto, cidx)

	lt, ok := bval.LessThan(cval)
	if !ok {
		var err error
		lt, err = e.callComparisonMetamethod("__lt", bval, cval)
		if err != nil {
			return fmt.Errorf("LT: %w", err)
		}
	}
	if lt != (a != 0) {
		// VM does PC++ to skip the next instruction. BaselinePC is
		// already pc+1; bump to pc+2.
		ctx.BaselinePC++
	}
	return nil
}

// handleLE mirrors handleLT for OP_LE.
func (e *BaselineJITEngine) handleLE(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	a := int(ctx.BaselineA)
	bidx := int(ctx.BaselineB)
	cidx := int(ctx.BaselineC)

	bval := resolveCmpRK(regs, base, proto, bidx)
	cval := resolveCmpRK(regs, base, proto, cidx)

	// (b <= c) == !(c < b)
	gt, ok := cval.LessThan(bval)
	if !ok {
		var err error
		gt, err = e.callComparisonMetamethod("__le", bval, cval)
		if err != nil {
			return fmt.Errorf("LE: %w", err)
		}
		if gt != (a != 0) {
			ctx.BaselinePC++
		}
		return nil
	}
	le := !gt
	if le != (a != 0) {
		ctx.BaselinePC++
	}
	return nil
}

func (e *BaselineJITEngine) callComparisonMetamethod(name string, left, right runtime.Value) (bool, error) {
	if e.callVM == nil {
		return false, fmt.Errorf("no callVM for comparison metamethod")
	}
	mm := lookupBinaryMetamethod(left, right, name)
	if mm.IsNil() {
		return false, fmt.Errorf("cannot compare %s with %s", left.TypeName(), right.TypeName())
	}
	args := [2]runtime.Value{left, right}
	results, err := e.callVM.CallValue(mm, args[:])
	if err != nil {
		return false, err
	}
	if len(results) == 0 {
		return false, nil
	}
	return results[0].Truthy(), nil
}

func lookupBinaryMetamethod(left, right runtime.Value, name string) runtime.Value {
	if left.IsTable() {
		if mt := left.Table().GetMetatable(); mt != nil {
			if mm := mt.RawGetString(name); !mm.IsNil() {
				return mm
			}
		}
	}
	if right.IsTable() {
		if mt := right.Table().GetMetatable(); mt != nil {
			if mm := mt.RawGetString(name); !mm.IsNil() {
				return mm
			}
		}
	}
	return runtime.NilValue()
}
