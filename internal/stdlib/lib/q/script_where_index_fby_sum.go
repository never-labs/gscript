package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptWhereIndexFbySumPlan struct {
	skipNames  map[string]bool
	indexPlan  qScriptBindingPlan
	groupPlan  qScriptBindingPlan
	valueExpr  data.I64IndexExpr
	statements []qScriptStatement
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
	sumName, countName, ok := qScriptWhereIndexFbySumTerminal(terminal, bindings)
	if !ok || sumName == "" || countName == "" {
		return nil
	}
	sumRHS := strings.TrimSpace(bindings[sumName])
	amendName, groupExpr, ok := qScriptWhereIndexFbySumAssignment(sumRHS)
	if !ok {
		return nil
	}
	amendRHS := strings.TrimSpace(bindings[amendName])
	indexName, valueName, ok := qScriptWhereIndexFbySumAmend(amendRHS)
	if !ok || indexName != countName {
		return nil
	}
	indexRHS := strings.TrimSpace(bindings[indexName])
	if _, ok := directWhereMaskExpr(indexRHS); !ok {
		return nil
	}
	valueRHS := strings.TrimSpace(bindings[valueName])
	valuePlan, ok := qScriptPipelineI64IndexExprPlan(valueRHS, indexName, bindings, nil)
	if !ok {
		return nil
	}
	skip := map[string]bool{
		indexName: true,
		valueName: true,
		amendName: true,
		sumName:   true,
	}
	return &qScriptWhereIndexFbySumPlan{
		skipNames:  skip,
		indexPlan:  buildQScriptBindingPlanForRHS(indexRHS, parseCachedValueExpr(indexRHS)),
		groupPlan:  buildQScriptBindingPlanForRHS(groupExpr, parseCachedValueExpr(groupExpr)),
		valueExpr:  valuePlan,
		statements: statements,
	}
}

func qScriptWhereIndexFbySumTerminal(src string, bindings map[string]string) (sumName, countName string, ok bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 2 {
		return "", "", false
	}
	for _, term := range terms {
		if expr, countOK := qScriptPipelineCountTermResolved(term, bindings, nil); countOK {
			if countName != "" {
				return "", "", false
			}
			countName = expr
			continue
		}
		expr, sumOK := qScriptPipelineSumExprTerm(term)
		if !sumOK || !qScriptPipelineSimpleName(expr) || sumName != "" {
			return "", "", false
		}
		sumName = expr
	}
	return sumName, countName, sumName != "" && countName != ""
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
	out, handled, err := data.TryTypedI64IndexExprFbySumTotal(indexes, plan.valueExpr, groups)
	out, handled, err = qTypedRuntimeResult("QScriptWhereIndexFbySumPlan", "where-index-fby-sum-total/i64", out, handled, err)
	if err != nil || !handled {
		return out, handled, err
	}
	return out + int64(indexes.Len()), true, nil
}
