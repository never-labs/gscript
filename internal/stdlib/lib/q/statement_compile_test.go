package q

import (
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// TestCompileQStatementExprCoverage pins which statement shapes compile and
// that compiled evaluation matches the string evaluator on a shared state.
func TestCompileQStatementExprCoverage(t *testing.T) {
	state := NewEvalState(nil)
	for _, stmt := range []string{
		"x:til 64", "f:0.25*x", "m:x mod 7", "s:`short$x", "b:m=1",
		"syms:8#`AAPL`MSFT`NVDA", "d:`a`b!(1 2;3 4)",
	} {
		if _, err := state.evalScript(stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}
	compiled := []string{
		"(til 64)*0.25",
		"((til 64) mod 8)*0.5",
		"`long$x",
		"\"h\"$x mod 95",
		"`short$(x mod 9)",
		"0^x",
		"9^m",
		"x mod 17",
		"x intersect 5 6 7",
		"8#`buy`sell`hold",
		"8#101001b",
		"x within 16 32",
		"d`a",
		"x[3]",
		"flip `a`b!(x;m)",
		"`a xasc flip `a`b!(m;x)",
		"upper string syms",
		"lower syms",
		"reverse x",
		"fills x",
		"1+((til 64) mod 4)",
		"x - 1+2",
		"(x;reverse x;8#x)",
		"5h",
		"1 2 3?2",
		"4_x",
		// Fused-probe families: compiled plan nodes bind the string
		// evaluator's fused kernels at compile time.
		"count where b",
		"count where (m>=1) and m<4",
		"count where m in 1 2 3",
		"count where x within 16 32",
		"count where syms like \"A*\"",
		"count where x in 1 2 3",
		"count where null x",
		"count fills x",
		"count deltas fills x",
		"count 10 xrank x",
		"count upper string syms",
		"+/x mod 17",
		"+/x*x",
		"+/fills x",
		"+/10 xrank x",
		"+/0 1 2 3?x",
		"+/`long$`int$x",
		"+/sqrt x",
		"sum x*x",
		"x@3",
	}
	for _, src := range compiled {
		expr := compiledQStatementExprCached(src)
		if expr == nil {
			t.Errorf("statement %q did not compile", src)
			continue
		}
		compiledValue, compiledErr := state.evalValueExpr(expr)
		stringValue, stringErr := state.eval(src)
		if compiledErr != nil || stringErr != nil {
			t.Errorf("statement %q: compiledErr=%v stringErr=%v", src, compiledErr, stringErr)
			continue
		}
		if !matchValue(compiledValue, stringValue) {
			t.Errorf("statement %q: compiled %v != string %v", src, compiledValue, stringValue)
		}
	}
	// Shapes that must stay on the string evaluator: semantic forms the
	// compiler cannot represent, and fused-probe reducer territory.
	uncompiled := []string{
		"$[1;2;3]",
		"if[1;x:2]",
		"do[3;x:x+1]",
		"{x+y}",
		"+/x+'y",
		"(+/)'x",
		"@[x;0;:;9]",
		"x each y",
		"sum v fby g",
		"\\v",
		"count sums x",
		"count flip `a`b!(x;m)",
		"+/x where m=1",
		"+/reverse x",
		"+/deltas x",
		"first asc x",
		"set y 5",
	}
	for _, src := range uncompiled {
		if expr := compiledQStatementExprCached(src); expr != nil {
			t.Errorf("statement %q compiled (%T); expected string-evaluator routing", src, expr)
		}
	}
}

// TestCompiledStatementLiteralReuse pins that compiled literal constants are
// not corrupted across evaluations (copy-on-write contract).
func TestCompiledStatementLiteralReuse(t *testing.T) {
	state := NewEvalState(nil)
	first, err := state.evalCachedOrStringUncached("8#`a`b`c", nil, nil, nil)
	if err != nil {
		t.Fatalf("first eval: %v", err)
	}
	second, err := state.evalCachedOrStringUncached("8#`a`b`c", nil, nil, nil)
	if err != nil {
		t.Fatalf("second eval: %v", err)
	}
	if !matchValue(first, second) {
		t.Fatalf("repeat eval mismatch: %v != %v", first, second)
	}
	if array, ok := first.(data.Array); !ok || array.Len() != 8 {
		t.Fatalf("unexpected take result %#v", first)
	}
}
