package data

import "math"

// Fused single-pass grouped aggregation for sum-family aggregate sets.
//
// The generic grouped pipeline resolves per-row group ids into a gid vector
// and then runs one accumulation loop per aggregate. That costs one full
// per-row pass per aggregate plus the gid materialization. For the most
// common aggregate family — count / sum / avg / var / dev over dense numeric
// sources — every accumulator is "shared row count plus one or two
// sum(src[row]) streams", so the whole query collapses into a single fused
// pass: resolve the composite group id arithmetically from the cached
// per-column dictionary codes (the same fby/ArrayIndex codes
// groupCompactRowIDs uses) and update every sum slot inline.
//
// The fused loop is deliberately monomorphic: slots are nil-guarded slices,
// not closures or kind switches, because per-row indirect dispatch measures
// 2-3x slower than straight-line code on Apple Silicon.
//
// Sources are flattened to dense []float64 views through the bulk pool
// (zero-copy for plain f64 columns, pooled conversion for integer columns,
// pooled fused products for binary mul/add/sub aggregates), and every pooled
// buffer is released before returning.

// fusedGroupMaxSumSlots bounds the inline sum slots; aggregate sets needing
// more fall back to the per-aggregate loops.
const fusedGroupMaxSumSlots = 3

// fusedSumSlot is one "sum of src[row] per group" accumulation stream.
// needSq additionally accumulates src[row]² inline for var/dev consumers.
type fusedSumSlot struct {
	src    []float64
	owned  bool // src is a pooled buffer this pass must release
	needSq bool
	sums   []float64
	sqsums []float64
}

// fusedAggPlan maps one aggregate to its output recipe over the shared row
// count and the sum slots.
type fusedAggPlan struct {
	fn   string // count, sum, avg, var, dev, wavg
	name Symbol
	slot int // primary sum slot (-1 for count); weight*value product for wavg
	// wslot is the weight sum slot for wavg (-1 otherwise): the per-group
	// result is sums[slot]/sums[wslot].
	wslot int
}

// fusedGroupSource returns a dense []float64 view of an aggregate source
// column, with ownership flag, or ok=false when the carrier is unsupported.
func fusedGroupSource(col Array, rows int) ([]float64, bool, bool) {
	if col == nil || col.Len() != rows || !isNumericArray(col) || arrayHasNullFast(col) {
		return nil, false, false
	}
	values, owned, ok := tryBulkF64Values(col)
	if !ok || len(values) != rows {
		if owned {
			bulkF64Release(values, owned)
		}
		return nil, false, false
	}
	return values, owned, true
}

// arrayHasNullFast reports whether the array can carry nulls. Plain dense
// columns cannot; anything else defers to the generic path.
func arrayHasNullFast(col Array) bool {
	switch unwrapAttributedArray(col).(type) {
	case columnArray[float64], columnArray[float32],
		columnArray[int64], columnArray[int32], columnArray[int16], columnArray[int8]:
		return false
	default:
		return true
	}
}

// fusedGroupBinarySource computes a pooled dense source for a binary
// aggregate expression (left op right) over dense numeric columns. Division
// is excluded: its divide-by-zero skip semantics cannot be expressed as an
// unconditional sum stream. The common typed multiply pairs fuse both
// operand reads into one pass, mirroring resolveAggNumericFn's semantics
// (i64*i64 multiplies in integer space before converting).
func fusedGroupBinarySource(agg aggregateInput, rows int) ([]float64, bool) {
	if agg.binaryOp != OpAdd && agg.binaryOp != OpSub && agg.binaryOp != OpMul {
		return nil, false
	}
	if agg.binaryOp == OpMul && agg.leftColumn.Len() == rows && agg.rightColumn.Len() == rows {
		switch l := unwrapAttributedArray(agg.leftColumn).(type) {
		case columnArray[float64]:
			switch r := unwrapAttributedArray(agg.rightColumn).(type) {
			case columnArray[float64]:
				out := bulkF64Get(rows)
				for i := range out {
					out[i] = l.data[i] * r.data[i]
				}
				return out, true
			case columnArray[int64]:
				out := bulkF64Get(rows)
				for i := range out {
					out[i] = l.data[i] * float64(r.data[i])
				}
				return out, true
			}
		case columnArray[int64]:
			switch r := unwrapAttributedArray(agg.rightColumn).(type) {
			case columnArray[float64]:
				out := bulkF64Get(rows)
				for i := range out {
					out[i] = float64(l.data[i]) * r.data[i]
				}
				return out, true
			case columnArray[int64]:
				out := bulkF64Get(rows)
				for i := range out {
					out[i] = float64(l.data[i] * r.data[i])
				}
				return out, true
			}
		}
	}
	left, leftOwned, ok := fusedGroupSource(agg.leftColumn, rows)
	if !ok {
		return nil, false
	}
	right, rightOwned, ok := fusedGroupSource(agg.rightColumn, rows)
	if !ok {
		bulkF64Release(left, leftOwned)
		return nil, false
	}
	out := bulkF64Get(rows)
	switch agg.binaryOp {
	case OpAdd:
		for i := range out {
			out[i] = left[i] + right[i]
		}
	case OpSub:
		for i := range out {
			out[i] = left[i] - right[i]
		}
	default:
		for i := range out {
			out[i] = left[i] * right[i]
		}
	}
	bulkF64Release(left, leftOwned)
	bulkF64Release(right, rightOwned)
	return out, true
}

// fusedWavgProductSource computes the pooled weight*value product stream for
// a wavg aggregate. Both operands convert to float64 before multiplying,
// mirroring the generic per-row accumulation (state.sum += n * w with n and w
// from numericAt), so integer columns do not multiply in integer space.
func fusedWavgProductSource(value, weight Array, rows int) ([]float64, bool) {
	left, leftOwned, ok := fusedGroupSource(value, rows)
	if !ok {
		return nil, false
	}
	right, rightOwned, ok := fusedGroupSource(weight, rows)
	if !ok {
		bulkF64Release(left, leftOwned)
		return nil, false
	}
	out := bulkF64Get(rows)
	for i := range out {
		out[i] = left[i] * right[i]
	}
	bulkF64Release(left, leftOwned)
	bulkF64Release(right, rightOwned)
	return out, true
}

// fusedSlotIndex returns the slot index for src, reusing an existing slot
// when the same backing storage is already registered.
func fusedSlotIndex(slots []fusedSumSlot, src []float64, owned bool) ([]fusedSumSlot, int) {
	for i, slot := range slots {
		if len(slot.src) == len(src) && len(src) > 0 && &slot.src[0] == &src[0] {
			if owned {
				bulkF64Release(src, owned)
			}
			return slots, i
		}
	}
	return append(slots, fusedSumSlot{src: src, owned: owned}), len(slots)
}

// fusedReleaseSlots returns every pooled source buffer to the bulk pool.
func fusedReleaseSlots(slots []fusedSumSlot) {
	for _, slot := range slots {
		bulkF64Release(slot.src, slot.owned)
	}
}

func appendFusedGroupKeyColumns(cols []Column, frame Frame, allRows bool, byInputs []groupInput, byArrays []Array, repRows []int) ([]Column, error) {
	if allRows && len(byInputs) == 1 && len(byArrays) == 1 && byInputs[0].column != nil {
		if ref, ok := byInputs[0].Expr.(ColumnRef); ok {
			if index, ok := groupedArrayIndexCached(frame, ref.Name, byArrays[0]); ok && len(index.Keys) == len(repRows) {
				if index.KeyArray != nil && index.KeyArray.Kind() == byInputs[0].keyKind() && index.KeyArray.Len() == len(repRows) {
					return append(cols, Column{Name: byInputs[0].Name, Data: index.KeyArray}), nil
				}
				col, handled, err := typedGroupedAggregateKeyColumnIdentity(byInputs[0].Name, byInputs[0].keyKind(), index.Keys)
				if err != nil {
					return nil, err
				}
				if handled {
					return append(cols, col), nil
				}
			}
		}
	}
	for i, item := range byInputs {
		cols = append(cols, Column{Name: item.Name, Data: unwrapAttributedArray(byArrays[i]).Gather(repRows)})
	}
	return cols, nil
}

// execGroupedFusedNumeric executes count/sum/avg/var/dev aggregate sets over
// at most two group-key columns in one fused pass. ok=false defers to the
// generic grouped pipeline with no side effects.
func execGroupedFusedNumeric(frame Frame, indexes []int, allRows bool, byInputs []groupInput, aggs []aggregateInput) (Frame, bool, error) {
	if out, ok, err := execGroupedFusedI64Sums(frame, indexes, allRows, byInputs, aggs); ok || err != nil {
		return out, ok, err
	}
	rows := frame.Len()
	if rows == 0 || len(byInputs) == 0 || len(byInputs) > 2 || len(aggs) == 0 {
		return Frame{}, false, nil
	}
	for _, agg := range aggs {
		switch agg.Func {
		case "count", "sum", "avg", "var", "dev":
			if agg.Weight != nil {
				return Frame{}, false, nil
			}
		case "wavg":
			// wavg fuses as two sum streams: sum(weight*value)/sum(weight)
			// per group, mirroring the generic per-row float accumulation.
			if agg.column == nil || agg.weightColumn == nil {
				return Frame{}, false, nil
			}
		default:
			return Frame{}, false, nil
		}
	}

	// Group-key dictionary codes, cached on the column index sidecars.
	byArrays, ok := groupByArrays(frame, byInputs)
	if !ok {
		return Frame{}, false, nil
	}
	var ids0, ids1 []int
	c1 := 1
	domain := 1
	for i, array := range byArrays {
		ids, count, ok := groupColumnRowIDs(frame, byInputs[i], array)
		if !ok || count <= 0 {
			return Frame{}, false, nil
		}
		if i == 0 {
			ids0 = ids
			domain = count
		} else {
			ids1 = ids
			c1 = count
			if int64(domain)*int64(count) > 1<<22 {
				return Frame{}, false, nil
			}
			domain *= count
		}
	}
	n := rows
	if !allRows {
		n = len(indexes)
		for _, row := range indexes {
			if row < 0 || row >= rows {
				return Frame{}, false, nil
			}
		}
	}
	if domain > 1<<22 || (domain > rows*4 && domain > 1<<12) {
		return Frame{}, false, nil
	}

	// Classify aggregates into sum slots over dense f64 sources.
	var slots []fusedSumSlot
	plans := make([]fusedAggPlan, len(aggs))
	for i, agg := range aggs {
		plans[i] = fusedAggPlan{fn: agg.Func, name: agg.Name, slot: -1, wslot: -1}
		if agg.Func == "count" {
			continue
		}
		if agg.Func == "wavg" {
			src, ok := fusedWavgProductSource(agg.column, agg.weightColumn, rows)
			if !ok {
				fusedReleaseSlots(slots)
				return Frame{}, false, nil
			}
			slots, plans[i].slot = fusedSlotIndex(slots, src, true)
			wsrc, wowned, ok := fusedGroupSource(agg.weightColumn, rows)
			if !ok {
				fusedReleaseSlots(slots)
				return Frame{}, false, nil
			}
			slots, plans[i].wslot = fusedSlotIndex(slots, wsrc, wowned)
			if len(slots) > fusedGroupMaxSumSlots {
				fusedReleaseSlots(slots)
				return Frame{}, false, nil
			}
			continue
		}
		var src []float64
		var owned, ok bool
		if agg.column != nil {
			src, owned, ok = fusedGroupSource(agg.column, rows)
		} else if agg.leftColumn != nil && agg.rightColumn != nil {
			src, ok = fusedGroupBinarySource(agg, rows)
			owned = true
		}
		if !ok {
			fusedReleaseSlots(slots)
			return Frame{}, false, nil
		}
		slots, plans[i].slot = fusedSlotIndex(slots, src, owned)
		if agg.Func == "var" || agg.Func == "dev" {
			slots[plans[i].slot].needSq = true
		}
		if len(slots) > fusedGroupMaxSumSlots {
			fusedReleaseSlots(slots)
			return Frame{}, false, nil
		}
	}

	// Accumulator storage: compact group ids are first-appearance ordered and
	// bounded by min(domain, participating rows).
	capN := domain
	if n < capN {
		capN = n
	}
	slate := bulkI32Get(domain)
	clear(slate)
	count := bulkI64Get(capN)
	clear(count)
	var sum0, sum1, sum2, sq0, sq1, sq2 []float64
	var s0, s1, s2 []float64
	for i := range slots {
		sums := bulkF64Get(capN)
		clear(sums)
		slots[i].sums = sums
		var sqs []float64
		if slots[i].needSq {
			sqs = bulkF64Get(capN)
			clear(sqs)
			slots[i].sqsums = sqs
		}
		switch i {
		case 0:
			s0, sum0, sq0 = slots[i].src, sums, sqs
		case 1:
			s1, sum1, sq1 = slots[i].src, sums, sqs
		case 2:
			s2, sum2, sq2 = slots[i].src, sums, sqs
		}
	}

	repRows := bulkIntGet(capN)
	defer bulkIntRelease(repRows)
	switch {
	case allRows:
		for row := 0; row < rows; row++ {
			key := ids0[row]
			if ids1 != nil {
				key = key*c1 + ids1[row]
			}
			g := slate[key]
			if g == 0 {
				repRows = append(repRows, row)
				g = int32(len(repRows))
				slate[key] = g
			}
			gi := int(g - 1)
			count[gi]++
			if s0 != nil {
				v := s0[row]
				sum0[gi] += v
				if sq0 != nil {
					sq0[gi] += v * v
				}
			}
			if s1 != nil {
				v := s1[row]
				sum1[gi] += v
				if sq1 != nil {
					sq1[gi] += v * v
				}
			}
			if s2 != nil {
				v := s2[row]
				sum2[gi] += v
				if sq2 != nil {
					sq2[gi] += v * v
				}
			}
		}
	default:
		for _, row := range indexes {
			key := ids0[row]
			if ids1 != nil {
				key = key*c1 + ids1[row]
			}
			g := slate[key]
			if g == 0 {
				repRows = append(repRows, row)
				g = int32(len(repRows))
				slate[key] = g
			}
			gi := int(g - 1)
			count[gi]++
			if s0 != nil {
				v := s0[row]
				sum0[gi] += v
				if sq0 != nil {
					sq0[gi] += v * v
				}
			}
			if s1 != nil {
				v := s1[row]
				sum1[gi] += v
				if sq1 != nil {
					sq1[gi] += v * v
				}
			}
			if s2 != nil {
				v := s2[row]
				sum2[gi] += v
				if sq2 != nil {
					sq2[gi] += v * v
				}
			}
		}
	}
	bulkI32Release(slate)

	groups := len(repRows)
	cols := make([]Column, 0, len(byInputs)+len(aggs))
	cols, err := appendFusedGroupKeyColumns(cols, frame, allRows, byInputs, byArrays, repRows)
	if err != nil {
		bulkI64Release(count, true)
		for i := range slots {
			bulkF64Release(slots[i].sums, true)
			if slots[i].sqsums != nil {
				bulkF64Release(slots[i].sqsums, true)
			}
		}
		fusedReleaseSlots(slots)
		return Frame{}, true, err
	}
	for planIdx, plan := range plans {
		switch plan.fn {
		case "count":
			values := make([]int64, groups)
			copy(values, count[:groups])
			cols = append(cols, Column{Name: plan.name, Data: columnArray[int64]{kind: KindI64, data: values}})
		case "sum":
			values := make([]float64, groups)
			copy(values, slots[plan.slot].sums[:groups])
			cols = append(cols, sumOutputColumnFromF64(frame, aggs[planIdx].Aggregate, values))
		case "avg":
			sums := slots[plan.slot].sums
			values := make([]float64, groups)
			for g := 0; g < groups; g++ {
				if count[g] > 0 {
					values[g] = sums[g] / float64(count[g])
				}
			}
			cols = append(cols, Column{Name: plan.name, Data: columnArray[float64]{kind: KindF64, data: values}})
		case "var", "dev":
			sums := slots[plan.slot].sums
			sqs := slots[plan.slot].sqsums
			values := make([]float64, groups)
			for g := 0; g < groups; g++ {
				if count[g] == 0 {
					continue
				}
				mean := sums[g] / float64(count[g])
				v := sqs[g]/float64(count[g]) - mean*mean
				if plan.fn == "dev" {
					v = math.Sqrt(v)
				}
				values[g] = v
			}
			cols = append(cols, Column{Name: plan.name, Data: columnArray[float64]{kind: KindF64, data: values}})
		case "wavg":
			prods := slots[plan.slot].sums
			weights := slots[plan.wslot].sums
			hasNull := false
			values := make([]float64, groups)
			for g := 0; g < groups; g++ {
				if count[g] == 0 || weights[g] == 0 {
					// Matches aggregateResult: zero participating weight is a
					// null result, not a division.
					hasNull = true
					continue
				}
				values[g] = prods[g] / weights[g]
			}
			if hasNull {
				boxed := make([]any, groups)
				for g := 0; g < groups; g++ {
					if count[g] == 0 || weights[g] == 0 {
						boxed[g] = NullValue
						continue
					}
					boxed[g] = values[g]
				}
				cols = append(cols, NewColumn(plan.name, boxed))
			} else {
				cols = append(cols, Column{Name: plan.name, Data: columnArray[float64]{kind: KindF64, data: values}})
			}
		}
	}
	bulkI64Release(count, true)
	for i := range slots {
		bulkF64Release(slots[i].sums, true)
		if slots[i].sqsums != nil {
			bulkF64Release(slots[i].sqsums, true)
		}
	}
	fusedReleaseSlots(slots)
	out, err := newFrameTrusted(cols...)
	return out, true, err
}

func execGroupedFusedI64Sums(frame Frame, indexes []int, allRows bool, byInputs []groupInput, aggs []aggregateInput) (Frame, bool, error) {
	rows := frame.Len()
	if rows == 0 || len(byInputs) == 0 || len(byInputs) > 2 || len(aggs) == 0 {
		return Frame{}, false, nil
	}
	var sources [][]int64
	plans := make([]fusedAggPlan, len(aggs))
	for i, agg := range aggs {
		plans[i] = fusedAggPlan{fn: agg.Func, name: agg.Name, slot: -1, wslot: -1}
		switch agg.Func {
		case "count":
			if agg.Weight != nil {
				return Frame{}, false, nil
			}
		case "sum":
			if agg.Weight != nil || agg.column == nil || agg.leftColumn != nil || agg.rightColumn != nil {
				return Frame{}, false, nil
			}
			src, ok := groupAggI64Slice(agg.column)
			if !ok || len(src) != rows {
				return Frame{}, false, nil
			}
			found := -1
			for j, existing := range sources {
				if len(existing) > 0 && len(src) > 0 && &existing[0] == &src[0] {
					found = j
					break
				}
			}
			if found < 0 {
				if len(sources) >= fusedGroupMaxSumSlots {
					return Frame{}, false, nil
				}
				found = len(sources)
				sources = append(sources, src)
			}
			plans[i].slot = found
		default:
			return Frame{}, false, nil
		}
	}

	byArrays, ok := groupByArrays(frame, byInputs)
	if !ok {
		return Frame{}, false, nil
	}
	var ids0, ids1 []int
	c1 := 1
	domain := 1
	for i, array := range byArrays {
		ids, count, ok := groupColumnRowIDs(frame, byInputs[i], array)
		if !ok || count <= 0 {
			return Frame{}, false, nil
		}
		if i == 0 {
			ids0 = ids
			domain = count
		} else {
			ids1 = ids
			c1 = count
			if int64(domain)*int64(count) > 1<<22 {
				return Frame{}, false, nil
			}
			domain *= count
		}
	}
	n := rows
	if !allRows {
		n = len(indexes)
		for _, row := range indexes {
			if row < 0 || row >= rows {
				return Frame{}, false, nil
			}
		}
	}
	if domain > 1<<22 || (domain > rows*4 && domain > 1<<12) {
		return Frame{}, false, nil
	}
	capN := domain
	if n < capN {
		capN = n
	}
	slate := bulkI32Get(domain)
	clear(slate)
	count := bulkI64Get(capN)
	clear(count)
	var sum0, sum1, sum2 []int64
	if len(sources) > 0 {
		sum0 = bulkI64Get(capN)
		clear(sum0)
	}
	if len(sources) > 1 {
		sum1 = bulkI64Get(capN)
		clear(sum1)
	}
	if len(sources) > 2 {
		sum2 = bulkI64Get(capN)
		clear(sum2)
	}

	repRows := bulkIntGet(capN)
	defer bulkIntRelease(repRows)
	if allRows {
		for row := 0; row < rows; row++ {
			key := ids0[row]
			if ids1 != nil {
				key = key*c1 + ids1[row]
			}
			g := slate[key]
			if g == 0 {
				repRows = append(repRows, row)
				g = int32(len(repRows))
				slate[key] = g
			}
			gi := int(g - 1)
			count[gi]++
			if sum0 != nil {
				sum0[gi] += sources[0][row]
			}
			if sum1 != nil {
				sum1[gi] += sources[1][row]
			}
			if sum2 != nil {
				sum2[gi] += sources[2][row]
			}
		}
	} else {
		for _, row := range indexes {
			key := ids0[row]
			if ids1 != nil {
				key = key*c1 + ids1[row]
			}
			g := slate[key]
			if g == 0 {
				repRows = append(repRows, row)
				g = int32(len(repRows))
				slate[key] = g
			}
			gi := int(g - 1)
			count[gi]++
			if sum0 != nil {
				sum0[gi] += sources[0][row]
			}
			if sum1 != nil {
				sum1[gi] += sources[1][row]
			}
			if sum2 != nil {
				sum2[gi] += sources[2][row]
			}
		}
	}
	bulkI32Release(slate)

	groups := len(repRows)
	cols := make([]Column, 0, len(byInputs)+len(aggs))
	cols, err := appendFusedGroupKeyColumns(cols, frame, allRows, byInputs, byArrays, repRows)
	if err != nil {
		bulkI64Release(count, true)
		bulkI64Release(sum0, true)
		bulkI64Release(sum1, true)
		bulkI64Release(sum2, true)
		return Frame{}, true, err
	}
	for _, plan := range plans {
		switch plan.fn {
		case "count":
			values := make([]int64, groups)
			copy(values, count[:groups])
			cols = append(cols, Column{Name: plan.name, Data: columnArray[int64]{kind: KindI64, data: values}})
		case "sum":
			values := make([]int64, groups)
			switch plan.slot {
			case 0:
				copy(values, sum0[:groups])
			case 1:
				copy(values, sum1[:groups])
			case 2:
				copy(values, sum2[:groups])
			}
			cols = append(cols, Column{Name: plan.name, Data: columnArray[int64]{kind: KindI64, data: values}})
		}
	}
	bulkI64Release(count, true)
	bulkI64Release(sum0, true)
	bulkI64Release(sum1, true)
	bulkI64Release(sum2, true)
	out, err := newFrameTrusted(cols...)
	return out, true, err
}
