package data

import (
	"reflect"
	"testing"
)

func TestTryProjectByI64IndexArrayProjectsCarrierArrays(t *testing.T) {
	indexes := NewI64([]int64{3, 0, 2})

	t.Run("encoded", func(t *testing.T) {
		values := NewEncodedSymbols([]Symbol{"AAPL", "MSFT", "NVDA", "AAPL"})
		projected, handled, err := TryProjectByI64IndexArray(values, indexes)
		if err != nil || !handled {
			t.Fatalf("TryProjectByI64IndexArray encoded handled=%v err=%v", handled, err)
		}
		if _, ok := projected.(encodedArray); !ok {
			t.Fatalf("encoded project returned %T, want encodedArray", projected)
		}
		if got, want := projected.Values(), []any{Symbol("AAPL"), Symbol("AAPL"), Symbol("NVDA")}; !reflect.DeepEqual(got, want) {
			t.Fatalf("encoded values = %#v, want %#v", got, want)
		}
	})

	t.Run("nullable", func(t *testing.T) {
		values := nullableArray{kind: KindSymbol, data: []any{Symbol("a"), NullValue, Symbol("c"), Symbol("d")}}
		projected, handled, err := TryProjectByI64IndexArray(values, indexes)
		if err != nil || !handled {
			t.Fatalf("TryProjectByI64IndexArray nullable handled=%v err=%v", handled, err)
		}
		if _, ok := projected.(nullableArray); !ok {
			t.Fatalf("nullable project returned %T, want nullableArray", projected)
		}
		if got, want := projected.Values(), []any{Symbol("d"), Symbol("a"), Symbol("c")}; !reflect.DeepEqual(got, want) {
			t.Fatalf("nullable values = %#v, want %#v", got, want)
		}
	})

	t.Run("null bitmap", func(t *testing.T) {
		values, err := nullableArrayWithKind(KindI64, []any{int64(10), NullValue, int64(30), int64(40)})
		if err != nil {
			t.Fatalf("nullableArrayWithKind: %v", err)
		}
		if _, ok := values.(nullBitmapArray[int64]); !ok {
			t.Fatalf("setup returned %T, want nullBitmapArray[int64]", values)
		}
		projected, handled, err := TryProjectByI64IndexArray(values, NewI64([]int64{1, 3, 0}))
		if err != nil || !handled {
			t.Fatalf("TryProjectByI64IndexArray null bitmap handled=%v err=%v", handled, err)
		}
		if _, ok := projected.(nullBitmapArray[int64]); !ok {
			t.Fatalf("null bitmap project returned %T, want nullBitmapArray[int64]", projected)
		}
		if got, want := projected.Values(), []any{NullValue, int64(40), int64(10)}; !reflect.DeepEqual(got, want) {
			t.Fatalf("null bitmap values = %#v, want %#v", got, want)
		}
	})
}

func TestTryProjectByI64IndexArrayProjectsDerivedCarriers(t *testing.T) {
	indexes := NewI64([]int64{3, 0, 2})

	t.Run("tiled encoded", func(t *testing.T) {
		source := NewEncodedSymbols([]Symbol{"AAPL", "MSFT", "NVDA"})
		values := tiledArray{source: source, start: 1, len: 5}
		projected, handled, err := TryProjectByI64IndexArray(values, indexes)
		if err != nil || !handled {
			t.Fatalf("TryProjectByI64IndexArray tiled handled=%v err=%v", handled, err)
		}
		if _, ok := projected.(encodedArray); !ok {
			t.Fatalf("tiled encoded project returned %T, want encodedArray", projected)
		}
		if got, want := projected.Values(), []any{Symbol("MSFT"), Symbol("MSFT"), Symbol("AAPL")}; !reflect.DeepEqual(got, want) {
			t.Fatalf("tiled encoded values = %#v, want %#v", got, want)
		}
	})

	t.Run("shifted", func(t *testing.T) {
		values := shiftedArray{source: NewI64([]int64{10, 20, 30, 40}), offset: -1}
		projected, handled, err := TryProjectByI64IndexArray(values, indexes)
		if err != nil || !handled {
			t.Fatalf("TryProjectByI64IndexArray shifted handled=%v err=%v", handled, err)
		}
		if _, ok := projected.(nullableArray); !ok {
			t.Fatalf("shifted project returned %T, want nullableArray", projected)
		}
		if got, want := projected.Values(), []any{int64(30), NullValue, int64(20)}; !reflect.DeepEqual(got, want) {
			t.Fatalf("shifted values = %#v, want %#v", got, want)
		}
	})

	t.Run("fills", func(t *testing.T) {
		source := nullableArray{kind: KindI64, data: []any{int64(10), NullValue, int64(30), NullValue}}
		values := i64FillArray{source: source, fill: 7}
		projected, handled, err := TryProjectByI64IndexArray(values, indexes)
		if err != nil || !handled {
			t.Fatalf("TryProjectByI64IndexArray fill handled=%v err=%v", handled, err)
		}
		if _, ok := projected.(columnArray[int64]); !ok {
			t.Fatalf("fill project returned %T, want columnArray[int64]", projected)
		}
		if got, want := projected.Values(), []any{int64(7), int64(10), int64(30)}; !reflect.DeepEqual(got, want) {
			t.Fatalf("fill values = %#v, want %#v", got, want)
		}
	})
}

func TestTryProjectByI64IndexArrayLeavesPlainColumnsForLazyGather(t *testing.T) {
	projected, handled, err := TryProjectByI64IndexArray(NewI64([]int64{10, 20, 30}), NewI64([]int64{2, 0}))
	if err != nil {
		t.Fatalf("TryProjectByI64IndexArray plain column err=%v", err)
	}
	if handled || projected != nil {
		t.Fatalf("plain column project = %T,%v; want unhandled nil,false", projected, handled)
	}
}
