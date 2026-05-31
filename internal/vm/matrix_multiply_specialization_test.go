package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func matrixMultiplySource(t *testing.T, file string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "numeric", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return string(src)
}

func TestMatrixMultiplyRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, matrixMultiplySource(t, "matmul.gs"))
	matmul := findTestProtoByName(top, "matmul")
	if matmul == nil {
		t.Fatal("missing matmul proto")
	}

	requireRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "matrix_multiply")
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(matmul), "matrix_multiply")
	if !cachedRuntimeSpecializationRecognized(matmul, runtimeSpecializationMatrixMultiply) {
		t.Fatal("matrix_multiply rejected by runtime specialization cache")
	}
	if matmul.MatrixMultiplySpecialization == nil || matmul.MatrixMultiplySpecialization.spec == nil {
		t.Fatal("matrix_multiply proto-local spec was not generated")
	}
	if matmul.MatrixMultiplySpecialization.spec.kind != matrixMultiplySpecializationPlain {
		t.Fatalf("matrix_multiply kind = %d, want plain", matmul.MatrixMultiplySpecialization.spec.kind)
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseCallSiteRuntimeSpecializationProto(matmul), "matrix_multiply")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
}

func TestMatrixMultiplyRuntimeSpecializationDenseVariant(t *testing.T) {
	top := compileProto(t, matrixMultiplySource(t, "matmul_dense.gs"))
	matmul := findTestProtoByName(top, "matmul")
	if matmul == nil {
		t.Fatal("missing dense matmul proto")
	}
	spec, ok := matrixMultiplySpecializationSpecForProto(matmul)
	if !ok {
		t.Fatal("dense matrix_multiply spec not generated")
	}
	if spec.kind != matrixMultiplySpecializationDense {
		t.Fatalf("dense matrix_multiply kind = %d, want dense", spec.kind)
	}
}

func TestMatrixMultiplyRuntimeSpecializationIgnoresBenchmarkMetadata(t *testing.T) {
	for _, tc := range []struct {
		file string
		kind matrixMultiplySpecializationKind
	}{
		{file: "matmul.gs", kind: matrixMultiplySpecializationPlain},
		{file: "matmul_dense.gs", kind: matrixMultiplySpecializationDense},
	} {
		top := compileProto(t, matrixMultiplySource(t, tc.file))
		matmul := findTestProtoByName(top, "matmul")
		if matmul == nil {
			t.Fatalf("%s: missing matmul proto", tc.file)
		}
		matmul.Name = "shape_only_matrix_product"
		matmul.Source = "host/generated/not-a-benchmark.gs"
		spec, ok := matrixMultiplySpecializationSpecForProto(matmul)
		if !ok {
			t.Fatalf("%s: matrix multiply should recognize bytecode shape independent of name/source", tc.file)
		}
		if spec.kind != tc.kind {
			t.Fatalf("%s: kind = %d, want %d", tc.file, spec.kind, tc.kind)
		}
	}
}

func TestMatrixMultiplyRuntimeSpecializationRecordsHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	src := matrixMultiplySource(t, "matmul.gs")
	src = strings.Replace(src, "N := 60", "N := 8", 1)
	src = strings.Replace(src, "REPS := 20", "REPS := 1", 1)
	globals := compileAndRun(t, src)
	if v := globals["result"]; !v.IsNumber() {
		t.Fatalf("result = %s (%v), want number", v.TypeName(), v)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "matrix_multiply"); got == 0 {
		t.Fatal("matrix_multiply structural hit count = 0, want at least 1")
	}
}

func TestDenseMatrixMultiplyTransposedRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, matrixMultiplySource(t, "matmul_dense_tb.gs"))
	matmul := findTestProtoByName(top, "matmul")
	if matmul == nil {
		t.Fatal("missing dense transposed matmul proto")
	}

	requireRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "dense_matrix_multiply_transposed")
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(matmul), "dense_matrix_multiply_transposed")
	if !cachedCallSiteNoResultRuntimeSpecializationRecognized(matmul, callSiteNoResultRuntimeSpecializationDenseMatrixMultiplyTransposed) {
		t.Fatal("dense_matrix_multiply_transposed rejected by no-result runtime specialization cache")
	}
	if matmul.DenseMatrixMultiplyTBSpecialization == nil || matmul.DenseMatrixMultiplyTBSpecialization.spec == nil {
		t.Fatal("dense transposed matrix multiply proto-local spec was not generated")
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseCallSiteRuntimeSpecializationProto(matmul), "dense_matrix_multiply_transposed")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
	if diag.Specialization.Route != RuntimeSpecializationRouteCallSiteNoResult ||
		diag.Specialization.Arity != 4 ||
		diag.Specialization.Results != runtimeSpecializationCallSiteInPlaceResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Specialization)
	}
}

func TestDenseMatrixMultiplyTransposedRuntimeSpecializationIgnoresBenchmarkMetadata(t *testing.T) {
	top := compileProto(t, matrixMultiplySource(t, "matmul_dense_tb.gs"))
	matmul := findTestProtoByName(top, "matmul")
	if matmul == nil {
		t.Fatal("missing dense transposed matmul proto")
	}
	matmul.Name = "shape_only_transposed_matrix_product"
	matmul.Source = "host/generated/not-a-benchmark.gs"
	if !isDenseMatrixMultiplyTransposedProto(matmul) {
		t.Fatal("dense matrix multiply transposed should recognize bytecode shape independent of name/source")
	}
}

func TestDenseMatrixMultiplyTransposedRuntimeSpecializationRecordsHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	src := matrixMultiplySource(t, "matmul_dense_tb.gs")
	src = strings.Replace(src, "N := 300", "N := 12", 1)
	globals := compileAndRun(t, src)
	if v := globals["c"]; !v.IsTable() {
		t.Fatalf("c = %s (%v), want table", v.TypeName(), v)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteNoResult, "dense_matrix_multiply_transposed"); got != 1 {
		t.Fatalf("dense_matrix_multiply_transposed structural hit count = %d, want 1", got)
	}
}
