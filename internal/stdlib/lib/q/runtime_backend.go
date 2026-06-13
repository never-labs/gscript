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

type qTypedWhereCompareDescriptor struct {
	kernel string
	shape  string
	kind   string
	array  data.Array
	op     data.Op
	scalar any
	low    any
	high   any
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

func evalQTypedWhereCompareIndexStats(desc qTypedWhereCompareDescriptor) (int64, int64, bool, error) {
	return evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
		kernel: desc.kernel,
		shape:  desc.shape,
		call: func() (int64, int64, bool, error) {
			if desc.kind == "within" {
				return data.TryTypedWithinIndexStatsI64(desc.array, desc.low, desc.high, true)
			}
			return data.TryTypedCompareIndexStatsI64(desc.array, desc.op, desc.scalar)
		},
	})
}

func evalQTypedWhereCompareCount(desc qTypedWhereCompareDescriptor) (int64, bool, error) {
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: desc.kernel,
		shape:  desc.shape,
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
		kernel: desc.kernel,
		shape:  desc.shape,
		call: func() (data.Array, bool, error) {
			if desc.kind == "within" {
				return data.TryTypedWithinIndexesI64(desc.array, desc.low, desc.high, true)
			}
			return data.TryTypedCompareIndexesI64(desc.array, desc.op, desc.scalar)
		},
	})
}
