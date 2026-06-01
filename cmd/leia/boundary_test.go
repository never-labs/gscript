package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLeiaPackageBoundary(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	cmd := exec.Command("go", "list", "-json", ".", "./cmd/leia")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list package boundary: %v\n%s", err, out)
	}

	pkgs := map[string]listedPackage{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg listedPackage
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list package: %v", err)
		}
		pkgs[pkg.ImportPath] = pkg
	}

	publicPkg := pkgs["github.com/never-labs/leia"]
	if publicPkg.ImportPath == "" {
		t.Fatalf("go list did not include public leia package; got packages %v", packageKeys(pkgs))
	}
	cliPkg := pkgs["github.com/never-labs/leia/cmd/leia"]
	if cliPkg.ImportPath == "" {
		t.Fatalf("go list did not include cmd/leia package; got packages %v", packageKeys(pkgs))
	}

	for _, dep := range append(publicPkg.Imports, publicPkg.Deps...) {
		if strings.Contains(dep, "/cmd/leia") {
			t.Fatalf("public leia package must not depend on CLI package, found %q", dep)
		}
	}
	for _, cliOnlyImport := range []string{"flag", "runtime/debug", "runtime/pprof", "text/tabwriter"} {
		if slices.Contains(publicPkg.Imports, cliOnlyImport) {
			t.Fatalf("public leia package imports CLI-only package %q", cliOnlyImport)
		}
	}
	if !slices.Contains(cliPkg.Imports, "github.com/never-labs/leia") {
		t.Fatalf("cmd/leia should depend on the public embedding API")
	}
}

type listedPackage struct {
	ImportPath string
	Imports    []string
	Deps       []string
}

func repoRootForBoundaryTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func packageKeys(pkgs map[string]listedPackage) []string {
	keys := make([]string, 0, len(pkgs))
	for key := range pkgs {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
