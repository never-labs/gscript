package runtime

var denseArrayStatsCtor = NewSmallTableCtorN([]string{"count", "sum", "min", "max", "mean"})

func (a *DenseArray) StatsValue() (Value, error) {
	if a == nil {
		return NilValue(), ErrDenseArrayOperand
	}
	if a.Len() == 0 {
		return NilValue(), ErrDenseArrayEmpty
	}
	switch a.dtype {
	case DenseArrayF64:
		sum, min, max := denseArrayStatsF64(a.f64)
		if v, ok := NewFixedRecordValue5KnownCtor(
			&denseArrayStatsCtor,
			IntValue(int64(len(a.f64))),
			FloatValue(sum),
			FloatValue(min),
			FloatValue(max),
			FloatValue(sum/float64(len(a.f64))),
		); ok {
			return v, nil
		}
	case DenseArrayI64:
		sum, min, max := denseArrayStatsI64(a.i64)
		if v, ok := NewFixedRecordValue5KnownCtor(
			&denseArrayStatsCtor,
			IntValue(int64(len(a.i64))),
			IntValue(sum),
			IntValue(min),
			IntValue(max),
			FloatValue(float64(sum)/float64(len(a.i64))),
		); ok {
			return v, nil
		}
	case DenseArrayBool:
		return NilValue(), ErrDenseArrayDType
	}
	return NilValue(), ErrDenseArrayDType
}

func denseArrayStatsF64(xs []float64) (sum, min, max float64) {
	n := len(xs)
	if n == 0 {
		return 0, 0, 0
	}
	_, _ = xs[n-1], xs[0]
	sum0, sum1, sum2, sum3 := 0.0, 0.0, 0.0, 0.0
	min0, min1, min2, min3 := xs[0], xs[0], xs[0], xs[0]
	max0, max1, max2, max3 := xs[0], xs[0], xs[0], xs[0]
	i := 0
	limit := n - n%4
	for ; i < limit; i += 4 {
		v0, v1, v2, v3 := xs[i], xs[i+1], xs[i+2], xs[i+3]
		sum0 += v0
		sum1 += v1
		sum2 += v2
		sum3 += v3
		if v0 < min0 {
			min0 = v0
		}
		if v1 < min1 {
			min1 = v1
		}
		if v2 < min2 {
			min2 = v2
		}
		if v3 < min3 {
			min3 = v3
		}
		if v0 > max0 {
			max0 = v0
		}
		if v1 > max1 {
			max1 = v1
		}
		if v2 > max2 {
			max2 = v2
		}
		if v3 > max3 {
			max3 = v3
		}
	}
	sum = sum0 + sum1 + sum2 + sum3
	min = min0
	if min1 < min {
		min = min1
	}
	if min2 < min {
		min = min2
	}
	if min3 < min {
		min = min3
	}
	max = max0
	if max1 > max {
		max = max1
	}
	if max2 > max {
		max = max2
	}
	if max3 > max {
		max = max3
	}
	for ; i < n; i++ {
		v := xs[i]
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return sum, min, max
}

func denseArrayStatsI64(xs []int64) (sum, min, max int64) {
	n := len(xs)
	if n == 0 {
		return 0, 0, 0
	}
	_, _ = xs[n-1], xs[0]
	var sum0, sum1, sum2, sum3 int64
	min0, min1, min2, min3 := xs[0], xs[0], xs[0], xs[0]
	max0, max1, max2, max3 := xs[0], xs[0], xs[0], xs[0]
	i := 0
	limit := n - n%4
	for ; i < limit; i += 4 {
		v0, v1, v2, v3 := xs[i], xs[i+1], xs[i+2], xs[i+3]
		sum0 += v0
		sum1 += v1
		sum2 += v2
		sum3 += v3
		if v0 < min0 {
			min0 = v0
		}
		if v1 < min1 {
			min1 = v1
		}
		if v2 < min2 {
			min2 = v2
		}
		if v3 < min3 {
			min3 = v3
		}
		if v0 > max0 {
			max0 = v0
		}
		if v1 > max1 {
			max1 = v1
		}
		if v2 > max2 {
			max2 = v2
		}
		if v3 > max3 {
			max3 = v3
		}
	}
	sum = sum0 + sum1 + sum2 + sum3
	min = min0
	if min1 < min {
		min = min1
	}
	if min2 < min {
		min = min2
	}
	if min3 < min {
		min = min3
	}
	max = max0
	if max1 > max {
		max = max1
	}
	if max2 > max {
		max = max2
	}
	if max3 > max {
		max = max3
	}
	for ; i < n; i++ {
		v := xs[i]
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return sum, min, max
}
