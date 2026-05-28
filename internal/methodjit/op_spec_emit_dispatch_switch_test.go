//go:build darwin && arm64

package methodjit

import (
	"go/ast"
	"go/parser"
	"go/token"
)

type testFataler interface {
	Helper()
	Fatalf(format string, args ...any)
}

func emitInstrSwitchHandledOps(t testFataler) map[Op]bool {
	t.Helper()
	return emitterSwitchHandledOps(t, "emit_dispatch.go", "emitInstr")
}

func emitterSwitchHandledOps(t testFataler, filename, funcName string) map[Op]bool {
	t.Helper()
	file := parseEmitterFile(t, filename)
	ops := make(map[Op]bool)
	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}
		found = true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			for _, stmt := range sw.Body.List {
				cc := stmt.(*ast.CaseClause)
				for _, expr := range cc.List {
					if ident, ok := expr.(*ast.Ident); ok {
						if op, ok := opByName(ident.Name); ok {
							ops[op] = true
							continue
						}
						t.Fatalf("%s has unknown op case %s", funcName, ident.Name)
					}
				}
			}
			return false
		})
	}
	if !found {
		t.Fatalf("%s not found in %s", funcName, filename)
	}
	return ops
}

func parseEmitterFile(t testFataler, filename string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	return file
}
