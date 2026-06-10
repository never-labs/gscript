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
	case columnArray[bool]:
		copy(dst, a.data)
		return true, nil
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
	case columnArray[string]:
		copy(dst, a.data)
		return true, nil
	}
	return false, nil
}
