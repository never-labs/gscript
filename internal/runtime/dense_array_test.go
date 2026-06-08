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

func TestDenseArrayMaskCombine(t *testing.T) {
	left := DenseArrayValue(NewDenseArrayBool([]bool{true, false, true, false}))
	right := DenseArrayValue(NewDenseArrayBool([]bool{true, true, false, false}))

	got, err := DenseArrayMaskCombine(DenseArrayMaskAnd, left, right)
	if err != nil {
		t.Fatalf("mask and error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{true, false, false, false})

	got, err = DenseArrayMaskCombine(DenseArrayMaskOr, left, BoolValue(false))
	if err != nil {
		t.Fatalf("mask array-scalar or error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{true, false, true, false})

	got, err = DenseArrayMaskCombine(DenseArrayMaskAndNot, BoolValue(true), right)
	if err != nil {
		t.Fatalf("mask scalar-array andNot error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{false, false, true, true})

	got, err = DenseArrayMaskCombine(DenseArrayMaskXor, left, right)
	if err != nil {
		t.Fatalf("mask xor error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{false, true, true, false})
}

func TestDenseArrayMaskCombineRejectsNonMaskOperands(t *testing.T) {
	_, err := DenseArrayMaskCombine(
		DenseArrayMaskAnd,
		DenseArrayValue(NewDenseArrayBool([]bool{true, false})),
		DenseArrayValue(NewDenseArrayBool([]bool{true})),
	)
	if !errors.Is(err, ErrDenseArrayLength) {
		t.Fatalf("length mismatch error = %v, want ErrDenseArrayLength", err)
	}
	_, err = DenseArrayMaskCombine(
		DenseArrayMaskAnd,
		DenseArrayValue(NewDenseArrayI64([]int64{1})),
		BoolValue(true),
	)
	if !errors.Is(err, ErrDenseArrayDType) {
		t.Fatalf("dtype error = %v, want ErrDenseArrayDType", err)
	}
	_, err = DenseArrayMaskCombine(DenseArrayMaskAnd, BoolValue(true), BoolValue(false))
	if !errors.Is(err, ErrDenseArrayOperand) {
		t.Fatalf("operand error = %v, want ErrDenseArrayOperand", err)
	}
	_, err = DenseArrayMaskCombine(DenseArrayMaskOp(255), DenseArrayValue(NewDenseArrayBool([]bool{true})), BoolValue(true))
	if !errors.Is(err, ErrDenseArrayMaskOp) {
		t.Fatalf("mask op error = %v, want ErrDenseArrayMaskOp", err)
	}
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

func TestDenseArrayMaskCombineArrayArray(t *testing.T) {
	left := DenseArrayValue(NewDenseArrayBool([]bool{true, true, false, false}))
	right := DenseArrayValue(NewDenseArrayBool([]bool{true, false, true, false}))

	got, err := DenseArrayMaskCombine(DenseArrayMaskAnd, left, right)
	if err != nil {
		t.Fatalf("DenseArrayMaskCombine and error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{true, false, false, false})

	got, err = DenseArrayMaskCombine(DenseArrayMaskOr, left, right)
	if err != nil {
		t.Fatalf("DenseArrayMaskCombine or error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{true, true, true, false})

	got, err = DenseArrayMaskCombine(DenseArrayMaskXor, left, right)
	if err != nil {
		t.Fatalf("DenseArrayMaskCombine xor error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{false, true, true, false})

	got, err = DenseArrayMaskCombine(DenseArrayMaskAndNot, left, right)
	if err != nil {
		t.Fatalf("DenseArrayMaskCombine andNot error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{false, true, false, false})
}

func TestDenseArrayMaskCombineArrayScalar(t *testing.T) {
	mask := DenseArrayValue(NewDenseArrayBool([]bool{true, false, true}))

	got, err := DenseArrayMaskCombine(DenseArrayMaskAnd, mask, BoolValue(false))
	if err != nil {
		t.Fatalf("DenseArrayMaskCombine mask&&false error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{false, false, false})

	got, err = DenseArrayMaskCombine(DenseArrayMaskAndNot, BoolValue(true), mask)
	if err != nil {
		t.Fatalf("DenseArrayMaskCombine true&&!mask error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{false, true, false})
}

func TestDenseArrayMaskCombineRejectsInvalidOperands(t *testing.T) {
	_, err := DenseArrayMaskCombine(
		DenseArrayMaskAnd,
		DenseArrayValue(NewDenseArrayBool([]bool{true, false})),
		DenseArrayValue(NewDenseArrayBool([]bool{true})),
	)
	if !errors.Is(err, ErrDenseArrayLength) {
		t.Fatalf("length mismatch error = %v, want ErrDenseArrayLength", err)
	}

	_, err = DenseArrayMaskCombine(
		DenseArrayMaskAnd,
		DenseArrayValue(NewDenseArrayBool([]bool{true})),
		DenseArrayValue(NewDenseArrayI64([]int64{1})),
	)
	if !errors.Is(err, ErrDenseArrayDType) {
		t.Fatalf("dtype error = %v, want ErrDenseArrayDType", err)
	}

	_, err = DenseArrayMaskCombine(DenseArrayMaskOp(99), DenseArrayValue(NewDenseArrayBool([]bool{true})), BoolValue(true))
	if !errors.Is(err, ErrDenseArrayMaskOp) {
		t.Fatalf("op error = %v, want ErrDenseArrayMaskOp", err)
	}
}

func TestDenseArrayWhereExportsTypedSelect(t *testing.T) {
	mask := NewDenseArrayBool([]bool{true, false, true})
	got, err := DenseArrayWhere(mask, DenseArrayValue(NewDenseArrayI64([]int64{10, 20, 30})), IntValue(7))
	if err != nil {
		t.Fatalf("DenseArrayWhere error: %v", err)
	}
	assertDenseI64(t, DenseArrayValue(got), []int64{10, 7, 30})

	if _, err := DenseArrayWhere(NewDenseArrayBool([]bool{true}), DenseArrayValue(NewDenseArrayI64([]int64{10, 20})), IntValue(7)); !errors.Is(err, ErrDenseArrayLength) {
		t.Fatalf("DenseArrayWhere length error = %v, want ErrDenseArrayLength", err)
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

func TestDenseArrayIndexFromValue(t *testing.T) {
	idx, ok, err := DenseArrayIndexFromValue(IntValue(2), 3)
	if err != nil || !ok || idx != 1 {
		t.Fatalf("int index = %d/%v/%v, want 1/true/nil", idx, ok, err)
	}
	idx, ok, err = DenseArrayIndexFromValue(FloatValue(3), 3)
	if err != nil || !ok || idx != 2 {
		t.Fatalf("float integer index = %d/%v/%v, want 2/true/nil", idx, ok, err)
	}
	_, ok, err = DenseArrayIndexFromValue(FloatValue(1.5), 3)
	if err != nil || ok {
		t.Fatalf("fractional index ok=%v err=%v, want false/nil", ok, err)
	}
	_, ok, err = DenseArrayIndexFromValue(FloatValue(4), 3)
	if err == nil || !ok {
		t.Fatalf("out-of-range index ok=%v err=%v, want true/error", ok, err)
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
