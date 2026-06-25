package q

import (
	"math"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

const qScriptWhereIndexClosedFormMaxPeriod = 512

type qScriptWhereIndexSumPlan struct {
	source        string
	sources       []string
	value         qScriptNumericSumSummary
	predicatePlan *qScriptWherePredicatePlan
	residues      []int
	residuePeriod int
	predicate     qScriptNumericSumSummary
	op            string
	scalar        qScriptNumericSumSummary
	withinLo      qScriptNumericSumSummary
	withinHi      qScriptNumericSumSummary
	countNull     *qScriptNumericSumSummary
}

type qScriptWherePredicateKind uint8

const (
	qScriptWherePredicateInvalid qScriptWherePredicateKind = iota
	qScriptWherePredicateAnd
	qScriptWherePredicateNot
	qScriptWherePredicateCompare
	qScriptWherePredicateWithin
	qScriptWherePredicateIn
)

type qScriptWherePredicatePlan struct {
	kind      qScriptWherePredicateKind
	left      *qScriptWherePredicatePlan
	right     *qScriptWherePredicatePlan
	value     qScriptNumericSumSummary
	symValue  qScriptSymbolSummary
	op        string
	scalar    qScriptNumericSumSummary
	lo        qScriptNumericSumSummary
	hi        qScriptNumericSumSummary
	values    []qScriptNumericSumSummary
	symValues []data.Symbol
}

type qScriptSymbolSummary struct {
	length int
	period []data.Symbol
}

type qScriptWhereRowConstraint struct {
	start          int
	end            int
	changed        bool
	exclusions     [8]int
	exclusionCount int
}

func buildQScriptWhereIndexSumPlan(statements []qScriptStatement) *qScriptWhereIndexSumPlan {
	if len(statements) < 3 {
		return nil
	}
	bindings := make(map[string]qScriptNumericExprPlan, len(statements))
	symbolBindings := make(map[string]qScriptSymbolSummary, len(statements))
	stringBindings := make(map[string]string, len(statements))
	for i := 0; i < len(statements)-1; i++ {
		stmt := statements[i]
		if stmt.assign == "" || stmt.idxAssignName != "" || stmt.rhs == "" {
			return nil
		}
		stringBindings[stmt.assign] = stmt.rhs
		expr := compileQEvalExpr(stmt.rhs, 0)
		if expr == nil {
			continue
		}
		if plan, ok := buildQScriptNumericExprPlan(expr, bindings); ok {
			bindings[stmt.assign] = plan
			continue
		}
		if summary, ok := qScriptSymbolExprSummary(stmt.rhs, symbolBindings); ok {
			symbolBindings[stmt.assign] = summary
		}
	}
	terminal := strings.TrimSpace(statements[len(statements)-1].src)
	valueName, indexName, countIndexName, countNullName, ok := qScriptWhereIndexSumTerminal(terminal, stringBindings)
	if !ok || valueName == "" || indexName == "" {
		return nil
	}
	if countIndexName != "" && countIndexName != indexName {
		return nil
	}
	indexBinding := strings.TrimSpace(stringBindings[indexName])
	maskExpr, ok := directWhereMaskExpr(indexBinding)
	if !ok {
		return nil
	}
	valuePlan, ok := bindings[valueName]
	if !ok {
		return nil
	}
	valueSummary, ok, err := qScriptNumericSummarize(valuePlan)
	if !ok || err != nil || valueSummary.length <= 0 {
		return nil
	}
	predicatePlan, ok := qScriptWhereIndexPredicatePlan(maskExpr, bindings, symbolBindings)
	if !ok || qScriptWherePredicateLength(predicatePlan) != valueSummary.length {
		return nil
	}
	var countNull *qScriptNumericSumSummary
	if countNullName != "" {
		countNullPlan, ok := bindings[countNullName]
		if !ok {
			return nil
		}
		nullSummary, ok, err := qScriptNumericSummarize(countNullPlan)
		if !ok || err != nil || nullSummary.length != valueSummary.length {
			return nil
		}
		countNull = &nullSummary
	}
	plan := &qScriptWhereIndexSumPlan{
		source:        terminal,
		sources:       qScriptNumericSumStatementSources(statements),
		value:         valueSummary,
		predicatePlan: predicatePlan,
		countNull:     countNull,
	}
	plan.buildResidues()
	return plan
}

func qScriptWhereIndexSumTerminal(src string, bindings map[string]string) (valueName, indexName, countIndexName, countNullName string, ok bool) {
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) != 2 {
		return "", "", "", "", false
	}
	for _, term := range terms {
		if expr, countOK := qScriptPipelineCountTermResolved(term, bindings, nil); countOK {
			if countIndexName != "" || countNullName != "" {
				return "", "", "", "", false
			}
			countIndexName = expr
			continue
		}
		if name, nullOK := qScriptNumericCountWhereNullTargetName(stripEnclosingParens(strings.TrimSpace(term))); nullOK {
			if countIndexName != "" || countNullName != "" {
				return "", "", "", "", false
			}
			countNullName = name
			continue
		}
		v, idx, sumOK := qScriptPipelineGatherSumTermResolved(term, bindings, nil)
		if !sumOK {
			v, idx, sumOK = qScriptWhereIndexAliasGatherSumTerm(term, bindings)
		}
		if !sumOK || valueName != "" {
			return "", "", "", "", false
		}
		if !qScriptPipelineSimpleName(v) || !qScriptPipelineSimpleName(idx) {
			return "", "", "", "", false
		}
		valueName, indexName = v, idx
	}
	return valueName, indexName, countIndexName, countNullName, valueName != "" && indexName != "" && (countIndexName != "" || countNullName != "")
}

func qScriptWhereIndexAliasGatherSumTerm(src string, bindings map[string]string) (string, string, bool) {
	name := qScriptPipelineAliasBodyName(src)
	if name == "" {
		return "", "", false
	}
	bound, ok := qScriptPipelineResolveSimpleBinding(name, bindings, nil)
	if !ok {
		return "", "", false
	}
	valueExpr, indexExpr, ok := findPostfixIndex(bound)
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(valueExpr), strings.TrimSpace(indexExpr), valueExpr != "" && indexExpr != ""
}

func qScriptWhereIndexPredicateSummary(maskExpr string, bindings map[string]qScriptNumericExprPlan) (predicate qScriptNumericSumSummary, op string, scalar, lo, hi qScriptNumericSumSummary, ok bool) {
	if predicate, lo, hi, ok := qScriptWhereIndexWithinSummary(maskExpr, bindings); ok {
		return predicate, "within", qScriptNumericSumSummary{}, lo, hi, true
	}
	for _, cmp := range []string{"<>", "<=", ">=", "=", "<", ">"} {
		if left, right, ok := splitTopLevelOperator(maskExpr, cmp); ok {
			if predicate, scalar, op, ok := qScriptWhereIndexCompareSummary(left, right, cmp, bindings); ok {
				return predicate, op, scalar, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, true
			}
		}
	}
	if left, right, cmp, isCompare := splitWhereCompareExpr(maskExpr); isCompare {
		if cmp == "within" {
			if predicate, lo, hi, ok := qScriptWhereIndexWithinSummary(left+" within "+right, bindings); ok {
				return predicate, "within", qScriptNumericSumSummary{}, lo, hi, true
			}
			return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
		}
		if predicate, scalar, op, ok := qScriptWhereIndexCompareSummary(left, right, cmp, bindings); ok {
			return predicate, op, scalar, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, true
		}
	}
	if left, right, isWithin := splitTopLevelWord(maskExpr, "within"); isWithin {
		if predicate, lo, hi, ok := qScriptWhereIndexWithinSummary(left+" within "+right, bindings); ok {
			return predicate, "within", qScriptNumericSumSummary{}, lo, hi, true
		}
	}
	return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
}

func qScriptWhereIndexCompareSummary(left, right, cmp string, bindings map[string]qScriptNumericExprPlan) (predicate, scalar qScriptNumericSumSummary, op string, ok bool) {
	leftSummary, leftOK := qScriptWhereIndexExprSummary(left, bindings)
	rightSummary, rightOK := qScriptWhereIndexExprSummary(right, bindings)
	if !leftOK || !rightOK {
		return qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, "", false
	}
	if leftSummary.scalar {
		flipped, flipOK := qScriptWhereIndexFlipCompare(cmp)
		if !flipOK {
			return qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, "", false
		}
		return rightSummary, leftSummary, flipped, true
	}
	if !rightSummary.scalar {
		return qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, "", false
	}
	return leftSummary, rightSummary, cmp, true
}

func qScriptWhereIndexWithinSummary(maskExpr string, bindings map[string]qScriptNumericExprPlan) (predicate, lo, hi qScriptNumericSumSummary, ok bool) {
	left, right, isWithin := splitTopLevelWord(maskExpr, "within")
	if !isWithin {
		return qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
	}
	predicate, predOK := qScriptWhereIndexExprSummary(left, bindings)
	if !predOK {
		return qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
	}
	bounds := strings.Fields(stripEnclosingParens(strings.TrimSpace(right)))
	if len(bounds) != 2 {
		return qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
	}
	lo, loOK := qScriptWhereIndexExprSummary(bounds[0], bindings)
	hi, hiOK := qScriptWhereIndexExprSummary(bounds[1], bindings)
	if !loOK || !hiOK || !lo.scalar || !hi.scalar {
		return qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
	}
	return predicate, lo, hi, true
}

func qScriptWhereIndexPredicatePlan(maskExpr string, bindings map[string]qScriptNumericExprPlan, symbolBindings map[string]qScriptSymbolSummary) (*qScriptWherePredicatePlan, bool) {
	maskExpr = stripEnclosingParens(strings.TrimSpace(maskExpr))
	if maskExpr == "" {
		return nil, false
	}
	if left, right, ok := splitTopLevelWord(maskExpr, "and"); ok {
		leftPlan, leftOK := qScriptWhereIndexPredicatePlan(left, bindings, symbolBindings)
		rightPlan, rightOK := qScriptWhereIndexPredicatePlan(right, bindings, symbolBindings)
		if !leftOK || !rightOK {
			return nil, false
		}
		return &qScriptWherePredicatePlan{kind: qScriptWherePredicateAnd, left: leftPlan, right: rightPlan}, true
	}
	if strings.HasPrefix(maskExpr, "not ") && wordBoundary(maskExpr, 0, len("not")) {
		child, ok := qScriptWhereIndexPredicatePlan(strings.TrimSpace(maskExpr[len("not "):]), bindings, symbolBindings)
		if !ok {
			return nil, false
		}
		return &qScriptWherePredicatePlan{kind: qScriptWherePredicateNot, left: child}, true
	}
	if left, right, ok := splitTopLevelWord(maskExpr, "in"); ok {
		value, valueOK := qScriptWhereIndexExprSummary(left, bindings)
		if valueOK {
			parts := strings.Fields(stripEnclosingParens(strings.TrimSpace(right)))
			if len(parts) == 0 {
				return nil, false
			}
			values := make([]qScriptNumericSumSummary, 0, len(parts))
			for _, part := range parts {
				summary, ok := qScriptWhereIndexExprSummary(part, bindings)
				if !ok || !summary.scalar {
					return nil, false
				}
				values = append(values, summary)
			}
			return &qScriptWherePredicatePlan{kind: qScriptWherePredicateIn, value: value, values: values}, true
		}
		symValue, symOK := qScriptSymbolExprSummary(left, symbolBindings)
		symValues, valuesOK := qScriptSymbolSetSummary(right)
		if !symOK || !valuesOK {
			return nil, false
		}
		return &qScriptWherePredicatePlan{kind: qScriptWherePredicateIn, symValue: symValue, symValues: symValues}, true
	}
	if predicate, op, scalar, lo, hi, ok := qScriptWhereIndexPredicateSummary(maskExpr, bindings); ok {
		switch op {
		case "within":
			return &qScriptWherePredicatePlan{kind: qScriptWherePredicateWithin, value: predicate, lo: lo, hi: hi}, true
		default:
			return &qScriptWherePredicatePlan{kind: qScriptWherePredicateCompare, value: predicate, op: op, scalar: scalar}, true
		}
	}
	return nil, false
}

func qScriptWhereIndexExprSummary(src string, bindings map[string]qScriptNumericExprPlan) (qScriptNumericSumSummary, bool) {
	expr := compileQEvalExpr(strings.TrimSpace(src), 0)
	if expr == nil {
		return qScriptNumericSumSummary{}, false
	}
	plan, ok := buildQScriptNumericExprPlan(expr, bindings)
	if !ok {
		return qScriptNumericSumSummary{}, false
	}
	summary, ok, err := qScriptNumericSummarize(plan)
	return summary, ok && err == nil
}

func qScriptSymbolExprSummary(src string, bindings map[string]qScriptSymbolSummary) (qScriptSymbolSummary, bool) {
	src = strings.TrimSpace(src)
	if summary, ok := bindings[src]; ok && qScriptPipelineSimpleName(src) {
		return summary, true
	}
	if left, right, ok := splitTopLevelOperator(src, "#"); ok {
		countExpr := compileQEvalExpr(strings.TrimSpace(left), 0)
		if countExpr == nil {
			return qScriptSymbolSummary{}, false
		}
		count, ok := qScriptNumericLiteralI64(countExpr)
		if !ok || count < 0 || count > int64(math.MaxInt) {
			return qScriptSymbolSummary{}, false
		}
		symbols, ok := qScriptSymbolSetSummary(right)
		if !ok || len(symbols) == 0 {
			return qScriptSymbolSummary{}, false
		}
		return qScriptSymbolSummary{length: int(count), period: symbols}, true
	}
	symbols, ok := qScriptSymbolSetSummary(src)
	if !ok || len(symbols) == 0 {
		return qScriptSymbolSummary{}, false
	}
	return qScriptSymbolSummary{length: len(symbols), period: symbols}, true
}

func qScriptSymbolSetSummary(src string) ([]data.Symbol, bool) {
	symbols, err := parseSymbolList(stripEnclosingParens(strings.TrimSpace(src)))
	if err != nil || len(symbols) == 0 || len(symbols) > qScriptWhereIndexClosedFormMaxPeriod {
		return nil, false
	}
	return symbols, true
}

func (s qScriptSymbolSummary) At(row int) data.Symbol {
	if len(s.period) == 0 {
		return ""
	}
	return s.period[row%len(s.period)]
}

func qScriptWhereIndexFlipCompare(op string) (string, bool) {
	switch op {
	case ">":
		return "<", true
	case ">=":
		return "<=", true
	case "<":
		return ">", true
	case "<=":
		return ">=", true
	case "=":
		return "=", true
	case "<>":
		return "<>", true
	default:
		return "", false
	}
}

func (s *EvalState) evalQScriptWhereIndexSumPlan(plan *qScriptWhereIndexSumPlan) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	sum, count, isFloat, ok := qScriptWhereIndexSum(plan)
	if !ok {
		return nil, false, nil
	}
	recordRuntimeKernelProbe("QScriptWhereIndexSumPlan", "where-index-reduce/periodic-sum-count", true, nil)
	for _, source := range plan.sources {
		recordQEvalDispatch(source, EvalDispatchScriptNumericSum)
	}
	if plan.countNull != nil {
		count = int64(plan.countNull.NullCount())
	}
	if isFloat {
		return sum + float64(count), true, nil
	}
	return int64(sum) + count, true, nil
}

func qScriptWhereIndexSum(plan *qScriptWhereIndexSumPlan) (float64, int64, bool, bool) {
	length := plan.value.length
	periodLen, ok := qScriptWhereIndexPeriodLen(plan)
	if !ok || periodLen <= 0 || periodLen > qScriptWhereIndexClosedFormMaxPeriod {
		return qScriptWhereIndexConstrainedPeriodicSum(plan)
	}
	if len(plan.residues) > 0 && plan.residuePeriod == periodLen {
		return qScriptWhereIndexResidueSum(plan, 0, length-1, periodLen, plan.residues)
	}
	var sum float64
	var count int64
	isFloat := plan.value.isFloat
	for offset := 0; offset < periodLen && offset < length; offset++ {
		if !qScriptWhereIndexPredicateAt(plan, offset) {
			continue
		}
		rows := (length-1-offset)/periodLen + 1
		if rows <= 0 {
			continue
		}
		count += int64(rows)
		if part, partFloat, ok := qScriptWhereIndexValueResidueSum(plan.value, offset, periodLen, rows); ok {
			sum += part
			isFloat = isFloat || partFloat
		}
	}
	return sum, count, isFloat, true
}

func (plan *qScriptWhereIndexSumPlan) buildResidues() {
	if plan == nil {
		return
	}
	periodLen, ok := qScriptWhereIndexPeriodLen(plan)
	if !ok || periodLen <= 0 || periodLen > qScriptWhereIndexClosedFormMaxPeriod {
		var constraint qScriptWhereRowConstraint
		constraint, periodLen, ok = qScriptWhereIndexConstrainedResidueSpec(plan)
		if !ok || constraint.start > constraint.end {
			return
		}
	}
	residues := make([]int, 0, periodLen)
	for offset := 0; offset < periodLen && offset < plan.value.length; offset++ {
		if qScriptWhereIndexResiduePredicateAt(plan, offset) {
			residues = append(residues, offset)
		}
	}
	if len(residues) == 0 {
		return
	}
	plan.residuePeriod = periodLen
	plan.residues = residues
}

func qScriptWhereIndexResiduePredicateAt(plan *qScriptWhereIndexSumPlan, offset int) bool {
	if plan == nil || plan.predicatePlan == nil {
		return qScriptWhereIndexPredicateAt(plan, offset)
	}
	return qScriptWherePredicateResidualAt(plan.predicatePlan, offset)
}

func qScriptWhereIndexConstrainedResidueSpec(plan *qScriptWhereIndexSumPlan) (qScriptWhereRowConstraint, int, bool) {
	if plan == nil || plan.predicatePlan == nil || plan.value.length <= 0 {
		return qScriptWhereRowConstraint{}, 0, false
	}
	constraint := qScriptWhereRowConstraint{start: 0, end: plan.value.length - 1}
	if !qScriptWherePredicateConstrainRows(plan.predicatePlan, &constraint) || constraint.start > constraint.end {
		return qScriptWhereRowConstraint{}, 0, false
	}
	if !constraint.changed && constraint.exclusionCount == 0 {
		return qScriptWhereRowConstraint{}, 0, false
	}
	periodLen := qScriptWherePredicateResidualPeriodLen(plan.predicatePlan)
	if periodLen == 0 && (constraint.changed || constraint.exclusionCount > 0) {
		periodLen = 1
	}
	if periodLen <= 0 || periodLen > qScriptWhereIndexClosedFormMaxPeriod {
		return qScriptWhereRowConstraint{}, 0, false
	}
	if valuePeriod := qScriptWhereIndexSummaryPeriodLen(plan.value); valuePeriod > 0 {
		periodLen = qScriptNumericLCM(periodLen, valuePeriod)
	}
	if periodLen <= 0 || periodLen > qScriptWhereIndexClosedFormMaxPeriod {
		return qScriptWhereRowConstraint{}, 0, false
	}
	return constraint, periodLen, true
}

func qScriptWhereIndexResidueSum(plan *qScriptWhereIndexSumPlan, start, end, periodLen int, residues []int) (float64, int64, bool, bool) {
	if plan == nil || periodLen <= 0 || start > end {
		return 0, 0, false, false
	}
	var sum float64
	var count int64
	isFloat := plan.value.isFloat
	for _, offset := range residues {
		if offset > end {
			continue
		}
		first := offset
		if first < start {
			delta := start - first
			first += ((delta + periodLen - 1) / periodLen) * periodLen
		}
		if first > end {
			continue
		}
		rows := (end-first)/periodLen + 1
		if rows <= 0 {
			continue
		}
		count += int64(rows)
		if part, partFloat, ok := qScriptWhereIndexValueResidueSum(plan.value, first, periodLen, rows); ok {
			sum += part
			isFloat = isFloat || partFloat
		}
	}
	return sum, count, isFloat, true
}

func qScriptWhereIndexConstrainedPeriodicSum(plan *qScriptWhereIndexSumPlan) (float64, int64, bool, bool) {
	constraint, periodLen, ok := qScriptWhereIndexConstrainedResidueSpec(plan)
	if !ok {
		return 0, 0, false, false
	}
	if len(plan.residues) > 0 && plan.residuePeriod == periodLen {
		sum, count, isFloat, ok := qScriptWhereIndexResidueSum(plan, constraint.start, constraint.end, periodLen, plan.residues)
		if !ok {
			return 0, 0, false, false
		}
		if constraint.exclusionCount == 0 {
			return sum, count, isFloat, true
		}
		for i := 0; i < constraint.exclusionCount; i++ {
			excluded := constraint.exclusions[i]
			if excluded < constraint.start || excluded > constraint.end || !qScriptWhereIndexResiduePredicateAt(plan, excluded) {
				continue
			}
			count--
			if !plan.value.IsNullAt(excluded) {
				sum -= plan.value.FloatAt(excluded)
			}
		}
		return sum, count, isFloat, true
	}
	var sum float64
	var count int64
	isFloat := plan.value.isFloat
	for offset := 0; offset < periodLen && offset <= constraint.end; offset++ {
		first := offset
		if first < constraint.start {
			delta := constraint.start - first
			first += ((delta + periodLen - 1) / periodLen) * periodLen
		}
		if first > constraint.end || !qScriptWherePredicateResidualAt(plan.predicatePlan, offset) {
			continue
		}
		rawRows := (constraint.end-first)/periodLen + 1
		excludedRows := qScriptWhereConstraintExclusionCount(constraint, first, periodLen)
		rows := rawRows - excludedRows
		if rawRows <= 0 || rows <= 0 {
			continue
		}
		count += int64(rows)
		if part, partFloat, ok := qScriptWhereIndexValueResidueSum(plan.value, first, periodLen, rawRows); ok {
			sum += part
			isFloat = isFloat || partFloat
		}
		for i := 0; i < constraint.exclusionCount; i++ {
			excluded := constraint.exclusions[i]
			if excluded < first || excluded > constraint.end || (excluded-first)%periodLen != 0 {
				continue
			}
			if plan.value.IsNullAt(excluded) {
				continue
			}
			sum -= plan.value.FloatAt(excluded)
		}
	}
	return sum, count, isFloat, true
}

func qScriptWhereIndexPeriodLen(plan *qScriptWhereIndexSumPlan) (int, bool) {
	if plan.predicatePlan != nil {
		periodLen := qScriptWherePredicatePeriodLen(plan.predicatePlan)
		if periodLen == 0 {
			return 0, false
		}
		if valuePeriod := qScriptWhereIndexSummaryPeriodLen(plan.value); valuePeriod > 0 {
			periodLen = qScriptNumericLCM(periodLen, valuePeriod)
		}
		if periodLen == 0 {
			return 0, false
		}
		return periodLen, true
	}
	periodLen := qScriptWhereIndexSummaryPeriodLen(plan.predicate)
	if periodLen == 0 {
		return 0, false
	}
	if valuePeriod := qScriptWhereIndexSummaryPeriodLen(plan.value); valuePeriod > 0 {
		periodLen = qScriptNumericLCM(periodLen, valuePeriod)
	}
	if periodLen == 0 {
		return 0, false
	}
	return periodLen, true
}

func qScriptWhereIndexSummaryPeriodLen(summary qScriptNumericSumSummary) int {
	switch {
	case len(summary.period) > 0:
		return len(summary.period)
	case len(summary.fperiod) > 0:
		return len(summary.fperiod)
	case summary.hasNull && len(summary.nulls) > 0:
		return len(summary.nulls)
	default:
		return 0
	}
}

func qScriptWhereIndexPredicateAt(plan *qScriptWhereIndexSumPlan, row int) bool {
	if plan.predicatePlan != nil {
		return qScriptWherePredicateAt(plan.predicatePlan, row)
	}
	if plan.predicate.IsNullAt(row) {
		return qScriptWhereIndexNullCompare(plan)
	}
	value := plan.predicate.FloatAt(row)
	if plan.op == "within" {
		return value >= plan.withinLo.FloatAt(0) && value <= plan.withinHi.FloatAt(0)
	}
	scalar := plan.scalar.FloatAt(0)
	switch plan.op {
	case ">":
		return value > scalar
	case ">=":
		return value >= scalar
	case "<":
		return value < scalar
	case "<=":
		return value <= scalar
	case "=":
		return value == scalar
	case "<>":
		return value != scalar
	default:
		return false
	}
}

func qScriptWherePredicateLength(plan *qScriptWherePredicatePlan) int {
	if plan == nil {
		return 0
	}
	switch plan.kind {
	case qScriptWherePredicateAnd:
		left := qScriptWherePredicateLength(plan.left)
		right := qScriptWherePredicateLength(plan.right)
		switch {
		case left > 0 && right > 0 && left == right:
			return left
		case left > 0 && right <= 0:
			return left
		case right > 0 && left <= 0:
			return right
		default:
			return 0
		}
	case qScriptWherePredicateNot:
		return qScriptWherePredicateLength(plan.left)
	case qScriptWherePredicateCompare, qScriptWherePredicateWithin:
		return plan.value.length
	case qScriptWherePredicateIn:
		if plan.symValue.length > 0 {
			return plan.symValue.length
		}
		return plan.value.length
	default:
		return 0
	}
}

func qScriptWherePredicatePeriodLen(plan *qScriptWherePredicatePlan) int {
	if plan == nil {
		return 0
	}
	switch plan.kind {
	case qScriptWherePredicateAnd:
		left := qScriptWherePredicatePeriodLen(plan.left)
		right := qScriptWherePredicatePeriodLen(plan.right)
		if left == 0 || right == 0 {
			return 0
		}
		return qScriptNumericLCM(left, right)
	case qScriptWherePredicateNot:
		return qScriptWherePredicatePeriodLen(plan.left)
	case qScriptWherePredicateCompare, qScriptWherePredicateWithin:
		return qScriptWhereIndexSummaryPeriodLen(plan.value)
	case qScriptWherePredicateIn:
		if len(plan.symValue.period) > 0 {
			return len(plan.symValue.period)
		}
		return qScriptWhereIndexSummaryPeriodLen(plan.value)
	default:
		return 0
	}
}

func qScriptWherePredicateConstrainRows(plan *qScriptWherePredicatePlan, constraint *qScriptWhereRowConstraint) bool {
	if plan == nil || constraint == nil {
		return false
	}
	switch plan.kind {
	case qScriptWherePredicateAnd:
		return qScriptWherePredicateConstrainRows(plan.left, constraint) &&
			qScriptWherePredicateConstrainRows(plan.right, constraint)
	case qScriptWherePredicateNot:
		if qScriptWherePredicateAddLinearInExclusions(plan.left, constraint) {
			return true
		}
		return qScriptWherePredicateResidualPeriodLen(plan) > 0
	case qScriptWherePredicateCompare:
		if qScriptWhereSummaryIsLinear(plan.value) {
			return qScriptWhereConstraintApplyCompare(constraint, plan.value, plan.op, plan.scalar)
		}
		return qScriptWhereIndexSummaryPeriodLen(plan.value) > 0
	case qScriptWherePredicateWithin:
		if qScriptWhereSummaryIsLinear(plan.value) {
			if !qScriptWhereConstraintApplyCompare(constraint, plan.value, ">=", plan.lo) {
				return false
			}
			return qScriptWhereConstraintApplyCompare(constraint, plan.value, "<=", plan.hi)
		}
		return qScriptWhereIndexSummaryPeriodLen(plan.value) > 0
	case qScriptWherePredicateIn:
		if len(plan.symValue.period) > 0 {
			return true
		}
		if qScriptWhereSummaryIsLinear(plan.value) {
			return false
		}
		return qScriptWhereIndexSummaryPeriodLen(plan.value) > 0
	default:
		return false
	}
}

func qScriptWherePredicateResidualPeriodLen(plan *qScriptWherePredicatePlan) int {
	if plan == nil {
		return 0
	}
	switch plan.kind {
	case qScriptWherePredicateAnd:
		left := qScriptWherePredicateResidualPeriodLen(plan.left)
		right := qScriptWherePredicateResidualPeriodLen(plan.right)
		if left == 0 {
			return right
		}
		if right == 0 {
			return left
		}
		return qScriptNumericLCM(left, right)
	case qScriptWherePredicateNot:
		return qScriptWherePredicateResidualPeriodLen(plan.left)
	case qScriptWherePredicateCompare, qScriptWherePredicateWithin:
		if qScriptWhereSummaryIsLinear(plan.value) {
			return 0
		}
		return qScriptWhereIndexSummaryPeriodLen(plan.value)
	case qScriptWherePredicateIn:
		if len(plan.symValue.period) > 0 {
			return len(plan.symValue.period)
		}
		if qScriptWhereSummaryIsLinear(plan.value) {
			return 0
		}
		return qScriptWhereIndexSummaryPeriodLen(plan.value)
	default:
		return 0
	}
}

func qScriptWherePredicateResidualAt(plan *qScriptWherePredicatePlan, row int) bool {
	if plan == nil {
		return true
	}
	switch plan.kind {
	case qScriptWherePredicateAnd:
		return qScriptWherePredicateResidualAt(plan.left, row) && qScriptWherePredicateResidualAt(plan.right, row)
	case qScriptWherePredicateNot:
		if qScriptWherePredicateLinearIn(plan.left) {
			return true
		}
		return !qScriptWherePredicateResidualAt(plan.left, row)
	case qScriptWherePredicateCompare, qScriptWherePredicateWithin:
		if qScriptWhereSummaryIsLinear(plan.value) {
			return true
		}
		return qScriptWherePredicateAt(plan, row)
	case qScriptWherePredicateIn:
		if len(plan.symValue.period) > 0 {
			return qScriptWherePredicateAt(plan, row)
		}
		if qScriptWhereSummaryIsLinear(plan.value) {
			return true
		}
		return qScriptWherePredicateAt(plan, row)
	default:
		return false
	}
}

func qScriptWherePredicateAddLinearInExclusions(plan *qScriptWherePredicatePlan, constraint *qScriptWhereRowConstraint) bool {
	if !qScriptWherePredicateLinearIn(plan) || constraint == nil {
		return false
	}
	for _, value := range plan.values {
		row, ok := qScriptWhereLinearRowForScalar(plan.value, value)
		if !ok {
			return false
		}
		if row >= constraint.start && row <= constraint.end {
			if constraint.exclusionCount >= len(constraint.exclusions) {
				return false
			}
			constraint.exclusions[constraint.exclusionCount] = row
			constraint.exclusionCount++
		}
	}
	return true
}

func qScriptWherePredicateLinearIn(plan *qScriptWherePredicatePlan) bool {
	return plan != nil && plan.kind == qScriptWherePredicateIn && qScriptWhereSummaryIsLinear(plan.value)
}

func qScriptWhereSummaryIsLinear(summary qScriptNumericSumSummary) bool {
	return summary.linear || summary.flinear
}

func qScriptWhereConstraintApplyCompare(constraint *qScriptWhereRowConstraint, value qScriptNumericSumSummary, op string, scalar qScriptNumericSumSummary) bool {
	if constraint == nil || scalar.IsNullAt(0) {
		return false
	}
	start, step, ok := qScriptWhereLinearStartStep(value)
	if !ok || step <= 0 {
		return false
	}
	target := scalar.FloatAt(0)
	switch op {
	case ">":
		return qScriptWhereConstraintUpdateStart(constraint, int(math.Floor((target-start)/step))+1)
	case ">=":
		return qScriptWhereConstraintUpdateStart(constraint, int(math.Ceil((target-start)/step)))
	case "<":
		return qScriptWhereConstraintUpdateEnd(constraint, int(math.Ceil((target-start)/step))-1)
	case "<=":
		return qScriptWhereConstraintUpdateEnd(constraint, int(math.Floor((target-start)/step)))
	case "=":
		pos := (target - start) / step
		row := int(math.Round(pos))
		if math.Abs(pos-float64(row)) > 1e-9 {
			constraint.start, constraint.end, constraint.changed = 1, 0, true
			return true
		}
		if !qScriptWhereConstraintUpdateStart(constraint, row) {
			return false
		}
		return qScriptWhereConstraintUpdateEnd(constraint, row)
	default:
		return false
	}
}

func qScriptWhereLinearStartStep(value qScriptNumericSumSummary) (float64, float64, bool) {
	switch {
	case value.linear:
		return float64(value.start), float64(value.step), true
	case value.flinear:
		return value.fstart, value.fstep, true
	default:
		return 0, 0, false
	}
}

func qScriptWhereLinearRowForScalar(value, scalar qScriptNumericSumSummary) (int, bool) {
	if scalar.IsNullAt(0) {
		return 0, false
	}
	start, step, ok := qScriptWhereLinearStartStep(value)
	if !ok || step == 0 {
		return 0, false
	}
	pos := (scalar.FloatAt(0) - start) / step
	row := int(math.Round(pos))
	if math.Abs(pos-float64(row)) > 1e-9 {
		return 0, false
	}
	return row, true
}

func qScriptWhereConstraintUpdateStart(constraint *qScriptWhereRowConstraint, start int) bool {
	if start > constraint.start {
		constraint.start = start
		constraint.changed = true
	}
	return true
}

func qScriptWhereConstraintUpdateEnd(constraint *qScriptWhereRowConstraint, end int) bool {
	if end < constraint.end {
		constraint.end = end
		constraint.changed = true
	}
	return true
}

func qScriptWhereConstraintExclusionCount(constraint qScriptWhereRowConstraint, first, periodLen int) int {
	if constraint.exclusionCount == 0 || periodLen <= 0 {
		return 0
	}
	count := 0
	for i := 0; i < constraint.exclusionCount; i++ {
		excluded := constraint.exclusions[i]
		if excluded < first || excluded > constraint.end || (excluded-first)%periodLen != 0 {
			continue
		}
		count++
	}
	return count
}

func qScriptWherePredicateAt(plan *qScriptWherePredicatePlan, row int) bool {
	if plan == nil {
		return false
	}
	switch plan.kind {
	case qScriptWherePredicateAnd:
		return qScriptWherePredicateAt(plan.left, row) && qScriptWherePredicateAt(plan.right, row)
	case qScriptWherePredicateNot:
		return !qScriptWherePredicateAt(plan.left, row)
	case qScriptWherePredicateCompare:
		if plan.value.IsNullAt(row) {
			return qScriptWherePredicateNullCompare(plan)
		}
		value := plan.value.FloatAt(row)
		scalar := plan.scalar.FloatAt(0)
		switch plan.op {
		case ">":
			return value > scalar
		case ">=":
			return value >= scalar
		case "<":
			return value < scalar
		case "<=":
			return value <= scalar
		case "=":
			return value == scalar
		case "<>":
			return value != scalar
		default:
			return false
		}
	case qScriptWherePredicateWithin:
		if plan.value.IsNullAt(row) {
			return false
		}
		value := plan.value.FloatAt(row)
		return value >= plan.lo.FloatAt(0) && value <= plan.hi.FloatAt(0)
	case qScriptWherePredicateIn:
		if len(plan.symValue.period) > 0 {
			probe := plan.symValue.At(row)
			for _, value := range plan.symValues {
				if probe == value {
					return true
				}
			}
			return false
		}
		if plan.value.IsNullAt(row) {
			for _, value := range plan.values {
				if value.IsNullAt(0) {
					return true
				}
			}
			return false
		}
		probe := plan.value.FloatAt(row)
		for _, value := range plan.values {
			if !value.IsNullAt(0) && probe == value.FloatAt(0) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func qScriptWherePredicateNullCompare(plan *qScriptWherePredicatePlan) bool {
	if plan == nil || plan.kind != qScriptWherePredicateCompare {
		return false
	}
	scalarNull := plan.scalar.IsNullAt(0)
	switch plan.op {
	case "=":
		return scalarNull
	case "<>":
		return !scalarNull
	case "<", "<=":
		return !scalarNull
	case ">", ">=":
		return false
	default:
		return false
	}
}

func qScriptWhereIndexNullCompare(plan *qScriptWhereIndexSumPlan) bool {
	if plan.op == "within" {
		return false
	}
	scalarNull := plan.scalar.IsNullAt(0)
	switch plan.op {
	case "=":
		return scalarNull
	case "<>":
		return !scalarNull
	case "<", "<=":
		return !scalarNull
	case ">", ">=":
		return false
	default:
		return false
	}
}

func qScriptWhereIndexValueResidueSum(value qScriptNumericSumSummary, offset, periodLen, rows int) (float64, bool, bool) {
	if rows <= 0 {
		return 0, value.isFloat, true
	}
	if value.linear {
		first := float64(value.start + int64(offset)*value.step)
		step := float64(periodLen) * float64(value.step)
		last := first + float64(rows-1)*step
		return float64(rows) * (first + last) / 2, false, true
	}
	if value.flinear {
		first := value.fstart + float64(offset)*value.fstep
		step := float64(periodLen) * value.fstep
		last := first + float64(rows-1)*step
		return float64(rows) * (first + last) / 2, true, true
	}
	if value.IsNullAt(offset) {
		return 0, value.isFloat, true
	}
	if value.isFloat {
		return float64(rows) * value.FloatAt(offset), true, true
	}
	if len(value.period) > 0 || value.scalar {
		return float64(rows) * float64(value.At(offset)), false, true
	}
	return math.NaN(), value.isFloat, false
}
