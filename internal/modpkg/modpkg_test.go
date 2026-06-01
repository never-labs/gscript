package modpkg

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestExplainResolvesStdlibCollectionReplaceAndModuleRoot(t *testing.T) {
	dir := newLockedModule(t)

	tests := []struct {
		name       string
		module     string
		wantKind   string
		wantPath   string
		wantSuffix string
	}{
		{name: "stdlib", module: "json", wantKind: "stdlib", wantPath: "json"},
		{name: "collection", module: "vendor:tool", wantKind: "collection", wantPath: "vendor", wantSuffix: filepath.Join("vendor", "tool.gs")},
		{name: "replace", module: "example.com/lib/sub", wantKind: "replace", wantPath: "example.com/lib", wantSuffix: filepath.Join("local", "lib", "sub.gs")},
		{name: "module root", module: "example.com/app/local", wantKind: "module", wantPath: "example.com/app", wantSuffix: filepath.Join("example.com", "app", "local.gs")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := Explain(dir, tt.module)
			if !report.OK || len(report.Diagnostics) != 0 {
				t.Fatalf("Explain(%q) = %#v, want ok", tt.module, report)
			}
			if report.Kind != tt.wantKind || report.Path != tt.wantPath {
				t.Fatalf("Explain(%q) kind/path = %q/%q, want %q/%q", tt.module, report.Kind, report.Path, tt.wantKind, tt.wantPath)
			}
			if tt.wantSuffix != "" && !strings.HasSuffix(report.File, tt.wantSuffix) {
				t.Fatalf("Explain(%q) file = %q, want suffix %q", tt.module, report.File, tt.wantSuffix)
			}
		})
	}
}

func TestListReportsManifestEntriesAndLocalResolution(t *testing.T) {
	dir := newLockedModule(t)

	report := List(dir)
	if !report.OK {
		t.Fatalf("List OK = false, diagnostics = %#v", report.Diagnostics)
	}
	if report.Module != "example.com/app" || report.GS != "0.1" {
		t.Fatalf("List module/gs = %q/%q, want example.com/app/0.1", report.Module, report.GS)
	}
	if len(report.Requires) != 0 {
		t.Fatalf("List requires = %#v, want none before adding require", report.Requires)
	}
	writeFile(t, filepath.Join(dir, "gscript.mod"), strings.Join([]string{
		"module example.com/app",
		"gs 0.1",
		"require example.com/lib v1.2.3",
		"collection vendor ./vendor",
		"replace example.com/lib v1.2.3 => ./local/lib",
		"",
	}, "\n"))

	report = List(dir)
	if !report.OK {
		t.Fatalf("List OK = false after require, diagnostics = %#v", report.Diagnostics)
	}
	if len(report.Requires) != 1 {
		t.Fatalf("List requires = %#v, want one", report.Requires)
	}
	req := report.Requires[0]
	if req.Path != "example.com/lib" || req.Version != "v1.2.3" || req.Kind != "replace" || req.Source != "example.com/lib" {
		t.Fatalf("List require = %#v, want replace resolution", req)
	}
	if !strings.HasSuffix(req.File, filepath.Join("local", "lib.gs")) {
		t.Fatalf("List require file = %q, want local replace file", req.File)
	}
	if len(report.Replaces) != 1 || !report.Replaces[0].Local {
		t.Fatalf("List replaces = %#v, want local replace", report.Replaces)
	}
	if len(report.Collections) != 1 || report.Collections[0].Name != "vendor" {
		t.Fatalf("List collections = %#v, want vendor collection", report.Collections)
	}
}

func TestDownloadFetchesGitHubTagArchiveIntoCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "gscript.mod"), strings.Join([]string{
		"module example.com/app",
		"gs 0.1",
		"require github.com/acme/toolkit/pkg v1.2.3",
		"",
	}, "\n"))
	archive := testGitHubZip(t, "toolkit-1.2.3/main.gs", "return 1\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/acme/toolkit/archive/refs/tags/v1.2.3.zip" {
			t.Fatalf("download path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	cache := filepath.Join(dir, "cache")
	report := Download(dir, DownloadOptions{CacheDir: cache, GitHubBaseURL: server.URL})
	if !report.OK {
		t.Fatalf("Download OK = false, diagnostics = %#v", report.Diagnostics)
	}
	if len(report.Modules) != 1 {
		t.Fatalf("Download modules = %#v, want one", report.Modules)
	}
	got := report.Modules[0]
	if got.Repo != "github.com/acme/toolkit" || got.Subdir != "pkg" || !got.Downloaded || !got.Extracted {
		t.Fatalf("Download entry = %#v, want github repo/subdir downloaded and extracted", got)
	}
	if _, err := os.Stat(filepath.Join(got.ExtractDir, "main.gs")); err != nil {
		t.Fatalf("extracted main.gs missing: %v", err)
	}

	again := Download(dir, DownloadOptions{CacheDir: cache, GitHubBaseURL: server.URL})
	if !again.OK || len(again.Modules) != 1 {
		t.Fatalf("second Download = %#v, want cached ok", again)
	}
	if again.Modules[0].Downloaded || again.Modules[0].Extracted {
		t.Fatalf("second Download entry = %#v, want cache hit", again.Modules[0])
	}
}

func TestDownloadRejectsNonGitHubModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "gscript.mod"), strings.Join([]string{
		"module example.com/app",
		"gs 0.1",
		"require example.com/lib v1.0.0",
		"",
	}, "\n"))

	report := Download(dir, DownloadOptions{CacheDir: filepath.Join(dir, "cache")})
	if report.OK || len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "GS9111" {
		t.Fatalf("Download = %#v, want unsupported github diagnostic", report)
	}
}

func TestScanStaticRequiresUsesAST(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.gs")
	writeFile(t, file, strings.Join([]string{
		`literal := "require(\"ignored.string\")"`,
		`// require("ignored.comment")`,
		`direct := require("example.com/direct")`,
		`func nested() { return require("example.com/nested") }`,
		`dynamic := require(name)`,
		`again := require("example.com/direct")`,
		"",
	}, "\n"))

	got, err := ScanStaticRequires(file)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.com/direct", "example.com/nested"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ScanStaticRequires = %#v, want %#v", got, want)
	}
}

func testGitHubZip(t *testing.T, name, data string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(w, data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
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
