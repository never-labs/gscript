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
			if err := ctx.validateAIToolsConfig("agent defaults", s.Config); err != nil {
				return err
			}
			defaults, err := ctx.staticAIConfig("agent defaults", s.Config)
			if err != nil {
				return err
			}
			ctx.agentDefaults = defaults
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
	tools         AIToolRegistry
	agentDefaults *aiStaticAIConfig
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
	child := &aiValidationContext{tools: AIToolRegistry{}, agentDefaults: ctx.agentDefaults}
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
		if err := ctx.validateAIToolsConfig(fmt.Sprintf("agent %s", s.Name), s.Config); err != nil {
			return err
		}
		if err := ctx.validateAgentDefaultsMerge(s.P, fmt.Sprintf("agent %s", s.Name), s.Config); err != nil {
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
		if err := ctx.validateAIToolsConfig("agent expression", e.Config); err != nil {
			return err
		}
		if err := ctx.validateAgentDefaultsMerge(e.P, "agent expression", e.Config); err != nil {
			return err
		}
		if e.Flow != nil {
			return ctx.child().validateAINativeStmtList(e.Flow.Stmts, false)
		}
	case *TurnExpr:
		return ctx.validateAIToolsConfig("turn", e.Config)
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

func (ctx *aiValidationContext) validateAIToolsConfig(owner string, config []ConfigField) error {
	info, err := ctx.staticAIConfig(owner, config)
	if err != nil {
		return err
	}
	return ctx.validateStaticToolCaps(owner, info.tools, info.toolsPresent && info.toolsStatic, info.caps, info.capsPresent && info.capsStatic, info.capsPos)
}

type aiStaticAIConfig struct {
	tools        []string
	toolsPresent bool
	toolsStatic  bool
	caps         []string
	capsPresent  bool
	capsStatic   bool
	capsPos      Pos
}

func (ctx *aiValidationContext) staticAIConfig(owner string, config []ConfigField) (*aiStaticAIConfig, error) {
	tools, toolsPresent, toolsStatic, err := ctx.staticToolsConfig(owner, config)
	if err != nil {
		return nil, err
	}
	caps, capsPresent, capsStatic, capsPos := staticCapsConfig(config)
	return &aiStaticAIConfig{
		tools:        tools,
		toolsPresent: toolsPresent,
		toolsStatic:  toolsStatic,
		caps:         caps,
		capsPresent:  capsPresent,
		capsStatic:   capsStatic,
		capsPos:      capsPos,
	}, nil
}

func (ctx *aiValidationContext) validateAgentDefaultsMerge(pos Pos, owner string, config []ConfigField) error {
	if ctx.agentDefaults == nil {
		return nil
	}
	agent, err := ctx.staticAIConfig(owner, config)
	if err != nil {
		return err
	}
	defaults := ctx.agentDefaults
	if (defaults.toolsPresent && !defaults.toolsStatic) || (defaults.capsPresent && !defaults.capsStatic) ||
		(agent.toolsPresent && !agent.toolsStatic) || (agent.capsPresent && !agent.capsStatic) {
		return nil
	}

	tools, toolsStatic := defaults.tools, defaults.toolsPresent && defaults.toolsStatic
	if agent.toolsPresent {
		tools, toolsStatic = agent.tools, agent.toolsStatic
	}
	caps, capsStatic, capsPos := defaults.caps, defaults.capsPresent && defaults.capsStatic, defaults.capsPos
	if agent.capsPresent {
		caps, capsStatic, capsPos = agent.caps, agent.capsStatic, agent.capsPos
	}
	return ctx.validateStaticToolCaps(owner, tools, toolsStatic, caps, capsStatic, capsPos)
}

func (ctx *aiValidationContext) validateStaticToolCaps(owner string, tools []string, toolsStatic bool, caps []string, capsStatic bool, capsPos Pos) error {
	if !toolsStatic || !capsStatic {
		return nil
	}
	for _, req := range ctx.tools.RequiredCapabilitiesForTools(tools) {
		if capAllows(caps, req) {
			continue
		}
		return fmt.Errorf("line %d: %s capabilities missing required capability %q", capsPos.Line, owner, req)
	}
	return nil
}

func (ctx *aiValidationContext) staticToolsConfig(owner string, config []ConfigField) ([]string, bool, bool, error) {
	for _, f := range config {
		key, ok := stringConfigKey(f.Key)
		if !ok || key != "tools" {
			continue
		}
		list, ok := f.Value.(*ListLitExpr)
		if !ok {
			return nil, true, false, nil
		}
		seen := map[string]bool{}
		names := make([]string, 0, len(list.Values))
		allStatic := true
		for _, v := range list.Values {
			ident, ok := v.(*IdentExpr)
			if !ok {
				allStatic = false
				continue
			}
			if seen[ident.Name] {
				return nil, false, false, fmt.Errorf("line %d: %s tools list includes duplicate tool %q", ident.P.Line, owner, ident.Name)
			}
			if _, ok := ctx.tools.Lookup(ident.Name); !ok {
				return nil, false, false, fmt.Errorf("line %d: %s tools list references undeclared tool %q", ident.P.Line, owner, ident.Name)
			}
			seen[ident.Name] = true
			names = append(names, ident.Name)
		}
		return names, true, allStatic, nil
	}
	return nil, false, false, nil
}

func staticCapsConfig(config []ConfigField) ([]string, bool, bool, Pos) {
	for _, f := range config {
		key, ok := stringConfigKey(f.Key)
		if !ok || (key != "capabilities" && key != "caps") {
			continue
		}
		caps, ok := staticStringList(f.Value)
		if !ok {
			return nil, true, false, f.P
		}
		return caps, true, true, f.P
	}
	return nil, false, false, Pos{}
}

func staticStringList(expr Expr) ([]string, bool) {
	switch e := expr.(type) {
	case *ListLitExpr:
		return staticStringExprs(e.Values)
	case *TableLitExpr:
		values := make([]Expr, 0, len(e.Fields))
		for _, f := range e.Fields {
			if f.Key != nil {
				return nil, false
			}
			values = append(values, f.Value)
		}
		return staticStringExprs(values)
	default:
		return nil, false
	}
}

func staticStringExprs(exprs []Expr) ([]string, bool) {
	out := make([]string, 0, len(exprs))
	for _, expr := range exprs {
		lit, ok := expr.(*StringLit)
		if !ok {
			return nil, false
		}
		out = append(out, lit.Value)
	}
	return out, true
}

func capAllows(caps []string, req string) bool {
	for _, cap := range caps {
		if cap == req || cap == "all" || cap == "cap.all" || cap == "*" {
			return true
		}
	}
	return false
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
	hasProtocol := false
	hasProviderModel := false
	for _, f := range tbl.Fields {
		key, ok := stringConfigKey(f.Key)
		if !ok {
			continue
		}
		switch key {
		case "api_key":
			if _, ok := f.Value.(*StringLit); ok {
				return fmt.Errorf("model api_key must not be a string literal")
			}
		case "protocol":
			hasProtocol = true
			protocol, ok := f.Value.(*StringLit)
			if !ok {
				return fmt.Errorf("model protocol must be a string literal")
			}
			if !isAllowedModelProtocol(protocol.Value) {
				return fmt.Errorf("unsupported model protocol %q", protocol.Value)
			}
		case "provider_model", "model":
			hasProviderModel = true
		}
		if nested, ok := f.Value.(*TableLitExpr); ok {
			if err := validateModelConfigTable(nested); err != nil {
				return err
			}
		}
	}
	if hasProtocol && !hasProviderModel {
		return fmt.Errorf("model provider config with protocol must include provider_model or model")
	}
	return nil
}

func isAllowedModelProtocol(protocol string) bool {
	switch strings.ToLower(strings.ReplaceAll(protocol, "_", "-")) {
	case "openai", "openai-compatible", "openai-compat", "chat-completions",
		"anthropic", "anthropic-compatible", "anthropic-compat", "messages":
		return true
	default:
		return false
	}
}
