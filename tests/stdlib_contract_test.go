package tests_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestStdlibContractDocumentsRuntimeModules(t *testing.T) {
	root := findRepoRoot(t)
	runtimeModules := readRuntimeStdlibModuleNames(t, root)
	contractRows := readStdlibContractRows(t, root)

	for _, name := range runtimeModules {
		if !contractRows[name] {
			t.Fatalf("docs/stdlib-contract.md missing runtime stdlib module %q", name)
		}
	}
}

func readRuntimeStdlibModuleNames(t *testing.T, root string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "internal", "runtime", "stdlib.go"))
	if err != nil {
		t.Fatalf("read runtime stdlib registry: %v", err)
	}

	blockRE := regexp.MustCompile(`(?s)var\s+stdlibModules\s*=\s*\[\]StdlibModuleInfo\s*\{(.*?)\n\}`)
	match := blockRE.FindStringSubmatch(string(data))
	if match == nil {
		t.Fatal("runtime stdlib registry stdlibModules block not found")
	}

	nameRE := regexp.MustCompile(`Name:\s*"([^"]+)"`)
	matches := nameRE.FindAllStringSubmatch(match[1], -1)
	if len(matches) == 0 {
		t.Fatal("runtime stdlib registry contains no module names")
	}

	seen := map[string]bool{}
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		name := match[1]
		if seen[name] {
			t.Fatalf("runtime stdlib registry has duplicate module %q", name)
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func readStdlibContractRows(t *testing.T, root string) map[string]bool {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "docs", "stdlib-contract.md"))
	if err != nil {
		t.Fatalf("read stdlib contract: %v", err)
	}

	rowRE := regexp.MustCompile("^\\|\\s*`([^`]+)`\\s*\\|")
	rows := map[string]bool{}
	for lineNo, line := range strings.Split(string(data), "\n") {
		match := rowRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		columns := strings.Split(line, "|")
		if len(columns) != 7 {
			t.Fatalf("docs/stdlib-contract.md:%d row for %q has %d table columns, want 5", lineNo+1, match[1], len(columns)-2)
		}
		for i := 2; i <= 5; i++ {
			if strings.TrimSpace(columns[i]) == "" {
				t.Fatalf("docs/stdlib-contract.md:%d row for %q has empty contract field", lineNo+1, match[1])
			}
		}
		name := match[1]
		if rows[name] {
			t.Fatalf("docs/stdlib-contract.md:%d duplicate module row %q", lineNo+1, name)
		}
		rows[name] = true
	}
	if len(rows) == 0 {
		t.Fatal("docs/stdlib-contract.md contains no machine-checkable module rows")
	}
	return rows
}
