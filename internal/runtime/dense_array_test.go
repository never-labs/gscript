package runtime

import (
	"errors"
	"math"
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
	strings := []string{"AAPL", "MSFT"}
	ia := NewDenseArrayI64(ints)
	ba := NewDenseArrayBool(bools)
	sa := NewDenseArrayString(strings)
	ints[0] = 99
	bools[0] = false
	strings[0] = "NVDA"

	gotInts, ok := ia.I64()
	if !ok || !reflect.DeepEqual(gotInts, []int64{1, 2, 3}) {
		t.Fatalf("I64() = %v/%v", gotInts, ok)
	}
	gotBools, ok := ba.Bool()
	if !ok || !reflect.DeepEqual(gotBools, []bool{true, false}) {
		t.Fatalf("Bool() = %v/%v", gotBools, ok)
	}
	gotStrings, ok := sa.StringValues()
	if !ok || !reflect.DeepEqual(gotStrings, []string{"AAPL", "MSFT"}) {
		t.Fatalf("StringValues() = %v/%v", gotStrings, ok)
	}
}

func TestDenseArrayStringBasicOperations(t *testing.T) {
	arr := NewDenseArrayString([]string{"AAPL", "MSFT", "NVDA"})
	if arr.DType() != DenseArrayString || arr.Len() != 3 {
		t.Fatalf("dtype/len = %s/%d, want string/3", arr.DType(), arr.Len())
	}
	if got := arr.String(); got != `array<string>["AAPL", "MSFT", "NVDA"]` {
		t.Fatalf("String() = %q", got)
	}
	if v, err := arr.At(2); err != nil || !v.IsString() || v.Str() != "NVDA" {
		t.Fatalf("At(2) = %#v, %v", v, err)
	}
	if err := arr.Set(1, StringValue("IBM")); err != nil {
		t.Fatalf("Set string: %v", err)
	}
	if err := arr.Append(StringValue("ORCL")); err != nil {
		t.Fatalf("Append string: %v", err)
	}
	gathered, err := arr.Gather(NewDenseArrayI64([]int64{4, 2}))
	if err != nil {
		t.Fatalf("Gather string: %v", err)
	}
	got, ok := gathered.StringValues()
	if !ok || !reflect.DeepEqual(got, []string{"ORCL", "IBM"}) {
		t.Fatalf("gathered strings = %#v/%v", got, ok)
	}
	filtered, err := arr.Filter(NewDenseArrayBool([]bool{true, false, true, false}))
	if err != nil {
		t.Fatalf("Filter string: %v", err)
	}
	got, ok = filtered.StringValues()
	if !ok || !reflect.DeepEqual(got, []string{"AAPL", "NVDA"}) {
		t.Fatalf("filtered strings = %#v/%v", got, ok)
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

func TestDenseArrayMaskCombineArrays(t *testing.T) {
	left := NewDenseArrayBool([]bool{true, true, false, false})
	right := NewDenseArrayBool([]bool{true, false, true, false})

	got, err := DenseArrayMaskCombineArrays(DenseArrayMaskAnd, left, right)
	if err != nil {
		t.Fatalf("DenseArrayMaskCombineArrays and error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{true, false, false, false})

	got, err = DenseArrayMaskCombineArrays(DenseArrayMaskOr, left, right)
	if err != nil {
		t.Fatalf("DenseArrayMaskCombineArrays or error: %v", err)
	}
	assertDenseBool(t, DenseArrayValue(got), []bool{true, true, true, false})

	if _, err := DenseArrayMaskCombineArrays(DenseArrayMaskAnd, left, NewDenseArrayBool([]bool{true})); err == nil {
		t.Fatalf("DenseArrayMaskCombineArrays accepted length mismatch")
	}
	if _, err := DenseArrayMaskCombineArrays(DenseArrayMaskAnd, left, NewDenseArrayI64([]int64{1, 2, 3, 4})); err == nil {
		t.Fatalf("DenseArrayMaskCombineArrays accepted non-bool rhs")
	}
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

func TestDenseArrayWhereReduceMatchesWhereThenReduce(t *testing.T) {
	cases := []struct {
		name       string
		op         DenseArrayReduceOp
		mask       *DenseArray
		trueValue  Value
		falseValue Value
	}{
		{
			name:       "i64 sum",
			op:         DenseArrayReduceSum,
			mask:       NewDenseArrayBool([]bool{true, false, true}),
			trueValue:  DenseArrayValue(NewDenseArrayI64([]int64{10, 20, 30})),
			falseValue: IntValue(7),
		},
		{
			name:       "i64 min keeps false branch rows",
			op:         DenseArrayReduceMin,
			mask:       NewDenseArrayBool([]bool{true, false, true}),
			trueValue:  DenseArrayValue(NewDenseArrayI64([]int64{10, 20, 30})),
			falseValue: IntValue(-5),
		},
		{
			name:       "i64 max",
			op:         DenseArrayReduceMax,
			mask:       NewDenseArrayBool([]bool{false, true, false}),
			trueValue:  IntValue(12),
			falseValue: DenseArrayValue(NewDenseArrayI64([]int64{4, 5, 6})),
		},
		{
			name:       "i64 mean",
			op:         DenseArrayReduceMean,
			mask:       NewDenseArrayBool([]bool{true, false, false, true}),
			trueValue:  DenseArrayValue(NewDenseArrayI64([]int64{4, 100, 100, 8})),
			falseValue: IntValue(2),
		},
		{
			name:       "f64 sum promotes i64",
			op:         DenseArrayReduceSum,
			mask:       NewDenseArrayBool([]bool{true, false, true}),
			trueValue:  DenseArrayValue(NewDenseArrayI64([]int64{1, 2, 3})),
			falseValue: FloatValue(0.5),
		},
		{
			name:       "f64 min",
			op:         DenseArrayReduceMin,
			mask:       NewDenseArrayBool([]bool{false, true, false}),
			trueValue:  FloatValue(1.25),
			falseValue: DenseArrayValue(NewDenseArrayF64([]float64{2.5, -10, 0.75})),
		},
		{
			name:       "f64 max",
			op:         DenseArrayReduceMax,
			mask:       NewDenseArrayBool([]bool{true, false, true}),
			trueValue:  DenseArrayValue(NewDenseArrayF64([]float64{-1, 4, 9.5})),
			falseValue: FloatValue(3),
		},
		{
			name:       "f64 mean",
			op:         DenseArrayReduceMean,
			mask:       NewDenseArrayBool([]bool{true, false, true, false}),
			trueValue:  DenseArrayValue(NewDenseArrayF64([]float64{1, 100, 5, 100})),
			falseValue: DenseArrayValue(NewDenseArrayI64([]int64{20, 2, 20, 6})),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := DenseArrayWhere(tc.mask, tc.trueValue, tc.falseValue)
			if err != nil {
				t.Fatalf("DenseArrayWhere error: %v", err)
			}
			want, err := DenseArrayReduce(tc.op, selected)
			if err != nil {
				t.Fatalf("DenseArrayReduce error: %v", err)
			}
			got, err := DenseArrayWhereReduce(tc.op, tc.mask, tc.trueValue, tc.falseValue)
			if err != nil {
				t.Fatalf("DenseArrayWhereReduce error: %v", err)
			}
			assertValueEqualOrBothNaN(t, got, want)
		})
	}
}

func TestDenseArrayWhereReduceMatchesWhereThenReduceEdges(t *testing.T) {
	cases := []struct {
		name       string
		op         DenseArrayReduceOp
		mask       *DenseArray
		trueValue  Value
		falseValue Value
	}{
		{
			name:       "f64 min propagates nan like reduce",
			op:         DenseArrayReduceMin,
			mask:       NewDenseArrayBool([]bool{true, false, true}),
			trueValue:  DenseArrayValue(NewDenseArrayF64([]float64{4, 5, math.NaN()})),
			falseValue: FloatValue(1),
		},
		{
			name:       "f64 max propagates nan like reduce",
			op:         DenseArrayReduceMax,
			mask:       NewDenseArrayBool([]bool{false, true, true}),
			trueValue:  DenseArrayValue(NewDenseArrayF64([]float64{4, math.NaN(), 8})),
			falseValue: FloatValue(1),
		},
		{
			name:       "i64 sum preserves overflow path",
			op:         DenseArrayReduceSum,
			mask:       NewDenseArrayBool([]bool{true, false, true}),
			trueValue:  DenseArrayValue(NewDenseArrayI64([]int64{math.MaxInt64, 0, 2})),
			falseValue: IntValue(1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := DenseArrayWhere(tc.mask, tc.trueValue, tc.falseValue)
			if err != nil {
				t.Fatalf("DenseArrayWhere error: %v", err)
			}
			want, err := DenseArrayReduce(tc.op, selected)
			if err != nil {
				t.Fatalf("DenseArrayReduce error: %v", err)
			}
			got, err := DenseArrayWhereReduce(tc.op, tc.mask, tc.trueValue, tc.falseValue)
			if err != nil {
				t.Fatalf("DenseArrayWhereReduce error: %v", err)
			}
			assertValueEqualOrBothNaN(t, got, want)
		})
	}
}

func assertValueEqualOrBothNaN(t *testing.T, got, want Value) {
	t.Helper()
	if got.IsFloat() && want.IsFloat() && math.IsNaN(got.Float()) && math.IsNaN(want.Float()) {
		return
	}
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDenseArrayWhereReduceErrorsMatchWhereThenReduce(t *testing.T) {
	if _, err := DenseArrayWhereReduce(DenseArrayReduceSum, NewDenseArrayBool([]bool{true}), DenseArrayValue(NewDenseArrayI64([]int64{10, 20})), IntValue(7)); !errors.Is(err, ErrDenseArrayLength) {
		t.Fatalf("length error = %v, want ErrDenseArrayLength", err)
	}
	if _, err := DenseArrayWhereReduce(DenseArrayReduceSum, NewDenseArrayBool([]bool{true}), BoolValue(true), BoolValue(false)); !errors.Is(err, ErrDenseArrayDType) {
		t.Fatalf("bool dtype error = %v, want ErrDenseArrayDType", err)
	}
	if _, err := DenseArrayWhereReduce(DenseArrayReduceSum, NewDenseArrayBool(nil), IntValue(1), IntValue(2)); !errors.Is(err, ErrDenseArrayEmpty) {
		t.Fatalf("empty error = %v, want ErrDenseArrayEmpty", err)
	}
	if _, err := DenseArrayWhereReduce(DenseArrayReduceOp(99), NewDenseArrayBool([]bool{true}), IntValue(1), IntValue(2)); !errors.Is(err, ErrDenseArrayReduceOp) {
		t.Fatalf("reduce op error = %v, want ErrDenseArrayReduceOp", err)
	}
	if _, err := DenseArrayWhereReduce(DenseArrayReduceSum, NewDenseArrayI64([]int64{1}), IntValue(1), IntValue(2)); err == nil {
		t.Fatalf("non-bool mask error = nil, want error")
	}
}

func TestDenseArrayGatherReduceMatchesGatherThenReduce(t *testing.T) {
	cases := []struct {
		name    string
		op      DenseArrayReduceOp
		array   *DenseArray
		indexes *DenseArray
	}{
		{"i64 sum", DenseArrayReduceSum, NewDenseArrayI64([]int64{10, -5, 30, 7}), NewDenseArrayI64([]int64{3, 1, 4})},
		{"i64 min", DenseArrayReduceMin, NewDenseArrayI64([]int64{10, -5, 30, 7}), NewDenseArrayI64([]int64{4, 2, 1})},
		{"i64 max", DenseArrayReduceMax, NewDenseArrayI64([]int64{10, -5, 30, 7}), NewDenseArrayI64([]int64{2, 4, 1})},
		{"i64 mean", DenseArrayReduceMean, NewDenseArrayI64([]int64{10, -5, 30, 7}), NewDenseArrayI64([]int64{1, 2, 4})},
		{"f64 sum", DenseArrayReduceSum, NewDenseArrayF64([]float64{1.5, -2.5, 6, 3}), NewDenseArrayI64([]int64{3, 1})},
		{"f64 min", DenseArrayReduceMin, NewDenseArrayF64([]float64{1.5, -2.5, 6, 3}), NewDenseArrayI64([]int64{1, 4, 2})},
		{"f64 max", DenseArrayReduceMax, NewDenseArrayF64([]float64{1.5, -2.5, 6, 3}), NewDenseArrayI64([]int64{2, 3, 1})},
		{"f64 mean", DenseArrayReduceMean, NewDenseArrayF64([]float64{1.5, -2.5, 6, 3}), NewDenseArrayI64([]int64{1, 3, 4})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gathered, err := tc.array.Gather(tc.indexes)
			if err != nil {
				t.Fatalf("DenseArray.Gather error: %v", err)
			}
			want, err := DenseArrayReduce(tc.op, gathered)
			if err != nil {
				t.Fatalf("DenseArrayReduce error: %v", err)
			}
			got, err := DenseArrayGatherReduce(tc.op, tc.array, tc.indexes)
			if err != nil {
				t.Fatalf("DenseArrayGatherReduce error: %v", err)
			}
			assertValueEqualOrBothNaN(t, got, want)
		})
	}
}

func TestDenseArrayGatherReduceErrorsMatchGatherThenReduce(t *testing.T) {
	if _, err := DenseArrayGatherReduce(DenseArrayReduceSum, NewDenseArrayI64([]int64{1, 2}), NewDenseArrayI64(nil)); !errors.Is(err, ErrDenseArrayEmpty) {
		t.Fatalf("empty index error = %v, want ErrDenseArrayEmpty", err)
	}
	if _, err := DenseArrayGatherReduce(DenseArrayReduceSum, NewDenseArrayBool([]bool{true}), NewDenseArrayI64([]int64{1})); !errors.Is(err, ErrDenseArrayDType) {
		t.Fatalf("bool dtype error = %v, want ErrDenseArrayDType", err)
	}
	if _, err := DenseArrayGatherReduce(DenseArrayReduceSum, NewDenseArrayI64([]int64{1}), NewDenseArrayF64([]float64{1})); err == nil {
		t.Fatalf("non-i64 index error = nil, want error")
	}
	if _, err := DenseArrayGatherReduce(DenseArrayReduceOp(99), NewDenseArrayI64([]int64{1}), NewDenseArrayI64([]int64{1})); !errors.Is(err, ErrDenseArrayReduceOp) {
		t.Fatalf("reduce op error = %v, want ErrDenseArrayReduceOp", err)
	}
	if _, err := DenseArrayGatherReduce(DenseArrayReduceSum, NewDenseArrayI64([]int64{1}), NewDenseArrayI64([]int64{2})); err == nil {
		t.Fatalf("out-of-range index error = nil, want error")
	}
}

func BenchmarkDenseArrayGatherReduce(b *testing.B) {
	const rows = 8192
	values := make([]float64, rows)
	indexes := make([]int64, rows/2)
	for i := range values {
		values[i] = float64(i) * 1.25
	}
	for i := range indexes {
		indexes[i] = int64(i*2 + 1)
	}
	array := NewDenseArrayF64(values)
	indexArray := NewDenseArrayI64(indexes)
	b.Run("GatherThenReduce", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gathered, err := array.Gather(indexArray)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := DenseArrayReduce(DenseArrayReduceSum, gathered); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Fused", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := DenseArrayGatherReduce(DenseArrayReduceSum, array, indexArray); err != nil {
				b.Fatal(err)
			}
		}
	})
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
