package runtime

// Top-level execution and statement evaluation for the tree-walking
// interpreter: Exec/ExecString, the statement dispatch (execStmt family),
// defer/go/send handling, and the per-statement-kind executors.
// Moved verbatim from interpreter.go (pure code movement).

import (
	"errors"
	"fmt"

	"github.com/gscript/gscript/internal/ast"
	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
)

// ====================================================================
// Exec -- top-level entry
// ====================================================================

// Exec executes a program (top-level statement list).
func (interp *Interpreter) Exec(prog *ast.Program) error {
	if err := ast.ValidateLabelControl(prog); err != nil {
		return err
	}
	interp.resetExecutionBudgets()
	interp.pushDeferFrame()
	_, _, _, _, err := interp.execBlockInEnv(&ast.BlockStmt{P: prog.GetPos(), Stmts: prog.Stmts}, interp.globals)
	if err != nil {
		_ = interp.runAndPopDeferFrame()
		var jump *gotoSignal
		if errors.As(err, &jump) {
			return fmt.Errorf("goto %q target not found", jump.name)
		}
		return err
	}
	return interp.runAndPopDeferFrame()
}

// ExecString parses and executes a source string, returning any top-level return values.
func (interp *Interpreter) ExecString(src string) ([]Value, error) {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil, err
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, err
	}
	if err := ast.ValidateLabelControl(prog); err != nil {
		return nil, err
	}
	interp.resetExecutionBudgets()
	// Execute and collect return values from the last return statement.
	var lastRet []Value
	interp.pushDeferFrame()
	retVals, isRet, _, _, err := interp.execBlockInEnv(&ast.BlockStmt{P: prog.GetPos(), Stmts: prog.Stmts}, interp.globals)
	if err != nil {
		_ = interp.runAndPopDeferFrame()
		var jump *gotoSignal
		if errors.As(err, &jump) {
			return nil, fmt.Errorf("goto %q target not found", jump.name)
		}
		return nil, err
	}
	if isRet {
		lastRet = retVals
	}
	if err := interp.runAndPopDeferFrame(); err != nil {
		return nil, err
	}
	return lastRet, nil
}

// ====================================================================
// Statement execution
// ====================================================================

type gotoSignal struct {
	name string
}

func (g *gotoSignal) Error() string {
	return "goto " + g.name
}

// execBlock executes a block of statements in a new child scope.
// Returns (returnValues, isReturn, isBreak, isContinue, error).
func (interp *Interpreter) execBlock(block *ast.BlockStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	child := NewEnvironment(env)
	return interp.execBlockInEnv(block, child)
}

// execBlockInEnv executes a block in the given environment (without creating a new scope).
func (interp *Interpreter) execBlockInEnv(block *ast.BlockStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	labels := make(map[string]int)
	for i, stmt := range block.Stmts {
		if label, ok := stmt.(*ast.LabelStmt); ok {
			labels[label.Name] = i
		}
	}
	for pc := 0; pc < len(block.Stmts); pc++ {
		stmt := block.Stmts[pc]
		retVals, isRet, isBrk, isCont, err := interp.execStmt(stmt, env)
		if err != nil {
			var jump *gotoSignal
			if errors.As(err, &jump) {
				if target, ok := labels[jump.name]; ok {
					pc = target
					continue
				}
			}
			return nil, false, false, false, err
		}
		if isRet || isBrk || isCont {
			return retVals, isRet, isBrk, isCont, nil
		}
	}
	return nil, false, false, false, nil
}

// execStmt dispatches a single statement.
func (interp *Interpreter) execStmt(stmt ast.Stmt, env *Environment) ([]Value, bool, bool, bool, error) {
	if err := interp.checkStepBudget(); err != nil {
		return nil, false, false, false, interp.wrapRuntimeError(err, stmt.GetPos())
	}
	retVals, isRet, isBrk, isCont, err := interp.execStmtRaw(stmt, env)
	if err != nil {
		var jump *gotoSignal
		if errors.As(err, &jump) {
			return nil, false, false, false, err
		}
		return nil, false, false, false, interp.wrapRuntimeError(err, stmt.GetPos())
	}
	return retVals, isRet, isBrk, isCont, nil
}

func (interp *Interpreter) execStmtRaw(stmt ast.Stmt, env *Environment) ([]Value, bool, bool, bool, error) {
	switch s := stmt.(type) {
	case *ast.DeclareStmt:
		return interp.execDeclare(s, env)
	case *ast.AssignStmt:
		return interp.execAssign(s, env)
	case *ast.CompoundAssignStmt:
		return interp.execCompoundAssign(s, env)
	case *ast.IncDecStmt:
		return interp.execIncDec(s, env)
	case *ast.CallStmt:
		_, err := interp.evalExpr(s.Call, env)
		return nil, false, false, false, err
	case *ast.IfStmt:
		return interp.execIf(s, env)
	case *ast.ForStmt:
		return interp.execFor(s, env)
	case *ast.ForNumStmt:
		return interp.execForNum(s, env)
	case *ast.ForRangeStmt:
		return interp.execForRange(s, env)
	case *ast.ReturnStmt:
		return interp.execReturn(s, env)
	case *ast.BreakStmt:
		return nil, false, true, false, nil
	case *ast.ContinueStmt:
		return nil, false, false, true, nil
	case *ast.LabelStmt:
		return nil, false, false, false, nil
	case *ast.GotoStmt:
		return nil, false, false, false, &gotoSignal{name: s.Name}
	case *ast.FuncDeclStmt:
		return interp.execFuncDecl(s, env)
	case *ast.BlockStmt:
		return interp.execBlock(s, env)
	case *ast.GoStmt:
		return interp.execGo(s, env)
	case *ast.DeferStmt:
		return interp.execDefer(s, env)
	case *ast.SendStmt:
		return interp.execSend(s, env)
	case *ast.SelectStmt:
		return interp.execSelect(s, env)
	default:
		return nil, false, false, false, fmt.Errorf("unknown statement type: %T", stmt)
	}
}

type deferredCall struct {
	fn   Value
	args []Value
}

func (interp *Interpreter) pushDeferFrame() {
	interp.deferStack = append(interp.deferStack, nil)
}

func (interp *Interpreter) runAndPopDeferFrame() error {
	if len(interp.deferStack) == 0 {
		return nil
	}
	idx := len(interp.deferStack) - 1
	calls := interp.deferStack[idx]
	interp.deferStack = interp.deferStack[:idx]

	var firstErr error
	for i := len(calls) - 1; i >= 0; i-- {
		if _, err := interp.callFunction(calls[i].fn, calls[i].args); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (interp *Interpreter) execDefer(s *ast.DeferStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	call, err := interp.prepareDeferredCall(s.Call, env)
	if err != nil {
		return nil, false, false, false, err
	}
	if len(interp.deferStack) == 0 {
		if _, err := interp.callFunction(call.fn, call.args); err != nil {
			return nil, false, false, false, err
		}
		return nil, false, false, false, nil
	}
	idx := len(interp.deferStack) - 1
	interp.deferStack[idx] = append(interp.deferStack[idx], call)
	return nil, false, false, false, nil
}

func (interp *Interpreter) prepareDeferredCall(expr ast.Expr, env *Environment) (deferredCall, error) {
	switch call := expr.(type) {
	case *ast.CallExpr:
		fn, err := interp.evalExprSingle(call.Func, env)
		if err != nil {
			return deferredCall{}, err
		}
		args, err := interp.evalExprList(call.Args, env)
		if err != nil {
			return deferredCall{}, err
		}
		return deferredCall{fn: fn, args: args}, nil
	case *ast.MethodCallExpr:
		obj, err := interp.evalExprSingle(call.Object, env)
		if err != nil {
			return deferredCall{}, err
		}
		method, err := interp.methodValue(obj, call.Method)
		if err != nil {
			return deferredCall{}, err
		}
		args, err := interp.evalExprList(call.Args, env)
		if err != nil {
			return deferredCall{}, err
		}
		args = append([]Value{obj}, args...)
		return deferredCall{fn: method, args: args}, nil
	default:
		return deferredCall{}, fmt.Errorf("defer statement requires a function call")
	}
}

func (interp *Interpreter) execGo(s *ast.GoStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	// Evaluate the function and arguments before launching the goroutine.
	switch call := s.Call.(type) {
	case *ast.CallExpr:
		fn, err := interp.evalExprSingle(call.Func, env)
		if err != nil {
			return nil, false, false, false, err
		}
		args, err := interp.evalExprList(call.Args, env)
		if err != nil {
			return nil, false, false, false, err
		}
		if err := interp.reserveGoroutineBudget(); err != nil {
			return nil, false, false, false, err
		}
		go func() {
			defer interp.releaseGoroutineBudget()
			childInterp := &Interpreter{
				globals:          interp.globals,
				stringMeta:       interp.stringMeta,
				maxSteps:         interp.maxSteps,
				maxNativeCalls:   interp.maxNativeCalls,
				maxCallDepth:     interp.maxCallDepth,
				maxGoroutines:    interp.maxGoroutines,
				activeGoroutines: interp.activeGoroutines,
				maxChannelCap:    interp.maxChannelCap,
				maxHostResult:    interp.maxHostResult,
				maxModuleBytes:   interp.maxModuleBytes,
				maxModuleDepth:   interp.maxModuleDepth,
			}
			childInterp.callFunction(fn, args)
		}()
	case *ast.MethodCallExpr:
		obj, err := interp.evalExprSingle(call.Object, env)
		if err != nil {
			return nil, false, false, false, err
		}
		method, err := interp.tableGet(obj, StringValue(call.Method))
		if err != nil {
			return nil, false, false, false, err
		}
		args, err := interp.evalExprList(call.Args, env)
		if err != nil {
			return nil, false, false, false, err
		}
		if err := interp.reserveGoroutineBudget(); err != nil {
			return nil, false, false, false, err
		}
		go func() {
			defer interp.releaseGoroutineBudget()
			childInterp := &Interpreter{
				globals:          interp.globals,
				stringMeta:       interp.stringMeta,
				maxSteps:         interp.maxSteps,
				maxNativeCalls:   interp.maxNativeCalls,
				maxCallDepth:     interp.maxCallDepth,
				maxGoroutines:    interp.maxGoroutines,
				activeGoroutines: interp.activeGoroutines,
				maxChannelCap:    interp.maxChannelCap,
				maxHostResult:    interp.maxHostResult,
				maxModuleBytes:   interp.maxModuleBytes,
				maxModuleDepth:   interp.maxModuleDepth,
			}
			childInterp.callFunction(method, args)
		}()
	default:
		return nil, false, false, false, fmt.Errorf("go statement requires a function call")
	}
	return nil, false, false, false, nil
}

func (interp *Interpreter) execSend(s *ast.SendStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	chVal, err := interp.evalExprSingle(s.Channel, env)
	if err != nil {
		return nil, false, false, false, err
	}
	if !chVal.IsChannel() {
		return nil, false, false, false, fmt.Errorf("send on non-channel value")
	}
	val, err := interp.evalExprSingle(s.Value, env)
	if err != nil {
		return nil, false, false, false, err
	}
	if err := chVal.Channel().Send(val); err != nil {
		return nil, false, false, false, err
	}
	return nil, false, false, false, nil
}

func (interp *Interpreter) execSelect(s *ast.SelectStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	if s.Default == nil {
		return interp.execBlockingSelect(s, env)
	}
	for _, cls := range s.Cases {
		chVal, err := interp.evalExprSingle(cls.Channel, env)
		if err != nil {
			return nil, false, false, false, err
		}
		if !chVal.IsChannel() {
			if cls.SendValue != nil {
				return nil, false, false, false, fmt.Errorf("send on non-channel value")
			}
			return nil, false, false, false, fmt.Errorf("receive from non-channel value")
		}

		if cls.SendValue != nil {
			val, err := interp.evalExprSingle(cls.SendValue, env)
			if err != nil {
				return nil, false, false, false, err
			}
			ok, err := chVal.Channel().TrySend(val)
			if err != nil {
				return nil, false, false, false, err
			}
			if !ok {
				continue
			}
			return interp.execSelectBody(cls.Body, "", "", NilValue(), false, env)
		}

		val, ready, recvOK := chVal.Channel().TryRecvOK()
		if !ready {
			continue
		}
		return interp.execSelectBody(cls.Body, cls.RecvName, cls.RecvOkName, val, recvOK, env)
	}
	if s.Default != nil {
		return interp.execSelectBody(s.Default, "", "", NilValue(), false, env)
	}
	return nil, false, false, false, nil
}

func (interp *Interpreter) execBlockingSelect(s *ast.SelectStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	if len(s.Cases) == 0 {
		return nil, false, false, false, fmt.Errorf("select requires at least one case")
	}
	cases := make([]ChannelSelectCase, len(s.Cases))
	for i, cls := range s.Cases {
		chVal, err := interp.evalExprSingle(cls.Channel, env)
		if err != nil {
			return nil, false, false, false, err
		}
		if !chVal.IsChannel() {
			return nil, false, false, false, fmt.Errorf("select case uses non-channel value")
		}
		cases[i].Channel = chVal.Channel()
		if cls.SendValue == nil {
			cases[i].Kind = ChannelSelectRecv
			continue
		}
		val, err := interp.evalExprSingle(cls.SendValue, env)
		if err != nil {
			return nil, false, false, false, err
		}
		cases[i].Kind = ChannelSelectSend
		cases[i].Value = val
	}
	chosen, val, recvOK, err := ChannelSelect(cases)
	if err != nil {
		return nil, false, false, false, err
	}
	cls := s.Cases[chosen]
	if cls.SendValue != nil {
		return interp.execSelectBody(cls.Body, "", "", NilValue(), false, env)
	}
	return interp.execSelectBody(cls.Body, cls.RecvName, cls.RecvOkName, val, recvOK, env)
}

func (interp *Interpreter) execSelectBody(body *ast.BlockStmt, recvName, recvOkName string, recvVal Value, recvOK bool, env *Environment) ([]Value, bool, bool, bool, error) {
	child := NewEnvironment(env)
	if recvName != "" {
		child.Define(recvName, recvVal)
	}
	if recvOkName != "" {
		child.Define(recvOkName, BoolValue(recvOK))
	}
	return interp.execBlock(body, child)
}

// ------------------------------------------------------------------
// DeclareStmt: a, b := expr1, expr2
// ------------------------------------------------------------------
func (interp *Interpreter) execDeclare(s *ast.DeclareStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	vals, err := interp.evalExprList(s.Values, env)
	if err != nil {
		return nil, false, false, false, err
	}
	for i, name := range s.Names {
		v := NilValue()
		if i < len(vals) {
			v = vals[i]
		}
		if env.IsLocalReadOnly(name) {
			return nil, false, false, false, fmt.Errorf("cannot redeclare readonly variable %q", name)
		}
		if s.ReadOnly {
			env.DefineReadOnly(name, v)
		} else {
			env.Define(name, v)
		}
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// AssignStmt: a, b = expr1, expr2
// ------------------------------------------------------------------
func (interp *Interpreter) execAssign(s *ast.AssignStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	vals, err := interp.evalExprList(s.Values, env)
	if err != nil {
		return nil, false, false, false, err
	}
	for i, target := range s.Targets {
		v := NilValue()
		if i < len(vals) {
			v = vals[i]
		}
		if err := interp.assignTarget(target, v, env); err != nil {
			return nil, false, false, false, err
		}
	}
	return nil, false, false, false, nil
}

// assignTarget assigns a value to an lvalue expression.
func (interp *Interpreter) assignTarget(target ast.Expr, val Value, env *Environment) error {
	switch t := target.(type) {
	case *ast.IdentExpr:
		if env.IsReadOnly(t.Name) {
			return fmt.Errorf("cannot assign to readonly variable %q", t.Name)
		}
		if !env.Set(t.Name, val) {
			// If variable doesn't exist anywhere, create it in the current env
			// (like a global implicit declaration)
			env.Define(t.Name, val)
		}
		return nil
	case *ast.IndexExpr:
		tbl, err := interp.evalExprSingle(t.Table, env)
		if err != nil {
			return err
		}
		key, err := interp.evalExprSingle(t.Index, env)
		if err != nil {
			return err
		}
		return interp.tableSet(tbl, key, val)
	case *ast.FieldExpr:
		tbl, err := interp.evalExprSingle(t.Table, env)
		if err != nil {
			return err
		}
		return interp.tableSet(tbl, StringValue(t.Field), val)
	default:
		return fmt.Errorf("invalid assignment target: %T", target)
	}
}

// ------------------------------------------------------------------
// CompoundAssignStmt: a += b
// ------------------------------------------------------------------
func (interp *Interpreter) execCompoundAssign(s *ast.CompoundAssignStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	lhs, err := interp.evalExprSingle(s.Target, env)
	if err != nil {
		return nil, false, false, false, err
	}
	rhs, err := interp.evalExprSingle(s.Value, env)
	if err != nil {
		return nil, false, false, false, err
	}

	var op string
	switch s.Op {
	case "+=":
		op = "+"
	case "-=":
		op = "-"
	case "*=":
		op = "*"
	case "/=":
		op = "/"
	default:
		return nil, false, false, false, fmt.Errorf("unknown compound operator: %s", s.Op)
	}

	result, err := interp.arith(op, lhs, rhs)
	if err != nil {
		return nil, false, false, false, err
	}

	if err := interp.assignTarget(s.Target, result, env); err != nil {
		return nil, false, false, false, err
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// IncDecStmt: a++ / a--
// ------------------------------------------------------------------
func (interp *Interpreter) execIncDec(s *ast.IncDecStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	lhs, err := interp.evalExprSingle(s.Target, env)
	if err != nil {
		return nil, false, false, false, err
	}

	var result Value
	one := IntValue(1)
	if s.Op == "++" {
		result, err = interp.arith("+", lhs, one)
	} else {
		result, err = interp.arith("-", lhs, one)
	}
	if err != nil {
		return nil, false, false, false, err
	}

	if err := interp.assignTarget(s.Target, result, env); err != nil {
		return nil, false, false, false, err
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// IfStmt
// ------------------------------------------------------------------
func (interp *Interpreter) execIf(s *ast.IfStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	cond, err := interp.evalExprSingle(s.Cond, env)
	if err != nil {
		return nil, false, false, false, err
	}
	if cond.Truthy() {
		return interp.execBlock(s.Body, env)
	}
	for _, ei := range s.ElseIfs {
		cond, err = interp.evalExprSingle(ei.Cond, env)
		if err != nil {
			return nil, false, false, false, err
		}
		if cond.Truthy() {
			return interp.execBlock(ei.Body, env)
		}
	}
	if s.ElseBody != nil {
		return interp.execBlock(s.ElseBody, env)
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// ForStmt (while-style): for cond { }
// ------------------------------------------------------------------
func (interp *Interpreter) execFor(s *ast.ForStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	for {
		if err := interp.checkStepBudget(); err != nil {
			return nil, false, false, false, err
		}
		if s.Cond != nil {
			cond, err := interp.evalExprSingle(s.Cond, env)
			if err != nil {
				return nil, false, false, false, err
			}
			if !cond.Truthy() {
				break
			}
		}
		retVals, isRet, isBrk, _, err := interp.execBlock(s.Body, env)
		if err != nil {
			return nil, false, false, false, err
		}
		if isRet {
			return retVals, true, false, false, nil
		}
		if isBrk {
			break
		}
		// isContinue just goes to next iteration
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// ForNumStmt (C-style): for init; cond; post { }
// ------------------------------------------------------------------
func (interp *Interpreter) execForNum(s *ast.ForNumStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	// The init, cond, and post all share a new scope
	loopEnv := NewEnvironment(env)
	// Execute init
	_, _, _, _, err := interp.execStmt(s.Init, loopEnv)
	if err != nil {
		return nil, false, false, false, err
	}
	for {
		if err := interp.checkStepBudget(); err != nil {
			return nil, false, false, false, err
		}
		// Evaluate condition
		cond, err := interp.evalExprSingle(s.Cond, loopEnv)
		if err != nil {
			return nil, false, false, false, err
		}
		if !cond.Truthy() {
			break
		}
		// Execute body in a child of loopEnv
		retVals, isRet, isBrk, _, err := interp.execBlock(s.Body, loopEnv)
		if err != nil {
			return nil, false, false, false, err
		}
		if isRet {
			return retVals, true, false, false, nil
		}
		if isBrk {
			break
		}
		// Execute post
		_, _, _, _, err = interp.execStmt(s.Post, loopEnv)
		if err != nil {
			return nil, false, false, false, err
		}
	}
	return nil, false, false, false, nil
}

// ------------------------------------------------------------------
// ForRangeStmt: for k, v := range expr { }
// ------------------------------------------------------------------
func (interp *Interpreter) execForRange(s *ast.ForRangeStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	iterVal, err := interp.evalExprSingle(s.Iter, env)
	if err != nil {
		return nil, false, false, false, err
	}

	if iterVal.IsTable() {
		tbl := iterVal.Table()
		var key Value = NilValue()
		for {
			if err := interp.checkStepBudget(); err != nil {
				return nil, false, false, false, err
			}
			nextKey, nextVal, ok := tbl.Next(key)
			if !ok {
				break
			}
			key = nextKey

			// Create new scope for each iteration
			iterEnv := NewEnvironment(env)
			iterEnv.Define(s.Key, nextKey)
			if s.Value != "" {
				iterEnv.Define(s.Value, nextVal)
			}

			retVals, isRet, isBrk, _, err := interp.execBlockInEnv(s.Body, iterEnv)
			if err != nil {
				return nil, false, false, false, err
			}
			if isRet {
				return retVals, true, false, false, nil
			}
			if isBrk {
				break
			}
		}
		return nil, false, false, false, nil
	}

	if iterVal.IsFunction() {
		// Iterator function: call repeatedly until nil
		for {
			if err := interp.checkStepBudget(); err != nil {
				return nil, false, false, false, err
			}
			results, err := interp.callFunction(iterVal, nil)
			if err != nil {
				return nil, false, false, false, err
			}
			if len(results) == 0 || results[0].IsNil() {
				break
			}
			iterEnv := NewEnvironment(env)
			iterEnv.Define(s.Key, results[0])
			if s.Value != "" {
				v := NilValue()
				if len(results) > 1 {
					v = results[1]
				}
				iterEnv.Define(s.Value, v)
			}
			retVals, isRet, isBrk, _, err := interp.execBlockInEnv(s.Body, iterEnv)
			if err != nil {
				return nil, false, false, false, err
			}
			if isRet {
				return retVals, true, false, false, nil
			}
			if isBrk {
				break
			}
		}
		return nil, false, false, false, nil
	}

	if iterVal.IsChannel() {
		ch := iterVal.Channel()
		for {
			if err := interp.checkStepBudget(); err != nil {
				return nil, false, false, false, err
			}
			val, ok := ch.Recv()
			if !ok {
				break
			}
			iterEnv := NewEnvironment(env)
			iterEnv.Define(s.Key, val)
			retVals, isRet, isBrk, _, err := interp.execBlockInEnv(s.Body, iterEnv)
			if err != nil {
				return nil, false, false, false, err
			}
			if isRet {
				return retVals, true, false, false, nil
			}
			if isBrk {
				break
			}
		}
		return nil, false, false, false, nil
	}

	return nil, false, false, false, fmt.Errorf("cannot range over %s", iterVal.TypeName())
}

// ------------------------------------------------------------------
// ReturnStmt
// ------------------------------------------------------------------
func (interp *Interpreter) execReturn(s *ast.ReturnStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	vals, err := interp.evalExprList(s.Values, env)
	if err != nil {
		return nil, false, false, false, err
	}
	return vals, true, false, false, nil
}

// ------------------------------------------------------------------
// FuncDeclStmt
// ------------------------------------------------------------------
func (interp *Interpreter) execFuncDecl(s *ast.FuncDeclStmt, env *Environment) ([]Value, bool, bool, bool, error) {
	proto := &FuncProto{
		Name:       s.Name,
		Body:       s.Body,
		SourceName: interp.currentSourceName,
		Line:       s.P.Line,
		Column:     s.P.Column,
	}
	paramNames := make([]string, 0, len(s.Params))
	for _, p := range s.Params {
		paramNames = append(paramNames, p.Name)
		proto.Params = append(proto.Params, p.Name)
		if p.IsVarArg {
			proto.HasVarArg = true
		}
	}

	// Define the function name first so it can self-reference (recursion)
	env.Define(s.Name, NilValue())

	// Capture free variables from the enclosing environment
	freeVarNames := FreeVars(s.Body, paramNames)
	upvalues := make(map[string]*Upvalue)
	for _, fv := range freeVarNames {
		if uv, ok := env.GetUpvalue(fv); ok {
			upvalues[fv] = uv
		}
	}

	closure := &Closure{
		Proto:    proto,
		Upvalues: upvalues,
		Env:      env,
	}
	env.Set(s.Name, FunctionValue(closure))
	return nil, false, false, false, nil
}
