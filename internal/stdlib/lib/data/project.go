package data

import "fmt"

// TryProjectByI64IndexArray materializes selected rows from carrier arrays with
// a typed i64 index vector. It is intentionally narrower than
// TryGatherByI64IndexArray: regular columns/ranges can stay lazy indexed views,
// while carriers that would otherwise fall through per-row Array.At get one
// typed project primitive for q/runtime/JIT backends to target.
func TryProjectByI64IndexArray(array Array, indexes Array) (Array, bool, error) {
	if array == nil || indexes == nil {
		return nil, true, fmt.Errorf("project array and indexes must be non-nil")
	}
	if indexes.Kind() != KindI64 {
		return nil, true, fmt.Errorf("index vector kind is %s, want %s", indexes.Kind(), KindI64)
	}
	if ok, err := validateI64IndexArray(indexes, array.Len()); err != nil || !ok {
		return nil, ok, err
	}
	return projectValidatedByI64IndexArray(array, indexes)
}

func projectValidatedByI64IndexArray(array Array, indexes Array) (Array, bool, error) {
	switch a := array.(type) {
	case attributedArray:
		out, handled, err := projectValidatedByI64IndexArray(a.array, indexes)
		if err != nil || !handled {
			return out, handled, err
		}
		return attributedArray{array: out, metadata: a.metadata.cloneWithRebuiltIndexes(out)}, true, nil
	case encodedArray:
		out := make([]int32, indexes.Len())
		for row := range out {
			index, ok, err := i64IndexArrayAt(indexes, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, true, fmt.Errorf("project index row %d out of range", row)
			}
			out[row] = a.codes[index]
		}
		return encodedArray{kind: a.kind, domain: append([]any(nil), a.domain...), codes: out}, true, nil
	case nullableArray:
		out := make([]any, indexes.Len())
		for row := range out {
			index, ok, err := i64IndexArrayAt(indexes, row)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				return nil, true, fmt.Errorf("project index row %d out of range", row)
			}
			out[row] = a.data[index]
		}
		return nullableArray{kind: a.kind, data: out}, true, nil
	case nullBitmapArray[int8]:
		return projectNullBitmapArray(a, indexes)
	case nullBitmapArray[int16]:
		return projectNullBitmapArray(a, indexes)
	case nullBitmapArray[int32]:
		return projectNullBitmapArray(a, indexes)
	case nullBitmapArray[int64]:
		return projectNullBitmapArray(a, indexes)
	case nullBitmapArray[float32]:
		return projectNullBitmapArray(a, indexes)
	case nullBitmapArray[float64]:
		return projectNullBitmapArray(a, indexes)
	case tiledArray:
		return projectTiledByI64IndexArray(a, indexes)
	case shiftedArray:
		return projectByTypedScalarIndex(a, indexes)
	case i64FillArray:
		return projectI64FillByI64IndexArray(a, indexes)
	case f64FillArray:
		return projectF64FillByI64IndexArray(a, indexes)
	default:
		return nil, false, nil
	}
}

func projectNullBitmapArray[T nullBitmapElem](array nullBitmapArray[T], indexes Array) (Array, bool, error) {
	out := make([]T, indexes.Len())
	nulls := newNullBitmap(indexes.Len())
	hasNull := false
	for row := range out {
		index, ok, err := i64IndexArrayAt(indexes, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("project index row %d out of range", row)
		}
		if nullBitGet(array.nulls, index) {
			nullBitSet(nulls, row)
			hasNull = true
			continue
		}
		out[row] = array.data[index]
	}
	if !hasNull {
		return columnArray[T]{kind: array.kind, data: out}, true, nil
	}
	return nullBitmapArray[T]{kind: array.kind, data: out, nulls: nulls}, true, nil
}

func projectTiledByI64IndexArray(array tiledArray, indexes Array) (Array, bool, error) {
	sourceLen := array.source.Len()
	if sourceLen == 0 {
		if indexes.Len() == 0 {
			return nullableArray{kind: array.Kind(), data: nil}, true, nil
		}
		return nil, true, fmt.Errorf("project tiled source is empty")
	}
	sourceRows := make([]int64, indexes.Len())
	for row := range sourceRows {
		index, ok, err := i64IndexArrayAt(indexes, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("project index row %d out of range", row)
		}
		sourceRows[row] = int64((array.start + index) % sourceLen)
	}
	sourceIndexes := newI64Trusted(sourceRows)
	if out, handled, err := TryProjectByI64IndexArray(array.source, sourceIndexes); handled || err != nil {
		return out, handled, err
	}
	return TryGatherByI64IndexArray(array.source, sourceIndexes)
}

func projectByTypedScalarIndex(array Array, indexes Array) (Array, bool, error) {
	out := make([]any, indexes.Len())
	for row := range out {
		index, ok, err := i64IndexArrayAt(indexes, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("project index row %d out of range", row)
		}
		value, handled, err := TryTypedScalarIndex(array, index)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return nil, false, nil
		}
		out[row] = value
	}
	return nullableArray{kind: array.Kind(), data: out}, true, nil
}

func projectI64FillByI64IndexArray(array i64FillArray, indexes Array) (Array, bool, error) {
	out := make([]int64, indexes.Len())
	for row := range out {
		index, ok, err := i64IndexArrayAt(indexes, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("project index row %d out of range", row)
		}
		value, ok, err := array.valueAt(index)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("project index row %d out of range", row)
		}
		out[row] = value
	}
	return newI64Trusted(out), true, nil
}

func projectF64FillByI64IndexArray(array f64FillArray, indexes Array) (Array, bool, error) {
	out := make([]float64, indexes.Len())
	for row := range out {
		index, ok, err := i64IndexArrayAt(indexes, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("project index row %d out of range", row)
		}
		value, ok, err := array.valueAt(index)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, fmt.Errorf("project index row %d out of range", row)
		}
		out[row] = value
	}
	return newF64Trusted(out), true, nil
}
