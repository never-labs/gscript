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

type ndArrayView struct {
	shape  []int
	data   Array
	offset int
}

type ndMatrixView struct {
	shape  []int
	data   Array
	offset int
}

func (m transposedMatrixArray) SourceMatrix() Matrix {
	return m.source
}

// ReshapeArray returns a q-style reshape view. Scalar shapes behave like take;
// two-dimensional shapes are represented as a matrix/list-of-lists value, and
// higher-dimensional shapes are represented as nested row-major views.
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
	if len(dims) == 2 {
		return matrixArray{shape: dims, data: values}, nil
	}
	return ndArrayView{shape: dims, data: values}, nil
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

// MatrixFromRows recognizes a rectangular list-of-lists as a Matrix. Existing
// Matrix values pass through unchanged; non-matrix arrays return ok=false.
func MatrixFromRows(value Array) (Matrix, bool, error) {
	if matrix, ok := value.(Matrix); ok {
		return matrix, true, nil
	}
	if value == nil || value.Len() == 0 {
		return nil, false, nil
	}
	rows := value.Len()
	cols := 0
	flat := make([]any, 0)
	for row := 0; row < rows; row++ {
		item, ok := value.At(row)
		if !ok {
			return nil, true, fmt.Errorf("matrix row %d out of range", row)
		}
		rowArray, ok := item.(Array)
		if !ok {
			return nil, false, nil
		}
		if row == 0 {
			cols = rowArray.Len()
		} else if rowArray.Len() != cols {
			return nil, true, fmt.Errorf("matrix row %d length %d does not match %d", row, rowArray.Len(), cols)
		}
		flat = append(flat, rowArray.Values()...)
	}
	reshaped, err := ReshapeArray([]int{rows, cols}, InferArray(flat))
	if err != nil {
		return nil, true, err
	}
	matrix, ok := reshaped.(Matrix)
	if !ok {
		return nil, true, fmt.Errorf("matrix reshape did not produce matrix")
	}
	return matrix, true, nil
}

// TryRowArrayIndex returns a row view for matrix and list-of-lists values.
// It is narrower than general indexing so callers can probe it before scalar
// Array.At and preserve typed row views without materializing nested Values.
func TryRowArrayIndex(value Array, row int) (Array, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	if row < 0 {
		return nil, true, fmt.Errorf("matrix row index must be non-negative")
	}
	if matrix, ok := value.(Matrix); ok {
		rowArray, ok := matrix.RowArray(row)
		if !ok {
			return nil, true, fmt.Errorf("matrix row index %d out of range", row)
		}
		return rowArray, true, nil
	}
	if row >= value.Len() {
		return nil, false, nil
	}
	item, ok := value.At(row)
	if !ok {
		return nil, true, fmt.Errorf("matrix row index %d out of range", row)
	}
	rowArray, ok := item.(Array)
	if !ok {
		return nil, false, nil
	}
	return rowArray, true, nil
}

// TryMatrixCellIndex returns a single matrix cell without materializing row
// arrays or routing through generic nested indexing.
func TryMatrixCellIndex(value Matrix, row, col int) (any, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	if row < 0 || col < 0 {
		return nil, true, fmt.Errorf("matrix cell index must be non-negative")
	}
	if cell, ok, err := tryMatrixCellIndexDirect(value, row, col); ok || err != nil {
		return cell, ok, err
	}
	cell, ok := value.Cell(row, col)
	if !ok {
		return nil, true, fmt.Errorf("matrix cell index %d %d out of range", row, col)
	}
	return cell, true, nil
}

func tryMatrixCellIndexDirect(value Matrix, row, col int) (any, bool, error) {
	switch matrix := value.(type) {
	case matrixArray:
		if len(matrix.shape) != 2 || row >= matrix.shape[0] || col >= matrix.shape[1] {
			return nil, true, fmt.Errorf("matrix cell index %d %d out of range", row, col)
		}
		return arrayScalarAt(matrix.data, row*matrix.shape[1]+col)
	case transposedMatrixArray:
		if row >= matrix.rows || col >= matrix.cols {
			return nil, true, fmt.Errorf("matrix cell index %d %d out of range", row, col)
		}
		return TryMatrixCellIndex(matrix.source, col, row)
	default:
		return nil, false, nil
	}
}

func arrayScalarAt(array Array, index int) (any, bool, error) {
	if array == nil {
		return nil, true, fmt.Errorf("array is nil")
	}
	if index < 0 || index >= array.Len() {
		return nil, true, fmt.Errorf("index %d out of range", index)
	}
	switch a := array.(type) {
	case attributedArray:
		return arrayScalarAt(a.array, index)
	case columnArray[int8]:
		return int64(a.data[index]), true, nil
	case columnArray[int16]:
		return int64(a.data[index]), true, nil
	case columnArray[int32]:
		return int64(a.data[index]), true, nil
	case columnArray[int64]:
		return a.data[index], true, nil
	case columnArray[uint8]:
		return int64(a.data[index]), true, nil
	case columnArray[uint16]:
		return int64(a.data[index]), true, nil
	case columnArray[uint32]:
		return int64(a.data[index]), true, nil
	case columnArray[uint64]:
		value := a.data[index]
		if value <= uint64(1<<63-1) {
			return int64(value), true, nil
		}
		return value, true, nil
	case columnArray[float32]:
		return float64(a.data[index]), true, nil
	case columnArray[float64]:
		return a.data[index], true, nil
	case i64RangeArray:
		return a.start + int64(index)*a.step, true, nil
	case f64RangeArray:
		return a.start + float64(index)*a.step, true, nil
	default:
		value, ok := array.At(index)
		if !ok {
			return nil, true, fmt.Errorf("index %d out of range", index)
		}
		return value, true, nil
	}
}

// TryMatrixRowNumericSum reduces a matrix row through the typed runtime path.
// It is shared by q matrix indexing and generic Leia/data callers.
func TryMatrixRowNumericSum(value Matrix, row int) (any, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	if sum, handled, err := tryMatrixRowNumericSumDirect(value, row); handled || err != nil {
		return sum, handled, err
	}
	rowArray, ok := value.RowArray(row)
	if !ok {
		return nil, true, fmt.Errorf("matrix row index %d out of range", row)
	}
	return typedKernels.NumericSumValue(rowArray)
}

func tryMatrixRowNumericSumDirect(value Matrix, row int) (any, bool, error) {
	switch matrix := value.(type) {
	case matrixArray:
		return numericSumMatrixRowDirect(matrix, row)
	case transposedMatrixArray:
		return numericSumTransposedMatrixRowDirect(matrix, row)
	default:
		return nil, false, nil
	}
}

func numericSumTransposedMatrixRowDirect(matrix transposedMatrixArray, row int) (any, bool, error) {
	if row < 0 || row >= matrix.rows {
		return nil, true, fmt.Errorf("matrix row index %d out of range", row)
	}
	switch source := matrix.source.(type) {
	case matrixArray:
		if len(source.shape) != 2 || row >= source.shape[1] {
			return nil, true, fmt.Errorf("matrix row index %d out of range", row)
		}
		var totalFloat float64
		var totalInt int64
		integer := true
		for sourceRow := 0; sourceRow < source.shape[0]; sourceRow++ {
			value, handled, err := arrayScalarAt(source.data, sourceRow*source.shape[1]+row)
			if err != nil || !handled {
				return nil, handled, err
			}
			n, ok := numeric(value)
			if !ok {
				return nil, false, nil
			}
			totalFloat += n
			if intValue, ok := coerceInt64Exact(value); ok && integer {
				totalInt += intValue
				continue
			}
			integer = false
		}
		if integer {
			return totalInt, true, nil
		}
		return totalFloat, true, nil
	default:
		return nil, false, nil
	}
}

// TryMatrixRowNumericSumCount reduces sum(row)+count(row) without exposing the
// row view to language frontends. It is a small but reusable matrix-row
// primitive for q/Leia/JIT pipeline backends.
func TryMatrixRowNumericSumCount(value Matrix, row int) (any, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	shape := value.Shape()
	if len(shape) != 2 {
		return nil, false, nil
	}
	if row < 0 || row >= shape[0] {
		return nil, true, fmt.Errorf("matrix row index %d out of range", row)
	}
	sum, handled, err := TryMatrixRowNumericSum(value, row)
	if err != nil || !handled {
		return nil, handled, err
	}
	if sumI, ok := coerceInt64Exact(sum); ok {
		return sumI + int64(shape[1]), true, nil
	}
	if sumF, ok := numeric(sum); ok {
		return sumF + float64(shape[1]), true, nil
	}
	return nil, false, nil
}

// TryMatrixRowsNumericSumPlusCount reduces the sum of one or more matrix rows
// plus count(matrix) without materializing row views. It is intentionally
// matrix-oriented so q, Leia, and JIT backends can share the same primitive.
func TryMatrixRowsNumericSumPlusCount(value Matrix, rows ...int) (any, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	shape := value.Shape()
	if len(shape) != 2 {
		return nil, false, nil
	}
	var totalInt int64
	var totalFloat float64
	integer := true
	for _, row := range rows {
		if row < 0 || row >= shape[0] {
			return nil, true, fmt.Errorf("matrix row index %d out of range", row)
		}
		sum, handled, err := TryMatrixRowNumericSum(value, row)
		if err != nil || !handled {
			return nil, handled, err
		}
		if sumI, ok := coerceInt64Exact(sum); ok && integer {
			totalInt += sumI
			totalFloat += float64(sumI)
			continue
		}
		sumF, ok := numeric(sum)
		if !ok {
			return nil, false, nil
		}
		integer = false
		totalFloat += sumF
	}
	if integer {
		return totalInt + int64(shape[0]), true, nil
	}
	return totalFloat + float64(shape[0]), true, nil
}

// TryMatrixNestedSumCellPlusCount computes sum(raze sumMatrix)+cell(cellMatrix)
// +count(countMatrix) through typed matrix views. The three inputs are separate
// on purpose: frontends can fuse expressions involving transpose or alias
// bindings without requiring them to be the same object.
func TryMatrixNestedSumCellPlusCount(sumMatrix Matrix, cellMatrix Matrix, countMatrix Matrix, row, col int) (any, bool, error) {
	if sumMatrix == nil || cellMatrix == nil || countMatrix == nil {
		return nil, false, nil
	}
	sum, handled, err := TryTypedNestedNumericSum(sumMatrix)
	if err != nil || !handled {
		return nil, handled, err
	}
	cell, handled, err := TryMatrixCellIndex(cellMatrix, row, col)
	if err != nil || !handled {
		return nil, handled, err
	}
	countShape := countMatrix.Shape()
	if len(countShape) != 2 {
		return nil, false, nil
	}
	if sumI, ok := coerceInt64Exact(sum); ok {
		if cellI, ok := coerceInt64Exact(cell); ok {
			return sumI + cellI + int64(countShape[0]), true, nil
		}
	}
	sumF, ok := numeric(sum)
	if !ok {
		return nil, false, nil
	}
	cellF, ok := numeric(cell)
	if !ok {
		return nil, false, nil
	}
	return sumF + cellF + float64(countShape[0]), true, nil
}

// TryMatrixCellNumericPlusCount returns matrix[row,col]+count(matrix) without
// materializing a row view. q count on a matrix/list-of-lists is the row count.
func TryMatrixCellNumericPlusCount(value Matrix, row, col int) (any, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	shape := value.Shape()
	if len(shape) != 2 {
		return nil, false, nil
	}
	cell, handled, err := TryMatrixCellIndex(value, row, col)
	if err != nil || !handled {
		return nil, handled, err
	}
	if cellI, ok := coerceInt64Exact(cell); ok {
		return cellI + int64(shape[0]), true, nil
	}
	if cellF, ok := numeric(cell); ok {
		return cellF + float64(shape[0]), true, nil
	}
	return nil, false, nil
}

// TryReshapedMatrixCellNumericPlusCount computes (shape#source)[row,col]+count
// for a two-dimensional reshape without constructing the matrix view.
func TryReshapedMatrixCellNumericPlusCount(shape []int, source Array, row, col int) (any, bool, error) {
	if source == nil {
		return nil, false, nil
	}
	if len(shape) != 2 {
		return nil, false, nil
	}
	rows, cols := shape[0], shape[1]
	if rows < 0 || cols < 0 {
		return nil, true, fmt.Errorf("reshape dimension must be non-negative")
	}
	if row < 0 || col < 0 || row >= rows || col >= cols {
		return nil, true, fmt.Errorf("matrix cell index %d %d out of range", row, col)
	}
	if source.Len() == 0 {
		return nil, true, fmt.Errorf("index %d out of range", row*cols+col)
	}
	cell, handled, err := arrayScalarAt(source, (row*cols+col)%source.Len())
	if err != nil || !handled {
		return nil, handled, err
	}
	if cellI, ok := coerceInt64Exact(cell); ok {
		return cellI + int64(rows), true, nil
	}
	if cellF, ok := numeric(cell); ok {
		return cellF + float64(rows), true, nil
	}
	return nil, false, nil
}

// TryReshapedMatrixRowNumericSumCount reduces sum((shape#source)[row])+count
// for a two-dimensional reshape without constructing the matrix or row views.
func TryReshapedMatrixRowNumericSumCount(shape []int, source Array, row int) (any, bool, error) {
	if source == nil {
		return nil, false, nil
	}
	if len(shape) != 2 {
		return nil, false, nil
	}
	rows, cols := shape[0], shape[1]
	if rows < 0 || cols < 0 {
		return nil, true, fmt.Errorf("reshape dimension must be non-negative")
	}
	if row < 0 || row >= rows {
		return nil, true, fmt.Errorf("matrix row index %d out of range", row)
	}
	if source.Len() == 0 && cols > 0 {
		return nil, true, fmt.Errorf("matrix row index %d out of range", row)
	}
	if source.Len() >= row*cols+cols {
		switch values := source.(type) {
		case i64RangeArray:
			first := values.start + int64(row*cols)*values.step
			total := int64(cols) * (2*first + int64(cols-1)*values.step) / 2
			return total + int64(cols), true, nil
		case f64RangeArray:
			first := values.start + float64(row*cols)*values.step
			total := float64(cols) * (2*first + float64(cols-1)*values.step) / 2
			return total + float64(cols), true, nil
		}
	}
	var totalFloat float64
	var totalInt int64
	integer := true
	for col := 0; col < cols; col++ {
		value, handled, err := arrayScalarAt(source, (row*cols+col)%source.Len())
		if err != nil || !handled {
			return nil, handled, err
		}
		n, ok := numeric(value)
		if !ok {
			return nil, false, nil
		}
		totalFloat += n
		if intValue, ok := coerceInt64Exact(value); ok && integer {
			totalInt += intValue
			continue
		}
		integer = false
	}
	if integer {
		return totalInt + int64(cols), true, nil
	}
	return totalFloat + float64(cols), true, nil
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

// MatrixMultiplyNumericSum computes sum(raze mmu[left;right]) without
// materializing the product matrix.
func MatrixMultiplyNumericSum(left, right Matrix) (float64, bool, error) {
	if left == nil || right == nil {
		return 0, false, fmt.Errorf("matrix multiply expects two matrices")
	}
	leftShape := left.Shape()
	rightShape := right.Shape()
	if len(leftShape) != 2 || len(rightShape) != 2 {
		return 0, false, fmt.Errorf("matrix multiply expects two-dimensional matrices")
	}
	if leftShape[1] != rightShape[0] {
		return 0, false, fmt.Errorf("matrix multiply shape %dx%d cannot conform to %dx%d", leftShape[0], leftShape[1], rightShape[0], rightShape[1])
	}
	if leftMatrix, ok := left.(matrixArray); ok {
		if rightMatrix, ok := right.(matrixArray); ok {
			return matrixMultiplyNumericSumFlat(leftMatrix, rightMatrix, leftShape, rightShape)
		}
	}
	var total float64
	for row := 0; row < leftShape[0]; row++ {
		for col := 0; col < rightShape[1]; col++ {
			var cell float64
			for inner := 0; inner < leftShape[1]; inner++ {
				lv, ok := left.Cell(row, inner)
				if !ok {
					return 0, false, fmt.Errorf("matrix multiply left cell %d,%d out of range", row, inner)
				}
				rv, ok := right.Cell(inner, col)
				if !ok {
					return 0, false, fmt.Errorf("matrix multiply right cell %d,%d out of range", inner, col)
				}
				ln, lok := numeric(lv)
				rn, rok := numeric(rv)
				if !lok || !rok {
					return 0, false, fmt.Errorf("matrix multiply expects numeric cells")
				}
				cell += ln * rn
			}
			total += cell
		}
	}
	return total, true, nil
}

func matrixMultiplyNumericSumFlat(left, right matrixArray, leftShape, rightShape []int) (float64, bool, error) {
	leftCols := leftShape[1]
	rightCols := rightShape[1]
	var total float64
	for row := 0; row < leftShape[0]; row++ {
		leftRowOffset := row * leftCols
		for col := 0; col < rightCols; col++ {
			var cell float64
			for inner := 0; inner < leftCols; inner++ {
				leftValue, ok, err := typedKernels.NumericAt(left.data, leftRowOffset+inner)
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("matrix multiply expects numeric cells")
					}
					return 0, ok, err
				}
				rightValue, ok, err := typedKernels.NumericAt(right.data, inner*rightCols+col)
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("matrix multiply expects numeric cells")
					}
					return 0, ok, err
				}
				cell += leftValue * rightValue
			}
			total += cell
		}
	}
	return total, true, nil
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

// MatrixInverseNumericSum computes sum(raze inv matrix). It keeps the
// elimination work buffer but avoids allocating the final matrix value.
func MatrixInverseNumericSum(matrix Matrix) (float64, bool, error) {
	if matrix == nil {
		return 0, false, fmt.Errorf("matrix inverse expects a matrix")
	}
	shape := matrix.Shape()
	if len(shape) != 2 {
		return 0, false, fmt.Errorf("matrix inverse expects a two-dimensional matrix")
	}
	n := shape[0]
	if n != shape[1] {
		return 0, false, fmt.Errorf("matrix inverse expects a square matrix, got %dx%d", shape[0], shape[1])
	}
	if n == 0 {
		return 0, true, nil
	}
	aug := make([]float64, n*2*n)
	for row := 0; row < n; row++ {
		for col := 0; col < n; col++ {
			value, ok := matrix.Cell(row, col)
			if !ok {
				return 0, false, fmt.Errorf("matrix inverse cell %d,%d out of range", row, col)
			}
			num, ok := numeric(value)
			if !ok {
				return 0, false, fmt.Errorf("matrix inverse expects numeric cells")
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
			return 0, false, fmt.Errorf("matrix inverse expects a non-singular matrix")
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
	var total float64
	for row := 0; row < n; row++ {
		for col := 0; col < n; col++ {
			total += aug[row*2*n+n+col]
		}
	}
	return total, true, nil
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
	out := make([]int, len(indexes))
	base := r.row * r.matrix.shape[1]
	for i, col := range indexes {
		if col < 0 || col >= r.Len() {
			panic(fmt.Sprintf("data matrix row gather col %d out of range", col))
		}
		out[i] = base + col
	}
	return r.matrix.data.Gather(out)
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
	sourceShape := r.matrix.source.Shape()
	if source, ok := r.matrix.source.(matrixArray); ok && len(sourceShape) == 2 {
		out := make([]int, len(indexes))
		for i, col := range indexes {
			if col < 0 || col >= r.Len() {
				panic(fmt.Sprintf("data transpose row gather col %d out of range", col))
			}
			out[i] = col*sourceShape[1] + r.row
		}
		return source.data.Gather(out)
	}
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

func (v ndArrayView) Kind() Kind {
	if len(v.shape) == 1 {
		return v.data.Kind()
	}
	return KindAny
}

func (v ndArrayView) Len() int {
	if len(v.shape) == 0 {
		return 0
	}
	return v.shape[0]
}

func (v ndArrayView) At(row int) (any, bool) {
	if row < 0 || row >= v.Len() {
		return nil, false
	}
	if len(v.shape) == 1 {
		value, handled, err := arrayScalarAt(v.data, v.offset+row)
		return value, handled && err == nil
	}
	stride := reshapeStride(v.shape[1:])
	nextShape := append([]int(nil), v.shape[1:]...)
	if len(nextShape) == 2 {
		return ndMatrixView{
			shape:  nextShape,
			data:   v.data,
			offset: v.offset + row*stride,
		}, true
	}
	return ndArrayView{
		shape:  nextShape,
		data:   v.data,
		offset: v.offset + row*stride,
	}, true
}

func (v ndArrayView) Values() []any {
	out := make([]any, v.Len())
	for row := range out {
		value, ok := v.At(row)
		if !ok {
			panic(fmt.Sprintf("data nd reshape row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (v ndArrayView) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		value, ok := v.At(row)
		if !ok {
			panic(fmt.Sprintf("data nd reshape gather row %d out of range", row))
		}
		out[i] = value
	}
	return NewAny(out)
}

func (v ndMatrixView) Kind() Kind { return KindAny }

func (v ndMatrixView) Len() int { return v.shape[0] }

func (v ndMatrixView) At(row int) (any, bool) {
	return v.RowArray(row)
}

func (v ndMatrixView) Values() []any {
	out := make([]any, v.Len())
	for row := range out {
		value, ok := v.At(row)
		if !ok {
			panic(fmt.Sprintf("data nd matrix row %d out of range", row))
		}
		out[row] = value
	}
	return out
}

func (v ndMatrixView) Gather(indexes []int) Array {
	out := make([]any, len(indexes))
	for i, row := range indexes {
		value, ok := v.At(row)
		if !ok {
			panic(fmt.Sprintf("data nd matrix gather row %d out of range", row))
		}
		out[i] = value
	}
	return NewAny(out)
}

func (v ndMatrixView) Shape() []int {
	return append([]int(nil), v.shape...)
}

func (v ndMatrixView) Cell(row, col int) (any, bool) {
	if len(v.shape) != 2 || row < 0 || col < 0 || row >= v.shape[0] || col >= v.shape[1] {
		return nil, false
	}
	value, handled, err := arrayScalarAt(v.data, v.offset+row*v.shape[1]+col)
	return value, handled && err == nil
}

func (v ndMatrixView) RowArray(row int) (Array, bool) {
	if len(v.shape) != 2 || row < 0 || row >= v.shape[0] {
		return nil, false
	}
	return ndArrayView{
		shape:  []int{v.shape[1]},
		data:   v.data,
		offset: v.offset + row*v.shape[1],
	}, true
}

func reshapeStride(shape []int) int {
	stride := 1
	for _, dim := range shape {
		stride *= dim
	}
	return stride
}
