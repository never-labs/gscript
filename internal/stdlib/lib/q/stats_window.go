package q

import (
	"fmt"
	"math"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func svarValue(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return data.NullValue, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("svar expects a numeric vector")
	}
	out, handled, err := data.NumericArrayVariance(array, true)
	if err != nil || handled {
		return out, err
	}
	return nil, fmt.Errorf("svar expects a numeric vector")
}

func sdevValue(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return data.NullValue, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("sdev expects a numeric vector")
	}
	out, handled, err := data.NumericArrayStdDev(array, true)
	if err != nil || handled {
		return out, err
	}
	return nil, fmt.Errorf("sdev expects a numeric vector")
}

func wsumValue(weights, values any) (any, error) {
	out, handled, err := data.NumericWeightedSum(weights, values)
	if err != nil || handled {
		return out, err
	}
	return nil, fmt.Errorf("wsum expects numeric weights and values")
}

func wsumUnaryValue(v any) (any, error) {
	return sum(v)
}

func covValue(left, right any) (any, error) {
	return covarianceValue("cov", left, right, false)
}

func scovValue(left, right any) (any, error) {
	return covarianceValue("scov", left, right, true)
}

func covarianceValue(name string, left, right any, sample bool) (any, error) {
	leftArray, leftOK := left.(data.Array)
	rightArray, rightOK := right.(data.Array)
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("%s expects numeric vectors", name)
	}
	out, handled, err := data.NumericArrayCovariance(leftArray, rightArray, sample)
	if err != nil || handled {
		return out, err
	}
	return nil, fmt.Errorf("%s expects numeric vectors", name)
}

func corValue(left, right any) (any, error) {
	leftArray, leftOK := left.(data.Array)
	rightArray, rightOK := right.(data.Array)
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("cor expects numeric vectors")
	}
	out, handled, err := data.NumericArrayCorrelation(leftArray, rightArray)
	if err != nil || handled {
		return out, err
	}
	return nil, fmt.Errorf("cor expects numeric vectors")
}

func mdevValue(width any, v any) (any, error) {
	n, ok := integerValue(width)
	if !ok || n <= 0 || int64(int(n)) != n {
		return nil, fmt.Errorf("mdev width must be a positive integer")
	}
	if data.IsNull(v) {
		return data.NullValue, nil
	}
	if _, ok := numeric(v); ok {
		return 0.0, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("mdev expects a numeric vector")
	}
	out, handled, err := data.NumericMovingStdDev(array, int(n), false)
	if err != nil || handled {
		return out, err
	}
	return nil, fmt.Errorf("mdev expects a numeric vector")
}

func emaValue(alpha any, v any) (any, error) {
	a, ok := numeric(alpha)
	if !ok {
		return nil, fmt.Errorf("ema alpha must be numeric")
	}
	if math.IsNaN(a) || math.IsInf(a, 0) {
		return nil, fmt.Errorf("ema alpha must be finite")
	}
	if data.IsNull(v) {
		return data.NullValue, nil
	}
	if n, ok := numeric(v); ok {
		return n, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("ema expects a numeric vector")
	}
	out, handled, err := data.NumericExponentialMovingAverage(array, a)
	if err != nil || handled {
		return out, err
	}
	return nil, fmt.Errorf("ema expects a numeric vector")
}
