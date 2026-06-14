package q

import "testing"

func TestKernelFallbackReasonCodeBucketsKernelReasons(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   string
	}{
		{
			name:   "empty",
			reason: "",
			want:   KernelFallbackUnsupported,
		},
		{
			name:   "grouped projection",
			reason: "grouped projection without aggregates requires QueryPlan fallback",
			want:   KernelFallbackGroupedProjection,
		},
		{
			name:   "by expression",
			reason: `by expression "bucket" is not supported by data query kernel: unsupported expression q.customExpr`,
			want:   KernelFallbackByExpression,
		},
		{
			name:   "aggregate function",
			reason: `aggregate "lasts" is not supported by data query kernel`,
			want:   KernelFallbackAggregateFunction,
		},
		{
			name:   "aggregate expression",
			reason: `aggregate "notional" expression is not supported by data query kernel: unsupported expression q.customExpr`,
			want:   KernelFallbackAggregateExpression,
		},
		{
			name:   "aggregate weight",
			reason: `aggregate "wavg_px" weight is not supported by data query kernel: unsupported expression q.customExpr`,
			want:   KernelFallbackAggregateWeight,
		},
		{
			name:   "select expression",
			reason: `select expression "marker" is not supported by data query kernel: unsupported expression q.customExpr`,
			want:   KernelFallbackSelectExpression,
		},
		{
			name:   "where expression",
			reason: "where expression is not supported by data query kernel: logical left operand: unsupported expression q.customExpr",
			want:   KernelFallbackWhereExpression,
		},
		{
			name:   "source unavailable",
			reason: `source "trades" not found`,
			want:   KernelFallbackSourceUnavailable,
		},
		{
			name:   "mutation",
			reason: "mutation plan cache requires QueryPlan fallback",
			want:   KernelFallbackMutationPlan,
		},
		{
			name:   "schema mismatch",
			reason: `query kernel schema mismatch for column "price": got i64, want f64`,
			want:   KernelFallbackSchemaMismatch,
		},
		{
			name:   "compile",
			reason: "compile query kernel: unsupported kernel expression q.customExpr",
			want:   KernelFallbackCompileError,
		},
		{
			name:   "join",
			reason: `join kind "window" requires QueryPlan fallback`,
			want:   KernelFallbackJoinPlan,
		},
		{
			name:   "unknown",
			reason: "unsupported expression q.customExpr",
			want:   KernelFallbackUnsupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KernelFallbackReasonCode(tt.reason); got != tt.want {
				t.Fatalf("KernelFallbackReasonCode(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}

func TestKernelFallbackReasonCodeNormalizesWhitespaceAndCase(t *testing.T) {
	reason := "  SELECT   expression \"marker\"\nIS NOT SUPPORTED BY DATA QUERY KERNEL: unsupported expression q.customExpr  "
	if got := KernelFallbackReasonCode(reason); got != KernelFallbackSelectExpression {
		t.Fatalf("KernelFallbackReasonCode normalized = %q, want %q", got, KernelFallbackSelectExpression)
	}
}

func TestRuntimeFallbackReasonCodeBucketsStableRuntimeReasons(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   string
	}{
		{name: "empty", reason: "", want: RuntimeFallbackUnsupportedShape},
		{name: "legacy runtime shape", reason: "unsupported_runtime_shape", want: RuntimeFallbackUnsupportedShape},
		{name: "type mismatch", reason: "typed helper kind mismatch", want: RuntimeFallbackUnsupportedType},
		{name: "runtime error", reason: "runtime error: divide by zero", want: RuntimeFallbackRuntimeError},
		{name: "planner", reason: "unsupported plan for terminal", want: RuntimeFallbackPlannerUnhandled},
		{name: "semantic guard", reason: "semantic guard: symbol cast", want: RuntimeFallbackSemanticGuard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RuntimeFallbackReasonCode(tt.reason); got != tt.want {
				t.Fatalf("RuntimeFallbackReasonCode(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}

func TestRuntimeErrorReasonCodeBucketsStableRuntimeReasons(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   string
	}{
		{name: "empty", reason: "", want: RuntimeFallbackRuntimeError},
		{name: "unknown", reason: "typed helper failed", want: RuntimeFallbackRuntimeError},
		{name: "runtime error", reason: "runtime error: divide by zero", want: RuntimeFallbackRuntimeError},
		{name: "type mismatch", reason: "typed helper kind mismatch", want: RuntimeFallbackUnsupportedType},
		{name: "planner", reason: "unsupported plan for terminal", want: RuntimeFallbackPlannerUnhandled},
		{name: "semantic guard", reason: "semantic guard: symbol cast", want: RuntimeFallbackSemanticGuard},
		{name: "apply error", reason: "callable apply failed", want: RuntimeFallbackApplyError},
		{name: "pipeline error", reason: "script pipeline failed", want: RuntimeFallbackPipelineError},
		{name: "backend compile", reason: "backend compile failed", want: RuntimeFallbackBackendCompile},
		{name: "backend exec", reason: "backend exec failed", want: RuntimeFallbackBackendExec},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RuntimeErrorReasonCode(tt.reason); got != tt.want {
				t.Fatalf("RuntimeErrorReasonCode(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}
