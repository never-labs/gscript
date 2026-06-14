package q

import "github.com/never-labs/leia/internal/stdlib/lib/data"

func recordQTypedRuntimeKernel(kernel, shape string, handled bool, err error) {
	recordQTypedRuntimeKernelReason(kernel, shape, handled, err, RuntimeFallbackUnsupportedShape)
}

func recordQTypedRuntimeKernelReason(kernel, shape string, handled bool, err error, fallbackReason string) {
	if fallbackReason == "" {
		fallbackReason = RuntimeFallbackUnsupportedShape
	}
	recordRuntimeKernelProbeReason(kernel, shape, handled, err, fallbackReason)
}

func qTypedRuntimeResult[T any](kernel, shape string, out T, handled bool, err error) (T, bool, error) {
	recordQTypedRuntimeKernel(kernel, shape, handled, err)
	return out, handled, err
}

func qTypedRuntimeResultReason[T any](kernel, shape, fallbackReason string, out T, handled bool, err error) (T, bool, error) {
	recordQTypedRuntimeKernelReason(kernel, shape, handled, err, fallbackReason)
	return out, handled, err
}

func qTypedRuntimeResult2[A, B any](kernel, shape string, a A, b B, handled bool, err error) (A, B, bool, error) {
	recordQTypedRuntimeKernel(kernel, shape, handled, err)
	return a, b, handled, err
}

func qTypedRuntimeResult2Reason[A, B any](kernel, shape, fallbackReason string, a A, b B, handled bool, err error) (A, B, bool, error) {
	recordQTypedRuntimeKernelReason(kernel, shape, handled, err, fallbackReason)
	return a, b, handled, err
}

type qTypedRuntimeKernel[T any] struct {
	kernel         string
	shape          string
	fallbackReason string
	call           func() (T, bool, error)
}

func evalQTypedRuntimeKernel[T any](kernel qTypedRuntimeKernel[T]) (T, bool, error) {
	out, handled, err := kernel.call()
	fallbackReason := kernel.fallbackReason
	if fallbackReason == "" {
		fallbackReason = RuntimeFallbackUnsupportedShape
	}
	recordRuntimeKernelProbeReason(kernel.kernel, kernel.shape, handled, err, fallbackReason)
	return out, handled, err
}

func evalQTypedRuntimeArrayInt64(kernel, shapePrefix string, array data.Array, call func(data.Array) (int64, bool, error)) (int64, bool, error) {
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: kernel,
		shape:  shapePrefix + "/" + string(array.Kind()),
		call: func() (int64, bool, error) {
			return call(array)
		},
	})
}

type qTypedRuntimeKernel2[A, B any] struct {
	kernel         string
	shape          string
	fallbackReason string
	call           func() (A, B, bool, error)
}

func evalQTypedRuntimeKernel2[A, B any](kernel qTypedRuntimeKernel2[A, B]) (A, B, bool, error) {
	a, b, handled, err := kernel.call()
	fallbackReason := kernel.fallbackReason
	if fallbackReason == "" {
		fallbackReason = RuntimeFallbackUnsupportedShape
	}
	recordRuntimeKernelProbeReason(kernel.kernel, kernel.shape, handled, err, fallbackReason)
	return a, b, handled, err
}

func qTryTypedRuntimeVectorCompareDyadic(op string, dataOp data.Op, left, right any, la, ra data.Array, recordUnsupportedProbe bool) (any, bool, error) {
	if la == nil && ra == nil {
		return nil, false, nil
	}
	shape := qRuntimeKernelCompositeVectorDyadicShape(op, left, right, la, ra)
	if !qVectorDyadicCanUseTypedCompare(left, right, la, ra) {
		if recordUnsupportedProbe {
			recordRuntimeKernelExecution("ArrayDyadicCompare", shape, "attempt", "attempt")
			recordRuntimeKernelExecution("ArrayDyadicCompare", shape, "fallback", RuntimeFallbackUnsupportedType)
		}
		return nil, false, nil
	}
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayDyadicCompare",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			out, handled, err := qTryTypedCompareMask(dataOp, left, right, la, ra)
			return out, handled, err
		},
	})
}

func qTryTypedRuntimeVectorArithmeticDyadic(op byte, dataOp data.Op, left, right any, la, ra data.Array, recordUnsupportedProbe bool) (any, bool, error) {
	if la == nil && ra == nil {
		return nil, false, nil
	}
	shape := qRuntimeKernelVectorDyadicShape(op, left, right, la, ra)
	typedLeft, typedRight, canUse, err := qVectorDyadicTypedOperands(left, right, la, ra)
	if err != nil {
		if recordUnsupportedProbe {
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "error", RuntimeFallbackUnsupportedType)
		}
		return nil, true, err
	}
	if !canUse || !qVectorDyadicCanUseTypedArithmetic(typedLeft, typedRight) {
		if recordUnsupportedProbe {
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "attempt", "attempt")
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "fallback", RuntimeFallbackUnsupportedType)
		}
		return nil, false, nil
	}
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayDyadicArithmetic",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			return qTryTypedArithmeticDyadic(dataOp, typedLeft, typedRight)
		},
	})
}

func qTryTypedRuntimeBoolLogical(logical, shape string, left, right any) (any, bool, error) {
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayBoolLogical",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			return data.TryTypedBoolLogical(logical, left, right)
		},
	})
}

func qTryTypedRuntimeDyadicMinMax(logical string, wantMax bool, left, right any, la, ra data.Array) (any, bool, error) {
	shape := "minmax/" + logical + "/" + string(qRuntimeKernelOperandKind(left, la)) + "/" + string(qRuntimeKernelOperandKind(right, ra))
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayDyadicMinMax",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			return data.TryTypedDyadicMinMax(left, right, wantMax)
		},
	})
}

func qTryTypedRuntimeScalarFill(scalar any, array data.Array) (data.Array, bool, error) {
	if array == nil {
		return nil, false, nil
	}
	shape := "scalar-fill/" + string(qRuntimeKernelOperandKind(scalar, nil)) + "/" + string(array.Kind())
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel:         "ArrayScalarFill",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (data.Array, bool, error) {
			return data.TryTypedScalarFill(scalar, array)
		},
	})
}

func qTryTypedRuntimeIntegerFloorDivide(op byte, left, right any, la, ra data.Array, n int) (data.Array, bool, error) {
	shape := qRuntimeKernelVectorDyadicShape(op, left, right, la, ra)
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel:         "ArrayDyadicArithmetic",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (data.Array, bool, error) {
			return data.TryTypedIntegerFloorDivide(left, right, n)
		},
	})
}

func qTryTypedRuntimeTemporalDyadic(op byte, dataOp data.Op, left, right any, la, ra data.Array) (data.Array, bool, error) {
	typedLeft, typedRight, canUse, err := qVectorDyadicTypedOperands(left, right, la, ra)
	if err != nil {
		return nil, true, err
	}
	if !canUse || !qVectorDyadicHasTemporalOperand(typedLeft, typedRight) {
		return nil, false, nil
	}
	shape := qRuntimeKernelVectorDyadicShape(op, left, right, la, ra)
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel:         "ArrayDyadicArithmetic",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (data.Array, bool, error) {
			return data.TryTypedTemporalDyadic(dataOp, typedLeft, typedRight)
		},
	})
}

type qTypedWhereCompareDescriptor struct {
	kernel         string
	shape          string
	fallbackReason string
	kind           string
	array          data.Array
	op             data.Op
	scalar         any
	low            any
	high           any
}

type qTypedWhereGatherSumCountDescriptor struct {
	shape         string
	kind          string
	values        data.Array
	predicate     data.Array
	op            data.Op
	scalar        any
	low           any
	high          any
	selfPredicate bool
}

type qTypedFindDescriptor struct {
	domain data.Array
	query  data.Array
}

func qTypedFindDescriptorFor(left, right any) (qTypedFindDescriptor, bool) {
	domain, ok := left.(data.Array)
	if !ok {
		return qTypedFindDescriptor{}, false
	}
	query, ok := right.(data.Array)
	if !ok {
		return qTypedFindDescriptor{}, false
	}
	return qTypedFindDescriptor{domain: domain, query: query}, true
}

func qTypedFindShape(desc qTypedFindDescriptor) string {
	return "find/" + string(desc.domain.Kind()) + "/" + string(desc.query.Kind())
}

func qTypedFindSumShape(desc qTypedFindDescriptor) string {
	return "vector-reduce/find-sum/" + string(desc.domain.Kind()) + "/" + string(desc.query.Kind())
}

func qTypedWhereCompareStatsDescriptor(left, right any, op, comparePrefix, withinStatsPrefix string) (qTypedWhereCompareDescriptor, bool, error) {
	if op == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return qTypedWhereCompareDescriptor{}, ok, err
		}
		return qTypedWhereCompareDescriptor{
			kernel: "ArrayWhereWithinStats",
			shape:  withinStatsPrefix + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil)),
			kind:   "within",
			array:  array,
			low:    low,
			high:   high,
		}, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return qTypedWhereCompareDescriptor{}, false, nil
	}
	return qTypedWhereCompareDescriptor{
		kernel: "ArrayWhereCompareStats",
		shape:  comparePrefix + "-stats/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil)),
		kind:   "compare",
		array:  array,
		op:     dataOp,
		scalar: scalar,
	}, true, nil
}

func qTypedWhereCompareIndexViewStatsDescriptor(left, right any, op, shapePrefix string) (qTypedWhereCompareDescriptor, bool) {
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return qTypedWhereCompareDescriptor{}, false
	}
	return qTypedWhereCompareDescriptor{
		kernel: "ArrayWhereCompareIndexView",
		shape:  shapePrefix + "/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil)),
		kind:   "compare",
		array:  array,
		op:     dataOp,
		scalar: scalar,
	}, true
}

func qTypedWhereCompareCountDescriptor(left, right any, op, comparePrefix, withinPrefix string) (qTypedWhereCompareDescriptor, bool, error) {
	if op == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return qTypedWhereCompareDescriptor{}, ok, err
		}
		return qTypedWhereCompareDescriptor{
			kernel: "ArrayWhereWithinCount",
			shape:  withinPrefix + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil)),
			kind:   "within",
			array:  array,
			low:    low,
			high:   high,
		}, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return qTypedWhereCompareDescriptor{}, false, nil
	}
	return qTypedWhereCompareDescriptor{
		kernel: "ArrayWhereCompareCount",
		shape:  comparePrefix + "/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil)),
		kind:   "compare",
		array:  array,
		op:     dataOp,
		scalar: scalar,
	}, true, nil
}

func qTypedWhereCompareIndexesDescriptor(left, right any, op, comparePrefix, withinPrefix string) (qTypedWhereCompareDescriptor, bool, error) {
	if op == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return qTypedWhereCompareDescriptor{}, ok, err
		}
		return qTypedWhereCompareDescriptor{
			kernel: "ArrayWhereWithin",
			shape:  withinPrefix + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil)),
			kind:   "within",
			array:  array,
			low:    low,
			high:   high,
		}, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return qTypedWhereCompareDescriptor{}, false, nil
	}
	return qTypedWhereCompareDescriptor{
		kernel: "ArrayWhereCompare",
		shape:  comparePrefix + "/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil)),
		kind:   "compare",
		array:  array,
		op:     dataOp,
		scalar: scalar,
	}, true, nil
}

func qTypedWhereGatherSumCountDescriptorFor(values data.Array, left, right any, op, shapePrefix string, selfPredicate bool) (qTypedWhereGatherSumCountDescriptor, bool, error) {
	if op == "within" {
		predicate, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return qTypedWhereGatherSumCountDescriptor{}, ok, err
		}
		return qTypedWhereGatherSumCountDescriptor{
			shape:         shapePrefix + "/" + string(values.Kind()) + "/within/" + string(predicate.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil)),
			kind:          "within",
			values:        values,
			predicate:     predicate,
			low:           low,
			high:          high,
			selfPredicate: selfPredicate,
		}, true, nil
	}
	predicate, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return qTypedWhereGatherSumCountDescriptor{}, false, nil
	}
	return qTypedWhereGatherSumCountDescriptor{
		shape:         shapePrefix + "/" + string(values.Kind()) + "/" + op + "/" + string(predicate.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil)),
		kind:          "compare",
		values:        values,
		predicate:     predicate,
		op:            dataOp,
		scalar:        scalar,
		selfPredicate: selfPredicate,
	}, true, nil
}

func evalQTypedWhereCompareIndexStats(desc qTypedWhereCompareDescriptor) (int64, int64, bool, error) {
	return evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
		kernel:         desc.kernel,
		shape:          desc.shape,
		fallbackReason: desc.fallbackReason,
		call: func() (int64, int64, bool, error) {
			if desc.kind == "within" {
				return data.TryTypedWithinIndexStatsI64(desc.array, desc.low, desc.high, true)
			}
			return data.TryTypedCompareIndexStatsI64(desc.array, desc.op, desc.scalar)
		},
	})
}

func evalQTypedWhereGatherSumCount(desc qTypedWhereGatherSumCountDescriptor) (any, int64, bool, error) {
	return evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[any, int64]{
		kernel:         "ArrayWhereGatherSumCount",
		shape:          desc.shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, int64, bool, error) {
			if desc.kind == "within" {
				if desc.selfPredicate {
					return data.TryTypedNumericSumCountWhereWithinSelf(desc.values, desc.low, desc.high, true)
				}
				return data.TryTypedNumericSumCountWhereWithin(desc.values, desc.predicate, desc.low, desc.high, true)
			}
			if desc.selfPredicate {
				return data.TryTypedNumericSumCountWhereCompareSelf(desc.values, desc.op, desc.scalar)
			}
			return data.TryTypedNumericSumCountWhereCompare(desc.values, desc.predicate, desc.op, desc.scalar)
		},
	})
}

func evalQTypedFind(desc qTypedFindDescriptor) (data.Array, bool, error) {
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel: "ArrayFind",
		shape:  qTypedFindShape(desc),
		call: func() (data.Array, bool, error) {
			out, handled := data.TryTypedFindComparable(desc.domain, desc.query)
			return out, handled, nil
		},
	})
}

func evalQTypedFindSum(desc qTypedFindDescriptor) (int64, bool, error) {
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: "ArrayFindSum",
		shape:  qTypedFindSumShape(desc),
		call: func() (int64, bool, error) {
			out, handled := data.TryTypedFindComparableSum(desc.domain, desc.query)
			return out, handled, nil
		},
	})
}

func evalQTypedWhereCompareCount(desc qTypedWhereCompareDescriptor) (int64, bool, error) {
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel:         desc.kernel,
		shape:          desc.shape,
		fallbackReason: desc.fallbackReason,
		call: func() (int64, bool, error) {
			if desc.kind == "within" {
				return data.TryTypedWithinCount(desc.array, desc.low, desc.high, true)
			}
			return data.TryTypedCompareCount(desc.array, desc.op, desc.scalar)
		},
	})
}

func evalQTypedWhereCompareIndexes(desc qTypedWhereCompareDescriptor) (data.Array, bool, error) {
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel:         desc.kernel,
		shape:          desc.shape,
		fallbackReason: desc.fallbackReason,
		call: func() (data.Array, bool, error) {
			if desc.kind == "within" {
				return data.TryTypedWithinIndexesI64(desc.array, desc.low, desc.high, true)
			}
			return data.TryTypedCompareIndexesI64(desc.array, desc.op, desc.scalar)
		},
	})
}
