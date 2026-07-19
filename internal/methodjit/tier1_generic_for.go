//go:build darwin && arm64

package methodjit

import (
	"github.com/never-labs/leia/internal/jit"
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// emitBaselineTForCall handles dense, in-bounds standard ipairs iterations in
// native code. Boundary, metamethod, sparse, concurrent, lazy and non-standard
// cases retain the existing OP_TFORCALL exit-resume semantics.
func emitBaselineTForCall(asm *jit.Assembler, inst uint32, pc int) {
	a := vm.DecodeA(inst)
	c := vm.DecodeC(inst)
	slowLabel := nextLabel("tforcall_slow")
	doneLabel := nextLabel("tforcall_done")
	nilLabel := nextLabel("tforcall_nil")
	intArrayLabel := nextLabel("tforcall_int_array")
	floatArrayLabel := nextLabel("tforcall_float_array")
	boolArrayLabel := nextLabel("tforcall_bool_array")
	valueReadyLabel := nextLabel("tforcall_value_ready")

	// Require the runtime-owned standard ipairs iterator. User iterators and
	// replaced globals retain the generic path.
	loadSlot(asm, jit.X0, a)
	emitStdNativeFunctionIdentityGuard(asm, jit.X0, runtime.NativeKindStdIPairsIter, runtime.StdIPairsIdentityPtr(), slowLabel)

	// State must be a plain table. Metatables use vm.tableGet so __index remains
	// observable; concurrent and lazy tables require their Go-side protocols.
	loadSlot(asm, jit.X0, a+1)
	jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, slowLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	asm.CBZ(jit.X0, slowLabel)
	asm.LDR(jit.X1, jit.X0, 0) // Table.mu
	asm.CBNZ(jit.X1, slowLabel)
	asm.LDR(jit.X1, jit.X0, jit.TableOffMetatable)
	asm.CBNZ(jit.X1, slowLabel)
	asm.LDR(jit.X1, jit.X0, jit.TableOffLazyTree)
	asm.CBNZ(jit.X1, slowLabel)

	// The standard setup starts at integer zero and TFORLOOP feeds each integer
	// key back as the next cursor. Exotic numeric cursors stay on the fallback.
	loadSlot(asm, jit.X1, a+2)
	asm.LSRimm(jit.X2, jit.X1, 48)
	asm.MOVimm16(jit.X3, uint16(jit.NB_TagIntShr48))
	asm.CMPreg(jit.X2, jit.X3)
	asm.BCond(jit.CondNE, slowLabel)
	asm.SBFX(jit.X1, jit.X1, 0, 48)
	asm.CMPimm(jit.X1, 0)
	asm.BCond(jit.CondLT, slowLabel)
	asm.ADDimm(jit.X1, jit.X1, 1) // X1 = next ipairs key / array index

	// Dispatch over every dense array representation supported by RawGetInt.
	asm.LDRB(jit.X2, jit.X0, jit.TableOffArrayKind)
	asm.CMPimm(jit.X2, jit.AKBool)
	asm.BCond(jit.CondEQ, boolArrayLabel)
	asm.CMPimm(jit.X2, jit.AKFloat)
	asm.BCond(jit.CondEQ, floatArrayLabel)
	asm.CMPimm(jit.X2, jit.AKInt)
	asm.BCond(jit.CondEQ, intArrayLabel)
	asm.CBNZ(jit.X2, slowLabel)

	asm.LDR(jit.X2, jit.X0, jit.TableOffArrayLen)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffArray)
	asm.LDRreg(jit.X2, jit.X2, jit.X1)
	asm.B(valueReadyLabel)

	asm.Label(intArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffIntArrayLen)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffIntArray)
	asm.LDRreg(jit.X2, jit.X2, jit.X1)
	jit.EmitBoxIntFast(asm, jit.X2, jit.X2, mRegTagInt)
	asm.B(valueReadyLabel)

	asm.Label(floatArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffFloatArrayLen)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffFloatArray)
	asm.LDRreg(jit.X2, jit.X2, jit.X1)
	asm.B(valueReadyLabel)

	asm.Label(boolArrayLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffBoolArrayLen)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondGE, slowLabel)
	asm.LDR(jit.X2, jit.X0, jit.TableOffBoolArray)
	asm.LDRBreg(jit.X2, jit.X2, jit.X1)
	asm.CBZ(jit.X2, nilLabel)
	asm.SUBimm(jit.X2, jit.X2, 1)
	asm.ORRreg(jit.X2, jit.X2, mRegTagBool)

	asm.Label(valueReadyLabel)
	asm.LoadImm64(jit.X3, nb64(jit.NB_ValNil))
	asm.CMPreg(jit.X2, jit.X3)
	asm.BCond(jit.CondEQ, nilLabel)
	if c > 0 {
		jit.EmitBoxIntFast(asm, jit.X3, jit.X1, mRegTagInt)
		storeSlot(asm, a+3, jit.X3)
	}
	if c > 1 {
		storeSlot(asm, a+4, jit.X2)
	}
	for i := 2; i < c; i++ {
		asm.LoadImm64(jit.X3, nb64(jit.NB_ValNil))
		storeSlot(asm, a+3+i, jit.X3)
	}
	asm.B(doneLabel)

	asm.Label(nilLabel)
	asm.LoadImm64(jit.X2, nb64(jit.NB_ValNil))
	for i := 0; i < c; i++ {
		storeSlot(asm, a+3+i, jit.X2)
	}
	asm.B(doneLabel)

	asm.Label(slowLabel)
	emitBaselineOpExitCommon(asm, vm.OP_TFORCALL, pc, a, vm.DecodeB(inst), c)
	asm.Label(doneLabel)
}
