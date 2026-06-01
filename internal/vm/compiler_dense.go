package vm

import (
	"fmt"

	"github.com/never-labs/leia/internal/ast"
)

func (c *compiler) compileDenseLitExpr(e *ast.DenseLitExpr, dest int) error {
	if e.Len > 0 && len(e.Values) != e.Len {
		return fmt.Errorf("line %d: dense literal length mismatch: declared %d, got %d", e.P.Line, e.Len, len(e.Values))
	}
	method, ok := denseLiteralArrayConstructor(e.DType)
	if !ok {
		return fmt.Errorf("line %d: unsupported dense literal dtype %q", e.P.Line, e.DType)
	}

	savedReg := c.nextReg
	c.nextReg = dest
	funcReg := c.allocReg()
	c.emitABx(OP_GETGLOBAL, funcReg, c.stringConst("array"), e.P.Line)
	c.emitABC(OP_GETFIELD, funcReg, funcReg, c.stringConst(method), e.P.Line)
	for _, value := range e.Values {
		argReg := c.allocReg()
		savedArgTop := c.nextReg
		if err := c.compileExprTo(value, argReg); err != nil {
			return err
		}
		c.nextReg = savedArgTop
	}
	c.emitABC(OP_CALL, funcReg, len(e.Values)+1, 2, e.P.Line)
	c.nextReg = savedReg
	if dest+1 > c.nextReg {
		c.nextReg = dest + 1
		if c.nextReg > c.maxReg {
			c.maxReg = c.nextReg
		}
	}
	return nil
}

func denseLiteralArrayConstructor(dtype string) (string, bool) {
	switch dtype {
	case "f64", "f32":
		return "f64", true
	case "i64", "i32":
		return "i64", true
	case "bool":
		return "bool", true
	default:
		return "", false
	}
}
