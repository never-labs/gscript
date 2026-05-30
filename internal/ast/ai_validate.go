package ast

import "fmt"

// ValidateAINative checks source-level invariants for AI-native declarations
// before they are lowered to ordinary stdlib calls.
func ValidateAINative(prog *Program) error {
	if prog == nil {
		return nil
	}
	seenDefaults := false
	for _, stmt := range prog.Stmts {
		switch s := stmt.(type) {
		case *AgentDefaultsDeclStmt:
			if seenDefaults {
				return fmt.Errorf("line %d: duplicate agent defaults declaration", s.P.Line)
			}
			seenDefaults = true
		case *ModelsDeclStmt:
			if err := validateModelsDecl(s); err != nil {
				return err
			}
		}
		if err := validateAINativeStmt(stmt, true); err != nil {
			return err
		}
	}
	return nil
}

func validateAINativeStmt(stmt Stmt, topLevel bool) error {
	if stmt == nil {
		return nil
	}
	switch s := stmt.(type) {
	case *AgentDefaultsDeclStmt:
		if !topLevel {
			return fmt.Errorf("line %d: agent defaults must be declared at module scope", s.P.Line)
		}
	case *ModelsDeclStmt:
		if !topLevel {
			return fmt.Errorf("line %d: models must be declared at module scope", s.P.Line)
		}
	case *BlockStmt:
		return validateAINativeStmtList(s.Stmts, false)
	case *FuncDeclStmt:
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *ToolDeclStmt:
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *AgentDeclStmt:
		if s.Flow != nil {
			return validateAINativeStmtList(s.Flow.Stmts, false)
		}
	case *BudgetStmt:
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *IfStmt:
		if err := validateAINativeStmtList(s.Body.Stmts, false); err != nil {
			return err
		}
		for _, ei := range s.ElseIfs {
			if err := validateAINativeStmtList(ei.Body.Stmts, false); err != nil {
				return err
			}
		}
		if s.ElseBody != nil {
			return validateAINativeStmtList(s.ElseBody.Stmts, false)
		}
	case *ForStmt:
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *ForNumStmt:
		if err := validateAINativeStmt(s.Init, false); err != nil {
			return err
		}
		if err := validateAINativeStmt(s.Post, false); err != nil {
			return err
		}
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *ForRangeStmt:
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *SelectStmt:
		for _, c := range s.Cases {
			if err := validateAINativeStmtList(c.Body.Stmts, false); err != nil {
				return err
			}
		}
		if s.Default != nil {
			return validateAINativeStmtList(s.Default.Stmts, false)
		}
	}
	return nil
}

func validateAINativeStmtList(stmts []Stmt, topLevel bool) error {
	for _, stmt := range stmts {
		if err := validateAINativeStmt(stmt, topLevel); err != nil {
			return err
		}
	}
	return nil
}

func validateModelsDecl(s *ModelsDeclStmt) error {
	aliases := map[string]string{}
	for _, f := range s.Config {
		key, ok := stringConfigKey(f.Key)
		if !ok {
			continue
		}
		if v, ok := f.Value.(*StringLit); ok {
			aliases[key] = v.Value
		}
		if tbl, ok := f.Value.(*TableLitExpr); ok {
			if err := validateModelConfigTable(tbl); err != nil {
				return fmt.Errorf("line %d: %w", f.P.Line, err)
			}
		}
	}
	for name := range aliases {
		seen := map[string]bool{}
		for cur := name; ; {
			next, ok := aliases[cur]
			if !ok {
				break
			}
			if seen[cur] {
				return fmt.Errorf("line %d: model alias cycle involving %q", s.P.Line, cur)
			}
			seen[cur] = true
			cur = next
		}
	}
	return nil
}

func validateModelConfigTable(tbl *TableLitExpr) error {
	for _, f := range tbl.Fields {
		key, ok := stringConfigKey(f.Key)
		if !ok {
			continue
		}
		if key == "api_key" {
			if _, ok := f.Value.(*StringLit); ok {
				return fmt.Errorf("model api_key must not be a string literal")
			}
		}
		if nested, ok := f.Value.(*TableLitExpr); ok {
			if err := validateModelConfigTable(nested); err != nil {
				return err
			}
		}
	}
	return nil
}
