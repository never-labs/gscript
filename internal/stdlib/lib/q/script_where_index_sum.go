package q

import (
	"math"
	"strings"
)

type qScriptWhereIndexSumPlan struct {
	source    string
	sources   []string
	value     qScriptNumericSumSummary
	predicate qScriptNumericSumSummary
	op        string
	scalar    qScriptNumericSumSummary
	withinLo  qScriptNumericSumSummary
	withinHi  qScriptNumericSumSummary
	countNull *qScriptNumericSumSummary
}

func buildQScriptWhereIndexSumPlan(statements []qScriptStatement) *qScriptWhereIndexSumPlan {
	if len(statements) < 3 {
		return nil
	}
	bindings := make(map[string]qScriptNumericExprPlan, len(statements))
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
	predicateSummary, op, scalar, withinLo, withinHi, ok := qScriptWhereIndexPredicateSummary(maskExpr, bindings)
	if !ok || predicateSummary.length != valueSummary.length {
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
	return &qScriptWhereIndexSumPlan{
		source:    terminal,
		sources:   qScriptNumericSumStatementSources(statements),
		value:     valueSummary,
		predicate: predicateSummary,
		op:        op,
		scalar:    scalar,
		withinLo:  withinLo,
		withinHi:  withinHi,
		countNull: countNull,
	}
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

func qScriptWhereIndexPredicateSummary(maskExpr string, bindings map[string]qScriptNumericExprPlan) (predicate qScriptNumericSumSummary, op string, scalar, lo, hi qScriptNumericSumSummary, ok bool) {
	if left, right, cmp, isCompare := splitWhereCompareExpr(maskExpr); isCompare {
		leftSummary, leftOK := qScriptWhereIndexExprSummary(left, bindings)
		rightSummary, rightOK := qScriptWhereIndexExprSummary(right, bindings)
		if !leftOK || !rightOK {
			return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
		}
		if leftSummary.scalar {
			flipped, flipOK := qScriptWhereIndexFlipCompare(cmp)
			if !flipOK {
				return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
			}
			return rightSummary, flipped, leftSummary, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, true
		}
		if !rightSummary.scalar {
			return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
		}
		return leftSummary, cmp, rightSummary, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, true
	}
	if left, right, isWithin := splitTopLevelWord(maskExpr, "within"); isWithin {
		predicate, predOK := qScriptWhereIndexExprSummary(left, bindings)
		if !predOK {
			return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
		}
		bounds := strings.Fields(stripEnclosingParens(strings.TrimSpace(right)))
		if len(bounds) != 2 {
			return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
		}
		lo, loOK := qScriptWhereIndexExprSummary(bounds[0], bindings)
		hi, hiOK := qScriptWhereIndexExprSummary(bounds[1], bindings)
		if !loOK || !hiOK || !lo.scalar || !hi.scalar {
			return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
		}
		return predicate, "within", qScriptNumericSumSummary{}, lo, hi, true
	}
	return qScriptNumericSumSummary{}, "", qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, qScriptNumericSumSummary{}, false
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
	if !ok || periodLen <= 0 || periodLen > qScriptNumericClosedFormMaxPeriod {
		return 0, 0, false, false
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

func qScriptWhereIndexPeriodLen(plan *qScriptWhereIndexSumPlan) (int, bool) {
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
