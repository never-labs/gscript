package bind

import (
	"fmt"
	"math"
)

type stateMeta struct {
	names     []string
	wrap      []int
	named     bool
	hasNamed  bool
	sourceKey string
}

func stateMetaFromOptions(name string, opts *Table, stateDim int, dimLabel string) (stateMeta, error) {
	if opts == nil {
		return stateMeta{}, nil
	}
	source := opts
	sourceKey := ""
	for _, key := range []string{"state", "state_descriptor", "stateDescriptor"} {
		value := opts.RawGetString(key)
		if value.IsNil() {
			continue
		}
		if !value.IsTable() {
			return stateMeta{}, fmt.Errorf("%s: %s must be a table", name, key)
		}
		source = value.Table()
		sourceKey = key
		break
	}
	names, err := stateNamesFromTable(name, source, stateDim, dimLabel)
	if err != nil {
		return stateMeta{}, err
	}
	wrap, err := stateWrapIndexesFromTable(name, source, stateDim, names)
	if err != nil {
		return stateMeta{}, err
	}
	named := stateBoolOption(source, "named_state", "namedState")
	return stateMeta{
		names:     names,
		wrap:      wrap,
		named:     named,
		hasNamed:  named || len(names) > 0,
		sourceKey: sourceKey,
	}, nil
}

func stateNamesFromTable(name string, opts *Table, stateDim int, dimLabel string) ([]string, error) {
	value := opts.RawGetString("state_names")
	if value.IsNil() {
		value = opts.RawGetString("stateNames")
	}
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
	if t.Length() != stateDim {
		return nil, fmt.Errorf("%s: state_names length %d does not match %s %d", name, t.Length(), dimLabel, stateDim)
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

func stateWrapIndexesFromTable(name string, opts *Table, stateDim int, names []string) ([]int, error) {
	value := opts.RawGetString("wrap_angles")
	if value.IsNil() {
		value = opts.RawGetString("wrapAngles")
	}
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
	for i, stateName := range names {
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
			if n > stateDim {
				return nil, fmt.Errorf("%s: wrap_angles[%d] index %d out of range for state length %d", name, i+1, n, stateDim)
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

func stateBoolOption(opts *Table, names ...string) bool {
	for _, name := range names {
		if opts.RawGetString(name).Truthy() {
			return true
		}
	}
	return false
}

func stateVectorFromValue(name string, value Value, names []string) ([]float64, error) {
	if value.IsTable() && len(names) > 0 && !value.Table().RawGetString(names[0]).IsNil() {
		return namedStateVectorFromTable(name, value.Table(), names)
	}
	values, err := linalgVectorValue(name, value)
	if err != nil {
		return nil, err
	}
	if len(names) > 0 && len(values) != len(names) {
		return nil, fmt.Errorf("%s: state length %d does not match state_names length %d", name, len(values), len(names))
	}
	return values, nil
}

func namedStateVectorFromTable(name string, state *Table, names []string) ([]float64, error) {
	out := make([]float64, len(names))
	for i, stateName := range names {
		value := state.RawGetString(stateName)
		if value.IsNil() {
			return nil, fmt.Errorf("%s: state missing field %q", name, stateName)
		}
		x, err := linalgNumber(name, value)
		if err != nil {
			return nil, fmt.Errorf("%s: state field %q must be numeric", name, stateName)
		}
		out[i] = x
	}
	return out, nil
}

func wrapStateIndexes(state []float64, indexes []int) {
	for _, idx := range indexes {
		state[idx] = math.Atan2(math.Sin(state[idx]), math.Cos(state[idx]))
	}
}

func namedStateTable(names []string, state []float64) *Table {
	t := NewTable()
	for i, name := range names {
		t.RawSetString(name, FloatValue(state[i]))
	}
	return t
}
