package q

import "strings"

type qScriptWhereIndexOnlySumPlan struct {
	source  string
	sources []string
	terms   []qScriptWhereIndexOnlySumTerm
}

type qScriptWhereIndexOnlySumTerm struct {
	name      string
	predicate *qScriptWherePredicatePlan
	length    int
}

func buildQScriptWhereIndexOnlySumPlan(statements []qScriptStatement) *qScriptWhereIndexOnlySumPlan {
	if len(statements) < 2 {
		return nil
	}
	numericBindings := make(map[string]qScriptNumericExprPlan, len(statements))
	indexBindings := make(map[string]qScriptWhereIndexOnlySumTerm, len(statements))
	for i := 0; i < len(statements)-1; i++ {
		stmt := statements[i]
		if stmt.assign == "" || stmt.idxAssignName != "" || stmt.rhs == "" {
			return nil
		}
		if predicate, length, ok := qScriptWhereIndexCountPredicate(stmt.rhs, numericBindings); ok {
			indexBindings[stmt.assign] = qScriptWhereIndexOnlySumTerm{name: stmt.assign, predicate: predicate, length: length}
			continue
		}
		expr := compileQEvalExpr(stmt.rhs, 0)
		if expr == nil {
			return nil
		}
		plan, ok := buildQScriptNumericExprPlan(expr, numericBindings)
		if !ok {
			return nil
		}
		numericBindings[stmt.assign] = plan
	}
	terminal := strings.TrimSpace(statements[len(statements)-1].src)
	terms := qScriptWhereIndexOnlySumTerminalTerms(terminal, indexBindings)
	if len(terms) == 0 {
		return nil
	}
	return &qScriptWhereIndexOnlySumPlan{
		source:  terminal,
		sources: qScriptNumericSumStatementSources(statements),
		terms:   terms,
	}
}

func qScriptWhereIndexOnlySumTerminalTerms(src string, bindings map[string]qScriptWhereIndexOnlySumTerm) []qScriptWhereIndexOnlySumTerm {
	parts := qScriptPipelinePlusTerms(src)
	if len(parts) == 0 {
		return nil
	}
	terms := make([]qScriptWhereIndexOnlySumTerm, 0, len(parts))
	for _, part := range parts {
		kind, name, ok := qScriptPipelineSumCountReducerTerm(part)
		if !ok || kind != "sum" {
			return nil
		}
		term, ok := bindings[name]
		if !ok || !qScriptPipelineSimpleName(name) {
			return nil
		}
		terms = append(terms, term)
	}
	return terms
}

func (s *EvalState) evalQScriptWhereIndexOnlySumPlan(plan *qScriptWhereIndexOnlySumPlan) (any, bool, error) {
	sum, ok := qScriptWhereIndexOnlySum(plan)
	if !ok {
		return nil, false, nil
	}
	recordRuntimeKernelProbe("QScriptWhereIndexOnlySumPlan", "where-index-reduce/sum", true, nil)
	for _, source := range plan.sources {
		recordQEvalDispatch(source, EvalDispatchScriptNumericSum)
	}
	return sum, true, nil
}

func (s *EvalState) evalQScriptWhereIndexOnlySumPlanScalar(plan *qScriptWhereIndexOnlySumPlan) (EvalScalarResult, bool, error) {
	sum, ok := qScriptWhereIndexOnlySum(plan)
	if !ok {
		return EvalScalarResult{}, false, nil
	}
	recordRuntimeKernelProbe("QScriptWhereIndexOnlySumPlan", "where-index-reduce/sum", true, nil)
	for _, source := range plan.sources {
		recordQEvalDispatch(source, EvalDispatchScriptNumericSum)
	}
	return evalScalarInt(sum), true, nil
}

func qScriptWhereIndexOnlySum(plan *qScriptWhereIndexOnlySumPlan) (int64, bool) {
	if plan == nil || len(plan.terms) == 0 {
		return 0, false
	}
	var total int64
	for _, term := range plan.terms {
		sum, ok := qScriptWhereIndexClosedSum(term.predicate, term.length)
		if !ok {
			return 0, false
		}
		total += sum
	}
	return total, true
}

func qScriptWhereIndexClosedSum(predicate *qScriptWherePredicatePlan, length int) (int64, bool) {
	if predicate == nil || length <= 0 {
		return 0, false
	}
	if periodLen := qScriptWherePredicatePeriodLen(predicate); periodLen > 0 && periodLen <= qScriptWhereIndexClosedFormMaxPeriod {
		return qScriptWhereIndexPeriodicRangeSum(predicate, 0, length-1, periodLen, qScriptWherePredicateAt), true
	}
	constraint := qScriptWhereRowConstraint{start: 0, end: length - 1}
	if !qScriptWherePredicateConstrainRows(predicate, &constraint) {
		return 0, false
	}
	if constraint.start > constraint.end {
		return 0, true
	}
	periodLen := qScriptWherePredicateResidualPeriodLen(predicate)
	if periodLen == 0 && constraint.exclusionCount == 0 {
		return qArithmeticSeriesSumI64(int64(constraint.start), 1, int64(constraint.end-constraint.start+1)), true
	}
	if periodLen == 0 && (constraint.changed || constraint.exclusionCount > 0) {
		periodLen = 1
	}
	if periodLen <= 0 || periodLen > qScriptWhereIndexClosedFormMaxPeriod {
		return 0, false
	}
	sum := qScriptWhereIndexPeriodicRangeSum(predicate, constraint.start, constraint.end, periodLen, qScriptWherePredicateResidualAt)
	for i := 0; i < constraint.exclusionCount; i++ {
		excluded := constraint.exclusions[i]
		if excluded >= constraint.start && excluded <= constraint.end && qScriptWherePredicateResidualAt(predicate, excluded) {
			sum -= int64(excluded)
		}
	}
	return sum, true
}

func qScriptWhereIndexPeriodicRangeSum(predicate *qScriptWherePredicatePlan, start, end, periodLen int, pred func(*qScriptWherePredicatePlan, int) bool) int64 {
	if periodLen <= 0 || start > end {
		return 0
	}
	var sum int64
	for offset := 0; offset < periodLen && offset <= end; offset++ {
		first := offset
		if first < start {
			delta := start - first
			first += ((delta + periodLen - 1) / periodLen) * periodLen
		}
		if first > end || !pred(predicate, offset) {
			continue
		}
		rows := int64((end-first)/periodLen + 1)
		sum += qArithmeticSeriesSumI64(int64(first), int64(periodLen), rows)
	}
	return sum
}
