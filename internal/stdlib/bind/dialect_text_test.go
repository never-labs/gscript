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
