package q

import (
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// assertEvalMatches evaluates src and want on fresh sessions and compares
// with q match (~) semantics, so table/keyed/dict results compare by value.
func assertEvalMatches(t *testing.T, src, want string) {
	t.Helper()
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	expected, err := Eval(want)
	if err != nil {
		t.Fatalf("Eval(%q) (expected spec) returned error: %v", want, err)
	}
	if !matchValue(got, expected) {
		t.Fatalf("Eval(%q) = %#v (%T), want %q = %#v (%T)", src, got, got, want, expected, expected)
	}
}

func TestFunctionalSelect(t *testing.T) {
	// Canonical operator-atom constraint parse tree.
	assertEvalMatches(t,
		"?[([] a:1 2 3;b:10 20 30);enlist (>;`a;1);0b;()]",
		"([] a:2 3;b:20 30)")
	// Multiple constraints conjoin like chained where clauses.
	assertEvalMatches(t,
		"?[([] a:1 2 3 4;b:10 20 30 40);((>;`a;1);(<;`a;4));0b;()]",
		"([] a:2 3;b:20 30)")
	// Composite comparison atoms (<= >= <>) work as parse-tree heads.
	assertEvalMatches(t,
		"?[([] a:1 2 3);enlist (<=;`a;2);0b;()]",
		"([] a:1 2)")
	assertEvalMatches(t,
		"?[([] a:1 2 3);enlist (<>;`a;2);0b;()]",
		"([] a:1 3)")
	// in over a symbol vector literal; = against an enlisted symbol literal.
	assertEvalMatches(t,
		"?[([] s:`x`y`z;b:1 2 3);enlist (in;`s;`x`z);0b;()]",
		"([] s:`x`z;b:1 3)")
	assertEvalMatches(t,
		"?[([] s:`x`y`x;b:1 2 3);enlist (=;`s;enlist `x);0b;()]",
		"([] s:`x`x;b:1 3)")
	// within over a two-item literal vector.
	assertEvalMatches(t,
		"?[([] a:1 2 3 4);enlist (within;`a;2 3);0b;()]",
		"([] a:2 3)")
	// Projection dict with a computed column.
	assertEvalMatches(t,
		"?[([] a:1 2 3;b:10 20 30);();0b;(enlist `b)!enlist `b]",
		"([] b:10 20 30)")
	// Source by name; select does not write back.
	assertEvalMatches(t,
		"t:([] a:1 2 3); r:?[`t;enlist (>;`a;1);0b;()]; r",
		"([] a:2 3)")
	// Distinct via b=1b.
	assertEvalMatches(t,
		"?[([] s:`x`y`x);();1b;()]",
		"([] s:`x`y)")
	// Five-argument row limit.
	assertEvalMatches(t,
		"?[([] a:1 2 3 4);();0b;();2]",
		"([] a:1 2)")
}

func TestFunctionalSelectGrouped(t *testing.T) {
	src := "?[([] s:`x`y`x;b:10 20 30);();(enlist `s)!enlist `s;(enlist `t)!enlist (sum;`b)]"
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("Eval(%q) = %T, want data.KeyedFrame", src, got)
	}
	if keys := keyed.Keys(); len(keys) != 1 || keys[0] != "s" {
		t.Fatalf("grouped select keys = %v, want [s]", keys)
	}
	column, ok := keyed.Frame().Column("t")
	if !ok || column.Len() != 2 {
		t.Fatalf("grouped select aggregate column missing or wrong length")
	}
	first, _ := column.At(0)
	second, _ := column.At(1)
	if first != int64(40) || second != int64(20) {
		t.Fatalf("grouped sums = %v %v, want long 40 20", first, second)
	}
}

func TestFunctionalExec(t *testing.T) {
	// Single column list.
	assertEvalMatches(t, "?[([] a:1 2 3);();();`a]", "1 2 3")
	// Whole-table aggregate yields an atom.
	assertEvalMatches(t, "?[([] b:10 20 30);();();(sum;`b)]", "60")
	// Aggregate respects constraints.
	assertEvalMatches(t, "?[([] a:1 2 3;b:10 20 30);enlist (>;`a;1);();(sum;`b)]", "50")
	// Dict projection yields a dict.
	assertEvalMatches(t,
		"?[([] a:1 2 3;b:10 20 30);();();`x`y!(`a;`b)]",
		"`x`y!(1 2 3;10 20 30)")
}

func TestFunctionalUpdateDelete(t *testing.T) {
	assertEvalMatches(t,
		"![([] a:1 2 3;b:10 20 30);enlist (>;`a;1);0b;(enlist `b)!enlist (*;`b;2)]",
		"([] a:1 2 3;b:10 40 60)")
	assertEvalMatches(t,
		"![([] a:1 2 3;b:10 20 30);();0b;enlist `b]",
		"([] a:1 2 3)")
	// Delete rows with the parseable empty-symbol-list spelling `$()
	// (canonical 0#` does not parse in this dialect) and with ().
	assertEvalMatches(t,
		"![([] a:1 2 3;b:10 20 30);enlist (>;`a;1);0b;`$()]",
		"1#([] a:1 2 3;b:10 20 30)")
	assertEvalMatches(t,
		"![([] a:1 2 3);enlist (=;`a;2);0b;()]",
		"([] a:1 3)")
	// Empty constraints and empty columns delete every row.
	src := "![([] a:1 2 3);();0b;`$()]"
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	frame, ok := got.(data.Frame)
	if !ok || frame.Len() != 0 {
		t.Fatalf("Eval(%q) = %#v, want empty frame", src, got)
	}
}

func TestFunctionalAmendByName(t *testing.T) {
	// Amend by symbol name writes back and returns the name symbol.
	state := NewEvalState(nil)
	if _, err := state.Eval("t:([] a:1 2)"); err != nil {
		t.Fatal(err)
	}
	out, err := state.Eval("![`t;();0b;(enlist `a)!enlist (*;`a;10)]")
	if err != nil {
		t.Fatal(err)
	}
	if sym, ok := out.(data.Symbol); !ok || sym != "t" {
		t.Fatalf("amend by name returned %#v (%T), want `t", out, out)
	}
	updated, err := state.Eval("t")
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := Eval("([] a:10 20)")
	if !matchValue(updated, expected) {
		t.Fatalf("amend by name did not write back: t = %#v", updated)
	}

	// Warm session repeats re-apply the mutation (no value memoization).
	session := NewEvalSession(nil)
	if _, err := session.Eval("t:([] a:1 2)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := session.Eval("![`t;();0b;(enlist `a)!enlist (*;`a;10)]"); err != nil {
			t.Fatal(err)
		}
	}
	twice, err := session.Eval("t")
	if err != nil {
		t.Fatal(err)
	}
	expectedTwice, _ := Eval("([] a:100 200)")
	if !matchValue(twice, expectedTwice) {
		t.Fatalf("warm amend by name applied wrong number of times: t = %#v", twice)
	}
}

func TestFunctionalKeyedTable(t *testing.T) {
	assertEvalMatches(t,
		"![([k:1 2 3] v:10 20 30);enlist (>;`k;1);0b;(enlist `v)!enlist (+;`v;5)]",
		"([k:1 2 3] v:10 25 35)")
	src := "?[([k:1 2] v:10 20);enlist (>;`v;10);0b;()]"
	got, err := Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", src, err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("Eval(%q) = %T, want data.KeyedFrame (keys preserved)", src, got)
	}
	if keyed.Frame().Len() != 1 {
		t.Fatalf("keyed select rows = %d, want 1", keyed.Frame().Len())
	}
}

func TestFunctionalFormErrors(t *testing.T) {
	assertEvalErrorContains(t, "?[1;2;3;4]", "expects a table")
	assertEvalErrorContains(t, "?[([] a:1 2);1b;0b;()]", "constraint spec")
	assertEvalErrorContains(t, "?[([] a:1 2);();();()]", "exec requires a projection")
	assertEvalErrorContains(t, "?[([] a:1 2);();0b;();-1]", "row limit")
	assertEvalErrorContains(t, "![`nosuchtable;();0b;enlist `a]", "is not bound to a table")
	assertEvalErrorContains(t, "![([] a:1 2;b:3 4);enlist (>;`a;1);0b;enlist `b]", "cannot specify both")
	assertEvalErrorContains(t, "![([] a:1 2);();0b;42]", "dict of assignments or a list of column symbols")
	assertEvalErrorContains(t, "?[([] a:1 2);enlist ({x>1};`a);0b;()]", "not bridgeable")
}

func TestFunctionalSourceEligibility(t *testing.T) {
	// Amend by name mutates the workspace: never memoizable, with or
	// without whitespace after the bracket.
	for _, src := range []string{
		"![`t;();0b;(enlist `a)!enlist (*;`a;10)]",
		"![ `t;();0b;(enlist `a)!enlist (*;`a;10)]",
	} {
		if EvalSourceCacheable(src) {
			t.Errorf("EvalSourceCacheable(%q) = true, want false (workspace write-back)", src)
		}
	}
	// Value-form functional queries stay deterministic.
	for _, src := range []string{
		"?[([] a:1 2 3);enlist (>;`a;1);0b;()]",
		"![([] a:1 2 3);();0b;enlist `a]",
		"`a`b!1 2",
	} {
		if !EvalSourceCacheable(src) {
			t.Errorf("EvalSourceCacheable(%q) = false, want true", src)
		}
	}
}

// TestFunctionalDualRouteDifferential replays functional-form scripts through
// the compiled-vs-string differential oracle: any statement the compiled
// route claims must agree with the string route.
func TestFunctionalDualRouteDifferential(t *testing.T) {
	scripts := []string{
		"t:([] a:1 2 3;b:10 20 30); ?[t;enlist (>;`a;1);0b;()]",
		"t:([] a:1 2 3;b:10 20 30); r:?[t;();();(sum;`b)]; r",
		"t:([] a:1 2 3;b:10 20 30); ![t;();0b;enlist `b]",
		"t:([] a:1 2 3); ![`t;();0b;(enlist `a)!enlist (*;`a;10)]; t",
		"?[([] s:`x`y`x;b:10 20 30);();(enlist `s)!enlist `s;(enlist `t)!enlist (sum;`b)]",
	}
	for _, src := range scripts {
		state := NewEvalState(nil)
		records, err := state.EvalScriptCompiledDifferential(src)
		if err != nil {
			t.Fatalf("differential %q: %v", src, err)
		}
		for _, record := range records {
			if !record.Match {
				t.Errorf("differential mismatch %q statement %q (compiled=%v err=%v)",
					src, record.Statement, record.Compiled, record.CompiledErr)
			}
		}
	}
}

func TestFunctionalConstraintSpellings(t *testing.T) {
	// The operator atoms used by parse trees evaluate standalone.
	for _, op := range []string{"<=", ">=", "<>"} {
		v, err := Eval(op)
		if err != nil {
			t.Fatalf("Eval(%q) returned error: %v", op, err)
		}
		fn, ok := v.(qDyadicFunction)
		if !ok || fn.name != op {
			t.Fatalf("Eval(%q) = %#v, want dyadic %q atom", op, v, op)
		}
		out, err := fn.fn(int64(1), int64(2))
		if err != nil {
			t.Fatalf("apply %q: %v", op, err)
		}
		want := op != ">="
		if out != want {
			t.Fatalf("1 %s 2 = %v, want %v", op, out, want)
		}
	}
	// A parse tree built from q literals evaluates to (fn;sym;literal),
	// which the functional bridge consumes as-is.
	tree, err := Eval("(>;`price;100)")
	if err != nil {
		t.Fatal(err)
	}
	list, ok := tree.(data.Array)
	if !ok || list.Len() != 3 {
		t.Fatalf("(>;`price;100) = %#v, want 3-item list", tree)
	}
	head, _ := list.At(0)
	if fn, ok := head.(qDyadicFunction); !ok || fn.name != ">" {
		t.Fatalf("parse tree head = %#v, want > atom", head)
	}
}

func TestRawLiteralLowering(t *testing.T) {
	expr, err := lowerExpr(RawLiteral{Value: data.Symbol("x")})
	if err != nil {
		t.Fatal(err)
	}
	lit, ok := expr.(data.Literal)
	if !ok || lit.Value != data.Symbol("x") {
		t.Fatalf("lowerExpr(RawLiteral) = %#v", expr)
	}
	value, err := lowerLiteralValue(RawLiteral{Value: int64(7)})
	if err != nil || value != int64(7) {
		t.Fatalf("lowerLiteralValue(RawLiteral) = %#v err=%v", value, err)
	}
}

func TestMatchValueFrames(t *testing.T) {
	// Lazy view columns (index views, fused kernels) compare by value.
	left, err := Eval("?[([] s:`x`y`x;b:1 2 3);enlist (=;`s;enlist `x);0b;()]")
	if err != nil {
		t.Fatal(err)
	}
	right, err := Eval("([] s:`x`x;b:1 3)")
	if err != nil {
		t.Fatal(err)
	}
	if !matchValue(left, right) {
		t.Fatalf("frame view did not match dense frame:\n left %#v\n right %#v", left, right)
	}
	other, _ := Eval("([] s:`x`x;b:1 4)")
	if matchValue(left, other) {
		t.Fatal("mismatched frames reported as matching")
	}
	reordered, _ := Eval("([] b:1 3;s:`x`x)")
	if matchValue(left, reordered) {
		t.Fatal("column order must be significant for table match")
	}
}
