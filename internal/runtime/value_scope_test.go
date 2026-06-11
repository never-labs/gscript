package runtime

import (
	goruntime "runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"
)

func valueScopeSupported() bool { return getgptr() != 0 }

func gcHammer(n int) {
	for i := 0; i < n; i++ {
		goruntime.GC()
	}
}

func makeScopeTestSoA(t testing.TB, n int, seed float64) *SoA {
	t.Helper()
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = seed + float64(i)
	}
	s, err := NewSoA(map[string]*DenseArray{"x": NewDenseArrayF64Owned(vals)})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	return s
}

func soaChecksum(t testing.TB, v Value) float64 {
	t.Helper()
	s := v.SoA()
	if s == nil {
		t.Fatalf("value is not a SoA: %s", v.TypeName())
	}
	col, ok := s.Column("x")
	if !ok {
		t.Fatalf("missing column x")
	}
	vals, ok := col.F64()
	if !ok {
		t.Fatalf("column x is not f64")
	}
	var sum float64
	for _, f := range vals {
		sum += f
	}
	return sum
}

// TestValueScopeCapturesAppends verifies that scope-created roots land in the
// scope buffer rather than the global log.
func TestValueScopeCapturesAppends(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	scope := PushValueScope()
	if !scope.Active() {
		t.Fatal("PushValueScope returned inert handle on supported platform")
	}
	before := GCRootLogSize()
	for i := 0; i < 64; i++ {
		_ = SoAValue(makeScopeTestSoA(t, 4, float64(i)))
		_ = LazyStringValue(StringValue(strings.Repeat("a", 80)), StringValue(strings.Repeat("b", 80)))
	}
	captured := scope.CapturedRootCount()
	grew := GCRootLogSize() - before
	scope.Release()
	if captured < 64 {
		t.Fatalf("scope captured %d roots, want >= 64", captured)
	}
	// Slab publishes and other shared roots may add a handful of global
	// entries, but per-object roots must not.
	if grew > 32 {
		t.Fatalf("global root log grew by %d during scope, want <= 32", grew)
	}
}

// TestValueScopeReleaseKeepsResultTree verifies that a kept result keeps every
// transitively reachable scope-created object alive across aggressive GC.
func TestValueScopeReleaseKeepsResultTree(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	scope := PushValueScope()
	tbl := NewTable()
	tbl.RawSetString("vec", SoAValue(makeScopeTestSoA(t, 128, 1)))
	tbl.RawSetString("lazy", LazyStringValue(
		StringValue(strings.Repeat("x", 100)), StringValue(strings.Repeat("y", 100))))
	inner := NewTable()
	inner.RawSetString("deep", SoAValue(makeScopeTestSoA(t, 16, 100)))
	tbl.RawSetString("nested", TableValue(inner))
	out := TableValue(tbl)
	scope.Release(out)

	gcHammer(6)

	wantVec := 0.0
	for i := 0; i < 128; i++ {
		wantVec += 1 + float64(i)
	}
	if got := soaChecksum(t, out.Table().RawGetString("vec")); got != wantVec {
		t.Fatalf("vec checksum = %v, want %v", got, wantVec)
	}
	lazy := out.Table().RawGetString("lazy")
	if got := lazy.String(); got != strings.Repeat("x", 100)+strings.Repeat("y", 100) {
		t.Fatalf("lazy string corrupted: %q", got)
	}
	wantDeep := 0.0
	for i := 0; i < 16; i++ {
		wantDeep += 100 + float64(i)
	}
	deep := out.Table().RawGetString("nested").Table().RawGetString("deep")
	if got := soaChecksum(t, deep); got != wantDeep {
		t.Fatalf("deep checksum = %v, want %v", got, wantDeep)
	}
}

// TestValueScopeReleaseDropsDeadObjects proves dropped roots actually allow
// collection (finalizer fires).
func TestValueScopeReleaseDropsDeadObjects(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	collected := make(chan struct{}, 1)
	func() {
		scope := PushValueScope()
		s := makeScopeTestSoA(t, 1024, 7)
		goruntime.SetFinalizer(s, func(*SoA) { collected <- struct{}{} })
		_ = SoAValue(s)
		scope.Release()
	}()
	deadline := time.After(5 * time.Second)
	for {
		goruntime.GC()
		select {
		case <-collected:
			return
		case <-deadline:
			t.Fatal("scope-dropped SoA was never collected: root leaked")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// TestValueScopeNested verifies inner keeps republish into the outer scope and
// die when the outer scope drops them, while outer-kept values survive.
func TestValueScopeNested(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	outer := PushValueScope()
	var innerKept Value
	func() {
		inner := PushValueScope()
		innerKept = SoAValue(makeScopeTestSoA(t, 8, 3))
		inner.Release(innerKept)
	}()
	if outer.CapturedRootCount() == 0 {
		t.Fatal("inner Release(keep) did not republish into outer scope")
	}
	// Value usable while outer scope still holds it.
	if got := soaChecksum(t, innerKept); got == 0 {
		t.Fatalf("unexpected checksum %v", got)
	}
	outer.Release(innerKept)
	gcHammer(4)
	want := 0.0
	for i := 0; i < 8; i++ {
		want += 3 + float64(i)
	}
	if got := soaChecksum(t, innerKept); got != want {
		t.Fatalf("checksum after nested release = %v, want %v", got, want)
	}
}

// TestValueScopeBarrierRoutesGlobal verifies barrier scopes send appends to
// the global log so script-published values keep append-only lifetimes.
func TestValueScopeBarrierRoutesGlobal(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	scope := PushValueScope()
	defer scope.Release()
	if !HasActiveValueScope() {
		t.Fatal("HasActiveValueScope = false with open scope")
	}
	barrier := PushValueScopeBarrierIfActive()
	if !barrier.Active() {
		t.Fatal("expected a barrier while scope is active")
	}
	if HasActiveValueScope() {
		t.Fatal("HasActiveValueScope = true under barrier")
	}
	capturedBefore := scope.CapturedRootCount()
	globalBefore := GCRootLogSize()
	_ = SoAValue(makeScopeTestSoA(t, 4, 1))
	if scope.CapturedRootCount() != capturedBefore {
		t.Fatal("barrier leaked appends into enclosing scope")
	}
	if GCRootLogSize() == globalBefore {
		t.Fatal("barriered append did not reach the global log")
	}
	// Nested barrier request is a no-op.
	if b2 := PushValueScopeBarrierIfActive(); b2.Active() {
		t.Fatal("expected inert barrier when already barriered")
	}
	barrier.Release()
	if !HasActiveValueScope() {
		t.Fatal("scope not restored after barrier release")
	}
}

// TestValueScopeOtherGoroutinesUnaffected: while one goroutine holds a scope,
// other goroutines' values stay globally rooted and survive the release.
func TestValueScopeOtherGoroutinesUnaffected(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	scope := PushValueScope()
	type res struct{ v Value }
	results := make(chan res, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			results <- res{v: SoAValue(makeScopeTestSoA(t, 32, float64(seed)))}
		}(i)
	}
	wg.Wait()
	scope.Release() // drop everything this goroutine captured
	gcHammer(6)
	close(results)
	for r := range results {
		if got := soaChecksum(t, r.v); got <= 0 {
			t.Fatalf("other-goroutine value corrupted: checksum %v", got)
		}
	}
}

// TestValueScopeConcurrentScopes runs scoped create/keep/drop loops on many
// goroutines simultaneously under GOGC=1.
func TestValueScopeConcurrentScopes(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	const workers = 6
	const iters = 300
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			var kept []Value
			var want []float64
			for i := 0; i < iters; i++ {
				scope := PushValueScope()
				n := 8 + i%16
				v := SoAValue(makeScopeTestSoA(t, n, float64(seed*1000+i)))
				// scratch garbage that must be droppable
				_ = LazyStringValue(StringValue(strings.Repeat("s", 80)), StringValue(strings.Repeat("t", 80)))
				if i%10 == 0 {
					scope.Release(v)
					kept = append(kept, v)
					sum := 0.0
					for j := 0; j < n; j++ {
						sum += float64(seed*1000+i) + float64(j)
					}
					want = append(want, sum)
				} else {
					_ = soaChecksum(t, v) // consume inside scope
					scope.Release()
				}
				if i%50 == 0 {
					goruntime.GC()
				}
			}
			gcHammer(2)
			for k, v := range kept {
				if got := soaChecksum(t, v); got != want[k] {
					errCh <- &scopeChecksumError{got: got, want: want[k]}
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

type scopeChecksumError struct{ got, want float64 }

func (e *scopeChecksumError) Error() string { return "kept value corrupted after concurrent scopes" }

// TestValueScopeWrongGoroutineReleasePanics enforces the ownership contract.
func TestValueScopeWrongGoroutineReleasePanics(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	scope := PushValueScope()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		scope.Release()
	}()
	if r := <-done; r == nil {
		t.Fatal("Release on wrong goroutine did not panic")
	}
	// Scope still owned here; release properly.
	scope.s = restoreScopeForTest(&scope)
	scope.Release()
}

// restoreScopeForTest reconstructs the internal scope pointer after the failed
// cross-goroutine Release cleared the handle.
func restoreScopeForTest(vs *ValueScope) *valueScope {
	return valueScopeScopes[vs.slot].Load()
}

// TestValueScopeSlabPublishBypassesScope: tables allocated after a scope that
// forced slab publishes must stay valid (slab roots are global).
func TestValueScopeSlabPublishBypassesScope(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	scope := PushValueScope()
	// Force several table slab publishes inside the scope.
	for i := 0; i < 4*tableSlabSize; i++ {
		tbl := NewTable()
		tbl.RawSetString("i", IntValue(int64(i)))
		_ = TableValue(tbl)
	}
	scope.Release() // drop all per-object roots captured in the scope
	gcHammer(4)
	// New tables allocated into the (scope-era) current slab must be fine.
	for i := 0; i < tableSlabSize/2; i++ {
		tbl := NewTable()
		tbl.RawSetString("v", IntValue(int64(i*3)))
		v := TableValue(tbl)
		goruntime.GC()
		if got := v.Table().RawGetString("v"); !got.IsInt() || got.Int() != int64(i*3) {
			t.Fatalf("post-scope slab table corrupted at %d: %v", i, got)
		}
	}
}

// TestGlobalizeValueRootsSurvivesScopeRelease covers runtime-internal caches
// that publish values to process-global storage from inside a scope.
func TestGlobalizeValueRootsSurvivesScopeRelease(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	scope := PushValueScope()
	v := SoAValue(makeScopeTestSoA(t, 64, 9))
	GlobalizeValueRoots(v)
	scope.Release() // drop scope roots; the globalized copy must keep v alive
	gcHammer(6)
	want := 0.0
	for i := 0; i < 64; i++ {
		want += 9 + float64(i)
	}
	if got := soaChecksum(t, v); got != want {
		t.Fatalf("globalized value corrupted: %v want %v", got, want)
	}
}

// TestValueScopeStressResultHeavy simulates the q bind-boundary pattern under
// GOGC=1: per-iteration scopes producing result vectors, kept briefly,
// consumed, dropped — the global root log must stay bounded.
func TestValueScopeStressResultHeavy(t *testing.T) {
	if !valueScopeSupported() {
		t.Skip("value scopes unsupported on this platform")
	}
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	warmup := 32
	var baseline int64
	const iters = 2000
	for i := 0; i < iters; i++ {
		scope := PushValueScope()
		tbl := NewTable()
		tbl.RawSetString("vec", SoAValue(makeScopeTestSoA(t, 256, float64(i))))
		tbl.RawSetString("name", StringValue(strings.Repeat("k", 64)))
		out := TableValue(tbl)
		// Consume inside the scope.
		want := 0.0
		for j := 0; j < 256; j++ {
			want += float64(i) + float64(j)
		}
		if got := soaChecksum(t, out.Table().RawGetString("vec")); got != want {
			t.Fatalf("iter %d: checksum %v want %v", i, got, want)
		}
		scope.Release()
		if i == warmup {
			baseline = GCRootLogSize()
		}
		if i%100 == 0 {
			goruntime.GC()
		}
	}
	growth := GCRootLogSize() - baseline
	// Slab publishes still append globally; allow a generous bound that is
	// still far below the ~3 roots/op the unscoped path would add.
	if growth > int64(iters) {
		t.Fatalf("global root log grew by %d over %d scoped iterations", growth, iters)
	}
}
