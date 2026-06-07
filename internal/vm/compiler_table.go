package vm

// compiler_table.go — index, field and table-constructor expression
// compilation for the AST→bytecode compiler, including small/fixed and
// spread table-literal fast paths and static table-ctor caching.
// Pure code movement from compiler.go; declarations are verbatim.

import (
	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/runtime"
)

func (c *compiler) compileIndexExpr(e *ast.IndexExpr, dest int) error {
	line := e.P.Line
	// Use compileExprReg to avoid allocating a temp for table if it's a local.
	// This prevents table-type temps from being reused by float/int computations,
	// which causes trace JIT guard failures from type-conflicting slot reuse.
	tableReg, tableIsTemp, err := c.compileExprReg(e.Table)
	if err != nil {
		return err
	}
	keyReg := c.allocReg()
	if err := c.compileExprTo(e.Index, keyReg); err != nil {
		return err
	}
	c.emitABC(OP_GETTABLE, dest, tableReg, keyReg, line)
	c.freeReg() // keyReg
	if tableIsTemp {
		c.freeReg() // tableReg (only if we allocated it)
	}
	return nil
}

func (c *compiler) compileFieldExpr(e *ast.FieldExpr, dest int) error {
	line := e.P.Line
	// Use compileExprReg: if table is a local variable, use its register directly
	// instead of allocating a temp. This avoids type-conflicting slot reuse in
	// the trace JIT (table temp later reused for float/int computation).
	tableReg, tableIsTemp, err := c.compileExprReg(e.Table)
	if err != nil {
		return err
	}
	// GETFIELD A B C: R(A) = R(B)[Constants[C]]
	fieldK := c.stringConst(e.Field)
	c.emitGetField(dest, tableReg, fieldK, line)
	if tableIsTemp {
		c.freeReg() // tableReg
	}
	return nil
}

func (c *compiler) emitGetField(dest, tableReg, fieldK, line int) {
	if fieldK <= 0xFF {
		c.emitABC(OP_GETFIELD, dest, tableReg, fieldK, line)
		return
	}
	keyReg := c.allocReg()
	c.emitABx(OP_LOADK, keyReg, fieldK, line)
	c.emitABC(OP_GETTABLE, dest, tableReg, keyReg, line)
	c.freeReg()
}

func (c *compiler) emitSetField(tableReg, fieldK, valueReg, line int) {
	if fieldK <= 0xFF {
		c.emitABC(OP_SETFIELD, tableReg, fieldK, valueReg, line)
		return
	}
	keyReg := c.allocReg()
	c.emitABx(OP_LOADK, keyReg, fieldK, line)
	c.emitABC(OP_SETTABLE, tableReg, keyReg, valueReg, line)
	c.freeReg()
}

// ---- Table construction ----

type staticStringField struct {
	key      string
	keyConst int
	value    ast.Expr
}

func (c *compiler) compileTableLitExpr(e *ast.TableLitExpr, dest int) error {
	line := e.P.Line
	if hasExplicitSpreadTableField(e) {
		return c.compileTableLitExprWithExplicitSpread(e, dest, line)
	}
	if ok, err := c.compileTwoFieldTableLitExpr(e, dest, line); ok || err != nil {
		return err
	}
	if ok, err := c.compileSmallFixedTableLitExpr(e, dest, line); ok || err != nil {
		return err
	}

	entryNextReg := c.nextReg
	arrayCount := 0
	hashCount := 0
	for _, f := range e.Fields {
		if f.Key == nil {
			arrayCount++
		} else if !isNilLiteral(f.Value) {
			hashCount++
		}
	}

	c.emitABC(OP_NEWTABLE, dest, arrayCount, hashCount, line)

	arrayIdx := 0
	pendingArrayBase := -1
	pendingArrayCount := 0

	flushArrayBatch := func() {
		if pendingArrayCount > 0 {
			batchNum := (arrayIdx-pendingArrayCount)/50 + 1
			if pendingArrayBase >= 0 && pendingArrayBase != dest+1 {
				for i := 0; i < pendingArrayCount; i++ {
					c.emitABC(OP_MOVE, dest+1+i, pendingArrayBase+i, 0, line)
				}
			}
			c.emitABC(OP_SETLIST, dest, pendingArrayCount, batchNum, line)
			pendingArrayCount = 0
			pendingArrayBase = -1
		}
	}

	for i, f := range e.Fields {
		if f.Key == nil {
			// Array-style field
			if pendingArrayBase == -1 {
				pendingArrayBase = c.nextReg
			}
			valueReg := c.allocReg()
			isLastField := (i == len(e.Fields)-1)

			if isLastField {
				switch v := f.Value.(type) {
				case *ast.CallExpr:
					if err := c.compileCallExprMulti(v, valueReg, -1); err != nil {
						return err
					}
					arrayIdx++
					pendingArrayCount++
					batchNum := (arrayIdx-pendingArrayCount)/50 + 1
					c.emitABC(OP_SETLIST, dest, 0, batchNum, line)
					c.nextReg = pendingArrayBase
					pendingArrayCount = 0
					pendingArrayBase = -1
					continue
				case *ast.MethodCallExpr:
					if err := c.compileMethodCallExprMulti(v, valueReg, -1); err != nil {
						return err
					}
					arrayIdx++
					pendingArrayCount++
					batchNum := (arrayIdx-pendingArrayCount)/50 + 1
					c.emitABC(OP_SETLIST, dest, 0, batchNum, line)
					c.nextReg = pendingArrayBase
					pendingArrayCount = 0
					pendingArrayBase = -1
					continue
				case *ast.VarArgExpr:
					c.emitABC(OP_VARARG, valueReg, 0, 0, line)
					arrayIdx++
					pendingArrayCount++
					batchNum := (arrayIdx-pendingArrayCount)/50 + 1
					c.emitABC(OP_SETLIST, dest, 0, batchNum, line)
					c.nextReg = pendingArrayBase
					pendingArrayCount = 0
					pendingArrayBase = -1
					continue
				}
			}

			if err := c.compileExprTo(f.Value, valueReg); err != nil {
				return err
			}
			c.nextReg = valueReg + 1
			arrayIdx++
			pendingArrayCount++

			if pendingArrayCount >= 50 {
				batchBase := pendingArrayBase
				flushArrayBatch()
				if batchBase >= 0 {
					c.nextReg = batchBase
				}
			}
		} else {
			// Flush pending array elements first
			if pendingArrayCount > 0 {
				flushArrayBatch()
				c.freeRegs(pendingArrayCount)
				pendingArrayBase = -1
			}

			// Key-value field
			if strKey, ok := f.Key.(*ast.StringLit); ok {
				if isNilLiteral(f.Value) {
					continue
				}
				fieldK := c.stringConst(strKey.Value)
				valReg := c.allocReg()
				if err := c.compileExprTo(f.Value, valReg); err != nil {
					return err
				}
				c.emitSetField(dest, fieldK, valReg, line)
				c.freeReg()
			} else if identKey, ok := f.Key.(*ast.IdentExpr); ok {
				if isNilLiteral(f.Value) {
					continue
				}
				fieldK := c.stringConst(identKey.Name)
				valReg := c.allocReg()
				if err := c.compileExprTo(f.Value, valReg); err != nil {
					return err
				}
				c.emitSetField(dest, fieldK, valReg, line)
				c.freeReg()
			} else {
				keyReg := c.allocReg()
				if err := c.compileExprTo(f.Key, keyReg); err != nil {
					return err
				}
				valReg := c.allocReg()
				if err := c.compileExprTo(f.Value, valReg); err != nil {
					return err
				}
				c.emitABC(OP_SETTABLE, dest, keyReg, valReg, line)
				c.freeRegs(2)
			}
		}
	}

	if pendingArrayCount > 0 {
		flushArrayBatch()
		if pendingArrayBase >= 0 {
			c.nextReg = pendingArrayBase
		}
	}

	c.nextReg = entryNextReg
	return nil
}

func hasExplicitSpreadTableField(e *ast.TableLitExpr) bool {
	for _, f := range e.Fields {
		if f.Key == nil {
			if _, ok := explicitSpreadExpr(f.Value); ok {
				return true
			}
		}
	}
	return false
}

func (c *compiler) compileTableLitExprWithExplicitSpread(e *ast.TableLitExpr, dest int, line int) error {
	arrayCount := 0
	hashCount := 0
	for _, f := range e.Fields {
		if f.Key == nil {
			arrayCount++
		} else if !isNilLiteral(f.Value) {
			hashCount++
		}
	}
	c.emitABC(OP_NEWTABLE, dest, arrayCount, hashCount, line)

	tempBase := c.nextReg
	idxReg := c.allocReg()
	c.emitAsBx(OP_LOADINT, idxReg, 1, line)

	for _, f := range e.Fields {
		if f.Key == nil {
			if spreadExpr, ok := explicitSpreadExpr(f.Value); ok {
				if err := c.compileAppendSpread(dest, idxReg, spreadExpr, line); err != nil {
					return err
				}
				continue
			}
			valueReg := c.allocReg()
			if err := c.compileExprTo(f.Value, valueReg); err != nil {
				return err
			}
			c.compileAppendSingle(dest, idxReg, valueReg, line)
			c.freeReg()
			continue
		}

		if strKey, ok := f.Key.(*ast.StringLit); ok {
			if isNilLiteral(f.Value) {
				continue
			}
			fieldK := c.stringConst(strKey.Value)
			valReg := c.allocReg()
			if err := c.compileExprTo(f.Value, valReg); err != nil {
				return err
			}
			c.emitSetField(dest, fieldK, valReg, line)
			c.freeReg()
		} else if identKey, ok := f.Key.(*ast.IdentExpr); ok {
			if isNilLiteral(f.Value) {
				continue
			}
			fieldK := c.stringConst(identKey.Name)
			valReg := c.allocReg()
			if err := c.compileExprTo(f.Value, valReg); err != nil {
				return err
			}
			c.emitSetField(dest, fieldK, valReg, line)
			c.freeReg()
		} else {
			keyReg := c.allocReg()
			if err := c.compileExprTo(f.Key, keyReg); err != nil {
				return err
			}
			valReg := c.allocReg()
			if err := c.compileExprTo(f.Value, valReg); err != nil {
				return err
			}
			c.emitABC(OP_SETTABLE, dest, keyReg, valReg, line)
			c.freeRegs(2)
		}
	}

	c.nextReg = tempBase
	return nil
}

func (c *compiler) compileSmallFixedTableLitExpr(e *ast.TableLitExpr, dest int, line int) (bool, error) {
	const maxSmallFixedFields = 8
	fields := make([]staticStringField, 0, len(e.Fields))
	seen := make(map[string]struct{}, len(e.Fields))
	for _, f := range e.Fields {
		if f.Key == nil {
			return false, nil
		}
		if isNilLiteral(f.Value) {
			continue
		}
		key, ok := staticStringFieldName(f.Key)
		if !ok {
			return false, nil
		}
		if _, dup := seen[key]; dup {
			return false, nil
		}
		if !c.smallFixedCtorValueSafe(f.Value) {
			return false, nil
		}
		seen[key] = struct{}{}
		fields = append(fields, staticStringField{
			key:      key,
			keyConst: c.stringConst(key),
			value:    f.Value,
		})
		if len(fields) > maxSmallFixedFields {
			return false, nil
		}
	}
	if len(fields) <= 2 {
		return false, nil
	}
	ctor := c.addTableCtorN(fields)
	if ctor < 0 || ctor > 255 {
		return false, nil
	}

	valueBase := c.nextReg
	for range fields {
		c.allocReg()
	}
	for i := range fields {
		if err := c.compileExprTo(fields[i].value, valueBase+i); err != nil {
			c.nextReg = valueBase
			return true, err
		}
	}
	c.emitABC(OP_NEWOBJECTN, dest, ctor, valueBase, line)
	c.nextReg = valueBase
	return true, nil
}

func (c *compiler) smallFixedCtorValueSafe(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.NumberLit, *ast.StringLit, *ast.BoolLit, *ast.NilLit:
		return true
	case *ast.IdentExpr:
		return c.resolveLocal(v.Name) >= 0
	case *ast.BinaryExpr:
		return c.smallFixedCtorValueSafe(v.Left) && c.smallFixedCtorValueSafe(v.Right)
	case *ast.UnaryExpr:
		return c.smallFixedCtorValueSafe(v.Operand)
	case *ast.ParenExpr:
		return c.smallFixedCtorValueSafe(v.Inner)
	default:
		return false
	}
}

func (c *compiler) compileTwoFieldTableLitExpr(e *ast.TableLitExpr, dest int, line int) (bool, error) {
	fields := make([]staticStringField, 0, 2)
	for _, f := range e.Fields {
		if f.Key == nil {
			return false, nil
		}
		if isNilLiteral(f.Value) {
			continue
		}
		key, ok := staticStringFieldName(f.Key)
		if !ok {
			return false, nil
		}
		fields = append(fields, staticStringField{
			key:      key,
			keyConst: c.stringConst(key),
			value:    f.Value,
		})
		if len(fields) > 2 {
			return false, nil
		}
	}
	if len(fields) != 2 || fields[0].key == fields[1].key {
		return false, nil
	}
	ctor := c.addTableCtor2(fields[0].keyConst, fields[1].keyConst, fields[0].key, fields[1].key)
	if ctor < 0 || ctor > 255 {
		return false, nil
	}

	valueBase := c.nextReg
	val1 := c.allocReg()
	val2 := c.allocReg()
	if err := c.compileExprTo(fields[0].value, val1); err != nil {
		return true, err
	}
	if err := c.compileExprTo(fields[1].value, val2); err != nil {
		return true, err
	}
	c.emitABC(OP_NEWOBJECT2, dest, ctor, valueBase, line)
	c.nextReg = valueBase
	return true, nil
}

func staticStringFieldName(key ast.Expr) (string, bool) {
	switch k := key.(type) {
	case *ast.StringLit:
		return k.Value, true
	case *ast.IdentExpr:
		return k.Name, true
	default:
		return "", false
	}
}

func (c *compiler) addTableCtor2(key1Const, key2Const int, key1, key2 string) int {
	for i := range c.proto.TableCtors2 {
		ctor := &c.proto.TableCtors2[i]
		if ctor.Key1Const == key1Const && ctor.Key2Const == key2Const {
			return i
		}
	}
	if len(c.proto.TableCtors2) >= 256 {
		return -1
	}
	c.proto.TableCtors2 = append(c.proto.TableCtors2, TableCtor2{
		Key1Const: key1Const,
		Key2Const: key2Const,
		Runtime:   runtime.NewSmallTableCtor2(key1, key2),
	})
	return len(c.proto.TableCtors2) - 1
}

func (c *compiler) addTableCtorN(fields []staticStringField) int {
	for i := range c.proto.TableCtorsN {
		ctor := &c.proto.TableCtorsN[i]
		if len(ctor.KeyConsts) != len(fields) {
			continue
		}
		match := true
		for j := range fields {
			if ctor.KeyConsts[j] != fields[j].keyConst {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	if len(c.proto.TableCtorsN) >= 256 {
		return -1
	}
	keyConsts := make([]int, len(fields))
	keys := make([]string, len(fields))
	for i := range fields {
		keyConsts[i] = fields[i].keyConst
		keys[i] = fields[i].key
	}
	c.proto.TableCtorsN = append(c.proto.TableCtorsN, TableCtorN{
		KeyConsts: keyConsts,
		Runtime:   runtime.NewSmallTableCtorN(keys),
	})
	return len(c.proto.TableCtorsN) - 1
}

func isNilLiteral(e ast.Expr) bool {
	_, ok := e.(*ast.NilLit)
	return ok
}
