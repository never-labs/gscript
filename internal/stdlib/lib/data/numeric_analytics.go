package data

import (
	"fmt"
	"math"
)

// NumericMoments captures reusable numeric aggregate state for language
// frontends and future typed kernels.
type NumericMoments struct {
	Sum        float64
	SumSquares float64
	Count      int64
}

func NumericArrayMoments(array Array) (NumericMoments, bool, error) {
	if array == nil {
		return NumericMoments{}, true, fmt.Errorf("numeric moments array is nil")
	}
	if !isNumericArray(array) {
		return NumericMoments{}, false, nil
	}
	// Range fast path: stream the carrier without materializing. The int64
	// walk and float64 conversion match tryBulkI64Values / NumericAt
	// bit-for-bit, and the accumulators update in the same row order.
	if r, ok := statsStreamI64Range(array); ok {
		var out NumericMoments
		value := r.start
		for i := 0; i < r.len; i++ {
			v := float64(value)
			out.Sum += v
			out.SumSquares += v * v
			value += r.step
		}
		out.Count = int64(r.len)
		return out, true, nil
	}
	// Bulk fast path: flatten once and accumulate over the dense slice with
	// the exact per-row updates of the NumericAt loop below (same element
	// values, same order), so results are bit-identical. tryBulkF64Values
	// bails on null rows, which keeps null-skipping on the boxed loop.
	if values, owned, ok := tryBulkF64Values(array); ok {
		var out NumericMoments
		for _, value := range values {
			out.Sum += value
			out.SumSquares += value * value
		}
		out.Count = int64(len(values))
		bulkF64Release(values, owned)
		return out, true, nil
	}
	var out NumericMoments
	for row := 0; row < array.Len(); row++ {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return NumericMoments{}, true, err
		}
		if !ok {
			continue
		}
		out.Sum += value
		out.SumSquares += value * value
		out.Count++
	}
	return out, true, nil
}

func NumericVarianceFromMoments(m NumericMoments, sample bool) (float64, bool) {
	if m.Count == 0 || (sample && m.Count < 2) {
		return 0, false
	}
	denom := float64(m.Count)
	if sample {
		denom = float64(m.Count - 1)
	}
	mean := m.Sum / float64(m.Count)
	variance := (m.SumSquares - float64(m.Count)*mean*mean) / denom
	if variance < 0 && variance > -1e-12 {
		variance = 0
	}
	return variance, true
}

func NumericArrayVariance(array Array, sample bool) (any, bool, error) {
	moments, handled, err := NumericArrayMoments(array)
	if err != nil || !handled {
		return nil, handled, err
	}
	variance, ok := NumericVarianceFromMoments(moments, sample)
	if !ok {
		return NullValue, true, nil
	}
	return variance, true, nil
}

func NumericArrayStdDev(array Array, sample bool) (any, bool, error) {
	variance, handled, err := NumericArrayVariance(array, sample)
	if err != nil || !handled || IsNull(variance) {
		return variance, handled, err
	}
	return math.Sqrt(variance.(float64)), true, nil
}

func NumericWeightedSum(weights, values any) (any, bool, error) {
	weightArray, weightIsArray := weights.(Array)
	valueArray, valueIsArray := values.(Array)
	if !weightIsArray && !valueIsArray {
		if IsNull(weights) || IsNull(values) {
			return NullValue, true, nil
		}
		w, wok := numeric(weights)
		v, vok := numeric(values)
		if !wok || !vok {
			return nil, false, nil
		}
		return w * v, true, nil
	}
	if weightIsArray && !isNumericArray(weightArray) {
		return nil, false, nil
	}
	if valueIsArray && !isNumericArray(valueArray) {
		return nil, false, nil
	}
	weightScalar, weightScalarNull, ok := numericWeightedScalar(weights, !weightIsArray)
	if !ok {
		return nil, false, nil
	}
	valueScalar, valueScalarNull, ok := numericWeightedScalar(values, !valueIsArray)
	if !ok {
		return nil, false, nil
	}
	length := 1
	if weightIsArray {
		length = weightArray.Len()
	}
	if valueIsArray {
		if weightIsArray && valueArray.Len() != length {
			return nil, true, fmt.Errorf("weighted sum length mismatch")
		}
		length = valueArray.Len()
	}
	// Range fast path for the array-array case: stream both carriers without
	// materializing; identical element values and accumulation order.
	if weightIsArray && valueIsArray {
		if wr, ok := statsStreamI64Range(weightArray); ok {
			if vr, ok := statsStreamI64Range(valueArray); ok && vr.len == wr.len {
				total := float64(0)
				wv, vv := wr.start, vr.start
				for i := 0; i < wr.len; i++ {
					total += float64(wv) * float64(vv)
					wv += wr.step
					vv += vr.step
				}
				return total, true, nil
			}
		}
	}
	// Bulk fast path for the array-array case: same per-row product and
	// accumulation order as the boxed loop below, over slices that flatten
	// to the exact NumericAt element values (nulls bail to the boxed loop).
	if weightIsArray && valueIsArray {
		if wvs, wOwned, ok := tryBulkF64Values(weightArray); ok {
			if vvs, vOwned, ok := tryBulkF64Values(valueArray); ok && len(vvs) == len(wvs) {
				total := float64(0)
				for i, w := range wvs {
					total += w * vvs[i]
				}
				bulkF64Release(vvs, vOwned)
				bulkF64Release(wvs, wOwned)
				return total, true, nil
			} else if ok {
				bulkF64Release(vvs, vOwned)
			}
			bulkF64Release(wvs, wOwned)
		}
	}
	total := float64(0)
	for row := 0; row < length; row++ {
		weight := weightScalar
		weightOK := !weightScalarNull
		if weightIsArray {
			var err error
			weight, weightOK, err = typedKernels.NumericAt(weightArray, row)
			if err != nil {
				return nil, true, err
			}
		}
		value := valueScalar
		valueOK := !valueScalarNull
		if valueIsArray {
			var err error
			value, valueOK, err = typedKernels.NumericAt(valueArray, row)
			if err != nil {
				return nil, true, err
			}
		}
		if !weightOK || !valueOK {
			continue
		}
		total += weight * value
	}
	return total, true, nil
}

func numericWeightedScalar(value any, required bool) (float64, bool, bool) {
	if !required {
		return 0, false, true
	}
	if IsNull(value) {
		return 0, true, true
	}
	n, ok := numeric(value)
	return n, false, ok
}

func NumericArrayCovariance(left, right Array, sample bool) (any, bool, error) {
	if left == nil || right == nil {
		return nil, true, fmt.Errorf("covariance expects two arrays")
	}
	if left.Len() != right.Len() {
		return nil, true, fmt.Errorf("covariance length mismatch")
	}
	if !isNumericArray(left) || !isNumericArray(right) {
		return nil, false, nil
	}
	var sx, sy, sxy float64
	var count int64
	// Range fast path: stream both carriers without materializing; identical
	// element values and accumulation order.
	if lr, ok := statsStreamI64Range(left); ok {
		if rr, ok := statsStreamI64Range(right); ok && rr.len == lr.len {
			lvalue, rvalue := lr.start, rr.start
			for i := 0; i < lr.len; i++ {
				lv := float64(lvalue)
				rv := float64(rvalue)
				sx += lv
				sy += rv
				sxy += lv * rv
				lvalue += lr.step
				rvalue += rr.step
			}
			return numericCovarianceFromSums(sx, sy, sxy, int64(lr.len), sample)
		}
	}
	// Bulk fast path: identical accumulators in identical row order over the
	// flattened slices (nulls bail to the boxed loop below).
	if lvs, lOwned, ok := tryBulkF64Values(left); ok {
		if rvs, rOwned, ok := tryBulkF64Values(right); ok && len(rvs) == len(lvs) {
			for i, lv := range lvs {
				rv := rvs[i]
				sx += lv
				sy += rv
				sxy += lv * rv
			}
			count = int64(len(lvs))
			bulkF64Release(rvs, rOwned)
			bulkF64Release(lvs, lOwned)
			return numericCovarianceFromSums(sx, sy, sxy, count, sample)
		} else if ok {
			bulkF64Release(rvs, rOwned)
		}
		bulkF64Release(lvs, lOwned)
	}
	for row := 0; row < left.Len(); row++ {
		lv, lok, err := typedKernels.NumericAt(left, row)
		if err != nil {
			return nil, true, err
		}
		rv, rok, err := typedKernels.NumericAt(right, row)
		if err != nil {
			return nil, true, err
		}
		if !lok || !rok {
			continue
		}
		sx += lv
		sy += rv
		sxy += lv * rv
		count++
	}
	return numericCovarianceFromSums(sx, sy, sxy, count, sample)
}

func numericCovarianceFromSums(sx, sy, sxy float64, count int64, sample bool) (any, bool, error) {
	if count == 0 || (sample && count < 2) {
		return NullValue, true, nil
	}
	denom := float64(count)
	if sample {
		denom = float64(count - 1)
	}
	cov := (sxy - sx*sy/float64(count)) / denom
	if cov == 0 {
		cov = 0
	}
	return cov, true, nil
}

func NumericArrayCorrelation(left, right Array) (any, bool, error) {
	if left == nil || right == nil {
		return nil, true, fmt.Errorf("correlation expects two arrays")
	}
	if left.Len() != right.Len() {
		return nil, true, fmt.Errorf("correlation length mismatch")
	}
	if !isNumericArray(left) || !isNumericArray(right) {
		return nil, false, nil
	}
	var sx, sy, sx2, sy2, sxy float64
	var count int64
	// Range fast path: stream both carriers without materializing; identical
	// element values and accumulation order.
	if lr, ok := statsStreamI64Range(left); ok {
		if rr, ok := statsStreamI64Range(right); ok && rr.len == lr.len {
			lvalue, rvalue := lr.start, rr.start
			for i := 0; i < lr.len; i++ {
				lv := float64(lvalue)
				rv := float64(rvalue)
				sx += lv
				sy += rv
				sx2 += lv * lv
				sy2 += rv * rv
				sxy += lv * rv
				lvalue += lr.step
				rvalue += rr.step
			}
			return numericCorrelationFromSums(sx, sy, sx2, sy2, sxy, int64(lr.len))
		}
	}
	// Bulk fast path: identical accumulators in identical row order over the
	// flattened slices (nulls bail to the boxed loop below).
	if lvs, lOwned, ok := tryBulkF64Values(left); ok {
		if rvs, rOwned, ok := tryBulkF64Values(right); ok && len(rvs) == len(lvs) {
			for i, lv := range lvs {
				rv := rvs[i]
				sx += lv
				sy += rv
				sx2 += lv * lv
				sy2 += rv * rv
				sxy += lv * rv
			}
			count = int64(len(lvs))
			bulkF64Release(rvs, rOwned)
			bulkF64Release(lvs, lOwned)
			return numericCorrelationFromSums(sx, sy, sx2, sy2, sxy, count)
		} else if ok {
			bulkF64Release(rvs, rOwned)
		}
		bulkF64Release(lvs, lOwned)
	}
	for row := 0; row < left.Len(); row++ {
		lv, lok, err := typedKernels.NumericAt(left, row)
		if err != nil {
			return nil, true, err
		}
		rv, rok, err := typedKernels.NumericAt(right, row)
		if err != nil {
			return nil, true, err
		}
		if !lok || !rok {
			continue
		}
		sx += lv
		sy += rv
		sx2 += lv * lv
		sy2 += rv * rv
		sxy += lv * rv
		count++
	}
	return numericCorrelationFromSums(sx, sy, sx2, sy2, sxy, count)
}

func numericCorrelationFromSums(sx, sy, sx2, sy2, sxy float64, count int64) (any, bool, error) {
	if count < 2 {
		return NullValue, true, nil
	}
	covNumerator := sxy - sx*sy/float64(count)
	leftNumerator := sx2 - sx*sx/float64(count)
	rightNumerator := sy2 - sy*sy/float64(count)
	denom := math.Sqrt(leftNumerator * rightNumerator)
	if denom == 0 {
		return NullValue, true, nil
	}
	return covNumerator / denom, true, nil
}

func NumericMovingStdDev(array Array, width int, sample bool) (Array, bool, error) {
	if width <= 0 {
		return nil, true, fmt.Errorf("moving stddev width must be positive")
	}
	if !isNumericArray(array) {
		return nil, false, nil
	}
	out := make([]float64, array.Len())
	var nullable []any
	var moments NumericMoments
	for row := 0; row < array.Len(); row++ {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if ok {
			moments.Sum += value
			moments.SumSquares += value * value
			moments.Count++
		}
		if remove := row - width; remove >= 0 {
			value, ok, err := typedKernels.NumericAt(array, remove)
			if err != nil {
				return nil, true, err
			}
			if ok {
				moments.Sum -= value
				moments.SumSquares -= value * value
				moments.Count--
			}
		}
		variance, ok := NumericVarianceFromMoments(moments, sample)
		if !ok {
			nullable = ensureNumericNullableOutput(nullable, out, row)
			nullable[row] = NullValue
		} else {
			value := math.Sqrt(variance)
			out[row] = value
			if nullable != nil {
				nullable[row] = value
			}
		}
	}
	if nullable != nil {
		return NewColumn("_", nullable).Data, true, nil
	}
	return newF64Trusted(out), true, nil
}

func NumericExponentialMovingAverage(array Array, alpha float64) (Array, bool, error) {
	if alpha < 0 || alpha > 1 {
		return nil, true, fmt.Errorf("ema alpha must be in range 0..1")
	}
	if !isNumericArray(array) {
		return nil, false, nil
	}
	out := make([]float64, array.Len())
	var nullable []any
	var prev float64
	hasPrev := false
	for row := 0; row < array.Len(); row++ {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			nullable = ensureNumericNullableOutput(nullable, out, row)
			nullable[row] = NullValue
			continue
		}
		if !hasPrev {
			prev = value
			hasPrev = true
		} else {
			prev = alpha*value + (1-alpha)*prev
		}
		out[row] = prev
		if nullable != nil {
			nullable[row] = prev
		}
	}
	if nullable != nil {
		return NewColumn("_", nullable).Data, true, nil
	}
	return newF64Trusted(out), true, nil
}

// I64RangeView reports the affine parameters of an integer range carrier,
// letting frontend reducers stream or close-form ranges without
// materializing them.
func I64RangeView(array Array) (start, step int64, n int, ok bool) {
	if r, isRange := unwrapAttributedArray(array).(i64RangeArray); isRange {
		return r.start, r.step, r.len, true
	}
	return 0, 0, 0, false
}

// numericStatsI64Range unwraps attribute wrappers and reports the underlying
// i64RangeArray, letting stats kernels stream range carriers without
// materializing them.
func statsStreamI64Range(array Array) (i64RangeArray, bool) {
	if r, ok := unwrapAttributedArray(array).(i64RangeArray); ok {
		return r, true
	}
	return i64RangeArray{}, false
}

func ensureNumericNullableOutput(nullable []any, values []float64, filled int) []any {
	if nullable != nil {
		return nullable
	}
	nullable = make([]any, len(values))
	for i := 0; i < filled; i++ {
		nullable[i] = values[i]
	}
	return nullable
}
