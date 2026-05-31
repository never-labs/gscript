package runtime

import basemath "github.com/never-labs/gscript/internal/stdlib/math"

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
		if n, ok := basemath.ToInteger(basemath.Float(v.Float())); ok {
			return IntValue(n)
		}
	}
	return NilValue()
}
