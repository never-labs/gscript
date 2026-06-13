package q

import (
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// Compiled fused plan nodes.
//
// The string evaluator keeps fused/O(1) typed probes on several reducer
// shapes (count where <compare/like/in/null/mask>, count <length-preserving
// chain>, +/<unary|find|cast-chain> ...). Statement compilation used to
// refuse any reducer-over-transform tree (qCompiledTreeShadowsFusedProbe) so
// those statements stayed on the per-call string cascade. The nodes in this
// file bind the SAME fused kernels at compile time: the probe's string
// splitting and kernel selection happen once in compileQ*Reduction, and the
// per-call work is operand evaluation plus the kernel call, exactly the work
// the string probe does after its splits.
//
// Every node's execution mirrors one string-evaluator probe (same kernels,
// same probe records, same surfaced errors). A node that would have fallen
// through to a later probe declines with unsupportedEvalValueExpr, which the
// compiled statement route converts into the string-evaluator fallback - the
// full cascade re-runs, so declines are always value-faithful.

// SafeCall is a fused-reducer word applied to an argument whose syntax the
// compile-time matcher proved matches NO fused string-evaluator probe: the
// string path would evaluate the argument and apply the generic verb, which
// is exactly what SafeCall does. It is a distinct node type so the
// fused-probe guard can keep rejecting reducer Calls produced anywhere else.
type SafeCall struct {
	Func string
	Arg  Expr
}

// FusedCountWhereKind selects which `count where <arg>` probe the node
// compiled, in the string evaluator's probe order.
type FusedCountWhereKind uint8

const (
	fusedCountWhereCompare FusedCountWhereKind = iota
	fusedCountWhereLike
	fusedCountWhereIn
	fusedCountWhereNull
	fusedCountWhereMask
)

// FusedCountWhere is `count where <arg>` with the probe variant and operand
// splits resolved at compile time. Exactly one variant's syntax matched at
// compile time, so the runtime probe ladder for this source can only reach
// the compiled variant and, when its kernels decline, the trailing mask
// probe (tryEvalCountWhereMask); Mask is the whole argument compiled for
// that fallback.
type FusedCountWhere struct {
	Kind  FusedCountWhereKind
	Op    string // compare kind: "=", "<>", "<", "<=", ">", ">=", "within"
	Left  Expr   // compare/like/in left; null arg; mask expression
	Right Expr   // compare/like/in right; nil otherwise
	Mask  Expr   // whole argument (mask-probe fallback); Left for mask kind
}

// FusedCountLen is `count <length-preserving chain> <base>` (count fills x,
// count deltas fills x): the transforms are never materialized, mirroring
// tryEvalCountLengthPreservingTransform.
type FusedCountLen struct {
	Base    Expr
	Applied []int // indices into qFusedCountLenTransforms, outermost-first
}

// FusedSumUnary is `+/<unaryop> ...` mirroring evalQPipelineSumUnaryPrimitive,
// the route s.eval dispatches these shapes to (the pipeline plan runs at the
// top of the string cascade): the monad-chain kernel when the argument is a
// unary chain (optionally over deltas), then the single unary+sum kernel
// over the evaluated input.
type FusedSumUnary struct {
	Op     string
	Ops    []string // full chain, innermost-first; nil when no chain kernel
	Deltas bool
	Base   Expr // chain base (after all unary words and optional deltas)
	Input  Expr // input after the first unary word (plain unary-sum path)
}

// FusedSumFind is `+/<domain>?<query>` (tryEvalFindSum): single-pass
// find+sum without materializing the index vector.
type FusedSumFind struct {
	Left  Expr
	Right Expr
}

// FusedSumCastChain is "+/`k_n$...`k_1$<expr>" (tryEvalCastChainSum): the
// cast kinds are parsed once at compile; execution is one fused kernel pass.
type FusedSumCastChain struct {
	Kinds []data.Kind // innermost-first (kernel application order)
	Value Expr
}

// ApplyAtExpr is a top-level `a@b` apply/index split (non-bracket form),
// mirroring evalApplyIndex(qApplyIndexAt, ...).
type ApplyAtExpr struct {
	Left  Expr
	Right Expr
}

func (SafeCall) exprNode()          {}
func (FusedCountWhere) exprNode()   {}
func (FusedCountLen) exprNode()     {}
func (FusedSumUnary) exprNode()     {}
func (FusedSumFind) exprNode()      {}
func (FusedSumCastChain) exprNode() {}
func (ApplyAtExpr) exprNode()       {}

// ---------------------------------------------------------------------------
// Compile-time matchers
// ---------------------------------------------------------------------------

// compileQCountReduction lowers `count <argSrc>`, mirroring the s.eval
// count-probe ladder syntactically. It returns the fused node for ladder
// rungs it compiles, nil for rungs it leaves on the string evaluator, and a
// SafeCall when no rung's syntax matches (the string path's generic tail).
func compileQCountReduction(argSrc string, depth int) Expr {
	argSrc = strings.TrimSpace(argSrc)
	stripped := stripEnclosingParens(argSrc)
	// tryEvalCountFrameMetadata / tryEvalCountFlip / tryEvalCountGroup /
	// tryEvalCountDistinct: stay on the string evaluator.
	for _, prefix := range []string{"cols ", "meta ", "flip "} {
		if strings.HasPrefix(stripped, prefix) {
			return nil
		}
	}
	for _, prefix := range []string{"group ", "distinct "} {
		if strings.HasPrefix(argSrc, prefix) {
			return nil
		}
	}
	// tryEvalCountWhere{Compare,Like,In,Null,Mask}.
	if strings.HasPrefix(argSrc, "where ") {
		return compileQCountWhere(strings.TrimSpace(argSrc[len("where "):]), depth)
	}
	// tryEvalCountFby.
	if _, _, ok := splitTopLevelWord(argSrc, "fby"); ok {
		return nil
	}
	// tryEvalCountLengthPreservingTransform.
	if node := compileQCountLen(argSrc, depth); node != nil {
		return node
	}
	// tryEvalCountSequencePrimitive / tryEvalCountReverse /
	// tryEvalCountRunningScan / tryEvalCountXbar: stay on the string
	// evaluator (their probes carry O(1)/fused kernels not compiled here).
	if qCountSequenceProbeSyntax(stripped) {
		return nil
	}
	if _, _, ok := splitTopLevelWord(argSrc, "xbar"); ok {
		return nil
	}
	// No count probe can match: the string path is count(s.eval(arg)).
	arg := compileQEvalExpr(argSrc, depth+1)
	if arg == nil {
		return nil
	}
	return SafeCall{Func: "count", Arg: arg}
}

// qCountSequenceProbeSyntax reports whether the (paren-stripped) count
// argument matches any tryEvalCountSequencePrimitive / reverse / running-scan
// probe syntax.
func qCountSequenceProbeSyntax(stripped string) bool {
	for _, prefix := range []string{
		"reverse ", "raze ", "deltas ", "ratios ", // sequenceTransformExpr
		"trim ", "ltrim ", "rtrim ", // string transforms
		"sums ", "prds ", "mins ", "maxs ", "avgs ", // running scans
	} {
		if strings.HasPrefix(stripped, prefix) && wordBoundary(stripped, 0, len(strings.TrimSpace(prefix))) {
			return true
		}
	}
	for _, word := range []string{"rotate", "sublist", "cross", "cut"} {
		if _, _, ok := splitTopLevelWord(stripped, word); ok {
			return true
		}
	}
	if args, ok := qFunctionCallArgs(stripped); ok && len(args) > 0 {
		if idx := strings.Index(stripped, "["); idx > 0 && strings.TrimSpace(stripped[:idx]) == "sublist" {
			return true
		}
	}
	if qPipelineMatrixReshapeExprCandidate(stripped) {
		return true
	}
	return false
}

// compileQCountWhere compiles `count where <arg>` when exactly one
// count-where probe's syntax matches.
func compileQCountWhere(arg string, depth int) Expr {
	arg = strings.TrimSpace(arg)
	compareLeft, compareRight, compareOp, isCompare := splitWhereCompareExpr(arg)
	likeLeft, likeRight, isLike := splitTopLevelWord(arg, "like")
	inLeft, inRight, isIn := splitTopLevelWord(arg, "in")
	isNull := strings.HasPrefix(arg, "null ")
	matches := 0
	for _, m := range []bool{isCompare, isLike, isIn, isNull} {
		if m {
			matches++
		}
	}
	if matches > 1 {
		// Ambiguous probe ladder (a later probe could run after an earlier
		// kernel decline): stay on the string evaluator.
		return nil
	}
	switch {
	case isNull:
		value := compileQEvalExpr(strings.TrimSpace(arg[len("null "):]), depth+1)
		if value == nil {
			return nil
		}
		// The null probe is total after its syntax match: no mask fallback.
		return FusedCountWhere{Kind: fusedCountWhereNull, Left: value}
	case isCompare, isLike, isIn:
		// When the variant's kernels decline at runtime, the string ladder
		// falls through to the mask probe over the whole argument.
		mask := compileQEvalExpr(arg, depth+1)
		if mask == nil {
			return nil
		}
		var kind FusedCountWhereKind
		var op, leftExpr, rightExpr string
		switch {
		case isCompare:
			kind, op, leftExpr, rightExpr = fusedCountWhereCompare, compareOp, compareLeft, compareRight
		case isLike:
			kind, leftExpr, rightExpr = fusedCountWhereLike, likeLeft, likeRight
		default:
			kind, leftExpr, rightExpr = fusedCountWhereIn, inLeft, inRight
		}
		left := compileQEvalExpr(leftExpr, depth+1)
		if left == nil {
			return nil
		}
		right := compileQEvalExpr(rightExpr, depth+1)
		if right == nil {
			return nil
		}
		return FusedCountWhere{Kind: kind, Op: op, Left: left, Right: right, Mask: mask}
	default:
		mask := compileQEvalExpr(arg, depth+1)
		if mask == nil {
			return nil
		}
		return FusedCountWhere{Kind: fusedCountWhereMask, Left: mask, Mask: mask}
	}
}

// qFusedCountLenTransforms mirrors tryEvalCountLengthPreservingTransform's
// transform table.
var qFusedCountLenTransforms = []struct {
	word    string
	kernel  string
	kindOK  func(data.Kind) bool
	verbErr string
}{
	{"prior ", "ArrayCountPrev", nil, ""},
	{"prev ", "ArrayCountPrev", nil, ""},
	{"next ", "ArrayCountNext", nil, ""},
	{"fills ", "ArrayCountFills", nil, ""},
	{"deltas ", "ArrayCountDeltas", qKindIsNumeric, "deltas expects a numeric vector"},
	{"ratios ", "ArrayCountRatios", qKindIsNumeric, "ratios expects a numeric vector"},
}

func compileQCountLen(argSrc string, depth int) Expr {
	src := stripEnclosingParens(strings.TrimSpace(argSrc))
	var applied []int
	for {
		matched := false
		for t, transform := range qFusedCountLenTransforms {
			if !strings.HasPrefix(src, transform.word) || !wordBoundary(src, 0, len(strings.TrimSpace(transform.word))) {
				continue
			}
			applied = append(applied, t)
			src = stripEnclosingParens(strings.TrimSpace(src[len(transform.word):]))
			matched = true
			break
		}
		if !matched {
			break
		}
	}
	if len(applied) == 0 {
		return nil
	}
	base := compileQEvalExpr(src, depth+1)
	if base == nil {
		return nil
	}
	return FusedCountLen{Base: base, Applied: applied}
}

// compileQSumOverReduction lowers `+/<argSrc>`, mirroring the s.eval "+/"
// probe ladder syntactically.
func compileQSumOverReduction(argSrc string, depth int) Expr {
	argSrc = strings.TrimSpace(argSrc)
	stripped := stripEnclosingParens(argSrc)
	// tryEvalSumFby.
	if _, _, ok := splitTopLevelWord(stripped, "fby"); ok {
		return nil
	}
	// tryEvalSumWhereGatherReduce / tryEvalSumWhereCompare (any top-level
	// `where` or postfix index keeps the string/pipeline fused routes).
	if _, _, ok := splitTopLevelWord(argSrc, "where"); ok {
		return nil
	}
	if _, _, ok := findPostfixIndex(argSrc); ok {
		return nil
	}
	// tryEvalSequenceTransformSum / tryEvalSumDeltas.
	if qSumSequenceProbeSyntax(stripped) {
		return nil
	}
	// tryEvalSortIndexSum / tryEvalRankSum.
	for _, prefix := range []string{"iasc ", "idesc ", "rank "} {
		if strings.HasPrefix(argSrc, prefix) {
			return nil
		}
	}
	// tryEvalTypedDyadicFloatSum.
	for _, word := range []string{data.NumericDyadicXExp, data.NumericDyadicXLog} {
		if _, _, ok := splitTopLevelWord(argSrc, word); ok {
			return nil
		}
	}
	// tryEvalTypedUnarySum.
	if op, rest, ok := splitLeadingNumericUnary(argSrc); ok {
		return compileQSumUnary(op, rest, depth)
	}
	// tryEvalTypedMovingWindowSum.
	for _, word := range []string{"msum", "mavg", "mcount", "mmin", "mmax"} {
		if _, _, ok := splitTopLevelWord(argSrc, word); ok {
			return nil
		}
	}
	// tryEvalFindSum.
	if leftExpr, rightExpr, ok := splitTopLevelOperator(argSrc, "?"); ok {
		left := compileQEvalExpr(leftExpr, depth+1)
		if left == nil {
			return nil
		}
		right := compileQEvalExpr(rightExpr, depth+1)
		if right == nil {
			return nil
		}
		return FusedSumFind{Left: left, Right: right}
	}
	// tryEvalCastChainSum.
	if kinds, rest, ok := qSplitSumCastChain(argSrc); ok {
		value := compileQEvalExpr(rest, depth+1)
		if value == nil {
			return nil
		}
		return FusedSumCastChain{Kinds: kinds, Value: value}
	}
	// No sum probe can match: the string path is sum(s.eval(arg)).
	arg := compileQEvalExpr(argSrc, depth+1)
	if arg == nil {
		return nil
	}
	return SafeCall{Func: "sum", Arg: arg}
}

// compileQSumWordReduction lowers `sum <argSrc>` (the word form): the only
// probe ahead of the generic prefix-loop tail is tryEvalSumDeltas.
func compileQSumWordReduction(argSrc string, depth int) Expr {
	argSrc = strings.TrimSpace(argSrc)
	stripped := stripEnclosingParens(argSrc)
	if strings.HasPrefix(stripped, "deltas ") && wordBoundary(stripped, 0, len("deltas")) {
		return nil
	}
	arg := compileQEvalExpr(argSrc, depth+1)
	if arg == nil {
		return nil
	}
	return SafeCall{Func: "sum", Arg: arg}
}

// qSumSequenceProbeSyntax reports whether the (paren-stripped) sum argument
// matches sequenceTransformExpr or tryEvalSumDeltas syntax.
func qSumSequenceProbeSyntax(stripped string) bool {
	for _, prefix := range []string{"reverse ", "raze ", "deltas ", "ratios "} {
		if strings.HasPrefix(stripped, prefix) && wordBoundary(stripped, 0, len(strings.TrimSpace(prefix))) {
			return true
		}
	}
	for _, word := range []string{"rotate", "sublist"} {
		if _, _, ok := splitTopLevelWord(stripped, word); ok {
			return true
		}
	}
	if idx := strings.Index(stripped, "["); idx > 0 && strings.TrimSpace(stripped[:idx]) == "sublist" {
		if _, ok := qFunctionCallArgs(stripped); ok {
			return true
		}
	}
	return false
}

func compileQSumUnary(op, rest string, depth int) Expr {
	input := compileQEvalExpr(rest, depth+1)
	if input == nil {
		return nil
	}
	node := FusedSumUnary{Op: op, Input: input}
	// Mirror tryEvalQPipelineSumMonadChain's chain derivation.
	ops := []string{op}
	chainRest := rest
	for {
		next, arg, ok := splitLeadingNumericUnary(chainRest)
		if !ok || arg == "" {
			break
		}
		ops = append(ops, next)
		chainRest = arg
	}
	deltas := false
	if strings.HasPrefix(chainRest, "deltas") && len(chainRest) > len("deltas") && isSpace(chainRest[len("deltas")]) {
		if arg := strings.TrimSpace(chainRest[len("deltas"):]); arg != "" {
			deltas = true
			chainRest = arg
		}
	}
	if len(ops) >= 2 || deltas {
		if _, ok := data.NewQNumericUnaryChainSumPlan(qReverseUnaryChain(ops), deltas); ok {
			base := compileQEvalExpr(chainRest, depth+1)
			if base == nil {
				return nil
			}
			node.Ops = qReverseUnaryChain(ops)
			node.Deltas = deltas
			node.Base = base
		}
	}
	return node
}

// qReverseUnaryChain returns the outermost-first chain reversed to the
// kernel's innermost-first order.
func qReverseUnaryChain(ops []string) []string {
	out := make([]string, len(ops))
	for i, op := range ops {
		out[len(ops)-1-i] = op
	}
	return out
}

// qSplitSumCastChain parses the leading "`kind$`kind$..." chain, mirroring
// tryEvalCastChainSum's parse; kinds return innermost-first.
func qSplitSumCastChain(src string) ([]data.Kind, string, bool) {
	var kinds []data.Kind
	rest := src
	for {
		word, after, ok := splitLeadingTickCast(rest)
		if !ok {
			break
		}
		kind, ok := qCastKindFromTypeText(word)
		if !ok {
			return nil, "", false
		}
		kinds = append(kinds, kind)
		rest = after
	}
	if len(kinds) < 2 || rest == "" {
		return nil, "", false
	}
	for i, j := 0, len(kinds)-1; i < j; i, j = i+1, j-1 {
		kinds[i], kinds[j] = kinds[j], kinds[i]
	}
	return kinds, rest, true
}

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

func (s *EvalState) evalFusedCountWhere(x FusedCountWhere) (any, error) {
	var out any
	var err error
	switch x.Kind {
	case fusedCountWhereCompare:
		out, err = s.evalFusedCountWhereCompare(x)
	case fusedCountWhereLike:
		out, err = s.evalFusedCountWhereLike(x)
	case fusedCountWhereIn:
		out, err = s.evalFusedCountWhereIn(x)
	case fusedCountWhereNull:
		return s.evalFusedCountWhereNull(x)
	default:
		return s.evalFusedCountWhereMask(x)
	}
	if err == nil || !isUnsupportedEvalValueExpr(err) {
		return out, err
	}
	// Variant kernels declined: the string ladder falls through to the mask
	// probe over the whole argument.
	return s.evalFusedCountWhereMask(FusedCountWhere{Kind: fusedCountWhereMask, Left: x.Mask, Mask: x.Mask})
}

// evalFusedCountWhereCompare mirrors tryEvalCountWhereCompare's kernel
// ladder: compare-count, compare-to-index stats, compare-to-index + len.
func (s *EvalState) evalFusedCountWhereCompare(x FusedCountWhere) (any, error) {
	left, err := s.evalValueExpr(x.Left)
	if err != nil {
		return nil, err
	}
	right, err := s.evalValueExpr(x.Right)
	if err != nil {
		return nil, err
	}
	if x.Op == "within" {
		array, low, high, ok, err := qWithinOperands(left, right)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, unsupportedEvalValueExpr{expr: x}
		}
		shape := "compare-count/within/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		count, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
			kernel: "ArrayWhereWithinCount",
			shape:  shape,
			call: func() (int64, bool, error) {
				return data.TryTypedWithinCount(array, low, high, true)
			},
		})
		if err != nil {
			return nil, err
		}
		if handled {
			return count, nil
		}
		shape = "compare-to-index-count-stats/within/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		count, _, handled, err = evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
			kernel: "ArrayWhereWithinStats",
			shape:  shape,
			call: func() (int64, int64, bool, error) {
				return data.TryTypedWithinIndexStatsI64(array, low, high, true)
			},
		})
		if err != nil {
			return nil, err
		}
		if handled {
			return count, nil
		}
		shape = "within-to-index/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(low, nil)) + "/" + string(qRuntimeKernelOperandKind(high, nil))
		indexes, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
			kernel: "ArrayWhereWithin",
			shape:  shape,
			call: func() (data.Array, bool, error) {
				return data.TryTypedWithinIndexesI64(array, low, high, true)
			},
		})
		if err != nil {
			return nil, err
		}
		if !handled {
			return nil, unsupportedEvalValueExpr{expr: x}
		}
		return int64(indexes.Len()), nil
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, x.Op)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	shape := "compare-count/" + x.Op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: "ArrayWhereCompareCount",
		shape:  shape,
		call: func() (int64, bool, error) {
			return data.TryTypedCompareCount(array, dataOp, scalar)
		},
	})
	if err != nil {
		return nil, err
	}
	if handled {
		return count, nil
	}
	shape = "compare-to-index-count-stats/" + x.Op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	count, _, handled, err = evalQTypedRuntimeKernel2(qTypedRuntimeKernel2[int64, int64]{
		kernel: "ArrayWhereCompareStats",
		shape:  shape,
		call: func() (int64, int64, bool, error) {
			return data.TryTypedCompareIndexStatsI64(array, dataOp, scalar)
		},
	})
	if err != nil {
		return nil, err
	}
	if handled {
		return count, nil
	}
	shape = "compare-to-index-count/" + x.Op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	indexes, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[data.Array]{
		kernel: "ArrayWhereCompare",
		shape:  shape,
		call: func() (data.Array, bool, error) {
			return data.TryTypedCompareIndexesI64(array, dataOp, scalar)
		},
	})
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	return int64(indexes.Len()), nil
}

func (s *EvalState) evalFusedCountWhereLike(x FusedCountWhere) (any, error) {
	left, err := s.evalValueExpr(x.Left)
	if err != nil {
		return nil, err
	}
	array, ok := left.(data.Array)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	right, err := s.evalValueExpr(x.Right)
	if err != nil {
		return nil, err
	}
	pattern, ok := qLikeText(right)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	shape := "like-count/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: "ArrayStringLikeCount",
		shape:  shape,
		call: func() (int64, bool, error) {
			return data.TryTypedStringLikeCount(array, pattern)
		},
	})
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	return out, nil
}

func (s *EvalState) evalFusedCountWhereIn(x FusedCountWhere) (any, error) {
	left, err := s.evalValueExpr(x.Left)
	if err != nil {
		return nil, err
	}
	array, ok := left.(data.Array)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	right, err := s.evalValueExpr(x.Right)
	if err != nil {
		return nil, err
	}
	values, err := setItems(right)
	if err != nil {
		return nil, err
	}
	shape := "in-count/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(right, nil))
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[int64]{
		kernel: "ArrayInCount",
		shape:  shape,
		call: func() (int64, bool, error) {
			return data.TryTypedInCount(array, values)
		},
	})
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	return out, nil
}

func (s *EvalState) evalFusedCountWhereNull(x FusedCountWhere) (any, error) {
	value, err := s.evalValueExpr(x.Left)
	if err != nil {
		return nil, err
	}
	array, ok := value.(data.Array)
	if !ok {
		if data.IsNull(value) {
			return int64(1), nil
		}
		return int64(0), nil
	}
	shape := "null-count/" + string(array.Kind())
	out, handled, err := data.TryTypedNullCount(array)
	out, handled, err = qTypedRuntimeResult("ArrayNullCount", shape, out, handled, err)
	if err != nil {
		return nil, err
	}
	if handled {
		return out, nil
	}
	nulls, err := nullValue(value)
	if err != nil {
		return nil, err
	}
	indexes, err := where(nulls)
	if err != nil {
		return nil, err
	}
	return count(indexes)
}

func (s *EvalState) evalFusedCountWhereMask(x FusedCountWhere) (any, error) {
	value, err := s.evalValueExpr(x.Left)
	if err != nil {
		return nil, err
	}
	array, ok := value.(data.Array)
	if !ok || array.Kind() != data.KindBool {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	out, handled, err := data.TryTypedTrueCount(array)
	out, handled, err = qTypedRuntimeResult("ArrayTrueCount", "true-count/"+string(array.Kind()), out, handled, err)
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	return out, nil
}

func (s *EvalState) evalFusedCountLen(x FusedCountLen) (any, error) {
	value, err := s.evalValueExpr(x.Base)
	if err != nil {
		return nil, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	for i := len(x.Applied) - 1; i >= 0; i-- {
		transform := qFusedCountLenTransforms[x.Applied[i]]
		shape := "vector-count/" + strings.TrimSpace(transform.word) + "/" + string(array.Kind())
		if transform.kindOK != nil && !transform.kindOK(array.Kind()) {
			err := fmt.Errorf("%s", transform.verbErr)
			recordRuntimeKernelProbe(transform.kernel, shape, false, err)
			return nil, err
		}
		recordRuntimeKernelProbe(transform.kernel, shape, true, nil)
	}
	return int64(array.Len()), nil
}

func (s *EvalState) evalFusedSumUnary(x FusedSumUnary) (any, error) {
	// Monad-chain kernel first, mirroring tryEvalQPipelineSumMonadChain.
	if len(x.Ops) > 0 {
		kernel, ok := data.NewQNumericUnaryChainSumPlan(x.Ops, x.Deltas)
		if ok {
			value, err := s.evalValueExpr(x.Base)
			if err != nil {
				return nil, err
			}
			if array, ok := value.(data.Array); ok {
				shape := "vector-reduce/chain-sum/" + strings.Join(x.Ops, "-")
				if x.Deltas {
					shape += "/deltas"
				}
				shape += "/" + string(array.Kind())
				out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
					kernel:         "ArrayNumericUnaryChainSum",
					shape:          shape,
					fallbackReason: RuntimeFallbackUnsupportedType,
					call: func() (any, bool, error) {
						return data.TryTypedQNumericUnaryChainSumPlanned(kernel, array)
					},
				})
				if err != nil {
					return nil, err
				}
				if handled {
					return out, nil
				}
			}
		}
	}
	// Plain unary+sum kernel over the evaluated input, mirroring
	// evalQPipelineSumUnaryPrimitive's staged route.
	value, err := s.evalValueExpr(x.Input)
	if err != nil {
		return nil, err
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	shape := "vector-reduce/sum-unary-" + x.Op + "/" + string(array.Kind())
	out, handled, err := evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "ArrayNumericUnarySum",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			return data.TryTypedQNumericUnarySum(x.Op, array)
		},
	})
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	return out, nil
}

func (s *EvalState) evalFusedSumFind(x FusedSumFind) (any, error) {
	left, err := s.evalValueExpr(x.Left)
	if err != nil {
		if isUnsupportedEvalValueExpr(err) {
			return nil, err
		}
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	domainArray, ok := left.(data.Array)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	right, err := s.evalValueExpr(x.Right)
	if err != nil {
		if isUnsupportedEvalValueExpr(err) {
			return nil, err
		}
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	queryArray, ok := right.(data.Array)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	out, handled := data.TryTypedFindComparableSum(domainArray, queryArray)
	shape := "vector-reduce/find-sum/" + string(domainArray.Kind()) + "/" + string(queryArray.Kind())
	recordRuntimeKernelProbe("ArrayFindSum", shape, handled, nil)
	if !handled {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	return out, nil
}

func (s *EvalState) evalFusedSumCastChain(x FusedSumCastChain) (any, error) {
	value, err := s.evalValueExpr(x.Value)
	if err != nil {
		if isUnsupportedEvalValueExpr(err) {
			return nil, err
		}
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	out, handled := data.TryTypedIntCastChainSum(x.Kinds, array)
	recordRuntimeKernelProbe("ArrayIntCastChainSum", "vector-reduce/cast-chain-sum/"+string(array.Kind()), handled, nil)
	if !handled {
		return nil, unsupportedEvalValueExpr{expr: x}
	}
	return out, nil
}

func (s *EvalState) evalApplyAtExpr(x ApplyAtExpr) (any, error) {
	left, err := s.evalValueExpr(x.Left)
	if err != nil {
		return nil, err
	}
	right, err := s.evalValueExpr(x.Right)
	if err != nil {
		return nil, err
	}
	return s.applyOrIndexValue(qApplyIndexAt, left, right)
}
