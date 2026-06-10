package data

import "fmt"

// TryExportI64Copy copies a typed i64-compatible array into dst without
// materializing []any or calling Array.At for every row. The bool return is
// false when the array carrier is not supported by the bulk exporter.
func TryExportI64Copy(array Array, dst []int64) (bool, error) {
	if array == nil {
		return false, nil
	}
	if len(dst) != array.Len() {
		return true, fmt.Errorf("i64 export destination length %d does not match array length %d", len(dst), array.Len())
	}
	switch a := array.(type) {
	case attributedArray:
		return TryExportI64Copy(a.array, dst)
	case indexedArray:
		for row := range dst {
			index, ok, err := i64IndexArrayAt(a.indexes, row)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fmt.Errorf("indexed i64 export row %d out of range", row)
			}
			value, ok, err := integerArrayAt(a.source, index)
			if err != nil {
				return true, err
			}
			if !ok {
				return false, nil
			}
			dst[row] = value
		}
		return true, nil
	case columnArray[int64]:
		copy(dst, a.data)
		return true, nil
	case columnArray[int32]:
		for i, v := range a.data {
			dst[i] = int64(v)
		}
		return true, nil
	case columnArray[int16]:
		for i, v := range a.data {
			dst[i] = int64(v)
		}
		return true, nil
	case columnArray[int8]:
		for i, v := range a.data {
			dst[i] = int64(v)
		}
		return true, nil
	case i64RangeArray:
		for i := range dst {
			dst[i] = a.start + int64(i)*a.step
		}
		return true, nil
	case i64SegmentArray:
		row := 0
		for _, segment := range a.segments {
			for i := 0; i < segment.len; i++ {
				dst[row] = segment.start + int64(i)*segment.step
				row++
			}
		}
		return true, nil
	case i64PeriodicIndexArray:
		for i := range dst {
			v, ok := a.i64At(i)
			if !ok {
				return true, fmt.Errorf("periodic i64 export row %d out of range", i)
			}
			dst[i] = v
		}
		return true, nil
	case i64RunningSumArray:
		for i := range dst {
			v, ok := a.i64At(i)
			if !ok {
				return true, fmt.Errorf("running i64 export row %d out of range", i)
			}
			dst[i] = v
		}
		return true, nil
	case i64ProductArray:
		for i := range dst {
			v, ok := a.i64At(i)
			if !ok {
				return true, fmt.Errorf("product i64 export row %d out of range", i)
			}
			dst[i] = v
		}
		return true, nil
	case i64BucketArray:
		for i := range dst {
			v, ok, err := a.i64At(i)
			if err != nil {
				return true, err
			}
			if !ok {
				return false, nil
			}
			dst[i] = v
		}
		return true, nil
	case i64XrankArray:
		for i := range dst {
			v, ok, err := a.i64At(i)
			if err != nil {
				return true, err
			}
			if !ok {
				return false, nil
			}
			dst[i] = v
		}
		return true, nil
	case i64FillArray:
		for i := range dst {
			v, ok, err := a.valueAt(i)
			if err != nil {
				return true, err
			}
			if !ok {
				return false, nil
			}
			dst[i] = v
		}
		return true, nil
	case nullableArray:
		return exportNullableI64Copy(a, dst)
	case shiftedArray:
		if a.offset != 0 {
			return false, nil
		}
		return TryExportI64Copy(a.source, dst)
	}
	return false, nil
}

// TryExportF64Copy copies a typed f64-compatible array into dst without
// materializing []any or per-row dynamic Array.At calls.
func TryExportF64Copy(array Array, dst []float64) (bool, error) {
	if array == nil {
		return false, nil
	}
	if len(dst) != array.Len() {
		return true, fmt.Errorf("f64 export destination length %d does not match array length %d", len(dst), array.Len())
	}
	switch a := array.(type) {
	case attributedArray:
		return TryExportF64Copy(a.array, dst)
	case indexedArray:
		for row := range dst {
			index, ok, err := i64IndexArrayAt(a.indexes, row)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fmt.Errorf("indexed f64 export row %d out of range", row)
			}
			value, ok, err := numericAt(a.source, index)
			if err != nil {
				return true, err
			}
			if !ok {
				return false, nil
			}
			dst[row] = value
		}
		return true, nil
	case columnArray[float64]:
		copy(dst, a.data)
		return true, nil
	case columnArray[float32]:
		for i, v := range a.data {
			dst[i] = float64(v)
		}
		return true, nil
	case f64RangeArray:
		for i := range dst {
			dst[i] = a.start + float64(i)*a.step
		}
		return true, nil
	case f64RunningSumArray:
		for i := range dst {
			v, ok := a.f64At(i)
			if !ok {
				return true, fmt.Errorf("running f64 export row %d out of range", i)
			}
			dst[i] = v
		}
		return true, nil
	case f64BucketArray:
		for i := range dst {
			v, ok, err := a.f64At(i)
			if err != nil {
				return true, err
			}
			if !ok {
				return false, nil
			}
			dst[i] = v
		}
		return true, nil
	case f64FillArray:
		for i := range dst {
			v, ok, err := a.valueAt(i)
			if err != nil {
				return true, err
			}
			if !ok {
				return false, nil
			}
			dst[i] = v
		}
		return true, nil
	case nullableArray:
		return exportNullableF64Copy(a, dst)
	case shiftedArray:
		if a.offset != 0 {
			return false, nil
		}
		return TryExportF64Copy(a.source, dst)
	}
	return false, nil
}

// TryExportBoolCopy copies a typed boolean array into dst without boxing.
func TryExportBoolCopy(array Array, dst []bool) (bool, error) {
	if array == nil {
		return false, nil
	}
	if len(dst) != array.Len() {
		return true, fmt.Errorf("bool export destination length %d does not match array length %d", len(dst), array.Len())
	}
	switch a := array.(type) {
	case attributedArray:
		return TryExportBoolCopy(a.array, dst)
	case indexedArray:
		for row := range dst {
			index, ok, err := i64IndexArrayAt(a.indexes, row)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fmt.Errorf("indexed bool export row %d out of range", row)
			}
			value, ok, err := boolArrayAt(a.source, index)
			if err != nil {
				return true, err
			}
			if !ok {
				return false, nil
			}
			dst[row] = value
		}
		return true, nil
	case columnArray[bool]:
		copy(dst, a.data)
		return true, nil
	case nullableArray:
		return exportNullableBoolCopy(a, dst)
	case shiftedArray:
		if a.offset != 0 {
			return false, nil
		}
		return TryExportBoolCopy(a.source, dst)
	}
	return false, nil
}

// TryExportStringCopy copies a typed string array into dst without boxing.
func TryExportStringCopy(array Array, dst []string) (bool, error) {
	if array == nil {
		return false, nil
	}
	if len(dst) != array.Len() {
		return true, fmt.Errorf("string export destination length %d does not match array length %d", len(dst), array.Len())
	}
	switch a := array.(type) {
	case attributedArray:
		return TryExportStringCopy(a.array, dst)
	case indexedArray:
		for row := range dst {
			index, ok, err := i64IndexArrayAt(a.indexes, row)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fmt.Errorf("indexed string export row %d out of range", row)
			}
			value, ok := stringArrayAt(a.source, index)
			if !ok {
				return false, nil
			}
			dst[row] = value
		}
		return true, nil
	case columnArray[string]:
		copy(dst, a.data)
		return true, nil
	case columnArray[Symbol]:
		for i, value := range a.data {
			dst[i] = string(value)
		}
		return true, nil
	case nullableArray:
		return exportNullableStringCopy(a, dst)
	case shiftedArray:
		if a.offset != 0 {
			return false, nil
		}
		return TryExportStringCopy(a.source, dst)
	}
	return false, nil
}

func exportNullableI64Copy(array nullableArray, dst []int64) (bool, error) {
	for i, value := range array.data {
		if IsNull(value) {
			return false, nil
		}
		n, ok := integerScalarValue(value)
		if !ok {
			return false, nil
		}
		dst[i] = n
	}
	return true, nil
}

func exportNullableF64Copy(array nullableArray, dst []float64) (bool, error) {
	for i, value := range array.data {
		if IsNull(value) {
			return false, nil
		}
		n, ok := numeric(value)
		if !ok {
			return false, nil
		}
		dst[i] = n
	}
	return true, nil
}

func exportNullableBoolCopy(array nullableArray, dst []bool) (bool, error) {
	for i, value := range array.data {
		if IsNull(value) {
			return false, nil
		}
		v, ok := value.(bool)
		if !ok {
			return false, nil
		}
		dst[i] = v
	}
	return true, nil
}

func exportNullableStringCopy(array nullableArray, dst []string) (bool, error) {
	for i, value := range array.data {
		if IsNull(value) {
			return false, nil
		}
		switch v := value.(type) {
		case string:
			dst[i] = v
		case Symbol:
			dst[i] = string(v)
		default:
			return false, nil
		}
	}
	return true, nil
}

func stringArrayAt(array Array, row int) (string, bool) {
	switch a := array.(type) {
	case attributedArray:
		return stringArrayAt(a.array, row)
	case indexedArray:
		index, ok, err := i64IndexArrayAt(a.indexes, row)
		if err != nil || !ok {
			return "", false
		}
		return stringArrayAt(a.source, index)
	case columnArray[string]:
		if row < 0 || row >= len(a.data) {
			return "", false
		}
		return a.data[row], true
	case columnArray[Symbol]:
		if row < 0 || row >= len(a.data) {
			return "", false
		}
		return string(a.data[row]), true
	case nullableArray:
		if row < 0 || row >= len(a.data) || IsNull(a.data[row]) {
			return "", false
		}
		switch value := a.data[row].(type) {
		case string:
			return value, true
		case Symbol:
			return string(value), true
		default:
			return "", false
		}
	case shiftedArray:
		if a.offset != 0 {
			return "", false
		}
		return stringArrayAt(a.source, row)
	default:
		value, ok := array.At(row)
		if !ok || IsNull(value) {
			return "", false
		}
		out, ok := value.(string)
		return out, ok
	}
}
