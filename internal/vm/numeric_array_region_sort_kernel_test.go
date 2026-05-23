package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
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

	requireKernelInfo(t, WholeCallRuntimeSpecializationCatalog(), "numeric_array_region_sort")
	requireKernelInfo(t, RecognizedWholeCallRuntimeSpecializations(quicksort), "numeric_array_region_sort")
	if !cachedWholeCallNoResultRuntimeSpecializationRecognized(quicksort, wholeCallNoResultRuntimeSpecializationNumericArrayRegionSort) {
		t.Fatal("numeric_array_region_sort rejected by no-result runtime specialization cache")
	}
	if quicksort.WholeCallNoResultRuntime == nil || quicksort.WholeCallNoResultRuntime.recognized == 0 {
		t.Fatal("no-result runtime specialization cache was not populated")
	}

	diag := requireKernelDiagnostic(t, DiagnoseWholeCallRuntimeSpecializationProto(quicksort), "numeric_array_region_sort")
	if !diag.Recognized || diag.Reason != kernelReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, kernelReasonRecognized)
	}
	if diag.Kernel.Route != KernelRouteWholeCallNoResult ||
		diag.Kernel.Arity != 3 ||
		diag.Kernel.Results != kernelWholeCallInPlaceResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Kernel)
	}
}

func TestNumericArrayRegionSortNoResultRuntimeSpecializationRecordsHits(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, numericArrayRegionSortSource)
	expectGlobalBool(t, globals, "sorted", true)
	if got := runtimeStructuralKernelHitCount(stats, KernelRouteWholeCallNoResult, "numeric_array_region_sort"); got == 0 {
		t.Fatal("numeric_array_region_sort structural hit count = 0, want at least 1")
	}
}
