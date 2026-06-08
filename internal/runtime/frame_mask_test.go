package runtime

import "testing"

func TestNativeFrameMaskBuildsDenseBoolMask(t *testing.T) {
	soa, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{10, 12, 8}),
		"limit": NewDenseArrayF64([]float64{9, 12, 9}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := NewTable()
	frame.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "mask-test",
	})

	mask, handled, err := TableValue(frame).NativeFrameMask("price", ">=", FloatValue(10))
	if err != nil {
		t.Fatalf("NativeFrameMask scalar: %v", err)
	}
	if !handled || !mask.IsDenseArray() {
		t.Fatalf("NativeFrameMask scalar = %v handled=%v, want dense array", mask, handled)
	}
	got, ok := mask.DenseArray().Bool()
	if !ok || len(got) != 3 || !got[0] || !got[1] || got[2] {
		t.Fatalf("scalar mask = %#v, want [true true false]", got)
	}

	columnMask, handled, err := TableValue(frame).NativeFrameMask("price", ">=", StringValue("limit"))
	if err != nil {
		t.Fatalf("NativeFrameMask column: %v", err)
	}
	if !handled || !columnMask.IsDenseArray() {
		t.Fatalf("NativeFrameMask column = %v handled=%v, want dense array", columnMask, handled)
	}
	colGot, ok := columnMask.DenseArray().Bool()
	if !ok || len(colGot) != 3 || !colGot[0] || !colGot[1] || colGot[2] {
		t.Fatalf("column mask = %#v, want [true true false]", colGot)
	}
}

func TestNativeFrameMaskOpUsesTypedComparison(t *testing.T) {
	soa, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{10, 12, 8}),
		"limit": NewDenseArrayF64([]float64{9, 12, 9}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := NewTable()
	frame.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "mask-op-test",
	})

	mask, handled, err := TableValue(frame).NativeFrameMaskOp("price", DenseArrayGE, StringValue("limit"))
	if err != nil {
		t.Fatalf("NativeFrameMaskOp: %v", err)
	}
	if !handled || !mask.IsDenseArray() {
		t.Fatalf("NativeFrameMaskOp = %v handled=%v, want dense array", mask, handled)
	}
	got, ok := mask.DenseArray().Bool()
	if !ok || len(got) != 3 || !got[0] || !got[1] || got[2] {
		t.Fatalf("typed op mask = %#v, want [true true false]", got)
	}

	if _, handled, err := TableValue(frame).NativeFrameMaskOp("price", DenseArrayAdd, FloatValue(10)); !handled || err == nil {
		t.Fatalf("NativeFrameMaskOp arithmetic op handled=%v err=%v, want handled error", handled, err)
	}
}

func TestNativeFrameProjectColumnValidatesProjection(t *testing.T) {
	soa, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{10, 12}),
		"size":  NewDenseArrayI64([]int64{100, 200}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := NewTable()
	frame.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "project-column-test",
	})

	col, handled, err := TableValue(frame).NativeFrameProjectColumn([]string{"size"}, "size")
	if err != nil {
		t.Fatalf("NativeFrameProjectColumn: %v", err)
	}
	if !handled || !col.IsDenseArray() {
		t.Fatalf("NativeFrameProjectColumn = %v handled=%v, want dense array", col, handled)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 200 {
		t.Fatalf("project column values = %#v, want [100 200]", got)
	}
	if _, handled, err := TableValue(frame).NativeFrameProjectColumn([]string{"price"}, "size"); !handled || err == nil {
		t.Fatalf("NativeFrameProjectColumn unprojected result handled=%v err=%v, want handled error", handled, err)
	}
}
