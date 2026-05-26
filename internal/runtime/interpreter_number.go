package runtime

// Numeric parsing/coercion helpers for the tree-walking interpreter:
// tonumberWithBase, digitValue, and parseNumber.
// Moved verbatim from interpreter.go (pure code movement).

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ------------------------------------------------------------------
// Number parsing
// ------------------------------------------------------------------
func tonumberWithBase(value, baseValue Value) (Value, bool) {
	if !value.IsString() {
		return NilValue(), false
	}
	base := toInt(baseValue)
	if base < 2 || base > 36 {
		return NilValue(), false
	}
	s := strings.TrimSpace(value.Str())
	if s == "" {
		return NilValue(), false
	}
	sign := int64(1)
	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		sign = -1
		s = s[1:]
	}
	if s == "" {
		return NilValue(), false
	}

	var acc uint64
	for i := 0; i < len(s); i++ {
		d := digitValue(s[i])
		if d < 0 || int64(d) >= base {
			return NilValue(), false
		}
		acc = acc*uint64(base) + uint64(d)
	}
	if sign < 0 {
		if acc <= uint64(math.MaxInt64)+1 {
			return IntValue(-int64(acc)), true
		}
		return FloatValue(-float64(acc)), true
	}
	if acc <= uint64(math.MaxInt64) {
		return IntValue(int64(acc)), true
	}
	return FloatValue(float64(acc)), true
}

func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	default:
		return -1
	}
}

func parseNumber(s string) (Value, error) {
	if i, err := strconv.ParseInt(s, 0, 64); err == nil {
		return IntValue(i), nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return FloatValue(f), nil
	}
	return NilValue(), fmt.Errorf("invalid number: %s", s)
}
