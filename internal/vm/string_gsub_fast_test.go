package vm

import "testing"

func TestStdStringGSubTwoCaptureConcatFastPath(t *testing.T) {
	globals := compileAndRun(t, `
func repl(num, tag) {
    return tag .. ":" .. num
}
out, count := string.gsub("item_123;tag45 item_7;tag08", "item_(%d+);(tag%d%d)", repl)
`)
	expectGlobalString(t, globals, "out", "tag45:123 tag08:7")
	expectGlobalInt(t, globals, "count", 2)
}

func TestStdStringGSubTwoCaptureConcatRejectsDifferentClosure(t *testing.T) {
	top := compileProto(t, `
func repl(num, tag) {
    return num .. ":" .. tag
}
`)
	child := findTestProtoByName(top, "repl")
	if child == nil {
		t.Fatal("missing repl proto")
	}
	spec, ok := concatTwoArgReplacementSpecForProto(child)
	if !ok {
		t.Fatal("expected concat replacement shape")
	}
	if spec.firstParam != 0 || spec.secondParam != 1 || spec.separator != ":" {
		t.Fatalf("unexpected replacement spec: %+v", spec)
	}
}
