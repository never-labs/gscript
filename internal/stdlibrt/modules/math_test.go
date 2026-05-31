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
