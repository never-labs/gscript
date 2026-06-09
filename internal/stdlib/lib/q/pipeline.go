package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qPipelineKind uint8

const (
	qPipelineInvalid qPipelineKind = iota
	qPipelineSumWhereMask
	qPipelineSumWhereIndex
	qPipelineSumGatherIndexes
	qPipelineSumWhereCompare
	qPipelineCountWhereCompare
	qPipelineWhereCompareIndexes
	qPipelineSumDeltas
)

type qPipelinePlan struct {
	kind           qPipelineKind
	shape          string
	valueExpr      string
	indexExpr      string
	maskExpr       string
	leftExpr       string
	rightExpr      string
	compareOp      string
	comparePrefix  string
	reductionInput string
}

func (s *EvalState) qPipelinePlan(src string) qPipelinePlan {
	src = strings.TrimSpace(src)
	if src == "" || !qPipelinePlanCandidate(src) {
		return qPipelinePlan{}
	}
	if s.pipelineCache != nil {
		if plan, ok := s.pipelineCache[src]; ok {
			return plan
		}
	}
	plan := buildQPipelinePlan(src)
	if s.pipelineCache == nil {
		s.pipelineCache = make(map[string]qPipelinePlan, 32)
	} else if len(s.pipelineCache) >= 512 {
		s.pipelineCache = make(map[string]qPipelinePlan, 32)
	}
	s.pipelineCache[src] = plan
	return plan
}

func (s *EvalState) rememberQPipelinePlan(src string, plan qPipelinePlan) {
	src = strings.TrimSpace(src)
	if src == "" || plan.kind == qPipelineInvalid {
		return
	}
	if s.pipelineCache != nil {
		if _, ok := s.pipelineCache[src]; ok {
			return
		}
	}
	if s.pipelineCache == nil {
		s.pipelineCache = make(map[string]qPipelinePlan, 32)
	} else if len(s.pipelineCache) >= 512 {
		s.pipelineCache = make(map[string]qPipelinePlan, 32)
	}
	s.pipelineCache[src] = plan
}

func qPipelinePlanCandidate(src string) bool {
	if strings.HasPrefix(src, "+/") {
		return true
	}
	if strings.HasPrefix(src, "sum ") && wordBoundary(src, 0, len("sum")) {
		return true
	}
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		return true
	}
	if strings.HasPrefix(src, "where ") && wordBoundary(src, 0, len("where")) {
		return true
	}
	return false
}

func buildQPipelinePlan(src string) qPipelinePlan {
	src = strings.TrimSpace(src)
	if src == "" {
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "+/") {
		right := strings.TrimSpace(src[2:])
		if right == "" {
			return qPipelinePlan{}
		}
		if plan, ok := buildQPipelineSumGatherPlan(right); ok {
			return plan
		}
		if plan, ok := buildQPipelineWhereComparePlan(right, qPipelineSumWhereCompare, "compare-to-index-sum"); ok {
			return plan
		}
		if input, ok := qPipelineDeltasInput(right); ok {
			return qPipelinePlan{kind: qPipelineSumDeltas, shape: "vector-reduce/sum-deltas", reductionInput: input}
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "sum ") && wordBoundary(src, 0, len("sum")) {
		if input, ok := qPipelineDeltasInput(strings.TrimSpace(src[len("sum "):])); ok {
			return qPipelinePlan{kind: qPipelineSumDeltas, shape: "vector-reduce/sum-deltas", reductionInput: input}
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		if plan, ok := buildQPipelineWhereComparePlan(strings.TrimSpace(src[len("count "):]), qPipelineCountWhereCompare, "compare-to-index-count"); ok {
			return plan
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "where ") && wordBoundary(src, 0, len("where")) {
		if plan, ok := buildQPipelineWhereComparePlan(src, qPipelineWhereCompareIndexes, "compare-to-index"); ok {
			return plan
		}
	}
	return qPipelinePlan{}
}

func buildQPipelineSumGatherPlan(src string) (qPipelinePlan, bool) {
	if valueExpr, maskExpr, ok := splitTopLevelWord(src, "where"); ok {
		return qPipelinePlan{
			kind:      qPipelineSumWhereMask,
			shape:     "where-reduce/sum",
			valueExpr: strings.TrimSpace(valueExpr),
			maskExpr:  strings.TrimSpace(maskExpr),
		}, true
	}
	collectionExpr, indexExpr, ok := findPostfixIndex(src)
	if !ok {
		return qPipelinePlan{}, false
	}
	plan := qPipelinePlan{
		kind:      qPipelineSumGatherIndexes,
		shape:     "gather-reduce/sum",
		valueExpr: strings.TrimSpace(collectionExpr),
		indexExpr: strings.TrimSpace(indexExpr),
	}
	if maskExpr, ok := directWhereMaskExpr(indexExpr); ok {
		plan.kind = qPipelineSumWhereIndex
		plan.shape = "where-index-reduce/sum"
		plan.maskExpr = strings.TrimSpace(maskExpr)
	}
	return plan, true
}

func buildQPipelineWhereComparePlan(src string, kind qPipelineKind, prefix string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "where ") || !wordBoundary(src, 0, len("where")) {
		return qPipelinePlan{}, false
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(arg)
	if !ok {
		return qPipelinePlan{}, false
	}
	if _, ok := qDataCompareOpString(op); !ok {
		return qPipelinePlan{}, false
	}
	return qPipelinePlan{
		kind:          kind,
		shape:         prefix,
		leftExpr:      strings.TrimSpace(leftExpr),
		rightExpr:     strings.TrimSpace(rightExpr),
		compareOp:     op,
		comparePrefix: prefix,
	}, true
}

func qPipelineDeltasInput(src string) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasPrefix(src, "deltas ") || !wordBoundary(src, 0, len("deltas")) {
		return "", false
	}
	input := strings.TrimSpace(src[len("deltas "):])
	if input == "" {
		return "", false
	}
	return input, true
}

func (s *EvalState) evalQPipelinePlan(plan qPipelinePlan) (any, bool, error) {
	if plan.kind == qPipelineInvalid {
		return nil, false, nil
	}
	recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "attempt", "attempt")
	var (
		out     any
		handled bool
		err     error
	)
	switch plan.kind {
	case qPipelineSumWhereMask:
		out, handled, err = s.evalQPipelineSumWhereMask(plan)
	case qPipelineSumWhereIndex:
		out, handled, err = s.evalQPipelineSumWhereIndex(plan)
	case qPipelineSumGatherIndexes:
		out, handled, err = s.evalQPipelineSumGatherIndexes(plan)
	case qPipelineSumWhereCompare:
		out, handled, err = s.evalQPipelineSumWhereCompare(plan)
	case qPipelineCountWhereCompare:
		out, handled, err = s.evalQPipelineCountWhereCompare(plan)
	case qPipelineWhereCompareIndexes:
		out, handled, err = s.evalQPipelineWhereCompareIndexes(plan)
	case qPipelineSumDeltas:
		out, handled, err = s.evalQPipelineSumDeltas(plan)
	default:
		return nil, false, nil
	}
	switch {
	case err != nil:
		recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "error", "runtime_error")
	case handled:
		recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "hit", "typed_pipeline")
	default:
		recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "fallback", "unsupported_runtime_shape")
	}
	return out, handled, err
}

func (s *EvalState) evalQPipelineSumWhereMask(plan qPipelinePlan) (any, bool, error) {
	value, err := s.eval(plan.valueExpr)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	maskValue, err := s.eval(plan.maskExpr)
	if err != nil {
		return nil, true, err
	}
	mask, ok := maskValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
	shape := "where-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
	recordRuntimeKernelProbe("ArrayWhereReduceSum", shape, handled, err)
	return out, handled, err
}

func (s *EvalState) evalQPipelineSumWhereIndex(plan qPipelinePlan) (any, bool, error) {
	value, err := s.eval(plan.valueExpr)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	maskValue, err := s.eval(plan.maskExpr)
	if err != nil {
		return nil, true, err
	}
	mask, ok := maskValue.(data.Array)
	if ok {
		out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
		shape := "where-index-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
		recordRuntimeKernelProbe("ArrayWhereReduceSum", shape, handled, err)
		if err != nil || handled {
			return out, handled, err
		}
	}
	indexValue, err := s.eval(plan.indexExpr)
	if err != nil {
		return nil, true, err
	}
	indexes, ok := indexValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	return qPipelineGatherReduceSum(array, indexes)
}

func (s *EvalState) evalQPipelineSumGatherIndexes(plan qPipelinePlan) (any, bool, error) {
	value, err := s.eval(plan.valueExpr)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	indexValue, err := s.eval(plan.indexExpr)
	if err != nil {
		return nil, true, err
	}
	indexes, ok := indexValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	return qPipelineGatherReduceSum(array, indexes)
}

func qPipelineGatherReduceSum(array, indexes data.Array) (any, bool, error) {
	out, handled, err := data.TryTypedNumericSumByI64Indexes(array, indexes)
	shape := "gather-reduce/" + string(array.Kind()) + "/" + string(indexes.Kind())
	if array.Kind() == data.KindI64 && indexes.Kind() == data.KindI64 {
		shape = "gather-reduce/i64/i64"
	}
	recordRuntimeKernelProbe("ArrayGatherReduceSum", shape, handled, err)
	return out, handled, err
}

func qPipelineGatherReduceSumWithPlanStats(plan qPipelinePlan, array, indexes data.Array) (any, bool, error) {
	recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "attempt", "attempt")
	out, handled, err := qPipelineGatherReduceSum(array, indexes)
	recordQPipelinePlanOutcome(plan, handled, err)
	return out, handled, err
}

func qPipelineWhereReduceSumWithPlanStats(plan qPipelinePlan, array, mask data.Array) (any, bool, error) {
	recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "attempt", "attempt")
	out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
	shape := "where-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
	if array.Kind() == data.KindI64 && mask.Kind() == data.KindBool {
		shape = "where-reduce/i64/bool"
	}
	recordRuntimeKernelProbe("ArrayWhereReduceSum", shape, handled, err)
	recordQPipelinePlanOutcome(plan, handled, err)
	return out, handled, err
}

func recordQPipelinePlanOutcome(plan qPipelinePlan, handled bool, err error) {
	switch {
	case err != nil:
		recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "error", "runtime_error")
	case handled:
		recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "hit", "typed_pipeline")
	default:
		recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "fallback", "unsupported_runtime_shape")
	}
}

func (s *EvalState) evalQPipelineSumWhereCompare(plan qPipelinePlan) (any, bool, error) {
	_, sum, handled, err := s.evalQPipelineWhereCompareIndexStats(plan)
	if err != nil || handled {
		return sum, handled, err
	}
	indexes, handled, err := s.evalQPipelineWhereCompareIndexesArray(plan)
	if err != nil || !handled {
		return nil, handled, err
	}
	out, handled, err := data.TryTypedNumericSum(indexes)
	recordRuntimeKernelProbe("ArrayWhereCompareSum", "index-sum/"+string(indexes.Kind()), handled, err)
	return out, handled, err
}

func (s *EvalState) evalQPipelineCountWhereCompare(plan qPipelinePlan) (any, bool, error) {
	count, _, handled, err := s.evalQPipelineWhereCompareIndexStats(plan)
	if err != nil || handled {
		return count, handled, err
	}
	indexes, handled, err := s.evalQPipelineWhereCompareIndexesArray(plan)
	if err != nil || !handled {
		return nil, handled, err
	}
	return int64(indexes.Len()), true, nil
}

func (s *EvalState) evalQPipelineWhereCompareIndexes(plan qPipelinePlan) (any, bool, error) {
	return s.evalQPipelineWhereCompareIndexesArray(plan)
}

func (s *EvalState) evalQPipelineWhereCompareIndexStats(plan qPipelinePlan) (count, sum int64, handled bool, err error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return 0, 0, true, err
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, plan.compareOp)
	if !ok {
		return 0, 0, false, nil
	}
	shape := plan.comparePrefix + "-stats/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, sum, handled, err = data.TryTypedCompareIndexStatsI64(array, dataOp, scalar)
	recordRuntimeKernelProbe("ArrayWhereCompareStats", shape, handled, err)
	if err != nil || !handled {
		return 0, 0, handled, err
	}
	return count, sum, true, nil
}

func (s *EvalState) evalQPipelineWhereCompareIndexesArray(plan qPipelinePlan) (data.Array, bool, error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return nil, true, err
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, plan.compareOp)
	if !ok {
		return nil, false, nil
	}
	shape := plan.comparePrefix + "/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	out, handled, err := data.TryTypedCompareIndexesI64(array, dataOp, scalar)
	recordRuntimeKernelProbe("ArrayWhereCompare", shape, handled, err)
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func (s *EvalState) evalQPipelineCompareOperands(plan qPipelinePlan) (any, any, error) {
	left, err := s.eval(plan.leftExpr)
	if err != nil {
		return nil, nil, err
	}
	right, err := s.eval(plan.rightExpr)
	if err != nil {
		return nil, nil, err
	}
	return left, right, nil
}

func (s *EvalState) evalQPipelineSumDeltas(plan qPipelinePlan) (any, bool, error) {
	value, err := s.eval(plan.reductionInput)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		delta, err := deltas(value)
		if err != nil {
			return nil, true, err
		}
		out, err := sum(delta)
		return out, true, err
	}
	shape := "vector-reduce/sum-deltas/" + string(array.Kind())
	if out, handled, err := data.TryTypedDeltasSum(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayDeltasSum", shape, handled, err)
		return out, handled, err
	} else {
		recordRuntimeKernelProbe("ArrayDeltasSum", shape, handled, err)
	}
	return nil, false, nil
}
