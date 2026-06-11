package data

import "fmt"

// This file holds the shared columnar fast paths for the frame group/ungroup
// pipeline. They are shape-generic: any frame whose key columns resolve to
// typed group ids (the same fbyGroupIDs mechanism fby uses) takes the typed
// route, and anything else falls back to the boxed row-key implementations.

// frameRowGroupIDs computes first-appearance-ordered group ids for the given
// key columns by reusing the fby group-id kernels per column and combining
// composite keys positionally. ok=false means the caller must use the boxed
// rowKey path (e.g. null-bearing integer keys or exotic carriers).
func frameRowGroupIDs(frame Frame, keyColumns []Symbol) ([]int, int, bool) {
	rows := frame.Len()
	if len(keyColumns) == 0 || rows == 0 {
		return nil, 0, false
	}
	arrays := make([]Array, len(keyColumns))
	for i, name := range keyColumns {
		col, ok := frame.Column(name)
		if !ok {
			return nil, 0, false
		}
		arrays[i] = col
	}
	return arraysRowGroupIDs(arrays, rows)
}

// arraysRowGroupIDs is the array-level core of frameRowGroupIDs: it computes
// first-appearance-ordered composite group ids for already-resolved key
// arrays (each of length rows).
func arraysRowGroupIDs(arrays []Array, rows int) ([]int, int, bool) {
	if len(arrays) == 0 || rows == 0 {
		return nil, 0, false
	}
	perColumn := make([][]int, len(arrays))
	counts := make([]int, len(arrays))
	for i, col := range arrays {
		ids, count, err := fbyGroupIDs(col)
		if err != nil || len(ids) != rows {
			return nil, 0, false
		}
		perColumn[i], counts[i] = ids, count
	}
	return combineRowGroupIDs(perColumn, counts, rows)
}

// combineRowGroupIDs folds per-column group ids into composite ids in
// first-appearance order. Small composite domains use a flat slate; larger
// ones fall back to a hash map.
func combineRowGroupIDs(perColumn [][]int, counts []int, rows int) ([]int, int, bool) {
	combined := perColumn[0]
	combinedCount := counts[0]
	for ki := 1; ki < len(perColumn); ki++ {
		ids, count := perColumn[ki], counts[ki]
		next := make([]int, rows)
		if domain := int64(combinedCount) * int64(count); domain > 0 && domain <= int64(rows)*4 && domain <= 1<<22 {
			// Small composite domain: assign pair ids through a flat
			// first-appearance slate instead of a hash map.
			slate := make([]int32, domain)
			assigned := 0
			for row := 0; row < rows; row++ {
				key := combined[row]*count + ids[row]
				id := slate[key]
				if id == 0 {
					assigned++
					id = int32(assigned)
					slate[key] = id
				}
				next[row] = int(id) - 1
			}
			combined, combinedCount = next, assigned
			continue
		}
		pairIDs := make(map[int64]int, combinedCount)
		for row := 0; row < rows; row++ {
			key := int64(combined[row])*int64(count) + int64(ids[row])
			id, seen := pairIDs[key]
			if !seen {
				id = len(pairIDs)
				pairIDs[key] = id
			}
			next[row] = id
		}
		combined, combinedCount = next, len(pairIDs)
	}
	return combined, combinedCount, true
}

// rowGroupsFromIDs converts per-row group ids into per-group row-index slices
// (first-appearance group order, ascending rows within each group) using one
// flat backing slice.
func rowGroupsFromIDs(ids []int, groupCount int) [][]int {
	counts := make([]int, groupCount)
	for _, id := range ids {
		counts[id]++
	}
	flat := make([]int, len(ids))
	offsets := make([]int, groupCount)
	pos := 0
	for g := 0; g < groupCount; g++ {
		offsets[g] = pos
		pos += counts[g]
	}
	fill := make([]int, groupCount)
	for row, id := range ids {
		flat[offsets[id]+fill[id]] = row
		fill[id]++
	}
	groups := make([][]int, groupCount)
	for g := 0; g < groupCount; g++ {
		groups[g] = flat[offsets[g] : offsets[g]+counts[g]]
	}
	return groups
}

// ungroupColumnar is the column-wise Ungroup fast path. It handles frames
// whose KindAny columns are uniformly nested (every row an array/list) and
// whose remaining columns are scalar-kind storage, producing typed output
// columns without per-row boxing. ok=false defers to the row-loop
// implementation for mixed or degenerate shapes.
func ungroupColumnar(frame Frame) (Frame, bool, error) {
	names := frame.schema.Names()
	rows := frame.Len()
	if rows == 0 || len(names) == 0 {
		return Frame{}, false, nil
	}
	nested := make(map[Symbol][]nestedRowValue, len(names))
	for _, name := range names {
		col := frame.columns[name]
		kind := col.Kind()
		if kind != KindAny && kind != KindNull {
			// Typed scalar storage never yields nested row values.
			continue
		}
		vals := make([]nestedRowValue, rows)
		allNested := true
		for row := 0; row < rows; row++ {
			v, ok := col.At(row)
			if !ok {
				return Frame{}, false, fmt.Errorf("ungroup column %q row %d out of range", name, row)
			}
			n, isNested := asNestedRowValue(v)
			if !isNested {
				allNested = false
				break
			}
			vals[row] = n
		}
		if !allNested {
			// Scalar-or-mixed any-kind column: keep the row-loop semantics.
			return Frame{}, false, nil
		}
		nested[name] = vals
	}
	if len(nested) == 0 {
		// Pure scalar frame: the row loop re-infers column kinds; defer.
		return Frame{}, false, nil
	}

	widths := make([]int, rows)
	total := 0
	for row := 0; row < rows; row++ {
		width := -1
		for _, name := range names {
			vals, ok := nested[name]
			if !ok {
				continue
			}
			n := vals[row].Len()
			if width < 0 {
				width = n
			} else if width != n {
				return Frame{}, true, fmt.Errorf("ungroup row %d has mismatched nested column lengths", row)
			}
		}
		widths[row] = width
		total += width
	}

	cols := make([]Column, 0, len(names))
	for _, name := range names {
		if vals, ok := nested[name]; ok {
			data, typed := concatNestedParts(vals, total)
			if !typed {
				data = concatNestedPartsBoxed(name, vals, total)
			}
			cols = append(cols, Column{Name: name, Data: data})
			continue
		}
		col := frame.columns[name]
		data, typed := repeatScalarColumn(col, widths, total)
		if !typed {
			data = repeatScalarColumnBoxed(col, widths, total)
		}
		cols = append(cols, Column{Name: name, Data: data})
	}
	out, err := NewFrameAdoptingColumns(cols...)
	if err != nil {
		return Frame{}, true, err
	}
	return out, true, nil
}

// concatNestedParts concatenates per-row nested vectors into one typed column
// when every part materializes to the same typed carrier.
func concatNestedParts(parts []nestedRowValue, total int) (Array, bool) {
	arrays := make([]Array, len(parts))
	for i, p := range parts {
		part, ok := p.array.(Array)
		if !ok {
			return nil, false
		}
		a := MaterializeArray(part)
		if att, ok := a.(attributedArray); ok {
			a = att.array
		}
		arrays[i] = a
	}
	switch arrays[0].(type) {
	case columnArray[bool]:
		return concatColumnArrays[bool](arrays, total)
	case columnArray[int8]:
		return concatColumnArrays[int8](arrays, total)
	case columnArray[int16]:
		return concatColumnArrays[int16](arrays, total)
	case columnArray[int32]:
		return concatColumnArrays[int32](arrays, total)
	case columnArray[int64]:
		return concatColumnArrays[int64](arrays, total)
	case columnArray[uint8]:
		return concatColumnArrays[uint8](arrays, total)
	case columnArray[uint16]:
		return concatColumnArrays[uint16](arrays, total)
	case columnArray[uint32]:
		return concatColumnArrays[uint32](arrays, total)
	case columnArray[uint64]:
		return concatColumnArrays[uint64](arrays, total)
	case columnArray[float32]:
		return concatColumnArrays[float32](arrays, total)
	case columnArray[float64]:
		return concatColumnArrays[float64](arrays, total)
	case columnArray[string]:
		return concatColumnArrays[string](arrays, total)
	case columnArray[Symbol]:
		return concatColumnArrays[Symbol](arrays, total)
	case columnArray[Month]:
		return concatColumnArrays[Month](arrays, total)
	case columnArray[Date]:
		return concatColumnArrays[Date](arrays, total)
	case columnArray[DateTime]:
		return concatColumnArrays[DateTime](arrays, total)
	case columnArray[Timespan]:
		return concatColumnArrays[Timespan](arrays, total)
	case columnArray[Minute]:
		return concatColumnArrays[Minute](arrays, total)
	case columnArray[Second]:
		return concatColumnArrays[Second](arrays, total)
	case columnArray[Time]:
		return concatColumnArrays[Time](arrays, total)
	case columnArray[Timestamp]:
		return concatColumnArrays[Timestamp](arrays, total)
	}
	return nil, false
}

func concatColumnArrays[T any](arrays []Array, total int) (Array, bool) {
	first, ok := arrays[0].(columnArray[T])
	if !ok {
		return nil, false
	}
	out := make([]T, 0, total)
	for _, a := range arrays {
		ca, ok := a.(columnArray[T])
		if !ok || ca.kind != first.kind {
			return nil, false
		}
		out = append(out, ca.data...)
	}
	return columnArray[T]{kind: first.kind, data: out}, true
}

func concatNestedPartsBoxed(name Symbol, parts []nestedRowValue, total int) Array {
	out := make([]any, 0, total)
	for _, p := range parts {
		n := p.Len()
		for i := 0; i < n; i++ {
			v, _ := p.At(i)
			out = append(out, v)
		}
	}
	return InferArray(out)
}

// repeatScalarColumn expands a scalar column by per-row widths into one typed
// column when the source materializes to a typed carrier.
func repeatScalarColumn(col Array, widths []int, total int) (Array, bool) {
	a := MaterializeArray(col)
	if att, ok := a.(attributedArray); ok {
		a = att.array
	}
	switch ca := a.(type) {
	case columnArray[bool]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[int8]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[int16]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[int32]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[int64]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[uint8]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[uint16]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[uint32]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[uint64]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[float32]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[float64]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[string]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[Symbol]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[Month]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[Date]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[DateTime]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[Timespan]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[Minute]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[Second]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[Time]:
		return repeatColumnValues(ca, widths, total), true
	case columnArray[Timestamp]:
		return repeatColumnValues(ca, widths, total), true
	}
	return nil, false
}

func repeatColumnValues[T any](a columnArray[T], widths []int, total int) Array {
	out := make([]T, 0, total)
	for row, width := range widths {
		value := a.data[row]
		for i := 0; i < width; i++ {
			out = append(out, value)
		}
	}
	return columnArray[T]{kind: a.kind, data: out}
}

func repeatScalarColumnBoxed(col Array, widths []int, total int) Array {
	out := make([]any, 0, total)
	for row, width := range widths {
		value, _ := col.At(row)
		for i := 0; i < width; i++ {
			out = append(out, value)
		}
	}
	return InferArray(out)
}
