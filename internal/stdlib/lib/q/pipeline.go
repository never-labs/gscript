package q

import (
	"fmt"
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
	qPipelineCountDistinct
	qPipelineCountWhereIn
	qPipelineSumMovingWindow
	qPipelineCountRunningScan
)

type qPipelinePlan struct {
	source         string
	kind           qPipelineKind
	shape          string
	shapeSpec      qPipelineShapeSpec
	operands       []qPipelineOperandPlan
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

type qPipelineOperandRole string

const (
	qPipelineOperandValue     qPipelineOperandRole = "value"
	qPipelineOperandIndex     qPipelineOperandRole = "index"
	qPipelineOperandMask      qPipelineOperandRole = "mask"
	qPipelineOperandLeft      qPipelineOperandRole = "left"
	qPipelineOperandRight     qPipelineOperandRole = "right"
	qPipelineOperandMod       qPipelineOperandRole = "mod"
	qPipelineOperandModulus   qPipelineOperandRole = "modulus"
	qPipelineOperandModTarget qPipelineOperandRole = "mod_target"
	qPipelineOperandReduction qPipelineOperandRole = "reduction"
)

type qPipelineOperandPlan struct {
	role qPipelineOperandRole
	expr string
	plan qScriptBindingPlan
}

type qPipelineOperandFingerprint struct {
	role  qPipelineOperandRole
	kind  data.Kind
	class string
}

type qPipelineBindingCacheKey struct {
	source      string
	kind        qPipelineKind
	shape       string
	fingerprint string
}

type qPipelineBoundPlan struct {
	key            qPipelineBindingCacheKey
	resultClass    string
	resultKind     data.Kind
	kernel         string
	kernelShape    string
	fallbackReason string
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
	if qPipelinePlanGlobalCacheable(src) {
		if plan, ok := qGlobalPipelinePlanCacheProbe(src); ok {
			s.rememberQPipelinePlan(src, plan)
			return plan
		}
	}
	plan := buildQPipelinePlan(src)
	if qPipelinePlanGlobalCacheable(src) {
		qGlobalPipelinePlanCacheStore(src, plan)
	}
	if s.pipelineCache == nil {
		s.pipelineCache = make(map[string]qPipelinePlan, 32)
	} else if len(s.pipelineCache) >= 512 {
		s.pipelineCache = make(map[string]qPipelinePlan, 32)
	}
	s.pipelineCache[src] = plan
	return plan
}

func qPipelinePlanGlobalCacheable(src string) bool {
	return EvalSourceCacheable(src)
}

func qGlobalPipelinePlanCacheProbe(src string) (qPipelinePlan, bool) {
	qGlobalScriptPlanCacheMu.Lock()
	plan, ok := qGlobalPipelinePlanCache[src]
	if ok {
		qGlobalScriptPlanStats.PipelineHits++
	} else {
		qGlobalScriptPlanStats.PipelineMisses++
	}
	qGlobalScriptPlanCacheMu.Unlock()
	if !ok {
		return qPipelinePlan{}, false
	}
	return cloneQPipelinePlan(plan), true
}

func qGlobalPipelinePlanCacheStore(src string, plan qPipelinePlan) {
	if src == "" || plan.kind == qPipelineInvalid {
		return
	}
	qGlobalScriptPlanCacheMu.Lock()
	if _, ok := qGlobalPipelinePlanCache[src]; !ok {
		qGlobalPipelinePlanCacheOrder = append(qGlobalPipelinePlanCacheOrder, src)
	}
	qGlobalPipelinePlanCache[src] = cloneQPipelinePlan(plan)
	qGlobalScriptPlanStats.PipelineStores++
	for len(qGlobalPipelinePlanCacheOrder) > qGlobalPipelinePlanCacheLimit {
		evict := qGlobalPipelinePlanCacheOrder[0]
		qGlobalPipelinePlanCacheOrder = qGlobalPipelinePlanCacheOrder[1:]
		delete(qGlobalPipelinePlanCache, evict)
		qGlobalScriptPlanStats.PipelineEvictions++
	}
	qGlobalScriptPlanCacheMu.Unlock()
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
	withSource := func(plan qPipelinePlan) qPipelinePlan {
		plan.source = src
		return plan
	}
	if strings.HasPrefix(src, "+/") {
		right := strings.TrimSpace(src[2:])
		if right == "" {
			return qPipelinePlan{}
		}
		if plan, ok := buildQPipelineSumGatherPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumBinPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineWhereComparePlan(right, qPipelineSumWhereCompare, "compare-to-index-sum"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if input, ok := qPipelineDeltasInput(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineSumDeltas, shape: "vector-reduce/sum-deltas", reductionInput: input}))
		}
		if plan, ok := buildQPipelineSumMovingWindowPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if qPipelineVectorTransformExprCandidate(right) {
			plan := qPipelineShapePlan(qPipelineSumVectorExpr, "")
			plan.reductionInput = right
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "sum ") && wordBoundary(src, 0, len("sum")) {
		inputExpr := strings.TrimSpace(src[len("sum "):])
		if input, ok := qPipelineDeltasInput(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineSumDeltas, shape: "vector-reduce/sum-deltas", reductionInput: input}))
		}
		if plan, ok := buildQPipelineSumMovingWindowPlan(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if qPipelineVectorTransformExprCandidate(inputExpr) {
			plan := qPipelineShapePlan(qPipelineSumVectorExpr, "")
			plan.reductionInput = inputExpr
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		inputExpr := strings.TrimSpace(src[len("count "):])
		if plan, ok := buildQPipelineWhereComparePlan(inputExpr, qPipelineCountWhereCompare, "compare-to-index-count"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineWhereInPlan(inputExpr, qPipelineCountWhereIn, "in-count"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if strings.HasPrefix(inputExpr, "distinct ") && wordBoundary(inputExpr, 0, len("distinct")) {
			arg := strings.TrimSpace(inputExpr[len("distinct "):])
			if arg != "" {
				return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineCountDistinct, shape: "distinct-count", reductionInput: arg}))
			}
		}
		if scan, arg, ok := qPipelineRunningScanInput(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineCountRunningScan, shape: "vector-count/" + scan, compareOp: scan, reductionInput: arg}))
		}
		if strings.HasPrefix(inputExpr, "where ") && wordBoundary(inputExpr, 0, len("where")) {
			return qPipelinePlan{}
		}
		if strings.HasPrefix(inputExpr, "reverse ") && wordBoundary(inputExpr, 0, len("reverse")) {
			return qPipelinePlan{}
		}
		if qPipelineVectorTransformExprCandidate(inputExpr) {
			plan := qPipelineShapePlan(qPipelineCountVectorExpr, "")
			plan.reductionInput = inputExpr
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "where ") && wordBoundary(src, 0, len("where")) {
		if plan, ok := buildQPipelineWhereComparePlan(src, qPipelineWhereCompareIndexes, "compare-to-index"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
	}
	return qPipelinePlan{}
}

func buildQPipelineSumMovingWindowPlan(src string) (qPipelinePlan, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, word := range []string{"msum", "mavg", "mcount", "mmin", "mmax"} {
		left, right, ok := splitTopLevelWord(src, word)
		if !ok {
			continue
		}
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		if left == "" || right == "" {
			return qPipelinePlan{}, false
		}
		plan := qPipelineShapePlan(qPipelineSumMovingWindow, word)
		plan.compareOp = word
		plan.leftExpr = left
		plan.rightExpr = right
		return plan, true
	}
	return qPipelinePlan{}, false
}

func qPipelineRunningScanInput(src string) (scan, arg string, ok bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, spec := range []struct {
		prefix string
		scan   string
	}{
		{"+\\", "sums"},
		{"sums ", "sums"},
		{"prds ", "prds"},
		{"mins ", "mins"},
		{"maxs ", "maxs"},
		{"avgs ", "avgs"},
	} {
		if !strings.HasPrefix(src, spec.prefix) {
			continue
		}
		arg = strings.TrimSpace(src[len(spec.prefix):])
		if arg == "" || (spec.prefix == "+\\" && strings.HasPrefix(arg, "[")) {
			return "", "", false
		}
		return spec.scan, arg, true
	}
	return "", "", false
}

func qPipelinePlanWithBindingPlans(plan qPipelinePlan) qPipelinePlan {
	if !plan.shapeSpec.valid() {
		if spec, ok := qPipelineShapeSpecForPlan(plan.kind, plan.compareOp); ok && (plan.shape == "" || plan.shape == spec.ID) {
			plan.shapeSpec = spec
			plan.shape = spec.ID
		}
	}
	if plan.valueExpr != "" {
		plan.valuePlan = buildQPipelineBindingPlan(plan.valueExpr)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandValue, expr: plan.valueExpr, plan: plan.valuePlan})
	}
	if plan.indexExpr != "" {
		plan.indexPlan = buildQPipelineBindingPlan(plan.indexExpr)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandIndex, expr: plan.indexExpr, plan: plan.indexPlan})
	}
	if plan.maskExpr != "" {
		plan.maskPlan = buildQPipelineBindingPlan(plan.maskExpr)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandMask, expr: plan.maskExpr, plan: plan.maskPlan})
		if modPlan, ok := qPipelineModuloComparePlanFromMask(plan.maskExpr); ok {
			plan.moduloMaskPlan = &modPlan
		}
	}
	if plan.leftExpr != "" {
		plan.leftPlan = buildQPipelineBindingPlan(plan.leftExpr)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandLeft, expr: plan.leftExpr, plan: plan.leftPlan})
	}
	if plan.rightExpr != "" {
		plan.rightPlan = buildQPipelineBindingPlan(plan.rightExpr)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandRight, expr: plan.rightExpr, plan: plan.rightPlan})
	}
	if plan.modExpr != "" {
		plan.modPlan = buildQPipelineBindingPlan(plan.modExpr)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandMod, expr: plan.modExpr, plan: plan.modPlan})
	}
	if plan.modulusExpr != "" {
		plan.modulusPlan = buildQPipelineBindingPlan(plan.modulusExpr)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandModulus, expr: plan.modulusExpr, plan: plan.modulusPlan})
	}
	if plan.modTargetExpr != "" {
		plan.modTargetPlan = buildQPipelineBindingPlan(plan.modTargetExpr)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandModTarget, expr: plan.modTargetExpr, plan: plan.modTargetPlan})
	}
	if plan.reductionInput != "" {
		plan.reductionPlan = buildQPipelineBindingPlan(plan.reductionInput)
		plan.operands = append(plan.operands, qPipelineOperandPlan{role: qPipelineOperandReduction, expr: plan.reductionInput, plan: plan.reductionPlan})
	}
	return plan
}

func buildQPipelineBindingPlan(src string) qScriptBindingPlan {
	if expr, ok, err := parseValueExpr(src); err == nil && ok {
		if plan := buildQScriptBindingPlanForRHS(src, expr); plan.kind != qScriptBindingInvalid {
			return plan
		}
	}
	return buildQScriptBindingPlanForRHS(src, nil)
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
		plan := qPipelineShapePlan(qPipelineSumWhereMask, "")
		plan.valueExpr = strings.TrimSpace(valueExpr)
		plan.maskExpr = strings.TrimSpace(maskExpr)
		return plan, true
	}
	collectionExpr, indexExpr, ok := findPostfixIndex(src)
	if !ok {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineSumGatherIndexes, "")
	plan.valueExpr = strings.TrimSpace(collectionExpr)
	plan.indexExpr = strings.TrimSpace(indexExpr)
	if maskExpr, ok := directWhereMaskExpr(indexExpr); ok {
		plan = qPipelineShapePlan(qPipelineSumWhereIndex, "")
		plan.valueExpr = strings.TrimSpace(collectionExpr)
		plan.indexExpr = strings.TrimSpace(indexExpr)
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
	if op != "within" {
		if _, ok := qDataCompareOpString(op); !ok {
			return qPipelinePlan{}, false
		}
	}
	if op == "within" && (strings.TrimSpace(leftExpr) == "" || strings.TrimSpace(rightExpr) == "") {
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

func buildQPipelineWhereInPlan(src string, kind qPipelineKind, prefix string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "where ") || !wordBoundary(src, 0, len("where")) {
		return qPipelinePlan{}, false
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, ok := splitTopLevelWord(arg, "in")
	if !ok {
		return qPipelinePlan{}, false
	}
	leftExpr = strings.TrimSpace(leftExpr)
	rightExpr = strings.TrimSpace(rightExpr)
	if leftExpr == "" || rightExpr == "" {
		return qPipelinePlan{}, false
	}
	return qPipelinePlan{
		kind:      kind,
		shape:     prefix,
		leftExpr:  leftExpr,
		rightExpr: rightExpr,
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
	shape := plan.stableShape()
	recordRuntimeKernelExecution("QPipelinePlan", shape, "attempt", "attempt")
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
	case qPipelineCountDistinct:
		out, handled, err = s.evalQPipelineCountDistinct(plan)
	case qPipelineCountWhereIn:
		out, handled, err = s.evalQPipelineCountWhereIn(plan)
	case qPipelineSumMovingWindow:
		out, handled, err = s.evalQPipelineSumMovingWindow(plan)
	case qPipelineCountRunningScan:
		out, handled, err = s.evalQPipelineCountRunningScan(plan)
	default:
		recordRuntimeKernelExecution("QPipelinePlan", shape, "fallback", RuntimeFallbackPlannerUnhandled)
		return nil, false, nil
	}
	switch {
	case err != nil:
		recordRuntimeKernelExecution("QPipelinePlan", shape, "error", "runtime_error")
	case handled:
		recordRuntimeKernelExecution("QPipelinePlan", shape, "hit", "typed_pipeline")
	default:
		recordRuntimeKernelExecution("QPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
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
	shape := "where-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
	out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
	return qTypedRuntimeResult("ArrayWhereReduceSum", shape, out, handled, err)
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
	if array.Kind() == data.KindI64 && isIdentityI64RangeArray(array) {
		count, sum, handled, err := s.evalQPipelineCompareIndexStatsForMask(plan.maskExpr)
		if err != nil || handled {
			if handled {
				recordRuntimeKernelProbe("ArrayWhereCompareRangeReduceSum", "where-index-reduce/i64-range/compare-stats", handled, err)
			}
			return sum, handled, err
		}
		_ = count
	}
	maskValue, err := s.evalQPipelinePlannedExpr(plan.maskExpr, &plan.maskPlan)
	if err != nil {
		return nil, true, err
	}
	mask, ok := maskValue.(data.Array)
	if ok {
		shape := "where-index-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
		out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
		out, handled, err = qTypedRuntimeResult("ArrayWhereReduceSum", shape, out, handled, err)
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

func (s *EvalState) evalQPipelineCompareIndexStatsForMask(maskExpr string) (count, sum int64, handled bool, err error) {
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(strings.TrimSpace(maskExpr))
	if !ok {
		return 0, 0, false, nil
	}
	dataOp, ok := qDataCompareOpString(op)
	if !ok {
		return 0, 0, false, nil
	}
	left, err := s.evalQPipelinePlannedExpr(leftExpr, nil)
	if err != nil {
		return 0, 0, true, err
	}
	right, err := s.evalQPipelinePlannedExpr(rightExpr, nil)
	if err != nil {
		return 0, 0, true, err
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return 0, 0, false, nil
	}
	shape := "where-index-stats/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, sum, handled, err = data.TryTypedCompareIndexStatsI64(array, dataOp, scalar)
	count, sum, handled, err = qTypedRuntimeResult2("ArrayWhereCompareStats", shape, count, sum, handled, err)
	return count, sum, handled, err
}

func isIdentityI64RangeArray(array data.Array) bool {
	if array == nil || array.Kind() != data.KindI64 {
		return false
	}
	if array.Len() == 0 {
		return true
	}
	first, ok := array.At(0)
	if !ok {
		return false
	}
	firstI, ok := integerValue(first)
	if !ok || firstI != 0 {
		return false
	}
	last, ok := array.At(array.Len() - 1)
	if !ok {
		return false
	}
	lastI, ok := integerValue(last)
	return ok && lastI == int64(array.Len()-1)
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
	if isIdentityI64RangeArray(array) {
		if view, ok := indexes.(qCompareIndexStatsView); ok {
			recordRuntimeKernelProbe("ArrayGatherReduceSum", "gather-reduce/i64-range/compare-index-view", true, nil)
			return view.sum, true, nil
		}
	}
	shape := ""
	if array.Kind() == data.KindI64 && indexes.Kind() == data.KindI64 {
		shape = "gather-reduce/i64/i64"
	} else {
		shape = "gather-reduce/" + string(array.Kind()) + "/" + string(indexes.Kind())
	}
	out, handled, err := data.TryTypedNumericSumByI64Indexes(array, indexes)
	return qTypedRuntimeResult("ArrayGatherReduceSum", shape, out, handled, err)
}

func qPipelineGatherReduceSumWithPlanStats(plan qPipelinePlan, array, indexes data.Array) (any, bool, error) {
	recordRuntimeKernelExecution("QPipelinePlan", plan.stableShape(), "attempt", "attempt")
	out, handled, err := qPipelineGatherReduceSum(array, indexes)
	recordQPipelinePlanOutcome(plan, handled, err)
	return out, handled, err
}

func qPipelineWhereReduceSumWithPlanStats(plan qPipelinePlan, array, mask data.Array) (any, bool, error) {
	recordRuntimeKernelExecution("QPipelinePlan", plan.stableShape(), "attempt", "attempt")
	shape := ""
	if array.Kind() == data.KindI64 && mask.Kind() == data.KindBool {
		shape = "where-reduce/i64/bool"
	} else {
		shape = "where-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
	}
	out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
	out, handled, err = qTypedRuntimeResult("ArrayWhereReduceSum", shape, out, handled, err)
	recordQPipelinePlanOutcome(plan, handled, err)
	return out, handled, err
}

func recordQPipelinePlanOutcome(plan qPipelinePlan, handled bool, err error) {
	shape := plan.stableShape()
	switch {
	case err != nil:
		recordRuntimeKernelExecution("QPipelinePlan", shape, "error", "runtime_error")
	case handled:
		recordRuntimeKernelExecution("QPipelinePlan", shape, "hit", "typed_pipeline")
	default:
		recordRuntimeKernelExecution("QPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
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
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayWhereCompareSum",
		shape:  "index-sum/" + string(indexes.Kind()),
		call: func() (any, bool, error) {
			return data.TryTypedNumericSum(indexes)
		},
	})
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
	shape := plan.comparePrefix + "/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel: "ArrayModuloCompare",
		shape:  shape,
		call: func() (data.Array, bool, error) {
			return data.TryTypedModuloCompareIndexesI64(array, modulus, dataOp, target)
		},
	})
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
	shape := plan.comparePrefix + "-stats/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	count, sum, handled, err = data.TryTypedModuloCompareIndexStatsI64(array, modulus, dataOp, target)
	count, sum, handled, err = qTypedRuntimeResult2("ArrayModuloCompareStats", shape, count, sum, handled, err)
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
	shape := "where-mod-reduce/" + string(values.Kind()) + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayModuloCompareReduceSum",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedNumericSumWhereModuloCompare(values, array, modulus, dataOp, target)
		},
	})
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
	if plan.compareOp == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return 0, 0, ok, err
		}
		shape := "within-to-index-" + strings.TrimPrefix(plan.comparePrefix, "compare-to-index-") + "-stats/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		count, sum, handled, err = evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
			kernel: "ArrayWhereWithinStats",
			shape:  shape,
			call: func() (int64, int64, bool, error) {
				return data.TryTypedWithinIndexStatsI64(array, low, high, true)
			},
		})
		if err != nil || !handled {
			return 0, 0, handled, err
		}
		return count, sum, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, plan.compareOp)
	if !ok {
		return 0, 0, false, nil
	}
	shape := plan.comparePrefix + "-stats/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, sum, handled, err = evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
		kernel: "ArrayWhereCompareStats",
		shape:  shape,
		call: func() (int64, int64, bool, error) {
			return data.TryTypedCompareIndexStatsI64(array, dataOp, scalar)
		},
	})
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
	if plan.compareOp == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return 0, ok, err
		}
		shape := "within-count/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		count, handled, err = evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
			kernel: "ArrayWhereWithinCount",
			shape:  shape,
			call: func() (int64, bool, error) {
				return data.TryTypedWithinCount(array, low, high, true)
			},
		})
		if err != nil || !handled {
			return 0, handled, err
		}
		return count, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, plan.compareOp)
	if !ok {
		return 0, false, nil
	}
	shape := "compare-count/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, handled, err = evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: "ArrayWhereCompareCount",
		shape:  shape,
		call: func() (int64, bool, error) {
			return data.TryTypedCompareCount(array, dataOp, scalar)
		},
	})
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
	if plan.compareOp == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil || !ok {
			return nil, ok, err
		}
		shape := "within-to-index/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
			kernel: "ArrayWhereWithin",
			shape:  shape,
			call: func() (data.Array, bool, error) {
				return data.TryTypedWithinIndexesI64(array, low, high, true)
			},
		})
		if err != nil || !handled {
			return nil, handled, err
		}
		return out, true, nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, plan.compareOp)
	if !ok {
		return nil, false, nil
	}
	shape := plan.comparePrefix + "/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel: "ArrayWhereCompare",
		shape:  shape,
		call: func() (data.Array, bool, error) {
			return data.TryTypedCompareIndexesI64(array, dataOp, scalar)
		},
	})
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
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayDeltasSum",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedDeltasSum(array)
		},
	})
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
	shape := "bin-reduce/sum/" + string(domain.Kind()) + "/" + string(qRuntimeKernelOperandKind(query, nil))
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayBinReduceSum",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedBinSum(domain, query)
		},
	})
}

func (s *EvalState) evalQPipelineSumVectorExpr(plan qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	bindingKey := qPipelineBindingKey(plan, []qPipelineOperandFingerprint{
		qPipelineOperandFingerprintForValue(qPipelineOperandReduction, value),
	})
	if bound, ok := qGlobalPipelineBindingCacheProbe(bindingKey); ok {
		return s.evalQPipelineSumVectorExprBound(bound, value)
	}
	if _, ok := numeric(value); ok {
		qGlobalPipelineBindingCacheStore(qPipelineBoundPlan{
			key:         bindingKey,
			resultClass: "scalar",
			resultKind:  qRuntimeKernelOperandKind(value, nil),
		})
		return value, true, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-reduce/sum-expr/" + string(array.Kind())
	bound := qPipelineBoundPlan{
		key:            bindingKey,
		resultClass:    "array",
		resultKind:     array.Kind(),
		kernel:         "ArraySumExpr",
		kernelShape:    shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
	}
	qGlobalPipelineBindingCacheStore(bound)
	return s.evalQPipelineSumVectorExprBound(bound, value)
}

func (s *EvalState) evalQPipelineSumVectorExprBound(bound qPipelineBoundPlan, value any) (any, bool, error) {
	if bound.resultClass == "scalar" {
		if _, ok := numeric(value); !ok {
			return nil, false, nil
		}
		return value, true, nil
	}
	array, ok := value.(data.Array)
	if !ok || array.Kind() != bound.resultKind {
		return nil, false, nil
	}
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArraySumExpr",
		shape:          bound.kernelShape,
		fallbackReason: bound.fallbackReason,
		call: func() (any, bool, error) {
			return data.TryTypedNumericSum(array)
		},
	})
}

func (s *EvalState) evalQPipelineSumMovingWindow(plan qPipelinePlan) (any, bool, error) {
	widthValue, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	width, ok := integerValue(widthValue)
	if !ok || width <= 0 || int64(int(width)) != width {
		return nil, true, errMovingWindowWidth(plan.compareOp)
	}
	value, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-reduce/sum-" + plan.compareOp + "/" + string(array.Kind())
	var (
		out     any
		handled bool
	)
	switch plan.compareOp {
	case "mcount":
		out, handled, err = data.TryTypedMCountSum(array, int(width))
	case "msum":
		out, handled, err = data.TryTypedMovingNumericSumSum(array, int(width), false)
	case "mavg":
		out, handled, err = data.TryTypedMovingNumericSumSum(array, int(width), true)
	case "mmin":
		out, handled, err = data.TryTypedMovingMinMaxSum(array, int(width), false)
	case "mmax":
		out, handled, err = data.TryTypedMovingMinMaxSum(array, int(width), true)
	default:
		return nil, false, nil
	}
	recordQTypedRuntimeKernel("ArrayMovingWindowSum", shape, handled, err)
	return out, handled, err
}

func errMovingWindowWidth(name string) error {
	return fmt.Errorf("%s width must be a positive integer", name)
}

func (s *EvalState) evalQPipelineCountRunningScan(plan qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-count/" + plan.compareOp + "/" + string(array.Kind())
	kernel := "ArrayCountRunningScan"
	switch plan.compareOp {
	case "sums", "prds", "avgs":
		if !qKindIsNumeric(array.Kind()) {
			err := fmt.Errorf("%s expects a numeric vector", plan.compareOp)
			recordQTypedRuntimeKernel(kernel, shape, false, err)
			return nil, true, err
		}
	case "mins", "maxs":
		if !qTypedCompareKindOK(array.Kind()) {
			err := fmt.Errorf("%s expects an ordered vector", plan.compareOp)
			recordQTypedRuntimeKernel(kernel, shape, false, err)
			return nil, true, err
		}
	default:
		return nil, false, nil
	}
	switch plan.compareOp {
	case "sums":
		kernel = "ArrayCountSums"
	case "prds":
		kernel = "ArrayCountProducts"
	case "mins":
		kernel = "ArrayCountMins"
	case "maxs":
		kernel = "ArrayCountMaxs"
	case "avgs":
		kernel = "ArrayCountAvgs"
	}
	recordQTypedRuntimeKernel(kernel, shape, true, nil)
	return int64(array.Len()), true, nil
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
	recordQTypedRuntimeKernel("ArrayCountExpr", shape, true, nil)
	return int64(array.Len()), true, nil
}

func (s *EvalState) evalQPipelineCountDistinct(plan qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		out, err := count(value)
		return out, true, err
	}
	shape := "distinct-count/" + string(array.Kind())
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayDistinctCount",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedDistinctCount(array)
		},
	})
}

func (s *EvalState) evalQPipelineCountWhereIn(plan qPipelinePlan) (any, bool, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := left.(data.Array)
	if !ok {
		return nil, false, nil
	}
	right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	values, err := setItems(right)
	if err != nil {
		return nil, true, err
	}
	shape := "in-count/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayInCount",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedInCount(array, values)
		},
	})
}

func (s *EvalState) evalQPipelinePlannedExpr(src string, plan *qScriptBindingPlan) (any, error) {
	if plan != nil && plan.kind != qScriptBindingInvalid {
		if value, handled, err := s.evalQScriptBindingPlan(plan); err != nil || handled {
			return value, err
		}
	}
	return s.eval(src)
}

func qPipelineOperandFingerprintForValue(role qPipelineOperandRole, value any) qPipelineOperandFingerprint {
	if array, ok := value.(data.Array); ok {
		return qPipelineOperandFingerprint{role: role, kind: array.Kind(), class: "array"}
	}
	if _, ok := numeric(value); ok {
		return qPipelineOperandFingerprint{role: role, kind: qRuntimeKernelOperandKind(value, nil), class: "scalar"}
	}
	if _, ok := value.(data.Frame); ok {
		return qPipelineOperandFingerprint{role: role, kind: data.KindAny, class: "frame"}
	}
	if _, ok := value.(data.KeyedFrame); ok {
		return qPipelineOperandFingerprint{role: role, kind: data.KindAny, class: "keyed_frame"}
	}
	return qPipelineOperandFingerprint{role: role, kind: qRuntimeKernelOperandKind(value, nil), class: "value"}
}

func qPipelineBindingKey(plan qPipelinePlan, operands []qPipelineOperandFingerprint) qPipelineBindingCacheKey {
	source := plan.source
	if source == "" {
		source = plan.shape
	}
	var b strings.Builder
	for i, operand := range operands {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(string(operand.role))
		b.WriteByte(':')
		b.WriteString(operand.class)
		b.WriteByte(':')
		b.WriteString(string(operand.kind))
	}
	return qPipelineBindingCacheKey{
		source:      source,
		kind:        plan.kind,
		shape:       plan.shape,
		fingerprint: b.String(),
	}
}

func qGlobalPipelineBindingCacheProbe(key qPipelineBindingCacheKey) (qPipelineBoundPlan, bool) {
	if key.source == "" || key.fingerprint == "" {
		return qPipelineBoundPlan{}, false
	}
	qGlobalScriptPlanCacheMu.Lock()
	bound, ok := qGlobalPipelineBindingCache[key]
	if ok {
		qGlobalScriptPlanStats.PipelineBindingHits++
	} else {
		qGlobalScriptPlanStats.PipelineBindingMisses++
	}
	qGlobalScriptPlanCacheMu.Unlock()
	if !ok {
		return qPipelineBoundPlan{}, false
	}
	return bound, true
}

func qGlobalPipelineBindingCacheStore(bound qPipelineBoundPlan) {
	key := bound.key
	if key.source == "" || key.fingerprint == "" {
		return
	}
	qGlobalScriptPlanCacheMu.Lock()
	if _, ok := qGlobalPipelineBindingCache[key]; !ok {
		qGlobalPipelineBindingCacheOrder = append(qGlobalPipelineBindingCacheOrder, key)
	}
	qGlobalPipelineBindingCache[key] = bound
	qGlobalScriptPlanStats.PipelineBindingStores++
	for len(qGlobalPipelineBindingCacheOrder) > qGlobalPipelineBindingCacheLimit {
		evict := qGlobalPipelineBindingCacheOrder[0]
		qGlobalPipelineBindingCacheOrder = qGlobalPipelineBindingCacheOrder[1:]
		delete(qGlobalPipelineBindingCache, evict)
		qGlobalScriptPlanStats.PipelineBindingEvictions++
	}
	qGlobalScriptPlanCacheMu.Unlock()
}
