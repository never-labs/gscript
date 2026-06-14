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

		eval := func(s []float64) ([]float64, error) {
			ret, err := call(fn, []Value{DenseArrayValue(NewDenseArrayF64(s))})
			if err != nil {
				return nil, err
			}
			if len(ret) == 0 {
				return nil, fmt.Errorf("ode.rk4: dynamics returned no state derivative")
			}
			out, err := odeVectorFromValue(ret[0], "ode.rk4 derivative")
			if err != nil {
				return nil, err
			}
			if len(out) != len(state) {
				return nil, fmt.Errorf("ode.rk4: derivative length %d does not match state length %d", len(out), len(state))
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
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(next))}, nil
	})

	return t
}

func odeVectorFromValue(v Value, name string) ([]float64, error) {
	if v.IsDenseArray() {
		arr := v.DenseArray()
		if xs, ok := arr.F64(); ok {
			return append([]float64(nil), xs...), nil
		}
		if xs, ok := arr.I64(); ok {
			out := make([]float64, len(xs))
			for i, x := range xs {
				out[i] = float64(x)
			}
			return out, nil
		}
		return nil, fmt.Errorf("%s must be a numeric dense array", name)
	}
	if v.IsTable() {
		t := v.Table()
		out := make([]float64, t.Length())
		for i := range out {
			x := t.RawGetInt(int64(i + 1))
			if !x.IsNumber() {
				return nil, fmt.Errorf("%s[%d] must be numeric", name, i+1)
			}
			out[i] = x.Number()
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s must be a dense array or numeric table", name)
}

func odeAddScaled(a, b []float64, scale float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] + scale*b[i]
	}
	return out
}
