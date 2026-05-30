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
	ctx := &aiValidationContext{}
	seenDefaults := false
	for _, stmt := range prog.Stmts {
		if tool, ok := stmt.(*ToolDeclStmt); ok {
			if err := ctx.declareTool(tool); err != nil {
				return err
			}
		}
		switch s := stmt.(type) {
		case *AgentDefaultsDeclStmt:
			if seenDefaults {
				return fmt.Errorf("line %d: duplicate agent defaults declaration", s.P.Line)
			}
			seenDefaults = true
			if err := ctx.validateAIToolsConfig(s.P, "agent defaults", s.Config); err != nil {
				return err
			}
		case *ModelsDeclStmt:
			if err := validateModelsDecl(s); err != nil {
				return err
			}
		}
		if err := ctx.validateAINativeStmt(stmt, true); err != nil {
			return err
		}
	}
	return nil
}

type aiValidationContext struct {
	tools AIToolRegistry
}

func (ctx *aiValidationContext) declareTool(tool *ToolDeclStmt) error {
	if tool == nil {
		return nil
	}
	if ctx.tools == nil {
		ctx.tools = AIToolRegistry{}
	}
	if prev, ok := ctx.tools[tool.Name]; ok {
		return fmt.Errorf("line %d: duplicate tool declaration %q, first declared at line %d", tool.P.Line, tool.Name, prev.Source.Line)
	}
	ctx.tools[tool.Name] = AIToolRegistryEntry{
		Name:      tool.Name,
		Requires:  append([]string(nil), tool.Requires...),
		Doc:       tool.DocComment,
		Params:    append([]FuncParam(nil), tool.Params...),
		ParamDocs: cloneStringMap(tool.ParamDocs),
		Source:    tool.P,
	}
	return nil
}

func (ctx *aiValidationContext) child() *aiValidationContext {
	child := &aiValidationContext{tools: AIToolRegistry{}}
	for name, entry := range ctx.tools {
		child.tools[name] = entry
	}
	return child
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (ctx *aiValidationContext) validateAINativeStmt(stmt Stmt, topLevel bool) error {
	if stmt == nil {
		return nil
	}
	switch s := stmt.(type) {
	case *DeclareStmt:
		return ctx.validateAINativeExprList(s.Values)
	case *AssignStmt:
		if err := ctx.validateAINativeExprList(s.Targets); err != nil {
			return err
		}
		return ctx.validateAINativeExprList(s.Values)
	case *CompoundAssignStmt:
		if err := ctx.validateAINativeExpr(s.Target); err != nil {
			return err
		}
		return ctx.validateAINativeExpr(s.Value)
	case *IncDecStmt:
		return ctx.validateAINativeExpr(s.Target)
	case *CallStmt:
		return ctx.validateAINativeExpr(s.Call)
	case *GoStmt:
		return ctx.validateAINativeExpr(s.Call)
	case *DeferStmt:
		return ctx.validateAINativeExpr(s.Call)
	case *SendStmt:
		if err := ctx.validateAINativeExpr(s.Channel); err != nil {
			return err
		}
		return ctx.validateAINativeExpr(s.Value)
	case *AgentDefaultsDeclStmt:
		if !topLevel {
			return fmt.Errorf("line %d: agent defaults must be declared at module scope", s.P.Line)
		}
	case *ModelsDeclStmt:
		if !topLevel {
			return fmt.Errorf("line %d: models must be declared at module scope", s.P.Line)
		}
	case *BlockStmt:
		return ctx.child().validateAINativeStmtList(s.Stmts, false)
	case *FuncDeclStmt:
		return ctx.child().validateAINativeStmtList(s.Body.Stmts, false)
	case *ToolDeclStmt:
		if err := validateToolDecl(s); err != nil {
			return err
		}
		return ctx.validateAINativeStmtList(s.Body.Stmts, false)
	case *AgentDeclStmt:
		if err := ctx.validateAIToolsConfig(s.P, fmt.Sprintf("agent %s", s.Name), s.Config); err != nil {
			return err
		}
		if s.Flow != nil {
			return ctx.child().validateAINativeStmtList(s.Flow.Stmts, false)
		}
	case *BudgetStmt:
		return ctx.child().validateAINativeStmtList(s.Body.Stmts, false)
	case *IfStmt:
		if err := ctx.validateAINativeExpr(s.Cond); err != nil {
			return err
		}
		if err := ctx.child().validateAINativeStmtList(s.Body.Stmts, false); err != nil {
			return err
		}
		for _, ei := range s.ElseIfs {
			if err := ctx.validateAINativeExpr(ei.Cond); err != nil {
				return err
			}
		}
		for _, ei := range s.ElseIfs {
			if err := ctx.child().validateAINativeStmtList(ei.Body.Stmts, false); err != nil {
				return err
			}
		}
		if s.ElseBody != nil {
			return ctx.child().validateAINativeStmtList(s.ElseBody.Stmts, false)
		}
	case *ForStmt:
		if err := ctx.validateAINativeExpr(s.Cond); err != nil {
			return err
		}
		return ctx.child().validateAINativeStmtList(s.Body.Stmts, false)
	case *ForNumStmt:
		if err := ctx.validateAINativeStmt(s.Init, false); err != nil {
			return err
		}
		if err := ctx.validateAINativeExpr(s.Cond); err != nil {
			return err
		}
		if err := ctx.validateAINativeStmt(s.Post, false); err != nil {
			return err
		}
		return ctx.child().validateAINativeStmtList(s.Body.Stmts, false)
	case *ForRangeStmt:
		if err := ctx.validateAINativeExpr(s.Iter); err != nil {
			return err
		}
		return ctx.child().validateAINativeStmtList(s.Body.Stmts, false)
	case *ReturnStmt:
		return ctx.validateAINativeExprList(s.Values)
	case *SelectStmt:
		for _, c := range s.Cases {
			if err := ctx.child().validateAINativeStmtList(c.Body.Stmts, false); err != nil {
				return err
			}
		}
		if s.Default != nil {
			return ctx.child().validateAINativeStmtList(s.Default.Stmts, false)
		}
	}
	return nil
}

func (ctx *aiValidationContext) validateAINativeExprList(exprs []Expr) error {
	for _, expr := range exprs {
		if err := ctx.validateAINativeExpr(expr); err != nil {
			return err
		}
	}
	return nil
}

func (ctx *aiValidationContext) validateAINativeExpr(expr Expr) error {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *BinaryExpr:
		if err := ctx.validateAINativeExpr(e.Left); err != nil {
			return err
		}
		return ctx.validateAINativeExpr(e.Right)
	case *UnaryExpr:
		return ctx.validateAINativeExpr(e.Operand)
	case *ParenExpr:
		return ctx.validateAINativeExpr(e.Inner)
	case *IndexExpr:
		if err := ctx.validateAINativeExpr(e.Table); err != nil {
			return err
		}
		return ctx.validateAINativeExpr(e.Index)
	case *FieldExpr:
		return ctx.validateAINativeExpr(e.Table)
	case *CallExpr:
		if err := ctx.validateAINativeExpr(e.Func); err != nil {
			return err
		}
		return ctx.validateAINativeExprList(e.Args)
	case *MethodCallExpr:
		if err := ctx.validateAINativeExpr(e.Object); err != nil {
			return err
		}
		return ctx.validateAINativeExprList(e.Args)
	case *FuncLitExpr:
		return ctx.child().validateAINativeStmtList(e.Body.Stmts, false)
	case *AgentLitExpr:
		if err := ctx.validateAIToolsConfig(e.P, "agent expression", e.Config); err != nil {
			return err
		}
		if e.Flow != nil {
			return ctx.child().validateAINativeStmtList(e.Flow.Stmts, false)
		}
	case *TurnExpr:
		return ctx.validateAIToolsConfig(e.P, "turn", e.Config)
	case *MessagesExpr:
		return ctx.validateAIConfigExprs(e.Fields)
	case *ListLitExpr:
		return ctx.validateAINativeExprList(e.Values)
	case *TableLitExpr:
		for _, f := range e.Fields {
			if err := ctx.validateAINativeExpr(f.Key); err != nil {
				return err
			}
			if err := ctx.validateAINativeExpr(f.Value); err != nil {
				return err
			}
		}
	case *DenseLitExpr:
		return ctx.validateAINativeExprList(e.Values)
	case *RecvExpr:
		return ctx.validateAINativeExpr(e.Channel)
	case *MakeChanExpr:
		return ctx.validateAINativeExpr(e.Size)
	}
	return nil
}

func (ctx *aiValidationContext) validateAIConfigExprs(config []ConfigField) error {
	for _, f := range config {
		if err := ctx.validateAINativeExpr(f.Key); err != nil {
			return err
		}
		if err := ctx.validateAINativeExpr(f.Value); err != nil {
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

func (ctx *aiValidationContext) validateAIToolsConfig(pos Pos, owner string, config []ConfigField) error {
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
			if _, ok := ctx.tools[ident.Name]; !ok {
				return fmt.Errorf("line %d: %s tools list references undeclared tool %q", pos.Line, owner, ident.Name)
			}
			seen[ident.Name] = true
		}
	}
	return nil
}

func (ctx *aiValidationContext) validateAINativeStmtList(stmts []Stmt, topLevel bool) error {
	for _, stmt := range stmts {
		if tool, ok := stmt.(*ToolDeclStmt); ok {
			if err := ctx.declareTool(tool); err != nil {
				return err
			}
		}
		if err := ctx.validateAINativeStmt(stmt, topLevel); err != nil {
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
