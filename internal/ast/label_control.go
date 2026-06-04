package ast

import "fmt"

// ValidateLabelControl checks Go-style label/goto control flow for each
// function independently. Labels are function-local and unique. A goto may jump
// backward or forward, but it may not enter a deeper block scope or skip a
// declaration in the same block.
func ValidateLabelControl(prog *Program) error {
	if prog == nil {
		return nil
	}
	return validateLabelStmtList(prog.Stmts)
}

type labelInfo struct {
	name      string
	pos       Pos
	block     *BlockStmt
	stmtIndex int
	depth     int
}

type gotoInfo struct {
	name      string
	pos       Pos
	block     *BlockStmt
	stmtIndex int
	depth     int
}

type labelScope struct {
	labels map[string]labelInfo
	gotos  []gotoInfo
}

func validateLabelStmtList(stmts []Stmt) error {
	root := &BlockStmt{P: Pos{Line: 1, Column: 1}, Stmts: stmts}
	scope := &labelScope{labels: make(map[string]labelInfo)}
	if err := scope.walkBlock(root, 0); err != nil {
		return err
	}
	for _, g := range scope.gotos {
		lbl, ok := scope.labels[g.name]
		if !ok {
			return fmt.Errorf("line %d: goto %q target not found", g.pos.Line, g.name)
		}
		if lbl.depth > g.depth {
			return fmt.Errorf("line %d: goto %q jumps into a deeper block scope", g.pos.Line, g.name)
		}
		if lbl.block == g.block && lbl.stmtIndex > g.stmtIndex {
			for i := g.stmtIndex + 1; i < lbl.stmtIndex; i++ {
				if stmtIntroducesLocal(g.block.Stmts[i]) {
					return fmt.Errorf("line %d: goto %q jumps over a local declaration", g.pos.Line, g.name)
				}
			}
		}
	}
	return nil
}

func (s *labelScope) walkBlock(block *BlockStmt, depth int) error {
	if block == nil {
		return nil
	}
	for i, stmt := range block.Stmts {
		switch st := stmt.(type) {
		case *LabelStmt:
			if prev, ok := s.labels[st.Name]; ok {
				return fmt.Errorf("line %d: duplicate label %q, first defined at line %d", st.P.Line, st.Name, prev.pos.Line)
			}
			s.labels[st.Name] = labelInfo{name: st.Name, pos: st.P, block: block, stmtIndex: i, depth: depth}
		case *GotoStmt:
			s.gotos = append(s.gotos, gotoInfo{name: st.Name, pos: st.P, block: block, stmtIndex: i, depth: depth})
		case *BlockStmt:
			if err := s.walkBlock(st, depth+1); err != nil {
				return err
			}
		case *IfStmt:
			if err := s.walkBlock(st.Body, depth+1); err != nil {
				return err
			}
			for _, ei := range st.ElseIfs {
				if err := s.walkBlock(ei.Body, depth+1); err != nil {
					return err
				}
			}
			if err := s.walkBlock(st.ElseBody, depth+1); err != nil {
				return err
			}
		case *ForStmt:
			if err := s.walkBlock(st.Body, depth+1); err != nil {
				return err
			}
		case *ForNumStmt:
			if err := s.walkBlock(st.Body, depth+1); err != nil {
				return err
			}
		case *ForRangeStmt:
			if err := s.walkBlock(st.Body, depth+1); err != nil {
				return err
			}
		case *FuncDeclStmt:
			if err := validateLabelStmtList(st.Body.Stmts); err != nil {
				return err
			}
		default:
			if err := s.walkStmtExprs(stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *labelScope) walkStmtExprs(stmt Stmt) error {
	switch st := stmt.(type) {
	case *DeclareStmt:
		for _, expr := range st.Values {
			if err := s.walkExpr(expr); err != nil {
				return err
			}
		}
	case *AssignStmt:
		for _, expr := range st.Values {
			if err := s.walkExpr(expr); err != nil {
				return err
			}
		}
	case *CallStmt:
		return s.walkExpr(st.Call)
	case *ReturnStmt:
		for _, expr := range st.Values {
			if err := s.walkExpr(expr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *labelScope) walkExpr(expr Expr) error {
	switch e := expr.(type) {
	case *FuncLitExpr:
		return validateLabelStmtList(e.Body.Stmts)
	case *BinaryExpr:
		if err := s.walkExpr(e.Left); err != nil {
			return err
		}
		return s.walkExpr(e.Right)
	case *UnaryExpr:
		return s.walkExpr(e.Operand)
	case *ParenExpr:
		return s.walkExpr(e.Inner)
	case *IndexExpr:
		if err := s.walkExpr(e.Table); err != nil {
			return err
		}
		return s.walkExpr(e.Index)
	case *FieldExpr:
		return s.walkExpr(e.Table)
	case *CallExpr:
		if err := s.walkExpr(e.Func); err != nil {
			return err
		}
		for _, arg := range e.Args {
			if err := s.walkExpr(arg); err != nil {
				return err
			}
		}
	case *MethodCallExpr:
		if err := s.walkExpr(e.Object); err != nil {
			return err
		}
		for _, arg := range e.Args {
			if err := s.walkExpr(arg); err != nil {
				return err
			}
		}
	case *TableLitExpr:
		for _, f := range e.Fields {
			if err := s.walkExpr(f.Key); err != nil {
				return err
			}
			if err := s.walkExpr(f.Value); err != nil {
				return err
			}
		}
	case *DenseLitExpr:
		for _, value := range e.Values {
			if err := s.walkExpr(value); err != nil {
				return err
			}
		}
	}
	return nil
}

func stmtIntroducesLocal(stmt Stmt) bool {
	switch stmt.(type) {
	case *DeclareStmt, *FuncDeclStmt, *ForNumStmt, *ForRangeStmt:
		return true
	default:
		return false
	}
}
