package dialect

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseMarkdownTable(t *testing.T) {
	src := `
| Name | Score | Note |
| :--- | ---: | :---: |
| Ada | 42 | uses \| safely |
| Bob | 7 |
`
	got, err := ParseMarkdownTable(src)
	if err != nil {
		t.Fatalf("ParseMarkdownTable: %v", err)
	}
	if want := []string{"Name", "Score", "Note"}; !reflect.DeepEqual(got.Headers, want) {
		t.Fatalf("headers = %#v, want %#v", got.Headers, want)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(got.Rows))
	}
	if got.Rows[0]["Name"] != "Ada" || got.Rows[0]["Note"] != "uses | safely" {
		t.Fatalf("first row = %#v", got.Rows[0])
	}
	if got.Rows[1]["Name"] != "Bob" || got.Rows[1]["Note"] != "" {
		t.Fatalf("second row = %#v", got.Rows[1])
	}
}

func TestEncodeMarkdownTable(t *testing.T) {
	text, err := EncodeMarkdownTable(MarkdownTable{
		Headers: []string{"Name", "Note"},
		Rows: []map[string]string{
			{"Name": "Ada", "Note": `uses | and \`},
			{"Name": "Bob", "Note": "line\nbreak"},
		},
	})
	if err != nil {
		t.Fatalf("EncodeMarkdownTable: %v", err)
	}
	want := "| Name | Note |\n| --- | --- |\n| Ada | uses \\| and \\\\ |\n| Bob | line<br>break |\n"
	if text != want {
		t.Fatalf("encoded = %q, want %q", text, want)
	}

	roundtrip, err := ParseMarkdownTable(text)
	if err != nil {
		t.Fatalf("ParseMarkdownTable(encoded): %v", err)
	}
	if roundtrip.Rows[0]["Note"] != `uses | and \` {
		t.Fatalf("roundtrip note = %q", roundtrip.Rows[0]["Note"])
	}
}

func TestParseMarkdownTableRejectsBadDelimiter(t *testing.T) {
	_, err := ParseMarkdownTable("| a | b |\n| -- | --- |\n| 1 | 2 |\n")
	if err == nil || !strings.Contains(err.Error(), "delimiter cells must contain at least three hyphens") {
		t.Fatalf("err = %v", err)
	}
}
