package q

import (
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptPipelineKind string

const (
	qScriptPipelineWhereReduceSum       qScriptPipelineKind = "where-reduce/sum"
	qScriptPipelineWhereIndexReduceSum  qScriptPipelineKind = "where-index-reduce/sum"
	qScriptPipelineGatherReduceSum      qScriptPipelineKind = "gather-reduce/sum"
	qScriptPipelineGatherReduceSumCount qScriptPipelineKind = "gather-reduce/sum-count"
	qScriptPipelineFindReduceSum        qScriptPipelineKind = "find-reduce/sum"
	qScriptPipelineIndexExprSumCount    qScriptPipelineKind = "index-expr-reduce/sum-count"
	qScriptPipelineSequenceEdgeSum      qScriptPipelineKind = "sequence-edge-reduce/sum-first-last"
	qScriptPipelineSequenceSumCount     qScriptPipelineKind = "sequence-reduce/sum-count"
	qScriptPipelineSumPlusDyadicFloat   qScriptPipelineKind = "multi-reduce/sum-plus-dyadic-float-sum"
	qScriptPipelineIntegerDivModReduce  qScriptPipelineKind = "multi-reduce/integer-divmod-sum-count"
	qScriptPipelineMatrixRowSumCount    qScriptPipelineKind = "matrix-row-reduce/sum-count"
	qScriptPipelineMatrixRowsSumCount   qScriptPipelineKind = "matrix-rows-reduce/sum-plus-count"
	qScriptPipelineMatrixCellPlusCount  qScriptPipelineKind = "matrix-cell-reduce/cell-plus-count"
	qScriptPipelineMatrixNestedCell     qScriptPipelineKind = "matrix-nested-reduce/sum-cell-count"
	qScriptPipelineCallableDotSumRight  qScriptPipelineKind = "callable-dot/sum-plus-right"
	qScriptPipelineCallableDotSumCount  qScriptPipelineKind = "callable-dot/sum-plus-count-right"
	qScriptPipelineCallableOverScanSum  qScriptPipelineKind = "callable-over/scan-sum-count"
	qScriptPipelineStringJoinCounts     qScriptPipelineKind = "string-join/counts"
	qScriptPipelineApplyScalarAt        qScriptPipelineKind = "apply-index/scalar-at"
	qScriptPipelineApplyGatherAt        qScriptPipelineKind = "apply-index/gather-at"
	qScriptPipelineApplyScalarDot       qScriptPipelineKind = "apply-index/scalar-dot"
	qScriptPipelineApplyPathDot         qScriptPipelineKind = "apply-index/path-dot"
	qScriptPipelineUnsupported          qScriptPipelineKind = ""
)

type qScriptPipelineDescriptor struct {
	kind              qScriptPipelineKind
	shapeText         string
	assignments       []qScriptPipelineAssignment
	terminal          string
	valueExpr         string
	valueBinding      string
	valuePlan         qScriptBindingPlan
	indexExpr         string
	indexBinding      string
	indexPlan         qScriptBindingPlan
	maskExpr          string
	maskBinding       string
	maskPlan          qScriptBindingPlan
	rowValueExpr      string
	rowIndexExpr      string
	colIndexExpr      string
	rowValuePlan      qScriptBindingPlan
	rowIndexPlan      qScriptBindingPlan
	colIndexPlan      qScriptBindingPlan
	callableExpr      string
	callableBinding   string
	dyadicOp          string
	scalarExpr        string
	scalarPlan        qScriptBindingPlan
	scalarLeft        bool
	integerTerms      []qScriptPipelineIntegerDivModTerm
	indexExprPlan     data.I64IndexExpr
	indexReducers     []data.I64IndexExprReducer
	includeCount      bool
	sequenceValueExpr string
	sequenceValuePlan qScriptBindingPlan
	sequenceSteps     []data.SequenceTransformStep
	sequenceBindings  []string
	sequenceShapeName string
	stringValues      []string
	stringRepeatCount int
	stringSep         string
	stringSearch      string
	stringReplaceOld  string
	stringReplaceNew  string
	terminalUsesWhere bool
	terminalPlan      qPipelinePlan
	moduloMaskPlan    *qPipelinePlan
	// indexExprAssignmentSkip[i] marks assignments absorbed into the
	// vectorized index-expr reducer plan; precomputed at plan-build time.
	indexExprAssignmentSkip []bool
	// whereIndexPlanBuilt/whereIndexPlan memoize the purely syntactic
	// where-compare plan build of the gather-sum-count where-index route
	// (a per-call string walk plus operand normalization otherwise);
	// runtime operand evaluation still runs per call.
	whereIndexPlanBuilt bool
	whereIndexPlan      *qPipelinePlan
	absorbedAssignments []string
}

type qScriptPipelineAssignment struct {
	name      string
	rhs       string
	valueExpr Expr
	binding   qScriptBindingPlan
}

func (d qScriptPipelineDescriptor) shape() string {
	if d.shapeText != "" {
		return d.shapeText
	}
	if d.kind == qScriptPipelineUnsupported {
		return "script-pipeline/unsupported"
	}
	if (d.kind == qScriptPipelineSequenceEdgeSum || d.kind == qScriptPipelineSequenceSumCount) && len(d.sequenceSteps) > 0 {
		if len(d.assignments) == 0 {
			return "script-pipeline/" + string(d.kind) + "-transform-chain/direct"
		}
		return "script-pipeline/" + string(d.kind) + "-transform-chain/assignments"
	}
	if len(d.assignments) == 0 {
		return "script-pipeline/" + string(d.kind) + "/direct"
	}
	return "script-pipeline/" + string(d.kind) + "/assignments"
}

func buildQScriptPipelineDescriptor(statements []qScriptStatement) (*qScriptPipelineDescriptor, bool) {
	compact := make([]qScriptStatement, 0, len(statements))
	for _, stmt := range statements {
		if strings.TrimSpace(stmt.src) != "" {
			compact = append(compact, stmt)
		}
	}
	if len(compact) < 2 {
		return nil, false
	}
	terminal := compact[len(compact)-1]
	if terminal.assign != "" {
		return nil, false
	}
	assignments := make([]qScriptPipelineAssignment, 0, len(compact)-1)
	bindings := make(map[string]string, len(compact)-1)
	for _, stmt := range compact[:len(compact)-1] {
		if stmt.assign == "" || stmt.rhs == "" {
			return nil, false
		}
		assignments = append(assignments, qScriptPipelineAssignment{name: stmt.assign, rhs: stmt.rhs, valueExpr: stmt.valueExpr, binding: stmt.bindingPlan})
		bindings[stmt.assign] = stmt.rhs
	}
	descriptor, ok := describeQScriptPipelineTerminal(terminal.src, bindings)
	if !ok {
		return nil, false
	}
	descriptor.assignments = assignments
	descriptor.terminal = terminal.src
	descriptor.terminalPlan = buildQPipelinePlan(terminal.src)
	descriptor.sequenceShapeName = qScriptPipelineSequenceTransformName(descriptor.sequenceSteps)
	descriptor = qNormalizeScriptPipelineDescriptor(descriptor)
	return &descriptor, true
}

func qNormalizeScriptPipelineDescriptor(descriptor qScriptPipelineDescriptor) qScriptPipelineDescriptor {
	descriptor.terminalPlan = qScriptPipelineDescriptorTerminalPlan(descriptor)
	descriptor.valuePlan = buildQScriptBindingPlanForRHS(descriptor.valueExpr, nil)
	if descriptor.kind == qScriptPipelineCallableOverScanSum && strings.TrimSpace(descriptor.valueBinding) != "" {
		descriptor.valuePlan = buildQScriptWarmBindingPlan(descriptor.valueBinding, parseCachedValueExpr(descriptor.valueBinding))
	}
	descriptor.indexPlan = buildQScriptBindingPlanForRHS(descriptor.indexExpr, nil)
	if descriptor.kind == qScriptPipelineFindReduceSum {
		if binding := strings.TrimSpace(descriptor.valueBinding); binding != "" {
			descriptor.valuePlan = buildQScriptWarmBindingPlan(binding, parseCachedValueExpr(binding))
		}
		if binding := strings.TrimSpace(descriptor.indexBinding); binding != "" {
			descriptor.indexPlan = buildQScriptWarmBindingPlan(binding, parseCachedValueExpr(binding))
		}
	}
	descriptor.maskPlan = buildQScriptBindingPlanForRHS(descriptor.maskExpr, nil)
	descriptor.rowValuePlan = buildQScriptBindingPlanForRHS(descriptor.rowValueExpr, nil)
	descriptor.rowIndexPlan = buildQScriptBindingPlanForRHS(descriptor.rowIndexExpr, nil)
	descriptor.colIndexPlan = buildQScriptBindingPlanForRHS(descriptor.colIndexExpr, nil)
	descriptor.scalarPlan = buildQScriptBindingPlanForRHS(descriptor.scalarExpr, nil)
	descriptor.sequenceValuePlan = buildQScriptBindingPlanForRHS(descriptor.sequenceValueExpr, nil)
	descriptor.sequenceShapeName = qScriptPipelineSequenceTransformName(descriptor.sequenceSteps)
	descriptor.moduloMaskPlan = qScriptPipelineModuloMaskPlan(descriptor.maskExpr)
	descriptor.indexExprAssignmentSkip = qScriptPipelineIndexExprSkippedAssignments(&descriptor)
	descriptor.absorbedAssignments = qScriptPipelineAbsorbedAssignments(descriptor)
	descriptor.shapeText = descriptor.shape()
	return descriptor
}

// qScriptPipelineIndexExprSkippedAssignments precomputes which pipeline
// assignments the index-expr reducer terminal absorbs into its vectorized
// expression plan (and therefore never needs materialized in the
// environment). The analysis is a pure function of the descriptor's static
// expressions, so it runs once at plan-build time instead of re-walking the
// binding strings on every execution.
func qScriptPipelineIndexExprSkippedAssignments(descriptor *qScriptPipelineDescriptor) []bool {
	if descriptor.kind != qScriptPipelineIndexExprSumCount || len(descriptor.assignments) == 0 {
		return nil
	}
	bindings := make(map[string]string, len(descriptor.assignments))
	for _, assignment := range descriptor.assignments {
		bindings[assignment.name] = assignment.rhs
	}
	skip := make([]bool, len(descriptor.assignments))
	indexExpr := strings.TrimSpace(descriptor.indexExpr)
	for i, assignment := range descriptor.assignments {
		name := strings.TrimSpace(assignment.name)
		skip[i] = name == indexExpr ||
			qScriptPipelineI64IndexExprDependsOnName(descriptor.valueExpr, name, descriptor.indexExpr, bindings, nil)
	}
	return skip
}

func qScriptPipelineDescriptorTerminalPlan(descriptor qScriptPipelineDescriptor) qPipelinePlan {
	if descriptor.terminalPlan.kind != qPipelineInvalid {
		return qPipelinePlanWithBindingPlans(descriptor.terminalPlan)
	}
	switch descriptor.kind {
	case qScriptPipelineApplyScalarAt:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineApplyScalarIndex,
			shape:     "apply-index/scalar-at",
			compareOp: "at",
			valueExpr: descriptor.valueExpr,
			indexExpr: descriptor.indexExpr,
		})
	case qScriptPipelineApplyGatherAt:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineApplyGatherIndex,
			shape:     "apply-index/gather-at",
			compareOp: "at",
			valueExpr: descriptor.valueExpr,
			indexExpr: descriptor.indexExpr,
		})
	case qScriptPipelineApplyScalarDot:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineApplyScalarIndex,
			shape:     "apply-index/scalar-dot",
			compareOp: "dot",
			valueExpr: descriptor.valueExpr,
			indexExpr: descriptor.indexExpr,
		})
	case qScriptPipelineApplyPathDot:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineApplyScalarIndex,
			shape:     "apply-index/path-dot",
			compareOp: "dot",
			valueExpr: descriptor.valueExpr,
			indexExpr: descriptor.indexExpr,
		})
	case qScriptPipelineWhereIndexReduceSum:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineSumWhereIndex,
			shape:     "where-index-reduce/sum",
			valueExpr: descriptor.valueExpr,
			indexExpr: descriptor.indexExpr,
			maskExpr:  descriptor.maskExpr,
		})
	case qScriptPipelineWhereReduceSum:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineSumWhereMask,
			shape:     "where-reduce/sum",
			valueExpr: descriptor.valueExpr,
			maskExpr:  descriptor.maskExpr,
		})
	case qScriptPipelineGatherReduceSum:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineSumGatherIndexes,
			shape:     "gather-reduce/sum",
			valueExpr: descriptor.valueExpr,
			indexExpr: descriptor.indexExpr,
		})
	case qScriptPipelineGatherReduceSumCount:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineSumGatherIndexes,
			shape:     "gather-reduce/sum-count",
			valueExpr: descriptor.valueExpr,
			indexExpr: descriptor.indexExpr,
		})
	case qScriptPipelineFindReduceSum:
		return qPipelinePlanWithBindingPlans(qPipelinePlan{
			kind:      qPipelineFindSum,
			shape:     "vector-reduce/find-sum",
			leftExpr:  descriptor.valueExpr,
			rightExpr: descriptor.indexExpr,
		})
	default:
		if strings.TrimSpace(descriptor.terminal) != "" {
			if plan := buildQPipelinePlan(descriptor.terminal); plan.kind != qPipelineInvalid {
				return plan
			}
		}
		return qPipelinePlan{}
	}
}

func describeQScriptPipelineTerminal(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	src = strings.TrimSpace(src)
	if plan, ok := buildQPipelineApplyScalarIndexPlan(src); ok {
		kind := qScriptPipelineApplyScalarAt
		if plan.compareOp == "dot" {
			kind = qScriptPipelineApplyScalarDot
		}
		return qScriptPipelineDescriptor{
			kind:         kind,
			valueExpr:    plan.valueExpr,
			valueBinding: qScriptPipelineBinding(plan.valueExpr, bindings),
			indexExpr:    plan.indexExpr,
			indexBinding: qScriptPipelineBinding(plan.indexExpr, bindings),
		}, true
	}
	if plan, ok := buildQPipelineApplyGatherIndexPlan(src); ok {
		return qScriptPipelineDescriptor{
			kind:         qScriptPipelineApplyGatherAt,
			valueExpr:    plan.valueExpr,
			valueBinding: qScriptPipelineBinding(plan.valueExpr, bindings),
			indexExpr:    plan.indexExpr,
			indexBinding: qScriptPipelineBinding(plan.indexExpr, bindings),
		}, true
	}
	if plan, ok := buildQPipelineApplyPathIndexPlan(src); ok {
		return qScriptPipelineDescriptor{
			kind:         qScriptPipelineApplyPathDot,
			valueExpr:    plan.valueExpr,
			valueBinding: qScriptPipelineBinding(plan.valueExpr, bindings),
			indexExpr:    plan.indexExpr,
			indexBinding: qScriptPipelineBinding(plan.indexExpr, bindings),
		}, true
	}
	if valueExpr, ok := qScriptPipelineSequenceEdgeReduceValue(src); ok {
		baseExpr, steps, chainBindings, chainOK := qScriptPipelineSequenceTransformChain(valueExpr, bindings)
		if chainOK && len(steps) > 0 {
			return qScriptPipelineDescriptor{
				kind:              qScriptPipelineSequenceEdgeSum,
				valueExpr:         valueExpr,
				valueBinding:      qScriptPipelineBinding(valueExpr, bindings),
				sequenceValueExpr: baseExpr,
				sequenceSteps:     steps,
				sequenceBindings:  chainBindings,
			}, true
		}
		return qScriptPipelineDescriptor{
			kind:         qScriptPipelineSequenceEdgeSum,
			valueExpr:    valueExpr,
			valueBinding: qScriptPipelineBinding(valueExpr, bindings),
		}, true
	}
	if descriptor, ok := qScriptPipelineSequenceSumCountDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineIndexExprSumCountDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineGatherSumCountDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineSumPlusDyadicFloatDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineIntegerDivModDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineMatrixCellPlusCountDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineMatrixRowsSumCountDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineMatrixNestedCellDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if valueExpr, ok := qScriptPipelineSumCountReduceValue(src); ok {
		binding := qScriptPipelineBinding(valueExpr, bindings)
		if plan, ok := buildQPipelineApplyScalarIndexPlan(binding); ok {
			return qScriptPipelineDescriptor{
				kind:         qScriptPipelineMatrixRowSumCount,
				valueExpr:    valueExpr,
				valueBinding: binding,
				rowValueExpr: plan.valueExpr,
				rowIndexExpr: plan.indexExpr,
			}, true
		}
	}
	if descriptor, ok := qScriptPipelineCallableDotSumPlusRight(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineCallableOverScanSumDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineStringJoinCountsDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if !strings.HasPrefix(src, "+/") {
		return qScriptPipelineDescriptor{}, false
	}
	body := strings.TrimSpace(src[len("+/"):])
	if body == "" {
		return qScriptPipelineDescriptor{}, false
	}
	if plan, ok := buildQPipelineFindPlan(body, qPipelineFindSum, "vector-reduce/find-sum"); ok {
		return qScriptPipelineDescriptor{
			kind:         qScriptPipelineFindReduceSum,
			valueExpr:    plan.leftExpr,
			valueBinding: qScriptPipelineBinding(plan.leftExpr, bindings),
			indexExpr:    plan.rightExpr,
			indexBinding: qScriptPipelineBinding(plan.rightExpr, bindings),
		}, true
	}
	if valueExpr, maskExpr, ok := splitTopLevelWord(body, "where"); ok {
		d := qScriptPipelineDescriptor{
			kind:              qScriptPipelineWhereReduceSum,
			valueExpr:         valueExpr,
			valueBinding:      qScriptPipelineBinding(valueExpr, bindings),
			maskExpr:          maskExpr,
			maskBinding:       qScriptPipelineBinding(maskExpr, bindings),
			terminalUsesWhere: true,
		}
		return d, true
	}
	valueExpr, indexExpr, ok := findPostfixIndex(body)
	if !ok {
		return qScriptPipelineDescriptor{}, false
	}
	d := qScriptPipelineDescriptor{
		kind:         qScriptPipelineGatherReduceSum,
		valueExpr:    valueExpr,
		valueBinding: qScriptPipelineBinding(valueExpr, bindings),
		indexExpr:    indexExpr,
		indexBinding: qScriptPipelineBinding(indexExpr, bindings),
	}
	if maskExpr, ok := qScriptPipelineIndexMaskExpr(indexExpr, bindings); ok {
		d.kind = qScriptPipelineWhereIndexReduceSum
		d.maskExpr = maskExpr
		d.maskBinding = qScriptPipelineBinding(maskExpr, bindings)
	}
	return d, true
}

func qScriptPipelineSequenceTransformChain(valueExpr string, bindings map[string]string) (string, []data.SequenceTransformStep, []string, bool) {
	valueExpr = strings.TrimSpace(valueExpr)
	if valueExpr == "" || len(bindings) == 0 {
		return "", nil, nil, false
	}
	seen := make(map[string]bool, len(bindings))
	return qScriptPipelineSequenceTransformChainFromExpr(valueExpr, bindings, seen)
}

func qScriptPipelineSequenceTransformChainFromExpr(expr string, bindings map[string]string, seen map[string]bool) (string, []data.SequenceTransformStep, []string, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", nil, nil, false
	}
	if qScriptPipelineSimpleName(expr) {
		rhs, ok := bindings[expr]
		if !ok {
			return expr, nil, nil, true
		}
		if seen[expr] {
			return "", nil, nil, false
		}
		seen[expr] = true
		if step, inner, ok := qScriptPipelineSequenceTransformStep(rhs); ok {
			base, steps, names, ok := qScriptPipelineSequenceTransformChainFromExpr(inner, bindings, seen)
			if !ok {
				return "", nil, nil, false
			}
			steps = append(steps, step)
			names = append(names, expr)
			return base, steps, names, true
		}
		return expr, nil, nil, true
	}
	if step, inner, ok := qScriptPipelineSequenceTransformStep(expr); ok {
		base, steps, names, ok := qScriptPipelineSequenceTransformChainFromExpr(inner, bindings, seen)
		if !ok {
			return "", nil, nil, false
		}
		steps = append(steps, step)
		return base, steps, names, true
	}
	return expr, nil, nil, true
}

func qScriptPipelineSequenceTransformStep(src string) (data.SequenceTransformStep, string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return data.SequenceTransformStep{}, "", false
	}
	if strings.HasPrefix(src, "reverse ") && wordBoundary(src, 0, len("reverse")) {
		valueExpr := strings.TrimSpace(src[len("reverse "):])
		if valueExpr == "" {
			return data.SequenceTransformStep{}, "", false
		}
		return data.SequenceTransformStep{Transform: data.SequenceTransformReverse}, valueExpr, true
	}
	if strings.HasPrefix(src, "drop ") && wordBoundary(src, 0, len("drop")) {
		countExpr, valueExpr, ok := splitQScriptPrefixDyadicArgs(strings.TrimSpace(src[len("drop "):]))
		n, staticOK := qScriptPipelineStaticInt(countExpr)
		if !ok || !staticOK || strings.TrimSpace(valueExpr) == "" {
			return data.SequenceTransformStep{}, "", false
		}
		return data.SequenceTransformStep{Transform: data.SequenceTransformDrop, Args: [2]int{n}, ArgCount: 1}, strings.TrimSpace(valueExpr), true
	}
	if left, right, ok := splitTopLevelOperator(src, "#"); ok {
		n, staticOK := qScriptPipelineStaticInt(left)
		if !staticOK || strings.HasPrefix(strings.TrimSpace(left), "`") || strings.TrimSpace(right) == "" {
			return data.SequenceTransformStep{}, "", false
		}
		return data.SequenceTransformStep{Transform: data.SequenceTransformSublist, Args: [2]int{n}, ArgCount: 1}, strings.TrimSpace(right), true
	}
	if left, right, ok := splitTopLevelWord(src, "rotate"); ok {
		n, ok := qScriptPipelineStaticInt(left)
		if !ok || strings.TrimSpace(right) == "" {
			return data.SequenceTransformStep{}, "", false
		}
		return data.SequenceTransformStep{Transform: data.SequenceTransformRotate, Args: [2]int{n}, ArgCount: 1}, strings.TrimSpace(right), true
	}
	if left, right, ok := splitTopLevelWord(src, "sublist"); ok {
		args, ok := qScriptPipelineStaticIntArgs(left)
		if !ok || len(args) == 0 || len(args) > 2 || strings.TrimSpace(right) == "" {
			return data.SequenceTransformStep{}, "", false
		}
		// q's one-argument sublist clamps (no overtake); the data layer's
		// one-argument sublist step is take (#), so encode start/count.
		converted, ok := qSublistTransformArgs(args)
		if !ok {
			return data.SequenceTransformStep{}, "", false
		}
		step := data.SequenceTransformStep{Transform: data.SequenceTransformSublist, ArgCount: len(converted)}
		copy(step.Args[:], converted)
		return step, strings.TrimSpace(right), true
	}
	return data.SequenceTransformStep{}, "", false
}

func qScriptPipelineStaticInt(src string) (int, bool) {
	value, ok := qScriptPipelineStaticValue(src)
	if !ok {
		return 0, false
	}
	n, ok := integerValue(value)
	if !ok || int64(int(n)) != n {
		return 0, false
	}
	return int(n), true
}

func qScriptPipelineStaticIntArgs(src string) ([]int, bool) {
	value, ok := qScriptPipelineStaticValue(src)
	if !ok {
		return nil, false
	}
	indexes, err := qIntegerIndexes("sequence transform", value)
	if err != nil || len(indexes) == 0 || len(indexes) > 2 {
		return nil, false
	}
	return indexes, true
}

func qScriptPipelineStaticValue(src string) (any, bool) {
	plan := buildQScriptBindingPlanForRHS(src, nil)
	if plan.kind == qScriptBindingInvalid || !qScriptBindingPlanCacheable(&plan) {
		return nil, false
	}
	value, handled, err := NewEvalState(nil).evalQScriptBindingPlan(&plan)
	if err != nil || !handled {
		return nil, false
	}
	return value, true
}

func qScriptPipelineCallableDotSumPlusRight(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	plan := buildDotApplyPlan(src)
	if !plan.valid || plan.tupleArgs || len(plan.argExprs) != 2 {
		return qScriptPipelineDescriptor{}, false
	}
	callableExpr := strings.TrimSpace(plan.fnExpr)
	if !qScriptPipelineSimpleName(callableExpr) {
		return qScriptPipelineDescriptor{}, false
	}
	binding := strings.TrimSpace(bindings[callableExpr])
	if !strings.HasPrefix(binding, "{") || !strings.HasSuffix(binding, "}") {
		return qScriptPipelineDescriptor{}, false
	}
	body := strings.TrimSpace(binding[1 : len(binding)-1])
	fastPlan, ok := qLambdaFastPlanFor(body)
	if !ok || (fastPlan.kind != qLambdaFastSumPlusRight && fastPlan.kind != qLambdaFastSumPlusCountRight) {
		return qScriptPipelineDescriptor{}, false
	}
	leftExpr := strings.TrimSpace(plan.argExprs[0])
	rightExpr := strings.TrimSpace(plan.argExprs[1])
	if leftExpr == "" || rightExpr == "" {
		return qScriptPipelineDescriptor{}, false
	}
	kind := qScriptPipelineCallableDotSumRight
	if fastPlan.kind == qLambdaFastSumPlusCountRight {
		kind = qScriptPipelineCallableDotSumCount
	}
	return qScriptPipelineDescriptor{
		kind:            kind,
		callableExpr:    callableExpr,
		callableBinding: binding,
		valueExpr:       leftExpr,
		valueBinding:    qScriptPipelineBinding(leftExpr, bindings),
		indexExpr:       rightExpr,
		indexBinding:    qScriptPipelineBinding(rightExpr, bindings),
		includeCount:    fastPlan.kind == qLambdaFastSumPlusCountRight,
	}, true
}

func qScriptPipelineCallableOverScanSumDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 3 {
		return qScriptPipelineDescriptor{}, false
	}
	var sourceName string
	var initialExpr string
	var scanName string
	seenOver := false
	seenLast := false
	seenCount := false
	for _, term := range terms {
		if fnSrc, initSrc, valueSrc, ok := parseCallableOverApplication(term); ok {
			if seenOver || initSrc == "" || valueSrc == "" || !qScriptPipelineCallableAddExpr(fnSrc, bindings) {
				return qScriptPipelineDescriptor{}, false
			}
			sourceName = strings.TrimSpace(valueSrc)
			initialExpr = strings.TrimSpace(initSrc)
			seenOver = true
			continue
		}
		if name, ok := qScriptPipelineUnaryNameTerm(term, "last"); ok {
			if seenLast {
				return qScriptPipelineDescriptor{}, false
			}
			scanName = name
			seenLast = true
			continue
		}
		if name, ok := qScriptPipelineUnaryNameTerm(term, "count"); ok {
			if seenCount {
				return qScriptPipelineDescriptor{}, false
			}
			if scanName != "" && scanName != name {
				return qScriptPipelineDescriptor{}, false
			}
			scanName = name
			seenCount = true
			continue
		}
		return qScriptPipelineDescriptor{}, false
	}
	if !seenOver || !seenLast || !seenCount || sourceName == "" || scanName == "" {
		return qScriptPipelineDescriptor{}, false
	}
	scanRHS := strings.TrimSpace(bindings[scanName])
	if !strings.HasPrefix(scanRHS, "+\\") {
		return qScriptPipelineDescriptor{}, false
	}
	scanSource := strings.TrimSpace(scanRHS[len("+\\"):])
	if scanSource != sourceName {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:         qScriptPipelineCallableOverScanSum,
		valueExpr:    sourceName,
		valueBinding: qScriptPipelineBinding(sourceName, bindings),
		scalarExpr:   initialExpr,
	}, true
}

func qScriptPipelineCallableAddExpr(src string, bindings map[string]string) bool {
	src = strings.TrimSpace(src)
	if bound := strings.TrimSpace(bindings[src]); bound != "" {
		src = bound
	}
	if strings.HasPrefix(src, "{") && strings.HasSuffix(src, "}") {
		body := strings.TrimSpace(src[1 : len(src)-1])
		plan, ok := qLambdaFastPlanFor(body)
		return ok && plan.kind == qLambdaFastDyadic && plan.op == '+'
	}
	return src == "+"
}

func qScriptPipelineUnaryNameTerm(src, word string) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasPrefix(src, word) || !wordBoundary(src, 0, len(word)) {
		return "", false
	}
	name := strings.TrimSpace(src[len(word):])
	if !qScriptPipelineSimpleName(name) {
		return "", false
	}
	return name, true
}

func qScriptPipelineStringJoinCountsDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 3 {
		return qScriptPipelineDescriptor{}, false
	}
	var joinedName string
	var splitSepExpr string
	var searchNeedleExpr string
	var replaceOldExpr string
	var replaceNewExpr string
	seenSplit := false
	seenSearch := false
	seenReplace := false
	for _, term := range terms {
		if sepExpr, name, ok := qScriptPipelineCountSplitTerm(term); ok {
			if seenSplit {
				return qScriptPipelineDescriptor{}, false
			}
			joinedName = qScriptPipelineMergeStringJoinName(joinedName, name)
			if joinedName == "" {
				return qScriptPipelineDescriptor{}, false
			}
			splitSepExpr = sepExpr
			seenSplit = true
			continue
		}
		if name, needleExpr, ok := qScriptPipelineCountSearchTerm(term); ok {
			if seenSearch {
				return qScriptPipelineDescriptor{}, false
			}
			joinedName = qScriptPipelineMergeStringJoinName(joinedName, name)
			if joinedName == "" {
				return qScriptPipelineDescriptor{}, false
			}
			searchNeedleExpr = needleExpr
			seenSearch = true
			continue
		}
		if name, oldExpr, newExpr, ok := qScriptPipelineCountReplaceTerm(term); ok {
			if seenReplace {
				return qScriptPipelineDescriptor{}, false
			}
			joinedName = qScriptPipelineMergeStringJoinName(joinedName, name)
			if joinedName == "" {
				return qScriptPipelineDescriptor{}, false
			}
			replaceOldExpr = oldExpr
			replaceNewExpr = newExpr
			seenReplace = true
			continue
		}
		return qScriptPipelineDescriptor{}, false
	}
	if !seenSplit || !seenSearch || !seenReplace || joinedName == "" {
		return qScriptPipelineDescriptor{}, false
	}
	joinRHS := strings.TrimSpace(bindings[joinedName])
	sepExpr, sourceExpr, ok := qScriptPipelineStringJoinSource(joinRHS)
	if !ok || strings.TrimSpace(sepExpr) != strings.TrimSpace(splitSepExpr) {
		return qScriptPipelineDescriptor{}, false
	}
	sourceName := strings.TrimSpace(sourceExpr)
	if !qScriptPipelineSimpleName(sourceName) || strings.TrimSpace(bindings[sourceName]) == "" {
		return qScriptPipelineDescriptor{}, false
	}
	descriptor := qScriptPipelineDescriptor{
		kind:         qScriptPipelineStringJoinCounts,
		valueExpr:    sourceName,
		valueBinding: qScriptPipelineBinding(sourceName, bindings),
		indexExpr:    joinedName,
		maskExpr:     splitSepExpr,
		rowValueExpr: searchNeedleExpr,
		scalarExpr:   replaceOldExpr,
		dyadicOp:     replaceNewExpr,
	}
	qScriptPipelineHoistStringJoinCounts(&descriptor)
	return descriptor, true
}

func qScriptPipelineMergeStringJoinName(current, next string) string {
	next = strings.TrimSpace(next)
	if current == "" {
		return next
	}
	if current != next {
		return ""
	}
	return current
}

func qScriptPipelineCountSplitTerm(src string) (string, string, bool) {
	expr, ok := qScriptPipelineCountTermExpr(src)
	if !ok {
		return "", "", false
	}
	left, right, ok := splitTopLevelWord(expr, "vs")
	if !ok || !qScriptPipelineSimpleName(strings.TrimSpace(right)) {
		return "", "", false
	}
	return strings.TrimSpace(left), strings.TrimSpace(right), true
}

func qScriptPipelineCountSearchTerm(src string) (string, string, bool) {
	expr, ok := qScriptPipelineCountTermExpr(src)
	if !ok {
		return "", "", false
	}
	left, right, ok := splitTopLevelWord(expr, "ss")
	if !ok || !qScriptPipelineSimpleName(strings.TrimSpace(left)) {
		return "", "", false
	}
	return strings.TrimSpace(left), strings.TrimSpace(right), true
}

func qScriptPipelineCountReplaceTerm(src string) (string, string, string, bool) {
	expr, ok := qScriptPipelineCountTermExpr(src)
	if !ok {
		return "", "", "", false
	}
	args, ok := qFunctionCallArgs(expr)
	if !ok || !strings.HasPrefix(strings.TrimSpace(expr), "ssr[") || len(args) != 3 {
		return "", "", "", false
	}
	name := strings.TrimSpace(args[0])
	if !qScriptPipelineSimpleName(name) {
		return "", "", "", false
	}
	return name, strings.TrimSpace(args[1]), strings.TrimSpace(args[2]), true
}

func qScriptPipelineCountTermExpr(src string) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "count") && wordBoundary(src, 0, len("count")) {
		body := strings.TrimSpace(src[len("count"):])
		return stripEnclosingParens(body), body != ""
	}
	if strings.HasPrefix(src, "#") {
		body := strings.TrimSpace(src[len("#"):])
		return stripEnclosingParens(body), body != ""
	}
	return "", false
}

func qScriptPipelineStringJoinSource(src string) (string, string, bool) {
	left, right, ok := splitTopLevelWord(stripEnclosingParens(strings.TrimSpace(src)), "sv")
	if !ok {
		return "", "", false
	}
	right = stripEnclosingParens(strings.TrimSpace(right))
	if !strings.HasPrefix(right, "string") || !wordBoundary(right, 0, len("string")) {
		return "", "", false
	}
	source := strings.TrimSpace(right[len("string"):])
	if source == "" {
		return "", "", false
	}
	return strings.TrimSpace(left), source, true
}

func qScriptPipelineHoistStringJoinCounts(descriptor *qScriptPipelineDescriptor) {
	values, count, ok := qScriptPipelineRepeatedStringValues(descriptor.valueBinding)
	if !ok {
		return
	}
	sep, ok := qScriptPipelineStaticString("sv", descriptor.maskExpr)
	if !ok {
		return
	}
	search, ok := qScriptPipelineStaticString("ss", descriptor.rowValueExpr)
	if !ok {
		return
	}
	old, ok := qScriptPipelineStaticString("ssr", descriptor.scalarExpr)
	if !ok {
		return
	}
	repl, ok := qScriptPipelineStaticString("ssr", descriptor.dyadicOp)
	if !ok {
		return
	}
	descriptor.stringValues = values
	descriptor.stringRepeatCount = count
	descriptor.stringSep = sep
	descriptor.stringSearch = search
	descriptor.stringReplaceOld = old
	descriptor.stringReplaceNew = repl
}

func qScriptPipelineStaticString(name, src string) (string, bool) {
	value, ok := qScriptPipelineStaticValue(src)
	if !ok {
		return "", false
	}
	text, err := qStringOperand(name, value)
	return text, err == nil
}

func qScriptPipelineSequenceEdgeReduceValue(src string) (string, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 3 {
		return "", false
	}
	var value string
	seen := map[string]bool{"sum": false, "first": false, "last": false}
	for _, term := range terms {
		kind, name, ok := qScriptPipelineSequenceReducerTerm(term)
		if !ok {
			return "", false
		}
		if value == "" {
			value = name
		} else if value != name {
			return "", false
		}
		if seen[kind] {
			return "", false
		}
		seen[kind] = true
	}
	return value, value != "" && seen["sum"] && seen["first"] && seen["last"]
}

func qScriptPipelineSumCountReduceValue(src string) (string, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 2 {
		return "", false
	}
	var value string
	seen := map[string]bool{"sum": false, "count": false}
	for _, term := range terms {
		kind, name, ok := qScriptPipelineSumCountReducerTerm(term)
		if !ok {
			return "", false
		}
		if value == "" {
			value = name
		} else if value != name {
			return "", false
		}
		if seen[kind] {
			return "", false
		}
		seen[kind] = true
	}
	return value, value != "" && seen["sum"] && seen["count"]
}

func qScriptPipelineSequenceSumCountDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	valueExpr, ok := qScriptPipelineSumCountReduceValue(src)
	if !ok {
		return qScriptPipelineDescriptor{}, false
	}
	baseExpr, steps, chainBindings, chainOK := qScriptPipelineSequenceTransformChain(valueExpr, bindings)
	if !chainOK || len(steps) == 0 {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:              qScriptPipelineSequenceSumCount,
		valueExpr:         valueExpr,
		valueBinding:      qScriptPipelineBinding(valueExpr, bindings),
		sequenceValueExpr: baseExpr,
		sequenceSteps:     steps,
		sequenceBindings:  chainBindings,
		includeCount:      true,
	}, true
}

func qScriptPipelineGatherSumCountDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 2 {
		return qScriptPipelineDescriptor{}, false
	}
	var valueExpr string
	var indexExpr string
	var countExpr string
	for _, term := range terms {
		if expr, ok := qScriptPipelineCountTermResolved(term, bindings, nil); ok {
			if countExpr != "" {
				return qScriptPipelineDescriptor{}, false
			}
			countExpr = expr
			continue
		}
		gatherValue, gatherIndex, ok := qScriptPipelineGatherSumTermResolved(term, bindings, nil)
		if !ok {
			return qScriptPipelineDescriptor{}, false
		}
		if valueExpr != "" {
			return qScriptPipelineDescriptor{}, false
		}
		valueExpr = gatherValue
		indexExpr = gatherIndex
	}
	if valueExpr == "" || indexExpr == "" || countExpr == "" || strings.TrimSpace(countExpr) != strings.TrimSpace(indexExpr) {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:                qScriptPipelineGatherReduceSumCount,
		valueExpr:           valueExpr,
		valueBinding:        qScriptPipelineBinding(valueExpr, bindings),
		indexExpr:           indexExpr,
		indexBinding:        qScriptPipelineBinding(indexExpr, bindings),
		includeCount:        true,
		absorbedAssignments: qScriptPipelineAbsorbedTermAliases(terms, bindings),
	}, true
}

func qScriptPipelineIndexExprSumCountDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) < 2 {
		return qScriptPipelineDescriptor{}, false
	}
	sumExprs := make([]string, 0, len(terms))
	var indexExpr string
	var includeCount bool
	for _, term := range terms {
		if expr, ok := qScriptPipelineCountTerm(term); ok {
			if indexExpr != "" && indexExpr != expr {
				return qScriptPipelineDescriptor{}, false
			}
			indexExpr = expr
			includeCount = true
			continue
		}
		expr, ok := qScriptPipelineSumExprTerm(term)
		if !ok {
			return qScriptPipelineDescriptor{}, false
		}
		sumExprs = append(sumExprs, expr)
	}
	if len(sumExprs) == 0 || indexExpr == "" || !includeCount {
		return qScriptPipelineDescriptor{}, false
	}
	indexBinding := qScriptPipelineBinding(indexExpr, bindings)
	if _, ok := directWhereMaskExpr(indexBinding); !ok {
		return qScriptPipelineDescriptor{}, false
	}
	reducers := make([]data.I64IndexExprReducer, 0, len(sumExprs)+1)
	for _, sumExpr := range sumExprs {
		exprPlan, ok := qScriptPipelineI64IndexExprPlan(sumExpr, indexExpr, bindings, nil)
		if !ok {
			return qScriptPipelineDescriptor{}, false
		}
		reducers = append(reducers, data.I64IndexExprReducer{Kind: data.I64IndexExprReducerSum, Expr: exprPlan})
	}
	reducers = append(reducers, data.I64IndexExprReducer{Kind: data.I64IndexExprReducerCount})
	shapeTerm := "sum-count"
	if len(sumExprs) > 1 {
		shapeTerm = "sum" + strconv.Itoa(len(sumExprs)) + "-count"
	}
	return qScriptPipelineDescriptor{
		kind:          qScriptPipelineIndexExprSumCount,
		shapeText:     "script-pipeline/index-expr-reduce/" + shapeTerm + "/assignments",
		valueExpr:     strings.Join(sumExprs, "+"),
		indexExpr:     indexExpr,
		indexBinding:  indexBinding,
		indexExprPlan: reducers[0].Expr,
		indexReducers: reducers,
		includeCount:  true,
	}, true
}

func qScriptPipelineSumExprTerm(src string) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		expr := strings.TrimSpace(src[len("+/"):])
		return expr, expr != ""
	}
	if strings.HasPrefix(src, "sum") && wordBoundary(src, 0, len("sum")) {
		expr := strings.TrimSpace(src[len("sum"):])
		return expr, expr != ""
	}
	return "", false
}

func qScriptPipelineI64IndexExprPlan(src, indexName string, bindings map[string]string, seen map[string]bool) (data.I64IndexExpr, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	indexName = strings.TrimSpace(indexName)
	if src == "" || indexName == "" {
		return data.I64IndexExpr{}, false
	}
	if src == indexName {
		return data.I64IndexExpr{Op: data.I64IndexExprIndex}, true
	}
	if qScriptPipelineSimpleName(src) {
		bound := strings.TrimSpace(bindings[src])
		if bound == "" {
			return data.I64IndexExpr{}, false
		}
		if seen == nil {
			seen = make(map[string]bool, len(bindings))
		}
		if seen[src] {
			return data.I64IndexExpr{}, false
		}
		seen[src] = true
		return qScriptPipelineI64IndexExprPlan(bound, indexName, bindings, seen)
	}
	if n, ok := qScriptPipelineStaticInt(src); ok {
		return data.I64IndexExpr{Op: data.I64IndexExprConst, Value: int64(n)}, true
	}
	if n, ok := qScriptPipelineRepeatedIndexConstant(src, indexName); ok {
		return data.I64IndexExpr{Op: data.I64IndexExprConst, Value: int64(n)}, true
	}
	if widthExpr, valueExpr, ok := splitTopLevelWord(src, "xbar"); ok {
		width, widthOK := qScriptPipelineStaticInt(widthExpr)
		if !widthOK || width <= 0 {
			return data.I64IndexExpr{}, false
		}
		valuePlan, ok := qScriptPipelineI64IndexExprPlan(valueExpr, indexName, bindings, cloneBoolMap(seen))
		if !ok {
			return data.I64IndexExpr{}, false
		}
		return data.I64IndexExpr{Op: data.I64IndexExprXbar, Value: int64(width), Left: &valuePlan}, true
	}
	for _, spec := range []struct {
		op   string
		kind data.I64IndexExprOp
	}{
		{"+", data.I64IndexExprAdd},
		{"-", data.I64IndexExprSub},
	} {
		left, right, ok := splitTopLevelOperator(src, spec.op)
		if !ok || strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
			continue
		}
		leftPlan, rightPlan, ok := qScriptPipelineI64IndexExprOperands(left, right, indexName, bindings, seen)
		if !ok {
			return data.I64IndexExpr{}, false
		}
		return data.I64IndexExpr{Op: spec.kind, Left: &leftPlan, Right: &rightPlan}, true
	}
	for _, spec := range []struct {
		op   string
		kind data.I64IndexExprOp
	}{
		{"*", data.I64IndexExprMul},
		{"div", data.I64IndexExprDiv},
		{"mod", data.I64IndexExprMod},
	} {
		var left, right string
		var ok bool
		if spec.op == "*" {
			left, right, ok = splitTopLevelOperator(src, spec.op)
		} else {
			left, right, ok = splitTopLevelWord(src, spec.op)
		}
		if !ok || strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
			continue
		}
		leftPlan, rightPlan, ok := qScriptPipelineI64IndexExprOperands(left, right, indexName, bindings, seen)
		if !ok {
			return data.I64IndexExpr{}, false
		}
		return data.I64IndexExpr{Op: spec.kind, Left: &leftPlan, Right: &rightPlan}, true
	}
	return data.I64IndexExpr{}, false
}

func qScriptPipelineRepeatedIndexConstant(src, indexName string) (int, bool) {
	left, right, ok := splitTopLevelOperator(stripEnclosingParens(strings.TrimSpace(src)), "#")
	if !ok {
		return 0, false
	}
	countName, ok := qScriptPipelineCountTerm(left)
	if !ok || strings.TrimSpace(countName) != strings.TrimSpace(indexName) {
		return 0, false
	}
	value, ok := qScriptPipelineStaticInt(right)
	return value, ok
}

func qScriptPipelineI64IndexExprOperands(left, right, indexName string, bindings map[string]string, seen map[string]bool) (data.I64IndexExpr, data.I64IndexExpr, bool) {
	leftPlan, ok := qScriptPipelineI64IndexExprPlan(left, indexName, bindings, cloneBoolMap(seen))
	if !ok {
		return data.I64IndexExpr{}, data.I64IndexExpr{}, false
	}
	rightPlan, ok := qScriptPipelineI64IndexExprPlan(right, indexName, bindings, cloneBoolMap(seen))
	if !ok {
		return data.I64IndexExpr{}, data.I64IndexExpr{}, false
	}
	return leftPlan, rightPlan, true
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func qScriptPipelineI64IndexExprDependsOnName(src, name, indexName string, bindings map[string]string, seen map[string]bool) bool {
	src = stripEnclosingParens(strings.TrimSpace(src))
	name = strings.TrimSpace(name)
	if src == "" || name == "" {
		return false
	}
	if src == name {
		return true
	}
	if src == indexName {
		return false
	}
	if qScriptPipelineSimpleName(src) {
		bound := strings.TrimSpace(bindings[src])
		if bound == "" {
			return false
		}
		if seen == nil {
			seen = make(map[string]bool, len(bindings))
		}
		if seen[src] {
			return false
		}
		seen[src] = true
		return qScriptPipelineI64IndexExprDependsOnName(bound, name, indexName, bindings, seen)
	}
	for _, op := range []string{"+", "-", "*"} {
		left, right, ok := splitTopLevelOperator(src, op)
		if ok && (qScriptPipelineI64IndexExprDependsOnName(left, name, indexName, bindings, cloneBoolMap(seen)) ||
			qScriptPipelineI64IndexExprDependsOnName(right, name, indexName, bindings, cloneBoolMap(seen))) {
			return true
		}
	}
	for _, op := range []string{"div", "mod"} {
		left, right, ok := splitTopLevelWord(src, op)
		if ok && (qScriptPipelineI64IndexExprDependsOnName(left, name, indexName, bindings, cloneBoolMap(seen)) ||
			qScriptPipelineI64IndexExprDependsOnName(right, name, indexName, bindings, cloneBoolMap(seen))) {
			return true
		}
	}
	if left, right, ok := splitTopLevelWord(src, "xbar"); ok {
		return qScriptPipelineI64IndexExprDependsOnName(left, name, indexName, bindings, cloneBoolMap(seen)) ||
			qScriptPipelineI64IndexExprDependsOnName(right, name, indexName, bindings, cloneBoolMap(seen))
	}
	if left, right, ok := splitTopLevelOperator(src, "#"); ok {
		return qScriptPipelineI64IndexExprDependsOnName(left, name, indexName, bindings, cloneBoolMap(seen)) ||
			qScriptPipelineI64IndexExprDependsOnName(right, name, indexName, bindings, cloneBoolMap(seen))
	}
	return false
}

func qScriptPipelineGatherSumTerm(src string) (string, string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		src = strings.TrimSpace(src[len("+/"):])
	} else if strings.HasPrefix(src, "sum") && wordBoundary(src, 0, len("sum")) {
		src = strings.TrimSpace(src[len("sum"):])
	} else {
		return "", "", false
	}
	valueExpr, indexExpr, ok := findPostfixIndex(src)
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(valueExpr), strings.TrimSpace(indexExpr), valueExpr != "" && indexExpr != ""
}

func qScriptPipelineGatherSumTermResolved(src string, bindings map[string]string, seen map[string]bool) (string, string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if valueExpr, indexExpr, ok := qScriptPipelineGatherSumTerm(src); ok {
		return valueExpr, indexExpr, true
	}
	if qScriptPipelineSimpleName(src) {
		bound, ok := qScriptPipelineResolveSimpleBinding(src, bindings, seen)
		if !ok {
			return "", "", false
		}
		if valueExpr, indexExpr, ok := qScriptPipelineGatherSumTerm(bound); ok {
			return valueExpr, indexExpr, true
		}
		if next := qScriptPipelineAliasBodyName(bound); next != "" {
			nextBound, ok := qScriptPipelineResolveSimpleBinding(next, bindings, seen)
			if !ok {
				return "", "", false
			}
			valueExpr, indexExpr, ok := findPostfixIndex(nextBound)
			if !ok {
				return "", "", false
			}
			return strings.TrimSpace(valueExpr), strings.TrimSpace(indexExpr), valueExpr != "" && indexExpr != ""
		}
		return qScriptPipelineGatherSumTermResolved(bound, bindings, seen)
	}
	return "", "", false
}

func qScriptPipelineSumPlusDyadicFloatDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 2 {
		return qScriptPipelineDescriptor{}, false
	}
	var valueExpr string
	var dyadic qScriptPipelineDyadicFloatReducerTerm
	for _, term := range terms {
		if kind, name, ok := qScriptPipelineSequenceReducerTerm(term); ok && kind == "sum" {
			if valueExpr != "" {
				return qScriptPipelineDescriptor{}, false
			}
			valueExpr = name
			continue
		}
		parsed, ok := qScriptPipelineDyadicFloatReducerTermFor(term)
		if !ok {
			return qScriptPipelineDescriptor{}, false
		}
		if dyadic.op != "" {
			return qScriptPipelineDescriptor{}, false
		}
		dyadic = parsed
	}
	if valueExpr == "" || dyadic.op == "" || valueExpr != dyadic.valueExpr {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:         qScriptPipelineSumPlusDyadicFloat,
		valueExpr:    valueExpr,
		valueBinding: qScriptPipelineBinding(valueExpr, bindings),
		dyadicOp:     dyadic.op,
		scalarExpr:   dyadic.scalarExpr,
		scalarLeft:   dyadic.scalarLeft,
	}, true
}

type qScriptPipelineDyadicFloatReducerTerm struct {
	op         string
	valueExpr  string
	scalarExpr string
	scalarLeft bool
}

func qScriptPipelineDyadicFloatReducerTermFor(src string) (qScriptPipelineDyadicFloatReducerTerm, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		src = strings.TrimSpace(src[len("+/"):])
	} else if strings.HasPrefix(src, "sum") && wordBoundary(src, 0, len("sum")) {
		src = strings.TrimSpace(src[len("sum"):])
	} else {
		return qScriptPipelineDyadicFloatReducerTerm{}, false
	}
	for _, op := range []string{data.NumericDyadicXExp, data.NumericDyadicXLog} {
		left, right, ok := splitTopLevelWord(src, op)
		if !ok {
			continue
		}
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		switch {
		case qScriptPipelineSimpleName(left) && right != "":
			return qScriptPipelineDyadicFloatReducerTerm{op: op, valueExpr: left, scalarExpr: right, scalarLeft: false}, true
		case qScriptPipelineSimpleName(right) && left != "":
			return qScriptPipelineDyadicFloatReducerTerm{op: op, valueExpr: right, scalarExpr: left, scalarLeft: true}, true
		default:
			return qScriptPipelineDyadicFloatReducerTerm{}, false
		}
	}
	return qScriptPipelineDyadicFloatReducerTerm{}, false
}

type qScriptPipelineIntegerDivModTerm struct {
	op         data.Op
	valueExpr  string
	scalarExpr string
}

func qScriptPipelineIntegerDivModDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) < 2 {
		return qScriptPipelineDescriptor{}, false
	}
	var valueExpr string
	var includeCount bool
	reducers := make([]qScriptPipelineIntegerDivModTerm, 0, len(terms))
	for _, term := range terms {
		if expr, ok := qScriptPipelineCountTerm(term); ok {
			if includeCount {
				return qScriptPipelineDescriptor{}, false
			}
			if valueExpr == "" {
				valueExpr = expr
			} else if valueExpr != expr {
				return qScriptPipelineDescriptor{}, false
			}
			includeCount = true
			continue
		}
		parsed, ok := qScriptPipelineIntegerDivModReducerTermFor(term)
		if !ok {
			return qScriptPipelineDescriptor{}, false
		}
		if valueExpr == "" {
			valueExpr = parsed.valueExpr
		} else if valueExpr != parsed.valueExpr {
			return qScriptPipelineDescriptor{}, false
		}
		reducers = append(reducers, parsed)
	}
	if valueExpr == "" || len(reducers) == 0 {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:         qScriptPipelineIntegerDivModReduce,
		valueExpr:    valueExpr,
		valueBinding: qScriptPipelineBinding(valueExpr, bindings),
		integerTerms: reducers,
		includeCount: includeCount,
	}, true
}

func qScriptPipelineIntegerDivModReducerTermFor(src string) (qScriptPipelineIntegerDivModTerm, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		src = strings.TrimSpace(src[len("+/"):])
	} else if strings.HasPrefix(src, "sum") && wordBoundary(src, 0, len("sum")) {
		src = strings.TrimSpace(src[len("sum"):])
	} else {
		return qScriptPipelineIntegerDivModTerm{}, false
	}
	for _, spec := range []struct {
		word string
		op   data.Op
	}{
		{"div", data.OpDiv},
		{"mod", data.OpMod},
	} {
		left, right, ok := splitTopLevelWord(src, spec.word)
		if !ok {
			continue
		}
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		if !qScriptPipelineSimpleName(left) || right == "" {
			return qScriptPipelineIntegerDivModTerm{}, false
		}
		return qScriptPipelineIntegerDivModTerm{op: spec.op, valueExpr: left, scalarExpr: right}, true
	}
	return qScriptPipelineIntegerDivModTerm{}, false
}

func qScriptPipelineMatrixCellPlusCountDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 2 {
		return qScriptPipelineDescriptor{}, false
	}
	var cell qScriptPipelineMatrixCellTerm
	var countExpr string
	for _, term := range terms {
		if parsed, ok := qScriptPipelineMatrixCellTermFor(term, bindings); ok {
			if cell.matrixExpr != "" {
				return qScriptPipelineDescriptor{}, false
			}
			cell = parsed
			continue
		}
		if expr, ok := qScriptPipelineCountTerm(term); ok {
			if countExpr != "" {
				return qScriptPipelineDescriptor{}, false
			}
			countExpr = expr
			continue
		}
		return qScriptPipelineDescriptor{}, false
	}
	if cell.matrixExpr == "" || countExpr == "" || countExpr != cell.matrixExpr {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:         qScriptPipelineMatrixCellPlusCount,
		valueExpr:    cell.matrixExpr,
		valueBinding: qScriptPipelineBinding(cell.matrixExpr, bindings),
		rowValueExpr: cell.matrixExpr,
		rowIndexExpr: cell.rowExpr,
		colIndexExpr: cell.colExpr,
	}, true
}

func qScriptPipelineMatrixRowsSumCountDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) < 3 {
		return qScriptPipelineDescriptor{}, false
	}
	var matrixExpr string
	var countExpr string
	rowIndexes := make([]string, 0, len(terms)-1)
	for _, term := range terms {
		if rowMatrix, rowIndex, ok := qScriptPipelineMatrixRowSumTerm(term); ok {
			if matrixExpr == "" {
				matrixExpr = rowMatrix
			} else if matrixExpr != rowMatrix {
				return qScriptPipelineDescriptor{}, false
			}
			rowIndexes = append(rowIndexes, rowIndex)
			continue
		}
		if expr, ok := qScriptPipelineCountTerm(term); ok {
			if countExpr != "" {
				return qScriptPipelineDescriptor{}, false
			}
			countExpr = expr
			continue
		}
		return qScriptPipelineDescriptor{}, false
	}
	if matrixExpr == "" || countExpr == "" || countExpr != matrixExpr || len(rowIndexes) < 2 {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:         qScriptPipelineMatrixRowsSumCount,
		valueExpr:    matrixExpr,
		valueBinding: qScriptPipelineBinding(matrixExpr, bindings),
		rowValueExpr: matrixExpr,
		rowIndexExpr: rowIndexes[0],
		indexExpr:    strings.Join(rowIndexes[1:], " "),
	}, true
}

func qScriptPipelineMatrixRowSumTerm(src string) (string, string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasPrefix(src, "+/") {
		return "", "", false
	}
	body := stripEnclosingParens(strings.TrimSpace(src[len("+/"):]))
	plan, ok := buildQPipelineApplyScalarIndexPlan(body)
	if !ok {
		return "", "", false
	}
	if plan.valueExpr == "" || plan.indexExpr == "" {
		return "", "", false
	}
	return plan.valueExpr, plan.indexExpr, true
}

func qScriptPipelineMatrixNestedCellDescriptor(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 3 {
		return qScriptPipelineDescriptor{}, false
	}
	var sumMatrixExpr string
	var cell qScriptPipelineMatrixCellTerm
	var countExpr string
	for _, term := range terms {
		if expr, ok := qScriptPipelineNestedRazeSumTerm(term); ok {
			if sumMatrixExpr != "" {
				return qScriptPipelineDescriptor{}, false
			}
			sumMatrixExpr = expr
			continue
		}
		if parsed, ok := qScriptPipelineMatrixCellTermFor(term, bindings); ok {
			if cell.matrixExpr != "" {
				return qScriptPipelineDescriptor{}, false
			}
			cell = parsed
			continue
		}
		if expr, ok := qScriptPipelineCountTerm(term); ok {
			if countExpr != "" {
				return qScriptPipelineDescriptor{}, false
			}
			countExpr = expr
			continue
		}
		return qScriptPipelineDescriptor{}, false
	}
	if sumMatrixExpr == "" || cell.matrixExpr == "" || countExpr == "" {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:         qScriptPipelineMatrixNestedCell,
		valueExpr:    sumMatrixExpr,
		valueBinding: qScriptPipelineBinding(sumMatrixExpr, bindings),
		indexExpr:    countExpr,
		indexBinding: qScriptPipelineBinding(countExpr, bindings),
		rowValueExpr: cell.matrixExpr,
		rowIndexExpr: cell.rowExpr,
		colIndexExpr: cell.colExpr,
	}, true
}

func qScriptPipelineNestedRazeSumTerm(src string) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasPrefix(src, "+/") {
		return "", false
	}
	body := strings.TrimSpace(src[len("+/"):])
	if !strings.HasPrefix(body, "raze") || !wordBoundary(body, 0, len("raze")) {
		return "", false
	}
	expr := strings.TrimSpace(body[len("raze"):])
	if !qScriptPipelineSimpleName(expr) {
		return "", false
	}
	return expr, true
}

type qScriptPipelineMatrixCellTerm struct {
	matrixExpr string
	rowExpr    string
	colExpr    string
}

func qScriptPipelineMatrixCellTermFor(src string, bindings map[string]string) (qScriptPipelineMatrixCellTerm, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	plan, ok := buildQPipelineApplyPathIndexPlan(src)
	if !ok || plan.compareOp != "dot" {
		return qScriptPipelineMatrixCellTerm{}, false
	}
	rowExpr, colExpr, ok := qScriptPipelinePathIndexPair(plan.indexExpr)
	if !ok {
		return qScriptPipelineMatrixCellTerm{}, false
	}
	matrixExpr := strings.TrimSpace(plan.valueExpr)
	if matrixExpr == "" {
		return qScriptPipelineMatrixCellTerm{}, false
	}
	if !qScriptPipelineSimpleName(matrixExpr) && qScriptPipelineBinding(matrixExpr, bindings) == "" {
		return qScriptPipelineMatrixCellTerm{}, false
	}
	return qScriptPipelineMatrixCellTerm{matrixExpr: matrixExpr, rowExpr: rowExpr, colExpr: colExpr}, true
}

func qScriptPipelinePathIndexPair(src string) (string, string, bool) {
	left, right, ok := splitQScriptPrefixDyadicArgs(src)
	return left, right, ok
}

func qScriptPipelineCountTerm(src string) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasPrefix(src, "count") || !wordBoundary(src, 0, len("count")) {
		return "", false
	}
	name := strings.TrimSpace(src[len("count"):])
	if !qScriptPipelineSimpleName(name) {
		return "", false
	}
	return name, true
}

func qScriptPipelineCountTermResolved(src string, bindings map[string]string, seen map[string]bool) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if name, ok := qScriptPipelineCountTerm(src); ok {
		return name, true
	}
	if !qScriptPipelineSimpleName(src) {
		return "", false
	}
	bound, ok := qScriptPipelineResolveSimpleBinding(src, bindings, seen)
	if !ok {
		return "", false
	}
	return qScriptPipelineCountTermResolved(bound, bindings, seen)
}

func qScriptPipelinePlusTerms(src string) []string {
	src = strings.TrimSpace(src)
	stripped := stripEnclosingParens(src)
	if left, right, ok := splitTopLevelOperator(stripped, "+"); ok {
		terms := qScriptPipelinePlusTerms(left)
		terms = append(terms, qScriptPipelinePlusTerms(right)...)
		return terms
	}
	if stripped == "" {
		if src == "" {
			return nil
		}
		// An empty-list literal (`()`) is a REAL term: dropping it would lose
		// the empty-broadcast semantics of `()+x` (which is `()`). Term
		// consumers that cannot represent it must decline the whole plan.
		return []string{src}
	}
	return []string{stripped}
}

func qScriptPipelineSequenceReducerTerm(src string) (string, string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		name := strings.TrimSpace(src[len("+/"):])
		if qScriptPipelineSimpleName(name) {
			return "sum", name, true
		}
		return "", "", false
	}
	for _, kind := range []string{"sum", "first", "last"} {
		if !strings.HasPrefix(src, kind) || !wordBoundary(src, 0, len(kind)) {
			continue
		}
		name := strings.TrimSpace(src[len(kind):])
		if qScriptPipelineSimpleName(name) {
			return kind, name, true
		}
		return "", "", false
	}
	return "", "", false
}

func qScriptPipelineSumCountReducerTerm(src string) (string, string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		name := strings.TrimSpace(src[len("+/"):])
		if qScriptPipelineSimpleName(name) {
			return "sum", name, true
		}
		return "", "", false
	}
	if strings.HasPrefix(src, "sum") && wordBoundary(src, 0, len("sum")) {
		name := strings.TrimSpace(src[len("sum"):])
		if qScriptPipelineSimpleName(name) {
			return "sum", name, true
		}
		return "", "", false
	}
	if strings.HasPrefix(src, "count") && wordBoundary(src, 0, len("count")) {
		name := strings.TrimSpace(src[len("count"):])
		if qScriptPipelineSimpleName(name) {
			return "count", name, true
		}
		return "", "", false
	}
	return "", "", false
}

func qScriptPipelineSimpleName(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" || !isQIdentStart(src[0]) {
		return false
	}
	for i := 1; i < len(src); i++ {
		if !isQIdentRest(src[i]) {
			return false
		}
	}
	return true
}

func qScriptPipelineResolveSimpleBinding(name string, bindings map[string]string, seen map[string]bool) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || bindings == nil {
		return "", false
	}
	bound := strings.TrimSpace(bindings[name])
	if bound == "" {
		return "", false
	}
	if seen == nil {
		seen = make(map[string]bool, len(bindings))
	}
	if seen[name] {
		return "", false
	}
	seen[name] = true
	return bound, true
}

func qScriptPipelineAbsorbedAssignments(descriptor qScriptPipelineDescriptor) []string {
	if descriptor.kind != qScriptPipelineGatherReduceSumCount {
		return descriptor.absorbedAssignments
	}
	seen := map[string]bool{}
	var out []string
	for _, name := range descriptor.absorbedAssignments {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func qScriptPipelineAbsorbedTermAliases(terms []string, bindings map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	var collect func(string)
	collect = func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || !qScriptPipelineSimpleName(name) || seen[name] {
			return
		}
		bound := strings.TrimSpace(bindings[name])
		if bound == "" {
			return
		}
		seen[name] = true
		out = append(out, name)
		if next := qScriptPipelineAliasBodyName(bound); next != "" {
			collect(next)
		}
	}
	for _, term := range terms {
		term = stripEnclosingParens(strings.TrimSpace(term))
		if qScriptPipelineSimpleName(term) {
			collect(term)
			continue
		}
		if next := qScriptPipelineAliasBodyName(term); next != "" {
			collect(next)
		}
	}
	return out
}

func qScriptPipelineAliasBodyName(src string) string {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		name := strings.TrimSpace(src[len("+/"):])
		if qScriptPipelineSimpleName(name) {
			return name
		}
	}
	if strings.HasPrefix(src, "sum") && wordBoundary(src, 0, len("sum")) {
		name := strings.TrimSpace(src[len("sum"):])
		if qScriptPipelineSimpleName(name) {
			return name
		}
	}
	return ""
}

func qScriptPipelineAssignmentAbsorbed(descriptor *qScriptPipelineDescriptor, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || descriptor == nil {
		return false
	}
	if name == strings.TrimSpace(descriptor.valueExpr) || name == strings.TrimSpace(descriptor.indexExpr) {
		return true
	}
	for _, absorbed := range descriptor.absorbedAssignments {
		if name == strings.TrimSpace(absorbed) {
			return true
		}
	}
	return false
}

func qScriptPipelineIndexMaskExpr(indexExpr string, bindings map[string]string) (string, bool) {
	if maskExpr, ok := directWhereMaskExpr(indexExpr); ok {
		return maskExpr, true
	}
	bound, ok := bindings[strings.TrimSpace(indexExpr)]
	if !ok {
		return "", false
	}
	return directWhereMaskExpr(bound)
}

func qScriptPipelineBinding(expr string, bindings map[string]string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	if rhs, ok := bindings[expr]; ok {
		return rhs
	}
	return ""
}

func (s *EvalState) tryEvalQScriptPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	if descriptor == nil || descriptor.kind == qScriptPipelineUnsupported {
		return nil, false, nil
	}
	shape := descriptor.shape()
	if descriptor.kind == qScriptPipelineSequenceEdgeSum {
		out, handled, err := s.evalQScriptSequenceEdgeSumPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineSequenceSumCount {
		out, handled, err := s.evalQScriptSequenceTransformChainSumCountPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineGatherReduceSumCount {
		out, handled, err := s.evalQScriptGatherSumCountPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineIndexExprSumCount {
		out, handled, err := s.evalQScriptIndexExprSumCountPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineSumPlusDyadicFloat {
		out, handled, err := s.evalQScriptSumPlusDyadicFloatPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineIntegerDivModReduce {
		out, handled, err := s.evalQScriptIntegerDivModReducePipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineMatrixRowSumCount {
		out, handled, err := s.evalQScriptMatrixRowSumCountPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineMatrixRowsSumCount {
		out, handled, err := s.evalQScriptMatrixRowsSumCountPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineMatrixCellPlusCount {
		out, handled, err := s.evalQScriptMatrixCellPlusCountPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineMatrixNestedCell {
		out, handled, err := s.evalQScriptMatrixNestedCellPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineCallableDotSumRight || descriptor.kind == qScriptPipelineCallableDotSumCount {
		out, handled, err := s.evalQScriptCallableDotSumPlusRightPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineCallableOverScanSum {
		out, handled, err := s.evalQScriptCallableOverScanSumPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineStringJoinCounts {
		out, handled, err := s.evalQScriptStringJoinCountsPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	terminal := &descriptor.terminalPlan
	if terminal.kind == qPipelineInvalid {
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", RuntimeFallbackPlannerUnhandled)
		return nil, false, nil
	}
	s.rememberQPipelinePlanKnownSource(descriptor.terminal, *terminal)
	for _, assignment := range descriptor.assignments {
		if qScriptPipelineCanDeferAssignment(descriptor, assignment) {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", RuntimeFallbackPipelineError)
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", RuntimeFallbackPipelineError)
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	out, handled, err := s.evalQScriptTerminalPipeline(descriptor, terminal)
	recordQScriptPipelineResult(shape, handled, err)
	return out, handled, err
}

func recordQScriptPipelineResult(shape string, handled bool, err error) {
	recordRuntimeKernelProbeReason("QScriptPipelinePlan", shape, handled, err, RuntimeFallbackUnsupportedShape)
}

func (s *EvalState) evalQScriptSumPlusDyadicFloatPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	for _, assignment := range descriptor.assignments {
		if strings.TrimSpace(descriptor.valueExpr) == assignment.name {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.valueExpr); err != nil {
			return nil, true, err
		}
		value, handled, err = s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
	}
	if !handled {
		return nil, false, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	scalar, handled, err := s.evalQScriptBindingPlan(&descriptor.scalarPlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		scalar, err = s.eval(strings.TrimSpace(descriptor.scalarExpr))
		if err != nil {
			return nil, true, err
		}
	}
	shape := "vector-reduce/sum-plus-dyadic-float-" + descriptor.dyadicOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	out, handled, err := data.TryTypedNumericSumPlusScalarDyadicFloatSum(array, descriptor.dyadicOp, scalar, descriptor.scalarLeft)
	out, handled, err = qTypedRuntimeResultReason("ArrayNumericSumPlusDyadicFloatSum", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	return out, handled, err
}

func (s *EvalState) evalQScriptIntegerDivModReducePipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	for _, assignment := range descriptor.assignments {
		if strings.TrimSpace(descriptor.valueExpr) == assignment.name {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.valueExpr); err != nil {
			return nil, true, err
		}
		value, handled, err = s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
	}
	if !handled {
		return nil, false, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	terms := make([]data.IntegerDivModReducerTerm, 0, len(descriptor.integerTerms))
	for _, term := range descriptor.integerTerms {
		divisorValue, err := s.eval(strings.TrimSpace(term.scalarExpr))
		if err != nil {
			return nil, true, err
		}
		divisor, ok := integerValue(divisorValue)
		if !ok {
			return nil, false, nil
		}
		terms = append(terms, data.IntegerDivModReducerTerm{Op: term.op, Divisor: divisor})
	}
	shape := "vector-reduce/integer-divmod-sum-count/" + string(array.Kind())
	out, handled, err := data.TryTypedIntegerDivModSumCount(array, terms, descriptor.includeCount)
	out, handled, err = qTypedRuntimeResultReason("ArrayIntegerDivModSumCount", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	return out, handled, err
}

func (s *EvalState) evalQScriptMatrixRowSumCountPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	if assignment, ok := qScriptPipelineAssignmentByName(descriptor, descriptor.rowValueExpr); ok {
		row, handled, err := s.evalQScriptScalarIndexPlan(&descriptor.rowIndexPlan)
		if err != nil || !handled {
			return nil, handled, err
		}
		if out, handled, err := s.evalQScriptReshapedMatrixRowSumCount(&assignment.binding, row); err != nil || handled {
			return out, handled, err
		}
	}
	for _, assignment := range descriptor.assignments {
		if strings.TrimSpace(descriptor.valueExpr) == assignment.name || strings.TrimSpace(descriptor.rowValueExpr) == assignment.name {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	matrixValue, handled, err := s.evalQScriptBindingPlan(&descriptor.rowValuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	matrix, ok := matrixValue.(data.Matrix)
	if !ok {
		return nil, false, nil
	}
	indexValue, handled, err := s.evalQScriptBindingPlan(&descriptor.rowIndexPlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	indexes, scalar, err := indexInts(indexValue)
	if err != nil {
		// Negative/null indexes: fall back to the generic read path, which
		// null-fills out-of-range reads.
		return nil, false, nil
	}
	if !scalar || len(indexes) != 1 {
		return nil, false, nil
	}
	shape := qMatrixIndexShape(matrix, 1) + "/sum-count"
	out, handled, err := data.TryMatrixRowNumericSumCount(matrix, indexes[0])
	return qTypedRuntimeResultReason("MatrixRowSumCount", shape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func (s *EvalState) evalQScriptMatrixRowsSumCountPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	if out, handled, err := s.evalQScriptTransposedReshapedMatrixRowsSumCountPipeline(descriptor); err != nil || handled {
		return out, handled, err
	}
	for _, assignment := range descriptor.assignments {
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	matrixValue, handled, err := s.evalQScriptBindingPlan(&descriptor.rowValuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	matrix, ok := matrixValue.(data.Matrix)
	if !ok {
		return nil, false, nil
	}
	rowExprs := append([]string{descriptor.rowIndexExpr}, strings.Fields(descriptor.indexExpr)...)
	if len(rowExprs) < 2 {
		return nil, false, nil
	}
	rows := make([]int, 0, len(rowExprs))
	for _, expr := range rowExprs {
		plan := buildQScriptBindingPlanForRHS(expr, nil)
		row, handled, err := s.evalQScriptScalarIndexPlan(&plan)
		if err != nil || !handled {
			return nil, handled, err
		}
		rows = append(rows, row)
	}
	shape := qMatrixIndexShape(matrix, len(rows)) + "/rows-sum-count"
	out, handled, err := data.TryMatrixRowsNumericSumPlusCount(matrix, rows...)
	return qTypedRuntimeResultReason("MatrixRowsSumCount", shape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func (s *EvalState) evalQScriptTransposedReshapedMatrixRowsSumCountPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	if descriptor == nil || strings.TrimSpace(descriptor.rowValueExpr) == "" {
		return nil, false, nil
	}
	flipAssignment, ok := qScriptPipelineAssignmentByName(descriptor, descriptor.rowValueExpr)
	if !ok || flipAssignment.binding.kind != qScriptBindingUnary || flipAssignment.binding.op != "flip" || flipAssignment.binding.left == nil || flipAssignment.binding.left.kind != qScriptBindingName {
		return nil, false, nil
	}
	sourceName := flipAssignment.binding.left.name
	reshapeAssignment, ok := qScriptPipelineAssignmentByName(descriptor, sourceName)
	if !ok || reshapeAssignment.binding.kind != qScriptBindingBinary || reshapeAssignment.binding.op != "#" || reshapeAssignment.binding.left == nil || reshapeAssignment.binding.right == nil {
		return nil, false, nil
	}
	rows, handled, err := s.evalQScriptMatrixRowsSumCountRows(descriptor)
	if err != nil || !handled {
		return nil, handled, err
	}
	resolver := qScriptPipelineAssignmentResolver(descriptor)
	shapeValue, handled, err := s.evalQScriptBindingPlanWithResolver(reshapeAssignment.binding.left, resolver)
	if err != nil || !handled {
		return nil, handled, err
	}
	matrixShape, nullDim, err := qReshapeShape(shapeValue)
	if err != nil {
		return nil, true, err
	}
	if nullDim >= 0 || len(matrixShape) != 2 {
		return nil, false, nil
	}
	sourceValue, handled, err := s.evalQScriptBindingPlanWithResolver(reshapeAssignment.binding.right, resolver)
	if err != nil || !handled {
		return nil, handled, err
	}
	source, ok := sourceValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "matrix-dot/reshape-transpose/" + qRuntimeCardinalityShape(matrixShape[0]) + "x" + qRuntimeCardinalityShape(matrixShape[1]) + "/" + strconv.Itoa(len(rows)) + "-rows-sum-count"
	out, handled, err := data.TryTransposedReshapedMatrixRowsNumericSumPlusCount(matrixShape, source, rows...)
	return qTypedRuntimeResultReason("MatrixRowsSumCount", shape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func (s *EvalState) evalQScriptMatrixRowsSumCountRows(descriptor *qScriptPipelineDescriptor) ([]int, bool, error) {
	rowExprs := append([]string{descriptor.rowIndexExpr}, strings.Fields(descriptor.indexExpr)...)
	if len(rowExprs) < 2 {
		return nil, false, nil
	}
	rows := make([]int, 0, len(rowExprs))
	for _, expr := range rowExprs {
		plan := buildQScriptBindingPlanForRHS(expr, nil)
		row, handled, err := s.evalQScriptScalarIndexPlan(&plan)
		if err != nil || !handled {
			return nil, handled, err
		}
		rows = append(rows, row)
	}
	return rows, true, nil
}

func qScriptPipelineAssignmentResolver(descriptor *qScriptPipelineDescriptor) qScriptBindingNameResolver {
	return func(name string) (*qScriptBindingPlan, bool, error) {
		assignment, ok := qScriptPipelineAssignmentByName(descriptor, name)
		if !ok {
			return nil, false, nil
		}
		return &assignment.binding, true, nil
	}
}

func (s *EvalState) evalQScriptReshapedMatrixRowSumCount(plan *qScriptBindingPlan, row int) (any, bool, error) {
	if plan == nil || plan.kind != qScriptBindingBinary || plan.op != "#" || plan.left == nil || plan.right == nil {
		return nil, false, nil
	}
	shapeValue, handled, err := s.evalQScriptBindingPlan(plan.left)
	if err != nil || !handled {
		return nil, handled, err
	}
	shape, nullDim, err := qReshapeShape(shapeValue)
	if err != nil {
		return nil, true, err
	}
	if nullDim >= 0 || len(shape) != 2 {
		return nil, false, nil
	}
	sourceValue, handled, err := s.evalQScriptBindingPlan(plan.right)
	if err != nil || !handled {
		return nil, handled, err
	}
	source, ok := sourceValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	kernelShape := "matrix-dot/reshape/" + qRuntimeCardinalityShape(shape[0]) + "x" + qRuntimeCardinalityShape(shape[1]) + "/1-index/sum-count"
	out, handled, err := data.TryReshapedMatrixRowNumericSumCount(shape, source, row)
	return qTypedRuntimeResultReason("MatrixRowSumCount", kernelShape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func (s *EvalState) evalQScriptMatrixCellPlusCountPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	for _, assignment := range descriptor.assignments {
		if strings.TrimSpace(descriptor.rowValueExpr) == assignment.name {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	row, handled, err := s.evalQScriptScalarIndexPlan(&descriptor.rowIndexPlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	col, handled, err := s.evalQScriptScalarIndexPlan(&descriptor.colIndexPlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	if assignment, ok := qScriptPipelineAssignmentByName(descriptor, descriptor.rowValueExpr); ok {
		if out, handled, err := s.evalQScriptReshapedMatrixCellPlusCount(&assignment.binding, row, col); err != nil || handled {
			return out, handled, err
		}
	}
	matrixValue, handled, err := s.evalQScriptBindingPlan(&descriptor.rowValuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.rowValueExpr); err != nil {
			return nil, true, err
		}
		matrixValue, handled, err = s.evalQScriptBindingPlan(&descriptor.rowValuePlan)
		if err != nil {
			return nil, true, err
		}
	}
	if !handled {
		return nil, false, nil
	}
	matrix, ok := matrixValue.(data.Matrix)
	if !ok {
		return nil, false, nil
	}
	shape := qMatrixIndexShape(matrix, 2) + "/cell-plus-count"
	out, handled, err := data.TryMatrixCellNumericPlusCount(matrix, row, col)
	return qTypedRuntimeResultReason("MatrixCellPlusCount", shape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func (s *EvalState) evalQScriptMatrixNestedCellPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	for _, assignment := range descriptor.assignments {
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	sumValue, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	sumMatrix, ok := sumValue.(data.Matrix)
	if !ok {
		return nil, false, nil
	}
	cellValue, handled, err := s.evalQScriptBindingPlan(&descriptor.rowValuePlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	cellMatrix, ok := cellValue.(data.Matrix)
	if !ok {
		return nil, false, nil
	}
	countValue, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	countMatrix, ok := countValue.(data.Matrix)
	if !ok {
		return nil, false, nil
	}
	row, handled, err := s.evalQScriptScalarIndexPlan(&descriptor.rowIndexPlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	col, handled, err := s.evalQScriptScalarIndexPlan(&descriptor.colIndexPlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	shape := qMatrixIndexShape(sumMatrix, 2) + "/nested-sum-cell-count"
	out, handled, err := data.TryMatrixNestedSumCellPlusCount(sumMatrix, cellMatrix, countMatrix, row, col)
	return qTypedRuntimeResultReason("MatrixNestedSumCellCount", shape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func qScriptPipelineAssignmentByName(descriptor *qScriptPipelineDescriptor, name string) (qScriptPipelineAssignment, bool) {
	name = strings.TrimSpace(name)
	if descriptor == nil || name == "" {
		return qScriptPipelineAssignment{}, false
	}
	for _, assignment := range descriptor.assignments {
		if assignment.name == name {
			return assignment, true
		}
	}
	return qScriptPipelineAssignment{}, false
}

func (s *EvalState) evalQScriptReshapedMatrixCellPlusCount(plan *qScriptBindingPlan, row, col int) (any, bool, error) {
	if plan == nil || plan.kind != qScriptBindingBinary || plan.op != "#" || plan.left == nil || plan.right == nil {
		return nil, false, nil
	}
	shapeValue, handled, err := s.evalQScriptBindingPlan(plan.left)
	if err != nil || !handled {
		return nil, handled, err
	}
	shape, nullDim, err := qReshapeShape(shapeValue)
	if err != nil {
		return nil, true, err
	}
	if nullDim >= 0 || len(shape) != 2 {
		return nil, false, nil
	}
	sourceValue, handled, err := s.evalQScriptBindingPlan(plan.right)
	if err != nil || !handled {
		return nil, handled, err
	}
	source, ok := sourceValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	kernelShape := "matrix-reshape-dot/" + qRuntimeCardinalityShape(shape[0]) + "x" + qRuntimeCardinalityShape(shape[1]) + "/2-indexes/cell-plus-count"
	out, handled, err := data.TryReshapedMatrixCellNumericPlusCount(shape, source, row, col)
	return qTypedRuntimeResultReason("MatrixCellPlusCount", kernelShape, RuntimeFallbackUnsupportedType, out, handled, err)
}

func (s *EvalState) evalQScriptScalarIndexPlan(plan *qScriptBindingPlan) (int, bool, error) {
	value, handled, err := s.evalQScriptBindingPlan(plan)
	if err != nil || !handled {
		return 0, handled, err
	}
	indexes, scalar, err := indexInts(value)
	if err != nil {
		// Negative/null indexes: fall back to the generic read path, which
		// null-fills out-of-range reads.
		return 0, false, nil
	}
	if !scalar || len(indexes) != 1 {
		return 0, false, nil
	}
	return indexes[0], true, nil
}

func (s *EvalState) evalQScriptCallableDotSumPlusRightPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	for _, assignment := range descriptor.assignments {
		if strings.TrimSpace(descriptor.callableExpr) == assignment.name {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	left, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	right, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	if descriptor.includeCount {
		out, err := qLambdaFastSumPlusCountRightValue(left, right)
		return out, true, err
	}
	sumValue, err := sum(left)
	if err != nil {
		return nil, true, err
	}
	out, err := applyDyadic('+', sumValue, right)
	return out, true, err
}

func (s *EvalState) evalQScriptCallableOverScanSumPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	resolver := func(name string) (*qScriptBindingPlan, bool, error) {
		for i := range descriptor.assignments {
			if descriptor.assignments[i].name == name {
				if descriptor.assignments[i].binding.kind == qScriptBindingInvalid {
					descriptor.assignments[i].binding = buildQScriptBindingPlanForRHS(descriptor.assignments[i].rhs, descriptor.assignments[i].valueExpr)
				}
				return &descriptor.assignments[i].binding, true, nil
			}
		}
		return nil, false, nil
	}
	if summary, handled, err := s.evalQScriptNumericSummaryWithResolver(&descriptor.valuePlan, resolver); err != nil || handled {
		if err != nil {
			return nil, true, err
		}
		initial, initialHandled, err := s.evalQScriptBindingPlan(&descriptor.scalarPlan)
		if err != nil || !initialHandled {
			return nil, initialHandled, err
		}
		return s.evalQScriptCallableOverScanSumSummary(initial, summary, "summary/"+string(summary.kind))
	}
	value, handled, err := s.evalQScriptBindingPlanWithResolver(&descriptor.valuePlan, resolver)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		for i := range descriptor.assignments {
			if descriptor.assignments[i].name != strings.TrimSpace(descriptor.valueExpr) {
				continue
			}
			value, err = s.evalCachedOrString(descriptor.assignments[i].rhs, descriptor.assignments[i].valueExpr, &descriptor.assignments[i].binding, nil)
			if err != nil {
				return nil, true, err
			}
			handled = true
			break
		}
	}
	if !handled {
		return nil, false, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	initial, handled, err := s.evalQScriptBindingPlan(&descriptor.scalarPlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	shape := "callable-over/scan-sum-count/" + string(array.Kind())
	sumValue, handled, err := data.TryTypedNumericSum(array)
	sumValue, handled, err = qTypedRuntimeResultReason("CallableOverScanSum", shape, RuntimeFallbackUnsupportedType, sumValue, handled, err)
	if err != nil || !handled {
		return nil, handled, err
	}
	if out, ok := applyScalarNumericAdd4(initial, sumValue, sumValue, int64(array.Len())); ok {
		return out, true, nil
	}
	out, err := applyDyadic('+', initial, sumValue)
	if err == nil {
		out, err = applyDyadic('+', out, sumValue)
	}
	if err == nil {
		out, err = applyDyadic('+', out, int64(array.Len()))
	}
	return out, true, err
}

type qScriptNumericSummary struct {
	kind   data.Kind
	count  int64
	sum    any
	sumI64 int64
	sumF64 float64
	hasI64 bool
	hasF64 bool
}

func (s *EvalState) evalQScriptCallableOverScanSumSummary(initial any, summary qScriptNumericSummary, shapeSuffix string) (any, bool, error) {
	shape := qCallableOverScanSumShape(shapeSuffix)
	if summary.kind == data.KindI64 && summary.hasI64 {
		sumValue, handled, err := qTypedRuntimeResultReason("CallableOverScanSum", shape, RuntimeFallbackUnsupportedType, summary.sumI64, true, nil)
		if err != nil || !handled {
			return nil, handled, err
		}
		if initialValue, ok := integerValue(initial); ok {
			return initialValue + sumValue + sumValue + summary.count, true, nil
		}
	}
	sumValue, handled, err := qTypedRuntimeResultReason("CallableOverScanSum", shape, RuntimeFallbackUnsupportedType, summary.sum, true, nil)
	if err != nil || !handled {
		return nil, handled, err
	}
	if out, ok := applyScalarNumericAdd4(initial, sumValue, sumValue, summary.count); ok {
		return out, true, nil
	}
	out, err := applyDyadic('+', initial, sumValue)
	if err != nil {
		return nil, true, err
	}
	out, err = applyDyadic('+', out, sumValue)
	if err != nil {
		return nil, true, err
	}
	out, err = applyDyadic('+', out, summary.count)
	return out, true, err
}

func qCallableOverScanSumShape(shapeSuffix string) string {
	switch shapeSuffix {
	case "summary/i64":
		return "callable-over/scan-sum-count/summary/i64"
	case "summary/f64":
		return "callable-over/scan-sum-count/summary/f64"
	default:
		return "callable-over/scan-sum-count/" + shapeSuffix
	}
}

func (s *EvalState) evalQScriptNumericSummaryWithResolver(plan *qScriptBindingPlan, resolver qScriptBindingNameResolver) (qScriptNumericSummary, bool, error) {
	if plan == nil {
		return qScriptNumericSummary{}, false, nil
	}
	switch plan.kind {
	case qScriptBindingName:
		if resolver != nil {
			resolved, ok, err := resolver(plan.name)
			if err != nil || ok {
				if err != nil || resolved == nil {
					return qScriptNumericSummary{}, ok, err
				}
				return s.evalQScriptNumericSummaryWithResolver(resolved, resolver)
			}
		}
		value, ok := s.lookupName(plan.name)
		if !ok {
			return qScriptNumericSummary{}, false, nil
		}
		return qNumericSummaryFromValue(value)
	case qScriptBindingUnary:
		if plan.op != "til" {
			return qScriptNumericSummary{}, false, nil
		}
		arg, handled, err := s.evalQScriptBindingPlanWithResolver(plan.left, resolver)
		if err != nil || !handled {
			return qScriptNumericSummary{}, handled, err
		}
		n, ok := integerValue(arg)
		if !ok || n < 0 {
			return qScriptNumericSummary{}, false, nil
		}
		sum := qArithmeticSeriesSumI64(0, 1, n)
		return qScriptNumericSummary{kind: data.KindI64, count: n, sum: sum, sumI64: sum, hasI64: true}, true, nil
	case qScriptBindingBinary:
		if plan.op != "+" && plan.op != "-" && plan.op != "*" {
			return qScriptNumericSummary{}, false, nil
		}
		leftSummary, leftHandled, err := s.evalQScriptNumericSummaryWithResolver(plan.left, resolver)
		if err != nil {
			return qScriptNumericSummary{}, true, err
		}
		rightSummary, rightHandled, err := s.evalQScriptNumericSummaryWithResolver(plan.right, resolver)
		if err != nil {
			return qScriptNumericSummary{}, true, err
		}
		if leftHandled && rightHandled {
			return qScriptNumericSummary{}, false, nil
		}
		if leftHandled {
			scalar, handled, err := s.evalQScriptScalarNumericWithResolver(plan.right, resolver)
			if err != nil || !handled {
				return qScriptNumericSummary{}, handled, err
			}
			return qScriptNumericSummaryApplyScalar(leftSummary, scalar, false, plan.op)
		}
		if rightHandled {
			scalar, handled, err := s.evalQScriptScalarNumericWithResolver(plan.left, resolver)
			if err != nil || !handled {
				return qScriptNumericSummary{}, handled, err
			}
			return qScriptNumericSummaryApplyScalar(rightSummary, scalar, true, plan.op)
		}
	}
	value, handled, err := s.evalQScriptBindingPlanWithResolver(plan, resolver)
	if err != nil || !handled {
		return qScriptNumericSummary{}, handled, err
	}
	return qNumericSummaryFromValue(value)
}

func (s *EvalState) evalQScriptScalarNumericWithResolver(plan *qScriptBindingPlan, resolver qScriptBindingNameResolver) (any, bool, error) {
	value, handled, err := s.evalQScriptBindingPlanWithResolver(plan, resolver)
	if err != nil || !handled {
		return nil, handled, err
	}
	if _, ok := numeric(value); !ok {
		return nil, false, nil
	}
	return value, true, nil
}

func qNumericSummaryFromValue(value any) (qScriptNumericSummary, bool, error) {
	array, ok := value.(data.Array)
	if !ok {
		return qScriptNumericSummary{}, false, nil
	}
	sum, handled, err := data.TryTypedNumericSum(array)
	if err != nil || !handled {
		return qScriptNumericSummary{}, handled, err
	}
	summary := qScriptNumericSummary{kind: array.Kind(), count: int64(array.Len()), sum: sum}
	switch v := sum.(type) {
	case int64:
		summary.sumI64 = v
		summary.hasI64 = true
	case int:
		summary.sumI64 = int64(v)
		summary.hasI64 = true
	case float64:
		summary.sumF64 = v
		summary.hasF64 = true
	case float32:
		summary.sumF64 = float64(v)
		summary.hasF64 = true
	}
	return summary, true, nil
}

func qScriptNumericSummaryApplyScalar(summary qScriptNumericSummary, scalar any, scalarLeft bool, op string) (qScriptNumericSummary, bool, error) {
	if summary.kind != data.KindI64 || !summary.hasI64 {
		return qScriptNumericSummary{}, false, nil
	}
	sum := summary.sumI64
	n := summary.count
	scalarI64, ok := integerValue(scalar)
	if !ok {
		return qScriptNumericSummary{}, false, nil
	}
	switch op {
	case "+":
		sum += scalarI64 * n
	case "-":
		if scalarLeft {
			sum = scalarI64*n - sum
		} else {
			sum -= scalarI64 * n
		}
	case "*":
		sum *= scalarI64
	default:
		return qScriptNumericSummary{}, false, nil
	}
	summary.sum = sum
	summary.sumI64 = sum
	summary.hasI64 = true
	return summary, true, nil
}

func qArithmeticSeriesSumI64(start, step, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return count * (2*start + (count-1)*step) / 2
}

func (s *EvalState) evalQScriptStringJoinCountsPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	values := descriptor.stringValues
	count := descriptor.stringRepeatCount
	sep := descriptor.stringSep
	search := descriptor.stringSearch
	old := descriptor.stringReplaceOld
	repl := descriptor.stringReplaceNew
	if len(values) == 0 {
		qScriptPipelineHoistStringJoinCounts(descriptor)
		values = descriptor.stringValues
		count = descriptor.stringRepeatCount
		sep = descriptor.stringSep
		search = descriptor.stringSearch
		old = descriptor.stringReplaceOld
		repl = descriptor.stringReplaceNew
		if len(values) == 0 {
			return nil, false, nil
		}
	}
	out, ok := data.RepeatedStringJoinCountSummary(values, count, sep, search, old, repl)
	if !ok {
		return nil, false, nil
	}
	return out.SplitCount + out.SearchCount + out.ReplaceResultLen, true, nil
}

func qScriptPipelineRepeatedStringValues(src string) ([]string, int, bool) {
	left, right, ok := splitTopLevelOperator(stripEnclosingParens(strings.TrimSpace(src)), "#")
	if !ok {
		return nil, 0, false
	}
	count, ok := qScriptPipelineStaticInt(left)
	if !ok || count < 0 {
		return nil, 0, false
	}
	value, ok := qScriptPipelineStaticValue(right)
	if !ok {
		return nil, 0, false
	}
	items := data.SequenceItems(value)
	if len(items) == 0 {
		if text, err := qStringOperand("string repeat", value); err == nil {
			items = []any{text}
		}
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		text, err := qStringOperand("string repeat", item)
		if err != nil {
			return nil, 0, false
		}
		values = append(values, text)
	}
	if len(values) == 0 {
		return nil, 0, false
	}
	return values, count, true
}

func (s *EvalState) evalQScriptSequenceEdgeSumPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	if len(descriptor.sequenceSteps) > 0 && strings.TrimSpace(descriptor.sequenceValueExpr) != "" {
		return s.evalQScriptSequenceTransformChainEdgeSumPipeline(descriptor)
	}
	for _, assignment := range descriptor.assignments {
		if strings.TrimSpace(descriptor.valueExpr) == assignment.name {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.valueExpr); err != nil {
			return nil, true, err
		}
		value, handled, err = s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
	}
	if !handled {
		return nil, false, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-reduce/sum-first-last/" + string(array.Kind())
	out, handled, err := data.TryTypedNumericSumFirstLast(array)
	out, handled, err = qTypedRuntimeResultReason("SequenceEdgeReduce", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	return out, handled, err
}

func (s *EvalState) evalQScriptSequenceTransformChainEdgeSumPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	for _, assignment := range descriptor.assignments {
		if qScriptPipelineNameIn(descriptor.sequenceBindings, assignment.name) {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	value, handled, err := s.evalQScriptBindingPlan(&descriptor.sequenceValuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	shape := "vector-transform-chain/sum-first-last/" + descriptor.sequenceShapeName + "/" + string(qRuntimeKernelOperandKind(value, nil))
	out, handled, err := data.TryTypedSequenceTransformChainNumericSumFirstLast(descriptor.sequenceSteps, value)
	out, handled, err = qTypedRuntimeResultReason("SequenceTransformChainEdgeReduce", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	return out, handled, err
}

func (s *EvalState) evalQScriptSequenceTransformChainSumCountPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	for _, assignment := range descriptor.assignments {
		if qScriptPipelineNameIn(descriptor.sequenceBindings, assignment.name) {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	value, handled, err := s.evalQScriptBindingPlan(&descriptor.sequenceValuePlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	shape := "vector-transform-chain/sum-count/" + descriptor.sequenceShapeName + "/" + string(qRuntimeKernelOperandKind(value, nil))
	out, handled, err := data.TryTypedSequenceTransformChainNumericSumCount(descriptor.sequenceSteps, value)
	out, handled, err = qTypedRuntimeResultReason("SequenceTransformChainSumCount", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	return out, handled, err
}

func (s *EvalState) evalQScriptGatherSumCountPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	for _, assignment := range descriptor.assignments {
		name := strings.TrimSpace(assignment.name)
		if qScriptPipelineAssignmentAbsorbed(descriptor, name) {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	if qScriptPipelineHasAssignment(descriptor, descriptor.valueExpr) {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.valueExpr); err != nil {
			return nil, true, err
		}
	}
	value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.valueExpr); err != nil {
			return nil, true, err
		}
		value, handled, err = s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil || !handled {
			return nil, handled, err
		}
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	if out, handled, err := s.evalQScriptGatherSumCountWhereIndexPipeline(descriptor, array); err != nil || handled {
		return out, handled, err
	}
	if qScriptPipelineHasAssignment(descriptor, descriptor.indexExpr) {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.indexExpr); err != nil {
			return nil, true, err
		}
	}
	index, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.indexExpr); err != nil {
			return nil, true, err
		}
		index, handled, err = s.evalQScriptBindingPlan(&descriptor.indexPlan)
		if err != nil || !handled {
			return nil, handled, err
		}
	}
	indexes, ok := index.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "gather-reduce/sum-count/" + string(array.Kind()) + "/" + string(indexes.Kind())
	sumValue, handled, err := data.TryTypedNumericSumByI64Indexes(array, indexes)
	sumValue, handled, err = qTypedRuntimeResultReason("ArrayGatherSumCount", shape, RuntimeFallbackUnsupportedType, sumValue, handled, err)
	if err != nil || !handled {
		return sumValue, handled, err
	}
	out, ok := dataNumericAddCount(sumValue, indexes.Len())
	if !ok {
		return nil, false, nil
	}
	return out, true, nil
}

func qScriptPipelineHasAssignment(descriptor *qScriptPipelineDescriptor, name string) bool {
	if descriptor == nil {
		return false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, assignment := range descriptor.assignments {
		if strings.TrimSpace(assignment.name) == name {
			return true
		}
	}
	return false
}

func (s *EvalState) evalQScriptIndexExprSumCountPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	skip := descriptor.indexExprAssignmentSkip
	if len(skip) != len(descriptor.assignments) {
		skip = qScriptPipelineIndexExprSkippedAssignments(descriptor)
	}
	for i, assignment := range descriptor.assignments {
		if i < len(skip) && skip[i] {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	index, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.indexExpr); err != nil {
			return nil, true, err
		}
		index, handled, err = s.evalQScriptBindingPlan(&descriptor.indexPlan)
		if err != nil || !handled {
			return nil, handled, err
		}
	}
	indexes, ok := index.(data.Array)
	if !ok {
		return nil, false, nil
	}
	reducers := descriptor.indexReducers
	if len(reducers) == 0 {
		reducers = []data.I64IndexExprReducer{{Kind: data.I64IndexExprReducerSum, Expr: descriptor.indexExprPlan}}
		if descriptor.includeCount {
			reducers = append(reducers, data.I64IndexExprReducer{Kind: data.I64IndexExprReducerCount})
		}
	}
	shape := "index-expr-reduce/reducers/" + string(indexes.Kind()) + "/" + strconv.Itoa(len(reducers))
	values, handled, err := data.TryTypedI64IndexExprReducers(indexes, reducers)
	values, handled, err = qTypedRuntimeResult("ArrayIndexExprReducers", shape, values, handled, err)
	if err != nil || !handled {
		return values, handled, err
	}
	var total int64
	for _, value := range values {
		total += value
	}
	return total, true, nil
}

// buildQScriptGatherSumCountWhereIndexPlan is the purely syntactic half of
// evalQScriptGatherSumCountWhereIndexPipeline; the result is memoized on the
// descriptor so warm statements skip the per-call string walk and operand
// normalization.
func buildQScriptGatherSumCountWhereIndexPlan(descriptor *qScriptPipelineDescriptor) *qPipelinePlan {
	whereExpr := strings.TrimSpace(descriptor.indexBinding)
	if whereExpr == "" {
		if maskExpr, ok := directWhereMaskExpr(descriptor.indexExpr); ok {
			whereExpr = "where " + maskExpr
		}
	}
	if whereExpr == "" {
		return nil
	}
	plan, ok := buildQPipelineWhereComparePlan(whereExpr, qPipelineWhereCompareIndexes, "compare-to-index-count-sum")
	if !ok || plan.kind != qPipelineWhereCompareIndexes {
		return nil
	}
	plan = qPipelinePlanWithBindingPlans(plan)
	return &plan
}

func (s *EvalState) evalQScriptGatherSumCountWhereIndexPipeline(descriptor *qScriptPipelineDescriptor, array data.Array) (any, bool, error) {
	if !descriptor.whereIndexPlanBuilt {
		descriptor.whereIndexPlanBuilt = true
		descriptor.whereIndexPlan = buildQScriptGatherSumCountWhereIndexPlan(descriptor)
	}
	if descriptor.whereIndexPlan == nil {
		return nil, false, nil
	}
	plan := descriptor.whereIndexPlan
	if isIdentityI64RangeArray(array) {
		count, sum, handled, err := s.evalQPipelineWhereCompareIndexStats(plan)
		if err != nil || !handled {
			return nil, handled, err
		}
		out, ok := dataNumericAddCount(sum, int(count))
		if !ok {
			return nil, false, nil
		}
		return out, true, nil
	}
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return nil, false, nil
	}
	// When the gathered value expression is the same pure expression as the
	// predicate source, the data layer can share one bulk carrier flatten
	// between the selection mask and the reduction.
	selfPredicate := strings.TrimSpace(plan.leftExpr) == strings.TrimSpace(descriptor.valueExpr)
	runtimePlan, ok, err := qTypedWhereGatherSumCountDescriptorFor(array, left, right, plan.compareOp, "where-index-reduce/sum-count", selfPredicate)
	if err != nil {
		return nil, false, nil
	}
	if !ok {
		return nil, false, nil
	}
	sum, count, handled, err := evalQTypedWhereGatherSumCount(runtimePlan)
	if err != nil || !handled {
		return nil, handled, err
	}
	out, ok := dataNumericAddCount(sum, int(count))
	if !ok {
		return nil, false, nil
	}
	return out, true, nil
}

func dataNumericAddCount(value any, count int) (any, bool) {
	switch x := value.(type) {
	case int64:
		return x + int64(count), true
	case int:
		return int64(x) + int64(count), true
	case float64:
		return x + float64(count), true
	case float32:
		return float64(x) + float64(count), true
	default:
		n, ok := numeric(value)
		if !ok {
			return nil, false
		}
		return n + float64(count), true
	}
}

func qScriptPipelineNameIn(names []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, name := range names {
		if strings.TrimSpace(name) == target {
			return true
		}
	}
	return false
}

func qScriptPipelineSequenceTransformName(steps []data.SequenceTransformStep) string {
	if len(steps) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		switch step.Transform {
		case data.SequenceTransformReverse:
			parts = append(parts, "reverse")
		case data.SequenceTransformRotate:
			parts = append(parts, "rotate")
		case data.SequenceTransformDrop:
			parts = append(parts, "drop")
		case data.SequenceTransformSublist:
			parts = append(parts, "sublist")
		default:
			parts = append(parts, step.Transform)
		}
	}
	return strings.Join(parts, ".")
}

func encodeQScriptPipelineSequenceTransformSteps(steps []data.SequenceTransformStep) string {
	if len(steps) == 0 {
		return ""
	}
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		part := strings.TrimSpace(step.Transform)
		switch step.ArgCount {
		case 0:
		case 1:
			part += ":" + strconv.Itoa(step.Args[0])
		case 2:
			part += ":" + strconv.Itoa(step.Args[0]) + "," + strconv.Itoa(step.Args[1])
		default:
			return ""
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "|")
}

func decodeQScriptPipelineSequenceTransformSteps(text string) ([]data.SequenceTransformStep, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, true
	}
	parts := strings.Split(text, "|")
	steps := make([]data.SequenceTransformStep, 0, len(parts))
	for _, part := range parts {
		name, argText, _ := strings.Cut(strings.TrimSpace(part), ":")
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, false
		}
		step := data.SequenceTransformStep{Transform: name}
		if argText != "" {
			args, ok := parseQScriptPipelineSequenceTransformArgs(argText)
			if !ok {
				return nil, false
			}
			step.ArgCount = len(args)
			copy(step.Args[:], args)
		}
		switch step.Transform {
		case data.SequenceTransformReverse:
			if step.ArgCount != 0 {
				return nil, false
			}
		case data.SequenceTransformRotate:
			if step.ArgCount != 1 {
				return nil, false
			}
		case data.SequenceTransformDrop:
			if step.ArgCount != 1 {
				return nil, false
			}
		case data.SequenceTransformSublist:
			if step.ArgCount != 1 && step.ArgCount != 2 {
				return nil, false
			}
		default:
			return nil, false
		}
		steps = append(steps, step)
	}
	return steps, true
}

func parseQScriptPipelineSequenceTransformArgs(text string) ([]int, bool) {
	if text == "" {
		return nil, true
	}
	parts := strings.Split(text, ",")
	if len(parts) == 0 || len(parts) > 2 {
		return nil, false
	}
	args := make([]int, len(parts))
	for i, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, false
		}
		args[i] = n
	}
	return args, true
}

func encodeQScriptPipelineNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\x1f')
		}
		b.WriteString(name)
	}
	return b.String()
}

func decodeQScriptPipelineNames(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\x1f")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (s *EvalState) evalQScriptTerminalPipeline(descriptor *qScriptPipelineDescriptor, terminal *qPipelinePlan) (any, bool, error) {
	switch terminal.kind {
	case qPipelineApplyScalarIndex, qPipelineApplyGatherIndex:
		return s.evalQPipelinePlan(terminal)
	case qPipelineFindSum:
		left, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		right, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		desc, ok := qTypedFindDescriptorFor(left, right)
		if !ok {
			return nil, false, nil
		}
		out, handled, err := evalQTypedFindSum(desc)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return nil, false, nil
		}
		return out, true, nil
	case qPipelineSumWhereIndex:
		value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		array, ok := value.(data.Array)
		if !ok {
			return nil, false, nil
		}
		if descriptor.moduloMaskPlan != nil {
			if out, handled, err := s.evalQPipelineModuloCompareValueSum(descriptor.moduloMaskPlan, array); err != nil || handled {
				return out, handled, err
			}
		}
		mask, handled, err := s.evalQScriptBindingPlan(&descriptor.maskPlan)
		if err != nil {
			return nil, true, err
		}
		if handled {
			if maskArray, ok := mask.(data.Array); ok {
				out, handled, err := qPipelineWhereReduceSumWithPlanStats(terminal, array, maskArray)
				if err != nil || handled {
					return out, handled, err
				}
			}
		}
		index, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.indexExpr); err != nil {
				return nil, true, err
			}
			index, handled, err = s.evalQScriptBindingPlan(&descriptor.indexPlan)
			if err != nil {
				return nil, true, err
			}
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		indexes, ok := index.(data.Array)
		if !ok {
			return nil, false, nil
		}
		return qPipelineGatherReduceSumWithPlanStats(terminal, array, indexes)
	case qPipelineSumGatherIndexes:
		value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		array, ok := value.(data.Array)
		if !ok {
			return nil, false, nil
		}
		if descriptor.kind == qScriptPipelineWhereIndexReduceSum {
			if descriptor.moduloMaskPlan != nil {
				if out, handled, err := s.evalQPipelineModuloCompareValueSum(descriptor.moduloMaskPlan, array); err != nil || handled {
					return out, handled, err
				}
			}
		}
		index, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.indexExpr); err != nil {
				return nil, true, err
			}
			index, handled, err = s.evalQScriptBindingPlan(&descriptor.indexPlan)
			if err != nil {
				return nil, true, err
			}
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		indexes, ok := index.(data.Array)
		if !ok {
			return nil, false, nil
		}
		return qPipelineGatherReduceSumWithPlanStats(terminal, array, indexes)
	case qPipelineSumWhereMask:
		value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		array, ok := value.(data.Array)
		if !ok {
			return nil, false, nil
		}
		mask, handled, err := s.evalQScriptBindingPlan(&descriptor.maskPlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		maskArray, ok := mask.(data.Array)
		if !ok {
			return nil, false, nil
		}
		return qPipelineWhereReduceSumWithPlanStats(terminal, array, maskArray)
	default:
		return s.evalQPipelinePlan(terminal)
	}
}

func qScriptPipelineCanDeferAssignment(descriptor *qScriptPipelineDescriptor, assignment qScriptPipelineAssignment) bool {
	if descriptor == nil {
		return false
	}
	switch descriptor.kind {
	case qScriptPipelineFindReduceSum:
		name := strings.TrimSpace(assignment.name)
		return name != "" && (name == strings.TrimSpace(descriptor.valueExpr) || name == strings.TrimSpace(descriptor.indexExpr))
	case qScriptPipelineWhereIndexReduceSum:
		if strings.TrimSpace(descriptor.indexExpr) != assignment.name {
			return false
		}
		return descriptor.moduloMaskPlan != nil
	default:
		return false
	}
}

func qScriptPipelineModuloMaskPlan(maskExpr string) *qPipelinePlan {
	if maskExpr == "" {
		return nil
	}
	plan, ok := qPipelineModuloComparePlanFromMask(maskExpr)
	if !ok {
		return nil
	}
	return &plan
}

func (s *EvalState) evalQScriptPipelineDeferredAssignment(descriptor *qScriptPipelineDescriptor, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	for _, assignment := range descriptor.assignments {
		if assignment.name != name {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(assignment.name)] = value
		return nil
	}
	return nil
}
