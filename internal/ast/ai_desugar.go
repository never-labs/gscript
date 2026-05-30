package ast

import "sort"

// DesugarAINative rewrites AI-native syntax into the existing stdlib-shaped
// AST. The parser keeps dedicated nodes so tooling can see source intent, while
// interpreters and bytecode compilers can call this boundary once and execute
// the same llm.* runtime substrate as hand-written library code.
func DesugarAINative(prog *Program) *Program {
	if prog == nil {
		return nil
	}
	out := &Program{Stmts: desugarStmtList(prog.Stmts)}
	return out
}

func desugarStmtList(stmts []Stmt) []Stmt {
	out := make([]Stmt, 0, len(stmts))
	for _, stmt := range stmts {
		out = append(out, desugarStmt(stmt))
	}
	return out
}

func desugarStmt(stmt Stmt) Stmt {
	switch s := stmt.(type) {
	case *DeclareStmt:
		values := make([]Expr, len(s.Values))
		for i, v := range s.Values {
			values[i] = desugarExpr(v)
		}
		return &DeclareStmt{P: s.P, Names: append([]string(nil), s.Names...), Values: values, ReadOnly: s.ReadOnly}
	case *AssignStmt:
		return &AssignStmt{P: s.P, Targets: desugarExprList(s.Targets), Values: desugarExprList(s.Values)}
	case *CompoundAssignStmt:
		return &CompoundAssignStmt{P: s.P, Target: desugarExpr(s.Target), Op: s.Op, Value: desugarExpr(s.Value)}
	case *IncDecStmt:
		return &IncDecStmt{P: s.P, Target: desugarExpr(s.Target), Op: s.Op}
	case *CallStmt:
		return &CallStmt{P: s.P, Call: desugarCallExpr(s.Call)}
	case *GoStmt:
		return &GoStmt{P: s.P, Call: desugarExpr(s.Call)}
	case *DeferStmt:
		return &DeferStmt{P: s.P, Call: desugarExpr(s.Call)}
	case *SendStmt:
		return &SendStmt{P: s.P, Channel: desugarExpr(s.Channel), Value: desugarExpr(s.Value)}
	case *SelectStmt:
		cp := *s
		cp.Cases = make([]SelectCase, len(s.Cases))
		for i, c := range s.Cases {
			c.Channel = desugarExpr(c.Channel)
			c.SendValue = desugarExpr(c.SendValue)
			c.Body = desugarBlock(c.Body)
			cp.Cases[i] = c
		}
		cp.Default = desugarBlock(s.Default)
		return &cp
	case *IfStmt:
		cp := *s
		cp.Cond = desugarExpr(s.Cond)
		cp.Body = desugarBlock(s.Body)
		cp.ElseIfs = make([]ElseIfClause, len(s.ElseIfs))
		for i, ei := range s.ElseIfs {
			ei.Cond = desugarExpr(ei.Cond)
			ei.Body = desugarBlock(ei.Body)
			cp.ElseIfs[i] = ei
		}
		cp.ElseBody = desugarBlock(s.ElseBody)
		return &cp
	case *ForNumStmt:
		return &ForNumStmt{P: s.P, Init: desugarStmt(s.Init), Cond: desugarExpr(s.Cond), Post: desugarStmt(s.Post), Body: desugarBlock(s.Body)}
	case *ForRangeStmt:
		return &ForRangeStmt{P: s.P, Key: s.Key, Value: s.Value, Iter: desugarExpr(s.Iter), Body: desugarBlock(s.Body)}
	case *ForStmt:
		return &ForStmt{P: s.P, Cond: desugarExpr(s.Cond), Body: desugarBlock(s.Body)}
	case *ReturnStmt:
		return &ReturnStmt{P: s.P, Values: desugarExprList(s.Values)}
	case *FuncDeclStmt:
		return &FuncDeclStmt{P: s.P, Name: s.Name, Params: append([]FuncParam(nil), s.Params...), Body: desugarBlock(s.Body)}
	case *BlockStmt:
		return desugarBlock(s)
	case *ToolDeclStmt:
		return &DeclareStmt{
			P:     s.P,
			Names: []string{s.Name},
			Values: []Expr{llmCall(s.P, "tool",
				&StringLit{P: s.P, Value: s.Name},
				&FuncLitExpr{P: s.P, Params: append([]FuncParam(nil), s.Params...), Body: desugarBlock(s.Body)},
				toolOptionsTable(s.P, s.Params, s.DocComment, s.Requires, s.ParamDocs),
			)},
		}
	case *AgentDeclStmt:
		return &DeclareStmt{
			P:      s.P,
			Names:  []string{s.Name},
			Values: []Expr{desugarAgentValue(s.P, s.Name, s.Params, s.Config, s.Flow)},
		}
	case *AgentDefaultsDeclStmt:
		return &CallStmt{P: s.P, Call: llmCall(s.P, "agent_defaults", configTable(s.P, s.Config))}
	case *ModelsDeclStmt:
		return &CallStmt{P: s.P, Call: llmCall(s.P, "register_models", configTable(s.P, s.Config))}
	case *BudgetStmt:
		return &CallStmt{P: s.P, Call: llmCall(s.P, "with_budget",
			configTable(s.P, s.Config),
			&FuncLitExpr{P: s.P, Body: desugarBlock(s.Body)},
		)}
	default:
		return stmt
	}
}

func desugarBlock(block *BlockStmt) *BlockStmt {
	if block == nil {
		return nil
	}
	return &BlockStmt{P: block.P, Stmts: desugarStmtList(block.Stmts)}
}

func desugarExprList(exprs []Expr) []Expr {
	out := make([]Expr, len(exprs))
	for i, e := range exprs {
		out[i] = desugarExpr(e)
	}
	return out
}

func desugarExpr(expr Expr) Expr {
	switch e := expr.(type) {
	case nil:
		return nil
	case *BinaryExpr:
		return &BinaryExpr{P: e.P, Left: desugarExpr(e.Left), Op: e.Op, Right: desugarExpr(e.Right)}
	case *UnaryExpr:
		return &UnaryExpr{P: e.P, Op: e.Op, Operand: desugarExpr(e.Operand)}
	case *ParenExpr:
		return &ParenExpr{P: e.P, Inner: desugarExpr(e.Inner)}
	case *IndexExpr:
		return &IndexExpr{P: e.P, Table: desugarExpr(e.Table), Index: desugarExpr(e.Index)}
	case *FieldExpr:
		return &FieldExpr{P: e.P, Table: desugarExpr(e.Table), Field: e.Field}
	case *CallExpr:
		return desugarCallExpr(e)
	case *MethodCallExpr:
		return &MethodCallExpr{P: e.P, Object: desugarExpr(e.Object), Method: e.Method, Args: desugarExprList(e.Args)}
	case *FuncLitExpr:
		return &FuncLitExpr{P: e.P, Params: append([]FuncParam(nil), e.Params...), Body: desugarBlock(e.Body)}
	case *TableLitExpr:
		return desugarTable(e)
	case *DenseLitExpr:
		return &DenseLitExpr{P: e.P, DType: e.DType, Len: e.Len, Values: desugarExprList(e.Values)}
	case *RecvExpr:
		return &RecvExpr{P: e.P, Channel: desugarExpr(e.Channel)}
	case *MakeChanExpr:
		return &MakeChanExpr{P: e.P, Size: desugarExpr(e.Size)}
	case *ListLitExpr:
		fields := make([]TableField, 0, len(e.Values))
		for _, v := range e.Values {
			fields = append(fields, TableField{Value: desugarExpr(v)})
		}
		return &TableLitExpr{P: e.P, Fields: fields}
	case *MessagesExpr:
		return messagesTable(e.P, e.Fields)
	case *TurnExpr:
		return llmCall(e.P, "turn", configTable(e.P, e.Config))
	case *AgentLitExpr:
		return desugarAgentValue(e.P, "", e.Params, e.Config, e.Flow)
	default:
		return expr
	}
}

func desugarCallExpr(e *CallExpr) *CallExpr {
	if e == nil {
		return nil
	}
	return &CallExpr{P: e.P, Func: desugarExpr(e.Func), Args: desugarExprList(e.Args)}
}

func desugarTable(e *TableLitExpr) *TableLitExpr {
	if e == nil {
		return nil
	}
	fields := make([]TableField, len(e.Fields))
	for i, f := range e.Fields {
		fields[i] = TableField{Key: desugarExpr(f.Key), Value: desugarExpr(f.Value)}
	}
	return &TableLitExpr{P: e.P, Fields: fields}
}

func desugarAgentValue(pos Pos, name string, params []FuncParam, config []ConfigField, flow *BlockStmt) Expr {
	configFn := &FuncLitExpr{P: pos, Params: append([]FuncParam(nil), params...), Body: &BlockStmt{P: pos, Stmts: []Stmt{
		&ReturnStmt{P: pos, Values: []Expr{configTable(pos, config)}},
	}}}
	if flow == nil {
		return llmCall(pos, "agent",
			&StringLit{P: pos, Value: name},
			configFn,
			&NilLit{P: pos},
			agentMetadataTable(pos, params, config),
		)
	}
	body := &BlockStmt{P: pos}
	body.Stmts = append(body.Stmts, configLocalDecls(config)...)
	body.Stmts = append(body.Stmts, desugarStmtList(flow.Stmts)...)
	return llmCall(pos, "agent",
		&StringLit{P: pos, Value: name},
		configFn,
		&FuncLitExpr{P: pos, Params: append([]FuncParam(nil), params...), Body: body},
		agentMetadataTable(pos, params, config),
	)
}

func agentMetadataTable(pos Pos, params []FuncParam, config []ConfigField) Expr {
	paramFields := make([]TableField, 0, len(params))
	for _, p := range params {
		if p.IsVarArg {
			continue
		}
		paramFields = append(paramFields, TableField{Value: &StringLit{P: pos, Value: p.Name}})
	}
	fields := []TableField{
		{Key: &StringLit{P: pos, Value: "params"}, Value: &TableLitExpr{P: pos, Fields: paramFields}},
	}
	var systemDesc, explicitDesc string
	for _, f := range config {
		key, ok := stringConfigKey(f.Key)
		if !ok {
			continue
		}
		switch key {
		case "output":
			fields = append(fields, TableField{Key: &StringLit{P: pos, Value: "output"}, Value: desugarExpr(f.Value)})
		case "system":
			if lit, ok := f.Value.(*StringLit); ok {
				systemDesc = lit.Value
			}
		case "description":
			if lit, ok := f.Value.(*StringLit); ok {
				explicitDesc = lit.Value
			}
		}
	}
	desc := explicitDesc
	if desc == "" {
		desc = systemDesc
	}
	if desc != "" {
		fields = append(fields, TableField{Key: &StringLit{P: pos, Value: "description"}, Value: &StringLit{P: pos, Value: desc}})
	}
	return &TableLitExpr{P: pos, Fields: fields}
}

func configLocalDecls(config []ConfigField) []Stmt {
	out := make([]Stmt, 0, len(config))
	for _, f := range config {
		key, ok := stringConfigKey(f.Key)
		if !ok || !isAgentFlowImplicitConfigLocal(key) {
			continue
		}
		out = append(out, &DeclareStmt{P: f.P, Names: []string{key}, Values: []Expr{desugarExpr(f.Value)}})
	}
	return out
}

func isAgentFlowImplicitConfigLocal(key string) bool {
	switch key {
	case "model", "system", "tools", "caps", "capabilities":
		return true
	default:
		return false
	}
}

func configTable(pos Pos, config []ConfigField) Expr {
	fields := make([]TableField, 0, len(config))
	for _, f := range config {
		fields = append(fields, TableField{Key: desugarExpr(f.Key), Value: desugarExpr(f.Value)})
	}
	return &TableLitExpr{P: pos, Fields: fields}
}

func messagesTable(pos Pos, fields []TableField) Expr {
	out := make([]TableField, 0, len(fields))
	for _, f := range fields {
		if f.Key == nil {
			out = append(out, TableField{Value: desugarExpr(f.Value)})
			continue
		}
		key, ok := stringConfigKey(f.Key)
		if ok && isMessageRole(key) {
			out = append(out, TableField{Value: llmCall(messageFieldPos(f), key, desugarExpr(f.Value))})
			continue
		}
		fieldPos := messageFieldPos(f)
		out = append(out, TableField{Value: &TableLitExpr{P: fieldPos, Fields: []TableField{
			{Key: &StringLit{P: fieldPos, Value: "role"}, Value: desugarExpr(f.Key)},
			{Key: &StringLit{P: fieldPos, Value: "text"}, Value: desugarExpr(f.Value)},
		}}})
	}
	return &TableLitExpr{P: pos, Fields: out}
}

func messageFieldPos(f TableField) Pos {
	if f.Key != nil {
		return f.Key.GetPos()
	}
	if f.Value != nil {
		return f.Value.GetPos()
	}
	return Pos{}
}

func toolOptionsTable(pos Pos, params []FuncParam, description string, requires []string, paramDocs map[string]string) Expr {
	paramFields := make([]TableField, 0, len(params))
	for _, p := range params {
		if p.IsVarArg {
			continue
		}
		paramFields = append(paramFields, TableField{Value: &StringLit{P: pos, Value: p.Name}})
	}
	fields := []TableField{
		{Key: &StringLit{P: pos, Value: "params"}, Value: &TableLitExpr{P: pos, Fields: paramFields}},
	}
	if description != "" {
		fields = append(fields, TableField{Key: &StringLit{P: pos, Value: "description"}, Value: &StringLit{P: pos, Value: description}})
	}
	if len(requires) > 0 {
		requireFields := make([]TableField, 0, len(requires))
		for _, capability := range requires {
			requireFields = append(requireFields, TableField{Value: &StringLit{P: pos, Value: capability}})
		}
		fields = append(fields, TableField{Key: &StringLit{P: pos, Value: "requires"}, Value: &TableLitExpr{P: pos, Fields: requireFields}})
	}
	if len(paramDocs) > 0 {
		paramDocFields := make([]TableField, 0, len(paramDocs))
		seen := map[string]bool{}
		for _, p := range params {
			if doc, ok := paramDocs[p.Name]; ok {
				paramDocFields = append(paramDocFields, TableField{Key: &StringLit{P: pos, Value: p.Name}, Value: &StringLit{P: pos, Value: doc}})
				seen[p.Name] = true
			}
		}
		var extra []string
		for name := range paramDocs {
			if !seen[name] {
				extra = append(extra, name)
			}
		}
		sort.Strings(extra)
		for _, name := range extra {
			paramDocFields = append(paramDocFields, TableField{Key: &StringLit{P: pos, Value: name}, Value: &StringLit{P: pos, Value: paramDocs[name]}})
		}
		fields = append(fields, TableField{Key: &StringLit{P: pos, Value: "param_docs"}, Value: &TableLitExpr{P: pos, Fields: paramDocFields}})
	}
	return &TableLitExpr{P: pos, Fields: fields}
}

func llmCall(pos Pos, name string, args ...Expr) *CallExpr {
	return &CallExpr{
		P: pos,
		Func: &FieldExpr{
			P:     pos,
			Table: &IdentExpr{P: pos, Name: "llm"},
			Field: name,
		},
		Args: args,
	}
}

func stringConfigKey(expr Expr) (string, bool) {
	if lit, ok := expr.(*StringLit); ok {
		return lit.Value, true
	}
	return "", false
}

func isMessageRole(role string) bool {
	switch role {
	case "system", "user", "assistant":
		return true
	default:
		return false
	}
}

func isIdentName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
