package q

import (
	"fmt"
	"math"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func sinValue(v any) (any, error) {
	return mapNumericUnary("sin", v, func(n float64, _ bool) any {
		return math.Sin(n)
	})
}

func cosValue(v any) (any, error) {
	return mapNumericUnary("cos", v, func(n float64, _ bool) any {
		return math.Cos(n)
	})
}

func tanValue(v any) (any, error) {
	return mapNumericUnary("tan", v, func(n float64, _ bool) any {
		return math.Tan(n)
	})
}

func asinValue(v any) (any, error) {
	return mapNumericUnary("asin", v, func(n float64, _ bool) any {
		return math.Asin(n)
	})
}

func acosValue(v any) (any, error) {
	return mapNumericUnary("acos", v, func(n float64, _ bool) any {
		return math.Acos(n)
	})
}

func atanValue(v any) (any, error) {
	return mapNumericUnary("atan", v, func(n float64, _ bool) any {
		return math.Atan(n)
	})
}

func xexpValue(left, right any) (any, error) {
	return mapNumericDyadicFloat("xexp", left, right, math.Pow)
}

func xlogValue(left, right any) (any, error) {
	return mapNumericDyadicFloat("xlog", left, right, func(base, value float64) float64 {
		return math.Log(value) / math.Log(base)
	})
}

func mapNumericDyadicFloat(name string, left, right any, fn func(float64, float64) float64) (any, error) {
	leftArray, leftIsArray := left.(data.Array)
	rightArray, rightIsArray := right.(data.Array)
	if !leftIsArray && !rightIsArray {
		return mapNumericDyadicFloatScalar(name, left, right, fn)
	}

	n, err := numericDyadicLength(name, leftArray, rightArray)
	if err != nil {
		return nil, err
	}
	out := make([]any, n)
	hasNull := false
	for i := 0; i < n; i++ {
		lv := left
		if leftIsArray {
			row := i
			if leftArray.Len() == 1 {
				row = 0
			}
			var ok bool
			lv, ok = leftArray.At(row)
			if !ok {
				return nil, fmt.Errorf("%s left row %d out of range", name, row)
			}
		}
		rv := right
		if rightIsArray {
			row := i
			if rightArray.Len() == 1 {
				row = 0
			}
			var ok bool
			rv, ok = rightArray.At(row)
			if !ok {
				return nil, fmt.Errorf("%s right row %d out of range", name, row)
			}
		}
		value, err := mapNumericDyadicFloatScalar(name, lv, rv, fn)
		if err != nil {
			return nil, err
		}
		if data.IsNull(value) {
			hasNull = true
			out[i] = data.NullValue
			continue
		}
		out[i] = value
	}
	if hasNull {
		return inferQArray(out, data.KindF64), nil
	}
	xs := make([]float64, n)
	for i, value := range out {
		xs[i], _ = numeric(value)
	}
	return data.NewF64(xs), nil
}

func mapNumericDyadicFloatScalar(name string, left, right any, fn func(float64, float64) float64) (any, error) {
	if data.IsNull(left) || data.IsNull(right) {
		return data.NullValue, nil
	}
	ln, lok := numeric(left)
	rn, rok := numeric(right)
	if !lok || !rok {
		return nil, fmt.Errorf("%s expects numeric operands", name)
	}
	return fn(ln, rn), nil
}

func numericDyadicLength(name string, left, right data.Array) (int, error) {
	switch {
	case left != nil && right != nil:
		switch {
		case left.Len() == right.Len():
			return left.Len(), nil
		case left.Len() == 1:
			return right.Len(), nil
		case right.Len() == 1:
			return left.Len(), nil
		default:
			return 0, fmt.Errorf("%s vector length mismatch", name)
		}
	case left != nil:
		return left.Len(), nil
	case right != nil:
		return right.Len(), nil
	default:
		return 0, nil
	}
}
