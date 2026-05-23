package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func recordPairwiseDriverLoopSource(t *testing.T, steps string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "suite", "nbody.gs"))
	if err != nil {
		t.Fatalf("read nbody benchmark: %v", err)
	}
	return strings.Replace(string(src), "N := 2000000", "N := "+steps, 1)
}

func TestRecordPairwiseDriverLoopRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, recordPairwiseDriverLoopSource(t, "1024"))
	advance := findTestProtoByName(top, "advance")
	if advance == nil {
		t.Fatal("missing advance proto")
	}
	globals := map[string]*FuncProto{"advance": advance}

	requireKernelInfo(t, DriverLoopKernelCatalog(), "record_pairwise_numeric_loop")
	requireKernelInfo(t, RecognizedDriverLoopKernels(top, globals), "record_pairwise_numeric_loop")
	diag := requireKernelDiagnostic(t, DiagnoseDriverLoopKernels(top, globals), "record_pairwise_numeric_loop")
	if !diag.Recognized || diag.Reason != kernelReasonDriverRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, kernelReasonDriverRecognized)
	}
	if diag.Kernel.Route != KernelRouteDriverLoop ||
		diag.Kernel.Arity != kernelUnknownDriverLoopArity ||
		diag.Kernel.Results != kernelUnknownDriverLoopResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Kernel)
	}
}

func TestRecordPairwiseDriverLoopRuntimeSpecializationMissingGlobals(t *testing.T) {
	top := compileProto(t, recordPairwiseDriverLoopSource(t, "1024"))

	rejectKernelInfo(t, RecognizedDriverLoopKernels(top, nil), "record_pairwise_numeric_loop")
	diag := requireKernelDiagnostic(t, DiagnoseDriverLoopKernels(top, nil), "record_pairwise_numeric_loop")
	if diag.Recognized || diag.Reason != kernelReasonMissingGlobalProtoMap {
		t.Fatalf("missing-global diagnostic = %+v, want %q", diag, kernelReasonMissingGlobalProtoMap)
	}
}

func TestRecordPairwiseDriverLoopRuntimeSpecializationRecordsSingleHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, recordPairwiseDriverLoopSource(t, "1024"))
	expectGlobalInt(t, globals, "N", 1024)
	if got := runtimeStructuralKernelHitCount(stats, KernelRouteDriverLoop, "record_pairwise_numeric_loop"); got != 1 {
		t.Fatalf("record_pairwise_numeric_loop structural hit count = %d, want 1", got)
	}
}
