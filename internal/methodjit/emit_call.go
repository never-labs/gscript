//go:build darwin && arm64

// emit_call.go handles deoptimization and extended operations for the Method JIT.
// When the JIT encounters an unsupported operation (calls, globals, table ops,
// concat, etc.), it "deopts" by setting ExitCode=2 in ExecContext and returning
// to Go. The Go-side Execute method then falls back to the VM interpreter.
//
// This file also implements operations that don't fit in emit.go:
// - OpDiv (float division, always returns float)
// - OpUnm (unary negate for int and float)
// - OpNot (logical not)
// - Float-aware arithmetic (OpAdd, OpSub, OpMul with float operands)

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/vm"
)

func (ec *emitContext) emitLenNative(instr *Instr) {
	if len(instr.Args) == 0 {
		ec.emitOpExit(instr)
		return
	}
	if lenArgKnownRawString(instr.Args[0]) {
		ec.emitRawStringLenNative(instr)
		return
	}
	asm := ec.asm
	slowLabel := ec.uniqueLabel("len_slow")
	doneLabel := ec.uniqueLabel("len_done")
	mixedLabel := ec.uniqueLabel("len_mixed")
	intLabel := ec.uniqueLabel("len_int")
	floatLabel := ec.uniqueLabel("len_float")
	boxLabel := ec.uniqueLabel("len_box_result")
	notStringLabel := ec.uniqueLabel("len_not_string")

	src := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if src != jit.X0 {
		asm.MOVreg(jit.X0, src)
	}

	if ec.isLocalNewTableWithoutMetatable(instr.Args[0]) {
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		ec.tableVerified[instr.Args[0].ID] = true
		if fbKind, ok := ec.localNewTableFBKind(instr.Args[0]); ok && ec.emitKnownTableLenNative(fbKind, slowLabel, boxLabel) {
			ec.kindVerified[instr.Args[0].ID] = fbKind
		} else {
			ec.emitTableLenKindDispatch(slowLabel, mixedLabel, intLabel, floatLabel)
		}
	} else {
		jit.EmitCheckIsString(asm, jit.X0, jit.X1, jit.X2, notStringLabel)
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		asm.LDR(jit.X1, jit.X0, 8) // Go string header length.
		asm.B(boxLabel)

		asm.Label(notStringLabel)
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, slowLabel)
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
		asm.CBZ(jit.X0, slowLabel)

		// Respect __len by falling back when a table has a metatable.
		asm.LDR(jit.X1, jit.X0, jit.TableOffMetatable)
		asm.CBNZ(jit.X1, slowLabel)
		ec.emitTableLenKindDispatch(slowLabel, mixedLabel, intLabel, floatLabel)
	}

	// Mixed arrays need the runtime's trailing-nil scan. Fast-path only when
	// the last array slot is non-nil, which is the common dense-array case.
	asm.Label(mixedLabel)
	ec.emitMixedTableLenNative(slowLabel, boxLabel)

	asm.Label(intLabel)
	ec.emitDenseTableLenNative(jit.TableOffIntArrayLen, boxLabel)

	asm.Label(floatLabel)
	ec.emitDenseTableLenNative(jit.TableOffFloatArrayLen, boxLabel)

	asm.Label(boxLabel)
	if instr.Type == TypeInt {
		ec.storeRawInt(jit.X1, instr.ID)
		asm.B(doneLabel)
	} else {
		jit.EmitBoxIntFast(asm, jit.X0, jit.X1, mRegTagInt)
		ec.storeResultNB(jit.X0, instr.ID)
		asm.B(doneLabel)
	}

	asm.Label(slowLabel)
	ec.emitOpExit(instr)
	asm.Label(doneLabel)
}

func (ec *emitContext) emitTableLenKindDispatch(slowLabel, mixedLabel, intLabel, floatLabel string) {
	asm := ec.asm
	asm.LDRB(jit.X1, jit.X0, jit.TableOffArrayKind)
	asm.CMPimm(jit.X1, jit.AKMixed)
	asm.BCond(jit.CondEQ, mixedLabel)
	asm.CMPimm(jit.X1, jit.AKInt)
	asm.BCond(jit.CondEQ, intLabel)
	asm.CMPimm(jit.X1, jit.AKFloat)
	asm.BCond(jit.CondEQ, floatLabel)
	asm.B(slowLabel)
}

func (ec *emitContext) emitKnownTableLenNative(fbKind uint16, slowLabel, boxLabel string) bool {
	switch fbKind {
	case uint16(vm.FBKindMixed):
		ec.emitMixedTableLenNative(slowLabel, boxLabel)
	case uint16(vm.FBKindInt):
		ec.emitDenseTableLenNative(jit.TableOffIntArrayLen, boxLabel)
	case uint16(vm.FBKindFloat):
		ec.emitDenseTableLenNative(jit.TableOffFloatArrayLen, boxLabel)
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitMixedTableLenNative(slowLabel, boxLabel string) {
	asm := ec.asm
	asm.LDR(jit.X1, jit.X0, jit.TableOffArrayLen)
	asm.CBZ(jit.X1, boxLabel)
	asm.SUBimm(jit.X1, jit.X1, 1)
	asm.CBZ(jit.X1, boxLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffArray)
	asm.LDRreg(jit.X3, jit.X2, jit.X1)
	asm.LoadImm64(jit.X2, nb64(jit.NB_ValNil))
	asm.CMPreg(jit.X3, jit.X2)
	asm.BCond(jit.CondEQ, slowLabel)
	asm.B(boxLabel)
}

func (ec *emitContext) emitDenseTableLenNative(lenOff int, boxLabel string) {
	asm := ec.asm
	asm.LDR(jit.X1, jit.X0, lenOff)
	asm.CBZ(jit.X1, boxLabel)
	asm.SUBimm(jit.X1, jit.X1, 1)
	asm.B(boxLabel)
}

func lenArgKnownRawString(v *Value) bool {
	if v == nil || v.Def == nil {
		return false
	}
	if v.Def.Type == TypeString {
		return true
	}
	return opHasRawStringResult(v.Def.Op)
}

func (ec *emitContext) emitRawStringLenNative(instr *Instr) {
	asm := ec.asm
	src := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
	if src != jit.X0 {
		asm.MOVreg(jit.X0, src)
	}
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.LDR(jit.X1, jit.X0, 8) // Go string header length.
	if instr.Type == TypeInt {
		ec.storeRawInt(jit.X1, instr.ID)
		return
	}
	jit.EmitBoxIntFast(asm, jit.X0, jit.X1, mRegTagInt)
	ec.storeResultNB(jit.X0, instr.ID)
}

// uniqueLabel generates a unique label for the emitter to avoid collisions.
func (ec *emitContext) uniqueLabel(prefix string) string {
	ec.labelCounter++
	return fmt.Sprintf("%s_%d", prefix, ec.labelCounter)
}
