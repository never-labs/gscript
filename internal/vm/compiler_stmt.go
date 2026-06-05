package vm

// compiler_stmt.go — statement compilation for the AST→bytecode compiler:
// the compileStmt dispatcher plus declaration, assignment, compound-assign,
// inc/dec, call, go, defer, send, receive and make-channel statements.
// Pure code movement from compiler.go; declarations are verbatim.

import (
	"fmt"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/runtime"
)

// --------------------------------------------------------------------
// Statement compilation
// --------------------------------------------------------------------

func (c *compiler) compileStmt(stmt ast.Stmt) error {
	switch s := stmt.(type) {
	case *ast.DeclareStmt:
		return c.compileDeclareStmt(s)
	case *ast.AssignStmt:
		return c.compileAssignStmt(s)
	case *ast.CompoundAssignStmt:
		return c.compileCompoundAssignStmt(s)
	case *ast.IncDecStmt:
		return c.compileIncDecStmt(s)
	case *ast.CallStmt:
		return c.compileCallStmt(s)
	case *ast.IfStmt:
		return c.compileIfStmt(s)
	case *ast.ForNumStmt:
		return c.compileForNumStmt(s)
	case *ast.ForStmt:
		return c.compileForStmt(s)
	case *ast.ForRangeStmt:
		return c.compileForRangeStmt(s)
	case *ast.ReturnStmt:
		return c.compileReturnStmt(s)
	case *ast.BreakStmt:
		return c.compileBreakStmt(s)
	case *ast.ContinueStmt:
		return c.compileContinueStmt(s)
	case *ast.LabelStmt:
		return c.compileLabelStmt(s)
	case *ast.GotoStmt:
		return c.compileGotoStmt(s)
	case *ast.FuncDeclStmt:
		return c.compileFuncDeclStmt(s)
	case *ast.BlockStmt:
		return c.compileBlockStmt(s)
	case *ast.GoStmt:
		return c.compileGoStmt(s)
	case *ast.DeferStmt:
		return c.compileDeferStmt(s)
	case *ast.SendStmt:
		return c.compileSendStmt(s)
	case *ast.SelectStmt:
		return c.compileSelectStmt(s)
	default:
		return fmt.Errorf("line %d: unsupported statement type %T", stmt.GetPos().Line, stmt)
	}
}

// ---- DeclareStmt ----

func (c *compiler) compileDeclareStmt(s *ast.DeclareStmt) error {
	if c.isMainTopLevel() {
		return c.compileDeclareGlobals(s)
	}

	nNames := len(s.Names)
	nValues := len(s.Values)

	if nValues == 1 && nNames > 1 {
		if recv, ok := s.Values[0].(*ast.RecvExpr); ok && nNames == 2 {
			return c.compileDeclareRecvOK(s, recv)
		}
		if call, ok := s.Values[0].(*ast.CallExpr); ok {
			return c.compileDeclareMultiCall(s, call)
		}
		if call, ok := s.Values[0].(*ast.MethodCallExpr); ok {
			return c.compileDeclareMultiMethodCall(s, call)
		}
	}

	tempBase := c.nextReg
	if err := c.compileAdjustedExprList(s.Values, nNames, s.P.Line); err != nil {
		return err
	}
	c.freeRegs(nNames)

	for i, name := range s.Names {
		reg := c.addLocalWithReadOnly(name, s.ReadOnly)
		if reg != tempBase+i {
			c.emitABC(OP_MOVE, reg, tempBase+i, 0, s.P.Line)
		}
	}
	return nil
}

func (c *compiler) compileDeclareGlobals(s *ast.DeclareStmt) error {
	nNames := len(s.Names)
	nValues := len(s.Values)

	if nValues == 1 && nNames > 1 {
		if recv, ok := s.Values[0].(*ast.RecvExpr); ok && nNames == 2 {
			tempBase := c.nextReg
			if err := c.compileRecvOKExpr(recv, tempBase, tempBase+1); err != nil {
				return err
			}
			for i, name := range s.Names {
				nameK := c.stringConst(name)
				if s.ReadOnly {
					c.emitABx(OP_SETGLOBALRO, tempBase+i, nameK, s.P.Line)
				} else {
					c.emitABx(OP_SETGLOBAL, tempBase+i, nameK, s.P.Line)
				}
			}
			c.nextReg = tempBase
			return nil
		}
		if call, ok := s.Values[0].(*ast.CallExpr); ok {
			tempBase := c.nextReg
			if err := c.compileCallExprMulti(call, tempBase, nNames); err != nil {
				return err
			}
			c.nextReg = tempBase + nNames
			if c.nextReg > c.maxReg {
				c.maxReg = c.nextReg
			}
			for i, name := range s.Names {
				nameK := c.stringConst(name)
				if s.ReadOnly {
					c.emitABx(OP_SETGLOBALRO, tempBase+i, nameK, s.P.Line)
				} else {
					c.emitABx(OP_SETGLOBAL, tempBase+i, nameK, s.P.Line)
				}
			}
			c.nextReg = tempBase
			return nil
		}
		if call, ok := s.Values[0].(*ast.MethodCallExpr); ok {
			tempBase := c.nextReg
			if err := c.compileMethodCallExprMulti(call, tempBase, nNames); err != nil {
				return err
			}
			c.nextReg = tempBase + nNames
			if c.nextReg > c.maxReg {
				c.maxReg = c.nextReg
			}
			for i, name := range s.Names {
				nameK := c.stringConst(name)
				if s.ReadOnly {
					c.emitABx(OP_SETGLOBALRO, tempBase+i, nameK, s.P.Line)
				} else {
					c.emitABx(OP_SETGLOBAL, tempBase+i, nameK, s.P.Line)
				}
			}
			c.nextReg = tempBase
			return nil
		}
	}

	tempBase := c.nextReg
	if err := c.compileAdjustedExprList(s.Values, nNames, s.P.Line); err != nil {
		return err
	}
	for i := 0; i < nNames; i++ {
		reg := tempBase + i
		nameK := c.stringConst(s.Names[i])
		if s.ReadOnly {
			c.emitABx(OP_SETGLOBALRO, reg, nameK, s.P.Line)
		} else {
			c.emitABx(OP_SETGLOBAL, reg, nameK, s.P.Line)
		}
	}
	c.nextReg = tempBase
	return nil
}

func (c *compiler) compileDeclareMultiCall(s *ast.DeclareStmt, call *ast.CallExpr) error {
	nNames := len(s.Names)
	base := c.nextReg
	if err := c.compileCallExprMulti(call, base, nNames); err != nil {
		return err
	}
	// Results are in registers base..base+nNames-1.
	// Reset nextReg so addLocal allocates those exact registers.
	c.nextReg = base
	for _, name := range s.Names {
		c.addLocalWithReadOnly(name, s.ReadOnly)
	}
	return nil
}

func (c *compiler) compileDeclareMultiMethodCall(s *ast.DeclareStmt, call *ast.MethodCallExpr) error {
	nNames := len(s.Names)
	base := c.nextReg
	if err := c.compileMethodCallExprMulti(call, base, nNames); err != nil {
		return err
	}
	// Results are in registers base..base+nNames-1.
	c.nextReg = base
	for _, name := range s.Names {
		c.addLocalWithReadOnly(name, s.ReadOnly)
	}
	return nil
}

// ---- AssignStmt ----

func (c *compiler) compileAssignStmt(s *ast.AssignStmt) error {
	nTargets := len(s.Targets)
	nValues := len(s.Values)

	if nValues == 1 && nTargets > 1 {
		if recv, ok := s.Values[0].(*ast.RecvExpr); ok && nTargets == 2 {
			return c.compileAssignRecvOK(s, recv)
		}
		if call, ok := s.Values[0].(*ast.CallExpr); ok {
			return c.compileAssignMultiCall(s, call)
		}
		if call, ok := s.Values[0].(*ast.MethodCallExpr); ok {
			return c.compileAssignMultiMethodCall(s, call)
		}
	}

	// Evaluate all values into temp registers. Keep them allocated
	// during the assignment phase so they don't get overwritten.
	tempBase := c.nextReg
	if err := c.compileAdjustedExprList(s.Values, nTargets, s.P.Line); err != nil {
		return err
	}
	// Do NOT free value registers yet — assignments may allocate temp
	// registers for table/field targets, which would overlap.

	for i, target := range s.Targets {
		if err := c.compileAssignTarget(target, tempBase+i, s.P.Line); err != nil {
			return err
		}
	}
	// Now free all value registers
	c.nextReg = tempBase
	return nil
}

func (c *compiler) compileAdjustedExprList(values []ast.Expr, nResults int, line int) error {
	for i := 0; i < nResults; i++ {
		reg := c.allocReg()
		if i >= len(values) {
			c.emitABC(OP_LOADNIL, reg, 0, 0, line)
			continue
		}
		if i == len(values)-1 {
			switch v := values[i].(type) {
			case *ast.RecvExpr:
				if nResults-i >= 2 {
					okReg := c.allocReg()
					if okReg != reg+1 {
						return fmt.Errorf("line %d: internal register allocation error for receive ok", line)
					}
					return c.compileRecvOKExpr(v, reg, okReg)
				}
			case *ast.CallExpr:
				return c.compileCallExprMulti(v, reg, nResults-i)
			case *ast.MethodCallExpr:
				return c.compileMethodCallExprMulti(v, reg, nResults-i)
			case *ast.VarArgExpr:
				c.emitABC(OP_VARARG, reg, nResults-i+1, 0, line)
				c.nextReg = reg + (nResults - i)
				if c.nextReg > c.maxReg {
					c.maxReg = c.nextReg
				}
				return nil
			}
		}
		if err := c.compileExprTo(values[i], reg); err != nil {
			return err
		}
		c.nextReg = reg + 1
	}
	return nil
}

func (c *compiler) compileDeclareRecvOK(s *ast.DeclareStmt, recv *ast.RecvExpr) error {
	tempBase := c.nextReg
	if err := c.compileRecvOKExpr(recv, tempBase, tempBase+1); err != nil {
		return err
	}
	c.nextReg = tempBase + 2
	if c.nextReg > c.maxReg {
		c.maxReg = c.nextReg
	}
	c.freeRegs(2)
	for i, name := range s.Names {
		reg := c.addLocalWithReadOnly(name, s.ReadOnly)
		if reg != tempBase+i {
			c.emitABC(OP_MOVE, reg, tempBase+i, 0, s.P.Line)
		}
	}
	return nil
}

func (c *compiler) compileAssignRecvOK(s *ast.AssignStmt, recv *ast.RecvExpr) error {
	tempBase := c.nextReg
	if err := c.compileRecvOKExpr(recv, tempBase, tempBase+1); err != nil {
		return err
	}
	c.nextReg = tempBase + 2
	if c.nextReg > c.maxReg {
		c.maxReg = c.nextReg
	}
	for i, target := range s.Targets {
		if err := c.compileAssignTarget(target, tempBase+i, s.P.Line); err != nil {
			return err
		}
	}
	c.nextReg = tempBase
	return nil
}

func (c *compiler) compileAssignMultiCall(s *ast.AssignStmt, call *ast.CallExpr) error {
	nTargets := len(s.Targets)
	tempBase := c.nextReg
	if err := c.compileCallExprMulti(call, tempBase, nTargets); err != nil {
		return err
	}
	c.nextReg = tempBase + nTargets
	if c.nextReg > c.maxReg {
		c.maxReg = c.nextReg
	}
	c.freeRegs(nTargets)
	for i, target := range s.Targets {
		if err := c.compileAssignTarget(target, tempBase+i, s.P.Line); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileAssignMultiMethodCall(s *ast.AssignStmt, call *ast.MethodCallExpr) error {
	nTargets := len(s.Targets)
	tempBase := c.nextReg
	if err := c.compileMethodCallExprMulti(call, tempBase, nTargets); err != nil {
		return err
	}
	c.nextReg = tempBase + nTargets
	if c.nextReg > c.maxReg {
		c.maxReg = c.nextReg
	}
	c.freeRegs(nTargets)
	for i, target := range s.Targets {
		if err := c.compileAssignTarget(target, tempBase+i, s.P.Line); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileAssignTarget(target ast.Expr, valueReg int, line int) error {
	switch t := target.(type) {
	case *ast.IdentExpr:
		reg, readOnly := c.resolveLocalInfo(t.Name)
		if reg >= 0 {
			if readOnly {
				c.emitABx(OP_CHECKCONST, reg, c.stringConst(t.Name), line)
			}
			if reg != valueReg {
				c.emitABC(OP_MOVE, reg, valueReg, 0, line)
			}
			return nil
		}
		upIdx, _ := c.resolveUpvalueInfo(t.Name)
		if upIdx >= 0 {
			c.emitABC(OP_SETUPVAL, valueReg, upIdx, 0, line)
			return nil
		}
		nameK := c.stringConst(t.Name)
		c.emitABx(OP_SETGLOBAL, valueReg, nameK, line)
		return nil

	case *ast.IndexExpr:
		tableReg, tableIsTemp, err := c.compileExprReg(t.Table)
		if err != nil {
			return err
		}
		keyReg := c.allocReg()
		if err := c.compileExprTo(t.Index, keyReg); err != nil {
			return err
		}
		c.emitABC(OP_SETTABLE, tableReg, keyReg, valueReg, line)
		c.freeReg() // keyReg
		if tableIsTemp {
			c.freeReg()
		}
		return nil

	case *ast.FieldExpr:
		tableReg, tableIsTemp, err := c.compileExprReg(t.Table)
		if err != nil {
			return err
		}
		fieldK := c.stringConst(t.Field)
		c.emitABC(OP_SETFIELD, tableReg, fieldK, valueReg, line)
		if tableIsTemp {
			c.freeReg()
		}
		return nil

	default:
		return fmt.Errorf("line %d: invalid assignment target %T", line, target)
	}
}

// ---- CompoundAssignStmt ----

func (c *compiler) compileCompoundAssignStmt(s *ast.CompoundAssignStmt) error {
	line := s.P.Line

	var opcode Opcode
	switch s.Op {
	case "+=":
		opcode = OP_ADD
	case "-=":
		opcode = OP_SUB
	case "*=":
		opcode = OP_MUL
	case "/=":
		opcode = OP_DIV
	case "%=":
		opcode = OP_MOD
	case "**=":
		opcode = OP_POW
	case "..=":
		opcode = OP_CONCAT
	default:
		return fmt.Errorf("line %d: unsupported compound assignment operator %s", line, s.Op)
	}

	switch t := s.Target.(type) {
	case *ast.IdentExpr:
		reg, readOnly := c.resolveLocalInfo(t.Name)
		if reg >= 0 {
			if readOnly {
				c.emitABx(OP_CHECKCONST, reg, c.stringConst(t.Name), line)
			}
			return c.emitCompoundOp(opcode, reg, s.Value, line,
				func(resultReg int) { /* result already in reg */ })
		}
		upIdx, _ := c.resolveUpvalueInfo(t.Name)
		if upIdx >= 0 {
			tmp := c.allocReg()
			c.emitABC(OP_GETUPVAL, tmp, upIdx, 0, line)
			err := c.emitCompoundOp(opcode, tmp, s.Value, line, func(int) {})
			if err != nil {
				return err
			}
			c.emitABC(OP_SETUPVAL, tmp, upIdx, 0, line)
			c.freeReg()
			return nil
		}
		nameK := c.stringConst(t.Name)
		tmp := c.allocReg()
		c.emitABx(OP_GETGLOBAL, tmp, nameK, line)
		err := c.emitCompoundOp(opcode, tmp, s.Value, line, func(int) {})
		if err != nil {
			return err
		}
		c.emitABx(OP_SETGLOBAL, tmp, nameK, line)
		c.freeReg()
		return nil

	case *ast.IndexExpr:
		tableReg, tableIsTemp, err := c.compileExprReg(t.Table)
		if err != nil {
			return err
		}
		keyReg := c.allocReg()
		if err := c.compileExprTo(t.Index, keyReg); err != nil {
			return err
		}
		oldReg := c.allocReg()
		c.emitABC(OP_GETTABLE, oldReg, tableReg, keyReg, line)
		err = c.emitCompoundOp(opcode, oldReg, s.Value, line, func(int) {})
		if err != nil {
			return err
		}
		c.emitABC(OP_SETTABLE, tableReg, keyReg, oldReg, line)
		c.freeRegs(2) // oldReg, keyReg
		if tableIsTemp {
			c.freeReg()
		}
		return nil

	case *ast.FieldExpr:
		tableReg, tableIsTemp, err := c.compileExprReg(t.Table)
		if err != nil {
			return err
		}
		fieldK := c.stringConst(t.Field)
		oldReg := c.allocReg()
		c.emitABC(OP_GETFIELD, oldReg, tableReg, fieldK, line)
		err = c.emitCompoundOp(opcode, oldReg, s.Value, line, func(int) {})
		if err != nil {
			return err
		}
		c.emitABC(OP_SETFIELD, tableReg, fieldK, oldReg, line)
		c.freeReg() // oldReg
		if tableIsTemp {
			c.freeReg()
		}
		return nil

	default:
		return fmt.Errorf("line %d: invalid compound assignment target %T", line, s.Target)
	}
}

// emitCompoundOp performs: targetReg = targetReg OP value
func (c *compiler) emitCompoundOp(opcode Opcode, targetReg int, value ast.Expr, line int, _ func(int)) error {
	if opcode == OP_CONCAT {
		tmpB := c.allocReg()
		c.emitABC(OP_MOVE, tmpB, targetReg, 0, line)
		tmpC := c.allocReg()
		if err := c.compileExprTo(value, tmpC); err != nil {
			return err
		}
		c.emitABC(OP_CONCAT, targetReg, tmpB, tmpC, line)
		c.freeRegs(2)
	} else {
		valReg := c.allocReg()
		if err := c.compileExprTo(value, valReg); err != nil {
			return err
		}
		c.emitABC(opcode, targetReg, targetReg, valReg, line)
		c.freeReg()
	}
	return nil
}

// ---- IncDecStmt ----

func (c *compiler) compileIncDecStmt(s *ast.IncDecStmt) error {
	line := s.P.Line
	var opcode Opcode
	if s.Op == "++" {
		opcode = OP_ADD
	} else {
		opcode = OP_SUB
	}

	// Load constant 1 into a temp register
	oneReg := c.allocReg()
	c.emitAsBx(OP_LOADINT, oneReg, 1, line)

	switch t := s.Target.(type) {
	case *ast.IdentExpr:
		reg, readOnly := c.resolveLocalInfo(t.Name)
		if reg >= 0 {
			if readOnly {
				c.emitABx(OP_CHECKCONST, reg, c.stringConst(t.Name), line)
			}
			c.emitABC(opcode, reg, reg, oneReg, line)
			c.freeReg() // oneReg
			return nil
		}
		upIdx, _ := c.resolveUpvalueInfo(t.Name)
		if upIdx >= 0 {
			tmp := c.allocReg()
			c.emitABC(OP_GETUPVAL, tmp, upIdx, 0, line)
			c.emitABC(opcode, tmp, tmp, oneReg, line)
			c.emitABC(OP_SETUPVAL, tmp, upIdx, 0, line)
			c.freeReg() // tmp
			c.freeReg() // oneReg
			return nil
		}
		nameK := c.stringConst(t.Name)
		tmp := c.allocReg()
		c.emitABx(OP_GETGLOBAL, tmp, nameK, line)
		c.emitABC(opcode, tmp, tmp, oneReg, line)
		c.emitABx(OP_SETGLOBAL, tmp, nameK, line)
		c.freeReg() // tmp
		c.freeReg() // oneReg
		return nil

	case *ast.IndexExpr:
		tableReg := c.allocReg()
		if err := c.compileExprTo(t.Table, tableReg); err != nil {
			return err
		}
		keyReg := c.allocReg()
		if err := c.compileExprTo(t.Index, keyReg); err != nil {
			return err
		}
		oldReg := c.allocReg()
		c.emitABC(OP_GETTABLE, oldReg, tableReg, keyReg, line)
		c.emitABC(opcode, oldReg, oldReg, oneReg, line)
		c.emitABC(OP_SETTABLE, tableReg, keyReg, oldReg, line)
		c.freeRegs(3) // oldReg, keyReg, tableReg
		c.freeReg()   // oneReg
		return nil

	case *ast.FieldExpr:
		tableReg := c.allocReg()
		if err := c.compileExprTo(t.Table, tableReg); err != nil {
			return err
		}
		fieldK := c.stringConst(t.Field)
		oldReg := c.allocReg()
		c.emitABC(OP_GETFIELD, oldReg, tableReg, fieldK, line)
		c.emitABC(opcode, oldReg, oldReg, oneReg, line)
		c.emitABC(OP_SETFIELD, tableReg, fieldK, oldReg, line)
		c.freeRegs(2) // oldReg, tableReg
		c.freeReg()   // oneReg
		return nil

	default:
		c.freeReg() // oneReg
		return fmt.Errorf("line %d: invalid inc/dec target %T", line, s.Target)
	}
}

// ---- CallStmt ----

func (c *compiler) compileCallStmt(s *ast.CallStmt) error {
	return c.compileCallExprDiscard(s.Call, s.P.Line)
}

func (c *compiler) compileCallExprDiscard(expr ast.Expr, line int) error {
	switch call := expr.(type) {
	case *ast.CallExpr:
		return c.compilePlainCallExprDiscard(call, line)
	case *ast.MethodCallExpr:
		return c.compileMethodCallExprMulti(call, c.nextReg, 0)
	default:
		return fmt.Errorf("line %d: call statement requires a function call", line)
	}
}

func (c *compiler) compilePlainCallExprDiscard(call *ast.CallExpr, line int) error {
	if hasExplicitSpread(call.Args) {
		return c.compileCallExprMulti(call, c.nextReg, 0)
	}
	if op, ok := c.staticCoroutineBuiltinOp(call); ok {
		return c.compileCoroutineBuiltinCall(call, c.nextReg, -1, line, op)
	}
	base := c.nextReg
	funcReg := c.allocReg()
	if err := c.compileExprTo(call.Func, funcReg); err != nil {
		return err
	}
	nArgs := len(call.Args)
	lastArgIsMulti := false
	for i, arg := range call.Args {
		argReg := c.allocReg()
		if i == nArgs-1 {
			switch a := arg.(type) {
			case *ast.CallExpr:
				if c.callExprKnownSingleResult(a) {
					if err := c.compileExprTo(a, argReg); err != nil {
						return err
					}
					continue
				}
				lastArgIsMulti = true
				c.proto.JITDisabled = true
				if err := c.compileCallExprMulti(a, argReg, -1); err != nil {
					return err
				}
				continue
			case *ast.MethodCallExpr:
				lastArgIsMulti = true
				c.proto.JITDisabled = true
				if err := c.compileMethodCallExprMulti(a, argReg, -1); err != nil {
					return err
				}
				continue
			case *ast.VarArgExpr:
				lastArgIsMulti = true
				c.emitABC(OP_VARARG, argReg, 0, 0, line)
				continue
			}
		}
		if err := c.compileExprTo(arg, argReg); err != nil {
			return err
		}
	}
	b := nArgs + 1
	if lastArgIsMulti {
		b = 0
	}
	c.emitABC(OP_CALL, funcReg, b, 1, line) // C=1: discard results
	c.nextReg = base
	return nil
}

// ---- GoStmt ----

func (c *compiler) compileGoStmt(s *ast.GoStmt) error {
	line := s.P.Line
	switch call := s.Call.(type) {
	case *ast.CallExpr:
		return c.compileGoCallExpr(call, line)
	case *ast.MethodCallExpr:
		return c.compileGoMethodCallExpr(call, line)
	default:
		return fmt.Errorf("line %d: go statement requires a function call", line)
	}
}

func (c *compiler) compileGoCallExpr(call *ast.CallExpr, line int) error {
	base := c.nextReg
	funcReg := c.allocReg()
	if err := c.compileExprTo(call.Func, funcReg); err != nil {
		return err
	}
	nArgs := len(call.Args)
	lastArgIsMulti := false
	for i, arg := range call.Args {
		argReg := c.allocReg()
		if i == nArgs-1 {
			switch a := arg.(type) {
			case *ast.CallExpr:
				lastArgIsMulti = true
				c.proto.JITDisabled = true
				if err := c.compileCallExprMulti(a, argReg, -1); err != nil {
					return err
				}
				continue
			case *ast.MethodCallExpr:
				lastArgIsMulti = true
				c.proto.JITDisabled = true
				if err := c.compileMethodCallExprMulti(a, argReg, -1); err != nil {
					return err
				}
				continue
			case *ast.VarArgExpr:
				lastArgIsMulti = true
				c.emitABC(OP_VARARG, argReg, 0, 0, line)
				continue
			}
		}
		if err := c.compileExprTo(arg, argReg); err != nil {
			return err
		}
	}
	b := nArgs + 1
	if lastArgIsMulti {
		b = 0
	}
	c.emitABC(OP_GO, funcReg, b, 0, line)
	c.nextReg = base
	return nil
}

func (c *compiler) compileGoMethodCallExpr(call *ast.MethodCallExpr, line int) error {
	base := c.nextReg
	selfReg := c.allocReg()
	c.allocReg() // reserve selfReg+1 for receiver

	objReg := c.allocReg()
	if err := c.compileExprTo(call.Object, objReg); err != nil {
		return err
	}
	c.emitMethodLookup(selfReg, objReg, c.stringConst(call.Method), line)

	nArgs := len(call.Args)
	for _, arg := range call.Args {
		argReg := c.allocReg()
		if err := c.compileExprTo(arg, argReg); err != nil {
			return err
		}
	}
	b := nArgs + 2 // +1 for self, +1 for encoding
	c.emitABC(OP_GO, selfReg, b, 0, line)
	c.nextReg = base
	return nil
}

// ---- DeferStmt ----

func (c *compiler) compileDeferStmt(s *ast.DeferStmt) error {
	line := s.P.Line
	c.proto.JITDisabled = true
	switch call := s.Call.(type) {
	case *ast.CallExpr:
		return c.compileDeferCallExpr(call, line)
	case *ast.MethodCallExpr:
		return c.compileDeferMethodCallExpr(call, line)
	default:
		return fmt.Errorf("line %d: defer statement requires a function call", line)
	}
}

func (c *compiler) compileDeferCallExpr(call *ast.CallExpr, line int) error {
	base := c.nextReg
	funcReg := c.allocReg()
	if err := c.compileExprTo(call.Func, funcReg); err != nil {
		return err
	}
	nArgs := len(call.Args)
	lastArgIsMulti := false
	for i, arg := range call.Args {
		argReg := c.allocReg()
		if i == nArgs-1 {
			switch a := arg.(type) {
			case *ast.CallExpr:
				lastArgIsMulti = true
				c.proto.JITDisabled = true
				if err := c.compileCallExprMulti(a, argReg, -1); err != nil {
					return err
				}
				continue
			case *ast.MethodCallExpr:
				lastArgIsMulti = true
				c.proto.JITDisabled = true
				if err := c.compileMethodCallExprMulti(a, argReg, -1); err != nil {
					return err
				}
				continue
			case *ast.VarArgExpr:
				lastArgIsMulti = true
				c.emitABC(OP_VARARG, argReg, 0, 0, line)
				continue
			}
		}
		if err := c.compileExprTo(arg, argReg); err != nil {
			return err
		}
	}
	b := nArgs + 1
	if lastArgIsMulti {
		b = 0
	}
	c.emitABC(OP_DEFER, funcReg, b, 0, line)
	c.nextReg = base
	return nil
}

func (c *compiler) compileDeferMethodCallExpr(call *ast.MethodCallExpr, line int) error {
	base := c.nextReg
	selfReg := c.allocReg()
	c.allocReg()
	objReg := c.allocReg()
	if err := c.compileExprTo(call.Object, objReg); err != nil {
		return err
	}
	c.emitMethodLookup(selfReg, objReg, c.stringConst(call.Method), line)
	c.freeReg()

	for _, arg := range call.Args {
		argReg := c.allocReg()
		if err := c.compileExprTo(arg, argReg); err != nil {
			return err
		}
	}
	c.emitABC(OP_DEFER, selfReg, len(call.Args)+2, 0, line)
	c.nextReg = base
	return nil
}

// ---- Channel operations ----

func (c *compiler) compileSendStmt(s *ast.SendStmt) error {
	line := s.P.Line
	base := c.nextReg

	// Special case: standalone <-ch (recv as statement, discard result)
	if recvExpr, ok := s.Channel.(*ast.RecvExpr); ok && s.Value == nil {
		chReg := c.allocReg()
		if err := c.compileExprTo(recvExpr.Channel, chReg); err != nil {
			return err
		}
		// Recv into a temp register (discarded)
		c.emitABC(OP_RECV, chReg, chReg, 0, line)
		c.nextReg = base
		return nil
	}

	chReg := c.allocReg()
	if err := c.compileExprTo(s.Channel, chReg); err != nil {
		return err
	}
	valReg := c.allocReg()
	if err := c.compileExprTo(s.Value, valReg); err != nil {
		return err
	}
	c.emitABC(OP_SEND, chReg, valReg, 0, line)
	c.nextReg = base
	return nil
}

func (c *compiler) compileRecvExpr(e *ast.RecvExpr, dest int) error {
	line := e.P.Line
	base := c.nextReg
	chReg := c.allocReg()
	if err := c.compileExprTo(e.Channel, chReg); err != nil {
		return err
	}
	c.emitABC(OP_RECV, dest, chReg, 0, line)
	c.nextReg = base
	return nil
}

func (c *compiler) compileRecvOKExpr(e *ast.RecvExpr, dest, okDest int) error {
	line := e.P.Line
	base := c.nextReg
	if c.nextReg <= dest {
		c.nextReg = dest + 1
	}
	if c.nextReg <= okDest {
		c.nextReg = okDest + 1
	}
	if c.nextReg > c.maxReg {
		c.maxReg = c.nextReg
	}
	chReg := c.allocReg()
	if err := c.compileExprTo(e.Channel, chReg); err != nil {
		return err
	}
	c.emitABC(OP_RECVOK, dest, chReg, okDest, line)
	c.nextReg = base
	return nil
}

func (c *compiler) compileMakeChanExpr(e *ast.MakeChanExpr, dest int) error {
	line := e.P.Line
	if e.Size != nil {
		base := c.nextReg
		sizeReg := c.allocReg()
		if err := c.compileExprTo(e.Size, sizeReg); err != nil {
			return err
		}
		c.emitABC(OP_MAKECHAN, dest, sizeReg, 1, line) // C=1 means size is in R(B)
		c.nextReg = base
	} else {
		c.emitABC(OP_MAKECHAN, dest, 0, 0, line) // C=0 means unbuffered
	}
	return nil
}

func (c *compiler) compileSelectStmt(s *ast.SelectStmt) error {
	if s.Default == nil {
		return c.compileBlockingSelectStmt(s)
	}
	return c.compileNonblockingSelectStmt(s)
}

func (c *compiler) compileNonblockingSelectStmt(s *ast.SelectStmt) error {
	line := s.P.Line
	base := c.nextReg
	var endJumps []int

	for _, cls := range s.Cases {
		caseBase := c.nextReg
		readyReg := c.allocReg()
		var recvReg int
		var recvOKReg int
		if cls.SendValue == nil {
			recvReg = c.allocReg()
			if cls.RecvOkName != "" {
				if c.allocReg() != recvReg+1 {
					return fmt.Errorf("line %d: internal register allocation error for select receive ok", cls.P.Line)
				}
				recvOKReg = c.allocReg()
			}
			chReg := c.allocReg()
			if err := c.compileExprTo(cls.Channel, chReg); err != nil {
				return err
			}
			if cls.RecvOkName != "" {
				c.emitABC(OP_TRYRECVOK, recvReg, chReg, recvOKReg, cls.P.Line)
				readyReg = recvReg + 1
			} else {
				c.emitABC(OP_TRYRECV, recvReg, chReg, readyReg, cls.P.Line)
			}
		} else {
			chReg := c.allocReg()
			if err := c.compileExprTo(cls.Channel, chReg); err != nil {
				return err
			}
			valReg := c.allocReg()
			if err := c.compileExprTo(cls.SendValue, valReg); err != nil {
				return err
			}
			c.emitABC(OP_TRYSEND, chReg, valReg, readyReg, cls.P.Line)
		}
		c.nextReg = caseBase + 1
		c.emitABC(OP_TEST, readyReg, 0, 0, cls.P.Line)
		skipJump := c.emitJump(cls.P.Line)

		c.enterScope()
		if cls.SendValue == nil && cls.RecvName != "" {
			localReg := c.addLocal(cls.RecvName)
			c.emitABC(OP_MOVE, localReg, recvReg, 0, cls.P.Line)
		}
		if cls.SendValue == nil && cls.RecvOkName != "" {
			localReg := c.addLocal(cls.RecvOkName)
			c.emitABC(OP_MOVE, localReg, recvOKReg, 0, cls.P.Line)
		}
		for _, st := range cls.Body.Stmts {
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		c.leaveScope()
		endJumps = append(endJumps, c.emitJump(line))
		c.patchJump(skipJump)
		c.nextReg = base
	}

	if s.Default != nil {
		c.enterScope()
		for _, st := range s.Default.Stmts {
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		c.leaveScope()
	}
	for _, j := range endJumps {
		c.patchJump(j)
	}
	c.nextReg = base
	return nil
}

func (c *compiler) compileBlockingSelectStmt(s *ast.SelectStmt) error {
	if len(s.Cases) == 0 {
		return fmt.Errorf("line %d: select without cases requires a default clause", s.P.Line)
	}
	if len(s.Cases) > 85 {
		return fmt.Errorf("line %d: select supports at most 85 cases", s.P.Line)
	}

	line := s.P.Line
	base := c.nextReg
	selectedReg := c.allocReg()
	recvReg := c.allocReg()
	recvOKReg := c.allocReg()
	caseBase := c.nextReg

	for _, cls := range s.Cases {
		modeReg := c.allocReg()
		chReg := c.allocReg()
		valReg := c.allocReg()
		if cls.SendValue == nil {
			c.emitAsBx(OP_LOADINT, modeReg, int(runtime.ChannelSelectRecv), cls.P.Line)
			if err := c.compileExprTo(cls.Channel, chReg); err != nil {
				return err
			}
			c.emitABC(OP_LOADNIL, valReg, 0, 0, cls.P.Line)
		} else {
			c.emitAsBx(OP_LOADINT, modeReg, int(runtime.ChannelSelectSend), cls.P.Line)
			if err := c.compileExprTo(cls.Channel, chReg); err != nil {
				return err
			}
			if err := c.compileExprTo(cls.SendValue, valReg); err != nil {
				return err
			}
		}
	}

	c.emitABC(OP_SELECT, selectedReg, caseBase, len(s.Cases), line)
	var endJumps []int
	for i, cls := range s.Cases {
		idxReg := c.allocReg()
		c.emitAsBx(OP_LOADINT, idxReg, i+1, cls.P.Line)
		c.emitABC(OP_EQ, 0, selectedReg, idxReg, cls.P.Line)
		nextJump := c.emitJump(cls.P.Line)
		c.freeReg()

		c.enterScope()
		if cls.SendValue == nil && cls.RecvName != "" {
			localReg := c.addLocal(cls.RecvName)
			c.emitABC(OP_MOVE, localReg, recvReg, 0, cls.P.Line)
		}
		if cls.SendValue == nil && cls.RecvOkName != "" {
			localReg := c.addLocal(cls.RecvOkName)
			c.emitABC(OP_MOVE, localReg, recvOKReg, 0, cls.P.Line)
		}
		for _, st := range cls.Body.Stmts {
			if err := c.compileStmt(st); err != nil {
				return err
			}
		}
		c.leaveScope()
		endJumps = append(endJumps, c.emitJump(line))
		c.patchJump(nextJump)
	}
	for _, j := range endJumps {
		c.patchJump(j)
	}
	c.nextReg = base
	return nil
}

// ---- IfStmt ----
