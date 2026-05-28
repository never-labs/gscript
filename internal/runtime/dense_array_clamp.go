package runtime

import "fmt"

func denseArrayClamp(src *DenseArray, minValue, maxValue Value) (*DenseArray, error) {
	if src == nil {
		return nil, ErrDenseArrayOperand
	}
	switch src.dtype {
	case DenseArrayF64:
		min, max, err := denseArrayF64ClampBounds(minValue, maxValue)
		if err != nil {
			return nil, err
		}
		out := make([]float64, len(src.f64))
		denseArrayClampF64Into(out, src.f64, min, max)
		return &DenseArray{dtype: DenseArrayF64, f64: out}, nil
	case DenseArrayI64:
		min, max, err := denseArrayI64ClampBounds(minValue, maxValue)
		if err != nil {
			return nil, err
		}
		out := make([]int64, len(src.i64))
		denseArrayClampI64Into(out, src.i64, min, max)
		return &DenseArray{dtype: DenseArrayI64, i64: out}, nil
	default:
		return nil, ErrDenseArrayDType
	}
}

func denseArrayClampInto(dst, src *DenseArray, minValue, maxValue Value) error {
	if dst == nil || src == nil {
		return ErrDenseArrayOperand
	}
	if dst.Len() != src.Len() {
		return ErrDenseArrayLength
	}
	switch {
	case dst.dtype == DenseArrayF64 && src.dtype == DenseArrayF64:
		min, max, err := denseArrayF64ClampBounds(minValue, maxValue)
		if err != nil {
			return err
		}
		denseArrayClampF64Into(dst.f64, src.f64, min, max)
	case dst.dtype == DenseArrayF64 && src.dtype == DenseArrayI64:
		min, max, err := denseArrayF64ClampBounds(minValue, maxValue)
		if err != nil {
			return err
		}
		denseArrayClampI64ToF64Into(dst.f64, src.i64, min, max)
	case dst.dtype == DenseArrayI64 && src.dtype == DenseArrayI64:
		min, max, err := denseArrayI64ClampBounds(minValue, maxValue)
		if err != nil {
			return err
		}
		denseArrayClampI64Into(dst.i64, src.i64, min, max)
	default:
		return ErrDenseArrayDType
	}
	dst.bumpVersion()
	return nil
}

func denseArrayF64ClampBounds(minValue, maxValue Value) (float64, float64, error) {
	if !minValue.IsNumber() || !maxValue.IsNumber() {
		return 0, 0, fmt.Errorf("dense array f64 clamp bounds must be numeric")
	}
	min, max := minValue.Number(), maxValue.Number()
	if min > max {
		return 0, 0, fmt.Errorf("dense array clamp min must be <= max")
	}
	return min, max, nil
}

func denseArrayI64ClampBounds(minValue, maxValue Value) (int64, int64, error) {
	min, ok := denseArrayClampI64Bound(minValue)
	if !ok {
		return 0, 0, fmt.Errorf("dense array i64 clamp bounds must be integer")
	}
	max, ok := denseArrayClampI64Bound(maxValue)
	if !ok {
		return 0, 0, fmt.Errorf("dense array i64 clamp bounds must be integer")
	}
	if min > max {
		return 0, 0, fmt.Errorf("dense array clamp min must be <= max")
	}
	return min, max, nil
}

func denseArrayClampI64Bound(v Value) (int64, bool) {
	if v.IsInt() {
		return v.Int(), true
	}
	if v.IsFloat() {
		f := v.Float()
		i := int64(f)
		if f == float64(i) {
			return i, true
		}
	}
	return 0, false
}

func denseArrayClampF64Into(dst, src []float64, min, max float64) {
	for i, v := range src {
		if v < min {
			v = min
		} else if v > max {
			v = max
		}
		dst[i] = v
	}
}

func denseArrayClampI64Into(dst, src []int64, min, max int64) {
	for i, v := range src {
		if v < min {
			v = min
		} else if v > max {
			v = max
		}
		dst[i] = v
	}
}

func denseArrayClampI64ToF64Into(dst []float64, src []int64, min, max float64) {
	for i, v := range src {
		x := float64(v)
		if x < min {
			x = min
		} else if x > max {
			x = max
		}
		dst[i] = x
	}
}
