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

func TestNativeFrameFilterProjectColumnFiltersSingleProjectedColumn(t *testing.T) {
	soa, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{10, 12, 8}),
		"size":  NewDenseArrayI64([]int64{100, 200, 300}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := NewTable()
	frame.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "filter-project-column-test",
	})

	col, handled, err := TableValue(frame).NativeFrameFilterProjectColumn(NewDenseArrayBool([]bool{true, false, true}), []string{"size"}, "size")
	if err != nil {
		t.Fatalf("NativeFrameFilterProjectColumn: %v", err)
	}
	if !handled || !col.IsDenseArray() {
		t.Fatalf("NativeFrameFilterProjectColumn = %v handled=%v, want dense array", col, handled)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 300 {
		t.Fatalf("filter project column values = %#v, want [100 300]", got)
	}
	if _, handled, err := TableValue(frame).NativeFrameFilterProjectColumn(NewDenseArrayBool([]bool{true, false, true}), []string{"price"}, "size"); !handled || err == nil {
		t.Fatalf("NativeFrameFilterProjectColumn unprojected result handled=%v err=%v, want handled error", handled, err)
	}
}

func TestNativeFrameFilterProjectFiltersProjectedFrame(t *testing.T) {
	soa, err := NewSoA(map[string]*DenseArray{
		"price": NewDenseArrayF64([]float64{10, 12, 8}),
		"size":  NewDenseArrayI64([]int64{100, 200, 300}),
		"flag":  NewDenseArrayBool([]bool{true, false, true}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := NewTable()
	frame.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    3,
		SchemaHash: "filter-project-test",
	})

	out, handled, err := TableValue(frame).NativeFrameFilterProject(NewDenseArrayBool([]bool{true, false, true}), []string{"size", "price"})
	if err != nil {
		t.Fatalf("NativeFrameFilterProject: %v", err)
	}
	if !handled || !out.IsFrame() {
		t.Fatalf("NativeFrameFilterProject = %v handled=%v, want native frame", out, handled)
	}
	payload, info, ok := out.Table().NativeFramePayload()
	if !ok {
		t.Fatalf("NativeFrameFilterProject payload missing")
	}
	filtered, ok := payload.(*SoA)
	if !ok {
		t.Fatalf("NativeFrameFilterProject payload = %T, want *SoA", payload)
	}
	if info.Rows != 2 || info.Columns != 2 || info.SchemaHash == "filter-project-test" {
		t.Fatalf("NativeFrameFilterProject info = %+v, want filtered/projected schema info", info)
	}
	if _, ok := filtered.Column("flag"); ok {
		t.Fatalf("NativeFrameFilterProject kept unprojected flag column")
	}
	size, ok := filtered.Column("size")
	if !ok {
		t.Fatalf("NativeFrameFilterProject missing size column")
	}
	got, ok := size.I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 300 {
		t.Fatalf("filter project size values = %#v, want [100 300]", got)
	}
}

func TestNativeFrameMaskFilterProjectComposesWithVectorWhereScanReduce(t *testing.T) {
	soa, err := NewSoA(map[string]*DenseArray{
		"price":  NewDenseArrayF64([]float64{100.5, 99, 120, 80}),
		"size":   NewDenseArrayI64([]int64{10, 20, 5, 30}),
		"active": NewDenseArrayBool([]bool{true, false, true, true}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := NewTable()
	frame.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    3,
		SchemaHash: "mask-filter-project-compose-test",
	})

	priceMask, handled, err := TableValue(frame).NativeFrameMaskOp("price", DenseArrayGE, FloatValue(100))
	if err != nil {
		t.Fatalf("NativeFrameMaskOp price: %v", err)
	}
	if !handled || !priceMask.IsDenseArray() {
		t.Fatalf("NativeFrameMaskOp price = %v handled=%v, want dense mask", priceMask, handled)
	}
	activeMask, handled, err := TableValue(frame).NativeFrameMaskOp("active", DenseArrayEQ, BoolValue(true))
	if err != nil {
		t.Fatalf("NativeFrameMaskOp active: %v", err)
	}
	if !handled || !activeMask.IsDenseArray() {
		t.Fatalf("NativeFrameMaskOp active = %v handled=%v, want dense mask", activeMask, handled)
	}
	mask, err := DenseArrayMaskCombine(DenseArrayMaskAnd, priceMask, activeMask)
	if err != nil {
		t.Fatalf("DenseArrayMaskCombine: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(mask), []bool{true, false, true, false})

	projected, handled, err := TableValue(frame).NativeFrameFilterProject(mask, []string{"size", "price"})
	if err != nil {
		t.Fatalf("NativeFrameFilterProject: %v", err)
	}
	if !handled || !projected.IsFrame() {
		t.Fatalf("NativeFrameFilterProject = %v handled=%v, want native frame", projected, handled)
	}
	if _, info, ok := projected.Table().NativeFramePayload(); !ok || info.Rows != 2 || info.Columns != 2 {
		t.Fatalf("projected frame info ok=%v info=%+v, want 2 rows and 2 columns", ok, info)
	}
	size, handled, err := projected.NativeFrameColumn("size")
	if err != nil {
		t.Fatalf("NativeFrameColumn size: %v", err)
	}
	if !handled || !size.IsDenseArray() {
		t.Fatalf("NativeFrameColumn size = %v handled=%v, want dense array", size, handled)
	}
	assertDenseI64(t, size, []int64{10, 5})

	selected, err := DenseArrayWhere(mask, DenseArrayValue(NewDenseArrayI64([]int64{10, 20, 5, 30})), IntValue(0))
	if err != nil {
		t.Fatalf("DenseArrayWhere: %v", err)
	}
	assertDenseI64(t, DenseArrayValue(selected), []int64{10, 0, 5, 0})
	scanned, err := DenseArrayScan(selected)
	if err != nil {
		t.Fatalf("DenseArrayScan: %v", err)
	}
	assertDenseI64(t, DenseArrayValue(scanned), []int64{10, 10, 15, 15})
	sum, err := DenseArrayReduce(DenseArrayReduceSum, size.DenseArray())
	if err != nil {
		t.Fatalf("DenseArrayReduce: %v", err)
	}
	if !sum.Equal(IntValue(15)) {
		t.Fatalf("DenseArrayReduce sum = %v, want 15", sum)
	}
}
