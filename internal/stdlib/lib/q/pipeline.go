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
	qPipelineSumWhereModuloCompare
	qPipelineCountWhereModuloCompare
	qPipelineWhereModuloCompareIndexes
	qPipelineSumDeltas
	qPipelineSumBin
	qPipelineSumVectorExpr
	qPipelineCountVectorExpr
)

type qPipelinePlan struct {
	kind           qPipelineKind
	shape          string
	valueExpr      string
	valuePlan      qScriptBindingPlan
	indexExpr      string
	indexPlan      qScriptBindingPlan
	maskExpr       string
	maskPlan       qScriptBindingPlan
	leftExpr       string
	leftPlan       qScriptBindingPlan
	rightExpr      string
	rightPlan      qScriptBindingPlan
	compareOp      string
	comparePrefix  string
	modExpr        string
	modPlan        qScriptBindingPlan
	modulusExpr    string
	modulusPlan    qScriptBindingPlan
	modTargetExpr  string
	modTargetPlan  qScriptBindingPlan
	reductionInput string
	reductionPlan  qScriptBindingPlan
	moduloMaskPlan *qPipelinePlan
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
			return qPipelinePlanWithBindingPlans(plan)
		}
		if plan, ok := buildQPipelineSumBinPlan(right); ok {
			return qPipelinePlanWithBindingPlans(plan)
		}
		if plan, ok := buildQPipelineWhereComparePlan(right, qPipelineSumWhereCompare, "compare-to-index-sum"); ok {
			return qPipelinePlanWithBindingPlans(plan)
		}
		if input, ok := qPipelineDeltasInput(right); ok {
			return qPipelinePlanWithBindingPlans(qPipelinePlan{kind: qPipelineSumDeltas, shape: "vector-reduce/sum-deltas", reductionInput: input})
		}
		if qPipelineVectorTransformExprCandidate(right) {
			return qPipelinePlanWithBindingPlans(qPipelinePlan{kind: qPipelineSumVectorExpr, shape: "vector-reduce/sum-expr", reductionInput: right})
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "sum ") && wordBoundary(src, 0, len("sum")) {
		inputExpr := strings.TrimSpace(src[len("sum "):])
		if input, ok := qPipelineDeltasInput(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(qPipelinePlan{kind: qPipelineSumDeltas, shape: "vector-reduce/sum-deltas", reductionInput: input})
		}
		if qPipelineVectorTransformExprCandidate(inputExpr) {
			return qPipelinePlanWithBindingPlans(qPipelinePlan{kind: qPipelineSumVectorExpr, shape: "vector-reduce/sum-expr", reductionInput: inputExpr})
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		inputExpr := strings.TrimSpace(src[len("count "):])
		if plan, ok := buildQPipelineWhereComparePlan(inputExpr, qPipelineCountWhereCompare, "compare-to-index-count"); ok {
			return qPipelinePlanWithBindingPlans(plan)
		}
		if strings.HasPrefix(inputExpr, "where ") && wordBoundary(inputExpr, 0, len("where")) {
			return qPipelinePlan{}
		}
		if strings.HasPrefix(inputExpr, "reverse ") && wordBoundary(inputExpr, 0, len("reverse")) {
			return qPipelinePlan{}
		}
		if qPipelineVectorTransformExprCandidate(inputExpr) {
			return qPipelinePlanWithBindingPlans(qPipelinePlan{kind: qPipelineCountVectorExpr, shape: "vector-count/expr", reductionInput: inputExpr})
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "where ") && wordBoundary(src, 0, len("where")) {
		if plan, ok := buildQPipelineWhereComparePlan(src, qPipelineWhereCompareIndexes, "compare-to-index"); ok {
			return qPipelinePlanWithBindingPlans(plan)
		}
	}
	return qPipelinePlan{}
}

func qPipelinePlanWithBindingPlans(plan qPipelinePlan) qPipelinePlan {
	if plan.valueExpr != "" {
		plan.valuePlan = buildQScriptBindingPlanForRHS(plan.valueExpr, nil)
	}
	if plan.indexExpr != "" {
		plan.indexPlan = buildQScriptBindingPlanForRHS(plan.indexExpr, nil)
	}
	if plan.maskExpr != "" {
		plan.maskPlan = buildQScriptBindingPlanForRHS(plan.maskExpr, nil)
		if modPlan, ok := qPipelineModuloComparePlanFromMask(plan.maskExpr); ok {
			plan.moduloMaskPlan = &modPlan
		}
	}
	if plan.leftExpr != "" {
		plan.leftPlan = buildQScriptBindingPlanForRHS(plan.leftExpr, nil)
	}
	if plan.rightExpr != "" {
		plan.rightPlan = buildQScriptBindingPlanForRHS(plan.rightExpr, nil)
	}
	if plan.modExpr != "" {
		plan.modPlan = buildQScriptBindingPlanForRHS(plan.modExpr, nil)
	}
	if plan.modulusExpr != "" {
		plan.modulusPlan = buildQScriptBindingPlanForRHS(plan.modulusExpr, nil)
	}
	if plan.modTargetExpr != "" {
		plan.modTargetPlan = buildQScriptBindingPlanForRHS(plan.modTargetExpr, nil)
	}
	if plan.reductionInput != "" {
		plan.reductionPlan = buildQScriptBindingPlanForRHS(plan.reductionInput, nil)
	}
	return plan
}

func qPipelineVectorTransformExprCandidate(src string) bool {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if src == "" {
		return false
	}
	for _, prefix := range []string{
		"reverse ",
		"til ",
		"drop ",
	} {
		if strings.HasPrefix(src, prefix) && wordBoundary(src, 0, len(strings.TrimSpace(prefix))) {
			return true
		}
	}
	for _, word := range []string{"rotate", "where", "bin", "xbar"} {
		if _, _, ok := splitTopLevelWord(src, word); ok {
			return true
		}
	}
	if len(splitTopLevel(src, '#')) > 1 {
		return true
	}
	if len(splitTopLevel(src, '_')) > 1 {
		return true
	}
	if _, _, ok := findPostfixIndex(src); ok {
		return true
	}
	return false
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

func buildQPipelineSumBinPlan(src string) (qPipelinePlan, bool) {
	leftExpr, rightExpr, ok := splitTopLevelWord(src, "bin")
	if !ok {
		return qPipelinePlan{}, false
	}
	return qPipelinePlan{
		kind:      qPipelineSumBin,
		shape:     "bin-reduce/sum",
		leftExpr:  strings.TrimSpace(leftExpr),
		rightExpr: strings.TrimSpace(rightExpr),
	}, true
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
	if plan, ok := buildQPipelineWhereModuloComparePlan(leftExpr, rightExpr, op, kind, prefix); ok {
		return plan, true
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

func buildQPipelineWhereModuloComparePlan(leftExpr, rightExpr, op string, kind qPipelineKind, prefix string) (qPipelinePlan, bool) {
	dataOp, ok := qDataCompareOpString(op)
	if !ok || (dataOp != data.OpEQ && dataOp != data.OpNE) {
		return qPipelinePlan{}, false
	}
	modExpr, modulusExpr, ok := splitQPipelineModExpr(leftExpr)
	targetExpr := strings.TrimSpace(rightExpr)
	compareOp := op
	if !ok {
		modExpr, modulusExpr, ok = splitQPipelineModExpr(rightExpr)
		if !ok {
			return qPipelinePlan{}, false
		}
		targetExpr = strings.TrimSpace(leftExpr)
		compareOp = qReverseCompareOpString(op)
	}
	modKind := qPipelineWhereModuloCompareIndexes
	switch kind {
	case qPipelineCountWhereCompare:
		modKind = qPipelineCountWhereModuloCompare
	case qPipelineSumWhereCompare:
		modKind = qPipelineSumWhereModuloCompare
	case qPipelineWhereCompareIndexes:
		modKind = qPipelineWhereModuloCompareIndexes
	default:
		return qPipelinePlan{}, false
	}
	return qPipelinePlan{
		kind:          modKind,
		shape:         prefix + "-mod",
		compareOp:     compareOp,
		comparePrefix: prefix + "-mod",
		modExpr:       modExpr,
		modulusExpr:   modulusExpr,
		modTargetExpr: targetExpr,
	}, true
}

func splitQPipelineModExpr(src string) (string, string, bool) {
	src = stripEnclosedParens(strings.TrimSpace(src))
	left, right, ok := splitTopLevelWord(src, "mod")
	if !ok {
		return "", "", false
	}
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return "", "", false
	}
	return left, right, true
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
	case qPipelineSumWhereModuloCompare:
		out, handled, err = s.evalQPipelineSumWhereModuloCompare(plan)
	case qPipelineCountWhereModuloCompare:
		out, handled, err = s.evalQPipelineCountWhereModuloCompare(plan)
	case qPipelineWhereModuloCompareIndexes:
		out, handled, err = s.evalQPipelineWhereModuloCompareIndexes(plan)
	case qPipelineSumDeltas:
		out, handled, err = s.evalQPipelineSumDeltas(plan)
	case qPipelineSumBin:
		out, handled, err = s.evalQPipelineSumBin(plan)
	case qPipelineSumVectorExpr:
		out, handled, err = s.evalQPipelineSumVectorExpr(plan)
	case qPipelineCountVectorExpr:
		out, handled, err = s.evalQPipelineCountVectorExpr(plan)
	default:
		recordRuntimeKernelExecution("QPipelinePlan", plan.shape, "fallback", RuntimeFallbackPlannerUnhandled)
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
	value, err := s.evalQPipelinePlannedExpr(plan.valueExpr, &plan.valuePlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	maskValue, err := s.evalQPipelinePlannedExpr(plan.maskExpr, &plan.maskPlan)
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
	value, err := s.evalQPipelinePlannedExpr(plan.valueExpr, &plan.valuePlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	if plan.moduloMaskPlan != nil {
		if out, handled, err := s.evalQPipelineModuloCompareValueSum(*plan.moduloMaskPlan, array); err != nil || handled {
			return out, handled, err
		}
	}
	maskValue, err := s.evalQPipelinePlannedExpr(plan.maskExpr, &plan.maskPlan)
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
	indexValue, err := s.evalQPipelinePlannedExpr(plan.indexExpr, &plan.indexPlan)
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
	value, err := s.evalQPipelinePlannedExpr(plan.valueExpr, &plan.valuePlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	indexValue, err := s.evalQPipelinePlannedExpr(plan.indexExpr, &plan.indexPlan)
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
	shape := ""
	if array.Kind() == data.KindI64 && indexes.Kind() == data.KindI64 {
		shape = "gather-reduce/i64/i64"
	} else {
		shape = "gather-reduce/" + string(array.Kind()) + "/" + string(indexes.Kind())
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
	shape := ""
	if array.Kind() == data.KindI64 && mask.Kind() == data.KindBool {
		shape = "where-reduce/i64/bool"
	} else {
		shape = "where-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
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
	count, handled, err := s.evalQPipelineWhereCompareCount(plan)
	if err != nil || handled {
		return count, handled, err
	}
	count, _, handled, err = s.evalQPipelineWhereCompareIndexStats(plan)
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

func (s *EvalState) evalQPipelineSumWhereModuloCompare(plan qPipelinePlan) (any, bool, error) {
	_, sum, handled, err := s.evalQPipelineWhereModuloCompareIndexStats(plan)
	if err != nil || handled {
		return sum, handled, err
	}
	return nil, false, nil
}

func (s *EvalState) evalQPipelineCountWhereModuloCompare(plan qPipelinePlan) (any, bool, error) {
	count, _, handled, err := s.evalQPipelineWhereModuloCompareIndexStats(plan)
	if err != nil || handled {
		return count, handled, err
	}
	return nil, false, nil
}

func (s *EvalState) evalQPipelineWhereModuloCompareIndexes(plan qPipelinePlan) (any, bool, error) {
	array, modulus, target, dataOp, handled, err := s.evalQPipelineModuloCompareOperands(plan)
	if err != nil || !handled {
		return nil, handled, err
	}
	out, handled, err := data.TryTypedModuloCompareIndexesI64(array, modulus, dataOp, target)
	shape := plan.comparePrefix + "/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	recordRuntimeKernelProbe("ArrayModuloCompare", shape, handled, err)
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func (s *EvalState) evalQPipelineWhereModuloCompareIndexStats(plan qPipelinePlan) (count, sum int64, handled bool, err error) {
	array, modulus, target, dataOp, handled, err := s.evalQPipelineModuloCompareOperands(plan)
	if err != nil || !handled {
		return 0, 0, handled, err
	}
	count, sum, handled, err = data.TryTypedModuloCompareIndexStatsI64(array, modulus, dataOp, target)
	shape := plan.comparePrefix + "-stats/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	recordRuntimeKernelProbe("ArrayModuloCompareStats", shape, handled, err)
	if err != nil || !handled {
		return 0, 0, handled, err
	}
	return count, sum, true, nil
}

func (s *EvalState) evalQPipelineModuloCompareValueSum(plan qPipelinePlan, values data.Array) (any, bool, error) {
	array, modulus, target, dataOp, handled, err := s.evalQPipelineModuloCompareOperands(plan)
	if err != nil || !handled {
		return nil, handled, err
	}
	out, handled, err := data.TryTypedNumericSumWhereModuloCompare(values, array, modulus, dataOp, target)
	shape := "where-mod-reduce/" + string(values.Kind()) + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	recordRuntimeKernelProbe("ArrayModuloCompareReduceSum", shape, handled, err)
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func (s *EvalState) evalQPipelineModuloCompareOperands(plan qPipelinePlan) (data.Array, any, any, data.Op, bool, error) {
	source, err := s.evalQPipelinePlannedExpr(plan.modExpr, &plan.modPlan)
	if err != nil {
		return nil, nil, nil, "", true, err
	}
	array, ok := source.(data.Array)
	if !ok {
		return nil, nil, nil, "", false, nil
	}
	modulus, err := s.evalQPipelinePlannedExpr(plan.modulusExpr, &plan.modulusPlan)
	if err != nil {
		return nil, nil, nil, "", true, err
	}
	target, err := s.evalQPipelinePlannedExpr(plan.modTargetExpr, &plan.modTargetPlan)
	if err != nil {
		return nil, nil, nil, "", true, err
	}
	dataOp, ok := qDataCompareOpString(plan.compareOp)
	if !ok {
		return nil, nil, nil, "", false, nil
	}
	return array, modulus, target, dataOp, true, nil
}

func qPipelineModuloComparePlanFromMask(maskExpr string) (qPipelinePlan, bool) {
	src := "where " + strings.TrimSpace(maskExpr)
	plan, ok := buildQPipelineWhereModuloComparePlanFromWhere(src, qPipelineWhereCompareIndexes, "compare-to-index")
	if !ok {
		return qPipelinePlan{}, false
	}
	return qPipelinePlanWithBindingPlans(plan), true
}

func buildQPipelineWhereModuloComparePlanFromWhere(src string, kind qPipelineKind, prefix string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "where ") || !wordBoundary(src, 0, len("where")) {
		return qPipelinePlan{}, false
	}
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(strings.TrimSpace(src[len("where "):]))
	if !ok {
		return qPipelinePlan{}, false
	}
	return buildQPipelineWhereModuloComparePlan(leftExpr, rightExpr, op, kind, prefix)
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

func (s *EvalState) evalQPipelineWhereCompareCount(plan qPipelinePlan) (count int64, handled bool, err error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return 0, true, err
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, plan.compareOp)
	if !ok {
		return 0, false, nil
	}
	shape := "compare-count/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, handled, err = data.TryTypedCompareCount(array, dataOp, scalar)
	recordRuntimeKernelProbe("ArrayWhereCompareCount", shape, handled, err)
	if err != nil || !handled {
		return 0, handled, err
	}
	return count, true, nil
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
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, nil, err
	}
	right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, nil, err
	}
	return left, right, nil
}

func (s *EvalState) evalQPipelineSumDeltas(plan qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
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

func (s *EvalState) evalQPipelineSumBin(plan qPipelinePlan) (any, bool, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	domain, ok := left.(data.Array)
	if !ok {
		return nil, false, nil
	}
	query, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	out, handled, err := data.TryTypedBinSum(domain, query)
	shape := "bin-reduce/sum/" + string(domain.Kind()) + "/" + string(qRuntimeKernelOperandKind(query, nil))
	recordRuntimeKernelProbe("ArrayBinReduceSum", shape, handled, err)
	return out, handled, err
}

func (s *EvalState) evalQPipelineSumVectorExpr(plan qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	if _, ok := numeric(value); ok {
		return value, true, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	out, handled, err := data.TryTypedNumericSum(array)
	shape := "vector-reduce/sum-expr/" + string(array.Kind())
	recordRuntimeKernelProbe("ArraySumExpr", shape, handled, err)
	return out, handled, err
}

func (s *EvalState) evalQPipelineCountVectorExpr(plan qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-count/expr/" + string(array.Kind())
	recordRuntimeKernelProbe("ArrayCountExpr", shape, true, nil)
	return int64(array.Len()), true, nil
}

func (s *EvalState) evalQPipelinePlannedExpr(src string, plan *qScriptBindingPlan) (any, error) {
	if plan != nil && plan.kind != qScriptBindingInvalid {
		if value, handled, err := s.evalQScriptBindingPlan(plan); err != nil || handled {
			return value, err
		}
	}
	return s.eval(src)
}
