package bind

import (
	"strings"
	"testing"
)

func TestDialectPEMParseSingleAndMultipleBlocks(t *testing.T) {
	interp := runWithLib(t, `
		pem_text := "-----BEGIN CERTIFICATE-----\nSGVsbG8=\n-----END CERTIFICATE-----\n-----BEGIN PRIVATE KEY-----\nd29ybGQ=\n-----END PRIVATE KEY-----\n"
		blocks := dialect.eval("pem", pem_text)
		first := dialect.eval("pem", "-----BEGIN CERTIFICATE-----\nSGVsbG8=\n-----END CERTIFICATE-----\n", {single: true})
		bad_single, bad_single_err := dialect.eval("pem", pem_text, {single: true})
		bad, bad_err := dialect.eval("pem", "not pem")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	blocks := interp.GetGlobal("blocks").Table()
	if got := blocks.Length(); got != 2 {
		t.Fatalf("blocks length = %d, want 2", got)
	}
	first := blocks.RawGetInt(1).Table()
	if got := first.RawGetString("type").Str(); got != "CERTIFICATE" {
		t.Fatalf("first.type = %q, want CERTIFICATE", got)
	}
	if got := first.RawGetString("body").Str(); got != "Hello" {
		t.Fatalf("first.body = %q, want Hello", got)
	}
	if got := first.RawGetString("text").Str(); got != "Hello" {
		t.Fatalf("first.text = %q, want Hello", got)
	}
	if got := first.RawGetString("raw").Str(); !strings.Contains(got, "-----BEGIN CERTIFICATE-----") {
		t.Fatalf("first.raw = %q, want PEM text", got)
	}
	if got := blocks.RawGetInt(2).Table().RawGetString("body").Str(); got != "world" {
		t.Fatalf("second.body = %q, want world", got)
	}
	if got := interp.GetGlobal("first").Table().RawGetString("type").Str(); got != "CERTIFICATE" {
		t.Fatalf("single type = %q, want CERTIFICATE", got)
	}
	if !interp.GetGlobal("bad_single").IsNil() || !strings.Contains(interp.GetGlobal("bad_single_err").Str(), "expected exactly one block") {
		t.Fatalf("bad_single = %v err = %q, want single-count error", interp.GetGlobal("bad_single"), interp.GetGlobal("bad_single_err").Str())
	}
	if !interp.GetGlobal("bad").IsNil() || !strings.Contains(interp.GetGlobal("bad_err").Str(), "no PEM block found") {
		t.Fatalf("bad = %v err = %q, want no-block error", interp.GetGlobal("bad"), interp.GetGlobal("bad_err").Str())
	}
}

func TestDialectPEMEncodeDeterministicBlocks(t *testing.T) {
	eval := BuildDialect(HostOptions{}, nil).RawGetString("eval").GoFunction()
	if eval == nil {
		t.Fatalf("dialect.eval is not a Go function")
	}
	headers := testTableFromMap(map[string]Value{
		"X-Name":    StringValue("fixture"),
		"Proc-Type": StringValue("4,ENCRYPTED"),
		"DEK-Info":  StringValue("AES-128-CBC,001122"),
	})
	block := testTableFromMap(map[string]Value{
		"type":    StringValue("CERTIFICATE"),
		"headers": TableValue(headers),
		"body":    StringValue("hello"),
	})
	gotValues, err := eval.Fn([]Value{
		StringValue("pem"),
		TableValue(block),
		TableValue(testTableFromMap(map[string]Value{"mode": StringValue("encode")})),
	})
	if err != nil {
		t.Fatalf("pem encode: %v", err)
	}
	want := "-----BEGIN CERTIFICATE-----\n" +
		"Proc-Type: 4,ENCRYPTED\n" +
		"DEK-Info: AES-128-CBC,001122\n" +
		"X-Name: fixture\n" +
		"\n" +
		"aGVsbG8=\n" +
		"-----END CERTIFICATE-----\n"
	if got := gotValues[0].Str(); got != want {
		t.Fatalf("encoded PEM = %q, want %q", got, want)
	}

	array := NewAppendArrayTable(2)
	array.RawSetInt(1, TableValue(block))
	array.RawSetInt(2, TableValue(testTableFromMap(map[string]Value{
		"type": StringValue("PRIVATE KEY"),
		"text": StringValue("world"),
	})))
	gotValues, err = eval.Fn([]Value{StringValue("pem"), TableValue(array), TableValue(testTableFromMap(map[string]Value{"mode": StringValue("format")}))})
	if err != nil {
		t.Fatalf("pem format array: %v", err)
	}
	if got := gotValues[0].Str(); !strings.Contains(got, "-----END CERTIFICATE-----\n-----BEGIN PRIVATE KEY-----") || !strings.Contains(got, "d29ybGQ=") {
		t.Fatalf("formatted array PEM = %q, want concatenated blocks", got)
	}
}
