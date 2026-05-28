//go:build darwin && arm64

package methodjit

import "go/ast"

func emitInstrOpsCalling(t testFataler, method string) map[Op]bool {
	t.Helper()
	return emitInstrOpsWithCaseBehavior(t, func(cc *ast.CaseClause) bool {
		return caseCallsSelector(cc, method)
	})
}

func emitInstrOpsInvalidatingShape(t testFataler) map[Op]bool {
	t.Helper()
	return emitInstrOpsWithCaseBehavior(t, caseInvalidatesShape)
}

func emitterOpsInvalidatingShape(t testFataler, filename, funcName string) map[Op]bool {
	t.Helper()
	return emitterOpsWithCaseBehavior(t, filename, funcName, caseInvalidatesShape)
}

func emitInstrOpsWithCaseBehavior(t testFataler, hasBehavior func(*ast.CaseClause) bool) map[Op]bool {
	t.Helper()
	return emitterOpsWithCaseBehavior(t, "emit_dispatch.go", "emitInstr", hasBehavior)
}

func emitterOpsCalling(t testFataler, filename, funcName, method string) map[Op]bool {
	t.Helper()
	return emitterOpsWithCaseBehavior(t, filename, funcName, func(cc *ast.CaseClause) bool {
		return caseCallsSelector(cc, method)
	})
}

func emitterOpsWithCaseBehavior(t testFataler, filename, funcName string, hasBehavior func(*ast.CaseClause) bool) map[Op]bool {
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
				if !hasBehavior(cc) {
					continue
				}
				for _, expr := range cc.List {
					if ident, ok := expr.(*ast.Ident); ok {
						if op, ok := opByName(ident.Name); ok {
							ops[op] = true
						}
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

func caseCallsSelector(cc *ast.CaseClause, method string) bool {
	found := false
	ast.Inspect(cc, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == method {
			found = true
			return false
		}
		return true
	})
	return found
}

func caseInvalidatesShape(cc *ast.CaseClause) bool {
	found := false
	ast.Inspect(cc, func(n ast.Node) bool {
		if found {
			return false
		}
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assign.Lhs {
			sel, ok := lhs.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "shapeVerified" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
