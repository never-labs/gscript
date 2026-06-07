package q

import (
	"reflect"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestIPCLoopbackPortableFrameCodecRoundTrip(t *testing.T) {
	state := NewEvalState(nil)
	got, err := state.Eval(`h:hopen "loopback";h["([] sym:` + "`AAPL`MSFT`AAPL" + `; ts:2026.06.06D09:30:00 0Np 2026.06.06D09:31:00; qty:10 0N 30)"]`)
	if err != nil {
		t.Fatal(err)
	}
	frame, ok := got.(data.Frame)
	if !ok {
		t.Fatalf("got %T, want frame", got)
	}
	if frame.Len() != 3 {
		t.Fatalf("frame len = %d, want 3", frame.Len())
	}
	ts, _ := frame.Column("ts")
	if ts.Kind() != data.KindTimestamp {
		t.Fatalf("ts kind = %s, want timestamp", ts.Kind())
	}
	middleTS, _ := ts.At(1)
	if !data.IsNull(middleTS) {
		t.Fatalf("ts[1] = %#v, want null", middleTS)
	}

	decoded := mustCodecRoundTrip(t, frame)
	roundTrip, ok := decoded.(data.Frame)
	if !ok {
		t.Fatalf("decoded = %T, want frame", decoded)
	}
	if got, want := roundTrip.Schema().Names(), []data.Symbol{"sym", "ts", "qty"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("schema = %#v, want %#v", got, want)
	}
	qty, _ := roundTrip.Column("qty")
	middleQty, _ := qty.At(1)
	if !data.IsNull(middleQty) {
		t.Fatalf("qty[1] = %#v, want null", middleQty)
	}
}

func TestIPCLoopbackMessageListBindsPortableTableArgument(t *testing.T) {
	frame, err := data.NewFrame(
		data.Column{Name: "sym", Data: data.NewSymbols([]string{"AAPL", "MSFT", "AAPL"})},
		data.Column{Name: "qty", Data: data.NewI64([]int64{10, 20, 30})},
	)
	if err != nil {
		t.Fatal(err)
	}
	state := NewEvalState(nil)
	handle := &qIPCHandle{target: "loopback", session: NewEvalState(nil)}
	got, err := state.applyIPCHandle(handle, []any{data.NewAny([]any{qUnaryFunction{name: "count", fn: count}, frame})})
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(3) {
		t.Fatalf("got %#v (%T), want 3", got, got)
	}
}

func TestIPCLoopbackKeyedFrameEnumTemporalNullCodecRoundTrip(t *testing.T) {
	state := NewEvalState(nil)
	got, err := state.Eval("h:hopen \"loopback\";h[\"([sym:`AAPL`MSFT] ts:2026.06.06D09:30:00 0Np; venue:`XNYS`XNAS; qty:10 0N)\"]")
	if err != nil {
		t.Fatal(err)
	}
	keyed, ok := got.(data.KeyedFrame)
	if !ok {
		t.Fatalf("got %T, want keyed frame", got)
	}
	if keys := keyed.Keys(); !reflect.DeepEqual(keys, []data.Symbol{"sym"}) {
		t.Fatalf("keys = %#v, want sym", keys)
	}
	frame := keyed.Frame()
	ts, _ := frame.Column("ts")
	if ts.Kind() != data.KindTimestamp {
		t.Fatalf("ts kind = %s, want timestamp", ts.Kind())
	}
	missingTS, _ := ts.At(1)
	if !data.IsNull(missingTS) {
		t.Fatalf("ts[1] = %#v, want null", missingTS)
	}
	decoded := mustCodecRoundTrip(t, keyed)
	roundTrip, ok := decoded.(data.KeyedFrame)
	if !ok {
		t.Fatalf("decoded = %T, want keyed frame", decoded)
	}
	row, err := roundTrip.LookupValueByKey(data.Symbol("MSFT"))
	if err != nil {
		t.Fatal(err)
	}
	qty, _ := row.Column("qty")
	missingQty, _ := qty.At(0)
	if !data.IsNull(missingQty) {
		t.Fatalf("MSFT qty = %#v, want null", missingQty)
	}
	enumAny, err := state.Eval("h[\"`venue$`XNYS`XNAS`XNYS\"]")
	if err != nil {
		t.Fatal(err)
	}
	enum, ok := enumAny.(qEnumVector)
	if !ok {
		t.Fatalf("enum = %T, want qEnumVector", enumAny)
	}
	if enum.domain != data.Symbol("venue") {
		t.Fatalf("enum domain = %v, want venue", enum.domain)
	}
	enumRoundTrip := mustCodecRoundTrip(t, enum)
	roundTripEnum, ok := enumRoundTrip.(qEnumVector)
	if !ok {
		t.Fatalf("roundtrip enum = %T, want qEnumVector", enumRoundTrip)
	}
	if codes := roundTripEnum.EncodedCodes(); !reflect.DeepEqual(codes, []int32{0, 1, 0}) {
		t.Fatalf("roundtrip enum codes = %#v", codes)
	}
}
