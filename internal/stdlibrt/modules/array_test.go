package modules

import "testing"

func TestArrayStdlibSumFastArg(t *testing.T) {
	lib := BuildArray()
	sum := lib.RawGetString("sum").GoFunction()
	if sum == nil || sum.FastArg1 == nil {
		t.Fatal("array.sum FastArg1 is nil")
	}
	got, err := sum.FastArg1(DenseArrayValue(NewDenseArrayF64([]float64{1.5, 2.5, 3})))
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsFloat() || got.Float() != 7 {
		t.Fatalf("array.sum f64 = %s, want 7", got.String())
	}
	got, err = sum.FastArg1(DenseArrayValue(NewDenseArrayI64([]int64{1, 2, 3})))
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsInt() || got.Int() != 6 {
		t.Fatalf("array.sum i64 = %s, want 6", got.String())
	}
}
