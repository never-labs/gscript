package q

import "testing"

func TestQPipelinePlanFromEvalDescriptorUsesShapeRegistry(t *testing.T) {
	tests := []struct {
		name              string
		shape             string
		wantKind          qPipelineKind
		wantCompareOp     string
		wantComparePrefix string
		wantPipelineShape string
	}{
		{
			name:              "where reduce",
			shape:             "where-reduce/sum",
			wantKind:          qPipelineSumWhereMask,
			wantComparePrefix: "where-reduce/sum",
			wantPipelineShape: "mask_reduce",
		},
		{
			name:              "where index reduce",
			shape:             "where-index-reduce/sum",
			wantKind:          qPipelineSumWhereIndex,
			wantComparePrefix: "where-index-reduce/sum",
			wantPipelineShape: "mask_reduce",
		},
		{
			name:              "gather reduce",
			shape:             "gather-reduce/sum",
			wantKind:          qPipelineSumGatherIndexes,
			wantComparePrefix: "gather-reduce/sum",
			wantPipelineShape: "gather_reduce",
		},
		{
			name:              "compare sum",
			shape:             "compare-to-index-sum",
			wantKind:          qPipelineSumWhereCompare,
			wantComparePrefix: "compare-to-index-sum",
			wantPipelineShape: "mask_reduce",
		},
		{
			name:              "compare count",
			shape:             "compare-to-index-count",
			wantKind:          qPipelineCountWhereCompare,
			wantComparePrefix: "compare-to-index-count",
			wantPipelineShape: "mask_reduce",
		},
		{
			name:              "compare indexes",
			shape:             "compare-to-index",
			wantKind:          qPipelineWhereCompareIndexes,
			wantComparePrefix: "compare-to-index",
			wantPipelineShape: "compare_index",
		},
		{
			name:              "compare sum mod",
			shape:             "compare-to-index-sum-mod",
			wantKind:          qPipelineSumWhereModuloCompare,
			wantComparePrefix: "compare-to-index-sum-mod",
			wantPipelineShape: "mask_reduce",
		},
		{
			name:              "compare count mod",
			shape:             "compare-to-index-count-mod",
			wantKind:          qPipelineCountWhereModuloCompare,
			wantComparePrefix: "compare-to-index-count-mod",
			wantPipelineShape: "mask_reduce",
		},
		{
			name:              "compare index mod",
			shape:             "compare-to-index-mod",
			wantKind:          qPipelineWhereModuloCompareIndexes,
			wantComparePrefix: "compare-to-index-mod",
			wantPipelineShape: "compare_index",
		},
		{
			name:              "bin reduce",
			shape:             "bin-reduce/sum",
			wantKind:          qPipelineSumBin,
			wantComparePrefix: "bin-reduce/sum",
			wantPipelineShape: "search_index_reduce",
		},
		{
			name:              "runtime unary",
			shape:             "runtime-unary/sqrt",
			wantKind:          qPipelineUnaryPrimitive,
			wantCompareOp:     "sqrt",
			wantComparePrefix: "runtime-unary/sqrt",
			wantPipelineShape: "numeric_math",
		},
		{
			name:              "runtime dyadic",
			shape:             "runtime-dyadic/cov",
			wantKind:          qPipelineDyadicPrimitive,
			wantCompareOp:     "cov",
			wantComparePrefix: "runtime-dyadic/cov",
			wantPipelineShape: "numeric_stats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, ok := qPipelinePlanFromEvalDescriptor(EvalPipelineDescriptor{
				Kind:  evalPipelineKindExpression,
				Shape: tt.shape,
			})
			if !ok {
				t.Fatalf("restore %q failed", tt.shape)
			}
			if plan.kind != tt.wantKind {
				t.Fatalf("kind = %v, want %v", plan.kind, tt.wantKind)
			}
			if got := plan.stableShape(); got != tt.shape {
				t.Fatalf("stable shape = %q, want %q", got, tt.shape)
			}
			if plan.compareOp != tt.wantCompareOp {
				t.Fatalf("compare op = %q, want %q", plan.compareOp, tt.wantCompareOp)
			}
			if plan.comparePrefix != tt.wantComparePrefix {
				t.Fatalf("compare prefix = %q, want %q", plan.comparePrefix, tt.wantComparePrefix)
			}
			if got := plan.stablePipelineShape(); got != tt.wantPipelineShape {
				t.Fatalf("pipeline shape = %q, want %q", got, tt.wantPipelineShape)
			}
		})
	}
}
