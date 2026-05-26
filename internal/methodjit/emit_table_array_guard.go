//go:build darwin && arm64

// emit_table_array_guard.go: table-kind/header verification guards
// (OpTableArrayHeader, OpGuardTableKind). Pure code movement from
// emit_table_array.go; no behavior change.

package methodjit

import (
	"github.com/gscript/gscript/internal/jit"
)

func (ec *emitContext) emitTableArrayHeader(instr *Instr) {
	if len(instr.Args) < 1 {
		return
	}
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("tarr_header_deopt")
	doneLabel := ec.uniqueLabel("tarr_header_done")

	expectedKind, ok := fbKindToAK(instr.Aux)
	if !ok {
		ec.emitDeopt(instr)
		return
	}

	tblID := instr.Args[0].ID
	ec.resolveValueToReg(tblID, jit.X0)
	deoptActiveRegs := cloneBoolMap(ec.activeRegs)
	deoptActiveFPRegs := cloneBoolMap(ec.activeFPRegs)
	deoptReprs := ec.snapshotValueReprs()
	if ec.tableVerified[tblID] {
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	} else if ec.isLocalNewTableWithoutMetatable(instr.Args[0]) {
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		ec.tableVerified[tblID] = true
	} else if ec.irTypes[tblID] == TypeTable {
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		// TypeTable producers/guards already exclude nil. Keep the dynamic
		// metatable and array-kind checks, but avoid repeating the nil check
		// for row tables loaded from mixed table arrays.
		asm.LDR(jit.X1, jit.X0, jit.TableOffMetatable)
		asm.CBNZ(jit.X1, deoptLabel)
		ec.tableVerified[tblID] = true
	} else {
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, deoptLabel)
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		asm.CBZ(jit.X0, deoptLabel)
		asm.LDR(jit.X1, jit.X0, jit.TableOffMetatable)
		asm.CBNZ(jit.X1, deoptLabel)
		ec.tableVerified[tblID] = true
	}
	if fbKind, ok := ec.localNewTableFBKind(instr.Args[0]); ok && fbKind == uint16(instr.Aux) {
		ec.kindVerified[tblID] = fbKind
	}
	if ec.kindVerified[tblID] != uint16(instr.Aux) {
		asm.LDRB(jit.X1, jit.X0, jit.TableOffArrayKind)
		asm.CMPimm(jit.X1, expectedKind)
		asm.BCond(jit.CondNE, deoptLabel)
	}
	ec.kindVerified[tblID] = uint16(instr.Aux)
	ec.storeRawTablePtr(jit.X0, instr.ID)
	successActiveRegs := cloneBoolMap(ec.activeRegs)
	successActiveFPRegs := cloneBoolMap(ec.activeFPRegs)
	successReprs := ec.snapshotValueReprs()
	asm.B(doneLabel)

	asm.Label(deoptLabel)
	ec.activeRegs = deoptActiveRegs
	ec.activeFPRegs = deoptActiveFPRegs
	ec.restoreValueReprSnapshot(deoptReprs)
	if instr.Aux2&tableArrayHeaderFlagHoisted != 0 {
		ec.emitDeopt(instr)
	} else {
		ec.emitPreciseDeopt(instr)
	}
	ec.activeRegs = successActiveRegs
	ec.activeFPRegs = successActiveFPRegs
	ec.restoreValueReprSnapshot(successReprs)
	asm.Label(doneLabel)
}

func (ec *emitContext) emitGuardTableKind(instr *Instr) {
	if len(instr.Args) == 0 {
		return
	}
	expectedKind, ok := fbKindToAK(instr.Aux)
	if !ok {
		ec.emitDeopt(instr)
		return
	}
	asm := ec.asm
	deoptLabel := ec.uniqueLabel("guard_table_kind_deopt")
	doneLabel := ec.uniqueLabel("guard_table_kind_done")
	tableID := instr.Args[0].ID
	ec.resolveValueToReg(tableID, jit.X0)
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, deoptLabel)
	jit.EmitExtractPtr(asm, jit.X1, jit.X0)
	asm.CBZ(jit.X1, deoptLabel)
	asm.LDR(jit.X2, jit.X1, jit.TableOffMetatable)
	asm.CBNZ(jit.X2, deoptLabel)
	asm.LDRB(jit.X2, jit.X1, jit.TableOffArrayKind)
	asm.CMPimm(jit.X2, expectedKind)
	asm.BCond(jit.CondNE, deoptLabel)
	ec.storeResultNB(jit.X0, instr.ID)
	ec.emitGuardDeoptExit(instr, deoptLabel, doneLabel, false)
}
