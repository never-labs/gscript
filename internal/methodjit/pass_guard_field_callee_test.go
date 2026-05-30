package methodjit

import (
	"testing"
	"unsafe"

	"github.com/Never-Labs/gscript/internal/runtime"
	"github.com/Never-Labs/gscript/internal/vm"
)

func TestGuardFieldCalleePassFusesSingleUseFixedShapeLoad(t *testing.T) {
	fn, b, obj := newFieldNumFusionFn("guard_field_callee")
	load := &Instr{ID: fn.newValueID(), Op: OpGetField, Type: TypeAny,
		Args: []*Value{obj.Value()}, Aux: 1, Aux2: int64(42)<<32 | 3, Block: b}
	guard := &Instr{ID: fn.newValueID(), Op: OpGuardCalleeProto, Type: TypeAny,
		Args: []*Value{load.Value()}, Aux: 1234, Block: b}
	ret := &Instr{ID: fn.newValueID(), Op: OpReturn, Args: []*Value{guard.Value()}, Block: b}
	b.Instrs = []*Instr{obj, load, guard, ret}

	out, err := GuardFieldCalleePass(fn)
	if err != nil {
		t.Fatalf("GuardFieldCalleePass: %v", err)
	}
	if errs := Validate(out); len(errs) > 0 {
		t.Fatalf("invalid IR after pass: %v\n%s", errs, Print(out))
	}
	if countOpHelper(out, OpGetField) != 0 {
		t.Fatalf("single-use field load should be removed:\n%s", Print(out))
	}
	if guard.Op != OpGuardFieldCalleeProto || len(guard.Args) != 1 || guard.Args[0].ID != obj.ID || guard.Aux != 1234 || guard.Aux2 != load.Aux2 {
		t.Fatalf("guard not fused correctly:\n%s", Print(out))
	}
}

func TestGuardFieldCalleePassKeepsSharedLoad(t *testing.T) {
	fn, b, obj := newFieldNumFusionFn("guard_field_callee_shared")
	load := &Instr{ID: fn.newValueID(), Op: OpGetField, Type: TypeAny,
		Args: []*Value{obj.Value()}, Aux: 1, Aux2: int64(42)<<32 | 3, Block: b}
	guard := &Instr{ID: fn.newValueID(), Op: OpGuardCalleeProto, Type: TypeAny,
		Args: []*Value{load.Value()}, Aux: 1234, Block: b}
	ret := &Instr{ID: fn.newValueID(), Op: OpReturn, Args: []*Value{guard.Value(), load.Value()}, Block: b}
	b.Instrs = []*Instr{obj, load, guard, ret}

	out, err := GuardFieldCalleePass(fn)
	if err != nil {
		t.Fatalf("GuardFieldCalleePass: %v", err)
	}
	if errs := Validate(out); len(errs) > 0 {
		t.Fatalf("invalid IR after pass: %v\n%s", errs, Print(out))
	}
	if countOpHelper(out, OpGetField) != 1 {
		t.Fatalf("shared field load should remain:\n%s", Print(out))
	}
	if guard.Op != OpGuardFieldCalleeProto {
		t.Fatalf("callee guard should still be fused:\n%s", Print(out))
	}
}

func TestStableFieldCalleeGuardPassHoistsStableClosureField(t *testing.T) {
	callee := &vm.FuncProto{Name: "method", NumParams: 1}
	cl := vm.NewClosure(callee)
	tbl := runtime.NewTable()
	tbl.RawSetString("step", runtime.VMClosureFastValue(unsafe.Pointer(cl)))
	shapeID := tbl.ShapeID()
	if shapeID == 0 {
		t.Fatal("expected shaped table")
	}

	fn, b, obj := newFieldNumFusionFn("stable_field_callee")
	guard := &Instr{
		ID:    fn.newValueID(),
		Op:    OpGuardFieldCalleeProto,
		Type:  TypeAny,
		Args:  []*Value{obj.Value()},
		Aux:   int64(uintptr(unsafe.Pointer(callee))),
		Aux2:  int64(shapeID)<<32 | 0,
		Block: b,
	}
	retVal := &Instr{ID: fn.newValueID(), Op: OpConstInt, Type: TypeInt, Aux: 1, Block: b}
	ret := &Instr{ID: fn.newValueID(), Op: OpReturn, Args: []*Value{retVal.Value()}, Block: b}
	b.Instrs = []*Instr{obj, guard, retVal, ret}
	fn.ensureAnalysis()
	fn.Analysis.TableShapeFacts().RecordFieldPolyShapeCases(guard.ID, []FieldPolyShapeCase{{
		ShapeID:   shapeID,
		FieldIdx:  0,
		VMProto:   callee,
		VMClosure: uintptr(unsafe.Pointer(cl)),
	}})

	out, err := StableFieldCalleeGuardPass(fn)
	if err != nil {
		t.Fatalf("StableFieldCalleeGuardPass: %v", err)
	}
	if errs := Validate(out); len(errs) > 0 {
		t.Fatalf("invalid IR after pass: %v\n%s", errs, Print(out))
	}
	if countOpHelper(out, OpGuardFieldCalleeProto) != 0 {
		t.Fatalf("field callee guard should be removed:\n%s", Print(out))
	}
	if countOpHelper(out, OpGuardShapeFieldVMClosure) != 1 {
		t.Fatalf("entry stable closure guard not inserted:\n%s", Print(out))
	}
}

func TestStableFieldCalleeGuardPassKeepsGuardWhenSameFieldMutates(t *testing.T) {
	callee := &vm.FuncProto{Name: "method", NumParams: 1}
	cl := vm.NewClosure(callee)
	tbl := runtime.NewTable()
	tbl.RawSetString("step", runtime.VMClosureFastValue(unsafe.Pointer(cl)))
	shapeID := tbl.ShapeID()
	if shapeID == 0 {
		t.Fatal("expected shaped table")
	}

	fn, b, obj := newFieldNumFusionFn("stable_field_callee_mutating")
	svals := &Instr{ID: fn.newValueID(), Op: OpFieldSvals, Type: TypeInt, Args: []*Value{obj.Value()}, Aux: int64(shapeID), Block: b}
	next := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeAny, Aux: 1, Block: b}
	store := &Instr{ID: fn.newValueID(), Op: OpFieldStore, Args: []*Value{svals.Value(), next.Value()}, Aux: 0, Block: b}
	guard := &Instr{
		ID:    fn.newValueID(),
		Op:    OpGuardFieldCalleeProto,
		Type:  TypeAny,
		Args:  []*Value{obj.Value()},
		Aux:   int64(uintptr(unsafe.Pointer(callee))),
		Aux2:  int64(shapeID)<<32 | 0,
		Block: b,
	}
	retVal := &Instr{ID: fn.newValueID(), Op: OpConstInt, Type: TypeInt, Aux: 1, Block: b}
	ret := &Instr{ID: fn.newValueID(), Op: OpReturn, Args: []*Value{retVal.Value()}, Block: b}
	b.Instrs = []*Instr{obj, svals, next, store, guard, retVal, ret}
	fn.ensureAnalysis()
	fn.Analysis.TableShapeFacts().RecordFieldPolyShapeCases(guard.ID, []FieldPolyShapeCase{{
		ShapeID:   shapeID,
		FieldIdx:  0,
		VMProto:   callee,
		VMClosure: uintptr(unsafe.Pointer(cl)),
	}})

	out, err := StableFieldCalleeGuardPass(fn)
	if err != nil {
		t.Fatalf("StableFieldCalleeGuardPass: %v", err)
	}
	if countOpHelper(out, OpGuardFieldCalleeProto) != 1 {
		t.Fatalf("mutating function must keep field callee guard:\n%s", Print(out))
	}
	if countOpHelper(out, OpGuardShapeFieldVMClosure) != 0 {
		t.Fatalf("mutating function must not insert entry closure guard:\n%s", Print(out))
	}
}
