package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func matrixMultiplySource(t *testing.T, file string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "suite", file))
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

	requireRuntimeSpecializationInfo(t, WholeCallRuntimeSpecializationCatalog(), "matrix_multiply")
	requireRuntimeSpecializationInfo(t, RecognizedWholeCallRuntimeSpecializations(matmul), "matrix_multiply")
	if !cachedRuntimeSpecializationRecognized(matmul, runtimeSpecializationMatrixMultiply) {
		t.Fatal("matrix_multiply rejected by runtime specialization cache")
	}
	if matmul.MatrixMultiplyKernel == nil || matmul.MatrixMultiplyKernel.spec == nil {
		t.Fatal("matrix_multiply proto-local spec was not generated")
	}
	if matmul.MatrixMultiplyKernel.spec.kind != matrixMultiplyKernelPlain {
		t.Fatalf("matrix_multiply kind = %d, want plain", matmul.MatrixMultiplyKernel.spec.kind)
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseWholeCallRuntimeSpecializationProto(matmul), "matrix_multiply")
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
	spec, ok := matrixMultiplyKernelSpecForProto(matmul)
	if !ok {
		t.Fatal("dense matrix_multiply spec not generated")
	}
	if spec.kind != matrixMultiplyKernelDense {
		t.Fatalf("dense matrix_multiply kind = %d, want dense", spec.kind)
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
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteWholeCallValue, "matrix_multiply"); got == 0 {
		t.Fatal("matrix_multiply structural hit count = 0, want at least 1")
	}
}

func TestDenseMatrixMultiplyTransposedRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, matrixMultiplySource(t, "matmul_dense_tb.gs"))
	matmul := findTestProtoByName(top, "matmul")
	if matmul == nil {
		t.Fatal("missing dense transposed matmul proto")
	}

	requireRuntimeSpecializationInfo(t, WholeCallRuntimeSpecializationCatalog(), "dense_matrix_multiply_transposed")
	requireRuntimeSpecializationInfo(t, RecognizedWholeCallRuntimeSpecializations(matmul), "dense_matrix_multiply_transposed")
	if !cachedWholeCallNoResultRuntimeSpecializationRecognized(matmul, wholeCallNoResultRuntimeSpecializationDenseMatrixMultiplyTransposed) {
		t.Fatal("dense_matrix_multiply_transposed rejected by no-result runtime specialization cache")
	}
	if matmul.DenseMatrixMultiplyTBKernel == nil || matmul.DenseMatrixMultiplyTBKernel.spec == nil {
		t.Fatal("dense transposed matrix multiply proto-local spec was not generated")
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseWholeCallRuntimeSpecializationProto(matmul), "dense_matrix_multiply_transposed")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
	if diag.Specialization.Route != RuntimeSpecializationRouteWholeCallNoResult ||
		diag.Specialization.Arity != 4 ||
		diag.Specialization.Results != runtimeSpecializationWholeCallInPlaceResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Specialization)
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
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteWholeCallNoResult, "dense_matrix_multiply_transposed"); got != 1 {
		t.Fatalf("dense_matrix_multiply_transposed structural hit count = %d, want 1", got)
	}
}
