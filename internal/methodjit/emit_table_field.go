//go:build darwin && arm64

// emit_table_field.go implements ARM64 code generation for table field
// operations (OpGetField, OpSetField) in the Method JIT. These use inline
// shape-guarded access with deopt fallback when the field cache is available.
//
// This file retains the shared field-svals cache state helpers and the table
// pointer/shape preparation used across the load (emit_table_field_load.go),
// store (emit_table_field_store.go), and shape (emit_table_field_shape.go)
// code paths in the same package.

package methodjit

import (
	"github.com/never-labs/leia/internal/jit"
)

func (ec *emitContext) hasFieldSvalsCache(tblValueID int, shapeID uint32) bool {
	return ec.fieldSvalsCacheValid &&
		ec.fieldSvalsCacheTableID == tblValueID &&
		ec.fieldSvalsCacheShapeID == shapeID
}

func (ec *emitContext) rememberFieldSvalsCache(tblValueID int, shapeID uint32) {
	if shapeID == 0 {
		ec.invalidateFieldSvalsCache()
		return
	}
	ec.fieldSvalsCacheValid = true
	ec.fieldSvalsCacheTableID = tblValueID
	ec.fieldSvalsCacheShapeID = shapeID
}

func (ec *emitContext) invalidateFieldSvalsCache() {
	ec.fieldSvalsCacheValid = false
	ec.fieldSvalsCacheTableID = 0
	ec.fieldSvalsCacheShapeID = 0
}

// emitPrepareFieldTablePtr leaves the raw *Table pointer in X0 and returns
// true when the field shape was already verified in this block. Raw table
// pointer values have already proved the receiver is a non-string table.
// Speculative IR TypeTable alone is not enough: recursive arguments can be
// warmed as tables and later receive non-table values, so boxed values still
// need a tag check until this block verifies them.
func (ec *emitContext) emitPrepareFieldTablePtr(tblValueID int, shapeID uint32, deoptLabel string) bool {
	asm := ec.asm
	if ec.hasReg(tblValueID) && ec.valueReprOf(tblValueID) == valueReprRawTablePtr {
		tblReg := ec.physReg(tblValueID)
		if tblReg != jit.X0 {
			asm.MOVreg(jit.X0, tblReg)
		}
		if prevShape, ok := ec.shapeVerified[tblValueID]; ok && prevShape == shapeID {
			return true
		}
		asm.LDRW(jit.X1, jit.X0, jit.TableOffShapeID)
		emitCMPWConst(asm, jit.X1, jit.X2, int64(shapeID))
		asm.BCond(jit.CondNE, deoptLabel)
		ec.shapeVerified[tblValueID] = shapeID
		return false
	}
	if ec.tableValueAlreadyChecked(tblValueID) {
		tblReg := ec.resolveRawTablePtr(tblValueID, jit.X0)
		if tblReg != jit.X0 {
			asm.MOVreg(jit.X0, tblReg)
		}
		ec.tableVerified[tblValueID] = true
		if prevShape, ok := ec.shapeVerified[tblValueID]; ok && prevShape == shapeID {
			return true
		}
		asm.LDRW(jit.X1, jit.X0, jit.TableOffShapeID)
		emitCMPWConst(asm, jit.X1, jit.X2, int64(shapeID))
		asm.BCond(jit.CondNE, deoptLabel)
		ec.shapeVerified[tblValueID] = shapeID
		return false
	}
	ec.resolveValueToReg(tblValueID, jit.X0)
	if prevShape, ok := ec.shapeVerified[tblValueID]; ok && prevShape == shapeID {
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		return true
	}
	if !ec.tableVerified[tblValueID] {
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, deoptLabel)
		ec.tableVerified[tblValueID] = true
	}
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, deoptLabel)
	asm.LDRW(jit.X1, jit.X0, jit.TableOffShapeID)
	emitCMPWConst(asm, jit.X1, jit.X2, int64(shapeID))
	asm.BCond(jit.CondNE, deoptLabel)
	ec.shapeVerified[tblValueID] = shapeID
	return false
}

func (ec *emitContext) tableValueAlreadyChecked(valueID int) bool {
	if ec == nil || ec.irTypes == nil || ec.irTypes[valueID] != TypeTable || ec.valueDefs == nil {
		return false
	}
	def := ec.valueDefs[valueID]
	if def == nil {
		return false
	}
	switch def.Op {
	case OpTableArrayLoad, OpNewFixedTable:
		return true
	default:
		return false
	}
}
