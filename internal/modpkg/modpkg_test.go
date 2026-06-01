package modpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRequireTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		wantPath    string
		wantVersion string
		wantErr     bool
	}{
		{
			name:        "path and version",
			target:      "example.com/lib@v1.2.3",
			wantPath:    "example.com/lib",
			wantVersion: "v1.2.3",
		},
		{
			name:        "last at separates version",
			target:      "example.com/user@host/lib@v0.1.0",
			wantPath:    "example.com/user@host/lib",
			wantVersion: "v0.1.0",
		},
		{
			name:    "missing version",
			target:  "example.com/lib@",
			wantErr: true,
		},
		{
			name:    "missing path",
			target:  "@v1.0.0",
			wantErr: true,
		},
		{
			name:    "missing separator",
			target:  "example.com/lib",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRequireTarget(tt.target)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRequireTarget(%q) error = nil, want error", tt.target)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRequireTarget(%q) error = %v", tt.target, err)
			}
			if got.Path != tt.wantPath || got.Version != tt.wantVersion {
				t.Fatalf("ParseRequireTarget(%q) = %#v, want path %q version %q", tt.target, got, tt.wantPath, tt.wantVersion)
			}
		})
	}
}

func TestLockWritesCollectionAndLocalReplaceSums(t *testing.T) {
	dir := newLockedModule(t)

	report := Lock(dir)
	if !report.OK {
		t.Fatalf("Lock OK = false, diagnostics = %#v", report.Diagnostics)
	}
	if report.Sum != filepath.Join(dir, SumFileName) {
		t.Fatalf("Lock sum path = %q, want %q", report.Sum, filepath.Join(dir, SumFileName))
	}
	if len(report.Entries) != 2 {
		t.Fatalf("Lock entries = %#v, want 2 entries", report.Entries)
	}

	assertSumEntry(t, report.Entries, SumEntry{
		Kind:   "collection",
		Path:   "vendor",
		Target: "./vendor",
	})
	assertSumEntry(t, report.Entries, SumEntry{
		Kind:    "replace",
		Path:    "example.com/lib",
		Version: "v1.2.3",
		Target:  "./local/lib",
	})

	onDisk, err := readSumFile(filepath.Join(dir, SumFileName))
	if err != nil {
		t.Fatalf("readSumFile error = %v", err)
	}
	if len(onDisk) != len(report.Entries) {
		t.Fatalf("sum file entries = %#v, want %#v", onDisk, report.Entries)
	}
	for _, entry := range report.Entries {
		assertSumEntry(t, onDisk, entry)
	}
}

func TestVerifySumDetectsCollectionAndLocalReplaceChanges(t *testing.T) {
	dir := newLockedModule(t)

	if report := Lock(dir); !report.OK {
		t.Fatalf("Lock OK = false, diagnostics = %#v", report.Diagnostics)
	}
	if diags := VerifySum(dir); len(diags) != 0 {
		t.Fatalf("VerifySum diagnostics after lock = %#v, want none", diags)
	}

	writeFile(t, filepath.Join(dir, "vendor", "vendor.gs"), "print(\"changed vendor\")\n")
	writeFile(t, filepath.Join(dir, "local", "lib", "lib.gs"), "print(\"changed lib\")\n")

	diags := VerifySum(dir)
	if len(diags) != 2 {
		t.Fatalf("VerifySum diagnostics = %#v, want 2 checksum mismatches", diags)
	}
	assertDiagnostic(t, diags, "GS9109", "checksum mismatch for vendor")
	assertDiagnostic(t, diags, "GS9109", "checksum mismatch for example.com/lib")
}

func newLockedModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "gscript.mod"), strings.Join([]string{
		"module example.com/app",
		"gs 0.1",
		"collection vendor ./vendor",
		"replace example.com/lib v1.2.3 => ./local/lib",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "main.gs"), "require(\"vendor:tool\")\nrequire(\"example.com/lib\")\n")
	writeFile(t, filepath.Join(dir, "vendor", "vendor.gs"), "print(\"vendor\")\n")
	writeFile(t, filepath.Join(dir, "local", "lib", "lib.gs"), "print(\"lib\")\n")
	return dir
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func assertSumEntry(t *testing.T, entries []SumEntry, want SumEntry) {
	t.Helper()

	for _, got := range entries {
		if got.Kind != want.Kind || got.Path != want.Path || got.Version != want.Version || got.Target != want.Target {
			continue
		}
		if want.Hash != "" && got.Hash != want.Hash {
			t.Fatalf("sum entry %#v hash = %q, want %q", want, got.Hash, want.Hash)
		}
		if got.Hash == "" || !strings.HasPrefix(got.Hash, "h1:") {
			t.Fatalf("sum entry %#v has invalid hash %q", got, got.Hash)
		}
		return
	}
	t.Fatalf("missing sum entry %#v in %#v", want, entries)
}

func assertDiagnostic(t *testing.T, diags []Diagnostic, code, message string) {
	t.Helper()

	for _, diag := range diags {
		if diag.Code == code && diag.Message == message {
			return
		}
	}
	t.Fatalf("missing diagnostic code %q message %q in %#v", code, message, diags)
}
