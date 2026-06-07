package runtime

import (
	"testing"
)

// ==================================================================
// Table operations edge cases
// ==================================================================

// --- Table creation edge cases ---

func TestTableEmptyCreation(t *testing.T) {
	v := getGlobal(t, `
		t := {}
		result := #t
	`, "result")
	if !v.IsInt() || v.Int() != 0 {
		t.Errorf("expected 0, got %v", v)
	}
}

func TestNewEmptyTableStartsWithCleanIterationKeys(t *testing.T) {
	tbl := NewTableSized(0, 0)
	if tbl.keysDirty {
		t.Fatal("new empty table should not require iteration-key rebuild")
	}
	if k, v, ok := tbl.Next(NilValue()); ok || !k.IsNil() || !v.IsNil() {
		t.Fatalf("empty Next = (%v, %v, %v), want nil, nil, false", k, v, ok)
	}
	if tbl.keysDirty {
		t.Fatal("empty Next should keep iteration keys clean")
	}

	tbl.RawSetString("missing", NilValue())
	if tbl.keysDirty {
		t.Fatal("nil write to absent string key should not dirty an empty table")
	}

	tbl.RawSetString("x", IntValue(1))
	if !tbl.keysDirty {
		t.Fatal("real string-key insert should dirty iteration keys")
	}
	k, v, ok := tbl.Next(NilValue())
	if !ok || !k.IsString() || k.Str() != "x" || !v.IsInt() || v.Int() != 1 {
		t.Fatalf("first Next = (%v, %v, %v), want x, 1, true", k, v, ok)
	}
	if tbl.keysDirty {
		t.Fatal("Next should rebuild and clean iteration keys after insert")
	}
}

func TestTableNativePayloadInvalidatesOnRawMutation(t *testing.T) {
	tbl := NewTable()
	payload := struct{ name string }{name: "frame"}
	info := NativePayloadInfo{Kind: NativePayloadDataFrame, Rows: 2, Columns: 1, SchemaHash: "abc"}
	tbl.SetNativePayloadWithInfo(payload, info)
	if got := tbl.NativePayload(); got != payload {
		t.Fatalf("native payload = %#v, want %#v", got, payload)
	}
	if got, ok := tbl.NativePayloadInfo(); !ok || got != info {
		t.Fatalf("native payload info = %#v, %v, want %#v, true", got, ok, info)
	}

	tbl.RawSetString("x", IntValue(1))
	if got := tbl.NativePayload(); got != nil {
		t.Fatalf("payload after string mutation = %#v, want nil", got)
	}
	if got, ok := tbl.NativePayloadInfo(); ok {
		t.Fatalf("payload info after string mutation = %#v, want none", got)
	}
	if got, ok := tbl.NativeFramePayloadInfo(); ok {
		t.Fatalf("frame payload info after string mutation = %#v, want none", got)
	}
	if got := TableValue(tbl).Type(); got != TypeTable {
		t.Fatalf("type after string mutation = %v, want TypeTable", got)
	}
	if got := TableValue(tbl).TypeName(); got != "table" {
		t.Fatalf("type name after string mutation = %q, want table", got)
	}

	tbl.SetNativePayload(payload)
	tbl.RawSetInt(1, IntValue(2))
	if got := tbl.NativePayload(); got != nil {
		t.Fatalf("payload after int mutation = %#v, want nil", got)
	}
}

func TestTableNativePayloadTypeNameReflectsFrameKinds(t *testing.T) {
	frame := NewTable()
	frameInfo := NativePayloadInfo{Kind: NativePayloadDataFrame, Rows: 2, Columns: 1, SchemaHash: "frame-schema"}
	frame.SetNativePayloadWithInfo(struct{}{}, frameInfo)
	if got, ok := frame.NativeFramePayloadInfo(); !ok || got != frameInfo {
		t.Fatalf("frame NativeFramePayloadInfo() = %#v, %v, want %#v, true", got, ok, frameInfo)
	}
	if got, ok := frame.NativeFramePayloadKind(); !ok || got != NativePayloadDataFrame {
		t.Fatalf("frame NativeFramePayloadKind() = %q, %v, want %q, true", got, ok, NativePayloadDataFrame)
	}
	if !frame.IsNativeFrame() {
		t.Fatal("frame IsNativeFrame() = false, want true")
	}
	if !frame.IsFrameFacade() {
		t.Fatal("frame IsFrameFacade() = false, want true")
	}
	if frame.IsNativeKeyedFrame() {
		t.Fatal("frame IsNativeKeyedFrame() = true, want false")
	}
	if got := TableValue(frame).Type(); got != TypeFrame {
		t.Fatalf("frame Type() = %v, want TypeFrame", got)
	}
	if got := TableValue(frame).IsFrame(); !got {
		t.Fatal("frame Value.IsFrame() = false, want true")
	}
	if got := TableValue(frame).TypeName(); got != "frame" {
		t.Fatalf("frame TypeName() = %q, want frame", got)
	}
	if got := TableValue(frame).String(); got[:6] != "frame:" {
		t.Fatalf("frame String() = %q, want frame:*", got)
	}

	keyed := NewTable()
	keyedInfo := NativePayloadInfo{Kind: NativePayloadKeyedFrame, Rows: 2, Columns: 3, SchemaHash: "keyed-schema"}
	keyed.SetNativePayloadWithInfo(struct{}{}, keyedInfo)
	if got, ok := keyed.NativeFramePayloadInfo(); !ok || got != keyedInfo {
		t.Fatalf("keyed NativeFramePayloadInfo() = %#v, %v, want %#v, true", got, ok, keyedInfo)
	}
	if got, ok := keyed.NativeFramePayloadKind(); !ok || got != NativePayloadKeyedFrame {
		t.Fatalf("keyed NativeFramePayloadKind() = %q, %v, want %q, true", got, ok, NativePayloadKeyedFrame)
	}
	if !keyed.IsNativeKeyedFrame() {
		t.Fatal("keyed IsNativeKeyedFrame() = false, want true")
	}
	if !keyed.IsKeyedFrameFacade() {
		t.Fatal("keyed IsKeyedFrameFacade() = false, want true")
	}
	if keyed.IsNativeFrame() {
		t.Fatal("keyed IsNativeFrame() = true, want false")
	}
	if got := TableValue(keyed).Type(); got != TypeKeyedFrame {
		t.Fatalf("keyed frame Type() = %v, want TypeKeyedFrame", got)
	}
	if got := TableValue(keyed).IsKeyedFrame(); !got {
		t.Fatal("keyed Value.IsKeyedFrame() = false, want true")
	}
	if got := TableValue(keyed).TypeName(); got != "keyed frame" {
		t.Fatalf("keyed frame TypeName() = %q, want keyed frame", got)
	}
	if got := TableValue(keyed).String(); got[:12] != "keyed frame:" {
		t.Fatalf("keyed frame String() = %q, want keyed frame:*", got)
	}
}

func TestNativePayloadKindRuntimeFrameMapping(t *testing.T) {
	cases := []struct {
		kind NativePayloadKind
		typ  ValueType
		name string
		ok   bool
	}{
		{kind: NativePayloadDataFrame, typ: TypeFrame, name: "frame", ok: true},
		{kind: NativePayloadKeyedFrame, typ: TypeKeyedFrame, name: "keyed frame", ok: true},
		{kind: NativePayloadDataColumn, ok: false},
		{kind: NativePayloadNone, ok: false},
	}

	for _, tc := range cases {
		if got := tc.kind.IsFrameFacadeKind(); got != tc.ok {
			t.Fatalf("%q IsFrameFacadeKind() = %v, want %v", tc.kind, got, tc.ok)
		}
		typ, typOK := tc.kind.ValueType()
		if typOK != tc.ok || (tc.ok && typ != tc.typ) {
			t.Fatalf("%q ValueType() = %v, %v, want %v, %v", tc.kind, typ, typOK, tc.typ, tc.ok)
		}
		name, nameOK := tc.kind.TypeName()
		if nameOK != tc.ok || (tc.ok && name != tc.name) {
			t.Fatalf("%q TypeName() = %q, %v, want %q, %v", tc.kind, name, nameOK, tc.name, tc.ok)
		}
	}
}

func TestTableNativePayloadWithoutKindStaysPlainTable(t *testing.T) {
	tbl := NewTable()
	tbl.SetNativePayload(struct{}{})
	value := TableValue(tbl)

	if tbl.IsFrameFacade() || tbl.IsKeyedFrameFacade() || tbl.IsNativeColumn() {
		t.Fatal("untyped native payload classified as a runtime facade")
	}
	if got := value.Type(); got != TypeTable {
		t.Fatalf("untyped native payload Type() = %v, want TypeTable", got)
	}
	if got := value.TypeName(); got != "table" {
		t.Fatalf("untyped native payload TypeName() = %q, want table", got)
	}
}

func TestTableNativeColumnPayloadStaysPlainTable(t *testing.T) {
	tbl := NewTable()
	info := NativePayloadInfo{Kind: NativePayloadDataColumn, Rows: 3, ColumnKind: "int"}
	tbl.SetNativePayloadWithInfo(struct{}{}, info)
	value := TableValue(tbl)

	if got, ok := tbl.NativePayloadKind(); !ok || got != NativePayloadDataColumn {
		t.Fatalf("NativePayloadKind() = %q, %v, want %q, true", got, ok, NativePayloadDataColumn)
	}
	if got, ok := tbl.NativeFramePayloadInfo(); ok {
		t.Fatalf("NativeFramePayloadInfo() = %#v, true, want none", got)
	}
	if got, ok := tbl.NativeFramePayloadKind(); ok {
		t.Fatalf("NativeFramePayloadKind() = %q, true, want none", got)
	}
	if !tbl.IsNativeColumn() {
		t.Fatal("IsNativeColumn() = false, want true")
	}
	if tbl.IsFrameFacade() || tbl.IsKeyedFrameFacade() {
		t.Fatal("native column payload classified as frame facade")
	}
	if got := value.Type(); got != TypeTable {
		t.Fatalf("native column Type() = %v, want TypeTable", got)
	}
	if got := value.TypeName(); got != "table" {
		t.Fatalf("native column TypeName() = %q, want table", got)
	}
}

func TestTableNativePayloadInvalidatesOnSpecializedMutation(t *testing.T) {
	payload := struct{ name string }{name: "frame"}

	shaped := NewTable()
	shaped.RawSetString("x", IntValue(1))
	shaped.SetNativePayload(payload)
	shaped.SvalsSet(0, IntValue(2))
	if got := shaped.NativePayload(); got != nil {
		t.Fatalf("payload after svals mutation = %#v, want nil", got)
	}

	ints := NewTableSizedKind(3, 0, ArrayInt)
	ints.RawSetInt(1, IntValue(3))
	ints.RawSetInt(2, IntValue(1))
	ints.RawSetInt(3, IntValue(2))
	ints.SetNativePayload(payload)
	if !ints.TryPlainArraySort(3) {
		t.Fatal("TryPlainArraySort failed")
	}
	if got := ints.NativePayload(); got != nil {
		t.Fatalf("payload after plain array sort = %#v, want nil", got)
	}

	src := NewTableSizedKind(2, 0, ArrayInt)
	src.RawSetInt(1, IntValue(10))
	src.RawSetInt(2, IntValue(20))
	dst := NewTableSizedKind(2, 0, ArrayInt)
	dst.SetNativePayload(payload)
	if !dst.TryPlainArrayMove(src, 1, 2, 1) {
		t.Fatal("TryPlainArrayMove failed")
	}
	if got := dst.NativePayload(); got != nil {
		t.Fatalf("payload after plain array move = %#v, want nil", got)
	}
}

func TestTableLazyIntGetterPreservesNativePayloadUntilMutation(t *testing.T) {
	tbl := NewTable()
	payload := struct{ name string }{name: "frame"}
	tbl.SetLazyIntGetter(2, func(key int64) (Value, bool) {
		return IntValue(key * 10), true
	})
	tbl.SetNativePayload(payload)

	if got := tbl.RawGetInt(1); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("lazy row = %v, want 10", got)
	}
	if got := tbl.NativePayload(); got != payload {
		t.Fatalf("native payload after lazy read = %#v, want %#v", got, payload)
	}

	tbl.RawSetInt(1, IntValue(99))
	if got := tbl.NativePayload(); got != nil {
		t.Fatalf("native payload after int mutation = %#v, want nil", got)
	}
	if got := tbl.RawGetInt(2); !got.IsNil() {
		t.Fatalf("lazy getter after int mutation = %v, want nil", got)
	}
}

func TestTableLazyIntGetterInvalidatesOnStringMutation(t *testing.T) {
	tbl := NewTable()
	payload := struct{ name string }{name: "frame"}
	tbl.SetLazyIntGetter(2, func(key int64) (Value, bool) {
		return IntValue(key * 10), true
	})
	tbl.SetNativePayload(payload)

	tbl.RawSetString("extra", IntValue(1))
	if got := tbl.NativePayload(); got != nil {
		t.Fatalf("native payload after string mutation = %#v, want nil", got)
	}
	if got := tbl.RawGetInt(1); !got.IsNil() {
		t.Fatalf("lazy getter after string mutation = %v, want nil", got)
	}
}

func TestTableSingleElement(t *testing.T) {
	v := getGlobal(t, `
		t := {42}
		result := t[1]
	`, "result")
	if !v.IsInt() || v.Int() != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

func TestTableMixedKeysAccess(t *testing.T) {
	interp := runProgram(t, `
		t := {10, 20, name: "test", 30}
		a := t[1]
		b := t[2]
		c := t[3]
		n := t.name
		l := #t
	`)
	if interp.GetGlobal("a").Int() != 10 {
		t.Errorf("expected a=10, got %v", interp.GetGlobal("a"))
	}
	if interp.GetGlobal("b").Int() != 20 {
		t.Errorf("expected b=20, got %v", interp.GetGlobal("b"))
	}
	if interp.GetGlobal("c").Int() != 30 {
		t.Errorf("expected c=30, got %v", interp.GetGlobal("c"))
	}
	if interp.GetGlobal("n").Str() != "test" {
		t.Errorf("expected n='test', got %v", interp.GetGlobal("n"))
	}
}

// --- Nested table creation ---

func TestTableNestedCreation(t *testing.T) {
	v := getGlobal(t, `
		t := {a: {b: {c: 42}}}
		result := t.a.b.c
	`, "result")
	if !v.IsInt() || v.Int() != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

func TestTableNestedArrayCreation(t *testing.T) {
	v := getGlobal(t, `
		t := {{1, 2}, {3, 4}, {5, 6}}
		result := t[2][1]
	`, "result")
	if !v.IsInt() || v.Int() != 3 {
		t.Errorf("expected 3, got %v", v)
	}
}

func TestTableNestedMixedCreation(t *testing.T) {
	v := getGlobal(t, `
		t := {data: {1, 2, 3}, info: {name: "test"}}
		result := t.data[2] + #t.data
	`, "result")
	// t.data[2] = 2, #t.data = 3, result = 5
	if !v.IsInt() || v.Int() != 5 {
		t.Errorf("expected 5, got %v", v)
	}
}

// --- Table mutation ---

func TestTableAddNewField(t *testing.T) {
	v := getGlobal(t, `
		t := {}
		t.x = 10
		t.y = 20
		t.z = 30
		result := t.x + t.y + t.z
	`, "result")
	if !v.IsInt() || v.Int() != 60 {
		t.Errorf("expected 60, got %v", v)
	}
}

func TestTableAppendNumericKeys(t *testing.T) {
	v := getGlobal(t, `
		t := {}
		for i := 1; i <= 5; i++ {
			t[i] = i * 10
		}
		result := t[3] + #t
	`, "result")
	// t[3] = 30, #t = 5, result = 35
	if !v.IsInt() || v.Int() != 35 {
		t.Errorf("expected 35, got %v", v)
	}
}

func TestTableOverwriteField(t *testing.T) {
	v := getGlobal(t, `
		t := {x: 10}
		t.x = 20
		t.x = 30
		result := t.x
	`, "result")
	if !v.IsInt() || v.Int() != 30 {
		t.Errorf("expected 30, got %v", v)
	}
}

// --- Table as function argument (by reference) ---

func TestTableByReference(t *testing.T) {
	v := getGlobal(t, `
		func addField(tbl, key, val) {
			tbl[key] = val
		}
		t := {}
		addField(t, "x", 42)
		result := t.x
	`, "result")
	if !v.IsInt() || v.Int() != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

func TestTableMutationInFunction(t *testing.T) {
	v := getGlobal(t, `
		func increment(tbl) {
			tbl.n = tbl.n + 1
		}
		t := {n: 0}
		increment(t)
		increment(t)
		increment(t)
		result := t.n
	`, "result")
	if !v.IsInt() || v.Int() != 3 {
		t.Errorf("expected 3, got %v", v)
	}
}

// --- Table mutation in closures ---

func TestTableMutationInClosure(t *testing.T) {
	v := getGlobal(t, `
		t := {n: 0}
		inc := func() {
			t.n = t.n + 1
		}
		inc()
		inc()
		result := t.n
	`, "result")
	if !v.IsInt() || v.Int() != 2 {
		t.Errorf("expected 2, got %v", v)
	}
}

// --- Table iteration ---

func TestTableForRangeArray(t *testing.T) {
	v := getGlobal(t, `
		t := {10, 20, 30, 40}
		sum := 0
		for k, v := range t {
			sum = sum + v
		}
		result := sum
	`, "result")
	if !v.IsInt() || v.Int() != 100 {
		t.Errorf("expected 100, got %v", v)
	}
}

func TestTableForRangeDict(t *testing.T) {
	v := getGlobal(t, `
		t := {a: 1, b: 2, c: 3}
		sum := 0
		for k, v := range t {
			sum = sum + v
		}
		result := sum
	`, "result")
	if !v.IsInt() || v.Int() != 6 {
		t.Errorf("expected 6, got %v", v)
	}
}

func TestTableIpairsStopsAtHole(t *testing.T) {
	// ipairs iterates consecutive integer keys from 1
	interp := runProgram(t, `
		t := {10, 20, 30}
		count := 0
		for i, v := range ipairs(t) {
			count = count + 1
		}
	`)
	count := interp.GetGlobal("count")
	if count.Int() != 3 {
		t.Errorf("expected 3 iterations, got %v", count)
	}
}

// --- Table as key ---

func TestTableStringKeyVsBracket(t *testing.T) {
	interp := runProgram(t, `
		t := {}
		t.x = 10
		t["x"] = 20
		r1 := t.x
		r2 := t["x"]
	`)
	r1 := interp.GetGlobal("r1")
	r2 := interp.GetGlobal("r2")
	// t.x and t["x"] should be the same
	if r1.Int() != 20 || r2.Int() != 20 {
		t.Errorf("expected both 20, got r1=%v, r2=%v", r1, r2)
	}
}

func TestTableNumericStringKeyDifference(t *testing.T) {
	interp := runProgram(t, `
		t := {}
		t[1] = "numeric"
		t["1"] = "string"
		r1 := t[1]
		r2 := t["1"]
	`)
	r1 := interp.GetGlobal("r1")
	r2 := interp.GetGlobal("r2")
	if r1.Str() != "numeric" {
		t.Errorf("expected r1='numeric', got %v", r1)
	}
	if r2.Str() != "string" {
		t.Errorf("expected r2='string', got %v", r2)
	}
}

// --- Table nil access ---

func TestTableNilFieldReturnsNil(t *testing.T) {
	v := getGlobal(t, `
		t := {x: 10}
		result := t.y
	`, "result")
	if !v.IsNil() {
		t.Errorf("expected nil for missing field, got %v", v)
	}
}

func TestTableNilIndexReturnsNil(t *testing.T) {
	v := getGlobal(t, `
		t := {10, 20}
		result := t[5]
	`, "result")
	if !v.IsNil() {
		t.Errorf("expected nil for missing index, got %v", v)
	}
}

func TestPlainArrayInsertRemoveFastPath(t *testing.T) {
	tbl := NewTableSizedKind(4, 0, ArrayInt)
	for i := int64(1); i <= 4; i++ {
		tbl.RawSetInt(i, IntValue(i*10))
	}
	if !tbl.TryPlainArrayInsert(2, IntValue(99)) {
		t.Fatal("TryPlainArrayInsert did not handle dense int array")
	}
	wantAfterInsert := []int64{10, 99, 20, 30, 40}
	for i, want := range wantAfterInsert {
		got := tbl.RawGetInt(int64(i + 1))
		if !got.IsInt() || got.Int() != want {
			t.Fatalf("after insert index %d = %v, want %d", i+1, got, want)
		}
	}
	removed, ok := tbl.TryPlainArrayRemove(3)
	if !ok {
		t.Fatal("TryPlainArrayRemove did not handle dense int array")
	}
	if !removed.IsInt() || removed.Int() != 20 {
		t.Fatalf("removed = %v, want 20", removed)
	}
	wantAfterRemove := []int64{10, 99, 30, 40}
	for i, want := range wantAfterRemove {
		got := tbl.RawGetInt(int64(i + 1))
		if !got.IsInt() || got.Int() != want {
			t.Fatalf("after remove index %d = %v, want %d", i+1, got, want)
		}
	}
	if got := tbl.Length(); got != len(wantAfterRemove) {
		t.Fatalf("length = %d, want %d", got, len(wantAfterRemove))
	}
}

func TestPlainArraySortFastPath(t *testing.T) {
	ints := NewTableSizedKind(5, 0, ArrayInt)
	for i, v := range []int64{5, 1, 4, 2, 3} {
		ints.RawSetInt(int64(i+1), IntValue(v))
	}
	if !ints.TryPlainArraySort(5) {
		t.Fatal("TryPlainArraySort did not handle dense int array")
	}
	for i, want := range []int64{1, 2, 3, 4, 5} {
		got := ints.RawGetInt(int64(i + 1))
		if !got.IsInt() || got.Int() != want {
			t.Fatalf("sorted int index %d = %v, want %d", i+1, got, want)
		}
	}

	floats := NewTableSizedKind(3, 0, ArrayFloat)
	for i, v := range []float64{2.5, -1.0, 1.25} {
		floats.RawSetInt(int64(i+1), FloatValue(v))
	}
	if !floats.TryPlainArraySort(3) {
		t.Fatal("TryPlainArraySort did not handle dense float array")
	}
	for i, want := range []float64{-1.0, 1.25, 2.5} {
		got := floats.RawGetInt(int64(i + 1))
		if !got.IsFloat() || got.Float() != want {
			t.Fatalf("sorted float index %d = %v, want %g", i+1, got, want)
		}
	}

	withMeta := NewTableSizedKind(2, 0, ArrayInt)
	withMeta.RawSetInt(1, IntValue(2))
	withMeta.RawSetInt(2, IntValue(1))
	withMeta.SetMetatable(NewTable())
	if withMeta.TryPlainArraySort(2) {
		t.Fatal("TryPlainArraySort should not bypass metatable-bearing tables")
	}
}

// --- Table with function values ---

func TestTableWithFunctions(t *testing.T) {
	v := getGlobal(t, `
		ops := {
			add: func(a, b) { return a + b },
			mul: func(a, b) { return a * b }
		}
		result := ops.add(3, 4) + ops.mul(5, 6)
	`, "result")
	if !v.IsInt() || v.Int() != 37 {
		t.Errorf("expected 37, got %v", v)
	}
}

// --- Table chained field assignment ---

func TestTableChainedFieldAssignment(t *testing.T) {
	v := getGlobal(t, `
		t := {inner: {}}
		t.inner.x = 42
		result := t.inner.x
	`, "result")
	if !v.IsInt() || v.Int() != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

// --- Table identity ---

func TestTableIdentityAfterAssignment(t *testing.T) {
	v := getGlobal(t, `
		a := {x: 1}
		b := a
		b.x = 2
		result := a.x
	`, "result")
	// b is same reference as a
	if !v.IsInt() || v.Int() != 2 {
		t.Errorf("expected 2, got %v", v)
	}
}

func TestTableDifferentInstances(t *testing.T) {
	v := getGlobal(t, `
		a := {x: 1}
		b := {x: 1}
		result := a == b
	`, "result")
	if v.Bool() {
		t.Errorf("different tables should not be equal")
	}
}
