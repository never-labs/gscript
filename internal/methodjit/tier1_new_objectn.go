//go:build darwin && arm64

package methodjit

import (
	"unsafe"

	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/vm"
)

func emitBaselineNewObjectN(asm *jit.Assembler, inst uint32, pc int, proto *vm.FuncProto, caches []newTableCacheEntry, preserveCoroutinePayloadFastPath bool) {
	if !baselineNewObjectNCacheable(proto, inst) {
		emitBaselineOpExit(asm, inst, pc, vm.OP_NEWOBJECTN)
		return
	}

	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)
	c := vm.DecodeC(inst)
	ctor := &proto.TableCtorsN[b].Runtime
	n := len(ctor.Keys)

	exitLabel := nextLabel("newobjectn_exit")
	doneLabel := nextLabel("newobjectn_done")

	asm.LoadImm64(jit.X7, nb64(jit.NB_ValNil))
	for i := 0; i < n; i++ {
		valReg := jit.Reg(int(jit.X8) + i)
		loadSlot(asm, valReg, c+i)
		asm.CMPreg(valReg, jit.X7)
		asm.BCond(jit.CondEQ, exitLabel)
	}

	if !preserveCoroutinePayloadFastPath && pc >= 0 && pc < len(caches) {
		cacheBase := uintptr(unsafe.Pointer(&caches[0]))
		entryOff := pc * newTableCacheEntrySize
		asm.LoadImm64(jit.X2, int64(cacheBase))
		if entryOff > 0 {
			if entryOff <= 4095 {
				asm.ADDimm(jit.X2, jit.X2, uint16(entryOff))
			} else {
				asm.LoadImm64(jit.X3, int64(entryOff))
				asm.ADDreg(jit.X2, jit.X2, jit.X3)
			}
		}
		asm.LDR(jit.X0, jit.X2, newTableCacheEntryValuesOff)
		asm.CBZ(jit.X0, exitLabel)
		asm.LDR(jit.X3, jit.X2, newTableCacheEntryPosOff)
		asm.LDR(jit.X4, jit.X2, newTableCacheEntryLenOff)
		asm.CMPreg(jit.X3, jit.X4)
		asm.BCond(jit.CondGE, exitLabel)
		asm.LDRreg(jit.X0, jit.X0, jit.X3)
		asm.ADDimm(jit.X3, jit.X3, 1)
		asm.STR(jit.X3, jit.X2, newTableCacheEntryPosOff)

		jit.EmitExtractPtr(asm, jit.X1, jit.X0)
		asm.LDR(jit.X2, jit.X1, jit.TableOffSvals)
		for i := 0; i < n; i++ {
			asm.STR(jit.Reg(int(jit.X8)+i), jit.X2, i*jit.ValueSize)
		}
		storeSlot(asm, a, jit.X0)
		asm.B(doneLabel)
	}

	asm.LDR(jit.X1, mRegCtx, execCtxOffCoroutineCurrentPtr)
	asm.CBZ(jit.X1, exitLabel)
	asm.LDRB(jit.X2, jit.X1, vm.VMCoroutineStackYieldEnabledOffset())
	asm.CBZ(jit.X2, exitLabel)
	asm.LDR(jit.X1, jit.X1, vm.VMCoroutinePooledFixedRecordOffset())
	asm.CBZ(jit.X1, exitLabel)

	asm.LoadImm64(jit.X2, int64(uintptr(unsafe.Pointer(ctor))))
	asm.STR(jit.X2, jit.X1, jit.FixedRecordOffCtor)
	asm.MOVimm16(jit.X2, 0)
	asm.STR(jit.X2, jit.X1, jit.FixedRecordOffMaterialized)
	asm.LoadImm64(jit.X2, int64(ctor.Shape.ID))
	asm.STRW(jit.X2, jit.X1, jit.FixedRecordOffShapeID)
	asm.MOVimm16(jit.X2, uint16(n))
	asm.STRB(jit.X2, jit.X1, jit.FixedRecordOffN)
	for i := 0; i < n; i++ {
		asm.STR(jit.Reg(int(jit.X8)+i), jit.X1, jit.FixedRecordOffValues+i*jit.ValueSize)
	}

	asm.LoadImm64(jit.X2, nb64(jit.NB_TagPtr|(uint64(jit.NB_PtrSubFixedRecord)<<jit.NB_PtrSubShift)))
	asm.ORRreg(jit.X0, jit.X1, jit.X2)
	storeSlot(asm, a, jit.X0)
	asm.B(doneLabel)

	asm.Label(exitLabel)
	emitBaselineOpExit(asm, inst, pc, vm.OP_NEWOBJECTN)
	asm.Label(doneLabel)
}
