package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func (s *EvalState) evalQPipelineRuntimePrimitive(plan qPipelinePlan) (any, bool, error) {
	verb := strings.TrimSpace(plan.compareOp)
	shape := qRuntimePrimitiveShape(plan)
	switch plan.kind {
	case qPipelineUnaryPrimitive:
		arg, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
		if err != nil {
			recordRuntimeKernelProbe("QRuntimePrimitive", shape, false, err)
			return nil, true, err
		}
		fn, ok := lookupUnaryVerb(verb)
		if !ok {
			recordRuntimeKernelProbeReason("QRuntimePrimitive", shape, false, nil, RuntimeFallbackPlannerUnhandled)
			return nil, false, nil
		}
		out, err := fn(arg)
		recordRuntimeKernelProbeReason("QRuntimePrimitive", shape, err == nil, err, qRuntimePrimitiveFallbackReason(arg, nil))
		return out, true, err
	case qPipelineDyadicPrimitive:
		left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
		if err != nil {
			recordRuntimeKernelProbe("QRuntimePrimitive", shape, false, err)
			return nil, true, err
		}
		right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
		if err != nil {
			recordRuntimeKernelProbe("QRuntimePrimitive", shape, false, err)
			return nil, true, err
		}
		fn, ok := lookupDyadicVerbFunc(verb)
		if !ok {
			recordRuntimeKernelProbeReason("QRuntimePrimitive", shape, false, nil, RuntimeFallbackPlannerUnhandled)
			return nil, false, nil
		}
		out, err := fn(left, right)
		recordRuntimeKernelProbeReason("QRuntimePrimitive", shape, err == nil, err, qRuntimePrimitiveFallbackReason(left, right))
		return out, true, err
	default:
		recordRuntimeKernelProbeReason("QRuntimePrimitive", shape, false, nil, RuntimeFallbackPlannerUnhandled)
		return nil, false, nil
	}
}

func qRuntimePrimitiveShape(plan qPipelinePlan) string {
	verb := strings.TrimSpace(plan.compareOp)
	if verb == "" {
		verb = "unknown"
	}
	prefix := "runtime-primitive"
	switch plan.kind {
	case qPipelineUnaryPrimitive:
		return qRuntimeUnaryPrimitiveShape(verb)
	case qPipelineDyadicPrimitive:
		prefix = "runtime-dyadic"
	}
	return prefix + "/" + verb
}

func qRuntimePrimitiveFallbackReason(values ...any) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		if _, ok := value.(data.Array); ok {
			continue
		}
		switch value.(type) {
		case bool, int, int8, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64, string, data.Symbol:
			continue
		default:
			return RuntimeFallbackUnsupportedType
		}
	}
	return RuntimeFallbackUnsupportedShape
}
