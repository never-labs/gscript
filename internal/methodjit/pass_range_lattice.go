// pass_range_lattice.go holds the integer-range lattice type and its
// saturating-arithmetic primitives used throughout range analysis. Pure code
// movement from pass_range.go; no behavior change.

package methodjit

import "math"

// Signed int48 limits. Any int64 value within [MinInt48, MaxInt48] is
// guaranteed to round-trip through SBFX(x, 0, 48).
const (
	MinInt48 int64 = -(1 << 47)
	MaxInt48 int64 = (1 << 47) - 1
)

// intRange represents a closed interval [min, max]. When known=false the
// range is "top" (unbounded) and the value's final range is MinInt64/MaxInt64
// conceptually. We still fill min/max in that case as sentinels so callers
// that forget to check `known` read safe values.
type intRange struct {
	min, max int64
	known    bool
}

func topRange() intRange {
	return intRange{min: math.MinInt64, max: math.MaxInt64, known: false}
}

func pointRange(v int64) intRange {
	return intRange{min: v, max: v, known: true}
}

func (r intRange) fitsInt48() bool {
	return r.known && r.min >= MinInt48 && r.max <= MaxInt48
}

func (r intRange) nonNegative() bool {
	return r.known && r.min >= 0
}

// rangeEqual reports whether two ranges are exactly the same (including
// known-status), used to detect phi convergence.
func rangeEqual(a, b intRange) bool {
	if a.known != b.known {
		return false
	}
	if !a.known {
		return true
	}
	return a.min == b.min && a.max == b.max
}

// join returns the union of two ranges. If either is top the result is top.
func joinRange(a, b intRange) intRange {
	if !a.known || !b.known {
		return topRange()
	}
	out := intRange{known: true, min: a.min, max: a.max}
	if b.min < out.min {
		out.min = b.min
	}
	if b.max > out.max {
		out.max = b.max
	}
	return out
}

func intersectRange(a, b intRange) intRange {
	if !a.known || !b.known {
		return topRange()
	}
	out := intRange{known: true, min: a.min, max: a.max}
	if b.min > out.min {
		out.min = b.min
	}
	if b.max < out.max {
		out.max = b.max
	}
	if out.min > out.max {
		return topRange()
	}
	return out
}

// --- Saturating arithmetic helpers ---

func satAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

func satSub(a, b int64) int64 {
	// a - b: reduce to satAdd(a, -b), careful with b=MinInt64.
	if b == math.MinInt64 {
		// -(MinInt64) overflows. If a >= 0, result saturates to MaxInt64;
		// otherwise a - MinInt64 = a + |MinInt64|, which overflows iff a < 0,
		// but we're in the a < 0 branch only if a == MinInt64 (impossible since
		// a >= 0 was handled). Conservatively saturate.
		if a >= 0 {
			return math.MaxInt64
		}
		return math.MaxInt64
	}
	return satAdd(a, -b)
}

func satMul(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	// Detect overflow via division. Guard MinInt64 / -1 case.
	if a == math.MinInt64 || b == math.MinInt64 {
		// Multiplying MinInt64 by anything != 0, ±1 overflows; by ±1 is ±MinInt64
		// which still overflows for -1. Just saturate by sign.
		if (a < 0) == (b < 0) {
			return math.MaxInt64
		}
		return math.MinInt64
	}
	result := a * b
	// Overflow iff sign of result disagrees with expected, or division doesn't
	// recover. Using division is robust for any non-MinInt64 operands.
	if result/b != a {
		if (a < 0) == (b < 0) {
			return math.MaxInt64
		}
		return math.MinInt64
	}
	return result
}

func satNeg(a int64) int64 {
	if a == math.MinInt64 {
		return math.MaxInt64
	}
	return -a
}

// --- Range arithmetic ---

func addRange(a, b intRange) intRange {
	if !a.known || !b.known {
		return topRange()
	}
	lo := satAdd(a.min, b.min)
	hi := satAdd(a.max, b.max)
	if lo == math.MinInt64 || hi == math.MaxInt64 {
		// Saturation hit — treat as top to be safe (we don't want to claim
		// a false-narrow range that happens to fit int48).
		if lo == math.MinInt64 && a.min+b.min != math.MinInt64 {
			return topRange()
		}
		if hi == math.MaxInt64 && a.max+b.max != math.MaxInt64 {
			return topRange()
		}
	}
	return intRange{min: lo, max: hi, known: true}
}

func subRange(a, b intRange) intRange {
	if !a.known || !b.known {
		return topRange()
	}
	lo := satSub(a.min, b.max)
	hi := satSub(a.max, b.min)
	return intRange{min: lo, max: hi, known: true}
}

func mulRange(a, b intRange) intRange {
	if !a.known || !b.known {
		return topRange()
	}
	p1 := satMul(a.min, b.min)
	p2 := satMul(a.min, b.max)
	p3 := satMul(a.max, b.min)
	p4 := satMul(a.max, b.max)
	lo := p1
	hi := p1
	for _, p := range []int64{p2, p3, p4} {
		if p < lo {
			lo = p
		}
		if p > hi {
			hi = p
		}
	}
	return intRange{min: lo, max: hi, known: true}
}

func negRange(a intRange) intRange {
	if !a.known {
		return topRange()
	}
	return intRange{min: satNeg(a.max), max: satNeg(a.min), known: true}
}

func modRange(a, b intRange) intRange {
	// Lua modulo has the divisor's sign. Positive divisors therefore produce a
	// non-negative result even when the dividend is negative. This range fact is
	// safe for downstream arithmetic; the emitter still uses IntModNoSignAdjust
	// to decide whether the native modulo operation may skip the sign-adjust
	// path for the original dividend.
	if !b.known {
		return topRange()
	}
	if b.min > 0 {
		return intRange{min: 0, max: satSub(b.max, 1), known: true}
	}
	if b.max < 0 {
		return intRange{min: satAdd(b.min, 1), max: 0, known: true}
	}
	if a.known && a.min >= 0 && b.max > 0 {
		return intRange{min: 0, max: satSub(b.max, 1), known: true}
	}
	if a.known && a.max <= 0 && b.min < 0 {
		return intRange{min: satAdd(b.min, 1), max: 0, known: true}
	}
	bound := int64(0)
	absMin := b.min
	if absMin < 0 {
		absMin = satNeg(absMin)
	}
	absMax := b.max
	if absMax < 0 {
		absMax = satNeg(absMax)
	}
	if absMin > bound {
		bound = absMin
	}
	if absMax > bound {
		bound = absMax
	}
	if bound == 0 {
		return topRange() // divisor is exactly zero, runtime error anyway
	}
	bound--
	return intRange{min: -bound, max: bound, known: true}
}

func divExactRange(a, b intRange) intRange {
	if !a.known || !b.known {
		return topRange()
	}
	if b.min <= 0 && b.max >= 0 {
		return topRange()
	}
	qs := []int64{
		safeDivBound(a.min, b.min),
		safeDivBound(a.min, b.max),
		safeDivBound(a.max, b.min),
		safeDivBound(a.max, b.max),
	}
	lo, hi := qs[0], qs[0]
	for _, q := range qs[1:] {
		if q < lo {
			lo = q
		}
		if q > hi {
			hi = q
		}
	}
	return intRange{min: lo, max: hi, known: true}
}

func safeDivBound(a, b int64) int64 {
	if b == 0 {
		return math.MaxInt64
	}
	if a == math.MinInt64 && b == -1 {
		return math.MaxInt64
	}
	return a / b
}

func rangeWithin(r, bounds intRange) bool {
	return r.known && bounds.known && r.min >= bounds.min && r.max <= bounds.max
}

func rangeExcludesZero(r intRange) bool {
	return r.known && (r.max < 0 || r.min > 0)
}

func rangesHaveSameKnownModuloSign(lhs, rhs intRange) bool {
	if !lhs.known || !rhs.known {
		return false
	}
	return (lhs.min >= 0 && rhs.min > 0) || (lhs.max <= 0 && rhs.max < 0)
}
