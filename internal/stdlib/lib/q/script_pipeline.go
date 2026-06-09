package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptPipelineKind string

const (
	qScriptPipelineWhereReduceSum      qScriptPipelineKind = "where-reduce/sum"
	qScriptPipelineWhereIndexReduceSum qScriptPipelineKind = "where-index-reduce/sum"
	qScriptPipelineGatherReduceSum     qScriptPipelineKind = "gather-reduce/sum"
	qScriptPipelineSequenceEdgeSum     qScriptPipelineKind = "sequence-edge-reduce/sum-first-last"
	qScriptPipelineSumPlusDyadicFloat  qScriptPipelineKind = "multi-reduce/sum-plus-dyadic-float-sum"
	qScriptPipelineMatrixRowSumCount   qScriptPipelineKind = "matrix-row-reduce/sum-count"
	qScriptPipelineCallableDotSumRight qScriptPipelineKind = "callable-dot/sum-plus-right"
	qScriptPipelineApplyScalarAt       qScriptPipelineKind = "apply-index/scalar-at"
	qScriptPipelineApplyScalarDot      qScriptPipelineKind = "apply-index/scalar-dot"
	qScriptPipelineApplyPathDot        qScriptPipelineKind = "apply-index/path-dot"
	qScriptPipelineUnsupported         qScriptPipelineKind = ""
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
	rowValuePlan      qScriptBindingPlan
	rowIndexPlan      qScriptBindingPlan
	callableExpr      string
	callableBinding   string
	dyadicOp          string
	scalarExpr        string
	scalarPlan        qScriptBindingPlan
	scalarLeft        bool
	terminalUsesWhere bool
	terminalPlan      qPipelinePlan
	moduloMaskPlan    *qPipelinePlan
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
		if _, _, ok := parseDeferredScan(stmt.rhs); ok {
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
	descriptor.valuePlan = buildQScriptBindingPlanForRHS(descriptor.valueExpr, nil)
	descriptor.indexPlan = buildQScriptBindingPlanForRHS(descriptor.indexExpr, nil)
	descriptor.maskPlan = buildQScriptBindingPlanForRHS(descriptor.maskExpr, nil)
	descriptor.rowValuePlan = buildQScriptBindingPlanForRHS(descriptor.rowValueExpr, nil)
	descriptor.rowIndexPlan = buildQScriptBindingPlanForRHS(descriptor.rowIndexExpr, nil)
	descriptor.scalarPlan = buildQScriptBindingPlanForRHS(descriptor.scalarExpr, nil)
	descriptor.moduloMaskPlan = qScriptPipelineModuloMaskPlan(descriptor.maskExpr)
	descriptor.shapeText = descriptor.shape()
	return &descriptor, true
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
		return qScriptPipelineDescriptor{
			kind:         qScriptPipelineSequenceEdgeSum,
			valueExpr:    valueExpr,
			valueBinding: qScriptPipelineBinding(valueExpr, bindings),
		}, true
	}
	if descriptor, ok := qScriptPipelineSumPlusDyadicFloatDescriptor(src, bindings); ok {
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
	if !strings.HasPrefix(src, "+/") {
		return qScriptPipelineDescriptor{}, false
	}
	body := strings.TrimSpace(src[len("+/"):])
	if body == "" {
		return qScriptPipelineDescriptor{}, false
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
	if !ok || fastPlan.kind != qLambdaFastSumPlusRight {
		return qScriptPipelineDescriptor{}, false
	}
	leftExpr := strings.TrimSpace(plan.argExprs[0])
	rightExpr := strings.TrimSpace(plan.argExprs[1])
	if leftExpr == "" || rightExpr == "" {
		return qScriptPipelineDescriptor{}, false
	}
	return qScriptPipelineDescriptor{
		kind:            qScriptPipelineCallableDotSumRight,
		callableExpr:    callableExpr,
		callableBinding: binding,
		valueExpr:       leftExpr,
		valueBinding:    qScriptPipelineBinding(leftExpr, bindings),
		indexExpr:       rightExpr,
		indexBinding:    qScriptPipelineBinding(rightExpr, bindings),
	}, true
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

func qScriptPipelinePlusTerms(src string) []string {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if left, right, ok := splitTopLevelOperator(src, "+"); ok {
		terms := qScriptPipelinePlusTerms(left)
		terms = append(terms, qScriptPipelinePlusTerms(right)...)
		return terms
	}
	if src == "" {
		return nil
	}
	return []string{src}
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
	recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "attempt", "attempt")
	if descriptor.kind == qScriptPipelineSequenceEdgeSum {
		out, handled, err := s.evalQScriptSequenceEdgeSumPipeline(descriptor)
		switch {
		case err != nil:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
		case handled:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "hit", "typed_pipeline")
		default:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
		}
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineSumPlusDyadicFloat {
		out, handled, err := s.evalQScriptSumPlusDyadicFloatPipeline(descriptor)
		switch {
		case err != nil:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
		case handled:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "hit", "typed_pipeline")
		default:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
		}
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineMatrixRowSumCount {
		out, handled, err := s.evalQScriptMatrixRowSumCountPipeline(descriptor)
		switch {
		case err != nil:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
		case handled:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "hit", "typed_pipeline")
		default:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
		}
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineCallableDotSumRight {
		out, handled, err := s.evalQScriptCallableDotSumPlusRightPipeline(descriptor)
		switch {
		case err != nil:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
		case handled:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "hit", "typed_pipeline")
		default:
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
		}
		return out, handled, err
	}
	terminal := descriptor.terminalPlan
	if terminal.kind == qPipelineInvalid {
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", RuntimeFallbackPlannerUnhandled)
		return nil, false, nil
	}
	s.rememberQPipelinePlan(descriptor.terminal, terminal)
	for _, assignment := range descriptor.assignments {
		if qScriptPipelineCanDeferAssignment(descriptor, assignment) {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
				return nil, true, err
			}
		}
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	out, handled, err := s.evalQScriptTerminalPipeline(descriptor, terminal)
	switch {
	case err != nil:
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
	case handled:
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "hit", "typed_pipeline")
	default:
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
	}
	return out, handled, err
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

func (s *EvalState) evalQScriptMatrixRowSumCountPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
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
		return nil, true, err
	}
	if !scalar || len(indexes) != 1 {
		return nil, false, nil
	}
	shape := qMatrixIndexShape(matrix, 1) + "/sum-count"
	out, handled, err := data.TryMatrixRowNumericSumCount(matrix, indexes[0])
	return qTypedRuntimeResultReason("MatrixRowSumCount", shape, RuntimeFallbackUnsupportedType, out, handled, err)
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
	sumValue, err := sum(left)
	if err != nil {
		return nil, true, err
	}
	out, err := applyDyadic('+', sumValue, right)
	return out, true, err
}

func (s *EvalState) evalQScriptSequenceEdgeSumPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
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

func (s *EvalState) evalQScriptTerminalPipeline(descriptor *qScriptPipelineDescriptor, terminal qPipelinePlan) (any, bool, error) {
	switch terminal.kind {
	case qPipelineApplyScalarIndex:
		return s.evalQPipelinePlan(terminal)
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
			if out, handled, err := s.evalQPipelineModuloCompareValueSum(*descriptor.moduloMaskPlan, array); err != nil || handled {
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
				if out, handled, err := s.evalQPipelineModuloCompareValueSum(*descriptor.moduloMaskPlan, array); err != nil || handled {
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
	if descriptor == nil || descriptor.kind != qScriptPipelineWhereIndexReduceSum {
		return false
	}
	if strings.TrimSpace(descriptor.indexExpr) != assignment.name {
		return false
	}
	return descriptor.moduloMaskPlan != nil
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
	if _, ok := s.lookupName(name); ok {
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
		s.env[s.resolveAssignmentName(assignment.name)] = value
		return nil
	}
	return nil
}
