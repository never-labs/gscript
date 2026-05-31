package runtime

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	basemath "github.com/never-labs/gscript/internal/stdlib/base/math"
	baserand "github.com/never-labs/gscript/internal/stdlib/base/rand"
)

// buildMathLib creates the "math" standard library table.
func buildMathLib() *Table {
	t := NewTable()
	randomSeed1 := time.Now().UnixNano()
	randomSeed2 := int64(0)
	rng := rand.New(rand.NewSource(randomSeed1))

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "math." + name,
			Fn:   fn,
		}))
	}
	setUnaryFloat := func(name string, fn func(float64) float64) {
		set(name, func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'math.%s'", name)
			}
			return []Value{FloatValue(fn(toFloat(args[0])))}, nil
		})
		if v := t.RawGetString(name); v.IsFunction() {
			gf := v.GoFunction()
			gf.FastArg1 = func(arg Value) (Value, error) {
				return FloatValue(fn(toFloat(arg))), nil
			}
			gf.Fast1 = func(args []Value) (Value, error) {
				if len(args) < 1 {
					return NilValue(), fmt.Errorf("bad argument #1 to 'math.%s'", name)
				}
				return FloatValue(fn(toFloat(args[0]))), nil
			}
		}
	}

	// Constants
	t.RawSet(StringValue("pi"), FloatValue(math.Pi))
	t.RawSet(StringValue("huge"), FloatValue(math.Inf(1)))
	t.RawSet(StringValue("maxinteger"), IntValue(math.MaxInt64))
	t.RawSet(StringValue("mininteger"), IntValue(math.MinInt64))

	// math.abs(x)
	set("abs", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.abs'")
		}
		return []Value{mathAbsValue(args[0])}, nil
	})
	if v := t.RawGetString("abs"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg1 = func(arg Value) (Value, error) {
			return mathAbsValue(arg), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'math.abs'")
			}
			return mathAbsValue(args[0]), nil
		}
	}

	// math.ceil(x) -> int
	set("ceil", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.ceil'")
		}
		return []Value{mathCeilValue(args[0])}, nil
	})
	if v := t.RawGetString("ceil"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg1 = func(arg Value) (Value, error) {
			return mathCeilValue(arg), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'math.ceil'")
			}
			return mathCeilValue(args[0]), nil
		}
	}

	// math.floor(x) -> int
	set("floor", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.floor'")
		}
		return []Value{mathFloorValue(args[0])}, nil
	})
	if v := t.RawGetString("floor"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg1 = func(arg Value) (Value, error) {
			return mathFloorValue(arg), nil
		}
		// Fast1 mirrors FastArg1 so VM dispatch sites that only check Fast1
		// (vm.go OP_CALL fast path, vm.callValue, tier2-exit fallback) avoid
		// the slow Fn path. Without this, log_tokenize_format records 144K
		// native_call.fallback hits despite math.floor being a one-arg builtin.
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'math.floor'")
			}
			return mathFloorValue(args[0]), nil
		}
	}

	// math.floorDiv(a, b) -> int|float
	set("floorDiv", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'math.floorDiv'")
		}
		v, err := mathFloorDivValue(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	if v := t.RawGetString("floorDiv"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg2 = mathFloorDivValue
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 2 {
				return NilValue(), fmt.Errorf("bad argument to 'math.floorDiv'")
			}
			return mathFloorDivValue(args[0], args[1])
		}
	}

	// math.sqrt(x)
	setUnaryFloat("sqrt", math.Sqrt)

	// math.sin(x)
	setUnaryFloat("sin", math.Sin)

	// math.cos(x)
	setUnaryFloat("cos", math.Cos)

	// math.tan(x)
	setUnaryFloat("tan", math.Tan)

	// math.asin(x)
	setUnaryFloat("asin", math.Asin)

	// math.acos(x)
	setUnaryFloat("acos", math.Acos)

	// math.atan(y [, x])
	set("atan", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.atan'")
		}
		y := toFloat(args[0])
		if len(args) >= 2 {
			x := toFloat(args[1])
			return []Value{FloatValue(math.Atan2(y, x))}, nil
		}
		return []Value{FloatValue(math.Atan(y))}, nil
	})
	if v := t.RawGetString("atan"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg1 = func(arg Value) (Value, error) {
			return FloatValue(math.Atan(toFloat(arg))), nil
		}
		gf.FastArg2 = func(a, b Value) (Value, error) {
			return FloatValue(math.Atan2(toFloat(a), toFloat(b))), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'math.atan'")
			}
			if len(args) >= 2 {
				return FloatValue(math.Atan2(toFloat(args[0]), toFloat(args[1]))), nil
			}
			return FloatValue(math.Atan(toFloat(args[0]))), nil
		}
	}

	// math.deg(x)
	setUnaryFloat("deg", func(x float64) float64 { return x * 180 / math.Pi })

	// math.rad(x)
	setUnaryFloat("rad", func(x float64) float64 { return x * math.Pi / 180 })

	// math.exp(x)
	setUnaryFloat("exp", math.Exp)

	// math.log(x [, base])
	set("log", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.log'")
		}
		x := toFloat(args[0])
		if len(args) >= 2 {
			base := toFloat(args[1])
			return []Value{FloatValue(math.Log(x) / math.Log(base))}, nil
		}
		return []Value{FloatValue(math.Log(x))}, nil
	})
	if v := t.RawGetString("log"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg1 = func(arg Value) (Value, error) {
			return FloatValue(math.Log(toFloat(arg))), nil
		}
		gf.FastArg2 = func(a, b Value) (Value, error) {
			return FloatValue(math.Log(toFloat(a)) / math.Log(toFloat(b))), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'math.log'")
			}
			x := toFloat(args[0])
			if len(args) >= 2 {
				return FloatValue(math.Log(x) / math.Log(toFloat(args[1]))), nil
			}
			return FloatValue(math.Log(x)), nil
		}
	}

	// math.max(x, ...)
	set("max", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.max'")
		}
		return []Value{mathMaxValue(args)}, nil
	})
	if v := t.RawGetString("max"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg2 = func(a, b Value) (Value, error) {
			return mathMax2Value(a, b), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'math.max'")
			}
			return mathMaxValue(args), nil
		}
	}

	// math.min(x, ...)
	set("min", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.min'")
		}
		return []Value{mathMinValue(args)}, nil
	})
	if v := t.RawGetString("min"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg2 = func(a, b Value) (Value, error) {
			return mathMin2Value(a, b), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), fmt.Errorf("bad argument #1 to 'math.min'")
			}
			return mathMinValue(args), nil
		}
	}

	// math.fmod(x, y)
	set("fmod", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'math.fmod'")
		}
		v, err := mathFmodValue(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})
	if v := t.RawGetString("fmod"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg2 = mathFmodValue
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 2 {
				return NilValue(), fmt.Errorf("bad argument to 'math.fmod'")
			}
			return mathFmodValue(args[0], args[1])
		}
	}

	// math.ult(m, n) -> unsigned integer comparison
	set("ult", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'math.ult'")
		}
		return []Value{BoolValue(uint64(toInt(args[0])) < uint64(toInt(args[1])))}, nil
	})
	if v := t.RawGetString("ult"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg2 = func(a, b Value) (Value, error) {
			return BoolValue(uint64(toInt(a)) < uint64(toInt(b))), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 2 {
				return NilValue(), fmt.Errorf("bad argument to 'math.ult'")
			}
			return BoolValue(uint64(toInt(args[0])) < uint64(toInt(args[1]))), nil
		}
	}

	// math.modf(x) -> int, frac
	set("modf", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.modf'")
		}
		if args[0].IsInt() {
			return []Value{args[0], FloatValue(0)}, nil
		}
		x := toFloat(args[0])
		if math.IsInf(x, 0) {
			return []Value{FloatValue(x), FloatValue(0)}, nil
		}
		i, f := math.Modf(x)
		return []Value{FloatValue(i), FloatValue(f)}, nil
	})

	// math.pow(x, y)  -- same as x ** y
	set("pow", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'math.pow'")
		}
		return []Value{FloatValue(math.Pow(toFloat(args[0]), toFloat(args[1])))}, nil
	})
	if v := t.RawGetString("pow"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg2 = func(a, b Value) (Value, error) {
			return FloatValue(math.Pow(toFloat(a), toFloat(b))), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 2 {
				return NilValue(), fmt.Errorf("bad argument to 'math.pow'")
			}
			return FloatValue(math.Pow(toFloat(args[0]), toFloat(args[1]))), nil
		}
	}

	// math.random([m [, n]])
	set("random", func(args []Value) ([]Value, error) {
		if len(args) > 2 {
			return nil, fmt.Errorf("wrong number of arguments")
		}
		if len(args) == 0 {
			return []Value{FloatValue(rng.Float64())}, nil
		}
		if len(args) == 1 {
			m := toInt(args[0])
			if m == 0 {
				return []Value{IntValue(rng.Int63())}, nil
			}
			if m < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'math.random' (interval is empty)")
			}
			return []Value{IntValue(rng.Int63n(m) + 1)}, nil
		}
		m := toInt(args[0])
		n := toInt(args[1])
		if m > n {
			return nil, fmt.Errorf("bad argument #2 to 'math.random' (interval is empty)")
		}
		return []Value{IntValue(m + rng.Int63n(n-m+1))}, nil
	})

	// math.randomseed([x [, y]])
	set("randomseed", func(args []Value) ([]Value, error) {
		if len(args) > 2 {
			return nil, fmt.Errorf("wrong number of arguments")
		}
		if len(args) == 0 {
			randomSeed1 = time.Now().UnixNano() & ((1 << 47) - 1)
			randomSeed2 = 0
		} else {
			randomSeed1 = toInt(args[0])
			if len(args) >= 2 {
				randomSeed2 = toInt(args[1])
			} else {
				randomSeed2 = 0
			}
		}
		rng = rand.New(rand.NewSource(baserand.MixSeedPair(randomSeed1, randomSeed2)))
		return []Value{IntValue(randomSeed1), IntValue(randomSeed2)}, nil
	})

	// math.type(x) -> "integer" | "float" | false
	set("type", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return []Value{BoolValue(false)}, nil
		}
		v := args[0]
		if v.IsInt() {
			return []Value{StringValue("integer")}, nil
		}
		if v.IsFloat() {
			return []Value{StringValue("float")}, nil
		}
		return []Value{BoolValue(false)}, nil
	})

	// math.tointeger(x) -- convert float to int (exact), nil if not exact
	set("tointeger", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return []Value{NilValue()}, nil
		}
		return []Value{mathToIntegerValue(args[0])}, nil
	})
	if v := t.RawGetString("tointeger"); v.IsFunction() {
		gf := v.GoFunction()
		gf.FastArg1 = func(arg Value) (Value, error) {
			return mathToIntegerValue(arg), nil
		}
		gf.Fast1 = func(args []Value) (Value, error) {
			if len(args) < 1 {
				return NilValue(), nil
			}
			return mathToIntegerValue(args[0]), nil
		}
	}

	// math.clamp(x, min, max) -- clamp x to [min, max] range
	set("clamp", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'math.clamp' (3 arguments expected)")
		}
		return []Value{mathNumberValue(basemath.Clamp(valueMathNumber(args[0]), valueMathNumber(args[1]), valueMathNumber(args[2])))}, nil
	})

	// math.lerp(a, b, t) -- linear interpolation: a + (b-a)*t
	set("lerp", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'math.lerp' (3 arguments expected)")
		}
		return []Value{FloatValue(basemath.Lerp(valueMathNumber(args[0]), valueMathNumber(args[1]), valueMathNumber(args[2])))}, nil
	})

	// math.sign(x) -- -1, 0, or 1
	set("sign", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.sign'")
		}
		return []Value{IntValue(basemath.Sign(valueMathNumber(args[0])))}, nil
	})

	// math.round(x [, n]) -- round to n decimal places (n=0 rounds to integer)
	set("round", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.round'")
		}
		n := int64(0)
		if len(args) >= 2 {
			n = toInt(args[1])
		}
		return []Value{mathNumberValue(basemath.Round(valueMathNumber(args[0]), n))}, nil
	})

	// math.trunc(x) -- truncate toward zero
	set("trunc", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.trunc'")
		}
		return []Value{mathNumberValue(basemath.Trunc(valueMathNumber(args[0])))}, nil
	})

	// math.hypot(x, y) -- sqrt(x*x + y*y)
	set("hypot", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'math.hypot'")
		}
		return []Value{FloatValue(math.Hypot(toFloat(args[0]), toFloat(args[1])))}, nil
	})

	// math.isnan(x) -> bool
	set("isnan", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.isnan'")
		}
		return []Value{BoolValue(math.IsNaN(toFloat(args[0])))}, nil
	})

	// math.isinf(x) -> bool
	set("isinf", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'math.isinf'")
		}
		return []Value{BoolValue(math.IsInf(toFloat(args[0]), 0))}, nil
	})

	return t
}

func mathFmodValue(a, b Value) (Value, error) {
	v, ok := basemath.Fmod(valueMathNumber(a), valueMathNumber(b))
	if !ok {
		return NilValue(), fmt.Errorf("bad argument #2 to 'math.fmod' (zero)")
	}
	return mathNumberValue(v), nil
}

func mathToIntegerValue(v Value) Value {
	return ToIntegerValue(v)
}

func mathAbsValue(arg Value) Value {
	return mathNumberValue(basemath.Abs(valueMathNumber(arg)))
}

func mathMaxValue(args []Value) Value {
	best := args[0]
	for _, v := range args[1:] {
		best = mathMax2Value(best, v)
	}
	return best
}

func mathMax2Value(a, b Value) Value {
	lt, ok := a.LessThan(b)
	if ok && lt {
		return b
	}
	return a
}

func mathMinValue(args []Value) Value {
	best := args[0]
	for _, v := range args[1:] {
		best = mathMin2Value(best, v)
	}
	return best
}

func mathMin2Value(a, b Value) Value {
	lt, ok := a.LessThan(b)
	if ok && lt {
		return a
	}
	return b
}

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
	if v.IsInt() || v.IsFloat() {
		if n, ok := basemath.ToInteger(valueMathNumber(v)); ok {
			return IntValue(n)
		}
	}
	return NilValue()
}

func mathFloorValue(arg Value) Value {
	return mathNumberValue(basemath.Floor(valueMathNumber(arg)))
}

func mathFloorDivValue(a, b Value) (Value, error) {
	v, ok := basemath.FloorDiv(valueMathNumber(a), valueMathNumber(b))
	if !ok {
		return NilValue(), fmt.Errorf("bad argument #2 to 'math.floorDiv' (zero)")
	}
	return mathNumberValue(v), nil
}

func mathCeilValue(arg Value) Value {
	return mathNumberValue(basemath.Ceil(valueMathNumber(arg)))
}

func valueMathNumber(v Value) basemath.Number {
	if v.IsInt() {
		return basemath.Int(v.Int())
	}
	return basemath.Float(toFloat(v))
}

func mathNumberValue(n basemath.Number) Value {
	if n.IsInt {
		return IntValue(n.Int)
	}
	return FloatValue(n.Float)
}
