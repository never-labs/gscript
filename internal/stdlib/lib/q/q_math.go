package q

import (
	"fmt"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qNumericUnaryVerbDescriptor struct {
	op string
}

type qNumericDyadicFloatVerbDescriptor struct {
	op string
}

var qNumericUnaryVerbDescriptors = map[string]qNumericUnaryVerbDescriptor{
	"abs":        {op: data.NumericUnaryAbs},
	"sqrt":       {op: data.NumericUnarySqrt},
	"log":        {op: data.NumericUnaryLog},
	"exp":        {op: data.NumericUnaryExp},
	"sin":        {op: data.NumericUnarySin},
	"cos":        {op: data.NumericUnaryCos},
	"tan":        {op: data.NumericUnaryTan},
	"asin":       {op: data.NumericUnaryAsin},
	"acos":       {op: data.NumericUnaryAcos},
	"atan":       {op: data.NumericUnaryAtan},
	"reciprocal": {op: data.NumericUnaryRecip},
	"signum":     {op: data.NumericUnarySignum},
	"floor":      {op: data.NumericUnaryFloor},
	"ceiling":    {op: data.NumericUnaryCeiling},
}

var qNumericDyadicFloatVerbDescriptors = map[string]qNumericDyadicFloatVerbDescriptor{
	"xexp": {op: data.NumericDyadicXExp},
	"xlog": {op: data.NumericDyadicXLog},
}

func lookupNumericUnaryVerb(verb string) (func(any) (any, error), bool) {
	desc, ok := qNumericUnaryVerbDescriptors[verb]
	if !ok {
		return nil, false
	}
	return desc.apply, true
}

func lookupNumericDyadicFloatVerb(verb string) (func(any, any) (any, error), bool) {
	desc, ok := qNumericDyadicFloatVerbDescriptors[verb]
	if !ok {
		return nil, false
	}
	return desc.apply, true
}

func (d qNumericUnaryVerbDescriptor) apply(v any) (any, error) {
	return qDataNumericUnary(d.op, v)
}

func (d qNumericDyadicFloatVerbDescriptor) apply(left, right any) (any, error) {
	return qDataNumericDyadicFloat(d.op, left, right)
}

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
	if _, leftIsArray := left.(data.Array); leftIsArray {
		if out, handled, err := qTypedNumericDyadicFloat(name, left, right); err != nil || handled {
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return out, nil
		}
	} else if _, rightIsArray := right.(data.Array); rightIsArray {
		if out, handled, err := qTypedNumericDyadicFloat(name, left, right); err != nil || handled {
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return out, nil
		}
	}
	out, ok, err := data.ApplyNumericDyadicFloat(name, left, right)
	if err != nil {
		return nil, err
	}
	if ok {
		return out, nil
	}
	return nil, fmt.Errorf("%s expects numeric operands", name)
}

func qTypedNumericDyadicFloat(name string, left, right any) (data.Array, bool, error) {
	shape := qRuntimeDyadicPrimitiveShape(name)
	typed, handled, err := data.TryTypedQNumericDyadicFloat(name, left, right)
	return qTypedRuntimeResult("ArrayNumericDyadicFloat", shape, typed, handled, err)
}

func qRuntimeDyadicPrimitiveShape(name string) string {
	if name == "" {
		name = "unknown"
	}
	return "runtime-dyadic/" + name
}
