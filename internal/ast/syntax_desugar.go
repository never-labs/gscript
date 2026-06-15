package ast

// DesugarSyntax rewrites parser-level syntax sugar into ordinary stdlib-shaped
// AST. It is intentionally small: language extensions should lower here only
// when they have a general syntactic surface such as interpolation, tagged
// dialect literals, or list literals.
func DesugarSyntax(prog *Program) *Program {
	if prog == nil {
		return nil
	}
	return &Program{
		Stmts:          desugarStmtList(prog.Stmts),
		FileDirectives: cloneFileDirectives(prog.FileDirectives),
	}
}

func cloneFileDirectives(directives []FileDirective) []FileDirective {
	if len(directives) == 0 {
		return nil
	}
	out := make([]FileDirective, len(directives))
	for i, directive := range directives {
		out[i] = directive
		out[i].Args = append([]string(nil), directive.Args...)
	}
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
	case *InterpolatedStringExpr:
		return interpolatedStringConcat(e)
	case *TaggedStringExpr:
		return dialectCall(e.P, "eval", e.Tag, taggedStringBody(e.P, e.Tag, e.Body), nil, e.FailFast)
	case *TaggedBlockExpr:
		if e.HasRawSource {
			return dialectCall(e.P, "eval", e.Tag, &StringLit{P: e.P, Value: e.RawSource}, nil, e.FailFast)
		}
		if e.Body != nil {
			return dialectCall(e.P, "eval_raw", e.Tag, &FuncLitExpr{P: e.P, Body: desugarBlock(e.Body)}, nil, e.FailFast)
		}
		return dialectCall(e.P, "eval_block", e.Tag, configTable(e.P, e.Config), nil, e.FailFast)
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
	default:
		return expr
	}
}

func taggedStringBody(pos Pos, tag string, body Expr) Expr {
	interp, ok := body.(*InterpolatedStringExpr)
	if !ok {
		return body
	}
	rawParts := make([]Expr, 0, len(interp.Parts)+1)
	values := make([]Expr, 0, len(interp.Parts))
	rawParts = append(rawParts, &StringLit{P: pos, Value: ""})
	for _, part := range interp.Parts {
		if part.Expr != nil {
			values = append(values, desugarExpr(part.Expr))
			rawParts = append(rawParts, &StringLit{P: pos, Value: ""})
			continue
		}
		if part.Text != "" {
			rawParts[len(rawParts)-1] = &StringLit{P: interp.P, Value: part.Text}
		}
	}
	return runtimeTableCall(pos, "dialect", "interpolate",
		&StringLit{P: pos, Value: tag},
		dialectInterpolationArray(pos, rawParts),
		dialectInterpolationArray(pos, values),
	)
}

func dialectInterpolationArray(pos Pos, values []Expr) Expr {
	fields := make([]TableField, 0, len(values))
	for _, value := range values {
		fields = append(fields, TableField{Value: value})
	}
	return &TableLitExpr{P: pos, Fields: fields}
}

func interpolatedStringConcat(e *InterpolatedStringExpr) Expr {
	if e == nil || len(e.Parts) == 0 {
		return &StringLit{P: Pos{}, Value: ""}
	}
	values := make([]Expr, 0, len(e.Parts))
	for _, part := range e.Parts {
		if part.Expr != nil {
			values = append(values, &CallExpr{
				P:    part.Expr.GetPos(),
				Func: &IdentExpr{P: part.Expr.GetPos(), Name: "tostring"},
				Args: []Expr{desugarExpr(part.Expr)},
			})
			continue
		}
		if part.Text != "" {
			values = append(values, &StringLit{P: e.P, Value: part.Text})
		}
	}
	if len(values) == 0 {
		return &StringLit{P: e.P, Value: ""}
	}
	out := values[0]
	for i := 1; i < len(values); i++ {
		out = &BinaryExpr{P: values[i].GetPos(), Left: out, Op: "..", Right: values[i]}
	}
	return out
}

func dialectCall(pos Pos, method, tag string, body Expr, opts Expr, failFast bool) Expr {
	if opts == nil {
		opts = &TableLitExpr{P: pos}
	}
	if table, ok := opts.(*TableLitExpr); ok {
		table = desugarTable(table)
		table.Fields = append(table.Fields, TableField{Key: &StringLit{P: pos, Value: "fail_fast"}, Value: &BoolLit{P: pos, Value: failFast}})
		opts = table
	}
	return runtimeTableCall(pos, "dialect", method,
		&StringLit{P: pos, Value: tag},
		desugarExpr(body),
		opts,
	)
}

func desugarCallExpr(e Expr) Expr {
	switch call := e.(type) {
	case *CallExpr:
		return desugarPlainCallExpr(call)
	case *MethodCallExpr:
		return &MethodCallExpr{P: call.P, Object: desugarExpr(call.Object), Method: call.Method, Args: desugarExprList(call.Args)}
	default:
		return desugarExpr(e)
	}
}

func desugarPlainCallExpr(e *CallExpr) *CallExpr {
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

func configTable(pos Pos, config []ConfigField) Expr {
	fields := make([]TableField, 0, len(config))
	for _, f := range config {
		fields = append(fields, TableField{Key: desugarExpr(f.Key), Value: desugarExpr(f.Value)})
	}
	return &TableLitExpr{P: pos, Fields: fields}
}

func runtimeTableCall(pos Pos, table, name string, args ...Expr) *CallExpr {
	return &CallExpr{
		P: pos,
		Func: &FieldExpr{
			P:     pos,
			Table: &IdentExpr{P: pos, Name: table},
			Field: name,
		},
		Args: args,
	}
}
