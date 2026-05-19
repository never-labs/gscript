package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func TestRecursiveTableWholeCallRunsWithoutTier2Promotion(t *testing.T) {
	top := compileProto(t, `
func makeTree(depth) {
	if depth == 0 {
		return {left: nil, right: nil}
	}
	return {left: makeTree(depth - 1), right: makeTree(depth - 1)}
}

func checkTree(node) {
	if node.left == nil {
		return 1
	}
	return 1 + checkTree(node.left) + checkTree(node.right)
}

root := makeTree(4)
result := checkTree(root)
`)
	globals := runtime.NewInterpreterGlobals()
	v := New(globals)
	defer v.Close()
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("execute: %v", err)
	}
	root := v.GetGlobal("root")
	if !root.IsTable() {
		t.Fatalf("root=%v, want table", root)
	}
	if depth, key1, key2, ok := root.Table().LazyRecursiveTablePureInfo(); !ok || depth != 4 || key1 != "left" || key2 != "right" {
		t.Fatalf("root lazy info depth=%d key1=%q key2=%q ok=%v", depth, key1, key2, ok)
	}
	result := v.GetGlobal("result")
	if !result.IsInt() || result.Int() != 31 {
		t.Fatalf("result=%v, want int 31", result)
	}
}
