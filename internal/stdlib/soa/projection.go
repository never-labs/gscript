package soa

import "fmt"

const (
	DTypeBool = "bool"
	DTypeI64  = "i64"
)

// ColumnProjection is the runtime-independent shape of a column projection
// after script values have been adapted to plain Go values.
type ColumnProjection struct {
	Name string
}

func NewColumnProjection(name string) ColumnProjection {
	return ColumnProjection{Name: name}
}

// DenseArrayMeta is the cacheable metadata for a dense array argument.
type DenseArrayMeta struct {
	Present bool
	DType   string
	Version uint64
}

func NewDenseArrayMeta(dtype string, version uint64) DenseArrayMeta {
	return DenseArrayMeta{Present: true, DType: dtype, Version: version}
}

func NoDenseArrayMeta() DenseArrayMeta {
	return DenseArrayMeta{}
}

func RequireBoolMask(dtype string, version uint64, name string) (DenseArrayMeta, error) {
	if dtype != DTypeBool {
		return DenseArrayMeta{}, fmt.Errorf("%s mask must be a bool dense array", name)
	}
	return NewDenseArrayMeta(dtype, version), nil
}

func RequireI64Indices(dtype string, version uint64, name string) (DenseArrayMeta, error) {
	if dtype != DTypeI64 {
		return DenseArrayMeta{}, fmt.Errorf("%s indices must be an i64 dense array", name)
	}
	return NewDenseArrayMeta(dtype, version), nil
}

// MaskQuery is the runtime-independent cache key for soa.mask after column
// lookup has resolved any string RHS into a column reference.
type MaskQuery struct {
	Left      ColumnProjection
	Op        string
	RHSColumn ColumnProjection
	RHSIsCol  bool
	LeftMeta  DenseArrayMeta
	RightMeta DenseArrayMeta
}

func NewMaskQuery(leftName, op string, left DenseArrayMeta, rhsIsColumn bool, rhsColumn string, right DenseArrayMeta) MaskQuery {
	return MaskQuery{
		Left:      NewColumnProjection(leftName),
		Op:        op,
		RHSColumn: NewColumnProjection(rhsColumn),
		RHSIsCol:  rhsIsColumn,
		LeftMeta:  left,
		RightMeta: right,
	}
}
