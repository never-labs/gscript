package dialect

import (
	"reflect"
	"testing"
)

func TestParseLogfmtParsesQuotedValuesAndFlags(t *testing.T) {
	got, err := ParseLogfmt(`level=info msg="hello world" ok trace_id=abc escaped="a\"b" path=/tmp/a`)
	if err != nil {
		t.Fatalf("ParseLogfmt: %v", err)
	}
	want := []LogfmtPair{
		{Key: "level", Value: "info"},
		{Key: "msg", Value: "hello world"},
		{Key: "ok", Value: "true"},
		{Key: "trace_id", Value: "abc"},
		{Key: "escaped", Value: `a"b`},
		{Key: "path", Value: "/tmp/a"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseLogfmt = %#v, want %#v", got, want)
	}
}

func TestParseLogfmtPreservesDuplicateKeys(t *testing.T) {
	got, err := ParseLogfmt(`tag=first tag=second ok tag="third value"`)
	if err != nil {
		t.Fatalf("ParseLogfmt: %v", err)
	}
	want := []LogfmtPair{
		{Key: "tag", Value: "first"},
		{Key: "tag", Value: "second"},
		{Key: "ok", Value: "true"},
		{Key: "tag", Value: "third value"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseLogfmt = %#v, want %#v", got, want)
	}
}

func TestParseLogfmtRejectsUnterminatedQuote(t *testing.T) {
	if _, err := ParseLogfmt(`level=info msg="oops`); err == nil {
		t.Fatalf("ParseLogfmt unterminated quote returned nil error")
	}
}

func TestEncodeLogfmtSortsAndQuotes(t *testing.T) {
	got := EncodeLogfmt(map[string]string{
		"msg":   "hello world",
		"level": "info",
		"empty": "",
		"quote": `a"b`,
	})
	want := `empty="" level=info msg="hello world" quote="a\"b"`
	if got != want {
		t.Fatalf("EncodeLogfmt = %q, want %q", got, want)
	}
}
