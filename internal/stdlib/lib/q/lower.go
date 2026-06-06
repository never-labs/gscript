package q

import (
	"fmt"
	"strconv"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type Lowered struct {
	Op       QueryKind
	Source   string
	Distinct bool
	Plan     data.QueryPlan
	Mutation *MutationPlan
	Original *Query
}

type MutationPlan struct {
	Kind          QueryKind
	Where         data.Expr
	Assignments   []MutationAssignment
	DeleteColumns []data.Symbol
}

type MutationAssignment struct {
	Name data.Symbol
	Expr data.Expr
}

func Lower(query *Query) (*Lowered, error) {
	if query == nil {
		return nil, fmt.Errorf("nil q query")
	}
	if query.From == "" {
		return nil, fmt.Errorf("q query missing from source")
	}
	switch query.Kind {
	case SelectQuery, ExecQuery:
		return lowerRead(query)
	case UpdateQuery, DeleteQuery:
		return lowerMutation(query)
	default:
		return nil, fmt.Errorf("unsupported q query kind %q", query.Kind)
	}
}

func lowerRead(query *Query) (*Lowered, error) {
	if len(query.Columns) == 0 {
		return nil, fmt.Errorf("q query missing projection")
	}

	plan := data.QueryPlan{LimitN: -1, Distinct: query.Distinct}
	if query.Where != nil {
		filter, err := lowerExpr(query.Where)
		if err != nil {
			return nil, err
		}
		plan.Where = filter
	}
	for _, by := range query.By {
		ident, ok := by.(Ident)
		if !ok {
			return nil, fmt.Errorf("q by expression must be a column identifier")
		}
		plan.By = append(plan.By, data.Symbol(ident.Name))
	}
	for _, column := range query.Columns {
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
			return nil, fmt.Errorf("q projection requires an alias for computed expressions")
		}
		plan.Select = append(plan.Select, data.SelectItem{Name: name, Expr: expr})
	}
	for _, order := range query.OrderBy {
		if order.Column == "" {
			return nil, fmt.Errorf("q order by requires a column identifier")
		}
		plan.OrderBy = append(plan.OrderBy, data.OrderSpec{Column: data.Symbol(order.Column), Desc: order.Desc})
	}
	if query.Limit != nil {
		plan.LimitN = *query.Limit
	}
	if query.Take != nil {
		plan.LimitN = *query.Take
	}
	return &Lowered{
		Op:       query.Kind,
		Source:   query.From,
		Distinct: plan.Distinct,
		Plan:     plan,
		Original: query,
	}, nil
}

func lowerMutation(query *Query) (*Lowered, error) {
	if query.Distinct || len(query.By) > 0 || len(query.OrderBy) > 0 || query.Limit != nil || query.Take != nil {
		return nil, fmt.Errorf("q %s does not support distinct, by, order, limit, or take", query.Kind)
	}

	mutation := &MutationPlan{Kind: query.Kind}
	if query.Where != nil {
		filter, err := lowerExpr(query.Where)
		if err != nil {
			return nil, err
		}
		mutation.Where = filter
	}

	switch query.Kind {
	case UpdateQuery:
		if len(query.Columns) == 0 {
			return nil, fmt.Errorf("q update requires at least one assignment")
		}
		for _, column := range query.Columns {
			if column.Name == "" {
				return nil, fmt.Errorf("q update requires a target column name")
			}
			expr, err := lowerExpr(column.Expr)
			if err != nil {
				return nil, err
			}
			mutation.Assignments = append(mutation.Assignments, MutationAssignment{
				Name: data.Symbol(column.Name),
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
		Op:       query.Kind,
		Source:   query.From,
		Mutation: mutation,
		Original: query,
	}, nil
}

func isAggregate(name string) bool {
	switch name {
	case "sum", "avg", "count":
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
	case Number:
		if i, err := strconv.ParseInt(x.Text, 10, 64); err == nil {
			return data.Literal{Value: i}, nil
		}
		f, err := strconv.ParseFloat(x.Text, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid q number %q", x.Text)
		}
		return data.Literal{Value: f}, nil
	case Binary:
		left, err := lowerExpr(x.Left)
		if err != nil {
			return nil, err
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
		return nil, fmt.Errorf("q function %q is not valid in this expression position", x.Func)
	default:
		return nil, fmt.Errorf("unsupported q expression %T", expr)
	}
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
