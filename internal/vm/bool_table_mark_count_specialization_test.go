package vm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func boolTableMarkCountSource(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "suite", "sieve.gs"))
	if err != nil {
		t.Fatalf("read sieve benchmark: %v", err)
	}
	return string(src)
}

func TestBoolTableMarkCountRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, boolTableMarkCountSource(t))
	sieve := findTestProtoByName(top, "sieve")
	if sieve == nil {
		t.Fatal("missing sieve proto")
	}

	requireRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "bool_table_mark_count")
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(sieve), "bool_table_mark_count")
	if !cachedRuntimeSpecializationRecognized(sieve, runtimeSpecializationBoolTableMarkCount) {
		t.Fatal("bool_table_mark_count rejected by runtime specialization cache")
	}
	if sieve.RuntimeSpecialization == nil || sieve.RuntimeSpecialization.recognized == 0 {
		t.Fatal("runtime specialization cache was not populated")
	}
	if sieve.BoolTableMarkCountSpecialization == nil || sieve.BoolTableMarkCountSpecialization.spec == nil {
		t.Fatal("bool table mark-count proto-local spec was not generated")
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseCallSiteRuntimeSpecializationProto(sieve), "bool_table_mark_count")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
	if diag.Specialization.Route != RuntimeSpecializationRouteCallSiteValue ||
		diag.Specialization.Arity != 1 ||
		diag.Specialization.Results != runtimeSpecializationCallSiteSingleResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Specialization)
	}
}

func TestBoolTableMarkCountIgnoresBenchmarkMetadata(t *testing.T) {
	top := compileProto(t, boolTableMarkCountSource(t))
	sieve := findTestProtoByName(top, "sieve")
	if sieve == nil {
		t.Fatal("missing sieve proto")
	}
	sieve.Name = "shape_only_bool_table_count"
	sieve.Source = "host/generated/not-a-benchmark.gs"
	if !isBoolTableMarkCountProto(sieve) {
		t.Fatalf("bool table mark count should recognize bytecode shape independent of name/source: code=%d const=%d maxstack=%d", len(sieve.Code), len(sieve.Constants), sieve.MaxStack)
	}
}

func TestBoolTableMarkCountRuntimeSpecializationRecordsHit(t *testing.T) {
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
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "bool_table_mark_count"); got != 1 {
		t.Fatalf("bool_table_mark_count structural hit count = %d, want 1", got)
	}
}
