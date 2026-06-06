package vm

// compiler_func.go — function-literal and function/closure compilation for
// the AST→bytecode compiler (params, upvalues, varargs, body lowering).
// Pure code movement from compiler.go; declarations are verbatim.

import (
	"github.com/never-labs/leia/internal/ast"
)

// ---- Function literal ----

func (c *compiler) compileFuncLitExpr(e *ast.FuncLitExpr, dest int) error {
	line := e.P.Line
	protoIdx, err := c.compileFunction("<anonymous>", e.Params, e.Body, line)
	if err != nil {
		return err
	}
	c.emitABx(OP_CLOSURE, dest, protoIdx, line)
	return nil
}

// --------------------------------------------------------------------
// Function compilation
// --------------------------------------------------------------------

func (c *compiler) compileFunction(name string, params []ast.FuncParam, body *ast.BlockStmt, line int) (int, error) {
	numFixedParams, isVarArg := countFixedFunctionParams(params)
	if c != nil {
		c.setFunctionArity(name, functionArity{numParams: numFixedParams, vararg: isVarArg})
	}

	child := newCompiler(c, name, line, isVarArg)
	child.enterScope()

	for _, p := range params {
		if p.IsVarArg {
			if p.Name != "" && p.Name != "..." {
				reg := child.addLocal(p.Name)
				child.emitABC(OP_NEWTABLE, reg, 0, 0, line)
				child.emitABC(OP_VARARG, reg+1, 0, 0, line)
				child.emitABC(OP_SETLIST, reg, 0, 1, line)
			}
			break
		}
		child.addLocal(p.Name)
	}
	child.proto.NumParams = numFixedParams
	child.collectFunctionArities(body.Stmts)
	child.collectFunctionResults(body.Stmts)
	child.collectLabelDepths(body.Stmts, child.depth)

	for _, stmt := range body.Stmts {
		if err := child.compileStmt(stmt); err != nil {
			return 0, err
		}
	}
	if err := child.patchGotos(); err != nil {
		return 0, err
	}

	child.leaveScope()

	code := child.proto.Code
	needReturn := true
	if len(code) > 0 {
		if DecodeOp(code[len(code)-1]) == OP_RETURN {
			needReturn = false
		}
	}
	if needReturn {
		child.emitReturn(0, 1)
	}

	proto := child.finish()
	proto.NumParams = numFixedParams
	proto.IsVarArg = isVarArg
	proto.Name = name

	idx := len(c.proto.Protos)
	c.proto.Protos = append(c.proto.Protos, proto)
	return idx, nil
}
