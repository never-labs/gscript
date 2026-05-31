package runtime

import stdmath "math"

// ToIntegerValue converts a numeric or numeric-string value to an exact integer,
// returning nil when the conversion is not possible.
func ToIntegerValue(v Value) Value {
	if v.IsInt() {
		return v
	}
	if v.IsString() {
		n, ok := v.ToNumber()
		if !ok {
			return NilValue()
		}
		v = n
	}
	if v.IsInt() {
		return v
	}
	if v.IsFloat() {
		f := v.Float()
		if f == stdmath.Trunc(f) && f >= stdmath.MinInt64 && f <= stdmath.MaxInt64 {
			return IntValue(int64(f))
		}
	}
	return NilValue()
}
