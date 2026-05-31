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

func TestGScriptPackageBoundary(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	cmd := exec.Command("go", "list", "-json", ".", "./cmd/gscript")
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

	publicPkg := pkgs["github.com/never-labs/gscript"]
	if publicPkg.ImportPath == "" {
		t.Fatalf("go list did not include public gscript package; got packages %v", packageKeys(pkgs))
	}
	cliPkg := pkgs["github.com/never-labs/gscript/cmd/gscript"]
	if cliPkg.ImportPath == "" {
		t.Fatalf("go list did not include cmd/gscript package; got packages %v", packageKeys(pkgs))
	}

	for _, dep := range append(publicPkg.Imports, publicPkg.Deps...) {
		if strings.Contains(dep, "/cmd/gscript") {
			t.Fatalf("public gscript package must not depend on CLI package, found %q", dep)
		}
	}
	for _, cliOnlyImport := range []string{"flag", "runtime/debug", "runtime/pprof", "text/tabwriter"} {
		if slices.Contains(publicPkg.Imports, cliOnlyImport) {
			t.Fatalf("public gscript package imports CLI-only package %q", cliOnlyImport)
		}
	}
	if !slices.Contains(cliPkg.Imports, "github.com/never-labs/gscript") {
		t.Fatalf("cmd/gscript should depend on the public embedding API")
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
