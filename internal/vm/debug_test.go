package vm

import (
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
	"github.com/never-labs/gscript/internal/runtime"
)

func TestVMDebugStackInfoSourceAndEvents(t *testing.T) {
	src := `
events := {}
sawCall := false
sawReturn := false
sawEmit := false
sawError := false
sawSink := false

func hookFn(e) {
  table.insert(events, e.type .. ":" .. e.kind .. ":" .. e.name)
  if e.type == "call" {
    sawCall = true
  }
  if e.type == "return" {
    sawReturn = true
  }
  if e.type == "emit" {
    sawEmit = true
  }
  if e.type == "error" {
    sawError = true
  }
}

func sinkFn(e) {
  table.insert(events, "sink:" .. e.event)
  sawSink = true
}

func leaf() {
  stack := debug.stack()
  i0 := debug.info(0)
  i1 := debug.info(1)
  debug.emit("mark", {ok: true})
  return stack[#stack].name, i0.name, i1.name, i0.sourceName, i0.line, i0.column
}

debug.setHook(hookFn)
debug.setSink(sinkFn)

func parent() {
  return leaf()
}

func fail() {
  error("debug-fail")
}

topName, info0Name, info1Name, sourceName, lineNumber, colNumber := parent()
pcall(fail)
debug.setHook(nil)
debug.setSink(nil)

eventCount := #events
`
	globals := compileAndRunWithSource(t, src, "vm_debug_fixture.gs")
	expectStringGlobal(t, globals, "topName", "leaf")
	expectStringGlobal(t, globals, "info0Name", "leaf")
	expectStringGlobal(t, globals, "info1Name", "parent")
	expectStringGlobal(t, globals, "sourceName", "vm_debug_fixture.gs")
	if got := globals["lineNumber"].Int(); got <= 0 {
		t.Fatalf("lineNumber = %d, want positive source line", got)
	}
	if got := globals["colNumber"].Int(); got != 1 {
		t.Fatalf("colNumber = %d, want 1", got)
	}
	if got := globals["eventCount"].Int(); got <= 0 {
		t.Fatalf("eventCount = %d, want hook/sink events", got)
	}
	for _, name := range []string{"sawCall", "sawReturn", "sawEmit", "sawError", "sawSink"} {
		if !globals[name].Bool() {
			t.Fatalf("%s = false, want observed debug event", name)
		}
	}
}

func TestVMDebugSinkReceivesGoroutineErrors(t *testing.T) {
	src := `
events := {}
done := make(chan, 1)

func sinkFn(e) {
  if e.type == "error" && e.kind == "goroutine" {
    table.insert(events, e.name .. ":" .. e.error)
    done <- e.stack
  }
}

debug.setSink(sinkFn)
go func() {
  error("go-fail")
}()
stack := <-done
debug.setSink(nil)

eventCount := #events
firstEvent := events[1]
`
	globals := compileAndRunWithSource(t, src, "vm_go_error_fixture.gs")
	if got := globals["eventCount"].Int(); got != 1 {
		t.Fatalf("eventCount = %d, want 1 goroutine error event", got)
	}
	expectStringGlobal(t, globals, "firstEvent", "go-fail")
	expectStringGlobal(t, globals, "stack", "vm_go_error_fixture.gs")
}

func compileAndRunWithSource(t *testing.T, src, sourceName string) map[string]runtime.Value {
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
	setProtoSourceForTest(proto, sourceName)
	globals := runtime.NewInterpreterGlobals()
	vm := New(globals)
	_, err = vm.Execute(proto)
	if err != nil {
		t.Fatalf("runtime error: %v", err)
	}
	return globals
}

func setProtoSourceForTest(proto *FuncProto, sourceName string) {
	if proto == nil {
		return
	}
	proto.Source = sourceName
	for _, child := range proto.Protos {
		setProtoSourceForTest(child, sourceName)
	}
}

func expectStringGlobal(t *testing.T, globals map[string]runtime.Value, name, want string) {
	t.Helper()
	got := globals[name].String()
	if !strings.Contains(got, want) {
		t.Fatalf("%s = %q, want contains %q", name, got, want)
	}
}
