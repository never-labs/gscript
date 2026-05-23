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

	requireKernelInfo(t, WholeCallKernelCatalog(), "matrix_multiply")
	requireKernelInfo(t, RecognizedWholeCallKernels(matmul), "matrix_multiply")
	if !cachedRuntimeSpecializationRecognized(matmul, runtimeSpecializationMatrixMultiply) {
		t.Fatal("matrix_multiply rejected by runtime specialization cache")
	}
	if matmul.MatrixMultiplyKernel == nil || matmul.MatrixMultiplyKernel.spec == nil {
		t.Fatal("matrix_multiply proto-local spec was not generated")
	}
	if matmul.MatrixMultiplyKernel.spec.kind != matrixMultiplyKernelPlain {
		t.Fatalf("matrix_multiply kind = %d, want plain", matmul.MatrixMultiplyKernel.spec.kind)
	}
	if got := cachedWholeCallKernelBits(matmul); got != 0 {
		t.Fatalf("matrix_multiply still recognized by legacy whole-call bits: %#x", got)
	}

	diag := requireKernelDiagnostic(t, DiagnoseWholeCallKernelProto(matmul), "matrix_multiply")
	if !diag.Recognized || diag.Reason != kernelReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, kernelReasonRecognized)
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
	if got := runtimeStructuralKernelHitCount(stats, KernelRouteWholeCallValue, "matrix_multiply"); got == 0 {
		t.Fatal("matrix_multiply structural hit count = 0, want at least 1")
	}
}
