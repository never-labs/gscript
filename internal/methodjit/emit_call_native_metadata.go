//go:build darwin && arm64

// emit_call_native_metadata.go: call cache, callee resolution, liveness, predicates.
//
// Pure code-movement split of emit_call_native.go (zero behavior change).

package methodjit

import (
	"fmt"
	"sort"
	"strings"
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
	"github.com/never-labs/leia/internal/vm"
)

const (
	tier2CallCacheWays        = 4
	tier2CallCacheWordsPerWay = 4
	tier2CallCacheWayBytes    = tier2CallCacheWordsPerWay * 8
	tier2CallCacheStrideWords = tier2CallCacheWays * tier2CallCacheWordsPerWay
	tier2CallCacheStrideBytes = tier2CallCacheStrideWords * 8
)

type callCalleeFlagSpec struct {
	protos        []*vm.FuncProto
	knownLeaf     bool
	knownNoGlobal bool
}

func (ec *emitContext) staticNoDepthCallee(instr *Instr) *vm.FuncProto {
	if ec == nil || instr == nil || ec.fn == nil {
		return nil
	}
	if ec.tailCallInstrs[instr.ID] || ec.isStaticSelfCall(instr) {
		return nil
	}
	_, callee := resolveCallee(instr, ec.fn, InlineConfig{Globals: functionGlobalFacts(ec.fn).GlobalsMap()})
	if callee == nil {
		if feedbackCallee, ok := callABIFeedbackCalleeProto(ec.fn, instr); ok {
			callee = feedbackCallee
		}
	}
	if !rawIntPeerLeafCallee(callee) {
		return nil
	}
	return callee
}

func (ec *emitContext) staticNativeCallUnsafeCallee(instr *Instr) *vm.FuncProto {
	if ec == nil || instr == nil || ec.fn == nil {
		return nil
	}
	if ec.tailCallInstrs[instr.ID] || ec.isStaticSelfCall(instr) {
		return nil
	}
	_, callee := resolveCallee(instr, ec.fn, InlineConfig{Globals: functionGlobalFacts(ec.fn).GlobalsMap()})
	if callee == nil {
		if feedbackCallee, ok := callABIFeedbackCalleeProto(ec.fn, instr); ok {
			callee = feedbackCallee
		}
	}
	if callee == nil || !protoHasNativeCallUnsafeTableBytecode(callee) {
		return nil
	}
	if callee.Tier2Promoted && callee.Tier2DirectEntryPtr != 0 {
		return nil
	}
	return callee
}

func (ec *emitContext) callCalleeFlagSpec(instr *Instr) callCalleeFlagSpec {
	protos := ec.callCalleeFeedbackProtos(instr)
	if len(protos) == 0 {
		return callCalleeFlagSpec{}
	}
	allLeaf := true
	allNoGlobal := true
	for _, proto := range protos {
		if proto == nil {
			return callCalleeFlagSpec{}
		}
		if !proto.LeafNoCall {
			allLeaf = false
		}
		if !proto.NoGlobalOps {
			allNoGlobal = false
		}
	}
	if !allLeaf && !allNoGlobal {
		return callCalleeFlagSpec{}
	}
	return callCalleeFlagSpec{
		protos:        protos,
		knownLeaf:     allLeaf,
		knownNoGlobal: allNoGlobal,
	}
}

func (ec *emitContext) callCalleeFeedbackProtos(instr *Instr) []*vm.FuncProto {
	if protos := ec.callCalleeFieldShapeProtos(instr); len(protos) > 0 {
		return protos
	}
	if ec == nil || ec.fn == nil || ec.fn.Proto == nil || instr == nil || instr.Op != OpCall ||
		!instr.HasSource || instr.SourcePC < 0 || instr.SourcePC >= len(ec.fn.Proto.CallSiteFeedback) {
		return nil
	}
	fb := ec.fn.Proto.CallSiteFeedback[instr.SourcePC]
	if fb.Count < callSiteRuntimeSpecializationMinStableObservations ||
		fb.Flags&vm.CallSiteArityPolymorphic != 0 ||
		int(fb.NArgs) != len(instr.Args)-1 ||
		fb.ResultArity != uint8(instr.Aux2) {
		return nil
	}
	if fb.Flags&vm.CallSiteCalleePolymorphic == 0 {
		if callee, ok := fb.StableCalleeVMProto(); ok && callee != nil {
			return []*vm.FuncProto{callee}
		}
		return nil
	}
	return fb.MaturePolymorphicVMProtos(callSiteRuntimeSpecializationMinStableObservations, len(instr.Args)-1, uint8(instr.Aux2))
}

func (ec *emitContext) callCalleeFieldShapeProtos(instr *Instr) []*vm.FuncProto {
	if ec == nil {
		return nil
	}
	return fieldShapeCalleeProtos(ec.fn, instr)
}

func (ec *emitContext) emitGuardCalleeProtoSet(protos []*vm.FuncProto, slowLabel string) {
	if ec == nil || len(protos) == 0 {
		return
	}
	asm := ec.asm
	okLabel := ec.uniqueLabel("t2call_feedback_proto_ok")
	for _, proto := range protos {
		if proto == nil {
			continue
		}
		asm.LoadImm64(jit.X3, int64(uintptr(unsafe.Pointer(proto))))
		asm.CMPreg(jit.X1, jit.X3)
		asm.BCond(jit.CondEQ, okLabel)
	}
	asm.B(slowLabel)
	asm.Label(okLabel)
}

func (ec *emitContext) recordCallCachePC(cacheIndex, pc int) {
	if ec == nil || cacheIndex < 0 {
		return
	}
	for len(ec.callCachePCs) <= cacheIndex {
		ec.callCachePCs = append(ec.callCachePCs, -1)
	}
	ec.callCachePCs[cacheIndex] = pc
}

func (ec *emitContext) formatLiveCallRegs(live map[int]bool) string {
	if len(live) == 0 {
		return "[]"
	}
	ids := make([]int, 0, len(live))
	for id := range live {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if pr, ok := ec.alloc.ValueRegs[id]; ok {
			if pr.IsFloat {
				parts = append(parts, fmt.Sprintf("v%d:D%d", id, pr.Reg))
			} else {
				parts = append(parts, fmt.Sprintf("v%d:X%d", id, pr.Reg))
			}
			continue
		}
		parts = append(parts, fmt.Sprintf("v%d", id))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (ec *emitContext) fnUsesConstPool() bool {
	if ec == nil || ec.fn == nil {
		return true
	}
	for _, block := range ec.fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instrUsesConstPool(instr) {
				return true
			}
		}
	}
	return false
}

// isNumericStaticSelfCall (R124) returns true when this OpCall can use
// the numeric self-call fast path: static-self (R110), proto qualifies
// for numeric (R121), all args are int-typed.
func (ec *emitContext) isNumericStaticSelfCall(instr *Instr) bool {
	if !ec.isStaticSelfCall(instr) {
		return false
	}
	abi := ec.rawIntSelfABI
	if !abi.Eligible {
		abi = AnalyzeRawIntSelfABI(ec.fn.Proto)
	}
	if !abi.Eligible {
		return false
	}
	numParams := abi.NumParams
	if len(instr.Args) != 1+numParams {
		return false
	}
	for i := 0; i < numParams; i++ {
		argID := instr.Args[1+i].ID
		if ec.hasReg(argID) && ec.valueReprOf(argID) == valueReprRawInt {
			continue
		}
		if ec.irTypes[argID] == TypeInt {
			continue
		}
		return false
	}
	return true
}

func (ec *emitContext) isTypedStaticSelfCall(instr *Instr) bool {
	if ec.numericMode || !ec.isStaticSelfCall(instr) {
		return false
	}
	abi := ec.typedSelfABI
	if !abi.Eligible {
		return false
	}
	if len(instr.Args) != 1+abi.NumParams || len(abi.Params) != abi.NumParams {
		return false
	}
	if ec.tailCallInstrs[instr.ID] {
		return false
	}
	for i, rep := range abi.Params {
		argID := instr.Args[1+i].ID
		switch rep {
		case SpecializedABIParamRawInt:
			if ec.hasReg(argID) && ec.valueReprOf(argID) == valueReprRawInt {
				continue
			}
			if ec.irTypes[argID] == TypeInt {
				continue
			}
			return false
		case SpecializedABIParamRawTablePtr:
			if ec.irTypes[argID] == TypeTable {
				continue
			}
			if (ec.irTypes[argID] == TypeAny || ec.irTypes[argID] == TypeUnknown) &&
				typedSelfCallArgSlotMatches(ec.fn.Proto, instr.SourcePC, i, SpecializedABIParamRawTablePtr) {
				continue
			}
			if ec.irTypes[argID] != TypeTable {
				return false
			}
		default:
			return false
		}
	}
	switch abi.Return {
	case SpecializedABIReturnNone:
		return callResultCountFromAux2(instr.Aux2) == 0
	case SpecializedABIReturnRawInt:
		return instr.Type == TypeInt
	case SpecializedABIReturnRawFloat:
		return instr.Type == TypeFloat
	case SpecializedABIReturnRawTablePtr:
		return instr.Type == TypeTable
	default:
		return false
	}
}

// qualifyForNumeric reports whether a proto is eligible for the raw-int
// self-recursive ABI. The predicate delegates to AnalyzeSpecializedABI so the
// compiler, tests, and future metadata all use the same structural contract.
// Returns (ok, numParams). When ok is true, numParams is in [1, 4].
func qualifyForNumeric(proto *vm.FuncProto) (bool, int) {
	abi := AnalyzeRawIntSelfABI(proto)
	if !abi.Eligible {
		return false, 0
	}
	return true, abi.NumParams
}

// isStaticSelfCall (R110) returns true when OpCall's function argument is
// an OpGetGlobal whose resolved constant-pool name matches the current
// function's proto name. In that case the target is (statically) our
// own proto, so the runtime Proto compare can be elided and we can BL
// t2_self_entry directly.
func (ec *emitContext) isStaticSelfCall(instr *Instr) bool {
	if ec.fn == nil || ec.fn.Proto == nil || instr == nil {
		return false
	}
	if len(instr.Args) == 0 || instr.Args[0] == nil || instr.Args[0].Def == nil {
		return false
	}
	def := instr.Args[0].Def
	if def.Op != OpGetGlobal {
		return false
	}
	globalIdx := int(def.Aux)
	constants := ec.fn.Proto.Constants
	if globalIdx < 0 || globalIdx >= len(constants) {
		return false
	}
	kv := constants[globalIdx]
	if !kv.IsString() {
		return false
	}
	return kv.Str() == ec.fn.Proto.Name
}

// computeLiveAcrossCall returns the set of GPR and FPR value IDs that are live
// across a CALL instruction. A value is live across the call if:
//  1. It's currently active in a register, AND
//  2. It's used by any instruction AFTER the call in the same block, OR
//  3. It's used by a phi in a successor block (cross-block live).
//
// Typically only a few registers are live across a recursive call. This lets
// selective spill emit a small number of STR instructions instead of spilling
// the full allocatable GPR/FPR pools.
func (ec *emitContext) computeLiveAcrossCall(callInstr *Instr) (gprLive map[int]bool, fprLive map[int]bool) {
	gprLive = make(map[int]bool)
	fprLive = make(map[int]bool)

	// Collect all value IDs used after the call in the same block.
	usedAfter := make(map[int]bool)
	block := callInstr.Block
	if block != nil {
		found := false
		for _, instr := range block.Instrs {
			if instr == callInstr {
				found = true
				continue
			}
			if !found {
				continue
			}
			for _, arg := range instr.Args {
				if arg != nil {
					usedAfter[arg.ID] = true
				}
			}
		}
	}

	liveOut := map[int]bool(nil)
	if callInstr.Block != nil {
		liveOut = ec.blockLiveOut[callInstr.Block.ID]
	}

	// Check GPRs: is the active value used after the call or live out of
	// this block? blockLiveOut is point-bounded; crossBlockLive is too broad
	// for values carried into this block and already consumed before the call.
	for valueID := range ec.activeRegs {
		if usedAfter[valueID] || liveOut[valueID] {
			gprLive[valueID] = true
		}
	}

	// Check FPRs: same criterion.
	for valueID := range ec.activeFPRegs {
		if usedAfter[valueID] || liveOut[valueID] {
			fprLive[valueID] = true
		}
	}

	return gprLive, fprLive
}

// emitSpillSelectiveForCall writes only the specified live register-resident
// values to their memory home slots. Called before a native BLR to save only
// registers that are actually needed after the call returns.
func (ec *emitContext) emitSpillSelectiveForCall(gprLive, fprLive map[int]bool) {
	for valueID := range gprLive {
		pr, ok := ec.alloc.ValueRegs[valueID]
		if !ok || pr.IsFloat {
			continue
		}
		slot, hasSlot := ec.slotMap[valueID]
		if !hasSlot {
			continue
		}
		reg := jit.Reg(pr.Reg)
		ec.emitStoreGPRValueAsBoxed(valueID, reg, slot)
	}

	for valueID := range fprLive {
		pr, ok := ec.alloc.ValueRegs[valueID]
		if !ok || !pr.IsFloat {
			continue
		}
		slot, hasSlot := ec.slotMap[valueID]
		if !hasSlot {
			continue
		}
		fpr := jit.FReg(pr.Reg)
		ec.asm.FMOVtoGP(jit.X0, fpr)
		ec.asm.STR(jit.X0, mRegRegs, slotOffset(slot))
		ec.emitExitResumeCheckShadowStoreGPR(slot, jit.X0)
	}
}

// emitReloadSelectiveForCall reloads only the specified live register-resident
// values from their memory home slots. Called after a native BLR to restore
// only registers that are needed after the call.
func (ec *emitContext) emitReloadSelectiveForCall(gprLive, fprLive map[int]bool) {
	for valueID := range gprLive {
		pr, ok := ec.alloc.ValueRegs[valueID]
		if !ok || pr.IsFloat {
			continue
		}
		slot, hasSlot := ec.slotMap[valueID]
		if !hasSlot {
			continue
		}
		reg := jit.Reg(pr.Reg)
		ec.emitReloadGPRValueFromBoxed(valueID, reg, slot)
	}

	for valueID := range fprLive {
		pr, ok := ec.alloc.ValueRegs[valueID]
		if !ok || !pr.IsFloat {
			continue
		}
		slot, hasSlot := ec.slotMap[valueID]
		if !hasSlot {
			continue
		}
		fpr := jit.FReg(pr.Reg)
		ec.asm.FLDRd(fpr, mRegRegs, slotOffset(slot))
	}
}

func cloneBoolMap(src map[int]bool) map[int]bool {
	dst := make(map[int]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
