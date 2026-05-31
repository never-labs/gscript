package tests_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestInternalPackageBoundaries(t *testing.T) {
	root := findRepoRoot(t)
	cmd := exec.Command("go", "list", "-json",
		"./internal/runtime",
		"./internal/stdlib/catalog",
		"./internal/jit",
		"./internal/vm",
		"./internal/methodjit",
	)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list internal packages: %v\n%s", err, out)
	}

	pkgs := map[string]goListPackage{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg goListPackage
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list package: %v", err)
		}
		pkgs[pkg.ImportPath] = pkg
	}

	modulePath := readModulePath(t, root)
	internal := func(suffix string) string {
		return modulePath + "/internal/" + suffix
	}

	assertNoImports(t, pkgs[internal("stdlib/catalog")], internal("runtime"), internal("jit"), internal("vm"), internal("methodjit"))
	assertNoImports(t, pkgs[internal("runtime")], internal("jit"), internal("vm"), internal("methodjit"))
	assertNoImports(t, pkgs[internal("jit")], internal("vm"), internal("methodjit"))
	assertNoImports(t, pkgs[internal("vm")], internal("methodjit"))
}

type goListPackage struct {
	ImportPath string
	Imports    []string
}

func assertNoImports(t *testing.T, pkg goListPackage, forbidden ...string) {
	t.Helper()
	if pkg.ImportPath == "" {
		t.Fatalf("go list did not include package for boundary assertion")
	}
	forbiddenSet := map[string]bool{}
	for _, path := range forbidden {
		forbiddenSet[path] = true
	}
	for _, path := range pkg.Imports {
		if forbiddenSet[path] {
			t.Fatalf("%s must not import %s", pkg.ImportPath, path)
		}
	}
}

func readModulePath(t *testing.T, root string) string {
	t.Helper()
	data := readFileString(t, filepath.Join(root, "go.mod"))
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "module "))
		if path == "" || strings.ContainsFunc(path, unicode.IsSpace) {
			t.Fatalf("invalid module path line in go.mod: %q", line)
		}
		return path
	}
	t.Fatal("go.mod missing module line")
	return ""
}
