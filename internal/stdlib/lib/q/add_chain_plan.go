package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// Add-chain statement plans cache the per-statement classification that
// tryEvalScalarAddChain re-derives from the source string on every call.
// buildQAddChainPlan runs once per statement (script plans persist per
// EvalState and in the global script-plan cache); evalQAddChainPlan then
// re-executes the resolved term routes per call, so kernel work still runs
// every call while the string walks (splitTopLevelPlusChain,
// parseCallableOverApplication, the s.eval probe cascade for reducer terms)
// are paid once.
//
// Every term kind mirrors an existing evalScalarAddChainTerm / s.eval route
// exactly — same value functions, same kernel-stat probes, same fallback to
// s.eval(src) when the resolved route does not apply at runtime.

type qAddChainTermKind uint8

const (
	// qAddChainTermEval falls back to the string interpreter per call,
	// exactly like evalScalarAddChainTerm's terminal s.eval(src).
	qAddChainTermEval qAddChainTermKind = iota
	// qAddChainTermCallableOver mirrors tryEvalCallableOverScalar
	// (fn/[initial;value]) with the application parse cached.
	qAddChainTermCallableOver
	// qAddChainTermLiteral is a parsed number/bool literal.
	qAddChainTermLiteral
	// qAddChainTermScalarIndex mirrors the buildScalarApplyIndexPlan probe.
	qAddChainTermScalarIndex
	// qAddChainTermCountName mirrors evalScalarAddChainTerm's
	// `count <name>` lookup fast path.
	qAddChainTermCountName
	// qAddChainTermFast wraps a nested qEvalFastPlan (pipeline /
	// sort-rank / scalar-index routes), matching the early
	// qPipelinePlanRef / bundle probes inside s.eval.
	qAddChainTermFast
	// qAddChainTermReducerName applies a unary reducer verb to a plain
	// name, matching s.eval's `+/<name>` branch and unary word prefix
	// table terminal (sum/prd/avg/min/max/first/last over a lookup).
	qAddChainTermReducerName
	// qAddChainTermLastScanWord mirrors tryEvalLastScan
	// (`last sums x` → sum x and friends).
	qAddChainTermLastScanWord
	// qAddChainTermLastCallableScan mirrors tryEvalLastCallableScan
	// (`last (fn\[init;value])`).
	qAddChainTermLastCallableScan
	// qAddChainTermName is a plain binding name: one lookupName per call,
	// mirroring evalScalarAddChainTerm's terminal s.eval(name) route.
	qAddChainTermName
	// qAddChainTermDyadicApply is a `<verb>[a;b]` application of a dyadic
	// stats word (wavg/wsum/cov/scov/cor). It calls the same function the
	// s.eval bracket-application terminal reaches, with the argument parse
	// cached.
	qAddChainTermDyadicApply
)

// qTermArgPlan caches how a small operand sub-expression resolves: literal,
// plain name lookup, or per-call s.eval fallback. Name lookups that miss at
// runtime fall back to s.eval(src) so error semantics stay identical.
type qTermArgKind uint8

const (
	qTermArgEval qTermArgKind = iota
	qTermArgLiteral
	qTermArgName
)

type qTermArgPlan struct {
	kind    qTermArgKind
	src     string
	literal any
	name    string
}

func buildQTermArgPlan(src string) qTermArgPlan {
	src = strings.TrimSpace(src)
	if src == "" {
		return qTermArgPlan{kind: qTermArgEval, src: src}
	}
	if value, _, err := parseNumberOrBool(src); err == nil {
		return qTermArgPlan{kind: qTermArgLiteral, src: src, literal: value}
	}
	if isQAssignmentName(src) {
		return qTermArgPlan{kind: qTermArgName, src: src, name: src}
	}
	return qTermArgPlan{kind: qTermArgEval, src: src}
}

func (s *EvalState) evalQTermArg(plan *qTermArgPlan) (any, error) {
	switch plan.kind {
	case qTermArgLiteral:
		return plan.literal, nil
	case qTermArgName:
		if value, ok := s.lookupName(plan.name); ok {
			return value, nil
		}
	}
	return s.eval(plan.src)
}

// qCallableApplicationPlan caches the parse of fn/[initial;value] and
// fn\[initial;value] applications. A brace-enclosed fn literal skips the
// s.eval cascade and constructs the qLambda directly (the same terminal
// s.eval reaches for an enclosed-brace source); env is still cloned per
// call, preserving capture semantics.
type qCallableApplicationPlan struct {
	fnSrc        string
	fnLambdaBody string
	fnIsLambda   bool
	hasInitial   bool
	initial      qTermArgPlan
	value        qTermArgPlan
}

func buildQCallableApplicationPlan(fnSrc, initialSrc, valueSrc string) qCallableApplicationPlan {
	plan := qCallableApplicationPlan{fnSrc: fnSrc, value: buildQTermArgPlan(valueSrc)}
	if initialSrc != "" {
		plan.hasInitial = true
		plan.initial = buildQTermArgPlan(initialSrc)
	}
	if enclosed(fnSrc, '{', '}') {
		plan.fnIsLambda = true
		plan.fnLambdaBody = strings.TrimSpace(fnSrc[1 : len(fnSrc)-1])
	}
	return plan
}

func (s *EvalState) evalQCallableApplicationFn(plan *qCallableApplicationPlan) (any, error) {
	if plan.fnIsLambda {
		return qLambda{body: plan.fnLambdaBody, env: cloneEnv(s.env), namespace: s.namespace}, nil
	}
	return s.eval(plan.fnSrc)
}

type qAddChainTermPlan struct {
	kind        qAddChainTermKind
	src         string
	literal     any
	name        string
	reducer     func(any) (any, error)
	reducerWord string
	dyadicFn    func(any, any) (any, error)
	dyadicLeft  qTermArgPlan
	dyadicRight qTermArgPlan
	scalarIndex qScalarApplyIndexPlan
	fast        *qEvalFastPlan
	over        qCallableApplicationPlan
	scanWord    string
	scanInner   qTermArgPlan
	// monadBundle, when set on the first term of a run of consecutive
	// sum-unary terms over the same reduction input, evaluates the whole
	// run through the fused multi-monad sum kernel (one pass over the
	// source). The covered terms keep their individual plans so a kernel
	// decline falls back to per-term evaluation unchanged.
	monadBundle *qMonadSumBundlePlan
}

// qMonadSumBundlePlan groups span consecutive add-chain terms of the form
// (+/<monad> <input>) sharing one reduction input. terms holds the original
// per-term plans: terms[0]'s pipeline supplies the input expression and
// binding plan, and all terms remain the fallback route when the fused
// kernel declines the runtime value shape.
type qMonadSumBundlePlan struct {
	ops    []string
	span   int
	terms  []qAddChainTermPlan
	kernel *data.QNumericUnaryMultiSumPlan
}

// buildQAddChainPlan validates src with the exact tryEvalScalarAddChain
// gate, then resolves a cached route per term. It returns nil when the
// statement must keep flowing through the per-call probe chain (including
// the tryEvalWhereCompareCountSum probe, which runs before
// tryEvalScalarAddChain in evalCachedOrStringUncached).
func buildQAddChainPlan(src string) []qAddChainTermPlan {
	terms := splitTopLevelPlusChain(src)
	if len(terms) < 2 {
		return nil
	}
	for _, term := range terms {
		if !isScalarAddChainTerm(term) {
			return nil
		}
	}
	if qWhereCompareCountSumCandidate(src) {
		return nil
	}
	plans := make([]qAddChainTermPlan, len(terms))
	for i, term := range terms {
		plans[i] = buildQAddChainTermPlan(term)
	}
	attachQMonadSumBundles(plans)
	return plans
}

// qMonadSumBundleTermPipeline returns the sum-unary pipeline plan backing an
// add-chain term, or nil when the term is not a fused-monad-sum candidate.
func qMonadSumBundleTermPipeline(plan *qAddChainTermPlan) *qPipelinePlan {
	if plan.kind != qAddChainTermFast || plan.fast == nil {
		return nil
	}
	if plan.fast.kind != qEvalFastPipeline || plan.fast.pipeline.kind != qPipelineSumUnaryPrimitive {
		return nil
	}
	if strings.TrimSpace(plan.fast.pipeline.unaryOp) == "" || strings.TrimSpace(plan.fast.pipeline.reductionInput) == "" {
		return nil
	}
	return &plan.fast.pipeline
}

// attachQMonadSumBundles marks runs of >=2 consecutive sum-unary terms over
// the same reduction input for fused evaluation (one source pass via
// data.TryTypedQNumericUnaryMultiSum instead of one per term).
func attachQMonadSumBundles(plans []qAddChainTermPlan) {
	for i := 0; i < len(plans); {
		first := qMonadSumBundleTermPipeline(&plans[i])
		if first == nil {
			i++
			continue
		}
		j := i + 1
		for j < len(plans) {
			next := qMonadSumBundleTermPipeline(&plans[j])
			if next == nil || next.reductionInput != first.reductionInput {
				break
			}
			j++
		}
		if j-i >= 2 {
			bundle := &qMonadSumBundlePlan{
				ops:   make([]string, j-i),
				span:  j - i,
				terms: make([]qAddChainTermPlan, j-i),
			}
			copy(bundle.terms, plans[i:j])
			for k := i; k < j; k++ {
				bundle.ops[k-i] = qMonadSumBundleTermPipeline(&plans[k]).unaryOp
			}
			if kernel, ok := data.NewQNumericUnaryMultiSumPlan(bundle.ops); ok {
				bundle.kernel = kernel
				plans[i].monadBundle = bundle
			}
		}
		i = j
	}
}

// evalQMonadSumBundlePlan evaluates the shared reduction input once and asks
// the fused multi-monad kernel for all per-term sums. handled=false keeps
// the caller on its per-term route (identical fallback semantics).
func (s *EvalState) evalQMonadSumBundlePlan(bundle *qMonadSumBundlePlan) ([]any, bool, error) {
	if bundle.kernel == nil {
		return nil, false, nil
	}
	pipeline := qMonadSumBundleTermPipeline(&bundle.terms[0])
	if pipeline == nil {
		return nil, false, nil
	}
	value, err := s.evalQPipelinePlannedExpr(pipeline.reductionInput, &pipeline.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	values, handled, err := data.TryTypedQNumericUnaryMultiSumPlanned(bundle.kernel, array)
	recordRuntimeKernelProbe("ArrayNumericUnaryMultiSum", "vector-reduce/multi-sum/"+string(array.Kind()), handled, err)
	return values, handled, err
}

// qWhereCompareCountSumCandidate is the pure syntactic gate of
// tryEvalWhereCompareCountSum.
func qWhereCompareCountSumCandidate(src string) bool {
	idx, op, ok := findDyadic(src)
	if !ok || op != '+' {
		return false
	}
	left := stripEnclosedParens(strings.TrimSpace(src[:idx]))
	right := stripEnclosedParens(strings.TrimSpace(src[idx+1:]))
	_, ok = matchingCountSumWhereCompare(left, right)
	return ok
}

// buildQAddChainTermPlan resolves one chain term following
// evalScalarAddChainTerm's probe order, then the s.eval cascade order for
// the routes hoisted out of it (fast plans before reducer words before the
// `last` scan heads).
func buildQAddChainTermPlan(termSrc string) qAddChainTermPlan {
	stripped := strings.TrimSpace(stripEnclosingParens(termSrc))
	plan := qAddChainTermPlan{kind: qAddChainTermEval, src: stripped}
	if stripped == "" {
		return plan
	}
	if fnSrc, initialSrc, valueSrc, ok := parseCallableOverApplication(stripped); ok {
		plan.kind = qAddChainTermCallableOver
		plan.over = buildQCallableApplicationPlan(fnSrc, initialSrc, valueSrc)
		return plan
	}
	if qScalarAddChainTermMayBeNumber(stripped) {
		if value, _, err := parseNumberOrBool(stripped); err == nil {
			plan.kind = qAddChainTermLiteral
			plan.literal = value
			return plan
		}
	}
	if qAddChainBareNameTerm(stripped) {
		plan.kind = qAddChainTermName
		plan.name = stripped
		return plan
	}
	if qScalarAddChainTermMayBeScalarIndex(stripped) {
		if scalar, ok := buildScalarApplyIndexPlan(stripped); ok {
			plan.kind = qAddChainTermScalarIndex
			plan.scalarIndex = scalar
			return plan
		}
	}
	if strings.HasPrefix(stripped, "count ") && wordBoundary(stripped, 0, len("count")) {
		if arg := strings.TrimSpace(stripped[len("count "):]); isQAssignmentName(arg) {
			plan.kind = qAddChainTermCountName
			plan.name = arg
			return plan
		}
	}
	if sub := buildQEvalFastPlanTermRoutes(stripped); sub.kind != qEvalFastInvalid {
		plan.kind = qAddChainTermFast
		plan.fast = &sub
		return plan
	}
	if reducer, word, name, ok := qAddChainReducerNameTerm(stripped); ok {
		plan.kind = qAddChainTermReducerName
		plan.reducer = reducer
		plan.reducerWord = word
		plan.name = name
		return plan
	}
	if fn, leftArg, rightArg, ok := qAddChainDyadicWordApplyTerm(stripped); ok {
		plan.kind = qAddChainTermDyadicApply
		plan.dyadicFn = fn
		plan.dyadicLeft = buildQTermArgPlan(leftArg)
		plan.dyadicRight = buildQTermArgPlan(rightArg)
		return plan
	}
	if lastPlan, ok := buildQLastHeadTermPlan(stripped); ok {
		return lastPlan
	}
	return plan
}

// qAddChainReducerNameTerm recognizes `+/<name>` and `<reducer> <name>` for
// the unary reducer verbs evalScalarAddChainTerm accepts. The reducer
// functions are the same terminals s.eval reaches for these shapes (the
// `+/` branch's sum and the unary word prefix table).
func qAddChainReducerNameTerm(src string) (func(any) (any, error), string, string, bool) {
	if strings.HasPrefix(src, "+/") {
		name := strings.TrimSpace(src[len("+/"):])
		if isQAssignmentName(name) {
			return sum, "sum", name, true
		}
		return nil, "", "", false
	}
	for _, spec := range []struct {
		word string
		fn   func(any) (any, error)
	}{
		{"sum", sum},
		{"prd", prd},
		{"avg", avg},
		{"min", minValue},
		{"max", maxValue},
		{"first", first},
		{"last", last},
		{"var", varValue},
		{"dev", devValue},
		{"svar", svarValue},
		{"sdev", sdevValue},
		{"med", medValue},
	} {
		if !strings.HasPrefix(src, spec.word) || !wordBoundary(src, 0, len(spec.word)) {
			continue
		}
		name := strings.TrimSpace(src[len(spec.word):])
		if isQAssignmentName(name) {
			return spec.fn, spec.word, name, true
		}
		return nil, "", "", false
	}
	return nil, "", "", false
}

// qAddChainDyadicWordApplyTerm recognizes `<verb>[a;b]` applications of the
// dyadic stats words. The functions are the same terminals the s.eval
// bracket-application route reaches through qDyadicWordOps; argument parsing
// stays conservative (one top-level `;`, no nesting) so anything else keeps
// the per-call eval route.
func qAddChainDyadicWordApplyTerm(src string) (func(any, any) (any, error), string, string, bool) {
	if !strings.HasSuffix(src, "]") {
		return nil, "", "", false
	}
	open := strings.IndexByte(src, '[')
	if open <= 0 {
		return nil, "", "", false
	}
	var fn func(any, any) (any, error)
	switch src[:open] {
	case "wavg":
		fn = wavg
	case "wsum":
		fn = wsumValue
	case "cov":
		fn = covValue
	case "scov":
		fn = scovValue
	case "cor":
		fn = corValue
	default:
		return nil, "", "", false
	}
	inner := src[open+1 : len(src)-1]
	if strings.ContainsAny(inner, "[](){}\"`") {
		return nil, "", "", false
	}
	parts := strings.Split(inner, ";")
	if len(parts) != 2 {
		return nil, "", "", false
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if left == "" || right == "" {
		return nil, "", "", false
	}
	return fn, left, right, true
}

var qLastScanWordSpecs = []struct {
	word string
	fn   func(any) (any, error)
}{
	{"sums ", sum},
	{"prds ", prd},
	{"mins ", minValue},
	{"maxs ", maxValue},
	{"avgs ", avg},
}

// buildQLastHeadTermPlan resolves `last <scan>` terms following the s.eval
// `last ` branch order: tryEvalLastDyadicTerminal candidates force the eval
// fallback (their evaluation is operand-driven), tryEvalLastScan words and
// tryEvalLastCallableScan applications get cached routes.
func buildQLastHeadTermPlan(src string) (qAddChainTermPlan, bool) {
	if !strings.HasPrefix(src, "last ") || !wordBoundary(src, 0, len("last")) {
		return qAddChainTermPlan{}, false
	}
	arg := strings.TrimSpace(src[len("last "):])
	if qLastDyadicTerminalCandidate(arg) {
		return qAddChainTermPlan{}, false
	}
	for _, spec := range qLastScanWordSpecs {
		if !strings.HasPrefix(arg, spec.word) {
			continue
		}
		return qAddChainTermPlan{
			kind:      qAddChainTermLastScanWord,
			src:       src,
			scanWord:  strings.TrimSpace(spec.word),
			reducer:   spec.fn,
			scanInner: buildQTermArgPlan(arg[len(spec.word):]),
		}, true
	}
	if fnSrc, initialSrc, valueSrc, ok := parseCallableScanApplication(stripEnclosingParens(arg)); ok {
		return qAddChainTermPlan{
			kind: qAddChainTermLastCallableScan,
			src:  src,
			over: buildQCallableApplicationPlan(fnSrc, initialSrc, valueSrc),
		}, true
	}
	return qAddChainTermPlan{}, false
}

// qLastDyadicTerminalCandidate is the syntactic gate of
// tryEvalLastDyadicTerminal.
func qLastDyadicTerminalCandidate(src string) bool {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, candidate := range []struct {
		token string
		word  bool
	}{
		{token: "+"},
		{token: "-"},
		{token: "*"},
		{token: "%"},
		{token: "mod", word: true},
		{token: "div", word: true},
		{token: "<"},
		{token: ">"},
		{token: "="},
	} {
		if candidate.word {
			if _, _, ok := splitTopLevelWord(src, candidate.token); ok {
				return true
			}
		} else if _, _, ok := splitTopLevelOperator(src, candidate.token); ok {
			return true
		}
	}
	return false
}

// evalQAddChainPlan mirrors tryEvalScalarAddChain's accumulation loop over
// the cached term plans.
func (s *EvalState) evalQAddChainPlan(terms []qAddChainTermPlan) (any, bool, error) {
	var acc any
	started := false
	fold := func(value any) error {
		if !started {
			acc = value
			started = true
			return nil
		}
		if out, ok := addScalarNumericFast(acc, value); ok {
			acc = out
			return nil
		}
		var err error
		acc, err = applyDyadic('+', acc, value)
		return err
	}
	for i := 0; i < len(terms); {
		// var+dev over the same binding share one moments pass: dev is
		// sqrt(var) by definition (devValue computes exactly
		// math.Sqrt(varValue(x))), so both term values are bit-identical to
		// their independent evaluation.
		if i+1 < len(terms) && terms[i].kind == qAddChainTermReducerName && terms[i].reducerWord == "var" &&
			terms[i+1].kind == qAddChainTermReducerName && terms[i+1].reducerWord == "dev" &&
			terms[i+1].name == terms[i].name {
			if value, ok := s.lookupName(terms[i].name); ok && !isCallable(value) {
				variance, err := varValue(value)
				if err != nil {
					return nil, true, err
				}
				if err := fold(variance); err != nil {
					return nil, true, err
				}
				deviation, err := devFromVariance(variance)
				if err != nil {
					return nil, true, err
				}
				if err := fold(deviation); err != nil {
					return nil, true, err
				}
				i += 2
				continue
			}
		}
		if bundle := terms[i].monadBundle; bundle != nil {
			values, handled, err := s.evalQMonadSumBundlePlan(bundle)
			if err != nil {
				return nil, true, err
			}
			if handled {
				for _, value := range values {
					if err := fold(value); err != nil {
						return nil, true, err
					}
				}
				i += bundle.span
				continue
			}
		}
		value, err := s.evalQAddChainTerm(&terms[i])
		if err != nil {
			return nil, true, err
		}
		if err := fold(value); err != nil {
			return nil, true, err
		}
		i++
	}
	return acc, true, nil
}

func (s *EvalState) evalQAddChainTerm(plan *qAddChainTermPlan) (any, error) {
	switch plan.kind {
	case qAddChainTermCallableOver:
		out, handled, err := s.evalQCallableOverPlan(&plan.over)
		if err != nil {
			return nil, err
		}
		if handled {
			return out, nil
		}
	case qAddChainTermLiteral:
		return plan.literal, nil
	case qAddChainTermScalarIndex:
		if target, ok := s.lookupName(plan.scalarIndex.target); ok && !isCallable(target) {
			if value, handled, err := scalarApplyIndexPlanValue(plan.scalarIndex, target); err != nil || handled {
				return value, err
			}
		}
	case qAddChainTermCountName:
		if value, ok := s.lookupName(plan.name); ok {
			return count(value)
		}
	case qAddChainTermName:
		if value, ok := s.lookupName(plan.name); ok {
			return value, nil
		}
	case qAddChainTermFast:
		out, handled, err := s.evalQFastPlan(plan.fast)
		if err != nil {
			return nil, err
		}
		if handled {
			return out, nil
		}
	case qAddChainTermReducerName:
		if value, ok := s.lookupName(plan.name); ok && !isCallable(value) {
			return plan.reducer(value)
		}
	case qAddChainTermDyadicApply:
		left, err := s.evalQTermArg(&plan.dyadicLeft)
		if err != nil {
			return nil, err
		}
		right, err := s.evalQTermArg(&plan.dyadicRight)
		if err != nil {
			return nil, err
		}
		return plan.dyadicFn(left, right)
	case qAddChainTermLastScanWord:
		out, handled, err := s.evalQLastScanWordPlan(plan)
		if err != nil {
			return nil, err
		}
		if handled {
			return out, nil
		}
	case qAddChainTermLastCallableScan:
		out, handled, err := s.evalQLastCallableScanPlan(&plan.over)
		if err != nil {
			return nil, err
		}
		if handled {
			return out, nil
		}
	}
	return s.eval(plan.src)
}

// evalQCallableOverPlan is tryEvalCallableOverScalar with the application
// parse and lambda construction hoisted to plan-build time.
func (s *EvalState) evalQCallableOverPlan(plan *qCallableApplicationPlan) (any, bool, error) {
	fn, err := s.evalQCallableApplicationFn(plan)
	if err != nil {
		return nil, true, err
	}
	if !isCallable(fn) {
		return nil, false, nil
	}
	var initial any
	if plan.hasInitial {
		initial, err = s.evalQTermArg(&plan.initial)
		if err != nil {
			return nil, true, err
		}
	}
	value, err := s.evalQTermArg(&plan.value)
	if err != nil {
		return nil, true, err
	}
	out, err := s.applyOverCallable(fn, initial, value)
	if isCallableAdd(fn) {
		recordRuntimeKernelProbe("CallableOverScalar", "over-scalar/add/"+string(qRuntimeKernelOperandKind(value, nil)), err == nil, err)
	}
	return out, true, err
}

// evalQLastCallableScanPlan is tryEvalLastCallableScan with the application
// parse hoisted to plan-build time.
func (s *EvalState) evalQLastCallableScanPlan(plan *qCallableApplicationPlan) (any, bool, error) {
	fn, err := s.evalQCallableApplicationFn(plan)
	if err != nil {
		return nil, true, err
	}
	if !isCallable(fn) {
		return nil, false, nil
	}
	var initial any
	if plan.hasInitial {
		initial, err = s.evalQTermArg(&plan.initial)
		if err != nil {
			return nil, true, err
		}
	}
	value, err := s.evalQTermArg(&plan.value)
	if err != nil {
		return nil, true, err
	}
	out, err := s.applyOverCallable(fn, initial, value)
	recordRuntimeKernelProbe("CallableLastScan", "last-scan/"+string(qRuntimeKernelOperandKind(value, nil)), err == nil, err)
	return out, true, err
}

// evalQLastScanWordPlan is tryEvalLastScan with the word match hoisted to
// plan-build time.
func (s *EvalState) evalQLastScanWordPlan(plan *qAddChainTermPlan) (any, bool, error) {
	value, err := s.evalQTermArg(&plan.scanInner)
	if err != nil {
		return nil, true, err
	}
	if array, ok := value.(data.Array); ok {
		if array.Len() == 0 {
			return nil, false, nil
		}
		recordRuntimeKernelProbe("ArrayLastScan", "vector-last/"+plan.scanWord+"/"+string(array.Kind()), true, nil)
	}
	out, err := plan.reducer(value)
	return out, true, err
}

// buildQSingleTermFastPlan recognizes whole statements that are a single
// reducer-over-name term (`+/s`, `sum x`, `count g`). The runtime route is
// the same terminal s.eval reaches after its probe cascade misses for a
// plain-name argument; lookups that miss or resolve to callables fall back
// to the probe chain so semantics stay identical.
func buildQSingleTermFastPlan(src string) (qAddChainTermPlan, bool) {
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		if arg := strings.TrimSpace(src[len("count "):]); isQAssignmentName(arg) {
			return qAddChainTermPlan{kind: qAddChainTermCountName, src: src, name: arg}, true
		}
		return qAddChainTermPlan{}, false
	}
	if reducer, word, name, ok := qAddChainReducerNameTerm(src); ok {
		return qAddChainTermPlan{kind: qAddChainTermReducerName, src: src, reducer: reducer, reducerWord: word, name: name}, true
	}
	return qAddChainTermPlan{}, false
}

// evalQSingleTermFastPlan executes a whole-statement reducer-over-name
// plan. Lookup misses and callable values leave the statement unhandled so
// it keeps flowing through the per-call probe chain (identical fallback
// semantics).
func (s *EvalState) evalQSingleTermFastPlan(plan *qAddChainTermPlan) (any, bool, error) {
	switch plan.kind {
	case qAddChainTermCountName:
		if value, ok := s.lookupName(plan.name); ok {
			out, err := count(value)
			return out, true, err
		}
	case qAddChainTermReducerName:
		if value, ok := s.lookupName(plan.name); ok && !isCallable(value) {
			out, err := plan.reducer(value)
			return out, true, err
		}
	}
	return nil, false, nil
}

// qFbyFastPlan caches the fby split and aggregate parse for
// `<agg> <name> fby <name>` statements; execution funnels into the same
// evalFbyArrays core (typed kernels and probes included) per call.
type qFbyFastPlan struct {
	agg      string
	valueArg qTermArgPlan
	groupArg qTermArgPlan
}

func buildQFbyFastPlan(src string) *qFbyFastPlan {
	leftExpr, groupExpr, ok := splitTopLevelWord(src, "fby")
	if !ok {
		return nil
	}
	agg, valueExpr, err := parseFbyAggregate(strings.TrimSpace(leftExpr))
	if err != nil || agg == "" {
		return nil
	}
	valueExpr = strings.TrimSpace(valueExpr)
	groupExpr = strings.TrimSpace(groupExpr)
	if !isQAssignmentName(valueExpr) || !isQAssignmentName(groupExpr) {
		return nil
	}
	return &qFbyFastPlan{
		agg:      agg,
		valueArg: buildQTermArgPlan(valueExpr),
		groupArg: buildQTermArgPlan(groupExpr),
	}
}

func (s *EvalState) evalQFbyFastPlan(plan *qFbyFastPlan) (any, bool, error) {
	values, err := s.evalQTermArg(&plan.valueArg)
	if err != nil {
		return nil, true, err
	}
	valueArray, ok := values.(data.Array)
	if !ok {
		return nil, false, nil
	}
	groups, err := s.evalQTermArg(&plan.groupArg)
	if err != nil {
		return nil, true, err
	}
	groupArray, ok := groups.(data.Array)
	if !ok {
		return nil, false, nil
	}
	out, err := s.evalFbyArrays(plan.agg, valueArray, groupArray)
	return out, true, err
}
