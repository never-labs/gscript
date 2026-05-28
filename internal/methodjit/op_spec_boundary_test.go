package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
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
		hasDirectSpecCall, err := fileHasDirectSpecCall(path)
		if err != nil {
			return err
		}
		if hasDirectSpecCall {
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

func fileHasDirectSpecCall(path string) (bool, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return false, err
	}
	found := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Spec" {
			found = true
			return false
		}
		return true
	})
	return found, nil
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
		"op_spec_policy_range.go",
		"op_spec_policy_type.go",
		"op_spec_policy_value.go",
		"op_spec_types.go",
		"op_spec_queries.go",
		"op_spec_query_traits.go",
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
		"op_spec_base.go":                       250,
		"op_spec_build.go":                      500,
		"op_spec_emit_dispatch_ast_test.go":     100,
		"op_spec_emit_dispatch_helpers_test.go": 350,
		"op_spec_emit_dispatch_test.go":         200,
		"op_spec_integrity_test.go":             200,
		"op_spec_policy_registry_test.go":       180,
		"op_spec_registry.go":                   100,
		"op_spec_types.go":                      400,
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
			limit = 250
		case strings.HasPrefix(name, "op_spec_query_") && strings.HasSuffix(name, ".go"):
			limit = 250
		case strings.HasPrefix(name, "op_spec_type_") && strings.HasSuffix(name, ".go"):
			limit = 250
		case strings.HasPrefix(name, "op_spec_contract_") && strings.HasSuffix(name, "_test.go"):
			limit = 250
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

func TestOpSpecBuildOnlyOrchestratesPolicyDomains(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	src, err := os.ReadFile(filepath.Join(dir, "op_spec_build.go"))
	if err != nil {
		t.Fatalf("read op_spec_build.go: %v", err)
	}
	text := string(src)
	if strings.Contains(text, "Policies[") {
		t.Fatal("op_spec_build.go should orchestrate domain apply functions, not read policy tables directly")
	}
	required := []string{
		"applyOpSpecBackendPolicies",
		"applyOpSpecFieldPolicies",
		"applyOpSpecValuePolicies",
		"applyOpSpecLICMPolicies",
		"applyOpSpecNumericPolicies",
		"applyOpSpecFieldBarrierPolicies",
		"applyOpSpecRangePolicies",
		"applyOpSpecStringUnrollPolicies",
		"applyOpSpecTableCallPolicies",
		"applyOpSpecTypePolicies",
	}
	last := -1
	for _, name := range required {
		idx := strings.Index(text, name+"(")
		if idx < 0 {
			t.Fatalf("op_spec_build.go does not call %s", name)
		}
		if idx <= last {
			t.Fatalf("op_spec_build.go calls %s out of policy-domain order", name)
		}
		last = idx
	}
}

func TestOpSpecPolicyApplyEntrypointsOnlyOrchestrateSubdomains(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	cases := []struct {
		file string
		fn   string
	}{
		{"op_spec_policy_backend_apply.go", "applyOpSpecBackendPolicies"},
		{"op_spec_policy_field_apply.go", "applyOpSpecFieldBarrierPolicies"},
		{"op_spec_policy_field_apply.go", "applyOpSpecFieldPolicies"},
		{"op_spec_policy_licm_apply.go", "applyOpSpecLICMPolicies"},
		{"op_spec_policy_numeric_apply.go", "applyOpSpecNumericPolicies"},
		{"op_spec_policy_range_apply.go", "applyOpSpecRangePolicies"},
		{"op_spec_policy_string_unroll_apply.go", "applyOpSpecStringUnrollPolicies"},
		{"op_spec_policy_table_call_apply.go", "applyOpSpecTableCallPolicies"},
		{"op_spec_policy_type_apply.go", "applyOpSpecTypePolicies"},
		{"op_spec_policy_value_apply.go", "applyOpSpecValuePolicies"},
	}
	for _, tc := range cases {
		hasPolicyIndex, err := funcHasPolicyTableIndex(filepath.Join(dir, tc.file), tc.fn)
		if err != nil {
			t.Fatalf("scan %s.%s: %v", tc.file, tc.fn, err)
		}
		if hasPolicyIndex {
			t.Fatalf("%s.%s should orchestrate subdomain apply functions, not index policy tables directly", tc.file, tc.fn)
		}
	}
}

func funcHasPolicyTableIndex(path, funcName string) (bool, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return false, err
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}
		found := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if found {
				return false
			}
			index, ok := node.(*ast.IndexExpr)
			if !ok {
				return true
			}
			ident, ok := index.X.(*ast.Ident)
			if ok && strings.HasSuffix(ident.Name, "Policies") {
				found = true
				return false
			}
			return true
		})
		return found, nil
	}
	return false, os.ErrNotExist
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
