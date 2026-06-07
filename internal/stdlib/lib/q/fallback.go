package q

import "strings"

const (
	KernelFallbackUnsupported         = "kernel_unsupported"
	KernelFallbackGroupedProjection   = "kernel_grouped_projection"
	KernelFallbackByExpression        = "kernel_by_expression"
	KernelFallbackAggregateFunction   = "kernel_aggregate_function"
	KernelFallbackAggregateExpression = "kernel_aggregate_expression"
	KernelFallbackAggregateWeight     = "kernel_aggregate_weight"
	KernelFallbackSelectExpression    = "kernel_select_expression"
	KernelFallbackWhereExpression     = "kernel_where_expression"
	KernelFallbackSourceUnavailable   = "kernel_source_unavailable"
	KernelFallbackMutationPlan        = "kernel_mutation_plan"
	KernelFallbackSchemaMismatch      = "kernel_schema_mismatch"
	KernelFallbackCompileError        = "kernel_compile_error"
	KernelFallbackJoinPlan            = "kernel_join_plan"
)

// KernelFallbackReasonCode converts kernel fallback text into a stable bucket
// suitable for explain output and fallback_stats aggregation.
func KernelFallbackReasonCode(reason string) string {
	reason = strings.ToLower(strings.Join(strings.Fields(reason), " "))
	switch {
	case reason == "":
		return KernelFallbackUnsupported
	case strings.Contains(reason, "mutation plan cache"):
		return KernelFallbackMutationPlan
	case strings.Contains(reason, "source") && (strings.Contains(reason, "not found") || strings.Contains(reason, "unavailable")):
		return KernelFallbackSourceUnavailable
	case strings.Contains(reason, "schema mismatch"):
		return KernelFallbackSchemaMismatch
	case strings.Contains(reason, "compile"):
		return KernelFallbackCompileError
	case strings.Contains(reason, "join"):
		return KernelFallbackJoinPlan
	case strings.HasPrefix(reason, "grouped projection without aggregates"):
		return KernelFallbackGroupedProjection
	case strings.HasPrefix(reason, "by expression "):
		return KernelFallbackByExpression
	case strings.HasPrefix(reason, "aggregate ") && strings.Contains(reason, " expression is not supported by data query kernel"):
		return KernelFallbackAggregateExpression
	case strings.HasPrefix(reason, "aggregate ") && strings.Contains(reason, " weight is not supported by data query kernel"):
		return KernelFallbackAggregateWeight
	case strings.HasPrefix(reason, "aggregate ") && strings.Contains(reason, " is not supported by data query kernel"):
		return KernelFallbackAggregateFunction
	case strings.HasPrefix(reason, "select expression "):
		return KernelFallbackSelectExpression
	case strings.HasPrefix(reason, "where expression "):
		return KernelFallbackWhereExpression
	default:
		return KernelFallbackUnsupported
	}
}
