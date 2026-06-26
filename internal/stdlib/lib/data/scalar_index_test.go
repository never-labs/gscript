package data

import (
	"reflect"
	"testing"
)

func TestTryTypedScalarIndexCoversColumnarCarriers(t *testing.T) {
	encoded, err := NewEncoded(KindSymbol, []any{Symbol("AAPL"), Symbol("MSFT")}, []int32{0, 1})
	if err != nil {
		t.Fatalf("NewEncoded: %v", err)
	}
	gathered, err := Gather(NewI64Range(10, 2, 8), []int{3, 1, 4})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	filled, handled, err := TryTypedFills(NewColumn("x", []any{int64(1), NullValue, int64(3)}).Data)
	if err != nil || !handled {
		t.Fatalf("TryTypedFills = %T,%v,%v; want handled nil error", filled, handled, err)
	}
	crossed := Cross(NewI64([]int64{1, 2}), NewI64([]int64{10, 20, 30}))

	tests := []struct {
		name  string
		array Array
		row   int
		want  any
	}{
		{name: "i64 column", array: NewI64([]int64{10, 20, 30}), row: 1, want: int64(20)},
		{name: "f64 column", array: NewF64([]float64{1.5, 2.5, 3.5}), row: 2, want: 3.5},
		{name: "bool column", array: NewBool([]bool{false, true}), row: 1, want: true},
		{name: "symbol column", array: NewSymbols([]string{"a", "b"}), row: 0, want: Symbol("a")},
		{name: "range", array: NewI64Range(10, 3, 5), row: 4, want: int64(22)},
		{name: "attributed range", array: WithArrayAttribute(NewI64Range(0, 2, 4), ArrayAttributeSorted), row: 3, want: int64(6)},
		{name: "gathered range", array: gathered, row: 0, want: int64(16)},
		{name: "encoded symbol", array: encoded, row: 1, want: Symbol("MSFT")},
		{name: "filled nullable", array: filled, row: 1, want: int64(1)},
		{name: "cross row", array: crossed, row: 4, want: NewI64([]int64{2, 20})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := TryTypedScalarIndex(tt.array, tt.row)
			if err != nil || !ok || !scalarIndexTestEqual(got, tt.want) {
				t.Fatalf("TryTypedScalarIndex(%s,row=%d) = %#v,%v,%v; want %#v,true,nil", tt.array.Kind(), tt.row, got, ok, err, tt.want)
			}
		})
	}
}

func TestTryTypedScalarIndexReportsRuntimeErrors(t *testing.T) {
	got, ok, err := TryTypedScalarIndex(NewI64([]int64{1}), 3)
	if err == nil || !ok || got != nil {
		t.Fatalf("out of range = %#v,%v,%v; want nil,true,error", got, ok, err)
	}

	crossed := Cross(NewI64([]int64{1}), NewI64([]int64{2}))
	got, ok, err = TryTypedScalarIndex(crossed, 1)
	if err == nil || !ok || got != nil {
		t.Fatalf("cross out of range = %#v,%v,%v; want nil,true,error", got, ok, err)
	}
}

func TestTryTypedScalarIndexI64CoversNoBoxingCarriers(t *testing.T) {
	gathered, err := Gather(NewI64Range(10, 2, 8), []int{3, 1, 4})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	tiled, handled, err := TryTypedRotate(NewI64([]int64{4, 5, 6}), -1)
	if err != nil || !handled {
		t.Fatalf("TryTypedRotate: %v,%v", handled, err)
	}

	tests := []struct {
		name  string
		array Array
		row   int
		want  int64
	}{
		{name: "i64 column", array: NewI64([]int64{10, 20, 30}), row: 1, want: 20},
		{name: "i64 range", array: NewI64Range(10, 3, 5), row: 4, want: 22},
		{name: "attributed range", array: WithArrayAttribute(NewI64Range(0, 2, 4), ArrayAttributeSorted), row: 3, want: 6},
		{name: "gathered range", array: gathered, row: 0, want: 16},
		{name: "tiled i64", array: tiled, row: 0, want: 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := TryTypedScalarIndexI64(tt.array, tt.row)
			if err != nil || !ok || got != tt.want {
				t.Fatalf("TryTypedScalarIndexI64(%s,row=%d) = %d,%v,%v; want %d,true,nil", tt.array.Kind(), tt.row, got, ok, err, tt.want)
			}
		})
	}
}

func TestTryTypedScalarIndexI64FallsBackForNullPreservingCarriers(t *testing.T) {
	shifted := shiftedArray{source: NewI64([]int64{1, 2, 3}), offset: 1}
	got, ok, err := TryTypedScalarIndexI64(shifted, 2)
	if err != nil || ok || got != 0 {
		t.Fatalf("shifted fallback = %d,%v,%v; want 0,false,nil", got, ok, err)
	}
}

func scalarIndexTestEqual(got, want any) bool {
	gotArray, gotIsArray := got.(Array)
	wantArray, wantIsArray := want.(Array)
	if gotIsArray || wantIsArray {
		if !gotIsArray || !wantIsArray {
			return false
		}
		return gotArray.Kind() == wantArray.Kind() && reflect.DeepEqual(gotArray.Values(), wantArray.Values())
	}
	return got == want
}
