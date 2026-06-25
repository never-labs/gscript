package data

import (
	"math"
	"sort"
)

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
	weightFn func(row int) (float64, bool)
	sum      []float64
	sumsq    []float64
	weight   []float64
	count    []int64
	firstRow []int
	lastRow  []int
	// bestRow tracks per-group argmin/argmax rows so min/max gather the
	// source-typed value without boxing; med holds per-group medians.
	bestRow []int
	med     []float64
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

// groupAggF64Slice exposes a plain dense float64 column's storage for direct
// aggregation loops (always-present values, no per-row reader call).
func groupAggF64Slice(col Array) ([]float64, bool) {
	if a, ok := unwrapAttributedArray(col).(columnArray[float64]); ok {
		return a.data, true
	}
	return nil, false
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
		// integer space before converting. The mixed f64/i64 products fuse
		// both operand reads into one closure instead of chaining two
		// per-operand readers per row.
		switch l := unwrapAttributedArray(agg.leftColumn).(type) {
		case columnArray[int64]:
			switch r := unwrapAttributedArray(agg.rightColumn).(type) {
			case columnArray[int64]:
				ldata, rdata := l.data, r.data
				return func(row int) (float64, bool) { return float64(ldata[row] * rdata[row]), true }
			case columnArray[float64]:
				ldata, rdata := l.data, r.data
				return func(row int) (float64, bool) { return float64(ldata[row]) * rdata[row], true }
			}
		case columnArray[float64]:
			switch r := unwrapAttributedArray(agg.rightColumn).(type) {
			case columnArray[int64]:
				ldata, rdata := l.data, r.data
				return func(row int) (float64, bool) { return ldata[row] * float64(rdata[row]), true }
			case columnArray[float64]:
				ldata, rdata := l.data, r.data
				return func(row int) (float64, bool) { return ldata[row] * rdata[row], true }
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
	switch agg.Func {
	case "count":
		return agg.Weight == nil
	case "sum", "avg", "var", "dev":
		if agg.Weight != nil {
			return false
		}
		if agg.column != nil {
			return isNumericArray(agg.column)
		}
		if agg.leftColumn != nil && agg.rightColumn != nil {
			return isNumericBinaryAggregateOp(agg.binaryOp) && isNumericArray(agg.leftColumn) && isNumericArray(agg.rightColumn)
		}
		return false
	case "wavg":
		return agg.Weight != nil && agg.weightColumn != nil && isNumericArray(agg.weightColumn) &&
			((agg.column != nil && isNumericArray(agg.column)) ||
				(agg.leftColumn != nil && agg.rightColumn != nil && isNumericBinaryAggregateOp(agg.binaryOp) && isNumericArray(agg.leftColumn) && isNumericArray(agg.rightColumn)))
	case "first", "last":
		return agg.Weight == nil && agg.column != nil
	case "min", "max", "med":
		if agg.Weight != nil {
			return false
		}
		if agg.column == nil {
			return false
		}
		switch unwrapAttributedArray(agg.column).(type) {
		case columnArray[float64], columnArray[int64]:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// groupAggI64Slice exposes a plain dense int64 column's storage for direct
// aggregation loops.
func groupAggI64Slice(col Array) ([]int64, bool) {
	if a, ok := unwrapAttributedArray(col).(columnArray[int64]); ok {
		return a.data, true
	}
	return nil, false
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

// groupCompactRowIDs assigns each participating row its compact group id in
// first-appearance order. Per-column ids come from the cached grouped
// ArrayIndex (no per-call allocation); composite keys are folded inline
// through a flat slate over the small composite domain, so multi-key groups
// allocate only the per-row gid vector instead of a combined full-frame id
// vector per key column.
func groupCompactRowIDs(frame Frame, byInputs []groupInput, byArrays []Array, rows int, indexes []int, allRows bool) ([]int32, []int, bool) {
	perColumn := make([][]int, len(byArrays))
	counts := make([]int, len(byArrays))
	for i, array := range byArrays {
		ids, count, ok := groupColumnRowIDs(frame, byInputs[i], array)
		if !ok {
			return nil, nil, false
		}
		perColumn[i], counts[i] = ids, count
	}

	n := rows
	if !allRows {
		n = len(indexes)
		for _, row := range indexes {
			if row < 0 || row >= rows {
				return nil, nil, false
			}
		}
	}

	// Composite domain: product of per-column group counts, used as the slate
	// size. Large or overflowing domains fall back to the pairwise combiner.
	domain := int64(1)
	for _, count := range counts {
		if count <= 0 {
			domain = 0
			break
		}
		domain *= int64(count)
		if domain > 1<<22 {
			break
		}
	}
	useSlate := domain > 0 && domain <= 1<<22 && (domain <= int64(rows)*4 || domain <= 1<<12)
	var compositeIDs []int
	if len(perColumn) > 1 && !useSlate {
		combined, _, ok := combineRowGroupIDs(perColumn, counts, rows)
		if !ok {
			return nil, nil, false
		}
		compositeIDs = combined
		domain = int64(rows)
		useSlate = false
	}

	// gids is fully overwritten below; the slates rely on zero initial state,
	// so pooled slates are cleared explicitly (alloc+GC saved, memclr kept).
	gids := bulkI32Get(n)
	repRows := make([]int, 0, 16)
	switch {
	case len(perColumn) == 1 || compositeIDs != nil:
		ids := perColumn[0]
		slateSize := counts[0]
		if compositeIDs != nil {
			ids = compositeIDs
			slateSize = rows
		}
		slate := bulkI32Get(slateSize)
		clear(slate)
		if allRows {
			for row := 0; row < rows; row++ {
				g := slate[ids[row]]
				if g == 0 {
					repRows = append(repRows, row)
					g = int32(len(repRows))
					slate[ids[row]] = g
				}
				gids[row] = g - 1
			}
		} else {
			for k, row := range indexes {
				g := slate[ids[row]]
				if g == 0 {
					repRows = append(repRows, row)
					g = int32(len(repRows))
					slate[ids[row]] = g
				}
				gids[k] = g - 1
			}
		}
		bulkI32Release(slate)
	default:
		slate := bulkI32Get(int(domain))
		clear(slate)
		if allRows {
			for row := 0; row < rows; row++ {
				key := perColumn[0][row]
				for ki := 1; ki < len(perColumn); ki++ {
					key = key*counts[ki] + perColumn[ki][row]
				}
				g := slate[key]
				if g == 0 {
					repRows = append(repRows, row)
					g = int32(len(repRows))
					slate[key] = g
				}
				gids[row] = g - 1
			}
		} else {
			for k, row := range indexes {
				key := perColumn[0][row]
				for ki := 1; ki < len(perColumn); ki++ {
					key = key*counts[ki] + perColumn[ki][row]
				}
				g := slate[key]
				if g == 0 {
					repRows = append(repRows, row)
					g = int32(len(repRows))
					slate[key] = g
				}
				gids[k] = g - 1
			}
		}
		bulkI32Release(slate)
	}
	return gids, repRows, true
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

	// Compact group ids to filtered first-appearance order, mirroring the
	// boxed map-insertion ordering. Phase 1 resolves each participating row's
	// compact group id once (folding the composite-key combine into the same
	// pass, so multi-key groups never materialize a full-frame combined id
	// vector); phase 2 runs one tight monomorphic loop per accumulator
	// instead of a per-row closure with a per-aggregate string switch.
	gids, repRows, ok := groupCompactRowIDs(frame, byInputs, byArrays, rows, indexes, allRows)
	if !ok {
		return Frame{}, false, nil
	}
	// gids is a transient per-row vector; nothing below retains it (outputs
	// gather through repRows / accumulator slices), so it goes back to the
	// bulk pool when this call finishes.
	defer bulkI32Release(gids)
	groupCount := len(repRows)

	accs := make([]typedRowIDsAccumulator, len(aggs))
	for i, agg := range aggs {
		accs[i].input = agg
		switch agg.Func {
		case "sum", "avg", "var", "dev", "wavg":
			accs[i].valueFn = resolveAggNumericFn(agg)
			if agg.Func == "wavg" {
				accs[i].weightFn = resolveNumericColumnFn(agg.weightColumn)
			}
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
		case "wavg":
			accs[i].sum = make([]float64, groupCount)
			accs[i].weight = make([]float64, groupCount)
			accs[i].count = make([]int64, groupCount)
		case "first":
			accs[i].firstRow = make([]int, groupCount)
			for g := range accs[i].firstRow {
				accs[i].firstRow[g] = -1
			}
		case "last":
			accs[i].lastRow = make([]int, groupCount)
		case "min", "max":
			accs[i].bestRow = make([]int, groupCount)
			for g := range accs[i].bestRow {
				accs[i].bestRow[g] = -1
			}
		}
	}
	rowAt := func(k int) int {
		if allRows {
			return k
		}
		return indexes[k]
	}
	for i := range accs {
		acc := &accs[i]
		switch acc.input.Func {
		case "count":
			for _, g := range gids {
				acc.count[g]++
			}
		case "sum", "avg", "var", "dev":
			if acc.input.column != nil {
				// Plain dense numeric columns accumulate straight off the
				// typed slice; no per-row reader call at all.
				if data, ok := groupAggF64Slice(acc.input.column); ok {
					if allRows {
						for row, g := range gids {
							value := data[row]
							acc.sum[g] += value
							if acc.sumsq != nil {
								acc.sumsq[g] += value * value
							}
							acc.count[g]++
						}
					} else {
						for k, g := range gids {
							value := data[indexes[k]]
							acc.sum[g] += value
							if acc.sumsq != nil {
								acc.sumsq[g] += value * value
							}
							acc.count[g]++
						}
					}
					continue
				}
			}
			if acc.valueFn != nil {
				fn := acc.valueFn
				if allRows {
					for row, g := range gids {
						value, ok := fn(row)
						if !ok {
							continue
						}
						acc.sum[g] += value
						if acc.sumsq != nil {
							acc.sumsq[g] += value * value
						}
						acc.count[g]++
					}
				} else {
					for k, g := range gids {
						value, ok := fn(indexes[k])
						if !ok {
							continue
						}
						acc.sum[g] += value
						if acc.sumsq != nil {
							acc.sumsq[g] += value * value
						}
						acc.count[g]++
					}
				}
				continue
			}
			for k, g := range gids {
				value, ok, err := aggregateIndexedNumericValue(acc.input, rowAt(k))
				if err != nil {
					return Frame{}, true, err
				}
				if !ok {
					continue
				}
				acc.sum[g] += value
				if acc.sumsq != nil {
					acc.sumsq[g] += value * value
				}
				acc.count[g]++
			}
		case "wavg":
			if acc.valueFn == nil || acc.weightFn == nil {
				return Frame{}, false, nil
			}
			valueFn, weightFn := acc.valueFn, acc.weightFn
			for k, g := range gids {
				row := rowAt(k)
				value, vok := valueFn(row)
				weight, wok := weightFn(row)
				if !vok || !wok {
					continue
				}
				acc.sum[g] += value * weight
				acc.weight[g] += weight
				acc.count[g]++
			}
		case "first":
			if allRows {
				for row, g := range gids {
					if acc.firstRow[g] < 0 {
						acc.firstRow[g] = row
					}
				}
			} else {
				for k, g := range gids {
					if acc.firstRow[g] < 0 {
						acc.firstRow[g] = indexes[k]
					}
				}
			}
		case "last":
			if allRows {
				for row, g := range gids {
					acc.lastRow[g] = row
				}
			} else {
				for k, g := range gids {
					acc.lastRow[g] = indexes[k]
				}
			}
		case "min", "max":
			isMin := acc.input.Func == "min"
			if data, ok := groupAggF64Slice(acc.input.column); ok {
				for k, g := range gids {
					row := rowAt(k)
					best := acc.bestRow[g]
					if best < 0 {
						acc.bestRow[g] = row
						continue
					}
					v, bv := data[row], data[best]
					if (isMin && v < bv) || (!isMin && v > bv) {
						acc.bestRow[g] = row
					}
				}
				continue
			}
			if data, ok := groupAggI64Slice(acc.input.column); ok {
				for k, g := range gids {
					row := rowAt(k)
					best := acc.bestRow[g]
					if best < 0 {
						acc.bestRow[g] = row
						continue
					}
					v, bv := data[row], data[best]
					if (isMin && v < bv) || (!isMin && v > bv) {
						acc.bestRow[g] = row
					}
				}
				continue
			}
			return Frame{}, false, nil
		case "med":
			// Bucket every participating value into one flat slice with
			// per-group extents, then take each group's median in place.
			counts := make([]int, groupCount)
			for _, g := range gids {
				counts[g]++
			}
			offsets := make([]int, groupCount+1)
			for g, c := range counts {
				offsets[g+1] = offsets[g] + c
			}
			flat := make([]float64, len(gids))
			fill := make([]int, groupCount)
			copy(fill, offsets[:groupCount])
			if data, ok := groupAggF64Slice(acc.input.column); ok {
				for k, g := range gids {
					flat[fill[g]] = data[rowAt(k)]
					fill[g]++
				}
			} else if data, ok := groupAggI64Slice(acc.input.column); ok {
				for k, g := range gids {
					flat[fill[g]] = float64(data[rowAt(k)])
					fill[g]++
				}
			} else {
				return Frame{}, false, nil
			}
			acc.med = make([]float64, groupCount)
			for g := 0; g < groupCount; g++ {
				values := flat[offsets[g]:offsets[g+1]]
				sort.Float64s(values)
				mid := len(values) / 2
				if len(values)%2 == 1 {
					acc.med[g] = values[mid]
				} else {
					acc.med[g] = (values[mid-1] + values[mid]) / 2
				}
			}
		}
	}

	outGroups := len(repRows)
	cols := make([]Column, 0, len(byInputs)+len(aggs))
	for i, item := range byInputs {
		// Gather the unwrapped storage: a grouped/unique attribute index is a
		// fact about the full input column, and rebuilding it for the tiny
		// per-group representative output costs more than it can ever save.
		cols = append(cols, Column{Name: item.Name, Data: unwrapAttributedArray(byArrays[i]).Gather(repRows)})
	}
	for i := range accs {
		acc := &accs[i]
		agg := acc.input
		switch agg.Func {
		case "count":
			cols = append(cols, Column{Name: agg.Name, Data: columnArray[int64]{kind: KindI64, data: acc.count[:outGroups]}})
		case "sum":
			cols = append(cols, sumOutputColumnFromF64(frame, agg.Aggregate, acc.sum[:outGroups]))
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
		case "wavg":
			cols = append(cols, groupedWavgOutputColumn(agg.Name, acc.sum[:outGroups], acc.weight[:outGroups], acc.count[:outGroups]))
		case "first":
			gathered := acc.firstRow[:outGroups]
			cols = append(cols, Column{Name: agg.Name, Data: agg.column.Gather(gathered)})
		case "last":
			gathered := acc.lastRow[:outGroups]
			cols = append(cols, Column{Name: agg.Name, Data: agg.column.Gather(gathered)})
		case "min", "max":
			gathered := acc.bestRow[:outGroups]
			cols = append(cols, Column{Name: agg.Name, Data: unwrapAttributedArray(agg.column).Gather(gathered)})
		case "med":
			cols = append(cols, Column{Name: agg.Name, Data: columnArray[float64]{kind: KindF64, data: acc.med[:outGroups]}})
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
