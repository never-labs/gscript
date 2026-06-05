package bind

import (
	"fmt"
	"sort"
	"strings"
)

// BuildQ creates the "q" column-query helper library. It intentionally
// accepts ordinary Leia tables as query plans and executes over SoA columns so
// data-heavy scripts can stay concise without introducing a second query
// language into the core grammar.
func BuildQ() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "q." + name, Fn: fn}))
	}
	set("query", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsSoA() {
			return nil, fmt.Errorf("q.query: argument 1 must be soa")
		}
		if len(args) < 2 || !args[1].IsTable() {
			return nil, fmt.Errorf("q.query: argument 2 must be a query plan table")
		}
		out, err := qRunQuery(args[0].SoA(), args[1].Table())
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(out)}, nil
	})
	return t
}

func qRunQuery(s *SoA, spec *Table) (*Table, error) {
	mask, err := qQueryMask(s, spec.RawGetString("where"))
	if err != nil {
		return nil, err
	}
	selects, err := qQuerySelects(spec.RawGetString("select"))
	if err != nil {
		return nil, err
	}
	if len(selects) == 0 {
		for _, name := range s.ColumnNames() {
			selects = append(selects, qSelect{Name: name, Expr: StringValue(name)})
		}
	}
	by, err := qStringList("q.query by", spec.RawGetString("by"))
	if err != nil {
		return nil, err
	}
	aggs, err := qAggregates(spec.RawGetString("aggregate"))
	if err != nil {
		return nil, err
	}
	var rows *Table
	if len(aggs) == 0 {
		rows, err = qRows(s, mask, selects)
	} else {
		rows, err = qGroupedRows(s, mask, by, selects, aggs)
	}
	if err != nil {
		return nil, err
	}
	return qApplyOrderAndLimit(rows, spec)
}

func qQueryMask(s *SoA, where Value) (*DenseArray, error) {
	if where.IsNil() {
		mask, err := NewDenseArrayOfLen(DenseArrayBool, s.Len())
		if err != nil {
			return nil, err
		}
		if err := mask.Fill(BoolValue(true)); err != nil {
			return nil, err
		}
		return mask, nil
	}
	if where.IsDenseArray() {
		mask := where.DenseArray()
		if mask.DType() != DenseArrayBool {
			return nil, fmt.Errorf("q.query where mask must be a bool dense array")
		}
		if mask.Len() != s.Len() {
			return nil, fmt.Errorf("q.query where mask length mismatch")
		}
		return mask, nil
	}
	if !where.IsTable() {
		return nil, fmt.Errorf("q.query where must be a bool dense array or condition table")
	}
	tbl := where.Table()
	column := tbl.RawGetString("column")
	if column.IsNil() {
		column = tbl.RawGetInt(1)
	}
	op := tbl.RawGetString("op")
	if op.IsNil() {
		op = tbl.RawGetInt(2)
	}
	value := tbl.RawGetString("value")
	if value.IsNil() {
		value = tbl.RawGetInt(3)
	}
	if !column.IsString() || !op.IsString() {
		return nil, fmt.Errorf("q.query where condition must provide column and op")
	}
	return s.Mask(column.Str(), op.Str(), value)
}

type qSelect struct {
	Name string
	Expr Value
}

func qQuerySelects(v Value) ([]qSelect, error) {
	if v.IsNil() {
		return nil, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query select must be a table")
	}
	var selects []qSelect
	ok := v.Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		selects = append(selects, qSelect{Name: key.Str(), Expr: val})
		return true
	})
	if !ok {
		return nil, fmt.Errorf("q.query select must be a plain string-keyed table")
	}
	sort.Slice(selects, func(i, j int) bool { return selects[i].Name < selects[j].Name })
	return selects, nil
}

func qStringList(name string, v Value) ([]string, error) {
	if v.IsNil() {
		return nil, nil
	}
	if v.IsString() {
		return []string{v.Str()}, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("%s must be a string or string array", name)
	}
	n := v.Table().Length()
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		item := v.Table().RawGetInt(int64(i))
		if !item.IsString() {
			return nil, fmt.Errorf("%s item %d must be a string", name, i)
		}
		out = append(out, item.Str())
	}
	return out, nil
}

func qAggregates(v Value) (map[string]string, error) {
	if v.IsNil() {
		return nil, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query aggregate must be a table")
	}
	out := make(map[string]string)
	ok := v.Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() || !val.IsString() {
			return false
		}
		agg := val.Str()
		switch agg {
		case "sum", "mean", "min", "max", "count":
			out[key.Str()] = agg
			return true
		default:
			return false
		}
	})
	if !ok {
		return nil, fmt.Errorf("q.query aggregate must be a plain table of supported aggregate names")
	}
	return out, nil
}

func qRows(s *SoA, mask *DenseArray, selects []qSelect) (*Table, error) {
	rows := NewTable()
	rowIndex := int64(1)
	for i := 0; i < s.Len(); i++ {
		include, err := qMaskAt(mask, i)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		row := NewTable()
		for _, sel := range selects {
			v, err := qEvalExprAt(s, sel.Expr, i)
			if err != nil {
				return nil, fmt.Errorf("q.query select %s: %w", sel.Name, err)
			}
			row.RawSetString(sel.Name, v)
		}
		rows.RawSetInt(rowIndex, TableValue(row))
		rowIndex++
	}
	return rows, nil
}

func qGroupedRows(s *SoA, mask *DenseArray, by []string, selects []qSelect, aggs map[string]string) (*Table, error) {
	groups := make(map[string]*qGroup)
	var order []string
	for i := 0; i < s.Len(); i++ {
		include, err := qMaskAt(mask, i)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		key, vals, err := qGroupKey(s, by, i)
		if err != nil {
			return nil, err
		}
		g := groups[key]
		if g == nil {
			g = &qGroup{ByValues: vals, Aggregates: make(map[string]*qAggState)}
			groups[key] = g
			order = append(order, key)
		}
		for _, sel := range selects {
			agg, ok := aggs[sel.Name]
			if !ok {
				continue
			}
			state := g.Aggregates[sel.Name]
			if state == nil {
				state = &qAggState{Kind: agg}
				g.Aggregates[sel.Name] = state
			}
			if agg == "count" {
				state.Add(IntValue(1))
				continue
			}
			v, err := qEvalExprAt(s, sel.Expr, i)
			if err != nil {
				return nil, fmt.Errorf("q.query aggregate %s: %w", sel.Name, err)
			}
			if !v.IsNumber() {
				return nil, fmt.Errorf("q.query aggregate %s requires numeric values", sel.Name)
			}
			state.Add(v)
		}
	}
	sort.Strings(order)
	rows := NewTable()
	for i, key := range order {
		g := groups[key]
		row := NewTable()
		for idx, name := range by {
			row.RawSetString(name, g.ByValues[idx])
		}
		names := make([]string, 0, len(g.Aggregates))
		for name := range g.Aggregates {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			row.RawSetString(name, g.Aggregates[name].Value())
		}
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows, nil
}

type qOrderSpec struct {
	Column string
	Desc   bool
}

func qApplyOrderAndLimit(rows *Table, spec *Table) (*Table, error) {
	order, err := qOrderSpecs(spec.RawGetString("order_by"))
	if err != nil {
		return nil, err
	}
	limit, err := qLimit(spec.RawGetString("limit"))
	if err != nil {
		return nil, err
	}
	if len(order) == 0 && limit < 0 {
		return rows, nil
	}
	values := make([]Value, 0, rows.Length())
	for i := 1; i <= rows.Length(); i++ {
		row := rows.RawGetInt(int64(i))
		if !row.IsNil() {
			values = append(values, row)
		}
	}
	if len(order) > 0 {
		sort.SliceStable(values, func(i, j int) bool {
			left := values[i].Table()
			right := values[j].Table()
			for _, ord := range order {
				cmp := qCompareValues(left.RawGetString(ord.Column), right.RawGetString(ord.Column))
				if cmp == 0 {
					continue
				}
				if ord.Desc {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}
	if limit >= 0 && limit < len(values) {
		values = values[:limit]
	}
	out := NewTable()
	for i, row := range values {
		out.RawSetInt(int64(i+1), row)
	}
	return out, nil
}

func qOrderSpecs(v Value) ([]qOrderSpec, error) {
	if v.IsNil() {
		return nil, nil
	}
	if v.IsString() {
		return []qOrderSpec{{Column: v.Str()}}, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query order_by must be a string, table, or array")
	}
	tbl := v.Table()
	if col := tbl.RawGetString("column"); col.IsString() {
		return []qOrderSpec{{
			Column: col.Str(),
			Desc:   qTruthy(tbl.RawGetString("desc")),
		}}, nil
	}
	n := tbl.Length()
	out := make([]qOrderSpec, 0, n)
	for i := 1; i <= n; i++ {
		item := tbl.RawGetInt(int64(i))
		switch {
		case item.IsString():
			out = append(out, qOrderSpec{Column: item.Str()})
		case item.IsTable():
			col := item.Table().RawGetString("column")
			if !col.IsString() {
				return nil, fmt.Errorf("q.query order_by item %d must provide column", i)
			}
			out = append(out, qOrderSpec{
				Column: col.Str(),
				Desc:   qTruthy(item.Table().RawGetString("desc")),
			})
		default:
			return nil, fmt.Errorf("q.query order_by item %d must be a string or table", i)
		}
	}
	return out, nil
}

func qLimit(v Value) (int, error) {
	if v.IsNil() {
		return -1, nil
	}
	if !v.IsInt() {
		return 0, fmt.Errorf("q.query limit must be an integer")
	}
	if v.Int() < 0 {
		return 0, fmt.Errorf("q.query limit must be non-negative")
	}
	return int(v.Int()), nil
}

func qCompareValues(left, right Value) int {
	switch {
	case left.IsNumber() && right.IsNumber():
		lf, rf := left.Number(), right.Number()
		if lf < rf {
			return -1
		}
		if lf > rf {
			return 1
		}
		return 0
	case left.IsString() && right.IsString():
		if left.Str() < right.Str() {
			return -1
		}
		if left.Str() > right.Str() {
			return 1
		}
		return 0
	case left.IsBool() && right.IsBool():
		if left.Bool() == right.Bool() {
			return 0
		}
		if !left.Bool() {
			return -1
		}
		return 1
	case left.IsNil() && right.IsNil():
		return 0
	case left.IsNil():
		return -1
	case right.IsNil():
		return 1
	default:
		ls, rs := left.String(), right.String()
		if ls < rs {
			return -1
		}
		if ls > rs {
			return 1
		}
		return 0
	}
}

func qTruthy(v Value) bool {
	return !(v.IsNil() || (v.IsBool() && !v.Bool()))
}

type qGroup struct {
	ByValues   []Value
	Aggregates map[string]*qAggState
}

type qAggState struct {
	Kind  string
	Count int64
	Sum   float64
	Min   float64
	Max   float64
}

func (a *qAggState) Add(v Value) {
	a.Count++
	if a.Kind == "count" {
		a.Sum++
		return
	}
	x := v.Number()
	a.Sum += x
	if a.Count == 1 || x < a.Min {
		a.Min = x
	}
	if a.Count == 1 || x > a.Max {
		a.Max = x
	}
}

func (a *qAggState) Value() Value {
	switch a.Kind {
	case "count":
		return IntValue(a.Count)
	case "mean":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Sum / float64(a.Count))
	case "min":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Min)
	case "max":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Max)
	default:
		return FloatValue(a.Sum)
	}
}

func qGroupKey(s *SoA, by []string, row int) (string, []Value, error) {
	if len(by) == 0 {
		return "", nil, nil
	}
	parts := make([]string, len(by))
	values := make([]Value, len(by))
	for i, name := range by {
		col, ok := s.Column(name)
		if !ok {
			return "", nil, fmt.Errorf("q.query by column %q not found", name)
		}
		v, err := col.At(row)
		if err != nil {
			return "", nil, err
		}
		values[i] = v
		parts[i] = qValueKey(v)
	}
	return strings.Join(parts, "\x00"), values, nil
}

func qValueKey(v Value) string {
	switch {
	case v.IsInt():
		return fmt.Sprintf("i:%d", v.Int())
	case v.IsFloat():
		return fmt.Sprintf("f:%g", v.Float())
	case v.IsBool():
		return fmt.Sprintf("b:%t", v.Bool())
	case v.IsString():
		return "s:" + v.Str()
	case v.IsNil():
		return "nil"
	default:
		return v.String()
	}
}

func qEvalExprAt(s *SoA, expr Value, row int) (Value, error) {
	if expr.IsString() {
		col, ok := s.Column(expr.Str())
		if !ok {
			return NilValue(), fmt.Errorf("column %q not found", expr.Str())
		}
		return col.At(row)
	}
	if !expr.IsTable() {
		return expr, nil
	}
	tbl := expr.Table()
	op := tbl.RawGetString("op")
	if op.IsNil() {
		op = tbl.RawGetInt(1)
	}
	if !op.IsString() {
		return NilValue(), fmt.Errorf("expression table must start with an operator")
	}
	left := tbl.RawGetString("left")
	if left.IsNil() {
		left = tbl.RawGetInt(2)
	}
	right := tbl.RawGetString("right")
	if right.IsNil() {
		right = tbl.RawGetInt(3)
	}
	lv, err := qEvalExprAt(s, left, row)
	if err != nil {
		return NilValue(), err
	}
	rv, err := qEvalExprAt(s, right, row)
	if err != nil {
		return NilValue(), err
	}
	if !lv.IsNumber() || !rv.IsNumber() {
		return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
	}
	switch op.Str() {
	case "+":
		return FloatValue(lv.Number() + rv.Number()), nil
	case "-":
		return FloatValue(lv.Number() - rv.Number()), nil
	case "*":
		return FloatValue(lv.Number() * rv.Number()), nil
	case "/":
		return FloatValue(lv.Number() / rv.Number()), nil
	default:
		return NilValue(), fmt.Errorf("operator %q is not supported", op.Str())
	}
}

func qMaskAt(mask *DenseArray, i int) (bool, error) {
	v, err := mask.At(i)
	if err != nil {
		return false, err
	}
	return v.Bool(), nil
}
