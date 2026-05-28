//go:build darwin && arm64

package methodjit

import "go/ast"

func emitterMethodCalls(t testFataler, filename, funcName string) map[string]bool {
	t.Helper()
	file := parseEmitterFile(t, filename)
	calls := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if ok {
				calls[sel.Sel.Name] = true
			}
			return true
		})
		return calls
	}

	t.Fatalf("%s not found in %s", funcName, filename)
	return nil
}
