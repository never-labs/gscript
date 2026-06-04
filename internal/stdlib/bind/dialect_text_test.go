package bind

import "testing"

func TestDialectDelimitedParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		csv_text := dialect.eval("csv", {{"name", "score"}, {"Ada", 42}, {"Bob", 7}}, {mode: "encode"})
		csv_roundtrip := dialect.eval("csv", csv_text)
		csv_header_text := dialect.eval("csv", {{name: "Ada", score: 42}, {name: "Bob", score: 7}}, {mode: "encode", headers: {"name", "score"}})
		csv_header_roundtrip := dialect.eval("csv", csv_header_text, {headers: true})
		tsv_text := dialect.eval("tsv", {{"name", "score"}, {"Ada", 42}}, {mode: "encode"})
		tsv_roundtrip := dialect.eval("tsv", tsv_text)
		bad, bad_err := dialect.eval("csv", {"not a row"}, {mode: "encode"})
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
