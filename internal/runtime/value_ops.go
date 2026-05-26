package runtime

import (
	"math"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// In-place mutation (hot-loop optimization)
// ---------------------------------------------------------------------------

// SetInt updates a Value to an integer in place.
func (v *Value) SetInt(i int64) {
	if i > maxInt48 || i < minInt48 {
		*v = FloatValue(float64(i))
	} else {
		*v = Value(tagInt | (uint64(i) & payloadMask))
	}
}

// SetIntUnchecked updates a Value to an integer without range checking.
// Only safe when the caller guarantees |i| < 2^47 (e.g., FORLOOP counters).
func (v *Value) SetIntUnchecked(i int64) {
	*v = Value(tagInt | (uint64(i) & payloadMask))
}

// ---------------------------------------------------------------------------
// Pointer-receiver fast paths (avoid copies in VM hot loop)
// ---------------------------------------------------------------------------

func (v *Value) RawType() ValueType { return v.Type() }

func (v *Value) RawInt() int64 {
	// Branchless sign-extend 48-bit integer to 64-bit.
	// Arithmetic shift: (raw << 16) >> 16 sign-extends bit 47.
	return int64(uint64(*v)<<16) >> 16
}

func (v *Value) RawFloat() float64 { return math.Float64frombits(uint64(*v)) }

func (v *Value) RawString() (string, bool) {
	if uint64(*v)&tagMask != tagPtr {
		return "", false
	}
	switch uint64(*v) & ptrSubMask {
	case ptrSubString:
		p := v.ptrPayload()
		if p == nil {
			return "", true
		}
		return *(*string)(p), true
	case ptrSubLazyString:
		if lz := v.lazyString(); lz != nil {
			return lz.materialize(), true
		}
		return "", true
	default:
		return "", false
	}
}

func AddInts(dst, a, b *Value) bool {
	if a.IsInt() && b.IsInt() {
		dst.SetInt(a.RawInt() + b.RawInt())
		return true
	}
	return false
}

// AddNums tries to add *a + *b as numbers (int or float), storing result in *dst.
func AddNums(dst, a, b *Value) bool {
	if a.IsInt() && b.IsInt() {
		dst.SetInt(a.RawInt() + b.RawInt())
		return true
	}
	if a.IsNumber() && b.IsNumber() {
		*dst = FloatValue(a.Number() + b.Number())
		return true
	}
	return false
}

func SubInts(dst, a, b *Value) bool {
	if a.IsInt() && b.IsInt() {
		dst.SetInt(a.RawInt() - b.RawInt())
		return true
	}
	return false
}

func SubNums(dst, a, b *Value) bool {
	if a.IsInt() && b.IsInt() {
		dst.SetInt(a.RawInt() - b.RawInt())
		return true
	}
	if a.IsNumber() && b.IsNumber() {
		*dst = FloatValue(a.Number() - b.Number())
		return true
	}
	return false
}

func MulInts(dst, a, b *Value) bool {
	if a.IsInt() && b.IsInt() {
		dst.SetInt(a.RawInt() * b.RawInt())
		return true
	}
	return false
}

func MulNums(dst, a, b *Value) bool {
	if a.IsInt() && b.IsInt() {
		dst.SetInt(a.RawInt() * b.RawInt())
		return true
	}
	if a.IsNumber() && b.IsNumber() {
		*dst = FloatValue(a.Number() * b.Number())
		return true
	}
	return false
}

func DivNums(dst, a, b *Value) bool {
	// DIV always returns float in Lua/GScript semantics (5/2 = 2.5).
	if a.IsInt() && b.IsInt() {
		*dst = FloatValue(float64(a.Int()) / float64(b.Int()))
		return true
	}
	if a.IsNumber() && b.IsNumber() {
		*dst = FloatValue(a.Number() / b.Number())
		return true
	}
	return false
}

func LTInts(a, b *Value) (bool, bool) {
	if a.IsInt() && b.IsInt() {
		return a.Int() < b.Int(), true
	}
	return false, false
}

func LEInts(a, b *Value) (bool, bool) {
	if a.IsInt() && b.IsInt() {
		return a.Int() <= b.Int(), true
	}
	return false, false
}

func EQStrings(a, b *Value) (bool, bool) {
	as, ok := a.RawString()
	if !ok {
		return false, false
	}
	bs, ok := b.RawString()
	if !ok {
		return false, false
	}
	return as == bs, true
}

func LTStrings(a, b *Value) (bool, bool) {
	as, ok := a.RawString()
	if !ok {
		return false, false
	}
	bs, ok := b.RawString()
	if !ok {
		return false, false
	}
	return as < bs, true
}

func LEStrings(a, b *Value) (bool, bool) {
	as, ok := a.RawString()
	if !ok {
		return false, false
	}
	bs, ok := b.RawString()
	if !ok {
		return false, false
	}
	return as <= bs, true
}

// ---------------------------------------------------------------------------
// String-to-number parsing helpers
// ---------------------------------------------------------------------------

// ParseNumberString applies the same string-to-number conversion used by
// tonumber and arithmetic coercions without requiring callers to allocate a
// transient runtime.StringValue.
func ParseNumberString(raw string) (Value, bool) {
	if v, ok := parseFastDecimalInt(raw); ok {
		return v, true
	}
	s := strings.TrimSpace(raw)
	if s != raw {
		if v, ok := parseFastDecimalInt(s); ok {
			return v, true
		}
	}
	if v, ok := parseLuaHexNumber(s); ok {
		return v, true
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return IntValue(i), true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return NilValue(), false
		}
		return FloatValue(f), true
	}
	return NilValue(), false
}

func parseFastDecimalInt(s string) (Value, bool) {
	if s == "" {
		return NilValue(), false
	}
	neg := false
	i := 0
	switch s[0] {
	case '-':
		neg = true
		i = 1
	case '+':
		i = 1
	}
	if i == len(s) {
		return NilValue(), false
	}
	var n uint64
	const maxPos = uint64(^uint64(0) >> 1)
	limit := maxPos
	if neg {
		limit++
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return NilValue(), false
		}
		d := uint64(c - '0')
		if n > (limit-d)/10 {
			return NilValue(), false
		}
		n = n*10 + d
	}
	if neg {
		if n == maxPos+1 {
			return IntValue(-1 << 63), true
		}
		return IntValue(-int64(n)), true
	}
	return IntValue(int64(n)), true
}

func parseLuaHexNumber(s string) (Value, bool) {
	if len(s) < 3 {
		return NilValue(), false
	}
	neg := false
	i := 0
	switch s[0] {
	case '-':
		neg = true
		i++
	case '+':
		i++
	}
	if i+2 > len(s) || s[i] != '0' || (s[i+1] != 'x' && s[i+1] != 'X') {
		return NilValue(), false
	}
	i += 2

	mant := 0.0
	digits := 0
	hasDot := false
	frac := 1.0 / 16.0
	for i < len(s) {
		c := s[i]
		if c == '.' {
			if hasDot {
				return NilValue(), false
			}
			hasDot = true
			i++
			continue
		}
		d := hexDigitValue(c)
		if d < 0 {
			break
		}
		if hasDot {
			mant += float64(d) * frac
			frac /= 16.0
		} else {
			mant = mant*16.0 + float64(d)
		}
		digits++
		i++
	}
	if digits == 0 {
		return NilValue(), false
	}

	hasExp := false
	exp := 0
	expNeg := false
	if i < len(s) && (s[i] == 'p' || s[i] == 'P') {
		hasExp = true
		i++
		if i < len(s) {
			switch s[i] {
			case '-':
				expNeg = true
				i++
			case '+':
				i++
			}
		}
		expDigits := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			exp = exp*10 + int(s[i]-'0')
			expDigits++
			i++
		}
		if expDigits == 0 {
			return NilValue(), false
		}
	}
	if i != len(s) {
		return NilValue(), false
	}
	if expNeg {
		exp = -exp
	}
	if hasExp {
		mant = math.Ldexp(mant, exp)
	}
	if neg {
		mant = -mant
	}
	if !hasDot && !hasExp && mant >= float64(math.MinInt64) && mant <= float64(math.MaxInt64) {
		return IntValue(int64(mant)), true
	}
	return FloatValue(mant), true
}

func hexDigitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}
