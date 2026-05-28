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

func (a *DenseArray) Resize(n int) error {
	if a == nil {
		return ErrDenseArrayOperand
	}
	if n < 0 {
		return fmt.Errorf("dense array length must be non-negative")
	}
	if n == a.Len() {
		return nil
	}
	switch a.dtype {
	case DenseArrayF64:
		a.f64 = denseArrayResize(a.f64, n)
	case DenseArrayI64:
		a.i64 = denseArrayResize(a.i64, n)
	case DenseArrayBool:
		a.bools = denseArrayResize(a.bools, n)
	default:
		return ErrDenseArrayDType
	}
	a.bumpVersion()
	return nil
}

func denseArrayResize[T any](xs []T, n int) []T {
	if n <= len(xs) {
		return xs[:n]
	}
	out := make([]T, n)
	copy(out, xs)
	return out
}

func (a *DenseArray) Append(v Value) error {
	if a == nil {
		return ErrDenseArrayOperand
	}
	switch a.dtype {
	case DenseArrayF64:
		if !v.IsNumber() {
			return fmt.Errorf("dense array f64 value must be numeric")
		}
		a.f64 = append(a.f64, v.Number())
	case DenseArrayI64:
		if !v.IsInt() {
			return fmt.Errorf("dense array i64 value must be integer")
		}
		a.i64 = append(a.i64, v.Int())
	case DenseArrayBool:
		if !v.IsBool() {
			return fmt.Errorf("dense array bool value must be boolean")
		}
		a.bools = append(a.bools, v.Bool())
	default:
		return ErrDenseArrayDType
	}
	a.bumpVersion()
	return nil
}

func (a *DenseArray) CanAppend(v Value) error {
	if a == nil {
		return ErrDenseArrayOperand
	}
	switch a.dtype {
	case DenseArrayF64:
		if !v.IsNumber() {
			return fmt.Errorf("dense array f64 value must be numeric")
		}
	case DenseArrayI64:
		if !v.IsInt() {
			return fmt.Errorf("dense array i64 value must be integer")
		}
	case DenseArrayBool:
		if !v.IsBool() {
			return fmt.Errorf("dense array bool value must be boolean")
		}
	default:
		return ErrDenseArrayDType
	}
	return nil
}

func (a *DenseArray) Fill(v Value) error {
	if err := a.CanAppend(v); err != nil {
		return err
	}
	switch a.dtype {
	case DenseArrayF64:
		x := v.Number()
		for i := range a.f64 {
			a.f64[i] = x
		}
	case DenseArrayI64:
		x := v.Int()
		for i := range a.i64 {
			a.i64[i] = x
		}
	case DenseArrayBool:
		x := v.Bool()
		for i := range a.bools {
			a.bools[i] = x
		}
	default:
		return ErrDenseArrayDType
	}
	a.bumpVersion()
	return nil
}

func (a *DenseArray) FillWhere(mask *DenseArray, v Value) error {
	if a == nil || mask == nil {
		return ErrDenseArrayOperand
	}
	if mask.dtype != DenseArrayBool {
		return fmt.Errorf("dense array fillWhere mask must be bool")
	}
	if a.Len() != mask.Len() {
		return ErrDenseArrayLength
	}
	if err := a.CanAppend(v); err != nil {
		return err
	}
	switch a.dtype {
	case DenseArrayF64:
		x := v.Number()
		for i, keep := range mask.bools {
			if keep {
				a.f64[i] = x
			}
		}
	case DenseArrayI64:
		x := v.Int()
		for i, keep := range mask.bools {
			if keep {
				a.i64[i] = x
			}
		}
	case DenseArrayBool:
		x := v.Bool()
		for i, keep := range mask.bools {
			if keep {
				a.bools[i] = x
			}
		}
	default:
		return ErrDenseArrayDType
	}
	a.bumpVersion()
	return nil
}

func denseArraySelect(mask *DenseArray, trueValue, falseValue Value, length int) (*DenseArray, error) {
	if mask == nil || mask.dtype != DenseArrayBool {
		return nil, fmt.Errorf("dense array select mask must be bool")
	}
	if mask.Len() != length {
		return nil, ErrDenseArrayLength
	}
	if out, ok, err := denseArraySelectFast(mask.bools, trueValue, falseValue, length); ok || err != nil {
		return out, err
	}
	dtype, err := denseArraySelectDType(trueValue, falseValue, length)
	if err != nil {
		return nil, err
	}
	switch dtype {
	case DenseArrayF64:
		out := make([]float64, length)
		tv := denseArrayF64Selector(trueValue)
		fv := denseArrayF64Selector(falseValue)
		for i, keep := range mask.bools {
			if keep {
				out[i] = tv(i)
			} else {
				out[i] = fv(i)
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, nil
	case DenseArrayI64:
		out := make([]int64, length)
		tv := denseArrayI64Selector(trueValue)
		fv := denseArrayI64Selector(falseValue)
		for i, keep := range mask.bools {
			if keep {
				out[i] = tv(i)
			} else {
				out[i] = fv(i)
			}
		}
		return &DenseArray{dtype: DenseArrayI64, i64: out}, nil
	case DenseArrayBool:
		out := make([]bool, length)
		tv := denseArrayBoolSelector(trueValue)
		fv := denseArrayBoolSelector(falseValue)
		for i, keep := range mask.bools {
			if keep {
				out[i] = tv(i)
			} else {
				out[i] = fv(i)
			}
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, nil
	default:
		return nil, ErrDenseArrayDType
	}
}

func denseArraySelectInto(dst *DenseArray, mask *DenseArray, trueValue, falseValue Value, length int) error {
	if dst == nil || mask == nil {
		return ErrDenseArrayOperand
	}
	if mask.dtype != DenseArrayBool {
		return fmt.Errorf("dense array selectInto mask must be bool")
	}
	if dst.Len() != length || mask.Len() != length {
		return ErrDenseArrayLength
	}
	if ok, err := denseArraySelectIntoFast(dst, mask.bools, trueValue, falseValue); ok || err != nil {
		return err
	}
	selected, err := denseArraySelect(mask, trueValue, falseValue, length)
	if err != nil {
		return err
	}
	if dst.dtype != selected.dtype {
		return ErrDenseArrayDType
	}
	switch dst.dtype {
	case DenseArrayF64:
		copy(dst.f64, selected.f64)
	case DenseArrayI64:
		copy(dst.i64, selected.i64)
	case DenseArrayBool:
		copy(dst.bools, selected.bools)
	default:
		return ErrDenseArrayDType
	}
	dst.bumpVersion()
	return nil
}

func denseArraySelectIntoFast(dst *DenseArray, mask []bool, trueValue, falseValue Value) (bool, error) {
	trueArray, falseArray := trueValue.DenseArray(), falseValue.DenseArray()
	if trueArray != nil && trueArray.Len() != len(mask) {
		return true, ErrDenseArrayLength
	}
	if falseArray != nil && falseArray.Len() != len(mask) {
		return true, ErrDenseArrayLength
	}
	if trueArray != nil && falseArray != nil {
		if ok, err := denseArraySelectIntoArrayArrayFast(dst, mask, trueArray, falseArray); ok || err != nil {
			return ok, err
		}
	}
	switch dst.dtype {
	case DenseArrayF64:
		tv, ok := denseArrayF64SelectorFast(trueValue)
		if !ok {
			return false, nil
		}
		fv, ok := denseArrayF64SelectorFast(falseValue)
		if !ok {
			return false, nil
		}
		for i, keep := range mask {
			if keep {
				dst.f64[i] = tv(i)
			} else {
				dst.f64[i] = fv(i)
			}
		}
	case DenseArrayI64:
		tv, ok := denseArrayI64SelectorFast(trueValue)
		if !ok {
			return false, nil
		}
		fv, ok := denseArrayI64SelectorFast(falseValue)
		if !ok {
			return false, nil
		}
		for i, keep := range mask {
			if keep {
				dst.i64[i] = tv(i)
			} else {
				dst.i64[i] = fv(i)
			}
		}
	case DenseArrayBool:
		tv, ok := denseArrayBoolSelectorFast(trueValue)
		if !ok {
			return false, nil
		}
		fv, ok := denseArrayBoolSelectorFast(falseValue)
		if !ok {
			return false, nil
		}
		for i, keep := range mask {
			if keep {
				dst.bools[i] = tv(i)
			} else {
				dst.bools[i] = fv(i)
			}
		}
	default:
		return true, ErrDenseArrayDType
	}
	dst.bumpVersion()
	return true, nil
}

func denseArraySelectIntoArrayArrayFast(dst *DenseArray, mask []bool, trueArray, falseArray *DenseArray) (bool, error) {
	switch {
	case dst.dtype == DenseArrayF64 && trueArray.dtype == DenseArrayF64 && falseArray.dtype == DenseArrayF64:
		for i, keep := range mask {
			if keep {
				dst.f64[i] = trueArray.f64[i]
			} else {
				dst.f64[i] = falseArray.f64[i]
			}
		}
	case dst.dtype == DenseArrayF64 && trueArray.dtype == DenseArrayF64 && falseArray.dtype == DenseArrayI64:
		for i, keep := range mask {
			if keep {
				dst.f64[i] = trueArray.f64[i]
			} else {
				dst.f64[i] = float64(falseArray.i64[i])
			}
		}
	case dst.dtype == DenseArrayF64 && trueArray.dtype == DenseArrayI64 && falseArray.dtype == DenseArrayF64:
		for i, keep := range mask {
			if keep {
				dst.f64[i] = float64(trueArray.i64[i])
			} else {
				dst.f64[i] = falseArray.f64[i]
			}
		}
	case dst.dtype == DenseArrayF64 && trueArray.dtype == DenseArrayI64 && falseArray.dtype == DenseArrayI64:
		for i, keep := range mask {
			if keep {
				dst.f64[i] = float64(trueArray.i64[i])
			} else {
				dst.f64[i] = float64(falseArray.i64[i])
			}
		}
	case dst.dtype == DenseArrayI64 && trueArray.dtype == DenseArrayI64 && falseArray.dtype == DenseArrayI64:
		for i, keep := range mask {
			if keep {
				dst.i64[i] = trueArray.i64[i]
			} else {
				dst.i64[i] = falseArray.i64[i]
			}
		}
	case dst.dtype == DenseArrayBool && trueArray.dtype == DenseArrayBool && falseArray.dtype == DenseArrayBool:
		for i, keep := range mask {
			if keep {
				dst.bools[i] = trueArray.bools[i]
			} else {
				dst.bools[i] = falseArray.bools[i]
			}
		}
	case trueArray.dtype == DenseArrayBool || falseArray.dtype == DenseArrayBool:
		return true, ErrDenseArrayDType
	default:
		return false, nil
	}
	dst.bumpVersion()
	return true, nil
}

func denseArraySumSelect(mask *DenseArray, trueValue, falseValue Value, length int) (Value, error) {
	if mask == nil || mask.dtype != DenseArrayBool {
		return NilValue(), fmt.Errorf("dense array sumSelect mask must be bool")
	}
	if mask.Len() != length {
		return NilValue(), ErrDenseArrayLength
	}
	trueArray, falseArray := trueValue.DenseArray(), falseValue.DenseArray()
	if trueArray != nil && trueArray.Len() != length {
		return NilValue(), ErrDenseArrayLength
	}
	if falseArray != nil && falseArray.Len() != length {
		return NilValue(), ErrDenseArrayLength
	}
	if trueArray != nil && falseArray != nil {
		switch {
		case trueArray.dtype == DenseArrayF64 && falseArray.dtype == DenseArrayF64:
			return FloatValue(denseArraySumSelectF64F64(mask.bools, trueArray.f64, falseArray.f64)), nil
		case trueArray.dtype == DenseArrayI64 && falseArray.dtype == DenseArrayI64:
			return IntValue(denseArraySumSelectI64I64(mask.bools, trueArray.i64, falseArray.i64)), nil
		case trueArray.dtype == DenseArrayBool || falseArray.dtype == DenseArrayBool:
			return NilValue(), ErrDenseArrayDType
		}
	}
	dtype, err := denseArraySelectDType(trueValue, falseValue, length)
	if err != nil {
		return NilValue(), err
	}
	switch dtype {
	case DenseArrayF64:
		tv := denseArrayF64Selector(trueValue)
		fv := denseArrayF64Selector(falseValue)
		sum := 0.0
		for i, keep := range mask.bools {
			if keep {
				sum += tv(i)
			} else {
				sum += fv(i)
			}
		}
		return FloatValue(sum), nil
	case DenseArrayI64:
		tv := denseArrayI64Selector(trueValue)
		fv := denseArrayI64Selector(falseValue)
		var sum int64
		for i, keep := range mask.bools {
			if keep {
				sum += tv(i)
			} else {
				sum += fv(i)
			}
		}
		return IntValue(sum), nil
	default:
		return NilValue(), ErrDenseArrayDType
	}
}

func denseArraySumSelectF64F64(mask []bool, ifTrue, ifFalse []float64) float64 {
	n := len(mask)
	if n == 0 {
		return 0
	}
	_, _, _ = mask[n-1], ifTrue[n-1], ifFalse[n-1]
	sum0, sum1, sum2, sum3 := 0.0, 0.0, 0.0, 0.0
	sum4, sum5, sum6, sum7 := 0.0, 0.0, 0.0, 0.0
	i := 0
	limit := n - n%8
	for ; i < limit; i += 8 {
		if mask[i] {
			sum0 += ifTrue[i]
		} else {
			sum0 += ifFalse[i]
		}
		if mask[i+1] {
			sum1 += ifTrue[i+1]
		} else {
			sum1 += ifFalse[i+1]
		}
		if mask[i+2] {
			sum2 += ifTrue[i+2]
		} else {
			sum2 += ifFalse[i+2]
		}
		if mask[i+3] {
			sum3 += ifTrue[i+3]
		} else {
			sum3 += ifFalse[i+3]
		}
		if mask[i+4] {
			sum4 += ifTrue[i+4]
		} else {
			sum4 += ifFalse[i+4]
		}
		if mask[i+5] {
			sum5 += ifTrue[i+5]
		} else {
			sum5 += ifFalse[i+5]
		}
		if mask[i+6] {
			sum6 += ifTrue[i+6]
		} else {
			sum6 += ifFalse[i+6]
		}
		if mask[i+7] {
			sum7 += ifTrue[i+7]
		} else {
			sum7 += ifFalse[i+7]
		}
	}
	sum := sum0 + sum1 + sum2 + sum3 + sum4 + sum5 + sum6 + sum7
	for ; i < n; i++ {
		if mask[i] {
			sum += ifTrue[i]
		} else {
			sum += ifFalse[i]
		}
	}
	return sum
}

func denseArraySumSelectI64I64(mask []bool, ifTrue, ifFalse []int64) int64 {
	n := len(mask)
	if n == 0 {
		return 0
	}
	_, _, _ = mask[n-1], ifTrue[n-1], ifFalse[n-1]
	var sum0, sum1, sum2, sum3 int64
	var sum4, sum5, sum6, sum7 int64
	i := 0
	limit := n - n%8
	for ; i < limit; i += 8 {
		if mask[i] {
			sum0 += ifTrue[i]
		} else {
			sum0 += ifFalse[i]
		}
		if mask[i+1] {
			sum1 += ifTrue[i+1]
		} else {
			sum1 += ifFalse[i+1]
		}
		if mask[i+2] {
			sum2 += ifTrue[i+2]
		} else {
			sum2 += ifFalse[i+2]
		}
		if mask[i+3] {
			sum3 += ifTrue[i+3]
		} else {
			sum3 += ifFalse[i+3]
		}
		if mask[i+4] {
			sum4 += ifTrue[i+4]
		} else {
			sum4 += ifFalse[i+4]
		}
		if mask[i+5] {
			sum5 += ifTrue[i+5]
		} else {
			sum5 += ifFalse[i+5]
		}
		if mask[i+6] {
			sum6 += ifTrue[i+6]
		} else {
			sum6 += ifFalse[i+6]
		}
		if mask[i+7] {
			sum7 += ifTrue[i+7]
		} else {
			sum7 += ifFalse[i+7]
		}
	}
	sum := sum0 + sum1 + sum2 + sum3 + sum4 + sum5 + sum6 + sum7
	for ; i < n; i++ {
		if mask[i] {
			sum += ifTrue[i]
		} else {
			sum += ifFalse[i]
		}
	}
	return sum
}

func denseArrayF64SelectorFast(v Value) (func(int) float64, bool) {
	if arr := v.DenseArray(); arr != nil {
		switch arr.dtype {
		case DenseArrayF64:
			return func(i int) float64 { return arr.f64[i] }, true
		case DenseArrayI64:
			return func(i int) float64 { return float64(arr.i64[i]) }, true
		default:
			return nil, false
		}
	}
	if !v.IsNumber() {
		return nil, false
	}
	x := v.Number()
	return func(int) float64 { return x }, true
}

func denseArrayI64SelectorFast(v Value) (func(int) int64, bool) {
	if arr := v.DenseArray(); arr != nil && arr.dtype == DenseArrayI64 {
		return func(i int) int64 { return arr.i64[i] }, true
	}
	if !v.IsInt() {
		return nil, false
	}
	x := v.Int()
	return func(int) int64 { return x }, true
}

func denseArrayBoolSelectorFast(v Value) (func(int) bool, bool) {
	if arr := v.DenseArray(); arr != nil && arr.dtype == DenseArrayBool {
		return func(i int) bool { return arr.bools[i] }, true
	}
	if !v.IsBool() {
		return nil, false
	}
	x := v.Bool()
	return func(int) bool { return x }, true
}

func denseArraySelectFast(mask []bool, trueValue, falseValue Value, length int) (*DenseArray, bool, error) {
	trueArray, falseArray := trueValue.DenseArray(), falseValue.DenseArray()
	if trueArray != nil && trueArray.Len() != length {
		return nil, true, ErrDenseArrayLength
	}
	if falseArray != nil && falseArray.Len() != length {
		return nil, true, ErrDenseArrayLength
	}
	switch {
	case trueArray != nil && falseArray != nil:
		return denseArraySelectArrayArrayFast(mask, trueArray, falseArray)
	case trueArray != nil:
		return denseArraySelectArrayScalarFast(mask, trueArray, falseValue)
	case falseArray != nil:
		return denseArraySelectScalarArrayFast(mask, trueValue, falseArray)
	default:
		return denseArraySelectScalarScalarFast(mask, trueValue, falseValue)
	}
}

func denseArraySelectArrayArrayFast(mask []bool, trueArray, falseArray *DenseArray) (*DenseArray, bool, error) {
	switch {
	case trueArray.dtype == DenseArrayF64 && falseArray.dtype == DenseArrayF64:
		out := make([]float64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = trueArray.f64[i]
			} else {
				out[i] = falseArray.f64[i]
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, true, nil
	case trueArray.dtype == DenseArrayI64 && falseArray.dtype == DenseArrayI64:
		out := make([]int64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = trueArray.i64[i]
			} else {
				out[i] = falseArray.i64[i]
			}
		}
		return &DenseArray{dtype: DenseArrayI64, i64: out}, true, nil
	case trueArray.dtype == DenseArrayBool && falseArray.dtype == DenseArrayBool:
		out := make([]bool, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = trueArray.bools[i]
			} else {
				out[i] = falseArray.bools[i]
			}
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, true, nil
	case trueArray.dtype == DenseArrayBool || falseArray.dtype == DenseArrayBool:
		return nil, true, ErrDenseArrayDType
	case trueArray.dtype == DenseArrayF64 && falseArray.dtype == DenseArrayI64:
		out := make([]float64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = trueArray.f64[i]
			} else {
				out[i] = float64(falseArray.i64[i])
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, true, nil
	case trueArray.dtype == DenseArrayI64 && falseArray.dtype == DenseArrayF64:
		out := make([]float64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = float64(trueArray.i64[i])
			} else {
				out[i] = falseArray.f64[i]
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, true, nil
	default:
		return nil, false, nil
	}
}

func denseArraySelectArrayScalarFast(mask []bool, trueArray *DenseArray, falseValue Value) (*DenseArray, bool, error) {
	switch {
	case trueArray.dtype == DenseArrayF64 && falseValue.IsNumber():
		x := falseValue.Number()
		out := make([]float64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = trueArray.f64[i]
			} else {
				out[i] = x
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, true, nil
	case trueArray.dtype == DenseArrayI64 && falseValue.IsInt():
		x := falseValue.Int()
		out := make([]int64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = trueArray.i64[i]
			} else {
				out[i] = x
			}
		}
		return &DenseArray{dtype: DenseArrayI64, i64: out}, true, nil
	case trueArray.dtype == DenseArrayI64 && falseValue.IsNumber():
		x := falseValue.Number()
		out := make([]float64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = float64(trueArray.i64[i])
			} else {
				out[i] = x
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, true, nil
	case trueArray.dtype == DenseArrayBool && falseValue.IsBool():
		x := falseValue.Bool()
		out := make([]bool, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = trueArray.bools[i]
			} else {
				out[i] = x
			}
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, true, nil
	case trueArray.dtype == DenseArrayBool || falseValue.IsBool():
		return nil, true, ErrDenseArrayDType
	default:
		return nil, false, nil
	}
}

func denseArraySelectScalarArrayFast(mask []bool, trueValue Value, falseArray *DenseArray) (*DenseArray, bool, error) {
	switch {
	case falseArray.dtype == DenseArrayF64 && trueValue.IsNumber():
		x := trueValue.Number()
		out := make([]float64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = x
			} else {
				out[i] = falseArray.f64[i]
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, true, nil
	case falseArray.dtype == DenseArrayI64 && trueValue.IsInt():
		x := trueValue.Int()
		out := make([]int64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = x
			} else {
				out[i] = falseArray.i64[i]
			}
		}
		return &DenseArray{dtype: DenseArrayI64, i64: out}, true, nil
	case falseArray.dtype == DenseArrayI64 && trueValue.IsNumber():
		x := trueValue.Number()
		out := make([]float64, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = x
			} else {
				out[i] = float64(falseArray.i64[i])
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, true, nil
	case falseArray.dtype == DenseArrayBool && trueValue.IsBool():
		x := trueValue.Bool()
		out := make([]bool, len(mask))
		for i, keep := range mask {
			if keep {
				out[i] = x
			} else {
				out[i] = falseArray.bools[i]
			}
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, true, nil
	case falseArray.dtype == DenseArrayBool || trueValue.IsBool():
		return nil, true, ErrDenseArrayDType
	default:
		return nil, false, nil
	}
}

func denseArraySelectScalarScalarFast(mask []bool, trueValue, falseValue Value) (*DenseArray, bool, error) {
	switch {
	case trueValue.IsBool() || falseValue.IsBool():
		if !trueValue.IsBool() || !falseValue.IsBool() {
			return nil, true, ErrDenseArrayDType
		}
		t, f := trueValue.Bool(), falseValue.Bool()
		out := make([]bool, len(mask))
		for i, keep := range mask {
			out[i] = f
			if keep {
				out[i] = t
			}
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, true, nil
	case trueValue.IsInt() && falseValue.IsInt():
		t, f := trueValue.Int(), falseValue.Int()
		out := make([]int64, len(mask))
		for i, keep := range mask {
			out[i] = f
			if keep {
				out[i] = t
			}
		}
		return &DenseArray{dtype: DenseArrayI64, i64: out}, true, nil
	case trueValue.IsNumber() && falseValue.IsNumber():
		t, f := trueValue.Number(), falseValue.Number()
		out := make([]float64, len(mask))
		for i, keep := range mask {
			out[i] = f
			if keep {
				out[i] = t
			}
		}
		return &DenseArray{dtype: DenseArrayF64, f64: out}, true, nil
	default:
		return nil, false, nil
	}
}

func denseArraySelectDType(trueValue, falseValue Value, length int) (DenseArrayDType, error) {
	trueKind, err := denseArraySelectOperandKind(trueValue, length)
	if err != nil {
		return DenseArrayF64, err
	}
	falseKind, err := denseArraySelectOperandKind(falseValue, length)
	if err != nil {
		return DenseArrayF64, err
	}
	if trueKind == DenseArrayBool || falseKind == DenseArrayBool {
		if trueKind == DenseArrayBool && falseKind == DenseArrayBool {
			return DenseArrayBool, nil
		}
		return DenseArrayF64, ErrDenseArrayDType
	}
	if trueKind == DenseArrayF64 || falseKind == DenseArrayF64 {
		return DenseArrayF64, nil
	}
	return DenseArrayI64, nil
}

func denseArraySelectOperandKind(v Value, length int) (DenseArrayDType, error) {
	if arr := v.DenseArray(); arr != nil {
		if arr.Len() != length {
			return DenseArrayF64, ErrDenseArrayLength
		}
		return arr.dtype, nil
	}
	switch {
	case v.IsBool():
		return DenseArrayBool, nil
	case v.IsInt():
		return DenseArrayI64, nil
	case v.IsNumber():
		return DenseArrayF64, nil
	default:
		return DenseArrayF64, ErrDenseArrayScalar
	}
}

func denseArrayF64Selector(v Value) func(int) float64 {
	if arr := v.DenseArray(); arr != nil {
		switch arr.dtype {
		case DenseArrayF64:
			return func(i int) float64 { return arr.f64[i] }
		case DenseArrayI64:
			return func(i int) float64 { return float64(arr.i64[i]) }
		}
	}
	x := v.Number()
	return func(int) float64 { return x }
}

func denseArrayI64Selector(v Value) func(int) int64 {
	if arr := v.DenseArray(); arr != nil && arr.dtype == DenseArrayI64 {
		return func(i int) int64 { return arr.i64[i] }
	}
	x := v.Int()
	return func(int) int64 { return x }
}

func denseArrayBoolSelector(v Value) func(int) bool {
	if arr := v.DenseArray(); arr != nil && arr.dtype == DenseArrayBool {
		return func(i int) bool { return arr.bools[i] }
	}
	x := v.Bool()
	return func(int) bool { return x }
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

func (a *DenseArray) MeanWhere(mask *DenseArray) (Value, error) {
	if a == nil || mask == nil {
		return NilValue(), ErrDenseArrayOperand
	}
	if mask.dtype != DenseArrayBool {
		return NilValue(), fmt.Errorf("dense array meanWhere mask must be bool")
	}
	if a.Len() != mask.Len() {
		return NilValue(), ErrDenseArrayLength
	}
	sum := 0.0
	count := 0
	switch a.dtype {
	case DenseArrayF64:
		for i, keep := range mask.bools {
			if keep {
				sum += a.f64[i]
				count++
			}
		}
	case DenseArrayI64:
		for i, keep := range mask.bools {
			if keep {
				sum += float64(a.i64[i])
				count++
			}
		}
	default:
		return NilValue(), ErrDenseArrayDType
	}
	if count == 0 {
		return NilValue(), ErrDenseArrayEmpty
	}
	return FloatValue(sum / float64(count)), nil
}

func (a *DenseArray) MinWhere(mask *DenseArray) (Value, error) {
	return a.extremeWhere(mask, false)
}

func (a *DenseArray) MaxWhere(mask *DenseArray) (Value, error) {
	return a.extremeWhere(mask, true)
}

func (a *DenseArray) StatsWhere(mask *DenseArray) (*Table, error) {
	if a == nil || mask == nil {
		return nil, ErrDenseArrayOperand
	}
	if mask.dtype != DenseArrayBool {
		return nil, fmt.Errorf("dense array statsWhere mask must be bool")
	}
	if a.Len() != mask.Len() {
		return nil, ErrDenseArrayLength
	}
	out := NewTable()
	switch a.dtype {
	case DenseArrayF64:
		var sum, min, max float64
		count := 0
		for i, keep := range mask.bools {
			if !keep {
				continue
			}
			v := a.f64[i]
			sum += v
			if count == 0 || v < min {
				min = v
			}
			if count == 0 || v > max {
				max = v
			}
			count++
		}
		out.RawSetString("count", IntValue(int64(count)))
		out.RawSetString("sum", FloatValue(sum))
		if count == 0 {
			out.RawSetString("min", NilValue())
			out.RawSetString("max", NilValue())
			out.RawSetString("mean", NilValue())
			return out, nil
		}
		out.RawSetString("min", FloatValue(min))
		out.RawSetString("max", FloatValue(max))
		out.RawSetString("mean", FloatValue(sum/float64(count)))
		return out, nil
	case DenseArrayI64:
		var sum, min, max int64
		count := 0
		for i, keep := range mask.bools {
			if !keep {
				continue
			}
			v := a.i64[i]
			sum += v
			if count == 0 || v < min {
				min = v
			}
			if count == 0 || v > max {
				max = v
			}
			count++
		}
		out.RawSetString("count", IntValue(int64(count)))
		out.RawSetString("sum", IntValue(sum))
		if count == 0 {
			out.RawSetString("min", NilValue())
			out.RawSetString("max", NilValue())
			out.RawSetString("mean", NilValue())
			return out, nil
		}
		out.RawSetString("min", IntValue(min))
		out.RawSetString("max", IntValue(max))
		out.RawSetString("mean", FloatValue(float64(sum)/float64(count)))
		return out, nil
	default:
		return nil, ErrDenseArrayDType
	}
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

func DenseArrayIndexFromValue(index Value, length int) (int, bool, error) {
	if index.IsInt() {
		i, err := denseArrayOneBasedIndex(index.Int(), length)
		return i, true, err
	}
	if index.IsFloat() {
		f := index.Float()
		i := int64(f)
		if f != float64(i) {
			return 0, false, nil
		}
		out, err := denseArrayOneBasedIndex(i, length)
		return out, true, err
	}
	return 0, false, nil
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

func denseArrayCompareMask(left *DenseArray, op DenseArrayBinaryOp, right Value) (*DenseArray, error) {
	if left == nil {
		return nil, ErrDenseArrayOperand
	}
	if !isComparisonOp(op) {
		return nil, ErrDenseArrayDType
	}
	if rightArray := right.DenseArray(); rightArray != nil {
		return denseArrayCompareMaskArray(left, op, rightArray)
	}
	return denseArrayCompareMaskScalar(left, op, right)
}

func denseArrayCompareMaskArray(left *DenseArray, op DenseArrayBinaryOp, right *DenseArray) (*DenseArray, error) {
	if right == nil {
		return nil, ErrDenseArrayOperand
	}
	if left.Len() != right.Len() {
		return nil, ErrDenseArrayLength
	}
	out := make([]bool, left.Len())
	if (op == DenseArrayEQ || op == DenseArrayNE) && left.dtype == DenseArrayBool && right.dtype == DenseArrayBool {
		for i := range out {
			out[i] = compareBools(op, left.bools[i], right.bools[i])
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, nil
	}
	if left.dtype == DenseArrayBool || right.dtype == DenseArrayBool {
		return nil, ErrDenseArrayDType
	}
	switch {
	case left.dtype == DenseArrayF64 && right.dtype == DenseArrayF64:
		denseArrayCompareF64F64Into(out, op, left.f64, right.f64)
	case left.dtype == DenseArrayF64 && right.dtype == DenseArrayI64:
		denseArrayCompareF64I64Into(out, op, left.f64, right.i64)
	case left.dtype == DenseArrayI64 && right.dtype == DenseArrayF64:
		denseArrayCompareI64F64Into(out, op, left.i64, right.f64)
	case left.dtype == DenseArrayI64 && right.dtype == DenseArrayI64:
		denseArrayCompareI64I64Into(out, op, left.i64, right.i64)
	default:
		return nil, ErrDenseArrayDType
	}
	return &DenseArray{dtype: DenseArrayBool, bools: out}, nil
}

func denseArrayCompareMaskScalar(left *DenseArray, op DenseArrayBinaryOp, right Value) (*DenseArray, error) {
	out := make([]bool, left.Len())
	if (op == DenseArrayEQ || op == DenseArrayNE) && left.dtype == DenseArrayBool && right.IsBool() {
		r := right.Bool()
		for i := range out {
			out[i] = compareBools(op, left.bools[i], r)
		}
		return &DenseArray{dtype: DenseArrayBool, bools: out}, nil
	}
	if left.dtype == DenseArrayBool {
		return nil, ErrDenseArrayDType
	}
	if !right.IsNumber() {
		return nil, ErrDenseArrayScalar
	}
	if left.dtype == DenseArrayI64 && right.IsInt() {
		r := right.Int()
		denseArrayCompareI64ScalarInto(out, op, left.i64, r)
		return &DenseArray{dtype: DenseArrayBool, bools: out}, nil
	}
	r := right.Number()
	switch left.dtype {
	case DenseArrayF64:
		denseArrayCompareF64ScalarInto(out, op, left.f64, r)
	case DenseArrayI64:
		denseArrayCompareI64F64ScalarInto(out, op, left.i64, r)
	default:
		return nil, ErrDenseArrayDType
	}
	return &DenseArray{dtype: DenseArrayBool, bools: out}, nil
}

func denseArrayCompareF64F64Into(out []bool, op DenseArrayBinaryOp, left, right []float64) {
	switch op {
	case DenseArrayEQ:
		for i, v := range left {
			out[i] = v == right[i]
		}
	case DenseArrayNE:
		for i, v := range left {
			out[i] = v != right[i]
		}
	case DenseArrayLT:
		for i, v := range left {
			out[i] = v < right[i]
		}
	case DenseArrayLE:
		for i, v := range left {
			out[i] = v <= right[i]
		}
	case DenseArrayGT:
		for i, v := range left {
			out[i] = v > right[i]
		}
	case DenseArrayGE:
		for i, v := range left {
			out[i] = v >= right[i]
		}
	}
}

func denseArrayCompareF64I64Into(out []bool, op DenseArrayBinaryOp, left []float64, right []int64) {
	switch op {
	case DenseArrayEQ:
		for i, v := range left {
			out[i] = v == float64(right[i])
		}
	case DenseArrayNE:
		for i, v := range left {
			out[i] = v != float64(right[i])
		}
	case DenseArrayLT:
		for i, v := range left {
			out[i] = v < float64(right[i])
		}
	case DenseArrayLE:
		for i, v := range left {
			out[i] = v <= float64(right[i])
		}
	case DenseArrayGT:
		for i, v := range left {
			out[i] = v > float64(right[i])
		}
	case DenseArrayGE:
		for i, v := range left {
			out[i] = v >= float64(right[i])
		}
	}
}

func denseArrayCompareI64F64Into(out []bool, op DenseArrayBinaryOp, left []int64, right []float64) {
	switch op {
	case DenseArrayEQ:
		for i, v := range left {
			out[i] = float64(v) == right[i]
		}
	case DenseArrayNE:
		for i, v := range left {
			out[i] = float64(v) != right[i]
		}
	case DenseArrayLT:
		for i, v := range left {
			out[i] = float64(v) < right[i]
		}
	case DenseArrayLE:
		for i, v := range left {
			out[i] = float64(v) <= right[i]
		}
	case DenseArrayGT:
		for i, v := range left {
			out[i] = float64(v) > right[i]
		}
	case DenseArrayGE:
		for i, v := range left {
			out[i] = float64(v) >= right[i]
		}
	}
}

func denseArrayCompareI64I64Into(out []bool, op DenseArrayBinaryOp, left, right []int64) {
	switch op {
	case DenseArrayEQ:
		for i, v := range left {
			out[i] = v == right[i]
		}
	case DenseArrayNE:
		for i, v := range left {
			out[i] = v != right[i]
		}
	case DenseArrayLT:
		for i, v := range left {
			out[i] = v < right[i]
		}
	case DenseArrayLE:
		for i, v := range left {
			out[i] = v <= right[i]
		}
	case DenseArrayGT:
		for i, v := range left {
			out[i] = v > right[i]
		}
	case DenseArrayGE:
		for i, v := range left {
			out[i] = v >= right[i]
		}
	}
}

func denseArrayCompareF64ScalarInto(out []bool, op DenseArrayBinaryOp, left []float64, right float64) {
	switch op {
	case DenseArrayEQ:
		for i, v := range left {
			out[i] = v == right
		}
	case DenseArrayNE:
		for i, v := range left {
			out[i] = v != right
		}
	case DenseArrayLT:
		for i, v := range left {
			out[i] = v < right
		}
	case DenseArrayLE:
		for i, v := range left {
			out[i] = v <= right
		}
	case DenseArrayGT:
		for i, v := range left {
			out[i] = v > right
		}
	case DenseArrayGE:
		for i, v := range left {
			out[i] = v >= right
		}
	}
}

func denseArrayCompareI64F64ScalarInto(out []bool, op DenseArrayBinaryOp, left []int64, right float64) {
	switch op {
	case DenseArrayEQ:
		for i, v := range left {
			out[i] = float64(v) == right
		}
	case DenseArrayNE:
		for i, v := range left {
			out[i] = float64(v) != right
		}
	case DenseArrayLT:
		for i, v := range left {
			out[i] = float64(v) < right
		}
	case DenseArrayLE:
		for i, v := range left {
			out[i] = float64(v) <= right
		}
	case DenseArrayGT:
		for i, v := range left {
			out[i] = float64(v) > right
		}
	case DenseArrayGE:
		for i, v := range left {
			out[i] = float64(v) >= right
		}
	}
}

func denseArrayCompareI64ScalarInto(out []bool, op DenseArrayBinaryOp, left []int64, right int64) {
	switch op {
	case DenseArrayEQ:
		for i, v := range left {
			out[i] = v == right
		}
	case DenseArrayNE:
		for i, v := range left {
			out[i] = v != right
		}
	case DenseArrayLT:
		for i, v := range left {
			out[i] = v < right
		}
	case DenseArrayLE:
		for i, v := range left {
			out[i] = v <= right
		}
	case DenseArrayGT:
		for i, v := range left {
			out[i] = v > right
		}
	case DenseArrayGE:
		for i, v := range left {
			out[i] = v >= right
		}
	}
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

func denseArrayAffineWhere(dst, src, mask *DenseArray, scale, bias float64) error {
	if dst == nil || src == nil || mask == nil {
		return ErrDenseArrayOperand
	}
	if dst.Len() != src.Len() || dst.Len() != mask.Len() {
		return ErrDenseArrayLength
	}
	if mask.dtype != DenseArrayBool {
		return fmt.Errorf("dense array affineWhere mask must be bool")
	}
	if dst.dtype != DenseArrayF64 || src.dtype == DenseArrayBool {
		return fmt.Errorf("dense array affineWhere requires f64 destination and numeric source")
	}
	switch src.dtype {
	case DenseArrayF64:
		for i, keep := range mask.bools {
			if keep {
				dst.f64[i] = src.f64[i]*scale + bias
			}
		}
	case DenseArrayI64:
		for i, keep := range mask.bools {
			if keep {
				dst.f64[i] = float64(src.i64[i])*scale + bias
			}
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

func compareInt64(op DenseArrayBinaryOp, left, right int64) bool {
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
