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
		"hash", "headers", "html_escape", "http_headers", "httpmsg", "ini", "json", "jsonl",
		"kv", "lines", "mime", "numbers", "nums", "path", "prompt",
		"quote", "re", "regexp", "sh", "shellwords", "split", "template", "tsv", "url",
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

func TestDialectShellwordsParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("shellwords", "printf 'hello world' a\\ b \"\"")
		to_encode := {}
		to_encode[1] = "printf"
		to_encode[2] = "%s\n"
		to_encode[3] = "hello world"
		to_encode[4] = "it's"
		to_encode[5] = ""
		encoded := dialect.eval("shellwords", to_encode, {mode: "encode"})
		roundtrip := dialect.eval("shellwords", encoded)
		bad, bad_err := dialect.eval("shellwords", "'unterminated")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := stringSliceFromArray(interp.GetGlobal("parsed").Table())
	wantParsed := []string{"printf", "hello world", "a b", ""}
	if !reflect.DeepEqual(parsed, wantParsed) {
		t.Fatalf("parsed = %#v, want %#v", parsed, wantParsed)
	}
	wantEncoded := "printf '%s\n' 'hello world' 'it'\\''s' ''"
	if got := interp.GetGlobal("encoded").Str(); got != wantEncoded {
		t.Fatalf("encoded = %q, want %q", got, wantEncoded)
	}
	roundtrip := stringSliceFromArray(interp.GetGlobal("roundtrip").Table())
	wantRoundtrip := []string{"printf", "%s\n", "hello world", "it's", ""}
	if !reflect.DeepEqual(roundtrip, wantRoundtrip) {
		t.Fatalf("roundtrip = %#v, want %#v", roundtrip, wantRoundtrip)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad shellwords returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err"); !got.IsString() || !strings.Contains(got.Str(), "unterminated single quote") {
		t.Fatalf("bad_err = %v, want unterminated single quote", got)
	}
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

func TestDialectINIDecodeAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		cfg := ini`+"`"+`
app = ledger
enabled: true

[database]
host = db.internal
port = 5432
`+"`"+`
		encoded := dialect.eval("ini", {app: "ledger", enabled: true, database: {host: "db.internal", port: 5432}}, {mode: "encode"})
		roundtrip := dialect.eval("ini", encoded)
		bad, bad_err := dialect.eval("ini", "[broken")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	cfg := interp.GetGlobal("cfg").Table()
	if got := cfg.RawGetString("app").Str(); got != "ledger" {
		t.Fatalf("ini app = %q, want ledger", got)
	}
	if got := cfg.RawGetString("enabled").Str(); got != "true" {
		t.Fatalf("ini enabled = %q, want true", got)
	}
	db := cfg.RawGetString("database").Table()
	if got := db.RawGetString("host").Str(); got != "db.internal" {
		t.Fatalf("ini database.host = %q, want db.internal", got)
	}
	if got := db.RawGetString("port").Str(); got != "5432" {
		t.Fatalf("ini database.port = %q, want 5432", got)
	}
	wantEncoded := "app=ledger\nenabled=true\n\n[database]\nhost=db.internal\nport=5432\n"
	if got := interp.GetGlobal("encoded").Str(); got != wantEncoded {
		t.Fatalf("encoded ini = %q, want %q", got, wantEncoded)
	}
	roundtripDB := interp.GetGlobal("roundtrip").Table().RawGetString("database").Table()
	if got := roundtripDB.RawGetString("port").Str(); got != "5432" {
		t.Fatalf("roundtrip database.port = %q, want 5432", got)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad ini result is not nil")
	}
	if got := interp.GetGlobal("bad_err").Str(); !strings.Contains(got, "ini dialect: line 1: malformed section header") {
		t.Fatalf("bad ini error = %q", got)
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
