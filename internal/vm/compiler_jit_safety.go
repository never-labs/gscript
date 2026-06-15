package vm

import "github.com/never-labs/leia/internal/ast"

func exprContainsShortCircuit(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if found {
			return false
		}
		bin, ok := node.(*ast.BinaryExpr)
		if ok && (bin.Op == "&&" || bin.Op == "||") {
			found = true
			return false
		}
		return true
	})
	return found
}

func exprIsIndexTarget(expr ast.Expr) bool {
	_, ok := expr.(*ast.IndexExpr)
	return ok
}

func blockContainsReturn(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		if found {
			return false
		}
		if _, ok := node.(*ast.ReturnStmt); ok {
			found = true
			return false
		}
		return true
	})
	return found
}
