package bind

import (
	"strings"
	"testing"
)

func TestDialectDelimitedParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		csv_text := dialect.eval("csv", [["name", "score"], ["Ada", 42], ["Bob", 7]], {mode: "encode"})
		csv_roundtrip := dialect.eval("csv", csv_text)
		csv_header_text := dialect.eval("csv", [{name: "Ada", score: 42}, {name: "Bob", score: 7}], {mode: "encode", headers: ["name", "score"]})
		csv_header_roundtrip := dialect.eval("csv", csv_header_text, {headers: true})
		tsv_text := dialect.eval("tsv", [["name", "score"], ["Ada", 42]], {mode: "encode"})
		tsv_roundtrip := dialect.eval("tsv", tsv_text)
		bad, bad_err := dialect.eval("csv", ["not a row"], {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got, want := interp.GetGlobal("csv_text").Str(), "name,score\nAda,42\nBob,7\n"; got != want {
		t.Fatalf("csv_text = %q, want %q", got, want)
	}
	csvRoundtrip := interp.GetGlobal("csv_roundtrip").Table()
	if got := csvRoundtrip.RawGetInt(2).Table().RawGetInt(1).Str(); got != "Ada" {
		t.Fatalf("csv roundtrip row = %q, want Ada", got)
	}
	if got, want := interp.GetGlobal("csv_header_text").Str(), "name,score\nAda,42\nBob,7\n"; got != want {
		t.Fatalf("csv_header_text = %q, want %q", got, want)
	}
	csvHeaderRoundtrip := interp.GetGlobal("csv_header_roundtrip").Table()
	if got := csvHeaderRoundtrip.RawGetInt(2).Table().RawGetString("score").Str(); got != "7" {
		t.Fatalf("csv header roundtrip score = %q, want 7", got)
	}
	if got, want := interp.GetGlobal("tsv_text").Str(), "name\tscore\nAda\t42\n"; got != want {
		t.Fatalf("tsv_text = %q, want %q", got, want)
	}
	tsvRoundtrip := interp.GetGlobal("tsv_roundtrip").Table()
	if got := tsvRoundtrip.RawGetInt(2).Table().RawGetInt(2).Str(); got != "42" {
		t.Fatalf("tsv roundtrip score = %q, want 42", got)
	}
	if !interp.GetGlobal("bad").IsNil() || !interp.GetGlobal("bad_err").IsString() {
		t.Fatalf("bad csv encode = %v err %v, want nil error string", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
}

func TestDialectKVEnvParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		kv_text := dialect.eval("kv", {score: 42, name: "Ada"}, {mode: "encode"})
		kv_roundtrip := dialect.eval("kv", kv_text)
		kv_colon := dialect.eval("kv", {score: 42, name: "Ada"}, {mode: "encode", sep: ":"})
		kv_colon_roundtrip := dialect.eval("kv", kv_colon, {sep: ":"})
		env_text := dialect.eval("env", {TOKEN: "abc123", EMPTY: ""}, {mode: "encode"})
		env_roundtrip := dialect.eval("env", env_text)
		bad_input := {}
		bad_input["bad=key"] = "1"
		bad, bad_err := dialect.eval("kv", bad_input, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got, want := interp.GetGlobal("kv_text").Str(), "name=Ada\nscore=42\n"; got != want {
		t.Fatalf("kv_text = %q, want %q", got, want)
	}
	kvRoundtrip := interp.GetGlobal("kv_roundtrip").Table()
	if got := kvRoundtrip.RawGetString("score").Str(); got != "42" {
		t.Fatalf("kv roundtrip score = %q, want 42", got)
	}
	if got, want := interp.GetGlobal("kv_colon").Str(), "name:Ada\nscore:42\n"; got != want {
		t.Fatalf("kv_colon = %q, want %q", got, want)
	}
	kvColonRoundtrip := interp.GetGlobal("kv_colon_roundtrip").Table()
	if got := kvColonRoundtrip.RawGetString("name").Str(); got != "Ada" {
		t.Fatalf("kv colon roundtrip name = %q, want Ada", got)
	}
	if got, want := interp.GetGlobal("env_text").Str(), "EMPTY=\nTOKEN=abc123\n"; got != want {
		t.Fatalf("env_text = %q, want %q", got, want)
	}
	envRoundtrip := interp.GetGlobal("env_roundtrip").Table()
	if got := envRoundtrip.RawGetString("EMPTY").Str(); got != "" {
		t.Fatalf("env roundtrip empty = %q, want empty string", got)
	}
	if !interp.GetGlobal("bad").IsNil() || !interp.GetGlobal("bad_err").IsString() {
		t.Fatalf("bad kv encode = %v err %v, want nil error string", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
}

func TestDialectEnvLookupUsesHostEnvironmentPolicy(t *testing.T) {
	t.Setenv("LEIA_DIALECT_ENV_ALLOWED", "visible")
	t.Setenv("LEIA_DIALECT_ENV_BLOCKED", "secret")
	interp := runWithLib(t, `
		allowed := env`+"`"+`LEIA_DIALECT_ENV_ALLOWED`+"`"+`
		missing := env`+"`"+`LEIA_DIALECT_ENV_MISSING`+"`"+`
		blocked := env`+"`"+`LEIA_DIALECT_ENV_BLOCKED`+"`"+`
		parsed := env`+"`"+`LEIA_DIALECT_ENV_ALLOWED=parsed`+"`"+`
		lookup_mode := dialect.eval("env", "LEIA_DIALECT_ENV_ALLOWED", {mode: "lookup"})
		missing_fast_ok, missing_fast_err := pcall(func() {
			return env!`+"`"+`LEIA_DIALECT_ENV_MISSING`+"`"+`
		})
	`, "dialect", BuildDialect(HostOptions{
		EnvironmentRead: func() bool { return true },
		EnvironmentAllowed: func(name string) bool {
			return name == "LEIA_DIALECT_ENV_ALLOWED" || name == "LEIA_DIALECT_ENV_MISSING"
		},
	}, nil))

	if got := interp.GetGlobal("allowed").Str(); got != "visible" {
		t.Fatalf("allowed env = %q, want visible", got)
	}
	if !interp.GetGlobal("missing").IsNil() {
		t.Fatalf("missing env = %v, want nil", interp.GetGlobal("missing"))
	}
	if !interp.GetGlobal("blocked").IsNil() {
		t.Fatalf("blocked env = %v, want nil", interp.GetGlobal("blocked"))
	}
	if got := interp.GetGlobal("parsed").Table().RawGetString("LEIA_DIALECT_ENV_ALLOWED").Str(); got != "parsed" {
		t.Fatalf("env parser compatibility = %q, want parsed", got)
	}
	if got := interp.GetGlobal("lookup_mode").Str(); got != "visible" {
		t.Fatalf("lookup mode env = %q, want visible", got)
	}
	if interp.GetGlobal("missing_fast_ok").Bool() {
		t.Fatalf("env! missing succeeded, want pcall false")
	}
	if got := interp.GetGlobal("missing_fast_err").Str(); !strings.Contains(got, "environment variable not set") {
		t.Fatalf("env! missing err = %q, want not set", got)
	}

	eval := BuildDialect(HostOptions{EnvironmentRead: func() bool { return false }}, nil).RawGetString("eval").GoFunction()
	_, err := eval.Fn([]Value{StringValue("env"), StringValue("LEIA_DIALECT_ENV_ALLOWED")})
	if err == nil || !strings.Contains(err.Error(), "environment read access disabled") {
		t.Fatalf("environment read disabled err = %v, want disabled", err)
	}

}

func TestDialectMarkdownSummary(t *testing.T) {
	interp := runWithLib(t, ""+
		"doc := markdown`# Release Notes\n\n"+
		"Leia ships [examples](https://example.test/examples).\n\n"+
		"## Changes\n\n"+
		"- dialects\n"+
		"- tooling\n\n"+
		"~~~leia\n"+
		"print(\"ok\")\n"+
		"~~~\n"+
		"`\n"+
		"plain := dialect.eval(\"md\", doc.plain_text, {mode: \"plain\"})\n",
		"dialect", BuildDialect(HostOptions{}, nil))

	doc := interp.GetGlobal("doc").Table()
	if got := doc.RawGetString("title").Str(); got != "Release Notes" {
		t.Fatalf("markdown title = %q, want Release Notes", got)
	}
	if got := doc.RawGetString("headings").Table().Length(); got != 2 {
		t.Fatalf("markdown headings = %d, want 2", got)
	}
	if got := doc.RawGetString("links").Table().RawGetInt(1).Table().RawGetString("url").Str(); got != "https://example.test/examples" {
		t.Fatalf("markdown link url = %q", got)
	}
	if got := doc.RawGetString("list_items").Int(); got != 2 {
		t.Fatalf("markdown list_items = %d, want 2", got)
	}
	if got := doc.RawGetString("code_blocks").Table().RawGetInt(1).Table().RawGetString("info").Str(); got != "leia" {
		t.Fatalf("markdown code info = %q, want leia", got)
	}
	if got := interp.GetGlobal("plain").Str(); !strings.Contains(got, "Release Notes") || strings.Contains(got, "```") {
		t.Fatalf("markdown plain text = %q", got)
	}
}

func TestDialectYAMLLiteParseAndEncode(t *testing.T) {
	interp := runWithLib(t, ""+
		"cfg := yaml`service: api\n"+
		"enabled: true\n"+
		"retries: 3\n"+
		"threshold: 2.5\n"+
		"owner:\n"+
		"  name: Ada\n"+
		"  team: platform\n"+
		"targets:\n"+
		"  - web\n"+
		"  - worker\n"+
		"checks:\n"+
		"  - name: unit\n"+
		"    required: true\n"+
		"  - name: smoke\n"+
		"    required: false\n"+
		"`\n"+
		"encoded := dialect.eval(\"yml\", {service: \"api\", enabled: true, retries: 3, targets: {\"web\", \"worker\"}}, {mode: \"encode\"})\n"+
		"roundtrip := dialect.eval(\"yaml\", encoded)\n"+
		"bad, bad_err := yaml`service api`\n",
		"dialect", BuildDialect(HostOptions{}, nil))

	cfg := interp.GetGlobal("cfg").Table()
	if got := cfg.RawGetString("service").Str(); got != "api" {
		t.Fatalf("yaml service = %q, want api", got)
	}
	if !cfg.RawGetString("enabled").Bool() {
		t.Fatalf("yaml enabled = false, want true")
	}
	if got := cfg.RawGetString("retries").Int(); got != 3 {
		t.Fatalf("yaml retries = %d, want 3", got)
	}
	if got := cfg.RawGetString("threshold").Float(); got != 2.5 {
		t.Fatalf("yaml threshold = %v, want 2.5", got)
	}
	if got := cfg.RawGetString("owner").Table().RawGetString("team").Str(); got != "platform" {
		t.Fatalf("yaml owner.team = %q", got)
	}
	if got := cfg.RawGetString("targets").Table().RawGetInt(2).Str(); got != "worker" {
		t.Fatalf("yaml target 2 = %q", got)
	}
	checks := cfg.RawGetString("checks").Table()
	if got := checks.RawGetInt(2).Table().RawGetString("name").Str(); got != "smoke" {
		t.Fatalf("yaml check 2 name = %q", got)
	}
	if checks.RawGetInt(2).Table().RawGetString("required").Bool() {
		t.Fatalf("yaml check 2 required = true, want false")
	}
	if got := interp.GetGlobal("roundtrip").Table().RawGetString("targets").Table().RawGetInt(1).Str(); got != "web" {
		t.Fatalf("yaml roundtrip target = %q", got)
	}
	if !interp.GetGlobal("bad").IsNil() || !strings.Contains(interp.GetGlobal("bad_err").Str(), "yaml dialect: line 1: missing ':'") {
		t.Fatalf("bad yaml = %v err %v", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
}

func TestDialectTimestampParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := timestamp`+"`"+`2026-06-05T12:34:56+08:00`+"`"+`
		nano := rfc3339`+"`"+`2026-06-05T04:34:56.123456789Z`+"`"+`
		unix_value := dialect.eval("timestamp", "1780648496", {mode: "unix"})
		encoded := dialect.eval("timestamp", {unix: parsed.unix}, {mode: "format"})
		encoded_nano := dialect.eval("timestamp", {unix: nano.unix, nsec: 123456789}, {mode: "format", nano: true})
		bad, bad_err := dialect.eval("timestamp", "not a timestamp")
		bad_mode, bad_mode_err := dialect.eval("timestamp", "2026-06-05T00:00:00Z", {mode: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("rfc3339").Str(); got != "2026-06-05T04:34:56Z" {
		t.Fatalf("timestamp rfc3339 = %q", got)
	}
	if got := parsed.RawGetString("date").Str(); got != "2026-06-05" {
		t.Fatalf("timestamp date = %q", got)
	}
	if got := parsed.RawGetString("time").Str(); got != "04:34:56" {
		t.Fatalf("timestamp time = %q", got)
	}
	if got := interp.GetGlobal("unix_value").Int(); got != 1780648496 {
		t.Fatalf("timestamp unix mode = %d", got)
	}
	if got := interp.GetGlobal("encoded").Str(); got != "2026-06-05T04:34:56Z" {
		t.Fatalf("timestamp encoded = %q", got)
	}
	if got := interp.GetGlobal("encoded_nano").Str(); got != "2026-06-05T04:34:56.123456789Z" {
		t.Fatalf("timestamp encoded_nano = %q", got)
	}
	if !interp.GetGlobal("bad").IsNil() || !strings.Contains(interp.GetGlobal("bad_err").Str(), "invalid RFC3339") {
		t.Fatalf("bad timestamp = %v err %v", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
	assertDialectModeError(t, interp.GetGlobal("bad_mode"), interp.GetGlobal("bad_mode_err"), "timestamp dialect: unknown mode")
}

func TestDialectTextInvalidInputsReturnErrors(t *testing.T) {
	interp := runWithLib(t, `
		bad_csv, bad_csv_err := dialect.eval("csv", "\"unterminated\n")
		bad_tsv, bad_tsv_err := dialect.eval("tsv", "a\t\"unterminated\n")
		bad_json, bad_json_err := dialect.eval("json", "{\"x\":")
		bad_json_trailing, bad_json_trailing_err := dialect.eval("json", "{\"x\":1} true")
		bad_jsonl_empty, bad_jsonl_empty_err := dialect.eval("jsonl", "{\"x\":1}\n\n")
		bad_jsonl_record, bad_jsonl_record_err := dialect.eval("jsonl", "{\"x\":1}\n{\"y\":\n")
		bad_nums, bad_nums_err := dialect.eval("nums", "1 two 3")
		bad_matrix, bad_matrix_err := dialect.eval("nums", "1 2\n3\n", {matrix: true})
		bad_kv, bad_kv_err := dialect.eval("kv", "no separator")
		bad_env, bad_env_err := dialect.eval("env", "bad-line")
		bad_logfmt, bad_logfmt_err := dialect.eval("logfmt", "level=info msg=\"oops")
		bad_mdtable, bad_mdtable_err := dialect.eval("mdtable", "| a |\n| bad |\n")
		bad_ini, bad_ini_err := dialect.eval("ini", "[broken\nx=1")
		bad_semver, bad_semver_err := dialect.eval("semver", "1.2")
		bad_duration, bad_duration_err := dialect.eval("duration", "soon")
		bad_tap, bad_tap_err := dialect.eval("tap", "not okish")
		bad_xml, bad_xml_err := dialect.eval("xml", "&#x;", {mode: "decode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	for _, pair := range []struct {
		value string
		err   string
	}{
		{"bad_csv", "bad_csv_err"},
		{"bad_tsv", "bad_tsv_err"},
		{"bad_json", "bad_json_err"},
		{"bad_json_trailing", "bad_json_trailing_err"},
		{"bad_jsonl_empty", "bad_jsonl_empty_err"},
		{"bad_jsonl_record", "bad_jsonl_record_err"},
		{"bad_nums", "bad_nums_err"},
		{"bad_matrix", "bad_matrix_err"},
		{"bad_kv", "bad_kv_err"},
		{"bad_env", "bad_env_err"},
		{"bad_logfmt", "bad_logfmt_err"},
		{"bad_mdtable", "bad_mdtable_err"},
		{"bad_ini", "bad_ini_err"},
		{"bad_semver", "bad_semver_err"},
		{"bad_duration", "bad_duration_err"},
		{"bad_tap", "bad_tap_err"},
		{"bad_xml", "bad_xml_err"},
	} {
		if !interp.GetGlobal(pair.value).IsNil() {
			t.Fatalf("%s = %v, want nil", pair.value, interp.GetGlobal(pair.value))
		}
		if got := interp.GetGlobal(pair.err); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want non-empty error string", pair.err, got)
		}
	}
}

func TestDialectTextUnknownModesAreReported(t *testing.T) {
	interp := runWithLib(t, `
		json_bad, json_bad_err := dialect.eval("json", "{}", {mode: "bogus"})
		jsonptr_bad, jsonptr_bad_err := dialect.eval("jsonptr", {data: {name: "Ada"}, path: "/name"}, {mode: "bogus"})
		jsonl_bad, jsonl_bad_err := dialect.eval("jsonl", "{}", {mode: "bogus"})
		csv_bad, csv_bad_err := dialect.eval("csv", "a,b\n", {mode: "bogus"})
		tsv_bad, tsv_bad_err := dialect.eval("tsv", "a\tb\n", {mode: "bogus"})
		md_bad, md_bad_err := dialect.eval("mdtable", "| a |\n| - |\n| b |\n", {mode: "bogus"})
		kv_bad, kv_bad_err := dialect.eval("kv", "a=1", {mode: "bogus"})
		env_bad, env_bad_err := dialect.eval("env", "A=1", {mode: "bogus"})
		logfmt_bad, logfmt_bad_err := dialect.eval("logfmt", "a=1", {mode: "bogus"})
		ini_bad, ini_bad_err := dialect.eval("ini", "a=1", {mode: "bogus"})
		semver_bad, semver_bad_err := dialect.eval("semver", "1.2.3", {mode: "bogus"})
		duration_bad, duration_bad_err := dialect.eval("duration", "1s", {mode: "bogus"})
		tap_bad, tap_bad_err := dialect.eval("tap", "1..1\nok 1\n", {mode: "bogus"})
		xml_bad, xml_bad_err := dialect.eval("xml", "<x>", {mode: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	assertDialectModeError(t, interp.GetGlobal("json_bad"), interp.GetGlobal("json_bad_err"), "json dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("jsonptr_bad"), interp.GetGlobal("jsonptr_bad_err"), "jsonptr dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("jsonl_bad"), interp.GetGlobal("jsonl_bad_err"), "jsonl dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("csv_bad"), interp.GetGlobal("csv_bad_err"), "csv dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("tsv_bad"), interp.GetGlobal("tsv_bad_err"), "tsv dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("md_bad"), interp.GetGlobal("md_bad_err"), "mdtable dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("kv_bad"), interp.GetGlobal("kv_bad_err"), "kv dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("env_bad"), interp.GetGlobal("env_bad_err"), "env dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("logfmt_bad"), interp.GetGlobal("logfmt_bad_err"), "logfmt dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("ini_bad"), interp.GetGlobal("ini_bad_err"), "ini dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("semver_bad"), interp.GetGlobal("semver_bad_err"), "semver dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("duration_bad"), interp.GetGlobal("duration_bad_err"), "duration dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("tap_bad"), interp.GetGlobal("tap_bad_err"), "tap dialect: unknown mode")
	assertDialectModeError(t, interp.GetGlobal("xml_bad"), interp.GetGlobal("xml_bad_err"), "xml dialect: unknown mode")
}

func TestDialectTextModeAliasesKeepDirectionInference(t *testing.T) {
	interp := runWithLib(t, `
		json_decoded := dialect.eval("json", "{\"name\":\"Ada\"}", {mode: "decode"})
		json_encoded := dialect.eval("json", {name: "Ada"}, {mode: "format"})
		jsonptr_lookup := dialect.eval("jsonptr", {data: {name: "Ada"}, path: "/name"}, {mode: "lookup"})
		jsonptr_encoded := dialect.eval("jsonptr", ["a/b", "c~d"], {mode: "format"})
		jsonl_rows := dialect.eval("jsonl", "{\"x\":1}\n", {mode: "parse"})
		jsonl_text := dialect.eval("jsonl", [{x: 1}], {mode: "format"})
		csv_rows := dialect.eval("csv", "a,b\n", {mode: "decode"})
		csv_text := dialect.eval("csv", [["a", "b"]], {mode: "format"})
		kv_table := dialect.eval("kv", "a=1", {mode: "parse"})
		kv_text := dialect.eval("kv", {a: 1}, {mode: "format"})
		logfmt_table := dialect.eval("logfmt", "a=1", {mode: "decode"})
		logfmt_text := dialect.eval("logfmt", {a: 1}, {mode: "format"})
		ini_table := dialect.eval("ini", "a=1", {mode: "decode"})
		ini_text := dialect.eval("ini", {a: 1}, {mode: "format"})
		semver_table := dialect.eval("semver", "1.2.3", {mode: "validate"})
		semver_text := dialect.eval("semver", {major: 1, minor: 2, patch: 3}, {mode: "format"})
		duration_table := dialect.eval("duration", "1s", {mode: "decode"})
		duration_text := dialect.eval("duration", {seconds: 1}, {mode: "format"})
		tap_rows := dialect.eval("tap", "1..1\nok 1\n", {mode: "parse"})
		tap_text := dialect.eval("tap", [{kind: "plan", first: 1, last: 1}, {kind: "test", ok: true, number: 1}], {mode: "format"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("json_decoded").Table().RawGetString("name").Str(); got != "Ada" {
		t.Fatalf("json decoded name = %q, want Ada", got)
	}
	if got := interp.GetGlobal("json_encoded").Str(); !strings.Contains(got, `"name":"Ada"`) {
		t.Fatalf("json encoded = %q, want name field", got)
	}
	if got := interp.GetGlobal("jsonptr_lookup").Str(); got != "Ada" {
		t.Fatalf("jsonptr lookup = %q, want Ada", got)
	}
	if got := interp.GetGlobal("jsonptr_encoded").Str(); got != "/a~1b/c~0d" {
		t.Fatalf("jsonptr encoded = %q, want escaped pointer", got)
	}
	if got := interp.GetGlobal("jsonl_rows").Table().RawGetInt(1).Table().RawGetString("x").Int(); got != 1 {
		t.Fatalf("jsonl row x = %d, want 1", got)
	}
	if got := interp.GetGlobal("jsonl_text").Str(); !strings.Contains(got, `"x":1`) {
		t.Fatalf("jsonl text = %q, want x record", got)
	}
	if got := interp.GetGlobal("csv_rows").Table().RawGetInt(1).Table().RawGetInt(2).Str(); got != "b" {
		t.Fatalf("csv parsed cell = %q, want b", got)
	}
	if got := interp.GetGlobal("csv_text").Str(); got != "a,b\n" {
		t.Fatalf("csv text = %q, want a,b newline", got)
	}
	if got := interp.GetGlobal("kv_table").Table().RawGetString("a").Str(); got != "1" {
		t.Fatalf("kv table a = %q, want 1", got)
	}
	if got := interp.GetGlobal("kv_text").Str(); got != "a=1\n" {
		t.Fatalf("kv text = %q, want a=1 newline", got)
	}
	if got := interp.GetGlobal("logfmt_table").Table().RawGetString("a").Str(); got != "1" {
		t.Fatalf("logfmt table a = %q, want 1", got)
	}
	if got := interp.GetGlobal("logfmt_text").Str(); got != "a=1" {
		t.Fatalf("logfmt text = %q, want a=1", got)
	}
	if got := interp.GetGlobal("ini_table").Table().RawGetString("a").Str(); got != "1" {
		t.Fatalf("ini table a = %q, want 1", got)
	}
	if got := interp.GetGlobal("ini_text").Str(); got != "a=1\n" {
		t.Fatalf("ini text = %q, want a=1 newline", got)
	}
	if got := interp.GetGlobal("semver_table").Table().RawGetString("version").Str(); got != "1.2.3" {
		t.Fatalf("semver version = %q, want 1.2.3", got)
	}
	if got := interp.GetGlobal("semver_text").Str(); got != "1.2.3" {
		t.Fatalf("semver text = %q, want 1.2.3", got)
	}
	if got := interp.GetGlobal("duration_table").Table().RawGetString("seconds").Float(); got != 1 {
		t.Fatalf("duration seconds = %f, want 1", got)
	}
	if got := interp.GetGlobal("duration_text").Str(); got != "1s" {
		t.Fatalf("duration text = %q, want 1s", got)
	}
	if got := interp.GetGlobal("tap_rows").Table().Length(); got != 2 {
		t.Fatalf("tap rows = %d, want 2", got)
	}
	if got := interp.GetGlobal("tap_text").Str(); !strings.Contains(got, "ok 1") {
		t.Fatalf("tap text = %q, want ok 1", got)
	}
}

func assertDialectModeError(t *testing.T, value Value, err Value, want string) {
	t.Helper()
	if !value.IsNil() {
		t.Fatalf("value = %v, want nil", value)
	}
	if !err.IsString() || !strings.Contains(err.Str(), want) {
		t.Fatalf("err = %v, want string containing %q", err, want)
	}
}
