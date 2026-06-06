package bind

import (
	"archive/zip"
	"bytes"
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

func TestDialectXLSXParsesFirstWorksheet(t *testing.T) {
	eval := BuildDialect(HostOptions{}, nil).RawGetString("eval").GoFunction()
	if eval == nil {
		t.Fatalf("dialect.eval is not a Go function")
	}
	xlsx := testXLSXWorkbook(t)
	gotValues, err := eval.Fn([]Value{StringValue("xlsx"), StringValue(xlsx)})
	if err != nil {
		t.Fatalf("xlsx parse: %v", err)
	}
	rows := gotValues[0].Table()
	if got := rows.Length(); got != 2 {
		t.Fatalf("rows length = %d, want 2", got)
	}
	header := rows.RawGetInt(1).Table()
	if got := header.RawGetInt(1).Str(); got != "name" {
		t.Fatalf("A1 = %q, want name", got)
	}
	if got := header.RawGetInt(2).Str(); got != "score" {
		t.Fatalf("B1 = %q, want score", got)
	}
	row := rows.RawGetInt(2).Table()
	if got := row.RawGetInt(1).Str(); got != "Ada" {
		t.Fatalf("A2 = %q, want Ada", got)
	}
	if got := row.RawGetInt(2).Str(); got != "42" {
		t.Fatalf("B2 = %q, want 42", got)
	}

	badValues, err := eval.Fn([]Value{StringValue("excel"), StringValue("not zip")})
	if err != nil {
		t.Fatalf("excel bad parse: %v", err)
	}
	if !badValues[0].IsNil() || !strings.Contains(badValues[1].Str(), "xlsx dialect: open zip") {
		t.Fatalf("bad xlsx = %v err = %q, want invalid zip tuple", badValues[0], badValues[1].Str())
	}
}

func TestDialectXLSXEncodesHeaderRowsAndExcelAliasRoundTrips(t *testing.T) {
	eval := BuildDialect(HostOptions{}, nil).RawGetString("eval").GoFunction()
	if eval == nil {
		t.Fatalf("dialect.eval is not a Go function")
	}
	rows := NewAppendArrayTable(1)
	rows.RawSetInt(1, TableValue(testTableFromMap(map[string]Value{
		"name":  StringValue("Ada"),
		"score": IntValue(42),
	})))
	headers := NewAppendArrayTable(2)
	headers.RawSetInt(1, StringValue("name"))
	headers.RawSetInt(2, StringValue("score"))
	encodeOpts := testTableFromMap(map[string]Value{
		"mode":    StringValue("encode"),
		"headers": TableValue(headers),
		"sheet":   StringValue("Summary"),
	})
	encoded, err := eval.Fn([]Value{StringValue("xlsx"), TableValue(rows), TableValue(encodeOpts)})
	if err != nil {
		t.Fatalf("xlsx encode: %v", err)
	}
	if len(encoded) != 1 || !encoded[0].IsString() || encoded[0].Str() == "" {
		t.Fatalf("encoded xlsx = %#v, want non-empty byte string", encoded)
	}
	decoded, err := eval.Fn([]Value{
		StringValue("excel"),
		encoded[0],
		TableValue(testTableFromMap(map[string]Value{"headers": BoolValue(true)})),
	})
	if err != nil {
		t.Fatalf("excel alias decode: %v", err)
	}
	gotRows := decoded[0].Table()
	if gotRows.Length() != 1 {
		t.Fatalf("decoded rows = %d, want 1", gotRows.Length())
	}
	row := gotRows.RawGetInt(1).Table()
	if got := row.RawGetString("name").Str(); got != "Ada" {
		t.Fatalf("decoded name = %q, want Ada", got)
	}
	if got := row.RawGetString("score").Str(); got != "42" {
		t.Fatalf("decoded score = %q, want 42", got)
	}
}

func TestDialectXLSXDecodePreservesSparseCellColumns(t *testing.T) {
	eval := BuildDialect(HostOptions{}, nil).RawGetString("eval").GoFunction()
	if eval == nil {
		t.Fatalf("dialect.eval is not a Go function")
	}
	xlsx := testSparseXLSXWorkbook(t)
	decoded, err := eval.Fn([]Value{
		StringValue("xlsx"),
		StringValue(xlsx),
		TableValue(testTableFromMap(map[string]Value{"headers": BoolValue(true)})),
	})
	if err != nil {
		t.Fatalf("xlsx sparse decode: %v", err)
	}
	rows := decoded[0].Table()
	if got := rows.Length(); got != 1 {
		t.Fatalf("rows length = %d, want 1", got)
	}
	row := rows.RawGetInt(1).Table()
	if got := row.RawGetString("name").Str(); got != "Ada" {
		t.Fatalf("decoded name = %q, want Ada", got)
	}
	if got := row.RawGetString("score").Str(); got != "" {
		t.Fatalf("decoded sparse score = %q, want empty string", got)
	}
	if got := row.RawGetString("note").Str(); got != "promoted" {
		t.Fatalf("decoded note = %q, want promoted", got)
	}
}

func testXLSXWorkbook(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipFile := func(name, body string) {
		t.Helper()
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	writeZipFile("xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>name</t></si>
  <si><t>score</t></si>
  <si><t>Ada</t></si>
</sst>`)
	writeZipFile("xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>
    <row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2"><v>42</v></c></row>
  </sheetData>
</worksheet>`)
	if err := zw.Close(); err != nil {
		t.Fatalf("close xlsx zip: %v", err)
	}
	return buf.String()
}

func testSparseXLSXWorkbook(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipFile := func(name, body string) {
		t.Helper()
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	writeZipFile("xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>name</t></si>
  <si><t>score</t></si>
  <si><t>note</t></si>
  <si><t>Ada</t></si>
  <si><t>promoted</t></si>
</sst>`)
	writeZipFile("xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="s"><v>2</v></c></row>
    <row r="2"><c r="A2" t="s"><v>3</v></c><c r="C2" t="s"><v>4</v></c></row>
  </sheetData>
</worksheet>`)
	if err := zw.Close(); err != nil {
		t.Fatalf("close sparse xlsx zip: %v", err)
	}
	return buf.String()
}
