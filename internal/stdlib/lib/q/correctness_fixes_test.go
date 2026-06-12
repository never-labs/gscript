package q

// Regression tests for the correctness-audit fixes:
//  1. mixed atom-vector literals must error, never panic
//  2. integer-atom `x?y` is roll/deal (seeded per-session PRNG, never memoized)
//  3. `/` and `\` with a MONADIC f are converge/do/while iterators
//  4. `` `t insert / `t upsert `` mutate the named session table
//  5. dict?value is reverse lookup returning the key
//  6. null ordered compares are canonical (null sorts first) and identical
//     across the scalar route, vector kernels, null-bitmap kernels, and the
//     where pipelines
//  7. n#empty fills with the type's zero; chained `#` is right-associative
//
// Where statements are compiler-representable, EvalScriptCompiledDifferential
// pins that the compiled route and the string evaluator agree.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// assertBothRoutes evaluates every statement of src through the compiled
// route and the string evaluator (where compiled) and requires agreement.
func assertBothRoutes(t *testing.T, src string) {
	t.Helper()
	state := NewEvalState(nil)
	records, err := state.EvalScriptCompiledDifferential(src)
	if err != nil {
		t.Fatalf("EvalScriptCompiledDifferential(%q) error: %v", src, err)
	}
	for _, record := range records {
		if !record.Match {
			t.Errorf("compiled/string divergence on %q (compiledErr=%v)", record.Statement, record.CompiledErr)
		}
	}
}

func assertEvalI64Vector(t *testing.T, src string, want []int64) {
	t.Helper()
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) error: %v", src, err)
	}
	array, ok := got.(data.Array)
	if !ok {
		t.Fatalf("Eval(%q) = %#v, want an i64 vector", src, got)
	}
	if array.Len() != len(want) {
		t.Fatalf("Eval(%q) len = %d, want %d", src, array.Len(), len(want))
	}
	for i, w := range want {
		v, _ := array.At(i)
		n, ok := v.(int64)
		if !ok || n != w {
			t.Fatalf("Eval(%q)[%d] = %#v, want %d", src, i, v, w)
		}
	}
	assertBothRoutes(t, src)
}

// --- fix 1: mixed literal vector crash --------------------------------------

func TestMixedLiteralVectorIsParseErrorNotPanic(t *testing.T) {
	for _, src := range []string{
		`1 "hello"`,
		`1 2 "x"`,
		`3 "a" 4`,
	} {
		got, err := Eval(src) // must not panic
		if err == nil {
			t.Errorf("Eval(%q) = %#v, want a parse error for mixed literal vectors", src, got)
			continue
		}
		if !strings.Contains(err.Error(), "mixed-type vector literal") {
			t.Errorf("Eval(%q) error = %v, want mixed-type vector literal error", src, err)
		}
	}
	// String-headed juxtaposition is canonical string indexing: "a" 1 reads
	// past the end and fills with the char null " " (see canonical_q.tsv).
	if got, err := Eval(`"a" 1`); err != nil || got != " " {
		t.Errorf("Eval(%q) = %#v, %v; want \" \"", `"a" 1`, got, err)
	}
}

// --- fix 2: roll / deal / rand ----------------------------------------------

func TestRollDealSemantics(t *testing.T) {
	// Roll: x random ints in [0,y).
	got, err := Eval("100?10")
	if err != nil {
		t.Fatalf("100?10 error: %v", err)
	}
	rolled, ok := got.(data.Array)
	if !ok || rolled.Len() != 100 {
		t.Fatalf("100?10 = %#v, want 100 draws", got)
	}
	for i := 0; i < rolled.Len(); i++ {
		v, _ := rolled.At(i)
		n, ok := v.(int64)
		if !ok || n < 0 || n >= 10 {
			t.Fatalf("100?10 row %d = %#v, want int64 in [0,10)", i, v)
		}
	}
	// Deal: distinct draws; -10?10 is a permutation of til 10.
	got, err = Eval("-10?10")
	if err != nil {
		t.Fatalf("-10?10 error: %v", err)
	}
	dealt := got.(data.Array)
	seen := map[int64]bool{}
	for i := 0; i < dealt.Len(); i++ {
		v, _ := dealt.At(i)
		n := v.(int64)
		if n < 0 || n >= 10 || seen[n] {
			t.Fatalf("-10?10 = %#v, want a permutation of til 10", got)
		}
		seen[n] = true
	}
	if len(seen) != 10 {
		t.Fatalf("-10?10 produced %d distinct values, want 10", len(seen))
	}
	// Deal larger than the domain errors (q 'length).
	if _, err := Eval("-11?10"); err == nil {
		t.Fatal("-11?10 succeeded, want deal-count error")
	}
	// Roll over a list picks elements.
	got, err = Eval("5?10 20 30")
	if err != nil {
		t.Fatalf("5?10 20 30 error: %v", err)
	}
	picks := got.(data.Array)
	for i := 0; i < picks.Len(); i++ {
		v, _ := picks.At(i)
		n := v.(int64)
		if n != 10 && n != 20 && n != 30 {
			t.Fatalf("5?10 20 30 row %d = %v, want a pick from the list", i, v)
		}
	}
	// 0?y is an empty draw.
	got, err = Eval("0?10")
	if err != nil {
		t.Fatalf("0?10 error: %v", err)
	}
	if array, ok := got.(data.Array); !ok || array.Len() != 0 {
		t.Fatalf("0?10 = %#v, want empty i64 vector", got)
	}
	// Float roll stays in [0,y).
	got, err = Eval("8?2.5")
	if err != nil {
		t.Fatalf("8?2.5 error: %v", err)
	}
	floats := got.(data.Array)
	for i := 0; i < floats.Len(); i++ {
		v, _ := floats.At(i)
		f := v.(float64)
		if f < 0 || f >= 2.5 {
			t.Fatalf("8?2.5 row %d = %v, want float64 in [0,2.5)", i, v)
		}
	}
	// rand y is a single draw.
	got, err = Eval("rand 10")
	if err != nil {
		t.Fatalf("rand 10 error: %v", err)
	}
	if n, ok := got.(int64); !ok || n < 0 || n >= 10 {
		t.Fatalf("rand 10 = %#v, want int64 in [0,10)", got)
	}
	// Find semantics stay for vector LHS.
	assertEvalValue(t, "1 2 3?2", int64(1))
}

func TestRollIsSeededPerSessionAndNeverMemoized(t *testing.T) {
	// Fresh sessions share the fixed default seed: reproducible.
	first, err := Eval("20?1000000")
	if err != nil {
		t.Fatalf("20?1000000 error: %v", err)
	}
	second, err := Eval("20?1000000")
	if err != nil {
		t.Fatalf("20?1000000 error: %v", err)
	}
	if !matchValue(first, second) {
		t.Fatal("fresh sessions with the default seed must produce identical draws")
	}
	// Within one session the PRNG stream advances: a memoized result would
	// repeat, so successive draws must differ (warm plan path included).
	state := NewEvalState(nil)
	a, err := state.Eval("20?1000000")
	if err != nil {
		t.Fatalf("session draw 1 error: %v", err)
	}
	b, err := state.Eval("20?1000000")
	if err != nil {
		t.Fatalf("session draw 2 error: %v", err)
	}
	if matchValue(a, b) {
		t.Fatal("same-session draws were identical: roll was memoized")
	}
	c, err := state.Eval("20?1000000")
	if err != nil {
		t.Fatalf("session draw 3 error: %v", err)
	}
	if matchValue(b, c) {
		t.Fatal("warm same-session draws were identical: roll was memoized")
	}
	// Assignment-form rolls draw independently too.
	got, err := Eval("x:20?1000000;y:20?1000000;x~y")
	if err != nil {
		t.Fatalf("assignment roll error: %v", err)
	}
	if b, ok := got.(bool); !ok || b {
		t.Fatalf("x:20?...;y:20?...;x~y = %#v, want false", got)
	}
}

func TestNondeterministicSourcesExcludedFromAllMemoLayers(t *testing.T) {
	for _, src := range []string{"5?10", "-5?10", "rand 10", "n?10", "x:5?10"} {
		if EvalSourceCacheable(src) {
			t.Errorf("EvalSourceCacheable(%q) = true, want false", src)
		}
		if qEvalConstantStatementSource(src) {
			t.Errorf("qEvalConstantStatementSource(%q) = true, want false", src)
		}
		if qScriptBindingPlanTextEligible(src) {
			t.Errorf("qScriptBindingPlanTextEligible(%q) = true, want false", src)
		}
		if cachedValueExprTextEligible(src) {
			t.Errorf("cachedValueExprTextEligible(%q) = true, want false", src)
		}
	}
	// Deterministic forms keep their eligibility.
	for _, src := range []string{"1+2", "til 10"} {
		if !EvalSourceCacheable(src) {
			t.Errorf("EvalSourceCacheable(%q) = false, want true", src)
		}
	}
	// The functional query prefix `?[` stays cacheable-shaped (it is the
	// deterministic select form, not roll).
	if qSourceHasRandomVerb("?[t;();0b;()]") {
		t.Error("qSourceHasRandomVerb flagged the functional query form")
	}
	if !qSourceHasRandomVerb("rand 10") || !qSourceHasRandomVerb("5?10") {
		t.Error("qSourceHasRandomVerb missed a random verb")
	}
}

// --- fix 3: converge / do / while iterate -----------------------------------

func TestMonadicOverIterateForms(t *testing.T) {
	// Do-iterate: bracket and infix forms.
	assertEvalValue(t, "{2*x}/[3;1]", int64(8))
	assertEvalValue(t, "3 {2*x}/ 1", int64(8))
	assertEvalValue(t, "f:{2*x}; 3 f/ 1", int64(8))
	assertEvalValue(t, "f:{2*x}; f/[3;1]", int64(8))
	assertEvalI64Vector(t, `{2*x}\[3;1]`, []int64{1, 2, 4, 8})
	assertEvalI64Vector(t, "3 {2*x}\\ 1", []int64{1, 2, 4, 8})
	// Converge.
	assertEvalValue(t, "{x div 2}/[16]", int64(0))
	assertEvalI64Vector(t, `{x div 2}\[16]`, []int64{16, 8, 4, 2, 1, 0})
	// While-iterate with a predicate seed.
	assertEvalValue(t, "{x<100} {2*x}/ 1", int64(128))
	assertEvalI64Vector(t, "{x<100} {2*x}\\ 1", []int64{1, 2, 4, 8, 16, 32, 64, 128})
	// Converge stops when the result cycles back to the original argument.
	assertEvalI64Vector(t, "reverse/[1 2 3]", []int64{1, 2, 3})
	// Rank dispatch: a DYADIC f keeps seeded-fold semantics.
	assertEvalValue(t, "{x+y}/[1 2 3]", int64(6))
	assertEvalValue(t, "{x+y}/[10;1 2 3]", int64(16))
	assertEvalValue(t, "0 {x+y}/ 1 2 3", int64(6))
	// Explicit-parameter lambdas rank correctly too.
	assertEvalValue(t, "{[a] a div 2}/[9]", int64(0))
	assertEvalValue(t, "3 {[a] 2*a}/ 1", int64(8))
}

// --- fix 4: named insert / upsert -------------------------------------------

func TestNamedInsertUpsert(t *testing.T) {
	state := NewEvalState(nil)
	mustEval := func(src string) any {
		t.Helper()
		out, err := state.Eval(src)
		if err != nil {
			t.Fatalf("Eval(%q) error: %v", src, err)
		}
		return out
	}
	mustEval("t:([] a:1 2; b:3 4)")
	// insert appends and returns the new row indexes.
	out := mustEval("`t insert (5;6)")
	indexes, ok := out.(data.Array)
	if !ok || indexes.Len() != 1 {
		t.Fatalf("`t insert returned %#v, want one row index", out)
	}
	if v, _ := indexes.At(0); v != int64(2) {
		t.Fatalf("`t insert row index = %#v, want 2", v)
	}
	if got := mustEval("count t"); got != int64(3) {
		t.Fatalf("count t after insert = %v, want 3", got)
	}
	if got := mustEval("+/t[`a]"); got != int64(8) {
		t.Fatalf("+/t[`a] after insert = %v, want 1+2+5=8", got)
	}
	// Column-vector multi-row insert.
	out = mustEval("`t insert (10 20;30 40)")
	indexes = out.(data.Array)
	if indexes.Len() != 2 {
		t.Fatalf("multi-row insert returned %#v, want two indexes", out)
	}
	if got := mustEval("count t"); got != int64(5) {
		t.Fatalf("count t after multi-row insert = %v, want 5", got)
	}
	// upsert on a plain table appends and returns the table name symbol.
	out = mustEval("`t upsert (7;8)")
	if sym, ok := out.(data.Symbol); !ok || sym != "t" {
		t.Fatalf("`t upsert returned %#v, want `t", out)
	}
	if got := mustEval("count t"); got != int64(6) {
		t.Fatalf("count t after upsert = %v, want 6", got)
	}
	// Keyed upsert merges on the key.
	mustEval("k:([s:`a`b] v:1 2)")
	mustEval("`k upsert (`a;100)")
	if got := mustEval("+/value[k][`v]"); got != int64(102) {
		t.Fatalf("sum k.v after upsert = %v, want 100+2=102", got)
	}
	if got := mustEval("count k"); got != int64(2) {
		t.Fatalf("count k after upsert = %v, want 2", got)
	}
	mustEval("`k upsert (`c;9)")
	if got := mustEval("count k"); got != int64(3) {
		t.Fatalf("count k after new-key upsert = %v, want 3", got)
	}
	// Keyed insert rejects duplicate keys.
	if _, err := state.Eval("`k insert (`a;1)"); err == nil {
		t.Fatal("keyed insert with duplicate key succeeded, want error")
	}
	// Errors instead of silent echoes.
	if _, err := state.Eval("`missing insert (1;2)"); err == nil {
		t.Fatal("insert into undefined name succeeded, want error")
	}
	if _, err := state.Eval("`t insert"); err == nil {
		t.Fatal("insert without rows succeeded, want error")
	}
	// A symbol literal cannot silently swallow trailing expression text.
	if _, err := Eval("`t insert (1;2)"); err == nil {
		t.Fatal("`t insert on a fresh session (t undefined) succeeded, want error")
	}
}

// --- fix 5: dict find -------------------------------------------------------

func TestDictFindReturnsKey(t *testing.T) {
	got, err := Eval("(`a`b!1 2)?2")
	if err != nil {
		t.Fatalf("dict find error: %v", err)
	}
	if sym, ok := got.(data.Symbol); !ok || sym != "b" {
		t.Fatalf("(`a`b!1 2)?2 = %#v, want `b", got)
	}
	// Missing value returns the null (empty) symbol key.
	got, err = Eval("(`a`b!1 2)?9")
	if err != nil {
		t.Fatalf("dict find missing error: %v", err)
	}
	if sym, ok := got.(data.Symbol); !ok || sym != "" {
		t.Fatalf("(`a`b!1 2)?9 = %#v, want null symbol", got)
	}
	// Vector query maps to a key per element.
	got, err = Eval("(`a`b!1 2)?2 1")
	if err != nil {
		t.Fatalf("dict find vector error: %v", err)
	}
	keysOut, ok := got.(data.Array)
	if !ok || keysOut.Len() != 2 {
		t.Fatalf("(`a`b!1 2)?2 1 = %#v, want 2 keys", got)
	}
	if v, _ := keysOut.At(0); v != data.Symbol("b") {
		t.Fatalf("dict find vector[0] = %#v, want `b", v)
	}
	if v, _ := keysOut.At(1); v != data.Symbol("a") {
		t.Fatalf("dict find vector[1] = %#v, want `a", v)
	}
	assertBothRoutes(t, "(`a`b!1 2)?2")
}

// --- fix 6: canonical null compares -----------------------------------------

// qNullCmpOracle is the canonical q scalar truth: null sorts before every
// value and equals itself.
func qNullCmpOracle(op string, l, r any) bool {
	ln, rn := data.IsNull(l), data.IsNull(r)
	if ln || rn {
		switch op {
		case "=":
			return ln && rn
		case "<>":
			return ln != rn
		case "<":
			return ln && !rn
		case ">":
			return !ln && rn
		case "<=":
			return ln
		case ">=":
			return rn
		}
		return false
	}
	toF := func(v any) float64 {
		switch x := v.(type) {
		case int64:
			return float64(x)
		case float64:
			return x
		}
		return 0
	}
	lf, rf := toF(l), toF(r)
	switch op {
	case "=":
		return lf == rf
	case "<>":
		return lf != rf
	case "<":
		return lf < rf
	case ">":
		return lf > rf
	case "<=":
		return lf <= rf
	case ">=":
		return lf >= rf
	}
	return false
}

func TestNullCompareScalarTruthTable(t *testing.T) {
	checks := []struct {
		expr string
		want bool
	}{
		{"0N<1", true}, {"1<0N", false}, {"0N<0N", false},
		{"0N>1", false}, {"1>0N", true}, {"0N>0N", false},
		{"0N<=1", true}, {"1<=0N", false}, {"0N<=0N", true},
		{"0N>=1", false}, {"1>=0N", true}, {"0N>=0N", true},
		{"0N=1", false}, {"0N=0N", true},
		{"0N<>1", true}, {"0N<>0N", false},
		{"0n<1.0", true}, {"1.0<=0n", false}, {"0n>=0n", true},
		{"0Nh<1h", true}, {"0Ni<=2i", true},
	}
	for _, c := range checks {
		got, err := Eval(c.expr)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", c.expr, err)
			continue
		}
		if b, ok := got.(bool); !ok || b != c.want {
			t.Errorf("Eval(%q) = %#v, want %v", c.expr, got, c.want)
		}
		assertBothRoutes(t, c.expr)
	}
}

// TestNullCompareVectorKernelsMatchScalarOracle pins the hard requirement of
// the audit: every vector representation (boxed nullable, typed null-bitmap,
// tiled views), every compare op, and every downstream pipeline (mask,
// where-count, where-sum) agrees with the canonical scalar truth table on
// cold and warm (plan-cached) evaluations.
func TestNullCompareVectorKernelsMatchScalarOracle(t *testing.T) {
	vals := []any{data.NullValue, int64(2), int64(3), int64(1), data.NullValue, int64(0)}
	vecSrcs := map[string]string{
		"boxed":  "x:0N 2 3 1 0N 0",
		"typedH": "x:0Nh 2h 3h 1h 0Nh 0h",
		"typedI": "x:0Ni 2i 3i 1i 0Ni 0i",
		"typedE": "x:0Ne 2e 3e 1e 0Ne 0e",
		"typedF": "x:0n 2.0 3.0 1.0 0n 0.0",
		"tiled":  "x:6#0N 2 3 1 0N 0",
	}
	scalars := map[string]any{"2": int64(2), "0N": data.NullValue}
	ops := []string{"<", "<=", ">", ">=", "=", "<>"}
	for vecName, vecSrc := range vecSrcs {
		for scalarName, scalarVal := range scalars {
			for _, op := range ops {
				state := NewEvalState(nil)
				for warm := 0; warm < 2; warm++ {
					expr := vecSrc + ";x" + op + scalarName
					got, err := state.Eval(expr)
					if err != nil {
						t.Fatalf("%s warm=%d Eval(%q) error: %v", vecName, warm, expr, err)
					}
					mask, ok := got.(data.Array)
					if !ok {
						t.Fatalf("%s Eval(%q) = %T, want bool vector", vecName, expr, got)
					}
					var wantCount, wantSum int64
					for i, v := range vals {
						want := qNullCmpOracle(op, v, scalarVal)
						g, _ := mask.At(i)
						if gb, _ := g.(bool); gb != want {
							t.Errorf("%s warm=%d %q row %d = %v, want %v", vecName, warm, expr, i, g, want)
						}
						if want {
							wantCount++
							wantSum += int64(i)
						}
					}
					countExpr := vecSrc + ";count where x" + op + scalarName
					if got, err := state.Eval(countExpr); err != nil {
						t.Errorf("%q error: %v", countExpr, err)
					} else if n, ok := got.(int64); !ok || n != wantCount {
						t.Errorf("%s warm=%d %q = %v, want %d", vecName, warm, countExpr, got, wantCount)
					}
					sumExpr := vecSrc + ";+/where x" + op + scalarName
					if got, err := state.Eval(sumExpr); err != nil {
						t.Errorf("%q error: %v", sumExpr, err)
					} else if wantCount == 0 {
						if !data.IsNull(got) && got != int64(0) {
							t.Errorf("%s warm=%d %q = %v, want empty sum", vecName, warm, sumExpr, got)
						}
					} else if n, ok := got.(int64); !ok || n != wantSum {
						t.Errorf("%s warm=%d %q = %v, want %d", vecName, warm, sumExpr, got, wantSum)
					}
				}
			}
		}
	}
}

func TestNullCompareScalarLHSVectorAndVectorVector(t *testing.T) {
	vals := []any{data.NullValue, int64(2), int64(3), int64(1), data.NullValue, int64(0)}
	ops := []string{"<", "<=", ">", ">=", "=", "<>"}
	for _, op := range ops {
		for scalarName, scalarVal := range map[string]any{"2": int64(2), "0N": data.NullValue} {
			for vecName, vecSrc := range map[string]string{
				"boxed": "x:0N 2 3 1 0N 0", "typedH": "x:0Nh 2h 3h 1h 0Nh 0h", "typedF": "x:0n 2.0 3.0 1.0 0n 0.0",
			} {
				expr := vecSrc + ";" + scalarName + op + "x"
				got, err := Eval(expr)
				if err != nil {
					t.Errorf("%q error: %v", expr, err)
					continue
				}
				mask, ok := got.(data.Array)
				if !ok {
					t.Errorf("%q returned %T", expr, got)
					continue
				}
				for i, v := range vals {
					want := qNullCmpOracle(op, scalarVal, v)
					g, _ := mask.At(i)
					if gb, _ := g.(bool); gb != want {
						t.Errorf("%s %q row %d = %v, want %v", vecName, expr, i, g, want)
					}
				}
			}
		}
		rvals := []any{int64(1), data.NullValue, int64(4), int64(1), data.NullValue, int64(2)}
		expr := "x:0N 2 3 1 0N 0;y:1 0N 4 1 0N 2;x" + op + "y"
		got, err := Eval(expr)
		if err != nil {
			t.Errorf("%q error: %v", expr, err)
			continue
		}
		mask := got.(data.Array)
		for i := range vals {
			want := qNullCmpOracle(op, vals[i], rvals[i])
			g, _ := mask.At(i)
			if gb, _ := g.(bool); gb != want {
				t.Errorf("%q row %d = %v, want %v", expr, i, g, want)
			}
		}
	}
}

func TestNullCompareQSQLWherePipeline(t *testing.T) {
	vals := []any{data.NullValue, int64(2), int64(3), int64(1), data.NullValue, int64(0)}
	for _, op := range []string{"<", "<=", ">", ">=", "=", "<>"} {
		frameValue, err := Eval("([] a:0N 2 3 1 0N 0; b:10 11 12 13 14 15)")
		if err != nil {
			t.Fatalf("frame literal error: %v", err)
		}
		lowered, err := Lower(mustParse(t, fmt.Sprintf("select b from t where a%s2", op)))
		if err != nil {
			t.Errorf("lower a%s2: %v", op, err)
			continue
		}
		out, err := lowered.ExecRuntime(frameValue.(data.Frame))
		if err != nil {
			t.Errorf("exec a%s2: %v", op, err)
			continue
		}
		var want []int64
		for i, v := range vals {
			if qNullCmpOracle(op, v, int64(2)) {
				want = append(want, int64(10+i))
			}
		}
		col, _ := out.Column("b")
		if col.Len() != len(want) {
			t.Errorf("qSQL where a%s2: got %d rows, want %d", op, col.Len(), len(want))
			continue
		}
		for i, w := range want {
			g, _ := col.At(i)
			if gi, _ := g.(int64); gi != w {
				t.Errorf("qSQL where a%s2 row %d = %v, want %v", op, i, g, w)
			}
		}
	}
}

// --- fix 7: take from empty + `#` associativity ------------------------------

func TestTakeFromEmptyFillsWithTypeZero(t *testing.T) {
	// Canonical: 3#0#1 -> 0 0 0 (chained # is right-associative; take from
	// an empty typed list yields the type's zero fill).
	assertEvalI64Vector(t, "3#0#1", []int64{0, 0, 0})
	assertEvalI64Vector(t, "3#(0#1)", []int64{0, 0, 0})
	assertEvalI64Vector(t, "-3#0#1", []int64{0, 0, 0})
	assertEvalI64Vector(t, "x:0#1;3#x", []int64{0, 0, 0})
	// Float / bool / symbol fills.
	got, err := Eval("2#0#1.5")
	if err != nil {
		t.Fatalf("2#0#1.5 error: %v", err)
	}
	floats := got.(data.Array)
	if floats.Kind() != data.KindF64 || floats.Len() != 2 {
		t.Fatalf("2#0#1.5 = %#v, want 2 float zeros", got)
	}
	if v, _ := floats.At(0); v != 0.0 {
		t.Fatalf("2#0#1.5[0] = %#v, want 0f", v)
	}
	got, err = Eval("2#0#`a")
	if err != nil {
		t.Fatalf("2#0#`a error: %v", err)
	}
	syms := got.(data.Array)
	if v, _ := syms.At(0); v != data.Symbol("") {
		t.Fatalf("2#0#`a[0] = %#v, want empty symbol", v)
	}
	// Generic empty () fills with nulls.
	got, err = Eval("3#()")
	if err != nil {
		t.Fatalf("3#() error: %v", err)
	}
	anyArray := got.(data.Array)
	if anyArray.Len() != 3 {
		t.Fatalf("3#() = %#v, want 3 nulls", got)
	}
	for i := 0; i < 3; i++ {
		v, _ := anyArray.At(i)
		if !data.IsNull(v) {
			t.Fatalf("3#()[%d] = %#v, want null", i, v)
		}
	}
	// 0#atom keeps the atom's type.
	got, err = Eval("0#1")
	if err != nil {
		t.Fatalf("0#1 error: %v", err)
	}
	if array, ok := got.(data.Array); !ok || array.Kind() != data.KindI64 || array.Len() != 0 {
		t.Fatalf("0#1 = %#v, want empty i64 vector", got)
	}
	// Reshape with a vector shape still works.
	got, err = Eval("2 3#til 6")
	if err != nil {
		t.Fatalf("2 3#til 6 error: %v", err)
	}
	if shaped, ok := got.(data.Array); !ok || shaped.Len() != 2 {
		t.Fatalf("2 3#til 6 = %#v, want 2x3 matrix", got)
	}
}

// ---------------------------------------------------------------------------
// Fuzzer-found panic fixes (formerly pinned in benchmarks/q_eval_fuzz_test.go
// qEvalKnownCrashers): every input below used to crash the process.
// ---------------------------------------------------------------------------

// TestDictEqualityPerKey pins canonical `=`/`<>` on dictionaries: a dict of
// booleans per key instead of a Go == panic on the uncomparable EvalDict.
func TestDictEqualityPerKey(t *testing.T) {
	assertEvalDictAny(t, "(`a`b!1 2)=(`a`b!1 3)",
		[]any{data.Symbol("a"), data.Symbol("b")}, []any{true, false})
	assertEvalDictAny(t, "(`a`b!1 2)<>`a`b!1 2",
		[]any{data.Symbol("a"), data.Symbol("b")}, []any{false, false})
	// Union alignment: keys present on one side only compare against null
	// and yield 0b (left key order first, then unseen right keys).
	assertEvalDictAny(t, "(`a`b!1 2)=(`b`c!2 9)",
		[]any{data.Symbol("a"), data.Symbol("b"), data.Symbol("c")},
		[]any{false, true, false})
	// dict = atom broadcasts over the values.
	assertEvalDictAny(t, "(`a`b!1 2)=(1)",
		[]any{data.Symbol("a"), data.Symbol("b")}, []any{true, false})
}

// TestCallableEqualityIdentity pins `=`/`<>` on callables as match-style
// identity (the rule ~ uses) instead of a Go == panic on uncomparable
// function values.
func TestCallableEqualityIdentity(t *testing.T) {
	assertEvalValue(t, "qv:neg;(qv)=(qv)", true)
	assertEvalValue(t, "qv:neg;(qv)<>(qv)", false)
	assertEvalValue(t, "qv:neg;qw:count;(qv)<>(qw)", true)
	assertBothRoutes(t, "qv:neg;(qv)<>(qv)")
}

// TestWhereReplicationLimit pins where on a long-infinity (or otherwise
// astronomically large) replication count: a q error instead of a makeslice
// overflow panic.
func TestWhereReplicationLimit(t *testing.T) {
	assertEvalErrorContains(t, "where 0W", "list limit")
	assertEvalErrorContains(t, "where 1 0W", "list limit")
	assertEvalI64Vector(t, "where 0 2 1", []int64{1, 1, 2})
	// The original fuzzer crasher: the compiled route used to makeslice-panic
	// before the string route's probe error could surface.
	if _, err := Eval("flip where 0W"); err == nil {
		t.Fatal("flip where 0W should error")
	}
}

// TestReshapeFromEmptyFills pins reshape from an empty list: canonical q
// fills with the type's zero (the 3#0#1 family) instead of building a matrix
// view over an empty base that panics at materialization.
func TestReshapeFromEmptyFills(t *testing.T) {
	got, err := Eval("2 2#til 0")
	if err != nil {
		t.Fatalf("2 2#til 0 error: %v", err)
	}
	matrix, ok := got.(data.Matrix)
	if !ok {
		t.Fatalf("2 2#til 0 = %#v, want a matrix", got)
	}
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			v, ok := matrix.Cell(row, col)
			if !ok || v != int64(0) {
				t.Fatalf("(2 2#til 0)[%d;%d] = %#v ok=%v, want 0", row, col, v, ok)
			}
		}
	}
	assertEvalI64Vector(t, "m:2 2#til 0;m . 0", []int64{0, 0})
	assertBothRoutes(t, "m:2 2#til 0;m . 0")
}

// TestGatherIndexBoundsError pins vector indexing over short/empty bases:
// canonical q read-path indexing null-fills out-of-range rows (identical on
// both routes) instead of erroring or panicking in Gather.
func TestGatherIndexBoundsError(t *testing.T) {
	assertBothRoutesValue(t, "all null m@0 0", "m:0#0;all null (m@0 0)", true)
	assertBothRoutesValue(t, "(til 3)@4 5 nulls", "all null ((til 3)@4 5)", true)
	assertBothRoutesValue(t, "count m@0 0", "m:0#0;count (m@0 0)", int64(2))
}

func assertBothRoutesValue(t *testing.T, label, src string, want any) {
	t.Helper()
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("%s: Eval(%q) error: %v", label, src, err)
	}
	if got != want {
		t.Fatalf("%s: Eval(%q) = %#v, want %#v", label, src, got, want)
	}
	assertBothRoutes(t, src)
}

// TestEmptyOperandLogicalBroadcast pins logical &/| between an empty list
// and a broadcastable operand: an empty result (the empty side wins, same
// rule as the len-1 broadcast) instead of an integer divide by zero when the
// mask cycles row%0.
func TestEmptyOperandLogicalBroadcast(t *testing.T) {
	for _, src := range []string{"qv:rank til 1;()&(qv)", "qv:rank til 1;(qv)&()", "()&(enlist 1b)", "()|()"} {
		got, err := Eval(src)
		if err != nil {
			t.Fatalf("Eval(%q) error: %v", src, err)
		}
		array, ok := got.(data.Array)
		if !ok || array.Len() != 0 {
			t.Fatalf("Eval(%q) = %#v, want an empty vector", src, got)
		}
		_ = array.Values() // materialization must not panic
	}
	// Mismatched non-broadcastable lengths still error.
	assertEvalErrorContains(t, "(1 0b)&(1 1 0b)", "length")
}

// TestLazyBoolMaskMaterializes pins not/&-composed masks over lazy carriers
// without a dedicated boolArrayAt case (membership and float-compare masks):
// they must materialize instead of panicking "out of range".
func TestLazyBoolMaskMaterializes(t *testing.T) {
	got, err := Eval("x:til 1;not (x in 0)")
	if err != nil {
		t.Fatalf("not-over-membership error: %v", err)
	}
	array, ok := got.(data.Array)
	if !ok || array.Len() != 1 {
		t.Fatalf("not-over-membership = %#v, want 1-element bool vector", got)
	}
	if v, ok := array.At(0); !ok || v != false {
		t.Fatalf("not (0 in 0) = %#v ok=%v, want 0b", v, ok)
	}
	values := array.Values()
	if len(values) != 1 || values[0] != false {
		t.Fatalf("materialized mask = %#v, want [false]", values)
	}
}
