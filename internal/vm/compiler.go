package vm

import (
	"fmt"
	"strings"

	"github.com/gscript/gscript/internal/ast"
	"github.com/gscript/gscript/internal/runtime"
)

// --------------------------------------------------------------------
// Compiler: AST -> bytecode FuncProto
// --------------------------------------------------------------------

// Compile compiles a top-level program into a FuncProto.
func Compile(prog *ast.Program) (*FuncProto, error) {
	prog = ast.DesugarAINative(prog)
	if err := ast.ValidateLabelControl(prog); err != nil {
		return nil, err
	}
	c := newCompiler(nil, "<main>", 0, false)
	c.collectFunctionArities(prog.Stmts)
	c.collectLabelDepths(prog.Stmts, c.depth)
	for _, stmt := range prog.Stmts {
		if err := c.compileStmt(stmt); err != nil {
			return nil, err
		}
	}
	if err := c.patchGotos(); err != nil {
		return nil, err
	}
	c.emitReturn(0, 1)
	return c.finish(), nil
}

// --------------------------------------------------------------------
// Internal types
// --------------------------------------------------------------------

type localVar struct {
	name     string
	reg      int
	depth    int
	captured bool
	readOnly bool
}

type loopInfo struct {
	breakJumps    []int
	continueJumps []int
	scopeDepth    int
}

type upvalInfo struct {
	name     string
	inStack  bool
	index    int
	readOnly bool
}

type compiler struct {
	parent         *compiler
	proto          *FuncProto
	locals         []localVar
	upvals         []upvalInfo
	readOnlyLocals map[int]string
	nextReg        int
	maxReg         int
	depth          int
	loops          []loopInfo
	isVarArg       bool
	labels         map[string]compiledLabel
	labelDepths    map[string]int
	gotos          []compiledGoto
	funcArities    map[string]functionArity
	arityScopes    []map[string]*functionArity
}

type compiledLabel struct {
	pc    int
	depth int
	line  int
}

type compiledGoto struct {
	name  string
	pc    int
	depth int
	line  int
}

type functionArity struct {
	numParams int
	vararg    bool
}

func newCompiler(parent *compiler, name string, line int, isVarArg bool) *compiler {
	var arities map[string]functionArity
	if parent != nil {
		arities = parent.funcArities
	}
	if arities == nil {
		arities = make(map[string]functionArity)
	}
	return &compiler{
		parent: parent,
		proto: &FuncProto{
			Name:        name,
			LineDefined: line,
		},
		isVarArg:    isVarArg,
		labels:      make(map[string]compiledLabel),
		labelDepths: make(map[string]int),
		funcArities: arities,
	}
}

// isMainTopLevel returns true when we are at the top scope of the main chunk.
// Declarations at this level create globals rather than locals.
func (c *compiler) isMainTopLevel() bool {
	return c.parent == nil && c.depth == 0
}

func (c *compiler) collectFunctionArities(stmts []ast.Stmt) {
	if c == nil || c.funcArities == nil {
		return
	}
	for _, stmt := range stmts {
		fn, ok := stmt.(*ast.FuncDeclStmt)
		if !ok {
			continue
		}
		numParams, vararg := countFixedFunctionParams(fn.Params)
		c.setFunctionArity(fn.Name, functionArity{numParams: numParams, vararg: vararg})
	}
}

func (c *compiler) setFunctionArity(name string, arity functionArity) {
	if c == nil || c.funcArities == nil || name == "" {
		return
	}
	if len(c.arityScopes) > 0 {
		scope := c.arityScopes[len(c.arityScopes)-1]
		if _, recorded := scope[name]; !recorded {
			if prev, ok := c.funcArities[name]; ok {
				prevCopy := prev
				scope[name] = &prevCopy
			} else {
				scope[name] = nil
			}
		}
	}
	c.funcArities[name] = arity
}

func countFixedFunctionParams(params []ast.FuncParam) (int, bool) {
	numFixedParams := 0
	for _, p := range params {
		if p.IsVarArg {
			return numFixedParams, true
		}
		numFixedParams++
	}
	return numFixedParams, false
}

// --------------------------------------------------------------------
// Register allocator
// --------------------------------------------------------------------

func (c *compiler) allocReg() int {
	r := c.nextReg
	c.nextReg++
	if c.nextReg > c.maxReg {
		c.maxReg = c.nextReg
	}
	return r
}

func (c *compiler) allocRegs(n int) int {
	base := c.nextReg
	c.nextReg += n
	if c.nextReg > c.maxReg {
		c.maxReg = c.nextReg
	}
	return base
}

func (c *compiler) freeReg()       { c.nextReg-- }
func (c *compiler) freeRegs(n int) { c.nextReg -= n }

// --------------------------------------------------------------------
// Scoping
// --------------------------------------------------------------------

func (c *compiler) enterScope() {
	c.depth++
	c.arityScopes = append(c.arityScopes, make(map[string]*functionArity))
}

func (c *compiler) leaveScope() {
	hasCaptured := false
	firstCapturedReg := c.nextReg
	count := 0
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].depth < c.depth {
			break
		}
		if c.locals[i].captured {
			hasCaptured = true
			if c.locals[i].reg < firstCapturedReg {
				firstCapturedReg = c.locals[i].reg
			}
		}
		count++
	}
	if hasCaptured {
		c.emit(EncodeABC(OP_CLOSE, firstCapturedReg, 0, 0), 0)
	}
	c.locals = c.locals[:len(c.locals)-count]
	if count > 0 {
		c.freeRegs(count)
	}
	if len(c.arityScopes) > 0 {
		scope := c.arityScopes[len(c.arityScopes)-1]
		for name, prev := range scope {
			if prev == nil {
				delete(c.funcArities, name)
			} else {
				c.funcArities[name] = *prev
			}
		}
		c.arityScopes = c.arityScopes[:len(c.arityScopes)-1]
	}
	c.depth--
}

// --------------------------------------------------------------------
// Local variable management
// --------------------------------------------------------------------

func (c *compiler) addLocal(name string) int {
	return c.addLocalWithReadOnly(name, false)
}

func (c *compiler) addLocalWithReadOnly(name string, readOnly bool) int {
	reg := c.allocReg()
	c.locals = append(c.locals, localVar{name: name, reg: reg, depth: c.depth, readOnly: readOnly})
	if readOnly {
		if c.readOnlyLocals == nil {
			c.readOnlyLocals = make(map[int]string)
		}
		c.readOnlyLocals[reg] = name
	}
	return reg
}

func (c *compiler) resolveLocalInfo(name string) (int, bool) {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].name == name {
			return c.locals[i].reg, c.locals[i].readOnly
		}
	}
	return -1, false
}

func (c *compiler) resolveLocal(name string) int {
	reg, _ := c.resolveLocalInfo(name)
	return reg
}

func (c *compiler) resolveUpvalueInfo(name string) (int, bool) {
	if c.parent == nil {
		return -1, false
	}
	for i := len(c.parent.locals) - 1; i >= 0; i-- {
		if c.parent.locals[i].name == name {
			c.parent.locals[i].captured = true
			readOnly := c.parent.locals[i].readOnly
			return c.addUpvalue(name, true, c.parent.locals[i].reg, readOnly), readOnly
		}
	}
	parentUpIdx, readOnly := c.parent.resolveUpvalueInfo(name)
	if parentUpIdx >= 0 {
		return c.addUpvalue(name, false, parentUpIdx, readOnly), readOnly
	}
	return -1, false
}

func (c *compiler) resolveUpvalue(name string) int {
	idx, _ := c.resolveUpvalueInfo(name)
	return idx
}

func (c *compiler) addUpvalue(name string, inStack bool, index int, readOnly bool) int {
	for i, uv := range c.upvals {
		if uv.inStack == inStack && uv.index == index {
			return i
		}
	}
	idx := len(c.upvals)
	c.upvals = append(c.upvals, upvalInfo{name: name, inStack: inStack, index: index, readOnly: readOnly})
	return idx
}

// --------------------------------------------------------------------
// Constant pool
// --------------------------------------------------------------------

func (c *compiler) addConst(v runtime.Value) int {
	for i, k := range c.proto.Constants {
		if k.Equal(v) && k.Type() == v.Type() {
			return i
		}
	}
	idx := len(c.proto.Constants)
	c.proto.Constants = append(c.proto.Constants, v)
	return idx
}

func (c *compiler) stringConst(s string) int { return c.addConst(runtime.StringValue(s)) }
func (c *compiler) intConst(i int64) int     { return c.addConst(runtime.IntValue(i)) }
func (c *compiler) floatConst(f float64) int { return c.addConst(runtime.FloatValue(f)) }

// --------------------------------------------------------------------
// Code emission
// --------------------------------------------------------------------

func (c *compiler) emit(inst uint32, line int) int {
	pos := len(c.proto.Code)
	c.proto.Code = append(c.proto.Code, inst)
	c.proto.LineInfo = append(c.proto.LineInfo, line)
	if DecodeOp(inst) == OP_VARARG {
		c.proto.UsesVarargBytecode = true
	}
	return pos
}

func (c *compiler) emitABC(op Opcode, a, b, cc int, line int) int {
	return c.emit(EncodeABC(op, a, b, cc), line)
}
func (c *compiler) emitABx(op Opcode, a, bx int, line int) int {
	return c.emit(EncodeABx(op, a, bx), line)
}
func (c *compiler) emitAsBx(op Opcode, a, sbx int, line int) int {
	return c.emit(EncodeAsBx(op, a, sbx), line)
}
func (c *compiler) emitJump(line int) int {
	return c.emit(EncodesBx(OP_JMP, 0), line)
}
func (c *compiler) emitReturn(a, b int) int {
	return c.emitABC(OP_RETURN, a, b, 0, 0)
}

func (c *compiler) emitSpreadOp(op Opcode, a, b, cc int, line int) int {
	c.proto.JITDisabled = true
	return c.emitABC(op, a, b, cc, line)
}

func (c *compiler) patchJump(jmpPos int) {
	target := len(c.proto.Code)
	offset := target - jmpPos - 1
	c.proto.Code[jmpPos] = EncodesBx(OP_JMP, offset)
}

func (c *compiler) patchJumpTo(jmpPos int, target int) {
	offset := target - jmpPos - 1
	c.proto.Code[jmpPos] = EncodesBx(OP_JMP, offset)
}

func (c *compiler) currentPC() int { return len(c.proto.Code) }

func explicitSpreadExpr(expr ast.Expr) (ast.Expr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	if spreadExpr, ok := globalSpreadExpr(call); ok {
		return spreadExpr, true
	}
	if isTableSpreadCall(call) {
		return call, true
	}
	return nil, false
}

func globalSpreadExpr(call *ast.CallExpr) (ast.Expr, bool) {
	if ident, ok := call.Func.(*ast.IdentExpr); ok && ident.Name == "spread" && len(call.Args) == 1 {
		return call.Args[0], true
	}
	return nil, false
}

func isTableSpreadCall(call *ast.CallExpr) bool {
	if field, ok := call.Func.(*ast.FieldExpr); ok {
		if ident, ok := field.Table.(*ast.IdentExpr); ok && ident.Name == "table" &&
			field.Field == "spread" {
			return true
		}
	}
	return false
}

func hasExplicitSpread(exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if _, ok := explicitSpreadExpr(expr); ok {
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------
// Loop management
// --------------------------------------------------------------------

func (c *compiler) pushLoop() {
	c.loops = append(c.loops, loopInfo{scopeDepth: c.depth})
}

func (c *compiler) popLoop() loopInfo {
	info := c.loops[len(c.loops)-1]
	c.loops = c.loops[:len(c.loops)-1]
	return info
}

func (c *compiler) currentLoop() *loopInfo {
	if len(c.loops) == 0 {
		return nil
	}
	return &c.loops[len(c.loops)-1]
}

func (c *compiler) patchBreaks(info loopInfo) {
	for _, pos := range info.breakJumps {
		c.patchJump(pos)
	}
}

func (c *compiler) patchContinues(info loopInfo, target int) {
	for _, pos := range info.continueJumps {
		c.patchJumpTo(pos, target)
	}
}

func (c *compiler) emitCloseForJump() {
	loop := c.currentLoop()
	if loop == nil {
		return
	}
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].depth <= loop.scopeDepth {
			break
		}
		if c.locals[i].captured {
			c.emit(EncodeABC(OP_CLOSE, c.locals[i].reg, 0, 0), 0)
			return
		}
	}
}

func (c *compiler) emitCloseCapturedAboveDepth(depth int) {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].depth <= depth {
			break
		}
		if c.locals[i].captured {
			c.emit(EncodeABC(OP_CLOSE, c.locals[i].reg, 0, 0), 0)
			return
		}
	}
}

// --------------------------------------------------------------------
// Finish: build the final FuncProto
// --------------------------------------------------------------------

func (c *compiler) finish() *FuncProto {
	c.proto.MaxStack = c.maxReg + 2
	c.proto.NumParams = 0
	c.proto.IsVarArg = c.isVarArg
	c.proto.Upvalues = make([]UpvalDesc, len(c.upvals))
	for i, uv := range c.upvals {
		c.proto.Upvalues[i] = UpvalDesc{Name: uv.name, InStack: uv.inStack, Index: uv.index, ReadOnly: uv.readOnly}
	}
	c.proto.ReadOnlyLocals = c.readOnlyLocals
	c.proto.LeafNoCall = protoHasNoCalls(c.proto)
	c.proto.NoGlobalOps = protoHasNoGlobalOps(c.proto)
	return c.proto
}

func (c *compiler) patchGotos() error {
	for _, g := range c.gotos {
		label, ok := c.labels[g.name]
		if !ok {
			return fmt.Errorf("line %d: goto %q target not found", g.line, g.name)
		}
		c.patchJumpTo(g.pc, label.pc)
	}
	return nil
}

func (c *compiler) collectLabelDepths(stmts []ast.Stmt, depth int) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.LabelStmt:
			c.labelDepths[s.Name] = depth
		case *ast.BlockStmt:
			c.collectLabelDepths(s.Stmts, depth+1)
		case *ast.IfStmt:
			c.collectLabelDepths(s.Body.Stmts, depth+1)
			for _, ei := range s.ElseIfs {
				c.collectLabelDepths(ei.Body.Stmts, depth+1)
			}
			if s.ElseBody != nil {
				c.collectLabelDepths(s.ElseBody.Stmts, depth+1)
			}
		case *ast.ForStmt:
			c.collectLabelDepths(s.Body.Stmts, depth+1)
		case *ast.ForNumStmt:
			c.collectLabelDepths(s.Body.Stmts, depth+2)
		case *ast.ForRangeStmt:
			c.collectLabelDepths(s.Body.Stmts, depth+2)
		case *ast.FuncDeclStmt:
			// Labels are function-local; nested functions collect separately.
		}
	}
}

func protoHasNoGlobalOps(proto *FuncProto) bool {
	for _, inst := range proto.Code {
		switch DecodeOp(inst) {
		case OP_GETGLOBAL, OP_SETGLOBAL, OP_SETGLOBALRO:
			return false
		}
	}
	return true
}

// --------------------------------------------------------------------
// Helper: compile an expression into an allocated temp register
// If the expression is a local variable reference, returns its register
// without allocating (isTemp=false).
// --------------------------------------------------------------------

func (c *compiler) compileExprReg(expr ast.Expr) (reg int, isTemp bool, err error) {
	if ident, ok := expr.(*ast.IdentExpr); ok {
		r := c.resolveLocal(ident.Name)
		if r >= 0 {
			return r, false, nil
		}
	}
	r := c.allocReg()
	if e := c.compileExprTo(expr, r); e != nil {
		return 0, false, e
	}
	return r, true, nil
}

// --------------------------------------------------------------------
// Disassemble: human-readable bytecode dump
// --------------------------------------------------------------------

func Disassemble(proto *FuncProto) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("function %q (%d instructions, %d constants, %d upvalues, %d protos)\n",
		proto.Name, len(proto.Code), len(proto.Constants), len(proto.Upvalues), len(proto.Protos)))
	sb.WriteString(fmt.Sprintf("  params=%d, vararg=%v, maxstack=%d\n", proto.NumParams, proto.IsVarArg, proto.MaxStack))

	if len(proto.Constants) > 0 {
		sb.WriteString("  constants:\n")
		for i, k := range proto.Constants {
			sb.WriteString(fmt.Sprintf("    [%d] %s\n", i, k.String()))
		}
	}
	if len(proto.Upvalues) > 0 {
		sb.WriteString("  upvalues:\n")
		for i, uv := range proto.Upvalues {
			sb.WriteString(fmt.Sprintf("    [%d] %s instack=%v index=%d\n", i, uv.Name, uv.InStack, uv.Index))
		}
	}

	for i, inst := range proto.Code {
		op := DecodeOp(inst)
		a := DecodeA(inst)
		b := DecodeB(inst)
		cc := DecodeC(inst)
		bx := DecodeBx(inst)
		sbx := DecodesBx(inst)

		line := 0
		if i < len(proto.LineInfo) {
			line = proto.LineInfo[i]
		}

		var desc string
		switch op {
		case OP_LOADNIL:
			desc = fmt.Sprintf("LOADNIL    R%d..R%d", a, a+b)
		case OP_LOADBOOL:
			desc = fmt.Sprintf("LOADBOOL   R%d %v skip=%d", a, b != 0, cc)
		case OP_LOADINT:
			desc = fmt.Sprintf("LOADINT    R%d %d", a, sbx)
		case OP_LOADK:
			desc = fmt.Sprintf("LOADK      R%d K%d  ; %s", a, bx, proto.Constants[bx].String())
		case OP_MOVE:
			desc = fmt.Sprintf("MOVE       R%d R%d", a, b)
		case OP_GETGLOBAL:
			desc = fmt.Sprintf("GETGLOBAL  R%d K%d  ; %s", a, bx, proto.Constants[bx].String())
		case OP_SETGLOBAL:
			desc = fmt.Sprintf("SETGLOBAL  R%d K%d  ; %s", a, bx, proto.Constants[bx].String())
		case OP_SETGLOBALRO:
			desc = fmt.Sprintf("SETGLOBALRO R%d K%d  ; %s", a, bx, proto.Constants[bx].String())
		case OP_CHECKCONST:
			desc = fmt.Sprintf("CHECKCONST R%d K%d  ; %s", a, bx, proto.Constants[bx].String())
		case OP_GETUPVAL:
			desc = fmt.Sprintf("GETUPVAL   R%d U%d", a, b)
		case OP_SETUPVAL:
			desc = fmt.Sprintf("SETUPVAL   R%d U%d", a, b)
		case OP_NEWTABLE:
			desc = fmt.Sprintf("NEWTABLE   R%d array=%d hash=%d", a, b, cc)
		case OP_NEWOBJECT2:
			desc = fmt.Sprintf("NEWOBJECT2 R%d ctor=%d values=R%d,R%d", a, b, cc, cc+1)
		case OP_NEWOBJECTN:
			n := 0
			if b >= 0 && b < len(proto.TableCtorsN) {
				n = len(proto.TableCtorsN[b].KeyConsts)
			}
			desc = fmt.Sprintf("NEWOBJECTN R%d ctor=%d values=R%d..R%d", a, b, cc, cc+n-1)
		case OP_GETTABLE:
			desc = fmt.Sprintf("GETTABLE   R%d R%d R%d", a, b, cc)
		case OP_SETTABLE:
			desc = fmt.Sprintf("SETTABLE   R%d R%d R%d", a, b, cc)
		case OP_GETFIELD:
			desc = fmt.Sprintf("GETFIELD   R%d R%d K%d", a, b, cc)
		case OP_SETFIELD:
			desc = fmt.Sprintf("SETFIELD   R%d K%d R%d", a, b, cc)
		case OP_SETLIST:
			desc = fmt.Sprintf("SETLIST    R%d count=%d batch=%d", a, b, cc)
		case OP_SETLISTDYN:
			desc = fmt.Sprintf("SETLISTDYN R%d idx=R%d values=R%d..top", a, b, cc)
		case OP_APPEND:
			desc = fmt.Sprintf("APPEND     R%d R%d", a, b)
		case OP_ADD:
			desc = fmt.Sprintf("ADD        R%d R%d R%d", a, b, cc)
		case OP_SUB:
			desc = fmt.Sprintf("SUB        R%d R%d R%d", a, b, cc)
		case OP_MUL:
			desc = fmt.Sprintf("MUL        R%d R%d R%d", a, b, cc)
		case OP_DIV:
			desc = fmt.Sprintf("DIV        R%d R%d R%d", a, b, cc)
		case OP_MOD:
			desc = fmt.Sprintf("MOD        R%d R%d R%d", a, b, cc)
		case OP_POW:
			desc = fmt.Sprintf("POW        R%d R%d R%d", a, b, cc)
		case OP_BAND:
			desc = fmt.Sprintf("BAND       R%d R%d R%d", a, b, cc)
		case OP_BOR:
			desc = fmt.Sprintf("BOR        R%d R%d R%d", a, b, cc)
		case OP_BXOR:
			desc = fmt.Sprintf("BXOR       R%d R%d R%d", a, b, cc)
		case OP_BANDN:
			desc = fmt.Sprintf("BANDN      R%d R%d R%d", a, b, cc)
		case OP_SHL:
			desc = fmt.Sprintf("SHL        R%d R%d R%d", a, b, cc)
		case OP_SHR:
			desc = fmt.Sprintf("SHR        R%d R%d R%d", a, b, cc)
		case OP_UNM:
			desc = fmt.Sprintf("UNM        R%d R%d", a, b)
		case OP_BNOT:
			desc = fmt.Sprintf("BNOT       R%d R%d", a, b)
		case OP_NOT:
			desc = fmt.Sprintf("NOT        R%d R%d", a, b)
		case OP_ISNUMBER:
			desc = fmt.Sprintf("ISNUMBER   R%d R%d", a, b)
		case OP_LEN:
			desc = fmt.Sprintf("LEN        R%d R%d", a, b)
		case OP_CONCAT:
			desc = fmt.Sprintf("CONCAT     R%d R%d..R%d", a, b, cc)
		case OP_EQ:
			desc = fmt.Sprintf("EQ         %d R%d R%d", a, b, cc)
		case OP_LT:
			desc = fmt.Sprintf("LT         %d R%d R%d", a, b, cc)
		case OP_LE:
			desc = fmt.Sprintf("LE         %d R%d R%d", a, b, cc)
		case OP_TEST:
			desc = fmt.Sprintf("TEST       R%d %d", a, cc)
		case OP_TESTSET:
			desc = fmt.Sprintf("TESTSET    R%d R%d %d", a, b, cc)
		case OP_JMP:
			target := i + 1 + sbx
			desc = fmt.Sprintf("JMP        %d  ; to %d", sbx, target)
		case OP_CALL:
			desc = fmt.Sprintf("CALL       R%d B=%d C=%d", a, b, cc)
		case OP_CALLTABLE:
			desc = fmt.Sprintf("CALLTABLE  R%d args=R%d C=%d", a, b, cc)
		case OP_YIELD:
			desc = fmt.Sprintf("YIELD      R%d B=%d C=%d", a, b, cc)
		case OP_RESUME:
			desc = fmt.Sprintf("RESUME     R%d B=%d C=%d", a, b, cc)
		case OP_RETURN:
			desc = fmt.Sprintf("RETURN     R%d B=%d", a, b)
		case OP_CLOSURE:
			desc = fmt.Sprintf("CLOSURE    R%d Proto%d", a, bx)
		case OP_CLOSE:
			desc = fmt.Sprintf("CLOSE      R%d", a)
		case OP_FORPREP:
			desc = fmt.Sprintf("FORPREP    R%d %d  ; to %d", a, sbx, i+1+sbx)
		case OP_FORLOOP:
			desc = fmt.Sprintf("FORLOOP    R%d %d  ; to %d", a, sbx, i+1+sbx)
		case OP_TFORCALL:
			desc = fmt.Sprintf("TFORCALL   R%d C=%d", a, cc)
		case OP_TFORLOOP:
			desc = fmt.Sprintf("TFORLOOP   R%d %d  ; to %d", a, sbx, i+1+sbx)
		case OP_VARARG:
			desc = fmt.Sprintf("VARARG     R%d B=%d", a, b)
		case OP_SELF:
			desc = fmt.Sprintf("SELF       R%d R%d K%d", a, b, cc)
		case OP_GO:
			desc = fmt.Sprintf("GO         R%d B=%d", a, b)
		case OP_DEFER:
			desc = fmt.Sprintf("DEFER      R%d B=%d", a, b)
		case OP_MAKECHAN:
			desc = fmt.Sprintf("MAKECHAN   R%d B=%d C=%d", a, b, cc)
		case OP_SEND:
			desc = fmt.Sprintf("SEND       R%d <- R%d", a, b)
		case OP_RECV:
			desc = fmt.Sprintf("RECV       R%d = <-R%d", a, b)
		case OP_RECVOK:
			desc = fmt.Sprintf("RECVOK     R%d, R%d = <-R%d", a, cc, b)
		case OP_TRYSEND:
			desc = fmt.Sprintf("TRYSEND    R%d <- R%d => R%d", a, b, cc)
		case OP_TRYRECV:
			desc = fmt.Sprintf("TRYRECV    R%d, R%d = <-R%d", a, cc, b)
		case OP_TRYRECVOK:
			desc = fmt.Sprintf("TRYRECVOK  R%d, R%d, R%d = <-R%d", a, a+1, cc, b)
		case OP_SELECT:
			desc = fmt.Sprintf("SELECT     R%d,R%d,R%d cases=R%d count=%d", a, a+1, a+2, b, cc)
		case OP_SETTOP:
			desc = fmt.Sprintf("SETTOP     R%d", a)
		default:
			desc = fmt.Sprintf("%-10s %d %d %d", OpName(op), a, b, cc)
		}

		sb.WriteString(fmt.Sprintf("  [%03d] %-4d %s\n", i, line, desc))
	}

	for i, p := range proto.Protos {
		sb.WriteString(fmt.Sprintf("\n--- Proto %d ---\n", i))
		sb.WriteString(Disassemble(p))
	}

	return sb.String()
}
