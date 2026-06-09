package q

import (
	"fmt"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func sinValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnarySin, v)
}

func cosValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryCos, v)
}

func tanValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryTan, v)
}

func asinValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryAsin, v)
}

func acosValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryAcos, v)
}

func atanValue(v any) (any, error) {
	return qDataNumericUnary(data.NumericUnaryAtan, v)
}

func xexpValue(left, right any) (any, error) {
	return qDataNumericDyadicFloat(data.NumericDyadicXExp, left, right)
}

func xlogValue(left, right any) (any, error) {
	return qDataNumericDyadicFloat(data.NumericDyadicXLog, left, right)
}

func qDataNumericUnary(name string, value any) (any, error) {
	if array, ok := value.(data.Array); ok {
		shape := "vector-unary/" + name + "/" + string(array.Kind())
		typed, handled, err := data.TryTypedQNumericUnary(name, array)
		typed, handled, err = qTypedRuntimeResult("ArrayNumericUnary", shape, typed, handled, err)
		if err != nil || handled {
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return typed, nil
		}
	}
	out, ok, err := data.ApplyNumericUnaryValue(name, value)
	if err != nil {
		return nil, err
	}
	if ok {
		return out, nil
	}
	return nil, fmt.Errorf("%s expects a numeric value or vector", name)
}

func qDataNumericDyadicFloat(name string, left, right any) (any, error) {
	out, ok, err := data.ApplyNumericDyadicFloat(name, left, right)
	if err != nil {
		return nil, err
	}
	if ok {
		return out, nil
	}
	return nil, fmt.Errorf("%s expects numeric operands", name)
}
