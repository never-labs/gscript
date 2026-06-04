package bind

import (
	"fmt"
	"reflect"
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
		"quote", "re", "regexp", "sh", "split", "template", "tsv", "url",
		"urlpath", "urlquery", "words",
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
	if gotList := stringSliceFromArray(interp.GetGlobal("tags").Table()); !reflect.DeepEqual(gotList, want) {
		t.Fatalf("dialect.tags = %#v, want %#v", gotList, want)
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

func TestDialectRegistryRejectsDuplicateNamesInSingleRegistration(t *testing.T) {
	registry := newDialectRegistry()
	handler := dialectHandler{eval: func(Value, *Table) ([]Value, error) {
		return []Value{NilValue()}, nil
	}}
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("duplicate dialect alias in same registration did not panic")
		}
		if got := fmt.Sprint(recovered); !strings.Contains(got, `duplicate dialect "json"`) {
			t.Fatalf("panic = %q, want duplicate dialect json", got)
		}
	}()
	registry.register([]string{"json", "json"}, handler)
}

func TestDialectBlockAndRawEvalHelpers(t *testing.T) {
	interp := runWithLib(t, `
		encoded := dialect.eval_block("json", {x: 1, label: "ok"})
		prompt_msg := dialect.eval_block("prompt", {role: "system", text: "hi"})
		raw := dialect.eval_raw("triage-plan", {step: "collect"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("encoded").Str(); got != `{"label":"ok","x":1}` {
		t.Fatalf("eval_block json = %q, want encoded object", got)
	}
	prompt := interp.GetGlobal("prompt_msg").Table()
	body := prompt.RawGetString("body").Table()
	if got := body.RawGetString("role").Str(); got != "system" {
		t.Fatalf("prompt body role = %q, want system", got)
	}
	if got := body.RawGetString("text").Str(); got != "hi" {
		t.Fatalf("prompt body text = %q, want hi", got)
	}
	raw := interp.GetGlobal("raw").Table()
	if got := raw.RawGetString("dialect").Str(); got != "triage-plan" {
		t.Fatalf("raw dialect = %q, want triage-plan", got)
	}
	if got := raw.RawGetString("kind").Str(); got != "table" {
		t.Fatalf("raw kind = %q, want table", got)
	}
	if got := raw.RawGetString("body").Table().RawGetString("step").Str(); got != "collect" {
		t.Fatalf("raw body step = %q, want collect", got)
	}
}

func stringSetFromArray(tbl *Table) map[string]bool {
	out := make(map[string]bool)
	for i := 1; i <= tbl.Length(); i++ {
		out[tbl.RawGetInt(int64(i)).Str()] = true
	}
	return out
}

func stringSliceFromArray(tbl *Table) []string {
	out := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		out = append(out, tbl.RawGetInt(int64(i)).Str())
	}
	return out
}
