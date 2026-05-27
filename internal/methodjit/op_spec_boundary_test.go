package methodjit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpSpecDirectAccessStaysBehindQueryBoundary(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	allowed := map[string]bool{
		"ir_ops.go":          true,
		"op_spec.go":         true,
		"op_spec_queries.go": true,
		"validator.go":       true,
	}
	var offenders []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || allowed[name] ||
			strings.HasPrefix(name, "op_spec_query_") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(src), ".Spec()") {
			offenders = append(offenders, name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan methodjit sources: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("Op.Spec() direct access must go through op_spec_query helpers; offenders: %s", strings.Join(offenders, ", "))
	}
}

func TestOpSpecAggregateFilesStayThin(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	aggregates := []string{
		"op_spec.go",
		"op_spec_policies.go",
		"op_spec_queries.go",
	}
	for _, name := range aggregates {
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if lines := countLines(src); lines > 8 {
			t.Fatalf("%s has %d lines; keep aggregate OpSpec files as thin domain-index shims", name, lines)
		}
	}
}

func TestOpSpecDomainFilesStayFocused(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	limits := map[string]int{
		"op_spec_base.go":     250,
		"op_spec_build.go":    500,
		"op_spec_registry.go": 100,
		"op_spec_types.go":    400,
	}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		limit, ok := limits[name]
		switch {
		case ok:
		case strings.HasPrefix(name, "op_spec_policy_") && strings.HasSuffix(name, ".go"):
			limit = 500
		case strings.HasPrefix(name, "op_spec_query_") && strings.HasSuffix(name, ".go"):
			limit = 400
		default:
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if lines := countLines(src); lines > limit {
			t.Fatalf("%s has %d lines, over focused-file limit %d", name, lines, limit)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan methodjit sources: %v", err)
	}
}

func countLines(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	lines := strings.Count(string(src), "\n")
	if src[len(src)-1] != '\n' {
		lines++
	}
	return lines
}
