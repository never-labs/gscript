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

func TestColumnCacheKey(t *testing.T) {
	key := NewColumnCacheKey("x", DTypeI64, 3)
	if !key.Matches("x", NewDenseArrayMeta(DTypeI64, 3)) {
		t.Fatalf("equivalent column cache key did not match")
	}
	if key.Matches("x", NewDenseArrayMeta(DTypeI64, 4)) {
		t.Fatalf("column cache key ignored version")
	}
	if key.Matches("y", NewDenseArrayMeta(DTypeI64, 3)) {
		t.Fatalf("column cache key ignored column")
	}
}

func TestResultMetaValid(t *testing.T) {
	if ResultMetaValid(NoDenseArrayMeta(), 1) {
		t.Fatalf("missing result meta should not be valid")
	}
	if !ResultMetaValid(NewDenseArrayMeta(DTypeBool, 7), 7) {
		t.Fatalf("matching result version should be valid")
	}
	if ResultMetaValid(NewDenseArrayMeta(DTypeBool, 7), 8) {
		t.Fatalf("stale result version should not be valid")
	}
}

func TestNextRingSlot(t *testing.T) {
	slot, next := NextRingSlot(3, 2)
	if slot != 1 || next != 4 {
		t.Fatalf("NextRingSlot(3,2) = (%d,%d), want (1,4)", slot, next)
	}
	slot, next = NextRingSlot(3, 0)
	if slot != 0 || next != 3 {
		t.Fatalf("NextRingSlot(3,0) = (%d,%d), want (0,3)", slot, next)
	}
}
