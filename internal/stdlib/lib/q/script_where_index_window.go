package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptWhereIndexWindowPlan struct {
	skipNames  map[string]bool
	indexPlan  qScriptBindingPlan
	valueExpr  qScriptSelectedExpr
	indexExpr  *data.I64IndexExpr
	length     int
	width      int
	statements []qScriptStatement
}

func buildQScriptWhereIndexWindowPlan(statements []qScriptStatement) *qScriptWhereIndexWindowPlan {
	if len(statements) < 4 {
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
	valueName, countName, width, ok := qScriptWhereIndexWindowTerminal(terminal, bindings)
	if !ok {
		return nil
	}
	amendRHS := strings.TrimSpace(bindings[valueName])
	length, indexName, selectedValueName, ok := qScriptWhereIndexWindowAmend(amendRHS)
	if !ok || countName != indexName {
		return nil
	}
	indexRHS := strings.TrimSpace(bindings[indexName])
	if _, ok := directWhereMaskExpr(indexRHS); !ok {
		return nil
	}
	valueRHS := strings.TrimSpace(bindings[selectedValueName])
	valuePlan, ok := qScriptSelectedExprPlan(valueRHS, indexName, bindings, nil)
	if !ok {
		return nil
	}
	var indexExpr *data.I64IndexExpr
	if expr, ok := qScriptPipelineI64IndexExprPlan(valueRHS, indexName, bindings, nil); ok {
		indexExpr = &expr
	}
	return &qScriptWhereIndexWindowPlan{
		skipNames: map[string]bool{
			indexName:         true,
			selectedValueName: true,
			valueName:         true,
		},
		indexPlan:  buildQScriptBindingPlanForRHS(indexRHS, parseCachedValueExpr(indexRHS)),
		valueExpr:  valuePlan,
		indexExpr:  indexExpr,
		length:     length,
		width:      width,
		statements: statements,
	}
}

func qScriptWhereIndexWindowTerminal(src string, bindings map[string]string) (valueName, countName string, width int, ok bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 3 {
		return "", "", 0, false
	}
	seenMSum := false
	seenMAvg := false
	for _, term := range terms {
		if expr, countOK := qScriptPipelineCountTermResolved(term, bindings, nil); countOK {
			if countName != "" {
				return "", "", 0, false
			}
			countName = expr
			continue
		}
		word, termWidth, termValue, termOK := qScriptWhereIndexWindowTerm(term)
		if !termOK {
			return "", "", 0, false
		}
		if valueName != "" && valueName != termValue {
			return "", "", 0, false
		}
		if width != 0 && width != termWidth {
			return "", "", 0, false
		}
		valueName = termValue
		width = termWidth
		switch word {
		case "msum":
			if seenMSum {
				return "", "", 0, false
			}
			seenMSum = true
		case "mavg":
			if seenMAvg {
				return "", "", 0, false
			}
			seenMAvg = true
		default:
			return "", "", 0, false
		}
	}
	return valueName, countName, width, valueName != "" && countName != "" && seenMSum && seenMAvg && width > 0
}

func qScriptWhereIndexWindowTerm(src string) (word string, width int, valueName string, ok bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if !strings.HasPrefix(src, "+/") {
		return "", 0, "", false
	}
	body := strings.TrimSpace(src[len("+/"):])
	for _, candidate := range []string{"msum", "mavg"} {
		left, right, found := splitTopLevelWord(body, candidate)
		if !found {
			continue
		}
		n, widthOK := qScriptPipelineStaticInt(left)
		right = strings.TrimSpace(right)
		if !widthOK || n <= 0 || !qScriptPipelineSimpleName(right) {
			return "", 0, "", false
		}
		return candidate, n, right, true
	}
	return "", 0, "", false
}

func qScriptWhereIndexWindowAmend(src string) (length int, indexName, valueName string, ok bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "@[") || !strings.HasSuffix(src, "]") {
		return 0, "", "", false
	}
	parts := splitTopLevelDelim(src[len("@["):len(src)-1], ';')
	if len(parts) != 4 || strings.TrimSpace(parts[2]) != "+" {
		return 0, "", "", false
	}
	left, right, ok := splitTopLevelOperator(stripEnclosingParens(strings.TrimSpace(parts[0])), "#")
	if !ok {
		return 0, "", "", false
	}
	length, ok = qScriptPipelineStaticInt(left)
	if !ok || length < 0 {
		return 0, "", "", false
	}
	zero, ok := qScriptPipelineStaticInt(right)
	if !ok || zero != 0 {
		return 0, "", "", false
	}
	indexName = strings.TrimSpace(parts[1])
	valueName = strings.TrimSpace(parts[3])
	if !qScriptPipelineSimpleName(indexName) || !qScriptPipelineSimpleName(valueName) {
		return 0, "", "", false
	}
	return length, indexName, valueName, true
}

func (s *EvalState) evalQScriptWhereIndexWindowPlan(plan *qScriptWhereIndexWindowPlan) (any, bool, error) {
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
	var msum int64
	var mavg float64
	if plan.indexExpr != nil {
		msum, mavg, handled, err = data.TryTypedI64IndexExprSparseZeroMovingSumAvg(indexes, *plan.indexExpr, plan.length, plan.width)
	} else {
		valueExpr, ok, err := s.qScriptSelectedExprRuntime(plan.valueExpr)
		if err != nil || !ok {
			return nil, ok, err
		}
		msum, mavg, handled, err = data.TryTypedI64SelectedExprSparseZeroMovingSumAvg(indexes, valueExpr, plan.length, plan.width)
	}
	msum, handled, err = qTypedRuntimeResult("QScriptWhereIndexWindowPlan", "where-index-window-sum-avg/i64", msum, handled, err)
	if err != nil || !handled {
		return msum, handled, err
	}
	out, err := evalValueBinary("+", msum, mavg)
	if err != nil {
		return nil, true, err
	}
	out, err = evalValueBinary("+", out, int64(indexes.Len()))
	return out, true, err
}
