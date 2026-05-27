// pass_range.go implements forward dataflow range analysis for integer SSA
// values. It computes [min, max] ranges over the IR, then marks every
// AddInt/SubInt/MulInt/DivIntExact/NegInt whose range provably fits in the signed int48
// space. The emitter consults NumericFacts to skip the 3-instruction
// SBFX+CMP+B.NE overflow check on provably safe arithmetic.
//
// Motivation: loop counters are already exempted at graph-build time (Aux2=1),
// but operations derived from loop counters still carry overflow checks. A
// single overflow check after every arithmetic op is expensive in tight numeric
// kernels; eliminating provably-safe checks recovers most of that cost.
//
// Algorithm (three phases):
//   Phase A: Seed loop-counter ranges from FORLOOP structure and from
//     guarded while-style forward induction variables. When the loop guard
//     compares an induction-derived expression against an int bound, the
//     counter is capped by the guard plus one positive step.
//   Phase B: Forward propagation via fixed-point iteration (RPO, cap=5).
//     Constants seed their own range. AddInt/SubInt/MulInt/NegInt/ModInt
//     propagate using saturating arithmetic. Phi nodes join (min-of-mins,
//     max-of-maxes). All other ops are "top" (unbounded).
//   Phase C: Populate NumericFacts safety and sign facts from the final ranges.
//
// The range lattice and saturating primitives live in pass_range_lattice.go;
// forward range computation in pass_range_compute.go; modulo recurrence and
// IntMod facts in pass_range_modulo.go; non-negativity facts in
// pass_range_nonneg.go; branch-env refinement in pass_range_refine.go; and
// Phase A loop-counter seeding in pass_range_seed.go.

package methodjit

var rangeAnalysisPassAllowedDomains = allowedDomainsForModule(nil, rangeAnalysisFacts(), nil)

// RangeAnalysisPass computes integer ranges across the IR and marks every
// AddInt/SubInt/MulInt/DivIntExact/NegInt whose range provably fits in signed int48.
// It also records ModInt facts whose operands make the native ARM64 remainder
// sequence equivalent to Lua modulo semantics.
func RangeAnalysisPass(fn *Function) (*Function, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	// Delegate through a non-enforcing PassContext so the single body lives in
	// the ctx form; direct callers (tests, the post-IntExactDivision skip path)
	// keep the plain PassFunc signature.
	return RangeAnalysisPassCtx(newPassContext(fn, nil, rangeAnalysisPassAllowedDomains, false))
}

// RangeAnalysisPassCtx is the domain-scoped form of RangeAnalysisPass. It
// reaches the IR via ctx.Func() and integer ranges/safety facts via
// ctx.Numeric(); it touches no other fact domain.
func RangeAnalysisPassCtx(ctx *PassContext) (*Function, error) {
	fn := ctx.Func()
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	numeric := ctx.Numeric()

	intInstrs := rangeAnalysisIntInstrs(fn)
	profiledIntRanges := numeric.ProfiledIntRangeMap()
	profiledLenRanges := numeric.ProfiledLenRangeMap()
	ranges := make(map[int]intRange, len(intInstrs)+len(profiledIntRanges))
	for id, r := range profiledIntRanges {
		if r.known {
			ranges[id] = r
		}
	}
	staticLens := collectStaticLenRanges(fn)

	// Phase A: seed loop counter ranges from FORLOOP/while-loop structure.
	seedLoopRanges(fn, ranges)
	seedGuardedForwardInductionRanges(fn, ranges)
	seedModuloRecurrenceRanges(fn, ranges)

	// Phase B: fixed-point propagation. Nested loop-carried values can need
	// several trips through the block list before modulo-bounded recurrences
	// become visible at outer phis.
	const maxIter = 16
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for _, instr := range intInstrs {
			newR := computeRange(instr, ranges, staticLens, profiledIntRanges, profiledLenRanges)
			if old, ok := ranges[instr.ID]; ok {
				if !rangeEqual(old, newR) {
					ranges[instr.ID] = newR
					changed = true
				}
			} else {
				ranges[instr.ID] = newR
				if newR.known {
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}

	// Phase C: populate Int48Safe for int-arithmetic ops whose range fits.
	safe := make(map[int]bool, len(intInstrs))
	for _, instr := range intInstrs {
		switch instr.Op {
		case OpAddInt, OpSubInt, OpMulInt, OpDivIntExact, OpNegInt:
			if r, ok := ranges[instr.ID]; ok && r.fitsInt48() {
				safe[instr.ID] = true
			}
		}
	}
	markConvergingInductionSafe(fn, safe)
	markGuardedForwardInductionUpdatesSafe(fn, ranges, safe)
	nonNegative := collectIntNonNegativeFacts(intInstrs, ranges)
	for id := range numeric.IntNonNegativeMap() {
		nonNegative[id] = true
	}
	numeric.SetComputedRanges(safe, ranges, nonNegative)
	populateIntModFacts(fn, ranges)
	return fn, nil
}

func markGuardedForwardInductionUpdatesSafe(fn *Function, ranges map[int]intRange, safe map[int]bool) {
	if fn == nil || ranges == nil || safe == nil {
		return
	}
	li := computeLoopInfo(fn)
	if !li.hasLoops() {
		return
	}
	for _, header := range fn.Blocks {
		if !li.loopHeaders[header.ID] {
			continue
		}
		cond := loopHeaderBranchCond(header)
		if cond == nil {
			continue
		}
		for _, phi := range header.Instrs {
			if phi == nil || phi.Op != OpPhi {
				break
			}
			if !phi.Type.isIntegerLike() {
				continue
			}
			ind, ok := analyzeForwardInduction(phi, li, ranges)
			if !ok || ind.update == nil || !ind.init.fitsInt48() {
				continue
			}
			trueMax, ok := guardedUpperBound(cond, phi, ranges)
			if !ok {
				continue
			}
			updateRange := intRange{min: satAdd(ind.init.min, ind.step), max: satAdd(trueMax, ind.step), known: true}
			if existing, ok := ranges[ind.update.ID]; !ok || !existing.known {
				ranges[ind.update.ID] = updateRange
			}
			if updateRange.fitsInt48() {
				safe[ind.update.ID] = true
			}
		}
	}
}

func rangeAnalysisIntInstrs(fn *Function) []*Instr {
	if fn == nil {
		return nil
	}
	out := make([]*Instr, 0)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr != nil && instr.Type.isIntegerLike() {
				out = append(out, instr)
			}
		}
	}
	return out
}

// isIntegerLike returns true for IR types whose runtime value is a raw int
// that the range analysis can track. We restrict to TypeInt only — other
// types carry NaN-boxed values that range analysis doesn't model.
func (t Type) isIntegerLike() bool {
	return t == TypeInt
}
