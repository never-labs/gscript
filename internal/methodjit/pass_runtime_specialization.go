//go:build darwin && arm64

package methodjit

import "github.com/gscript/gscript/internal/vm"

const callSiteRuntimeSpecializationMinStableObservations = 2

func CallSiteRuntimeSpecializationExitPass(globals map[string]*vm.FuncProto) PassFunc {
	return func(fn *Function) (*Function, error) {
		allowed := allowedDomainsForModule(
			analysisFacts(AnalysisFactCallABIs),
			analysisFacts(AnalysisFactCallSiteNoResultRuntimeSpecializations, AnalysisFactCallSiteNoResultRuntimeSpecializationBatches),
			nil,
		)
		return CallSiteRuntimeSpecializationExitPassCtx(globals)(newPassContext(fn, nil, allowed, false))
	}
}

func CallSiteRuntimeSpecializationExitPassCtx(globals map[string]*vm.FuncProto) CtxPassFunc {
	return func(ctx *PassContext) (*Function, error) {
		return annotateCallSiteRuntimeSpecializationExits(ctx.Func(), globals, ctx.Call()), nil
	}
}

func AnnotateCallSiteRuntimeSpecializationExits(fn *Function, globals map[string]*vm.FuncProto) *Function {
	if fn != nil {
		fn.ensureAnalysis()
	}
	allowed := allowedDomainsForModule(
		analysisFacts(AnalysisFactCallABIs),
		analysisFacts(AnalysisFactCallSiteNoResultRuntimeSpecializations, AnalysisFactCallSiteNoResultRuntimeSpecializationBatches),
		nil,
	)
	out, _ := CallSiteRuntimeSpecializationExitPassCtx(globals)(newPassContext(fn, nil, allowed, false))
	return out
}

func annotateCallSiteRuntimeSpecializationExits(fn *Function, globals map[string]*vm.FuncProto, callFacts *CallFacts) *Function {
	if fn == nil {
		return fn
	}
	if callFacts == nil {
		return fn
	}
	specializations := make(map[int]bool)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall || callResultCountFromAux2(instr.Aux2) != 0 {
				continue
			}
			nArgs := len(instr.Args) - 1
			if !vmCallSiteRuntimeSpecializationArity(nArgs) {
				continue
			}
			if !stableNoResultCallSiteCandidate(fn, instr, globals, nArgs) {
				continue
			}
			specializations[instr.ID] = true
		}
	}
	if len(specializations) == 0 {
		callFacts.SetCallSiteNoResultRuntimeSpecializations(nil)
		callFacts.SetCallSiteNoResultRuntimeSpecializationBatches(nil)
		return fn
	}
	callFacts.SetCallSiteNoResultRuntimeSpecializations(specializations)
	callFacts.SetCallSiteNoResultRuntimeSpecializationBatches(buildCallSiteNoResultRuntimeSpecializationBatches(fn, globals, specializations))
	return fn
}

func vmCallSiteRuntimeSpecializationArity(n int) bool {
	return n == 1 || n == 2 || n == 3 || n == 4
}

func stableNoResultCallSiteCandidate(fn *Function, instr *Instr, globals map[string]*vm.FuncProto, nArgs int) bool {
	if fn == nil || instr == nil {
		return false
	}
	if proto, ok := stableFeedbackCalleeProto(fn, instr, nArgs); ok {
		return protoHasNoResultCallSiteRuntimeSpecialization(proto) || protoReturnsNoValuesWithArity(proto, nArgs)
	}
	_, callee := resolveCallee(instr, fn, InlineConfig{Globals: globals})
	return protoHasNoResultCallSiteRuntimeSpecialization(callee) || protoReturnsNoValuesWithArity(callee, nArgs)
}

func stableFeedbackCalleeProto(fn *Function, instr *Instr, nArgs int) (*vm.FuncProto, bool) {
	if fn == nil || fn.Proto == nil || instr == nil || !instr.HasSource ||
		instr.SourcePC < 0 || instr.SourcePC >= len(fn.Proto.CallSiteFeedback) {
		return nil, false
	}
	fb := fn.Proto.CallSiteFeedback[instr.SourcePC]
	if fb.Count < callSiteRuntimeSpecializationMinStableObservations || fb.Flags&vm.CallSiteArityPolymorphic != 0 ||
		int(fb.NArgs) != nArgs || fb.ResultArity != 1 {
		return nil, false
	}
	return fb.StableCalleeVMProto()
}

func protoHasNoResultCallSiteRuntimeSpecialization(proto *vm.FuncProto) bool {
	for _, info := range vm.RecognizedCallSiteRuntimeSpecializations(proto) {
		if info.Route == vm.RuntimeSpecializationRouteCallSiteNoResult && info.Results == 0 {
			return true
		}
	}
	return false
}

func protoReturnsNoValuesWithArity(proto *vm.FuncProto, nArgs int) bool {
	if proto == nil || proto.IsVarArg || proto.NumParams != nArgs || len(proto.Code) == 0 {
		return false
	}
	seenReturn := false
	for _, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_RETURN {
			continue
		}
		seenReturn = true
		if vm.DecodeB(inst) != 1 {
			return false
		}
	}
	return seenReturn
}

func buildCallSiteNoResultRuntimeSpecializationBatches(fn *Function, globals map[string]*vm.FuncProto, specializations map[int]bool) map[int]CallSiteNoResultRuntimeSpecializationBatchFact {
	if fn == nil || fn.Proto == nil || len(specializations) == 0 {
		return nil
	}
	byPC := make(map[int]*Instr, len(specializations))
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr != nil && instr.Op == OpCall && instr.HasSource && specializations[instr.ID] {
				byPC[instr.SourcePC] = instr
			}
		}
	}
	out := make(map[int]CallSiteNoResultRuntimeSpecializationBatchFact)
	code := fn.Proto.Code
	for pc, inst := range code {
		if vm.DecodeOp(inst) != vm.OP_FORLOOP {
			continue
		}
		loopStart := pc + 1 + vm.DecodesBx(inst)
		if loopStart < 0 || loopStart >= pc {
			continue
		}
		loopBase := vm.DecodeA(inst)
		var calls []CallSiteNoResultRuntimeSpecializationBatchCall
		var last *Instr
		ok := true
		for callPC := loopStart; callPC < pc; callPC++ {
			callInst := code[callPC]
			if vm.DecodeOp(callInst) != vm.OP_CALL {
				continue
			}
			instr := byPC[callPC]
			if instr == nil {
				ok = false
				break
			}
			call, callOK := callSiteNoResultGlobalCallRecipe(fn.Proto, code, loopStart, callPC, globals)
			if !callOK {
				ok = false
				break
			}
			calls = append(calls, call)
			last = instr
		}
		if !ok || last == nil || len(calls) == 0 {
			continue
		}
		out[last.ID] = CallSiteNoResultRuntimeSpecializationBatchFact{
			LoopBase: loopBase,
			ExitPC:   pc + 1,
			Calls:    calls,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func callSiteNoResultGlobalCallRecipe(proto *vm.FuncProto, code []uint32, loopStart, callPC int, globals map[string]*vm.FuncProto) (CallSiteNoResultRuntimeSpecializationBatchCall, bool) {
	inst := code[callPC]
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)
	if b <= 0 || b > 4 {
		return CallSiteNoResultRuntimeSpecializationBatchCall{}, false
	}
	call := CallSiteNoResultRuntimeSpecializationBatchCall{FuncConst: -1, ArgConsts: make([]int, 0, b-1)}
	for slot := a; slot < a+b; slot++ {
		constIdx, ok := lastGlobalWriterBeforeCall(code, loopStart, callPC, slot)
		if !ok {
			return CallSiteNoResultRuntimeSpecializationBatchCall{}, false
		}
		if slot == a {
			name := protoConstString(proto, constIdx)
			callee := globals[name]
			if name == "" || (!protoHasNoResultCallSiteRuntimeSpecialization(callee) && !protoReturnsNoValuesWithArity(callee, b-1)) {
				return CallSiteNoResultRuntimeSpecializationBatchCall{}, false
			}
			call.FuncConst = constIdx
			continue
		}
		call.ArgConsts = append(call.ArgConsts, constIdx)
	}
	return call, call.FuncConst >= 0
}

func lastGlobalWriterBeforeCall(code []uint32, startPC, callPC, slot int) (int, bool) {
	for pc := callPC - 1; pc >= startPC; pc-- {
		inst := code[pc]
		a := vm.DecodeA(inst)
		if a != slot {
			continue
		}
		if vm.DecodeOp(inst) == vm.OP_GETGLOBAL {
			return vm.DecodeBx(inst), true
		}
		return -1, false
	}
	return -1, false
}
