package dialect

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTAP(t *testing.T) {
	doc, err := ParseTAP(`TAP version 13
1..3
ok 1 - boot
not ok 2 - deploy # TODO flaky
# expected ready
# got timeout
ok 3 - cleanup # SKIP already done
`)
	if err != nil {
		t.Fatalf("ParseTAP: %v", err)
	}
	if len(doc.Rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(doc.Rows))
	}
	if got := doc.Rows[0]; got.Kind != "version" || got.Version != 13 {
		t.Fatalf("version row = %+v", got)
	}
	if got := doc.Rows[1]; got.Kind != "plan" || got.First != 1 || got.Last != 3 {
		t.Fatalf("plan row = %+v", got)
	}
	if got := doc.Rows[2]; got.Kind != "test" || !got.OK || got.Number != 1 || got.Name != "boot" {
		t.Fatalf("first test row = %+v", got)
	}
	if got := doc.Rows[3]; got.Kind != "test" || got.OK || got.Directive != "TODO" || got.Reason != "flaky" {
		t.Fatalf("second test row = %+v", got)
	}
	if got := doc.Rows[3].Diagnostics; !reflect.DeepEqual(got, []string{"expected ready", "got timeout"}) {
		t.Fatalf("diagnostics = %#v", got)
	}
	if got := doc.Rows[4]; got.Kind != "test" || got.Directive != "SKIP" || got.Reason != "already done" {
		t.Fatalf("third test row = %+v", got)
	}
}

func TestParseTAPToleratesDiagnosticsAndUnknownDirectives(t *testing.T) {
	doc, err := ParseTAP(`
# suite starting
ok - implicit number # note from harness
# attached detail
1..1 # skip no numbered tests
`)
	if err != nil {
		t.Fatalf("ParseTAP: %v", err)
	}
	if len(doc.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(doc.Rows))
	}
	if got := doc.Rows[0]; got.Kind != "diagnostic" || got.Text != "suite starting" {
		t.Fatalf("leading diagnostic = %+v", got)
	}
	if got := doc.Rows[1]; got.Kind != "test" || !got.OK || got.Number != 0 || got.Name != "implicit number" || got.Directive != "" || got.Reason != "note from harness" {
		t.Fatalf("test row = %+v", got)
	}
	if got := doc.Rows[1].Diagnostics; !reflect.DeepEqual(got, []string{"attached detail"}) {
		t.Fatalf("diagnostics = %#v", got)
	}
	if got := doc.Rows[2]; got.Kind != "plan" || got.Directive != "SKIP" || got.Reason != "no numbered tests" {
		t.Fatalf("plan row = %+v", got)
	}
}

func TestEncodeTAP(t *testing.T) {
	got, err := EncodeTAP(TAPDocument{Rows: []TAPRow{
		{Kind: "version", Version: 13},
		{Kind: "plan", First: 1, Last: 2},
		{Kind: "test", OK: true, Number: 1, Name: "boot"},
		{Kind: "test", OK: false, Number: 2, Name: "deploy", Directive: "TODO", Reason: "flaky", Diagnostics: []string{"expected ready"}},
	}})
	if err != nil {
		t.Fatalf("EncodeTAP: %v", err)
	}
	want := "TAP version 13\n1..2\nok 1 - boot\nnot ok 2 - deploy # TODO flaky\n# expected ready\n"
	if got != want {
		t.Fatalf("EncodeTAP = %q, want %q", got, want)
	}
}

func TestEncodeTAPRejectsUnsupportedRowKind(t *testing.T) {
	_, err := EncodeTAP(TAPDocument{Rows: []TAPRow{{Kind: "bogus"}}})
	if err == nil || !strings.Contains(err.Error(), `tap dialect: row 1: unsupported kind "bogus"`) {
		t.Fatalf("EncodeTAP error = %v", err)
	}
}

func TestParseTAPErrors(t *testing.T) {
	_, err := ParseTAP("looks wrong")
	if err == nil || !strings.Contains(err.Error(), `tap dialect: line 1: unrecognized TAP line "looks wrong"`) {
		t.Fatalf("ParseTAP error = %v", err)
	}
}
