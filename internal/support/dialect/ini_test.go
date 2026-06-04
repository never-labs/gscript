package dialect

import (
	"strings"
	"testing"
)

func TestParseINI(t *testing.T) {
	doc, err := ParseINI(`
# comment
app = ledger
enabled: true

[database]
host = db.internal
port = 5432
`)
	if err != nil {
		t.Fatalf("ParseINI: %v", err)
	}
	if got := doc.Root["app"]; got != "ledger" {
		t.Fatalf("root app = %q, want ledger", got)
	}
	if got := doc.Root["enabled"]; got != "true" {
		t.Fatalf("root enabled = %q, want true", got)
	}
	if got := doc.Sections["database"]["host"]; got != "db.internal" {
		t.Fatalf("database.host = %q, want db.internal", got)
	}
	if got := doc.Sections["database"]["port"]; got != "5432" {
		t.Fatalf("database.port = %q, want 5432", got)
	}
}

func TestEncodeINI(t *testing.T) {
	got, err := EncodeINI(INIDocument{
		Root: map[string]string{
			"enabled": "true",
			"app":     "ledger",
		},
		Sections: map[string]map[string]string{
			"database": {
				"port": "5432",
				"host": "db.internal",
			},
		},
	})
	if err != nil {
		t.Fatalf("EncodeINI: %v", err)
	}
	want := "app=ledger\nenabled=true\n\n[database]\nhost=db.internal\nport=5432\n"
	if got != want {
		t.Fatalf("EncodeINI = %q, want %q", got, want)
	}
}

func TestParseINIErrors(t *testing.T) {
	_, err := ParseINI("[broken")
	if err == nil || !strings.Contains(err.Error(), "ini dialect: line 1: malformed section header") {
		t.Fatalf("ParseINI error = %v", err)
	}
}
