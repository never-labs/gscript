package q

// value / eval / parse: the q evaluation verbs.
//
// Dispatch matrix for `value x` (canonical kdb+/q semantics by argument type):
//
//	string      -> evaluate x as q source IN THE CURRENT session (the live
//	               env is visible and assignments inside the string mutate
//	               it, mirroring q's value-runs-in-current-context rule).
//	symbol      -> dereference the name in the session env; unbound names
//	               signal the q name error 'name.
//	parse tree  -> evaluate the tree (a generic list headed by a verb
//	               symbol or callable), same as `eval`.
//	dict        -> its values (pre-existing behavior, kept).
//	enum/attr   -> decoded values / underlying vector (kept).
//	keyed table -> value frame (kept).
//	lambda      -> KNOWN GAP: canonical q returns the lambda's metadata
//	               list; this dialect returns the lambda unchanged.
//	other atoms -> returned unchanged (canonical: value 42 -> 42).
//
// `eval x` evaluates a parse tree; non-tree atoms and typed vectors evaluate
// to themselves; symbols resolve as names (canonical q: eval `a is the value
// of a, literal symbols round-trip through parse as enlisted symbols).
//
// `parse "src"` compiles src (single expression statement) and converts the
// compiled Expr to a nested-list parse tree.
//
// TREE SPELLING (dialect deviation, documented):
// canonical q parse returns function ATOMS in head position ((+;2;3) prints
// the + function). This dialect spells verbs as their SYMBOL names instead:
//
//	parse "2*3+1"     -> (`$"*";2;(`$"+";3;1))   (generic list, symbol head)
//	parse "7 mod 3"   -> (`mod;7;3)
//	parse "neg 3"     -> (`neg;3)
//	parse "a+1"       -> (`$"+";`a;1)            names are plain symbols
//	parse "`a"        -> ,`a                     literal symbols are enlisted
//	parse "(1;2;3)"   -> (`enlist;1;2;3)         list construction
//	parse "a[1]"      -> (`a;1)                  juxtaposed application
//	parse "1 2 3"     -> 1 2 3                   literal vectors are constants
//
// ROUND-TRIP CONTRACT: eval parse src ≡ value src for the supported subset
// (literals, names, unary/dyadic verb applications, casts, dicts, indexing,
// list construction). Out-of-subset sources (assignments, multi-statement
// scripts, lambdas, adverbs, fused reducer chains) make parse error with a
// clear message rather than produce an unfaithful tree.
//
// MEMO SAFETY: `value` and `eval` are registered Stateful in verb_metadata.go
// (value-of-string runs arbitrary q source against the live session; eval can
// reach value through a tree), so the four source-eligibility predicates keep
// them out of every memo layer. The compiled-statement route (evalValueCall)
// and the script-binding plan builder both decline value-of-string/tree and
// eval so the string evaluator is the single execution route — the same
// pattern roll/deal use — keeping the compiled/string dual-route differential
// free of double evaluation. `parse` is pure (string -> tree) and stays
// deterministic.

import (
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// qValueEvalMaxDepth bounds self-referential value/eval recursion
// (a:"value a";value a). Canonical q signals 'stack on runaway recursion.
const qValueEvalMaxDepth = 64

// unaryVerbFunc resolves a unary verb name, routing the session-dependent
// verbs (value, eval) through s so string evaluation, symbol dereference,
// and parse-tree evaluation see the live environment. Every dispatch site
// with an EvalState in scope should prefer this over lookupUnaryVerb.
func (s *EvalState) unaryVerbFunc(name string) (func(any) (any, error), bool) {
	switch strings.TrimSpace(name) {
	case "value":
		return s.valueVerb, true
	case "eval":
		return s.evalTreeVerb, true
	}
	return lookupUnaryVerb(name)
}

// bindStatefulUnaryFns rebinds the session-dependent members of a unary
// composition (parseUnaryComposition builds them stateless) to s.
func (s *EvalState) bindStatefulUnaryFns(funcs []qUnaryFunction) []qUnaryFunction {
	for i := range funcs {
		switch funcs[i].name {
		case "value":
			funcs[i].fn = s.valueVerb
		case "eval":
			funcs[i].fn = s.evalTreeVerb
		}
	}
	return funcs
}

// valueVerb is the session-aware `value` implementation (dispatch matrix in
// the file comment).
func (s *EvalState) valueVerb(v any) (any, error) {
	switch x := v.(type) {
	case string:
		src := strings.TrimSpace(x)
		if src == "" {
			return nil, fmt.Errorf("value: empty source string")
		}
		if s.valueDepth >= qValueEvalMaxDepth {
			return nil, fmt.Errorf("'stack (value recursion limit %d exceeded)", qValueEvalMaxDepth)
		}
		s.valueDepth++
		defer func() { s.valueDepth-- }()
		return s.evalScript(src)
	case data.Symbol:
		return s.valueSymbolDeref(x)
	case data.Array:
		if qIsParseTreeList(x) {
			return s.evalParseTree(x)
		}
		return value(v)
	default:
		// Dict/enum/attributed/keyed dispatch, the documented lambda gap,
		// and the atom identity (value 42 -> 42) all live in the stateless
		// registry function.
		return value(v)
	}
}

// valueSymbolDeref resolves `value `name: the session binding, or the
// canonical q name error 'name when unbound.
func (s *EvalState) valueSymbolDeref(sym data.Symbol) (any, error) {
	name := string(sym)
	if value, ok := s.lookupName(name); ok {
		return value, nil
	}
	return nil, fmt.Errorf("'%s", name)
}

// evalTreeVerb is the session-aware `eval` implementation: parse trees
// evaluate, symbols resolve as names, everything else is already a value.
func (s *EvalState) evalTreeVerb(v any) (any, error) {
	return s.evalParseTree(v)
}

// evalParseTree evaluates one parse-tree node (tree spelling in the file
// comment).
func (s *EvalState) evalParseTree(v any) (any, error) {
	if s.valueDepth >= qValueEvalMaxDepth {
		return nil, fmt.Errorf("'stack (eval recursion limit %d exceeded)", qValueEvalMaxDepth)
	}
	s.valueDepth++
	defer func() { s.valueDepth-- }()
	switch x := v.(type) {
	case data.Symbol:
		return s.resolveParseTreeName(string(x))
	case data.Array:
		if x.Kind() != data.KindAny {
			return v, nil // typed vectors are constants
		}
		n := x.Len()
		if n == 0 {
			return v, nil
		}
		items := make([]any, n)
		for i := 0; i < n; i++ {
			item, ok := x.At(i)
			if !ok {
				return nil, fmt.Errorf("eval: tree item %d out of range", i)
			}
			items[i] = item
		}
		// Enlisted symbol = literal symbol (mirrors q's ,`a parse spelling).
		if n == 1 {
			if sym, ok := items[0].(data.Symbol); ok {
				return sym, nil
			}
			return v, nil
		}
		if sym, ok := items[0].(data.Symbol); ok {
			return s.applyParseTreeVerb(string(sym), items[1:])
		}
		// Non-symbol head: evaluate it and apply/index.
		head, err := s.evalParseTree(items[0])
		if err != nil {
			return nil, err
		}
		args, err := s.evalParseTreeArgs(items[1:])
		if err != nil {
			return nil, err
		}
		return s.applyParseTreeHead(head, args)
	default:
		return v, nil // atoms evaluate to themselves
	}
}

func (s *EvalState) evalParseTreeArgs(trees []any) ([]any, error) {
	args := make([]any, len(trees))
	for i, tree := range trees {
		arg, err := s.evalParseTree(tree)
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}
	return args, nil
}

func (s *EvalState) applyParseTreeHead(head any, args []any) (any, error) {
	if isCallable(head) {
		return s.applyCallable(head, args)
	}
	if len(args) == 1 {
		return indexValue(head, args[0])
	}
	return nil, fmt.Errorf("eval: tree head %T is not callable", head)
}

// resolveParseTreeName resolves a bare symbol in tree position: session
// bindings shadow verbs (matching the Ident resolution order of the value
// expression evaluator), then dyadic and unary verb values, else the q name
// error.
func (s *EvalState) resolveParseTreeName(name string) (any, error) {
	if value, ok := s.lookupName(name); ok {
		return value, nil
	}
	if fn, ok := lookupDyadicVerbFunc(name); ok {
		return qDyadicFunction{name: name, fn: fn}, nil
	}
	if fn, ok := s.unaryVerbFunc(name); ok {
		return qUnaryFunction{name: name, fn: fn}, nil
	}
	return nil, fmt.Errorf("'%s", name)
}

// applyParseTreeVerb applies a symbol-headed tree node: bound names first
// (application/indexing), then arity-directed verb dispatch through the same
// machinery the compiled statement route executes, so eval∘parse stays
// value-identical to the string route.
func (s *EvalState) applyParseTreeVerb(name string, trees []any) (any, error) {
	if head, ok := s.lookupName(name); ok {
		args, err := s.evalParseTreeArgs(trees)
		if err != nil {
			return nil, err
		}
		return s.applyParseTreeHead(head, args)
	}
	switch len(trees) {
	case 1:
		arg, err := s.evalParseTree(trees[0])
		if err != nil {
			return nil, err
		}
		switch name {
		case "value":
			return s.valueVerb(arg)
		case "eval":
			return s.evalTreeVerb(arg)
		}
		out, err := s.evalValueCall(Call{Func: name, Arg: Const{Value: arg}})
		if err != nil && isUnsupportedEvalValueExpr(err) {
			return nil, fmt.Errorf("'%s", name)
		}
		return out, err
	case 2:
		// `$` cast carries the bare-cast-name fallback (`long$x with long
		// unbound) before its operands evaluate, mirroring CastExpr.
		if name == "$" {
			return s.evalParseTreeCast(trees[0], trees[1])
		}
		left, err := s.evalParseTree(trees[0])
		if err != nil {
			return nil, err
		}
		right, err := s.evalParseTree(trees[1])
		if err != nil {
			return nil, err
		}
		if name == "enlist" {
			return inferQArray([]any{left, right}), nil
		}
		if op, ok := qDyadicWordOps[name]; ok {
			return op.fn(left, right)
		}
		if op, ok := qSetWordOps[name]; ok {
			return op.fn(left, right)
		}
		switch name {
		case "!":
			return s.evalValueExpr(DictExpr{Keys: Const{Value: left}, Values: Const{Value: right}})
		case "@":
			return s.evalValueExpr(ApplyAtExpr{Left: Const{Value: left}, Right: Const{Value: right}})
		case "?":
			// Atom-int LHS is canonical roll/deal and needs the session PRNG.
			if n, ok := qRollDealCount(left); ok {
				return s.rollOrDeal(n, right)
			}
			return findValue(left, right)
		}
		out, err := evalValueBinary(name, left, right)
		if err != nil && isUnsupportedEvalValueExpr(err) {
			return nil, fmt.Errorf("'%s", name)
		}
		return out, err
	default:
		// n-ary list construction (`enlist;a;b;c) from parenthesized lists.
		if name == "enlist" {
			args, err := s.evalParseTreeArgs(trees)
			if err != nil {
				return nil, err
			}
			return inferQArray(args), nil
		}
		return nil, fmt.Errorf("eval: verb %s does not take %d arguments", name, len(trees))
	}
}

// evalParseTreeCast evaluates (`$;domain;value): the domain tree, with the
// bare-cast-name fallback when the domain is an unbound name that spells a
// cast kind (mirrors CastExpr.BareSym).
func (s *EvalState) evalParseTreeCast(domainTree, valueTree any) (any, error) {
	domain, err := s.evalParseTree(domainTree)
	if err != nil {
		sym, ok := domainTree.(data.Symbol)
		if !ok || !isBareQCastName(string(sym)) {
			return nil, err
		}
		domain = sym
	}
	value, err := s.evalParseTree(valueTree)
	if err != nil {
		return nil, err
	}
	return castOrEnum(domain, value)
}

// qIsParseTreeList reports whether v looks like a parse tree: a generic list
// whose head is a verb/name symbol or a callable, or an enlisted symbol
// literal. Other generic lists keep the pre-existing value passthrough.
func qIsParseTreeList(v data.Array) bool {
	if v.Kind() != data.KindAny || v.Len() == 0 {
		return false
	}
	head, ok := v.At(0)
	if !ok {
		return false
	}
	if _, isSym := head.(data.Symbol); isSym {
		return true
	}
	return isCallable(head)
}

// qParseVerb is the stateless `parse` verb: q source string -> parse tree.
// Pure (no session reads), so it stays deterministic in verb metadata and is
// safe on every dispatch route.
func qParseVerb(v any) (any, error) {
	src, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("parse expects a string, got %T", v)
	}
	return qSourceToParseTree(src)
}

func qSourceToParseTree(src string) (any, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("parse: empty source")
	}
	if stmts := splitQScriptStatements(src); len(stmts) > 1 {
		return nil, fmt.Errorf("parse: multi-statement source is not supported (got %d statements)", len(stmts))
	}
	if name, _, ok := splitTopLevelAssignment(src); ok && name != "" {
		return nil, fmt.Errorf("parse: assignment statements are not supported")
	}
	expr := qCompileParseExpr(src)
	if expr == nil {
		return nil, fmt.Errorf("parse: unsupported source %q (supported: literals, names, verb applications)", src)
	}
	return qExprToParseTree(expr)
}

// qCompileParseExpr compiles src for parse-tree conversion. count/sum
// prefixes route around the fused-probe matchers (fused nodes have no
// faithful tree spelling); everything else reuses the statement compiler.
func qCompileParseExpr(src string) Expr {
	if space := strings.IndexByte(src, ' '); space > 0 {
		word := src[:space]
		if word == "count" || word == "sum" {
			arg := qCompileParseExpr(strings.TrimSpace(src[space+1:]))
			if arg == nil {
				return nil
			}
			return Call{Func: word, Arg: arg}
		}
	}
	return compileQEvalExpr(src, 0)
}

func qParseTreeList(items ...any) any {
	return data.NewAny(items)
}

func qParseTreeSymbolLiteral(sym data.Symbol) any {
	return data.NewAny([]any{sym})
}

// qExprToParseTree converts a compiled Expr to the nested-list tree spelling
// documented in the file comment.
func qExprToParseTree(expr Expr) (any, error) {
	switch x := expr.(type) {
	case Const:
		if sym, ok := x.Value.(data.Symbol); ok {
			return qParseTreeSymbolLiteral(sym), nil
		}
		return x.Value, nil
	case Number:
		value, _, err := parseNumberOrBool(x.Text)
		return value, err
	case Bool:
		return x.Value, nil
	case String:
		return x.Value, nil
	case Symbol:
		return qParseTreeSymbolLiteral(data.Symbol(x.Name)), nil
	case Null:
		return data.NullValue, nil
	case Temporal:
		return parseQTemporal(x.Kind, x.Text)
	case TypedNull:
		return data.NullForKind(data.Kind(x.Kind)), nil
	case Ident:
		return data.Symbol(x.Name), nil
	case Call:
		arg, err := qExprToParseTree(x.Arg)
		if err != nil {
			return nil, err
		}
		return qParseTreeList(data.Symbol(x.Func), arg), nil
	case SafeCall:
		return qExprToParseTree(Call{Func: x.Func, Arg: x.Arg})
	case Binary:
		return qExprToParseTreeDyad(x.Op, x.Left, x.Right)
	case DyadicWordExpr:
		return qExprToParseTreeDyad(x.Word, x.Left, x.Right)
	case CastExpr:
		var domain any
		switch {
		case x.Domain != nil:
			tree, err := qExprToParseTree(x.Domain)
			if err != nil {
				return nil, err
			}
			domain = tree
		case x.BareSym != "":
			domain = data.Symbol(x.BareSym)
		default:
			domain = qParseTreeSymbolLiteral(data.Symbol(""))
		}
		value, err := qExprToParseTree(x.Value)
		if err != nil {
			return nil, err
		}
		return qParseTreeList(data.Symbol("$"), domain, value), nil
	case DictExpr:
		return qExprToParseTreeDyad("!", x.Keys, x.Values)
	case ApplyAtExpr:
		return qExprToParseTreeDyad("@", x.Left, x.Right)
	case IndexExpr:
		collection, err := qExprToParseTree(x.Expr)
		if err != nil {
			return nil, err
		}
		index, err := qExprToParseTree(x.Index)
		if err != nil {
			return nil, err
		}
		return qParseTreeList(collection, index), nil
	case ListExpr:
		if x.Bare {
			return nil, fmt.Errorf("parse: bare expression lists are not supported")
		}
		items := make([]any, 0, len(x.Items)+1)
		items = append(items, data.Symbol("enlist"))
		for _, item := range x.Items {
			tree, err := qExprToParseTree(item)
			if err != nil {
				return nil, err
			}
			items = append(items, tree)
		}
		return qParseTreeList(items...), nil
	case Vector:
		// Literal vectors const-fold before reaching here; a residual Vector
		// carries non-literal items and spells as list construction.
		items := make([]any, 0, len(x.Items)+1)
		items = append(items, data.Symbol("enlist"))
		for _, item := range x.Items {
			tree, err := qExprToParseTree(item)
			if err != nil {
				return nil, err
			}
			items = append(items, tree)
		}
		return qParseTreeList(items...), nil
	case NameVectorExpr:
		return nil, fmt.Errorf("parse: juxtaposed name vectors are not supported")
	default:
		return nil, fmt.Errorf("parse: unsupported expression form %T", expr)
	}
}

func qExprToParseTreeDyad(op string, left, right Expr) (any, error) {
	l, err := qExprToParseTree(left)
	if err != nil {
		return nil, err
	}
	r, err := qExprToParseTree(right)
	if err != nil {
		return nil, err
	}
	return qParseTreeList(data.Symbol(op), l, r), nil
}
