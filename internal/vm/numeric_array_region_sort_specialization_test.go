package vm

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

const numericArrayRegionSortSource = `
func quicksort(arr, lo, hi) {
    if lo >= hi { return }
    pivot := arr[hi]
    i := lo
    for j := lo; j < hi; j++ {
        if arr[j] <= pivot {
            t := arr[i]
            arr[i] = arr[j]
            arr[j] = t
            i = i + 1
        }
    }
    t := arr[i]
    arr[i] = arr[hi]
    arr[hi] = t
    quicksort(arr, lo, i - 1)
    quicksort(arr, i + 1, hi)
}

arr := {4, 1, 3, 2}
quicksort(arr, 1, 4)
sorted := arr[1] == 1 && arr[2] == 2 && arr[3] == 3 && arr[4] == 4
`

func TestNumericArrayRegionSortNoResultRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, numericArrayRegionSortSource)
	quicksort := findTestProtoByName(top, "quicksort")
	if quicksort == nil {
		t.Fatal("missing quicksort proto")
	}

	requireRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "numeric_array_region_sort")
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(quicksort), "numeric_array_region_sort")
	if !cachedCallSiteNoResultRuntimeSpecializationRecognized(quicksort, callSiteNoResultRuntimeSpecializationNumericArrayRegionSort) {
		t.Fatal("numeric_array_region_sort rejected by no-result runtime specialization cache")
	}
	if quicksort.RuntimeSpecs.CallSiteNoResultRuntime == nil || quicksort.RuntimeSpecs.CallSiteNoResultRuntime.recognized == 0 {
		t.Fatal("no-result runtime specialization cache was not populated")
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseCallSiteRuntimeSpecializationProto(quicksort), "numeric_array_region_sort")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
	if diag.Specialization.Route != RuntimeSpecializationRouteCallSiteNoResult ||
		diag.Specialization.Arity != 3 ||
		diag.Specialization.Results != runtimeSpecializationCallSiteInPlaceResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Specialization)
	}
}

func TestNumericArrayRegionSortIgnoresBenchmarkMetadata(t *testing.T) {
	top := compileProto(t, numericArrayRegionSortSource)
	quicksort := findTestProtoByName(top, "quicksort")
	if quicksort == nil {
		t.Fatal("missing quicksort proto")
	}
	quicksort.Name = "runtime_generated_region_sort"
	quicksort.Source = "host/generated/not-a-benchmark.gs"

	if !isNumericArrayRegionSortProto(quicksort) {
		t.Fatal("numeric_array_region_sort recognizer should depend on bytecode shape, not proto metadata")
	}
	if !cachedCallSiteNoResultRuntimeSpecializationRecognized(quicksort, callSiteNoResultRuntimeSpecializationNumericArrayRegionSort) {
		t.Fatal("numeric_array_region_sort cache rejected metadata-renamed proto")
	}
}

func TestNumericArrayRegionSortNoResultRuntimeSpecializationRecordsHits(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, numericArrayRegionSortSource)
	expectGlobalBool(t, globals, "sorted", true)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteNoResult, "numeric_array_region_sort"); got == 0 {
		t.Fatal("numeric_array_region_sort structural hit count = 0, want at least 1")
	}
}
