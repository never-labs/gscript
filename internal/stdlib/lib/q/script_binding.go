package q

import (
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptBindingKind uint8

const (
	qScriptBindingInvalid qScriptBindingKind = iota
	qScriptBindingLiteral
	qScriptBindingName
	qScriptBindingVector
	qScriptBindingUnary
	qScriptBindingBinary
	qScriptBindingIndex
)

type qScriptBindingPlan struct {
	kind           qScriptBindingKind
	op             string
	name           string
	literal        any
	items          []qScriptBindingPlan
	left           *qScriptBindingPlan
	right          *qScriptBindingPlan
	cacheableKnown bool
	cacheable      bool
	cached         bool
	cache          any
	// juxtaposedIndex marks an Ident/String-headed vector with an all-Number
	// tail: canonical q juxtaposition indexes when the head resolves to a
	// container (x 1 -> x[1]), mirroring evalJuxtaposedIndexVector.
	juxtaposedIndex bool
}

type qScriptBindingNameResolver func(name string) (*qScriptBindingPlan, bool, error)

func buildQScriptBindingPlan(expr Expr) qScriptBindingPlan {
	switch x := expr.(type) {
	case nil:
		return qScriptBindingPlan{}
	case Number:
		value, _, err := parseNumberOrBool(x.Text)
		if err != nil {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: value}
	case String:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: x.Value}
	case Symbol:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: data.Symbol(x.Name)}
	case Bool:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: x.Value}
	case Null:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: data.NullValue}
	case Temporal:
		value, err := parseQTemporal(x.Kind, x.Text)
		if err != nil {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: value}
	case TypedNull:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: data.NullForKind(data.Kind(x.Kind))}
	case Ident:
		return qScriptBindingPlan{kind: qScriptBindingName, name: x.Name}
	case Vector:
		items := make([]qScriptBindingPlan, len(x.Items))
		literals := make([]any, len(x.Items))
		allLiterals := true
		for i, item := range x.Items {
			items[i] = buildQScriptBindingPlan(item)
			if items[i].kind == qScriptBindingInvalid {
				return qScriptBindingPlan{}
			}
			if items[i].kind != qScriptBindingLiteral {
				allLiterals = false
				continue
			}
			literals[i] = items[i].literal
		}
		juxtaposed := qJuxtaposedIndexVectorShape(x)
		if allLiterals && !juxtaposed {
			value, err := evalValueVector(literals)
			if err != nil {
				return qScriptBindingPlan{}
			}
			return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: value}
		}
		return qScriptBindingPlan{kind: qScriptBindingVector, items: items, juxtaposedIndex: juxtaposed}
	case Call:
		// value/eval are session-routed (value of a string evaluates source
		// against the live env); the binding executor's stateless unary
		// terminal must not claim them (see value_eval_parse.go).
		if x.Func == "value" || x.Func == "eval" {
			return qScriptBindingPlan{}
		}
		arg := buildQScriptBindingPlan(x.Arg)
		if arg.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingUnary, op: x.Func, left: &arg}
	case Binary:
		// Attribute application (`s#10 20 30) is the cascade's dedicated
		// parseAttributeMarker branch, not a take; the binary executor must
		// not claim it (the compiled front-end declines it the same way).
		if x.Op == "#" {
			if sym, ok := x.Left.(Symbol); ok && len(sym.Name) == 1 && isQAttributeMarker(sym.Name[0]) {
				return qScriptBindingPlan{}
			}
		}
		left := buildQScriptBindingPlan(x.Left)
		right := buildQScriptBindingPlan(x.Right)
		if left.kind == qScriptBindingInvalid || right.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingBinary, op: x.Op, left: &left, right: &right}
	case IndexExpr:
		left := buildQScriptBindingPlan(x.Expr)
		right := buildQScriptBindingPlan(x.Index)
		if left.kind == qScriptBindingInvalid || right.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingIndex, left: &left, right: &right}
	default:
		return qScriptBindingPlan{}
	}
}

// buildQScriptBindingPlanForRHS builds the binding plan for one statement
// right-hand side. The canonical text→Expr step is compileQEvalExpr — the
// same front-end the compiled statement route uses — pattern-matched into a
// binding plan by buildQScriptBindingPlanFromCompiled. Forms the compiled
// front-end cannot express or the matcher declines fall back to the qSQL
// parsed tree (the historical route for Pratt-only shapes) and finally to the
// scalar dyadic composition probe.
func buildQScriptBindingPlanForRHS(src string, expr Expr) qScriptBindingPlan {
	src = strings.TrimSpace(src)
	// `drop n v` is the one binding shape the compiled front-end cannot
	// express (compileQEvalExpr reserves the `drop ` prefix for the
	// deferred-state forms), so it keeps a dedicated probe.
	if plan := buildQScriptDropBindingPlan(src); plan.kind != qScriptBindingInvalid {
		return plan
	}
	if compiled := compileQEvalExpr(src, 0); compiled != nil {
		if plan := buildQScriptBindingPlanFromCompiled(compiled); plan.kind != qScriptBindingInvalid {
			return plan
		}
	}
	if expr == nil {
		parsed, ok, err := parseValueExpr(src)
		if err != nil || !ok {
			return buildQScriptScalarDyadicCompositionPlan(src)
		}
		expr = parsed
	}
	plan := buildQScriptBindingPlan(expr)
	if plan.kind != qScriptBindingInvalid {
		return plan
	}
	return buildQScriptScalarDyadicCompositionPlan(src)
}

// buildQScriptBindingPlanFromCompiled maps a compiled Expr tree
// (compileQEvalExpr output) onto a binding plan. Every mapped node executes
// through the same terminal the compiled statement evaluator dispatches to
// (evalValueBinary, lookupUnaryVerb plus the shared til/where branches,
// indexValue/applyCallable), so a claimed plan is value-identical to the
// compiled route, which the dual-route differential pins to the string
// evaluator. Nodes whose binding execution could diverge decline and leave
// the statement on the cascade:
//
//   - fused-probe reducer words over non-leaf arguments stay on the
//     statement-level routes (the add-chain split gives `sum x+y` the string
//     evaluator's (sum x)+y grouping, and the fused kernels outrun a generic
//     unary plan); `where` is exempt because the unary binding evaluator
//     carries its own typed where-compare/within kernels for exactly these
//     shapes,
//   - `?` splits (roll/deal is nondeterministic and must keep drawing through
//     the string evaluator; the binding executor has no runtime decline for
//     it),
//   - word verbs without a lookupDyadicVerbFunc terminal (evalValueBinary
//     would error instead of declining),
//   - list/cast/dict/apply-at/fused forms the binding executor has no kind
//     for (they keep their dedicated plan layers and the compiled statement
//     route).
func buildQScriptBindingPlanFromCompiled(expr Expr) qScriptBindingPlan {
	switch x := expr.(type) {
	case Const:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: x.Value}
	case Call:
		if qFusedReducerWords[x.Func] && x.Func != "where" && !qCompiledLeafExpr(x.Arg) {
			return qScriptBindingPlan{}
		}
		// value/eval are session-routed; see buildQScriptBindingPlan.
		if x.Func == "value" || x.Func == "eval" {
			return qScriptBindingPlan{}
		}
		arg := buildQScriptBindingPlanFromCompiled(x.Arg)
		if arg.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingUnary, op: x.Func, left: &arg}
	case Binary:
		if x.Op == "?" {
			return qScriptBindingPlan{}
		}
		left := buildQScriptBindingPlanFromCompiled(x.Left)
		if left.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		right := buildQScriptBindingPlanFromCompiled(x.Right)
		if right.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingBinaryPlan(x.Op, left, right)
	case DyadicWordExpr:
		if _, ok := lookupDyadicVerbFunc(x.Word); !ok {
			return qScriptBindingPlan{}
		}
		left := buildQScriptBindingPlanFromCompiled(x.Left)
		if left.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		right := buildQScriptBindingPlanFromCompiled(x.Right)
		if right.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingBinaryPlan(x.Word, left, right)
	case IndexExpr:
		collection := buildQScriptBindingPlanFromCompiled(x.Expr)
		if collection.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		index := buildQScriptBindingPlanFromCompiled(x.Index)
		if index.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingIndex, left: &collection, right: &index}
	default:
		// Leaves (Ident, Number, Vector, ...) share the parsed-tree mapping;
		// every unsupported kind declines there too.
		return buildQScriptBindingPlan(expr)
	}
}

// buildQScriptDropBindingPlan plans prefix-dyadic `drop <n> <value>`
// statements (the word form, not the `_` operator).
func buildQScriptDropBindingPlan(src string) qScriptBindingPlan {
	if !strings.HasPrefix(src, "drop ") || !wordBoundary(src, 0, len("drop")) {
		return qScriptBindingPlan{}
	}
	countExpr, valueExpr, ok := splitQScriptPrefixDyadicArgs(strings.TrimSpace(src[len("drop "):]))
	if !ok {
		return qScriptBindingPlan{}
	}
	countPlan := buildQScriptScalarLiteralBindingPlan(countExpr)
	if countPlan.kind == qScriptBindingInvalid {
		return qScriptBindingPlan{}
	}
	valuePlan := buildQScriptBindingPlanForRHS(valueExpr, nil)
	if valuePlan.kind == qScriptBindingInvalid {
		return qScriptBindingPlan{}
	}
	return qScriptBindingBinaryPlan("drop", countPlan, valuePlan)
}

// buildQScriptScalarDyadicCompositionPlan plans `<scalar-literal> <op> <group>`
// and `<group> <op> <scalar-literal>` compositions for the single-character
// arithmetic verbs, where <group> is one fully parenthesized expression or a
// plain name and the other operand is a numeric scalar literal. The split
// position mirrors s.eval's findDyadic scan exactly, and the operand shapes
// guarantee no earlier special form in the eval cascade (take/drop/cast/dict,
// adverbs, postfix lookups, statement lists) can claim the source first, so
// the cached plan evaluates identically to string evaluation.
func buildQScriptScalarDyadicCompositionPlan(src string) qScriptBindingPlan {
	src = strings.TrimSpace(src)
	idx, op, ok := findDyadic(src)
	if !ok || idx <= 0 || idx+1 >= len(src) {
		return qScriptBindingPlan{}
	}
	switch op {
	case '+', '-', '*', '%':
	default:
		return qScriptBindingPlan{}
	}
	left, leftLiteral, ok := qScriptScalarCompositionOperandPlan(strings.TrimSpace(src[:idx]))
	if !ok {
		return qScriptBindingPlan{}
	}
	right, rightLiteral, ok := qScriptScalarCompositionOperandPlan(strings.TrimSpace(src[idx+1:]))
	if !ok {
		return qScriptBindingPlan{}
	}
	if !leftLiteral && !rightLiteral {
		return qScriptBindingPlan{}
	}
	return qScriptBindingBinaryPlan(string(op), left, right)
}

// qScriptCompositionPlainName matches underscore-free identifiers: '_' is
// also s.eval's top-level drop operator, so names containing it stay on the
// string-eval path.
func qScriptCompositionPlainName(src string) bool {
	if src == "" || !isQIdentStart(src[0]) {
		return false
	}
	for i := 1; i < len(src); i++ {
		if !isQIdentRest(src[i]) || src[i] == '_' {
			return false
		}
	}
	return true
}

func qScriptScalarCompositionOperandPlan(src string) (plan qScriptBindingPlan, literal bool, ok bool) {
	if src == "" {
		return qScriptBindingPlan{}, false, false
	}
	if value, _, err := parseNumberOrBool(src); err == nil {
		if _, isNumeric := numeric(value); isNumeric {
			return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: value}, true, true
		}
		return qScriptBindingPlan{}, false, false
	}
	if qScriptCompositionPlainName(src) {
		return qScriptBindingPlan{kind: qScriptBindingName, name: src}, false, true
	}
	if len(src) >= 2 && src[0] == '(' && src[len(src)-1] == ')' {
		inner := stripEnclosingParens(src)
		if inner == src {
			return qScriptBindingPlan{}, false, false
		}
		groupPlan := buildQScriptBindingPlanForRHS(strings.TrimSpace(inner), nil)
		if groupPlan.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}, false, false
		}
		return groupPlan, false, true
	}
	return qScriptBindingPlan{}, false, false
}

func splitQScriptPrefixDyadicArgs(src string) (string, string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", "", false
	}
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
		case ' ', '\t', '\n', '\r':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				left := strings.TrimSpace(src[:i])
				right := strings.TrimSpace(src[i+1:])
				return left, right, left != "" && right != ""
			}
		}
	}
	return "", "", false
}

func buildQScriptPrefixBindingPlan(src string) qScriptBindingPlan {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "til ") && wordBoundary(src, 0, len("til")) {
		arg := strings.TrimSpace(src[len("til "):])
		expr, ok, err := parseValueExpr(arg)
		if err != nil || !ok {
			return qScriptBindingPlan{}
		}
		argPlan := buildQScriptBindingPlan(expr)
		if argPlan.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingUnary, op: "til", left: &argPlan}
	}
	if strings.HasPrefix(src, "where ") && wordBoundary(src, 0, len("where")) {
		arg := strings.TrimSpace(src[len("where "):])
		expr, ok, err := parseValueExpr(arg)
		if err != nil || !ok {
			return qScriptBindingPlan{}
		}
		argPlan := buildQScriptBindingPlan(expr)
		if argPlan.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingUnary, op: "where", left: &argPlan}
	}
	return qScriptBindingPlan{}
}

func buildQScriptRangeBindingPlan(src string) qScriptBindingPlan {
	src = strings.TrimSpace(src)
	if src == "" {
		return qScriptBindingPlan{}
	}
	if plan := buildQScriptPrefixBindingPlan(src); plan.kind != qScriptBindingInvalid {
		return plan
	}
	for _, op := range []string{"+", "-"} {
		left, right, ok := splitTopLevelOperator(src, op)
		if !ok {
			continue
		}
		if til := buildQScriptPrefixBindingPlan(right); til.kind != qScriptBindingInvalid {
			if scalar := buildQScriptScalarLiteralBindingPlan(left); scalar.kind != qScriptBindingInvalid {
				return qScriptBindingBinaryPlan(op, scalar, til)
			}
		}
		if til := buildQScriptPrefixBindingPlan(left); til.kind != qScriptBindingInvalid {
			if scalar := buildQScriptScalarLiteralBindingPlan(right); scalar.kind != qScriptBindingInvalid {
				return qScriptBindingBinaryPlan(op, til, scalar)
			}
		}
	}
	left, right, ok := splitTopLevelOperator(src, "*")
	if !ok {
		return qScriptBindingPlan{}
	}
	if til := buildQScriptPrefixBindingPlan(right); til.kind != qScriptBindingInvalid {
		if scalar := buildQScriptScalarLiteralBindingPlan(left); scalar.kind != qScriptBindingInvalid {
			return qScriptBindingBinaryPlan("*", scalar, til)
		}
	}
	if til := buildQScriptPrefixBindingPlan(left); til.kind != qScriptBindingInvalid {
		if scalar := buildQScriptScalarLiteralBindingPlan(right); scalar.kind != qScriptBindingInvalid {
			return qScriptBindingBinaryPlan("*", til, scalar)
		}
	}
	return qScriptBindingPlan{}
}

func buildQScriptScalarLiteralBindingPlan(src string) qScriptBindingPlan {
	src = strings.TrimSpace(src)
	if value, _, err := parseNumberOrBool(src); err == nil {
		plan := qScriptBindingPlan{kind: qScriptBindingLiteral, literal: value}
		if _, ok := integerValue(plan.literal); ok {
			return plan
		}
	}
	expr, ok, err := parseValueExpr(src)
	if err != nil || !ok {
		return qScriptBindingPlan{}
	}
	plan := buildQScriptBindingPlan(expr)
	if plan.kind != qScriptBindingLiteral {
		return qScriptBindingPlan{}
	}
	if _, ok := integerValue(plan.literal); !ok {
		return qScriptBindingPlan{}
	}
	return plan
}

func qScriptBindingBinaryPlan(op string, left, right qScriptBindingPlan) qScriptBindingPlan {
	return qScriptBindingPlan{kind: qScriptBindingBinary, op: op, left: &left, right: &right}
}

func (s *EvalState) evalQScriptBindingPlan(plan *qScriptBindingPlan) (any, bool, error) {
	return s.evalQScriptBindingPlanWithResolver(plan, nil)
}

func (s *EvalState) evalQScriptBindingPlanWithResolver(plan *qScriptBindingPlan, resolver qScriptBindingNameResolver) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	if plan.cached {
		return plan.cache, true, nil
	}
	cacheable := qScriptBindingPlanCacheable(plan)
	var (
		value   any
		handled bool
		err     error
	)
	switch plan.kind {
	case qScriptBindingInvalid:
		return nil, false, nil
	case qScriptBindingLiteral:
		return plan.literal, true, nil
	case qScriptBindingName:
		if resolver != nil {
			resolved, ok, err := resolver(plan.name)
			if err != nil || ok {
				if err != nil || resolved == nil {
					return nil, ok, err
				}
				return s.evalQScriptBindingPlanWithResolver(resolved, resolver)
			}
		}
		value, ok := s.lookupName(plan.name)
		if !ok {
			return nil, false, nil
		}
		return value, true, nil
	case qScriptBindingVector:
		values := make([]any, len(plan.items))
		for i := range plan.items {
			value, handled, err := s.evalQScriptBindingPlanWithResolver(&plan.items[i], resolver)
			if err != nil || !handled {
				return nil, handled, err
			}
			values[i] = value
		}
		if plan.juxtaposedIndex {
			if out, indexed, err := qJuxtaposedIndexVectorValue(values); indexed || err != nil {
				return out, true, err
			}
		}
		out, err := evalValueVector(values)
		value, handled = out, true
		if err != nil {
			return value, handled, err
		}
	case qScriptBindingUnary:
		value, handled, err = s.evalQScriptUnaryBinding(plan, resolver)
	case qScriptBindingBinary:
		value, handled, err = s.evalQScriptBinaryBinding(plan, resolver)
	case qScriptBindingIndex:
		collection, collectionHandled, collectionErr := s.evalQScriptBindingPlanWithResolver(plan.left, resolver)
		if collectionErr != nil || !collectionHandled {
			return nil, collectionHandled, collectionErr
		}
		index, indexHandled, indexErr := s.evalQScriptBindingPlanWithResolver(plan.right, resolver)
		if indexErr != nil || !indexHandled {
			return nil, indexHandled, indexErr
		}
		if isCallable(collection) {
			out, applyErr := s.applyCallable(collection, []any{index})
			value, handled = out, true
			if applyErr != nil {
				return value, handled, applyErr
			}
			break
		}
		out, indexValueErr := indexValue(collection, index)
		value, handled = out, true
		if indexValueErr != nil {
			return value, handled, indexValueErr
		}
	default:
		return nil, false, nil
	}
	if cacheable && handled && err == nil {
		plan.cache = value
		plan.cached = true
	}
	return value, handled, err
}

func (s *EvalState) evalQScriptUnaryBinding(plan *qScriptBindingPlan, resolver qScriptBindingNameResolver) (any, bool, error) {
	if plan.op == "where" && plan.left != nil && plan.left.kind == qScriptBindingBinary && plan.left.op == "within" {
		left, handled, err := s.evalQScriptBindingPlanWithResolver(plan.left.left, resolver)
		if err != nil || !handled {
			return nil, handled, err
		}
		right, handled, err := s.evalQScriptBindingPlanWithResolver(plan.left.right, resolver)
		if err != nil || !handled {
			return nil, handled, err
		}
		desc, ok, err := qTypedWhereCompareIndexesDescriptor(left, right, plan.left.op, "compare-to-index", "within-to-index")
		if err != nil || !ok {
			return nil, ok, err
		}
		desc.fallbackReason = RuntimeFallbackUnsupportedType
		out, handled, err := evalQTypedWhereCompareIndexes(desc)
		if err != nil || handled {
			return out, true, err
		}
	}
	if plan.op == "where" && plan.left != nil && plan.left.kind == qScriptBindingBinary {
		if _, opOK := qDataCompareOpString(plan.left.op); opOK {
			left, handled, err := s.evalQScriptBindingPlanWithResolver(plan.left.left, resolver)
			if err != nil || !handled {
				return nil, handled, err
			}
			right, handled, err := s.evalQScriptBindingPlanWithResolver(plan.left.right, resolver)
			if err != nil || !handled {
				return nil, handled, err
			}
			desc, ok, err := qTypedWhereCompareIndexesDescriptor(left, right, plan.left.op, "compare-to-index", "within-to-index")
			if err != nil || ok {
				if err != nil {
					return nil, ok, err
				}
				desc.fallbackReason = RuntimeFallbackUnsupportedType
				out, handled, err := evalQTypedWhereCompareIndexes(desc)
				if err != nil || handled {
					return out, true, err
				}
			}
		}
	}
	arg, handled, err := s.evalQScriptBindingPlanWithResolver(plan.left, resolver)
	if err != nil || !handled {
		return nil, handled, err
	}
	switch plan.op {
	case "til":
		n, ok := integerValue(arg)
		if !ok {
			return nil, true, fmt.Errorf("til expects an integer")
		}
		if n < 0 {
			return nil, true, fmt.Errorf("til expects a non-negative integer")
		}
		if int64(int(n)) != n {
			return nil, true, fmt.Errorf("til count is too large")
		}
		return data.NewI64Range(0, 1, int(n)), true, nil
	case "where":
		if mask, ok := arg.(data.Array); ok && mask.Kind() == data.KindBool {
			out, handled, err := qEvalWhereMaskI64Primitive(mask)
			if err != nil || handled {
				return out, true, err
			}
		}
		out, err := where(arg)
		return out, true, err
	}
	fn, ok := lookupUnaryVerb(plan.op)
	if !ok {
		return nil, false, nil
	}
	out, err := fn(arg)
	return out, true, err
}

func (s *EvalState) evalQScriptBinaryBinding(plan *qScriptBindingPlan, resolver qScriptBindingNameResolver) (any, bool, error) {
	left, handled, err := s.evalQScriptBindingPlanWithResolver(plan.left, resolver)
	if err != nil || !handled {
		return nil, handled, err
	}
	right, handled, err := s.evalQScriptBindingPlanWithResolver(plan.right, resolver)
	if err != nil || !handled {
		return nil, handled, err
	}
	if plan.op == "#" {
		if _, ok := left.(data.Array); ok {
			out, err := reshapeValue(left, right)
			return out, true, err
		}
		n, ok := integerValue(left)
		if !ok || int64(int(n)) != n {
			return nil, true, fmt.Errorf("# left operand must be an integer count")
		}
		out, err := take(int(n), right)
		return out, true, err
	}
	if plan.op == "drop" {
		n, ok := integerValue(left)
		if !ok || int64(int(n)) != n {
			return nil, true, fmt.Errorf("drop expects an integer count")
		}
		out, err := drop(int(n), right)
		return out, true, err
	}
	if plan.op == "rotate" {
		out, err := rotateValue(left, right)
		return out, true, err
	}
	if plan.op == "sublist" {
		out, err := qSublistValue(left, right)
		return out, true, err
	}
	if plan.op == "and" || plan.op == "or" {
		if out, handled, err := data.TryTypedBoolLogical(plan.op, left, right); err != nil || handled {
			out, handled, err = qTypedRuntimeResultReason("ArrayBoolLogical", qScriptBoolLogicalShape(plan.op), RuntimeFallbackUnsupportedType, out, handled, err)
			if err != nil {
				return nil, true, err
			}
			return out, true, nil
		}
		out, err := evalValueBinary(plan.op, left, right)
		return out, true, err
	}
	// Tiled-cycle pushdown for compares mirrors the string evaluator's
	// applyVectorDyadic probe: comparing a long cyclic view computes on the
	// short tile and re-tiles, keeping downstream `where` on the O(period)
	// periodic kernels. Arithmetic ops stay on the typed dyadic kernels
	// below, whose eager null-bitmap forms feed the bulk reducers directly.
	if opByte, _, ok := lookupDyadicVerb(plan.op); ok {
		switch opByte {
		case '=', '<', '>':
			la, _ := left.(data.Array)
			ra, _ := right.(data.Array)
			if la != nil || ra != nil {
				if out, handled, err := qTryTiledCycleVectorDyadic(opByte, left, right, la, ra); err != nil || handled {
					if err != nil {
						return nil, true, err
					}
					return out, true, nil
				}
			}
		}
	}
	if dataOp, ok := qDataCompareOpString(plan.op); ok {
		la, _ := left.(data.Array)
		ra, _ := right.(data.Array)
		if la != nil || ra != nil {
			out, handled, err := qTryTypedCompareMask(dataOp, left, right, la, ra)
			out, handled, err = qTypedRuntimeResultReason("ArrayDyadicCompare", qRuntimeKernelCompositeVectorDyadicShape(plan.op, left, right, la, ra), RuntimeFallbackUnsupportedType, out, handled, err)
			if err != nil {
				return nil, true, err
			}
			if handled {
				return out, true, nil
			}
		}
	}
	if len(plan.op) == 1 {
		op := plan.op[0]
		if dataOp, ok := qDataArithmeticOp(op); ok {
			la, _ := left.(data.Array)
			ra, _ := right.(data.Array)
			if la != nil || ra != nil {
				typedLeft, typedRight, canUse, err := qVectorDyadicTypedOperands(left, right, la, ra)
				if err != nil {
					return nil, true, err
				}
				if canUse && qVectorDyadicCanUseTypedArithmetic(typedLeft, typedRight) {
					out, handled, err := qTryTypedArithmeticDyadic(dataOp, typedLeft, typedRight)
					out, handled, err = qTypedRuntimeResultReason("ArrayDyadicArithmetic", qRuntimeKernelVectorDyadicShape(op, left, right, la, ra), RuntimeFallbackUnsupportedType, out, handled, err)
					if err != nil {
						return nil, true, err
					}
					if handled {
						return out, true, nil
					}
				}
			}
		}
	}
	out, err := evalValueBinary(plan.op, left, right)
	return out, true, err
}

func qScriptBindingPlanCacheable(plan *qScriptBindingPlan) bool {
	if plan == nil {
		return false
	}
	if plan.cacheableKnown {
		return plan.cacheable
	}
	cacheable := false
	switch plan.kind {
	case qScriptBindingInvalid, qScriptBindingName:
		cacheable = false
	case qScriptBindingLiteral:
		cacheable = true
	case qScriptBindingVector:
		cacheable = true
		for i := range plan.items {
			if !qScriptBindingPlanCacheable(&plan.items[i]) {
				cacheable = false
				break
			}
		}
	case qScriptBindingUnary:
		cacheable = qScriptBindingPlanCacheable(plan.left)
	case qScriptBindingBinary, qScriptBindingIndex:
		cacheable = qScriptBindingPlanCacheable(plan.left) && qScriptBindingPlanCacheable(plan.right)
	default:
		cacheable = false
	}
	plan.cacheable = cacheable
	plan.cacheableKnown = true
	return cacheable
}

func qScriptBoolLogicalShape(op string) string {
	switch op {
	case "and":
		return "logical/and"
	case "or":
		return "logical/or"
	default:
		return "logical/" + op
	}
}
