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
	total := float64(0)
	for row := 0; row < length; row++ {
		weight := weights
		if weightIsArray {
			var ok bool
			weight, ok = weightArray.At(row)
			if !ok {
				return nil, true, fmt.Errorf("weighted sum weight row %d out of range", row)
			}
		}
		value := values
		if valueIsArray {
			var ok bool
			value, ok = valueArray.At(row)
			if !ok {
				return nil, true, fmt.Errorf("weighted sum value row %d out of range", row)
			}
		}
		if IsNull(weight) || IsNull(value) {
			continue
		}
		w, wok := numeric(weight)
		v, vok := numeric(value)
		if !wok || !vok {
			return nil, false, nil
		}
		total += w * v
	}
	return total, true, nil
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
	out := make([]any, array.Len())
	for row := 0; row < array.Len(); row++ {
		start := row - width + 1
		if start < 0 {
			start = 0
		}
		var moments NumericMoments
		for i := start; i <= row; i++ {
			value, ok, err := typedKernels.NumericAt(array, i)
			if err != nil {
				return nil, true, err
			}
			if !ok {
				continue
			}
			moments.Sum += value
			moments.SumSquares += value * value
			moments.Count++
		}
		variance, ok := NumericVarianceFromMoments(moments, sample)
		if !ok {
			out[row] = NullValue
		} else {
			out[row] = math.Sqrt(variance)
		}
	}
	return NewColumn("_", out).Data, true, nil
}

func NumericExponentialMovingAverage(array Array, alpha float64) (Array, bool, error) {
	if alpha < 0 || alpha > 1 {
		return nil, true, fmt.Errorf("ema alpha must be in range 0..1")
	}
	if !isNumericArray(array) {
		return nil, false, nil
	}
	out := make([]any, array.Len())
	var prev float64
	hasPrev := false
	for row := 0; row < array.Len(); row++ {
		value, ok, err := typedKernels.NumericAt(array, row)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			out[row] = NullValue
			continue
		}
		if !hasPrev {
			prev = value
			hasPrev = true
		} else {
			prev = alpha*value + (1-alpha)*prev
		}
		out[row] = prev
	}
	return NewColumn("_", out).Data, true, nil
}
