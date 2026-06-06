package data

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewFrameRejectsUnequalColumnLengths(t *testing.T) {
	_, err := NewFrame(
		NewColumn("a", []any{1, 2}),
		NewColumn("b", []any{1}),
	)
	if err == nil {
		t.Fatal("NewFrame accepted columns with unequal lengths")
	}
	if !strings.Contains(err.Error(), "length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewFramePreservesColumnOrderAndKinds(t *testing.T) {
	frame, err := NewFrame(
		NewColumn("z", []any{int64(3)}),
		NewColumn("a", []any{"x"}),
		NewColumn("m", []any{Symbol("MSFT")}),
	)
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}

	if got, want := frame.Schema().Names(), []Symbol{"z", "a", "m"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("column order = %v, want %v", got, want)
	}
	if got, _ := frame.Schema().Kind("m"); got != KindSymbol {
		t.Fatalf("kind(m) = %s, want %s", got, KindSymbol)
	}
}

func TestInferArrayRetainsTypedKindsWithNulls(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		kind   Kind
		want   []any
	}{
		{name: "bool", values: []any{true, nil, false}, kind: KindBool, want: []any{true, NullValue, false}},
		{name: "i8", values: []any{int8(1), nil, int8(3)}, kind: KindI8, want: []any{int8(1), NullValue, int8(3)}},
		{name: "i16", values: []any{int16(1), nil, int16(3)}, kind: KindI16, want: []any{int16(1), NullValue, int16(3)}},
		{name: "i32", values: []any{int32(1), nil, int32(3)}, kind: KindI32, want: []any{int32(1), NullValue, int32(3)}},
		{name: "i64", values: []any{1, nil, int64(3)}, kind: KindI64, want: []any{int64(1), NullValue, int64(3)}},
		{name: "u8", values: []any{uint8(1), nil, uint8(3)}, kind: KindU8, want: []any{uint8(1), NullValue, uint8(3)}},
		{name: "u16", values: []any{uint16(1), nil, uint16(3)}, kind: KindU16, want: []any{uint16(1), NullValue, uint16(3)}},
		{name: "u32", values: []any{uint32(1), nil, uint32(3)}, kind: KindU32, want: []any{uint32(1), NullValue, uint32(3)}},
		{name: "u64", values: []any{uint64(1), nil, uint64(3)}, kind: KindU64, want: []any{uint64(1), NullValue, uint64(3)}},
		{name: "f32", values: []any{float32(1.5), nil, float32(2.5)}, kind: KindF32, want: []any{float32(1.5), NullValue, float32(2.5)}},
		{name: "f64", values: []any{1.5, nil, float32(2.5)}, kind: KindF64, want: []any{1.5, NullValue, 2.5}},
		{name: "string", values: []any{"a", nil, "b"}, kind: KindString, want: []any{"a", NullValue, "b"}},
		{name: "symbol", values: []any{Symbol("a"), nil, Symbol("b")}, kind: KindSymbol, want: []any{Symbol("a"), NullValue, Symbol("b")}},
		{name: "date", values: []any{DateFromDays(1), nil, DateFromDays(3)}, kind: KindDate, want: []any{Date(1), NullValue, Date(3)}},
		{name: "time", values: []any{TimeFromNanos(10), nil, TimeFromNanos(30)}, kind: KindTime, want: []any{Time(10), NullValue, Time(30)}},
		{name: "timestamp", values: []any{TimestampFromUnixNanos(100), nil, TimestampFromUnixNanos(300)}, kind: KindTimestamp, want: []any{Timestamp(100), NullValue, Timestamp(300)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := NewColumn(Symbol(tt.name), tt.values)
			if got := col.Data.Kind(); got != tt.kind {
				t.Fatalf("kind = %s, want %s", got, tt.kind)
			}
			if got := col.Data.Values(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("values = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNumericConstructors(t *testing.T) {
	tests := []struct {
		name string
		got  Array
		kind Kind
		want []any
	}{
		{name: "i8", got: NewI8([]int8{1, 2}), kind: KindI8, want: []any{int8(1), int8(2)}},
		{name: "i16", got: NewI16([]int16{1, 2}), kind: KindI16, want: []any{int16(1), int16(2)}},
		{name: "i32", got: NewI32([]int32{1, 2}), kind: KindI32, want: []any{int32(1), int32(2)}},
		{name: "u8", got: NewU8([]uint8{1, 2}), kind: KindU8, want: []any{uint8(1), uint8(2)}},
		{name: "u16", got: NewU16([]uint16{1, 2}), kind: KindU16, want: []any{uint16(1), uint16(2)}},
		{name: "u32", got: NewU32([]uint32{1, 2}), kind: KindU32, want: []any{uint32(1), uint32(2)}},
		{name: "u64", got: NewU64([]uint64{1, 2}), kind: KindU64, want: []any{uint64(1), uint64(2)}},
		{name: "f32", got: NewF32([]float32{1.5, 2.5}), kind: KindF32, want: []any{float32(1.5), float32(2.5)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got.Kind(); got != tt.kind {
				t.Fatalf("kind = %s, want %s", got, tt.kind)
			}
			if got := tt.got.Values(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("values = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTemporalConstructors(t *testing.T) {
	date := NewDate([]Date{DateFromDays(1), DateFromDays(2)})
	if got := date.Kind(); got != KindDate {
		t.Fatalf("date kind = %s, want %s", got, KindDate)
	}
	if got, want := date.Values(), []any{Date(1), Date(2)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("date values = %#v, want %#v", got, want)
	}

	time := NewTime([]Time{TimeFromNanos(10), TimeFromNanos(20)})
	if got := time.Kind(); got != KindTime {
		t.Fatalf("time kind = %s, want %s", got, KindTime)
	}
	if !TimeFromNanos(20).Valid() {
		t.Fatal("expected in-day time to be valid")
	}
	if TimeFromNanos(24 * 60 * 60 * 1_000_000_000).Valid() {
		t.Fatal("expected next-midnight time to be invalid")
	}

	timestamp := NewTimestamp([]Timestamp{TimestampFromUnixNanos(100), TimestampFromUnixNanos(200)})
	if got := timestamp.Kind(); got != KindTimestamp {
		t.Fatalf("timestamp kind = %s, want %s", got, KindTimestamp)
	}
	if got, want := timestamp.Values(), []any{Timestamp(100), Timestamp(200)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("timestamp values = %#v, want %#v", got, want)
	}
}

func TestInferArrayNullAndMixedKinds(t *testing.T) {
	nulls := NewColumn("missing", []any{nil, NullValue})
	if got := nulls.Data.Kind(); got != KindNull {
		t.Fatalf("all-null kind = %s, want %s", got, KindNull)
	}
	if got, want := nulls.Data.Values(), []any{NullValue, NullValue}; !reflect.DeepEqual(got, want) {
		t.Fatalf("all-null values = %#v, want %#v", got, want)
	}

	mixed := NewColumn("mixed", []any{"a", nil, Symbol("a")})
	if got := mixed.Data.Kind(); got != KindAny {
		t.Fatalf("mixed string/symbol kind = %s, want %s", got, KindAny)
	}
}

func TestFoundationNumericEqualityAndComparison(t *testing.T) {
	tests := []struct {
		name        string
		op          Op
		left, right any
		want        any
	}{
		{name: "i32 less", op: OpLT, left: int32(-2), right: int32(3), want: true},
		{name: "i32 equal i32", op: OpEQ, left: int32(7), right: int32(7), want: true},
		{name: "i32 not equal i64", op: OpEQ, left: int32(7), right: int64(7), want: false},
		{name: "u64 greater preserves integer order", op: OpGT, left: uint64(1 << 63), right: uint64(1<<63 - 1), want: true},
		{name: "f32 less or equal", op: OpLE, left: float32(1.25), right: float32(1.5), want: true},
		{name: "f32 equal f32", op: OpEQ, left: float32(2.5), right: float32(2.5), want: true},
		{name: "f32 not equal f64", op: OpEQ, left: float32(2.5), right: float64(2.5), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyBinary(tt.op, tt.left, tt.right)
			if err != nil {
				t.Fatalf("ApplyBinary returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ApplyBinary = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTemporalEqualityAndComparison(t *testing.T) {
	tests := []struct {
		name        string
		op          Op
		left, right any
		want        any
	}{
		{name: "date equal", op: OpEQ, left: DateFromDays(2), right: DateFromDays(2), want: true},
		{name: "date not equal to i64", op: OpEQ, left: DateFromDays(2), right: int64(2), want: false},
		{name: "time less", op: OpLT, left: TimeFromNanos(1), right: TimeFromNanos(2), want: true},
		{name: "timestamp greater or equal", op: OpGE, left: TimestampFromUnixNanos(5), right: TimestampFromUnixNanos(5), want: true},
		{name: "null equal null", op: OpEQ, left: nil, right: NullValue, want: true},
		{name: "null not equal date", op: OpNE, left: nil, right: DateFromDays(0), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyBinary(tt.op, tt.left, tt.right)
			if err != nil {
				t.Fatalf("ApplyBinary returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ApplyBinary = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSymbolAndStringAreDistinctScalars(t *testing.T) {
	got, err := ApplyBinary(OpEQ, Symbol("a"), "a")
	if err != nil {
		t.Fatalf("ApplyBinary returned error: %v", err)
	}
	if got != false {
		t.Fatalf("Symbol(\"a\") = \"a\" returned %v, want false", got)
	}
}

func TestQueryBoolComparisonAndGrouping(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("active", []any{true, false, true, nil}),
		NewColumn("qty", []any{2, 5, 3, 7}),
	)

	filtered, err := From(frame).
		WhereEq("active", true).
		SelectColumns("qty").
		Exec()
	if err != nil {
		t.Fatalf("filtered Exec returned error: %v", err)
	}
	assertColumnValues(t, filtered, "qty", []any{int64(2), int64(3)})

	grouped, err := From(frame).
		GroupBy("active").
		Count("n").
		OrderByColumn("active", Asc).
		Exec()
	if err != nil {
		t.Fatalf("grouped Exec returned error: %v", err)
	}
	assertColumnValues(t, grouped, "active", []any{NullValue, false, true})
	assertColumnValues(t, grouped, "n", []any{int64(1), int64(1), int64(2)})
}

func TestQueryFoundationNumericComparisonAndOrder(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("qty", []any{int32(3), nil, int32(1), int32(3)}),
		NewColumn("seq", []any{uint64(3), uint64(1 << 63), uint64(2), uint64(1<<63 - 1)}),
		NewColumn("score", []any{float32(2.5), float32(1.5), float32(3.5), float32(2.5)}),
	)

	filtered, err := From(frame).
		WhereEq("qty", int32(3)).
		SelectColumns("seq", "score").
		OrderByColumn("seq", Asc).
		Exec()
	if err != nil {
		t.Fatalf("filtered Exec returned error: %v", err)
	}
	assertColumnValues(t, filtered, "seq", []any{uint64(3), uint64(1<<63 - 1)})
	assertColumnValues(t, filtered, "score", []any{float32(2.5), float32(2.5)})

	ordered, err := From(frame).
		SelectColumns("score", "qty").
		OrderByColumn("score", Desc).
		Exec()
	if err != nil {
		t.Fatalf("ordered Exec returned error: %v", err)
	}
	assertColumnValues(t, ordered, "score", []any{float32(3.5), float32(2.5), float32(2.5), float32(1.5)})
	assertColumnValues(t, ordered, "qty", []any{int32(1), int32(3), int32(3), NullValue})
}

func TestQueryTemporalComparisonGroupingAndOrder(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("d", []any{DateFromDays(2), nil, DateFromDays(1), DateFromDays(2)}),
		NewColumn("ts", []any{
			TimestampFromUnixNanos(20),
			TimestampFromUnixNanos(10),
			TimestampFromUnixNanos(30),
			TimestampFromUnixNanos(40),
		}),
	)

	filtered, err := From(frame).
		WhereExpr(Binary{Op: OpGE, Left: ColumnRef{Name: "d"}, Right: Literal{Value: DateFromDays(2)}}).
		SelectColumns("ts").
		Exec()
	if err == nil {
		t.Fatalf("expected null temporal comparison to fail")
	}

	filtered, err = From(frame).
		WhereEq("d", DateFromDays(2)).
		SelectColumns("ts").
		OrderByColumn("ts", Desc).
		Exec()
	if err != nil {
		t.Fatalf("filtered Exec returned error: %v", err)
	}
	assertColumnValues(t, filtered, "ts", []any{Timestamp(40), Timestamp(20)})

	grouped, err := From(frame).
		GroupBy("d").
		Count("n").
		OrderByColumn("d", Asc).
		Exec()
	if err != nil {
		t.Fatalf("grouped Exec returned error: %v", err)
	}
	assertColumnValues(t, grouped, "d", []any{NullValue, Date(1), Date(2)})
	assertColumnValues(t, grouped, "n", []any{int64(1), int64(1), int64(2)})
}

func TestQueryWhereSelect(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("qty", []any{2, 5, 3}),
		NewColumn("price", []any{10, 20, 30}),
	)

	got, err := From(frame).
		WhereEq("sym", Symbol("a")).
		SelectColumns("qty", "price").
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"qty", "price"})
	assertColumnValues(t, got, "qty", []any{int64(2), int64(3)})
	assertColumnValues(t, got, "price", []any{int64(10), int64(30)})
}

func TestQueryComputedProjection(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("price", []any{10, 20}),
		NewColumn("size", []any{3, 4}),
	)

	got, err := Exec(frame, QueryPlan{
		Source: frame,
		Select: []SelectItem{{
			Name: "notional",
			Expr: Binary{Op: OpMul, Left: ColumnRef{Name: "price"}, Right: ColumnRef{Name: "size"}},
		}},
		LimitN: -1,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	assertColumnValues(t, got, "notional", []any{30.0, 80.0})
}

func TestQueryGroupBySymbolSumCount(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{Symbol("b"), Symbol("a"), Symbol("b"), Symbol("a")}),
		NewColumn("qty", []any{2, 5, 3, 7}),
	)

	got, err := From(frame).
		GroupBy("sym").
		Sum("qty", "total_qty").
		Count("n").
		OrderByColumn("sym", Asc).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnNames(t, got, []Symbol{"sym", "total_qty", "n"})
	assertColumnValues(t, got, "sym", []any{Symbol("a"), Symbol("b")})
	assertColumnValues(t, got, "total_qty", []any{12.0, 5.0})
	assertColumnValues(t, got, "n", []any{int64(2), int64(2)})
}

func TestQueryOrderLimit(t *testing.T) {
	frame := mustFrame(t,
		NewColumn("sym", []any{"a", "b", "c"}),
		NewColumn("qty", []any{2, 5, 3}),
	)

	got, err := From(frame).
		OrderByColumn("qty", Desc).
		Limit(2).
		Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertColumnValues(t, got, "sym", []any{"b", "c"})
	assertColumnValues(t, got, "qty", []any{int64(5), int64(3)})
}

func mustFrame(t *testing.T, cols ...Column) Frame {
	t.Helper()
	frame, err := NewFrame(cols...)
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}
	return frame
}

func assertColumnNames(t *testing.T, frame Frame, want []Symbol) {
	t.Helper()
	if got := frame.Schema().Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("column names = %v, want %v", got, want)
	}
}

func assertColumnValues(t *testing.T, frame Frame, name Symbol, want []any) {
	t.Helper()
	col, ok := frame.Column(name)
	if !ok {
		t.Fatalf("missing column %q", name)
	}
	if got := col.Values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("column %q = %#v, want %#v", name, got, want)
	}
}
