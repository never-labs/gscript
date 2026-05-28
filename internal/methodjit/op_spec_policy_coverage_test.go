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

func TestOpSpecPolicyTableIntegrityCoversEveryPolicyVar(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	covered := make(map[string]bool)
	for _, table := range opSpecPolicyTables() {
		if covered[table.name] {
			t.Fatalf("duplicate OpSpec policy table registry entry: %s", table.name)
		}
		covered[table.name] = true
	}
	var missing []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "op_spec_policy_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, ident := range valueSpec.Names {
					if strings.HasPrefix(ident.Name, "op") && strings.HasSuffix(ident.Name, "Policies") && !covered[ident.Name] {
						missing = append(missing, ident.Name)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan OpSpec policy files: %v", err)
	}
	if len(missing) > 0 {
		t.Fatalf("OpSpec policy integrity test does not cover policy vars: %s", strings.Join(missing, ", "))
	}
}
