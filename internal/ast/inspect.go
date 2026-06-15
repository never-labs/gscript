package ast

import "reflect"

// Inspect traverses an AST in depth-first order. Returning false from fn skips
// that node's children.
func Inspect(node Node, fn func(Node) bool) {
	if isNilNode(node) || !fn(node) {
		return
	}
	switch n := node.(type) {
	case *Program:
		for _, stmt := range n.Stmts {
			Inspect(stmt, fn)
		}
	case *BlockStmt:
		for _, stmt := range n.Stmts {
			Inspect(stmt, fn)
		}
	case *AssignStmt:
		inspectExprs(n.Targets, fn)
		inspectExprs(n.Values, fn)
	case *DeclareStmt:
		inspectExprs(n.Values, fn)
	case *CompoundAssignStmt:
		Inspect(n.Target, fn)
		Inspect(n.Value, fn)
	case *IncDecStmt:
		Inspect(n.Target, fn)
	case *CallStmt:
		Inspect(n.Call, fn)
	case *GoStmt:
		Inspect(n.Call, fn)
	case *DeferStmt:
		Inspect(n.Call, fn)
	case *SendStmt:
		Inspect(n.Channel, fn)
		Inspect(n.Value, fn)
	case *SelectStmt:
		for _, c := range n.Cases {
			Inspect(c.Channel, fn)
			Inspect(c.SendValue, fn)
			Inspect(c.Body, fn)
		}
		Inspect(n.Default, fn)
	case *IfStmt:
		Inspect(n.Cond, fn)
		Inspect(n.Body, fn)
		for _, elseif := range n.ElseIfs {
			Inspect(elseif.Cond, fn)
			Inspect(elseif.Body, fn)
		}
		Inspect(n.ElseBody, fn)
	case *ForNumStmt:
		Inspect(n.Init, fn)
		Inspect(n.Cond, fn)
		Inspect(n.Post, fn)
		Inspect(n.Body, fn)
	case *ForRangeStmt:
		Inspect(n.Iter, fn)
		Inspect(n.Body, fn)
	case *ForStmt:
		Inspect(n.Cond, fn)
		Inspect(n.Body, fn)
	case *ReturnStmt:
		inspectExprs(n.Values, fn)
	case *EvaluateStmt:
		Inspect(n.Body, fn)
	case *FuncDeclStmt:
		Inspect(n.Body, fn)
	case *BinaryExpr:
		Inspect(n.Left, fn)
		Inspect(n.Right, fn)
	case *UnaryExpr:
		Inspect(n.Operand, fn)
	case *ParenExpr:
		Inspect(n.Inner, fn)
	case *IndexExpr:
		Inspect(n.Table, fn)
		Inspect(n.Index, fn)
	case *FieldExpr:
		Inspect(n.Table, fn)
	case *CallExpr:
		Inspect(n.Func, fn)
		inspectExprs(n.Args, fn)
	case *MethodCallExpr:
		Inspect(n.Object, fn)
		inspectExprs(n.Args, fn)
	case *FuncLitExpr:
		Inspect(n.Body, fn)
	case *ListLitExpr:
		inspectExprs(n.Values, fn)
	case *TableLitExpr:
		for _, field := range n.Fields {
			Inspect(field.Key, fn)
			Inspect(field.Value, fn)
		}
	case *DenseLitExpr:
		inspectExprs(n.Values, fn)
	case *RecvExpr:
		Inspect(n.Channel, fn)
	case *MakeChanExpr:
		Inspect(n.Size, fn)
	case *InterpolatedStringExpr:
		for _, part := range n.Parts {
			Inspect(part.Expr, fn)
		}
	case *TaggedStringExpr:
		Inspect(n.Body, fn)
	case *TaggedBlockExpr:
		for _, field := range n.Config {
			Inspect(field.Key, fn)
			Inspect(field.Value, fn)
		}
		Inspect(n.Body, fn)
		Inspect(n.RawSourceExpr, fn)
	}
}

func inspectExprs(exprs []Expr, fn func(Node) bool) {
	for _, expr := range exprs {
		Inspect(expr, fn)
	}
}

func isNilNode(node Node) bool {
	if node == nil {
		return true
	}
	value := reflect.ValueOf(node)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
