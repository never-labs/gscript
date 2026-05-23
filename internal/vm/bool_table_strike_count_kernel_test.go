package vm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func boolTableStrikeCountSource(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "suite", "sieve.gs"))
	if err != nil {
		t.Fatalf("read sieve benchmark: %v", err)
	}
	return string(src)
}

func TestBoolTableStrikeCountRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, boolTableStrikeCountSource(t))
	sieve := findTestProtoByName(top, "sieve")
	if sieve == nil {
		t.Fatal("missing sieve proto")
	}

	requireRuntimeSpecializationInfo(t, WholeCallRuntimeSpecializationCatalog(), "bool_table_strike_count")
	requireRuntimeSpecializationInfo(t, RecognizedWholeCallRuntimeSpecializations(sieve), "bool_table_strike_count")
	if !cachedRuntimeSpecializationRecognized(sieve, runtimeSpecializationBoolTableStrikeCount) {
		t.Fatal("bool_table_strike_count rejected by runtime specialization cache")
	}
	if sieve.RuntimeSpecialization == nil || sieve.RuntimeSpecialization.recognized == 0 {
		t.Fatal("runtime specialization cache was not populated")
	}
	if sieve.BoolTableStrikeCountKernel == nil || sieve.BoolTableStrikeCountKernel.spec == nil {
		t.Fatal("bool table strike-count proto-local spec was not generated")
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseWholeCallRuntimeSpecializationProto(sieve), "bool_table_strike_count")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
	if diag.Specialization.Route != RuntimeSpecializationRouteWholeCallValue ||
		diag.Specialization.Arity != 1 ||
		diag.Specialization.Results != runtimeSpecializationWholeCallSingleResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Specialization)
	}
}

func TestBoolTableStrikeCountRuntimeSpecializationRecordsHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
func sieve(n) {
    is_prime := {}
    for i := 2; i <= n; i++ {
        is_prime[i] = true
    }
    i := 2
    for i * i <= n {
        if is_prime[i] {
            j := i * i
            for j <= n {
                is_prime[j] = false
                j = j + i
            }
        }
        i = i + 1
    }
    count := 0
    for i := 2; i <= n; i++ {
        if is_prime[i] { count = count + 1 }
    }
    return count
}
result := sieve(100)
`)
	expectGlobalInt(t, globals, "result", 25)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteWholeCallValue, "bool_table_strike_count"); got != 1 {
		t.Fatalf("bool_table_strike_count structural hit count = %d, want 1", got)
	}
}
