package vm

import (
	"testing"

	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
)

func runGCRootRegression(t *testing.T, src string) map[string]runtime.Value {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	proto, err := Compile(prog)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	v := New(vmtest.NewInterpreterGlobals())
	defer v.Close()
	if _, err := v.Execute(proto); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return v.Globals()
}

func TestCollectGarbageKeepsTableArgumentsAlive(t *testing.T) {
	globals := runGCRootRegression(t, `
func read_after_collect(t) {
    collectgarbage("collect")
    return t.value
}

result := 0
for i := 1; i <= 3000; i++ {
    result = read_after_collect({value: i})
}
`)
	result := globals["result"]
	if !result.IsInt() || result.Int() != 3000 {
		t.Fatalf("result = %v (%s), want int 3000", result, result.TypeName())
	}
}

func TestCollectGarbageKeepsLazyStringArgumentsAlive(t *testing.T) {
	globals := runGCRootRegression(t, `
path := require("path")

result := ""
for i := 1; i <= 3000; i++ {
    date := "1596-04-" .. tostring(i)
    suffix := date .. ".jsonl"
    collectgarbage("collect")
    result = path.join("persistence", "eventlog", suffix)
}
`)
	result := globals["result"]
	if !result.IsString() || result.Str() != "persistence/eventlog/1596-04-3000.jsonl" {
		t.Fatalf("result = %v (%s), want final path", result, result.TypeName())
	}
}
