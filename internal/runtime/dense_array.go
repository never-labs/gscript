package runtime

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"
)

// DenseArrayDType is the physical element type for a DenseArray.
type DenseArrayDType uint8

const (
	DenseArrayF64 DenseArrayDType = iota
	DenseArrayI64
	DenseArrayBool
)

func (dt DenseArrayDType) String() string {
	switch dt {
	case DenseArrayF64:
		return "f64"
	case DenseArrayI64:
		return "i64"
	case DenseArrayBool:
		return "bool"
	default:
		return "unknown"
	}
}

// DenseArray is a compact, homogeneous array value for data-oriented helpers.
// Exactly one backing slice is populated according to dtype.
type DenseArray struct {
	dtype   DenseArrayDType
	version uint64
	f64     []float64
	i64     []int64
	bools   []bool
}

func NewDenseArrayF64(values []float64) *DenseArray {
	a := &DenseArray{dtype: DenseArrayF64}
	if len(values) > 0 {
		a.f64 = append([]float64(nil), values...)
	}
	return a
}

func NewDenseArrayI64(values []int64) *DenseArray {
	a := &DenseArray{dtype: DenseArrayI64}
	if len(values) > 0 {
		a.i64 = append([]int64(nil), values...)
	}
	return a
}

func NewDenseArrayBool(values []bool) *DenseArray {
	a := &DenseArray{dtype: DenseArrayBool}
	if len(values) > 0 {
		a.bools = append([]bool(nil), values...)
	}
	return a
}

func NewDenseArrayOfLen(dtype DenseArrayDType, n int) (*DenseArray, error) {
	if n < 0 {
		return nil, fmt.Errorf("dense array length must be non-negative")
	}
	switch dtype {
	case DenseArrayF64:
		return &DenseArray{dtype: dtype, f64: make([]float64, n)}, nil
	case DenseArrayI64:
		return &DenseArray{dtype: dtype, i64: make([]int64, n)}, nil
	case DenseArrayBool:
		return &DenseArray{dtype: dtype, bools: make([]bool, n)}, nil
	default:
		return nil, ErrDenseArrayDType
	}
}

func DenseArrayValue(a *DenseArray) Value {
	if a == nil {
		return NilValue()
	}
	p := unsafe.Pointer(a)
	keepAlive(p, a)
	return Value(tagPtr | ptrSubDenseArray | (uint64(uintptr(p)) & ptrAddrMask))
}

func (v Value) IsDenseArray() bool {
	return uint64(v)&tagMask == tagPtr && v.ptrSubType() == ptrSubDenseArray
}

func (v Value) DenseArray() *DenseArray {
	if !v.IsDenseArray() {
		return nil
	}
	p := v.ptrPayload()
	if p == nil {
		return nil
	}
	return (*DenseArray)(p)
}

func (a *DenseArray) DType() DenseArrayDType {
	if a == nil {
		return DenseArrayF64
	}
	return a.dtype
}

func (a *DenseArray) Version() uint64 {
	if a == nil {
		return 0
	}
	return a.version
}

func (a *DenseArray) bumpVersion() {
	if a == nil {
		return
	}
	a.version++
	if a.version == 0 {
		a.version = 1
	}
}

func (a *DenseArray) Len() int {
	if a == nil {
		return 0
	}
	switch a.dtype {
	case DenseArrayF64:
		return len(a.f64)
	case DenseArrayI64:
		return len(a.i64)
	case DenseArrayBool:
		return len(a.bools)
	default:
		return 0
	}
}

func (a *DenseArray) F64() ([]float64, bool) {
	if a == nil || a.dtype != DenseArrayF64 {
		return nil, false
	}
	return a.f64, true
}

func (a *DenseArray) I64() ([]int64, bool) {
	if a == nil || a.dtype != DenseArrayI64 {
		return nil, false
	}
	return a.i64, true
}

func (a *DenseArray) Bool() ([]bool, bool) {
	if a == nil || a.dtype != DenseArrayBool {
		return nil, false
	}
	return a.bools, true
}

func (a *DenseArray) Clone() (*DenseArray, error) {
	if a == nil {
		return nil, ErrDenseArrayOperand
	}
	switch a.dtype {
	case DenseArrayF64:
		return NewDenseArrayF64(a.f64), nil
	case DenseArrayI64:
		return NewDenseArrayI64(a.i64), nil
	case DenseArrayBool:
		return NewDenseArrayBool(a.bools), nil
	default:
		return nil, ErrDenseArrayDType
	}
}

func (a *DenseArray) Slice(start, end int) (*DenseArray, error) {
	if a == nil {
		return nil, ErrDenseArrayOperand
	}
	if start < 0 || end < start || end > a.Len() {
		return nil, fmt.Errorf("dense array slice out of range")
	}
	switch a.dtype {
	case DenseArrayF64:
		return NewDenseArrayF64(a.f64[start:end]), nil
	case DenseArrayI64:
		return NewDenseArrayI64(a.i64[start:end]), nil
	case DenseArrayBool:
		return NewDenseArrayBool(a.bools[start:end]), nil
	default:
		return nil, ErrDenseArrayDType
	}
}

func (a *DenseArray) Filter(mask *DenseArray) (*DenseArray, error) {
	if mask == nil {
		return nil, ErrDenseArrayOperand
	}
	return a.filterKnownCount(mask, denseArrayBoolCount(mask.bools))
}

func (a *DenseArray) filterKnownCount(mask *DenseArray, count int) (*DenseArray, error) {
	if a == nil || mask == nil {
		return nil, ErrDenseArrayOperand
	}
	if mask.dtype != DenseArrayBool {
		return nil, fmt.Errorf("dense array filter mask must be bool")
	}
	if a.Len() != mask.Len() {
		return nil, ErrDenseArrayLength
	}
	switch a.dtype {
	case DenseArrayF64:
		out := make([]float64, count)
		j := 0
		for i, keep := range mask.bools {
			if keep {
				out[j] = a.f64[i]
				j++
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, nil
	case DenseArrayI64:
		out := make([]int64, count)
		j := 0
		for i, keep := range mask.bools {
			if keep {
				out[j] = a.i64[i]
				j++
			}
		}
		return &DenseArray{dtype: DenseArrayI64, i64: out}, nil
	case DenseArrayBool:
		out := make([]bool, count)
		j := 0
		for i, keep := range mask.bools {
			if keep {
				out[j] = a.bools[i]
				j++
			}
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, nil
	default:
		return nil, ErrDenseArrayDType
	}
}

func (a *DenseArray) Gather(indices *DenseArray) (*DenseArray, error) {
	if a == nil || indices == nil {
		return nil, ErrDenseArrayOperand
	}
	if indices.dtype != DenseArrayI64 {
		return nil, fmt.Errorf("dense array gather indices must be i64")
	}
	if err := denseArrayValidateGatherIndices(indices, a.Len()); err != nil {
		return nil, err
	}
	return a.gatherValidatedI64(indices.i64)
}

func (a *DenseArray) gatherValidatedI64(indices []int64) (*DenseArray, error) {
	if a == nil {
		return nil, ErrDenseArrayOperand
	}
	switch a.dtype {
	case DenseArrayF64:
		out := make([]float64, len(indices))
		for i, index := range indices {
			out[i] = a.f64[index-1]
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, nil
	case DenseArrayI64:
		out := make([]int64, len(indices))
		for i, index := range indices {
			out[i] = a.i64[index-1]
		}
		return &DenseArray{dtype: DenseArrayI64, i64: out}, nil
	case DenseArrayBool:
		out := make([]bool, len(indices))
		for i, index := range indices {
			out[i] = a.bools[index-1]
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, nil
	default:
		return nil, ErrDenseArrayDType
	}
}

func denseArrayValidateGatherIndices(indices *DenseArray, length int) error {
	if indices == nil || indices.dtype != DenseArrayI64 {
		return fmt.Errorf("dense array gather indices must be i64")
	}
	for _, index := range indices.i64 {
		if index < 1 || index > int64(length) {
			return fmt.Errorf("dense array index out of range")
		}
	}
	return nil
}

func (a *DenseArray) SumWhere(mask *DenseArray) (Value, error) {
	if a == nil || mask == nil {
		return NilValue(), ErrDenseArrayOperand
	}
	if mask.dtype != DenseArrayBool {
		return NilValue(), fmt.Errorf("dense array sumWhere mask must be bool")
	}
	if a.Len() != mask.Len() {
		return NilValue(), ErrDenseArrayLength
	}
	switch a.dtype {
	case DenseArrayF64:
		sum := 0.0
		for i, keep := range mask.bools {
			if keep {
				sum += a.f64[i]
			}
		}
		return FloatValue(sum), nil
	case DenseArrayI64:
		var sum int64
		for i, keep := range mask.bools {
			if keep {
				sum += a.i64[i]
			}
		}
		return IntValue(sum), nil
	default:
		return NilValue(), ErrDenseArrayDType
	}
}

func (a *DenseArray) MinWhere(mask *DenseArray) (Value, error) {
	return a.extremeWhere(mask, false)
}

func (a *DenseArray) MaxWhere(mask *DenseArray) (Value, error) {
	return a.extremeWhere(mask, true)
}

func (a *DenseArray) extremeWhere(mask *DenseArray, max bool) (Value, error) {
	if a == nil || mask == nil {
		return NilValue(), ErrDenseArrayOperand
	}
	if mask.dtype != DenseArrayBool {
		return NilValue(), fmt.Errorf("dense array min/max mask must be bool")
	}
	if a.Len() != mask.Len() {
		return NilValue(), ErrDenseArrayLength
	}
	switch a.dtype {
	case DenseArrayF64:
		var out float64
		seen := false
		for i, keep := range mask.bools {
			if !keep {
				continue
			}
			v := a.f64[i]
			if !seen || (max && v > out) || (!max && v < out) {
				out = v
				seen = true
			}
		}
		if !seen {
			return NilValue(), ErrDenseArrayEmpty
		}
		return FloatValue(out), nil
	case DenseArrayI64:
		var out int64
		seen := false
		for i, keep := range mask.bools {
			if !keep {
				continue
			}
			v := a.i64[i]
			if !seen || (max && v > out) || (!max && v < out) {
				out = v
				seen = true
			}
		}
		if !seen {
			return NilValue(), ErrDenseArrayEmpty
		}
		return IntValue(out), nil
	default:
		return NilValue(), ErrDenseArrayDType
	}
}

func denseArrayOneBasedIndex(index int64, length int) (int, error) {
	if index < 1 || index > int64(length) {
		return 0, fmt.Errorf("dense array index out of range")
	}
	return int(index - 1), nil
}

func denseArrayBoolCount(xs []bool) int {
	n := 0
	for _, v := range xs {
		if v {
			n++
		}
	}
	return n
}

func (a *DenseArray) At(i int) (Value, error) {
	if a == nil {
		return NilValue(), ErrDenseArrayOperand
	}
	if i < 0 || i >= a.Len() {
		return NilValue(), fmt.Errorf("dense array index out of range")
	}
	switch a.dtype {
	case DenseArrayF64:
		return FloatValue(a.f64[i]), nil
	case DenseArrayI64:
		return IntValue(a.i64[i]), nil
	case DenseArrayBool:
		return BoolValue(a.bools[i]), nil
	default:
		return NilValue(), ErrDenseArrayDType
	}
}

func (a *DenseArray) Set(i int, v Value) error {
	if a == nil {
		return ErrDenseArrayOperand
	}
	if i < 0 || i >= a.Len() {
		return fmt.Errorf("dense array index out of range")
	}
	switch a.dtype {
	case DenseArrayF64:
		if !v.IsNumber() {
			return fmt.Errorf("dense array f64 value must be numeric")
		}
		a.f64[i] = v.Number()
	case DenseArrayI64:
		if !v.IsInt() {
			return fmt.Errorf("dense array i64 value must be integer")
		}
		a.i64[i] = v.Int()
	case DenseArrayBool:
		if !v.IsBool() {
			return fmt.Errorf("dense array bool value must be boolean")
		}
		a.bools[i] = v.Bool()
	default:
		return ErrDenseArrayDType
	}
	a.bumpVersion()
	return nil
}

func (a *DenseArray) String() string {
	if a == nil {
		return "array<nil>[]"
	}
	var b strings.Builder
	b.WriteString("array<")
	b.WriteString(a.dtype.String())
	b.WriteString(">[")
	for i := 0; i < a.Len(); i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		switch a.dtype {
		case DenseArrayF64:
			b.WriteString(strconv.FormatFloat(a.f64[i], 'g', -1, 64))
		case DenseArrayI64:
			b.WriteString(strconv.FormatInt(a.i64[i], 10))
		case DenseArrayBool:
			b.WriteString(strconv.FormatBool(a.bools[i]))
		default:
			b.WriteString("?")
		}
	}
	b.WriteByte(']')
	return b.String()
}

type DenseArrayBinaryOp uint8

const (
	DenseArrayAdd DenseArrayBinaryOp = iota
	DenseArraySub
	DenseArrayMul
	DenseArrayDiv
	DenseArrayEQ
	DenseArrayNE
	DenseArrayLT
	DenseArrayLE
	DenseArrayGT
	DenseArrayGE
)

var (
	ErrDenseArrayOperand  = errors.New("dense array operation requires at least one dense array operand")
	ErrDenseArrayLength   = errors.New("dense array length mismatch")
	ErrDenseArrayDType    = errors.New("dense array dtype is not supported for this operation")
	ErrDenseArrayScalar   = errors.New("scalar is not supported for dense array operation")
	ErrDenseArrayEmpty    = errors.New("dense array reduce on empty array")
	ErrDenseArrayReduceOp = errors.New("dense array reduce operation is not supported")
)

// DenseArrayElementwise applies op to array-array, array-scalar, or scalar-array
// operands. Arithmetic returns an i64 array only for i64+i64 +, -, and *;
// division and mixed numeric inputs return f64. Comparisons return bool arrays.
func DenseArrayElementwise(op DenseArrayBinaryOp, left, right Value) (Value, error) {
	la, ra := left.DenseArray(), right.DenseArray()
	switch {
	case la != nil && ra != nil:
		if la.Len() != ra.Len() {
			return NilValue(), ErrDenseArrayLength
		}
		return denseArrayArrayOp(op, la, ra)
	case la != nil:
		return denseArrayScalarOp(op, la, right, false)
	case ra != nil:
		return denseArrayScalarOp(op, ra, left, true)
	default:
		return NilValue(), ErrDenseArrayOperand
	}
}

func denseArrayArrayOp(op DenseArrayBinaryOp, left, right *DenseArray) (Value, error) {
	if op == DenseArrayEQ || op == DenseArrayNE {
		if left.dtype == DenseArrayBool && right.dtype == DenseArrayBool {
			out := make([]bool, left.Len())
			for i := range out {
				out[i] = compareBools(op, left.bools[i], right.bools[i])
			}
			return DenseArrayValue(&DenseArray{dtype: DenseArrayBool, bools: out}), nil
		}
	}
	if left.dtype == DenseArrayBool || right.dtype == DenseArrayBool {
		return NilValue(), ErrDenseArrayDType
	}
	if isComparisonOp(op) {
		out := make([]bool, left.Len())
		for i := range out {
			out[i] = compareFloat64(op, denseArrayFloatAt(left, i), denseArrayFloatAt(right, i))
		}
		return DenseArrayValue(&DenseArray{dtype: DenseArrayBool, bools: out}), nil
	}
	if left.dtype == DenseArrayI64 && right.dtype == DenseArrayI64 && op != DenseArrayDiv {
		out := make([]int64, left.Len())
		for i := range out {
			out[i] = arithmeticInt64(op, left.i64[i], right.i64[i])
		}
		return DenseArrayValue(&DenseArray{dtype: DenseArrayI64, i64: out}), nil
	}
	out := make([]float64, left.Len())
	for i := range out {
		out[i] = arithmeticFloat64(op, denseArrayFloatAt(left, i), denseArrayFloatAt(right, i))
	}
	return DenseArrayValue(&DenseArray{dtype: DenseArrayF64, f64: out}), nil
}

func denseArrayScalarOp(op DenseArrayBinaryOp, arr *DenseArray, scalar Value, scalarLeft bool) (Value, error) {
	if scalar.IsDenseArray() {
		panic("denseArrayScalarOp called with array scalar")
	}
	if op == DenseArrayEQ || op == DenseArrayNE {
		if arr.dtype == DenseArrayBool && scalar.IsBool() {
			out := make([]bool, arr.Len())
			for i := range out {
				out[i] = compareBools(op, scalar.Bool(), arr.bools[i])
				if !scalarLeft {
					out[i] = compareBools(op, arr.bools[i], scalar.Bool())
				}
			}
			return DenseArrayValue(&DenseArray{dtype: DenseArrayBool, bools: out}), nil
		}
	}
	if arr.dtype == DenseArrayBool {
		return NilValue(), ErrDenseArrayDType
	}
	if !scalar.IsNumber() {
		return NilValue(), ErrDenseArrayScalar
	}
	if isComparisonOp(op) {
		out := make([]bool, arr.Len())
		s := scalar.Number()
		for i := range out {
			l, r := denseArrayFloatAt(arr, i), s
			if scalarLeft {
				l, r = s, l
			}
			out[i] = compareFloat64(op, l, r)
		}
		return DenseArrayValue(&DenseArray{dtype: DenseArrayBool, bools: out}), nil
	}
	if arr.dtype == DenseArrayI64 && scalar.IsInt() && op != DenseArrayDiv {
		out := make([]int64, arr.Len())
		s := scalar.Int()
		for i := range out {
			l, r := arr.i64[i], s
			if scalarLeft {
				l, r = s, l
			}
			out[i] = arithmeticInt64(op, l, r)
		}
		return DenseArrayValue(&DenseArray{dtype: DenseArrayI64, i64: out}), nil
	}
	out := make([]float64, arr.Len())
	s := scalar.Number()
	for i := range out {
		l, r := denseArrayFloatAt(arr, i), s
		if scalarLeft {
			l, r = s, l
		}
		out[i] = arithmeticFloat64(op, l, r)
	}
	return DenseArrayValue(&DenseArray{dtype: DenseArrayF64, f64: out}), nil
}

type DenseArrayReduceOp uint8

const (
	DenseArrayReduceSum DenseArrayReduceOp = iota
	DenseArrayReduceMin
	DenseArrayReduceMax
	DenseArrayReduceMean
)

func DenseArrayReduce(op DenseArrayReduceOp, arr *DenseArray) (Value, error) {
	if arr == nil {
		return NilValue(), ErrDenseArrayOperand
	}
	if arr.dtype == DenseArrayBool {
		return NilValue(), ErrDenseArrayDType
	}
	if arr.Len() == 0 {
		return NilValue(), ErrDenseArrayEmpty
	}
	switch arr.dtype {
	case DenseArrayI64:
		return denseArrayReduceI64(op, arr.i64)
	case DenseArrayF64:
		return denseArrayReduceF64(op, arr.f64)
	default:
		return NilValue(), ErrDenseArrayDType
	}
}

func denseArrayReduceI64(op DenseArrayReduceOp, xs []int64) (Value, error) {
	switch op {
	case DenseArrayReduceSum:
		var sum int64
		for _, v := range xs {
			sum += v
		}
		return IntValue(sum), nil
	case DenseArrayReduceMin:
		min := xs[0]
		for _, v := range xs[1:] {
			if v < min {
				min = v
			}
		}
		return IntValue(min), nil
	case DenseArrayReduceMax:
		max := xs[0]
		for _, v := range xs[1:] {
			if v > max {
				max = v
			}
		}
		return IntValue(max), nil
	case DenseArrayReduceMean:
		var sum float64
		for _, v := range xs {
			sum += float64(v)
		}
		return FloatValue(sum / float64(len(xs))), nil
	default:
		return NilValue(), ErrDenseArrayReduceOp
	}
}

func denseArrayReduceF64(op DenseArrayReduceOp, xs []float64) (Value, error) {
	switch op {
	case DenseArrayReduceSum:
		var sum float64
		for _, v := range xs {
			sum += v
		}
		return FloatValue(sum), nil
	case DenseArrayReduceMin:
		min := xs[0]
		for _, v := range xs[1:] {
			min = math.Min(min, v)
		}
		return FloatValue(min), nil
	case DenseArrayReduceMax:
		max := xs[0]
		for _, v := range xs[1:] {
			max = math.Max(max, v)
		}
		return FloatValue(max), nil
	case DenseArrayReduceMean:
		var sum float64
		for _, v := range xs {
			sum += v
		}
		return FloatValue(sum / float64(len(xs))), nil
	default:
		return NilValue(), ErrDenseArrayReduceOp
	}
}

func denseArrayAddScaled(dst, src *DenseArray, scale float64) error {
	if dst == nil || src == nil {
		return ErrDenseArrayOperand
	}
	if dst.Len() != src.Len() {
		return ErrDenseArrayLength
	}
	if dst.dtype == DenseArrayBool || src.dtype == DenseArrayBool {
		return ErrDenseArrayDType
	}
	if dst.dtype == DenseArrayF64 {
		switch src.dtype {
		case DenseArrayF64:
			for i := range dst.f64 {
				dst.f64[i] += src.f64[i] * scale
			}
		case DenseArrayI64:
			for i := range dst.f64 {
				dst.f64[i] += float64(src.i64[i]) * scale
			}
		default:
			return ErrDenseArrayDType
		}
		dst.bumpVersion()
		return nil
	}
	if dst.dtype == DenseArrayI64 && src.dtype == DenseArrayI64 && scale == float64(int64(scale)) {
		s := int64(scale)
		for i := range dst.i64 {
			dst.i64[i] += src.i64[i] * s
		}
		dst.bumpVersion()
		return nil
	}
	return fmt.Errorf("dense array addScaled requires f64 destination or integral i64 scale")
}

func denseArrayAffine(dst, src *DenseArray, scale, bias float64) error {
	if dst == nil || src == nil {
		return ErrDenseArrayOperand
	}
	if dst.Len() != src.Len() {
		return ErrDenseArrayLength
	}
	if dst.dtype != DenseArrayF64 || src.dtype == DenseArrayBool {
		return fmt.Errorf("dense array affine requires f64 destination and numeric source")
	}
	switch src.dtype {
	case DenseArrayF64:
		for i := range dst.f64 {
			dst.f64[i] = src.f64[i]*scale + bias
		}
	case DenseArrayI64:
		for i := range dst.f64 {
			dst.f64[i] = float64(src.i64[i])*scale + bias
		}
	default:
		return ErrDenseArrayDType
	}
	dst.bumpVersion()
	return nil
}

func denseArrayFloatAt(a *DenseArray, i int) float64 {
	switch a.dtype {
	case DenseArrayF64:
		return a.f64[i]
	case DenseArrayI64:
		return float64(a.i64[i])
	default:
		panic(fmt.Sprintf("denseArrayFloatAt unsupported dtype %s", a.dtype))
	}
}

func isComparisonOp(op DenseArrayBinaryOp) bool {
	return op >= DenseArrayEQ && op <= DenseArrayGE
}

func arithmeticInt64(op DenseArrayBinaryOp, left, right int64) int64 {
	switch op {
	case DenseArrayAdd:
		return left + right
	case DenseArraySub:
		return left - right
	case DenseArrayMul:
		return left * right
	default:
		panic(fmt.Sprintf("unsupported i64 arithmetic op %d", op))
	}
}

func arithmeticFloat64(op DenseArrayBinaryOp, left, right float64) float64 {
	switch op {
	case DenseArrayAdd:
		return left + right
	case DenseArraySub:
		return left - right
	case DenseArrayMul:
		return left * right
	case DenseArrayDiv:
		return left / right
	default:
		panic(fmt.Sprintf("unsupported f64 arithmetic op %d", op))
	}
}

func compareFloat64(op DenseArrayBinaryOp, left, right float64) bool {
	switch op {
	case DenseArrayEQ:
		return left == right
	case DenseArrayNE:
		return left != right
	case DenseArrayLT:
		return left < right
	case DenseArrayLE:
		return left <= right
	case DenseArrayGT:
		return left > right
	case DenseArrayGE:
		return left >= right
	default:
		panic(fmt.Sprintf("unsupported comparison op %d", op))
	}
}

func compareBools(op DenseArrayBinaryOp, left, right bool) bool {
	switch op {
	case DenseArrayEQ:
		return left == right
	case DenseArrayNE:
		return left != right
	default:
		panic(fmt.Sprintf("unsupported bool comparison op %d", op))
	}
}
