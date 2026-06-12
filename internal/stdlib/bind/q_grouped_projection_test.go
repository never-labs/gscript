package bind

// Regression coverage for grouped NON-aggregate projections (window verbs
// evaluated per group, scattered back to source-row positions). The
// all-rows shape regressed in ab1fe69c: the query kernel started passing
// nil indexes ("all rows") for unfiltered grouped queries, and
// execGroupedProjection treated nil as zero rows, silently returning an
// empty frame.

import "testing"

func TestQSQLGroupedNonAggregateProjectionAllRows(t *testing.T) {
	interp := runWithQAndSOA(t,
		"trades := q.eval(\"([] a:1 2 3 4; s:`x`x`y`y; p:10.0 11.0 20.0 22.0)\")\n"+
			"grouped := q.sql(\"select a,d:deltas p by s from trades\", {trades: trades})\n")

	grouped := interp.GetGlobal("grouped").Table()
	if grouped == nil || grouped.Length() != 4 {
		t.Fatalf("grouped length = %v, want 4", grouped)
	}
	// deltas restarts per group: group x rows (a=1,2) and group y rows (a=3,4).
	wantA := []int64{1, 2, 3, 4}
	wantD := []float64{10, 1, 20, 2}
	for i := 0; i < 4; i++ {
		row := grouped.RawGetInt(int64(i) + 1).Table()
		if got := row.RawGetString("a"); !got.IsInt() || got.Int() != wantA[i] {
			t.Fatalf("grouped[%d].a = %v, want %d", i+1, got, wantA[i])
		}
		if got := row.RawGetString("d"); !got.IsFloat() || got.Float() != wantD[i] {
			t.Fatalf("grouped[%d].d = %v, want %v", i+1, got, wantD[i])
		}
	}
}

func TestQSQLGroupedNonAggregateProjectionWithWhere(t *testing.T) {
	interp := runWithQAndSOA(t,
		"trades := q.eval(\"([] a:1 2 3 4; s:`x`x`y`y; p:10.0 11.0 20.0 22.0)\")\n"+
			"grouped := q.sql(\"select a,d:deltas p by s from trades where a>1\", {trades: trades})\n")

	grouped := interp.GetGlobal("grouped").Table()
	if grouped == nil || grouped.Length() != 3 {
		t.Fatalf("grouped length = %v, want 3", grouped)
	}
	// Filter drops a=1; group x = {11.0}, group y = {20.0, 22.0}, so deltas
	// seeds each group with its first surviving value.
	wantA := []int64{2, 3, 4}
	wantD := []float64{11, 20, 2}
	for i := 0; i < 3; i++ {
		row := grouped.RawGetInt(int64(i) + 1).Table()
		if got := row.RawGetString("a"); !got.IsInt() || got.Int() != wantA[i] {
			t.Fatalf("grouped[%d].a = %v, want %d", i+1, got, wantA[i])
		}
		if got := row.RawGetString("d"); !got.IsFloat() || got.Float() != wantD[i] {
			t.Fatalf("grouped[%d].d = %v, want %v", i+1, got, wantD[i])
		}
	}
}

func TestQSQLGroupedNonAggregateProjectionMavgAndXprev(t *testing.T) {
	interp := runWithQAndSOA(t,
		"trades := q.eval(\"([] a:1 2 3 4; s:`x`x`y`y; p:10.0 11.0 20.0 22.0)\")\n"+
			"grouped := q.sql(\"select a,m:2 mavg p,x:1 xprev p by s from trades\", {trades: trades})\n")

	grouped := interp.GetGlobal("grouped").Table()
	if grouped == nil || grouped.Length() != 4 {
		t.Fatalf("grouped length = %v, want 4", grouped)
	}
	wantM := []float64{10, 10.5, 20, 21}
	for i := 0; i < 4; i++ {
		row := grouped.RawGetInt(int64(i) + 1).Table()
		if got := row.RawGetString("m"); !got.IsFloat() || got.Float() != wantM[i] {
			t.Fatalf("grouped[%d].m = %v, want %v", i+1, got, wantM[i])
		}
	}
	// xprev resets per group: first row of each group has no predecessor.
	if got := grouped.RawGetInt(1).Table().RawGetString("x"); !got.IsNil() {
		t.Fatalf("grouped[1].x = %v, want nil", got)
	}
	if got := grouped.RawGetInt(2).Table().RawGetString("x"); !got.IsFloat() || got.Float() != 10 {
		t.Fatalf("grouped[2].x = %v, want 10", got)
	}
	if got := grouped.RawGetInt(3).Table().RawGetString("x"); !got.IsNil() {
		t.Fatalf("grouped[3].x = %v, want nil (group restart)", got)
	}
	if got := grouped.RawGetInt(4).Table().RawGetString("x"); !got.IsFloat() || got.Float() != 20 {
		t.Fatalf("grouped[4].x = %v, want 20", got)
	}
}

// Companion regression from the same commit's lazy row views: delete-where
// leaves columns as shared indexedArray views, and the numeric unary kernel
// (neg/sqrt/log/...) rejected that carrier with "expects numeric values".
func TestQSQLNumericUnaryVerbsOverRowDeleteView(t *testing.T) {
	interp := runWithQAndSOA(t,
		"trades := q.eval(\"([] trade_id:101 102 103 104; size:100 50 40 5; cond:`regular`regular`regular`oddlot)\")\n"+
			"kept := q.sql(trades, \"delete from trades where cond=`oddlot\")\n"+
			"math := q.sql(kept, \"select trade_id,n:neg size,r:sqrt size from trades\")\n")

	math := interp.GetGlobal("math").Table()
	if math == nil || math.Length() != 3 {
		t.Fatalf("math length = %v, want 3", math)
	}
	wantN := []float64{-100, -50, -40}
	wantR := []float64{10, 7.0710678118654755, 6.324555320336759}
	for i := 0; i < 3; i++ {
		row := math.RawGetInt(int64(i) + 1).Table()
		if got := row.RawGetString("n"); !got.IsFloat() || got.Float() != wantN[i] {
			t.Fatalf("math[%d].n = %v, want %v", i+1, got, wantN[i])
		}
		if got := row.RawGetString("r"); !got.IsFloat() || got.Float() != wantR[i] {
			t.Fatalf("math[%d].r = %v, want %v", i+1, got, wantR[i])
		}
	}
}

func TestQSQLGroupedNonAggregateProjectionFillsNulls(t *testing.T) {
	interp := runWithQAndSOA(t,
		"trades := q.eval(\"([] a:1 2 3 4; s:`x`x`y`y; p:10.0 0n 20.0 22.0)\")\n"+
			"grouped := q.sql(\"select a,f:fills p by s from trades\", {trades: trades})\n"+
			"filtered := q.sql(\"select a,f:fills p by s from trades where a>=1\", {trades: trades})\n")

	for _, name := range []string{"grouped", "filtered"} {
		out := interp.GetGlobal(name).Table()
		if out == nil || out.Length() != 4 {
			t.Fatalf("%s length = %v, want 4", name, out)
		}
		wantF := []float64{10, 10, 20, 22}
		for i := 0; i < 4; i++ {
			row := out.RawGetInt(int64(i) + 1).Table()
			if got := row.RawGetString("f"); !got.IsFloat() || got.Float() != wantF[i] {
				t.Fatalf("%s[%d].f = %v, want %v", name, i+1, got, wantF[i])
			}
		}
	}
}
