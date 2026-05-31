package math

import stdmath "math"

// Number is the pure stdlib math representation used by runtime adapters.
type Number struct {
	Int   int64
	Float float64
	IsInt bool
}

func Int(n int64) Number {
	return Number{Int: n, Float: float64(n), IsInt: true}
}

func Float(f float64) Number {
	return Number{Float: f}
}

func (n Number) Float64() float64 {
	if n.IsInt {
		return float64(n.Int)
	}
	return n.Float
}

func Abs(n Number) Number {
	if n.IsInt {
		v := n.Int
		if v < 0 {
			v = -v
		}
		return Int(v)
	}
	return Float(stdmath.Abs(n.Float))
}

func Floor(n Number) Number {
	if n.IsInt {
		return n
	}
	f := stdmath.Floor(n.Float)
	if stdmath.IsInf(f, 0) || stdmath.IsNaN(f) || f < -(1<<47) || f > (1<<47)-1 {
		return Float(f)
	}
	return Int(int64(f))
}

func Ceil(n Number) Number {
	if n.IsInt {
		return n
	}
	f := stdmath.Ceil(n.Float)
	if stdmath.IsInf(f, 0) || stdmath.IsNaN(f) || f < -(1<<47) || f > (1<<47)-1 {
		return Float(f)
	}
	return Int(int64(f))
}

func FloorDiv(a, b Number) (Number, bool) {
	if b.Float64() == 0 {
		return Number{}, false
	}
	if a.IsInt && b.IsInt {
		if a.Int == stdmath.MinInt64 && b.Int == -1 {
			return Float(stdmath.Floor(float64(a.Int) / float64(b.Int))), true
		}
		q := a.Int / b.Int
		r := a.Int % b.Int
		if r != 0 && ((r < 0) != (b.Int < 0)) {
			q--
		}
		return Int(q), true
	}
	return Floor(Float(a.Float64() / b.Float64())), true
}

func Fmod(a, b Number) (Number, bool) {
	if b.Float64() == 0 {
		return Number{}, false
	}
	if a.IsInt && b.IsInt {
		return Int(a.Int % b.Int), true
	}
	if !a.IsInt && !b.IsInt {
		return Float(stdmath.Mod(a.Float, b.Float)), true
	}
	return Float(stdmath.Mod(a.Float64(), b.Float64())), true
}

func Clamp(x, min, max Number) Number {
	xf := x.Float64()
	mn := min.Float64()
	mx := max.Float64()
	if xf < mn {
		xf = mn
	} else if xf > mx {
		xf = mx
	}
	if x.IsInt && min.IsInt && max.IsInt {
		return Int(int64(xf))
	}
	return Float(xf)
}

func Lerp(a, b, t Number) float64 {
	af := a.Float64()
	return af + (b.Float64()-af)*t.Float64()
}

func Sign(n Number) int64 {
	x := n.Float64()
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

func Round(x Number, places int64) Number {
	xf := x.Float64()
	if places == 0 {
		return Int(int64(stdmath.Round(xf)))
	}
	factor := stdmath.Pow(10, float64(places))
	return Float(stdmath.Round(xf*factor) / factor)
}

func Trunc(n Number) Number {
	if n.IsInt {
		return n
	}
	return Int(int64(stdmath.Trunc(n.Float)))
}

func ToInteger(n Number) (int64, bool) {
	if n.IsInt {
		return n.Int, true
	}
	if n.Float == stdmath.Trunc(n.Float) && n.Float >= stdmath.MinInt64 && n.Float <= stdmath.MaxInt64 {
		return int64(n.Float), true
	}
	return 0, false
}
