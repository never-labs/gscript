package q

import (
	"strings"
	"sync"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// Apply-form statement plans.
//
// evalApplyIndexForm walks @/. apply, index, and amend statements with
// per-call string splitting. The plans in this file compile the same forms
// once - the bracket splits, operand sub-expressions, and the amend verb
// resolve at build time - and execute through the SAME functions the string
// walk dispatches to (applyOrIndexValue, amendValue, amendValueWithFunction,
// applyCallable*). A plan that cannot represent the runtime values (an
// unsupported sub-expression, an unbound name the string cascade resolves
// further) declines, and the per-call string walk runs unchanged.

type qApplyFormKind uint8

const (
	qApplyFormInvalid qApplyFormKind = iota
	// qApplyFormAt is a top-level `left@right` apply/index split.
	qApplyFormAt
	// qApplyFormAmendAt is `@[target;index;op;value]` with op either `:` or
	// a registered dyadic verb token.
	qApplyFormAmendAt
	// qApplyFormDotApply is two-part `.[fn;args]`.
	qApplyFormDotApply
)

type qApplyFormPlan struct {
	kind      qApplyFormKind
	at        ApplyAtExpr
	target    Expr
	index     Expr
	op        any // nil means `:` assignment
	value     Expr
	fn        Expr
	args      []Expr
	tupleArgs bool
}

var (
	qGlobalApplyFormPlanMu    sync.Mutex
	qGlobalApplyFormPlanCache = map[string]*qApplyFormPlan{}
)

const qGlobalApplyFormPlanCacheLimit = 4096

var qApplyFormPlanInvalidShared = &qApplyFormPlan{}

func qApplyFormPlanCached(src string) *qApplyFormPlan {
	if qStatementCompileDisabled() {
		return qApplyFormPlanInvalidShared
	}
	qGlobalApplyFormPlanMu.Lock()
	if plan, ok := qGlobalApplyFormPlanCache[src]; ok {
		qGlobalApplyFormPlanMu.Unlock()
		return plan
	}
	qGlobalApplyFormPlanMu.Unlock()
	plan := buildQApplyFormPlan(src)
	qGlobalApplyFormPlanMu.Lock()
	if len(qGlobalApplyFormPlanCache) >= qGlobalApplyFormPlanCacheLimit {
		qGlobalApplyFormPlanCache = make(map[string]*qApplyFormPlan, 64)
	}
	qGlobalApplyFormPlanCache[src] = plan
	qGlobalApplyFormPlanMu.Unlock()
	return plan
}

func buildQApplyFormPlan(src string) *qApplyFormPlan {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "@[") && strings.HasSuffix(src, "]") {
		// Mirror evalAtApplyOrAmend's arity gate, then evalAmend's split.
		inner := strings.TrimSpace(src[2 : len(src)-1])
		if len(splitTopLevelDelim(inner, ';')) != 4 {
			return qApplyFormPlanInvalidShared
		}
		parts := splitQBracketFormArgs(inner)
		if len(parts) != 4 {
			return qApplyFormPlanInvalidShared
		}
		target := compileQEvalExpr(parts[0], 0)
		index := compileQEvalExpr(parts[1], 0)
		value := compileQEvalExpr(parts[3], 0)
		if target == nil || index == nil || value == nil {
			return qApplyFormPlanInvalidShared
		}
		plan := &qApplyFormPlan{kind: qApplyFormAmendAt, target: target, index: index, value: value}
		if opSrc := strings.TrimSpace(parts[2]); opSrc != ":" {
			fn, ok := lookupDyadicVerbFunc(opSrc)
			if !ok {
				return qApplyFormPlanInvalidShared
			}
			// The same value s.eval(opSrc) resolves for a bare verb token.
			plan.op = qDyadicFunction{name: opSrc, fn: fn}
		}
		return plan
	}
	if strings.HasPrefix(src, ".[") && strings.HasSuffix(src, "]") {
		// Mirror buildDotApplyPlan's two-part shape with compiled operands.
		inner := strings.TrimSpace(src[2 : len(src)-1])
		parts := splitTopLevelDelim(inner, ';')
		if len(parts) != 2 {
			return qApplyFormPlanInvalidShared
		}
		fn := compileQEvalExpr(parts[0], 0)
		if fn == nil {
			return qApplyFormPlanInvalidShared
		}
		plan := &qApplyFormPlan{kind: qApplyFormDotApply, fn: fn}
		argSrc := strings.TrimSpace(parts[1])
		if argSrc == "()" {
			return plan
		}
		if enclosed(argSrc, '(', ')') {
			tuple := strings.TrimSpace(argSrc[1 : len(argSrc)-1])
			argParts := splitTopLevelDelim(tuple, ';')
			if len(argParts) > 1 {
				plan.args = make([]Expr, len(argParts))
				for i, part := range argParts {
					if plan.args[i] = compileQEvalExpr(part, 0); plan.args[i] == nil {
						return qApplyFormPlanInvalidShared
					}
				}
				return plan
			}
		}
		arg := compileQEvalExpr(argSrc, 0)
		if arg == nil {
			return qApplyFormPlanInvalidShared
		}
		plan.args = []Expr{arg}
		plan.tupleArgs = true
		return plan
	}
	if _, _, ok := splitTopLevelOperator(src, "@"); ok {
		if expr := compileQEvalExpr(src, 0); expr != nil {
			if at, ok := expr.(ApplyAtExpr); ok {
				return &qApplyFormPlan{kind: qApplyFormAt, at: at}
			}
		}
		return qApplyFormPlanInvalidShared
	}
	return qApplyFormPlanInvalidShared
}

// evalQApplyFormPlan executes the compiled apply-form plan for src.
// handled=false (including unsupported sub-expressions) leaves the statement
// on the per-call apply string walk.
func (s *EvalState) evalQApplyFormPlan(src string) (any, bool, error) {
	plan := qApplyFormPlanCached(src)
	switch plan.kind {
	case qApplyFormAt:
		out, err := s.evalApplyAtExpr(plan.at)
		if err != nil && isUnsupportedEvalValueExpr(err) {
			return nil, false, nil
		}
		return out, true, err
	case qApplyFormAmendAt:
		// Mirror evalAmend's operand evaluation order: target, index, value.
		target, err := s.evalValueExpr(plan.target)
		if err != nil {
			return s.qApplyFormPlanError(err)
		}
		index, err := s.evalValueExpr(plan.index)
		if err != nil {
			return s.qApplyFormPlanError(err)
		}
		value, err := s.evalValueExpr(plan.value)
		if err != nil {
			return s.qApplyFormPlanError(err)
		}
		if plan.op == nil {
			out, err := amendValue(target, index, value)
			return out, true, err
		}
		out, err := s.amendValueWithFunction(target, index, plan.op, value)
		return out, true, err
	case qApplyFormDotApply:
		fn, err := s.evalValueExpr(plan.fn)
		if err != nil {
			return s.qApplyFormPlanError(err)
		}
		switch len(plan.args) {
		case 0:
			out, err := s.applyCallable(fn, nil)
			return out, true, err
		case 1:
			arg, err := s.evalValueExpr(plan.args[0])
			if err != nil {
				return s.qApplyFormPlanError(err)
			}
			if plan.tupleArgs {
				if array, ok := arg.(data.Array); ok {
					out, err := s.applyCallableArrayArgs(fn, array)
					return out, true, err
				}
				out, err := s.applyCallable(fn, qApplyArgs(arg))
				return out, true, err
			}
			out, err := s.applyCallable1(fn, arg)
			return out, true, err
		case 2:
			left, err := s.evalValueExpr(plan.args[0])
			if err != nil {
				return s.qApplyFormPlanError(err)
			}
			right, err := s.evalValueExpr(plan.args[1])
			if err != nil {
				return s.qApplyFormPlanError(err)
			}
			out, err := s.applyCallable2(fn, left, right)
			return out, true, err
		default:
			args := make([]any, len(plan.args))
			for i := range plan.args {
				value, err := s.evalValueExpr(plan.args[i])
				if err != nil {
					return s.qApplyFormPlanError(err)
				}
				args[i] = value
			}
			out, err := s.applyCallable(fn, args)
			return out, true, err
		}
	default:
		return nil, false, nil
	}
}

// qApplyFormPlanError maps unsupported sub-expressions to a plan decline
// (the string walk re-runs) and surfaces every other evaluation error, which
// the string walk would raise identically.
func (s *EvalState) qApplyFormPlanError(err error) (any, bool, error) {
	if isUnsupportedEvalValueExpr(err) {
		return nil, false, nil
	}
	return nil, true, err
}
