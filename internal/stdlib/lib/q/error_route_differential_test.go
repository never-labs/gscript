package q

// Error-route differential corpus.
//
// Every known-error expression asserted via assertEvalErrorContains in
// eval_test.go is extracted mechanically (go/ast) and replayed through the
// compiled-vs-string dual route (EvalScriptCompiledDifferential), then warm
// repeated on the same session. The contract:
//
//  1. both routes agree on the error (record.Match for every statement),
//  2. the error message is stable across a warm repeat on the same session
//     (no error-cache poisoning), and
//  3. a healthy expression still evaluates on that session afterwards.

import (
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"strconv"
	"strings"
	"testing"
)

// qErrorCorpusFloor guards the mechanical extraction: if eval_test.go is
// restructured so fewer error expressions are found, this fails loudly
// instead of silently testing nothing.
const qErrorCorpusFloor = 90

type qErrorCase struct {
	src  string
	want string
}

func extractQErrorCorpus(t *testing.T) []qErrorCase {
	t.Helper()
	fset := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fset, "eval_test.go", nil, 0)
	if err != nil {
		t.Fatalf("parse eval_test.go: %v", err)
	}
	var cases []qErrorCase
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok || ident.Name != "assertEvalErrorContains" || len(call.Args) != 3 {
			return true
		}
		src, okSrc := qErrorStringLiteral(call.Args[1])
		want, okWant := qErrorStringLiteral(call.Args[2])
		if !okSrc || !okWant {
			// Non-literal arguments (built strings) are skipped; the floor
			// below keeps the corpus from silently shrinking.
			return true
		}
		cases = append(cases, qErrorCase{src: src, want: want})
		return true
	})
	if len(cases) < qErrorCorpusFloor {
		t.Fatalf("extracted only %d error expressions from eval_test.go, floor %d; extraction may be broken", len(cases), qErrorCorpusFloor)
	}
	return cases
}

func qErrorStringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != gotoken.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func TestQErrorRouteDifferential(t *testing.T) {
	cases := extractQErrorCorpus(t)
	t.Logf("error-route corpus: %d expressions extracted from eval_test.go", len(cases))
	for i, tc := range cases {
		tc := tc
		t.Run(strings.ReplaceAll(tc.src, "/", "_"), func(t *testing.T) {
			_ = i
			state := NewEvalState(nil)

			records, err := state.EvalScriptCompiledDifferential(tc.src)
			if err == nil {
				t.Fatalf("Eval(%q) succeeded, eval_test.go expects an error containing %q", tc.src, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Eval(%q) error = %q, want substring %q", tc.src, err.Error(), tc.want)
			}
			for _, record := range records {
				if !record.Match {
					t.Errorf("compiled/string route divergence on statement %q (compiledErr=%v)", record.Statement, record.CompiledErr)
				}
			}
			first := err.Error()

			// Warm repeat on the SAME session: identical message, still both
			// routes agreeing. A poisoned plan/error cache would surface as a
			// different message or a success.
			records, err = state.EvalScriptCompiledDifferential(tc.src)
			if err == nil {
				t.Fatalf("warm repeat of %q succeeded after first error %q", tc.src, first)
			}
			if err.Error() != first {
				t.Errorf("warm repeat of %q changed the error\n  first  %q\n  repeat %q", tc.src, first, err.Error())
			}
			for _, record := range records {
				if !record.Match {
					t.Errorf("warm repeat compiled/string divergence on statement %q (compiledErr=%v)", record.Statement, record.CompiledErr)
				}
			}

			// The session must stay healthy: a trivial expression evaluates.
			value, verr := state.Eval("1+1")
			if verr != nil {
				t.Errorf("session poisoned after %q: 1+1 returned error %v", tc.src, verr)
			} else if got, ok := value.(int64); !ok || got != 2 {
				t.Errorf("session poisoned after %q: 1+1 = %#v", tc.src, value)
			}
		})
	}
}
