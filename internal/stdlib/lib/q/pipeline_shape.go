package q

type qPipelineShapeFamily string

const (
	qPipelineShapeFamilyUnknown qPipelineShapeFamily = ""
	qPipelineShapeFamilyWhere   qPipelineShapeFamily = "where"
	qPipelineShapeFamilyGather  qPipelineShapeFamily = "gather"
	qPipelineShapeFamilyVector  qPipelineShapeFamily = "vector"
)

type qPipelineShapeSpec struct {
	ID            string
	Family        qPipelineShapeFamily
	Reducer       string
	Selector      string
	Transform     string
	PipelineShape string
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
	default:
		return qPipelineShapeSpec{}, false
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

func qPipelinePlanShapeSpec(plan qPipelinePlan) qPipelineShapeSpec {
	if plan.shapeSpec.valid() {
		return plan.shapeSpec
	}
	if spec, ok := qPipelineShapeSpecForPlan(plan.kind, plan.compareOp); ok && (plan.shape == "" || spec.ID == plan.shape) {
		return spec
	}
	return qPipelineShapeSpec{
		ID:            plan.shape,
		PipelineShape: qRuntimeKernelPipelineShape("QPipelinePlan", plan.shape),
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
