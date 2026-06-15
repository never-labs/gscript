package vm

// compiler_expr.go — expression compilation for the AST→bytecode compiler:
// literals, identifiers, binary/unary operators, comparisons, logical
// and/or, concat, conditional jumps, and call / method-call lowering
// (including spread argument handling). Pure code movement from compiler.go;
// declarations are verbatim.

import (
	"fmt"
	"strconv"

	"github.com/never-labs/leia/internal/ast"
)

func (c *compiler) compileExprTo(expr ast.Expr, dest int) error {
	switch e := expr.(type) {
	case *ast.NumberLit:
		return c.compileNumberLit(e, dest)
	case *ast.StringLit:
		k := c.stringConst(e.Value)
		c.emitABx(OP_LOADK, dest, k, e.P.Line)
		return nil
	case *ast.BoolLit:
		b := 0
		if e.Value {
			b = 1
		}
		c.emitABC(OP_LOADBOOL, dest, b, 0, e.P.Line)
		return nil
	case *ast.NilLit:
		c.emitABC(OP_LOADNIL, dest, 0, 0, e.P.Line)
		return nil
	case *ast.IdentExpr:
		return c.compileIdentExpr(e, dest)
	case *ast.BinaryExpr:
		return c.compileBinaryExpr(e, dest)
	case *ast.UnaryExpr:
		return c.compileUnaryExpr(e, dest)
	case *ast.ParenExpr:
		return c.compileExprTo(e.Inner, dest)
	case *ast.CallExpr:
		return c.compileCallExprMulti(e, dest, 1)
	case *ast.MethodCallExpr:
		return c.compileMethodCallExprMulti(e, dest, 1)
	case *ast.IndexExpr:
		return c.compileIndexExpr(e, dest)
	case *ast.FieldExpr:
		return c.compileFieldExpr(e, dest)
	case *ast.TableLitExpr:
		return c.compileTableLitExpr(e, dest)
	case *ast.ListLitExpr:
		return c.compileListLitExpr(e, dest)
	case *ast.DenseLitExpr:
		return c.compileDenseLitExpr(e, dest)
	case *ast.FuncLitExpr:
		return c.compileFuncLitExpr(e, dest)
	case *ast.VarArgExpr:
		c.emitABC(OP_VARARG, dest, 2, 0, e.P.Line)
		return nil
	case *ast.RecvExpr:
		return c.compileRecvExpr(e, dest)
	case *ast.MakeChanExpr:
		return c.compileMakeChanExpr(e, dest)
	default:
		return fmt.Errorf("line %d: unsupported expression type %T", expr.GetPos().Line, expr)
	}
}

func (c *compiler) compileNumberLit(e *ast.NumberLit, dest int) error {
	line := e.P.Line
	if i, err := strconv.ParseInt(e.Value, 0, 64); err == nil {
		if i >= -32767 && i <= 32767 {
			c.emitAsBx(OP_LOADINT, dest, int(i), line)
		} else {
			k := c.intConst(i)
			c.emitABx(OP_LOADK, dest, k, line)
		}
		return nil
	}
	if f, err := strconv.ParseFloat(e.Value, 64); err == nil {
		k := c.floatConst(f)
		c.emitABx(OP_LOADK, dest, k, line)
		return nil
	}
	return fmt.Errorf("line %d: invalid number literal %q", line, e.Value)
}

func (c *compiler) compileIdentExpr(e *ast.IdentExpr, dest int) error {
	line := e.P.Line
	reg := c.resolveLocal(e.Name)
	if reg >= 0 {
		if reg != dest {
			c.emitABC(OP_MOVE, dest, reg, 0, line)
		}
		return nil
	}
	upIdx := c.resolveUpvalue(e.Name)
	if upIdx >= 0 {
		c.emitABC(OP_GETUPVAL, dest, upIdx, 0, line)
		return nil
	}
	nameK := c.stringConst(e.Name)
	c.emitABx(OP_GETGLOBAL, dest, nameK, line)
	return nil
}

func (c *compiler) compileBinaryExpr(e *ast.BinaryExpr, dest int) error {
	line := e.P.Line

	if e.Op == "&&" {
		return c.compileAnd(e, dest)
	}
	if e.Op == "||" {
		return c.compileOr(e, dest)
	}

	switch e.Op {
	case "==":
		if arg, ok := typeNumberComparisonArg(e); ok {
			return c.compileIsNumberComparison(arg, dest, false, line)
		}
		return c.compileComparison(e, dest, OP_EQ, 0, false)
	case "!=":
		if arg, ok := typeNumberComparisonArg(e); ok {
			return c.compileIsNumberComparison(arg, dest, true, line)
		}
		return c.compileComparison(e, dest, OP_EQ, 1, false)
	case "<":
		return c.compileComparison(e, dest, OP_LT, 0, false)
	case "<=":
		return c.compileComparison(e, dest, OP_LE, 0, false)
	case ">":
		return c.compileComparison(e, dest, OP_LT, 0, true)
	case ">=":
		return c.compileComparison(e, dest, OP_LE, 0, true)
	}

	if e.Op == ".." {
		return c.compileConcat(e, dest)
	}

	var opcode Opcode
	switch e.Op {
	case "+":
		opcode = OP_ADD
	case "-":
		opcode = OP_SUB
	case "*":
		opcode = OP_MUL
	case "/":
		opcode = OP_DIV
	case "%":
		opcode = OP_MOD
	case "**":
		opcode = OP_POW
	case "&":
		opcode = OP_BAND
	case "|":
		opcode = OP_BOR
	case "^":
		opcode = OP_BXOR
	case "&^":
		opcode = OP_BANDN
	case "<<":
		opcode = OP_SHL
	case ">>":
		opcode = OP_SHR
	default:
		return fmt.Errorf("line %d: unsupported binary operator %q", line, e.Op)
	}

	leftReg, leftIsTemp, err := c.compileExprReg(e.Left)
	if err != nil {
		return err
	}
	rightReg, rightIsTemp, err := c.compileExprReg(e.Right)
	if err != nil {
		return err
	}

	c.emitABC(opcode, dest, leftReg, rightReg, line)

	if rightIsTemp {
		c.freeReg()
	}
	if leftIsTemp {
		c.freeReg()
	}
	return nil
}

func (c *compiler) compileAnd(e *ast.BinaryExpr, dest int) error {
	line := e.P.Line
	if err := c.compileExprTo(e.Left, dest); err != nil {
		return err
	}
	c.emitABC(OP_TESTSET, dest, dest, 0, line)
	skipJump := c.emitJump(line)
	if err := c.compileExprTo(e.Right, dest); err != nil {
		return err
	}
	c.patchJump(skipJump)
	return nil
}

func (c *compiler) compileOr(e *ast.BinaryExpr, dest int) error {
	line := e.P.Line
	if err := c.compileExprTo(e.Left, dest); err != nil {
		return err
	}
	c.emitABC(OP_TESTSET, dest, dest, 1, line)
	skipJump := c.emitJump(line)
	if err := c.compileExprTo(e.Right, dest); err != nil {
		return err
	}
	c.patchJump(skipJump)
	return nil
}

// compileCondJump compiles an expression as a branch condition.
// It emits instructions such that execution falls through when the condition is truthy,
// and takes the returned jump when the condition is falsy.
// For comparison expressions, this avoids materializing a boolean value.
func (c *compiler) compileCondJump(expr ast.Expr, line int) (int, error) {
	if binExpr, ok := expr.(*ast.BinaryExpr); ok {
		switch binExpr.Op {
		case "<":
			return c.compileCondCmp(binExpr, OP_LT, 0, false)
		case "<=":
			return c.compileCondCmp(binExpr, OP_LE, 0, false)
		case ">":
			return c.compileCondCmp(binExpr, OP_LT, 0, true)
		case ">=":
			return c.compileCondCmp(binExpr, OP_LE, 0, true)
		case "==":
			if arg, ok := typeNumberComparisonArg(binExpr); ok {
				return c.compileIsNumberCondJump(arg, false, false, line)
			}
			return c.compileCondCmp(binExpr, OP_EQ, 0, false)
		case "!=":
			if arg, ok := typeNumberComparisonArg(binExpr); ok {
				return c.compileIsNumberCondJump(arg, true, false, line)
			}
			return c.compileCondCmp(binExpr, OP_EQ, 1, false)
		}
	}
	if unExpr, ok := expr.(*ast.UnaryExpr); ok && unExpr.Op == "!" {
		return c.compileCondJumpInv(unExpr.Operand, line)
	}
	// Fallback: compile to register, then TEST
	condReg := c.allocReg()
	if err := c.compileExprTo(expr, condReg); err != nil {
		return 0, err
	}
	c.freeReg()
	c.emitABC(OP_TEST, condReg, 0, 0, line)
	return c.emitJump(line), nil
}

// compileCondJumpInv is like compileCondJump but with inverted sense:
// falls through when the condition is falsy, jumps when truthy.
func (c *compiler) compileCondJumpInv(expr ast.Expr, line int) (int, error) {
	if binExpr, ok := expr.(*ast.BinaryExpr); ok {
		switch binExpr.Op {
		case "<":
			return c.compileCondCmp(binExpr, OP_LT, 1, false)
		case "<=":
			return c.compileCondCmp(binExpr, OP_LE, 1, false)
		case ">":
			return c.compileCondCmp(binExpr, OP_LT, 1, true)
		case ">=":
			return c.compileCondCmp(binExpr, OP_LE, 1, true)
		case "==":
			if arg, ok := typeNumberComparisonArg(binExpr); ok {
				return c.compileIsNumberCondJump(arg, false, true, line)
			}
			return c.compileCondCmp(binExpr, OP_EQ, 1, false)
		case "!=":
			if arg, ok := typeNumberComparisonArg(binExpr); ok {
				return c.compileIsNumberCondJump(arg, true, true, line)
			}
			return c.compileCondCmp(binExpr, OP_EQ, 0, false)
		}
	}
	if unExpr, ok := expr.(*ast.UnaryExpr); ok && unExpr.Op == "!" {
		return c.compileCondJump(unExpr.Operand, line)
	}
	condReg := c.allocReg()
	if err := c.compileExprTo(expr, condReg); err != nil {
		return 0, err
	}
	c.freeReg()
	c.emitABC(OP_TEST, condReg, 0, 1, line) // C=1: skip if NOT truthy
	return c.emitJump(line), nil
}

// compileCondCmp emits a comparison opcode + JMP for use as a branch condition.
// Falls through when the comparison matches, jumps when it doesn't.
func (c *compiler) compileCondCmp(e *ast.BinaryExpr, op Opcode, a int, swap bool) (int, error) {
	line := e.P.Line
	var leftExpr, rightExpr ast.Expr
	if swap {
		leftExpr = e.Right
		rightExpr = e.Left
	} else {
		leftExpr = e.Left
		rightExpr = e.Right
	}
	leftReg, leftIsTemp, err := c.compileExprReg(leftExpr)
	if err != nil {
		return 0, err
	}
	rightReg, rightIsTemp, err := c.compileExprReg(rightExpr)
	if err != nil {
		return 0, err
	}
	c.emitABC(op, a, leftReg, rightReg, line)
	jmp := c.emitJump(line)
	if rightIsTemp {
		c.freeReg()
	}
	if leftIsTemp {
		c.freeReg()
	}
	return jmp, nil
}

// smallIntLit returns (value, true) if expr is an integer literal in [0, 255].
// Currently unused but retained for future immediate-operand opcodes.
func smallIntLit(expr ast.Expr) (int, bool) {
	num, ok := expr.(*ast.NumberLit)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(num.Value, 0, 64)
	if err != nil || i < 0 || i > 255 {
		return 0, false
	}
	return int(i), true
}

func (c *compiler) compileIsNumberComparison(arg ast.Expr, dest int, invert bool, line int) error {
	argReg, argIsTemp, err := c.compileExprReg(arg)
	if err != nil {
		return err
	}
	c.emitABC(OP_ISNUMBER, dest, argReg, 0, line)
	if invert {
		c.emitABC(OP_NOT, dest, dest, 0, line)
	}
	if argIsTemp {
		c.freeReg()
	}
	return nil
}

func (c *compiler) compileIsNumberCondJump(arg ast.Expr, invert bool, jumpWhenTrue bool, line int) (int, error) {
	condReg := c.allocReg()
	if err := c.compileExprTo(arg, condReg); err != nil {
		return 0, err
	}
	c.emitABC(OP_ISNUMBER, condReg, condReg, 0, line)
	c.freeReg()

	testC := 0
	if jumpWhenTrue {
		testC = 1
	}
	if invert {
		testC ^= 1
	}
	c.emitABC(OP_TEST, condReg, 0, testC, line)
	return c.emitJump(line), nil
}

func typeNumberComparisonArg(e *ast.BinaryExpr) (ast.Expr, bool) {
	if arg, ok := typeNumberCallArg(e.Left); ok && isStringLiteral(e.Right, "number") {
		return arg, true
	}
	if arg, ok := typeNumberCallArg(e.Right); ok && isStringLiteral(e.Left, "number") {
		return arg, true
	}
	return nil, false
}

func typeNumberCallArg(expr ast.Expr) (ast.Expr, bool) {
	expr = unwrapParenExpr(expr)
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	fn, ok := unwrapParenExpr(call.Func).(*ast.IdentExpr)
	if !ok || fn.Name != "type" {
		return nil, false
	}
	return call.Args[0], true
}

func isStringLiteral(expr ast.Expr, value string) bool {
	lit, ok := unwrapParenExpr(expr).(*ast.StringLit)
	return ok && lit.Value == value
}

func unwrapParenExpr(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.Inner
	}
}

func (c *compiler) compileComparison(e *ast.BinaryExpr, dest int, op Opcode, a int, swap bool) error {
	line := e.P.Line
	var leftExpr, rightExpr ast.Expr
	if swap {
		leftExpr = e.Right
		rightExpr = e.Left
	} else {
		leftExpr = e.Left
		rightExpr = e.Right
	}

	leftReg, leftIsTemp, err := c.compileExprReg(leftExpr)
	if err != nil {
		return err
	}
	rightReg, rightIsTemp, err := c.compileExprReg(rightExpr)
	if err != nil {
		return err
	}

	// OP_CMP A B C: if (R(B) op R(C)) != bool(A) then PC++
	// Pattern:
	//   [0] CMP A B C          ; if condition matches A, skip next
	//   [1] JMP to [3]         ; jump to false LOADBOOL
	//   [2] LOADBOOL dest 1 1  ; true, skip next
	//   [3] LOADBOOL dest 0 0  ; false
	c.emitABC(op, a, leftReg, rightReg, line)
	jmpToFalse := c.emitJump(line)
	c.emitABC(OP_LOADBOOL, dest, 1, 1, line) // true, skip next
	falsePos := c.currentPC()
	c.emitABC(OP_LOADBOOL, dest, 0, 0, line) // false
	c.patchJumpTo(jmpToFalse, falsePos)

	if rightIsTemp {
		c.freeReg()
	}
	if leftIsTemp {
		c.freeReg()
	}
	return nil
}

func (c *compiler) compileConcat(e *ast.BinaryExpr, dest int) error {
	line := e.P.Line
	parts := c.flattenConcat(e)
	base := c.nextReg
	for _, part := range parts {
		reg := c.allocReg()
		if err := c.compileExprTo(part, reg); err != nil {
			return err
		}
	}
	c.emitABC(OP_CONCAT, dest, base, base+len(parts)-1, line)
	c.freeRegs(len(parts))
	return nil
}

func (c *compiler) flattenConcat(expr ast.Expr) []ast.Expr {
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok || bin.Op != ".." {
		return []ast.Expr{expr}
	}
	return append(c.flattenConcat(bin.Left), c.flattenConcat(bin.Right)...)
}

func (c *compiler) compileUnaryExpr(e *ast.UnaryExpr, dest int) error {
	line := e.P.Line
	var op Opcode
	switch e.Op {
	case "-":
		op = OP_UNM
	case "^":
		op = OP_BNOT
	case "!":
		op = OP_NOT
	case "#":
		op = OP_LEN
	default:
		return fmt.Errorf("line %d: unsupported unary operator %q", line, e.Op)
	}
	operandReg := c.allocReg()
	if err := c.compileExprTo(e.Operand, operandReg); err != nil {
		return err
	}
	c.emitABC(op, dest, operandReg, 0, line)
	c.freeReg()
	return nil
}

// ---- Call expressions ----
