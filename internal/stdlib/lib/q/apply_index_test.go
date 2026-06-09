package q

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestQApplyIndexCoreSemantics(t *testing.T) {
	t.Run("at index", func(t *testing.T) {
		assertEvalValue(t, "x:10 20 30;x@1", int64(20))
		assertEvalArray(t, "x:10 20 30;x@2 0", data.KindI64, []any{int64(30), int64(10)})
		assertEvalValue(t, `s:"abcd";s@2`, "c")
		assertEvalValue(t, "d:`a`b!10 20;d@`b", int64(20))
	})

	t.Run("dot index", func(t *testing.T) {
		assertEvalValue(t, "x:10 20 30;x . 1", int64(20))
		assertEvalValue(t, "x:(10 20;30 40);x . (1;0)", int64(30))
		assertEvalValue(t, "d:`a`b!(10;`c`d!20 30);d . (`b;`d)", int64(30))
		assertEvalArray(t, "t:flip `sym`px!(`AAPL`MSFT;100 200);t . `px", data.KindI64, []any{int64(100), int64(200)})
	})

	t.Run("callable apply", func(t *testing.T) {
		assertEvalValue(t, "sum@1 2 3", int64(6))
		assertEvalValue(t, "+ . (4;5)", int64(9))
		assertEvalValue(t, ".[+;(1;2)]", int64(3))
		assertEvalValue(t, ".[+;1 2]", int64(3))
		assertEvalValue(t, ".[{x+y};(2;3)]", int64(5))
		assertEvalValue(t, ".[{[a;b] a + b};(2;3)]", int64(5))
		assertEvalValue(t, ".[{42};()]", int64(42))
		assertEvalValue(t, "f:{(+/x)+y};.[f;(til 8;10)]", int64(38))
		assertEvalValue(t, "f:{(+/x)+count y};.[f;(til 8;10#1)]", int64(38))
		assertEvalValue(t, "f:{count y+(+/x)};.[f;(til 8;10#1)]", int64(38))
		assertEvalValue(t, "f:{[xs;n]sum xs+n};.[f;(til 8;10)]", int64(38))
		assertEvalValue(t, "f:{y+(+/x)};.[f;(til 8;10)]", int64(38))
		assertEvalValue(t, "a:10;f:{x+a};a:20;.[f;(1)]", int64(11))
	})

	t.Run("amend forms remain functional", func(t *testing.T) {
		assertEvalDict(t, "@[(`a`b!10 20);`b;:;25]", []data.Symbol{"a", "b"}, []any{int64(10), int64(25)})
		assertEvalDict(t, ".[(`a`b!10 20);`b;:;25]", []data.Symbol{"a", "b"}, []any{int64(10), int64(25)})
		assertEvalArray(t, "@[(10 20 30);1;+;5]", data.KindI64, []any{int64(10), int64(25), int64(30)})
	})
}

func TestQMatrixRowIndexRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "m:2 4#til 8;row:m@1;(+/row)+count row", int64(26))
	assertEvalValue(t, "m:2 4#til 8;row:m . 1;(+/row)+count row", int64(26))

	seenRowIndex := false
	seenRowSum := false
	seenRowSumCount := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected matrix row runtime fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "ArrayMatrixRowIndex" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && strings.HasPrefix(stat.Shape, "matrix-row/") {
			seenRowIndex = true
		}
		if stat.Kernel == "ArraySum" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && strings.HasPrefix(stat.Shape, "vector-reduce/sum/i64") {
			seenRowSum = true
		}
		if stat.Kernel == "MatrixRowSumCount" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && strings.HasPrefix(stat.Shape, "matrix-dot/") {
			seenRowSumCount = true
		}
	}
	if !seenRowSumCount && (!seenRowIndex || !seenRowSum) {
		t.Fatalf("missing matrix row typed runtime stats: rowIndex=%v rowSum=%v rowSumCount=%v stats=%#v", seenRowIndex, seenRowSum, seenRowSumCount, RuntimeKernelExecutionStats())
	}
}

func TestQDotApplyCallableSumPlusCountRecordsTypedRuntimeKernel(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	assertEvalValue(t, "f:{(+/x)+count y};.[f;(til 8;10#1)]", int64(38))

	seen := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "CallableDotSumPlusCount" && (stat.Outcome == "fallback" || stat.Outcome == "error") {
			t.Fatalf("unexpected callable dot fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "CallableDotSumPlusCount" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" {
			seen = true
			if stat.PipelineShape != "apply_index" {
				t.Fatalf("callable dot pipeline shape = %q, want apply_index; stat=%#v", stat.PipelineShape, stat)
			}
		}
	}
	if !seen {
		t.Fatalf("missing CallableDotSumPlusCount hit: %#v", RuntimeKernelExecutionStats())
	}
}

func TestQDotApplyPlanCachesCallableArgs(t *testing.T) {
	state := NewEvalState(nil)
	if got, err := state.Eval("f:{x+y};.[f;(100;23)]"); err != nil || got != int64(123) {
		t.Fatalf("first dot apply = %#v, %v; want 123,nil", got, err)
	}
	if got, err := state.Eval("f:{x+y};.[f;(100;23)]"); err != nil || got != int64(123) {
		t.Fatalf("warm dot apply = %#v, %v; want 123,nil", got, err)
	}
	if len(state.dotApplyCache) == 0 {
		t.Fatal("dot apply plan cache was not populated")
	}
	plan := state.dotApplyCache[".[f;(100;23)]"]
	if !plan.valid || len(plan.argExprs) != 2 {
		t.Fatalf("dot apply plan = %#v, want valid two-arg plan", plan)
	}
}

func TestQDotApplyArrayArgsRecordsTypedRuntimeKernel(t *testing.T) {
	state := NewEvalState(nil)
	if _, err := state.Eval("f:{x+y};g:{[a;b;c]a+b+c}"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	if got, err := state.Eval(".[f;100 23]"); err != nil || got != int64(123) {
		t.Fatalf(".[f;100 23] = %v, %v; want 123,nil", got, err)
	}
	if got, err := state.Eval("g . (1;2;3)"); err != nil || got != int64(6) {
		t.Fatalf("g . (1;2;3) = %v, %v; want 6,nil", got, err)
	}

	var hits uint64
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel != "ArrayCallableArgs" {
			continue
		}
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected callable array args fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Outcome == "hit" && stat.ReasonCode == "typed_apply_args" && strings.HasPrefix(stat.Shape, "apply-index/dot-args/") {
			hits += stat.Count
			if stat.PipelineShape != "apply_index" {
				t.Fatalf("callable array args pipeline shape = %q, want apply_index; stat=%#v", stat.PipelineShape, stat)
			}
		}
	}
	if hits != 2 {
		t.Fatalf("ArrayCallableArgs hits=%d; stats=%#v", hits, RuntimeKernelExecutionStats())
	}
}

func TestQMatrixCellLiteralDotIndexFastPathRecordsTypedRuntimeKernel(t *testing.T) {
	state := NewEvalState(nil)
	if _, err := state.Eval("m:2 4#til 8"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	if got, err := state.Eval("m . 1 2"); err != nil || got != int64(6) {
		t.Fatalf("m . 1 2 = %v, %v; want 6,nil", got, err)
	}
	if got, err := state.Eval("(m . 1 2)+count m"); err != nil || got != int64(8) {
		t.Fatalf("(m . 1 2)+count m = %v, %v; want 8,nil", got, err)
	}

	seenCell := false
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected matrix cell runtime fallback/error: %#v stats=%#v", stat, RuntimeKernelExecutionStats())
		}
		if stat.Kernel == "MatrixIndex" && stat.Outcome == "hit" && stat.ReasonCode == "typed_kernel" && strings.HasPrefix(stat.Shape, "matrix-dot/") {
			seenCell = true
		}
	}
	if !seenCell {
		t.Fatalf("missing matrix cell typed runtime stats: %#v", RuntimeKernelExecutionStats())
	}
}

func TestQApplyIndexScalarFastPathRecordsRuntimeStats(t *testing.T) {
	state := NewEvalState(nil)
	if _, err := state.Eval("x:til 256;s:\"abcd\";m:2 4#til 8"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	if got, err := state.Eval("x@100"); err != nil || got != int64(100) {
		t.Fatalf("x@100 = %v, %v; want 100, nil", got, err)
	}
	if got, err := state.Eval("x . 101"); err != nil || got != int64(101) {
		t.Fatalf("x . 101 = %v, %v; want 101, nil", got, err)
	}
	if got, err := state.Eval("s@2"); err != nil || got != "c" {
		t.Fatalf("s@2 = %v, %v; want c, nil", got, err)
	}
	if got, err := state.Eval("m@1"); err != nil {
		t.Fatalf("m@1 returned error: %v", err)
	} else if row, ok := got.(data.Array); !ok || row.Len() != 4 {
		t.Fatalf("m@1 = %#v, want row array len 4", got)
	}

	var scalarHits, matrixHits uint64
	for _, stat := range RuntimeKernelExecutionStats() {
		if (stat.Kernel == "ArrayScalarIndex" || stat.Kernel == "StringScalarIndex") && stat.Outcome == "hit" {
			scalarHits += stat.Count
			if stat.PipelineShape != "apply_index" {
				t.Fatalf("scalar index pipeline shape = %q, want apply_index; stat=%#v", stat.PipelineShape, stat)
			}
		}
		if stat.Kernel == "ArrayMatrixRowIndex" && stat.Outcome == "hit" {
			matrixHits += stat.Count
		}
		if stat.Outcome == "fallback" || stat.Outcome == "error" {
			t.Fatalf("unexpected scalar index fallback/error: %#v all=%#v", stat, RuntimeKernelExecutionStats())
		}
	}
	if scalarHits != 3 || matrixHits != 1 {
		t.Fatalf("scalar hits=%d matrix hits=%d; stats=%#v", scalarHits, matrixHits, RuntimeKernelExecutionStats())
	}
}
