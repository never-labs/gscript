package ast

import (
	"fmt"
	"strings"
)

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
			if err := validateAIToolsConfig(s.P, "agent defaults", s.Config); err != nil {
				return err
			}
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
	case *DeclareStmt:
		return validateAINativeExprList(s.Values)
	case *AssignStmt:
		if err := validateAINativeExprList(s.Targets); err != nil {
			return err
		}
		return validateAINativeExprList(s.Values)
	case *CompoundAssignStmt:
		if err := validateAINativeExpr(s.Target); err != nil {
			return err
		}
		return validateAINativeExpr(s.Value)
	case *IncDecStmt:
		return validateAINativeExpr(s.Target)
	case *CallStmt:
		return validateAINativeExpr(s.Call)
	case *GoStmt:
		return validateAINativeExpr(s.Call)
	case *DeferStmt:
		return validateAINativeExpr(s.Call)
	case *SendStmt:
		if err := validateAINativeExpr(s.Channel); err != nil {
			return err
		}
		return validateAINativeExpr(s.Value)
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
		if err := validateToolDecl(s); err != nil {
			return err
		}
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *AgentDeclStmt:
		if err := validateAIToolsConfig(s.P, fmt.Sprintf("agent %s", s.Name), s.Config); err != nil {
			return err
		}
		if s.Flow != nil {
			return validateAINativeStmtList(s.Flow.Stmts, false)
		}
	case *BudgetStmt:
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *IfStmt:
		if err := validateAINativeExpr(s.Cond); err != nil {
			return err
		}
		if err := validateAINativeStmtList(s.Body.Stmts, false); err != nil {
			return err
		}
		for _, ei := range s.ElseIfs {
			if err := validateAINativeExpr(ei.Cond); err != nil {
				return err
			}
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
		if err := validateAINativeExpr(s.Cond); err != nil {
			return err
		}
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *ForNumStmt:
		if err := validateAINativeStmt(s.Init, false); err != nil {
			return err
		}
		if err := validateAINativeExpr(s.Cond); err != nil {
			return err
		}
		if err := validateAINativeStmt(s.Post, false); err != nil {
			return err
		}
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *ForRangeStmt:
		if err := validateAINativeExpr(s.Iter); err != nil {
			return err
		}
		return validateAINativeStmtList(s.Body.Stmts, false)
	case *ReturnStmt:
		return validateAINativeExprList(s.Values)
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

func validateAINativeExprList(exprs []Expr) error {
	for _, expr := range exprs {
		if err := validateAINativeExpr(expr); err != nil {
			return err
		}
	}
	return nil
}

func validateAINativeExpr(expr Expr) error {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *BinaryExpr:
		if err := validateAINativeExpr(e.Left); err != nil {
			return err
		}
		return validateAINativeExpr(e.Right)
	case *UnaryExpr:
		return validateAINativeExpr(e.Operand)
	case *ParenExpr:
		return validateAINativeExpr(e.Inner)
	case *IndexExpr:
		if err := validateAINativeExpr(e.Table); err != nil {
			return err
		}
		return validateAINativeExpr(e.Index)
	case *FieldExpr:
		return validateAINativeExpr(e.Table)
	case *CallExpr:
		if err := validateAINativeExpr(e.Func); err != nil {
			return err
		}
		return validateAINativeExprList(e.Args)
	case *MethodCallExpr:
		if err := validateAINativeExpr(e.Object); err != nil {
			return err
		}
		return validateAINativeExprList(e.Args)
	case *FuncLitExpr:
		return validateAINativeStmtList(e.Body.Stmts, false)
	case *AgentLitExpr:
		if err := validateAIToolsConfig(e.P, "agent expression", e.Config); err != nil {
			return err
		}
		if e.Flow != nil {
			return validateAINativeStmtList(e.Flow.Stmts, false)
		}
	case *TurnExpr:
		return validateAIToolsConfig(e.P, "turn", e.Config)
	case *MessagesExpr:
		return validateAIConfigExprs(e.Fields)
	case *ListLitExpr:
		return validateAINativeExprList(e.Values)
	case *TableLitExpr:
		for _, f := range e.Fields {
			if err := validateAINativeExpr(f.Key); err != nil {
				return err
			}
			if err := validateAINativeExpr(f.Value); err != nil {
				return err
			}
		}
	case *DenseLitExpr:
		return validateAINativeExprList(e.Values)
	case *RecvExpr:
		return validateAINativeExpr(e.Channel)
	case *MakeChanExpr:
		return validateAINativeExpr(e.Size)
	}
	return nil
}

func validateAIConfigExprs(config []ConfigField) error {
	for _, f := range config {
		if err := validateAINativeExpr(f.Key); err != nil {
			return err
		}
		if err := validateAINativeExpr(f.Value); err != nil {
			return err
		}
	}
	return nil
}

func validateToolDecl(s *ToolDeclStmt) error {
	if len(s.Requires) == 0 {
		return fmt.Errorf("line %d: tool %s missing gscript:requires directive", s.P.Line, s.Name)
	}
	if err := validateToolRequires(s); err != nil {
		return err
	}
	return validateToolParamDocs(s)
}

func validateToolRequires(s *ToolDeclStmt) error {
	seen := map[string]bool{}
	for _, req := range s.Requires {
		if !isValidCapabilityName(req) {
			return fmt.Errorf("line %d: tool %s has invalid gscript:requires capability %q", s.P.Line, s.Name, req)
		}
		if seen[req] {
			return fmt.Errorf("line %d: tool %s has duplicate gscript:requires capability %q", s.P.Line, s.Name, req)
		}
		seen[req] = true
	}
	if len(s.Requires) > 1 && seen["none"] {
		return fmt.Errorf("line %d: tool %s uses gscript:requires none with other capabilities", s.P.Line, s.Name)
	}
	return nil
}

func isValidCapabilityName(req string) bool {
	if req == "none" || req == "all" {
		return true
	}
	if req == "" || strings.HasPrefix(req, ".") || strings.HasSuffix(req, ".") || strings.Contains(req, "..") {
		return false
	}
	for _, part := range strings.Split(req, ".") {
		if part == "" || !isCapabilityStart(part[0]) {
			return false
		}
		for i := 1; i < len(part); i++ {
			if !isCapabilityPart(part[i]) {
				return false
			}
		}
	}
	return true
}

func isCapabilityStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isCapabilityPart(b byte) bool {
	return isCapabilityStart(b) || (b >= '0' && b <= '9')
}

func validateToolParamDocs(s *ToolDeclStmt) error {
	params := map[string]bool{}
	for _, p := range s.Params {
		if p.Name != "" && p.Name != "..." {
			params[p.Name] = true
		}
	}
	seen := map[string]bool{}
	for _, d := range s.ParamDocEntries {
		if !params[d.Name] {
			return fmt.Errorf("line %d: tool %s has gscript:param for unknown parameter %q", s.P.Line, s.Name, d.Name)
		}
		if seen[d.Name] {
			return fmt.Errorf("line %d: tool %s has duplicate gscript:param for %q", s.P.Line, s.Name, d.Name)
		}
		seen[d.Name] = true
	}
	return nil
}

func validateAIToolsConfig(pos Pos, owner string, config []ConfigField) error {
	for _, f := range config {
		key, ok := stringConfigKey(f.Key)
		if !ok || key != "tools" {
			continue
		}
		list, ok := f.Value.(*ListLitExpr)
		if !ok {
			continue
		}
		seen := map[string]bool{}
		for _, v := range list.Values {
			ident, ok := v.(*IdentExpr)
			if !ok {
				continue
			}
			if seen[ident.Name] {
				return fmt.Errorf("line %d: %s tools list includes duplicate tool %q", pos.Line, owner, ident.Name)
			}
			seen[ident.Name] = true
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
