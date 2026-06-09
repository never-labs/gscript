package data

import (
	"fmt"
	"math"
)

// Matrix is the runtime representation for rectangular list-of-lists values.
// It deliberately remains an Array: q sees a matrix as a list of rows, while
// typed runtime/JIT kernels can inspect Shape and Cell without materializing
// nested any slices.
type Matrix interface {
	Array
	Shape() []int
	Cell(row, col int) (any, bool)
	RowArray(row int) (Array, bool)
}

type matrixArray struct {
	shape []int
	data  Array
}

type matrixRowArray struct {
	matrix matrixArray
	row    int
}

type transposedMatrixArray struct {
	source Matrix
	rows   int
	cols   int
}

type transposedMatrixRowArray struct {
	matrix transposedMatrixArray
	row    int
}

// ReshapeArray returns a q-style reshape view. Scalar shapes behave like take;
// two-dimensional shapes are represented as a matrix/list-of-lists value.
func ReshapeArray(shape []int, source Array) (Array, error) {
	if source == nil {
		return nil, fmt.Errorf("reshape source is nil")
	}
	if len(shape) == 0 {
		return nil, fmt.Errorf("reshape expects at least one dimension")
	}
	dims := append([]int(nil), shape...)
	total := 1
	for _, dim := range dims {
		if dim < 0 {
			return nil, fmt.Errorf("reshape dimension %d must be non-negative", dim)
		}
		total *= dim
	}
	values, err := TakeRepeat(source, total)
	if err != nil {
		return nil, err
	}
	if len(dims) == 1 {
		return values, nil
	}
	if len(dims) != 2 {
		return nil, fmt.Errorf("reshape currently supports one or two dimensions, got %d", len(dims))
	}
	return matrixArray{shape: dims, data: values}, nil
}

// TransposeMatrix returns a view over a rectangular matrix/list-of-lists value.
func TransposeMatrix(matrix Matrix) (Array, error) {
	if matrix == nil {
		return nil, fmt.Errorf("transpose matrix is nil")
	}
	shape := matrix.Shape()
	if len(shape) != 2 {
		return nil, fmt.Errorf("transpose expects a two-dimensional matrix")
	}
	return transposedMatrixArray{source: matrix, rows: shape[1], cols: shape[0]}, nil
}

// MatrixMultiplyNumeric multiplies two numeric two-dimensional matrices. The
// f64 result keeps the semantic layer simple while leaving a single replacement
// point for future typed BLAS-style kernels.
func MatrixMultiplyNumeric(left, right Matrix) (Array, error) {
	if left == nil || right == nil {
		return nil, fmt.Errorf("matrix multiply expects two matrices")
	}
	leftShape := left.Shape()
	rightShape := right.Shape()
	if len(leftShape) != 2 || len(rightShape) != 2 {
		return nil, fmt.Errorf("matrix multiply expects two-dimensional matrices")
	}
	if leftShape[1] != rightShape[0] {
		return nil, fmt.Errorf("matrix multiply shape %dx%d cannot conform to %dx%d", leftShape[0], leftShape[1], rightShape[0], rightShape[1])
	}
	out := make([]float64, leftShape[0]*rightShape[1])
	for row := 0; row < leftShape[0]; row++ {
		for col := 0; col < rightShape[1]; col++ {
			sum := float64(0)
			for inner := 0; inner < leftShape[1]; inner++ {
				lv, ok := left.Cell(row, inner)
				if !ok {
					return nil, fmt.Errorf("matrix multiply left cell %d,%d out of range", row, inner)
				}
				rv, ok := right.Cell(inner, col)
				if !ok {
					return nil, fmt.Errorf("matrix multiply right cell %d,%d out of range", inner, col)
				}
				ln, lok := numeric(lv)
				rn, rok := numeric(rv)
				if !lok || !rok {
					return nil, fmt.Errorf("matrix multiply expects numeric cells")
				}
				sum += ln * rn
			}
			out[row*rightShape[1]+col] = sum
		}
	}
	return matrixArray{shape: []int{leftShape[0], rightShape[1]}, data: NewF64(out)}, nil
}

// MatrixInverseNumeric inverts a numeric square matrix with Gauss-Jordan
// elimination. It is intentionally centralized here so future typed kernels can
// replace the implementation without changing q eval semantics.
func MatrixInverseNumeric(matrix Matrix) (Array, error) {
	if matrix == nil {
		return nil, fmt.Errorf("matrix inverse expects a matrix")
	}
	shape := matrix.Shape()
	if len(shape) != 2 {
		return nil, fmt.Errorf("matrix inverse expects a two-dimensional matrix")
	}
	n := shape[0]
	if n != shape[1] {
		return nil, fmt.Errorf("matrix inverse expects a square matrix, got %dx%d", shape[0], shape[1])
	}
	if n == 0 {
		return matrixArray{shape: []int{0, 0}, data: NewF64(nil)}, nil
	}
	aug := make([]float64, n*2*n)
	for row := 0; row < n; row++ {
		for col := 0; col < n; col++ {
			value, ok := matrix.Cell(row, col)
			if !ok {
				return nil, fmt.Errorf("matrix inverse cell %d,%d out of range", row, col)
			}
			num, ok := numeric(value)
			if !ok {
				return nil, fmt.Errorf("matrix inverse expects numeric cells")
			}
			aug[row*2*n+col] = num
		}
		aug[row*2*n+n+row] = 1
	}
	for col := 0; col < n; col++ {
		pivotRow := col
		pivotAbs := math.Abs(aug[pivotRow*2*n+col])
		for row := col + 1; row < n; row++ {
			candidate := math.Abs(aug[row*2*n+col])
			if candidate > pivotAbs {
				pivotAbs = candidate
				pivotRow = row
			}
		}
		if pivotAbs == 0 {
			return nil, fmt.Errorf("matrix inverse expects a non-singular matrix")
		}
		if pivotRow != col {
			for j := 0; j < 2*n; j++ {
				aug[col*2*n+j], aug[pivotRow*2*n+j] = aug[pivotRow*2*n+j], aug[col*2*n+j]
			}
		}
		pivot := aug[col*2*n+col]
		for j := 0; j < 2*n; j++ {
			aug[col*2*n+j] /= pivot
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := aug[row*2*n+col]
			if factor == 0 {
				continue
			}
			for j := 0; j < 2*n; j++ {
				aug[row*2*n+j] -= factor * aug[col*2*n+j]
			}
		}
	}
	out := make([]float64, n*n)
	for row := 0; row < n; row++ {
		copy(out[row*n:(row+1)*n], aug[row*2*n+n:row*2*n+2*n])
	}
	return matrixArray{shape: []int{n, n}, data: NewF64(out)}, nil
}

func (m matrixArray) Kind() Kind { return KindAny }

func (m matrixArray) Len() int {
	if len(m.shape) == 0 {
		return 0
	}
	return m.shape[0]
}

func (m matrixArray) At(row int) (any, bool) {
	return m.RowArray(row)
}

func (m matrixArray) Values() []any {
	out := make([]any, m.Len())
	for row := range out {
		value, ok := m.At(row)
		if !ok {
			panic(fmt.Sprintf("data matrix row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (m matrixArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		value, ok := m.At(row)
		if !ok {
			panic(fmt.Sprintf("data matrix gather row %d out of range", row))
		}
		out[i] = value
	}
	return NewAny(out)
}

func (m matrixArray) Shape() []int {
	return append([]int(nil), m.shape...)
}

func (m matrixArray) Cell(row, col int) (any, bool) {
	if len(m.shape) != 2 || row < 0 || col < 0 || row >= m.shape[0] || col >= m.shape[1] {
		return nil, false
	}
	return m.data.At(row*m.shape[1] + col)
}

func (m matrixArray) RowArray(row int) (Array, bool) {
	if len(m.shape) != 2 || row < 0 || row >= m.shape[0] {
		return nil, false
	}
	return matrixRowArray{matrix: m, row: row}, true
}

func (r matrixRowArray) Kind() Kind { return r.matrix.data.Kind() }

func (r matrixRowArray) Len() int { return r.matrix.shape[1] }

func (r matrixRowArray) At(col int) (any, bool) {
	return r.matrix.Cell(r.row, col)
}

func (r matrixRowArray) Values() []any {
	out := make([]any, r.Len())
	for col := range out {
		value, ok := r.At(col)
		if !ok {
			panic(fmt.Sprintf("data matrix row col %d out of range", col))
		}
		out[col] = value
	}
	return out
}

func (r matrixRowArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, col := range indexes {
		value, ok := r.At(col)
		if !ok {
			panic(fmt.Sprintf("data matrix row gather col %d out of range", col))
		}
		out[i] = value
	}
	return InferArray(out)
}

func (m transposedMatrixArray) Kind() Kind { return KindAny }

func (m transposedMatrixArray) Len() int { return m.rows }

func (m transposedMatrixArray) At(row int) (any, bool) {
	return m.RowArray(row)
}

func (m transposedMatrixArray) Values() []any {
	out := make([]any, m.Len())
	for row := range out {
		value, ok := m.At(row)
		if !ok {
			panic(fmt.Sprintf("data transpose row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (m transposedMatrixArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		value, ok := m.At(row)
		if !ok {
			panic(fmt.Sprintf("data transpose gather row %d out of range", row))
		}
		out[i] = value
	}
	return NewAny(out)
}

func (m transposedMatrixArray) Shape() []int {
	return []int{m.rows, m.cols}
}

func (m transposedMatrixArray) Cell(row, col int) (any, bool) {
	if row < 0 || col < 0 || row >= m.rows || col >= m.cols {
		return nil, false
	}
	return m.source.Cell(col, row)
}

func (m transposedMatrixArray) RowArray(row int) (Array, bool) {
	if row < 0 || row >= m.rows {
		return nil, false
	}
	return transposedMatrixRowArray{matrix: m, row: row}, true
}

func (r transposedMatrixRowArray) Kind() Kind {
	shape := r.matrix.source.Shape()
	if len(shape) == 2 && shape[0] > 0 {
		if row, ok := r.matrix.source.RowArray(0); ok {
			return row.Kind()
		}
	}
	return KindAny
}

func (r transposedMatrixRowArray) Len() int { return r.matrix.cols }

func (r transposedMatrixRowArray) At(col int) (any, bool) {
	return r.matrix.Cell(r.row, col)
}

func (r transposedMatrixRowArray) Values() []any {
	out := make([]any, r.Len())
	for col := range out {
		value, ok := r.At(col)
		if !ok {
			panic(fmt.Sprintf("data transpose row col %d out of range", col))
		}
		out[col] = value
	}
	return out
}

func (r transposedMatrixRowArray) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, col := range indexes {
		value, ok := r.At(col)
		if !ok {
			panic(fmt.Sprintf("data transpose row gather col %d out of range", col))
		}
		out[i] = value
	}
	return InferArray(out)
}
