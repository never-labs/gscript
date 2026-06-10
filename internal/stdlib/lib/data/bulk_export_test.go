package data

import (
	"reflect"
	"testing"
)

func TestTryExportI64CopyTypedCarriers(t *testing.T) {
	bucketed, err := BucketFloor(NewI64Range(0, 5, 5), int64(10))
	if err != nil {
		t.Fatalf("BucketFloor returned error: %v", err)
	}
	cases := []struct {
		name  string
		array Array
		want  []int64
	}{
		{"i64 column", NewI64([]int64{10, -2, 30}), []int64{10, -2, 30}},
		{"i32 column", NewI32([]int32{1, 2, -3}), []int64{1, 2, -3}},
		{"i64 range", NewI64Range(5, 3, 4), []int64{5, 8, 11, 14}},
		{"i64 running sum", i64RunningSumArray{source: i64RangeArray{start: 1, step: 1, len: 4}}, []int64{1, 3, 6, 10}},
		{"i64 product", i64ProductArray{left: i64RangeArray{start: 1, step: 1, len: 4}, right: i64RangeArray{start: 10, step: 10, len: 4}}, []int64{10, 40, 90, 160}},
		{"i64 bucket", bucketed, []int64{0, 0, 10, 10, 20}},
		{"i64 fill", i64FillArray{source: nullableArray{kind: KindI64, data: []any{int64(1), NullValue, int64(3)}}, fill: 7}, []int64{1, 7, 3}},
		{"indexed i64 range", indexedArray{source: NewI64Range(10, 2, 6), indexes: NewI64([]int64{3, 1, 4}), len: 3}, []int64{16, 12, 18}},
		{"indexed i64 segment", indexedArray{source: newI64SegmentArray(i64RangeArray{start: 0, step: 1, len: 3}, i64RangeArray{start: 10, step: 2, len: 3}), indexes: NewI64Range(1, 2, 3), len: 3}, []int64{1, 10, 14}},
		{"nullable no null", nullableArray{kind: KindI64, data: []any{int8(1), int32(2), int64(3)}}, []int64{1, 2, 3}},
		{"shifted zero offset", shiftedArray{source: NewI64([]int64{4, 5}), offset: 0}, []int64{4, 5}},
		{"attributed", WithArrayAttribute(NewI64([]int64{7, 8}), ArrayAttributeSorted), []int64{7, 8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := make([]int64, tc.array.Len())
			handled, err := TryExportI64Copy(tc.array, got)
			if err != nil {
				t.Fatalf("TryExportI64Copy returned error: %v", err)
			}
			if !handled {
				t.Fatal("TryExportI64Copy did not handle typed carrier")
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("TryExportI64Copy = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTryExportTypedCopyFloatBoolString(t *testing.T) {
	floatBucket, err := BucketFloor(NewF64([]float64{1.2, 2.9, 3.1}), float64(1))
	if err != nil {
		t.Fatalf("BucketFloor float returned error: %v", err)
	}
	floats := make([]float64, 3)
	handled, err := TryExportF64Copy(NewF32([]float32{1.5, -2, 3.25}), floats)
	if err != nil || !handled {
		t.Fatalf("TryExportF64Copy = %v, %v; want handled nil error", handled, err)
	}
	if want := []float64{1.5, -2, 3.25}; !reflect.DeepEqual(floats, want) {
		t.Fatalf("TryExportF64Copy = %v, want %v", floats, want)
	}
	for _, tc := range []struct {
		name  string
		array Array
		want  []float64
	}{
		{"indexed f64", indexedArray{source: NewF64([]float64{1.5, 2.5, 3.5}), indexes: NewI64([]int64{2, 0}), len: 2}, []float64{3.5, 1.5}},
		{"f64 bucket", floatBucket, []float64{1, 2, 3}},
		{"f64 fill", f64FillArray{source: nullableArray{kind: KindF64, data: []any{float64(1.25), NullValue, int64(3)}}, fill: 2.5}, []float64{1.25, 2.5, 3}},
		{"nullable numeric", nullableArray{kind: KindF64, data: []any{float32(1.5), int64(2), float64(3.25)}}, []float64{1.5, 2, 3.25}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := make([]float64, tc.array.Len())
			handled, err := TryExportF64Copy(tc.array, got)
			if err != nil || !handled {
				t.Fatalf("TryExportF64Copy = %v, %v; want handled nil error", handled, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("TryExportF64Copy = %v, want %v", got, tc.want)
			}
		})
	}

	bools := make([]bool, 3)
	handled, err = TryExportBoolCopy(NewBool([]bool{true, false, true}), bools)
	if err != nil || !handled {
		t.Fatalf("TryExportBoolCopy = %v, %v; want handled nil error", handled, err)
	}
	if want := []bool{true, false, true}; !reflect.DeepEqual(bools, want) {
		t.Fatalf("TryExportBoolCopy = %v, want %v", bools, want)
	}
	for _, tc := range []struct {
		name  string
		array Array
		want  []bool
	}{
		{"indexed bool", indexedArray{source: NewBool([]bool{true, false, true}), indexes: NewI64([]int64{1, 2}), len: 2}, []bool{false, true}},
		{"nullable bool", nullableArray{kind: KindBool, data: []any{true, false, true}}, []bool{true, false, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := make([]bool, tc.array.Len())
			handled, err := TryExportBoolCopy(tc.array, got)
			if err != nil || !handled {
				t.Fatalf("TryExportBoolCopy = %v, %v; want handled nil error", handled, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("TryExportBoolCopy = %v, want %v", got, tc.want)
			}
		})
	}

	strings := make([]string, 2)
	handled, err = TryExportStringCopy(NewString([]string{"AAPL", "MSFT"}), strings)
	if err != nil || !handled {
		t.Fatalf("TryExportStringCopy = %v, %v; want handled nil error", handled, err)
	}
	if want := []string{"AAPL", "MSFT"}; !reflect.DeepEqual(strings, want) {
		t.Fatalf("TryExportStringCopy = %v, want %v", strings, want)
	}
	for _, tc := range []struct {
		name  string
		array Array
		want  []string
	}{
		{"indexed string", indexedArray{source: NewString([]string{"AAPL", "MSFT", "NVDA"}), indexes: NewI64([]int64{2, 0}), len: 2}, []string{"NVDA", "AAPL"}},
		{"nullable string", nullableArray{kind: KindString, data: []any{"bid", "ask"}}, []string{"bid", "ask"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := make([]string, tc.array.Len())
			handled, err := TryExportStringCopy(tc.array, got)
			if err != nil || !handled {
				t.Fatalf("TryExportStringCopy = %v, %v; want handled nil error", handled, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("TryExportStringCopy = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTryExportTypedCopyRejectsUnsupportedAndBadDestination(t *testing.T) {
	dst := make([]int64, 2)
	handled, err := TryExportI64Copy(NewColumn("x", []any{int64(1), NullValue}).Data, dst)
	if err != nil {
		t.Fatalf("TryExportI64Copy unsupported returned error: %v", err)
	}
	if handled {
		t.Fatal("TryExportI64Copy handled nullable array; want generic fallback")
	}

	handled, err = TryExportI64Copy(shiftedArray{source: NewI64([]int64{1, 2, 3}), offset: 1}, make([]int64, 3))
	if err != nil {
		t.Fatalf("TryExportI64Copy shifted returned error: %v", err)
	}
	if handled {
		t.Fatal("TryExportI64Copy handled shifted array with boundary nulls; want generic fallback")
	}

	handled, err = TryExportI64Copy(NewI64([]int64{1, 2, 3}), dst)
	if err == nil || !handled {
		t.Fatalf("TryExportI64Copy short dst = %v, %v; want handled error", handled, err)
	}
}
