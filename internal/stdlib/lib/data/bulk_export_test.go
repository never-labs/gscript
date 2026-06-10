package data

import (
	"reflect"
	"testing"
)

func TestTryExportI64CopyTypedCarriers(t *testing.T) {
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
	floats := make([]float64, 3)
	handled, err := TryExportF64Copy(NewF32([]float32{1.5, -2, 3.25}), floats)
	if err != nil || !handled {
		t.Fatalf("TryExportF64Copy = %v, %v; want handled nil error", handled, err)
	}
	if want := []float64{1.5, -2, 3.25}; !reflect.DeepEqual(floats, want) {
		t.Fatalf("TryExportF64Copy = %v, want %v", floats, want)
	}

	bools := make([]bool, 3)
	handled, err = TryExportBoolCopy(NewBool([]bool{true, false, true}), bools)
	if err != nil || !handled {
		t.Fatalf("TryExportBoolCopy = %v, %v; want handled nil error", handled, err)
	}
	if want := []bool{true, false, true}; !reflect.DeepEqual(bools, want) {
		t.Fatalf("TryExportBoolCopy = %v, want %v", bools, want)
	}

	strings := make([]string, 2)
	handled, err = TryExportStringCopy(NewString([]string{"AAPL", "MSFT"}), strings)
	if err != nil || !handled {
		t.Fatalf("TryExportStringCopy = %v, %v; want handled nil error", handled, err)
	}
	if want := []string{"AAPL", "MSFT"}; !reflect.DeepEqual(strings, want) {
		t.Fatalf("TryExportStringCopy = %v, want %v", strings, want)
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

	handled, err = TryExportI64Copy(NewI64([]int64{1, 2, 3}), dst)
	if err == nil || !handled {
		t.Fatalf("TryExportI64Copy short dst = %v, %v; want handled error", handled, err)
	}
}
