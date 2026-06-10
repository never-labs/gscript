package data

import "fmt"

// TryTypedScalarIndex reads one row from an Array through the data runtime
// carrier layer. It gives q/@/dot apply-index and future JIT backends a single
// primitive to target instead of routing scalar indexing through Array.At first.
func TryTypedScalarIndex(array Array, row int) (any, bool, error) {
	if array == nil {
		return nil, false, nil
	}
	switch a := array.(type) {
	case attributedArray:
		return TryTypedScalarIndex(a.array, row)
	case columnArray[bool]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[int8]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[int16]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[int32]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[int64]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[uint8]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[uint16]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[uint32]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[uint64]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[float32]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[float64]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[string]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[Symbol]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[Month]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[Date]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[DateTime]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[Timespan]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[Minute]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[Second]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[Time]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[Timestamp]:
		return scalarIndexFromSlice(a.data, row)
	case columnArray[any]:
		return scalarIndexFromSlice(a.data, row)
	case nullableArray:
		return scalarIndexFromSlice(a.data, row)
	case indexedArray:
		index, ok, err := i64IndexArrayAt(a.indexes, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return TryTypedScalarIndex(a.source, index)
	case i64RangeArray:
		if row < 0 || row >= a.len {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return a.start + int64(row)*a.step, true, nil
	case f64RangeArray:
		if row < 0 || row >= a.len {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return a.start + float64(row)*a.step, true, nil
	case i64RunningSumArray:
		value, ok := a.i64At(row)
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return value, true, nil
	case f64RunningSumArray:
		value, ok := a.f64At(row)
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return value, true, nil
	case i64ProductArray:
		value, ok := a.i64At(row)
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return value, true, nil
	case i64SegmentArray:
		value, ok := a.i64At(row)
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return value, true, nil
	case i64PeriodicIndexArray:
		value, ok := a.i64At(row)
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return value, true, nil
	case i64BucketArray:
		value, ok, err := a.i64At(row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return value, true, nil
	case f64BucketArray:
		value, ok, err := a.f64At(row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		if a.kind == KindF32 {
			return float32(value), true, nil
		}
		return value, true, nil
	case i64XrankArray:
		value, ok, err := a.i64At(row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return value, true, nil
	case tiledArray:
		if row < 0 || row >= a.len || a.source.Len() == 0 {
			return nil, true, scalarIndexOutOfRange(row)
		}
		return TryTypedScalarIndex(a.source, (a.start+row)%a.source.Len())
	case shiftedArray:
		if row < 0 || row >= a.Len() {
			return nil, true, scalarIndexOutOfRange(row)
		}
		sourceRow := row + a.offset
		if sourceRow < 0 || sourceRow >= a.source.Len() {
			return NullValue, true, nil
		}
		value, handled, err := TryTypedScalarIndex(a.source, sourceRow)
		if err != nil || !handled {
			return value, handled, err
		}
		if IsNull(value) {
			return NullValue, true, nil
		}
		return value, true, nil
	case encodedArray:
		if row < 0 || row >= len(a.codes) {
			return nil, true, scalarIndexOutOfRange(row)
		}
		code := a.codes[row]
		if code < 0 {
			return NullValue, true, nil
		}
		if int(code) >= len(a.domain) {
			return nil, true, fmt.Errorf("encoded array code %d at row %d outside domain length %d", code, row, len(a.domain))
		}
		return a.domain[code], true, nil
	default:
		return nil, false, nil
	}
}

func scalarIndexFromSlice[T any](values []T, row int) (any, bool, error) {
	if row < 0 || row >= len(values) {
		return nil, true, scalarIndexOutOfRange(row)
	}
	return values[row], true, nil
}

func scalarIndexOutOfRange(row int) error {
	return fmt.Errorf("index %d out of range", row)
}
