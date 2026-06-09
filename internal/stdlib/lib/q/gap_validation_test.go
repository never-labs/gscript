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
