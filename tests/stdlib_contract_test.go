package tests_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/catalog"
)

func TestStdlibContractDocumentsCatalogModules(t *testing.T) {
	root := findRepoRoot(t)
	catalogModules := readStdlibModuleNames(t)
	contractRows := readStdlibContractRows(t, root)

	for _, name := range catalogModules {
		if !contractRows[name] {
			t.Fatalf("docs/reference/stdlib/index.md missing catalog stdlib module %q", name)
		}
	}
}

func readStdlibModuleNames(t *testing.T) []string {
	t.Helper()

	seen := map[string]bool{}
	names := catalog.ModuleNames()
	for _, name := range names {
		if seen[name] {
			t.Fatalf("stdlib catalog has duplicate module %q", name)
		}
		seen[name] = true
	}
	if len(names) == 0 {
		t.Fatal("stdlib catalog contains no module names")
	}
	sort.Strings(names)
	return names
}

func readStdlibContractRows(t *testing.T, root string) map[string]bool {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "docs", "reference", "stdlib", "index.md"))
	if err != nil {
		t.Fatalf("read stdlib contract: %v", err)
	}

	rowRE := regexp.MustCompile("^\\|\\s*`([^`]+)`\\s*\\|\\s*`([^`]+)`\\s*\\|")
	rows := map[string]bool{}
	for lineNo, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "## Default Imports" {
			break
		}
		match := rowRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		columns := strings.Split(line, "|")
		if len(columns) != 7 {
			t.Fatalf("docs/reference/stdlib/index.md:%d row for %q has %d table columns, want 5", lineNo+1, match[1], len(columns)-2)
		}
		for i := 1; i <= 5; i++ {
			if strings.TrimSpace(columns[i]) == "" {
				t.Fatalf("docs/reference/stdlib/index.md:%d row for %q has empty contract field", lineNo+1, match[2])
			}
		}
		name := match[2]
		if rows[name] {
			t.Fatalf("docs/reference/stdlib/index.md:%d duplicate module row %q", lineNo+1, name)
		}
		rows[name] = true
	}
	if len(rows) == 0 {
		t.Fatal("docs/reference/stdlib/index.md contains no machine-checkable module rows")
	}
	return rows
}
