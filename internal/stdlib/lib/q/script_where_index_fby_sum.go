package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptWhereIndexFbySumPlan struct {
	skipNames        map[string]bool
	indexPlan        qScriptBindingPlan
	groupPlan        qScriptBindingPlan
	valueExpr        qScriptSelectedExpr
	indexValueExpr   *data.I64IndexExpr
	addendSrc        string
	addendExpr       Expr
	addendPlan       qScriptBindingPlan
	addendCountIndex bool
	statements       []qScriptStatement
}

type qScriptSelectedExpr struct {
	op         data.I64SelectedExprOp
	value      int64
	sourceName string
	left       *qScriptSelectedExpr
	right      *qScriptSelectedExpr
}

func buildQScriptWhereIndexFbySumPlan(statements []qScriptStatement) *qScriptWhereIndexFbySumPlan {
	if len(statements) < 5 {
		return nil
	}
	bindings := make(map[string]string, len(statements))
	for i := 0; i < len(statements)-1; i++ {
		stmt := statements[i]
		if stmt.assign == "" || stmt.rhs == "" {
			return nil
		}
		bindings[stmt.assign] = stmt.rhs
	}
	terminal := strings.TrimSpace(statements[len(statements)-1].src)
	sumName, addendSrc, ok := qScriptWhereIndexFbySumTerminal(terminal, bindings)
	if !ok || sumName == "" || addendSrc == "" {
		return nil
	}
	sumRHS := strings.TrimSpace(bindings[sumName])
	amendName, groupExpr, ok := qScriptWhereIndexFbySumAssignment(sumRHS)
	if !ok {
		return nil
	}
	amendRHS := strings.TrimSpace(bindings[amendName])
	indexName, valueName, ok := qScriptWhereIndexFbySumAmend(amendRHS)
	if !ok {
		return nil
	}
	indexRHS := strings.TrimSpace(bindings[indexName])
	if _, ok := directWhereMaskExpr(indexRHS); !ok {
		return nil
	}
	valueRHS := strings.TrimSpace(bindings[valueName])
	valuePlan, ok := qScriptSelectedExprPlan(valueRHS, indexName, bindings, nil)
	if !ok {
		return nil
	}
	var indexValueExpr *data.I64IndexExpr
	if expr, ok := qScriptPipelineI64IndexExprPlan(valueRHS, indexName, bindings, nil); ok {
		indexValueExpr = &expr
	}
	addendCountIndex := false
	if countName, ok := qScriptPipelineCountTermResolved(addendSrc, bindings, nil); ok {
		if countName != indexName {
			return nil
		}
		addendCountIndex = true
	} else if _, ok := qScriptNumericCountWhereNullTargetName(stripEnclosingParens(strings.TrimSpace(addendSrc))); !ok {
		return nil
	}
	skip := map[string]bool{
		indexName: true,
		valueName: true,
		amendName: true,
		sumName:   true,
	}
	return &qScriptWhereIndexFbySumPlan{
		skipNames:        skip,
		indexPlan:        buildQScriptBindingPlanForRHS(indexRHS, parseCachedValueExpr(indexRHS)),
		groupPlan:        buildQScriptBindingPlanForRHS(groupExpr, parseCachedValueExpr(groupExpr)),
		valueExpr:        valuePlan,
		indexValueExpr:   indexValueExpr,
		addendSrc:        addendSrc,
		addendExpr:       parseCachedValueExpr(addendSrc),
		addendPlan:       buildQScriptBindingPlanForRHS(addendSrc, parseCachedValueExpr(addendSrc)),
		addendCountIndex: addendCountIndex,
		statements:       statements,
	}
}

func qScriptWhereIndexFbySumTerminal(src string, bindings map[string]string) (sumName, addendSrc string, ok bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 2 {
		return "", "", false
	}
	for _, term := range terms {
		expr, sumOK := qScriptPipelineSumExprTerm(term)
		if sumOK && qScriptPipelineSimpleName(expr) {
			if sumName != "" {
				return "", "", false
			}
			sumName = expr
			continue
		}
		if addendSrc != "" {
			return "", "", false
		}
		addendSrc = stripEnclosingParens(strings.TrimSpace(term))
	}
	return sumName, addendSrc, sumName != "" && addendSrc != ""
}

func qScriptWhereIndexFbySumAssignment(src string) (valueName, groupExpr string, ok bool) {
	left, right, ok := splitTopLevelWord(strings.TrimSpace(src), "fby")
	if !ok {
		return "", "", false
	}
	valueName, ok = qScriptPipelineSumExprTerm(left)
	if !ok || !qScriptPipelineSimpleName(valueName) {
		return "", "", false
	}
	right = strings.TrimSpace(right)
	return valueName, right, right != ""
}

func qScriptWhereIndexFbySumAmend(src string) (indexName, valueName string, ok bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "@[") || !strings.HasSuffix(src, "]") {
		return "", "", false
	}
	parts := splitTopLevelDelim(src[len("@["):len(src)-1], ';')
	if len(parts) != 4 {
		return "", "", false
	}
	if !qScriptWhereIndexFbySumZeroSource(parts[0]) || strings.TrimSpace(parts[2]) != "+" {
		return "", "", false
	}
	indexName = strings.TrimSpace(parts[1])
	valueName = strings.TrimSpace(parts[3])
	if !qScriptPipelineSimpleName(indexName) || !qScriptPipelineSimpleName(valueName) {
		return "", "", false
	}
	return indexName, valueName, true
}

func qScriptWhereIndexFbySumZeroSource(src string) bool {
	left, right, ok := splitTopLevelOperator(stripEnclosingParens(strings.TrimSpace(src)), "#")
	if !ok || strings.TrimSpace(left) == "" {
		return false
	}
	n, ok := qScriptPipelineStaticInt(right)
	return ok && n == 0
}

func qScriptSelectedExprPlan(src, indexName string, bindings map[string]string, seen map[string]bool) (qScriptSelectedExpr, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	indexName = strings.TrimSpace(indexName)
	if src == "" || indexName == "" {
		return qScriptSelectedExpr{}, false
	}
	if src == indexName {
		return qScriptSelectedExpr{op: data.I64SelectedExprIndex}, true
	}
	if valueExpr, indexExpr, ok := findPostfixIndex(src); ok && strings.TrimSpace(indexExpr) == indexName {
		valueExpr = strings.TrimSpace(valueExpr)
		if qScriptPipelineSimpleName(valueExpr) {
			return qScriptSelectedExpr{op: data.I64SelectedExprGather, sourceName: valueExpr}, true
		}
	}
	if qScriptPipelineSimpleName(src) {
		bound := strings.TrimSpace(bindings[src])
		if bound == "" {
			return qScriptSelectedExpr{}, false
		}
		if seen == nil {
			seen = make(map[string]bool, len(bindings))
		}
		if seen[src] {
			return qScriptSelectedExpr{}, false
		}
		seen[src] = true
		return qScriptSelectedExprPlan(bound, indexName, bindings, seen)
	}
	if n, ok := qScriptPipelineStaticInt(src); ok {
		return qScriptSelectedExpr{op: data.I64SelectedExprConst, value: int64(n)}, true
	}
	if n, ok := qScriptPipelineRepeatedIndexConstant(src, indexName); ok {
		return qScriptSelectedExpr{op: data.I64SelectedExprConst, value: int64(n)}, true
	}
	if widthExpr, valueExpr, ok := splitTopLevelWord(src, "xbar"); ok {
		width, widthOK := qScriptPipelineStaticInt(widthExpr)
		if !widthOK || width <= 0 {
			return qScriptSelectedExpr{}, false
		}
		valuePlan, ok := qScriptSelectedExprPlan(valueExpr, indexName, bindings, cloneBoolMap(seen))
		if !ok {
			return qScriptSelectedExpr{}, false
		}
		return qScriptSelectedExpr{op: data.I64SelectedExprXbar, value: int64(width), left: &valuePlan}, true
	}
	for _, spec := range []struct {
		op   string
		kind data.I64SelectedExprOp
	}{
		{"+", data.I64SelectedExprAdd},
		{"-", data.I64SelectedExprSub},
	} {
		left, right, ok := splitTopLevelOperator(src, spec.op)
		if !ok || strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
			continue
		}
		leftPlan, rightPlan, ok := qScriptSelectedExprOperands(left, right, indexName, bindings, seen)
		if !ok {
			return qScriptSelectedExpr{}, false
		}
		return qScriptSelectedExpr{op: spec.kind, left: &leftPlan, right: &rightPlan}, true
	}
	for _, spec := range []struct {
		op   string
		kind data.I64SelectedExprOp
	}{
		{"*", data.I64SelectedExprMul},
		{"div", data.I64SelectedExprDiv},
		{"mod", data.I64SelectedExprMod},
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
		leftPlan, rightPlan, ok := qScriptSelectedExprOperands(left, right, indexName, bindings, seen)
		if !ok {
			return qScriptSelectedExpr{}, false
		}
		return qScriptSelectedExpr{op: spec.kind, left: &leftPlan, right: &rightPlan}, true
	}
	return qScriptSelectedExpr{}, false
}

func qScriptSelectedExprOperands(left, right, indexName string, bindings map[string]string, seen map[string]bool) (qScriptSelectedExpr, qScriptSelectedExpr, bool) {
	leftPlan, ok := qScriptSelectedExprPlan(left, indexName, bindings, cloneBoolMap(seen))
	if !ok {
		return qScriptSelectedExpr{}, qScriptSelectedExpr{}, false
	}
	rightPlan, ok := qScriptSelectedExprPlan(right, indexName, bindings, cloneBoolMap(seen))
	if !ok {
		return qScriptSelectedExpr{}, qScriptSelectedExpr{}, false
	}
	return leftPlan, rightPlan, true
}

func (s *EvalState) evalQScriptWhereIndexFbySumPlan(plan *qScriptWhereIndexFbySumPlan) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	for i := 0; i < len(plan.statements)-1; i++ {
		stmt := &plan.statements[i]
		if plan.skipNames[stmt.assign] {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&stmt.bindingPlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(stmt.rhs, stmt.valueExpr, &stmt.bindingPlan, &stmt.fastPlan)
			if err != nil {
				return nil, true, err
			}
		}
		s.ensureOwnedEnv()
		s.env[s.resolveAssignmentName(stmt.assign)] = value
	}
	indexValue, handled, err := s.evalQScriptBindingPlan(&plan.indexPlan)
	if err != nil || !handled {
		return indexValue, handled, err
	}
	indexes, ok := indexValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	groupValue, handled, err := s.evalQScriptBindingPlan(&plan.groupPlan)
	if err != nil || !handled {
		return groupValue, handled, err
	}
	groups, ok := groupValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	var out int64
	var kernelHandled bool
	if plan.indexValueExpr != nil {
		out, kernelHandled, err = data.TryTypedI64IndexExprFbySumTotal(indexes, *plan.indexValueExpr, groups)
	} else {
		valueExpr, ok, err := s.qScriptSelectedExprRuntime(plan.valueExpr)
		if err != nil || !ok {
			return nil, ok, err
		}
		out, kernelHandled, err = data.TryTypedI64SelectedExprFbySumTotal(indexes, valueExpr, groups)
	}
	out, kernelHandled, err = qTypedRuntimeResult("QScriptWhereIndexFbySumPlan", "where-index-fby-sum-total/i64", out, kernelHandled, err)
	if err != nil || !kernelHandled {
		return out, kernelHandled, err
	}
	addend, handled, err := s.evalQScriptWhereIndexFbySumAddend(plan, indexes)
	if err != nil || !handled {
		return addend, handled, err
	}
	if addendI64, ok := integerValue(addend); ok {
		return out + addendI64, true, nil
	}
	value, err := evalValueBinary("+", out, addend)
	return value, true, err
}

func (s *EvalState) qScriptSelectedExprRuntime(expr qScriptSelectedExpr) (data.I64SelectedExpr, bool, error) {
	switch expr.op {
	case data.I64SelectedExprConst:
		return data.I64SelectedExpr{Op: expr.op, Value: expr.value}, true, nil
	case data.I64SelectedExprIndex:
		return data.I64SelectedExpr{Op: expr.op}, true, nil
	case data.I64SelectedExprGather:
		value, ok := s.env[s.resolveAssignmentName(expr.sourceName)]
		if !ok {
			return data.I64SelectedExpr{}, false, nil
		}
		array, ok := value.(data.Array)
		if !ok {
			return data.I64SelectedExpr{}, false, nil
		}
		return data.I64SelectedExpr{Op: expr.op, Source: array}, true, nil
	case data.I64SelectedExprAdd, data.I64SelectedExprSub, data.I64SelectedExprMul, data.I64SelectedExprDiv, data.I64SelectedExprMod:
		left, ok, err := s.qScriptSelectedExprRuntime(*expr.left)
		if err != nil || !ok {
			return data.I64SelectedExpr{}, ok, err
		}
		right, ok, err := s.qScriptSelectedExprRuntime(*expr.right)
		if err != nil || !ok {
			return data.I64SelectedExpr{}, ok, err
		}
		return data.I64SelectedExpr{Op: expr.op, Left: &left, Right: &right}, true, nil
	case data.I64SelectedExprXbar:
		left, ok, err := s.qScriptSelectedExprRuntime(*expr.left)
		if err != nil || !ok {
			return data.I64SelectedExpr{}, ok, err
		}
		return data.I64SelectedExpr{Op: expr.op, Value: expr.value, Left: &left}, true, nil
	default:
		return data.I64SelectedExpr{}, false, nil
	}
}

func (s *EvalState) evalQScriptWhereIndexFbySumAddend(plan *qScriptWhereIndexFbySumPlan, indexes data.Array) (any, bool, error) {
	if plan.addendCountIndex {
		return int64(indexes.Len()), true, nil
	}
	value, handled, err := s.evalQScriptBindingPlan(&plan.addendPlan)
	if err != nil {
		return nil, true, err
	}
	if handled {
		return value, true, nil
	}
	value, err = s.evalCachedOrString(plan.addendSrc, plan.addendExpr, &plan.addendPlan, nil)
	if err != nil {
		return nil, true, err
	}
	return value, true, nil
}
