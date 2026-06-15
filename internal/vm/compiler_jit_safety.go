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

func blockHasLoopReturnTableLiteral(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Stmts {
		if stmtHasLoopReturnTableLiteral(stmt, false) {
			return true
		}
	}
	return false
}

func stmtHasLoopReturnTableLiteral(stmt ast.Stmt, inLoop bool) bool {
	switch s := stmt.(type) {
	case nil:
		return false
	case *ast.ReturnStmt:
		if !inLoop {
			return false
		}
		for _, value := range s.Values {
			if exprContainsTableLiteral(value) {
				return true
			}
		}
	case *ast.BlockStmt:
		return blockHasLoopReturnTableLiteralWithState(s, inLoop)
	case *ast.IfStmt:
		if blockHasLoopReturnTableLiteralWithState(s.Body, inLoop) {
			return true
		}
		for _, elseIf := range s.ElseIfs {
			if blockHasLoopReturnTableLiteralWithState(elseIf.Body, inLoop) {
				return true
			}
		}
		return blockHasLoopReturnTableLiteralWithState(s.ElseBody, inLoop)
	case *ast.ForNumStmt:
		return blockHasLoopReturnTableLiteralWithState(s.Body, true)
	case *ast.ForRangeStmt:
		return blockHasLoopReturnTableLiteralWithState(s.Body, true)
	case *ast.ForStmt:
		return blockHasLoopReturnTableLiteralWithState(s.Body, true)
	case *ast.SelectStmt:
		for _, selCase := range s.Cases {
			if blockHasLoopReturnTableLiteralWithState(selCase.Body, inLoop) {
				return true
			}
		}
	case *ast.FuncDeclStmt:
		return false
	}
	return false
}

func blockHasLoopReturnTableLiteralWithState(block *ast.BlockStmt, inLoop bool) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Stmts {
		if stmtHasLoopReturnTableLiteral(stmt, inLoop) {
			return true
		}
	}
	return false
}

func exprContainsTableLiteral(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if found {
			return false
		}
		if _, ok := node.(*ast.FuncLitExpr); ok {
			return false
		}
		if _, ok := node.(*ast.TableLitExpr); ok {
			found = true
			return false
		}
		return true
	})
	return found
}
