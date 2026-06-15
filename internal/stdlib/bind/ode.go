package bind

import (
	"fmt"
	"math"
)

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
		next, err := odeRK4Step(call, fn, state, dt, "ode.rk4", nil, nil)
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
		var stateNames []string
		var wrapAngles []int
		namedStateInput := false
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
			stateNames, err = odeStateNamesFromOptions("ode.integrate", opts, len(state))
			if err != nil {
				return nil, err
			}
			wrapAngles, err = odeWrapAnglesFromOptions("ode.integrate", opts, len(state), stateNames)
			if err != nil {
				return nil, err
			}
			namedStateInput = opts.RawGetString("named_state").Truthy()
			if !namedStateInput {
				namedStateInput = opts.RawGetString("namedState").Truthy()
			}
			if namedStateInput && len(stateNames) == 0 {
				return nil, fmt.Errorf("ode.integrate: named_state requires state_names")
			}
		}
		hasNamedState := len(stateNames) > 0
		stateArg := odeStateValue
		if namedStateInput {
			stateArg = func(s []float64) Value {
				return TableValue(odeNamedStateTable(stateNames, s))
			}
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
		if hasObserve || hasNamedState {
			observations = NewAppendArrayTable(steps)
		}
		current := append([]float64(nil), state...)
		for i := 0; i < steps; i++ {
			next, err := odeRK4Step(call, fn, current, dt, "ode.integrate", stateArg, stateNames)
			if err != nil {
				return nil, err
			}
			current = next
			step := IntValue(int64(i + 1))
			t := FloatValue(float64(i+1) * dt)
			if hasProject {
				projected, err := call(project, []Value{stateArg(current), step, t})
				if err != nil {
					return nil, err
				}
				if len(projected) == 0 {
					return nil, fmt.Errorf("ode.integrate: project returned no state")
				}
				current, err = odeVectorFromValueNamed(projected[0], "ode.integrate projected state", stateNames)
				if err != nil {
					return nil, err
				}
			}
			odeWrapAngleIndexes(current, wrapAngles)
			if trajectory {
				states.RawSetInt(int64(i+1), odeStateValue(current))
			}
			if hasObserve || hasNamedState {
				var observedValue Value
				if hasObserve {
					observed, err := call(observe, []Value{stateArg(current), step, t})
					if err != nil {
						return nil, err
					}
					if len(observed) == 0 {
						return nil, fmt.Errorf("ode.integrate: observe returned no value")
					}
					observedValue = observed[0]
				}
				if hasNamedState {
					row := odeNamedStateTable(stateNames, current)
					if hasObserve {
						if observedValue.IsTable() {
							if err := odeMergeObservationTable(row, observedValue.Table(), i+1); err != nil {
								return nil, err
							}
						} else {
							row.RawSetString("value", observedValue)
						}
					}
					observedValue = TableValue(row)
				}
				if observedValue.IsTable() {
					if observationVectors == nil {
						if i != 0 {
							return nil, fmt.Errorf("ode.integrate: observe returned table at step %d after scalar observations", i+1)
						}
						observationVectors = newODEObservationVectors(observations)
					}
					if err := observationVectors.append(observedValue.Table(), i+1); err != nil {
						return nil, err
					}
				} else {
					if observationVectors != nil {
						return nil, fmt.Errorf("ode.integrate: observe returned scalar at step %d after table observations", i+1)
					}
					observations.RawSetInt(int64(i+1), observedValue)
				}
			}
		}
		final := odeStateValue(current)
		hasObservations := hasObserve || hasNamedState
		if trajectory && hasObservations {
			return []Value{final, TableValue(states), TableValue(observations)}, nil
		}
		if trajectory {
			return []Value{final, TableValue(states)}, nil
		}
		if hasObservations {
			return []Value{final, TableValue(observations)}, nil
		}
		return []Value{final}, nil
	}
	set("integrate", integrate)

	solve := func(args []Value) ([]Value, error) {
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
		stateValues, err := odeVectorFromValue(state, "ode.solve state")
		if err != nil {
			return nil, err
		}
		if _, err := linalgPositiveInt("ode.solve", steps, "steps"); err != nil {
			return nil, err
		}
		integrateArgs := []Value{fn, state, dt, steps}
		trajectory := false
		hasObserve := false
		hasNamedState := false
		var stateNames []string
		if hasOpts {
			options := opts.Table()
			trajectory = options.RawGetString("trajectory").Truthy()
			hasObserve = options.RawGetString("observe").IsFunction()
			stateNames, err = odeStateNamesFromOptions("ode.solve", options, len(stateValues))
			if err != nil {
				return nil, err
			}
			hasNamedState = len(stateNames) > 0
			integrateArgs = append(integrateArgs, opts)
		}
		values, err := integrate(integrateArgs)
		if err != nil {
			return nil, err
		}
		out := NewTable()
		out.RawSetString("final", values[0])
		if hasNamedState {
			finalValues, err := odeVectorFromValue(values[0], "ode.solve final")
			if err != nil {
				return nil, err
			}
			out.RawSetString("final_state", TableValue(odeNamedStateTable(stateNames, finalValues)))
		}
		next := 1
		if trajectory {
			out.RawSetString("trajectory", values[next])
			next++
		}
		if hasObserve || hasNamedState {
			out.RawSetString("observed", values[next])
		}
		return []Value{TableValue(out)}, nil
	}
	set("solve", solve)

	return t
}

func odeCallFunction(call ScriptFunctionCaller, fn Value, args []Value, name string) ([]Value, error) {
	if gf := fn.GoFunction(); gf != nil && gf.Fn != nil {
		return gf.Fn(args)
	}
	if call == nil {
		return nil, fmt.Errorf("%s: script caller unavailable", name)
	}
	return call(fn, args)
}

func odeRK4Step(call ScriptFunctionCaller, fn Value, state []float64, dt float64, name string, stateArg func([]float64) Value, stateNames []string) ([]float64, error) {
	if stateArg == nil {
		stateArg = odeStateValue
	}
	eval := func(s []float64) ([]float64, error) {
		ret, err := odeCallFunction(call, fn, []Value{stateArg(s)}, name)
		if err != nil {
			return nil, err
		}
		if len(ret) == 0 {
			return nil, fmt.Errorf("%s: dynamics returned no state derivative", name)
		}
		out, err := odeVectorFromValueNamed(ret[0], name+" derivative", stateNames)
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

func odeVectorFromValueNamed(v Value, name string, stateNames []string) ([]float64, error) {
	if len(stateNames) > 0 && v.IsTable() && !v.Table().RawGetString(stateNames[0]).IsNil() {
		return odeNamedVectorFromTable(v.Table(), name, stateNames)
	}
	values, err := odeVectorFromValue(v, name)
	if err == nil {
		return values, nil
	}
	if len(stateNames) == 0 || !v.IsTable() {
		return nil, err
	}
	return odeNamedVectorFromTable(v.Table(), name, stateNames)
}

func odeNamedVectorFromTable(t *Table, name string, stateNames []string) ([]float64, error) {
	out := make([]float64, len(stateNames))
	for i, stateName := range stateNames {
		value := t.RawGetString(stateName)
		if value.IsNil() {
			return nil, fmt.Errorf("%s: missing field %q", name, stateName)
		}
		number, numberErr := linalgNumber(name, value)
		if numberErr != nil {
			return nil, fmt.Errorf("%s: field %q must be numeric", name, stateName)
		}
		out[i] = number
	}
	return out, nil
}

func odeStateValue(state []float64) Value {
	return DenseArrayValue(NewDenseArrayF64Owned(append([]float64(nil), state...)))
}

func odeStateNamesFromOptions(name string, opts *Table, stateLen int) ([]string, error) {
	value := opts.RawGetString("state_names")
	if value.IsNil() {
		value = opts.RawGetString("names")
	}
	if value.IsNil() {
		return nil, nil
	}
	if !value.IsTable() {
		return nil, fmt.Errorf("%s: state_names must be a string table", name)
	}
	t := value.Table()
	if t.Length() != stateLen {
		return nil, fmt.Errorf("%s: state_names length %d does not match state length %d", name, t.Length(), stateLen)
	}
	seen := make(map[string]bool, t.Length())
	names := make([]string, t.Length())
	for i := range names {
		item := t.RawGetInt(int64(i + 1))
		if !item.IsString() {
			return nil, fmt.Errorf("%s: state_names[%d] must be a string, got %s", name, i+1, item.TypeName())
		}
		field := item.Str()
		if field == "" {
			return nil, fmt.Errorf("%s: state_names[%d] must not be empty", name, i+1)
		}
		if seen[field] {
			return nil, fmt.Errorf("%s: duplicate state name %q", name, field)
		}
		seen[field] = true
		names[i] = field
	}
	return names, nil
}

func odeWrapAnglesFromOptions(name string, opts *Table, stateLen int, stateNames []string) ([]int, error) {
	value := opts.RawGetString("wrap_angles")
	if value.IsNil() {
		value = opts.RawGetString("wrap")
	}
	if value.IsNil() {
		return nil, nil
	}
	var items []Value
	if value.IsTable() {
		t := value.Table()
		items = make([]Value, t.Length())
		for i := range items {
			items[i] = t.RawGetInt(int64(i + 1))
		}
	} else {
		items = []Value{value}
	}
	nameToIndex := map[string]int{}
	for i, stateName := range stateNames {
		nameToIndex[stateName] = i
	}
	seen := make(map[int]bool, len(items))
	out := make([]int, 0, len(items))
	for i, item := range items {
		var idx int
		switch {
		case item.IsString():
			if len(nameToIndex) == 0 {
				return nil, fmt.Errorf("%s: wrap_angles[%d] uses name %q without state_names", name, i+1, item.Str())
			}
			var ok bool
			idx, ok = nameToIndex[item.Str()]
			if !ok {
				return nil, fmt.Errorf("%s: wrap_angles[%d] unknown state name %q", name, i+1, item.Str())
			}
		case item.IsNumber():
			n, err := linalgPositiveInt(name, item, fmt.Sprintf("wrap_angles[%d]", i+1))
			if err != nil {
				return nil, err
			}
			if n > stateLen {
				return nil, fmt.Errorf("%s: wrap_angles[%d] index %d out of range for state length %d", name, i+1, n, stateLen)
			}
			idx = n - 1
		default:
			return nil, fmt.Errorf("%s: wrap_angles[%d] must be a state name or 1-based index, got %s", name, i+1, item.TypeName())
		}
		if !seen[idx] {
			seen[idx] = true
			out = append(out, idx)
		}
	}
	return out, nil
}

func odeWrapAngleIndexes(state []float64, indexes []int) {
	for _, idx := range indexes {
		state[idx] = math.Atan2(math.Sin(state[idx]), math.Cos(state[idx]))
	}
}

func odeNamedStateTable(names []string, state []float64) *Table {
	t := NewTable()
	for i, name := range names {
		t.RawSetString(name, FloatValue(state[i]))
	}
	return t
}

func odeMergeObservationTable(dst, src *Table, step int) error {
	for _, key := range src.PairsKeysSnapshot() {
		if !key.IsString() {
			return fmt.Errorf("ode.integrate: observe table key at step %d must be a string, got %s", step, key.TypeName())
		}
		name := key.Str()
		if !dst.RawGetString(name).IsNil() {
			return fmt.Errorf("ode.integrate: observe table at step %d field %q conflicts with state_names", step, name)
		}
		dst.RawSetString(name, src.RawGetString(name))
	}
	return nil
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
