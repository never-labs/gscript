package csv

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	rows, err := Parse("name,score\n\"a,b\",10\n c ,20\n", Options{TrimSpace: true})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := [][]string{
		{"name", "score"},
		{"a,b", "10"},
		{"c ", "20"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("Parse() = %#v, want %#v", rows, want)
	}
}

func TestParseOptions(t *testing.T) {
	rows, err := Parse("# skip\n name;note\n1;bare \"quote\n", Options{
		Sep:        ';',
		Comment:    '#',
		TrimSpace:  true,
		LazyQuotes: true,
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := [][]string{
		{"name", "note"},
		{"1", "bare \"quote"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("Parse() = %#v, want %#v", rows, want)
	}
}

func TestParseWithHeaders(t *testing.T) {
	rows, err := ParseWithHeaders("name,score\nalice,10\nbob,20\n", Options{})
	if err != nil {
		t.Fatalf("ParseWithHeaders returned error: %v", err)
	}

	want := []map[string]string{
		{"name": "alice", "score": "10"},
		{"name": "bob", "score": "20"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("ParseWithHeaders() = %#v, want %#v", rows, want)
	}
}

func TestParseWithHeadersEmptyInput(t *testing.T) {
	rows, err := ParseWithHeaders("", Options{})
	if err != nil {
		t.Fatalf("ParseWithHeaders returned error: %v", err)
	}
	if rows != nil {
		t.Fatalf("ParseWithHeaders empty = %#v, want nil", rows)
	}
}

func TestEncode(t *testing.T) {
	got, err := Encode([][]string{
		{"a", "b"},
		{"1", "2"},
		{"x,y", "line\nbreak"},
	}, Options{})
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}

	want := "a,b\n1,2\n\"x,y\",\"line\nbreak\"\n"
	if got != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}
}

func TestEncodeWithHeaders(t *testing.T) {
	got, err := EncodeWithHeaders(
		[]map[string]string{{"name": "alice", "score": "10"}},
		[]string{"name", "score"},
		Options{Sep: ';'},
	)
	if err != nil {
		t.Fatalf("EncodeWithHeaders returned error: %v", err)
	}

	want := "name;score\nalice;10\n"
	if got != want {
		t.Fatalf("EncodeWithHeaders() = %q, want %q", got, want)
	}
}

func TestWritePropagatesWriterErrors(t *testing.T) {
	err := Write([][]string{{strings.Repeat("x", 8)}}, Options{}, shortWriter{limit: 4})
	if err == nil {
		t.Fatalf("Write returned nil error")
	}
}

type shortWriter struct {
	limit int
}

func (w shortWriter) Write(p []byte) (int, error) {
	if len(p) > w.limit {
		return w.limit, errShortWrite
	}
	return len(p), nil
}

type shortWriteError string

func (e shortWriteError) Error() string {
	return string(e)
}

const errShortWrite shortWriteError = "short write"
