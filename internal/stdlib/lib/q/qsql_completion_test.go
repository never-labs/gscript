package q

// qSQL completion-round feature tests: ungrouped aggregates, aggregate
// subexpressions in where, subquery sources, canonical select[n] forms,
// like-in-where, grouped window updates, wavg[w;v], the virtual column i,
// and the kind-preserving integer aggregate/projection rules.

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func qCompletionFrame(t *testing.T) data.Frame {
	t.Helper()
	frame, err := data.NewFrame(
		data.NewColumn("g", []any{data.Symbol("a"), data.Symbol("b"), data.Symbol("a"), data.Symbol("b")}),
		data.NewColumn("v", []any{int64(1), int64(2), int64(3), int64(4)}),
		data.NewColumn("w", []any{int64(1), int64(1), int64(2), int64(2)}),
		data.NewColumn("px", []any{10.5, 20.5, 30.5, 40.5}),
		data.NewColumn("name", []any{"Alpha", "Beta", "Anna", "Borg"}),
	)
	if err != nil {
		t.Fatalf("frame: %v", err)
	}
	return frame
}

func qCompletionRun(t *testing.T, src string) data.Frame {
	t.Helper()
	lowered, err := Lower(mustParse(t, src))
	if err != nil {
		t.Fatalf("Lower(%q): %v", src, err)
	}
	out, err := lowered.ExecRuntime(qCompletionFrame(t))
	if err != nil {
		t.Fatalf("ExecRuntime(%q): %v", src, err)
	}
	return out
}

func qCompletionColumn(t *testing.T, frame data.Frame, name data.Symbol) data.Array {
	t.Helper()
	col, ok := frame.Column(name)
	if !ok {
		t.Fatalf("missing column %q in %v", name, frame.Schema().Names())
	}
	return col
}

func qCompletionAt(t *testing.T, frame data.Frame, name data.Symbol, row int) any {
	t.Helper()
	v, ok := qCompletionColumn(t, frame, name).At(row)
	if !ok {
		t.Fatalf("column %q row %d out of range", name, row)
	}
	return v
}

// --- 1. Ungrouped aggregates -----------------------------------------------

func TestQSQLUngroupedAggregatesReturnOneRowTable(t *testing.T) {
	out := qCompletionRun(t, "select s:sum v,c:count i,m:max v,a:avg v from t")
	if out.Len() != 1 {
		t.Fatalf("rows = %d, want 1", out.Len())
	}
	if got := qCompletionAt(t, out, "s", 0); got != int64(10) {
		t.Fatalf("sum = %#v, want long 10", got)
	}
	if got := qCompletionAt(t, out, "c", 0); got != int64(4) {
		t.Fatalf("count = %#v, want 4", got)
	}
	if got := qCompletionAt(t, out, "m", 0); got != int64(4) {
		t.Fatalf("max = %#v, want long 4", got)
	}
	if got := qCompletionAt(t, out, "a", 0); got != 2.5 {
		t.Fatalf("avg = %#v, want 2.5", got)
	}
}

func TestQSQLUngroupedAggregatesApplyWhereBeforeAggregation(t *testing.T) {
	out := qCompletionRun(t, "select s:sum v from t where v>1")
	if out.Len() != 1 {
		t.Fatalf("rows = %d, want 1", out.Len())
	}
	if got := qCompletionAt(t, out, "s", 0); got != int64(9) {
		t.Fatalf("filtered sum = %#v, want long 9", got)
	}
}

func TestFunctionalUngroupedAggregatesUseColumnarPath(t *testing.T) {
	got, err := Eval("?[([] v:1 2 3);();();(sum;`v)]")
	if err != nil {
		t.Fatalf("functional exec sum: %v", err)
	}
	if got != int64(6) {
		t.Fatalf("functional exec sum = %#v, want long 6", got)
	}
	got, err = Eval("?[([] v:1 2 3);();();(enlist `s)!enlist (sum;`v)]")
	if err != nil {
		t.Fatalf("functional exec sum dict: %v", err)
	}
	dict, ok := got.(EvalDict)
	if !ok || len(dict.Values) != 1 || dict.Values[0] != int64(6) {
		t.Fatalf("functional exec sum dict = %#v, want dict with long 6", got)
	}
	got, err = Eval("?[([] v:1 2 3);();0b;(enlist `s)!enlist (sum;`v)]")
	if err != nil {
		t.Fatalf("functional select sum: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok || frame.Len() != 1 {
		t.Fatalf("functional select sum = %#v, want one-row table", got)
	}
	if v, _ := qCompletionColumn(t, frame, "s").At(0); v != int64(6) {
		t.Fatalf("functional select sum value = %#v, want long 6", v)
	}
}

// --- 2. Aggregate subexpressions in where ------------------------------------

func TestQSQLAggregateInWhereFiltersAgainstWholeColumn(t *testing.T) {
	out := qCompletionRun(t, "select v from t where v=max v")
	if out.Len() != 1 {
		t.Fatalf("rows = %d, want 1", out.Len())
	}
	if got := qCompletionAt(t, out, "v", 0); got != int64(4) {
		t.Fatalf("max row v = %#v, want 4", got)
	}
	out = qCompletionRun(t, "select v from t where v>avg v")
	if out.Len() != 2 {
		t.Fatalf("rows above mean = %d, want 2", out.Len())
	}
}

func TestFunctionalAggregateInConstraint(t *testing.T) {
	got, err := Eval("?[([] v:1 2 3 4);enlist (=;`v;(max;`v));0b;(enlist `v)!enlist `v]")
	if err != nil {
		t.Fatalf("functional aggregate constraint: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok || frame.Len() != 1 {
		t.Fatalf("functional aggregate constraint = %#v, want one-row table", got)
	}
	if v, _ := qCompletionColumn(t, frame, "v").At(0); v != int64(4) {
		t.Fatalf("constraint row v = %#v, want 4", v)
	}
}

// --- 3. Subqueries ------------------------------------------------------------

func TestQSQLSubquerySource(t *testing.T) {
	out := qCompletionRun(t, "select from (select v,px from t where v>1) where v<4")
	if out.Len() != 2 {
		t.Fatalf("rows = %d, want 2", out.Len())
	}
	names := out.Schema().Names()
	if len(names) != 2 || names[0] != "v" || names[1] != "px" {
		t.Fatalf("subquery projection columns = %v, want [v px]", names)
	}
	// Aggregate over a subquery result.
	out = qCompletionRun(t, "select s:sum v from (select v from t where v>1)")
	if got := qCompletionAt(t, out, "s", 0); got != int64(9) {
		t.Fatalf("sum over subquery = %#v, want long 9", got)
	}
	// Nested subqueries chain innermost-first.
	out = qCompletionRun(t, "select from (select from (select v from t where v>1) where v>2)")
	if out.Len() != 2 {
		t.Fatalf("nested subquery rows = %d, want 2", out.Len())
	}
}

func TestQSQLSubqueryRestrictions(t *testing.T) {
	if _, err := Lower(mustParse(t, "update v:v+1 from (select v from t)")); err == nil || !strings.Contains(err.Error(), "subquery") {
		t.Fatalf("mutation over subquery error = %v, want subquery rejection", err)
	}
	if _, err := Lower(mustParse(t, "select from (exec v from t)")); err == nil || !strings.Contains(err.Error(), "subquery must be a select") {
		t.Fatalf("exec subquery error = %v, want select-only rejection", err)
	}
}

// --- 4. select[n] / select[>c] / select[n;>c] ---------------------------------

func TestQSQLSelectBracketForms(t *testing.T) {
	out := qCompletionRun(t, "select[2] v from t")
	if out.Len() != 2 {
		t.Fatalf("select[2] rows = %d, want 2", out.Len())
	}
	if got := qCompletionAt(t, out, "v", 0); got != int64(1) {
		t.Fatalf("select[2] first v = %#v, want 1", got)
	}

	out = qCompletionRun(t, "select[>v] v from t")
	if got := qCompletionAt(t, out, "v", 0); got != int64(4) {
		t.Fatalf("select[>v] first v = %#v, want 4", got)
	}
	out = qCompletionRun(t, "select[<v] v from t")
	if got := qCompletionAt(t, out, "v", 0); got != int64(1) {
		t.Fatalf("select[<v] first v = %#v, want 1", got)
	}

	out = qCompletionRun(t, "select[2;>v] v from t")
	if out.Len() != 2 {
		t.Fatalf("select[2;>v] rows = %d, want 2", out.Len())
	}
	if first, second := qCompletionAt(t, out, "v", 0), qCompletionAt(t, out, "v", 1); first != int64(4) || second != int64(3) {
		t.Fatalf("select[2;>v] = %#v %#v, want 4 3", first, second)
	}
}

func TestQSQLSelectBracketFormErrors(t *testing.T) {
	if _, err := Parse("select[2;3] v from t"); err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("select[2;3] error = %v, want duplicate row limit", err)
	}
	if _, err := Parse("select[>] v from t"); err == nil {
		t.Fatalf("select[>] parsed; want sort column error")
	}
	if _, err := Parse("select[2 v from t"); err == nil {
		t.Fatalf("unterminated select[ parsed; want error")
	}
}

// --- 5a. like in where ---------------------------------------------------------

func TestQSQLLikeInWhere(t *testing.T) {
	out := qCompletionRun(t, "select name from t where name like \"A*\"")
	if out.Len() != 2 {
		t.Fatalf("like rows = %d, want 2", out.Len())
	}
	if got := qCompletionAt(t, out, "name", 0); got != "Alpha" {
		t.Fatalf("like row 0 = %#v, want Alpha", got)
	}
	out = qCompletionRun(t, "select name from t where name like \"B*\",v>2")
	if out.Len() != 1 {
		t.Fatalf("like+compare rows = %d, want 1", out.Len())
	}
	if _, err := Lower(mustParse(t, "select from t where name like v")); err == nil || !strings.Contains(err.Error(), "literal") {
		t.Fatalf("non-literal like pattern error = %v, want literal pattern requirement", err)
	}
}

func TestQSQLLikeOnSymbolColumn(t *testing.T) {
	out := qCompletionRun(t, "select g from t where g like \"a\"")
	if out.Len() != 2 {
		t.Fatalf("symbol like rows = %d, want 2", out.Len())
	}
}

// --- 5b. Grouped window updates -------------------------------------------------

func TestQSQLGroupedWindowUpdate(t *testing.T) {
	lowered, err := Lower(mustParse(t, "update s:sums v by g from t"))
	if err != nil {
		t.Fatalf("Lower grouped window update: %v", err)
	}
	if lowered.Mutation == nil || len(lowered.Mutation.Assignments) != 1 || lowered.Mutation.Assignments[0].Func != "" {
		t.Fatalf("mutation = %#v, want window assignment", lowered.Mutation)
	}
	out, err := qFunctionalRunMutation(qCompletionFrame(t), lowered.Mutation)
	if err != nil {
		t.Fatalf("grouped window update exec: %v", err)
	}
	// g: a b a b, v: 1 2 3 4 -> per-group running sums: a:[1 4] b:[2 6]
	// (the sums vector transform accumulates in float64).
	want := []float64{1, 2, 4, 6}
	for row, w := range want {
		if got := qCompletionAt(t, out, "s", row); got != w {
			t.Fatalf("sums row %d = %#v, want %v", row, got, w)
		}
	}
	// Aggregate assignments still broadcast per group.
	lowered, err = Lower(mustParse(t, "update m:max v by g from t"))
	if err != nil {
		t.Fatalf("Lower grouped aggregate update: %v", err)
	}
	out, err = qFunctionalRunMutation(qCompletionFrame(t), lowered.Mutation)
	if err != nil {
		t.Fatalf("grouped aggregate update exec: %v", err)
	}
	wantMax := []int64{3, 4, 3, 4}
	for row, w := range wantMax {
		if got := qCompletionAt(t, out, "m", row); got != w {
			t.Fatalf("max row %d = %#v, want %d", row, got, w)
		}
	}
}

func TestFunctionalGroupedWindowUpdate(t *testing.T) {
	got, err := Eval("![([] g:`a`b`a`b;v:1 2 3 4);();(enlist `g)!enlist `g;(enlist `s)!enlist (sums;`v)]")
	if err != nil {
		t.Fatalf("functional grouped window update: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("functional grouped window update = %T, want frame", got)
	}
	want := []float64{1, 2, 4, 6}
	for row, w := range want {
		if got := qCompletionAt(t, frame, "s", row); got != w {
			t.Fatalf("functional sums row %d = %#v, want %v", row, got, w)
		}
	}
}

// --- 5c. wavg[w;v] ---------------------------------------------------------------

func TestQSQLWavgCarriesWeight(t *testing.T) {
	out := qCompletionRun(t, "select x:wavg[w;v] from t")
	// (1*1 + 1*2 + 2*3 + 2*4) / (1+1+2+2) = 17/6
	if got := qCompletionAt(t, out, "x", 0); got != 17.0/6.0 {
		t.Fatalf("wavg = %#v, want 17/6", got)
	}
	out = qCompletionRun(t, "select x:wavg[w;v] by g from t")
	if out.Len() != 2 {
		t.Fatalf("grouped wavg rows = %d, want 2", out.Len())
	}
	// a: (1*1+2*3)/3 = 7/3, b: (1*2+2*4)/3 = 10/3
	if got := qCompletionAt(t, out, "x", 0); got != 7.0/3.0 {
		t.Fatalf("grouped wavg a = %#v, want 7/3", got)
	}
	if got := qCompletionAt(t, out, "x", 1); got != 10.0/3.0 {
		t.Fatalf("grouped wavg b = %#v, want 10/3", got)
	}
	if _, err := Lower(mustParse(t, "select x:wavg v from t")); err == nil || !strings.Contains(err.Error(), "wavg") {
		t.Fatalf("single-argument wavg error = %v, want wavg arity diagnostic", err)
	}
}

func TestFunctionalWavgParseTree(t *testing.T) {
	got, err := Eval("?[([] w:1 1 2;v:10 20 30);();0b;(enlist `x)!enlist (wavg;`w;`v)]")
	if err != nil {
		t.Fatalf("functional wavg: %v", err)
	}
	frame, ok := got.(data.Frame)
	if !ok || frame.Len() != 1 {
		t.Fatalf("functional wavg = %#v, want one-row table", got)
	}
	if v, _ := qCompletionColumn(t, frame, "x").At(0); v != 22.5 {
		t.Fatalf("functional wavg = %#v, want 22.5", v)
	}
}

// --- 7. Virtual column i ----------------------------------------------------------

func TestQSQLVirtualRowIndexInWhere(t *testing.T) {
	out := qCompletionRun(t, "select v from t where i<2")
	if out.Len() != 2 {
		t.Fatalf("i<2 rows = %d, want 2", out.Len())
	}
	if got := qCompletionAt(t, out, "v", 1); got != int64(2) {
		t.Fatalf("i<2 second v = %#v, want 2", got)
	}
	out = qCompletionRun(t, "select v from t where i>2")
	if out.Len() != 1 {
		t.Fatalf("i>2 rows = %d, want 1", out.Len())
	}
	if got := qCompletionAt(t, out, "v", 0); got != int64(4) {
		t.Fatalf("i>2 v = %#v, want 4", got)
	}
}

func TestQSQLVirtualRowIndexPrefersRealColumn(t *testing.T) {
	frame, err := data.NewFrame(
		data.NewColumn("i", []any{int64(10), int64(20), int64(30)}),
		data.NewColumn("v", []any{int64(1), int64(2), int64(3)}),
	)
	if err != nil {
		t.Fatalf("frame: %v", err)
	}
	lowered, err := Lower(mustParse(t, "select v from t where i>15"))
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}
	out, err := lowered.ExecRuntime(frame)
	if err != nil {
		t.Fatalf("ExecRuntime: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("real column i rows = %d, want 2 (column wins over row index)", out.Len())
	}
}

// --- 6. Kind-preserving integer aggregates and projections -------------------------

func TestQSQLIntegerKindPreservation(t *testing.T) {
	out := qCompletionRun(t, "select s:sum v by g from t")
	if kind, _ := out.Schema().Kind("s"); kind != data.KindI64 {
		t.Fatalf("grouped integer sum kind = %s, want i64", kind)
	}
	out = qCompletionRun(t, "select c:v+w from t")
	if kind, _ := out.Schema().Kind("c"); kind != data.KindI64 {
		t.Fatalf("computed integer projection kind = %s, want i64", kind)
	}
	out = qCompletionRun(t, "select a:avg v by g from t")
	if kind, _ := out.Schema().Kind("a"); kind != data.KindF64 {
		t.Fatalf("grouped integer avg kind = %s, want f64", kind)
	}
	out = qCompletionRun(t, "select s:sum px by g from t")
	if kind, _ := out.Schema().Kind("s"); kind != data.KindF64 {
		t.Fatalf("grouped float sum kind = %s, want f64", kind)
	}
}
