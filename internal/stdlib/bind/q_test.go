package bind

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/stdlib/lib/data"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

func TestQDataFrameValueKeepsNativeFramePayload(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatal(err)
	}
	table := value.Table()
	if _, ok := table.NativePayload().(data.Frame); !ok {
		t.Fatalf("q frame native payload = %T, want data.Frame", table.NativePayload())
	}
	info, ok := table.NativeFramePayloadInfo()
	if !ok {
		t.Fatal("q frame native frame payload info missing")
	}
	if info.Kind != NativePayloadDataFrame || info.Rows != 2 || info.Columns != 2 || info.SchemaHash != frame.SchemaFingerprint() {
		t.Fatalf("q frame native payload info = %#v, want data_frame rows=2 columns=2 schema=%s", info, frame.SchemaFingerprint())
	}
	roundTrip, err := qDataFrameFromValue(value, "")
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Len() != 2 {
		t.Fatalf("round-trip frame len = %d, want 2", roundTrip.Len())
	}
	if _, ok := roundTrip.Column("price"); !ok {
		t.Fatal("round-trip frame missing price column")
	}
}

func TestQDataFrameValueInstallsRuntimeSoAPayloadForDenseFrame(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20})},
		data.Column{Name: "active", Data: data.NewBool([]bool{true, false})},
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatal(err)
	}
	table := value.Table()
	if _, ok := table.NativePayload().(*runtime.SoA); !ok {
		t.Fatalf("q dense frame native payload = %T, want *runtime.SoA", table.NativePayload())
	}
	col, handled, err := value.NativeFrameColumn("price")
	if err != nil {
		t.Fatalf("NativeFrameColumn(price): %v", err)
	}
	if !handled || !col.IsDenseArray() {
		t.Fatalf("NativeFrameColumn(price) = %v handled=%v, want dense array", col, handled)
	}
	got, ok := col.DenseArray().F64()
	if !ok || len(got) != 2 || got[0] != 100.5 || got[1] != 101.25 {
		t.Fatalf("price column = %#v, want [100.5 101.25]", got)
	}
	roundTrip, err := qDataFrameFromValue(value, "")
	if err != nil {
		t.Fatal(err)
	}
	if kind, ok := roundTrip.Schema().Kind("size"); !ok || kind != data.KindI64 {
		t.Fatalf("round-trip size kind = %q, %v; want i64", kind, ok)
	}
}

func TestQDataFrameValueKeepsDataPayloadForNonRuntimeDenseKind(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "qty", Data: data.NewI32([]int32{10, 20})},
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatal(err)
	}
	table := value.Table()
	if _, ok := table.NativePayload().(data.Frame); !ok {
		t.Fatalf("q i32 frame native payload = %T, want data.Frame", table.NativePayload())
	}
	roundTrip, err := qDataFrameFromValue(value, "")
	if err != nil {
		t.Fatal(err)
	}
	if kind, ok := roundTrip.Schema().Kind("qty"); !ok || kind != data.KindI32 {
		t.Fatalf("round-trip qty kind = %q, %v; want i32", kind, ok)
	}
}

func TestQDataFrameFromValueUsesNativeFramePayload(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5})},
	)
	if err != nil {
		t.Fatal(err)
	}
	table := NewTable()
	table.SetNativePayload(frame)

	roundTrip, err := qDataFrameFromValue(TableValue(table), "")
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Len() != 1 {
		t.Fatalf("round-trip frame len = %d, want 1", roundTrip.Len())
	}
	if _, ok := roundTrip.Column("price"); !ok {
		t.Fatal("round-trip frame missing price column")
	}
}

func TestQSQLSourceCarrierUsesNativePayloadInfoWithoutFacadeFields(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameTable := NewTable()
	frameTable.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	if got := frameTable.RawGetString("columns"); !got.IsNil() {
		t.Fatalf("test frame table columns = %v, want no facade fields", got)
	}

	carrier, err := qSQLSourceCarrierFromValue(TableValue(frameTable), "")
	if err != nil {
		t.Fatalf("frame carrier: %v", err)
	}
	if !carrier.hasInfo {
		t.Fatal("frame carrier native payload info missing")
	}
	if carrier.rows != frame.Len() || carrier.info.SchemaHash != frame.SchemaFingerprint() {
		t.Fatalf("frame carrier info rows=%d schema=%q, want rows=%d schema=%q", carrier.rows, carrier.info.SchemaHash, frame.Len(), frame.SchemaFingerprint())
	}

	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	keyedTable := NewTable()
	keyedTable.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	if got := keyedTable.RawGetString("frame"); !got.IsNil() {
		t.Fatalf("test keyed table frame = %v, want no facade fields", got)
	}

	keyedCarrier, err := qSQLSourceCarrierFromValue(TableValue(keyedTable), "")
	if err != nil {
		t.Fatalf("keyed carrier: %v", err)
	}
	if !keyedCarrier.hasKeyed || !keyedCarrier.hasInfo {
		t.Fatalf("keyed carrier hasKeyed=%v hasInfo=%v, want true true", keyedCarrier.hasKeyed, keyedCarrier.hasInfo)
	}
	if keyedCarrier.rows != frame.Len() || keyedCarrier.info.SchemaHash != frame.SchemaFingerprint() {
		t.Fatalf("keyed carrier info rows=%d schema=%q, want rows=%d schema=%q", keyedCarrier.rows, keyedCarrier.info.SchemaHash, frame.Len(), frame.SchemaFingerprint())
	}
}

func TestQSQLSourceCarrierUsesLegacyNativePayloadAsRuntimeCarrier(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameTable := NewTable()
	frameTable.SetNativePayload(frame)

	carrier, err := qSQLSourceCarrierFromValue(TableValue(frameTable), "trades")
	if err != nil {
		t.Fatalf("frame carrier: %v", err)
	}
	if carrier.bridge != "frame_native" || !carrier.native {
		t.Fatalf("frame carrier bridge=%q native=%v, want frame_native true", carrier.bridge, carrier.native)
	}
	if carrier.rows != frame.Len() {
		t.Fatalf("frame carrier rows = %d, want %d", carrier.rows, frame.Len())
	}
	if !qLooksLikeFrame(frameTable) || qIsKeyedFrameTable(frameTable) {
		t.Fatalf("legacy native frame classification looksLikeFrame=%v keyed=%v, want true false", qLooksLikeFrame(frameTable), qIsKeyedFrameTable(frameTable))
	}

	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	keyedTable := NewTable()
	keyedTable.SetNativePayload(keyed)

	keyedCarrier, err := qSQLSourceCarrierFromValue(TableValue(keyedTable), "trades")
	if err != nil {
		t.Fatalf("keyed carrier: %v", err)
	}
	if keyedCarrier.bridge != "keyed_frame_native" || !keyedCarrier.native || !keyedCarrier.hasKeyed {
		t.Fatalf("keyed carrier bridge=%q native=%v hasKeyed=%v, want keyed_frame_native true true", keyedCarrier.bridge, keyedCarrier.native, keyedCarrier.hasKeyed)
	}
	if keyedCarrier.rows != frame.Len() {
		t.Fatalf("keyed carrier rows = %d, want %d", keyedCarrier.rows, frame.Len())
	}
	if qLooksLikeFrame(keyedTable) || !qIsKeyedFrameTable(keyedTable) {
		t.Fatalf("legacy native keyed classification looksLikeFrame=%v keyed=%v, want false true", qLooksLikeFrame(keyedTable), qIsKeyedFrameTable(keyedTable))
	}
	roundTrip, err := qKeyedFrameFromValue(TableValue(keyedTable))
	if err != nil {
		t.Fatalf("qKeyedFrameFromValue legacy native keyed: %v", err)
	}
	if keys := roundTrip.Keys(); len(keys) != 1 || keys[0] != "sym" {
		t.Fatalf("legacy native keyed keys = %v, want [sym]", keys)
	}
}

func TestQSQLSourceCarrierFallsBackToFrameRowsWhenNativeInfoRowsMissing(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameTable := NewTable()
	frameTable.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})

	carrier, err := qSQLSourceCarrierFromValue(TableValue(frameTable), "")
	if err != nil {
		t.Fatalf("frame carrier: %v", err)
	}
	if carrier.rows != frame.Len() {
		t.Fatalf("frame carrier rows = %d, want %d", carrier.rows, frame.Len())
	}
	explained, err := qExplainSQL(qSQLArgsResult{
		frameValue: TableValue(frameTable),
		source:     "select price from trades where price>=100",
	})
	if err != nil {
		t.Fatalf("explain native frame: %v", err)
	}
	if got := explained.Table().RawGetString("source_rows"); !got.IsInt() || got.Int() != int64(frame.Len()) {
		t.Fatalf("frame source_rows = %v, want %d", got, frame.Len())
	}

	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	keyedTable := NewTable()
	keyedTable.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	sources := NewTable()
	sources.RawSetString("trades", TableValue(keyedTable))

	keyedCarrier, err := qSQLSourceCarrierFromValue(TableValue(sources), "trades")
	if err != nil {
		t.Fatalf("keyed carrier: %v", err)
	}
	if keyedCarrier.rows != frame.Len() {
		t.Fatalf("keyed carrier rows = %d, want %d", keyedCarrier.rows, frame.Len())
	}
	explained, err = qExplainSQL(qSQLArgsResult{
		frameValue:    TableValue(sources),
		source:        "select price from trades where price>=100",
		resolveSource: true,
	})
	if err != nil {
		t.Fatalf("explain native keyed source map: %v", err)
	}
	if got := explained.Table().RawGetString("source_rows"); !got.IsInt() || got.Int() != int64(frame.Len()) {
		t.Fatalf("keyed source_rows = %v, want %d", got, frame.Len())
	}
}

func TestQSQLSourceCarrierFallsBackToFrameRowsWhenNativeInfoRowsStale(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameTable := NewTable()
	frameTable.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       frame.Len() + 100,
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})

	carrier, err := qSQLSourceCarrierFromValue(TableValue(frameTable), "")
	if err != nil {
		t.Fatalf("frame carrier: %v", err)
	}
	if carrier.rows != frame.Len() {
		t.Fatalf("frame carrier rows = %d, want %d", carrier.rows, frame.Len())
	}
	explained, err := qExplainSQL(qSQLArgsResult{
		frameValue: TableValue(frameTable),
		source:     "select price from trades where price>=100",
	})
	if err != nil {
		t.Fatalf("explain native frame: %v", err)
	}
	if got := explained.Table().RawGetString("source_rows"); !got.IsInt() || got.Int() != int64(frame.Len()) {
		t.Fatalf("frame source_rows = %v, want %d", got, frame.Len())
	}

	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	keyedTable := NewTable()
	keyedTable.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len() + 100,
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	sources := NewTable()
	sources.RawSetString("trades", TableValue(keyedTable))

	keyedCarrier, err := qSQLSourceCarrierFromValue(TableValue(sources), "trades")
	if err != nil {
		t.Fatalf("keyed carrier: %v", err)
	}
	if keyedCarrier.rows != frame.Len() {
		t.Fatalf("keyed carrier rows = %d, want %d", keyedCarrier.rows, frame.Len())
	}
	explained, err = qExplainSQL(qSQLArgsResult{
		frameValue:    TableValue(sources),
		source:        "select price from trades where price>=100",
		resolveSource: true,
	})
	if err != nil {
		t.Fatalf("explain native keyed source map: %v", err)
	}
	if got := explained.Table().RawGetString("source_rows"); !got.IsInt() || got.Int() != int64(frame.Len()) {
		t.Fatalf("keyed source_rows = %v, want %d", got, frame.Len())
	}
}

func TestQExplainFallsBackToFrameSchemaHashWhenNativeInfoHashMissingOrStale(t *testing.T) {
	qSQLResetPlanCachesForTest()
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameTable := NewTable()
	frameTable.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:    NativePayloadDataFrame,
		Rows:    frame.Len(),
		Columns: len(frame.Schema().Names()),
	})
	explained, err := qExplainSQL(qSQLArgsResult{
		frameValue: TableValue(frameTable),
		source:     "select price from trades where price>=100",
	})
	if err != nil {
		t.Fatalf("explain native frame missing schema hash: %v", err)
	}
	if got := explained.Table().RawGetString("source_schema_hash"); !got.IsString() || got.Str() != frame.SchemaFingerprint() {
		t.Fatalf("frame source_schema_hash = %v, want %s", got, frame.SchemaFingerprint())
	}

	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	keyedTable := NewTable()
	keyedTable.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: "stale-schema-hash",
	})
	sources := NewTable()
	sources.RawSetString("trades", TableValue(keyedTable))
	explained, err = qExplainSQL(qSQLArgsResult{
		frameValue:    TableValue(sources),
		source:        "select price from trades where price>=100",
		resolveSource: true,
	})
	if err != nil {
		t.Fatalf("explain native keyed source map stale schema hash: %v", err)
	}
	if got := explained.Table().RawGetString("source_schema_hash"); !got.IsString() || got.Str() != frame.SchemaFingerprint() {
		t.Fatalf("keyed source_schema_hash = %v, want %s", got, frame.SchemaFingerprint())
	}
}

func TestQExplainUsesNativePayloadInfoBeforeWrapperFallback(t *testing.T) {
	qSQLResetPlanCachesForTest()
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameTable := NewTable()
	frameTable.RawSetString(qKeyedFrameMarker, BoolValue(true))
	frameTable.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	explained, err := qExplainSQL(qSQLArgsResult{
		frameValue: TableValue(frameTable),
		source:     "select price from trades where price>=100",
	})
	if err != nil {
		t.Fatalf("explain native frame: %v", err)
	}
	table := explained.Table()
	if got := table.RawGetString("source_bridge"); !got.IsString() || got.Str() != "frame_native" {
		t.Fatalf("source_bridge = %v, want frame_native", got)
	}
	if got := table.RawGetString("source_keyed"); !got.IsBool() || got.Bool() {
		t.Fatalf("source_keyed = %v, want false", got)
	}
	if got := table.RawGetString("source_schema_hash"); !got.IsString() || got.Str() != frame.SchemaFingerprint() {
		t.Fatalf("source_schema_hash = %v, want %s", got, frame.SchemaFingerprint())
	}

	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	keyedTable := NewTable()
	keyedTable.RawSetString("kind", StringValue("data_frame"))
	keyedTable.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	explained, err = qExplainSQL(qSQLArgsResult{
		frameValue: TableValue(keyedTable),
		source:     "select price from trades where price>=100",
	})
	if err != nil {
		t.Fatalf("explain native keyed frame: %v", err)
	}
	table = explained.Table()
	if got := table.RawGetString("source_bridge"); !got.IsString() || got.Str() != "keyed_frame_native" {
		t.Fatalf("keyed source_bridge = %v, want keyed_frame_native", got)
	}
	if got := table.RawGetString("source_keyed"); !got.IsBool() || !got.Bool() {
		t.Fatalf("keyed source_keyed = %v, want true", got)
	}
	if got := table.RawGetString("source_schema_hash"); !got.IsString() || got.Str() != frame.SchemaFingerprint() {
		t.Fatalf("keyed source_schema_hash = %v, want %s", got, frame.SchemaFingerprint())
	}
}

func TestQSQLSourceMapKeyedMutationUsesNativeCarrierBeforeWrapperFallback(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.0, 80.0})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	decoyFrame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"IBM"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{1.0})},
	)
	if err != nil {
		t.Fatal(err)
	}
	decoyValue, err := qDataFrameValue(decoyFrame)
	if err != nil {
		t.Fatal(err)
	}
	keyedTable := NewTable()
	keyedTable.RawSetString("frame", decoyValue)
	keyedTable.RawSetString("keys", qDataSymbolListValue([]data.Symbol{"missing"}))
	keyedTable.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	sources := NewTable()
	sources.RawSetString("trades", TableValue(keyedTable))

	out, err := qRunSQL("q.sql", qSQLArgsResult{
		frameValue:    TableValue(sources),
		source:        "upsert into trades (sym,price) values (`AAPL,101)",
		resolveSource: true,
		envValue:      TableValue(sources),
	})
	if err != nil {
		t.Fatalf("q.sql keyed mutation: %v", err)
	}
	table := out.Table()
	if table == nil {
		t.Fatal("keyed mutation result is nil")
	}
	if keys := qTestArrayStrings(t, table.RawGetString("keys").Table()); len(keys) != 1 || keys[0] != "sym" {
		t.Fatalf("keyed mutation keys = %v, want [sym]", keys)
	}
	rows := table.RawGetString("frame").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("keyed mutation frame len = %v, want 2", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("keyed mutation frame[1].price = %v, want 101", got)
	}
}

func TestQSQLJoinRightSourceUsesNativeCarrierBeforeWrapperFallback(t *testing.T) {
	trades, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "TSLA"})},
		data.Column{Name: "qty", Data: data.NewI64([]int64{10, 20, 30})},
	)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		data.Column{Name: "seq", Data: data.NewI64([]int64{1, 2, 3})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyedRef, err := data.KeyBy(ref, "sym")
	if err != nil {
		t.Fatal(err)
	}
	decoyFrame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"TSLA"})},
		data.Column{Name: "seq", Data: data.NewI64([]int64{99})},
	)
	if err != nil {
		t.Fatal(err)
	}
	decoyValue, err := qDataFrameValue(decoyFrame)
	if err != nil {
		t.Fatal(err)
	}
	tradesValue, err := qDataFrameValue(trades)
	if err != nil {
		t.Fatal(err)
	}
	refTable := NewTable()
	refTable.RawSetString("frame", decoyValue)
	refTable.RawSetString("keys", qDataSymbolListValue([]data.Symbol{"sym"}))
	refTable.SetNativePayloadWithInfo(keyedRef, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       ref.Len(),
		Columns:    len(ref.Schema().Names()),
		SchemaHash: ref.SchemaFingerprint(),
	})
	sources := NewTable()
	sources.RawSetString("trades", tradesValue)
	sources.RawSetString("ref", TableValue(refTable))

	out, err := qRunSQL("q.sql", qSQLArgsResult{
		frameValue:    TableValue(sources),
		source:        "select sym,qty,seq from trades left join ref on sym order by sym asc",
		resolveSource: true,
		envValue:      TableValue(sources),
	})
	if err != nil {
		t.Fatalf("q.sql join: %v", err)
	}
	rows := out.Table()
	if rows == nil || rows.Length() != 3 {
		t.Fatalf("join rows len = %v, want 3", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("seq"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("join rows[1].seq = %v, want keyed latest seq 2", got)
	}
	if got := rows.RawGetInt(3).Table().RawGetString("seq"); !got.IsNil() {
		t.Fatalf("join rows[3].seq = %v, want nil unmatched native keyed source", got)
	}
}

func TestQSQLResultKeepsNativeFrameFacade(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20, 30})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatal(err)
	}
	result, err := qRunSQL("q.sql", qSQLArgsResult{
		frameValue: frameValue,
		source:     "select sym,notional:price*size from trades where price>=100 order by notional desc",
	})
	if err != nil {
		t.Fatalf("q.sql: %v", err)
	}
	table := result.Table()
	if _, ok := table.NativePayload().(data.Frame); !ok {
		t.Fatalf("q.sql result native payload = %T, want data.Frame", table.NativePayload())
	}
	if got := table.Length(); got != 2 {
		t.Fatalf("q.sql facade length = %d, want 2", got)
	}
	if got := table.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("q.sql row 1 sym = %v, want NVDA", got)
	}
	notional := table.RawGetString("notional").Table()
	if _, ok := notional.NativePayload().(data.Array); !ok {
		t.Fatalf("q.sql notional native payload = %T, want data.Array", notional.NativePayload())
	}
	native, err := qDataFrameFromValue(result, "")
	if err != nil {
		t.Fatal(err)
	}
	if native.Len() != 2 {
		t.Fatalf("native round-trip len = %d, want 2", native.Len())
	}
	if _, ok := native.Column("notional"); !ok {
		t.Fatal("native round-trip missing notional column")
	}
}

func TestQSQLChainedJoinsExecuteInOrder(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := data.frame({
  trade_id: data.i64({1, 2, 3}),
  sym: data.symbols({"AAPL", "MSFT", "NVDA"}),
  venue: data.strings({"XNYS", "XNAS", "XNYS"}),
  ts: data.i64({10, 20, 30}),
})
quotes := data.frame({
  sym: data.symbols({"AAPL", "MSFT"}),
  ts: data.i64({10, 20}),
  bid: data.f64({100.5, 90.25}),
})
venues := data.frame({
  venue: data.strings({"XNYS", "XNAS"}),
  region: data.symbols({"US", "US"}),
  tier: data.i64({1, 2}),
})

joined := q.sql(
  "select trade_id,sym,venue,bid,region,tier from trades ij quotes on sym,ts lj venues on venue order by trade_id asc",
  {trades: trades, quotes: quotes, venues: venues}
)
explained := q.explain(
  "select trade_id,sym,venue,bid,region,tier from trades ij quotes on sym,ts lj venues on venue order by trade_id asc",
  {trades: trades, quotes: quotes, venues: venues}
)
`)

	joined := interp.GetGlobal("joined").Table()
	if joined == nil || joined.Length() != 2 {
		t.Fatalf("joined len = %v, want 2", joined)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("joined[1].bid = %v, want 100.5", got)
	}
	if got := joined.RawGetInt(2).Table().RawGetString("tier"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("joined[2].tier = %v, want 2", got)
	}
	explained := interp.GetGlobal("explained").Table()
	if explained == nil {
		t.Fatal("explained is nil")
	}
	joins := explained.RawGetString("joins").Table()
	if joins == nil || joins.Length() != 2 {
		t.Fatalf("explain joins len = %v, want 2", joins)
	}
	if got := explained.RawGetString("source_query"); !got.IsString() || !strings.Contains(got.Str(), "ij quotes") {
		t.Fatalf("explain source_query = %v, want original query", got)
	}
	if got := explained.RawGetString("join_count"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("explain join_count = %v, want 2", got)
	}
	if got := joins.RawGetInt(1).Table().RawGetString("ordinal"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("first explain join ordinal = %v, want 1", got)
	}
	if got := joins.RawGetInt(1).Table().RawGetString("right"); !got.IsString() || got.Str() != "quotes" {
		t.Fatalf("first explain join right = %v, want quotes", got)
	}
	if got := joins.RawGetInt(2).Table().RawGetString("ordinal"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("second explain join ordinal = %v, want 2", got)
	}
	if got := joins.RawGetInt(2).Table().RawGetString("right"); !got.IsString() || got.Str() != "venues" {
		t.Fatalf("second explain join right = %v, want venues", got)
	}
}

func TestQSQLPercentDivideAndQEvalPercentEndToEnd(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := data.frame({
  sym: data.symbols({"AAPL", "MSFT", "NVDA"}),
  price: data.f64({100.0, 90.0, 120.0}),
  qty: data.i64({10, 30, 20}),
  arrival: data.f64({99.0, 90.0, 100.0}),
})
projected := q.sql(trades, "select sym,ratio:price%qty,bps:(price-arrival)*10000%arrival from trades where price%qty>=6 order by price%qty desc")
updated := q.sql(trades, "update ratio:price%qty from trades where price%qty>=6")
scalar_div := q.eval("10%2")
vector_div := q.eval("10 20 30%10")
reciprocal_ok := q.eval("reciprocal 4")
`)

	projected := interp.GetGlobal("projected").Table()
	if projected == nil || projected.Length() != 2 {
		t.Fatalf("projected length = %v, want 2", projected)
	}
	first := projected.RawGetInt(1).Table()
	if got := first.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("projected[1].sym = %v, want AAPL", got)
	}
	if got := first.RawGetString("ratio"); !got.IsFloat() || got.Float() != 10.0 {
		t.Fatalf("projected[1].ratio = %v, want 10", got)
	}
	second := projected.RawGetInt(2).Table()
	if got := second.RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("projected[2].sym = %v, want NVDA", got)
	}
	if got := second.RawGetString("bps"); !got.IsFloat() || got.Float() != 2000.0 {
		t.Fatalf("projected[2].bps = %v, want 2000", got)
	}

	updated := interp.GetGlobal("updated").Table()
	if updated == nil || updated.Length() != 3 {
		t.Fatalf("updated length = %v, want 3", updated)
	}
	if got := updated.RawGetInt(1).Table().RawGetString("ratio"); !got.IsFloat() || got.Float() != 10.0 {
		t.Fatalf("updated[1].ratio = %v, want 10", got)
	}
	if got := updated.RawGetInt(3).Table().RawGetString("ratio"); !got.IsFloat() || got.Float() != 6.0 {
		t.Fatalf("updated[3].ratio = %v, want 6", got)
	}
	if got := interp.GetGlobal("scalar_div"); !got.IsFloat() || got.Float() != 5.0 {
		t.Fatalf("scalar_div = %v, want 5", got)
	}
	if got := interp.GetGlobal("reciprocal_ok"); !got.IsFloat() || got.Float() != 0.25 {
		t.Fatalf("reciprocal_ok = %v, want 0.25", got)
	}
	vector := interp.GetGlobal("vector_div")
	if !vector.IsDenseArray() || vector.DenseArray().Len() != 3 {
		t.Fatalf("vector_div = %v, want 3 item vector", interp.GetGlobal("vector_div"))
	}
	if got, _ := vector.DenseArray().At(0); !got.IsFloat() || got.Float() != 1.0 {
		t.Fatalf("vector_div[1] = %v, want 1", got)
	}
	if got, _ := vector.DenseArray().At(2); !got.IsFloat() || got.Float() != 3.0 {
		t.Fatalf("vector_div[3] = %v, want 3", got)
	}
}

func TestQKeyedFrameValueKeepsNativePayload(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5, 101.25})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	value := qKeyedFrameToValue(keyed)
	table := value.Table()
	if _, ok := table.NativePayload().(data.KeyedFrame); !ok {
		t.Fatalf("q keyed frame native payload = %T, want data.KeyedFrame", table.NativePayload())
	}
	info, ok := table.NativeFramePayloadInfo()
	if !ok {
		t.Fatal("q keyed frame native frame payload info missing")
	}
	if info.Kind != NativePayloadKeyedFrame || info.Rows != 2 || info.Columns != 2 || info.SchemaHash != frame.SchemaFingerprint() {
		t.Fatalf("q keyed frame native payload info = %#v, want data_keyed_frame rows=2 columns=2 schema=%s", info, frame.SchemaFingerprint())
	}

	roundTrip, err := qKeyedFrameFromValue(value)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Frame().Len() != 2 {
		t.Fatalf("round-trip keyed frame len = %d, want 2", roundTrip.Frame().Len())
	}
	if keys := roundTrip.Keys(); len(keys) != 1 || keys[0] != "sym" {
		t.Fatalf("round-trip keyed keys = %v, want [sym]", keys)
	}

	table.RawSetString("extra", IntValue(1))
	if got := table.NativePayload(); got != nil {
		t.Fatalf("payload after wrapper mutation = %#v, want nil", got)
	}
}

func TestQKeyedFrameFromValueUsesNativePayload(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	table := NewTable()
	table.RawSetString(qKeyedFrameMarker, BoolValue(true))
	table.SetNativePayload(keyed)

	roundTrip, err := qKeyedFrameFromValue(TableValue(table))
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Frame().Len() != 1 {
		t.Fatalf("round-trip keyed frame len = %d, want 1", roundTrip.Frame().Len())
	}
	if keys := roundTrip.Keys(); len(keys) != 1 || keys[0] != "sym" {
		t.Fatalf("round-trip keyed keys = %v, want [sym]", keys)
	}
}

func TestQKeyedFrameFromValueUsesNativePayloadInfoWithoutMarker(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	table := NewTable()
	table.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})

	roundTrip, err := qKeyedFrameFromValue(TableValue(table))
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Frame().Len() != 1 {
		t.Fatalf("round-trip keyed frame len = %d, want 1", roundTrip.Frame().Len())
	}
	if keys := roundTrip.Keys(); len(keys) != 1 || keys[0] != "sym" {
		t.Fatalf("round-trip keyed keys = %v, want [sym]", keys)
	}
}

func TestQFrameKindPrefersNativePayloadInfoOverMarkers(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}

	keyedTable := NewTable()
	keyedTable.RawSetString(dataFrameMarker, BoolValue(true))
	keyedTable.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	if qLooksLikeFrame(keyedTable) {
		t.Fatal("keyed native frame with data frame marker resolved as plain frame")
	}
	if !qIsKeyedFrameTable(keyedTable) {
		t.Fatal("keyed native frame with data frame marker did not resolve as keyed frame")
	}
	if _, err := qKeyedFrameFromValue(TableValue(keyedTable)); err != nil {
		t.Fatalf("qKeyedFrameFromValue: %v", err)
	}

	frameTable := NewTable()
	frameTable.RawSetString(qKeyedFrameMarker, BoolValue(true))
	frameTable.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	if !qLooksLikeFrame(frameTable) {
		t.Fatal("native data frame with keyed marker did not resolve as frame")
	}
	if qIsKeyedFrameTable(frameTable) {
		t.Fatal("native data frame with keyed marker resolved as keyed frame")
	}

	columnTable := NewTable()
	columnTable.RawSetString(dataFrameMarker, BoolValue(true))
	columnTable.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadDataColumn,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	if qLooksLikeFrame(columnTable) {
		t.Fatal("typed data column payload resolved as frame via concrete payload fallback")
	}
	if qIsKeyedFrameTable(columnTable) {
		t.Fatal("typed data column payload resolved as keyed frame")
	}
}

func TestQNativeFramePayloadKindMismatchFailsClosed(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}

	frameAsKeyed := NewTable()
	frameAsKeyed.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	if qLooksLikeFrame(frameAsKeyed) || !qIsKeyedFrameTable(frameAsKeyed) {
		t.Fatalf("frame payload with keyed kind classification looksLikeFrame=%v keyed=%v, want false true", qLooksLikeFrame(frameAsKeyed), qIsKeyedFrameTable(frameAsKeyed))
	}
	if _, err := qSQLSourceCarrierFromValue(TableValue(frameAsKeyed), "trades"); err == nil || !strings.Contains(err.Error(), "native keyed frame payload is invalid") {
		t.Fatalf("frame payload with keyed kind carrier err = %v, want invalid keyed payload", err)
	}
	if _, err := qKeyedFrameFromValue(TableValue(frameAsKeyed)); err == nil || !strings.Contains(err.Error(), "native keyed frame payload is invalid") {
		t.Fatalf("frame payload with keyed kind qKeyedFrameFromValue err = %v, want invalid keyed payload", err)
	}
	if _, err := qDataFrameFromValue(TableValue(frameAsKeyed), ""); err == nil || !strings.Contains(err.Error(), "native keyed frame payload is invalid") {
		t.Fatalf("frame payload with keyed kind qDataFrameFromValue err = %v, want invalid keyed payload", err)
	}

	keyedAsFrame := NewTable()
	keyedAsFrame.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
	if !qLooksLikeFrame(keyedAsFrame) || qIsKeyedFrameTable(keyedAsFrame) {
		t.Fatalf("keyed payload with frame kind classification looksLikeFrame=%v keyed=%v, want true false", qLooksLikeFrame(keyedAsFrame), qIsKeyedFrameTable(keyedAsFrame))
	}
	if _, err := qSQLSourceCarrierFromValue(TableValue(keyedAsFrame), "trades"); err == nil || !strings.Contains(err.Error(), "native data frame payload is invalid") {
		t.Fatalf("keyed payload with frame kind carrier err = %v, want invalid data frame payload", err)
	}
	if _, err := qDataFrameFromValue(TableValue(keyedAsFrame), ""); err == nil || !strings.Contains(err.Error(), "native data frame payload is invalid") {
		t.Fatalf("keyed payload with frame kind qDataFrameFromValue err = %v, want invalid data frame payload", err)
	}
}

func TestQDataFrameFromValueDoesNotResolveKeyedFrameAsSourceMap(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100.5})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	decoyFrame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"MSFT", "NVDA"})},
		data.Column{Name: "px", Data: data.NewF64([]float64{80, 210})},
	)
	if err != nil {
		t.Fatal(err)
	}
	decoyValue, err := qDataFrameValue(decoyFrame)
	if err != nil {
		t.Fatal(err)
	}
	keyedValue := qKeyedFrameToValue(keyed)
	keyedTable := keyedValue.Table()
	keyedTable.RawSetString("trades", decoyValue)
	setQKeyedFrameNativePayload(keyedTable, keyed)

	roundTrip, err := qDataFrameFromValue(keyedValue, "trades")
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Len() != 1 {
		t.Fatalf("round-trip frame len = %d, want 1", roundTrip.Len())
	}
	if _, ok := roundTrip.Column("price"); !ok {
		t.Fatal("round-trip frame missing price column")
	}
}

func TestQQueryFiltersSelectsAndGroupsSOA(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := soa.zip({
    sym: []i64{1, 1, 2, 2},
    price: []f64{10, 12, 7.5, 8},
    size: []f64{100, 50, 200, 150},
    flag: []bool{true, false, true, false},
})

rows := q.query(trades, {
    where: {column: "size", op: ">=", value: 100},
    by: {"sym"},
    select: {
        notional: {"*", "price", "size"},
        size: "size",
        fills: 1,
    },
    aggregate: {
        notional: "sum",
        size: "sum",
        fills: "count",
    },
})

flat := q.query(trades, {
    where: soa.mask(trades, "sym", "==", 1),
    select: {px: "price", qty: "size"},
})

calc := q.query(trades, {
    where: soa.mask(trades, "sym", "==", 1),
    select: {notional: {"*", "price", "size"}, adjusted: {"+", "price", 1.5}, large: {">=", "size", 75}, flagged: {"==", "flag", true}, marker: 1, active: true},
})

limited := q.query(trades, {
    where: {column: "size", op: ">=", value: 50},
    select: {px: "price", qty: "size"},
    limit: 2,
})

ordered := q.query(trades, {
    where: soa.mask(trades, "sym", "==", 1),
    select: {px: "price", qty: "size"},
    order_by: {column: "px", desc: true},
})
`)

	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("rows length = %v, want 2", rows)
	}
	first := rows.RawGetInt(1).Table()
	if got := first.RawGetString("sym"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("first.sym = %v, want 1", got)
	}
	if got := first.RawGetString("notional"); !got.IsFloat() || got.Float() != 1000 {
		t.Fatalf("first.notional = %v, want 1000", got)
	}
	if got := first.RawGetString("fills"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("first.fills = %v, want 1", got)
	}
	second := rows.RawGetInt(2).Table()
	if got := second.RawGetString("sym"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("second.sym = %v, want 2", got)
	}
	if got := second.RawGetString("notional"); !got.IsFloat() || got.Float() != 2700 {
		t.Fatalf("second.notional = %v, want 2700", got)
	}
	if got := second.RawGetString("size"); !got.IsFloat() || got.Float() != 350 {
		t.Fatalf("second.size = %v, want 350", got)
	}
	if got := second.RawGetString("fills"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("second.fills = %v, want 2", got)
	}
	flat := interp.GetGlobal("flat").Table()
	if flat == nil || flat.Length() != 2 {
		t.Fatalf("flat length = %v, want 2", flat)
	}
	if got := flat.RawGetInt(2).Table().RawGetString("qty"); !got.IsFloat() || got.Float() != 50 {
		t.Fatalf("flat[2].qty = %v, want 50", got)
	}
	if _, ok := flat.NativePayload().(*runtime.SoA); !ok {
		t.Fatalf("flat native payload = %T, want *runtime.SoA", flat.NativePayload())
	}
	flatValue := TableValue(flat)
	info, ok := flatValue.NativeFramePayloadInfo()
	if !ok || info.Rows != 2 || info.Columns != 2 {
		t.Fatalf("flat native frame info = %+v ok=%v, want rows=2 columns=2", info, ok)
	}
	if !strings.HasPrefix(info.SchemaHash, "q.query.kernel:") {
		t.Fatalf("flat schema hash = %q, want q.query kernel payload", info.SchemaHash)
	}
	col, handled, err := flatValue.NativeFrameColumn("px")
	if err != nil {
		t.Fatalf("flat NativeFrameColumn(px): %v", err)
	}
	if !handled || !col.IsDenseArray() {
		t.Fatalf("flat NativeFrameColumn(px) = %v handled=%v, want dense array", col, handled)
	}
	px, ok := col.DenseArray().F64()
	if !ok || len(px) != 2 || px[0] != 10 || px[1] != 12 {
		t.Fatalf("flat px column = %#v, want [10 12]", px)
	}
	calc := interp.GetGlobal("calc").Table()
	if calc == nil || calc.Length() != 2 {
		t.Fatalf("calc length = %v, want 2", calc)
	}
	calcInfo, ok := TableValue(calc).NativeFramePayloadInfo()
	if !ok || !strings.HasPrefix(calcInfo.SchemaHash, "q.query.kernel:") {
		t.Fatalf("calc native frame info = %+v ok=%v, want q.query kernel payload", calcInfo, ok)
	}
	notionalCol, handled, err := TableValue(calc).NativeFrameColumn("notional")
	if err != nil {
		t.Fatalf("calc NativeFrameColumn(notional): %v", err)
	}
	if !handled || !notionalCol.IsDenseArray() {
		t.Fatalf("calc notional column = %v handled=%v, want dense array", notionalCol, handled)
	}
	notional, ok := notionalCol.DenseArray().F64()
	if !ok || len(notional) != 2 || notional[0] != 1000 || notional[1] != 600 {
		t.Fatalf("calc notional = %#v, want [1000 600]", notional)
	}
	adjustedCol, handled, err := TableValue(calc).NativeFrameColumn("adjusted")
	if err != nil {
		t.Fatalf("calc NativeFrameColumn(adjusted): %v", err)
	}
	if !handled || !adjustedCol.IsDenseArray() {
		t.Fatalf("calc adjusted column = %v handled=%v, want dense array", adjustedCol, handled)
	}
	adjusted, ok := adjustedCol.DenseArray().F64()
	if !ok || len(adjusted) != 2 || adjusted[0] != 11.5 || adjusted[1] != 13.5 {
		t.Fatalf("calc adjusted = %#v, want [11.5 13.5]", adjusted)
	}
	largeCol, handled, err := TableValue(calc).NativeFrameColumn("large")
	if err != nil {
		t.Fatalf("calc NativeFrameColumn(large): %v", err)
	}
	if !handled || !largeCol.IsDenseArray() {
		t.Fatalf("calc large column = %v handled=%v, want dense array", largeCol, handled)
	}
	large, ok := largeCol.DenseArray().Bool()
	if !ok || len(large) != 2 || !large[0] || large[1] {
		t.Fatalf("calc large = %#v, want [true false]", large)
	}
	flaggedCol, handled, err := TableValue(calc).NativeFrameColumn("flagged")
	if err != nil {
		t.Fatalf("calc NativeFrameColumn(flagged): %v", err)
	}
	if !handled || !flaggedCol.IsDenseArray() {
		t.Fatalf("calc flagged column = %v handled=%v, want dense array", flaggedCol, handled)
	}
	flagged, ok := flaggedCol.DenseArray().Bool()
	if !ok || len(flagged) != 2 || !flagged[0] || flagged[1] {
		t.Fatalf("calc flagged = %#v, want [true false]", flagged)
	}
	markerCol, handled, err := TableValue(calc).NativeFrameColumn("marker")
	if err != nil {
		t.Fatalf("calc NativeFrameColumn(marker): %v", err)
	}
	if !handled || !markerCol.IsDenseArray() {
		t.Fatalf("calc marker column = %v handled=%v, want dense array", markerCol, handled)
	}
	marker, ok := markerCol.DenseArray().I64()
	if !ok || len(marker) != 2 || marker[0] != 1 || marker[1] != 1 {
		t.Fatalf("calc marker = %#v, want [1 1]", marker)
	}
	activeCol, handled, err := TableValue(calc).NativeFrameColumn("active")
	if err != nil {
		t.Fatalf("calc NativeFrameColumn(active): %v", err)
	}
	if !handled || !activeCol.IsDenseArray() {
		t.Fatalf("calc active column = %v handled=%v, want dense array", activeCol, handled)
	}
	active, ok := activeCol.DenseArray().Bool()
	if !ok || len(active) != 2 || !active[0] || !active[1] {
		t.Fatalf("calc active = %#v, want [true true]", active)
	}
	limited := interp.GetGlobal("limited").Table()
	if limited == nil || limited.Length() != 2 {
		t.Fatalf("limited length = %v, want 2", limited)
	}
	limitedInfo, ok := TableValue(limited).NativeFramePayloadInfo()
	if !ok || limitedInfo.Rows != 2 || !strings.HasPrefix(limitedInfo.SchemaHash, "q.query.kernel:") {
		t.Fatalf("limited native frame info = %+v ok=%v, want 2 q.query kernel rows", limitedInfo, ok)
	}
	limitedCol, handled, err := TableValue(limited).NativeFrameColumn("px")
	if err != nil {
		t.Fatalf("limited NativeFrameColumn(px): %v", err)
	}
	if !handled || !limitedCol.IsDenseArray() {
		t.Fatalf("limited px column = %v handled=%v, want dense array", limitedCol, handled)
	}
	limitedPx, ok := limitedCol.DenseArray().F64()
	if !ok || len(limitedPx) != 2 || limitedPx[0] != 10 || limitedPx[1] != 12 {
		t.Fatalf("limited px = %#v, want [10 12]", limitedPx)
	}
	ordered := interp.GetGlobal("ordered").Table()
	if ordered == nil || ordered.Length() != 2 {
		t.Fatalf("ordered length = %v, want 2", ordered)
	}
	if got := ordered.RawGetInt(1).Table().RawGetString("px"); !got.IsFloat() || got.Float() != 12 {
		t.Fatalf("ordered[1].px = %v, want 12", got)
	}
	orderedInfo, ok := TableValue(ordered).NativeFramePayloadInfo()
	if !ok {
		t.Fatal("ordered missing native frame payload info")
	}
	if !strings.HasPrefix(orderedInfo.SchemaHash, "q.query.kernel:") {
		t.Fatalf("ordered schema hash = %q, want q.query kernel payload", orderedInfo.SchemaHash)
	}
	orderedCol, handled, err := TableValue(ordered).NativeFrameColumn("px")
	if err != nil {
		t.Fatalf("ordered NativeFrameColumn(px): %v", err)
	}
	if !handled || !orderedCol.IsDenseArray() {
		t.Fatalf("ordered px column = %v handled=%v, want dense array", orderedCol, handled)
	}
	orderedPx, ok := orderedCol.DenseArray().F64()
	if !ok || len(orderedPx) != 2 || orderedPx[0] != 12 || orderedPx[1] != 10 {
		t.Fatalf("ordered px = %#v, want [12 10]", orderedPx)
	}
}

func TestQQueryKernelPayloadFeedsRuntimePrimitivePipeline(t *testing.T) {
	trades, err := NewSoA(map[string]*DenseArray{
		"sym":   NewDenseArrayI64([]int64{1, 1, 2, 2}),
		"price": NewDenseArrayF64([]float64{10, 12, 7.5, 8}),
		"size":  NewDenseArrayI64([]int64{100, 50, 200, 150}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	selects := NewTable()
	selects.RawSetString("px", StringValue("price"))
	selects.RawSetString("qty", StringValue("size"))
	spec := NewTable()
	spec.RawSetString("where", DenseArrayValue(NewDenseArrayBool([]bool{true, true, false, true})))
	spec.RawSetString("select", TableValue(selects))
	rows, err := qRunQuery(trades, spec)
	if err != nil {
		t.Fatalf("qRunQuery: %v", err)
	}
	value := TableValue(rows)
	info, ok := value.NativeFramePayloadInfo()
	if !ok || !strings.HasPrefix(info.SchemaHash, "q.query.kernel:") {
		t.Fatalf("q.query payload info = %+v ok=%v, want q.query kernel payload", info, ok)
	}

	price, handled, err := value.NativeFrameColumn("px")
	if err != nil {
		t.Fatalf("NativeFrameColumn(px): %v", err)
	}
	if !handled || !price.IsDenseArray() {
		t.Fatalf("NativeFrameColumn(px) = %v handled=%v, want dense array", price, handled)
	}
	mask, err := runtime.DenseArrayElementwise(runtime.DenseArrayGE, price, runtime.FloatValue(9))
	if err != nil {
		t.Fatalf("DenseArrayElementwise(px >= 9): %v", err)
	}
	if !mask.IsDenseArray() {
		t.Fatalf("compare mask = %v, want dense array", mask)
	}
	filtered, handled, err := value.NativeFrameFilter(mask.DenseArray())
	if err != nil {
		t.Fatalf("NativeFrameFilter: %v", err)
	}
	if !handled {
		t.Fatal("NativeFrameFilter was not handled")
	}
	projected, handled, err := filtered.NativeFrameProject([]string{"qty"})
	if err != nil {
		t.Fatalf("NativeFrameProject(qty): %v", err)
	}
	if !handled {
		t.Fatal("NativeFrameProject was not handled")
	}
	qty, handled, err := projected.NativeFrameColumn("qty")
	if err != nil {
		t.Fatalf("NativeFrameColumn(qty): %v", err)
	}
	if !handled || !qty.IsDenseArray() {
		t.Fatalf("NativeFrameColumn(qty) = %v handled=%v, want dense array", qty, handled)
	}
	got, ok := qty.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 50 {
		t.Fatalf("primitive pipeline qty = %#v, want [100 50]", got)
	}
}

func TestQModuleRejectsInvalidPlans(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := soa.zip({x: []f64{1}})
ok, err := pcall(func() {
    return q.query(trades, {where: {column: "x", op: "contains", value: 1}})
})
`)
	if got := interp.GetGlobal("ok"); !got.IsBool() || got.Bool() {
		t.Fatalf("ok = %v, want false", got)
	}
	if got := interp.GetGlobal("err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("err = %v, want non-empty string", got)
	}
}

func TestQExplainQueryReportsNativeKernelSupport(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	interp := runWithQAndSOA(t, `
trades := soa.zip({
    price: []f64{10, 12, 7.5},
    size: []f64{100, 50, 200},
})

primed := q.query(trades, {
    where: {column: "size", op: ">=", value: 50},
    select: {notional: {"*", "price", "size"}},
    order_by: {column: "notional", desc: true},
    limit: 2,
})

supported := q.explain_query(trades, {
    where: {column: "size", op: ">=", value: 50},
    select: {notional: {"*", "price", "size"}},
    order_by: {column: "notional", desc: true},
    limit: 2,
})

unsupported := q.explain_query(trades, {
    select: {price: "price", marker: {"negate", "price"}},
})
`)

	primed := interp.GetGlobal("primed").Table()
	if primed == nil || primed.Length() != 2 {
		t.Fatalf("primed query = %v, want two rows", primed)
	}
	supported := interp.GetGlobal("supported").Table()
	if supported == nil {
		t.Fatal("supported explain is nil")
	}
	if got := supported.RawGetString("kernel_cached"); !got.IsBool() || !got.Bool() {
		t.Fatalf("supported kernel_cached = %v, want true", got)
	}
	if got := supported.RawGetString("kernel_supported"); !got.IsBool() || !got.Bool() {
		t.Fatalf("supported kernel_supported = %v, want true", got)
	}
	if got := supported.RawGetString("kernel_reason_code"); !got.IsString() || got.Str() != qKernelReasonSupported {
		t.Fatalf("supported kernel_reason_code = %v, want %s", got, qKernelReasonSupported)
	}
	if got := supported.RawGetString("kernel_shape"); !got.IsString() || !strings.Contains(got.Str(), "select=") {
		t.Fatalf("supported kernel_shape = %v, want stable q.query select shape", got)
	}
	if got := supported.RawGetString("kernel_rows"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("supported kernel_rows = %v, want 2", got)
	}
	if got := supported.RawGetString("kernel_columns"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("supported kernel_columns = %v, want 1", got)
	}
	if got := supported.RawGetString("source_schema_hash"); !got.IsString() || !strings.HasPrefix(got.Str(), "q.query.kernel:") {
		t.Fatalf("supported schema hash = %v, want q.query kernel hash", got)
	}
	if got := supported.RawGetString("kernel_schema_hash"); !got.IsString() || !strings.HasPrefix(got.Str(), "q.query.kernel:") || got.Str() == supported.RawGetString("source_schema_hash").Str() {
		t.Fatalf("supported kernel_schema_hash = %v, want distinct projected kernel schema hash", got)
	}
	kernelSchema := supported.RawGetString("kernel_schema").Table()
	if kernelSchema == nil || kernelSchema.Length() != 1 {
		t.Fatalf("supported kernel_schema = %v, want one column", kernelSchema)
	}
	kernelColumn := kernelSchema.RawGetInt(1).Table()
	if kernelColumn == nil {
		t.Fatal("supported kernel_schema[1] is nil")
	}
	if got := kernelColumn.RawGetString("name"); !got.IsString() || got.Str() != "notional" {
		t.Fatalf("supported kernel_schema[1].name = %v, want notional", got)
	}
	if got := kernelColumn.RawGetString("kind"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("supported kernel_schema[1].kind = %v, want f64", got)
	}

	unsupported := interp.GetGlobal("unsupported").Table()
	if unsupported == nil {
		t.Fatal("unsupported explain is nil")
	}
	if got := unsupported.RawGetString("kernel_cached"); !got.IsBool() || got.Bool() {
		t.Fatalf("unsupported kernel_cached = %v, want false", got)
	}
	if got := unsupported.RawGetString("kernel_supported"); !got.IsBool() || got.Bool() {
		t.Fatalf("unsupported kernel_supported = %v, want false", got)
	}
	if got := unsupported.RawGetString("kernel_reason_code"); !got.IsString() || got.Str() != qQueryKernelReasonSelect {
		t.Fatalf("unsupported kernel_reason_code = %v, want %s", got, qQueryKernelReasonSelect)
	}
	if got := unsupported.RawGetString("source_schema_hash"); !got.IsString() || !strings.HasPrefix(got.Str(), "q.query.kernel:") {
		t.Fatalf("unsupported source_schema_hash = %v, want q.query source schema hash", got)
	}
	if got := unsupported.RawGetString("source_schema_hash"); got.Str() != supported.RawGetString("source_schema_hash").Str() {
		t.Fatalf("unsupported source_schema_hash = %v, want same input schema as supported explain", got)
	}
	if got := unsupported.RawGetString("kernel_schema_hash"); !got.IsString() || got.Str() != "" {
		t.Fatalf("unsupported kernel_schema_hash = %v, want empty string", got)
	}
	if got := unsupported.RawGetString("kernel_shape"); !got.IsString() || !strings.Contains(got.Str(), "select=") {
		t.Fatalf("unsupported kernel_shape = %v, want stable fallback select shape", got)
	}
	if schema := unsupported.RawGetString("kernel_schema").Table(); schema == nil || schema.Length() != 0 {
		t.Fatalf("unsupported kernel_schema = %v, want empty table", schema)
	}
	stats := qTestFallbackStatsRows(t, qFallbackStatsTable())
	if got := stats[qFallbackQueryKernel]; got != 0 {
		t.Fatalf("explain_query fallback count = %d, want 0", got)
	}
}

func TestQSessionKeepsWorkspaceStateWithoutChangingQEval(t *testing.T) {
	interp := runWithQAndSOA(t, `
s := q.session()
first := s.eval("x:41")
second := s.eval("x+1")
port_before := s.eval("\\p")
port_set := s.eval("\\p 5000")
port_after := s.eval("\\p")

q.eval("y:10")
stateless_ok, stateless_err := pcall(q.eval, "y+1")
`)

	if got := interp.GetGlobal("first"); !got.IsInt() || got.Int() != 41 {
		t.Fatalf("session first = %v, want 41", got)
	}
	if got := interp.GetGlobal("second"); !got.IsInt() || got.Int() != 42 {
		t.Fatalf("session second = %v, want 42", got)
	}
	if got := interp.GetGlobal("port_before"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("session port_before = %v, want 0", got)
	}
	if got := interp.GetGlobal("port_set"); !got.IsInt() || got.Int() != 5000 {
		t.Fatalf("session port_set = %v, want 5000", got)
	}
	if got := interp.GetGlobal("port_after"); !got.IsInt() || got.Int() != 5000 {
		t.Fatalf("session port_after = %v, want 5000", got)
	}
	if got := interp.GetGlobal("stateless_ok"); !got.IsBool() || got.Bool() {
		t.Fatalf("q.eval stateless_ok = %v, want false", got)
	}
	if got := interp.GetGlobal("stateless_err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("q.eval stateless_err = %v, want non-empty error", got)
	}
}

func TestQQueryOrdersAndLimitsRows(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := soa.zip({
    sym: []i64{1, 2, 3, 4},
    price: []f64{10, 30, 20, 40},
    size: []f64{3, 1, 4, 2},
})

top_prices := q.query(trades, {
    select: {sym: "sym", price: "price", size: "size"},
    order_by: {column: "price", desc: true},
    limit: 2,
})

by_size := q.query(trades, {
    select: {sym: "sym", price: "price", size: "size"},
    order_by: "size",
    limit: 3,
})

notional_by_sym := q.query(trades, {
    by: {"sym"},
    select: {notional: {"*", "price", "size"}},
    aggregate: {notional: "sum"},
    order_by: {column: "notional", desc: true},
    limit: 2,
})
`)
	top := interp.GetGlobal("top_prices").Table()
	if top == nil || top.Length() != 2 {
		t.Fatalf("top_prices length = %v, want 2", top)
	}
	if got := top.RawGetInt(1).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 4 {
		t.Fatalf("top_prices[1].sym = %v, want 4", got)
	}
	if got := top.RawGetInt(2).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("top_prices[2].sym = %v, want 2", got)
	}
	bySize := interp.GetGlobal("by_size").Table()
	if got := bySize.RawGetInt(1).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("by_size[1].sym = %v, want 2", got)
	}
	if got := bySize.RawGetInt(3).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("by_size[3].sym = %v, want 1", got)
	}
	grouped := interp.GetGlobal("notional_by_sym").Table()
	if got := grouped.RawGetInt(1).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("notional_by_sym[1].sym = %v, want 3", got)
	}
	if got := grouped.RawGetInt(2).Table().RawGetString("notional"); !got.IsFloat() || got.Float() != 80 {
		t.Fatalf("notional_by_sym[2].notional = %v, want 80", got)
	}
}

func TestQSQLSelectWhereByOverFrame(t *testing.T) {
	interp := runWithQAndSOA(t,
		"trades := q.eval(\"flip `sym`price`size!(`AAPL`MSFT`AAPL`MSFT;100 80 120 110;10 20 30 40)\")\n"+
			"rollup := q.sql(trades, \"select notional:sum price*size, fills:count i by sym from trades where price>=100\")\n"+
			"also := q.select(trades, \"select px:price, qty:size from trades where sym=`AAPL\")\n"+
			"exec_prices := q.sql(trades, \"exec price from trades where sym=`AAPL order by price asc\")\n"+
			"all_cols := q.sql(trades, \"select * from trades where price>=100 order by price asc\")\n"+
			"omitted_select := q.sql(trades, \"select from trades where price>=100 order by price asc\")\n"+
			"omitted_exec := q.sql(trades, \"exec from trades where price>=100 order by price asc\")\n")

	rollup := interp.GetGlobal("rollup").Table()
	if rollup == nil || rollup.Length() != 2 {
		t.Fatalf("rollup length = %v, want 2", rollup)
	}
	first := rollup.RawGetInt(1).Table()
	if got := first.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rollup[1].sym = %v, want AAPL", got)
	}
	if got := first.RawGetString("notional"); !got.IsFloat() || got.Float() != 4600 {
		t.Fatalf("rollup[1].notional = %v, want 4600", got)
	}
	if got := first.RawGetString("fills"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("rollup[1].fills = %v, want 2", got)
	}
	second := rollup.RawGetInt(2).Table()
	if got := second.RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("rollup[2].sym = %v, want MSFT", got)
	}
	if got := second.RawGetString("notional"); !got.IsFloat() || got.Float() != 4400 {
		t.Fatalf("rollup[2].notional = %v, want 4400", got)
	}

	also := interp.GetGlobal("also").Table()
	if also == nil || also.Length() != 2 {
		t.Fatalf("also length = %v, want 2", also)
	}
	if got := also.RawGetInt(2).Table().RawGetString("qty"); !got.IsInt() || got.Int() != 30 {
		t.Fatalf("also[2].qty = %v, want 30", got)
	}

	execPrices := interp.GetGlobal("exec_prices").DenseArray()
	if execPrices == nil || execPrices.Len() != 2 {
		t.Fatalf("exec_prices length = %v, want 2", execPrices)
	}
	if got, err := execPrices.At(0); err != nil || !got.IsInt() || got.Int() != 100 {
		t.Fatalf("exec_prices[1] = %v, want 100", got)
	}
	if got, err := execPrices.At(1); err != nil || !got.IsInt() || got.Int() != 120 {
		t.Fatalf("exec_prices[2] = %v, want 120", got)
	}

	allCols := interp.GetGlobal("all_cols").Table()
	if allCols == nil || allCols.Length() != 3 {
		t.Fatalf("all_cols length = %v, want 3", allCols)
	}
	firstAll := allCols.RawGetInt(1).Table()
	if got := firstAll.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("all_cols[1].sym = %v, want AAPL", got)
	}
	if got := firstAll.RawGetString("price"); !got.IsInt() || got.Int() != 100 {
		t.Fatalf("all_cols[1].price = %v, want 100", got)
	}
	if got := firstAll.RawGetString("size"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("all_cols[1].size = %v, want 10", got)
	}
	omittedSelect := interp.GetGlobal("omitted_select").Table()
	if omittedSelect == nil || omittedSelect.Length() != 3 {
		t.Fatalf("omitted_select length = %v, want 3", omittedSelect)
	}
	firstOmitted := omittedSelect.RawGetInt(1).Table()
	if got := firstOmitted.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("omitted_select[1].sym = %v, want AAPL", got)
	}
	if got := firstOmitted.RawGetString("price"); !got.IsInt() || got.Int() != 100 {
		t.Fatalf("omitted_select[1].price = %v, want 100", got)
	}
	if got := firstOmitted.RawGetString("size"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("omitted_select[1].size = %v, want 10", got)
	}
	omittedExec := interp.GetGlobal("omitted_exec").Table()
	if omittedExec == nil {
		t.Fatalf("omitted_exec = nil, want dictionary")
	}
	if got := omittedExec.RawGetString("sym").Table().RawGetInt(1); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("omitted_exec.sym[1] = %v, want AAPL", got)
	}
	if got, err := omittedExec.RawGetString("price").DenseArray().At(0); err != nil || !got.IsInt() || got.Int() != 100 {
		t.Fatalf("omitted_exec.price[1] = %v, want 100", got)
	}
}

func TestQSQLCapturesOuterScalarsFromEnv(t *testing.T) {
	interp := runWithQAndSOA(t,
		"trades := q.eval(\"flip `sym`price`size!(`AAPL`MSFT`AAPL`MSFT;100 80 120 110;10 20 30 40)\")\n"+
			"threshold := 100\n"+
			"min_size := 20\n"+
			"target := \"AAPL\"\n"+
			"window := 2\n"+
			"price := 999\n"+
			"from_map := q.sql(\"select sym,price,size from trades where price>threshold order by price asc\", {trades: trades, threshold: threshold})\n"+
			"from_frame_env := q.sql(trades, \"select sym,price,size from trades where price>=threshold,size>=min_size order by price asc\", {threshold: threshold, min_size: min_size})\n"+
			"by_symbol_string := q.sql(\"select sym,price from trades where sym=target order by price asc\", {trades: trades, target: target})\n"+
			"column_wins := q.sql(\"select sym,price from trades where price<110 order by price asc\", {trades: trades, price: price})\n"+
			"windowed := q.sql(\"select sym,lag:window xprev price,mavg:window mavg price from trades order by price asc\", {trades: trades, window: window})\n")

	fromMap := interp.GetGlobal("from_map").Table()
	if fromMap == nil || fromMap.Length() != 2 {
		t.Fatalf("from_map length = %v, want 2", fromMap)
	}
	if got := fromMap.RawGetInt(1).Table().RawGetString("price"); !got.IsInt() || got.Int() != 110 {
		t.Fatalf("from_map[1].price = %v, want 110", got)
	}
	if got := fromMap.RawGetInt(2).Table().RawGetString("price"); !got.IsInt() || got.Int() != 120 {
		t.Fatalf("from_map[2].price = %v, want 120", got)
	}

	fromFrameEnv := interp.GetGlobal("from_frame_env").Table()
	if fromFrameEnv == nil || fromFrameEnv.Length() != 2 {
		t.Fatalf("from_frame_env length = %v, want 2", fromFrameEnv)
	}
	if got := fromFrameEnv.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("from_frame_env[1].sym = %v, want MSFT", got)
	}
	if got := fromFrameEnv.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("from_frame_env[2].sym = %v, want AAPL", got)
	}

	bySymbolString := interp.GetGlobal("by_symbol_string").Table()
	if bySymbolString == nil || bySymbolString.Length() != 2 {
		t.Fatalf("by_symbol_string length = %v, want 2", bySymbolString)
	}
	if got := bySymbolString.RawGetInt(1).Table().RawGetString("price"); !got.IsInt() || got.Int() != 100 {
		t.Fatalf("by_symbol_string[1].price = %v, want 100", got)
	}

	columnWins := interp.GetGlobal("column_wins").Table()
	if columnWins == nil || columnWins.Length() != 2 {
		t.Fatalf("column_wins length = %v, want 2", columnWins)
	}
	if got := columnWins.RawGetInt(1).Table().RawGetString("price"); !got.IsInt() || got.Int() != 80 {
		t.Fatalf("column_wins[1].price = %v, want 80", got)
	}

	windowed := interp.GetGlobal("windowed").Table()
	if windowed == nil || windowed.Length() != 4 {
		t.Fatalf("windowed length = %v, want 4", windowed)
	}
	if got := windowed.RawGetInt(1).Table().RawGetString("lag"); !got.IsNil() {
		t.Fatalf("windowed[1].lag = %v, want nil", got)
	}
	if got := windowed.RawGetInt(3).Table().RawGetString("lag"); !got.IsInt() || got.Int() != 80 {
		t.Fatalf("windowed[3].lag = %v, want 80", got)
	}
	if got := windowed.RawGetInt(2).Table().RawGetString("mavg"); !got.IsFloat() || got.Float() != 90 {
		t.Fatalf("windowed[2].mavg = %v, want 90", got)
	}
}

func TestQSQLTemporalStringPredicatesAlignToColumnKind(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := data.frame({
    sym: data.symbols({"AAPL", "MSFT", "NVDA"}),
    trade_date: data.date({"2026-06-01", "2026-06-02", data.null}),
    session_time: data.time({"09:30:00", "15:59:00", "16:00:00"}),
    event_ts: data.timestamp({"2026-06-01T09:30:00Z", "2026-06-02T15:59:00Z", data.null}),
    price: data.f64({100.5, 90.0, 120.0}),
})
by_date := q.sql(trades, "select sym,trade_date from trades where trade_date=\"2026-06-01\"")
by_q_date := q.sql(trades, "select sym,trade_date from trades where trade_date=\"2026.06.02\"")
by_time := q.sql(trades, "select sym,session_time from trades where session_time=\"09:30:00\"")
by_fractional_time := q.sql(trades, "select sym,session_time from trades where session_time=\"15:59:00.000000000\"")
by_ts := q.sql(trades, "select sym,event_ts from trades where event_ts=\"2026-06-02T15:59:00Z\"")
by_q_ts := q.sql(trades, "select sym,event_ts from trades where event_ts=\"2026.06.01D09:30:00\"")
in_time := q.sql(trades, "select sym,session_time from trades where session_time in (\"09:30:00\" \"16:00:00\") order by sym asc")
null_date := q.sql(trades, "select sym,trade_date from trades where trade_date=0Nd")
null_ts := q.sql(trades, "select sym,event_ts from trades where event_ts=0Np")
`)

	byDate := interp.GetGlobal("by_date").Table()
	if byDate == nil || byDate.Length() != 1 {
		t.Fatalf("by_date length = %v, want 1", byDate)
	}
	if got := byDate.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("by_date[1].sym = %v, want AAPL", got)
	}
	byQDate := interp.GetGlobal("by_q_date").Table()
	if byQDate == nil || byQDate.Length() != 1 {
		t.Fatalf("by_q_date length = %v, want 1", byQDate)
	}
	if got := byQDate.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("by_q_date[1].sym = %v, want MSFT", got)
	}
	byTime := interp.GetGlobal("by_time").Table()
	if byTime == nil || byTime.Length() != 1 {
		t.Fatalf("by_time length = %v, want 1", byTime)
	}
	if got := byTime.RawGetInt(1).Table().RawGetString("session_time"); !got.IsString() || got.Str() != "09:30:00" {
		t.Fatalf("by_time[1].session_time = %v, want 09:30:00", got)
	}
	byFractionalTime := interp.GetGlobal("by_fractional_time").Table()
	if byFractionalTime == nil || byFractionalTime.Length() != 1 {
		t.Fatalf("by_fractional_time length = %v, want 1", byFractionalTime)
	}
	if got := byFractionalTime.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("by_fractional_time[1].sym = %v, want MSFT", got)
	}
	byTS := interp.GetGlobal("by_ts").Table()
	if byTS == nil || byTS.Length() != 1 {
		t.Fatalf("by_ts length = %v, want 1", byTS)
	}
	if got := byTS.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("by_ts[1].sym = %v, want MSFT", got)
	}
	byQTS := interp.GetGlobal("by_q_ts").Table()
	if byQTS == nil || byQTS.Length() != 1 {
		t.Fatalf("by_q_ts length = %v, want 1", byQTS)
	}
	if got := byQTS.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("by_q_ts[1].sym = %v, want AAPL", got)
	}
	inTime := interp.GetGlobal("in_time").Table()
	if inTime == nil || inTime.Length() != 2 {
		t.Fatalf("in_time length = %v, want 2", inTime)
	}
	if got := inTime.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("in_time[2].sym = %v, want NVDA", got)
	}
	nullDate := interp.GetGlobal("null_date").Table()
	if nullDate == nil || nullDate.Length() != 1 {
		t.Fatalf("null_date length = %v, want 1", nullDate)
	}
	if got := nullDate.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("null_date[1].sym = %v, want NVDA", got)
	}
	nullTS := interp.GetGlobal("null_ts").Table()
	if nullTS == nil || nullTS.Length() != 1 {
		t.Fatalf("null_ts length = %v, want 1", nullTS)
	}
	if got := nullTS.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("null_ts[1].sym = %v, want NVDA", got)
	}
}

func TestQSQLTypedMutationJoinAsofGroupBoundary(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := data.frame({
    trade_id: data.i64({1, 2, 3, 4}),
    sym: data.symbols({"AAPL", "AAPL", "MSFT", "TSLA"}),
    venue: data.strings({"XNAS", "XNYS", "XNYS", "XNAS"}),
    trade_date: data.date({"2026-06-01", "2026-06-01", "2026-06-02", "2026-06-02"}),
    event_ts: data.timestamp({"2026-06-01T09:30:00Z", "2026-06-01T09:31:00Z", "2026-06-02T09:30:30Z", "2026-06-02T09:31:00Z"}),
    price: data.f64({100.0, 101.0, 80.0, 200.0}),
    size: data.i64({10, 20, 30, 5}),
})

updated := q.sql(trades, "update price:price+1.0,size:size+1 from trades where sym=\"AAPL\"")
trimmed := q.sql(updated, "delete from trades where sym=\"TSLA\"")

venues := data.frame({
    venue: data.strings({"XNAS", "XNYS"}),
    region: data.symbols({"US", "US"}),
    tier: data.i64({1, 2}),
})

quotes := data.frame({
    sym: data.symbols({"AAPL", "AAPL", "MSFT"}),
    event_ts: data.timestamp({"2026-06-01T09:29:30Z", "2026-06-01T09:30:30Z", "2026-06-02T09:30:00Z"}),
    bid: data.f64({99.0, 100.5, 79.5}),
})

enriched := q.sql(
    "select trade_id,sym,venue,trade_date,event_ts,price,size,region,tier from trades left join venues on venue order by trade_id asc",
    {trades: trimmed, venues: venues}
)
asofed := q.sql(
    "select trade_id,sym,trade_date,event_ts,price,size,region,bid from enriched asof join quotes on sym,event_ts order by trade_id asc",
    {enriched: enriched, quotes: quotes}
)
rollup := q.sql(
    asofed,
    "select notional:sum price*size, fills:count i, first_bid:first bid, last_region:last region by sym,trade_date from trades where trade_date within (\"2026-06-01\" \"2026-06-02\") order by sym asc,trade_date asc"
)
empty_delete := q.sql(asofed, "delete from trades where event_ts=\"2026-06-03T00:00:00Z\"")
`)

	updated := interp.GetGlobal("updated").Table()
	if updated == nil || updated.Length() != 4 {
		t.Fatalf("updated len = %v, want 4", updated)
	}
	if got := updated.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("updated[1].price = %v, want 101", got)
	}
	if got := updated.RawGetInt(2).Table().RawGetString("size"); !got.IsInt() || got.Int() != 21 {
		t.Fatalf("updated[2].size = %v, want 21", got)
	}

	asofed := interp.GetGlobal("asofed").Table()
	if asofed == nil || asofed.Length() != 3 {
		t.Fatalf("asofed len = %v, want 3", asofed)
	}
	if got := asofed.RawGetInt(1).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 99 {
		t.Fatalf("asofed[1].bid = %v, want 99", got)
	}
	if got := asofed.RawGetInt(2).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("asofed[2].bid = %v, want 100.5", got)
	}
	if got := asofed.RawGetInt(3).Table().RawGetString("event_ts"); !got.IsString() || got.Str() != "2026-06-02T09:30:30Z" {
		t.Fatalf("asofed[3].event_ts = %v, want timestamp string", got)
	}
	if got := asofed.RawGetString("column_kinds").Table().RawGetString("event_ts"); !got.IsString() || got.Str() != "timestamp" {
		t.Fatalf("asofed.column_kinds.event_ts = %v, want timestamp", got)
	}

	rollup := interp.GetGlobal("rollup").Table()
	if rollup == nil || rollup.Length() != 2 {
		t.Fatalf("rollup len = %v, want 2", rollup)
	}
	first := rollup.RawGetInt(1).Table()
	if got := first.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rollup[1].sym = %v, want AAPL", got)
	}
	if got := first.RawGetString("trade_date"); !got.IsString() || got.Str() != "2026-06-01" {
		t.Fatalf("rollup[1].trade_date = %v, want 2026-06-01", got)
	}
	if got := first.RawGetString("notional"); !got.IsFloat() || got.Float() != 3253 {
		t.Fatalf("rollup[1].notional = %v, want 3253", got)
	}
	if got := first.RawGetString("first_bid"); !got.IsFloat() || got.Float() != 99 {
		t.Fatalf("rollup[1].first_bid = %v, want 99", got)
	}
	second := rollup.RawGetInt(2).Table()
	if got := second.RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("rollup[2].sym = %v, want MSFT", got)
	}
	if got := second.RawGetString("last_region"); !got.IsString() || got.Str() != "US" {
		t.Fatalf("rollup[2].last_region = %v, want US", got)
	}
	if got := rollup.RawGetString("column_kinds").Table().RawGetString("trade_date"); !got.IsString() || got.Str() != "date" {
		t.Fatalf("rollup.column_kinds.trade_date = %v, want date", got)
	}

	emptyDelete := interp.GetGlobal("empty_delete").Table()
	if emptyDelete == nil || emptyDelete.Length() != 3 {
		t.Fatalf("empty_delete len = %v, want 3", emptyDelete)
	}
	if got := emptyDelete.RawGetString("column_kinds").Table().RawGetString("event_ts"); !got.IsString() || got.Str() != "timestamp" {
		t.Fatalf("empty_delete.column_kinds.event_ts = %v, want timestamp", got)
	}
}

func TestQSQLTypedUnmatchedJoinAsofAndEmptyMutationBoundary(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := data.frame({
    trade_id: data.i64({1, 2}),
    sym: data.symbols({"AAPL", "MSFT"}),
    venue: data.strings({"XNAS", "XASE"}),
    event_ts: data.timestamp({"2026-06-01T09:30:00Z", "2026-06-01T09:31:00Z"}),
    price: data.f64({100.0, 80.0}),
})
updated := q.sql(trades, "update price:price+1.0 from trades where sym=\"IBM\"")
venues := data.frame({
    venue: data.strings({"XNAS"}),
    region: data.symbols({"US"}),
})
quotes := data.frame({
    sym: data.symbols({"AAPL"}),
    event_ts: data.timestamp({"2026-06-01T09:30:30Z"}),
    bid: data.f64({99.0}),
})
enriched := q.sql(
    "select trade_id,sym,venue,event_ts,price,region from trades left join venues on venue order by trade_id asc",
    {trades: updated, venues: venues}
)
asofed := q.sql(
    "select trade_id,sym,event_ts,price,region,bid from enriched asof join quotes on sym,event_ts order by trade_id asc",
    {enriched: enriched, quotes: quotes}
)
rollup := q.sql(asofed, "select fills:count i, first_bid:first bid by region from trades order by region asc")
empty := q.sql(asofed, "delete from trades where price>=0.0")
`)

	updated := interp.GetGlobal("updated").Table()
	if updated == nil || updated.Length() != 2 {
		t.Fatalf("updated len = %v, want 2", updated)
	}
	if got := updated.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("updated[1].price = %v, want unchanged 100", got)
	}

	enriched := interp.GetGlobal("enriched").Table()
	if enriched == nil || enriched.Length() != 2 {
		t.Fatalf("enriched len = %v, want 2", enriched)
	}
	if got := enriched.RawGetInt(1).Table().RawGetString("region"); !got.IsString() || got.Str() != "US" {
		t.Fatalf("enriched[1].region = %v, want US", got)
	}
	if got := enriched.RawGetInt(2).Table().RawGetString("region"); !got.IsNil() {
		t.Fatalf("enriched[2].region = %v, want nil unmatched left join value", got)
	}

	asofed := interp.GetGlobal("asofed").Table()
	if asofed == nil || asofed.Length() != 2 {
		t.Fatalf("asofed len = %v, want 2", asofed)
	}
	if got := asofed.RawGetInt(1).Table().RawGetString("bid"); !got.IsNil() {
		t.Fatalf("asofed[1].bid = %v, want nil because quote is after trade", got)
	}
	if got := asofed.RawGetInt(2).Table().RawGetString("bid"); !got.IsNil() {
		t.Fatalf("asofed[2].bid = %v, want nil unmatched symbol", got)
	}
	if got := asofed.RawGetString("column_kinds").Table().RawGetString("event_ts"); !got.IsString() || got.Str() != "timestamp" {
		t.Fatalf("asofed.column_kinds.event_ts = %v, want timestamp", got)
	}
	if got := asofed.RawGetString("column_kinds").Table().RawGetString("bid"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("asofed.column_kinds.bid = %v, want f64", got)
	}

	rollup := interp.GetGlobal("rollup").Table()
	if rollup == nil || rollup.Length() != 2 {
		t.Fatalf("rollup len = %v, want 2", rollup)
	}
	if got := rollup.RawGetInt(1).Table().RawGetString("region"); !got.IsNil() {
		t.Fatalf("rollup[1].region = %v, want nil group first", got)
	}
	if got := rollup.RawGetInt(1).Table().RawGetString("fills"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("rollup[1].fills = %v, want 1", got)
	}
	if got := rollup.RawGetInt(2).Table().RawGetString("region"); !got.IsString() || got.Str() != "US" {
		t.Fatalf("rollup[2].region = %v, want US", got)
	}

	empty := interp.GetGlobal("empty").Table()
	if empty == nil || empty.Length() != 0 {
		t.Fatalf("empty len = %v, want 0", empty)
	}
	if got := empty.RawGetString("column_kinds").Table().RawGetString("event_ts"); !got.IsString() || got.Str() != "timestamp" {
		t.Fatalf("empty.column_kinds.event_ts = %v, want timestamp", got)
	}
	if got := empty.RawGetString("column_kinds").Table().RawGetString("region"); !got.IsString() || got.Str() != "symbol" {
		t.Fatalf("empty.column_kinds.region = %v, want symbol", got)
	}
	if got := empty.RawGetString("column_kinds").Table().RawGetString("bid"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("empty.column_kinds.bid = %v, want f64", got)
	}
}

func TestQSQLWhereGreaterThanIsStrict(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp, `
frame := data.frame({
    sym: {"low", "edge", "high"},
    price: array.i64({99, 100, 101}),
})
rows := q.sql(frame, "select sym,price from frame where price>100")
`)
	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 1 {
		t.Fatalf("rows len = %v, want 1", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "high" {
		t.Fatalf("rows[1].sym = %v, want high", got)
	}
}

func TestQSQLAcceptsSQLFirstNamedFrame(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"AAPL\"},\n"+
			"    price: array.i64({100, 80, 120}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"i64\"}\n"+
			"rows := q.sql(\"select sym,price from trades where sym=`AAPL\", {trades: trades})\n"+
			"also := q.select(\"select price from trades where sym=`MSFT\", {trades: trades})\n")

	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("rows len = %v, want 2", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rows[1].sym = %v, want AAPL", got)
	}
	if got := rows.RawGetInt(2).Table().RawGetString("price"); !got.IsInt() || got.Int() != 120 {
		t.Fatalf("rows[2].price = %v, want 120", got)
	}
	also := interp.GetGlobal("also").Table()
	if also == nil || also.Length() != 1 {
		t.Fatalf("also len = %v, want 1", also)
	}
	if got := also.RawGetInt(1).Table().RawGetString("price"); !got.IsInt() || got.Int() != 80 {
		t.Fatalf("also[1].price = %v, want 80", got)
	}
}

func TestQSQLTypedFrameWhereAndOutputSemantics(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"frame := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"AAPL\", \"IBM\"},\n"+
			"    venue: {\"XNYS\", \"XNAS\", \"BATS\", \"XNYS\"},\n"+
			"    active: array.bool({true, false, true, true}),\n"+
			"    price: array.f64({100.5, 80.25, 120.0, 90.0}),\n"+
			"})\n"+
			"frame.column_kinds = {\n"+
			"    sym: \"symbol\",\n"+
			"    venue: \"string\",\n"+
			"    active: \"bool\",\n"+
			"    price: \"f64\",\n"+
			"}\n"+
			"symbol_rows := q.sql(frame, \"select sym,price from frame where sym=`AAPL\")\n"+
			"string_rows := q.sql(frame, \"select venue from frame where venue=`XNYS\")\n"+
			"bool_rows := q.sql(frame, \"select sym,active from frame where active=true\")\n")

	symbolRows := interp.GetGlobal("symbol_rows").Table()
	if symbolRows == nil || symbolRows.Length() != 2 {
		t.Fatalf("symbol_rows len = %v, want 2", symbolRows)
	}
	if got := symbolRows.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 120 {
		t.Fatalf("symbol_rows[2].price = %v, want 120", got)
	}
	stringRows := interp.GetGlobal("string_rows").Table()
	if stringRows == nil || stringRows.Length() != 2 {
		t.Fatalf("string_rows len = %v, want 2", stringRows)
	}
	if got := stringRows.RawGetInt(1).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNYS" {
		t.Fatalf("string_rows[1].venue = %v, want XNYS", got)
	}
	boolRows := interp.GetGlobal("bool_rows").Table()
	if boolRows == nil || boolRows.Length() != 3 {
		t.Fatalf("bool_rows len = %v, want 3", boolRows)
	}
	if got := boolRows.RawGetInt(1).Table().RawGetString("active"); !got.IsBool() || !got.Bool() {
		t.Fatalf("bool_rows[1].active = %v, want true", got)
	}
}

func TestQSQLPlanCacheKeepsSchemaLiteralAlignmentSeparate(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"symbol_frame := data.frame({name: {\"AAPL\", \"MSFT\"}})\n"+
			"symbol_frame.column_kinds = {name: \"symbol\"}\n"+
			"string_frame := data.frame({name: {\"AAPL\", \"MSFT\"}})\n"+
			"string_frame.column_kinds = {name: \"string\"}\n"+
			"src := \"select name from frame where name=`AAPL\"\n"+
			"symbol_rows := q.sql(symbol_frame, src)\n"+
			"string_rows := q.sql(string_frame, src)\n")

	symbolRows := interp.GetGlobal("symbol_rows").Table()
	if symbolRows == nil || symbolRows.Length() != 1 {
		t.Fatalf("symbol_rows len = %v, want 1", symbolRows)
	}
	if got := symbolRows.RawGetInt(1).Table().RawGetString("name"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("symbol_rows[1].name = %v, want AAPL", got)
	}
	stringRows := interp.GetGlobal("string_rows").Table()
	if stringRows == nil || stringRows.Length() != 1 {
		t.Fatalf("string_rows len = %v, want 1", stringRows)
	}
	if got := stringRows.RawGetInt(1).Table().RawGetString("name"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("string_rows[1].name = %v, want AAPL", got)
	}
}

func TestQSQLReturnsDataFrameCompatibleRowsAndTemporalStrings(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"events := data.frame({\n"+
			"    day: {\"2026-06-05\", \"2026-06-06\"},\n"+
			"    ts: {\"2026-06-05T09:30:00Z\", \"2026-06-06T09:30:00Z\"},\n"+
			"    active: array.bool({true, false}),\n"+
			"    qty: array.i64({10, 20}),\n"+
			"    px: array.f64({1.5, 2.25}),\n"+
			"    note: {nil, \"close\"},\n"+
			"})\n"+
			"events.column_kinds = {day: \"date\", ts: \"timestamp\", active: \"bool\", qty: \"i64\", px: \"f64\", note: \"string\"}\n"+
			"rows := q.sql(events, \"select day,ts,active,qty,px,note from events where active=true\")\n"+
			"data_rows := data.rows(rows)\n")

	rows := interp.GetGlobal("rows").Table()
	if rows == nil {
		t.Fatalf("rows = nil")
	}
	if got := rows.RawGetString("kind"); !got.IsString() || got.Str() != "data_frame" {
		t.Fatalf("rows.kind = %v, want data_frame", got)
	}
	if got := rows.RawGetString("len"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("rows.len = %v, want 1", got)
	}
	if got := rows.RawGetString("ncols"); !got.IsInt() || got.Int() != 6 {
		t.Fatalf("rows.ncols = %v, want 6", got)
	}
	oldRow := rows.RawGetInt(1).Table()
	if got := oldRow.RawGetString("day"); !got.IsString() || got.Str() != "2026-06-05" {
		t.Fatalf("rows[1].day = %v, want 2026-06-05", got)
	}
	if got := oldRow.RawGetString("ts"); !got.IsString() || got.Str() != "2026-06-05T09:30:00Z" {
		t.Fatalf("rows[1].ts = %v, want timestamp string", got)
	}
	if got := oldRow.RawGetString("active"); !got.IsBool() || !got.Bool() {
		t.Fatalf("rows[1].active = %v, want true", got)
	}
	if got := oldRow.RawGetString("qty"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("rows[1].qty = %v, want 10", got)
	}
	if got := oldRow.RawGetString("px"); !got.IsFloat() || got.Float() != 1.5 {
		t.Fatalf("rows[1].px = %v, want 1.5", got)
	}
	if got := oldRow.RawGetString("note"); !got.IsNil() {
		t.Fatalf("rows[1].note = %v, want nil", got)
	}
	if got := rows.RawGetString("rows").Table().RawGetInt(1).Table().RawGetString("day"); !got.IsString() || got.Str() != "2026-06-05" {
		t.Fatalf("rows.rows[1].day = %v, want 2026-06-05", got)
	}
	if got := rows.RawGetString("column_kinds").Table().RawGetString("day"); !got.IsString() || got.Str() != "date" {
		t.Fatalf("rows.column_kinds.day = %v, want date", got)
	}
	if got := rows.RawGetString("schema").Table().RawGetString("kinds").Table().RawGetString("ts"); !got.IsString() || got.Str() != "timestamp" {
		t.Fatalf("rows.schema.kinds.ts = %v, want timestamp", got)
	}
	dataRows := interp.GetGlobal("data_rows").Table()
	if got := dataRows.RawGetInt(1).Table().RawGetString("day"); !got.IsString() || got.Str() != "2026-06-05" {
		t.Fatalf("data.rows(rows)[1].day = %v, want 2026-06-05", got)
	}
}

func TestQSQLNullOutputFromTypedFrameWrapper(t *testing.T) {
	frame := NewTable()
	columns := NewTable()
	names := NewAppendArrayTable(2)
	kinds := NewTable()
	sym := NewAppendArrayTable(3)
	note := NewAppendArrayTable(3)
	for i, value := range []Value{StringValue("AAPL"), StringValue("MSFT"), StringValue("IBM")} {
		sym.RawSetInt(int64(i+1), value)
	}
	note.RawSetInt(1, StringValue("open"))
	note.RawSetInt(2, NilValue())
	note.RawSetInt(3, StringValue("close"))
	columns.RawSetString("sym", TableValue(sym))
	columns.RawSetString("note", TableValue(note))
	names.RawSetInt(1, StringValue("sym"))
	names.RawSetInt(2, StringValue("note"))
	kinds.RawSetString("sym", StringValue("symbol"))
	kinds.RawSetString("note", StringValue("string"))
	frame.RawSetString(dataFrameMarker, BoolValue(true))
	frame.RawSetString("len", IntValue(3))
	frame.RawSetString("columns", TableValue(columns))
	frame.RawSetString("column_names", TableValue(names))
	frame.RawSetString("column_kinds", TableValue(kinds))

	fn := BuildQ().RawGetString("sql").GoFunction()
	out, err := fn.Fn([]Value{TableValue(frame), StringValue("select sym,note from frame where note=null")})
	if err != nil {
		t.Fatalf("q.sql returned error: %v", err)
	}
	nullRows := out[0].Table()
	if nullRows == nil || nullRows.Length() != 1 {
		t.Fatalf("null_rows len = %v, want 1", nullRows)
	}
	if got := nullRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("null_rows[1].sym = %v, want MSFT", got)
	}
	if got := nullRows.RawGetInt(1).Table().RawGetString("note"); !got.IsNil() {
		t.Fatalf("null_rows[1].note = %v (%s), want nil", got, got.TypeName())
	}
}

func TestQSQLOrderByAndLimit(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"NVDA\", \"IBM\"},\n"+
			"    price: array.f64({100, 80, 120, 90}),\n"+
			"    active: array.bool({true, false, true, true}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\", active: \"bool\"}\n"+
			"top := q.sql(trades, \"select sym,price from trades where active=true order by price desc limit 2\")\n"+
			"bottom := q.sql(\"select sym,price from trades order by price asc limit 1\", {trades: trades})\n")

	top := interp.GetGlobal("top").Table()
	if top == nil || top.Length() != 2 {
		t.Fatalf("top len = %v, want 2", top)
	}
	if got := top.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("top[1].sym = %v, want NVDA", got)
	}
	if got := top.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("top[2].sym = %v, want AAPL", got)
	}
	bottom := interp.GetGlobal("bottom").Table()
	if bottom == nil || bottom.Length() != 1 {
		t.Fatalf("bottom len = %v, want 1", bottom)
	}
	if got := bottom.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("bottom[1].sym = %v, want MSFT", got)
	}
}

func TestQSQLOrderByComputedProjection(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"NVDA\"},\n"+
			"    price: array.f64({100, 80, 120}),\n"+
			"    size: array.i64({10, 30, 20}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\", size: \"i64\"}\n"+
			"default_name := q.sql(trades, \"select price*size from trades order by price*size desc\")\n"+
			"aliased := q.sql(trades, \"select sym,notional:price*size from trades order by notional desc\")\n")

	defaultName := interp.GetGlobal("default_name").Table()
	if defaultName == nil || defaultName.Length() != 3 {
		t.Fatalf("default_name len = %v, want 3", defaultName)
	}
	if got := defaultName.RawGetInt(1).Table().RawGetString("price*size"); !got.IsFloat() || got.Float() != 2400 {
		t.Fatalf("default_name[1][price*size] = %v, want 2400", got)
	}
	if got := defaultName.RawGetInt(3).Table().RawGetString("price*size"); !got.IsFloat() || got.Float() != 1000 {
		t.Fatalf("default_name[3][price*size] = %v, want 1000", got)
	}
	aliased := interp.GetGlobal("aliased").Table()
	if aliased == nil || aliased.Length() != 3 {
		t.Fatalf("aliased len = %v, want 3", aliased)
	}
	if got := aliased.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("aliased[1].sym = %v, want MSFT", got)
	}
	if got := aliased.RawGetInt(2).Table().RawGetString("notional"); !got.IsFloat() || got.Float() != 2400 {
		t.Fatalf("aliased[2].notional = %v, want 2400", got)
	}
}

func TestQSQLExposesLibQExecOrderLimitAndLiterals(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"events := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"NVDA\", \"IBM\"},\n"+
			"    venue: {\"XNYS\", \"XNAS\", \"XNYS\", \"XNYS\"},\n"+
			"    active: array.bool({true, true, true, false}),\n"+
			"    price: array.f64({100, 80, 120, 90}),\n"+
			"    note: {nil, \"open\", \"open\", \"halted\"},\n"+
			"})\n"+
			"events.column_kinds = {sym: \"symbol\", venue: \"string\", active: \"bool\", price: \"f64\", note: \"string\"}\n"+
			"prices := q.sql(events, \"exec price,sym from events where venue=\\\"XNYS\\\" order by price desc limit 2\")\n"+
			"nulls := q.select(\"exec sym,note from events where note=null\", {events: events})\n"+
			"live := q.sql(events, \"select sym,active from events where active=true order by sym asc limit 1\")\n"+
			"venues := q.sql(events, \"select distinct venue from events order by venue asc take 1\")\n"+
			"prefix := q.select(\"2#select sym,price from events order by price desc\", {events: events})\n")

	prices := interp.GetGlobal("prices").Table()
	if prices == nil {
		t.Fatalf("prices = nil, want dictionary")
	}
	if got := prices.RawGetString("sym").Table().RawGetInt(1); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("prices.sym[1] = %v, want NVDA", got)
	}
	got, err := prices.RawGetString("price").DenseArray().At(1)
	if err != nil || !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("prices.price[2] = %v, want 100", got)
	}
	nulls := interp.GetGlobal("nulls").Table()
	if nulls == nil {
		t.Fatalf("nulls = nil, want dictionary")
	}
	if got := nulls.RawGetString("sym").Table().RawGetInt(1); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("nulls.sym[1] = %v, want AAPL", got)
	}
	if got := nulls.RawGetString("note").Table().RawGetInt(1); !got.IsTable() || !got.Table().RawGetString("__data_null").Bool() {
		t.Fatalf("nulls.note[1] = %v, want q null marker", got)
	}
	live := interp.GetGlobal("live").Table()
	if live == nil || live.Length() != 1 {
		t.Fatalf("live len = %v, want 1", live)
	}
	if got := live.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("live[1].sym = %v, want AAPL", got)
	}
	venues := interp.GetGlobal("venues").Table()
	if venues == nil || venues.Length() != 1 {
		t.Fatalf("venues len = %v, want 1", venues)
	}
	if got := venues.RawGetInt(1).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNAS" {
		t.Fatalf("venues[1].venue = %v, want XNAS", got)
	}
	prefix := interp.GetGlobal("prefix").Table()
	if prefix == nil || prefix.Length() != 2 {
		t.Fatalf("prefix len = %v, want 2", prefix)
	}
	if got := prefix.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("prefix[1].sym = %v, want NVDA", got)
	}
	if got := prefix.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("prefix[2].sym = %v, want AAPL", got)
	}
}

func TestQSQLExecDictionaryProjectionReturnsDictionary(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"MSFT\", \"NVDA\"}),\n"+
			"    price: array.i64({100, 80, 120}),\n"+
			"    size: array.i64({10, 20, 30}),\n"+
			"})\n"+
			"prices_by_sym := q.sql(trades, \"exec sym!price from trades where size>=20 order by sym asc\")\n"+
			"notional_by_sym := q.sql(trades, \"exec sym!(price*size) from trades order by sym asc\")\n"+
			"price_keys := q.keys(prices_by_sym)\n"+
			"price_values := q.value(prices_by_sym)\n"+
			"notional_values := q.value(notional_by_sym)\n")

	keys := interp.GetGlobal("price_keys").Table()
	if keys == nil || keys.Length() != 2 {
		t.Fatalf("price_keys len = %v, want 2", keys)
	}
	if got := keys.RawGetInt(1); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("price_keys[1] = %v, want MSFT", got)
	}
	if got := keys.RawGetInt(2); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("price_keys[2] = %v, want NVDA", got)
	}
	values := interp.GetGlobal("price_values").Table()
	if values == nil || values.Length() != 2 {
		t.Fatalf("price_values len = %v, want 2", values)
	}
	if got := values.RawGetInt(1); !got.IsInt() || got.Int() != 80 {
		t.Fatalf("price_values[1] = %v, want 80", got)
	}
	if got := values.RawGetInt(2); !got.IsInt() || got.Int() != 120 {
		t.Fatalf("price_values[2] = %v, want 120", got)
	}
	dict := interp.GetGlobal("prices_by_sym").Table()
	if dict == nil || !dict.RawGetString("MSFT").IsInt() || dict.RawGetString("MSFT").Int() != 80 {
		t.Fatalf("prices_by_sym.MSFT = %v, want 80", interp.GetGlobal("prices_by_sym"))
	}
	notionals := interp.GetGlobal("notional_values").Table()
	if notionals == nil || notionals.Length() != 3 {
		t.Fatalf("notional_values len = %v, want 3", notionals)
	}
	if got := notionals.RawGetInt(1); !got.IsFloat() || got.Float() != 1000 {
		t.Fatalf("notional_values[1] = %v, want 1000", got)
	}
	if got := notionals.RawGetInt(3); !got.IsFloat() || got.Float() != 3600 {
		t.Fatalf("notional_values[3] = %v, want 3600", got)
	}
}

func TestQSQLUpdateDeleteExecution(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"AAPL\"},\n"+
			"    price: array.f64({100, 80, 120}),\n"+
			"    size: array.i64({10, 20, 30}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\", size: \"i64\"}\n"+
			"updated := q.sql(trades, \"update price:price+10 from trades where sym=`AAPL\")\n"+
			"enriched := q.sql(trades, \"update notional:price*size from trades where sym=`AAPL\")\n"+
			"grouped := q.sql(trades, \"update avg_price:avg price by sym from trades\")\n"+
			"trimmed := q.sql(trades, \"delete size from trades\")\n"+
			"deleted := q.select(updated, \"delete from updated where price<100\")\n"+
			"original := q.sql(trades, \"select sym,price from trades where sym=`AAPL order by price asc\")\n")

	updated := interp.GetGlobal("updated").Table()
	if updated == nil || updated.Length() != 3 {
		t.Fatalf("updated len = %v, want 3", updated)
	}
	if got := updated.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 110 {
		t.Fatalf("updated[1].price = %v, want 110", got)
	}
	if got := updated.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 80 {
		t.Fatalf("updated[2].price = %v, want 80", got)
	}
	if got := updated.RawGetInt(3).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 130 {
		t.Fatalf("updated[3].price = %v, want 130", got)
	}
	if got := updated.RawGetString("column_kinds").Table().RawGetString("price"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("updated.column_kinds.price = %v, want f64", got)
	}
	if rows := updated.RawGetString("rows").Table(); rows == nil || rows.Length() != 3 {
		t.Fatalf("updated.rows len = %v, want 3", rows)
	}

	enriched := interp.GetGlobal("enriched").Table()
	if enriched == nil || enriched.Length() != 3 {
		t.Fatalf("enriched len = %v, want 3", enriched)
	}
	if got := enriched.RawGetInt(1).Table().RawGetString("notional"); !got.IsFloat() || got.Float() != 1000 {
		t.Fatalf("enriched[1].notional = %v, want 1000", got)
	}
	if got := enriched.RawGetInt(2).Table().RawGetString("notional"); !got.IsNil() {
		t.Fatalf("enriched[2].notional = %v, want nil", got)
	}
	if got := enriched.RawGetInt(3).Table().RawGetString("notional"); !got.IsFloat() || got.Float() != 3600 {
		t.Fatalf("enriched[3].notional = %v, want 3600", got)
	}

	grouped := interp.GetGlobal("grouped").Table()
	if grouped == nil || grouped.Length() != 3 {
		t.Fatalf("grouped len = %v, want 3", grouped)
	}
	if got := grouped.RawGetInt(1).Table().RawGetString("avg_price"); !got.IsFloat() || got.Float() != 110 {
		t.Fatalf("grouped[1].avg_price = %v, want 110", got)
	}
	if got := grouped.RawGetInt(2).Table().RawGetString("avg_price"); !got.IsFloat() || got.Float() != 80 {
		t.Fatalf("grouped[2].avg_price = %v, want 80", got)
	}
	if got := grouped.RawGetInt(3).Table().RawGetString("avg_price"); !got.IsFloat() || got.Float() != 110 {
		t.Fatalf("grouped[3].avg_price = %v, want 110", got)
	}

	trimmed := interp.GetGlobal("trimmed").Table()
	if trimmed == nil || trimmed.Length() != 3 {
		t.Fatalf("trimmed len = %v, want 3", trimmed)
	}
	if got := trimmed.RawGetInt(1).Table().RawGetString("size"); !got.IsNil() {
		t.Fatalf("trimmed[1].size = %v, want nil", got)
	}
	if got := trimmed.RawGetString("column_kinds").Table().RawGetString("size"); !got.IsNil() {
		t.Fatalf("trimmed.column_kinds.size = %v, want nil", got)
	}

	deleted := interp.GetGlobal("deleted").Table()
	if deleted == nil || deleted.Length() != 2 {
		t.Fatalf("deleted len = %v, want 2", deleted)
	}
	if got := deleted.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("deleted[1].sym = %v, want AAPL", got)
	}
	if got := deleted.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 130 {
		t.Fatalf("deleted[2].price = %v, want 130", got)
	}

	original := interp.GetGlobal("original").Table()
	if original == nil || original.Length() != 2 {
		t.Fatalf("original len = %v, want 2", original)
	}
	if got := original.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("original[1].price = %v, want 100", got)
	}
}

func TestQSQLUpdateDeleteBoundarySemantics(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"NVDA\"},\n"+
			"    price: array.f64({100, 80, 120}),\n"+
			"    size: array.i64({10, 20, 30}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\", size: \"i64\"}\n"+
			"none_updated := q.sql(trades, \"update price:price+10 from trades where sym=`IBM\")\n"+
			"none_deleted := q.sql(trades, \"delete from trades where price>1000\")\n"+
			"all_deleted := q.sql(trades, \"delete from trades where price>=80\")\n"+
			"drop_missing_ok, drop_missing_err := pcall(func() {\n"+
			"    return q.sql(trades, \"delete missing from trades\")\n"+
			"})\n")

	noneUpdated := interp.GetGlobal("none_updated").Table()
	if noneUpdated == nil || noneUpdated.Length() != 3 {
		t.Fatalf("none_updated len = %v, want 3", noneUpdated)
	}
	if got := noneUpdated.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("none_updated[1].price = %v, want 100", got)
	}
	if got := noneUpdated.RawGetInt(3).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 120 {
		t.Fatalf("none_updated[3].price = %v, want 120", got)
	}

	noneDeleted := interp.GetGlobal("none_deleted").Table()
	if noneDeleted == nil || noneDeleted.Length() != 3 {
		t.Fatalf("none_deleted len = %v, want 3", noneDeleted)
	}
	if got := noneDeleted.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("none_deleted[2].sym = %v, want MSFT", got)
	}

	allDeleted := interp.GetGlobal("all_deleted").Table()
	if allDeleted == nil || allDeleted.Length() != 0 {
		t.Fatalf("all_deleted len = %v, want 0", allDeleted)
	}
	if got := allDeleted.RawGetString("column_kinds").Table().RawGetString("price"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("all_deleted.column_kinds.price = %v, want f64", got)
	}
	assertPCallErrorContains(t, interp, "drop_missing", "drop column")
}

func TestQSQLInsertUpsertExecution(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\"},\n"+
			"    price: array.f64({100, 80}),\n"+
			"    size: array.i64({10, 20}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\", size: \"i64\"}\n"+
			"inserted := q.sql(trades, \"insert into trades (sym,price) values (`TSLA,220)\")\n"+
			"column_ordered := q.sql(trades, \"insert into trades (size,sym,price) values (77,`IBM,101)\")\n"+
			"upserted := q.sql(inserted, \"upsert into trades values (`NVDA,190,30)\")\n"+
			"keyed := q.key_by(trades, \"sym\")\n"+
			"keyed_upserted := q.sql(keyed, \"upsert into trades (sym,price) values (`AAPL,101)\")\n"+
			"keyed_upserted_ordered := q.sql(keyed, \"upsert into trades (price,sym,size) values (102,`MSFT,25)\")\n"+
			"keyed_inserted := q.sql(keyed_upserted, \"insert into trades (sym,price,size) values (`TSLA,220,40)\")\n"+
			"dup_insert_ok, dup_insert_err := pcall(func() {\n"+
			"    return q.sql(keyed, \"insert into trades (sym,price) values (`AAPL,101)\")\n"+
			"})\n"+
			"unknown_column_insert_ok, unknown_column_insert_err := pcall(func() {\n"+
			"    return q.sql(trades, \"insert into trades (sym,missing) values (`IBM,1)\")\n"+
			"})\n"+
			"full_row_count_insert_ok, full_row_count_insert_err := pcall(func() {\n"+
			"    return q.sql(trades, \"insert into trades values (`IBM,101)\")\n"+
			"})\n"+
			"missing_key_insert_ok, missing_key_insert_err := pcall(func() {\n"+
			"    return q.sql(keyed, \"insert into trades (price,size) values (101,15)\")\n"+
			"})\n"+
			"missing_key_upsert_ok, missing_key_upsert_err := pcall(func() {\n"+
			"    return q.sql(keyed, \"upsert into trades (price,size) values (101,15)\")\n"+
			"})\n")

	inserted := interp.GetGlobal("inserted").Table()
	if inserted == nil || inserted.Length() != 3 {
		t.Fatalf("inserted len = %v, want 3", inserted)
	}
	if got := inserted.RawGetInt(3).Table().RawGetString("sym"); !got.IsString() || got.Str() != "TSLA" {
		t.Fatalf("inserted[3].sym = %v, want TSLA", got)
	}
	if got := inserted.RawGetInt(3).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 220 {
		t.Fatalf("inserted[3].price = %v, want 220", got)
	}
	if got := inserted.RawGetInt(3).Table().RawGetString("size"); !got.IsNil() {
		t.Fatalf("inserted[3].size = %v, want nil", got)
	}
	if got := inserted.RawGetString("column_kinds").Table().RawGetString("sym"); !got.IsString() || got.Str() != "symbol" {
		t.Fatalf("inserted.column_kinds.sym = %v, want symbol", got)
	}
	columnOrdered := interp.GetGlobal("column_ordered").Table()
	if columnOrdered == nil || columnOrdered.Length() != 3 {
		t.Fatalf("column_ordered len = %v, want 3", columnOrdered)
	}
	columnOrderedRow := columnOrdered.RawGetInt(3).Table()
	if got := columnOrderedRow.RawGetString("sym"); !got.IsString() || got.Str() != "IBM" {
		t.Fatalf("column_ordered[3].sym = %v, want IBM", got)
	}
	if got := columnOrderedRow.RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("column_ordered[3].price = %v, want 101", got)
	}
	if got := columnOrderedRow.RawGetString("size"); !got.IsInt() || got.Int() != 77 {
		t.Fatalf("column_ordered[3].size = %v, want 77", got)
	}
	upserted := interp.GetGlobal("upserted").Table()
	if upserted == nil || upserted.Length() != 4 {
		t.Fatalf("upserted len = %v, want 4", upserted)
	}
	if got := upserted.RawGetInt(4).Table().RawGetString("size"); !got.IsInt() || got.Int() != 30 {
		t.Fatalf("upserted[4].size = %v, want 30", got)
	}

	keyedUpserted := interp.GetGlobal("keyed_upserted").Table()
	keyedFrame := keyedUpserted.RawGetString("frame").Table()
	if keyedFrame == nil || keyedFrame.Length() != 2 {
		t.Fatalf("keyed_upserted.frame len = %v, want 2", keyedFrame)
	}
	if got := keyedFrame.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("keyed_upserted.frame[1].price = %v, want 101", got)
	}
	if got := keyedFrame.RawGetInt(1).Table().RawGetString("size"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("keyed_upserted.frame[1].size = %v, want unchanged 10", got)
	}
	keyedOrderedFrame := interp.GetGlobal("keyed_upserted_ordered").Table().RawGetString("frame").Table()
	if keyedOrderedFrame == nil || keyedOrderedFrame.Length() != 2 {
		t.Fatalf("keyed_upserted_ordered.frame len = %v, want 2", keyedOrderedFrame)
	}
	if got := keyedOrderedFrame.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 102 {
		t.Fatalf("keyed_upserted_ordered.frame[2].price = %v, want 102", got)
	}
	if got := keyedOrderedFrame.RawGetInt(2).Table().RawGetString("size"); !got.IsInt() || got.Int() != 25 {
		t.Fatalf("keyed_upserted_ordered.frame[2].size = %v, want 25", got)
	}
	keyedInserted := interp.GetGlobal("keyed_inserted").Table().RawGetString("frame").Table()
	if keyedInserted == nil || keyedInserted.Length() != 3 {
		t.Fatalf("keyed_inserted.frame len = %v, want 3", keyedInserted)
	}
	if got := keyedInserted.RawGetInt(3).Table().RawGetString("sym"); !got.IsString() || got.Str() != "TSLA" {
		t.Fatalf("keyed_inserted.frame[3].sym = %v, want TSLA", got)
	}
	assertPCallErrorContains(t, interp, "dup_insert", "duplicate key")
	assertPCallErrorContains(t, interp, "unknown_column_insert", "q insert column \"missing\" does not exist in target schema (sym,price,size)")
	assertPCallErrorContains(t, interp, "full_row_count_insert", "q insert values count 2 does not match target schema column count 3 (sym,price,size)")
	assertPCallErrorContains(t, interp, "missing_key_insert", "q insert keyed mutation requires key column \"sym\"")
	assertPCallErrorContains(t, interp, "missing_key_upsert", "q upsert keyed mutation requires key column \"sym\"")
}

func TestQSQLInnerJoinExecution(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"AAPL\"},\n"+
			"    price: array.f64({100, 80, 120}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\"}\n"+
			"quotes := data.frame({sym: {\"AAPL\", \"MSFT\"}, bid: array.f64({99, 79})})\n"+
			"quotes.column_kinds = {sym: \"symbol\", bid: \"f64\"}\n"+
			"joined := q.sql(\"select sym,price,bid from trades join quotes on sym order by price asc\", {trades: trades, quotes: quotes})\n"+
			"joined_ij := q.sql(\"select sym,price,bid from trades ij quotes on sym order by price asc\", {trades: trades, quotes: quotes})\n"+
			"missing_map_ok, missing_map_err := pcall(func() {\n"+
			"    return q.sql(trades, \"select sym,price,bid from trades join quotes on sym\")\n"+
			"})\n")

	joined := interp.GetGlobal("joined").Table()
	if joined == nil || joined.Length() != 3 {
		t.Fatalf("joined len = %v, want 3", joined)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("joined[1].sym = %v, want MSFT", got)
	}
	if got := joined.RawGetInt(2).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 99 {
		t.Fatalf("joined[2].bid = %v, want 99", got)
	}
	if got := joined.RawGetInt(3).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 120 {
		t.Fatalf("joined[3].price = %v, want 120", got)
	}
	joinedIJ := interp.GetGlobal("joined_ij").Table()
	if joinedIJ == nil || joinedIJ.Length() != joined.Length() {
		t.Fatalf("joined_ij len = %v, want %d", joinedIJ, joined.Length())
	}
	if got := joinedIJ.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("joined_ij[1].sym = %v, want MSFT", got)
	}
	if got := joinedIJ.RawGetInt(3).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 99 {
		t.Fatalf("joined_ij[3].bid = %v, want 99", got)
	}
	assertPCallErrorContains(t, interp, "missing_map", "q.sql: join queries require a source table map")
}

func TestQSQLInnerJoinExecutionWithMultipleAliasedKeys(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"accounts := data.frame({\n"+
			"    id: {\"a1\", \"a1\", \"a2\", \"a3\"},\n"+
			"    venue: {\"XNYS\", \"XNAS\", \"XNYS\", \"XNAS\"},\n"+
			"    value: array.f64({10, 20, 30, 40}),\n"+
			"})\n"+
			"accounts.column_kinds = {id: \"symbol\", venue: \"symbol\", value: \"f64\"}\n"+
			"fills := data.frame({\n"+
			"    account_id: {\"a1\", \"a1\", \"a2\", \"a2\"},\n"+
			"    exchange: {\"XNAS\", \"XNYS\", \"XNAS\", \"XNYS\"},\n"+
			"    qty: array.f64({200, 100, 999, 300}),\n"+
			"})\n"+
			"fills.column_kinds = {account_id: \"symbol\", exchange: \"symbol\", qty: \"f64\"}\n"+
			"joined := q.sql(\"select id,venue,value,qty from accounts join fills on id=account_id,venue=exchange order by value asc\", {accounts: accounts, fills: fills})\n")

	joined := interp.GetGlobal("joined").Table()
	if joined == nil || joined.Length() != 3 {
		t.Fatalf("joined len = %v, want 3", joined)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("id"); !got.IsString() || got.Str() != "a1" {
		t.Fatalf("joined[1].id = %v, want a1", got)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNYS" {
		t.Fatalf("joined[1].venue = %v, want XNYS", got)
	}
	if got := joined.RawGetInt(2).Table().RawGetString("qty"); !got.IsFloat() || got.Float() != 200 {
		t.Fatalf("joined[2].qty = %v, want 200", got)
	}
	if got := joined.RawGetInt(3).Table().RawGetString("value"); !got.IsFloat() || got.Float() != 30 {
		t.Fatalf("joined[3].value = %v, want 30", got)
	}
}

func TestQSQLJoinBindingCacheLiteralAndPathSources(t *testing.T) {
	qSQLResetPlanCachesForTest()

	dir := t.TempDir()
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	interp.SetGlobal("store_path", StringValue(dir))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"AAPL\", \"TSLA\"},\n"+
			"    price: array.f64({100, 80, 120, 90}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\"}\n"+
			"quotes := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\"},\n"+
			"    bid: array.f64({99, 79}),\n"+
			"})\n"+
			"quotes.column_kinds = {sym: \"symbol\", bid: \"f64\"}\n"+
			"src := \"select sym,price,bid from trades left join quotes on sym where price>=threshold order by price asc\"\n"+
			"first := q.sql(src, {trades: trades, quotes: quotes, threshold: 90.0})\n"+
			"second := q.sql(src, {trades: trades, quotes: quotes, threshold: 110.0})\n"+
			"literal_joined := q.sql(\n"+
			"    \"select sym,price,bid from ([] sym:`AAPL`MSFT; price:100 80) join quotes on sym order by price asc\",\n"+
			"    {quotes: quotes}\n"+
			")\n"+
			"assert(q.save_splayed(trades, store_path .. \"/splayed\"))\n"+
			"path_joined := q.sql(\n"+
			"    \"select sym,price,bid from `:\" .. store_path .. \"/splayed left join quotes on sym order by price asc\",\n"+
			"    {quotes: quotes}\n"+
			")\n")

	first := interp.GetGlobal("first").Table()
	if first == nil || first.Length() != 3 {
		t.Fatalf("first len = %v, want 3", first)
	}
	if got := first.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "TSLA" {
		t.Fatalf("first[1].sym = %v, want TSLA unmatched left row", got)
	}
	if got := first.RawGetInt(1).Table().RawGetString("bid"); !got.IsNil() {
		t.Fatalf("first[1].bid = %v, want nil unmatched left join value", got)
	}
	second := interp.GetGlobal("second").Table()
	if second == nil || second.Length() != 1 {
		t.Fatalf("second len = %v, want 1", second)
	}
	if got := second.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 120 {
		t.Fatalf("second[1].price = %v, want 120 after env threshold bind", got)
	}
	literalJoined := interp.GetGlobal("literal_joined").Table()
	if literalJoined == nil || literalJoined.Length() != 2 {
		t.Fatalf("literal_joined len = %v, want 2", literalJoined)
	}
	if got := literalJoined.RawGetInt(1).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 79 {
		t.Fatalf("literal_joined[1].bid = %v, want 79", got)
	}
	pathJoined := interp.GetGlobal("path_joined").Table()
	if pathJoined == nil || pathJoined.Length() != 4 {
		t.Fatalf("path_joined len = %v, want 4", pathJoined)
	}
	if got := pathJoined.RawGetInt(2).Table().RawGetString("bid"); !got.IsNil() {
		t.Fatalf("path_joined[2].bid = %v, want nil unmatched path left join value", got)
	}
	if got := pathJoined.RawGetInt(3).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 99 {
		t.Fatalf("path_joined[3].bid = %v, want 99", got)
	}

	qSQLTemplateCacheMu.Lock()
	if len(qSQLTemplateCache) != 3 {
		t.Fatalf("template cache entries = %d, want 3", len(qSQLTemplateCache))
	}
	for key, tmpl := range qSQLTemplateCache {
		if tmpl.join == nil {
			t.Fatalf("template cache %q missing join plan", key)
		}
		if got := tmpl.plan.Source.Len(); got != 0 {
			t.Fatalf("template cache %q Source.Len() = %d, want zero frame", key, got)
		}
	}
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	if len(qSQLAlignedPlanCache) != 3 {
		t.Fatalf("aligned cache entries = %d, want 3", len(qSQLAlignedPlanCache))
	}
	for key, plan := range qSQLAlignedPlanCache {
		if got := plan.Source.Len(); got != 0 {
			t.Fatalf("aligned cache %q Source.Len() = %d, want zero frame", key, got)
		}
		if len(plan.Source.Schema().Names()) != 0 {
			t.Fatalf("aligned cache %q stored source schema", key)
		}
	}
	qSQLAlignedPlanCacheMu.Unlock()

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.TemplateMisses != 3 || stats.TemplateHits != 1 {
		t.Fatalf("template cache stats = %+v, want 3 misses and 1 hit", stats)
	}
	if stats.AlignedMisses != 3 || stats.AlignedHits != 1 {
		t.Fatalf("aligned cache stats = %+v, want 3 misses and 1 hit", stats)
	}
}

func TestQCacheStatsAndClearPublicAPI(t *testing.T) {
	qClearCaches()

	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\"},\n"+
			"    price: array.f64({100, 80}),\n"+
			"    size: array.f64({10, 20}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\", size: \"f64\"}\n"+
			"before := q.cache_stats()\n"+
			"first_eval := q.eval(\"1+2\")\n"+
			"second_eval := q.eval(\"1+2\")\n"+
			"src := \"select px:price from trades where price>=90\"\n"+
			"first_sql := q.sql(trades, src)\n"+
			"second_sql := q.sql(trades, src)\n"+
			"stats := q.cache_stats()\n"+
			"cleared := q.cache_clear()\n"+
			"after := q.cache_stats()\n")

	before := qTestCacheStatsRows(t, interp.GetGlobal("before").Table())
	if before["qsql_template"]["entries"] != 0 || before["qsql_aligned"]["entries"] != 0 || before["qsql_kernel"]["entries"] != 0 || before["qsql_kernel_decision"]["entries"] != 0 || before["q_runtime_kernel_execution"]["entries"] != 0 || before["q_eval"]["entries"] != 0 {
		t.Fatalf("initial cache entries = %#v, want all zero", before)
	}

	stats := qTestCacheStatsRows(t, interp.GetGlobal("stats").Table())
	if got := stats["qsql_template"]; got["entries"] != 1 || got["hits"] != 1 || got["misses"] != 1 || got["evictions"] != 0 {
		t.Fatalf("qsql_template stats = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", got)
	}
	if got := stats["qsql_aligned"]; got["entries"] != 1 || got["hits"] != 1 || got["misses"] != 1 || got["evictions"] != 0 {
		t.Fatalf("qsql_aligned stats = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", got)
	}
	if got := stats["qsql_kernel"]; got["entries"] != 1 || got["hits"] != 1 || got["misses"] != 1 || got["evictions"] != 0 {
		t.Fatalf("qsql_kernel stats = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", got)
	}
	if got := stats["qsql_kernel_decision"]; got["entries"] != 0 || got["hits"] != 0 || got["misses"] != 0 || got["evictions"] != 0 {
		t.Fatalf("qsql_kernel_decision stats = %#v, want no unsupported decisions for supported query", got)
	}
	if got := stats["q_eval"]; got["entries"] != 1 || got["hits"] != 1 || got["misses"] != 1 || got["evictions"] != 0 {
		t.Fatalf("q_eval stats = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", got)
	}
	runtimeRow := qTestCacheStatsRowTable(t, interp.GetGlobal("stats").Table(), "q_runtime_kernel_execution")
	if got := runtimeRow.RawGetString("stats_domain"); !got.IsString() || got.Str() != qStatsDomainJITExecution {
		t.Fatalf("q_runtime_kernel_execution stats_domain = %v, want %s", got, qStatsDomainJITExecution)
	}
	if got := runtimeRow.RawGetString("stats_source"); !got.IsString() || got.Str() != qStatsSourceMethodJIT {
		t.Fatalf("q_runtime_kernel_execution stats_source = %v, want %s", got, qStatsSourceMethodJIT)
	}
	if got := runtimeRow.RawGetString("cache_backed"); !got.IsBool() || got.Bool() {
		t.Fatalf("q_runtime_kernel_execution cache_backed = %v, want false", got)
	}
	if got := runtimeRow.RawGetString("executions"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("q_runtime_kernel_execution executions = %v, want 0", got)
	}
	if got := runtimeRow.RawGetString("successes"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("q_runtime_kernel_execution successes = %v, want 0", got)
	}
	if got := runtimeRow.RawGetString("errors"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("q_runtime_kernel_execution errors = %v, want 0", got)
	}
	if stats := runtimeRow.RawGetString("stats").Table(); stats == nil || stats.Length() != 0 {
		t.Fatalf("q_runtime_kernel_execution stats = %v, want empty table until MethodJIT diagnostics are attached", stats)
	}
	if shapes := runtimeRow.RawGetString("shapes").Table(); shapes == nil || shapes.Length() != 0 {
		t.Fatalf("q_runtime_kernel_execution shapes = %v, want empty table until MethodJIT diagnostics are attached", shapes)
	}
	if kernels := runtimeRow.RawGetString("kernels").Table(); kernels == nil || kernels.Length() != 0 {
		t.Fatalf("q_runtime_kernel_execution kernels = %v, want empty table until MethodJIT diagnostics are attached", kernels)
	}
	if routes := runtimeRow.RawGetString("routes").Table(); routes == nil || routes.Length() != 0 {
		t.Fatalf("q_runtime_kernel_execution routes = %v, want empty table until MethodJIT diagnostics are attached", routes)
	}
	kernelRow := qTestCacheStatsRowTable(t, interp.GetGlobal("stats").Table(), "qsql_kernel")
	if got := kernelRow.RawGetString("stats_domain"); !got.IsString() || got.Str() != qStatsDomainSemanticCache {
		t.Fatalf("qsql_kernel stats_domain = %v, want %s", got, qStatsDomainSemanticCache)
	}
	if got := kernelRow.RawGetString("cache_backed"); !got.IsBool() || !got.Bool() {
		t.Fatalf("qsql_kernel cache_backed = %v, want true", got)
	}
	evalRow := qTestCacheStatsRowTable(t, interp.GetGlobal("stats").Table(), "q_eval")
	if got := evalRow.RawGetString("stats_domain"); !got.IsString() || got.Str() != qStatsDomainEvalCache {
		t.Fatalf("q_eval stats_domain = %v, want %s", got, qStatsDomainEvalCache)
	}

	cleared := qTestCacheStatsRows(t, interp.GetGlobal("cleared").Table())
	after := qTestCacheStatsRows(t, interp.GetGlobal("after").Table())
	for name, row := range cleared {
		if row["entries"] != 0 || row["hits"] != 0 || row["misses"] != 0 || row["evictions"] != 0 {
			t.Fatalf("cleared %s stats = %#v, want zeroed", name, row)
		}
	}
	for name, row := range after {
		if row["entries"] != 0 || row["hits"] != 0 || row["misses"] != 0 || row["evictions"] != 0 {
			t.Fatalf("after clear %s stats = %#v, want zeroed", name, row)
		}
	}
}

func TestQRuntimeKernelExecutionStatsProviderFeedsCacheStats(t *testing.T) {
	qClearCaches()
	restore := SetQRuntimeKernelExecutionStatsProvider(func() []QRuntimeKernelExecutionStat {
		return []QRuntimeKernelExecutionStat{
			{
				Source:  "methodjit_q_frame_runtime",
				Kernel:  "QFrameSelectColumn",
				Shape:   "compare/filter/project/column",
				Route:   "typed_runtime_op_exit",
				Outcome: "success",
				Count:   2,
			},
			{
				Source:  "methodjit_q_frame_runtime",
				Kernel:  "QFrameSelectColumn",
				Shape:   "compare/filter/project/column",
				Route:   "typed_runtime_op_exit",
				Outcome: "success",
				Count:   3,
			},
			{
				Source:  "methodjit_q_frame_runtime",
				Kernel:  "QFrameSelectColumn",
				Shape:   "compare/filter/project/column",
				Route:   "typed_runtime_op_exit",
				Outcome: "error",
				Count:   1,
			},
			{
				Source:  "methodjit_q_vector_runtime",
				Kernel:  "QVectorWhereReduce",
				Shape:   "compare/vector-where/vector-reduce",
				Route:   "typed_runtime_op_exit",
				Outcome: "success",
				Count:   4,
			},
			{
				Source: "",
				Kernel: "",
				Shape:  "",
				Route:  "",
				Count:  0,
			},
		}
	})
	defer restore()

	rows := qTestCacheStatsRows(t, qCacheStatsTable())
	if got := rows["qsql_kernel"]; got["entries"] != 0 || got["hits"] != 0 || got["misses"] != 0 || got["evictions"] != 0 {
		t.Fatalf("qsql_kernel stats = %#v, want semantic cache untouched by runtime execution stats", got)
	}
	if got := rows["q_runtime_kernel_execution"]; got["entries"] != 3 || got["hits"] != 0 || got["misses"] != 0 || got["evictions"] != 0 || got["limit"] != 0 {
		t.Fatalf("q_runtime_kernel_execution stats = %#v, want three non-cache execution rows", got)
	}

	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_execution")
	if got := row.RawGetString("stats_domain"); !got.IsString() || got.Str() != qStatsDomainJITExecution {
		t.Fatalf("q_runtime_kernel_execution stats_domain = %v, want %s", got, qStatsDomainJITExecution)
	}
	if got := row.RawGetString("cache_backed"); !got.IsBool() || got.Bool() {
		t.Fatalf("q_runtime_kernel_execution cache_backed = %v, want false", got)
	}
	if got := row.RawGetString("executions"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("q_runtime_kernel_execution executions = %v, want 10", got)
	}
	if got := row.RawGetString("successes"); !got.IsInt() || got.Int() != 9 {
		t.Fatalf("q_runtime_kernel_execution successes = %v, want 9", got)
	}
	if got := row.RawGetString("errors"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("q_runtime_kernel_execution errors = %v, want 1", got)
	}

	stats := row.RawGetString("stats").Table()
	if stats == nil || stats.Length() != 3 {
		t.Fatalf("q_runtime_kernel_execution stats table = %v, want three rows", stats)
	}
	first := stats.RawGetInt(1).Table()
	if first == nil {
		t.Fatal("q_runtime_kernel_execution stats[1] is nil")
	}
	for field, want := range map[string]string{
		"source":  "methodjit_q_frame_runtime",
		"kernel":  "QFrameSelectColumn",
		"shape":   "compare/filter/project/column",
		"route":   "typed_runtime_op_exit",
		"outcome": "error",
	} {
		if got := first.RawGetString(field); !got.IsString() || got.Str() != want {
			t.Fatalf("q_runtime_kernel_execution stats[1].%s = %v, want %q", field, got, want)
		}
	}
	if got := first.RawGetString("count"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("q_runtime_kernel_execution stats[1].count = %v, want 1", got)
	}

	shapes := row.RawGetString("shapes").Table()
	if shapes == nil || shapes.Length() != 3 {
		t.Fatalf("q_runtime_kernel_execution shapes table = %v, want three rows", shapes)
	}
	top := shapes.RawGetInt(1).Table()
	if top == nil {
		t.Fatal("q_runtime_kernel_execution shapes[1] is nil")
	}
	if got := top.RawGetString("source"); !got.IsString() || got.Str() != "methodjit_q_frame_runtime" {
		t.Fatalf("q_runtime_kernel_execution shapes[1].source = %v, want methodjit_q_frame_runtime", got)
	}
	if got := top.RawGetString("shape"); !got.IsString() || got.Str() != "compare/filter/project/column" {
		t.Fatalf("q_runtime_kernel_execution shapes[1].shape = %v, want compare/filter/project/column", got)
	}
	if got := top.RawGetString("outcome"); !got.IsString() || got.Str() != "success" {
		t.Fatalf("q_runtime_kernel_execution shapes[1].outcome = %v, want success", got)
	}
	if got := top.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution shapes[1].count = %v, want 5", got)
	}
	kernels := row.RawGetString("kernels").Table()
	if kernels == nil || kernels.Length() != 3 {
		t.Fatalf("q_runtime_kernel_execution kernels table = %v, want three rows", kernels)
	}
	kernel := kernels.RawGetInt(1).Table()
	if kernel == nil {
		t.Fatal("q_runtime_kernel_execution kernels[1] is nil")
	}
	if got := kernel.RawGetString("source"); !got.IsString() || got.Str() != "methodjit_q_frame_runtime" {
		t.Fatalf("q_runtime_kernel_execution kernels[1].source = %v, want methodjit_q_frame_runtime", got)
	}
	if got := kernel.RawGetString("kernel"); !got.IsString() || got.Str() != "QFrameSelectColumn" {
		t.Fatalf("q_runtime_kernel_execution kernels[1].kernel = %v, want QFrameSelectColumn", got)
	}
	if got := kernel.RawGetString("outcome"); !got.IsString() || got.Str() != "success" {
		t.Fatalf("q_runtime_kernel_execution kernels[1].outcome = %v, want success", got)
	}
	if got := kernel.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution kernels[1].count = %v, want 5", got)
	}
	routes := row.RawGetString("routes").Table()
	if routes == nil || routes.Length() != 3 {
		t.Fatalf("q_runtime_kernel_execution routes table = %v, want three rows", routes)
	}
	route := routes.RawGetInt(1).Table()
	if route == nil {
		t.Fatal("q_runtime_kernel_execution routes[1] is nil")
	}
	for field, want := range map[string]string{
		"source":  "methodjit_q_frame_runtime",
		"kernel":  "QFrameSelectColumn",
		"route":   "typed_runtime_op_exit",
		"outcome": "success",
	} {
		if got := route.RawGetString(field); !got.IsString() || got.Str() != want {
			t.Fatalf("q_runtime_kernel_execution routes[1].%s = %v, want %q", field, got, want)
		}
	}
	if got := route.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution routes[1].count = %v, want 5", got)
	}
}

func TestQRuntimeKernelExecutionStatsProviderCleanupIsGenerationGuarded(t *testing.T) {
	qClearCaches()
	restoreA := SetQRuntimeKernelExecutionStatsProvider(func() []QRuntimeKernelExecutionStat {
		return []QRuntimeKernelExecutionStat{{
			Source:  "methodjit_q_frame_runtime",
			Kernel:  "QFrameSelectColumn",
			Shape:   "compare/filter/project/column",
			Route:   "typed_runtime_op_exit",
			Outcome: "success",
			Count:   1,
		}}
	})
	restoreB := SetQRuntimeKernelExecutionStatsProvider(func() []QRuntimeKernelExecutionStat {
		return []QRuntimeKernelExecutionStat{
			{
				Source:  "methodjit_q_frame_runtime",
				Kernel:  "QFrameSelectColumn",
				Shape:   "compare/filter/project/column",
				Route:   "typed_runtime_op_exit",
				Outcome: "success",
				Count:   2,
			},
			{
				Source:  "methodjit_q_frame_runtime",
				Kernel:  "QFrameSelectColumn",
				Shape:   "compare/filter/project/column",
				Route:   "typed_runtime_op_exit",
				Outcome: "success",
				Count:   3,
			},
		}
	})
	defer restoreB()

	restoreA()
	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_execution")
	if got := row.RawGetString("executions"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution executions after stale restore = %v, want provider B total 5", got)
	}
	shapes := row.RawGetString("shapes").Table()
	if shapes == nil || shapes.Length() != 1 {
		t.Fatalf("q_runtime_kernel_execution shapes after stale restore = %v, want one aggregated shape", shapes)
	}
	shape := shapes.RawGetInt(1).Table()
	if shape == nil {
		t.Fatal("q_runtime_kernel_execution shapes[1] is nil")
	}
	if got := shape.RawGetString("count"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution shapes[1].count = %v, want duplicate provider rows aggregated to 5", got)
	}

	qClearCaches()
	cleared := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_execution")
	if got := cleared.RawGetString("executions"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("q_runtime_kernel_execution executions after q.cache_clear-equivalent = %v, want provider stats retained", got)
	}

	restoreB()
	empty := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_runtime_kernel_execution")
	if got := empty.RawGetString("executions"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("q_runtime_kernel_execution executions after current restore = %v, want 0", got)
	}
}

func TestQFallbackStatsTrackKernelFallback(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 101, 80})},
	)
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	plan := data.QueryPlan{
		Select: []data.SelectItem{{Name: "marker", Expr: qFallbackStatsTestExpr{}}},
		LimitN: -1,
	}
	out, err := qRunSQLPlan("fallback-stats-test", plan, frame)
	if err != nil {
		t.Fatalf("fallback qRunSQLPlan: %v", err)
	}
	marker, ok := out.Column("marker")
	if !ok {
		t.Fatal("fallback output missing marker column")
	}
	if got, ok := marker.At(1); !ok || got != data.Symbol("fallback") {
		t.Fatalf("fallback marker row 1 = %v, %v; want fallback symbol", got, ok)
	}

	stats := qTestFallbackStatsRows(t, qFallbackStatsTable())
	if got := stats[qFallbackKernelUnsupported]; got != 1 {
		t.Fatalf("kernel unsupported fallback count = %d, want 1", got)
	}
	for _, code := range []string{qFallbackKernelCompileErr, qFallbackSourceErr, qFallbackJoinErr, qFallbackMutationPlan} {
		if got := stats[code]; got != 0 {
			t.Fatalf("fallback count %s = %d, want 0", code, got)
		}
	}
	if got := stats[qFallbackQueryKernel]; got != 0 {
		t.Fatalf("query kernel fallback count = %d, want 0", got)
	}
	if got := stats[qQueryKernelSupported]; got != 0 {
		t.Fatalf("query kernel hit count = %d, want 0", got)
	}

	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatalf("qDataFrameValue: %v", err)
	}
	explainValue, err := qExplainSQL(qSQLArgsResult{frameValue: frameValue, source: "update price:price+1 from trades"})
	if err != nil {
		t.Fatalf("qExplainSQL mutation: %v", err)
	}
	explained := explainValue.Table()
	if explained == nil {
		t.Fatal("explained table is nil")
	}
	if got := explained.RawGetString("kernel_reason_code"); !got.IsString() || got.Str() != qKernelReasonMutationPlan {
		t.Fatalf("kernel_reason_code = %v, want %s", got, qKernelReasonMutationPlan)
	}
	if got := explained.RawGetString("kernel_reason"); !got.IsString() || !strings.Contains(got.Str(), "mutation plan cache") {
		t.Fatalf("kernel_reason = %v, want mutation plan cache reason", got)
	}
	if got := explained.RawGetString("kernel_fallback_code"); !got.IsString() || got.Str() != qFallbackMutationPlan {
		t.Fatalf("kernel_fallback_code = %v, want %s", got, qFallbackMutationPlan)
	}
	if got := explained.RawGetString("kernel_fallback_family"); !got.IsString() || got.Str() != qFallbackFamilyMutation {
		t.Fatalf("kernel_fallback_family = %v, want %s", got, qFallbackFamilyMutation)
	}
	if got := explained.RawGetString("kernel_fallback_reason_code_count"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("kernel_fallback_reason_code_count = %v, want 0 before mutation execution", got)
	}
	if got := explained.RawGetString("kernel_fallback_reason_count"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("kernel_fallback_reason_count = %v, want 0 before mutation execution", got)
	}

	qClearCaches()
	after := qTestFallbackStatsRows(t, qFallbackStatsTable())
	for code, count := range after {
		if count != 0 {
			t.Fatalf("fallback count after clear %s = %d, want 0", code, count)
		}
	}
}

func TestQFallbackStatsTrackQueryKernelFallback(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	interp := runWithQAndSOA(t, `
trades := soa.zip({price: []f64{100, 101}})
rows := q.query(trades, {select: {price: "price"}, order_by: "missing"})
again := q.query(trades, {select: {price: "price"}, order_by: "missing"})
`)
	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("rows length = %v, want 2", rows)
	}
	again := interp.GetGlobal("again").Table()
	if again == nil || again.Length() != 2 {
		t.Fatalf("again length = %v, want 2", again)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("rows[1].price = %v, want 100", got)
	}
	stats := qTestFallbackStatsRows(t, qFallbackStatsTable())
	if got := stats[qFallbackQueryKernel]; got != 2 {
		t.Fatalf("query kernel fallback count = %d, want 2", got)
	}
	if got := stats[qQueryKernelSupported]; got != 0 {
		t.Fatalf("query kernel hit count = %d, want 0", got)
	}
	details := qTestFallbackStatsDetailRows(t, qFallbackStatsTable())
	if got := qTestFallbackFamilyCount(details, qFallbackFamilyOrder); got != 2 {
		t.Fatalf("query kernel order family count = %d, want 2", got)
	}
	if got := qTestFallbackDetailCount(details, "reason_code", qFallbackQueryKernel, qQueryKernelReasonOrder, ""); got != 2 {
		t.Fatalf("query kernel reason_code count = %d, want 2", got)
	}
	if got := qTestFallbackDetailFamily(details, "reason_code", qFallbackQueryKernel, qQueryKernelReasonOrder, ""); got != qFallbackFamilyOrder {
		t.Fatalf("query kernel reason_code family = %q, want %s", got, qFallbackFamilyOrder)
	}
	reason := `query native kernel order failed: FRAME_ORDER_INDEXES unknown column "missing"`
	if got := qTestFallbackDetailCount(details, "reason", qFallbackQueryKernel, "", reason); got != 2 {
		t.Fatalf("query kernel reason detail count = %d, want 2", got)
	}
	cacheStats := qTestCacheStatsRows(t, qCacheStatsTable())
	cache := cacheStats["q_query_kernel"]
	if cache["entries"] != 1 || cache["hits"] != 1 || cache["misses"] != 1 || cache["evictions"] != 0 {
		t.Fatalf("q_query_kernel unsupported stats = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", cache)
	}
	queryKernelRow := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_query_kernel")
	keyRows := qTestQueryKernelKeyRows(t, queryKernelRow.RawGetString("keys").Table())
	if len(keyRows) != 1 {
		t.Fatalf("q_query_kernel order keys = %+v, want one row", keyRows)
	}
	if got := keyRows[0]; got.Namespace != "q.query" || got.Kind != "query_kernel" || got.PlanFingerprint != "" || got.Supported || got.ReasonFamily != qFallbackFamilyOrder || got.ReasonCode != qQueryKernelReasonOrder || got.Hits != 1 || got.Misses != 1 || got.Evictions != 0 {
		t.Fatalf("q_query_kernel order key row = %+v, want unsupported order hit=1 miss=1", got)
	}
	keyJSONRows := qTestQueryKernelSupportKeyJSONRows(t)
	if len(keyJSONRows) != 1 {
		t.Fatalf("QQueryKernelSupportKeyStatJSONRows = %+v, want one unsupported row", keyJSONRows)
	}
	if got := keyJSONRows[0]; got.Key != keyRows[0].Key || got.Namespace != "q.query" || got.Kind != "query_kernel" || got.PlanFingerprint != "" || got.Supported || got.ReasonFamily != qFallbackFamilyOrder || got.ReasonCode != qQueryKernelReasonOrder || got.SchemaHash != keyRows[0].SchemaHash || got.Shape != keyRows[0].Shape || got.Hits != 1 || got.Misses != 1 || got.Evictions != 0 {
		t.Fatalf("QQueryKernelSupportKeyStatJSONRows[0] = %+v, want stable unsupported q.query key row", got)
	}
	keyJSON, err := json.Marshal(keyJSONRows)
	if err != nil {
		t.Fatalf("marshal QQueryKernelSupportKeyStatJSONRows: %v", err)
	}
	if !strings.Contains(string(keyJSON), `"reason_family"`) || !strings.Contains(string(keyJSON), `"schema_hash"`) || !strings.Contains(string(keyJSON), `"plan_fingerprint"`) || strings.Contains(string(keyJSON), "ReasonFamily") || strings.Contains(string(keyJSON), "SchemaHash") || strings.Contains(string(keyJSON), "PlanFingerprint") {
		t.Fatalf("QQueryKernelSupportKeyStatJSONRows JSON = %s, want snake_case stable fields", keyJSON)
	}
	shapeRows := qTestQueryKernelShapeRows(t, queryKernelRow.RawGetString("shapes").Table())
	if len(shapeRows) != 1 {
		t.Fatalf("q_query_kernel order shapes = %+v, want one row", shapeRows)
	}
	if got := qTestQueryKernelShapeCount(shapeRows, false, qFallbackFamilyOrder, qQueryKernelReasonOrder, shapeRows[0].SchemaHash); got != 1 {
		t.Fatalf("q_query_kernel order shape count = %d, want 1", got)
	}
	if got := qTestFallbackAttributionCount(details, qFallbackQueryKernel, qQueryKernelReasonOrder, "q.query", shapeRows[0].SchemaHash, shapeRows[0].Shape); got != 2 {
		t.Fatalf("q_query_kernel fallback attribution count = %d, want 2 for schema/shape", got)
	}
}

func TestQFallbackStatsTrackQueryKernelHits(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	interp := runWithQAndSOA(t, `
trades := soa.zip({price: []f64{100, 101}, size: []f64{10, 20}})
rows := q.query(trades, {select: {notional: {"*", "price", "size"}}})
`)
	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("rows length = %v, want 2", rows)
	}
	stats := qTestFallbackStatsRows(t, qFallbackStatsTable())
	if got := stats[qQueryKernelSupported]; got != 1 {
		t.Fatalf("query kernel hit count = %d, want 1", got)
	}
	if got := stats[qFallbackQueryKernel]; got != 0 {
		t.Fatalf("query kernel fallback count = %d, want 0", got)
	}
}

func TestQFallbackStatsExplainQueryKernelSelectReason(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	trades, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{100, 101}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	selects := NewTable()
	selects.RawSetString("bad", FunctionValue(&GoFunction{
		Name: "bad_select_constant",
		Fn: func(args []Value) ([]Value, error) {
			return []Value{NilValue()}, nil
		},
	}))
	spec := NewTable()
	spec.RawSetString("select", TableValue(selects))

	rows, err := qRunQuery(trades, spec)
	if err != nil {
		t.Fatalf("qRunQuery: %v", err)
	}
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("rows length = %v, want 2", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("bad"); !got.IsFunction() {
		t.Fatalf("rows[1].bad = %v, want function", got)
	}
	explained, err := qExplainQuery(trades, spec)
	if err != nil {
		t.Fatalf("qExplainQuery: %v", err)
	}
	if got := explained.RawGetString("kernel_reason_code"); !got.IsString() || got.Str() != qQueryKernelReasonSelect {
		t.Fatalf("kernel_reason_code = %v, want %s", got, qQueryKernelReasonSelect)
	}
	reason := `select expression "bad" is not supported by q query native kernel: expression type function is not native-kernel supported`
	if got := explained.RawGetString("kernel_reason"); !got.IsString() || got.Str() != reason {
		t.Fatalf("kernel_reason = %v, want %q", got, reason)
	}
	if got := explained.RawGetString("kernel_fallback_code"); !got.IsString() || got.Str() != qFallbackQueryKernel {
		t.Fatalf("kernel_fallback_code = %v, want %s", got, qFallbackQueryKernel)
	}
	if got := explained.RawGetString("kernel_fallback_family"); !got.IsString() || got.Str() != qFallbackFamilySelect {
		t.Fatalf("kernel_fallback_family = %v, want %s", got, qFallbackFamilySelect)
	}
	if got := explained.RawGetString("kernel_fallback_reason_code_count"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("kernel_fallback_reason_code_count = %v, want 1", got)
	}
	if got := explained.RawGetString("kernel_fallback_reason_count"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("kernel_fallback_reason_count = %v, want 1", got)
	}
	details := qTestFallbackStatsDetailRows(t, qFallbackStatsTable())
	if got := qTestFallbackDetailCount(details, "reason", qFallbackQueryKernel, "", reason); got != 1 {
		t.Fatalf("query kernel select reason detail count = %d, want 1", got)
	}
	if got := qTestFallbackDetailFamily(details, "reason", qFallbackQueryKernel, "", reason); got != qFallbackFamilySelect {
		t.Fatalf("query kernel select reason family = %q, want %s", got, qFallbackFamilySelect)
	}
}

func TestQQueryKernelSupportCacheStats(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	interp := runWithQAndSOA(t, `
trades := soa.zip({price: []f64{100, 101}, size: []f64{10, 20}})
first := q.query(trades, {select: {notional: {"*", "price", "size"}, large: {">=", "size", 15}}})
second := q.query(trades, {select: {large: {">=", "size", 15}, notional: {"*", "price", "size"}}})
explained := q.explain_query(trades, {select: {large: {">=", "size", 15}, notional: {"*", "price", "size"}}})
`)
	first := interp.GetGlobal("first").Table()
	second := interp.GetGlobal("second").Table()
	if first == nil || first.Length() != 2 || second == nil || second.Length() != 2 {
		t.Fatalf("query results first=%v second=%v, want two rows each", first, second)
	}
	explained := interp.GetGlobal("explained").Table()
	if explained == nil {
		t.Fatal("explained query kernel cache table is nil")
	}
	if got := explained.RawGetString("kernel_cached"); !got.IsBool() || !got.Bool() {
		t.Fatalf("explained kernel_cached = %v, want true", got)
	}
	stats := qTestCacheStatsRows(t, qCacheStatsTable())
	got := stats["q_query_kernel"]
	if got["entries"] != 1 || got["hits"] != 1 || got["misses"] != 1 || got["evictions"] != 0 {
		t.Fatalf("q_query_kernel stats = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", got)
	}
	queryKernelRow := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_query_kernel")
	keyRows := qTestQueryKernelKeyRows(t, queryKernelRow.RawGetString("keys").Table())
	if len(keyRows) != 1 {
		t.Fatalf("q_query_kernel supported keys = %+v, want one row", keyRows)
	}
	if got := keyRows[0]; got.Namespace != "q.query" || got.Kind != "query_kernel" || got.PlanFingerprint != "" || !got.Supported || got.ReasonFamily != qFallbackFamilySupported || got.ReasonCode != qKernelReasonSupported || got.Hits != 1 || got.Misses != 1 || got.Evictions != 0 {
		t.Fatalf("q_query_kernel supported key row = %+v, want supported hit=1 miss=1", got)
	}
	keyJSONRows := qTestQueryKernelSupportKeyJSONRows(t)
	if len(keyJSONRows) != 1 {
		t.Fatalf("QQueryKernelSupportKeyStatJSONRows = %+v, want one supported row", keyJSONRows)
	}
	if got := keyJSONRows[0]; got.Key != keyRows[0].Key || got.Namespace != "q.query" || got.Kind != "query_kernel" || got.PlanFingerprint != "" || !got.Supported || got.ReasonFamily != qFallbackFamilySupported || got.ReasonCode != qKernelReasonSupported || got.SchemaHash != keyRows[0].SchemaHash || got.Shape != keyRows[0].Shape || got.Hits != 1 || got.Misses != 1 || got.Evictions != 0 {
		t.Fatalf("QQueryKernelSupportKeyStatJSONRows[0] = %+v, want stable supported q.query key row", got)
	}
	shapeRows := qTestQueryKernelShapeRows(t, queryKernelRow.RawGetString("shapes").Table())
	if len(shapeRows) != 1 {
		t.Fatalf("q_query_kernel supported shapes = %+v, want one row", shapeRows)
	}
	if got := qTestQueryKernelShapeCount(shapeRows, true, qFallbackFamilySupported, qKernelReasonSupported, shapeRows[0].SchemaHash); got != 1 {
		t.Fatalf("q_query_kernel supported shape count = %d, want 1", got)
	}
	if shapeRows[0].Shape == "" || !strings.Contains(shapeRows[0].Shape, "select=") {
		t.Fatalf("q_query_kernel supported shape = %q, want select shape", shapeRows[0].Shape)
	}
	if !strings.HasPrefix(shapeRows[0].SchemaHash, "q.query.kernel:") {
		t.Fatalf("q_query_kernel supported schema_hash = %q, want q.query kernel hash", shapeRows[0].SchemaHash)
	}
	fallback := qTestFallbackStatsRows(t, qFallbackStatsTable())
	if got := fallback[qQueryKernelSupported]; got != 2 {
		t.Fatalf("query kernel hit count = %d, want 2", got)
	}
}

func TestQQueryKernelSupportCacheKeyIsSchemaStable(t *testing.T) {
	trades, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{100, 101}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	spec := NewTable()
	key, ok := qQueryKernelSupportCacheKey(trades, spec, []qSelect{{
		Name: "a|b=c",
		Expr: StringValue("price"),
	}})
	if !ok {
		t.Fatal("qQueryKernelSupportCacheKey returned false")
	}
	parsed, ok := data.ParseSchemaStableCacheKey(key)
	if !ok {
		t.Fatalf("ParseSchemaStableCacheKey(%q) returned false", key)
	}
	if parsed.Namespace != "q.query" || parsed.Kind != "query_kernel" || parsed.SchemaHash != qQueryNativeSoASchemaHash(trades) {
		t.Fatalf("parsed q query key = %+v, want q.query/query_kernel schema hash", parsed)
	}
	wantExtra := []string{"select", "a|b=c", `s:"price"`, "limit", "-1"}
	if len(parsed.Extra) != len(wantExtra) {
		t.Fatalf("parsed extra = %#v, want %#v", parsed.Extra, wantExtra)
	}
	for i := range wantExtra {
		if parsed.Extra[i] != wantExtra[i] {
			t.Fatalf("parsed extra[%d] = %q, want %q; all=%#v", i, parsed.Extra[i], wantExtra[i], parsed.Extra)
		}
	}
	if got := qQueryKernelSchemaHashFromCacheKey(key); got != parsed.SchemaHash {
		t.Fatalf("qQueryKernelSchemaHashFromCacheKey = %q, want %q", got, parsed.SchemaHash)
	}
	if got := qQueryKernelShapeFromCacheKey(key); got != "select=column|order=0|limit=none" {
		t.Fatalf("qQueryKernelShapeFromCacheKey = %q, want column select shape", got)
	}
	legacy := "source=legacy-schema|select:price=s:\"price\"|order:price:asc|limit=2"
	if got := qQueryKernelSchemaHashFromCacheKey(legacy); got != "legacy-schema" {
		t.Fatalf("legacy schema hash = %q, want legacy-schema", got)
	}
	if got := qQueryKernelShapeFromCacheKey(legacy); got != "select=column|order=1|limit=2" {
		t.Fatalf("legacy shape = %q, want select=column|order=1|limit=2", got)
	}
}

func TestQQueryKernelShapeStatsSplitBySchemaHash(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	trades, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{100, 101}),
		"size":  NewDenseArrayF64([]float64{10, 20}),
	})
	if err != nil {
		t.Fatalf("NewSoA trades: %v", err)
	}
	quotes, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{80, 90}),
		"qty":   NewDenseArrayF64([]float64{1, 2}),
	})
	if err != nil {
		t.Fatalf("NewSoA quotes: %v", err)
	}
	spec := NewTable()
	selects := NewTable()
	selects.RawSetString("price", StringValue("price"))
	spec.RawSetString("select", TableValue(selects))

	for label, soa := range map[string]*SoA{"trades": trades, "quotes": quotes} {
		if _, err := qRunQuery(soa, spec); err != nil {
			t.Fatalf("qRunQuery %s: %v", label, err)
		}
	}

	queryKernelRow := qTestCacheStatsRowTable(t, qCacheStatsTable(), "q_query_kernel")
	keyRows := qTestQueryKernelKeyRows(t, queryKernelRow.RawGetString("keys").Table())
	if len(keyRows) != 2 {
		t.Fatalf("q_query_kernel keys = %+v, want one row per schema", keyRows)
	}
	shapeRows := qTestQueryKernelShapeRows(t, queryKernelRow.RawGetString("shapes").Table())
	if len(shapeRows) != 2 {
		t.Fatalf("q_query_kernel shapes = %+v, want one row per schema", shapeRows)
	}
	wantSchemas := map[string]bool{
		qQueryNativeSoASchemaHash(trades): false,
		qQueryNativeSoASchemaHash(quotes): false,
	}
	for _, row := range shapeRows {
		if !row.Supported || row.ReasonFamily != qFallbackFamilySupported || row.ReasonCode != qKernelReasonSupported || row.Count != 1 {
			t.Fatalf("q_query_kernel shape row = %+v, want supported count=1", row)
		}
		if row.Shape != "select=column|order=0|limit=none" {
			t.Fatalf("q_query_kernel shape = %q, want column select shape", row.Shape)
		}
		if _, ok := wantSchemas[row.SchemaHash]; !ok {
			t.Fatalf("q_query_kernel shape schema_hash = %q, want one of %#v", row.SchemaHash, wantSchemas)
		}
		wantSchemas[row.SchemaHash] = true
	}
	for _, row := range keyRows {
		if row.Namespace != "q.query" || row.Kind != "query_kernel" || row.PlanFingerprint != "" || !row.Supported || row.ReasonFamily != qFallbackFamilySupported || row.ReasonCode != qKernelReasonSupported || row.Hits != 0 || row.Misses != 1 || row.Evictions != 0 {
			t.Fatalf("q_query_kernel key row = %+v, want supported miss=1", row)
		}
		if _, ok := wantSchemas[row.SchemaHash]; !ok {
			t.Fatalf("q_query_kernel key schema_hash = %q, want one of %#v", row.SchemaHash, wantSchemas)
		}
	}
	for schemaHash, seen := range wantSchemas {
		if !seen {
			t.Fatalf("q_query_kernel shape schema_hash %q was not reported; rows=%+v", schemaHash, shapeRows)
		}
	}
}

func TestQFallbackStatsAggregateTopReasons(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 101, 80})},
	)
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	plan := data.QueryPlan{
		Select: []data.SelectItem{{Name: "marker", Expr: qFallbackStatsTestExpr{}}},
		LimitN: -1,
	}
	for i := 0; i < 2; i++ {
		if _, err := qRunSQLPlan("fallback-stats-top-test", plan, frame); err != nil {
			t.Fatalf("fallback qRunSQLPlan %d: %v", i, err)
		}
	}

	rows := qTestFallbackStatsDetailRows(t, qFallbackStatsTable())
	codeRows := qTestFallbackStatsRows(t, qFallbackStatsTable())
	if got := codeRows[qFallbackKernelUnsupported]; got != 2 {
		t.Fatalf("kernel unsupported fallback count = %d, want 2", got)
	}
	if got := qTestFallbackFamilyCount(rows, qFallbackFamilySelect); got != 2 {
		t.Fatalf("kernel unsupported select family count = %d, want 2", got)
	}
	if got := qTestFallbackDetailCount(rows, "reason_code", qFallbackKernelUnsupported, stdq.KernelFallbackSelectExpression, ""); got != 2 {
		t.Fatalf("kernel unsupported reason_code top count = %d, want 2", got)
	}
	if got := qTestFallbackDetailFamily(rows, "reason_code", qFallbackKernelUnsupported, stdq.KernelFallbackSelectExpression, ""); got != qFallbackFamilySelect {
		t.Fatalf("kernel unsupported reason_code family = %q, want %s", got, qFallbackFamilySelect)
	}
	reason := fmt.Sprintf("select expression %q is not supported by data query kernel: unsupported expression %T", "marker", qFallbackStatsTestExpr{})
	if got := qTestFallbackDetailCount(rows, "reason", qFallbackKernelUnsupported, "", reason); got != 2 {
		t.Fatalf("kernel unsupported reason top count = %d, want 2", got)
	}
	if got := qTestFallbackDetailFamily(rows, "reason", qFallbackKernelUnsupported, "", reason); got != qFallbackFamilySelect {
		t.Fatalf("kernel unsupported reason family = %q, want %s", got, qFallbackFamilySelect)
	}
	statsFn := BuildQ().RawGetString("fallback_stats").GoFunction()
	if statsFn == nil {
		t.Fatal("q.fallback_stats function missing")
	}
	values, err := statsFn.Fn(nil)
	if err != nil {
		t.Fatalf("q.fallback_stats: %v", err)
	}
	if len(values) != 1 || values[0].Table() == nil {
		t.Fatalf("q.fallback_stats values = %#v, want one table", values)
	}
	if got := qTestFallbackDetailCount(qTestFallbackStatsDetailRows(t, values[0].Table()), "reason", qFallbackKernelUnsupported, "", reason); got != 2 {
		t.Fatalf("q.fallback_stats reason top count = %d, want 2", got)
	}
	if got := qTestFallbackFamilyCount(qTestFallbackStatsDetailRows(t, values[0].Table()), qFallbackFamilySelect); got != 2 {
		t.Fatalf("q.fallback_stats select family count = %d, want 2", got)
	}

	qClearCaches()
	if rows := qTestFallbackStatsDetailRows(t, qFallbackStatsTable()); len(rows) != 0 {
		t.Fatalf("fallback detail rows after clear = %#v, want none", rows)
	}
}

func TestQSQLFastArg2ExecutesTwoArgumentForms(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80})},
	)
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatalf("qDataFrameValue: %v", err)
	}

	q := BuildQ()
	sql := q.RawGetString("sql").GoFunction()
	if sql == nil || sql.FastArg2 == nil {
		t.Fatalf("q.sql FastArg2 missing: %#v", sql)
	}
	if sql.NativeKind != NativeKindStdQSQL || sql.NativeData != StdQSQLIdentityPtr() {
		t.Fatalf("q.sql native identity kind=%d data=%p, want q sql identity", sql.NativeKind, sql.NativeData)
	}
	selected, err := sql.FastArg2(frameValue, StringValue("select sym,price from trades where price>=90"))
	if err != nil {
		t.Fatalf("q.sql FastArg2 frame/source: %v", err)
	}
	rows := selected.Table()
	if rows == nil || rows.Length() != 1 {
		t.Fatalf("q.sql FastArg2 frame/source rows = %v, want one row", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("q.sql FastArg2 sym = %v, want AAPL", got)
	}

	env := NewTable()
	env.RawSetString("trades", frameValue)
	fromEnv, err := sql.FastArg2(StringValue("select sym from trades where price<90"), TableValue(env))
	if err != nil {
		t.Fatalf("q.sql FastArg2 source/env: %v", err)
	}
	envRows := fromEnv.Table()
	if envRows == nil || envRows.Length() != 1 {
		t.Fatalf("q.sql FastArg2 source/env rows = %v, want one row", envRows)
	}
	if got := envRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("q.sql FastArg2 source/env sym = %v, want MSFT", got)
	}

	selectFn := q.RawGetString("select").GoFunction()
	if selectFn == nil || selectFn.FastArg2 == nil {
		t.Fatalf("q.select FastArg2 missing: %#v", selectFn)
	}
	if selectFn.NativeKind != NativeKindStdQSelect || selectFn.NativeData != StdQSelectIdentityPtr() {
		t.Fatalf("q.select native identity kind=%d data=%p, want q select identity", selectFn.NativeKind, selectFn.NativeData)
	}
	if _, err := selectFn.FastArg2(frameValue, StringValue("select sym from trades")); err != nil {
		t.Fatalf("q.select FastArg2: %v", err)
	}
}

func TestQSQLFastArg2CacheStatsParity(t *testing.T) {
	qClearCaches()
	defer qClearCaches()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20})},
	)
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatalf("qDataFrameValue: %v", err)
	}

	q := BuildQ()
	sql := q.RawGetString("sql").GoFunction()
	if sql == nil || sql.FastArg2 == nil {
		t.Fatalf("q.sql FastArg2 missing: %#v", sql)
	}

	source := StringValue("select sym,px:price from trades where price>=90")
	if _, err := sql.FastArg2(frameValue, source); err != nil {
		t.Fatalf("first q.sql FastArg2: %v", err)
	}
	if _, err := sql.FastArg2(frameValue, source); err != nil {
		t.Fatalf("second q.sql FastArg2: %v", err)
	}

	stats := qTestCacheStatsRows(t, qCacheStatsTable())
	for _, name := range []string{"qsql_template", "qsql_aligned", "qsql_kernel"} {
		got := stats[name]
		if got["entries"] != 1 || got["hits"] != 1 || got["misses"] != 1 || got["evictions"] != 0 {
			t.Fatalf("%s stats after q.sql FastArg2 = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", name, got)
		}
	}

	qClearCaches()
	selectFn := q.RawGetString("select").GoFunction()
	if selectFn == nil || selectFn.FastArg2 == nil {
		t.Fatalf("q.select FastArg2 missing: %#v", selectFn)
	}
	if _, err := selectFn.FastArg2(frameValue, source); err != nil {
		t.Fatalf("first q.select FastArg2: %v", err)
	}
	if _, err := selectFn.FastArg2(frameValue, source); err != nil {
		t.Fatalf("second q.select FastArg2: %v", err)
	}

	stats = qTestCacheStatsRows(t, qCacheStatsTable())
	for _, name := range []string{"qsql_template", "qsql_aligned", "qsql_kernel"} {
		got := stats[name]
		if got["entries"] != 1 || got["hits"] != 1 || got["misses"] != 1 || got["evictions"] != 0 {
			t.Fatalf("%s stats after q.select FastArg2 = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", name, got)
		}
	}
}

func TestQSQLArgs2MatchesTwoArgumentSemantics(t *testing.T) {
	frame := TableValue(NewTable())
	source := StringValue("select sym from trades")
	env := TableValue(NewTable())
	extra := TableValue(NewTable())

	slowFrameSource, err := qSQLArgs("q.sql", []Value{frame, source})
	if err != nil {
		t.Fatalf("qSQLArgs frame/source: %v", err)
	}
	fastFrameSource, ok, err := qSQLArgs2("q.sql", frame, source)
	if err != nil || !ok {
		t.Fatalf("qSQLArgs2 frame/source = ok %v err %v, want ok", ok, err)
	}
	if slowFrameSource.frameValue != fastFrameSource.frameValue ||
		slowFrameSource.source != fastFrameSource.source ||
		slowFrameSource.resolveSource != fastFrameSource.resolveSource ||
		slowFrameSource.envValue != fastFrameSource.envValue {
		t.Fatalf("qSQLArgs2 frame/source = %#v, want %#v", fastFrameSource, slowFrameSource)
	}

	slowSourceEnv, err := qSQLArgs("q.sql", []Value{source, env})
	if err != nil {
		t.Fatalf("qSQLArgs source/env: %v", err)
	}
	fastSourceEnv, ok, err := qSQLArgs2("q.sql", source, env)
	if err != nil || !ok {
		t.Fatalf("qSQLArgs2 source/env = ok %v err %v, want ok", ok, err)
	}
	if slowSourceEnv.frameValue != fastSourceEnv.frameValue ||
		slowSourceEnv.source != fastSourceEnv.source ||
		slowSourceEnv.resolveSource != fastSourceEnv.resolveSource ||
		slowSourceEnv.envValue != fastSourceEnv.envValue {
		t.Fatalf("qSQLArgs2 source/env = %#v, want %#v", fastSourceEnv, slowSourceEnv)
	}

	withEnv, err := qSQLArgs("q.sql", []Value{frame, source, extra})
	if err != nil {
		t.Fatalf("qSQLArgs frame/source/env: %v", err)
	}
	if withEnv.envValue != extra {
		t.Fatalf("qSQLArgs frame/source/env env = %s, want extra env", withEnv.envValue.String())
	}

	ignoredExtra, err := qSQLArgs("q.sql", []Value{source, env, extra})
	if err != nil {
		t.Fatalf("qSQLArgs source/env/extra: %v", err)
	}
	if ignoredExtra.envValue != env {
		t.Fatalf("qSQLArgs source/env/extra env = %s, want second argument env", ignoredExtra.envValue.String())
	}
}

func TestQEvalSkipsCacheForStatefulSources(t *testing.T) {
	qClearCaches()

	if _, err := qEvalSymbolic(`\p`); err != nil {
		t.Fatalf("first q.eval system command: %v", err)
	}
	if _, err := qEvalSymbolic(`\p`); err != nil {
		t.Fatalf("second q.eval system command: %v", err)
	}
	qEvalCacheMu.Lock()
	entries := len(qEvalCache)
	stats := qEvalStats
	qEvalCacheMu.Unlock()
	if entries != 0 || stats.Hits != 0 || stats.Misses != 0 {
		t.Fatalf("stateful q.eval cache entries=%d stats=%+v, want no eval-cache activity", entries, stats)
	}

	if _, err := qEvalSymbolic("1+2"); err != nil {
		t.Fatalf("first cacheable q.eval: %v", err)
	}
	if _, err := qEvalSymbolic("1+2"); err != nil {
		t.Fatalf("second cacheable q.eval: %v", err)
	}
	qEvalCacheMu.Lock()
	entries = len(qEvalCache)
	stats = qEvalStats
	qEvalCacheMu.Unlock()
	if entries != 1 || stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("cacheable q.eval cache entries=%d stats=%+v, want one miss and one hit", entries, stats)
	}
}

func TestQSQLQueryKernelCacheExecutesSchemaStableHotPath(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA", "TSLA"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210, 190})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20, 30, 40})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatal(err)
	}
	query := "select sym,px:price,notional:price*size from trades where price>=90 order by notional desc take 2"

	first, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query})
	if err != nil {
		t.Fatalf("first q.sql: %v", err)
	}
	second, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query})
	if err != nil {
		t.Fatalf("second q.sql: %v", err)
	}
	firstRows := first.Table()
	secondRows := second.Table()
	if firstRows == nil || secondRows == nil || firstRows.Length() != 2 || secondRows.Length() != 2 {
		t.Fatalf("kernel query rows = first %v second %v, want two-row tables", firstRows, secondRows)
	}
	if got := secondRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "TSLA" {
		t.Fatalf("second first sym = %v, want TSLA", got)
	}
	if got := secondRows.RawGetInt(1).Table().RawGetString("notional"); !got.IsFloat() || got.Float() != 7600 {
		t.Fatalf("second first notional = %v, want 7600", got)
	}
	if got := secondRows.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("second second sym = %v, want NVDA", got)
	}

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.KernelMisses != 1 || stats.KernelHits != 1 {
		t.Fatalf("kernel cache stats = %+v, want one miss and one hit", stats)
	}
}

func TestQExplainReportsQueryKernelVisibility(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA", "TSLA"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210, 190})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20, 30, 40})},
	)
	if err != nil {
		t.Fatalf("frame: %v", err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatalf("frame value: %v", err)
	}
	query := "select notional:price*size from trades where price>=90 order by notional desc take 2"

	before, err := qExplainSQL(qSQLArgsResult{frameValue: frameValue, source: query})
	if err != nil {
		t.Fatalf("explain before q.sql: %v", err)
	}
	beforeTable := before.Table()
	if got := beforeTable.RawGetString("kernel_supported"); !got.IsBool() || !got.Bool() {
		t.Fatalf("kernel_supported before = %v, want true", got)
	}
	if got := beforeTable.RawGetString("kernel_cached"); !got.IsBool() || got.Bool() {
		t.Fatalf("kernel_cached before = %v, want false", got)
	}
	if got := beforeTable.RawGetString("kernel_reason"); !got.IsString() || !strings.Contains(got.Str(), "post-project ordered projection path") {
		t.Fatalf("kernel_reason before = %v, want specific ordered projection kernel reason", got)
	}
	kernelShape := beforeTable.RawGetString("kernel_shape")
	if !kernelShape.IsString() || kernelShape.Str() != "post_project_ordered_projection|where=typed_column_literal|projection=typed_binary|order=post_project:1|limit=bounded" {
		t.Fatalf("kernel_shape before = %v, want stable filtered ordered projection shape", kernelShape)
	}
	if got := beforeTable.RawGetString("source_bridge"); !got.IsString() || got.Str() != "frame_native" {
		t.Fatalf("source_bridge before = %v, want frame_native", got)
	}
	if got := beforeTable.RawGetString("source_native"); !got.IsBool() || !got.Bool() {
		t.Fatalf("source_native before = %v, want true", got)
	}
	if got := beforeTable.RawGetString("source_keyed"); !got.IsBool() || got.Bool() {
		t.Fatalf("source_keyed before = %v, want false", got)
	}
	if got := beforeTable.RawGetString("source_rows"); !got.IsInt() || got.Int() != 4 {
		t.Fatalf("source_rows before = %v, want 4", got)
	}
	if got := beforeTable.RawGetString("source_schema_hash"); !got.IsString() || got.Str() != frame.SchemaFingerprint() {
		t.Fatalf("source_schema_hash before = %v, want %s", got, frame.SchemaFingerprint())
	}
	if got := beforeTable.RawGetString("kernel_cache_stats_domain"); !got.IsString() || got.Str() != qStatsDomainSemanticCache {
		t.Fatalf("kernel_cache_stats_domain before = %v, want %s", got, qStatsDomainSemanticCache)
	}
	if got := beforeTable.RawGetString("kernel_execution_stats_domain"); !got.IsString() || got.Str() != qStatsDomainJITExecution {
		t.Fatalf("kernel_execution_stats_domain before = %v, want %s", got, qStatsDomainJITExecution)
	}
	if got := beforeTable.RawGetString("kernel_execution_stats_source"); !got.IsString() || got.Str() != qStatsSourceMethodJIT {
		t.Fatalf("kernel_execution_stats_source before = %v, want %s", got, qStatsSourceMethodJIT)
	}
	if got := beforeTable.RawGetString("kernel_execution_stats_cache_backed"); !got.IsBool() || got.Bool() {
		t.Fatalf("kernel_execution_stats_cache_backed before = %v, want false", got)
	}
	cacheKey := beforeTable.RawGetString("kernel_cache_key")
	if !cacheKey.IsString() || cacheKey.Str() == "" {
		t.Fatalf("kernel_cache_key before = %v, want stable non-empty key", cacheKey)
	}
	if got := beforeTable.RawGetString("kernel_cache_namespace"); !got.IsString() || got.Str() != query {
		t.Fatalf("kernel_cache_namespace before = %v, want %q", got, query)
	}
	if got := beforeTable.RawGetString("kernel_cache_kind"); !got.IsString() || got.Str() != "kernel" {
		t.Fatalf("kernel_cache_kind before = %v, want kernel", got)
	}
	if got := beforeTable.RawGetString("kernel_cache_schema_hash"); !got.IsString() || got.Str() != frame.SchemaFingerprint() {
		t.Fatalf("kernel_cache_schema_hash before = %v, want %s", got, frame.SchemaFingerprint())
	}
	if got := beforeTable.RawGetString("kernel_cache_schema_match"); !got.IsBool() || !got.Bool() {
		t.Fatalf("kernel_cache_schema_match before = %v, want true", got)
	}
	planFingerprint := beforeTable.RawGetString("kernel_plan_fingerprint")
	if !planFingerprint.IsString() || planFingerprint.Str() == "" {
		t.Fatalf("kernel_plan_fingerprint before = %v, want stable non-empty fingerprint", planFingerprint)
	}
	schema := beforeTable.RawGetString("source_schema").Table()
	if schema == nil || schema.Length() != 3 {
		t.Fatalf("source_schema len = %v, want 3", schema)
	}
	if got := schema.RawGetInt(1).Table().RawGetString("name"); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("source_schema[1].name = %v, want sym", got)
	}
	sameSchema, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"IBM"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{99})},
		data.Column{Name: "size", Data: data.NewI64([]int64{1})},
	)
	if err != nil {
		t.Fatalf("same schema frame: %v", err)
	}
	sameSchemaValue, err := qDataFrameValue(sameSchema)
	if err != nil {
		t.Fatalf("same schema frame value: %v", err)
	}
	sameSchemaExplain, err := qExplainSQL(qSQLArgsResult{frameValue: sameSchemaValue, source: query})
	if err != nil {
		t.Fatalf("same schema explain: %v", err)
	}
	if got := sameSchemaExplain.Table().RawGetString("kernel_cache_key"); !got.IsString() || got.Str() != cacheKey.Str() {
		t.Fatalf("same schema kernel_cache_key = %v, want %s", got, cacheKey.Str())
	}
	if got := sameSchemaExplain.Table().RawGetString("kernel_cache_schema_match"); !got.IsBool() || !got.Bool() {
		t.Fatalf("same schema kernel_cache_schema_match = %v, want true", got)
	}
	differentSchema, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"IBM"})},
		data.Column{Name: "size", Data: data.NewI64([]int64{1})},
		data.Column{Name: "price", Data: data.NewF64([]float64{99})},
	)
	if err != nil {
		t.Fatalf("different schema frame: %v", err)
	}
	differentSchemaValue, err := qDataFrameValue(differentSchema)
	if err != nil {
		t.Fatalf("different schema frame value: %v", err)
	}
	differentSchemaExplain, err := qExplainSQL(qSQLArgsResult{frameValue: differentSchemaValue, source: query})
	if err != nil {
		t.Fatalf("different schema explain: %v", err)
	}
	if got := differentSchemaExplain.Table().RawGetString("kernel_cache_key"); !got.IsString() || got.Str() == cacheKey.Str() {
		t.Fatalf("different schema kernel_cache_key = %v, want it to differ from %s", got, cacheKey.Str())
	}
	if got := differentSchemaExplain.Table().RawGetString("kernel_cache_schema_match"); !got.IsBool() || !got.Bool() {
		t.Fatalf("different schema kernel_cache_schema_match = %v, want true for its own cache key", got)
	}

	if _, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query}); err != nil {
		t.Fatalf("first q.sql: %v", err)
	}
	if _, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query}); err != nil {
		t.Fatalf("second q.sql: %v", err)
	}
	after, err := qExplainSQL(qSQLArgsResult{frameValue: frameValue, source: query})
	if err != nil {
		t.Fatalf("explain after q.sql: %v", err)
	}
	afterTable := after.Table()
	if got := afterTable.RawGetString("kernel_supported"); !got.IsBool() || !got.Bool() {
		t.Fatalf("kernel_supported after = %v, want true", got)
	}
	if got := afterTable.RawGetString("kernel_cached"); !got.IsBool() || !got.Bool() {
		t.Fatalf("kernel_cached after = %v, want true", got)
	}
	if got := afterTable.RawGetString("kernel_decision_cached"); !got.IsBool() || !got.Bool() {
		t.Fatalf("kernel_decision_cached after = %v, want true", got)
	}
	if got := afterTable.RawGetString("kernel_reason"); !got.IsString() || got.Str() != beforeTable.RawGetString("kernel_reason").Str() {
		t.Fatalf("kernel_reason after = %v, want cached reason %q", got, beforeTable.RawGetString("kernel_reason").Str())
	}
	if got := afterTable.RawGetString("kernel_shape"); !got.IsString() || got.Str() != kernelShape.Str() {
		t.Fatalf("kernel_shape after = %v, want cached shape %q", got, kernelShape.Str())
	}
	if got := afterTable.RawGetString("kernel_cache_key"); !got.IsString() || got.Str() != cacheKey.Str() {
		t.Fatalf("kernel_cache_key after = %v, want %s", got, cacheKey.Str())
	}

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.KernelMisses != 1 || stats.KernelHits != 1 {
		t.Fatalf("kernel cache stats = %+v, want q.explain to avoid changing kernel hit/miss counts", stats)
	}
	keyStats := qTestKernelCacheKeyStats(t, stats.KernelKeys, cacheKey.Str())
	if keyStats.Misses != 1 || keyStats.Hits != 1 || keyStats.Evictions != 0 {
		t.Fatalf("kernel cache key stats = %+v, want one miss, one hit, zero evictions", keyStats)
	}
	keyJSONRows := QSQLKernelCacheKeyStatJSONRows(stats.KernelKeys)
	if len(keyJSONRows) != 1 {
		t.Fatalf("QSQLKernelCacheKeyStatJSONRows = %+v, want one row", keyJSONRows)
	}
	if got := keyJSONRows[0]; got.Key != cacheKey.Str() || got.Namespace != query || got.Kind != "kernel" || got.SchemaHash != frame.SchemaFingerprint() || got.PlanFingerprint != planFingerprint.Str() || got.Shape != kernelShape.Str() || got.Hits != 1 || got.Misses != 1 || got.Evictions != 0 {
		t.Fatalf("QSQLKernelCacheKeyStatJSONRows[0] = %+v, want stable qsql kernel key row", got)
	}
	keyJSON, err := json.Marshal(keyJSONRows)
	if err != nil {
		t.Fatalf("marshal QSQLKernelCacheKeyStatJSONRows: %v", err)
	}
	if !strings.Contains(string(keyJSON), `"schema_hash"`) || !strings.Contains(string(keyJSON), `"plan_fingerprint"`) || strings.Contains(string(keyJSON), "SchemaHash") || strings.Contains(string(keyJSON), "PlanFingerprint") {
		t.Fatalf("QSQLKernelCacheKeyStatJSONRows JSON = %s, want snake_case stable fields", keyJSON)
	}
	cacheRows := qCacheStatsTable()
	kernelRow := qTestCacheStatsRowTable(t, cacheRows, "qsql_kernel")
	keyRows := kernelRow.RawGetString("keys").Table()
	if keyRows == nil || keyRows.Length() != 1 {
		t.Fatalf("qsql_kernel keys = %v, want one per-key stats row", keyRows)
	}
	keyRow := keyRows.RawGetInt(1).Table()
	if keyRow == nil {
		t.Fatal("qsql_kernel keys[1] is nil")
	}
	if got := keyRow.RawGetString("key"); !got.IsString() || got.Str() != cacheKey.Str() {
		t.Fatalf("qsql_kernel keys[1].key = %v, want %s", got, cacheKey.Str())
	}
	for field, want := range map[string]string{
		"namespace":        query,
		"kind":             "kernel",
		"schema_hash":      frame.SchemaFingerprint(),
		"plan_fingerprint": planFingerprint.Str(),
		"shape":            kernelShape.Str(),
	} {
		if got := keyRow.RawGetString(field); !got.IsString() || got.Str() != want {
			t.Fatalf("qsql_kernel keys[1].%s = %v, want %q", field, got, want)
		}
	}
	for field, want := range map[string]int64{"hits": 1, "misses": 1, "evictions": 0} {
		if got := keyRow.RawGetString(field); !got.IsInt() || got.Int() != want {
			t.Fatalf("qsql_kernel keys[1].%s = %v, want %d", field, got, want)
		}
	}
	shapeRows := qTestKernelShapeRows(t, kernelRow.RawGetString("shapes").Table())
	if len(shapeRows) != 1 {
		t.Fatalf("qsql_kernel shapes = %+v, want one shape row", shapeRows)
	}
	if got := shapeRows[0]; got.Shape != kernelShape.Str() || got.SchemaHash != frame.SchemaFingerprint() || got.Count != 1 || got.Hits != 1 || got.Misses != 1 || got.Evictions != 0 {
		t.Fatalf("qsql_kernel shape aggregate = %+v, want shape %q schema %q count=1 hits=1 misses=1 evictions=0", got, kernelShape.Str(), frame.SchemaFingerprint())
	}
}

func TestQExplainReportsVectorTransformKernelVisibility(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210})},
	)
	if err != nil {
		t.Fatalf("frame: %v", err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatalf("frame value: %v", err)
	}
	explained, err := qExplainSQL(qSQLArgsResult{frameValue: frameValue, source: "select p:prev price from trades order by price asc"})
	if err != nil {
		t.Fatalf("explain vector q.sql: %v", err)
	}
	table := explained.Table()
	if got := table.RawGetString("kernel_supported"); !got.IsBool() || !got.Bool() {
		t.Fatalf("kernel_supported = %v, want true", got)
	}
	reason := table.RawGetString("kernel_reason")
	if !reason.IsString() || !strings.Contains(reason.Str(), "data query kernel supported") {
		t.Fatalf("kernel_reason = %v, want supported reason", reason)
	}
	selects := table.RawGetString("select").Table()
	if selects == nil || selects.Length() != 1 {
		t.Fatalf("select explain = %v, want one projection", selects)
	}
	if got := selects.RawGetInt(1).Table().RawGetString("expr"); !got.IsString() || got.Str() != "prev price" {
		t.Fatalf("select expr = %v, want prev price", got)
	}
}

func TestQExplainReportsPostJoinKernelCacheVisibility(t *testing.T) {
	qSQLResetPlanCachesForTest()

	trades, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		data.Column{Name: "ts", Data: data.NewI64([]int64{10, 20, 15})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 101, 80})},
	)
	if err != nil {
		t.Fatalf("trades frame: %v", err)
	}
	quotes, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		data.Column{Name: "ts", Data: data.NewI64([]int64{5, 18, 10})},
		data.Column{Name: "bid", Data: data.NewF64([]float64{99, 100.5, 79.5})},
	)
	if err != nil {
		t.Fatalf("quotes frame: %v", err)
	}
	tradesValue, err := qDataFrameValue(trades)
	if err != nil {
		t.Fatalf("trades value: %v", err)
	}
	quotesValue, err := qDataFrameValue(quotes)
	if err != nil {
		t.Fatalf("quotes value: %v", err)
	}
	env := NewTable()
	env.RawSetString("trades", tradesValue)
	env.RawSetString("quotes", quotesValue)
	envValue := TableValue(env)

	tests := []struct {
		name  string
		query string
		kind  string
	}{
		{
			name:  "asof",
			query: "select sym,ts,bid from trades asof join quotes on sym,ts order by ts asc",
			kind:  "asof",
		},
		{
			name:  "window",
			query: "select sym,ts,bid from trades wj1[-10 0] quotes on sym,ts order by ts asc",
			kind:  "window1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, err := qExplainSQL(qSQLArgsResult{frameValue: envValue, source: tt.query, resolveSource: true, envValue: envValue})
			if err != nil {
				t.Fatalf("explain before: %v", err)
			}
			beforeTable := before.Table()
			if got := beforeTable.RawGetString("kernel_supported"); !got.IsBool() || !got.Bool() {
				t.Fatalf("kernel_supported before = %v, want true", got)
			}
			if got := beforeTable.RawGetString("kernel_cached"); !got.IsBool() || got.Bool() {
				t.Fatalf("kernel_cached before = %v, want false", got)
			}
			join := beforeTable.RawGetString("join").Table()
			if join == nil {
				t.Fatalf("join explain is nil")
			}
			if got := join.RawGetString("kind"); !got.IsString() || got.Str() != tt.kind {
				t.Fatalf("join.kind = %v, want %s", got, tt.kind)
			}
			if got := join.RawGetString("time_key"); !got.IsString() || got.Str() != "ts" {
				t.Fatalf("join.time_key = %v, want ts", got)
			}
			partitions := join.RawGetString("partition_keys").Table()
			if partitions == nil || partitions.Length() != 1 || !partitions.RawGetInt(1).IsString() || partitions.RawGetInt(1).Str() != "sym" {
				t.Fatalf("join.partition_keys = %v, want [sym]", partitions)
			}
			if _, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: envValue, source: tt.query, resolveSource: true, envValue: envValue}); err != nil {
				t.Fatalf("first q.sql: %v", err)
			}
			if _, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: envValue, source: tt.query, resolveSource: true, envValue: envValue}); err != nil {
				t.Fatalf("second q.sql: %v", err)
			}
			after, err := qExplainSQL(qSQLArgsResult{frameValue: envValue, source: tt.query, resolveSource: true, envValue: envValue})
			if err != nil {
				t.Fatalf("explain after: %v", err)
			}
			if got := after.Table().RawGetString("kernel_cached"); !got.IsBool() || !got.Bool() {
				t.Fatalf("kernel_cached after = %v, want true", got)
			}
		})
	}

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.KernelMisses != 2 || stats.KernelHits != 2 {
		t.Fatalf("post-join kernel cache stats = %+v, want two misses and two hits", stats)
	}
}

func TestQExplainReportsGlobalTemporalJoinShape(t *testing.T) {
	qSQLResetPlanCachesForTest()

	trades, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
		data.Column{Name: "ts", Data: data.NewI64([]int64{10, 20, 30})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210})},
	)
	if err != nil {
		t.Fatalf("trades frame: %v", err)
	}
	quotes, err := data.NewFrame(
		data.Column{Name: "ts", Data: data.NewI64([]int64{5, 18, 25})},
		data.Column{Name: "bid", Data: data.NewF64([]float64{99, 100.5, 209.5})},
	)
	if err != nil {
		t.Fatalf("quotes frame: %v", err)
	}
	tradesValue, err := qDataFrameValue(trades)
	if err != nil {
		t.Fatalf("trades value: %v", err)
	}
	quotesValue, err := qDataFrameValue(quotes)
	if err != nil {
		t.Fatalf("quotes value: %v", err)
	}
	env := NewTable()
	env.RawSetString("trades", tradesValue)
	env.RawSetString("quotes", quotesValue)
	envValue := TableValue(env)

	tests := []struct {
		name  string
		query string
		kind  string
	}{
		{name: "global_asof", query: "select sym,ts,bid from trades aj quotes on ts order by ts asc", kind: "asof"},
		{name: "global_asof0", query: "select sym,ts,bid from trades aj0 quotes on ts order by ts asc", kind: "asof0"},
		{name: "global_window", query: "select sym,ts,bid from trades wj[-10 0] quotes on ts order by ts asc", kind: "window"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			explained, err := qExplainSQL(qSQLArgsResult{frameValue: envValue, source: tt.query, resolveSource: true, envValue: envValue})
			if err != nil {
				t.Fatalf("explain: %v", err)
			}
			table := explained.Table()
			if got := table.RawGetString("source_query"); !got.IsString() || got.Str() != tt.query {
				t.Fatalf("source_query = %v, want %q", got, tt.query)
			}
			if got := table.RawGetString("join_count"); !got.IsInt() || got.Int() != 1 {
				t.Fatalf("join_count = %v, want 1", got)
			}
			if got := table.RawGetString("source_schema_hash"); !got.IsString() || got.Str() == "" {
				t.Fatalf("source_schema_hash = %v, want stable non-empty hash", got)
			}
			join := table.RawGetString("join").Table()
			if join == nil {
				t.Fatal("join explain is nil")
			}
			if got := join.RawGetString("kind"); !got.IsString() || got.Str() != tt.kind {
				t.Fatalf("join.kind = %v, want %s", got, tt.kind)
			}
			if got := join.RawGetString("time_key"); !got.IsString() || got.Str() != "ts" {
				t.Fatalf("join.time_key = %v, want ts", got)
			}
			partitions := join.RawGetString("partition_keys").Table()
			if partitions == nil || partitions.Length() != 0 {
				t.Fatalf("join.partition_keys = %v, want empty global join partition list", partitions)
			}
			if got := table.RawGetString("kernel_reason"); !got.IsString() || got.Str() == "" {
				t.Fatalf("kernel_reason = %v, want stable non-empty reason", got)
			}
		})
	}
}

func TestQExplainReportsKeyedFrameBridge(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		data.Column{Name: "bucket", Data: data.NewI64([]int64{1, 2, 1})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 101, 80})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym", "bucket")
	if err != nil {
		t.Fatal(err)
	}
	keyedValue := qKeyedFrameToValue(keyed)
	query := "select price from trades where price>=100 order by price asc"

	explained, err := qExplainSQL(qSQLArgsResult{frameValue: keyedValue, source: query})
	if err != nil {
		t.Fatalf("explain keyed q.sql: %v", err)
	}
	table := explained.Table()
	if got := table.RawGetString("source_bridge"); !got.IsString() || got.Str() != "keyed_frame_native" {
		t.Fatalf("source_bridge = %v, want keyed_frame_native", got)
	}
	if got := table.RawGetString("source_native"); !got.IsBool() || !got.Bool() {
		t.Fatalf("source_native = %v, want true", got)
	}
	if got := table.RawGetString("source_keyed"); !got.IsBool() || !got.Bool() {
		t.Fatalf("source_keyed = %v, want true", got)
	}
	if got := table.RawGetString("source_rows"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("source_rows = %v, want 3", got)
	}
	keys := table.RawGetString("source_keys").Table()
	if keys == nil || keys.Length() != 2 {
		t.Fatalf("source_keys = %v, want two keys", keys)
	}
	if got := keys.RawGetInt(1); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("source_keys[1] = %v, want sym", got)
	}
	if got := keys.RawGetInt(2); !got.IsString() || got.Str() != "bucket" {
		t.Fatalf("source_keys[2] = %v, want bucket", got)
	}
}

func TestQExplainReportsKeyedFrameBridgeFromSourceMap(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		data.Column{Name: "bucket", Data: data.NewI64([]int64{1, 2, 1})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 101, 80})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := data.KeyBy(frame, "sym", "bucket")
	if err != nil {
		t.Fatal(err)
	}
	sources := NewTable()
	sources.RawSetString("trades", qKeyedFrameToValue(keyed))
	query := "select price from trades where price>=100 order by price asc"

	explained, err := qExplainSQL(qSQLArgsResult{
		frameValue:    TableValue(sources),
		source:        query,
		resolveSource: true,
		envValue:      TableValue(sources),
	})
	if err != nil {
		t.Fatalf("explain keyed q.sql source map: %v", err)
	}
	table := explained.Table()
	if got := table.RawGetString("source_bridge"); !got.IsString() || got.Str() != "keyed_frame_native" {
		t.Fatalf("source_bridge = %v, want keyed_frame_native", got)
	}
	if got := table.RawGetString("source_keyed"); !got.IsBool() || !got.Bool() {
		t.Fatalf("source_keyed = %v, want true", got)
	}
	if got := table.RawGetString("source_rows"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("source_rows = %v, want 3", got)
	}
	if got := table.RawGetString("source_schema_hash"); !got.IsString() || got.Str() != frame.SchemaFingerprint() {
		t.Fatalf("source_schema_hash = %v, want %s", got, frame.SchemaFingerprint())
	}
}

func TestQSQLQueryKernelCacheExecutesGroupedRollupHotPath(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT", "TSLA"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 101, 80, 190})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20, 30, 40})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatal(err)
	}
	query := "select notional:sum price*size, fills:count i by sym from trades where price>=90 order by notional desc"

	if _, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query}); err != nil {
		t.Fatalf("first grouped q.sql: %v", err)
	}
	second, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query})
	if err != nil {
		t.Fatalf("second grouped q.sql: %v", err)
	}
	rows := second.Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("grouped kernel rows = %v, want two rows", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "TSLA" {
		t.Fatalf("first grouped sym = %v, want TSLA", got)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("notional"); !got.IsFloat() || got.Float() != 7600 {
		t.Fatalf("first grouped notional = %v, want 7600", got)
	}
	if got := rows.RawGetInt(2).Table().RawGetString("fills"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("second grouped fills = %v, want 2", got)
	}

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.KernelMisses != 1 || stats.KernelHits != 1 {
		t.Fatalf("grouped kernel cache stats = %+v, want one miss and one hit", stats)
	}
}

func TestQSQLQueryKernelCacheExecutesDistinctHotPath(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "AAPL", "MSFT"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 100, 80})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatal(err)
	}
	query := "select distinct sym,price from trades order by sym asc"

	if _, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query}); err != nil {
		t.Fatalf("first distinct q.sql: %v", err)
	}
	second, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query})
	if err != nil {
		t.Fatalf("second distinct q.sql: %v", err)
	}
	rows := second.Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("distinct kernel rows = %v, want two rows", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("first distinct sym = %v, want AAPL", got)
	}
	if got := rows.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("second distinct sym = %v, want MSFT", got)
	}

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.KernelMisses != 1 || stats.KernelHits != 1 {
		t.Fatalf("distinct kernel cache stats = %+v, want one miss and one hit", stats)
	}
}

func TestQSQLQueryKernelCacheSplitsBoundScalarValues(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210})},
	)
	if err != nil {
		t.Fatal(err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		t.Fatal(err)
	}
	env := NewTable()
	query := "select sym,price from trades where price>=threshold order by price asc"

	env.RawSetString("threshold", FloatValue(90))
	low, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query, envValue: TableValue(env)})
	if err != nil {
		t.Fatalf("low threshold q.sql: %v", err)
	}
	env.RawSetString("threshold", FloatValue(150))
	high, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query, envValue: TableValue(env)})
	if err != nil {
		t.Fatalf("high threshold q.sql: %v", err)
	}
	lowRows := low.Table()
	highRows := high.Table()
	if lowRows == nil || lowRows.Length() != 2 {
		t.Fatalf("low threshold rows = %v, want 2", lowRows)
	}
	if highRows == nil || highRows.Length() != 1 {
		t.Fatalf("high threshold rows = %v, want 1", highRows)
	}
	if got := highRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("high threshold sym = %v, want NVDA", got)
	}

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.TemplateMisses != 1 || stats.TemplateHits != 1 || stats.AlignedMisses != 1 || stats.AlignedHits != 1 {
		t.Fatalf("plan cache stats = %+v, want template/aligned reused across scalar values", stats)
	}
	if stats.KernelMisses != 2 || stats.KernelHits != 0 {
		t.Fatalf("kernel cache stats = %+v, want bound scalar values to split kernels", stats)
	}
	if len(stats.KernelKeys) != 2 {
		t.Fatalf("kernel key stats rows = %d, want 2 split kernel keys", len(stats.KernelKeys))
	}
	if stats.KernelKeys[0].Shape == "" || stats.KernelKeys[1].Shape == "" || stats.KernelKeys[0].Shape != stats.KernelKeys[1].Shape {
		t.Fatalf("split kernel key shapes = %+v, want same non-empty shape", stats.KernelKeys)
	}
	kernelRow := qTestCacheStatsRowTable(t, qCacheStatsTable(), "qsql_kernel")
	shapeRows := qTestKernelShapeRows(t, kernelRow.RawGetString("shapes").Table())
	if len(shapeRows) != 1 {
		t.Fatalf("qsql_kernel shapes = %+v, want one aggregated shape", shapeRows)
	}
	if got := shapeRows[0]; got.Shape != stats.KernelKeys[0].Shape || got.SchemaHash != frame.SchemaFingerprint() || got.Count != 2 || got.Hits != 0 || got.Misses != 2 || got.Evictions != 0 {
		t.Fatalf("qsql_kernel shape aggregate = %+v, want shape %q schema %q count=2 hits=0 misses=2 evictions=0", got, stats.KernelKeys[0].Shape, frame.SchemaFingerprint())
	}
}

func TestQSQLKernelShapeStatsSplitBySchemaHash(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20, 30})},
	)
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20, 30})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210})},
	)
	if err != nil {
		t.Fatal(err)
	}
	if frame.SchemaFingerprint() == reordered.SchemaFingerprint() {
		t.Fatalf("test setup schema fingerprints match: %s", frame.SchemaFingerprint())
	}

	query := "select sym,notional:price*size from trades where price>=90"
	for name, frame := range map[string]data.Frame{"base": frame, "reordered": reordered} {
		frameValue, err := qDataFrameValue(frame)
		if err != nil {
			t.Fatalf("%s qDataFrameValue: %v", name, err)
		}
		out, err := qRunSQL("q.sql", qSQLArgsResult{frameValue: frameValue, source: query})
		if err != nil {
			t.Fatalf("%s q.sql: %v", name, err)
		}
		rows := out.Table()
		if rows == nil || rows.Length() != 2 {
			t.Fatalf("%s rows = %v, want 2", name, rows)
		}
	}

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.KernelMisses != 2 || stats.KernelHits != 0 || len(stats.KernelKeys) != 2 {
		t.Fatalf("kernel cache stats = %+v, want two schema-split kernel misses", stats)
	}
	if stats.KernelKeys[0].Shape == "" || stats.KernelKeys[0].Shape != stats.KernelKeys[1].Shape {
		t.Fatalf("kernel key shapes = %+v, want same non-empty shape across schemas", stats.KernelKeys)
	}

	kernelRow := qTestCacheStatsRowTable(t, qCacheStatsTable(), "qsql_kernel")
	shapeRows := qTestKernelShapeRows(t, kernelRow.RawGetString("shapes").Table())
	if len(shapeRows) != 2 {
		t.Fatalf("qsql_kernel shapes = %+v, want one row per schema hash", shapeRows)
	}
	wantSchemas := map[string]bool{
		frame.SchemaFingerprint():     false,
		reordered.SchemaFingerprint(): false,
	}
	for _, row := range shapeRows {
		if row.Shape != stats.KernelKeys[0].Shape || row.Count != 1 || row.Hits != 0 || row.Misses != 1 || row.Evictions != 0 {
			t.Fatalf("qsql_kernel shape row = %+v, want schema-local count=1 miss=1", row)
		}
		if _, ok := wantSchemas[row.SchemaHash]; !ok {
			t.Fatalf("qsql_kernel shape schema hash = %q, want one of %#v", row.SchemaHash, wantSchemas)
		}
		wantSchemas[row.SchemaHash] = true
	}
	for schemaHash, seen := range wantSchemas {
		if !seen {
			t.Fatalf("qsql_kernel shape schema hash %q missing from %+v", schemaHash, shapeRows)
		}
	}
}

func TestQSQLKernelUnsupportedDecisionCacheIsSchemaStable(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
	)
	if err != nil {
		t.Fatal(err)
	}
	plan := data.QueryPlan{
		Select: []data.SelectItem{{Name: "marker", Expr: qFallbackStatsTestExpr{}}},
		LimitN: -1,
	}
	src := "unsupported-kernel-decision-cache"
	for i := 0; i < 2; i++ {
		if _, err := qRunSQLPlan(src, plan, frame); err != nil {
			t.Fatalf("unsupported qRunSQLPlan run %d: %v", i+1, err)
		}
	}

	qSQLAlignedPlanCacheMu.Lock()
	if len(qSQLKernelUnsupported) != 1 {
		t.Fatalf("unsupported kernel decision entries = %d, want 1", len(qSQLKernelUnsupported))
	}
	var reason string
	for _, cachedReason := range qSQLKernelUnsupported {
		reason = cachedReason
	}
	qSQLAlignedPlanCacheMu.Unlock()
	wantReason := fmt.Sprintf("select expression %q is not supported by data query kernel: unsupported expression %T", "marker", qFallbackStatsTestExpr{})
	if reason != wantReason {
		t.Fatalf("unsupported kernel reason = %q, want %q", reason, wantReason)
	}
	stats := qSQLPlanCacheStatsSnapshot()
	if stats.KernelMisses != 0 || stats.KernelHits != 0 || stats.KernelEvictions != 0 {
		t.Fatalf("positive kernel cache stats = %+v, want unsupported decisions outside hit/miss/eviction stats", stats)
	}
	if stats.KernelDecisionMisses != 1 || stats.KernelDecisionHits != 1 || stats.KernelDecisionEvictions != 0 {
		t.Fatalf("kernel decision cache stats = %+v, want one unsupported decision miss, one hit, zero evictions", stats)
	}
	cacheStats := qTestCacheStatsRows(t, qCacheStatsTable())
	if got := cacheStats["qsql_kernel_decision"]; got["entries"] != 1 || got["hits"] != 1 || got["misses"] != 1 || got["evictions"] != 0 {
		t.Fatalf("qsql_kernel_decision cache stats = %#v, want 1 entry, 1 hit, 1 miss, 0 evictions", got)
	}
	decisionRow := qTestCacheStatsRowTable(t, qCacheStatsTable(), "qsql_kernel_decision")
	keyRows := qTestKernelDecisionKeyRows(t, decisionRow.RawGetString("keys").Table())
	if len(keyRows) != 1 {
		t.Fatalf("qsql_kernel_decision keys = %#v, want one row", keyRows)
	}
	if keyRows[0].Namespace != src || keyRows[0].Kind != "kernel" || keyRows[0].SchemaHash != frame.SchemaFingerprint() {
		t.Fatalf("qsql_kernel_decision key row = %+v, want source/schema key metadata", keyRows[0])
	}
	if keyRows[0].Shape != "projection|projection=computed" {
		t.Fatalf("qsql_kernel_decision key shape = %+v, want computed projection shape", keyRows[0])
	}
	if keyRows[0].ReasonFamily != qFallbackFamilySelect || keyRows[0].ReasonCode != stdq.KernelFallbackSelectExpression || keyRows[0].Count != 1 {
		t.Fatalf("qsql_kernel_decision key reason = %+v, want select reason count 1", keyRows[0])
	}
	qSQLAlignedPlanCacheMu.Lock()
	decisionKeyStats := qSQLKernelDecisionKeyStatsSnapshotLocked()
	qSQLAlignedPlanCacheMu.Unlock()
	decisionJSONRows := QSQLKernelDecisionKeyStatJSONRows(decisionKeyStats)
	if len(decisionJSONRows) != 1 {
		t.Fatalf("QSQLKernelDecisionKeyStatJSONRows = %+v, want one row", decisionJSONRows)
	}
	if got := decisionJSONRows[0]; got.Key != keyRows[0].Key || got.Namespace != src || got.Kind != "kernel" || got.SchemaHash != frame.SchemaFingerprint() || got.Shape != keyRows[0].Shape || got.ReasonFamily != qFallbackFamilySelect || got.ReasonCode != stdq.KernelFallbackSelectExpression || got.Count != 1 {
		t.Fatalf("QSQLKernelDecisionKeyStatJSONRows[0] = %+v, want stable qsql decision key row", got)
	}
	decisionJSON, err := json.Marshal(decisionJSONRows)
	if err != nil {
		t.Fatalf("marshal QSQLKernelDecisionKeyStatJSONRows: %v", err)
	}
	if !strings.Contains(string(decisionJSON), `"schema_hash"`) || !strings.Contains(string(decisionJSON), `"reason_code"`) || strings.Contains(string(decisionJSON), "SchemaHash") || strings.Contains(string(decisionJSON), "ReasonCode") {
		t.Fatalf("QSQLKernelDecisionKeyStatJSONRows JSON = %s, want snake_case stable fields", decisionJSON)
	}
	reasonRows := qTestKernelDecisionReasonRows(t, decisionRow.RawGetString("reasons").Table())
	if got := qTestKernelDecisionReasonCount(reasonRows, qFallbackFamilySelect, stdq.KernelFallbackSelectExpression); got != 1 {
		t.Fatalf("qsql_kernel_decision select reason aggregate = %d, want 1", got)
	}
	shapeRows := qTestKernelDecisionShapeRows(t, decisionRow.RawGetString("shapes").Table())
	if got := qTestKernelDecisionShapeCount(shapeRows, qFallbackFamilySelect, stdq.KernelFallbackSelectExpression, keyRows[0].SchemaHash, keyRows[0].Shape); got != 1 {
		t.Fatalf("qsql_kernel_decision select shape aggregate = %d, want 1", got)
	}
	fallbackRows := qTestFallbackStatsDetailRows(t, qFallbackStatsTable())
	if got := qTestFallbackAttributionCount(fallbackRows, qFallbackKernelUnsupported, stdq.KernelFallbackSelectExpression, src, keyRows[0].SchemaHash, keyRows[0].Shape); got != 2 {
		t.Fatalf("qsql_kernel_decision fallback attribution count = %d, want 2 executions for schema/shape", got)
	}
}

func TestQSQLKernelUnsupportedDecisionShapeStatsAreSchemaStable(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frameA, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA"})},
	)
	if err != nil {
		t.Fatalf("NewFrame frameA: %v", err)
	}
	frameB, err := data.NewFrame(
		data.Column{Name: "venue", Data: data.NewSymbols([]string{"XNYS", "XNAS", "BATS"})},
	)
	if err != nil {
		t.Fatalf("NewFrame frameB: %v", err)
	}
	plan := data.QueryPlan{
		Select: []data.SelectItem{{Name: "marker", Expr: qFallbackStatsTestExpr{}}},
		LimitN: -1,
	}
	src := "unsupported-kernel-decision-shape-schema-cache"
	for label, frame := range map[string]data.Frame{"frameA": frameA, "frameB": frameB} {
		if _, err := qRunSQLPlan(src, plan, frame); err != nil {
			t.Fatalf("unsupported qRunSQLPlan %s: %v", label, err)
		}
	}

	decisionRow := qTestCacheStatsRowTable(t, qCacheStatsTable(), "qsql_kernel_decision")
	keyRows := qTestKernelDecisionKeyRows(t, decisionRow.RawGetString("keys").Table())
	if len(keyRows) != 2 {
		t.Fatalf("qsql_kernel_decision keys = %#v, want two schema-specific rows", keyRows)
	}
	shapeRows := qTestKernelDecisionShapeRows(t, decisionRow.RawGetString("shapes").Table())
	if len(shapeRows) != 2 {
		t.Fatalf("qsql_kernel_decision shapes = %#v, want one row per schema", shapeRows)
	}
	seenSchemas := make(map[string]bool)
	for _, keyRow := range keyRows {
		if keyRow.Shape != "projection|projection=computed" {
			t.Fatalf("qsql_kernel_decision key shape = %+v, want computed projection shape", keyRow)
		}
		if got := qTestKernelDecisionShapeCount(shapeRows, keyRow.ReasonFamily, keyRow.ReasonCode, keyRow.SchemaHash, keyRow.Shape); got != 1 {
			t.Fatalf("qsql_kernel_decision shape aggregate for schema %s = %d, want 1; rows=%+v", keyRow.SchemaHash, got, shapeRows)
		}
		seenSchemas[keyRow.SchemaHash] = true
	}
	if !seenSchemas[frameA.SchemaFingerprint()] || !seenSchemas[frameB.SchemaFingerprint()] {
		t.Fatalf("qsql_kernel_decision schemas = %#v, want %s and %s", seenSchemas, frameA.SchemaFingerprint(), frameB.SchemaFingerprint())
	}
}

func TestQSQLAsofJoinExecution(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"AAPL\", \"MSFT\", \"AAPL\"},\n"+
			"    ts: array.i64({10, 15, 12, 25}),\n"+
			"    price: array.f64({100, 101, 80, 103}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", ts: \"i64\", price: \"f64\"}\n"+
			"quotes := data.frame({\n"+
			"    sym: {\"AAPL\", \"AAPL\", \"MSFT\", \"AAPL\"},\n"+
			"    ts: array.i64({8, 20, 11, 14}),\n"+
			"    bid: array.f64({99, 102, 79, 100.5}),\n"+
			"})\n"+
			"quotes.column_kinds = {sym: \"symbol\", ts: \"i64\", bid: \"f64\"}\n"+
			"joined := q.sql(\"select sym,ts,price,bid from trades aj quotes on sym,ts order by ts asc\", {trades: trades, quotes: quotes})\n"+
			"joined_aj0 := q.sql(\"select sym,ts,price,bid from trades aj0 quotes on sym,ts order by ts asc\", {trades: trades, quotes: quotes})\n"+
			"joined_ajf := q.sql(\"select sym,ts,price,bid from trades ajf quotes on sym,ts order by ts asc\", {trades: trades, quotes: quotes})\n"+
			"joined_ajf0 := q.sql(\"select sym,ts,price,bid from trades ajf0 quotes on sym,ts order by ts asc\", {trades: trades, quotes: quotes})\n"+
			"global_joined := q.sql(\"select sym,ts,price,bid from trades aj quotes on ts order by ts asc\", {trades: trades, quotes: quotes})\n")

	joined := interp.GetGlobal("joined").Table()
	if joined == nil || joined.Length() != 4 {
		t.Fatalf("joined len = %v, want 4", joined)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 99 {
		t.Fatalf("joined[1].bid = %v, want 99", got)
	}
	if got := joined.RawGetInt(2).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 79 {
		t.Fatalf("joined[2].bid = %v, want 79", got)
	}
	if got := joined.RawGetInt(3).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("joined[3].bid = %v, want 100.5", got)
	}
	if got := joined.RawGetInt(4).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 102 {
		t.Fatalf("joined[4].bid = %v, want 102", got)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("ts"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("joined[1].ts = %v, want left time 10", got)
	}
	if got := interp.GetGlobal("joined_ajf").Table().RawGetInt(1).Table().RawGetString("ts"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("joined_ajf[1].ts = %v, want left time 10", got)
	}
	if got := interp.GetGlobal("joined_aj0").Table().RawGetInt(1).Table().RawGetString("ts"); !got.IsInt() || got.Int() != 8 {
		t.Fatalf("joined_aj0[1].ts = %v, want right time 8", got)
	}
	if got := interp.GetGlobal("joined_ajf0").Table().RawGetInt(1).Table().RawGetString("ts"); !got.IsInt() || got.Int() != 8 {
		t.Fatalf("joined_ajf0[1].ts = %v, want right time 8", got)
	}
	for _, name := range []string{"joined_aj0", "joined_ajf", "joined_ajf0"} {
		variant := interp.GetGlobal(name).Table()
		if variant == nil || variant.Length() != 4 {
			t.Fatalf("%s len = %v, want 4", name, variant)
		}
		for i, want := range []float64{99, 79, 100.5, 102} {
			got := variant.RawGetInt(int64(i + 1)).Table().RawGetString("bid")
			if !got.IsFloat() || got.Float() != want {
				t.Fatalf("%s[%d].bid = %v, want %v", name, i+1, got, want)
			}
		}
	}
	globalJoined := interp.GetGlobal("global_joined").Table()
	if globalJoined == nil || globalJoined.Length() != 4 {
		t.Fatalf("global_joined len = %v, want 4", globalJoined)
	}
	for i, want := range []float64{99, 79, 100.5, 102} {
		got := globalJoined.RawGetInt(int64(i + 1)).Table().RawGetString("bid")
		if !got.IsFloat() || got.Float() != want {
			t.Fatalf("global_joined[%d].bid = %v, want %v", i+1, got, want)
		}
	}
}

func TestQSQLAsofJoinBoundaryRows(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"MSFT\", \"TSLA\"}),\n"+
			"    ts: array.i64({5, 10, 9, 12}),\n"+
			"    price: data.f64({99.0, 100.0, 80.0, 200.0}),\n"+
			"})\n"+
			"quotes := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"MSFT\"}),\n"+
			"    ts: array.i64({5, 8, 10}),\n"+
			"    bid: data.f64({98.5, 99.5, 79.5}),\n"+
			"})\n"+
			"joined := q.sql(\"select sym,ts,price,bid from trades asof join quotes on sym,ts order by sym asc,ts asc\", {trades: trades, quotes: quotes})\n"+
			"joined_aj0 := q.sql(\"select sym,ts,price,bid from trades aj0 quotes on sym,ts order by price asc\", {trades: trades, quotes: quotes})\n")

	joined := interp.GetGlobal("joined").Table()
	if joined == nil || joined.Length() != 4 {
		t.Fatalf("joined len = %v, want 4", joined)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 98.5 {
		t.Fatalf("joined[1].bid = %v, want exact-time match 98.5", got)
	}
	if got := joined.RawGetInt(2).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 99.5 {
		t.Fatalf("joined[2].bid = %v, want latest prior 99.5", got)
	}
	if got := joined.RawGetInt(3).Table().RawGetString("bid"); !got.IsNil() {
		t.Fatalf("joined[3].bid = %v, want nil before first quote", got)
	}
	if got := joined.RawGetInt(4).Table().RawGetString("bid"); !got.IsNil() {
		t.Fatalf("joined[4].bid = %v, want nil for missing partition", got)
	}
	joinedAJ0 := interp.GetGlobal("joined_aj0").Table()
	if joinedAJ0 == nil || joinedAJ0.Length() != 4 {
		t.Fatalf("joined_aj0 len = %v, want 4", joinedAJ0)
	}
	if got := joinedAJ0.RawGetInt(1).Table().RawGetString("ts"); !got.IsNil() {
		t.Fatalf("joined_aj0[1].ts = %v, want nil before first quote", got)
	}
	if got := joinedAJ0.RawGetInt(2).Table().RawGetString("ts"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("joined_aj0[2].ts = %v, want exact right time 5", got)
	}
	if got := joinedAJ0.RawGetInt(3).Table().RawGetString("ts"); !got.IsInt() || got.Int() != 8 {
		t.Fatalf("joined_aj0[3].ts = %v, want latest right time 8", got)
	}
	if got := joinedAJ0.RawGetInt(4).Table().RawGetString("ts"); !got.IsNil() {
		t.Fatalf("joined_aj0[4].ts = %v, want nil for missing partition", got)
	}
}

func TestQSQLLeftJoinExecution(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"MSFT\", \"NVDA\"}),\n"+
			"    price: data.f64({100.0, 80.0, 200.0}),\n"+
			"})\n"+
			"ref := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"MSFT\"}),\n"+
			"    sector: {\"tech\", \"software\"},\n"+
			"})\n"+
			"joined := q.sql(\"select sym,price,sector from trades left join ref on sym order by sym asc\", {trades: trades, ref: ref})\n"+
			"joined_lj := q.sql(\"select sym,price,sector from trades lj ref on sym order by sym asc\", {trades: trades, ref: ref})\n")

	joined := interp.GetGlobal("joined").Table()
	if joined == nil || joined.Length() != 3 {
		t.Fatalf("joined len = %v, want 3", joined)
	}
	if got := joined.RawGetInt(3).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("joined[3].sym = %v, want NVDA", got)
	}
	if got := joined.RawGetInt(3).Table().RawGetString("sector"); !got.IsNil() {
		t.Fatalf("joined[3].sector = %v, want nil", got)
	}
	if got := joined.RawGetString("column_kinds").Table().RawGetString("sector"); !got.IsString() || got.Str() != "string" {
		t.Fatalf("joined.column_kinds.sector = %v, want string", got)
	}
	if got := joined.RawGetString("schema").Table().RawGetString("kinds").Table().RawGetString("sector"); !got.IsString() || got.Str() != "string" {
		t.Fatalf("joined.schema.kinds.sector = %v, want string", got)
	}
	joinedLJ := interp.GetGlobal("joined_lj").Table()
	if joinedLJ == nil || joinedLJ.Length() != 3 {
		t.Fatalf("joined_lj len = %v, want 3", joinedLJ)
	}
	if got := joinedLJ.RawGetString("column_kinds").Table().RawGetString("sector"); !got.IsString() || got.Str() != "string" {
		t.Fatalf("joined_lj.column_kinds.sector = %v, want string", got)
	}
}

func TestQSQLJoinWithKeyedRightUsesLatestValueRow(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"MSFT\", \"TSLA\"}),\n"+
			"    qty: data.i64({10, 20, 30}),\n"+
			"})\n"+
			"ref := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"MSFT\"}),\n"+
			"    seq: data.i64({1, 2, 3}),\n"+
			"    sector: data.strings({\"old-tech\", \"tech\", \"software\"}),\n"+
			"})\n"+
			"keyed_ref := q.key_by(ref, \"sym\")\n"+
			"left_joined := q.sql(\"select sym,qty,seq,sector from trades left join ref on sym order by sym asc\", {trades: trades, ref: keyed_ref})\n"+
			"inner_joined := q.sql(\"select sym,qty,seq,sector from trades join ref on sym order by sym asc\", {trades: trades, ref: keyed_ref})\n")

	leftJoined := interp.GetGlobal("left_joined").Table()
	if leftJoined == nil || leftJoined.Length() != 3 {
		t.Fatalf("left_joined len = %v, want 3", leftJoined)
	}
	if got := leftJoined.RawGetInt(1).Table().RawGetString("seq"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("left_joined[1].seq = %v, want latest AAPL row 2", got)
	}
	if got := leftJoined.RawGetInt(1).Table().RawGetString("sector"); !got.IsString() || got.Str() != "tech" {
		t.Fatalf("left_joined[1].sector = %v, want tech", got)
	}
	if got := leftJoined.RawGetInt(3).Table().RawGetString("seq"); !got.IsNil() {
		t.Fatalf("left_joined[3].seq = %v, want nil unmatched keyed row", got)
	}

	innerJoined := interp.GetGlobal("inner_joined").Table()
	if innerJoined == nil || innerJoined.Length() != 2 {
		t.Fatalf("inner_joined len = %v, want 2", innerJoined)
	}
	if got := innerJoined.RawGetInt(1).Table().RawGetString("seq"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("inner_joined[1].seq = %v, want latest AAPL row 2", got)
	}
	if got := innerJoined.RawGetInt(2).Table().RawGetString("seq"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("inner_joined[2].seq = %v, want MSFT row 3", got)
	}
}

func TestQSQLUnionJoinExecution(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp, `
left := data.frame({
    sym: data.symbols({"AAPL", "MSFT"}),
    price: data.f64({100.0, 80.0}),
})
rhs := data.frame({
    sym: data.symbols({"AAPL", "NVDA"}),
    price: data.f64({101.0, 200.0}),
    venue: data.strings({"XASE", "XNAS"}),
})
joined := q.sql("select sym,price,venue from left uj rhs on sym order by sym asc", {left: left, rhs: rhs})
plus_left := data.frame({
    sym: data.symbols({"AAPL", "MSFT"}),
    qty: data.i64({10, 20}),
})
plus_right := data.frame({
    sym: data.symbols({"AAPL"}),
    qty: data.i64({3}),
    venue: data.strings({"XNAS"}),
})
plus_joined := q.sql("select sym,qty,venue from plus_left pj plus_right on sym order by sym asc", {plus_left: plus_left, plus_right: plus_right})
window_left := data.frame({
    sym: data.symbols({"AAPL", "AAPL", "MSFT"}),
    ts: data.timestamp({"2026-06-02T09:30:10Z", "2026-06-02T09:30:20Z", "2026-06-02T09:30:20Z"}),
})
window_right := data.frame({
    sym: data.symbols({"AAPL", "AAPL", "MSFT"}),
    ts: data.timestamp({"2026-06-02T09:30:05Z", "2026-06-02T09:30:15Z", "2026-06-02T09:30:21Z"}),
    bid: data.f64({100.0, 101.0, 200.0}),
})
window_joined := q.sql("select sym,ts,bid from window_left wj window_right on sym,ts order by sym asc,ts asc", {window_left: window_left, window_right: window_right})
window_bounded := q.sql("select sym,ts,bid from window_left wj[-10000000000 0] window_right on sym,ts order by sym asc,ts asc", {window_left: window_left, window_right: window_right})
window_last := q.sql("select sym,ts,bid from window_left wj1[-10000000000 0] window_right on sym,ts order by sym asc,ts asc", {window_left: window_left, window_right: window_right})
window_aggs := q.sql("select sym,ts,n:count bid,sum_bid:sum bid,avg_bid:avg bid,first_bid:first bid,last_bid:last bid from window_left wj window_right on sym,ts order by sym asc,ts asc", {window_left: window_left, window_right: window_right})
window_partition_boundary := q.sql("select sym,ts,bid from window_left wj[-10000000000 0] window_right on sym,ts order by sym asc,ts asc", {window_left: data.frame({sym: data.symbols({"MSFT"}), ts: data.timestamp({"2026-06-02T09:30:20Z"})}), window_right: data.frame({sym: data.symbols({"AAPL"}), ts: data.timestamp({"2026-06-02T09:30:15Z"}), bid: data.f64({101.0})})})
global_window_left := data.frame({
    ts: data.i64({10, 20, 30}),
})
global_window_right := data.frame({
    ts: data.i64({5, 15, 25}),
    bid: data.f64({100.0, 101.0, 102.0}),
})
global_window := q.sql("select ts,bid from global_window_left wj[-10 0] global_window_right on ts order by ts asc", {global_window_left: global_window_left, global_window_right: global_window_right})
global_window_last := q.sql("select ts,bid from global_window_left wj1[-10 0] global_window_right on ts order by ts asc", {global_window_left: global_window_left, global_window_right: global_window_right})
global_asof := q.sql("select ts,bid from global_window_left aj global_window_right on ts order by ts asc", {global_window_left: global_window_left, global_window_right: global_window_right})
	`)

	joined := interp.GetGlobal("joined").Table()
	if joined == nil || joined.Length() != 3 {
		t.Fatalf("joined len = %v, want 3", joined)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("joined[1].sym = %v, want AAPL", got)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("joined[1].price = %v, want left price 100", got)
	}
	if got := joined.RawGetInt(1).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XASE" {
		t.Fatalf("joined[1].venue = %v, want matched right venue XASE", got)
	}
	if got := joined.RawGetInt(3).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("joined[3].sym = %v, want NVDA", got)
	}
	if got := joined.RawGetInt(3).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNAS" {
		t.Fatalf("joined[3].venue = %v, want XNAS", got)
	}
	kinds := joined.RawGetString("column_kinds").Table()
	if got := kinds.RawGetString("price"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("joined.column_kinds.price = %v, want f64", got)
	}
	if got := kinds.RawGetString("venue"); !got.IsString() || got.Str() != "string" {
		t.Fatalf("joined.column_kinds.venue = %v, want string", got)
	}
	plusJoined := interp.GetGlobal("plus_joined").Table()
	if plusJoined == nil || plusJoined.Length() != 2 {
		t.Fatalf("plus_joined len = %v, want 2", plusJoined)
	}
	if got := plusJoined.RawGetInt(1).Table().RawGetString("qty"); !got.IsFloat() || got.Float() != 13 {
		t.Fatalf("plus_joined[1].qty = %v, want 13", got)
	}
	if got := plusJoined.RawGetInt(2).Table().RawGetString("qty"); !got.IsFloat() || got.Float() != 20 {
		t.Fatalf("plus_joined[2].qty = %v, want 20", got)
	}
	if got := plusJoined.RawGetInt(1).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNAS" {
		t.Fatalf("plus_joined[1].venue = %v, want XNAS", got)
	}
	if got := plusJoined.RawGetInt(2).Table().RawGetString("venue"); !got.IsNil() {
		t.Fatalf("plus_joined[2].venue = %v, want nil", got)
	}
	windowJoined := interp.GetGlobal("window_joined").Table()
	if windowJoined == nil || windowJoined.Length() != 3 {
		t.Fatalf("window_joined len = %v, want 3", windowJoined)
	}
	firstBids := windowJoined.RawGetInt(1).Table().RawGetString("bid").Table()
	if firstBids == nil || firstBids.Length() != 1 {
		t.Fatalf("window_joined[1].bid = %v, want one prior bid", firstBids)
	}
	if got := firstBids.RawGetInt(1); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("window_joined[1].bid[1] = %v, want 100", got)
	}
	secondBids := windowJoined.RawGetInt(2).Table().RawGetString("bid").Table()
	if secondBids == nil || secondBids.Length() != 2 {
		t.Fatalf("window_joined[2].bid = %v, want two prior bids", secondBids)
	}
	if got := secondBids.RawGetInt(2); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("window_joined[2].bid[2] = %v, want 101", got)
	}
	thirdBids := windowJoined.RawGetInt(3).Table().RawGetString("bid").Table()
	if thirdBids == nil || thirdBids.Length() != 0 {
		t.Fatalf("window_joined[3].bid = %v, want empty bid list", thirdBids)
	}
	partitionBoundary := interp.GetGlobal("window_partition_boundary").Table()
	if partitionBoundary == nil || partitionBoundary.Length() != 1 {
		t.Fatalf("window_partition_boundary len = %v, want 1", partitionBoundary)
	}
	partitionBoundaryBids := partitionBoundary.RawGetInt(1).Table().RawGetString("bid").Table()
	if partitionBoundaryBids == nil || partitionBoundaryBids.Length() != 0 {
		t.Fatalf("window_partition_boundary[1].bid = %v, want empty bid list", partitionBoundaryBids)
	}
	windowBounded := interp.GetGlobal("window_bounded").Table()
	if windowBounded == nil || windowBounded.Length() != 3 {
		t.Fatalf("window_bounded len = %v, want 3", windowBounded)
	}
	boundedSecondBids := windowBounded.RawGetInt(2).Table().RawGetString("bid").Table()
	if boundedSecondBids == nil || boundedSecondBids.Length() != 1 {
		t.Fatalf("window_bounded[2].bid = %v, want one in-bounds bid", boundedSecondBids)
	}
	if got := boundedSecondBids.RawGetInt(1); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("window_bounded[2].bid[1] = %v, want 101", got)
	}
	windowLast := interp.GetGlobal("window_last").Table()
	if windowLast == nil || windowLast.Length() != 3 {
		t.Fatalf("window_last len = %v, want 3", windowLast)
	}
	if got := windowLast.RawGetInt(1).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("window_last[1].bid = %v, want 100", got)
	}
	if got := windowLast.RawGetInt(2).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("window_last[2].bid = %v, want 101", got)
	}
	if got := windowLast.RawGetInt(3).Table().RawGetString("bid"); !got.IsNil() {
		t.Fatalf("window_last[3].bid = %v, want nil", got)
	}
	windowAggs := interp.GetGlobal("window_aggs").Table()
	if windowAggs == nil || windowAggs.Length() != 3 {
		t.Fatalf("window_aggs len = %v, want 3", windowAggs)
	}
	if got := windowAggs.RawGetInt(1).Table().RawGetString("n"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("window_aggs[1].n = %v, want 1", got)
	}
	if got := windowAggs.RawGetInt(2).Table().RawGetString("sum_bid"); !got.IsFloat() || got.Float() != 201 {
		t.Fatalf("window_aggs[2].sum_bid = %v, want 201", got)
	}
	if got := windowAggs.RawGetInt(2).Table().RawGetString("avg_bid"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("window_aggs[2].avg_bid = %v, want 100.5", got)
	}
	if got := windowAggs.RawGetInt(2).Table().RawGetString("first_bid"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("window_aggs[2].first_bid = %v, want 100", got)
	}
	if got := windowAggs.RawGetInt(2).Table().RawGetString("last_bid"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("window_aggs[2].last_bid = %v, want 101", got)
	}
	if got := windowAggs.RawGetInt(3).Table().RawGetString("n"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("window_aggs[3].n = %v, want 0", got)
	}
	if got := windowAggs.RawGetInt(3).Table().RawGetString("avg_bid"); !got.IsNil() {
		t.Fatalf("window_aggs[3].avg_bid = %v, want nil", got)
	}
	if got := windowAggs.RawGetInt(3).Table().RawGetString("sum_bid"); !got.IsFloat() || got.Float() != 0 {
		t.Fatalf("window_aggs[3].sum_bid = %v, want 0", got)
	}
	globalWindow := interp.GetGlobal("global_window").Table()
	if globalWindow == nil || globalWindow.Length() != 3 {
		t.Fatalf("global_window len = %v, want 3", globalWindow)
	}
	globalFirstBids := globalWindow.RawGetInt(1).Table().RawGetString("bid").Table()
	if globalFirstBids == nil || globalFirstBids.Length() != 1 {
		t.Fatalf("global_window[1].bid = %v, want one value", globalFirstBids)
	}
	if got := globalFirstBids.RawGetInt(1); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("global_window[1].bid[1] = %v, want 100", got)
	}
	globalLast := interp.GetGlobal("global_window_last").Table()
	if globalLast == nil || globalLast.Length() != 3 {
		t.Fatalf("global_window_last len = %v, want 3", globalLast)
	}
	if got := globalLast.RawGetInt(2).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("global_window_last[2].bid = %v, want 101", got)
	}
	globalAsof := interp.GetGlobal("global_asof").Table()
	if globalAsof == nil || globalAsof.Length() != 3 {
		t.Fatalf("global_asof len = %v, want 3", globalAsof)
	}
	if got := globalAsof.RawGetInt(3).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 102 {
		t.Fatalf("global_asof[3].bid = %v, want 102", got)
	}
}

func TestQSQLWindowJoinTimespanBoundsOnTimestampKeys(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp, `
trades := data.frame({
    sym: data.symbols({"AAPL", "AAPL", "AAPL"}),
    ts: data.timestamp({"2026-06-06T09:31:00Z", "2026-06-06T09:32:00Z", data.null}),
})
quotes := data.frame({
    sym: data.symbols({"AAPL", "AAPL", "AAPL", "AAPL"}),
    ts: data.timestamp({"2026-06-06T09:30:00Z", "2026-06-06T09:31:00Z", "2026-06-06T09:31:30Z", "2026-06-06T09:32:00Z"}),
    bid: data.f64({100.0, 100.5, 100.75, 101.0}),
})
windowed := q.sql("select sym,ts,bid from trades wj[-0D00:01:00 0D00:00:00] quotes on sym,ts order by ts asc", {trades: trades, quotes: quotes})
lasted := q.sql("select sym,ts,bid from trades wj1[-0D00:01:00 0D00:00:00] quotes on sym,ts order by ts asc", {trades: trades, quotes: quotes})
`)

	windowed := interp.GetGlobal("windowed").Table()
	if windowed == nil || windowed.Length() != 3 {
		t.Fatalf("windowed len = %v, want 3", windowed)
	}
	firstBids := windowed.RawGetInt(1).Table().RawGetString("bid").Table()
	if firstBids == nil || firstBids.Length() != 0 {
		t.Fatalf("windowed[1].bid = %v, want empty list for null timestamp", firstBids)
	}
	firstBids = windowed.RawGetInt(2).Table().RawGetString("bid").Table()
	if firstBids == nil || firstBids.Length() != 2 {
		t.Fatalf("windowed[2].bid = %v, want two bids", firstBids)
	}
	if got := firstBids.RawGetInt(1); !got.IsFloat() || got.Float() != 100.0 {
		t.Fatalf("windowed[2].bid[1] = %v, want 100.0", got)
	}
	if got := firstBids.RawGetInt(2); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("windowed[2].bid[2] = %v, want 100.5", got)
	}
	secondBids := windowed.RawGetInt(3).Table().RawGetString("bid").Table()
	if secondBids == nil || secondBids.Length() != 3 {
		t.Fatalf("windowed[3].bid = %v, want three bids", secondBids)
	}
	if got := secondBids.RawGetInt(3); !got.IsFloat() || got.Float() != 101.0 {
		t.Fatalf("windowed[3].bid[3] = %v, want 101.0", got)
	}

	lasted := interp.GetGlobal("lasted").Table()
	if lasted == nil || lasted.Length() != 3 {
		t.Fatalf("lasted len = %v, want 3", lasted)
	}
	if got := lasted.RawGetInt(1).Table().RawGetString("bid"); !got.IsNil() {
		t.Fatalf("lasted[1].bid = %v, want nil", got)
	}
	if got := lasted.RawGetInt(2).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("lasted[2].bid = %v, want 100.5", got)
	}
	if got := lasted.RawGetInt(3).Table().RawGetString("bid"); !got.IsFloat() || got.Float() != 101.0 {
		t.Fatalf("lasted[3].bid = %v, want 101.0", got)
	}
}

func TestQSQLXbarBucketExecution(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"AAPL\", \"MSFT\", \"AAPL\"},\n"+
			"    ts: array.i64({3, 8, 12, 17}),\n"+
			"    size: array.i64({10, 20, 30, 40}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", ts: \"i64\", size: \"i64\"}\n"+
			"buckets := q.sql(trades, \"select qty:sum size, fills:count i by bucket:xbar 10 ts from trades order by bucket asc\")\n")

	buckets := interp.GetGlobal("buckets").Table()
	if buckets == nil || buckets.Length() != 2 {
		t.Fatalf("buckets len = %v, want 2", buckets)
	}
	if got := buckets.RawGetInt(1).Table().RawGetString("bucket"); !got.IsInt() || got.Int() != 0 {
		t.Fatalf("buckets[1].bucket = %v, want 0", got)
	}
	if got := buckets.RawGetInt(1).Table().RawGetString("qty"); !got.IsFloat() || got.Float() != 30 {
		t.Fatalf("buckets[1].qty = %v, want 30", got)
	}
	if got := buckets.RawGetInt(2).Table().RawGetString("bucket"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("buckets[2].bucket = %v, want 10", got)
	}
	if got := buckets.RawGetInt(2).Table().RawGetString("fills"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("buckets[2].fills = %v, want 2", got)
	}
}

func TestQSQLAdditionalAggregatesAndKeyedLookup(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"MSFT\"}),\n"+
			"    price: data.f64({100.5, 101.5, 80.0}),\n"+
			"    size: array.i64({10, 20, 30}),\n"+
			"})\n"+
			"stats := q.sql(trades, \"select lo:min price, hi:max price, first_px:first price, last_px:last price by sym from trades order by sym asc\")\n"+
			"keyed := q.key_by(trades, \"sym\")\n"+
			"aapl := q.lookup(keyed, \"AAPL\")\n"+
			"miss := q.lookup(keyed, \"IBM\")\n"+
			"bad_arity_ok, bad_arity_err := pcall(func() {\n"+
			"    return q.lookup(keyed, \"AAPL\", \"XNYS\")\n"+
			"})\n")

	stats := interp.GetGlobal("stats").Table()
	if stats == nil || stats.Length() != 2 {
		t.Fatalf("stats len = %v, want 2", stats)
	}
	first := stats.RawGetInt(1).Table()
	if got := first.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("stats[1].sym = %v, want AAPL", got)
	}
	if got := first.RawGetString("lo"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("stats[1].lo = %v, want 100.5", got)
	}
	if got := first.RawGetString("hi"); !got.IsFloat() || got.Float() != 101.5 {
		t.Fatalf("stats[1].hi = %v, want 101.5", got)
	}
	if got := first.RawGetString("first_px"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("stats[1].first_px = %v, want 100.5", got)
	}
	if got := first.RawGetString("last_px"); !got.IsFloat() || got.Float() != 101.5 {
		t.Fatalf("stats[1].last_px = %v, want 101.5", got)
	}

	aapl := interp.GetGlobal("aapl").Table()
	if aapl == nil || aapl.Length() != 1 {
		t.Fatalf("aapl len = %v, want 1", aapl)
	}
	if got := aapl.RawGetInt(1).Table().RawGetString("size"); !got.IsInt() || got.Int() != 20 {
		t.Fatalf("aapl[1].size = %v, want latest row size 20", got)
	}
	if got := aapl.RawGetInt(1).Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("aapl[1].sym = %v, want nil because keyed lookup returns value columns", got)
	}
	if got := aapl.RawGetString("column_kinds").Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("aapl.column_kinds.sym = %v, want nil", got)
	}
	miss := interp.GetGlobal("miss").Table()
	if miss == nil || miss.Length() != 0 {
		t.Fatalf("miss len = %v, want 0", miss)
	}
	assertPCallErrorContains(t, interp, "bad_arity", "lookup key has 2 values, want 1")
}

func TestQKeyedLookupMultiKeyBoundaries(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"AAPL\", \"MSFT\"}),\n"+
			"    venue: data.symbols({\"XNYS\", \"XNYS\", \"XNAS\", \"XNAS\"}),\n"+
			"    trade_id: array.i64({1, 2, 3, 4}),\n"+
			"    price: data.f64({100.0, 100.5, 101.0, 80.0}),\n"+
			"})\n"+
			"keyed := q.key_by(trades, \"sym\", \"venue\")\n"+
			"reversed_keyed := q.key_by(trades, \"venue\", \"sym\")\n"+
			"aapl_xnys := q.lookup(keyed, \"AAPL\", \"XNYS\")\n"+
			"aapl_xnas_by_dict := q.lookup(keyed, {sym: \"AAPL\", venue: \"XNAS\"})\n"+
			"aapl_xnas_by_dict_extra := q.lookup(keyed, {venue: \"XNAS\", sym: \"AAPL\", ignored: 1})\n"+
			"aapl_xnas_by_reversed_keys := q.lookup(reversed_keyed, {sym: \"AAPL\", venue: \"XNAS\"})\n"+
			"aapl_xnas_by_reversed_positional := q.lookup(reversed_keyed, \"XNAS\", \"AAPL\")\n"+
			"aapl_xnas_by_array := q.lookup(keyed, {\"AAPL\", \"XNAS\"})\n"+
			"missing := q.lookup(keyed, \"IBM\", \"XNYS\")\n"+
			"missing_reversed := q.lookup(reversed_keyed, \"XNYS\", \"IBM\")\n"+
			"missing_key_ok, missing_key_err := pcall(func() {\n"+
			"    return q.lookup(keyed, {sym: \"AAPL\"})\n"+
			"})\n"+
			"key_columns := q.keys(keyed)\n"+
			"reversed_key_columns := q.keys(reversed_keyed)\n"+
			"key_frame := q.key(keyed)\n"+
			"reversed_key_frame := q.key(reversed_keyed)\n"+
			"plain_keys := q.keys(trades)\n"+
			"dict_values := q.value({b: 2, a: 1})\n"+
			"qdict := q.eval(\"`b`a!2 1\")\n"+
			"qdict_keys := q.keys(qdict)\n"+
			"qdict_values := q.value(qdict)\n"+
			"value_frame := q.value(keyed)\n"+
			"columns := q.cols(keyed)\n"+
			"metadata := q.meta(keyed)\n"+
			"queried_keyed := q.sql(keyed, \"select trade_id,sym,venue,price from trades where sym=`AAPL order by trade_id asc\")\n"+
			"frame := keyed.frame\n")

	aapl := interp.GetGlobal("aapl_xnys").Table()
	if aapl == nil || aapl.Length() != 1 {
		t.Fatalf("aapl_xnys len = %v, want 1", aapl)
	}
	if got := aapl.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("aapl_xnys[1].trade_id = %v, want latest row 2", got)
	}
	if got := aapl.RawGetInt(1).Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("aapl_xnys[1].sym = %v, want nil because keyed lookup returns value columns", got)
	}
	if got := aapl.RawGetInt(1).Table().RawGetString("venue"); !got.IsNil() {
		t.Fatalf("aapl_xnys[1].venue = %v, want nil because keyed lookup returns value columns", got)
	}
	if got := aapl.RawGetString("column_kinds").Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("aapl_xnys.column_kinds.sym = %v, want nil", got)
	}
	if got := aapl.RawGetString("column_kinds").Table().RawGetString("venue"); !got.IsNil() {
		t.Fatalf("aapl_xnys.column_kinds.venue = %v, want nil", got)
	}
	aaplByDict := interp.GetGlobal("aapl_xnas_by_dict").Table()
	if aaplByDict == nil || aaplByDict.Length() != 1 {
		t.Fatalf("aapl_xnas_by_dict len = %v, want 1", aaplByDict)
	}
	if got := aaplByDict.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("aapl_xnas_by_dict[1].trade_id = %v, want 3", got)
	}
	aaplByDictExtra := interp.GetGlobal("aapl_xnas_by_dict_extra").Table()
	if aaplByDictExtra == nil || aaplByDictExtra.Length() != 1 {
		t.Fatalf("aapl_xnas_by_dict_extra len = %v, want 1", aaplByDictExtra)
	}
	if got := aaplByDictExtra.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("aapl_xnas_by_dict_extra[1].trade_id = %v, want 3", got)
	}
	aaplByReversedKeys := interp.GetGlobal("aapl_xnas_by_reversed_keys").Table()
	if aaplByReversedKeys == nil || aaplByReversedKeys.Length() != 1 {
		t.Fatalf("aapl_xnas_by_reversed_keys len = %v, want 1", aaplByReversedKeys)
	}
	if got := aaplByReversedKeys.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("aapl_xnas_by_reversed_keys[1].trade_id = %v, want 3", got)
	}
	aaplByReversedPositional := interp.GetGlobal("aapl_xnas_by_reversed_positional").Table()
	if aaplByReversedPositional == nil || aaplByReversedPositional.Length() != 1 {
		t.Fatalf("aapl_xnas_by_reversed_positional len = %v, want 1", aaplByReversedPositional)
	}
	if got := aaplByReversedPositional.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("aapl_xnas_by_reversed_positional[1].trade_id = %v, want 3", got)
	}
	aaplByArray := interp.GetGlobal("aapl_xnas_by_array").Table()
	if aaplByArray == nil || aaplByArray.Length() != 1 {
		t.Fatalf("aapl_xnas_by_array len = %v, want 1", aaplByArray)
	}
	if got := aaplByArray.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("aapl_xnas_by_array[1].trade_id = %v, want 3", got)
	}
	assertPCallErrorContains(t, interp, "missing_key", "key \"venue\" is missing")
	dictValues := interp.GetGlobal("dict_values").Table()
	if dictValues == nil || dictValues.Length() != 2 {
		t.Fatalf("dict_values len = %v, want 2", dictValues)
	}
	if got := dictValues.RawGetInt(1); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("dict_values[1] = %v, want value for sorted key a", got)
	}
	if got := dictValues.RawGetInt(2); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("dict_values[2] = %v, want value for sorted key b", got)
	}
	qdictKeys := interp.GetGlobal("qdict_keys").Table()
	if keys := qTestArrayStrings(t, qdictKeys); len(keys) != 2 || keys[0] != "b" || keys[1] != "a" {
		t.Fatalf("qdict_keys = %v, want source order [b a]", keys)
	}
	qdictValues := interp.GetGlobal("qdict_values").Table()
	if qdictValues == nil || qdictValues.Length() != 2 {
		t.Fatalf("qdict_values len = %v, want 2", qdictValues)
	}
	if got := qdictValues.RawGetInt(1); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("qdict_values[1] = %v, want value for key b", got)
	}
	if got := qdictValues.RawGetInt(2); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("qdict_values[2] = %v, want value for key a", got)
	}

	missing := interp.GetGlobal("missing").Table()
	if missing == nil || missing.Length() != 0 {
		t.Fatalf("missing len = %v, want 0", missing)
	}
	if got := missing.RawGetString("column_kinds").Table().RawGetString("trade_id"); !got.IsString() || got.Str() != "i64" {
		t.Fatalf("missing.column_kinds.trade_id = %v, want i64", got)
	}
	if got := missing.RawGetString("column_kinds").Table().RawGetString("price"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("missing.column_kinds.price = %v, want f64", got)
	}
	if got := missing.RawGetString("column_kinds").Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("missing.column_kinds.sym = %v, want nil because missing lookup returns value columns", got)
	}
	missingReversed := interp.GetGlobal("missing_reversed").Table()
	if missingReversed == nil || missingReversed.Length() != 0 {
		t.Fatalf("missing_reversed len = %v, want 0", missingReversed)
	}
	if got := missingReversed.RawGetString("column_kinds").Table().RawGetString("price"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("missing_reversed.column_kinds.price = %v, want f64", got)
	}
	keyColumns := interp.GetGlobal("key_columns").Table()
	if keyColumns == nil || keyColumns.Length() != 2 {
		t.Fatalf("key_columns len = %v, want 2", keyColumns)
	}
	if got := keyColumns.RawGetInt(1); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("key_columns[1] = %v, want sym", got)
	}
	if got := keyColumns.RawGetInt(2); !got.IsString() || got.Str() != "venue" {
		t.Fatalf("key_columns[2] = %v, want venue", got)
	}
	reversedKeyColumns := interp.GetGlobal("reversed_key_columns").Table()
	if got := reversedKeyColumns.RawGetInt(1); !got.IsString() || got.Str() != "venue" {
		t.Fatalf("reversed_key_columns[1] = %v, want venue", got)
	}
	if got := reversedKeyColumns.RawGetInt(2); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("reversed_key_columns[2] = %v, want sym", got)
	}
	keyFrame := interp.GetGlobal("key_frame").Table()
	if keyFrame == nil || keyFrame.Length() != 4 {
		t.Fatalf("key_frame len = %v, want 4", keyFrame)
	}
	if got := keyFrame.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("key_frame[1].sym = %v, want AAPL", got)
	}
	if got := keyFrame.RawGetInt(3).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNAS" {
		t.Fatalf("key_frame[3].venue = %v, want XNAS", got)
	}
	if got := keyFrame.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsNil() {
		t.Fatalf("key_frame[1].trade_id = %v, want nil because q.key returns key columns", got)
	}
	reversedKeyFrame := interp.GetGlobal("reversed_key_frame").Table()
	if reversedKeyFrame == nil || reversedKeyFrame.Length() != 4 {
		t.Fatalf("reversed_key_frame len = %v, want 4", reversedKeyFrame)
	}
	if got := reversedKeyFrame.RawGetInt(1).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNYS" {
		t.Fatalf("reversed_key_frame[1].venue = %v, want XNYS", got)
	}
	if got := reversedKeyFrame.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("reversed_key_frame[1].sym = %v, want AAPL", got)
	}
	if got := reversedKeyFrame.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsNil() {
		t.Fatalf("reversed_key_frame[1].trade_id = %v, want nil because q.key returns key columns", got)
	}
	plainKeys := interp.GetGlobal("plain_keys").Table()
	if plainKeys == nil || plainKeys.Length() != 0 {
		t.Fatalf("plain_keys len = %v, want 0", plainKeys)
	}
	valueFrame := interp.GetGlobal("value_frame").Table()
	if valueFrame == nil || valueFrame.Length() != 4 {
		t.Fatalf("value_frame len = %v, want 4", valueFrame)
	}
	if got := valueFrame.RawGetInt(2).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("value_frame[2].trade_id = %v, want 2", got)
	}
	if got := valueFrame.RawGetInt(2).Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("value_frame[2].sym = %v, want nil because q.value returns value columns", got)
	}
	columns := interp.GetGlobal("columns").Table()
	if columns == nil || columns.Length() != 4 {
		t.Fatalf("columns len = %v, want 4", columns)
	}
	if got := columns.RawGetInt(1); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("columns[1] = %v, want sym", got)
	}
	if got := columns.RawGetInt(2); !got.IsString() || got.Str() != "venue" {
		t.Fatalf("columns[2] = %v, want venue", got)
	}
	if got := columns.RawGetInt(3); !got.IsString() || got.Str() != "trade_id" {
		t.Fatalf("columns[3] = %v, want trade_id", got)
	}
	if got := columns.RawGetInt(4); !got.IsString() || got.Str() != "price" {
		t.Fatalf("columns[4] = %v, want price", got)
	}
	metadata := interp.GetGlobal("metadata").Table()
	if metadata == nil || metadata.Length() != 4 {
		t.Fatalf("metadata len = %v, want 4", metadata)
	}
	if got := metadata.RawGetInt(1).Table().RawGetString("c"); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("metadata[1].c = %v, want sym", got)
	}
	if got := metadata.RawGetInt(1).Table().RawGetString("t"); !got.IsString() || got.Str() != "symbol" {
		t.Fatalf("metadata[1].t = %v, want symbol", got)
	}
	if got := metadata.RawGetInt(2).Table().RawGetString("c"); !got.IsString() || got.Str() != "venue" {
		t.Fatalf("metadata[2].c = %v, want venue", got)
	}
	if got := metadata.RawGetInt(2).Table().RawGetString("t"); !got.IsString() || got.Str() != "symbol" {
		t.Fatalf("metadata[2].t = %v, want symbol", got)
	}
	if got := metadata.RawGetInt(3).Table().RawGetString("c"); !got.IsString() || got.Str() != "trade_id" {
		t.Fatalf("metadata[3].c = %v, want trade_id", got)
	}
	if got := metadata.RawGetInt(4).Table().RawGetString("c"); !got.IsString() || got.Str() != "price" {
		t.Fatalf("metadata[4].c = %v, want price", got)
	}
	queriedKeyed := interp.GetGlobal("queried_keyed").Table()
	if queriedKeyed == nil || queriedKeyed.Length() != 3 {
		t.Fatalf("queried_keyed len = %v, want 3", queriedKeyed)
	}
	if got := queriedKeyed.RawGetInt(2).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("queried_keyed[2].trade_id = %v, want 2", got)
	}
	frame := interp.GetGlobal("frame").Table()
	if frame == nil || frame.Length() != 4 {
		t.Fatalf("keyed.frame len = %v, want 4", frame)
	}
	if got := frame.RawGetInt(3).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNAS" {
		t.Fatalf("keyed.frame[3].venue = %v, want XNAS", got)
	}
}

func TestQKeyedLookupKeyValueKDBSubsetSemantics(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"bars := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"MSFT\"}),\n"+
			"    bucket: data.strings({\"09:30\", \"09:30\", \"09:31\"}),\n"+
			"    seq: array.i64({1, 2, 3}),\n"+
			"    px: data.f64({100.0, 100.5, 80.0}),\n"+
			"})\n"+
			"keyed := q.key_by(bars, \"sym\", \"bucket\")\n"+
			"key_rows := q.key(keyed)\n"+
			"value_rows := q.value(keyed)\n"+
			"latest := q.lookup(keyed, \"AAPL\", \"09:30\")\n"+
			"latest_by_key_row := q.lookup(keyed, key_rows[1])\n"+
			"missing := q.lookup(keyed, \"AAPL\", \"09:32\")\n"+
			"all_duplicate_rows := q.sql(keyed, \"select sym,bucket,seq,px from bars where sym=`AAPL order by seq asc\")\n")

	keyRows := interp.GetGlobal("key_rows").Table()
	if keyRows == nil || keyRows.Length() != 3 {
		t.Fatalf("key_rows len = %v, want 3", keyRows)
	}
	if got := keyRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("key_rows[1].sym = %v, want AAPL", got)
	}
	if got := keyRows.RawGetInt(1).Table().RawGetString("seq"); !got.IsNil() {
		t.Fatalf("key_rows[1].seq = %v, want nil", got)
	}

	valueRows := interp.GetGlobal("value_rows").Table()
	if valueRows == nil || valueRows.Length() != 3 {
		t.Fatalf("value_rows len = %v, want 3", valueRows)
	}
	if got := valueRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("value_rows[1].sym = %v, want nil", got)
	}
	if got := valueRows.RawGetInt(2).Table().RawGetString("seq"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("value_rows[2].seq = %v, want 2", got)
	}

	latest := interp.GetGlobal("latest").Table()
	if latest == nil || latest.Length() != 1 {
		t.Fatalf("latest len = %v, want 1", latest)
	}
	if got := latest.RawGetInt(1).Table().RawGetString("seq"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("latest[1].seq = %v, want latest duplicate row 2", got)
	}
	if got := latest.RawGetInt(1).Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("latest[1].sym = %v, want nil because q.lookup returns value columns", got)
	}

	latestByKeyRow := interp.GetGlobal("latest_by_key_row").Table()
	if latestByKeyRow == nil || latestByKeyRow.Length() != 1 {
		t.Fatalf("latest_by_key_row len = %v, want 1", latestByKeyRow)
	}
	if got := latestByKeyRow.RawGetInt(1).Table().RawGetString("px"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("latest_by_key_row[1].px = %v, want 100.5", got)
	}

	missing := interp.GetGlobal("missing").Table()
	if missing == nil || missing.Length() != 0 {
		t.Fatalf("missing len = %v, want 0", missing)
	}
	if got := missing.RawGetString("column_kinds").Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("missing.column_kinds.sym = %v, want nil", got)
	}
	if got := missing.RawGetString("column_kinds").Table().RawGetString("px"); !got.IsString() || got.Str() != "f64" {
		t.Fatalf("missing.column_kinds.px = %v, want f64", got)
	}

	allDuplicateRows := interp.GetGlobal("all_duplicate_rows").Table()
	if allDuplicateRows == nil || allDuplicateRows.Length() != 2 {
		t.Fatalf("all_duplicate_rows len = %v, want 2", allDuplicateRows)
	}
	if got := allDuplicateRows.RawGetInt(1).Table().RawGetString("seq"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("all_duplicate_rows[1].seq = %v, want 1", got)
	}
}

func TestQNestedDataArraysAreScriptVisible(t *testing.T) {
	interp := runWithQAndSOA(t,
		"xgrouped := q.eval(\"`sym xgroup flip `sym`price`size!(`AAPL`MSFT`AAPL;100 101 102;10 20 30)\")\n"+
			"hit := q.lookup(xgrouped, \"AAPL\")\n"+
			"prices := hit[1].price\n"+
			"prices_len := #prices\n"+
			"prices_count := q.count(prices)\n"+
			"prices_first := prices[1]\n"+
			"prices_second := prices[2]\n"+
			"prices_kind := prices.kind\n"+
			"prices_text := tostring(prices)\n"+
			"nested := q.eval(\"(1 2;3 4 5;6)\")\n"+
			"nested_len := #nested\n"+
			"nested_second := nested[2]\n"+
			"nested_second_len := #nested_second\n"+
			"nested_second_third := nested_second[3]\n"+
			"nested_second_text := tostring(nested_second)\n")

	if got := interp.GetGlobal("prices_len"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("prices_len = %v, want 2", got)
	}
	if got := interp.GetGlobal("prices_count"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("prices_count = %v, want 2", got)
	}
	if got := interp.GetGlobal("prices_first"); !got.IsInt() || got.Int() != 100 {
		t.Fatalf("prices_first = %v, want 100", got)
	}
	if got := interp.GetGlobal("prices_second"); !got.IsInt() || got.Int() != 102 {
		t.Fatalf("prices_second = %v, want 102", got)
	}
	if got := interp.GetGlobal("prices_kind"); !got.IsString() || got.Str() != "i64" {
		t.Fatalf("prices_kind = %v, want i64", got)
	}
	if got := interp.GetGlobal("prices_text"); !got.IsString() || !strings.Contains(got.Str(), "100") || !strings.Contains(got.Str(), "102") {
		t.Fatalf("prices_text = %v, want inspectable grouped array", got)
	}
	if got := interp.GetGlobal("nested_len"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("nested_len = %v, want 3", got)
	}
	if got := interp.GetGlobal("nested_second_len"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("nested_second_len = %v, want 3", got)
	}
	if got := interp.GetGlobal("nested_second_third"); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("nested_second_third = %v, want 5", got)
	}
	if got := interp.GetGlobal("nested_second_text"); !got.IsString() || !strings.Contains(got.Str(), "3") || !strings.Contains(got.Str(), "5") {
		t.Fatalf("nested_second_text = %v, want inspectable nested array", got)
	}
}

func TestQSQLKeyedMutationPreservesWrapper(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"MSFT\"}),\n"+
			"    venue: data.symbols({\"XNYS\", \"XNAS\", \"XNYS\"}),\n"+
			"    trade_id: array.i64({1, 2, 3}),\n"+
			"    price: data.f64({100.0, 101.0, 80.0}),\n"+
			"})\n"+
			"keyed := q.key_by(trades, \"sym\", \"venue\")\n"+
			"updated := q.sql(keyed, \"update price:price+1 from trades where sym=`AAPL\")\n"+
			"updated_lookup := q.lookup(updated, \"AAPL\", \"XNYS\")\n"+
			"deleted := q.sql(updated, \"delete from trades where venue=`XNAS\")\n"+
			"selected := q.sql(updated, \"select trade_id,sym,venue,price from trades where sym=`AAPL order by trade_id asc\")\n"+
			"map_updated := q.sql(\"update price:price+2 from trades where venue=`XNYS\", {trades: keyed})\n"+
			"drop_key_ok, drop_key_err := pcall(func() {\n"+
			"    return q.sql(keyed, \"delete sym from trades\")\n"+
			"})\n")

	updated := interp.GetGlobal("updated").Table()
	if updated == nil {
		t.Fatalf("updated keyed result is nil")
	}
	if keys := qTestArrayStrings(t, updated.RawGetString("keys").Table()); len(keys) != 2 || keys[0] != "sym" || keys[1] != "venue" {
		t.Fatalf("updated.keys = %v, want [sym venue]", keys)
	}
	updatedFrame := updated.RawGetString("frame").Table()
	if updatedFrame == nil || updatedFrame.Length() != 3 {
		t.Fatalf("updated.frame len = %v, want 3", updatedFrame)
	}
	if got := updatedFrame.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("updated.frame[1].price = %v, want 101", got)
	}
	if got := updatedFrame.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 102 {
		t.Fatalf("updated.frame[2].price = %v, want 102", got)
	}
	updatedLookup := interp.GetGlobal("updated_lookup").Table()
	if updatedLookup == nil || updatedLookup.Length() != 1 {
		t.Fatalf("updated_lookup len = %v, want 1", updatedLookup)
	}
	if got := updatedLookup.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("updated_lookup[1].trade_id = %v, want 1", got)
	}
	if got := updatedLookup.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("updated_lookup[1].price = %v, want 101", got)
	}

	deleted := interp.GetGlobal("deleted").Table()
	if deleted == nil {
		t.Fatalf("deleted keyed result is nil")
	}
	if keys := qTestArrayStrings(t, deleted.RawGetString("keys").Table()); len(keys) != 2 || keys[0] != "sym" || keys[1] != "venue" {
		t.Fatalf("deleted.keys = %v, want [sym venue]", keys)
	}
	deletedFrame := deleted.RawGetString("frame").Table()
	if deletedFrame == nil || deletedFrame.Length() != 2 {
		t.Fatalf("deleted.frame len = %v, want 2", deletedFrame)
	}
	if got := deletedFrame.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("deleted.frame[2].sym = %v, want MSFT", got)
	}

	selected := interp.GetGlobal("selected").Table()
	if selected == nil || selected.Length() != 2 {
		t.Fatalf("selected len = %v, want 2", selected)
	}
	if got := selected.RawGetString("keys"); !got.IsNil() {
		t.Fatalf("selected.keys = %v, want nil because select returns a plain frame", got)
	}
	if got := selected.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 102 {
		t.Fatalf("selected[2].price = %v, want 102", got)
	}

	mapUpdated := interp.GetGlobal("map_updated").Table()
	if mapUpdated == nil {
		t.Fatalf("map_updated keyed result is nil")
	}
	if keys := qTestArrayStrings(t, mapUpdated.RawGetString("keys").Table()); len(keys) != 2 || keys[0] != "sym" || keys[1] != "venue" {
		t.Fatalf("map_updated.keys = %v, want [sym venue]", keys)
	}
	mapFrame := mapUpdated.RawGetString("frame").Table()
	if mapFrame == nil || mapFrame.Length() != 3 {
		t.Fatalf("map_updated.frame len = %v, want 3", mapFrame)
	}
	if got := mapFrame.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 102 {
		t.Fatalf("map_updated.frame[1].price = %v, want 102", got)
	}
	if got := mapFrame.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("map_updated.frame[2].price = %v, want unchanged 101", got)
	}
	assertPCallErrorContains(t, interp, "drop_key", "keyed mutation")
}

func qTestArrayStrings(t *testing.T, tbl *Table) []string {
	t.Helper()
	if tbl == nil {
		return nil
	}
	out := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		v := tbl.RawGetInt(int64(i))
		if !v.IsString() {
			t.Fatalf("array item %d = %v, want string", i, v)
		}
		out = append(out, v.Str())
	}
	return out
}

func qSQLResetPlanCachesForTest() {
	qClearCaches()
}

func qTestCacheStatsRows(t *testing.T, tbl *Table) map[string]map[string]int64 {
	t.Helper()
	if tbl == nil {
		t.Fatal("cache stats table is nil")
	}
	out := make(map[string]map[string]int64, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("cache stats row %d is nil", i)
		}
		name := row.RawGetString("cache")
		if !name.IsString() {
			t.Fatalf("cache stats row %d cache = %v, want string", i, name)
		}
		values := make(map[string]int64, 5)
		for _, field := range []string{"entries", "hits", "misses", "evictions", "limit"} {
			v := row.RawGetString(field)
			if !v.IsInt() {
				t.Fatalf("cache stats row %s.%s = %v, want int", name.Str(), field, v)
			}
			values[field] = v.Int()
		}
		out[name.Str()] = values
	}
	for _, name := range []string{"qsql_template", "qsql_aligned", "qsql_kernel", "qsql_kernel_decision", "q_query_kernel", "q_runtime_kernel_execution", "q_eval"} {
		if _, ok := out[name]; !ok {
			t.Fatalf("cache stats missing row %q in %#v", name, out)
		}
	}
	return out
}

func qTestCacheStatsRowTable(t *testing.T, tbl *Table, cache string) *Table {
	t.Helper()
	if tbl == nil {
		t.Fatal("cache stats table is nil")
	}
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("cache stats row %d is nil", i)
		}
		name := row.RawGetString("cache")
		if name.IsString() && name.Str() == cache {
			return row
		}
	}
	t.Fatalf("cache stats missing row %q", cache)
	return nil
}

type qQueryKernelShapeRow struct {
	Supported    bool
	ReasonFamily string
	ReasonCode   string
	SchemaHash   string
	Shape        string
	Count        int64
}

type qQueryKernelKeyRow struct {
	Key             string
	Namespace       string
	Kind            string
	PlanFingerprint string
	Supported       bool
	ReasonFamily    string
	ReasonCode      string
	SchemaHash      string
	Shape           string
	Hits            int64
	Misses          int64
	Evictions       int64
}

func qTestQueryKernelKeyRows(t *testing.T, tbl *Table) []qQueryKernelKeyRow {
	t.Helper()
	if tbl == nil {
		t.Fatal("q_query_kernel keys table is nil")
	}
	rows := make([]qQueryKernelKeyRow, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("q_query_kernel key row %d is nil", i)
		}
		key := row.RawGetString("key")
		namespace := row.RawGetString("namespace")
		kind := row.RawGetString("kind")
		planFingerprint := row.RawGetString("plan_fingerprint")
		supported := row.RawGetString("supported")
		reasonFamily := row.RawGetString("reason_family")
		reasonCode := row.RawGetString("reason_code")
		schemaHash := row.RawGetString("schema_hash")
		shape := row.RawGetString("shape")
		hits := row.RawGetString("hits")
		misses := row.RawGetString("misses")
		evictions := row.RawGetString("evictions")
		if !key.IsString() || !namespace.IsString() || !kind.IsString() || !planFingerprint.IsString() || !supported.IsBool() || !reasonFamily.IsString() || !reasonCode.IsString() || !schemaHash.IsString() || !shape.IsString() || !hits.IsInt() || !misses.IsInt() || !evictions.IsInt() {
			t.Fatalf("q_query_kernel key row %d malformed: %#v", i, row)
		}
		rows = append(rows, qQueryKernelKeyRow{
			Key:             key.Str(),
			Namespace:       namespace.Str(),
			Kind:            kind.Str(),
			PlanFingerprint: planFingerprint.Str(),
			Supported:       supported.Bool(),
			ReasonFamily:    reasonFamily.Str(),
			ReasonCode:      reasonCode.Str(),
			SchemaHash:      schemaHash.Str(),
			Shape:           shape.Str(),
			Hits:            hits.Int(),
			Misses:          misses.Int(),
			Evictions:       evictions.Int(),
		})
	}
	return rows
}

func qTestQueryKernelSupportKeyJSONRows(t *testing.T) []QQueryKernelSupportKeyStatJSONRow {
	t.Helper()
	qQueryKernelSupportCacheMu.Lock()
	rows := qQueryKernelSupportKeyStatsSnapshotLocked()
	qQueryKernelSupportCacheMu.Unlock()
	return QQueryKernelSupportKeyStatJSONRows(rows)
}

func qTestQueryKernelShapeRows(t *testing.T, tbl *Table) []qQueryKernelShapeRow {
	t.Helper()
	if tbl == nil {
		t.Fatal("q_query_kernel shapes table is nil")
	}
	rows := make([]qQueryKernelShapeRow, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("q_query_kernel shape row %d is nil", i)
		}
		supported := row.RawGetString("supported")
		reasonFamily := row.RawGetString("reason_family")
		reasonCode := row.RawGetString("reason_code")
		schemaHash := row.RawGetString("schema_hash")
		shape := row.RawGetString("shape")
		count := row.RawGetString("count")
		if !supported.IsBool() || !reasonFamily.IsString() || !reasonCode.IsString() || !schemaHash.IsString() || !shape.IsString() || !count.IsInt() {
			t.Fatalf("q_query_kernel shape row %d malformed: %#v", i, row)
		}
		rows = append(rows, qQueryKernelShapeRow{
			Supported:    supported.Bool(),
			ReasonFamily: reasonFamily.Str(),
			ReasonCode:   reasonCode.Str(),
			SchemaHash:   schemaHash.Str(),
			Shape:        shape.Str(),
			Count:        count.Int(),
		})
	}
	return rows
}

func qTestQueryKernelShapeCount(rows []qQueryKernelShapeRow, supported bool, reasonFamily, reasonCode, schemaHash string) int64 {
	for _, row := range rows {
		if row.Supported == supported && row.ReasonFamily == reasonFamily && row.ReasonCode == reasonCode && row.SchemaHash == schemaHash {
			return row.Count
		}
	}
	return 0
}

type qKernelShapeRow struct {
	Shape      string
	SchemaHash string
	Count      int64
	Hits       int64
	Misses     int64
	Evictions  int64
}

func qTestKernelShapeRows(t *testing.T, tbl *Table) []qKernelShapeRow {
	t.Helper()
	if tbl == nil {
		t.Fatal("qsql_kernel shapes table is nil")
	}
	rows := make([]qKernelShapeRow, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("qsql_kernel shape row %d is nil", i)
		}
		shape := row.RawGetString("shape")
		schemaHash := row.RawGetString("schema_hash")
		count := row.RawGetString("count")
		hits := row.RawGetString("hits")
		misses := row.RawGetString("misses")
		evictions := row.RawGetString("evictions")
		if !shape.IsString() || !schemaHash.IsString() || !count.IsInt() || !hits.IsInt() || !misses.IsInt() || !evictions.IsInt() {
			t.Fatalf("qsql_kernel shape row %d malformed: %#v", i, row)
		}
		rows = append(rows, qKernelShapeRow{
			Shape:      shape.Str(),
			SchemaHash: schemaHash.Str(),
			Count:      count.Int(),
			Hits:       hits.Int(),
			Misses:     misses.Int(),
			Evictions:  evictions.Int(),
		})
	}
	return rows
}

type qKernelDecisionKeyRow struct {
	Key             string
	Namespace       string
	Kind            string
	SchemaHash      string
	PlanFingerprint string
	Shape           string
	ReasonFamily    string
	ReasonCode      string
	Count           int64
}

func qTestKernelDecisionKeyRows(t *testing.T, tbl *Table) []qKernelDecisionKeyRow {
	t.Helper()
	if tbl == nil {
		t.Fatal("qsql_kernel_decision keys table is nil")
	}
	rows := make([]qKernelDecisionKeyRow, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("qsql_kernel_decision key row %d is nil", i)
		}
		key := row.RawGetString("key")
		namespace := row.RawGetString("namespace")
		kind := row.RawGetString("kind")
		schemaHash := row.RawGetString("schema_hash")
		planFingerprint := row.RawGetString("plan_fingerprint")
		shape := row.RawGetString("shape")
		reasonFamily := row.RawGetString("reason_family")
		reasonCode := row.RawGetString("reason_code")
		count := row.RawGetString("count")
		if !key.IsString() || !namespace.IsString() || !kind.IsString() || !schemaHash.IsString() || !planFingerprint.IsString() || !shape.IsString() || !reasonFamily.IsString() || !reasonCode.IsString() || !count.IsInt() {
			t.Fatalf("qsql_kernel_decision key row %d malformed: %#v", i, row)
		}
		rows = append(rows, qKernelDecisionKeyRow{
			Key:             key.Str(),
			Namespace:       namespace.Str(),
			Kind:            kind.Str(),
			SchemaHash:      schemaHash.Str(),
			PlanFingerprint: planFingerprint.Str(),
			Shape:           shape.Str(),
			ReasonFamily:    reasonFamily.Str(),
			ReasonCode:      reasonCode.Str(),
			Count:           count.Int(),
		})
	}
	return rows
}

type qKernelDecisionShapeRow struct {
	ReasonFamily string
	ReasonCode   string
	SchemaHash   string
	Shape        string
	Count        int64
}

func qTestKernelDecisionShapeRows(t *testing.T, tbl *Table) []qKernelDecisionShapeRow {
	t.Helper()
	if tbl == nil {
		t.Fatal("qsql_kernel_decision shapes table is nil")
	}
	rows := make([]qKernelDecisionShapeRow, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("qsql_kernel_decision shape row %d is nil", i)
		}
		reasonFamily := row.RawGetString("reason_family")
		reasonCode := row.RawGetString("reason_code")
		schemaHash := row.RawGetString("schema_hash")
		shape := row.RawGetString("shape")
		count := row.RawGetString("count")
		if !reasonFamily.IsString() || !reasonCode.IsString() || !schemaHash.IsString() || !shape.IsString() || !count.IsInt() {
			t.Fatalf("qsql_kernel_decision shape row %d malformed: %#v", i, row)
		}
		rows = append(rows, qKernelDecisionShapeRow{
			ReasonFamily: reasonFamily.Str(),
			ReasonCode:   reasonCode.Str(),
			SchemaHash:   schemaHash.Str(),
			Shape:        shape.Str(),
			Count:        count.Int(),
		})
	}
	return rows
}

func qTestKernelDecisionShapeCount(rows []qKernelDecisionShapeRow, reasonFamily, reasonCode, schemaHash, shape string) int64 {
	for _, row := range rows {
		if row.ReasonFamily == reasonFamily && row.ReasonCode == reasonCode && row.SchemaHash == schemaHash && row.Shape == shape {
			return row.Count
		}
	}
	return 0
}

type qKernelDecisionReasonRow struct {
	ReasonFamily string
	ReasonCode   string
	Count        int64
}

func qTestKernelDecisionReasonRows(t *testing.T, tbl *Table) []qKernelDecisionReasonRow {
	t.Helper()
	if tbl == nil {
		t.Fatal("qsql_kernel_decision reasons table is nil")
	}
	rows := make([]qKernelDecisionReasonRow, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("qsql_kernel_decision reason row %d is nil", i)
		}
		reasonFamily := row.RawGetString("reason_family")
		reasonCode := row.RawGetString("reason_code")
		count := row.RawGetString("count")
		if !reasonFamily.IsString() || !reasonCode.IsString() || !count.IsInt() {
			t.Fatalf("qsql_kernel_decision reason row %d malformed: %#v", i, row)
		}
		rows = append(rows, qKernelDecisionReasonRow{
			ReasonFamily: reasonFamily.Str(),
			ReasonCode:   reasonCode.Str(),
			Count:        count.Int(),
		})
	}
	return rows
}

func qTestKernelDecisionReasonCount(rows []qKernelDecisionReasonRow, reasonFamily, reasonCode string) int64 {
	for _, row := range rows {
		if row.ReasonFamily == reasonFamily && row.ReasonCode == reasonCode {
			return row.Count
		}
	}
	return 0
}

func qTestKernelCacheKeyStats(t *testing.T, stats []qSQLKernelCacheKeyStats, key string) qSQLKernelCacheKeyStats {
	t.Helper()
	for _, stat := range stats {
		if stat.Key == key {
			return stat
		}
	}
	t.Fatalf("kernel cache key stats missing key %q in %+v", key, stats)
	return qSQLKernelCacheKeyStats{}
}

func qTestFallbackStatsRows(t *testing.T, tbl *Table) map[string]int64 {
	t.Helper()
	if tbl == nil {
		t.Fatal("fallback stats table is nil")
	}
	out := make(map[string]int64, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("fallback stats row %d is nil", i)
		}
		code := row.RawGetString("code")
		if !code.IsString() {
			t.Fatalf("fallback stats row %d code = %v, want string", i, code)
		}
		count := row.RawGetString("count")
		if !count.IsInt() {
			t.Fatalf("fallback stats row %s count = %v, want int", code.Str(), count)
		}
		kind := row.RawGetString("kind")
		if kind.IsString() && kind.Str() != "code" {
			continue
		}
		out[code.Str()] = count.Int()
	}
	for _, code := range []string{
		qFallbackKernelUnsupported,
		qFallbackKernelCompileErr,
		qFallbackSourceErr,
		qFallbackJoinErr,
		qFallbackMutationPlan,
		qQueryKernelSupported,
		qFallbackQueryKernel,
	} {
		if _, ok := out[code]; !ok {
			t.Fatalf("fallback stats missing code %q in %#v", code, out)
		}
	}
	return out
}

type qFallbackStatsDetailRow struct {
	Kind         string
	Code         string
	ReasonFamily string
	ReasonCode   string
	Reason       string
	Source       string
	SchemaHash   string
	Shape        string
	Count        int64
}

func qTestFallbackStatsDetailRows(t *testing.T, tbl *Table) []qFallbackStatsDetailRow {
	t.Helper()
	if tbl == nil {
		t.Fatal("fallback stats table is nil")
	}
	var out []qFallbackStatsDetailRow
	for i := 1; i <= tbl.Length(); i++ {
		row := tbl.RawGetInt(int64(i)).Table()
		if row == nil {
			t.Fatalf("fallback stats row %d is nil", i)
		}
		kind := row.RawGetString("kind")
		if !kind.IsString() || kind.Str() == "code" {
			continue
		}
		code := row.RawGetString("code")
		reasonFamily := row.RawGetString("reason_family")
		reasonCode := row.RawGetString("reason_code")
		reason := row.RawGetString("reason")
		source := row.RawGetString("source")
		schemaHash := row.RawGetString("schema_hash")
		shape := row.RawGetString("shape")
		count := row.RawGetString("count")
		if !code.IsString() || !reasonFamily.IsString() || !reasonCode.IsString() || !reason.IsString() || !source.IsString() || !schemaHash.IsString() || !shape.IsString() || !count.IsInt() {
			t.Fatalf("fallback stats detail row %d malformed: %#v", i, row)
		}
		out = append(out, qFallbackStatsDetailRow{
			Kind:         kind.Str(),
			Code:         code.Str(),
			ReasonFamily: reasonFamily.Str(),
			ReasonCode:   reasonCode.Str(),
			Reason:       reason.Str(),
			Source:       source.Str(),
			SchemaHash:   schemaHash.Str(),
			Shape:        shape.Str(),
			Count:        count.Int(),
		})
	}
	return out
}

func qTestFallbackDetailCount(rows []qFallbackStatsDetailRow, kind, code, reasonCode, reason string) int64 {
	for _, row := range rows {
		if row.Kind == kind && row.Code == code && row.ReasonCode == reasonCode && row.Reason == reason {
			return row.Count
		}
	}
	return 0
}

func qTestFallbackDetailFamily(rows []qFallbackStatsDetailRow, kind, code, reasonCode, reason string) string {
	for _, row := range rows {
		if row.Kind == kind && row.Code == code && row.ReasonCode == reasonCode && row.Reason == reason {
			return row.ReasonFamily
		}
	}
	return ""
}

func qTestFallbackFamilyCount(rows []qFallbackStatsDetailRow, family string) int64 {
	for _, row := range rows {
		if row.Kind == "reason_family" && row.ReasonFamily == family {
			return row.Count
		}
	}
	return 0
}

func qTestFallbackAttributionCount(rows []qFallbackStatsDetailRow, code, reasonCode, source, schemaHash, shape string) int64 {
	for _, row := range rows {
		if row.Kind == "reason_shape" && row.Code == code && row.ReasonCode == reasonCode && row.Source == source && row.SchemaHash == schemaHash && row.Shape == shape {
			return row.Count
		}
	}
	return 0
}

type qFallbackStatsTestExpr struct{}

func (qFallbackStatsTestExpr) EvalRow(data.Frame, int) (any, error) {
	return data.Symbol("fallback"), nil
}

func TestQSQLTableLiteralSourceExecution(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"rows := q.sql(\"select notional:sum price*size, fills:count i by sym from ([] sym:`AAPL`AAPL`MSFT; price:100.0 101.0 80.0; size:10 20 30) order by sym asc\", {})\n"+
			"keyed_rows := q.sql(\"select sym,price from ([sym:`AAPL`MSFT] price:100.0 80.0; size:10 20) where price<100\", {})\n"+
			"keyed_all := q.sql(\"select * from ([sym:`AAPL`MSFT] price:100.0 80.0; size:10 20) where sym=`AAPL\", {})\n"+
			"updated_literal := q.sql(\"update px:px+1 from ([] sym:`AAPL`MSFT; px:10 20)\", {})\n"+
			"deleted_literal := q.sql(\"delete from ([] sym:`AAPL`MSFT; px:10 20) where sym=`MSFT\", {})\n"+
			"keyed_updated_literal := q.sql(\"update price:price+1 from ([sym:`AAPL`MSFT] price:100.0 80.0; size:10 20) where sym=`AAPL\", {})\n"+
			"keyed_deleted_literal := q.sql(\"delete from ([sym:`AAPL`MSFT] price:100.0 80.0; size:10 20) where sym=`MSFT\", {})\n"+
			"drop_keyed_literal_ok, drop_keyed_literal_err := pcall(func() {\n"+
			"    return q.sql(\"delete sym from ([sym:`AAPL`MSFT] price:100.0 80.0; size:10 20)\", {})\n"+
			"})\n")

	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("rows len = %v, want 2", rows)
	}
	first := rows.RawGetInt(1).Table()
	if got := first.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rows[1].sym = %v, want AAPL", got)
	}
	if got := first.RawGetString("notional"); !got.IsFloat() || got.Float() != 3020 {
		t.Fatalf("rows[1].notional = %v, want 3020", got)
	}
	if got := first.RawGetString("fills"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("rows[1].fills = %v, want 2", got)
	}

	keyedRows := interp.GetGlobal("keyed_rows").Table()
	if keyedRows == nil || keyedRows.Length() != 1 {
		t.Fatalf("keyed_rows len = %v, want 1", keyedRows)
	}
	if got := keyedRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("keyed_rows[1].sym = %v, want MSFT", got)
	}
	if got := keyedRows.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 80 {
		t.Fatalf("keyed_rows[1].price = %v, want 80", got)
	}

	keyedAll := interp.GetGlobal("keyed_all").Table()
	if keyedAll == nil || keyedAll.Length() != 1 {
		t.Fatalf("keyed_all len = %v, want 1", keyedAll)
	}
	allRow := keyedAll.RawGetInt(1).Table()
	if got := allRow.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("keyed_all[1].sym = %v, want AAPL", got)
	}
	if got := allRow.RawGetString("price"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("keyed_all[1].price = %v, want 100", got)
	}
	if got := allRow.RawGetString("size"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("keyed_all[1].size = %v, want 10", got)
	}

	updatedLiteral := interp.GetGlobal("updated_literal").Table()
	if updatedLiteral == nil || updatedLiteral.Length() != 2 {
		t.Fatalf("updated_literal len = %v, want 2", updatedLiteral)
	}
	if got := updatedLiteral.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("updated_literal[1].sym = %v, want AAPL", got)
	}
	if got := updatedLiteral.RawGetInt(1).Table().RawGetString("px"); !got.IsInt() || got.Int() != 11 {
		t.Fatalf("updated_literal[1].px = %v, want 11", got)
	}
	if got := updatedLiteral.RawGetInt(2).Table().RawGetString("px"); !got.IsInt() || got.Int() != 21 {
		t.Fatalf("updated_literal[2].px = %v, want 21", got)
	}

	deletedLiteral := interp.GetGlobal("deleted_literal").Table()
	if deletedLiteral == nil || deletedLiteral.Length() != 1 {
		t.Fatalf("deleted_literal len = %v, want 1", deletedLiteral)
	}
	if got := deletedLiteral.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("deleted_literal[1].sym = %v, want AAPL", got)
	}
	if got := deletedLiteral.RawGetInt(1).Table().RawGetString("px"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("deleted_literal[1].px = %v, want 10", got)
	}

	keyedUpdatedLiteral := interp.GetGlobal("keyed_updated_literal").Table()
	if keyedUpdatedLiteral == nil {
		t.Fatalf("keyed_updated_literal is nil")
	}
	if keys := qTestArrayStrings(t, keyedUpdatedLiteral.RawGetString("keys").Table()); len(keys) != 1 || keys[0] != "sym" {
		t.Fatalf("keyed_updated_literal.keys = %v, want [sym]", keys)
	}
	keyedUpdatedFrame := keyedUpdatedLiteral.RawGetString("frame").Table()
	if keyedUpdatedFrame == nil || keyedUpdatedFrame.Length() != 2 {
		t.Fatalf("keyed_updated_literal.frame len = %v, want 2", keyedUpdatedFrame)
	}
	if got := keyedUpdatedFrame.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("keyed_updated_literal.frame[1].sym = %v, want AAPL", got)
	}
	if got := keyedUpdatedFrame.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("keyed_updated_literal.frame[1].price = %v, want 101", got)
	}
	if got := keyedUpdatedFrame.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 80 {
		t.Fatalf("keyed_updated_literal.frame[2].price = %v, want 80", got)
	}

	keyedDeletedLiteral := interp.GetGlobal("keyed_deleted_literal").Table()
	if keyedDeletedLiteral == nil {
		t.Fatalf("keyed_deleted_literal is nil")
	}
	if keys := qTestArrayStrings(t, keyedDeletedLiteral.RawGetString("keys").Table()); len(keys) != 1 || keys[0] != "sym" {
		t.Fatalf("keyed_deleted_literal.keys = %v, want [sym]", keys)
	}
	keyedDeletedFrame := keyedDeletedLiteral.RawGetString("frame").Table()
	if keyedDeletedFrame == nil || keyedDeletedFrame.Length() != 1 {
		t.Fatalf("keyed_deleted_literal.frame len = %v, want 1", keyedDeletedFrame)
	}
	if got := keyedDeletedFrame.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("keyed_deleted_literal.frame[1].sym = %v, want AAPL", got)
	}
	if got := keyedDeletedFrame.RawGetInt(1).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("keyed_deleted_literal.frame[1].price = %v, want 100", got)
	}
	assertPCallErrorContains(t, interp, "drop_keyed_literal", "keyed literal mutation")
}

func TestQSQLTemporalLiteralTableAndWhere(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"rows := q.sql(\"select sym,d,t,ts from ([] sym:`AAPL`MSFT`NVDA; d:2026.06.06 0Nd 2026.06.07; t:09:30:00 0Nt 09:31:00.250; ts:2026.06.06D09:30:00 0Np 2026.06.07D09:31:00) where d=2026.06.06\", {})\n"+
			"typed := q.eval(\"2026.06.06D09:30:00 0Np\")\n")

	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 1 {
		t.Fatalf("rows len = %v, want 1", rows)
	}
	row := rows.RawGetInt(1).Table()
	if got := row.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rows[1].sym = %v, want AAPL", got)
	}
	if got := row.RawGetString("d"); !got.IsString() || got.Str() != "2026-06-06" {
		t.Fatalf("rows[1].d = %v, want 2026-06-06", got)
	}
	if got := row.RawGetString("t"); !got.IsString() || got.Str() != "09:30:00" {
		t.Fatalf("rows[1].t = %v, want 09:30:00", got)
	}
	if got := row.RawGetString("ts"); !got.IsString() || got.Str() != "2026-06-06T09:30:00Z" {
		t.Fatalf("rows[1].ts = %v, want 2026-06-06T09:30:00Z", got)
	}
	if got := rows.RawGetString("column_kinds").Table().RawGetString("d"); !got.IsString() || got.Str() != "date" {
		t.Fatalf("rows.column_kinds.d = %v, want date", got)
	}
	typed := interp.GetGlobal("typed").Table()
	if typed == nil || typed.Length() != 2 {
		t.Fatalf("typed = %v, want 2-row array table", typed)
	}
	if got := typed.RawGetInt(1); !got.IsString() || got.Str() != "2026-06-06T09:30:00Z" {
		t.Fatalf("typed[1] = %v, want timestamp", got)
	}
	if got := typed.RawGetInt(2); !isDataNullValue(got) {
		t.Fatalf("typed[2] = %v, want data.null", got)
	}
}

func TestQSQLWherePredicates(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"MSFT\", \"NVDA\", \"TSLA\"}),\n"+
			"    price: data.f64({100.5, 80.0, 210.0, 190.0}),\n"+
			"    size: array.i64({10, 20, 30, 40}),\n"+
			"    active: array.bool({true, false, true, true}),\n"+
			"})\n"+
			"filtered := q.sql(trades, \"select sym,price,size from trades where sym in `AAPL`NVDA and price within 100 220 and not active=false order by sym asc\")\n"+
			"comma_filtered := q.sql(trades, \"select sym,price from trades where active=true,price>=190 order by sym asc\")\n"+
			"semicolon_filtered := q.sql(trades, \"select sym,price from trades where active=true;price>=190 order by sym asc\")\n")

	filtered := interp.GetGlobal("filtered").Table()
	if filtered == nil || filtered.Length() != 2 {
		t.Fatalf("filtered len = %v, want 2", filtered)
	}
	if got := filtered.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("filtered[1].sym = %v, want AAPL", got)
	}
	if got := filtered.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("filtered[2].sym = %v, want NVDA", got)
	}
	commaFiltered := interp.GetGlobal("comma_filtered").Table()
	if commaFiltered == nil || commaFiltered.Length() != 2 {
		t.Fatalf("comma_filtered len = %v, want 2", commaFiltered)
	}
	if got := commaFiltered.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("comma_filtered[1].sym = %v, want NVDA", got)
	}
	semicolonFiltered := interp.GetGlobal("semicolon_filtered").Table()
	if semicolonFiltered == nil || semicolonFiltered.Length() != 2 {
		t.Fatalf("semicolon_filtered len = %v, want 2", semicolonFiltered)
	}
	if got := semicolonFiltered.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "TSLA" {
		t.Fatalf("semicolon_filtered[2].sym = %v, want TSLA", got)
	}
}

func TestQSQLPlanCachesDoNotStoreFrameData(t *testing.T) {
	qSQLResetPlanCachesForTest()

	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"events := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"NVDA\"},\n"+
			"    price: array.f64({100, 80, 120}),\n"+
			"})\n"+
			"events.column_kinds = {sym: \"symbol\", price: \"f64\"}\n"+
			"rows := q.sql(events, \"select sym,price from events where price>=100 order by price desc limit 1\")\n")

	qSQLTemplateCacheMu.Lock()
	if len(qSQLTemplateCache) != 1 {
		t.Fatalf("template cache entries = %d, want 1", len(qSQLTemplateCache))
	}
	for key, tmpl := range qSQLTemplateCache {
		if got := tmpl.plan.Source.Len(); got != 0 {
			t.Fatalf("template cache %q Source.Len() = %d, want zero frame", key, got)
		}
		if len(tmpl.plan.Source.Schema().Names()) != 0 {
			t.Fatalf("template cache %q stored source schema", key)
		}
	}
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	if len(qSQLAlignedPlanCache) != 1 {
		t.Fatalf("aligned cache entries = %d, want 1", len(qSQLAlignedPlanCache))
	}
	for key, plan := range qSQLAlignedPlanCache {
		if got := plan.Source.Len(); got != 0 {
			t.Fatalf("aligned cache %q Source.Len() = %d, want zero frame", key, got)
		}
		if len(plan.Source.Schema().Names()) != 0 {
			t.Fatalf("aligned cache %q stored source schema", key)
		}
	}
	qSQLAlignedPlanCacheMu.Unlock()

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.TemplateMisses != 1 || stats.TemplateHits != 0 {
		t.Fatalf("template cache stats = %+v, want 1 miss and 0 hits", stats)
	}
	if stats.AlignedMisses != 1 || stats.AlignedHits != 0 {
		t.Fatalf("aligned cache stats = %+v, want 1 miss and 0 hits", stats)
	}
}

func TestQSQLPlanCacheIgnoresKeyedWrapperShape(t *testing.T) {
	qSQLResetPlanCachesForTest()

	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"MSFT\"}),\n"+
			"    trade_id: array.i64({1, 2, 3}),\n"+
			"    price: data.f64({100.0, 101.0, 80.0}),\n"+
			"})\n"+
			"src := \"select trade_id,price from trades where price>=100 order by trade_id asc\"\n"+
			"plain_rows := q.sql(trades, src)\n"+
			"keyed := q.key_by(trades, \"sym\")\n"+
			"keyed.extra_shape = {debug: true}\n"+
			"keyed_rows := q.sql(keyed, src)\n"+
			"map_rows := q.sql(src, {trades: keyed})\n"+
			"value_rows := q.sql(q.value(keyed), src)\n"+
			"value_rows_again := q.sql(q.value(keyed), src)\n")

	for _, name := range []string{"plain_rows", "keyed_rows", "map_rows", "value_rows", "value_rows_again"} {
		rows := interp.GetGlobal(name).Table()
		if rows == nil || rows.Length() != 2 {
			t.Fatalf("%s len = %v, want 2", name, rows)
		}
		if got := rows.RawGetInt(1).Table().RawGetString("trade_id"); !got.IsInt() || got.Int() != 1 {
			t.Fatalf("%s[1].trade_id = %v, want 1", name, got)
		}
		if got := rows.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
			t.Fatalf("%s[2].price = %v, want 101", name, got)
		}
	}

	qSQLTemplateCacheMu.Lock()
	if len(qSQLTemplateCache) != 1 {
		t.Fatalf("template cache entries = %d, want 1", len(qSQLTemplateCache))
	}
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	if len(qSQLAlignedPlanCache) != 2 {
		t.Fatalf("aligned cache entries = %d, want 2", len(qSQLAlignedPlanCache))
	}
	for key := range qSQLAlignedPlanCache {
		if strings.Contains(key, qKeyedFrameMarker) || strings.Contains(key, "extra_shape") {
			t.Fatalf("aligned cache key includes keyed wrapper shape: %q", key)
		}
	}
	qSQLAlignedPlanCacheMu.Unlock()

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.TemplateMisses != 1 || stats.TemplateHits != 4 {
		t.Fatalf("template cache stats = %+v, want 1 miss and 4 hits", stats)
	}
	if stats.AlignedMisses != 2 || stats.AlignedHits != 3 {
		t.Fatalf("aligned cache stats = %+v, want 2 misses and 3 hits", stats)
	}
}

func TestQSQLMutationPlanCacheUsesSchemaStableUnboundPlans(t *testing.T) {
	qSQLResetPlanCachesForTest()

	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"AAPL\", \"MSFT\"}),\n"+
			"    price: data.f64({100.0, 101.0, 80.0}),\n"+
			"})\n"+
			"src := \"update adjusted:price+1 from trades where price>=threshold\"\n"+
			"u1 := q.sql(trades, src, {threshold: 100.0})\n"+
			"u2 := q.sql(trades, src, {threshold: 101.0})\n"+
			"u3 := q.sql(trades, src, {threshold: 200.0})\n")

	u1 := interp.GetGlobal("u1").Table()
	u2 := interp.GetGlobal("u2").Table()
	u3 := interp.GetGlobal("u3").Table()
	if u1 == nil || u1.Length() != 3 || u2 == nil || u2.Length() != 3 || u3 == nil || u3.Length() != 3 {
		t.Fatalf("unexpected mutation result lengths: u1=%v u2=%v u3=%v", u1, u2, u3)
	}
	if got := u1.RawGetInt(1).Table().RawGetString("adjusted"); !got.IsFloat() || got.Float() != 101.0 {
		t.Fatalf("u1[1].adjusted = %v, want 101", got)
	}
	if got := u2.RawGetInt(1).Table().RawGetString("adjusted"); !got.IsNil() {
		t.Fatalf("u2[1].adjusted = %v, want nil because threshold changed", got)
	}
	if got := u2.RawGetInt(2).Table().RawGetString("adjusted"); !got.IsFloat() || got.Float() != 102.0 {
		t.Fatalf("u2[2].adjusted = %v, want 102", got)
	}
	if got := u3.RawGetInt(1).Table().RawGetString("adjusted"); !got.IsNil() {
		t.Fatalf("u3[1].adjusted = %v, want nil because no rows matched", got)
	}

	qSQLAlignedPlanCacheMu.Lock()
	if len(qSQLAlignedPlanCache) != 0 {
		t.Fatalf("aligned query plan cache entries = %d, want 0 for mutation-only test", len(qSQLAlignedPlanCache))
	}
	if len(qSQLAlignedMutationCache) != 1 {
		t.Fatalf("aligned mutation cache entries = %d, want 1", len(qSQLAlignedMutationCache))
	}
	for key, mutation := range qSQLAlignedMutationCache {
		if qDataExprHasLiteralValue(mutation.Where, float64(100)) || qDataExprHasLiteralValue(mutation.Where, float64(101)) || qDataExprHasLiteralValue(mutation.Where, float64(200)) {
			t.Fatalf("aligned mutation cache %q stored bound threshold literal: %#v", key, mutation.Where)
		}
		if !qDataExprHasColumnRef(mutation.Where, "threshold") {
			t.Fatalf("aligned mutation cache %q lost unbound threshold column ref: %#v", key, mutation.Where)
		}
	}
	qSQLAlignedPlanCacheMu.Unlock()

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.TemplateMisses != 1 || stats.TemplateHits != 2 {
		t.Fatalf("template cache stats = %+v, want 1 miss and 2 hits", stats)
	}
	if stats.AlignedMisses != 1 || stats.AlignedHits != 2 {
		t.Fatalf("aligned cache stats = %+v, want 1 miss and 2 hits", stats)
	}
}

func TestQSQLMutationPlanCacheSplitsBySchemaFingerprint(t *testing.T) {
	qSQLResetPlanCachesForTest()

	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades_f64 := data.frame({sym: data.symbols({\"AAPL\"}), price: data.f64({100.0})})\n"+
			"trades_i64 := data.frame({sym: data.symbols({\"AAPL\"}), price: array.i64({100})})\n"+
			"src := \"update adjusted:price+1 from trades where price>=100\"\n"+
			"f64_rows := q.sql(trades_f64, src)\n"+
			"i64_rows := q.sql(trades_i64, src)\n"+
			"f64_again := q.sql(trades_f64, src)\n")

	if got := interp.GetGlobal("f64_rows").Table().RawGetInt(1).Table().RawGetString("adjusted"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("f64 adjusted = %v, want 101", got)
	}
	if got := interp.GetGlobal("i64_rows").Table().RawGetInt(1).Table().RawGetString("adjusted"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("i64 adjusted = %v, want 101", got)
	}
	if got := interp.GetGlobal("f64_again").Table().RawGetInt(1).Table().RawGetString("adjusted"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("f64_again adjusted = %v, want 101", got)
	}

	qSQLAlignedPlanCacheMu.Lock()
	if len(qSQLAlignedMutationCache) != 2 {
		t.Fatalf("aligned mutation cache entries = %d, want 2 for f64/i64 schemas", len(qSQLAlignedMutationCache))
	}
	qSQLAlignedPlanCacheMu.Unlock()

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.TemplateMisses != 1 || stats.TemplateHits != 2 {
		t.Fatalf("template cache stats = %+v, want 1 miss and 2 hits", stats)
	}
	if stats.AlignedMisses != 2 || stats.AlignedHits != 1 {
		t.Fatalf("aligned cache stats = %+v, want 2 misses and 1 hit", stats)
	}
}

func TestQSQLPlanCacheEvictsOldestEntriesAndStats(t *testing.T) {
	qSQLResetPlanCachesForTest()

	qSQLTemplateCacheMu.Lock()
	for i := 0; i < qSQLPlanCacheLimit+2; i++ {
		qSQLTemplateCacheStoreLocked(strings.Repeat("t", i+1), qSQLPlanTemplate{})
	}
	if len(qSQLTemplateCache) != qSQLPlanCacheLimit {
		t.Fatalf("template cache entries = %d, want %d", len(qSQLTemplateCache), qSQLPlanCacheLimit)
	}
	if _, ok := qSQLTemplateCache["t"]; ok {
		t.Fatalf("template cache kept oldest entry")
	}
	if _, ok := qSQLTemplateCache[strings.Repeat("t", qSQLPlanCacheLimit+2)]; !ok {
		t.Fatalf("template cache dropped newest entry")
	}
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	for i := 0; i < qSQLPlanCacheLimit+2; i++ {
		qSQLAlignedPlanCacheStoreLocked(strings.Repeat("a", i+1), data.QueryPlan{})
	}
	if len(qSQLAlignedPlanCache) != qSQLPlanCacheLimit {
		t.Fatalf("aligned cache entries = %d, want %d", len(qSQLAlignedPlanCache), qSQLPlanCacheLimit)
	}
	if _, ok := qSQLAlignedPlanCache["a"]; ok {
		t.Fatalf("aligned cache kept oldest entry")
	}
	if _, ok := qSQLAlignedPlanCache[strings.Repeat("a", qSQLPlanCacheLimit+2)]; !ok {
		t.Fatalf("aligned cache dropped newest entry")
	}
	qSQLAlignedPlanCacheMu.Unlock()

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.TemplateEvictions != 2 || stats.AlignedEvictions != 2 {
		t.Fatalf("cache stats = %+v, want 2 evictions in both caches", stats)
	}
}

func TestQSQLKernelCacheEvictionPrunesPerKeyStats(t *testing.T) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100})},
	)
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	plan := data.QueryPlan{
		Select: []data.SelectItem{{Name: "sym", Expr: data.ColumnRef{Name: "sym"}}},
		LimitN: -1,
	}
	firstKey := data.QueryKernelCacheKey("kernel-evict-0", frame, plan)
	lastKey := ""

	qSQLAlignedPlanCacheMu.Lock()
	for i := 0; i < qSQLPlanCacheLimit+2; i++ {
		key := data.QueryKernelCacheKey(fmt.Sprintf("kernel-evict-%d", i), frame, plan)
		lastKey = key
		qSQLKernelStatsForKeyLocked(key).Misses++
		qSQLKernelCacheStoreLocked(key, nil)
	}
	if len(qSQLKernelCache) != qSQLPlanCacheLimit {
		t.Fatalf("kernel cache entries = %d, want %d", len(qSQLKernelCache), qSQLPlanCacheLimit)
	}
	if len(qSQLKernelStatsByKey) != qSQLPlanCacheLimit {
		t.Fatalf("kernel per-key stats entries = %d, want %d active keys", len(qSQLKernelStatsByKey), qSQLPlanCacheLimit)
	}
	if _, ok := qSQLKernelCache[firstKey]; ok {
		t.Fatalf("kernel cache kept oldest entry")
	}
	if _, ok := qSQLKernelStatsByKey[firstKey]; ok {
		t.Fatalf("kernel per-key stats kept evicted oldest key")
	}
	if _, ok := qSQLKernelCache[lastKey]; !ok {
		t.Fatalf("kernel cache dropped newest entry")
	}
	if _, ok := qSQLKernelStatsByKey[lastKey]; !ok {
		t.Fatalf("kernel per-key stats dropped newest key")
	}
	qSQLAlignedPlanCacheMu.Unlock()

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.KernelEvictions != 2 {
		t.Fatalf("kernel cache evictions = %d, want 2", stats.KernelEvictions)
	}
	if len(stats.KernelKeys) != qSQLPlanCacheLimit {
		t.Fatalf("kernel key stats rows = %d, want %d active keys", len(stats.KernelKeys), qSQLPlanCacheLimit)
	}

	row := qTestCacheStatsRowTable(t, qCacheStatsTable(), "qsql_kernel")
	keys := row.RawGetString("keys").Table()
	if keys == nil || keys.Length() != qSQLPlanCacheLimit {
		t.Fatalf("qsql_kernel keys rows = %v, want %d active keys", keys, qSQLPlanCacheLimit)
	}
	shapes := qTestKernelShapeRows(t, row.RawGetString("shapes").Table())
	if len(shapes) != 1 || shapes[0].Shape != "unknown" || shapes[0].Count != int64(qSQLPlanCacheLimit) {
		t.Fatalf("qsql_kernel shapes = %+v, want one unknown shape for active keys only", shapes)
	}
	if got := qTestKernelCacheKeyStats(t, stats.KernelKeys, lastKey); got.Misses != 1 {
		t.Fatalf("newest kernel key stats = %+v, want one miss", got)
	}
}

func BenchmarkQSQLRepeatedMutationPlanCacheSmoke(b *testing.B) {
	qSQLResetPlanCachesForTest()

	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "NVDA", "TSLA"})},
		data.Column{Name: "price", Data: data.NewF64([]float64{100, 80, 210, 190})},
		data.Column{Name: "size", Data: data.NewI64([]int64{10, 20, 30, 40})},
	)
	if err != nil {
		b.Fatal(err)
	}
	frameValue, err := qDataFrameValue(frame)
	if err != nil {
		b.Fatal(err)
	}
	query := "update notional:price*size from trades where price>=threshold"
	env := NewTable()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.RawSetString("threshold", FloatValue(float64(90+i%3)))
		_, err := qRunSQL("q.sql", qSQLArgsResult{
			frameValue: frameValue,
			source:     query,
			envValue:   TableValue(env),
		})
		if err != nil {
			b.Fatalf("q.sql failed: %v", err)
		}
	}
	b.StopTimer()

	stats := qSQLPlanCacheStatsSnapshot()
	if stats.TemplateMisses != 1 || stats.AlignedMisses != 1 {
		b.Fatalf("cache stats = %+v, want one template miss and one aligned miss", stats)
	}
}

func qDataExprHasColumnRef(expr data.Expr, name string) bool {
	switch x := expr.(type) {
	case nil:
		return false
	case data.ColumnRef:
		return string(x.Name) == name
	case data.Binary:
		return qDataExprHasColumnRef(x.Left, name) || qDataExprHasColumnRef(x.Right, name)
	case data.Logical:
		return qDataExprHasColumnRef(x.Left, name) || qDataExprHasColumnRef(x.Right, name)
	case data.Not:
		return qDataExprHasColumnRef(x.Expr, name)
	case data.In:
		return qDataExprHasColumnRef(x.Expr, name)
	case data.Within:
		return qDataExprHasColumnRef(x.Expr, name)
	case data.BucketFloorExpr:
		return qDataExprHasColumnRef(x.Expr, name)
	case data.ListAggregateExpr:
		return qDataExprHasColumnRef(x.Expr, name)
	default:
		return false
	}
}

func qDataExprHasLiteralValue(expr data.Expr, value any) bool {
	switch x := expr.(type) {
	case nil:
		return false
	case data.Literal:
		return x.Value == value
	case data.Binary:
		return qDataExprHasLiteralValue(x.Left, value) || qDataExprHasLiteralValue(x.Right, value)
	case data.Logical:
		return qDataExprHasLiteralValue(x.Left, value) || qDataExprHasLiteralValue(x.Right, value)
	case data.Not:
		return qDataExprHasLiteralValue(x.Expr, value)
	case data.In:
		return qDataExprHasLiteralValue(x.Expr, value)
	case data.Within:
		return qDataExprHasLiteralValue(x.Expr, value)
	case data.BucketFloorExpr:
		return qDataExprHasLiteralValue(x.Expr, value)
	case data.ListAggregateExpr:
		return qDataExprHasLiteralValue(x.Expr, value)
	default:
		return false
	}
}

func assertPCallErrorContains(t *testing.T, interp *runtime.Interpreter, prefix, want string) {
	t.Helper()
	ok := interp.GetGlobal(prefix + "_ok")
	if !ok.IsBool() || ok.Bool() {
		t.Fatalf("%s_ok = %v, %s_err = %v, want false", prefix, ok, prefix, interp.GetGlobal(prefix+"_err"))
	}
	errValue := interp.GetGlobal(prefix + "_err")
	if !errValue.IsString() {
		t.Fatalf("%s_err = %v (%s), want string", prefix, errValue, errValue.TypeName())
	}
	if got := errValue.Str(); !strings.Contains(got, want) {
		t.Fatalf("%s_err = %q, want substring %q", prefix, got, want)
	}
}

func TestQSymbolicCoreDataForms(t *testing.T) {
	interp := runWithQAndSOA(t,
		"syms := q.eval(\"`AAPL`MSFT`NVDA\")\n"+
			"spread := q.eval(\"100 101.5 103 - 99.5 100 101\")\n"+
			"running := q.eval(\"+\\\\100 101.5 103\")\n"+
			"total := q.eval(\"+/10 20 30 40\")\n"+
			"named_total := q.eval(\"sum 10 20 30 40\")\n"+
			"named_running := q.eval(\"sums 100 101.5 103\")\n"+
			"idx := q.eval(\"where false true true\")\n"+
			"idx_count := q.count(idx)\n"+
			"first_two := q.eval(\"2#10 20 30\")\n"+
			"dict := q.eval(\"`bid`ask`last!(99.5 100;100.5 101;100 101.5)\")\n"+
			"trades := q.eval(\"flip `sym`side`price`size!(`AAPL`MSFT`AAPL;`buy`sell`buy;100.5 200 101;10 15 20)\")\n"+
			"tagged_total := q`+/1 2 3`\n"+
			"tagged_running := q`+\\1 2 3`\n"+
			"tagged_idx := q`where true false true`\n"+
			"tagged_take := q`2#7 8 9`\n")

	if got := interp.GetGlobal("syms").Table().RawGetInt(2); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("syms[2] = %v, want MSFT", got)
	}
	if got, _ := interp.GetGlobal("spread").DenseArray().At(2); !got.IsFloat() || got.Float() != 2 {
		t.Fatalf("spread[3] = %v, want 2", got)
	}
	if got, _ := interp.GetGlobal("running").DenseArray().At(2); !got.IsFloat() || got.Float() != 304.5 {
		t.Fatalf("running[3] = %v, want 304.5", got)
	}
	if got := interp.GetGlobal("total"); !got.IsInt() || got.Int() != 100 {
		t.Fatalf("total = %v, want 100", got)
	}
	if got := interp.GetGlobal("named_total"); !got.IsInt() || got.Int() != 100 {
		t.Fatalf("named_total = %v, want 100", got)
	}
	if got, _ := interp.GetGlobal("named_running").DenseArray().At(2); !got.IsFloat() || got.Float() != 304.5 {
		t.Fatalf("named_running[3] = %v, want 304.5", got)
	}
	if got, _ := interp.GetGlobal("idx").DenseArray().At(0); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("idx[1] = %v, want 1", got)
	}
	if got := interp.GetGlobal("idx_count"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("idx_count = %v, want 2", got)
	}
	if got, _ := interp.GetGlobal("first_two").DenseArray().At(1); !got.IsInt() || got.Int() != 20 {
		t.Fatalf("first_two[2] = %v, want 20", got)
	}
	dict := interp.GetGlobal("dict").Table()
	if got, _ := dict.RawGetString("ask").DenseArray().At(1); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("dict.ask[2] = %v, want 101", got)
	}
	trades := interp.GetGlobal("trades").Table()
	if trades == nil || trades.Length() != 3 {
		t.Fatalf("trades length = %v, want 3", trades)
	}
	if got := trades.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("trades[2].sym = %v, want MSFT", got)
	}
	if got := trades.RawGetInt(3).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("trades[3].price = %v, want 101", got)
	}
	if got := interp.GetGlobal("tagged_total"); !got.IsInt() || got.Int() != 6 {
		t.Fatalf("tagged_total = %v, want 6", got)
	}
	if got, _ := interp.GetGlobal("tagged_running").DenseArray().At(2); !got.IsInt() || got.Int() != 6 {
		t.Fatalf("tagged_running[3] = %v, want 6", got)
	}
	if got, _ := interp.GetGlobal("tagged_idx").DenseArray().At(1); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("tagged_idx[2] = %v, want 2", got)
	}
	if got, _ := interp.GetGlobal("tagged_take").DenseArray().At(1); !got.IsInt() || got.Int() != 8 {
		t.Fatalf("tagged_take[2] = %v, want 8", got)
	}
}

func TestQKeyedAmendAndUpsertAPI(t *testing.T) {
	interp := runWithQAndSOA(t,
		"base := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"MSFT\"}),\n"+
			"    venue: data.symbols({\"XNAS\", \"XNYS\"}),\n"+
			"    qty: data.i64({10, 20}),\n"+
			"    note: {\"old-a\", \"old-m\"},\n"+
			"})\n"+
			"keyed := q.key_by(base, q.eval(\"`sym`venue\"))\n"+
			"amend_delta := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"TSLA\"}),\n"+
			"    venue: data.symbols({\"XNAS\", \"XNAS\"}),\n"+
			"    qty: data.i64({15, 30}),\n"+
			"    note: {\"amend-a\", \"ignored-t\"},\n"+
			"})\n"+
			"amended := q.amend(keyed, amend_delta, q.eval(\"`qty\"))\n"+
			"upsert_delta := data.frame({\n"+
			"    sym: data.symbols({\"MSFT\", \"TSLA\"}),\n"+
			"    venue: data.symbols({\"XNYS\", \"XNAS\"}),\n"+
			"    qty: data.i64({25, 40}),\n"+
			"    note: {\"new-m\", \"new-t\"},\n"+
			"})\n"+
			"upserted := q.upsert(amended, upsert_delta, q.eval(\"`qty`note\"))\n"+
			"aapl := q.lookup(upserted, \"AAPL\", \"XNAS\")\n"+
			"msft := q.lookup(upserted, \"MSFT\", \"XNYS\")\n"+
			"tsla := q.lookup(upserted, \"TSLA\", \"XNAS\")\n")
	amended := interp.GetGlobal("amended").Table()
	if got := amended.RawGetString("frame").Table().RawGetInt(1).Table().RawGetString("qty"); !got.IsInt() || got.Int() != 15 {
		t.Fatalf("amended AAPL qty = %v, want 15", got)
	}
	if got := amended.RawGetString("frame").Table().RawGetInt(1).Table().RawGetString("note"); !got.IsString() || got.Str() != "old-a" {
		t.Fatalf("amended AAPL note = %v, want old-a because value column list limits q.amend", got)
	}
	if got := amended.RawGetString("frame").Table().Length(); got != 2 {
		t.Fatalf("amended row count = %d, want 2", got)
	}
	upserted := interp.GetGlobal("upserted").Table()
	if got := upserted.RawGetString("frame").Table().Length(); got != 3 {
		t.Fatalf("upserted row count = %d, want 3", got)
	}
	if keys := qTestArrayStrings(t, upserted.RawGetString("keys").Table()); len(keys) != 2 || keys[0] != "sym" || keys[1] != "venue" {
		t.Fatalf("upserted keys = %v, want [sym venue]", keys)
	}
	if got := interp.GetGlobal("aapl").Table().RawGetInt(1).Table().RawGetString("qty"); !got.IsInt() || got.Int() != 15 {
		t.Fatalf("aapl qty = %v, want 15", got)
	}
	if got := interp.GetGlobal("msft").Table().RawGetInt(1).Table().RawGetString("note"); !got.IsString() || got.Str() != "new-m" {
		t.Fatalf("msft note = %v, want new-m", got)
	}
	if got := interp.GetGlobal("tsla").Table().RawGetInt(1).Table().RawGetString("qty"); !got.IsInt() || got.Int() != 40 {
		t.Fatalf("tsla qty = %v, want 40", got)
	}
}

func TestQKeyedAmendKeepsKeysAndTypedNullsFromEval(t *testing.T) {
	interp := runWithQAndSOA(t,
		"keyed := q.eval(\"`sym xkey flip `sym`qty!(`AAPL`MSFT;1 2)\")\n"+
			"delta := q.eval(\"flip `sym`qty!(enlist `AAPL;enlist 0Ni)\")\n"+
			"amended := q.amend(keyed, delta, q.eval(\"`qty\"))\n"+
			"functional_upserted := q.eval(\"@[`sym xkey flip `sym`qty!(`AAPL`MSFT;1 2);`TSLA;+;`qty!5]\")\n"+
			"functional_hit := q.lookup(functional_upserted, \"TSLA\")\n"+
			"functional_keys := q.keys(functional_upserted)\n"+
			"hit := q.lookup(amended, \"AAPL\")\n"+
			"keys := q.keys(amended)\n"+
			"value_rows := q.value(amended)\n")

	keys := qTestArrayStrings(t, interp.GetGlobal("keys").Table())
	if len(keys) != 1 || keys[0] != "sym" {
		t.Fatalf("amended keys = %v, want [sym]", keys)
	}
	hit := interp.GetGlobal("hit").Table()
	if hit == nil || hit.Length() != 1 {
		t.Fatalf("hit len = %v, want 1", hit)
	}
	row := hit.RawGetInt(1).Table()
	if got := row.RawGetString("sym"); !got.IsNil() {
		t.Fatalf("lookup value row sym = %v, want nil because key columns stay outside value frame", got)
	}
	if got := hit.RawGetString("qty").Table().RawGetInt(1); !isDataNullValue(got) {
		t.Fatalf("amended qty = %v, want typed data null", got)
	} else if kind := dataNullValueKind(got); kind != data.KindI64 {
		t.Fatalf("amended qty null kind = %q, want i64", kind)
	}
	valueRows := interp.GetGlobal("value_rows").Table()
	if got := valueRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("value frame row sym = %v, want nil", got)
	}
	if got := valueRows.RawGetString("qty").Table().RawGetInt(1); !isDataNullValue(got) {
		t.Fatalf("value frame qty = %v, want typed data null", got)
	} else if kind := dataNullValueKind(got); kind != data.KindI64 {
		t.Fatalf("value frame qty null kind = %q, want i64", kind)
	}
	if got := qTestArrayStrings(t, interp.GetGlobal("functional_keys").Table()); len(got) != 1 || got[0] != "sym" {
		t.Fatalf("functional keyed amend keys = %v, want [sym]", got)
	}
	functionalHit := interp.GetGlobal("functional_hit").Table()
	if functionalHit == nil || functionalHit.Length() != 1 {
		t.Fatalf("functional_hit len = %v, want 1", functionalHit)
	}
	if got := functionalHit.RawGetString("qty").Table().RawGetInt(1); !got.IsInt() || got.Int() != 5 {
		t.Fatalf("functional missing-key qty = %v, want 5", got)
	}
}

func TestQDictionaryAndKeyedAPIBoundaryErrors(t *testing.T) {
	interp := runWithQAndSOA(t,
		"base := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"MSFT\"}),\n"+
			"    venue: data.symbols({\"XNAS\", \"XNYS\"}),\n"+
			"    qty: data.i64({10, 20}),\n"+
			"})\n"+
			"keyed := q.key_by(base, q.eval(\"`sym`venue\"))\n"+
			"delta_missing_key := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\"}),\n"+
			"    qty: data.i64({15}),\n"+
			"})\n"+
			"\n"+
			"lookup_non_keyed_ok, lookup_non_keyed_err := pcall(func() {\n"+
			"    return q.lookup(base, \"AAPL\", \"XNAS\")\n"+
			"})\n"+
			"lookup_missing_field_ok, lookup_missing_field_err := pcall(func() {\n"+
			"    return q.lookup(keyed, {sym: \"AAPL\"})\n"+
			"})\n"+
			"amend_non_keyed_ok, amend_non_keyed_err := pcall(func() {\n"+
			"    return q.amend(base, delta_missing_key)\n"+
			"})\n"+
			"amend_missing_delta_key_ok, amend_missing_delta_key_err := pcall(func() {\n"+
			"    return q.amend(keyed, delta_missing_key)\n"+
			"})\n"+
			"amend_bad_value_cols_ok, amend_bad_value_cols_err := pcall(func() {\n"+
			"    return q.amend(keyed, base, {qty: true})\n"+
			"})\n"+
			"upsert_missing_delta_key_ok, upsert_missing_delta_key_err := pcall(func() {\n"+
			"    return q.upsert(keyed, delta_missing_key)\n"+
			"})\n"+
			"keys_scalar_ok, keys_scalar_err := pcall(func() {\n"+
			"    return q.keys(42)\n"+
			"})\n"+
			"key_scalar_ok, key_scalar_err := pcall(func() {\n"+
			"    return q.key(42)\n"+
			"})\n"+
			"cols_scalar_ok, cols_scalar_err := pcall(func() {\n"+
			"    return q.cols(42)\n"+
			"})\n"+
			"meta_scalar_ok, meta_scalar_err := pcall(func() {\n"+
			"    return q.meta(42)\n"+
			"})\n"+
			"value_noarg_ok, value_noarg_err := pcall(func() {\n"+
			"    return q.value()\n"+
			"})\n"+
			"\n"+
			"dict := q.eval(\"`b`a!2 1\")\n"+
			"dict_keys := q.keys(dict)\n"+
			"dict_values := q.value(dict)\n"+
			"key_names := q.keys(keyed)\n"+
			"value_rows := q.value(keyed)\n"+
			"cols := q.cols(keyed)\n"+
			"meta := q.meta(keyed)\n")

	assertPCallErrorContains(t, interp, "lookup_non_keyed", "argument 1 must be a keyed frame")
	assertPCallErrorContains(t, interp, "lookup_missing_field", "key \"venue\" is missing")
	assertPCallErrorContains(t, interp, "amend_non_keyed", "argument 1 must be a keyed frame")
	assertPCallErrorContains(t, interp, "amend_missing_delta_key", "key column \"venue\" does not exist")
	assertPCallErrorContains(t, interp, "amend_bad_value_cols", "keys must be strings or an array table of strings")
	assertPCallErrorContains(t, interp, "upsert_missing_delta_key", "key column \"venue\" does not exist")
	assertPCallErrorContains(t, interp, "keys_scalar", "expected dictionary, table, or keyed frame")
	assertPCallErrorContains(t, interp, "key_scalar", "expected dictionary, table, or keyed frame")
	assertPCallErrorContains(t, interp, "cols_scalar", "argument 1 must be a frame table or soa")
	assertPCallErrorContains(t, interp, "meta_scalar", "argument 1 must be a frame table or soa")
	assertPCallErrorContains(t, interp, "value_noarg", "argument 1 required")

	if got := qTestArrayStrings(t, interp.GetGlobal("dict_keys").Table()); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("dict_keys = %v, want q source order [b a]", got)
	}
	dictValues := interp.GetGlobal("dict_values").Table()
	if dictValues == nil || dictValues.RawGetInt(1).Int() != 2 || dictValues.RawGetInt(2).Int() != 1 {
		t.Fatalf("dict_values = %v, want [2 1]", dictValues)
	}
	if got := qTestArrayStrings(t, interp.GetGlobal("key_names").Table()); len(got) != 2 || got[0] != "sym" || got[1] != "venue" {
		t.Fatalf("key_names = %v, want [sym venue]", got)
	}
	valueRows := interp.GetGlobal("value_rows").Table()
	if valueRows == nil || valueRows.Length() != 2 {
		t.Fatalf("value_rows len = %v, want 2", valueRows)
	}
	if got := valueRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsNil() {
		t.Fatalf("value_rows[1].sym = %v, want nil", got)
	}
	if got := qTestArrayStrings(t, interp.GetGlobal("cols").Table()); len(got) != 3 || got[0] != "sym" || got[1] != "venue" || got[2] != "qty" {
		t.Fatalf("cols = %v, want [sym venue qty]", got)
	}
	meta := interp.GetGlobal("meta").Table()
	if meta == nil || meta.Length() != 3 {
		t.Fatalf("meta len = %v, want 3", meta)
	}
	if got := meta.RawGetInt(1).Table().RawGetString("c"); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("meta[1].c = %v, want sym", got)
	}
	if got := meta.RawGetInt(3).Table().RawGetString("c"); !got.IsString() || got.Str() != "qty" {
		t.Fatalf("meta[3].c = %v, want qty", got)
	}
	if got := meta.RawGetInt(3).Table().RawGetString("t"); !got.IsString() || got.Str() != "i64" {
		t.Fatalf("meta[3].t = %v, want i64", got)
	}
}

func TestQEvalNestedDictsStayScriptVisible(t *testing.T) {
	interp := runWithQAndSOA(t,
		"nested := q.eval(\"(meta `p#`AAPL`AAPL`MSFT;meta `sym$`AAPL`MSFT`AAPL)\")\n"+
			"attr := nested[1]\n"+
			"enum_meta := nested[2]\n")

	attr := interp.GetGlobal("attr").Table()
	if attr == nil {
		t.Fatalf("attr = nil, want table")
	}
	if got := attr.RawGetString("attribute"); !got.IsString() || got.Str() != "p" {
		t.Fatalf("attr.attribute = %v, want p", got)
	}
	if got := attr.RawGetString("count"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("attr.count = %v, want 3", got)
	}
	enumMeta := interp.GetGlobal("enum_meta").Table()
	if enumMeta == nil {
		t.Fatalf("enum_meta = nil, want table")
	}
	if got := enumMeta.RawGetString("domain"); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("enum_meta.domain = %v, want sym", got)
	}
	if got := enumMeta.RawGetString("count"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("enum_meta.count = %v, want 3", got)
	}
}

func TestQEvalTypedNullsExposeStableKindMetadata(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"timestamp_null := q.eval(\"0Np\")\n"+
			"timestamp_prev := q.eval(\"prev 0Np 0Np\")\n"+
			"symbol_next := q.eval(\"next `AAPL`MSFT\")\n"+
			"typed_empty_prior := q.eval(\"-':0#10 20\")\n"+
			"timestamp_null_ok := data.is_null(timestamp_null)\n"+
			"timestamp_prev_1_ok := data.is_null(timestamp_prev[1])\n"+
			"timestamp_prev_2_ok := data.is_null(timestamp_prev[2])\n"+
			"symbol_next_2_ok := data.is_null(symbol_next[2])\n"+
			"typed_empty_prior_ok := q.count(typed_empty_prior) == 0 && typed_empty_prior.kind == \"i64\"\n")

	timestampNull := interp.GetGlobal("timestamp_null")
	if !isDataNullValue(timestampNull) {
		t.Fatalf("timestamp_null = %v, want data null sentinel", timestampNull)
	}
	if got := dataNullValueKind(timestampNull); got != data.KindTimestamp {
		t.Fatalf("timestamp_null kind = %s, want timestamp", got)
	}

	timestampPrev := interp.GetGlobal("timestamp_prev").Table()
	if timestampPrev == nil || timestampPrev.Length() != 2 {
		t.Fatalf("timestamp_prev = %v, want length 2 table", timestampPrev)
	}
	for i := int64(1); i <= 2; i++ {
		item := timestampPrev.RawGetInt(i)
		if !isDataNullValue(item) {
			t.Fatalf("timestamp_prev[%d] = %v, want data null sentinel", i, item)
		}
		if got := dataNullValueKind(item); got != data.KindTimestamp {
			t.Fatalf("timestamp_prev[%d] kind = %s, want timestamp", i, got)
		}
	}

	symbolNext := interp.GetGlobal("symbol_next").Table()
	if symbolNext == nil || symbolNext.Length() != 2 {
		t.Fatalf("symbol_next = %v, want length 2 table", symbolNext)
	}
	if got := symbolNext.RawGetInt(1); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("symbol_next[1] = %v, want MSFT", got)
	}
	last := symbolNext.RawGetInt(2)
	if !isDataNullValue(last) {
		t.Fatalf("symbol_next[2] = %v, want data null sentinel", last)
	}
	if got := dataNullValueKind(last); got != data.KindSymbol {
		t.Fatalf("symbol_next[2] kind = %s, want symbol", got)
	}

	for _, name := range []string{"timestamp_null_ok", "timestamp_prev_1_ok", "timestamp_prev_2_ok", "symbol_next_2_ok", "typed_empty_prior_ok"} {
		if got := interp.GetGlobal(name); !got.IsBool() || !got.Bool() {
			t.Fatalf("%s = %v, want true", name, got)
		}
	}
}

func TestQEvalTypedNumericScalarsExposeAsNumbers(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"i16s := q.eval(\"1h 2h 0Nh\")\n"+
			"i32s := q.eval(\"1i 2i 0Ni\")\n"+
			"f32s := q.eval(\"1e 0Ne\")\n"+
			"short_i32 := q.eval(\"i$3\")\n"+
			"short_f64_null_type := q.eval(\"type f$0N\")\n"+
			"short_f32s := q.eval(\"e$1 2 0Nf\")\n"+
			"amended := q.eval(\"@[(1 2 0Ni);2;:;3]\")\n"+
			"short_amended := q.eval(\"@[(1 2 0Ni);2;:;i$3]\")\n"+
			"i16_ok := i16s[1] == 1 && i16s[2] == 2\n"+
			"i32_ok := i32s[1] == 1 && i32s[2] == 2\n"+
			"f32_ok := f32s[1] == 1.0\n"+
			"short_cast_ok := short_i32 == 3 && short_f64_null_type == -9 && short_f32s.kind == \"f32\" && data.is_null(short_f32s[3])\n"+
			"amended_ok := amended.kind == \"i32\" && amended[1] == 1 && amended[2] == 2 && amended[3] == 3\n"+
			"short_amended_ok := short_amended.kind == \"i32\" && short_amended[3] == 3\n")
	for _, name := range []string{"i16_ok", "i32_ok", "f32_ok", "short_cast_ok", "amended_ok", "short_amended_ok"} {
		if got := interp.GetGlobal(name); !got.IsBool() || !got.Bool() {
			t.Fatalf("%s = %v, want true", name, got)
		}
	}
}

func TestQEncodeDecodeRoundTripAPI(t *testing.T) {
	interp := runWithQAndSOA(t,
		"base := data.frame({\n"+
			"    sym: data.symbols({\"AAPL\", \"MSFT\"}),\n"+
			"    qty: data.i64({10, 20}),\n"+
			"    trade_date: data.date({\"2026-06-06\", data.null}),\n"+
			"    event_ts: data.timestamp({\"2026-06-06T09:30:00Z\", data.null}),\n"+
			"    session_time: data.time({\"09:30:00\", data.null}),\n"+
			"})\n"+
			"keyed := q.key_by(base, \"sym\")\n"+
			"decoded_keyed := q.decode(q.encode(keyed))\n"+
			"decoded_aapl := q.lookup(decoded_keyed, \"AAPL\")\n"+
			"decoded_msft := q.lookup(decoded_keyed, \"MSFT\")\n"+
			"decoded_frame := q.decode(q.encode(base))\n"+
			"\n"+
			"dict := q.eval(\"`bid`ask!99 101\")\n"+
			"decoded_dict := q.decode(q.encode(dict))\n"+
			"decoded_dict_keys := q.keys(decoded_dict)\n"+
			"decoded_dict_values := q.value(decoded_dict)\n"+
			"\n"+
			"nested := q.eval(\"`frame`keyed`dict!(`placeholder`placeholder`placeholder)\")\n"+
			"nested.frame = base\n"+
			"nested.keyed = keyed\n"+
			"nested.dict = dict\n"+
			"decoded_nested := q.decode(q.encode(nested))\n"+
			"decoded_nested_keys := q.keys(decoded_nested)\n"+
			"decoded_nested_frame := decoded_nested.frame\n"+
			"decoded_nested_hit := q.lookup(decoded_nested.keyed, \"AAPL\")\n"+
			"decoded_nested_dict_keys := q.keys(decoded_nested.dict)\n"+
			"\n"+
			"syms := q.eval(\"`AAPL`MSFT`TSLA\")\n"+
			"decoded_syms := q.decode(q.encode(syms))\n"+
			"\n"+
			"vec := q.eval(\"1 2 3\")\n"+
			"decoded_vec := q.decode(q.encode(vec))\n"+
			"\n"+
			"encode_missing_ok, encode_missing_err := pcall(func() { return q.encode() })\n"+
			"encode_fn_ok, encode_fn_err := pcall(func() { return q.encode(q.encode) })\n"+
			"decode_type_ok, decode_type_err := pcall(func() { return q.decode(42) })\n"+
			"decode_payload_ok, decode_payload_err := pcall(func() { return q.decode(\"not-json\") })\n")
	if got := interp.GetGlobal("decoded_aapl").Table().RawGetInt(1).Table().RawGetString("qty"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("decoded AAPL qty = %v, want 10", got)
	}
	decodedFrame := interp.GetGlobal("decoded_frame").Table()
	if got := decodedFrame.RawGetString("column_kinds").Table().RawGetString("trade_date"); !got.IsString() || got.Str() != "date" {
		t.Fatalf("decoded frame trade_date kind = %v, want date", got)
	}
	if got := decodedFrame.RawGetString("column_kinds").Table().RawGetString("event_ts"); !got.IsString() || got.Str() != "timestamp" {
		t.Fatalf("decoded frame event_ts kind = %v, want timestamp", got)
	}
	if got := decodedFrame.RawGetString("column_kinds").Table().RawGetString("session_time"); !got.IsString() || got.Str() != "time" {
		t.Fatalf("decoded frame session_time kind = %v, want time", got)
	}
	if got := decodedFrame.RawGetInt(1).Table().RawGetString("trade_date"); !got.IsString() || got.Str() != "2026-06-06" {
		t.Fatalf("decoded frame trade_date[1] = %v, want 2026-06-06", got)
	}
	if got := decodedFrame.RawGetInt(1).Table().RawGetString("event_ts"); !got.IsString() || got.Str() != "2026-06-06T09:30:00Z" {
		t.Fatalf("decoded frame event_ts[1] = %v, want 2026-06-06T09:30:00Z", got)
	}
	if got := decodedFrame.RawGetInt(1).Table().RawGetString("session_time"); !got.IsString() || got.Str() != "09:30:00" {
		t.Fatalf("decoded frame session_time[1] = %v, want 09:30:00", got)
	}
	if got := interp.GetGlobal("decoded_msft").Table().RawGetInt(1).Table().RawGetString("event_ts"); !got.IsNil() && !isDataNullValue(got) {
		t.Fatalf("decoded MSFT event_ts = %v, want null", got)
	}
	if got := qTestArrayStrings(t, interp.GetGlobal("decoded_dict_keys").Table()); len(got) != 2 || got[0] != "bid" || got[1] != "ask" {
		t.Fatalf("decoded dict keys = %v, want [bid ask]", got)
	}
	values := interp.GetGlobal("decoded_dict_values").Table()
	if values == nil || values.RawGetInt(1).Int() != 99 || values.RawGetInt(2).Int() != 101 {
		t.Fatalf("decoded dict values = %v, want [99 101]", values)
	}
	if got := qTestArrayStrings(t, interp.GetGlobal("decoded_syms").Table()); len(got) != 3 || got[2] != "TSLA" {
		t.Fatalf("decoded syms = %v, want [AAPL MSFT TSLA]", got)
	}
	vec := interp.GetGlobal("decoded_vec").DenseArray()
	if vec == nil || vec.Len() != 3 {
		t.Fatalf("decoded vec = %v, want dense len 3", interp.GetGlobal("decoded_vec"))
	}
	if got, err := vec.At(2); err != nil || !got.IsInt() || got.Int() != 3 {
		t.Fatalf("decoded vec[2] = %v err=%v, want 3", got, err)
	}
	if got := qTestArrayStrings(t, interp.GetGlobal("decoded_nested_keys").Table()); len(got) != 3 || got[0] != "frame" || got[1] != "keyed" || got[2] != "dict" {
		t.Fatalf("decoded nested keys = %v, want [frame keyed dict]", got)
	}
	if got := interp.GetGlobal("decoded_nested_frame").Table().RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("decoded nested frame sym[2] = %v, want MSFT", got)
	}
	if got := interp.GetGlobal("decoded_nested_hit").Table().RawGetInt(1).Table().RawGetString("qty"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("decoded nested keyed qty = %v, want 10", got)
	}
	if got := qTestArrayStrings(t, interp.GetGlobal("decoded_nested_dict_keys").Table()); len(got) != 2 || got[0] != "bid" || got[1] != "ask" {
		t.Fatalf("decoded nested dict keys = %v, want [bid ask]", got)
	}
	assertPCallErrorContains(t, interp, "encode_missing", "argument 1 required")
	assertPCallErrorContains(t, interp, "encode_fn", "unsupported value type")
	assertPCallErrorContains(t, interp, "decode_type", "argument 1 must be encoded string")
	assertPCallErrorContains(t, interp, "decode_payload", "invalid character")
}

func TestQSplayedAndPartitionedStoreRoundTripFeedsQSQL(t *testing.T) {
	dir := t.TempDir()
	interp := runWithQAndSOA(t, "trades := q.eval(\"([] day:1 1 2 2; sym:`AAPL`MSFT`AAPL`MSFT; qty:10 20 30 40; price:100.5 101.25 102.75 103.5)\")")
	interp.SetGlobal("store_path", StringValue(dir))
	execOnInterp(t, interp, `
ok1 := q.save_splayed(trades, store_path .. "/splayed")
loaded := q.load_splayed(store_path .. "/splayed")
splayed_result := q.sql(loaded, "select total:sum qty by sym from trades")

ok2 := q.save_partitioned(trades, store_path .. "/partitioned", "day", "sym")
only := q.load_partitioned(store_path .. "/partitioned", {sym: "AAPL"})
partitioned_result := q.sql(only, "select total:sum qty by sym from trades")
`)

	if got := interp.GetGlobal("ok1"); !got.IsBool() || !got.Bool() {
		t.Fatalf("q.save_splayed ok = %v, want true", got)
	}
	if got := interp.GetGlobal("ok2"); !got.IsBool() || !got.Bool() {
		t.Fatalf("q.save_partitioned ok = %v, want true", got)
	}
	splayedRows := interp.GetGlobal("splayed_result").Table()
	if got := splayedRows.Length(); got != 2 {
		t.Fatalf("splayed result rows = %d, want 2", got)
	}
	partitionedRows := interp.GetGlobal("partitioned_result").Table()
	if got := partitionedRows.Length(); got != 1 {
		t.Fatalf("partitioned result rows = %d, want 1", got)
	}
	row := partitionedRows.RawGetInt(1).Table()
	if got := row.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("partitioned sym = %v, want AAPL", got)
	}
	if got := row.RawGetString("total"); !got.IsNumber() || got.Float() != 40 {
		t.Fatalf("partitioned total = %v, want 40", got)
	}
}

func runWithQAndSOA(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	for name, lib := range map[string]*Table{
		"array":   BuildArray(),
		"data":    BuildData(),
		"dialect": BuildDialect(HostOptions{Call: interp.CallFunction}, nil),
		"soa":     BuildSOA(),
		"q":       BuildQ(),
	} {
		interp.SetGlobal(name, runtime.TableValue(lib))
		interp.SetModule(name, runtime.TableValue(lib))
	}
	execOnInterp(t, interp, src)
	return interp
}
