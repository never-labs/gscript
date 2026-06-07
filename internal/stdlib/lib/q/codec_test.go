package q

import (
	"reflect"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestCodecRoundTripsScalarVectorDictFrameAndKeyedFrame(t *testing.T) {
	date, err := ParseTemporal("date", "2026-06-07")
	if err != nil {
		t.Fatal(err)
	}
	trades, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "qty", Data: data.NewI64([]int64{10, 20})},
		data.Column{Name: "when", Data: mustDataColumn(t, "when", data.KindDate, []any{date, data.NullValue}).Data},
	)
	if err != nil {
		t.Fatal(err)
	}
	typedNumerics, err := data.NewFrame(
		data.Column{Name: "i", Data: mustDataColumn(t, "i", data.KindI32, []any{int32(1), data.NullValue, int32(3)}).Data},
		data.Column{Name: "f", Data: mustDataColumn(t, "f", data.KindF32, []any{float32(1.5), data.NullValue, float32(3.25)}).Data},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(trades, "sym")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		in   any
		want func(t *testing.T, got any)
	}{
		{
			name: "symbol",
			in:   data.Symbol("AAPL"),
			want: func(t *testing.T, got any) {
				if got != data.Symbol("AAPL") {
					t.Fatalf("got %v (%T), want AAPL symbol", got, got)
				}
			},
		},
		{
			name: "temporal",
			in:   date,
			want: func(t *testing.T, got any) {
				if got != date {
					t.Fatalf("got %v (%T), want %v", got, got, date)
				}
			},
		},
		{
			name: "vector",
			in:   data.NewF64([]float64{1.25, 2.5}),
			want: func(t *testing.T, got any) {
				arr, ok := got.(data.Array)
				if !ok || arr.Kind() != data.KindF64 || arr.Len() != 2 {
					t.Fatalf("got %T %v, want f64 vector len 2", got, got)
				}
				v, _ := arr.At(1)
				if v != 2.5 {
					t.Fatalf("arr[1] = %v, want 2.5", v)
				}
			},
		},
		{
			name: "typed numeric vector",
			in:   typedNumerics,
			want: func(t *testing.T, got any) {
				frame, ok := got.(data.Frame)
				if !ok || frame.Len() != 3 {
					t.Fatalf("got %T, want frame len 3", got)
				}
				i, _ := frame.Column("i")
				f, _ := frame.Column("f")
				if i.Kind() != data.KindI32 || f.Kind() != data.KindF32 {
					t.Fatalf("kinds = %s/%s, want i32/f32", i.Kind(), f.Kind())
				}
				iv, _ := i.At(2)
				fv, _ := f.At(0)
				if iv != int32(3) || fv != float32(1.5) {
					t.Fatalf("values = %v/%v, want 3/1.5", iv, fv)
				}
			},
		},
		{
			name: "dict",
			in: Dict{
				Keys:   []any{data.Symbol("bid"), data.Symbol("ask")},
				Values: []any{99.5, 100.25},
			},
			want: func(t *testing.T, got any) {
				dict, ok := got.(Dict)
				if !ok || len(dict.Keys) != 2 || len(dict.Values) != 2 {
					t.Fatalf("got %T %v, want dict len 2", got, got)
				}
				if dict.Keys[0] != data.Symbol("bid") || dict.Values[1] != 100.25 {
					t.Fatalf("dict = %#v", dict)
				}
			},
		},
		{
			name: "frame",
			in:   trades,
			want: func(t *testing.T, got any) {
				frame, ok := got.(data.Frame)
				if !ok || frame.Len() != 2 {
					t.Fatalf("got %T, want frame len 2", got)
				}
				col, _ := frame.Column("sym")
				v, _ := col.At(1)
				if v != data.Symbol("MSFT") {
					t.Fatalf("sym[1] = %v, want MSFT", v)
				}
			},
		},
		{
			name: "keyed",
			in:   keyed,
			want: func(t *testing.T, got any) {
				keyed, ok := got.(data.KeyedFrame)
				if !ok {
					t.Fatalf("got %T, want keyed frame", got)
				}
				if keys := keyed.Keys(); len(keys) != 1 || keys[0] != "sym" {
					t.Fatalf("keys = %v, want sym", keys)
				}
				row, err := keyed.LookupValueByKey(data.Symbol("AAPL"))
				if err != nil {
					t.Fatal(err)
				}
				col, _ := row.Column("qty")
				v, _ := col.At(0)
				if v != int64(10) {
					t.Fatalf("qty = %v, want 10", v)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := Marshal(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Unmarshal(buf)
			if err != nil {
				t.Fatal(err)
			}
			tc.want(t, got)
		})
	}
}

func TestCodecRoundTripsAttributedArrays(t *testing.T) {
	in := qAttributedVector{
		attribute: data.ArrayAttributeGrouped,
		vector:    data.WithArrayAttribute(data.NewSymbols([]string{"AAPL", "MSFT", "AAPL"}), data.ArrayAttributeGrouped),
	}
	gotAny := mustCodecRoundTrip(t, in)
	got, ok := gotAny.(qAttributedVector)
	if !ok {
		t.Fatalf("got %T, want qAttributedVector", gotAny)
	}
	if got.attribute != data.ArrayAttributeGrouped {
		t.Fatalf("attribute = %v, want grouped", got.attribute)
	}
	if got.vector.Kind() != data.KindSymbol {
		t.Fatalf("kind = %s, want symbol", got.vector.Kind())
	}
	if !data.ArrayMetadataOf(got).HasAttribute(data.ArrayAttributeGrouped) {
		t.Fatalf("metadata = %#v, want grouped attribute", data.ArrayMetadataOf(got))
	}
	if _, ok := data.ArrayMetadataOf(got).Index(data.ArrayAttributeGrouped); !ok {
		t.Fatalf("metadata = %#v, want rebuilt grouped index", data.ArrayMetadataOf(got))
	}
}

func TestCodecRoundTripsTypedNullNumericVectors(t *testing.T) {
	cases := []struct {
		name string
		in   data.Array
		kind data.Kind
		want []any
	}{
		{
			name: "i32",
			in:   data.InferArray([]any{int64(1), int64(2), data.NullForKind(data.KindI32)}),
			kind: data.KindI32,
			want: []any{int32(1), int32(2), data.NullValue},
		},
		{
			name: "f32",
			in:   data.InferArray([]any{int64(1), int64(2), data.NullForKind(data.KindF32)}),
			kind: data.KindF32,
			want: []any{float32(1), float32(2), data.NullValue},
		},
		{
			name: "f64",
			in:   data.InferArray([]any{int64(1), int64(2), data.NullForKind(data.KindF64)}),
			kind: data.KindF64,
			want: []any{1.0, 2.0, data.NullValue},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotAny := mustCodecRoundTrip(t, tc.in)
			got, ok := gotAny.(data.Array)
			if !ok {
				t.Fatalf("got %T, want data.Array", gotAny)
			}
			if got.Kind() != tc.kind {
				t.Fatalf("kind = %s, want %s", got.Kind(), tc.kind)
			}
			if values := got.Values(); !reflect.DeepEqual(values, tc.want) {
				t.Fatalf("values = %#v, want %#v", values, tc.want)
			}
		})
	}
}

func TestCodecRoundTripsTypedNullScalarsAndAllNullVectors(t *testing.T) {
	scalars := []data.Kind{
		data.KindSymbol,
		data.KindDate,
		data.KindTimestamp,
	}
	for _, kind := range scalars {
		t.Run("scalar_"+string(kind), func(t *testing.T) {
			gotAny := mustCodecRoundTrip(t, data.NullForKind(kind))
			gotKind, ok := data.NullKind(gotAny)
			if !ok || gotKind != kind {
				t.Fatalf("round-trip null kind = %s ok %v, want %s (%#v)", gotKind, ok, kind, gotAny)
			}
		})
	}

	vectors := []data.Kind{
		data.KindSymbol,
		data.KindDate,
		data.KindTimestamp,
	}
	for _, kind := range vectors {
		t.Run("vector_"+string(kind), func(t *testing.T) {
			in, err := data.NewColumnWithKind("v", kind, []any{
				data.NullForKind(kind),
				data.NullForKind(kind),
			})
			if err != nil {
				t.Fatal(err)
			}
			gotAny := mustCodecRoundTrip(t, in.Data)
			got, ok := gotAny.(data.Array)
			if !ok {
				t.Fatalf("got %T, want data.Array", gotAny)
			}
			if got.Kind() != kind {
				t.Fatalf("kind = %s, want %s", got.Kind(), kind)
			}
			if values := got.Values(); !reflect.DeepEqual(values, []any{data.NullValue, data.NullValue}) {
				t.Fatalf("values = %#v, want two nulls", values)
			}
		})
	}
}

func TestCodecRoundTripsEncodedEnumVector(t *testing.T) {
	in := qEnumVector{
		domain:  data.Symbol("symDomain"),
		encoded: data.NewEncodedSymbols([]data.Symbol{"AAPL", "MSFT", "AAPL"}),
	}
	gotAny := mustCodecRoundTrip(t, in)
	got, ok := gotAny.(qEnumVector)
	if !ok {
		t.Fatalf("got %T, want qEnumVector", gotAny)
	}
	if got.domain != data.Symbol("symDomain") {
		t.Fatalf("domain = %v, want symDomain", got.domain)
	}
	if got.Kind() != data.KindSymbol {
		t.Fatalf("kind = %s, want symbol", got.Kind())
	}
	if codes := got.EncodedCodes(); !reflect.DeepEqual(codes, []int32{0, 1, 0}) {
		t.Fatalf("codes = %#v, want 0 1 0", codes)
	}
	if domain := got.EncodedDomain(); !reflect.DeepEqual(domain, []any{data.Symbol("AAPL"), data.Symbol("MSFT")}) {
		t.Fatalf("encoded domain = %#v", domain)
	}
}

func TestCodecRoundTripsFrameColumnAttributesAndEnum(t *testing.T) {
	enum := qEnumVector{
		domain:  data.Symbol("venueDomain"),
		encoded: data.NewEncodedSymbols([]data.Symbol{"XNYS", "XNAS", "XNYS"}),
	}
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.WithArrayAttribute(data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"}), data.ArrayAttributeGrouped)},
		data.Column{Name: "venue", Data: enum},
	)
	if err != nil {
		t.Fatal(err)
	}
	gotAny := mustCodecRoundTrip(t, frame)
	got, ok := gotAny.(data.Frame)
	if !ok {
		t.Fatalf("got %T, want frame", gotAny)
	}
	sym, _ := got.Column("sym")
	if !data.ArrayMetadataOf(sym).HasAttribute(data.ArrayAttributeGrouped) {
		t.Fatalf("sym metadata = %#v, want grouped attribute", data.ArrayMetadataOf(sym))
	}
	venue, _ := got.Column("venue")
	venueEnum, ok := venue.(qEnumVector)
	if !ok {
		t.Fatalf("venue column = %T, want qEnumVector", venue)
	}
	if venueEnum.domain != data.Symbol("venueDomain") {
		t.Fatalf("venue enum domain = %v, want venueDomain", venueEnum.domain)
	}
	if codes := venueEnum.EncodedCodes(); !reflect.DeepEqual(codes, []int32{0, 1, 0}) {
		t.Fatalf("venue codes = %#v", codes)
	}
}

func TestCodecRoundTripsPortableNestedQValues(t *testing.T) {
	ts0, err := ParseTemporal("timestamp", "2026-06-06T09:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	ts1, err := ParseTemporal("timestamp", "2026-06-06T09:31:00Z")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := data.NewEncoded(data.KindSymbol, []any{data.Symbol("XNYS"), data.Symbol("XNAS")}, []int32{0, -1, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.WithArrayAttribute(data.NewSymbols([]string{"AAPL", "AAPL", "MSFT", "NVDA"}), data.ArrayAttributeParted)},
		data.Column{Name: "venue", Data: qEnumVector{domain: data.Symbol("venueDomain"), encoded: encoded}},
		mustDataColumn(t, "ts", data.KindTimestamp, []any{ts0, data.NullValue, ts1, ts1}),
		mustDataColumn(t, "qty", data.KindI64, []any{int64(10), data.NullValue, int64(30), int64(40)}),
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym", "ts")
	if err != nil {
		t.Fatal(err)
	}
	in := Dict{
		Keys: []any{data.Symbol("book"), data.Symbol("payload"), data.Symbol("flags")},
		Values: []any{
			keyed,
			data.NewAny([]any{data.Symbol("AAPL"), data.NullValue, ts0}),
			qAttributedVector{attribute: data.ArrayAttributeUnique, vector: data.NewSymbols([]string{"seen", "done"})},
		},
	}
	gotAny := mustCodecRoundTrip(t, in)
	got, ok := gotAny.(Dict)
	if !ok {
		t.Fatalf("got %T, want Dict", gotAny)
	}
	gotKeyed, ok := got.Values[0].(data.KeyedFrame)
	if !ok {
		t.Fatalf("book = %T, want keyed frame", got.Values[0])
	}
	if keys := gotKeyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym", "ts"}) {
		t.Fatalf("keys = %#v", keys)
	}
	gotFrame := gotKeyed.Frame()
	sym, _ := gotFrame.Column("sym")
	if !data.ArrayMetadataOf(sym).HasAttribute(data.ArrayAttributeParted) {
		t.Fatalf("sym metadata = %#v, want parted", data.ArrayMetadataOf(sym))
	}
	venue, _ := gotFrame.Column("venue")
	venueEnum, ok := venue.(qEnumVector)
	if !ok {
		t.Fatalf("venue = %T, want qEnumVector", venue)
	}
	if codes := venueEnum.EncodedCodes(); !reflect.DeepEqual(codes, []int32{0, -1, 1, 0}) {
		t.Fatalf("venue codes = %#v", codes)
	}
	if v, _ := venueEnum.At(1); !data.IsNull(v) {
		t.Fatalf("venue[1] = %#v, want null", v)
	}
	flags, ok := got.Values[2].(qAttributedVector)
	if !ok || flags.attribute != data.ArrayAttributeUnique {
		t.Fatalf("flags = %#v (%T), want unique attributed vector", got.Values[2], got.Values[2])
	}
}

func TestCodecRoundTripsKeyedFrameWithTemporalAndNulls(t *testing.T) {
	date0, err := ParseTemporal("date", "2026-06-06")
	if err != nil {
		t.Fatal(err)
	}
	ts0, err := ParseTemporal("timestamp", "2026-06-06T09:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		mustDataColumn(t, "trade_date", data.KindDate, []any{date0, data.NullValue}),
		mustDataColumn(t, "event_ts", data.KindTimestamp, []any{data.NullValue, ts0}),
		mustDataColumn(t, "qty", data.KindI64, []any{int64(10), data.NullValue}),
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	gotAny := mustCodecRoundTrip(t, keyed)
	got, ok := gotAny.(data.KeyedFrame)
	if !ok {
		t.Fatalf("got %T, want keyed frame", gotAny)
	}
	if keys := got.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("keys = %#v, want sym", keys)
	}
	gotFrame := got.Frame()
	for name, kind := range map[data.Symbol]data.Kind{
		"trade_date": data.KindDate,
		"event_ts":   data.KindTimestamp,
		"qty":        data.KindI64,
	} {
		if gotKind, ok := gotFrame.Schema().Kind(name); !ok || gotKind != kind {
			t.Fatalf("%s kind = %s, ok %v; want %s", name, gotKind, ok, kind)
		}
	}
	eventTS, _ := gotFrame.Column("event_ts")
	first, _ := eventTS.At(0)
	second, _ := eventTS.At(1)
	if !data.IsNull(first) || second != ts0 {
		t.Fatalf("event_ts values = %#v", eventTS.Values())
	}
	row, err := got.LookupValueByKey(data.Symbol("MSFT"))
	if err != nil {
		t.Fatal(err)
	}
	qty, _ := row.Column("qty")
	v, _ := qty.At(0)
	if !data.IsNull(v) {
		t.Fatalf("MSFT qty = %#v, want null", v)
	}
}

func TestCodecRoundTripsTemporalVectorsWithTypedNulls(t *testing.T) {
	cases := []struct {
		kind string
		text string
	}{
		{"month", "2026-06"},
		{"date", "2026-06-07"},
		{"datetime", "2026-06-07T09:30:00"},
		{"timespan", "1D09:30:00"},
		{"minute", "09:30"},
		{"second", "09:30:01"},
		{"time", "09:30:01.250"},
		{"timestamp", "2026-06-07T09:30:01.250Z"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			first, err := ParseTemporal(tc.kind, tc.text)
			if err != nil {
				t.Fatal(err)
			}
			col := mustDataColumn(t, "v", data.Kind(tc.kind), []any{first, data.NullValue, first})
			gotAny := mustCodecRoundTrip(t, col.Data)
			got, ok := gotAny.(data.Array)
			if !ok {
				t.Fatalf("got %T, want data.Array", gotAny)
			}
			if got.Kind() != data.Kind(tc.kind) {
				t.Fatalf("kind = %s, want %s", got.Kind(), tc.kind)
			}
			middle, _ := got.At(1)
			if !data.IsNull(middle) {
				t.Fatalf("middle = %#v, want typed null", middle)
			}
			last, _ := got.At(2)
			if last != first {
				t.Fatalf("last = %#v (%T), want %#v (%T)", last, last, first, first)
			}
		})
	}
}

func TestCodecRoundTripsEncodedSymbolVectorWithNullCode(t *testing.T) {
	in, err := data.NewEncoded(data.KindSymbol, []any{data.Symbol("AAPL"), data.Symbol("MSFT")}, []int32{0, -1, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	gotAny := mustCodecRoundTrip(t, in)
	got, ok := gotAny.(data.Array)
	if !ok {
		t.Fatalf("got %T, want data.Array", gotAny)
	}
	domain, ok := data.EncodedDomainOf(got)
	if !ok {
		t.Fatalf("got %T, want encoded array", got)
	}
	if !reflect.DeepEqual(domain, []any{data.Symbol("AAPL"), data.Symbol("MSFT")}) {
		t.Fatalf("domain = %#v", domain)
	}
	codes, _ := data.EncodedCodesOf(got)
	if !reflect.DeepEqual(codes, []int32{0, -1, 1, 0}) {
		t.Fatalf("codes = %#v", codes)
	}
	missing, _ := got.At(1)
	if !data.IsNull(missing) {
		t.Fatalf("got[1] = %#v, want null", missing)
	}
}

func TestCodecRoundTripsAllTemporalScalars(t *testing.T) {
	cases := []struct {
		kind string
		text string
	}{
		{"month", "2026-06"},
		{"date", "2026-06-07"},
		{"datetime", "2026-06-07T09:30:00"},
		{"timespan", "1D09:30:00"},
		{"minute", "09:30"},
		{"second", "09:30:01"},
		{"time", "09:30:01.250"},
		{"timestamp", "2026-06-07T09:30:01.250Z"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			in, err := ParseTemporal(tc.kind, tc.text)
			if err != nil {
				t.Fatal(err)
			}
			got := mustCodecRoundTrip(t, in)
			if got != in {
				t.Fatalf("got %#v (%T), want %#v (%T)", got, got, in, in)
			}
		})
	}
}

func mustCodecRoundTrip(t *testing.T, in any) any {
	t.Helper()
	buf, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(buf)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func mustDataColumn(t *testing.T, name data.Symbol, kind data.Kind, values []any) data.Column {
	t.Helper()
	col, err := data.NewColumnWithKind(name, kind, values)
	if err != nil {
		t.Fatal(err)
	}
	return col
}
