package data

import (
	"fmt"
)

// Typed copy-on-write mutation kernels.
//
// Frames and keyed frames are immutable values, so a mutation only needs to
// rebuild the columns it actually touches. The helpers in this file produce
// dense typed copies with point edits (scatter assignments and tail appends)
// without round-tripping every row through boxed []any storage, and let the
// keyed-frame mutation paths reuse the existing rowsByKey index instead of
// re-keying the whole table per operation. Every fast path is optimistic:
// ok=false means the caller must run the legacy boxed implementation, which
// remains the semantic ground truth (including its error messages).

// arrayWithTypedEdits returns a dense copy of arr with setValues[i] assigned
// at row setRows[i] (applied in order; later edits win) and appendValues
// appended at the tail. ok=false means the carrier/kind/value combination is
// not supported and the caller must use the boxed fallback.
func arrayWithTypedEdits(arr Array, setRows []int, setValues []any, appendValues []any) (Array, bool) {
	if len(setRows) != len(setValues) {
		return nil, false
	}
	if a, ok := arr.(attributedArray); ok {
		// Point edits invalidate attribute facts (sorted/unique/grouped), so
		// edit the underlying storage and drop the metadata wrapper.
		return arrayWithTypedEdits(a.array, setRows, setValues, appendValues)
	}
	if anyNullEditValue(setValues) || anyNullEditValue(appendValues) {
		return arrayWithNullableTypedEdits(arr, setRows, setValues, appendValues)
	}
	switch a := arr.(type) {
	case columnArray[bool]:
		return editColumnArray(a.kind, a.data, convertEditBool, setRows, setValues, appendValues)
	case columnArray[int8]:
		return editColumnArray(a.kind, a.data, convertEditI8, setRows, setValues, appendValues)
	case columnArray[int16]:
		return editColumnArray(a.kind, a.data, convertEditI16, setRows, setValues, appendValues)
	case columnArray[int32]:
		return editColumnArray(a.kind, a.data, convertEditI32, setRows, setValues, appendValues)
	case columnArray[int64]:
		return editColumnArray(a.kind, a.data, convertEditI64, setRows, setValues, appendValues)
	case columnArray[float64]:
		return editColumnArray(a.kind, a.data, convertEditF64, setRows, setValues, appendValues)
	case columnArray[string]:
		if a.kind == KindString {
			return editColumnArray(a.kind, a.data, convertEditString, setRows, setValues, appendValues)
		}
		return nil, false
	case columnArray[Symbol]:
		return editColumnArray(a.kind, a.data, convertEditSymbol, setRows, setValues, appendValues)
	case columnArray[Month]:
		return editColumnArray(a.kind, a.data, convertEditScalarKind[Month](a.kind), setRows, setValues, appendValues)
	case columnArray[Date]:
		return editColumnArray(a.kind, a.data, convertEditScalarKind[Date](a.kind), setRows, setValues, appendValues)
	case columnArray[DateTime]:
		return editColumnArray(a.kind, a.data, convertEditScalarKind[DateTime](a.kind), setRows, setValues, appendValues)
	case columnArray[Timespan]:
		return editColumnArray(a.kind, a.data, convertEditScalarKind[Timespan](a.kind), setRows, setValues, appendValues)
	case columnArray[Minute]:
		return editColumnArray(a.kind, a.data, convertEditScalarKind[Minute](a.kind), setRows, setValues, appendValues)
	case columnArray[Second]:
		return editColumnArray(a.kind, a.data, convertEditScalarKind[Second](a.kind), setRows, setValues, appendValues)
	case columnArray[Time]:
		return editColumnArray(a.kind, a.data, convertEditScalarKind[Time](a.kind), setRows, setValues, appendValues)
	case columnArray[Timestamp]:
		return editColumnArray(a.kind, a.data, convertEditScalarKind[Timestamp](a.kind), setRows, setValues, appendValues)
	case nullBitmapArray[int64]:
		return editNullBitmapArrayNullable(a, convertEditI64, setRows, setValues, appendValues)
	case nullBitmapArray[float64]:
		return editNullBitmapArrayNullable(a, convertEditF64, setRows, setValues, appendValues)
	}
	return nil, false
}

// arrayWithNullableTypedEdits handles edit batches that contain null values.
// Kinds with a null-bitmap carrier stay typed; other kinds support an
// all-null tail append through the lazy nullTailArray view; everything else
// falls back to the boxed path.
func arrayWithNullableTypedEdits(arr Array, setRows []int, setValues []any, appendValues []any) (Array, bool) {
	switch a := arr.(type) {
	case columnArray[int64]:
		base := nullBitmapArray[int64]{kind: a.kind, data: a.data, nulls: newNullBitmap(len(a.data))}
		return editNullBitmapArrayNullable(base, convertEditI64, setRows, setValues, appendValues)
	case columnArray[float64]:
		base := nullBitmapArray[float64]{kind: a.kind, data: a.data, nulls: newNullBitmap(len(a.data))}
		return editNullBitmapArrayNullable(base, convertEditF64, setRows, setValues, appendValues)
	case nullBitmapArray[int64]:
		return editNullBitmapArrayNullable(a, convertEditI64, setRows, setValues, appendValues)
	case nullBitmapArray[float64]:
		return editNullBitmapArrayNullable(a, convertEditF64, setRows, setValues, appendValues)
	}
	if len(setRows) == 0 && len(appendValues) > 0 && allNullEditValues(appendValues) {
		return appendNullTail(arr, len(appendValues))
	}
	return nil, false
}

func allNullEditValues(values []any) bool {
	for _, v := range values {
		if !IsNull(v) {
			return false
		}
	}
	return true
}

// appendNullTail returns a lazy view of arr extended with extra trailing null
// rows, for typed kinds that have no null-bitmap carrier. Chained appends
// extend the existing tail instead of nesting views.
func appendNullTail(arr Array, extra int) (Array, bool) {
	switch a := arr.(type) {
	case nullTailArray:
		return nullTailArray{source: a.source, extra: a.extra + extra}, true
	case columnArray[bool], columnArray[string], columnArray[Symbol],
		columnArray[Month], columnArray[Date], columnArray[DateTime],
		columnArray[Timespan], columnArray[Minute], columnArray[Second],
		columnArray[Time], columnArray[Timestamp]:
		return nullTailArray{source: arr, extra: extra}, true
	}
	return nil, false
}

// nullTailArray is an immutable view of source with extra trailing null rows.
// At/Values behave exactly like the boxed nullableArray the legacy append
// path would have produced: value rows return the source scalar, tail rows
// return NullValue.
type nullTailArray struct {
	source Array
	extra  int
}

func (a nullTailArray) Kind() Kind { return a.source.Kind() }

func (a nullTailArray) Len() int { return a.source.Len() + a.extra }

func (a nullTailArray) At(row int) (any, bool) {
	if row < 0 || row >= a.Len() {
		return nil, false
	}
	if row < a.source.Len() {
		return a.source.At(row)
	}
	return NullValue, true
}

func (a nullTailArray) Values() []any {
	out := make([]any, a.Len())
	copy(out, a.source.Values())
	for i := a.source.Len(); i < len(out); i++ {
		out[i] = NullValue
	}
	return out
}

func (a nullTailArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		v, ok := a.At(row)
		if !ok {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		out[i] = v
	}
	return nullableArray{kind: a.Kind(), data: out}
}

// newAllNullBitmap returns a bitmap with exactly the first n bits set.
func newAllNullBitmap(n int) []uint64 {
	nulls := newNullBitmap(n)
	for i := range nulls {
		nulls[i] = ^uint64(0)
	}
	if rest := n & 63; rest != 0 && len(nulls) > 0 {
		nulls[len(nulls)-1] = 1<<uint(rest) - 1
	}
	return nulls
}

// newSparseColumnFromGroupResults builds a full-length column whose matched
// rows broadcast per-group scalar results and whose remaining rows are null,
// staying typed for uniformly f64/i64 group results.
func newSparseColumnFromGroupResults(n int, indexes []int, ids []int, results []any) (Array, bool) {
	allF64, allI64 := true, true
	for _, v := range results {
		switch v.(type) {
		case nil:
		case float64:
			allI64 = false
		case int64:
			allF64 = false
		default:
			return nil, false
		}
	}
	switch {
	case allF64:
		data := make([]float64, n)
		nulls := newAllNullBitmap(n)
		for _, row := range indexes {
			v, ok := results[ids[row]].(float64)
			if !ok {
				return nil, false
			}
			data[row] = v
			nulls[row>>6] &^= uint64(1) << uint(row&63)
		}
		if !nullBitmapHasNull(nulls, n) {
			return columnArray[float64]{kind: KindF64, data: data}, true
		}
		return nullBitmapArray[float64]{kind: KindF64, data: data, nulls: nulls}, true
	case allI64:
		data := make([]int64, n)
		nulls := newAllNullBitmap(n)
		for _, row := range indexes {
			v, ok := results[ids[row]].(int64)
			if !ok {
				return nil, false
			}
			data[row] = v
			nulls[row>>6] &^= uint64(1) << uint(row&63)
		}
		if !nullBitmapHasNull(nulls, n) {
			return columnArray[int64]{kind: KindI64, data: data}, true
		}
		return nullBitmapArray[int64]{kind: KindI64, data: data, nulls: nulls}, true
	}
	return nil, false
}

func anyNullEditValue(values []any) bool {
	for _, v := range values {
		if IsNull(v) {
			return true
		}
	}
	return false
}

func editColumnArray[T any](kind Kind, data []T, conv func(any) (T, bool), setRows []int, setValues []any, appendValues []any) (Array, bool) {
	out := make([]T, len(data), len(data)+len(appendValues))
	copy(out, data)
	for i, row := range setRows {
		if row < 0 || row >= len(out) {
			return nil, false
		}
		v, ok := conv(setValues[i])
		if !ok {
			return nil, false
		}
		out[row] = v
	}
	for _, value := range appendValues {
		v, ok := conv(value)
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	return columnArray[T]{kind: kind, data: out}, true
}

func editNullBitmapArrayNullable[T nullBitmapElem](a nullBitmapArray[T], conv func(any) (T, bool), setRows []int, setValues []any, appendValues []any) (Array, bool) {
	n := len(a.data) + len(appendValues)
	data := make([]T, len(a.data), n)
	copy(data, a.data)
	nulls := newNullBitmap(n)
	copy(nulls, a.nulls)
	for i, row := range setRows {
		if row < 0 || row >= len(data) {
			return nil, false
		}
		if IsNull(setValues[i]) {
			var zero T
			data[row] = zero
			nulls[row>>6] |= uint64(1) << uint(row&63)
			continue
		}
		v, ok := conv(setValues[i])
		if !ok {
			return nil, false
		}
		data[row] = v
		nulls[row>>6] &^= uint64(1) << uint(row&63)
	}
	for _, value := range appendValues {
		row := len(data)
		if IsNull(value) {
			var zero T
			data = append(data, zero)
			nulls[row>>6] |= uint64(1) << uint(row&63)
			continue
		}
		v, ok := conv(value)
		if !ok {
			return nil, false
		}
		data = append(data, v)
	}
	if !nullBitmapHasNull(nulls, len(data)) {
		return columnArray[T]{kind: a.kind, data: data}, true
	}
	return nullBitmapArray[T]{kind: a.kind, data: data, nulls: nulls}, true
}

// Element converters mirror the arrayWithKind coercions for each kind.

func convertEditBool(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func convertEditI8(v any) (int8, bool) {
	if n, ok := coerceInt64Exact(v); ok && n >= -128 && n <= 127 {
		return int8(n), true
	}
	x, ok := v.(int8)
	return x, ok
}

func convertEditI16(v any) (int16, bool) {
	if n, ok := coerceInt64Exact(v); ok && n >= -32768 && n <= 32767 {
		return int16(n), true
	}
	x, ok := v.(int16)
	return x, ok
}

func convertEditI32(v any) (int32, bool) {
	if n, ok := coerceInt64Exact(v); ok && n >= -2147483648 && n <= 2147483647 {
		return int32(n), true
	}
	x, ok := v.(int32)
	return x, ok
}

func convertEditI64(v any) (int64, bool) {
	return coerceInt64Exact(v)
}

func convertEditF64(v any) (float64, bool) {
	return numeric(v)
}

func convertEditString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func convertEditSymbol(v any) (Symbol, bool) {
	switch s := v.(type) {
	case Symbol:
		return s, true
	case string:
		return Symbol(s), true
	default:
		return "", false
	}
}

func convertEditScalarKind[T any](kind Kind) func(any) (T, bool) {
	return func(v any) (T, bool) {
		var zero T
		if x, ok := v.(T); ok {
			return x, true
		}
		normalized, err := normalizeScalarForKind(kind, v, 0)
		if err != nil {
			return zero, false
		}
		x, ok := normalized.(T)
		if !ok {
			return zero, false
		}
		return x, true
	}
}

// arrayScatterArray returns a copy of dst with src[i] assigned at rows[i].
// src must have exactly len(rows) elements. Common kind pairs scatter through
// typed slices; anything else converts per element through the typed edit
// kernel; ok=false defers to the caller's boxed fallback.
func arrayScatterArray(dst Array, rows []int, src Array) (Array, bool) {
	if src == nil || src.Len() != len(rows) {
		return nil, false
	}
	if a, ok := dst.(attributedArray); ok {
		return arrayScatterArray(a.array, rows, src)
	}
	switch d := dst.(type) {
	case columnArray[float64]:
		vals := make([]float64, len(rows))
		if ok, err := TryExportF64Copy(src, vals); ok && err == nil {
			out := make([]float64, len(d.data))
			copy(out, d.data)
			for i, row := range rows {
				if row < 0 || row >= len(out) {
					return nil, false
				}
				out[row] = vals[i]
			}
			return columnArray[float64]{kind: d.kind, data: out}, true
		}
	case columnArray[int64]:
		vals := make([]int64, len(rows))
		if ok, err := TryExportI64Copy(src, vals); ok && err == nil {
			out := make([]int64, len(d.data))
			copy(out, d.data)
			for i, row := range rows {
				if row < 0 || row >= len(out) {
					return nil, false
				}
				out[row] = vals[i]
			}
			return columnArray[int64]{kind: d.kind, data: out}, true
		}
	case columnArray[bool]:
		vals := make([]bool, len(rows))
		if ok, err := TryExportBoolCopy(src, vals); ok && err == nil {
			out := make([]bool, len(d.data))
			copy(out, d.data)
			for i, row := range rows {
				if row < 0 || row >= len(out) {
					return nil, false
				}
				out[row] = vals[i]
			}
			return columnArray[bool]{kind: d.kind, data: out}, true
		}
	case columnArray[string]:
		if d.kind != KindString {
			break
		}
		vals := make([]string, len(rows))
		if ok, err := TryExportStringCopy(src, vals); ok && err == nil {
			out := make([]string, len(d.data))
			copy(out, d.data)
			for i, row := range rows {
				if row < 0 || row >= len(out) {
					return nil, false
				}
				out[row] = vals[i]
			}
			return columnArray[string]{kind: d.kind, data: out}, true
		}
	case columnArray[Symbol]:
		vals := make([]string, len(rows))
		if ok, err := TryExportStringCopy(src, vals); ok && err == nil {
			out := make([]Symbol, len(d.data))
			copy(out, d.data)
			for i, row := range rows {
				if row < 0 || row >= len(out) {
					return nil, false
				}
				out[row] = Symbol(vals[i])
			}
			return columnArray[Symbol]{kind: d.kind, data: out}, true
		}
	}
	// Generic lane: boxed only over the matched rows, never the whole column.
	return arrayWithTypedEdits(dst, rows, src.Values(), nil)
}

// newSparseColumnArray builds a column of n rows whose values at rows[i] come
// from src and whose remaining rows are null, without boxing the full column.
func newSparseColumnArray(n int, rows []int, src Array) (Array, bool) {
	if src == nil || src.Len() != len(rows) {
		return nil, false
	}
	if len(rows) == n && allIndexesPresent(rows) {
		return MaterializeFrameColumn(src), true
	}
	switch src.Kind() {
	case KindF64:
		vals := make([]float64, len(rows))
		if ok, err := TryExportF64Copy(src, vals); !ok || err != nil {
			return nil, false
		}
		data := make([]float64, n)
		nulls := newAllNullBitmap(n)
		for i, row := range rows {
			if row < 0 || row >= n {
				return nil, false
			}
			data[row] = vals[i]
			nulls[row>>6] &^= uint64(1) << uint(row&63)
		}
		return nullBitmapArray[float64]{kind: KindF64, data: data, nulls: nulls}, true
	case KindI64:
		vals := make([]int64, len(rows))
		if ok, err := TryExportI64Copy(src, vals); !ok || err != nil {
			return nil, false
		}
		data := make([]int64, n)
		nulls := newAllNullBitmap(n)
		for i, row := range rows {
			if row < 0 || row >= n {
				return nil, false
			}
			data[row] = vals[i]
			nulls[row>>6] &^= uint64(1) << uint(row&63)
		}
		return nullBitmapArray[int64]{kind: KindI64, data: data, nulls: nulls}, true
	}
	return nil, false
}

// sharedRowsByKey adopts an existing rowsByKey map. The keyed-frame contract
// is that rowsByKey maps are never mutated after construction, so sharing is
// safe across derived keyed frames.
func sharedRowsByKey(rowsByKey map[string][]int) map[string][]int {
	return rowsByKey
}

func cloneRowsByKeyShallow(rowsByKey map[string][]int) map[string][]int {
	out := make(map[string][]int, len(rowsByKey)+1)
	for key, rows := range rowsByKey {
		out[key] = rows
	}
	return out
}

// Schema returns the keyed frame's full schema (key and value columns)
// without copying row data.
func (k KeyedFrame) Schema() Schema {
	return k.frame.Schema()
}

// UpdateWhereKeyed applies UpdateWhere to the keyed frame's backing frame.
// When no key column is assigned the row->key mapping is unchanged, so the
// existing key index is reused instead of re-keying the whole table.
func UpdateWhereKeyed(keyed KeyedFrame, where Expr, assignments map[Symbol]Expr) (KeyedFrame, error) {
	if len(keyed.keys) == 0 {
		return KeyedFrame{}, fmt.Errorf("keyed frame is not initialized")
	}
	out, err := UpdateWhere(keyed.frame, where, assignments)
	if err != nil {
		return KeyedFrame{}, err
	}
	return rekeyedAfterValueUpdate(keyed, out, assignments)
}

// UpdateByKeyed applies a grouped UpdateBy to the keyed frame's backing frame
// reusing the key index when no key column is assigned.
func UpdateByKeyed(keyed KeyedFrame, where Expr, by []SelectItem, assignments []GroupedAssignment) (KeyedFrame, error) {
	if len(keyed.keys) == 0 {
		return KeyedFrame{}, fmt.Errorf("keyed frame is not initialized")
	}
	out, err := UpdateBy(keyed.frame, where, by, assignments)
	if err != nil {
		return KeyedFrame{}, err
	}
	assigned := make(map[Symbol]Expr, len(assignments))
	for _, assign := range assignments {
		assigned[assign.Name] = assign.Expr
	}
	return rekeyedAfterValueUpdate(keyed, out, assigned)
}

func rekeyedAfterValueUpdate(keyed KeyedFrame, out Frame, assignments map[Symbol]Expr) (KeyedFrame, error) {
	if out.Len() != keyed.frame.Len() {
		return KeyBy(out, keyed.keys...)
	}
	for _, key := range keyed.keys {
		if _, ok := assignments[key]; ok {
			return KeyBy(out, keyed.keys...)
		}
	}
	schemaHash := out.SchemaFingerprint()
	fp := keyed.fp
	if schemaHash != keyed.index.SchemaHash || fp == nil {
		fp = &keyedFingerprintCell{}
	}
	return KeyedFrame{
		frame:       out,
		keys:        append([]Symbol(nil), keyed.keys...),
		rowsByKey:   sharedRowsByKey(keyed.rowsByKey),
		rowsOverlay: keyed.rowsOverlay,
		index: KeyedIndexMetadata{
			Rows:       out.Len(),
			Keys:       append([]Symbol(nil), keyed.keys...),
			SchemaHash: schemaHash,
		},
		fp: fp,
	}, nil
}

// gatherFrameRowsView builds a frame whose columns are lazy index views over
// the source frame's columns: one shared trusted index array, no dense
// per-column copies. Frames are immutable values, so the views stay valid for
// the result's lifetime; dense-copy gathers remain available through
// Frame.Gather for callers that need detached storage. rows must already be
// validated in-range and is adopted without copying. Wide-survivor mutations
// (delete a few rows from a large table) use this to keep the mutation cost
// proportional to the index, not the surviving table.
func gatherFrameRowsView(frame Frame, rows []int) (Frame, error) {
	indexes := make([]int64, len(rows))
	for i, row := range rows {
		indexes[i] = int64(row)
	}
	return gatherFrameRowsViewI64(frame, indexes)
}

// gatherFrameRowsViewI64 is gatherFrameRowsView with an adopted (trusted,
// in-range, caller-owned) int64 row index slice.
func gatherFrameRowsViewI64(frame Frame, rows []int64) (Frame, error) {
	indexArray := columnArray[int64]{kind: KindI64, data: rows}
	cols := make([]Column, 0, len(frame.schema.names))
	for _, name := range frame.schema.names {
		cols = append(cols, Column{Name: name, Data: indexedArray{
			source:  frame.columns[name],
			indexes: indexArray,
			len:     len(rows),
		}})
	}
	return newFrameTrusted(cols...)
}

// DeleteWhereKeyed deletes matching rows from a keyed frame, remapping the
// existing key index instead of re-keying the whole table.
func DeleteWhereKeyed(keyed KeyedFrame, where Expr) (KeyedFrame, error) {
	if len(keyed.keys) == 0 {
		return KeyedFrame{}, fmt.Errorf("keyed frame is not initialized")
	}
	deleteIndexes, err := filterIndexes(keyed.frame, where)
	if err != nil {
		return KeyedFrame{}, err
	}
	if len(deleteIndexes) == 0 {
		return keyed, nil
	}
	n := keyed.frame.Len()
	rowMap := make([]int32, n)
	for _, row := range deleteIndexes {
		if row < 0 || row >= n {
			return KeyedFrame{}, fmt.Errorf("delete index %d out of range", row)
		}
		rowMap[row] = -1
	}
	keep := make([]int64, 0, n-len(deleteIndexes))
	for row := 0; row < n; row++ {
		if rowMap[row] < 0 {
			continue
		}
		rowMap[row] = int32(len(keep))
		keep = append(keep, int64(row))
	}
	out, err := gatherFrameRowsViewI64(keyed.frame, keep)
	if err != nil {
		return KeyedFrame{}, err
	}
	// Each surviving row belongs to exactly one key, so the remapped per-key
	// row lists share one flat backing slice instead of one allocation per key.
	flat := make([]int, 0, len(keep))
	newRows := make(map[string][]int, keyed.indexKeyCount())
	keyed.forEachIndexKey(func(key string, rows []int) {
		start := len(flat)
		for _, row := range rows {
			if nr := rowMap[row]; nr >= 0 {
				flat = append(flat, int(nr))
			}
		}
		if len(flat) > start {
			newRows[key] = flat[start:len(flat):len(flat)]
		}
	})
	return KeyedFrame{
		frame:     out,
		keys:      append([]Symbol(nil), keyed.keys...),
		rowsByKey: newRows,
		index: KeyedIndexMetadata{
			Rows:       out.Len(),
			Keys:       append([]Symbol(nil), keyed.keys...),
			SchemaHash: out.SchemaFingerprint(),
		},
		fp: &keyedFingerprintCell{},
	}, nil
}

// updateByGroupedTyped is the typed fast path for grouped updates: it uses
// the fby group-id kernels instead of per-row string keys and copy-on-writes
// only the assigned columns. ok=false defers to the boxed rowKey path.
func updateByGroupedTyped(frame Frame, indexes []int, byInputs []groupInput, aggs []aggregateInput, assignments []GroupedAssignment) (Frame, bool, error) {
	keyColumns := make([]Symbol, len(byInputs))
	for i, item := range byInputs {
		ref, ok := item.Expr.(ColumnRef)
		if !ok {
			return Frame{}, false, nil
		}
		keyColumns[i] = ref.Name
	}
	ids, groupCount, ok := frameRowGroupIDs(frame, keyColumns)
	if !ok {
		return Frame{}, false, nil
	}
	states := make([]*groupState, groupCount)
	for _, row := range indexes {
		if row < 0 || row >= len(ids) {
			return Frame{}, false, nil
		}
		id := ids[row]
		state := states[id]
		if state == nil {
			state = &groupState{aggs: make([]aggregateState, len(aggs))}
			for i, agg := range aggs {
				state.aggs[i].fn = agg.Func
			}
			states[id] = state
		}
		for i := range aggs {
			if err := accumulateAggregate(&state.aggs[i], aggs[i], frame, row); err != nil {
				return Frame{}, true, err
			}
		}
	}
	assignmentByName := make(map[Symbol]int, len(assignments))
	for i, assign := range assignments {
		if _, ok := assignmentByName[assign.Name]; ok {
			return Frame{}, true, fmt.Errorf("duplicate grouped update assignment for column %q", assign.Name)
		}
		assignmentByName[assign.Name] = i
	}
	// Box each group result once; matched rows then share the boxed value.
	results := make([][]any, len(aggs))
	for i := range aggs {
		vals := make([]any, groupCount)
		for g, state := range states {
			if state != nil {
				vals[g] = aggregateResult(state.aggs[i])
			}
		}
		results[i] = vals
	}
	cols := make([]Column, 0, len(frame.schema.names)+len(assignments))
	for _, name := range frame.schema.names {
		col := frame.columns[name]
		assignIndex, assigned := assignmentByName[name]
		if !assigned || len(indexes) == 0 {
			cols = append(cols, Column{Name: name, Data: col})
			continue
		}
		setValues := make([]any, len(indexes))
		for i, row := range indexes {
			setValues[i] = results[assignIndex][ids[row]]
		}
		if out, ok := arrayWithTypedEdits(col, indexes, setValues, nil); ok {
			cols = append(cols, Column{Name: name, Data: out})
			continue
		}
		values := col.Values()
		for i, row := range indexes {
			values[row] = setValues[i]
		}
		out, err := columnWithKind(name, col.Kind(), values)
		if err != nil {
			return Frame{}, true, err
		}
		cols = append(cols, out)
	}
	for i, assign := range assignments {
		if _, ok := frame.Column(assign.Name); ok {
			continue
		}
		if out, ok := newSparseColumnFromGroupResults(frame.Len(), indexes, ids, results[i]); ok {
			cols = append(cols, Column{Name: assign.Name, Data: out})
			continue
		}
		values := make([]any, frame.Len())
		for row := range values {
			values[row] = NullValue
		}
		for _, row := range indexes {
			values[row] = results[i][ids[row]]
		}
		cols = append(cols, NewColumn(assign.Name, values))
	}
	out, err := newFrameTrusted(cols...)
	return out, true, err
}

type keyedTypedAppend struct {
	key       string
	keyValues []any
	deltaRow  int
}

// mutateTyped is the typed O(delta) fast path for KeyedFrame.mutate: it
// copy-on-writes only the touched columns and updates the key index
// incrementally. ok=false defers to the boxed legacy implementation.
func (k KeyedFrame) mutateTyped(delta Frame, appendMissing bool, valueCols []Symbol) (KeyedFrame, bool, error) {
	for _, name := range valueCols {
		if _, ok := k.frame.Column(name); !ok {
			// Adding a brand-new column is handled by the boxed path.
			return KeyedFrame{}, false, nil
		}
	}
	valueSet := make(map[Symbol]struct{}, len(valueCols))
	for _, name := range valueCols {
		valueSet[name] = struct{}{}
	}
	type colEdit struct {
		rows   []int
		values []any
	}
	var edits map[Symbol]*colEdit
	var appends []keyedTypedAppend
	for row := 0; row < delta.Len(); row++ {
		key, keyValues, err := deltaRowKey(k, delta, row)
		if err != nil {
			return KeyedFrame{}, true, err
		}
		targetRows := k.lookupRowsByKey(key)
		if len(targetRows) > 0 {
			if edits == nil {
				edits = make(map[Symbol]*colEdit, len(valueCols))
			}
			for _, name := range valueCols {
				v, _ := delta.columns[name].At(row)
				edit := edits[name]
				if edit == nil {
					edit = &colEdit{}
					edits[name] = edit
				}
				for _, targetRow := range targetRows {
					edit.rows = append(edit.rows, targetRow)
					edit.values = append(edit.values, v)
				}
			}
			continue
		}
		if !appendMissing {
			continue
		}
		appends = append(appends, keyedTypedAppend{key: key, keyValues: keyValues, deltaRow: row})
	}
	outCols := make([]Column, 0, len(k.frame.schema.names))
	for _, name := range k.frame.schema.names {
		col := k.frame.columns[name]
		var setRows []int
		var setValues []any
		if edit := edits[name]; edit != nil {
			setRows, setValues = edit.rows, edit.values
		}
		var appendValues []any
		if len(appends) > 0 {
			appendValues = make([]any, len(appends))
			keyIndex := symbolIndex(k.keys, name)
			for i, ap := range appends {
				switch {
				case keyIndex >= 0:
					appendValues[i] = ap.keyValues[keyIndex]
				case hasColumn(delta, name) && hasSymbol(valueSet, name):
					v, _ := delta.columns[name].At(ap.deltaRow)
					appendValues[i] = v
				default:
					appendValues[i] = NullValue
				}
			}
		}
		if len(setRows) == 0 && len(appendValues) == 0 {
			outCols = append(outCols, Column{Name: name, Data: col})
			continue
		}
		out, ok := arrayWithTypedEdits(col, setRows, setValues, appendValues)
		if !ok {
			return KeyedFrame{}, false, nil
		}
		outCols = append(outCols, Column{Name: name, Data: out})
	}
	outFrame, err := newFrameTrusted(outCols...)
	if err != nil {
		return KeyedFrame{}, true, err
	}
	if len(appends) == 0 {
		return KeyedFrame{
			frame:       outFrame,
			keys:        append([]Symbol(nil), k.keys...),
			rowsByKey:   sharedRowsByKey(k.rowsByKey),
			rowsOverlay: k.rowsOverlay,
			index:       k.index,
			fp:          k.fp,
		}, true, nil
	}
	// Appended rows introduce only brand-new keys (existing keys take the
	// in-place update path above), so they accumulate in a small overlay
	// instead of cloning the whole base index map per insert.
	overlay := cloneRowsByKeyShallow(k.rowsOverlay)
	base := k.frame.Len()
	for i, ap := range appends {
		overlay[ap.key] = append(append([]int(nil), overlay[ap.key]...), base+i)
	}
	rowsByKey := k.rowsByKey
	if len(overlay) > keyedRowsOverlayFlattenLimit {
		merged := make(map[string][]int, len(k.rowsByKey)+len(overlay))
		for key, rows := range k.rowsByKey {
			merged[key] = rows
		}
		for key, rows := range overlay {
			merged[key] = rows
		}
		rowsByKey, overlay = merged, nil
	}
	return KeyedFrame{
		frame:       outFrame,
		keys:        append([]Symbol(nil), k.keys...),
		rowsByKey:   rowsByKey,
		rowsOverlay: overlay,
		index: KeyedIndexMetadata{
			Rows:       outFrame.Len(),
			Keys:       append([]Symbol(nil), k.keys...),
			SchemaHash: k.index.SchemaHash,
		},
		fp: &keyedFingerprintCell{},
	}, true, nil
}
