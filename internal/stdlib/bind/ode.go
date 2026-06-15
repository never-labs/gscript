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

	integrate := func(args []Value) ([]Value, error) {
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
		var project, observe Value
		hasProject := false
		hasObserve := false
		if len(args) >= 5 && args[4].IsTable() {
			opts := args[4].Table()
			trajectory = opts.RawGetString("trajectory").Truthy()
			if method := opts.RawGetString("method"); method.IsString() && method.Str() != "rk4" {
				return nil, fmt.Errorf("ode.integrate: unsupported method %q", method.Str())
			}
			project = opts.RawGetString("project")
			if !project.IsNil() && !project.IsFunction() {
				return nil, fmt.Errorf("ode.integrate: project must be a function")
			}
			hasProject = project.IsFunction()
			observe = opts.RawGetString("observe")
			if !observe.IsNil() && !observe.IsFunction() {
				return nil, fmt.Errorf("ode.integrate: observe must be a function")
			}
			hasObserve = observe.IsFunction()
		}
		if call == nil {
			return nil, fmt.Errorf("ode.integrate: script caller unavailable")
		}
		dt := args[2].Number()
		var states *Table
		if trajectory {
			states = NewAppendArrayTable(steps)
		}
		var observations *Table
		var observationVectors *odeObservationVectors
		if hasObserve {
			observations = NewAppendArrayTable(steps)
		}
		current := append([]float64(nil), state...)
		for i := 0; i < steps; i++ {
			next, err := odeRK4Step(call, fn, current, dt, "ode.integrate")
			if err != nil {
				return nil, err
			}
			current = next
			step := IntValue(int64(i + 1))
			t := FloatValue(float64(i+1) * dt)
			if hasProject {
				projected, err := call(project, []Value{odeStateValue(current), step, t})
				if err != nil {
					return nil, err
				}
				if len(projected) == 0 {
					return nil, fmt.Errorf("ode.integrate: project returned no state")
				}
				current, err = odeVectorFromValue(projected[0], "ode.integrate projected state")
				if err != nil {
					return nil, err
				}
			}
			if trajectory {
				states.RawSetInt(int64(i+1), odeStateValue(current))
			}
			if hasObserve {
				observed, err := call(observe, []Value{odeStateValue(current), step, t})
				if err != nil {
					return nil, err
				}
				if len(observed) == 0 {
					return nil, fmt.Errorf("ode.integrate: observe returned no value")
				}
				if observed[0].IsTable() {
					if observationVectors == nil {
						if i != 0 {
							return nil, fmt.Errorf("ode.integrate: observe returned table at step %d after scalar observations", i+1)
						}
						observationVectors = newODEObservationVectors(observations)
					}
					if err := observationVectors.append(observed[0].Table(), i+1); err != nil {
						return nil, err
					}
				} else {
					if observationVectors != nil {
						return nil, fmt.Errorf("ode.integrate: observe returned scalar at step %d after table observations", i+1)
					}
					observations.RawSetInt(int64(i+1), observed[0])
				}
			}
		}
		final := odeStateValue(current)
		if trajectory && hasObserve {
			return []Value{final, TableValue(states), TableValue(observations)}, nil
		}
		if trajectory {
			return []Value{final, TableValue(states)}, nil
		}
		if hasObserve {
			return []Value{final, TableValue(observations)}, nil
		}
		return []Value{final}, nil
	}
	set("integrate", integrate)

	set("solve", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("ode.solve: need function, state, options")
		}
		fn := args[0]
		state := args[1]
		var dt, steps Value
		var opts Value
		hasOpts := false
		if args[2].IsTable() {
			opts = args[2]
			options := opts.Table()
			dt = options.RawGetString("dt")
			steps = options.RawGetString("steps")
			hasOpts = true
		} else {
			if len(args) < 4 {
				return nil, fmt.Errorf("ode.solve: need function, state, options")
			}
			dt = args[2]
			steps = args[3]
			if len(args) >= 5 {
				if !args[4].IsTable() {
					return nil, fmt.Errorf("ode.solve: options must be a table")
				}
				opts = args[4]
				hasOpts = true
			}
		}
		if !dt.IsNumber() {
			return nil, fmt.Errorf("ode.solve: dt must be numeric")
		}
		if _, err := linalgPositiveInt("ode.solve", steps, "steps"); err != nil {
			return nil, err
		}
		integrateArgs := []Value{fn, state, dt, steps}
		trajectory := false
		hasObserve := false
		if hasOpts {
			options := opts.Table()
			trajectory = options.RawGetString("trajectory").Truthy()
			hasObserve = options.RawGetString("observe").IsFunction()
			integrateArgs = append(integrateArgs, opts)
		}
		values, err := integrate(integrateArgs)
		if err != nil {
			return nil, err
		}
		out := NewTable()
		out.RawSetString("final", values[0])
		next := 1
		if trajectory {
			out.RawSetString("trajectory", values[next])
			next++
		}
		if hasObserve {
			out.RawSetString("observed", values[next])
		}
		return []Value{TableValue(out)}, nil
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

func odeStateValue(state []float64) Value {
	return DenseArrayValue(NewDenseArrayF64Owned(append([]float64(nil), state...)))
}

type odeObservationVectors struct {
	out   *Table
	keys  []string
	cols  map[string]*Table
	kinds map[string]string
}

func newODEObservationVectors(out *Table) *odeObservationVectors {
	return &odeObservationVectors{
		out:   out,
		cols:  map[string]*Table{},
		kinds: map[string]string{},
	}
}

func (o *odeObservationVectors) append(observed *Table, step int) error {
	if step == 1 {
		for _, key := range observed.PairsKeysSnapshot() {
			if !key.IsString() {
				return fmt.Errorf("ode.integrate: observe table key at step %d must be a string, got %s", step, key.TypeName())
			}
			name := key.Str()
			value := observed.RawGetString(name)
			if value.IsNil() {
				continue
			}
			col := NewAppendArrayTable(0)
			col.RawSetInt(int64(step), value)
			o.keys = append(o.keys, name)
			o.cols[name] = col
			o.kinds[name] = odeObservationValueKind(value)
			o.out.RawSetString(name, TableValue(col))
		}
		if len(o.keys) == 0 {
			return fmt.Errorf("ode.integrate: observe table at step %d has no string fields", step)
		}
		return nil
	}
	seen := make(map[string]bool, len(o.keys))
	for _, key := range observed.PairsKeysSnapshot() {
		if !key.IsString() {
			return fmt.Errorf("ode.integrate: observe table key at step %d must be a string, got %s", step, key.TypeName())
		}
		name := key.Str()
		value := observed.RawGetString(name)
		if value.IsNil() {
			continue
		}
		col := o.cols[name]
		if col == nil {
			return fmt.Errorf("ode.integrate: observe table at step %d added field %q not present at step 1", step, name)
		}
		kind := odeObservationValueKind(value)
		if kind != o.kinds[name] {
			return fmt.Errorf("ode.integrate: observe table field %q changed type at step %d: got %s, want %s", name, step, kind, o.kinds[name])
		}
		col.RawSetInt(int64(step), value)
		seen[name] = true
	}
	for _, name := range o.keys {
		if !seen[name] {
			return fmt.Errorf("ode.integrate: observe table at step %d missing field %q from step 1", step, name)
		}
	}
	return nil
}

func odeObservationValueKind(value Value) string {
	if value.IsNumber() {
		return "number"
	}
	return value.TypeName()
}

func odeAddScaled(a, b []float64, scale float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] + scale*b[i]
	}
	return out
}
