package q

import (
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
		assertEvalValue(t, ".[{42};()]", int64(42))
	})

	t.Run("amend forms remain functional", func(t *testing.T) {
		assertEvalDict(t, "@[(`a`b!10 20);`b;:;25]", []data.Symbol{"a", "b"}, []any{int64(10), int64(25)})
		assertEvalDict(t, ".[(`a`b!10 20);`b;:;25]", []data.Symbol{"a", "b"}, []any{int64(10), int64(25)})
		assertEvalArray(t, "@[(10 20 30);1;+;5]", data.KindI64, []any{int64(10), int64(25), int64(30)})
	})
}
