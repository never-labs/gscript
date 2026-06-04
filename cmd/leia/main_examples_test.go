package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/modpkg"
	"github.com/never-labs/leia/internal/stdlib/catalog"
)

func TestRunCommandExampleDialects(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "examples", "hello", "dialects.leia")

	var stdout, stderr bytes.Buffer
	code := runRunCommand([]string{"--vm", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runRunCommand code = %d, stderr = %q", code, stderr.String())
	}
}

func TestRunCommandDialectExamples(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"text_parsing.leia",
		"web_text.leia",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "examples", "dialects", name)

			var stdout, stderr bytes.Buffer
			code := runRunCommand([]string{"--vm", path}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("runRunCommand code = %d, stderr = %q", code, stderr.String())
			}
		})
	}
}

func TestPackageManagedUIExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "ui", "package_managed")

	for _, forbidden := range []string{"ui", "dom", "widget", "widgets", "canvas"} {
		if _, ok := catalog.Module(forbidden); ok {
			t.Fatalf("stdlib catalog contains UI-shaped module %q; UI runtimes should stay package-managed", forbidden)
		}
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
	if !verify.OK {
		t.Fatalf("verify = %+v, want package-managed UI example manifest to verify", verify)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"list", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod list code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var list modListReport
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("stdout is not JSON list report: %v; stdout = %q", err, stdout.String())
	}
	if !list.OK || !containsResolvedRequire(list.Requires, "github.com/never-labs/leia-ui/raylib", "v0.1.0") {
		t.Fatalf("list = %+v, want external Leia UI runtime requirement", list)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"gomod", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod gomod code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if got := stdout.String(); !strings.Contains(got, "github.com/gen2brain/raylib-go/raylib v0.55.1") {
		t.Fatalf("generated go.mod = %q, want native UI adapter dependency", got)
	}
}

func containsResolvedRequire(reqs []modpkg.ListRequire, path, version string) bool {
	for _, req := range reqs {
		if req.Path == path && req.Version == version {
			return true
		}
	}
	return false
}
