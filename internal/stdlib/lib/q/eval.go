package q

import (
	"fmt"
	"math"
	"os"
	"path"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// EvalDict is a q dictionary literal lowered into library-owned values.
type EvalDict struct {
	Keys   []any
	Values []any
}

type Dict = EvalDict

type qLambda struct {
	body      string
	env       map[string]any
	namespace string
}

type qProjection struct {
	fn   any
	args []projectionArg
}

type qAdverbFunction struct {
	verb   string
	adverb string
}

type qCallableAdverb struct {
	fn     any
	adverb string
}

type qAttributedVector struct {
	attribute data.Symbol
	vector    data.Array
}

type qEnumVector struct {
	domain  data.Symbol
	encoded data.Array
}

type qIPCHandle struct {
	target  string
	async   bool
	session *EvalState
}

type qDyadicFunction struct {
	name string
	fn   func(any, any) (any, error)
}

type qUnaryFunction struct {
	name string
	fn   func(any) (any, error)
}

type qComposition struct {
	funcs []qUnaryFunction
}

// RuntimeKernelExecutionStat reports q.eval typed-runtime primitive execution.
// The shape matches bind's q.cache_stats runtime-kernel rows without importing
// bind into the q evaluator.
type RuntimeKernelExecutionStat struct {
	Source     string
	Kernel     string
	Shape      string
	Route      string
	Outcome    string
	ReasonCode string
	Count      uint64
}

type runtimeKernelExecutionKey struct {
	source     string
	kernel     string
	shape      string
	route      string
	outcome    string
	reasonCode string
}

var (
	runtimeKernelStatsMu sync.Mutex
	runtimeKernelStats   map[runtimeKernelExecutionKey]uint64
)

func recordRuntimeKernelExecution(kernel, shape, outcome, reasonCode string) {
	if kernel == "" {
		kernel = "unknown"
	}
	if shape == "" {
		shape = "unknown"
	}
	if outcome == "" {
		outcome = "unknown"
	}
	if reasonCode == "" {
		reasonCode = outcome
	}
	key := runtimeKernelExecutionKey{
		source:     "q_eval_vector_runtime",
		kernel:     kernel,
		shape:      shape,
		route:      "typed_data_kernel",
		outcome:    outcome,
		reasonCode: reasonCode,
	}
	runtimeKernelStatsMu.Lock()
	if runtimeKernelStats == nil {
		runtimeKernelStats = make(map[runtimeKernelExecutionKey]uint64)
	}
	runtimeKernelStats[key]++
	runtimeKernelStatsMu.Unlock()
}

func recordRuntimeKernelProbe(kernel, shape string, handled bool, err error) {
	recordRuntimeKernelExecution(kernel, shape, "attempt", "attempt")
	switch {
	case err != nil:
		recordRuntimeKernelExecution(kernel, shape, "error", "runtime_error")
	case handled:
		recordRuntimeKernelExecution(kernel, shape, "hit", "typed_kernel")
	default:
		recordRuntimeKernelExecution(kernel, shape, "fallback", "unsupported_shape")
	}
}

// RuntimeKernelExecutionStats returns a stable snapshot of q.eval typed
// primitive executions for q.cache_stats.
func RuntimeKernelExecutionStats() []RuntimeKernelExecutionStat {
	runtimeKernelStatsMu.Lock()
	defer runtimeKernelStatsMu.Unlock()
	if len(runtimeKernelStats) == 0 {
		return nil
	}
	out := make([]RuntimeKernelExecutionStat, 0, len(runtimeKernelStats))
	for key, count := range runtimeKernelStats {
		out = append(out, RuntimeKernelExecutionStat{
			Source:     key.source,
			Kernel:     key.kernel,
			Shape:      key.shape,
			Route:      key.route,
			Outcome:    key.outcome,
			ReasonCode: key.reasonCode,
			Count:      count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kernel != b.Kernel {
			return a.Kernel < b.Kernel
		}
		if a.Shape != b.Shape {
			return a.Shape < b.Shape
		}
		if a.Route != b.Route {
			return a.Route < b.Route
		}
		if a.Outcome != b.Outcome {
			return a.Outcome < b.Outcome
		}
		return a.ReasonCode < b.ReasonCode
	})
	return out
}

// ClearRuntimeKernelExecutionStats resets q.eval runtime-kernel counters.
func ClearRuntimeKernelExecutionStats() {
	runtimeKernelStatsMu.Lock()
	runtimeKernelStats = nil
	runtimeKernelStatsMu.Unlock()
}

type projectionArg struct {
	value   any
	missing bool
}

// EvalState carries q script bindings through recursive evaluation without
// relying on package-global state.
type EvalState struct {
	env       map[string]any
	port      int64
	namespace string
}

func NewEvalState(env map[string]any) *EvalState {
	return &EvalState{env: cloneEnv(env), namespace: "."}
}

func Eval(src string) (any, error) {
	return NewEvalState(nil).Eval(src)
}

func EvalWithEnv(src string, env map[string]any) (any, error) {
	return NewEvalState(env).Eval(src)
}

// EvalSourceCacheable reports whether src is safe for callers to memoize across
// stateless Eval calls. Stateful workspace, system, IPC, and filesystem forms
// stay uncached so repeated q.eval calls remain observably fresh.
func EvalSourceCacheable(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	scan := qEvalCacheScanText(src)
	lower := strings.ToLower(scan)
	parts := splitTopLevelDelim(lower, ';')
	if len(parts) == 0 {
		parts = []string{lower}
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "\\") {
			return false
		}
	}
	uncacheable := []string{
		".z.", "hopen", "hsym", "set ", "get ", "`:", "system ",
	}
	for _, marker := range uncacheable {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func qEvalCacheScanText(src string) string {
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if ch != '"' {
			out.WriteByte(ch)
			continue
		}
		out.WriteByte(' ')
		for i++; i < len(src); i++ {
			ch = src[i]
			out.WriteByte(' ')
			if ch == '\\' {
				i++
				if i < len(src) {
					out.WriteByte(' ')
				}
				continue
			}
			if ch == '"' {
				break
			}
		}
	}
	return out.String()
}

// EvalValueCacheable reports whether a returned Eval value is immutable enough
// for bind layers to store and convert again on cache hits.
func EvalValueCacheable(v any) bool {
	switch v.(type) {
	case Dict, data.Array, data.Frame, data.KeyedFrame,
		nil, bool, int, int64, float32, float64, string, data.Symbol,
		data.Month, data.Date, data.DateTime, data.Timespan,
		data.Minute, data.Second, data.Time, data.Timestamp:
		return true
	default:
		return false
	}
}

func (s *EvalState) Eval(src string) (any, error) {
	return s.evalScript(strings.TrimSpace(src))
}

func (s *EvalState) evalScript(src string) (any, error) {
	parts := splitTopLevelDelim(src, ';')
	if len(parts) <= 1 {
		if name, rhs, ok := splitTopLevelAssignment(src); ok {
			v, err := s.eval(rhs)
			if err != nil {
				return nil, err
			}
			s.env[s.resolveAssignmentName(name)] = v
			return v, nil
		}
		return s.eval(src)
	}
	var last any
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if name, rhs, ok := splitTopLevelAssignment(part); ok {
			v, err := s.eval(rhs)
			if err != nil {
				return nil, err
			}
			s.env[s.resolveAssignmentName(name)] = v
			last = v
			continue
		}
		v, err := s.eval(part)
		if err != nil {
			return nil, err
		}
		last = v
	}
	if last == nil {
		return nil, fmt.Errorf("empty q script")
	}
	return last, nil
}

func cloneEnv(env map[string]any) map[string]any {
	if len(env) == 0 {
		return make(map[string]any)
	}
	out := make(map[string]any, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}

func (s *EvalState) currentNamespace() string {
	if s.namespace == "" {
		return "."
	}
	return s.namespace
}

func (s *EvalState) resolveAssignmentName(name string) string {
	if strings.HasPrefix(name, ".") {
		return name
	}
	if s.currentNamespace() == "." {
		return name
	}
	return s.currentNamespace() + "." + name
}

func (s *EvalState) lookupName(name string) (any, bool) {
	if strings.HasPrefix(name, ".") {
		v, ok := s.env[name]
		return v, ok
	}
	if s.currentNamespace() != "." {
		v, ok := s.env[s.currentNamespace()+"."+name]
		return v, ok
	}
	v, ok := s.env[name]
	return v, ok
}

func (s *EvalState) visibleEnv() map[string]any {
	if s.currentNamespace() == "." {
		out := make(map[string]any)
		for name, value := range s.env {
			if strings.HasPrefix(name, ".") {
				continue
			}
			out[name] = value
		}
		return out
	}
	prefix := s.currentNamespace() + "."
	out := make(map[string]any)
	for name, value := range s.env {
		if strings.HasPrefix(name, prefix) {
			out[strings.TrimPrefix(name, prefix)] = value
		}
	}
	return out
}

func (s *EvalState) eval(src string) (any, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty q expression")
	}
	if value, ok, err := s.evalSystemCommand(src); ok {
		return value, err
	}
	if isQAssignmentName(src) {
		if v, ok := s.lookupName(src); ok {
			return v, nil
		}
	}
	if src == "()" {
		return data.NewAny(nil), nil
	}
	if value, ok, err := evalZNamespaceValue(src); ok {
		return value, err
	}
	if v, ok, err := s.evalTableLiteral(src); ok || err != nil {
		return v, err
	}
	if fn, ok := parseDyadicAdverbFunction(src); ok {
		return fn, nil
	}
	if enclosed(src, '(', ')') {
		inner := strings.TrimSpace(src[1 : len(src)-1])
		if parts := splitTopLevelDelim(inner, ';'); len(parts) > 1 {
			items := make([]any, 0, len(parts))
			for _, part := range parts {
				v, err := s.eval(part)
				if err != nil {
					return nil, err
				}
				items = append(items, v)
			}
			return inferQArray(items), nil
		}
		return s.eval(inner)
	}
	if value, ok, err := parseTemporalAtomOrVector(src); ok {
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	if strings.HasPrefix(src, "set ") {
		return s.evalSetPrefix(strings.TrimSpace(src[len("set "):]))
	}
	if leftExpr, rightExpr, ok := splitPathStyleSet(src); ok {
		return s.evalSet(leftExpr, rightExpr)
	}
	if strings.HasPrefix(src, "`:") {
		syms, err := parseSymbolList(src)
		if err == nil && len(syms) == 1 {
			return syms[0], nil
		}
	}
	if strings.HasPrefix(src, "@[") {
		return s.evalAmend(src)
	}
	if strings.HasPrefix(src, ".[") {
		return s.evalDotAmend(src)
	}
	if callableExpr, rightExpr, ok := splitTopLevelWord(src, "each"); ok {
		callable, err := s.eval(callableExpr)
		if err != nil {
			return nil, err
		}
		if !isCallable(callable) {
			return nil, fmt.Errorf("left side of callable each is not callable")
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return s.evalCallableAdverb(callable, "'", right)
	}
	if funcs, ok := parseUnaryComposition(src); ok {
		return qComposition{funcs: funcs}, nil
	}
	if callableExpr, adverb, rightExpr, ok := s.findCallablePostfixAdverb(src); ok {
		callable, err := s.eval(callableExpr)
		if err != nil {
			return nil, err
		}
		if !isCallable(callable) {
			return nil, fmt.Errorf("left side of callable adverb is not callable")
		}
		if strings.HasPrefix(rightExpr, "[") && enclosed(rightExpr, '[', ']') {
			return s.applyCallableIndex(qCallableAdverb{fn: callable, adverb: adverb}, strings.TrimSpace(rightExpr[1:len(rightExpr)-1]))
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return s.evalCallableAdverb(callable, adverb, right)
	}
	if collectionExpr, indexExpr, ok := findPostfixIndex(src); ok {
		if fn, ok := parseDyadicAdverbFunction(collectionExpr); ok {
			return s.applyCallableIndex(fn, indexExpr)
		}
		collection, err := s.eval(collectionExpr)
		if err != nil {
			return nil, err
		}
		if isCallable(collection) {
			return s.applyCallableIndex(collection, indexExpr)
		}
		index, err := s.eval(indexExpr)
		if err != nil {
			return nil, err
		}
		return indexValue(collection, index)
	}
	if collectionExpr, symbolExpr, ok := findPostfixSymbolLookup(src); ok {
		collection, err := s.eval(collectionExpr)
		if err != nil {
			return nil, err
		}
		key, err := s.eval(symbolExpr)
		if err != nil {
			return nil, err
		}
		return indexValue(collection, key)
	}
	if lambdaSrc, adverb, ok := findLambdaAdverbFunction(src); ok {
		return qCallableAdverb{
			fn:     qLambda{body: strings.TrimSpace(lambdaSrc[1 : len(lambdaSrc)-1]), env: cloneEnv(s.env), namespace: s.namespace},
			adverb: adverb,
		}, nil
	}
	if callableExpr, adverb, ok := s.findCallableAdverbFunction(src); ok {
		callable, err := s.eval(callableExpr)
		if err != nil {
			return nil, err
		}
		if !isCallable(callable) {
			return nil, fmt.Errorf("left side of callable adverb is not callable")
		}
		return qCallableAdverb{fn: callable, adverb: adverb}, nil
	}
	if lambdaSrc, adverb, rightExpr, ok := findLambdaAdverb(src); ok {
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return s.evalCallableAdverb(qLambda{body: strings.TrimSpace(lambdaSrc[1 : len(lambdaSrc)-1]), env: cloneEnv(s.env), namespace: s.namespace}, adverb, right)
	}
	if enclosed(src, '{', '}') {
		return qLambda{body: strings.TrimSpace(src[1 : len(src)-1]), env: cloneEnv(s.env), namespace: s.namespace}, nil
	}
	if strings.HasPrefix(src, "?") {
		v, err := s.eval(strings.TrimSpace(src[1:]))
		if err != nil {
			return nil, err
		}
		return distinct(v)
	}
	if strings.HasPrefix(src, "lookup ") {
		return s.evalLookup(strings.TrimSpace(src[len("lookup "):]))
	}
	if strings.HasPrefix(src, "get ") {
		return s.evalGet(strings.TrimSpace(src[len("get "):]))
	}
	if strings.HasPrefix(src, "hopen ") {
		return s.evalHopen(strings.TrimSpace(src[len("hopen "):]))
	}
	if leftExpr, rightExpr, ok := splitTopLevelWord(src, "fby"); ok {
		return s.evalFby(leftExpr, rightExpr)
	}
	if strings.HasPrefix(src, "+/") {
		if strings.TrimSpace(src[2:]) == "" {
			return qAdverbFunction{verb: "+", adverb: "/"}, nil
		}
		v, err := s.eval(strings.TrimSpace(src[2:]))
		if err != nil {
			return nil, err
		}
		return sum(v)
	}
	if strings.HasPrefix(src, "+\\") {
		if strings.TrimSpace(src[2:]) == "" {
			return qAdverbFunction{verb: "+", adverb: "\\"}, nil
		}
		v, err := s.eval(strings.TrimSpace(src[2:]))
		if err != nil {
			return nil, err
		}
		return sums(v)
	}
	if fn, ok := lookupDyadicVerbFunc(src); ok {
		return qDyadicFunction{name: strings.TrimSpace(src), fn: fn}, nil
	}
	if fn, ok := lookupUnaryVerb(src); ok {
		return qUnaryFunction{name: strings.TrimSpace(src), fn: fn}, nil
	}
	if expr, ok := findAdverb(src); ok {
		return s.evalAdverb(expr)
	}
	if strings.HasPrefix(src, "flip ") {
		v, err := s.eval(strings.TrimSpace(src[len("flip "):]))
		if err != nil {
			return nil, err
		}
		return flip(v)
	}
	if strings.HasPrefix(src, "enlist ") {
		v, err := s.eval(strings.TrimSpace(src[len("enlist "):]))
		if err != nil {
			return nil, err
		}
		return data.NewAny([]any{v}), nil
	}
	if strings.HasPrefix(src, "til ") {
		return s.evalTil(strings.TrimSpace(src[len("til "):]))
	}
	if strings.HasPrefix(src, "take ") {
		return s.evalTake(strings.TrimSpace(src[len("take "):]))
	}
	if strings.HasPrefix(src, "drop ") {
		return s.evalDrop(strings.TrimSpace(src[len("drop "):]))
	}
	for _, prefix := range []struct {
		word string
		fn   func(any) (any, error)
	}{
		{"where ", where},
		{"not ", notValue},
		{"null ", nullValue},
		{"raze ", raze},
		{"keys ", keys},
		{"key ", keys},
		{"value ", value},
		{"cols ", cols},
		{"meta ", meta},
		{"attr ", attrValue},
		{"codes ", enumCodes},
		{"domain ", enumDomainValues},
		{"group ", group},
		{"ungroup ", ungroup},
		{"type ", typeOf},
		{"string ", stringValue},
		{"lower ", lowerValue},
		{"upper ", upperValue},
		{"count ", count},
		{"first ", first},
		{"last ", last},
		{"sum ", sum},
		{"avg ", avg},
		{"var ", varValue},
		{"dev ", devValue},
		{"med ", medValue},
		{"min ", minValue},
		{"max ", maxValue},
		{"prd ", prd},
		{"distinct ", distinct},
		{"reverse ", reverse},
		{"prior ", prev},
		{"prev ", prev},
		{"next ", nextValue},
		{"deltas ", deltas},
		{"fills ", fills},
		{"differ ", differ},
		{"ratios ", ratios},
		{"asc ", asc},
		{"desc ", desc},
		{"iasc ", iasc},
		{"idesc ", idesc},
		{"rank ", rank},
		{"neg ", negValue},
		{"abs ", absValue},
		{"exp ", expValue},
		{"reciprocal ", reciprocalValue},
		{"signum ", signumValue},
		{"floor ", floorValue},
		{"ceiling ", ceilingValue},
		{"sums ", sums},
		{"prds ", prds},
		{"mins ", mins},
		{"maxs ", maxs},
		{"avgs ", avgs},
		{"all ", allValue},
		{"any ", anyValue},
	} {
		if strings.HasPrefix(src, prefix.word) {
			arg := strings.TrimSpace(src[len(prefix.word):])
			if prefix.word == "where " {
				if out, ok, err := s.evalWhereCompare(arg); ok || err != nil {
					return out, err
				}
			}
			v, err := s.eval(arg)
			if err != nil {
				return nil, err
			}
			return prefix.fn(v)
		}
	}
	if kind, text, ok := parseTemporalToken(src); ok {
		return ParseTemporal(kind, text)
	}
	if looksLikeTemporalVector(src) {
		return parseAtomOrVector(src)
	}
	for _, op := range []string{"<>", "<=", ">="} {
		if leftExpr, rightExpr, ok := splitTopLevelOperator(src, op); ok {
			left, err := s.eval(leftExpr)
			if err != nil {
				return nil, err
			}
			right, err := s.eval(rightExpr)
			if err != nil {
				return nil, err
			}
			return applyCompositeDyadic(op, left, right)
		}
	}
	if leftExpr, rightExpr, ok := splitTopLevelOperator(src, "~"); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return matchValue(left, right), nil
	}
	if leftExpr, rightExpr, ok := splitTopLevelOperator(src, "?"); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return findValue(left, right)
	}
	if leftExpr, rightExpr, ok := findPostfixLookup(src); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return dictLookup(left, right)
	}
	for _, op := range []struct {
		word string
		fn   func(any, any) (any, error)
	}{
		{"bin", bin},
		{"binr", binr},
		{"xbar", xbar},
		{"xrank", xrank},
		{"msum", msum},
		{"mavg", mavg},
		{"mcount", mcount},
		{"mmin", mmin},
		{"mmax", mmax},
		{"xprev", xprev},
		{"xrank", xrank},
		{"xcols", xcols},
		{"xkey", xkey},
		{"xgroup", xgroup},
		{"xasc", xasc},
		{"xdesc", xdesc},
		{"rotate", rotateValue},
		{"plus", dyadicVerbFunc('+')},
		{"minus", dyadicVerbFunc('-')},
		{"times", dyadicVerbFunc('*')},
		{"divide", dyadicVerbFunc('%')},
		{"div", dyadicVerbFunc('d')},
		{"mod", modValue},
		{"fill", dyadicVerbFunc('^')},
		{"match", func(left, right any) (any, error) { return matchValue(left, right), nil }},
		{"like", likeValue},
		{"equal", dyadicVerbFunc('=')},
		{"equals", dyadicVerbFunc('=')},
		{"less", dyadicVerbFunc('<')},
		{"more", dyadicVerbFunc('>')},
		{"greater", dyadicVerbFunc('>')},
		{"min", dyadicVerbFunc('m')},
		{"max", dyadicVerbFunc('M')},
		{"wavg", wavg},
		{"left", dyadicVerbFunc('L')},
		{"right", dyadicVerbFunc('R')},
		{"within", within},
		{"in", membership},
		{"and", logicalAnd},
		{"or", logicalOr},
	} {
		if leftExpr, rightExpr, ok := splitTopLevelWord(src, op.word); ok {
			left, err := s.eval(leftExpr)
			if err != nil {
				return nil, err
			}
			right, err := s.eval(rightExpr)
			if err != nil {
				return nil, err
			}
			return op.fn(left, right)
		}
	}
	for _, op := range []struct {
		word string
		fn   func(any, any) (any, error)
	}{
		{"intersect", inter},
		{"except", except},
		{"union", union},
		{"inter", inter},
	} {
		if leftExpr, rightExpr, ok := splitTopLevelWord(src, op.word); ok {
			left, err := s.eval(leftExpr)
			if err != nil {
				return nil, err
			}
			right, err := s.eval(rightExpr)
			if err != nil {
				return nil, err
			}
			return op.fn(left, right)
		}
	}
	if hash := findTopLevel(src, "#"); hash >= 0 {
		if marker, ok := parseAttributeMarker(strings.TrimSpace(src[:hash])); ok {
			v, err := s.eval(strings.TrimSpace(src[hash+1:]))
			if err != nil {
				return nil, err
			}
			return attributeVector(marker, v)
		}
		n, err := parseTakeCount(strings.TrimSpace(src[:hash]))
		if err != nil {
			return nil, err
		}
		v, err := s.eval(strings.TrimSpace(src[hash+1:]))
		if err != nil {
			return nil, err
		}
		return take(n, v)
	}
	if underscore := findTopLevel(src, "_"); underscore >= 0 {
		left, err := s.eval(strings.TrimSpace(src[:underscore]))
		if err != nil {
			return nil, err
		}
		right, err := s.eval(strings.TrimSpace(src[underscore+1:]))
		if err != nil {
			return nil, err
		}
		return cutOrDrop(left, right)
	}
	if dollar := findTopLevel(src, "$"); dollar >= 0 {
		domain, err := s.evalCastDomain(strings.TrimSpace(src[:dollar]))
		if err != nil {
			return nil, err
		}
		values, err := s.eval(strings.TrimSpace(src[dollar+1:]))
		if err != nil {
			return nil, err
		}
		return castOrEnum(domain, values)
	}
	if bang := findTopLevel(src, "!"); bang >= 0 {
		keys, err := s.eval(strings.TrimSpace(src[:bang]))
		if err != nil {
			return nil, err
		}
		values, err := s.eval(strings.TrimSpace(src[bang+1:]))
		if err != nil {
			return nil, err
		}
		if table, ok, err := keyedTableByCount(keys, values); ok || err != nil {
			return table, err
		}
		if keyed, ok, err := keyedTable(keys, values); ok || err != nil {
			return keyed, err
		}
		return dict(keys, values)
	}
	if leftExpr, rightExpr, ok := findPostfixSymbolLookup(src); ok {
		left, err := s.eval(leftExpr)
		if err != nil {
			return nil, err
		}
		right, err := s.eval(rightExpr)
		if err != nil {
			return nil, err
		}
		return dictLookup(left, right)
	}
	if parts := splitTopLevelDelim(src, ';'); len(parts) > 1 {
		out := make([]any, len(parts))
		for i, part := range parts {
			v, err := s.eval(part)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return data.NewAny(out), nil
	}
	if strings.HasPrefix(src, "`:") {
		return data.Symbol(src[1:]), nil
	}
	if looksLikeLiteralVector(src) {
		return parseAtomOrVector(src)
	}
	if idx, op, ok := findDyadic(src); ok {
		left, err := s.eval(strings.TrimSpace(src[:idx]))
		if err != nil {
			return nil, err
		}
		right, err := s.eval(strings.TrimSpace(src[idx+1:]))
		if err != nil {
			return nil, err
		}
		return applyDyadic(op, left, right)
	}
	if strings.HasPrefix(src, "`") {
		syms, err := parseSymbolList(src)
		if err == nil {
			if len(syms) == 1 {
				return syms[0], nil
			}
			values := make([]string, len(syms))
			for i, sym := range syms {
				values[i] = string(sym)
			}
			return data.NewSymbols(values), nil
		}
	}
	if value, ok, err := s.evalNameVector(src); ok || err != nil {
		return value, err
	}
	if value, ok, err := s.evalParsedValueExpr(src); ok || err != nil {
		return value, err
	}
	return parseAtomOrVector(src)
}

func (s *EvalState) evalNameVector(src string) (any, bool, error) {
	parts := splitTopLevelFields(src)
	if len(parts) < 2 {
		return nil, false, nil
	}
	values := make([]any, len(parts))
	for i, part := range parts {
		if !isQAssignmentName(part) {
			return nil, false, nil
		}
		value, ok := s.lookupName(part)
		if !ok {
			return nil, false, nil
		}
		values[i] = value
	}
	array, err := evalValueVector(values)
	return array, true, err
}

func (s *EvalState) evalParsedValueExpr(src string) (any, bool, error) {
	expr, ok, err := parseValueExpr(src)
	if err != nil || !ok {
		return nil, false, nil
	}
	value, err := s.evalValueExpr(expr)
	if err != nil {
		if isUnsupportedEvalValueExpr(err) {
			return nil, false, nil
		}
		return nil, true, err
	}
	return value, true, nil
}

type unsupportedEvalValueExpr struct {
	expr Expr
}

func (e unsupportedEvalValueExpr) Error() string {
	return fmt.Sprintf("unsupported q value expression %T", e.expr)
}

func isUnsupportedEvalValueExpr(err error) bool {
	_, ok := err.(unsupportedEvalValueExpr)
	return ok
}

func (s *EvalState) evalValueExpr(expr Expr) (any, error) {
	switch x := expr.(type) {
	case Number:
		value, _, err := parseNumberOrBool(x.Text)
		return value, err
	case String:
		return x.Value, nil
	case Symbol:
		return data.Symbol(x.Name), nil
	case Bool:
		return x.Value, nil
	case Null:
		return data.NullValue, nil
	case Temporal:
		return parseQTemporal(x.Kind, x.Text)
	case TypedNull:
		return data.NullForKind(data.Kind(x.Kind)), nil
	case Ident:
		if value, ok := s.lookupName(x.Name); ok {
			return value, nil
		}
		if fn, ok := lookupUnaryVerb(x.Name); ok {
			return qUnaryFunction{name: x.Name, fn: fn}, nil
		}
		if fn, ok := lookupDyadicVerbFunc(x.Name); ok {
			return qDyadicFunction{name: x.Name, fn: fn}, nil
		}
		return nil, unsupportedEvalValueExpr{expr: expr}
	case Vector:
		values := make([]any, len(x.Items))
		for i, item := range x.Items {
			value, err := s.evalValueExpr(item)
			if err != nil {
				return nil, err
			}
			values[i] = value
		}
		return evalValueVector(values)
	case DictExpr:
		keys, err := s.evalValueExpr(x.Keys)
		if err != nil {
			return nil, err
		}
		values, err := s.evalValueExpr(x.Values)
		if err != nil {
			return nil, err
		}
		if table, ok, err := keyedTableByCount(keys, values); ok || err != nil {
			return table, err
		}
		if keyed, ok, err := keyedTable(keys, values); ok || err != nil {
			return keyed, err
		}
		return dict(keys, values)
	case IndexExpr:
		collection, err := s.evalValueExpr(x.Expr)
		if err != nil {
			return nil, err
		}
		index, err := s.evalValueExpr(x.Index)
		if err != nil {
			return nil, err
		}
		if isCallable(collection) {
			return s.applyCallable(collection, []any{index})
		}
		return indexValue(collection, index)
	case Binary:
		left, err := s.evalValueExpr(x.Left)
		if err != nil {
			return nil, err
		}
		right, err := s.evalValueExpr(x.Right)
		if err != nil {
			return nil, err
		}
		return evalValueBinary(x.Op, left, right)
	case Call:
		return s.evalValueCall(x)
	case Flip:
		frame, err := lowerStaticFrame(x)
		if err != nil {
			return nil, err
		}
		if len(x.Keys) == 0 {
			return frame, nil
		}
		keys := make([]data.Symbol, len(x.Keys))
		for i, column := range x.Keys {
			keys[i] = data.Symbol(column.Name)
		}
		return data.KeyBy(frame, keys...)
	default:
		return nil, unsupportedEvalValueExpr{expr: expr}
	}
}

func (s *EvalState) evalValueCall(expr Call) (any, error) {
	arg, err := s.evalValueExpr(expr.Arg)
	if err != nil {
		return nil, err
	}
	if fn, ok := lookupUnaryVerb(expr.Func); ok {
		return fn(arg)
	}
	if fn, ok := lookupDyadicVerbFunc(expr.Func); ok {
		args, err := vectorValues(arg)
		if err != nil {
			return nil, unsupportedEvalValueExpr{expr: expr}
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s expects 2 arguments, got %d", expr.Func, len(args))
		}
		return fn(args[0], args[1])
	}
	return nil, unsupportedEvalValueExpr{expr: expr}
}

func evalValueBinary(op string, left, right any) (any, error) {
	switch op {
	case "<>", "<=", ">=":
		return applyCompositeDyadic(op, left, right)
	case "~":
		return matchValue(left, right), nil
	}
	if op == "-" && isNumericZero(left) {
		if value, ok := negateTypedNumeric(right); ok {
			return value, nil
		}
	}
	if fn, ok := lookupDyadicVerbFunc(op); ok {
		return fn(left, right)
	}
	return nil, unsupportedEvalValueExpr{expr: Binary{Op: op}}
}

func isNumericZero(v any) bool {
	switch x := v.(type) {
	case int:
		return x == 0
	case int16:
		return x == 0
	case int32:
		return x == 0
	case int64:
		return x == 0
	case float32:
		return x == 0
	case float64:
		return x == 0
	default:
		return false
	}
}

func negateTypedNumeric(v any) (any, bool) {
	switch x := v.(type) {
	case int16:
		return -x, true
	case int32:
		return -x, true
	case float32:
		return -x, true
	default:
		return nil, false
	}
}

func evalValueVector(values []any) (data.Array, error) {
	kinds := make([]data.Kind, 0, len(values))
	temporalKinds := make([]data.Kind, len(values))
	hasTemporal := false
	for i, value := range values {
		kind := qKindOfValue(value)
		if kind != "" {
			kinds = append(kinds, kind)
		}
		if kind == "" {
			continue
		}
		if isTemporalDataKind(kind) {
			temporalKinds[i] = kind
			hasTemporal = true
		}
	}
	if hasTemporal {
		for i, value := range values {
			if data.IsNull(value) {
				continue
			}
			if temporalKindOfValue(value) == "" {
				return nil, fmt.Errorf("mixed temporal and non-temporal vectors are not supported")
			}
			if temporalKinds[i] == "" {
				temporalKinds[i] = temporalKindOfValue(value)
			}
		}
		kind, err := coerceTemporalVectorValues(values, temporalKinds)
		if err != nil {
			return nil, err
		}
		column, err := data.NewColumnWithKind("_", kind, values)
		if err != nil {
			return nil, err
		}
		return column.Data, nil
	}
	return inferQArray(values, kinds...), nil
}

func isTemporalDataKind(kind data.Kind) bool {
	switch kind {
	case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan,
		data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		return true
	default:
		return false
	}
}

func (s *EvalState) evalSystemCommand(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "\\") {
		return nil, false, nil
	}
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return nil, true, fmt.Errorf("empty q system command")
	}
	cmd := fields[0]
	switch cmd {
	case "\\p":
		if len(fields) == 1 {
			return s.port, true, nil
		}
		if len(fields) != 2 {
			return nil, true, fmt.Errorf(`q system command \p expects zero or one numeric port argument`)
		}
		port, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || port < 0 || port > 65535 {
			return nil, true, fmt.Errorf(`q system command \p expects a port in range 0..65535`)
		}
		s.port = port
		return s.port, true, nil
	case "\\v":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \v expects no arguments`)
		}
		return s.systemNames(qSystemVariables), true, nil
	case "\\f":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \f expects no arguments`)
		}
		return s.systemNames(qSystemFunctions), true, nil
	case "\\a":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \a expects no arguments`)
		}
		return s.systemNames(qSystemTables), true, nil
	case "\\b":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \b expects no arguments`)
		}
		return data.NewSymbols(nil), true, nil
	case "\\d":
		if len(fields) == 1 {
			return data.Symbol(s.currentNamespace()), true, nil
		}
		if len(fields) != 2 {
			return nil, true, fmt.Errorf(`q system command \d expects zero or one namespace argument`)
		}
		if !isQNamespaceName(fields[1]) {
			return nil, true, fmt.Errorf(`q system command \d expects . or a single-level namespace like .foo`)
		}
		s.namespace = fields[1]
		return data.Symbol(s.currentNamespace()), true, nil
	case "\\w":
		if len(fields) != 1 {
			return nil, true, fmt.Errorf(`q system command \w expects no arguments`)
		}
		return systemWorkspaceStats(), true, nil
	case "\\l":
		if len(fields) != 2 {
			return nil, true, fmt.Errorf(`q system command \l expects one file path argument`)
		}
		out, err := s.loadScriptFile(fields[1])
		return out, true, err
	default:
		return nil, true, fmt.Errorf("unsupported q system command %q", cmd)
	}
}

func (s *EvalState) loadScriptFile(arg string) (any, error) {
	file := strings.TrimSpace(arg)
	if path, ok := qPathLiteralString(file); ok {
		file = path
	} else if strings.HasPrefix(file, "\"") && strings.HasSuffix(file, "\"") && len(file) >= 2 {
		file = strings.Trim(file, "\"")
	}
	if file == "" || strings.HasPrefix(file, ":") {
		return nil, fmt.Errorf(`q system command \l expects a local file path`)
	}
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf(`q system command \l %q: %w`, file, err)
	}
	return s.evalScript(strings.TrimSpace(string(src)))
}

type qSystemNameKind int

const (
	qSystemVariables qSystemNameKind = iota
	qSystemFunctions
	qSystemTables
)

func (s *EvalState) systemNames(kind qSystemNameKind) data.Array {
	names := make([]string, 0, len(s.env))
	for name, value := range s.env {
		if s.currentNamespace() == "." {
			if strings.HasPrefix(name, ".") {
				continue
			}
		} else {
			prefix := s.currentNamespace() + "."
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			name = strings.TrimPrefix(name, prefix)
		}
		switch kind {
		case qSystemVariables:
			if isCallable(value) || isTableLikeValue(value) {
				continue
			}
		case qSystemFunctions:
			if !isCallable(value) {
				continue
			}
		case qSystemTables:
			if !isTableLikeValue(value) {
				continue
			}
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return data.NewSymbols(names)
}

func isTableLikeValue(v any) bool {
	switch v.(type) {
	case data.Frame, data.KeyedFrame:
		return true
	default:
		return false
	}
}

func systemWorkspaceStats() data.Array {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return data.NewI64([]int64{
		uint64ToInt64(stats.Alloc),
		uint64ToInt64(stats.TotalAlloc),
		uint64ToInt64(stats.Sys),
		uint64ToInt64(uint64(stats.NumGC)),
	})
}

func uint64ToInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

func (s *EvalState) evalTableLiteral(src string) (any, bool, error) {
	if !looksLikeTableLiteral(src) {
		return nil, false, nil
	}
	expr, ok, err := parseValueExpr(src)
	if err != nil || !ok {
		return nil, ok, err
	}
	flipExpr, ok := expr.(Flip)
	if !ok {
		return nil, false, nil
	}
	frame, err := lowerStaticFrame(flipExpr)
	if err != nil {
		return nil, true, err
	}
	if len(flipExpr.Keys) == 0 {
		return frame, true, nil
	}
	keys := make([]data.Symbol, len(flipExpr.Keys))
	for i, column := range flipExpr.Keys {
		keys[i] = data.Symbol(column.Name)
	}
	keyed, err := data.KeyBy(frame, keys...)
	if err != nil {
		return nil, true, fmt.Errorf("keyed table literal: %w", err)
	}
	return keyed, true, nil
}

func looksLikeTableLiteral(src string) bool {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "(") {
		return false
	}
	for i := 1; i < len(src); i++ {
		if isQWhitespace(src[i]) {
			continue
		}
		return src[i] == '['
	}
	return false
}

func parseValueExpr(src string) (Expr, bool, error) {
	tokens, err := lex(src)
	if err != nil {
		return nil, true, err
	}
	p := parser{tokens: tokens}
	expr, err := p.parseExpr(0)
	if err != nil {
		return nil, true, err
	}
	if p.peek().kind != tokenEOF {
		return nil, false, nil
	}
	return expr, true, nil
}

func evalZNamespaceValue(src string) (any, bool, error) {
	if !strings.HasPrefix(src, ".z.") {
		return nil, false, nil
	}
	now := time.Now()
	utc := now.UTC()
	switch src {
	case ".z.P":
		return data.TimestampFromUnixNanos(utc.UnixNano()), true, nil
	case ".z.p":
		return data.TimestampFromUnixNanos(civilUnixNanos(now)), true, nil
	case ".z.D":
		return dateFromCivil(utc), true, nil
	case ".z.d":
		return dateFromCivil(now), true, nil
	case ".z.T":
		return timeFromCivil(utc), true, nil
	case ".z.t":
		return timeFromCivil(now), true, nil
	case ".z.Z":
		return data.DateTimeFromUnixNanos(civilUnixNanos(utc)), true, nil
	case ".z.z":
		return data.DateTimeFromUnixNanos(civilUnixNanos(now)), true, nil
	case ".z.i":
		return int64(os.Getpid()), true, nil
	case ".z.K":
		return int64(4), true, nil
	case ".z.k":
		return "leia-q", true, nil
	case ".z.o":
		return "leia", true, nil
	case ".z.q":
		return data.Symbol("leia"), true, nil
	default:
		return nil, true, fmt.Errorf("unsupported q .z namespace value %q", src)
	}
}

func dateFromCivil(t time.Time) data.Date {
	date := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return data.DateFromDays(date.Unix() / 86400)
}

func timeFromCivil(t time.Time) data.Time {
	return data.TimeFromNanos(int64(t.Hour())*3600*1_000_000_000 +
		int64(t.Minute())*60*1_000_000_000 +
		int64(t.Second())*1_000_000_000 +
		int64(t.Nanosecond()))
}

func civilUnixNanos(t time.Time) int64 {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).UnixNano()
}

func (s *EvalState) evalHopen(src string) (any, error) {
	target, err := s.eval(src)
	if err != nil {
		return nil, err
	}
	targetName, ok := qIPCTargetName(target)
	if !ok {
		return nil, fmt.Errorf("hopen expects a loopback target string or symbol")
	}
	switch targetName {
	case "loopback", ":loopback", "local", ":local", "memory", ":memory":
		return &qIPCHandle{
			target:  "loopback",
			session: NewEvalState(nil),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported q IPC target %q; only in-process loopback is supported", targetName)
	}
}

func (s *EvalState) evalGet(src string) (any, error) {
	path, ok := qPathLiteralString(src)
	if !ok {
		value, err := s.eval(src)
		if err != nil {
			return nil, err
		}
		path, ok = qPathString(value)
		if !ok {
			return nil, fmt.Errorf("get expects a q path symbol like `:/path")
		}
	}
	frame, err := data.LoadPartitionedFrameDir(path, nil)
	if err != nil {
		return nil, fmt.Errorf("get %q: %w", path, err)
	}
	return frame, nil
}

func (s *EvalState) evalSetPrefix(src string) (any, error) {
	pathExpr, valueExpr, ok := splitLeadingSetArg(src)
	if !ok {
		return nil, fmt.Errorf("set expects a q path symbol and frame value")
	}
	return s.evalSet(pathExpr, valueExpr)
}

func (s *EvalState) evalSet(pathExpr, valueExpr string) (any, error) {
	path, ok := qPathLiteralString(pathExpr)
	if !ok {
		value, err := s.eval(pathExpr)
		if err != nil {
			return nil, err
		}
		path, ok = qPathString(value)
		if !ok {
			return nil, fmt.Errorf("set expects a q path symbol like `:/path")
		}
	}
	value, err := s.eval(valueExpr)
	if err != nil {
		return nil, err
	}
	frame, ok := qStorageFrame(value)
	if !ok {
		return nil, fmt.Errorf("set %q expects a table value", path)
	}
	if err := data.SaveFrameDir(path, frame); err != nil {
		return nil, fmt.Errorf("set %q: %w", path, err)
	}
	return value, nil
}

func qStorageFrame(value any) (data.Frame, bool) {
	switch x := value.(type) {
	case data.Frame:
		return x, true
	case data.KeyedFrame:
		return x.Frame(), true
	default:
		return data.Frame{}, false
	}
}

func qPathLiteralString(src string) (string, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "`:") || strings.Contains(src[1:], "`") {
		return "", false
	}
	path := strings.TrimPrefix(src[1:], ":")
	if path == "" {
		return "", false
	}
	return path, true
}

func splitPathStyleSet(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "`:") {
		end := qSymbolLiteralEnd(src, 0)
		if end <= 0 || end >= len(src) {
			return "", "", false
		}
		rest := strings.TrimSpace(src[end:])
		if strings.HasPrefix(rest, "set") && wordBoundary(rest, 0, len("set")) {
			right := strings.TrimSpace(rest[len("set"):])
			if right != "" {
				return strings.TrimSpace(src[:end]), right, true
			}
		}
		return "", "", false
	}
	return splitTopLevelWord(src, "set")
}

func splitLeadingSetArg(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", "", false
	}
	end := 0
	switch {
	case strings.HasPrefix(src, "`"):
		end = qSymbolLiteralEnd(src, 0)
	case src[0] == '(':
		end = findMatchingDelimiter(src, 0, '(', ')') + 1
	default:
		for end < len(src) && !isQWhitespace(src[end]) {
			end++
		}
	}
	if end <= 0 || end >= len(src) {
		return "", "", false
	}
	right := strings.TrimSpace(src[end:])
	return strings.TrimSpace(src[:end]), right, right != ""
}

func qPathString(v any) (string, bool) {
	sym, ok := v.(data.Symbol)
	if !ok {
		return "", false
	}
	text := string(sym)
	if !strings.HasPrefix(text, ":") {
		return "", false
	}
	path := strings.TrimPrefix(text, ":")
	if path == "" {
		return "", false
	}
	return path, true
}

func isQWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func qIPCTargetName(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case data.Symbol:
		return string(x), true
	default:
		return "", false
	}
}

func enclosed(src string, open, close byte) bool {
	src = strings.TrimSpace(src)
	if len(src) < 2 || src[0] != open || src[len(src)-1] != close {
		return false
	}
	depth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == open {
			depth++
		} else if ch == close {
			depth--
			if depth == 0 && i != len(src)-1 {
				return false
			}
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func splitTopLevelDelim(src string, sep byte) []string {
	var parts []string
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	start := 0
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case sep:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				parts = append(parts, strings.TrimSpace(src[start:i]))
				start = i + 1
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	parts = append(parts, strings.TrimSpace(src[start:]))
	return parts
}

func splitTopLevel(src string, sep byte) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(src[start:i]))
				start = i + 1
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	parts = append(parts, strings.TrimSpace(src[start:]))
	return parts
}

func splitTopLevelAssignment(src string) (string, string, bool) {
	colon := findTopLevel(src, ":")
	if colon < 0 {
		return "", "", false
	}
	name := strings.TrimSpace(src[:colon])
	if !isQAssignmentName(name) {
		return "", "", false
	}
	rhs := strings.TrimSpace(src[colon+1:])
	return name, rhs, rhs != ""
}

func expandScriptVars(src string, vars map[string]any) string {
	if len(vars) == 0 {
		return strings.TrimSpace(src)
	}
	var out strings.Builder
	for i := 0; i < len(src); {
		switch src[i] {
		case '"':
			end := i + 1
			for end < len(src) {
				if src[end] == '\\' {
					end += 2
					continue
				}
				if src[end] == '"' {
					end++
					break
				}
				end++
			}
			out.WriteString(src[i:end])
			i = end
		case '`':
			end := i + 1
			for end < len(src) && !isQSymbolDelimiter(src[end]) {
				end++
			}
			out.WriteString(src[i:end])
			i = end
		case '{':
			end := findMatchingDelimiter(src, i, '{', '}')
			if end < 0 {
				out.WriteByte(src[i])
				i++
				continue
			}
			out.WriteString(src[i : end+1])
			i = end + 1
		default:
			if isQIdentStart(src[i]) && (i == 0 || src[i-1] != '.') {
				end := i + 1
				for end < len(src) && isQIdentRest(src[end]) {
					end++
				}
				name := src[i:end]
				if v, ok := vars[name]; ok {
					if isCallable(v) {
						out.WriteString(name)
					} else {
						out.WriteString(qExprLiteral(v))
					}
				} else {
					out.WriteString(name)
				}
				i = end
				continue
			}
			out.WriteByte(src[i])
			i++
		}
	}
	return strings.TrimSpace(out.String())
}

func findTopLevel(src string, chars string) int {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		default:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && strings.ContainsRune(chars, rune(ch)) && !isSign(src, i) {
				return i
			}
		}
	}
	return -1
}

func qSymbolLiteralEnd(src string, start int) int {
	if start < 0 || start >= len(src) || src[start] != '`' {
		return start + 1
	}
	pos := start + 1
	if pos < len(src) && src[pos] == ':' {
		pos++
		for pos < len(src) && !isQPathSymbolDelimiter(src[pos]) {
			pos++
		}
		return pos
	}
	for pos < len(src) && !isQSymbolDelimiter(src[pos]) {
		pos++
	}
	return pos
}

func splitTopLevelWord(src, word string) (string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i+len(word) <= len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		default:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && strings.HasPrefix(src[i:], word) && wordBoundary(src, i, len(word)) {
				left := strings.TrimSpace(src[:i])
				right := strings.TrimSpace(src[i+len(word):])
				return left, right, left != "" && right != ""
			}
		}
	}
	return "", "", false
}

func wordBoundary(src string, start, length int) bool {
	before := start == 0 || !isQIdentRest(src[start-1])
	afterAt := start + length
	after := afterAt >= len(src) || !isQIdentRest(src[afterAt])
	return before && after
}

func splitTopLevelOperator(src, op string) (string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i <= len(src)-len(op); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		default:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && strings.HasPrefix(src[i:], op) {
				left := strings.TrimSpace(src[:i])
				right := strings.TrimSpace(src[i+len(op):])
				return left, right, left != "" && right != ""
			}
		}
	}
	return "", "", false
}

func findPostfixSymbolLookup(src string) (string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	candidate := -1
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '`':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && i > 0 {
				left := strings.TrimSpace(src[:i])
				if left == "" || strings.HasPrefix(left, "`") || isDyadicOp(left[len(left)-1]) || findTopLevel(left, "!") >= 0 || qPrefixExprTakesSymbolArgument(left) || qLeftLooksLikePrefixArgument(left) || !qCanReceivePostfixSymbolLookup(left) {
					continue
				}
				candidate = i
				i = len(src)
			}
		}
	}
	if candidate < 0 {
		return "", "", false
	}
	collection := strings.TrimSpace(src[:candidate])
	symbols := strings.TrimSpace(src[candidate:])
	if collection == "" || symbols == "" || !strings.HasPrefix(symbols, "`") {
		return "", "", false
	}
	if _, err := parseSymbolList(symbols); err != nil {
		return "", "", false
	}
	return collection, symbols, true
}

func qLeftLooksLikePrefixArgument(left string) bool {
	left = strings.TrimSpace(left)
	return strings.HasSuffix(left, " flip") || strings.HasSuffix(left, "!flip") || strings.HasSuffix(left, " enlist") || strings.HasSuffix(left, "!enlist")
}

func qCanReceivePostfixSymbolLookup(left string) bool {
	left = strings.TrimSpace(left)
	if left == "" {
		return false
	}
	if strings.HasSuffix(left, ")") || strings.HasSuffix(left, "]") {
		return true
	}
	return isQAssignmentName(left)
}

func qPrefixExprTakesSymbolArgument(left string) bool {
	left = strings.TrimSpace(left)
	if strings.HasPrefix(left, "flip ") || strings.HasPrefix(left, "enlist ") {
		return true
	}
	first := left
	if fields := strings.Fields(left); len(fields) > 0 {
		first = fields[0]
	}
	switch first {
	case "flip", "enlist", "keys", "key", "value", "cols", "meta", "attr", "type", "count",
		"where", "domain", "codes", "lookup", "hopen", "hsym", "system",
		"group", "string", "not", "iasc", "idesc", "rank", "asc", "desc", "min", "max",
		"sum", "avg", "sums", "prds", "mins", "maxs", "avgs", "distinct", "reverse", "prior", "prev", "next", "deltas", "fills", "differ",
		"first", "last", "take", "drop", "til", "lower", "upper", "null", "get":
		return true
	default:
		return false
	}
}

func findPostfixIndex(src string) (string, string, bool) {
	if !strings.HasSuffix(src, "]") {
		return "", "", false
	}
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	candidate := -1
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				candidate = i
			}
			bracketDepth++
		case ']':
			bracketDepth--
			if i == len(src)-1 && parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && candidate >= 0 {
				collection := strings.TrimSpace(src[:candidate])
				index := strings.TrimSpace(src[candidate+1 : len(src)-1])
				if collection == "" || !qCanReceivePostfixIndex(collection) {
					return "", "", false
				}
				return collection, index, index != "" || strings.Contains(src[candidate+1:len(src)-1], ";")
			}
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		}
	}
	return "", "", false
}

func qCanReceivePostfixIndex(collection string) bool {
	collection = strings.TrimSpace(collection)
	if collection == "" {
		return false
	}
	if _, _, ok := findDyadic(collection); ok {
		return false
	}
	if isQAssignmentName(collection) {
		return true
	}
	if _, ok := lookupUnaryVerb(collection); ok {
		return true
	}
	if _, ok := lookupDyadicVerbFunc(collection); ok {
		return true
	}
	if _, ok := parseDyadicAdverbFunction(collection); ok {
		return true
	}
	if qCanReceivePostfixCallableAdverbIndex(collection) {
		return true
	}
	if strings.HasPrefix(collection, "{") && strings.HasSuffix(collection, "}") {
		return true
	}
	return strings.HasSuffix(collection, ")") || strings.HasSuffix(collection, "]")
}

func qCanReceivePostfixCallableAdverbIndex(collection string) bool {
	for _, adverb := range []string{"\\:", "/:", "':", "'", "/", "\\"} {
		if !strings.HasSuffix(collection, adverb) {
			continue
		}
		base := strings.TrimSpace(collection[:len(collection)-len(adverb)])
		if base == "" {
			continue
		}
		if strings.HasPrefix(base, "{") && strings.HasSuffix(base, "}") {
			return true
		}
		if isQAssignmentName(base) {
			return true
		}
		if strings.HasSuffix(base, ")") || strings.HasSuffix(base, "]") {
			return true
		}
		if _, ok := lookupDyadicVerbFunc(base); ok {
			return true
		}
		if _, ok := lookupUnaryVerb(base); ok {
			return true
		}
	}
	return false
}

func findMatchingDelimiter(src string, start int, open, close byte) int {
	if start < 0 || start >= len(src) || src[start] != open {
		return -1
	}
	depth := 0
	inString := false
	for i := start; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isQAssignmentName(name string) bool {
	if isQBareName(name) {
		return true
	}
	if !strings.HasPrefix(name, ".") {
		return false
	}
	parts := strings.Split(name[1:], ".")
	if len(parts) != 2 {
		return false
	}
	return isQBareName(parts[0]) && isQBareName(parts[1])
}

func isQNamespaceName(name string) bool {
	if name == "." {
		return true
	}
	if !strings.HasPrefix(name, ".") {
		return false
	}
	return isQBareName(strings.TrimPrefix(name, "."))
}

func isQBareName(name string) bool {
	if name == "" || !isQIdentStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isQIdentRest(name[i]) {
			return false
		}
	}
	return true
}

func parseAttributeMarker(src string) (data.Symbol, bool) {
	src = strings.TrimSpace(src)
	if len(src) == 2 && src[0] == '`' && isQAttributeMarker(src[1]) {
		return data.Symbol(src[1:]), true
	}
	return "", false
}

func isQAttributeMarker(ch byte) bool {
	switch ch {
	case 's', 'g', 'p', 'u':
		return true
	default:
		return false
	}
}

func attributeVector(marker data.Symbol, v any) (qAttributedVector, error) {
	array, ok := v.(data.Array)
	if !ok {
		return qAttributedVector{}, fmt.Errorf("q attribute marker `%s expects a vector", marker)
	}
	return qAttributedVector{attribute: marker, vector: data.WithArrayAttribute(array, marker)}, nil
}

func (v qAttributedVector) Kind() data.Kind {
	return v.vector.Kind()
}

func (v qAttributedVector) Len() int {
	return v.vector.Len()
}

func (v qAttributedVector) At(row int) (any, bool) {
	return v.vector.At(row)
}

func (v qAttributedVector) Values() []any {
	return v.vector.Values()
}

func (v qAttributedVector) Gather(indexes []int) data.Array {
	return qAttributedVector{attribute: v.attribute, vector: v.vector.Gather(indexes)}
}

func (v qAttributedVector) ArrayMetadata() data.ArrayMetadata {
	metadata := data.ArrayMetadataOf(v.vector)
	if !metadata.HasAttribute(v.attribute) {
		return data.ArrayMetadataOf(data.WithArrayAttribute(v.vector, v.attribute))
	}
	return metadata
}

func castOrEnum(domain any, values any) (any, error) {
	if sym, ok := domain.(data.Symbol); ok {
		if kind, ok := qCastKindFromSymbol(sym); ok {
			return castValue(kind, values)
		}
	}
	return enumCast(domain, values)
}

func qCastKindFromSymbol(sym data.Symbol) (data.Kind, bool) {
	switch strings.ToLower(string(sym)) {
	case "b", "bool", "boolean":
		return data.KindBool, true
	case "x", "byte":
		return data.KindU8, true
	case "h", "short":
		return data.KindI16, true
	case "i", "int":
		return data.KindI32, true
	case "j", "long":
		return data.KindI64, true
	case "e", "real":
		return data.KindF32, true
	case "f", "float":
		return data.KindF64, true
	case "s", "symbol":
		return data.KindSymbol, true
	case "c", "char", "string":
		return data.KindString, true
	case "m", "month":
		return data.KindMonth, true
	case "d", "date":
		return data.KindDate, true
	case "z", "datetime":
		return data.KindDateTime, true
	case "n", "timespan":
		return data.KindTimespan, true
	case "u", "minute":
		return data.KindMinute, true
	case "v", "second":
		return data.KindSecond, true
	case "t", "time":
		return data.KindTime, true
	case "p", "timestamp":
		return data.KindTimestamp, true
	default:
		return "", false
	}
}

func castValue(kind data.Kind, values any) (any, error) {
	if array, ok := values.(data.Array); ok {
		out := array.Values()
		for i, value := range out {
			normalized, err := castScalarValue(kind, value)
			if err != nil {
				return nil, fmt.Errorf("q cast `%s value %d: %w", kind, i+1, err)
			}
			out[i] = normalized
		}
		column, err := data.NewColumnWithKind("_", kind, out)
		if err != nil {
			return nil, fmt.Errorf("q cast `%s: %w", kind, err)
		}
		return column.Data, nil
	}
	normalized, err := castScalarValue(kind, values)
	if err != nil {
		return nil, fmt.Errorf("q cast `%s: %w", kind, err)
	}
	return normalized, nil
}

func castScalarValue(kind data.Kind, value any) (any, error) {
	if data.IsNull(value) {
		return data.NullForKind(kind), nil
	}
	if temporalKind, ok := qTemporalCastKindName(kind); ok {
		if text, ok := value.(string); ok {
			return parseQTemporal(temporalKind, text)
		}
	}
	if kind == data.KindString {
		switch x := value.(type) {
		case data.Symbol:
			return string(x), nil
		default:
			return data.NormalizeValueForKind(kind, value)
		}
	}
	if kind == data.KindBool {
		switch x := value.(type) {
		case bool:
			return x, nil
		case int64:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		case int32:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		case int16:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		case int8:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		case uint8:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		}
	}
	return data.NormalizeValueForKind(kind, value)
}

func (s *EvalState) evalCastDomain(src string) (any, error) {
	value, err := s.eval(src)
	if err == nil {
		return value, nil
	}
	if isBareQCastName(src) {
		return data.Symbol(src), nil
	}
	return nil, err
}

func isBareQCastName(src string) bool {
	if !isQAssignmentName(src) {
		return false
	}
	_, ok := qCastKindFromSymbol(data.Symbol(src))
	return ok
}

func qTemporalCastKindName(kind data.Kind) (string, bool) {
	switch kind {
	case data.KindMonth:
		return "month", true
	case data.KindDate:
		return "date", true
	case data.KindDateTime:
		return "datetime", true
	case data.KindTimespan:
		return "timespan", true
	case data.KindMinute:
		return "minute", true
	case data.KindSecond:
		return "second", true
	case data.KindTime:
		return "time", true
	case data.KindTimestamp:
		return "timestamp", true
	default:
		return "", false
	}
}

func enumCast(domain any, values any) (qEnumVector, error) {
	sym, ok := domain.(data.Symbol)
	if !ok {
		return qEnumVector{}, fmt.Errorf("q enum cast expects a symbol domain")
	}
	var symbols []data.Symbol
	switch x := values.(type) {
	case qEnumVector:
		if x.domain == sym {
			return x, nil
		}
		for _, value := range x.Values() {
			symbol, ok := value.(data.Symbol)
			if !ok {
				return qEnumVector{}, fmt.Errorf("q enum cast `%s expects symbol values", sym)
			}
			symbols = append(symbols, symbol)
		}
	case data.Symbol:
		symbols = []data.Symbol{x}
	case data.Array:
		if x.Kind() != data.KindSymbol {
			return qEnumVector{}, fmt.Errorf("q enum cast `%s expects symbol values", sym)
		}
		values := x.Values()
		symbols = make([]data.Symbol, len(values))
		for i, value := range values {
			symbol, ok := value.(data.Symbol)
			if !ok {
				return qEnumVector{}, fmt.Errorf("q enum cast `%s expects symbol values", sym)
			}
			symbols[i] = symbol
		}
	default:
		return qEnumVector{}, fmt.Errorf("q enum cast `%s expects symbol values", sym)
	}
	return qEnumVector{domain: sym, encoded: data.NewEncodedSymbols(symbols)}, nil
}

func (v qEnumVector) Kind() data.Kind {
	return v.encoded.Kind()
}

func (v qEnumVector) Len() int {
	return v.encoded.Len()
}

func (v qEnumVector) At(row int) (any, bool) {
	return v.encoded.At(row)
}

func (v qEnumVector) Values() []any {
	return v.encoded.Values()
}

func (v qEnumVector) Gather(indexes []int) data.Array {
	return qEnumVector{domain: v.domain, encoded: v.encoded.Gather(indexes)}
}

func (v qEnumVector) decodedArray() data.Array {
	return data.InferArray(v.Values())
}

func (v qEnumVector) EncodedDomain() []any {
	if domain, ok := data.EncodedDomainOf(v.encoded); ok {
		return domain
	}
	return v.Values()
}

func (v qEnumVector) EncodedCodes() []int32 {
	if codes, ok := data.EncodedCodesOf(v.encoded); ok {
		return codes
	}
	codes := make([]int32, v.Len())
	for i := range codes {
		codes[i] = int32(i)
	}
	return codes
}

func parseDyadicAdverbFunction(src string) (qAdverbFunction, bool) {
	src = strings.TrimSpace(src)
	body := src
	if enclosed(src, '(', ')') {
		body = strings.TrimSpace(src[1 : len(src)-1])
	}
	for _, adverb := range []string{"':", "\\:", "/:", "'", "/", "\\"} {
		if strings.HasSuffix(body, adverb) {
			verb := strings.TrimSpace(body[:len(body)-len(adverb)])
			if verb == "" {
				return qAdverbFunction{}, false
			}
			if _, ok := lookupDyadicVerbFunc(verb); ok {
				return qAdverbFunction{verb: verb, adverb: adverb}, true
			}
		}
	}
	return qAdverbFunction{}, false
}

func isQIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isQIdentRest(ch byte) bool {
	return isQIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func isQSymbolDelimiter(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '(', ')', '[', ']', ';', ',', '!', '+', '-', '/', '=', '<', '>', '#', '$':
		return true
	default:
		return false
	}
}

func isQPathSymbolDelimiter(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '(', ')', '[', ']', ';', ',':
		return true
	default:
		return false
	}
}

func qExprLiteral(v any) string {
	if data.IsNull(v) {
		return "0N"
	}
	if array, ok := v.(data.Array); ok {
		items := make([]string, 0, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, _ := array.At(i)
			items = append(items, qExprLiteral(item))
		}
		switch array.Kind() {
		case data.KindSymbol:
			return strings.Join(items, "")
		case data.KindAny:
			return "(" + strings.Join(items, ";") + ")"
		}
		return strings.Join(items, " ")
	}
	if dict, ok := v.(EvalDict); ok {
		keys := make([]string, len(dict.Keys))
		for i, key := range dict.Keys {
			keys[i] = qExprLiteral(key)
		}
		values := make([]string, len(dict.Values))
		for i, value := range dict.Values {
			values[i] = qExprLiteral(value)
		}
		keySep := " "
		if len(keys) > 0 && strings.HasPrefix(keys[0], "`") {
			keySep = ""
		}
		valueSrc := strings.Join(values, " ")
		if len(values) > 1 {
			valueSrc = "(" + strings.Join(values, ";") + ")"
		}
		return "(" + strings.Join(keys, keySep) + "!" + valueSrc + ")"
	}
	if s, ok := FormatTemporal(v); ok {
		return s
	}
	switch x := v.(type) {
	case data.Symbol:
		return "`" + string(x)
	case string:
		return strconv.Quote(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int8, int16, int32, int64, float32, float64:
		return fmt.Sprint(x)
	default:
		return fmt.Sprint(x)
	}
}

func findPostfixLookup(src string) (string, string, bool) {
	if !strings.HasPrefix(src, "(") {
		return "", "", false
	}
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				key := strings.TrimSpace(src[i+1:])
				if key == "" {
					return "", "", false
				}
				switch {
				case strings.HasPrefix(key, "[") && strings.HasSuffix(key, "]") && enclosed(key, '[', ']'):
					key = strings.TrimSpace(key[1 : len(key)-1])
				case strings.HasPrefix(key, "`"):
				default:
					return "", "", false
				}
				return strings.TrimSpace(src[:i+1]), key, true
			}
		}
	}
	return "", "", false
}

func isSign(src string, i int) bool {
	if i+1 >= len(src) || !isQNumericSignStart(src[i+1]) {
		return false
	}
	if i == 0 {
		return true
	}
	switch src[i-1] {
	case ' ', '\t', '\n', '(', ';':
		return true
	default:
		return false
	}
}

func isQNumericSignStart(ch byte) bool {
	return (ch >= '0' && ch <= '9') || ch == '.'
}

func findDyadic(src string) (int, byte, bool) {
	for _, ops := range []string{"=<>", "+-", "*%", "&|", "^"} {
		if idx := findTopLevel(src, ops); idx >= 0 {
			return idx, src[idx], true
		}
	}
	return 0, 0, false
}

type adverbExpr struct {
	left   string
	verb   string
	adverb string
	right  string
}

func findAdverb(src string) (adverbExpr, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		if inString {
			if src[i] == '\\' {
				i++
				continue
			}
			if src[i] == '"' {
				inString = false
			}
			continue
		}
		switch src[i] {
		case '"':
			inString = true
			continue
		case '(':
			parenDepth++
			continue
		case ')':
			parenDepth--
			continue
		case '[':
			bracketDepth++
			continue
		case ']':
			bracketDepth--
			continue
		case '{':
			braceDepth++
			continue
		case '}':
			braceDepth--
			continue
		}
		if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 {
			continue
		}
		adverb := ""
		switch {
		case strings.HasPrefix(src[i:], "\\:"):
			adverb = "\\:"
		case strings.HasPrefix(src[i:], "/:"):
			adverb = "/:"
		case strings.HasPrefix(src[i:], "':"):
			adverb = "':"
		case src[i] == '/':
			adverb = "/"
		case src[i] == '\\':
			adverb = "\\"
		case src[i] == '\'':
			adverb = "'"
		default:
			continue
		}
		verbStart, verbEnd, ok := adverbVerbBounds(src, i)
		if !ok {
			continue
		}
		left := strings.TrimSpace(src[:verbStart])
		right := strings.TrimSpace(src[i+len(adverb):])
		if right == "" {
			continue
		}
		verb := strings.TrimSpace(src[verbStart:verbEnd])
		if _, ok := lookupDyadicVerbFunc(verb); !ok {
			if _, ok := lookupUnaryVerb(verb); !ok {
				continue
			}
		}
		return adverbExpr{left: left, verb: verb, adverb: adverb, right: right}, true
	}
	return adverbExpr{}, false
}

func adverbVerbBounds(src string, adverbStart int) (int, int, bool) {
	end := adverbStart
	for end > 0 && isSpace(src[end-1]) {
		end--
	}
	if end == 0 {
		return 0, 0, false
	}
	if isDyadicOp(src[end-1]) {
		return end - 1, end, true
	}
	start := end
	for start > 0 && isIdentByte(src[start-1]) {
		start--
	}
	return start, end, start < end
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDyadicOp(b byte) bool {
	return b == '+' || b == '-' || b == '*' || b == '%' || b == '=' || b == '<' || b == '>' || b == '&' || b == '|' || b == '^' || b == '~'
}

func (s *EvalState) evalAdverb(expr adverbExpr) (any, error) {
	right, err := s.eval(expr.right)
	if err != nil {
		return nil, err
	}
	if expr.left == "" {
		switch expr.adverb {
		case "'":
			return applyEachUnary(expr.verb, right)
		case "':":
			fn, ok := lookupDyadicVerbFunc(expr.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with each-prior", expr.verb)
			}
			return applyEachPriorFunc(fn, nil, right)
		case "/":
			op, _, ok := lookupDyadicVerb(expr.verb)
			if ok {
				return applyOver(op, nil, right)
			}
			if _, ok := lookupDyadicVerbFunc(expr.verb); ok {
				return nil, fmt.Errorf("%s cannot be used with over", expr.verb)
			}
			if fn, ok := lookupUnaryVerb(expr.verb); ok {
				return fn(right)
			}
			return nil, fmt.Errorf("%s cannot be used with over", expr.verb)
		case "\\":
			op, _, ok := lookupDyadicVerb(expr.verb)
			if !ok {
				if _, ok := lookupDyadicVerbFunc(expr.verb); ok {
					return nil, fmt.Errorf("%s cannot be used with scan", expr.verb)
				}
				return nil, fmt.Errorf("%s cannot be used with scan", expr.verb)
			}
			return applyScan(op, nil, right)
		default:
			return nil, fmt.Errorf("%s requires a left operand", expr.adverb)
		}
	}
	left, err := s.eval(expr.left)
	if err != nil {
		return nil, err
	}
	fn, ok := lookupDyadicVerbFunc(expr.verb)
	if !ok {
		return nil, fmt.Errorf("%s cannot be used as a dyadic verb", expr.verb)
	}
	switch expr.adverb {
	case "'":
		return applyEachDyadicFunc(fn, left, right)
	case "':":
		return applyEachPriorFunc(fn, left, right)
	case "\\:":
		return applyEachLeftFunc(fn, left, right)
	case "/:":
		return applyEachRightFunc(fn, left, right)
	case "/":
		op, _, ok := lookupDyadicVerb(expr.verb)
		if !ok {
			return nil, fmt.Errorf("%s cannot be used with over", expr.verb)
		}
		return applyOver(op, left, right)
	case "\\":
		op, _, ok := lookupDyadicVerb(expr.verb)
		if !ok {
			return nil, fmt.Errorf("%s cannot be used with scan", expr.verb)
		}
		return applyScan(op, left, right)
	default:
		return nil, fmt.Errorf("adverb %q is not supported", expr.adverb)
	}
}

func (s *EvalState) evalFby(leftExpr, groupExpr string) (any, error) {
	agg, valueExpr, err := parseFbyAggregate(leftExpr)
	if err != nil {
		return nil, err
	}
	values, err := s.eval(valueExpr)
	if err != nil {
		return nil, err
	}
	valueArray, ok := values.(data.Array)
	if !ok {
		return nil, fmt.Errorf("fby aggregate value must be a vector")
	}
	groups, err := s.eval(groupExpr)
	if err != nil {
		return nil, err
	}
	groupArray, ok := groups.(data.Array)
	if !ok {
		return nil, fmt.Errorf("fby group must be a vector")
	}
	if valueArray.Len() != groupArray.Len() {
		return nil, fmt.Errorf("fby value length %d does not match group length %d", valueArray.Len(), groupArray.Len())
	}
	groupRows := make(map[string][]int, groupArray.Len())
	groupOrder := make([]string, 0, groupArray.Len())
	for row, value := range groupArray.Values() {
		key := evalValueKey(value)
		if _, ok := groupRows[key]; !ok {
			groupOrder = append(groupOrder, key)
		}
		groupRows[key] = append(groupRows[key], row)
	}
	groupValues := make(map[string]any, len(groupRows))
	for _, key := range groupOrder {
		rows := groupRows[key]
		part := valueArray.Gather(rows)
		v, err := applyFbyAggregate(agg, part)
		if err != nil {
			return nil, err
		}
		groupValues[key] = v
	}
	out := make([]any, groupArray.Len())
	for row, value := range groupArray.Values() {
		out[row] = groupValues[evalValueKey(value)]
	}
	return data.InferArray(out), nil
}

func evalValueKey(v any) string {
	if data.IsNull(v) {
		return "null"
	}
	return fmt.Sprintf("%T:%#v", v, v)
}

func parseFbyAggregate(src string) (string, string, error) {
	src = strings.TrimSpace(src)
	for _, name := range []string{"sum", "sums", "avg", "var", "dev", "med", "min", "max", "first", "last", "count"} {
		if src == name {
			return "", "", fmt.Errorf("fby %s requires a value vector", name)
		}
		prefix := name + " "
		if strings.HasPrefix(src, prefix) {
			return name, strings.TrimSpace(src[len(prefix):]), nil
		}
	}
	return "", "", fmt.Errorf("fby left operand must be an aggregate expression")
}

func parseUnaryComposition(src string) ([]qUnaryFunction, bool) {
	parts := splitTopLevelFields(src)
	if len(parts) < 2 {
		return nil, false
	}
	funcs := make([]qUnaryFunction, 0, len(parts))
	for _, part := range parts {
		fn, ok := lookupUnaryVerb(part)
		if !ok {
			return nil, false
		}
		funcs = append(funcs, qUnaryFunction{name: part, fn: fn})
	}
	return funcs, true
}

func splitTopLevelFields(src string) []string {
	var parts []string
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	start := -1
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			if start < 0 {
				start = i
			}
			inString = true
		case '(':
			if start < 0 {
				start = i
			}
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			if start < 0 {
				start = i
			}
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			if start < 0 {
				start = i
			}
			braceDepth++
		case '}':
			braceDepth--
		case ' ', '\t', '\n', '\r':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				if start >= 0 {
					parts = append(parts, strings.TrimSpace(src[start:i]))
					start = -1
				}
			}
		default:
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		parts = append(parts, strings.TrimSpace(src[start:]))
	}
	return parts
}

func applyFbyAggregate(name string, values data.Array) (any, error) {
	switch name {
	case "sum":
		return sum(values)
	case "sums":
		return sums(values)
	case "avg":
		return avg(values)
	case "var":
		return varValue(values)
	case "dev":
		return devValue(values)
	case "med":
		return medValue(values)
	case "min":
		return minValue(values)
	case "max":
		return maxValue(values)
	case "first":
		return first(values)
	case "last":
		return last(values)
	case "count":
		return count(values)
	default:
		return nil, fmt.Errorf("fby aggregate %q is not supported", name)
	}
}

func lookupDyadicVerb(verb string) (byte, string, bool) {
	switch strings.TrimSpace(verb) {
	case "+", "plus":
		return '+', "plus", true
	case "-", "minus":
		return '-', "minus", true
	case "*", "times":
		return '*', "times", true
	case "%", "divide":
		return '%', "divide", true
	case "div":
		return 'd', "div", true
	case "mod":
		return 'r', "mod", true
	case "^", "fill":
		return '^', "fill", true
	case "~", "match":
		return '~', "match", true
	case "=", "equal", "equals":
		return '=', "equal", true
	case "<", "less":
		return '<', "less", true
	case ">", "more", "greater":
		return '>', "more", true
	case "min":
		return 'm', "min", true
	case "max":
		return 'M', "max", true
	case "[", "left":
		return 'L', "left", true
	case "]", "right":
		return 'R', "right", true
	case "&", "and":
		return '&', "and", true
	case "|", "or":
		return '|', "or", true
	default:
		return 0, "", false
	}
}

func lookupDyadicVerbFunc(verb string) (func(any, any) (any, error), bool) {
	verb = strings.TrimSpace(verb)
	if op, _, ok := lookupDyadicVerb(verb); ok {
		return dyadicVerbFunc(op), true
	}
	switch verb {
	case "bin":
		return bin, true
	case "binr":
		return binr, true
	case "xbar":
		return xbar, true
	case "xrank":
		return xrank, true
	case "msum":
		return msum, true
	case "mavg":
		return mavg, true
	case "mcount":
		return mcount, true
	case "mmin":
		return mmin, true
	case "mmax":
		return mmax, true
	case "xprev":
		return xprev, true
	case "xcols":
		return xcols, true
	case "xkey":
		return xkey, true
	case "xgroup":
		return xgroup, true
	case "xasc":
		return xasc, true
	case "xdesc":
		return xdesc, true
	case "rotate":
		return rotateValue, true
	case "wavg":
		return wavg, true
	case "within":
		return within, true
	case "like":
		return likeValue, true
	case "in":
		return membership, true
	case "intersect", "inter":
		return inter, true
	case "except":
		return except, true
	case "union":
		return union, true
	default:
		return nil, false
	}
}

func lookupUnaryVerb(verb string) (func(any) (any, error), bool) {
	switch strings.TrimSpace(verb) {
	case "count":
		return count, true
	case "enlist":
		return enlist, true
	case "type":
		return typeOf, true
	case "string":
		return stringValue, true
	case "lower":
		return lowerValue, true
	case "upper":
		return upperValue, true
	case "distinct":
		return distinct, true
	case "first":
		return first, true
	case "last":
		return last, true
	case "keys":
		return keys, true
	case "key":
		return keys, true
	case "value":
		return value, true
	case "cols":
		return cols, true
	case "meta":
		return meta, true
	case "attr":
		return attrValue, true
	case "codes":
		return enumCodes, true
	case "domain":
		return enumDomainValues, true
	case "group":
		return group, true
	case "ungroup":
		return ungroup, true
	case "raze":
		return raze, true
	case "avg":
		return avg, true
	case "var":
		return varValue, true
	case "dev":
		return devValue, true
	case "med":
		return medValue, true
	case "prd":
		return prd, true
	case "min":
		return minValue, true
	case "max":
		return maxValue, true
	case "reverse":
		return reverse, true
	case "prior":
		return prev, true
	case "prev":
		return prev, true
	case "next":
		return nextValue, true
	case "deltas":
		return deltas, true
	case "fills":
		return fills, true
	case "differ":
		return differ, true
	case "ratios":
		return ratios, true
	case "asc":
		return asc, true
	case "desc":
		return desc, true
	case "iasc":
		return iasc, true
	case "idesc":
		return idesc, true
	case "rank":
		return rank, true
	case "neg":
		return negValue, true
	case "abs":
		return absValue, true
	case "exp":
		return expValue, true
	case "reciprocal":
		return reciprocalValue, true
	case "signum":
		return signumValue, true
	case "floor":
		return floorValue, true
	case "ceiling":
		return ceilingValue, true
	case "sum":
		return sum, true
	case "sums":
		return sums, true
	case "prds":
		return prds, true
	case "mins":
		return mins, true
	case "maxs":
		return maxs, true
	case "avgs":
		return avgs, true
	case "all":
		return allValue, true
	case "any":
		return anyValue, true
	case "where":
		return where, true
	case "not":
		return notValue, true
	case "null":
		return nullValue, true
	default:
		return nil, false
	}
}

func isCallable(v any) bool {
	switch v.(type) {
	case qLambda, qProjection, qAdverbFunction, qCallableAdverb, qDyadicFunction, qUnaryFunction, qComposition, *qIPCHandle:
		return true
	default:
		return false
	}
}

func (s *EvalState) applyCallableIndex(fn any, argSrc string) (any, error) {
	args, hasMissing, err := s.parseCallableArgs(argSrc)
	if err != nil {
		return nil, err
	}
	if hasMissing {
		return qProjection{fn: fn, args: args}, nil
	}
	values := make([]any, len(args))
	for i, arg := range args {
		values[i] = arg.value
	}
	return s.applyCallable(fn, values)
}

func (s *EvalState) parseCallableArgs(src string) ([]projectionArg, bool, error) {
	parts := splitTopLevelDelim(src, ';')
	if parts == nil {
		parts = []string{strings.TrimSpace(src)}
	}
	args := make([]projectionArg, len(parts))
	hasMissing := false
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			args[i] = projectionArg{missing: true}
			hasMissing = true
			continue
		}
		v, err := s.eval(part)
		if err != nil {
			return nil, false, err
		}
		args[i] = projectionArg{value: v}
	}
	return args, hasMissing, nil
}

func (s *EvalState) applyCallable(fn any, args []any) (any, error) {
	switch f := fn.(type) {
	case qLambda:
		return s.applyLambda(f, args)
	case qUnaryFunction:
		return applyUnaryFunction(f, args)
	case qComposition:
		return applyComposition(f, args)
	case qDyadicFunction:
		return applyDyadicFunction(f, args)
	case qAdverbFunction:
		return applyAdverbFunction(f, args)
	case qCallableAdverb:
		return s.applyCallableAdverbFunction(f, args)
	case *qIPCHandle:
		return s.applyIPCHandle(f, args)
	case qProjection:
		projected := make([]projectionArg, 0, len(f.args))
		merged := make([]any, 0, len(f.args)+len(args))
		next := 0
		hasMissing := false
		for _, arg := range f.args {
			if arg.missing {
				if next >= len(args) {
					projected = append(projected, projectionArg{missing: true})
					hasMissing = true
					continue
				}
				projected = append(projected, projectionArg{value: args[next]})
				merged = append(merged, args[next])
				next++
				continue
			}
			projected = append(projected, arg)
			merged = append(merged, arg.value)
		}
		if hasMissing {
			return qProjection{fn: f.fn, args: projected}, nil
		}
		merged = append(merged, args[next:]...)
		return s.applyCallable(f.fn, merged)
	default:
		return nil, fmt.Errorf("value is not callable")
	}
}

func (s *EvalState) applyIPCHandle(h *qIPCHandle, args []any) (any, error) {
	if h == nil || h.session == nil {
		return nil, fmt.Errorf("q IPC handle is closed")
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("q IPC handle call expected 1 command argument, got %d", len(args))
	}
	value, err := h.session.evalIPCMessage(args[0])
	if err != nil {
		return nil, fmt.Errorf("q IPC %s: %w", h.target, err)
	}
	if h.async {
		return data.NullValue, nil
	}
	return value, nil
}

func (s *EvalState) evalIPCMessage(message any) (any, error) {
	if list, ok := message.(data.Array); ok {
		items := list.Values()
		if len(items) == 0 {
			return nil, fmt.Errorf("q IPC message list is empty")
		}
		return s.evalIPCListMessage(items)
	}
	command, ok := message.(string)
	if !ok {
		return nil, fmt.Errorf("q IPC handle call expects a string command or message list")
	}
	return s.evalIPCStringCommand(command, nil)
}

func (s *EvalState) evalIPCListMessage(items []any) (any, error) {
	head := items[0]
	args := items[1:]
	switch fn := head.(type) {
	case string:
		return s.evalIPCStringCommand(fn, args)
	case data.Symbol:
		v, ok := s.env[string(fn)]
		if !ok {
			return nil, fmt.Errorf("q IPC function %q is not defined", fn)
		}
		if !isCallable(v) {
			return nil, fmt.Errorf("q IPC function %q is not callable", fn)
		}
		return s.applyCallable(v, args)
	default:
		if isCallable(fn) {
			return s.applyCallable(fn, args)
		}
		return nil, fmt.Errorf("q IPC message list head must be a command string or callable")
	}
}

func (s *EvalState) evalIPCStringCommand(command string, args []any) (any, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("q IPC handle command is empty")
	}
	if strings.HasPrefix(command, "\\") {
		return nil, fmt.Errorf("q IPC system commands are not supported")
	}
	restore := s.bindIPCArgs(args)
	value, err := s.Eval(command)
	restore()
	if err != nil {
		return nil, err
	}
	if len(args) > 0 && isCallable(value) {
		return s.applyCallable(value, args)
	}
	return value, nil
}

func (s *EvalState) bindIPCArgs(args []any) func() {
	if len(args) == 0 {
		return func() {}
	}
	names := make([]string, 0, len(args)+3)
	for i := range args {
		names = append(names, fmt.Sprintf("x%d", i))
	}
	for i, name := range []string{"x", "y", "z"} {
		if i < len(args) {
			names = append(names, name)
		}
	}
	type oldValue struct {
		value any
		ok    bool
	}
	old := make(map[string]oldValue, len(names))
	for _, name := range names {
		if _, exists := old[name]; exists {
			continue
		}
		v, ok := s.env[name]
		old[name] = oldValue{value: v, ok: ok}
	}
	for i, arg := range args {
		s.env[fmt.Sprintf("x%d", i)] = arg
	}
	if len(args) > 0 {
		s.env["x"] = args[0]
	}
	if len(args) > 1 {
		s.env["y"] = args[1]
	}
	if len(args) > 2 {
		s.env["z"] = args[2]
	}
	return func() {
		for name, previous := range old {
			if previous.ok {
				s.env[name] = previous.value
				continue
			}
			delete(s.env, name)
		}
	}
}

func applyUnaryFunction(fn qUnaryFunction, args []any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("unary function %s expected 1 argument, got %d", fn.name, len(args))
	}
	return fn.fn(args[0])
}

func applyComposition(fn qComposition, args []any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("composition expected 1 argument, got %d", len(args))
	}
	value := args[0]
	for i := len(fn.funcs) - 1; i >= 0; i-- {
		next, err := fn.funcs[i].fn(value)
		if err != nil {
			return nil, fmt.Errorf("composition %s failed: %w", fn.funcs[i].name, err)
		}
		value = next
	}
	return value, nil
}

func applyDyadicFunction(fn qDyadicFunction, args []any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("dyadic function %s expected 2 arguments, got %d", fn.name, len(args))
	}
	return fn.fn(args[0], args[1])
}

func (s *EvalState) applyCallableAdverbFunction(fn qCallableAdverb, args []any) (any, error) {
	if err := validateCallableReduceScanBoundary(fn.fn, fn.adverb); err != nil {
		return nil, err
	}
	switch len(args) {
	case 1:
		switch fn.adverb {
		case "\\:", "/:":
			return nil, fmt.Errorf("callable %s expected 2 arguments, got 1", adverbName(fn.adverb))
		}
		return s.evalCallableAdverb(fn.fn, fn.adverb, args[0])
	case 2:
		switch fn.adverb {
		case "'":
			return s.applyEachDyadicCallable(fn.fn, args[0], args[1])
		case "/":
			return s.applyOverCallable(fn.fn, args[0], args[1])
		case "\\":
			return s.applyScanCallable(fn.fn, args[0], args[1])
		case "':":
			return s.applyEachPriorCallable(fn.fn, args[0], args[1])
		case "\\:":
			return s.applyEachLeftCallable(fn.fn, args[0], args[1])
		case "/:":
			return s.applyEachRightCallable(fn.fn, args[0], args[1])
		default:
			return nil, fmt.Errorf("callable adverb %q is not supported", fn.adverb)
		}
	default:
		switch fn.adverb {
		case "'":
			return nil, fmt.Errorf("callable each expected 1 or 2 arguments, got %d", len(args))
		case "/", "\\":
			return nil, fmt.Errorf("callable %s expected 1 or 2 arguments, got %d", adverbName(fn.adverb), len(args))
		case "':":
			return nil, fmt.Errorf("callable %s expected 1 or 2 arguments, got %d", adverbName(fn.adverb), len(args))
		case "\\:", "/:":
			return nil, fmt.Errorf("callable %s expected 2 arguments, got %d", adverbName(fn.adverb), len(args))
		default:
			return nil, fmt.Errorf("callable adverb %q is not supported", fn.adverb)
		}
	}
}

func applyAdverbFunction(fn qAdverbFunction, args []any) (any, error) {
	dyad, ok := lookupDyadicVerbFunc(fn.verb)
	if !ok {
		return nil, fmt.Errorf("%s cannot be used as a dyadic verb", fn.verb)
	}
	switch len(args) {
	case 1:
		switch fn.adverb {
		case "/":
			op, _, ok := lookupDyadicVerb(fn.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with over", fn.verb)
			}
			return applyOver(op, nil, args[0])
		case "\\":
			op, _, ok := lookupDyadicVerb(fn.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with scan", fn.verb)
			}
			return applyScan(op, nil, args[0])
		case "':":
			return applyEachPriorFunc(dyad, nil, args[0])
		case "'":
			return nil, fmt.Errorf("adverb function each expected 2 arguments, got 1")
		case "\\:", "/:":
			return nil, fmt.Errorf("adverb function %s expected 2 arguments, got 1", adverbName(fn.adverb))
		default:
			return nil, fmt.Errorf("adverb %q is not supported as a function", fn.adverb)
		}
	case 2:
		switch fn.adverb {
		case "'":
			return applyEachDyadicFunc(dyad, args[0], args[1])
		case "/":
			op, _, ok := lookupDyadicVerb(fn.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with over", fn.verb)
			}
			return applyOver(op, args[0], args[1])
		case "\\":
			op, _, ok := lookupDyadicVerb(fn.verb)
			if !ok {
				return nil, fmt.Errorf("%s cannot be used with scan", fn.verb)
			}
			return applyScan(op, args[0], args[1])
		case "':":
			return applyEachPriorFunc(dyad, args[0], args[1])
		case "\\:":
			return applyEachLeftFunc(dyad, args[0], args[1])
		case "/:":
			return applyEachRightFunc(dyad, args[0], args[1])
		default:
			return nil, fmt.Errorf("adverb %q is not supported as a function", fn.adverb)
		}
	default:
		switch fn.adverb {
		case "/", "\\", "':":
			return nil, fmt.Errorf("adverb function %s expected 1 or 2 arguments, got %d", adverbName(fn.adverb), len(args))
		case "\\:", "/:":
			return nil, fmt.Errorf("adverb function %s expected 2 arguments, got %d", adverbName(fn.adverb), len(args))
		default:
			return nil, fmt.Errorf("adverb function expected 1 or 2 arguments, got %d", len(args))
		}
	}
}

func (s *EvalState) applyLambda(fn qLambda, args []any) (any, error) {
	params, body, err := lambdaSignature(fn.body)
	if err != nil {
		return nil, err
	}
	if len(args) > len(params) {
		return nil, fmt.Errorf("lambda expected at most %d arguments, got %d", len(params), len(args))
	}
	vars := cloneEnv(fn.env)
	state := NewEvalState(vars)
	state.namespace = fn.namespace
	for i, arg := range args {
		state.env[state.resolveAssignmentName(params[i])] = arg
	}
	return state.Eval(body)
}

func lambdaSignature(src string) ([]string, string, error) {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "[") {
		end := findMatchingDelimiter(src, 0, '[', ']')
		if end < 0 {
			return nil, "", fmt.Errorf("lambda parameter list is not closed")
		}
		paramSrc := strings.TrimSpace(src[1:end])
		parts := splitTopLevelDelim(paramSrc, ';')
		if parts == nil && paramSrc != "" {
			parts = []string{paramSrc}
		}
		params := make([]string, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if !isQBareName(name) {
				return nil, "", fmt.Errorf("invalid lambda parameter %q", name)
			}
			params = append(params, name)
		}
		return params, strings.TrimSpace(src[end+1:]), nil
	}
	return []string{"x", "y", "z"}, src, nil
}

func findLambdaAdverb(src string) (string, string, string, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "{") {
		return "", "", "", false
	}
	end := findMatchingDelimiter(src, 0, '{', '}')
	if end < 0 {
		return "", "", "", false
	}
	rest := strings.TrimSpace(src[end+1:])
	for _, adverb := range []string{"':", "\\:", "/:", "'", "/", "\\"} {
		if strings.HasPrefix(rest, adverb) {
			right := strings.TrimSpace(rest[len(adverb):])
			if strings.HasPrefix(right, "[") {
				return "", "", "", false
			}
			return src[:end+1], adverb, right, right != ""
		}
	}
	return "", "", "", false
}

func findLambdaAdverbFunction(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "{") {
		return "", "", false
	}
	end := findMatchingDelimiter(src, 0, '{', '}')
	if end < 0 {
		return "", "", false
	}
	rest := strings.TrimSpace(src[end+1:])
	for _, adverb := range []string{"':", "\\:", "/:", "'", "/", "\\"} {
		if rest == adverb {
			return src[:end+1], adverb, true
		}
	}
	return "", "", false
}

func (s *EvalState) findCallableAdverbFunction(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	for _, adverb := range []string{"':", "\\:", "/:", "'", "/", "\\"} {
		if !strings.HasSuffix(src, adverb) {
			continue
		}
		left := strings.TrimSpace(src[:len(src)-len(adverb)])
		if left == "" {
			continue
		}
		if _, ok := lookupDyadicVerbFunc(left); ok {
			return left, adverb, true
		}
		if !s.isPotentialCallableExpr(left) {
			continue
		}
		return left, adverb, true
	}
	return "", "", false
}

func (s *EvalState) findCallablePostfixAdverb(src string) (string, string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
			continue
		case '(':
			parenDepth++
			continue
		case ')':
			parenDepth--
			continue
		case '[':
			bracketDepth++
			continue
		case ']':
			bracketDepth--
			continue
		case '{':
			braceDepth++
			continue
		case '}':
			braceDepth--
			continue
		}
		if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 {
			continue
		}
		adverb := ""
		switch {
		case strings.HasPrefix(src[i:], "':"):
			adverb = "':"
		case strings.HasPrefix(src[i:], "\\:"):
			adverb = "\\:"
		case strings.HasPrefix(src[i:], "/:"):
			adverb = "/:"
		case src[i] == '\'':
			adverb = "'"
		case src[i] == '/':
			adverb = "/"
		case src[i] == '\\':
			adverb = "\\"
		default:
			continue
		}
		left := strings.TrimSpace(src[:i])
		right := strings.TrimSpace(src[i+len(adverb):])
		if left == "" || right == "" {
			continue
		}
		if adverb == "':" && strings.HasPrefix(right, "[") {
			continue
		}
		if !s.isPotentialCallableExpr(left) {
			continue
		}
		return left, adverb, right, true
	}
	return "", "", "", false
}

func (s *EvalState) isPotentialCallableExpr(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	if isQAssignmentName(src) {
		if v, ok := s.env[src]; ok && isCallable(v) {
			return true
		}
	}
	if strings.HasSuffix(src, "}") || strings.HasSuffix(src, "]") || strings.HasSuffix(src, ")") {
		return true
	}
	return false
}

func validateCallableReduceScanBoundary(fn any, adverb string) error {
	if adverb != "/" && adverb != "\\" {
		return nil
	}
	if _, ok := fn.(qComposition); ok {
		return fmt.Errorf("composition cannot be used with %s", adverbName(adverb))
	}
	dyad, ok := fn.(qDyadicFunction)
	if !ok {
		return nil
	}
	if _, _, ok := lookupDyadicVerb(dyad.name); ok {
		return nil
	}
	switch adverb {
	case "/":
		return fmt.Errorf("%s cannot be used with over", dyad.name)
	case "\\":
		return fmt.Errorf("%s cannot be used with scan", dyad.name)
	default:
		return nil
	}
}

func (s *EvalState) evalCallableAdverb(fn any, adverb string, right any) (any, error) {
	if err := validateCallableReduceScanBoundary(fn, adverb); err != nil {
		return nil, err
	}
	switch adverb {
	case "'":
		return s.applyEachCallable(fn, right)
	case "/":
		return s.applyOverCallable(fn, nil, right)
	case "\\":
		return s.applyScanCallable(fn, nil, right)
	case "':":
		return s.applyEachPriorCallable(fn, nil, right)
	case "\\:", "/:":
		return nil, fmt.Errorf("callable %s expected 2 arguments, got 1", adverbName(adverb))
	default:
		return nil, fmt.Errorf("callable adverb %q is not supported", adverb)
	}
}

func adverbName(adverb string) string {
	switch adverb {
	case "/":
		return "over"
	case "\\":
		return "scan"
	case "'":
		return "each"
	case "':":
		return "each-prior"
	case "/:":
		return "each-right"
	case "\\:":
		return "each-left"
	default:
		return "adverb"
	}
}

func (s *EvalState) applyEachCallable(fn any, v any) (any, error) {
	if dict, ok := v.(EvalDict); ok {
		out := EvalDict{
			Keys:   append([]any(nil), dict.Keys...),
			Values: make([]any, len(dict.Values)),
		}
		for i, item := range dict.Values {
			value, err := s.applyCallable(fn, []any{item})
			if err != nil {
				return nil, err
			}
			out.Values[i] = value
		}
		return out, nil
	}
	items, err := vectorValues(v)
	if err != nil {
		return s.applyCallable(fn, []any{v})
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := s.applyCallable(fn, []any{item})
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return inferQArray(out, qKindOfValue(v)), nil
}

func (s *EvalState) applyEachDyadicCallable(fn any, left any, right any) (any, error) {
	if isUnaryOnlyCallable(fn) {
		return s.applyCallable(fn, []any{right})
	}
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if !lok && !rok {
		return s.applyCallable(fn, []any{left, right})
	}
	if lok && rok && la.Len() != ra.Len() {
		return nil, fmt.Errorf("callable each length mismatch")
	}
	n := 0
	switch {
	case lok:
		n = la.Len()
	case rok:
		n = ra.Len()
	}
	out := make([]any, n)
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if lok {
			var ok bool
			lv, ok = la.At(i)
			if !ok {
				return nil, fmt.Errorf("callable each left row %d out of range", i)
			}
		}
		if rok {
			var ok bool
			rv, ok = ra.At(i)
			if !ok {
				return nil, fmt.Errorf("callable each right row %d out of range", i)
			}
		}
		value, err := s.applyCallable(fn, []any{lv, rv})
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func isUnaryOnlyCallable(fn any) bool {
	switch fn.(type) {
	case qUnaryFunction, qComposition:
		return true
	default:
		return false
	}
}

func (s *EvalState) applyEachLeftCallable(fn any, left any, right any) (any, error) {
	items, err := vectorValues(right)
	if err != nil {
		return s.applyCallable(fn, []any{left, right})
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := s.applyCallable(fn, []any{left, item})
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	if len(out) == 0 && !isTypedEmptyAdverbSource(right) {
		return inferQArray(out), nil
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func (s *EvalState) applyEachRightCallable(fn any, left any, right any) (any, error) {
	items, err := vectorValues(left)
	if err != nil {
		return s.applyCallable(fn, []any{left, right})
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := s.applyCallable(fn, []any{item, right})
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	if len(out) == 0 && !isTypedEmptyAdverbSource(left) {
		return inferQArray(out), nil
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func (s *EvalState) applyOverCallable(fn any, initial any, v any) (any, error) {
	if adverbFn, ok := fn.(qAdverbFunction); ok && adverbFn.verb == "+" && adverbFn.adverb == "/" && initial == nil {
		return sum(v)
	}
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return s.applyCallable(fn, []any{initial, v})
		}
		return v, nil
	}
	if len(items) == 0 {
		if initial != nil {
			return initial, nil
		}
		return data.NullValue, nil
	}
	acc := initial
	start := 0
	if acc == nil {
		acc = items[0]
		start = 1
	}
	for i := start; i < len(items); i++ {
		next, err := s.applyCallable(fn, []any{acc, items[i]})
		if err != nil {
			return nil, err
		}
		acc = next
	}
	return acc, nil
}

func (s *EvalState) applyScanCallable(fn any, initial any, v any) (any, error) {
	if adverbFn, ok := fn.(qAdverbFunction); ok && adverbFn.verb == "+" && adverbFn.adverb == "\\" && initial == nil {
		return sums(v)
	}
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return s.applyCallable(fn, []any{initial, v})
		}
		return v, nil
	}
	if len(items) == 0 {
		if initial != nil {
			return data.InferArray(nil), nil
		}
		return inferQArray(nil, qKindOfValue(initial), qKindOfValue(v)), nil
	}
	out := make([]any, len(items))
	acc := initial
	start := 0
	if acc == nil {
		acc = items[0]
		out[0] = acc
		start = 1
	}
	for i := start; i < len(items); i++ {
		next, err := s.applyCallable(fn, []any{acc, items[i]})
		if err != nil {
			return nil, err
		}
		acc = next
		out[i] = acc
	}
	return inferQArray(out, qKindOfValue(initial), qKindOfValue(v)), nil
}

func (s *EvalState) applyEachPriorCallable(fn any, initial any, v any) (any, error) {
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return s.applyCallable(fn, []any{v, initial})
		}
		return v, nil
	}
	out := make([]any, len(items))
	var previous any
	if initial != nil {
		previous = initial
	}
	for i, item := range items {
		if i == 0 && initial == nil {
			out[i] = item
		} else {
			value, err := s.applyCallable(fn, []any{item, previous})
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		previous = item
	}
	return inferQArray(out, qKindOfValue(initial), qKindOfValue(v)), nil
}

func applyEachUnary(verb string, v any) (any, error) {
	fn, ok := lookupUnaryVerb(verb)
	if !ok {
		return nil, fmt.Errorf("%s cannot be used monadically with each", verb)
	}
	if dict, ok := v.(EvalDict); ok {
		out := EvalDict{
			Keys:   append([]any(nil), dict.Keys...),
			Values: make([]any, len(dict.Values)),
		}
		for i, item := range dict.Values {
			value, err := fn(item)
			if err != nil {
				return nil, err
			}
			out.Values[i] = value
		}
		return out, nil
	}
	items, err := vectorValues(v)
	if err != nil {
		return fn(v)
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := fn(item)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return inferQArray(out, qKindOfValue(v)), nil
}

func applyEachDyadic(op byte, left, right any) (any, error) {
	return applyEachDyadicFunc(dyadicVerbFunc(op), left, right)
}

func applyEachDyadicFunc(fn func(any, any) (any, error), left, right any) (any, error) {
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if !lok && !rok {
		return fn(left, right)
	}
	if lok && rok && la.Len() != ra.Len() {
		return nil, fmt.Errorf("each length mismatch")
	}
	n := 0
	switch {
	case lok:
		n = la.Len()
	case rok:
		n = ra.Len()
	}
	out := make([]any, n)
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if lok {
			var ok bool
			lv, ok = la.At(i)
			if !ok {
				return nil, fmt.Errorf("left vector row %d out of range", i)
			}
		}
		if rok {
			var ok bool
			rv, ok = ra.At(i)
			if !ok {
				return nil, fmt.Errorf("right vector row %d out of range", i)
			}
		}
		value, err := fn(lv, rv)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func applyEachPrior(op byte, initial any, v any) (any, error) {
	return applyEachPriorFunc(dyadicVerbFunc(op), initial, v)
}

func applyEachPriorFunc(fn func(any, any) (any, error), initial any, v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		if initial != nil {
			return fn(v, initial)
		}
		return v, nil
	}
	out := make([]any, array.Len())
	var previous any
	if initial != nil {
		previous = initial
	}
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("each-prior row %d out of range", i)
		}
		if i == 0 && initial == nil {
			out[i] = item
		} else {
			value, err := fn(item, previous)
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		previous = item
	}
	return inferQArray(out, qKindOfValue(initial), qKindOfValue(v)), nil
}

func applyEachLeft(op byte, left, right any) (any, error) {
	return applyEachLeftFunc(dyadicVerbFunc(op), left, right)
}

func applyEachLeftFunc(fn func(any, any) (any, error), left, right any) (any, error) {
	items, err := vectorValues(right)
	if err != nil {
		return fn(left, right)
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := fn(left, item)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	if len(out) == 0 && !isTypedEmptyAdverbSource(right) {
		return inferQArray(out), nil
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func applyEachRight(op byte, left, right any) (any, error) {
	return applyEachRightFunc(dyadicVerbFunc(op), left, right)
}

func applyEachRightFunc(fn func(any, any) (any, error), left, right any) (any, error) {
	items, err := vectorValues(left)
	if err != nil {
		return fn(left, right)
	}
	out := make([]any, len(items))
	for i, item := range items {
		value, err := fn(item, right)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	if len(out) == 0 && !isTypedEmptyAdverbSource(left) {
		return inferQArray(out), nil
	}
	return inferQArray(out, qKindOfValue(left), qKindOfValue(right)), nil
}

func isTypedEmptyAdverbSource(v any) bool {
	array, ok := v.(data.Array)
	if !ok || array.Len() != 0 {
		return false
	}
	kind := array.Kind()
	return kind != "" && kind != data.KindNull && kind != data.KindAny
}

func applyOver(op byte, initial any, v any) (any, error) {
	if op == '+' && initial == nil {
		return sum(v)
	}
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return applyDyadic(op, initial, v)
		}
		return v, nil
	}
	if len(items) == 0 {
		if initial != nil {
			return initial, nil
		}
		return data.NullValue, nil
	}
	acc := initial
	start := 0
	if acc == nil {
		acc = items[0]
		start = 1
	}
	for i := start; i < len(items); i++ {
		next, err := applyDyadic(op, acc, items[i])
		if err != nil {
			return nil, err
		}
		acc = next
	}
	return acc, nil
}

func applyScan(op byte, initial any, v any) (any, error) {
	if op == '+' && initial == nil {
		return sums(v)
	}
	items, err := vectorValues(v)
	if err != nil {
		if initial != nil {
			return applyDyadic(op, initial, v)
		}
		return v, nil
	}
	if len(items) == 0 {
		if initial != nil && !isTypedEmptyAdverbSource(v) {
			return data.InferArray(nil), nil
		}
		return inferQArray(nil, qKindOfValue(initial), qKindOfValue(v)), nil
	}
	out := make([]any, len(items))
	acc := initial
	start := 0
	if acc == nil {
		acc = items[0]
		out[0] = acc
		start = 1
	}
	for i := start; i < len(items); i++ {
		next, err := applyDyadic(op, acc, items[i])
		if err != nil {
			return nil, err
		}
		acc = next
		out[i] = acc
	}
	return inferQArray(out, qKindOfValue(initial), qKindOfValue(v)), nil
}

func parseSymbolList(src string) ([]data.Symbol, error) {
	if !strings.HasPrefix(src, "`") {
		return nil, fmt.Errorf("symbol list must start with `")
	}
	var out []data.Symbol
	for len(src) > 0 {
		if src[0] != '`' {
			return nil, fmt.Errorf("malformed symbol list near %q", src)
		}
		src = src[1:]
		next := strings.IndexByte(src, '`')
		var sym string
		if next < 0 {
			sym = strings.TrimSpace(src)
			src = ""
		} else {
			sym = strings.TrimSpace(src[:next])
			src = src[next:]
		}
		if sym == "" {
			return nil, fmt.Errorf("empty symbol")
		}
		out = append(out, data.Symbol(sym))
	}
	return out, nil
}

func parseAtomOrVector(src string) (any, error) {
	if strings.HasPrefix(src, "\"") && strings.HasSuffix(src, "\"") {
		v, err := strconv.Unquote(src)
		if err == nil {
			return v, nil
		}
	}
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty q expression")
	}
	values := make([]any, len(fields))
	temporalKinds := make([]data.Kind, len(fields))
	hasTemporal := false
	hasFloat := false
	hasNull := false
	hasBool := false
	hasTypedScalar := false
	for i, field := range fields {
		if strings.HasPrefix(field, "\"") {
			v, err := strconv.Unquote(field)
			if err != nil {
				return nil, fmt.Errorf("invalid string %q", field)
			}
			values[i] = v
			continue
		}
		if kind, text, ok := parseTemporalToken(field); ok {
			v, err := parseQTemporal(kind, text)
			if err != nil {
				return nil, err
			}
			temporalKinds[i] = data.Kind(kind)
			hasTemporal = true
			values[i] = v
			continue
		}
		v, isFloat, err := parseNumberOrBool(field)
		if err != nil {
			return nil, err
		}
		values[i] = v
		hasFloat = hasFloat || isFloat
		hasNull = hasNull || data.IsNull(v)
		if _, ok := v.(bool); ok {
			hasBool = true
		}
		switch v.(type) {
		case int16, int32, float32:
			hasTypedScalar = true
		}
	}
	if len(values) == 1 {
		return values[0], nil
	}
	if kind, ok := commonTypedNullKind(values); ok && allValuesNull(values) {
		column, err := data.NewColumnWithKind("_", kind, values)
		if err != nil {
			return nil, err
		}
		return column.Data, nil
	}
	if _, ok := values[0].(string); ok {
		xs := make([]string, len(values))
		for i, v := range values {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("mixed string and non-string vectors are not supported")
			}
			xs[i] = s
		}
		return data.NewString(xs), nil
	}
	if hasTemporal {
		for _, v := range values {
			if data.IsNull(v) {
				continue
			}
			if temporalKindOfValue(v) == "" {
				return nil, fmt.Errorf("mixed temporal and non-temporal vectors are not supported")
			}
		}
		kind, err := coerceTemporalVectorValues(values, temporalKinds)
		if err != nil {
			return nil, err
		}
		column, err := data.NewColumnWithKind("_", kind, values)
		if err != nil {
			return nil, err
		}
		return column.Data, nil
	}
	if hasTypedScalar {
		return data.NewColumn("_", values).Data, nil
	}
	if hasFloat {
		xs := make([]float64, len(values))
		for i, v := range values {
			if data.IsNull(v) {
				continue
			}
			n, ok := numeric(v)
			if !ok {
				return data.InferArray(values), nil
			}
			xs[i] = n
		}
		if hasNull {
			return data.NewColumn("_", values).Data, nil
		}
		return data.NewF64(xs), nil
	}
	if _, ok := values[0].(bool); ok {
		xs := make([]bool, len(values))
		for i, v := range values {
			if data.IsNull(v) {
				continue
			}
			b, ok := v.(bool)
			if !ok {
				return data.InferArray(values), nil
			}
			xs[i] = b
		}
		if hasNull {
			return data.NewColumn("_", values).Data, nil
		}
		return data.NewBool(xs), nil
	}
	if hasNull {
		return data.NewColumn("_", values).Data, nil
	}
	if hasBool {
		return data.InferArray(values), nil
	}
	xs := make([]int64, len(values))
	for i, v := range values {
		xs[i] = v.(int64)
	}
	return data.NewI64(xs), nil
}

func looksLikeTemporalVector(src string) bool {
	fields := strings.Fields(src)
	if len(fields) <= 1 {
		return false
	}
	hasTemporal := false
	for _, field := range fields {
		if _, _, ok := parseTemporalToken(field); ok {
			hasTemporal = true
			continue
		}
		if _, _, err := parseNumberOrBool(field); err == nil {
			continue
		}
		return false
	}
	return hasTemporal
}

func looksLikeLiteralVector(src string) bool {
	fields := strings.Fields(src)
	if len(fields) <= 1 {
		return false
	}
	for _, field := range fields {
		if strings.HasPrefix(field, "\"") {
			if _, err := strconv.Unquote(field); err != nil {
				return false
			}
			continue
		}
		if containsInternalDyadicSign(field) {
			return false
		}
		if _, _, ok := parseTemporalToken(field); ok {
			continue
		}
		if _, _, err := parseNumberOrBool(field); err == nil {
			continue
		}
		return false
	}
	return true
}

func containsInternalDyadicSign(field string) bool {
	for i := 1; i < len(field); i++ {
		if field[i] == '+' || field[i] == '-' {
			return true
		}
	}
	return false
}

func parseNumberOrBool(src string) (any, bool, error) {
	if strings.HasPrefix(src, "-") || strings.HasPrefix(src, "+") {
		sign := src[0]
		body := src[1:]
		if v, isFloat, ok := parseQInfinity(body); ok {
			if sign == '-' {
				v = negateQInfinity(v)
			}
			return v, isFloat, nil
		}
	}
	switch src {
	case "true":
		return true, false, nil
	case "false":
		return false, false, nil
	case "0N", "0n":
		return data.NullValue, false, nil
	case "0Nb", "0Nx", "0Nc", "0Nh", "0Ni", "0Nj", "0Ne", "0Nf":
		switch src {
		case "0Nb":
			return data.NullForKind(data.KindBool), false, nil
		case "0Nx":
			return data.NullForKind(data.KindU8), false, nil
		case "0Nc":
			return data.NullForKind(data.KindString), false, nil
		case "0Nh":
			return data.NullForKind(data.KindI16), false, nil
		case "0Ni":
			return data.NullForKind(data.KindI32), false, nil
		case "0Nj":
			return data.NullForKind(data.KindI64), false, nil
		case "0Ne":
			return data.NullForKind(data.KindF32), true, nil
		case "0Nf":
			return data.NullForKind(data.KindF64), true, nil
		}
	}
	if v, isFloat, ok := parseQInfinity(src); ok {
		return v, isFloat, nil
	}
	if len(src) > 1 {
		suffix := src[len(src)-1]
		body := src[:len(src)-1]
		switch suffix {
		case 'h':
			i, err := strconv.ParseInt(body, 10, 16)
			if err != nil {
				return nil, false, fmt.Errorf("invalid short literal %q", src)
			}
			return int16(i), false, nil
		case 'i':
			i, err := strconv.ParseInt(body, 10, 32)
			if err != nil {
				return nil, false, fmt.Errorf("invalid int literal %q", src)
			}
			return int32(i), false, nil
		case 'j':
			i, err := strconv.ParseInt(body, 10, 64)
			if err != nil {
				return nil, false, fmt.Errorf("invalid long literal %q", src)
			}
			return i, false, nil
		case 'e':
			f, err := strconv.ParseFloat(body, 32)
			if err != nil {
				return nil, false, fmt.Errorf("invalid real literal %q", src)
			}
			return float32(f), true, nil
		case 'f':
			f, err := strconv.ParseFloat(body, 64)
			if err != nil {
				return nil, false, fmt.Errorf("invalid float literal %q", src)
			}
			return f, true, nil
		}
	}
	if strings.ContainsAny(src, ".eE") {
		f, err := strconv.ParseFloat(src, 64)
		if err != nil {
			return nil, false, fmt.Errorf("invalid number %q", src)
		}
		return f, true, nil
	}
	i, err := strconv.ParseInt(src, 10, 64)
	if err != nil {
		return nil, false, fmt.Errorf("invalid number %q", src)
	}
	return i, false, nil
}

func commonTypedNullKind(values []any) (data.Kind, bool) {
	kind := data.Kind("")
	for _, value := range values {
		valueKind, ok := data.NullKind(value)
		if !ok || valueKind == data.KindNull || valueKind == data.KindAny {
			return "", false
		}
		merged, ok := mergeQResultKinds(kind, valueKind)
		if !ok {
			return "", false
		}
		kind = merged
	}
	return kind, kind != ""
}

func allValuesNull(values []any) bool {
	for _, value := range values {
		if !data.IsNull(value) {
			return false
		}
	}
	return len(values) > 0
}

func parseQInfinity(src string) (any, bool, bool) {
	switch src {
	case "0W", "0Wj":
		return int64(math.MaxInt64), false, true
	case "0Wh":
		return int16(math.MaxInt16), false, true
	case "0Wi":
		return int32(math.MaxInt32), false, true
	case "0w", "0Wf":
		return math.Inf(1), true, true
	case "0We":
		return float32(math.Inf(1)), true, true
	default:
		return nil, false, false
	}
}

func negateQInfinity(v any) any {
	switch x := v.(type) {
	case int16:
		return -x
	case int32:
		return -x
	case int64:
		return -x
	case float32:
		return float32(math.Inf(-1))
	case float64:
		return math.Inf(-1)
	default:
		return v
	}
}

func parseTakeCount(src string) (int, error) {
	v, err := parseAtomOrVector(src)
	if err != nil {
		return 0, fmt.Errorf("# left operand must be an integer count")
	}
	i, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("# left operand must be an integer count")
	}
	return int(i), nil
}

func (s *EvalState) evalTake(src string) (any, error) {
	fields := strings.Fields(src)
	if len(fields) < 2 {
		return nil, fmt.Errorf("take expects a count and value")
	}
	n, err := parseTakeCount(fields[0])
	if err != nil {
		return nil, err
	}
	v, err := s.eval(strings.TrimSpace(src[len(fields[0]):]))
	if err != nil {
		return nil, err
	}
	return take(n, v)
}

func (s *EvalState) evalDrop(src string) (any, error) {
	fields := strings.Fields(src)
	if len(fields) < 2 {
		return nil, fmt.Errorf("drop expects a count and value")
	}
	n, err := parseTakeCount(fields[0])
	if err != nil {
		return nil, err
	}
	v, err := s.eval(strings.TrimSpace(src[len(fields[0]):]))
	if err != nil {
		return nil, err
	}
	return drop(n, v)
}

func (s *EvalState) evalTil(src string) (any, error) {
	v, err := s.eval(src)
	if err != nil {
		return nil, err
	}
	n, ok := v.(int64)
	if !ok {
		return nil, fmt.Errorf("til expects an integer")
	}
	if n < 0 {
		return nil, fmt.Errorf("til expects a non-negative integer")
	}
	if int64(int(n)) != n {
		return nil, fmt.Errorf("til count is too large")
	}
	return data.NewI64Range(0, 1, int(n)), nil
}

func (s *EvalState) evalLookup(src string) (any, error) {
	dictExpr, keyExpr, ok := splitLookupArgs(src)
	if !ok {
		return nil, fmt.Errorf("lookup expects a dictionary and key")
	}
	d, err := s.eval(dictExpr)
	if err != nil {
		return nil, err
	}
	key, err := s.eval(keyExpr)
	if err != nil {
		return nil, err
	}
	return indexValue(d, key)
}

func splitLookupArgs(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", "", false
	}
	if strings.HasPrefix(src, "(") {
		depth := 0
		for i := 0; i < len(src); i++ {
			switch src[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					rest := strings.TrimSpace(src[i+1:])
					return strings.TrimSpace(src[:i+1]), rest, rest != ""
				}
			}
		}
		return "", "", false
	}
	fields := strings.Fields(src)
	if len(fields) < 2 {
		return "", "", false
	}
	return fields[0], strings.TrimSpace(src[len(fields[0]):]), true
}

func (s *EvalState) evalAmend(src string) (any, error) {
	if !strings.HasPrefix(src, "@[") || !strings.HasSuffix(src, "]") {
		return nil, fmt.Errorf("amend expects @[dict;key;op;value]")
	}
	inner := strings.TrimSpace(src[2 : len(src)-1])
	parts := splitTopLevelDelim(inner, ';')
	if len(parts) != 4 {
		return nil, fmt.Errorf("amend expects @[dict;key;op;value]")
	}
	d, err := s.eval(parts[0])
	if err != nil {
		return nil, err
	}
	key, err := s.eval(parts[1])
	if err != nil {
		return nil, err
	}
	value, err := s.eval(parts[3])
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parts[2]) == ":" {
		return amendValue(d, key, value)
	}
	op, err := s.eval(parts[2])
	if err != nil {
		return nil, err
	}
	return s.amendValueWithFunction(d, key, op, value)
}

func (s *EvalState) evalDotAmend(src string) (any, error) {
	if !strings.HasPrefix(src, ".[") || !strings.HasSuffix(src, "]") {
		return nil, fmt.Errorf("dot amend expects .[dict;path;op;value]")
	}
	inner := strings.TrimSpace(src[2 : len(src)-1])
	parts := splitTopLevelDelim(inner, ';')
	if len(parts) != 4 {
		return nil, fmt.Errorf("dot amend expects .[dict;path;op;value]")
	}
	target, err := s.eval(parts[0])
	if err != nil {
		return nil, err
	}
	pathValue, err := s.eval(parts[1])
	if err != nil {
		return nil, err
	}
	pathItems, err := amendPathItems(pathValue)
	if err != nil {
		return nil, err
	}
	value, err := s.eval(parts[3])
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parts[2]) == ":" {
		return s.amendPath(target, pathItems, nil, value)
	}
	op, err := s.eval(parts[2])
	if err != nil {
		return nil, err
	}
	return s.amendPath(target, pathItems, op, value)
}

func amendPathItems(v any) ([]any, error) {
	if array, ok := v.(data.Array); ok {
		return array.Values(), nil
	}
	return []any{v}, nil
}

func (s *EvalState) amendPath(target any, path []any, op any, value any) (any, error) {
	if len(path) == 0 {
		if op == nil {
			return value, nil
		}
		return s.applyCallable(op, []any{target, value})
	}
	if len(path) == 1 {
		if op == nil {
			return amendValue(target, path[0], value)
		}
		return s.amendValueWithFunction(target, path[0], op, value)
	}
	child, err := indexValue(target, path[0])
	if err != nil {
		return nil, err
	}
	next, err := s.amendPath(child, path[1:], op, value)
	if err != nil {
		return nil, err
	}
	return setContainerChild(target, path[0], next)
}

func dict(keys any, values any) (EvalDict, error) {
	keyItems, err := dictKeyItems(keys)
	if err != nil {
		return EvalDict{}, err
	}
	items, err := vectorValues(values)
	if err != nil {
		if len(keyItems) != 1 {
			return EvalDict{}, fmt.Errorf("dict key/value length mismatch")
		}
		items = []any{values}
	}
	if len(keyItems) != len(items) {
		return EvalDict{}, fmt.Errorf("dict key/value length mismatch")
	}
	return EvalDict{Keys: append([]any(nil), keyItems...), Values: append([]any(nil), items...)}, nil
}

func dictKeyItems(keys any) ([]any, error) {
	switch x := keys.(type) {
	case data.Array:
		items := x.Values()
		out := make([]any, len(items))
		copy(out, items)
		return out, nil
	default:
		return []any{x}, nil
	}
}

func dictLookup(v any, key any) (any, error) {
	d, ok := v.(EvalDict)
	if !ok {
		return nil, fmt.Errorf("lookup expects a dictionary")
	}
	keys, scalar, err := lookupKeys(key)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(keys))
	for i, k := range keys {
		var value interface{} = data.NullValue
		for j, existing := range d.Keys {
			if equalValue(existing, k) {
				value = d.Values[j]
				break
			}
		}
		if value == nil {
			value = data.NullValue
		}
		out[i] = value
	}
	if scalar {
		return out[0], nil
	}
	return data.InferArray(out), nil
}

func indexValue(v any, index any) (any, error) {
	if _, ok := v.(EvalDict); ok {
		return dictLookup(v, index)
	}
	if frame, ok := v.(data.Frame); ok {
		return frameColumnLookup(frame, index)
	}
	if keyed, ok := v.(data.KeyedFrame); ok {
		if frameHasLookupColumns(keyed.Frame(), index) {
			return frameColumnLookup(keyed.Frame(), index)
		}
		return keyedTableLookup(keyed, index)
	}
	indexes, scalar, err := indexInts(index)
	if err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case data.Array:
		if scalar {
			item, ok := x.At(indexes[0])
			if !ok {
				return nil, fmt.Errorf("index %d out of range", indexes[0])
			}
			return item, nil
		}
		return data.Gather(x, indexes)
	case string:
		runes := []rune(x)
		if scalar {
			if indexes[0] < 0 || indexes[0] >= len(runes) {
				return nil, fmt.Errorf("index %d out of range", indexes[0])
			}
			return string(runes[indexes[0]]), nil
		}
		out := make([]string, len(indexes))
		for i, index := range indexes {
			if index < 0 || index >= len(runes) {
				return nil, fmt.Errorf("index %d out of range", index)
			}
			out[i] = string(runes[index])
		}
		return data.NewString(out), nil
	default:
		return nil, fmt.Errorf("index expects a vector, string, or dictionary")
	}
}

func frameColumnLookup(frame data.Frame, key any) (any, error) {
	keys, scalar, err := lookupKeys(key)
	if err != nil {
		return nil, err
	}
	cols := make([]data.Column, 0, len(keys))
	for _, raw := range keys {
		sym, ok := raw.(data.Symbol)
		if !ok {
			if s, ok := raw.(string); ok {
				sym = data.Symbol(s)
			} else {
				return nil, fmt.Errorf("table column lookup expects symbol keys")
			}
		}
		col, ok := frame.Column(sym)
		if !ok {
			return nil, fmt.Errorf("table column %q not found", sym)
		}
		if scalar {
			return col, nil
		}
		cols = append(cols, data.Column{Name: sym, Data: col})
	}
	return data.NewFrame(cols...)
}

func frameHasLookupColumns(frame data.Frame, key any) bool {
	keys, _, err := lookupKeys(key)
	if err != nil || len(keys) == 0 {
		return false
	}
	for _, raw := range keys {
		sym, ok := raw.(data.Symbol)
		if !ok {
			if s, ok := raw.(string); ok {
				sym = data.Symbol(s)
			} else {
				return false
			}
		}
		if _, ok := frame.Column(sym); !ok {
			return false
		}
	}
	return true
}

func keyedTableLookup(keyed data.KeyedFrame, key any) (data.Frame, error) {
	if selector, ok := key.(EvalDict); ok {
		keyValues, err := keyedLookupDictValues(keyed, selector)
		if err != nil {
			return data.Frame{}, err
		}
		frame, err := keyed.LookupValueByKey(keyValues...)
		if err != nil {
			return data.Frame{}, err
		}
		return frame, nil
	}
	keyValues := []any{key}
	if array, ok := key.(data.Array); ok && len(keyed.Keys()) > 1 {
		keyValues = array.Values()
	}
	frame, err := keyed.LookupValueByKey(keyValues...)
	if err != nil {
		return data.Frame{}, err
	}
	return frame, nil
}

func keyedLookupDictValues(keyed data.KeyedFrame, selector EvalDict) ([]any, error) {
	keyValues := make([]any, 0, len(keyed.Keys()))
	for _, name := range keyed.Keys() {
		found := false
		for i, rawKey := range selector.Keys {
			sym, ok := rawKey.(data.Symbol)
			if !ok {
				return nil, fmt.Errorf("keyed table lookup selector keys must be symbols")
			}
			if sym != name {
				continue
			}
			keyValues = append(keyValues, selector.Values[i])
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("keyed table lookup selector missing key column %q", name)
		}
	}
	return keyValues, nil
}

func indexInts(v any) ([]int, bool, error) {
	switch x := v.(type) {
	case int64:
		if x < 0 || int64(int(x)) != x {
			return nil, false, fmt.Errorf("index must be a non-negative integer")
		}
		return []int{int(x)}, true, nil
	case data.Array:
		if x.Kind() != data.KindI64 {
			return nil, false, fmt.Errorf("index vector must contain integers")
		}
		values := x.Values()
		indexes := make([]int, len(values))
		for i, value := range values {
			n, ok := value.(int64)
			if !ok || n < 0 || int64(int(n)) != n {
				return nil, false, fmt.Errorf("index vector must contain non-negative integers")
			}
			indexes[i] = int(n)
		}
		return indexes, len(indexes) == 1, nil
	default:
		return nil, false, fmt.Errorf("index must be an integer or integer vector")
	}
}

func amendValue(v any, key any, value any) (any, error) {
	if d, ok := v.(EvalDict); ok {
		return dictAmend(d, key, value)
	}
	if array, ok := v.(data.Array); ok {
		return vectorAmend(array, key, value)
	}
	if frame, ok := v.(data.Frame); ok {
		return frameAmend(frame, key, value)
	}
	if keyed, ok := v.(data.KeyedFrame); ok {
		return keyedFrameAmend(keyed, key, value)
	}
	return nil, fmt.Errorf("amend expects a dictionary, vector, table, or keyed table")
}

func setContainerChild(target any, key any, value any) (any, error) {
	switch x := target.(type) {
	case EvalDict:
		return dictAmend(x, key, value)
	case data.Array:
		return vectorAmend(x, key, value)
	case data.Frame:
		if name, err := symbolName(key); err == nil {
			array, ok := value.(data.Array)
			if !ok {
				return nil, fmt.Errorf("table dot amend column %q expects a vector", name)
			}
			return frameColumnAmend(x, name, array)
		}
		return frameAmend(x, key, value)
	case data.KeyedFrame:
		if name, err := symbolName(key); err == nil {
			array, ok := value.(data.Array)
			if !ok {
				return nil, fmt.Errorf("keyed table dot amend column %q expects a vector", name)
			}
			frame, err := frameColumnAmend(x.Frame(), name, array)
			if err != nil {
				return nil, err
			}
			return data.KeyBy(frame, x.Keys()...)
		}
		return keyedFrameAmend(x, key, value)
	default:
		return nil, fmt.Errorf("dot amend path expects a dictionary, vector, table, or keyed table")
	}
}

func frameColumnAmend(frame data.Frame, name data.Symbol, array data.Array) (data.Frame, error) {
	if array.Len() != frame.Len() {
		return data.Frame{}, fmt.Errorf("table dot amend column %q length %d does not match row count %d", name, array.Len(), frame.Len())
	}
	schema := frame.Schema()
	kind, ok := schema.Kind(name)
	if !ok {
		return data.Frame{}, fmt.Errorf("table amend column %q does not exist", name)
	}
	cols := make([]data.Column, 0, len(schema.Names()))
	for _, colName := range schema.Names() {
		if colName != name {
			col, ok := frame.Column(colName)
			if !ok {
				return data.Frame{}, fmt.Errorf("table amend column %q does not exist", colName)
			}
			cols = append(cols, data.Column{Name: colName, Data: col})
			continue
		}
		values := array.Values()
		next, err := data.NewColumnWithKind(name, kind, values)
		if err != nil {
			return data.Frame{}, err
		}
		cols = append(cols, next)
	}
	return data.NewFrame(cols...)
}

func (s *EvalState) amendValueWithFunction(v any, key any, fn any, value any) (any, error) {
	if !isCallable(fn) {
		return nil, fmt.Errorf("amend function is not callable")
	}
	switch x := v.(type) {
	case EvalDict:
		return s.dictAmendFunction(x, key, fn, value)
	case data.Array:
		return s.vectorAmendFunction(x, key, fn, value)
	case data.Frame:
		return s.frameAmendFunction(x, key, fn, value)
	case data.KeyedFrame:
		return s.keyedFrameAmendFunction(x, key, fn, value)
	default:
		return nil, fmt.Errorf("amend expects a dictionary, vector, table, or keyed table")
	}
}

func (s *EvalState) dictAmendFunction(d EvalDict, key any, fn any, value any) (EvalDict, error) {
	keys, _, err := lookupKeys(key)
	if err != nil {
		return EvalDict{}, err
	}
	values, err := amendInputValues(value, len(keys))
	if err != nil {
		return EvalDict{}, err
	}
	out := d
	for i, k := range keys {
		old, err := dictLookup(out, k)
		if err != nil {
			return EvalDict{}, err
		}
		next, err := s.applyCallable(fn, []any{old, values[i]})
		if err != nil {
			return EvalDict{}, err
		}
		amended, err := dictAmend(out, k, next)
		if err != nil {
			return EvalDict{}, err
		}
		out = amended
	}
	return out, nil
}

func (s *EvalState) vectorAmendFunction(array data.Array, key any, fn any, value any) (data.Array, error) {
	indexes, _, err := indexInts(key)
	if err != nil {
		return nil, err
	}
	values, err := amendInputValues(value, len(indexes))
	if err != nil {
		return nil, err
	}
	out := array
	for i, index := range indexes {
		old, ok := out.At(index)
		if !ok {
			return nil, fmt.Errorf("amend index %d out of range", index)
		}
		next, err := s.applyCallable(fn, []any{old, values[i]})
		if err != nil {
			return nil, err
		}
		amended, err := vectorAmend(out, int64(index), next)
		if err != nil {
			return nil, err
		}
		out = amended
	}
	return out, nil
}

func (s *EvalState) frameAmendFunction(frame data.Frame, key any, fn any, value any) (data.Frame, error) {
	indexes, _, err := indexInts(key)
	if err != nil {
		return data.Frame{}, err
	}
	for _, index := range indexes {
		if index < 0 || index >= frame.Len() {
			return data.Frame{}, fmt.Errorf("amend row %d out of range", index)
		}
	}
	updates, err := tableAmendRows(value, len(indexes))
	if err != nil {
		return data.Frame{}, err
	}
	for row, update := range updates {
		for name, nextValue := range update {
			col, ok := frame.Column(name)
			if !ok {
				return data.Frame{}, fmt.Errorf("table amend column %q does not exist", name)
			}
			old, ok := col.At(indexes[row])
			if !ok {
				return data.Frame{}, fmt.Errorf("amend row %d out of range", indexes[row])
			}
			updated, err := s.applyCallable(fn, []any{old, nextValue})
			if err != nil {
				return data.Frame{}, err
			}
			update[name] = updated
		}
	}
	return applyFrameRowUpdates(frame, indexes, updates)
}

func (s *EvalState) keyedFrameAmendFunction(keyed data.KeyedFrame, key any, fn any, value any) (data.KeyedFrame, error) {
	keyRow, err := keyedSelectorRow(keyed, key)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	updates, err := tableAmendRows(value, 1)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	row := updates[0]
	for name, keyValue := range keyRow {
		if existing, ok := row[name]; ok && !equalValue(existing, keyValue) {
			return data.KeyedFrame{}, fmt.Errorf("keyed table amend key column %q conflicts with selector", name)
		}
		delete(row, name)
		_ = keyValue
	}
	hit, err := keyed.LookupValueByKey(keyedSelectorValues(keyed, keyRow)...)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	if hit.Len() == 1 {
		amended, err := s.frameAmendFunction(hit, int64(0), fn, evalDictFromRow(row))
		if err != nil {
			return data.KeyedFrame{}, err
		}
		row, err = amended.Row(0)
		if err != nil {
			return data.KeyedFrame{}, err
		}
	} else {
		for name, nextValue := range row {
			kind, ok := keyed.Frame().Schema().Kind(name)
			if !ok {
				return data.KeyedFrame{}, fmt.Errorf("table amend column %q does not exist", name)
			}
			old := keyedMissingAmendOldValue(fn, kind)
			updated, err := s.applyCallable(fn, []any{old, nextValue})
			if err != nil {
				return data.KeyedFrame{}, err
			}
			row[name] = updated
		}
	}
	for name, keyValue := range keyRow {
		row[name] = keyValue
	}
	delta, err := rowMapsFrame([]map[data.Symbol]any{row}, keyed.Frame().Schema().Names(), true)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	return data.UpsertKeyed(keyed, delta)
}

func keyedMissingAmendOldValue(fn any, kind data.Kind) any {
	if dyad, ok := fn.(qDyadicFunction); ok && dyad.name == "+" {
		if zero, ok := zeroValueForKind(kind); ok {
			return zero
		}
	}
	return data.NullForKind(kind)
}

func zeroValueForKind(kind data.Kind) (any, bool) {
	switch kind {
	case data.KindBool:
		return false, true
	case data.KindI8:
		return int8(0), true
	case data.KindI16:
		return int16(0), true
	case data.KindI32:
		return int32(0), true
	case data.KindI64:
		return int64(0), true
	case data.KindU8:
		return uint8(0), true
	case data.KindU16:
		return uint16(0), true
	case data.KindU32:
		return uint32(0), true
	case data.KindU64:
		return uint64(0), true
	case data.KindF32:
		return float32(0), true
	case data.KindF64:
		return float64(0), true
	default:
		return nil, false
	}
}

func keyedSelectorValues(keyed data.KeyedFrame, row map[data.Symbol]any) []any {
	values := make([]any, 0, len(keyed.Keys()))
	for _, name := range keyed.Keys() {
		values = append(values, row[name])
	}
	return values
}

func evalDictFromRow(row map[data.Symbol]any) EvalDict {
	keys := make([]any, 0, len(row))
	values := make([]any, 0, len(row))
	for name, value := range row {
		keys = append(keys, name)
		values = append(values, value)
	}
	return EvalDict{Keys: keys, Values: values}
}

func amendInputValues(value any, count int) ([]any, error) {
	if count == 1 {
		return []any{value}, nil
	}
	items, err := vectorValues(value)
	if err != nil {
		return nil, fmt.Errorf("amend value length mismatch")
	}
	if len(items) != count {
		return nil, fmt.Errorf("amend value length mismatch")
	}
	return items, nil
}

func vectorAmend(array data.Array, key any, value any) (data.Array, error) {
	indexes, _, err := indexInts(key)
	if err != nil {
		return nil, err
	}
	values := make([]any, len(indexes))
	if len(indexes) == 1 {
		values[0] = value
	} else {
		items, err := vectorValues(value)
		if err != nil {
			return nil, fmt.Errorf("amend value length mismatch")
		}
		if len(items) != len(indexes) {
			return nil, fmt.Errorf("amend value length mismatch")
		}
		copy(values, items)
	}
	out := array.Values()
	for i, index := range indexes {
		if index < 0 || index >= len(out) {
			return nil, fmt.Errorf("amend index %d out of range", index)
		}
		out[index] = normalizeVectorAmendValue(array.Kind(), values[i])
	}
	return inferQArray(out, array.Kind()), nil
}

func normalizeVectorAmendValue(kind data.Kind, value any) any {
	if kind == "" || kind == data.KindAny || kind == data.KindNull {
		return value
	}
	normalized, err := data.NormalizeValueForKind(kind, value)
	if err != nil {
		return value
	}
	return normalized
}

func frameAmend(frame data.Frame, key any, value any) (data.Frame, error) {
	indexes, _, err := indexInts(key)
	if err != nil {
		return data.Frame{}, err
	}
	for _, index := range indexes {
		if index < 0 || index >= frame.Len() {
			return data.Frame{}, fmt.Errorf("amend row %d out of range", index)
		}
	}
	updates, err := tableAmendRows(value, len(indexes))
	if err != nil {
		return data.Frame{}, err
	}
	return applyFrameRowUpdates(frame, indexes, updates)
}

func keyedFrameAmend(keyed data.KeyedFrame, key any, value any) (data.KeyedFrame, error) {
	keyRow, err := keyedSelectorRow(keyed, key)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	updates, err := tableAmendRows(value, 1)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	row := updates[0]
	for name, keyValue := range keyRow {
		if existing, ok := row[name]; ok && !equalValue(existing, keyValue) {
			return data.KeyedFrame{}, fmt.Errorf("keyed table amend key column %q conflicts with selector", name)
		}
		row[name] = keyValue
	}
	delta, err := rowMapsFrame([]map[data.Symbol]any{row}, keyed.Frame().Schema().Names(), true)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	return data.UpsertKeyed(keyed, delta)
}

func keyedSelectorRow(keyed data.KeyedFrame, key any) (map[data.Symbol]any, error) {
	keys := keyed.Keys()
	if len(keys) == 0 {
		return nil, fmt.Errorf("keyed table is not initialized")
	}
	values := []any{key}
	if len(keys) > 1 {
		array, ok := key.(data.Array)
		if !ok {
			return nil, fmt.Errorf("keyed table amend expects %d key values", len(keys))
		}
		values = array.Values()
	}
	if len(values) != len(keys) {
		return nil, fmt.Errorf("keyed table amend expects %d key values", len(keys))
	}
	out := make(map[data.Symbol]any, len(keys))
	for i, name := range keys {
		out[name] = values[i]
	}
	return out, nil
}

func applyFrameRowUpdates(frame data.Frame, indexes []int, updates []map[data.Symbol]any) (data.Frame, error) {
	if len(indexes) != len(updates) {
		return data.Frame{}, fmt.Errorf("amend update row count mismatch")
	}
	schema := frame.Schema()
	known := make(map[data.Symbol]struct{}, len(schema.Names()))
	for _, name := range schema.Names() {
		known[name] = struct{}{}
	}
	for _, update := range updates {
		for name := range update {
			if _, ok := known[name]; !ok {
				return data.Frame{}, fmt.Errorf("table amend column %q does not exist", name)
			}
		}
	}
	cols := make([]data.Column, 0, len(schema.Names()))
	for _, name := range schema.Names() {
		col, ok := frame.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("table amend column %q does not exist", name)
		}
		values := col.Values()
		for i, index := range indexes {
			if v, ok := updates[i][name]; ok {
				values[index] = v
			}
		}
		next, err := data.NewColumnWithKind(name, col.Kind(), values)
		if err != nil {
			return data.Frame{}, err
		}
		cols = append(cols, next)
	}
	return data.NewFrame(cols...)
}

func tableAmendRows(value any, count int) ([]map[data.Symbol]any, error) {
	if count <= 0 {
		return nil, fmt.Errorf("amend requires at least one target row")
	}
	switch x := value.(type) {
	case EvalDict:
		return dictRowUpdates(x, count)
	case data.Frame:
		return frameRowUpdates(x, count)
	case data.KeyedFrame:
		return frameRowUpdates(x.Frame(), count)
	default:
		return nil, fmt.Errorf("table amend expects a dictionary or table value")
	}
}

func dictRowUpdates(d EvalDict, count int) ([]map[data.Symbol]any, error) {
	out := make([]map[data.Symbol]any, count)
	for i := range out {
		out[i] = make(map[data.Symbol]any, len(d.Keys))
	}
	for i, rawKey := range d.Keys {
		name, err := symbolName(rawKey)
		if err != nil {
			return nil, fmt.Errorf("table amend: %w", err)
		}
		values := make([]any, count)
		if array, ok := d.Values[i].(data.Array); ok && array.Len() == count {
			copy(values, array.Values())
		} else {
			for row := range values {
				values[row] = d.Values[i]
			}
		}
		for row := range out {
			out[row][name] = values[row]
		}
	}
	return out, nil
}

func frameRowUpdates(frame data.Frame, count int) ([]map[data.Symbol]any, error) {
	if frame.Len() != count {
		return nil, fmt.Errorf("table amend value row count %d does not match target row count %d", frame.Len(), count)
	}
	out := make([]map[data.Symbol]any, count)
	for row := 0; row < count; row++ {
		values, err := frame.Row(row)
		if err != nil {
			return nil, err
		}
		out[row] = values
	}
	return out, nil
}

func rowMapsFrame(rows []map[data.Symbol]any, preferred []data.Symbol, allowNewColumns bool) (data.Frame, error) {
	if len(rows) == 0 {
		return data.Frame{}, fmt.Errorf("table amend requires at least one row")
	}
	names := make([]data.Symbol, 0, len(preferred))
	seen := make(map[data.Symbol]struct{}, len(preferred))
	for _, name := range preferred {
		if name == "" {
			continue
		}
		if _, ok := rows[0][name]; !ok && !allowNewColumns {
			continue
		}
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for _, row := range rows {
		for name := range row {
			if _, ok := seen[name]; ok {
				continue
			}
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}
	cols := make([]data.Column, 0, len(names))
	for _, name := range names {
		values := make([]any, len(rows))
		for row := range rows {
			if v, ok := rows[row][name]; ok {
				values[row] = v
			} else {
				values[row] = data.NullValue
			}
		}
		cols = append(cols, data.NewColumn(name, values))
	}
	return data.NewFrame(cols...)
}

func symbolName(v any) (data.Symbol, error) {
	switch x := v.(type) {
	case data.Symbol:
		return x, nil
	case string:
		if x == "" {
			return "", fmt.Errorf("column name must not be empty")
		}
		return data.Symbol(x), nil
	default:
		return "", fmt.Errorf("table amend column names must be symbols")
	}
}

func dictAmend(d EvalDict, key any, value any) (EvalDict, error) {
	keys, _, err := lookupKeys(key)
	if err != nil {
		return EvalDict{}, err
	}
	values := make([]any, len(keys))
	if len(keys) == 1 {
		values[0] = value
	} else {
		items, err := vectorValues(value)
		if err != nil {
			return EvalDict{}, fmt.Errorf("amend value length mismatch")
		}
		if len(items) != len(keys) {
			return EvalDict{}, fmt.Errorf("amend value length mismatch")
		}
		copy(values, items)
	}

	out := EvalDict{
		Keys:   append([]any(nil), d.Keys...),
		Values: append([]any(nil), d.Values...),
	}
	for i, key := range keys {
		found := false
		for j, existing := range out.Keys {
			if equalValue(existing, key) {
				out.Values[j] = values[i]
				found = true
				break
			}
		}
		if !found {
			out.Keys = append(out.Keys, key)
			out.Values = append(out.Values, values[i])
		}
	}
	return out, nil
}

func lookupKeys(v any) ([]any, bool, error) {
	switch x := v.(type) {
	case data.Symbol:
		return []any{x}, true, nil
	case data.Array:
		keys := x.Values()
		return keys, len(keys) == 1, nil
	default:
		return []any{x}, true, nil
	}
}

func dictSymbolKeys(d EvalDict) ([]data.Symbol, error) {
	keys := make([]data.Symbol, len(d.Keys))
	for i, key := range d.Keys {
		sym, ok := key.(data.Symbol)
		if !ok {
			return nil, fmt.Errorf("dictionary key %d is %T, want symbol", i, key)
		}
		keys[i] = sym
	}
	return keys, nil
}

func keyedTableByCount(keys any, values any) (any, bool, error) {
	frame, ok := values.(data.Frame)
	if !ok {
		return nil, false, nil
	}
	count, ok := keys.(int64)
	if !ok {
		return nil, false, nil
	}
	if count < 0 || int64(int(count)) != count {
		return nil, true, fmt.Errorf("keyed table key count must be a non-negative integer")
	}
	names := frame.Schema().Names()
	n := int(count)
	if n > len(names) {
		return nil, true, fmt.Errorf("keyed table key count %d exceeds column count %d", n, len(names))
	}
	if n == 0 {
		return frame, true, nil
	}
	keyed, err := data.KeyBy(frame, names[:n]...)
	if err != nil {
		return nil, true, fmt.Errorf("keyed table: %w", err)
	}
	return keyed, true, nil
}

func keyedTable(keys any, values any) (data.KeyedFrame, bool, error) {
	keyFrame, keyOK := keys.(data.Frame)
	valueFrame, valueOK := values.(data.Frame)
	if !keyOK && !valueOK {
		return data.KeyedFrame{}, false, nil
	}
	if !keyOK || !valueOK {
		return data.KeyedFrame{}, true, fmt.Errorf("keyed table expects key and value tables")
	}
	if keyFrame.Len() != valueFrame.Len() {
		return data.KeyedFrame{}, true, fmt.Errorf("keyed table key/value row length mismatch")
	}
	keyNames := keyFrame.Schema().Names()
	cols := make([]data.Column, 0, len(keyNames)+len(valueFrame.Schema().Names()))
	cols = append(cols, keyFrame.Columns()...)
	cols = append(cols, valueFrame.Columns()...)
	frame, err := data.NewFrame(cols...)
	if err != nil {
		return data.KeyedFrame{}, true, fmt.Errorf("keyed table: %w", err)
	}
	keyed, err := data.KeyBy(frame, keyNames...)
	if err != nil {
		return data.KeyedFrame{}, true, fmt.Errorf("keyed table: %w", err)
	}
	return keyed, true, nil
}

func flip(v any) (data.Frame, error) {
	if _, ok := v.(data.KeyedFrame); ok {
		return data.Frame{}, fmt.Errorf("flip expects a plain dictionary, got keyed table")
	}
	d, ok := v.(EvalDict)
	if !ok {
		return data.Frame{}, fmt.Errorf("flip expects a dictionary")
	}
	keys, err := dictSymbolKeys(d)
	if err != nil {
		return data.Frame{}, fmt.Errorf("flip expects symbol column names: %w", err)
	}
	values := make(map[data.Symbol]any, len(d.Keys))
	for i, key := range keys {
		values[key] = d.Values[i]
	}
	rows := 1
	for _, name := range keys {
		if array, ok := values[name].(data.Array); ok && array.Len() > rows {
			rows = array.Len()
		}
	}
	cols := make([]data.Column, 0, len(keys))
	for _, name := range keys {
		array, ok := values[name].(data.Array)
		if !ok {
			repeated := make([]any, rows)
			for i := range repeated {
				repeated[i] = values[name]
			}
			array = data.InferArray(repeated)
		}
		cols = append(cols, data.Column{Name: name, Data: array})
	}
	return data.NewFrame(cols...)
}

func applyDyadic(op byte, left, right any) (any, error) {
	if op == '~' {
		return matchValue(left, right), nil
	}
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if lok || rok {
		return applyVectorDyadic(op, left, right, la, ra)
	}
	switch op {
	case 'L':
		return left, nil
	case 'R':
		return right, nil
	case '&', '|':
		return applyDyadicLogical(op, left, right)
	}
	if data.IsNull(left) || data.IsNull(right) {
		switch op {
		case 'm':
			return minDyadic(left, right)
		case 'M':
			return maxDyadic(left, right)
		case '+', '-', '*', '%', 'r', 'd':
			if !hasTypedNullKind(left) && !hasTypedNullKind(right) {
				return data.NullValue, nil
			}
			return data.NullForKind(qDyadicResultKind(op, left, right)), nil
		case '^':
			if data.IsNull(right) {
				if data.IsNull(left) {
					return data.NullValue, nil
				}
				return left, nil
			}
			return right, nil
		case '=':
			return data.IsNull(left) && data.IsNull(right), nil
		case '<', '>':
			cmp, err := compareSortable(left, right)
			if err != nil {
				return nil, err
			}
			if op == '<' {
				return cmp < 0, nil
			}
			return cmp > 0, nil
		}
	}
	if op == 'm' {
		return minDyadic(left, right)
	}
	if op == 'M' {
		return maxDyadic(left, right)
	}
	if op == '^' {
		return right, nil
	}
	ln, lok := numeric(left)
	rn, rok := numeric(right)
	if !lok || !rok {
		switch op {
		case '=':
			return equalComparableValue(left, right), nil
		case '<', '>':
			cmp, err := compareOrdered(left, right)
			if err != nil {
				return nil, err
			}
			if op == '<' {
				return cmp < 0, nil
			}
			return cmp > 0, nil
		default:
			return nil, fmt.Errorf("operator %q expects numeric operands", string(op))
		}
	}
	if li, ok := integerValue(left); ok && op != '%' {
		if ri, ok := integerValue(right); ok {
			switch op {
			case '+':
				return li + ri, nil
			case '-':
				return li - ri, nil
			case '*':
				return li * ri, nil
			case 'r':
				if ri == 0 {
					return data.NullValue, nil
				}
				return li - ri*int64(math.Floor(float64(li)/float64(ri))), nil
			case 'd':
				return floorDivInt(li, ri)
			case '=':
				return li == ri, nil
			case '<':
				return li < ri, nil
			case '>':
				return li > ri, nil
			}
		}
	}
	switch op {
	case 'm':
		return minDyadic(left, right)
	case 'M':
		return maxDyadic(left, right)
	case '^':
		return right, nil
	case '+':
		return ln + rn, nil
	case '-':
		return ln - rn, nil
	case '*':
		return ln * rn, nil
	case '%':
		return ln / rn, nil
	case 'r':
		if rn == 0 {
			return data.NullValue, nil
		}
		return ln - rn*math.Floor(ln/rn), nil
	case 'd':
		if rn == 0 {
			return data.NullValue, nil
		}
		return int64(math.Floor(ln / rn)), nil
	case '=':
		return ln == rn, nil
	case '<':
		return ln < rn, nil
	case '>':
		return ln > rn, nil
	default:
		return nil, fmt.Errorf("operator %q is not supported", string(op))
	}
}

func hasTypedNullKind(v any) bool {
	kind, ok := data.NullKind(v)
	return ok && kind != data.KindNull
}

func applyDyadicLogical(op byte, left, right any) (any, error) {
	lv, err := boolValue(left)
	if err != nil {
		return nil, err
	}
	rv, err := boolValue(right)
	if err != nil {
		return nil, err
	}
	switch op {
	case '&':
		return lv && rv, nil
	case '|':
		return lv || rv, nil
	default:
		return nil, fmt.Errorf("operator %q is not a logical verb", string(op))
	}
}

func dyadicVerbFunc(op byte) func(any, any) (any, error) {
	return func(left, right any) (any, error) {
		return applyDyadic(op, left, right)
	}
}

func modValue(left, right any) (any, error) {
	return applyDyadic('r', left, right)
}

func floorDivInt(left, right int64) (any, error) {
	if right == 0 {
		return data.NullValue, nil
	}
	if left == math.MinInt64 && right == -1 {
		return data.NullValue, nil
	}
	q := left / right
	r := left % right
	if r != 0 && ((r > 0) != (right > 0)) {
		q--
	}
	return q, nil
}

func minDyadic(left, right any) (any, error) {
	if data.IsNull(left) {
		return right, nil
	}
	if data.IsNull(right) {
		return left, nil
	}
	cmp, err := compareOrdered(left, right)
	if err != nil {
		return nil, err
	}
	if cmp <= 0 {
		return left, nil
	}
	return right, nil
}

func maxDyadic(left, right any) (any, error) {
	if data.IsNull(left) {
		return right, nil
	}
	if data.IsNull(right) {
		return left, nil
	}
	cmp, err := compareOrdered(left, right)
	if err != nil {
		return nil, err
	}
	if cmp >= 0 {
		return left, nil
	}
	return right, nil
}

func applyCompositeDyadic(op string, left, right any) (any, error) {
	var base any
	var err error
	switch op {
	case "<>":
		base, err = applyDyadic('=', left, right)
	case "<=":
		base, err = applyDyadic('>', left, right)
	case ">=":
		base, err = applyDyadic('<', left, right)
	default:
		return nil, fmt.Errorf("operator %q is not supported", op)
	}
	if err != nil {
		return nil, err
	}
	return invertBoolResult(base)
}

func invertBoolResult(v any) (any, error) {
	switch x := v.(type) {
	case bool:
		return !x, nil
	case data.Array:
		if x.Kind() != data.KindBool {
			return nil, fmt.Errorf("composite comparison produced non-bool vector")
		}
		values := x.Values()
		out := make([]bool, len(values))
		for i, value := range values {
			b, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("composite comparison produced non-bool vector")
			}
			out[i] = !b
		}
		return data.NewBool(out), nil
	default:
		return nil, fmt.Errorf("composite comparison produced %T", v)
	}
}

func applyVectorDyadic(op byte, left, right any, la, ra data.Array) (data.Array, error) {
	n := 0
	switch {
	case la != nil && ra != nil:
		switch {
		case la.Len() == ra.Len():
			n = la.Len()
		case la.Len() == 1:
			n = ra.Len()
		case ra.Len() == 1:
			n = la.Len()
		default:
			return nil, fmt.Errorf("vector length mismatch")
		}
	case la != nil:
		n = la.Len()
	case ra != nil:
		n = ra.Len()
	}
	if dataOp, ok := qDataArithmeticOp(op); ok {
		shape := qRuntimeKernelVectorDyadicShape(op, left, right, la, ra)
		typedLeft, typedRight, canUse, err := qVectorDyadicTypedOperands(left, right, la, ra)
		if err != nil {
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "error", "runtime_error")
			return nil, err
		}
		if !canUse || !qVectorDyadicCanUseTypedArithmetic(typedLeft, typedRight) {
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "attempt", "attempt")
			recordRuntimeKernelExecution("ArrayDyadicArithmetic", shape, "fallback", "unsupported_shape")
		} else if out, handled, err := qTryTypedArithmeticDyadic(dataOp, typedLeft, typedRight); err != nil || handled {
			recordRuntimeKernelProbe("ArrayDyadicArithmetic", shape, handled, err)
			if err != nil {
				return nil, err
			}
			if array, ok := out.(data.Array); ok {
				return array, nil
			}
		} else {
			recordRuntimeKernelProbe("ArrayDyadicArithmetic", shape, handled, err)
		}
	}
	if dataOp, ok := qDataComparisonOp(op); ok {
		shape := qRuntimeKernelVectorDyadicShape(op, left, right, la, ra)
		if !qVectorDyadicCanUseTypedCompare(left, right, la, ra) {
			recordRuntimeKernelExecution("ArrayDyadicCompare", shape, "attempt", "attempt")
			recordRuntimeKernelExecution("ArrayDyadicCompare", shape, "fallback", "unsupported_shape")
		} else if out, handled, err := data.TryTypedDyadic(dataOp, left, right); err != nil || handled {
			recordRuntimeKernelProbe("ArrayDyadicCompare", shape, handled, err)
			if err != nil {
				return nil, err
			}
			if array, ok := out.(data.Array); ok {
				return array, nil
			}
		} else {
			recordRuntimeKernelProbe("ArrayDyadicCompare", shape, handled, err)
		}
	}
	out := make([]any, n)
	hasFloat := op == '%'
	hasNull := false
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if la != nil {
			var ok bool
			row := i
			if la.Len() == 1 {
				row = 0
			}
			lv, ok = la.At(row)
			if !ok {
				return nil, fmt.Errorf("left vector row %d out of range", i)
			}
		}
		if ra != nil {
			var ok bool
			row := i
			if ra.Len() == 1 {
				row = 0
			}
			rv, ok = ra.At(row)
			if !ok {
				return nil, fmt.Errorf("right vector row %d out of range", i)
			}
		}
		v, err := applyDyadic(op, lv, rv)
		if err != nil {
			return nil, err
		}
		if data.IsNull(v) {
			hasNull = true
			out[i] = data.NullValue
			continue
		}
		if b, ok := v.(bool); ok {
			out[i] = b
			continue
		}
		if _, ok := v.(float64); ok {
			hasFloat = true
		}
		out[i] = v
	}
	if n == 0 {
		return data.InferArray(out), nil
	}
	if _, ok := out[0].(bool); ok {
		xs := make([]bool, n)
		for i, v := range out {
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("operator %q produced mixed result types", string(op))
			}
			xs[i] = b
		}
		return data.NewBool(xs), nil
	}
	if hasNull {
		return inferQArray(out, qDyadicResultKind(op, left, right)), nil
	}
	if hasFloat {
		xs := make([]float64, n)
		for i, v := range out {
			f, ok := numeric(v)
			if !ok {
				return nil, fmt.Errorf("operator %q expects numeric operands", string(op))
			}
			xs[i] = f
		}
		return data.NewF64(xs), nil
	}
	xs := make([]int64, n)
	for i, v := range out {
		x, ok := integerValue(v)
		if !ok {
			return data.InferArray(out), nil
		}
		xs[i] = x
	}
	return data.NewI64(xs), nil
}

func qDataArithmeticOp(op byte) (data.Op, bool) {
	switch op {
	case '+':
		return data.OpAdd, true
	case '-':
		return data.OpSub, true
	case '*':
		return data.OpMul, true
	case '%':
		return data.OpDiv, true
	default:
		return "", false
	}
}

func qTryTypedArithmeticDyadic(op data.Op, left, right any) (any, bool, error) {
	if op != data.OpDiv && qTypedIntegerOperandOK(left) && qTypedIntegerOperandOK(right) {
		return data.TryTypedIntegerDyadic(op, left, right)
	}
	return data.TryTypedDyadic(op, left, right)
}

func qDataComparisonOp(op byte) (data.Op, bool) {
	switch op {
	case '=':
		return data.OpEQ, true
	case '<':
		return data.OpLT, true
	case '>':
		return data.OpGT, true
	default:
		return "", false
	}
}

func qRuntimeKernelVectorDyadicShape(op byte, left, right any, la, ra data.Array) string {
	leftKind := qRuntimeKernelOperandKind(left, la)
	rightKind := qRuntimeKernelOperandKind(right, ra)
	return "vector-dyadic/" + string(op) + "/" + string(leftKind) + "/" + string(rightKind)
}

func qVectorDyadicTypedOperands(left, right any, la, ra data.Array) (any, any, bool, error) {
	typedLeft := left
	typedRight := right
	if la != nil && ra != nil {
		switch {
		case la.Len() == ra.Len():
		case la.Len() == 1:
			value, ok := la.At(0)
			if !ok {
				return nil, nil, false, fmt.Errorf("left vector row 0 out of range")
			}
			typedLeft = value
		case ra.Len() == 1:
			value, ok := ra.At(0)
			if !ok {
				return nil, nil, false, fmt.Errorf("right vector row 0 out of range")
			}
			typedRight = value
		default:
			return nil, nil, false, nil
		}
	}
	return typedLeft, typedRight, true, nil
}

func qRuntimeKernelOperandKind(value any, array data.Array) data.Kind {
	if array != nil {
		return array.Kind()
	}
	kind := qKindOfValue(value)
	if kind == "" {
		return data.KindAny
	}
	return kind
}

func qVectorDyadicCanUseTypedArithmetic(left, right any) bool {
	return qTypedArithmeticOperandOK(left) && qTypedArithmeticOperandOK(right)
}

func qTypedArithmeticOperandOK(value any) bool {
	if array, ok := value.(data.Array); ok {
		return qKindIsNumeric(array.Kind())
	}
	if data.IsNull(value) {
		return true
	}
	return qKindIsNumeric(qKindOfValue(value))
}

func qTypedIntegerOperandOK(value any) bool {
	if array, ok := value.(data.Array); ok {
		return qKindIsInteger(array.Kind())
	}
	if kind, ok := data.NullKind(value); ok {
		return qKindIsInteger(kind)
	}
	return qKindIsInteger(qKindOfValue(value))
}

func qVectorDyadicCanUseTypedCompare(left, right any, la, ra data.Array) bool {
	if la != nil && ra != nil && la.Len() != ra.Len() {
		return false
	}
	return qTypedCompareOperandOK(left, la) && qTypedCompareOperandOK(right, ra)
}

func qTypedCompareOperandOK(value any, array data.Array) bool {
	if array != nil {
		return qTypedCompareKindOK(array.Kind())
	}
	return qTypedCompareKindOK(qKindOfValue(value))
}

func qTypedCompareKindOK(kind data.Kind) bool {
	switch kind {
	case data.KindBool,
		data.KindI8, data.KindI16, data.KindI32, data.KindI64,
		data.KindU8, data.KindU16, data.KindU32, data.KindU64,
		data.KindF32, data.KindF64,
		data.KindMonth, data.KindDate, data.KindDateTime,
		data.KindTimespan, data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		return true
	default:
		return false
	}
}

func vectorIndex(i, length int) int {
	if length == 1 {
		return 0
	}
	return i
}

func inferQArray(values []any, hints ...data.Kind) data.Array {
	inferred := data.InferArray(values)
	if len(values) == 0 {
		if typed, ok := emptyQArrayFromHints(hints...); ok {
			return typed
		}
		return inferred
	}
	if inferred.Kind() != data.KindNull {
		return inferred
	}
	kind := data.Kind("")
	for _, value := range values {
		valueKind := qKindOfValue(value)
		if valueKind == "" || valueKind == data.KindNull || valueKind == data.KindAny {
			continue
		}
		merged, ok := mergeQResultKinds(kind, valueKind)
		if !ok {
			return inferred
		}
		kind = merged
	}
	for _, hint := range hints {
		if hint == "" || hint == data.KindNull || hint == data.KindAny {
			continue
		}
		merged, ok := mergeQResultKinds(kind, hint)
		if !ok {
			return inferred
		}
		kind = merged
	}
	if kind == "" {
		return inferred
	}
	column, err := data.NewColumnWithKind("_", kind, values)
	if err != nil {
		return inferred
	}
	return column.Data
}

func emptyQArrayFromHints(hints ...data.Kind) (data.Array, bool) {
	kind := data.Kind("")
	for _, hint := range hints {
		if hint == "" || hint == data.KindNull || hint == data.KindAny {
			continue
		}
		merged, ok := mergeQResultKinds(kind, hint)
		if !ok {
			return nil, false
		}
		kind = merged
	}
	if kind == "" {
		return nil, false
	}
	column, err := data.NewColumnWithKind("_", kind, nil)
	if err != nil {
		return nil, false
	}
	return column.Data, true
}

func qDyadicResultKind(op byte, left, right any) data.Kind {
	switch op {
	case '=', '<', '>', '&', '|', '~':
		return data.KindBool
	case '%':
		return data.KindF64
	case '+', '-', '*', 'r', 'd':
		leftKind := qKindOfValue(left)
		rightKind := qKindOfValue(right)
		if qKindIsNumeric(leftKind) || qKindIsNumeric(rightKind) {
			kind, ok := mergeQResultKinds(leftKind, rightKind)
			if ok {
				return kind
			}
		}
	case 'm', 'M', '^':
		kind, ok := mergeQResultKinds(qKindOfValue(left), qKindOfValue(right))
		if ok {
			return kind
		}
	case 'L':
		return qKindOfValue(left)
	case 'R':
		return qKindOfValue(right)
	}
	return ""
}

func qKindOfValue(v any) data.Kind {
	if kind, ok := data.NullKind(v); ok {
		return kind
	}
	switch x := v.(type) {
	case data.Array:
		return x.Kind()
	case bool:
		return data.KindBool
	case int8:
		return data.KindI8
	case int16:
		return data.KindI16
	case int32:
		return data.KindI32
	case int, int64:
		return data.KindI64
	case uint8:
		return data.KindU8
	case uint16:
		return data.KindU16
	case uint32:
		return data.KindU32
	case uint64:
		return data.KindU64
	case float32:
		return data.KindF32
	case float64:
		return data.KindF64
	case string:
		return data.KindString
	case data.Symbol:
		return data.KindSymbol
	case data.Month:
		return data.KindMonth
	case data.Date:
		return data.KindDate
	case data.DateTime:
		return data.KindDateTime
	case data.Timespan:
		return data.KindTimespan
	case data.Minute:
		return data.KindMinute
	case data.Second:
		return data.KindSecond
	case data.Time:
		return data.KindTime
	case data.Timestamp:
		return data.KindTimestamp
	default:
		return ""
	}
}

func mergeQResultKinds(left, right data.Kind) (data.Kind, bool) {
	if left == "" || left == data.KindNull || left == data.KindAny {
		return right, true
	}
	if right == "" || right == data.KindNull || right == data.KindAny {
		return left, true
	}
	if left == right {
		return left, true
	}
	if qKindIsNumeric(left) && qKindIsNumeric(right) {
		if left == data.KindF64 || right == data.KindF64 || left == data.KindF32 || right == data.KindF32 {
			return data.KindF64, true
		}
		return data.KindI64, true
	}
	if merged, ok := mergeTemporalKinds(left, right); ok {
		return merged, true
	}
	return "", false
}

func qKindIsNumeric(kind data.Kind) bool {
	switch kind {
	case data.KindI8, data.KindI16, data.KindI32, data.KindI64, data.KindF32, data.KindF64:
		return true
	default:
		return false
	}
}

func qKindIsInteger(kind data.Kind) bool {
	switch kind {
	case data.KindI8, data.KindI16, data.KindI32, data.KindI64:
		return true
	default:
		return false
	}
}

func count(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return int64(0), nil
	case qEnumVector:
		return int64(x.Len()), nil
	case data.Array:
		return int64(x.Len()), nil
	case data.Frame:
		return int64(x.Len()), nil
	case EvalDict:
		return int64(len(x.Keys)), nil
	case string:
		return int64(len(x)), nil
	default:
		return int64(1), nil
	}
}

func enlist(v any) (any, error) {
	return data.NewAny([]any{v}), nil
}

func typeOf(v any) (any, error) {
	if kind, ok := data.NullKind(v); ok && kind != data.KindNull {
		return int64(qTypeCode(kind, true)), nil
	}
	if data.IsNull(v) {
		return int64(101), nil
	}
	switch x := v.(type) {
	case bool:
		return int64(-1), nil
	case int16:
		return int64(-5), nil
	case int32:
		return int64(-6), nil
	case int64:
		return int64(-7), nil
	case uint8:
		return int64(-4), nil
	case float32:
		return int64(-8), nil
	case float64:
		return int64(-9), nil
	case string:
		return int64(10), nil
	case data.Symbol:
		return int64(-11), nil
	case data.Month:
		return int64(-13), nil
	case data.Date:
		return int64(-14), nil
	case data.DateTime:
		return int64(-15), nil
	case data.Timespan:
		return int64(-16), nil
	case data.Minute:
		return int64(-17), nil
	case data.Second:
		return int64(-18), nil
	case data.Time:
		return int64(-19), nil
	case data.Timestamp:
		return int64(-12), nil
	case qEnumVector:
		return int64(qTypeCode(x.Kind(), false)), nil
	case data.Array:
		return int64(qTypeCode(x.Kind(), false)), nil
	case EvalDict:
		return int64(99), nil
	case data.Frame:
		return int64(98), nil
	default:
		return int64(0), nil
	}
}

func qTypeCode(kind data.Kind, atom bool) int {
	code := 0
	switch kind {
	case data.KindBool:
		code = 1
	case data.KindI8, data.KindU8:
		code = 4
	case data.KindI16:
		code = 5
	case data.KindI32:
		code = 6
	case data.KindI64:
		code = 7
	case data.KindF32:
		code = 8
	case data.KindF64:
		code = 9
	case data.KindString:
		code = 10
	case data.KindSymbol:
		code = 11
	case data.KindMonth:
		code = 13
	case data.KindTimestamp:
		code = 12
	case data.KindDate:
		code = 14
	case data.KindDateTime:
		code = 15
	case data.KindTimespan:
		code = 16
	case data.KindMinute:
		code = 17
	case data.KindSecond:
		code = 18
	case data.KindTime:
		code = 19
	case data.KindNull:
		code = 101
	default:
		code = 0
	}
	if atom && code > 0 {
		return -code
	}
	return code
}

func stringValue(v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		out := make([]string, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, fmt.Errorf("string row %d out of range", i)
			}
			out[i] = qStringScalar(item)
		}
		return data.NewString(out), nil
	}
	return qStringScalar(v), nil
}

func qStringScalar(v any) string {
	if data.IsNull(v) {
		return ""
	}
	switch x := v.(type) {
	case data.Symbol:
		return string(x)
	}
	if s, ok := FormatTemporal(v); ok {
		return s
	}
	return fmt.Sprint(v)
}

func lowerValue(v any) (any, error) {
	return mapStringValue("lower", v, strings.ToLower)
}

func upperValue(v any) (any, error) {
	return mapStringValue("upper", v, strings.ToUpper)
}

func mapStringValue(name string, v any, fn func(string) string) (any, error) {
	if array, ok := v.(data.Array); ok {
		out := make([]string, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, fmt.Errorf("%s row %d out of range", name, i)
			}
			if data.IsNull(item) {
				out[i] = ""
				continue
			}
			switch x := item.(type) {
			case string:
				out[i] = fn(x)
			case data.Symbol:
				out[i] = fn(string(x))
			default:
				return nil, fmt.Errorf("%s expects string or symbol values", name)
			}
		}
		return data.NewString(out), nil
	}
	if data.IsNull(v) {
		return "", nil
	}
	switch x := v.(type) {
	case string:
		return fn(x), nil
	case data.Symbol:
		return fn(string(x)), nil
	default:
		return nil, fmt.Errorf("%s expects string or symbol values", name)
	}
}

func sum(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("sum expects a numeric vector")
	}
	if array.Len() == 0 {
		return data.NullValue, nil
	}
	if out, handled, err := data.TryTypedNumericSum(array); err != nil || handled {
		recordRuntimeKernelProbe("ArraySum", "vector-reduce/sum/"+string(array.Kind()), handled, err)
		if err != nil {
			return nil, err
		}
		return out, nil
	} else {
		recordRuntimeKernelProbe("ArraySum", "vector-reduce/sum/"+string(array.Kind()), handled, err)
	}
	totalI := int64(0)
	totalF := float64(0)
	hasFloat := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("sum row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		if n, ok := integerValue(item); ok {
			totalI += n
			totalF += float64(n)
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF += float64(n)
		case float64:
			hasFloat = true
			totalF += n
		default:
			return nil, fmt.Errorf("sum expects a numeric vector")
		}
	}
	if hasFloat {
		return totalF, nil
	}
	return totalI, nil
}

func avg(v any) (any, error) {
	if n, ok := numeric(v); ok {
		return n, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("avg expects a numeric vector")
	}
	total := float64(0)
	count := 0
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("avg row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return nil, fmt.Errorf("avg expects a numeric vector")
		}
		total += n
		count++
	}
	if count == 0 {
		return data.NullValue, nil
	}
	return total / float64(count), nil
}

func varValue(v any) (any, error) {
	total, sumsq, count, err := numericAggregateStats("var", v)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return data.NullValue, nil
	}
	mean := total / float64(count)
	return sumsq/float64(count) - mean*mean, nil
}

func devValue(v any) (any, error) {
	variance, err := varValue(v)
	if err != nil {
		return nil, err
	}
	if data.IsNull(variance) {
		return data.NullValue, nil
	}
	n, ok := numeric(variance)
	if !ok {
		return nil, fmt.Errorf("dev expects a numeric vector")
	}
	return math.Sqrt(n), nil
}

func medValue(v any) (any, error) {
	if n, ok := numeric(v); ok {
		return n, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("med expects a numeric vector")
	}
	values := make([]float64, 0, array.Len())
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("med row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return nil, fmt.Errorf("med expects a numeric vector")
		}
		values = append(values, n)
	}
	if len(values) == 0 {
		return data.NullValue, nil
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid], nil
	}
	return (values[mid-1] + values[mid]) / 2, nil
}

func numericAggregateStats(name string, v any) (float64, float64, int, error) {
	if n, ok := numeric(v); ok {
		return n, n * n, 1, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return 0, 0, 0, fmt.Errorf("%s expects a numeric vector", name)
	}
	total := float64(0)
	sumsq := float64(0)
	count := 0
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return 0, 0, 0, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return 0, 0, 0, fmt.Errorf("%s expects a numeric vector", name)
		}
		total += n
		sumsq += n * n
		count++
	}
	return total, sumsq, count, nil
}

func wavg(weights, values any) (any, error) {
	weightArray, weightIsArray := weights.(data.Array)
	valueArray, valueIsArray := values.(data.Array)
	if !weightIsArray && !valueIsArray {
		w, wok := numeric(weights)
		v, vok := numeric(values)
		if !wok || !vok {
			return nil, fmt.Errorf("wavg expects numeric weights and values")
		}
		if w == 0 {
			return data.NullValue, nil
		}
		return v, nil
	}
	length := 1
	if weightIsArray {
		length = weightArray.Len()
	}
	if valueIsArray {
		if weightIsArray && valueArray.Len() != length {
			return nil, fmt.Errorf("wavg weight length %d does not match value length %d", length, valueArray.Len())
		}
		length = valueArray.Len()
	}
	total := float64(0)
	denom := float64(0)
	for i := 0; i < length; i++ {
		weightItem := weights
		if weightIsArray {
			item, ok := weightArray.At(i)
			if !ok {
				return nil, fmt.Errorf("wavg weight row %d out of range", i)
			}
			weightItem = item
		}
		valueItem := values
		if valueIsArray {
			item, ok := valueArray.At(i)
			if !ok {
				return nil, fmt.Errorf("wavg value row %d out of range", i)
			}
			valueItem = item
		}
		if data.IsNull(weightItem) || data.IsNull(valueItem) {
			continue
		}
		w, wok := numeric(weightItem)
		v, vok := numeric(valueItem)
		if !wok || !vok {
			return nil, fmt.Errorf("wavg expects numeric weights and values")
		}
		total += w * v
		denom += w
	}
	if denom == 0 {
		return data.NullValue, nil
	}
	return total / denom, nil
}

func prd(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("prd expects a numeric vector")
	}
	if array.Len() == 0 {
		return data.NullValue, nil
	}
	totalI := int64(1)
	totalF := float64(1)
	hasFloat := false
	seen := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("prd row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		seen = true
		if n, ok := integerValue(item); ok {
			totalI *= n
			totalF *= float64(n)
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF *= float64(n)
		case float64:
			hasFloat = true
			totalF *= n
		default:
			return nil, fmt.Errorf("prd expects a numeric vector")
		}
	}
	if !seen {
		return data.NullValue, nil
	}
	if hasFloat {
		return totalF, nil
	}
	return totalI, nil
}

func sums(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("sums expects a numeric vector")
	}
	if out, handled, err := data.TryTypedNumericSums(array); err != nil || handled {
		recordRuntimeKernelProbe("ArraySums", "vector-scan/sum/"+string(array.Kind()), handled, err)
		if err != nil {
			return nil, err
		}
		return out, nil
	} else {
		recordRuntimeKernelProbe("ArraySums", "vector-scan/sum/"+string(array.Kind()), handled, err)
	}
	out := make([]any, array.Len())
	totalI := int64(0)
	totalF := float64(0)
	hasFloat := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("sums row %d out of range", i)
		}
		if data.IsNull(item) {
			if hasFloat {
				out[i] = totalF
			} else {
				out[i] = totalI
			}
			continue
		}
		if n, ok := integerValue(item); ok {
			totalI += n
			totalF += float64(n)
			if hasFloat {
				out[i] = totalF
			} else {
				out[i] = totalI
			}
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF += float64(n)
			out[i] = totalF
		case float64:
			hasFloat = true
			totalF += n
			out[i] = totalF
		default:
			return nil, fmt.Errorf("sums expects a numeric vector")
		}
	}
	if hasFloat {
		xs := make([]float64, len(out))
		for i, v := range out {
			xs[i], _ = numeric(v)
		}
		return data.NewF64(xs), nil
	}
	xs := make([]int64, len(out))
	for i, v := range out {
		xs[i] = v.(int64)
	}
	return data.NewI64(xs), nil
}

func prds(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("prds expects a numeric vector")
	}
	out := make([]any, array.Len())
	totalI := int64(1)
	totalF := float64(1)
	hasFloat := false
	seen := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("prds row %d out of range", i)
		}
		if data.IsNull(item) {
			if seen {
				if hasFloat {
					out[i] = totalF
				} else {
					out[i] = totalI
				}
			} else {
				out[i] = data.NullValue
			}
			continue
		}
		seen = true
		if n, ok := integerValue(item); ok {
			totalI *= n
			totalF *= float64(n)
			if hasFloat {
				out[i] = totalF
			} else {
				out[i] = totalI
			}
			continue
		}
		switch n := item.(type) {
		case float32:
			hasFloat = true
			totalF *= float64(n)
			out[i] = totalF
		case float64:
			hasFloat = true
			totalF *= n
			out[i] = totalF
		default:
			return nil, fmt.Errorf("prds expects a numeric vector")
		}
	}
	if hasFloat {
		xs := make([]float64, len(out))
		for i, v := range out {
			xs[i], _ = numeric(v)
		}
		return data.NewF64(xs), nil
	}
	return data.InferArray(out), nil
}

func mins(v any) (any, error) {
	return runningExtrema("mins", v, false)
}

func maxs(v any) (any, error) {
	return runningExtrema("maxs", v, true)
}

func runningExtrema(name string, v any, maximum bool) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	if data.IsNull(v) {
		return v, nil
	}
	if _, err := compareOrdered(v, v); err == nil {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("%s expects an ordered vector", name)
	}
	out := make([]any, array.Len())
	var best any
	hasBest := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			if hasBest {
				out[i] = best
			} else {
				out[i] = data.NullValue
			}
			continue
		}
		if !hasBest {
			best = item
			hasBest = true
			out[i] = best
			continue
		}
		cmp, err := compareOrdered(item, best)
		if err != nil {
			return nil, fmt.Errorf("%s expects ordered values: %w", name, err)
		}
		if (!maximum && cmp < 0) || (maximum && cmp > 0) {
			best = item
		}
		out[i] = best
	}
	return inferQArray(out, array.Kind()), nil
}

func avgs(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("avgs expects a numeric vector")
	}
	out := make([]any, array.Len())
	total := float64(0)
	count := 0
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("avgs row %d out of range", i)
		}
		if !data.IsNull(item) {
			n, ok := numeric(item)
			if !ok {
				return nil, fmt.Errorf("avgs expects a numeric vector")
			}
			total += n
			count++
		}
		if count == 0 {
			out[i] = data.NullForKind(data.KindF64)
		} else {
			out[i] = total / float64(count)
		}
	}
	return inferQArray(out, data.KindF64), nil
}

func ratios(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	if data.IsNull(v) {
		return data.NullForKind(data.KindF64), nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("ratios expects a numeric vector")
	}
	out := make([]any, array.Len())
	var previous any
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("ratios row %d out of range", i)
		}
		if data.IsNull(item) {
			out[i] = data.NullValue
			previous = item
			continue
		}
		if _, ok := numeric(item); !ok {
			return nil, fmt.Errorf("ratios expects a numeric vector")
		}
		if i == 0 || data.IsNull(previous) {
			out[i] = item
		} else {
			ratio, err := applyDyadic('%', item, previous)
			if err != nil {
				return nil, err
			}
			out[i] = ratio
		}
		previous = item
	}
	xs := make([]float64, len(out))
	for i, v := range out {
		if data.IsNull(v) {
			return data.InferArray(out), nil
		}
		n, ok := numeric(v)
		if !ok {
			return nil, fmt.Errorf("ratios expects a numeric vector")
		}
		xs[i] = n
	}
	return data.NewF64(xs), nil
}

func negValue(v any) (any, error) {
	if h, ok := v.(*qIPCHandle); ok {
		if h == nil || h.session == nil {
			return nil, fmt.Errorf("q IPC handle is closed")
		}
		return &qIPCHandle{
			target:  h.target,
			async:   true,
			session: h.session,
		}, nil
	}
	return mapNumericUnary("neg", v, func(n float64, isInt bool) any {
		if isInt {
			return int64(-n)
		}
		return -n
	})
}

func absValue(v any) (any, error) {
	return mapNumericUnary("abs", v, func(n float64, isInt bool) any {
		if n < 0 {
			n = -n
		}
		if isInt {
			return int64(n)
		}
		return n
	})
}

func expValue(v any) (any, error) {
	return mapNumericUnary("exp", v, func(n float64, _ bool) any {
		return math.Exp(n)
	})
}

func reciprocalValue(v any) (any, error) {
	return mapNumericUnary("reciprocal", v, func(n float64, _ bool) any {
		return 1 / n
	})
}

func signumValue(v any) (any, error) {
	return mapNumericUnary("signum", v, func(n float64, _ bool) any {
		switch {
		case n < 0:
			return int64(-1)
		case n > 0:
			return int64(1)
		default:
			return int64(0)
		}
	})
}

func floorValue(v any) (any, error) {
	return mapNumericUnary("floor", v, func(n float64, _ bool) any {
		return int64(math.Floor(n))
	})
}

func ceilingValue(v any) (any, error) {
	return mapNumericUnary("ceiling", v, func(n float64, _ bool) any {
		return int64(math.Ceil(n))
	})
}

func mapNumericUnary(name string, v any, fn func(float64, bool) any) (any, error) {
	if data.IsNull(v) {
		return data.NullValue, nil
	}
	if n, ok := numeric(v); ok {
		_, isInt := integerValue(v)
		return fn(n, isInt), nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("%s expects a numeric value or vector", name)
	}
	out := make([]any, array.Len())
	hasFloat := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			out[i] = data.NullValue
			continue
		}
		n, ok := numeric(item)
		if !ok {
			return nil, fmt.Errorf("%s expects a numeric value or vector", name)
		}
		_, isInt := integerValue(item)
		value := fn(n, isInt)
		if _, ok := value.(float64); ok {
			hasFloat = true
		}
		out[i] = value
	}
	if hasFloat {
		for _, value := range out {
			if data.IsNull(value) {
				return data.InferArray(out), nil
			}
		}
		xs := make([]float64, len(out))
		for i, value := range out {
			xs[i], _ = numeric(value)
		}
		return data.NewF64(xs), nil
	}
	return data.InferArray(out), nil
}

func allValue(v any) (any, error) {
	return boolAggregate("all", v, true, func(acc, item bool) bool { return acc && item })
}

func anyValue(v any) (any, error) {
	return boolAggregate("any", v, false, func(acc, item bool) bool { return acc || item })
}

func boolAggregate(name string, v any, initial bool, fn func(bool, bool) bool) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		b, err := boolValue(v)
		if err != nil {
			return nil, fmt.Errorf("%s expects bool or numeric values", name)
		}
		return b, nil
	}
	acc := initial
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("%s row %d out of range", name, i)
		}
		if data.IsNull(item) {
			continue
		}
		b, err := boolValue(item)
		if err != nil {
			return nil, fmt.Errorf("%s expects bool or numeric values", name)
		}
		acc = fn(acc, b)
	}
	return acc, nil
}

func first(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if array.Len() == 0 {
		return data.NullValue, nil
	}
	item, ok := array.At(0)
	if !ok {
		return nil, fmt.Errorf("first row 0 out of range")
	}
	return item, nil
}

func last(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if array.Len() == 0 {
		return data.NullValue, nil
	}
	item, ok := array.At(array.Len() - 1)
	if !ok {
		return nil, fmt.Errorf("last row %d out of range", array.Len()-1)
	}
	return item, nil
}

func keys(v any) (any, error) {
	switch x := v.(type) {
	case qAttributedVector:
		if qAttributeHasIndex(x) {
			return symbolArray([]data.Symbol{"attribute", "value", "index"}), nil
		}
		return symbolArray([]data.Symbol{"attribute", "value"}), nil
	case qEnumVector:
		return symbolArray([]data.Symbol{"domain", "value"}), nil
	case EvalDict:
		return data.InferArray(x.Keys), nil
	case data.KeyedFrame:
		return symbolArray(x.Keys()), nil
	case data.Frame:
		return data.NewSymbols(nil), nil
	default:
		return nil, fmt.Errorf("keys expects a dictionary or keyed table")
	}
}

func value(v any) (any, error) {
	switch x := v.(type) {
	case qAttributedVector:
		return x.vector, nil
	case qEnumVector:
		return x.decodedArray(), nil
	case EvalDict:
		return data.NewAny(x.Values), nil
	case data.KeyedFrame:
		return x.ValueFrame()
	default:
		return v, nil
	}
}

func cols(v any) (any, error) {
	switch x := v.(type) {
	case data.Frame:
		return symbolArray(x.Schema().Names()), nil
	case data.KeyedFrame:
		return symbolArray(x.Frame().Schema().Names()), nil
	case EvalDict:
		keys, err := dictSymbolKeys(x)
		if err != nil {
			return nil, fmt.Errorf("cols expects dictionary symbol keys: %w", err)
		}
		return symbolArray(keys), nil
	default:
		return nil, fmt.Errorf("cols expects a table or dictionary")
	}
}

func xcols(left any, right any) (any, error) {
	names, err := qColumnNameList(left)
	if err != nil {
		return nil, fmt.Errorf("xcols: %w", err)
	}
	switch x := right.(type) {
	case data.Frame:
		return reorderFrameColumns(x, names)
	case data.KeyedFrame:
		frame, err := reorderFrameColumns(x.Frame(), names)
		if err != nil {
			return nil, err
		}
		return data.KeyBy(frame, x.Keys()...)
	case EvalDict:
		return reorderDictColumns(x, names)
	default:
		return nil, fmt.Errorf("xcols expects a table or dictionary")
	}
}

func xgroup(left any, right any) (any, error) {
	names, err := qColumnNameList(left)
	if err != nil {
		return nil, fmt.Errorf("xgroup: %w", err)
	}
	switch x := right.(type) {
	case data.Frame:
		return data.XGroup(x, names...)
	case data.KeyedFrame:
		return data.XGroup(x.Frame(), names...)
	default:
		return nil, fmt.Errorf("xgroup expects a table")
	}
}

func xkey(left any, right any) (any, error) {
	names, err := qColumnNameList(left)
	if err != nil {
		return nil, fmt.Errorf("xkey: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("xkey requires at least one key column")
	}
	switch x := right.(type) {
	case data.Frame:
		return data.KeyBy(x, names...)
	case data.KeyedFrame:
		return data.KeyBy(x.Frame(), names...)
	default:
		return nil, fmt.Errorf("xkey expects a table")
	}
}

func ungroup(v any) (any, error) {
	switch x := v.(type) {
	case data.Frame:
		return data.Ungroup(x)
	case data.KeyedFrame:
		return data.Ungroup(x.Frame())
	default:
		return nil, fmt.Errorf("ungroup expects a table")
	}
}

func xasc(left any, right any) (any, error) {
	return xsort(left, right, false)
}

func xdesc(left any, right any) (any, error) {
	return xsort(left, right, true)
}

func xsort(left any, right any, descending bool) (any, error) {
	names, err := qColumnNameList(left)
	if err != nil {
		return nil, fmt.Errorf("xsort: %w", err)
	}
	switch x := right.(type) {
	case data.Frame:
		return sortFrameByColumns(x, names, descending)
	case data.KeyedFrame:
		frame, err := sortFrameByColumns(x.Frame(), names, descending)
		if err != nil {
			return nil, err
		}
		return data.KeyBy(frame, x.Keys()...)
	default:
		return nil, fmt.Errorf("xsort expects a table")
	}
}

func sortFrameByColumns(frame data.Frame, names []data.Symbol, descending bool) (data.Frame, error) {
	bound := make([]data.Array, len(names))
	for i, name := range names {
		col, ok := frame.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("sort column %q does not exist", name)
		}
		bound[i] = col
	}
	indexes := make([]int, frame.Len())
	for i := range indexes {
		indexes[i] = i
	}
	var cmpErr error
	sort.SliceStable(indexes, func(i, j int) bool {
		leftRow, rightRow := indexes[i], indexes[j]
		for _, col := range bound {
			left, lok := col.At(leftRow)
			right, rok := col.At(rightRow)
			if !lok || !rok {
				cmpErr = fmt.Errorf("sort row out of range")
				return false
			}
			cmp, err := compareSortable(left, right)
			if err != nil {
				cmpErr = err
				return false
			}
			if cmp == 0 {
				continue
			}
			if descending {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
	if cmpErr != nil {
		return data.Frame{}, cmpErr
	}
	return frame.Gather(indexes)
}

func qColumnNameList(v any) ([]data.Symbol, error) {
	switch x := v.(type) {
	case data.Symbol:
		return []data.Symbol{x}, nil
	case string:
		return []data.Symbol{data.Symbol(x)}, nil
	case data.Array:
		values := x.Values()
		out := make([]data.Symbol, len(values))
		for i, value := range values {
			switch item := value.(type) {
			case data.Symbol:
				out[i] = item
			case string:
				out[i] = data.Symbol(item)
			default:
				return nil, fmt.Errorf("column %d is %T, want symbol", i, value)
			}
			if out[i] == "" {
				return nil, fmt.Errorf("column %d is empty", i)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("left operand must be a symbol or symbol vector")
	}
}

func reorderFrameColumns(frame data.Frame, requested []data.Symbol) (data.Frame, error) {
	order, err := xcolsOrder(frame.Schema().Names(), requested)
	if err != nil {
		return data.Frame{}, err
	}
	return data.SelectFrameColumns(frame, order...)
}

func reorderDictColumns(d EvalDict, requested []data.Symbol) (EvalDict, error) {
	keys, err := dictSymbolKeys(d)
	if err != nil {
		return EvalDict{}, fmt.Errorf("dictionary keys must be symbols: %w", err)
	}
	order, err := xcolsOrder(keys, requested)
	if err != nil {
		return EvalDict{}, err
	}
	position := make(map[data.Symbol]int, len(keys))
	for i, key := range keys {
		position[key] = i
	}
	out := EvalDict{
		Keys:   make([]any, 0, len(order)),
		Values: make([]any, 0, len(order)),
	}
	for _, name := range order {
		index := position[name]
		out.Keys = append(out.Keys, name)
		out.Values = append(out.Values, d.Values[index])
	}
	return out, nil
}

func xcolsOrder(existing []data.Symbol, requested []data.Symbol) ([]data.Symbol, error) {
	available := make(map[data.Symbol]struct{}, len(existing))
	for _, name := range existing {
		available[name] = struct{}{}
	}
	seen := make(map[data.Symbol]struct{}, len(existing))
	order := make([]data.Symbol, 0, len(existing))
	for _, name := range requested {
		if name == "" {
			return nil, fmt.Errorf("column name must not be empty")
		}
		if _, ok := available[name]; !ok {
			return nil, fmt.Errorf("column %q does not exist", name)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("column %q is duplicated", name)
		}
		order = append(order, name)
		seen[name] = struct{}{}
	}
	for _, name := range existing {
		if _, ok := seen[name]; ok {
			continue
		}
		order = append(order, name)
	}
	return order, nil
}

func meta(v any) (any, error) {
	if x, ok := v.(qAttributedVector); ok {
		typ, err := typeOf(x.vector)
		if err != nil {
			return nil, err
		}
		indexed := data.Symbol("")
		if qAttributeHasIndex(x) {
			indexed = x.attribute
		}
		return EvalDict{
			Keys:   []any{data.Symbol("attribute"), data.Symbol("type"), data.Symbol("count"), data.Symbol("index")},
			Values: []any{x.attribute, typ, int64(x.Len()), indexed},
		}, nil
	}
	if x, ok := v.(qEnumVector); ok {
		typ, err := typeOf(x.decodedArray())
		if err != nil {
			return nil, err
		}
		return EvalDict{
			Keys:   []any{data.Symbol("domain"), data.Symbol("type"), data.Symbol("count")},
			Values: []any{x.domain, typ, int64(x.Len())},
		}, nil
	}
	var frame data.Frame
	switch x := v.(type) {
	case data.Frame:
		frame = x
	case data.KeyedFrame:
		frame = x.Frame()
	default:
		return nil, fmt.Errorf("meta expects a table")
	}
	names := frame.Schema().Names()
	typeNames := make([]string, len(names))
	attributes := make([]any, len(names))
	for i, name := range names {
		kind, _ := frame.Schema().Kind(name)
		typeNames[i] = string(kind)
		attributes[i] = data.NullValue
		if column, ok := frame.Column(name); ok {
			metadata := data.ArrayMetadataOf(column)
			if len(metadata.Attributes) > 0 {
				attributes[i] = metadata.Attributes[0]
			}
		}
	}
	return data.NewFrame(
		data.Column{Name: "c", Data: symbolArray(names)},
		data.Column{Name: "t", Data: data.NewString(typeNames)},
		data.Column{Name: "a", Data: data.NewAny(attributes)},
	)
}

func attrValue(v any) (any, error) {
	if x, ok := v.(qAttributedVector); ok {
		return x.attribute, nil
	}
	if array, ok := v.(data.Array); ok {
		metadata := data.ArrayMetadataOf(array)
		if len(metadata.Attributes) > 0 {
			return metadata.Attributes[0], nil
		}
		return data.Symbol(""), nil
	}
	return data.Symbol(""), nil
}

func enumCodes(v any) (any, error) {
	var codes []int32
	switch x := v.(type) {
	case qEnumVector:
		codes = x.EncodedCodes()
	case data.Array:
		var ok bool
		codes, ok = data.EncodedCodesOf(x)
		if !ok {
			return nil, fmt.Errorf("codes expects an encoded vector")
		}
	default:
		return nil, fmt.Errorf("codes expects an encoded vector")
	}
	out := make([]int64, len(codes))
	for i, code := range codes {
		out[i] = int64(code)
	}
	return data.NewI64(out), nil
}

func enumDomainValues(v any) (any, error) {
	var domain []any
	switch x := v.(type) {
	case qEnumVector:
		domain = x.EncodedDomain()
	case data.Array:
		var ok bool
		domain, ok = data.EncodedDomainOf(x)
		if !ok {
			return nil, fmt.Errorf("domain expects an encoded vector")
		}
	default:
		return nil, fmt.Errorf("domain expects an encoded vector")
	}
	return data.InferArray(domain), nil
}

func group(v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		if index, ok := data.ArrayIndexFor(array, data.ArrayAttributeGrouped); ok {
			return qGroupFromArrayIndex(index), nil
		}
		if index, ok := data.ArrayIndexFor(array, data.ArrayAttributeUnique); ok {
			return qGroupFromArrayIndex(index), nil
		}
	}
	values, err := setItems(v)
	if err != nil {
		return nil, err
	}
	keys := make([]any, 0)
	indexes := make([][]int64, 0)
	for i, value := range values {
		found := -1
		for j, key := range keys {
			if equalValue(key, value) {
				found = j
				break
			}
		}
		if found < 0 {
			keys = append(keys, value)
			indexes = append(indexes, []int64{int64(i)})
			continue
		}
		indexes[found] = append(indexes[found], int64(i))
	}
	out := make([]any, len(indexes))
	for i, rows := range indexes {
		out[i] = data.NewI64(rows)
	}
	return EvalDict{Keys: keys, Values: out}, nil
}

func qAttributeHasIndex(v qAttributedVector) bool {
	_, ok := data.ArrayIndexFor(v.vector, v.attribute)
	return ok
}

func qGroupFromArrayIndex(index data.ArrayIndex) EvalDict {
	out := make([]any, len(index.Rows))
	for i, rows := range index.Rows {
		ints := make([]int64, len(rows))
		for j, row := range rows {
			ints[j] = int64(row)
		}
		out[i] = data.NewI64(ints)
	}
	return EvalDict{Keys: append([]any(nil), index.Keys...), Values: out}
}

func symbolArray(symbols []data.Symbol) data.Array {
	names := make([]string, len(symbols))
	for i, sym := range symbols {
		names[i] = string(sym)
	}
	return data.NewSymbols(names)
}

func minValue(v any) (any, error) {
	return extrema(v, false)
}

func maxValue(v any) (any, error) {
	return extrema(v, true)
}

func extrema(v any, wantMax bool) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	if value, handled, has, err := data.TryTypedMinMax(array, wantMax); err != nil || handled {
		kernel := "ArrayMin"
		if wantMax {
			kernel = "ArrayMax"
		}
		recordRuntimeKernelProbe(kernel, "vector-reduce/extrema/"+string(array.Kind()), handled, err)
		if err != nil {
			return nil, err
		}
		if !has {
			return data.NullValue, nil
		}
		return value, nil
	}
	var best any
	hasBest := false
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("extrema row %d out of range", i)
		}
		if data.IsNull(item) {
			continue
		}
		if !hasBest {
			best = item
			hasBest = true
			continue
		}
		cmp, err := compareOrdered(item, best)
		if err != nil {
			return nil, err
		}
		if (wantMax && cmp > 0) || (!wantMax && cmp < 0) {
			best = item
		}
	}
	if !hasBest {
		return data.NullValue, nil
	}
	return best, nil
}

func where(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		if truth, ok := v.(bool); ok {
			if truth {
				return data.NewI64([]int64{0}), nil
			}
			return data.NewI64(nil), nil
		}
		n, ok := integerValue(v)
		if !ok {
			return nil, fmt.Errorf("where expects a bool or integer vector")
		}
		if n < 0 {
			return nil, fmt.Errorf("where expects non-negative integer counts")
		}
		return data.NewI64(make([]int64, n)), nil
	}
	if array.Kind() == data.KindBool {
		typedOut, handled, err := data.TryTypedWhereMaskI64(array)
		recordRuntimeKernelProbe("ArrayWhere", "mask-to-index/i64", handled, err)
		if err != nil {
			return nil, err
		}
		if handled {
			return typedOut, nil
		}
		indexes, err := data.WhereMask(array)
		if err != nil {
			return nil, err
		}
		out := make([]int64, len(indexes))
		for i, index := range indexes {
			out[i] = int64(index)
		}
		return data.NewI64(out), nil
	}
	var out []int64
	for i, value := range array.Values() {
		if data.IsNull(value) {
			continue
		}
		n, ok := integerValue(value)
		if !ok {
			return nil, fmt.Errorf("where expects a bool or integer vector")
		}
		if n < 0 {
			return nil, fmt.Errorf("where expects non-negative integer counts")
		}
		for j := int64(0); j < n; j++ {
			out = append(out, int64(i))
		}
	}
	return data.NewI64(out), nil
}

func (s *EvalState) evalWhereCompare(src string) (any, bool, error) {
	leftExpr, rightExpr, op, ok := splitWhereCompareExpr(src)
	if !ok {
		return nil, false, nil
	}
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, true, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, true, err
	}
	array, scalar, dataOp, ok := qWhereCompareOperands(left, right, op)
	if !ok {
		return nil, false, nil
	}
	shape := "compare-to-index/" + op + "/" + string(array.Kind()) + "/" + string(qRuntimeKernelOperandKind(scalar, nil))
	out, handled, err := data.TryTypedCompareIndexesI64(array, dataOp, scalar)
	recordRuntimeKernelProbe("ArrayWhereCompare", shape, handled, err)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	return out, true, nil
}

func splitWhereCompareExpr(src string) (string, string, string, bool) {
	for _, op := range []string{"<>", "<=", ">=", "=", "<", ">"} {
		if left, right, ok := splitTopLevelOperator(src, op); ok {
			return left, right, op, true
		}
	}
	return "", "", "", false
}

func qWhereCompareOperands(left, right any, op string) (data.Array, any, data.Op, bool) {
	if array, ok := left.(data.Array); ok {
		if _, rightIsArray := right.(data.Array); rightIsArray {
			return nil, nil, "", false
		}
		dataOp, ok := qDataCompareOpString(op)
		return array, right, dataOp, ok
	}
	array, ok := right.(data.Array)
	if !ok {
		return nil, nil, "", false
	}
	dataOp, ok := qDataCompareOpString(qReverseCompareOpString(op))
	return array, left, dataOp, ok
}

func qDataCompareOpString(op string) (data.Op, bool) {
	switch op {
	case "=":
		return data.OpEQ, true
	case "<>":
		return data.OpNE, true
	case "<":
		return data.OpLT, true
	case "<=":
		return data.OpLE, true
	case ">":
		return data.OpGT, true
	case ">=":
		return data.OpGE, true
	default:
		return "", false
	}
}

func qReverseCompareOpString(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	default:
		return op
	}
}

func notValue(v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		out := make([]bool, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, fmt.Errorf("not row %d out of range", i)
			}
			truth, err := boolValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = !truth
		}
		return data.NewBool(out), nil
	}
	truth, err := boolValue(v)
	if err != nil {
		return nil, err
	}
	return !truth, nil
}

func nullValue(v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		out := make([]bool, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, fmt.Errorf("null row %d out of range", i)
			}
			out[i] = data.IsNull(item)
		}
		return data.NewBool(out), nil
	}
	return data.IsNull(v), nil
}

func logicalAnd(left, right any) (any, error) {
	return applyLogical(left, right, func(a, b bool) bool { return a && b })
}

func logicalOr(left, right any) (any, error) {
	return applyLogical(left, right, func(a, b bool) bool { return a || b })
}

func applyLogical(left, right any, fn func(bool, bool) bool) (any, error) {
	la, lok := left.(data.Array)
	ra, rok := right.(data.Array)
	if !lok && !rok {
		lv, err := boolValue(left)
		if err != nil {
			return nil, err
		}
		rv, err := boolValue(right)
		if err != nil {
			return nil, err
		}
		return fn(lv, rv), nil
	}
	n := 0
	switch {
	case lok && rok:
		if la.Len() != ra.Len() && la.Len() != 1 && ra.Len() != 1 {
			return nil, fmt.Errorf("logical length mismatch")
		}
		n = la.Len()
		if ra.Len() > n {
			n = ra.Len()
		}
	case lok:
		n = la.Len()
	case rok:
		n = ra.Len()
	}
	out := make([]bool, n)
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if lok {
			var ok bool
			lv, ok = la.At(vectorIndex(i, la.Len()))
			if !ok {
				return nil, fmt.Errorf("logical left row %d out of range", i)
			}
		}
		if rok {
			var ok bool
			rv, ok = ra.At(vectorIndex(i, ra.Len()))
			if !ok {
				return nil, fmt.Errorf("logical right row %d out of range", i)
			}
		}
		lt, err := boolValue(lv)
		if err != nil {
			return nil, err
		}
		rt, err := boolValue(rv)
		if err != nil {
			return nil, err
		}
		out[i] = fn(lt, rt)
	}
	return data.NewBool(out), nil
}

func boolValue(v any) (bool, error) {
	if data.IsNull(v) {
		return false, nil
	}
	switch x := v.(type) {
	case bool:
		return x, nil
	case int:
		return x != 0, nil
	case int8:
		return x != 0, nil
	case int16:
		return x != 0, nil
	case int32:
		return x != 0, nil
	case int64:
		return x != 0, nil
	case uint:
		return x != 0, nil
	case uint8:
		return x != 0, nil
	case uint16:
		return x != 0, nil
	case uint32:
		return x != 0, nil
	case uint64:
		return x != 0, nil
	case float32:
		return x != 0, nil
	case float64:
		return x != 0, nil
	default:
		return false, fmt.Errorf("boolean operation expects bool or numeric values")
	}
}

func findValue(left, right any) (any, error) {
	domain, err := findDomainValues(left)
	if err != nil {
		return nil, err
	}
	queries, scalar, err := findQueryValues(right)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(queries))
	for i, query := range queries {
		out[i] = int64(len(domain))
		for j, candidate := range domain {
			if equalValue(candidate, query) {
				out[i] = int64(j)
				break
			}
		}
	}
	if scalar {
		return out[0], nil
	}
	return data.NewI64(out), nil
}

func findDomainValues(v any) ([]any, error) {
	switch x := v.(type) {
	case data.Array:
		return x.Values(), nil
	case string:
		runes := []rune(x)
		out := make([]any, len(runes))
		for i, r := range runes {
			out[i] = string(r)
		}
		return out, nil
	default:
		return []any{v}, nil
	}
}

func findQueryValues(v any) ([]any, bool, error) {
	switch x := v.(type) {
	case data.Array:
		return x.Values(), false, nil
	case string:
		runes := []rune(x)
		if len(runes) == 1 {
			return []any{x}, true, nil
		}
		out := make([]any, len(runes))
		for i, r := range runes {
			out[i] = string(r)
		}
		return out, false, nil
	default:
		return []any{v}, true, nil
	}
}

func membership(left, right any) (any, error) {
	if leftArray, ok := left.(data.Array); ok {
		out := make([]bool, leftArray.Len())
		for i := 0; i < leftArray.Len(); i++ {
			value, ok := leftArray.At(i)
			if !ok {
				return nil, fmt.Errorf("in left row %d out of range", i)
			}
			out[i] = containsValue(right, value)
		}
		return data.NewBool(out), nil
	}
	return containsValue(right, left), nil
}

func bin(left, right any) (any, error) {
	if domainArray, ok := left.(data.Array); ok {
		result, ok, err := data.Bin(domainArray, right)
		if err != nil {
			return nil, err
		}
		if ok {
			return result, nil
		}
	}
	domain, err := vectorValues(left)
	if err != nil {
		return nil, fmt.Errorf("bin expects a sorted vector domain")
	}
	if rightArray, ok := right.(data.Array); ok {
		out := make([]int64, rightArray.Len())
		for i := 0; i < rightArray.Len(); i++ {
			value, ok := rightArray.At(i)
			if !ok {
				return nil, fmt.Errorf("bin query row %d out of range", i)
			}
			out[i], err = binScalar(domain, value)
			if err != nil {
				return nil, err
			}
		}
		return data.NewI64(out), nil
	}
	return binScalar(domain, right)
}

func binScalar(domain []any, query any) (int64, error) {
	if len(domain) == 0 || data.IsNull(query) {
		return -1, nil
	}
	lo, hi := 0, len(domain)
	for lo < hi {
		mid := lo + (hi-lo)/2
		cmp, err := compareOrdered(domain[mid], query)
		if err != nil {
			return 0, fmt.Errorf("bin compare: %w", err)
		}
		if cmp <= 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return int64(lo - 1), nil
}

func binr(left, right any) (any, error) {
	domain, err := vectorValues(left)
	if err != nil {
		return nil, fmt.Errorf("binr expects a sorted vector domain")
	}
	if rightArray, ok := right.(data.Array); ok {
		out := make([]int64, rightArray.Len())
		for i := 0; i < rightArray.Len(); i++ {
			value, ok := rightArray.At(i)
			if !ok {
				return nil, fmt.Errorf("binr query row %d out of range", i)
			}
			out[i], err = binrScalar(domain, value)
			if err != nil {
				return nil, err
			}
		}
		return data.NewI64(out), nil
	}
	return binrScalar(domain, right)
}

func binrScalar(domain []any, query any) (int64, error) {
	if len(domain) == 0 || data.IsNull(query) {
		return 0, nil
	}
	lo, hi := 0, len(domain)
	for lo < hi {
		mid := lo + (hi-lo)/2
		cmp, err := compareOrdered(domain[mid], query)
		if err != nil {
			return 0, fmt.Errorf("binr compare: %w", err)
		}
		if cmp < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return int64(lo), nil
}

func within(left, right any) (any, error) {
	bounds, err := vectorValues(right)
	if err != nil || len(bounds) != 2 {
		return nil, fmt.Errorf("within expects a two-item bounds vector")
	}
	if leftArray, ok := left.(data.Array); ok {
		out := make([]bool, leftArray.Len())
		for i := 0; i < leftArray.Len(); i++ {
			value, ok := leftArray.At(i)
			if !ok {
				return nil, fmt.Errorf("within left row %d out of range", i)
			}
			ok, err := scalarWithin(value, bounds[0], bounds[1])
			if err != nil {
				return nil, err
			}
			out[i] = ok
		}
		return data.NewBool(out), nil
	}
	return scalarWithin(left, bounds[0], bounds[1])
}

func scalarWithin(value, lo, hi any) (bool, error) {
	if data.IsNull(value) || data.IsNull(lo) || data.IsNull(hi) {
		return false, nil
	}
	loCmp, err := compareOrdered(value, lo)
	if err != nil {
		return false, fmt.Errorf("within lower bound: %w", err)
	}
	hiCmp, err := compareOrdered(value, hi)
	if err != nil {
		return false, fmt.Errorf("within upper bound: %w", err)
	}
	return loCmp >= 0 && hiCmp <= 0, nil
}

func except(left, right any) (any, error) {
	leftArray, ok := left.(data.Array)
	if !ok {
		if containsValue(right, left) {
			return data.NewAny(nil), nil
		}
		return data.NewAny([]any{left}), nil
	}
	indexes := make([]int, 0, leftArray.Len())
	for i := 0; i < leftArray.Len(); i++ {
		value, ok := leftArray.At(i)
		if !ok {
			return nil, fmt.Errorf("except left row %d out of range", i)
		}
		if !containsValue(right, value) {
			indexes = append(indexes, i)
		}
	}
	return data.Gather(leftArray, indexes)
}

func inter(left, right any) (any, error) {
	leftArray, ok := left.(data.Array)
	if !ok {
		if containsValue(right, left) {
			return data.NewAny([]any{left}), nil
		}
		return data.NewAny(nil), nil
	}
	indexes := make([]int, 0, leftArray.Len())
	var seen []any
	for i := 0; i < leftArray.Len(); i++ {
		value, ok := leftArray.At(i)
		if !ok {
			return nil, fmt.Errorf("inter left row %d out of range", i)
		}
		if containsRawValue(seen, value) || !containsValue(right, value) {
			continue
		}
		seen = append(seen, value)
		indexes = append(indexes, i)
	}
	return data.Gather(leftArray, indexes)
}

func union(left, right any) (any, error) {
	leftItems, err := setItems(left)
	if err != nil {
		return nil, err
	}
	rightItems, err := setItems(right)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(leftItems)+len(rightItems))
	for _, value := range leftItems {
		if containsRawValue(out, value) {
			continue
		}
		out = append(out, value)
	}
	for _, value := range rightItems {
		if containsRawValue(out, value) {
			continue
		}
		out = append(out, value)
	}
	return data.InferArray(out), nil
}

func setItems(v any) ([]any, error) {
	if array, ok := v.(data.Array); ok {
		return array.Values(), nil
	}
	return []any{v}, nil
}

func containsValue(collection any, needle any) bool {
	items, err := setItems(collection)
	if err != nil {
		return false
	}
	return containsRawValue(items, needle)
}

func containsRawValue(items []any, needle any) bool {
	for _, item := range items {
		if equalValue(item, needle) {
			return true
		}
	}
	return false
}

func asc(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	indexes, err := sortedIndexes(array, false)
	if err != nil {
		return nil, err
	}
	return array.Gather(indexes), nil
}

func desc(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	indexes, err := sortedIndexes(array, true)
	if err != nil {
		return nil, err
	}
	return array.Gather(indexes), nil
}

func iasc(v any) (any, error) {
	return sortIndex(v, false)
}

func idesc(v any) (any, error) {
	return sortIndex(v, true)
}

func rank(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return data.NewI64([]int64{0}), nil
	}
	indexes, err := sortedIndexes(array, false)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(indexes))
	for sortedPosition, originalIndex := range indexes {
		out[originalIndex] = int64(sortedPosition)
	}
	return data.NewI64(out), nil
}

func sortIndex(v any, descending bool) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return data.NewI64([]int64{0}), nil
	}
	indexes, err := sortedIndexes(array, descending)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(indexes))
	for i, index := range indexes {
		out[i] = int64(index)
	}
	return data.NewI64(out), nil
}

func sortedIndexes(array data.Array, descending bool) ([]int, error) {
	indexes := make([]int, array.Len())
	for i := range indexes {
		indexes[i] = i
	}
	values := array.Values()
	var cmpErr error
	sort.SliceStable(indexes, func(i, j int) bool {
		cmp, err := compareSortable(values[indexes[i]], values[indexes[j]])
		if err != nil {
			cmpErr = err
			return false
		}
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})
	if cmpErr != nil {
		return nil, cmpErr
	}
	return indexes, nil
}

func compareSortable(left, right any) (int, error) {
	leftNull := data.IsNull(left)
	rightNull := data.IsNull(right)
	switch {
	case leftNull && rightNull:
		return 0, nil
	case leftNull:
		return -1, nil
	case rightNull:
		return 1, nil
	}
	cmp, err := compareOrdered(left, right)
	if err != nil {
		return 0, fmt.Errorf("sort expects comparable values: %w", err)
	}
	return cmp, nil
}

func distinct(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	values := array.Values()
	indexes := make([]int, 0, len(values))
	for i, value := range values {
		seen := false
		for _, index := range indexes {
			if equalValue(values[index], value) {
				seen = true
				break
			}
		}
		if !seen {
			indexes = append(indexes, i)
		}
	}
	return array.Gather(indexes), nil
}

func reverse(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		if s, ok := v.(string); ok {
			runes := []rune(s)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return string(runes), nil
		}
		return v, nil
	}
	indexes := make([]int, array.Len())
	for i := range indexes {
		indexes[i] = array.Len() - 1 - i
	}
	return array.Gather(indexes), nil
}

func prev(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return data.NullValue, nil
	}
	values := array.Values()
	out := make([]any, len(values))
	if len(out) == 0 {
		return data.InferArray(out), nil
	}
	out[0] = data.NullValue
	copy(out[1:], values[:len(values)-1])
	return inferQArray(out, array.Kind()), nil
}

func nextValue(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return data.NullValue, nil
	}
	values := array.Values()
	out := make([]any, len(values))
	if len(out) == 0 {
		return data.InferArray(out), nil
	}
	copy(out, values[1:])
	out[len(out)-1] = data.NullValue
	return inferQArray(out, array.Kind()), nil
}

func xprev(width any, v any) (any, error) {
	n64, ok := integerValue(width)
	if !ok || int64(int(n64)) != n64 {
		return nil, fmt.Errorf("xprev expects an integer count")
	}
	array, ok := v.(data.Array)
	if !ok {
		if n64 == 0 {
			return v, nil
		}
		return data.NullValue, nil
	}
	n := int(n64)
	values := array.Values()
	out := make([]any, len(values))
	for i := range out {
		src := i - n
		if src < 0 || src >= len(values) {
			out[i] = data.NullValue
			continue
		}
		out[i] = values[src]
	}
	return inferQArray(out, array.Kind()), nil
}

func differ(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return true, nil
	}
	out := make([]bool, array.Len())
	for i := 0; i < array.Len(); i++ {
		if i == 0 {
			out[i] = true
			continue
		}
		current, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("differ row %d out of range", i)
		}
		previous, ok := array.At(i - 1)
		if !ok {
			return nil, fmt.Errorf("differ row %d out of range", i-1)
		}
		out[i] = !equalValue(current, previous)
	}
	return data.NewBool(out), nil
}

func deltas(v any) (any, error) {
	if _, ok := numeric(v); ok {
		return v, nil
	}
	array, ok := v.(data.Array)
	if !ok {
		return nil, fmt.Errorf("deltas expects a numeric vector")
	}
	if out, handled, err := data.TryTypedDeltas(array); err != nil || handled {
		recordRuntimeKernelProbe("ArrayDeltas", "vector-scan/deltas/"+string(array.Kind()), handled, err)
		return out, err
	}
	out := make([]any, array.Len())
	var previous any
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("deltas row %d out of range", i)
		}
		if !data.IsNull(item) {
			if _, ok := numeric(item); !ok {
				return nil, fmt.Errorf("deltas expects a numeric vector")
			}
		}
		if !data.IsNull(previous) {
			if _, ok := numeric(previous); !ok {
				return nil, fmt.Errorf("deltas expects a numeric vector")
			}
		}
		if !data.IsNull(item) && i == 0 {
			out[i] = item
		} else if i == 0 {
			out[i] = data.NullValue
		} else {
			delta, err := applyDyadic('-', item, previous)
			if err != nil {
				return nil, err
			}
			out[i] = delta
		}
		previous = item
	}
	return inferQArray(out, array.Kind()), nil
}

func fills(v any) (any, error) {
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	values := array.Values()
	out := make([]any, len(values))
	var fill any
	hasFill := false
	for i, value := range values {
		if data.IsNull(value) {
			if hasFill {
				out[i] = fill
			} else {
				out[i] = data.NullValue
			}
			continue
		}
		out[i] = value
		fill = value
		hasFill = true
	}
	if kind := array.Kind(); kind != "" && kind != data.KindNull && kind != data.KindAny {
		if column, err := data.NewColumnWithKind("_", kind, out); err == nil {
			return column.Data, nil
		}
	}
	return inferQArray(out, array.Kind()), nil
}

func raze(v any) (any, error) {
	items, err := vectorValues(v)
	if err != nil {
		return v, nil
	}
	var out []any
	for _, item := range items {
		if array, ok := item.(data.Array); ok {
			out = append(out, array.Values()...)
			continue
		}
		out = append(out, item)
	}
	return data.InferArray(out), nil
}

func take(n int, v any) (any, error) {
	switch x := v.(type) {
	case data.Array:
		indexes := qTakeIndexes(x.Len(), n)
		if len(indexes) == 0 && n != 0 && x.Len() == 0 {
			return x.Gather(nil), nil
		}
		return data.Gather(x, indexes)
	case data.Frame:
		indexes := qTakeIndexes(x.Len(), n)
		if len(indexes) == 0 && n != 0 && x.Len() == 0 {
			return x.Gather(nil)
		}
		return data.GatherFrame(x, indexes)
	case data.KeyedFrame:
		return takeKeyedFrame(x, n)
	case string:
		return takeString(n, x), nil
	default:
		if n == 0 {
			return data.NewAny(nil), nil
		}
		if n < 0 {
			n = -n
		}
		out := make([]any, n)
		for i := range out {
			out[i] = v
		}
		return data.NewAny(out), nil
	}
}

func xbar(width any, v any) (any, error) {
	if array, ok := v.(data.Array); ok {
		width = normalizeXbarIntervalForKind(width, array.Kind())
		return data.BucketFloor(array, width)
	}
	column := data.NewColumn("_", []any{v}).Data
	width = normalizeXbarIntervalForKind(width, column.Kind())
	bucketed, err := data.BucketFloor(column, width)
	if err != nil {
		return nil, err
	}
	out, ok := bucketed.At(0)
	if !ok {
		return nil, fmt.Errorf("xbar result row 0 out of range")
	}
	return out, nil
}

func xrank(bucketCount any, v any) (any, error) {
	n64, ok := integerValue(bucketCount)
	if !ok || int64(int(n64)) != n64 {
		return nil, fmt.Errorf("xrank expects an integer bucket count")
	}
	if n64 <= 0 {
		return nil, fmt.Errorf("xrank expects a positive bucket count")
	}
	buckets := int(n64)
	array, ok := v.(data.Array)
	if !ok {
		if data.IsNull(v) {
			return data.NullValue, nil
		}
		return int64(0), nil
	}
	values := array.Values()
	out := make([]any, len(values))
	indexes := make([]int, 0, len(values))
	for i, value := range values {
		if data.IsNull(value) {
			out[i] = data.NullValue
			continue
		}
		indexes = append(indexes, i)
	}
	if len(indexes) == 0 {
		return inferQArray(out, data.KindI64), nil
	}
	var cmpErr error
	sort.SliceStable(indexes, func(i, j int) bool {
		cmp, err := compareSortable(values[indexes[i]], values[indexes[j]])
		if err != nil {
			cmpErr = err
			return false
		}
		return cmp < 0
	})
	if cmpErr != nil {
		return nil, fmt.Errorf("xrank expects ordered values: %w", cmpErr)
	}
	rank := 0
	for rank < len(indexes) {
		next := rank + 1
		for next < len(indexes) {
			cmp, err := compareSortable(values[indexes[rank]], values[indexes[next]])
			if err != nil {
				return nil, fmt.Errorf("xrank expects ordered values: %w", err)
			}
			if cmp != 0 {
				break
			}
			next++
		}
		bucket := int64((rank * buckets) / len(indexes))
		if bucket >= int64(buckets) {
			bucket = int64(buckets - 1)
		}
		for _, index := range indexes[rank:next] {
			out[index] = bucket
		}
		rank = next
	}
	return inferQArray(out, data.KindI64), nil
}

func normalizeXbarInterval(width any) any {
	return normalizeXbarIntervalForKind(width, "")
}

func normalizeXbarIntervalForKind(width any, kind data.Kind) any {
	if normalized, ok := normalizeTemporalXbarIntervalForKind(width, kind); ok {
		return normalized
	}
	switch x := width.(type) {
	case data.Timespan:
		return x.Nanos()
	case data.Time:
		return x.Nanos()
	case data.Timestamp:
		return x.UnixNanos()
	case data.DateTime:
		return x.UnixNanos()
	case data.Second:
		return x.Seconds()
	case data.Minute:
		return x.Minutes()
	case data.Date:
		return x.Days()
	case data.Month:
		return x.Months()
	default:
		return width
	}
}

func normalizeTemporalXbarIntervalForKind(width any, kind data.Kind) (any, bool) {
	if kind == "" {
		return nil, false
	}
	nanos, nanosOK := xbarIntervalNanos(width)
	switch kind {
	case data.KindDateTime, data.KindTimestamp, data.KindTime, data.KindTimespan:
		if nanosOK {
			return nanos, true
		}
	case data.KindSecond:
		if nanosOK && nanos%1_000_000_000 == 0 {
			return nanos / 1_000_000_000, true
		}
	case data.KindMinute:
		if nanosOK && nanos%(60*1_000_000_000) == 0 {
			return nanos / (60 * 1_000_000_000), true
		}
	case data.KindDate:
		if nanosOK && nanos%(24*60*60*1_000_000_000) == 0 {
			return nanos / (24 * 60 * 60 * 1_000_000_000), true
		}
	case data.KindMonth:
		if x, ok := width.(data.Month); ok {
			return x.Months(), true
		}
	}
	return nil, false
}

func xbarIntervalNanos(width any) (int64, bool) {
	switch x := width.(type) {
	case data.Timespan:
		return x.Nanos(), true
	case data.Time:
		return x.Nanos(), true
	case data.Timestamp:
		return x.UnixNanos(), true
	case data.DateTime:
		return x.UnixNanos(), true
	case data.Second:
		return x.Seconds() * 1_000_000_000, true
	case data.Minute:
		return x.Minutes() * 60 * 1_000_000_000, true
	case data.Date:
		return x.Days() * 24 * 60 * 60 * 1_000_000_000, true
	default:
		return 0, false
	}
}

func msum(width any, v any) (any, error) {
	return movingNumericWindow("msum", width, v, false)
}

func mavg(width any, v any) (any, error) {
	return movingNumericWindow("mavg", width, v, true)
}

func mcount(width any, v any) (any, error) {
	n, ok := integerValue(width)
	if !ok || n <= 0 || int64(int(n)) != n {
		return nil, fmt.Errorf("mcount width must be a positive integer")
	}
	array, ok := v.(data.Array)
	if !ok {
		if data.IsNull(v) {
			return int64(0), nil
		}
		return int64(1), nil
	}
	out := make([]int64, array.Len())
	for i := 0; i < array.Len(); i++ {
		start := i - int(n) + 1
		if start < 0 {
			start = 0
		}
		count := int64(0)
		for row := start; row <= i; row++ {
			item, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("mcount row %d out of range", row)
			}
			if !data.IsNull(item) {
				count++
			}
		}
		out[i] = count
	}
	return data.NewI64(out), nil
}

func mmin(width any, v any) (any, error) {
	return movingExtremaWindow("mmin", width, v, false)
}

func mmax(width any, v any) (any, error) {
	return movingExtremaWindow("mmax", width, v, true)
}

func movingExtremaWindow(name string, width any, v any, wantMax bool) (any, error) {
	n, ok := integerValue(width)
	if !ok || n <= 0 || int64(int(n)) != n {
		return nil, fmt.Errorf("%s width must be a positive integer", name)
	}
	array, ok := v.(data.Array)
	if !ok {
		return v, nil
	}
	out := make([]any, array.Len())
	for i := 0; i < array.Len(); i++ {
		start := i - int(n) + 1
		if start < 0 {
			start = 0
		}
		var best any
		hasBest := false
		for row := start; row <= i; row++ {
			item, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("%s row %d out of range", name, row)
			}
			if data.IsNull(item) {
				continue
			}
			if !hasBest {
				best = item
				hasBest = true
				continue
			}
			cmp, err := compareOrdered(item, best)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if (wantMax && cmp > 0) || (!wantMax && cmp < 0) {
				best = item
			}
		}
		if hasBest {
			out[i] = best
		} else {
			out[i] = data.NullValue
		}
	}
	return inferQArray(out, array.Kind()), nil
}

func movingNumericWindow(name string, width any, v any, average bool) (any, error) {
	n, ok := integerValue(width)
	if !ok || n <= 0 || int64(int(n)) != n {
		return nil, fmt.Errorf("%s width must be a positive integer", name)
	}
	array, ok := v.(data.Array)
	if !ok {
		if data.IsNull(v) {
			return data.NullValue, nil
		}
		if _, ok := numeric(v); !ok {
			return nil, fmt.Errorf("%s expects a numeric vector", name)
		}
		return v, nil
	}
	out := make([]any, array.Len())
	hasFloat := average
	for i := 0; i < array.Len(); i++ {
		start := i - int(n) + 1
		if start < 0 {
			start = 0
		}
		totalI := int64(0)
		totalF := float64(0)
		count := 0
		localFloat := false
		for row := start; row <= i; row++ {
			item, ok := array.At(row)
			if !ok {
				return nil, fmt.Errorf("%s row %d out of range", name, row)
			}
			if data.IsNull(item) {
				continue
			}
			if integer, ok := integerValue(item); ok {
				totalI += integer
				totalF += float64(integer)
				count++
				continue
			}
			numeric, ok := numeric(item)
			if !ok {
				return nil, fmt.Errorf("%s expects a numeric vector", name)
			}
			localFloat = true
			hasFloat = true
			totalF += numeric
			count++
		}
		if count == 0 {
			out[i] = data.NullValue
			continue
		}
		if average {
			out[i] = totalF / float64(count)
		} else if localFloat {
			out[i] = totalF
		} else {
			out[i] = totalI
		}
	}
	if hasFloat {
		for _, value := range out {
			if data.IsNull(value) {
				return data.InferArray(out), nil
			}
		}
		xs := make([]float64, len(out))
		for i, value := range out {
			xs[i], _ = numeric(value)
		}
		return data.NewF64(xs), nil
	}
	return data.InferArray(out), nil
}

func qTakeIndexes(length, n int) []int {
	if n == 0 || length == 0 {
		return nil
	}
	count := n
	if count < 0 {
		count = -count
	}
	indexes := make([]int, count)
	start := 0
	if n < 0 {
		start = length - count%length
		if start == length {
			start = 0
		}
	}
	for i := range indexes {
		indexes[i] = (start + i) % length
	}
	return indexes
}

func takeString(n int, v string) string {
	runes := []rune(v)
	indexes := qTakeIndexes(len(runes), n)
	out := make([]rune, len(indexes))
	for i, index := range indexes {
		out[i] = runes[index]
	}
	return string(out)
}

func drop(n int, v any) (any, error) {
	switch x := v.(type) {
	case data.Array:
		return dropArray(x, n)
	case data.Frame:
		return dropFrame(x, n)
	case data.KeyedFrame:
		return dropKeyedFrame(x, n)
	case string:
		return dropString(x, n), nil
	default:
		if n == 0 {
			return v, nil
		}
		return data.NewAny(nil), nil
	}
}

func rotateValue(left any, right any) (any, error) {
	n64, ok := left.(int64)
	if !ok || int64(int(n64)) != n64 {
		return nil, fmt.Errorf("rotate expects an integer count")
	}
	n := int(n64)
	switch x := right.(type) {
	case data.Array:
		if rotated, handled, err := data.TryTypedRotate(x, n); err != nil || handled {
			if err != nil {
				return nil, err
			}
			return rotated, nil
		}
		return data.Gather(x, rotateIndexes(x.Len(), n))
	case data.Frame:
		return data.GatherFrame(x, rotateIndexes(x.Len(), n))
	case string:
		runes := []rune(x)
		indexes := rotateIndexes(len(runes), n)
		out := make([]rune, len(indexes))
		for i, index := range indexes {
			out[i] = runes[index]
		}
		return string(out), nil
	default:
		return right, nil
	}
}

func rotateIndexes(length, n int) []int {
	indexes := make([]int, length)
	if length == 0 {
		return indexes
	}
	n %= length
	if n < 0 {
		n += length
	}
	for i := range indexes {
		indexes[i] = (i + n) % length
	}
	return indexes
}

func cutOrDrop(left any, right any) (any, error) {
	switch x := left.(type) {
	case int64:
		return drop(int(x), right)
	case data.Array:
		if x.Kind() != data.KindI64 {
			return nil, fmt.Errorf("_ left vector must contain integer cut indexes")
		}
		indexes := make([]int, x.Len())
		for i, value := range x.Values() {
			n, ok := value.(int64)
			if !ok || n < 0 || int64(int(n)) != n {
				return nil, fmt.Errorf("_ cut indexes must be non-negative integers")
			}
			indexes[i] = int(n)
		}
		return cut(indexes, right)
	default:
		return nil, fmt.Errorf("_ left operand must be an integer or integer vector")
	}
}

func cut(indexes []int, v any) (any, error) {
	switch x := v.(type) {
	case data.Array:
		segments := make([]any, len(indexes))
		for i, start := range indexes {
			end := x.Len()
			if i+1 < len(indexes) {
				end = indexes[i+1]
			}
			gather := segmentIndexes(x.Len(), start, end)
			part, err := data.Gather(x, gather)
			if err != nil {
				return nil, err
			}
			segments[i] = part
		}
		return data.NewAny(segments), nil
	case string:
		runes := []rune(x)
		segments := make([]any, len(indexes))
		for i, start := range indexes {
			end := len(runes)
			if i+1 < len(indexes) {
				end = indexes[i+1]
			}
			gather := segmentIndexes(len(runes), start, end)
			out := make([]rune, len(gather))
			for j, index := range gather {
				out[j] = runes[index]
			}
			segments[i] = string(out)
		}
		return data.NewAny(segments), nil
	case data.Frame:
		segments := make([]any, len(indexes))
		for i, start := range indexes {
			end := x.Len()
			if i+1 < len(indexes) {
				end = indexes[i+1]
			}
			part, err := data.GatherFrame(x, segmentIndexes(x.Len(), start, end))
			if err != nil {
				return nil, err
			}
			segments[i] = part
		}
		return data.NewAny(segments), nil
	default:
		return nil, fmt.Errorf("_ cut expects a vector, string, or frame")
	}
}

func segmentIndexes(length, start, end int) []int {
	if start < 0 {
		start = 0
	}
	if start > length {
		start = length
	}
	if end < start {
		end = start
	}
	if end > length {
		end = length
	}
	indexes := make([]int, end-start)
	for i := range indexes {
		indexes[i] = start + i
	}
	return indexes
}

func dropArray(array data.Array, n int) (data.Array, error) {
	indexes := dropIndexes(array.Len(), n)
	return data.Gather(array, indexes)
}

func dropFrame(frame data.Frame, n int) (data.Frame, error) {
	indexes := dropIndexes(frame.Len(), n)
	return data.GatherFrame(frame, indexes)
}

func takeKeyedFrame(frame data.KeyedFrame, n int) (data.KeyedFrame, error) {
	gathered, err := take(n, frame.Frame())
	if err != nil {
		return data.KeyedFrame{}, err
	}
	out, ok := gathered.(data.Frame)
	if !ok {
		return data.KeyedFrame{}, fmt.Errorf("take keyed table returned %T, want table", gathered)
	}
	return data.KeyBy(out, frame.Keys()...)
}

func dropKeyedFrame(frame data.KeyedFrame, n int) (data.KeyedFrame, error) {
	gathered, err := dropFrame(frame.Frame(), n)
	if err != nil {
		return data.KeyedFrame{}, err
	}
	return data.KeyBy(gathered, frame.Keys()...)
}

func dropString(s string, n int) string {
	runes := []rune(s)
	indexes := dropIndexes(len(runes), n)
	out := make([]rune, len(indexes))
	for i, index := range indexes {
		out[i] = runes[index]
	}
	return string(out)
}

func dropIndexes(length, n int) []int {
	start := 0
	end := length
	if n >= 0 {
		if n > length {
			n = length
		}
		start = n
	} else {
		n = -n
		if n > length {
			n = length
		}
		end = length - n
	}
	indexes := make([]int, end-start)
	for i := range indexes {
		indexes[i] = start + i
	}
	return indexes
}

func takeArrayTail(array data.Array, n int) (data.Array, error) {
	if n > array.Len() {
		n = array.Len()
	}
	start := array.Len() - n
	indexes := make([]int, n)
	for i := range indexes {
		indexes[i] = start + i
	}
	return data.Gather(array, indexes)
}

func takeFrameTail(frame data.Frame, n int) (data.Frame, error) {
	if n > frame.Len() {
		n = frame.Len()
	}
	start := frame.Len() - n
	indexes := make([]int, n)
	for i := range indexes {
		indexes[i] = start + i
	}
	return data.GatherFrame(frame, indexes)
}

func vectorValues(v any) ([]any, error) {
	switch x := v.(type) {
	case data.Array:
		return x.Values(), nil
	default:
		return nil, fmt.Errorf("not a vector")
	}
}

func equalValue(left, right any) bool {
	if data.IsNull(left) || data.IsNull(right) {
		return data.IsNull(left) && data.IsNull(right)
	}
	return reflect.DeepEqual(left, right)
}

func equalComparableValue(left, right any) bool {
	if data.IsNull(left) || data.IsNull(right) {
		return data.IsNull(left) && data.IsNull(right)
	}
	switch l := left.(type) {
	case string:
		if r, ok := right.(data.Symbol); ok {
			return l == string(r)
		}
	case data.Symbol:
		if r, ok := right.(string); ok {
			return string(l) == r
		}
	}
	return reflect.DeepEqual(left, right)
}

func matchValue(left, right any) bool {
	if data.IsNull(left) || data.IsNull(right) {
		return data.IsNull(left) && data.IsNull(right)
	}
	leftEnum, leftIsEnum := left.(qEnumVector)
	rightEnum, rightIsEnum := right.(qEnumVector)
	if leftIsEnum || rightIsEnum {
		if !leftIsEnum || !rightIsEnum || leftEnum.domain != rightEnum.domain {
			return false
		}
		return reflect.DeepEqual(leftEnum.EncodedDomain(), rightEnum.EncodedDomain()) &&
			reflect.DeepEqual(leftEnum.EncodedCodes(), rightEnum.EncodedCodes())
	}
	leftArray, leftIsArray := left.(data.Array)
	rightArray, rightIsArray := right.(data.Array)
	if leftIsArray || rightIsArray {
		if !leftIsArray || !rightIsArray || leftArray.Kind() != rightArray.Kind() || leftArray.Len() != rightArray.Len() {
			return false
		}
		for i := 0; i < leftArray.Len(); i++ {
			leftItem, leftOK := leftArray.At(i)
			rightItem, rightOK := rightArray.At(i)
			if !leftOK || !rightOK || !matchValue(leftItem, rightItem) {
				return false
			}
		}
		return true
	}
	leftDict, leftIsDict := left.(EvalDict)
	rightDict, rightIsDict := right.(EvalDict)
	if leftIsDict || rightIsDict {
		if !leftIsDict || !rightIsDict || len(leftDict.Keys) != len(rightDict.Keys) || len(leftDict.Values) != len(rightDict.Values) {
			return false
		}
		for i := range leftDict.Keys {
			if !matchValue(leftDict.Keys[i], rightDict.Keys[i]) || !matchValue(leftDict.Values[i], rightDict.Values[i]) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(left, right)
}

func likeValue(left, right any) (any, error) {
	leftArray, leftIsArray := left.(data.Array)
	rightArray, rightIsArray := right.(data.Array)
	if !leftIsArray && !rightIsArray {
		return likeScalar(left, right)
	}
	length := 1
	if leftIsArray {
		length = leftArray.Len()
	}
	if rightIsArray {
		if leftIsArray && rightArray.Len() != length {
			return nil, fmt.Errorf("like vector length mismatch: %d vs %d", leftArray.Len(), rightArray.Len())
		}
		length = rightArray.Len()
	}
	out := make([]bool, length)
	for i := 0; i < length; i++ {
		leftItem := left
		if leftIsArray {
			item, ok := leftArray.At(i)
			if !ok {
				return nil, fmt.Errorf("like left row %d out of range", i)
			}
			leftItem = item
		}
		rightItem := right
		if rightIsArray {
			item, ok := rightArray.At(i)
			if !ok {
				return nil, fmt.Errorf("like right row %d out of range", i)
			}
			rightItem = item
		}
		matched, err := likeScalar(leftItem, rightItem)
		if err != nil {
			return nil, err
		}
		out[i] = matched
	}
	return data.NewBool(out), nil
}

func likeScalar(left, right any) (bool, error) {
	if data.IsNull(left) || data.IsNull(right) {
		return false, nil
	}
	text, ok := qLikeText(left)
	if !ok {
		return false, fmt.Errorf("like left expects string or symbol, got %T", left)
	}
	pattern, ok := qLikeText(right)
	if !ok {
		return false, fmt.Errorf("like right expects string or symbol pattern, got %T", right)
	}
	matched, err := path.Match(pattern, text)
	if err != nil {
		return false, fmt.Errorf("like pattern %q: %w", pattern, err)
	}
	return matched, nil
}

func qLikeText(value any) (string, bool) {
	switch x := value.(type) {
	case string:
		return x, true
	case data.Symbol:
		return string(x), true
	default:
		return "", false
	}
}

func qValueKey(v any) string {
	if data.IsNull(v) {
		return "null"
	}
	return fmt.Sprintf("%T:%v", v, v)
}

func compareOrdered(left, right any) (int, error) {
	if ln, ok := numeric(left); ok {
		rn, ok := numeric(right)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareFloat(ln, rn), nil
	}
	switch l := left.(type) {
	case string:
		if r, ok := right.(data.Symbol); ok {
			return strings.Compare(l, string(r)), nil
		}
		r, ok := right.(string)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return strings.Compare(l, r), nil
	case data.Symbol:
		if r, ok := right.(string); ok {
			return strings.Compare(string(l), r), nil
		}
		r, ok := right.(data.Symbol)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return strings.Compare(string(l), string(r)), nil
	case data.Date:
		r, ok := right.(data.Date)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(l.Days(), r.Days()), nil
	case data.Month:
		r, ok := right.(data.Month)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.DateTime:
		r, ok := right.(data.DateTime)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.Timespan:
		r, ok := right.(data.Timespan)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.Minute:
		r, ok := right.(data.Minute)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.Second:
		r, ok := right.(data.Second)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(int64(l), int64(r)), nil
	case data.Time:
		r, ok := right.(data.Time)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(l.Nanos(), r.Nanos()), nil
	case data.Timestamp:
		r, ok := right.(data.Timestamp)
		if !ok {
			return 0, fmt.Errorf("ordered comparison type mismatch %T and %T", left, right)
		}
		return compareInt64(l.UnixNanos(), r.UnixNanos()), nil
	default:
		return 0, fmt.Errorf("ordered comparison is not supported for %T", left)
	}
}

func compareFloat(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func integerValue(v any) (int64, bool) {
	switch n := v.(type) {
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}
