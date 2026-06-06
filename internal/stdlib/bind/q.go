package bind

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

const qSymbolVectorMarker = "__q_symbol_vector"

const qSQLPlanCacheLimit = 128

type qSQLPlanTemplate struct {
	source string
	plan   data.QueryPlan
}

var (
	qSQLTemplateCacheMu sync.Mutex
	qSQLTemplateCache   = make(map[string]qSQLPlanTemplate)
	qSQLTemplateOrder   []string

	qSQLAlignedPlanCacheMu sync.Mutex
	qSQLAlignedPlanCache   = make(map[string]data.QueryPlan)
	qSQLAlignedPlanOrder   []string
)

// BuildQ creates the "q" column-query helper library. q.query keeps the
// ordinary Leia table-plan API, while q.eval and q`...` expose a small
// q/kdb+-style symbolic vector subset.
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
	set("eval", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("q.eval: argument 1 must be a q source string")
		}
		return qEvalSymbolic(args[0].Str())
	})
	qsql := func(name string, args []Value) ([]Value, error) {
		frameValue, src, resolveSource, err := qSQLArgs(name, args)
		if err != nil {
			return nil, err
		}
		out, err := qRunSQL(name, frameValue, src, resolveSource)
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(out)}, nil
	}
	set("sql", func(args []Value) ([]Value, error) {
		return qsql("q.sql", args)
	})
	set("select", func(args []Value) ([]Value, error) {
		return qsql("q.select", args)
	})
	set("count", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.count: argument 1 required")
		}
		return []Value{IntValue(int64(qCount(args[0])))}, nil
	})
	return t
}

func qSQLArgs(name string, args []Value) (Value, string, bool, error) {
	if len(args) < 2 {
		return NilValue(), "", false, fmt.Errorf("%s: expected (frame, qSQL source) or (qSQL source, frames)", name)
	}
	if args[0].IsString() {
		return args[1], args[0].Str(), true, nil
	}
	if args[1].IsString() {
		return args[0], args[1].Str(), false, nil
	}
	return NilValue(), "", false, fmt.Errorf("%s: expected one qSQL source string argument", name)
}

func dialectQ(body Value, opts *Table) ([]Value, error) {
	mode := dialectMode(opts)
	if mode != "" && mode != "eval" && mode != "parse" {
		return dialectUnknownMode("q", mode)
	}
	return qEvalSymbolic(body.String())
}

func qEvalSymbolic(src string) ([]Value, error) {
	v, err := qParseSymbolic(strings.TrimSpace(src))
	if err != nil {
		return nil, fmt.Errorf("q dialect: %w", err)
	}
	return []Value{v}, nil
}

func qParseSymbolic(src string) (Value, error) {
	if src == "" {
		return NilValue(), fmt.Errorf("empty q expression")
	}
	if qEnclosed(src, '(', ')') {
		return qParseSymbolic(strings.TrimSpace(src[1 : len(src)-1]))
	}
	if strings.HasPrefix(src, "flip ") {
		v, err := qParseSymbolic(strings.TrimSpace(src[len("flip "):]))
		if err != nil {
			return NilValue(), err
		}
		return qFlip(v)
	}
	if strings.HasPrefix(src, "where ") {
		v, err := qParseSymbolic(strings.TrimSpace(src[len("where "):]))
		if err != nil {
			return NilValue(), err
		}
		return qWhere(v)
	}
	if strings.HasPrefix(src, "sums ") {
		v, err := qParseSymbolic(strings.TrimSpace(src[len("sums "):]))
		if err != nil {
			return NilValue(), err
		}
		return qSums(v)
	}
	if strings.HasPrefix(src, "sum ") {
		v, err := qParseSymbolic(strings.TrimSpace(src[len("sum "):]))
		if err != nil {
			return NilValue(), err
		}
		return qSum(v)
	}
	if bang := qFindTopLevel(src, "!"); bang >= 0 {
		keys, err := qParseSymbolList(strings.TrimSpace(src[:bang]))
		if err != nil {
			return NilValue(), err
		}
		values, err := qParseSymbolic(strings.TrimSpace(src[bang+1:]))
		if err != nil {
			return NilValue(), err
		}
		return qSymbolDict(keys, values)
	}
	if hash := qFindTopLevel(src, "#"); hash >= 0 {
		n, _, err := qParseNumber(strings.TrimSpace(src[:hash]))
		if err != nil {
			return NilValue(), fmt.Errorf("# left operand must be an integer count")
		}
		if !n.IsInt() {
			return NilValue(), fmt.Errorf("# left operand must be an integer count")
		}
		v, err := qParseSymbolic(strings.TrimSpace(src[hash+1:]))
		if err != nil {
			return NilValue(), err
		}
		return qTake(int(n.Int()), v)
	}
	if strings.HasPrefix(src, "+/") {
		v, err := qParseSymbolic(strings.TrimSpace(src[2:]))
		if err != nil {
			return NilValue(), err
		}
		return qSum(v)
	}
	if strings.HasPrefix(src, "+\\") {
		v, err := qParseSymbolic(strings.TrimSpace(src[2:]))
		if err != nil {
			return NilValue(), err
		}
		return qSums(v)
	}
	if parts := qSplitTopLevel(src, ';'); len(parts) > 1 {
		out := NewAppendArrayTable(len(parts))
		for i, part := range parts {
			v, err := qParseSymbolic(part)
			if err != nil {
				return NilValue(), err
			}
			out.RawSetInt(int64(i+1), v)
		}
		return TableValue(out), nil
	}
	if strings.HasPrefix(src, "`") {
		keys, err := qParseSymbolList(src)
		if err != nil {
			return NilValue(), err
		}
		return qSymbolListValue(keys), nil
	}
	if idx, op, ok := qFindDyadic(src); ok {
		left, err := qParseSymbolic(strings.TrimSpace(src[:idx]))
		if err != nil {
			return NilValue(), err
		}
		right, err := qParseSymbolic(strings.TrimSpace(src[idx+1:]))
		if err != nil {
			return NilValue(), err
		}
		return qApplyDyadic(op, left, right)
	}
	return qParseNumericAtom(src)
}

func qEnclosed(src string, open, close byte) bool {
	if len(src) < 2 || src[0] != open || src[len(src)-1] != close {
		return false
	}
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case open:
			depth++
		case close:
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

func qFindTopLevel(src, ops string) int {
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
		default:
			if depth == 0 && strings.ContainsRune(ops, rune(src[i])) {
				if (src[i] == '+' || src[i] == '-') && qIsSign(src, i) {
					continue
				}
				return i
			}
		}
	}
	return -1
}

func qSplitTopLevel(src string, sep byte) []string {
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

func qIsSign(src string, i int) bool {
	if i+1 >= len(src) || (src[i+1] < '0' || src[i+1] > '9') {
		return false
	}
	if i == 0 {
		return true
	}
	prev := src[i-1]
	return prev == ' ' || prev == '\t' || prev == '\n' || prev == '(' || prev == ';'
}

func qParseNumericAtom(src string) (Value, error) {
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return NilValue(), fmt.Errorf("empty q expression")
	}
	values := make([]Value, len(fields))
	hasFloat := false
	for i, field := range fields {
		v, isFloat, err := qParseNumber(field)
		if err != nil {
			return NilValue(), err
		}
		values[i] = v
		hasFloat = hasFloat || isFloat
	}
	if len(values) == 1 {
		return values[0], nil
	}
	if hasFloat {
		xs := make([]float64, len(values))
		for i, v := range values {
			xs[i] = v.Number()
		}
		return DenseArrayValue(NewDenseArrayF64(xs)), nil
	}
	xs := make([]int64, len(values))
	for i, v := range values {
		xs[i] = v.Int()
	}
	return DenseArrayValue(NewDenseArrayI64(xs)), nil
}

func qParseNumber(src string) (Value, bool, error) {
	if strings.ContainsAny(src, ".eE") {
		f, err := strconv.ParseFloat(src, 64)
		if err != nil {
			return NilValue(), false, fmt.Errorf("invalid number %q", src)
		}
		return FloatValue(f), true, nil
	}
	i, err := strconv.ParseInt(src, 10, 64)
	if err != nil {
		return NilValue(), false, fmt.Errorf("invalid number %q", src)
	}
	return IntValue(i), false, nil
}

func qFindDyadic(src string) (int, byte, bool) {
	for _, ops := range []string{"=<>", "+-", "*%"} {
		if idx := qFindTopLevel(src, ops); idx >= 0 {
			return idx, src[idx], true
		}
	}
	return 0, 0, false
}

func qApplyDyadic(op byte, left, right Value) (Value, error) {
	if left.IsDenseArray() || right.IsDenseArray() {
		if op == '=' || op == '<' || op == '>' {
			return qCompareVector(op, left, right)
		}
		denseOp, err := qDenseArrayOp(op)
		if err != nil {
			return NilValue(), err
		}
		return DenseArrayElementwise(denseOp, left, right)
	}
	if !left.IsNumber() || !right.IsNumber() {
		return NilValue(), fmt.Errorf("operator %q expects numeric operands", string(op))
	}
	if left.IsInt() && right.IsInt() && op != '%' {
		switch op {
		case '+':
			return IntValue(left.Int() + right.Int()), nil
		case '-':
			return IntValue(left.Int() - right.Int()), nil
		case '*':
			return IntValue(left.Int() * right.Int()), nil
		case '=':
			return BoolValue(left.Int() == right.Int()), nil
		case '<':
			return BoolValue(left.Int() < right.Int()), nil
		case '>':
			return BoolValue(left.Int() > right.Int()), nil
		}
	}
	switch op {
	case '+':
		return FloatValue(left.Number() + right.Number()), nil
	case '-':
		return FloatValue(left.Number() - right.Number()), nil
	case '*':
		return FloatValue(left.Number() * right.Number()), nil
	case '%':
		return FloatValue(left.Number() / right.Number()), nil
	case '=':
		return BoolValue(left.Number() == right.Number()), nil
	case '<':
		return BoolValue(left.Number() < right.Number()), nil
	case '>':
		return BoolValue(left.Number() > right.Number()), nil
	default:
		return NilValue(), fmt.Errorf("operator %q is not supported", string(op))
	}
}

func qCompareVector(op byte, left, right Value) (Value, error) {
	la, ra := left.DenseArray(), right.DenseArray()
	n := 0
	switch {
	case la != nil && ra != nil:
		if la.Len() != ra.Len() {
			return NilValue(), fmt.Errorf("comparison length mismatch")
		}
		n = la.Len()
	case la != nil:
		n = la.Len()
	case ra != nil:
		n = ra.Len()
	default:
		return qApplyDyadic(op, left, right)
	}
	out, err := NewDenseArrayOfLen(DenseArrayBool, n)
	if err != nil {
		return NilValue(), err
	}
	for i := 0; i < n; i++ {
		lv, rv := left, right
		if la != nil {
			lv, err = la.At(i)
			if err != nil {
				return NilValue(), err
			}
		}
		if ra != nil {
			rv, err = ra.At(i)
			if err != nil {
				return NilValue(), err
			}
		}
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q expects numeric operands", string(op))
		}
		var ok bool
		switch op {
		case '=':
			ok = lv.Number() == rv.Number()
		case '<':
			ok = lv.Number() < rv.Number()
		case '>':
			ok = lv.Number() > rv.Number()
		}
		if err := out.Set(i, BoolValue(ok)); err != nil {
			return NilValue(), err
		}
	}
	return DenseArrayValue(out), nil
}

func qDenseArrayOp(op byte) (DenseArrayBinaryOp, error) {
	switch op {
	case '+':
		return DenseArrayAdd, nil
	case '-':
		return DenseArraySub, nil
	case '*':
		return DenseArrayMul, nil
	case '%':
		return DenseArrayDiv, nil
	default:
		return DenseArrayAdd, fmt.Errorf("operator %q is not supported for dense arrays", string(op))
	}
}

func qParseSymbolList(src string) ([]string, error) {
	if !strings.HasPrefix(src, "`") {
		return nil, fmt.Errorf("symbol list must start with `")
	}
	var out []string
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
		out = append(out, sym)
	}
	return out, nil
}

func qSymbolListValue(keys []string) Value {
	out := NewAppendArrayTable(len(keys))
	for i, key := range keys {
		out.RawSetInt(int64(i+1), StringValue(key))
	}
	out.RawSetString(qSymbolVectorMarker, BoolValue(true))
	return TableValue(out)
}

func qIsSymbolVector(v Value) bool {
	return v.IsTable() && v.Table().RawGetString(qSymbolVectorMarker).Truthy()
}

func qSymbolDict(keys []string, values Value) (Value, error) {
	out := NewTable()
	switch {
	case values.IsDenseArray():
		if values.DenseArray().Len() != len(keys) {
			return NilValue(), fmt.Errorf("dict key/value length mismatch")
		}
		for i, key := range keys {
			v, err := values.DenseArray().At(i)
			if err != nil {
				return NilValue(), err
			}
			out.RawSetString(key, v)
		}
	case values.IsTable():
		if values.Table().Length() != len(keys) {
			return NilValue(), fmt.Errorf("dict key/value length mismatch")
		}
		for i, key := range keys {
			out.RawSetString(key, values.Table().RawGetInt(int64(i+1)))
		}
	default:
		if len(keys) != 1 {
			return NilValue(), fmt.Errorf("dict key/value length mismatch")
		}
		out.RawSetString(keys[0], values)
	}
	return TableValue(out), nil
}

func qFlip(v Value) (Value, error) {
	if !v.IsTable() {
		return NilValue(), fmt.Errorf("flip expects a dictionary")
	}
	dict := v.Table()
	names := make([]string, 0, dict.Length())
	ok := dict.ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		switch {
		case val.IsDenseArray(), val.IsTable():
			names = append(names, key.Str())
			return true
		default:
			return false
		}
	})
	if !ok {
		return NilValue(), fmt.Errorf("flip expects a plain dictionary of vectors")
	}
	sort.Strings(names)
	rows := -1
	for _, name := range names {
		n := qVectorLen(dict.RawGetString(name))
		if rows < 0 {
			rows = n
		} else if rows != n {
			return NilValue(), fmt.Errorf("flip column length mismatch")
		}
	}
	out := NewAppendArrayTable(rows)
	kinds := NewTable()
	for i := 0; i < rows; i++ {
		row := NewTable()
		for _, name := range names {
			source := dict.RawGetString(name)
			if qIsSymbolVector(source) {
				kinds.RawSetString(name, StringValue(string(data.KindSymbol)))
			}
			item, err := qVectorAt(source, i)
			if err != nil {
				return NilValue(), err
			}
			row.RawSetString(name, item)
		}
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	if kinds.Length() > 0 {
		out.RawSetString("column_kinds", TableValue(kinds))
	}
	return TableValue(out), nil
}

func qVectorLen(v Value) int {
	if v.IsDenseArray() {
		return v.DenseArray().Len()
	}
	if v.IsTable() {
		return v.Table().Length()
	}
	return 0
}

func qCount(v Value) int {
	switch {
	case v.IsNil():
		return 0
	case v.IsDenseArray(), v.IsTable():
		return qVectorLen(v)
	case v.IsString():
		return len(v.Str())
	default:
		return 1
	}
}

func qSum(v Value) (Value, error) {
	if v.IsNumber() {
		return v, nil
	}
	if !v.IsDenseArray() {
		return NilValue(), fmt.Errorf("sum expects a numeric vector")
	}
	return DenseArrayReduce(DenseArrayReduceSum, v.DenseArray())
}

func qSums(v Value) (Value, error) {
	if v.IsNumber() {
		return v, nil
	}
	if !v.IsDenseArray() {
		return NilValue(), fmt.Errorf("sums expects a numeric vector")
	}
	scan, err := DenseArrayScan(v.DenseArray())
	if err != nil {
		return NilValue(), err
	}
	return DenseArrayValue(scan), nil
}

func qVectorAt(v Value, i int) (Value, error) {
	if v.IsDenseArray() {
		return v.DenseArray().At(i)
	}
	if v.IsTable() {
		return v.Table().RawGetInt(int64(i + 1)), nil
	}
	return NilValue(), fmt.Errorf("not a vector")
}

func qWhere(v Value) (Value, error) {
	if !v.IsDenseArray() || v.DenseArray().DType() != DenseArrayBool {
		return NilValue(), fmt.Errorf("where expects a bool vector")
	}
	var indexes []int64
	for i := 0; i < v.DenseArray().Len(); i++ {
		item, err := v.DenseArray().At(i)
		if err != nil {
			return NilValue(), err
		}
		if item.IsBool() && item.Bool() {
			indexes = append(indexes, int64(i+1))
		}
	}
	return DenseArrayValue(NewDenseArrayI64(indexes)), nil
}

func qTake(n int, v Value) (Value, error) {
	if n < 0 {
		return NilValue(), fmt.Errorf("# negative counts are not supported")
	}
	switch {
	case v.IsDenseArray():
		if n > v.DenseArray().Len() {
			n = v.DenseArray().Len()
		}
		switch v.DenseArray().DType() {
		case DenseArrayI64:
			out := make([]int64, n)
			for i := 0; i < n; i++ {
				item, err := v.DenseArray().At(i)
				if err != nil {
					return NilValue(), err
				}
				out[i] = item.Int()
			}
			return DenseArrayValue(NewDenseArrayI64(out)), nil
		case DenseArrayF64:
			out := make([]float64, n)
			for i := 0; i < n; i++ {
				item, err := v.DenseArray().At(i)
				if err != nil {
					return NilValue(), err
				}
				out[i] = item.Number()
			}
			return DenseArrayValue(NewDenseArrayF64(out)), nil
		default:
			out, err := NewDenseArrayOfLen(DenseArrayBool, n)
			if err != nil {
				return NilValue(), err
			}
			for i := 0; i < n; i++ {
				item, err := v.DenseArray().At(i)
				if err != nil {
					return NilValue(), err
				}
				if err := out.Set(i, item); err != nil {
					return NilValue(), err
				}
			}
			return DenseArrayValue(out), nil
		}
	case v.IsTable():
		if n > v.Table().Length() {
			n = v.Table().Length()
		}
		out := NewAppendArrayTable(n)
		for i := 1; i <= n; i++ {
			out.RawSetInt(int64(i), v.Table().RawGetInt(int64(i)))
		}
		return TableValue(out), nil
	default:
		if n == 0 {
			return TableValue(NewAppendArrayTable(0)), nil
		}
		out := NewAppendArrayTable(n)
		for i := 1; i <= n; i++ {
			out.RawSetInt(int64(i), v)
		}
		return TableValue(out), nil
	}
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

func qRunSQL(name string, frameValue Value, src string, resolveSource bool) (*Table, error) {
	tmpl, err := qSQLCachedPlanTemplate(name, src)
	if err != nil {
		return nil, err
	}
	sourceName := ""
	if resolveSource {
		sourceName = tmpl.source
	}
	frame, err := qDataFrameFromValue(frameValue, sourceName)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	plan := qSQLPlanForFrame(src, tmpl.plan, frame)
	plan.Source = frame
	out, err := plan.Exec()
	if err != nil {
		return nil, fmt.Errorf("%s: exec: %w", name, err)
	}
	return qRowsFromDataFrame(out)
}

func qSQLCachedPlanTemplate(name, src string) (qSQLPlanTemplate, error) {
	qSQLTemplateCacheMu.Lock()
	if tmpl, ok := qSQLTemplateCache[src]; ok {
		qSQLTemplateCacheMu.Unlock()
		tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
		return tmpl, nil
	}
	qSQLTemplateCacheMu.Unlock()

	query, err := stdq.Parse(strings.TrimSpace(src))
	if err != nil {
		if qSQLHasJoinToken(src) {
			return qSQLPlanTemplate{}, fmt.Errorf("%s: parse: q joins are not supported", name)
		}
		return qSQLPlanTemplate{}, fmt.Errorf("%s: parse: %w", name, err)
	}
	if query.Kind != stdq.SelectQuery && query.Kind != stdq.ExecQuery {
		return qSQLPlanTemplate{}, fmt.Errorf("%s: lower: q %s queries are not implemented", name, query.Kind)
	}
	lowered, err := stdq.Lower(query)
	if err != nil {
		return qSQLPlanTemplate{}, fmt.Errorf("%s: lower: %w", name, err)
	}
	if lowered.Distinct {
		lowered.Plan.Distinct = true
	}
	qNormalizePlanLiterals(&lowered.Plan)
	lowered.Plan.Source = data.Frame{}
	tmpl := qSQLPlanTemplate{
		source: lowered.Source,
		plan:   qCloneDataQueryPlan(lowered.Plan),
	}

	qSQLTemplateCacheMu.Lock()
	qSQLTemplateCacheStoreLocked(src, tmpl)
	qSQLTemplateCacheMu.Unlock()

	tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
	return tmpl, nil
}

func qSQLHasJoinToken(src string) bool {
	for _, tok := range qSQLBareTokens(src) {
		switch strings.ToLower(tok) {
		case "join", "lj", "ij", "uj", "aj", "wj", "pj":
			return true
		}
	}
	return false
}

func qSQLBareTokens(src string) []string {
	var toks []string
	start := -1
	flush := func(end int) {
		if start >= 0 {
			toks = append(toks, src[start:end])
			start = -1
		}
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == '"' {
			flush(i)
			i++
			for i < len(src) {
				if src[i] == '\\' {
					i += 2
					continue
				}
				if src[i] == '"' {
					break
				}
				i++
			}
			continue
		}
		if c == '`' {
			flush(i)
			i++
			for i < len(src) && !qSQLTokenDelimiter(src[i]) {
				i++
			}
			i--
			continue
		}
		if qSQLTokenDelimiter(c) {
			flush(i)
			continue
		}
		if start < 0 {
			start = i
		}
	}
	flush(len(src))
	return toks
}

func qSQLTokenDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == ':' || c == '(' || c == ')' || c == ';'
}

func qSQLPlanForFrame(src string, tmpl data.QueryPlan, frame data.Frame) data.QueryPlan {
	key := src + "\x00" + qDataFrameSchemaSignature(frame)
	qSQLAlignedPlanCacheMu.Lock()
	if plan, ok := qSQLAlignedPlanCache[key]; ok {
		qSQLAlignedPlanCacheMu.Unlock()
		return qCloneDataQueryPlan(plan)
	}
	qSQLAlignedPlanCacheMu.Unlock()

	plan := qCloneDataQueryPlan(tmpl)
	qAlignPlanLiteralsToFrame(&plan, frame)

	qSQLAlignedPlanCacheMu.Lock()
	qSQLAlignedPlanCacheStoreLocked(key, plan)
	qSQLAlignedPlanCacheMu.Unlock()

	return qCloneDataQueryPlan(plan)
}

func qSQLTemplateCacheStoreLocked(key string, tmpl qSQLPlanTemplate) {
	if _, ok := qSQLTemplateCache[key]; !ok {
		qSQLTemplateOrder = append(qSQLTemplateOrder, key)
	}
	tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
	qSQLTemplateCache[key] = tmpl
	for len(qSQLTemplateOrder) > qSQLPlanCacheLimit {
		evict := qSQLTemplateOrder[0]
		qSQLTemplateOrder = qSQLTemplateOrder[1:]
		delete(qSQLTemplateCache, evict)
	}
}

func qSQLAlignedPlanCacheStoreLocked(key string, plan data.QueryPlan) {
	if _, ok := qSQLAlignedPlanCache[key]; !ok {
		qSQLAlignedPlanOrder = append(qSQLAlignedPlanOrder, key)
	}
	qSQLAlignedPlanCache[key] = qCloneDataQueryPlan(plan)
	for len(qSQLAlignedPlanOrder) > qSQLPlanCacheLimit {
		evict := qSQLAlignedPlanOrder[0]
		qSQLAlignedPlanOrder = qSQLAlignedPlanOrder[1:]
		delete(qSQLAlignedPlanCache, evict)
	}
}

func qDataFrameSchemaSignature(frame data.Frame) string {
	schema := frame.Schema()
	names := schema.Names()
	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(string(name))
		b.WriteByte(':')
		if kind, ok := schema.Kind(name); ok {
			b.WriteString(string(kind))
		}
	}
	return b.String()
}

func qCloneDataQueryPlan(plan data.QueryPlan) data.QueryPlan {
	plan.Source = data.Frame{}
	plan.Where = qCloneDataExpr(plan.Where)
	plan.By = append([]data.Symbol(nil), plan.By...)
	plan.OrderBy = append([]data.OrderSpec(nil), plan.OrderBy...)
	if plan.Select != nil {
		selects := make([]data.SelectItem, len(plan.Select))
		for i, item := range plan.Select {
			selects[i] = data.SelectItem{Name: item.Name, Expr: qCloneDataExpr(item.Expr)}
		}
		plan.Select = selects
	}
	if plan.Aggregates != nil {
		aggs := make([]data.Aggregate, len(plan.Aggregates))
		for i, agg := range plan.Aggregates {
			aggs[i] = data.Aggregate{Name: agg.Name, Func: agg.Func, Expr: qCloneDataExpr(agg.Expr)}
		}
		plan.Aggregates = aggs
	}
	return plan
}

func qCloneDataExpr(expr data.Expr) data.Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case data.Binary:
		x.Left = qCloneDataExpr(x.Left)
		x.Right = qCloneDataExpr(x.Right)
		return x
	default:
		return expr
	}
}

func qAlignPlanLiteralsToFrame(plan *data.QueryPlan, frame data.Frame) {
	if plan == nil {
		return
	}
	schema := frame.Schema()
	plan.Where = qAlignDataExpr(plan.Where, schema)
	for i := range plan.Select {
		plan.Select[i].Expr = qAlignDataExpr(plan.Select[i].Expr, schema)
	}
	for i := range plan.Aggregates {
		plan.Aggregates[i].Expr = qAlignDataExpr(plan.Aggregates[i].Expr, schema)
	}
}

func qAlignDataExpr(expr data.Expr, schema data.Schema) data.Expr {
	switch x := expr.(type) {
	case data.Binary:
		x.Left = qAlignDataExpr(x.Left, schema)
		x.Right = qAlignDataExpr(x.Right, schema)
		if x.Op == data.OpEQ || x.Op == data.OpNE {
			x.Left, x.Right = qAlignBinaryLiteral(x.Left, x.Right, schema)
			x.Right, x.Left = qAlignBinaryLiteral(x.Right, x.Left, schema)
		}
		return x
	default:
		return expr
	}
}

func qAlignBinaryLiteral(left, right data.Expr, schema data.Schema) (data.Expr, data.Expr) {
	col, ok := left.(data.ColumnRef)
	if !ok {
		return left, right
	}
	lit, ok := right.(data.Literal)
	if !ok {
		return left, right
	}
	kind, ok := schema.Kind(col.Name)
	if !ok {
		return left, right
	}
	switch kind {
	case data.KindString:
		if sym, ok := lit.Value.(data.Symbol); ok {
			right = data.Literal{Value: string(sym)}
		}
	case data.KindSymbol:
		if s, ok := lit.Value.(string); ok {
			right = data.Literal{Value: data.Symbol(s)}
		}
	}
	return left, right
}

func qNormalizePlanLiterals(plan *data.QueryPlan) {
	if plan == nil {
		return
	}
	plan.Where = qNormalizeDataExpr(plan.Where)
	for i := range plan.Select {
		plan.Select[i].Expr = qNormalizeDataExpr(plan.Select[i].Expr)
	}
	for i := range plan.Aggregates {
		plan.Aggregates[i].Expr = qNormalizeDataExpr(plan.Aggregates[i].Expr)
	}
}

func qNormalizeDataExpr(expr data.Expr) data.Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case data.ColumnRef:
		switch strings.ToLower(string(x.Name)) {
		case "true":
			return data.Literal{Value: true}
		case "false":
			return data.Literal{Value: false}
		case "null":
			return data.Literal{Value: nil}
		default:
			return x
		}
	case data.Binary:
		x.Left = qNormalizeDataExpr(x.Left)
		x.Right = qNormalizeDataExpr(x.Right)
		return x
	default:
		return x
	}
}

type qSQLOptions struct {
	order []data.OrderSpec
	limit int
}

func (o qSQLOptions) apply(plan *data.QueryPlan) {
	if len(o.order) > 0 {
		plan.OrderBy = o.order
	}
	if o.limit >= 0 {
		plan.LimitN = o.limit
	}
}

func qSQLSourceAndOptions(src string) (string, qSQLOptions, error) {
	opts := qSQLOptions{limit: -1}
	base := strings.TrimSpace(src)
	if idx := qLastKeywordIndex(base, "limit"); idx >= 0 {
		n, err := strconv.Atoi(strings.TrimSpace(base[idx+len("limit"):]))
		if err != nil || n < 0 {
			return "", opts, fmt.Errorf("limit must be a non-negative integer")
		}
		opts.limit = n
		base = strings.TrimSpace(base[:idx])
	}
	if idx := qLastKeywordIndex(base, "order by"); idx >= 0 {
		specs, err := qSQLOrderSpecs(strings.TrimSpace(base[idx+len("order by"):]))
		if err != nil {
			return "", opts, err
		}
		opts.order = specs
		base = strings.TrimSpace(base[:idx])
	}
	return base, opts, nil
}

func qLastKeywordIndex(src, keyword string) int {
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return -1
	}
	lowerKeyword := strings.Fields(strings.ToLower(keyword))
	positions := make([]int, 0, len(fields))
	pos := 0
	for _, field := range fields {
		idx := strings.Index(src[pos:], field)
		if idx < 0 {
			return -1
		}
		pos += idx
		positions = append(positions, pos)
		pos += len(field)
	}
	for i := len(fields) - len(lowerKeyword); i >= 0; i-- {
		match := true
		for j, want := range lowerKeyword {
			if strings.ToLower(fields[i+j]) != want {
				match = false
				break
			}
		}
		if match {
			return positions[i]
		}
	}
	return -1
}

func qSQLOrderSpecs(src string) ([]data.OrderSpec, error) {
	if src == "" {
		return nil, fmt.Errorf("order by requires at least one column")
	}
	parts := strings.Split(src, ",")
	specs := make([]data.OrderSpec, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 || len(fields) > 2 {
			return nil, fmt.Errorf("invalid order by specification %q", part)
		}
		spec := data.OrderSpec{Column: data.Symbol(fields[0])}
		if len(fields) == 2 {
			switch strings.ToLower(fields[1]) {
			case "asc":
			case "desc":
				spec.Desc = true
			default:
				return nil, fmt.Errorf("invalid order direction %q", fields[1])
			}
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func qDataFrameFromValue(v Value, sourceName string) (data.Frame, error) {
	if sourceName != "" && v.IsTable() && !qLooksLikeFrame(v.Table()) {
		source := v.Table().RawGetString(sourceName)
		if source.IsNil() {
			return data.Frame{}, fmt.Errorf("source %q not found", sourceName)
		}
		return qDataFrameFromValue(source, "")
	}
	if v.IsSoA() {
		return qDataFrameFromSoA(v.SoA())
	}
	if !v.IsTable() {
		return data.Frame{}, fmt.Errorf("argument 1 must be a frame table or soa")
	}
	tbl := v.Table()
	if cols := tbl.RawGetString("columns"); cols.IsTable() {
		if names := tbl.RawGetString("column_names"); names.IsTable() {
			return qDataFrameFromColumns(cols.Table(), names.Table(), qFrameColumnKinds(tbl))
		}
	}
	if soa := tbl.RawGetString("soa"); soa.IsSoA() {
		return qDataFrameFromSoA(soa.SoA())
	}
	return qDataFrameFromRowTable(tbl)
}

func qLooksLikeFrame(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if isDataFrameTable(tbl) {
		return true
	}
	if kind := tbl.RawGetString("kind"); kind.IsString() && kind.Str() == "data_frame" {
		return true
	}
	if typ := tbl.RawGetString("type"); typ.IsString() && typ.Str() == "data_frame" {
		return true
	}
	return tbl.RawGetString("columns").IsTable() && tbl.RawGetString("column_names").IsTable()
}

func qDataFrameFromSoA(s *SoA) (data.Frame, error) {
	if s == nil {
		return data.Frame{}, fmt.Errorf("soa is nil")
	}
	cols := make([]data.Column, 0, len(s.ColumnNames()))
	for _, name := range s.ColumnNames() {
		col, ok := s.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("soa column %q not found", name)
		}
		array, err := qDataArrayFromDense(col)
		if err != nil {
			return data.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, data.Column{Name: data.Symbol(name), Data: array})
	}
	return data.NewFrame(cols...)
}

func qDataFrameFromColumns(columns, columnNames *Table, kinds map[string]data.Kind) (data.Frame, error) {
	cols := make([]data.Column, 0, columnNames.Length())
	for i := 1; i <= columnNames.Length(); i++ {
		nameValue := columnNames.RawGetInt(int64(i))
		if !nameValue.IsString() {
			return data.Frame{}, fmt.Errorf("frame column_names[%d] must be a string", i)
		}
		name := nameValue.Str()
		col, err := qDataColumnFromVector(data.Symbol(name), columns.RawGetString(name), kinds[name])
		if err != nil {
			return data.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, col)
	}
	return data.NewFrame(cols...)
}

func qFrameColumnKinds(frame *Table) map[string]data.Kind {
	if frame == nil {
		return nil
	}
	out := map[string]data.Kind{}
	addKinds := func(v Value) {
		if !v.IsTable() {
			return
		}
		v.Table().ForEachPlainRaw(func(key, val Value) bool {
			if key.IsString() && val.IsString() {
				if kind, ok := qDataKind(val.Str()); ok {
					out[key.Str()] = kind
				}
			}
			return true
		})
	}
	addKinds(frame.RawGetString("column_kinds"))
	addKinds(frame.RawGetString("kinds"))
	if schema := frame.RawGetString("schema"); schema.IsTable() {
		addKinds(schema.Table().RawGetString("kinds"))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func qDataKind(kind string) (data.Kind, bool) {
	switch strings.ToLower(kind) {
	case "i64", "int", "integer":
		return data.KindI64, true
	case "f64", "float", "real", "number":
		return data.KindF64, true
	case "bool", "boolean":
		return data.KindBool, true
	case "string", "text":
		return data.KindString, true
	case "symbol":
		return data.KindSymbol, true
	case "date", "time", "timestamp":
		return data.KindString, true
	case "null":
		return data.KindAny, true
	case "any":
		return data.KindAny, true
	default:
		return "", false
	}
}

func qDataColumnFromVector(name data.Symbol, v Value, kind data.Kind) (data.Column, error) {
	values, err := qAnyValuesFromVector(v)
	if err != nil {
		return data.Column{}, err
	}
	if kind == "" || kind == data.KindAny {
		return data.NewColumn(name, values), nil
	}
	array, ok := qTypedDataArray(values, kind)
	if !ok {
		return data.NewColumn(name, values), nil
	}
	return data.Column{Name: name, Data: array}, nil
}

func qTypedDataArray(values []any, kind data.Kind) (data.Array, bool) {
	for _, v := range values {
		if data.IsNull(v) {
			return data.NewAny(qCoerceDataValues(values, kind)), true
		}
	}
	switch kind {
	case data.KindBool:
		xs := make([]bool, len(values))
		for i, v := range values {
			b, ok := v.(bool)
			if !ok {
				return nil, false
			}
			xs[i] = b
		}
		return data.NewBool(xs), true
	case data.KindI64:
		xs := make([]int64, len(values))
		for i, v := range values {
			switch n := v.(type) {
			case int:
				xs[i] = int64(n)
			case int64:
				xs[i] = n
			default:
				return nil, false
			}
		}
		return data.NewI64(xs), true
	case data.KindF64:
		xs := make([]float64, len(values))
		for i, v := range values {
			n, ok := qNumericAny(v)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewF64(xs), true
	case data.KindString:
		xs := make([]string, len(values))
		for i, v := range values {
			s, ok := v.(string)
			if !ok {
				return nil, false
			}
			xs[i] = s
		}
		return data.NewString(xs), true
	case data.KindSymbol:
		xs := make([]string, len(values))
		for i, v := range values {
			switch s := v.(type) {
			case data.Symbol:
				xs[i] = string(s)
			case string:
				xs[i] = s
			default:
				return nil, false
			}
		}
		return data.NewSymbols(xs), true
	default:
		return nil, false
	}
}

func qCoerceDataValues(values []any, kind data.Kind) []any {
	out := append([]any(nil), values...)
	if kind == data.KindSymbol {
		for i, v := range out {
			if s, ok := v.(string); ok {
				out[i] = data.Symbol(s)
			}
		}
	}
	return out
}

func qNumericAny(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
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

func qDataFrameFromRowTable(rows *Table) (data.Frame, error) {
	if rows == nil || rows.Length() == 0 {
		return data.Frame{}, fmt.Errorf("frame requires at least one row")
	}
	first := rows.RawGetInt(1)
	if !first.IsTable() {
		return data.Frame{}, fmt.Errorf("frame rows must be tables")
	}
	names, err := qRowColumnNames(first.Table())
	if err != nil {
		return data.Frame{}, err
	}
	values := make(map[string][]any, len(names))
	for _, name := range names {
		values[name] = make([]any, 0, rows.Length())
	}
	for i := 1; i <= rows.Length(); i++ {
		rowValue := rows.RawGetInt(int64(i))
		if !rowValue.IsTable() {
			return data.Frame{}, fmt.Errorf("frame row %d must be a table", i)
		}
		row := rowValue.Table()
		for _, name := range names {
			values[name] = append(values[name], qValueToAny(row.RawGetString(name)))
		}
	}
	cols := make([]data.Column, 0, len(names))
	kinds := qFrameColumnKinds(rows)
	for _, name := range names {
		kind := data.Kind("")
		if kinds != nil {
			kind = kinds[name]
		}
		if kind != "" && kind != data.KindAny {
			if array, ok := qTypedDataArray(values[name], kind); ok {
				cols = append(cols, data.Column{Name: data.Symbol(name), Data: array})
				continue
			}
		}
		cols = append(cols, data.NewColumn(data.Symbol(name), values[name]))
	}
	return data.NewFrame(cols...)
}

func qRowColumnNames(row *Table) ([]string, error) {
	names := make([]string, 0, row.Length())
	ok := row.ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		names = append(names, key.Str())
		return true
	})
	if !ok {
		return nil, fmt.Errorf("frame row must be a plain string-keyed table")
	}
	sort.Strings(names)
	return names, nil
}

func qDataArrayFromDense(col *DenseArray) (data.Array, error) {
	switch col.DType() {
	case DenseArrayF64:
		xs := make([]float64, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Number()
		}
		return data.NewF64(xs), nil
	case DenseArrayI64:
		xs := make([]int64, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Int()
		}
		return data.NewI64(xs), nil
	case DenseArrayBool:
		xs := make([]bool, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Bool()
		}
		return data.NewBool(xs), nil
	default:
		return nil, fmt.Errorf("unsupported dense array type %s", col.DType())
	}
}

func qAnyValuesFromVector(v Value) ([]any, error) {
	if v.IsDenseArray() {
		col := v.DenseArray()
		out := make([]any, col.Len())
		for i := range out {
			item, err := col.At(i)
			if err != nil {
				return nil, err
			}
			out[i] = qValueToAny(item)
		}
		return out, nil
	}
	if v.IsTable() {
		tbl := v.Table()
		out := make([]any, tbl.Length())
		for i := range out {
			out[i] = qValueToAny(tbl.RawGetInt(int64(i + 1)))
		}
		return out, nil
	}
	return nil, fmt.Errorf("must be a dense array or array table")
}

func qRowsFromDataFrame(frame data.Frame) (*Table, error) {
	names := frame.Schema().Names()
	cols := NewTable()
	for _, name := range names {
		col, ok := frame.Column(name)
		if !ok {
			return nil, fmt.Errorf("column %q not found", name)
		}
		values := NewAppendArrayTable(frame.Len())
		for i := 0; i < frame.Len(); i++ {
			v, ok := col.At(i)
			if !ok {
				return nil, fmt.Errorf("column %q row %d out of range", name, i)
			}
			values.RawSetInt(int64(i+1), qAnyToColumnValue(v))
		}
		cols.RawSetString(string(name), dataColumnValue(col.Kind(), TableValue(values)))
	}
	frameValue, err := dataFrameFromColumns(cols)
	if err != nil {
		return nil, err
	}
	out := frameValue.Table()
	rowTable := NewAppendArrayTable(frame.Len())
	for i := 0; i < frame.Len(); i++ {
		row := NewTable()
		for _, name := range names {
			col, ok := frame.Column(name)
			if !ok {
				return nil, fmt.Errorf("column %q not found", name)
			}
			v, ok := col.At(i)
			if !ok {
				return nil, fmt.Errorf("column %q row %d out of range", name, i)
			}
			row.RawSetString(string(name), qAnyToValue(v))
		}
		rowValue := TableValue(row)
		out.RawSetInt(int64(i+1), rowValue)
		rowTable.RawSetInt(int64(i+1), rowValue)
	}
	qDecorateDataFrameTable(out, rowTable)
	return out, nil
}

func qDecorateDataFrameTable(frame, rows *Table) {
	if frame == nil {
		return
	}
	nrows := int64(0)
	if lenValue := frame.RawGetString("len"); lenValue.IsInt() {
		nrows = lenValue.Int()
	}
	ncols := int64(0)
	if names := frame.RawGetString("column_names"); names.IsTable() {
		ncols = int64(names.Table().Length())
	}
	columns := frame.RawGetString("columns")
	frame.RawSetString("kind", StringValue("data_frame"))
	frame.RawSetString("type", StringValue("data_frame"))
	if columns.IsTable() {
		frame.RawSetString("data", columns)
	}
	if rows != nil {
		frame.RawSetString("rows", TableValue(rows))
	}
	frame.RawSetString("nrows", IntValue(nrows))
	frame.RawSetString("ncols", IntValue(ncols))
	shape := NewTable()
	shape.RawSetString("rows", IntValue(nrows))
	shape.RawSetString("columns", IntValue(ncols))
	frame.RawSetString("shape", TableValue(shape))
}

func qValueToAny(v Value) any {
	switch {
	case v.IsNil():
		return nil
	case isDataNullValue(v):
		return data.NullValue
	case v.IsBool():
		return v.Bool()
	case v.IsInt():
		return v.Int()
	case v.IsFloat():
		return v.Float()
	case v.IsString():
		return v.Str()
	case v.IsTable() && v.Table().Length() == 0:
		return nil
	default:
		return v.String()
	}
}

func qAnyToValue(v any) Value {
	switch x := v.(type) {
	case nil:
		return NilValue()
	case data.Null:
		return NilValue()
	case bool:
		return BoolValue(x)
	case int:
		return IntValue(int64(x))
	case int64:
		return IntValue(x)
	case float32:
		return FloatValue(float64(x))
	case float64:
		return FloatValue(x)
	case string:
		return StringValue(x)
	case data.Symbol:
		return StringValue(string(x))
	default:
		return StringValue(fmt.Sprint(x))
	}
}

func qAnyToColumnValue(v any) Value {
	if data.IsNull(v) {
		return dataNullValue()
	}
	return qAnyToValue(v)
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
