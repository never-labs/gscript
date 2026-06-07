package data

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type typedKernelRegistry struct{}

var typedKernels typedKernelRegistry

const (
	NumericUnaryNeg     = "neg"
	NumericUnaryAbs     = "abs"
	NumericUnarySqrt    = "sqrt"
	NumericUnaryLog     = "log"
	NumericUnaryExp     = "exp"
	NumericUnaryRecip   = "reciprocal"
	NumericUnarySignum  = "signum"
	NumericUnaryFloor   = "floor"
	NumericUnaryCeiling = "ceiling"
)

func compareMaskTyped(array Array, op Op, value any, out []bool) bool {
	return typedKernels.CompareMask(array, op, value, out)
}

func coerceComparableSymbol(value any) (Symbol, bool) {
	switch x := value.(type) {
	case Symbol:
		return x, true
	case string:
		return Symbol(x), true
	default:
		return "", false
	}
}

func coerceComparableString(value any) (string, bool) {
	switch x := value.(type) {
	case string:
		return x, true
	case Symbol:
		return string(x), true
	default:
		return "", false
	}
}

func withinMaskTyped(array Array, low, high any, highClosed bool, out []bool) bool {
	return typedKernels.WithinMask(array, low, high, highClosed, out)
}

func numericAt(array Array, row int) (float64, bool, error) {
	return typedKernels.NumericAt(array, row)
}

func Bin(domain Array, query any) (any, bool, error) {
	return typedKernels.Bin(domain, query)
}

func (typedKernelRegistry) CompareMask(array Array, op Op, value any, out []bool) bool {
	if len(out) < array.Len() {
		return false
	}
	switch a := array.(type) {
	case attributedArray:
		return typedKernels.CompareMask(a.array, op, value, out)
	case encodedArray:
		return compareEncodedMask(a, op, value, out)
	case columnArray[bool]:
		v, ok := value.(bool)
		if !ok {
			return false
		}
		for i, x := range a.data {
			out[i] = boolCompare(op, x == v, compareBool(x, v))
		}
		return true
	case columnArray[int8]:
		v, ok := value.(int8)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[int16]:
		v, ok := value.(int16)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[int32]:
		v, ok := value.(int32)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[int64]:
		v, ok := coerceInt64Exact(value)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[uint8]:
		v, ok := value.(uint8)
		return compareUnsignedSlice(a.data, v, ok, op, out)
	case columnArray[uint16]:
		v, ok := value.(uint16)
		return compareUnsignedSlice(a.data, v, ok, op, out)
	case columnArray[uint32]:
		v, ok := value.(uint32)
		return compareUnsignedSlice(a.data, v, ok, op, out)
	case columnArray[uint64]:
		v, ok := value.(uint64)
		return compareUnsignedSlice(a.data, v, ok, op, out)
	case columnArray[float32]:
		v, ok := value.(float32)
		return compareFloatSlice(a.data, v, ok, op, out)
	case columnArray[float64]:
		v, ok := numeric(value)
		return compareFloatSlice(a.data, v, ok, op, out)
	case columnArray[string]:
		v, ok := coerceComparableString(value)
		return compareStringSlice(a.data, v, ok, op, out)
	case columnArray[Symbol]:
		v, ok := coerceComparableSymbol(value)
		return compareSymbolSlice(a.data, v, ok, op, out)
	case columnArray[Month]:
		v, ok := value.(Month)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[Date]:
		v, ok := value.(Date)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[DateTime]:
		v, ok := value.(DateTime)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[Timespan]:
		v, ok := value.(Timespan)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[Minute]:
		v, ok := value.(Minute)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[Second]:
		v, ok := value.(Second)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[Time]:
		v, ok := value.(Time)
		return compareSignedSlice(a.data, v, ok, op, out)
	case columnArray[Timestamp]:
		v, ok := value.(Timestamp)
		return compareSignedSlice(a.data, v, ok, op, out)
	default:
		return false
	}
}

func (k typedKernelRegistry) CompareIndexes(array Array, op Op, value any, out []int) ([]int, bool) {
	switch a := array.(type) {
	case attributedArray:
		return k.CompareIndexes(a.array, op, value, out)
	case encodedArray:
		return compareEncodedIndexes(a, op, value, out)
	case columnArray[bool]:
		v, ok := value.(bool)
		return compareBoolIndexes(a.data, v, ok, op, out)
	case columnArray[int8]:
		v, ok := value.(int8)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[int16]:
		v, ok := value.(int16)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[int32]:
		v, ok := value.(int32)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[int64]:
		v, ok := coerceInt64Exact(value)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[uint8]:
		v, ok := value.(uint8)
		return compareUnsignedIndexes(a.data, v, ok, op, out)
	case columnArray[uint16]:
		v, ok := value.(uint16)
		return compareUnsignedIndexes(a.data, v, ok, op, out)
	case columnArray[uint32]:
		v, ok := value.(uint32)
		return compareUnsignedIndexes(a.data, v, ok, op, out)
	case columnArray[uint64]:
		v, ok := value.(uint64)
		return compareUnsignedIndexes(a.data, v, ok, op, out)
	case columnArray[float32]:
		v, ok := value.(float32)
		return compareFloatIndexes(a.data, v, ok, op, out)
	case columnArray[float64]:
		v, ok := numeric(value)
		return compareFloatIndexes(a.data, v, ok, op, out)
	case columnArray[string]:
		v, ok := coerceComparableString(value)
		return compareStringIndexes(a.data, v, ok, op, out)
	case columnArray[Symbol]:
		v, ok := coerceComparableSymbol(value)
		return compareSymbolIndexes(a.data, v, ok, op, out)
	case columnArray[Month]:
		v, ok := value.(Month)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[Date]:
		v, ok := value.(Date)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[DateTime]:
		v, ok := value.(DateTime)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[Timespan]:
		v, ok := value.(Timespan)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[Minute]:
		v, ok := value.(Minute)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[Second]:
		v, ok := value.(Second)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[Time]:
		v, ok := value.(Time)
		return compareSignedIndexes(a.data, v, ok, op, out)
	case columnArray[Timestamp]:
		v, ok := value.(Timestamp)
		return compareSignedIndexes(a.data, v, ok, op, out)
	default:
		return nil, false
	}
}

func (typedKernelRegistry) WithinMask(array Array, low, high any, highClosed bool, out []bool) bool {
	if len(out) < array.Len() {
		return false
	}
	if IsNull(low) || IsNull(high) {
		return false
	}
	switch a := array.(type) {
	case attributedArray:
		return typedKernels.WithinMask(a.array, low, high, highClosed, out)
	case columnArray[int8]:
		lo, lok := low.(int8)
		hi, hok := high.(int8)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[int16]:
		lo, lok := low.(int16)
		hi, hok := high.(int16)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[int32]:
		lo, lok := low.(int32)
		hi, hok := high.(int32)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[int64]:
		lo, lok := coerceInt64Exact(low)
		hi, hok := coerceInt64Exact(high)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[uint8]:
		lo, lok := low.(uint8)
		hi, hok := high.(uint8)
		return withinUnsignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[uint16]:
		lo, lok := low.(uint16)
		hi, hok := high.(uint16)
		return withinUnsignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[uint32]:
		lo, lok := low.(uint32)
		hi, hok := high.(uint32)
		return withinUnsignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[uint64]:
		lo, lok := low.(uint64)
		hi, hok := high.(uint64)
		return withinUnsignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[float32]:
		lo, lok := low.(float32)
		hi, hok := high.(float32)
		return withinFloatSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[float64]:
		lo, lok := numeric(low)
		hi, hok := numeric(high)
		return withinFloatSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[string]:
		lo, lok := coerceComparableString(low)
		hi, hok := coerceComparableString(high)
		return withinStringSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Symbol]:
		lo, lok := coerceComparableSymbol(low)
		hi, hok := coerceComparableSymbol(high)
		return withinSymbolSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Month]:
		lo, lok := low.(Month)
		hi, hok := high.(Month)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Date]:
		lo, lok := low.(Date)
		hi, hok := high.(Date)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[DateTime]:
		lo, lok := low.(DateTime)
		hi, hok := high.(DateTime)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Timespan]:
		lo, lok := low.(Timespan)
		hi, hok := high.(Timespan)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Minute]:
		lo, lok := low.(Minute)
		hi, hok := high.(Minute)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Second]:
		lo, lok := low.(Second)
		hi, hok := high.(Second)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Time]:
		lo, lok := low.(Time)
		hi, hok := high.(Time)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Timestamp]:
		lo, lok := low.(Timestamp)
		hi, hok := high.(Timestamp)
		return withinSignedSlice(a.data, lo, hi, lok && hok, highClosed, out)
	default:
		return false
	}
}

func (k typedKernelRegistry) WithinIndexes(array Array, low, high any, highClosed bool, out []int) ([]int, bool) {
	if IsNull(low) || IsNull(high) {
		return nil, false
	}
	switch a := array.(type) {
	case attributedArray:
		return k.WithinIndexes(a.array, low, high, highClosed, out)
	case columnArray[int8]:
		lo, lok := low.(int8)
		hi, hok := high.(int8)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[int16]:
		lo, lok := low.(int16)
		hi, hok := high.(int16)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[int32]:
		lo, lok := low.(int32)
		hi, hok := high.(int32)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[int64]:
		lo, lok := coerceInt64Exact(low)
		hi, hok := coerceInt64Exact(high)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[uint8]:
		lo, lok := low.(uint8)
		hi, hok := high.(uint8)
		return withinUnsignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[uint16]:
		lo, lok := low.(uint16)
		hi, hok := high.(uint16)
		return withinUnsignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[uint32]:
		lo, lok := low.(uint32)
		hi, hok := high.(uint32)
		return withinUnsignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[uint64]:
		lo, lok := low.(uint64)
		hi, hok := high.(uint64)
		return withinUnsignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[float32]:
		lo, lok := low.(float32)
		hi, hok := high.(float32)
		return withinFloatIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[float64]:
		lo, lok := numeric(low)
		hi, hok := numeric(high)
		return withinFloatIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[string]:
		lo, lok := coerceComparableString(low)
		hi, hok := coerceComparableString(high)
		return withinStringIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Symbol]:
		lo, lok := coerceComparableSymbol(low)
		hi, hok := coerceComparableSymbol(high)
		return withinSymbolIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Month]:
		lo, lok := low.(Month)
		hi, hok := high.(Month)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Date]:
		lo, lok := low.(Date)
		hi, hok := high.(Date)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[DateTime]:
		lo, lok := low.(DateTime)
		hi, hok := high.(DateTime)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Timespan]:
		lo, lok := low.(Timespan)
		hi, hok := high.(Timespan)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Minute]:
		lo, lok := low.(Minute)
		hi, hok := high.(Minute)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Second]:
		lo, lok := low.(Second)
		hi, hok := high.(Second)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Time]:
		lo, lok := low.(Time)
		hi, hok := high.(Time)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	case columnArray[Timestamp]:
		lo, lok := low.(Timestamp)
		hi, hok := high.(Timestamp)
		return withinSignedIndexes(a.data, lo, hi, lok && hok, highClosed, out)
	default:
		return nil, false
	}
}

func (typedKernelRegistry) IndexedInRows(array Array, values []any) ([]int, bool) {
	if len(values) == 0 {
		if _, ok := ArrayIndexFor(array, ArrayAttributeUnique); ok {
			return []int{}, true
		}
		if _, ok := ArrayIndexFor(array, ArrayAttributeGrouped); ok {
			return []int{}, true
		}
		return nil, false
	}
	index, ok := ArrayIndexFor(array, ArrayAttributeUnique)
	if !ok {
		index, ok = ArrayIndexFor(array, ArrayAttributeGrouped)
	}
	if !ok {
		return nil, false
	}
	matched := make([]bool, array.Len())
	for _, value := range values {
		normalized, err := normalizeKeyValue(array.Kind(), value)
		if err != nil {
			return nil, false
		}
		for _, row := range index.RowsByKey[arrayValueKey(array.Kind(), normalized)] {
			if row < 0 || row >= len(matched) {
				return nil, false
			}
			matched[row] = true
		}
	}
	rows := make([]int, 0)
	for row, keep := range matched {
		if keep {
			rows = append(rows, row)
		}
	}
	return rows, true
}

func (typedKernelRegistry) GroupCounts(index ArrayIndex) ([]int64, bool) {
	counts := make([]int64, len(index.Rows))
	for group, rows := range index.Rows {
		counts[group] = int64(len(rows))
	}
	return counts, true
}

func (typedKernelRegistry) FilteredGroupCounts(index ArrayIndex, indexes []int) ([]int, []int64, bool, error) {
	if len(index.Rows) == 0 || len(indexes) == 0 {
		return nil, make([]int64, len(index.Rows)), true, nil
	}
	maxRow := -1
	for _, rows := range index.Rows {
		for _, row := range rows {
			if row < 0 {
				return nil, nil, false, fmt.Errorf("group index contains negative row %d", row)
			}
			if row > maxRow {
				maxRow = row
			}
		}
	}
	if maxRow < 0 {
		return nil, make([]int64, len(index.Rows)), true, nil
	}
	rowToGroup := make([]int, maxRow+1)
	for row := range rowToGroup {
		rowToGroup[row] = -1
	}
	for group, rows := range index.Rows {
		for _, row := range rows {
			if row >= len(rowToGroup) {
				return nil, nil, false, fmt.Errorf("group index row %d out of range", row)
			}
			if rowToGroup[row] >= 0 {
				return nil, nil, false, fmt.Errorf("group index row %d appears in multiple groups", row)
			}
			rowToGroup[row] = group
		}
	}
	counts := make([]int64, len(index.Rows))
	seen := make([]bool, len(index.Rows))
	order := make([]int, 0, len(index.Rows))
	for _, row := range indexes {
		if row < 0 || row >= len(rowToGroup) {
			return nil, nil, false, fmt.Errorf("filter row %d out of range for grouped index", row)
		}
		group := rowToGroup[row]
		if group < 0 {
			return nil, nil, false, fmt.Errorf("filter row %d is missing from grouped index", row)
		}
		if !seen[group] {
			seen[group] = true
			order = append(order, group)
		}
		counts[group]++
	}
	return order, counts, true, nil
}

func (typedKernelRegistry) GroupAggregateStates(index ArrayIndex, aggs []aggregateInput) ([]groupState, bool, error) {
	if !groupAggregatesSupportedByTypedIndex(aggs) {
		return nil, false, nil
	}
	states := make([]groupState, len(index.Rows))
	for group, rows := range index.Rows {
		states[group] = groupState{
			keys: []any{index.Keys[group]},
			aggs: make([]aggregateState, len(aggs)),
		}
		for i, agg := range aggs {
			states[group].aggs[i].fn = agg.Func
			if err := accumulateIndexedAggregate(&states[group].aggs[i], agg, rows); err != nil {
				return nil, true, err
			}
		}
	}
	return states, true, nil
}

func (typedKernelRegistry) FilteredGroupAggregateStates(index ArrayIndex, indexes []int, aggs []aggregateInput) ([]int, []groupState, bool, error) {
	if !groupAggregatesSupportedByTypedIndex(aggs) {
		return nil, nil, false, nil
	}
	groupOrder, _, ok, err := typedKernels.FilteredGroupCounts(index, indexes)
	if err != nil || !ok {
		return nil, nil, ok, err
	}
	states := make([]groupState, len(index.Rows))
	for group := range index.Rows {
		states[group] = groupState{
			keys: []any{index.Keys[group]},
			aggs: make([]aggregateState, len(aggs)),
		}
		for i, agg := range aggs {
			states[group].aggs[i].fn = agg.Func
		}
	}
	if len(indexes) == 0 {
		return groupOrder, states, true, nil
	}
	rowToGroup, err := rowToGroupFromIndex(index)
	if err != nil {
		return nil, nil, true, err
	}
	for _, row := range indexes {
		if row < 0 || row >= len(rowToGroup) {
			return nil, nil, true, fmt.Errorf("filter row %d out of range for grouped index", row)
		}
		group := rowToGroup[row]
		if group < 0 {
			return nil, nil, true, fmt.Errorf("filter row %d is missing from grouped index", row)
		}
		for i, agg := range aggs {
			if err := accumulateIndexedAggregateRow(&states[group].aggs[i], agg, row); err != nil {
				return nil, nil, true, err
			}
		}
	}
	return groupOrder, states, true, nil
}

func groupAggregatesSupportedByTypedIndex(aggs []aggregateInput) bool {
	if len(aggs) == 0 {
		return false
	}
	for _, agg := range aggs {
		if agg.Weight != nil {
			return false
		}
		switch agg.Func {
		case "count":
		case "sum", "avg":
			if agg.column == nil || !isNumericArray(agg.column) {
				return false
			}
		case "min", "max":
			if agg.column == nil || !isTypedMinMaxArray(agg.column) {
				return false
			}
		case "first", "last":
			if agg.column == nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func isTypedMinMaxArray(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return isTypedMinMaxArray(a.array)
	case columnArray[bool], columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		columnArray[float32], columnArray[float64], columnArray[string], columnArray[Symbol],
		columnArray[Month], columnArray[Date], columnArray[DateTime], columnArray[Timespan],
		columnArray[Minute], columnArray[Second], columnArray[Time], columnArray[Timestamp], nullableArray:
		return true
	default:
		return false
	}
}

func accumulateIndexedAggregate(state *aggregateState, agg aggregateInput, rows []int) error {
	switch agg.Func {
	case "count":
		state.count = int64(len(rows))
		return nil
	default:
		for _, row := range rows {
			if err := accumulateIndexedAggregateRow(state, agg, row); err != nil {
				return err
			}
		}
		return nil
	}
}

func accumulateIndexedAggregateRow(state *aggregateState, agg aggregateInput, row int) error {
	switch agg.Func {
	case "count":
		state.count++
	case "sum", "avg":
		n, ok, err := typedKernels.NumericAt(agg.column, row)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		state.sum += n
		state.count++
	case "min", "max":
		v, ok := agg.column.At(row)
		if !ok {
			return fmt.Errorf("column row %d out of range", row)
		}
		if IsNull(v) {
			return nil
		}
		if !state.hasValue || (agg.Func == "min" && compare(v, state.value) < 0) || (agg.Func == "max" && compare(v, state.value) > 0) {
			state.value = v
			state.hasValue = true
		}
	case "first":
		if !state.hasValue {
			v, ok := agg.column.At(row)
			if !ok {
				return fmt.Errorf("column row %d out of range", row)
			}
			state.value = normalizeAggregateValue(v)
			state.hasValue = true
		}
	case "last":
		v, ok := agg.column.At(row)
		if !ok {
			return fmt.Errorf("column row %d out of range", row)
		}
		state.lastValue = normalizeAggregateValue(v)
		state.hasLastVal = true
	}
	return nil
}

func rowToGroupFromIndex(index ArrayIndex) ([]int, error) {
	maxRow := -1
	for _, rows := range index.Rows {
		for _, row := range rows {
			if row < 0 {
				return nil, fmt.Errorf("group index contains negative row %d", row)
			}
			if row > maxRow {
				maxRow = row
			}
		}
	}
	if maxRow < 0 {
		return nil, nil
	}
	rowToGroup := make([]int, maxRow+1)
	for row := range rowToGroup {
		rowToGroup[row] = -1
	}
	for group, rows := range index.Rows {
		for _, row := range rows {
			if rowToGroup[row] >= 0 {
				return nil, fmt.Errorf("group index row %d appears in multiple groups", row)
			}
			rowToGroup[row] = group
		}
	}
	return rowToGroup, nil
}

func (typedKernelRegistry) NullMask(array Array, out []bool) bool {
	if len(out) < array.Len() {
		return false
	}
	switch a := array.(type) {
	case nullableArray:
		for i, v := range a.data {
			out[i] = IsNull(v)
		}
	default:
		for i := 0; i < array.Len(); i++ {
			out[i] = false
		}
	}
	return true
}

func (typedKernelRegistry) Count(array Array) (int64, bool) {
	return int64(array.Len()), true
}

func (typedKernelRegistry) ComplementSortedIndexes(length int, exclude []int) ([]int, bool, error) {
	if len(exclude) == 0 {
		return allIndexes(length), true, nil
	}
	if len(exclude) > length {
		return nil, false, nil
	}
	prev := -1
	for _, row := range exclude {
		if row < 0 || row >= length {
			return nil, false, fmt.Errorf("index row %d out of range for length %d", row, length)
		}
		if row <= prev {
			return nil, false, nil
		}
		prev = row
	}
	if len(exclude) == length {
		return []int{}, true, nil
	}
	out := make([]int, 0, length-len(exclude))
	nextExcluded := 0
	for row := 0; row < length; row++ {
		if nextExcluded < len(exclude) && row == exclude[nextExcluded] {
			nextExcluded++
			continue
		}
		out = append(out, row)
	}
	return out, true, nil
}

func (k typedKernelRegistry) NonNullCount(array Array) (int64, bool) {
	mask := make([]bool, array.Len())
	if !k.NullMask(array, mask) {
		return 0, false
	}
	var count int64
	for _, isNull := range mask {
		if !isNull {
			count++
		}
	}
	return count, true
}

func (typedKernelRegistry) NumericUnary(op string, array Array) (Array, bool, error) {
	switch a := array.(type) {
	case columnArray[int8]:
		return numericUnarySlice(op, a.data)
	case columnArray[int16]:
		return numericUnarySlice(op, a.data)
	case columnArray[int32]:
		return numericUnarySlice(op, a.data)
	case columnArray[int64]:
		return numericUnarySlice(op, a.data)
	case columnArray[uint8]:
		return numericUnarySlice(op, a.data)
	case columnArray[uint16]:
		return numericUnarySlice(op, a.data)
	case columnArray[uint32]:
		return numericUnarySlice(op, a.data)
	case columnArray[uint64]:
		return numericUnarySlice(op, a.data)
	case columnArray[float32]:
		return numericUnarySlice(op, a.data)
	case columnArray[float64]:
		return numericUnarySlice(op, a.data)
	case nullableArray:
		return numericUnaryNullable(op, a.data)
	default:
		return nil, false, nil
	}
}

func (typedKernelRegistry) NumericBinary(op Op, left, right Array) (Array, bool, error) {
	out, ok, err := typedKernels.Dyadic(op, left, right)
	if err != nil || !ok {
		return nil, ok, err
	}
	array, ok := out.(Array)
	if !ok {
		return nil, false, nil
	}
	return array, true, nil
}

func (typedKernelRegistry) Dyadic(op Op, left, right any) (any, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray && !rightIsArray {
		return nil, false, nil
	}
	length := 0
	switch {
	case leftIsArray && rightIsArray:
		if leftArray.Len() != rightArray.Len() {
			return nil, true, fmt.Errorf("typed dyadic kernel length mismatch: %d != %d", leftArray.Len(), rightArray.Len())
		}
		length = leftArray.Len()
	case leftIsArray:
		length = leftArray.Len()
	case rightIsArray:
		length = rightArray.Len()
	}
	if isArithmeticOp(op) {
		if !typedNumericOperand(left) || !typedNumericOperand(right) {
			return nil, false, nil
		}
		return numericDyadic(op, left, right, length)
	}
	if isComparisonOp(op) {
		return compareDyadic(op, left, right, length)
	}
	return nil, false, nil
}

func (typedKernelRegistry) NumericSum(array Array) (float64, int64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedKernels.NumericSum(a.array)
	case columnArray[int8]:
		return numericSumSlice(a.data)
	case columnArray[int16]:
		return numericSumSlice(a.data)
	case columnArray[int32]:
		return numericSumSlice(a.data)
	case columnArray[int64]:
		return numericSumSlice(a.data)
	case columnArray[uint8]:
		return numericSumSlice(a.data)
	case columnArray[uint16]:
		return numericSumSlice(a.data)
	case columnArray[uint32]:
		return numericSumSlice(a.data)
	case columnArray[uint64]:
		return numericSumSlice(a.data)
	case columnArray[float32]:
		return numericSumSlice(a.data)
	case columnArray[float64]:
		return numericSumSlice(a.data)
	case nullableArray:
		var sum float64
		var count int64
		for _, v := range a.data {
			if IsNull(v) {
				continue
			}
			n, ok := numeric(v)
			if !ok {
				return 0, 0, true, fmt.Errorf("sum expects numeric values, got %T (%v)", v, v)
			}
			sum += n
			count++
		}
		return sum, count, true, nil
	default:
		return 0, 0, false, nil
	}
}

func (typedKernelRegistry) Min(array Array) (any, bool, bool, error) {
	return minMax(array, "min")
}

func (typedKernelRegistry) Max(array Array) (any, bool, bool, error) {
	return minMax(array, "max")
}

func (typedKernelRegistry) RowsByKey(frame Frame, columns []Symbol) (map[string][]int, error) {
	if len(columns) == 1 {
		if column, ok := frame.Column(columns[0]); ok {
			if index, ok := ArrayIndexFor(column, ArrayAttributeUnique); ok {
				return cloneRowsByKey(index.RowsByKey), nil
			}
			if index, ok := ArrayIndexFor(column, ArrayAttributeGrouped); ok {
				return cloneRowsByKey(index.RowsByKey), nil
			}
		}
	}
	encoder, err := newRowKeyEncoder(frame, columns)
	if err != nil {
		return nil, err
	}
	rowsByKey := make(map[string][]int, frame.Len())
	var b strings.Builder
	for row := 0; row < frame.Len(); row++ {
		key, err := encoder.keyWithBuilder(row, &b)
		if err != nil {
			return nil, err
		}
		rowsByKey[key] = append(rowsByKey[key], row)
	}
	return rowsByKey, nil
}

func (k typedKernelRegistry) JoinIndexes(left, right Frame, keepUnmatchedLeft bool, keys []JoinKey) ([]int, []int, error) {
	rightRowsByKey, rightKeyCols, err := rightRowsByJoinKey(right, keys)
	if err != nil {
		return nil, nil, err
	}
	rightKinds := make([]Kind, len(rightKeyCols))
	for i, name := range rightKeyCols {
		col, ok := right.Column(name)
		if !ok {
			return nil, nil, fmt.Errorf("join right key column %q does not exist", name)
		}
		rightKinds[i] = col.Kind()
	}
	leftKeyCols := leftKeyColumns(keys)
	encoder, err := newRowKeyEncoderWithKinds(left, leftKeyCols, rightKinds)
	if err != nil {
		return nil, nil, err
	}

	leftIndexes := make([]int, 0)
	rightIndexes := make([]int, 0)
	var b strings.Builder
	for row := 0; row < left.Len(); row++ {
		key, ok, err := encoder.lookupKeyWithBuilder(row, &b)
		if err != nil {
			return nil, nil, err
		}
		matches := []int(nil)
		if ok {
			matches = rightRowsByKey[key]
		}
		if keepUnmatchedLeft && len(matches) == 0 {
			leftIndexes = append(leftIndexes, row)
			rightIndexes = append(rightIndexes, -1)
			continue
		}
		for _, rightRow := range matches {
			leftIndexes = append(leftIndexes, row)
			rightIndexes = append(rightIndexes, rightRow)
		}
	}
	return leftIndexes, rightIndexes, nil
}

func (typedKernelRegistry) Bin(domain Array, query any) (any, bool, error) {
	if domain == nil {
		return nil, false, fmt.Errorf("bin domain is nil")
	}
	if queryArray, ok := query.(Array); ok {
		out := make([]int64, queryArray.Len())
		for i := 0; i < queryArray.Len(); i++ {
			value, ok := queryArray.At(i)
			if !ok {
				return nil, true, fmt.Errorf("bin query row %d out of range", i)
			}
			index, err := kdbBinScalar(domain, value)
			if err != nil {
				return nil, true, err
			}
			out[i] = index
		}
		return NewI64(out), true, nil
	}
	index, err := kdbBinScalar(domain, query)
	if err != nil {
		return nil, true, err
	}
	return index, true, nil
}

func kdbBinScalar(domain Array, query any) (int64, error) {
	if domain.Len() == 0 || IsNull(query) {
		return -1, nil
	}
	// `s#` is a planner-facing promise. q trusts it; using the original order
	// here makes the metadata observable and lets later kernels specialize.
	_ = ArrayHasAttribute(domain, ArrayAttributeSorted)
	lo, hi := 0, domain.Len()
	for lo < hi {
		mid := lo + (hi-lo)/2
		value, ok := domain.At(mid)
		if !ok {
			return 0, fmt.Errorf("bin domain row %d out of range", mid)
		}
		cmp := compare(value, query)
		if cmp <= 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return int64(lo - 1), nil
}

func (typedKernelRegistry) SortedRowsByPartition(frame Frame, timeColumn Array, partitionColumns []Symbol) (map[string][]int, error) {
	if len(partitionColumns) == 1 {
		if partition, ok := frame.Column(partitionColumns[0]); ok {
			if index, ok := ArrayIndexFor(partition, ArrayAttributeUnique); ok {
				return sortedRowsByPartitionIndex(timeColumn, index)
			}
			if index, ok := ArrayIndexFor(partition, ArrayAttributeGrouped); ok {
				return sortedRowsByPartitionIndex(timeColumn, index)
			}
		}
	}
	encoder, err := newRowKeyEncoder(frame, partitionColumns)
	if err != nil {
		return nil, err
	}
	rowsByPartition := make(map[string][]int, frame.Len())
	var b strings.Builder
	for row := 0; row < frame.Len(); row++ {
		timeValue, ok := timeColumn.At(row)
		if !ok {
			return nil, fmt.Errorf("time column row %d out of range", row)
		}
		if IsNull(timeValue) {
			continue
		}
		key, err := encoder.keyWithBuilder(row, &b)
		if err != nil {
			return nil, err
		}
		rowsByPartition[key] = append(rowsByPartition[key], row)
	}
	if !ArrayHasAttribute(timeColumn, ArrayAttributeSorted) {
		if times, ok := int64ColumnData(timeColumn); ok {
			for key, rows := range rowsByPartition {
				if !int64RowsSorted(times, rows) {
					sort.SliceStable(rows, func(i, j int) bool {
						return times[rows[i]] < times[rows[j]]
					})
				}
				rowsByPartition[key] = rows
			}
			return rowsByPartition, nil
		}
		for key, rows := range rowsByPartition {
			if !rowsSortedByTime(timeColumn, rows) {
				sort.SliceStable(rows, func(i, j int) bool {
					leftTime, _ := timeColumn.At(rows[i])
					rightTime, _ := timeColumn.At(rows[j])
					return compare(leftTime, rightTime) < 0
				})
			}
			rowsByPartition[key] = rows
		}
	}
	return rowsByPartition, nil
}

func sortedRowsByPartitionIndex(timeColumn Array, index ArrayIndex) (map[string][]int, error) {
	rowsByPartition := make(map[string][]int, len(index.RowsByKey))
	for key, rows := range index.RowsByKey {
		out := make([]int, 0, len(rows))
		for _, row := range rows {
			timeValue, ok := timeColumn.At(row)
			if !ok {
				return nil, fmt.Errorf("time column row %d out of range", row)
			}
			if IsNull(timeValue) {
				continue
			}
			out = append(out, row)
		}
		rowsByPartition[key] = out
	}
	if !ArrayHasAttribute(timeColumn, ArrayAttributeSorted) {
		if times, ok := int64ColumnData(timeColumn); ok {
			for key, rows := range rowsByPartition {
				if !int64RowsSorted(times, rows) {
					sort.SliceStable(rows, func(i, j int) bool {
						return times[rows[i]] < times[rows[j]]
					})
				}
				rowsByPartition[key] = rows
			}
			return rowsByPartition, nil
		}
		for key, rows := range rowsByPartition {
			if !rowsSortedByTime(timeColumn, rows) {
				sort.SliceStable(rows, func(i, j int) bool {
					leftTime, _ := timeColumn.At(rows[i])
					rightTime, _ := timeColumn.At(rows[j])
					return compare(leftTime, rightTime) < 0
				})
			}
			rowsByPartition[key] = rows
		}
	}
	return rowsByPartition, nil
}

func (typedKernelRegistry) AsofMatchIndexes(left Frame, leftTime Array, leftPartitionColumns []Symbol, rightTime Array, rightByPartition map[string][]int, partitionKinds ...[]Kind) ([]int, error) {
	kinds := optionalPartitionKinds(partitionKinds)
	if leftTimes, ok := int64ColumnData(leftTime); ok {
		if rightTimes, ok := int64ColumnData(rightTime); ok {
			return asofMatchIndexesI64(left, leftTimes, leftPartitionColumns, rightTimes, rightByPartition, kinds)
		}
	}
	encoder, err := newRowKeyEncoderWithKinds(left, leftPartitionColumns, kinds)
	if err != nil {
		return nil, err
	}
	rightIndexes := make([]int, left.Len())
	var b strings.Builder
	for row := 0; row < left.Len(); row++ {
		rightIndexes[row] = -1
		timeValue, ok := leftTime.At(row)
		if !ok {
			return nil, fmt.Errorf("time column row %d out of range", row)
		}
		if IsNull(timeValue) {
			continue
		}
		key, err := encoder.keyWithBuilder(row, &b)
		if err != nil {
			return nil, err
		}
		rows := rightByPartition[key]
		if len(rows) == 0 {
			continue
		}
		match := sort.Search(len(rows), func(i int) bool {
			rightTimeValue, _ := rightTime.At(rows[i])
			return compare(rightTimeValue, timeValue) > 0
		}) - 1
		if match >= 0 {
			rightIndexes[row] = rows[match]
		}
	}
	return rightIndexes, nil
}

func (typedKernelRegistry) WindowMatchIndexes(left Frame, leftTime Array, leftPartitionColumns []Symbol, rightTime Array, rightByPartition map[string][]int, opts WindowJoinOptions, partitionKinds ...[]Kind) ([][]int, error) {
	kinds := optionalPartitionKinds(partitionKinds)
	if !opts.HasBounds {
		if leftTimes, ok := int64ColumnData(leftTime); ok {
			if rightTimes, ok := int64ColumnData(rightTime); ok {
				return windowMatchIndexesI64(left, leftTimes, leftPartitionColumns, rightTimes, rightByPartition, kinds)
			}
		}
	}
	encoder, err := newRowKeyEncoderWithKinds(left, leftPartitionColumns, kinds)
	if err != nil {
		return nil, err
	}
	rightIndexes := make([][]int, left.Len())
	var b strings.Builder
	for row := 0; row < left.Len(); row++ {
		timeValue, ok := leftTime.At(row)
		if !ok {
			return nil, fmt.Errorf("time column row %d out of range", row)
		}
		if IsNull(timeValue) {
			rightIndexes[row] = []int{}
			continue
		}
		key, err := encoder.keyWithBuilder(row, &b)
		if err != nil {
			return nil, err
		}
		rows := rightByPartition[key]
		if len(rows) == 0 {
			rightIndexes[row] = []int{}
			continue
		}
		if opts.HasBounds {
			low, high, err := windowJoinAbsoluteBounds(timeValue, opts.Low, opts.High)
			if err != nil {
				return nil, err
			}
			start := sort.Search(len(rows), func(i int) bool {
				rightTimeValue, _ := rightTime.At(rows[i])
				return compare(rightTimeValue, low) >= 0
			})
			end := sort.Search(len(rows), func(i int) bool {
				rightTimeValue, _ := rightTime.At(rows[i])
				return compare(rightTimeValue, high) > 0
			})
			if start > end {
				start = end
			}
			rightIndexes[row] = cloneWindowRows(rows[start:end])
			continue
		}
		end := sort.Search(len(rows), func(i int) bool {
			rightTimeValue, _ := rightTime.At(rows[i])
			return compare(rightTimeValue, timeValue) > 0
		})
		rightIndexes[row] = cloneWindowRows(rows[:end])
	}
	return rightIndexes, nil
}

func optionalPartitionKinds(kinds [][]Kind) []Kind {
	if len(kinds) == 0 {
		return nil
	}
	return kinds[0]
}

func cloneWindowRows(rows []int) []int {
	if len(rows) == 0 {
		return []int{}
	}
	return append([]int(nil), rows...)
}

func (typedKernelRegistry) GatherOptional(array Array, indexes []int) Array {
	allPresent := true
	for _, row := range indexes {
		if row < 0 {
			allPresent = false
			break
		}
	}
	if allPresent {
		return array.Gather(indexes)
	}
	out := make([]any, len(indexes))
	for i, row := range indexes {
		if row < 0 {
			out[i] = NullValue
			continue
		}
		v, ok := array.At(row)
		if !ok {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		out[i] = v
	}
	return nullableArray{kind: array.Kind(), data: out}
}

type rowKeyEncoder struct {
	columns []keyColumn
	single  func(row int) (string, error)
}

type keyColumn struct {
	name  Symbol
	kind  Kind
	array Array
}

func newRowKeyEncoder(frame Frame, columns []Symbol) (rowKeyEncoder, error) {
	return newRowKeyEncoderWithKinds(frame, columns, nil)
}

func newRowKeyEncoderWithKinds(frame Frame, columns []Symbol, targetKinds []Kind) (rowKeyEncoder, error) {
	encoder := rowKeyEncoder{columns: make([]keyColumn, len(columns))}
	for i, name := range columns {
		col, ok := frame.Column(name)
		if !ok {
			return rowKeyEncoder{}, fmt.Errorf("key column %q does not exist", name)
		}
		kind := col.Kind()
		if i < len(targetKinds) && targetKinds[i] != "" {
			kind = targetKinds[i]
		}
		encoder.columns[i] = keyColumn{name: name, kind: kind, array: col}
	}
	if len(encoder.columns) == 1 && len(targetKinds) == 0 {
		encoder.single = singleColumnKeyFunc(encoder.columns[0])
	}
	return encoder, nil
}

func (e rowKeyEncoder) key(row int) (string, error) {
	if e.single != nil {
		return e.single(row)
	}
	var b strings.Builder
	return e.keyWithBuilder(row, &b)
}

func (e rowKeyEncoder) keyWithBuilder(row int, b *strings.Builder) (string, error) {
	if e.single != nil {
		return e.single(row)
	}
	b.Reset()
	for _, col := range e.columns {
		v, ok := col.array.At(row)
		if !ok {
			return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
		}
		var err error
		v, err = normalizeKeyValue(col.kind, v)
		if err != nil {
			return "", fmt.Errorf("key column %q: %w", col.name, err)
		}
		appendKeyPart(b, col.kind, v)
	}
	return b.String(), nil
}

func (e rowKeyEncoder) lookupKeyWithBuilder(row int, b *strings.Builder) (string, bool, error) {
	if e.single != nil {
		key, err := e.single(row)
		return key, err == nil, err
	}
	b.Reset()
	for _, col := range e.columns {
		v, ok := col.array.At(row)
		if !ok {
			return "", false, fmt.Errorf("key column %q row %d out of range", col.name, row)
		}
		var err error
		v, err = normalizeKeyValue(col.kind, v)
		if err != nil {
			return "", false, nil
		}
		appendKeyPart(b, col.kind, v)
	}
	return b.String(), true, nil
}

func singleColumnKeyFunc(col keyColumn) func(row int) (string, error) {
	switch a := col.array.(type) {
	case attributedArray:
		return singleColumnKeyFunc(keyColumn{name: col.name, kind: col.kind, array: a.array})
	case columnArray[bool]:
		return func(row int) (string, error) {
			if row < 0 || row >= len(a.data) {
				return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
			}
			return arrayValueKey(col.kind, a.data[row]), nil
		}
	case columnArray[int32]:
		return func(row int) (string, error) {
			if row < 0 || row >= len(a.data) {
				return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
			}
			return arrayValueKey(col.kind, a.data[row]), nil
		}
	case columnArray[int64]:
		return func(row int) (string, error) {
			if row < 0 || row >= len(a.data) {
				return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
			}
			return arrayValueKey(col.kind, a.data[row]), nil
		}
	case columnArray[float64]:
		return func(row int) (string, error) {
			if row < 0 || row >= len(a.data) {
				return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
			}
			return arrayValueKey(col.kind, a.data[row]), nil
		}
	case columnArray[string]:
		return func(row int) (string, error) {
			if row < 0 || row >= len(a.data) {
				return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
			}
			return arrayValueKey(col.kind, a.data[row]), nil
		}
	case columnArray[Symbol]:
		return func(row int) (string, error) {
			if row < 0 || row >= len(a.data) {
				return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
			}
			return arrayValueKey(col.kind, a.data[row]), nil
		}
	default:
		return nil
	}
}

func int64ColumnData(array Array) ([]int64, bool) {
	switch a := array.(type) {
	case attributedArray:
		return int64ColumnData(a.array)
	case columnArray[int64]:
		return a.data, true
	default:
		return nil, false
	}
}

func int64RowsSorted(values []int64, rows []int) bool {
	for i := 1; i < len(rows); i++ {
		if values[rows[i-1]] > values[rows[i]] {
			return false
		}
	}
	return true
}

func rowsSortedByTime(timeColumn Array, rows []int) bool {
	for i := 1; i < len(rows); i++ {
		leftTime, _ := timeColumn.At(rows[i-1])
		rightTime, _ := timeColumn.At(rows[i])
		if compare(leftTime, rightTime) > 0 {
			return false
		}
	}
	return true
}

func asofMatchIndexesI64(left Frame, leftTime []int64, leftPartitionColumns []Symbol, rightTime []int64, rightByPartition map[string][]int, partitionKinds []Kind) ([]int, error) {
	encoder, err := newRowKeyEncoderWithKinds(left, leftPartitionColumns, partitionKinds)
	if err != nil {
		return nil, err
	}
	rightIndexes := make([]int, left.Len())
	var b strings.Builder
	for row := 0; row < left.Len(); row++ {
		rightIndexes[row] = -1
		key, err := encoder.keyWithBuilder(row, &b)
		if err != nil {
			return nil, err
		}
		rows := rightByPartition[key]
		if len(rows) == 0 {
			continue
		}
		timeValue := leftTime[row]
		match := sort.Search(len(rows), func(i int) bool {
			return rightTime[rows[i]] > timeValue
		}) - 1
		if match >= 0 {
			rightIndexes[row] = rows[match]
		}
	}
	return rightIndexes, nil
}

func windowMatchIndexesI64(left Frame, leftTime []int64, leftPartitionColumns []Symbol, rightTime []int64, rightByPartition map[string][]int, partitionKinds []Kind) ([][]int, error) {
	encoder, err := newRowKeyEncoderWithKinds(left, leftPartitionColumns, partitionKinds)
	if err != nil {
		return nil, err
	}
	rightIndexes := make([][]int, left.Len())
	var b strings.Builder
	for row := 0; row < left.Len(); row++ {
		key, err := encoder.keyWithBuilder(row, &b)
		if err != nil {
			return nil, err
		}
		rows := rightByPartition[key]
		if len(rows) == 0 {
			rightIndexes[row] = []int{}
			continue
		}
		timeValue := leftTime[row]
		end := sort.Search(len(rows), func(i int) bool {
			return rightTime[rows[i]] > timeValue
		})
		rightIndexes[row] = cloneWindowRows(rows[:end])
	}
	return rightIndexes, nil
}

func (typedKernelRegistry) GatherWindowLists(array Array, indexes [][]int) Array {
	out := make([]any, len(indexes))
	for i, rows := range indexes {
		values := make([]any, len(rows))
		for j, row := range rows {
			v, ok := array.At(row)
			if !ok {
				panic(fmt.Sprintf("data array gather index %d out of range", row))
			}
			values[j] = v
		}
		out[i] = values
	}
	return nullableArray{kind: KindAny, data: out}
}

func (typedKernelRegistry) GatherLastOptional(array Array, indexes [][]int) Array {
	out := make([]any, len(indexes))
	for i, rows := range indexes {
		if len(rows) == 0 {
			out[i] = NullValue
			continue
		}
		v, ok := array.At(rows[len(rows)-1])
		if !ok {
			panic(fmt.Sprintf("data array gather index %d out of range", rows[len(rows)-1]))
		}
		out[i] = v
	}
	return nullableArray{kind: array.Kind(), data: out}
}

func (typedKernelRegistry) NumericAt(array Array, row int) (float64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedKernels.NumericAt(a.array, row)
	case columnArray[int8]:
		return numericColumnAt(a.data, row)
	case columnArray[int16]:
		return numericColumnAt(a.data, row)
	case columnArray[int32]:
		return numericColumnAt(a.data, row)
	case columnArray[int64]:
		return numericColumnAt(a.data, row)
	case columnArray[uint8]:
		return numericColumnAt(a.data, row)
	case columnArray[uint16]:
		return numericColumnAt(a.data, row)
	case columnArray[uint32]:
		return numericColumnAt(a.data, row)
	case columnArray[uint64]:
		return numericColumnAt(a.data, row)
	case columnArray[float32]:
		return numericColumnAt(a.data, row)
	case columnArray[float64]:
		return numericColumnAt(a.data, row)
	default:
		v, ok := array.At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		if IsNull(v) {
			return 0, false, nil
		}
		n, ok := numeric(v)
		if !ok {
			return 0, false, fmt.Errorf("aggregate expects numeric expression, got %T (%v)", v, v)
		}
		return n, true, nil
	}
}

func numericColumnAt[T signedScalar | unsignedScalar | floatScalar](values []T, row int) (float64, bool, error) {
	if row < 0 || row >= len(values) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(values[row]), true, nil
}

func numericValueAt(array Array, row int) (float64, bool) {
	v, ok := array.At(row)
	if !ok || IsNull(v) {
		return 0, false
	}
	n, ok := numeric(v)
	return n, ok
}

func typedNumericOperand(value any) bool {
	if array, ok := value.(Array); ok {
		return isNumericArray(array)
	}
	if IsNull(value) {
		return true
	}
	_, ok := numeric(value)
	return ok
}

func isNumericArray(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return isNumericArray(a.array)
	case columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		columnArray[float32], columnArray[float64]:
		return true
	case nullableArray:
		for i := 0; i < array.Len(); i++ {
			v, ok := array.At(i)
			if !ok {
				return false
			}
			if IsNull(v) {
				continue
			}
			if _, ok := numeric(v); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func numericDyadic(op Op, left, right any, length int) (Array, bool, error) {
	values := make([]any, length)
	hasNull := false
	for i := 0; i < length; i++ {
		lv, ok, err := numericOperandAt(left, i)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			values[i] = NullValue
			hasNull = true
			continue
		}
		rv, ok, err := numericOperandAt(right, i)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			values[i] = NullValue
			hasNull = true
			continue
		}
		out, err := applyNumericBinaryFloat(op, lv, rv)
		if err != nil {
			return nil, true, err
		}
		values[i] = out
	}
	if hasNull {
		return newNullableArray(KindF64, values), true, nil
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = v.(float64)
	}
	return NewF64(out), true, nil
}

func compareDyadic(op Op, left, right any, length int) (Array, bool, error) {
	out := make([]bool, length)
	for i := 0; i < length; i++ {
		lv, ok, err := operandAt(left, i)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("typed compare left row %d out of range", i)
		}
		rv, ok, err := operandAt(right, i)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("typed compare right row %d out of range", i)
		}
		if keep, ok := compareSymbolStringScalar(op, lv, rv); ok {
			out[i] = keep
			continue
		}
		result, err := ApplyBinary(op, lv, rv)
		if err != nil {
			return nil, false, nil
		}
		keep, ok := result.(bool)
		if !ok {
			return nil, true, fmt.Errorf("typed compare operator %s did not return bool", op)
		}
		out[i] = keep
	}
	return NewBool(out), true, nil
}

func compareSymbolStringScalar(op Op, left, right any) (bool, bool) {
	var ls, rs string
	switch l := left.(type) {
	case Symbol:
		ls = string(l)
	case string:
		ls = l
	default:
		return false, false
	}
	switch r := right.(type) {
	case Symbol:
		rs = string(r)
	case string:
		rs = r
	default:
		return false, false
	}
	return boolCompare(op, ls == rs, compareString(ls, rs)), true
}

func numericOperandAt(value any, row int) (float64, bool, error) {
	v, ok, err := operandAt(value, row)
	if err != nil || !ok || IsNull(v) {
		return 0, false, err
	}
	n, ok := numeric(v)
	if !ok {
		return 0, false, fmt.Errorf("typed numeric operand row %d is %T, want numeric", row, v)
	}
	return n, true, nil
}

func operandAt(value any, row int) (any, bool, error) {
	if array, ok := value.(Array); ok {
		v, ok := array.At(row)
		if !ok {
			return nil, false, fmt.Errorf("array row %d out of range", row)
		}
		return v, true, nil
	}
	return value, true, nil
}

func isArithmeticOp(op Op) bool {
	switch op {
	case OpAdd, OpSub, OpMul, OpDiv:
		return true
	default:
		return false
	}
}

func numericUnarySlice[T signedScalar | unsignedScalar | floatScalar](op string, values []T) (Array, bool, error) {
	out := make([]float64, len(values))
	for i, v := range values {
		n, err := applyNumericUnaryFloat(op, float64(v))
		if err != nil {
			return nil, true, err
		}
		out[i] = n
	}
	return NewF64(out), true, nil
}

func numericUnaryNullable(op string, values []any) (Array, bool, error) {
	out := make([]any, len(values))
	for i, v := range values {
		if IsNull(v) {
			out[i] = NullValue
			continue
		}
		n, ok := numeric(v)
		if !ok {
			return nil, false, nil
		}
		result, err := applyNumericUnaryFloat(op, n)
		if err != nil {
			return nil, true, err
		}
		out[i] = result
	}
	return newNullableArray(KindF64, out), true, nil
}

func applyNumericUnaryFloat(op string, value float64) (float64, error) {
	switch op {
	case NumericUnaryNeg, string(OpSub):
		return -value, nil
	case NumericUnaryAbs:
		return math.Abs(value), nil
	case NumericUnarySqrt:
		return math.Sqrt(value), nil
	case NumericUnaryLog:
		return math.Log(value), nil
	case NumericUnaryExp:
		return math.Exp(value), nil
	case NumericUnaryRecip:
		return 1 / value, nil
	case NumericUnarySignum:
		switch {
		case value > 0:
			return 1, nil
		case value < 0:
			return -1, nil
		default:
			return 0, nil
		}
	case NumericUnaryFloor:
		return math.Floor(value), nil
	case NumericUnaryCeiling:
		return math.Ceil(value), nil
	default:
		return 0, fmt.Errorf("unsupported numeric unary kernel %q", op)
	}
}

func applyNumericBinaryFloat(op Op, left, right float64) (float64, error) {
	switch op {
	case OpAdd:
		return left + right, nil
	case OpSub:
		return left - right, nil
	case OpMul:
		return left * right, nil
	case OpDiv:
		return left / right, nil
	default:
		return 0, fmt.Errorf("unsupported numeric binary kernel %s", op)
	}
}

func numericSumSlice[T signedScalar | unsignedScalar | floatScalar](values []T) (float64, int64, bool, error) {
	var sum float64
	for _, v := range values {
		sum += float64(v)
	}
	return sum, int64(len(values)), true, nil
}

func minMax(array Array, mode string) (any, bool, bool, error) {
	switch a := array.(type) {
	case columnArray[bool]:
		return minMaxOrdered(a.data, mode)
	case columnArray[int8]:
		return minMaxOrdered(a.data, mode)
	case columnArray[int16]:
		return minMaxOrdered(a.data, mode)
	case columnArray[int32]:
		return minMaxOrdered(a.data, mode)
	case columnArray[int64]:
		return minMaxOrdered(a.data, mode)
	case columnArray[uint8]:
		return minMaxOrdered(a.data, mode)
	case columnArray[uint16]:
		return minMaxOrdered(a.data, mode)
	case columnArray[uint32]:
		return minMaxOrdered(a.data, mode)
	case columnArray[uint64]:
		return minMaxOrdered(a.data, mode)
	case columnArray[float32]:
		return minMaxOrdered(a.data, mode)
	case columnArray[float64]:
		return minMaxOrdered(a.data, mode)
	case columnArray[string]:
		return minMaxOrdered(a.data, mode)
	case columnArray[Symbol]:
		return minMaxOrdered(a.data, mode)
	case columnArray[Month]:
		return minMaxOrdered(a.data, mode)
	case columnArray[Date]:
		return minMaxOrdered(a.data, mode)
	case columnArray[DateTime]:
		return minMaxOrdered(a.data, mode)
	case columnArray[Timespan]:
		return minMaxOrdered(a.data, mode)
	case columnArray[Minute]:
		return minMaxOrdered(a.data, mode)
	case columnArray[Second]:
		return minMaxOrdered(a.data, mode)
	case columnArray[Time]:
		return minMaxOrdered(a.data, mode)
	case columnArray[Timestamp]:
		return minMaxOrdered(a.data, mode)
	case nullableArray:
		var best any
		hasBest := false
		for _, v := range a.data {
			if IsNull(v) {
				continue
			}
			if !hasBest || (mode == "min" && compare(v, best) < 0) || (mode == "max" && compare(v, best) > 0) {
				best = v
				hasBest = true
			}
		}
		return best, hasBest, true, nil
	default:
		return nil, false, false, nil
	}
}

func minMaxOrdered[T any](values []T, mode string) (any, bool, bool, error) {
	if len(values) == 0 {
		return nil, false, true, nil
	}
	best := values[0]
	for _, v := range values[1:] {
		cmp := compare(v, best)
		if (mode == "min" && cmp < 0) || (mode == "max" && cmp > 0) {
			best = v
		}
	}
	return best, true, true, nil
}

type signedScalar interface {
	~int8 | ~int16 | ~int32 | ~int64
}

type unsignedScalar interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64
}

type floatScalar interface {
	~float32 | ~float64
}

func compareSignedSlice[T signedScalar](values []T, target T, ok bool, op Op, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = boolCompare(op, int64(v) == int64(target), compareInt64(int64(v), int64(target)))
	}
	return true
}

func compareBoolIndexes(values []bool, target bool, ok bool, op Op, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if boolCompare(op, v == target, compareBool(v, target)) {
			out = append(out, i)
		}
	}
	return out, true
}

func compareEncodedMask(array encodedArray, op Op, value any, out []bool) bool {
	code, ok := encodedComparableCode(array, value)
	if !ok || (op != OpEQ && op != OpNE) {
		return false
	}
	for i, rowCode := range array.codes {
		matched := rowCode == code
		if code < 0 {
			matched = rowCode < 0
		}
		out[i] = matched
		if op == OpNE {
			out[i] = !out[i]
		}
	}
	return true
}

func compareEncodedIndexes(array encodedArray, op Op, value any, out []int) ([]int, bool) {
	code, ok := encodedComparableCode(array, value)
	if !ok || (op != OpEQ && op != OpNE) {
		return nil, false
	}
	out = out[:0]
	for i, rowCode := range array.codes {
		matched := rowCode == code
		if code < 0 {
			matched = rowCode < 0
		}
		if op == OpNE {
			matched = !matched
		}
		if matched {
			out = append(out, i)
		}
	}
	return out, true
}

func encodedComparableCode(array encodedArray, value any) (int32, bool) {
	if IsNull(value) {
		return -1, true
	}
	normalized := normalizeScalar(array.kind, value)
	if array.kind == KindSymbol {
		target, ok := normalized.(Symbol)
		if !ok {
			return -1, false
		}
		for code, domainValue := range array.domain {
			if domainSymbol, ok := domainValue.(Symbol); ok && domainSymbol == target {
				return int32(code), true
			}
		}
		return -1, false
	}
	if array.kind == KindString {
		target, ok := normalized.(string)
		if !ok {
			return -1, false
		}
		for code, domainValue := range array.domain {
			if domainString, ok := domainValue.(string); ok && domainString == target {
				return int32(code), true
			}
		}
		return -1, false
	}
	key := arrayValueKey(array.kind, normalized)
	for code, domainValue := range array.domain {
		if arrayValueKey(array.kind, domainValue) == key {
			return int32(code), true
		}
	}
	return -1, false
}

func compareSignedIndexes[T signedScalar](values []T, target T, ok bool, op Op, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if boolCompare(op, int64(v) == int64(target), compareInt64(int64(v), int64(target))) {
			out = append(out, i)
		}
	}
	return out, true
}

func compareUnsignedIndexes[T unsignedScalar](values []T, target T, ok bool, op Op, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if boolCompare(op, uint64(v) == uint64(target), compareUint64(uint64(v), uint64(target))) {
			out = append(out, i)
		}
	}
	return out, true
}

func compareFloatIndexes[T floatScalar](values []T, target T, ok bool, op Op, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if boolCompare(op, float64(v) == float64(target), compareFloat64(float64(v), float64(target))) {
			out = append(out, i)
		}
	}
	return out, true
}

func compareStringIndexes(values []string, target string, ok bool, op Op, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if boolCompare(op, v == target, compareString(v, target)) {
			out = append(out, i)
		}
	}
	return out, true
}

func compareSymbolIndexes(values []Symbol, target Symbol, ok bool, op Op, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if boolCompare(op, v == target, compareString(string(v), string(target))) {
			out = append(out, i)
		}
	}
	return out, true
}

func compareUnsignedSlice[T unsignedScalar](values []T, target T, ok bool, op Op, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = boolCompare(op, uint64(v) == uint64(target), compareUint64(uint64(v), uint64(target)))
	}
	return true
}

func compareFloatSlice[T floatScalar](values []T, target T, ok bool, op Op, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = boolCompare(op, float64(v) == float64(target), compareFloat64(float64(v), float64(target)))
	}
	return true
}

func compareStringSlice(values []string, target string, ok bool, op Op, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = boolCompare(op, v == target, compareString(v, target))
	}
	return true
}

func compareSymbolSlice(values []Symbol, target Symbol, ok bool, op Op, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = boolCompare(op, v == target, compareString(string(v), string(target)))
	}
	return true
}

func withinSignedSlice[T signedScalar](values []T, low, high T, ok bool, highClosed bool, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = v >= low && (v < high || (highClosed && v == high))
	}
	return true
}

func withinSignedIndexes[T signedScalar](values []T, low, high T, ok bool, highClosed bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if v >= low && (v < high || (highClosed && v == high)) {
			out = append(out, i)
		}
	}
	return out, true
}

func withinUnsignedSlice[T unsignedScalar](values []T, low, high T, ok bool, highClosed bool, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = v >= low && (v < high || (highClosed && v == high))
	}
	return true
}

func withinUnsignedIndexes[T unsignedScalar](values []T, low, high T, ok bool, highClosed bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if v >= low && (v < high || (highClosed && v == high)) {
			out = append(out, i)
		}
	}
	return out, true
}

func withinFloatSlice[T floatScalar](values []T, low, high T, ok bool, highClosed bool, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = v >= low && (v < high || (highClosed && v == high))
	}
	return true
}

func withinFloatIndexes[T floatScalar](values []T, low, high T, ok bool, highClosed bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if v >= low && (v < high || (highClosed && v == high)) {
			out = append(out, i)
		}
	}
	return out, true
}

func withinStringSlice(values []string, low, high string, ok bool, highClosed bool, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = v >= low && (v < high || (highClosed && v == high))
	}
	return true
}

func withinStringIndexes(values []string, low, high string, ok bool, highClosed bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if v >= low && (v < high || (highClosed && v == high)) {
			out = append(out, i)
		}
	}
	return out, true
}

func withinSymbolSlice(values []Symbol, low, high Symbol, ok bool, highClosed bool, out []bool) bool {
	if !ok {
		return false
	}
	for i, v := range values {
		out[i] = v >= low && (v < high || (highClosed && v == high))
	}
	return true
}

func withinSymbolIndexes(values []Symbol, low, high Symbol, ok bool, highClosed bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i, v := range values {
		if v >= low && (v < high || (highClosed && v == high)) {
			out = append(out, i)
		}
	}
	return out, true
}
