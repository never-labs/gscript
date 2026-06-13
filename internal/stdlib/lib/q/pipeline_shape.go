package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qPipelineShapeFamily string

const (
	qPipelineShapeFamilyUnknown qPipelineShapeFamily = ""
	qPipelineShapeFamilyWhere   qPipelineShapeFamily = "where"
	qPipelineShapeFamilyGather  qPipelineShapeFamily = "gather"
	qPipelineShapeFamilyVector  qPipelineShapeFamily = "vector"
	qPipelineShapeFamilyStats   qPipelineShapeFamily = "stats"
	qPipelineShapeFamilyWindow  qPipelineShapeFamily = "window"
	qPipelineShapeFamilyApply   qPipelineShapeFamily = "apply"
	qPipelineShapeFamilyCast    qPipelineShapeFamily = "cast"
)

type qPipelineShapeSpec struct {
	ID            string
	Family        qPipelineShapeFamily
	Reducer       string
	Selector      string
	Transform     string
	PipelineShape string
}

type qPipelineShapeVariantField uint8

const (
	qPipelineShapeVariantNone qPipelineShapeVariantField = iota
	qPipelineShapeVariantCompareOp
	qPipelineShapeVariantUnaryOp
)

type qPipelineShapeRegistryEntry struct {
	ID            string
	Prefix        string
	Kind          qPipelineKind
	Variant       string
	VariantField  qPipelineShapeVariantField
	ComparePrefix string
}

var qPipelineDescriptorShapeRegistry = []qPipelineShapeRegistryEntry{
	{ID: "where-reduce/sum", Kind: qPipelineSumWhereMask},
	{ID: "where-index-reduce/sum", Kind: qPipelineSumWhereIndex},
	{ID: "gather-reduce/sum", Kind: qPipelineSumGatherIndexes},
	{ID: "compare-to-index-sum", Kind: qPipelineSumWhereCompare},
	{ID: "compare-to-index-count", Kind: qPipelineCountWhereCompare},
	{ID: "compare-to-index", Kind: qPipelineWhereCompareIndexes},
	{ID: "compare-to-index-sum-mod", Kind: qPipelineSumWhereModuloCompare},
	{ID: "compare-to-index-count-mod", Kind: qPipelineCountWhereModuloCompare},
	{ID: "compare-to-index-mod", Kind: qPipelineWhereModuloCompareIndexes},
	{ID: "bin-reduce/sum", Kind: qPipelineSumBin},
	{ID: "vector-reduce/sum-deltas", Kind: qPipelineSumDeltas},
	{ID: "vector-reduce/sum-expr", Kind: qPipelineSumVectorExpr},
	{ID: "vector-reduce/sum-dyadic-min", Kind: qPipelineSumDyadicMinMax, Variant: "min", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-dyadic-max", Kind: qPipelineSumDyadicMinMax, Variant: "max", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-dyadic-float-xexp", Kind: qPipelineSumDyadicFloatMath, Variant: data.NumericDyadicXExp, VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-dyadic-float-xlog", Kind: qPipelineSumDyadicFloatMath, Variant: data.NumericDyadicXLog, VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-reverse", Kind: qPipelineSumSequenceTransform, Variant: data.SequenceTransformReverse, VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-rotate", Kind: qPipelineSumSequenceTransform, Variant: data.SequenceTransformRotate, VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-sublist", Kind: qPipelineSumSequenceTransform, Variant: data.SequenceTransformSublist, VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-ratios", Kind: qPipelineSumSequenceTransform, Variant: data.SequenceTransformRatios, VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-raze", Kind: qPipelineSumRaze},
	{ID: "vector-count/expr", Kind: qPipelineCountVectorExpr},
	{ID: "vector-reduce/sum-msum", Kind: qPipelineSumMovingWindow, Variant: "msum", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-mavg", Kind: qPipelineSumMovingWindow, Variant: "mavg", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-mcount", Kind: qPipelineSumMovingWindow, Variant: "mcount", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-mmin", Kind: qPipelineSumMovingWindow, Variant: "mmin", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-mmax", Kind: qPipelineSumMovingWindow, Variant: "mmax", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-mdev", Kind: qPipelineSumMovingWindow, Variant: "mdev", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-reduce/sum-ema", Kind: qPipelineSumMovingWindow, Variant: "ema", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-count/sums", Kind: qPipelineCountRunningScan, Variant: "sums", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-count/prds", Kind: qPipelineCountRunningScan, Variant: "prds", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-count/mins", Kind: qPipelineCountRunningScan, Variant: "mins", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-count/maxs", Kind: qPipelineCountRunningScan, Variant: "maxs", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-count/avgs", Kind: qPipelineCountRunningScan, Variant: "avgs", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-last/sums", Kind: qPipelineLastRunningScan, Variant: "sums", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-last/prds", Kind: qPipelineLastRunningScan, Variant: "prds", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-last/mins", Kind: qPipelineLastRunningScan, Variant: "mins", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-last/maxs", Kind: qPipelineLastRunningScan, Variant: "maxs", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "vector-last/avgs", Kind: qPipelineLastRunningScan, Variant: "avgs", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "sequence-count/trim", Kind: qPipelineCountSequencePrimitive, Variant: "trim", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "sequence-count/ltrim", Kind: qPipelineCountSequencePrimitive, Variant: "ltrim", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "sequence-count/rtrim", Kind: qPipelineCountSequencePrimitive, Variant: "rtrim", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "sequence-count/cross", Kind: qPipelineCountSequencePrimitive, Variant: "cross", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "sequence-count/cut", Kind: qPipelineCountSequencePrimitive, Variant: "cut", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "sequence-count/sublist", Kind: qPipelineCountSequencePrimitive, Variant: "sublist", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "sequence-count/raze", Kind: qPipelineCountSequencePrimitive, Variant: "raze", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "sequence-count/value", Kind: qPipelineCountSequencePrimitive, Variant: "value", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "apply-index/scalar-at", Kind: qPipelineApplyScalarIndex, Variant: "at", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "apply-index/gather-at", Kind: qPipelineApplyGatherIndex, Variant: "at", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "apply-index/scalar-dot", Kind: qPipelineApplyScalarIndex, Variant: "dot", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "apply-index/path-dot", Kind: qPipelineApplyScalarIndex, Variant: "dot", VariantField: qPipelineShapeVariantCompareOp},
	{ID: "cast-envelope/sum", Kind: qPipelineCastEnvelopeSum},
	{Prefix: "runtime-unary/", Kind: qPipelineUnaryPrimitive, VariantField: qPipelineShapeVariantCompareOp},
	{Prefix: "runtime-dyadic/", Kind: qPipelineDyadicPrimitive, VariantField: qPipelineShapeVariantCompareOp},
	{Prefix: "vector-reduce/sum-unary-", Kind: qPipelineSumUnaryPrimitive, VariantField: qPipelineShapeVariantUnaryOp},
	{Prefix: "numeric-unary-compare-to-index/", Kind: qPipelineWhereUnaryCompareIndexes, VariantField: qPipelineShapeVariantUnaryOp, ComparePrefix: "numeric-unary-compare-to-index"},
}

func (s qPipelineShapeSpec) valid() bool {
	return s.ID != ""
}

func (s qPipelineShapeSpec) stableID() string {
	return s.ID
}

func qPipelineShapeSpecForPlan(kind qPipelineKind, variant string) (qPipelineShapeSpec, bool) {
	switch kind {
	case qPipelineSumWhereMask:
		return qPipelineShapeSpec{
			ID:            "where-reduce/sum",
			Family:        qPipelineShapeFamilyWhere,
			Reducer:       "sum",
			Selector:      "mask",
			PipelineShape: "mask_reduce",
		}, true
	case qPipelineSumWhereIndex:
		return qPipelineShapeSpec{
			ID:            "where-index-reduce/sum",
			Family:        qPipelineShapeFamilyWhere,
			Reducer:       "sum",
			Selector:      "index",
			PipelineShape: "mask_reduce",
		}, true
	case qPipelineSumGatherIndexes:
		return qPipelineShapeSpec{
			ID:            "gather-reduce/sum",
			Family:        qPipelineShapeFamilyGather,
			Reducer:       "sum",
			Selector:      "index",
			PipelineShape: "gather_reduce",
		}, true
	case qPipelineSumWhereCompare:
		return qPipelineShapeSpec{
			ID:            "compare-to-index-sum",
			Family:        qPipelineShapeFamilyWhere,
			Reducer:       "sum",
			Selector:      "index",
			PipelineShape: "mask_reduce",
		}, true
	case qPipelineCountWhereCompare:
		return qPipelineShapeSpec{
			ID:            "compare-to-index-count",
			Family:        qPipelineShapeFamilyWhere,
			Reducer:       "count",
			Selector:      "index",
			PipelineShape: "mask_reduce",
		}, true
	case qPipelineWhereCompareIndexes:
		return qPipelineShapeSpec{
			ID:            "compare-to-index",
			Family:        qPipelineShapeFamilyWhere,
			Selector:      "index",
			PipelineShape: "compare_index",
		}, true
	case qPipelineSumWhereModuloCompare:
		return qPipelineShapeSpec{
			ID:            "compare-to-index-sum-mod",
			Family:        qPipelineShapeFamilyWhere,
			Reducer:       "sum",
			Selector:      "index",
			Transform:     "mod",
			PipelineShape: "mask_reduce",
		}, true
	case qPipelineCountWhereModuloCompare:
		return qPipelineShapeSpec{
			ID:            "compare-to-index-count-mod",
			Family:        qPipelineShapeFamilyWhere,
			Reducer:       "count",
			Selector:      "index",
			Transform:     "mod",
			PipelineShape: "mask_reduce",
		}, true
	case qPipelineWhereModuloCompareIndexes:
		return qPipelineShapeSpec{
			ID:            "compare-to-index-mod",
			Family:        qPipelineShapeFamilyWhere,
			Selector:      "index",
			Transform:     "mod",
			PipelineShape: "compare_index",
		}, true
	case qPipelineSumVectorExpr:
		return qPipelineShapeSpec{
			ID:            "vector-reduce/sum-expr",
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "sum",
			Transform:     "expr",
			PipelineShape: "vector_reduce",
		}, true
	case qPipelineSumDyadicMinMax:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "vector-reduce/sum-dyadic-" + variant,
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "sum",
			Transform:     "dyadic-" + variant,
			PipelineShape: "vector_reduce",
		}, true
	case qPipelineSumDyadicFloatMath:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "vector-reduce/sum-dyadic-float-" + variant,
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "sum",
			Transform:     "dyadic-float-" + variant,
			PipelineShape: "vector_reduce",
		}, true
	case qPipelineSumUnaryPrimitive:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "vector-reduce/sum-unary-" + variant,
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "sum",
			Transform:     variant,
			PipelineShape: "vector_reduce",
		}, true
	case qPipelineWhereUnaryCompareIndexes:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "numeric-unary-compare-to-index/" + variant,
			Family:        qPipelineShapeFamilyWhere,
			Selector:      "index",
			Transform:     variant,
			PipelineShape: "compare_index",
		}, true
	case qPipelineSumSequenceTransform:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "vector-reduce/sum-" + variant,
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "sum",
			Transform:     variant,
			PipelineShape: "vector_reduce",
		}, true
	case qPipelineSumRaze:
		return qPipelineShapeSpec{
			ID:            "vector-reduce/sum-raze",
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "sum",
			Transform:     "raze",
			PipelineShape: "vector_reduce",
		}, true
	case qPipelineCountVectorExpr:
		return qPipelineShapeSpec{
			ID:            "vector-count/expr",
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "count",
			Transform:     "expr",
			PipelineShape: "vector_scan",
		}, true
	case qPipelineSumDeltas:
		return qPipelineShapeSpec{
			ID:            "vector-reduce/sum-deltas",
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "sum",
			Transform:     "deltas",
			PipelineShape: "vector_reduce",
		}, true
	case qPipelineSumMovingWindow:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "vector-reduce/sum-" + variant,
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "sum",
			Transform:     variant,
			PipelineShape: "vector_reduce",
		}, true
	case qPipelineCountRunningScan:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "vector-count/" + variant,
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "count",
			Transform:     variant,
			PipelineShape: "vector_scan",
		}, true
	case qPipelineLastRunningScan:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "vector-last/" + variant,
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "last",
			Transform:     variant,
			PipelineShape: "vector_scan",
		}, true
	case qPipelineCountSequencePrimitive:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineShapeSpec{
			ID:            "sequence-count/" + variant,
			Family:        qPipelineShapeFamilyVector,
			Reducer:       "count",
			Transform:     variant,
			PipelineShape: "sequence_count",
		}, true
	case qPipelineUnaryPrimitive:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineRuntimePrimitiveShapeSpec("runtime-unary", variant), true
	case qPipelineDyadicPrimitive:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		return qPipelineRuntimePrimitiveShapeSpec("runtime-dyadic", variant), true
	case qPipelineApplyGatherIndex:
		return qPipelineShapeSpec{
			ID:            "apply-index/gather-at",
			Family:        qPipelineShapeFamilyApply,
			Selector:      "gather",
			Transform:     "at",
			PipelineShape: "apply_index",
		}, true
	case qPipelineApplyScalarIndex:
		if variant == "" {
			return qPipelineShapeSpec{}, false
		}
		if variant == "path-dot" {
			return qPipelineShapeSpec{
				ID:            "apply-index/path-dot",
				Family:        qPipelineShapeFamilyApply,
				Selector:      "path",
				Transform:     "dot",
				PipelineShape: "apply_index",
			}, true
		}
		return qPipelineShapeSpec{
			ID:            "apply-index/scalar-" + variant,
			Family:        qPipelineShapeFamilyApply,
			Selector:      "scalar",
			Transform:     variant,
			PipelineShape: "apply_index",
		}, true
	case qPipelineCastEnvelopeSum:
		return qPipelineShapeSpec{
			ID:            "cast-envelope/sum",
			Family:        qPipelineShapeFamilyCast,
			Reducer:       "sum",
			Transform:     "typed-cast",
			PipelineShape: "cast",
		}, true
	default:
		return qPipelineShapeSpec{}, false
	}
}

func qPipelineApplyDescriptorShapeRegistry(plan *qPipelinePlan, shape string) bool {
	for _, entry := range qPipelineDescriptorShapeRegistry {
		var variant string
		switch {
		case entry.ID != "" && shape == entry.ID:
			variant = entry.Variant
		case entry.Prefix != "" && strings.HasPrefix(shape, entry.Prefix):
			variant = strings.TrimPrefix(shape, entry.Prefix)
		default:
			continue
		}
		if entry.Kind == qPipelineInvalid {
			return false
		}
		plan.kind = entry.Kind
		switch entry.VariantField {
		case qPipelineShapeVariantCompareOp:
			if variant != "" {
				plan.compareOp = variant
			}
		case qPipelineShapeVariantUnaryOp:
			if plan.unaryOp == "" {
				plan.unaryOp = variant
			}
		}
		if entry.ComparePrefix != "" && plan.comparePrefix == "" {
			plan.comparePrefix = entry.ComparePrefix
		}
		if spec, ok := qPipelineShapeSpecForPlan(plan.kind, plan.shapeVariant()); ok && spec.ID == shape {
			plan.shapeSpec = spec
		}
		return true
	}
	return false
}

func qPipelineShapeSpecForShape(shape string) (qPipelineShapeSpec, bool) {
	shape = strings.TrimSpace(shape)
	if shape == "" {
		return qPipelineShapeSpec{}, false
	}
	plan := qPipelinePlan{shape: shape}
	if !qPipelineApplyDescriptorShapeRegistry(&plan, shape) {
		return qPipelineShapeSpec{}, false
	}
	if plan.shapeSpec.valid() {
		return plan.shapeSpec, true
	}
	if spec, ok := qPipelineShapeSpecForPlan(plan.kind, plan.shapeVariant()); ok && spec.ID == shape {
		return spec, true
	}
	return qPipelineShapeSpec{}, false
}

func qPipelineRuntimePrimitiveShapeSpec(prefix, verb string) qPipelineShapeSpec {
	spec := qPipelineShapeSpec{
		ID:            prefix + "/" + verb,
		Transform:     verb,
		PipelineShape: qRuntimePrimitivePipelineShape(verb),
	}
	switch spec.PipelineShape {
	case "numeric_math":
		spec.Family = qPipelineShapeFamilyVector
	case "numeric_stats":
		spec.Family = qPipelineShapeFamilyStats
		spec.Reducer = verb
	case "window_scan":
		spec.Family = qPipelineShapeFamilyWindow
	default:
		spec.Family = qPipelineShapeFamilyVector
	}
	return spec
}

func qRuntimePrimitivePipelineShape(verb string) string {
	switch verb {
	case "abs", "sqrt", "log", "exp", "sin", "cos", "tan", "asin", "acos", "atan", "reciprocal", "signum", "floor", "ceiling", "xexp", "xlog":
		return "numeric_math"
	case "svar", "sdev", "wsum", "cov", "scov", "cor":
		return "numeric_stats"
	case "mdev", "ema":
		return "window_scan"
	default:
		return "runtime_primitive"
	}
}

func qPipelineShapePlan(kind qPipelineKind, variant string) qPipelinePlan {
	spec, ok := qPipelineShapeSpecForPlan(kind, variant)
	if !ok {
		return qPipelinePlan{kind: kind}
	}
	return qPipelinePlan{
		kind:      kind,
		shape:     spec.ID,
		shapeSpec: spec,
	}
}

func qNormalizePipelinePlan(plan qPipelinePlan) qPipelinePlan {
	plan = qPipelinePlanWithShapeSpec(plan)
	plan.operands = plan.operands[:0]
	qAddPipelineOperand(&plan, qPipelineOperandValue, plan.valueExpr, &plan.valuePlan)
	qAddPipelineOperand(&plan, qPipelineOperandIndex, plan.indexExpr, &plan.indexPlan)
	qAddPipelineOperand(&plan, qPipelineOperandMask, plan.maskExpr, &plan.maskPlan)
	qAddPipelineOperand(&plan, qPipelineOperandLeft, plan.leftExpr, &plan.leftPlan)
	qAddPipelineOperand(&plan, qPipelineOperandRight, plan.rightExpr, &plan.rightPlan)
	qAddPipelineOperand(&plan, qPipelineOperandMod, plan.modExpr, &plan.modPlan)
	qAddPipelineOperand(&plan, qPipelineOperandModulus, plan.modulusExpr, &plan.modulusPlan)
	qAddPipelineOperand(&plan, qPipelineOperandModTarget, plan.modTargetExpr, &plan.modTargetPlan)
	qAddPipelineOperand(&plan, qPipelineOperandReduction, plan.reductionInput, &plan.reductionPlan)
	for i := range plan.castTerms {
		if plan.castTerms[i].valueExpr != "" {
			plan.castTerms[i].valuePlan = buildQPipelineBindingPlan(plan.castTerms[i].valueExpr)
		}
	}
	if plan.maskExpr != "" {
		if modPlan, ok := qPipelineModuloComparePlanFromMask(plan.maskExpr); ok {
			plan.moduloMaskPlan = &modPlan
		}
	}
	return plan
}

func qPipelinePlanWithShapeSpec(plan qPipelinePlan) qPipelinePlan {
	if !plan.shapeSpec.valid() {
		if spec, ok := qPipelineShapeSpecForPlan(plan.kind, plan.shapeVariant()); ok && (plan.shape == "" || plan.shape == spec.ID) {
			plan.shapeSpec = spec
			plan.shape = spec.ID
		}
	}
	return plan
}

func qAddPipelineOperand(plan *qPipelinePlan, role qPipelineOperandRole, expr string, dst *qScriptBindingPlan) {
	if expr == "" {
		return
	}
	*dst = buildQPipelineBindingPlan(expr)
	plan.operands = append(plan.operands, qPipelineOperandPlan{role: role, expr: expr, plan: *dst})
}

func qPipelinePlanShapeSpec(plan qPipelinePlan) qPipelineShapeSpec {
	if plan.shapeSpec.valid() {
		return plan.shapeSpec
	}
	if spec, ok := qPipelineShapeSpecForPlan(plan.kind, plan.shapeVariant()); ok && (plan.shape == "" || spec.ID == plan.shape) {
		return spec
	}
	return qPipelineShapeSpec{
		ID:            plan.shape,
		PipelineShape: qRuntimeKernelPipelineShape("QPipelinePlan", plan.shape),
	}
}

func (plan qPipelinePlan) shapeVariant() string {
	switch plan.kind {
	case qPipelineSumUnaryPrimitive, qPipelineWhereUnaryCompareIndexes:
		return plan.unaryOp
	default:
		return plan.compareOp
	}
}

func (plan qPipelinePlan) stableShape() string {
	if spec := qPipelinePlanShapeSpec(plan); spec.ID != "" {
		return spec.ID
	}
	return plan.shape
}

func (plan qPipelinePlan) stablePipelineShape() string {
	if spec := qPipelinePlanShapeSpec(plan); spec.PipelineShape != "" {
		return spec.PipelineShape
	}
	return qRuntimeKernelPipelineShape("QPipelinePlan", plan.stableShape())
}
