package data

import (
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
	"sync"
)

type typedKernelRegistry struct{}

var typedKernels typedKernelRegistry

type I64IndexExprOp uint8

const (
	I64IndexExprConst I64IndexExprOp = iota
	I64IndexExprIndex
	I64IndexExprAdd
	I64IndexExprSub
	I64IndexExprMul
	I64IndexExprDiv
	I64IndexExprMod
	I64IndexExprXbar
)

type I64IndexExpr struct {
	Op    I64IndexExprOp
	Value int64
	Left  *I64IndexExpr
	Right *I64IndexExpr
}

type I64SelectedExprOp uint8

const (
	I64SelectedExprConst I64SelectedExprOp = iota
	I64SelectedExprIndex
	I64SelectedExprGather
	I64SelectedExprAdd
	I64SelectedExprSub
	I64SelectedExprMul
	I64SelectedExprDiv
	I64SelectedExprMod
	I64SelectedExprXbar
)

type I64SelectedExpr struct {
	Op     I64SelectedExprOp
	Value  int64
	Source Array
	Left   *I64SelectedExpr
	Right  *I64SelectedExpr
}

type I64IndexExprReducerKind uint8

const (
	I64IndexExprReducerSum I64IndexExprReducerKind = iota
	I64IndexExprReducerCount
)

type I64IndexExprReducer struct {
	Kind I64IndexExprReducerKind
	Expr I64IndexExpr
}

const (
	NumericUnaryNeg     = "neg"
	NumericUnaryAbs     = "abs"
	NumericUnarySqrt    = "sqrt"
	NumericUnaryLog     = "log"
	NumericUnaryExp     = "exp"
	NumericUnarySin     = "sin"
	NumericUnaryCos     = "cos"
	NumericUnaryTan     = "tan"
	NumericUnaryAsin    = "asin"
	NumericUnaryAcos    = "acos"
	NumericUnaryAtan    = "atan"
	NumericUnaryRecip   = "reciprocal"
	NumericUnarySignum  = "signum"
	NumericUnaryFloor   = "floor"
	NumericUnaryCeiling = "ceiling"

	NumericDyadicXExp = "xexp"
	NumericDyadicXLog = "xlog"
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
	case i64SegmentArray:
		v, ok := coerceInt64Exact(value)
		return compareI64SegmentMask(a, v, ok, op, out)
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
		if values, valuesOwned, ok := tryBulkI64Values(array); ok {
			v, exact := coerceInt64Exact(value)
			if exact {
				fillCompareI64Mask(values, op, v, out)
			}
			bulkI64Release(values, valuesOwned)
			if exact {
				return true
			}
		}
		if values, valuesOwned, ok := tryBulkF64Values(array); ok {
			v, numericOK := numeric(value)
			handled := compareFloatSlice(values, v, numericOK, op, out)
			bulkF64Release(values, valuesOwned)
			return handled
		}
		return false
	}
}

func (k typedKernelRegistry) CompareIndexes(array Array, op Op, value any, out []int) ([]int, bool) {
	if out == nil {
		// Bool columns count matches and size their index vector exactly, so
		// a heuristic scratch allocation would only be thrown away.
		if _, isBool := unwrapAttributedArray(array).(columnArray[bool]); !isBool {
			out = filterIndexScratch(array.Len())
		}
	}
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
		if values, valuesOwned, ok := tryBulkI64Values(array); ok {
			lo, lok := coerceInt64Exact(low)
			hi, hok := coerceInt64Exact(high)
			handled := withinSignedSlice(values, lo, hi, lok && hok, highClosed, out)
			bulkI64Release(values, valuesOwned)
			if handled {
				return true
			}
		}
		if values, valuesOwned, ok := tryBulkF64Values(array); ok {
			lo, lok := numeric(low)
			hi, hok := numeric(high)
			handled := withinFloatSlice(values, lo, hi, lok && hok, highClosed, out)
			bulkF64Release(values, valuesOwned)
			return handled
		}
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
		columnArray[Minute], columnArray[Second], columnArray[Time], columnArray[Timestamp], nullableArray,
		nullBitmapCarrier:
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
		if carrier, ok := asNullBitmapCarrier(array); ok {
			nullBitmapNullMask(carrier, out)
			return true
		}
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
	case i64SegmentCompareMask:
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
		if values, owned, ok := tryBulkBoolValues(mask); ok {
			var count int64
			for _, keep := range values {
				if keep {
					count++
				}
			}
			bulkBoolRelease(values, owned)
			return count, true, nil
		}
		return 0, false, nil
	}
}

// TryTypedBoolAggregate reduces bool/numeric vectors using q-style truthiness.
// Null values are ignored by the reduction, matching q frontend all/any
// semantics while keeping carrier-aware array access in the data runtime.
func TryTypedBoolAggregate(array Array, wantAny bool) (bool, bool, error) {
	if array == nil {
		return false, true, fmt.Errorf("bool aggregate array is nil")
	}
	if array.Kind() == KindBool {
		trueCount, handled, err := TryTypedTrueCount(array)
		if err != nil || !handled {
			return false, handled, err
		}
		if wantAny {
			return trueCount > 0, true, nil
		}
		nullCount, handled, err := TryTypedNullCount(array)
		if err != nil || !handled {
			return false, handled, err
		}
		return trueCount == int64(array.Len())-nullCount, true, nil
	}
	if !isNumericArray(array) {
		return false, false, nil
	}
	if wantAny {
		for row := 0; row < array.Len(); row++ {
			value, ok, err := numericAt(array, row)
			if err != nil {
				return false, true, err
			}
			if !ok {
				continue
			}
			if value != 0 {
				return true, true, nil
			}
		}
		return false, true, nil
	}
	for row := 0; row < array.Len(); row++ {
		value, ok, err := numericAt(array, row)
		if err != nil {
			return false, true, err
		}
		if !ok {
			continue
		}
		if value == 0 {
			return false, true, nil
		}
	}
	return true, true, nil
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
		if carrier, ok := asNullBitmapCarrier(array); ok {
			return int64(nullBitmapCount(carrier.nullBits(), carrier.Len())), true, nil
		}
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
	case i64ScalarDyadicArray:
		if count, handled := i64ScalarDyadicDistinctCount(a); handled {
			return count, true, nil
		}
		return distinctCountRows(array, array.Len())
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

func i64ScalarDyadicDistinctCount(array i64ScalarDyadicArray) (int64, bool) {
	if array.len <= 0 {
		return 0, true
	}
	if array.op != OpMod || array.scalarLeft || array.scalar <= 0 {
		return 0, false
	}
	rangeSource, ok := array.source.(i64RangeArray)
	if !ok || rangeSource.step != 1 {
		return 0, false
	}
	if int64(array.len) < array.scalar {
		return int64(array.len), true
	}
	return array.scalar, true
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

// TryTypedInMask returns a boolean membership mask for typed arrays. It shares
// the same membership coercion rules as TryTypedInCount and TryTypedInIndexesI64
// so expression eval, where planning, and future JIT lowering can target the
// same runtime primitive with different result carriers.
func TryTypedInMask(array Array, values []any) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("in mask array is nil")
	}
	return typedInMask(array, values)
}

// TryTypedInIndexesI64 returns q-style row indexes selected by a typed
// membership predicate without materializing a boolean mask.
func TryTypedInIndexesI64(array Array, values []any) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("in indexes array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		if rows, ok := typedKernels.IndexedInRows(a, values); ok {
			return intIndexesToI64Array(rows), true, nil
		}
		return TryTypedInIndexesI64(a.array, values)
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return i64RangeArray{len: 0}, true, nil
		}
		sourceRows, handled, err := TryTypedInIndexesI64(a.source, values)
		if err != nil || !handled {
			return nil, handled, err
		}
		rows, handled, err := TryTypedI64Indexes(sourceRows)
		if err != nil || !handled {
			return nil, handled, err
		}
		residues := make([]int64, 0, len(rows))
		for _, sourceRow := range rows {
			residue := (sourceRow - a.start) % sourceLen
			if residue < 0 {
				residue += sourceLen
			}
			residues = append(residues, int64(residue))
		}
		return newI64PeriodicIndexArray(int64(sourceLen), residues, a.len), true, nil
	default:
		indexes, ok := typedKernels.InIndexes(a, values, nil)
		if !ok {
			if rows, rowsOwned, bulkOK := tryBulkI64Values(a); bulkOK {
				set, setOK := int64Membership(normalizeMembershipValues(KindI64, values))
				if !setOK {
					bulkI64Release(rows, rowsOwned)
					return nil, false, nil
				}
				mask := bulkI64MembershipMask(rows, set)
				bulkI64Release(rows, rowsOwned)
				return boolMaskIndexArray(mask), true, nil
			}
			return nil, false, nil
		}
		return intIndexesToI64Array(indexes), true, nil
	}
}

// TryTypedInIndexStatsI64 returns the selected row count and row-index sum for
// a typed membership predicate without materializing a boolean mask.
func TryTypedInIndexStatsI64(array Array, values []any) (count, sum int64, handled bool, err error) {
	if array == nil {
		return 0, 0, true, fmt.Errorf("in index stats array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedInIndexStatsI64(a.array, values)
	case tiledArray:
		sourceLen := a.source.Len()
		if sourceLen == 0 || a.len == 0 {
			return 0, 0, true, nil
		}
		sourceIndexes, handled, err := TryTypedInIndexesI64(a.source, values)
		if err != nil || !handled {
			return 0, 0, handled, err
		}
		rows, handled, err := TryTypedI64Indexes(sourceIndexes)
		if err != nil || !handled {
			return 0, 0, handled, err
		}
		if len(rows) == 0 {
			return 0, 0, true, nil
		}
		matches := make([]bool, sourceLen)
		for _, sourceRow := range rows {
			residue := (sourceRow - a.start) % sourceLen
			if residue < 0 {
				residue += sourceLen
			}
			matches[residue] = true
		}
		for row := 0; row < a.len; row++ {
			if matches[row%sourceLen] {
				count++
				sum += int64(row)
			}
		}
		return count, sum, true, nil
	default:
		indexes, handled, err := TryTypedInIndexesI64(array, values)
		if err != nil || !handled {
			return 0, 0, handled, err
		}
		return int64(indexes.Len()), i64IndexArraySum(indexes), true, nil
	}
}

func intIndexesToI64Array(indexes []int) Array {
	if out, ok := i64RangeIndexArrayFromInts(indexes); ok {
		return out
	}
	out := make([]int64, len(indexes))
	for i, index := range indexes {
		out[i] = int64(index)
	}
	return newI64Trusted(out)
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
			// Same error text the generic vector dyadic raises, so typed and
			// generic logical routes fail identically.
			return nil, true, fmt.Errorf("vector length mismatch")
		}
		length = leftArray.Len()
		if rightArray.Len() > length {
			length = rightArray.Len()
		}
		// Empty-operand broadcast: ()&x follows the same rule as the len-1
		// broadcast above — the broadcastable side stretches to the other
		// side's length, so the result over an empty operand is empty
		// (mismatched non-broadcastable lengths errored above). Without this
		// the mask would cycle row%0 at materialization.
		if leftArray.Len() == 0 || rightArray.Len() == 0 {
			length = 0
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

// tryDyadicMinMaxI64Bulk computes elementwise integer min/max over dense
// bulk-flattenable carriers (with len-1 / scalar broadcast) in one tight
// loop. handled=false defers to the per-row operand walk.
func tryDyadicMinMaxI64Bulk(left, right any, length int, wantMax bool) (Array, bool) {
	scalarOf := func(value any) (int64, bool) {
		if array, ok := value.(Array); ok {
			if array.Len() != 1 {
				return 0, false
			}
			v, ok, err := integerArrayAt(array, 0)
			if err != nil || !ok {
				return 0, false
			}
			return v, true
		}
		if IsNull(value) {
			return 0, false
		}
		return integerScalarValue(value)
	}
	bulkOf := func(value any) ([]int64, bool, bool) {
		array, ok := value.(Array)
		if !ok || array.Len() != length {
			return nil, false, false
		}
		return tryBulkI64Values(array)
	}
	lv, lvOwned, lok := bulkOf(left)
	rv, rvOwned, rok := bulkOf(right)
	defer func() {
		if lok {
			bulkI64Release(lv, lvOwned)
		}
		if rok {
			bulkI64Release(rv, rvOwned)
		}
	}()
	out := make([]int64, length)
	switch {
	case lok && rok:
		for i := range out {
			a, b := lv[i], rv[i]
			if wantMax == (b > a) {
				a = b
			}
			out[i] = a
		}
	case lok:
		b, ok := scalarOf(right)
		if !ok {
			return nil, false
		}
		for i, a := range lv {
			if wantMax == (b > a) {
				a = b
			}
			out[i] = a
		}
	case rok:
		a, ok := scalarOf(left)
		if !ok {
			return nil, false
		}
		for i, b := range rv {
			v := a
			if wantMax == (b > v) {
				v = b
			}
			out[i] = v
		}
	default:
		return nil, false
	}
	return newI64Trusted(out), true
}

// TryTypedDyadicMinMax materializes canonical q `&`/`|` — elementwise
// minimum/maximum with scalar broadcast — over typed numeric operands.
// Boolean operands stay on TryTypedBoolLogical (min/max on booleans IS
// logical and/or); operands carrying nulls report handled=false so the boxed
// route applies canonical null ordering per row (null&x is null, null|x is
// x). Float NaN ranks below every other value, mirroring the null ordering.
func TryTypedDyadicMinMax(left, right any, wantMax bool) (Array, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray && !rightIsArray {
		return nil, false, nil
	}
	length := 0
	switch {
	case leftIsArray && rightIsArray:
		switch {
		case leftArray.Len() == rightArray.Len():
			length = leftArray.Len()
		case leftArray.Len() == 1:
			length = rightArray.Len()
		case rightArray.Len() == 1:
			length = leftArray.Len()
		default:
			return nil, true, fmt.Errorf("vector length mismatch")
		}
	case leftIsArray:
		length = leftArray.Len()
	default:
		length = rightArray.Len()
	}
	if leftIsArray && leftArray.Kind() == KindBool {
		return nil, false, nil
	}
	if rightIsArray && rightArray.Kind() == KindBool {
		return nil, false, nil
	}
	if IsNull(left) || IsNull(right) {
		return nil, false, nil
	}
	if !typedNumericOperand(left) || !typedNumericOperand(right) {
		return nil, false, nil
	}
	if typedIntegerOperand(left) && typedIntegerOperand(right) {
		// Same-length dense integer vectors stay lazy: streaming consumers
		// (where-replication counts, sums, bulk flattens) combine the
		// operands in pooled buffers, and materializing consumers densify
		// through tryBulkI64Values with the exact eager-loop values.
		if leftIsArray && rightIsArray && length > 0 &&
			leftArray.Len() == length && rightArray.Len() == length &&
			lazyMinMaxI64Carrier(leftArray) && lazyMinMaxI64Carrier(rightArray) {
			return i64DyadicMinMaxArray{left: leftArray, right: rightArray, wantMax: wantMax, len: length}, true, nil
		}
		if out, handled := tryDyadicMinMaxI64Bulk(left, right, length, wantMax); handled {
			return out, true, nil
		}
		out := make([]int64, length)
		for row := 0; row < length; row++ {
			lv, ok, err := integerMinMaxOperandAt(left, row, length)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			rv, ok, err := integerMinMaxOperandAt(right, row, length)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			if wantMax == (rv > lv) {
				lv = rv
			}
			out[row] = lv
		}
		return newI64Trusted(out), true, nil
	}
	out := make([]float64, length)
	for row := 0; row < length; row++ {
		lv, ok, err := numericMinMaxOperandAt(left, row, length)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		rv, ok, err := numericMinMaxOperandAt(right, row, length)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		switch {
		case math.IsNaN(lv):
			if wantMax {
				lv = rv
			}
		case math.IsNaN(rv):
			if !wantMax {
				lv = rv
			}
		case wantMax == (rv > lv):
			lv = rv
		}
		out[row] = lv
	}
	return newF64Trusted(out), true, nil
}

// TryTypedIsNullMask produces the `null xs` bool mask without routing every
// row through Array.At boxing. Dense typed storage cannot hold nulls, so it
// collapses to a constant-false cyclic view; boxed nullable columns scan
// their backing slice once; cyclic takes push the scan onto the short cycle.
func TryTypedIsNullMask(array Array) (Array, bool, error) {
	if array == nil {
		return nil, false, nil
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedIsNullMask(a.array)
	case tiledArray:
		source, handled, err := TryTypedIsNullMask(a.source)
		if err != nil || !handled {
			return nil, false, err
		}
		return tiledArray{source: source, start: a.start, len: a.len}, true, nil
	case nullableArray:
		out := make([]bool, len(a.data))
		for i, v := range a.data {
			out[i] = IsNull(v)
		}
		return newBoolTrusted(out), true, nil
	case nullBitmapCarrier:
		out := make([]bool, a.Len())
		nullBitmapNullMask(a, out)
		return newBoolTrusted(out), true, nil
	case encodedArray:
		domainNull := make([]bool, len(a.domain))
		for i, v := range a.domain {
			domainNull[i] = IsNull(v)
		}
		out := make([]bool, len(a.codes))
		for i, code := range a.codes {
			if int(code) >= 0 && int(code) < len(domainNull) {
				out[i] = domainNull[code]
			}
		}
		return newBoolTrusted(out), true, nil
	case columnArray[bool], columnArray[int8], columnArray[int16], columnArray[int32],
		columnArray[int64], columnArray[uint8], columnArray[uint16], columnArray[uint32],
		columnArray[uint64], columnArray[float32], columnArray[float64],
		i64RangeArray, i64SegmentArray, f64RangeArray:
		if array.Len() == 0 {
			return newBoolTrusted(nil), true, nil
		}
		return tiledArray{source: newBoolTrusted([]bool{false}), start: 0, len: array.Len()}, true, nil
	default:
		return nil, false, nil
	}
}

// TiledCycleView exposes a cyclic-take carrier (n#xs, rotations) so sibling
// runtime packages can push elementwise work onto the short cycle and re-tile
// the result instead of walking every row of the long view.
func TiledCycleView(array Array) (source Array, start, length int, ok bool) {
	switch a := array.(type) {
	case attributedArray:
		return TiledCycleView(a.array)
	case tiledArray:
		return a.source, a.start, a.len, true
	default:
		return nil, 0, 0, false
	}
}

// NewTiledCycleView wraps source as a cyclic view of length rows starting at
// offset start into the cycle.
func NewTiledCycleView(source Array, start, length int) (Array, bool) {
	if source == nil || source.Len() == 0 || length < 0 || start < 0 || start >= source.Len() {
		return nil, false
	}
	return tiledArray{source: source, start: start, len: length}, true
}

func typedInMask(array Array, values []any) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedInMask(a.array, values)
	case encodedArray:
		codes := make(map[int32]struct{}, len(values))
		for _, value := range values {
			code, ok := encodedComparableCode(a, value)
			if !ok {
				return nil, false, nil
			}
			codes[code] = struct{}{}
		}
		out := make([]bool, len(a.codes))
		for row, code := range a.codes {
			_, out[row] = codes[code]
		}
		return newBoolTrusted(out), true, nil
	case tiledArray:
		sourceMask, handled, err := typedInMask(a.source, values)
		if err != nil || !handled {
			return nil, handled, err
		}
		return tiledArray{source: sourceMask, start: a.start, len: a.len}, true, nil
	case columnArray[bool]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := boolMembership(values)
		return membershipBoolMask(a.data, set, ok), ok, nil
	case columnArray[int8]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int8](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[int16]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int16](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[int32]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := signedMembership[int32](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[int64]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := int64Membership(values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[uint8]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint8](values)
		return membershipUnsignedMask(a.data, set, ok), ok, nil
	case columnArray[uint16]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint16](values)
		return membershipUnsignedMask(a.data, set, ok), ok, nil
	case columnArray[uint32]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint32](values)
		return membershipUnsignedMask(a.data, set, ok), ok, nil
	case columnArray[uint64]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := unsignedMembership[uint64](values)
		return membershipUnsignedMask(a.data, set, ok), ok, nil
	case columnArray[string]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := stringMembership(values)
		return membershipStringMask(a.data, set, ok), ok, nil
	case columnArray[Symbol]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := symbolMembership(values)
		return membershipSymbolMask(a.data, set, ok), ok, nil
	case columnArray[Month]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Month](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[Date]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Date](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[DateTime]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[DateTime](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[Timespan]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Timespan](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[Minute]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Minute](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[Second]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Second](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[Time]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Time](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	case columnArray[Timestamp]:
		values = normalizeMembershipValues(array.Kind(), values)
		set, ok := exactMembership[Timestamp](values)
		return membershipSignedMask(a.data, set, ok), ok, nil
	default:
		set, setOK := int64Membership(normalizeMembershipValues(KindI64, values))
		if !setOK {
			return nil, false, nil
		}
		// Small literal sets over derived integer carriers stay lazy so the
		// fused where evaluator can fold the probe into its single pass over
		// the shared flattened source instead of materializing the mask (and
		// re-flattening the source) here.
		if len(set) <= 8 && isDenseIntegerArray(array) {
			probes := make([]int64, 0, len(set))
			for _, value := range normalizeMembershipValues(KindI64, values) {
				v, ok := coerceInt64Exact(value)
				if !ok {
					return nil, false, nil
				}
				if !i64ProbesContain(probes, v) {
					probes = append(probes, v)
				}
			}
			return i64MembershipMask{source: array, probes: probes, len: array.Len()}, true, nil
		}
		if rows, rowsOwned, ok := tryBulkI64Values(array); ok {
			mask := bulkI64MembershipMask(rows, set)
			bulkI64Release(rows, rowsOwned)
			return newBoolTrusted(mask), true, nil
		}
		return nil, false, nil
	}
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
		return a.withLazyRebuiltIndexes(out), true, nil
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
		return a.withLazyRebuiltIndexes(out), true, nil
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
		if _, ok := asNullBitmapCarrier(array); ok {
			return numericUnaryNullBitmap(op, array)
		}
		return numericUnaryDensified(op, array)
	}
}

// numericUnaryDensified handles numeric-kinded lazy carriers (for example the
// shared indexedArray row views that delete-where leaves behind) by exporting
// them to dense typed storage before running the unary kernel. Views whose
// rows the bulk exporter rejects (typically embedded nulls) fall back to a
// boxed nullable pass so view columns keep the same unary semantics their
// dense gathers had.
func numericUnaryDensified(op string, array Array) (Array, bool, error) {
	switch array.Kind() {
	case KindF32, KindF64:
		dst := make([]float64, array.Len())
		ok, err := TryExportF64Copy(array, dst)
		if err != nil {
			return nil, true, err
		}
		if ok {
			return numericUnarySlice(op, dst)
		}
	case KindI8, KindI16, KindI32, KindI64, KindU8, KindU16, KindU32, KindU64:
		dst := make([]int64, array.Len())
		ok, err := TryExportI64Copy(array, dst)
		if err != nil {
			return nil, true, err
		}
		if ok {
			return numericUnarySlice(op, dst)
		}
	default:
		return nil, false, nil
	}
	return numericUnaryNullable(op, array.Values())
}

func ApplyNumericUnaryValue(op string, value any) (any, bool, error) {
	if array, ok := value.(Array); ok {
		switch op {
		case NumericUnarySignum, NumericUnaryFloor, NumericUnaryCeiling:
			return applyNumericUnaryArray(op, array)
		}
		if out, handled, err := typedKernels.NumericUnary(op, array); err != nil || handled {
			return out, handled, err
		}
		return applyNumericUnaryArray(op, array)
	}
	if IsNull(value) {
		return NullValue, true, nil
	}
	n, ok := numeric(value)
	if !ok {
		return nil, false, nil
	}
	result, err := applyNumericUnaryFloat(op, n)
	if err != nil {
		return nil, true, err
	}
	switch op {
	case NumericUnaryNeg:
		if i, ok := integerValue(value); ok {
			return -i, true, nil
		}
	case NumericUnaryAbs:
		if i, ok := integerValue(value); ok {
			if i < 0 {
				i = -i
			}
			return i, true, nil
		}
	case NumericUnarySignum, NumericUnaryFloor, NumericUnaryCeiling:
		return int64(result), true, nil
	}
	return result, true, nil
}

func applyNumericUnaryArray(op string, array Array) (any, bool, error) {
	out := make([]float64, array.Len())
	nulls := make([]any, array.Len())
	hasNull := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, true, fmt.Errorf("%s row %d out of range", op, i)
		}
		value, handled, err := ApplyNumericUnaryValue(op, item)
		if err != nil || !handled {
			return nil, handled, err
		}
		if IsNull(value) {
			hasNull = true
			nulls[i] = NullValue
			continue
		}
		n, ok := numeric(value)
		if !ok {
			return nil, false, nil
		}
		out[i] = n
		nulls[i] = n
	}
	switch op {
	case NumericUnarySignum, NumericUnaryFloor, NumericUnaryCeiling:
		if hasNull {
			return newNullableArray(KindI64, nulls), true, nil
		}
		ints := make([]int64, len(out))
		for i, value := range out {
			ints[i] = int64(value)
		}
		return newI64Trusted(ints), true, nil
	default:
		if hasNull {
			return newNullableArray(KindF64, nulls), true, nil
		}
		return newF64Trusted(out), true, nil
	}
}

func ApplyNumericDyadicFloat(op string, left, right any) (any, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray && !rightIsArray {
		return applyNumericDyadicFloatValue(op, left, right)
	}
	if out, handled, err := TryTypedQNumericDyadicFloat(op, left, right); err != nil || handled {
		return out, handled, err
	}
	n, err := numericDyadicLength(op, leftArray, rightArray)
	if err != nil {
		return nil, true, err
	}
	out := make([]float64, n)
	nulls := make([]any, n)
	hasNull := false
	for i := 0; i < n; i++ {
		lv := left
		if leftIsArray {
			row := numericDyadicBroadcastRow(leftArray, i)
			var ok bool
			lv, ok = leftArray.At(row)
			if !ok {
				return nil, true, fmt.Errorf("%s left row %d out of range", op, row)
			}
		}
		rv := right
		if rightIsArray {
			row := numericDyadicBroadcastRow(rightArray, i)
			var ok bool
			rv, ok = rightArray.At(row)
			if !ok {
				return nil, true, fmt.Errorf("%s right row %d out of range", op, row)
			}
		}
		value, ok, err := applyNumericDyadicFloatValue(op, lv, rv)
		if err != nil || !ok {
			return nil, ok, err
		}
		if IsNull(value) {
			hasNull = true
			nulls[i] = NullValue
			continue
		}
		f, ok := value.(float64)
		if !ok {
			return nil, true, fmt.Errorf("%s expects numeric operands", op)
		}
		out[i] = f
		nulls[i] = f
	}
	if hasNull {
		return newNullableArray(KindF64, nulls), true, nil
	}
	return newF64Trusted(out), true, nil
}

func TryTypedQNumericDyadicFloat(op string, left, right any) (Array, bool, error) {
	bound, handled, err := BindNumericDyadicFloat(op, left, right)
	if err != nil || !handled {
		return nil, handled, err
	}
	return bound.Array(), true, nil
}

func TryTypedQNumericDyadicFloatSum(op string, left, right any) (any, bool, error) {
	bound, handled, err := BindNumericDyadicFloat(op, left, right)
	if err != nil || !handled {
		return nil, handled, err
	}
	sum, err := bound.Sum()
	if err != nil {
		return nil, true, err
	}
	return sum, true, nil
}

// TryTypedNumericSumPlusScalarDyadicFloatSum fuses two reducers over one
// numeric vector: sum(v) + sum(scalar op v) or sum(v) + sum(v op scalar).
// Language frontends and JIT backends can use it for multi-reducer pipeline
// shapes without materializing intermediate arrays or repeating producer setup.
func TryTypedNumericSumPlusScalarDyadicFloatSum(array Array, op string, scalar any, scalarLeft bool) (any, bool, error) {
	if array == nil || !typedNumericOperand(array) || !typedNumericOperand(scalar) {
		return nil, false, nil
	}
	scalarValue, ok := numeric(scalar)
	if !ok {
		return nil, false, nil
	}
	if out, ok, err := tryTypedNumericSumPlusInverseDyadicFloatSum(array, op, scalarValue, scalarLeft); ok || err != nil {
		return out, ok, err
	}
	apply, ok := numericDyadicFloatFunc(op)
	if !ok {
		return nil, true, fmt.Errorf("unsupported numeric dyadic float kernel %q", op)
	}
	producer, err := newF64NumericProducer(array, array.Len())
	if err != nil {
		return nil, true, err
	}
	total, err := f64ProducerSumPlusScalarDyadicSum(producer, scalarValue, scalarLeft, op, apply)
	if err != nil {
		return nil, true, err
	}
	return total, true, nil
}

// NumericDyadicFloatBound is an opaque pre-bound typed runtime operand for
// dyadic float kernels such as xexp/xlog. It is immutable and reusable by q
// warm paths and future JIT backends.
type NumericDyadicFloatBound struct {
	op       string
	producer f64DyadicProducer
}

// BindNumericDyadicFloat pre-binds numeric operands for a dyadic float kernel.
// At least one operand must be array-like; scalar/scalar calls stay on the
// regular scalar evaluator.
func BindNumericDyadicFloat(op string, left, right any) (NumericDyadicFloatBound, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray && !rightIsArray {
		return NumericDyadicFloatBound{}, false, nil
	}
	if !typedNumericOperand(left) || !typedNumericOperand(right) {
		return NumericDyadicFloatBound{}, false, nil
	}
	length, err := numericDyadicLength(op, leftArray, rightArray)
	if err != nil {
		return NumericDyadicFloatBound{}, true, err
	}
	apply, ok := numericDyadicFloatFunc(op)
	if !ok {
		return NumericDyadicFloatBound{}, true, fmt.Errorf("unsupported numeric dyadic float kernel %q", op)
	}
	leftProducer, err := newF64NumericProducer(left, length)
	if err != nil {
		return NumericDyadicFloatBound{}, true, err
	}
	rightProducer, err := newF64NumericProducer(right, length)
	if err != nil {
		return NumericDyadicFloatBound{}, true, err
	}
	return NumericDyadicFloatBound{
		op:       op,
		producer: f64DyadicProducer{left: leftProducer, right: rightProducer, op: op, apply: apply, len: length},
	}, true, nil
}

func (b NumericDyadicFloatBound) Len() int {
	return b.producer.len
}

func (b NumericDyadicFloatBound) Sum() (float64, error) {
	return f64ProducerSum(b.producer)
}

func (b NumericDyadicFloatBound) RatiosSum() (float64, error) {
	return f64ProducerRatiosSum(b.producer)
}

func (b NumericDyadicFloatBound) Array() Array {
	return f64NumericDyadicArray{
		op:    b.op,
		len:   b.producer.len,
		bound: b,
	}
}

func numericDyadicFloatOperandAt(value any, array Array, isArray bool, row, length int) (float64, bool, error) {
	if isArray {
		if array == nil {
			return 0, false, nil
		}
		if array.Len() == 1 && length != 1 {
			row = 0
		}
		return typedKernels.NumericAt(array, row)
	}
	if IsNull(value) {
		return 0, false, nil
	}
	n, ok := numeric(value)
	if !ok {
		return 0, false, fmt.Errorf("typed numeric dyadic float operand row %d is %T, want numeric", row, value)
	}
	return n, true, nil
}

type f64NumericProducer interface {
	Len() int
	f64At(row int) (float64, bool, error)
}

type f64NullProducer struct {
	len int
}

type f64ScalarProducer struct {
	value float64
	len   int
}

type f64BroadcastProducer struct {
	source f64NumericProducer
	len    int
}

type f64ArrayProducer struct {
	array Array
}

type f64I8ColumnProducer struct {
	data []int8
}

type f64I16ColumnProducer struct {
	data []int16
}

type f64I32ColumnProducer struct {
	data []int32
}

type f64I64ColumnProducer struct {
	data []int64
}

type f64F32ColumnProducer struct {
	data []float32
}

type f64F64ColumnProducer struct {
	data []float64
}

type f64I64RangeProducer struct {
	values i64RangeArray
}

type f64F64RangeProducer struct {
	values f64RangeArray
}

type f64I64ScalarDyadicProducer struct {
	values i64ScalarDyadicArray
}

type f64DyadicProducer struct {
	left  f64NumericProducer
	right f64NumericProducer
	op    string
	apply f64DyadicFunc
	len   int
}

type f64DyadicEvalCache struct {
	entries [16]f64DyadicEvalCacheEntry
	next    int
}

type f64DyadicEvalCacheEntry struct {
	left  float64
	right float64
	value float64
	valid bool
}

func newF64NumericProducer(value any, length int) (f64NumericProducer, error) {
	if array, ok := value.(Array); ok {
		producer, err := newF64NumericArrayProducer(array)
		if err != nil {
			return nil, err
		}
		if array.Len() == 1 && length != 1 {
			return f64BroadcastProducer{source: producer, len: length}, nil
		}
		return producer, nil
	}
	if IsNull(value) {
		return f64NullProducer{len: length}, nil
	}
	n, ok := numeric(value)
	if !ok {
		return nil, fmt.Errorf("typed numeric producer operand is %T, want numeric", value)
	}
	return f64ScalarProducer{value: n, len: length}, nil
}

func newF64NumericArrayProducer(array Array) (f64NumericProducer, error) {
	switch a := array.(type) {
	case attributedArray:
		return newF64NumericArrayProducer(a.array)
	case columnArray[int8]:
		return f64I8ColumnProducer{data: a.data}, nil
	case columnArray[int16]:
		return f64I16ColumnProducer{data: a.data}, nil
	case columnArray[int32]:
		return f64I32ColumnProducer{data: a.data}, nil
	case columnArray[int64]:
		return f64I64ColumnProducer{data: a.data}, nil
	case columnArray[float32]:
		return f64F32ColumnProducer{data: a.data}, nil
	case columnArray[float64]:
		return f64F64ColumnProducer{data: a.data}, nil
	case i64RangeArray:
		return f64I64RangeProducer{values: a}, nil
	case f64RangeArray:
		return f64F64RangeProducer{values: a}, nil
	case i64ScalarDyadicArray:
		return f64I64ScalarDyadicProducer{values: a}, nil
	case f64NumericDyadicArray:
		producer, err := newF64NumericDyadicProducer(a)
		if err != nil {
			return nil, err
		}
		return producer, nil
	case castF32Array:
		inner, err := newF64NumericArrayProducer(a.source)
		if err != nil {
			return nil, err
		}
		return f64CastF32Producer{source: inner}, nil
	case castI64Array:
		inner, err := newF64NumericArrayProducer(a.source)
		if err != nil {
			return nil, err
		}
		return f64CastI64Producer{source: inner}, nil
	default:
		if !isNumericArray(array) {
			return nil, fmt.Errorf("typed numeric producer operand is %s, want numeric", array.Kind())
		}
		return f64ArrayProducer{array: array}, nil
	}
}

// f64CastF32Producer streams a lazy `real$` view: source values rounded
// through float32.
type f64CastF32Producer struct {
	source f64NumericProducer
}

func (p f64CastF32Producer) Len() int { return p.source.Len() }

func (p f64CastF32Producer) f64At(row int) (float64, bool, error) {
	value, ok, err := p.source.f64At(row)
	if err != nil || !ok {
		return 0, ok, err
	}
	return float64(float32(value)), true, nil
}

// f64CastI64Producer streams a lazy integer cast view: source values
// rounded half-to-even (canonical q integer cast).
type f64CastI64Producer struct {
	source f64NumericProducer
}

func (p f64CastI64Producer) Len() int { return p.source.Len() }

func (p f64CastI64Producer) f64At(row int) (float64, bool, error) {
	value, ok, err := p.source.f64At(row)
	if err != nil || !ok {
		return 0, ok, err
	}
	return float64(int64(math.RoundToEven(value))), true, nil
}

func (p f64NullProducer) Len() int { return p.len }

func (p f64NullProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= p.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return 0, false, nil
}

func (p f64ScalarProducer) Len() int { return p.len }

func (p f64ScalarProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= p.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return p.value, true, nil
}

func (p f64BroadcastProducer) Len() int { return p.len }

func (p f64BroadcastProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= p.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return p.source.f64At(0)
}

func (p f64ArrayProducer) Len() int { return p.array.Len() }

func (p f64ArrayProducer) f64At(row int) (float64, bool, error) {
	return typedKernels.NumericAt(p.array, row)
}

func (p f64I8ColumnProducer) Len() int { return len(p.data) }

func (p f64I8ColumnProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= len(p.data) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(p.data[row]), true, nil
}

func (p f64I16ColumnProducer) Len() int { return len(p.data) }

func (p f64I16ColumnProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= len(p.data) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(p.data[row]), true, nil
}

func (p f64I32ColumnProducer) Len() int { return len(p.data) }

func (p f64I32ColumnProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= len(p.data) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(p.data[row]), true, nil
}

func (p f64I64ColumnProducer) Len() int { return len(p.data) }

func (p f64I64ColumnProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= len(p.data) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(p.data[row]), true, nil
}

func (p f64F32ColumnProducer) Len() int { return len(p.data) }

func (p f64F32ColumnProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= len(p.data) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return float64(p.data[row]), true, nil
}

func (p f64F64ColumnProducer) Len() int { return len(p.data) }

func (p f64F64ColumnProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= len(p.data) {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return p.data[row], true, nil
}

func (p f64I64RangeProducer) Len() int { return p.values.len }

func (p f64I64RangeProducer) f64At(row int) (float64, bool, error) {
	return numericI64RangeAt(p.values, row)
}

func (p f64F64RangeProducer) Len() int { return p.values.len }

func (p f64F64RangeProducer) f64At(row int) (float64, bool, error) {
	return numericF64RangeAt(p.values, row)
}

func (p f64I64ScalarDyadicProducer) Len() int { return p.values.len }

func (p f64I64ScalarDyadicProducer) f64At(row int) (float64, bool, error) {
	value, ok, err := p.values.i64At(row)
	if err != nil || !ok {
		return 0, ok, err
	}
	return float64(value), true, nil
}

func newF64NumericDyadicProducer(array f64NumericDyadicArray) (f64DyadicProducer, error) {
	if array.bound.producer.apply != nil && array.bound.producer.len == array.len {
		return array.bound.producer, nil
	}
	apply, ok := numericDyadicFloatFunc(array.op)
	if !ok {
		return f64DyadicProducer{}, fmt.Errorf("unsupported numeric dyadic float kernel %q", array.op)
	}
	leftProducer, err := newF64NumericProducer(array.left, array.len)
	if err != nil {
		return f64DyadicProducer{}, err
	}
	rightProducer, err := newF64NumericProducer(array.right, array.len)
	if err != nil {
		return f64DyadicProducer{}, err
	}
	return f64DyadicProducer{left: leftProducer, right: rightProducer, op: array.op, apply: apply, len: array.len}, nil
}

func (p f64DyadicProducer) Len() int { return p.len }

func (p f64DyadicProducer) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= p.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	leftValue, leftOK, err := p.left.f64At(row)
	if err != nil || !leftOK {
		return 0, leftOK, err
	}
	rightValue, rightOK, err := p.right.f64At(row)
	if err != nil || !rightOK {
		return 0, rightOK, err
	}
	return p.apply(leftValue, rightValue), true, nil
}

func (p f64DyadicProducer) f64AtCached(row int, cache *f64DyadicEvalCache) (float64, bool, error) {
	if cache == nil {
		return p.f64At(row)
	}
	if row < 0 || row >= p.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	leftValue, leftOK, err := p.left.f64At(row)
	if err != nil || !leftOK {
		return 0, leftOK, err
	}
	rightValue, rightOK, err := p.right.f64At(row)
	if err != nil || !rightOK {
		return 0, rightOK, err
	}
	return cache.apply(p.apply, leftValue, rightValue), true, nil
}

func f64ProducerSum(producer f64NumericProducer) (float64, error) {
	switch p := producer.(type) {
	case f64NullProducer:
		return 0, nil
	case f64ScalarProducer:
		return p.value * float64(p.len), nil
	case f64BroadcastProducer:
		value, ok, err := p.source.f64At(0)
		if err != nil || !ok {
			return 0, err
		}
		return value * float64(p.len), nil
	case f64I8ColumnProducer:
		return f64ProducerSumSigned(p.data), nil
	case f64I16ColumnProducer:
		return f64ProducerSumSigned(p.data), nil
	case f64I32ColumnProducer:
		return f64ProducerSumSigned(p.data), nil
	case f64I64ColumnProducer:
		return f64ProducerSumSigned(p.data), nil
	case f64F32ColumnProducer:
		return f64ProducerSumFloat(p.data), nil
	case f64F64ColumnProducer:
		return f64ProducerSumFloat(p.data), nil
	case f64I64RangeProducer:
		return float64(i64RangeSum(p.values)), nil
	case f64F64RangeProducer:
		return f64RangeSum(p.values), nil
	case f64I64ScalarDyadicProducer:
		sum, ok, err := i64ScalarDyadicSum(p.values)
		if err != nil || !ok {
			return 0, err
		}
		total, ok := sum.(int64)
		if !ok {
			return 0, fmt.Errorf("typed numeric producer sum is %T, want int64", sum)
		}
		return float64(total), nil
	case f64DyadicProducer:
		return f64DyadicProducerSumFloat(p)
	}
	var total float64
	for row := 0; row < producer.Len(); row++ {
		value, ok, err := producer.f64At(row)
		if err != nil {
			return 0, err
		}
		if ok {
			total += value
		}
	}
	return total, nil
}

func f64ProducerRatiosSum(producer f64NumericProducer) (float64, error) {
	switch p := producer.(type) {
	case f64NullProducer:
		return 0, nil
	case f64ScalarProducer:
		if p.len <= 0 {
			return 0, nil
		}
		if p.len == 1 {
			return p.value, nil
		}
		if p.value == 0 {
			return math.NaN(), nil
		}
		return p.value + float64(p.len-1), nil
	case f64BroadcastProducer:
		value, ok, err := p.source.f64At(0)
		if err != nil || !ok {
			return 0, err
		}
		if p.len <= 0 {
			return 0, nil
		}
		if p.len == 1 {
			return value, nil
		}
		if value == 0 {
			return math.NaN(), nil
		}
		return value + float64(p.len-1), nil
	case f64I8ColumnProducer:
		return f64ProducerRatiosSumSigned(p.data), nil
	case f64I16ColumnProducer:
		return f64ProducerRatiosSumSigned(p.data), nil
	case f64I32ColumnProducer:
		return f64ProducerRatiosSumSigned(p.data), nil
	case f64I64ColumnProducer:
		return f64ProducerRatiosSumSigned(p.data), nil
	case f64F32ColumnProducer:
		return f64ProducerRatiosSumFloat(p.data), nil
	case f64F64ColumnProducer:
		return f64ProducerRatiosSumFloat(p.data), nil
	case f64I64RangeProducer:
		return f64ProducerRatiosSumI64Range(p.values), nil
	case f64F64RangeProducer:
		return f64ProducerRatiosSumF64Range(p.values), nil
	case f64I64ScalarDyadicProducer:
		return f64ProducerRatiosSumI64ScalarDyadic(p.values)
	case f64DyadicProducer:
		return f64DyadicRatiosSumFloat(p)
	}
	var total float64
	var previous float64
	var hasPrevious bool
	for row := 0; row < producer.Len(); row++ {
		current, ok, err := producer.f64At(row)
		if err != nil {
			return 0, err
		}
		if !ok {
			hasPrevious = false
			continue
		}
		if !hasPrevious {
			total += current
		} else {
			total += current / previous
		}
		previous = current
		hasPrevious = true
	}
	return total, nil
}

func f64ProducerSumSigned[T signedScalar](values []T) float64 {
	var total float64
	for _, value := range values {
		total += float64(value)
	}
	return total
}

func f64ProducerSumFloat[T floatScalar](values []T) float64 {
	var total float64
	for _, value := range values {
		total += float64(value)
	}
	return total
}

func f64ProducerRatiosSumSigned[T signedScalar](values []T) float64 {
	var total float64
	var previous float64
	for i, value := range values {
		current := float64(value)
		if i == 0 {
			total += current
		} else {
			total += current / previous
		}
		previous = current
	}
	return total
}

func f64ProducerRatiosSumFloat[T floatScalar](values []T) float64 {
	var total float64
	var previous float64
	for i, value := range values {
		current := float64(value)
		if i == 0 {
			total += current
		} else {
			total += current / previous
		}
		previous = current
	}
	return total
}

func f64ProducerRatiosSumI64Range(values i64RangeArray) float64 {
	var total float64
	var previous float64
	for row := 0; row < values.len; row++ {
		current := float64(values.start + int64(row)*values.step)
		if row == 0 {
			total += current
		} else {
			total += current / previous
		}
		previous = current
	}
	return total
}

func f64ProducerRatiosSumF64Range(values f64RangeArray) float64 {
	var total float64
	var previous float64
	for row := 0; row < values.len; row++ {
		current := values.start + float64(row)*values.step
		if row == 0 {
			total += current
		} else {
			total += current / previous
		}
		previous = current
	}
	return total
}

func f64ProducerRatiosSumI64ScalarDyadic(values i64ScalarDyadicArray) (float64, error) {
	var total float64
	var previous float64
	for row := 0; row < values.len; row++ {
		current, ok, err := values.i64At(row)
		if err != nil || !ok {
			return 0, err
		}
		f := float64(current)
		if row == 0 {
			total += f
		} else {
			total += f / previous
		}
		previous = f
	}
	return total, nil
}

func f64DyadicProducerSumFloat(producer f64DyadicProducer) (float64, error) {
	if left, ok := producer.left.(f64ScalarProducer); ok {
		return f64DyadicScalarProducerSum(left.value, producer.right, true, producer.op, producer.apply)
	}
	if right, ok := producer.right.(f64ScalarProducer); ok {
		return f64DyadicScalarProducerSum(right.value, producer.left, false, producer.op, producer.apply)
	}
	if values, owned, ok := tryBulkF64ProducerValues(producer); ok {
		var total float64
		for _, value := range values {
			total += value
		}
		bulkF64Release(values, owned)
		return total, nil
	}
	var total float64
	var cache f64DyadicEvalCache
	for row := 0; row < producer.len; row++ {
		value, ok, err := producer.f64AtCached(row, &cache)
		if err != nil {
			return 0, err
		}
		if ok {
			total += value
		}
	}
	return total, nil
}

func f64DyadicScalarProducerSum(scalar float64, producer f64NumericProducer, scalarLeft bool, op string, apply f64DyadicFunc) (float64, error) {
	if reducer, ok := newF64DyadicScalarReducer(op, scalar, scalarLeft); ok {
		if total, handled, err := f64DyadicScalarProducerSumFast(producer, reducer); handled || err != nil {
			return total, err
		}
	}
	// Null-bitmap backed arrays flatten once and reduce densely, skipping
	// null rows; the per-row producer walk below would box every access.
	if p, ok := producer.(f64ArrayProducer); ok && nullBitmapBackedArray(p.array) {
		if values, nulls, owned, ok := tryBulkF64NullableValues(p.array); ok {
			var total float64
			for i, value := range values {
				if nulls != nil && nullBitGet(nulls, i) {
					continue
				}
				total += applyScalarDyadicFloat(scalar, value, scalarLeft, apply)
			}
			bulkF64Release(values, owned)
			return total, nil
		}
	}
	switch p := producer.(type) {
	case f64NullProducer:
		return 0, nil
	case f64ScalarProducer:
		return applyScalarDyadicFloat(scalar, p.value, scalarLeft, apply) * float64(p.len), nil
	case f64BroadcastProducer:
		value, ok, err := p.source.f64At(0)
		if err != nil || !ok {
			return 0, err
		}
		return applyScalarDyadicFloat(scalar, value, scalarLeft, apply) * float64(p.len), nil
	case f64I8ColumnProducer:
		return f64DyadicScalarSumSigned(scalar, p.data, scalarLeft, op, apply), nil
	case f64I16ColumnProducer:
		return f64DyadicScalarSumSigned(scalar, p.data, scalarLeft, op, apply), nil
	case f64I32ColumnProducer:
		return f64DyadicScalarSumSigned(scalar, p.data, scalarLeft, op, apply), nil
	case f64I64ColumnProducer:
		return f64DyadicScalarSumSigned(scalar, p.data, scalarLeft, op, apply), nil
	case f64F32ColumnProducer:
		return f64DyadicScalarSumFloat(scalar, p.data, scalarLeft, op, apply), nil
	case f64F64ColumnProducer:
		return f64DyadicScalarSumFloat(scalar, p.data, scalarLeft, op, apply), nil
	case f64I64RangeProducer:
		var total float64
		for row := 0; row < p.values.len; row++ {
			total += applyScalarDyadicFloat(scalar, float64(p.values.start+int64(row)*p.values.step), scalarLeft, apply)
		}
		return total, nil
	case f64F64RangeProducer:
		var total float64
		for row := 0; row < p.values.len; row++ {
			total += applyScalarDyadicFloat(scalar, p.values.start+float64(row)*p.values.step, scalarLeft, apply)
		}
		return total, nil
	case f64I64ScalarDyadicProducer:
		if total, ok, err := f64DyadicScalarI64ScalarDyadicRangeSum(scalar, p.values, scalarLeft, apply); ok || err != nil {
			return total, err
		}
		var total float64
		for row := 0; row < p.values.len; row++ {
			value, ok, err := p.values.i64At(row)
			if err != nil || !ok {
				return 0, err
			}
			total += applyScalarDyadicFloat(scalar, float64(value), scalarLeft, apply)
		}
		return total, nil
	case f64DyadicProducer:
		return f64DyadicScalarNestedProducerSum(scalar, p, scalarLeft, op, apply)
	}
	var total float64
	for row := 0; row < producer.Len(); row++ {
		value, ok, err := producer.f64At(row)
		if err != nil {
			return 0, err
		}
		if ok {
			total += applyScalarDyadicFloat(scalar, value, scalarLeft, apply)
		}
	}
	return total, nil
}

func f64DyadicScalarSumSigned[T signedScalar](scalar float64, values []T, scalarLeft bool, op string, apply f64DyadicFunc) float64 {
	var total float64
	if reducer, ok := newF64DyadicScalarReducer(op, scalar, scalarLeft); ok {
		for _, value := range values {
			total += reducer.apply(float64(value))
		}
		return total
	}
	for _, value := range values {
		total += applyScalarDyadicFloat(scalar, float64(value), scalarLeft, apply)
	}
	return total
}

func f64DyadicScalarSumFloat[T floatScalar](scalar float64, values []T, scalarLeft bool, op string, apply f64DyadicFunc) float64 {
	var total float64
	if reducer, ok := newF64DyadicScalarReducer(op, scalar, scalarLeft); ok {
		for _, value := range values {
			total += reducer.apply(float64(value))
		}
		return total
	}
	for _, value := range values {
		total += applyScalarDyadicFloat(scalar, float64(value), scalarLeft, apply)
	}
	return total
}

type f64DyadicScalarReducer struct {
	apply func(float64) float64
}

func newF64DyadicScalarReducer(op string, scalar float64, scalarLeft bool) (f64DyadicScalarReducer, bool) {
	switch op {
	case NumericDyadicXExp:
		if scalarLeft {
			switch scalar {
			case 2:
				return f64DyadicScalarReducer{apply: math.Exp2}, true
			case 1:
				return f64DyadicScalarReducer{apply: func(float64) float64 {
					return 1
				}}, true
			default:
				if scalar <= 0 {
					return f64DyadicScalarReducer{}, false
				}
				logBase := math.Log(scalar)
				return f64DyadicScalarReducer{apply: func(value float64) float64 {
					return math.Exp(logBase * value)
				}}, true
			}
		}
		switch scalar {
		case 2:
			return f64DyadicScalarReducer{apply: func(value float64) float64 {
				return value * value
			}}, true
		case 1:
			// x xexp 1 is x (the exponent is 1), not the constant 1.
			return f64DyadicScalarReducer{apply: func(value float64) float64 {
				return value
			}}, true
		case 0:
			return f64DyadicScalarReducer{apply: func(float64) float64 {
				return 1
			}}, true
		default:
			return f64DyadicScalarReducer{apply: func(value float64) float64 {
				return math.Pow(value, scalar)
			}}, true
		}
	case NumericDyadicXLog:
		if scalarLeft {
			if scalar == 2 {
				return f64DyadicScalarReducer{apply: math.Log2}, true
			}
			if scalar > 0 && scalar != 1 {
				invLogBase := 1 / math.Log(scalar)
				return f64DyadicScalarReducer{apply: func(value float64) float64 {
					return math.Log(value) * invLogBase
				}}, true
			}
			return f64DyadicScalarReducer{}, false
		}
		logScalar := math.Log(scalar)
		return f64DyadicScalarReducer{apply: func(value float64) float64 {
			return logScalar / math.Log(value)
		}}, true
	default:
		return f64DyadicScalarReducer{}, false
	}
}

func f64DyadicScalarProducerSumFast(producer f64NumericProducer, reducer f64DyadicScalarReducer) (float64, bool, error) {
	switch p := producer.(type) {
	case f64NullProducer:
		return 0, true, nil
	case f64ScalarProducer:
		return reducer.apply(p.value) * float64(p.len), true, nil
	case f64BroadcastProducer:
		value, ok, err := p.source.f64At(0)
		if err != nil || !ok {
			return 0, true, err
		}
		return reducer.apply(value) * float64(p.len), true, nil
	case f64I8ColumnProducer:
		return f64DyadicScalarReducerSumSigned(p.data, reducer), true, nil
	case f64I16ColumnProducer:
		return f64DyadicScalarReducerSumSigned(p.data, reducer), true, nil
	case f64I32ColumnProducer:
		return f64DyadicScalarReducerSumSigned(p.data, reducer), true, nil
	case f64I64ColumnProducer:
		return f64DyadicScalarReducerSumSigned(p.data, reducer), true, nil
	case f64F32ColumnProducer:
		return f64DyadicScalarReducerSumFloat(p.data, reducer), true, nil
	case f64F64ColumnProducer:
		return f64DyadicScalarReducerSumFloat(p.data, reducer), true, nil
	case f64I64RangeProducer:
		var total float64
		for row := 0; row < p.values.len; row++ {
			total += reducer.apply(float64(p.values.start + int64(row)*p.values.step))
		}
		return total, true, nil
	case f64F64RangeProducer:
		var total float64
		for row := 0; row < p.values.len; row++ {
			total += reducer.apply(p.values.start + float64(row)*p.values.step)
		}
		return total, true, nil
	case f64I64ScalarDyadicProducer:
		return f64DyadicScalarReducerI64ScalarDyadicSum(p.values, reducer)
	case f64DyadicProducer:
		return f64DyadicScalarNestedProducerSumFast(p, reducer)
	default:
		return 0, false, nil
	}
}

func f64DyadicScalarProducerRatiosSumFast(producer f64NumericProducer, reducer f64DyadicScalarReducer) (float64, bool, error) {
	switch p := producer.(type) {
	case f64NullProducer:
		return 0, true, nil
	case f64ScalarProducer:
		if p.len <= 0 {
			return 0, true, nil
		}
		value := reducer.apply(p.value)
		return value + float64(p.len-1), true, nil
	case f64BroadcastProducer:
		value, ok, err := p.source.f64At(0)
		if err != nil || !ok {
			return 0, true, err
		}
		if p.len <= 0 {
			return 0, true, nil
		}
		current := reducer.apply(value)
		return current + float64(p.len-1), true, nil
	case f64DyadicProducer:
		total, err := f64DyadicScalarNestedProducerRatiosSumFast(p, reducer)
		return total, true, err
	case f64I64ScalarDyadicProducer:
		return f64DyadicScalarReducerI64ScalarDyadicRatiosSum(p.values, reducer)
	default:
		return 0, false, nil
	}
}

func f64DyadicScalarReducerSumSigned[T signedScalar](values []T, reducer f64DyadicScalarReducer) float64 {
	var total float64
	for _, value := range values {
		total += reducer.apply(float64(value))
	}
	return total
}

func f64DyadicScalarReducerSumFloat[T floatScalar](values []T, reducer f64DyadicScalarReducer) float64 {
	var total float64
	for _, value := range values {
		total += reducer.apply(float64(value))
	}
	return total
}

func f64DyadicScalarReducerI64ScalarDyadicSum(values i64ScalarDyadicArray, reducer f64DyadicScalarReducer) (float64, bool, error) {
	source, ok := values.source.(i64RangeArray)
	if !ok || source.len != values.len {
		var total float64
		for row := 0; row < values.len; row++ {
			value, ok, err := values.i64At(row)
			if err != nil || !ok {
				return 0, true, err
			}
			total += reducer.apply(float64(value))
		}
		return total, true, nil
	}
	var total float64
	for row := 0; row < values.len; row++ {
		value, err := f64I64ScalarDyadicRangeValue(source, values, row)
		if err != nil {
			return 0, true, err
		}
		total += reducer.apply(value)
	}
	return total, true, nil
}

func f64DyadicScalarReducerI64ScalarDyadicRatiosSum(values i64ScalarDyadicArray, reducer f64DyadicScalarReducer) (float64, bool, error) {
	if total, handled := f64DyadicScalarReducerI64ModuloRangeRatiosSum(values, reducer); handled {
		return total, true, nil
	}
	source, isRange := values.source.(i64RangeArray)
	var total float64
	var previous float64
	var hasPrevious bool
	for row := 0; row < values.len; row++ {
		var value float64
		if isRange && source.len == values.len {
			rangeValue, err := f64I64ScalarDyadicRangeValue(source, values, row)
			if err != nil {
				return 0, true, err
			}
			value = rangeValue
		} else {
			current, ok, err := values.i64At(row)
			if err != nil || !ok {
				return 0, true, err
			}
			value = float64(current)
		}
		current := reducer.apply(value)
		if !hasPrevious {
			total += current
		} else {
			total += current / previous
		}
		previous = current
		hasPrevious = true
	}
	return total, true, nil
}

func f64DyadicScalarReducerI64ModuloRangeRatiosSum(values i64ScalarDyadicArray, reducer f64DyadicScalarReducer) (float64, bool) {
	if values.op != OpMod || values.scalarLeft || values.scalar <= 0 || values.len <= 0 {
		return 0, false
	}
	source, ok := values.source.(i64RangeArray)
	if !ok || source.step != 1 || source.len != values.len {
		return 0, false
	}
	modulus := values.scalar
	if int64(int(modulus)) != modulus {
		return 0, false
	}
	remaining := values.len
	previousResidue := qPositiveMod(source.start, modulus)
	previous := reducer.apply(float64(previousResidue))
	total := previous
	remaining--
	if remaining == 0 {
		return total, true
	}
	period := int(modulus)
	if period <= 0 {
		return 0, false
	}
	cycleSum := 0.0
	residue := (previousResidue + 1) % modulus
	cyclePrevious := previous
	for i := 0; i < period; i++ {
		current := reducer.apply(float64(residue))
		cycleSum += current / cyclePrevious
		cyclePrevious = current
		residue = (residue + 1) % modulus
	}
	fullCycles := remaining / period
	if fullCycles > 0 {
		total += float64(fullCycles) * cycleSum
		remaining -= fullCycles * period
		previous = cyclePrevious
		previousResidue = qPositiveMod(source.start+int64(fullCycles*period), modulus)
	}
	residue = (previousResidue + 1) % modulus
	for i := 0; i < remaining; i++ {
		current := reducer.apply(float64(residue))
		total += current / previous
		previous = current
		residue = (residue + 1) % modulus
	}
	return total, true
}

func f64DyadicScalarNestedProducerSumFast(producer f64DyadicProducer, reducer f64DyadicScalarReducer) (float64, bool, error) {
	var total float64
	var innerCache f64DyadicEvalCache
	for row := 0; row < producer.len; row++ {
		value, ok, err := producer.f64AtCached(row, &innerCache)
		if err != nil {
			return 0, true, err
		}
		if ok {
			total += reducer.apply(value)
		}
	}
	return total, true, nil
}

func f64DyadicScalarNestedProducerRatiosSumFast(producer f64DyadicProducer, reducer f64DyadicScalarReducer) (float64, error) {
	var total float64
	var previous float64
	var hasPrevious bool
	var innerCache f64DyadicEvalCache
	for row := 0; row < producer.len; row++ {
		value, ok, err := producer.f64AtCached(row, &innerCache)
		if err != nil {
			return 0, err
		}
		if !ok {
			hasPrevious = false
			continue
		}
		current := reducer.apply(value)
		if !hasPrevious {
			total += current
		} else {
			total += current / previous
		}
		previous = current
		hasPrevious = true
	}
	return total, nil
}

func f64ProducerSumPlusScalarDyadicSum(producer f64NumericProducer, scalar float64, scalarLeft bool, op string, apply f64DyadicFunc) (float64, error) {
	reducer, hasReducer := newF64DyadicScalarReducer(op, scalar, scalarLeft)
	var total float64
	var cache f64DyadicEvalCache
	for row := 0; row < producer.Len(); row++ {
		var value float64
		var ok bool
		var err error
		if p, isDyadic := producer.(f64DyadicProducer); isDyadic {
			value, ok, err = p.f64AtCached(row, &cache)
		} else {
			value, ok, err = producer.f64At(row)
		}
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		total += value
		if hasReducer {
			total += reducer.apply(value)
			continue
		}
		total += applyScalarDyadicFloat(scalar, value, scalarLeft, apply)
	}
	return total, nil
}

func tryTypedNumericSumPlusInverseDyadicFloatSum(array Array, op string, scalar float64, scalarLeft bool) (any, bool, error) {
	if op != NumericDyadicXLog || !scalarLeft || scalar <= 0 || scalar == 1 {
		return nil, false, nil
	}
	dyadic, ok := array.(f64NumericDyadicArray)
	if !ok {
		return nil, false, nil
	}
	producer, err := newF64NumericDyadicProducer(dyadic)
	if err != nil {
		return nil, true, err
	}
	if producer.op != NumericDyadicXExp {
		return nil, false, nil
	}
	left, ok := producer.left.(f64ScalarProducer)
	if !ok || left.value != scalar {
		return nil, false, nil
	}
	valueSum, err := f64ProducerSum(producer)
	if err != nil {
		return nil, true, err
	}
	exponentSum, err := f64ProducerSum(producer.right)
	if err != nil {
		return nil, true, err
	}
	return valueSum + exponentSum, true, nil
}

func f64DyadicScalarNestedProducerSum(scalar float64, producer f64DyadicProducer, scalarLeft bool, op string, apply f64DyadicFunc) (float64, error) {
	if reducer, ok := newF64DyadicScalarReducer(op, scalar, scalarLeft); ok {
		if total, handled, err := f64DyadicScalarNestedProducerSumFast(producer, reducer); handled || err != nil {
			return total, err
		}
	}
	var total float64
	var innerCache f64DyadicEvalCache
	var outerCache f64DyadicEvalCache
	for row := 0; row < producer.len; row++ {
		value, ok, err := producer.f64AtCached(row, &innerCache)
		if err != nil {
			return 0, err
		}
		if ok {
			total += applyScalarDyadicFloatCached(scalar, value, scalarLeft, apply, &outerCache)
		}
	}
	return total, nil
}

func applyScalarDyadicFloat(scalar, value float64, scalarLeft bool, apply f64DyadicFunc) float64 {
	if scalarLeft {
		return apply(scalar, value)
	}
	return apply(value, scalar)
}

func f64DyadicRatiosSumFloat(producer f64DyadicProducer) (float64, error) {
	if left, ok := producer.left.(f64ScalarProducer); ok {
		return f64DyadicScalarProducerRatiosSum(left.value, producer.right, true, producer.op, producer.apply)
	}
	if right, ok := producer.right.(f64ScalarProducer); ok {
		return f64DyadicScalarProducerRatiosSum(right.value, producer.left, false, producer.op, producer.apply)
	}
	var total float64
	var previous float64
	var hasPrevious bool
	var cache f64DyadicEvalCache
	for row := 0; row < producer.len; row++ {
		current, ok, err := producer.f64AtCached(row, &cache)
		if err != nil {
			return 0, err
		}
		if !ok {
			hasPrevious = false
			continue
		}
		if !hasPrevious {
			total += current
		} else {
			total += current / previous
		}
		previous = current
		hasPrevious = true
	}
	return total, nil
}

func f64DyadicScalarProducerRatiosSum(scalar float64, producer f64NumericProducer, scalarLeft bool, op string, apply f64DyadicFunc) (float64, error) {
	if reducer, ok := newF64DyadicScalarReducer(op, scalar, scalarLeft); ok {
		if total, handled, err := f64DyadicScalarProducerRatiosSumFast(producer, reducer); handled || err != nil {
			return total, err
		}
	}
	switch p := producer.(type) {
	case f64NullProducer:
		return 0, nil
	case f64ScalarProducer:
		if p.len <= 0 {
			return 0, nil
		}
		value := applyScalarDyadicFloat(scalar, p.value, scalarLeft, apply)
		return value + float64(p.len-1), nil
	case f64BroadcastProducer:
		value, ok, err := p.source.f64At(0)
		if err != nil || !ok {
			return 0, err
		}
		if p.len <= 0 {
			return 0, nil
		}
		current := applyScalarDyadicFloat(scalar, value, scalarLeft, apply)
		return current + float64(p.len-1), nil
	case f64I64ScalarDyadicProducer:
		if total, ok, err := f64DyadicScalarI64ScalarDyadicRangeRatiosSum(scalar, p.values, scalarLeft, apply); ok || err != nil {
			return total, err
		}
	case f64DyadicProducer:
		return f64DyadicScalarNestedProducerRatiosSum(scalar, p, scalarLeft, apply)
	}
	var total float64
	var previous float64
	var hasPrevious bool
	var cache f64DyadicEvalCache
	for row := 0; row < producer.Len(); row++ {
		value, ok, err := producer.f64At(row)
		if err != nil {
			return 0, err
		}
		if !ok {
			hasPrevious = false
			continue
		}
		current := applyScalarDyadicFloatCached(scalar, value, scalarLeft, apply, &cache)
		if !hasPrevious {
			total += current
		} else {
			total += current / previous
		}
		previous = current
		hasPrevious = true
	}
	return total, nil
}

func f64DyadicScalarNestedProducerRatiosSum(scalar float64, producer f64DyadicProducer, scalarLeft bool, apply f64DyadicFunc) (float64, error) {
	var total float64
	var previous float64
	var hasPrevious bool
	var innerCache f64DyadicEvalCache
	var outerCache f64DyadicEvalCache
	for row := 0; row < producer.len; row++ {
		value, ok, err := producer.f64AtCached(row, &innerCache)
		if err != nil {
			return 0, err
		}
		if !ok {
			hasPrevious = false
			continue
		}
		current := applyScalarDyadicFloatCached(scalar, value, scalarLeft, apply, &outerCache)
		if !hasPrevious {
			total += current
		} else {
			total += current / previous
		}
		previous = current
		hasPrevious = true
	}
	return total, nil
}

func f64DyadicScalarI64ScalarDyadicRangeSum(scalar float64, values i64ScalarDyadicArray, scalarLeft bool, apply f64DyadicFunc) (float64, bool, error) {
	source, ok := values.source.(i64RangeArray)
	if !ok || source.len != values.len {
		return 0, false, nil
	}
	var total float64
	var cache f64DyadicEvalCache
	for row := 0; row < values.len; row++ {
		value, err := f64I64ScalarDyadicRangeValue(source, values, row)
		if err != nil {
			return 0, true, err
		}
		total += applyScalarDyadicFloatCached(scalar, value, scalarLeft, apply, &cache)
	}
	return total, true, nil
}

func f64DyadicScalarI64ScalarDyadicRangeRatiosSum(scalar float64, values i64ScalarDyadicArray, scalarLeft bool, apply f64DyadicFunc) (float64, bool, error) {
	source, ok := values.source.(i64RangeArray)
	if !ok || source.len != values.len {
		return 0, false, nil
	}
	var total float64
	var previous float64
	var cache f64DyadicEvalCache
	for row := 0; row < values.len; row++ {
		value, err := f64I64ScalarDyadicRangeValue(source, values, row)
		if err != nil {
			return 0, true, err
		}
		current := applyScalarDyadicFloatCached(scalar, value, scalarLeft, apply, &cache)
		if row == 0 {
			total += current
		} else {
			total += current / previous
		}
		previous = current
	}
	return total, true, nil
}

func f64I64ScalarDyadicRangeValue(source i64RangeArray, values i64ScalarDyadicArray, row int) (float64, error) {
	if row < 0 || row >= values.len || row >= source.len {
		return 0, fmt.Errorf("array row %d out of range", row)
	}
	sourceValue := source.start + int64(row)*source.step
	value, ok, err := applyI64ScalarDyadicValue(values.op, sourceValue, values.scalar, values.scalarLeft)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("unsupported i64 scalar dyadic op %s", values.op)
	}
	return float64(value), nil
}

func applyScalarDyadicFloatCached(scalar, value float64, scalarLeft bool, apply f64DyadicFunc, cache *f64DyadicEvalCache) float64 {
	if scalarLeft {
		return cache.apply(apply, scalar, value)
	}
	return cache.apply(apply, value, scalar)
}

func (c *f64DyadicEvalCache) apply(apply f64DyadicFunc, left, right float64) float64 {
	for i := range c.entries {
		entry := c.entries[i]
		if entry.valid && entry.left == left && entry.right == right {
			return entry.value
		}
	}
	value := apply(left, right)
	slot := c.next & (len(c.entries) - 1)
	c.entries[slot] = f64DyadicEvalCacheEntry{left: left, right: right, value: value, valid: true}
	c.next++
	return value
}

func numericDyadicLength(name string, left, right Array) (int, error) {
	switch {
	case left != nil && right != nil:
		switch {
		case left.Len() == right.Len():
			return left.Len(), nil
		case left.Len() == 1:
			return right.Len(), nil
		case right.Len() == 1:
			return left.Len(), nil
		default:
			return 0, fmt.Errorf("%s vector length mismatch", name)
		}
	case left != nil:
		return left.Len(), nil
	case right != nil:
		return right.Len(), nil
	default:
		return 0, nil
	}
}

func numericDyadicBroadcastRow(array Array, row int) int {
	if array.Len() == 1 {
		return 0
	}
	return row
}

func applyNumericDyadicFloatValue(op string, left, right any) (any, bool, error) {
	if IsNull(left) || IsNull(right) {
		return NullValue, true, nil
	}
	ln, lok := numeric(left)
	rn, rok := numeric(right)
	if !lok || !rok {
		return nil, false, nil
	}
	out, err := applyNumericDyadicFloatNumbers(op, ln, rn)
	if err != nil {
		return nil, true, err
	}
	return out, true, nil
}

func applyNumericDyadicFloatNumbers(op string, left, right float64) (float64, error) {
	switch op {
	case string(OpAdd):
		return left + right, nil
	case string(OpSub):
		return left - right, nil
	case string(OpMul):
		return left * right, nil
	case string(OpDiv):
		return left / right, nil
	case string(OpMod):
		if right == 0 {
			return math.NaN(), nil
		}
		return left - right*math.Floor(left/right), nil
	case NumericDyadicXExp:
		return math.Pow(left, right), nil
	case NumericDyadicXLog:
		return math.Log(right) / math.Log(left), nil
	default:
		return 0, fmt.Errorf("unsupported numeric dyadic float kernel %q", op)
	}
}

type f64DyadicFunc func(left, right float64) float64

func numericDyadicFloatFuncForXExp(left, right float64) float64 {
	return math.Pow(left, right)
}

func numericDyadicFloatFuncForXLog(left, right float64) float64 {
	return math.Log(right) / math.Log(left)
}

func numericDyadicFloatFunc(op string) (f64DyadicFunc, bool) {
	switch op {
	case string(OpAdd):
		return func(left, right float64) float64 { return left + right }, true
	case string(OpSub):
		return func(left, right float64) float64 { return left - right }, true
	case string(OpMul):
		return func(left, right float64) float64 { return left * right }, true
	case string(OpDiv):
		return func(left, right float64) float64 { return left / right }, true
	case string(OpMod):
		return func(left, right float64) float64 {
			if right == 0 {
				return math.NaN()
			}
			return left - right*math.Floor(left/right)
		}, true
	case NumericDyadicXExp:
		return numericDyadicFloatFuncForXExp, true
	case NumericDyadicXLog:
		return numericDyadicFloatFuncForXLog, true
	default:
		return nil, false
	}
}

func TryTypedQNumericUnary(op string, array Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedQNumericUnary(op, a.array)
	case nullableArray:
		if qNumericUnaryReturnsFloat(op) {
			return numericUnaryNullable(op, a.data)
		}
	case nullBitmapCarrier:
		if qNumericUnaryReturnsFloat(op) {
			return numericUnaryNullBitmap(op, a)
		}
		return nil, false, nil
	case f64RangeArray:
		return qNumericUnaryFloatArray(op, a)
	case f64RunningSumArray:
		return qNumericUnaryFloatArray(op, a)
	case f64BucketArray:
		return qNumericUnaryFloatArray(op, a)
	case columnArray[float32]:
		return qNumericUnaryFloatSlice(op, a.data)
	case columnArray[float64]:
		return qNumericUnaryFloatSlice(op, a.data)
	}
	if !isDenseIntegerArray(array) {
		if qNumericUnaryReturnsFloat(op) && isNumericArray(array) {
			return qNumericUnaryFloatArray(op, array)
		}
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
	case f64RangeArray:
		return qNumericUnarySumFloatArray(op, a)
	case f64RunningSumArray:
		return qNumericUnarySumFloatArray(op, a)
	case f64BucketArray:
		return qNumericUnarySumFloatArray(op, a)
	case columnArray[float32]:
		return qNumericUnarySumFloatSlice(op, a.data)
	case columnArray[float64]:
		return qNumericUnarySumFloatSlice(op, a.data)
	}
	if !isDenseIntegerArray(array) {
		if qNumericUnaryReturnsFloat(op) && isNumericArray(array) {
			return qNumericUnarySumFloatArray(op, array)
		}
		return nil, false, nil
	}
	return qNumericUnarySumIntegerArray(op, array)
}

func TryTypedQNumericUnaryCompareIndexes(op string, array Array, compareOp Op, scalar any) (Array, bool, error) {
	transformed, handled, err := TryTypedQNumericUnary(op, array)
	if err != nil || !handled {
		return nil, handled, err
	}
	return TryTypedCompareIndexesI64(transformed, compareOp, scalar)
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

// TryTypedDyadicMinMaxSum reduces min/max applied pairwise to numeric operands
// without materializing the intermediate dyadic vector.
func TryTypedDyadicMinMaxSum(left, right any, wantMax bool) (any, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray && !rightIsArray {
		return nil, false, nil
	}
	length := 0
	switch {
	case leftIsArray && rightIsArray:
		switch {
		case leftArray.Len() == rightArray.Len():
			length = leftArray.Len()
		case leftArray.Len() == 1:
			length = rightArray.Len()
		case rightArray.Len() == 1:
			length = leftArray.Len()
		default:
			// Same error text the generic vector dyadic raises, so fused and
			// generic routes fail identically on mismatched operands.
			return nil, true, fmt.Errorf("vector length mismatch")
		}
	case leftIsArray:
		length = leftArray.Len()
	case rightIsArray:
		length = rightArray.Len()
	}
	if !typedNumericOperand(left) || !typedNumericOperand(right) {
		return nil, false, nil
	}
	if typedIntegerOperand(left) && typedIntegerOperand(right) {
		if sum, ok := i64RangeDyadicMinMaxSum(left, right, length, wantMax); ok {
			return sum, true, nil
		}
		var sum int64
		for row := 0; row < length; row++ {
			lv, lok, err := integerMinMaxOperandAt(left, row, length)
			if err != nil {
				return nil, true, err
			}
			rv, rok, err := integerMinMaxOperandAt(right, row, length)
			if err != nil {
				return nil, true, err
			}
			// q min/max null rule: a null yields the OTHER operand (the
			// generic minDyadic/maxDyadic behavior); both-null rows skip.
			if !lok && !rok {
				continue
			}
			if !lok {
				sum += rv
				continue
			}
			if !rok {
				sum += lv
				continue
			}
			if wantMax {
				if rv > lv {
					lv = rv
				}
			} else if rv < lv {
				lv = rv
			}
			sum += lv
		}
		return sum, true, nil
	}
	var sum float64
	for row := 0; row < length; row++ {
		lv, lok, err := numericMinMaxOperandAt(left, row, length)
		if err != nil {
			return nil, true, err
		}
		rv, rok, err := numericMinMaxOperandAt(right, row, length)
		if err != nil {
			return nil, true, err
		}
		// Null rule: see the integer loop above.
		if !lok && !rok {
			continue
		}
		if !lok {
			sum += rv
			continue
		}
		if !rok {
			sum += lv
			continue
		}
		if wantMax {
			if rv > lv {
				lv = rv
			}
		} else if rv < lv {
			lv = rv
		}
		sum += lv
	}
	return sum, true, nil
}

func i64RangeDyadicMinMaxSum(left, right any, length int, wantMax bool) (int64, bool) {
	leftRange, leftOK := i64RangeOperand(left, length)
	rightRange, rightOK := i64RangeOperand(right, length)
	if !leftOK || !rightOK {
		return 0, false
	}
	return i64RangeRangeMinMaxSum(leftRange, rightRange, wantMax), true
}

func i64RangeOperand(value any, length int) (i64RangeArray, bool) {
	if rangeValue, ok := asI64RangeArray(value); ok {
		if rangeValue.len == length {
			return rangeValue, true
		}
		if rangeValue.len == 1 && length != 1 {
			return i64RangeArray{start: rangeValue.start, len: length}, true
		}
		return i64RangeArray{}, false
	}
	if scalar, ok := integerScalarValue(value); ok {
		return i64RangeArray{start: scalar, len: length}, true
	}
	return i64RangeArray{}, false
}

func i64RangeRangeMinMaxSum(left, right i64RangeArray, wantMax bool) int64 {
	n := left.len
	if n <= 0 {
		return 0
	}
	firstDiff := left.start - right.start
	stepDiff := left.step - right.step
	leftTotal := i64RangeSegmentSum(left, 0, n)
	rightTotal := i64RangeSegmentSum(right, 0, n)
	leftWins := func(diff int64) bool {
		if wantMax {
			return diff >= 0
		}
		return diff <= 0
	}
	if stepDiff == 0 {
		if leftWins(firstDiff) {
			return leftTotal
		}
		return rightTotal
	}
	lastDiff := firstDiff + int64(n-1)*stepDiff
	firstWins := leftWins(firstDiff)
	lastWins := leftWins(lastDiff)
	if firstWins && lastWins {
		return leftTotal
	}
	if !firstWins && !lastWins {
		return rightTotal
	}
	cut := sort.Search(n, func(i int) bool {
		diff := firstDiff + int64(i)*stepDiff
		return leftWins(diff) == lastWins
	})
	if lastWins {
		return i64RangeSegmentSum(right, 0, cut) + i64RangeSegmentSum(left, cut, n-cut)
	}
	return i64RangeSegmentSum(left, 0, cut) + i64RangeSegmentSum(right, cut, n-cut)
}

func i64RangeSegmentSum(array i64RangeArray, start int, length int) int64 {
	if length <= 0 {
		return 0
	}
	first := array.start + int64(start)*array.step
	last := first + int64(length-1)*array.step
	n := int64(length)
	endpoints := first + last
	if n%2 == 0 {
		return (n / 2) * endpoints
	}
	return n * (endpoints / 2)
}

func integerMinMaxOperandAt(value any, row int, length int) (int64, bool, error) {
	if array, ok := value.(Array); ok && array.Len() == 1 && length != 1 {
		row = 0
	}
	return integerOperandAt(value, row)
}

func numericMinMaxOperandAt(value any, row int, length int) (float64, bool, error) {
	if array, ok := value.(Array); ok && array.Len() == 1 && length != 1 {
		row = 0
	}
	return numericOperandAt(value, row)
}

// tryTypedCastBulk lowers numeric casts over bulk-flattenable carriers to
// dense loops while keeping TryTypedCast's exact range-check errors and null
// fallback semantics. Shapes with dedicated lazy fast paths (i64 range to
// f64) are left to the caller.
func tryTypedCastBulk(kind Kind, array Array) (Array, bool, error) {
	switch kind {
	case KindI16, KindI32, KindI64:
		if kind == KindI64 && array.Kind() == KindI64 && castLazyNumericSource(array) {
			// Identity cast over an immutable null-free integer carrier.
			return array, true, nil
		}
		if values, owned, ok := tryBulkI64Values(array); ok {
			out, handled, err := castBulkI64Values(kind, values)
			bulkI64Release(values, owned)
			return out, handled, err
		}
		if !isIntegerArray(array) {
			if kind == KindI64 && castLazyNumericSource(array) {
				if out, handled := tryLazyCastI64(array); handled {
					return out, true, nil
				}
			}
			if values, owned, ok := tryBulkF64Values(array); ok {
				truncated := bulkI64Get(len(values))
				for i, value := range values {
					if math.IsNaN(value) || math.IsInf(value, 0) || value < -9223372036854775808.0 || value >= 9223372036854775808.0 {
						bulkF64Release(values, owned)
						bulkI64Release(truncated, true)
						return nil, false, nil
					}
					// Canonical q integer casts round half-to-even.
					truncated[i] = int64(math.RoundToEven(value))
				}
				bulkF64Release(values, owned)
				out, handled, err := castBulkI64Values(kind, truncated)
				bulkI64Release(truncated, true)
				return out, handled, err
			}
		}
		return nil, false, nil
	case KindF32:
		if array.Kind() == KindF32 && castLazyNumericSource(array) {
			return array, true, nil
		}
		if castLazyNumericSource(array) {
			if out, handled := tryLazyCastF32(array); handled {
				return out, true, nil
			}
		}
		if values, owned, ok := tryBulkF64Values(array); ok {
			out := make([]float32, len(values))
			for i, value := range values {
				out[i] = float32(value)
			}
			bulkF64Release(values, owned)
			return columnArray[float32]{kind: KindF32, data: out}, true, nil
		}
		return nil, false, nil
	case KindF64:
		if _, ok := asI64RangeArray(array); ok {
			return nil, false, nil
		}
		if values, owned, ok := tryBulkF64Values(array); ok {
			out := make([]float64, len(values))
			copy(out, values)
			bulkF64Release(values, owned)
			return newF64Trusted(out), true, nil
		}
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

// castLazyNumericSource reports whether array is an immutable, null-free
// numeric carrier that is cheap to re-stream, making it safe to wrap in a
// lazy cast view instead of materializing a fresh column.
func castLazyNumericSource(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return castLazyNumericSource(a.array)
	case columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[float32], columnArray[float64],
		i64RangeArray, f64RangeArray, castF32Array, castI64Array:
		return true
	case i64ScalarDyadicArray:
		return castLazyNumericSource(a.source)
	case f64NumericDyadicArray:
		// Lazy float dyadic trees over lazy-safe array operands re-stream
		// through the fused bulk flatteners. The lazy-cast validators still
		// flatten-probe once, so null/NaN-producing trees keep the eager
		// fallback semantics.
		if _, ok := numericDyadicFloatFunc(a.op); !ok {
			return false
		}
		if left, ok := a.left.(Array); ok && !castLazyNumericSource(left) {
			return false
		}
		if right, ok := a.right.(Array); ok && !castLazyNumericSource(right) {
			return false
		}
		return true
	default:
		return false
	}
}

// tryLazyCastF32 confirms the source streams through the bulk flatteners
// (boxed nulls bail out, keeping the eager fallback semantics) and returns a
// lazy `real$` view.
func tryLazyCastF32(array Array) (Array, bool) {
	values, owned, ok := tryBulkF64Values(array)
	if !ok {
		return nil, false
	}
	bulkF64Release(values, owned)
	return castF32Array{source: array}, true
}

// tryLazyCastI64 validates a float carrier once (preserving eager `long$`
// error and fallback semantics) and returns a lazy i64 cast view on success.
func tryLazyCastI64(array Array) (Array, bool) {
	values, owned, ok := tryBulkF64Values(array)
	if !ok {
		return nil, false
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < -9223372036854775808.0 || value >= 9223372036854775808.0 {
			bulkF64Release(values, owned)
			return nil, false
		}
	}
	bulkF64Release(values, owned)
	return castI64Array{source: array}, true
}

func castBulkI64Values(kind Kind, values []int64) (Array, bool, error) {
	switch kind {
	case KindI16:
		out := make([]int16, len(values))
		for row, value := range values {
			if value < -32768 || value > 32767 {
				return nil, true, fmt.Errorf("value %d must be i16 for %s", row+1, kind)
			}
			out[row] = int16(value)
		}
		return columnArray[int16]{kind: KindI16, data: out}, true, nil
	case KindI32:
		out := make([]int32, len(values))
		for row, value := range values {
			if value < -2147483648 || value > 2147483647 {
				return nil, true, fmt.Errorf("value %d must be i32 for %s", row+1, kind)
			}
			out[row] = int32(value)
		}
		return columnArray[int32]{kind: KindI32, data: out}, true, nil
	case KindI64:
		out := make([]int64, len(values))
		copy(out, values)
		return newI64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}

func TryTypedCast(kind Kind, array Array) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("typed cast array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedCast(kind, a.array)
	}
	if kind == KindBool {
		return tryTypedBoolCast(array)
	}
	if !isIntegerArray(array) && !isNumericArray(array) {
		return nil, false, nil
	}
	if out, handled, err := tryTypedCastBulk(kind, array); handled || err != nil {
		return out, handled, err
	}
	switch kind {
	case KindI16:
		out := make([]int16, array.Len())
		for row := 0; row < array.Len(); row++ {
			value, ok, err := numericArrayTruncatedIntegerAt(array, row)
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
			value, ok, err := numericArrayTruncatedIntegerAt(array, row)
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
			value, ok, err := numericArrayTruncatedIntegerAt(array, row)
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
			value, ok, err := typedKernels.NumericAt(array, row)
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

// tryTypedBoolCast lowers `"b"$xs` for boolean and 0/1 integer vectors. The q
// bool cast only accepts 0 and 1, so any other value defers to the generic
// fallback. Integer scalar-dyadic carriers (x mod 2, ...) stay lazy as
// compare masks so where/count over the result keep periodic fast paths.
func tryTypedBoolCast(array Array) (Array, bool, error) {
	if array.Kind() == KindBool {
		return array, true, nil
	}
	if !isIntegerArray(array) {
		return nil, false, nil
	}
	// q-mod carriers over null-free integer sources have a provable [0,m-1]
	// value domain, so `"b"$x mod 2` stays lazy without a validation scan.
	if dyadic, isDyadic := array.(i64ScalarDyadicArray); isDyadic &&
		dyadic.op == OpMod && !dyadic.scalarLeft && (dyadic.scalar == 1 || dyadic.scalar == 2) {
		switch dyadic.source.(type) {
		case i64RangeArray, i64SegmentArray:
			return i64ScalarDyadicCompareMask{values: dyadic, op: OpNE, scalar: 0}, true, nil
		}
	}
	values, owned, ok := tryBulkI64Values(array)
	if !ok {
		return nil, false, nil
	}
	for _, v := range values {
		if v != 0 && v != 1 {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
	}
	if dyadic, isDyadic := array.(i64ScalarDyadicArray); isDyadic {
		bulkI64Release(values, owned)
		return i64ScalarDyadicCompareMask{values: dyadic, op: OpNE, scalar: 0}, true, nil
	}
	out := make([]bool, len(values))
	for i, v := range values {
		out[i] = v == 1
	}
	bulkI64Release(values, owned)
	return newBoolTrusted(out), true, nil
}

func numericArrayTruncatedIntegerAt(array Array, row int) (int64, bool, error) {
	if isIntegerArray(array) {
		return integerArrayAt(array, row)
	}
	value, ok, err := typedKernels.NumericAt(array, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value < -9223372036854775808.0 || value >= 9223372036854775808.0 {
		return 0, false, nil
	}
	// Canonical q integer casts round half-to-even.
	return int64(math.RoundToEven(value)), true, nil
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
			if leftArray.Len() == 1 || rightArray.Len() == 1 {
				// q vector rules broadcast a 1-element side; the typed
				// kernels do not implement that, so decline to the generic
				// route instead of erroring.
				return nil, false, nil
			}
			// Same error text the generic vector dyadic raises.
			return nil, true, fmt.Errorf("vector length mismatch")
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
	case indexedArray:
		return numericSumIndexed(a)
	case tiledArray:
		if sum, count, handled, err := numericSumTiled(a); handled || err != nil {
			return sum, count, handled, err
		}
		return numericSumByAccess(a)
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
	case i64BucketArray:
		sum, handled, err := i64BucketSum(a)
		return float64(sum), int64(a.Len()), handled, err
	case i64XrankArray:
		sum, handled, err := i64XrankSum(a)
		return float64(sum), int64(a.Len()), handled, err
	case i64SparseAmendArray:
		sum, handled, err := i64SparseAmendSum(a)
		return float64(sum), int64(a.Len()), handled, err
	case i64FillArray:
		return float64(a.sum()), int64(a.Len()), true, nil
	case fbyI64BroadcastArray:
		return float64(a.total()), int64(a.len), true, nil
	case fbyI64TiledBroadcastArray:
		return float64(a.total()), int64(a.len), true, nil
	case f64RangeArray:
		return f64RangeSum(a), int64(a.len), true, nil
	case f64BucketArray:
		sum, handled, err := f64BucketSum(a)
		return sum, int64(a.Len()), handled, err
	case f64FillArray:
		return a.sum(), int64(a.Len()), true, nil
	case fbyF64BroadcastArray:
		return a.total(), int64(a.len), true, nil
	case fbyF64TiledBroadcastArray:
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
	case i64DyadicProductArray:
		sum, handled, err := i64DyadicProductSum(a)
		return float64(sum), int64(a.Len()), handled, err
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
		return numericSumByAccess(array)
	}
}

func numericSumByAccess(array Array) (float64, int64, bool, error) {
	if array == nil || !isNumericArray(array) {
		return 0, 0, false, nil
	}
	var sum float64
	var count int64
	for row := 0; row < array.Len(); row++ {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return 0, 0, true, err
		}
		if !ok {
			continue
		}
		sum += value
		count++
	}
	return sum, count, true, nil
}

func numericSumIndexed(array indexedArray) (float64, int64, bool, error) {
	if !isNumericArray(array.source) {
		return 0, 0, false, nil
	}
	var sum float64
	var count int64
	for row := 0; row < array.len; row++ {
		index, ok, err := i64IndexArrayAt(array.indexes, row)
		if err != nil || !ok {
			return 0, 0, ok, err
		}
		value, ok, err := typedKernels.NumericAt(array.source, index)
		if err != nil {
			return 0, 0, true, err
		}
		if !ok {
			continue
		}
		sum += value
		count++
	}
	return sum, count, true, nil
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

// TryTypedNumericSumFirstLast reduces sum(array)+first(array)+last(array)
// without materializing intermediate sequence views. It is intentionally
// array-oriented so language frontends can reuse it for recognized reducer
// chains over lazy sequence transforms.
func TryTypedNumericSumFirstLast(array Array) (any, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("sum-first-last array is nil")
	}
	if array.Len() == 0 {
		return nil, false, nil
	}
	sum, handled, err := typedKernels.NumericSumValue(array)
	if err != nil || !handled {
		return nil, handled, err
	}
	first, ok, err := numericEdgeValue(array, 0)
	if err != nil || !ok {
		return nil, ok, err
	}
	last, ok, err := numericEdgeValue(array, array.Len()-1)
	if err != nil || !ok {
		return nil, ok, err
	}
	if total, ok := numericAdd3(sum, first, last); ok {
		return total, true, nil
	}
	return nil, false, nil
}

func numericEdgeValue(array Array, row int) (any, bool, error) {
	if isIntegerArray(array) {
		value, ok, err := integerArrayAt(array, row)
		if err != nil || !ok {
			return nil, ok, err
		}
		return value, true, nil
	}
	value, ok := array.At(row)
	if !ok {
		return nil, false, fmt.Errorf("array row %d out of range", row)
	}
	if IsNull(value) {
		return nil, false, nil
	}
	if _, ok := numeric(value); !ok {
		return nil, false, nil
	}
	return value, true, nil
}

func numericAdd3(a, b, c any) (any, bool) {
	ai, aInt := coerceInt64Exact(a)
	bi, bInt := coerceInt64Exact(b)
	ci, cInt := coerceInt64Exact(c)
	if aInt && bInt && cInt {
		return ai + bi + ci, true
	}
	af, ok := numeric(a)
	if !ok {
		return nil, false
	}
	bf, ok := numeric(b)
	if !ok {
		return nil, false
	}
	cf, ok := numeric(c)
	if !ok {
		return nil, false
	}
	return af + bf + cf, true
}

// TryTypedIntegerDyadicSum reduces sum(left op right) for integer operands
// without materializing the dyadic result vector. It is a reusable reducer
// primitive for q `+/x op y` and for JIT backends that recognize the same
// vector-dyadic-reduce shape.
func TryTypedIntegerDyadicSum(op Op, left, right any) (any, bool, error) {
	switch op {
	case OpAdd, OpSub, OpMul:
	default:
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
			return nil, true, fmt.Errorf("typed integer dyadic sum length mismatch: %d != %d", leftArray.Len(), rightArray.Len())
		}
		length = leftArray.Len()
	case leftIsArray:
		length = leftArray.Len()
	case rightIsArray:
		length = rightArray.Len()
	}
	if length == 0 {
		return NullValue, true, nil
	}
	if !typedIntegerOperand(left) || !typedIntegerOperand(right) {
		return nil, false, nil
	}
	if leftIsArray && rightIsArray {
		leftValues, leftOwned, ok := tryBulkI64Values(leftArray)
		if !ok || len(leftValues) < length {
			bulkI64Release(leftValues, leftOwned)
			return nil, false, nil
		}
		rightValues, rightOwned, ok := tryBulkI64Values(rightArray)
		if !ok || len(rightValues) < length {
			bulkI64Release(leftValues, leftOwned)
			bulkI64Release(rightValues, rightOwned)
			return nil, false, nil
		}
		var total int64
		switch op {
		case OpAdd:
			for i := 0; i < length; i++ {
				total += leftValues[i] + rightValues[i]
			}
		case OpSub:
			for i := 0; i < length; i++ {
				total += leftValues[i] - rightValues[i]
			}
		case OpMul:
			for i := 0; i < length; i++ {
				total += leftValues[i] * rightValues[i]
			}
		}
		bulkI64Release(leftValues, leftOwned)
		bulkI64Release(rightValues, rightOwned)
		return total, true, nil
	}
	array := leftArray
	scalar := right
	scalarLeft := false
	if rightIsArray {
		array = rightArray
		scalar = left
		scalarLeft = true
	}
	scalarValue, ok := integerScalarValue(scalar)
	if !ok {
		return nil, false, nil
	}
	values, owned, ok := tryBulkI64Values(array)
	if !ok || len(values) < length {
		bulkI64Release(values, owned)
		return nil, false, nil
	}
	var total int64
	switch op {
	case OpAdd:
		for i := 0; i < length; i++ {
			total += values[i] + scalarValue
		}
	case OpSub:
		if scalarLeft {
			for i := 0; i < length; i++ {
				total += scalarValue - values[i]
			}
		} else {
			for i := 0; i < length; i++ {
				total += values[i] - scalarValue
			}
		}
	case OpMul:
		for i := 0; i < length; i++ {
			total += values[i] * scalarValue
		}
	}
	bulkI64Release(values, owned)
	return total, true, nil
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

// TryTypedXrank returns q xrank buckets for typed integer arrays without
// materializing []any values or sorting when the source shape is already
// describable as a compact integer bucket domain.
func TryTypedXrank(bucketCount int64, array Array) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("xrank array must be non-nil")
	}
	if bucketCount <= 0 || int64(int(bucketCount)) != bucketCount {
		return nil, true, fmt.Errorf("xrank expects a positive integer bucket count")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedXrank(bucketCount, a.array)
	case columnArray[int8]:
		return xrankSignedSlice(a.data, bucketCount), true, nil
	case columnArray[int16]:
		return xrankSignedSlice(a.data, bucketCount), true, nil
	case columnArray[int32]:
		return xrankSignedSlice(a.data, bucketCount), true, nil
	case columnArray[int64]:
		return xrankSignedSlice(a.data, bucketCount), true, nil
	case i64ScalarDyadicArray:
		if a.op == OpMod && !a.scalarLeft && a.scalar > 0 {
			return i64XrankArray{source: a, bucketCount: bucketCount, domainSize: a.scalar, len: a.Len()}, true, nil
		}
	case i64RangeArray:
		if a.step == 1 && a.start == 0 {
			return i64XrankArray{source: a, bucketCount: bucketCount, domainSize: int64(a.len), len: a.Len()}, true, nil
		}
	}
	return nil, false, nil
}

func xrankSignedSlice[T signedScalar](values []T, bucketCount int64) Array {
	out := make([]int64, len(values))
	if len(values) == 0 {
		return newI64Trusted(out)
	}
	if len(values) > math.MaxInt32 {
		return xrankSignedSliceStableSort(values, bucketCount, out)
	}
	keys := bulkU64Get(len(values))
	defer bulkU64Release(keys)
	for i, value := range values {
		keys[i] = uint64(int64(value)) ^ (1 << 63)
	}
	indexes := stableKeyPermutationI32(keys)
	if indexes == nil {
		return newI64Trusted(out)
	}
	rank := 0
	for rank < len(indexes) {
		next := rank + 1
		for next < len(indexes) && values[indexes[next]] == values[indexes[rank]] {
			next++
		}
		bucket := int64(rank) * bucketCount / int64(len(indexes))
		if bucket >= bucketCount {
			bucket = bucketCount - 1
		}
		for _, index := range indexes[rank:next] {
			out[index] = bucket
		}
		rank = next
	}
	return newI64Trusted(out)
}

func xrankSignedSliceStableSort[T signedScalar](values []T, bucketCount int64, out []int64) Array {
	indexes := make([]int, len(values))
	for i := range indexes {
		indexes[i] = i
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		return values[indexes[i]] < values[indexes[j]]
	})
	rank := 0
	for rank < len(indexes) {
		next := rank + 1
		for next < len(indexes) && values[indexes[next]] == values[indexes[rank]] {
			next++
		}
		bucket := int64(rank) * bucketCount / int64(len(indexes))
		if bucket >= bucketCount {
			bucket = bucketCount - 1
		}
		for _, index := range indexes[rank:next] {
			out[index] = bucket
		}
		rank = next
	}
	return newI64Trusted(out)
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

// TryTypedNumericSumCountWhereCompare reduces a numeric value array over rows
// selected by a typed comparison predicate without materializing a mask, index
// vector, or gathered value vector.
func TryTypedNumericSumCountWhereCompare(values, predicate Array, op Op, scalar any) (any, int64, bool, error) {
	if values == nil || predicate == nil {
		return nil, 0, true, fmt.Errorf("sum count where compare arrays must be non-nil")
	}
	if values.Len() != predicate.Len() {
		return nil, 0, true, fmt.Errorf("sum count where compare length mismatch: values=%d predicate=%d", values.Len(), predicate.Len())
	}
	scalar = normalizeScalar(predicate.Kind(), scalar)
	if sum, count, ok := bulkNumericSumCountWhereMask(values, predicate, func(maskSource Array, out []bool) bool {
		return typedKernels.CompareMask(maskSource, op, scalar, out)
	}); ok {
		return sum, count, true, nil
	}
	pred, handled, err := typedCompareRowPredicate(predicate, op, scalar)
	if err != nil || !handled {
		return nil, 0, handled, err
	}
	sum, count, err := typedNumericSumCountWherePredicate(values, pred)
	if err == errUnsupportedNumericSumCountWhereValues {
		return nil, 0, false, nil
	}
	return sum, count, true, err
}

// TryTypedNumericSumCountWhereCompareSelf is the predicate==values form of
// TryTypedNumericSumCountWhereCompare: the carrier tree is flattened once and
// shared between the compare mask and the reduction.
func TryTypedNumericSumCountWhereCompareSelf(values Array, op Op, scalar any) (any, int64, bool, error) {
	if values == nil {
		return nil, 0, true, fmt.Errorf("sum count where compare arrays must be non-nil")
	}
	if sum, count, ok, err := TryTypedIntegerSumCountWhereCompareSelf(values, op, scalar); err != nil || ok {
		if err != nil || !ok {
			return nil, count, ok, err
		}
		if count == 0 {
			return NullValue, 0, true, nil
		}
		return sum, count, ok, err
	}
	scalar = normalizeScalar(values.Kind(), scalar)
	if sum, count, ok := f64AffineSumCountWhereCompareSelf(values, op, scalar); ok {
		return sum, count, true, nil
	}
	if sum, count, ok := bulkNumericSumCountWhereMask(values, nil, func(maskSource Array, out []bool) bool {
		return typedKernels.CompareMask(maskSource, op, scalar, out)
	}); ok {
		return sum, count, true, nil
	}
	return TryTypedNumericSumCountWhereCompare(values, values, op, scalar)
}

func TryTypedIntegerSumCountWhereCompareSelf(values Array, op Op, scalar any) (int64, int64, bool, error) {
	if values == nil {
		return 0, 0, true, fmt.Errorf("sum count where compare arrays must be non-nil")
	}
	if sum, count, ok, err := i64RangeSumCountWhereCompareSelf(values, op, scalar); err != nil || ok {
		return sum, count, ok, err
	}
	if sum, count, ok := integerColumnSumCountWhereCompareSelf(values, op, scalar); ok {
		return sum, count, true, nil
	}
	return 0, 0, false, nil
}

func integerColumnSumCountWhereCompareSelf(values Array, op Op, scalar any) (int64, int64, bool) {
	source := unwrapAttributedArray(values)
	target, ok := integerScalarValue(scalar)
	if !ok {
		return 0, 0, false
	}
	switch a := source.(type) {
	case columnArray[int8]:
		return integerSliceSumCountWhereCompareSelf(a.data, op, target)
	case columnArray[int16]:
		return integerSliceSumCountWhereCompareSelf(a.data, op, target)
	case columnArray[int32]:
		return integerSliceSumCountWhereCompareSelf(a.data, op, target)
	case columnArray[int64]:
		return integerSliceSumCountWhereCompareSelf(a.data, op, target)
	case columnArray[uint8]:
		return integerUnsignedSliceSumCountWhereCompareSelf(a.data, op, target)
	case columnArray[uint16]:
		return integerUnsignedSliceSumCountWhereCompareSelf(a.data, op, target)
	case columnArray[uint32]:
		return integerUnsignedSliceSumCountWhereCompareSelf(a.data, op, target)
	case columnArray[uint64]:
		return integerUnsignedSliceSumCountWhereCompareSelf(a.data, op, target)
	default:
		return 0, 0, false
	}
}

func integerSliceSumCountWhereCompareSelf[T signedScalar](values []T, op Op, target int64) (int64, int64, bool) {
	var total int64
	var count int64
	switch op {
	case OpEQ:
		for _, raw := range values {
			value := int64(raw)
			if value == target {
				total += value
				count++
			}
		}
	case OpNE:
		for _, raw := range values {
			value := int64(raw)
			if value != target {
				total += value
				count++
			}
		}
	case OpLT:
		for _, raw := range values {
			value := int64(raw)
			if value < target {
				total += value
				count++
			}
		}
	case OpLE:
		for _, raw := range values {
			value := int64(raw)
			if value <= target {
				total += value
				count++
			}
		}
	case OpGT:
		for _, raw := range values {
			value := int64(raw)
			if value > target {
				total += value
				count++
			}
		}
	case OpGE:
		for _, raw := range values {
			value := int64(raw)
			if value >= target {
				total += value
				count++
			}
		}
	default:
		return 0, 0, false
	}
	return total, count, true
}

func integerUnsignedSliceSumCountWhereCompareSelf[T unsignedScalar](values []T, op Op, target int64) (int64, int64, bool) {
	if target < 0 {
		switch op {
		case OpGT, OpGE, OpNE:
			var total int64
			for _, raw := range values {
				total += int64(raw)
			}
			return total, int64(len(values)), true
		case OpLT, OpLE, OpEQ:
			return 0, 0, true
		}
	}
	var total int64
	var count int64
	targetU := uint64(target)
	switch op {
	case OpEQ:
		for _, raw := range values {
			value := uint64(raw)
			if value == targetU {
				total += int64(value)
				count++
			}
		}
	case OpNE:
		for _, raw := range values {
			value := uint64(raw)
			if value != targetU {
				total += int64(value)
				count++
			}
		}
	case OpLT:
		for _, raw := range values {
			value := uint64(raw)
			if value < targetU {
				total += int64(value)
				count++
			}
		}
	case OpLE:
		for _, raw := range values {
			value := uint64(raw)
			if value <= targetU {
				total += int64(value)
				count++
			}
		}
	case OpGT:
		for _, raw := range values {
			value := uint64(raw)
			if value > targetU {
				total += int64(value)
				count++
			}
		}
	case OpGE:
		for _, raw := range values {
			value := uint64(raw)
			if value >= targetU {
				total += int64(value)
				count++
			}
		}
	default:
		return 0, 0, false
	}
	return total, count, true
}

func i64RangeSumCountWhereCompareSelf(values Array, op Op, scalar any) (int64, int64, bool, error) {
	source := unwrapAttributedArray(values)
	var predicate i64RangeArray
	switch a := source.(type) {
	case i64RangeArray:
		predicate = a
	case i64ScalarDyadicArray:
		rng, ok := i64ScalarDyadicAffineRange(a)
		if !ok {
			return 0, 0, false, nil
		}
		predicate = rng
	default:
		return 0, 0, false, nil
	}
	target, ok := integerScalarValue(scalar)
	if !ok {
		return 0, 0, false, nil
	}
	start, count, ok := i64RangeCompareSelection(predicate, op, target)
	if !ok {
		return 0, 0, false, nil
	}
	if count == 0 {
		return 0, 0, true, nil
	}
	selected := i64RangeArray{
		start: predicate.start + int64(start)*predicate.step,
		step:  predicate.step,
		len:   count,
	}
	return i64RangeSum(selected), int64(count), true, nil
}

func i64RangeCompareSelection(values i64RangeArray, op Op, target int64) (start, count int, ok bool) {
	if values.len <= 0 {
		return 0, 0, true
	}
	if values.step == 0 {
		keep := boolCompare(op, values.start == target, compareInt64(values.start, target))
		if keep {
			return 0, values.len, true
		}
		return 0, 0, true
	}
	if !i64RangeIsMonotonic(values) {
		return 0, 0, false
	}
	switch op {
	case OpGT, OpGE:
		if values.step > 0 {
			first, ok := i64RangeFirstCompare(values, op, target)
			if !ok {
				return 0, 0, false
			}
			return first, values.len - first, true
		}
		stopOp := OpLE
		if op == OpGE {
			stopOp = OpLT
		}
		firstFalse, ok := i64RangeFirstCompare(values, stopOp, target)
		if !ok {
			return 0, 0, false
		}
		return 0, firstFalse, true
	case OpLT, OpLE:
		if values.step > 0 {
			stopOp := OpGE
			if op == OpLE {
				stopOp = OpGT
			}
			firstFalse, ok := i64RangeFirstCompare(values, stopOp, target)
			if !ok {
				return 0, 0, false
			}
			return 0, firstFalse, true
		}
		first, ok := i64RangeFirstCompare(values, op, target)
		if !ok {
			return 0, 0, false
		}
		return first, values.len - first, true
	default:
		return 0, 0, false
	}
}

func i64RangeFirstCompare(values i64RangeArray, op Op, target int64) (int, bool) {
	lo, hi := 0, values.len
	for lo < hi {
		mid := lo + (hi-lo)/2
		offset, ok := checkedI64Mul(int64(mid), values.step)
		if !ok {
			return 0, false
		}
		value, ok := checkedI64Add(values.start, offset)
		if !ok {
			return 0, false
		}
		if boolCompare(op, value == target, compareInt64(value, target)) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo, true
}

// TryTypedNumericSumCountWhereWithinSelf is the predicate==values form of
// TryTypedNumericSumCountWhereWithin: the carrier tree is flattened once and
// shared between the within mask and the reduction.
func TryTypedNumericSumCountWhereWithinSelf(values Array, low, high any, highClosed bool) (any, int64, bool, error) {
	if values == nil {
		return nil, 0, true, fmt.Errorf("sum count where within arrays must be non-nil")
	}
	if !IsNull(low) && !IsNull(high) {
		if sum, count, ok := bulkNumericSumCountWhereMask(values, nil, func(maskSource Array, out []bool) bool {
			return typedKernels.WithinMask(maskSource, low, high, highClosed, out)
		}); ok {
			return sum, count, true, nil
		}
	}
	return TryTypedNumericSumCountWhereWithin(values, values, low, high, highClosed)
}

// TryTypedNumericSumCountWhereWithin reduces a numeric value array over rows
// selected by a typed within predicate without materializing a mask, index
// vector, or gathered value vector.
func TryTypedNumericSumCountWhereWithin(values, predicate Array, low, high any, highClosed bool) (any, int64, bool, error) {
	if values == nil || predicate == nil {
		return nil, 0, true, fmt.Errorf("sum count where within arrays must be non-nil")
	}
	if values.Len() != predicate.Len() {
		return nil, 0, true, fmt.Errorf("sum count where within length mismatch: values=%d predicate=%d", values.Len(), predicate.Len())
	}
	if sum, count, ok := bulkNumericSumCountWhereMask(values, predicate, func(maskSource Array, out []bool) bool {
		return typedKernels.WithinMask(maskSource, low, high, highClosed, out)
	}); ok {
		return sum, count, true, nil
	}
	pred, handled, err := typedWithinRowPredicate(predicate, low, high, highClosed)
	if err != nil || !handled {
		return nil, 0, handled, err
	}
	sum, count, err := typedNumericSumCountWherePredicate(values, pred)
	if err == errUnsupportedNumericSumCountWhereValues {
		return nil, 0, false, nil
	}
	return sum, count, true, err
}

func TryTypedI64IndexExprReducers(indexes Array, reducers []I64IndexExprReducer) ([]int64, bool, error) {
	if indexes == nil {
		return nil, true, fmt.Errorf("index expression indexes must be non-nil")
	}
	if indexes.Kind() != KindI64 {
		return nil, true, fmt.Errorf("index expression indexes kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if len(reducers) == 0 {
		return nil, false, nil
	}
	for _, reducer := range reducers {
		switch reducer.Kind {
		case I64IndexExprReducerSum:
			if !i64IndexExprValid(reducer.Expr) {
				return nil, false, nil
			}
		case I64IndexExprReducerCount:
		default:
			return nil, false, nil
		}
	}
	out := make([]int64, len(reducers))
	// Vectorized path: flatten the index vector once, then evaluate each
	// reducer expression tree over the whole dense index slice (one loop per
	// distinct expression node) instead of re-walking the tree per selected
	// row. Shared subtrees across reducers evaluate once. Wrapping int64 sums
	// accumulate in the same index order as the per-row loop, so results
	// match bit-for-bit.
	if rows, rowsOwned, ok := tryBulkI64Values(indexes); ok {
		valid := true
		for _, row := range rows {
			if row < 0 {
				valid = false
				break
			}
		}
		if valid {
			eval := i64IndexExprBulkEval{rows: rows}
			for i, reducer := range reducers {
				switch reducer.Kind {
				case I64IndexExprReducerSum:
					if value, isConst := i64IndexExprConstValue(reducer.Expr); isConst {
						// n wrapping additions of one constant equal one
						// wrapping multiplication: int64 is a ring mod 2^64.
						out[i] = value * int64(len(rows))
						continue
					}
					values, err := eval.eval(reducer.Expr)
					if err != nil {
						eval.release()
						bulkI64Release(rows, rowsOwned)
						return nil, true, err
					}
					var total int64
					for _, value := range values {
						total += value
					}
					out[i] = total
				case I64IndexExprReducerCount:
					out[i] = int64(len(rows))
				}
			}
			eval.release()
			bulkI64Release(rows, rowsOwned)
			return out, true, nil
		}
		bulkI64Release(rows, rowsOwned)
	}
	if err := forEachTypedI64Index(indexes, int(^uint(0)>>1), func(index int) error {
		for i, reducer := range reducers {
			switch reducer.Kind {
			case I64IndexExprReducerSum:
				value, err := evalI64IndexExpr(reducer.Expr, int64(index))
				if err != nil {
					return err
				}
				out[i] += value
			case I64IndexExprReducerCount:
				out[i]++
			}
		}
		return nil
	}); err != nil {
		return nil, true, err
	}
	return out, true, nil
}

// i64IndexExprBulkEval evaluates integer index expressions over a dense
// index vector with one tight loop per distinct expression node. Structurally
// equal subtrees (within and across reducers) evaluate once and share their
// pooled result slice. Arithmetic mirrors evalI64IndexExpr exactly: wrapping
// int64 ops, Go truncated / and %, and the same divide/modulo-by-zero errors.
type i64IndexExprBulkEval struct {
	rows    []int64
	exprs   []I64IndexExpr
	results [][]int64
}

func i64IndexExprConstValue(expr I64IndexExpr) (int64, bool) {
	if expr.Op == I64IndexExprConst {
		return expr.Value, true
	}
	return 0, false
}

func i64IndexExprEqual(a, b I64IndexExpr) bool {
	if a.Op != b.Op || a.Value != b.Value {
		return false
	}
	if (a.Left == nil) != (b.Left == nil) || (a.Right == nil) != (b.Right == nil) {
		return false
	}
	if a.Left != nil && !i64IndexExprEqual(*a.Left, *b.Left) {
		return false
	}
	if a.Right != nil && !i64IndexExprEqual(*a.Right, *b.Right) {
		return false
	}
	return true
}

// eval returns a slice owned by the evaluator (or the shared rows slice);
// callers must treat it as read-only and call release when finished.
func (e *i64IndexExprBulkEval) eval(expr I64IndexExpr) ([]int64, error) {
	switch expr.Op {
	case I64IndexExprIndex:
		return e.rows, nil
	case I64IndexExprConst:
		out := bulkI64Get(len(e.rows))
		for i := range out {
			out[i] = expr.Value
		}
		e.remember(expr, out)
		return out, nil
	}
	for i := range e.exprs {
		if i64IndexExprEqual(e.exprs[i], expr) {
			return e.results[i], nil
		}
	}
	switch expr.Op {
	case I64IndexExprAdd, I64IndexExprSub, I64IndexExprMul, I64IndexExprDiv, I64IndexExprMod:
		out, err := e.evalDyadic(expr)
		if err != nil {
			return nil, err
		}
		e.remember(expr, out)
		return out, nil
	case I64IndexExprXbar:
		if expr.Value <= 0 {
			return nil, fmt.Errorf("integer index expression xbar width must be positive")
		}
		left, err := e.eval(*expr.Left)
		if err != nil {
			return nil, err
		}
		out := bulkI64Get(len(e.rows))
		for i, v := range left {
			out[i] = floorInt64(v, expr.Value)
		}
		e.remember(expr, out)
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported integer index expression op %d", expr.Op)
	}
}

func (e *i64IndexExprBulkEval) evalDyadic(expr I64IndexExpr) ([]int64, error) {
	rightConst, rightIsConst := i64IndexExprConstValue(*expr.Right)
	left, err := e.eval(*expr.Left)
	if err != nil {
		return nil, err
	}
	out := bulkI64Get(len(e.rows))
	if rightIsConst {
		switch expr.Op {
		case I64IndexExprAdd:
			for i, v := range left {
				out[i] = v + rightConst
			}
		case I64IndexExprSub:
			for i, v := range left {
				out[i] = v - rightConst
			}
		case I64IndexExprMul:
			for i, v := range left {
				out[i] = v * rightConst
			}
		case I64IndexExprDiv:
			if rightConst == 0 && len(left) > 0 {
				bulkI64Release(out, true)
				return nil, fmt.Errorf("integer index expression divide by zero")
			}
			for i, v := range left {
				out[i] = v / rightConst
			}
		case I64IndexExprMod:
			if rightConst == 0 && len(left) > 0 {
				bulkI64Release(out, true)
				return nil, fmt.Errorf("integer index expression modulo by zero")
			}
			for i, v := range left {
				out[i] = v % rightConst
			}
		}
		return out, nil
	}
	right, err := e.eval(*expr.Right)
	if err != nil {
		bulkI64Release(out, true)
		return nil, err
	}
	switch expr.Op {
	case I64IndexExprAdd:
		for i, v := range left {
			out[i] = v + right[i]
		}
	case I64IndexExprSub:
		for i, v := range left {
			out[i] = v - right[i]
		}
	case I64IndexExprMul:
		for i, v := range left {
			out[i] = v * right[i]
		}
	case I64IndexExprDiv:
		for i, v := range left {
			if right[i] == 0 {
				bulkI64Release(out, true)
				return nil, fmt.Errorf("integer index expression divide by zero")
			}
			out[i] = v / right[i]
		}
	case I64IndexExprMod:
		for i, v := range left {
			if right[i] == 0 {
				bulkI64Release(out, true)
				return nil, fmt.Errorf("integer index expression modulo by zero")
			}
			out[i] = v % right[i]
		}
	}
	return out, nil
}

func (e *i64IndexExprBulkEval) remember(expr I64IndexExpr, values []int64) {
	e.exprs = append(e.exprs, expr)
	e.results = append(e.results, values)
}

func (e *i64IndexExprBulkEval) release() {
	for _, values := range e.results {
		bulkI64Release(values, true)
	}
	e.exprs = nil
	e.results = nil
}

func TryTypedI64IndexExprSumCount(indexes Array, expr I64IndexExpr, includeCount bool) (int64, bool, error) {
	reducers := []I64IndexExprReducer{{Kind: I64IndexExprReducerSum, Expr: expr}}
	if includeCount {
		reducers = append(reducers, I64IndexExprReducer{Kind: I64IndexExprReducerCount})
	}
	values, handled, err := TryTypedI64IndexExprReducers(indexes, reducers)
	if err != nil || !handled {
		return 0, handled, err
	}
	var total int64
	for _, value := range values {
		total += value
	}
	return total, true, nil
}

func TryTypedI64IndexExprFbySumTotal(indexes Array, expr I64IndexExpr, groups Array) (int64, bool, error) {
	if indexes == nil || groups == nil {
		return 0, true, fmt.Errorf("index expression fby sum total expects non-nil indexes and groups")
	}
	if indexes.Kind() != KindI64 {
		return 0, true, fmt.Errorf("index expression indexes kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if !i64IndexExprValid(expr) {
		return 0, false, nil
	}
	var lookup fbyGroupLookupFn
	var groupCount int
	var counts []int64
	if sourceGroups, count, start, sourceLen, tiled, err := fbyTiledSourceGroups(groups); err != nil || tiled {
		if err != nil {
			return 0, true, err
		}
		groupCount = count
		counts = bulkI64Get(groupCount)
		clear(counts)
		for sourceRow, group := range sourceGroups {
			row := (int64(sourceRow) - int64(start)) % int64(sourceLen)
			if row < 0 {
				row += int64(sourceLen)
			}
			if row >= int64(groups.Len()) {
				continue
			}
			counts[group] += 1 + (int64(groups.Len())-1-row)/int64(sourceLen)
		}
		lookup = func(row int) (int, error) {
			if row < 0 || row >= groups.Len() {
				return 0, fmt.Errorf("fby group row %d out of range", row)
			}
			return sourceGroups[(start+row)%sourceLen], nil
		}
	} else {
		var ok bool
		var err error
		lookup, groupCount, ok, err = fbyGroupLookup(groups)
		if err != nil || !ok {
			return 0, ok, err
		}
		counts = bulkI64Get(groupCount)
		clear(counts)
		for row := 0; row < groups.Len(); row++ {
			group, err := lookup(row)
			if err != nil {
				bulkI64Release(counts, true)
				return 0, true, err
			}
			counts[group]++
		}
	}
	sums := bulkI64Get(groupCount)
	clear(sums)
	if rows, rowsOwned, ok := tryBulkI64Values(indexes); ok {
		for _, row := range rows {
			if row < 0 || row >= int64(groups.Len()) {
				bulkI64Release(rows, rowsOwned)
				bulkI64Release(sums, true)
				bulkI64Release(counts, true)
				return 0, true, fmt.Errorf("index expression row %d out of range", row)
			}
		}
		eval := i64IndexExprBulkEval{rows: rows}
		values, err := eval.eval(expr)
		if err != nil {
			eval.release()
			bulkI64Release(rows, rowsOwned)
			bulkI64Release(sums, true)
			bulkI64Release(counts, true)
			return 0, true, err
		}
		for i, row := range rows {
			group, err := lookup(int(row))
			if err != nil {
				eval.release()
				bulkI64Release(rows, rowsOwned)
				bulkI64Release(sums, true)
				bulkI64Release(counts, true)
				return 0, true, err
			}
			sums[group] += values[i]
		}
		eval.release()
		bulkI64Release(rows, rowsOwned)
	} else if err := forEachTypedI64Index(indexes, groups.Len(), func(row int) error {
		value, err := evalI64IndexExpr(expr, int64(row))
		if err != nil {
			return err
		}
		group, err := lookup(row)
		if err != nil {
			return err
		}
		sums[group] += value
		return nil
	}); err != nil {
		bulkI64Release(sums, true)
		bulkI64Release(counts, true)
		return 0, true, err
	}
	var total int64
	for group, sum := range sums {
		total += sum * counts[group]
	}
	bulkI64Release(sums, true)
	bulkI64Release(counts, true)
	return total, true, nil
}

func TryTypedI64SelectedExprFbySumTotal(indexes Array, expr I64SelectedExpr, groups Array) (int64, bool, error) {
	if indexes == nil || groups == nil {
		return 0, true, fmt.Errorf("index expression fby sum total expects non-nil indexes and groups")
	}
	if indexes.Kind() != KindI64 {
		return 0, true, fmt.Errorf("index expression indexes kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if !i64SelectedExprValid(expr) {
		return 0, false, nil
	}
	var lookup fbyGroupLookupFn
	var groupCount int
	var counts []int64
	if sourceGroups, count, start, sourceLen, tiled, err := fbyTiledSourceGroups(groups); err != nil || tiled {
		if err != nil {
			return 0, true, err
		}
		groupCount = count
		counts = bulkI64Get(groupCount)
		clear(counts)
		for sourceRow, group := range sourceGroups {
			row := (int64(sourceRow) - int64(start)) % int64(sourceLen)
			if row < 0 {
				row += int64(sourceLen)
			}
			if row >= int64(groups.Len()) {
				continue
			}
			counts[group] += 1 + (int64(groups.Len())-1-row)/int64(sourceLen)
		}
		lookup = func(row int) (int, error) {
			if row < 0 || row >= groups.Len() {
				return 0, fmt.Errorf("fby group row %d out of range", row)
			}
			return sourceGroups[(start+row)%sourceLen], nil
		}
	} else {
		var ok bool
		var err error
		lookup, groupCount, ok, err = fbyGroupLookup(groups)
		if err != nil || !ok {
			return 0, ok, err
		}
		counts = bulkI64Get(groupCount)
		clear(counts)
		for row := 0; row < groups.Len(); row++ {
			group, err := lookup(row)
			if err != nil {
				bulkI64Release(counts, true)
				return 0, true, err
			}
			counts[group]++
		}
	}
	sums := bulkI64Get(groupCount)
	clear(sums)
	if rows, rowsOwned, ok := tryBulkI64Values(indexes); ok {
		for _, row := range rows {
			if row < 0 || row >= int64(groups.Len()) {
				bulkI64Release(rows, rowsOwned)
				bulkI64Release(sums, true)
				bulkI64Release(counts, true)
				return 0, true, fmt.Errorf("index expression row %d out of range", row)
			}
		}
		if err := accumulateSelectedExprFbyRows(rows, expr, lookup, sums); err != nil {
			bulkI64Release(rows, rowsOwned)
			bulkI64Release(sums, true)
			bulkI64Release(counts, true)
			return 0, true, err
		}
		bulkI64Release(rows, rowsOwned)
	} else if err := forEachTypedI64Index(indexes, groups.Len(), func(row int) error {
		value, err := evalI64SelectedExpr(expr, int64(row))
		if err != nil {
			return err
		}
		group, err := lookup(row)
		if err != nil {
			return err
		}
		sums[group] += value
		return nil
	}); err != nil {
		bulkI64Release(sums, true)
		bulkI64Release(counts, true)
		return 0, true, err
	}
	var total int64
	for group, sum := range sums {
		total += sum * counts[group]
	}
	bulkI64Release(sums, true)
	bulkI64Release(counts, true)
	return total, true, nil
}

func TryTypedI64SelectedExprSparseZeroMovingSumAvg(indexes Array, expr I64SelectedExpr, length int, width int) (int64, float64, bool, error) {
	if indexes == nil {
		return 0, 0, true, fmt.Errorf("sparse moving window indexes must be non-nil")
	}
	if indexes.Kind() != KindI64 {
		return 0, 0, true, fmt.Errorf("sparse moving window indexes kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if length < 0 {
		return 0, 0, true, fmt.Errorf("sparse moving window length must be non-negative")
	}
	if width <= 0 {
		return 0, 0, true, fmt.Errorf("sparse moving window width must be positive")
	}
	if !i64SelectedExprValid(expr) {
		return 0, 0, false, nil
	}
	recip := movingAvgReciprocalPrefix(length, width)
	var msum int64
	var mavg float64
	accumulate := func(row int64, value int64) error {
		if row < 0 || row >= int64(length) {
			return fmt.Errorf("sparse moving window index %d out of range", row)
		}
		count := length - int(row)
		if count > width {
			count = width
		}
		msum += value * int64(count)
		mavg += float64(value) * movingAvgSparseContribution(int(row), count, width, recip)
		return nil
	}
	if rows, owned, ok := tryBulkI64Values(indexes); ok {
		for _, row := range rows {
			if row < 0 || row >= int64(length) {
				bulkI64Release(rows, owned)
				return 0, 0, true, fmt.Errorf("sparse moving window index %d out of range", row)
			}
		}
		eval := i64SelectedExprBulkEval{rows: rows}
		values, err := eval.eval(expr)
		if err != nil {
			eval.release()
			bulkI64Release(rows, owned)
			return 0, 0, true, err
		}
		for i, row := range rows {
			if err := accumulate(row, values[i]); err != nil {
				eval.release()
				bulkI64Release(rows, owned)
				return 0, 0, true, err
			}
		}
		eval.release()
		bulkI64Release(rows, owned)
		return msum, mavg, true, nil
	}
	if err := forEachTypedI64Index(indexes, length, func(row int) error {
		value, err := evalI64SelectedExpr(expr, int64(row))
		if err != nil {
			return err
		}
		return accumulate(int64(row), value)
	}); err != nil {
		return 0, 0, true, err
	}
	return msum, mavg, true, nil
}

func TryTypedI64IndexExprSparseZeroMovingSumAvg(indexes Array, expr I64IndexExpr, length int, width int) (int64, float64, bool, error) {
	if indexes == nil {
		return 0, 0, true, fmt.Errorf("sparse moving window indexes must be non-nil")
	}
	if indexes.Kind() != KindI64 {
		return 0, 0, true, fmt.Errorf("sparse moving window indexes kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if length < 0 {
		return 0, 0, true, fmt.Errorf("sparse moving window length must be non-negative")
	}
	if width <= 0 {
		return 0, 0, true, fmt.Errorf("sparse moving window width must be positive")
	}
	if !i64IndexExprValid(expr) {
		return 0, 0, false, nil
	}
	recip := movingAvgReciprocalPrefix(length, width)
	var msum int64
	var mavg float64
	accumulate := func(row int64, value int64) error {
		if row < 0 || row >= int64(length) {
			return fmt.Errorf("sparse moving window index %d out of range", row)
		}
		count := length - int(row)
		if count > width {
			count = width
		}
		msum += value * int64(count)
		mavg += float64(value) * movingAvgSparseContribution(int(row), count, width, recip)
		return nil
	}
	if rows, rowsOwned, ok := tryBulkI64Values(indexes); ok {
		for _, row := range rows {
			if row < 0 || row >= int64(length) {
				bulkI64Release(rows, rowsOwned)
				return 0, 0, true, fmt.Errorf("sparse moving window index %d out of range", row)
			}
		}
		eval := i64IndexExprBulkEval{rows: rows}
		values, err := eval.eval(expr)
		if err != nil {
			eval.release()
			bulkI64Release(rows, rowsOwned)
			return 0, 0, true, err
		}
		for i, row := range rows {
			if err := accumulate(row, values[i]); err != nil {
				eval.release()
				bulkI64Release(rows, rowsOwned)
				return 0, 0, true, err
			}
		}
		eval.release()
		bulkI64Release(rows, rowsOwned)
		return msum, mavg, true, nil
	}
	if err := forEachTypedI64Index(indexes, length, func(row int) error {
		value, err := evalI64IndexExpr(expr, int64(row))
		if err != nil {
			return err
		}
		return accumulate(int64(row), value)
	}); err != nil {
		return 0, 0, true, err
	}
	return msum, mavg, true, nil
}

func movingAvgReciprocalPrefix(length int, width int) []float64 {
	n := width
	if length < n {
		n = length
	}
	prefix := make([]float64, n+1)
	for i := 0; i < n; i++ {
		prefix[i+1] = prefix[i] + 1/float64(i+1)
	}
	return prefix
}

func movingAvgSparseContribution(row int, count int, width int, recip []float64) float64 {
	if count <= 0 {
		return 0
	}
	end := row + count - 1
	if row >= width {
		return float64(count) / float64(width)
	}
	prefixEnd := end
	if prefixEnd >= width {
		prefixEnd = width - 1
	}
	total := recip[prefixEnd+1] - recip[row]
	if end >= width {
		total += float64(end-width+1) / float64(width)
	}
	return total
}

func i64SelectedExprValid(expr I64SelectedExpr) bool {
	switch expr.Op {
	case I64SelectedExprConst, I64SelectedExprIndex:
		return true
	case I64SelectedExprGather:
		return expr.Source != nil && isDenseIntegerArray(expr.Source)
	case I64SelectedExprAdd, I64SelectedExprSub, I64SelectedExprMul, I64SelectedExprDiv, I64SelectedExprMod:
		return expr.Left != nil && expr.Right != nil && i64SelectedExprValid(*expr.Left) && i64SelectedExprValid(*expr.Right)
	case I64SelectedExprXbar:
		return expr.Value > 0 && expr.Left != nil && i64SelectedExprValid(*expr.Left)
	default:
		return false
	}
}

func evalI64SelectedExpr(expr I64SelectedExpr, row int64) (int64, error) {
	switch expr.Op {
	case I64SelectedExprConst:
		return expr.Value, nil
	case I64SelectedExprIndex:
		return row, nil
	case I64SelectedExprGather:
		if row < 0 || row >= int64(expr.Source.Len()) {
			return 0, fmt.Errorf("selected expression gather row %d out of range", row)
		}
		value, ok, err := integerArrayAt(expr.Source, int(row))
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, fmt.Errorf("selected expression gather row %d is null", row)
		}
		return value, nil
	case I64SelectedExprAdd:
		left, right, err := evalI64SelectedExprOperands(expr, row)
		if err != nil {
			return 0, err
		}
		return left + right, nil
	case I64SelectedExprSub:
		left, right, err := evalI64SelectedExprOperands(expr, row)
		if err != nil {
			return 0, err
		}
		return left - right, nil
	case I64SelectedExprMul:
		left, right, err := evalI64SelectedExprOperands(expr, row)
		if err != nil {
			return 0, err
		}
		return left * right, nil
	case I64SelectedExprDiv:
		left, right, err := evalI64SelectedExprOperands(expr, row)
		if err != nil {
			return 0, err
		}
		if right == 0 {
			return 0, fmt.Errorf("selected expression divide by zero")
		}
		return left / right, nil
	case I64SelectedExprMod:
		left, right, err := evalI64SelectedExprOperands(expr, row)
		if err != nil {
			return 0, err
		}
		if right == 0 {
			return 0, fmt.Errorf("selected expression modulo by zero")
		}
		return left % right, nil
	case I64SelectedExprXbar:
		value, err := evalI64SelectedExpr(*expr.Left, row)
		if err != nil {
			return 0, err
		}
		if expr.Value <= 0 {
			return 0, fmt.Errorf("selected expression xbar width must be positive")
		}
		return floorInt64(value, expr.Value), nil
	default:
		return 0, fmt.Errorf("unsupported selected expression op %d", expr.Op)
	}
}

func evalI64SelectedExprOperands(expr I64SelectedExpr, row int64) (int64, int64, error) {
	left, err := evalI64SelectedExpr(*expr.Left, row)
	if err != nil {
		return 0, 0, err
	}
	right, err := evalI64SelectedExpr(*expr.Right, row)
	if err != nil {
		return 0, 0, err
	}
	return left, right, nil
}

func accumulateSelectedExprFbyRows(rows []int64, expr I64SelectedExpr, lookup fbyGroupLookupFn, sums []int64) error {
	switch expr.Op {
	case I64SelectedExprConst:
		for _, row := range rows {
			group, err := lookup(int(row))
			if err != nil {
				return err
			}
			sums[group] += expr.Value
		}
		return nil
	case I64SelectedExprIndex:
		for _, row := range rows {
			group, err := lookup(int(row))
			if err != nil {
				return err
			}
			sums[group] += row
		}
		return nil
	}
	eval := i64SelectedExprBulkEval{rows: rows}
	defer eval.release()
	switch expr.Op {
	case I64SelectedExprAdd, I64SelectedExprSub, I64SelectedExprMul, I64SelectedExprDiv, I64SelectedExprMod:
		left, err := eval.eval(*expr.Left)
		if err != nil {
			return err
		}
		if rightConst, ok := i64SelectedExprConstValue(*expr.Right); ok {
			switch expr.Op {
			case I64SelectedExprDiv:
				if rightConst == 0 && len(rows) > 0 {
					return fmt.Errorf("selected expression divide by zero")
				}
			case I64SelectedExprMod:
				if rightConst == 0 && len(rows) > 0 {
					return fmt.Errorf("selected expression modulo by zero")
				}
			}
			for i, row := range rows {
				group, err := lookup(int(row))
				if err != nil {
					return err
				}
				value := left[i]
				switch expr.Op {
				case I64SelectedExprAdd:
					value += rightConst
				case I64SelectedExprSub:
					value -= rightConst
				case I64SelectedExprMul:
					value *= rightConst
				case I64SelectedExprDiv:
					value /= rightConst
				case I64SelectedExprMod:
					value %= rightConst
				}
				sums[group] += value
			}
			return nil
		}
		right, err := eval.eval(*expr.Right)
		if err != nil {
			return err
		}
		for i, row := range rows {
			group, err := lookup(int(row))
			if err != nil {
				return err
			}
			value := left[i]
			switch expr.Op {
			case I64SelectedExprAdd:
				value += right[i]
			case I64SelectedExprSub:
				value -= right[i]
			case I64SelectedExprMul:
				value *= right[i]
			case I64SelectedExprDiv:
				if right[i] == 0 {
					return fmt.Errorf("selected expression divide by zero")
				}
				value /= right[i]
			case I64SelectedExprMod:
				if right[i] == 0 {
					return fmt.Errorf("selected expression modulo by zero")
				}
				value %= right[i]
			}
			sums[group] += value
		}
		return nil
	case I64SelectedExprXbar:
		if expr.Value <= 0 {
			return fmt.Errorf("selected expression xbar width must be positive")
		}
		left, err := eval.eval(*expr.Left)
		if err != nil {
			return err
		}
		for i, row := range rows {
			group, err := lookup(int(row))
			if err != nil {
				return err
			}
			sums[group] += floorInt64(left[i], expr.Value)
		}
		return nil
	default:
		values, err := eval.eval(expr)
		if err != nil {
			return err
		}
		for i, row := range rows {
			group, err := lookup(int(row))
			if err != nil {
				return err
			}
			sums[group] += values[i]
		}
		return nil
	}
}

// i64SelectedExprBulkEval evaluates selected-row expressions over a dense
// index vector. It is the selected-value counterpart to i64IndexExprBulkEval:
// one loop per expression node, pooled temporaries, and scalar-compatible
// arithmetic/error semantics.
type i64SelectedExprBulkEval struct {
	rows        []int64
	results     [8][]int64
	resultCount int
	overflow    [][]int64
}

func i64SelectedExprConstValue(expr I64SelectedExpr) (int64, bool) {
	if expr.Op == I64SelectedExprConst {
		return expr.Value, true
	}
	return 0, false
}

// eval returns a slice owned by the evaluator (or the shared rows slice);
// callers must treat it as read-only and call release when finished.
func (e *i64SelectedExprBulkEval) eval(expr I64SelectedExpr) ([]int64, error) {
	switch expr.Op {
	case I64SelectedExprIndex:
		return e.rows, nil
	case I64SelectedExprConst:
		out := bulkI64Get(len(e.rows))
		for i := range out {
			out[i] = expr.Value
		}
		e.remember(out)
		return out, nil
	case I64SelectedExprGather:
		if expr.Source == nil {
			return nil, fmt.Errorf("selected expression gather source is nil")
		}
		out := bulkI64Get(len(e.rows))
		if err := gatherSelectedI64Rows(expr.Source, e.rows, out); err != nil {
			bulkI64Release(out, true)
			return nil, err
		}
		e.remember(out)
		return out, nil
	case I64SelectedExprAdd, I64SelectedExprSub, I64SelectedExprMul, I64SelectedExprDiv, I64SelectedExprMod:
		out, err := e.evalDyadic(expr)
		if err != nil {
			return nil, err
		}
		e.remember(out)
		return out, nil
	case I64SelectedExprXbar:
		if expr.Value <= 0 {
			return nil, fmt.Errorf("selected expression xbar width must be positive")
		}
		left, err := e.eval(*expr.Left)
		if err != nil {
			return nil, err
		}
		out := bulkI64Get(len(e.rows))
		for i, v := range left {
			out[i] = floorInt64(v, expr.Value)
		}
		e.remember(out)
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported selected expression op %d", expr.Op)
	}
}

func (e *i64SelectedExprBulkEval) evalDyadic(expr I64SelectedExpr) ([]int64, error) {
	rightConst, rightIsConst := i64SelectedExprConstValue(*expr.Right)
	left, err := e.eval(*expr.Left)
	if err != nil {
		return nil, err
	}
	out := bulkI64Get(len(e.rows))
	if rightIsConst {
		switch expr.Op {
		case I64SelectedExprAdd:
			for i, v := range left {
				out[i] = v + rightConst
			}
		case I64SelectedExprSub:
			for i, v := range left {
				out[i] = v - rightConst
			}
		case I64SelectedExprMul:
			for i, v := range left {
				out[i] = v * rightConst
			}
		case I64SelectedExprDiv:
			if rightConst == 0 && len(left) > 0 {
				bulkI64Release(out, true)
				return nil, fmt.Errorf("selected expression divide by zero")
			}
			for i, v := range left {
				out[i] = v / rightConst
			}
		case I64SelectedExprMod:
			if rightConst == 0 && len(left) > 0 {
				bulkI64Release(out, true)
				return nil, fmt.Errorf("selected expression modulo by zero")
			}
			for i, v := range left {
				out[i] = v % rightConst
			}
		}
		return out, nil
	}
	right, err := e.eval(*expr.Right)
	if err != nil {
		bulkI64Release(out, true)
		return nil, err
	}
	switch expr.Op {
	case I64SelectedExprAdd:
		for i, v := range left {
			out[i] = v + right[i]
		}
	case I64SelectedExprSub:
		for i, v := range left {
			out[i] = v - right[i]
		}
	case I64SelectedExprMul:
		for i, v := range left {
			out[i] = v * right[i]
		}
	case I64SelectedExprDiv:
		for i, v := range left {
			if right[i] == 0 {
				bulkI64Release(out, true)
				return nil, fmt.Errorf("selected expression divide by zero")
			}
			out[i] = v / right[i]
		}
	case I64SelectedExprMod:
		for i, v := range left {
			if right[i] == 0 {
				bulkI64Release(out, true)
				return nil, fmt.Errorf("selected expression modulo by zero")
			}
			out[i] = v % right[i]
		}
	}
	return out, nil
}

func (e *i64SelectedExprBulkEval) remember(values []int64) {
	if e.resultCount < len(e.results) {
		e.results[e.resultCount] = values
		e.resultCount++
		return
	}
	e.overflow = append(e.overflow, values)
}

func (e *i64SelectedExprBulkEval) release() {
	for i := 0; i < e.resultCount; i++ {
		bulkI64Release(e.results[i], true)
		e.results[i] = nil
	}
	for _, values := range e.overflow {
		bulkI64Release(values, true)
	}
	e.resultCount = 0
	e.overflow = nil
}

func gatherSelectedI64Rows(source Array, rows []int64, out []int64) error {
	switch a := source.(type) {
	case attributedArray:
		return gatherSelectedI64Rows(a.array, rows, out)
	case i64FillArray:
		return gatherSelectedI64RowsFilled(a.source, a.fill, rows, out)
	case columnArray[int8]:
		return gatherSelectedI64ColumnRows(a.data, rows, out)
	case columnArray[int16]:
		return gatherSelectedI64ColumnRows(a.data, rows, out)
	case columnArray[int32]:
		return gatherSelectedI64ColumnRows(a.data, rows, out)
	case columnArray[int64]:
		return gatherSelectedI64ColumnRows(a.data, rows, out)
	case columnArray[uint8]:
		return gatherSelectedI64ColumnRows(a.data, rows, out)
	case columnArray[uint16]:
		return gatherSelectedI64ColumnRows(a.data, rows, out)
	case columnArray[uint32]:
		return gatherSelectedI64ColumnRows(a.data, rows, out)
	case columnArray[uint64]:
		return gatherSelectedI64Uint64ColumnRows(a.data, rows, out)
	case nullBitmapArray[int8]:
		return gatherSelectedI64NullBitmapRows(a.data, a.nulls, rows, out)
	case nullBitmapArray[int16]:
		return gatherSelectedI64NullBitmapRows(a.data, a.nulls, rows, out)
	case nullBitmapArray[int32]:
		return gatherSelectedI64NullBitmapRows(a.data, a.nulls, rows, out)
	case nullBitmapArray[int64]:
		return gatherSelectedI64NullBitmapRows(a.data, a.nulls, rows, out)
	case i64RangeArray:
		for i, row := range rows {
			if row < 0 || row >= int64(a.len) {
				return fmt.Errorf("selected expression gather row %d out of range", row)
			}
			out[i] = a.start + row*a.step
		}
		return nil
	default:
		for i, row := range rows {
			if row < 0 || row >= int64(source.Len()) {
				return fmt.Errorf("selected expression gather row %d out of range", row)
			}
			value, ok, err := integerArrayAt(source, int(row))
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("selected expression gather row %d is null", row)
			}
			out[i] = value
		}
		return nil
	}
}

func gatherSelectedI64RowsFilled(source Array, fill int64, rows []int64, out []int64) error {
	switch a := source.(type) {
	case attributedArray:
		return gatherSelectedI64RowsFilled(a.array, fill, rows, out)
	case nullBitmapArray[int8]:
		return gatherSelectedI64NullBitmapRowsFilled(a.data, a.nulls, fill, rows, out)
	case nullBitmapArray[int16]:
		return gatherSelectedI64NullBitmapRowsFilled(a.data, a.nulls, fill, rows, out)
	case nullBitmapArray[int32]:
		return gatherSelectedI64NullBitmapRowsFilled(a.data, a.nulls, fill, rows, out)
	case nullBitmapArray[int64]:
		return gatherSelectedI64NullBitmapRowsFilled(a.data, a.nulls, fill, rows, out)
	case shiftedArray:
		for i, row := range rows {
			if row < 0 || row >= int64(a.Len()) {
				return fmt.Errorf("selected expression gather row %d out of range", row)
			}
			sourceRow := int(row) + a.offset
			if sourceRow < 0 || sourceRow >= a.source.Len() {
				out[i] = fill
				continue
			}
			value, ok, err := integerArrayAt(a.source, sourceRow)
			if err != nil {
				return err
			}
			if !ok {
				out[i] = fill
				continue
			}
			out[i] = value
		}
		return nil
	default:
		for i, row := range rows {
			if row < 0 || row >= int64(source.Len()) {
				return fmt.Errorf("selected expression gather row %d out of range", row)
			}
			value, ok := source.At(int(row))
			if !ok {
				return fmt.Errorf("selected expression gather row %d out of range", row)
			}
			if IsNull(value) {
				out[i] = fill
				continue
			}
			n, ok := coerceInt64Exact(value)
			if !ok {
				return fmt.Errorf("fill row %d is %T, want integer", row, value)
			}
			out[i] = n
		}
		return nil
	}
}

func gatherSelectedI64ColumnRows[T ~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32](values []T, rows []int64, out []int64) error {
	for i, row := range rows {
		if row < 0 || row >= int64(len(values)) {
			return fmt.Errorf("selected expression gather row %d out of range", row)
		}
		out[i] = int64(values[row])
	}
	return nil
}

func gatherSelectedI64Uint64ColumnRows(values []uint64, rows []int64, out []int64) error {
	for i, row := range rows {
		if row < 0 || row >= int64(len(values)) {
			return fmt.Errorf("selected expression gather row %d out of range", row)
		}
		value := values[row]
		if value > math.MaxInt64 {
			return fmt.Errorf("selected expression gather row %d is uint64 overflow", row)
		}
		out[i] = int64(value)
	}
	return nil
}

func gatherSelectedI64NullBitmapRows[T ~int8 | ~int16 | ~int32 | ~int64](values []T, nulls []uint64, rows []int64, out []int64) error {
	for i, row := range rows {
		if row < 0 || row >= int64(len(values)) {
			return fmt.Errorf("selected expression gather row %d out of range", row)
		}
		if nullBitGet(nulls, int(row)) {
			return fmt.Errorf("selected expression gather row %d is null", row)
		}
		out[i] = int64(values[row])
	}
	return nil
}

func gatherSelectedI64NullBitmapRowsFilled[T ~int8 | ~int16 | ~int32 | ~int64](values []T, nulls []uint64, fill int64, rows []int64, out []int64) error {
	for i, row := range rows {
		if row < 0 || row >= int64(len(values)) {
			return fmt.Errorf("selected expression gather row %d out of range", row)
		}
		if nullBitGet(nulls, int(row)) {
			out[i] = fill
			continue
		}
		out[i] = int64(values[row])
	}
	return nil
}

func i64IndexExprValid(expr I64IndexExpr) bool {
	switch expr.Op {
	case I64IndexExprConst, I64IndexExprIndex:
		return true
	case I64IndexExprAdd, I64IndexExprSub, I64IndexExprMul, I64IndexExprDiv, I64IndexExprMod:
		return expr.Left != nil && expr.Right != nil && i64IndexExprValid(*expr.Left) && i64IndexExprValid(*expr.Right)
	case I64IndexExprXbar:
		return expr.Value > 0 && expr.Left != nil && i64IndexExprValid(*expr.Left)
	default:
		return false
	}
}

func evalI64IndexExpr(expr I64IndexExpr, index int64) (int64, error) {
	switch expr.Op {
	case I64IndexExprConst:
		return expr.Value, nil
	case I64IndexExprIndex:
		return index, nil
	case I64IndexExprAdd:
		left, right, err := evalI64IndexExprOperands(expr, index)
		if err != nil {
			return 0, err
		}
		return left + right, nil
	case I64IndexExprSub:
		left, right, err := evalI64IndexExprOperands(expr, index)
		if err != nil {
			return 0, err
		}
		return left - right, nil
	case I64IndexExprMul:
		left, right, err := evalI64IndexExprOperands(expr, index)
		if err != nil {
			return 0, err
		}
		return left * right, nil
	case I64IndexExprDiv:
		left, right, err := evalI64IndexExprOperands(expr, index)
		if err != nil {
			return 0, err
		}
		if right == 0 {
			return 0, fmt.Errorf("integer index expression divide by zero")
		}
		return left / right, nil
	case I64IndexExprMod:
		left, right, err := evalI64IndexExprOperands(expr, index)
		if err != nil {
			return 0, err
		}
		if right == 0 {
			return 0, fmt.Errorf("integer index expression modulo by zero")
		}
		return left % right, nil
	case I64IndexExprXbar:
		value, err := evalI64IndexExpr(*expr.Left, index)
		if err != nil {
			return 0, err
		}
		if expr.Value <= 0 {
			return 0, fmt.Errorf("integer index expression xbar width must be positive")
		}
		return floorInt64(value, expr.Value), nil
	default:
		return 0, fmt.Errorf("unsupported integer index expression op %d", expr.Op)
	}
}

func evalI64IndexExprOperands(expr I64IndexExpr, index int64) (int64, int64, error) {
	left, err := evalI64IndexExpr(*expr.Left, index)
	if err != nil {
		return 0, 0, err
	}
	right, err := evalI64IndexExpr(*expr.Right, index)
	if err != nil {
		return 0, 0, err
	}
	return left, right, nil
}

func typedNumericSumCountWherePredicate(values Array, pred rowPredicate) (any, int64, error) {
	var count int64
	if isDenseIntegerArray(values) {
		var total int64
		for row := 0; row < values.Len(); row++ {
			if !pred(row) {
				continue
			}
			count++
			value, ok, err := integerArrayAt(values, row)
			if err != nil {
				return nil, 0, err
			}
			if ok {
				total += value
			}
		}
		if count == 0 {
			return NullValue, 0, nil
		}
		return total, count, nil
	}
	if !isNumericArray(values) {
		return nil, 0, errUnsupportedNumericSumCountWhereValues
	}
	var total float64
	for row := 0; row < values.Len(); row++ {
		if !pred(row) {
			continue
		}
		count++
		value, ok, err := typedKernels.NumericAt(values, row)
		if err != nil {
			return nil, 0, err
		}
		if ok {
			total += value
		}
	}
	if count == 0 {
		return NullValue, 0, nil
	}
	return total, count, nil
}

var errUnsupportedNumericSumCountWhereValues = fmt.Errorf("unsupported sum count where values")

type f64AffineView struct {
	start float64
	step  float64
	len   int
}

func f64AffineArrayView(array Array) (f64AffineView, bool) {
	switch a := unwrapAttributedArray(array).(type) {
	case i64RangeArray:
		return f64AffineView{start: float64(a.start), step: float64(a.step), len: a.len}, true
	case f64RangeArray:
		return f64AffineView{start: a.start, step: a.step, len: a.len}, true
	case i64ScalarDyadicArray:
		if source, ok := i64ScalarDyadicAffineRange(a); ok {
			return f64AffineView{start: float64(source.start), step: float64(source.step), len: source.len}, true
		}
	case f64NumericDyadicArray:
		producer, err := newF64NumericDyadicProducer(a)
		if err == nil {
			return f64AffineProducerView(producer)
		}
	}
	return f64AffineView{}, false
}

func f64AffineProducerView(producer f64NumericProducer) (f64AffineView, bool) {
	switch p := producer.(type) {
	case f64I64RangeProducer:
		return f64AffineView{start: float64(p.values.start), step: float64(p.values.step), len: p.values.len}, true
	case f64F64RangeProducer:
		return f64AffineView{start: p.values.start, step: p.values.step, len: p.values.len}, true
	case f64I64ScalarDyadicProducer:
		if source, ok := i64ScalarDyadicAffineRange(p.values); ok {
			return f64AffineView{start: float64(source.start), step: float64(source.step), len: source.len}, true
		}
	case f64DyadicProducer:
		left, leftOK := f64AffineProducerView(p.left)
		if scalar, rightOK := f64ScalarProducerValue(p.right); leftOK && rightOK {
			return f64AffineApplyScalar(left, p.op, scalar, false)
		}
		right, rightOK := f64AffineProducerView(p.right)
		if scalar, leftScalarOK := f64ScalarProducerValue(p.left); rightOK && leftScalarOK {
			return f64AffineApplyScalar(right, p.op, scalar, true)
		}
	}
	return f64AffineView{}, false
}

func f64ScalarProducerValue(producer f64NumericProducer) (float64, bool) {
	switch p := producer.(type) {
	case f64ScalarProducer:
		return p.value, true
	case f64BroadcastProducer:
		return f64ScalarProducerValue(p.source)
	default:
		return 0, false
	}
}

func f64AffineApplyScalar(view f64AffineView, op string, scalar float64, scalarLeft bool) (f64AffineView, bool) {
	switch op {
	case string(OpAdd):
		view.start += scalar
	case string(OpSub):
		if scalarLeft {
			view.start = scalar - view.start
			view.step = -view.step
		} else {
			view.start -= scalar
		}
	case string(OpMul):
		view.start *= scalar
		view.step *= scalar
	case string(OpDiv):
		if scalarLeft || scalar == 0 {
			return f64AffineView{}, false
		}
		view.start /= scalar
		view.step /= scalar
	default:
		return f64AffineView{}, false
	}
	return view, true
}

func f64AffineSumCountWhereCompareSelf(values Array, op Op, scalar any) (any, int64, bool) {
	if isDenseIntegerArray(values) {
		return nil, 0, false
	}
	view, ok := f64AffineArrayView(values)
	if !ok || view.len <= 0 || view.step == 0 {
		return nil, 0, false
	}
	target, ok := numeric(scalar)
	if !ok {
		return nil, 0, false
	}
	start, count, ok := f64AffineCompareSelection(view, op, target)
	if !ok {
		return nil, 0, false
	}
	if count == 0 {
		return NullValue, 0, true
	}
	return f64AffineRangeSliceSum(view, start, count), int64(count), true
}

func f64AffineCompareSelection(view f64AffineView, op Op, scalar float64) (int, int, bool) {
	switch op {
	case OpGT, OpGE, OpLT, OpLE:
	default:
		return 0, 0, false
	}
	firstTrue := -1
	for i := 0; i < view.len; i++ {
		if f64CompareOp(view.start+float64(i)*view.step, op, scalar) {
			firstTrue = i
			break
		}
	}
	if firstTrue < 0 {
		return 0, 0, true
	}
	lastTrue := firstTrue
	for i := view.len - 1; i >= firstTrue; i-- {
		if f64CompareOp(view.start+float64(i)*view.step, op, scalar) {
			lastTrue = i
			break
		}
	}
	return firstTrue, lastTrue - firstTrue + 1, true
}

func f64CompareOp(value float64, op Op, scalar float64) bool {
	switch op {
	case OpGT:
		return value > scalar
	case OpGE:
		return value >= scalar
	case OpLT:
		return value < scalar
	case OpLE:
		return value <= scalar
	default:
		return false
	}
}

func f64AffineRangeSliceSum(view f64AffineView, start, count int) float64 {
	if count <= 0 {
		return 0
	}
	first := view.start + float64(start)*view.step
	last := first + float64(count-1)*view.step
	return float64(count) * (first + last) / 2
}

func typedWithinRowPredicate(array Array, low, high any, highClosed bool) (rowPredicate, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedWithinRowPredicate(a.array, low, high, highClosed)
	case tiledArray:
		if a.source.Len() == 0 {
			return nil, false, nil
		}
		low = normalizeScalar(a.source.Kind(), low)
		high = normalizeScalar(a.source.Kind(), high)
		return func(row int) bool {
			if row < 0 || row >= a.len {
				return false
			}
			value, ok := a.source.At((a.start + row) % a.source.Len())
			if !ok {
				return false
			}
			if compare(value, low) < 0 {
				return false
			}
			if highClosed {
				return compare(value, high) <= 0
			}
			return compare(value, high) < 0
		}, true, nil
	default:
		low = normalizeScalar(array.Kind(), low)
		high = normalizeScalar(array.Kind(), high)
		if !qTypedWithinPredicateKind(array.Kind()) {
			return nil, false, nil
		}
		return func(row int) bool {
			value, ok := array.At(row)
			if !ok {
				return false
			}
			if compare(value, low) < 0 {
				return false
			}
			if highClosed {
				return compare(value, high) <= 0
			}
			return compare(value, high) < 0
		}, true, nil
	}
}

func qTypedWithinPredicateKind(kind Kind) bool {
	switch kind {
	case KindBool,
		KindI8, KindI16, KindI32, KindI64,
		KindU8, KindU16, KindU32, KindU64,
		KindF32, KindF64,
		KindString, KindSymbol,
		KindMonth, KindDate, KindDateTime,
		KindTimespan, KindMinute, KindSecond, KindTime, KindTimestamp:
		return true
	default:
		return false
	}
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
	if indexes, ok := i64SegmentModuloEqualIndexes(array, modulusI64, op, targetI64); ok {
		return indexes, true, nil
	}
	if plan, ok := i64ModuloComparePlanForArray(array, modulusI64, op, targetI64); ok {
		return i64ModuloComparePlanIndexArray(plan), true, nil
	}
	if !isDenseIntegerArray(array) {
		return nil, false, nil
	}
	// Bulk-flatten the carrier once and select in a dense loop instead of
	// walking the carrier tree per row. Null rows bail out of the flatteners,
	// so the per-row fallback keeps its null compare semantics.
	if values, owned, ok := tryBulkI64Values(array); ok {
		indexes := make([]int64, 0, len(values)/2)
		switch op {
		case OpEQ:
			for row, v := range values {
				if qModInt64(v, modulusI64) == targetI64 {
					indexes = append(indexes, int64(row))
				}
			}
		case OpNE:
			for row, v := range values {
				if qModInt64(v, modulusI64) != targetI64 {
					indexes = append(indexes, int64(row))
				}
			}
		}
		bulkI64Release(values, owned)
		return newI64Trusted(indexes), true, nil
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

func i64SegmentModuloEqualIndexes(array Array, modulus int64, op Op, target int64) (Array, bool) {
	if modulus <= 0 || op != OpEQ || target < 0 || target >= modulus {
		return nil, false
	}
	switch a := array.(type) {
	case attributedArray:
		return i64SegmentModuloEqualIndexes(a.array, modulus, op, target)
	case i64SegmentArray:
		segments := make([]i64RangeArray, 0, len(a.segments))
		logicalStart := int64(0)
		for _, segment := range a.segments {
			if segment.len <= 0 {
				continue
			}
			if segment.step != 1 {
				return nil, false
			}
			first := qPositiveMod(target-qPositiveMod(segment.start, modulus), modulus)
			if first < int64(segment.len) {
				count := int((int64(segment.len)-first-1)/modulus) + 1
				segments = append(segments, i64RangeArray{
					start: logicalStart + first,
					step:  modulus,
					len:   count,
				})
			}
			logicalStart += int64(segment.len)
		}
		if logicalStart != int64(a.len) {
			return nil, false
		}
		return newI64SegmentArray(segments...), true
	default:
		return nil, false
	}
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
	if sum, count, handled, err := tryTypedIntegerSumWhereModuloCompare(values, modSource, modulusI64, op, targetI64); err != nil || handled {
		if err != nil || !handled {
			return nil, handled, err
		}
		if count == 0 {
			return NullValue, true, nil
		}
		return sum, true, nil
	}
	if !isNumericArray(values) {
		return nil, false, nil
	}
	if plan, ok := i64ModuloComparePlanForArray(modSource, modulusI64, op, targetI64); ok {
		if out, handled, err := tryTypedNumericSumWhereModuloComparePlan(values, plan); err != nil || handled {
			return out, handled, err
		}
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

func TryTypedIntegerSumWhereModuloCompare(values, modSource Array, modulus any, op Op, target any) (int64, int64, bool, error) {
	modulusI64, targetI64, ok := moduloCompareOperands(modulus, op, target)
	if !ok {
		return 0, 0, false, nil
	}
	if values == nil || modSource == nil {
		return 0, 0, true, fmt.Errorf("modulo compare sum arrays must be non-nil")
	}
	if values.Len() != modSource.Len() {
		return 0, 0, true, fmt.Errorf("modulo compare sum length mismatch: values=%d source=%d", values.Len(), modSource.Len())
	}
	return tryTypedIntegerSumWhereModuloCompare(values, modSource, modulusI64, op, targetI64)
}

func tryTypedIntegerSumWhereModuloCompare(values, modSource Array, modulus int64, op Op, target int64) (int64, int64, bool, error) {
	if !isDenseIntegerArray(values) || !isDenseIntegerArray(modSource) {
		return 0, 0, false, nil
	}
	if plan, ok := i64ModuloComparePlanForArray(modSource, modulus, op, target); ok {
		count, ok := plan.trueCount()
		if !ok {
			return 0, 0, false, nil
		}
		if count == 0 {
			return 0, 0, true, nil
		}
		switch a := unwrapAttributedArray(values).(type) {
		case i64RangeArray:
			if a.len != plan.length {
				return 0, 0, true, fmt.Errorf("modulo compare sum length mismatch: values=%d source=%d", a.len, plan.length)
			}
			return a.start*count + a.step*plan.indexSum(), count, true, nil
		}
	}
	var total int64
	var count int64
	for row := 0; row < modSource.Len(); row++ {
		match, err := integerModuloCompareAt(modSource, row, modulus, op, target)
		if err != nil {
			return 0, 0, true, err
		}
		if !match {
			continue
		}
		value, ok, err := integerArrayAt(values, row)
		if err != nil {
			return 0, 0, true, err
		}
		if ok {
			total += value
		}
		count++
	}
	return total, count, true, nil
}

func tryTypedNumericSumWhereModuloComparePlan(values Array, plan i64ModuloComparePlan) (any, bool, error) {
	count, ok := plan.trueCount()
	if !ok {
		return nil, false, nil
	}
	if count == 0 {
		return NullValue, true, nil
	}
	switch a := values.(type) {
	case attributedArray:
		return tryTypedNumericSumWhereModuloComparePlan(a.array, plan)
	case i64RangeArray:
		if a.len != plan.length {
			return nil, true, fmt.Errorf("modulo compare sum length mismatch: values=%d source=%d", a.len, plan.length)
		}
		return a.start*count + a.step*plan.indexSum(), true, nil
	default:
		return nil, false, nil
	}
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
	if rows, ok := i64Int32IndexRows(indexes); ok {
		if sum, handled, err := typedIntegerSumByI32Indexes(array, rows); handled || err != nil {
			return sum, handled, err
		}
	}
	// Affine range sources reduce over a dense index vector in one tight
	// loop: sum = start*count + step*sum(indexes).
	source := array
	if attributed, ok := source.(attributedArray); ok {
		source = attributed.array
	}
	if values, ok := source.(i64RangeArray); ok {
		if rows, owned, ok := tryBulkI64Values(indexes); ok {
			var rowSum int64
			for _, row := range rows {
				if row < 0 || row >= int64(values.len) {
					bulkI64Release(rows, owned)
					return nil, true, fmt.Errorf("index %d out of bounds for length %d", row, values.len)
				}
				rowSum += row
			}
			total := values.start*int64(len(rows)) + values.step*rowSum
			bulkI64Release(rows, owned)
			return total, true, nil
		}
	}
	// Scalar-dyadic gather-then-transform: gather only the selected base rows
	// and apply the chain's scalar steps to that small buffer, so low
	// selectivity avoids a full-length transform pass per consumer.
	if dyadic, ok := source.(i64ScalarDyadicArray); ok && dyadic.len == array.Len() {
		var steps [4]i64ScalarDyadicStep
		base, stepCount, status := collectI64ScalarDyadicSteps(dyadic, &steps)
		if status == i64DyadicStepsOK {
			if baseValues, baseOwned, ok := tryBulkI64Values(base); ok && len(baseValues) >= dyadic.len {
				if rows, rowsOwned, ok := tryBulkI64Values(indexes); ok {
					limit := int64(dyadic.len)
					gathered := bulkI64Get(len(rows))
					for i, row := range rows {
						if row < 0 || row >= limit {
							bulkI64Release(gathered, true)
							bulkI64Release(rows, rowsOwned)
							bulkI64Release(baseValues, baseOwned)
							return nil, true, fmt.Errorf("index %d out of bounds for length %d", row, limit)
						}
						gathered[i] = baseValues[row]
					}
					bulkI64Release(rows, rowsOwned)
					bulkI64Release(baseValues, baseOwned)
					stepsOK := true
					for s := stepCount - 1; s >= 0; s-- {
						if !applyI64ScalarDyadicStep(steps[s], gathered, gathered) {
							stepsOK = false
							break
						}
					}
					if stepsOK {
						var total int64
						for _, v := range gathered {
							total += v
						}
						bulkI64Release(gathered, true)
						return total, true, nil
					}
					bulkI64Release(gathered, true)
				} else {
					bulkI64Release(baseValues, baseOwned)
				}
			}
		}
	}
	// Dense gather-sum: flatten the source once and reduce with direct slice
	// indexing instead of per-row carrier dispatch through integerArrayAt.
	if rows, rowsOwned, ok := tryBulkI64Values(indexes); ok {
		if values, valuesOwned, ok := tryBulkI64Values(array); ok && len(values) == array.Len() {
			limit := int64(len(values))
			var total int64
			for _, row := range rows {
				if row < 0 || row >= limit {
					bulkI64Release(values, valuesOwned)
					bulkI64Release(rows, rowsOwned)
					return nil, true, fmt.Errorf("index %d out of bounds for length %d", row, limit)
				}
				total += values[row]
			}
			bulkI64Release(values, valuesOwned)
			bulkI64Release(rows, rowsOwned)
			return total, true, nil
		} else if ok {
			bulkI64Release(values, valuesOwned)
		}
		bulkI64Release(rows, rowsOwned)
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

func i64Int32IndexRows(indexes Array) ([]int32, bool) {
	switch idx := indexes.(type) {
	case attributedArray:
		return i64Int32IndexRows(idx.array)
	case i64Int32IndexArray:
		return idx.rows, true
	default:
		return nil, false
	}
}

func typedIntegerSumByI32Indexes(array Array, rows []int32) (any, bool, error) {
	if len(rows) == 0 {
		return NullValue, true, nil
	}
	source := array
	if attributed, ok := source.(attributedArray); ok {
		source = attributed.array
	}
	if values, ok := source.(i64RangeArray); ok {
		var rowSum int64
		for _, row := range rows {
			if row < 0 || int(row) >= values.len {
				return nil, true, fmt.Errorf("index %d out of bounds for length %d", row, values.len)
			}
			rowSum += int64(row)
		}
		return values.start*int64(len(rows)) + values.step*rowSum, true, nil
	}
	if values, owned, ok := tryBulkI64Values(array); ok && len(values) == array.Len() {
		limit := int32(len(values))
		if len(values) > math.MaxInt32 {
			limit = math.MaxInt32
		}
		var total int64
		for _, row := range rows {
			if row < 0 || row >= limit || int(row) >= len(values) {
				bulkI64Release(values, owned)
				return nil, true, fmt.Errorf("index %d out of bounds for length %d", row, len(values))
			}
			total += values[row]
		}
		bulkI64Release(values, owned)
		return total, true, nil
	} else if ok {
		bulkI64Release(values, owned)
	}
	return nil, false, nil
}

func typedIntegerSumByI64IndexView(array, indexes Array) (any, bool, error) {
	switch idx := indexes.(type) {
	case attributedArray:
		return typedIntegerSumByI64IndexView(array, idx.array)
	case i64Int32IndexArray:
		return typedIntegerSumByI32Indexes(array, idx.rows)
	case i64RangeArray:
		return typedIntegerSumRange(array, idx)
	case i64PeriodicIndexArray:
		return typedIntegerSumPeriodicIndex(array, idx)
	case i64SegmentArray:
		return typedIntegerSumSegments(array, idx)
	default:
		return nil, false, nil
	}
}

// typedIntegerSumSegments reduces a gather-sum over a segment-compressed
// where-result with one closed-form arithmetic-series sum per segment when
// the source is an affine i64 range, so alternating masks (the canonical
// null-mixed compare shape) cost O(segments) instead of O(rows) with a
// boxed interface conversion per segment.
func typedIntegerSumSegments(array Array, idx i64SegmentArray) (any, bool, error) {
	source := array
	if attributed, ok := source.(attributedArray); ok {
		source = attributed.array
	}
	values, ok := source.(i64RangeArray)
	if !ok {
		return nil, false, nil
	}
	var total int64
	for _, segment := range idx.segments {
		if segment.len == 0 {
			continue
		}
		first := segment.start
		last := segment.start + int64(segment.len-1)*segment.step
		low, high := first, last
		if low > high {
			low, high = high, low
		}
		if low < 0 || high >= int64(values.len) {
			return nil, true, fmt.Errorf("index range %d..%d out of bounds for length %d", first, last, values.len)
		}
		rowSum := (first + last) * int64(segment.len) / 2
		total += values.start*int64(segment.len) + values.step*rowSum
	}
	return total, true, nil
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
	if total, count, ok := fusedPredicateI64CompareAndSum(array, mask); ok {
		if count == 0 {
			return NullValue, true, nil
		}
		return total, true, nil
	}
	// Compound mask trees (bool-logical and/or chains, membership, within)
	// lower to one dense pooled bool pass through the same compiled
	// predicate `where` uses, instead of a per-row carrier-tree walk.
	if dense, ok := fusedPredicateDenseBoolMask(mask); ok {
		if bulk, owned, okValues := tryBulkI64Values(array); okValues && len(bulk) >= len(dense) {
			var total int64
			count := 0
			for row, keep := range dense {
				if keep {
					total += bulk[row]
					count++
				}
			}
			bulkI64Release(bulk, owned)
			bulkBoolRelease(dense, true)
			if count == 0 {
				return NullValue, true, nil
			}
			return total, true, nil
		}
		bulkBoolRelease(dense, true)
	}
	// Dense bool mask over a dense integer source: one fused slice pass
	// instead of a per-true-row closure call with boxed integerArrayAt
	// dispatch (the canonical `+/v where v>x` shape).
	if m, isDense := unwrapAttributedArray(mask).(columnArray[bool]); isDense {
		if vals, owned, ok := tryBulkI64Values(array); ok && len(vals) >= len(m.data) {
			var total int64
			count := 0
			for row, keep := range m.data {
				if keep {
					total += vals[row]
					count++
				}
			}
			bulkI64Release(vals, owned)
			if count == 0 {
				return NullValue, true, nil
			}
			return total, true, nil
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
		// Iterate segments inline: passing each i64RangeArray segment through
		// the Array parameter would box one interface value per segment.
		for _, segment := range idx.segments {
			for i := 0; i < segment.len; i++ {
				row, err := checkedI64Index(segment.start + int64(i)*segment.step)
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
		}
		return nil
	case i64PeriodicIndexArray:
		for i := 0; i < idx.Len(); i++ {
			value, ok := idx.i64At(i)
			if !ok {
				return fmt.Errorf("index vector row %d is out of range", i)
			}
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
	if arrayKnownNoNulls(array) {
		return array, true, nil
	}
	if values, ok := array.(nullableArray); ok {
		if out, handled := typedScalarFillNullable(fill, values); handled {
			return out, true, nil
		}
	}
	if n, ok := coerceInt64Exact(fill); ok && isIntegerArray(array) {
		return i64FillArray{source: array, fill: n}, true, nil
	}
	// Bitmap-backed float carriers materialize the fill densely in one pass:
	// the result has no nulls, so downstream reads stay unboxed (mirrors the
	// boxed typedScalarFillNullable eager path). Integer carriers stay lazy
	// above so fused reducers can consume null/fill state without an O(n)
	// temporary column.
	if nullBitmapBackedArray(array) {
		if fillF, ok := numeric(fill); ok && isNumericArray(array) {
			if values, nulls, owned, ok := tryBulkF64NullableValues(array); ok {
				out := make([]float64, len(values))
				for i, v := range values {
					if nulls != nil && nullBitGet(nulls, i) {
						out[i] = fillF
						continue
					}
					out[i] = v
				}
				bulkF64Release(values, owned)
				return newF64Trusted(out), true, nil
			}
		}
	}
	if n, ok := numeric(fill); ok && isNumericArray(array) {
		return f64FillArray{source: array, fill: n}, true, nil
	}
	return nil, false, nil
}

func arrayKnownNoNulls(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return arrayKnownNoNulls(a.array)
	case columnArray[bool], columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		columnArray[float32], columnArray[float64], columnArray[string], columnArray[Symbol],
		columnArray[Month], columnArray[Date], columnArray[DateTime], columnArray[Timespan],
		columnArray[Minute], columnArray[Second], columnArray[Time], columnArray[Timestamp]:
		return true
	case i64RangeArray, f64RangeArray, i64SegmentArray, i64Int32IndexArray, i64PeriodicIndexArray,
		i64ProductArray, i64DyadicProductArray, i64BucketArray, i64XrankArray, i64FillArray,
		f64BucketArray, f64FillArray, i64RunningSumArray, i64ScalarDyadicArray,
		i64ScalarDyadicRunningSumArray, f64NumericDyadicArray, i64DyadicMinMaxArray,
		qRatiosArray:
		return true
	case tiledArray:
		return arrayKnownNoNulls(a.source)
	default:
		return false
	}
}

// typedScalarFillNullable materializes a scalar fill over boxed nullable
// storage into a dense typed column in one pass. After the fill no nulls
// remain, so the result supports unboxed O(1) row access instead of paying
// boxed dispatch and coercion on every downstream read. The output kinds
// mirror the lazy fill views: KindI64 for integer data, KindF64 otherwise.
func typedScalarFillNullable(fill any, values nullableArray) (Array, bool) {
	if fillI, ok := coerceInt64Exact(fill); ok {
		out := make([]int64, len(values.data))
		allInt := true
		for i, v := range values.data {
			if IsNull(v) {
				out[i] = fillI
				continue
			}
			n, ok := coerceInt64Exact(v)
			if !ok {
				allInt = false
				break
			}
			out[i] = n
		}
		if allInt {
			return newI64Trusted(out), true
		}
	}
	fillF, ok := numeric(fill)
	if !ok {
		return nil, false
	}
	out := make([]float64, len(values.data))
	for i, v := range values.data {
		if IsNull(v) {
			out[i] = fillF
			continue
		}
		n, ok := numeric(v)
		if !ok {
			return nil, false
		}
		out[i] = n
	}
	return newF64Trusted(out), true
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
	case tiledArray:
		return typedSortIndexesTiled(a, descending)
	case nullableArray:
		return typedSortRowIndexesByArray(a, descending), true, nil
	case nullBitmapCarrier:
		return typedSortRowIndexesByArray(a, descending), true, nil
	case columnArray[int64]:
		// Dense i64 runs the prepared-key radix/pair sort instead of a
		// sort.SliceStable comparison loop; the permutation is identical.
		return typedSortIndexesI64Keys(a.data, descending), true, nil
	case i64ScalarDyadicArray:
		if indexes, ok := typedSortIndexesI64ModuloRange(a, descending); ok {
			return indexes, true, nil
		}
		values, owned, ok := tryBulkI64Values(a)
		if !ok || len(values) < a.len {
			bulkI64Release(values, owned)
			return nil, false, nil
		}
		indexes := typedSortIndexesI64Keys(values[:a.len], descending)
		bulkI64Release(values, owned)
		return indexes, true, nil
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
	case columnArray[string]:
		return typedSortIndexesBy(a.data, descending, compareString), true, nil
	case columnArray[Symbol]:
		return typedSortIndexesBy(a.data, descending, func(left, right Symbol) int {
			return compareString(string(left), string(right))
		}), true, nil
	case columnArray[Month]:
		return typedSortIndexesBy(a.data, descending, compareTypedSigned[Month]), true, nil
	case columnArray[Date]:
		return typedSortIndexesBy(a.data, descending, compareTypedSigned[Date]), true, nil
	case columnArray[DateTime]:
		return typedSortIndexesBy(a.data, descending, compareTypedSigned[DateTime]), true, nil
	case columnArray[Timespan]:
		return typedSortIndexesBy(a.data, descending, compareTypedSigned[Timespan]), true, nil
	case columnArray[Minute]:
		return typedSortIndexesBy(a.data, descending, compareTypedSigned[Minute]), true, nil
	case columnArray[Second]:
		return typedSortIndexesBy(a.data, descending, compareTypedSigned[Second]), true, nil
	case columnArray[Time]:
		return typedSortIndexesBy(a.data, descending, compareTypedSigned[Time]), true, nil
	case columnArray[Timestamp]:
		return typedSortIndexesBy(a.data, descending, compareTypedSigned[Timestamp]), true, nil
	default:
		return nil, false, nil
	}
}

// TryTypedSortIndexSumI64 reduces iasc/idesc directly. The sum of a stable
// permutation's row indexes is invariant, so supported sortable arrays do not
// need to materialize the index vector before a +/ reduction.
func TryTypedSortIndexSumI64(array Array, descending bool) (int64, bool, error) {
	if array == nil {
		return 0, true, fmt.Errorf("sort index sum array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedSortIndexSumI64(a.array, descending)
	case tiledArray:
		if a.len > 0 && a.source.Len() == 0 {
			return 0, true, fmt.Errorf("sort index tiled source is empty")
		}
		if _, handled, err := TryTypedSortIndexSumI64(a.source, descending); err != nil || !handled {
			return 0, handled, err
		}
		return arithmeticSeriesSum(a.len), true, nil
	case nullableArray,
		nullBitmapCarrier,
		columnArray[int64],
		i64RangeArray,
		columnArray[string],
		columnArray[Symbol],
		columnArray[Month],
		columnArray[Date],
		columnArray[DateTime],
		columnArray[Timespan],
		columnArray[Minute],
		columnArray[Second],
		columnArray[Time],
		columnArray[Timestamp]:
		return arithmeticSeriesSum(array.Len()), true, nil
	default:
		return 0, false, nil
	}
}

// TryTypedRankSumI64 reduces +/rank directly. q rank produces a permutation of
// row positions for supported sortable arrays, so the sum is invariant.
func TryTypedRankSumI64(array Array) (int64, bool, error) {
	if array == nil {
		return 0, true, fmt.Errorf("rank sum array is nil")
	}
	if _, handled, err := TryTypedSortIndexSumI64(array, false); err != nil || !handled {
		return 0, handled, err
	}
	return arithmeticSeriesSum(array.Len()), true, nil
}

// TryTypedSortedEdge returns first/last asc/desc without materializing the
// sorted vector. It keeps the sort-index semantics, including typed temporal
// null ordering, but gathers only one row.
func TryTypedSortedEdge(array Array, descending bool, last bool) (any, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("sorted edge array is nil")
	}
	if array.Len() == 0 {
		return NullValue, true, nil
	}
	wantMax := descending != last
	switch a := array.(type) {
	case attributedArray:
		return TryTypedSortedEdge(a.array, descending, last)
	case i64RangeArray:
		row := 0
		if (a.step >= 0 && wantMax) || (a.step < 0 && !wantMax) {
			row = a.len - 1
		}
		return a.start + int64(row)*a.step, true, nil
	case columnArray[int64]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[int64]), true, nil
	case columnArray[string]:
		return typedSortedEdgeBy(a.data, wantMax, compareString), true, nil
	case columnArray[Symbol]:
		return typedSortedEdgeBy(a.data, wantMax, func(left, right Symbol) int {
			return compareString(string(left), string(right))
		}), true, nil
	case columnArray[Month]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[Month]), true, nil
	case columnArray[Date]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[Date]), true, nil
	case columnArray[DateTime]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[DateTime]), true, nil
	case columnArray[Timespan]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[Timespan]), true, nil
	case columnArray[Minute]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[Minute]), true, nil
	case columnArray[Second]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[Second]), true, nil
	case columnArray[Time]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[Time]), true, nil
	case columnArray[Timestamp]:
		return typedSortedEdgeBy(a.data, wantMax, compareTypedSigned[Timestamp]), true, nil
	}
	indexes, handled, err := TryTypedSortIndexesI64(array, descending)
	if err != nil || !handled {
		return nil, handled, err
	}
	row := 0
	if last {
		row = indexes.Len() - 1
	}
	value, ok := indexes.At(row)
	if !ok {
		return nil, true, fmt.Errorf("sorted edge index %d out of bounds", row)
	}
	var rawIndex int64
	switch x := value.(type) {
	case int64:
		rawIndex = x
	case int:
		rawIndex = int64(x)
	default:
		return nil, true, fmt.Errorf("sorted edge index has type %T", value)
	}
	index, err := checkedI64Index(rawIndex)
	if err != nil {
		return nil, true, err
	}
	out, ok := array.At(index)
	if !ok {
		return nil, true, fmt.Errorf("sorted edge row %d out of bounds", index)
	}
	return out, true, nil
}

func typedSortedEdgeBy[T any](values []T, wantMax bool, compare func(T, T) int) T {
	best := values[0]
	for _, value := range values[1:] {
		cmp := compare(value, best)
		if (wantMax && cmp > 0) || (!wantMax && cmp < 0) {
			best = value
		}
	}
	return best
}

func arithmeticSeriesSum(n int) int64 {
	if n <= 1 {
		return 0
	}
	count := int64(n)
	return count * (count - 1) / 2
}

func typedSortIndexesTiled(array tiledArray, descending bool) (Array, bool, error) {
	if array.len == 0 {
		return NewI64Range(0, 1, 0), true, nil
	}
	sourceLen := array.source.Len()
	if sourceLen == 0 {
		return nil, true, fmt.Errorf("sort index tiled source is empty")
	}
	if sourceLen == 1 {
		return NewI64Range(0, 1, array.len), true, nil
	}
	offsets := make([]int, sourceLen)
	for i := range offsets {
		offsets[i] = i
	}
	sort.SliceStable(offsets, func(i, j int) bool {
		left := (array.start + offsets[i]) % sourceLen
		right := (array.start + offsets[j]) % sourceLen
		cmp := compareArrayRows(array.source, left, right)
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})
	out := make([]int64, 0, array.len)
	fullCycles := array.len / sourceLen
	remainder := array.len % sourceLen
	for groupStart := 0; groupStart < len(offsets); {
		groupEnd := groupStart + 1
		for groupEnd < len(offsets) {
			left := (array.start + offsets[groupStart]) % sourceLen
			right := (array.start + offsets[groupEnd]) % sourceLen
			if compareArrayRows(array.source, left, right) != 0 {
				break
			}
			groupEnd++
		}
		for cycle := 0; cycle < fullCycles; cycle++ {
			base := cycle * sourceLen
			for _, offset := range offsets[groupStart:groupEnd] {
				out = append(out, int64(base+offset))
			}
		}
		base := fullCycles * sourceLen
		for _, offset := range offsets[groupStart:groupEnd] {
			if offset < remainder {
				out = append(out, int64(base+offset))
			}
		}
		groupStart = groupEnd
	}
	return newI64Trusted(out), true, nil
}

// TryTypedRankI64 returns q rank positions for typed arrays without boxing the
// source values through []any.
func TryTypedRankI64(array Array) (Array, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("rank array is nil")
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedRankI64(a.array)
	case i64RangeArray:
		if a.len == 0 {
			return NewI64Range(0, 1, 0), true, nil
		}
		if a.step >= 0 {
			return NewI64Range(0, 1, a.len), true, nil
		}
		return NewI64Range(int64(a.len-1), -1, a.len), true, nil
	}
	indexes, handled, err := TryTypedSortIndexesI64(array, false)
	if err != nil || !handled {
		return nil, handled, err
	}
	rows, handled, err := TryTypedI64Indexes(indexes)
	if err != nil || !handled {
		return nil, handled, err
	}
	out := make([]int64, len(rows))
	for sortedPosition, originalIndex := range rows {
		out[originalIndex] = int64(sortedPosition)
	}
	return newI64Trusted(out), true, nil
}

func typedSortIndexesBy[T any](values []T, descending bool, compare func(T, T) int) Array {
	if len(values) == 0 {
		return NewI64Range(0, 1, 0)
	}
	indexes := make([]int64, len(values))
	for i := range indexes {
		indexes[i] = int64(i)
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		cmp := compare(values[int(indexes[i])], values[int(indexes[j])])
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})
	return newI64Trusted(indexes)
}

func typedSortRowIndexesByArray(array Array, descending bool) Array {
	if array.Len() == 0 {
		return NewI64Range(0, 1, 0)
	}
	indexes := make([]int64, array.Len())
	for i := range indexes {
		indexes[i] = int64(i)
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		cmp := compareArrayRows(array, int(indexes[i]), int(indexes[j]))
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})
	return newI64Trusted(indexes)
}

func compareTypedSigned[T signedScalar](left, right T) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
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

// TryTypedFbySumTotal returns +/sum values fby groups without materializing the
// broadcast fby result. Tiled groups use a compact source group map and stream
// row values directly.
func TryTypedFbySumTotal(values, groups Array) (any, bool, error) {
	if values == nil || groups == nil {
		return nil, true, fmt.Errorf("fby sum arrays must be non-nil")
	}
	if values.Len() != groups.Len() {
		return nil, true, fmt.Errorf("fby sum length mismatch: %d != %d", values.Len(), groups.Len())
	}
	return typedKernels.FbySumTotal(values, groups)
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
	case i64SparseAmendArray:
		return fbySumI64SparseAmend(v, groups)
	case i64DyadicProductArray:
		return fbySumI64DyadicProduct(v, groups)
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
		if isDenseIntegerArray(values) {
			return fbySumIntegerArray(values, groups)
		}
		return nil, false, nil
	}
}

func (typedKernelRegistry) FbySumTotal(values, groups Array) (any, bool, error) {
	switch v := values.(type) {
	case attributedArray:
		return typedKernels.FbySumTotal(v.array, groups)
	case columnArray[int8]:
		return fbySumTotalIntegral(v.data, groups)
	case columnArray[int16]:
		return fbySumTotalIntegral(v.data, groups)
	case columnArray[int32]:
		return fbySumTotalIntegral(v.data, groups)
	case columnArray[int64]:
		return fbySumTotalIntegral(v.data, groups)
	case i64RangeArray:
		return fbySumTotalI64Range(v, groups)
	case i64SparseAmendArray:
		return fbySumTotalI64SparseAmend(v, groups)
	case i64DyadicProductArray:
		return fbySumTotalI64DyadicProduct(v, groups)
	case columnArray[uint8]:
		return fbySumTotalIntegral(v.data, groups)
	case columnArray[uint16]:
		return fbySumTotalIntegral(v.data, groups)
	case columnArray[uint32]:
		return fbySumTotalIntegral(v.data, groups)
	case columnArray[uint64]:
		return fbySumTotalIntegral(v.data, groups)
	case columnArray[float32]:
		return fbySumTotalFloat(v.data, groups)
	case columnArray[float64]:
		return fbySumTotalFloat(v.data, groups)
	default:
		if isDenseIntegerArray(values) {
			return fbySumTotalIntegerArray(values, groups)
		}
		return nil, false, nil
	}
}

// TryTypedGroupCount returns the number of q group keys without building the
// grouped dictionary payload.
func TryTypedGroupCount(array Array) (int64, bool, error) {
	return TryTypedDistinctCount(array)
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
	case i64DyadicProductArray:
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
		return numericSumValueByAccess(a)
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
	case i64BucketArray:
		return i64BucketSum(a)
	case i64XrankArray:
		return i64XrankSum(a)
	case i64ScalarDyadicArray:
		if value, handled, err := i64ScalarDyadicSum(a); err != nil || handled {
			return value, handled, err
		}
		return numericSumValueByAccess(a)
	case indexedArray:
		if value, handled, err := indexedArrayIntegerSum(a); err != nil || handled {
			return value, handled, err
		}
		return numericSumValueByAccess(a)
	case i64ScalarDyadicRunningSumArray:
		return i64ScalarDyadicRunningSumSum(a)
	case i64SparseAmendArray:
		return i64SparseAmendSum(a)
	case i64FillArray:
		return a.sum(), true, nil
	case fbyI64BroadcastArray:
		return a.total(), true, nil
	case fbyI64TiledBroadcastArray:
		return a.total(), true, nil
	case f64RangeArray:
		return f64RangeSum(a), true, nil
	case f64NumericDyadicArray:
		return f64NumericDyadicSum(a)
	case qRatiosArray:
		return TryTypedRatiosSum(a.source)
	case f64BucketArray:
		return f64BucketSum(a)
	case f64FillArray:
		return a.sum(), true, nil
	case fbyF64BroadcastArray:
		return a.total(), true, nil
	case fbyF64TiledBroadcastArray:
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
	case i64DyadicProductArray:
		return i64DyadicProductSum(a)
	case crossPairArray:
		return numericSumCrossPairValue(a)
	case matrixRowArray:
		return numericSumMatrixRowValue(a)
	case transposedMatrixRowArray:
		return numericSumTransposedMatrixRowValue(a)
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
	case nullBitmapArray[int8]:
		return nullBitmapSumInteger(a), true, nil
	case nullBitmapArray[int16]:
		return nullBitmapSumInteger(a), true, nil
	case nullBitmapArray[int32]:
		return nullBitmapSumInteger(a), true, nil
	case nullBitmapArray[int64]:
		return nullBitmapSumInteger(a), true, nil
	case nullBitmapArray[float32]:
		return nullBitmapSumFloat(a), true, nil
	case nullBitmapArray[float64]:
		return nullBitmapSumFloat(a), true, nil
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
		return numericSumValueByAccess(array)
	}
}

// indexedArrayIntegerSum reduces a gathered integer view without per-row
// interface dispatch. Range sources use the affine closed form
// sum = start*len + step*sum(indexes); index carriers with exact closed-form
// sums (ranges, segments, periodic where-results) stay O(carrier), and dense
// cases flatten through the bulk kernels.
func indexedArrayIntegerSum(a indexedArray) (any, bool, error) {
	source := a.source
	if attributed, ok := source.(attributedArray); ok {
		source = attributed.array
	}
	if values, ok := source.(i64RangeArray); ok {
		switch a.indexes.(type) {
		case i64RangeArray, i64SegmentArray, i64PeriodicIndexArray:
			return values.start*int64(a.len) + values.step*i64IndexArraySum(a.indexes), true, nil
		}
		if rows, ok := i64Int32IndexRows(a.indexes); ok {
			var total int64
			for _, index := range rows {
				if index < 0 || int(index) >= values.len {
					return nil, false, nil
				}
				total += int64(index)
			}
			return values.start*int64(a.len) + values.step*total, true, nil
		}
		indexes, owned, ok := tryBulkI64Values(a.indexes)
		if !ok || len(indexes) != a.len {
			bulkI64Release(indexes, owned)
			return nil, false, nil
		}
		var total int64
		for _, index := range indexes {
			if index < 0 || index >= int64(values.len) {
				bulkI64Release(indexes, owned)
				return nil, false, nil
			}
			total += index
		}
		bulkI64Release(indexes, owned)
		return values.start*int64(a.len) + values.step*total, true, nil
	}
	if !isIntegerArray(source) {
		return nil, false, nil
	}
	sourceValues, sourceOwned, ok := tryBulkI64Values(source)
	if !ok {
		return nil, false, nil
	}
	if rows, ok := i64Int32IndexRows(a.indexes); ok {
		var total int64
		for _, index := range rows {
			if index < 0 || int(index) >= len(sourceValues) {
				bulkI64Release(sourceValues, sourceOwned)
				return nil, false, nil
			}
			total += sourceValues[index]
		}
		bulkI64Release(sourceValues, sourceOwned)
		return total, true, nil
	}
	indexes, indexesOwned, ok := tryBulkI64Values(a.indexes)
	if !ok || len(indexes) != a.len {
		bulkI64Release(sourceValues, sourceOwned)
		bulkI64Release(indexes, indexesOwned)
		return nil, false, nil
	}
	var total int64
	for _, index := range indexes {
		if index < 0 || index >= int64(len(sourceValues)) {
			bulkI64Release(sourceValues, sourceOwned)
			bulkI64Release(indexes, indexesOwned)
			return nil, false, nil
		}
		total += sourceValues[index]
	}
	bulkI64Release(sourceValues, sourceOwned)
	bulkI64Release(indexes, indexesOwned)
	return total, true, nil
}

func numericSumValueByAccess(array Array) (any, bool, error) {
	if array == nil {
		return nil, false, nil
	}
	if isIntegerArray(array) {
		if values, nulls, owned, ok := tryBulkI64NullableValues(array); ok {
			var total int64
			for i, value := range values {
				if nulls != nil && nullBitGet(nulls, i) {
					continue
				}
				total += value
			}
			bulkI64Release(values, owned)
			return total, true, nil
		}
		var total int64
		for row := 0; row < array.Len(); row++ {
			value, ok, err := integerArrayAt(array, row)
			if err != nil {
				return nil, true, err
			}
			if ok {
				total += value
			}
		}
		return total, true, nil
	}
	if isNumericArray(array) {
		if values, nulls, owned, ok := tryBulkF64NullableValues(array); ok {
			var total float64
			for i, value := range values {
				if nulls != nil && nullBitGet(nulls, i) {
					continue
				}
				total += value
			}
			bulkF64Release(values, owned)
			return total, true, nil
		}
		var total float64
		for row := 0; row < array.Len(); row++ {
			value, ok, err := typedKernels.NumericAt(array, row)
			if err != nil {
				return nil, true, err
			}
			if ok {
				total += value
			}
		}
		return total, true, nil
	}
	return nil, false, nil
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
		if err != nil {
			return 0, false, err
		}
		if !ok {
			// Null row: sum skips nulls.
			continue
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
	if values, ok := asI64RangeArray(array); ok {
		if average {
			return movingAvgSumI64Range(values, width), true, nil
		}
		return movingSumSumI64Range(values, width), true, nil
	}
	if sparse, ok := array.(i64SparseAmendArray); ok && isZeroLikeI64Array(sparse.source) {
		if average {
			return movingAvgSumI64SparseZeroAmend(sparse, width), true, nil
		}
		return movingSumSumI64SparseZeroAmend(sparse, width), true, nil
	}
	if fill, ok := array.(i64FillArray); ok {
		if out, handled := movingNumericSumSumI64Fill(fill, width, average); handled {
			return out, true, nil
		}
	}
	if array.Kind() != KindF64 && array.Kind() != KindF32 {
		// Null-free integer carriers flatten once and run the identical
		// sliding-window accumulation over a dense slice, replacing the
		// per-row integerArrayAt dispatch below. tryBulkI64Values declines
		// on nulls, so the fallback semantics are unchanged. The mavg branch
		// keeps the boxed route's per-window arithmetic bit-for-bit: an int64
		// sliding window divided by the same prefix-clamped count per row.
		if values, owned, ok := tryBulkI64Values(array); ok {
			if average {
				var window int64
				var total float64
				for row, value := range values {
					window += value
					if row >= width {
						window -= values[row-width]
					}
					count := row + 1
					if count > width {
						count = width
					}
					total += float64(window) / float64(count)
				}
				bulkI64Release(values, owned)
				return total, true, nil
			}
			var window, total int64
			for row, value := range values {
				window += value
				if row >= width {
					window -= values[row-width]
				}
				total += window
			}
			bulkI64Release(values, owned)
			return total, true, nil
		}
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

// TryTypedMovingFillsScalarFillSum computes sum(width msum/mavg (fill ^ fills
// array)) without materializing either the forward-filled column or the scalar
// filled column. It is a generic columnar pipeline kernel for q shapes such as
// `+/20 msum 0^fills c`.
func TryTypedMovingFillsScalarFillSum(array Array, fill int64, width int, average bool) (any, bool, error) {
	if array == nil {
		return nil, false, nil
	}
	if width <= 0 {
		return nil, true, fmt.Errorf("moving numeric width must be positive")
	}
	values, nulls, owned, ok := tryBulkI64NullableValues(array)
	if !ok || len(values) != array.Len() {
		bulkI64Release(values, owned)
		return nil, false, nil
	}
	ring := bulkI64Get(width)
	for i := range ring {
		ring[i] = 0
	}
	last := fill
	var window int64
	var totalI int64
	var totalF float64
	for row, value := range values {
		if nulls == nil || !nullBitGet(nulls, row) {
			last = value
		}
		window += last
		if row >= width {
			window -= ring[row%width]
		}
		ring[row%width] = last
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
	bulkI64Release(ring, true)
	bulkI64Release(values, owned)
	if average {
		return totalF, true, nil
	}
	return totalI, true, nil
}

func movingSumSumI64Range(values i64RangeArray, width int) int64 {
	n := values.len
	if n == 0 {
		return 0
	}
	if width >= n {
		var total int64
		for row := 0; row < n; row++ {
			value := values.start + int64(row)*values.step
			total += value * int64(n-row)
		}
		return total
	}
	stableLen := n - width + 1
	total := int64(width) * i64RangeSum(i64RangeArray{
		start: values.start,
		step:  values.step,
		len:   stableLen,
	})
	for row := stableLen; row < n; row++ {
		value := values.start + int64(row)*values.step
		total += value * int64(n-row)
	}
	return total
}

func movingNumericSumSumI64Fill(array i64FillArray, width int, average bool) (any, bool) {
	if width <= 0 || array.source == nil {
		return nil, false
	}
	values, nulls, owned, ok := tryBulkI64NullableValues(array.source)
	if !ok || len(values) != array.Len() {
		bulkI64Release(values, owned)
		return nil, false
	}
	ring := bulkI64Get(width)
	for i := range ring {
		ring[i] = 0
	}
	var window int64
	var totalI int64
	var totalF float64
	for row, value := range values {
		if nulls != nil && nullBitGet(nulls, row) {
			value = array.fill
		}
		window += value
		if row >= width {
			window -= ring[row%width]
		}
		ring[row%width] = value
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
	bulkI64Release(ring, true)
	bulkI64Release(values, owned)
	if average {
		return totalF, true
	}
	return totalI, true
}

func movingAvgSumI64Range(values i64RangeArray, width int) float64 {
	n := values.len
	if n == 0 {
		return 0
	}
	prefixLen := width - 1
	if prefixLen > n {
		prefixLen = n
	}
	total := 0.0
	for row := 0; row < prefixLen; row++ {
		total += float64(values.start) + float64(values.step)*float64(row)/2
	}
	if width > n {
		return total
	}
	stableLen := n - width + 1
	firstStable := float64(values.start) + float64(values.step)*float64(width-1)/2
	lastStable := firstStable + float64(stableLen-1)*float64(values.step)
	return total + float64(stableLen)*(firstStable+lastStable)/2
}

func movingSumSumI64SparseZeroAmend(values i64SparseAmendArray, width int) int64 {
	n := values.Len()
	var total int64
	for i, row := range values.indexes {
		if row < 0 || row >= n {
			continue
		}
		count := width
		if remaining := n - row; remaining < count {
			count = remaining
		}
		total += values.values[i] * int64(count)
	}
	return total
}

func movingAvgSumI64SparseZeroAmend(values i64SparseAmendArray, width int) float64 {
	n := values.Len()
	var total float64
	for i, row := range values.indexes {
		if row < 0 || row >= n {
			continue
		}
		last := row + width - 1
		if last >= n {
			last = n - 1
		}
		value := float64(values.values[i])
		for outRow := row; outRow <= last; outRow++ {
			count := outRow + 1
			if count > width {
				count = width
			}
			total += value / float64(count)
		}
	}
	return total
}

// TryTypedDeltas applies q-style deltas for dense typed numeric arrays.
func TryTypedDeltas(array Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedDeltas(a.array)
	case columnArray[int8]:
		return deltasSignedTypedSlice(a.kind, a.data), true, nil
	case columnArray[int16]:
		return deltasSignedTypedSlice(a.kind, a.data), true, nil
	case columnArray[int32]:
		return deltasSignedTypedSlice(a.kind, a.data), true, nil
	case columnArray[int64]:
		return deltasI64Slice(a.data), true, nil
	case i64RangeArray:
		return deltasI64Range(a), true, nil
	case columnArray[uint8]:
		return deltasUnsignedSlice(a.data), true, nil
	case columnArray[uint16]:
		return deltasUnsignedSlice(a.data), true, nil
	case columnArray[uint32]:
		return deltasUnsignedSlice(a.data), true, nil
	case columnArray[uint64]:
		return deltasUnsignedSlice(a.data), true, nil
	case columnArray[float32]:
		return deltasFloatSlice(a.data), true, nil
	case columnArray[float64]:
		return deltasFloatSlice(a.data), true, nil
	case nullableArray:
		return deltasNullableArray(a)
	default:
		if carrier, ok := asNullBitmapCarrier(array); ok {
			return carrier.deltas(), true, nil
		}
		// Lazy numeric carriers (scalar-dyadic chains, tiled views, ...) flatten
		// through the typed each-prior kernel: deltas is each-prior subtraction
		// over a null-free vector.
		return TryTypedEachPriorDyadic(OpSub, nil, array)
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
	case nullableArray:
		return deltasNullableSum(a)
	default:
		if carrier, ok := asNullBitmapCarrier(array); ok {
			return deltasNullableSum(nullableArray{kind: carrier.Kind(), data: carrier.Values()})
		}
		return nil, false, nil
	}
}

// TryTypedRatiosSum applies sum ratios x without materializing the ratio
// vector. q ratios preserve the first non-null item and divide each subsequent
// item by its previous row; sum ignores null ratio rows.
func TryTypedRatiosSum(array Array) (any, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return TryTypedRatiosSum(a.array)
	case qRatiosArray:
		return TryTypedRatiosSum(a.source)
	}
	if !isNumericArray(array) {
		return nil, false, nil
	}
	producer, err := newF64NumericProducer(array, array.Len())
	if err != nil {
		return nil, true, err
	}
	total, err := f64ProducerRatiosSum(producer)
	if err != nil {
		return nil, true, err
	}
	return total, true, nil
}

// TryTypedQRatios returns a lazy typed carrier for q's ratios primitive. This
// keeps q list evaluation on the shared data numeric producer path without
// changing data.VectorTransformExpr("ratios"), whose first-row null semantics
// are query-engine specific.
func TryTypedQRatios(array Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		out, handled, err := TryTypedQRatios(a.array)
		if err != nil || !handled {
			return out, handled, err
		}
		return a.withLazyRebuiltIndexes(out), true, nil
	}
	if !isNumericArray(array) {
		return nil, false, nil
	}
	return qRatiosArray{source: array}, true, nil
}

type qRatiosArray struct {
	source Array
}

func (a qRatiosArray) Kind() Kind { return KindF64 }

func (a qRatiosArray) Len() int { return a.source.Len() }

func (a qRatiosArray) At(row int) (any, bool) {
	value, ok, err := a.f64At(row)
	if err != nil || !ok {
		return NullValue, err == nil
	}
	return value, true
}

func (a qRatiosArray) Values() []any {
	out := make([]any, a.Len())
	for row := range out {
		value, ok, err := a.f64At(row)
		if err != nil {
			panic(fmt.Sprintf("data q ratios row %d: %v", row, err))
		}
		if !ok {
			out[row] = NullValue
			continue
		}
		out[row] = value
	}
	return out
}

func (a qRatiosArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.f64At(row)
		if err != nil {
			panic(fmt.Sprintf("data q ratios gather index %d: %v", row, err))
		}
		if !ok {
			out[i] = NullValue
			continue
		}
		out[i] = value
	}
	return nullableArray{kind: KindF64, data: out}
}

func (a qRatiosArray) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= a.Len() {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	current, currentOK, err := typedKernels.NumericAt(a.source, row)
	if err != nil || !currentOK {
		return 0, currentOK, err
	}
	if row == 0 {
		return current, true, nil
	}
	previous, previousOK, err := typedKernels.NumericAt(a.source, row-1)
	if err != nil {
		return 0, false, err
	}
	if !previousOK {
		return current, true, nil
	}
	return current / previous, true, nil
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
		if carrier, ok := asNullBitmapCarrier(array); ok {
			return k.NumericSums(nullableArray{kind: carrier.Kind(), data: carrier.Values()})
		}
		// Bulk fallback: flatten lazy carriers (gathers, buckets, tiles, ...)
		// once and stream the running sum in a dense pass. The flatteners bail
		// out on null rows, so the boxed null-aware fallback is preserved. The
		// fold runs in source order with the same accumulator types as the
		// dense column paths above, so results match bit-for-bit.
		switch array.Kind() {
		case KindI64:
			if values, owned, ok := tryBulkI64Values(array); ok {
				out := make([]int64, len(values))
				var acc int64
				for i, v := range values {
					acc += v
					out[i] = acc
				}
				bulkI64Release(values, owned)
				return newI64Trusted(out), true, nil
			}
		case KindF64:
			if values, owned, ok := tryBulkF64Values(array); ok {
				out := make([]float64, len(values))
				var acc float64
				for i, v := range values {
					acc += v
					out[i] = acc
				}
				bulkF64Release(values, owned)
				return newF64Trusted(out), true, nil
			}
		}
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
	// Bulk fast path: the i64FillArray flatten applies the exact valueAt
	// fill/value selection per row, so summing the dense slice in row order
	// is bit-identical to the boxed loop below.
	if values, owned, ok := tryBulkI64Values(a); ok {
		var total int64
		for _, v := range values {
			total += v
		}
		bulkI64Release(values, owned)
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
		if carrier, ok := asNullBitmapCarrier(array); ok && carrier.isIntegerCarrier() {
			if values, nulls, owned, ok := tryBulkI64NullableValues(carrier); ok {
				var total int64
				for i, v := range values {
					if nulls != nil && nullBitGet(nulls, i) {
						total += fill
						continue
					}
					total += v
				}
				bulkI64Release(values, owned)
				return total, true
			}
		}
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
	// Bitmap-backed sources flatten once: non-null rows add their value and
	// null rows add the fill, with no per-row boxed Array.At walk.
	if nullBitmapBackedArray(a.source) {
		if values, nulls, owned, ok := tryBulkF64NullableValues(a.source); ok {
			var total float64
			for i, value := range values {
				if nulls != nil && nullBitGet(nulls, i) {
					total += a.fill
					continue
				}
				total += value
			}
			bulkF64Release(values, owned)
			return total
		}
	}
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
	counts    []int64
	len       int
}

type fbyI64TiledBroadcastArray struct {
	sourceGroups []int
	sums         []int64
	counts       []int64
	start        int
	sourceLen    int
	len          int
}

func (a fbyI64BroadcastArray) Kind() Kind { return KindI64 }

func (a fbyI64BroadcastArray) Len() int { return a.len }

func (a fbyI64BroadcastArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
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
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data fby gather index %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a fbyI64BroadcastArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return a.sums[a.rowGroups[row]], true, nil
}

func (a fbyI64BroadcastArray) total() int64 {
	var total int64
	for group, sum := range a.sums {
		total += sum * a.counts[group]
	}
	return total
}

func (a fbyI64TiledBroadcastArray) Kind() Kind { return KindI64 }

func (a fbyI64TiledBroadcastArray) Len() int { return a.len }

func (a fbyI64TiledBroadcastArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a fbyI64TiledBroadcastArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		out[row] = a.sums[a.sourceGroups[(a.start+row)%a.sourceLen]]
	}
	return out
}

func (a fbyI64TiledBroadcastArray) Gather(indexes []int) Array {
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data fby gather index %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a fbyI64TiledBroadcastArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.len || a.sourceLen <= 0 {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return a.sums[a.sourceGroups[(a.start+row)%a.sourceLen]], true, nil
}

func (a fbyI64TiledBroadcastArray) total() int64 {
	var total int64
	for group, sum := range a.sums {
		total += sum * a.counts[group]
	}
	return total
}

type fbyF64BroadcastArray struct {
	rowGroups []int
	sums      []float64
	counts    []int64
	len       int
}

type fbyF64TiledBroadcastArray struct {
	sourceGroups []int
	sums         []float64
	counts       []int64
	start        int
	sourceLen    int
	len          int
}

func (a fbyF64BroadcastArray) Kind() Kind { return KindF64 }

func (a fbyF64BroadcastArray) Len() int { return a.len }

func (a fbyF64BroadcastArray) At(row int) (any, bool) {
	value, ok, err := a.f64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
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
		value, ok, err := a.f64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data fby gather index %d out of range", row))
		}
		out[i] = value
	}
	return newF64Trusted(out)
}

func (a fbyF64BroadcastArray) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= a.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return a.sums[a.rowGroups[row]], true, nil
}

func (a fbyF64BroadcastArray) total() float64 {
	var total float64
	for group, sum := range a.sums {
		total += sum * float64(a.counts[group])
	}
	return total
}

func (a fbyF64TiledBroadcastArray) Kind() Kind { return KindF64 }

func (a fbyF64TiledBroadcastArray) Len() int { return a.len }

func (a fbyF64TiledBroadcastArray) At(row int) (any, bool) {
	value, ok, err := a.f64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a fbyF64TiledBroadcastArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		out[row] = a.sums[a.sourceGroups[(a.start+row)%a.sourceLen]]
	}
	return out
}

func (a fbyF64TiledBroadcastArray) Gather(indexes []int) Array {
	out := make([]float64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.f64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data fby gather index %d out of range", row))
		}
		out[i] = value
	}
	return newF64Trusted(out)
}

func (a fbyF64TiledBroadcastArray) f64At(row int) (float64, bool, error) {
	if row < 0 || row >= a.len || a.sourceLen <= 0 {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	return a.sums[a.sourceGroups[(a.start+row)%a.sourceLen]], true, nil
}

func (a fbyF64TiledBroadcastArray) total() float64 {
	var total float64
	for group, sum := range a.sums {
		total += sum * float64(a.counts[group])
	}
	return total
}

func fbySumTotalIntegral[T signedScalar | unsignedScalar](values []T, groups Array) (any, bool, error) {
	// Dense comparable group carriers: pooled id vector plus a tight slice
	// loop (no per-row closure dispatch, no per-call id allocation).
	if ids, groupCount, ok, err := fbySumRowGroupIDs(groups); ok || err != nil {
		if err != nil {
			return nil, true, err
		}
		if len(ids) != len(values) {
			bulkIntRelease(ids)
			return nil, true, fmt.Errorf("fby sum group ids length mismatch: %d != %d", len(ids), len(values))
		}
		sums := bulkI64Get(groupCount)
		clear(sums)
		counts := bulkI64Get(groupCount)
		clear(counts)
		for row, value := range values {
			group := ids[row]
			sums[group] += int64(value)
			counts[group]++
		}
		var total int64
		for group, sum := range sums {
			total += sum * counts[group]
		}
		bulkI64Release(sums, true)
		bulkI64Release(counts, true)
		bulkIntRelease(ids)
		return total, true, nil
	}
	lookup, groupCount, ok, err := fbyGroupLookup(groups)
	if err != nil || !ok {
		return nil, ok, err
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	for row, value := range values {
		group, err := lookup(row)
		if err != nil {
			return nil, true, err
		}
		sums[group] += int64(value)
		counts[group]++
	}
	var total int64
	for group, sum := range sums {
		total += sum * counts[group]
	}
	return total, true, nil
}

func fbySumTotalI64Range(values i64RangeArray, groups Array) (any, bool, error) {
	lookup, groupCount, ok, err := fbyGroupLookup(groups)
	if err != nil || !ok {
		return nil, ok, err
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	for row := 0; row < values.len; row++ {
		group, err := lookup(row)
		if err != nil {
			return nil, true, err
		}
		sums[group] += values.start + int64(row)*values.step
		counts[group]++
	}
	var total int64
	for group, sum := range sums {
		total += sum * counts[group]
	}
	return total, true, nil
}

func fbySumTotalFloat[T floatScalar](values []T, groups Array) (any, bool, error) {
	// Dense comparable group carriers: pooled id vector plus a tight slice
	// loop (no per-row closure dispatch, no per-call id allocation).
	if ids, groupCount, ok, err := fbySumRowGroupIDs(groups); ok || err != nil {
		if err != nil {
			return nil, true, err
		}
		if len(ids) != len(values) {
			bulkIntRelease(ids)
			return nil, true, fmt.Errorf("fby sum group ids length mismatch: %d != %d", len(ids), len(values))
		}
		sums := bulkF64Get(groupCount)
		clear(sums)
		counts := bulkI64Get(groupCount)
		clear(counts)
		for row, value := range values {
			group := ids[row]
			sums[group] += float64(value)
			counts[group]++
		}
		var total float64
		for group, sum := range sums {
			total += sum * float64(counts[group])
		}
		bulkF64Release(sums, true)
		bulkI64Release(counts, true)
		bulkIntRelease(ids)
		return total, true, nil
	}
	lookup, groupCount, ok, err := fbyGroupLookup(groups)
	if err != nil || !ok {
		return nil, ok, err
	}
	sums := make([]float64, groupCount)
	counts := make([]int64, groupCount)
	for row, value := range values {
		group, err := lookup(row)
		if err != nil {
			return nil, true, err
		}
		sums[group] += float64(value)
		counts[group]++
	}
	var total float64
	for group, sum := range sums {
		total += sum * float64(counts[group])
	}
	return total, true, nil
}

func fbySumIntegral[T signedScalar | unsignedScalar](values []T, valueKind Kind, groups Array) (Array, bool, error) {
	if out, ok, err := fbySumIntegralTiled(values, groups); ok || err != nil {
		return out, ok, err
	}
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	for row, value := range values {
		group := rowGroups[row]
		sums[group] += int64(value)
		counts[group]++
	}
	_ = valueKind
	return fbyI64BroadcastArray{rowGroups: rowGroups, sums: sums, counts: counts, len: len(values)}, true, nil
}

func fbySumI64Range(values i64RangeArray, groups Array) (Array, bool, error) {
	if out, ok, err := fbySumI64RangeTiled(values, groups); ok || err != nil {
		return out, ok, err
	}
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	sums := make([]int64, groupCount)
	for row, group := range rowGroups {
		sums[group] += values.start + int64(row)*values.step
	}
	return fbyI64BroadcastArray{rowGroups: rowGroups, sums: sums, counts: fbyGroupCounts(rowGroups, groupCount), len: values.len}, true, nil
}

func fbySumI64SparseAmend(values i64SparseAmendArray, groups Array) (Array, bool, error) {
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	if len(rowGroups) != values.Len() {
		return nil, true, fmt.Errorf("fby group length %d does not match value length %d", len(rowGroups), values.Len())
	}
	sums, _, handled, err := fbySparseAmendGroupSums(values, rowGroups, groupCount)
	if err != nil || !handled {
		return nil, handled, err
	}
	return fbyI64BroadcastArray{rowGroups: rowGroups, sums: sums, counts: fbyGroupCounts(rowGroups, groupCount), len: values.Len()}, true, nil
}

func fbySumI64DyadicProduct(values i64DyadicProductArray, groups Array) (Array, bool, error) {
	if out, ok, err := fbySumI64DyadicProductTiled(values, groups); ok || err != nil {
		return out, ok, err
	}
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	if len(rowGroups) != values.Len() {
		return nil, true, fmt.Errorf("fby group length %d does not match value length %d", len(rowGroups), values.Len())
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	for row, group := range rowGroups {
		value, ok, err := values.i64At(row)
		if err != nil || !ok {
			return nil, ok, err
		}
		sums[group] += value
		counts[group]++
	}
	return fbyI64BroadcastArray{rowGroups: rowGroups, sums: sums, counts: counts, len: values.Len()}, true, nil
}

func fbySumFloat[T floatScalar](values []T, valueKind Kind, groups Array) (Array, bool, error) {
	if out, ok, err := fbySumFloatTiled(values, groups); ok || err != nil {
		return out, ok, err
	}
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	sums := make([]float64, groupCount)
	counts := make([]int64, groupCount)
	for row, value := range values {
		group := rowGroups[row]
		sums[group] += float64(value)
		counts[group]++
	}
	_ = valueKind
	return fbyF64BroadcastArray{rowGroups: rowGroups, sums: sums, counts: counts, len: len(values)}, true, nil
}

func fbySumIntegerArray(values Array, groups Array) (Array, bool, error) {
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	for row, group := range rowGroups {
		value, ok, err := integerArrayAt(values, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		sums[group] += value
		counts[group]++
	}
	return fbyI64BroadcastArray{rowGroups: rowGroups, sums: sums, counts: counts, len: values.Len()}, true, nil
}

func fbySumTotalI64SparseAmend(values i64SparseAmendArray, groups Array) (any, bool, error) {
	rowGroups, groupCount, err := fbyGroupIDs(groups)
	if err != nil {
		return nil, true, err
	}
	if len(rowGroups) != values.Len() {
		return nil, true, fmt.Errorf("fby group length %d does not match value length %d", len(rowGroups), values.Len())
	}
	sums, counts, handled, err := fbySparseAmendGroupSums(values, rowGroups, groupCount)
	if err != nil || !handled {
		return nil, handled, err
	}
	var total int64
	for group, sum := range sums {
		total += sum * counts[group]
	}
	return total, true, nil
}

func fbySumTotalI64DyadicProduct(values i64DyadicProductArray, groups Array) (any, bool, error) {
	if out, ok, err := fbySumI64DyadicProductTiled(values, groups); ok || err != nil {
		if err != nil || !ok {
			return nil, ok, err
		}
		return out.total(), true, nil
	}
	lookup, groupCount, ok, err := fbyGroupLookup(groups)
	if err != nil || !ok {
		return nil, ok, err
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	for row := 0; row < values.Len(); row++ {
		group, err := lookup(row)
		if err != nil {
			return nil, true, err
		}
		value, ok, err := values.i64At(row)
		if err != nil || !ok {
			return nil, ok, err
		}
		sums[group] += value
		counts[group]++
	}
	var total int64
	for group, sum := range sums {
		total += sum * counts[group]
	}
	return total, true, nil
}

func fbySumTotalIntegerArray(values Array, groups Array) (any, bool, error) {
	lookup, groupCount, ok, err := fbyGroupLookup(groups)
	if err != nil || !ok {
		return nil, ok, err
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	for row := 0; row < values.Len(); row++ {
		group, err := lookup(row)
		if err != nil {
			return nil, true, err
		}
		value, ok, err := integerArrayAt(values, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, false, nil
		}
		sums[group] += value
		counts[group]++
	}
	var total int64
	for group, sum := range sums {
		total += sum * counts[group]
	}
	return total, true, nil
}

func fbySparseAmendGroupSums(values i64SparseAmendArray, rowGroups []int, groupCount int) ([]int64, []int64, bool, error) {
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	if isZeroLikeI64Array(values.source) {
		for _, group := range rowGroups {
			counts[group]++
		}
		for i, row := range values.indexes {
			if row < 0 || row >= len(rowGroups) {
				return nil, nil, true, fmt.Errorf("amend index %d out of range", row)
			}
			sums[rowGroups[row]] += values.values[i]
		}
		return sums, counts, true, nil
	}
	for row, group := range rowGroups {
		value, ok, err := integerArrayAt(values.source, row)
		if err != nil || !ok {
			return nil, nil, ok, err
		}
		sums[group] += value
		counts[group]++
	}
	for i, row := range values.indexes {
		if row < 0 || row >= len(rowGroups) {
			return nil, nil, true, fmt.Errorf("amend index %d out of range", row)
		}
		old, ok, err := integerArrayAt(values.source, row)
		if err != nil || !ok {
			return nil, nil, ok, err
		}
		group := rowGroups[row]
		sums[group] += values.values[i] - old
	}
	return sums, counts, true, nil
}

func fbySumIntegralTiled[T signedScalar | unsignedScalar](values []T, groups Array) (Array, bool, error) {
	sourceGroups, groupCount, start, sourceLen, ok, err := fbyTiledSourceGroups(groups)
	if err != nil || !ok {
		return nil, ok, err
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	for row, value := range values {
		group := sourceGroups[(start+row)%sourceLen]
		sums[group] += int64(value)
		counts[group]++
	}
	return fbyI64TiledBroadcastArray{sourceGroups: sourceGroups, sums: sums, counts: counts, start: start, sourceLen: sourceLen, len: len(values)}, true, nil
}

func fbySumI64RangeTiled(values i64RangeArray, groups Array) (Array, bool, error) {
	sourceGroups, groupCount, start, sourceLen, ok, err := fbyTiledSourceGroups(groups)
	if err != nil || !ok {
		return nil, ok, err
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	// Affine closed form per residue class: rows r in [0, len) with
	// (start+r) mod sourceLen == j form an arithmetic progression
	// r0, r0+L, r0+2L, ... so each group's sum over the range carrier
	// start + r*step reduces to O(sourceLen) work instead of O(len).
	n := int64(values.len)
	l := int64(sourceLen)
	for j := 0; j < sourceLen; j++ {
		r0 := (int64(j) - int64(start)) % l
		if r0 < 0 {
			r0 += l
		}
		if r0 >= n {
			continue
		}
		k := (n - 1 - r0) / l // k+1 rows: r0, r0+L, ..., r0+k*L
		rowSum := (k + 1) * r0
		if k%2 == 0 {
			rowSum += (k + 1) * (k / 2) * l
		} else {
			rowSum += ((k + 1) / 2) * k * l
		}
		group := sourceGroups[j]
		sums[group] += (k+1)*values.start + values.step*rowSum
		counts[group] += k + 1
	}
	return fbyI64TiledBroadcastArray{sourceGroups: sourceGroups, sums: sums, counts: counts, start: start, sourceLen: sourceLen, len: values.len}, true, nil
}

func fbySumI64DyadicProductTiled(values i64DyadicProductArray, groups Array) (fbyI64TiledBroadcastArray, bool, error) {
	sourceGroups, groupCount, start, sourceLen, ok, err := fbyTiledSourceGroups(groups)
	if err != nil || !ok {
		return fbyI64TiledBroadcastArray{}, ok, err
	}
	if sourceLen <= 0 {
		return fbyI64TiledBroadcastArray{}, false, nil
	}
	sums := make([]int64, groupCount)
	counts := make([]int64, groupCount)
	if period, ok := fbyI64DyadicProductTiledPeriod(values, sourceLen); ok && period > 0 && period <= 65536 && period < int64(values.Len()) {
		for residue := int64(0); residue < period; residue++ {
			hits := int64(1 + (int64(values.Len())-1-residue)/period)
			row := int(residue)
			group := sourceGroups[(start+row)%sourceLen]
			value, ok, err := values.i64At(row)
			if err != nil || !ok {
				return fbyI64TiledBroadcastArray{}, ok, err
			}
			sums[group] += value * hits
			counts[group] += hits
		}
		return fbyI64TiledBroadcastArray{sourceGroups: sourceGroups, sums: sums, counts: counts, start: start, sourceLen: sourceLen, len: values.Len()}, true, nil
	}
	for row := 0; row < values.Len(); row++ {
		group := sourceGroups[(start+row)%sourceLen]
		value, ok, err := values.i64At(row)
		if err != nil || !ok {
			return fbyI64TiledBroadcastArray{}, ok, err
		}
		sums[group] += value
		counts[group]++
	}
	return fbyI64TiledBroadcastArray{sourceGroups: sourceGroups, sums: sums, counts: counts, start: start, sourceLen: sourceLen, len: values.Len()}, true, nil
}

func fbyI64DyadicProductTiledPeriod(values i64DyadicProductArray, groupSourceLen int) (int64, bool) {
	leftPeriod, ok := fbyI64DyadicProductOperandPeriod(values.left, values.Len())
	if !ok {
		return 0, false
	}
	rightPeriod, ok := fbyI64DyadicProductOperandPeriod(values.right, values.Len())
	if !ok {
		return 0, false
	}
	period, ok := lcmInt64(leftPeriod, rightPeriod)
	if !ok {
		return 0, false
	}
	return lcmInt64(period, int64(groupSourceLen))
}

func fbyI64DyadicProductOperandPeriod(array Array, length int) (int64, bool) {
	switch a := array.(type) {
	case attributedArray:
		return fbyI64DyadicProductOperandPeriod(a.array, length)
	case i64ScalarDyadicArray:
		return i64ScalarDyadicCyclePeriod(a, length)
	case i64DyadicProductArray:
		return fbyI64DyadicProductTiledPeriod(a, 1)
	case tiledArray:
		if a.Len() != length || a.source.Len() <= 0 {
			return 0, false
		}
		return int64(a.source.Len()), true
	default:
		if array.Len() == 1 {
			return 1, true
		}
		return arrayCyclePeriod(array)
	}
}

func i64ScalarDyadicCyclePeriod(array i64ScalarDyadicArray, length int) (int64, bool) {
	if array.Len() != length {
		return 0, false
	}
	switch array.op {
	case OpAdd, OpSub, OpMul:
		return fbyI64DyadicProductOperandPeriod(array.source, length)
	case OpMod:
		if array.scalarLeft || array.scalar <= 0 {
			return 0, false
		}
		var source i64RangeArray
		sourceOK := false
		switch s := array.source.(type) {
		case i64RangeArray:
			source, sourceOK = s, true
		case i64ScalarDyadicArray:
			source, sourceOK = i64ScalarDyadicAffineRange(s)
		}
		if sourceOK {
			period := array.scalar / gcdInt64(source.step, array.scalar)
			if period > 0 {
				return period, true
			}
		}
		return fbyI64DyadicProductOperandPeriod(array.source, length)
	default:
		return 0, false
	}
}

func fbySumFloatTiled[T floatScalar](values []T, groups Array) (Array, bool, error) {
	sourceGroups, groupCount, start, sourceLen, ok, err := fbyTiledSourceGroups(groups)
	if err != nil || !ok {
		return nil, ok, err
	}
	sums := make([]float64, groupCount)
	counts := make([]int64, groupCount)
	for row, value := range values {
		group := sourceGroups[(start+row)%sourceLen]
		sums[group] += float64(value)
		counts[group]++
	}
	return fbyF64TiledBroadcastArray{sourceGroups: sourceGroups, sums: sums, counts: counts, start: start, sourceLen: sourceLen, len: len(values)}, true, nil
}

func fbyGroupCounts(rowGroups []int, groupCount int) []int64 {
	counts := make([]int64, groupCount)
	for _, group := range rowGroups {
		counts[group]++
	}
	return counts
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

type fbyGroupLookupFn func(row int) (int, error)

func fbyTiledSourceGroups(groups Array) ([]int, int, int, int, bool, error) {
	switch g := groups.(type) {
	case attributedArray:
		return fbyTiledSourceGroups(g.array)
	case tiledArray:
		sourceLen := g.source.Len()
		if sourceLen == 0 || g.len == 0 {
			return nil, 0, 0, 0, false, nil
		}
		sourceGroups, groupCount, err := fbyGroupIDs(g.source)
		if err != nil {
			return nil, 0, 0, 0, true, err
		}
		return sourceGroups, groupCount, g.start, sourceLen, true, nil
	default:
		return nil, 0, 0, 0, false, nil
	}
}

func fbyGroupLookup(groups Array) (fbyGroupLookupFn, int, bool, error) {
	switch g := groups.(type) {
	case attributedArray:
		return fbyGroupLookup(g.array)
	case tiledArray:
		sourceLen := g.source.Len()
		if sourceLen == 0 || g.len == 0 {
			return func(row int) (int, error) {
				if row < 0 || row >= g.len {
					return 0, fmt.Errorf("fby group row %d out of range", row)
				}
				return 0, nil
			}, 0, true, nil
		}
		sourceGroups, groupCount, err := fbyGroupIDs(g.source)
		if err != nil {
			return nil, 0, true, err
		}
		return func(row int) (int, error) {
			if row < 0 || row >= g.len {
				return 0, fmt.Errorf("fby group row %d out of range", row)
			}
			return sourceGroups[(g.start+row)%sourceLen], nil
		}, groupCount, true, nil
	default:
		rowGroups, groupCount, err := fbyGroupIDs(groups)
		if err != nil {
			return nil, 0, true, err
		}
		return func(row int) (int, error) {
			if row < 0 || row >= len(rowGroups) {
				return 0, fmt.Errorf("fby group row %d out of range", row)
			}
			return rowGroups[row], nil
		}, groupCount, true, nil
	}
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
	case i64BucketArray:
		if rowGroups, groupCount, ok := fbyGroupIDsBulkI64(groups); ok {
			return rowGroups, groupCount, nil
		}
		return fbyGroupIDsI64Computed(g)
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
	if isDenseIntegerArray(groups) {
		return fbyGroupIDsIntegerArray(groups)
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

// fbyGroupIDsBulkI64 computes group IDs for dense integer group columns by
// flattening the carrier once and running the comparable (linear-probe /
// hash) grouper over the flat slice, instead of one interface dispatch and
// map probe per row over lazy carriers such as xbar bucket chains.
func fbyGroupIDsBulkI64(groups Array) ([]int, int, bool) {
	values, owned, ok := tryBulkI64Values(groups)
	if !ok || len(values) < groups.Len() {
		bulkI64Release(values, owned)
		return nil, 0, false
	}
	rowGroups, groupCount, err := fbyGroupIDsComparable(values[:groups.Len()])
	bulkI64Release(values, owned)
	if err != nil {
		return nil, 0, false
	}
	return rowGroups, groupCount, true
}

func fbyGroupIDsIntegerArray(groups Array) ([]int, int, error) {
	if rowGroups, groupCount, ok := fbyGroupIDsBulkI64(groups); ok {
		return rowGroups, groupCount, nil
	}
	rowGroups := make([]int, groups.Len())
	groupIDs := make(map[int64]int)
	for row := 0; row < groups.Len(); row++ {
		value, ok, err := integerArrayAt(groups, row)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			return nil, 0, fmt.Errorf("fby integer group row %d is null", row)
		}
		id, ok := groupIDs[value]
		if !ok {
			id = len(groupIDs)
			groupIDs[value] = id
		}
		rowGroups[row] = id
	}
	return rowGroups, len(groupIDs), nil
}

func fbyGroupIDsI64Computed(values interface {
	Len() int
	i64At(int) (int64, bool, error)
}) ([]int, int, error) {
	rowGroups := make([]int, values.Len())
	groupIDs := make(map[int64]int)
	for row := 0; row < values.Len(); row++ {
		value, ok, err := values.i64At(row)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			return nil, 0, fmt.Errorf("fby group row %d out of range", row)
		}
		id, ok := groupIDs[value]
		if !ok {
			id = len(groupIDs)
			groupIDs[value] = id
		}
		rowGroups[row] = id
	}
	return rowGroups, len(groupIDs), nil
}

func fbyGroupIDsComparable[T comparable](values []T) ([]int, int, error) {
	return fbyGroupIDsComparableInto(values, make([]int, len(values)))
}

// fbyGroupIDsComparableInto is fbyGroupIDsComparable writing into a
// caller-provided id vector (len(values) long), so single-call consumers can
// hand in a pooled buffer instead of allocating one per evaluation.
func fbyGroupIDsComparableInto[T comparable](values []T, rowGroups []int) ([]int, int, error) {
	// Small-cardinality fast path: market-data style group columns carry a
	// handful of distinct values, so a linear probe over the seen slice (with
	// a last-value run check) beats hashing every row. Falls over to the map
	// path the moment cardinality outgrows the probe window.
	const linearMaxGroups = 16
	seen := make([]T, 0, linearMaxGroups)
	row := 0
	for ; row < len(values); row++ {
		value := values[row]
		if row > 0 && value == values[row-1] {
			rowGroups[row] = rowGroups[row-1]
			continue
		}
		id := -1
		for g := range seen {
			if seen[g] == value {
				id = g
				break
			}
		}
		if id < 0 {
			if len(seen) == linearMaxGroups {
				break
			}
			id = len(seen)
			seen = append(seen, value)
		}
		rowGroups[row] = id
	}
	if row == len(values) {
		return rowGroups, len(seen), nil
	}
	// Cardinality already outgrew the linear probe window, so size the map
	// for a high-cardinality column up front: repeated power-of-two grows
	// (and their full rehashes) measure ~20% of the whole group-id pass on
	// kilo-group symbol columns.
	hint := (len(values) - row) / 4
	if hint < 64 {
		hint = 64
	} else if hint > 1<<14 {
		hint = 1 << 14
	}
	groupIDs := make(map[T]int, hint)
	for g := range seen {
		groupIDs[seen[g]] = g
	}
	for ; row < len(values); row++ {
		value := values[row]
		// Run check mirrors the linear phase: adjacent repeats skip the hash.
		if value == values[row-1] {
			rowGroups[row] = rowGroups[row-1]
			continue
		}
		id, ok := groupIDs[value]
		if !ok {
			id = len(groupIDs)
			groupIDs[value] = id
		}
		rowGroups[row] = id
	}
	return rowGroups, len(groupIDs), nil
}

// fbySumRowGroupIDs resolves per-row group ids into a pooled vector for the
// dense comparable group carriers the fby total-sum kernels see in practice.
// pooled ids must be released with bulkIntRelease by the caller; ok=false
// defers to the closure-based fbyGroupLookup with no side effects.
func fbySumRowGroupIDs(groups Array) (ids []int, count int, ok bool, err error) {
	if ids, count, ok, err := fbyPooledGroupIDsFromIndex(groups); ok || err != nil {
		return ids, count, ok, err
	}
	if ids, count, ok := cachedDenseGroupIDs(groups); ok {
		return ids, count, true, nil
	}
	switch g := unwrapAttributedArray(groups).(type) {
	case columnArray[Symbol]:
		return fbyPooledGroupIDsTextCached(groups, g.data)
	case columnArray[string]:
		return fbyPooledGroupIDsTextCached(groups, g.data)
	case columnArray[int64]:
		return fbyPooledGroupIDsCached(groups, g.data)
	case columnArray[int32]:
		return fbyPooledGroupIDsCached(groups, g.data)
	case columnArray[int16]:
		return fbyPooledGroupIDsCached(groups, g.data)
	case columnArray[int8]:
		return fbyPooledGroupIDsCached(groups, g.data)
	case columnArray[bool]:
		return fbyPooledGroupIDsCached(groups, g.data)
	default:
		return nil, 0, false, nil
	}
}

func fbyPooledGroupIDsFromIndex(groups Array) ([]int, int, bool, error) {
	if index, ok := arrayIndexForBorrowed(groups, ArrayAttributeUnique); ok {
		return fbyPooledGroupIDsFromArrayIndex(index, groups.Len())
	}
	if index, ok := arrayIndexForBorrowed(groups, ArrayAttributeGrouped); ok {
		return fbyPooledGroupIDsFromArrayIndex(index, groups.Len())
	}
	return nil, 0, false, nil
}

func fbyPooledGroupIDsFromArrayIndex(index ArrayIndex, rows int) ([]int, int, bool, error) {
	rowToGroup, err := rowToGroupFromIndex(index)
	if err != nil {
		return nil, 0, true, err
	}
	if len(rowToGroup) != rows {
		return nil, 0, true, fmt.Errorf("fby grouped index length mismatch: %d != %d", len(rowToGroup), rows)
	}
	ids := bulkIntGetLen(rows)
	copy(ids, rowToGroup)
	return ids, len(index.Rows), true, nil
}

func fbyPooledGroupIDs[T comparable](values []T) ([]int, int, bool, error) {
	ids, count, err := fbyGroupIDsComparableInto(values, bulkIntGetLen(len(values)))
	if err != nil {
		bulkIntRelease(ids)
		return nil, 0, true, err
	}
	return ids, count, true, nil
}

func fbyPooledGroupIDsCached[T comparable](groups Array, values []T) ([]int, int, bool, error) {
	ids, count, ok, err := fbyPooledGroupIDs(values)
	if ok && err == nil {
		storeDenseGroupIDs(groups, ids, count)
	}
	return ids, count, ok, err
}

func fbyPooledGroupIDsText[T ~string](values []T) ([]int, int, bool, error) {
	ids, count, err := fbyGroupIDsTextInto(values, bulkIntGetLen(len(values)))
	if err != nil {
		bulkIntRelease(ids)
		return nil, 0, true, err
	}
	return ids, count, true, nil
}

func fbyPooledGroupIDsTextCached[T ~string](groups Array, values []T) ([]int, int, bool, error) {
	ids, count, ok, err := fbyPooledGroupIDsText(values)
	if ok && err == nil {
		storeDenseGroupIDs(groups, ids, count)
	}
	return ids, count, ok, err
}

// fbyTextGroupIDMapPool recycles the high-cardinality content map for the
// text group-id pass: clear() keeps the grown bucket array, so a warm fby
// over a kilo-group symbol column skips both the per-call map allocation and
// the incremental rehash-to-capacity it would otherwise pay every call.
var fbyTextGroupIDMapPool = sync.Pool{New: func() any { return make(map[string]int, 1<<11) }}

// fbyGroupIDsTextInto is fbyGroupIDsComparableInto for string-like columns
// with a pooled content map. Group ids are identical to the generic path.
func fbyGroupIDsTextInto[T ~string](values []T, rowGroups []int) ([]int, int, error) {
	const linearMaxGroups = 16
	seen := make([]T, 0, linearMaxGroups)
	row := 0
	for ; row < len(values); row++ {
		value := values[row]
		if row > 0 && value == values[row-1] {
			rowGroups[row] = rowGroups[row-1]
			continue
		}
		id := -1
		for g := range seen {
			if seen[g] == value {
				id = g
				break
			}
		}
		if id < 0 {
			if len(seen) == linearMaxGroups {
				break
			}
			id = len(seen)
			seen = append(seen, value)
		}
		rowGroups[row] = id
	}
	if row == len(values) {
		return rowGroups, len(seen), nil
	}
	groupIDs := fbyTextGroupIDMapPool.Get().(map[string]int)
	for g := range seen {
		groupIDs[string(seen[g])] = g
	}
	for ; row < len(values); row++ {
		value := values[row]
		if value == values[row-1] {
			rowGroups[row] = rowGroups[row-1]
			continue
		}
		id, ok := groupIDs[string(value)]
		if !ok {
			id = len(groupIDs)
			groupIDs[string(value)] = id
		}
		rowGroups[row] = id
	}
	count := len(groupIDs)
	clear(groupIDs)
	fbyTextGroupIDMapPool.Put(groupIDs)
	return rowGroups, count, nil
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
		if carrier, ok := asNullBitmapCarrier(array); ok {
			return nullBitmapNumericSumRows(carrier, rows)
		}
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
	leftIndexes := bulkIntGet(len(left.data))
	rightIndexes := bulkIntGet(len(left.data))
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

func (typedKernelRegistry) WindowLastMatchIndexes(left Frame, leftTime Array, leftPartitionColumns []Symbol, rightTime Array, rightByPartition map[string][]int, opts WindowJoinOptions, partitionKinds ...[]Kind) ([]int, error) {
	kinds := optionalPartitionKinds(partitionKinds)
	if leftTimes, ok := int64ColumnData(leftTime); ok {
		if rightTimes, ok := int64ColumnData(rightTime); ok {
			if !opts.HasBounds {
				return asofMatchIndexesI64(left, leftTimes, leftPartitionColumns, rightTimes, rightByPartition, kinds)
			}
			low, okLow := windowDeltaI64(leftTime.Kind(), opts.Low)
			high, okHigh := windowDeltaI64(leftTime.Kind(), opts.High)
			if okLow && okHigh && low <= high {
				return windowLastMatchIndexesI64(left, leftTimes, leftPartitionColumns, rightTimes, rightByPartition, kinds, windowI64Bounds{has: true, low: low, high: high})
			}
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
			if start >= end {
				continue
			}
			rightIndexes[row] = rows[end-1]
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
	if out, ok := gatherOptionalNullBitmap(array, indexes); ok {
		return out
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

// gatherOptionalNullBitmap gathers a dense numeric column with missing (-1)
// indexes into a typed null-bitmap carrier instead of per-cell boxed storage.
func gatherOptionalNullBitmap(array Array, indexes []int) (Array, bool) {
	switch a := unwrapAttributedArray(array).(type) {
	case columnArray[int8]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[int16]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[int32]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[int64]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[float32]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[float64]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[Month]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[Date]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[DateTime]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[Timespan]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[Minute]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[Second]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[Time]:
		return gatherNullBitmapColumn(a, indexes), true
	case columnArray[Timestamp]:
		return gatherNullBitmapColumn(a, indexes), true
	default:
		return nil, false
	}
}

func gatherNullBitmapColumn[T nullBitmapElem](a columnArray[T], indexes []int) Array {
	data := make([]T, len(indexes))
	nulls := newNullBitmap(len(indexes))
	for i, row := range indexes {
		if row < 0 {
			nullBitSet(nulls, i)
			continue
		}
		data[i] = a.data[row]
	}
	return nullBitmapArray[T]{kind: a.kind, data: data, nulls: nulls}
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
	case nullBitmapCarrier:
		return nullableColumnKeyFunc(col, a.Values())
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
	// Window-join list columns stay lazy: most consumers immediately
	// aggregate each window (sum/avg/count), which the windowListArray fast
	// path serves straight from the typed source without boxing any element.
	return windowListArray{source: array, windows: indexes}
}

// windowListArray is a lazy list column whose row i is the source values at
// windows[i]. At/Values reproduce the boxed list shape the eager gather
// produced; aggregation consumers read source+windows directly.
type windowListArray struct {
	source  Array
	windows [][]int
}

func (a windowListArray) Kind() Kind { return KindAny }

func (a windowListArray) Len() int { return len(a.windows) }

func (a windowListArray) At(row int) (any, bool) {
	if row < 0 || row >= len(a.windows) {
		return nil, false
	}
	rows := a.windows[row]
	values := make([]any, len(rows))
	for j, r := range rows {
		v, ok := a.source.At(r)
		if !ok {
			return nil, false
		}
		values[j] = v
	}
	return values, true
}

func (a windowListArray) Values() []any {
	out := make([]any, len(a.windows))
	for i := range a.windows {
		v, ok := a.At(i)
		if !ok {
			panic(fmt.Sprintf("window list row %d out of range", i))
		}
		out[i] = v
	}
	return out
}

func (a windowListArray) Gather(indexes []int) Array {
	windows := make([][]int, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= len(a.windows) {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		windows[i] = a.windows[row]
	}
	return windowListArray{source: a.source, windows: windows}
}

func (typedKernelRegistry) GatherLastOptional(array Array, indexes [][]int) Array {
	last := make([]int, len(indexes))
	for i, rows := range indexes {
		if len(rows) == 0 {
			last[i] = -1
			continue
		}
		last[i] = rows[len(rows)-1]
	}
	return joinGatherOptional(array, last)
}

func (typedKernelRegistry) NumericAt(array Array, row int) (float64, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		return typedKernels.NumericAt(a.array, row)
	case indexedArray:
		index, ok, err := i64IndexArrayAt(a.indexes, row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return typedKernels.NumericAt(a.source, index)
	case shiftedArray:
		sourceRow := row + a.offset
		if row < 0 || row >= a.Len() {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		if sourceRow < 0 || sourceRow >= a.source.Len() {
			return 0, false, nil
		}
		return typedKernels.NumericAt(a.source, sourceRow)
	case i64SparseAmendArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case fbyI64BroadcastArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case fbyI64TiledBroadcastArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case fbyF64BroadcastArray:
		return a.f64At(row)
	case fbyF64TiledBroadcastArray:
		return a.f64At(row)
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
	case f64NumericDyadicArray:
		return a.f64At(row)
	case qRatiosArray:
		return a.f64At(row)
	case f64BucketArray:
		return a.f64At(row)
	case castF32Array:
		return a.f64At(row)
	case castI64Array:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case f64FillArray:
		return a.valueAt(row)
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
	case i64BucketArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case i64XrankArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
	case i64PeriodicIndexArray:
		value, ok := a.i64At(row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		return float64(value), true, nil
	case i64ProductArray:
		return numericI64ProductAt(a, row)
	case i64DyadicProductArray:
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return float64(value), true, nil
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
	case nullBitmapArray[int8]:
		return nullBitmapNumericAt(a, row)
	case nullBitmapArray[int16]:
		return nullBitmapNumericAt(a, row)
	case nullBitmapArray[int32]:
		return nullBitmapNumericAt(a, row)
	case nullBitmapArray[int64]:
		return nullBitmapNumericAt(a, row)
	case nullBitmapArray[float32]:
		return nullBitmapNumericAt(a, row)
	case nullBitmapArray[float64]:
		return nullBitmapNumericAt(a, row)
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
	case indexedArray:
		return isNumericArray(a.source)
	case shiftedArray:
		return isNumericArray(a.source)
	case columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		columnArray[float32], columnArray[float64], i64RangeArray, f64RangeArray,
		i64RunningSumArray, f64RunningSumArray, i64SegmentArray, i64ProductArray, i64DyadicProductArray,
		i64SparseAmendArray, i64ScalarDyadicArray, i64ScalarDyadicRunningSumArray, f64NumericDyadicArray,
		castF32Array, castI64Array,
		qRatiosArray, i64BucketArray, i64XrankArray, i64FillArray, f64BucketArray, f64FillArray,
		fbyI64BroadcastArray, fbyI64TiledBroadcastArray, fbyF64BroadcastArray, fbyF64TiledBroadcastArray:
		return true
	case nullBitmapArray[int8], nullBitmapArray[int16], nullBitmapArray[int32],
		nullBitmapArray[int64], nullBitmapArray[float32], nullBitmapArray[float64]:
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
	if bound, ok, err := BindNumericDyadicFloat(string(op), left, right); ok || err != nil {
		if err != nil {
			return nil, true, err
		}
		return bound.Array(), true, nil
	}
	if out, ok := numericDyadicNullBitmapBulk(op, left, right, length); ok {
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
	if out, ok, err := numericIntegerDyadicArrayScalar(op, left, right, length); ok || err != nil {
		return out, ok, err
	}
	if out, ok := numericIntegerDyadicLazyProduct(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok := numericIntegerDyadicBulk(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok := numericIntegerDyadicNullBitmapBulk(op, left, right, length); ok {
		return out, true, nil
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

func numericIntegerDyadicLazyProduct(op Op, left, right any, length int) (Array, bool) {
	if op != OpMul {
		return nil, false
	}
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	if !leftIsArray || !rightIsArray || leftArray.Len() != length || rightArray.Len() != length {
		return nil, false
	}
	if !isDenseIntegerArray(leftArray) || !isDenseIntegerArray(rightArray) {
		return nil, false
	}
	return i64DyadicProductArray{left: leftArray, right: rightArray, len: length}, true
}

func numericIntegerDyadicArrayScalar(op Op, left, right any, length int) (Array, bool, error) {
	leftArray, leftIsArray := left.(Array)
	rightArray, rightIsArray := right.(Array)
	switch {
	case leftIsArray && !rightIsArray:
		if leftArray.Len() != length || !isDenseIntegerArray(leftArray) {
			return nil, false, nil
		}
		scalar, ok := integerScalarValue(right)
		if !ok {
			return nil, false, nil
		}
		return applyI64ArrayScalar(op, leftArray, scalar, false)
	case rightIsArray && !leftIsArray:
		if rightArray.Len() != length || !isDenseIntegerArray(rightArray) {
			return nil, false, nil
		}
		scalar, ok := integerScalarValue(left)
		if !ok {
			return nil, false, nil
		}
		return applyI64ArrayScalar(op, rightArray, scalar, true)
	default:
		return nil, false, nil
	}
}

func applyI64ArrayScalar(op Op, values Array, scalar int64, scalarLeft bool) (Array, bool, error) {
	if values.Len() == 0 || op == OpMod && (!scalarLeft && scalar == 0 || scalarLeft) {
		return nil, false, nil
	}
	if op == OpIDiv && (scalarLeft || scalar == 0 || scalar == -1) {
		return nil, false, nil
	}
	// Tiled pushdown: an elementwise scalar dyadic commutes with tiling, so
	// wrap the (small) tile source instead of the tiled view. Downstream
	// whole-vector consumers then see a tiledArray and stay on the
	// O(period) cycle kernels instead of flattening N rows.
	if tiled, ok := values.(tiledArray); ok && tiled.source.Len() > 0 {
		if inner, handled, err := applyI64ArrayScalar(op, tiled.source, scalar, scalarLeft); err == nil && handled {
			return tiledArray{source: inner, start: tiled.start, len: tiled.len}, true, nil
		}
	}
	return i64ScalarDyadicArray{source: values, op: op, scalar: scalar, scalarLeft: scalarLeft, len: values.Len()}, true, nil
}

func qModInt64(left, right int64) int64 {
	// Non-negative left with positive right is the hot kernel shape; the
	// hardware remainder is exact there (and exact where the float-floor
	// route below would lose precision past 2^53).
	if left >= 0 && right > 0 {
		return left % right
	}
	return left - right*int64(math.Floor(float64(left)/float64(right)))
}

// IntegerDivModReducerTerm describes a fused integer reducer over one input
// array. OpDiv means q integer div (floor division), not floating division.
type IntegerDivModReducerTerm struct {
	Op      Op
	Divisor int64
}

// TryTypedIntegerDivModSumCount fuses terms like sum(x div n), sum(x mod n),
// and count x over the same integer input without materializing intermediate
// vectors. It is intentionally shape-based: callers provide normalized reducer
// terms, and the data layer chooses formulas for ranges or a single scan for
// other integer arrays.
func TryTypedIntegerDivModSumCount(array Array, terms []IntegerDivModReducerTerm, includeCount bool) (int64, bool, error) {
	if array == nil || len(terms) == 0 && !includeCount {
		return 0, false, nil
	}
	for _, term := range terms {
		if term.Op != OpDiv && term.Op != OpMod {
			return 0, false, nil
		}
		if term.Divisor == 0 {
			return 0, true, fmt.Errorf("divide by zero")
		}
	}
	length := array.Len()
	if rangeValues, ok := asI64RangeArray(array); ok && rangeValues.len == length {
		total, ok := i64RangeIntegerDivModSumCount(rangeValues, terms, includeCount)
		if ok {
			return total, true, nil
		}
	}
	total := int64(0)
	if includeCount {
		total += int64(length)
	}
	for row := 0; row < length; row++ {
		value, ok, err := integerArrayAt(array, row)
		if err != nil {
			return 0, true, err
		}
		if !ok {
			continue
		}
		for _, term := range terms {
			switch term.Op {
			case OpDiv:
				total += floorDivInt64(value, term.Divisor)
			case OpMod:
				total += qModInt64(value, term.Divisor)
			}
		}
	}
	return total, true, nil
}

func i64RangeIntegerDivModSumCount(values i64RangeArray, terms []IntegerDivModReducerTerm, includeCount bool) (int64, bool) {
	total := int64(0)
	if includeCount {
		total += int64(values.len)
	}
	for _, term := range terms {
		switch term.Op {
		case OpDiv:
			if term.Divisor <= 0 {
				return 0, false
			}
			sum, ok := floorSumArithmetic(values.len, values.start, values.step, term.Divisor)
			if !ok {
				return 0, false
			}
			total += sum
		case OpMod:
			if term.Divisor <= 0 {
				return 0, false
			}
			if values.step == 1 {
				total += i64RangePositiveModSum(values.start, values.len, term.Divisor)
				continue
			}
			sumFloor, ok := floorSumArithmetic(values.len, values.start, values.step, term.Divisor)
			if !ok {
				return 0, false
			}
			total += i64RangeSum(values) - term.Divisor*sumFloor
		default:
			return 0, false
		}
	}
	return total, true
}

func applyI64ScalarDyadicValue(op Op, value, scalar int64, scalarLeft bool) (int64, bool, error) {
	const minInt64 = -1 << 63
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
	case OpIDiv:
		divisor := scalar
		dividend := value
		if scalarLeft {
			divisor = value
			dividend = scalar
		}
		if divisor == 0 || (dividend == minInt64 && divisor == -1) {
			return 0, false, nil
		}
		return floorDivInt64(dividend, divisor), true, nil
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

func asI64SegmentArray(value any) (i64SegmentArray, bool) {
	switch a := value.(type) {
	case attributedArray:
		return asI64SegmentArray(a.array)
	case i64SegmentArray:
		return a, true
	default:
		return i64SegmentArray{}, false
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
	case indexedArray:
		return isIntegerArray(a.source)
	case shiftedArray:
		return isIntegerArray(a.source)
	case i64SparseAmendArray:
		return true
	case i64ScalarDyadicArray:
		return true
	case i64ScalarDyadicRunningSumArray:
		return true
	case castI64Array:
		return true
	case columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		i64RangeArray, i64RunningSumArray, i64SegmentArray, i64Int32IndexArray, i64PeriodicIndexArray, i64ProductArray, i64DyadicProductArray, i64BucketArray, i64XrankArray, i64FillArray,
		fbyI64BroadcastArray, fbyI64TiledBroadcastArray:
		return true
	case matrixRowArray:
		return isIntegerArray(a.matrix.data)
	case transposedMatrixRowArray:
		shape := a.matrix.source.Shape()
		if len(shape) != 2 || shape[0] == 0 {
			return false
		}
		row, ok := a.matrix.source.RowArray(0)
		return ok && isIntegerArray(row)
	case nullBitmapArray[int8], nullBitmapArray[int16], nullBitmapArray[int32], nullBitmapArray[int64]:
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
	case indexedArray:
		return isDenseIntegerArray(a.source)
	case i64SparseAmendArray:
		return true
	case i64ScalarDyadicArray:
		return true
	case i64ScalarDyadicRunningSumArray:
		return true
	case castI64Array:
		return true
	case columnArray[int8], columnArray[int16], columnArray[int32], columnArray[int64],
		columnArray[uint8], columnArray[uint16], columnArray[uint32], columnArray[uint64],
		i64RangeArray, i64RunningSumArray, i64SegmentArray, i64Int32IndexArray, i64PeriodicIndexArray, i64ProductArray, i64DyadicProductArray, i64BucketArray, i64XrankArray, i64FillArray,
		fbyI64BroadcastArray, fbyI64TiledBroadcastArray:
		return true
	case matrixRowArray:
		return isDenseIntegerArray(a.matrix.data)
	case transposedMatrixRowArray:
		shape := a.matrix.source.Shape()
		if len(shape) != 2 || shape[0] == 0 {
			return false
		}
		row, ok := a.matrix.source.RowArray(0)
		return ok && isDenseIntegerArray(row)
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
	case indexedArray:
		index, ok, err := i64IndexArrayAt(a.indexes, row)
		if err != nil || !ok {
			return 0, ok, err
		}
		return integerArrayAt(a.source, index)
	case shiftedArray:
		sourceRow := row + a.offset
		if row < 0 || row >= a.Len() {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		if sourceRow < 0 || sourceRow >= a.source.Len() {
			return 0, false, nil
		}
		return integerArrayAt(a.source, sourceRow)
	case i64SparseAmendArray:
		return a.i64At(row)
	case castI64Array:
		return a.i64At(row)
	case fbyI64BroadcastArray:
		return a.i64At(row)
	case fbyI64TiledBroadcastArray:
		return a.i64At(row)
	case i64BucketArray:
		return a.i64At(row)
	case i64XrankArray:
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
	case nullBitmapArray[int8]:
		return nullBitmapIntegerAt(a, row)
	case nullBitmapArray[int16]:
		return nullBitmapIntegerAt(a, row)
	case nullBitmapArray[int32]:
		return nullBitmapIntegerAt(a, row)
	case nullBitmapArray[int64]:
		return nullBitmapIntegerAt(a, row)
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
	case i64DyadicProductArray:
		return a.i64At(row)
	case matrixRowArray:
		if row < 0 || row >= a.Len() {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		value, ok := a.matrix.Cell(a.row, row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		n, ok := integerScalarValue(value)
		if !ok {
			return 0, false, fmt.Errorf("typed integer operand row %d is %T, want integer", row, value)
		}
		return n, true, nil
	case transposedMatrixRowArray:
		if row < 0 || row >= a.Len() {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		value, ok := a.matrix.Cell(a.row, row)
		if !ok {
			return 0, false, fmt.Errorf("array row %d out of range", row)
		}
		n, ok := integerScalarValue(value)
		if !ok {
			return 0, false, fmt.Errorf("typed integer operand row %d is %T, want integer", row, value)
		}
		return n, true, nil
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
	if out, ok := compareI64SegmentScalarDyadic(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok := compareI64ScalarDyadicScalarDyadic(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok := compareI64ArrayScalarDyadic(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok := lazyF64CompareMask(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok := compareDyadicBulk(op, left, right, length); ok {
		return out, true, nil
	}
	if out, ok := compareNullBitmapDyadicMask(op, left, right, length); ok {
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

type i64ArrayCompareMask struct {
	source     Array
	op         Op
	scalar     int64
	scalarLeft bool
	len        int
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

func (a i64ArrayCompareMask) Kind() Kind { return KindBool }

func (a i64ArrayCompareMask) Len() int { return a.len }

func (a i64ArrayCompareMask) At(row int) (any, bool) {
	if row < 0 || row >= a.len {
		return nil, false
	}
	value, ok, err := a.valueAt(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64ArrayCompareMask) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data i64 compare row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64ArrayCompareMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.len {
			panic(fmt.Sprintf("data i64 compare gather index %d out of range", row))
		}
		value, ok, err := a.valueAt(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data i64 compare row %d out of range", row))
		}
		out[i] = value
	}
	return newBoolTrusted(out)
}

func (a i64ArrayCompareMask) valueAt(row int) (bool, bool, error) {
	value, ok, err := integerArrayAt(a.source, row)
	if err != nil || !ok {
		return false, ok, err
	}
	if a.scalarLeft {
		return boolCompare(a.op, a.scalar == value, compareInt64(a.scalar, value)), true, nil
	}
	return boolCompare(a.op, value == a.scalar, compareInt64(value, a.scalar)), true, nil
}

func (a i64ArrayCompareMask) trueCount() (int64, bool, error) {
	values, owned, ok := tryBulkI64Values(a.source)
	if !ok || len(values) < a.len {
		bulkI64Release(values, owned)
		return 0, false, nil
	}
	values = values[:a.len]
	op := effectiveRangeCompareOp(a.op, a.scalarLeft)
	var count int64
	for _, value := range values {
		if boolCompare(op, value == a.scalar, compareInt64(value, a.scalar)) {
			count++
		}
	}
	bulkI64Release(values, owned)
	return count, true, nil
}

func i64ScalarDyadicCompareMaskTrueCount(mask i64ScalarDyadicCompareMask) (int64, bool) {
	if indexes, ok := i64ScalarDyadicCompareSegmentModuloIndexes(mask); ok {
		return int64(indexes.Len()), true
	}
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
	indexes, ok := compareI64RangeIndexArray(a.values, effectiveRangeCompareOp(a.op, a.scalarLeft), a.scalar)
	if ok {
		return int64(indexes.Len())
	}
	var count int64
	for row := 0; row < a.values.len; row++ {
		if a.valueAt(row) {
			count++
		}
	}
	return count
}

type i64SegmentCompareMask struct {
	values     i64SegmentArray
	op         Op
	scalar     int64
	scalarLeft bool
}

func (a i64SegmentCompareMask) Kind() Kind { return KindBool }

func (a i64SegmentCompareMask) Len() int { return a.values.len }

func (a i64SegmentCompareMask) At(row int) (any, bool) {
	if row < 0 || row >= a.values.len {
		return nil, false
	}
	return a.valueAt(row), true
}

func (a i64SegmentCompareMask) Values() []any {
	out := make([]any, a.values.len)
	for row := range out {
		out[row] = a.valueAt(row)
	}
	return out
}

func (a i64SegmentCompareMask) Gather(indexes []int) Array {
	out := make([]bool, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= a.values.len {
			panic(fmt.Sprintf("data segment compare gather index %d out of range", row))
		}
		out[i] = a.valueAt(row)
	}
	return newBoolTrusted(out)
}

func (a i64SegmentCompareMask) valueAt(row int) bool {
	value, ok := a.values.i64At(row)
	if !ok {
		return false
	}
	if a.scalarLeft {
		return boolCompare(a.op, a.scalar == value, compareInt64(a.scalar, value))
	}
	return boolCompare(a.op, value == a.scalar, compareInt64(value, a.scalar))
}

func (a i64SegmentCompareMask) trueCount() int64 {
	selected, _, handled, _ := compareI64SegmentIndexStats(a.values, effectiveRangeCompareOp(a.op, a.scalarLeft), a.scalar)
	if handled {
		return selected
	}
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

func compareI64ArrayScalarDyadic(op Op, left, right any, length int) (Array, bool) {
	leftArray, leftOK := left.(Array)
	rightArray, rightOK := right.(Array)
	leftScalar, leftScalarOK := integerScalarValue(left)
	rightScalar, rightScalarOK := integerScalarValue(right)
	switch {
	case leftOK && !rightOK && rightScalarOK:
		if leftArray.Len() != length || !i64ArrayCompareMaskSource(leftArray, length) {
			return nil, false
		}
		return i64ArrayCompareMask{source: leftArray, op: op, scalar: rightScalar, len: length}, true
	case !leftOK && rightOK && leftScalarOK:
		if rightArray.Len() != length || !i64ArrayCompareMaskSource(rightArray, length) {
			return nil, false
		}
		return i64ArrayCompareMask{source: rightArray, op: op, scalar: leftScalar, scalarLeft: true, len: length}, true
	default:
		return nil, false
	}
}

func i64ArrayCompareMaskSource(array Array, length int) bool {
	if array == nil || array.Len() != length {
		return false
	}
	values, owned, ok := tryBulkI64Values(array)
	bulkI64Release(values, owned)
	return ok && len(values) >= length
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

func compareI64SegmentScalarDyadic(op Op, left, right any, length int) (Array, bool) {
	leftSegment, leftSegmentOK := asI64SegmentArray(left)
	rightSegment, rightSegmentOK := asI64SegmentArray(right)
	leftScalar, leftScalarOK := integerScalarValue(left)
	rightScalar, rightScalarOK := integerScalarValue(right)
	switch {
	case leftSegmentOK && rightScalarOK:
		if leftSegment.len != length {
			return nil, false
		}
		return i64SegmentCompareMask{values: leftSegment, op: op, scalar: rightScalar}, true
	case leftScalarOK && rightSegmentOK:
		if rightSegment.len != length {
			return nil, false
		}
		return i64SegmentCompareMask{values: rightSegment, op: op, scalar: leftScalar, scalarLeft: true}, true
	default:
		return nil, false
	}
}

func effectiveRangeCompareOp(op Op, scalarLeft bool) Op {
	if !scalarLeft {
		return op
	}
	switch op {
	case OpLT:
		return OpGT
	case OpLE:
		return OpGE
	case OpGT:
		return OpLT
	case OpGE:
		return OpLE
	default:
		return op
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
	case NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
		out := make([]float64, array.Len())
		for i := range out {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			result, err := applyNumericUnaryFloat(op, float64(value))
			if err != nil {
				return nil, true, err
			}
			out[i] = result
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
	case NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
		out := make([]float64, len(values))
		for i, value := range values {
			result, err := applyNumericUnaryFloat(op, float64(value))
			if err != nil {
				return nil, true, err
			}
			out[i] = result
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

func qNumericUnaryFloatArray(op string, array Array) (Array, bool, error) {
	switch op {
	case NumericUnaryNeg, NumericUnaryAbs, NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
		out := make([]float64, array.Len())
		for i := range out {
			value, ok, err := numericAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			result, err := applyNumericUnaryFloat(op, value)
			if err != nil {
				return nil, true, err
			}
			out[i] = result
		}
		return newF64Trusted(out), true, nil
	case NumericUnarySignum, NumericUnaryFloor, NumericUnaryCeiling:
		out := make([]int64, array.Len())
		for i := range out {
			value, ok, err := numericAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			switch op {
			case NumericUnarySignum:
				switch {
				case value < 0:
					out[i] = -1
				case value > 0:
					out[i] = 1
				}
			case NumericUnaryFloor:
				out[i] = int64(math.Floor(value))
			case NumericUnaryCeiling:
				out[i] = int64(math.Ceil(value))
			}
		}
		return newI64Trusted(out), true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnaryReturnsFloat(op string) bool {
	switch op {
	case NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
		return true
	default:
		return false
	}
}

// qNumericUnarySumIntegerSlice mirrors qNumericUnarySumIntegerArray over a
// dense int64 slice so bulk-flattened lazy carriers skip per-row dispatch.
func qNumericUnarySumIntegerSlice(op string, values []int64) (any, bool, error) {
	const minInt64 = -1 << 63
	switch op {
	case NumericUnaryNeg:
		var sum int64
		for _, value := range values {
			if value == minInt64 {
				return nil, false, nil
			}
			sum -= value
		}
		return sum, true, nil
	case NumericUnaryAbs:
		var sum int64
		for _, value := range values {
			if value == minInt64 {
				return nil, false, nil
			}
			if value < 0 {
				value = -value
			}
			sum += value
		}
		return sum, true, nil
	case NumericUnarySignum:
		var sum int64
		for _, value := range values {
			switch {
			case value < 0:
				sum--
			case value > 0:
				sum++
			}
		}
		return sum, true, nil
	case NumericUnarySqrt:
		var sum float64
		for _, value := range values {
			sum += math.Sqrt(float64(value))
		}
		return sum, true, nil
	case NumericUnaryLog:
		var sum float64
		for _, value := range values {
			sum += math.Log(float64(value))
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
	case NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan:
		var sum float64
		for _, value := range values {
			out, err := applyNumericUnaryFloat(op, float64(value))
			if err != nil {
				return nil, true, err
			}
			sum += out
		}
		return sum, true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnarySumIntegerArray(op string, array Array) (any, bool, error) {
	switch op {
	case NumericUnaryNeg, NumericUnaryAbs, NumericUnarySignum,
		NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan,
		NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
		if values, owned, ok := tryBulkI64Values(array); ok {
			out, handled, err := qNumericUnarySumI64Values(op, values)
			bulkI64Release(values, owned)
			return out, handled, err
		}
	}
	return qNumericUnarySumIntegerArraySlow(op, array)
}

// qNumericUnarySumI64Values reduces a unary primitive over bulk-materialized
// integer values in one tight loop, mirroring the per-row loop semantics
// (minInt64 bails out to the generic fallback, float primitives accumulate
// through applyNumericUnaryFloat).
func qNumericUnarySumI64Values(op string, values []int64) (any, bool, error) {
	const minInt64 = -1 << 63
	switch op {
	case NumericUnaryNeg:
		var sum int64
		for _, value := range values {
			if value == minInt64 {
				return nil, false, nil
			}
			sum -= value
		}
		return sum, true, nil
	case NumericUnaryAbs:
		var sum int64
		for _, value := range values {
			if value == minInt64 {
				return nil, false, nil
			}
			if value < 0 {
				value = -value
			}
			sum += value
		}
		return sum, true, nil
	case NumericUnarySignum:
		var sum int64
		for _, value := range values {
			switch {
			case value < 0:
				sum--
			case value > 0:
				sum++
			}
		}
		return sum, true, nil
	case NumericUnarySqrt:
		var sum float64
		for _, value := range values {
			sum += math.Sqrt(float64(value))
		}
		return sum, true, nil
	case NumericUnaryLog:
		var sum float64
		for _, value := range values {
			sum += math.Log(float64(value))
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
	case NumericUnarySin:
		// Trig ops hoist the per-element applyNumericUnaryFloat dispatch out
		// of the loop: identical math.* call per element, identical
		// accumulation order, and these ops never error.
		var sum float64
		for _, value := range values {
			sum += math.Sin(float64(value))
		}
		return sum, true, nil
	case NumericUnaryCos:
		var sum float64
		for _, value := range values {
			sum += math.Cos(float64(value))
		}
		return sum, true, nil
	case NumericUnaryTan:
		var sum float64
		for _, value := range values {
			sum += math.Tan(float64(value))
		}
		return sum, true, nil
	case NumericUnaryAsin:
		var sum float64
		for _, value := range values {
			sum += math.Asin(float64(value))
		}
		return sum, true, nil
	case NumericUnaryAcos:
		var sum float64
		for _, value := range values {
			sum += math.Acos(float64(value))
		}
		return sum, true, nil
	case NumericUnaryAtan:
		var sum float64
		for _, value := range values {
			sum += math.Atan(float64(value))
		}
		return sum, true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnarySumIntegerArraySlow(op string, array Array) (any, bool, error) {
	const minInt64 = -1 << 63
	switch op {
	case NumericUnaryFloor, NumericUnaryCeiling:
		return TryTypedNumericSum(array)
	}
	if values, owned, ok := tryBulkI64Values(array); ok {
		out, handled, err := qNumericUnarySumIntegerSlice(op, values)
		bulkI64Release(values, owned)
		return out, handled, err
	}
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
	case NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
		var sum float64
		for i := 0; i < array.Len(); i++ {
			value, ok, err := integerArrayAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			out, err := applyNumericUnaryFloat(op, float64(value))
			if err != nil {
				return nil, true, err
			}
			sum += out
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
	case NumericUnarySqrt:
		var sum float64
		for _, value := range values {
			sum += math.Sqrt(float64(value))
		}
		return sum, true, nil
	case NumericUnaryLog:
		var sum float64
		for _, value := range values {
			sum += math.Log(float64(value))
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
	case NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan:
		var sum float64
		for _, value := range values {
			out, err := applyNumericUnaryFloat(op, float64(value))
			if err != nil {
				return nil, true, err
			}
			sum += out
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

func qNumericUnarySumFloatArray(op string, array Array) (any, bool, error) {
	if values, owned, ok := tryBulkF64Values(array); ok {
		out, handled, err := qNumericUnarySumFloatSlice(op, values)
		bulkF64Release(values, owned)
		return out, handled, err
	}
	switch op {
	case NumericUnaryNeg, NumericUnaryAbs, NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
		var sum float64
		for i := 0; i < array.Len(); i++ {
			value, ok, err := numericAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			switch op {
			case NumericUnaryNeg:
				sum -= value
			case NumericUnaryAbs:
				sum += math.Abs(value)
			default:
				out, err := applyNumericUnaryFloat(op, value)
				if err != nil {
					return nil, true, err
				}
				sum += out
			}
		}
		return sum, true, nil
	case NumericUnarySignum, NumericUnaryFloor, NumericUnaryCeiling:
		var sum int64
		for i := 0; i < array.Len(); i++ {
			value, ok, err := numericAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, false, nil
			}
			switch op {
			case NumericUnarySignum:
				switch {
				case value < 0:
					sum--
				case value > 0:
					sum++
				}
			case NumericUnaryFloor:
				sum += int64(math.Floor(value))
			case NumericUnaryCeiling:
				sum += int64(math.Ceil(value))
			}
		}
		return sum, true, nil
	default:
		return nil, false, nil
	}
}

func qNumericUnaryTiledSumReturnsFloat(op string, source Array) bool {
	switch op {
	case NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
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
		case NumericUnarySqrt, NumericUnaryLog, NumericUnaryExp, NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan, NumericUnaryRecip:
			out, err := applyNumericUnaryFloat(op, value)
			if err != nil {
				return 0, true, err
			}
			sum += out
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
	case NumericUnarySqrt:
		var sum float64
		value := values.start
		for i := 0; i < values.len; i++ {
			sum += math.Sqrt(float64(value))
			value += values.step
		}
		return sum, true
	case NumericUnaryLog:
		var sum float64
		value := values.start
		for i := 0; i < values.len; i++ {
			sum += math.Log(float64(value))
			value += values.step
		}
		return sum, true
	case NumericUnaryExp:
		var sum float64
		value := values.start
		for i := 0; i < values.len; i++ {
			sum += math.Exp(float64(value))
			value += values.step
		}
		return sum, true
	case NumericUnaryRecip:
		var sum float64
		value := values.start
		for i := 0; i < values.len; i++ {
			sum += 1 / float64(value)
			value += values.step
		}
		return sum, true
	case NumericUnarySin, NumericUnaryCos, NumericUnaryTan, NumericUnaryAsin, NumericUnaryAcos, NumericUnaryAtan:
		var sum float64
		value := values.start
		for i := 0; i < values.len; i++ {
			out, err := applyNumericUnaryFloat(op, float64(value))
			if err != nil {
				return nil, false
			}
			sum += out
			value += values.step
		}
		return sum, true
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
	case NumericUnarySin:
		return math.Sin(value), nil
	case NumericUnaryCos:
		return math.Cos(value), nil
	case NumericUnaryTan:
		return math.Tan(value), nil
	case NumericUnaryAsin:
		return math.Asin(value), nil
	case NumericUnaryAcos:
		return math.Acos(value), nil
	case NumericUnaryAtan:
		return math.Atan(value), nil
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
	case OpMod:
		if right == 0 {
			return math.NaN(), nil
		}
		return left - right*math.Floor(left/right), nil
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

func numericSumMatrixRowValue(row matrixRowArray) (any, bool, error) {
	if sum, handled, err := numericSumMatrixRowDirect(row.matrix, row.row); handled || err != nil {
		return sum, handled, err
	}
	return numericSumRowArrayValue(row)
}

func numericSumTransposedMatrixRowValue(row transposedMatrixRowArray) (any, bool, error) {
	if sum, handled, err := numericSumTransposedMatrixRowDirect(row.matrix, row.row); handled || err != nil {
		return sum, handled, err
	}
	return numericSumRowArrayValue(row)
}

func numericSumMatrixRowDirect(matrix matrixArray, row int) (any, bool, error) {
	if len(matrix.shape) != 2 || row < 0 || row >= matrix.shape[0] {
		return nil, true, fmt.Errorf("matrix row index %d out of range", row)
	}
	cols := matrix.shape[1]
	start := row * cols
	end := start + cols
	switch source := matrix.data.(type) {
	case attributedArray:
		return numericSumMatrixRowDirect(matrixArray{shape: matrix.shape, data: source.array}, row)
	case columnArray[int8]:
		return numericSumIntegerValue(source.data[start:end]), true, nil
	case columnArray[int16]:
		return numericSumIntegerValue(source.data[start:end]), true, nil
	case columnArray[int32]:
		return numericSumIntegerValue(source.data[start:end]), true, nil
	case columnArray[int64]:
		return numericSumIntegerValue(source.data[start:end]), true, nil
	case i64RangeArray:
		return i64RangeSum(i64RangeArray{start: source.start + int64(start)*source.step, step: source.step, len: cols}), true, nil
	case columnArray[uint8]:
		return numericSumUnsignedValue(source.data[start:end]), true, nil
	case columnArray[uint16]:
		return numericSumUnsignedValue(source.data[start:end]), true, nil
	case columnArray[uint32]:
		return numericSumUnsignedValue(source.data[start:end]), true, nil
	case columnArray[uint64]:
		return numericSumUnsignedValue(source.data[start:end]), true, nil
	case columnArray[float32]:
		return numericSumFloatValue(source.data[start:end]), true, nil
	case columnArray[float64]:
		return numericSumFloatValue(source.data[start:end]), true, nil
	default:
		return nil, false, nil
	}
}

func numericSumRowArrayValue(array Array) (any, bool, error) {
	if array == nil {
		return nil, false, nil
	}
	if isIntegerArray(array) {
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
		}
		return sum, true, nil
	}
	var sum float64
	for row := 0; row < array.Len(); row++ {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			continue
		}
		sum += value
	}
	return sum, true, nil
}

func numericSumCrossPairValue(pair crossPairArray) (any, bool, error) {
	leftInt, leftIsInt := coerceInt64Exact(pair.left)
	rightInt, rightIsInt := coerceInt64Exact(pair.right)
	if leftIsInt && rightIsInt {
		return leftInt + rightInt, true, nil
	}
	left, ok := numeric(pair.left)
	if !ok {
		return nil, false, nil
	}
	right, ok := numeric(pair.right)
	if !ok {
		return nil, false, nil
	}
	return left + right, true, nil
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

func i64BucketSum(values i64BucketArray) (int64, bool, error) {
	if values.width <= 0 {
		return 0, true, fmt.Errorf("bucket floor interval must be positive")
	}
	switch source := values.source.(type) {
	case attributedArray:
		return i64BucketSum(i64BucketArray{source: source.array, width: values.width, len: values.len})
	case i64RangeArray:
		if source.step == 1 && source.start >= 0 && values.len == source.len {
			return i64BucketRangeStepOneSum(source.start, values.len, values.width), true, nil
		}
	}
	// Bulk-flatten the source once and bucket-reduce in a dense loop instead
	// of re-walking the carrier tree per row. Null sources bail out of the
	// flatteners, so the per-row fallback keeps its null semantics.
	if source, owned, ok := tryBulkI64Values(values.source); ok {
		if len(source) >= values.len {
			var total int64
			for _, v := range source[:values.len] {
				total += floorInt64(v, values.width)
			}
			bulkI64Release(source, owned)
			return total, true, nil
		}
		bulkI64Release(source, owned)
	}
	var total int64
	for row := 0; row < values.len; row++ {
		value, ok, err := values.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		total += value
	}
	return total, true, nil
}

func i64BucketRangeStepOneSum(start int64, length int, width int64) int64 {
	if length <= 0 {
		return 0
	}
	end := start + int64(length)
	return width * (floorQuotientPrefixSum(end, width) - floorQuotientPrefixSum(start, width))
}

func floorQuotientPrefixSum(n, width int64) int64 {
	if n <= 0 {
		return 0
	}
	full := n / width
	rem := n % width
	return width*full*(full-1)/2 + rem*full
}

func f64BucketSum(values f64BucketArray) (float64, bool, error) {
	if values.width <= 0 || math.IsNaN(values.width) || math.IsInf(values.width, 0) {
		return 0, true, fmt.Errorf("bucket floor interval must be finite and positive")
	}
	// Bulk-flatten the source once and bucket-reduce in source order, exactly
	// matching the per-row fold below. Null sources bail out of the flattener.
	if source, owned, ok := tryBulkF64Values(values.source); ok {
		if len(source) >= values.len {
			var total float64
			for _, v := range source[:values.len] {
				total += math.Floor(v/values.width) * values.width
			}
			bulkF64Release(source, owned)
			return total, true, nil
		}
		bulkF64Release(source, owned)
	}
	var total float64
	for row := 0; row < values.len; row++ {
		value, ok, err := values.f64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		total += value
	}
	return total, true, nil
}

func i64XrankSum(values i64XrankArray) (int64, bool, error) {
	if values.bucketCount <= 0 || values.domainSize <= 0 {
		return 0, true, fmt.Errorf("xrank domain is invalid")
	}
	switch source := values.source.(type) {
	case attributedArray:
		return i64XrankSum(i64XrankArray{source: source.array, bucketCount: values.bucketCount, domainSize: values.domainSize, len: values.len})
	case i64RangeArray:
		if source.start == 0 && source.step == 1 && int64(source.len) == values.domainSize && values.len == source.len {
			return i64XrankDomainPrefixSum(values.domainSize, values.bucketCount, int64(values.len)), true, nil
		}
	case i64ScalarDyadicArray:
		if source.op == OpMod && !source.scalarLeft && source.scalar == values.domainSize && source.scalar > 0 {
			period := i64XrankDomainPrefixSum(values.domainSize, values.bucketCount, values.domainSize)
			full := int64(values.len) / values.domainSize
			rem := int64(values.len) % values.domainSize
			return period*full + i64XrankDomainPrefixSum(values.domainSize, values.bucketCount, rem), true, nil
		}
	}
	var total int64
	for row := 0; row < values.len; row++ {
		value, ok, err := values.i64At(row)
		if err != nil || !ok {
			return 0, ok, err
		}
		total += value
	}
	return total, true, nil
}

func i64XrankDomainPrefixSum(domainSize, bucketCount, length int64) int64 {
	if domainSize <= 0 || bucketCount <= 0 || length <= 0 {
		return 0
	}
	if length > domainSize {
		length = domainSize
	}
	var total int64
	for value := int64(0); value < length; value++ {
		bucket := (value * bucketCount) / domainSize
		if bucket >= bucketCount {
			bucket = bucketCount - 1
		}
		total += bucket
	}
	return total
}

func gatherI64BucketRange(values i64BucketArray, indexes []int) (Array, bool) {
	if len(indexes) == 0 {
		return i64BucketArray{source: values.source.Gather(nil), width: values.width, len: 0}, true
	}
	gathered := values.source.Gather(indexes)
	return i64BucketArray{source: gathered, width: values.width, len: gathered.Len()}, true
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

func i64DyadicProductSum(values i64DyadicProductArray) (int64, bool, error) {
	left, leftOwned, ok := tryBulkI64Values(values.left)
	if !ok || len(left) < values.len {
		bulkI64Release(left, leftOwned)
		return 0, false, nil
	}
	right, rightOwned, ok := tryBulkI64Values(values.right)
	if !ok || len(right) < values.len {
		bulkI64Release(left, leftOwned)
		bulkI64Release(right, rightOwned)
		return 0, false, nil
	}
	var total int64
	for i := 0; i < values.len; i++ {
		total += left[i] * right[i]
	}
	bulkI64Release(left, leftOwned)
	bulkI64Release(right, rightOwned)
	return total, true, nil
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

func deltasSignedTypedSlice[T signedScalar](kind Kind, values []T) Array {
	out := make([]T, len(values))
	for i, value := range values {
		if i == 0 {
			out[i] = value
			continue
		}
		out[i] = T(int64(value) - int64(values[i-1]))
	}
	return columnArray[T]{kind: kind, data: out}
}

func deltasUnsignedSlice[T unsignedScalar](values []T) Array {
	out := make([]int64, len(values))
	for i, value := range values {
		if i == 0 {
			out[i] = int64(value)
			continue
		}
		out[i] = int64(value) - int64(values[i-1])
	}
	return newI64Trusted(out)
}

func deltasFloatSlice[T floatScalar](values []T) Array {
	out := make([]float64, len(values))
	for i, value := range values {
		if i == 0 {
			out[i] = float64(value)
			continue
		}
		out[i] = float64(value) - float64(values[i-1])
	}
	return newF64Trusted(out)
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

func deltasNullableArray(values nullableArray) (Array, bool, error) {
	out := make([]any, len(values.data))
	hasFloat := false
	for row, current := range values.data {
		if row == 0 {
			if IsNull(current) {
				out[row] = NullValue
				continue
			}
			value := current
			if values.kind != "" && values.kind != KindAny && values.kind != KindNull {
				normalized, err := normalizeScalarForKind(values.kind, current, row)
				if err != nil {
					return nil, false, nil
				}
				value = normalized
			}
			hasFloat = !integerLikeKind(values.kind)
			out[row] = value
			continue
		}
		previous := values.data[row-1]
		if IsNull(current) || IsNull(previous) {
			out[row] = NullValue
			continue
		}
		value, ok := normalizeNullableDeltaValue(values.kind, current, previous)
		if !ok {
			return nil, false, nil
		}
		if _, ok := numeric(value); ok && !integerLikeKind(values.kind) {
			hasFloat = true
		}
		out[row] = value
	}
	if hasFloat {
		return nullableArray{kind: KindF64, data: out}, true, nil
	}
	if values.kind != "" && values.kind != KindAny && values.kind != KindNull {
		return nullableArray{kind: values.kind, data: out}, true, nil
	}
	return nullableArray{kind: KindI64, data: out}, true, nil
}

func normalizeNullableDeltaValue(kind Kind, current, previous any) (any, bool) {
	switch kind {
	case KindI8:
		currentI, ok := coerceInt64Exact(current)
		if !ok {
			return nil, false
		}
		previousI, ok := coerceInt64Exact(previous)
		if !ok {
			return nil, false
		}
		return int8(currentI - previousI), true
	case KindI16:
		currentI, ok := coerceInt64Exact(current)
		if !ok {
			return nil, false
		}
		previousI, ok := coerceInt64Exact(previous)
		if !ok {
			return nil, false
		}
		return int16(currentI - previousI), true
	case KindI32:
		currentI, ok := coerceInt64Exact(current)
		if !ok {
			return nil, false
		}
		previousI, ok := coerceInt64Exact(previous)
		if !ok {
			return nil, false
		}
		return int32(currentI - previousI), true
	case KindI64:
		currentI, ok := coerceInt64Exact(current)
		if !ok {
			return nil, false
		}
		previousI, ok := coerceInt64Exact(previous)
		if !ok {
			return nil, false
		}
		return currentI - previousI, true
	case KindF32:
		currentF, ok := numeric(current)
		if !ok {
			return nil, false
		}
		previousF, ok := numeric(previous)
		if !ok {
			return nil, false
		}
		return float32(currentF - previousF), true
	case KindF64:
		currentF, ok := numeric(current)
		if !ok {
			return nil, false
		}
		previousF, ok := numeric(previous)
		if !ok {
			return nil, false
		}
		return currentF - previousF, true
	default:
		currentI, currentIOK := coerceInt64Exact(current)
		previousI, previousIOK := coerceInt64Exact(previous)
		if currentIOK && previousIOK {
			return currentI - previousI, true
		}
		currentF, currentFOK := numeric(current)
		previousF, previousFOK := numeric(previous)
		if currentFOK && previousFOK {
			return currentF - previousF, true
		}
		return nil, false
	}
}

func integerLikeKind(kind Kind) bool {
	switch kind {
	case KindI8, KindI16, KindI32, KindI64, KindU8, KindU16, KindU32, KindU64:
		return true
	default:
		return false
	}
}

func deltasNullableSum(values nullableArray) (any, bool, error) {
	var totalI int64
	var totalF float64
	hasFloat := false
	for row, current := range values.data {
		if IsNull(current) {
			continue
		}
		if row == 0 {
			if n, ok := coerceInt64Exact(current); ok && !hasFloat {
				totalI += n
				totalF += float64(n)
				continue
			}
			n, ok := numeric(current)
			if !ok {
				return nil, false, nil
			}
			hasFloat = true
			totalF += n
			continue
		}
		previous := values.data[row-1]
		if IsNull(previous) {
			continue
		}
		if !hasFloat {
			currentI, currentOK := coerceInt64Exact(current)
			previousI, previousOK := coerceInt64Exact(previous)
			if currentOK && previousOK {
				delta := currentI - previousI
				totalI += delta
				totalF += float64(delta)
				continue
			}
		}
		currentF, currentOK := numeric(current)
		previousF, previousOK := numeric(previous)
		if !currentOK || !previousOK {
			return nil, false, nil
		}
		hasFloat = true
		totalF += currentF - previousF
	}
	if hasFloat {
		return totalF, true, nil
	}
	return totalI, true, nil
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
		if carrier, ok := asNullBitmapCarrier(array); ok {
			var best any
			hasBest := false
			nulls := carrier.nullBits()
			for i := 0; i < carrier.Len(); i++ {
				if nullBitGet(nulls, i) {
					continue
				}
				v, _ := carrier.At(i)
				if !hasBest || (mode == "min" && compare(v, best) < 0) || (mode == "max" && compare(v, best) > 0) {
					best = v
					hasBest = true
				}
			}
			return best, hasBest, true, nil
		}
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

func compareI64SegmentMask(values i64SegmentArray, target int64, ok bool, op Op, out []bool) bool {
	if !ok || len(out) < values.len {
		return false
	}
	row := 0
	for _, segment := range values.segments {
		for i := 0; i < segment.len; i++ {
			v := segment.start + int64(i)*segment.step
			out[row] = boolCompare(op, v == target, compareInt64(v, target))
			row++
		}
	}
	return true
}

func compareBoolIndexes(values []bool, target bool, ok bool, op Op, out []int) ([]int, bool) {
	if !ok {
		return nil, false
	}
	// Bool predicates have only two cell outcomes. Index vectors come from
	// the bulk pool sized to the row count, so the old exact-sizing count
	// pass would only add a second scan.
	holdsTrue := boolCompare(op, target, compareBool(true, target))
	holdsFalse := boolCompare(op, !target, compareBool(false, target))
	if !holdsTrue && !holdsFalse {
		// Callers distinguish nil (unhandled / all rows) from empty.
		if out == nil {
			return []int{}, true
		}
		return out[:0], true
	}
	if cap(out) < len(values) {
		out = bulkIntGet(len(values))
	} else {
		out = out[:0]
	}
	if holdsTrue && holdsFalse {
		for i := range values {
			out = append(out, i)
		}
		return out, true
	}
	for i, v := range values {
		if v == holdsTrue {
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

type f64NumericDyadicArray struct {
	op    string
	left  any
	right any
	len   int
	bound NumericDyadicFloatBound
}

type i64ScalarDyadicRunningSumArray struct {
	source i64ScalarDyadicArray
}

func (a f64NumericDyadicArray) Kind() Kind { return KindF64 }

func (a f64NumericDyadicArray) Len() int { return a.len }

func (a f64NumericDyadicArray) At(row int) (any, bool) {
	value, ok, err := a.f64At(row)
	if err != nil {
		return nil, false
	}
	if !ok {
		return NullValue, true
	}
	return value, true
}

func (a f64NumericDyadicArray) Values() []any {
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.f64At(row)
		if err != nil {
			panic(fmt.Sprintf("data f64 dyadic row %d out of range", row))
		}
		if !ok {
			out[row] = NullValue
			continue
		}
		out[row] = value
	}
	return out
}

// dyadicDenseF64Reader returns a direct float64 row reader for dense numeric
// column producers; nil defers to the generic producer call chain.
func dyadicDenseF64Reader(p f64NumericProducer) (func(int) float64, bool) {
	switch v := p.(type) {
	case f64F64ColumnProducer:
		data := v.data
		return func(row int) float64 { return data[row] }, true
	case f64I64ColumnProducer:
		data := v.data
		return func(row int) float64 { return float64(data[row]) }, true
	case f64I32ColumnProducer:
		data := v.data
		return func(row int) float64 { return float64(data[row]) }, true
	case f64F32ColumnProducer:
		data := v.data
		return func(row int) float64 { return float64(data[row]) }, true
	default:
		return nil, false
	}
}

func (a f64NumericDyadicArray) Gather(indexes []int) Array {
	// Dense column-vs-column dyadics gather through one fused loop (two
	// direct slice reads plus the op) instead of three producer calls with
	// bool/error plumbing per row.
	if p := a.bound.producer; p.apply != nil && p.len == a.len {
		if lf, lok := dyadicDenseF64Reader(p.left); lok {
			if rf, rok := dyadicDenseF64Reader(p.right); rok {
				apply := p.apply
				fused := make([]float64, len(indexes))
				for i, row := range indexes {
					if row < 0 || row >= a.len {
						panic(fmt.Sprintf("data f64 dyadic gather row %d out of range", row))
					}
					fused[i] = apply(lf(row), rf(row))
				}
				return newF64Trusted(fused)
			}
		}
	}
	out := make([]float64, len(indexes))
	var nullable []any
	for i, row := range indexes {
		value, ok, err := a.f64At(row)
		if err != nil {
			panic(fmt.Sprintf("data f64 dyadic gather row %d out of range", row))
		}
		if !ok {
			if nullable == nil {
				nullable = make([]any, len(indexes))
				for j := 0; j < i; j++ {
					nullable[j] = out[j]
				}
			}
			nullable[i] = NullValue
			continue
		}
		out[i] = value
		if nullable != nil {
			nullable[i] = value
		}
	}
	if nullable != nil {
		return newNullableArray(KindF64, nullable)
	}
	return newF64Trusted(out)
}

func (a f64NumericDyadicArray) f64At(row int) (float64, bool, error) {
	if a.bound.producer.apply != nil && a.bound.producer.len == a.len {
		return a.bound.producer.f64At(row)
	}
	if row < 0 || row >= a.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	leftArray, leftIsArray := a.left.(Array)
	rightArray, rightIsArray := a.right.(Array)
	leftValue, leftOK, err := numericDyadicFloatOperandAt(a.left, leftArray, leftIsArray, row, a.len)
	if err != nil || !leftOK {
		return 0, leftOK, err
	}
	rightValue, rightOK, err := numericDyadicFloatOperandAt(a.right, rightArray, rightIsArray, row, a.len)
	if err != nil || !rightOK {
		return 0, rightOK, err
	}
	value, err := applyNumericDyadicFloatNumbers(a.op, leftValue, rightValue)
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

// f64NumericDyadicOperandsSum reduces an array<op>array dyadic tree by
// flattening both operands and accumulating apply(left[i], right[i]) directly
// — the same per-element values in the same order as flatten-then-sum, minus
// the materialized result buffer and its extra read pass.
func f64NumericDyadicOperandsSum(a f64NumericDyadicArray) (float64, bool) {
	leftArray, leftOK := a.left.(Array)
	rightArray, rightOK := a.right.(Array)
	if !leftOK || !rightOK || leftArray.Len() != a.len || rightArray.Len() != a.len {
		return 0, false
	}
	switch a.op {
	case string(OpAdd), string(OpSub), string(OpMul), string(OpDiv), string(OpMod):
	default:
		return 0, false
	}
	left, leftOwned, ok := tryBulkF64Values(leftArray)
	if !ok || len(left) < a.len {
		bulkF64Release(left, leftOwned)
		return 0, false
	}
	right, rightOwned, ok := tryBulkF64Values(rightArray)
	if !ok || len(right) < a.len {
		bulkF64Release(left, leftOwned)
		bulkF64Release(right, rightOwned)
		return 0, false
	}
	left = left[:a.len]
	right = right[:a.len]
	var total float64
	switch a.op {
	case string(OpAdd):
		for i, v := range left {
			total += v + right[i]
		}
	case string(OpSub):
		for i, v := range left {
			total += v - right[i]
		}
	case string(OpMul):
		for i, v := range left {
			total += v * right[i]
		}
	case string(OpDiv):
		for i, v := range left {
			total += v / right[i]
		}
	case string(OpMod):
		for i, v := range left {
			total += bulkF64ModFloat(v, right[i])
		}
	}
	bulkF64Release(left, leftOwned)
	bulkF64Release(right, rightOwned)
	return total, true
}

func f64NumericDyadicSum(array f64NumericDyadicArray) (any, bool, error) {
	if total, ok := f64NumericDyadicOperandsSum(array); ok {
		return total, true, nil
	}
	if values, ok := tryBulkF64NumericDyadicValues(array); ok {
		var total float64
		for _, value := range values {
			total += value
		}
		bulkF64Release(values, true)
		return total, true, nil
	}
	producer, err := newF64NumericDyadicProducer(array)
	if err != nil {
		return nil, true, err
	}
	if total, ok := f64DyadicProducerOperandsSum(producer); ok {
		return total, true, nil
	}
	if values, owned, ok := tryBulkF64ProducerValues(producer); ok {
		var total float64
		for _, value := range values {
			total += value
		}
		bulkF64Release(values, owned)
		return total, true, nil
	}
	total, err := f64ProducerSum(producer)
	if err != nil {
		return nil, true, err
	}
	return total, true, nil
}

// f64DyadicProducerOperandsSum reduces a dyadic producer by flattening both
// operand producers and accumulating the op directly — the same per-element
// values in the same order as flatten-then-sum, minus the result buffer and
// its extra read pass. Scalar-operand trees decline (the fused scalar flatten
// already produces a single buffer).
func f64DyadicProducerOperandsSum(p f64DyadicProducer) (float64, bool) {
	switch p.op {
	case string(OpAdd), string(OpSub), string(OpMul), string(OpDiv), string(OpMod):
	default:
		return 0, false
	}
	if _, scalarLeft := p.left.(f64ScalarProducer); scalarLeft {
		return 0, false
	}
	if _, scalarRight := p.right.(f64ScalarProducer); scalarRight {
		return 0, false
	}
	left, leftOwned, ok := tryBulkF64ProducerValues(p.left)
	if !ok || len(left) < p.len {
		bulkF64Release(left, leftOwned)
		return 0, false
	}
	right, rightOwned, ok := tryBulkF64ProducerValues(p.right)
	if !ok || len(right) < p.len {
		bulkF64Release(left, leftOwned)
		bulkF64Release(right, rightOwned)
		return 0, false
	}
	left = left[:p.len]
	right = right[:p.len]
	var total float64
	switch p.op {
	case string(OpAdd):
		for i, v := range left {
			total += v + right[i]
		}
	case string(OpSub):
		for i, v := range left {
			total += v - right[i]
		}
	case string(OpMul):
		for i, v := range left {
			total += v * right[i]
		}
	case string(OpDiv):
		for i, v := range left {
			total += v / right[i]
		}
	case string(OpMod):
		for i, v := range left {
			total += bulkF64ModFloat(v, right[i])
		}
	}
	bulkF64Release(left, leftOwned)
	bulkF64Release(right, rightOwned)
	return total, true
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
	// Dense-ish gathers (frame column clones, sort permutations) flatten
	// through the bulk kernel: one fused loop plus an index pass instead of
	// a carrier-tree walk per row. Sparse gathers (head-N of a long column)
	// keep the per-row path so they do not pay a full flatten.
	if len(indexes)*4 >= a.len {
		if values, owned, ok := tryBulkI64Values(a); ok {
			if len(indexes) == a.len && isIdentityIndexes(indexes) {
				if !owned {
					values = append([]int64(nil), values...)
				}
				return newI64Trusted(values)
			}
			out := make([]int64, len(indexes))
			for i, row := range indexes {
				if row < 0 || row >= len(values) {
					bulkI64Release(values, owned)
					panic(fmt.Sprintf("data scalar dyadic gather row %d out of range", row))
				}
				out[i] = values[row]
			}
			bulkI64Release(values, owned)
			return newI64Trusted(out)
		}
	}
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

func isIdentityIndexes(indexes []int) bool {
	for i, row := range indexes {
		if row != i {
			return false
		}
	}
	return true
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

// i64DyadicMinMaxArray is the lazy carrier for canonical q `&`/`|` —
// elementwise integer min/max — over two same-length bulk-flattenable
// integer vectors. Keeping the node lazy lets streaming consumers (where
// replication counts, sums, bulk flatten chains) combine the operands in
// pooled buffers instead of materializing the min/max vector per binding.
// Row values are bit-exact with the eager tryDyadicMinMaxI64Bulk loop.
type i64DyadicMinMaxArray struct {
	left    Array
	right   Array
	wantMax bool
	len     int
}

func (a i64DyadicMinMaxArray) Kind() Kind { return KindI64 }

func (a i64DyadicMinMaxArray) Len() int { return a.len }

func (a i64DyadicMinMaxArray) At(row int) (any, bool) {
	value, ok, err := a.i64At(row)
	if err != nil || !ok {
		return nil, false
	}
	return value, true
}

func (a i64DyadicMinMaxArray) Values() []any {
	if values, owned, ok := tryBulkI64Values(a); ok && len(values) >= a.len {
		out := make([]any, a.len)
		for row := range out {
			out[row] = values[row]
		}
		bulkI64Release(values, owned)
		return out
	}
	out := make([]any, a.len)
	for row := range out {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data dyadic min/max row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (a i64DyadicMinMaxArray) Gather(indexes []int) Array {
	// Dense-ish gathers flatten once through the bulk kernel; sparse gathers
	// keep the per-row path so they do not pay a full flatten (same policy
	// as i64ScalarDyadicArray).
	if len(indexes)*4 >= a.len {
		if values, owned, ok := tryBulkI64Values(a); ok {
			if len(indexes) == a.len && isIdentityIndexes(indexes) {
				if !owned {
					values = append([]int64(nil), values...)
				}
				return newI64Trusted(values)
			}
			out := make([]int64, len(indexes))
			for i, row := range indexes {
				if row < 0 || row >= len(values) {
					bulkI64Release(values, owned)
					panic(fmt.Sprintf("data dyadic min/max gather row %d out of range", row))
				}
				out[i] = values[row]
			}
			bulkI64Release(values, owned)
			return newI64Trusted(out)
		}
	}
	out := make([]int64, len(indexes))
	for i, row := range indexes {
		value, ok, err := a.i64At(row)
		if err != nil || !ok {
			panic(fmt.Sprintf("data dyadic min/max gather row %d out of range", row))
		}
		out[i] = value
	}
	return newI64Trusted(out)
}

func (a i64DyadicMinMaxArray) i64At(row int) (int64, bool, error) {
	if row < 0 || row >= a.len {
		return 0, false, fmt.Errorf("array row %d out of range", row)
	}
	lv, ok, err := integerArrayAt(a.left, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	rv, ok, err := integerArrayAt(a.right, row)
	if err != nil || !ok {
		return 0, ok, err
	}
	if a.wantMax == (rv > lv) {
		lv = rv
	}
	return lv, true, nil
}

// lazyMinMaxI64Carrier gates the lazy min/max node on operand carriers whose
// bulk flatten (tryBulkI64Values) is known to succeed, so the node never
// strands a consumer on the per-row walk for the dense shapes the eager
// kernel handled.
func lazyMinMaxI64Carrier(array Array) bool {
	switch a := array.(type) {
	case attributedArray:
		return lazyMinMaxI64Carrier(a.array)
	case columnArray[int64]:
		return true
	case i64RangeArray, i64SegmentArray:
		return true
	case i64ScalarDyadicArray:
		return lazyMinMaxI64Carrier(a.source)
	case i64DyadicMinMaxArray:
		return lazyMinMaxI64Carrier(a.left) && lazyMinMaxI64Carrier(a.right)
	}
	return false
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
	// Dense single pass first: flatten the dyadic source once and prefix-sum,
	// covering op/scalar shapes the closed-form range sums decline (e.g.
	// negative-modulus mod) in O(n) instead of per-row prefix walks.
	if values, owned, ok := tryBulkI64Values(a.source); ok && len(values) >= a.Len() {
		out := make([]any, a.Len())
		var acc int64
		for row := range out {
			acc += values[row]
			out[row] = acc
		}
		bulkI64Release(values, owned)
		return out
	}
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
	value, handled, err := i64ScalarDyadicRangeSumI64(a.source, 0, row+1)
	if handled || err != nil {
		return value, handled, err
	}
	// The closed-form range sums decline some op/scalar shapes (e.g.
	// negative-modulus mod): accumulate the prefix directly so the decline
	// never surfaces as an out-of-range panic at materialization.
	var acc int64
	for i := 0; i <= row; i++ {
		v, ok, err := a.source.i64At(i)
		if err != nil || !ok {
			return 0, ok, err
		}
		acc += v
	}
	return acc, true, nil
}

func i64ScalarDyadicRunningSumSum(array i64ScalarDyadicRunningSumArray) (int64, bool, error) {
	// Flatten the dyadic source once and reduce the running sum in a single
	// dense pass instead of evaluating a prefix range sum per row.
	if values, owned, ok := tryBulkI64Values(array.source); ok && len(values) >= array.Len() {
		var acc, total int64
		for _, v := range values[:array.Len()] {
			acc += v
			total += acc
		}
		bulkI64Release(values, owned)
		return total, true, nil
	}
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
	if out, handled, err := i64ScalarDyadicLinearSum(array); handled || err != nil {
		return out, handled, err
	}
	return i64ScalarDyadicRangeSum(array, 0, array.len)
}

// i64ScalarDyadicLinearSum reduces scalar +, -, and * carriers to one source
// sum. Two's-complement wrap-around matches per-element evaluation because
// int64 add/sub/mul are ring operations mod 2^64.
func i64ScalarDyadicLinearSum(array i64ScalarDyadicArray) (any, bool, error) {
	switch array.op {
	case OpAdd, OpSub, OpMul:
	default:
		return nil, false, nil
	}
	if array.source == nil || array.source.Len() != array.len {
		return nil, false, nil
	}
	value, handled, err := typedKernels.NumericSumValue(array.source)
	if err != nil || !handled {
		return nil, false, err
	}
	sum, ok := value.(int64)
	if !ok {
		return nil, false, nil
	}
	n := int64(array.len)
	switch array.op {
	case OpAdd:
		return sum + array.scalar*n, true, nil
	case OpMul:
		return sum * array.scalar, true, nil
	default: // OpSub
		if array.scalarLeft {
			return array.scalar*n - sum, true, nil
		}
		return sum - array.scalar*n, true, nil
	}
}

func i64ScalarDyadicWholeRangeSum(array i64ScalarDyadicArray) (int64, bool) {
	if array.op != OpMod || array.scalarLeft || array.scalar <= 0 {
		return 0, false
	}
	switch source := array.source.(type) {
	case i64RangeArray:
		if source.step == 1 {
			return i64RangePositiveModSum(source.start, array.len, array.scalar), true
		}
		return i64RangePositiveModStepSum(source.start, source.step, array.len, array.scalar)
	case i64ScalarDyadicArray:
		return i64NestedPositiveModSum(source, 0, array.len, array.scalar)
	default:
		return 0, false
	}
}

func i64ScalarDyadicModuloRangeSum(array i64ScalarDyadicArray, start, length int) (int64, bool) {
	if array.op != OpMod || array.scalarLeft || array.scalar <= 0 || length < 0 {
		return 0, false
	}
	switch source := array.source.(type) {
	case i64RangeArray:
		startValue := source.start + int64(start)*source.step
		if source.step == 1 {
			return i64RangePositiveModSum(startValue, length, array.scalar), true
		}
		return i64RangePositiveModStepSum(startValue, source.step, length, array.scalar)
	case i64ScalarDyadicArray:
		return i64NestedPositiveModSum(source, start, length, array.scalar)
	default:
		return 0, false
	}
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

func i64RangePositiveModStepSum(start, step int64, length int, modulus int64) (int64, bool) {
	if length <= 0 {
		return 0, true
	}
	if modulus <= 0 {
		return 0, false
	}
	period := modulus / gcdInt64(step, modulus)
	if period <= 0 || period > 1<<20 {
		return 0, false
	}
	var periodSum int64
	value := qPositiveMod(start, modulus)
	step = qPositiveMod(step, modulus)
	for i := int64(0); i < period; i++ {
		periodSum += value
		value = (value + step) % modulus
	}
	n := int64(length)
	full := n / period
	rem := n % period
	sum := full * periodSum
	value = qPositiveMod(start, modulus)
	for i := int64(0); i < rem; i++ {
		sum += value
		value = (value + step) % modulus
	}
	return sum, true
}

func i64NestedPositiveModSum(source i64ScalarDyadicArray, start, length int, outerModulus int64) (int64, bool) {
	if source.op != OpMod || source.scalarLeft || source.scalar <= 0 || outerModulus <= 0 || length < 0 {
		return 0, false
	}
	rangeSource, ok := source.source.(i64RangeArray)
	if !ok {
		return 0, false
	}
	startValue := rangeSource.start + int64(start)*rangeSource.step
	return i64NestedPositiveModStepSum(startValue, rangeSource.step, length, source.scalar, outerModulus)
}

func i64NestedPositiveModStepSum(start, step int64, length int, innerModulus, outerModulus int64) (int64, bool) {
	if length <= 0 {
		return 0, true
	}
	if innerModulus <= 0 || outerModulus <= 0 {
		return 0, false
	}
	period := innerModulus / gcdInt64(step, innerModulus)
	if period <= 0 || period > 1<<20 {
		return 0, false
	}
	var periodSum int64
	value := qPositiveMod(start, innerModulus)
	step = qPositiveMod(step, innerModulus)
	for i := int64(0); i < period; i++ {
		periodSum += qPositiveMod(value, outerModulus)
		value = (value + step) % innerModulus
	}
	n := int64(length)
	full := n / period
	rem := n % period
	sum := full * periodSum
	value = qPositiveMod(start, innerModulus)
	for i := int64(0); i < rem; i++ {
		sum += qPositiveMod(value, outerModulus)
		value = (value + step) % innerModulus
	}
	return sum, true
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

func i64ScalarDyadicCompareSegmentModuloIndexes(mask i64ScalarDyadicCompareMask) (Array, bool) {
	values := mask.values
	if values.op != OpMod || values.scalarLeft || values.scalar <= 0 || values.len <= 0 || values.len != values.source.Len() {
		return nil, false
	}
	if mask.scalarLeft && mask.op != OpEQ {
		return nil, false
	}
	return i64SegmentModuloEqualIndexes(values.source, values.scalar, mask.op, mask.scalar)
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

// tiledRowOccurrences counts rows r in [0, length) whose tiled source index
// (start+r) mod sourceLen equals j (the arithmetic-progression closed form
// shared with the fby residue kernels).
func tiledRowOccurrences(start, length, sourceLen, j int) int64 {
	r0 := (j - start) % sourceLen
	if r0 < 0 {
		r0 += sourceLen
	}
	if r0 >= length {
		return 0
	}
	return int64((length-1-r0)/sourceLen) + 1
}

func (a notMask) trueCount() (int64, bool, error) {
	if tiled, ok := a.array.(tiledArray); ok && tiled.source.Len() > 0 {
		// O(period) cycle count: the mask value is constant per source
		// index, so count one period and weight by row occurrences.
		inner := notMask{array: tiled.source}
		sourceLen := tiled.source.Len()
		total := int64(0)
		handled := true
		for j := 0; j < sourceLen; j++ {
			value, ok, err := inner.valueAt(j)
			if err != nil {
				return 0, true, err
			}
			if !ok {
				handled = false
				break
			}
			if value {
				total += tiledRowOccurrences(tiled.start, tiled.len, sourceLen, j)
			}
		}
		if handled {
			return total, true, nil
		}
	}
	if values, owned, ok := tryBulkBoolValues(a); ok {
		var count int64
		for _, value := range values {
			if value {
				count++
			}
		}
		bulkBoolRelease(values, owned)
		return count, true, nil
	}
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
		if a.left.Len() == 0 {
			return false, true, fmt.Errorf("logical row %d out of range", row)
		}
		value, ok, err := boolArrayAt(a.left, row%a.left.Len())
		if err != nil || !ok {
			return false, ok, err
		}
		left = value
	}
	right := a.rightScalar
	if !a.rightIsScalar {
		if a.right.Len() == 0 {
			return false, true, fmt.Errorf("logical row %d out of range", row)
		}
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
	if values, owned, ok := tryBulkBoolValues(a); ok {
		var count int64
		for _, keep := range values {
			if keep {
				count++
			}
		}
		bulkBoolRelease(values, owned)
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
	if leftOK && rightOK && sameI64Range(left.values, right.values) {
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
	leftSegment, leftSegmentOK := a.left.(i64SegmentCompareMask)
	rightSegment, rightSegmentOK := a.right.(i64SegmentCompareMask)
	if !leftSegmentOK || !rightSegmentOK || !sameI64Segment(leftSegment.values, rightSegment.values) {
		return 0, false
	}
	leftLow, leftHigh, ok := compareSegmentMaskValueInterval(leftSegment)
	if !ok {
		return 0, false
	}
	rightLow, rightHigh, ok := compareSegmentMaskValueInterval(rightSegment)
	if !ok {
		return 0, false
	}
	intersect, ok := i64SegmentIntervalIndexArray(leftSegment.values, maxInt64Value(leftLow, rightLow), minInt64Value(leftHigh, rightHigh))
	if !ok {
		return 0, false
	}
	if a.op == "and" {
		return int64(intersect.Len()), true
	}
	if a.op == "or" {
		leftIndexes, leftOK := i64SegmentIntervalIndexArray(leftSegment.values, leftLow, leftHigh)
		rightIndexes, rightOK := i64SegmentIntervalIndexArray(rightSegment.values, rightLow, rightHigh)
		if !leftOK || !rightOK {
			return 0, false
		}
		return int64(leftIndexes.Len() + rightIndexes.Len() - intersect.Len()), true
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

func sameI64Segment(left, right i64SegmentArray) bool {
	if left.len != right.len || len(left.segments) != len(right.segments) {
		return false
	}
	for i := range left.segments {
		if !sameI64Range(left.segments[i], right.segments[i]) {
			return false
		}
	}
	return true
}

func compareMaskValueInterval(mask i64RangeCompareMask) (int64, int64, bool) {
	return compareValueInterval(effectiveRangeCompareOp(mask.op, mask.scalarLeft), mask.scalar)
}

func compareSegmentMaskValueInterval(mask i64SegmentCompareMask) (int64, int64, bool) {
	return compareValueInterval(effectiveRangeCompareOp(mask.op, mask.scalarLeft), mask.scalar)
}

func compareValueInterval(op Op, scalar int64) (int64, int64, bool) {
	minInt := -int64(^uint64(0)>>1) - 1
	maxInt := int64(^uint64(0) >> 1)
	switch op {
	case OpEQ:
		return scalar, scalar, true
	case OpGT:
		if scalar == maxInt {
			return 1, 0, true
		}
		return scalar + 1, maxInt, true
	case OpGE:
		return scalar, maxInt, true
	case OpLT:
		if scalar == minInt {
			return 1, 0, true
		}
		return minInt, scalar - 1, true
	case OpLE:
		return minInt, scalar, true
	default:
		return 0, 0, false
	}
}

func i64RangeIntervalCount(values i64RangeArray, low, high int64) int64 {
	if values.len <= 0 || low > high {
		return 0
	}
	step := values.step
	if step == 0 {
		if low <= values.start && values.start <= high {
			return int64(values.len)
		}
		return 0
	}
	if step < 0 {
		// A descending range holds the same value multiset as the ascending
		// range starting at its minimum, so counting is order-insensitive.
		return i64RangeIntervalCount(i64RangeArray{
			start: values.start + int64(values.len-1)*step,
			step:  -step,
			len:   values.len,
		}, low, high)
	}
	start := values.start
	last := start + int64(values.len-1)*step
	if high < start || low > last {
		return 0
	}
	firstRow := int64(0)
	if low > start {
		firstRow = (low - start + step - 1) / step
	}
	lastRow := int64(values.len - 1)
	if high < last {
		lastRow = (high - start) / step
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
	case i64SegmentCompareMask:
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
		// Generic fallback for lazy bool carriers without a dedicated case
		// (membership masks, float compare masks, ...): route through At so
		// composed masks (not/&/|) over them materialize instead of
		// translating the unknown type into an out-of-range panic.
		if array.Kind() != KindBool {
			return false, false, nil
		}
		value, ok := array.At(row)
		if !ok {
			return false, true, fmt.Errorf("logical row %d out of range", row)
		}
		out, isBool := value.(bool)
		if !isBool {
			return false, false, nil
		}
		return out, true, nil
	}
}

// boolScalarValue admits only genuine booleans: under canonical q `&`/`|`
// are min/max, so a null operand must fall back to the boxed route (nulls
// sort smallest) instead of coercing to false inside the logical mask.
func boolScalarValue(value any) (bool, bool) {
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

func membershipBoolMask(values []bool, set map[bool]struct{}, ok bool) Array {
	if !ok {
		return nil
	}
	out := make([]bool, len(values))
	for row, value := range values {
		_, out[row] = set[value]
	}
	return newBoolTrusted(out)
}

func membershipSignedMask[T signedScalar](values []T, set map[T]struct{}, ok bool) Array {
	if !ok {
		return nil
	}
	out := make([]bool, len(values))
	for row, value := range values {
		_, out[row] = set[value]
	}
	return newBoolTrusted(out)
}

func membershipUnsignedMask[T unsignedScalar](values []T, set map[T]struct{}, ok bool) Array {
	if !ok {
		return nil
	}
	out := make([]bool, len(values))
	for row, value := range values {
		_, out[row] = set[value]
	}
	return newBoolTrusted(out)
}

func membershipStringMask(values []string, set map[string]struct{}, ok bool) Array {
	if !ok {
		return nil
	}
	out := make([]bool, len(values))
	for row, value := range values {
		_, out[row] = set[value]
	}
	return newBoolTrusted(out)
}

func membershipSymbolMask(values []Symbol, set map[Symbol]struct{}, ok bool) Array {
	if !ok {
		return nil
	}
	out := make([]bool, len(values))
	for row, value := range values {
		_, out[row] = set[value]
	}
	return newBoolTrusted(out)
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
