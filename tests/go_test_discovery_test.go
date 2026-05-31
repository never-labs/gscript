package tests_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGoTestAllPackagesIncludesRestructuredTestDirs(t *testing.T) {
	root := findRepoRoot(t)
	modulePath := readModulePath(t, root)

	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list ./...: %v\n%s", err, out)
	}

	packages := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			packages[line] = true
		}
	}

	for _, suffix := range []string{
		"tests/llm",
		"tests/integration/llm",
		"tests/sdk",
	} {
		importPath := modulePath + "/" + suffix
		if !packages[importPath] {
			t.Fatalf("go test ./... package discovery missed %s", importPath)
		}
	}
}
