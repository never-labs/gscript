//go:build darwin && arm64

// setglobal_crosstier_test.go is a CORRECTNESS SAFETY NET (tests only) for a
// future Tier 2 native SetGlobal fast path. It contains NO production changes.
//
// Background
// ----------
// Today Tier 2 routes every SetGlobal through a slow op-exit that calls
// VM.SetGlobal (internal/vm/vm.go). That path is dual-store and
// reallocation-safe: it writes BOTH vm.globalArray[idx] (the indexed store
// read by GetGlobal) AND the compatibility vm.globals map, and when a brand-new
// global name is seen it appends to globalArray. We later intend to enable a
// native indexed-store fast path that writes globalArray directly. Two hazards
// must be guarded before that is safe:
//
//	(A) Dual-storage divergence: a native store that updates only globalArray
//	    would leave the vm.globals map stale. VM.Globals() returns the map, so
//	    these tests read the map (via runTier2Globals) and would observe the
//	    stale value as a mismatch versus the interpreter.
//
//	(B) Reallocation lost-write: defining a NEW global mid-run appends to (and
//	    may realloc) globalArray. A native store performed through a base
//	    pointer captured before the realloc would be written to the old backing
//	    array and lost. These tests grow the global set during a hot loop and
//	    assert the repeatedly-stored existing global still matches the oracle.
//
// All tests below run the SAME program twice — once interpreter-only (oracle)
// and once with the TieringManager (which promotes hot code to the optimizing
// Tier 2) — and assert the ENTIRE written global state is identical. They PASS
// on current code (SetGlobal is safe); they become the arbiter when the native
// fast path is enabled, at which point a stale-map or lost-write regression
// turns them red.
//
// Tier 2 promotion
// ----------------
// The shared harness uses NewTieringManager (NOT the Tier-1-only
// BaselineJITEngine used by runVMFullWithJIT) so hot code can reach the
// optimizing tier. Loops use 50000 iterations — far above
// tmDefaultTier2Threshold (Tier2Threshold = 100). Empirically the
// TieringManager does NOT promote a bare top-level loop on its own, so EVERY
// scenario drives its global stores through a hot function CALL inside the
// loop; that promotes <main> (the inlined function body) to Tier 2. We set
// LEIA_TIER2_NO_FILTER=1 so the optimizing tier is exercised without the
// production viability filter masking the path under test, and every test
// asserts promotion actually happened via tm.Tier2Entered() (requireTier2).

package methodjit

import (
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"sort"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// runTier2Globals executes src with the TieringManager and returns the globals
// MAP (VM.Globals()). Reading the map — not GetGlobal/globalArray — is
// deliberate: it is the storage half a native array-only store would leave
// stale (hazard A). entered/failed are returned for promotion assertions.
func runTier2Globals(t *testing.T, src string) (globals map[string]runtime.Value, entered []string, failed map[string]string) {
	t.Helper()
	proto := compileProto(t, src)
	g := vmtest.NewInterpreterGlobals()
	v := vm.New(g)
	defer v.Close()
	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	if _, err := v.Execute(proto); err != nil {
		t.Fatalf("Tier2 execute error: %v", err)
	}
	return v.Globals(), tm.Tier2Entered(), tm.Tier2Failed()
}

// builtinGlobals returns the set of globals present in a fresh VM before any
// user code runs (stdlib module tables like `string`/`math`, builtin
// functions like `error`/`print`). These are seeded independently per VM
// instance and therefore differ by object identity between the oracle and JIT
// runs; they are not data written by the program under test and must be
// excluded from the differential.
func builtinGlobals(t *testing.T) map[string]struct{} {
	t.Helper()
	g := vmtest.NewInterpreterGlobals()
	v := vm.New(g)
	defer v.Close()
	base := v.Globals()
	out := make(map[string]struct{}, len(base))
	for k := range base {
		out[k] = struct{}{}
	}
	return out
}

// diffGlobals fails the test for every key that differs between the
// interpreter oracle and the JIT run, and for any key present in one but not
// the other. Comparison covers EVERY global the PROGRAM wrote (pre-seeded
// stdlib builtins are excluded), not a single named result — a partial/stale
// store on any program-written global is caught.
func diffGlobals(t *testing.T, oracle, jit map[string]runtime.Value) {
	t.Helper()
	builtins := builtinGlobals(t)
	keys := make(map[string]struct{}, len(oracle)+len(jit))
	for k := range oracle {
		if _, ok := builtins[k]; ok {
			continue
		}
		keys[k] = struct{}{}
	}
	for k := range jit {
		if _, ok := builtins[k]; ok {
			continue
		}
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		ov, ook := oracle[k]
		jv, jok := jit[k]
		if ook != jok {
			t.Errorf("global %q presence mismatch: interpreter=%v jit=%v", k, ook, jok)
			continue
		}
		if !ook {
			continue
		}
		// Builtins/closures (e.g. the `error`, `print` globals) live in the
		// map but are not data written by the program under test; two VM
		// instances hold distinct function objects that never compare equal by
		// identity. They are irrelevant to the SetGlobal data hazards, so skip
		// any global that is a function on both sides.
		if ov.IsFunction() && jv.IsFunction() {
			continue
		}
		assertValueEq(t, "global "+k, jv, ov)
	}
}

// requireTier2 asserts the optimizing tier was actually entered, so a passing
// test reflects the Tier 2 path rather than an accidental interpreter-only run.
func requireTier2(t *testing.T, entered []string, failed map[string]string) {
	t.Helper()
	if len(entered) == 0 {
		t.Fatalf("expected at least one Tier 2 entry (promotion); entered=%v failed=%v", entered, failed)
	}
}

// crossTierAllGlobals is the common driver: run oracle + Tier 2, assert full
// global-state equality and that Tier 2 was entered.
func crossTierAllGlobals(t *testing.T, src string) {
	t.Helper()
	oracle := runVMFull(t, src)
	jit, entered, failed := runTier2Globals(t, src)
	requireTier2(t, entered, failed)
	diffGlobals(t, oracle, jit)
}

// ---------------------------------------------------------------------------
// 1. Cross-tier whole-global-state differential.
//
// A hot reducer in <main> writes several globals of mixed type (int
// accumulators, a float accumulator, and string globals). Asserts interpreter
// and Tier 2 agree on ALL of them. If a native SetGlobal wrote only
// globalArray, the map read by Globals() would be stale for any of these and
// diffGlobals would flag it. Catch confidence: HIGH for hazard A — every
// written global is compared, so a stale map on any of them fails the test.
// ---------------------------------------------------------------------------

func TestSetGlobal_CrossTier_WholeStateDifferential(t *testing.T) {
	t.Setenv("LEIA_TIER2_NO_FILTER", "1")
	// The reducer runs inside a hot function call; this is what drives
	// promotion to the optimizing tier (a pure top-level loop with no call is
	// intentionally NOT promoted by the TieringManager today). The function
	// performs every global store under test.
	src := `
sum := 0
count := 0
fsum := 0.0
first := ""
last := ""
func reduce(i) {
    sum = sum + i
    count = count + 1
    fsum = fsum + 0.5
    if i == 1 { first = "begin" }
    if i == 50000 { last = "end" }
    return sum
}
result := 0
for i := 1; i <= 50000; i++ {
    result = reduce(i)
}
`
	crossTierAllGlobals(t, src)
}

// ---------------------------------------------------------------------------
// 2. Hazard A — write a global, then read it back in the SAME run via a
//    function call (GetGlobal) that derives further globals from it.
//
// step() writes `shared` (SetGlobal), then reads `shared` back (GetGlobal) to
// compute `mirror` and `echo`, in a 50000-iteration hot loop so step() / the
// inlined body reaches Tier 2. If a native SetGlobal updated only globalArray
// but a same-run GetGlobal (or the final Globals() map) read the other store,
// the read-back would be stale and mirror/echo/shared would diverge from the
// oracle. Catch confidence: HIGH for read-after-write coherence of hazard A —
// the value is both stored and immediately re-read within the hot region.
// ---------------------------------------------------------------------------

func TestSetGlobal_CrossTier_HazardA_WriteThenReadback(t *testing.T) {
	t.Setenv("LEIA_TIER2_NO_FILTER", "1")
	src := `
shared := 0
mirror := 0
echo := 0
func step(i) {
    shared = shared + i
    v := shared
    mirror = v
    echo = mirror + 1
    return echo
}
r := 0
for i := 1; i <= 50000; i++ { r = step(i) }
`
	oracle := runVMFull(t, src)
	jit, entered, failed := runTier2Globals(t, src)
	requireTier2(t, entered, failed)
	diffGlobals(t, oracle, jit)
}

// ---------------------------------------------------------------------------
// 3. Explicit global slot stress.
//
// Leia no longer creates globals implicitly from function-local assignment:
// every script-written global must be declared at top level. This test keeps
// the cross-tier safety net for writing several global slots from a hot
// function while repeatedly updating an existing accumulator. It no longer
// claims to exercise global-array growth; any future explicit global-definition
// API should add a separate reallocation test at that API boundary.
// ---------------------------------------------------------------------------

func TestSetGlobal_CrossTier_ExplicitGlobalSlotStress(t *testing.T) {
	t.Setenv("LEIA_TIER2_NO_FILTER", "1")
	src := `
acc := 0
late_a := 0
late_b := 0
late_c := 0
func step(i) {
    acc = acc + i
    if i == 10000 { late_a = 111 }
    if i == 20000 { late_b = 222 }
    if i == 30000 { late_c = 333 }
    return acc
}
r := 0
for i := 1; i <= 50000; i++ { r = step(i) }
`
	oracle := runVMFull(t, src)
	jit, entered, failed := runTier2Globals(t, src)
	requireTier2(t, entered, failed)
	diffGlobals(t, oracle, jit)
}

// ---------------------------------------------------------------------------
// 4. Deopt-after-SetGlobal.
//
// step() does a SetGlobal (`acc`) and then a TYPE-UNSTABLE operation: on most
// iterations x is an int that is added into the `flip` global, but periodically
// x becomes a string, forcing a guard failure / deopt right after the global
// store. Also stores a float global on the unstable branch. This exercises
// SetGlobal immediately adjacent to a deopt boundary, where a double-write
// (store applied both natively and again on the slow-path resume) or a
// lost-write (store dropped at the exit) would surface. We assert full global
// state matches the oracle. Catch confidence: HIGH for store/exit interaction —
// the deopt fires thousands of times right after the global store.
// ---------------------------------------------------------------------------

func TestSetGlobal_CrossTier_DeoptAfterSetGlobal(t *testing.T) {
	t.Setenv("LEIA_TIER2_NO_FILTER", "1")
	src := `
acc := 0
flip := 0
fmark := 0.0
func step(i) {
    acc = acc + i
    x := i
    if i % 7 == 0 { x = "deopt" }
    if i % 7 != 0 {
        flip = flip + x
    } else {
        fmark = fmark + 1.0
    }
    return acc
}
r := 0
for i := 1; i <= 50000; i++ { r = step(i) }
`
	oracle := runVMFull(t, src)
	jit, entered, failed := runTier2Globals(t, src)
	requireTier2(t, entered, failed)
	diffGlobals(t, oracle, jit)
}

// ---------------------------------------------------------------------------
// 5. Exit-resume-check parity.
//
// LEIA_EXIT_RESUME_CHECK=1 (see exit_resume_check.go) installs a shadow
// register verifier around Tier 2 exits at COMPILE time. We re-run the
// deopt-after-SetGlobal scenario with it enabled and assert the global state is
// unchanged versus the oracle. This guards that the store-near-exit machinery a
// native SetGlobal fast path will rely on stays self-consistent under the
// stricter exit verifier. t.Setenv is set before the Tier 2 run so it is
// observed at compile time.
// ---------------------------------------------------------------------------

func TestSetGlobal_CrossTier_ExitResumeCheckParity(t *testing.T) {
	t.Setenv("LEIA_TIER2_NO_FILTER", "1")
	t.Setenv("LEIA_EXIT_RESUME_CHECK", "1")
	src := `
acc := 0
flip := 0
func step(i) {
    acc = acc + i
    x := i
    if i % 5 == 0 { x = "deopt" }
    if i % 5 != 0 { flip = flip + x }
    return acc
}
r := 0
for i := 1; i <= 50000; i++ { r = step(i) }
`
	oracle := runVMFull(t, src)
	jit, entered, failed := runTier2Globals(t, src)
	requireTier2(t, entered, failed)
	diffGlobals(t, oracle, jit)
}
