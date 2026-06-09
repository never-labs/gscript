package q

import (
	"strconv"
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
	qScriptPipelineIntegerDivModReduce qScriptPipelineKind = "multi-reduce/integer-divmod-sum-count"
	qScriptPipelineMatrixRowSumCount   qScriptPipelineKind = "matrix-row-reduce/sum-count"
	qScriptPipelineMatrixCellPlusCount qScriptPipelineKind = "matrix-cell-reduce/cell-plus-count"
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
	includeCount      bool
	sequenceValueExpr string
	sequenceValuePlan qScriptBindingPlan
	sequenceSteps     []data.SequenceTransformStep
	sequenceBindings  []string
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
	if d.kind == qScriptPipelineSequenceEdgeSum && len(d.sequenceSteps) > 0 {
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
	descriptor.colIndexPlan = buildQScriptBindingPlanForRHS(descriptor.colIndexExpr, nil)
	descriptor.scalarPlan = buildQScriptBindingPlanForRHS(descriptor.scalarExpr, nil)
	descriptor.sequenceValuePlan = buildQScriptBindingPlanForRHS(descriptor.sequenceValueExpr, nil)
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
	if descriptor, ok := qScriptPipelineSumPlusDyadicFloatDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineIntegerDivModDescriptor(src, bindings); ok {
		return descriptor, true
	}
	if descriptor, ok := qScriptPipelineMatrixCellPlusCountDescriptor(src, bindings); ok {
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
		step := data.SequenceTransformStep{Transform: data.SequenceTransformSublist, ArgCount: len(args)}
		copy(step.Args[:], args)
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
	if descriptor.kind == qScriptPipelineSequenceEdgeSum {
		out, handled, err := s.evalQScriptSequenceEdgeSumPipeline(descriptor)
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
	if descriptor.kind == qScriptPipelineMatrixCellPlusCount {
		out, handled, err := s.evalQScriptMatrixCellPlusCountPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
		return out, handled, err
	}
	if descriptor.kind == qScriptPipelineCallableDotSumRight {
		out, handled, err := s.evalQScriptCallableDotSumPlusRightPipeline(descriptor)
		recordQScriptPipelineResult(shape, handled, err)
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

func (s *EvalState) evalQScriptReshapedMatrixRowSumCount(plan *qScriptBindingPlan, row int) (any, bool, error) {
	if plan == nil || plan.kind != qScriptBindingBinary || plan.op != "#" || plan.left == nil || plan.right == nil {
		return nil, false, nil
	}
	shapeValue, handled, err := s.evalQScriptBindingPlan(plan.left)
	if err != nil || !handled {
		return nil, handled, err
	}
	shape, err := qReshapeShape(shapeValue)
	if err != nil {
		return nil, true, err
	}
	if len(shape) != 2 {
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
	shape, err := qReshapeShape(shapeValue)
	if err != nil {
		return nil, true, err
	}
	if len(shape) != 2 {
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
		return 0, true, err
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
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	value, handled, err := s.evalQScriptBindingPlan(&descriptor.sequenceValuePlan)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	shape := "vector-transform-chain/sum-first-last/" + qScriptPipelineSequenceTransformName(descriptor.sequenceSteps) + "/" + string(qRuntimeKernelOperandKind(value, nil))
	out, handled, err := data.TryTypedSequenceTransformChainNumericSumFirstLast(descriptor.sequenceSteps, value)
	out, handled, err = qTypedRuntimeResultReason("SequenceTransformChainEdgeReduce", shape, RuntimeFallbackUnsupportedType, out, handled, err)
	return out, handled, err
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
