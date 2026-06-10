package q

import (
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// q `not` is right-to-left: it negates everything to its right, so
// `not x in s` means `not (x in s)`. These cases pin agreement between the
// three evaluation routes that parse q expressions independently: the string
// interpreter (bare expressions), the Pratt-parser-backed script binding
// plans (assignment right-hand sides), and the where-pipeline recognizer
// (deferred `idx:where ...` assignments feeding gather/reduce pipelines).
func TestNotMembershipPrecedenceAgreesAcrossRoutes(t *testing.T) {
	// til 64 minus {3,5} leaves 62 rows.
	routes := []struct {
		name string
		src  string
	}{
		{"string-interpreter", "x:til 64;count where not x in (3;5)"},
		{"binding-assignment-mask", "x:til 64;m:not x in (3;5);count where m"},
		{"where-pipeline-index", "x:til 64;idx:where not x in (3;5);count idx"},
		{"explicit-parens", "x:til 64;count where not (x in (3;5))"},
		{"vector-literal-set", "x:til 64;count where not x in 3 5"},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			assertEvalValue(t, route.src, int64(62))
		})
	}
}

func TestNotMembershipInsideCompoundWherePredicate(t *testing.T) {
	// til 64 with x>1 (drops 0,1) and x not in {3,5} leaves 60 rows.
	routes := []struct {
		name string
		src  string
	}{
		{"string-interpreter", "x:til 64;count where (x>1) and (not x in (3;5))"},
		{"where-pipeline-index", "x:til 64;idx:where (x>1) and (not x in (3;5));count idx"},
		{"explicit-parens", "x:til 64;idx:where (x>1) and (not (x in 3 5));count idx"},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			assertEvalValue(t, route.src, int64(60))
		})
	}
}

func TestNotBindsLooserThanComparisonAndLogical(t *testing.T) {
	// `not` consumes the whole expression to its right, including compare,
	// within, and and/or, matching the string interpreter and kdb q.
	assertEvalArray(t, "not 1 2 3 = 1 5 3", data.KindBool, []any{false, true, false})
	assertEvalArray(t, "not 1 2 3 within 2 3", data.KindBool, []any{true, false, false})
	assertEvalArray(t, "not 1 0 1 and 0 0 1", data.KindBool, []any{true, true, false})
	assertEvalArray(t, "not 1 0 1 or 0 0 1", data.KindBool, []any{false, true, false})
	// Assignment route must agree with the bare-expression route.
	assertEvalArray(t, "m:not 1 2 3 = 1 5 3;m", data.KindBool, []any{false, true, false})
	assertEvalValue(t, "m:not 1 0 1 and 0 0 1;count where m", int64(2))
}
