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

func TestRecordPairwiseWholeCallNoResultRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, recordPairwiseDriverLoopSource(t, "1"))
	advance := findTestProtoByName(top, "advance")
	if advance == nil {
		t.Fatal("missing advance proto")
	}

	requireKernelInfo(t, WholeCallKernelCatalog(), "record_pairwise_numeric")
	requireKernelInfo(t, RecognizedWholeCallKernels(advance), "record_pairwise_numeric")
	if !cachedWholeCallNoResultRuntimeSpecializationRecognized(advance, wholeCallNoResultRuntimeSpecializationRecordPairwiseNumeric) {
		t.Fatal("record_pairwise_numeric rejected by no-result runtime specialization cache")
	}
	if advance.WholeCallNoResultRuntime == nil || advance.WholeCallNoResultRuntime.recognized == 0 {
		t.Fatal("no-result runtime specialization cache was not populated")
	}

	diag := requireKernelDiagnostic(t, DiagnoseWholeCallKernelProto(advance), "record_pairwise_numeric")
	if !diag.Recognized || diag.Reason != kernelReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, kernelReasonRecognized)
	}
	if diag.Kernel.Route != KernelRouteWholeCallNoResult ||
		diag.Kernel.Arity != 1 ||
		diag.Kernel.Results != kernelWholeCallInPlaceResultCount {
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

func TestRecordPairwiseWholeCallNoResultRuntimeSpecializationRecordsSingleHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, recordPairwiseDriverLoopSource(t, "1"))
	expectGlobalInt(t, globals, "N", 1)
	if got := runtimeStructuralKernelHitCount(stats, KernelRouteWholeCallNoResult, "record_pairwise_numeric"); got != 1 {
		t.Fatalf("record_pairwise_numeric structural hit count = %d, want 1", got)
	}
}
