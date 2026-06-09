package q

func recordQTypedRuntimeKernel(kernel, shape string, handled bool, err error) {
	recordQTypedRuntimeKernelReason(kernel, shape, handled, err, RuntimeFallbackUnsupportedShape)
}

func recordQTypedRuntimeKernelReason(kernel, shape string, handled bool, err error, fallbackReason string) {
	if fallbackReason == "" {
		fallbackReason = RuntimeFallbackUnsupportedShape
	}
	recordRuntimeKernelProbeReason(kernel, shape, handled, err, fallbackReason)
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
