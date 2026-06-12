package q

// Canonical kdb+/q dictionary operations and table row indexing.
//
// Dict arithmetic (applyDictDyadic and friends) implements the canonical
// key-union semantics for atomic dyads over dictionaries: keys present on
// both sides combine through the verb, keys present on one side keep that
// side's value (equivalently: the absent side fills with the verb's
// identity for + - * %). Dict-vs-atom broadcasts the atom over the value
// list and dict-vs-vector pairs positionally; both reuse the vector dyadic
// kernels by routing the value list through applyDyadic as one array.
//
// Dict take (`a`b#d, 2#d), dict drop (`a _ d, 1 _ d), where-on-dict, and
// aggregate/unary mapping over dict values live here too, as do the table
// row-indexing helpers (t[0] row dict, t[0 2] sub-table) shared by every
// eval route through indexValue.

import (
	"fmt"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// dictValuesArray materializes a dict's value list as an array for the
// value-side vector kernels. The values are copied so kernel results can
// never alias the source dict (q copy-on-write contract).
func dictValuesArray(d EvalDict) data.Array {
	values := make([]any, len(d.Values))
	copy(values, d.Values)
	return data.InferArray(values)
}

// applyDictDyadic claims dyadic applications with at least one dictionary
// operand. `=` (union compare), `~` (match), and `,` (join) keep their
// dedicated handlers; `L`/`R` (left/right) keep the whole-value scalar
// branches; table operands keep their existing dispatch errors.
func applyDictDyadic(op byte, left, right any) (any, bool, error) {
	leftDict, leftIsDict := left.(EvalDict)
	rightDict, rightIsDict := right.(EvalDict)
	if !leftIsDict && !rightIsDict {
		return nil, false, nil
	}
	switch op {
	case '=', '~', ',', 'L', 'R':
		return nil, false, nil
	}
	if leftIsDict && rightIsDict {
		out, err := dictDyadicUnion(op, leftDict, rightDict)
		return out, true, err
	}
	if leftIsDict {
		if !qDictBroadcastOperand(right) {
			return nil, false, nil
		}
		out, err := dictDyadicValueSide(op, leftDict, right, false)
		return out, true, err
	}
	if !qDictBroadcastOperand(left) {
		return nil, false, nil
	}
	out, err := dictDyadicValueSide(op, rightDict, left, true)
	return out, true, err
}

// qDictBroadcastOperand reports whether the non-dict operand may broadcast
// over a dict's values. Tables and callables keep their existing errors.
func qDictBroadcastOperand(v any) bool {
	switch v.(type) {
	case data.Frame, data.KeyedFrame:
		return false
	default:
		return !isCallable(v)
	}
}

// qDictDyadicIdentity is the absent-side fill for key-union arithmetic:
// 0 for +/- and 1 for */%, matching canonical q (the right-only key of
// d1-d2 surfaces negated, the right-only key of d1%d2 as a reciprocal).
func qDictDyadicIdentity(op byte) (any, bool) {
	switch op {
	case '+', '-':
		return int64(0), true
	case '*', '%':
		return int64(1), true
	default:
		return nil, false
	}
}

// qDictDyadicKeepPresent lists union verbs whose single-side keys keep the
// present value unchanged: min/max (& | m M, identity is the extreme) and
// fill (^, union-fill prefers the present entry).
func qDictDyadicKeepPresent(op byte) bool {
	switch op {
	case '&', '|', 'm', 'M', '^':
		return true
	default:
		return false
	}
}

// dictDyadicUnion applies op over the key union of two dicts: left key
// order first, then unseen right keys (the same order dictEqualsDict uses).
func dictDyadicUnion(op byte, left, right EvalDict) (EvalDict, error) {
	fill, hasFill := qDictDyadicIdentity(op)
	if !hasFill && !qDictDyadicKeepPresent(op) {
		return EvalDict{}, fmt.Errorf("operator %q is not supported between dictionaries", string(op))
	}
	keys := make([]any, 0, len(left.Keys)+len(right.Keys))
	values := make([]any, 0, len(left.Keys)+len(right.Keys))
	for i, key := range left.Keys {
		keys = append(keys, key)
		if j, found := dictKeyIndex(right, key); found {
			v, err := applyDyadic(op, left.Values[i], right.Values[j])
			if err != nil {
				return EvalDict{}, err
			}
			values = append(values, v)
			continue
		}
		if hasFill {
			v, err := applyDyadic(op, left.Values[i], fill)
			if err != nil {
				return EvalDict{}, err
			}
			values = append(values, v)
			continue
		}
		values = append(values, left.Values[i])
	}
	for j, key := range right.Keys {
		if _, found := dictKeyIndex(left, key); found {
			continue
		}
		keys = append(keys, key)
		if hasFill {
			v, err := applyDyadic(op, fill, right.Values[j])
			if err != nil {
				return EvalDict{}, err
			}
			values = append(values, v)
			continue
		}
		values = append(values, right.Values[j])
	}
	return EvalDict{Keys: keys, Values: values}, nil
}

// dictDyadicValueSide broadcasts an atom (or pairs a vector positionally)
// over a dict's values through one vector applyDyadic call, so the typed
// vector kernels do the value-side work.
func dictDyadicValueSide(op byte, d EvalDict, other any, dictOnRight bool) (EvalDict, error) {
	values := dictValuesArray(d)
	var out any
	var err error
	if dictOnRight {
		out, err = applyDyadic(op, other, values)
	} else {
		out, err = applyDyadic(op, values, other)
	}
	if err != nil {
		return EvalDict{}, err
	}
	array, ok := out.(data.Array)
	if !ok || array.Len() != len(d.Keys) {
		return EvalDict{}, fmt.Errorf("operator %q over dictionary values produced %T", string(op), out)
	}
	return EvalDict{Keys: append([]any(nil), d.Keys...), Values: array.Values()}, nil
}

// dictAggregateArgument unwraps a dict argument to its value list so
// aggregates (sum, avg, min, max, prd, med, all, any) reduce over the
// values, matching canonical q.
func dictAggregateArgument(v any) any {
	if d, ok := v.(EvalDict); ok {
		return dictValuesArray(d)
	}
	return v
}

// dictWhere implements canonical `where dict`: keys replicated by their
// counts (booleans count 0/1), e.g. where `a`b!2 1 -> `a`a`b.
func dictWhere(d EvalDict) (any, error) {
	var out []any
	total := int64(0)
	for i, value := range d.Values {
		var n int64
		switch x := value.(type) {
		case bool:
			if x {
				n = 1
			}
		default:
			if data.IsNull(value) {
				continue
			}
			v, ok := integerValue(value)
			if !ok {
				return nil, fmt.Errorf("where expects a bool or integer vector")
			}
			n = v
		}
		if n < 0 {
			return nil, fmt.Errorf("where expects non-negative integer counts")
		}
		if n > qMaxListLength-total {
			return nil, fmt.Errorf("where count %d exceeds the %d list limit", n, int64(qMaxListLength))
		}
		total += n
		for j := int64(0); j < n; j++ {
			out = append(out, d.Keys[i])
		}
	}
	return data.InferArray(out), nil
}

// dictTakeCount implements `n#dict`: the first n (last -n) entries, cycling
// past the end exactly like vector take (canonical q cycles entries too).
func dictTakeCount(d EvalDict, n int) (EvalDict, error) {
	indexes := qTakeIndexes(len(d.Keys), n)
	keys := make([]any, len(indexes))
	values := make([]any, len(indexes))
	for i, index := range indexes {
		keys[i] = d.Keys[index]
		values[i] = d.Values[index]
	}
	return EvalDict{Keys: keys, Values: values}, nil
}

// dictTakeKeys implements `keys#dict`: the sub-dict in key-list order,
// filling absent keys with null exactly like d[key] lookup.
func dictTakeKeys(keys []any, d EvalDict) (EvalDict, error) {
	outKeys := make([]any, len(keys))
	outValues := make([]any, len(keys))
	for i, key := range keys {
		outKeys[i] = key
		value, err := dictLookup(d, key)
		if err != nil {
			return EvalDict{}, err
		}
		outValues[i] = value
	}
	return EvalDict{Keys: outKeys, Values: outValues}, nil
}

// takeOrReshapeValue is the shared `left#right` dispatch of the string and
// compiled routes: dict take (key list or symbol atom left), array-left
// reshape, then integer take.
func takeOrReshapeValue(left, right any) (any, error) {
	if d, ok := right.(EvalDict); ok {
		switch x := left.(type) {
		case data.Array:
			return dictTakeKeys(x.Values(), d)
		case data.Symbol:
			return dictTakeKeys([]any{x}, d)
		}
	}
	if _, ok := left.(data.Array); ok {
		return reshapeValue(left, right)
	}
	n, ok := integerValue(left)
	if !ok || int64(int(n)) != n {
		return nil, fmt.Errorf("# left operand must be an integer count")
	}
	return take(int(n), right)
}

// dictDropCount implements `n _ dict`: drop the first n (last -n) entries.
func dictDropCount(d EvalDict, n int) EvalDict {
	indexes := dropIndexes(len(d.Keys), n)
	keys := make([]any, len(indexes))
	values := make([]any, len(indexes))
	for i, index := range indexes {
		keys[i] = d.Keys[index]
		values[i] = d.Values[index]
	}
	return EvalDict{Keys: keys, Values: values}
}

// qDictDropKeyOperand extracts an unambiguous key-drop operand: a symbol
// atom or symbol vector. Integer operands keep entry-count drop semantics
// (canonical q needs enlist to drop an integer key).
func qDictDropKeyOperand(v any) ([]any, bool) {
	switch x := v.(type) {
	case data.Symbol:
		return []any{x}, true
	case data.Array:
		if x.Kind() != data.KindSymbol {
			return nil, false
		}
		return x.Values(), true
	default:
		return nil, false
	}
}

// dictDropKeys removes the listed keys from the dict, keeping order.
func dictDropKeys(d EvalDict, keys []any) EvalDict {
	outKeys := make([]any, 0, len(d.Keys))
	outValues := make([]any, 0, len(d.Values))
	for i, key := range d.Keys {
		dropped := false
		for _, candidate := range keys {
			if equalValue(key, candidate) {
				dropped = true
				break
			}
		}
		if dropped {
			continue
		}
		outKeys = append(outKeys, key)
		outValues = append(outValues, d.Values[i])
	}
	return EvalDict{Keys: outKeys, Values: outValues}
}

// frameRowIndexValue claims integer indexing of an unkeyed table: t[0] is
// the row dictionary, t[0 2] the gathered sub-table. Symbol indexes fall
// through to column lookup.
func frameRowIndexValue(frame data.Frame, index any) (any, bool, error) {
	switch x := index.(type) {
	case int64:
		out, err := frameRowDict(frame, x)
		return out, true, err
	case data.Array:
		if x.Kind() != data.KindI64 {
			return nil, false, nil
		}
		indexes, _, err := indexInts(x)
		if err != nil {
			return nil, true, err
		}
		for _, row := range indexes {
			if row < 0 || row >= frame.Len() {
				return nil, true, fmt.Errorf("index %d out of range", row)
			}
		}
		out, err := qGatherFrameRuntime("index", frame, indexes)
		return out, true, err
	default:
		return nil, false, nil
	}
}

// frameRowDict builds the ordered column-name!cell dictionary for one row.
func frameRowDict(frame data.Frame, row int64) (EvalDict, error) {
	if row < 0 || row >= int64(frame.Len()) {
		return EvalDict{}, fmt.Errorf("index %d out of range", row)
	}
	names := frame.Schema().Names()
	keys := make([]any, len(names))
	values := make([]any, len(names))
	for i, name := range names {
		col, ok := frame.Column(name)
		if !ok {
			return EvalDict{}, fmt.Errorf("table column %q not found", name)
		}
		cell, ok := col.At(int(row))
		if !ok {
			return EvalDict{}, fmt.Errorf("index %d out of range", row)
		}
		keys[i] = name
		values[i] = cell
	}
	return EvalDict{Keys: keys, Values: values}, nil
}

// frameColumnAssignValue claims symbol-keyed table amend: t[`col]:vec
// assigns or appends a column (atoms broadcast over the row count).
func frameColumnAssignValue(frame data.Frame, key any, value any) (data.Frame, bool, error) {
	sym, ok := key.(data.Symbol)
	if !ok {
		return data.Frame{}, false, nil
	}
	column, err := frameAssignColumnData(frame, sym, value)
	if err != nil {
		return data.Frame{}, true, err
	}
	schema := frame.Schema()
	cols := make([]data.Column, 0, len(schema.Names())+1)
	replaced := false
	for _, name := range schema.Names() {
		if name == sym {
			cols = append(cols, data.Column{Name: name, Data: column})
			replaced = true
			continue
		}
		existing, ok := frame.Column(name)
		if !ok {
			return data.Frame{}, true, fmt.Errorf("table column %q not found", name)
		}
		cols = append(cols, data.Column{Name: name, Data: existing})
	}
	if !replaced {
		cols = append(cols, data.Column{Name: sym, Data: column})
	}
	out, err := data.NewFrame(cols...)
	return out, true, err
}

func frameAssignColumnData(frame data.Frame, name data.Symbol, value any) (data.Array, error) {
	if array, ok := value.(data.Array); ok {
		if array.Len() != frame.Len() {
			return nil, fmt.Errorf("table column %q assignment length %d does not match row count %d", name, array.Len(), frame.Len())
		}
		return array, nil
	}
	values := make([]any, frame.Len())
	for i := range values {
		values[i] = value
	}
	return data.NewColumn(name, values).Data, nil
}
