//go:build leia_q

package bind

// Stress tests for the q bind-boundary value-scope lifetime discipline:
// GOGC=1 plus explicit GC hammering while result-heavy q paths run inside
// per-call ValueScopes. Any use-after-release bug surfaces as checksum
// corruption or a crash here.

import (
	goruntime "runtime"
	"runtime/debug"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func qScopeStressGC(n int) {
	for i := 0; i < n; i++ {
		goruntime.GC()
	}
}

// TestQSQLValueScopeLifetimeStress runs representative result-heavy qSQL
// shapes (vector exec, grouped aggregation, join) in tight scoped loops under
// GOGC=1, verifying checksums stay stable and the global root log stays
// bounded.
func TestQSQLValueScopeLifetimeStress(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)
	qClearCaches()
	defer qClearCaches()

	queries := []string{
		"exec size from trades where active=true,price>=100",
		"select notional:sum price*size,fills:count i,avg_px:avg price by sym,venue from trades where active=true",
		"select sym,price,bid,ask from trades ij quotes on sym=qsym order by price desc take 128",
	}
	const rows = 4096
	const iters = 120

	for _, query := range queries {
		query := query
		t.Run(query[:24], func(t *testing.T) {
			args := qSQLMatrixEnvArgs(t, rows, query)
			var want int64
			var baseline int64
			for i := 0; i < iters; i++ {
				scope := PushValueScope()
				out, err := qRunSQL("q.sql", args)
				if err != nil {
					scope.Release()
					t.Fatalf("iter %d: %v", i, err)
				}
				got := qSQLMatrixResultChecksum(t, out)
				scope.Release()
				if i == 0 {
					want = got
					baseline = runtime.GCRootLogSize()
				} else if got != want {
					t.Fatalf("iter %d: checksum %d, want %d (use-after-release?)", i, got, want)
				}
				if i%10 == 0 {
					qScopeStressGC(2)
				}
			}
			growth := runtime.GCRootLogSize() - baseline
			// Each unscoped run of these queries appends hundreds of roots;
			// scoped runs should add at most slab publishes and stray shared
			// roots.
			if growth > int64(iters)*16 {
				t.Fatalf("root log grew by %d over %d scoped queries", growth, iters)
			}
		})
	}
}

// TestQSQLValueScopeKeptResultSurvives keeps a scoped qSQL result alive past
// its producing scope (the embedder "hold the result" pattern) and verifies
// the kept value is intact after aggressive GC while later scoped queries
// drop their results.
func TestQSQLValueScopeKeptResultSurvives(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)
	qClearCaches()
	defer qClearCaches()

	const rows = 2048
	args := qSQLMatrixEnvArgs(t, rows, "exec size from trades where active=true,price>=100")

	scope := PushValueScope()
	kept, err := qRunSQL("q.sql", args)
	if err != nil {
		scope.Release()
		t.Fatal(err)
	}
	want := qSQLMatrixResultChecksum(t, kept)
	scope.Release(kept)

	// Churn: more scoped queries whose results all get dropped.
	for i := 0; i < 40; i++ {
		s := PushValueScope()
		out, err := qRunSQL("q.sql", args)
		if err != nil {
			s.Release()
			t.Fatal(err)
		}
		_ = qSQLMatrixResultChecksum(t, out)
		s.Release()
		if i%8 == 0 {
			qScopeStressGC(2)
		}
	}
	qScopeStressGC(4)

	if got := qSQLMatrixResultChecksum(t, kept); got != want {
		t.Fatalf("kept result corrupted after churn: %d want %d", got, want)
	}
}

// TestQSessionEvalValueScopeStress hammers the q.session.eval host path —
// internally scoped per call — with GOGC=1 across result-vector shapes.
func TestQSessionEvalValueScopeStress(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	session := qSessionValue()
	eval := session.RawGetString("eval").GoFunction()
	if eval == nil || eval.FastArg1 == nil {
		t.Fatal("q.session.eval missing FastArg1")
	}
	exprs := []string{
		"sum til 1000",
		"reverse til 257",
		"(til 100) + 1",
		"avg til 999",
	}
	var want [4]string
	for i, expr := range exprs {
		v, err := eval.FastArg1(StringValue(expr))
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		want[i] = v.String()
	}
	for i := 0; i < 200; i++ {
		expr := exprs[i%len(exprs)]
		v, err := eval.FastArg1(StringValue(expr))
		if err != nil {
			t.Fatalf("iter %d %s: %v", i, expr, err)
		}
		if got := v.String(); got != want[i%len(exprs)] {
			t.Fatalf("iter %d %s: %q want %q", i, expr, got, want[i%len(exprs)])
		}
		if i%20 == 0 {
			qScopeStressGC(2)
		}
	}
}
