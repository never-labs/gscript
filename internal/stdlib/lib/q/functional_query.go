package q

// Functional qSQL forms: ?[t;c;b;a] (select/exec) and ![t;c;b;a]
// (update/delete). The four argument values are translated into the same
// Query AST the qSQL parser produces and executed wholesale through the
// existing Lower / ExecRuntime / mutation machinery.
//
// Canonical alignment and documented deviations:
//
//   - Constraints (c) are lists of parse trees whose head is an operator
//     ATOM, exactly the canonical q spelling: (>;`price;100) evaluates to a
//     general list holding the > function atom, the symbol `price, and 100,
//     because bare operator names evaluate to callable atoms in this dialect
//     too. Symbol operands are column references; enlisted symbol lists are
//     symbol literals — both per canonical parse-tree semantics.
//   - Char-vector literals are indistinguishable from symbol atoms in this
//     dialect (both evaluate to Go strings), so a string operand always
//     resolves as a column reference. Documented deviation.
//   - The canonical empty-symbol-list spelling 0#` does not parse here; use
//     the equivalent canonical spelling `$() (or a bare () list) for the
//     delete-rows form ![t;c;0b;`$()].
//   - b=() selects the exec form, b=0b the select form, b=1b distinct, and a
//     by-dict grouped select, per canonical q. Exec with groupings is not
//     bridged (error).
//   - Lambdas, projections, compositions, and adverb-derived callables in
//     parse-tree head position are not bridgeable into the columnar Lower
//     machinery and report an error.

import (
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// RawLiteral is an AST literal carrying an already-evaluated q value. Only
// the functional-form bridge produces it: functional arguments arrive as
// runtime values, so re-encoding them as source-text literals would be lossy.
type RawLiteral struct {
	Value any
}

func (RawLiteral) exprNode() {}

// functionalQuerySource names the lowered source of functional-form queries;
// the source frame is always supplied to the runtime directly.
const functionalQuerySource = "[functional]"

// qFunctionalTable is a resolved first argument of a functional form: a
// table value, a keyed table value, or a workspace name bound to either.
type qFunctionalTable struct {
	frame    data.Frame
	keyed    data.KeyedFrame
	hasKeyed bool
	// name is non-empty when the table was passed by symbol name; the `!`
	// form then writes the mutated table back to the workspace.
	name string
}

func qFunctionalSymbolName(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case data.Symbol:
		return string(x), true
	default:
		return "", false
	}
}

func (s *EvalState) qFunctionalTableArg(v any) (qFunctionalTable, bool) {
	switch x := v.(type) {
	case data.Frame:
		return qFunctionalTable{frame: x}, true
	case data.KeyedFrame:
		return qFunctionalTable{frame: x.Frame(), keyed: x, hasKeyed: true}, true
	}
	name, ok := qFunctionalSymbolName(v)
	if !ok {
		return qFunctionalTable{}, false
	}
	bound, found := s.lookupName(name)
	if !found {
		return qFunctionalTable{}, false
	}
	if _, nested := qFunctionalSymbolName(bound); nested {
		return qFunctionalTable{}, false
	}
	table, ok := s.qFunctionalTableArg(bound)
	if !ok {
		return qFunctionalTable{}, false
	}
	table.name = name
	return table, true
}

// ---------------------------------------------------------------------------
// Value -> parser AST translation.
// ---------------------------------------------------------------------------

// qFunctionalBinaryOpName reports whether a parse-tree head with this verb
// name lowers as a Binary node (the spelling the qSQL parser produces for
// infix operators); other dyadic verbs lower as Call{Func, Vector} instead.
func qFunctionalBinaryOpName(name string) bool {
	switch name {
	case "+", "-", "*", "%", "=", "<>", "<", "<=", ">", ">=",
		"&", "|", "and", "or", "in", "within", "like", "~":
		return true
	default:
		return false
	}
}

// qFunctionalExprFromValue translates one evaluated functional-form value in
// expression position into the parser AST. Symbol atoms are column
// references; lists are parse trees when their head is a function atom and
// literal vectors otherwise.
func qFunctionalExprFromValue(v any) (Expr, error) {
	switch x := v.(type) {
	case data.Array:
		return qFunctionalListExpr(x)
	case string:
		return Ident{Name: x}, nil
	case data.Symbol:
		return Ident{Name: string(x)}, nil
	case bool:
		return Bool{Value: x}, nil
	case nil, data.Null:
		return Null{}, nil
	case EvalDict:
		return nil, fmt.Errorf("q functional expression does not accept a dictionary operand")
	}
	if isCallable(v) {
		return nil, fmt.Errorf("q functional expression cannot bridge callable operand %T; use operator-atom parse trees like (>;`price;100)", v)
	}
	return RawLiteral{Value: v}, nil
}

func qFunctionalListExpr(list data.Array) (Expr, error) {
	n := list.Len()
	if n == 0 {
		return nil, fmt.Errorf("q functional expression list must not be empty")
	}
	head, _ := list.At(0)
	switch fn := head.(type) {
	case qDyadicFunction:
		if n == 2 && isAggregate(fn.name) {
			// Dual unary/dyadic verbs (min/max) evaluate to dyadic atoms in
			// this dialect; their one-operand parse-tree spelling is the
			// unary aggregate ((max;`v) is the column maximum).
			argValue, _ := list.At(1)
			arg, err := qFunctionalExprFromValue(argValue)
			if err != nil {
				return nil, err
			}
			return Call{Func: fn.name, Arg: arg}, nil
		}
		if n != 3 {
			return nil, fmt.Errorf("q functional parse tree (%s;...) expects two operands, got %d", fn.name, n-1)
		}
		leftValue, _ := list.At(1)
		rightValue, _ := list.At(2)
		left, err := qFunctionalExprFromValue(leftValue)
		if err != nil {
			return nil, err
		}
		right, err := qFunctionalExprFromValue(rightValue)
		if err != nil {
			return nil, err
		}
		if qFunctionalBinaryOpName(fn.name) {
			return Binary{Op: fn.name, Left: left, Right: right}, nil
		}
		return Call{Func: fn.name, Arg: Vector{Items: []Expr{left, right}}}, nil
	case qUnaryFunction:
		if n != 2 {
			return nil, fmt.Errorf("q functional parse tree (%s;...) expects one operand, got %d", fn.name, n-1)
		}
		argValue, _ := list.At(1)
		if fn.name == "enlist" {
			// (enlist;`x) is the canonical parse-tree spelling of a literal:
			// the enlisted value, not a column reference.
			return qFunctionalEnlistLiteral(argValue)
		}
		arg, err := qFunctionalExprFromValue(argValue)
		if err != nil {
			return nil, err
		}
		return Call{Func: fn.name, Arg: arg}, nil
	}
	if isCallable(head) {
		return nil, fmt.Errorf("q functional parse tree head %T is not bridgeable; only operator/verb atoms are supported", head)
	}
	return qFunctionalLiteralListExpr(list)
}

// qFunctionalEnlistLiteral translates the operand of an (enlist;x) parse
// tree: a symbol becomes a symbol literal atom, anything else a one-item
// literal vector.
func qFunctionalEnlistLiteral(v any) (Expr, error) {
	if name, ok := qFunctionalSymbolName(v); ok {
		return RawLiteral{Value: data.Symbol(name)}, nil
	}
	item, err := qFunctionalScalarLiteralExpr(v)
	if err != nil {
		return nil, err
	}
	return Vector{Items: []Expr{item}}, nil
}

// qFunctionalLiteralListExpr translates a non-tree list operand as a literal:
// a one-item symbol list is the canonical enlisted symbol literal atom, any
// other list is a literal vector.
func qFunctionalLiteralListExpr(list data.Array) (Expr, error) {
	n := list.Len()
	if n == 1 {
		item, _ := list.At(0)
		if name, ok := qFunctionalSymbolName(item); ok {
			return RawLiteral{Value: data.Symbol(name)}, nil
		}
	}
	items := make([]Expr, n)
	for i := 0; i < n; i++ {
		item, _ := list.At(i)
		expr, err := qFunctionalScalarLiteralExpr(item)
		if err != nil {
			return nil, err
		}
		items[i] = expr
	}
	return Vector{Items: items}, nil
}

func qFunctionalScalarLiteralExpr(v any) (Expr, error) {
	if name, ok := qFunctionalSymbolName(v); ok {
		return RawLiteral{Value: data.Symbol(name)}, nil
	}
	switch v.(type) {
	case data.Array, EvalDict:
		return nil, fmt.Errorf("q functional literal vector items must be atoms")
	}
	if isCallable(v) {
		return nil, fmt.Errorf("q functional literal vector items must be atoms")
	}
	return RawLiteral{Value: v}, nil
}

// ---------------------------------------------------------------------------
// Constraint (c), grouping (b), and projection (a) specs.
// ---------------------------------------------------------------------------

// qFunctionalWhereExpr translates the constraint spec: a list of parse
// trees, conjoined left-to-right like chained qSQL where clauses. A single
// bare parse tree and the no-constraint spellings () and 0b are accepted.
func qFunctionalWhereExpr(cv any) (Expr, error) {
	switch x := cv.(type) {
	case nil, data.Null:
		return nil, nil
	case bool:
		if !x {
			return nil, nil
		}
		return nil, fmt.Errorf("q functional constraint spec must be a list of parse trees, got 1b")
	case data.Array:
		if x.Len() == 0 {
			return nil, nil
		}
		if head, ok := x.At(0); ok && isCallable(head) {
			return qFunctionalListExpr(x)
		}
		var where Expr
		for i := 0; i < x.Len(); i++ {
			item, _ := x.At(i)
			tree, ok := item.(data.Array)
			if !ok {
				return nil, fmt.Errorf("q functional constraint %d must be a parse tree list, got %T", i, item)
			}
			expr, err := qFunctionalListExpr(tree)
			if err != nil {
				return nil, err
			}
			if where == nil {
				where = expr
				continue
			}
			where = Binary{Op: "and", Left: where, Right: expr}
		}
		return where, nil
	default:
		return nil, fmt.Errorf("q functional constraint spec must be a list of parse trees, got %T", cv)
	}
}

// qFunctionalBySpec translates the grouping spec: 0b for none, 1b for
// distinct, () for the exec form, or a dict of groupings.
func qFunctionalBySpec(bv any) (by []Column, distinct bool, execForm bool, err error) {
	switch x := bv.(type) {
	case nil, data.Null:
		return nil, false, false, nil
	case bool:
		return nil, x, false, nil
	case data.Array:
		if x.Len() == 0 {
			return nil, false, true, nil
		}
		return nil, false, false, fmt.Errorf("q functional by spec must be 0b, 1b, (), or a dict of groupings")
	case EvalDict:
		for i, key := range x.Keys {
			name, ok := qFunctionalSymbolName(key)
			if !ok || name == "" {
				return nil, false, false, fmt.Errorf("q functional by spec keys must be symbols")
			}
			expr, exprErr := qFunctionalExprFromValue(x.Values[i])
			if exprErr != nil {
				return nil, false, false, exprErr
			}
			by = append(by, Column{Name: name, Expr: expr})
		}
		return by, false, false, nil
	default:
		return nil, false, false, fmt.Errorf("q functional by spec must be 0b, 1b, (), or a dict of groupings, got %T", bv)
	}
}

// qFunctionalProjection is a translated a-spec for the ? form.
type qFunctionalProjection struct {
	columns []Column
	// aggregate marks columns whose top-level expression is an aggregate
	// call; the exec form returns those as atoms.
	aggregate []bool
	dict      bool
	selectAll bool
}

func (p qFunctionalProjection) allAggregates() bool {
	if p.selectAll || len(p.aggregate) == 0 {
		return false
	}
	for _, agg := range p.aggregate {
		if !agg {
			return false
		}
	}
	return true
}

func qFunctionalTopAggregate(v any) bool {
	list, ok := v.(data.Array)
	if !ok {
		return false
	}
	head, _ := list.At(0)
	switch fn := head.(type) {
	case qUnaryFunction:
		return list.Len() == 2 && isAggregate(fn.name)
	case qDyadicFunction:
		// Dual unary/dyadic aggregates spelled unary ((max;`v)) and the
		// weighted aggregate (wavg;`w;`v).
		if list.Len() == 2 {
			return isAggregate(fn.name)
		}
		return list.Len() == 3 && fn.name == "wavg"
	default:
		return false
	}
}

func qFunctionalProjectionSpec(av any) (qFunctionalProjection, error) {
	switch x := av.(type) {
	case EvalDict:
		proj := qFunctionalProjection{dict: true}
		for i, key := range x.Keys {
			name, ok := qFunctionalSymbolName(key)
			if !ok || name == "" {
				return qFunctionalProjection{}, fmt.Errorf("q functional projection dict keys must be symbols")
			}
			expr, err := qFunctionalExprFromValue(x.Values[i])
			if err != nil {
				return qFunctionalProjection{}, err
			}
			proj.columns = append(proj.columns, Column{Name: name, Expr: expr})
			proj.aggregate = append(proj.aggregate, qFunctionalTopAggregate(x.Values[i]))
		}
		return proj, nil
	case string:
		return qFunctionalSingleColumn(x)
	case data.Symbol:
		return qFunctionalSingleColumn(string(x))
	case data.Array:
		if x.Len() == 0 {
			return qFunctionalProjection{selectAll: true}, nil
		}
		if head, ok := x.At(0); ok && isCallable(head) {
			expr, err := qFunctionalListExpr(x)
			if err != nil {
				return qFunctionalProjection{}, err
			}
			name := columnName(expr)
			if name == "" {
				name = "x"
			}
			return qFunctionalProjection{
				columns:   []Column{{Name: name, Expr: expr}},
				aggregate: []bool{qFunctionalTopAggregate(av)},
			}, nil
		}
		// A bare symbol list projects the named columns (lenient spelling of
		// the canonical one-entry-per-column dict).
		proj := qFunctionalProjection{}
		for i := 0; i < x.Len(); i++ {
			item, _ := x.At(i)
			name, ok := qFunctionalSymbolName(item)
			if !ok || name == "" {
				return qFunctionalProjection{}, fmt.Errorf("q functional projection spec must be a dict, symbol, parse tree, or ()")
			}
			proj.columns = append(proj.columns, Column{Name: name, Expr: Ident{Name: name}})
			proj.aggregate = append(proj.aggregate, false)
		}
		return proj, nil
	default:
		return qFunctionalProjection{}, fmt.Errorf("q functional projection spec must be a dict, symbol, parse tree, or (), got %T", av)
	}
}

func qFunctionalSingleColumn(name string) (qFunctionalProjection, error) {
	if name == "" {
		return qFunctionalProjection{}, fmt.Errorf("q functional projection column symbol must not be empty")
	}
	return qFunctionalProjection{
		columns:   []Column{{Name: name, Expr: Ident{Name: name}}},
		aggregate: []bool{false},
	}, nil
}

// ---------------------------------------------------------------------------
// ?[t;c;b;a] — functional select / exec.
// ---------------------------------------------------------------------------

// evalFunctionalQueryForm evaluates ?[t;c;b;a] and ?[t;c;b;a;n]. The caller
// (evalConditionalSpecialForm) routes every 4+-argument ?[...] here.
func (s *EvalState) evalFunctionalQueryForm(args []string) (any, error) {
	if len(args) > 5 {
		return nil, fmt.Errorf("?[t;c;b;a] expects four or five arguments, got %d", len(args))
	}
	tv, err := s.eval(args[0])
	if err != nil {
		return nil, err
	}
	table, ok := s.qFunctionalTableArg(tv)
	if !ok {
		return nil, fmt.Errorf("?[t;c;b;a] expects a table or table name as first argument")
	}
	cv, err := s.eval(args[1])
	if err != nil {
		return nil, err
	}
	where, err := qFunctionalWhereExpr(cv)
	if err != nil {
		return nil, err
	}
	bv, err := s.eval(args[2])
	if err != nil {
		return nil, err
	}
	by, distinct, execForm, err := qFunctionalBySpec(bv)
	if err != nil {
		return nil, err
	}
	av, err := s.eval(args[3])
	if err != nil {
		return nil, err
	}
	proj, err := qFunctionalProjectionSpec(av)
	if err != nil {
		return nil, err
	}

	query := &Query{Kind: SelectQuery, From: functionalQuerySource, Distinct: distinct, Where: where, By: by}
	if execForm {
		query.Kind = ExecQuery
		if proj.selectAll {
			return nil, fmt.Errorf("q functional exec requires a projection (a must not be ())")
		}
		if !proj.dict && len(proj.columns) != 1 {
			return nil, fmt.Errorf("q functional exec projection must be a dict, symbol, or parse tree")
		}
	}
	if len(args) == 5 {
		nv, evalErr := s.eval(args[4])
		if evalErr != nil {
			return nil, evalErr
		}
		n, isInt := integerValue(nv)
		if !isInt || n < 0 || int64(int(n)) != n {
			return nil, fmt.Errorf("?[t;c;b;a;n] row limit must be a non-negative integer")
		}
		limit := int(n)
		query.Take = &limit
	}
	if proj.selectAll {
		if len(by) == 0 {
			for _, name := range table.frame.Schema().Names() {
				query.Columns = append(query.Columns, Column{Name: string(name), Expr: Ident{Name: string(name)}})
			}
		}
		// With groupings and a=(), Columns stays empty: the lowered plan is
		// the grouped projection shape qSQL produces for `select by x from t`.
	} else {
		query.Columns = proj.columns
	}
	// Whole-table aggregates (no by) lower to the columnar ungrouped
	// aggregate path: a one-row frame whose atoms qFunctionalExecResult
	// extracts for the exec form.

	lowered, err := Lower(query)
	if err != nil {
		return nil, err
	}
	out, err := lowered.ExecRuntime(table.frame)
	if err != nil {
		return nil, err
	}
	if execForm {
		return qFunctionalExecResult(out, proj)
	}
	if len(by) > 0 {
		keys := make([]data.Symbol, len(by))
		for i, column := range by {
			keys[i] = data.Symbol(column.Name)
		}
		keyed, keyErr := data.KeyBy(out, keys...)
		if keyErr != nil {
			return nil, keyErr
		}
		return keyed, nil
	}
	if table.hasKeyed {
		if keyed, ok := qFunctionalRekey(out, table.keyed.Keys()); ok {
			return keyed, nil
		}
	}
	return out, nil
}

// qFunctionalRekey restores the source key columns on a select result when
// every key column survived the projection.
func qFunctionalRekey(out data.Frame, keys []data.Symbol) (data.KeyedFrame, bool) {
	for _, key := range keys {
		if _, ok := out.Column(key); !ok {
			return data.KeyedFrame{}, false
		}
	}
	keyed, err := data.KeyBy(out, keys...)
	if err != nil {
		return data.KeyedFrame{}, false
	}
	return keyed, true
}

// qFunctionalExecResult shapes an exec-form result: a single projection
// yields the column (an atom for aggregates), a dict projection yields a
// dict of columns.
func qFunctionalExecResult(out data.Frame, proj qFunctionalProjection) (any, error) {
	column := func(i int) (any, error) {
		arr, ok := out.Column(data.Symbol(proj.columns[i].Name))
		if !ok {
			return nil, fmt.Errorf("q functional exec result is missing column %q", proj.columns[i].Name)
		}
		if proj.aggregate[i] && arr.Len() == 1 {
			v, _ := arr.At(0)
			return v, nil
		}
		return arr, nil
	}
	if !proj.dict {
		return column(0)
	}
	result := EvalDict{
		Keys:   make([]any, len(proj.columns)),
		Values: make([]any, len(proj.columns)),
	}
	for i := range proj.columns {
		value, err := column(i)
		if err != nil {
			return nil, err
		}
		result.Keys[i] = data.Symbol(proj.columns[i].Name)
		result.Values[i] = value
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// ![t;c;b;a] — functional update / delete.
// ---------------------------------------------------------------------------

// evalFunctionalAmendForm claims ![t;c;b;a] when the first argument resolves
// to a table, keyed table, or workspace name bound to one. Non-table shapes
// fall through to the pre-existing cascade.
func (s *EvalState) evalFunctionalAmendForm(src string) (any, bool, error) {
	src = strings.TrimSpace(src)
	if len(src) < 4 || src[0] != '!' || src[1] != '[' || src[len(src)-1] != ']' {
		return nil, false, nil
	}
	if !enclosed(src[1:], '[', ']') {
		return nil, false, nil
	}
	args := splitQBracketFormArgs(src[2 : len(src)-1])
	if len(args) != 4 {
		return nil, false, nil
	}
	tv, err := s.eval(args[0])
	if err != nil {
		return nil, false, nil
	}
	table, ok := s.qFunctionalTableArg(tv)
	if !ok {
		if name, isName := qFunctionalSymbolName(tv); isName {
			return nil, true, fmt.Errorf("![t;c;b;a] table name `%s is not bound to a table", name)
		}
		return nil, false, nil
	}
	out, err := s.evalFunctionalAmend(table, args)
	return out, true, err
}

func (s *EvalState) evalFunctionalAmend(table qFunctionalTable, args []string) (any, error) {
	cv, err := s.eval(args[1])
	if err != nil {
		return nil, err
	}
	where, err := qFunctionalWhereExpr(cv)
	if err != nil {
		return nil, err
	}
	bv, err := s.eval(args[2])
	if err != nil {
		return nil, err
	}
	by, distinct, _, err := qFunctionalBySpec(bv)
	if err != nil {
		return nil, err
	}
	if distinct {
		return nil, fmt.Errorf("![t;c;b;a] by spec must be 0b, (), or a dict of groupings")
	}
	av, err := s.eval(args[3])
	if err != nil {
		return nil, err
	}

	query := &Query{From: functionalQuerySource, Where: where, By: by}
	switch x := av.(type) {
	case EvalDict:
		query.Kind = UpdateQuery
		for i, key := range x.Keys {
			name, ok := qFunctionalSymbolName(key)
			if !ok || name == "" {
				return nil, fmt.Errorf("q functional update assignment keys must be symbols")
			}
			expr, exprErr := qFunctionalExprFromValue(x.Values[i])
			if exprErr != nil {
				return nil, exprErr
			}
			query.Columns = append(query.Columns, Column{Name: name, Expr: expr})
		}
	case data.Array:
		query.Kind = DeleteQuery
		if x.Len() > 0 && where != nil {
			return nil, fmt.Errorf("q functional delete cannot specify both row constraints and columns")
		}
		for i := 0; i < x.Len(); i++ {
			item, _ := x.At(i)
			name, ok := qFunctionalSymbolName(item)
			if !ok || name == "" {
				return nil, fmt.Errorf("q functional delete columns must be symbols")
			}
			query.Columns = append(query.Columns, Column{Name: name, Expr: Ident{Name: name}})
		}
	default:
		return nil, fmt.Errorf("![t;c;b;a] expects a dict of assignments or a list of column symbols, got %T", av)
	}

	lowered, err := Lower(query)
	if err != nil {
		return nil, err
	}
	var result any
	if table.hasKeyed {
		keyed, mutErr := qFunctionalRunKeyedMutation(table.keyed, lowered.Mutation)
		if mutErr != nil {
			return nil, mutErr
		}
		result = keyed
	} else {
		frame, mutErr := qFunctionalRunMutation(table.frame, lowered.Mutation)
		if mutErr != nil {
			return nil, mutErr
		}
		result = frame
	}
	if table.name != "" {
		s.env[s.resolveAssignmentName(table.name)] = result
		// Canonical amend-by-name returns the table name symbol.
		return data.Symbol(table.name), nil
	}
	return result, nil
}

// qFunctionalRunMutation executes a lowered update/delete plan against a
// frame (mirrors the bind layer's qRunSQLMutation for the functional kinds).
func qFunctionalRunMutation(frame data.Frame, mutation *MutationPlan) (data.Frame, error) {
	if mutation == nil {
		return data.Frame{}, fmt.Errorf("q functional mutation plan is nil")
	}
	switch mutation.Kind {
	case UpdateQuery:
		if len(mutation.ByExprs) > 0 {
			assignments := make([]data.GroupedAssignment, len(mutation.Assignments))
			for i, assign := range mutation.Assignments {
				assignments[i] = data.GroupedAssignment{Name: assign.Name, Func: assign.Func, Expr: assign.Expr, Weight: assign.Weight}
			}
			return data.UpdateBy(frame, mutation.Where, mutation.ByExprs, assignments)
		}
		assignments := make(map[data.Symbol]data.Expr, len(mutation.Assignments))
		for _, assign := range mutation.Assignments {
			assignments[assign.Name] = assign.Expr
		}
		return data.UpdateWhere(frame, mutation.Where, assignments)
	case DeleteQuery:
		if len(mutation.DeleteColumns) > 0 {
			return data.DropColumns(frame, mutation.DeleteColumns...)
		}
		return data.DeleteWhere(frame, mutation.Where)
	default:
		return data.Frame{}, fmt.Errorf("unsupported q functional mutation kind %q", mutation.Kind)
	}
}

// qFunctionalRunKeyedMutation executes a lowered update/delete plan against
// a keyed table, preserving the key columns.
func qFunctionalRunKeyedMutation(keyed data.KeyedFrame, mutation *MutationPlan) (data.KeyedFrame, error) {
	if mutation == nil {
		return data.KeyedFrame{}, fmt.Errorf("q functional mutation plan is nil")
	}
	switch mutation.Kind {
	case UpdateQuery:
		if len(mutation.ByExprs) > 0 {
			assignments := make([]data.GroupedAssignment, len(mutation.Assignments))
			for i, assign := range mutation.Assignments {
				assignments[i] = data.GroupedAssignment{Name: assign.Name, Func: assign.Func, Expr: assign.Expr, Weight: assign.Weight}
			}
			return data.UpdateByKeyed(keyed, mutation.Where, mutation.ByExprs, assignments)
		}
		assignments := make(map[data.Symbol]data.Expr, len(mutation.Assignments))
		for _, assign := range mutation.Assignments {
			assignments[assign.Name] = assign.Expr
		}
		return data.UpdateWhereKeyed(keyed, mutation.Where, assignments)
	case DeleteQuery:
		if len(mutation.DeleteColumns) == 0 {
			return data.DeleteWhereKeyed(keyed, mutation.Where)
		}
		frame, err := qFunctionalRunMutation(keyed.Frame(), mutation)
		if err != nil {
			return data.KeyedFrame{}, err
		}
		return data.KeyBy(frame, keyed.Keys()...)
	default:
		return data.KeyedFrame{}, fmt.Errorf("unsupported q functional mutation kind %q", mutation.Kind)
	}
}
