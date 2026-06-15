package bind

import "fmt"

// BuildODE creates the "ode" standard library table.
func BuildODE(call ScriptFunctionCaller) *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "ode." + name, Fn: fn}))
	}

	set("rk4", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("ode.rk4: need function, state, dt")
		}
		if call == nil {
			return nil, fmt.Errorf("ode.rk4: script caller unavailable")
		}
		fn := args[0]
		if !fn.IsFunction() {
			return nil, fmt.Errorf("ode.rk4: argument 1 must be a function")
		}
		state, err := odeVectorFromValue(args[1], "ode.rk4 state")
		if err != nil {
			return nil, err
		}
		if !args[2].IsNumber() {
			return nil, fmt.Errorf("ode.rk4: dt must be numeric")
		}
		dt := args[2].Number()
		next, err := odeRK4Step(call, fn, state, dt, "ode.rk4")
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(next))}, nil
	})

	set("integrate", func(args []Value) ([]Value, error) {
		if len(args) < 4 {
			return nil, fmt.Errorf("ode.integrate: need function, state, dt, steps")
		}
		fn := args[0]
		if !fn.IsFunction() {
			return nil, fmt.Errorf("ode.integrate: argument 1 must be a function")
		}
		state, err := odeVectorFromValue(args[1], "ode.integrate state")
		if err != nil {
			return nil, err
		}
		if !args[2].IsNumber() {
			return nil, fmt.Errorf("ode.integrate: dt must be numeric")
		}
		steps, err := linalgPositiveInt("ode.integrate", args[3], "steps")
		if err != nil {
			return nil, err
		}
		trajectory := false
		if len(args) >= 5 && args[4].IsTable() {
			trajectory = args[4].Table().RawGetString("trajectory").Truthy()
		}
		if call == nil {
			return nil, fmt.Errorf("ode.integrate: script caller unavailable")
		}
		dt := args[2].Number()
		var states *Table
		if trajectory {
			states = NewAppendArrayTable(steps)
		}
		current := append([]float64(nil), state...)
		for i := 0; i < steps; i++ {
			next, err := odeRK4Step(call, fn, current, dt, "ode.integrate")
			if err != nil {
				return nil, err
			}
			current = next
			if trajectory {
				states.RawSetInt(int64(i+1), DenseArrayValue(NewDenseArrayF64Owned(append([]float64(nil), current...))))
			}
		}
		final := DenseArrayValue(NewDenseArrayF64Owned(current))
		if trajectory {
			return []Value{final, TableValue(states)}, nil
		}
		return []Value{final}, nil
	})

	return t
}

func odeRK4Step(call ScriptFunctionCaller, fn Value, state []float64, dt float64, name string) ([]float64, error) {
	eval := func(s []float64) ([]float64, error) {
		ret, err := call(fn, []Value{DenseArrayValue(NewDenseArrayF64(s))})
		if err != nil {
			return nil, err
		}
		if len(ret) == 0 {
			return nil, fmt.Errorf("%s: dynamics returned no state derivative", name)
		}
		out, err := odeVectorFromValue(ret[0], name+" derivative")
		if err != nil {
			return nil, err
		}
		if len(out) != len(state) {
			return nil, fmt.Errorf("%s: derivative length %d does not match state length %d", name, len(out), len(state))
		}
		return out, nil
	}
	k1, err := eval(state)
	if err != nil {
		return nil, err
	}
	k2, err := eval(odeAddScaled(state, k1, 0.5*dt))
	if err != nil {
		return nil, err
	}
	k3, err := eval(odeAddScaled(state, k2, 0.5*dt))
	if err != nil {
		return nil, err
	}
	k4, err := eval(odeAddScaled(state, k3, dt))
	if err != nil {
		return nil, err
	}
	next := make([]float64, len(state))
	for i := range state {
		next[i] = state[i] + (dt/6)*(k1[i]+2*k2[i]+2*k3[i]+k4[i])
	}
	return next, nil
}

func odeVectorFromValue(v Value, name string) ([]float64, error) {
	return linalgVectorValue(name, v)
}

func odeAddScaled(a, b []float64, scale float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] + scale*b[i]
	}
	return out
}
