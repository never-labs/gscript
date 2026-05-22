package methodjit

import (
	"strings"
	"testing"
)

func TestValidatorOpSpecArgCountError(t *testing.T) {
	fn := &Function{}
	b0 := &Block{ID: 0}
	tbl := &Instr{ID: 0, Op: OpLoadSlot, Type: TypeTable, Block: b0}
	key := &Instr{ID: 1, Op: OpLoadSlot, Type: TypeAny, Block: b0}
	set := &Instr{ID: 2, Op: OpSetTable, Block: b0, Args: []*Value{tbl.Value(), key.Value()}}
	ret := &Instr{ID: 3, Op: OpReturn, Block: b0}

	b0.Instrs = []*Instr{tbl, key, set, ret}
	fn.Entry = b0
	fn.Blocks = []*Block{b0}

	errs := Validate(fn)
	if !hasValidationError(errs, "SetTable", "exactly 3 args") {
		t.Fatalf("expected OpSpec arg count error for SetTable, got: %v", errs)
	}
}

func TestValidatorTerminatorShapeComesFromOpSpec(t *testing.T) {
	spec, ok := OpBranch.Spec()
	if !ok {
		t.Fatalf("Branch should have an OpSpec")
	}
	if !spec.Terminator {
		t.Fatalf("Branch should be marked as a terminator by OpSpec")
	}
	contract := validatorContractForOp(OpBranch)
	if contract.SuccCount != 2 {
		t.Fatalf("Branch should require 2 successors by validator contract, got %d", contract.SuccCount)
	}
	if !contract.Args.Set || contract.Args.Min != 1 || contract.Args.Max != 1 {
		t.Fatalf("Branch should require exactly 1 arg by validator contract, got %+v", contract.Args)
	}

	fn := &Function{}
	b0 := &Block{ID: 0}
	b1 := &Block{ID: 1}
	cond := &Value{ID: 99}

	b0.Succs = []*Block{b1}
	b1.Preds = []*Block{b0}
	b0.Instrs = []*Instr{{ID: 0, Op: OpBranch, Block: b0, Args: []*Value{cond}}}
	b1.Instrs = []*Instr{{ID: 1, Op: OpReturn, Block: b1}}
	fn.Entry = b0
	fn.Blocks = []*Block{b0, b1}

	errs := Validate(fn)
	if !hasValidationError(errs, "Branch", "2 successors") {
		t.Fatalf("expected OpSpec terminator successor error for Branch, got: %v", errs)
	}
}

func hasValidationError(errs []error, parts ...string) bool {
	for _, err := range errs {
		msg := err.Error()
		found := true
		for _, part := range parts {
			if !strings.Contains(msg, part) {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}
