// Package data provides Leia's runtime-independent columnar data foundation.
package data

import (
	"fmt"
	"sort"
)

type Kind string

const (
	KindAny       Kind = "any"
	KindNull      Kind = "null"
	KindBool      Kind = "bool"
	KindI8        Kind = "i8"
	KindI16       Kind = "i16"
	KindI32       Kind = "i32"
	KindI64       Kind = "i64"
	KindU8        Kind = "u8"
	KindU16       Kind = "u16"
	KindU32       Kind = "u32"
	KindU64       Kind = "u64"
	KindF32       Kind = "f32"
	KindF64       Kind = "f64"
	KindString    Kind = "string"
	KindSymbol    Kind = "symbol"
	KindDate      Kind = "date"
	KindTime      Kind = "time"
	KindTimestamp Kind = "timestamp"
)

// Symbol is Leia's categorical scalar. q symbols lower to this type, but the
// type is general-purpose and belongs to the data foundation.
type Symbol string

// Date is a calendar date encoded as days since 1970-01-01.
type Date int64

// Time is a time-of-day encoded as nanoseconds since midnight.
type Time int64

// Timestamp is an instant encoded as Unix nanoseconds.
type Timestamp int64

const nanosPerDay int64 = 24 * 60 * 60 * 1_000_000_000

func DateFromDays(days int64) Date { return Date(days) }

func (d Date) Days() int64 { return int64(d) }

func TimeFromNanos(nanos int64) Time { return Time(nanos) }

func (t Time) Nanos() int64 { return int64(t) }

func (t Time) Valid() bool { return t >= 0 && int64(t) < nanosPerDay }

func TimestampFromUnixNanos(nanos int64) Timestamp { return Timestamp(nanos) }

func (t Timestamp) UnixNanos() int64 { return int64(t) }

// Null is Leia's missing scalar. Use NullValue for explicit nulls; nil inputs
// are accepted by constructors and normalized to NullValue in arrays.
type Null struct{}

var NullValue = Null{}

func IsNull(v any) bool {
	if v == nil {
		return true
	}
	_, ok := v.(Null)
	return ok
}

type Array interface {
	Kind() Kind
	Len() int
	At(row int) (any, bool)
	Values() []any
	Gather(indexes []int) Array
}

type columnArray[T any] struct {
	kind Kind
	data []T
}

type nullableArray struct {
	kind Kind
	data []any
}

func NewBool(values []bool) Array {
	return columnArray[bool]{kind: KindBool, data: append([]bool(nil), values...)}
}

func NewI8(values []int8) Array {
	return columnArray[int8]{kind: KindI8, data: append([]int8(nil), values...)}
}

func NewI16(values []int16) Array {
	return columnArray[int16]{kind: KindI16, data: append([]int16(nil), values...)}
}

func NewI32(values []int32) Array {
	return columnArray[int32]{kind: KindI32, data: append([]int32(nil), values...)}
}

func NewI64(values []int64) Array {
	return columnArray[int64]{kind: KindI64, data: append([]int64(nil), values...)}
}

func NewU8(values []uint8) Array {
	return columnArray[uint8]{kind: KindU8, data: append([]uint8(nil), values...)}
}

func NewU16(values []uint16) Array {
	return columnArray[uint16]{kind: KindU16, data: append([]uint16(nil), values...)}
}

func NewU32(values []uint32) Array {
	return columnArray[uint32]{kind: KindU32, data: append([]uint32(nil), values...)}
}

func NewU64(values []uint64) Array {
	return columnArray[uint64]{kind: KindU64, data: append([]uint64(nil), values...)}
}

func NewF32(values []float32) Array {
	return columnArray[float32]{kind: KindF32, data: append([]float32(nil), values...)}
}

func NewF64(values []float64) Array {
	return columnArray[float64]{kind: KindF64, data: append([]float64(nil), values...)}
}

func NewString(values []string) Array {
	return columnArray[string]{kind: KindString, data: append([]string(nil), values...)}
}

func NewSymbols(values []string) Array {
	out := make([]Symbol, len(values))
	for i, value := range values {
		out[i] = Symbol(value)
	}
	return columnArray[Symbol]{kind: KindSymbol, data: out}
}

func NewDate(values []Date) Array {
	return columnArray[Date]{kind: KindDate, data: append([]Date(nil), values...)}
}

func NewTime(values []Time) Array {
	return columnArray[Time]{kind: KindTime, data: append([]Time(nil), values...)}
}

func NewTimestamp(values []Timestamp) Array {
	return columnArray[Timestamp]{kind: KindTimestamp, data: append([]Timestamp(nil), values...)}
}

func NewAny(values []any) Array {
	return nullableArray{kind: KindAny, data: normalizeNulls(values)}
}

func (a columnArray[T]) Kind() Kind { return a.kind }

func (a columnArray[T]) Len() int { return len(a.data) }

func (a columnArray[T]) At(row int) (any, bool) {
	if row < 0 || row >= len(a.data) {
		return nil, false
	}
	return a.data[row], true
}

func (a columnArray[T]) Values() []any {
	out := make([]any, len(a.data))
	for i, v := range a.data {
		out[i] = v
	}
	return out
}

func (a columnArray[T]) Gather(indexes []int) Array {
	out := make([]T, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= len(a.data) {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		out[i] = a.data[row]
	}
	return columnArray[T]{kind: a.kind, data: out}
}

func (a nullableArray) Kind() Kind { return a.kind }

func (a nullableArray) Len() int { return len(a.data) }

func (a nullableArray) At(row int) (any, bool) {
	if row < 0 || row >= len(a.data) {
		return nil, false
	}
	return a.data[row], true
}

func (a nullableArray) Values() []any {
	return append([]any(nil), a.data...)
}

func (a nullableArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		if row < 0 || row >= len(a.data) {
			panic(fmt.Sprintf("data array gather index %d out of range", row))
		}
		out[i] = a.data[row]
	}
	return nullableArray{kind: a.kind, data: out}
}

type Column struct {
	Name Symbol
	Data Array
}

func NewColumn(name Symbol, values []any) Column {
	return Column{Name: name, Data: InferArray(values)}
}

type Schema struct {
	names []Symbol
	kinds map[Symbol]Kind
}

func (s Schema) Names() []Symbol {
	return append([]Symbol(nil), s.names...)
}

func (s Schema) Kind(name Symbol) (Kind, bool) {
	kind, ok := s.kinds[name]
	return kind, ok
}

type Frame struct {
	schema  Schema
	columns map[Symbol]Array
	rows    int
}

func NewFrame(cols ...Column) (Frame, error) {
	if len(cols) == 0 {
		return Frame{}, fmt.Errorf("frame requires at least one column")
	}
	frame := Frame{
		schema: Schema{
			names: make([]Symbol, 0, len(cols)),
			kinds: make(map[Symbol]Kind, len(cols)),
		},
		columns: make(map[Symbol]Array, len(cols)),
		rows:    -1,
	}
	for i, col := range cols {
		if col.Name == "" {
			return Frame{}, fmt.Errorf("frame column %d name must not be empty", i+1)
		}
		if col.Data == nil {
			return Frame{}, fmt.Errorf("frame column %q is nil", col.Name)
		}
		if _, exists := frame.columns[col.Name]; exists {
			return Frame{}, fmt.Errorf("frame column %q is duplicated", col.Name)
		}
		if frame.rows < 0 {
			frame.rows = col.Data.Len()
		} else if frame.rows != col.Data.Len() {
			return Frame{}, fmt.Errorf("frame column %q length %d does not match frame length %d", col.Name, col.Data.Len(), frame.rows)
		}
		frame.schema.names = append(frame.schema.names, col.Name)
		frame.schema.kinds[col.Name] = col.Data.Kind()
		frame.columns[col.Name] = col.Data.Gather(allIndexes(col.Data.Len()))
	}
	return frame, nil
}

func (f Frame) Len() int { return f.rows }

func (f Frame) Schema() Schema {
	kinds := make(map[Symbol]Kind, len(f.schema.kinds))
	for name, kind := range f.schema.kinds {
		kinds[name] = kind
	}
	return Schema{names: append([]Symbol(nil), f.schema.names...), kinds: kinds}
}

func (f Frame) Column(name Symbol) (Array, bool) {
	col, ok := f.columns[name]
	return col, ok
}

func (f Frame) Row(row int) (map[Symbol]any, error) {
	if row < 0 || row >= f.rows {
		return nil, fmt.Errorf("frame row index %d out of range", row)
	}
	out := make(map[Symbol]any, len(f.schema.names))
	for _, name := range f.schema.names {
		v, _ := f.columns[name].At(row)
		out[name] = v
	}
	return out, nil
}

func (f Frame) Gather(indexes []int) (Frame, error) {
	cols := make([]Column, 0, len(f.schema.names))
	for _, name := range f.schema.names {
		cols = append(cols, Column{Name: name, Data: f.columns[name].Gather(indexes)})
	}
	return NewFrame(cols...)
}

type Expr interface {
	EvalRow(frame Frame, row int) (any, error)
}

type ColumnRef struct {
	Name Symbol
}

func (e ColumnRef) EvalRow(frame Frame, row int) (any, error) {
	col, ok := frame.Column(e.Name)
	if !ok {
		return nil, fmt.Errorf("unknown column %q", e.Name)
	}
	v, ok := col.At(row)
	if !ok {
		return nil, fmt.Errorf("column %q row %d out of range", e.Name, row)
	}
	return v, nil
}

type Literal struct {
	Value any
}

func (e Literal) EvalRow(Frame, int) (any, error) { return e.Value, nil }

type Op string

const (
	OpAdd Op = "+"
	OpSub Op = "-"
	OpMul Op = "*"
	OpDiv Op = "/"
	OpEQ  Op = "="
	OpNE  Op = "!="
	OpLT  Op = "<"
	OpLE  Op = "<="
	OpGT  Op = ">"
	OpGE  Op = ">="
)

type Binary struct {
	Op          Op
	Left, Right Expr
}

func (e Binary) EvalRow(frame Frame, row int) (any, error) {
	left, err := e.Left.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	right, err := e.Right.EvalRow(frame, row)
	if err != nil {
		return nil, err
	}
	return ApplyBinary(e.Op, left, right)
}

func ApplyBinary(op Op, left, right any) (any, error) {
	switch op {
	case OpEQ:
		return equalScalar(left, right), nil
	case OpNE:
		return !equalScalar(left, right), nil
	}
	if cmp, ok := compareSameKind(left, right); ok {
		switch op {
		case OpLT:
			return cmp < 0, nil
		case OpLE:
			return cmp <= 0, nil
		case OpGT:
			return cmp > 0, nil
		case OpGE:
			return cmp >= 0, nil
		}
	}
	lf, lok := numeric(left)
	rf, rok := numeric(right)
	if !lok || !rok {
		return nil, fmt.Errorf("operator %s expects numeric operands", op)
	}
	switch op {
	case OpAdd:
		return lf + rf, nil
	case OpSub:
		return lf - rf, nil
	case OpMul:
		return lf * rf, nil
	case OpDiv:
		return lf / rf, nil
	case OpLT:
		return lf < rf, nil
	case OpLE:
		return lf <= rf, nil
	case OpGT:
		return lf > rf, nil
	case OpGE:
		return lf >= rf, nil
	default:
		return nil, fmt.Errorf("unsupported operator %s", op)
	}
}

type SelectItem struct {
	Name Symbol
	Expr Expr
}

type Aggregate struct {
	Name Symbol
	Func string
	Expr Expr
}

type OrderSpec struct {
	Column Symbol
	Desc   bool
}

type SortDirection int

const (
	Asc SortDirection = iota
	Desc
)

type QueryPlan struct {
	Source     Frame
	Where      Expr
	By         []Symbol
	Select     []SelectItem
	Aggregates []Aggregate
	OrderBy    []OrderSpec
	LimitN     int
}

func From(frame Frame) QueryPlan {
	return QueryPlan{Source: frame, LimitN: -1}
}

func (p QueryPlan) WhereExpr(expr Expr) QueryPlan {
	p.Where = expr
	return p
}

func (p QueryPlan) WhereEq(name Symbol, value any) QueryPlan {
	p.Where = Binary{Op: OpEQ, Left: ColumnRef{Name: name}, Right: Literal{Value: value}}
	return p
}

func (p QueryPlan) SelectColumns(names ...Symbol) QueryPlan {
	p.Select = p.Select[:0]
	for _, name := range names {
		p.Select = append(p.Select, SelectItem{Name: name, Expr: ColumnRef{Name: name}})
	}
	return p
}

func (p QueryPlan) GroupBy(names ...Symbol) QueryPlan {
	p.By = append([]Symbol(nil), names...)
	return p
}

func (p QueryPlan) Sum(source, as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "sum", Expr: ColumnRef{Name: source}})
	return p
}

func (p QueryPlan) Count(as Symbol) QueryPlan {
	p.Aggregates = append(p.Aggregates, Aggregate{Name: as, Func: "count"})
	return p
}

func (p QueryPlan) OrderByColumn(name Symbol, dir SortDirection) QueryPlan {
	p.OrderBy = []OrderSpec{{Column: name, Desc: dir == Desc}}
	return p
}

func (p QueryPlan) Limit(n int) QueryPlan {
	p.LimitN = n
	return p
}

func (p QueryPlan) Exec() (Frame, error) {
	return Exec(p.Source, p)
}

func Exec(frame Frame, plan QueryPlan) (Frame, error) {
	if frame.Len() < 0 {
		return Frame{}, fmt.Errorf("query frame is empty")
	}
	indexes, err := filterIndexes(frame, plan.Where)
	if err != nil {
		return Frame{}, err
	}
	var out Frame
	if len(plan.By) > 0 || len(plan.Aggregates) > 0 {
		out, err = execGrouped(frame, indexes, plan)
	} else {
		out, err = execProject(frame, indexes, plan.Select)
	}
	if err != nil {
		return Frame{}, err
	}
	if len(plan.OrderBy) > 0 {
		out, err = orderFrame(out, plan.OrderBy)
		if err != nil {
			return Frame{}, err
		}
	}
	if plan.LimitN >= 0 && plan.LimitN < out.Len() {
		return out.Gather(allIndexes(plan.LimitN))
	}
	return out, nil
}

func filterIndexes(frame Frame, where Expr) ([]int, error) {
	indexes := make([]int, 0, frame.Len())
	for i := 0; i < frame.Len(); i++ {
		if where == nil {
			indexes = append(indexes, i)
			continue
		}
		v, err := where.EvalRow(frame, i)
		if err != nil {
			return nil, err
		}
		keep, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("where expression must evaluate to bool")
		}
		if keep {
			indexes = append(indexes, i)
		}
	}
	return indexes, nil
}

func execProject(frame Frame, indexes []int, items []SelectItem) (Frame, error) {
	if len(items) == 0 {
		items = make([]SelectItem, 0, len(frame.schema.names))
		for _, name := range frame.schema.names {
			items = append(items, SelectItem{Name: name, Expr: ColumnRef{Name: name}})
		}
	}
	cols := make([]Column, 0, len(items))
	for _, item := range items {
		if item.Name == "" {
			return Frame{}, fmt.Errorf("select item name must not be empty")
		}
		values := make([]any, len(indexes))
		for i, row := range indexes {
			v, err := item.Expr.EvalRow(frame, row)
			if err != nil {
				return Frame{}, err
			}
			values[i] = v
		}
		cols = append(cols, NewColumn(item.Name, values))
	}
	return NewFrame(cols...)
}

type groupState struct {
	keys []any
	aggs []aggregateState
}

type aggregateState struct {
	fn    string
	sum   float64
	count int64
}

func execGrouped(frame Frame, indexes []int, plan QueryPlan) (Frame, error) {
	if len(plan.By) == 0 {
		return Frame{}, fmt.Errorf("aggregate queries require at least one by column")
	}
	if len(plan.Aggregates) == 0 {
		return Frame{}, fmt.Errorf("grouped queries require aggregates")
	}
	groups := map[string]*groupState{}
	order := make([]string, 0)
	for _, row := range indexes {
		keyVals := make([]any, len(plan.By))
		key := ""
		for i, name := range plan.By {
			v, err := (ColumnRef{Name: name}).EvalRow(frame, row)
			if err != nil {
				return Frame{}, err
			}
			keyVals[i] = v
			key += fmt.Sprintf("%#v\x00", v)
		}
		state := groups[key]
		if state == nil {
			state = &groupState{keys: keyVals, aggs: make([]aggregateState, len(plan.Aggregates))}
			for i, agg := range plan.Aggregates {
				state.aggs[i].fn = agg.Func
			}
			groups[key] = state
			order = append(order, key)
		}
		for i, agg := range plan.Aggregates {
			switch agg.Func {
			case "count":
				state.aggs[i].count++
			case "sum", "avg":
				v, err := agg.Expr.EvalRow(frame, row)
				if err != nil {
					return Frame{}, err
				}
				n, ok := numeric(v)
				if !ok {
					return Frame{}, fmt.Errorf("aggregate %s expects numeric expression", agg.Func)
				}
				state.aggs[i].sum += n
				state.aggs[i].count++
			default:
				return Frame{}, fmt.Errorf("unsupported aggregate %q", agg.Func)
			}
		}
	}

	cols := make([]Column, 0, len(plan.By)+len(plan.Aggregates))
	for i, name := range plan.By {
		values := make([]any, len(order))
		for row, key := range order {
			values[row] = groups[key].keys[i]
		}
		cols = append(cols, NewColumn(name, values))
	}
	for i, agg := range plan.Aggregates {
		values := make([]any, len(order))
		for row, key := range order {
			state := groups[key].aggs[i]
			switch agg.Func {
			case "count":
				values[row] = state.count
			case "sum":
				values[row] = state.sum
			case "avg":
				if state.count == 0 {
					values[row] = float64(0)
				} else {
					values[row] = state.sum / float64(state.count)
				}
			}
		}
		cols = append(cols, NewColumn(agg.Name, values))
	}
	return NewFrame(cols...)
}

func orderFrame(frame Frame, specs []OrderSpec) (Frame, error) {
	for _, spec := range specs {
		if _, ok := frame.Column(spec.Column); !ok {
			return Frame{}, fmt.Errorf("order column %q does not exist", spec.Column)
		}
	}
	indexes := allIndexes(frame.Len())
	sort.SliceStable(indexes, func(i, j int) bool {
		leftRow, rightRow := indexes[i], indexes[j]
		for _, spec := range specs {
			col, _ := frame.Column(spec.Column)
			lv, _ := col.At(leftRow)
			rv, _ := col.At(rightRow)
			cmp := compare(lv, rv)
			if cmp == 0 {
				continue
			}
			if spec.Desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
	return frame.Gather(indexes)
}

func InferArray(values []any) Array {
	hasNull, hasValue := false, false
	allBool := true
	allI8, allI16, allI32, allI64 := true, true, true, true
	allU8, allU16, allU32, allU64 := true, true, true, true
	allF32, allF64, allNumber := true, true, true
	allString, allSymbol := true, true
	allDate, allTime, allTimestamp := true, true, true
	for _, v := range values {
		if IsNull(v) {
			hasNull = true
			continue
		}
		hasValue = true
		if _, ok := v.(bool); !ok {
			allBool = false
		}
		if _, ok := v.(int8); !ok {
			allI8 = false
		}
		if _, ok := v.(int16); !ok {
			allI16 = false
		}
		if _, ok := v.(int32); !ok {
			allI32 = false
		}
		switch v.(type) {
		case int, int64:
		default:
			allI64 = false
		}
		if _, ok := v.(uint8); !ok {
			allU8 = false
		}
		if _, ok := v.(uint16); !ok {
			allU16 = false
		}
		if _, ok := v.(uint32); !ok {
			allU32 = false
		}
		if _, ok := v.(uint64); !ok {
			allU64 = false
		}
		if _, ok := v.(float32); !ok {
			allF32 = false
		}
		if _, ok := v.(float64); !ok {
			allF64 = false
		}
		if _, ok := numeric(v); !ok {
			allNumber = false
		}
		if _, ok := v.(string); !ok {
			allString = false
		}
		if _, ok := v.(Symbol); !ok {
			allSymbol = false
		}
		if _, ok := v.(Date); !ok {
			allDate = false
		}
		if _, ok := v.(Time); !ok {
			allTime = false
		}
		if _, ok := v.(Timestamp); !ok {
			allTimestamp = false
		}
	}
	if !hasValue {
		return nullableArray{kind: KindNull, data: normalizeNulls(values)}
	}
	switch {
	case allBool:
		out := make([]bool, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(bool)
		}
		if hasNull {
			return newNullableArray(KindBool, values)
		}
		return NewBool(out)
	case allI8:
		out := make([]int8, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(int8)
		}
		if hasNull {
			return newNullableArray(KindI8, values)
		}
		return NewI8(out)
	case allI16:
		out := make([]int16, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(int16)
		}
		if hasNull {
			return newNullableArray(KindI16, values)
		}
		return NewI16(out)
	case allI32:
		out := make([]int32, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(int32)
		}
		if hasNull {
			return newNullableArray(KindI32, values)
		}
		return NewI32(out)
	case allI64:
		out := make([]int64, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			switch x := v.(type) {
			case int:
				out[i] = int64(x)
			case int64:
				out[i] = x
			}
		}
		if hasNull {
			return newNullableArray(KindI64, values)
		}
		return NewI64(out)
	case allU8:
		out := make([]uint8, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(uint8)
		}
		if hasNull {
			return newNullableArray(KindU8, values)
		}
		return NewU8(out)
	case allU16:
		out := make([]uint16, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(uint16)
		}
		if hasNull {
			return newNullableArray(KindU16, values)
		}
		return NewU16(out)
	case allU32:
		out := make([]uint32, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(uint32)
		}
		if hasNull {
			return newNullableArray(KindU32, values)
		}
		return NewU32(out)
	case allU64:
		out := make([]uint64, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(uint64)
		}
		if hasNull {
			return newNullableArray(KindU64, values)
		}
		return NewU64(out)
	case allF32:
		out := make([]float32, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(float32)
		}
		if hasNull {
			return newNullableArray(KindF32, values)
		}
		return NewF32(out)
	case allF64:
		out := make([]float64, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(float64)
		}
		if hasNull {
			return newNullableArray(KindF64, values)
		}
		return NewF64(out)
	case allNumber:
		out := make([]float64, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i], _ = numeric(v)
		}
		if hasNull {
			return newNullableArray(KindF64, values)
		}
		return NewF64(out)
	case allString:
		out := make([]string, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(string)
		}
		if hasNull {
			return newNullableArray(KindString, values)
		}
		return NewString(out)
	case allSymbol:
		out := make([]string, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = string(v.(Symbol))
		}
		if hasNull {
			return newNullableArray(KindSymbol, values)
		}
		return NewSymbols(out)
	case allDate:
		out := make([]Date, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Date)
		}
		if hasNull {
			return newNullableArray(KindDate, values)
		}
		return NewDate(out)
	case allTime:
		out := make([]Time, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Time)
		}
		if hasNull {
			return newNullableArray(KindTime, values)
		}
		return NewTime(out)
	case allTimestamp:
		out := make([]Timestamp, len(values))
		for i, v := range values {
			if IsNull(v) {
				continue
			}
			out[i] = v.(Timestamp)
		}
		if hasNull {
			return newNullableArray(KindTimestamp, values)
		}
		return NewTimestamp(out)
	default:
		return NewAny(values)
	}
}

func newNullableArray(kind Kind, values []any) Array {
	out := make([]any, len(values))
	for i, v := range values {
		if IsNull(v) {
			out[i] = NullValue
			continue
		}
		out[i] = normalizeScalar(kind, v)
	}
	return nullableArray{kind: kind, data: out}
}

func normalizeNulls(values []any) []any {
	out := make([]any, len(values))
	for i, v := range values {
		if IsNull(v) {
			out[i] = NullValue
		} else {
			out[i] = v
		}
	}
	return out
}

func normalizeScalar(kind Kind, v any) any {
	switch kind {
	case KindI8:
		return v.(int8)
	case KindI16:
		return v.(int16)
	case KindI32:
		return v.(int32)
	case KindI64:
		if n, ok := v.(int); ok {
			return int64(n)
		}
		return v.(int64)
	case KindU8:
		return v.(uint8)
	case KindU16:
		return v.(uint16)
	case KindU32:
		return v.(uint32)
	case KindU64:
		return v.(uint64)
	case KindF32:
		return v.(float32)
	case KindF64:
		n, _ := numeric(v)
		return n
	}
	return v
}

func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

func equalScalar(left, right any) bool {
	if IsNull(left) || IsNull(right) {
		return IsNull(left) && IsNull(right)
	}
	if cmp, ok := compareSameKind(left, right); ok {
		return cmp == 0
	}
	switch l := left.(type) {
	case Symbol:
		r, ok := right.(Symbol)
		return ok && l == r
	case string:
		return l == right
	default:
		return left == right
	}
}

func compareSameKind(left, right any) (int, bool) {
	switch l := left.(type) {
	case bool:
		r, ok := right.(bool)
		if !ok {
			return 0, false
		}
		switch {
		case l == r:
			return 0, true
		case !l && r:
			return -1, true
		default:
			return 1, true
		}
	case int8:
		r, ok := right.(int8)
		return compareInt64(int64(l), int64(r)), ok
	case int16:
		r, ok := right.(int16)
		return compareInt64(int64(l), int64(r)), ok
	case int32:
		r, ok := right.(int32)
		return compareInt64(int64(l), int64(r)), ok
	case int64:
		r, ok := right.(int64)
		return compareInt64(l, r), ok
	case uint8:
		r, ok := right.(uint8)
		return compareUint64(uint64(l), uint64(r)), ok
	case uint16:
		r, ok := right.(uint16)
		return compareUint64(uint64(l), uint64(r)), ok
	case uint32:
		r, ok := right.(uint32)
		return compareUint64(uint64(l), uint64(r)), ok
	case uint64:
		r, ok := right.(uint64)
		return compareUint64(l, r), ok
	case float32:
		r, ok := right.(float32)
		return compareFloat64(float64(l), float64(r)), ok
	case float64:
		r, ok := right.(float64)
		return compareFloat64(l, r), ok
	case Date:
		r, ok := right.(Date)
		return compareInt64(int64(l), int64(r)), ok
	case Time:
		r, ok := right.(Time)
		return compareInt64(int64(l), int64(r)), ok
	case Timestamp:
		r, ok := right.(Timestamp)
		return compareInt64(int64(l), int64(r)), ok
	case Symbol:
		r, ok := right.(Symbol)
		return compareString(string(l), string(r)), ok
	case string:
		r, ok := right.(string)
		return compareString(l, r), ok
	}
	return 0, false
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

func compareUint64(left, right uint64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareFloat64(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareString(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compare(left, right any) int {
	if IsNull(left) || IsNull(right) {
		switch {
		case IsNull(left) && IsNull(right):
			return 0
		case IsNull(left):
			return -1
		default:
			return 1
		}
	}
	if cmp, ok := compareSameKind(left, right); ok {
		return cmp
	}
	if lf, ok := numeric(left); ok {
		if rf, ok := numeric(right); ok {
			switch {
			case lf < rf:
				return -1
			case lf > rf:
				return 1
			default:
				return 0
			}
		}
	}
	ls, rs := fmt.Sprint(left), fmt.Sprint(right)
	switch {
	case ls < rs:
		return -1
	case ls > rs:
		return 1
	default:
		return 0
	}
}

func allIndexes(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}
