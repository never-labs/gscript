package q

import (
	"fmt"
	"strings"
)

type qScriptNumericMultiSumPlan struct {
	source    string
	sources   []string
	roots     []qScriptNumericExprPlan
	closed    []qScriptNumericSumSummary
	closedOK  bool
	closedErr string
}

func buildQScriptNumericMultiSumPlan(statements []qScriptStatement) *qScriptNumericMultiSumPlan {
	if len(statements) < 2 {
		return nil
	}
	terminal := strings.TrimSpace(statements[len(statements)-1].src)
	targets, ok := qScriptNumericMultiSumTargets(terminal)
	if !ok {
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
	roots := make([]qScriptNumericExprPlan, 0, len(targets))
	closed := make([]qScriptNumericSumSummary, 0, len(targets))
	closedOK := true
	var closedErr string
	for _, target := range targets {
		var root qScriptNumericExprPlan
		if target.expr != "" {
			expr := compileQEvalExpr(target.expr, 0)
			if expr == nil {
				return nil
			}
			var ok bool
			root, ok = buildQScriptNumericExprPlan(expr, bindings)
			if !ok {
				return nil
			}
		} else {
			var ok bool
			root, ok = bindings[target.name]
			if !ok {
				return nil
			}
		}
		if _, ok := qScriptNumericExprLength(root, bindings); !ok {
			return nil
		}
		roots = append(roots, root)
		summary, ok, err := qScriptNumericSummarize(root)
		if !ok {
			closedOK = false
		}
		if err != nil && closedErr == "" {
			closedErr = err.Error()
		}
		if ok && !root.hasCast && !root.hasNull && !qScriptNumericMultiClosedAllowed(root, summary) {
			closedOK = false
		}
		closed = append(closed, summary)
	}
	if !closedOK {
		return nil
	}
	return &qScriptNumericMultiSumPlan{
		source:    terminal,
		sources:   qScriptNumericSumStatementSources(statements),
		roots:     roots,
		closed:    closed,
		closedOK:  closedOK,
		closedErr: closedErr,
	}
}

func qScriptNumericMultiClosedAllowed(plan qScriptNumericExprPlan, summary qScriptNumericSumSummary) bool {
	if qScriptNumericPlainClosedAllowed(plan, summary) {
		return true
	}
	return !summary.isFloat && !summary.hasNull && (summary.scalar || summary.linear || summary.custom || len(summary.period) > 0)
}

func qScriptNumericMultiSumTargets(src string) ([]qScriptNumericSumTargetPlan, bool) {
	terms := splitTopLevelPlusChain(src)
	if len(terms) < 2 {
		return nil, false
	}
	targets := make([]qScriptNumericSumTargetPlan, 0, len(terms))
	for _, term := range terms {
		target, ok := qScriptNumericSumTarget(stripEnclosingParens(strings.TrimSpace(term)))
		if !ok {
			return nil, false
		}
		targets = append(targets, target)
	}
	return targets, true
}

func (s *EvalState) evalQScriptNumericMultiSumPlan(plan *qScriptNumericMultiSumPlan) (any, bool, error) {
	sum, handled, err := qScriptNumericMultiSum(plan)
	if err != nil || !handled {
		return nil, handled, err
	}
	recordRuntimeKernelProbe("QScriptNumericMultiSumPlan", "multi-vector-reduce/int-expr-sum", true, nil)
	for _, source := range plan.sources {
		recordQEvalDispatch(source, EvalDispatchScriptNumericSum)
	}
	return sum, true, nil
}

func (s *EvalState) evalQScriptNumericMultiSumPlanScalar(plan *qScriptNumericMultiSumPlan) (EvalScalarResult, bool, error) {
	sum, handled, err := qScriptNumericMultiSum(plan)
	if err != nil || !handled {
		return EvalScalarResult{}, handled, err
	}
	recordRuntimeKernelProbe("QScriptNumericMultiSumPlan", "multi-vector-reduce/int-expr-sum", true, nil)
	for _, source := range plan.sources {
		recordQEvalDispatch(source, EvalDispatchScriptNumericSum)
	}
	return evalScalarInt(sum), true, nil
}

func qScriptNumericMultiSum(plan *qScriptNumericMultiSumPlan) (int64, bool, error) {
	if plan == nil || len(plan.roots) == 0 {
		return 0, false, nil
	}
	if plan.closedOK {
		if plan.closedErr != "" {
			err := fmt.Errorf("%s", plan.closedErr)
			recordRuntimeKernelProbe("QScriptNumericMultiSumPlan", "multi-vector-reduce/int-expr-sum", false, err)
			return 0, true, err
		}
		var total int64
		for _, summary := range plan.closed {
			if summary.isFloat {
				return 0, false, nil
			}
			total += summary.Sum()
		}
		return total, true, nil
	}
	return 0, false, nil
}
