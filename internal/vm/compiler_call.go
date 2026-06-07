package vm

// compiler_call.go — call and method-call expression lowering for the
// AST→bytecode compiler: multi-result calls, fixed-arity / single-result
// detection, explicit-spread argument lists, coroutine builtin lowering and
// method lookup. Pure code movement from compiler.go; declarations verbatim.

import (
	"github.com/never-labs/leia/internal/ast"
)

func (c *compiler) compileCallExprMulti(call *ast.CallExpr, dest int, nResults int) error {
	line := call.P.Line
	if spreadExpr, ok := globalSpreadExpr(call); ok {
		return c.compileExplicitSpreadExprMulti(spreadExpr, dest, nResults, line)
	}
	if hasExplicitSpread(call.Args) {
		return c.compileCallExprWithExplicitSpread(call, dest, nResults)
	}
	if op, ok := c.staticCoroutineBuiltinOp(call); ok {
		return c.compileCoroutineBuiltinCall(call, dest, nResults, line, op)
	}
	savedReg := c.nextReg

	c.nextReg = dest
	funcReg := c.allocReg()
	if err := c.compileExprTo(call.Func, funcReg); err != nil {
		return err
	}

	nArgs := len(call.Args)
	lastArgIsMulti := false
	for i, arg := range call.Args {
		argReg := c.allocReg()
		savedArgTop := c.nextReg
		expandFinalArg := i == nArgs-1 && !c.callTargetsFixedArityFunction(call)
		if expandFinalArg {
			switch a := arg.(type) {
			case *ast.CallExpr:
				if c.callExprKnownSingleResult(a) {
					if err := c.compileExprTo(a, argReg); err != nil {
						return err
					}
					c.nextReg = savedArgTop
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
		// Non-final nested calls compile to a fresh scratch reg then MOVE to
		// argReg, preserving list-adjustment's single-value collapse without
		// aliasing the inner call result slot with the outer argument slot.
		switch arg.(type) {
		case *ast.CallExpr, *ast.MethodCallExpr:
			saved := c.nextReg
			scratch := c.allocReg()
			if err := c.compileExprTo(arg, scratch); err != nil {
				return err
			}
			c.emitABC(OP_MOVE, argReg, scratch, 0, line)
			c.nextReg = saved
		default:
			if err := c.compileExprTo(arg, argReg); err != nil {
				return err
			}
			c.nextReg = savedArgTop
		}
	}

	b := nArgs + 1
	if lastArgIsMulti {
		b = 0
	}
	cc := nResults + 1
	if nResults == -1 {
		cc = 0
	}

	c.emitABC(OP_CALL, funcReg, b, cc, line)

	c.nextReg = savedReg
	if nResults > 0 {
		needed := dest + nResults
		if needed > c.nextReg {
			c.nextReg = needed
			if c.nextReg > c.maxReg {
				c.maxReg = c.nextReg
			}
		}
	}
	return nil
}

func (c *compiler) compileExplicitSpreadExprMulti(expr ast.Expr, dest int, nResults int, line int) error {
	c.proto.JITDisabled = true
	switch v := expr.(type) {
	case *ast.CallExpr:
		return c.compileCallExprMulti(v, dest, nResults)
	case *ast.MethodCallExpr:
		return c.compileMethodCallExprMulti(v, dest, nResults)
	case *ast.VarArgExpr:
		if nResults == -1 {
			c.emitABC(OP_VARARG, dest, 0, 0, line)
			return nil
		}
		c.emitABC(OP_VARARG, dest, nResults+1, 0, line)
		return nil
	default:
		if nResults == 0 {
			tmp := c.allocReg()
			if err := c.compileExprTo(expr, tmp); err != nil {
				return err
			}
			c.freeReg()
			return nil
		}
		if err := c.compileExprTo(expr, dest); err != nil {
			return err
		}
		if nResults == -1 {
			c.emitSpreadOp(OP_SETTOP, dest+1, 0, 0, line)
			return nil
		}
		for i := 1; i < nResults; i++ {
			c.emitABC(OP_LOADNIL, dest+i, 0, 0, line)
		}
		return nil
	}
}

func (c *compiler) callTargetsFixedArityFunction(call *ast.CallExpr) bool {
	if call == nil || c == nil {
		return false
	}
	if fn, ok := call.Func.(*ast.IdentExpr); ok {
		if c.proto != nil && c.proto.Name != "" && !c.proto.IsVarArg && fn.Name == c.proto.Name {
			return len(call.Args) == c.proto.NumParams
		}
		if arity, ok := c.funcArities[fn.Name]; ok {
			return !arity.vararg && len(call.Args) == arity.numParams
		}
		return builtinFixedArityCall(fn.Name, "", len(call.Args))
	}
	if field, ok := call.Func.(*ast.FieldExpr); ok {
		if recv, ok := field.Table.(*ast.IdentExpr); ok {
			return builtinFixedArityCall(recv.Name, field.Field, len(call.Args))
		}
	}
	return false
}

func (c *compiler) callExprKnownSingleResult(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	if c != nil && c.proto != nil && c.proto.Name == "<main>" {
		return false
	}
	if fn, ok := call.Func.(*ast.IdentExpr); ok {
		if c != nil {
			if c.proto != nil && c.proto.Name != "" && fn.Name == c.proto.Name {
				if res, ok := c.funcResults[fn.Name]; ok && res.single {
					return true
				}
			}
			if res, ok := c.funcResults[fn.Name]; ok && res.single {
				return true
			}
		}
		switch fn.Name {
		case "tonumber", "type", "len", "tostring", "error", "assert":
			return true
		}
		return false
	}
	if field, ok := call.Func.(*ast.FieldExpr); ok {
		recv, ok := field.Table.(*ast.IdentExpr)
		if !ok {
			return false
		}
		switch recv.Name {
		case "bit32":
			return true
		case "math":
			return true
		case "string":
			switch field.Field {
			case "format", "sub", "len", "upper", "lower", "reverse", "rep", "split", "trim", "trimLeft", "trimRight", "hasPrefix", "hasSuffix", "contains", "count", "replaceAll":
				return true
			}
		case "time":
			switch field.Field {
			case "now", "since":
				return true
			}
		case "utf8":
			switch field.Field {
			case "char", "codes", "offset", "valid", "validate", "sanitize", "reverse", "sub", "upper", "lower", "charclass":
				return true
			case "codepoint":
				return len(call.Args) <= 2
			}
		}
	}
	return false
}

func builtinFixedArityCall(name, field string, argc int) bool {
	switch name {
	case "tonumber":
		return field == "" && (argc == 1 || argc == 2)
	case "type", "len", "pairs", "ipairs":
		return field == "" && argc == 1
	case "math":
		switch field {
		case "abs", "ceil", "floor", "sqrt", "sin", "cos", "tan", "asin", "acos", "log", "exp", "deg", "rad", "tointeger":
			return argc == 1
		case "atan", "fmod":
			return argc == 1 || argc == 2
		case "floorDiv", "min", "max", "random":
			return argc == 2
		}
	case "string":
		switch field {
		case "len", "upper", "lower", "reverse":
			return argc == 1
		case "sub", "byte":
			return argc == 2 || argc == 3
		case "split", "find", "match", "trim", "trimLeft", "trimRight", "hasPrefix", "hasSuffix", "contains", "count":
			return argc == 2 || argc == 3
		case "replaceAll":
			return argc == 3
		}
	}
	return false
}

func (c *compiler) compileCallExprWithExplicitSpread(call *ast.CallExpr, dest int, nResults int) error {
	line := call.P.Line
	savedReg := c.nextReg
	c.nextReg = dest

	funcReg := c.allocReg()
	if err := c.compileExprTo(call.Func, funcReg); err != nil {
		return err
	}
	argsTableReg := c.allocReg()
	idxReg := c.allocReg()
	c.emitABC(OP_NEWTABLE, argsTableReg, len(call.Args), 1, line)
	c.emitAsBx(OP_LOADINT, idxReg, 1, line)
	if err := c.compileExplicitSpreadArgList(argsTableReg, idxReg, call.Args, line); err != nil {
		return err
	}
	c.emitArgCount(argsTableReg, idxReg, line)

	cc := nResults + 1
	if nResults == -1 {
		cc = 0
	}
	c.emitSpreadOp(OP_CALLTABLE, funcReg, argsTableReg, cc, line)

	c.nextReg = savedReg
	if nResults > 0 {
		needed := dest + nResults
		if needed > c.nextReg {
			c.nextReg = needed
			if c.nextReg > c.maxReg {
				c.maxReg = c.nextReg
			}
		}
	}
	return nil
}

func (c *compiler) staticCoroutineBuiltinOp(call *ast.CallExpr) (Opcode, bool) {
	field, ok := call.Func.(*ast.FieldExpr)
	if !ok {
		return 0, false
	}
	ident, ok := field.Table.(*ast.IdentExpr)
	if !ok || ident.Name != "coroutine" {
		return 0, false
	}
	switch field.Field {
	case "yield":
		return OP_YIELD, true
	case "resume":
		return OP_RESUME, true
	default:
		return 0, false
	}
}

func (c *compiler) compileCoroutineBuiltinCall(call *ast.CallExpr, dest int, nResults int, line int, op Opcode) error {
	savedReg := c.nextReg
	c.nextReg = dest
	funcReg := c.allocReg()

	nArgs := len(call.Args)
	lastArgIsMulti := false
	for i, arg := range call.Args {
		argReg := c.allocReg()
		savedArgTop := c.nextReg
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
		c.nextReg = savedArgTop
	}

	b := nArgs + 1
	if lastArgIsMulti {
		b = 0
	}
	cc := nResults + 1
	if nResults == -1 {
		cc = 0
	}
	c.emitABC(op, funcReg, b, cc, line)
	c.nextReg = savedReg
	return nil
}

func (c *compiler) compileMethodCallExprMulti(call *ast.MethodCallExpr, dest int, nResults int) error {
	line := call.P.Line
	if hasExplicitSpread(call.Args) {
		return c.compileMethodCallExprWithExplicitSpread(call, dest, nResults)
	}
	savedReg := c.nextReg

	c.nextReg = dest
	selfReg := c.allocReg() // dest = funcReg
	_ = c.allocReg()        // dest+1 = self (filled by OP_SELF)

	objReg := c.allocReg()
	if err := c.compileExprTo(call.Object, objReg); err != nil {
		return err
	}

	c.emitMethodLookup(selfReg, objReg, c.stringConst(call.Method), line)
	c.freeReg() // objReg

	nArgs := len(call.Args)
	lastArgIsMulti := false
	for i, arg := range call.Args {
		argReg := c.allocReg()
		savedArgTop := c.nextReg
		if i == nArgs-1 {
			switch a := arg.(type) {
			case *ast.CallExpr:
				lastArgIsMulti = true
				if err := c.compileCallExprMulti(a, argReg, -1); err != nil {
					return err
				}
				continue
			case *ast.MethodCallExpr:
				lastArgIsMulti = true
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
		c.nextReg = savedArgTop
	}

	b := nArgs + 2
	if lastArgIsMulti {
		b = 0
	}
	cc := nResults + 1
	if nResults == -1 {
		cc = 0
	}

	c.emitABC(OP_CALL, selfReg, b, cc, line)

	c.nextReg = savedReg
	if nResults > 0 {
		needed := dest + nResults
		if needed > c.nextReg {
			c.nextReg = needed
			if c.nextReg > c.maxReg {
				c.maxReg = c.nextReg
			}
		}
	}
	return nil
}

func (c *compiler) compileMethodCallExprWithExplicitSpread(call *ast.MethodCallExpr, dest int, nResults int) error {
	line := call.P.Line
	savedReg := c.nextReg
	c.nextReg = dest

	selfReg := c.allocReg()
	_ = c.allocReg()
	objReg := c.allocReg()
	if err := c.compileExprTo(call.Object, objReg); err != nil {
		return err
	}
	c.emitMethodLookup(selfReg, objReg, c.stringConst(call.Method), line)
	c.freeReg()

	argsTableReg := c.allocReg()
	idxReg := c.allocReg()
	c.emitABC(OP_NEWTABLE, argsTableReg, len(call.Args)+1, 1, line)
	c.emitAsBx(OP_LOADINT, idxReg, 1, line)
	c.compileAppendSingle(argsTableReg, idxReg, selfReg+1, line)
	if err := c.compileExplicitSpreadArgList(argsTableReg, idxReg, call.Args, line); err != nil {
		return err
	}
	c.emitArgCount(argsTableReg, idxReg, line)

	cc := nResults + 1
	if nResults == -1 {
		cc = 0
	}
	c.emitSpreadOp(OP_CALLTABLE, selfReg, argsTableReg, cc, line)

	c.nextReg = savedReg
	if nResults > 0 {
		needed := dest + nResults
		if needed > c.nextReg {
			c.nextReg = needed
			if c.nextReg > c.maxReg {
				c.maxReg = c.nextReg
			}
		}
	}
	return nil
}

func (c *compiler) compileExplicitSpreadArgList(tableReg, idxReg int, args []ast.Expr, line int) error {
	for _, arg := range args {
		if spreadExpr, ok := explicitSpreadExpr(arg); ok {
			if err := c.compileAppendSpread(tableReg, idxReg, spreadExpr, line); err != nil {
				return err
			}
			continue
		}
		valueReg := c.allocReg()
		if err := c.compileExprTo(arg, valueReg); err != nil {
			return err
		}
		c.compileAppendSingle(tableReg, idxReg, valueReg, line)
		c.freeReg()
	}
	return nil
}

func (c *compiler) compileAppendSingle(tableReg, idxReg, valueReg int, line int) {
	c.emitABC(OP_SETTABLE, tableReg, idxReg, valueReg, line)
	oneReg := c.allocReg()
	c.emitAsBx(OP_LOADINT, oneReg, 1, line)
	c.emitABC(OP_ADD, idxReg, idxReg, oneReg, line)
	c.freeReg()
}

func (c *compiler) compileAppendSpread(tableReg, idxReg int, expr ast.Expr, line int) error {
	switch v := expr.(type) {
	case *ast.CallExpr:
		valueReg := c.nextReg
		if err := c.compileCallExprMulti(v, valueReg, -1); err != nil {
			return err
		}
		c.emitSpreadOp(OP_SETLISTDYN, tableReg, idxReg, valueReg, line)
		c.nextReg = valueReg
	case *ast.MethodCallExpr:
		valueReg := c.nextReg
		if err := c.compileMethodCallExprMulti(v, valueReg, -1); err != nil {
			return err
		}
		c.emitSpreadOp(OP_SETLISTDYN, tableReg, idxReg, valueReg, line)
		c.nextReg = valueReg
	case *ast.VarArgExpr:
		valueReg := c.nextReg
		c.emitABC(OP_VARARG, valueReg, 0, 0, line)
		c.emitSpreadOp(OP_SETLISTDYN, tableReg, idxReg, valueReg, line)
	default:
		valueReg := c.allocReg()
		if err := c.compileExprTo(expr, valueReg); err != nil {
			return err
		}
		c.compileAppendSingle(tableReg, idxReg, valueReg, line)
		c.freeReg()
	}
	return nil
}

func (c *compiler) emitArgCount(tableReg, idxReg int, line int) {
	countReg := c.allocReg()
	oneReg := c.allocReg()
	c.emitAsBx(OP_LOADINT, oneReg, 1, line)
	c.emitABC(OP_SUB, countReg, idxReg, oneReg, line)
	c.emitSetField(tableReg, c.stringConst("n"), countReg, line)
	c.freeRegs(2)
}

func (c *compiler) emitMethodLookup(selfReg, objReg, methodK, line int) {
	// OP_SELF's C operand is only 8 bits in ABC encoding, so RK constants
	// cannot be represented there. For encodable method constants, use
	// MOVE+GETFIELD so method calls share the field inline-cache path.
	if methodK <= 0xFF {
		c.emitABC(OP_MOVE, selfReg+1, objReg, 0, line)
		c.emitABC(OP_GETFIELD, selfReg, objReg, methodK, line)
		return
	}

	methodReg := c.allocReg()
	c.emitABx(OP_LOADK, methodReg, methodK, line)
	c.emitABC(OP_SELF, selfReg, objReg, methodReg, line)
	c.freeReg() // methodReg
}

// ---- Index / Field expressions ----
