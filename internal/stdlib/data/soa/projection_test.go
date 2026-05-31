package soa

import "testing"

func TestColumnProjection(t *testing.T) {
	proj := NewColumnProjection("x")
	if proj.Name != "x" {
		t.Fatalf("projection name = %q, want x", proj.Name)
	}
}

func TestDenseArrayMeta(t *testing.T) {
	meta := NewDenseArrayMeta(DTypeBool, 7)
	if !meta.Present || meta.DType != DTypeBool || meta.Version != 7 {
		t.Fatalf("dense array meta mismatch: %+v", meta)
	}
	if NoDenseArrayMeta().Present {
		t.Fatalf("missing dense array meta should not be present")
	}
}

func TestRequireDenseArrayKinds(t *testing.T) {
	if _, err := RequireBoolMask(DTypeBool, 1, "soa.test"); err != nil {
		t.Fatalf("RequireBoolMask returned error: %v", err)
	}
	if _, err := RequireBoolMask(DTypeI64, 1, "soa.test"); err == nil {
		t.Fatalf("RequireBoolMask accepted i64")
	}
	if _, err := RequireI64Indices(DTypeI64, 2, "soa.test"); err != nil {
		t.Fatalf("RequireI64Indices returned error: %v", err)
	}
	if _, err := RequireI64Indices(DTypeBool, 2, "soa.test"); err == nil {
		t.Fatalf("RequireI64Indices accepted bool")
	}
}

func TestMaskQueryIsComparableShape(t *testing.T) {
	left := NewDenseArrayMeta(DTypeI64, 3)
	right := NewDenseArrayMeta(DTypeI64, 4)
	a := NewMaskQuery("x", ">", left, true, "y", right)
	b := NewMaskQuery("x", ">", left, true, "y", right)
	if a != b {
		t.Fatalf("equivalent mask queries differ: %+v vs %+v", a, b)
	}
	if a == NewMaskQuery("x", "<", left, true, "y", right) {
		t.Fatalf("mask queries with different operators matched")
	}
}
