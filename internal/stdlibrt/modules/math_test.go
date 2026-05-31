package modules

import (
	stdmath "math"
	"testing"
)

func TestMathModuleBasicExecution(t *testing.T) {
	interp := runProgram(t, `
		f := math.floor(3.7)
		c := math.ceil(3.2)
		p := math.pow(2, 5)
		d := math.floorDiv(-7, 3)
	`)
	if got := interp.GetGlobal("f").Int(); got != 3 {
		t.Fatalf("math.floor = %d, want 3", got)
	}
	if got := interp.GetGlobal("c").Int(); got != 4 {
		t.Fatalf("math.ceil = %d, want 4", got)
	}
	if got := interp.GetGlobal("p").Number(); got != 32 {
		t.Fatalf("math.pow = %v, want 32", got)
	}
	if got := interp.GetGlobal("d").Int(); got != -3 {
		t.Fatalf("math.floorDiv = %d, want -3", got)
	}
}

func TestMathModuleRuntimeMigratedCoverage(t *testing.T) {
	interp := runProgram(t, `
		pi := math.pi
		huge := math.huge
		s := math.sin(0)
		c := math.cos(0)
		mx := math.max(1, 5, 3)
		mn := math.min(1, 5, 3)
		typ_i := math.type(42)
		typ_f := math.type(3.14)
		typ_s := math.type("hello")
		e := math.exp(1)
		l := math.log(math.exp(1))
		l10 := math.log(100, 10)
		i, frac := math.modf(3.75)
		math.randomseed(42)
		r0 := math.random()
		r1 := math.random(10)
		r2 := math.random(5, 10)
	`)
	if stdmath.Abs(interp.GetGlobal("pi").Number()-stdmath.Pi) > 1e-10 {
		t.Fatalf("math.pi = %v", interp.GetGlobal("pi"))
	}
	if !stdmath.IsInf(interp.GetGlobal("huge").Number(), 1) {
		t.Fatalf("math.huge = %v, want +Inf", interp.GetGlobal("huge"))
	}
	if got := interp.GetGlobal("s").Number(); got != 0 {
		t.Fatalf("math.sin(0) = %v, want 0", got)
	}
	if got := interp.GetGlobal("c").Number(); got != 1 {
		t.Fatalf("math.cos(0) = %v, want 1", got)
	}
	if got := interp.GetGlobal("mx").Int(); got != 5 {
		t.Fatalf("math.max = %d, want 5", got)
	}
	if got := interp.GetGlobal("mn").Int(); got != 1 {
		t.Fatalf("math.min = %d, want 1", got)
	}
	if got := interp.GetGlobal("typ_i").Str(); got != "integer" {
		t.Fatalf("math.type(integer) = %q", got)
	}
	if got := interp.GetGlobal("typ_f").Str(); got != "float" {
		t.Fatalf("math.type(float) = %q", got)
	}
	if interp.GetGlobal("typ_s").Truthy() {
		t.Fatalf("math.type(non-number) = %v, want nil/falsey", interp.GetGlobal("typ_s"))
	}
	if stdmath.Abs(interp.GetGlobal("e").Number()-stdmath.E) > 1e-10 {
		t.Fatalf("math.exp(1) = %v", interp.GetGlobal("e"))
	}
	if stdmath.Abs(interp.GetGlobal("l").Number()-1) > 1e-10 {
		t.Fatalf("math.log(e) = %v", interp.GetGlobal("l"))
	}
	if stdmath.Abs(interp.GetGlobal("l10").Number()-2) > 1e-10 {
		t.Fatalf("math.log(100, 10) = %v", interp.GetGlobal("l10"))
	}
	if got := interp.GetGlobal("i").Number(); got != 3 {
		t.Fatalf("math.modf integer part = %v", got)
	}
	if stdmath.Abs(interp.GetGlobal("frac").Number()-0.75) > 1e-10 {
		t.Fatalf("math.modf fraction = %v", interp.GetGlobal("frac"))
	}
	if r0 := interp.GetGlobal("r0"); !r0.IsFloat() || r0.Number() < 0 || r0.Number() >= 1 {
		t.Fatalf("math.random() = %v, want [0,1)", r0)
	}
	if r1 := interp.GetGlobal("r1").Int(); r1 < 1 || r1 > 10 {
		t.Fatalf("math.random(10) = %d, want [1,10]", r1)
	}
	if r2 := interp.GetGlobal("r2").Int(); r2 < 5 || r2 > 10 {
		t.Fatalf("math.random(5, 10) = %d, want [5,10]", r2)
	}
}

func TestMathModuleUnaryFastPaths(t *testing.T) {
	mathLib := BuildMath()
	cases := map[string]struct {
		in   Value
		want float64
	}{
		"abs":   {FloatValue(-10.5), 10.5},
		"ceil":  {FloatValue(3.1), 4},
		"floor": {FloatValue(3.7), 3},
		"sqrt":  {FloatValue(16), 4},
		"sin":   {FloatValue(0), 0},
		"cos":   {FloatValue(0), 1},
		"tan":   {FloatValue(0), 0},
		"asin":  {FloatValue(0), 0},
		"acos":  {FloatValue(1), 0},
		"atan":  {FloatValue(1), stdmath.Atan(1)},
		"deg":   {FloatValue(stdmath.Pi), 180},
		"rad":   {FloatValue(180), stdmath.Pi},
		"exp":   {FloatValue(1), stdmath.E},
		"log":   {FloatValue(stdmath.E), 1},
	}
	for name, tc := range cases {
		gf := requireMathGoFunction(t, mathLib, name)
		if gf.FastArg1 == nil || gf.Fast1 == nil {
			t.Fatalf("math.%s missing unary fast paths: %#v", name, gf)
		}
		gotArg, err := gf.FastArg1(tc.in)
		if err != nil {
			t.Fatalf("math.%s.FastArg1 error: %v", name, err)
		}
		gotSlice, err := gf.Fast1([]Value{tc.in})
		if err != nil {
			t.Fatalf("math.%s.Fast1 error: %v", name, err)
		}
		if stdmath.Abs(gotArg.Number()-tc.want) > 1e-12 {
			t.Fatalf("math.%s.FastArg1 = %.17g, want %.17g", name, gotArg.Number(), tc.want)
		}
		if stdmath.Abs(gotSlice.Number()-tc.want) > 1e-12 {
			t.Fatalf("math.%s.Fast1 = %.17g, want %.17g", name, gotSlice.Number(), tc.want)
		}
	}
}

func TestMathModuleBinaryFastPaths(t *testing.T) {
	mathLib := BuildMath()
	cases := map[string]struct {
		a, b Value
		want Value
	}{
		"atan":     {FloatValue(1), FloatValue(0), FloatValue(stdmath.Pi / 2)},
		"log":      {FloatValue(9), FloatValue(3), FloatValue(2)},
		"pow":      {IntValue(2), IntValue(5), FloatValue(32)},
		"floorDiv": {IntValue(7), IntValue(3), IntValue(2)},
		"fmod":     {IntValue(10), IntValue(3), IntValue(1)},
		"ult":      {IntValue(3), IntValue(4), BoolValue(true)},
		"max":      {IntValue(3), IntValue(9), IntValue(9)},
		"min":      {FloatValue(3.2), FloatValue(-9.2), FloatValue(-9.2)},
	}
	for name, tc := range cases {
		gf := requireMathGoFunction(t, mathLib, name)
		if gf.FastArg2 == nil || gf.Fast1 == nil {
			t.Fatalf("math.%s missing binary fast paths: %#v", name, gf)
		}
		gotArg, err := gf.FastArg2(tc.a, tc.b)
		if err != nil {
			t.Fatalf("math.%s.FastArg2 error: %v", name, err)
		}
		gotSlice, err := gf.Fast1([]Value{tc.a, tc.b})
		if err != nil {
			t.Fatalf("math.%s.Fast1 error: %v", name, err)
		}
		if !mathFastPathValueEqual(gotArg, tc.want) {
			t.Fatalf("math.%s.FastArg2 = %v, want %v", name, gotArg, tc.want)
		}
		if !mathFastPathValueEqual(gotSlice, tc.want) {
			t.Fatalf("math.%s.Fast1 = %v, want %v", name, gotSlice, tc.want)
		}
	}
}

func requireMathGoFunction(t *testing.T, mathLib *Table, name string) *GoFunction {
	t.Helper()
	v := mathLib.RawGetString(name)
	if !v.IsFunction() {
		t.Fatalf("math.%s not registered as function: %v", name, v)
	}
	gf := v.GoFunction()
	if gf == nil {
		t.Fatalf("math.%s GoFunction is nil", name)
	}
	return gf
}

func mathFastPathValueEqual(got, want Value) bool {
	if got.Type() != want.Type() {
		if (got.IsInt() || got.IsFloat()) && (want.IsInt() || want.IsFloat()) {
			return stdmath.Abs(got.Number()-want.Number()) <= 1e-12
		}
		return false
	}
	if got.IsFloat() || got.IsInt() {
		return stdmath.Abs(got.Number()-want.Number()) <= 1e-12
	}
	return got == want
}
