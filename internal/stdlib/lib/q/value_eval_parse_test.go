package q

// Tests for the value / eval / parse evaluation verbs (value_eval_parse.go):
// the value dispatch matrix, the parse tree spelling, the round-trip
// contract eval parse src ≡ value src, the compiled/string dual-route
// safety, and the memo-eligibility registration.

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func evalOnFresh(t *testing.T, src string) any {
	t.Helper()
	v, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) error: %v", src, err)
	}
	return v
}

func TestValueVerbDispatchMatrix(t *testing.T) {
	// string -> evaluate as q source in the current session.
	if got := evalOnFresh(t, `value "1+2"`); got != int64(3) {
		t.Errorf("value \"1+2\" = %v, want 3", got)
	}
	// Multi-statement source with assignment; the result is the last value.
	if got := evalOnFresh(t, `value "x:9;x*2"`); got != int64(18) {
		t.Errorf("value \"x:9;x*2\" = %v, want 18", got)
	}
	// SCOPING PIN: value runs in the CURRENT context (q semantics): the
	// session env is visible inside the string and assignments mutate it.
	if got := evalOnFresh(t, `value "x:9";x`); got != int64(9) {
		t.Errorf("value \"x:9\";x = %v, want 9 (value must mutate the outer env)", got)
	}
	if got := evalOnFresh(t, `a:7;value "a+1"`); got != int64(8) {
		t.Errorf("a:7;value \"a+1\" = %v, want 8 (value must see the outer env)", got)
	}
	// symbol -> dereference in env.
	if got := evalOnFresh(t, "a:5;value `a"); got != int64(5) {
		t.Errorf("a:5;value `a = %v, want 5", got)
	}
	// unbound symbol -> canonical q name error 'name.
	if _, err := Eval("value `undefined"); err == nil || !strings.Contains(err.Error(), "undefined") {
		t.Errorf("value `undefined error = %v, want 'undefined name error", err)
	}
	// parse tree -> evaluate the tree.
	if got := evalOnFresh(t, `value parse "1+2"`); got != int64(3) {
		t.Errorf("value parse \"1+2\" = %v, want 3", got)
	}
	// atom identity.
	if got := evalOnFresh(t, "value 42"); got != int64(42) {
		t.Errorf("value 42 = %v, want 42", got)
	}
	// dict -> values, typed when homogeneous (canonical: value `a`b!1 2 is
	// the long vector 1 2, not a generic list).
	got := evalOnFresh(t, "value `a`b!1 2")
	if array, ok := got.(data.Array); !ok || array.Kind() != data.KindI64 || !matchValue(got, data.NewI64([]int64{1, 2})) {
		t.Errorf("value `a`b!1 2 = %#v, want typed long vector 1 2", got)
	}
	// enum -> decoded values (pre-existing behavior kept).
	gotEnum := evalOnFresh(t, "value `sym$`AAPL`MSFT")
	wantEnum := evalOnFresh(t, "`AAPL`MSFT")
	if !matchValue(gotEnum, wantEnum) {
		t.Errorf("value `sym$`AAPL`MSFT = %#v, want `AAPL`MSFT", gotEnum)
	}
	// typed vectors stay constants.
	gotVec := evalOnFresh(t, "value 1 2 3")
	if !matchValue(gotVec, evalOnFresh(t, "1 2 3")) {
		t.Errorf("value 1 2 3 = %#v, want 1 2 3", gotVec)
	}
}

func TestValueVerbAlternativeRoutes(t *testing.T) {
	// Bracket application, each, composition-free juxtaposed name operand:
	// every route must reach the session-aware verb, not the stateless one.
	if got := evalOnFresh(t, `value["1+2"]`); got != int64(3) {
		t.Errorf("value[\"1+2\"] = %v, want 3", got)
	}
	got := evalOnFresh(t, `value each ("1+2";"3*3")`)
	if !matchValue(got, evalOnFresh(t, "3 9")) {
		t.Errorf("value each = %#v, want 3 9", got)
	}
	if got := evalOnFresh(t, `x:"1+2";value x`); got != int64(3) {
		t.Errorf("value x (bound string) = %v, want 3", got)
	}
	// Bound-string operand through an assignment RHS (the binding-plan and
	// compiled layers must decline to the string evaluator).
	if got := evalOnFresh(t, `v:"3*4";a:value v;a+1`); got != int64(13) {
		t.Errorf("a:value v;a+1 = %v, want 13", got)
	}
}

func TestValueRecursionGuard(t *testing.T) {
	_, err := Eval(`b:"value b";value b`)
	if err == nil || !strings.Contains(err.Error(), "stack") {
		t.Fatalf("self-referential value error = %v, want 'stack recursion error", err)
	}
}

func TestEvalVerb(t *testing.T) {
	// Atoms evaluate to themselves.
	if got := evalOnFresh(t, "eval 42"); got != int64(42) {
		t.Errorf("eval 42 = %v, want 42", got)
	}
	// Symbols resolve as names (literal symbols are enlisted by parse).
	if got := evalOnFresh(t, "a:3;eval `a"); got != int64(3) {
		t.Errorf("a:3;eval `a = %v, want 3", got)
	}
	// Hand-written trees: word-verb head symbols resolve through the verb
	// registries (dialect tree spelling).
	if got := evalOnFresh(t, "eval (`mod;7;3)"); got != int64(1) {
		t.Errorf("eval (`mod;7;3) = %v, want 1", got)
	}
	// Nested tree evaluation.
	if got := evalOnFresh(t, "eval (`mod;(`mod;17;10);3)"); got != int64(1) {
		t.Errorf("eval (`mod;(`mod;17;10);3) = %v, want 1", got)
	}
	// Unary word verb in head position.
	if got := evalOnFresh(t, "eval (`neg;5)"); got != int64(-5) {
		t.Errorf("eval (`neg;5) = %v, want -5", got)
	}
	// Unknown head symbol signals the q name error.
	if _, err := Eval("eval (`nosuchverb;1)"); err == nil || !strings.Contains(err.Error(), "nosuchverb") {
		t.Errorf("eval unknown head error = %v, want 'nosuchverb", err)
	}
}

func TestParseTreeSpelling(t *testing.T) {
	// Documented dialect spelling: verbs are SYMBOL names in head position
	// (canonical q returns function atoms), literal symbols are enlisted,
	// names are plain symbols, literal vectors stay constants.
	cases := []struct {
		src  string
		want string // leia expression building the expected tree
	}{
		{`parse "7 mod 3"`, "(`mod;7;3)"},
		{`parse "neg 3"`, "(`neg;3)"},
		{`parse "til 5"`, "(`til;5)"},
		{`parse "\"abc\""`, `"abc"`},
		{`parse "1 2 3"`, "1 2 3"},
	}
	for _, tc := range cases {
		got := evalOnFresh(t, tc.src)
		want := evalOnFresh(t, tc.want)
		if !matchValue(got, want) {
			t.Errorf("%s = %#v, want %s", tc.src, got, tc.want)
		}
	}
	// parse "`a" -> enlisted symbol (mirrors q's ,`a).
	tree := evalOnFresh(t, "parse \"`a\"")
	array, ok := tree.(data.Array)
	if !ok || array.Kind() != data.KindAny || array.Len() != 1 {
		t.Fatalf("parse \"`a\" = %#v, want 1-item generic list", tree)
	}
	if item, _ := array.At(0); item != data.Symbol("a") {
		t.Errorf("parse \"`a\" item = %#v, want `a", tree)
	}
	// parse "1+2" -> (`$\"+\";1;2): symbol-op head.
	opTree := evalOnFresh(t, `parse "1+2"`)
	opArray, ok := opTree.(data.Array)
	if !ok || opArray.Len() != 3 {
		t.Fatalf("parse \"1+2\" = %#v, want 3-item list", opTree)
	}
	if head, _ := opArray.At(0); head != data.Symbol("+") {
		t.Errorf("parse \"1+2\" head = %#v, want `+ symbol", head)
	}
}

func TestParseOutOfSubsetErrors(t *testing.T) {
	for _, src := range []string{
		`parse "x:1"`,
		`parse "x:1;x"`,
		`parse "{x+1}"`,
		`parse "f each 1 2 3"`,
		"parse 42",
	} {
		if _, err := Eval(src); err == nil {
			t.Errorf("%s succeeded, want a clear unsupported-source error", src)
		}
	}
}

// TestEvalParseRoundTrip pins the round-trip contract: eval parse src is
// value-identical to value src (≡ Eval src) for the supported subset.
func TestEvalParseRoundTrip(t *testing.T) {
	corpus := []string{
		// literals
		"42", "1.5", "1b", `"abc"`, "`a", "1 2 3", "`a`b`c", "2024.01.05",
		// arithmetic / symbol ops (right-to-left grouping)
		"1+2", "2*3+1", "10-3-2", "100%7", "2#1 2 3", "1_til 5",
		"3=1 2 3", "1<til 4", "1 2,3 4", "5^0N", "1 2 3~1 2 3",
		// unary verbs
		"neg 3", "count til 7", "sum 1 2 3", "reverse til 4", "distinct 1 2 2",
		"asc 3 1 2", "deltas 1 3 6", "string 12", "upper \"ab\"", "abs -5",
		"sqrt 16", "floor 3.7", "til 5", "enlist 5", "first 1 2 3",
		"deltas sums til 5", "count where 1 0 1b",
		// dyadic word verbs
		"7 mod 3", "7 div 3", "2 xexp 3", "1 2 3 in 2 3", "3 xbar til 10",
		"2 cut til 6", "1 rotate til 5", "1 2 cross 3 4", "1 2 3 union 3 4",
		"3 msum til 6", "til 10 within 2 6",
		// dict / cast / apply-at / lists / indexing (with env)
		"`a`b!1 2", "`long$1.5", "`float$1 2 3", "(1;2;3)", "(1;`a)",
		"a[1]", "a+1", "sum a", "d`x", "1 2 3?2",
	}
	for _, src := range corpus {
		seed := "a:10 20 30;d:`x`y!1 2;"
		direct, directErr := Eval(seed + src)
		state := NewEvalState(nil)
		if _, err := state.Eval(seed[:len(seed)-1]); err != nil {
			t.Fatalf("seed eval: %v", err)
		}
		tree, parseErr := state.Eval("parse " + quoteQString(src))
		if parseErr != nil {
			t.Errorf("parse %q: %v", src, parseErr)
			continue
		}
		state.env["zztree"] = tree
		roundTrip, evalErr := state.Eval("eval zztree")
		switch {
		case directErr != nil:
			if evalErr == nil {
				t.Errorf("round-trip %q: direct route errored (%v), eval parse succeeded with %#v", src, directErr, roundTrip)
			}
		case evalErr != nil:
			t.Errorf("round-trip %q: eval parse error %v, direct route gave %#v", src, evalErr, direct)
		case !matchValue(direct, roundTrip):
			t.Errorf("round-trip %q: eval parse = %#v, value = %#v", src, roundTrip, direct)
		}
	}
}

// quoteQString embeds src in a q string literal.
func quoteQString(src string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(src, `\`, `\\`), `"`, `\"`) + `"`
}

// TestValueEvalDualRouteSafety: the compiled statement route and the
// script-binding plans must DECLINE value-of-string/tree and eval (single
// execution through the string evaluator, mirroring roll/deal), so the
// dual-route differential never evaluates a stateful form twice.
func TestValueEvalDualRouteSafety(t *testing.T) {
	state := NewEvalState(nil)
	records, err := state.EvalScriptCompiledDifferential(`x:"c:1+2;c";y:value x;y*2`)
	if err != nil {
		t.Fatalf("differential error: %v", err)
	}
	for _, r := range records {
		if !r.Match {
			t.Errorf("differential mismatch on %q", r.Statement)
		}
		if strings.HasPrefix(r.Statement, "value ") && r.Compiled {
			t.Errorf("compiled route claimed %q; value-of-string must stay string-routed", r.Statement)
		}
	}
	// Binding plans must not claim value/eval applications.
	for _, src := range []string{"value x", "eval x"} {
		if compiled := compileQEvalExpr(src, 0); compiled != nil {
			if plan := buildQScriptBindingPlanFromCompiled(compiled); plan.kind != qScriptBindingInvalid {
				t.Errorf("binding plan claimed %q (kind %v); must decline", src, plan.kind)
			}
		}
	}
	// Symbol dereference is read-only and may stay compiled, but both routes
	// must agree.
	state2 := NewEvalState(nil)
	records2, err := state2.EvalScriptCompiledDifferential("a:5;b:value `a;b+1")
	if err != nil {
		t.Fatalf("differential error: %v", err)
	}
	for _, r := range records2 {
		if !r.Match {
			t.Errorf("differential mismatch on %q", r.Statement)
		}
	}
}

// TestValueEvalParseVerbMetadata pins the memo-safety registration: value
// and eval are stateful-capable (never memoized), parse is deterministic.
func TestValueEvalParseVerbMetadata(t *testing.T) {
	if props := qVerbMetadata(qVerbNameValueSession); !props.Stateful || props.Deterministic {
		t.Errorf("value(session) metadata = %+v, want Stateful", props)
	}
	if props := qVerbMetadata("eval"); !props.Stateful || props.Deterministic {
		t.Errorf("eval metadata = %+v, want Stateful", props)
	}
	if props := qVerbMetadata("parse"); !props.Deterministic || qVerbMemoUnsafe(props) {
		t.Errorf("parse metadata = %+v, want deterministic", props)
	}
	// Session-dependent value/eval shapes are excluded from every memo layer.
	for _, src := range []string{
		`value "1+2"`, `value "x:1"`, "value d", "value `a", "eval t",
		"value[x]", "b:value v", "count value x",
	} {
		if EvalSourceCacheable(src) {
			t.Errorf("EvalSourceCacheable(%q) = true, want false (value/eval are stateful-capable)", src)
		}
		if qEvalConstantStatementSource(src) {
			t.Errorf("qEvalConstantStatementSource(%q) = true, want false", src)
		}
	}
	// Pure shapes stay memoizable: parse, and value over literal dict/enum
	// operands (the structural scan distinguishes them).
	for _, src := range []string{
		`parse "1+2"`,
		"value `a`b!1 2",
		"count value `bid`ask!99 101",
		"value `sym$`AAPL`MSFT",
	} {
		if !EvalSourceCacheable(src) {
			t.Errorf("EvalSourceCacheable(%q) = false, want true (deterministic shape)", src)
		}
	}
}
