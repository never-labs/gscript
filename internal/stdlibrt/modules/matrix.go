package modules

import (
	"fmt"

	stdmatrix "github.com/never-labs/leia/internal/stdlib/matrix"
)

// BuildMatrix creates the "matrix" standard library table.
func BuildMatrix() *Table {
	t := NewTable()
	set := func(name string, gf *GoFunction) {
		gf.Name = "matrix." + name
		t.RawSetString(name, FunctionValue(gf))
	}

	set("dense", &GoFunction{
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("matrix.dense: need 2 arguments (rows, cols)")
			}
			v, err := matrixDenseValue(args[0], args[1])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg2: matrixDenseValue,
	})

	set("getf", &GoFunction{
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 3 {
				return nil, fmt.Errorf("matrix.getf: need 3 arguments (m, i, j)")
			}
			v, err := matrixGetfValue(args[0], args[1], args[2])
			if err != nil {
				return nil, err
			}
			return []Value{v}, nil
		},
		FastArg3: matrixGetfValue,
	})

	set("setf", &GoFunction{
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 4 {
				return nil, fmt.Errorf("matrix.setf: need 4 arguments (m, i, j, v)")
			}
			if _, err := matrixSetfValue(args[0], args[1], args[2], args[3]); err != nil {
				return nil, err
			}
			return nil, nil
		},
		FastArg4: matrixSetfValue,
	})

	return t
}

func matrixDenseValue(rowsValue, colsValue Value) (Value, error) {
	if !rowsValue.IsInt() || !colsValue.IsInt() {
		return NilValue(), fmt.Errorf("matrix.dense: rows and cols must be integers")
	}
	rows, cols, err := stdmatrix.Shape(rowsValue.Int(), colsValue.Int())
	if err != nil {
		return NilValue(), err
	}
	return TableValue(NewDenseMatrix(rows, cols)), nil
}

func matrixGetfValue(matrixValue, rowValue, colValue Value) (Value, error) {
	backing, offset, err := matrixDenseAccess(matrixValue, rowValue, colValue, "matrix.getf")
	if err != nil {
		return NilValue(), err
	}
	return FloatValue(backing[offset]), nil
}

func matrixSetfValue(matrixValue, rowValue, colValue, value Value) (Value, error) {
	backing, offset, err := matrixDenseAccess(matrixValue, rowValue, colValue, "matrix.setf")
	if err != nil {
		return NilValue(), err
	}
	var f float64
	switch {
	case value.IsFloat():
		f = value.Float()
	case value.IsInt():
		f = float64(value.Int())
	default:
		return NilValue(), fmt.Errorf("matrix.setf: value must be numeric")
	}
	backing[offset] = f
	return NilValue(), nil
}

func matrixDenseAccess(matrixValue, rowValue, colValue Value, name string) ([]float64, int, error) {
	if !matrixValue.IsTable() {
		return nil, 0, fmt.Errorf("%s: argument 1 must be a matrix", name)
	}
	m := matrixValue.Table()
	backing, rows, stride, ok := m.DenseMatrixBacking()
	if !ok {
		return nil, 0, fmt.Errorf("%s: argument 1 is not a DenseMatrix", name)
	}
	if !rowValue.IsInt() || !colValue.IsInt() {
		return nil, 0, fmt.Errorf("%s: row and column must be integers", name)
	}
	offset, err := stdmatrix.Offset(name, rowValue.Int(), colValue.Int(), rows, stride)
	if err != nil {
		return nil, 0, err
	}
	return backing, offset, nil
}
