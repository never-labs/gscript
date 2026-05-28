package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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
		assertOpSpecTarget(t, op, "MatrixLoweredOp", spec.MatrixLoweredOp)
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
		if spec.MatrixLoweredOp != OpMax {
			t.Fatalf("%s MatrixLoweredOp default=%s, want OpMax", op, spec.MatrixLoweredOp)
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
		if _, ok := matrixLoweredOp(op); ok {
			t.Fatalf("%s should not report a matrix lowering target", op)
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

func TestOpSpecPolicyTableIntegrityCoversEveryPolicyVar(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	covered := make(map[string]bool)
	for _, table := range opSpecPolicyTables() {
		if covered[table.name] {
			t.Fatalf("duplicate OpSpec policy table registry entry: %s", table.name)
		}
		covered[table.name] = true
	}
	var missing []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "op_spec_policy_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, ident := range valueSpec.Names {
					if strings.HasPrefix(ident.Name, "op") && strings.HasSuffix(ident.Name, "Policies") &&
						!covered[ident.Name] {
						missing = append(missing, ident.Name)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan OpSpec policy files: %v", err)
	}
	if len(missing) > 0 {
		t.Fatalf("OpSpec policy integrity test does not cover policy vars: %s", strings.Join(missing, ", "))
	}
}
