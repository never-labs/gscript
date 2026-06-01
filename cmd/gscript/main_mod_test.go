package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModInitGraphAndVerify(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.gs"), []byte(`helper := require("pkg.helper")
jsonMod := require("json")
`), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"init", "--module", "example.com/demo", "--dir", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod init code = %d, stderr = %q", code, stderr.String())
	}
	manifestPath := filepath.Join(dir, "gscript.mod")
	if !strings.Contains(stdout.String(), manifestPath) {
		t.Fatalf("stdout = %q, want manifest path", stdout.String())
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(manifestBytes); !strings.Contains(got, "module example.com/demo\n") || !strings.Contains(got, "gs 0.1\n") {
		t.Fatalf("manifest = %q, want compact gscript.mod format", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"graph", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod graph code = %d, stderr = %q", code, stderr.String())
	}
	var graph modGraphReport
	if err := json.Unmarshal(stdout.Bytes(), &graph); err != nil {
		t.Fatalf("stdout is not JSON graph: %v; stdout = %q", err, stdout.String())
	}
	if len(graph.Files) != 1 || !containsString(graph.Files[0].Requires, "pkg.helper") || !containsString(graph.Files[0].Requires, "json") {
		t.Fatalf("graph = %+v, want static requires", graph)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod verify code = %d, stderr = %q", code, stderr.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if !verify.OK || verify.Manifest != manifestPath {
		t.Fatalf("verify = %+v, want ok manifest", verify)
	}
}

func TestModVerifyReportsMissingManifest(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("mod verify code = %d, want 1", code)
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if verify.OK || len(verify.Diagnostics) == 0 || verify.Diagnostics[0].Code != "GS9103" {
		t.Fatalf("verify = %+v, want missing manifest diagnostic", verify)
	}
}

func TestModCheckRunsVerification(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte("module example.com/demo\ngs 0.1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"check", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod check code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if !verify.OK || verify.Manifest != filepath.Join(dir, "gscript.mod") {
		t.Fatalf("verify = %+v, want ok manifest", verify)
	}
}

func TestModListReportsManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte(`module example.com/demo
gs 0.1
require example.com/lib v1.2.3
replace example.com/lib => ./local/lib
collection vendor ./vendor
`), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"list", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod list code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var list modListReport
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("stdout is not JSON list report: %v; stdout = %q", err, stdout.String())
	}
	if !list.OK || list.Module != "example.com/demo" || len(list.Requires) != 1 || list.Requires[0].Kind != "replace" {
		t.Fatalf("list = %+v, want module and resolved replace require", list)
	}
}

func TestModDownloadFetchesGitHubArchive(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte(`module example.com/demo
gs 0.1
require github.com/acme/toolkit v1.2.3
`), 0644); err != nil {
		t.Fatal(err)
	}
	archive := testCommandGitHubZip(t, "toolkit-1.2.3/main.gs", "return 1\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/acme/toolkit/archive/refs/tags/v1.2.3.zip" {
			t.Fatalf("download path = %q", r.URL.Path)
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	cache := filepath.Join(dir, "cache")
	code := runModCommand([]string{"download", "--json", "--cache", cache, "--github-base", server.URL, dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod download code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var report modDownloadReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON download report: %v; stdout = %q", err, stdout.String())
	}
	if !report.OK || len(report.Modules) != 1 || !report.Modules[0].Downloaded || !report.Modules[0].Extracted {
		t.Fatalf("download report = %+v, want downloaded and extracted module", report)
	}
	if _, err := os.Stat(filepath.Join(report.Modules[0].ExtractDir, "main.gs")); err != nil {
		t.Fatalf("extracted module file missing: %v", err)
	}
}

func TestModAddAndTidy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.gs"), []byte(`net := require("example.com/lib/net")
json := require("json")
localMod := require("pkg.helper")
vendored := require("vendor:foo")
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte(`module example.com/demo
gs 0.1
require example.com/lib v0.1.0
require example.com/unused v9.9.9
collection vendor ./vendor
`), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"add", "--dir", dir, "example.com/lib@v0.2.0"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod add code = %d, stderr = %q", code, stderr.String())
	}
	manifestBytes, err := os.ReadFile(filepath.Join(dir, "gscript.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(manifestBytes); !strings.Contains(got, "require example.com/lib v0.2.0") {
		t.Fatalf("manifest after add = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"tidy", "--json", "--dir", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod tidy code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var tidy modTidyReport
	if err := json.Unmarshal(stdout.Bytes(), &tidy); err != nil {
		t.Fatalf("stdout is not JSON tidy report: %v; stdout = %q", err, stdout.String())
	}
	if !tidy.OK || !containsString(tidy.Removed, "example.com/unused") || len(tidy.Missing) != 0 {
		t.Fatalf("tidy = %+v, want removed unused and no missing", tidy)
	}
	manifestBytes, err = os.ReadFile(filepath.Join(dir, "gscript.mod"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(manifestBytes)
	if strings.Contains(got, "example.com/unused") {
		t.Fatalf("manifest after tidy still contains unused require: %q", got)
	}
	if strings.Contains(got, "json") || strings.Contains(got, "vendor:foo") || strings.Contains(got, "pkg.helper") {
		t.Fatalf("manifest after tidy added non-third-party require: %q", got)
	}
}

func testCommandGitHubZip(t *testing.T, name, data string) []byte {
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

func TestModTidyReportsMissingExternalRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.gs"), []byte(`lib := require("example.com/lib/net")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte("module example.com/demo\ngs 0.1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"tidy", "--json", "--dir", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("mod tidy code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var tidy modTidyReport
	if err := json.Unmarshal(stdout.Bytes(), &tidy); err != nil {
		t.Fatalf("stdout is not JSON tidy report: %v; stdout = %q", err, stdout.String())
	}
	if tidy.OK || !containsString(tidy.Missing, "example.com/lib/net") {
		t.Fatalf("tidy = %+v, want missing external require", tidy)
	}
}

func TestModVerifyReportsMissingExternalRequire(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.gs"), []byte(`lib := require("example.com/lib/net")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte("module example.com/demo\ngs 0.1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("mod verify code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if verify.OK || len(verify.Diagnostics) == 0 || verify.Diagnostics[0].Code != "GS9106" {
		t.Fatalf("verify = %+v, want missing require diagnostic", verify)
	}
}

func TestModVerifyChecksLocalCollectionsAndReplaces(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.gs"), []byte(`vendored := require("vendor:foo")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte(`module example.com/demo
gs 0.1
require example.com/lib v0.1.0
replace example.com/lib => ./missing-lib
collection vendor ./missing-vendor
`), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("mod verify code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	count := 0
	for _, diag := range verify.Diagnostics {
		if diag.Code == "GS9107" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("verify diagnostics = %+v, want two local path diagnostics", verify.Diagnostics)
	}
}

func TestModLockWritesSumAndVerifyDetectsLocalMutation(t *testing.T) {
	dir := t.TempDir()
	vendorDir := filepath.Join(dir, "vendor", "pkg")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	vendorFile := filepath.Join(vendorDir, "util.gs")
	if err := os.WriteFile(vendorFile, []byte(`func value() { return 1 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.gs"), []byte(`util := require("vendor:pkg.util")`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte(`module example.com/demo
gs 0.1
collection vendor ./vendor
`), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"lock", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod lock code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	sumPath := filepath.Join(dir, "gscript.sum")
	if _, err := os.Stat(sumPath); err != nil {
		t.Fatalf("expected gscript.sum: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod verify after lock code = %d, stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	if err := os.WriteFile(vendorFile, []byte(`func value() { return 2 }`), 0644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("mod verify after mutation code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if verify.OK {
		t.Fatalf("verify = %+v, want checksum failure", verify)
	}
	found := false
	for _, diag := range verify.Diagnostics {
		if diag.Code == "GS9109" && strings.Contains(diag.Message, "checksum mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("verify diagnostics = %+v, want checksum mismatch", verify.Diagnostics)
	}
}

func TestModExplainReportsResolvedModuleKind(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "local", "lib"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte(`module example.com/demo
gs 0.1
require example.com/lib v0.1.0
replace example.com/lib => ./local/lib
`), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"explain", "--json", "--dir", dir, "example.com/lib/foo"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod explain code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var explain modExplainReport
	if err := json.Unmarshal(stdout.Bytes(), &explain); err != nil {
		t.Fatalf("stdout is not JSON explain report: %v; stdout = %q", err, stdout.String())
	}
	if !explain.OK || explain.Kind != "replace" || explain.Path != "example.com/lib" {
		t.Fatalf("explain = %+v, want replace resolution", explain)
	}
	if !strings.HasSuffix(explain.File, filepath.Join("local", "lib", "foo.gs")) {
		t.Fatalf("explain file = %q, want local replace target", explain.File)
	}
}
