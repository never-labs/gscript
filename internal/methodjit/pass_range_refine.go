// pass_range_refine.go holds branch-driven range refinement: narrowing the
// true/false successor environments from comparison and equality conditions.
// Pure code movement from pass_range.go.

package methodjit

import "math"

func refineBranchEnvs(condValue *Value, trueEnv, falseEnv map[int]intRange) {
	if condValue == nil || condValue.Def == nil || len(condValue.Def.Args) < 2 {
		return
	}
	cond := condValue.Def
	switch rangeRefineKind(cond.Op) {
	case OpRangeRefineLessThan:
		refineComparison(cond.Args[0], cond.Args[1], trueEnv, falseEnv, true)
	case OpRangeRefineLessEqual:
		refineComparison(cond.Args[0], cond.Args[1], trueEnv, falseEnv, false)
	case OpRangeRefineEqualInt:
		refineEqInt(cond.Args[0], cond.Args[1], trueEnv, falseEnv)
	}
}

func refineComparison(lhs, rhs *Value, trueEnv, falseEnv map[int]intRange, strict bool) {
	if c, ok := constIntFromValue(rhs); ok && lhs != nil {
		trueMax := c
		falseMin := c
		if strict {
			trueMax = satSub(c, 1)
		} else {
			falseMin = satAdd(c, 1)
		}
		constrainUpper(trueEnv, lhs.ID, trueMax)
		constrainLower(falseEnv, lhs.ID, falseMin)
		return
	}
	if c, ok := constIntFromValue(lhs); ok && rhs != nil {
		trueMin := c
		falseMax := c
		if strict {
			trueMin = satAdd(c, 1)
		} else {
			falseMax = satSub(c, 1)
		}
		constrainLower(trueEnv, rhs.ID, trueMin)
		constrainUpper(falseEnv, rhs.ID, falseMax)
	}
}

// refineEqInt handles OpEqInt conditions for range refinement.
// For `x == const`: true branch narrows to [const, const].
// For `x == const` false branch: if x was known >= 0, then x >= 1 (excluding 0);
// if x was known <= 0, then x <= -1 (excluding 0).
func refineEqInt(lhs, rhs *Value, trueEnv, falseEnv map[int]intRange) {
	c, ok := constIntFromValue(rhs)
	if !ok {
		return
	}
	if lhs == nil {
		return
	}
	// True branch: lhs == c
	constrainLower(trueEnv, lhs.ID, c)
	constrainUpper(trueEnv, lhs.ID, c)
	// False branch: lhs != c
	if c == 0 {
		refineNonZero(lhs.ID, falseEnv)
	}
}

// refineNonZero refines the range of id to exclude 0.
// If the existing range is known non-negative (min >= 0), set min to 1.
// If the existing range is known non-positive (max <= 0), set max to -1.
func refineNonZero(id int, env map[int]intRange) {
	r := env[id]
	if !r.known {
		return
	}
	if r.min >= 0 {
		constrainLower(env, id, 1)
	} else if r.max <= 0 {
		constrainUpper(env, id, -1)
	}
}

func constrainLower(env map[int]intRange, id int, min int64) {
	r := env[id]
	if !r.known {
		r = intRange{min: math.MinInt64, max: math.MaxInt64, known: true}
	}
	if min > r.min {
		r.min = min
	}
	env[id] = r
}

func constrainUpper(env map[int]intRange, id int, max int64) {
	r := env[id]
	if !r.known {
		r = intRange{min: math.MinInt64, max: math.MaxInt64, known: true}
	}
	if max < r.max {
		r.max = max
	}
	env[id] = r
}
