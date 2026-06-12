package q

import (
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestQGapValidationConditionalsApplyIndexAndCasts(t *testing.T) {
	assertEvalValue(t, "$[0;1;2]", int64(2))
	assertEvalValue(t, "$[1;42;1%0]", int64(42))
	assertEvalValue(t, "?[1;42;1%0]", int64(42))
	assertEvalValue(t, "f:{$[x>0;1;-1]};f[-2]", int64(-1))
	assertEvalValue(t, "f:{[x]y:$[x>0;10;20];y+1};f[-2]", int64(21))
	assertEvalValue(t, "f:{[x]y:?[x>0;10;20];y+1};f[2]", int64(11))
	assertEvalValue(t, "a:$[1;2;1%0];b:?[0;1%0;3];a+b", int64(5))
	assertEvalValue(t, "i:0;while[i<3;i:i+1];i", int64(3))
	assertEvalValue(t, "i:0;while[i<3;i+:1];i", int64(3))

	assertEvalValue(t, "x:10 20 30;x@1", int64(20))
	assertEvalValue(t, "x:10 20 30;x . 1", int64(20))
	assertEvalValue(t, "x:(10 20;30 40);x . (0;1)", int64(20))
	assertEvalValue(t, ".[+;(1;2)]", int64(3))

	assertEvalValue(t, "`$\"hello\"", data.Symbol("hello"))
	assertEvalValue(t, "\"J\"$\"42\"", int64(42))
	assertEvalValue(t, "\"I\"$\"42\"", int32(42))
	assertEvalValue(t, "`long$\"42\"", int64(42))
	assertEvalValue(t, "`int$\"42\"", int32(42))
	// Canonical q integer casts round half-to-even.
	assertEvalValue(t, "`long$3.7", int64(4))
	assertEvalValue(t, "`int$-3.7", int32(-4))
	assertEvalArray(t, "`long$1.2 2.8 -3.7", data.KindI64, []any{int64(1), int64(3), int64(-4)})
	assertEvalArray(t, "`int$1.2 -2.8 3.0", data.KindI32, []any{int32(1), int32(-3), int32(3)})
}

func TestQGapValidationBooleanVectorLiterals(t *testing.T) {
	assertEvalValue(t, "1b", true)
	assertEvalArray(t, "101b", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "0 1b", data.KindBool, []any{false, true})
	assertEvalArray(t, "1 0 1b", data.KindBool, []any{true, false, true})
	assertEvalArray(t, "where 101b", data.KindI64, []any{int64(0), int64(2)})
	assertEvalArray(t, "m:1 0 1b;where m", data.KindI64, []any{int64(0), int64(2)})
	assertEvalArray(t, "{where x}[101b]", data.KindI64, []any{int64(0), int64(2)})
}

func TestQGapValidationReshapeMatrixAndFlip(t *testing.T) {
	assertEvalArray(t, "2 3#1 2 3 4 5 6", data.KindAny, []any{
		data.NewI64([]int64{1, 2, 3}),
		data.NewI64([]int64{4, 5, 6}),
	})
	assertEvalArray(t, "2 3#1 2 3 4", data.KindAny, []any{
		data.NewI64([]int64{1, 2, 3}),
		data.NewI64([]int64{4, 1, 2}),
	})
	assertEvalValue(t, "(2 3#1 2 3 4 5 6) . (1;2)", int64(6))
	assertEvalArray(t, "flip (1 2 3;4 5 6)", data.KindAny, []any{
		data.NewI64([]int64{1, 4}),
		data.NewI64([]int64{2, 5}),
		data.NewI64([]int64{3, 6}),
	})
	assertEvalArray(t, "flip ((1 2);(3 4);(5 6))", data.KindAny, []any{
		data.NewI64([]int64{1, 3, 5}),
		data.NewI64([]int64{2, 4, 6}),
	})
	tableValue, err := Eval("flip `sym`px!(`AAPL`MSFT;100 101)")
	if err != nil {
		t.Fatalf("Eval(dictionary flip) returned error: %v", err)
	}
	table, ok := tableValue.(data.Frame)
	if !ok {
		t.Fatalf("dictionary flip = %T, want data.Frame", tableValue)
	}
	if got := table.Len(); got != 2 {
		t.Fatalf("dictionary flip rows = %d, want 2", got)
	}
	assertEvalArray(t, "mmu[(2 2#1 2 3 4);(2 2#5 6 7 8)]", data.KindAny, []any{
		data.NewF64([]float64{19, 22}),
		data.NewF64([]float64{43, 50}),
	})
	assertEvalArray(t, "inv 2 2#1 0 0 1", data.KindAny, []any{
		data.NewF64([]float64{1, 0}),
		data.NewF64([]float64{0, 1}),
	})
}

func TestQGapValidationApplyIndexReshapeMatrixCompositions(t *testing.T) {
	assertEvalValue(t, "m:2 3#til 6;(+/(m@1))+(m . 0 2)", int64(14))
	assertEvalValue(t, "m:2 3#til 6;t:flip m;(+/(t . 2))+count t", int64(10))
	assertEvalValue(t, "d:`a`b!(10;2 2#1 2 3 4);d . (`b;1;0)", int64(3))
	assertEvalValue(t, ".[{(+/raze x)+x . 1 2};enlist 2 3#til 6]", int64(20))
	assertEvalArray(t, "flip 3 2#1 2 3", data.KindAny, []any{
		data.NewI64([]int64{1, 3, 2}),
		data.NewI64([]int64{2, 1, 3}),
	})
	assertEvalValue(t, "m:2 3#1 2 3 4;s:+/raze flip m;s+m . 1 1", int64(14))
	assertEvalValue(t, "t:2 3 4#til 24;t . (1;2;3)", int64(23))
	assertEvalArray(t, "t:2 3 4#til 24;raze t", data.KindAny, []any{
		data.NewI64([]int64{0, 1, 2, 3}),
		data.NewI64([]int64{4, 5, 6, 7}),
		data.NewI64([]int64{8, 9, 10, 11}),
		data.NewI64([]int64{12, 13, 14, 15}),
		data.NewI64([]int64{16, 17, 18, 19}),
		data.NewI64([]int64{20, 21, 22, 23}),
	})
	assertEvalArray(t, "t:2 3 4#til 24;raze (t@1)", data.KindI64, []any{
		int64(12), int64(13), int64(14), int64(15),
		int64(16), int64(17), int64(18), int64(19),
		int64(20), int64(21), int64(22), int64(23),
	})
	assertEvalArray(t, "t:2 3 4#til 24;flip (t@1)", data.KindAny, []any{
		data.NewI64([]int64{12, 16, 20}),
		data.NewI64([]int64{13, 17, 21}),
		data.NewI64([]int64{14, 18, 22}),
		data.NewI64([]int64{15, 19, 23}),
	})
	assertEvalValue(t, "t:2 3 4#til 24;(+/raze raze t)+t . (0;1;2)", int64(282))
}

func TestQGapValidationStringAndListVerbCompositions(t *testing.T) {
	assertEvalValue(t, `f:{trim x};f["  AAPL  "]`, "AAPL")
	assertEvalValue(t, "sym:`AAPL;\",\" sv (string sym;\"MSFT\";upper trim \" nvda \")", "AAPL,MSFT,NVDA")
	assertEvalArray(t, `"," vs ("," sv ("AAPL";"MSFT";"NVDA"))`, data.KindString, []any{"AAPL", "MSFT", "NVDA"})
	// DIALECT GROUPING: a registered dyadic word claims the leftmost split
	// before symbol comparisons on every route (`1<"banana" ss "an"` groups
	// as (1<"banana") ss "an" and signals). The canonical 1<(...) grouping
	// keeps the parenthesized spelling. This keeps `where <expr>` grouping
	// identical to the bare <expr> statement (one consistent grammar).
	assertEvalArray(t, `where 1<("banana" ss "an")`, data.KindI64, []any{int64(1)})
	assertEvalValue(t, `s:" a-b-c ";trim s ssr ("-";":")`, "a:b:c")
	assertEvalValue(t, `ssr["," sv ("AAPL";"MSFT");",";"|"]`, "AAPL|MSFT")

	assertEvalArray(t, "raze cut[0 2;10 20 30 40]", data.KindI64, []any{int64(10), int64(20), int64(30), int64(40)})
	assertEvalArray(t, "raze 0 2 cut 10 20 30 40", data.KindI64, []any{int64(10), int64(20), int64(30), int64(40)})
	assertEvalArray(t, "reverse sublist[1 3;10 20 30 40 50]", data.KindI64, []any{int64(40), int64(30), int64(20)})
	assertEvalValue(t, "count (1 2 cross `a`b)[where 1010b]", int64(2))
}

func TestQGapValidationDyadicWordChainsUseQRightAssociativity(t *testing.T) {
	assertEvalValue(t, `count "," vs "," sv "AAPL" "MSFT" "NVDA"`, int64(3))
	assertEvalArray(t, `"," vs "," sv "AAPL" "MSFT" "NVDA"`, data.KindString, []any{"AAPL", "MSFT", "NVDA"})
	assertEvalValue(t, `";" sv "," vs "AAPL,MSFT,NVDA"`, "AAPL;MSFT;NVDA")
	assertEvalValue(t, `"a-b-c" ssr "," vs "-,+"`, "a+b+c")
	assertEvalValue(t, `"banana" ssr "," vs "na,NA"`, "baNANA")

	assertEvalArray(t, "1 2 3 union 3 4 except 2 4", data.KindI64, []any{int64(1), int64(2), int64(3)})
	assertEvalArray(t, "1 2 3 inter 2 3 4 union 4 5", data.KindI64, []any{int64(2), int64(3)})
	assertEvalValue(t, "10 plus 2 times 3", int64(16))
	assertEvalValue(t, "100 minus 20 div 5", int64(96))
}

func TestQGapValidationDyadicWhereFiltersLikeIndexWhere(t *testing.T) {
	assertEvalArray(t, "10 20 30 40 where 1010b", data.KindI64, []any{int64(10), int64(30)})
	assertEvalArray(t, "10 20 30 40 where 0 2 1 0", data.KindI64, []any{int64(20), int64(20), int64(30)})
	assertEvalArray(t, `"abcd" where 1010b`, data.KindString, []any{"a", "c"})
	assertEvalValue(t, "sum (10 20 30 40 where 10 20 30 40>20)", int64(70))
	assertEvalValue(t, "x:til 16;m:(x mod 3)=0;(+/x where m)+count x where m", int64(51))
	assertEvalArray(t, "(1 2;3 4;5 6) where 101b", data.KindAny, []any{
		data.NewI64([]int64{1, 2}),
		data.NewI64([]int64{5, 6}),
	})
}
