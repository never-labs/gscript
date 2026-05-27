package methodjit

// IntAlgebraSimplifyPass removes checked integer add/sub pairs that cancel
// each other after RangeAnalysis has proven both checked operations int48-safe.
func IntAlgebraSimplifyPass(fn *Function) (*Function, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	// Delegate through a non-enforcing PassContext so the single body lives in
	// the ctx form; direct callers keep the plain PassFunc signature.
	allowed := allowedDomainsForModule(analysisFacts(AnalysisFactIntRanges), nil, nil, "IntAlgebraSimplify")
	return IntAlgebraSimplifyPassCtx(newPassContext(fn, nil, allowed, false))
}

// IntAlgebraSimplifyPassCtx is the domain-scoped form of IntAlgebraSimplifyPass.
// It reaches the IR via ctx.Func() and int48-safety facts via ctx.Numeric(); it
// touches no other fact domain.
func IntAlgebraSimplifyPassCtx(ctx *PassContext) (*Function, error) {
	fn := ctx.Func()
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	numeric := ctx.Numeric()
	if numeric.Int48SafeCount() == 0 {
		return fn, nil
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Type != TypeInt || len(instr.Args) < 2 || !numeric.IsInt48Safe(instr.ID) {
				continue
			}
			base, ok := cancellingIntAddSubBase(instr, numeric)
			if !ok {
				continue
			}
			replaceValueUses(fn, instr.ID, base, instr.ID)
			instr.Op = OpNop
			instr.Type = TypeUnknown
			instr.Args = nil
			instr.Aux = 0
			instr.Aux2 = 0
			functionRemarks(fn).Add("IntAlgebraSimplify", "changed", block.ID, instr.ID, instr.Op,
				"removed cancelling checked integer add/sub pair")
		}
	}
	return fn, nil
}

func cancellingIntAddSubBase(instr *Instr, numeric *NumericFacts) (*Value, bool) {
	if instr == nil || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[0].Def == nil || instr.Args[1] == nil {
		return nil, false
	}
	outerConst, ok := constIntFromValue(instr.Args[1])
	if !ok {
		return nil, false
	}
	inner := instr.Args[0].Def
	if inner == nil || len(inner.Args) < 2 || inner.Args[0] == nil || inner.Args[1] == nil || !numeric.IsInt48Safe(inner.ID) {
		return nil, false
	}
	innerConst, ok := constIntFromValue(inner.Args[1])
	if !ok || innerConst != outerConst {
		return nil, false
	}
	switch instr.Op {
	case OpSubInt:
		if inner.Op == OpAddInt {
			return inner.Args[0], true
		}
	case OpAddInt:
		if inner.Op == OpSubInt {
			return inner.Args[0], true
		}
	}
	return nil, false
}
