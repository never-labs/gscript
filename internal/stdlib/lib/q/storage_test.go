package q

import (
	"reflect"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestStoragePortableSplayedRoundTripTemporalNulls(t *testing.T) {
	ts0, err := ParseTemporal("timestamp", "2026-06-06T09:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	ts1, err := ParseTemporal("timestamp", "2026-06-06T09:31:00Z")
	if err != nil {
		t.Fatal(err)
	}
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "AAPL"})},
		mustDataColumn(t, "ts", data.KindTimestamp, []any{ts0, data.NullValue, ts1}),
		mustDataColumn(t, "qty", data.KindI64, []any{int64(10), data.NullValue, int64(30)}),
	)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := data.SaveFrameDir(dir, frame); err != nil {
		t.Fatalf("SaveFrameDir returned error: %v", err)
	}
	loaded, err := data.LoadFrameDir(dir)
	if err != nil {
		t.Fatalf("LoadFrameDir returned error: %v", err)
	}
	if got, want := loaded.Schema().Names(), []data.Symbol{"sym", "ts", "qty"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("schema = %#v, want %#v", got, want)
	}
	if got, _ := loaded.Schema().Kind("ts"); got != data.KindTimestamp {
		t.Fatalf("ts kind = %s, want timestamp", got)
	}
	ts, _ := loaded.Column("ts")
	middleTS, _ := ts.At(1)
	if !data.IsNull(middleTS) {
		t.Fatalf("ts[1] = %#v, want null", middleTS)
	}
	qty, _ := loaded.Column("qty")
	middleQty, _ := qty.At(1)
	if !data.IsNull(middleQty) {
		t.Fatalf("qty[1] = %#v, want null", middleQty)
	}
}

func TestStoragePortableSplayedRoundTripEncodedSymbolAttributes(t *testing.T) {
	encoded, err := data.NewEncoded(data.KindSymbol, []any{data.Symbol("AAPL"), data.Symbol("MSFT")}, []int32{0, -1, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.WithArrayAttribute(encoded, data.ArrayAttributeParted)},
		data.Column{Name: "qty", Data: data.NewI64([]int64{10, 20, 30, 40})},
	)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := data.SaveFrameDir(dir, frame); err != nil {
		t.Fatalf("SaveFrameDir returned error: %v", err)
	}
	info, err := data.ReadFrameStoreInfo(dir)
	if err != nil {
		t.Fatalf("ReadFrameStoreInfo returned error: %v", err)
	}
	if got, want := info.Columns[0].Domain, []any{"AAPL", "MSFT"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stored domain = %#v, want %#v", got, want)
	}
	if got, want := info.Columns[0].Codes, []int32{0, -1, 1, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stored codes = %#v, want %#v", got, want)
	}
	if got, want := info.Columns[0].Attributes, []string{"p"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stored attributes = %#v, want %#v", got, want)
	}
	loaded, err := data.LoadFrameDir(dir)
	if err != nil {
		t.Fatalf("LoadFrameDir returned error: %v", err)
	}
	sym, _ := loaded.Column("sym")
	if !data.ArrayMetadataOf(sym).HasAttribute(data.ArrayAttributeParted) {
		t.Fatalf("sym metadata = %#v, want parted", data.ArrayMetadataOf(sym))
	}
	domain, ok := data.EncodedDomainOf(sym)
	if !ok {
		t.Fatalf("sym = %T, want encoded array", sym)
	}
	if !reflect.DeepEqual(domain, []any{data.Symbol("AAPL"), data.Symbol("MSFT")}) {
		t.Fatalf("loaded domain = %#v", domain)
	}
	codes, _ := data.EncodedCodesOf(sym)
	if !reflect.DeepEqual(codes, []int32{0, -1, 1, 0}) {
		t.Fatalf("loaded codes = %#v", codes)
	}
	missing, _ := sym.At(1)
	if !data.IsNull(missing) {
		t.Fatalf("sym[1] = %#v, want null", missing)
	}
}

func TestStoragePortablePartitionedRoundTripAndFilters(t *testing.T) {
	day0, err := ParseTemporal("date", "2026-06-06")
	if err != nil {
		t.Fatal(err)
	}
	day1, err := ParseTemporal("date", "2026-06-07")
	if err != nil {
		t.Fatal(err)
	}
	frame, err := data.NewFrame(
		mustDataColumn(t, "day", data.KindDate, []any{day0, day0, day1, day1}),
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "AAPL", "MSFT"})},
		mustDataColumn(t, "qty", data.KindI64, []any{int64(10), int64(20), int64(30), data.NullValue}),
	)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := data.SavePartitionedFrameDir(dir, frame, "day", "sym"); err != nil {
		t.Fatalf("SavePartitionedFrameDir returned error: %v", err)
	}
	loaded, err := data.LoadPartitionedFrameDir(dir, map[data.Symbol]any{"sym": data.Symbol("AAPL")})
	if err != nil {
		t.Fatalf("LoadPartitionedFrameDir returned error: %v", err)
	}
	if loaded.Len() != 2 {
		t.Fatalf("filtered len = %d, want 2", loaded.Len())
	}
	sym, _ := loaded.Column("sym")
	for i, value := range sym.Values() {
		if value != data.Symbol("AAPL") {
			t.Fatalf("sym[%d] = %#v, want AAPL", i, value)
		}
	}
	empty, err := data.LoadPartitionedFrameDir(dir, map[data.Symbol]any{"sym": data.Symbol("IBM")})
	if err != nil {
		t.Fatalf("LoadPartitionedFrameDir empty returned error: %v", err)
	}
	if empty.Len() != 0 {
		t.Fatalf("empty len = %d, want 0", empty.Len())
	}
	if got, _ := empty.Schema().Kind("day"); got != data.KindDate {
		t.Fatalf("empty day kind = %s, want date", got)
	}
	dayLoaded, err := data.LoadPartitionedFrameDir(dir, map[data.Symbol]any{"day": day1})
	if err != nil {
		t.Fatalf("LoadPartitionedFrameDir day returned error: %v", err)
	}
	if dayLoaded.Len() != 2 {
		t.Fatalf("day filtered len = %d, want 2", dayLoaded.Len())
	}
	if got, _ := dayLoaded.Schema().Kind("day"); got != data.KindDate {
		t.Fatalf("day filtered day kind = %s, want date", got)
	}
	qty, _ := dayLoaded.Column("qty")
	lastQty, _ := qty.At(1)
	if !data.IsNull(lastQty) {
		t.Fatalf("day filtered qty[1] = %#v, want null", lastQty)
	}
}
