package q

import "github.com/never-labs/leia/internal/stdlib/lib/data"

// Typed adverb routing.
//
// Each-prior, over, and scan adverbs historically walked their vectors one
// boxed pair at a time through applyDyadic / applyCallable. When the verb is
// a plain arithmetic dyadic (native `+ - * %`, or a two-parameter lambda with
// a fast dyadic plan such as {x+y}), the whole adverb application is a single
// typed columnar kernel; route it onto the data-package adverb kernels and
// keep the boxed loop as the semantic fallback for every other shape.

// qTypedAdverbOp maps a dyadic verb op byte onto the arithmetic data.Op set
// the typed adverb kernels understand.
func qTypedAdverbOp(op byte) (data.Op, bool) {
	switch op {
	case '+', '-', '*', '%':
		return qDataArithmeticOp(op)
	default:
		return "", false
	}
}

// callableFastDyadicOp recognizes callables whose application is exactly one
// scalar dyadic verb: native verb functions and two-parameter lambdas with a
// fast dyadic plan ({x+y}, {x-y}, ...).
func callableFastDyadicOp(fn any) (byte, bool) {
	switch f := fn.(type) {
	case qDyadicFunction:
		if op, _, ok := lookupDyadicVerb(f.name); ok {
			return op, true
		}
	case qLambda:
		if plan, ok := qLambdaFastPlanFor(f.body); ok && plan.kind == qLambdaFastDyadic {
			return plan.op, true
		}
	}
	return 0, false
}

func qAdverbKernelShape(adverb string, op byte, initial, v any) string {
	shape := "adverb-" + qAdverbShapeName(adverb) + "/" + string(op) + "/" + string(qRuntimeKernelOperandKind(v, nil))
	if initial != nil {
		shape += "/seeded"
	}
	return shape
}

func tryApplyTypedEachPrior(op byte, initial any, v any) (any, bool, error) {
	dataOp, ok := qTypedAdverbOp(op)
	if !ok {
		return nil, false, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, false, nil
	}
	out, handled, err := data.TryTypedEachPriorDyadic(dataOp, initial, array)
	if handled || err != nil {
		recordRuntimeKernelProbe("ArrayAdverbEachPrior", qAdverbKernelShape("':", op, initial, v), handled, err)
		return out, handled, err
	}
	return nil, false, nil
}

func tryApplyTypedOverScan(op byte, initial any, v any, scan bool) (any, bool, error) {
	dataOp, ok := qTypedAdverbOp(op)
	if !ok {
		return nil, false, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, false, nil
	}
	adverb := "/"
	kernel := "ArrayAdverbOver"
	var out any
	var handled bool
	var err error
	if scan {
		adverb = "\\"
		kernel = "ArrayAdverbScan"
		out, handled, err = data.TryTypedScanDyadic(dataOp, initial, array)
	} else {
		out, handled, err = data.TryTypedOverDyadic(dataOp, initial, array)
	}
	if handled || err != nil {
		recordRuntimeKernelProbe(kernel, qAdverbKernelShape(adverb, op, initial, v), handled, err)
		return out, handled, err
	}
	return nil, false, nil
}
