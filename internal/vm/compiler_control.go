package vm

// compiler_control.go — control-flow statement compilation for the
// AST→bytecode compiler: if, numeric/generic/range for, while, return,
// break, continue, goto, label, function-declaration and block statements.
// Pure code movement from compiler.go; declarations are verbatim.

import (
	"fmt"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/runtime"
)

func (c *compiler) compileIfStmt(s *ast.IfStmt) error {
	line := s.P.Line
	var endJumps []int

	elseJump, err := c.compileCondJump(s.Cond, line)
	if err != nil {
		return err
	}

	c.enterScope()
	for _, st := range s.Body.Stmts {
		if err := c.compileStmt(st); err != nil {
			return err
		}
	}
	c.leaveScope()

	if len(s.ElseIfs) > 0 || s.ElseBody != nil {
		endJumps = append(endJumps, c.emitJump(line))
	}
	c.patchJump(elseJump)

	for _, elif := range s.ElseIfs {
		nextJump, err := c.compileCondJump(elif.Cond, elif.P.Line)
		if err != nil {
			return err
		}

		c.enterScope()
		for _, st := range elif.Body.Stmts {
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		c.leaveScope()

		endJumps = append(endJumps, c.emitJump(elif.P.Line))
		c.patchJump(nextJump)
	}

	if s.ElseBody != nil {
		c.enterScope()
		for _, st := range s.ElseBody.Stmts {
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		c.leaveScope()
	}

	for _, j := range endJumps {
		c.patchJump(j)
	}
	return nil
}

// ---- ForNumStmt ----

func (c *compiler) compileForNumStmt(s *ast.ForNumStmt) error {
	line := s.P.Line

	if forInfo, ok := c.detectSimpleNumericFor(s); ok {
		return c.compileOptimizedForNum(s, forInfo, line)
	}

	// Generic C-style for loop
	c.enterScope()

	if s.Init != nil {
		if err := c.compileStmt(s.Init); err != nil {
			return err
		}
	}

	c.pushLoop()
	loopTop := c.currentPC()

	if s.Cond != nil {
		breakJump, err := c.compileCondJump(s.Cond, line)
		if err != nil {
			return err
		}
		c.currentLoop().breakJumps = append(c.currentLoop().breakJumps, breakJump)
	}

	c.enterScope()
	for _, st := range s.Body.Stmts {
		if err := c.compileStmt(st); err != nil {
			return err
		}
	}
	c.leaveScope()

	continueTarget := c.currentPC()

	if s.Post != nil {
		if err := c.compileStmt(s.Post); err != nil {
			return err
		}
	}

	loopBack := c.emitJump(line)
	c.patchJumpTo(loopBack, loopTop)

	info := c.popLoop()
	c.patchBreaks(info)
	c.patchContinues(info, continueTarget)

	c.leaveScope()
	return nil
}

type simpleForInfo struct {
	varName string
}

func (c *compiler) detectSimpleNumericFor(s *ast.ForNumStmt) (simpleForInfo, bool) {
	decl, ok := s.Init.(*ast.DeclareStmt)
	if !ok || len(decl.Names) != 1 || len(decl.Values) != 1 {
		return simpleForInfo{}, false
	}
	varName := decl.Names[0]

	bin, ok := s.Cond.(*ast.BinaryExpr)
	if !ok {
		return simpleForInfo{}, false
	}
	ident, ok := bin.Left.(*ast.IdentExpr)
	if !ok || ident.Name != varName {
		return simpleForInfo{}, false
	}
	if bin.Op != "<" && bin.Op != "<=" {
		return simpleForInfo{}, false
	}

	switch post := s.Post.(type) {
	case *ast.IncDecStmt:
		if post.Op != "++" {
			return simpleForInfo{}, false
		}
		ident2, ok := post.Target.(*ast.IdentExpr)
		if !ok || ident2.Name != varName {
			return simpleForInfo{}, false
		}
	case *ast.CompoundAssignStmt:
		if post.Op != "+=" {
			return simpleForInfo{}, false
		}
		ident2, ok := post.Target.(*ast.IdentExpr)
		if !ok || ident2.Name != varName {
			return simpleForInfo{}, false
		}
	case *ast.AssignStmt:
		// Handle `i = i + step` pattern as equivalent to `i += step`
		if len(post.Targets) != 1 || len(post.Values) != 1 {
			return simpleForInfo{}, false
		}
		ident2, ok := post.Targets[0].(*ast.IdentExpr)
		if !ok || ident2.Name != varName {
			return simpleForInfo{}, false
		}
		binExpr, ok := post.Values[0].(*ast.BinaryExpr)
		if !ok || binExpr.Op != "+" {
			return simpleForInfo{}, false
		}
		leftIdent, ok := binExpr.Left.(*ast.IdentExpr)
		if !ok || leftIdent.Name != varName {
			return simpleForInfo{}, false
		}
	default:
		return simpleForInfo{}, false
	}

	return simpleForInfo{varName: varName}, true
}

func (c *compiler) compileOptimizedForNum(s *ast.ForNumStmt, info simpleForInfo, line int) error {
	c.enterScope()

	// Reserve 4 consecutive registers: R(A)=index, R(A+1)=limit, R(A+2)=step, R(A+3)=loop var
	baseReg := c.allocRegs(4)

	decl := s.Init.(*ast.DeclareStmt)
	bin := s.Cond.(*ast.BinaryExpr)

	// Compile init value into R(A)
	if err := c.compileExprTo(decl.Values[0], baseReg); err != nil {
		return err
	}

	// Compile limit into R(A+1)
	if bin.Op == "<" {
		// Subtract 1 from the limit to turn `<` into `<=`
		limitTmp := c.allocReg()
		if err := c.compileExprTo(bin.Right, limitTmp); err != nil {
			return err
		}
		oneReg := c.allocReg()
		c.emitAsBx(OP_LOADINT, oneReg, 1, line)
		c.emitABC(OP_SUB, baseReg+1, limitTmp, oneReg, line)
		c.freeRegs(2) // oneReg, limitTmp
	} else {
		if err := c.compileExprTo(bin.Right, baseReg+1); err != nil {
			return err
		}
	}

	// Compile step into R(A+2)
	switch post := s.Post.(type) {
	case *ast.IncDecStmt:
		c.emitAsBx(OP_LOADINT, baseReg+2, 1, line)
	case *ast.CompoundAssignStmt:
		if err := c.compileExprTo(post.Value, baseReg+2); err != nil {
			return err
		}
	case *ast.AssignStmt:
		// i = i + step → extract step from BinaryExpr.Right
		binExpr := post.Values[0].(*ast.BinaryExpr)
		if err := c.compileExprTo(binExpr.Right, baseReg+2); err != nil {
			return err
		}
	}

	// Declare the loop variable as a local mapping to R(A+3)
	c.locals = append(c.locals, localVar{name: info.varName, reg: baseReg + 3, depth: c.depth})

	c.pushLoop()

	// FORPREP A sBx: R(A) -= R(A+2); PC += sBx
	forprepPos := c.emitAsBx(OP_FORPREP, baseReg, 0, line)

	loopBodyStart := c.currentPC()
	c.enterScope()
	for _, st := range s.Body.Stmts {
		if err := c.compileStmt(st); err != nil {
			return err
		}
	}
	c.leaveScope()

	continueTarget := c.currentPC()

	forloopOffset := loopBodyStart - c.currentPC() - 1
	c.emitAsBx(OP_FORLOOP, baseReg, forloopOffset, line)

	forprepOffset := continueTarget - forprepPos - 1
	c.proto.Code[forprepPos] = EncodeAsBx(OP_FORPREP, baseReg, forprepOffset)

	info2 := c.popLoop()
	c.patchBreaks(info2)
	c.patchContinues(info2, continueTarget)

	c.leaveScope()
	return nil
}

// ---- ForStmt ----

func (c *compiler) compileForStmt(s *ast.ForStmt) error {
	line := s.P.Line
	c.pushLoop()
	loopTop := c.currentPC()

	if s.Cond != nil {
		breakJump, err := c.compileCondJump(s.Cond, line)
		if err != nil {
			return err
		}
		c.currentLoop().breakJumps = append(c.currentLoop().breakJumps, breakJump)
	}

	c.enterScope()
	for _, st := range s.Body.Stmts {
		if err := c.compileStmt(st); err != nil {
			return err
		}
	}
	c.leaveScope()

	continueTarget := c.currentPC()
	loopBack := c.emitJump(line)
	c.patchJumpTo(loopBack, loopTop)

	info := c.popLoop()
	c.patchBreaks(info)
	c.patchContinues(info, continueTarget)
	return nil
}

// ---- ForRangeStmt ----

func (c *compiler) compileForRangeStmt(s *ast.ForRangeStmt) error {
	line := s.P.Line
	c.enterScope()

	iterBase := c.allocRegs(3)

	switch iter := s.Iter.(type) {
	case *ast.CallExpr:
		if err := c.compileCallExprMulti(iter, iterBase, 3); err != nil {
			return err
		}
	case *ast.MethodCallExpr:
		if err := c.compileMethodCallExprMulti(iter, iterBase, 3); err != nil {
			return err
		}
	default:
		if err := c.compileExprTo(s.Iter, iterBase); err != nil {
			return err
		}
		c.emitABC(OP_LOADNIL, iterBase+1, 1, 0, line)
	}

	nVars := 1
	if s.Value != "" {
		nVars = 2
	}
	capturesRangeVar := forRangeBodyCapturesVars(s)
	for c.nextReg < iterBase+3+nVars {
		c.allocReg()
	}
	if !capturesRangeVar {
		c.locals = append(c.locals, localVar{name: s.Key, reg: iterBase + 3, depth: c.depth})
		if s.Value != "" {
			c.locals = append(c.locals, localVar{name: s.Value, reg: iterBase + 4, depth: c.depth})
		}
	}

	c.pushLoop()

	loopTop := c.currentPC()
	c.emitABC(OP_TFORCALL, iterBase, 0, nVars, line)
	tforloopPos := c.emitAsBx(OP_TFORLOOP, iterBase+2, 0, line)
	exitJump := c.emitJump(line)
	bodyStart := c.currentPC()

	c.enterScope()
	if capturesRangeVar {
		keyReg := c.addLocal(s.Key)
		c.emitABC(OP_MOVE, keyReg, iterBase+3, 0, line)
		if s.Value != "" {
			valueReg := c.addLocal(s.Value)
			c.emitABC(OP_MOVE, valueReg, iterBase+4, 0, line)
		}
	}
	for _, st := range s.Body.Stmts {
		if err := c.compileStmt(st); err != nil {
			return err
		}
	}
	c.leaveScope()

	continueTarget := c.currentPC()
	loopBack := c.emitJump(line)
	c.patchJumpTo(loopBack, loopTop)

	tforloopOffset := bodyStart - tforloopPos - 1
	c.proto.Code[tforloopPos] = EncodeAsBx(OP_TFORLOOP, iterBase+2, tforloopOffset)

	c.patchJump(exitJump)

	info := c.popLoop()
	c.patchBreaks(info)
	c.patchContinues(info, continueTarget)

	c.leaveScope()
	if capturesRangeVar {
		c.nextReg = iterBase
	}
	return nil
}

func forRangeBodyCapturesVars(s *ast.ForRangeStmt) bool {
	if s == nil || s.Body == nil {
		return false
	}
	names := map[string]bool{s.Key: true}
	if s.Value != "" {
		names[s.Value] = true
	}
	return blockNestedFunctionCapturesAny(s.Body, names)
}

func blockNestedFunctionCapturesAny(block *ast.BlockStmt, names map[string]bool) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Stmts {
		if stmtNestedFunctionCapturesAny(stmt, names) {
			return true
		}
	}
	return false
}

func stmtNestedFunctionCapturesAny(stmt ast.Stmt, names map[string]bool) bool {
	switch s := stmt.(type) {
	case *ast.DeclareStmt:
		for _, value := range s.Values {
			if exprNestedFunctionCapturesAny(value, names) {
				return true
			}
		}
	case *ast.AssignStmt:
		for _, target := range s.Targets {
			if exprNestedFunctionCapturesAny(target, names) {
				return true
			}
		}
		for _, value := range s.Values {
			if exprNestedFunctionCapturesAny(value, names) {
				return true
			}
		}
	case *ast.CompoundAssignStmt:
		return exprNestedFunctionCapturesAny(s.Target, names) || exprNestedFunctionCapturesAny(s.Value, names)
	case *ast.IncDecStmt:
		return exprNestedFunctionCapturesAny(s.Target, names)
	case *ast.CallStmt:
		return exprNestedFunctionCapturesAny(s.Call, names)
	case *ast.GoStmt:
		return exprNestedFunctionCapturesAny(s.Call, names)
	case *ast.DeferStmt:
		return exprNestedFunctionCapturesAny(s.Call, names)
	case *ast.SendStmt:
		return exprNestedFunctionCapturesAny(s.Channel, names) || exprNestedFunctionCapturesAny(s.Value, names)
	case *ast.SelectStmt:
		for _, selCase := range s.Cases {
			if exprNestedFunctionCapturesAny(selCase.Channel, names) ||
				exprNestedFunctionCapturesAny(selCase.SendValue, names) ||
				blockNestedFunctionCapturesAny(selCase.Body, names) {
				return true
			}
		}
		return blockNestedFunctionCapturesAny(s.Default, names)
	case *ast.IfStmt:
		if exprNestedFunctionCapturesAny(s.Cond, names) || blockNestedFunctionCapturesAny(s.Body, names) {
			return true
		}
		for _, elseIf := range s.ElseIfs {
			if exprNestedFunctionCapturesAny(elseIf.Cond, names) || blockNestedFunctionCapturesAny(elseIf.Body, names) {
				return true
			}
		}
		return blockNestedFunctionCapturesAny(s.ElseBody, names)
	case *ast.ForNumStmt:
		return stmtNestedFunctionCapturesAny(s.Init, names) ||
			exprNestedFunctionCapturesAny(s.Cond, names) ||
			stmtNestedFunctionCapturesAny(s.Post, names) ||
			blockNestedFunctionCapturesAny(s.Body, names)
	case *ast.ForRangeStmt:
		return exprNestedFunctionCapturesAny(s.Iter, names) || blockNestedFunctionCapturesAny(s.Body, names)
	case *ast.ForStmt:
		return exprNestedFunctionCapturesAny(s.Cond, names) || blockNestedFunctionCapturesAny(s.Body, names)
	case *ast.ReturnStmt:
		for _, value := range s.Values {
			if exprNestedFunctionCapturesAny(value, names) {
				return true
			}
		}
	case *ast.FuncDeclStmt:
		return functionBodyCapturesAny(s.Body, s.Params, names)
	case *ast.ToolDeclStmt:
		return functionBodyCapturesAny(s.Body, s.Params, names)
	case *ast.AgentDeclStmt:
		return blockNestedFunctionCapturesAny(s.Flow, names)
	case *ast.AgentDefaultsDeclStmt:
		return configNestedFunctionCapturesAny(s.Config, names)
	case *ast.ModelsDeclStmt:
		return configNestedFunctionCapturesAny(s.Config, names)
	case *ast.BudgetStmt:
		return configNestedFunctionCapturesAny(s.Config, names) || blockNestedFunctionCapturesAny(s.Body, names)
	case *ast.EvaluateBlockStmt:
		return blockNestedFunctionCapturesAny(s.Body, names)
	case *ast.BlockStmt:
		return blockNestedFunctionCapturesAny(s, names)
	}
	return false
}

func functionBodyCapturesAny(body *ast.BlockStmt, params []ast.FuncParam, names map[string]bool) bool {
	paramNames := make([]string, 0, len(params))
	for _, param := range params {
		paramNames = append(paramNames, param.Name)
	}
	freeVars := runtime.FreeVars(body, paramNames)
	for _, name := range freeVars {
		if names[name] {
			return true
		}
	}
	return false
}

func exprNestedFunctionCapturesAny(expr ast.Expr, names map[string]bool) bool {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		return exprNestedFunctionCapturesAny(e.Left, names) || exprNestedFunctionCapturesAny(e.Right, names)
	case *ast.UnaryExpr:
		return exprNestedFunctionCapturesAny(e.Operand, names)
	case *ast.ParenExpr:
		return exprNestedFunctionCapturesAny(e.Inner, names)
	case *ast.IndexExpr:
		return exprNestedFunctionCapturesAny(e.Table, names) || exprNestedFunctionCapturesAny(e.Index, names)
	case *ast.FieldExpr:
		return exprNestedFunctionCapturesAny(e.Table, names)
	case *ast.CallExpr:
		if exprNestedFunctionCapturesAny(e.Func, names) {
			return true
		}
		for _, arg := range e.Args {
			if exprNestedFunctionCapturesAny(arg, names) {
				return true
			}
		}
	case *ast.MethodCallExpr:
		if exprNestedFunctionCapturesAny(e.Object, names) {
			return true
		}
		for _, arg := range e.Args {
			if exprNestedFunctionCapturesAny(arg, names) {
				return true
			}
		}
	case *ast.FuncLitExpr:
		return functionBodyCapturesAny(e.Body, e.Params, names)
	case *ast.AgentLitExpr:
		return configNestedFunctionCapturesAny(e.Config, names) || blockNestedFunctionCapturesAny(e.Flow, names)
	case *ast.TurnExpr:
		return configNestedFunctionCapturesAny(e.Config, names)
	case *ast.MessagesExpr:
		for _, field := range e.Fields {
			if exprNestedFunctionCapturesAny(field.Value, names) || exprNestedFunctionCapturesAny(field.Key, names) {
				return true
			}
		}
	case *ast.ListLitExpr:
		for _, value := range e.Values {
			if exprNestedFunctionCapturesAny(value, names) {
				return true
			}
		}
	case *ast.TableLitExpr:
		for _, field := range e.Fields {
			if exprNestedFunctionCapturesAny(field.Key, names) || exprNestedFunctionCapturesAny(field.Value, names) {
				return true
			}
		}
	case *ast.DenseLitExpr:
		for _, value := range e.Values {
			if exprNestedFunctionCapturesAny(value, names) {
				return true
			}
		}
	case *ast.RecvExpr:
		return exprNestedFunctionCapturesAny(e.Channel, names)
	case *ast.MakeChanExpr:
		return exprNestedFunctionCapturesAny(e.Size, names)
	}
	return false
}

func configNestedFunctionCapturesAny(config []ast.ConfigField, names map[string]bool) bool {
	for _, field := range config {
		if exprNestedFunctionCapturesAny(field.Key, names) || exprNestedFunctionCapturesAny(field.Value, names) {
			return true
		}
	}
	return false
}

// ---- ReturnStmt ----

func (c *compiler) compileReturnStmt(s *ast.ReturnStmt) error {
	line := s.P.Line
	nValues := len(s.Values)

	if nValues == 0 {
		c.emitReturn(0, 1)
		return nil
	}

	lastIsMulti := false
	switch v := s.Values[nValues-1].(type) {
	case *ast.CallExpr:
		lastIsMulti = !c.callExprKnownSingleResult(v)
	case *ast.MethodCallExpr, *ast.VarArgExpr:
		lastIsMulti = true
	}

	base := c.nextReg
	for i := 0; i < nValues; i++ {
		reg := c.allocReg()
		if i == nValues-1 && lastIsMulti {
			switch v := s.Values[i].(type) {
			case *ast.CallExpr:
				if err := c.compileCallExprMulti(v, reg, -1); err != nil {
					return err
				}
			case *ast.MethodCallExpr:
				if err := c.compileMethodCallExprMulti(v, reg, -1); err != nil {
					return err
				}
			case *ast.VarArgExpr:
				c.emitABC(OP_VARARG, reg, 0, 0, line)
			}
		} else {
			if err := c.compileExprTo(s.Values[i], reg); err != nil {
				return err
			}
		}
	}

	if lastIsMulti {
		c.emitReturn(base, 0)
	} else {
		c.emitReturn(base, nValues+1)
	}
	c.nextReg = base
	return nil
}

// ---- BreakStmt / ContinueStmt ----

func (c *compiler) compileBreakStmt(s *ast.BreakStmt) error {
	loop := c.currentLoop()
	if loop == nil {
		return fmt.Errorf("line %d: break outside loop", s.P.Line)
	}
	c.emitCloseForJump()
	jmp := c.emitJump(s.P.Line)
	loop.breakJumps = append(loop.breakJumps, jmp)
	return nil
}

func (c *compiler) compileContinueStmt(s *ast.ContinueStmt) error {
	loop := c.currentLoop()
	if loop == nil {
		return fmt.Errorf("line %d: continue outside loop", s.P.Line)
	}
	c.emitCloseForJump()
	jmp := c.emitJump(s.P.Line)
	loop.continueJumps = append(loop.continueJumps, jmp)
	return nil
}

func (c *compiler) compileLabelStmt(s *ast.LabelStmt) error {
	c.labels[s.Name] = compiledLabel{pc: c.currentPC(), depth: c.depth, line: s.P.Line}
	return nil
}

func (c *compiler) compileGotoStmt(s *ast.GotoStmt) error {
	if label, ok := c.labels[s.Name]; ok {
		c.emitCloseCapturedAboveDepth(label.depth)
		jmp := c.emitJump(s.P.Line)
		c.patchJumpTo(jmp, label.pc)
		return nil
	}
	if targetDepth, ok := c.labelDepths[s.Name]; ok {
		c.emitCloseCapturedAboveDepth(targetDepth)
	}
	jmp := c.emitJump(s.P.Line)
	c.gotos = append(c.gotos, compiledGoto{name: s.Name, pc: jmp, depth: c.depth, line: s.P.Line})
	return nil
}

// ---- FuncDeclStmt ----

func (c *compiler) compileFuncDeclStmt(s *ast.FuncDeclStmt) error {
	line := s.P.Line
	protoIdx, err := c.compileFunction(s.Name, s.Params, s.Body, line)
	if err != nil {
		return err
	}

	reg := c.resolveLocal(s.Name)
	if reg >= 0 {
		c.emitABx(OP_CLOSURE, reg, protoIdx, line)
		return nil
	}
	upIdx := c.resolveUpvalue(s.Name)
	if upIdx >= 0 {
		tmpReg := c.allocReg()
		c.emitABx(OP_CLOSURE, tmpReg, protoIdx, line)
		c.emitABC(OP_SETUPVAL, tmpReg, upIdx, 0, line)
		c.freeReg()
		return nil
	}
	// Global
	tmpReg := c.allocReg()
	c.emitABx(OP_CLOSURE, tmpReg, protoIdx, line)
	nameK := c.stringConst(s.Name)
	c.emitABx(OP_SETGLOBAL, tmpReg, nameK, line)
	c.freeReg()
	return nil
}

// ---- BlockStmt ----

func (c *compiler) compileBlockStmt(s *ast.BlockStmt) error {
	c.enterScope()
	c.collectFunctionArities(s.Stmts)
	for _, st := range s.Stmts {
		if err := c.compileStmt(st); err != nil {
			return err
		}
	}
	c.leaveScope()
	return nil
}

// --------------------------------------------------------------------
// Expression compilation
// --------------------------------------------------------------------
