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
		shape := qRuntimeUnaryPrimitiveShape(name)
		typed, handled, err := data.TryTypedQNumericUnary(name, array)
		typed, handled, err = qTypedRuntimeResult("ArrayNumericUnary", shape, typed, handled, err)
		if err != nil || handled {
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return typed, nil
		}
		return qDataNumericUnaryArrayFallback(name, array)
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

func qRuntimeUnaryPrimitiveShape(name string) string {
	if name == "" {
		name = "unknown"
	}
	return "runtime-unary/" + name
}

func qDataNumericUnaryArrayFallback(name string, array data.Array) (any, error) {
	out := make([]any, array.Len())
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			out[i] = data.NullValue
			continue
		}
		value, ok, err := data.ApplyNumericUnaryValue(name, item)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%s expects a numeric value or vector", name)
		}
		out[i] = value
	}
	return data.InferArray(out), nil
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
