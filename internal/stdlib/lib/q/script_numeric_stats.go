package q

import (
	"math"
	"sort"
	"strings"
)

type qScriptNumericStatsPlan struct {
	source  string
	sources []string
	value   float64
}

type qScriptNumericStatsTerm string

const (
	qScriptNumericStatsSum  qScriptNumericStatsTerm = "sum"
	qScriptNumericStatsAvg  qScriptNumericStatsTerm = "avg"
	qScriptNumericStatsMed  qScriptNumericStatsTerm = "med"
	qScriptNumericStatsVar  qScriptNumericStatsTerm = "var"
	qScriptNumericStatsDev  qScriptNumericStatsTerm = "dev"
	qScriptNumericStatsWavg qScriptNumericStatsTerm = "wavg"
)

func buildQScriptNumericStatsPlan(statements []qScriptStatement) *qScriptNumericStatsPlan {
	if len(statements) < 2 {
		return nil
	}
	terminal := strings.TrimSpace(statements[len(statements)-1].src)
	terms, name, ok := qScriptNumericStatsTerminal(terminal)
	if !ok || len(terms) == 0 || name == "" {
		return nil
	}
	bindings := make(map[string]qScriptNumericExprPlan, len(statements)-1)
	for i := 0; i < len(statements)-1; i++ {
		stmt := statements[i]
		if stmt.assign == "" || stmt.idxAssignName != "" || stmt.rhs == "" {
			return nil
		}
		expr := compileQEvalExpr(stmt.rhs, 0)
		if expr == nil {
			return nil
		}
		plan, ok := buildQScriptNumericExprPlan(expr, bindings)
		if !ok {
			return nil
		}
		bindings[stmt.assign] = plan
	}
	root, ok := bindings[name]
	if !ok {
		return nil
	}
	summary, ok, err := qScriptNumericSummarize(root)
	if !ok || err != nil || summary.length <= 0 {
		return nil
	}
	stats := qScriptNumericStatsFromSummary(summary)
	var total float64
	for _, term := range terms {
		switch term {
		case qScriptNumericStatsSum:
			total += stats.sum
		case qScriptNumericStatsAvg:
			total += stats.avg
		case qScriptNumericStatsMed:
			total += stats.med
		case qScriptNumericStatsVar:
			total += stats.variance
		case qScriptNumericStatsDev:
			total += stats.dev
		case qScriptNumericStatsWavg:
			if stats.sum == 0 {
				return nil
			}
			total += stats.sumSquares / stats.sum
		default:
			return nil
		}
	}
	return &qScriptNumericStatsPlan{source: terminal, sources: qScriptNumericSumStatementSources(statements), value: total}
}

func qScriptNumericStatsTerminal(src string) ([]qScriptNumericStatsTerm, string, bool) {
	terms := splitTopLevelPlusChain(src)
	if len(terms) < 2 {
		return nil, "", false
	}
	out := make([]qScriptNumericStatsTerm, 0, len(terms))
	name := ""
	for _, raw := range terms {
		term, termName, ok := qScriptNumericStatsTerminalTerm(raw)
		if !ok {
			return nil, "", false
		}
		if name == "" {
			name = termName
		} else if name != termName {
			return nil, "", false
		}
		out = append(out, term)
	}
	return out, name, true
}

func qScriptNumericStatsTerminalTerm(src string) (qScriptNumericStatsTerm, string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "+/") {
		name := strings.TrimSpace(src[2:])
		if isQAssignmentName(name) {
			return qScriptNumericStatsSum, name, true
		}
		return "", "", false
	}
	for _, word := range []struct {
		prefix string
		term   qScriptNumericStatsTerm
	}{
		{"sum", qScriptNumericStatsSum},
		{"avg", qScriptNumericStatsAvg},
		{"med", qScriptNumericStatsMed},
		{"var", qScriptNumericStatsVar},
		{"dev", qScriptNumericStatsDev},
	} {
		if strings.HasPrefix(src, word.prefix+" ") && wordBoundary(src, 0, len(word.prefix)) {
			name := strings.TrimSpace(src[len(word.prefix)+1:])
			if isQAssignmentName(name) {
				return word.term, name, true
			}
			return "", "", false
		}
	}
	if strings.HasPrefix(src, "wavg[") && strings.HasSuffix(src, "]") {
		inner := strings.TrimSpace(src[len("wavg[") : len(src)-1])
		parts := splitTopLevelDelim(inner, ';')
		if len(parts) != 2 {
			return "", "", false
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left != right || !isQAssignmentName(left) {
			return "", "", false
		}
		return qScriptNumericStatsWavg, left, true
	}
	return "", "", false
}

type qScriptNumericStatsSummary struct {
	length     int
	sum        float64
	sumSquares float64
	avg        float64
	med        float64
	variance   float64
	dev        float64
}

func qScriptNumericStatsFromSummary(summary qScriptNumericSumSummary) qScriptNumericStatsSummary {
	n := summary.length
	var sum, sumSquares float64
	for row := 0; row < n; row++ {
		value := summary.FloatAt(row)
		sum += value
		sumSquares += value * value
	}
	avg := sum / float64(n)
	med := qScriptNumericStatsMedian(summary)
	variance := sumSquares/float64(n) - avg*avg
	if variance < 0 && variance > -1e-12 {
		variance = 0
	}
	return qScriptNumericStatsSummary{
		length:     n,
		sum:        sum,
		sumSquares: sumSquares,
		avg:        avg,
		med:        med,
		variance:   variance,
		dev:        math.Sqrt(variance),
	}
}

func qScriptNumericStatsMedian(summary qScriptNumericSumSummary) float64 {
	n := summary.length
	if n <= 0 {
		return math.NaN()
	}
	values := make([]float64, n)
	for row := 0; row < n; row++ {
		values[row] = summary.FloatAt(row)
	}
	// Numeric summaries used here are deterministic and bounded by source
	// length at plan build time; sorting once avoids imposing monotonicity.
	sort.Float64s(values)
	mid := n / 2
	if n%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

func (s *EvalState) evalQScriptNumericStatsPlan(plan *qScriptNumericStatsPlan) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	recordRuntimeKernelProbe("QScriptNumericStatsPlan", "vector-reduce/stats-envelope", true, nil)
	for _, source := range plan.sources {
		recordQEvalDispatch(source, EvalDispatchScriptStats)
	}
	if len(plan.sources) == 0 {
		recordQEvalDispatch(plan.source, EvalDispatchScriptStats)
	}
	return plan.value, true, nil
}
