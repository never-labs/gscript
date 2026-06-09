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
