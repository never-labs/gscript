package q

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type Lowered struct {
	Op          QueryKind
	Source      string
	SourcePath  string
	Frame       *data.Frame
	LiteralKeys []data.Symbol
	HiddenCols  []data.Symbol
	Distinct    bool
	Plan        data.QueryPlan
	ExecDict    *ExecDictPlan
	Mutation    *MutationPlan
	Join        *JoinPlan
	Joins       []*JoinPlan
	Original    *Query
}

type ExecDictPlan struct {
	KeyName   data.Symbol
	ValueName data.Symbol
}

type JoinPlan struct {
	Kind       string
	Right      string
	Keys       []data.JoinKey
	WindowLow  any
	WindowHigh any
	HasWindow  bool
}

type MutationPlan struct {
	Kind          QueryKind
	Where         data.Expr
	ByExprs       []data.SelectItem
	Assignments   []MutationAssignment
	DeleteColumns []data.Symbol
	InsertColumns []data.Symbol
	InsertValues  []data.Literal
}

type MutationAssignment struct {
	Name data.Symbol
	Func string
	Expr data.Expr
}

func Lower(query *Query) (*Lowered, error) {
	if query == nil {
		return nil, fmt.Errorf("nil q query")
	}
	if query.From == "" && query.FromExpr == nil {
		return nil, fmt.Errorf("q query missing from source")
	}
	switch query.Kind {
	case SelectQuery, ExecQuery:
		return lowerRead(query)
	case UpdateQuery, DeleteQuery, InsertQuery, UpsertQuery:
		return lowerMutation(query)
	default:
		return nil, fmt.Errorf("unsupported q query kind %q", query.Kind)
	}
}

func lowerRead(query *Query) (*Lowered, error) {
	if len(query.Columns) == 0 && len(query.By) == 0 {
		return nil, fmt.Errorf("q query missing projection")
	}

	plan := data.QueryPlan{LimitN: -1, Distinct: query.Distinct}
	var sourceFrame *data.Frame
	var literalKeys []data.Symbol
	sourcePath := ""
	if query.FromExpr != nil {
		literalKeys = keyedTableLiteralKeys(query.FromExpr)
		if path, ok := qPathSource(query.FromExpr); ok {
			sourcePath = path
		} else {
			frame, err := lowerStaticFrame(query.FromExpr)
			if err != nil {
				return nil, err
			}
			sourceFrame = &frame
			plan.Source = frame
		}
	}
	joins, err := lowerJoins(query)
	if err != nil {
		return nil, err
	}
	var join *JoinPlan
	if len(joins) > 0 {
		join = joins[0]
	}
	windowProjectionAggregates := join != nil && (join.Kind == "window" || join.Kind == "window1") && len(query.By) == 0
	if query.Where != nil {
		filter, err := lowerExpr(query.Where)
		if err != nil {
			return nil, err
		}
		plan.Where = filter
	}
	execDict, err := lowerExecDictProjection(query, &plan)
	if err != nil {
		return nil, err
	}
	if execDict == nil {
		if err := diagnoseDictProjection(query); err != nil {
			return nil, err
		}
	}
	for _, by := range query.By {
		expr, err := lowerExpr(by.Expr)
		if err != nil {
			return nil, err
		}
		name := data.Symbol(by.Name)
		if name == "" {
			name = data.Symbol(columnName(by.Expr))
		}
		if name == "" {
			return nil, fmt.Errorf("q by expression requires an alias")
		}
		plan.ByExprs = append(plan.ByExprs, data.SelectItem{Name: name, Expr: expr})
	}
	if len(query.Columns) == 0 {
		for _, by := range plan.ByExprs {
			plan.Select = append(plan.Select, by)
		}
		plan.ByExprs = nil
		plan.Distinct = true
	}
	if execDict == nil {
		for _, column := range query.Columns {
			if _, ok := column.Expr.(AllColumns); ok {
				if sourceFrame != nil {
					for _, name := range sourceFrame.Schema().Names() {
						plan.Select = append(plan.Select, data.SelectItem{Name: name, Expr: data.ColumnRef{Name: name}})
					}
					continue
				}
				plan.Select = append(plan.Select, data.SelectItem{Name: "*", Expr: data.Literal{Value: AllColumns{}}})
				continue
			}
			if call, ok := column.Expr.(Call); ok && call.Func == "distinct" {
				plan.Distinct = true
				expr, err := lowerExpr(call.Arg)
				if err != nil {
					return nil, err
				}
				name := data.Symbol(column.Name)
				if name == "" {
					name = data.Symbol(columnName(call.Arg))
				}
				if name == "" {
					return nil, fmt.Errorf("q projection requires an alias for computed expressions")
				}
				plan.Select = append(plan.Select, data.SelectItem{Name: name, Expr: expr})
				continue
			}
			if call, ok := column.Expr.(Call); ok && isAggregate(call.Func) {
				name := data.Symbol(column.Name)
				if name == "" {
					name = data.Symbol(call.Func)
				}
				expr, err := lowerExpr(call.Arg)
				if err != nil {
					return nil, err
				}
				if windowProjectionAggregates {
					plan.Select = append(plan.Select, data.SelectItem{Name: name, Expr: data.ListAggregateExpr{Func: call.Func, Expr: expr}})
					continue
				}
				if call.Func == "count" {
					plan.Aggregates = append(plan.Aggregates, data.Aggregate{Name: name, Func: "count"})
				} else {
					plan.Aggregates = append(plan.Aggregates, data.Aggregate{Name: name, Func: call.Func, Expr: expr})
				}
				continue
			}
			expr, err := lowerExpr(column.Expr)
			if err != nil {
				return nil, err
			}
			name := data.Symbol(column.Name)
			if name == "" {
				name = data.Symbol(columnName(column.Expr))
			}
			if name == "" {
				return nil, fmt.Errorf("q projection requires an alias for computed expressions")
			}
			plan.Select = append(plan.Select, data.SelectItem{Name: name, Expr: expr})
		}
	}
	hiddenCols := make([]data.Symbol, 0)
	for _, order := range query.OrderBy {
		name := order.Column
		if name == "" && order.Expr != nil {
			name = columnName(order.Expr)
		}
		if name == "" && order.Expr == nil {
			return nil, fmt.Errorf("q order by requires a column identifier or named expression")
		}
		orderName := data.Symbol(name)
		if _, ok := order.Expr.(Ident); ok && !qOrderColumnAvailable(plan, orderName) {
			plan.PreProjectOrder = true
			plan.OrderBy = append(plan.OrderBy, data.OrderSpec{Column: orderName, Desc: order.Desc})
			continue
		}
		if order.Expr != nil && !qOrderColumnAvailable(plan, orderName) {
			expr, err := lowerExpr(order.Expr)
			if err != nil {
				return nil, err
			}
			if orderName == "" {
				orderName = "expr"
			}
			orderName = qHiddenOrderColumnName(len(hiddenCols), orderName)
			hiddenCols = append(hiddenCols, orderName)
			plan.Select = append(plan.Select, data.SelectItem{Name: orderName, Expr: expr})
		}
		plan.OrderBy = append(plan.OrderBy, data.OrderSpec{Column: orderName, Desc: order.Desc})
	}
	if query.Limit != nil {
		plan.LimitN = *query.Limit
	}
	if query.Take != nil {
		plan.LimitN = *query.Take
	}
	return &Lowered{
		Op:          query.Kind,
		Source:      query.From,
		SourcePath:  sourcePath,
		Frame:       sourceFrame,
		LiteralKeys: literalKeys,
		HiddenCols:  hiddenCols,
		Distinct:    plan.Distinct,
		Plan:        plan,
		ExecDict:    execDict,
		Join:        join,
		Joins:       joins,
		Original:    query,
	}, nil
}

func qOrderColumnAvailable(plan data.QueryPlan, name data.Symbol) bool {
	for _, item := range plan.Select {
		if item.Name == name {
			return true
		}
	}
	for _, item := range plan.ByExprs {
		if item.Name == name {
			return true
		}
	}
	for _, by := range plan.By {
		if by == name {
			return true
		}
	}
	for _, agg := range plan.Aggregates {
		if agg.Name == name {
			return true
		}
	}
	return false
}

func qHiddenOrderColumnName(index int, fallback data.Symbol) data.Symbol {
	if fallback != "" {
		return data.Symbol(fmt.Sprintf("__q_order_%d_%s", index, fallback))
	}
	return data.Symbol(fmt.Sprintf("__q_order_%d", index))
}

func lowerExecDictProjection(query *Query, plan *data.QueryPlan) (*ExecDictPlan, error) {
	if query.Kind != ExecQuery || len(query.Columns) != 1 {
		return nil, nil
	}
	dict, ok := query.Columns[0].Expr.(DictExpr)
	if !ok {
		return nil, nil
	}
	if len(query.By) > 0 {
		return nil, fmt.Errorf("q exec dictionary projection does not support by")
	}
	keyExpr, err := lowerExpr(dict.Keys)
	if err != nil {
		return nil, err
	}
	valueExpr, err := lowerExpr(dict.Values)
	if err != nil {
		return nil, err
	}
	execDict := &ExecDictPlan{
		KeyName:   "__q_exec_dict_key",
		ValueName: "__q_exec_dict_value",
	}
	plan.Select = append(plan.Select,
		data.SelectItem{Name: execDict.KeyName, Expr: keyExpr},
		data.SelectItem{Name: execDict.ValueName, Expr: valueExpr},
	)
	return execDict, nil
}

func diagnoseDictProjection(query *Query) error {
	for _, column := range query.Columns {
		if _, ok := column.Expr.(DictExpr); !ok {
			continue
		}
		if query.Kind == ExecQuery {
			return fmt.Errorf("q exec dictionary projection %q must be the only projection", column.Name)
		}
		return fmt.Errorf("q dictionary projection %q is only valid for exec", column.Name)
	}
	return nil
}

func lowerStaticFrame(expr Expr) (data.Frame, error) {
	flip, ok := expr.(Flip)
	if !ok {
		return data.Frame{}, fmt.Errorf("q from expression must be a table literal")
	}
	if len(flip.Keys) == 0 && len(flip.Columns) == 0 {
		return data.Frame{}, fmt.Errorf("q table literal requires at least one column")
	}
	cols := make([]data.Column, 0, len(flip.Keys)+len(flip.Columns))
	for _, column := range append(append([]Column(nil), flip.Keys...), flip.Columns...) {
		if column.Name == "" {
			return data.Frame{}, fmt.Errorf("q table literal column requires a name")
		}
		value, err := lowerStaticValue(column.Expr)
		if err != nil {
			return data.Frame{}, fmt.Errorf("q table literal column %q: %w", column.Name, err)
		}
		array, ok := value.(data.Array)
		if !ok {
			return data.Frame{}, fmt.Errorf("q table literal column %q must be a vector", column.Name)
		}
		cols = append(cols, data.Column{Name: data.Symbol(column.Name), Data: array})
	}
	return data.NewFrame(cols...)
}

func lowerStaticValue(expr Expr) (any, error) {
	switch x := expr.(type) {
	case Number:
		return lowerLiteralValue(x)
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
	case Vector:
		values := make([]any, len(x.Items))
		temporalKinds := make([]data.Kind, len(x.Items))
		hasTemporal := false
		for i, item := range x.Items {
			v, err := lowerStaticValue(item)
			if err != nil {
				return nil, err
			}
			values[i] = v
			if temporal, ok := item.(Temporal); ok {
				temporalKinds[i] = data.Kind(temporal.Kind)
				hasTemporal = true
				continue
			}
			if typedNull, ok := item.(TypedNull); ok {
				temporalKinds[i] = data.Kind(typedNull.Kind)
				hasTemporal = hasTemporal || isTemporalDataKind(temporalKinds[i])
				continue
			}
			if kind := temporalKindOfValue(v); kind != "" {
				temporalKinds[i] = kind
				hasTemporal = true
			}
		}
		if hasTemporal {
			kind, err := coerceTemporalVectorValues(values, temporalKinds)
			if err != nil {
				return nil, err
			}
			col, err := data.NewColumnWithKind("_", kind, values)
			if err != nil {
				return nil, err
			}
			return col.Data, nil
		}
		return data.NewColumn("_", values).Data, nil
	case Binary:
		if x.Op == "-" {
			if isStaticZeroNumber(x.Left) {
				value, err := lowerStaticValue(x.Right)
				if err != nil {
					return nil, err
				}
				return negateStaticValue(value)
			}
		}
		if x.Op == "+" {
			if isStaticZeroNumber(x.Left) {
				return lowerStaticValue(x.Right)
			}
		}
		left, err := lowerStaticValue(x.Left)
		if err != nil {
			return nil, err
		}
		right, err := lowerStaticValue(x.Right)
		if err != nil {
			return nil, err
		}
		return applyStaticBinary(x.Op, left, right)
	default:
		return nil, fmt.Errorf("unsupported static expression %T", expr)
	}
}

func applyStaticBinary(op string, left, right any) (any, error) {
	leftArray, leftIsArray := left.(data.Array)
	rightArray, rightIsArray := right.(data.Array)
	if leftIsArray || rightIsArray {
		var n int
		switch {
		case leftIsArray && rightIsArray:
			if leftArray.Len() != rightArray.Len() {
				return nil, fmt.Errorf("static %s length mismatch", op)
			}
			n = leftArray.Len()
		case leftIsArray:
			n = leftArray.Len()
		default:
			n = rightArray.Len()
		}
		values := make([]any, n)
		for i := 0; i < n; i++ {
			lv := left
			if leftIsArray {
				v, _ := leftArray.At(i)
				lv = v
			}
			rv := right
			if rightIsArray {
				v, _ := rightArray.At(i)
				rv = v
			}
			value, err := applyStaticScalarBinary(op, lv, rv)
			if err != nil {
				return nil, err
			}
			values[i] = value
		}
		return data.NewColumn("_", values).Data, nil
	}
	return applyStaticScalarBinary(op, left, right)
}

func applyStaticScalarBinary(op string, left, right any) (any, error) {
	li, leftInt := staticInt(left)
	ri, rightInt := staticInt(right)
	if leftInt && rightInt && op != "%" && op != "/" {
		switch op {
		case "+":
			return li + ri, nil
		case "-":
			return li - ri, nil
		case "*":
			return li * ri, nil
		}
	}
	lf, lok := staticFloat(left)
	rf, rok := staticFloat(right)
	if !lok || !rok {
		return nil, fmt.Errorf("static %s expects numeric values", op)
	}
	switch op {
	case "+":
		return lf + rf, nil
	case "-":
		return lf - rf, nil
	case "*":
		return lf * rf, nil
	case "%", "/":
		return lf / rf, nil
	default:
		return nil, fmt.Errorf("unsupported static operator %q", op)
	}
}

func staticInt(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	default:
		return 0, false
	}
}

func staticFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	default:
		return 0, false
	}
}

func isStaticZeroNumber(expr Expr) bool {
	number, ok := expr.(Number)
	if !ok {
		return false
	}
	value, err := strconv.ParseFloat(number.Text, 64)
	return err == nil && value == 0
}

func negateStaticValue(value any) (any, error) {
	switch v := value.(type) {
	case int64:
		return -v, nil
	case int:
		return -v, nil
	case int16:
		return -v, nil
	case int32:
		return -v, nil
	case float64:
		return -v, nil
	case float32:
		return -v, nil
	case data.Array:
		values := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			item, _ := v.At(i)
			negated, err := negateStaticValue(item)
			if err != nil {
				return nil, err
			}
			values[i] = negated
		}
		return data.NewColumn("_", values).Data, nil
	default:
		return nil, fmt.Errorf("static unary - expects numeric value, got %T", value)
	}
}

func lowerJoin(join *Join) (*JoinPlan, error) {
	if join == nil {
		return nil, nil
	}
	kind := normalizeJoinKind(join.Kind)
	switch kind {
	case "inner", "left", "asof", "asof0", "asof_fill", "asof_fill0", "union", "plus", "window", "window1":
	default:
		return nil, fmt.Errorf("q join kind %q is not supported", join.Kind)
	}
	if join.Window != nil && kind != "window" && kind != "window1" {
		return nil, fmt.Errorf("q window bounds are only valid for window joins")
	}
	if join.Right == "" {
		return nil, fmt.Errorf("q join missing right source")
	}
	if len(join.Keys) == 0 {
		return nil, fmt.Errorf("q join requires at least one key")
	}
	keys := make([]data.JoinKey, len(join.Keys))
	for i, key := range join.Keys {
		if key.Left == "" || key.Right == "" {
			return nil, fmt.Errorf("q join key cannot be empty")
		}
		keys[i] = data.JoinKey{Left: data.Symbol(key.Left), Right: data.Symbol(key.Right)}
	}
	plan := &JoinPlan{Kind: kind, Right: join.Right, Keys: keys}
	if join.Window != nil {
		low, err := lowerWindowBound(join.Window.Low)
		if err != nil {
			return nil, fmt.Errorf("q window lower bound: %w", err)
		}
		high, err := lowerWindowBound(join.Window.High)
		if err != nil {
			return nil, fmt.Errorf("q window upper bound: %w", err)
		}
		plan.WindowLow = low
		plan.WindowHigh = high
		plan.HasWindow = true
	}
	return plan, nil
}

func lowerJoins(query *Query) ([]*JoinPlan, error) {
	if query == nil {
		return nil, nil
	}
	joins := query.Joins
	if len(joins) == 0 && query.Join != nil {
		joins = []Join{*query.Join}
	}
	out := make([]*JoinPlan, 0, len(joins))
	for i := range joins {
		plan, err := lowerJoin(&joins[i])
		if err != nil {
			return nil, err
		}
		if plan != nil {
			out = append(out, plan)
		}
	}
	return out, nil
}

func normalizeJoinKind(kind string) string {
	switch kind {
	case "":
		return "inner"
	case "aj":
		return "asof"
	case "aj0":
		return "asof0"
	case "ajf":
		return "asof_fill"
	case "ajf0":
		return "asof_fill0"
	default:
		return kind
	}
}

func lowerWindowBound(expr Expr) (any, error) {
	if binary, ok := expr.(Binary); ok && binary.Op == "-" {
		left, ok := binary.Left.(Number)
		if !ok || left.Text != "0" {
			return nil, fmt.Errorf("literal expected")
		}
		value, err := lowerLiteralValue(binary.Right)
		if err != nil {
			return nil, err
		}
		return negateWindowBound(value)
	}
	return lowerLiteralValue(expr)
}

func negateWindowBound(value any) (any, error) {
	switch x := value.(type) {
	case int64:
		return -x, nil
	case int:
		return -x, nil
	case float64:
		return -x, nil
	case float32:
		return -x, nil
	case int16:
		return -x, nil
	case int32:
		return -x, nil
	case data.Timespan:
		return data.TimespanFromNanos(-x.Nanos()), nil
	default:
		return nil, fmt.Errorf("negative window bound must be numeric or timespan")
	}
}

func lowerMutation(query *Query) (*Lowered, error) {
	if query.Distinct || len(query.OrderBy) > 0 || query.Limit != nil || query.Take != nil {
		return nil, fmt.Errorf("q %s does not support distinct, order, limit, or take", query.Kind)
	}
	if query.Kind != UpdateQuery && len(query.By) > 0 {
		return nil, fmt.Errorf("q %s does not support by", query.Kind)
	}

	var sourceFrame *data.Frame
	var literalKeys []data.Symbol
	sourcePath := ""
	if query.FromExpr != nil {
		literalKeys = keyedTableLiteralKeys(query.FromExpr)
		if path, ok := qPathSource(query.FromExpr); ok {
			sourcePath = path
		} else {
			frame, err := lowerStaticFrame(query.FromExpr)
			if err != nil {
				return nil, err
			}
			sourceFrame = &frame
		}
	}

	mutation := &MutationPlan{Kind: query.Kind}
	if query.Where != nil {
		filter, err := lowerExpr(query.Where)
		if err != nil {
			return nil, err
		}
		mutation.Where = filter
	}
	for _, by := range query.By {
		expr, err := lowerExpr(by.Expr)
		if err != nil {
			return nil, err
		}
		name := data.Symbol(by.Name)
		if name == "" {
			name = data.Symbol(columnName(by.Expr))
		}
		if name == "" {
			return nil, fmt.Errorf("q update by expression requires an alias")
		}
		mutation.ByExprs = append(mutation.ByExprs, data.SelectItem{Name: name, Expr: expr})
	}

	switch query.Kind {
	case InsertQuery, UpsertQuery:
		if len(query.Values) == 0 {
			return nil, fmt.Errorf("q %s requires at least one value", query.Kind)
		}
		for _, column := range query.Columns {
			if column.Name == "" {
				return nil, fmt.Errorf("q %s insert column name must not be empty", query.Kind)
			}
			if _, ok := column.Expr.(Ident); !ok {
				return nil, fmt.Errorf("q %s insert column list must contain column identifiers", query.Kind)
			}
			mutation.InsertColumns = append(mutation.InsertColumns, data.Symbol(column.Name))
		}
		for _, value := range query.Values {
			lit, err := lowerLiteralValue(value)
			if err != nil {
				return nil, fmt.Errorf("q %s values must be literal scalars: %w", query.Kind, err)
			}
			mutation.InsertValues = append(mutation.InsertValues, data.Literal{Value: lit})
		}
	case UpdateQuery:
		if len(query.Columns) == 0 {
			return nil, fmt.Errorf("q update requires at least one assignment")
		}
		for _, column := range query.Columns {
			if column.Name == "" {
				return nil, fmt.Errorf("q update requires a target column name")
			}
			funcName := ""
			exprNode := column.Expr
			if len(mutation.ByExprs) > 0 {
				call, ok := column.Expr.(Call)
				if !ok || !isAggregate(call.Func) {
					return nil, fmt.Errorf("q grouped update assignment %q requires an aggregate expression", column.Name)
				}
				funcName = call.Func
				exprNode = call.Arg
			}
			expr, err := lowerExpr(exprNode)
			if err != nil {
				return nil, err
			}
			mutation.Assignments = append(mutation.Assignments, MutationAssignment{
				Name: data.Symbol(column.Name),
				Func: funcName,
				Expr: expr,
			})
		}
	case DeleteQuery:
		for _, column := range query.Columns {
			ident, ok := column.Expr.(Ident)
			if !ok || column.Name != ident.Name {
				return nil, fmt.Errorf("q delete column list must contain column identifiers")
			}
			mutation.DeleteColumns = append(mutation.DeleteColumns, data.Symbol(ident.Name))
		}
	default:
		return nil, fmt.Errorf("unsupported q mutation kind %q", query.Kind)
	}

	return &Lowered{
		Op:          query.Kind,
		Source:      query.From,
		SourcePath:  sourcePath,
		Frame:       sourceFrame,
		LiteralKeys: literalKeys,
		Mutation:    mutation,
		Original:    query,
	}, nil
}

func qPathSource(expr Expr) (string, bool) {
	sym, ok := expr.(Symbol)
	if !ok || !strings.HasPrefix(sym.Name, ":") {
		return "", false
	}
	path := strings.TrimPrefix(sym.Name, ":")
	if path == "" {
		return "", false
	}
	return path, true
}

func keyedTableLiteralKeys(expr Expr) []data.Symbol {
	flip, ok := expr.(Flip)
	if !ok || len(flip.Keys) == 0 {
		return nil
	}
	keys := make([]data.Symbol, 0, len(flip.Keys))
	for _, column := range flip.Keys {
		if column.Name == "" {
			continue
		}
		keys = append(keys, data.Symbol(column.Name))
	}
	return keys
}

func isAggregate(name string) bool {
	switch name {
	case "sum", "avg", "var", "dev", "med", "wavg", "count", "min", "max", "first", "last":
		return true
	default:
		return false
	}
}

func lowerExpr(expr Expr) (data.Expr, error) {
	switch x := expr.(type) {
	case Ident:
		return data.ColumnRef{Name: data.Symbol(x.Name)}, nil
	case Symbol:
		return data.Literal{Value: data.Symbol(x.Name)}, nil
	case String:
		return data.Literal{Value: x.Value}, nil
	case Bool:
		return data.Literal{Value: x.Value}, nil
	case Null:
		return data.Literal{Value: nil}, nil
	case Temporal:
		v, err := parseQTemporal(x.Kind, x.Text)
		if err != nil {
			return nil, err
		}
		return data.Literal{Value: v}, nil
	case TypedNull:
		return data.Literal{Value: data.NullForKind(data.Kind(x.Kind))}, nil
	case Number:
		v, err := lowerNumberLiteral(x.Text)
		if err != nil {
			return nil, err
		}
		return data.Literal{Value: v}, nil
	case RawLiteral:
		return data.Literal{Value: x.Value}, nil
	case Binary:
		left, err := lowerExpr(x.Left)
		if err != nil {
			return nil, err
		}
		switch qLogicalOp(x.Op) {
		case "and", "or":
			right, err := lowerExpr(x.Right)
			if err != nil {
				return nil, err
			}
			return data.Logical{Op: qLogicalOp(x.Op), Left: left, Right: right}, nil
		case "in":
			values, err := lowerStaticVectorValues(x.Right)
			if err != nil {
				return nil, fmt.Errorf("q in expects a literal vector: %w", err)
			}
			return data.In{Expr: left, Values: values}, nil
		case "within":
			values, err := lowerStaticVectorValues(x.Right)
			if err != nil {
				return nil, fmt.Errorf("q within expects a two-item literal vector: %w", err)
			}
			if len(values) != 2 {
				return nil, fmt.Errorf("q within expects a two-item literal vector")
			}
			return data.Within{Expr: left, Low: values[0], High: values[1], HighClosed: true}, nil
		}
		right, err := lowerExpr(x.Right)
		if err != nil {
			return nil, err
		}
		op, err := lowerOp(x.Op)
		if err != nil {
			return nil, err
		}
		return data.Binary{Op: op, Left: left, Right: right}, nil
	case Call:
		if x.Func == "?" {
			return lowerConditional(x.Arg)
		}
		if strings.EqualFold(x.Func, "xbar") {
			return lowerXbar(x.Arg)
		}
		if strings.EqualFold(x.Func, "not") {
			expr, err := lowerExpr(x.Arg)
			if err != nil {
				return nil, err
			}
			return data.Not{Expr: expr}, nil
		}
		if qVectorPrefixFunction(x.Func) {
			expr, err := lowerExpr(x.Arg)
			if err != nil {
				return nil, err
			}
			return data.VectorTransformExpr{Func: strings.ToLower(x.Func), Expr: expr}, nil
		}
		if qSQLDyadicFunction(x.Func) {
			return lowerTimeSeriesDyadicCall(x.Func, x.Arg)
		}
		return nil, fmt.Errorf("q function %q is not valid in this expression position", x.Func)
	case DictExpr:
		return nil, fmt.Errorf("q dictionary expression is only recognized as an exec projection")
	default:
		return nil, fmt.Errorf("unsupported q expression %T", expr)
	}
}

func qLogicalOp(op string) string {
	switch strings.ToLower(op) {
	case "&":
		return "and"
	case "|":
		return "or"
	default:
		return strings.ToLower(op)
	}
}

func lowerConditional(arg Expr) (data.Expr, error) {
	vector, ok := arg.(Vector)
	if !ok || len(vector.Items) != 3 {
		return nil, fmt.Errorf("q conditional expects three arguments")
	}
	cond, err := lowerExpr(vector.Items[0])
	if err != nil {
		return nil, err
	}
	thenExpr, err := lowerExpr(vector.Items[1])
	if err != nil {
		return nil, err
	}
	elseExpr, err := lowerExpr(vector.Items[2])
	if err != nil {
		return nil, err
	}
	return data.Conditional{Cond: cond, Then: thenExpr, Else: elseExpr}, nil
}

func qVectorPrefixFunction(name string) bool {
	switch strings.ToLower(name) {
	case "prev", "next", "deltas", "fills", "ratios", "sums", "prds", "mins", "maxs", "avgs",
		"abs", "neg", "sqrt", "log", "exp", "sin", "cos", "tan", "asin", "acos", "atan",
		"reciprocal", "signum", "floor", "ceiling", "null", "rank":
		return true
	default:
		return false
	}
}

func lowerTimeSeriesDyadicCall(funcName string, arg Expr) (data.Expr, error) {
	vector, ok := arg.(Vector)
	if !ok || len(vector.Items) != 2 {
		return nil, fmt.Errorf("q dyadic function %q expects left and right arguments", funcName)
	}
	left, err := lowerExpr(vector.Items[0])
	if err != nil {
		return nil, err
	}
	right, err := lowerExpr(vector.Items[1])
	if err != nil {
		return nil, err
	}
	name := strings.ToLower(funcName)
	if name == "xbar" {
		return lowerXbar(arg)
	}
	if name == "xprev" {
		return data.VectorTransformExpr{Func: name, Arg: left, Expr: right}, nil
	}
	agg, ok := movingAggregateFunc(name)
	if !ok {
		return nil, fmt.Errorf("q time-series dyadic function %q is not supported", funcName)
	}
	window := data.VectorTransformExpr{Func: "moving", Arg: left, Expr: right}
	return data.ListAggregateExpr{Func: agg, Expr: window}, nil
}

func movingAggregateFunc(name string) (string, bool) {
	switch name {
	case "msum":
		return "sum", true
	case "mavg":
		return "avg", true
	case "mcount":
		return "count", true
	case "mmin":
		return "min", true
	case "mmax":
		return "max", true
	default:
		return "", false
	}
}

func lowerStaticVectorValues(expr Expr) ([]any, error) {
	switch x := expr.(type) {
	case Vector:
		values := make([]any, len(x.Items))
		for i, item := range x.Items {
			v, err := lowerLiteralValue(item)
			if err != nil {
				return nil, err
			}
			values[i] = v
		}
		return values, nil
	default:
		v, err := lowerLiteralValue(expr)
		if err != nil {
			return nil, err
		}
		return []any{v}, nil
	}
}

func lowerXbar(arg Expr) (data.Expr, error) {
	vector, ok := arg.(Vector)
	if !ok || len(vector.Items) != 2 {
		return nil, fmt.Errorf("q xbar expects an interval and expression")
	}
	interval, err := lowerLiteralValue(vector.Items[0])
	if err != nil {
		return nil, fmt.Errorf("q xbar interval: %w", err)
	}
	expr, err := lowerExpr(vector.Items[1])
	if err != nil {
		return nil, err
	}
	return data.BucketFloorExpr{Expr: expr, Interval: interval}, nil
}

func lowerLiteralValue(expr Expr) (any, error) {
	switch x := expr.(type) {
	case Number:
		return lowerNumberLiteral(x.Text)
	case RawLiteral:
		return x.Value, nil
	case Symbol:
		return data.Symbol(x.Name), nil
	case String:
		return x.Value, nil
	case Bool:
		return x.Value, nil
	case Null:
		return nil, nil
	case Temporal:
		return parseQTemporal(x.Kind, x.Text)
	case TypedNull:
		return data.NullForKind(data.Kind(x.Kind)), nil
	case Binary:
		if x.Op != "-" {
			return nil, fmt.Errorf("literal expected")
		}
		left, ok := x.Left.(Number)
		if !ok || left.Text != "0" {
			return nil, fmt.Errorf("literal expected")
		}
		value, err := lowerLiteralValue(x.Right)
		if err != nil {
			return nil, err
		}
		return negateWindowBound(value)
	default:
		return nil, fmt.Errorf("literal expected")
	}
}

func lowerNumberLiteral(text string) (any, error) {
	if v, _, ok := parseQInfinity(text); ok {
		return v, nil
	}
	switch strings.ToLower(text) {
	case "0n":
		return data.NullForKind(data.KindF64), nil
	case "0nb":
		return data.NullForKind(data.KindBool), nil
	case "0nx":
		return data.NullForKind(data.KindU8), nil
	case "0nc":
		return data.NullForKind(data.KindString), nil
	case "0nh":
		return data.NullForKind(data.KindI16), nil
	case "0ni":
		return data.NullForKind(data.KindI32), nil
	case "0nj":
		return data.NullForKind(data.KindI64), nil
	case "0ne":
		return data.NullForKind(data.KindF32), nil
	case "0nf":
		return data.NullForKind(data.KindF64), nil
	}
	if len(text) > 1 {
		suffix := text[len(text)-1]
		body := text[:len(text)-1]
		switch suffix {
		case 'h':
			i, err := strconv.ParseInt(body, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid q short literal %q", text)
			}
			return int16(i), nil
		case 'i':
			i, err := strconv.ParseInt(body, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid q int literal %q", text)
			}
			return int32(i), nil
		case 'j':
			i, err := strconv.ParseInt(body, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid q long literal %q", text)
			}
			return i, nil
		case 'e':
			f, err := strconv.ParseFloat(body, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid q real literal %q", text)
			}
			return float32(f), nil
		case 'f':
			f, err := strconv.ParseFloat(body, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid q float literal %q", text)
			}
			return f, nil
		}
	}
	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return i, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid q number %q", text)
	}
	return f, nil
}

func lowerOp(op string) (data.Op, error) {
	switch op {
	case "+":
		return data.OpAdd, nil
	case "-":
		return data.OpSub, nil
	case "*":
		return data.OpMul, nil
	case "/", "%":
		return data.OpDiv, nil
	case "=":
		return data.OpEQ, nil
	case "!=", "<>":
		return data.OpNE, nil
	case "<":
		return data.OpLT, nil
	case "<=":
		return data.OpLE, nil
	case ">":
		return data.OpGT, nil
	case ">=":
		return data.OpGE, nil
	default:
		return "", fmt.Errorf("unsupported q operator %q", op)
	}
}
