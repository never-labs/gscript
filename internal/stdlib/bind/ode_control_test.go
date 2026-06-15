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
	feedback := control.RawGetString("feedback").GoFunction()
	got, err = feedback.Fn([]Value{
		DenseArrayValue(NewDenseArrayF64([]float64{2, 3})),
		DenseArrayValue(NewDenseArrayF64([]float64{4, 5})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Number() != -23 {
		t.Fatalf("control.feedback = %v, want -23", got[0])
	}
	got, err = feedback.Fn([]Value{
		DenseArrayValue(NewDenseArrayF64([]float64{2, 3})),
		DenseArrayValue(NewDenseArrayF64([]float64{4, 5})),
		TableValue(controlTestOptions(map[string]Value{"limit": FloatValue(10)})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Number() != -10 {
		t.Fatalf("limited control.feedback = %v, want -10", got[0])
	}
	got, err = feedback.Fn([]Value{
		controlTestMatrix(1, 2, []float64{2, 3}),
		DenseArrayValue(NewDenseArrayF64([]float64{4, 5})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Number() != -23 {
		t.Fatalf("matrix-row control.feedback = %v, want -23", got[0])
	}
	got, err = saturate.Fn([]Value{
		DenseArrayValue(NewDenseArrayF64([]float64{5, -6})),
		FloatValue(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertTableFloat(t, got[0], 1, 3)
	assertTableFloat(t, got[0], 2, -3)
	clamp := control.RawGetString("clamp").GoFunction()
	got, err = clamp.Fn([]Value{
		controlTestMatrix(1, 2, []float64{-2, 5}),
		FloatValue(0),
		FloatValue(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertMatrixFloat(t, got[0], 1, 2, 1, 0)
	assertMatrixFloat(t, got[0], 1, 2, 2, 3)
	got, err = feedback.Fn([]Value{
		controlTestMatrix(2, 2, []float64{1, 0, 0, 2}),
		DenseArrayValue(NewDenseArrayF64([]float64{3, 4})),
		TableValue(controlTestOptions(map[string]Value{
			"reference":   DenseArrayValue(NewDenseArrayF64([]float64{1, 1})),
			"feedforward": DenseArrayValue(NewDenseArrayF64([]float64{10, 20})),
			"limit":       FloatValue(15),
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertTableFloat(t, got[0], 1, 8)
	assertTableFloat(t, got[0], 2, 14)
}

func TestControlLQR2ReturnsFiniteGain(t *testing.T) {
	control := BuildControl()
	lqr := control.RawGetString("lqr2").GoFunction()
	generic := control.RawGetString("lqr").GoFunction()
	A := controlTestMatrix(2, 2, []float64{0, 1, 9.81, -0.1})
	B := controlTestMatrix(2, 1, []float64{0, 1})
	Q := controlTestMatrix(2, 2, []float64{10, 0, 0, 1})
	opts := TableValue(controlTestOptions(map[string]Value{
		"iterations": IntValue(20000),
		"step":       FloatValue(0.001),
	}))
	got, err := lqr.Fn([]Value{A, B, Q, FloatValue(1), opts})
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
	genericGain, err := generic.Fn([]Value{A, B, Q, FloatValue(1), opts})
	if err != nil {
		t.Fatal(err)
	}
	backing, rows, stride, ok := genericGain[0].Table().DenseMatrixBacking()
	if !ok || rows != 1 || stride != 2 {
		t.Fatalf("control.lqr returned %v, want 1x2 matrix", genericGain[0])
	}
	for i, want := range xs {
		if math.Abs(backing[i]-want) > 1e-9 {
			t.Fatalf("control.lqr gain[%d] = %.12f, want lqr2 %.12f", i+1, backing[i], want)
		}
	}
}

func TestControlLQRReturnsFiniteSingleInputGain(t *testing.T) {
	control := BuildControl()
	lqr := control.RawGetString("lqr").GoFunction()
	A := controlTestMatrix(2, 2, []float64{0, 1, 9.81, -0.1})
	B := controlTestMatrix(2, 1, []float64{0, 1})
	Q := controlTestMatrix(2, 2, []float64{10, 0, 0, 1})
	got, err := lqr.Fn([]Value{A, B, Q, FloatValue(1)})
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].IsTable() {
		t.Fatalf("control.lqr returned %v, want gain matrix", got[0])
	}
	if backing, _, _, ok := got[0].Table().DenseMatrixBacking(); ok {
		for _, x := range backing[:2] {
			if math.IsNaN(x) || math.IsInf(x, 0) {
				t.Fatalf("control.lqr gain is not finite: %#v", backing[:2])
			}
		}
	}
}

func TestControlLQRKnownScalarCARE(t *testing.T) {
	control := BuildControl()
	lqr := control.RawGetString("lqr").GoFunction()
	got, err := lqr.Fn([]Value{
		controlTestMatrix(1, 1, []float64{1}),
		controlTestMatrix(1, 1, []float64{1}),
		controlTestMatrix(1, 1, []float64{1}),
		FloatValue(1),
		TableValue(controlTestOptions(map[string]Value{
			"iterations": IntValue(20000),
			"step":       FloatValue(0.001),
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := 1 + math.Sqrt2
	backing, rows, stride, ok := got[0].Table().DenseMatrixBacking()
	if !ok || rows != 1 || stride != 1 {
		t.Fatalf("control.lqr scalar CARE returned %v, want 1x1 matrix", got[0])
	}
	if math.Abs(backing[0]-want) > 1e-3 {
		t.Fatalf("control.lqr scalar CARE = %.12f, want %.12f", backing[0], want)
	}
}

func TestControlLQRReturnsMatrixGainForMultiInput(t *testing.T) {
	control := BuildControl()
	lqr := control.RawGetString("lqr").GoFunction()
	A := controlTestMatrix(2, 2, []float64{0, 1, -1, -0.2})
	B := controlTestMatrix(2, 2, []float64{1, 0, 0, 1})
	Q := controlTestMatrix(2, 2, []float64{2, 0, 0, 1})
	R := controlTestMatrix(2, 2, []float64{1, 0, 0, 1})
	got, err := lqr.Fn([]Value{A, B, Q, R, TableValue(controlTestOptions(map[string]Value{
		"iterations": IntValue(1000),
		"step":       FloatValue(0.001),
	}))})
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].IsTable() {
		t.Fatalf("control.lqr returned %s, want matrix", got[0].TypeName())
	}
	if backing, _, _, ok := got[0].Table().DenseMatrixBacking(); ok {
		for _, x := range backing[:4] {
			if math.IsNaN(x) || math.IsInf(x, 0) {
				t.Fatalf("control.lqr matrix gain is not finite: %#v", backing[:4])
			}
		}
	}
}

func TestControlLQRRejectsInvalidWeights(t *testing.T) {
	control := BuildControl()
	lqr := control.RawGetString("lqr").GoFunction()
	A := controlTestMatrix(1, 1, []float64{1})
	B := controlTestMatrix(1, 1, []float64{1})
	if _, err := lqr.Fn([]Value{A, B, controlTestMatrix(1, 1, []float64{-1}), FloatValue(1)}); err == nil {
		t.Fatal("control.lqr accepted negative Q diagonal")
	}
	if _, err := lqr.Fn([]Value{A, B, controlTestMatrix(1, 1, []float64{1}), FloatValue(-1)}); err == nil {
		t.Fatal("control.lqr accepted negative R")
	}
	if _, err := lqr.Fn([]Value{A, B, controlTestMatrix(1, 1, []float64{1}), FloatValue(1), TableValue(controlTestOptions(map[string]Value{"iterations": IntValue(0)}))}); err == nil {
		t.Fatal("control.lqr accepted zero iterations")
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

func TestODEIntegrateRunsFixedRK4Steps(t *testing.T) {
	calls := 0
	ode := BuildODE(func(fn Value, args []Value) ([]Value, error) {
		calls++
		state, err := odeVectorFromValue(args[0], "test state")
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(NewDenseArrayF64([]float64{state[0]}))}, nil
	})
	integrate := ode.RawGetString("integrate").GoFunction()
	got, err := integrate.Fn([]Value{
		FunctionValue(&GoFunction{Name: "test.dyn"}),
		DenseArrayValue(NewDenseArrayF64([]float64{1})),
		FloatValue(0.1),
		IntValue(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 8 {
		t.Fatalf("ode.integrate calls = %d, want 8", calls)
	}
	xs, ok := got[0].DenseArray().F64()
	if !ok || len(xs) != 1 {
		t.Fatalf("ode.integrate output = %v, want f64[1]", got[0])
	}
	if math.Abs(xs[0]-math.Exp(0.2)) > 2e-5 {
		t.Fatalf("ode.integrate exp result = %.12f, want %.12f", xs[0], math.Exp(0.2))
	}
}

func TestODEIntegrateCanReturnTrajectory(t *testing.T) {
	ode := BuildODE(func(fn Value, args []Value) ([]Value, error) {
		state, err := odeVectorFromValue(args[0], "test state")
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(NewDenseArrayF64([]float64{1 + 0*state[0]}))}, nil
	})
	integrate := ode.RawGetString("integrate").GoFunction()
	got, err := integrate.Fn([]Value{
		FunctionValue(&GoFunction{Name: "test.dyn"}),
		DenseArrayValue(NewDenseArrayF64([]float64{0})),
		FloatValue(0.25),
		IntValue(3),
		TableValue(controlTestOptions(map[string]Value{"trajectory": BoolValue(true)})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[1].IsTable() {
		t.Fatalf("ode.integrate trajectory return = %#v, want final and table", got)
	}
	final, ok := got[0].DenseArray().F64()
	if !ok || len(final) != 1 || math.Abs(final[0]-0.75) > 1e-12 {
		t.Fatalf("ode.integrate final = %#v, want 0.75", got[0])
	}
	if got[1].Table().Length() != 3 {
		t.Fatalf("trajectory length = %d, want 3", got[1].Table().Length())
	}
}

func TestODEIntegrateProjectAndObserveHooks(t *testing.T) {
	calls := map[string]int{}
	ode := BuildODE(func(fn Value, args []Value) ([]Value, error) {
		name := fn.GoFunction().Name
		calls[name]++
		state, err := odeVectorFromValue(args[0], "test state")
		if err != nil {
			return nil, err
		}
		switch name {
		case "test.dyn":
			return []Value{DenseArrayValue(NewDenseArrayF64([]float64{1}))}, nil
		case "test.project":
			return []Value{DenseArrayValue(NewDenseArrayF64([]float64{2 * state[0]}))}, nil
		case "test.observe":
			step := args[1].Number()
			tm := args[2].Number()
			return []Value{FloatValue(state[0] + step + tm)}, nil
		default:
			t.Fatalf("unexpected function %s", name)
			return nil, nil
		}
	})
	integrate := ode.RawGetString("integrate").GoFunction()
	got, err := integrate.Fn([]Value{
		FunctionValue(&GoFunction{Name: "test.dyn"}),
		DenseArrayValue(NewDenseArrayF64([]float64{0})),
		FloatValue(1),
		IntValue(3),
		TableValue(controlTestOptions(map[string]Value{
			"trajectory": BoolValue(true),
			"project":    FunctionValue(&GoFunction{Name: "test.project"}),
			"observe":    FunctionValue(&GoFunction{Name: "test.observe"}),
			"method":     StringValue("rk4"),
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("ode.integrate returns %d values, want 3", len(got))
	}
	final, ok := got[0].DenseArray().F64()
	if !ok || len(final) != 1 || final[0] != 14 {
		t.Fatalf("final = %#v, want 14", got[0])
	}
	assertTableFloat(t, got[1].Table().RawGetInt(1), 1, 2)
	assertTableFloat(t, got[1].Table().RawGetInt(2), 1, 6)
	assertTableFloat(t, got[1].Table().RawGetInt(3), 1, 14)
	assertFloat(t, got[2].Table().RawGetInt(1), 4)
	assertFloat(t, got[2].Table().RawGetInt(2), 10)
	assertFloat(t, got[2].Table().RawGetInt(3), 20)
	if calls["test.dyn"] != 12 || calls["test.project"] != 3 || calls["test.observe"] != 3 {
		t.Fatalf("calls = %#v, want dyn=12 project=3 observe=3", calls)
	}
}

func TestODESolveWrapsTrajectoryAndObservedResult(t *testing.T) {
	ode := BuildODE(func(fn Value, args []Value) ([]Value, error) {
		name := fn.GoFunction().Name
		state, err := odeVectorFromValue(args[0], "test state")
		if err != nil {
			return nil, err
		}
		switch name {
		case "test.dyn":
			return []Value{DenseArrayValue(NewDenseArrayF64([]float64{1}))}, nil
		case "test.project":
			return []Value{DenseArrayValue(NewDenseArrayF64([]float64{2 * state[0]}))}, nil
		case "test.observe":
			step := args[1].Number()
			tm := args[2].Number()
			return []Value{FloatValue(state[0] + step + tm)}, nil
		default:
			t.Fatalf("unexpected function %s", name)
			return nil, nil
		}
	})
	solve := ode.RawGetString("solve").GoFunction()
	got, err := solve.Fn([]Value{
		FunctionValue(&GoFunction{Name: "test.dyn"}),
		DenseArrayValue(NewDenseArrayF64([]float64{0})),
		TableValue(controlTestOptions(map[string]Value{
			"dt":         FloatValue(1),
			"steps":      IntValue(3),
			"trajectory": BoolValue(true),
			"project":    FunctionValue(&GoFunction{Name: "test.project"}),
			"observe":    FunctionValue(&GoFunction{Name: "test.observe"}),
			"method":     StringValue("rk4"),
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].IsTable() {
		t.Fatalf("ode.solve returns %#v, want result table", got)
	}
	result := got[0].Table()
	assertTableFloat(t, result.RawGetString("final"), 1, 14)
	assertTableFloat(t, result.RawGetString("trajectory").Table().RawGetInt(1), 1, 2)
	assertTableFloat(t, result.RawGetString("trajectory").Table().RawGetInt(2), 1, 6)
	assertTableFloat(t, result.RawGetString("trajectory").Table().RawGetInt(3), 1, 14)
	assertFloat(t, result.RawGetString("observed").Table().RawGetInt(1), 4)
	assertFloat(t, result.RawGetString("observed").Table().RawGetInt(2), 10)
	assertFloat(t, result.RawGetString("observed").Table().RawGetInt(3), 20)
}

func TestODESolveWrapsObservedWithoutTrajectory(t *testing.T) {
	ode := BuildODE(func(fn Value, args []Value) ([]Value, error) {
		name := fn.GoFunction().Name
		state, err := odeVectorFromValue(args[0], "test state")
		if err != nil {
			return nil, err
		}
		switch name {
		case "test.dyn":
			return []Value{DenseArrayValue(NewDenseArrayF64([]float64{1 + 0*state[0]}))}, nil
		case "test.observe":
			return []Value{FloatValue(state[0])}, nil
		default:
			t.Fatalf("unexpected function %s", name)
			return nil, nil
		}
	})
	solve := ode.RawGetString("solve").GoFunction()
	got, err := solve.Fn([]Value{
		FunctionValue(&GoFunction{Name: "test.dyn"}),
		DenseArrayValue(NewDenseArrayF64([]float64{0})),
		TableValue(controlTestOptions(map[string]Value{
			"dt":      FloatValue(0.25),
			"steps":   IntValue(3),
			"observe": FunctionValue(&GoFunction{Name: "test.observe"}),
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := got[0].Table()
	assertTableFloat(t, result.RawGetString("final"), 1, 0.75)
	if !result.RawGetString("trajectory").IsNil() {
		t.Fatalf("trajectory = %v, want nil", result.RawGetString("trajectory"))
	}
	assertFloat(t, result.RawGetString("observed").Table().RawGetInt(1), 0.25)
	assertFloat(t, result.RawGetString("observed").Table().RawGetInt(2), 0.5)
	assertFloat(t, result.RawGetString("observed").Table().RawGetInt(3), 0.75)
}

func TestODESolveAcceptsIntegrateStyleArguments(t *testing.T) {
	ode := BuildODE(func(fn Value, args []Value) ([]Value, error) {
		state, err := odeVectorFromValue(args[0], "test state")
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(NewDenseArrayF64([]float64{state[0]}))}, nil
	})
	solve := ode.RawGetString("solve").GoFunction()
	got, err := solve.Fn([]Value{
		FunctionValue(&GoFunction{Name: "test.dyn"}),
		DenseArrayValue(NewDenseArrayF64([]float64{1})),
		FloatValue(0.1),
		IntValue(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := got[0].Table()
	final, ok := result.RawGetString("final").DenseArray().F64()
	if !ok || len(final) != 1 {
		t.Fatalf("ode.solve final = %v, want f64[1]", result.RawGetString("final"))
	}
	if math.Abs(final[0]-math.Exp(0.2)) > 2e-5 {
		t.Fatalf("ode.solve final = %.12f, want %.12f", final[0], math.Exp(0.2))
	}
}

func controlTestOptions(values map[string]Value) *Table {
	t := NewTable()
	for key, value := range values {
		t.RawSetString(key, value)
	}
	return t
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
