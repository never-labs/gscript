package runtime

import "testing"

func TestNativeFrameOrderIndexesSortsAndLimitsRows(t *testing.T) {
	soa, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{10, 12, 12, 8}),
		"size":  NewDenseArrayI64([]int64{100, 50, 75, 200}),
		"live":  NewDenseArrayBool([]bool{true, false, true, true}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := NewTable()
	frame.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    3,
		SchemaHash: "order-test",
	})

	indexes, handled, err := TableValue(frame).NativeFrameOrderIndexes(
		[]string{"price", "size"},
		[]bool{true, false},
		3,
	)
	if err != nil {
		t.Fatalf("NativeFrameOrderIndexes: %v", err)
	}
	if !handled || !indexes.IsDenseArray() {
		t.Fatalf("NativeFrameOrderIndexes = %v handled=%v, want dense array", indexes, handled)
	}
	got, ok := indexes.DenseArray().I64()
	if !ok || len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 1 {
		t.Fatalf("order indexes = %#v, want [2 3 1]", got)
	}
}

func TestNativeFrameOrderIndexesRejectsMissingColumn(t *testing.T) {
	soa, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{10, 12}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := NewTable()
	frame.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    1,
		SchemaHash: "order-test",
	})

	_, handled, err := TableValue(frame).NativeFrameOrderIndexes([]string{"missing"}, nil, -1)
	if !handled || err == nil {
		t.Fatalf("NativeFrameOrderIndexes missing column handled=%v err=%v, want handled error", handled, err)
	}
}
