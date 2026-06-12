package q

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestQTrapAtForm(t *testing.T) {
	t.Run("no error passes result through", func(t *testing.T) {
		assertEvalValue(t, "@[{x+1};1;`oops]", int64(2))
		assertEvalValue(t, "g:{x*3};@[g;14;`e]", int64(42))
	})

	t.Run("data handler is the trapped result", func(t *testing.T) {
		assertEvalValue(t, "@[{1+`a};0;`caught]", data.Symbol("caught"))
		assertEvalValue(t, "@[{'x};\"sig\";42]", int64(42))
	})

	t.Run("callable handler receives the error text", func(t *testing.T) {
		assertEvalArray(t, "@[{'x};\"boom\";{(\"caught\";x)}]", data.KindString, []any{"caught", "boom"})
		assertEvalValue(t, "@[{'x};\"boom\";{x}]", "boom")
	})

	t.Run("trapped text equals the untrapped error text", func(t *testing.T) {
		_, err := Eval("{1+`a}[0]")
		if err == nil {
			t.Fatalf("expected untrapped type error")
		}
		got, terr := Eval("@[{1+`a};0;{x}]")
		if terr != nil {
			t.Fatalf("trap returned error: %v", terr)
		}
		if got != err.Error() {
			t.Fatalf("trap handler text = %#v, untrapped error = %q", got, err.Error())
		}
	})

	t.Run("handler error propagates", func(t *testing.T) {
		assertEvalErrorContains(t, "@[{'x};\"deep\";{'x}]", "deep")
	})
}

func TestQTrapDotForm(t *testing.T) {
	assertEvalValue(t, ".[{x+y};(1;2);`e]", int64(3))
	assertEvalValue(t, ".[{x+y};1 2;`e]", int64(3))
	assertEvalValue(t, ".[{x+y};(1;`a);`e]", data.Symbol("e"))
	assertEvalArray(t, ".[{x+y};(1;`a);{(\"caught\";x)}]", data.KindString,
		[]any{"caught", "operator \"+\" expects numeric operands"})
	assertEvalValue(t, ".[{42};();`e]", int64(42))
}

func TestQTrapNesting(t *testing.T) {
	// Inner trap catches first; the outer handler never runs.
	assertEvalArray(t, "@[{@[{'x};x;{(\"inner\";x)}]};\"deep\";{(\"outer\";x)}]",
		data.KindString, []any{"inner", "deep"})
	// Inner handler re-signals; the outer trap catches the re-raised text.
	assertEvalArray(t, "@[{@[{'x};x;{'x}]};\"sig\";{(\"outer\";x)}]",
		data.KindString, []any{"outer", "sig"})
	// A signal crosses a non-trapping lambda frame to the nearest trap.
	assertEvalValue(t, "f:{'x};g:{f[x]};@[g;\"thru\";{x}]", "thru")
}

func TestQTripleAmendDisambiguation(t *testing.T) {
	// Data first argument: @[v;i;f] is amend-with-unary-op, not trap.
	assertEvalArray(t, "@[10 20 30;1;{x*10}]", data.KindI64,
		[]any{int64(10), int64(200), int64(30)})
	assertEvalArray(t, "@[10 20 30;0 2;neg]", data.KindI64,
		[]any{int64(-10), int64(20), int64(-30)})
	assertEvalDict(t, "@[`a`b!10 20;`a;neg]", []data.Symbol{"a", "b"},
		[]any{int64(-10), int64(20)})
	assertEvalArray(t, "v:10 20 30;@[v;1;{x*10}]", data.KindI64,
		[]any{int64(10), int64(200), int64(30)})
	// Copy-on-write: the named source is unchanged.
	assertEvalArray(t, "v:10 20 30;r:@[v;1;{x*10}];v", data.KindI64,
		[]any{int64(10), int64(20), int64(30)})
	// .[d;path;f] deep unary amend.
	assertEvalArray(t, "m:(1 2;3 4);r:.[m;(0;1);{x+100}];r[0]", data.KindI64,
		[]any{int64(1), int64(102)})
	assertEvalValue(t, "d:`a`b!(10;`c`d!20 30);(.[d;(`b;`d);{x+1}]) . (`b;`d)", int64(31))
	// Data first with a non-callable third argument is an amend error.
	assertEvalErrorContains(t, "@[10 20 30;1;2]", "amend function is not callable")
}

func TestQSignalForm(t *testing.T) {
	t.Run("string and symbol literals", func(t *testing.T) {
		assertEvalErrorContains(t, "'\"err\"", "err")
		assertEvalErrorContains(t, "'`oops", "oops")
	})

	t.Run("signal text is exactly the operand", func(t *testing.T) {
		_, err := Eval("'\"some error text\"")
		if err == nil || err.Error() != "some error text" {
			t.Fatalf("signal error = %v, want exactly %q", err, "some error text")
		}
	})

	t.Run("bound name signals its string or symbol value", func(t *testing.T) {
		assertEvalErrorContains(t, "e:\"bad input\";'e", "bad input")
		assertEvalErrorContains(t, "e:`badsym;'e", "badsym")
		assertEvalErrorContains(t, "e:42;'e", "signal expects a string or symbol")
	})

	t.Run("signal inside control flow", func(t *testing.T) {
		assertEvalValue(t, "f:{$[x<0;'\"neg\";x*2]};f[3]", int64(6))
		assertEvalErrorContains(t, "f:{$[x<0;'\"neg\";x*2]};f[-1]", "neg")
		assertEvalArray(t, "f:{$[x<0;'\"neg\";x*2]};@[f;-1;{(\"got\";x)}]",
			data.KindString, []any{"got", "neg"})
	})

	t.Run("unclaimed quote statements keep their errors", func(t *testing.T) {
		// Bare quote and structured operands are outside the signal scope.
		if _, err := Eval("'"); err == nil {
			t.Fatalf("bare quote should still error")
		}
		if _, err := Eval("'(1;2)"); err == nil {
			t.Fatalf("structured signal operand should still error")
		}
	})
}

// TestQTrapPlanStringRouteAgree drives the same trap/amend statements through
// the compiled apply-form plan and the per-call string walk on identically
// prepared sessions; both routes must produce the same value or error.
func TestQTrapPlanStringRouteAgree(t *testing.T) {
	setup := "g:{x+1};h:{x+y};hnd:{(\"c\";x)};v:10 20 30;dbl:{x*2}"
	srcs := []string{
		"@[g;41;`e]",
		"@[g;`bad;hnd]",
		"@[g;`bad;`fallback]",
		".[h;(1;2);`e]",
		".[h;(1;`a);hnd]",
		".[h;(1;`a);`fallback]",
		"@[v;1;dbl]",
		"@[v;0 2;dbl]",
	}
	for _, src := range srcs {
		planState := NewEvalState(nil)
		if _, err := planState.Eval(setup); err != nil {
			t.Fatalf("setup: %v", err)
		}
		stringState := NewEvalState(nil)
		if _, err := stringState.Eval(setup); err != nil {
			t.Fatalf("setup: %v", err)
		}
		planOut, planHandled, planErr := planState.evalQApplyFormPlan(src)
		if !planHandled && planErr == nil {
			t.Errorf("plan route did not claim %q", src)
			continue
		}
		stringOut, stringHandled, stringErr := stringState.evalApplyIndexForm(src)
		if !stringHandled && stringErr == nil {
			t.Errorf("string route did not claim %q", src)
			continue
		}
		if (planErr == nil) != (stringErr == nil) {
			t.Errorf("route error divergence on %q: plan=%v string=%v", src, planErr, stringErr)
			continue
		}
		if planErr != nil {
			if planErr.Error() != stringErr.Error() {
				t.Errorf("route error text divergence on %q: plan=%q string=%q", src, planErr, stringErr)
			}
			continue
		}
		if !matchValue(planOut, stringOut) {
			t.Errorf("route value divergence on %q: plan=%#v string=%#v", src, planOut, stringOut)
		}
	}
}

// TestQTrapWarmRepeat guards the plan caches: a trapped error must not poison
// the cached plan for subsequent successful calls on the same session.
func TestQTrapWarmRepeat(t *testing.T) {
	state := NewEvalState(nil)
	if _, err := state.Eval("g:{$[x<0;'\"neg\";x+1]}"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	for i := 0; i < 3; i++ {
		out, err := state.Eval("@[g;-1;{x}]")
		if err != nil {
			t.Fatalf("warm trap run %d: %v", i, err)
		}
		if out != "neg" {
			t.Fatalf("warm trap run %d = %#v, want \"neg\"", i, out)
		}
		ok, err := state.Eval("@[g;41;`e]")
		if err != nil {
			t.Fatalf("warm success run %d: %v", i, err)
		}
		if ok != int64(42) {
			t.Fatalf("warm success run %d = %#v, want 42", i, ok)
		}
	}
}

// TestQSignalErrorTextStable ensures the signal error survives the dual-route
// differential and a warm repeat unchanged.
func TestQSignalErrorTextStable(t *testing.T) {
	state := NewEvalState(nil)
	records, err := state.EvalScriptCompiledDifferential("'\"stable text\"")
	if err == nil || err.Error() != "stable text" {
		t.Fatalf("differential signal error = %v, want %q", err, "stable text")
	}
	for _, record := range records {
		if !record.Match {
			t.Errorf("route divergence on %q", record.Statement)
		}
	}
	if _, err := state.Eval("'\"stable text\""); err == nil || err.Error() != "stable text" {
		t.Fatalf("warm signal error = %v, want %q", err, "stable text")
	}
	if out, err := state.Eval("1+1"); err != nil || out != int64(2) {
		t.Fatalf("session unusable after signal: out=%v err=%v", out, err)
	}
}

func TestQSymbolDelimiterBrace(t *testing.T) {
	// `a}` must end the symbol at the brace: lambda bodies ending in a symbol
	// literal split correctly inside bracket forms.
	parts := splitTopLevelDelim("{1+`a};0;{x}", ';')
	if len(parts) != 3 {
		t.Fatalf("splitTopLevelDelim = %q, want 3 parts", parts)
	}
	if got := strings.TrimSpace(parts[0]); got != "{1+`a}" {
		t.Fatalf("first part = %q", got)
	}
}
