//go:build darwin && arm64

// tier1_dynamic_string_cache.go emits ARM64 templates for the baseline
// dynamic string-key cache: the shared cache-probe primitive and the
// GETTABLE/SETTABLE string-key entry points built on top of it. Pure code
// movement out of tier1_table.go.

package methodjit

import (
	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

func emitBaselineDynamicStringCacheProbe(asm *jit.Assembler, pc int, slowLabel string, hit func(fieldIdxReg jit.Reg), valueHit func(valueReg jit.Reg)) {
	// Inputs: X0 = *Table, X1 = NaN-boxed string candidate.
	// Clobbers X2-X11. Falls through to slowLabel on cache miss.
	jit.EmitCheckIsString(asm, jit.X1, jit.X2, jit.X3, slowLabel)
	jit.EmitExtractPtr(asm, jit.X4, jit.X1) // X4 = *string header
	asm.CBZ(jit.X4, slowLabel)
	asm.LDR(jit.X5, jit.X4, 0) // X5 = string data
	asm.LDR(jit.X6, jit.X4, 8) // X6 = string len

	asm.LDR(jit.X3, mRegCtx, execCtxOffBaselineTableStringKeyCache)
	asm.CBZ(jit.X3, slowLabel)
	entryOff := pc * runtime.TableStringKeyCacheWays * tableStringKeyCacheEntrySize
	if entryOff > 0 {
		if entryOff <= 4095 {
			asm.ADDimm(jit.X3, jit.X3, uint16(entryOff))
		} else {
			asm.LoadImm64(jit.X7, int64(entryOff))
			asm.ADDreg(jit.X3, jit.X3, jit.X7)
		}
	}

	asm.LDR(jit.X8, jit.X0, jit.TableOffLazyTree)
	asm.CBNZ(jit.X8, slowLabel)
	asm.LDRW(jit.X7, jit.X0, jit.TableOffShapeID)
	smapCacheLabel := nextLabel("dyn_string_smap_cache")
	asm.CBZ(jit.X7, smapCacheLabel)

	loopLabel := nextLabel("dyn_string_cache_loop")
	nextEntryLabel := nextLabel("dyn_string_cache_next")
	asm.MOVimm16(jit.X9, 0)
	asm.Label(loopLabel)
	asm.LDR(jit.X10, jit.X3, tableStringKeyCacheEntryKeyData)
	asm.CMPreg(jit.X10, jit.X5)
	asm.BCond(jit.CondNE, nextEntryLabel)
	asm.LDR(jit.X10, jit.X3, tableStringKeyCacheEntryKeyLen)
	asm.CMPreg(jit.X10, jit.X6)
	asm.BCond(jit.CondNE, nextEntryLabel)
	asm.LDRW(jit.X10, jit.X3, tableStringKeyCacheEntryShapeID)
	asm.CMPreg(jit.X10, jit.X7)
	asm.BCond(jit.CondNE, nextEntryLabel)
	asm.LDR(jit.X11, jit.X3, tableStringKeyCacheEntryFieldIdx)
	asm.LDR(jit.X10, jit.X0, jit.TableOffSvalsLen)
	asm.CMPreg(jit.X11, jit.X10)
	asm.BCond(jit.CondGE, slowLabel)
	hit(jit.X11)

	asm.Label(nextEntryLabel)
	asm.ADDimm(jit.X3, jit.X3, uint16(tableStringKeyCacheEntrySize))
	asm.ADDimm(jit.X9, jit.X9, 1)
	asm.CMPimm(jit.X9, runtime.TableStringKeyCacheWays)
	asm.BCond(jit.CondLT, loopLabel)
	asm.B(slowLabel)

	asm.Label(smapCacheLabel)
	if valueHit == nil {
		asm.B(slowLabel)
		return
	}
	asm.LDR(jit.X8, jit.X0, jit.TableOffStringLookupCache)
	asm.CBZ(jit.X8, slowLabel)
	asm.LDR(jit.X3, jit.X8, jit.StringLookupCacheOffEntries)
	asm.CBZ(jit.X3, slowLabel)
	asm.LDR(jit.X10, jit.X8, jit.StringLookupCacheOffMask)
	hashLoopLabel := nextLabel("dyn_string_smap_hash_loop")
	hashDoneLabel := nextLabel("dyn_string_smap_hash_done")
	asm.LoadImm64(jit.X9, int64(1469598103934665603))
	asm.LoadImm64(jit.X15, int64(1099511628211))
	asm.MOVimm16(jit.X11, 0)
	asm.Label(hashLoopLabel)
	asm.CMPreg(jit.X11, jit.X6)
	asm.BCond(jit.CondGE, hashDoneLabel)
	asm.LDRBreg(jit.X14, jit.X5, jit.X11)
	asm.EORreg(jit.X9, jit.X9, jit.X14)
	asm.MUL(jit.X9, jit.X9, jit.X15)
	asm.ADDimm(jit.X11, jit.X11, 1)
	asm.B(hashLoopLabel)
	asm.Label(hashDoneLabel)
	asm.MOVreg(jit.X15, jit.X9)
	asm.ANDreg(jit.X9, jit.X9, jit.X10)

	smapLoopLabel := nextLabel("dyn_string_smap_loop")
	smapNextLabel := nextLabel("dyn_string_smap_next")
	smapFoundLabel := nextLabel("dyn_string_smap_found")
	smapByteLoopLabel := nextLabel("dyn_string_smap_bytes")
	asm.MOVimm16(jit.X13, 0)
	asm.Label(smapLoopLabel)
	asm.ADDreg(jit.X11, jit.X9, jit.X13)
	asm.ANDreg(jit.X11, jit.X11, jit.X10)
	asm.LSLimm(jit.X12, jit.X11, 6) // idx * 64
	asm.ADDreg(jit.X12, jit.X3, jit.X12)
	asm.LDRB(jit.X14, jit.X12, jit.StringLookupCacheEntryOffValid)
	asm.CBZ(jit.X14, slowLabel)
	asm.LDR(jit.X14, jit.X12, jit.StringLookupCacheEntryOffHash)
	asm.CMPreg(jit.X14, jit.X15)
	asm.BCond(jit.CondNE, smapNextLabel)
	asm.LDR(jit.X14, jit.X12, jit.StringLookupCacheEntryOffKeyLen)
	asm.CMPreg(jit.X14, jit.X6)
	asm.BCond(jit.CondNE, smapNextLabel)
	asm.LDR(jit.X14, jit.X12, jit.StringLookupCacheEntryOffKeyData)
	asm.CMPreg(jit.X14, jit.X5)
	asm.BCond(jit.CondEQ, smapFoundLabel)
	asm.CBZ(jit.X6, smapFoundLabel)
	asm.MOVimm16(jit.X15, 0)
	asm.Label(smapByteLoopLabel)
	asm.LDRBreg(jit.X16, jit.X14, jit.X15)
	asm.LDRBreg(jit.X17, jit.X5, jit.X15)
	asm.CMPreg(jit.X16, jit.X17)
	asm.BCond(jit.CondNE, smapNextLabel)
	asm.ADDimm(jit.X15, jit.X15, 1)
	asm.CMPreg(jit.X15, jit.X6)
	asm.BCond(jit.CondLT, smapByteLoopLabel)
	asm.Label(smapFoundLabel)
	asm.LDR(jit.X0, jit.X12, jit.StringLookupCacheEntryOffValue)
	valueHit(jit.X0)

	asm.Label(smapNextLabel)
	asm.ADDimm(jit.X13, jit.X13, 1)
	asm.CMPimm(jit.X13, runtime.StringLookupCacheProbeLimit)
	asm.BCond(jit.CondLT, smapLoopLabel)
	asm.B(slowLabel)
}

func emitBaselineDynamicStringGetTable(asm *jit.Assembler, a, pc int, feedbackEnabled bool, slowLabel, doneLabel string) {
	emitBaselineDynamicStringCacheProbe(asm, pc, slowLabel, func(fieldIdxReg jit.Reg) {
		asm.LDR(jit.X10, jit.X0, jit.TableOffSvals)
		asm.LDRreg(jit.X0, jit.X10, fieldIdxReg)
		if feedbackEnabled {
			emitBaselineTableStringKeyCacheHitFeedback(asm, pc, vm.TableAccessKindGet, jit.X0, "gettable_string")
			emitBaselineFeedbackResultFromValue(asm, pc, jit.X0, "gettable_string")
		}
		storeSlot(asm, a, jit.X0)
		asm.B(doneLabel)
	}, func(valueReg jit.Reg) {
		if feedbackEnabled {
			emitBaselineFeedbackResultFromValue(asm, pc, valueReg, "gettable_string_map")
		}
		storeSlot(asm, a, valueReg)
		asm.B(doneLabel)
	})
}

func emitBaselineDynamicStringSetTable(asm *jit.Assembler, cidx, pc int, feedbackEnabled bool, slowLabel, doneLabel string) {
	emitBaselineDynamicStringCacheProbe(asm, pc, slowLabel, func(fieldIdxReg jit.Reg) {
		loadRK(asm, jit.X4, cidx)
		asm.LoadImm64(jit.X12, nb64(jit.NB_ValNil))
		asm.CMPreg(jit.X4, jit.X12)
		asm.BCond(jit.CondEQ, slowLabel)
		if feedbackEnabled {
			emitBaselineTableStringKeyCacheHitFeedback(asm, pc, vm.TableAccessKindSet, jit.X4, "settable_string")
			emitBaselineFeedbackResultFromValue(asm, pc, jit.X4, "settable_string")
		}
		asm.LDR(jit.X10, jit.X0, jit.TableOffSvals)
		asm.STRreg(jit.X4, jit.X10, fieldIdxReg)
		asm.MOVimm16(jit.X5, 1)
		asm.STRB(jit.X5, jit.X0, jit.TableOffKeysDirty)
		asm.B(doneLabel)
	}, nil)
}
