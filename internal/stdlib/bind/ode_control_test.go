package bind

import (
	"math"
	"testing"
)

func TestControlModulePrimitives(t *testing.T) {
	control := BuildControl()
	saturate := control.RawGetString("saturate").GoFunction()
	got, err := saturate.Fn([]Value{FloatValue(5), FloatValue(3)})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Number() != 3 {
		t.Fatalf("control.saturate = %v, want 3", got[0])
	}
	wrap := control.RawGetString("wrap_angle").GoFunction()
	got, err = wrap.Fn([]Value{FloatValue(3 * math.Pi)})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got[0].Number()+math.Pi) > 1e-12 && math.Abs(got[0].Number()-math.Pi) > 1e-12 {
		t.Fatalf("control.wrap_angle = %.17g, want +/-pi", got[0].Number())
	}
}

func TestControlLQR2ReturnsFiniteGain(t *testing.T) {
	control := BuildControl()
	lqr := control.RawGetString("lqr2").GoFunction()
	A := controlTestMatrix(2, 2, []float64{0, 1, 9.81, -0.1})
	B := controlTestMatrix(2, 1, []float64{0, 1})
	Q := controlTestMatrix(2, 2, []float64{10, 0, 0, 1})
	got, err := lqr.Fn([]Value{A, B, Q, FloatValue(1)})
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].IsDenseArray() {
		t.Fatalf("control.lqr2 returned %v, want dense array", got[0])
	}
	xs, ok := got[0].DenseArray().F64()
	if !ok || len(xs) != 2 {
		t.Fatalf("control.lqr2 gain = %#v, want f64[2]", got[0])
	}
	if math.IsNaN(xs[0]) || math.IsNaN(xs[1]) || math.IsInf(xs[0], 0) || math.IsInf(xs[1], 0) {
		t.Fatalf("control.lqr2 gain is not finite: %#v", xs)
	}
}

func TestODERK4CallsScriptDynamics(t *testing.T) {
	calls := 0
	ode := BuildODE(func(fn Value, args []Value) ([]Value, error) {
		calls++
		state, err := odeVectorFromValue(args[0], "test state")
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(NewDenseArrayF64([]float64{state[0]}))}, nil
	})
	rk4 := ode.RawGetString("rk4").GoFunction()
	got, err := rk4.Fn([]Value{
		FunctionValue(&GoFunction{Name: "test.dyn"}),
		DenseArrayValue(NewDenseArrayF64([]float64{1})),
		FloatValue(0.1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 {
		t.Fatalf("ode.rk4 calls = %d, want 4", calls)
	}
	xs, ok := got[0].DenseArray().F64()
	if !ok || len(xs) != 1 {
		t.Fatalf("ode.rk4 output = %v, want f64[1]", got[0])
	}
	if math.Abs(xs[0]-math.Exp(0.1)) > 1e-5 {
		t.Fatalf("ode.rk4 exp step = %.12f, want %.12f", xs[0], math.Exp(0.1))
	}
}

func controlTestMatrix(rows, cols int, values []float64) Value {
	m := NewDenseMatrix(rows, cols)
	backing, _, stride, _ := m.DenseMatrixBacking()
	for i, v := range values {
		row := i / cols
		col := i % cols
		backing[row*stride+col] = v
	}
	return TableValue(m)
}
