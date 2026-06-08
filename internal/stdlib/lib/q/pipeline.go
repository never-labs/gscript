package q

import "github.com/never-labs/leia/internal/stdlib/lib/data"

type qPipelineKind string

const (
	qPipelineWhereReduceSum      qPipelineKind = "where-reduce/sum"
	qPipelineWhereIndexReduceSum qPipelineKind = "where-index-reduce/sum"
	qPipelineGatherReduceSum     qPipelineKind = "gather-reduce/sum"
)

type qPipelinePlan struct {
	kind    qPipelineKind
	values  data.Array
	mask    data.Array
	indexes data.Array
}

func (p qPipelinePlan) shape() string {
	switch p.kind {
	case qPipelineWhereReduceSum:
		return string(p.kind) + "/" + string(p.values.Kind()) + "/" + string(p.mask.Kind())
	case qPipelineWhereIndexReduceSum:
		return string(p.kind) + "/" + string(p.values.Kind()) + "/" + string(p.mask.Kind())
	case qPipelineGatherReduceSum:
		return string(p.kind) + "/" + string(p.values.Kind()) + "/" + string(p.indexes.Kind())
	default:
		return "unknown"
	}
}

func (s *EvalState) buildQPipelineSumPlan(src string) (qPipelinePlan, bool, error) {
	if valueExpr, maskExpr, ok := splitTopLevelWord(src, "where"); ok {
		value, err := s.eval(valueExpr)
		if err != nil {
			return qPipelinePlan{}, true, err
		}
		array, ok := value.(data.Array)
		if !ok {
			return qPipelinePlan{}, false, nil
		}
		maskValue, err := s.eval(maskExpr)
		if err != nil {
			return qPipelinePlan{}, true, err
		}
		mask, ok := maskValue.(data.Array)
		if !ok {
			return qPipelinePlan{}, false, nil
		}
		return qPipelinePlan{kind: qPipelineWhereReduceSum, values: array, mask: mask}, true, nil
	}
	collectionExpr, indexExpr, ok := findPostfixIndex(src)
	if !ok {
		return qPipelinePlan{}, false, nil
	}
	value, err := s.eval(collectionExpr)
	if err != nil {
		return qPipelinePlan{}, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return qPipelinePlan{}, false, nil
	}
	if maskExpr, ok := directWhereMaskExpr(indexExpr); ok {
		maskValue, err := s.eval(maskExpr)
		if err != nil {
			return qPipelinePlan{}, true, err
		}
		mask, ok := maskValue.(data.Array)
		if ok {
			return qPipelinePlan{kind: qPipelineWhereIndexReduceSum, values: array, mask: mask}, true, nil
		}
	}
	indexValue, err := s.eval(indexExpr)
	if err != nil {
		return qPipelinePlan{}, true, err
	}
	indexes, ok := indexValue.(data.Array)
	if !ok {
		return qPipelinePlan{}, false, nil
	}
	return qPipelinePlan{kind: qPipelineGatherReduceSum, values: array, indexes: indexes}, true, nil
}

func executeQPipelinePlan(plan qPipelinePlan) (any, bool, error) {
	switch plan.kind {
	case qPipelineWhereReduceSum:
		out, handled, err := data.TryTypedNumericSumWhereMask(plan.values, plan.mask)
		recordRuntimeKernelProbe("QPipelinePlan", plan.shape(), handled, err)
		shape := "where-reduce/" + string(plan.values.Kind()) + "/" + string(plan.mask.Kind())
		recordRuntimeKernelProbe("ArrayWhereReduceSum", shape, handled, err)
		return out, handled, err
	case qPipelineWhereIndexReduceSum:
		out, handled, err := data.TryTypedNumericSumWhereMask(plan.values, plan.mask)
		recordRuntimeKernelProbe("QPipelinePlan", plan.shape(), handled, err)
		shape := "where-index-reduce/" + string(plan.values.Kind()) + "/" + string(plan.mask.Kind())
		recordRuntimeKernelProbe("ArrayWhereReduceSum", shape, handled, err)
		return out, handled, err
	case qPipelineGatherReduceSum:
		out, handled, err := data.TryTypedNumericSumByI64Indexes(plan.values, plan.indexes)
		recordRuntimeKernelProbe("QPipelinePlan", plan.shape(), handled, err)
		shape := "gather-reduce/" + string(plan.values.Kind()) + "/" + string(plan.indexes.Kind())
		recordRuntimeKernelProbe("ArrayGatherReduceSum", shape, handled, err)
		return out, handled, err
	default:
		recordRuntimeKernelProbe("QPipelinePlan", plan.shape(), false, nil)
		return nil, false, nil
	}
}
