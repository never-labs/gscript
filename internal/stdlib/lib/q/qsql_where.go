package q

// qSQL where-clause lowering extensions:
//
//   - whereAggregate marks aggregate calls inside a where clause so they lower
//     to data.ScalarAggregateExpr (canonical `where v=max v`: the aggregate is
//     computed over the whole column, then rows are filtered).
//   - qLikeExpr implements the q `like` glob match as a row predicate the
//     data engine evaluates through its generic where fallback.
//   - rewriteWhereVirtualColumns maps the virtual column i to the row index.

import (
	"fmt"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// whereAggregate is an internal AST wrapper produced by
// rewriteWhereAggregates; lowerExpr translates it to data.ScalarAggregateExpr.
type whereAggregate struct {
	Func string
	Arg  Expr
}

func (whereAggregate) exprNode() {}

// rewriteWhereAggregates rewrites aggregate calls inside a where clause into
// whereAggregate markers so lowering produces whole-column scalar aggregates
// instead of rejecting the call position.
func rewriteWhereAggregates(expr Expr) Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case Call:
		if isAggregate(x.Func) {
			return whereAggregate{Func: x.Func, Arg: rewriteWhereAggregates(x.Arg)}
		}
		x.Arg = rewriteWhereAggregates(x.Arg)
		return x
	case Binary:
		x.Left = rewriteWhereAggregates(x.Left)
		x.Right = rewriteWhereAggregates(x.Right)
		return x
	case Vector:
		items := make([]Expr, len(x.Items))
		for i, item := range x.Items {
			items[i] = rewriteWhereAggregates(item)
		}
		return Vector{Items: items}
	case IndexExpr:
		x.Expr = rewriteWhereAggregates(x.Expr)
		x.Index = rewriteWhereAggregates(x.Index)
		return x
	default:
		return expr
	}
}

// lowerWhereAggregate lowers one whereAggregate marker. wavg carries its
// weight as the first bracket argument.
func lowerWhereAggregate(x whereAggregate) (data.Expr, error) {
	if x.Func == "wavg" {
		weight, value, err := splitWavgArgs(x.Arg)
		if err != nil {
			return nil, err
		}
		weightExpr, err := lowerExpr(weight)
		if err != nil {
			return nil, err
		}
		valueExpr, err := lowerExpr(value)
		if err != nil {
			return nil, err
		}
		return data.ScalarAggregateExpr{Func: x.Func, Expr: valueExpr, Weight: weightExpr}, nil
	}
	if x.Func == "count" {
		return data.ScalarAggregateExpr{Func: "count"}, nil
	}
	arg, err := lowerExpr(x.Arg)
	if err != nil {
		return nil, err
	}
	return data.ScalarAggregateExpr{Func: x.Func, Expr: arg}, nil
}

// splitWavgArgs splits the wavg[w;v] argument vector.
func splitWavgArgs(arg Expr) (weight, value Expr, err error) {
	vector, ok := arg.(Vector)
	if !ok || len(vector.Items) != 2 {
		return nil, nil, fmt.Errorf("q wavg expects weight and value arguments: wavg[w;v]")
	}
	return vector.Items[0], vector.Items[1], nil
}

// lowerWhereExpr lowers a qSQL where clause: aggregate calls become
// whole-column scalar aggregates and the virtual column i becomes the row
// index.
func lowerWhereExpr(expr Expr) (data.Expr, error) {
	filter, err := lowerExpr(rewriteWhereAggregates(expr))
	if err != nil {
		return nil, err
	}
	return rewriteWhereVirtualColumns(filter), nil
}

// rewriteWhereVirtualColumns maps ColumnRef{"i"} to data.RowIndexRef, the
// virtual row-index column of canonical qSQL where clauses. RowIndexRef still
// prefers a real column named i when the frame carries one.
func rewriteWhereVirtualColumns(expr data.Expr) data.Expr {
	switch e := expr.(type) {
	case nil:
		return nil
	case data.ColumnRef:
		if e.Name == "i" {
			return data.RowIndexRef{}
		}
		return e
	case data.Binary:
		e.Left = rewriteWhereVirtualColumns(e.Left)
		e.Right = rewriteWhereVirtualColumns(e.Right)
		return e
	case data.Logical:
		e.Left = rewriteWhereVirtualColumns(e.Left)
		e.Right = rewriteWhereVirtualColumns(e.Right)
		return e
	case data.Not:
		e.Expr = rewriteWhereVirtualColumns(e.Expr)
		return e
	case data.Conditional:
		e.Cond = rewriteWhereVirtualColumns(e.Cond)
		e.Then = rewriteWhereVirtualColumns(e.Then)
		e.Else = rewriteWhereVirtualColumns(e.Else)
		return e
	case data.In:
		e.Expr = rewriteWhereVirtualColumns(e.Expr)
		return e
	case data.Within:
		e.Expr = rewriteWhereVirtualColumns(e.Expr)
		return e
	case data.ScalarAggregateExpr:
		e.Expr = rewriteWhereVirtualColumns(e.Expr)
		e.Weight = rewriteWhereVirtualColumns(e.Weight)
		return e
	case qLikeExpr:
		e.Expr = rewriteWhereVirtualColumns(e.Expr)
		return e
	default:
		return expr
	}
}

// qLikeExpr is the lowered q `like` predicate: a glob match of a string or
// symbol expression against a literal pattern.
type qLikeExpr struct {
	Expr    data.Expr
	Pattern string
}

func (e qLikeExpr) EvalRow(frame data.Frame, row int) (any, error) {
	v, err := e.Expr.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	return likeScalar(v, e.Pattern)
}

// lowerLikeExpr lowers `expr like "pattern"`. The pattern must be a literal
// string or symbol.
func lowerLikeExpr(left Expr, right Expr) (data.Expr, error) {
	expr, err := lowerExpr(left)
	if err != nil {
		return nil, err
	}
	pattern, err := lowerLiteralValue(right)
	if err != nil {
		return nil, fmt.Errorf("q like expects a literal string or symbol pattern: %w", err)
	}
	text, ok := qLikeText(pattern)
	if !ok {
		return nil, fmt.Errorf("q like expects a literal string or symbol pattern, got %T", pattern)
	}
	return qLikeExpr{Expr: expr, Pattern: text}, nil
}
