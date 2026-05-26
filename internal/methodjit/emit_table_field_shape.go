//go:build darwin && arm64

// emit_table_field_shape.go: shape-id materialization and shape field-type/
// vmclosure guard code paths. Pure code movement from emit_table_field.go.

package methodjit

import (
	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/runtime"
)

func (ec *emitContext) emitTableShapeID(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("table_shape_deopt")
	doneLabel := ec.uniqueLabel("table_shape_done")
	tblID := instr.Args[0].ID
	rawTablePtr := ec.valueReprOf(tblID) == valueReprRawTablePtr
	var tblReg jit.Reg
	if rawTablePtr {
		tblReg = ec.resolveRawTablePtr(tblID, jit.X0)
	} else {
		tblReg = ec.resolveValueNB(tblID, jit.X0)
	}
	if tblReg != jit.X0 {
		asm.MOVreg(jit.X0, tblReg)
	}
	if ec.tableVerified[tblID] || ec.irTypes[tblID] == TypeTable {
		if !rawTablePtr {
			jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		}
	} else {
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, deoptLabel)
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		asm.CBZ(jit.X0, deoptLabel)
		ec.tableVerified[tblID] = true
	}
	asm.LDRW(jit.X3, jit.X0, jit.TableOffShapeID)
	ec.storeRawTablePtr(jit.X0, tblID)
	ec.storeRawInt(jit.X3, instr.ID)
	ec.emitGuardDeoptExit(instr, deoptLabel, doneLabel, true)
}
func (ec *emitContext) emitGuardShapeFieldType(instr *Instr) {
	shapeID := uint32(instr.Aux >> 32)
	fieldIdx := int(int32(instr.Aux & 0xFFFFFFFF))
	want := Type(instr.Aux2)
	runtimeType, ok := irTypeToRuntimeValueType(want)
	deoptLabel := ec.uniqueLabel("guard_shape_field_type_deopt")
	doneLabel := ec.uniqueLabel("guard_shape_field_type_done")
	if !ok {
		ec.emitPreciseDeopt(instr)
		return
	}
	if got, stable := runtime.ShapeFieldStableType(shapeID, fieldIdx); !stable || got != runtimeType {
		ec.emitPreciseDeopt(instr)
		return
	}
	epochPtr := uintptr(runtime.ShapeFieldTypeEpochPtr(shapeID, fieldIdx))
	if epochPtr == 0 {
		ec.emitPreciseDeopt(instr)
		return
	}
	epoch := runtime.ShapeFieldTypeEpoch(shapeID, fieldIdx)
	ec.asm.LoadImm64(jit.X8, int64(epochPtr))
	ec.asm.LDR(jit.X8, jit.X8, 0)
	ec.emitCMPRegConst(jit.X8, int64(epoch), jit.X9)
	ec.asm.BCond(jit.CondEQ, doneLabel)
	ec.asm.Label(deoptLabel)
	ec.emitPreciseDeopt(instr)
	ec.asm.Label(doneLabel)
}

func (ec *emitContext) emitGuardShapeFieldTypeMask(instr *Instr) {
	shapeID := uint32(instr.Aux >> 32)
	want := Type(uint32(instr.Aux))
	mask := uint64(instr.Aux2)
	runtimeType, ok := irTypeToRuntimeValueType(want)
	deoptLabel := ec.uniqueLabel("guard_shape_field_type_mask_deopt")
	doneLabel := ec.uniqueLabel("guard_shape_field_type_mask_done")
	if !ok || shapeID == 0 || mask == 0 {
		ec.emitPreciseDeopt(instr)
		return
	}
	basePtr := uintptr(runtime.ShapeFieldTypeEpochPtr(shapeID, 0))
	if basePtr == 0 {
		ec.emitPreciseDeopt(instr)
		return
	}
	ec.asm.LoadImm64(jit.X8, int64(basePtr))
	for fieldIdx := 0; fieldIdx < 64; fieldIdx++ {
		if mask&(uint64(1)<<uint(fieldIdx)) == 0 {
			continue
		}
		if got, stable := runtime.ShapeFieldStableType(shapeID, fieldIdx); !stable || got != runtimeType {
			ec.emitPreciseDeopt(instr)
			return
		}
		epoch := runtime.ShapeFieldTypeEpoch(shapeID, fieldIdx)
		offset := fieldIdx * 8
		if offset <= 32760 {
			ec.asm.LDR(jit.X9, jit.X8, offset)
		} else {
			ec.asm.LoadImm64(jit.X10, int64(offset))
			ec.asm.LDRreg(jit.X9, jit.X8, jit.X10)
		}
		ec.emitCMPRegConst(jit.X9, int64(epoch), jit.X10)
		ec.asm.BCond(jit.CondNE, deoptLabel)
	}
	ec.emitGuardDeoptExit(instr, deoptLabel, doneLabel, true)
}

func (ec *emitContext) emitGuardShapeFieldVMClosure(instr *Instr) {
	shapeID := uint32(instr.Aux >> 32)
	fieldIdx := int(int32(instr.Aux & 0xFFFFFFFF))
	wantClosure := uintptr(instr.Aux2)
	deoptLabel := ec.uniqueLabel("guard_shape_field_vmclosure_deopt")
	doneLabel := ec.uniqueLabel("guard_shape_field_vmclosure_done")
	if shapeID == 0 || fieldIdx < 0 || wantClosure == 0 {
		ec.emitPreciseDeopt(instr)
		return
	}
	got, stable := runtime.ShapeFieldStableVMClosure(shapeID, fieldIdx)
	if !stable || got != wantClosure {
		ec.emitPreciseDeopt(instr)
		return
	}
	epochPtr := uintptr(runtime.ShapeFieldVMClosureEpochPtr(shapeID, fieldIdx))
	if epochPtr == 0 {
		ec.emitPreciseDeopt(instr)
		return
	}
	epoch := runtime.ShapeFieldVMClosureEpoch(shapeID, fieldIdx)
	ec.asm.LoadImm64(jit.X8, int64(epochPtr))
	ec.asm.LDR(jit.X8, jit.X8, 0)
	ec.emitCMPRegConst(jit.X8, int64(epoch), jit.X9)
	ec.asm.BCond(jit.CondEQ, doneLabel)
	ec.asm.Label(deoptLabel)
	ec.emitPreciseDeopt(instr)
	ec.asm.Label(doneLabel)
}

func (ec *emitContext) emitCMPRegConst(reg jit.Reg, value int64, scratch jit.Reg) {
	if value >= 0 && value <= 0xFFF {
		ec.asm.CMPimm(reg, uint16(value))
		return
	}
	ec.asm.LoadImm64(scratch, value)
	ec.asm.CMPreg(reg, scratch)
}

func irTypeToRuntimeValueType(t Type) (runtime.ValueType, bool) {
	switch t {
	case TypeInt:
		return runtime.TypeInt, true
	case TypeFloat:
		return runtime.TypeFloat, true
	case TypeBool:
		return runtime.TypeBool, true
	case TypeString:
		return runtime.TypeString, true
	case TypeTable:
		return runtime.TypeTable, true
	default:
		return runtime.TypeNil, false
	}
}
