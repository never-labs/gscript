package methodjit

import "testing"

func TestQKernelDescriptorsUseStructuredRemarkFields(t *testing.T) {
	remarks := []OptimizationRemark{
		{
			Pass:   "QQueryNativeLowering",
			Kind:   "missed",
			Op:     OpCall.String(),
			Reason: "group aggregate remains on opaque call fallback",
			Fields: map[string]string{
				"kernel":        "QGroupAggregate",
				"shape":         "select/where/group/aggregate/order",
				"reason_family": "lowering",
				"reason_code":   qQueryLoweringFallbackGroupAggregateCall,
				"route":         "lowering",
				"outcome":       "fallback",
			},
		},
	}

	fallbacks := CountQQueryLoweringFallbackReasons(remarks)
	if got := fallbacks[qQueryLoweringFallbackGroupAggregateCall]; got != 1 {
		t.Fatalf("CountQQueryLoweringFallbackReasons = %+v, want structured reason count 1", fallbacks)
	}
	descriptors := BuildQKernelDescriptors(nil, nil, nil, remarks)
	assertQKernelDescriptor(t, descriptors, "methodjit_q_query_lowering", "fallback", "QGroupAggregate", "select/where/group/aggregate/order", "lowering", "fallback", qQueryLoweringFallbackGroupAggregateCall)
}

func TestQKernelDescriptorsUseStructuredRemarkRouteFields(t *testing.T) {
	remarks := []OptimizationRemark{
		{
			Pass:   "QQueryNativeLowering",
			Kind:   "missed",
			Op:     OpCall.String(),
			Reason: "future route fallback",
			Fields: map[string]string{
				"kind":          "fallback",
				"kernel":        "QJoin",
				"shape":         "join/inner",
				"reason_family": "schema",
				"reason_code":   qQueryLoweringFallbackJoinCall,
				"route":         "schema_stable_lowering",
				"outcome":       "fallback",
			},
		},
	}

	descriptors := BuildQKernelDescriptors(nil, nil, nil, remarks)
	assertQKernelDescriptor(t, descriptors, "methodjit_q_query_lowering", "fallback", "QJoin", "join/inner", "schema_stable_lowering", "fallback", qQueryLoweringFallbackJoinCall)
	if descriptors[0].ReasonFamily != "schema" {
		t.Fatalf("descriptor reason family = %q, want schema", descriptors[0].ReasonFamily)
	}
}

func TestQVectorLoweringFallbackReasonsUseStructuredRemarkFields(t *testing.T) {
	remarks := []OptimizationRemark{
		{
			Pass:   "QVectorNativeLowering",
			Kind:   "missed",
			Op:     OpVectorReduce.String(),
			Reason: "where reduce remains on primitive fallback",
			Fields: map[string]string{
				"kernel":        "QVectorWhereReduce",
				"shape":         "compare/vector-where/vector-reduce",
				"reason_family": "lowering",
				"reason_code":   qVectorWhereReduceFallbackSharedWhere,
				"route":         "lowering",
				"outcome":       "fallback",
			},
		},
	}

	fallbacks := CountQVectorLoweringFallbackReasons(remarks)
	if got := fallbacks[qVectorWhereReduceFallbackSharedWhere]; got != 1 {
		t.Fatalf("CountQVectorLoweringFallbackReasons = %+v, want structured reason count 1", fallbacks)
	}
	descriptors := BuildQKernelDescriptors(nil, nil, nil, remarks)
	assertQKernelDescriptor(t, descriptors, "methodjit_q_vector_lowering", "fallback", "QVectorWhereReduce", "compare/vector-where/vector-reduce", "lowering", "fallback", qVectorWhereReduceFallbackSharedWhere)
}

func TestQKernelDescriptorsKeepLegacyRemarkFieldFallback(t *testing.T) {
	remarks := []OptimizationRemark{
		{
			Pass:   "QQueryNativeLowering",
			Kind:   "missed",
			Op:     OpCall.String(),
			Reason: "kernel=QJoin reason_code=join_call shape=join/window1; legacy fallback",
		},
	}

	descriptors := BuildQKernelDescriptors(nil, nil, nil, remarks)
	assertQKernelDescriptor(t, descriptors, "methodjit_q_query_lowering", "fallback", "QJoin", "join/window1", "lowering", "fallback", qQueryLoweringFallbackJoinCall)
}
