package data

import (
	"fmt"
	"math"
	"path"
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
	case i64RangeArray:
		v, ok := coerceInt64Exact(value)
		return compareI64RangeMask(a, v, ok, op, out)
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
	case i64RangeArray:
		v, ok := coerceInt64Exact(value)
		return compareI64RangeIndexes(a, v, ok, op, out)
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
		if _, ok := arrayIndexForBorrowed(array, ArrayAttributeUnique); ok {
			return []int{}, true
		}
		if _, ok := arrayIndexForBorrowed(array, ArrayAttributeGrouped); ok {
			return []int{}, true
		}
		return nil, false
	}
	index, ok := arrayIndexForBorrowed(array, ArrayAttributeUnique)
	if !ok {
		index, ok = arrayIndexForBorrowed(array, ArrayAttributeGrouped)
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

func (k typedKernelRegistry) InIndexes(array Array, values []any, out []int) ([]int, bool) {
	switch a := array.(type) {
	case attributedArray:
		return k.InIndexes(a.array, values, out)
	case encodedArray:
		return encodedInIndexes(a, values, out)
	case columnArray[bool]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := boolMembership(values)
		return membershipBoolIndexes(a.data, set, ok, out)
	case columnArray[int8]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int8](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[int16]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int16](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[int32]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int32](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[int64]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := int64Membership(values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[uint8]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint8](values)
		return membershipUnsignedIndexes(a.data, set, ok, out)
	case columnArray[uint16]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint16](values)
		return membershipUnsignedIndexes(a.data, set, ok, out)
	case columnArray[uint32]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint32](values)
		return membershipUnsignedIndexes(a.data, set, ok, out)
	case columnArray[uint64]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint64](values)
		return membershipUnsignedIndexes(a.data, set, ok, out)
	case columnArray[string]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := stringMembership(values)
		return membershipStringIndexes(a.data, set, ok, out)
	case columnArray[Symbol]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := symbolMembership(values)
		return membershipSymbolIndexes(a.data, set, ok, out)
	case columnArray[Month]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Month](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[Date]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Date](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[DateTime]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[DateTime](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[Timespan]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Timespan](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[Minute]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Minute](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[Second]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Second](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[Time]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Time](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	case columnArray[Timestamp]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Timestamp](values)
		return membershipSignedIndexes(a.data, set, ok, out)
	default:
		return nil, false
	}
}

func normalizeMembershipValues(kind Kind, values []any) []any {
	if len(values) == 0 {
		return values
	}
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = normalizeScalar(kind, value)
	}
	return out
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
	rowToGroup, err := rowToGroupFromIndex(index)
	if err != nil {
		return nil, nil, false, err
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
		return nil, states, true, nil
	}
	rowToGroup, err := rowToGroupFromIndex(index)
	if err != nil {
		return nil, nil, true, err
	}
	seen := make([]bool, len(index.Rows))
	groupOrder := make([]int, 0, len(index.Rows))
	for _, row := range indexes {
		if row < 0 || row >= len(rowToGroup) {
			return nil, nil, true, fmt.Errorf("filter row %d out of range for grouped index", row)
		}
		group := rowToGroup[row]
		if group < 0 {
			return nil, nil, true, fmt.Errorf("filter row %d is missing from grouped index", row)
		}
		if !seen[group] {
			seen[group] = true
			groupOrder = append(groupOrder, group)
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
			if agg.column != nil && isNumericArray(agg.column) {
				continue
			}
			if agg.leftColumn != nil && agg.rightColumn != nil && isNumericArray(agg.leftColumn) && isNumericArray(agg.rightColumn) {
				continue
			}
			{
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
	case "sum", "avg":
		var sum float64
		var count int64
		var ok bool
		var err error
		if agg.leftColumn != nil && agg.rightColumn != nil {
			sum, count, ok, err = typedKernels.NumericBinarySumRows(agg.leftColumn, agg.binaryOp, agg.rightColumn, rows)
		} else {
			sum, count, ok, err = typedKernels.NumericSumRows(agg.column, rows)
		}
		if err != nil || ok {
			state.sum = sum
			state.count = count
			return err
		}
		fallthrough
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
		n, ok, err := aggregateIndexedNumericValue(agg, row)
		if err != nil || !ok {
			return err
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
	if len(index.RowToGroup) > 0 {
		return index.RowToGroup, nil
	}
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

func TryTypedNot(array Array) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("not array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		out, handled, err := TryTypedNot(a.array)
		if err != nil || !handled {
			return nil, handled, err
		}
		return out, true, nil
	default:
		if !notMaskAcceptsArray(array) {
			return nil, false, nil
		}
		return notMask{array: array}, true, nil
	}
}

func notMaskAcceptsArray(array Array) bool {
	switch array.Kind() {
	case KindBool, KindI8, KindI16, KindI32, KindI64, KindU8, KindU16, KindU32, KindU64, KindF32, KindF64:
		return true
	default:
		return false
	}
}

func TryTypedTrueCount(mask Array) (int64, bool, error) {
	if mask == nil {
		return 0, true, fmt.Errorf("true count mask is nil")
	}
	if mask.Kind() != KindBool {
		return 0, true, fmt.Errorf("true count mask kind is %s, want %s", mask.Kind(), KindBool)
	}
	switch a := mask.(type) {
	case attributedArray:
		return TryTypedTrueCount(a.array)
	case i64RangeCompareMask:
		return a.trueCount(), true, nil
	case i64ScalarDyadicCompareMask:
		return a.trueCount()
	case boolLogicalMask:
		return a.trueCount()
	case notMask:
		return a.trueCount()
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return 0, true, nil
		}
		sourceCount, handled, err := TryTypedTrueCount(a.source)
		if err != nil || !handled {
			return 0, handled, err
		}
		fullCycles := a.len / sourceLen
		remainder := a.len % sourceLen
		count := sourceCount * int64(fullCycles)
		for row := 0; row < remainder; row++ {
			value, ok, err := boolArrayAt(a.source, (a.start+row)%sourceLen)
			if err != nil || !ok {
				return 0, ok, err
			}
			if value {
				count++
			}
		}
		return count, true, nil
	case columnArray[bool]:
		var count int64
		for _, keep := range a.data {
			if keep {
				count++
			}
		}
		return count, true, nil
	case nullableArray:
		var count int64
		for row, value := range a.data {
			if IsNull(value) {
				continue
			}
			keep, ok := value.(bool)
			if !ok {
				return 0, true, fmt.Errorf("true count mask row %d is %T, want bool", row, value)
			}
			if keep {
				count++
			}
		}
		return count, true, nil
	default:
		return 0, false, nil
	}
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

func TryTypedNullCount(array Array) (int64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedNullCount(a.array)
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 {
			return 0, true, nil
		}
		var count int64
		fullCycles := a.len / sourceLen
		remainder := a.len % sourceLen
		for row := 0; row < sourceLen; row++ {
			value, ok := a.source.At((a.start + row) % sourceLen)
			if !ok {
				return 0, true, fmt.Errorf("tiled null count source row %d out of range", row)
			}
			if IsNull(value) {
				count += int64(fullCycles)
				if row < remainder {
					count++
				}
			}
		}
		return count, true, nil
	case nullableArray:
		var count int64
		for _, value := range a.data {
			if IsNull(value) {
				count++
			}
		}
		return count, true, nil
	default:
		return 0, true, nil
	}
}

func TryTypedDistinctCount(array Array) (int64, bool, error) {
	if array == nil {
		return 0, true, fmt.Errorf("distinct count array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedDistinctCount(a.array)
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return 0, true, nil
		}
		if a.len >= sourceLen {
			return TryTypedDistinctCount(a.source)
		}
		return distinctCountRows(a, a.len)
	case i64RangeArray:
		if a.len == 0 {
			return 0, true, nil
		}
		if a.step == 0 {
			return 1, true, nil
		}
		return int64(a.len), true, nil
	case f64RangeArray:
		if a.len == 0 {
			return 0, true, nil
		}
		if a.step == 0 {
			return 1, true, nil
		}
		return int64(a.len), true, nil
	case columnArray[bool]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[int8]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[int16]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[int32]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[int64]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[uint8]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[uint16]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[uint32]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[uint64]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[float32]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[float64]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[string]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[Symbol]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[Month]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[Date]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[DateTime]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[Timespan]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[Minute]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[Second]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[Time]:
		return distinctCountComparable(a.data), true, nil
	case columnArray[Timestamp]:
		return distinctCountComparable(a.data), true, nil
	default:
		return distinctCountRows(array, array.Len())
	}
}

func distinctCountComparable[T comparable](values []T) int64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) <= 16 {
		seen := make([]T, 0, len(values))
		for _, value := range values {
			found := false
			for _, existing := range seen {
				if existing == value {
					found = true
					break
				}
			}
			if !found {
				seen = append(seen, value)
			}
		}
		return int64(len(seen))
	}
	seen := make(map[T]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	return int64(len(seen))
}

func distinctCountRows(array Array, rows int) (int64, bool, error) {
	if rows <= 0 {
		return 0, true, nil
	}
	seen := make(map[string]struct{}, min(rows, 64))
	for row := 0; row < rows; row++ {
		value, ok := array.At(row)
		if !ok {
			return 0, true, fmt.Errorf("distinct count row %d out of range", row)
		}
		seen[arrayValueKey(array.Kind(), value)] = struct{}{}
	}
	return int64(len(seen)), true, nil
}

// TryTypedStringLikeCount counts string/symbol rows matching a q-like glob
// pattern without materializing a boolean mask or where index vector.
func TryTypedStringLikeCount(array Array, pattern string) (int64, bool, error) {
	if array == nil {
		return 0, true, fmt.Errorf("like count array is nil")
	}
	matcher, err := newStringLikeMatcher(pattern)
	if err != nil {
		return 0, true, err
	}
	return typedStringLikeCount(array, matcher)
}

// TryTypedInCount counts rows whose values are members of values without
// materializing a boolean mask or row-index vector.
func TryTypedInCount(array Array, values []any) (int64, bool, error) {
	if array == nil {
		return 0, true, fmt.Errorf("in count array is nil")
	}
	return typedInCount(array, values)
}

// TryTypedBoolLogical composes boolean masks without routing each row through
// Array.At or []any boxing. Scalars are broadcast using q vector rules.
func TryTypedBoolLogical(op string, left, right any) (Array, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray && !rightIsArray {
		return nil, false, nil
	}
	length := 0
	switch {
	case leftIsArray && rightIsArray:
		if leftArray.Kind() != KindBool || rightArray.Kind() != KindBool {
			return nil, false, nil
		}
		if leftArray.Len() != rightArray.Len() && leftArray.Len() != 1 && rightArray.Len() != 1 {
			return nil, true, fmt.Errorf("logical length mismatch")
		}
		length = leftArray.Len()
		if rightArray.Len() > length {
			length = rightArray.Len()
		}
	case leftIsArray:
		if leftArray.Kind() != KindBool {
			return nil, false, nil
		}
		if _, ok := left.(Array); ok && right == nil {
			return nil, false, nil
		}
		length = leftArray.Len()
	case rightIsArray:
		if rightArray.Kind() != KindBool {
			return nil, false, nil
		}
		length = rightArray.Len()
	}
	if leftIsArray && rightIsArray {
		return boolLogicalMask{op: op, left: leftArray, right: rightArray, len: length}, true, nil
	}
	if leftIsArray {
		rv, ok := boolScalarValue(right)
		if !ok {
			return nil, false, nil
		}
		return boolLogicalMask{op: op, left: leftArray, rightScalar: rv, rightIsScalar: true, len: length}, true, nil
	}
	lv, ok := boolScalarValue(left)
	if !ok {
		return nil, false, nil
	}
	return boolLogicalMask{op: op, leftScalar: lv, leftIsScalar: true, right: rightArray, len: length}, true, nil
}

func typedInCount(array Array, values []any) (int64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedInCount(a.array, values)
	case encodedArray:
		codes := make(map[int32]struct{}, len(values))
		for _, value := range values {
			code, ok := encodedComparableCode(a, value)
			if !ok {
				return 0, false, nil
			}
			codes[code] = struct{}{}
		}
		var count int64
		for _, code := range a.codes {
			if _, matched := codes[code]; matched {
				count++
			}
		}
		return count, true, nil
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return 0, true, nil
		}
		sourceCount, handled, err := typedInCount(a.source, values)
		if err != nil || !handled {
			return 0, handled, err
		}
		fullCycles := a.len / sourceLen
		remainder := a.len % sourceLen
		count := sourceCount * int64(fullCycles)
		matches, ok := typedInPredicate(a.source.Kind(), values)
		if !ok {
			return 0, false, nil
		}
		for row := 0; row < remainder; row++ {
			value, ok := a.source.At((a.start + row) % sourceLen)
			if !ok {
				return 0, true, fmt.Errorf("tiled in count source row %d out of range", row)
			}
			matched, ok := matches(value)
			if !ok {
				return 0, false, nil
			}
			if matched {
				count++
			}
		}
		return count, true, nil
	case columnArray[bool]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := boolMembership(values)
		return countMembershipBool(a.data, set, ok), ok, nil
	case columnArray[int8]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int8](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[int16]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int16](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[int32]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int32](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[int64]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := int64Membership(values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[uint8]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint8](values)
		return countMembershipUnsigned(a.data, set, ok), ok, nil
	case columnArray[uint16]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint16](values)
		return countMembershipUnsigned(a.data, set, ok), ok, nil
	case columnArray[uint32]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint32](values)
		return countMembershipUnsigned(a.data, set, ok), ok, nil
	case columnArray[uint64]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint64](values)
		return countMembershipUnsigned(a.data, set, ok), ok, nil
	case columnArray[string]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := stringMembership(values)
		return countMembershipString(a.data, set, ok), ok, nil
	case columnArray[Symbol]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := symbolMembership(values)
		return countMembershipSymbol(a.data, set, ok), ok, nil
	case columnArray[Month]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Month](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[Date]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Date](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[DateTime]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[DateTime](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[Timespan]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Timespan](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[Minute]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Minute](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[Second]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Second](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[Time]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Time](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	case columnArray[Timestamp]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Timestamp](values)
		return countMembershipSigned(a.data, set, ok), ok, nil
	default:
		return 0, false, nil
	}
}

func typedStringLikeCount(array Array, matcher stringLikeMatcher) (int64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedStringLikeCount(a.array, matcher)
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return 0, true, nil
		}
		sourceCount, handled, err := typedStringLikeCount(a.source, matcher)
		if err != nil || !handled {
			return 0, handled, err
		}
		fullCycles := a.len / sourceLen
		remainder := a.len % sourceLen
		count := sourceCount * int64(fullCycles)
		for row := 0; row < remainder; row++ {
			value, ok := a.source.At((a.start + row) % sourceLen)
			if !ok {
				return 0, true, fmt.Errorf("tiled like count source row %d out of range", row)
			}
			matched, ok, err := matcher.matchValue(value)
			if err != nil || !ok {
				return 0, ok, err
			}
			if matched {
				count++
			}
		}
		return count, true, nil
	case columnArray[string]:
		return matcher.countStrings(a.data)
	case columnArray[Symbol]:
		return matcher.countSymbols(a.data)
	case nullableArray:
		var count int64
		for _, value := range a.data {
			if IsNull(value) {
				continue
			}
			matched, ok, err := matcher.matchValue(value)
			if err != nil || !ok {
				return 0, ok, err
			}
			if matched {
				count++
			}
		}
		return count, true, nil
	default:
		return 0, false, nil
	}
}

type stringLikeMatcher struct {
	pattern string
	prefix  string
	fast    bool
}

func newStringLikeMatcher(pattern string) (stringLikeMatcher, error) {
	if _, err := path.Match(pattern, ""); err != nil {
		return stringLikeMatcher{}, fmt.Errorf("like pattern %q: %w", pattern, err)
	}
	prefix, fast := simpleTrailingStarPattern(pattern)
	return stringLikeMatcher{pattern: pattern, prefix: prefix, fast: fast}, nil
}

func simpleTrailingStarPattern(pattern string) (string, bool) {
	if !strings.HasSuffix(pattern, "*") {
		return "", false
	}
	prefix := strings.TrimSuffix(pattern, "*")
	if strings.ContainsAny(prefix, "*?[\\") {
		return "", false
	}
	return prefix, true
}

func (m stringLikeMatcher) countStrings(values []string) (int64, bool, error) {
	var count int64
	for _, value := range values {
		matched, err := m.matchString(value)
		if err != nil {
			return 0, true, err
		}
		if matched {
			count++
		}
	}
	return count, true, nil
}

func (m stringLikeMatcher) countSymbols(values []Symbol) (int64, bool, error) {
	var count int64
	for _, value := range values {
		matched, err := m.matchString(string(value))
		if err != nil {
			return 0, true, err
		}
		if matched {
			count++
		}
	}
	return count, true, nil
}

func (m stringLikeMatcher) matchValue(value any) (bool, bool, error) {
	switch x := value.(type) {
	case string:
		matched, err := m.matchString(x)
		return matched, true, err
	case Symbol:
		matched, err := m.matchString(string(x))
		return matched, true, err
	default:
		return false, false, nil
	}
}

func (m stringLikeMatcher) matchString(value string) (bool, error) {
	if m.fast {
		return strings.HasPrefix(value, m.prefix), nil
	}
	matched, err := path.Match(m.pattern, value)
	if err != nil {
		return false, fmt.Errorf("like pattern %q: %w", m.pattern, err)
	}
	return matched, nil
}

// TryTypedStringCast converts string/symbol arrays to string arrays while
// preserving lazy tiled storage when possible.
func TryTypedStringCast(array Array) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("string cast array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		out, handled, err := TryTypedStringCast(a.array)
		if err != nil || !handled {
			return out, handled, err
		}
		return attributedArray{array: out, metadata: a.metadata.cloneWithRebuiltIndexes(out)}, true, nil
	case tiledArray:
		source, handled, err := TryTypedStringCast(a.source)
		if err != nil || !handled {
			return source, handled, err
		}
		return tiledArray{source: source, start: a.start, len: a.len}, true, nil
	case columnArray[string]:
		return a, true, nil
	case columnArray[Symbol]:
		out := make([]string, len(a.data))
		for i, value := range a.data {
			out[i] = string(value)
		}
		return columnArray[string]{kind: KindString, data: out}, true, nil
	case nullableArray:
		out := make([]any, len(a.data))
		for i, value := range a.data {
			if IsNull(value) {
				out[i] = NullValue
				continue
			}
			switch x := value.(type) {
			case string:
				out[i] = x
			case Symbol:
				out[i] = string(x)
			default:
				return nil, false, nil
			}
		}
		return nullableArray{kind: KindString, data: out}, true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedStringCase applies lower/upper to string/symbol arrays while
// preserving lazy tiled storage when possible.
func TryTypedStringCase(array Array, upper bool) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("string case array is nil")
	}
	fn := strings.ToLower
	if upper {
		fn = strings.ToUpper
	}
	switch a := array.(type) {
	case attributedArray:
		out, handled, err := TryTypedStringCase(a.array, upper)
		if err != nil || !handled {
			return out, handled, err
		}
		return attributedArray{array: out, metadata: a.metadata.cloneWithRebuiltIndexes(out)}, true, nil
	case tiledArray:
		source, handled, err := TryTypedStringCase(a.source, upper)
		if err != nil || !handled {
			return source, handled, err
		}
		return tiledArray{source: source, start: a.start, len: a.len}, true, nil
	case columnArray[string]:
		out := make([]string, len(a.data))
		for i, value := range a.data {
			out[i] = fn(value)
		}
		return columnArray[string]{kind: KindString, data: out}, true, nil
	case columnArray[Symbol]:
		out := make([]string, len(a.data))
		for i, value := range a.data {
			out[i] = fn(string(value))
		}
		return columnArray[string]{kind: KindString, data: out}, true, nil
	case nullableArray:
		out := make([]any, len(a.data))
		for i, value := range a.data {
			if IsNull(value) {
				out[i] = NullValue
				continue
			}
			switch x := value.(type) {
			case string:
				out[i] = fn(x)
			case Symbol:
				out[i] = fn(string(x))
			default:
				return nil, false, nil
			}
		}
		return nullableArray{kind: KindString, data: out}, true, nil
	default:
		return nil, false, nil
	}
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

func TryTypedQNumericUnary(op string, array Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedQNumericUnary(op, a.array)
	case columnArray[float32]:
		return qNumericUnaryFloatSlice(op, a.data)
	case columnArray[float64]:
		return qNumericUnaryFloatSlice(op, a.data)
	}
	if !isDenseIntegerArray(array) {
		return nil, false, nil
	}
	return qNumericUnaryIntegerArray(op, array)
}

func TryTypedQNumericUnarySum(op string, array Array) (any, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedQNumericUnarySum(op, a.array)
	case tiledArray:
		if out, ok, err := qNumericUnarySumTiled(op, a); ok || err != nil {
			return out, ok, err
		}
	case i64RangeArray:
		if out, ok := qNumericUnarySumI64Range(op, a); ok {
			return out, true, nil
		}
	case columnArray[float32]:
		return qNumericUnarySumFloatSlice(op, a.data)
	case columnArray[float64]:
		return qNumericUnarySumFloatSlice(op, a.data)
	}
	if !isDenseIntegerArray(array) {
		return nil, false, nil
	}
	return qNumericUnarySumIntegerArray(op, array)
}

func qNumericUnarySumTiled(op string, array tiledArray) (any, bool, error) {
	sourceLen := array.source.Len()
	if array.len == 0 {
		if qNumericUnaryTiledSumReturnsFloat(op, array.source) {
			return float64(0), true, nil
		}
		if qNumericUnaryTiledSumReturnsInt(op, array.source) {
			return int64(0), true, nil
		}
		return nil, false, nil
	}
	if sourceLen == 0 {
		return nil, false, nil
	}
	cycles := array.len / sourceLen
	remainder := array.len % sourceLen
	if qNumericUnaryTiledSumReturnsFloat(op, array.source) {
		period, ok, err := qNumericUnaryFloatSumWindow(op, array.source, array.start, sourceLen)
		if err != nil || !ok {
			return nil, ok, err
		}
		tail, ok, err := qNumericUnaryFloatSumWindow(op, array.source, array.start, remainder)
		if err != nil || !ok {
			return nil, ok, err
		}
		return period*float64(cycles) + tail, true, nil
	}
	if qNumericUnaryTiledSumReturnsInt(op, array.source) {
		period, ok, err := qNumericUnaryIntSumWindow(op, array.source, array.start, sourceLen)
		if err != nil || !ok {
			return nil, ok, err
		}
		tail, ok, err := qNumericUnaryIntSumWindow(op, array.source, array.start, remainder)
		if err != nil || !ok {
			return nil, ok, err
		}
		return period*int64(cycles) + tail, true, nil
	}
	return nil, false, nil
}

func TryTypedQNumericUnaryDyadicSum(unaryOp string, dyadicOp Op, left, right any) (any, bool, error) {
	if !isArithmeticOp(dyadicOp) {
		return nil, false, nil
	}
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray && !rightIsArray {
		return nil, false, nil
	}
	length := 0
	switch {
	case leftIsArray && rightIsArray:
		if leftArray.Len() != rightArray.Len() {
			return nil, true, fmt.Errorf("typed unary dyadic sum length mismatch: %d != %d", leftArray.Len(), rightArray.Len())
		}
		length = leftArray.Len()
	case leftIsArray:
		length = leftArray.Len()
	case rightIsArray:
		length = rightArray.Len()
	}
	if !typedNumericOperand(left) || !typedNumericOperand(right) {
		return nil, false, nil
	}
	if out, ok, err := qNumericUnaryDyadicSumRangeScalar(unaryOp, dyadicOp, left, right); ok || err != nil {
		return out, ok, err
	}
	if out, ok, err := qNumericUnaryDyadicSumTiledScalar(unaryOp, dyadicOp, left, right); ok || err != nil {
		return out, ok, err
	}
	return qNumericUnaryDyadicSum(unaryOp, dyadicOp, left, right, length)
}

func TryTypedCast(kind Kind, array Array) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("typed cast array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedCast(kind, a.array)
	}
	if !isIntegerArray(array) {
		return nil, false, nil
	}
	switch kind {
	case KindI16:
		out := make([]int16, array.Len())
		for row := 0; row < array.Len(); row++ {
			value, ok, err := integerArrayAt(array, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			if value < -32768 || value > 32767 {
				return nil, true, fmt.Errorf("value %d must be i16 for %s", row+1, kind)
			}
			out[row] = int16(value)
		}
		return columnArray[int16]{kind: KindI16, data: out}, true, nil
	case KindI32:
		out := make([]int32, array.Len())
		for row := 0; row < array.Len(); row++ {
			value, ok, err := integerArrayAt(array, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			if value < -2147483648 || value > 2147483647 {
				return nil, true, fmt.Errorf("value %d must be i32 for %s", row+1, kind)
			}
			out[row] = int32(value)
		}
		return columnArray[int32]{kind: KindI32, data: out}, true, nil
	case KindI64:
		out := make([]int64, array.Len())
		for row := 0; row < array.Len(); row++ {
			value, ok, err := integerArrayAt(array, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out[row] = value
		}
		return newI64Trusted(out), true, nil
	case KindF64:
		if values, ok := asI64RangeArray(array); ok {
			return f64RangeArray{start: float64(values.start), step: float64(values.step), len: values.len}, true, nil
		}
		out := make([]float64, array.Len())
		for row := 0; row < array.Len(); row++ {
			value, ok, err := integerArrayAt(array, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out[row] = float64(value)
		}
		return newF64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}

func aggregateIndexedNumericValue(agg aggregateInput, row int) (float64, bool, error) {
	if agg.leftColumn != nil && agg.rightColumn != nil {
		return aggregateBinaryNumericValue(agg, row)
	}
	return typedKernels.NumericAt(agg.column, row)
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

func (typedKernelRegistry) IntegerDyadic(op Op, left, right any) (any, bool, error) {
	if op == OpDiv {
		return nil, false, nil
	}
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray && !rightIsArray {
		return nil, false, nil
	}
	length := 0
	switch {
	case leftIsArray && rightIsArray:
		if leftArray.Len() != rightArray.Len() {
			return nil, true, fmt.Errorf("typed integer dyadic kernel length mismatch: %d != %d", leftArray.Len(), rightArray.Len())
		}
		length = leftArray.Len()
	case leftIsArray:
		length = leftArray.Len()
	case rightIsArray:
		length = rightArray.Len()
	}
	if !typedIntegerOperand(left) || !typedIntegerOperand(right) {
		return nil, false, nil
	}
	return numericIntegerDyadic(op, left, right, length)
}

func (typedKernelRegistry) NumericSum(array Array) (float64, int64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedKernels.NumericSum(a.array)
	case tiledArray:
		if sum, count, handled, err := numericSumTiled(a); handled || err != nil {
			return sum, count, handled, err
		}
		return 0, 0, false, nil
	case columnArray[int8]:
		return numericSumSlice(a.data)
	case columnArray[int16]:
		return numericSumSlice(a.data)
	case columnArray[int32]:
		return numericSumSlice(a.data)
	case columnArray[int64]:
		return numericSumSlice(a.data)
	case i64RangeArray:
		sum := i64RangeSum(a)
		return float64(sum), int64(a.len), true, nil
	case i64SparseAmendArray:
		sum, handled, err := i64SparseAmendSum(a)
		return float64(sum), int64(a.Len()), handled, err
	case i64FillArray:
		return float64(a.sum()), int64(a.Len()), true, nil
	case fbyI64BroadcastArray:
		return float64(a.total()), int64(a.len), true, nil
	case f64RangeArray:
		return f64RangeSum(a), int64(a.len), true, nil
	case f64FillArray:
		return a.sum(), int64(a.Len()), true, nil
	case fbyF64BroadcastArray:
		return a.total(), int64(a.len), true, nil
	case i64RunningSumArray:
		sum := i64RunningSumSum(a)
		return float64(sum), int64(a.Len()), true, nil
	case f64RunningSumArray:
		return f64RunningSumSum(a), int64(a.Len()), true, nil
	case i64SegmentArray:
		sum := i64SegmentSum(a)
		return float64(sum), int64(a.len), true, nil
	case i64ProductArray:
		sum := i64ProductSum(a)
		return float64(sum), int64(a.Len()), true, nil
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

func numericSumTiled(array tiledArray) (float64, int64, bool, error) {
	if array.len == 0 {
		return 0, 0, true, nil
	}
	sourceLen := array.source.Len()
	if sourceLen == 0 {
		return 0, 0, true, nil
	}
	if source, ok := array.source.(i64RangeArray); ok {
		return float64(tiledI64RangeSum(source, array.start, array.len)), int64(array.len), true, nil
	}
	if source, ok := array.source.(attributedArray); ok {
		return numericSumTiled(tiledArray{source: source.array, start: array.start, len: array.len})
	}
	if !isNumericArray(array.source) {
		return 0, 0, false, nil
	}
	fullCycles := array.len / sourceLen
	remainder := array.len % sourceLen
	periodSum, periodCount, ok, err := numericSumWindow(array.source, array.start, sourceLen)
	if err != nil || !ok {
		return 0, 0, ok, err
	}
	tailSum, tailCount, ok, err := numericSumWindow(array.source, array.start, remainder)
	if err != nil || !ok {
		return 0, 0, ok, err
	}
	return periodSum*float64(fullCycles) + tailSum, periodCount*int64(fullCycles) + tailCount, true, nil
}

func numericSumWindow(array Array, start, length int) (float64, int64, bool, error) {
	if length == 0 {
		return 0, 0, true, nil
	}
	sourceLen := array.Len()
	if sourceLen == 0 {
		return 0, 0, true, nil
	}
	var sum float64
	var count int64
	for offset := 0; offset < length; offset++ {
		row := start + offset
		if row >= sourceLen {
			row %= sourceLen
		}
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return 0, 0, true, err
		}
		if ok {
			sum += value
			count++
		}
	}
	return sum, count, true, nil
}

func tiledI64RangeSum(source i64RangeArray, start, length int) int64 {
	if length <= 0 || source.len <= 0 {
		return 0
	}
	sourceLen := source.len
	if start < 0 {
		start %= sourceLen
		if start < 0 {
			start += sourceLen
		}
	}
	start %= sourceLen
	fullCycles := length / sourceLen
	remainder := length % sourceLen
	sum := i64RangeSum(source) * int64(fullCycles)
	return sum + i64RangeCyclicWindowSum(source, start, remainder)
}

func i64RangeCyclicWindowSum(source i64RangeArray, start, length int) int64 {
	if length <= 0 {
		return 0
	}
	firstLen := length
	if firstLen > source.len-start {
		firstLen = source.len - start
	}
	first := i64RangeArray{
		start: source.start + int64(start)*source.step,
		step:  source.step,
		len:   firstLen,
	}
	sum := i64RangeSum(first)
	if firstLen == length {
		return sum
	}
	second := i64RangeArray{
		start: source.start,
		step:  source.step,
		len:   length - firstLen,
	}
	return sum + i64RangeSum(second)
}

// TryTypedNumericSum applies the shared typed numeric reduction kernel and
// returns the q-style scalar result: integer vectors keep an integer sum, while
// float or mixed nullable vectors produce a float sum.
func TryTypedNumericSum(array Array) (any, bool, error) {
	return typedKernels.NumericSumValue(array)
}

// TryTypedBinSum reduces q's `domain bin query` result directly. It preserves
// the scalar result shape of sum while avoiding the intermediate i64 bin vector.
func TryTypedBinSum(domain Array, query any) (any, bool, error) {
	if domain == nil {
		return nil, true, fmt.Errorf("bin sum domain must be non-nil")
	}
	if sum, handled, err := binSumTyped(domain, query); err != nil || handled {
		return sum, handled, err
	}
	if queryArray, ok := query.(Array); ok {
		var total int64
		for row := 0; row < queryArray.Len(); row++ {
			value, ok := queryArray.At(row)
			if !ok {
				return nil, true, fmt.Errorf("bin query row %d out of range", row)
			}
			index, err := kdbBinScalar(domain, value)
			if err != nil {
				return nil, true, err
			}
			total += index
		}
		return total, true, nil
	}
	index, err := kdbBinScalar(domain, query)
	if err != nil {
		return nil, true, err
	}
	return index, true, nil
}

// TryTypedNumericSumByI64Indexes reduces array rows selected by a typed i64
// index vector without materializing the gathered vector.
func TryTypedNumericSumByI64Indexes(array, indexes Array) (any, bool, error) {
	if array == nil || indexes == nil {
		return nil, true, fmt.Errorf("sum gather array and indexes must be non-nil")
	}
	if indexes.Kind() != KindI64 {
		return nil, true, fmt.Errorf("index vector kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if indexes.Len() == 0 {
		return NullValue, true, nil
	}
	if isDenseIntegerArray(array) {
		return typedIntegerSumByI64Indexes(array, indexes)
	}
	if !isNumericArray(array) {
		return nil, false, nil
	}
	var total float64
	if err := forEachTypedI64Index(indexes, array.Len(), func(row int) error {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return err
		}
		if ok {
			total += value
		}
		return nil
	}); err != nil {
		return nil, true, err
	}
	return total, true, nil
}

// TryTypedNumericSumWhereMask reduces rows whose typed boolean mask is true
// without materializing where indexes or a gathered value vector.
func TryTypedNumericSumWhereMask(array, mask Array) (any, bool, error) {
	if array == nil || mask == nil {
		return nil, true, fmt.Errorf("sum where array and mask must be non-nil")
	}
	if mask.Kind() != KindBool {
		return nil, true, fmt.Errorf("where mask kind is %s, want %s", mask.Kind(), KindBool)
	}
	if array.Len() != mask.Len() {
		return nil, true, fmt.Errorf("sum where length mismatch: values=%d mask=%d", array.Len(), mask.Len())
	}
	if isDenseIntegerArray(array) {
		return typedIntegerSumWhereMask(array, mask)
	}
	if !isNumericArray(array) {
		return nil, false, nil
	}
	var total float64
	selected := 0
	if err := forEachTypedBoolMask(mask, func(row int) error {
		selected++
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return err
		}
		if ok {
			total += value
		}
		return nil
	}); err != nil {
		return nil, true, err
	}
	if selected == 0 {
		return NullValue, true, nil
	}
	return total, true, nil
}

// TryTypedModuloCompareIndexStatsI64 computes count and q-index sum for
// where (array mod modulus) op target without materializing the modulo vector,
// boolean mask, or where index vector.
func TryTypedModuloCompareIndexStatsI64(array Array, modulus any, op Op, target any) (count, sum int64, handled bool, err error) {
	modulusI64, targetI64, ok := moduloCompareOperands(modulus, op, target)
	if !ok {
		return 0, 0, false, nil
	}
	if array == nil {
		return 0, 0, true, fmt.Errorf("modulo compare array is nil")
	}
	if plan, ok := i64ModuloComparePlanForArray(array, modulusI64, op, targetI64); ok {
		count, ok := plan.trueCount()
		if !ok {
			return 0, 0, false, nil
		}
		return count, plan.indexSum(), true, nil
	}
	if !isDenseIntegerArray(array) {
		return 0, 0, false, nil
	}
	for row := 0; row < array.Len(); row++ {
		selected, err := integerModuloCompareAt(array, row, modulusI64, op, targetI64)
		if err != nil {
			return 0, 0, true, err
		}
		if selected {
			count++
			sum += int64(row)
		}
	}
	return count, sum, true, nil
}

// TryTypedModuloCompareIndexesI64 returns q where indexes for modulo compare
// shapes without first building a modulo vector and compare mask.
func TryTypedModuloCompareIndexesI64(array Array, modulus any, op Op, target any) (Array, bool, error) {
	modulusI64, targetI64, ok := moduloCompareOperands(modulus, op, target)
	if !ok {
		return nil, false, nil
	}
	if array == nil {
		return nil, true, fmt.Errorf("modulo compare array is nil")
	}
	if plan, ok := i64ModuloComparePlanForArray(array, modulusI64, op, targetI64); ok {
		return i64ModuloComparePlanIndexArray(plan), true, nil
	}
	if !isDenseIntegerArray(array) {
		return nil, false, nil
	}
	indexes := make([]int64, 0, array.Len()/2)
	for row := 0; row < array.Len(); row++ {
		selected, err := integerModuloCompareAt(array, row, modulusI64, op, targetI64)
		if err != nil {
			return nil, true, err
		}
		if selected {
			indexes = append(indexes, int64(row))
		}
	}
	return newI64Trusted(indexes), true, nil
}

// TryTypedNumericSumWhereModuloCompare reduces values selected by a modulo
// compare over a second integer array in one pass.
func TryTypedNumericSumWhereModuloCompare(values, modSource Array, modulus any, op Op, target any) (any, bool, error) {
	modulusI64, targetI64, ok := moduloCompareOperands(modulus, op, target)
	if !ok {
		return nil, false, nil
	}
	if values == nil || modSource == nil {
		return nil, true, fmt.Errorf("modulo compare sum arrays must be non-nil")
	}
	if values.Len() != modSource.Len() {
		return nil, true, fmt.Errorf("modulo compare sum length mismatch: values=%d source=%d", values.Len(), modSource.Len())
	}
	if !isDenseIntegerArray(modSource) || !isNumericArray(values) {
		return nil, false, nil
	}
	if plan, ok := i64ModuloComparePlanForArray(modSource, modulusI64, op, targetI64); ok {
		indexes := i64ModuloComparePlanIndexArray(plan)
		return TryTypedNumericSumByI64Indexes(values, indexes)
	}
	if isDenseIntegerArray(values) {
		var total int64
		selected := 0
		for row := 0; row < modSource.Len(); row++ {
			match, err := integerModuloCompareAt(modSource, row, modulusI64, op, targetI64)
			if err != nil {
				return nil, true, err
			}
			if !match {
				continue
			}
			value, ok, err := integerArrayAt(values, row)
			if err != nil {
				return nil, true, err
			}
			if ok {
				total += value
			}
			selected++
		}
		if selected == 0 {
			return NullValue, true, nil
		}
		return total, true, nil
	}
	var total float64
	selected := 0
	for row := 0; row < modSource.Len(); row++ {
		match, err := integerModuloCompareAt(modSource, row, modulusI64, op, targetI64)
		if err != nil {
			return nil, true, err
		}
		if !match {
			continue
		}
		value, ok, err := typedKernels.NumericAt(values, row)
		if err != nil {
			return nil, true, err
		}
		if ok {
			total += value
		}
		selected++
	}
	if selected == 0 {
		return NullValue, true, nil
	}
	return total, true, nil
}

func moduloCompareOperands(modulus any, op Op, target any) (int64, int64, bool) {
	if op != OpEQ && op != OpNE {
		return 0, 0, false
	}
	modulusI64, ok := coerceInt64Exact(modulus)
	if !ok || modulusI64 == 0 {
		return 0, 0, false
	}
	targetI64, ok := coerceInt64Exact(target)
	if !ok {
		return 0, 0, false
	}
	return modulusI64, targetI64, true
}

func integerModuloCompareAt(array Array, row int, modulus int64, op Op, target int64) (bool, error) {
	value, ok, err := integerArrayAt(array, row)
	if err != nil {
		return false, err
	}
	if !ok {
		return op == OpNE, nil
	}
	mod := qModInt64(value, modulus)
	switch op {
	case OpEQ:
		return mod == target, nil
	case OpNE:
		return mod != target, nil
	default:
		return false, nil
	}
}

func typedIntegerSumByI64Indexes(array, indexes Array) (any, bool, error) {
	if sum, handled, err := typedIntegerSumByI64IndexView(array, indexes); handled || err != nil {
		return sum, handled, err
	}
	var total int64
	if err := forEachTypedI64Index(indexes, array.Len(), func(row int) error {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return err
		}
		if ok {
			total += value
		}
		return nil
	}); err != nil {
		return nil, true, err
	}
	return total, true, nil
}

func typedIntegerSumByI64IndexView(array, indexes Array) (any, bool, error) {
	switch idx := indexes.(type) {
	case attributedArray:
		return typedIntegerSumByI64IndexView(array, idx.array)
	case i64RangeArray:
		return typedIntegerSumRange(array, idx)
	case i64PeriodicIndexArray:
		return typedIntegerSumPeriodicIndex(array, idx)
	default:
		return nil, false, nil
	}
}

func typedIntegerSumContiguousRange(array Array, rows i64RangeArray) (any, bool, error) {
	if rows.step != 1 {
		return nil, false, nil
	}
	return typedIntegerSumRange(array, rows)
}

func typedIntegerSumRange(array Array, rows i64RangeArray) (any, bool, error) {
	if rows.len == 0 {
		return NullValue, true, nil
	}
	start, err := checkedI64Index(rows.start)
	if err != nil {
		return nil, true, err
	}
	last, err := checkedI64Index(rows.start + int64(rows.len-1)*rows.step)
	if err != nil {
		return nil, true, err
	}
	low, high := start, last
	if low > high {
		low, high = high, low
	}
	if low < 0 || high >= array.Len() {
		return nil, true, fmt.Errorf("index range %d..%d out of bounds for length %d", start, last, array.Len())
	}
	switch a := array.(type) {
	case attributedArray:
		return typedIntegerSumRange(a.array, rows)
	case i64RangeArray:
		return i64RangeSum(i64RangeArray{start: a.start + int64(start)*a.step, step: a.step * rows.step, len: rows.len}), true, nil
	case i64ScalarDyadicArray:
		if rows.step == 1 {
			return i64ScalarDyadicRangeSum(a, start, rows.len)
		}
		sourceSumValue, handled, err := typedIntegerSumRange(a.source, rows)
		if err != nil || !handled {
			return nil, handled, err
		}
		sourceSum, ok := sourceSumValue.(int64)
		if !ok {
			return nil, false, nil
		}
		return i64ScalarDyadicApplySum(a, sourceSum, rows.len)
	case tiledArray:
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

func typedIntegerSumPeriodicIndex(array Array, indexes i64PeriodicIndexArray) (any, bool, error) {
	if indexes.len == 0 {
		return NullValue, true, nil
	}
	switch a := array.(type) {
	case attributedArray:
		return typedIntegerSumPeriodicIndex(a.array, indexes)
	case i64RangeArray:
		sourceSum := i64PeriodicIndexSum(indexes)
		return a.start*int64(indexes.len) + a.step*sourceSum, true, nil
	case i64ScalarDyadicArray:
		sourceSumValue, handled, err := typedIntegerSumPeriodicIndex(a.source, indexes)
		if err != nil || !handled {
			return nil, handled, err
		}
		sourceSum, ok := sourceSumValue.(int64)
		if !ok {
			return nil, false, nil
		}
		return i64ScalarDyadicApplySum(a, sourceSum, indexes.len)
	default:
		return nil, false, nil
	}
}

func i64ScalarDyadicApplySum(array i64ScalarDyadicArray, sourceSum int64, length int) (any, bool, error) {
	n := int64(length)
	switch array.op {
	case OpAdd:
		if array.scalarLeft {
			return array.scalar*n + sourceSum, true, nil
		}
		return sourceSum + array.scalar*n, true, nil
	case OpSub:
		if array.scalarLeft {
			return array.scalar*n - sourceSum, true, nil
		}
		return sourceSum - array.scalar*n, true, nil
	case OpMul:
		return sourceSum * array.scalar, true, nil
	default:
		return nil, false, nil
	}
}

func typedIntegerSumWhereMask(array, mask Array) (any, bool, error) {
	if rows, ok := boolMaskContiguousTrueRows(mask); ok {
		if rows.len == 0 {
			return NullValue, true, nil
		}
		if sum, handled, err := typedIntegerSumContiguousRange(array, rows); err != nil || handled {
			return sum, handled, err
		}
	}
	var total int64
	count := 0
	if err := forEachTypedBoolMask(mask, func(row int) error {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return err
		}
		if ok {
			total += value
			count++
		}
		return nil
	}); err != nil {
		return nil, true, err
	}
	if count == 0 {
		return NullValue, true, nil
	}
	return total, true, nil
}

func boolMaskContiguousTrueRows(mask Array) (i64RangeArray, bool) {
	switch a := mask.(type) {
	case attributedArray:
		return boolMaskContiguousTrueRows(a.array)
	case i64RangeCompareMask:
		low, high, ok := compareMaskValueInterval(a)
		if !ok {
			return i64RangeArray{}, false
		}
		return i64RangeIntervalRows(a.values, low, high)
	case boolLogicalMask:
		if a.op != "and" || a.leftIsScalar || a.rightIsScalar {
			return i64RangeArray{}, false
		}
		left, leftOK := a.left.(i64RangeCompareMask)
		right, rightOK := a.right.(i64RangeCompareMask)
		if !leftOK || !rightOK || !sameI64Range(left.values, right.values) {
			return i64RangeArray{}, false
		}
		leftLow, leftHigh, ok := compareMaskValueInterval(left)
		if !ok {
			return i64RangeArray{}, false
		}
		rightLow, rightHigh, ok := compareMaskValueInterval(right)
		if !ok {
			return i64RangeArray{}, false
		}
		return i64RangeIntervalRows(left.values, maxInt64Value(leftLow, rightLow), minInt64Value(leftHigh, rightHigh))
	default:
		return i64RangeArray{}, false
	}
}

func i64RangeIntervalRows(values i64RangeArray, low, high int64) (i64RangeArray, bool) {
	if values.step != 1 {
		return i64RangeArray{}, false
	}
	if values.len <= 0 || low > high {
		return i64RangeArray{len: 0}, true
	}
	last := values.start + int64(values.len-1)
	startValue := maxInt64Value(values.start, low)
	endValue := minInt64Value(last, high)
	if startValue > endValue {
		return i64RangeArray{len: 0}, true
	}
	return i64RangeArray{start: startValue - values.start, step: 1, len: int(endValue - startValue + 1)}, true
}

func forEachTypedI64Index(indexes Array, limit int, fn func(row int) error) error {
	switch idx := indexes.(type) {
	case attributedArray:
		return forEachTypedI64Index(idx.array, limit, fn)
	case i64RangeArray:
		for i := 0; i < idx.len; i++ {
			row, err := checkedI64Index(idx.start + int64(i)*idx.step)
			if err != nil {
				return err
			}
			if row >= limit {
				return fmt.Errorf("index %d out of bounds for length %d", row, limit)
			}
			if err := fn(row); err != nil {
				return err
			}
		}
		return nil
	case i64SegmentArray:
		for _, segment := range idx.segments {
			if err := forEachTypedI64Index(segment, limit, fn); err != nil {
				return err
			}
		}
		return nil
	case columnArray[int64]:
		for _, value := range idx.data {
			row, err := checkedI64Index(value)
			if err != nil {
				return err
			}
			if row >= limit {
				return fmt.Errorf("index %d out of bounds for length %d", row, limit)
			}
			if err := fn(row); err != nil {
				return err
			}
		}
		return nil
	case nullableArray:
		for i, value := range idx.data {
			n64, ok := value.(int64)
			if !ok {
				return fmt.Errorf("index vector row %d is %T, want i64", i, value)
			}
			row, err := checkedI64Index(n64)
			if err != nil {
				return err
			}
			if row >= limit {
				return fmt.Errorf("index %d out of bounds for length %d", row, limit)
			}
			if err := fn(row); err != nil {
				return err
			}
		}
		return nil
	default:
		rows, handled, err := TryTypedI64Indexes(indexes)
		if err != nil {
			return err
		}
		if !handled {
			return fmt.Errorf("unsupported i64 index array %T", indexes)
		}
		for _, row := range rows {
			if row >= limit {
				return fmt.Errorf("index %d out of bounds for length %d", row, limit)
			}
			if err := fn(row); err != nil {
				return err
			}
		}
		return nil
	}
}

func forEachTypedBoolMask(mask Array, fn func(row int) error) error {
	switch m := mask.(type) {
	case attributedArray:
		return forEachTypedBoolMask(m.array, fn)
	case columnArray[bool]:
		for row, keep := range m.data {
			if keep {
				if err := fn(row); err != nil {
					return err
				}
			}
		}
		return nil
	case nullableArray:
		for row, value := range m.data {
			if IsNull(value) {
				continue
			}
			keep, ok := value.(bool)
			if !ok {
				return fmt.Errorf("where mask row %d is %T, want bool", row, value)
			}
			if keep {
				if err := fn(row); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		for row := 0; row < mask.Len(); row++ {
			value, ok := mask.At(row)
			if !ok {
				return fmt.Errorf("where mask row %d out of range", row)
			}
			if IsNull(value) {
				continue
			}
			keep, ok := value.(bool)
			if !ok {
				return fmt.Errorf("where mask row %d is %T, want bool", row, value)
			}
			if keep {
				if err := fn(row); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

type NumericStats struct {
	Sum      any
	Min      any
	Max      any
	Count    int64
	HasValue bool
}

// TryTypedNumericStats computes the common scalar reductions for one numeric
// array in a single typed pass. Count follows q count semantics and reports the
// vector length, while sum/min/max ignore null values.
func TryTypedNumericStats(array Array) (NumericStats, bool, error) {
	if array == nil {
		return NumericStats{}, true, fmt.Errorf("numeric stats array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedNumericStats(a.array)
	case i64RangeArray:
		return numericStatsI64Range(a), true, nil
	case f64RangeArray:
		return numericStatsF64Range(a), true, nil
	}
	if isDenseIntegerArray(array) {
		return numericStatsIntegerArray(array)
	}
	if !isNumericArray(array) {
		return NumericStats{}, false, nil
	}
	var sum float64
	var min float64
	var max float64
	hasValue := false
	for row := 0; row < array.Len(); row++ {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return NumericStats{}, true, err
		}
		if !ok {
			continue
		}
		sum += value
		if !hasValue || value < min {
			min = value
		}
		if !hasValue || value > max {
			max = value
		}
		hasValue = true
	}
	if !hasValue {
		return NumericStats{Count: int64(array.Len())}, true, nil
	}
	return NumericStats{
		Sum:      sum,
		Min:      min,
		Max:      max,
		Count:    int64(array.Len()),
		HasValue: true,
	}, true, nil
}

func numericStatsI64Range(array i64RangeArray) NumericStats {
	if array.len == 0 {
		return NumericStats{Count: 0}
	}
	first := array.start
	last := array.start + int64(array.len-1)*array.step
	min := first
	max := last
	if last < first {
		min = last
		max = first
	}
	return NumericStats{
		Sum:      i64RangeSum(array),
		Min:      min,
		Max:      max,
		Count:    int64(array.len),
		HasValue: true,
	}
}

func numericStatsF64Range(array f64RangeArray) NumericStats {
	if array.len == 0 {
		return NumericStats{Count: 0}
	}
	first := array.start
	last := array.start + float64(array.len-1)*array.step
	min := first
	max := last
	if last < first {
		min = last
		max = first
	}
	return NumericStats{
		Sum:      f64RangeSum(array),
		Min:      min,
		Max:      max,
		Count:    int64(array.len),
		HasValue: true,
	}
}

func numericStatsIntegerArray(array Array) (NumericStats, bool, error) {
	var sum int64
	var min int64
	var max int64
	hasValue := false
	for row := 0; row < array.Len(); row++ {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return NumericStats{}, true, err
		}
		if !ok {
			continue
		}
		sum += value
		if !hasValue || value < min {
			min = value
		}
		if !hasValue || value > max {
			max = value
		}
		hasValue = true
	}
	if !hasValue {
		return NumericStats{Count: int64(array.Len())}, true, nil
	}
	return NumericStats{
		Sum:      sum,
		Min:      min,
		Max:      max,
		Count:    int64(array.Len()),
		HasValue: true,
	}, true, nil
}

// TryTypedScalarFill applies q-style scalar fill to an array lazily. It keeps
// downstream reductions from materializing a dense replacement column.
func TryTypedScalarFill(fill any, array Array) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("fill array is nil")
	}
	if IsNull(fill) {
		return array, true, nil
	}
	if n, ok := coerceInt64Exact(fill); ok && isIntegerArray(array) {
		return i64FillArray{source: array, fill: n}, true, nil
	}
	if n, ok := numeric(fill); ok && isNumericArray(array) {
		return f64FillArray{source: array, fill: n}, true, nil
	}
	return nil, false, nil
}

// TryTypedSortIndexesI64 returns stable row indexes for typed arrays when the
// ordering can be described without materializing and sorting an index slice.
func TryTypedSortIndexesI64(array Array, descending bool) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("sort index array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedSortIndexesI64(a.array, descending)
	case i64RangeArray:
		if a.len == 0 {
			return NewI64Range(0, 1, 0), true, nil
		}
		if a.step == 0 {
			return NewI64Range(0, 1, a.len), true, nil
		}
		ascendingData := a.step > 0
		if descending == ascendingData {
			return NewI64Range(int64(a.len-1), -1, a.len), true, nil
		}
		return NewI64Range(0, 1, a.len), true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedFbySum broadcasts per-group sums back to the original row shape.
// It avoids group row slices, gather materialization, and boxed []any output.
func TryTypedFbySum(values, groups Array) (Array, bool, error) {
	if values == nil || groups == nil {
		return nil, true, fmt.Errorf("fby sum arrays must be non-nil")
	}
	if values.Len() != groups.Len() {
		return nil, true, fmt.Errorf("fby sum length mismatch: %d != %d", values.Len(), groups.Len())
	}
	return typedKernels.FbySum(values, groups)
}

func (typedKernelRegistry) FbySum(values, groups Array) (Array, bool, error) {
	switch v := values.(type) {
	case attributedArray:
		return typedKernels.FbySum(v.array, groups)
	case columnArray[int8]:
		return fbySumIntegral(v.data, values.Kind(), groups)
	case columnArray[int16]:
		return fbySumIntegral(v.data, values.Kind(), groups)
	case columnArray[int32]:
		return fbySumIntegral(v.data, values.Kind(), groups)
	case columnArray[int64]:
		return fbySumIntegral(v.data, values.Kind(), groups)
	case i64RangeArray:
		return fbySumI64Range(v, groups)
	case columnArray[uint8]:
		return fbySumIntegral(v.data, values.Kind(), groups)
	case columnArray[uint16]:
		return fbySumIntegral(v.data, values.Kind(), groups)
	case columnArray[uint32]:
		return fbySumIntegral(v.data, values.Kind(), groups)
	case columnArray[uint64]:
		return fbySumIntegral(v.data, values.Kind(), groups)
	case columnArray[float32]:
		return fbySumFloat(v.data, values.Kind(), groups)
	case columnArray[float64]:
		return fbySumFloat(v.data, values.Kind(), groups)
	case nullableArray:
		return fbySumNullable(v, groups)
	default:
		return nil, false, nil
	}
}

func TryTypedNumericAvg(array Array) (any, bool, error) {
	sum, count, handled, err := typedKernels.NumericSum(array)
	if err != nil || !handled {
		return nil, handled, err
	}
	if count == 0 {
		return NullValue, true, nil
	}
	return sum / float64(count), true, nil
}

func TryTypedNumericProduct(array Array) (any, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedNumericProduct(a.array)
	case tiledArray:
		if isIntegerArray(a) {
			if product, ok := numericProductTiledInteger(a); ok {
				return product, true, nil
			}
			return numericProductIntegerArray(a), true, nil
		}
		return nil, false, nil
	case columnArray[int8]:
		return numericProductIntegerValue(a.data), true, nil
	case columnArray[int16]:
		return numericProductIntegerValue(a.data), true, nil
	case columnArray[int32]:
		return numericProductIntegerValue(a.data), true, nil
	case columnArray[int64]:
		return numericProductIntegerValue(a.data), true, nil
	case i64RangeArray:
		return numericProductIntegerArray(a), true, nil
	case i64SegmentArray:
		return numericProductIntegerArray(a), true, nil
	case i64ProductArray:
		return numericProductIntegerArray(a), true, nil
	case columnArray[uint8]:
		return numericProductUnsignedValue(a.data), true, nil
	case columnArray[uint16]:
		return numericProductUnsignedValue(a.data), true, nil
	case columnArray[uint32]:
		return numericProductUnsignedValue(a.data), true, nil
	case columnArray[uint64]:
		return numericProductUnsignedValue(a.data), true, nil
	case columnArray[float32]:
		return numericProductFloatValue(a.data), true, nil
	case columnArray[float64]:
		return numericProductFloatValue(a.data), true, nil
	default:
		return nil, false, nil
	}
}

func TryTypedNumericProducts(array Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedNumericProducts(a.array)
	case tiledArray:
		if isIntegerArray(a) {
			if products, ok := numericProductsTiledInteger(a); ok {
				return products, true, nil
			}
			return numericProductsIntegerArray(a), true, nil
		}
		return nil, false, nil
	case columnArray[int8]:
		return numericProductsInteger(a.data), true, nil
	case columnArray[int16]:
		return numericProductsInteger(a.data), true, nil
	case columnArray[int32]:
		return numericProductsInteger(a.data), true, nil
	case columnArray[int64]:
		return numericProductsInteger(a.data), true, nil
	case i64RangeArray:
		return numericProductsIntegerArray(a), true, nil
	case i64SegmentArray:
		return numericProductsIntegerArray(a), true, nil
	case i64ProductArray:
		return numericProductsIntegerArray(a), true, nil
	case columnArray[uint8]:
		return numericProductsUnsigned(a.data), true, nil
	case columnArray[uint16]:
		return numericProductsUnsigned(a.data), true, nil
	case columnArray[uint32]:
		return numericProductsUnsigned(a.data), true, nil
	case columnArray[uint64]:
		return numericProductsUnsigned(a.data), true, nil
	case columnArray[float32]:
		return numericProductsFloat(a.data), true, nil
	case columnArray[float64]:
		return numericProductsFloat(a.data), true, nil
	default:
		return nil, false, nil
	}
}

func TryTypedNumericArrayLen(array Array) (int64, bool) {
	if !isNumericArray(array) {
		return 0, false
	}
	return int64(array.Len()), true
}

func (k typedKernelRegistry) NumericSumValue(array Array) (any, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return k.NumericSumValue(a.array)
	case tiledArray:
		if value, handled, err := numericSumTiledIntegerValue(a); handled || err != nil {
			return value, handled, err
		}
		return nil, false, nil
	case columnArray[int8]:
		return numericSumIntegerValue(a.data), true, nil
	case columnArray[int16]:
		return numericSumIntegerValue(a.data), true, nil
	case columnArray[int32]:
		return numericSumIntegerValue(a.data), true, nil
	case columnArray[int64]:
		return numericSumIntegerValue(a.data), true, nil
	case i64RangeArray:
		return i64RangeSum(a), true, nil
	case i64ScalarDyadicArray:
		return i64ScalarDyadicSum(a)
	case i64ScalarDyadicRunningSumArray:
		return i64ScalarDyadicRunningSumSum(a)
	case i64SparseAmendArray:
		return i64SparseAmendSum(a)
	case i64FillArray:
		return a.sum(), true, nil
	case fbyI64BroadcastArray:
		return a.total(), true, nil
	case f64RangeArray:
		return f64RangeSum(a), true, nil
	case f64FillArray:
		return a.sum(), true, nil
	case fbyF64BroadcastArray:
		return a.total(), true, nil
	case i64RunningSumArray:
		return i64RunningSumSum(a), true, nil
	case f64RunningSumArray:
		return f64RunningSumSum(a), true, nil
	case i64SegmentArray:
		return i64SegmentSum(a), true, nil
	case i64PeriodicIndexArray:
		return i64PeriodicIndexSum(a), true, nil
	case i64ProductArray:
		return i64ProductSum(a), true, nil
	case columnArray[uint8]:
		return numericSumUnsignedValue(a.data), true, nil
	case columnArray[uint16]:
		return numericSumUnsignedValue(a.data), true, nil
	case columnArray[uint32]:
		return numericSumUnsignedValue(a.data), true, nil
	case columnArray[uint64]:
		return numericSumUnsignedValue(a.data), true, nil
	case columnArray[float32]:
		return numericSumFloatValue(a.data), true, nil
	case columnArray[float64]:
		return numericSumFloatValue(a.data), true, nil
	case nullableArray:
		var sumI int64
		var sumF float64
		hasFloat := false
		for _, v := range a.data {
			if IsNull(v) {
				continue
			}
			if n, ok := coerceInt64Exact(v); ok {
				sumI += n
				sumF += float64(n)
				continue
			}
			n, ok := numeric(v)
			if !ok {
				return nil, true, fmt.Errorf("sum expects numeric values, got %T (%v)", v, v)
			}
			hasFloat = true
			sumF += n
		}
		if hasFloat {
			return sumF, true, nil
		}
		return sumI, true, nil
	default:
		return nil, false, nil
	}
}

func numericSumTiledIntegerValue(array tiledArray) (any, bool, error) {
	if array.len == 0 {
		return int64(0), true, nil
	}
	switch source := array.source.(type) {
	case attributedArray:
		return numericSumTiledIntegerValue(tiledArray{source: source.array, start: array.start, len: array.len})
	case i64RangeArray:
		return tiledI64RangeSum(source, array.start, array.len), true, nil
	}
	if !isIntegerArray(array.source) {
		return nil, false, nil
	}
	sourceLen := array.source.Len()
	if sourceLen == 0 {
		return int64(0), true, nil
	}
	fullCycles := array.len / sourceLen
	remainder := array.len % sourceLen
	period, ok, err := integerSumWindow(array.source, array.start, sourceLen)
	if err != nil || !ok {
		return nil, ok, err
	}
	tail, ok, err := integerSumWindow(array.source, array.start, remainder)
	if err != nil || !ok {
		return nil, ok, err
	}
	return period*int64(fullCycles) + tail, true, nil
}

func integerSumWindow(array Array, start, length int) (int64, bool, error) {
	if length == 0 {
		return 0, true, nil
	}
	sourceLen := array.Len()
	if sourceLen == 0 {
		return 0, true, nil
	}
	var sum int64
	for offset := 0; offset < length; offset++ {
		row := start + offset
		if row >= sourceLen {
			row %= sourceLen
		}
		value, ok, err := integerArrayAt(array, row)
		if err != nil || !ok {
			return 0, ok, err
		}
		sum += value
	}
	return sum, true, nil
}

// TryTypedNumericSums applies the shared typed numeric scan kernel for q sums
// and +\ without materializing []any intermediates.
func TryTypedNumericSums(array Array) (Array, bool, error) {
	return typedKernels.NumericSums(array)
}

// TryTypedMinMax applies the shared typed extrema kernel without routing every
// row through Array.At and interface comparisons.
func TryTypedMinMax(array Array, wantMax bool) (value any, handled bool, has bool, err error) {
	if wantMax {
		return typedKernels.Max(array)
	}
	return typedKernels.Min(array)
}

func TryTypedRunningMinMax(array Array, wantMax bool) (Array, bool, error) {
	if !isDenseIntegerArray(array) {
		return nil, false, nil
	}
	out := make([]int64, array.Len())
	var best int64
	hasBest := false
	for row := 0; row < array.Len(); row++ {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		if !hasBest || (wantMax && value > best) || (!wantMax && value < best) {
			best = value
			hasBest = true
		}
		out[row] = best
	}
	return newI64Trusted(out), true, nil
}

func TryTypedAvgs(array Array) (Array, bool, error) {
	if !isDenseIntegerArray(array) {
		return nil, false, nil
	}
	out := make([]float64, array.Len())
	var sum int64
	for row := 0; row < array.Len(); row++ {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		sum += value
		out[row] = float64(sum) / float64(row+1)
	}
	return newF64Trusted(out), true, nil
}

func TryTypedMCount(array Array, width int) (Array, bool, error) {
	if width <= 0 {
		return nil, true, fmt.Errorf("mcount width must be positive")
	}
	if !isDenseIntegerArray(array) {
		return nil, false, nil
	}
	out := make([]int64, array.Len())
	for row := 0; row < array.Len(); row++ {
		count := row + 1
		if count > width {
			count = width
		}
		out[row] = int64(count)
	}
	return newI64Trusted(out), true, nil
}

func TryTypedMCountSum(array Array, width int) (int64, bool, error) {
	if width <= 0 {
		return 0, true, fmt.Errorf("mcount width must be positive")
	}
	if !isDenseIntegerArray(array) {
		return 0, false, nil
	}
	n := array.Len()
	if n == 0 {
		return 0, true, nil
	}
	if width >= n {
		return int64(n*(n+1)) / 2, true, nil
	}
	return int64(width*(width+1)/2 + (n-width)*width), true, nil
}

func TryTypedMovingMinMax(array Array, width int, wantMax bool) (Array, bool, error) {
	if width <= 0 {
		return nil, true, fmt.Errorf("moving extrema width must be positive")
	}
	if !isDenseIntegerArray(array) {
		return nil, false, nil
	}
	out := make([]int64, array.Len())
	if array.Len() == 0 {
		return newI64Trusted(out), true, nil
	}
	dequeCap := width
	if dequeCap > array.Len() {
		dequeCap = array.Len()
	}
	indexes := make([]int, dequeCap)
	values := make([]int64, dequeCap)
	head, tail := 0, 0
	for row := 0; row < array.Len(); row++ {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		expiredBefore := row - width + 1
		for head < tail && indexes[head%dequeCap] < expiredBefore {
			head++
		}
		for head < tail {
			last := values[(tail-1)%dequeCap]
			if (wantMax && last > value) || (!wantMax && last < value) {
				break
			}
			tail--
		}
		indexes[tail%dequeCap] = row
		values[tail%dequeCap] = value
		tail++
		out[row] = values[head%dequeCap]
	}
	return newI64Trusted(out), true, nil
}

func TryTypedMovingMinMaxSum(array Array, width int, wantMax bool) (int64, bool, error) {
	if width <= 0 {
		return 0, true, fmt.Errorf("moving extrema width must be positive")
	}
	if !isDenseIntegerArray(array) {
		return 0, false, nil
	}
	if array.Len() == 0 {
		return 0, true, nil
	}
	if out, ok := movingMinMaxSumI64Range(array, width, wantMax); ok {
		return out, true, nil
	}
	dequeCap := width
	if dequeCap > array.Len() {
		dequeCap = array.Len()
	}
	indexes := make([]int, dequeCap)
	values := make([]int64, dequeCap)
	head, tail := 0, 0
	var total int64
	for row := 0; row < array.Len(); row++ {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return 0, true, err
		}
		if !ok {
			return 0, false, nil
		}
		expiredBefore := row - width + 1
		for head < tail && indexes[head%dequeCap] < expiredBefore {
			head++
		}
		for head < tail {
			last := values[(tail-1)%dequeCap]
			if (wantMax && last > value) || (!wantMax && last < value) {
				break
			}
			tail--
		}
		indexes[tail%dequeCap] = row
		values[tail%dequeCap] = value
		tail++
		total += values[head%dequeCap]
	}
	return total, true, nil
}

func movingMinMaxSumI64Range(array Array, width int, wantMax bool) (int64, bool) {
	values, ok := asI64RangeArray(array)
	if !ok || !i64RangeIsMonotonic(values) {
		return 0, false
	}
	if values.step == 0 {
		return values.start * int64(values.len), true
	}
	useWindowStart := (values.step > 0 && !wantMax) || (values.step < 0 && wantMax)
	if !useWindowStart {
		return i64RangeSum(values), true
	}
	return movingWindowStartSumI64Range(values, width), true
}

func movingWindowStartSumI64Range(values i64RangeArray, width int) int64 {
	if values.len == 0 {
		return 0
	}
	if width >= values.len {
		return values.start * int64(values.len)
	}
	repeatFirst := width
	if repeatFirst > values.len {
		repeatFirst = values.len
	}
	total := values.start * int64(repeatFirst)
	tailLen := values.len - width
	if tailLen <= 0 {
		return total
	}
	tail := i64RangeArray{
		start: values.start + values.step,
		step:  values.step,
		len:   tailLen,
	}
	return total + i64RangeSum(tail)
}

func TryTypedMovingNumericSumSum(array Array, width int, average bool) (any, bool, error) {
	if width <= 0 {
		return nil, true, fmt.Errorf("moving numeric width must be positive")
	}
	if !isNumericArray(array) {
		return nil, false, nil
	}
	if isDenseIntegerArray(array) {
		var window int64
		var totalI int64
		var totalF float64
		for row := 0; row < array.Len(); row++ {
			value, ok, err := integerArrayAt(array, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			window += value
			if row >= width {
				expired, ok, err := integerArrayAt(array, row-width)
				if err != nil {
					return nil, true, err
				}
				if !ok {
					return nil, false, nil
				}
				window -= expired
			}
			if average {
				count := row + 1
				if count > width {
					count = width
				}
				totalF += float64(window) / float64(count)
			} else {
				totalI += window
			}
		}
		if average {
			return totalF, true, nil
		}
		return totalI, true, nil
	}
	var window float64
	var total float64
	for row := 0; row < array.Len(); row++ {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		window += value
		if row >= width {
			expired, ok, err := typedKernels.NumericAt(array, row-width)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			window -= expired
		}
		if average {
			count := row + 1
			if count > width {
				count = width
			}
			total += window / float64(count)
		} else {
			total += window
		}
	}
	return total, true, nil
}

// TryTypedDeltas applies q-style deltas for dense typed numeric arrays.
func TryTypedDeltas(array Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedDeltas(a.array)
	case columnArray[int64]:
		return deltasI64Slice(a.data), true, nil
	case i64RangeArray:
		return deltasI64Range(a), true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedDeltasSum applies sum deltas x without materializing deltas. The
// telescoping reduction is valid for dense, null-free numeric arrays; nullable
// and generic arrays deliberately fall back to the regular deltas + sum path.
func TryTypedDeltasSum(array Array) (any, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedDeltasSum(a.array)
	case columnArray[int8]:
		return lastSignedDeltasSum(a.data), true, nil
	case columnArray[int16]:
		return lastSignedDeltasSum(a.data), true, nil
	case columnArray[int32]:
		return lastSignedDeltasSum(a.data), true, nil
	case columnArray[int64]:
		return lastSignedDeltasSum(a.data), true, nil
	case i64RangeArray:
		if a.len == 0 {
			return NullValue, true, nil
		}
		return a.start + int64(a.len-1)*a.step, true, nil
	case columnArray[uint8]:
		return lastUnsignedDeltasSum(a.data), true, nil
	case columnArray[uint16]:
		return lastUnsignedDeltasSum(a.data), true, nil
	case columnArray[uint32]:
		return lastUnsignedDeltasSum(a.data), true, nil
	case columnArray[uint64]:
		return lastUnsignedDeltasSum(a.data), true, nil
	case columnArray[float32]:
		return lastFloatDeltasSum(a.data), true, nil
	case columnArray[float64]:
		return lastFloatDeltasSum(a.data), true, nil
	case f64RangeArray:
		if a.len == 0 {
			return NullValue, true, nil
		}
		return a.start + float64(a.len-1)*a.step, true, nil
	default:
		return nil, false, nil
	}
}

func (k typedKernelRegistry) NumericSums(array Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return k.NumericSums(a.array)
	case columnArray[int8]:
		return numericSumsInteger(a.data), true, nil
	case columnArray[int16]:
		return numericSumsInteger(a.data), true, nil
	case columnArray[int32]:
		return numericSumsInteger(a.data), true, nil
	case columnArray[int64]:
		return numericSumsInteger(a.data), true, nil
	case i64RangeArray:
		return i64RunningSumArray{source: a}, true, nil
	case i64ScalarDyadicArray:
		return i64ScalarDyadicRunningSumArray{source: a}, true, nil
	case f64RangeArray:
		return f64RunningSumArray{source: a}, true, nil
	case columnArray[uint8]:
		return numericSumsUnsigned(a.data), true, nil
	case columnArray[uint16]:
		return numericSumsUnsigned(a.data), true, nil
	case columnArray[uint32]:
		return numericSumsUnsigned(a.data), true, nil
	case columnArray[uint64]:
		return numericSumsUnsigned(a.data), true, nil
	case columnArray[float32]:
		return numericSumsFloat(a.data), true, nil
	case columnArray[float64]:
		return numericSumsFloat(a.data), true, nil
	case nullableArray:
		outI := make([]int64, len(a.data))
		var outF []float64
		var sumI int64
		var sumF float64
		hasFloat := false
		for i, v := range a.data {
			if IsNull(v) {
				if hasFloat {
					outF[i] = sumF
				} else {
					outI[i] = sumI
				}
				continue
			}
			if n, ok := coerceInt64Exact(v); ok {
				sumI += n
				sumF += float64(n)
				if hasFloat {
					outF[i] = sumF
				} else {
					outI[i] = sumI
				}
				continue
			}
			n, ok := numeric(v)
			if !ok {
				return nil, true, fmt.Errorf("sums expects numeric values, got %T (%v)", v, v)
			}
			if !hasFloat {
				outF = make([]float64, len(a.data))
				for j := 0; j < i; j++ {
					outF[j] = float64(outI[j])
				}
				hasFloat = true
			}
			sumF += n
			outF[i] = sumF
		}
		if hasFloat {
			return newF64Trusted(outF), true, nil
		}
		return newI64Trusted(outI), true, nil
	default:
		return nil, false, nil
	}
}

type i64FillArray struct {
	source Array
	fill   int64
}

func (a i64FillArray) Kind() Kind { return KindI64 }

func (a i64FillArray) Len() int { return a.source.Len() }

func (a i64FillArray) At(row int) (any, bool) {
	value, ok, err := a.valueAt(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64FillArray) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data fill row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64FillArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data fill row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64FillArray) valueAt(row int) (int64, bool, error) {
	value, ok := a.source.At(row)
	if !ok {
		return 0, false, nil
	}
	if IsNull(value) {
		return a.fill, true, nil
	}
	n, ok := coerceInt64Exact(value)
	if !ok {
		return 0, false, fmt.Errorf("fill row %d is %T, want integer", row, value)
	}
	return n, true, nil
}

func (a i64FillArray) sum() int64 {
	if total, ok := i64FilledSum(a.source, a.fill); ok {
		return total
	}
	var total int64
	for row := 0; row < a.Len(); row++ {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			return 0
		}
		total += value
	}
	return total
}

func i64FilledSum(array Array, fill int64) (int64, bool) {
	switch a := array.(type) {
	case attributedArray:
		return i64FilledSum(a.array, fill)
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return 0, true
		}
		sourceTotal, ok := i64FilledSum(a.source, fill)
		if !ok {
			return 0, false
		}
		fullCycles := a.len / sourceLen
		remainder := a.len % sourceLen
		total := sourceTotal * int64(fullCycles)
		for row := 0; row < remainder; row++ {
			value, ok := i64FilledValueAt(a.source, (a.start+row)%sourceLen, fill)
			if !ok {
				return 0, false
			}
			total += value
		}
		return total, true
	case nullableArray:
		var total int64
		for _, value := range a.data {
			if IsNull(value) {
				total += fill
				continue
			}
			n, ok := coerceInt64Exact(value)
			if !ok {
				return 0, false
			}
			total += n
		}
		return total, true
	case columnArray[int8]:
		return numericSumIntegerValue(a.data), true
	case columnArray[int16]:
		return numericSumIntegerValue(a.data), true
	case columnArray[int32]:
		return numericSumIntegerValue(a.data), true
	case columnArray[int64]:
		return numericSumIntegerValue(a.data), true
	case i64RangeArray:
		return i64RangeSum(a), true
	default:
		return 0, false
	}
}

func i64FilledValueAt(array Array, row int, fill int64) (int64, bool) {
	value, ok := array.At(row)
	if !ok {
		return 0, false
	}
	if IsNull(value) {
		return fill, true
	}
	n, ok := coerceInt64Exact(value)
	return n, ok
}

type f64FillArray struct {
	source Array
	fill   float64
}

func (a f64FillArray) Kind() Kind { return KindF64 }

func (a f64FillArray) Len() int { return a.source.Len() }

func (a f64FillArray) At(row int) (any, bool) {
	value, ok, err := a.valueAt(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a f64FillArray) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data fill row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a f64FillArray) Gather(indexes []int) Array {
	out := make([]float64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data fill row %d out of range", row))
		}
		out[i] = value
	}
	return newF64Trusted(out)
}

func (a f64FillArray) valueAt(row int) (float64, bool, error) {
	value, ok := a.source.At(row)
	if !ok {
		return 0, false, nil
	}
	if IsNull(value) {
		return a.fill, true, nil
	}
	n, ok := numeric(value)
	if !ok {
		return 0, false, fmt.Errorf("fill row %d is %T, want numeric", row, value)
	}
	return n, true, nil
}

func (a f64FillArray) sum() float64 {
	var total float64
	for row := 0; row < a.Len(); row++ {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			return 0
		}
		total += value
	}
	return total
}

type fbyI64BroadcastArray struct {
	rowGroups []int
	sums      []int64
	len       int
}

func (a fbyI64BroadcastArray) Kind() Kind { return KindI64 }

func (a fbyI64BroadcastArray) Len() int { return a.len }

func (a fbyI64BroadcastArray) At(row int) (any, bool) {
	if row < 0 || row >= a.len {
		return nil, false
	}
	return a.sums[a.rowGroups[row]], true
}

func (a fbyI64BroadcastArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		out[row] = a.sums[a.rowGroups[row]]
	}
	return out
}

func (a fbyI64BroadcastArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data fby gather index %d out of range", row))
		}
		out[i] = a.sums[a.rowGroups[row]]
	}
	return newI64Trusted(out)
}

func (a fbyI64BroadcastArray) total() int64 {
	counts := make([]int64, len(a.sums))
	for _, group := range a.rowGroups {
		counts[group]++
	}
	var total int64
	for group, sum := range a.sums {
		total += sum * counts[group]
	}
	return total
}

type fbyF64BroadcastArray struct {
	rowGroups []int
	sums      []float64
	len       int
}

func (a fbyF64BroadcastArray) Kind() Kind { return KindF64 }

func (a fbyF64BroadcastArray) Len() int { return a.len }

func (a fbyF64BroadcastArray) At(row int) (any, bool) {
	if row < 0 || row >= a.len {
		return nil, false
	}
	return a.sums[a.rowGroups[row]], true
}

func (a fbyF64BroadcastArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		out[row] = a.sums[a.rowGroups[row]]
	}
	return out
}

func (a fbyF64BroadcastArray) Gather(indexes []int) Array {
	out := make([]float64, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data fby gather index %d out of range", row))
		}
		out[i] = a.sums[a.rowGroups[row]]
	}
	return newF64Trusted(out)
}

func (a fbyF64BroadcastArray) total() float64 {
	counts := make([]int64, len(a.sums))
	for _, group := range a.rowGroups {
		counts[group]++
	}
	var total float64
	for group, sum := range a.sums {
		total += sum * float64(counts[group])
	}
	return total
}

func fbySumIntegral[T signedScalar | unsignedScalar](values []T, valueKind Kind, groups Array) (Array, bool, error) {
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	sums := make([]int64, groupCount)
	for row, value := range values {
		sums[rowGroups[row]] += int64(value)
	}
	_ = valueKind
	return fbyI64BroadcastArray{rowGroups: rowGroups, sums: sums, len: len(values)}, true, nil
}

func fbySumI64Range(values i64RangeArray, groups Array) (Array, bool, error) {
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	sums := make([]int64, groupCount)
	for row, group := range rowGroups {
		sums[group] += values.start + int64(row)*values.step
	}
	return fbyI64BroadcastArray{rowGroups: rowGroups, sums: sums, len: values.len}, true, nil
}

func fbySumFloat[T floatScalar](values []T, valueKind Kind, groups Array) (Array, bool, error) {
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	sums := make([]float64, groupCount)
	for row, value := range values {
		sums[rowGroups[row]] += float64(value)
	}
	_ = valueKind
	return fbyF64BroadcastArray{rowGroups: rowGroups, sums: sums, len: len(values)}, true, nil
}

func fbySumNullable(values nullableArray, groups Array) (Array, bool, error) {
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	sumsI := make([]int64, groupCount)
	sumsF := make([]float64, groupCount)
	counts := make([]int64, groupCount)
	hasFloat := false
	for row, value := range values.data {
		if IsNull(value) {
			continue
		}
		group := rowGroups[row]
		if n, ok := coerceInt64Exact(value); ok {
			sumsI[group] += n
			sumsF[group] += float64(n)
			counts[group]++
			continue
		}
		n, ok := numeric(value)
		if !ok {
			return nil, false, nil
		}
		hasFloat = true
		sumsF[group] += n
		counts[group]++
	}
	out := make([]any, len(values.data))
	for row, group := range rowGroups {
		if counts[group] == 0 {
			out[row] = NullValue
		} else if hasFloat {
			out[row] = sumsF[group]
		} else {
			out[row] = sumsI[group]
		}
	}
	return InferArray(out), true, nil
}

func fbyGroupIDs(groups Array) ([]int, int, error) {
	switch g := groups.(type) {
	case attributedArray:
		return fbyGroupIDs(g.array)
	case tiledArray:
		sourceLen := g.source.Len()
		if sourceLen == 0 || g.len == 0 {
			return make([]int, g.len), 0, nil
		}
		sourceGroups, groupCount, err := fbyGroupIDs(g.source)
		if err != nil {
			return nil, 0, err
		}
		rowGroups := make([]int, g.len)
		for row := range rowGroups {
			rowGroups[row] = sourceGroups[(g.start+row)%sourceLen]
		}
		return rowGroups, groupCount, nil
	case columnArray[bool]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[int8]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[int16]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[int32]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[int64]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[uint8]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[uint16]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[uint32]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[uint64]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[string]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[Symbol]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[Month]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[Date]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[DateTime]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[Timespan]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[Minute]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[Second]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[Time]:
		return fbyGroupIDsComparable(g.data)
	case columnArray[Timestamp]:
		return fbyGroupIDsComparable(g.data)
	}
	rowGroups := make([]int, groups.Len())
	groupIDs := make(map[string]int)
	for row := 0; row < groups.Len(); row++ {
		value, ok := groups.At(row)
		if !ok {
			return nil, 0, fmt.Errorf("fby group row %d out of range", row)
		}
		key := arrayValueKey(groups.Kind(), value)
		id, ok := groupIDs[key]
		if !ok {
			id = len(groupIDs)
			groupIDs[key] = id
		}
		rowGroups[row] = id
	}
	return rowGroups, len(groupIDs), nil
}

func fbyGroupIDsComparable[T comparable](values []T) ([]int, int, error) {
	rowGroups := make([]int, len(values))
	groupIDs := make(map[T]int)
	for row, value := range values {
		id, ok := groupIDs[value]
		if !ok {
			id = len(groupIDs)
			groupIDs[value] = id
		}
		rowGroups[row] = id
	}
	return rowGroups, len(groupIDs), nil
}

func (typedKernelRegistry) NumericSumRows(array Array, rows []int) (float64, int64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedKernels.NumericSumRows(a.array, rows)
	case columnArray[int8]:
		return numericSumRowsSlice(a.data, rows)
	case columnArray[int16]:
		return numericSumRowsSlice(a.data, rows)
	case columnArray[int32]:
		return numericSumRowsSlice(a.data, rows)
	case columnArray[int64]:
		return numericSumRowsSlice(a.data, rows)
	case i64RangeArray:
		return numericSumRowsI64Range(a, rows)
	case columnArray[uint8]:
		return numericSumRowsSlice(a.data, rows)
	case columnArray[uint16]:
		return numericSumRowsSlice(a.data, rows)
	case columnArray[uint32]:
		return numericSumRowsSlice(a.data, rows)
	case columnArray[uint64]:
		return numericSumRowsSlice(a.data, rows)
	case columnArray[float32]:
		return numericSumRowsSlice(a.data, rows)
	case columnArray[float64]:
		return numericSumRowsSlice(a.data, rows)
	case nullableArray:
		var sum float64
		var count int64
		for _, row := range rows {
			if row < 0 || row >= len(a.data) {
				return 0, 0, true, fmt.Errorf("sum row %d out of range", row)
			}
			v := a.data[row]
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

func (typedKernelRegistry) NumericBinarySumRows(left Array, op Op, right Array, rows []int) (float64, int64, bool, error) {
	if !isNumericBinaryAggregateOp(op) {
		return 0, 0, false, nil
	}
	var sum float64
	var count int64
	for _, row := range rows {
		leftValue, leftOK, err := typedKernels.NumericAt(left, row)
		if err != nil {
			return 0, 0, true, err
		}
		if !leftOK {
			continue
		}
		rightValue, rightOK, err := typedKernels.NumericAt(right, row)
		if err != nil {
			return 0, 0, true, err
		}
		if !rightOK {
			continue
		}
		switch op {
		case OpAdd:
			sum += leftValue + rightValue
		case OpSub:
			sum += leftValue - rightValue
		case OpMul:
			sum += leftValue * rightValue
		case OpDiv:
			if rightValue == 0 {
				continue
			}
			sum += leftValue / rightValue
		default:
			return 0, 0, false, nil
		}
		count++
	}
	return sum, count, true, nil
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
			if index, ok := arrayIndexForBorrowed(column, ArrayAttributeUnique); ok {
				return cloneRowsByKey(index.RowsByKey), nil
			}
			if index, ok := arrayIndexForBorrowed(column, ArrayAttributeGrouped); ok {
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
	if leftIndexes, rightIndexes, ok, err := k.singleColumnTypedJoinIndexes(left, right, keepUnmatchedLeft, keys); err != nil || ok {
		return leftIndexes, rightIndexes, err
	}
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

func (typedKernelRegistry) singleColumnTypedJoinIndexes(left, right Frame, keepUnmatchedLeft bool, keys []JoinKey) ([]int, []int, bool, error) {
	if len(keys) != 1 {
		return nil, nil, false, nil
	}
	leftColumn, ok := left.Column(keys[0].Left)
	if !ok {
		return nil, nil, true, fmt.Errorf("join left key column %q does not exist", keys[0].Left)
	}
	rightColumn, ok := right.Column(keys[0].Right)
	if !ok {
		return nil, nil, true, fmt.Errorf("join right key column %q does not exist", keys[0].Right)
	}
	if leftColumn.Kind() != rightColumn.Kind() {
		return nil, nil, false, nil
	}
	return singleColumnTypedJoinIndexes(leftColumn, rightColumn, keepUnmatchedLeft)
}

func singleColumnTypedJoinIndexes(left, right Array, keepUnmatchedLeft bool) ([]int, []int, bool, error) {
	switch l := left.(type) {
	case attributedArray:
		return singleColumnTypedJoinIndexes(l.array, right, keepUnmatchedLeft)
	case columnArray[bool]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[int8]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[int16]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[int32]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[int64]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[uint8]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[uint16]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[uint32]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[uint64]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[float32]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[float64]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[string]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[Symbol]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[Month]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[Date]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[DateTime]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[Timespan]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[Minute]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[Second]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[Time]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	case columnArray[Timestamp]:
		return singleColumnTypedJoinIndexesFor(l, right, keepUnmatchedLeft)
	default:
		return nil, nil, false, nil
	}
}

func singleColumnTypedJoinIndexesFor[T comparable](left columnArray[T], right Array, keepUnmatchedLeft bool) ([]int, []int, bool, error) {
	indexRowsByKey, hasIndexRowsByKey := typedRowsByKeyForArray[T](right)
	right = unwrapAttributedArray(right)
	r, ok := right.(columnArray[T])
	if !ok {
		return nil, nil, false, nil
	}
	if leftIndexes, rightIndexes, ok := alignedTypedJoinIndexes(left.data, r.data); ok {
		return leftIndexes, rightIndexes, true, nil
	}
	rowsByKey := indexRowsByKey
	if !hasIndexRowsByKey {
		rowsByKey = make(map[T][]int, len(r.data))
		for row, value := range r.data {
			rowsByKey[value] = append(rowsByKey[value], row)
		}
	}
	leftIndexes := make([]int, 0, len(left.data))
	rightIndexes := make([]int, 0, len(left.data))
	for row, value := range left.data {
		matches := rowsByKey[value]
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
	return leftIndexes, rightIndexes, true, nil
}

func alignedTypedJoinIndexes[T comparable](left, right []T) ([]int, []int, bool) {
	if len(left) != len(right) {
		return nil, nil, false
	}
	seen := make(map[T]struct{}, len(left))
	for row, value := range left {
		if right[row] != value {
			return nil, nil, false
		}
		if _, ok := seen[value]; ok {
			return nil, nil, false
		}
		seen[value] = struct{}{}
	}
	return allIndexes(len(left)), allIndexes(len(right)), true
}

func unwrapAttributedArray(array Array) Array {
	for {
		attributed, ok := array.(attributedArray)
		if !ok {
			return array
		}
		array = attributed.array
	}
}

func (typedKernelRegistry) Bin(domain Array, query any) (any, bool, error) {
	if domain == nil {
		return nil, false, fmt.Errorf("bin domain is nil")
	}
	if out, ok, err := binTyped(domain, query); ok || err != nil {
		return out, ok, err
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

func binTyped(domain Array, query any) (any, bool, error) {
	switch d := domain.(type) {
	case attributedArray:
		return binTyped(d.array, query)
	case columnArray[int8]:
		return binSigned[int8](d.data, query)
	case columnArray[int16]:
		return binSigned[int16](d.data, query)
	case columnArray[int32]:
		return binSigned[int32](d.data, query)
	case columnArray[int64]:
		if queryArray, ok := query.(i64RangeArray); ok {
			return binI64Range(d.data, queryArray), true, nil
		}
		return binI64(d.data, query)
	case i64RangeArray:
		return binI64RangeDomain(d, query)
	case columnArray[uint8]:
		return binUnsigned[uint8](d.data, query)
	case columnArray[uint16]:
		return binUnsigned[uint16](d.data, query)
	case columnArray[uint32]:
		return binUnsigned[uint32](d.data, query)
	case columnArray[uint64]:
		return binUnsigned[uint64](d.data, query)
	case columnArray[float32]:
		return binFloat[float32](d.data, query)
	case columnArray[float64]:
		return binF64(d.data, query)
	case columnArray[string]:
		return binString(d.data, query)
	case columnArray[Symbol]:
		return binSymbol(d.data, query)
	case columnArray[Month]:
		return binSigned[Month](d.data, query)
	case columnArray[Date]:
		return binSigned[Date](d.data, query)
	case columnArray[DateTime]:
		return binSigned[DateTime](d.data, query)
	case columnArray[Timespan]:
		return binSigned[Timespan](d.data, query)
	case columnArray[Minute]:
		return binSigned[Minute](d.data, query)
	case columnArray[Second]:
		return binSigned[Second](d.data, query)
	case columnArray[Time]:
		return binSigned[Time](d.data, query)
	case columnArray[Timestamp]:
		return binSigned[Timestamp](d.data, query)
	default:
		return nil, false, nil
	}
}

func binSumTyped(domain Array, query any) (int64, bool, error) {
	switch d := domain.(type) {
	case attributedArray:
		return binSumTyped(d.array, query)
	case columnArray[int8]:
		return binSignedSum[int8](d.data, query)
	case columnArray[int16]:
		return binSignedSum[int16](d.data, query)
	case columnArray[int32]:
		return binSignedSum[int32](d.data, query)
	case columnArray[int64]:
		return binI64Sum(d.data, query)
	case i64RangeArray:
		return binI64RangeDomainSum(d, query)
	case i64ScalarDyadicArray:
		if domain, ok := i64ScalarDyadicAffineRange(d); ok {
			return binI64RangeDomainSum(domain, query)
		}
		return 0, false, nil
	case columnArray[uint8]:
		return binUnsignedSum[uint8](d.data, query)
	case columnArray[uint16]:
		return binUnsignedSum[uint16](d.data, query)
	case columnArray[uint32]:
		return binUnsignedSum[uint32](d.data, query)
	case columnArray[uint64]:
		return binUnsignedSum[uint64](d.data, query)
	case columnArray[float32]:
		return binFloatSum[float32](d.data, query)
	case columnArray[float64]:
		return binF64Sum(d.data, query)
	case columnArray[string]:
		return binStringSum(d.data, query)
	case columnArray[Symbol]:
		return binSymbolSum(d.data, query)
	case columnArray[Month]:
		return binSignedSum[Month](d.data, query)
	case columnArray[Date]:
		return binSignedSum[Date](d.data, query)
	case columnArray[DateTime]:
		return binSignedSum[DateTime](d.data, query)
	case columnArray[Timespan]:
		return binSignedSum[Timespan](d.data, query)
	case columnArray[Minute]:
		return binSignedSum[Minute](d.data, query)
	case columnArray[Second]:
		return binSignedSum[Second](d.data, query)
	case columnArray[Time]:
		return binSignedSum[Time](d.data, query)
	case columnArray[Timestamp]:
		return binSignedSum[Timestamp](d.data, query)
	default:
		return 0, false, nil
	}
}

func binSigned[T signedScalar](domain []T, query any) (any, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := signedArrayData[T](queryArray)
		if !ok {
			return nil, false, nil
		}
		return NewI64(binSignedSlice(domain, values)), true, nil
	}
	value, ok := query.(T)
	if !ok {
		return nil, false, nil
	}
	if len(domain) == 0 {
		return int64(-1), true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binSignedSum[T signedScalar](domain []T, query any) (int64, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := signedArrayData[T](queryArray)
		if !ok {
			return 0, false, nil
		}
		return binSignedSliceSum(domain, values), true, nil
	}
	value, ok := query.(T)
	if !ok {
		return 0, false, nil
	}
	if len(domain) == 0 {
		return -1, true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binI64(domain []int64, query any) (any, bool, error) {
	if queryArray, ok := query.(i64RangeArray); ok {
		return binI64Range(domain, queryArray), true, nil
	}
	if queryArray, ok := query.(Array); ok {
		values, ok := signedArrayData[int64](queryArray)
		if !ok {
			return nil, false, nil
		}
		return NewI64(binSignedSlice(domain, values)), true, nil
	}
	value, ok := coerceInt64Exact(query)
	if !ok {
		return nil, false, nil
	}
	if len(domain) == 0 {
		return int64(-1), true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binI64Sum(domain []int64, query any) (int64, bool, error) {
	if queryArray, ok := query.(i64RangeArray); ok {
		return binI64RangeSum(domain, queryArray), true, nil
	}
	if queryArray, ok := query.(Array); ok {
		values, ok := signedArrayData[int64](queryArray)
		if !ok {
			return 0, false, nil
		}
		return binSignedSliceSum(domain, values), true, nil
	}
	value, ok := coerceInt64Exact(query)
	if !ok {
		return 0, false, nil
	}
	if len(domain) == 0 {
		return -1, true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binI64Range(domain []int64, query i64RangeArray) Array {
	out := make([]int64, query.Len())
	for i := range out {
		value := query.start + int64(i)*query.step
		out[i] = int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
	}
	return NewI64(out)
}

func binI64RangeSum(domain []int64, query i64RangeArray) int64 {
	var total int64
	for i := 0; i < query.Len(); i++ {
		value := query.start + int64(i)*query.step
		total += int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
	}
	return total
}

func binI64RangeDomain(domain i64RangeArray, query any) (any, bool, error) {
	if domain.step <= 0 {
		return nil, false, nil
	}
	if queryArray, ok := query.(i64RangeArray); ok {
		out := make([]int64, queryArray.Len())
		for i := range out {
			out[i] = binAscendingI64RangeScalar(domain, queryArray.start+int64(i)*queryArray.step)
		}
		return NewI64(out), true, nil
	}
	if queryArray, ok := query.(Array); ok {
		values, ok := signedArrayData[int64](queryArray)
		if !ok {
			return nil, false, nil
		}
		out := make([]int64, len(values))
		for i, value := range values {
			out[i] = binAscendingI64RangeScalar(domain, value)
		}
		return NewI64(out), true, nil
	}
	value, ok := coerceInt64Exact(query)
	if !ok {
		return nil, false, nil
	}
	return binAscendingI64RangeScalar(domain, value), true, nil
}

func binI64RangeDomainSum(domain i64RangeArray, query any) (int64, bool, error) {
	if domain.step <= 0 {
		return 0, false, nil
	}
	if queryArray, ok := query.(i64RangeArray); ok {
		if sum, ok := binAscendingI64RangeQuerySum(domain, queryArray); ok {
			return sum, true, nil
		}
		var total int64
		for i := 0; i < queryArray.Len(); i++ {
			total += binAscendingI64RangeScalar(domain, queryArray.start+int64(i)*queryArray.step)
		}
		return total, true, nil
	}
	if queryArray, ok := query.(Array); ok {
		values, ok := signedArrayData[int64](queryArray)
		if !ok {
			return 0, false, nil
		}
		var total int64
		for _, value := range values {
			total += binAscendingI64RangeScalar(domain, value)
		}
		return total, true, nil
	}
	value, ok := coerceInt64Exact(query)
	if !ok {
		return 0, false, nil
	}
	return binAscendingI64RangeScalar(domain, value), true, nil
}

func i64ScalarDyadicAffineRange(array i64ScalarDyadicArray) (i64RangeArray, bool) {
	source, ok := array.source.(i64RangeArray)
	if !ok {
		return i64RangeArray{}, false
	}
	start := source.start
	step := source.step
	switch array.op {
	case OpAdd:
		start += array.scalar
	case OpSub:
		if array.scalarLeft {
			start = array.scalar - start
			step = -step
		} else {
			start -= array.scalar
		}
	case OpMul:
		start *= array.scalar
		step *= array.scalar
	default:
		return i64RangeArray{}, false
	}
	if step <= 0 {
		return i64RangeArray{}, false
	}
	return i64RangeArray{start: start, step: step, len: array.len}, true
}

func binAscendingI64RangeQuerySum(domain, query i64RangeArray) (int64, bool) {
	if domain.step <= 0 || query.step <= 0 {
		return 0, false
	}
	n := query.len
	if n == 0 {
		return 0, true
	}
	if domain.len == 0 {
		return -int64(n), true
	}
	last := domain.start + int64(domain.len-1)*domain.step
	lowEnd := lowerBoundI64RangeIndex(query, domain.start)
	highStart := lowerBoundI64RangeIndex(query, last)
	if lowEnd > n {
		lowEnd = n
	}
	if highStart < lowEnd {
		highStart = lowEnd
	}
	if highStart > n {
		highStart = n
	}
	total := -int64(lowEnd)
	middleN := highStart - lowEnd
	if middleN > 0 {
		first := query.start + int64(lowEnd)*query.step - domain.start
		total += floorSumNonNegative(int64(middleN), domain.step, query.step, first)
	}
	total += int64(n-highStart) * int64(domain.len-1)
	return total, true
}

func lowerBoundI64RangeIndex(array i64RangeArray, target int64) int {
	if array.len <= 0 {
		return 0
	}
	if target <= array.start {
		return 0
	}
	diff := target - array.start
	index := diff / array.step
	if diff%array.step != 0 {
		index++
	}
	if index > int64(array.len) {
		return array.len
	}
	return int(index)
}

func binAscendingI64RangeScalar(domain i64RangeArray, query int64) int64 {
	if domain.len == 0 || query < domain.start {
		return -1
	}
	last := domain.start + int64(domain.len-1)*domain.step
	if query >= last {
		return int64(domain.len - 1)
	}
	return (query - domain.start) / domain.step
}

func binUnsigned[T unsignedScalar](domain []T, query any) (any, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := unsignedArrayData[T](queryArray)
		if !ok {
			return nil, false, nil
		}
		return NewI64(binUnsignedSlice(domain, values)), true, nil
	}
	value, ok := query.(T)
	if !ok {
		return nil, false, nil
	}
	if len(domain) == 0 {
		return int64(-1), true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binUnsignedSum[T unsignedScalar](domain []T, query any) (int64, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := unsignedArrayData[T](queryArray)
		if !ok {
			return 0, false, nil
		}
		return binUnsignedSliceSum(domain, values), true, nil
	}
	value, ok := query.(T)
	if !ok {
		return 0, false, nil
	}
	if len(domain) == 0 {
		return -1, true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binFloat[T floatScalar](domain []T, query any) (any, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := floatArrayData[T](queryArray)
		if !ok {
			return nil, false, nil
		}
		return NewI64(binFloatSlice(domain, values)), true, nil
	}
	value, ok := query.(T)
	if !ok {
		return nil, false, nil
	}
	if len(domain) == 0 || math.IsNaN(float64(value)) {
		return int64(-1), true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binFloatSum[T floatScalar](domain []T, query any) (int64, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := floatArrayData[T](queryArray)
		if !ok {
			return 0, false, nil
		}
		return binFloatSliceSum(domain, values), true, nil
	}
	value, ok := query.(T)
	if !ok {
		return 0, false, nil
	}
	if len(domain) == 0 || math.IsNaN(float64(value)) {
		return -1, true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binF64(domain []float64, query any) (any, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := floatArrayData[float64](queryArray)
		if !ok {
			return nil, false, nil
		}
		return NewI64(binFloatSlice(domain, values)), true, nil
	}
	value, ok := numeric(query)
	if !ok {
		return nil, false, nil
	}
	if len(domain) == 0 || math.IsNaN(value) {
		return int64(-1), true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binF64Sum(domain []float64, query any) (int64, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := floatArrayData[float64](queryArray)
		if !ok {
			return 0, false, nil
		}
		return binFloatSliceSum(domain, values), true, nil
	}
	value, ok := numeric(query)
	if !ok {
		return 0, false, nil
	}
	if len(domain) == 0 || math.IsNaN(value) {
		return -1, true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binString(domain []string, query any) (any, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := stringArrayData(queryArray)
		if !ok {
			return nil, false, nil
		}
		out := make([]int64, len(values))
		for i, value := range values {
			out[i] = int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
		}
		return NewI64(out), true, nil
	}
	value, ok := coerceComparableString(query)
	if !ok {
		return nil, false, nil
	}
	if len(domain) == 0 {
		return int64(-1), true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binStringSum(domain []string, query any) (int64, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := stringArrayData(queryArray)
		if !ok {
			return 0, false, nil
		}
		var total int64
		for _, value := range values {
			total += int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
		}
		return total, true, nil
	}
	value, ok := coerceComparableString(query)
	if !ok {
		return 0, false, nil
	}
	if len(domain) == 0 {
		return -1, true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binSymbol(domain []Symbol, query any) (any, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := symbolArrayData(queryArray)
		if !ok {
			return nil, false, nil
		}
		out := make([]int64, len(values))
		for i, value := range values {
			out[i] = int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
		}
		return NewI64(out), true, nil
	}
	value, ok := coerceComparableSymbol(query)
	if !ok {
		return nil, false, nil
	}
	if len(domain) == 0 {
		return int64(-1), true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binSymbolSum(domain []Symbol, query any) (int64, bool, error) {
	if queryArray, ok := query.(Array); ok {
		values, ok := symbolArrayData(queryArray)
		if !ok {
			return 0, false, nil
		}
		var total int64
		for _, value := range values {
			total += int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
		}
		return total, true, nil
	}
	value, ok := coerceComparableSymbol(query)
	if !ok {
		return 0, false, nil
	}
	if len(domain) == 0 {
		return -1, true, nil
	}
	return int64(sort.Search(len(domain), func(i int) bool { return domain[i] > value }) - 1), true, nil
}

func binSignedSlice[T signedScalar](domain, query []T) []int64 {
	out := make([]int64, len(query))
	for i, value := range query {
		out[i] = int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
	}
	return out
}

func binSignedSliceSum[T signedScalar](domain, query []T) int64 {
	var total int64
	for _, value := range query {
		total += int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
	}
	return total
}

func binUnsignedSlice[T unsignedScalar](domain, query []T) []int64 {
	out := make([]int64, len(query))
	for i, value := range query {
		out[i] = int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
	}
	return out
}

func binUnsignedSliceSum[T unsignedScalar](domain, query []T) int64 {
	var total int64
	for _, value := range query {
		total += int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
	}
	return total
}

func binFloatSlice[T floatScalar](domain, query []T) []int64 {
	out := make([]int64, len(query))
	for i, value := range query {
		if math.IsNaN(float64(value)) {
			out[i] = -1
			continue
		}
		out[i] = int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
	}
	return out
}

func binFloatSliceSum[T floatScalar](domain, query []T) int64 {
	var total int64
	for _, value := range query {
		if math.IsNaN(float64(value)) {
			total--
			continue
		}
		total += int64(sort.Search(len(domain), func(row int) bool { return domain[row] > value }) - 1)
	}
	return total
}

func signedArrayData[T signedScalar](array Array) ([]T, bool) {
	for {
		attributed, ok := array.(attributedArray)
		if !ok {
			break
		}
		array = attributed.array
	}
	if typed, ok := array.(columnArray[T]); ok {
		return typed.data, true
	}
	return nil, false
}

func unsignedArrayData[T unsignedScalar](array Array) ([]T, bool) {
	for {
		attributed, ok := array.(attributedArray)
		if !ok {
			break
		}
		array = attributed.array
	}
	if typed, ok := array.(columnArray[T]); ok {
		return typed.data, true
	}
	return nil, false
}

func floatArrayData[T floatScalar](array Array) ([]T, bool) {
	for {
		attributed, ok := array.(attributedArray)
		if !ok {
			break
		}
		array = attributed.array
	}
	if typed, ok := array.(columnArray[T]); ok {
		return typed.data, true
	}
	return nil, false
}

func stringArrayData(array Array) ([]string, bool) {
	for {
		attributed, ok := array.(attributedArray)
		if !ok {
			break
		}
		array = attributed.array
	}
	switch typed := array.(type) {
	case columnArray[string]:
		return typed.data, true
	case columnArray[Symbol]:
		out := make([]string, len(typed.data))
		for i, value := range typed.data {
			out[i] = string(value)
		}
		return out, true
	default:
		return nil, false
	}
}

func symbolArrayData(array Array) ([]Symbol, bool) {
	for {
		attributed, ok := array.(attributedArray)
		if !ok {
			break
		}
		array = attributed.array
	}
	switch typed := array.(type) {
	case columnArray[Symbol]:
		return typed.data, true
	case columnArray[string]:
		out := make([]Symbol, len(typed.data))
		for i, value := range typed.data {
			out[i] = Symbol(value)
		}
		return out, true
	default:
		return nil, false
	}
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
			if index, ok := arrayIndexForBorrowed(partition, ArrayAttributeUnique); ok {
				return sortedRowsByPartitionIndex(timeColumn, index)
			}
			if index, ok := arrayIndexForBorrowed(partition, ArrayAttributeGrouped); ok {
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
	if len(encoder.columns) == 1 && (len(targetKinds) == 0 || encoder.columns[0].kind == frame.columns[columns[0]].Kind()) {
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
		return typedColumnKeyFunc(col, a.data)
	case columnArray[int8]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[int16]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[int32]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[int64]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[uint8]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[uint16]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[uint32]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[uint64]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[float32]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[float64]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[string]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[Symbol]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[Month]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[Date]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[DateTime]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[Timespan]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[Minute]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[Second]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[Time]:
		return typedColumnKeyFunc(col, a.data)
	case columnArray[Timestamp]:
		return typedColumnKeyFunc(col, a.data)
	case nullableArray:
		return nullableColumnKeyFunc(col, a.data)
	case encodedArray:
		return encodedColumnKeyFunc(col, a)
	default:
		return nil
	}
}

func typedColumnKeyFunc[T any](col keyColumn, values []T) func(row int) (string, error) {
	return func(row int) (string, error) {
		if row < 0 || row >= len(values) {
			return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
		}
		return arrayValueKey(col.kind, values[row]), nil
	}
}

func nullableColumnKeyFunc(col keyColumn, values []any) func(row int) (string, error) {
	return func(row int) (string, error) {
		if row < 0 || row >= len(values) {
			return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
		}
		value, err := normalizeKeyValue(col.kind, values[row])
		if err != nil {
			return "", fmt.Errorf("key column %q: %w", col.name, err)
		}
		return arrayValueKey(col.kind, value), nil
	}
}

func encodedColumnKeyFunc(col keyColumn, array encodedArray) func(row int) (string, error) {
	return func(row int) (string, error) {
		if row < 0 || row >= len(array.codes) {
			return "", fmt.Errorf("key column %q row %d out of range", col.name, row)
		}
		code := array.codes[row]
		if code < 0 {
			return arrayValueKey(col.kind, NullValue), nil
		}
		if int(code) >= len(array.domain) {
			return "", fmt.Errorf("encoded key column %q code %d at row %d outside domain length %d", col.name, code, row, len(array.domain))
		}
		value, err := normalizeKeyValue(col.kind, array.domain[code])
		if err != nil {
			return "", fmt.Errorf("key column %q: %w", col.name, err)
		}
		return arrayValueKey(col.kind, value), nil
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
	case i64SparseAmendArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case i64ScalarDyadicArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case i64ScalarDyadicRunningSumArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case tiledArray:
		if row < 0 || row >= a.len || a.source.Len() == 0 {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return typedKernels.NumericAt(a.source, (a.start+row)%a.source.Len())
	case columnArray[int8]:
		return numericColumnAt(a.data, row)
	case columnArray[int16]:
		return numericColumnAt(a.data, row)
	case columnArray[int32]:
		return numericColumnAt(a.data, row)
	case columnArray[int64]:
		return numericColumnAt(a.data, row)
	case i64RangeArray:
		return numericI64RangeAt(a, row)
	case f64RangeArray:
		return numericF64RangeAt(a, row)
	case i64RunningSumArray:
		value, ok := a.i64At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return float64(value), true, nil
	case f64RunningSumArray:
		value, ok := a.f64At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return value, true, nil
	case i64SegmentArray:
		return numericI64SegmentAt(a, row)
	case i64PeriodicIndexArray:
		value, ok := a.i64At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return float64(value), true, nil
	case i64ProductArray:
		return numericI64ProductAt(a, row)
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

func numericI64RangeAt(values i64RangeArray, row int) (float64, bool, error) {
	if row < 0 || row >= values.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(values.start + int64(row)*values.step), true, nil
}

func numericF64RangeAt(values f64RangeArray, row int) (float64, bool, error) {
	if row < 0 || row >= values.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return values.start + float64(row)*values.step, true, nil
}

func numericI64SegmentAt(values i64SegmentArray, row int) (float64, bool, error) {
	value, ok := values.i64At(row)
	if !ok {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(value), true, nil
}

func numericI64ProductAt(values i64ProductArray, row int) (float64, bool, error) {
	value, ok := values.i64At(row)
	if !ok {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(value), true, nil
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
	case tiledArray:
		return isNumericArray(a.source)
	case columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		columnArray[float32], columnArray[float64], i64RangeArray, f64RangeArray,
		i64RunningSumArray, f64RunningSumArray, i64SegmentArray, i64ProductArray,
		i64ScalarDyadicArray, i64ScalarDyadicRunningSumArray:
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
	if out, ok := numericDyadicRange(op, left, right, length); ok {
		return out, true, nil
	}
	values := make([]float64, length)
	var nullable []any
	for i := 0; i < length; i++ {
		lv, ok, err := numericOperandAt(left, i)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			if nullable == nil {
				nullable = numericDyadicNullablePrefix(values, i)
			}
			nullable[i] = NullValue
			continue
		}
		rv, ok, err := numericOperandAt(right, i)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			if nullable == nil {
				nullable = numericDyadicNullablePrefix(values, i)
			}
			nullable[i] = NullValue
			continue
		}
		out, err := applyNumericBinaryFloat(op, lv, rv)
		if err != nil {
			return nil, true, err
		}
		if nullable != nil {
			nullable[i] = out
			continue
		}
		values[i] = out
	}
	if nullable != nil {
		return newNullableArray(KindF64, nullable), true, nil
	}
	return columnArray[float64]{kind: KindF64, data: values}, true, nil
}

func numericDyadicRange(op Op, left, right any, length int) (Array, bool) {
	if leftRange, ok := asNumericRangeArray(left); ok {
		if scalar, ok := numericScalarValue(right); ok {
			return applyF64RangeScalar(op, leftRange, scalar, false, length)
		}
	}
	if rightRange, ok := asNumericRangeArray(right); ok {
		if scalar, ok := numericScalarValue(left); ok {
			return applyF64RangeScalar(op, rightRange, scalar, true, length)
		}
	}
	return nil, false
}

func asNumericRangeArray(value any) (f64RangeArray, bool) {
	switch a := value.(type) {
	case attributedArray:
		return asNumericRangeArray(a.array)
	case i64RangeArray:
		return f64RangeArray{start: float64(a.start), step: float64(a.step), len: a.len}, true
	case f64RangeArray:
		return a, true
	default:
		return f64RangeArray{}, false
	}
}

func applyF64RangeScalar(op Op, values f64RangeArray, scalar float64, scalarLeft bool, length int) (Array, bool) {
	if values.len != length {
		return nil, false
	}
	switch op {
	case OpAdd:
		return f64RangeArray{start: values.start + scalar, step: values.step, len: values.len}, true
	case OpSub:
		if scalarLeft {
			return f64RangeArray{start: scalar - values.start, step: -values.step, len: values.len}, true
		}
		return f64RangeArray{start: values.start - scalar, step: values.step, len: values.len}, true
	case OpMul:
		return f64RangeArray{start: values.start * scalar, step: values.step * scalar, len: values.len}, true
	case OpDiv:
		if scalarLeft {
			return nil, false
		}
		return f64RangeArray{start: values.start / scalar, step: values.step / scalar, len: values.len}, true
	default:
		return nil, false
	}
}

func numericIntegerDyadic(op Op, left, right any, length int) (Array, bool, error) {
	if out, ok := numericIntegerDyadicRange(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok, err := numericIntegerDyadicTiledScalar(op, left, right); ok || err != nil {
		return out, ok, err
	}
	values := make([]int64, length)
	var nullable []any
	for i := 0; i < length; i++ {
		lv, ok, err := integerOperandAt(left, i)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			if nullable == nil {
				nullable = numericIntegerDyadicNullablePrefix(values, i)
			}
			nullable[i] = NullValue
			continue
		}
		rv, ok, err := integerOperandAt(right, i)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			if nullable == nil {
				nullable = numericIntegerDyadicNullablePrefix(values, i)
			}
			nullable[i] = NullValue
			continue
		}
		var out int64
		switch op {
		case OpAdd:
			out = lv + rv
		case OpSub:
			out = lv - rv
		case OpMul:
			out = lv * rv
		case OpMod:
			if rv == 0 {
				if nullable == nil {
					nullable = numericIntegerDyadicNullablePrefix(values, i)
				}
				nullable[i] = NullValue
				continue
			}
			out = qModInt64(lv, rv)
		default:
			return nil, false, nil
		}
		if nullable != nil {
			nullable[i] = out
			continue
		}
		values[i] = out
	}
	if nullable != nil {
		return newNullableArray(KindI64, nullable), true, nil
	}
	return columnArray[int64]{kind: KindI64, data: values}, true, nil
}

func numericIntegerDyadicTiledScalar(op Op, left, right any) (Array, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	switch {
	case leftIsArray && !rightIsArray:
		tiled, ok := unwrapTiledArray(leftArray)
		if !ok {
			return nil, false, nil
		}
		scalar, ok := integerScalarValue(right)
		if !ok {
			return nil, false, nil
		}
		return applyI64TiledScalar(op, tiled, scalar, false)
	case rightIsArray && !leftIsArray:
		tiled, ok := unwrapTiledArray(rightArray)
		if !ok {
			return nil, false, nil
		}
		scalar, ok := integerScalarValue(left)
		if !ok {
			return nil, false, nil
		}
		return applyI64TiledScalar(op, tiled, scalar, true)
	default:
		return nil, false, nil
	}
}

func applyI64TiledScalar(op Op, values tiledArray, scalar int64, scalarLeft bool) (Array, bool, error) {
	sourceLen := values.source.Len()
	if sourceLen == 0 {
		return nil, false, nil
	}
	if op == OpMod && !scalarLeft && scalar == 0 {
		return nil, false, nil
	}
	return i64ScalarDyadicArray{source: values, op: op, scalar: scalar, scalarLeft: scalarLeft, len: values.len}, true, nil
}

func qModInt64(left, right int64) int64 {
	return left - right*int64(math.Floor(float64(left)/float64(right)))
}

func applyI64ScalarDyadicValue(op Op, value, scalar int64, scalarLeft bool) (int64, bool, error) {
	switch op {
	case OpAdd:
		return value + scalar, true, nil
	case OpSub:
		if scalarLeft {
			return scalar - value, true, nil
		}
		return value - scalar, true, nil
	case OpMul:
		return value * scalar, true, nil
	case OpMod:
		divisor := scalar
		dividend := value
		if scalarLeft {
			divisor = value
			dividend = scalar
		}
		if divisor == 0 {
			return 0, false, nil
		}
		return qModInt64(dividend, divisor), true, nil
	default:
		return 0, false, nil
	}
}

func numericIntegerDyadicRange(op Op, left, right any, length int) (Array, bool) {
	leftRange, leftRangeOK := asI64RangeArray(left)
	rightRange, rightRangeOK := asI64RangeArray(right)
	leftScalar, leftScalarOK := integerScalarValue(left)
	rightScalar, rightScalarOK := integerScalarValue(right)
	switch {
	case leftRangeOK && rightScalarOK:
		if leftRange.len != length {
			return nil, false
		}
		return applyI64RangeScalar(op, leftRange, rightScalar, false)
	case leftScalarOK && rightRangeOK:
		if rightRange.len != length {
			return nil, false
		}
		return applyI64RangeScalar(op, rightRange, leftScalar, true)
	case leftRangeOK && rightRangeOK:
		if leftRange.len != length || rightRange.len != length || leftRange.len != rightRange.len {
			return nil, false
		}
		return applyI64RangeRange(op, leftRange, rightRange)
	default:
		return nil, false
	}
}

func asI64RangeArray(value any) (i64RangeArray, bool) {
	switch a := value.(type) {
	case attributedArray:
		return asI64RangeArray(a.array)
	case i64RangeArray:
		return a, true
	default:
		return i64RangeArray{}, false
	}
}

func applyI64RangeScalar(op Op, values i64RangeArray, scalar int64, scalarLeft bool) (Array, bool) {
	switch op {
	case OpAdd:
		return i64RangeArray{start: values.start + scalar, step: values.step, len: values.len}, true
	case OpSub:
		if scalarLeft {
			return i64RangeArray{start: scalar - values.start, step: -values.step, len: values.len}, true
		}
		return i64RangeArray{start: values.start - scalar, step: values.step, len: values.len}, true
	case OpMul:
		return i64RangeArray{start: values.start * scalar, step: values.step * scalar, len: values.len}, true
	case OpMod:
		if scalarLeft || scalar == 0 {
			return nil, false
		}
		return i64ScalarDyadicArray{source: values, op: op, scalar: scalar, len: values.len}, true
	default:
		return nil, false
	}
}

func applyI64RangeRange(op Op, left, right i64RangeArray) (Array, bool) {
	switch op {
	case OpAdd:
		return i64RangeArray{start: left.start + right.start, step: left.step + right.step, len: left.len}, true
	case OpSub:
		return i64RangeArray{start: left.start - right.start, step: left.step - right.step, len: left.len}, true
	case OpMul:
		return i64ProductArray{left: left, right: right}, true
	default:
		return nil, false
	}
}

func numericDyadicNullablePrefix(values []float64, upto int) []any {
	out := make([]any, len(values))
	for i := 0; i < upto; i++ {
		out[i] = values[i]
	}
	return out
}

func numericIntegerDyadicNullablePrefix(values []int64, upto int) []any {
	out := make([]any, len(values))
	for i := 0; i < upto; i++ {
		out[i] = values[i]
	}
	return out
}

func typedIntegerOperand(value any) bool {
	if array, ok := value.(Array); ok {
		return isIntegerArray(array)
	}
	if IsNull(value) {
		return true
	}
	_, ok := integerScalarValue(value)
	return ok
}

func isIntegerArray(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return isIntegerArray(a.array)
	case tiledArray:
		return isIntegerArray(a.source)
	case i64SparseAmendArray:
		return true
	case i64ScalarDyadicArray:
		return true
	case i64ScalarDyadicRunningSumArray:
		return true
	case columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		i64RangeArray, i64RunningSumArray, i64SegmentArray, i64PeriodicIndexArray, i64ProductArray:
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
			if _, ok := integerScalarValue(v); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isDenseIntegerArray(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return isDenseIntegerArray(a.array)
	case tiledArray:
		return isDenseIntegerArray(a.source)
	case i64SparseAmendArray:
		return true
	case i64ScalarDyadicArray:
		return true
	case i64ScalarDyadicRunningSumArray:
		return true
	case columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		i64RangeArray, i64RunningSumArray, i64SegmentArray, i64PeriodicIndexArray, i64ProductArray:
		return true
	default:
		return false
	}
}

func integerOperandAt(value any, row int) (int64, bool, error) {
	if array, ok := value.(Array); ok {
		return integerArrayAt(array, row)
	}
	if IsNull(value) {
		return 0, false, nil
	}
	n, ok := integerScalarValue(value)
	if !ok {
		return 0, false, fmt.Errorf("typed integer operand row %d is %T, want integer", row, value)
	}
	return n, true, nil
}

func integerArrayAt(array Array, row int) (int64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return integerArrayAt(a.array, row)
	case i64SparseAmendArray:
		return a.i64At(row)
	case tiledArray:
		if row < 0 || row >= a.len || a.source.Len() == 0 {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return integerArrayAt(a.source, (a.start+row)%a.source.Len())
	case columnArray[int8]:
		return integerColumnAt(a.data, row)
	case columnArray[int16]:
		return integerColumnAt(a.data, row)
	case columnArray[int32]:
		return integerColumnAt(a.data, row)
	case columnArray[int64]:
		return integerColumnAt(a.data, row)
	case i64RangeArray:
		if row < 0 || row >= a.len {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return a.start + int64(row)*a.step, true, nil
	case i64RunningSumArray:
		value, ok := a.i64At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return value, true, nil
	case i64ScalarDyadicRunningSumArray:
		return a.i64At(row)
	case i64SegmentArray:
		value, ok := a.i64At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return value, true, nil
	case i64PeriodicIndexArray:
		value, ok := a.i64At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return value, true, nil
	case i64ProductArray:
		value, ok := a.i64At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return value, true, nil
	case columnArray[uint8]:
		return integerColumnAt(a.data, row)
	case columnArray[uint16]:
		return integerColumnAt(a.data, row)
	case columnArray[uint32]:
		return integerColumnAt(a.data, row)
	case columnArray[uint64]:
		return integerColumnAt(a.data, row)
	default:
		v, ok := array.At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		if IsNull(v) {
			return 0, false, nil
		}
		n, ok := integerScalarValue(v)
		if !ok {
			return 0, false, fmt.Errorf("typed integer operand row %d is %T, want integer", row, v)
		}
		return n, true, nil
	}
}

func integerColumnAt[T signedScalar | unsignedScalar](values []T, row int) (int64, bool, error) {
	if row < 0 || row >= len(values) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return int64(values[row]), true, nil
}

func integerScalarValue(value any) (int64, bool) {
	switch n := value.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	default:
		return 0, false
	}
}

func compareDyadic(op Op, left, right any, length int) (Array, bool, error) {
	if out, ok := compareI64RangeScalarDyadic(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok := compareI64ScalarDyadicScalarDyadic(op, left, right, length); ok {
		return out, true, nil
	}
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
	return newBoolTrusted(out), true, nil
}

type i64ScalarDyadicCompareMask struct {
	values     i64ScalarDyadicArray
	op         Op
	scalar     int64
	scalarLeft bool
}

func (a i64ScalarDyadicCompareMask) Kind() Kind { return KindBool }

func (a i64ScalarDyadicCompareMask) Len() int { return a.values.len }

func (a i64ScalarDyadicCompareMask) At(row int) (any, bool) {
	if row < 0 || row >= a.values.len {
		return nil, false
	}
	value, ok, err := a.valueAt(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64ScalarDyadicCompareMask) Values() []any {
	out := make([]any, a.values.len)
	for row := range out {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data scalar dyadic compare row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64ScalarDyadicCompareMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.values.len {
			panic(fmt.Sprintf("data scalar dyadic compare gather index %d out of range", row))
		}
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data scalar dyadic compare row %d out of range", row))
		}
		out[i] = value
	}
	return newBoolTrusted(out)
}

func (a i64ScalarDyadicCompareMask) valueAt(row int) (bool, bool, error) {
	value, ok, err := a.values.i64At(row)
	if err != nil || !ok {
		return false, ok, err
	}
	if a.scalarLeft {
		return boolCompare(a.op, a.scalar == value, compareInt64(a.scalar, value)), true, nil
	}
	return boolCompare(a.op, value == a.scalar, compareInt64(value, a.scalar)), true, nil
}

func (a i64ScalarDyadicCompareMask) trueCount() (int64, bool, error) {
	if count, ok := i64ScalarDyadicCompareMaskTrueCount(a); ok {
		return count, true, nil
	}
	var count int64
	for row := 0; row < a.values.len; row++ {
		keep, ok, err := a.valueAt(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		if keep {
			count++
		}
	}
	return count, true, nil
}

func i64ScalarDyadicCompareMaskTrueCount(mask i64ScalarDyadicCompareMask) (int64, bool) {
	plan, ok := i64ScalarDyadicCompareModuloPlan(mask)
	if !ok {
		return 0, false
	}
	count, ok := plan.trueCount()
	return count, ok
}

type i64RangeCompareMask struct {
	values     i64RangeArray
	op         Op
	scalar     int64
	scalarLeft bool
}

func (a i64RangeCompareMask) Kind() Kind { return KindBool }

func (a i64RangeCompareMask) Len() int { return a.values.len }

func (a i64RangeCompareMask) At(row int) (any, bool) {
	if row < 0 || row >= a.values.len {
		return nil, false
	}
	return a.valueAt(row), true
}

func (a i64RangeCompareMask) Values() []any {
	out := make([]any, a.values.len)
	for row := range out {
		out[row] = a.valueAt(row)
	}
	return out
}

func (a i64RangeCompareMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.values.len {
			panic(fmt.Sprintf("data range compare gather index %d out of range", row))
		}
		out[i] = a.valueAt(row)
	}
	return newBoolTrusted(out)
}

func (a i64RangeCompareMask) valueAt(row int) bool {
	value := a.values.start + int64(row)*a.values.step
	if a.scalarLeft {
		return boolCompare(a.op, a.scalar == value, compareInt64(a.scalar, value))
	}
	return boolCompare(a.op, value == a.scalar, compareInt64(value, a.scalar))
}

func (a i64RangeCompareMask) trueCount() int64 {
	var count int64
	for row := 0; row < a.values.len; row++ {
		if a.valueAt(row) {
			count++
		}
	}
	return count
}

func compareI64ScalarDyadicScalarDyadic(op Op, left, right any, length int) (Array, bool) {
	leftValues, leftOK := left.(i64ScalarDyadicArray)
	rightValues, rightOK := right.(i64ScalarDyadicArray)
	leftScalar, leftScalarOK := integerScalarValue(left)
	rightScalar, rightScalarOK := integerScalarValue(right)
	switch {
	case leftOK && rightScalarOK:
		if leftValues.len != length {
			return nil, false
		}
		return i64ScalarDyadicCompareMask{values: leftValues, op: op, scalar: rightScalar}, true
	case leftScalarOK && rightOK:
		if rightValues.len != length {
			return nil, false
		}
		return i64ScalarDyadicCompareMask{values: rightValues, op: op, scalar: leftScalar, scalarLeft: true}, true
	default:
		return nil, false
	}
}

func compareI64RangeScalarDyadic(op Op, left, right any, length int) (Array, bool) {
	leftRange, leftRangeOK := asI64RangeArray(left)
	rightRange, rightRangeOK := asI64RangeArray(right)
	leftScalar, leftScalarOK := integerScalarValue(left)
	rightScalar, rightScalarOK := integerScalarValue(right)
	switch {
	case leftRangeOK && rightScalarOK:
		if leftRange.len != length {
			return nil, false
		}
		return i64RangeCompareMask{values: leftRange, op: op, scalar: rightScalar}, true
	case leftScalarOK && rightRangeOK:
		if rightRange.len != length {
			return nil, false
		}
		return i64RangeCompareMask{values: rightRange, op: op, scalar: leftScalar, scalarLeft: true}, true
	default:
		return nil, false
	}
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
	if array, ok := value.(Array); ok {
		return typedKernels.NumericAt(array, row)
	}
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
	case OpAdd, OpSub, OpMul, OpDiv, OpMod:
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
	return newF64Trusted(out), true, nil
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

func qNumericUnaryIntegerArray(op string, array Array) (Array, bool, error) {
	const minInt64 = -1 << 63
	switch op {
	case NumericUnaryNeg:
		out := make([]int64, array.Len())
		for i := range out {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok || value == minInt64 {
				return nil, false, nil
			}
			out[i] = -value
		}
		return newI64Trusted(out), true, nil
	case NumericUnaryAbs:
		out := make([]int64, array.Len())
		for i := range out {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok || value == minInt64 {
				return nil, false, nil
			}
			if value < 0 {
				value = -value
			}
			out[i] = value
		}
		return newI64Trusted(out), true, nil
	case NumericUnaryFloor, NumericUnaryCeiling:
		out := make([]int64, array.Len())
		for i := range out {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out[i] = value
		}
		return newI64Trusted(out), true, nil
	case NumericUnarySignum:
		out := make([]int64, array.Len())
		for i := range out {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			switch {
			case value < 0:
				out[i] = -1
			case value > 0:
				out[i] = 1
			default:
				out[i] = 0
			}
		}
		return newI64Trusted(out), true, nil
	case NumericUnaryExp, NumericUnaryRecip:
		out := make([]float64, array.Len())
		for i := range out {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			switch op {
			case NumericUnaryExp:
				out[i] = math.Exp(float64(value))
			case NumericUnaryRecip:
				out[i] = 1 / float64(value)
			}
		}
		return newF64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnaryFloatSlice[T floatScalar](op string, values []T) (Array, bool, error) {
	switch op {
	case NumericUnaryNeg:
		out := make([]float64, len(values))
		for i, value := range values {
			out[i] = -float64(value)
		}
		return newF64Trusted(out), true, nil
	case NumericUnaryAbs:
		out := make([]float64, len(values))
		for i, value := range values {
			out[i] = math.Abs(float64(value))
		}
		return newF64Trusted(out), true, nil
	case NumericUnaryExp:
		out := make([]float64, len(values))
		for i, value := range values {
			out[i] = math.Exp(float64(value))
		}
		return newF64Trusted(out), true, nil
	case NumericUnaryRecip:
		out := make([]float64, len(values))
		for i, value := range values {
			out[i] = 1 / float64(value)
		}
		return newF64Trusted(out), true, nil
	case NumericUnarySignum:
		out := make([]int64, len(values))
		for i, value := range values {
			switch n := float64(value); {
			case n < 0:
				out[i] = -1
			case n > 0:
				out[i] = 1
			default:
				out[i] = 0
			}
		}
		return newI64Trusted(out), true, nil
	case NumericUnaryFloor:
		out := make([]int64, len(values))
		for i, value := range values {
			out[i] = int64(math.Floor(float64(value)))
		}
		return newI64Trusted(out), true, nil
	case NumericUnaryCeiling:
		out := make([]int64, len(values))
		for i, value := range values {
			out[i] = int64(math.Ceil(float64(value)))
		}
		return newI64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnarySumIntegerArray(op string, array Array) (any, bool, error) {
	const minInt64 = -1 << 63
	switch op {
	case NumericUnaryNeg:
		var sum int64
		for i := 0; i < array.Len(); i++ {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok || value == minInt64 {
				return nil, false, nil
			}
			sum -= value
		}
		return sum, true, nil
	case NumericUnaryAbs:
		var sum int64
		for i := 0; i < array.Len(); i++ {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok || value == minInt64 {
				return nil, false, nil
			}
			if value < 0 {
				value = -value
			}
			sum += value
		}
		return sum, true, nil
	case NumericUnaryFloor, NumericUnaryCeiling:
		return TryTypedNumericSum(array)
	case NumericUnarySignum:
		var sum int64
		for i := 0; i < array.Len(); i++ {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			switch {
			case value < 0:
				sum--
			case value > 0:
				sum++
			}
		}
		return sum, true, nil
	case NumericUnaryExp:
		var sum float64
		for i := 0; i < array.Len(); i++ {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			sum += math.Exp(float64(value))
		}
		return sum, true, nil
	case NumericUnaryRecip:
		var sum float64
		for i := 0; i < array.Len(); i++ {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			sum += 1 / float64(value)
		}
		return sum, true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnarySumFloatSlice[T floatScalar](op string, values []T) (any, bool, error) {
	switch op {
	case NumericUnaryNeg:
		var sum float64
		for _, value := range values {
			sum -= float64(value)
		}
		return sum, true, nil
	case NumericUnaryAbs:
		var sum float64
		for _, value := range values {
			sum += math.Abs(float64(value))
		}
		return sum, true, nil
	case NumericUnaryExp:
		var sum float64
		for _, value := range values {
			sum += math.Exp(float64(value))
		}
		return sum, true, nil
	case NumericUnaryRecip:
		var sum float64
		for _, value := range values {
			sum += 1 / float64(value)
		}
		return sum, true, nil
	case NumericUnarySignum:
		var sum int64
		for _, value := range values {
			switch n := float64(value); {
			case n < 0:
				sum--
			case n > 0:
				sum++
			}
		}
		return sum, true, nil
	case NumericUnaryFloor:
		var sum int64
		for _, value := range values {
			sum += int64(math.Floor(float64(value)))
		}
		return sum, true, nil
	case NumericUnaryCeiling:
		var sum int64
		for _, value := range values {
			sum += int64(math.Ceil(float64(value)))
		}
		return sum, true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnaryTiledSumReturnsFloat(op string, source Array) bool {
	switch op {
	case NumericUnaryExp, NumericUnaryRecip:
		return true
	case NumericUnaryNeg, NumericUnaryAbs:
		return !isDenseIntegerArray(source)
	default:
		return false
	}
}

func qNumericUnaryTiledSumReturnsInt(op string, source Array) bool {
	switch op {
	case NumericUnaryFloor, NumericUnaryCeiling, NumericUnarySignum:
		return true
	case NumericUnaryNeg, NumericUnaryAbs:
		return isDenseIntegerArray(source)
	default:
		return false
	}
}

func qNumericUnaryFloatSumWindow(op string, array Array, start, count int) (float64, bool, error) {
	var sum float64
	sourceLen := array.Len()
	for i := 0; i < count; i++ {
		value, ok, err := numericAt(array, (start+i)%sourceLen)
		if err != nil {
			return 0, true, err
		}
		if !ok {
			return 0, false, nil
		}
		switch op {
		case NumericUnaryNeg:
			sum -= value
		case NumericUnaryAbs:
			sum += math.Abs(value)
		case NumericUnaryExp:
			sum += math.Exp(value)
		case NumericUnaryRecip:
			sum += 1 / value
		default:
			return 0, false, nil
		}
	}
	return sum, true, nil
}

func qNumericUnaryIntSumWindow(op string, array Array, start, count int) (int64, bool, error) {
	var sum int64
	sourceLen := array.Len()
	switch op {
	case NumericUnaryNeg:
		for i := 0; i < count; i++ {
			value, ok, err := integerArrayAt(array, (start+i)%sourceLen)
			if err != nil {
				return 0, true, err
			}
			if !ok || value == math.MinInt64 {
				return 0, false, nil
			}
			sum -= value
		}
		return sum, true, nil
	case NumericUnaryAbs:
		for i := 0; i < count; i++ {
			value, ok, err := integerArrayAt(array, (start+i)%sourceLen)
			if err != nil {
				return 0, true, err
			}
			if !ok || value == math.MinInt64 {
				return 0, false, nil
			}
			if value < 0 {
				value = -value
			}
			sum += value
		}
		return sum, true, nil
	case NumericUnaryFloor, NumericUnaryCeiling:
		for i := 0; i < count; i++ {
			value, ok, err := numericAt(array, (start+i)%sourceLen)
			if err != nil {
				return 0, true, err
			}
			if !ok {
				return 0, false, nil
			}
			if op == NumericUnaryFloor {
				sum += int64(math.Floor(value))
			} else {
				sum += int64(math.Ceil(value))
			}
		}
		return sum, true, nil
	case NumericUnarySignum:
		for i := 0; i < count; i++ {
			value, ok, err := numericAt(array, (start+i)%sourceLen)
			if err != nil {
				return 0, true, err
			}
			if !ok {
				return 0, false, nil
			}
			switch {
			case value < 0:
				sum--
			case value > 0:
				sum++
			}
		}
		return sum, true, nil
	default:
		return 0, false, nil
	}
}

func qNumericUnaryDyadicSumTiledScalar(unaryOp string, dyadicOp Op, left, right any) (any, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	switch {
	case leftIsArray && !rightIsArray:
		tiled, ok := unwrapTiledArray(leftArray)
		if !ok {
			return nil, false, nil
		}
		scalar, ok := numericScalarValue(right)
		if !ok {
			return nil, false, nil
		}
		return qNumericUnaryDyadicSumTiledScalarSide(unaryOp, dyadicOp, tiled, scalar, false)
	case rightIsArray && !leftIsArray:
		tiled, ok := unwrapTiledArray(rightArray)
		if !ok {
			return nil, false, nil
		}
		scalar, ok := numericScalarValue(left)
		if !ok {
			return nil, false, nil
		}
		return qNumericUnaryDyadicSumTiledScalarSide(unaryOp, dyadicOp, tiled, scalar, true)
	default:
		return nil, false, nil
	}
}

func unwrapTiledArray(array Array) (tiledArray, bool) {
	switch a := array.(type) {
	case attributedArray:
		return unwrapTiledArray(a.array)
	case tiledArray:
		return a, true
	default:
		return tiledArray{}, false
	}
}

func qNumericUnaryDyadicSumTiledScalarSide(unaryOp string, dyadicOp Op, array tiledArray, scalar float64, scalarLeft bool) (any, bool, error) {
	sourceLen := array.source.Len()
	if array.len == 0 {
		if qNumericUnaryDyadicSumReturnsInt(unaryOp) {
			return int64(0), true, nil
		}
		if qNumericUnaryDyadicSumReturnsFloat(unaryOp) {
			return float64(0), true, nil
		}
		return nil, false, nil
	}
	if sourceLen == 0 {
		return nil, false, nil
	}
	cycles := array.len / sourceLen
	remainder := array.len % sourceLen
	if qNumericUnaryDyadicSumReturnsInt(unaryOp) {
		period, ok, err := qNumericUnaryDyadicIntSumWindow(unaryOp, dyadicOp, array.source, array.start, sourceLen, scalar, scalarLeft)
		if err != nil || !ok {
			return nil, ok, err
		}
		tail, ok, err := qNumericUnaryDyadicIntSumWindow(unaryOp, dyadicOp, array.source, array.start, remainder, scalar, scalarLeft)
		if err != nil || !ok {
			return nil, ok, err
		}
		return period*int64(cycles) + tail, true, nil
	}
	if qNumericUnaryDyadicSumReturnsFloat(unaryOp) {
		period, ok, err := qNumericUnaryDyadicFloatSumWindow(unaryOp, dyadicOp, array.source, array.start, sourceLen, scalar, scalarLeft)
		if err != nil || !ok {
			return nil, ok, err
		}
		tail, ok, err := qNumericUnaryDyadicFloatSumWindow(unaryOp, dyadicOp, array.source, array.start, remainder, scalar, scalarLeft)
		if err != nil || !ok {
			return nil, ok, err
		}
		return period*float64(cycles) + tail, true, nil
	}
	return nil, false, nil
}

func qNumericUnaryDyadicSumReturnsFloat(op string) bool {
	switch op {
	case NumericUnaryNeg, NumericUnaryAbs, NumericUnaryExp, NumericUnaryRecip:
		return true
	default:
		return false
	}
}

func qNumericUnaryDyadicSumReturnsInt(op string) bool {
	switch op {
	case NumericUnaryFloor, NumericUnaryCeiling, NumericUnarySignum:
		return true
	default:
		return false
	}
}

func qNumericUnaryDyadicFloatSumWindow(unaryOp string, dyadicOp Op, array Array, start, count int, scalar float64, scalarLeft bool) (float64, bool, error) {
	var sum float64
	sourceLen := array.Len()
	for i := 0; i < count; i++ {
		base, ok, err := numericAt(array, (start+i)%sourceLen)
		if err != nil {
			return 0, true, err
		}
		if !ok {
			return 0, false, nil
		}
		value, err := applyTiledScalarDyadic(dyadicOp, base, scalar, scalarLeft)
		if err != nil {
			return 0, true, err
		}
		switch unaryOp {
		case NumericUnaryNeg:
			sum -= value
		case NumericUnaryAbs:
			sum += math.Abs(value)
		case NumericUnaryExp:
			sum += math.Exp(value)
		case NumericUnaryRecip:
			sum += 1 / value
		default:
			return 0, false, nil
		}
	}
	return sum, true, nil
}

func qNumericUnaryDyadicIntSumWindow(unaryOp string, dyadicOp Op, array Array, start, count int, scalar float64, scalarLeft bool) (int64, bool, error) {
	var sum int64
	sourceLen := array.Len()
	for i := 0; i < count; i++ {
		base, ok, err := numericAt(array, (start+i)%sourceLen)
		if err != nil {
			return 0, true, err
		}
		if !ok {
			return 0, false, nil
		}
		value, err := applyTiledScalarDyadic(dyadicOp, base, scalar, scalarLeft)
		if err != nil {
			return 0, true, err
		}
		switch unaryOp {
		case NumericUnaryFloor:
			sum += int64(math.Floor(value))
		case NumericUnaryCeiling:
			sum += int64(math.Ceil(value))
		case NumericUnarySignum:
			switch {
			case value < 0:
				sum--
			case value > 0:
				sum++
			}
		default:
			return 0, false, nil
		}
	}
	return sum, true, nil
}

func applyTiledScalarDyadic(op Op, value, scalar float64, scalarLeft bool) (float64, error) {
	if scalarLeft {
		return applyNumericBinaryFloat(op, scalar, value)
	}
	return applyNumericBinaryFloat(op, value, scalar)
}

func qNumericUnaryDyadicSum(unaryOp string, dyadicOp Op, left, right any, length int) (any, bool, error) {
	switch unaryOp {
	case NumericUnaryNeg, NumericUnaryAbs, NumericUnaryExp, NumericUnaryRecip:
		var sum float64
		for row := 0; row < length; row++ {
			value, ok, err := numericDyadicValueAt(dyadicOp, left, right, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				continue
			}
			switch unaryOp {
			case NumericUnaryNeg:
				sum -= value
			case NumericUnaryAbs:
				sum += math.Abs(value)
			case NumericUnaryExp:
				sum += math.Exp(value)
			case NumericUnaryRecip:
				sum += 1 / value
			}
		}
		return sum, true, nil
	case NumericUnaryFloor, NumericUnaryCeiling:
		var sum int64
		for row := 0; row < length; row++ {
			value, ok, err := numericDyadicValueAt(dyadicOp, left, right, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				continue
			}
			if unaryOp == NumericUnaryFloor {
				sum += int64(math.Floor(value))
			} else {
				sum += int64(math.Ceil(value))
			}
		}
		return sum, true, nil
	case NumericUnarySignum:
		var sum int64
		for row := 0; row < length; row++ {
			value, ok, err := numericDyadicValueAt(dyadicOp, left, right, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				continue
			}
			switch {
			case value < 0:
				sum--
			case value > 0:
				sum++
			}
		}
		return sum, true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnaryDyadicSumRangeScalar(unaryOp string, dyadicOp Op, left, right any) (any, bool, error) {
	if leftRange, ok := asI64RangeArray(left); ok {
		if scalar, ok := numericScalarValue(right); ok {
			return qNumericUnaryDyadicSumI64RangeScalar(unaryOp, dyadicOp, leftRange, scalar, false)
		}
	}
	if rightRange, ok := asI64RangeArray(right); ok {
		if scalar, ok := numericScalarValue(left); ok {
			return qNumericUnaryDyadicSumI64RangeScalar(unaryOp, dyadicOp, rightRange, scalar, true)
		}
	}
	return nil, false, nil
}

func qNumericUnaryDyadicSumI64RangeScalar(unaryOp string, dyadicOp Op, values i64RangeArray, scalar float64, scalarLeft bool) (any, bool, error) {
	if unaryOp == NumericUnaryNeg || unaryOp == NumericUnaryAbs {
		if sum, ok, err := qNumericUnaryDyadicLinearSumI64RangeScalar(unaryOp, dyadicOp, values, scalar, scalarLeft); ok || err != nil {
			return sum, ok, err
		}
	}
	if unaryOp == NumericUnaryFloor || unaryOp == NumericUnaryCeiling {
		if sum, ok, err := qNumericUnaryDyadicFloorCeilSumI64RangeScalar(unaryOp, dyadicOp, values, scalar, scalarLeft); ok || err != nil {
			return sum, ok, err
		}
	}
	switch unaryOp {
	case NumericUnaryNeg, NumericUnaryAbs, NumericUnaryExp, NumericUnaryRecip:
		var sum float64
		for i := 0; i < values.len; i++ {
			value, err := numericRangeScalarValueAt(dyadicOp, values, scalar, scalarLeft, i)
			if err != nil {
				return nil, true, err
			}
			switch unaryOp {
			case NumericUnaryNeg:
				sum -= value
			case NumericUnaryAbs:
				sum += math.Abs(value)
			case NumericUnaryExp:
				sum += math.Exp(value)
			case NumericUnaryRecip:
				sum += 1 / value
			}
		}
		return sum, true, nil
	case NumericUnaryFloor, NumericUnaryCeiling:
		var sum int64
		for i := 0; i < values.len; i++ {
			value, err := numericRangeScalarValueAt(dyadicOp, values, scalar, scalarLeft, i)
			if err != nil {
				return nil, true, err
			}
			if unaryOp == NumericUnaryFloor {
				sum += int64(math.Floor(value))
			} else {
				sum += int64(math.Ceil(value))
			}
		}
		return sum, true, nil
	case NumericUnarySignum:
		var sum int64
		for i := 0; i < values.len; i++ {
			value, err := numericRangeScalarValueAt(dyadicOp, values, scalar, scalarLeft, i)
			if err != nil {
				return nil, true, err
			}
			switch {
			case value < 0:
				sum--
			case value > 0:
				sum++
			}
		}
		return sum, true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnarySumI64Range(op string, values i64RangeArray) (any, bool) {
	switch op {
	case NumericUnaryNeg:
		return -i64RangeSum(values), true
	case NumericUnaryAbs:
		if values.len == 0 {
			return int64(0), true
		}
		first := values.start
		last := values.start + int64(values.len-1)*values.step
		if first >= 0 && last >= 0 {
			return i64RangeSum(values), true
		}
		if first <= 0 && last <= 0 {
			return -i64RangeSum(values), true
		}
	case NumericUnaryFloor, NumericUnaryCeiling:
		return i64RangeSum(values), true
	case NumericUnarySignum:
		if values.len == 0 {
			return int64(0), true
		}
		first := values.start
		last := values.start + int64(values.len-1)*values.step
		switch {
		case first > 0 && last > 0:
			return int64(values.len), true
		case first < 0 && last < 0:
			return -int64(values.len), true
		case first == 0 && last == 0:
			return int64(0), true
		}
	}
	return nil, false
}

func qNumericUnaryDyadicLinearSumI64RangeScalar(unaryOp string, dyadicOp Op, values i64RangeArray, scalar float64, scalarLeft bool) (float64, bool, error) {
	a, b, denominator, ok, err := rangeScalarLinearRational(values, dyadicOp, scalar, scalarLeft)
	if err != nil || !ok {
		return 0, ok, err
	}
	sum := linearRationalRangeSum(values.len, a, b, denominator)
	if unaryOp == NumericUnaryNeg {
		return -sum, true, nil
	}
	if values.len == 0 {
		return 0, true, nil
	}
	first := float64(a) / float64(denominator)
	last := float64(a+b*int64(values.len-1)) / float64(denominator)
	if first >= 0 && last >= 0 {
		return sum, true, nil
	}
	if first <= 0 && last <= 0 {
		return -sum, true, nil
	}
	return 0, false, nil
}

func qNumericUnaryDyadicFloorCeilSumI64RangeScalar(unaryOp string, dyadicOp Op, values i64RangeArray, scalar float64, scalarLeft bool) (int64, bool, error) {
	a, b, denominator, ok, err := rangeScalarLinearRational(values, dyadicOp, scalar, scalarLeft)
	if err != nil || !ok {
		return 0, ok, err
	}
	if unaryOp == NumericUnaryCeiling {
		sum, ok := floorSumArithmetic(values.len, -a, -b, denominator)
		return -sum, ok, nil
	}
	sum, ok := floorSumArithmetic(values.len, a, b, denominator)
	return sum, ok, nil
}

func rangeScalarLinearRational(values i64RangeArray, op Op, scalar float64, scalarLeft bool) (int64, int64, int64, bool, error) {
	num, den, ok := rationalScalarValue(scalar)
	if !ok {
		return 0, 0, 0, false, nil
	}
	a := values.start * den
	b := values.step * den
	switch op {
	case OpAdd:
		return a + num, b, den, true, nil
	case OpSub:
		if scalarLeft {
			return num - a, -b, den, true, nil
		}
		return a - num, b, den, true, nil
	case OpMul:
		return values.start * num, values.step * num, den, true, nil
	case OpDiv:
		if scalarLeft {
			return 0, 0, 0, false, nil
		}
		if num == 0 {
			return 0, 0, 0, true, fmt.Errorf("divide by zero")
		}
		linearDen := num
		linearA := values.start * den
		linearB := values.step * den
		if linearDen < 0 {
			linearA = -linearA
			linearB = -linearB
			linearDen = -linearDen
		}
		return linearA, linearB, linearDen, true, nil
	default:
		return 0, 0, 0, false, nil
	}
}

func rationalScalarValue(value float64) (int64, int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, 0, false
	}
	for _, den := range []int64{1, 2, 4, 5, 8, 10, 16, 20, 25, 32, 40, 50, 64, 100, 125, 128, 200, 250, 256, 500, 512, 1000, 1024} {
		scaled := value * float64(den)
		num := math.Round(scaled)
		if num < -9223372036854775808.0 || num > 9223372036854775807.0 {
			return 0, 0, false
		}
		if scaled == num {
			return int64(num), den, true
		}
	}
	return 0, 0, false
}

func linearRationalRangeSum(n int, constant, step, denominator int64) float64 {
	if n <= 0 {
		return 0
	}
	count := int64(n)
	numerator := constant*count + step*count*(count-1)/2
	return float64(numerator) / float64(denominator)
}

func floorSumArithmetic(n int, constant, step, m int64) (int64, bool) {
	if n <= 0 {
		return 0, true
	}
	if m <= 0 {
		return 0, false
	}
	count := int64(n)
	var total int64
	if step < 0 || step >= m {
		q := floorDivInt64(step, m)
		total += q * count * (count - 1) / 2
		step -= q * m
	}
	if constant < 0 || constant >= m {
		q := floorDivInt64(constant, m)
		total += q * count
		constant -= q * m
	}
	return total + floorSumNonNegative(count, m, step, constant), true
}

func floorSumNonNegative(n, m, a, b int64) int64 {
	var total int64
	for {
		if a >= m {
			total += (n - 1) * n * (a / m) / 2
			a %= m
		}
		if b >= m {
			total += n * (b / m)
			b %= m
		}
		yMax := a*n + b
		if yMax < m {
			return total
		}
		n = yMax / m
		b = yMax % m
		m, a = a, m
	}
}

func floorDivInt64(value, divisor int64) int64 {
	quotient := value / divisor
	remainder := value % divisor
	if remainder != 0 && ((remainder < 0) != (divisor < 0)) {
		quotient--
	}
	return quotient
}

func numericRangeScalarValueAt(op Op, values i64RangeArray, scalar float64, scalarLeft bool, row int) (float64, error) {
	value := float64(values.start + int64(row)*values.step)
	switch op {
	case OpAdd:
		return value + scalar, nil
	case OpSub:
		if scalarLeft {
			return scalar - value, nil
		}
		return value - scalar, nil
	case OpMul:
		return value * scalar, nil
	case OpDiv:
		if scalarLeft {
			return scalar / value, nil
		}
		return value / scalar, nil
	default:
		return 0, fmt.Errorf("unsupported numeric dyadic kernel %s", op)
	}
}

func numericScalarValue(value any) (float64, bool) {
	if _, ok := value.(Array); ok {
		return 0, false
	}
	if IsNull(value) {
		return 0, false
	}
	return numeric(value)
}

func numericDyadicValueAt(op Op, left, right any, row int) (float64, bool, error) {
	lv, ok, err := numericOperandAt(left, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	rv, ok, err := numericOperandAt(right, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	out, err := applyNumericBinaryFloat(op, lv, rv)
	if err != nil {
		return 0, true, err
	}
	return out, true, nil
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

func numericSumIntegerValue[T signedScalar](values []T) int64 {
	var sum int64
	for _, v := range values {
		sum += int64(v)
	}
	return sum
}

func numericSumIntegerArray(array Array) int64 {
	var sum int64
	for row := 0; row < array.Len(); row++ {
		value, ok, err := integerArrayAt(array, row)
		if err != nil || !ok {
			return 0
		}
		sum += value
	}
	return sum
}

func i64SparseAmendSum(array i64SparseAmendArray) (int64, bool, error) {
	base, handled, err := typedKernels.NumericSumValue(array.source)
	if err != nil || !handled {
		return 0, handled, err
	}
	total, ok := base.(int64)
	if !ok {
		return 0, false, nil
	}
	for i, index := range array.indexes {
		old, ok, err := integerArrayAt(array.source, index)
		if err != nil || !ok {
			return 0, ok, err
		}
		total += array.values[i] - old
	}
	return total, true, nil
}

func numericSumUnsignedValue[T unsignedScalar](values []T) int64 {
	var sum int64
	for _, v := range values {
		sum += int64(v)
	}
	return sum
}

func numericSumFloatValue[T floatScalar](values []T) float64 {
	var sum float64
	for _, v := range values {
		sum += float64(v)
	}
	return sum
}

func numericProductIntegerValue[T signedScalar](values []T) int64 {
	var product int64 = 1
	for _, v := range values {
		product *= int64(v)
	}
	return product
}

func numericProductUnsignedValue[T unsignedScalar](values []T) int64 {
	var product int64 = 1
	for _, v := range values {
		product *= int64(v)
	}
	return product
}

func numericProductFloatValue[T floatScalar](values []T) float64 {
	product := float64(1)
	for _, v := range values {
		product *= float64(v)
	}
	return product
}

func numericProductIntegerArray(array Array) int64 {
	var product int64 = 1
	for i := 0; i < array.Len(); i++ {
		value, ok, err := integerArrayAt(array, i)
		if err != nil || !ok {
			return 0
		}
		product *= value
	}
	return product
}

func numericProductTiledInteger(array tiledArray) (int64, bool) {
	if array.len == 0 {
		return 1, true
	}
	allOne := true
	for row := 0; row < array.source.Len(); row++ {
		value, ok, err := integerArrayAt(array.source, row)
		if err != nil || !ok {
			return 0, false
		}
		if value == 0 {
			return 0, true
		}
		if value != 1 {
			allOne = false
		}
	}
	if allOne {
		return 1, true
	}
	return 0, false
}

func i64RangeSum(values i64RangeArray) int64 {
	if values.len == 0 {
		return 0
	}
	n := int64(values.len)
	last := values.start + int64(values.len-1)*values.step
	endpoints := values.start + last
	if n%2 == 0 {
		return (n / 2) * endpoints
	}
	return n * (endpoints / 2)
}

func f64RangeSum(values f64RangeArray) float64 {
	if values.len == 0 {
		return 0
	}
	n := float64(values.len)
	last := values.start + float64(values.len-1)*values.step
	return n * (values.start + last) / 2
}

func i64RunningSumSum(values i64RunningSumArray) int64 {
	var sum int64
	for i := 0; i < values.Len(); i++ {
		value, _ := values.i64At(i)
		sum += value
	}
	return sum
}

func f64RunningSumSum(values f64RunningSumArray) float64 {
	var sum float64
	for i := 0; i < values.Len(); i++ {
		value, _ := values.f64At(i)
		sum += value
	}
	return sum
}

func i64SegmentSum(values i64SegmentArray) int64 {
	var sum int64
	for _, segment := range values.segments {
		sum += i64RangeSum(segment)
	}
	return sum
}

func i64PeriodicIndexSum(values i64PeriodicIndexArray) int64 {
	if values.len == 0 || values.period <= 0 || len(values.residues) == 0 {
		return 0
	}
	var residueSum int64
	for _, residue := range values.residues {
		residueSum += residue
	}
	residueCount := int64(len(values.residues))
	full := values.fullCycles
	sum := full*residueSum + values.period*residueCount*full*(full-1)/2
	base := full * values.period
	for _, residue := range values.tailResidues {
		sum += base + residue
	}
	return sum
}

func i64ProductSum(values i64ProductArray) int64 {
	if sum, ok := i64ProductRangeSum(values); ok {
		return sum
	}
	var sum int64
	for i := 0; i < values.Len(); i++ {
		value, _ := values.i64At(i)
		sum += value
	}
	return sum
}

func i64ProductRangeSum(values i64ProductArray) (int64, bool) {
	n := int64(values.Len())
	if n == 0 {
		return 0, true
	}
	sumI, ok := i64IndexSumChecked(n)
	if !ok {
		return 0, false
	}
	sumI2, ok := i64IndexSquareSumChecked(n)
	if !ok {
		return 0, false
	}
	constant, ok := i64MulChecked(values.left.start, values.right.start)
	if !ok {
		return 0, false
	}
	constant, ok = i64MulChecked(constant, n)
	if !ok {
		return 0, false
	}
	leftLinear, ok := i64MulChecked(values.left.start, values.right.step)
	if !ok {
		return 0, false
	}
	rightLinear, ok := i64MulChecked(values.right.start, values.left.step)
	if !ok {
		return 0, false
	}
	linear, ok := i64AddChecked(leftLinear, rightLinear)
	if !ok {
		return 0, false
	}
	linear, ok = i64MulChecked(linear, sumI)
	if !ok {
		return 0, false
	}
	quadratic, ok := i64MulChecked(values.left.step, values.right.step)
	if !ok {
		return 0, false
	}
	quadratic, ok = i64MulChecked(quadratic, sumI2)
	if !ok {
		return 0, false
	}
	total, ok := i64AddChecked(constant, linear)
	if !ok {
		return 0, false
	}
	total, ok = i64AddChecked(total, quadratic)
	if !ok {
		return 0, false
	}
	return total, true
}

func i64IndexSumChecked(n int64) (int64, bool) {
	if n <= 0 {
		return 0, true
	}
	if n%2 == 0 {
		return i64MulChecked(n/2, n-1)
	}
	return i64MulChecked(n, (n-1)/2)
}

func i64IndexSquareSumChecked(n int64) (int64, bool) {
	if n <= 0 {
		return 0, true
	}
	if n > int64(^uint64(0)>>1)/2+1 {
		return 0, false
	}
	factors := [3]int64{n, n - 1, 2*n - 1}
	divideFactor := func(divisor int64) bool {
		for i := range factors {
			if factors[i]%divisor == 0 {
				factors[i] /= divisor
				return true
			}
		}
		return false
	}
	if !divideFactor(2) || !divideFactor(3) {
		return 0, false
	}
	product, ok := i64MulChecked(factors[0], factors[1])
	if !ok {
		return 0, false
	}
	return i64MulChecked(product, factors[2])
}

func i64AddChecked(left, right int64) (int64, bool) {
	out := left + right
	if (right > 0 && out < left) || (right < 0 && out > left) {
		return 0, false
	}
	return out, true
}

func i64MulChecked(left, right int64) (int64, bool) {
	if left == 0 || right == 0 {
		return 0, true
	}
	minInt := -int64(^uint64(0)>>1) - 1
	if (left == minInt && right == -1) || (right == minInt && left == -1) {
		return 0, false
	}
	out := left * right
	if out/right != left {
		return 0, false
	}
	return out, true
}

func numericSumsInteger[T signedScalar](values []T) Array {
	out := make([]int64, len(values))
	var sum int64
	for i, v := range values {
		sum += int64(v)
		out[i] = sum
	}
	return newI64Trusted(out)
}

func numericSumsUnsigned[T unsignedScalar](values []T) Array {
	out := make([]int64, len(values))
	var sum int64
	for i, v := range values {
		sum += int64(v)
		out[i] = sum
	}
	return newI64Trusted(out)
}

func numericSumsFloat[T floatScalar](values []T) Array {
	out := make([]float64, len(values))
	var sum float64
	for i, v := range values {
		sum += float64(v)
		out[i] = sum
	}
	return newF64Trusted(out)
}

func numericSumsI64Range(values i64RangeArray) Array {
	out := make([]int64, values.len)
	var sum int64
	for i := range out {
		sum += values.start + int64(i)*values.step
		out[i] = sum
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func numericProductsInteger[T signedScalar](values []T) Array {
	out := make([]int64, len(values))
	var product int64 = 1
	for i, value := range values {
		product *= int64(value)
		out[i] = product
	}
	return newI64Trusted(out)
}

func numericProductsUnsigned[T unsignedScalar](values []T) Array {
	out := make([]int64, len(values))
	var product int64 = 1
	for i, value := range values {
		product *= int64(value)
		out[i] = product
	}
	return newI64Trusted(out)
}

func numericProductsFloat[T floatScalar](values []T) Array {
	out := make([]float64, len(values))
	product := float64(1)
	for i, value := range values {
		product *= float64(value)
		out[i] = product
	}
	return newF64Trusted(out)
}

func numericProductsIntegerArray(array Array) Array {
	out := make([]int64, array.Len())
	var product int64 = 1
	for i := range out {
		value, ok, err := integerArrayAt(array, i)
		if err != nil || !ok {
			return nil
		}
		product *= value
		out[i] = product
	}
	return newI64Trusted(out)
}

func numericProductsTiledInteger(array tiledArray) (Array, bool) {
	if array.len == 0 {
		return i64RangeArray{len: 0}, true
	}
	allOne := true
	for row := 0; row < array.source.Len(); row++ {
		value, ok, err := integerArrayAt(array.source, row)
		if err != nil || !ok {
			return nil, false
		}
		if value == 0 {
			out := make([]int64, array.len)
			product := int64(1)
			for i := range out {
				item, ok, err := integerArrayAt(array, i)
				if err != nil || !ok {
					return nil, false
				}
				product *= item
				out[i] = product
				if product == 0 {
					break
				}
			}
			return newI64Trusted(out), true
		}
		if value != 1 {
			allOne = false
		}
	}
	if allOne {
		return i64RangeArray{start: 1, step: 0, len: array.len}, true
	}
	return nil, false
}

func deltasI64Slice(values []int64) Array {
	out := make([]int64, len(values))
	for i, value := range values {
		if i == 0 {
			out[i] = value
			continue
		}
		out[i] = value - values[i-1]
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func deltasI64Range(values i64RangeArray) Array {
	out := make([]int64, values.len)
	if values.len == 0 {
		return columnArray[int64]{kind: KindI64, data: out}
	}
	out[0] = values.start
	for i := 1; i < values.len; i++ {
		out[i] = values.step
	}
	return columnArray[int64]{kind: KindI64, data: out}
}

func lastSignedDeltasSum[T signedScalar](values []T) any {
	if len(values) == 0 {
		return NullValue
	}
	return int64(values[len(values)-1])
}

func lastUnsignedDeltasSum[T unsignedScalar](values []T) any {
	if len(values) == 0 {
		return NullValue
	}
	return int64(values[len(values)-1])
}

func lastFloatDeltasSum[T floatScalar](values []T) any {
	if len(values) == 0 {
		return NullValue
	}
	return float64(values[len(values)-1])
}

func numericSumRowsSlice[T signedScalar | unsignedScalar | floatScalar](values []T, rows []int) (float64, int64, bool, error) {
	var sum float64
	for _, row := range rows {
		if row < 0 || row >= len(values) {
			return 0, 0, true, fmt.Errorf("sum row %d out of range", row)
		}
		sum += float64(values[row])
	}
	return sum, int64(len(rows)), true, nil
}

func numericSumRowsI64Range(values i64RangeArray, rows []int) (float64, int64, bool, error) {
	var sum int64
	for _, row := range rows {
		if row < 0 || row >= values.len {
			return 0, 0, true, fmt.Errorf("sum row %d out of range", row)
		}
		sum += values.start + int64(row)*values.step
	}
	return float64(sum), int64(len(rows)), true, nil
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
	case i64RangeArray:
		return minMaxI64Range(a, mode)
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

func minMaxI64Range(values i64RangeArray, mode string) (any, bool, bool, error) {
	if values.len == 0 {
		return nil, false, true, nil
	}
	first := values.start
	last := values.start + int64(values.len-1)*values.step
	if mode == "min" {
		if first <= last {
			return first, true, true, nil
		}
		return last, true, true, nil
	}
	if first >= last {
		return first, true, true, nil
	}
	return last, true, true, nil
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

func compareI64RangeMask(values i64RangeArray, target int64, ok bool, op Op, out []bool) bool {
	if !ok || len(out) < values.len {
		return false
	}
	for i := 0; i < values.len; i++ {
		v := values.start + int64(i)*values.step
		out[i] = boolCompare(op, v == target, compareInt64(v, target))
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

func compareI64RangeIndexes(values i64RangeArray, target int64, ok bool, op Op, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for i := 0; i < values.len; i++ {
		v := values.start + int64(i)*values.step
		if boolCompare(op, v == target, compareInt64(v, target)) {
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

func encodedInIndexes(array encodedArray, values []any, out []int) ([]int, bool) {
	codes := make(map[int32]struct{}, len(values))
	for _, value := range values {
		code, ok := encodedComparableCode(array, value)
		if !ok {
			return nil, false
		}
		codes[code] = struct{}{}
	}
	out = out[:0]
	for row, code := range array.codes {
		if _, ok := codes[code]; ok {
			out = append(out, row)
		}
	}
	return out, true
}

func boolMembership(values []any) (map[bool]struct{}, bool) {
	set := make(map[bool]struct{}, len(values))
	for _, value := range values {
		v, ok := value.(bool)
		if !ok {
			return nil, false
		}
		set[v] = struct{}{}
	}
	return set, true
}

func exactMembership[T comparable](values []any) (map[T]struct{}, bool) {
	set := make(map[T]struct{}, len(values))
	for _, value := range values {
		v, ok := value.(T)
		if !ok {
			return nil, false
		}
		set[v] = struct{}{}
	}
	return set, true
}

func int64Membership(values []any) (map[int64]struct{}, bool) {
	set := make(map[int64]struct{}, len(values))
	for _, value := range values {
		v, ok := coerceInt64Exact(value)
		if !ok {
			return nil, false
		}
		set[v] = struct{}{}
	}
	return set, true
}

func signedMembership[T signedScalar](values []any) (map[T]struct{}, bool) {
	set := make(map[T]struct{}, len(values))
	for _, value := range values {
		v, ok := value.(T)
		if !ok {
			return nil, false
		}
		set[v] = struct{}{}
	}
	return set, true
}

func unsignedMembership[T unsignedScalar](values []any) (map[T]struct{}, bool) {
	set := make(map[T]struct{}, len(values))
	for _, value := range values {
		v, ok := value.(T)
		if !ok {
			return nil, false
		}
		set[v] = struct{}{}
	}
	return set, true
}

func stringMembership(values []any) (map[string]struct{}, bool) {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		v, ok := coerceComparableString(value)
		if !ok {
			return nil, false
		}
		set[v] = struct{}{}
	}
	return set, true
}

func symbolMembership(values []any) (map[Symbol]struct{}, bool) {
	set := make(map[Symbol]struct{}, len(values))
	for _, value := range values {
		v, ok := coerceComparableSymbol(value)
		if !ok {
			return nil, false
		}
		set[v] = struct{}{}
	}
	return set, true
}

func typedInPredicate(kind Kind, values []any) (func(any) (bool, bool), bool) {
	values = normalizeMembershipValues(kind, values)
	switch kind {
	case KindBool:
		set, ok := boolMembership(values)
		return func(value any) (bool, bool) {
			v, ok := value.(bool)
			if !ok {
				return false, false
			}
			_, matched := set[v]
			return matched, true
		}, ok
	case KindI8:
		set, ok := signedMembership[int8](values)
		return typedInSignedPredicate[int8](set), ok
	case KindI16:
		set, ok := signedMembership[int16](values)
		return typedInSignedPredicate[int16](set), ok
	case KindI32:
		set, ok := signedMembership[int32](values)
		return typedInSignedPredicate[int32](set), ok
	case KindI64:
		set, ok := int64Membership(values)
		return func(value any) (bool, bool) {
			v, ok := coerceInt64Exact(value)
			if !ok {
				return false, false
			}
			_, matched := set[v]
			return matched, true
		}, ok
	case KindU8:
		set, ok := unsignedMembership[uint8](values)
		return typedInUnsignedPredicate[uint8](set), ok
	case KindU16:
		set, ok := unsignedMembership[uint16](values)
		return typedInUnsignedPredicate[uint16](set), ok
	case KindU32:
		set, ok := unsignedMembership[uint32](values)
		return typedInUnsignedPredicate[uint32](set), ok
	case KindU64:
		set, ok := unsignedMembership[uint64](values)
		return typedInUnsignedPredicate[uint64](set), ok
	case KindString:
		set, ok := stringMembership(values)
		return func(value any) (bool, bool) {
			v, ok := coerceComparableString(value)
			if !ok {
				return false, false
			}
			_, matched := set[v]
			return matched, true
		}, ok
	case KindSymbol:
		set, ok := symbolMembership(values)
		return func(value any) (bool, bool) {
			v, ok := coerceComparableSymbol(value)
			if !ok {
				return false, false
			}
			_, matched := set[v]
			return matched, true
		}, ok
	case KindMonth:
		set, ok := exactMembership[Month](values)
		return typedInSignedPredicate[Month](set), ok
	case KindDate:
		set, ok := exactMembership[Date](values)
		return typedInSignedPredicate[Date](set), ok
	case KindDateTime:
		set, ok := exactMembership[DateTime](values)
		return typedInSignedPredicate[DateTime](set), ok
	case KindTimespan:
		set, ok := exactMembership[Timespan](values)
		return typedInSignedPredicate[Timespan](set), ok
	case KindMinute:
		set, ok := exactMembership[Minute](values)
		return typedInSignedPredicate[Minute](set), ok
	case KindSecond:
		set, ok := exactMembership[Second](values)
		return typedInSignedPredicate[Second](set), ok
	case KindTime:
		set, ok := exactMembership[Time](values)
		return typedInSignedPredicate[Time](set), ok
	case KindTimestamp:
		set, ok := exactMembership[Timestamp](values)
		return typedInSignedPredicate[Timestamp](set), ok
	default:
		return nil, false
	}
}

func typedInSignedPredicate[T signedScalar](set map[T]struct{}) func(any) (bool, bool) {
	return func(value any) (bool, bool) {
		v, ok := value.(T)
		if !ok {
			return false, false
		}
		_, matched := set[v]
		return matched, true
	}
}

func typedInUnsignedPredicate[T unsignedScalar](set map[T]struct{}) func(any) (bool, bool) {
	return func(value any) (bool, bool) {
		v, ok := value.(T)
		if !ok {
			return false, false
		}
		_, matched := set[v]
		return matched, true
	}
}

type boolLogicalMask struct {
	op            string
	left          Array
	leftScalar    bool
	leftIsScalar  bool
	right         Array
	rightScalar   bool
	rightIsScalar bool
	len           int
}

type notMask struct {
	array Array
}

type i64ScalarDyadicArray struct {
	source     Array
	op         Op
	scalar     int64
	scalarLeft bool
	len        int
}

type i64ScalarDyadicRunningSumArray struct {
	source i64ScalarDyadicArray
}

func (a i64ScalarDyadicArray) Kind() Kind { return KindI64 }

func (a i64ScalarDyadicArray) Len() int { return a.len }

func (a i64ScalarDyadicArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64ScalarDyadicArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data scalar dyadic row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64ScalarDyadicArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data scalar dyadic gather row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64ScalarDyadicArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	value, ok, err := integerArrayAt(a.source, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	return applyI64ScalarDyadicValue(a.op, value, a.scalar, a.scalarLeft)
}

func (a i64ScalarDyadicRunningSumArray) Kind() Kind { return KindI64 }

func (a i64ScalarDyadicRunningSumArray) Len() int { return a.source.len }

func (a i64ScalarDyadicRunningSumArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64ScalarDyadicRunningSumArray) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data scalar dyadic running sum row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64ScalarDyadicRunningSumArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data scalar dyadic running sum gather row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64ScalarDyadicRunningSumArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.source.len {
		return 0, false, fmt.Errorf("running sum row %d out of range", row)
	}
	return i64ScalarDyadicRangeSumI64(a.source, 0, row+1)
}

func i64ScalarDyadicRunningSumSum(array i64ScalarDyadicRunningSumArray) (int64, bool, error) {
	var total int64
	for row := 0; row < array.Len(); row++ {
		value, ok, err := array.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		total += value
	}
	return total, true, nil
}

func i64ScalarDyadicRangeSum(array i64ScalarDyadicArray, start, length int) (any, bool, error) {
	if length == 0 {
		return NullValue, true, nil
	}
	if out, handled, err := i64ScalarDyadicRangeSumI64(array, start, length); handled || err != nil {
		return out, handled, err
	}
	return nil, false, nil
}

func i64ScalarDyadicRangeSumI64(array i64ScalarDyadicArray, start, length int) (int64, bool, error) {
	if length == 0 {
		return 0, true, nil
	}
	if start == 0 && length == array.len {
		if out, handled := i64ScalarDyadicWholeRangeSum(array); handled {
			return out, true, nil
		}
	}
	if out, handled := i64ScalarDyadicModuloRangeSum(array, start, length); handled {
		return out, true, nil
	}
	sourceRows := i64RangeArray{start: int64(start), step: 1, len: length}
	sourceSumValue, handled, err := typedIntegerSumContiguousRange(array.source, sourceRows)
	if err != nil || !handled {
		return 0, handled, err
	}
	sourceSum, ok := sourceSumValue.(int64)
	if !ok {
		return 0, false, nil
	}
	n := int64(length)
	switch array.op {
	case OpAdd:
		if array.scalarLeft {
			return array.scalar*n + sourceSum, true, nil
		}
		return sourceSum + array.scalar*n, true, nil
	case OpSub:
		if array.scalarLeft {
			return array.scalar*n - sourceSum, true, nil
		}
		return sourceSum - array.scalar*n, true, nil
	case OpMul:
		return sourceSum * array.scalar, true, nil
	default:
		return 0, false, nil
	}
}

func i64ScalarDyadicSum(array i64ScalarDyadicArray) (any, bool, error) {
	if array.len == 0 {
		return int64(0), true, nil
	}
	if out, handled := i64ScalarDyadicWholeRangeSum(array); handled {
		return out, true, nil
	}
	return i64ScalarDyadicRangeSum(array, 0, array.len)
}

func i64ScalarDyadicWholeRangeSum(array i64ScalarDyadicArray) (int64, bool) {
	if array.op != OpMod || array.scalarLeft || array.scalar <= 0 {
		return 0, false
	}
	rangeSource, ok := array.source.(i64RangeArray)
	if !ok || rangeSource.step != 1 {
		return 0, false
	}
	return i64RangePositiveModSum(rangeSource.start, array.len, array.scalar), true
}

func i64ScalarDyadicModuloRangeSum(array i64ScalarDyadicArray, start, length int) (int64, bool) {
	if array.op != OpMod || array.scalarLeft || array.scalar <= 0 || length < 0 {
		return 0, false
	}
	rangeSource, ok := array.source.(i64RangeArray)
	if !ok || rangeSource.step != 1 {
		return 0, false
	}
	return i64RangePositiveModSum(rangeSource.start+int64(start), length, array.scalar), true
}

func i64RangePositiveModSum(start int64, length int, modulus int64) int64 {
	if length <= 0 {
		return 0
	}
	period := modulus
	if period <= 0 {
		return 0
	}
	periodSum := period * (period - 1) / 2
	first := qPositiveMod(start, modulus)
	n := int64(length)
	if first == 0 {
		full := n / period
		rem := n % period
		return full*periodSum + rem*(rem-1)/2
	}
	firstRun := period - first
	if n <= firstRun {
		return n * (2*first + n - 1) / 2
	}
	sum := firstRun * (first + period - 1) / 2
	remaining := n - firstRun
	full := remaining / period
	rem := remaining % period
	return sum + full*periodSum + rem*(rem-1)/2
}

func qPositiveMod(value, modulus int64) int64 {
	out := value % modulus
	if out < 0 {
		out += modulus
	}
	return out
}

type i64ModuloComparePlan struct {
	startResidue int64
	modulus      int64
	length       int
	op           Op
	scalar       int64
	scalarLeft   bool
}

func i64ScalarDyadicCompareModuloPlan(mask i64ScalarDyadicCompareMask) (i64ModuloComparePlan, bool) {
	values := mask.values
	if values.op != OpMod || values.scalarLeft || values.scalar <= 0 || values.len <= 0 {
		return i64ModuloComparePlan{}, false
	}
	source, ok := values.source.(i64RangeArray)
	if !ok || source.step != 1 {
		return i64ModuloComparePlan{}, false
	}
	return i64ModuloComparePlan{
		startResidue: qPositiveMod(source.start, values.scalar),
		modulus:      values.scalar,
		length:       values.len,
		op:           mask.op,
		scalar:       mask.scalar,
		scalarLeft:   mask.scalarLeft,
	}, true
}

func i64ModuloComparePlanForArray(array Array, modulus int64, op Op, target int64) (i64ModuloComparePlan, bool) {
	if modulus <= 0 {
		return i64ModuloComparePlan{}, false
	}
	switch a := array.(type) {
	case attributedArray:
		return i64ModuloComparePlanForArray(a.array, modulus, op, target)
	case i64RangeArray:
		if a.step != 1 || a.len <= 0 {
			return i64ModuloComparePlan{}, false
		}
		return i64ModuloComparePlan{
			startResidue: qPositiveMod(a.start, modulus),
			modulus:      modulus,
			length:       a.len,
			op:           op,
			scalar:       target,
		}, true
	case i64ScalarDyadicArray:
		if a.op != OpMod || a.scalarLeft || a.scalar != modulus || a.len <= 0 {
			return i64ModuloComparePlan{}, false
		}
		source, ok := a.source.(i64RangeArray)
		if !ok || source.step != 1 {
			return i64ModuloComparePlan{}, false
		}
		return i64ModuloComparePlan{
			startResidue: qPositiveMod(source.start, modulus),
			modulus:      modulus,
			length:       a.len,
			op:           op,
			scalar:       target,
		}, true
	default:
		return i64ModuloComparePlan{}, false
	}
}

func (p i64ModuloComparePlan) trueCount() (int64, bool) {
	if p.length <= 0 {
		return 0, true
	}
	if p.op == OpEQ || p.op == OpNE {
		eq := p.countEqualResidue(p.scalar)
		if p.op == OpEQ {
			return eq, true
		}
		return int64(p.length) - eq, true
	}
	low, high, ok := p.residueInterval()
	if !ok {
		return 0, false
	}
	return p.countResidueInterval(low, high), true
}

func (p i64ModuloComparePlan) indexSum() int64 {
	if p.length <= 0 {
		return 0
	}
	switch p.op {
	case OpEQ:
		return p.indexSumEqualResidue(p.scalar)
	case OpNE:
		return i64RangeSum(i64RangeArray{start: 0, step: 1, len: p.length}) - p.indexSumEqualResidue(p.scalar)
	default:
		indexes := i64ModuloComparePlanIndexArray(p)
		return i64IndexArraySum(indexes)
	}
}

func (p i64ModuloComparePlan) indexSumEqualResidue(target int64) int64 {
	if target < 0 || target >= p.modulus || p.length <= 0 {
		return 0
	}
	first := qPositiveMod(target-p.startResidue, p.modulus)
	if first >= int64(p.length) {
		return 0
	}
	count := int((int64(p.length)-first-1)/p.modulus) + 1
	return i64RangeSum(i64RangeArray{start: first, step: p.modulus, len: count})
}

func (p i64ModuloComparePlan) countEqualResidue(target int64) int64 {
	if target < 0 || target >= p.modulus || p.length <= 0 {
		return 0
	}
	first := qPositiveMod(target-p.startResidue, p.modulus)
	if first >= int64(p.length) {
		return 0
	}
	return 1 + (int64(p.length)-first-1)/p.modulus
}

func (p i64ModuloComparePlan) countResidueInterval(low, high int64) int64 {
	if low < 0 {
		low = 0
	}
	if high >= p.modulus {
		high = p.modulus - 1
	}
	if low > high || p.length <= 0 {
		return 0
	}
	periodCount := high - low + 1
	n := int64(p.length)
	full := n / p.modulus
	rem := n % p.modulus
	count := full * periodCount
	for row := int64(0); row < rem; row++ {
		residue := (p.startResidue + row) % p.modulus
		if residue >= low && residue <= high {
			count++
		}
	}
	return count
}

func (p i64ModuloComparePlan) residueInterval() (int64, int64, bool) {
	op := p.op
	if p.scalarLeft {
		switch op {
		case OpLT:
			op = OpGT
		case OpLE:
			op = OpGE
		case OpGT:
			op = OpLT
		case OpGE:
			op = OpLE
		}
	}
	switch op {
	case OpGT:
		return p.scalar + 1, p.modulus - 1, true
	case OpGE:
		return p.scalar, p.modulus - 1, true
	case OpLT:
		return 0, p.scalar - 1, true
	case OpLE:
		return 0, p.scalar, true
	default:
		return 0, 0, false
	}
}

func (p i64ModuloComparePlan) valueAtRow(row int) bool {
	residue := (p.startResidue + int64(row)) % p.modulus
	if p.scalarLeft {
		return boolCompare(p.op, p.scalar == residue, compareInt64(p.scalar, residue))
	}
	return boolCompare(p.op, residue == p.scalar, compareInt64(residue, p.scalar))
}

func (a notMask) Kind() Kind { return KindBool }

func (a notMask) Len() int { return a.array.Len() }

func (a notMask) At(row int) (any, bool) {
	if row < 0 || row >= a.Len() {
		return nil, false
	}
	value, ok, err := a.valueAt(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a notMask) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data not mask row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a notMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.Len() {
			panic(fmt.Sprintf("data not mask gather index %d out of range", row))
		}
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data not mask row %d out of range", row))
		}
		out[i] = value
	}
	return newBoolTrusted(out)
}

func (a notMask) valueAt(row int) (bool, bool, error) {
	if a.array.Kind() == KindBool {
		value, ok, err := boolArrayAt(a.array, row)
		if err != nil || !ok {
			return false, ok, err
		}
		return !value, true, nil
	}
	value, ok, err := numericAt(a.array, row)
	if err != nil {
		return false, true, err
	}
	if !ok {
		return true, true, nil
	}
	return value == 0, true, nil
}

func (a notMask) trueCount() (int64, bool, error) {
	var count int64
	for row := 0; row < a.Len(); row++ {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		if value {
			count++
		}
	}
	return count, true, nil
}

func (a boolLogicalMask) Kind() Kind { return KindBool }

func (a boolLogicalMask) Len() int { return a.len }

func (a boolLogicalMask) At(row int) (any, bool) {
	if row < 0 || row >= a.len {
		return nil, false
	}
	value, ok, err := a.valueAt(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a boolLogicalMask) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data logical mask row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a boolLogicalMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data logical mask gather index %d out of range", row))
		}
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data logical mask row %d out of range", row))
		}
		out[i] = value
	}
	return newBoolTrusted(out)
}

func (a boolLogicalMask) valueAt(row int) (bool, bool, error) {
	left := a.leftScalar
	if !a.leftIsScalar {
		value, ok, err := boolArrayAt(a.left, row%a.left.Len())
		if err != nil || !ok {
			return false, ok, err
		}
		left = value
	}
	right := a.rightScalar
	if !a.rightIsScalar {
		value, ok, err := boolArrayAt(a.right, row%a.right.Len())
		if err != nil || !ok {
			return false, ok, err
		}
		right = value
	}
	return applyBoolLogical(a.op, left, right), true, nil
}

func (a boolLogicalMask) trueCount() (int64, bool, error) {
	if count, ok := a.rangeCompareTrueCount(); ok {
		return count, true, nil
	}
	if count, ok := a.moduloCompareTrueCount(); ok {
		return count, true, nil
	}
	var count int64
	for row := 0; row < a.len; row++ {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		if value {
			count++
		}
	}
	return count, true, nil
}

func (a boolLogicalMask) rangeCompareTrueCount() (int64, bool) {
	if a.leftIsScalar || a.rightIsScalar {
		return 0, false
	}
	left, leftOK := a.left.(i64RangeCompareMask)
	right, rightOK := a.right.(i64RangeCompareMask)
	if !leftOK || !rightOK || !sameI64Range(left.values, right.values) || left.values.step != 1 {
		return 0, false
	}
	leftLow, leftHigh, ok := compareMaskValueInterval(left)
	if !ok {
		return 0, false
	}
	rightLow, rightHigh, ok := compareMaskValueInterval(right)
	if !ok {
		return 0, false
	}
	intersect := i64RangeIntervalCount(left.values, maxInt64Value(leftLow, rightLow), minInt64Value(leftHigh, rightHigh))
	if a.op == "and" {
		return intersect, true
	}
	if a.op == "or" {
		leftCount := i64RangeIntervalCount(left.values, leftLow, leftHigh)
		rightCount := i64RangeIntervalCount(right.values, rightLow, rightHigh)
		return leftCount + rightCount - intersect, true
	}
	return 0, false
}

func (a boolLogicalMask) moduloCompareTrueCount() (int64, bool) {
	plan, ok := boolLogicalModuloComparePlan(a)
	if !ok {
		return 0, false
	}
	return plan.trueCount()
}

type boolLogicalModuloPlan struct {
	left   i64ModuloComparePlan
	right  i64ModuloComparePlan
	op     string
	period int64
	length int
}

func boolLogicalModuloComparePlan(mask boolLogicalMask) (boolLogicalModuloPlan, bool) {
	if mask.leftIsScalar || mask.rightIsScalar {
		return boolLogicalModuloPlan{}, false
	}
	leftMask, leftOK := mask.left.(i64ScalarDyadicCompareMask)
	rightMask, rightOK := mask.right.(i64ScalarDyadicCompareMask)
	if !leftOK || !rightOK || mask.len <= 0 {
		return boolLogicalModuloPlan{}, false
	}
	left, leftOK := i64ScalarDyadicCompareModuloPlan(leftMask)
	right, rightOK := i64ScalarDyadicCompareModuloPlan(rightMask)
	if !leftOK || !rightOK || left.length != right.length || left.length != mask.len {
		return boolLogicalModuloPlan{}, false
	}
	period, ok := lcmInt64(left.modulus, right.modulus)
	if !ok || period > 65536 {
		return boolLogicalModuloPlan{}, false
	}
	return boolLogicalModuloPlan{left: left, right: right, op: mask.op, period: period, length: mask.len}, true
}

func (p boolLogicalModuloPlan) trueCount() (int64, bool) {
	if p.length <= 0 {
		return 0, true
	}
	var periodCount int64
	for offset := int64(0); offset < p.period; offset++ {
		row := int(offset)
		left := p.left.valueAtRow(row)
		right := p.right.valueAtRow(row)
		if applyBoolLogical(p.op, left, right) {
			periodCount++
		}
	}
	n := int64(p.length)
	full := n / p.period
	rem := n % p.period
	count := full * periodCount
	for row := int64(0); row < rem; row++ {
		left := p.left.valueAtRow(int(row))
		right := p.right.valueAtRow(int(row))
		if applyBoolLogical(p.op, left, right) {
			count++
		}
	}
	return count, true
}

func lcmInt64(left, right int64) (int64, bool) {
	if left <= 0 || right <= 0 {
		return 0, false
	}
	g := gcdInt64(left, right)
	div := left / g
	if div > math.MaxInt64/right {
		return 0, false
	}
	return div * right, true
}

func gcdInt64(left, right int64) int64 {
	for right != 0 {
		left, right = right, left%right
	}
	if left < 0 {
		return -left
	}
	return left
}

func sameI64Range(left, right i64RangeArray) bool {
	return left.start == right.start && left.step == right.step && left.len == right.len
}

func compareMaskValueInterval(mask i64RangeCompareMask) (int64, int64, bool) {
	op := mask.op
	if mask.scalarLeft {
		switch op {
		case OpLT:
			op = OpGT
		case OpLE:
			op = OpGE
		case OpGT:
			op = OpLT
		case OpGE:
			op = OpLE
		}
	}
	minInt := -int64(^uint64(0)>>1) - 1
	maxInt := int64(^uint64(0) >> 1)
	switch op {
	case OpEQ:
		return mask.scalar, mask.scalar, true
	case OpGT:
		if mask.scalar == maxInt {
			return 1, 0, true
		}
		return mask.scalar + 1, maxInt, true
	case OpGE:
		return mask.scalar, maxInt, true
	case OpLT:
		if mask.scalar == minInt {
			return 1, 0, true
		}
		return minInt, mask.scalar - 1, true
	case OpLE:
		return minInt, mask.scalar, true
	default:
		return 0, 0, false
	}
}

func i64RangeIntervalCount(values i64RangeArray, low, high int64) int64 {
	if values.len <= 0 || low > high {
		return 0
	}
	start := values.start
	last := values.start + int64(values.len-1)
	if high < start || low > last {
		return 0
	}
	firstRow := int64(0)
	if low > start {
		firstRow = low - start
	}
	lastRow := int64(values.len - 1)
	if high < last {
		lastRow = high - start
	}
	if lastRow < firstRow {
		return 0
	}
	return lastRow - firstRow + 1
}

func maxInt64Value(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func minInt64Value(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func boolLogicalArrayArray(op string, left, right Array, out []bool) (Array, bool, error) {
	for row := range out {
		lv, ok, err := boolArrayAt(left, row%left.Len())
		if err != nil || !ok {
			return nil, ok, err
		}
		rv, ok, err := boolArrayAt(right, row%right.Len())
		if err != nil || !ok {
			return nil, ok, err
		}
		out[row] = applyBoolLogical(op, lv, rv)
	}
	return newBoolTrusted(out), true, nil
}

func boolLogicalArrayScalar(op string, left Array, right bool, out []bool) (Array, bool, error) {
	for row := range out {
		leftValue, ok, err := boolArrayAt(left, row)
		if err != nil || !ok {
			return nil, ok, err
		}
		out[row] = applyBoolLogical(op, leftValue, right)
	}
	return newBoolTrusted(out), true, nil
}

func boolLogicalScalarArray(op string, left bool, right Array, out []bool) (Array, bool, error) {
	for row := range out {
		rightValue, ok, err := boolArrayAt(right, row)
		if err != nil || !ok {
			return nil, ok, err
		}
		out[row] = applyBoolLogical(op, left, rightValue)
	}
	return newBoolTrusted(out), true, nil
}

func boolArrayAt(array Array, row int) (bool, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return boolArrayAt(a.array, row)
	case i64RangeCompareMask:
		if row < 0 || row >= a.Len() {
			return false, true, fmt.Errorf("logical row %d out of range", row)
		}
		return a.valueAt(row), true, nil
	case i64ScalarDyadicCompareMask:
		if row < 0 || row >= a.Len() {
			return false, true, fmt.Errorf("logical row %d out of range", row)
		}
		return a.valueAt(row)
	case boolLogicalMask:
		return a.valueAt(row)
	case notMask:
		return a.valueAt(row)
	case tiledArray:
		sourceLen := a.source.Len()
		if row < 0 || row >= a.len || sourceLen == 0 {
			return false, true, fmt.Errorf("logical row %d out of range", row)
		}
		return boolArrayAt(a.source, (a.start+row)%sourceLen)
	case columnArray[bool]:
		if row < 0 || row >= len(a.data) {
			return false, true, fmt.Errorf("logical row %d out of range", row)
		}
		return a.data[row], true, nil
	case nullableArray:
		if row < 0 || row >= len(a.data) {
			return false, true, fmt.Errorf("logical row %d out of range", row)
		}
		value := a.data[row]
		if IsNull(value) {
			return false, true, nil
		}
		out, ok := value.(bool)
		if !ok {
			return false, true, fmt.Errorf("logical row %d is %T, want bool", row, value)
		}
		return out, true, nil
	default:
		return false, false, nil
	}
}

func boolScalarValue(value any) (bool, bool) {
	if IsNull(value) {
		return false, true
	}
	out, ok := value.(bool)
	return out, ok
}

func applyBoolLogical(op string, left, right bool) bool {
	if op == "or" {
		return left || right
	}
	return left && right
}

func countMembershipBool(values []bool, set map[bool]struct{}, ok bool) int64 {
	if !ok {
		return 0
	}
	var count int64
	for _, value := range values {
		if _, matched := set[value]; matched {
			count++
		}
	}
	return count
}

func countMembershipSigned[T signedScalar](values []T, set map[T]struct{}, ok bool) int64 {
	if !ok {
		return 0
	}
	var count int64
	for _, value := range values {
		if _, matched := set[value]; matched {
			count++
		}
	}
	return count
}

func countMembershipUnsigned[T unsignedScalar](values []T, set map[T]struct{}, ok bool) int64 {
	if !ok {
		return 0
	}
	var count int64
	for _, value := range values {
		if _, matched := set[value]; matched {
			count++
		}
	}
	return count
}

func countMembershipString(values []string, set map[string]struct{}, ok bool) int64 {
	if !ok {
		return 0
	}
	var count int64
	for _, value := range values {
		if _, matched := set[value]; matched {
			count++
		}
	}
	return count
}

func countMembershipSymbol(values []Symbol, set map[Symbol]struct{}, ok bool) int64 {
	if !ok {
		return 0
	}
	var count int64
	for _, value := range values {
		if _, matched := set[value]; matched {
			count++
		}
	}
	return count
}

func membershipBoolIndexes(values []bool, set map[bool]struct{}, ok bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for row, value := range values {
		if _, matched := set[value]; matched {
			out = append(out, row)
		}
	}
	return out, true
}

func membershipSignedIndexes[T signedScalar](values []T, set map[T]struct{}, ok bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for row, value := range values {
		if _, matched := set[value]; matched {
			out = append(out, row)
		}
	}
	return out, true
}

func membershipUnsignedIndexes[T unsignedScalar](values []T, set map[T]struct{}, ok bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for row, value := range values {
		if _, matched := set[value]; matched {
			out = append(out, row)
		}
	}
	return out, true
}

func membershipStringIndexes(values []string, set map[string]struct{}, ok bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for row, value := range values {
		if _, matched := set[value]; matched {
			out = append(out, row)
		}
	}
	return out, true
}

func membershipSymbolIndexes(values []Symbol, set map[Symbol]struct{}, ok bool, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	out = out[:0]
	for row, value := range values {
		if _, matched := set[value]; matched {
			out = append(out, row)
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
