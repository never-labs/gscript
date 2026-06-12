package data

// Typed whole-table (no-by) aggregate execution.
//
// The generic ungrouped path accumulates one boxed value per aggregate per
// row (aggregateInput.value / aggregateNumericValue), which costs an
// interface allocation and a kind switch per cell. For the dominant aggregate
// family — count / sum / avg / var / dev / wavg / min / max over dense
// numeric columns — the whole query collapses into straight column
// reductions over the same dense []float64 views the fused grouped kernel
// uses (zero-copy for f64 columns, pooled conversion otherwise).
//
// The typed path fills the exact same aggregateState fields the generic
// per-row loop would produce, so result construction (aggregateResult +
// aggregateOutputColumn) is shared and output parity is structural.

// ungroupedAggregateStatesTyped computes every aggregate state with typed
// column reductions. ok=false leaves states untouched and defers to the
// generic per-row loop with no side effects.
func ungroupedAggregateStatesTyped(frame Frame, indexes []int, aggs []aggregateInput, states []aggregateState) bool {
	rows := frame.Len()
	if indexes != nil {
		for _, row := range indexes {
			if row < 0 || row >= rows {
				return false
			}
		}
	}

	// Phase 1: resolve every aggregate's dense sources up front so failure on
	// a later aggregate cannot leave earlier states half-accumulated.
	type ungroupedSrc struct {
		src    []float64
		owned  bool
		wsrc   []float64
		wowned bool
	}
	srcs := make([]ungroupedSrc, len(aggs))
	release := func(upto int) {
		for i := 0; i < upto; i++ {
			bulkF64Release(srcs[i].src, srcs[i].owned)
			bulkF64Release(srcs[i].wsrc, srcs[i].wowned)
		}
	}
	for i, agg := range aggs {
		if agg.Weight != nil && agg.Func != "wavg" {
			release(i)
			return false
		}
		switch agg.Func {
		case "count":
			// No source needed.
		case "sum", "avg", "var", "dev":
			src, owned, ok := ungroupedAggF64Source(agg, rows)
			if !ok {
				release(i)
				return false
			}
			srcs[i].src, srcs[i].owned = src, owned
		case "wavg":
			if agg.weightColumn == nil {
				release(i)
				return false
			}
			src, owned, ok := ungroupedAggF64Source(agg, rows)
			if !ok {
				release(i)
				return false
			}
			srcs[i].src, srcs[i].owned = src, owned
			wsrc, wowned, ok := fusedGroupSource(agg.weightColumn, rows)
			if !ok {
				bulkF64Release(src, owned)
				release(i)
				return false
			}
			srcs[i].wsrc, srcs[i].wowned = wsrc, wowned
		case "min", "max":
			if agg.column == nil || agg.column.Len() != rows {
				release(i)
				return false
			}
			switch unwrapAttributedArray(agg.column).(type) {
			case columnArray[int64], columnArray[float64]:
				// Typed reduction below; raw values match agg.value's boxing.
			default:
				release(i)
				return false
			}
		default:
			release(i)
			return false
		}
	}

	// Phase 2: reduce.
	n := rows
	if indexes != nil {
		n = len(indexes)
	}
	for i := range aggs {
		state := &states[i]
		switch aggs[i].Func {
		case "count":
			state.count = int64(n)
		case "sum", "avg":
			state.sum = ungroupedSumF64(srcs[i].src, indexes)
			state.count = int64(n)
		case "var", "dev":
			state.sum, state.sumsq = ungroupedSumSqF64(srcs[i].src, indexes)
			state.count = int64(n)
		case "wavg":
			state.sum, state.weight = ungroupedWeightedSumF64(srcs[i].src, srcs[i].wsrc, indexes)
			state.count = int64(n)
		case "min", "max":
			ungroupedMinMaxTyped(state, aggs[i].column, indexes, aggs[i].Func == "max")
		}
	}
	release(len(aggs))
	return true
}

// ungroupedAggF64Source resolves an aggregate's value expression to a dense
// rows-length []float64 view: a plain column reference or a fused dense
// binary (add/sub/mul) expression, matching the fused grouped kernel's
// source domain.
func ungroupedAggF64Source(agg aggregateInput, rows int) ([]float64, bool, bool) {
	if agg.column != nil {
		return fusedGroupSource(agg.column, rows)
	}
	if agg.leftColumn != nil && agg.rightColumn != nil {
		src, ok := fusedGroupBinarySource(agg, rows)
		return src, true, ok
	}
	return nil, false, false
}

func ungroupedSumF64(src []float64, indexes []int) float64 {
	var sum float64
	if indexes == nil {
		for _, v := range src {
			sum += v
		}
		return sum
	}
	for _, row := range indexes {
		sum += src[row]
	}
	return sum
}

func ungroupedSumSqF64(src []float64, indexes []int) (sum, sumsq float64) {
	if indexes == nil {
		for _, v := range src {
			sum += v
			sumsq += v * v
		}
		return sum, sumsq
	}
	for _, row := range indexes {
		v := src[row]
		sum += v
		sumsq += v * v
	}
	return sum, sumsq
}

func ungroupedWeightedSumF64(src, weights []float64, indexes []int) (sum, weight float64) {
	if indexes == nil {
		for row, v := range src {
			w := weights[row]
			sum += v * w
			weight += w
		}
		return sum, weight
	}
	for _, row := range indexes {
		w := weights[row]
		sum += src[row] * w
		weight += w
	}
	return sum, weight
}

// ungroupedMinMaxTyped reduces min/max over a dense i64/f64 column,
// producing the same raw boxed extreme the generic agg.value loop yields.
func ungroupedMinMaxTyped(state *aggregateState, col Array, indexes []int, isMax bool) {
	switch c := unwrapAttributedArray(col).(type) {
	case columnArray[int64]:
		best, ok := ungroupedExtremum(c.data, indexes, isMax)
		if ok {
			state.value, state.hasValue = best, true
		}
	case columnArray[float64]:
		best, ok := ungroupedExtremum(c.data, indexes, isMax)
		if ok {
			state.value, state.hasValue = best, true
		}
	}
}

func ungroupedExtremum[T int64 | float64](data []T, indexes []int, isMax bool) (T, bool) {
	if indexes == nil {
		if len(data) == 0 {
			var zero T
			return zero, false
		}
		best := data[0]
		if isMax {
			for _, v := range data[1:] {
				if v > best {
					best = v
				}
			}
		} else {
			for _, v := range data[1:] {
				if v < best {
					best = v
				}
			}
		}
		return best, true
	}
	if len(indexes) == 0 {
		var zero T
		return zero, false
	}
	best := data[indexes[0]]
	if isMax {
		for _, row := range indexes[1:] {
			if v := data[row]; v > best {
				best = v
			}
		}
	} else {
		for _, row := range indexes[1:] {
			if v := data[row]; v < best {
				best = v
			}
		}
	}
	return best, true
}
