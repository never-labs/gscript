package runtime

import "fmt"

// NewDenseArrayF64FromCopy allocates a DenseArray once and lets copyValues fill
// its backing slice directly. It is intended for typed columnar bridges that can
// bulk-copy into a caller-provided destination.
func NewDenseArrayF64FromCopy(n int, copyValues func([]float64) error) (*DenseArray, error) {
	if n < 0 {
		return nil, fmt.Errorf("dense array length must be non-negative")
	}
	if copyValues == nil {
		return nil, ErrDenseArrayOperand
	}
	out := &DenseArray{dtype: DenseArrayF64, f64: make([]float64, n)}
	if err := copyValues(out.f64); err != nil {
		return nil, err
	}
	return out, nil
}

// NewDenseArrayI64FromCopy allocates a DenseArray once and lets copyValues fill
// its backing slice directly.
func NewDenseArrayI64FromCopy(n int, copyValues func([]int64) error) (*DenseArray, error) {
	if n < 0 {
		return nil, fmt.Errorf("dense array length must be non-negative")
	}
	if copyValues == nil {
		return nil, ErrDenseArrayOperand
	}
	out := &DenseArray{dtype: DenseArrayI64, i64: make([]int64, n)}
	if err := copyValues(out.i64); err != nil {
		return nil, err
	}
	return out, nil
}

// NewDenseArrayBoolFromCopy allocates a DenseArray once and lets copyValues
// fill its backing slice directly.
func NewDenseArrayBoolFromCopy(n int, copyValues func([]bool) error) (*DenseArray, error) {
	if n < 0 {
		return nil, fmt.Errorf("dense array length must be non-negative")
	}
	if copyValues == nil {
		return nil, ErrDenseArrayOperand
	}
	out := &DenseArray{dtype: DenseArrayBool, bools: make([]bool, n)}
	if err := copyValues(out.bools); err != nil {
		return nil, err
	}
	return out, nil
}

// NewDenseArrayStringFromCopy allocates a DenseArray once and lets copyValues
// fill its backing slice directly.
func NewDenseArrayStringFromCopy(n int, copyValues func([]string) error) (*DenseArray, error) {
	if n < 0 {
		return nil, fmt.Errorf("dense array length must be non-negative")
	}
	if copyValues == nil {
		return nil, ErrDenseArrayOperand
	}
	out := &DenseArray{dtype: DenseArrayString, strings: make([]string, n)}
	if err := copyValues(out.strings); err != nil {
		return nil, err
	}
	return out, nil
}
