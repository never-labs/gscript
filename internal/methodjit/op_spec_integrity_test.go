package methodjit

import (
	"reflect"
	"testing"
)

func TestOpSpecPolicyTablesDoNotExceedOpSpace(t *testing.T) {
	for _, table := range opSpecPolicyTables() {
		if got := reflect.ValueOf(table.table).Len(); got > int(OpMax) {
			t.Fatalf("%s has length %d beyond OpMax %d", table.name, got, OpMax)
		}
	}
}

func TestOpSpecLookupAndTargetIntegrity(t *testing.T) {
	seenNames := make(map[string]Op, int(OpMax))
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if prior, exists := seenNames[spec.Name]; exists {
			t.Fatalf("duplicate OpSpec name %q for %s and %s", spec.Name, prior, op)
		}
		seenNames[spec.Name] = op
		if got, ok := OpByName(spec.Name); !ok || got != op {
			t.Fatalf("OpByName(%q)=(%s,%v), want (%s,true)", spec.Name, got, ok, op)
		}
		assertOpSpecTarget(t, op, "TypeSpecializeIntOp", spec.TypeSpecializeIntOp)
		assertOpSpecTarget(t, op, "TypeSpecializeFloatOp", spec.TypeSpecializeFloatOp)
		assertOpSpecTarget(t, op, "TypeSpecializeStringOp", spec.TypeSpecializeStringOp)
		assertOpSpecTarget(t, op, "RawIntSpecializedOp", spec.RawIntSpecializedOp)
		assertOpSpecTarget(t, op, "ExactIntNarrowOp", spec.ExactIntNarrowOp)
		assertOpSpecTarget(t, op, "BoxedFallbackOp", spec.BoxedFallbackOp)
		assertOpSpecTarget(t, op, "FieldSvalsLoweredOp", spec.FieldSvalsLoweredOp)
		assertOpSpecTarget(t, op, "FieldNumFusionLoweredOp", spec.FieldNumFusionLoweredOp)
		assertOpSpecTarget(t, op, "MatrixLoweredOp", spec.MatrixLoweredOp)
		assertOpSpecTarget(t, op, "MatrixRowLoweredOp", spec.MatrixRowLoweredOp)
		assertOpSpecTarget(t, op, "MatrixRowConstLoweredOp", spec.MatrixRowConstLoweredOp)
		assertOpSpecTarget(t, op, "TableArrayLoweredOp", spec.TableArrayLoweredOp)
		assertOpSpecTarget(t, op, "TableArrayNestedLoweredOp", spec.TableArrayNestedLoweredOp)
		assertOpSpecTarget(t, op, "CallFloorProjectionOp", spec.CallFloorProjectionOp)
		assertOpSpecTarget(t, op, "FieldCallFloorProjectionOp", spec.FieldCallFloorProjectionOp)
		assertOpSpecTarget(t, op, "FieldCalleeGuardLoweredOp", spec.FieldCalleeGuardLoweredOp)
	}
	if len(seenNames) != int(OpMax) {
		t.Fatalf("OpSpec name lookup saw %d names, want %d", len(seenNames), OpMax)
	}
}

func TestOpSpecUnsetSentinelsDoNotLookLikePolicies(t *testing.T) {
	for _, op := range []Op{OpConstInt, OpConstBool, OpNop, OpReturn} {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if spec.TypeSpecializeIntOp != OpMax || spec.TypeSpecializeFloatOp != OpMax || spec.TypeSpecializeStringOp != OpMax {
			t.Fatalf("%s has unexpected type-specialization defaults: int=%s float=%s string=%s",
				op, spec.TypeSpecializeIntOp, spec.TypeSpecializeFloatOp, spec.TypeSpecializeStringOp)
		}
		if spec.RawIntSpecializedOp != OpMax {
			t.Fatalf("%s RawIntSpecializedOp default=%s, want OpMax", op, spec.RawIntSpecializedOp)
		}
		if spec.ExactIntNarrowOp != OpMax {
			t.Fatalf("%s ExactIntNarrowOp default=%s, want OpMax", op, spec.ExactIntNarrowOp)
		}
		if spec.BoxedFallbackOp != OpMax {
			t.Fatalf("%s BoxedFallbackOp default=%s, want OpMax", op, spec.BoxedFallbackOp)
		}
		if spec.FieldSvalsLoweredOp != OpMax {
			t.Fatalf("%s FieldSvalsLoweredOp default=%s, want OpMax", op, spec.FieldSvalsLoweredOp)
		}
		if spec.FieldNumFusionLoweredOp != OpMax {
			t.Fatalf("%s FieldNumFusionLoweredOp default=%s, want OpMax", op, spec.FieldNumFusionLoweredOp)
		}
		if spec.MatrixLoweredOp != OpMax {
			t.Fatalf("%s MatrixLoweredOp default=%s, want OpMax", op, spec.MatrixLoweredOp)
		}
		if spec.MatrixRowLoweredOp != OpMax {
			t.Fatalf("%s MatrixRowLoweredOp default=%s, want OpMax", op, spec.MatrixRowLoweredOp)
		}
		if spec.MatrixRowConstLoweredOp != OpMax {
			t.Fatalf("%s MatrixRowConstLoweredOp default=%s, want OpMax", op, spec.MatrixRowConstLoweredOp)
		}
		if spec.CallUserArgStart != -1 {
			t.Fatalf("%s CallUserArgStart default=%d, want -1", op, spec.CallUserArgStart)
		}
		if spec.TableArrayKeyArgIndex != -1 {
			t.Fatalf("%s TableArrayKeyArgIndex default=%d, want -1", op, spec.TableArrayKeyArgIndex)
		}
		if spec.TableArrayTableArgIndex != -1 || spec.TableArrayDataArgIndex != -1 || spec.TableArrayLenArgIndex != -1 {
			t.Fatalf("%s table-array access layout default = table %d data %d len %d, want all -1",
				op, spec.TableArrayTableArgIndex, spec.TableArrayDataArgIndex, spec.TableArrayLenArgIndex)
		}
		if spec.TableArrayLoweredOp != OpMax {
			t.Fatalf("%s TableArrayLoweredOp default=%s, want OpMax", op, spec.TableArrayLoweredOp)
		}
		if spec.TableArrayNestedLoweredOp != OpMax {
			t.Fatalf("%s TableArrayNestedLoweredOp default=%s, want OpMax", op, spec.TableArrayNestedLoweredOp)
		}
		if spec.CallFloorProjectionOp != OpMax || spec.FieldCallFloorProjectionOp != OpMax {
			t.Fatalf("%s call projection defaults=%s/%s, want OpMax/OpMax",
				op, spec.CallFloorProjectionOp, spec.FieldCallFloorProjectionOp)
		}
		if spec.FieldCalleeGuardLoweredOp != OpMax {
			t.Fatalf("%s FieldCalleeGuardLoweredOp default=%s, want OpMax", op, spec.FieldCalleeGuardLoweredOp)
		}
		if spec.ClosureScalarLocalUseArgIndex != -1 || spec.ClosureScalarLoadClosureArgIndex != -1 ||
			spec.ClosureScalarStoreClosureArgIndex != -1 || spec.ClosureScalarStoreValueArgIndex != -1 {
			t.Fatalf("%s closure scalar arg defaults=%d/%d/%d/%d, want all -1",
				op, spec.ClosureScalarLocalUseArgIndex, spec.ClosureScalarLoadClosureArgIndex,
				spec.ClosureScalarStoreClosureArgIndex, spec.ClosureScalarStoreValueArgIndex)
		}
		if spec.LocalStringArrayTableArgIndex != -1 {
			t.Fatalf("%s LocalStringArrayTableArgIndex default=%d, want -1", op, spec.LocalStringArrayTableArgIndex)
		}
		if spec.BoolTableFillStoreTableArg != -1 || spec.BoolTableFillStoreKeyArg != -1 || spec.BoolTableFillStoreValueArg != -1 {
			t.Fatalf("%s bool-fill store arg defaults=%d/%d/%d, want all -1",
				op, spec.BoolTableFillStoreTableArg, spec.BoolTableFillStoreKeyArg, spec.BoolTableFillStoreValueArg)
		}
		if spec.LoadElimTableCacheKeyArgIndex != -1 || spec.LoadElimTableCacheValueArgIndex != -1 {
			t.Fatalf("%s load-elim table-cache arg defaults=%d/%d, want both -1",
				op, spec.LoadElimTableCacheKeyArgIndex, spec.LoadElimTableCacheValueArgIndex)
		}
		if _, ok := exactIntNarrowOp(op); ok {
			t.Fatalf("%s should not report an exact int-narrow target", op)
		}
		if _, ok := boxedFallbackOp(op); ok {
			t.Fatalf("%s should not report a boxed fallback target", op)
		}
		if _, ok := rawIntSpecializedOp(op); ok {
			t.Fatalf("%s should not report a raw-int specialization target", op)
		}
		if _, ok := fieldSvalsLoweredOp(op); ok {
			t.Fatalf("%s should not report a FieldSvals lowering target", op)
		}
		if _, ok := fieldNumFusionLoweredOp(op); ok {
			t.Fatalf("%s should not report a field numeric-fusion lowering target", op)
		}
		if _, ok := matrixLoweredOp(op); ok {
			t.Fatalf("%s should not report a matrix lowering target", op)
		}
		if _, ok := matrixRowLoweredOp(op); ok {
			t.Fatalf("%s should not report a matrix row lowering target", op)
		}
		if _, ok := matrixRowConstLoweredOp(op); ok {
			t.Fatalf("%s should not report a matrix row const lowering target", op)
		}
		if _, ok := callUserArgStart(op); ok {
			t.Fatalf("%s should not report a call-user arg start", op)
		}
		if _, ok := tableArrayKeyArgIndex(op); ok {
			t.Fatalf("%s should not report a table-array key arg index", op)
		}
		if _, ok := tableArrayAccessLayoutForOp(op); ok {
			t.Fatalf("%s should not report a table-array access layout", op)
		}
		if _, ok := tableArrayLoweredOp(op); ok {
			t.Fatalf("%s should not report a table-array lowering target", op)
		}
		if _, ok := tableArrayNestedLoweredOp(op); ok {
			t.Fatalf("%s should not report a table-array nested lowering target", op)
		}
		if _, ok := callFloorProjectionOp(op); ok {
			t.Fatalf("%s should not report a call-floor projection target", op)
		}
		if _, ok := fieldCallFloorProjectionOp(op); ok {
			t.Fatalf("%s should not report a field-call-floor projection target", op)
		}
		if _, ok := fieldCalleeGuardLoweredOp(op); ok {
			t.Fatalf("%s should not report a field-callee guard lowering target", op)
		}
	}
}

func assertOpSpecTarget(t *testing.T, owner Op, field string, target Op) {
	t.Helper()
	if target == 0 || target == OpMax {
		return
	}
	if target < 0 || target >= OpMax {
		t.Fatalf("%s.%s targets invalid op %d", owner, field, target)
	}
	if _, ok := target.Spec(); !ok {
		t.Fatalf("%s.%s targets op %d without OpSpec", owner, field, target)
	}
}
