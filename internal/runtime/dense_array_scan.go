package runtime

// DenseArrayScan returns the prefix sum scan of a numeric dense array.
func DenseArrayScan(src *DenseArray) (*DenseArray, error) {
	return denseArrayScan(src)
}

func denseArrayScan(src *DenseArray) (*DenseArray, error) {
	if src == nil {
		return nil, ErrDenseArrayOperand
	}
	switch src.dtype {
	case DenseArrayF64:
		out := make([]float64, len(src.f64))
		denseArrayScanF64Into(out, src.f64)
		return &DenseArray{dtype: DenseArrayF64, f64: out}, nil
	case DenseArrayI64:
		out := make([]int64, len(src.i64))
		denseArrayScanI64Into(out, src.i64)
		return &DenseArray{dtype: DenseArrayI64, i64: out}, nil
	default:
		return nil, ErrDenseArrayDType
	}
}

func denseArrayScanInto(dst, src *DenseArray) error {
	if dst == nil || src == nil {
		return ErrDenseArrayOperand
	}
	if dst.Len() != src.Len() {
		return ErrDenseArrayLength
	}
	switch {
	case dst.dtype == DenseArrayF64 && src.dtype == DenseArrayF64:
		denseArrayScanF64Into(dst.f64, src.f64)
	case dst.dtype == DenseArrayF64 && src.dtype == DenseArrayI64:
		denseArrayScanI64ToF64Into(dst.f64, src.i64)
	case dst.dtype == DenseArrayI64 && src.dtype == DenseArrayI64:
		denseArrayScanI64Into(dst.i64, src.i64)
	default:
		return ErrDenseArrayDType
	}
	dst.bumpVersion()
	return nil
}

func denseArrayScanF64Into(dst, src []float64) {
	sum := 0.0
	for i, v := range src {
		sum += v
		dst[i] = sum
	}
}

func denseArrayScanI64Into(dst, src []int64) {
	var sum int64
	for i, v := range src {
		sum += v
		dst[i] = sum
	}
}

func denseArrayScanI64ToF64Into(dst []float64, src []int64) {
	sum := 0.0
	for i, v := range src {
		sum += float64(v)
		dst[i] = sum
	}
}
