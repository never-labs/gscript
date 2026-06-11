package data

import "math"

// Typed grouped-aggregate execution over positional row group ids.
//
// execGroupedTypedRowIDs is the shape-generic fast path for grouped
// aggregates that do not fit the single-column ArrayIndex machinery
// (multi-key and computed-key group-bys) or whose aggregate set the older
// typed kernels decline (avg/var/dev/first/last). Group keys are resolved
// positionally through the fby group-id kernels — the same mechanism
// frameRowGroupIDs uses — and combined for composite keys, so no per-row
// string keys or boxed key values are built. Aggregates accumulate into flat
// typed slices and first/last resolve to row indexes gathered once at the
// end, so output columns keep their source kinds without boxing.

type typedRowIDsAccumulator struct {
	input    aggregateInput
	valueFn  func(row int) (float64, bool)
	sum      []float64
	sumsq    []float64
	count    []int64
	firstRow []int
	lastRow  []int
}

// resolveNumericColumnFn returns a direct typed row reader for dense numeric
// columns, avoiding the per-row type switch in NumericAt. nil defers to the
// generic accessor.
func resolveNumericColumnFn(col Array) func(row int) (float64, bool) {
	switch a := unwrapAttributedArray(col).(type) {
	case columnArray[float64]:
		data := a.data
		return func(row int) (float64, bool) { return data[row], true }
	case columnArray[float32]:
		data := a.data
		return func(row int) (float64, bool) { return float64(data[row]), true }
	case columnArray[int64]:
		data := a.data
		return func(row int) (float64, bool) { return float64(data[row]), true }
	case columnArray[int32]:
		data := a.data
		return func(row int) (float64, bool) { return float64(data[row]), true }
	case columnArray[int16]:
		data := a.data
		return func(row int) (float64, bool) { return float64(data[row]), true }
	case columnArray[int8]:
		data := a.data
		return func(row int) (float64, bool) { return float64(data[row]), true }
	default:
		return nil
	}
}

// resolveAggNumericFn pre-binds the aggregate's numeric row reader for the
// dense typed shapes (plain column or binary op over two dense columns).
func resolveAggNumericFn(agg aggregateInput) func(row int) (float64, bool) {
	if agg.column != nil {
		return resolveNumericColumnFn(agg.column)
	}
	if agg.leftColumn == nil || agg.rightColumn == nil {
		return nil
	}
	if agg.binaryOp == OpMul {
		// Mirror aggregateBinaryNumericValueFast: i64*i64 multiplies in
		// integer space before converting.
		if li, ok := unwrapAttributedArray(agg.leftColumn).(columnArray[int64]); ok {
			if ri, ok := unwrapAttributedArray(agg.rightColumn).(columnArray[int64]); ok {
				ldata, rdata := li.data, ri.data
				return func(row int) (float64, bool) { return float64(ldata[row] * rdata[row]), true }
			}
		}
	}
	lf := resolveNumericColumnFn(agg.leftColumn)
	rf := resolveNumericColumnFn(agg.rightColumn)
	if lf == nil || rf == nil {
		return nil
	}
	switch agg.binaryOp {
	case OpAdd:
		return func(row int) (float64, bool) {
			l, _ := lf(row)
			r, _ := rf(row)
			return l + r, true
		}
	case OpSub:
		return func(row int) (float64, bool) {
			l, _ := lf(row)
			r, _ := rf(row)
			return l - r, true
		}
	case OpMul:
		return func(row int) (float64, bool) {
			l, _ := lf(row)
			r, _ := rf(row)
			return l * r, true
		}
	case OpDiv:
		return func(row int) (float64, bool) {
			l, _ := lf(row)
			r, _ := rf(row)
			if r == 0 {
				return 0, false
			}
			return l / r, true
		}
	default:
		return nil
	}
}

func typedRowIDsAggSupported(agg aggregateInput) bool {
	if agg.Weight != nil {
		return false
	}
	switch agg.Func {
	case "count":
		return true
	case "sum", "avg", "var", "dev":
		if agg.column != nil {
			return isNumericArray(agg.column)
		}
		if agg.leftColumn != nil && agg.rightColumn != nil {
			return isNumericBinaryAggregateOp(agg.binaryOp) && isNumericArray(agg.leftColumn) && isNumericArray(agg.rightColumn)
		}
		return false
	case "first", "last":
		return agg.column != nil
	default:
		return false
	}
}

// groupByArrays resolves every by-item to a full-frame Array: bound columns
// directly, computed keys (e.g. xbar buckets) through the vectorized
// expression evaluator. ok=false defers to the boxed row-key path.
func groupByArrays(frame Frame, byInputs []groupInput) ([]Array, bool) {
	rows := frame.Len()
	byArrays := make([]Array, len(byInputs))
	for i, item := range byInputs {
		if item.column != nil {
			byArrays[i] = item.column
			continue
		}
		array, ok, err := evalExprArray(frame, item.Expr)
		if err != nil || !ok || array == nil || array.Len() != rows {
			return nil, false
		}
		byArrays[i] = array
	}
	return byArrays, true
}

// groupColumnRowIDs resolves per-row group ids for one by-array. Plain column
// references borrow the column's grouped/unique ArrayIndex when present and
// attach a freshly built one otherwise (the same warm-query caching
// groupIndexForSingleColumn performs), so repeated grouped queries skip the
// hash pass entirely. Computed keys go through the fby group-id kernels.
func groupColumnRowIDs(frame Frame, item groupInput, array Array) ([]int, int, bool) {
	ref, isRef := item.Expr.(ColumnRef)
	if isRef && item.column != nil {
		if index, ok := groupedArrayIndexCached(frame, ref.Name, array); ok {
			if ids, err := rowToGroupFromIndex(index); err == nil && len(ids) == array.Len() {
				return ids, len(index.Rows), true
			}
		}
	}
	ids, count, err := fbyGroupIDs(array)
	if err != nil || len(ids) != array.Len() {
		return nil, 0, false
	}
	return ids, count, true
}

// groupedArrayIndexCached borrows the column's unique/grouped ArrayIndex when
// present, building and attaching one onto the frame's column otherwise so
// repeated queries against the same frame reuse it.
func groupedArrayIndexCached(frame Frame, name Symbol, array Array) (ArrayIndex, bool) {
	if index, ok := arrayIndexForBorrowed(array, ArrayAttributeUnique); ok {
		return index, true
	}
	if index, ok := arrayIndexForBorrowed(array, ArrayAttributeGrouped); ok {
		return index, true
	}
	indexed := WithArrayAttribute(array, ArrayAttributeGrouped)
	if index, ok := arrayIndexForBorrowed(indexed, ArrayAttributeGrouped); ok {
		frame.columns[name] = indexed
		return index, true
	}
	return ArrayIndex{}, false
}

// groupRowIDsCached combines per-column cached group ids positionally into
// composite ids, mirroring arraysRowGroupIDs but with index reuse.
func groupRowIDsCached(frame Frame, byInputs []groupInput, byArrays []Array, rows int) ([]int, int, bool) {
	if len(byArrays) == 1 {
		return groupColumnRowIDs(frame, byInputs[0], byArrays[0])
	}
	perColumn := make([][]int, len(byArrays))
	counts := make([]int, len(byArrays))
	for i, array := range byArrays {
		ids, count, ok := groupColumnRowIDs(frame, byInputs[i], array)
		if !ok {
			return nil, 0, false
		}
		perColumn[i], counts[i] = ids, count
	}
	return combineRowGroupIDs(perColumn, counts, rows)
}

// execGroupedTypedRowIDs executes grouped aggregates through positional group
// ids. allRows=true aggregates every frame row; otherwise only the rows in
// indexes (in order) participate. ok=false means the caller must fall back.
func execGroupedTypedRowIDs(frame Frame, indexes []int, allRows bool, byInputs []groupInput, aggs []aggregateInput) (Frame, bool, error) {
	rows := frame.Len()
	if rows == 0 || len(byInputs) == 0 || len(aggs) == 0 {
		return Frame{}, false, nil
	}
	for _, agg := range aggs {
		if !typedRowIDsAggSupported(agg) {
			return Frame{}, false, nil
		}
	}
	byArrays, ok := groupByArrays(frame, byInputs)
	if !ok {
		return Frame{}, false, nil
	}
	ids, groupCount, ok := groupRowIDsCached(frame, byInputs, byArrays, rows)
	if !ok {
		return Frame{}, false, nil
	}

	accs := make([]typedRowIDsAccumulator, len(aggs))
	for i, agg := range aggs {
		accs[i].input = agg
		switch agg.Func {
		case "sum", "avg", "var", "dev":
			accs[i].valueFn = resolveAggNumericFn(agg)
		}
		switch agg.Func {
		case "count":
			accs[i].count = make([]int64, groupCount)
		case "sum", "avg":
			accs[i].sum = make([]float64, groupCount)
			accs[i].count = make([]int64, groupCount)
		case "var", "dev":
			accs[i].sum = make([]float64, groupCount)
			accs[i].sumsq = make([]float64, groupCount)
			accs[i].count = make([]int64, groupCount)
		case "first":
			accs[i].firstRow = make([]int, groupCount)
			for g := range accs[i].firstRow {
				accs[i].firstRow[g] = -1
			}
		case "last":
			accs[i].lastRow = make([]int, groupCount)
		}
	}

	// Compact group ids to filtered first-appearance order, mirroring the
	// boxed map-insertion ordering, while accumulating in the same pass.
	remap := make([]int, groupCount)
	for i := range remap {
		remap[i] = -1
	}
	repRows := make([]int, 0, groupCount)
	accumulate := func(row int) error {
		id := ids[row]
		g := remap[id]
		if g < 0 {
			g = len(repRows)
			remap[id] = g
			repRows = append(repRows, row)
		}
		for i := range accs {
			acc := &accs[i]
			switch acc.input.Func {
			case "count":
				acc.count[g]++
			case "sum", "avg", "var", "dev":
				var value float64
				var ok bool
				if acc.valueFn != nil {
					value, ok = acc.valueFn(row)
				} else {
					var err error
					value, ok, err = aggregateIndexedNumericValue(acc.input, row)
					if err != nil {
						return err
					}
				}
				if !ok {
					continue
				}
				acc.sum[g] += value
				if acc.sumsq != nil {
					acc.sumsq[g] += value * value
				}
				acc.count[g]++
			case "first":
				if acc.firstRow[g] < 0 {
					acc.firstRow[g] = row
				}
			case "last":
				acc.lastRow[g] = row
			}
		}
		return nil
	}
	if allRows {
		for row := 0; row < rows; row++ {
			if err := accumulate(row); err != nil {
				return Frame{}, true, err
			}
		}
	} else {
		for _, row := range indexes {
			if row < 0 || row >= rows {
				return Frame{}, false, nil
			}
			if err := accumulate(row); err != nil {
				return Frame{}, true, err
			}
		}
	}

	outGroups := len(repRows)
	cols := make([]Column, 0, len(byInputs)+len(aggs))
	for i, item := range byInputs {
		cols = append(cols, Column{Name: item.Name, Data: byArrays[i].Gather(repRows)})
	}
	for i := range accs {
		acc := &accs[i]
		agg := acc.input
		switch agg.Func {
		case "count":
			cols = append(cols, Column{Name: agg.Name, Data: columnArray[int64]{kind: KindI64, data: acc.count[:outGroups]}})
		case "sum":
			cols = append(cols, Column{Name: agg.Name, Data: columnArray[float64]{kind: KindF64, data: acc.sum[:outGroups]}})
		case "avg":
			values := make([]float64, outGroups)
			for g := 0; g < outGroups; g++ {
				if acc.count[g] > 0 {
					values[g] = acc.sum[g] / float64(acc.count[g])
				}
			}
			cols = append(cols, Column{Name: agg.Name, Data: columnArray[float64]{kind: KindF64, data: values}})
		case "var", "dev":
			values := make([]float64, outGroups)
			for g := 0; g < outGroups; g++ {
				if acc.count[g] == 0 {
					continue
				}
				mean := acc.sum[g] / float64(acc.count[g])
				v := acc.sumsq[g]/float64(acc.count[g]) - mean*mean
				if agg.Func == "dev" {
					v = math.Sqrt(v)
				}
				values[g] = v
			}
			cols = append(cols, Column{Name: agg.Name, Data: columnArray[float64]{kind: KindF64, data: values}})
		case "first":
			gathered := acc.firstRow[:outGroups]
			cols = append(cols, Column{Name: agg.Name, Data: agg.column.Gather(gathered)})
		case "last":
			gathered := acc.lastRow[:outGroups]
			cols = append(cols, Column{Name: agg.Name, Data: agg.column.Gather(gathered)})
		}
	}
	out, err := newFrameTrusted(cols...)
	return out, true, err
}

// listAggregateTypedArrayValue reduces a typed nested-array cell (as produced
// by the typed window-join gather and the group pipeline) without boxing each
// element. handled=false defers to the boxed list-aggregate loop.
func listAggregateTypedArrayValue(fn string, arr Array) (any, bool, error) {
	switch a := unwrapAttributedArray(arr).(type) {
	case columnArray[float64]:
		return listAggregateNumericSlice(fn, a.data, false)
	case columnArray[int64]:
		return listAggregateNumericSlice(fn, a.data, true)
	default:
		return nil, false, nil
	}
}

func listAggregateNumericSlice[T int64 | float64](fn string, values []T, intMinMax bool) (any, bool, error) {
	switch fn {
	case "count":
		return int64(len(values)), true, nil
	case "first":
		if len(values) == 0 {
			return NullValue, true, nil
		}
		return values[0], true, nil
	case "last":
		if len(values) == 0 {
			return NullValue, true, nil
		}
		return values[len(values)-1], true, nil
	case "sum":
		var sum float64
		for _, v := range values {
			sum += float64(v)
		}
		return sum, true, nil
	case "avg", "var", "dev":
		if len(values) == 0 {
			return NullValue, true, nil
		}
		var sum, sumsq float64
		for _, v := range values {
			f := float64(v)
			sum += f
			sumsq += f * f
		}
		count := float64(len(values))
		mean := sum / count
		switch fn {
		case "avg":
			return mean, true, nil
		case "var":
			return sumsq/count - mean*mean, true, nil
		default:
			return math.Sqrt(sumsq/count - mean*mean), true, nil
		}
	case "min", "max":
		// Float min/max keeps the boxed compare semantics (NaN ordering).
		if !intMinMax {
			return nil, false, nil
		}
		if len(values) == 0 {
			return NullValue, true, nil
		}
		best := values[0]
		for _, v := range values[1:] {
			if (fn == "min" && v < best) || (fn == "max" && v > best) {
				best = v
			}
		}
		return best, true, nil
	default:
		return nil, false, nil
	}
}
