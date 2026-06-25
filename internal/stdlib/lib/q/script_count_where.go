package q

import "strings"

type qScriptCountWherePlan struct {
	source  string
	sources []string
	counts  []qScriptCountWhereTerm
}

type qScriptCountWhereTerm struct {
	name      string
	predicate *qScriptWherePredicatePlan
	length    int
}

func buildQScriptCountWherePlan(statements []qScriptStatement) *qScriptCountWherePlan {
	if len(statements) < 2 {
		return nil
	}
	numericBindings := make(map[string]qScriptNumericExprPlan, len(statements))
	countBindings := make(map[string]qScriptCountWhereTerm, len(statements))
	for i := 0; i < len(statements)-1; i++ {
		stmt := statements[i]
		if stmt.assign == "" || stmt.idxAssignName != "" || stmt.rhs == "" {
			return nil
		}
		if predicate, length, ok := qScriptCountWherePredicate(stmt.rhs, numericBindings); ok {
			countBindings[stmt.assign] = qScriptCountWhereTerm{name: stmt.assign, predicate: predicate, length: length}
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
	terms := qScriptCountWhereTerminalTerms(terminal, countBindings)
	if len(terms) == 0 {
		return nil
	}
	return &qScriptCountWherePlan{
		source:  terminal,
		sources: qScriptNumericSumStatementSources(statements),
		counts:  terms,
	}
}

func qScriptCountWherePredicate(src string, bindings map[string]qScriptNumericExprPlan) (*qScriptWherePredicatePlan, int, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "count where ") || !wordBoundary(src, 0, len("count")) {
		return nil, 0, false
	}
	predicateSrc := strings.TrimSpace(src[len("count where "):])
	predicate, ok := qScriptWhereIndexPredicatePlan(predicateSrc, bindings, nil)
	if !ok {
		return nil, 0, false
	}
	length := qScriptWherePredicateLength(predicate)
	if length <= 0 {
		return nil, 0, false
	}
	return predicate, length, true
}

func qScriptCountWhereTerminalTerms(src string, bindings map[string]qScriptCountWhereTerm) []qScriptCountWhereTerm {
	src = strings.TrimSpace(src)
	if term, ok := bindings[src]; ok && qScriptPipelineSimpleName(src) {
		return []qScriptCountWhereTerm{term}
	}
	parts := qScriptPipelinePlusTerms(src)
	if len(parts) < 2 {
		return nil
	}
	terms := make([]qScriptCountWhereTerm, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		term, ok := bindings[name]
		if !ok || !qScriptPipelineSimpleName(name) {
			return nil
		}
		terms = append(terms, term)
	}
	return terms
}

func (s *EvalState) evalQScriptCountWherePlan(plan *qScriptCountWherePlan) (any, bool, error) {
	if plan == nil || len(plan.counts) == 0 {
		return nil, false, nil
	}
	var total int64
	for _, term := range plan.counts {
		count, ok := qScriptCountWhereClosedCount(term.predicate, term.length)
		if !ok {
			return nil, false, nil
		}
		total += count
	}
	recordRuntimeKernelProbe("QScriptCountWherePlan", "count-where/periodic", true, nil)
	for _, source := range plan.sources {
		recordQEvalDispatch(source, EvalDispatchScriptCountWhere)
	}
	return total, true, nil
}

func qScriptCountWhereClosedCount(predicate *qScriptWherePredicatePlan, length int) (int64, bool) {
	if predicate == nil || length <= 0 {
		return 0, false
	}
	if periodLen := qScriptWherePredicatePeriodLen(predicate); periodLen > 0 && periodLen <= qScriptWhereIndexClosedFormMaxPeriod {
		return qScriptCountWherePeriodicCount(predicate, length, periodLen, qScriptWherePredicateAt), true
	}
	constraint := qScriptWhereRowConstraint{start: 0, end: length - 1}
	if !qScriptWherePredicateConstrainRows(predicate, &constraint) || constraint.start > constraint.end {
		return 0, false
	}
	periodLen := qScriptWherePredicateResidualPeriodLen(predicate)
	if periodLen == 0 && (constraint.changed || constraint.exclusionCount > 0) {
		periodLen = 1
	}
	if periodLen <= 0 || periodLen > qScriptWhereIndexClosedFormMaxPeriod {
		return 0, false
	}
	count := qScriptCountWherePeriodicRangeCount(predicate, constraint.start, constraint.end, periodLen, qScriptWherePredicateResidualAt)
	for i := 0; i < constraint.exclusionCount; i++ {
		excluded := constraint.exclusions[i]
		if excluded >= constraint.start && excluded <= constraint.end && qScriptWherePredicateResidualAt(predicate, excluded) {
			count--
		}
	}
	return count, true
}

func qScriptCountWherePeriodicCount(predicate *qScriptWherePredicatePlan, length, periodLen int, pred func(*qScriptWherePredicatePlan, int) bool) int64 {
	return qScriptCountWherePeriodicRangeCount(predicate, 0, length-1, periodLen, pred)
}

func qScriptCountWherePeriodicRangeCount(predicate *qScriptWherePredicatePlan, start, end, periodLen int, pred func(*qScriptWherePredicatePlan, int) bool) int64 {
	if periodLen <= 0 || start > end {
		return 0
	}
	var count int64
	for offset := 0; offset < periodLen && offset <= end; offset++ {
		first := offset
		if first < start {
			delta := start - first
			first += ((delta + periodLen - 1) / periodLen) * periodLen
		}
		if first > end || !pred(predicate, offset) {
			continue
		}
		count += int64((end-first)/periodLen + 1)
	}
	return count
}
