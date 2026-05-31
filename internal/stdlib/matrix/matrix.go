package matrix

import "fmt"

// Shape validates matrix.dense dimensions and returns host-sized ints for the
// runtime DenseMatrix allocator.
func Shape(rows, cols int64) (int, int, error) {
	if rows < 0 || cols < 0 {
		return 0, 0, fmt.Errorf("matrix.dense: rows and cols must be non-negative")
	}
	return int(rows), int(cols), nil
}

// Offset validates a dense matrix row/column pair and returns the flattened
// backing offset. rowCount is the logical outer table length; stride is the
// dense row width.
func Offset(name string, row, col int64, rowCount, stride int) (int, error) {
	i := int(row)
	j := int(col)
	if i < 0 || i >= rowCount || j < 0 || j >= stride {
		return 0, fmt.Errorf("%s: index out of range", name)
	}
	return i*stride + j, nil
}
