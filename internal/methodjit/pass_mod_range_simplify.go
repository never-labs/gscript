package methodjit

var modRangeSimplifyPassAllowedDomains = allowedDomainsForModule(analysisFacts(AnalysisFactIntRanges), nil, nil, "ModRangeSimplify")

// ModRangeSimplifyPass removes integer modulo operations that range analysis
// proves are identity operations. It is deliberately conservative: it only
// rewrites positive constant divisors and non-negative dividends whose maximum
// is strictly below the divisor, preserving Lua modulo semantics for negatives.
func ModRangeSimplifyPass(fn *Function) (*Function, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	// Delegate through a non-enforcing PassContext so the single body lives in
	// the ctx form; direct callers keep the plain PassFunc signature.
	return ModRangeSimplifyPassCtx(newPassContext(fn, nil, modRangeSimplifyPassAllowedDomains, false))
}

// ModRangeSimplifyPassCtx is the domain-scoped form of ModRangeSimplifyPass. It
// reaches the IR via ctx.Func() and integer ranges via ctx.Numeric(); it
// touches no other fact domain.
func ModRangeSimplifyPassCtx(ctx *PassContext) (*Function, error) {
	fn := ctx.Func()
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	numeric := ctx.Numeric()
	if len(numeric.IntRangeMap()) == 0 {
		return fn, nil
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpModInt || len(instr.Args) < 2 || instr.Args[0] == nil || instr.Args[1] == nil {
				continue
			}
			divisor, ok := constIntFromValue(instr.Args[1])
			if !ok || divisor <= 0 {
				continue
			}
			if divisor == 1 {
				instr.Op = OpConstInt
				instr.Type = TypeInt
				instr.Args = nil
				instr.Aux = 0
				instr.Aux2 = 0
				functionRemarks(fn).Add("ModRangeSimplify", "changed", block.ID, instr.ID, instr.Op,
					"folded x % 1 to zero")
				continue
			}
			lhs := instr.Args[0]
			r, ok := numeric.IntRange(lhs.ID)
			if !ok || !r.known || r.min < 0 || r.max >= divisor {
				continue
			}
			replaceValueUses(fn, instr.ID, lhs, lhs.ID)
			instr.Op = OpNop
			instr.Type = TypeUnknown
			instr.Args = nil
			instr.Aux = 0
			instr.Aux2 = 0
			functionRemarks(fn).Add("ModRangeSimplify", "changed", block.ID, instr.ID, instr.Op,
				"replaced range-proven x % const with x")
		}
	}
	return fn, nil
}
