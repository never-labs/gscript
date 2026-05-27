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
