package bind

import (
	"fmt"
	"strings"
	"testing"
)

func TestDialectTagsExposeInstalledHandlers(t *testing.T) {
	interp := runWithLib(t, `
		tags := dialect.tags()
	`, "dialect", BuildDialect(HostOptions{}, nil))

	got := stringSetFromArray(interp.GetGlobal("tags").Table())
	want := []string{
		"base64", "cmd", "cookie", "cookies", "csv", "env", "glob",
		"hash", "headers", "html_escape", "http_headers", "json", "jsonl",
		"kv", "lines", "mime", "numbers", "nums", "path", "prompt",
		"quote", "re", "regexp", "split", "template", "tsv", "url",
		"urlquery", "words",
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("dialect.tags missing %q; got %#v", name, got)
		}
	}
	for _, reserved := range []string{"agent", "command", "data-science", "db", "fs", "network", "shell", "test", "text", "web", "workflow"} {
		if got[reserved] {
			t.Fatalf("dialect.tags unexpectedly exposes reserved category %q", reserved)
		}
	}
}

func TestDialectUnknownTagListsInstalledHandlers(t *testing.T) {
	eval := BuildDialect(HostOptions{}, nil).RawGetString("eval").GoFunction()
	if eval == nil {
		t.Fatalf("dialect.eval is not a Go function")
	}
	_, err := eval.Fn([]Value{StringValue("workflow"), StringValue("")})
	if err == nil {
		t.Fatalf("unknown dialect returned nil error")
	}
	msg := err.Error()
	for _, want := range []string{`unknown dialect "workflow"`, "available:", "cmd", "json", "prompt", "urlquery"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("unknown dialect error = %q, want containing %q", msg, want)
		}
	}
}

func TestDialectRegistryRejectsDuplicateNames(t *testing.T) {
	registry := newDialectRegistry()
	handler := dialectHandler{eval: func(Value, *Table) ([]Value, error) {
		return []Value{NilValue()}, nil
	}}
	registry.register([]string{"json"}, handler)
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("duplicate dialect registration did not panic")
		}
		if got := fmt.Sprint(recovered); !strings.Contains(got, `duplicate dialect "json"`) {
			t.Fatalf("panic = %q, want duplicate dialect json", got)
		}
	}()
	registry.register([]string{"json"}, handler)
}

func stringSetFromArray(tbl *Table) map[string]bool {
	out := make(map[string]bool)
	for i := 1; i <= tbl.Length(); i++ {
		out[tbl.RawGetInt(int64(i)).Str()] = true
	}
	return out
}
