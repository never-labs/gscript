package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func recordPairwiseDriverLoopSource(t *testing.T, steps string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "numeric", "nbody.gs"))
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

	requireRuntimeSpecializationInfo(t, DriverLoopRuntimeSpecializationCatalog(), "record_pairwise_numeric_loop")
	requireRuntimeSpecializationInfo(t, RecognizedDriverLoopRuntimeSpecializations(top, globals), "record_pairwise_numeric_loop")
	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseDriverLoopRuntimeSpecializations(top, globals), "record_pairwise_numeric_loop")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonDriverRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonDriverRecognized)
	}
	if diag.Specialization.Route != RuntimeSpecializationRouteDriverLoop ||
		diag.Specialization.Arity != runtimeSpecializationUnknownDriverLoopArity ||
		diag.Specialization.Results != runtimeSpecializationUnknownDriverLoopResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Specialization)
	}
}

func TestRecordPairwiseCallSiteNoResultRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, recordPairwiseDriverLoopSource(t, "1"))
	advance := findTestProtoByName(top, "advance")
	if advance == nil {
		t.Fatal("missing advance proto")
	}

	requireRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "record_pairwise_numeric")
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(advance), "record_pairwise_numeric")
	if !cachedCallSiteNoResultRuntimeSpecializationRecognized(advance, callSiteNoResultRuntimeSpecializationRecordPairwiseNumeric) {
		t.Fatal("record_pairwise_numeric rejected by no-result runtime specialization cache")
	}
	if advance.CallSiteNoResultRuntime == nil || advance.CallSiteNoResultRuntime.recognized == 0 {
		t.Fatal("no-result runtime specialization cache was not populated")
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseCallSiteRuntimeSpecializationProto(advance), "record_pairwise_numeric")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
	if diag.Specialization.Route != RuntimeSpecializationRouteCallSiteNoResult ||
		diag.Specialization.Arity != 1 ||
		diag.Specialization.Results != runtimeSpecializationCallSiteInPlaceResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Specialization)
	}
}

func TestRecordPairwiseCallSiteNoResultIgnoresBenchmarkMetadata(t *testing.T) {
	top := compileProto(t, recordPairwiseDriverLoopSource(t, "1"))
	advance := findTestProtoByName(top, "advance")
	if advance == nil {
		t.Fatal("missing advance proto")
	}
	advance.Name = "shape_only_pairwise_update"
	advance.Source = "host/generated/not-a-benchmark.gs"
	if !isRecordPairwiseNumericProto(advance) {
		t.Fatalf("record pairwise numeric should recognize bytecode shape independent of name/source: code=%d const=%d maxstack=%d", len(advance.Code), len(advance.Constants), advance.MaxStack)
	}
	if !cachedCallSiteNoResultRuntimeSpecializationRecognized(advance, callSiteNoResultRuntimeSpecializationRecordPairwiseNumeric) {
		t.Fatal("record_pairwise_numeric rejected by no-result runtime specialization cache after metadata rewrite")
	}
}

func TestRecordPairwiseDriverLoopIgnoresCalleeMetadata(t *testing.T) {
	top := compileProto(t, recordPairwiseDriverLoopSource(t, "1024"))
	advance := findTestProtoByName(top, "advance")
	if advance == nil {
		t.Fatal("missing advance proto")
	}
	advance.Name = "shape_only_pairwise_update"
	advance.Source = "host/generated/not-a-benchmark.gs"
	globals := map[string]*FuncProto{"advance": advance}
	requireRuntimeSpecializationInfo(t, RecognizedDriverLoopRuntimeSpecializations(top, globals), "record_pairwise_numeric_loop")
}

func TestRecordPairwiseDriverLoopRuntimeSpecializationMissingGlobals(t *testing.T) {
	top := compileProto(t, recordPairwiseDriverLoopSource(t, "1024"))

	rejectRuntimeSpecializationInfo(t, RecognizedDriverLoopRuntimeSpecializations(top, nil), "record_pairwise_numeric_loop")
	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseDriverLoopRuntimeSpecializations(top, nil), "record_pairwise_numeric_loop")
	if diag.Recognized || diag.Reason != runtimeSpecializationReasonMissingGlobalProtoMap {
		t.Fatalf("missing-global diagnostic = %+v, want %q", diag, runtimeSpecializationReasonMissingGlobalProtoMap)
	}
}

func TestRecordPairwiseDriverLoopRuntimeSpecializationRecordsSingleHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, recordPairwiseDriverLoopSource(t, "1024"))
	expectGlobalInt(t, globals, "N", 1024)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteDriverLoop, "record_pairwise_numeric_loop"); got != 1 {
		t.Fatalf("record_pairwise_numeric_loop structural hit count = %d, want 1", got)
	}
}

func TestRecordPairwiseCallSiteNoResultRuntimeSpecializationRecordsSingleHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, recordPairwiseDriverLoopSource(t, "1"))
	expectGlobalInt(t, globals, "N", 1)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteNoResult, "record_pairwise_numeric"); got != 1 {
		t.Fatalf("record_pairwise_numeric structural hit count = %d, want 1", got)
	}
}
