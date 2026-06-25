package q

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qPipelineKind uint8

const (
	qPipelineInvalid qPipelineKind = iota
	qPipelineSumWhereMask
	qPipelineSumWhereIndex
	qPipelineSumGatherIndexes
	qPipelineSumWhereCompare
	qPipelineCountWhereCompare
	qPipelineWhereCompareIndexes
	qPipelineSumWhereModuloCompare
	qPipelineCountWhereModuloCompare
	qPipelineWhereModuloCompareIndexes
	qPipelineSumDeltas
	qPipelineSumBin
	qPipelineSumVectorExpr
	qPipelineSumDyadicMinMax
	qPipelineSumDyadicFloatMath
	qPipelineSumUnaryPrimitive
	qPipelineWhereUnaryCompareIndexes
	qPipelineSumSequenceTransform
	qPipelineCountVectorExpr
	qPipelineCountDistinct
	qPipelineCountWhereIn
	qPipelineSumMovingWindow
	qPipelineCountRunningScan
	qPipelineLastRunningScan
	qPipelineCountSequencePrimitive
	qPipelineSumRaze
	qPipelineFindIndexes
	qPipelineFindSum
	qPipelineUnaryPrimitive
	qPipelineDyadicPrimitive
	qPipelineApplyScalarIndex
	qPipelineApplyGatherIndex
	qPipelineCastEnvelopeSum
)

type qPipelinePlan struct {
	source         string
	kind           qPipelineKind
	shape          string
	shapeSpec      qPipelineShapeSpec
	stableShapeID  string
	pipelineShape  string
	operands       []qPipelineOperandPlan
	valueExpr      string
	valuePlan      qScriptBindingPlan
	indexExpr      string
	indexPlan      qScriptBindingPlan
	maskExpr       string
	maskPlan       qScriptBindingPlan
	leftExpr       string
	leftPlan       qScriptBindingPlan
	rightExpr      string
	rightPlan      qScriptBindingPlan
	compareOp      string
	comparePrefix  string
	unaryOp        string
	modExpr        string
	modPlan        qScriptBindingPlan
	modulusExpr    string
	modulusPlan    qScriptBindingPlan
	modTargetExpr  string
	modTargetPlan  qScriptBindingPlan
	reductionInput string
	reductionPlan  qScriptBindingPlan
	moduloMaskPlan *qPipelinePlan
	castTerms      []qPipelineCastTermPlan
}

type qPipelineCastTermPlan struct {
	domainExpr string
	valueExpr  string
	valuePlan  qScriptBindingPlan
	target     qCastTarget
	stringCast bool
	count      bool
}

type qPipelineOperandRole string

const (
	qPipelineOperandValue     qPipelineOperandRole = "value"
	qPipelineOperandIndex     qPipelineOperandRole = "index"
	qPipelineOperandMask      qPipelineOperandRole = "mask"
	qPipelineOperandLeft      qPipelineOperandRole = "left"
	qPipelineOperandRight     qPipelineOperandRole = "right"
	qPipelineOperandMod       qPipelineOperandRole = "mod"
	qPipelineOperandModulus   qPipelineOperandRole = "modulus"
	qPipelineOperandModTarget qPipelineOperandRole = "mod_target"
	qPipelineOperandReduction qPipelineOperandRole = "reduction"
)

type qPipelineOperandPlan struct {
	role qPipelineOperandRole
	expr string
	plan qScriptBindingPlan
}

type qPipelineOperandFingerprint struct {
	role  qPipelineOperandRole
	kind  data.Kind
	class string
}

type qPipelineBindingCacheKey struct {
	source      string
	kind        qPipelineKind
	shape       string
	fingerprint string
}

type qPipelineBoundPlan struct {
	key            qPipelineBindingCacheKey
	resultClass    qPipelineBoundResultClass
	resultKind     data.Kind
	kernel         string
	kernelShape    string
	fallbackReason string
}

type qPipelineBoundResultClass uint8

const (
	qPipelineBoundResultInvalid qPipelineBoundResultClass = iota
	qPipelineBoundResultArray
	qPipelineBoundResultScalar
	qPipelineBoundResultMovingSum
	qPipelineBoundResultArrayCount
	qPipelineBoundResultCompareStatsSum
	qPipelineBoundResultCompareIndexSum
	qPipelineBoundResultCompareCount
	qPipelineBoundResultCompareStatsCount
	qPipelineBoundResultCompareIndexCount
)

func qPipelineStoreWhereCompareBound(key qPipelineBindingCacheKey, plan *qPipelinePlan, left, right any, resultClass qPipelineBoundResultClass, resultKind data.Kind) bool {
	kernel, kernelShape, ok := qPipelineWhereCompareBoundMetadata(plan, left, right, resultClass, resultKind)
	if !ok {
		return false
	}
	qPipelineStoreBound(key, resultClass, resultKind, kernel, kernelShape, RuntimeFallbackUnsupportedType)
	return true
}

func qPipelineStoreBound(key qPipelineBindingCacheKey, resultClass qPipelineBoundResultClass, resultKind data.Kind, kernel, kernelShape, fallbackReason string) qPipelineBoundPlan {
	bound := qPipelineBoundPlan{
		key:            key,
		resultClass:    resultClass,
		resultKind:     resultKind,
		kernel:         kernel,
		kernelShape:    kernelShape,
		fallbackReason: fallbackReason,
	}
	qGlobalPipelineBindingCacheStore(bound)
	return bound
}

var qPipelinePlanInvalidShared = &qPipelinePlan{}

// qPipelinePlanRef returns the session-cached plan for src, or a shared
// invalid plan when src is not a pipeline candidate. The returned pointer
// aliases the per-state cache entry: callers execute it in place so constant
// binding-plan caches persist across session-warm calls, mirroring the
// statement-level qEvalFastPlan persistence.
func (s *EvalState) qPipelinePlanRef(src string) *qPipelinePlan {
	src = strings.TrimSpace(src)
	if src == "" {
		return qPipelinePlanInvalidShared
	}
	// Probe the per-state cache before any candidate string scanning: hot
	// session-warm evals re-visit the same subexpression sources every call,
	// and the candidate prefilter alone walks the source several times.
	// Negative results are cached too, so steady-state planning is one map hit.
	if s.pipelineCache1Plan != nil && s.pipelineCache1Src == src {
		return s.pipelineCache1Plan
	}
	if s.pipelineCache != nil {
		if plan, ok := s.pipelineCache[src]; ok {
			return plan
		}
	}
	if !qPipelinePlanCandidate(src) {
		if !s.oneShot {
			s.storeQPipelinePlan(src, qPipelinePlanInvalidShared)
		}
		return qPipelinePlanInvalidShared
	}
	if qPipelinePlanGlobalCacheable(src) {
		if plan, ok := qGlobalPipelinePlanCacheProbe(src); ok {
			cached := new(qPipelinePlan)
			*cached = plan
			s.storeQPipelinePlan(src, cached)
			return cached
		}
	}
	plan := buildQPipelinePlan(src)
	if qPipelinePlanGlobalCacheable(src) {
		qGlobalPipelinePlanCacheStore(src, plan)
	}
	cached := new(qPipelinePlan)
	*cached = plan
	s.storeQPipelinePlan(src, cached)
	return cached
}

func (s *EvalState) qPipelinePlan(src string) qPipelinePlan {
	return *s.qPipelinePlanRef(src)
}

func (s *EvalState) storeQPipelinePlan(src string, plan *qPipelinePlan) {
	if s.pipelineCache1Plan == nil {
		s.pipelineCache1Src = src
		s.pipelineCache1Plan = plan
		return
	}
	if s.pipelineCache1Src == src {
		s.pipelineCache1Plan = plan
		return
	}
	if s.pipelineCache == nil {
		s.pipelineCache = make(map[string]*qPipelinePlan, 32)
		s.pipelineCache[s.pipelineCache1Src] = s.pipelineCache1Plan
	} else if len(s.pipelineCache) >= 512 {
		s.pipelineCache = make(map[string]*qPipelinePlan, 32)
		s.pipelineCache1Src = src
		s.pipelineCache1Plan = plan
	}
	s.pipelineCache[src] = plan
}

func qPipelinePlanGlobalCacheable(src string) bool {
	return EvalSourceCacheable(src)
}

func qGlobalPipelinePlanCacheProbe(src string) (qPipelinePlan, bool) {
	qGlobalScriptPlanCacheMu.Lock()
	plan, ok := qGlobalPipelinePlanCache[src]
	if ok {
		qGlobalScriptPlanStats.PipelineHits++
	} else {
		qGlobalScriptPlanStats.PipelineMisses++
	}
	qGlobalScriptPlanCacheMu.Unlock()
	if !ok {
		return qPipelinePlan{}, false
	}
	return cloneQPipelinePlan(plan), true
}

func qGlobalPipelinePlanCacheStore(src string, plan qPipelinePlan) {
	if src == "" || plan.kind == qPipelineInvalid {
		return
	}
	qGlobalScriptPlanCacheMu.Lock()
	if _, ok := qGlobalPipelinePlanCache[src]; !ok {
		qGlobalPipelinePlanCacheOrder = append(qGlobalPipelinePlanCacheOrder, src)
	}
	qGlobalPipelinePlanCache[src] = cloneQPipelinePlan(plan)
	qGlobalScriptPlanStats.PipelineStores++
	for len(qGlobalPipelinePlanCacheOrder) > qGlobalPipelinePlanCacheLimit {
		evict := qGlobalPipelinePlanCacheOrder[0]
		qGlobalPipelinePlanCacheOrder = qGlobalPipelinePlanCacheOrder[1:]
		delete(qGlobalPipelinePlanCache, evict)
		qGlobalScriptPlanStats.PipelineEvictions++
	}
	qGlobalScriptPlanCacheMu.Unlock()
}

func (s *EvalState) rememberQPipelinePlanKnownSource(src string, plan qPipelinePlan) {
	if src == "" || plan.kind == qPipelineInvalid {
		return
	}
	if s.skipPipelineRemember {
		return
	}
	if s.pipelineCache != nil {
		if _, ok := s.pipelineCache[src]; ok {
			return
		}
	}
	cached := new(qPipelinePlan)
	*cached = plan
	s.storeQPipelinePlan(src, cached)
}

func qPipelinePlanCandidate(src string) bool {
	if strings.HasPrefix(src, "+/") {
		return true
	}
	if strings.HasPrefix(src, "sum ") && wordBoundary(src, 0, len("sum")) {
		return true
	}
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		return true
	}
	if strings.HasPrefix(src, "last ") && wordBoundary(src, 0, len("last")) {
		return true
	}
	if strings.HasPrefix(src, "where ") && wordBoundary(src, 0, len("where")) {
		return true
	}
	if qPipelineFindCandidate(src) {
		return true
	}
	if qPipelineRuntimePrimitiveCandidate(src) {
		return true
	}
	if qPipelineApplyScalarIndexCandidate(src) {
		return true
	}
	if qPipelineApplyGatherIndexCandidate(src) {
		return true
	}
	if qPipelineApplyPathIndexCandidate(src) {
		return true
	}
	if qPipelineCastEnvelopeCandidate(src) {
		return true
	}
	return false
}

func buildQPipelinePlan(src string) qPipelinePlan {
	src = strings.TrimSpace(src)
	if src == "" {
		return qPipelinePlan{}
	}
	withSource := func(plan qPipelinePlan) qPipelinePlan {
		plan.source = src
		return plan
	}
	if plan, ok := buildQPipelineRuntimePrimitivePlan(src); ok {
		return qPipelinePlanWithBindingPlans(withSource(plan))
	}
	if plan, ok := buildQPipelineApplyScalarIndexPlan(src); ok {
		return qPipelinePlanWithBindingPlans(withSource(plan))
	}
	if plan, ok := buildQPipelineApplyGatherIndexPlan(src); ok {
		return qPipelinePlanWithBindingPlans(withSource(plan))
	}
	if plan, ok := buildQPipelineApplyPathIndexPlan(src); ok {
		return qPipelinePlanWithBindingPlans(withSource(plan))
	}
	// A top-level apply-at the apply plans above declined belongs to the
	// cascade's evalApplyIndexForm claim (and the compiled route's
	// ApplyAtExpr split): `count where 0 within 0@0` splits at `@`, so the
	// fused reducer/compare plan families below must yield.
	if _, _, ok := splitTopLevelOperator(src, "@"); ok {
		return qPipelinePlan{}
	}
	if plan, ok := buildQPipelineCastEnvelopePlan(src); ok {
		return qPipelinePlanWithBindingPlans(withSource(plan))
	}
	if strings.HasPrefix(src, "+/") {
		right := strings.TrimSpace(src[2:])
		if right == "" {
			return qPipelinePlan{}
		}
		if input, ok := qPipelineRazeInput(right); ok {
			plan := qPipelineShapePlan(qPipelineSumRaze, "")
			plan.reductionInput = input
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumGatherPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineFindPlan(right, qPipelineFindSum, "vector-reduce/find-sum"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumBinPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineWhereComparePlan(right, qPipelineSumWhereCompare, "compare-to-index-sum"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if input, ok := qPipelineDeltasInput(right); ok {
			plan := qPipelineShapePlan(qPipelineSumDeltas, "")
			plan.reductionInput = input
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumMovingWindowPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumDyadicMinMaxPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumDyadicFloatMathPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumUnaryPrimitivePlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumSequenceTransformPlan(right); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if qPipelineVectorTransformExprCandidate(right) {
			plan := qPipelineShapePlan(qPipelineSumVectorExpr, "")
			plan.reductionInput = right
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		return qPipelinePlan{}
	}
	if plan, ok := buildQPipelineFindPlan(src, qPipelineFindIndexes, "find"); ok {
		return qPipelinePlanWithBindingPlans(withSource(plan))
	}
	if strings.HasPrefix(src, "sum ") && wordBoundary(src, 0, len("sum")) {
		inputExpr := strings.TrimSpace(src[len("sum "):])
		if input, ok := qPipelineDeltasInput(inputExpr); ok {
			plan := qPipelineShapePlan(qPipelineSumDeltas, "")
			plan.reductionInput = input
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumMovingWindowPlan(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumDyadicMinMaxPlan(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumDyadicFloatMathPlan(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumUnaryPrimitivePlan(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineSumSequenceTransformPlan(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if input, ok := qPipelineRazeInput(inputExpr); ok {
			plan := qPipelineShapePlan(qPipelineSumRaze, "")
			plan.reductionInput = input
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if qPipelineVectorTransformExprCandidate(inputExpr) {
			plan := qPipelineShapePlan(qPipelineSumVectorExpr, "")
			plan.reductionInput = inputExpr
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		inputExpr := strings.TrimSpace(src[len("count "):])
		if plan, ok := buildQPipelineWhereComparePlan(inputExpr, qPipelineCountWhereCompare, "compare-to-index-count"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineWhereInPlan(inputExpr, qPipelineCountWhereIn, "in-count"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if strings.HasPrefix(inputExpr, "distinct ") && wordBoundary(inputExpr, 0, len("distinct")) {
			arg := strings.TrimSpace(inputExpr[len("distinct "):])
			if arg != "" {
				return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineCountDistinct, shape: "distinct-count", reductionInput: arg}))
			}
		}
		// count group <arg> has the same cardinality as count distinct
		// <arg> over arrays (group keys are exactly the distinct values);
		// comparePrefix gates the array-only fallback in the evaluator so
		// non-array group semantics stay on the probe chain.
		if strings.HasPrefix(inputExpr, "group ") && wordBoundary(inputExpr, 0, len("group")) {
			arg := strings.TrimSpace(inputExpr[len("group "):])
			if arg != "" {
				return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineCountDistinct, shape: "distinct-count", comparePrefix: "group", reductionInput: arg}))
			}
		}
		if scan, arg, ok := qPipelineRunningScanInput(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineCountRunningScan, shape: "vector-count/" + scan, compareOp: scan, reductionInput: arg}))
		}
		if plan, ok := buildQPipelineCountSequencePlan(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if input, ok := qPipelineRazeInput(inputExpr); ok {
			return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineCountSequencePrimitive, shape: "sequence-count/raze", compareOp: "raze", reductionInput: input}))
		}
		if qPipelineMatrixReshapeExprCandidate(inputExpr) {
			return qPipelinePlanWithBindingPlans(withSource(qPipelinePlan{kind: qPipelineCountSequencePrimitive, shape: "sequence-count/value", compareOp: "value", reductionInput: inputExpr}))
		}
		if strings.HasPrefix(inputExpr, "where ") && wordBoundary(inputExpr, 0, len("where")) {
			return qPipelinePlan{}
		}
		if strings.HasPrefix(inputExpr, "reverse ") && wordBoundary(inputExpr, 0, len("reverse")) {
			return qPipelinePlan{}
		}
		if qPipelineVectorTransformExprCandidate(inputExpr) {
			plan := qPipelineShapePlan(qPipelineCountVectorExpr, "")
			plan.reductionInput = inputExpr
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "last ") && wordBoundary(src, 0, len("last")) {
		inputExpr := strings.TrimSpace(src[len("last "):])
		if scan, arg, ok := qPipelineRunningScanInput(inputExpr); ok {
			plan := qPipelineShapePlan(qPipelineLastRunningScan, scan)
			plan.compareOp = scan
			plan.reductionInput = arg
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		return qPipelinePlan{}
	}
	if strings.HasPrefix(src, "where ") && wordBoundary(src, 0, len("where")) {
		if plan, ok := buildQPipelineWhereUnaryComparePlan(src); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
		if plan, ok := buildQPipelineWhereComparePlan(src, qPipelineWhereCompareIndexes, "compare-to-index"); ok {
			return qPipelinePlanWithBindingPlans(withSource(plan))
		}
	}
	return qPipelinePlan{}
}

func qPipelineApplyScalarIndexCandidate(src string) bool {
	_, ok := buildScalarApplyIndexPlan(src)
	return ok
}

func qPipelineApplyPathIndexCandidate(src string) bool {
	_, ok := buildQPipelineApplyPathIndexPlan(src)
	return ok
}

func qPipelineApplyGatherIndexCandidate(src string) bool {
	_, ok := buildQPipelineApplyGatherIndexPlan(src)
	return ok
}

func buildQPipelineApplyScalarIndexPlan(src string) (qPipelinePlan, bool) {
	apply, ok := buildScalarApplyIndexPlan(src)
	if !ok {
		return qPipelinePlan{}, false
	}
	if !apply.scalar || len(apply.indexes) != 1 {
		return qPipelinePlan{}, false
	}
	op := "at"
	if apply.mode == qApplyIndexDot {
		op = "dot"
	}
	return qPipelinePlan{
		kind:      qPipelineApplyScalarIndex,
		shape:     "apply-index/scalar-" + op,
		compareOp: op,
		valueExpr: apply.target,
		indexExpr: fmt.Sprintf("%d", apply.index),
	}, true
}

func buildQPipelineApplyGatherIndexPlan(src string) (qPipelinePlan, bool) {
	apply, ok := buildScalarApplyIndexPlan(src)
	if !ok || apply.mode != qApplyIndexAt || apply.scalar || len(apply.indexes) == 0 {
		return qPipelinePlan{}, false
	}
	indexExpr := make([]string, 0, len(apply.indexes))
	for _, index := range apply.indexes {
		indexExpr = append(indexExpr, fmt.Sprintf("%d", index))
	}
	return qPipelinePlan{
		kind:      qPipelineApplyGatherIndex,
		shape:     "apply-index/gather-at",
		compareOp: "at",
		valueExpr: apply.target,
		indexExpr: strings.Join(indexExpr, " "),
	}, true
}

func buildQPipelineApplyPathIndexPlan(src string) (qPipelinePlan, bool) {
	apply, ok := buildScalarApplyIndexPlan(src)
	if !ok || apply.mode != qApplyIndexDot || apply.scalar || len(apply.indexes) < 2 {
		return qPipelinePlan{}, false
	}
	indexExpr := make([]string, 0, len(apply.indexes))
	for _, index := range apply.indexes {
		indexExpr = append(indexExpr, fmt.Sprintf("%d", index))
	}
	return qPipelinePlan{
		kind:      qPipelineApplyScalarIndex,
		shape:     "apply-index/path-dot",
		compareOp: "dot",
		valueExpr: apply.target,
		indexExpr: strings.Join(indexExpr, " "),
	}, true
}

func buildQPipelineSumSequenceTransformPlan(src string) (qPipelinePlan, bool) {
	transform, leftExpr, valueExpr, ok := qPipelineSequenceTransformInput(src)
	if !ok {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineSumSequenceTransform, transform)
	plan.compareOp = transform
	plan.leftExpr = leftExpr
	plan.reductionInput = valueExpr
	return plan, true
}

func qPipelineSequenceTransformInput(src string) (transform, leftExpr, valueExpr string, ok bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, spec := range []struct {
		prefix    string
		transform string
	}{
		{"reverse ", data.SequenceTransformReverse},
		{"ratios ", data.SequenceTransformRatios},
	} {
		if strings.HasPrefix(src, spec.prefix) && wordBoundary(src, 0, len(strings.TrimSpace(spec.prefix))) {
			arg := strings.TrimSpace(src[len(spec.prefix):])
			return spec.transform, "", arg, arg != ""
		}
	}
	if left, right, found := splitTopLevelWord(src, "rotate"); found {
		return data.SequenceTransformRotate, strings.TrimSpace(left), strings.TrimSpace(right), strings.TrimSpace(left) != "" && strings.TrimSpace(right) != ""
	}
	if args, found := qFunctionCallArgs(src); found && strings.TrimSpace(src[:strings.Index(src, "[")]) == "sublist" {
		if len(args) != 2 {
			return "", "", "", false
		}
		return data.SequenceTransformSublist, strings.TrimSpace(args[0]), strings.TrimSpace(args[1]), strings.TrimSpace(args[0]) != "" && strings.TrimSpace(args[1]) != ""
	}
	if left, right, found := splitTopLevelWord(src, "sublist"); found {
		return data.SequenceTransformSublist, strings.TrimSpace(left), strings.TrimSpace(right), strings.TrimSpace(left) != "" && strings.TrimSpace(right) != ""
	}
	return "", "", "", false
}

func buildQPipelineCountSequencePlan(src string) (qPipelinePlan, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if src == "" {
		return qPipelinePlan{}, false
	}
	for _, transform := range []string{"trim", "ltrim", "rtrim"} {
		prefix := transform + " "
		if strings.HasPrefix(src, prefix) && wordBoundary(src, 0, len(transform)) {
			arg := strings.TrimSpace(src[len(prefix):])
			if arg == "" {
				return qPipelinePlan{}, false
			}
			return qPipelinePlan{
				kind:           qPipelineCountSequencePrimitive,
				shape:          "sequence-count/" + transform,
				compareOp:      transform,
				reductionInput: arg,
			}, true
		}
	}
	if left, right, ok := splitTopLevelWord(src, "cross"); ok {
		return qPipelinePlan{
			kind:      qPipelineCountSequencePrimitive,
			shape:     "sequence-count/cross",
			compareOp: "cross",
			leftExpr:  strings.TrimSpace(left),
			rightExpr: strings.TrimSpace(right),
		}, true
	}
	if left, right, ok := splitTopLevelWord(src, "cut"); ok {
		return qPipelinePlan{
			kind:      qPipelineCountSequencePrimitive,
			shape:     "sequence-count/cut",
			compareOp: "cut",
			leftExpr:  strings.TrimSpace(left),
			rightExpr: strings.TrimSpace(right),
		}, true
	}
	if args, ok := qFunctionCallArgs(src); ok && strings.TrimSpace(src[:strings.Index(src, "[")]) == "cut" && len(args) == 2 {
		return qPipelinePlan{
			kind:      qPipelineCountSequencePrimitive,
			shape:     "sequence-count/cut",
			compareOp: "cut",
			leftExpr:  strings.TrimSpace(args[0]),
			rightExpr: strings.TrimSpace(args[1]),
		}, true
	}
	if left, right, ok := splitTopLevelWord(src, "sublist"); ok {
		return qPipelinePlan{
			kind:      qPipelineCountSequencePrimitive,
			shape:     "sequence-count/sublist",
			compareOp: "sublist",
			leftExpr:  strings.TrimSpace(left),
			rightExpr: strings.TrimSpace(right),
		}, true
	}
	if args, ok := qFunctionCallArgs(src); ok && strings.TrimSpace(src[:strings.Index(src, "[")]) == "sublist" && len(args) == 2 {
		return qPipelinePlan{
			kind:      qPipelineCountSequencePrimitive,
			shape:     "sequence-count/sublist",
			compareOp: "sublist",
			leftExpr:  strings.TrimSpace(args[0]),
			rightExpr: strings.TrimSpace(args[1]),
		}, true
	}
	return qPipelinePlan{}, false
}

func qPipelineRazeInput(src string) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "raze ") && wordBoundary(src, 0, len("raze")) {
		input := strings.TrimSpace(src[len("raze "):])
		return input, input != ""
	}
	// `,/x` is canonical raze (join over): same one-level flatten, so the
	// raze reduction plans apply. `,/:` (join each-right) is excluded.
	if strings.HasPrefix(src, ",/") && !strings.HasPrefix(src, ",/:") {
		input := strings.TrimSpace(src[len(",/"):])
		return input, input != ""
	}
	args, ok := qFunctionCallArgs(src)
	if !ok || strings.TrimSpace(src[:strings.Index(src, "[")]) != "raze" || len(args) != 1 {
		if !strings.HasPrefix(src, "raze[") || !strings.HasSuffix(src, "]") || !enclosed(src[len("raze"):], '[', ']') {
			return "", false
		}
		input := strings.TrimSpace(src[len("raze[") : len(src)-1])
		return input, input != ""
	}
	input := strings.TrimSpace(args[0])
	return input, input != ""
}

func qPipelineMatrixReshapeExprCandidate(src string) bool {
	parts := splitTopLevel(strings.TrimSpace(src), '#')
	if len(parts) != 2 {
		return false
	}
	left := strings.TrimSpace(parts[0])
	if left == "" {
		return false
	}
	if strings.Contains(left, " ") {
		return true
	}
	if strings.HasPrefix(left, "(") && strings.HasSuffix(left, ")") {
		return true
	}
	return false
}

func qPipelineRuntimePrimitiveCandidate(src string) bool {
	if !qPipelineRuntimePrimitiveTopLevelEligible(src) {
		return false
	}
	if _, _, ok := qPipelineRuntimePrimitivePrefix(src); ok {
		return true
	}
	if name, args, ok := qPipelineRuntimePrimitiveCall(src); ok {
		return qPipelineRuntimePrimitiveCallArity(name, len(args))
	}
	if qPipelineDyadicWordShadowedByEarlierSplit(src) {
		return false
	}
	if op, left, right, ok := splitTopLevelDyadicWordMap(src, qDyadicWordOps); ok && qPipelineRuntimePrimitiveDyadicWord(op.word) {
		return qPipelineRuntimeDyadicPrimitiveOperands(op.word, left, right)
	}
	return false
}

// qPipelineDyadicWordShadowedByEarlierSplit reports whether s.eval would
// split src at a probe that precedes its dyadic word map (apply-at, composite
// compares, match, find/roll, join, postfix symbol lookup) or at a DIFFERENT
// word: the word map splits at the LEFTMOST registered dyadic word, so a
// dyadic-primitive plan may only claim when that leftmost word is its own
// verb (`2 rotate where 0 1 1` splits at `rotate`, not `where`; `0 or wsum 0`
// at `or`, not `wsum`; `count@where 0` is an apply, not a `where` split).
func qPipelineDyadicWordShadowedByEarlierSplit(src string) bool {
	// Unary prefix-word claims (til/where/count/... plus take/drop and the
	// deferred-state prefixes) run before the cascade's dyadic word map:
	// `til wsum 0` is til (wsum 0), never a wsum split.
	if space := strings.IndexByte(src, ' '); space > 0 {
		head := src[:space]
		if qCompilePrefixWords[head] || head == "take" || head == "drop" ||
			head == "lookup" || head == "get" || head == "set" || head == "rand" || head == "hopen" {
			return true
		}
	}
	if qPipelineWordSplitShadowedByEarlierClaim(src) {
		return true
	}
	if op, _, _, ok := splitTopLevelDyadicWordMap(src, qDyadicWordOps); ok {
		if !qPipelineRuntimePrimitiveDyadicWord(op.word) {
			return true
		}
	}
	return false
}

// qPipelineWordSplitShadowedByEarlierClaim reports whether a cascade probe
// that precedes the dyadic word map (apply-at, composite compares, match,
// find, join, postfix lookup) claims src, so word-keyed plans must yield.
func qPipelineWordSplitShadowedByEarlierClaim(src string) bool {
	for _, op := range []string{"@", "<>", "<=", ">=", "~", "?"} {
		if _, _, ok := splitTopLevelOperator(src, op); ok {
			return true
		}
	}
	if _, ok := qTopLevelJoinSplit(src); ok {
		return true
	}
	if _, _, ok := findPostfixLookup(src); ok {
		return true
	}
	return false
}

func qPipelineRuntimePrimitiveDyadicWord(word string) bool {
	for _, w := range qPipelineRuntimeDyadicPrimitiveVerbs() {
		if w == word {
			return true
		}
	}
	return false
}

func buildQPipelineRuntimePrimitivePlan(src string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if !qPipelineRuntimePrimitiveTopLevelEligible(src) {
		return qPipelinePlan{}, false
	}
	if name, arg, ok := qPipelineRuntimePrimitivePrefix(src); ok {
		plan := qPipelineShapePlan(qPipelineUnaryPrimitive, name)
		plan.compareOp = name
		plan.reductionInput = arg
		return plan, true
	}
	if name, args, ok := qPipelineRuntimePrimitiveCall(src); ok && qPipelineRuntimePrimitiveCallArity(name, len(args)) {
		switch len(args) {
		case 1:
			plan := qPipelineShapePlan(qPipelineUnaryPrimitive, name)
			plan.compareOp = name
			plan.reductionInput = strings.TrimSpace(args[0])
			return plan, plan.reductionInput != ""
		case 2:
			plan := qPipelineShapePlan(qPipelineDyadicPrimitive, name)
			plan.compareOp = name
			plan.leftExpr = strings.TrimSpace(args[0])
			plan.rightExpr = strings.TrimSpace(args[1])
			return plan, plan.leftExpr != "" && plan.rightExpr != ""
		default:
			return qPipelinePlan{}, false
		}
	}
	if qPipelineDyadicWordShadowedByEarlierSplit(src) {
		return qPipelinePlan{}, false
	}
	if op, left, right, ok := splitTopLevelDyadicWordMap(src, qDyadicWordOps); ok && qPipelineRuntimePrimitiveDyadicWord(op.word) {
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		if left == "" || right == "" {
			return qPipelinePlan{}, false
		}
		if !qPipelineRuntimeDyadicPrimitiveOperands(op.word, left, right) {
			return qPipelinePlan{}, false
		}
		plan := qPipelineShapePlan(qPipelineDyadicPrimitive, op.word)
		plan.compareOp = op.word
		plan.leftExpr = left
		plan.rightExpr = right
		return plan, true
	}
	return qPipelinePlan{}, false
}

func qPipelineRuntimePrimitiveTopLevelEligible(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	for _, prefix := range []string{"+/", "sum ", "count ", "last ", "where "} {
		if strings.HasPrefix(src, prefix) {
			return false
		}
	}
	return true
}

func qPipelineRuntimePrimitivePrefix(src string) (name, arg string, ok bool) {
	for _, word := range qPipelineRuntimeUnaryPrimitiveVerbs() {
		prefix := word + " "
		if strings.HasPrefix(src, prefix) && wordBoundary(src, 0, len(word)) {
			arg = strings.TrimSpace(src[len(prefix):])
			return word, arg, qPipelineRuntimePrimitiveSimpleUnaryArg(arg)
		}
	}
	return "", "", false
}

func qPipelineRuntimePrimitiveSimpleUnaryArg(arg string) bool {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return false
	}
	if strings.Contains(arg, ";") {
		return false
	}
	// `@` apply claims before the unary prefix words in the cascade
	// (`sin @0` is (sin)@0, not sin(@0)).
	for _, op := range []string{"+", "*", "%", "@"} {
		if strings.Contains(arg, op) {
			return false
		}
	}
	for i, r := range arg {
		if r != '-' {
			continue
		}
		if i == 0 {
			continue
		}
		prev := previousNonSpaceByte(arg, i)
		if prev == 0 || strings.ContainsRune("([{;", rune(prev)) {
			continue
		}
		return false
	}
	for _, word := range qPipelineRuntimeDyadicPrimitiveVerbs() {
		if _, _, ok := splitTopLevelWord(arg, word); ok {
			return false
		}
	}
	return true
}

func previousNonSpaceByte(s string, before int) byte {
	if before > len(s) {
		before = len(s)
	}
	for i := before - 1; i >= 0; i-- {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			return s[i]
		}
	}
	return 0
}

func qPipelineRuntimePrimitiveCall(src string) (string, []string, bool) {
	args, ok := qFunctionCallArgs(src)
	if !ok {
		return "", nil, false
	}
	name := strings.TrimSpace(src[:strings.Index(src, "[")])
	if !qPipelineRuntimePrimitiveVerb(name) {
		return "", nil, false
	}
	return name, args, true
}

func qPipelineRuntimePrimitiveCallArity(name string, n int) bool {
	name = strings.TrimSpace(name)
	switch name {
	case "sqrt", "log", "exp", "sin", "cos", "tan", "asin", "acos", "atan", "reciprocal", "signum", "floor", "ceiling",
		"svar", "sdev":
		return n == 1
	case "xexp", "xlog", "mdev", "ema", "cov", "scov", "cor", "where":
		return n == 2
	case "wsum":
		return n == 1 || n == 2
	default:
		return false
	}
}

func qPipelineRuntimeUnaryPrimitiveVerbs() []string {
	return []string{
		"sqrt", "log", "exp", "sin", "cos", "tan", "asin", "acos", "atan", "reciprocal", "signum", "floor", "ceiling",
		"svar", "sdev", "wsum",
	}
}

func qPipelineRuntimeDyadicPrimitiveVerbs() []string {
	return []string{"xexp", "xlog", "mdev", "ema", "wsum", "cov", "scov", "cor", "where"}
}

func qPipelineRuntimeDyadicPrimitiveOperands(word, left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if word != "where" {
		return true
	}
	if findTopLevel(left, "+-*%") >= 0 {
		return false
	}
	for _, prefix := range []string{"count ", "sum ", "last ", "+/", "where "} {
		if strings.HasPrefix(left, prefix) {
			return false
		}
	}
	switch left {
	case "count", "sum", "last", "where":
		return false
	default:
		return true
	}
}

func qPipelineRuntimePrimitiveVerb(name string) bool {
	name = strings.TrimSpace(name)
	for _, word := range qPipelineRuntimeUnaryPrimitiveVerbs() {
		if name == word {
			return true
		}
	}
	for _, word := range qPipelineRuntimeDyadicPrimitiveVerbs() {
		if name == word {
			return true
		}
	}
	return false
}

func buildQPipelineSumMovingWindowPlan(src string) (qPipelinePlan, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	word, left, right, ok := qPipelineLeftmostWordSplit(src, []string{"msum", "mavg", "mcount", "mmin", "mmax", "mdev", "ema"}...)
	if !ok || left == "" || right == "" {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineSumMovingWindow, word)
	plan.compareOp = word
	plan.leftExpr = left
	plan.rightExpr = right
	return plan, true
}

func buildQPipelineSumDyadicMinMaxPlan(src string) (qPipelinePlan, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	word, left, right, ok := qPipelineLeftmostWordSplit(src, []string{"min", "max"}...)
	if !ok || left == "" || right == "" {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineSumDyadicMinMax, word)
	plan.compareOp = word
	plan.leftExpr = left
	plan.rightExpr = right
	return plan, true
}

func buildQPipelineSumDyadicFloatMathPlan(src string) (qPipelinePlan, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	word, left, right, ok := qPipelineLeftmostWordSplit(src, []string{data.NumericDyadicXExp, data.NumericDyadicXLog}...)
	if !ok || left == "" || right == "" {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineSumDyadicFloatMath, word)
	plan.compareOp = word
	plan.leftExpr = left
	plan.rightExpr = right
	return plan, true
}

func buildQPipelineSumUnaryPrimitivePlan(src string) (qPipelinePlan, bool) {
	op, arg, ok := splitLeadingNumericUnary(stripEnclosingParens(strings.TrimSpace(src)))
	if !ok || arg == "" {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineSumUnaryPrimitive, op)
	plan.unaryOp = op
	plan.reductionInput = arg
	return plan, true
}

func buildQPipelineWhereUnaryComparePlan(src string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "where ") || !wordBoundary(src, 0, len("where")) {
		return qPipelinePlan{}, false
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(arg)
	if !ok {
		return qPipelinePlan{}, false
	}
	dataOp, ok := qDataCompareOpString(op)
	if !ok {
		return qPipelinePlan{}, false
	}
	unaryOp, valueExpr, ok := splitLeadingNumericUnary(leftExpr)
	if ok && valueExpr != "" {
		plan := qPipelineShapePlan(qPipelineWhereUnaryCompareIndexes, unaryOp)
		plan.unaryOp = unaryOp
		plan.compareOp = op
		plan.leftExpr = strings.TrimSpace(valueExpr)
		plan.rightExpr = strings.TrimSpace(rightExpr)
		plan.comparePrefix = "numeric-unary-compare-to-index"
		return plan, dataOp != ""
	}
	unaryOp, valueExpr, ok = splitLeadingNumericUnary(rightExpr)
	if !ok || valueExpr == "" {
		return qPipelinePlan{}, false
	}
	reversed := qReverseCompareOpString(op)
	if reversed == "" {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineWhereUnaryCompareIndexes, unaryOp)
	plan.unaryOp = unaryOp
	plan.compareOp = reversed
	plan.leftExpr = strings.TrimSpace(valueExpr)
	plan.rightExpr = strings.TrimSpace(leftExpr)
	plan.comparePrefix = "numeric-unary-compare-to-index"
	return plan, true
}

func qPipelineRunningScanInput(src string) (scan, arg string, ok bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	for _, spec := range []struct {
		prefix string
		scan   string
	}{
		{"+\\", "sums"},
		{"sums ", "sums"},
		{"prds ", "prds"},
		{"mins ", "mins"},
		{"maxs ", "maxs"},
		{"avgs ", "avgs"},
	} {
		if !strings.HasPrefix(src, spec.prefix) {
			continue
		}
		arg = strings.TrimSpace(src[len(spec.prefix):])
		if arg == "" || (spec.prefix == "+\\" && strings.HasPrefix(arg, "[")) {
			return "", "", false
		}
		return spec.scan, arg, true
	}
	return "", "", false
}

func qPipelinePlanWithBindingPlans(plan qPipelinePlan) qPipelinePlan {
	return qNormalizePipelinePlan(plan)
}

func buildQPipelineBindingPlan(src string) qScriptBindingPlan {
	if expr, ok, err := parseValueExpr(src); err == nil && ok {
		if plan := buildQScriptBindingPlanForRHS(src, expr); plan.kind != qScriptBindingInvalid {
			return plan
		}
	}
	return buildQScriptBindingPlanForRHS(src, nil)
}

func qPipelineCastEnvelopeCandidate(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" || !strings.Contains(src, "$") {
		return false
	}
	if !strings.Contains(src, "+") && !strings.HasPrefix(src, "count ") && !strings.HasPrefix(src, "string ") {
		return false
	}
	_, ok := buildQPipelineCastEnvelopePlan(src)
	return ok
}

func buildQPipelineCastEnvelopePlan(src string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if src == "" || !strings.Contains(src, "$") {
		return qPipelinePlan{}, false
	}
	terms := qScriptPipelinePlusTerms(src)
	if len(terms) == 0 {
		return qPipelinePlan{}, false
	}
	castTerms := make([]qPipelineCastTermPlan, 0, len(terms))
	countTerms := 0
	for _, term := range terms {
		parsed, ok := qPipelineCastEnvelopeTerm(term)
		if !ok {
			return qPipelinePlan{}, false
		}
		if parsed.count {
			countTerms++
		}
		castTerms = append(castTerms, parsed)
	}
	if len(castTerms) == 1 && countTerms == 0 {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineCastEnvelopeSum, "")
	plan.castTerms = castTerms
	return plan, true
}

func qPipelineCastEnvelopeTerm(src string) (qPipelineCastTermPlan, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if src == "" {
		return qPipelineCastTermPlan{}, false
	}
	term := qPipelineCastTermPlan{}
	if strings.HasPrefix(src, "count ") && wordBoundary(src, 0, len("count")) {
		term.count = true
		src = stripEnclosingParens(strings.TrimSpace(src[len("count "):]))
	}
	if strings.HasPrefix(src, "string ") && wordBoundary(src, 0, len("string")) {
		term.stringCast = true
		src = stripEnclosingParens(strings.TrimSpace(src[len("string "):]))
	}
	dollar := findTopLevel(src, "$")
	if dollar < 0 {
		return qPipelineCastTermPlan{}, false
	}
	domainExpr := strings.TrimSpace(src[:dollar])
	valueExpr := strings.TrimSpace(src[dollar+1:])
	if valueExpr == "" {
		return qPipelineCastTermPlan{}, false
	}
	target, ok := qStaticCastTargetFromExpr(domainExpr)
	if !ok {
		return qPipelineCastTermPlan{}, false
	}
	term.domainExpr = domainExpr
	term.valueExpr = valueExpr
	term.target = target
	return term, true
}

func qStaticCastTargetFromExpr(src string) (qCastTarget, bool) {
	src = strings.TrimSpace(src)
	switch src {
	case "", "`":
		return qSymbolCastTarget(), true
	}
	if isBareQCastName(src) {
		if kind, ok := qCastKindFromSymbol(data.Symbol(src)); ok {
			return qCastTarget{kind: kind, sourceText: src}, true
		}
	}
	if strings.HasPrefix(src, "`") && !strings.ContainsAny(src[1:], " \t\r\n;()[]{}") {
		name := strings.TrimPrefix(src, "`")
		if kind, ok := qCastKindFromSymbol(data.Symbol(name)); ok {
			return qCastTarget{kind: kind, sourceText: src}, true
		}
	}
	if len(src) >= 2 && src[0] == '"' && src[len(src)-1] == '"' {
		text, err := strconv.Unquote(src)
		if err != nil {
			return qCastTarget{}, false
		}
		if kind, ok := qCastKindFromTypeText(text); ok {
			return qCastTarget{kind: kind, sourceText: src}, true
		}
	}
	return qCastTarget{}, false
}

func qPipelineVectorTransformExprCandidate(src string) bool {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if src == "" {
		return false
	}
	for _, prefix := range []string{
		"reverse ",
		"til ",
		"drop ",
	} {
		if strings.HasPrefix(src, prefix) && wordBoundary(src, 0, len(strings.TrimSpace(prefix))) {
			return true
		}
	}
	for _, word := range []string{"rotate", "where", "bin", "xbar"} {
		if _, _, ok := splitTopLevelWord(src, word); ok {
			return true
		}
	}
	if len(splitTopLevel(src, '#')) > 1 {
		return true
	}
	if len(splitTopLevel(src, '_')) > 1 {
		return true
	}
	if _, _, ok := findPostfixIndex(src); ok {
		return true
	}
	return false
}

func buildQPipelineSumGatherPlan(src string) (qPipelinePlan, bool) {
	if valueExpr, maskExpr, ok := splitTopLevelWord(src, "where"); ok {
		plan := qPipelineShapePlan(qPipelineSumWhereMask, "")
		plan.valueExpr = strings.TrimSpace(valueExpr)
		plan.maskExpr = strings.TrimSpace(maskExpr)
		return plan, true
	}
	collectionExpr, indexExpr, ok := findPostfixIndex(src)
	if !ok {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(qPipelineSumGatherIndexes, "")
	plan.valueExpr = strings.TrimSpace(collectionExpr)
	plan.indexExpr = strings.TrimSpace(indexExpr)
	if maskExpr, ok := directWhereMaskExpr(indexExpr); ok {
		plan = qPipelineShapePlan(qPipelineSumWhereIndex, "")
		plan.valueExpr = strings.TrimSpace(collectionExpr)
		plan.indexExpr = strings.TrimSpace(indexExpr)
		plan.maskExpr = strings.TrimSpace(maskExpr)
	}
	return plan, true
}

func buildQPipelineSumBinPlan(src string) (qPipelinePlan, bool) {
	_, leftExpr, rightExpr, ok := qPipelineLeftmostWordSplit(src, "bin")
	if !ok {
		return qPipelinePlan{}, false
	}
	return qPipelinePlan{
		kind:      qPipelineSumBin,
		shape:     "bin-reduce/sum",
		leftExpr:  leftExpr,
		rightExpr: rightExpr,
	}, true
}

// qPipelineLeftmostWordSplit returns the LEFTMOST top-level registered
// dyadic word split of src when that word is one of words. The string
// evaluator's word map splits at the leftmost registered word, so a plan
// keyed to a specific word must not claim a different (later) occurrence
// (`0 max bin 0` splits at `max`, never at `bin`).
func qPipelineLeftmostWordSplit(src string, words ...string) (string, string, string, bool) {
	// Apply-at, composite compares, match, find, join, and postfix lookups
	// all claim before the word map on every route (`0 0 xexp exp@0` nests
	// the apply, `+/0~min 0` splits at `~`), so word-keyed plans must yield.
	if qPipelineWordSplitShadowedByEarlierClaim(src) {
		return "", "", "", false
	}
	op, left, right, ok := splitTopLevelDyadicWordMap(src, qDyadicWordOps)
	if !ok {
		return "", "", "", false
	}
	for _, word := range words {
		if op.word == word {
			return op.word, strings.TrimSpace(left), strings.TrimSpace(right), true
		}
	}
	return "", "", "", false
}

func buildQPipelineWhereComparePlan(src string, kind qPipelineKind, prefix string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "where ") || !wordBoundary(src, 0, len("where")) {
		return qPipelinePlan{}, false
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(arg)
	if !ok {
		return qPipelinePlan{}, false
	}
	if op != "within" {
		if _, ok := qDataCompareOpString(op); !ok {
			return qPipelinePlan{}, false
		}
	}
	if op == "within" && (strings.TrimSpace(leftExpr) == "" || strings.TrimSpace(rightExpr) == "") {
		return qPipelinePlan{}, false
	}
	if plan, ok := buildQPipelineWhereModuloComparePlan(leftExpr, rightExpr, op, kind, prefix); ok {
		return plan, true
	}
	return qPipelinePlan{
		kind:          kind,
		shape:         prefix,
		leftExpr:      strings.TrimSpace(leftExpr),
		rightExpr:     strings.TrimSpace(rightExpr),
		compareOp:     op,
		comparePrefix: prefix,
	}, true
}

func buildQPipelineWhereInPlan(src string, kind qPipelineKind, prefix string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "where ") || !wordBoundary(src, 0, len("where")) {
		return qPipelinePlan{}, false
	}
	arg := strings.TrimSpace(src[len("where "):])
	leftExpr, rightExpr, ok := splitTopLevelWord(arg, "in")
	if !ok {
		return qPipelinePlan{}, false
	}
	leftExpr = strings.TrimSpace(leftExpr)
	rightExpr = strings.TrimSpace(rightExpr)
	if leftExpr == "" || rightExpr == "" {
		return qPipelinePlan{}, false
	}
	return qPipelinePlan{
		kind:      kind,
		shape:     prefix,
		leftExpr:  leftExpr,
		rightExpr: rightExpr,
	}, true
}

func buildQPipelineWhereModuloComparePlan(leftExpr, rightExpr, op string, kind qPipelineKind, prefix string) (qPipelinePlan, bool) {
	dataOp, ok := qDataCompareOpString(op)
	if !ok || (dataOp != data.OpEQ && dataOp != data.OpNE) {
		return qPipelinePlan{}, false
	}
	modExpr, modulusExpr, ok := splitQPipelineModExpr(leftExpr)
	targetExpr := strings.TrimSpace(rightExpr)
	compareOp := op
	if !ok {
		modExpr, modulusExpr, ok = splitQPipelineModExpr(rightExpr)
		if !ok {
			return qPipelinePlan{}, false
		}
		targetExpr = strings.TrimSpace(leftExpr)
		compareOp = qReverseCompareOpString(op)
	}
	modKind := qPipelineWhereModuloCompareIndexes
	switch kind {
	case qPipelineCountWhereCompare:
		modKind = qPipelineCountWhereModuloCompare
	case qPipelineSumWhereCompare:
		modKind = qPipelineSumWhereModuloCompare
	case qPipelineWhereCompareIndexes:
		modKind = qPipelineWhereModuloCompareIndexes
	default:
		return qPipelinePlan{}, false
	}
	return qPipelinePlan{
		kind:          modKind,
		shape:         prefix + "-mod",
		compareOp:     compareOp,
		comparePrefix: prefix + "-mod",
		modExpr:       modExpr,
		modulusExpr:   modulusExpr,
		modTargetExpr: targetExpr,
	}, true
}

func splitQPipelineModExpr(src string) (string, string, bool) {
	src = stripEnclosedParens(strings.TrimSpace(src))
	// Apply-at claims before the word map on every route (`0@mod 0` is
	// 0@(mod 0)), so the mod plan must yield.
	if _, _, ok := splitTopLevelOperator(src, "@"); ok {
		return "", "", false
	}
	word, left, right, ok := qPipelineLeftmostWordSplit(src, "mod")
	if !ok || word != "mod" {
		return "", "", false
	}
	if left == "" || right == "" {
		return "", "", false
	}
	return left, right, true
}

func qPipelineFindCandidate(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	if strings.HasPrefix(src, "+/") {
		src = strings.TrimSpace(src[2:])
	}
	left, right, ok := splitTopLevelOperator(src, "?")
	return ok && strings.TrimSpace(left) != "" && strings.TrimSpace(right) != ""
}

func buildQPipelineFindPlan(src string, kind qPipelineKind, shape string) (qPipelinePlan, bool) {
	left, right, ok := splitTopLevelOperator(strings.TrimSpace(src), "?")
	if !ok {
		return qPipelinePlan{}, false
	}
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return qPipelinePlan{}, false
	}
	plan := qPipelineShapePlan(kind, "")
	plan.shape = shape
	plan.leftExpr = left
	plan.rightExpr = right
	return plan, true
}

func qPipelineDeltasInput(src string) (string, bool) {
	src = stripEnclosingParens(strings.TrimSpace(src))
	if strings.HasPrefix(src, "deltas ") && wordBoundary(src, 0, len("deltas")) {
		input := strings.TrimSpace(src[len("deltas "):])
		if input == "" {
			return "", false
		}
		return input, true
	}
	// Each-prior subtraction is deltas-shaped: `-':x` and `(-':)[x]` reduce
	// through the same typed deltas-sum kernel, with the boxed each-prior path
	// kept as the fallback for kinds the kernel does not handle.
	input := ""
	switch {
	case strings.HasPrefix(src, "-':"):
		input = strings.TrimSpace(src[len("-':"):])
	case strings.HasPrefix(src, "(-':)"):
		input = strings.TrimSpace(src[len("(-':)"):])
		if strings.HasPrefix(input, "[") {
			if !strings.HasSuffix(input, "]") {
				return "", false
			}
			inner := strings.TrimSpace(input[1 : len(input)-1])
			if inner == "" || strings.ContainsAny(inner, ";[](){}\"") {
				return "", false
			}
			input = inner
		}
	default:
		return "", false
	}
	if input == "" {
		return "", false
	}
	return input, true
}

func (s *EvalState) evalQPipelinePlan(plan *qPipelinePlan) (any, bool, error) {
	if plan.kind == qPipelineInvalid {
		return nil, false, nil
	}
	recordRuntimeQPipelinePlanExecution(plan, "attempt", "attempt")
	var (
		out     any
		handled bool
		err     error
	)
	switch plan.kind {
	case qPipelineSumWhereMask:
		out, handled, err = s.evalQPipelineSumWhereMask(plan)
	case qPipelineSumWhereIndex:
		out, handled, err = s.evalQPipelineSumWhereIndex(plan)
	case qPipelineSumGatherIndexes:
		out, handled, err = s.evalQPipelineSumGatherIndexes(plan)
	case qPipelineSumWhereCompare:
		out, handled, err = s.evalQPipelineSumWhereCompare(plan)
	case qPipelineCountWhereCompare:
		out, handled, err = s.evalQPipelineCountWhereCompare(plan)
	case qPipelineWhereCompareIndexes:
		out, handled, err = s.evalQPipelineWhereCompareIndexes(plan)
	case qPipelineSumWhereModuloCompare:
		out, handled, err = s.evalQPipelineSumWhereModuloCompare(plan)
	case qPipelineCountWhereModuloCompare:
		out, handled, err = s.evalQPipelineCountWhereModuloCompare(plan)
	case qPipelineWhereModuloCompareIndexes:
		out, handled, err = s.evalQPipelineWhereModuloCompareIndexes(plan)
	case qPipelineSumDeltas:
		out, handled, err = s.evalQPipelineSumDeltas(plan)
	case qPipelineSumBin:
		out, handled, err = s.evalQPipelineSumBin(plan)
	case qPipelineSumVectorExpr:
		out, handled, err = s.evalQPipelineSumVectorExpr(plan)
	case qPipelineSumDyadicMinMax:
		out, handled, err = s.evalQPipelineSumDyadicMinMax(plan)
	case qPipelineSumDyadicFloatMath:
		out, handled, err = s.evalQPipelineSumDyadicFloatMath(plan)
	case qPipelineSumUnaryPrimitive:
		out, handled, err = s.evalQPipelineSumUnaryPrimitive(plan)
	case qPipelineWhereUnaryCompareIndexes:
		out, handled, err = s.evalQPipelineWhereUnaryCompareIndexes(plan)
	case qPipelineSumSequenceTransform:
		out, handled, err = s.evalQPipelineSumSequenceTransform(plan)
	case qPipelineCountVectorExpr:
		out, handled, err = s.evalQPipelineCountVectorExpr(plan)
	case qPipelineCountDistinct:
		out, handled, err = s.evalQPipelineCountDistinct(plan)
	case qPipelineCountWhereIn:
		out, handled, err = s.evalQPipelineCountWhereIn(plan)
	case qPipelineSumMovingWindow:
		out, handled, err = s.evalQPipelineSumMovingWindow(plan)
	case qPipelineCountRunningScan:
		out, handled, err = s.evalQPipelineCountRunningScan(plan)
	case qPipelineLastRunningScan:
		out, handled, err = s.evalQPipelineLastRunningScan(plan)
	case qPipelineCountSequencePrimitive:
		out, handled, err = s.evalQPipelineCountSequencePrimitive(plan)
	case qPipelineSumRaze:
		out, handled, err = s.evalQPipelineSumRaze(plan)
	case qPipelineFindIndexes:
		out, handled, err = s.evalQPipelineFindIndexes(plan)
	case qPipelineFindSum:
		out, handled, err = s.evalQPipelineFindSum(plan)
	case qPipelineUnaryPrimitive, qPipelineDyadicPrimitive:
		out, handled, err = s.evalQPipelineRuntimePrimitive(*plan)
	case qPipelineApplyScalarIndex:
		out, handled, err = s.evalQPipelineApplyScalarIndex(plan)
	case qPipelineApplyGatherIndex:
		out, handled, err = s.evalQPipelineApplyGatherIndex(plan)
	case qPipelineCastEnvelopeSum:
		out, handled, err = s.evalQPipelineCastEnvelopeSum(plan)
	default:
		recordRuntimeQPipelinePlanExecution(plan, "fallback", RuntimeFallbackPlannerUnhandled)
		return nil, false, nil
	}
	switch {
	case err != nil:
		recordRuntimeQPipelinePlanExecution(plan, "error", RuntimeFallbackPipelineError)
	case handled:
		recordRuntimeQPipelinePlanExecution(plan, "hit", "typed_pipeline")
	default:
		recordRuntimeQPipelinePlanExecution(plan, "fallback", "unsupported_runtime_shape")
	}
	return out, handled, err
}

func (s *EvalState) evalQPipelineCastEnvelopeSum(plan *qPipelinePlan) (any, bool, error) {
	if len(plan.castTerms) == 0 {
		return nil, false, nil
	}
	var total int64
	for i := range plan.castTerms {
		term := &plan.castTerms[i]
		value, err := s.evalQPipelinePlannedExpr(term.valueExpr, &term.valuePlan)
		if err != nil {
			return nil, true, err
		}
		casted, handled, err := evalQTypedCastPrimitive(term.target, value)
		if err != nil || !handled {
			return nil, handled, err
		}
		if term.stringCast {
			casted, err = stringValue(casted)
			if err != nil {
				return nil, true, err
			}
		}
		if term.count {
			counted, err := count(casted)
			if err != nil {
				return nil, true, err
			}
			n, ok := integerValue(counted)
			if !ok {
				return nil, false, nil
			}
			total += n
			continue
		}
		n, ok := integerValue(casted)
		if !ok {
			return nil, false, nil
		}
		total += n
	}
	return total, true, nil
}

func (s *EvalState) evalQPipelineApplyGatherIndex(plan *qPipelinePlan) (any, bool, error) {
	target, err := s.evalQPipelinePlannedExpr(plan.valueExpr, &plan.valuePlan)
	if err != nil {
		return nil, true, err
	}
	indexValue, err := s.evalQPipelinePlannedExpr(plan.indexExpr, &plan.indexPlan)
	if err != nil {
		return nil, true, err
	}
	indexes, scalar, err := indexInts(indexValue)
	if err == nil && (scalar || len(indexes) == 0) {
		return nil, false, nil
	}
	if err == nil {
		if array, ok := target.(data.Array); ok {
			shape := "gather-at/" + string(array.Kind()) + "/" + qRuntimeCardinalityShape(len(indexes))
			recordRuntimeKernelExecution("ArrayGatherIndex", shape, "attempt", "attempt")
			// Array.Gather panics on out-of-range rows; validate up front and
			// route out-of-range reads through the generic null-filling read
			// path (arrayReadIndexValue) so both eval routes agree.
			length := array.Len()
			inRange := true
			for _, row := range indexes {
				if row < 0 || row >= length {
					inRange = false
					break
				}
			}
			if inRange {
				out := array.Gather(indexes)
				recordRuntimeKernelExecution("ArrayGatherIndex", shape, "hit", "typed_gather_index")
				return out, true, nil
			}
			recordRuntimeKernelExecution("ArrayGatherIndex", shape, "fallback", "out_of_range_null_fill")
		}
	}
	if err != nil && scalarOrVectorIndexFallbackEligible(target) {
		// Negative or null indexes: defer to the null-filling read path.
		err = nil
	}
	if err != nil {
		return nil, true, err
	}
	out, err := s.applyOrIndexValue(qApplyIndexAt, target, indexValue)
	if err != nil {
		return nil, true, err
	}
	return out, true, nil
}

// scalarOrVectorIndexFallbackEligible reports whether a read-path index
// error from the strict indexInts parse should fall back to the generic
// null-filling read route (vectors and strings) instead of erroring.
func scalarOrVectorIndexFallbackEligible(target any) bool {
	switch target.(type) {
	case data.Array, string:
		return true
	default:
		return false
	}
}

func (s *EvalState) evalQPipelineApplyScalarIndex(plan *qPipelinePlan) (any, bool, error) {
	target, err := s.evalQPipelinePlannedExpr(plan.valueExpr, &plan.valuePlan)
	if err != nil {
		return nil, true, err
	}
	indexValue, err := s.evalQPipelinePlannedExpr(plan.indexExpr, &plan.indexPlan)
	if err != nil {
		return nil, true, err
	}
	mode := qApplyIndexAt
	if plan.compareOp == "dot" {
		mode = qApplyIndexDot
	}
	indexes, scalar, err := indexInts(indexValue)
	if err != nil {
		if !scalarOrVectorIndexFallbackEligible(target) {
			return nil, true, err
		}
		// Negative or null indexes: defer to the null-filling read path.
		out, err := s.applyOrIndexValue(mode, target, indexValue)
		if err != nil {
			return nil, true, err
		}
		return out, true, nil
	}
	if !scalar || len(indexes) != 1 {
		indexPlan := qScalarApplyIndexPlan{
			mode:    mode,
			target:  plan.valueExpr,
			indexes: indexes,
			scalar:  scalar,
		}
		if len(indexes) > 0 {
			indexPlan.index = indexes[0]
		}
		if out, handled, err := scalarApplyIndexPlanValue(indexPlan, target); err != nil || handled {
			return out, handled, err
		}
		out, err := s.applyOrIndexValue(mode, target, indexValue)
		if err != nil {
			return nil, true, err
		}
		return out, true, nil
	}
	if out, handled, err := scalarIndexValue(mode, target, indexes[0]); err != nil || handled {
		return out, handled, err
	}
	out, err := s.applyOrIndexValue(mode, target, indexValue)
	if err != nil {
		return nil, true, err
	}
	return out, true, nil
}

func (s *EvalState) evalQPipelineSumSequenceTransform(plan *qPipelinePlan) (any, bool, error) {
	var left any
	var leftOK bool
	if plan.leftExpr != "" {
		var err error
		left, err = s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
		if err != nil {
			return nil, true, err
		}
		leftOK = true
	}
	args, err := s.evalQPipelineSequenceTransformArgs(plan, left, leftOK)
	if err != nil {
		if errors.Is(err, errQSublistGenericFallback) {
			return nil, false, nil
		}
		return nil, true, err
	}
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	shape := qRuntimeKernelSequenceTransformSumShape(plan.compareOp, qRuntimeKernelOperandKind(value, nil), len(args))
	operands := []qPipelineOperandFingerprint{
		qPipelineOperandFingerprintForValue(qPipelineOperandReduction, value),
	}
	if leftOK {
		operands = append([]qPipelineOperandFingerprint{qPipelineOperandFingerprintForValue(qPipelineOperandLeft, left)}, operands...)
	}
	bindingKey := qPipelineBindingKey(plan, operands)
	if bound, ok := qGlobalPipelineBindingCacheProbe(bindingKey); ok {
		return evalQPipelineSumSequenceTransformBound(plan, bound, args, value)
	}
	bound := qPipelineStoreBound(bindingKey, qPipelineBoundResultArray, qRuntimeKernelOperandKind(value, nil), "SequenceTransformSum", shape, RuntimeFallbackUnsupportedType)
	return evalQPipelineSumSequenceTransformBound(plan, bound, args, value)
}

func evalQPipelineSumSequenceTransformBound(plan *qPipelinePlan, bound qPipelineBoundPlan, args []int, value any) (any, bool, error) {
	if qRuntimeKernelOperandKind(value, nil) != bound.resultKind {
		return nil, false, nil
	}
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "SequenceTransformSum",
		shape:          bound.kernelShape,
		fallbackReason: bound.fallbackReason,
		call: func() (any, bool, error) {
			return data.TryTypedSequenceTransformNumericSum(plan.compareOp, args, value)
		},
	})
}

func (s *EvalState) evalQPipelineSumUnaryPrimitive(plan *qPipelinePlan) (any, bool, error) {
	op := strings.TrimSpace(plan.unaryOp)
	if op == "" {
		op = strings.TrimPrefix(plan.stableShape(), "vector-reduce/sum-unary-")
	}
	if out, handled, err := s.tryEvalQPipelineSumMonadChain(plan, op); err != nil || handled {
		return out, handled, err
	}
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok || array.Len() == 0 {
		// Empty inputs keep the generic empty-sum identity (typed zero of
		// the SOURCE kind); the float-accumulating kernel must decline.
		return nil, false, nil
	}
	shape := "vector-reduce/sum-unary-" + op + "/" + string(array.Kind())
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayNumericUnarySum",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			return data.TryTypedQNumericUnarySum(op, array)
		},
	})
}

// tryEvalQPipelineSumMonadChain fuses +/op_k ... op_1 [deltas] <expr> into a
// single source pass through data.TryTypedQNumericUnaryChainSum. The chain is
// re-derived from the plan strings per call (cheap prefix splits); a kernel
// decline falls through to the staged per-stage route unchanged.
func (s *EvalState) tryEvalQPipelineSumMonadChain(plan *qPipelinePlan, op string) (any, bool, error) {
	ops := []string{op}
	rest := plan.reductionInput
	for {
		next, arg, ok := splitLeadingNumericUnary(rest)
		if !ok || arg == "" {
			break
		}
		ops = append(ops, next)
		rest = arg
	}
	deltas := false
	if strings.HasPrefix(rest, "deltas") && len(rest) > len("deltas") && isSpace(rest[len("deltas")]) {
		arg := strings.TrimSpace(rest[len("deltas"):])
		if arg != "" {
			deltas = true
			rest = arg
		}
	}
	if len(ops) < 2 && !deltas {
		return nil, false, nil
	}
	// ops were collected outermost-first; the kernel applies innermost-first.
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}
	kernel, ok := data.NewQNumericUnaryChainSumPlan(ops, deltas)
	if !ok {
		return nil, false, nil
	}
	value, err := s.eval(rest)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-reduce/chain-sum/" + strings.Join(ops, "-")
	if deltas {
		shape += "/deltas"
	}
	shape += "/" + string(array.Kind())
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayNumericUnaryChainSum",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			return data.TryTypedQNumericUnaryChainSumPlanned(kernel, array)
		},
	})
}

func (s *EvalState) evalQPipelineWhereUnaryCompareIndexes(plan *qPipelinePlan) (any, bool, error) {
	op := strings.TrimSpace(plan.unaryOp)
	if op == "" {
		op = strings.TrimPrefix(plan.stableShape(), "numeric-unary-compare-to-index/")
	}
	dataOp, ok := qDataCompareOpString(plan.compareOp)
	if !ok {
		return nil, false, nil
	}
	value, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	scalar, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	shape := "numeric-unary-compare-to-index/" + op + "/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayNumericUnaryCompareIndexes",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			return data.TryTypedQNumericUnaryCompareIndexes(op, array, dataOp, scalar)
		},
	})
}

func (s *EvalState) evalQPipelineSequenceTransformArgs(plan *qPipelinePlan, left any, leftOK bool) ([]int, error) {
	switch plan.compareOp {
	case data.SequenceTransformRotate:
		if !leftOK {
			return nil, fmt.Errorf("rotate expects an integer count")
		}
		n, ok := integerValue(left)
		if !ok || int64(int(n)) != n {
			return nil, fmt.Errorf("rotate expects an integer count")
		}
		return []int{int(n)}, nil
	case data.SequenceTransformSublist:
		if !leftOK {
			return nil, fmt.Errorf("sublist expects integer indexes")
		}
		args, err := qIntegerIndexes("sublist", left)
		if err != nil {
			return nil, err
		}
		// q's one-argument sublist clamps (no overtake); the data layer's
		// one-argument sublist step is take, so encode start/count instead.
		converted, ok := qSublistTransformArgs(args)
		if !ok {
			return nil, errQSublistGenericFallback
		}
		return converted, nil
	default:
		return nil, nil
	}
}

// errQSublistGenericFallback signals that a sublist pipeline claim must defer
// to the generic route (negative one-argument sublist takes from the back,
// which the start/count transform form cannot express).
var errQSublistGenericFallback = errors.New("sublist pipeline fallback")

func (s *EvalState) evalQPipelineSumRaze(plan *qPipelinePlan) (any, bool, error) {
	if out, handled, err := s.tryEvalQPipelineMatrixOpSumRaze(plan.reductionInput); err != nil || handled {
		return out, handled, err
	}
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	shape := "vector-reduce/sum-raze/" + string(qRuntimeKernelOperandKind(value, nil))
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayNestedRazeSum",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			// The kernel computes sum(raze value) as an all-levels scalar
			// reduction, which is only faithful when ONE raze level fully
			// flattens value. Deeper nesting (raze ((0;0 0);0) is (0;0 0;0))
			// leaves array elements whose sum BROADCASTS on the generic
			// route, so the kernel must decline.
			if array, ok := value.(data.Array); ok && array.Kind() == data.KindAny && qSumRazeLeavesNestedArrays(array) {
				return nil, false, nil
			}
			if _, isArray := value.(data.Array); !isArray {
				if _, isMatrix := value.(data.Matrix); !isMatrix {
					// Atom inputs (raze 0n is the null atom) keep the
					// cascade's sum-verb semantics, including its errors.
					return nil, false, nil
				}
			}
			return data.TryTypedNestedNumericSum(value)
		},
	})
}

func (s *EvalState) evalQPipelineFindIndexes(plan *qPipelinePlan) (any, bool, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	desc, ok := qTypedFindDescriptorFor(left, right)
	if !ok {
		return nil, false, nil
	}
	out, handled, err := evalQTypedFind(desc)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

func (s *EvalState) evalQPipelineFindSum(plan *qPipelinePlan) (any, bool, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	desc, ok := qTypedFindDescriptorFor(left, right)
	if !ok {
		return nil, false, nil
	}
	out, handled, err := evalQTypedFindSum(desc)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

// qSumRazeLeavesNestedArrays reports whether razing array ONE level still
// leaves array elements behind (any element is itself a generic list with
// array elements): those shapes broadcast under sum instead of reducing.
func qSumRazeLeavesNestedArrays(array data.Array) bool {
	// Rectangular matrix views with typed scalar cells (flip/reshape results)
	// flatten fully after ONE raze level by construction: skip the O(rows)
	// walk, which would otherwise allocate a row view per At plus shape/row
	// copies per Kind probe on every evaluation.
	if kind, ok := data.MatrixCellKind(array); ok && kind != data.KindAny && kind != data.KindNull {
		return false
	}
	for row := 0; row < array.Len(); row++ {
		item, ok := array.At(row)
		if !ok {
			return true
		}
		inner, isArray := item.(data.Array)
		if !isArray {
			continue
		}
		if kind := inner.Kind(); kind != data.KindAny && kind != data.KindNull {
			continue
		}
		for innerRow := 0; innerRow < inner.Len(); innerRow++ {
			innerItem, ok := inner.At(innerRow)
			if !ok {
				return true
			}
			if _, nested := innerItem.(data.Array); nested {
				return true
			}
		}
	}
	return false
}

func (s *EvalState) tryEvalQPipelineMatrixOpSumRaze(input string) (any, bool, error) {
	input = stripEnclosingParens(strings.TrimSpace(input))
	if args, ok := qFunctionCallArgs(input); ok && strings.TrimSpace(input[:strings.Index(input, "[")]) == "mmu" {
		if len(args) != 2 {
			return nil, true, fmt.Errorf("mmu expects 2 arguments")
		}
		left, err := s.eval(strings.TrimSpace(args[0]))
		if err != nil {
			return nil, true, err
		}
		right, err := s.eval(strings.TrimSpace(args[1]))
		if err != nil {
			return nil, true, err
		}
		leftMatrix, ok, err := qMatrixValue(left)
		if err != nil || !ok {
			return nil, ok, err
		}
		rightMatrix, ok, err := qMatrixValue(right)
		if err != nil || !ok {
			return nil, ok, err
		}
		shape := "matrix-reduce/sum-mmu/" + qMatrixIndexShape(leftMatrix, 2)
		return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
			kernel:         "MatrixMultiplySum",
			shape:          shape,
			fallbackReason: RuntimeFallbackUnsupportedType,
			call: func() (any, bool, error) {
				return data.MatrixMultiplyNumericSum(leftMatrix, rightMatrix)
			},
		})
	}
	if strings.HasPrefix(input, "inv ") && wordBoundary(input, 0, len("inv")) {
		value, err := s.eval(strings.TrimSpace(input[len("inv "):]))
		if err != nil {
			return nil, true, err
		}
		matrix, ok, err := qMatrixValue(value)
		if err != nil || !ok {
			return nil, ok, err
		}
		shape := "matrix-reduce/sum-inv/" + qMatrixIndexShape(matrix, 2)
		return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
			kernel:         "MatrixInverseSum",
			shape:          shape,
			fallbackReason: RuntimeFallbackUnsupportedType,
			call: func() (any, bool, error) {
				return data.MatrixInverseNumericSum(matrix)
			},
		})
	}
	return nil, false, nil
}

func (s *EvalState) evalQPipelineSumWhereMask(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.valueExpr, &plan.valuePlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	maskValue, err := s.evalQPipelinePlannedExpr(plan.maskExpr, &plan.maskPlan)
	if err != nil {
		return nil, true, err
	}
	mask, ok := maskValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "where-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
	out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
	return qTypedRuntimeResult("ArrayWhereReduceSum", shape, out, handled, err)
}

func (s *EvalState) evalQPipelineSumWhereIndex(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.valueExpr, &plan.valuePlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	if plan.moduloMaskPlan != nil {
		if out, handled, err := s.evalQPipelineModuloCompareValueSum(plan.moduloMaskPlan, array); err != nil || handled {
			return out, handled, err
		}
	}
	if array.Kind() == data.KindI64 && isIdentityI64RangeArray(array) {
		count, sum, handled, err := s.evalQPipelineCompareIndexStatsForMask(plan.maskExpr)
		if err != nil || handled {
			if handled {
				recordRuntimeKernelProbe("ArrayWhereCompareRangeReduceSum", "where-index-reduce/i64-range/compare-stats", handled, err)
			}
			return sum, handled, err
		}
		_ = count
	}
	maskValue, err := s.evalQPipelinePlannedExpr(plan.maskExpr, &plan.maskPlan)
	if err != nil {
		return nil, true, err
	}
	mask, ok := maskValue.(data.Array)
	if ok {
		shape := "where-index-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
		out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
		out, handled, err = qTypedRuntimeResult("ArrayWhereReduceSum", shape, out, handled, err)
		if err != nil || handled {
			return out, handled, err
		}
	}
	indexValue, err := s.evalQPipelinePlannedExpr(plan.indexExpr, &plan.indexPlan)
	if err != nil {
		return nil, true, err
	}
	indexes, ok := indexValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	return qPipelineGatherReduceSum(array, indexes)
}

func (s *EvalState) evalQPipelineCompareIndexStatsForMask(maskExpr string) (count, sum int64, handled bool, err error) {
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(strings.TrimSpace(maskExpr))
	if !ok {
		return 0, 0, false, nil
	}
	if _, ok := qDataCompareOpString(op); !ok {
		return 0, 0, false, nil
	}
	left, err := s.evalQPipelinePlannedExpr(leftExpr, nil)
	if err != nil {
		return 0, 0, true, err
	}
	right, err := s.evalQPipelinePlannedExpr(rightExpr, nil)
	if err != nil {
		return 0, 0, true, err
	}
	desc, ok, err := qTypedWhereCompareStatsDescriptor(left, right, op, "where-index", "")
	if err != nil || !ok {
		return 0, 0, ok, err
	}
	count, sum, handled, err = evalQTypedWhereCompareIndexStats(desc)
	return count, sum, handled, err
}

func isIdentityI64RangeArray(array data.Array) bool {
	if array == nil || array.Kind() != data.KindI64 {
		return false
	}
	if array.Len() == 0 {
		return true
	}
	first, ok := array.At(0)
	if !ok {
		return false
	}
	firstI, ok := integerValue(first)
	if !ok || firstI != 0 {
		return false
	}
	last, ok := array.At(array.Len() - 1)
	if !ok {
		return false
	}
	lastI, ok := integerValue(last)
	return ok && lastI == int64(array.Len()-1)
}

func (s *EvalState) evalQPipelineSumGatherIndexes(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.valueExpr, &plan.valuePlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	indexValue, err := s.evalQPipelinePlannedExpr(plan.indexExpr, &plan.indexPlan)
	if err != nil {
		return nil, true, err
	}
	indexes, ok := indexValue.(data.Array)
	if !ok {
		return nil, false, nil
	}
	return qPipelineGatherReduceSum(array, indexes)
}

func qPipelineGatherReduceSum(array, indexes data.Array) (any, bool, error) {
	if isIdentityI64RangeArray(array) {
		if view, ok := indexes.(qCompareIndexStatsView); ok {
			recordRuntimeKernelProbe("ArrayGatherReduceSum", "gather-reduce/i64-range/compare-index-view", true, nil)
			return view.sum, true, nil
		}
	}
	shape := ""
	if array.Kind() == data.KindI64 && indexes.Kind() == data.KindI64 {
		shape = "gather-reduce/i64/i64"
	} else {
		shape = "gather-reduce/" + string(array.Kind()) + "/" + string(indexes.Kind())
	}
	out, handled, err := data.TryTypedNumericSumByI64Indexes(array, indexes)
	return qTypedRuntimeResult("ArrayGatherReduceSum", shape, out, handled, err)
}

func qPipelineGatherReduceSumWithPlanStats(plan *qPipelinePlan, array, indexes data.Array) (any, bool, error) {
	recordRuntimeQPipelinePlanExecution(plan, "attempt", "attempt")
	out, handled, err := qPipelineGatherReduceSum(array, indexes)
	recordQPipelinePlanOutcome(plan, handled, err)
	return out, handled, err
}

func qPipelineWhereReduceSumWithPlanStats(plan *qPipelinePlan, array, mask data.Array) (any, bool, error) {
	recordRuntimeQPipelinePlanExecution(plan, "attempt", "attempt")
	shape := ""
	if array.Kind() == data.KindI64 && mask.Kind() == data.KindBool {
		shape = "where-reduce/i64/bool"
	} else {
		shape = "where-reduce/" + string(array.Kind()) + "/" + string(mask.Kind())
	}
	out, handled, err := data.TryTypedNumericSumWhereMask(array, mask)
	out, handled, err = qTypedRuntimeResult("ArrayWhereReduceSum", shape, out, handled, err)
	recordQPipelinePlanOutcome(plan, handled, err)
	return out, handled, err
}

func recordQPipelinePlanOutcome(plan *qPipelinePlan, handled bool, err error) {
	switch {
	case err != nil:
		recordRuntimeQPipelinePlanExecution(plan, "error", RuntimeFallbackPipelineError)
	case handled:
		recordRuntimeQPipelinePlanExecution(plan, "hit", "typed_pipeline")
	default:
		recordRuntimeQPipelinePlanExecution(plan, "fallback", "unsupported_runtime_shape")
	}
}

func recordRuntimeQPipelinePlanExecution(plan *qPipelinePlan, outcome, reasonCode string) {
	shape := "unknown"
	pipelineShape := ""
	if plan != nil {
		shape = plan.stableShapeID
		if shape == "" {
			shape = plan.stableShape()
			plan.stableShapeID = shape
		}
		pipelineShape = plan.pipelineShape
		if pipelineShape == "" {
			pipelineShape = plan.stablePipelineShape()
			plan.pipelineShape = pipelineShape
		}
	}
	recordRuntimeExecutionWithPipelineShape("q_eval_vector_runtime", "QPipelinePlan", shape, pipelineShape, "typed_data_kernel", outcome, reasonCode)
}

func (s *EvalState) evalQPipelineSumWhereCompare(plan *qPipelinePlan) (any, bool, error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return nil, true, err
	}
	bindingKey := qPipelineBindingKey(plan, []qPipelineOperandFingerprint{
		qPipelineOperandFingerprintForValue(qPipelineOperandLeft, left),
		qPipelineOperandFingerprintForValue(qPipelineOperandRight, right),
	})
	if bound, ok := qGlobalPipelineBindingCacheProbe(bindingKey); ok {
		return s.evalQPipelineSumWhereCompareBound(plan, bound, left, right)
	}
	_, sum, handled, err := s.evalQPipelineWhereCompareIndexStatsForOperands(plan, left, right)
	if err != nil || handled {
		if handled {
			qPipelineStoreWhereCompareBound(bindingKey, plan, left, right, qPipelineBoundResultCompareStatsSum, "")
		}
		return sum, handled, err
	}
	indexes, handled, err := s.evalQPipelineWhereCompareIndexesArrayForOperands(plan, left, right)
	if err != nil || !handled {
		return nil, handled, err
	}
	qPipelineStoreWhereCompareBound(bindingKey, plan, left, right, qPipelineBoundResultCompareIndexSum, indexes.Kind())
	return qPipelineWhereCompareIndexesSum(indexes)
}

func (s *EvalState) evalQPipelineCountWhereCompare(plan *qPipelinePlan) (any, bool, error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return nil, true, err
	}
	bindingKey := qPipelineBindingKey(plan, []qPipelineOperandFingerprint{
		qPipelineOperandFingerprintForValue(qPipelineOperandLeft, left),
		qPipelineOperandFingerprintForValue(qPipelineOperandRight, right),
	})
	if bound, ok := qGlobalPipelineBindingCacheProbe(bindingKey); ok {
		return s.evalQPipelineCountWhereCompareBound(plan, bound, left, right)
	}
	count, handled, err := s.evalQPipelineWhereCompareCountForOperands(plan, left, right)
	if err != nil || handled {
		if handled {
			qPipelineStoreWhereCompareBound(bindingKey, plan, left, right, qPipelineBoundResultCompareCount, "")
		}
		return count, handled, err
	}
	count, _, handled, err = s.evalQPipelineWhereCompareIndexStatsForOperands(plan, left, right)
	if err != nil || handled {
		if handled {
			qPipelineStoreWhereCompareBound(bindingKey, plan, left, right, qPipelineBoundResultCompareStatsCount, "")
		}
		return count, handled, err
	}
	indexes, handled, err := s.evalQPipelineWhereCompareIndexesArrayForOperands(plan, left, right)
	if err != nil || !handled {
		return nil, handled, err
	}
	qPipelineStoreWhereCompareBound(bindingKey, plan, left, right, qPipelineBoundResultCompareIndexCount, indexes.Kind())
	return int64(indexes.Len()), true, nil
}

func (s *EvalState) evalQPipelineSumWhereCompareBound(plan *qPipelinePlan, bound qPipelineBoundPlan, left, right any) (any, bool, error) {
	switch bound.resultClass {
	case qPipelineBoundResultCompareStatsSum:
		_, sum, handled, err := s.evalQPipelineWhereCompareIndexStatsForOperands(plan, left, right)
		return sum, handled, err
	case qPipelineBoundResultCompareIndexSum:
		indexes, handled, err := s.evalQPipelineWhereCompareIndexesArrayForOperands(plan, left, right)
		if err != nil || !handled {
			return nil, handled, err
		}
		if indexes.Kind() != bound.resultKind {
			return nil, false, nil
		}
		return qPipelineWhereCompareIndexesSum(indexes)
	default:
		return nil, false, nil
	}
}

func (s *EvalState) evalQPipelineCountWhereCompareBound(plan *qPipelinePlan, bound qPipelineBoundPlan, left, right any) (any, bool, error) {
	switch bound.resultClass {
	case qPipelineBoundResultCompareCount:
		count, handled, err := s.evalQPipelineWhereCompareCountForOperands(plan, left, right)
		return count, handled, err
	case qPipelineBoundResultCompareStatsCount:
		count, _, handled, err := s.evalQPipelineWhereCompareIndexStatsForOperands(plan, left, right)
		return count, handled, err
	case qPipelineBoundResultCompareIndexCount:
		indexes, handled, err := s.evalQPipelineWhereCompareIndexesArrayForOperands(plan, left, right)
		if err != nil || !handled {
			return nil, handled, err
		}
		if indexes.Kind() != bound.resultKind {
			return nil, false, nil
		}
		return int64(indexes.Len()), true, nil
	default:
		return nil, false, nil
	}
}

func qPipelineWhereCompareIndexesSum(indexes data.Array) (any, bool, error) {
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayWhereCompareSum",
		shape:  "index-sum/" + string(indexes.Kind()),
		call: func() (any, bool, error) {
			return data.TryTypedNumericSum(indexes)
		},
	})
}

func (s *EvalState) evalQPipelineWhereCompareIndexes(plan *qPipelinePlan) (any, bool, error) {
	return s.evalQPipelineWhereCompareIndexesArray(plan)
}

func (s *EvalState) evalQPipelineSumWhereModuloCompare(plan *qPipelinePlan) (any, bool, error) {
	_, sum, handled, err := s.evalQPipelineWhereModuloCompareIndexStats(plan)
	if err != nil || handled {
		return sum, handled, err
	}
	return nil, false, nil
}

func (s *EvalState) evalQPipelineCountWhereModuloCompare(plan *qPipelinePlan) (any, bool, error) {
	count, _, handled, err := s.evalQPipelineWhereModuloCompareIndexStats(plan)
	if err != nil || handled {
		return count, handled, err
	}
	return nil, false, nil
}

func (s *EvalState) evalQPipelineWhereModuloCompareIndexes(plan *qPipelinePlan) (any, bool, error) {
	array, modulus, target, dataOp, handled, err := s.evalQPipelineModuloCompareOperands(plan)
	if err != nil || !handled {
		return nil, handled, err
	}
	shape := plan.comparePrefix + "/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel: "ArrayModuloCompare",
		shape:  shape,
		call: func() (data.Array, bool, error) {
			return data.TryTypedModuloCompareIndexesI64(array, modulus, dataOp, target)
		},
	})
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func (s *EvalState) evalQPipelineWhereModuloCompareIndexStats(plan *qPipelinePlan) (count, sum int64, handled bool, err error) {
	array, modulus, target, dataOp, handled, err := s.evalQPipelineModuloCompareOperands(plan)
	if err != nil || !handled {
		return 0, 0, handled, err
	}
	shape := plan.comparePrefix + "-stats/" + plan.compareOp + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	count, sum, handled, err = data.TryTypedModuloCompareIndexStatsI64(array, modulus, dataOp, target)
	count, sum, handled, err = qTypedRuntimeResult2Reason("ArrayModuloCompareStats", shape, RuntimeFallbackUnsupportedType, count, sum, handled, err)
	if err != nil || !handled {
		return 0, 0, handled, err
	}
	return count, sum, true, nil
}

func (s *EvalState) evalQPipelineModuloCompareValueSum(plan *qPipelinePlan, values data.Array) (any, bool, error) {
	array, modulus, target, dataOp, handled, err := s.evalQPipelineModuloCompareOperands(plan)
	if err != nil || !handled {
		return nil, handled, err
	}
	shape := "where-mod-reduce/" + string(values.Kind()) + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(modulus, nil)) + "/" + string(qRuntimeKernelOperandKind(target, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayModuloCompareReduceSum",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedNumericSumWhereModuloCompare(values, array, modulus, dataOp, target)
		},
	})
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func (s *EvalState) evalQPipelineModuloCompareOperands(plan *qPipelinePlan) (data.Array, any, any, data.Op, bool, error) {
	source, err := s.evalQPipelinePlannedExpr(plan.modExpr, &plan.modPlan)
	if err != nil {
		return nil, nil, nil, "", true, err
	}
	array, ok := source.(data.Array)
	if !ok {
		return nil, nil, nil, "", false, nil
	}
	modulus, err := s.evalQPipelinePlannedExpr(plan.modulusExpr, &plan.modulusPlan)
	if err != nil {
		return nil, nil, nil, "", true, err
	}
	target, err := s.evalQPipelinePlannedExpr(plan.modTargetExpr, &plan.modTargetPlan)
	if err != nil {
		return nil, nil, nil, "", true, err
	}
	dataOp, ok := qDataCompareOpString(plan.compareOp)
	if !ok {
		return nil, nil, nil, "", false, nil
	}
	return array, modulus, target, dataOp, true, nil
}

func qPipelineModuloComparePlanFromMask(maskExpr string) (qPipelinePlan, bool) {
	src := "where " + strings.TrimSpace(maskExpr)
	plan, ok := buildQPipelineWhereModuloComparePlanFromWhere(src, qPipelineWhereCompareIndexes, "compare-to-index")
	if !ok {
		return qPipelinePlan{}, false
	}
	return qPipelinePlanWithBindingPlans(plan), true
}

func buildQPipelineWhereModuloComparePlanFromWhere(src string, kind qPipelineKind, prefix string) (qPipelinePlan, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "where ") || !wordBoundary(src, 0, len("where")) {
		return qPipelinePlan{}, false
	}
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(strings.TrimSpace(src[len("where "):]))
	if !ok {
		return qPipelinePlan{}, false
	}
	return buildQPipelineWhereModuloComparePlan(leftExpr, rightExpr, op, kind, prefix)
}

func (s *EvalState) evalQPipelineWhereCompareIndexStats(plan *qPipelinePlan) (count, sum int64, handled bool, err error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return 0, 0, true, err
	}
	return s.evalQPipelineWhereCompareIndexStatsForOperands(plan, left, right)
}

func (s *EvalState) evalQPipelineWhereCompareIndexStatsForOperands(plan *qPipelinePlan, left, right any) (count, sum int64, handled bool, err error) {
	withinPrefix := "within-to-index-" + strings.TrimPrefix(plan.comparePrefix, "compare-to-index-") + "-stats"
	desc, ok, err := qTypedWhereCompareStatsDescriptor(left, right, plan.compareOp, plan.comparePrefix, withinPrefix)
	if err != nil || !ok {
		return 0, 0, ok, err
	}
	count, sum, handled, err = evalQTypedWhereCompareIndexStats(desc)
	if err != nil || !handled {
		return 0, 0, handled, err
	}
	return count, sum, true, nil
}

func (s *EvalState) evalQPipelineWhereCompareCount(plan *qPipelinePlan) (count int64, handled bool, err error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return 0, true, err
	}
	return s.evalQPipelineWhereCompareCountForOperands(plan, left, right)
}

func (s *EvalState) evalQPipelineWhereCompareCountForOperands(plan *qPipelinePlan, left, right any) (count int64, handled bool, err error) {
	desc, ok, err := qTypedWhereCompareCountDescriptor(left, right, plan.compareOp, "compare-count", "within-count")
	if err != nil || !ok {
		return 0, ok, err
	}
	count, handled, err = evalQTypedWhereCompareCount(desc)
	if err != nil || !handled {
		return 0, handled, err
	}
	return count, true, nil
}

func (s *EvalState) evalQPipelineWhereCompareIndexesArray(plan *qPipelinePlan) (data.Array, bool, error) {
	left, right, err := s.evalQPipelineCompareOperands(plan)
	if err != nil {
		return nil, true, err
	}
	return s.evalQPipelineWhereCompareIndexesArrayForOperands(plan, left, right)
}

func (s *EvalState) evalQPipelineWhereCompareIndexesArrayForOperands(plan *qPipelinePlan, left, right any) (data.Array, bool, error) {
	desc, ok, err := qTypedWhereCompareIndexesDescriptor(left, right, plan.compareOp, plan.comparePrefix, "within-to-index")
	if err != nil || !ok {
		return nil, ok, err
	}
	out, handled, err := evalQTypedWhereCompareIndexes(desc)
	if err != nil || !handled {
		return nil, handled, err
	}
	return out, true, nil
}

func qPipelineWhereCompareStatsShape(plan *qPipelinePlan, left, right any) string {
	_, shape, ok := qPipelineWhereCompareBoundMetadata(plan, left, right, qPipelineBoundResultCompareStatsSum, "")
	if !ok {
		return ""
	}
	return shape
}

func qPipelineWhereCompareCountShape(plan *qPipelinePlan, left, right any) string {
	_, shape, ok := qPipelineWhereCompareBoundMetadata(plan, left, right, qPipelineBoundResultCompareCount, "")
	if !ok {
		return ""
	}
	return shape
}

func qPipelineWhereCompareIndexShape(plan *qPipelinePlan, left, right any) string {
	_, shape, ok := qPipelineWhereCompareBoundMetadata(plan, left, right, qPipelineBoundResultCompareIndexCount, "")
	if !ok {
		return ""
	}
	return shape
}

func qPipelineWhereCompareBoundMetadata(plan *qPipelinePlan, left, right any, resultClass qPipelineBoundResultClass, resultKind data.Kind) (string, string, bool) {
	if plan == nil {
		return "", "", false
	}
	switch resultClass {
	case qPipelineBoundResultCompareStatsSum, qPipelineBoundResultCompareStatsCount:
		withinPrefix := "within-to-index-" + strings.TrimPrefix(plan.comparePrefix, "compare-to-index-") + "-stats"
		desc, ok, err := qTypedWhereCompareStatsDescriptor(left, right, plan.compareOp, plan.comparePrefix, withinPrefix)
		if err != nil || !ok {
			return "", "", false
		}
		return desc.kernel, desc.shape, true
	case qPipelineBoundResultCompareCount:
		desc, ok, err := qTypedWhereCompareCountDescriptor(left, right, plan.compareOp, "compare-count", "within-count")
		if err != nil || !ok {
			return "", "", false
		}
		return desc.kernel, desc.shape, true
	case qPipelineBoundResultCompareIndexCount:
		desc, ok, err := qTypedWhereCompareIndexesDescriptor(left, right, plan.compareOp, plan.comparePrefix, "within-to-index")
		if err != nil || !ok {
			return "", "", false
		}
		return desc.kernel, desc.shape, true
	case qPipelineBoundResultCompareIndexSum:
		if resultKind == "" {
			return "", "", false
		}
		return "ArrayWhereCompareSum", "index-sum/" + string(resultKind), true
	default:
		return "", "", false
	}
}

func (s *EvalState) evalQPipelineCompareOperands(plan *qPipelinePlan) (any, any, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, nil, err
	}
	right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, nil, err
	}
	return left, right, nil
}

func (s *EvalState) evalQPipelineSumDeltas(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		delta, err := deltas(value)
		if err != nil {
			return nil, true, err
		}
		out, err := sum(delta)
		return out, true, err
	}
	shape := "vector-reduce/sum-deltas/" + string(array.Kind())
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayDeltasSum",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedDeltasSum(array)
		},
	})
}

func (s *EvalState) evalQPipelineSumBin(plan *qPipelinePlan) (any, bool, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	domain, ok := left.(data.Array)
	if !ok {
		return nil, false, nil
	}
	query, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	shape := "bin-reduce/sum/" + string(domain.Kind()) + "/" + string(qRuntimeKernelOperandKind(query, nil))
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayBinReduceSum",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedBinSum(domain, query)
		},
	})
}

func (s *EvalState) evalQPipelineSumVectorExpr(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	bindingKey := qPipelineBindingKey(plan, []qPipelineOperandFingerprint{
		qPipelineOperandFingerprintForValue(qPipelineOperandReduction, value),
	})
	if bound, ok := qGlobalPipelineBindingCacheProbe(bindingKey); ok {
		return s.evalQPipelineSumVectorExprBound(bound, value)
	}
	if _, ok := numeric(value); ok {
		qPipelineStoreBound(bindingKey, qPipelineBoundResultScalar, qRuntimeKernelOperandKind(value, nil), "", "", "")
		return value, true, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-reduce/sum-expr/" + string(array.Kind())
	bound := qPipelineStoreBound(bindingKey, qPipelineBoundResultArray, array.Kind(), "ArraySumExpr", shape, RuntimeFallbackUnsupportedType)
	return s.evalQPipelineSumVectorExprBound(bound, value)
}

func (s *EvalState) evalQPipelineSumVectorExprBound(bound qPipelineBoundPlan, value any) (any, bool, error) {
	if bound.resultClass == qPipelineBoundResultScalar {
		if _, ok := numeric(value); !ok {
			return nil, false, nil
		}
		return value, true, nil
	}
	array, ok := value.(data.Array)
	if !ok || array.Kind() != bound.resultKind {
		return nil, false, nil
	}
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArraySumExpr",
		shape:          bound.kernelShape,
		fallbackReason: bound.fallbackReason,
		call: func() (any, bool, error) {
			return data.TryTypedNumericSum(array)
		},
	})
}

func (s *EvalState) evalQPipelineSumMovingWindow(plan *qPipelinePlan) (any, bool, error) {
	leftValue, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	width := int64(0)
	alpha := 0.0
	if plan.compareOp == "ema" {
		var ok bool
		alpha, ok = numeric(leftValue)
		if !ok {
			return nil, true, fmt.Errorf("ema alpha must be numeric")
		}
		if alpha < 0 || alpha > 1 {
			return nil, true, fmt.Errorf("ema alpha must be in range 0..1")
		}
	} else {
		var ok bool
		width, ok = integerValue(leftValue)
		if !ok || width <= 0 || int64(int(width)) != width {
			return nil, true, errMovingWindowWidth(plan.compareOp)
		}
	}
	if (plan.compareOp == "msum" || plan.compareOp == "mavg") && width > 0 {
		if fillPlan := buildQFillsFillPlan(plan.rightExpr); fillPlan != nil {
			source, err := s.evalQPipelinePlannedExpr(fillPlan.source, nil)
			if err != nil {
				return nil, true, err
			}
			if array, ok := source.(data.Array); ok {
				out, handled, err := data.TryTypedMovingFillsScalarFillSum(array, fillPlan.fill, int(width), plan.compareOp == "mavg")
				out, handled, err = qTypedRuntimeResult("ArrayMovingFillsFillSum", "vector-reduce/sum-"+plan.compareOp+"-fills-fill/"+string(array.Kind()), out, handled, err)
				if err != nil || handled {
					return out, handled, err
				}
			}
		}
	}
	value, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-reduce/sum-" + plan.compareOp + "/" + string(array.Kind())
	bound := qPipelineBoundPlan{
		resultClass:    qPipelineBoundResultMovingSum,
		resultKind:     array.Kind(),
		kernel:         "ArrayMovingWindowSum",
		kernelShape:    shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
	}
	return evalQPipelineSumMovingWindowBound(plan, bound, array, int(width), alpha)
}

func evalQPipelineSumMovingWindowBound(plan *qPipelinePlan, bound qPipelineBoundPlan, array data.Array, width int, alpha float64) (any, bool, error) {
	if bound.resultClass != qPipelineBoundResultMovingSum || array.Kind() != bound.resultKind {
		return nil, false, nil
	}
	var (
		out     any
		handled bool
		err     error
	)
	switch plan.compareOp {
	case "mcount":
		out, handled, err = data.TryTypedMCountSum(array, width)
	case "msum":
		out, handled, err = data.TryTypedMovingNumericSumSum(array, width, false)
	case "mavg":
		out, handled, err = data.TryTypedMovingNumericSumSum(array, width, true)
	case "mmin":
		out, handled, err = data.TryTypedMovingMinMaxSum(array, width, false)
	case "mmax":
		out, handled, err = data.TryTypedMovingMinMaxSum(array, width, true)
	case "mdev":
		out, handled, err = data.NumericMovingStdDevSum(array, width, false)
	case "ema":
		out, handled, err = data.NumericExponentialMovingAverageSum(array, alpha)
	default:
		return nil, false, nil
	}
	shape := bound.kernelShape
	if shape == "" {
		shape = "vector-reduce/sum-" + plan.compareOp + "/" + string(array.Kind())
	}
	recordQTypedRuntimeKernel("ArrayMovingWindowSum", shape, handled, err)
	return out, handled, err
}

func errMovingWindowWidth(name string) error {
	return fmt.Errorf("%s width must be a positive integer", name)
}

func (s *EvalState) evalQPipelineSumDyadicMinMax(plan *qPipelinePlan) (any, bool, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	if qPipelineFusedSumEmptyOperand(left, right) {
		// Empty operands collapse to the canonical empty-sum identity on the
		// generic route (sum of an empty broadcast is the typed zero); the
		// float-accumulating fused kernel must decline.
		return nil, false, nil
	}
	if data.IsNull(left) || data.IsNull(right) {
		// Null scalar operands keep the generic min/max null rules (the null
		// yields to the other operand); the fused kernel propagates nulls and
		// would sum a different vector.
		return nil, false, nil
	}
	wantMax := plan.compareOp == "max"
	shape := "vector-reduce/sum-dyadic-" + plan.compareOp + "/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	out, handled, err := data.TryTypedDyadicMinMaxSum(left, right, wantMax)
	return qTypedRuntimeResult("ArrayDyadicMinMaxSum", shape, out, handled, err)
}

// qPipelineFusedSumEmptyOperand reports whether either fused-sum operand is
// an empty array (the generic route's empty-identity shapes).
func qPipelineFusedSumEmptyOperand(left, right any) bool {
	if la, ok := left.(data.Array); ok && la.Len() == 0 {
		return true
	}
	if ra, ok := right.(data.Array); ok && ra.Len() == 0 {
		return true
	}
	return false
}

func (s *EvalState) evalQPipelineSumDyadicFloatMath(plan *qPipelinePlan) (any, bool, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	if qPipelineFusedSumEmptyOperand(left, right) {
		return nil, false, nil
	}
	shape := qRuntimeKernelDyadicFloatSumShape(plan.compareOp, qRuntimeKernelOperandKind(left, nil), qRuntimeKernelOperandKind(right, nil))
	out, handled, err := data.TryTypedQNumericDyadicFloatSum(plan.compareOp, left, right)
	return qTypedRuntimeResult("ArrayNumericDyadicFloatSum", shape, out, handled, err)
}

func (s *EvalState) evalQPipelineCountRunningScan(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	shape := "vector-count/" + plan.compareOp + "/" + string(array.Kind())
	kernel := "ArrayCountRunningScan"
	if array.Len() == 0 {
		recordQTypedRuntimeKernel(kernel, shape, true, nil)
		return int64(0), true, nil
	}
	switch plan.compareOp {
	case "sums", "prds", "avgs":
		if !qKindIsNumeric(array.Kind()) {
			err := fmt.Errorf("%s expects a numeric vector", plan.compareOp)
			recordQTypedRuntimeKernel(kernel, shape, false, err)
			return nil, true, err
		}
	case "mins", "maxs":
		if !qTypedCompareKindOK(array.Kind()) {
			err := fmt.Errorf("%s expects an ordered vector", plan.compareOp)
			recordQTypedRuntimeKernel(kernel, shape, false, err)
			return nil, true, err
		}
	default:
		return nil, false, nil
	}
	switch plan.compareOp {
	case "sums":
		kernel = "ArrayCountSums"
	case "prds":
		kernel = "ArrayCountProducts"
	case "mins":
		kernel = "ArrayCountMins"
	case "maxs":
		kernel = "ArrayCountMaxs"
	case "avgs":
		kernel = "ArrayCountAvgs"
	}
	recordQTypedRuntimeKernel(kernel, shape, true, nil)
	return int64(array.Len()), true, nil
}

func (s *EvalState) evalQPipelineLastRunningScan(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	if array, ok := value.(data.Array); ok {
		if array.Len() == 0 {
			return nil, false, nil
		}
		if err := qValidateRunningScanKind(plan.compareOp, array.Kind()); err != nil {
			recordRuntimeKernelProbe("ArrayLastScan", "vector-last/"+plan.compareOp+"/"+string(array.Kind()), false, err)
			return nil, true, err
		}
		recordRuntimeKernelProbe("ArrayLastScan", "vector-last/"+plan.compareOp+"/"+string(array.Kind()), true, nil)
	}
	out, err := qLastRunningScanValue(plan.compareOp, value)
	if err != nil {
		return nil, true, err
	}
	return out, true, nil
}

func qValidateRunningScanKind(scan string, kind data.Kind) error {
	switch scan {
	case "sums", "prds", "avgs":
		if !qKindIsNumeric(kind) {
			return fmt.Errorf("%s expects a numeric vector", scan)
		}
	case "mins", "maxs":
		if !qTypedCompareKindOK(kind) {
			return fmt.Errorf("%s expects an ordered vector", scan)
		}
	}
	return nil
}

func qLastRunningScanValue(scan string, value any) (any, error) {
	switch scan {
	case "sums":
		return sum(value)
	case "prds":
		return prd(value)
	case "mins":
		return minValue(value)
	case "maxs":
		return maxValue(value)
	case "avgs":
		return avg(value)
	default:
		return nil, fmt.Errorf("unsupported running scan %q", scan)
	}
}

func (s *EvalState) evalQPipelineCountVectorExpr(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, false, nil
	}
	bindingKey := qPipelineBindingKey(plan, []qPipelineOperandFingerprint{
		qPipelineOperandFingerprintForValue(qPipelineOperandReduction, value),
	})
	if bound, ok := qGlobalPipelineBindingCacheProbe(bindingKey); ok {
		return evalQPipelineCountVectorExprBound(bound, array)
	}
	bound := qPipelineStoreBound(bindingKey, qPipelineBoundResultArrayCount, array.Kind(), "ArrayCountExpr", "vector-count/expr/"+string(array.Kind()), "")
	return evalQPipelineCountVectorExprBound(bound, array)
}

func evalQPipelineCountVectorExprBound(bound qPipelineBoundPlan, array data.Array) (any, bool, error) {
	if bound.resultClass != qPipelineBoundResultArrayCount || array.Kind() != bound.resultKind {
		return nil, false, nil
	}
	shape := bound.kernelShape
	if shape == "" {
		shape = "vector-count/expr/" + string(array.Kind())
	}
	recordQTypedRuntimeKernel("ArrayCountExpr", shape, true, nil)
	return int64(array.Len()), true, nil
}

func (s *EvalState) evalQPipelineCountSequencePrimitive(plan *qPipelinePlan) (any, bool, error) {
	switch plan.compareOp {
	case "value":
		value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
		if err != nil {
			return nil, true, err
		}
		shape := "value-count/" + string(qRuntimeKernelOperandKind(value, nil))
		recordRuntimeKernelProbe("SequenceValueCount", shape, true, nil)
		return data.SequenceCount(value), true, nil
	case "raze":
		value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
		if err != nil {
			return nil, true, err
		}
		out, handled, err := data.RazeCount(value)
		shape := "raze-count/" + string(qRuntimeKernelOperandKind(value, nil))
		recordRuntimeKernelProbe("SequenceRazeCount", shape, handled && err == nil, err)
		if err != nil || handled {
			return out, handled, err
		}
		return nil, false, nil
	case "trim", "ltrim", "rtrim":
		out, handled, err := s.tryEvalCountStringTransform(plan.compareOp + " " + plan.reductionInput)
		return out, handled, err
	case "cross":
		left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
		if err != nil {
			return nil, true, err
		}
		right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
		if err != nil {
			return nil, true, err
		}
		out := data.CrossCount(left, right)
		shape := "cross-count/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(qRuntimeKernelOperandKind(right, nil))
		recordRuntimeKernelProbe("SequenceCrossCount", shape, true, nil)
		return out, true, nil
	case "cut":
		left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
		if err != nil {
			return nil, true, err
		}
		right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
		if err != nil {
			return nil, true, err
		}
		indexes, atom, err := qAtomCutStarts(left, right)
		if err != nil {
			return nil, true, err
		}
		if !atom {
			indexes, err = qIntegerIndexes("cut", left)
			if err != nil {
				return nil, true, err
			}
		}
		out, err := data.CutCount(indexes, right)
		shape := "cut-count/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(qRuntimeKernelOperandKind(right, nil))
		recordRuntimeKernelProbe("SequenceCutCount", shape, err == nil, err)
		if err != nil {
			return nil, true, err
		}
		return out, true, nil
	case "sublist":
		left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
		if err != nil {
			return nil, true, err
		}
		args, err := qIntegerIndexes("sublist", left)
		if err != nil {
			return nil, true, err
		}
		right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
		if err != nil {
			return nil, true, err
		}
		var out int64
		switch len(args) {
		case 1:
			out = qSublistTakeCount(args[0], right)
		case 2:
			out, err = data.SublistCount(args[0], args[1], right)
		default:
			err = fmt.Errorf("sublist expects count or start count")
		}
		shape := "sublist-count/" + string(qRuntimeKernelOperandKind(left, nil)) + "/" + string(qRuntimeKernelOperandKind(right, nil))
		recordRuntimeKernelProbe("SequenceSublistCount", shape, err == nil, err)
		if err != nil {
			return nil, true, err
		}
		return out, true, nil
	default:
		return nil, false, nil
	}
}

func (s *EvalState) evalQPipelineCountDistinct(plan *qPipelinePlan) (any, bool, error) {
	value, err := s.evalQPipelinePlannedExpr(plan.reductionInput, &plan.reductionPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := value.(data.Array)
	if !ok {
		if plan.comparePrefix == "group" {
			// Keep non-array `count group` on the generic probe chain;
			// only the array cardinality equivalence is pinned here.
			return nil, false, nil
		}
		out, err := count(value)
		return out, true, err
	}
	shape := "distinct-count/" + string(array.Kind())
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayDistinctCount",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedDistinctCount(array)
		},
	})
}

func (s *EvalState) evalQPipelineCountWhereIn(plan *qPipelinePlan) (any, bool, error) {
	left, err := s.evalQPipelinePlannedExpr(plan.leftExpr, &plan.leftPlan)
	if err != nil {
		return nil, true, err
	}
	array, ok := left.(data.Array)
	if !ok {
		return nil, false, nil
	}
	right, err := s.evalQPipelinePlannedExpr(plan.rightExpr, &plan.rightPlan)
	if err != nil {
		return nil, true, err
	}
	values, err := setItems(right)
	if err != nil {
		return nil, true, err
	}
	shape := "in-count/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel: "ArrayInCount",
		shape:  shape,
		call: func() (any, bool, error) {
			return data.TryTypedInCount(array, values)
		},
	})
}

func (s *EvalState) evalQPipelinePlannedExpr(src string, plan *qScriptBindingPlan) (any, error) {
	if plan != nil && plan.kind != qScriptBindingInvalid {
		if value, handled, err := s.evalQScriptBindingPlan(plan); err != nil || handled {
			return value, err
		}
	}
	return s.eval(src)
}

func qPipelineOperandFingerprintForValue(role qPipelineOperandRole, value any) qPipelineOperandFingerprint {
	if array, ok := value.(data.Array); ok {
		return qPipelineOperandFingerprint{role: role, kind: array.Kind(), class: "array"}
	}
	if _, ok := numeric(value); ok {
		return qPipelineOperandFingerprint{role: role, kind: qRuntimeKernelOperandKind(value, nil), class: "scalar"}
	}
	if _, ok := value.(data.Frame); ok {
		return qPipelineOperandFingerprint{role: role, kind: data.KindAny, class: "frame"}
	}
	if _, ok := value.(data.KeyedFrame); ok {
		return qPipelineOperandFingerprint{role: role, kind: data.KindAny, class: "keyed_frame"}
	}
	return qPipelineOperandFingerprint{role: role, kind: qRuntimeKernelOperandKind(value, nil), class: "value"}
}

func qPipelineBindingKey(plan *qPipelinePlan, operands []qPipelineOperandFingerprint) qPipelineBindingCacheKey {
	if plan == nil {
		return qPipelineBindingCacheKey{}
	}
	source := plan.source
	if source == "" {
		source = plan.shape
	}
	var b strings.Builder
	for i, operand := range operands {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(string(operand.role))
		b.WriteByte(':')
		b.WriteString(operand.class)
		b.WriteByte(':')
		b.WriteString(string(operand.kind))
	}
	return qPipelineBindingCacheKey{
		source:      source,
		kind:        plan.kind,
		shape:       plan.shape,
		fingerprint: b.String(),
	}
}

func qGlobalPipelineBindingCacheProbe(key qPipelineBindingCacheKey) (qPipelineBoundPlan, bool) {
	if key.source == "" || key.fingerprint == "" {
		return qPipelineBoundPlan{}, false
	}
	qGlobalScriptPlanCacheMu.Lock()
	bound, ok := qGlobalPipelineBindingCache[key]
	if ok {
		qGlobalScriptPlanStats.PipelineBindingHits++
	} else {
		qGlobalScriptPlanStats.PipelineBindingMisses++
	}
	qGlobalScriptPlanCacheMu.Unlock()
	if !ok {
		return qPipelineBoundPlan{}, false
	}
	return bound, true
}

func qGlobalPipelineBindingCacheStore(bound qPipelineBoundPlan) {
	key := bound.key
	if key.source == "" || key.fingerprint == "" {
		return
	}
	qGlobalScriptPlanCacheMu.Lock()
	if _, ok := qGlobalPipelineBindingCache[key]; !ok {
		qGlobalPipelineBindingCacheOrder = append(qGlobalPipelineBindingCacheOrder, key)
	}
	qGlobalPipelineBindingCache[key] = bound
	qGlobalScriptPlanStats.PipelineBindingStores++
	for len(qGlobalPipelineBindingCacheOrder) > qGlobalPipelineBindingCacheLimit {
		evict := qGlobalPipelineBindingCacheOrder[0]
		qGlobalPipelineBindingCacheOrder = qGlobalPipelineBindingCacheOrder[1:]
		delete(qGlobalPipelineBindingCache, evict)
		qGlobalScriptPlanStats.PipelineBindingEvictions++
	}
	qGlobalScriptPlanCacheMu.Unlock()
}
