package q

import (
	"os"
	"strings"
	"sync"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// Statement compilation: the warm-path replacement for the per-call string
// evaluator cascade.
//
// compileQStatementExpr lowers one ordinary q statement into an Expr tree by
// mirroring s.eval's probe order with THE SAME split helpers
// (splitTopLevelDyadicWordMap, findDyadic, findPostfixIndex, ...) and THE
// SAME leaf parsers (parseAtomOrVector, parseSymbolList,
// parseTemporalAtomOrVector, ...). The compiled tree binds to the same verb
// functions the string evaluator dispatches to, so a compiled statement is
// value-identical to its string evaluation; the per-call gain is that all
// splitting, literal parsing, and probe walking happens once at compile time.
//
// Forms the compiler cannot faithfully represent (lambdas, adverbs, apply/
// amend, control flow, system commands, IPC, set forms, table literals,
// projections) compile to nil and keep using the string evaluator. At
// runtime a compiled statement that fails evaluation (for example an unbound
// name) falls back to the string evaluator too, so user-visible errors are
// always the string evaluator's errors.

// Const is a literal compiled to its parsed value. Values follow the q
// copy-on-write contract: consumers never mutate operand arrays in place, so
// one parsed literal can be shared by every execution (the same contract the
// script binding-plan literal cache relies on).
type Const struct {
	Value any
}

// ListExpr is a parenthesized `(a;b;c)` or bare `a;b;c` expression list.
// Parenthesized lists infer a typed array (inferQArray); bare lists build a
// generic list (data.NewAny), mirroring the two string-evaluator branches.
type ListExpr struct {
	Items []Expr
	Bare  bool
}

// CastExpr is `domain$value`. Domain nil means the empty/backtick symbol
// cast target. BareSym carries evalCastDomain's bare-cast-name fallback
// (`"long"$x` written as `long$x` with `long` unbound).
type CastExpr struct {
	DomainSrc string
	Domain    Expr
	BareSym   data.Symbol
	Value     Expr
}

// DyadicWordExpr is a word-form dyadic verb split (`x mod y`, `a xasc t`).
// Execution dispatches through the same qDyadicWordOps/qSetWordOps registry
// entry the string evaluator's split probe uses.
type DyadicWordExpr struct {
	Word  string
	Set   bool
	Left  Expr
	Right Expr
}

// NameVectorExpr is a juxtaposed bound-name vector (`x y z`), mirroring
// evalNameVector.
type NameVectorExpr struct {
	Names []string
}

func (Const) exprNode()          {}
func (ListExpr) exprNode()       {}
func (CastExpr) exprNode()       {}
func (DyadicWordExpr) exprNode() {}
func (NameVectorExpr) exprNode() {}

const qCompileMaxDepth = 48

// qDifferentialMatchBudget bounds the per-statement comparison work in
// EvalScriptCompiledDifferential.
const qDifferentialMatchBudget = 1 << 22

var (
	qGlobalCompiledStatementMu    sync.Mutex
	qGlobalCompiledStatementCache = map[string]Expr{}
)

const qGlobalCompiledStatementCacheLimit = 4096

// qStatementCompileDisabled is a same-binary benchmarking and diagnosis
// toggle: Q_EVAL_DISABLE_STATEMENT_COMPILE=1 routes every statement the
// plans do not claim back through the string evaluator.
var qStatementCompileDisabled = sync.OnceValue(func() bool {
	return os.Getenv("Q_EVAL_DISABLE_STATEMENT_COMPILE") == "1"
})

// compiledQStatementExprCached resolves the compiled form of one statement
// source through a bounded process-wide cache. nil entries memoize
// statements the compiler cannot represent.
func compiledQStatementExprCached(src string) Expr {
	if qStatementCompileDisabled() {
		return nil
	}
	qGlobalCompiledStatementMu.Lock()
	if expr, ok := qGlobalCompiledStatementCache[src]; ok {
		qGlobalCompiledStatementMu.Unlock()
		return expr
	}
	qGlobalCompiledStatementMu.Unlock()
	expr := compileQStatementExpr(src)
	qGlobalCompiledStatementMu.Lock()
	if len(qGlobalCompiledStatementCache) >= qGlobalCompiledStatementCacheLimit {
		qGlobalCompiledStatementCache = make(map[string]Expr, 64)
	}
	qGlobalCompiledStatementCache[src] = expr
	qGlobalCompiledStatementMu.Unlock()
	return expr
}

// ClearCompiledStatementCache resets the process-wide compiled statement
// and apply-form plan caches (test support).
func ClearCompiledStatementCache() {
	qGlobalCompiledStatementMu.Lock()
	qGlobalCompiledStatementCache = make(map[string]Expr, 64)
	qGlobalCompiledStatementMu.Unlock()
	qGlobalApplyFormPlanMu.Lock()
	qGlobalApplyFormPlanCache = make(map[string]*qApplyFormPlan, 64)
	qGlobalApplyFormPlanMu.Unlock()
}

func compileQStatementExpr(src string) Expr {
	expr := compileQEvalExpr(src, 0)
	if expr == nil || qCompiledTreeShadowsFusedProbe(expr) {
		return nil
	}
	return expr
}

// qFusedReducerWords lists unary prefix words whose string-evaluator
// branches carry fused or O(1) typed probes over transform arguments
// (tryEvalCount*, tryEvalSum*, evalWhereCompare/In, sorted-edge, first/last
// terminal scans). Compiling `word <transform>` would re-route those
// statements onto multi-pass generic kernels, so they stay on the string
// evaluator; `word <leaf>` carries no fusion and compiles.
var qFusedReducerWords = map[string]bool{
	"count": true, "sum": true, "first": true, "last": true,
	"min": true, "max": true, "avg": true, "med": true,
	"var": true, "dev": true, "svar": true, "sdev": true,
	"all": true, "any": true, "where": true,
}

// qCompiledTreeShadowsFusedProbe reports whether any node in the compiled
// tree is a fused-probe reducer applied to a non-leaf argument.
func qCompiledTreeShadowsFusedProbe(expr Expr) bool {
	switch x := expr.(type) {
	case Call:
		if qFusedReducerWords[x.Func] && !qCompiledLeafExpr(x.Arg) {
			return true
		}
		return qCompiledTreeShadowsFusedProbe(x.Arg)
	case SafeCall:
		// The compile-time matcher proved the argument syntax matches no
		// fused string-evaluator probe.
		return qCompiledTreeShadowsFusedProbe(x.Arg)
	case FusedCountWhere:
		if qCompiledTreeShadowsFusedProbe(x.Left) {
			return true
		}
		if x.Right != nil && qCompiledTreeShadowsFusedProbe(x.Right) {
			return true
		}
		return x.Mask != nil && qCompiledTreeShadowsFusedProbe(x.Mask)
	case FusedCountLen:
		return qCompiledTreeShadowsFusedProbe(x.Base)
	case FusedSumUnary:
		if qCompiledTreeShadowsFusedProbe(x.Input) {
			return true
		}
		return x.Base != nil && qCompiledTreeShadowsFusedProbe(x.Base)
	case FusedSumFind:
		return qCompiledTreeShadowsFusedProbe(x.Left) || qCompiledTreeShadowsFusedProbe(x.Right)
	case FusedSumCastChain:
		return qCompiledTreeShadowsFusedProbe(x.Value)
	case ApplyAtExpr:
		return qCompiledTreeShadowsFusedProbe(x.Left) || qCompiledTreeShadowsFusedProbe(x.Right)
	case Binary:
		return qCompiledTreeShadowsFusedProbe(x.Left) || qCompiledTreeShadowsFusedProbe(x.Right)
	case DyadicWordExpr:
		return qCompiledTreeShadowsFusedProbe(x.Left) || qCompiledTreeShadowsFusedProbe(x.Right)
	case CastExpr:
		if x.Domain != nil && qCompiledTreeShadowsFusedProbe(x.Domain) {
			return true
		}
		return qCompiledTreeShadowsFusedProbe(x.Value)
	case DictExpr:
		return qCompiledTreeShadowsFusedProbe(x.Keys) || qCompiledTreeShadowsFusedProbe(x.Values)
	case IndexExpr:
		return qCompiledTreeShadowsFusedProbe(x.Expr) || qCompiledTreeShadowsFusedProbe(x.Index)
	case ListExpr:
		for _, item := range x.Items {
			if qCompiledTreeShadowsFusedProbe(item) {
				return true
			}
		}
		return false
	case Vector:
		for _, item := range x.Items {
			if qCompiledTreeShadowsFusedProbe(item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func qCompiledLeafExpr(expr Expr) bool {
	switch x := expr.(type) {
	case Ident, Const, NameVectorExpr, Number, Symbol, Bool, String, Temporal, TypedNull, Null:
		return true
	case IndexExpr:
		return qCompiledLeafExpr(x.Expr) && qCompiledLeafExpr(x.Index)
	default:
		return false
	}
}

// qCompilePrefixWords mirrors the unary prefix-word probes of the string
// evaluator: the big prefix loop plus the dedicated flip/enlist/til
// branches. Runtime dispatch goes through evalValueCall, which resolves the
// same functions.
var qCompilePrefixWords = map[string]bool{
	"where": true, "not": true, "null": true, "raze": true, "keys": true,
	"key": true, "value": true, "cols": true, "meta": true, "attr": true,
	"codes": true, "domain": true, "group": true, "ungroup": true,
	"type": true, "string": true, "lower": true, "upper": true,
	"trim": true, "ltrim": true, "rtrim": true, "ssr": true, "count": true,
	"first": true, "last": true, "sum": true, "avg": true, "var": true,
	"dev": true, "svar": true, "sdev": true, "wsum": true, "med": true,
	"min": true, "max": true, "prd": true, "distinct": true,
	"reverse": true, "prior": true, "prev": true, "next": true,
	"deltas": true, "fills": true, "differ": true, "ratios": true,
	"asc": true, "desc": true, "iasc": true, "idesc": true, "rank": true,
	"neg": true, "abs": true, "sqrt": true, "log": true, "exp": true,
	"sin": true, "cos": true, "tan": true, "asin": true, "acos": true,
	"atan": true, "reciprocal": true, "signum": true, "floor": true,
	"ceiling": true, "inv": true, "sums": true, "prds": true, "mins": true,
	"maxs": true, "avgs": true, "all": true, "any": true,
	"flip": true, "enlist": true, "til": true, "parse": true,
}

func compileQEvalExpr(src string, depth int) Expr {
	src = strings.TrimSpace(src)
	if src == "" || depth > qCompileMaxDepth {
		return nil
	}
	// Conditional and control special forms stay on the string evaluator.
	if len(src) >= 2 && (src[0] == '$' || src[0] == '?') && src[1] == '[' {
		return nil
	}
	if name, _, ok := parseNamedBracketForm(src); ok {
		switch name {
		case "if", "do", "while", "cut", "sublist", "ssr":
			return nil
		}
	}
	// System commands, namespaces, IPC, set forms, deferred-state prefixes.
	if strings.HasPrefix(src, "\\") || strings.HasPrefix(src, ".z.") {
		return nil
	}
	// `eval ` stays on the string evaluator: parse-tree evaluation is
	// session-routed (see value_eval_parse.go), and the cascade below has no
	// faithful compiled form for it.
	for _, prefix := range []string{"set ", "lookup ", "get ", "hopen ", "take ", "drop ", "eval "} {
		if strings.HasPrefix(src, prefix) {
			return nil
		}
	}
	if _, _, ok := splitPathStyleSet(src); ok {
		return nil
	}
	// Lambda, adverb, projection, composition, and apply/amend territory:
	// these probes are state-dependent (callable detection consults
	// bindings), so any statement that could reach them stays on the string
	// evaluator. The scan is whole-span and conservative.
	if strings.ContainsAny(src, "'{}") {
		return nil
	}
	// '/' and '\' only ever compile as part of a "+/" (sum-over) or "+\"
	// (running-sum) pair; every other occurrence (adverbs, system commands,
	// paths, comments) stays on the string evaluator. Pair occurrences in
	// nested positions recurse through the sum branch, which re-applies this
	// rule to operands.
	for idx := 0; idx < len(src); idx++ {
		if (src[idx] == '/' || src[idx] == '\\') && (idx == 0 || src[idx-1] != '+') {
			return nil
		}
	}
	if qMayBeApplyIndexForm(src) {
		// Bracket apply/amend forms and dot-apply stay on the per-call apply
		// plan layer; the plain top-level `a@b` split compiles, mirroring
		// evalApplyIndex(qApplyIndexAt, ...).
		if strings.HasPrefix(src, "@[") || strings.HasPrefix(src, ".[") {
			return nil
		}
		if _, _, ok := splitTopLevelDotApply(src); ok {
			return nil
		}
		if leftExpr, rightExpr, ok := splitTopLevelOperator(src, "@"); ok {
			left := compileQEvalExpr(leftExpr, depth+1)
			if left == nil {
				return nil
			}
			right := compileQEvalExpr(rightExpr, depth+1)
			if right == nil {
				return nil
			}
			return ApplyAtExpr{Left: left, Right: right}
		}
		return nil
	}
	if _, _, ok := splitTopLevelWord(src, "each"); ok {
		return nil
	}
	if _, _, ok := splitTopLevelWord(src, "fby"); ok {
		return nil
	}
	if _, ok := parseUnaryComposition(src); ok {
		return nil
	}
	if _, ok := parseDyadicAdverbFunction(src); ok {
		return nil
	}
	if looksLikeTableLiteral(src) {
		return nil
	}
	// Bound-name and bare-verb references resolve per call through Ident.
	if isQAssignmentName(src) {
		return Ident{Name: src}
	}
	if src == "()" {
		return Const{Value: data.NewAny(nil)}
	}
	// Parenthesized group or inline list.
	if enclosed(src, '(', ')') {
		inner := strings.TrimSpace(src[1 : len(src)-1])
		if parts := splitTopLevelDelim(inner, ';'); len(parts) > 1 {
			items := make([]Expr, len(parts))
			for i, part := range parts {
				if items[i] = compileQEvalExpr(part, depth+1); items[i] == nil {
					return nil
				}
			}
			return ListExpr{Items: items}
		}
		return compileQEvalExpr(inner, depth+1)
	}
	// Temporal and boolean literals parse once.
	if value, ok, err := parseTemporalAtomOrVector(src); ok {
		if err != nil {
			return nil
		}
		return Const{Value: value}
	}
	if value, ok, err := parseQBoolLiteral(src); ok {
		if err != nil {
			return nil
		}
		return Const{Value: value}
	}
	if strings.HasPrefix(src, "`:") {
		if syms, err := parseSymbolList(src); err == nil && len(syms) == 1 {
			return Const{Value: syms[0]}
		}
		return nil
	}
	// Postfix bracket index and postfix symbol lookup.
	if collectionExpr, indexExpr, ok := findPostfixIndex(src); ok {
		if strings.Contains(indexExpr, ";") || strings.TrimSpace(indexExpr) == "" {
			return nil
		}
		collection := compileQEvalExpr(collectionExpr, depth+1)
		if collection == nil {
			return nil
		}
		index := compileQEvalExpr(indexExpr, depth+1)
		if index == nil {
			return nil
		}
		return IndexExpr{Expr: collection, Index: index}
	}
	if collectionExpr, symbolExpr, ok := findPostfixSymbolLookup(src); ok {
		collection := compileQEvalExpr(collectionExpr, depth+1)
		if collection == nil {
			return nil
		}
		index := compileQEvalExpr(symbolExpr, depth+1)
		if index == nil {
			return nil
		}
		return IndexExpr{Expr: collection, Index: index}
	}
	// `?x` distinct, `+/x` sum-over, `+\x` running-sum prefixes.
	if strings.HasPrefix(src, "?") {
		arg := compileQEvalExpr(src[1:], depth+1)
		if arg == nil {
			return nil
		}
		return Call{Func: "distinct", Arg: arg}
	}
	if strings.HasPrefix(src, "+/") {
		return compileQSumOverReduction(src[2:], depth)
	}
	if strings.HasPrefix(src, "+\\") {
		arg := compileQEvalExpr(src[2:], depth+1)
		if arg == nil {
			return nil
		}
		return Call{Func: "sums", Arg: arg}
	}
	// Unary prefix words (count/where/string/... plus flip/enlist/til).
	// count and sum route through the fused-probe matchers, which bind the
	// string evaluator's fused kernels at compile time.
	if space := strings.IndexByte(src, ' '); space > 0 {
		word := src[:space]
		switch word {
		case "count":
			return compileQCountReduction(src[space+1:], depth)
		case "sum":
			return compileQSumWordReduction(src[space+1:], depth)
		}
		if qCompilePrefixWords[word] {
			arg := compileQEvalExpr(src[space+1:], depth+1)
			if arg == nil {
				return nil
			}
			return Call{Func: word, Arg: arg}
		}
	}
	// `,` join: mirrors the string evaluator's qTopLevelJoinSplit claim just
	// before the loose-binding dyadic probes (see join.go).
	if idx, ok := qTopLevelJoinSplit(src); ok {
		if idx == 0 {
			// Prefix `,x` enlist; Call enlist executes data.NewAny([]any{v}),
			// the same shape as the string evaluator's branch.
			arg := compileQEvalExpr(src[1:], depth+1)
			if arg == nil {
				return nil
			}
			return Call{Func: "enlist", Arg: arg}
		}
		return compileQBinarySplit(",", src[:idx], src[idx+1:], depth)
	}
	// Composite comparisons, match, and find splits.
	for _, op := range []string{"<>", "<=", ">="} {
		if leftExpr, rightExpr, ok := splitTopLevelOperator(src, op); ok {
			return compileQBinarySplit(op, leftExpr, rightExpr, depth)
		}
	}
	if leftExpr, rightExpr, ok := splitTopLevelOperator(src, "~"); ok {
		return compileQBinarySplit("~", leftExpr, rightExpr, depth)
	}
	if leftExpr, rightExpr, ok := splitTopLevelOperator(src, "?"); ok {
		return compileQBinarySplit("?", leftExpr, rightExpr, depth)
	}
	if _, _, ok := findPostfixLookup(src); ok {
		return nil
	}
	// Word-form dyadic verbs, then set words.
	if op, leftExpr, rightExpr, ok := splitTopLevelDyadicWordMap(src, qDyadicWordOps); ok {
		left := compileQEvalExpr(leftExpr, depth+1)
		if left == nil {
			return nil
		}
		right := compileQEvalExpr(rightExpr, depth+1)
		if right == nil {
			return nil
		}
		return DyadicWordExpr{Word: op.word, Left: left, Right: right}
	}
	if op, leftExpr, rightExpr, ok := splitTopLevelDyadicWordMap(src, qSetWordOps); ok {
		left := compileQEvalExpr(leftExpr, depth+1)
		if left == nil {
			return nil
		}
		right := compileQEvalExpr(rightExpr, depth+1)
		if right == nil {
			return nil
		}
		return DyadicWordExpr{Word: op.word, Set: true, Left: left, Right: right}
	}
	// Take/reshape, drop/cut, cast, dict-bang splits.
	if hash := findTopLevel(src, "#"); hash >= 0 {
		if _, ok := parseAttributeMarker(strings.TrimSpace(src[:hash])); ok {
			return nil
		}
		return compileQBinarySplit("#", src[:hash], src[hash+1:], depth)
	}
	if underscore := findTopLevel(src, "_"); underscore >= 0 {
		return compileQBinarySplit("_", src[:underscore], src[underscore+1:], depth)
	}
	if dollar := findTopLevel(src, "$"); dollar >= 0 {
		return compileQCast(strings.TrimSpace(src[:dollar]), src[dollar+1:], depth)
	}
	if bang := findTopLevel(src, "!"); bang >= 0 && !qBangYieldsToLeftmostDyadic(src, bang) {
		keys := compileQEvalExpr(src[:bang], depth+1)
		if keys == nil {
			return nil
		}
		values := compileQEvalExpr(src[bang+1:], depth+1)
		if values == nil {
			return nil
		}
		return DictExpr{Keys: keys, Values: values}
	}
	// Bare top-level `a;b` list.
	if parts := splitTopLevelDelim(src, ';'); len(parts) > 1 {
		items := make([]Expr, len(parts))
		for i, part := range parts {
			if items[i] = compileQEvalExpr(part, depth+1); items[i] == nil {
				return nil
			}
		}
		return ListExpr{Items: items, Bare: true}
	}
	// Literal vectors parse once. They sit before the arithmetic split,
	// mirroring the string evaluator (`1 2 -3` is a vector, not a minus).
	if looksLikeLiteralVector(src) {
		value, err := parseAtomOrVector(src)
		if err != nil {
			return nil
		}
		return Const{Value: value}
	}
	// Symbol-character arithmetic splits. Canonical q has no precedence among
	// the dyadic verbs and evaluates right-to-left, so we split at the single
	// LEFTMOST top-level operator (mirroring findDyadic): the leftmost verb is
	// the outermost operation and the rest is its right argument.
	if idx := findTopLevel(src, "=<>+-*%&|^,"); idx >= 0 {
		return compileQBinarySplit(string(src[idx]), src[:idx], src[idx+1:], depth)
	}
	// Symbol literals and symbol lists.
	if strings.HasPrefix(src, "`") {
		syms, err := parseSymbolList(src)
		if err != nil {
			return nil
		}
		if len(syms) == 1 {
			return Const{Value: syms[0]}
		}
		values := make([]string, len(syms))
		for i, sym := range syms {
			values[i] = string(sym)
		}
		return Const{Value: data.NewSymbols(values)}
	}
	// Juxtaposed bound-name vectors.
	if parts := splitTopLevelFields(src); len(parts) >= 2 {
		allNames := true
		for _, part := range parts {
			if !isQAssignmentName(part) {
				allNames = false
				break
			}
		}
		if allNames {
			return NameVectorExpr{Names: parts}
		}
	}
	// Pratt-parsed value expressions (the string evaluator's own tail).
	if expr, ok, err := parseValueExpr(src); err == nil && ok && expr != nil {
		if value, folded := foldQConstExpr(expr); folded {
			return Const{Value: value}
		}
		return expr
	}
	// Final leaf: atoms and literal vectors.
	value, err := parseAtomOrVector(src)
	if err != nil {
		return nil
	}
	return Const{Value: value}
}

func compileQBinarySplit(op, leftExpr, rightExpr string, depth int) Expr {
	left := compileQEvalExpr(leftExpr, depth+1)
	if left == nil {
		return nil
	}
	right := compileQEvalExpr(rightExpr, depth+1)
	if right == nil {
		return nil
	}
	return Binary{Op: op, Left: left, Right: right}
}

func compileQCast(domainSrc, valueSrc string, depth int) Expr {
	value := compileQEvalExpr(valueSrc, depth+1)
	if value == nil {
		return nil
	}
	node := CastExpr{DomainSrc: domainSrc}
	switch domainSrc {
	case "", "`":
		// evalCastDomain's symbol cast target; Domain stays nil.
	default:
		node.Domain = compileQEvalExpr(domainSrc, depth+1)
		if isBareQCastName(domainSrc) {
			node.BareSym = data.Symbol(domainSrc)
		}
		if node.Domain == nil && node.BareSym == "" {
			return nil
		}
	}
	node.Value = value
	return node
}

// foldQConstExpr folds literal-only parsed expressions to their value at
// compile time, using the same leaf evaluation the runtime would run.
func foldQConstExpr(expr Expr) (any, bool) {
	switch x := expr.(type) {
	case Number:
		value, _, err := parseNumberOrBool(x.Text)
		if err != nil {
			return nil, false
		}
		return value, true
	case Bool:
		return x.Value, true
	case String:
		return x.Value, true
	case Symbol:
		return data.Symbol(x.Name), true
	case Temporal:
		value, err := parseQTemporal(x.Kind, x.Text)
		if err != nil {
			return nil, false
		}
		return value, true
	case TypedNull:
		return data.NullForKind(data.Kind(x.Kind)), true
	case Vector:
		values := make([]any, len(x.Items))
		for i, item := range x.Items {
			value, ok := foldQConstExpr(item)
			if !ok {
				return nil, false
			}
			values[i] = value
		}
		array, err := evalValueVector(values)
		if err != nil {
			return nil, false
		}
		return array, true
	default:
		return nil, false
	}
}

// EvalCompiledDifferentialRecord reports one statement's compiled-route
// versus string-route comparison.
type EvalCompiledDifferentialRecord struct {
	Statement   string
	Compiled    bool
	CompiledErr error
	Match       bool
}

// EvalScriptCompiledDifferential executes src statement by statement. Before
// each statement runs normally, any statement the compiler can represent is
// evaluated through BOTH the compiled route and the string evaluator against
// the current session state; the two results must match. This is the
// differential oracle for statement compilation: it proves that compiling a
// statement never changes its value. Statements run normally afterwards so
// assignments and stateful forms keep ordinary session semantics.
func (s *EvalState) EvalScriptCompiledDifferential(src string) ([]EvalCompiledDifferentialRecord, error) {
	parts := splitQScriptStatements(src)
	records := make([]EvalCompiledDifferentialRecord, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		target := part
		if name, op, rhs, ok := splitTopLevelAugmentedAssignment(part); ok {
			target = name + op + rhs
		} else if _, rhs, ok := splitTopLevelAssignment(part); ok {
			target = rhs
		} else if _, _, rhs, ok := splitTopLevelIndexedAssignment(part); ok {
			// Indexed assignment: only the rhs is an ordinary expression;
			// the amend itself runs through s.evalScript below.
			target = rhs
		}
		record := EvalCompiledDifferentialRecord{Statement: target, Match: true}
		if compiled := compiledQStatementExprCached(target); compiled != nil {
			compiledValue, compiledErr := s.evalValueExpr(compiled)
			record.CompiledErr = compiledErr
			switch {
			case compiledErr == nil:
				record.Compiled = true
				stringValue, stringErr := s.eval(target)
				// Bounded comparison: astronomically large lazy values (a
				// fuzzer can build multi-hundred-million element views) give
				// up after the budget instead of walking every element.
				budget := qDifferentialMatchBudget
				record.Match = stringErr == nil && matchValueBudget(compiledValue, stringValue, &budget)
			case isUnsupportedEvalValueExpr(compiledErr):
				// Falls back to the string evaluator at runtime; nothing
				// surfaces to users.
			default:
				// Non-unsupported compiled errors surface directly, so the
				// string route must fail with the identical message.
				record.Compiled = true
				_, stringErr := s.eval(target)
				record.Match = stringErr != nil && stringErr.Error() == compiledErr.Error()
			}
		}
		records = append(records, record)
		if _, err := s.evalScript(part); err != nil {
			return records, err
		}
	}
	return records, nil
}
