package runtime

import (
	"errors"
	"reflect"
	"testing"
)

func TestDenseArrayValueBasics(t *testing.T) {
	arr := NewDenseArrayF64([]float64{1.5, 2, 3.25})
	v := DenseArrayValue(arr)
	if v.Type() != TypeDenseArray {
		t.Fatalf("Type() = %v, want TypeDenseArray", v.Type())
	}
	if v.TypeName() != "array" {
		t.Fatalf("TypeName() = %q, want array", v.TypeName())
	}
	if !v.IsDenseArray() || v.DenseArray() != arr {
		t.Fatalf("DenseArray accessor did not recover original array")
	}
	if arr.DType() != DenseArrayF64 || arr.Len() != 3 {
		t.Fatalf("dtype/len = %s/%d, want f64/3", arr.DType(), arr.Len())
	}
	if got := v.String(); got != "array<f64>[1.5, 2, 3.25]" {
		t.Fatalf("String() = %q", got)
	}
	if p, ok := v.Ptr().(*DenseArray); !ok || p != arr {
		t.Fatalf("Ptr() = %#v, want original *DenseArray", v.Ptr())
	}
}

func TestDenseArrayConstructorsCopyInput(t *testing.T) {
	ints := []int64{1, 2, 3}
	bools := []bool{true, false}
	ia := NewDenseArrayI64(ints)
	ba := NewDenseArrayBool(bools)
	ints[0] = 99
	bools[0] = false

	gotInts, ok := ia.I64()
	if !ok || !reflect.DeepEqual(gotInts, []int64{1, 2, 3}) {
		t.Fatalf("I64() = %v/%v", gotInts, ok)
	}
	gotBools, ok := ba.Bool()
	if !ok || !reflect.DeepEqual(gotBools, []bool{true, false}) {
		t.Fatalf("Bool() = %v/%v", gotBools, ok)
	}
}

func TestDenseArrayElementwiseArrayArrayI64(t *testing.T) {
	left := DenseArrayValue(NewDenseArrayI64([]int64{1, 2, 3}))
	right := DenseArrayValue(NewDenseArrayI64([]int64{10, 20, 30}))
	got, err := DenseArrayElementwise(DenseArrayAdd, left, right)
	if err != nil {
		t.Fatalf("DenseArrayElementwise add error: %v", err)
	}
	assertDenseI64(t, got, []int64{11, 22, 33})

	got, err = DenseArrayElementwise(DenseArrayGT, right, left)
	if err != nil {
		t.Fatalf("DenseArrayElementwise gt error: %v", err)
	}
	assertDenseBool(t, got, []bool{true, true, true})
}

func TestDenseArrayElementwiseArrayArrayMixedNumeric(t *testing.T) {
	left := DenseArrayValue(NewDenseArrayI64([]int64{2, 5, 9}))
	right := DenseArrayValue(NewDenseArrayF64([]float64{0.5, 2.5, 3}))
	got, err := DenseArrayElementwise(DenseArrayDiv, left, right)
	if err != nil {
		t.Fatalf("DenseArrayElementwise div error: %v", err)
	}
	assertDenseF64(t, got, []float64{4, 2, 3})
}

func TestDenseArrayElementwiseArrayScalarAndScalarArray(t *testing.T) {
	arr := DenseArrayValue(NewDenseArrayF64([]float64{1, 2, 4}))
	got, err := DenseArrayElementwise(DenseArrayMul, arr, FloatValue(2.5))
	if err != nil {
		t.Fatalf("array-scalar mul error: %v", err)
	}
	assertDenseF64(t, got, []float64{2.5, 5, 10})

	got, err = DenseArrayElementwise(DenseArraySub, IntValue(10), arr)
	if err != nil {
		t.Fatalf("scalar-array sub error: %v", err)
	}
	assertDenseF64(t, got, []float64{9, 8, 6})
}

func TestDenseArrayElementwiseBoolComparisons(t *testing.T) {
	left := DenseArrayValue(NewDenseArrayBool([]bool{true, false, true}))
	right := DenseArrayValue(NewDenseArrayBool([]bool{true, true, false}))
	got, err := DenseArrayElementwise(DenseArrayEQ, left, right)
	if err != nil {
		t.Fatalf("bool eq error: %v", err)
	}
	assertDenseBool(t, got, []bool{true, false, false})

	got, err = DenseArrayElementwise(DenseArrayNE, BoolValue(false), left)
	if err != nil {
		t.Fatalf("bool scalar-array ne error: %v", err)
	}
	assertDenseBool(t, got, []bool{true, false, true})
}

func TestDenseArrayElementwiseLengthMismatch(t *testing.T) {
	_, err := DenseArrayElementwise(
		DenseArrayAdd,
		DenseArrayValue(NewDenseArrayI64([]int64{1, 2})),
		DenseArrayValue(NewDenseArrayI64([]int64{1})),
	)
	if !errors.Is(err, ErrDenseArrayLength) {
		t.Fatalf("error = %v, want ErrDenseArrayLength", err)
	}
}

func TestDenseArrayReduceI64(t *testing.T) {
	arr := NewDenseArrayI64([]int64{5, -1, 8, 2})
	cases := []struct {
		op   DenseArrayReduceOp
		want Value
	}{
		{DenseArrayReduceSum, IntValue(14)},
		{DenseArrayReduceMin, IntValue(-1)},
		{DenseArrayReduceMax, IntValue(8)},
		{DenseArrayReduceMean, FloatValue(3.5)},
	}
	for _, tc := range cases {
		got, err := DenseArrayReduce(tc.op, arr)
		if err != nil {
			t.Fatalf("DenseArrayReduce(%d) error: %v", tc.op, err)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("DenseArrayReduce(%d) = %v, want %v", tc.op, got, tc.want)
		}
	}
}

func TestDenseArrayReduceF64(t *testing.T) {
	arr := NewDenseArrayF64([]float64{1.5, -2, 6.5})
	cases := []struct {
		op   DenseArrayReduceOp
		want Value
	}{
		{DenseArrayReduceSum, FloatValue(6)},
		{DenseArrayReduceMin, FloatValue(-2)},
		{DenseArrayReduceMax, FloatValue(6.5)},
		{DenseArrayReduceMean, FloatValue(2)},
	}
	for _, tc := range cases {
		got, err := DenseArrayReduce(tc.op, arr)
		if err != nil {
			t.Fatalf("DenseArrayReduce(%d) error: %v", tc.op, err)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("DenseArrayReduce(%d) = %v, want %v", tc.op, got, tc.want)
		}
	}
}

func TestDenseArrayReduceRejectsBoolAndEmpty(t *testing.T) {
	if _, err := DenseArrayReduce(DenseArrayReduceSum, NewDenseArrayBool([]bool{true})); !errors.Is(err, ErrDenseArrayDType) {
		t.Fatalf("bool reduce error = %v, want ErrDenseArrayDType", err)
	}
	if _, err := DenseArrayReduce(DenseArrayReduceSum, NewDenseArrayF64(nil)); !errors.Is(err, ErrDenseArrayEmpty) {
		t.Fatalf("empty reduce error = %v, want ErrDenseArrayEmpty", err)
	}
}

func assertDenseI64(t *testing.T, v Value, want []int64) {
	t.Helper()
	arr := v.DenseArray()
	if arr == nil || arr.DType() != DenseArrayI64 {
		t.Fatalf("got %v, want i64 dense array", v)
	}
	got, _ := arr.I64()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("i64 array = %v, want %v", got, want)
	}
}

func assertDenseF64(t *testing.T, v Value, want []float64) {
	t.Helper()
	arr := v.DenseArray()
	if arr == nil || arr.DType() != DenseArrayF64 {
		t.Fatalf("got %v, want f64 dense array", v)
	}
	got, _ := arr.F64()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("f64 array = %v, want %v", got, want)
	}
}

func assertDenseBool(t *testing.T, v Value, want []bool) {
	t.Helper()
	arr := v.DenseArray()
	if arr == nil || arr.DType() != DenseArrayBool {
		t.Fatalf("got %v, want bool dense array", v)
	}
	got, _ := arr.Bool()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bool array = %v, want %v", got, want)
	}
}
