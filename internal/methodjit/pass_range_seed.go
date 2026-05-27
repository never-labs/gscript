// pass_range_seed.go holds Phase A loop-counter seeding: FORLOOP induction
// ranges, guarded forward-induction recognition, converging two-pointer
// induction safety, and the linear/square phi-expression analysis helpers they
// rely on. Pure code movement from pass_range.go.

package methodjit

import "math"

// --- Phase A: seed loop-counter ranges ---

// seedLoopRanges scans blocks for FORLOOP structure and seeds the induction
// phi's range based on the static loop bounds.
//
// Structure recognized:
//   - A loop header block has a Phi at the start.
//   - One of the phi's inputs is an add-like op with Aux2=1 (emitted by
//     FORLOOP back-edge), whose first arg is the phi itself.
//   - The block containing this back-edge add also contains a <= comparison
//     whose first arg is the back-edge add; its second arg is the limit.
//   - The other phi input is the loop-entry value: either a sub-like op
//     with Aux2=1 (emitted by FORPREP), or a direct ConstInt when ConstProp
//     has already folded the subtraction.
//
// When start/limit/step all resolve to concrete ints, we seed both the
// induction phi and the back-edge add with `[lo, hi]` expanded by |step|
// to cover the full trajectory including post-increment/exit values.
func seedLoopRanges(fn *Function, ranges map[int]intRange) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpPhi {
				break // phis live only at block entry
			}
			// Find the back-edge Add (Aux2=1, first arg = phi itself).
			var backAdd *Instr
			var initialArg *Value
			for _, arg := range instr.Args {
				if arg == nil || arg.Def == nil {
					continue
				}
				def := arg.Def
				if opIsBoxedOrFallback(def.Op, OpAdd) && def.Aux2 == 1 &&
					len(def.Args) >= 1 && def.Args[0] != nil && def.Args[0].ID == instr.ID {
					backAdd = def
					continue
				}
				initialArg = arg
			}
			if backAdd == nil || initialArg == nil {
				continue
			}

			// Resolve the step from the back-edge Add's Args[1].
			stepVal, stepOk := constIntFromValue(backAdd.Args[1])
			if !stepOk {
				continue
			}

			// Resolve the initial counter value from initialArg. Two forms:
			//   (a) OpSub/OpSubInt with Aux2=1: initialCounter = Args[0] - Args[1]
			//   (b) ConstInt (ConstProp folded the sub): initialCounter = Aux
			var initialCounter int64
			var initOk bool
			if initialArg.Def != nil {
				def := initialArg.Def
				if opIsBoxedOrFallback(def.Op, OpSub) {
					if def.Aux2 == 1 && len(def.Args) >= 2 {
						s, ok1 := constIntFromValue(def.Args[0])
						k, ok2 := constIntFromValue(def.Args[1])
						if ok1 && ok2 {
							initialCounter = s - k
							initOk = true
						}
					}
				} else if def.Op == OpConstInt {
					initialCounter = def.Aux
					initOk = true
				}
			}
			if !initOk {
				continue
			}

			// Find the limit: look for a <= comparison in backAdd's block whose
			// first arg is backAdd and whose second arg is a ConstInt.
			var limitVal int64
			var limitOk bool
			if backAdd.Block != nil {
				for _, bi := range backAdd.Block.Instrs {
					strict, ok := orderedRangeRefineKind(bi.Op)
					if !ok || strict {
						continue
					}
					if len(bi.Args) < 2 {
						continue
					}
					if bi.Args[0] == nil || bi.Args[0].ID != backAdd.ID {
						continue
					}
					if lv, lOk := constIntFromValue(bi.Args[1]); lOk {
						limitVal = lv
						limitOk = true
						break
					}
				}
			}
			if !limitOk {
				continue
			}

			// Bounding interval: [min(initialCounter, limit), max(...)].
			// Expand by |step| on both sides to cover post-increment extremes
			// and any guard slack.
			lo := initialCounter
			hi := limitVal
			if lo > hi {
				lo, hi = hi, lo
			}
			absStep := stepVal
			if absStep < 0 {
				absStep = -absStep
			}
			lo = satSub(lo, absStep)
			hi = satAdd(hi, absStep)

			seeded := intRange{min: lo, max: hi, known: true}
			ranges[instr.ID] = seeded
			ranges[backAdd.ID] = seeded
		}
	}
}

func constIntFromValue(v *Value) (int64, bool) {
	if v == nil || v.Def == nil {
		return 0, false
	}
	if v.Def.Op != OpConstInt {
		return 0, false
	}
	return v.Def.Aux, true
}

// --- Guarded forward induction ranges ---

// seedGuardedForwardInductionRanges recognizes while-style positive induction
// variables:
//
//	header:
//	  i = Phi(init, i + step)
//	  cond = f(i) <= bound
//	  Branch cond -> body, exit
//
// When init fits int48, step is a positive constant, bound is an int48 runtime
// value, and f(i) gives an upper bound for i on the true branch, every value
// carried back to the header is bounded by [init, true-branch-max(i)+step].
// This covers both while-style loops like `i = 5; while i*i <= n { ...; i += 6 }`
// and the graph builder's pre-increment for-loop shape where `for i := 0`
// becomes phi init -1 with the guarded value represented as phi+1.
func seedGuardedForwardInductionRanges(fn *Function, ranges map[int]intRange) {
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
			if phi.Op != OpPhi {
				break
			}
			if !phi.Type.isIntegerLike() {
				continue
			}
			ind, ok := analyzeForwardInduction(phi, li, ranges)
			if !ok || !ind.init.fitsInt48() {
				continue
			}
			trueMax, ok := guardedUpperBound(cond, phi, ranges)
			if !ok {
				continue
			}
			backMax := satAdd(trueMax, ind.step)
			seeded := intRange{
				min:   ind.init.min,
				max:   max64(ind.init.max, backMax),
				known: true,
			}
			if !seeded.fitsInt48() {
				continue
			}
			ranges[phi.ID] = seeded
			ranges[ind.update.ID] = seeded
		}
	}
}

type forwardInduction struct {
	init   intRange
	step   int64
	update *Instr
}

func analyzeForwardInduction(phi *Instr, li *loopInfo, rangeMaps ...map[int]intRange) (forwardInduction, bool) {
	var out forwardInduction
	var ranges map[int]intRange
	if len(rangeMaps) != 0 {
		ranges = rangeMaps[0]
	}
	bodyBlocks := li.headerBlocks[phi.Block.ID]
	if bodyBlocks == nil {
		return out, false
	}

	for predIdx, arg := range phi.Args {
		if arg == nil || arg.Def == nil {
			continue
		}
		var fromLoop bool
		if predIdx < len(phi.Block.Preds) {
			fromLoop = bodyBlocks[phi.Block.Preds[predIdx].ID]
		} else if arg.Def.Block != nil {
			fromLoop = bodyBlocks[arg.Def.Block.ID]
		}
		if fromLoop {
			step, ok := forwardStepFromPhi(arg.Def, phi.ID)
			if !ok {
				continue
			}
			if step <= 0 {
				return forwardInduction{}, false
			}
			if out.update != nil {
				if out.update == arg.Def && out.step == step {
					continue
				}
				return forwardInduction{}, false
			}
			out.step = step
			out.update = arg.Def
			continue
		}

		init := initialRangeFromValue(arg, ranges)
		if !init.known {
			return forwardInduction{}, false
		}
		if out.init.known {
			out.init = joinRange(out.init, init)
		} else {
			out.init = init
		}
	}

	if out.update == nil || !out.init.known {
		return forwardInduction{}, false
	}
	return out, true
}

func forwardStepFromPhi(instr *Instr, phiID int) (int64, bool) {
	if instr == nil {
		return 0, false
	}
	if lin, ok := linearExprOfPhi(instr.Value(), phiID); ok && lin.scale == 1 && lin.offset != 0 {
		return lin.offset, true
	}
	if opIsBoxedOrFallback(instr.Op, OpAdd) {
		if len(instr.Args) < 2 {
			return 0, false
		}
		if instr.Args[0] != nil && instr.Args[0].ID == phiID {
			if c, ok := constIntFromValue(instr.Args[1]); ok {
				return c, true
			}
		}
		if instr.Args[1] != nil && instr.Args[1].ID == phiID {
			if c, ok := constIntFromValue(instr.Args[0]); ok {
				return c, true
			}
		}
	} else if opIsBoxedOrFallback(instr.Op, OpSub) {
		if len(instr.Args) < 2 {
			return 0, false
		}
		if instr.Args[0] != nil && instr.Args[0].ID == phiID {
			if c, ok := constIntFromValue(instr.Args[1]); ok {
				return satNeg(c), true
			}
		}
	}
	return 0, false
}

func initialRangeFromValue(v *Value, ranges map[int]intRange) intRange {
	if v != nil && ranges != nil {
		if r, ok := ranges[v.ID]; ok && r.known {
			return r
		}
	}
	if c, ok := constIntFromValue(v); ok {
		return pointRange(c)
	}
	return topRange()
}

func loopHeaderBranchCond(header *Block) *Instr {
	if header == nil || len(header.Instrs) == 0 {
		return nil
	}
	term := header.Instrs[len(header.Instrs)-1]
	if term.Op != OpBranch || len(term.Args) == 0 || term.Args[0] == nil {
		return nil
	}
	return term.Args[0].Def
}

func guardedUpperBound(cond *Instr, phi *Instr, ranges map[int]intRange) (int64, bool) {
	if cond == nil || len(cond.Args) < 2 {
		return 0, false
	}
	strict, ok := orderedRangeRefineKind(cond.Op)
	if !ok {
		return 0, false
	}
	return compareUpperBound(cond.Args[0], cond.Args[1], phi, ranges, strict)
}

func compareUpperBound(lhs, rhs *Value, phi *Instr, ranges map[int]intRange, strict bool) (int64, bool) {
	bound, ok := valueIntUpperBound(rhs, ranges)
	if !ok {
		return 0, false
	}
	if strict {
		bound = satSub(bound, 1)
	}
	return deriveUpperBoundFromExpr(lhs, phi.ID, bound)
}

func valueIntUpperBound(v *Value, ranges map[int]intRange) (int64, bool) {
	if v == nil || v.Def == nil {
		return 0, false
	}
	if r, ok := ranges[v.ID]; ok && r.known {
		return r.max, true
	}
	if c, ok := constIntFromValue(v); ok {
		return c, true
	}
	if v.Def.Op == OpGuardIntRange {
		return v.Def.Aux2, true
	}
	if opIsBoxedOrFallback(v.Def.Op, OpAdd) {
		if len(v.Def.Args) < 2 {
			return 0, false
		}
		if c, ok := constIntFromValue(v.Def.Args[1]); ok {
			if upper, ok := valueIntUpperBound(v.Def.Args[0], ranges); ok {
				return satAdd(upper, c), true
			}
		}
		if c, ok := constIntFromValue(v.Def.Args[0]); ok {
			if upper, ok := valueIntUpperBound(v.Def.Args[1], ranges); ok {
				return satAdd(upper, c), true
			}
		}
	} else if opIsBoxedOrFallback(v.Def.Op, OpSub) {
		if len(v.Def.Args) < 2 {
			return 0, false
		}
		if c, ok := constIntFromValue(v.Def.Args[1]); ok {
			if upper, ok := valueIntUpperBound(v.Def.Args[0], ranges); ok {
				return satSub(upper, c), true
			}
		}
	}
	if isInt48RuntimeValue(v.Def) {
		return MaxInt48, true
	}
	return 0, false
}

func deriveUpperBoundFromExpr(v *Value, phiID int, bound int64) (int64, bool) {
	lin, ok := linearExprOfPhi(v, phiID)
	if ok && lin.scale > 0 {
		return floorDiv(satSub(bound, lin.offset), lin.scale), true
	}
	if square, ok := squareExprOfPhi(v, phiID); ok && square.scale > 0 {
		if bound < 0 {
			return 0, true
		}
		return floorDiv(satSub(isqrt64(bound), square.offset), square.scale), true
	}
	return 0, false
}

type phiLinearExpr struct {
	scale  int64
	offset int64
}

func linearExprOfPhi(v *Value, phiID int) (phiLinearExpr, bool) {
	if v == nil || v.Def == nil {
		return phiLinearExpr{}, false
	}
	if v.ID == phiID {
		return phiLinearExpr{scale: 1}, true
	}
	instr := v.Def
	if opIsBoxedOrFallback(instr.Op, OpAdd) {
		if len(instr.Args) < 2 {
			return phiLinearExpr{}, false
		}
		if lin, ok := linearExprOfPhi(instr.Args[0], phiID); ok {
			if c, ok := constIntFromValue(instr.Args[1]); ok {
				lin.offset = satAdd(lin.offset, c)
				return lin, true
			}
		}
		if lin, ok := linearExprOfPhi(instr.Args[1], phiID); ok {
			if c, ok := constIntFromValue(instr.Args[0]); ok {
				lin.offset = satAdd(lin.offset, c)
				return lin, true
			}
		}
	} else if opIsBoxedOrFallback(instr.Op, OpSub) {
		if len(instr.Args) < 2 {
			return phiLinearExpr{}, false
		}
		if lin, ok := linearExprOfPhi(instr.Args[0], phiID); ok {
			if c, ok := constIntFromValue(instr.Args[1]); ok {
				lin.offset = satSub(lin.offset, c)
				return lin, true
			}
		}
	}
	return phiLinearExpr{}, false
}

func squareExprOfPhi(v *Value, phiID int) (phiLinearExpr, bool) {
	if v == nil || v.Def == nil {
		return phiLinearExpr{}, false
	}
	instr := v.Def
	if !opIsBoxedOrFallback(instr.Op, OpMul) || len(instr.Args) < 2 {
		return phiLinearExpr{}, false
	}
	left, ok1 := linearExprOfPhi(instr.Args[0], phiID)
	right, ok2 := linearExprOfPhi(instr.Args[1], phiID)
	if !ok1 || !ok2 || left != right {
		return phiLinearExpr{}, false
	}
	return left, true
}

func isqrt64(v int64) int64 {
	if v <= 0 {
		return 0
	}
	x := int64(math.Sqrt(float64(v)))
	for x < math.MaxInt64 {
		next := x + 1
		if next > v/next {
			break
		}
		x++
	}
	for x > 0 && x > v/x {
		x--
	}
	return x
}

// markConvergingInductionSafe recognizes the common two-pointer loop:
//
//	header:
//	  lo = Phi(initLo, lo + 1)
//	  hi = Phi(initHi, hi - 1)
//	  lo < hi
//
// On the true branch, both operands are int48 values and the strict comparison
// proves lo <= MaxInt48-1 and hi >= MinInt48+1. Therefore lo+1 and hi-1 cannot
// leave the int48 payload range. This keeps swap/reverse loops in raw-int form
// without making a workload-specific assumption about arrays or table values.
func markConvergingInductionSafe(fn *Function, safe map[int]bool) {
	if fn == nil || safe == nil {
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
		if cond == nil || len(cond.Args) < 2 {
			continue
		}
		strict, ok := orderedRangeRefineKind(cond.Op)
		if !ok || !strict {
			continue
		}
		leftPhi := headerPhiValue(cond.Args[0], header)
		rightPhi := headerPhiValue(cond.Args[1], header)
		if leftPhi == nil || rightPhi == nil {
			continue
		}
		if !leftPhi.Type.isIntegerLike() || !rightPhi.Type.isIntegerLike() {
			continue
		}
		body := li.headerBlocks[header.ID]
		leftUpdate, ok := loopPhiBackedgeValue(leftPhi, body)
		if !ok || !isSelfAddConst(leftUpdate, leftPhi.ID, 1) {
			continue
		}
		rightUpdate, ok := loopPhiBackedgeValue(rightPhi, body)
		if !ok || !isSelfSubConst(rightUpdate, rightPhi.ID, 1) {
			continue
		}
		safe[leftUpdate.ID] = true
		safe[rightUpdate.ID] = true
	}
}

func headerPhiValue(v *Value, header *Block) *Instr {
	if v == nil || v.Def == nil || header == nil {
		return nil
	}
	if v.Def.Op != OpPhi || v.Def.Block != header {
		return nil
	}
	return v.Def
}

func loopPhiBackedgeValue(phi *Instr, body map[int]bool) (*Instr, bool) {
	if phi == nil || body == nil {
		return nil, false
	}
	var update *Instr
	for predIdx, arg := range phi.Args {
		if arg == nil || arg.Def == nil {
			continue
		}
		fromLoop := false
		if predIdx < len(phi.Block.Preds) {
			fromLoop = body[phi.Block.Preds[predIdx].ID]
		} else if arg.Def.Block != nil {
			fromLoop = body[arg.Def.Block.ID]
		}
		if !fromLoop {
			continue
		}
		if update != nil {
			return nil, false
		}
		update = arg.Def
	}
	return update, update != nil
}

func isSelfAddConst(instr *Instr, phiID int, c int64) bool {
	if instr == nil || instr.Op != OpAddInt || len(instr.Args) < 2 {
		return false
	}
	if instr.Args[0] != nil && instr.Args[0].ID == phiID {
		return valueIsConstInt(instr.Args[1], c)
	}
	if instr.Args[1] != nil && instr.Args[1].ID == phiID {
		return valueIsConstInt(instr.Args[0], c)
	}
	return false
}

func isSelfSubConst(instr *Instr, phiID int, c int64) bool {
	if instr == nil || instr.Op != OpSubInt || len(instr.Args) < 2 {
		return false
	}
	return instr.Args[0] != nil && instr.Args[0].ID == phiID && valueIsConstInt(instr.Args[1], c)
}

func valueIsConstInt(v *Value, want int64) bool {
	got, ok := constIntFromValue(v)
	return ok && got == want
}

func floorDiv(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	q := a / b
	r := a % b
	if r != 0 && ((r < 0) != (b < 0)) {
		q--
	}
	return q
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
