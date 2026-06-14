package bind

import (
	"fmt"

	stddata "github.com/never-labs/leia/internal/stdlib/lib/data"
)

// BuildLinalg creates the "linalg" standard library table.
func BuildLinalg() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "linalg." + name, Fn: fn}))
	}

	set("vector", linalgVector)
	set("matrix", linalgMatrix)
	set("zeros", linalgZeros)
	set("eye", linalgEye)
	set("diag", linalgDiag)
	set("get", linalgGet)
	set("set", linalgSet)
	set("add", linalgAdd)
	set("sub", linalgSub)
	set("scale", linalgScale)
	set("dot", linalgDot)
	set("matvec", linalgMatvec)
	set("matmul", linalgMatmul)
	set("transpose", linalgTranspose)
	set("norm", linalgNorm)
	set("solve2", linalgSolve2)
	return t
}

func linalgVector(args []Value) ([]Value, error) {
	values, err := linalgVectorArgs("linalg.vector", args)
	if err != nil {
		return nil, err
	}
	stddata.RecordLinalgVectorKernel("LinalgVectorConstruct", "construct", len(values))
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(values))}, nil
}

func linalgMatrix(args []Value) ([]Value, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("linalg.matrix: need rows, cols, values")
	}
	rows, cols, err := linalgShape("linalg.matrix", args[0], args[1])
	if err != nil {
		return nil, err
	}
	values, err := linalgNumericTable("linalg.matrix", args[2])
	if err != nil {
		return nil, err
	}
	if len(values) != rows*cols {
		return nil, fmt.Errorf("linalg.matrix: values length %d does not match %dx%d", len(values), rows, cols)
	}
	stddata.RecordLinalgMatrixKernel("LinalgMatrixConstruct", "construct", rows, cols)
	return []Value{linalgMatrixDenseValue(rows, cols, values)}, nil
}

func linalgZeros(args []Value) ([]Value, error) {
	switch len(args) {
	case 1:
		n, err := linalgPositiveInt("linalg.zeros", args[0], "size")
		if err != nil {
			return nil, err
		}
		stddata.RecordLinalgVectorKernel("LinalgVectorZeros", "zeros", n)
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(make([]float64, n)))}, nil
	default:
		if len(args) < 2 {
			return nil, fmt.Errorf("linalg.zeros: need size or rows, cols")
		}
		rows, cols, err := linalgShape("linalg.zeros", args[0], args[1])
		if err != nil {
			return nil, err
		}
		stddata.RecordLinalgMatrixKernel("LinalgMatrixZeros", "zeros", rows, cols)
		return []Value{linalgMatrixDenseValue(rows, cols, make([]float64, rows*cols))}, nil
	}
}

func linalgEye(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("linalg.eye: need size")
	}
	n, err := linalgPositiveInt("linalg.eye", args[0], "size")
	if err != nil {
		return nil, err
	}
	values := make([]float64, n*n)
	for i := 0; i < n; i++ {
		values[i*n+i] = 1
	}
	stddata.RecordLinalgMatrixKernel("LinalgMatrixEye", "eye", n, n)
	return []Value{linalgMatrixDenseValue(n, n, values)}, nil
}

func linalgDiag(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("linalg.diag: need vector")
	}
	values, err := linalgVectorValue("linalg.diag", args[0])
	if err != nil {
		return nil, err
	}
	n := len(values)
	out := make([]float64, n*n)
	for i, v := range values {
		out[i*n+i] = v
	}
	stddata.RecordLinalgMatrixKernel("LinalgMatrixDiag", "diag", n, n)
	return []Value{linalgMatrixDenseValue(n, n, out)}, nil
}

func linalgGet(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.get: need value and index")
	}
	if rows, cols, values, ok, err := linalgMatrixValue("linalg.get", args[0]); err != nil {
		return nil, err
	} else if ok {
		if len(args) < 3 {
			return nil, fmt.Errorf("linalg.get: matrix get needs row and col")
		}
		r, c, err := linalgIndex2("linalg.get", args[1], args[2], rows, cols)
		if err != nil {
			return nil, err
		}
		return []Value{FloatValue(values[r*cols+c])}, nil
	}
	values, err := linalgVectorValue("linalg.get", args[0])
	if err != nil {
		return nil, err
	}
	i, err := linalgIndex("linalg.get", args[1], len(values))
	if err != nil {
		return nil, err
	}
	return []Value{FloatValue(values[i])}, nil
}

func linalgSet(args []Value) ([]Value, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("linalg.set: need value, index, new value")
	}
	if rows, cols, _, ok, err := linalgMatrixValue("linalg.set", args[0]); err != nil {
		return nil, err
	} else if ok {
		if len(args) < 4 {
			return nil, fmt.Errorf("linalg.set: matrix set needs row, col, value")
		}
		r, c, err := linalgIndex2("linalg.set", args[1], args[2], rows, cols)
		if err != nil {
			return nil, err
		}
		value, err := linalgNumber("linalg.set", args[3])
		if err != nil {
			return nil, err
		}
		if err := linalgSetMatrixValue(args[0], r, c, rows, cols, value); err != nil {
			return nil, err
		}
		return []Value{args[0]}, nil
	}
	values, err := linalgVectorValue("linalg.set", args[0])
	if err != nil {
		return nil, err
	}
	i, err := linalgIndex("linalg.set", args[1], len(values))
	if err != nil {
		return nil, err
	}
	value, err := linalgNumber("linalg.set", args[2])
	if err != nil {
		return nil, err
	}
	if args[0].IsDenseArray() {
		if err := args[0].DenseArray().Set(i, FloatValue(value)); err != nil {
			return nil, err
		}
	} else {
		args[0].Table().RawSetInt(int64(i+1), FloatValue(value))
	}
	return []Value{args[0]}, nil
}

func linalgAdd(args []Value) ([]Value, error) {
	return linalgBinary(args, "linalg.add", func(a, b float64) float64 { return a + b })
}
func linalgSub(args []Value) ([]Value, error) {
	return linalgBinary(args, "linalg.sub", func(a, b float64) float64 { return a - b })
}

func linalgScale(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.scale: need value and scalar")
	}
	s, err := linalgNumber("linalg.scale", args[1])
	if err != nil {
		return nil, err
	}
	if rows, cols, values, ok, err := linalgMatrixValue("linalg.scale", args[0]); err != nil {
		return nil, err
	} else if ok {
		return []Value{linalgMatrixDenseValue(rows, cols, stddata.LinalgF64MatrixScale(rows, cols, values, s))}, nil
	}
	values, err := linalgVectorValue("linalg.scale", args[0])
	if err != nil {
		return nil, err
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(stddata.LinalgF64VectorScale(values, s)))}, nil
}

func linalgDot(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.dot: need two vectors")
	}
	a, err := linalgVectorValue("linalg.dot", args[0])
	if err != nil {
		return nil, err
	}
	b, err := linalgVectorValue("linalg.dot", args[1])
	if err != nil {
		return nil, err
	}
	if len(a) != len(b) {
		return nil, fmt.Errorf("linalg.dot: vector length mismatch")
	}
	return []Value{FloatValue(stddata.LinalgF64VectorDot(a, b))}, nil
}

func linalgMatvec(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.matvec: need matrix and vector")
	}
	rows, cols, m, ok, err := linalgMatrixValue("linalg.matvec", args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.matvec: argument 1 must be a matrix")
	}
	v, err := linalgVectorValue("linalg.matvec", args[1])
	if err != nil {
		return nil, err
	}
	if len(v) != cols {
		return nil, fmt.Errorf("linalg.matvec: dimension mismatch")
	}
	out := stddata.LinalgF64Matvec(rows, cols, m, v)
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func linalgMatmul(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.matmul: need two matrices")
	}
	ar, ac, a, ok, err := linalgMatrixValue("linalg.matmul", args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.matmul: argument 1 must be a matrix")
	}
	br, bc, b, ok, err := linalgMatrixValue("linalg.matmul", args[1])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.matmul: argument 2 must be a matrix")
	}
	if ac != br {
		return nil, fmt.Errorf("linalg.matmul: dimension mismatch")
	}
	out := stddata.LinalgF64Matmul(ar, ac, bc, a, b)
	return []Value{linalgMatrixDenseValue(ar, bc, out)}, nil
}

func linalgTranspose(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("linalg.transpose: need matrix")
	}
	rows, cols, values, ok, err := linalgMatrixValue("linalg.transpose", args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.transpose: argument 1 must be a matrix")
	}
	out := stddata.LinalgF64Transpose(rows, cols, values)
	return []Value{linalgMatrixDenseValue(cols, rows, out)}, nil
}

func linalgNorm(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("linalg.norm: need vector")
	}
	values, err := linalgVectorValue("linalg.norm", args[0])
	if err != nil {
		return nil, err
	}
	return []Value{FloatValue(stddata.LinalgF64VectorNorm(values))}, nil
}

func linalgSolve2(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.solve2: need matrix and vector")
	}
	rows, cols, a, ok, err := linalgMatrixValue("linalg.solve2", args[0])
	if err != nil {
		return nil, err
	}
	if !ok || rows != 2 || cols != 2 {
		return nil, fmt.Errorf("linalg.solve2: argument 1 must be a 2x2 matrix")
	}
	b, err := linalgVectorValue("linalg.solve2", args[1])
	if err != nil {
		return nil, err
	}
	if len(b) != 2 {
		return nil, fmt.Errorf("linalg.solve2: argument 2 must be a length-2 vector")
	}
	det := a[0]*a[3] - a[1]*a[2]
	if det == 0 {
		return nil, fmt.Errorf("linalg.solve2: singular matrix")
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(stddata.LinalgF64Solve2(a, b)))}, nil
}

func linalgBinary(args []Value, name string, op func(float64, float64) float64) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("%s: need two values", name)
	}
	if ar, ac, a, ok, err := linalgMatrixValue(name, args[0]); err != nil {
		return nil, err
	} else if ok {
		br, bc, b, ok, err := linalgMatrixValue(name, args[1])
		if err != nil {
			return nil, err
		}
		if !ok || ar != br || ac != bc {
			return nil, fmt.Errorf("%s: matrix shape mismatch", name)
		}
		out := stddata.LinalgF64BinaryMatrix("LinalgMatrixBinary", name, ar, ac, a, b, op)
		return []Value{linalgMatrixDenseValue(ar, ac, out)}, nil
	}
	a, err := linalgVectorValue(name, args[0])
	if err != nil {
		return nil, err
	}
	b, err := linalgVectorValue(name, args[1])
	if err != nil {
		return nil, err
	}
	if len(a) != len(b) {
		return nil, fmt.Errorf("%s: vector length mismatch", name)
	}
	out := stddata.LinalgF64BinaryVector("LinalgVectorBinary", name, a, b, op)
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func linalgVectorArgs(name string, args []Value) ([]float64, error) {
	if len(args) == 1 && (args[0].IsTable() || args[0].IsDenseArray()) {
		return linalgVectorValue(name, args[0])
	}
	out := make([]float64, len(args))
	for i, arg := range args {
		v, err := linalgNumber(name, arg)
		if err != nil {
			return nil, fmt.Errorf("%s argument %d: %w", name, i+1, err)
		}
		out[i] = v
	}
	return out, nil
}

func linalgVectorValue(name string, value Value) ([]float64, error) {
	if value.IsDenseArray() {
		arr := value.DenseArray()
		if xs, ok := arr.F64(); ok {
			return append([]float64(nil), xs...), nil
		}
		if xs, ok := arr.I64(); ok {
			out := make([]float64, len(xs))
			for i, x := range xs {
				out[i] = float64(x)
			}
			return out, nil
		}
		return nil, fmt.Errorf("%s: expected numeric dense array, got %s", name, arr.DType())
	}
	if !value.IsTable() {
		return nil, fmt.Errorf("%s: expected vector table or dense array, got %s", name, value.TypeName())
	}
	return linalgNumericTable(name, value)
}

func linalgMatrixValue(name string, value Value) (int, int, []float64, bool, error) {
	if !value.IsTable() {
		return 0, 0, nil, false, nil
	}
	t := value.Table()
	if backing, rows, stride, ok := t.DenseMatrixBacking(); ok {
		data := make([]float64, rows*stride)
		copy(data, backing[:rows*stride])
		return rows, stride, data, true, nil
	}
	rowsValue := t.RawGetString("rows")
	colsValue := t.RawGetString("cols")
	valuesValue := t.RawGetString("values")
	if rowsValue.IsNil() && colsValue.IsNil() && valuesValue.IsNil() {
		return 0, 0, nil, false, nil
	}
	rows, cols, err := linalgShape(name, rowsValue, colsValue)
	if err != nil {
		return 0, 0, nil, true, err
	}
	values, err := linalgNumericTable(name, valuesValue)
	if err != nil {
		return 0, 0, nil, true, err
	}
	if len(values) != rows*cols {
		return 0, 0, nil, true, fmt.Errorf("%s: matrix values length mismatch", name)
	}
	return rows, cols, values, true, nil
}

func linalgNumericTable(name string, value Value) ([]float64, error) {
	if !value.IsTable() {
		return nil, fmt.Errorf("%s: expected table, got %s", name, value.TypeName())
	}
	t := value.Table()
	out := make([]float64, t.Length())
	for i := 0; i < len(out); i++ {
		v, err := linalgNumber(name, t.RawGetInt(int64(i+1)))
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i+1, err)
		}
		out[i] = v
	}
	return out, nil
}

func linalgVectorTable(values []float64) *Table {
	t := NewAppendArrayTable(len(values) + 1)
	for i, v := range values {
		t.RawSetInt(int64(i+1), FloatValue(v))
	}
	t.RawSetString("_type", StringValue("linalg.vector"))
	return t
}

func linalgMatrixTable(rows, cols int, values []float64) *Table {
	t := NewTable()
	t.RawSetString("_type", StringValue("linalg.matrix"))
	t.RawSetString("rows", IntValue(int64(rows)))
	t.RawSetString("cols", IntValue(int64(cols)))
	t.RawSetString("values", TableValue(linalgVectorTable(values)))
	return t
}

func linalgMatrixDenseValue(rows, cols int, values []float64) Value {
	m := NewDenseMatrix(rows, cols)
	backing, _, stride, ok := m.DenseMatrixBacking()
	if ok {
		for r := 0; r < rows; r++ {
			copy(backing[r*stride:r*stride+cols], values[r*cols:r*cols+cols])
		}
	}
	return TableValue(m)
}

func linalgSetMatrixValue(value Value, row, col, rows, cols int, x float64) error {
	if value.IsTable() {
		t := value.Table()
		if backing, denseRows, stride, ok := t.DenseMatrixBacking(); ok {
			if denseRows != rows || stride != cols {
				return fmt.Errorf("linalg.set: dense matrix shape changed")
			}
			backing[row*stride+col] = x
			return nil
		}
		values := t.RawGetString("values")
		if values.IsTable() {
			values.Table().RawSetInt(int64(row*cols+col+1), FloatValue(x))
			return nil
		}
	}
	return fmt.Errorf("linalg.set: argument 1 must be a mutable matrix")
}

func linalgShape(name string, rowsValue, colsValue Value) (int, int, error) {
	rows, err := linalgPositiveInt(name, rowsValue, "rows")
	if err != nil {
		return 0, 0, err
	}
	cols, err := linalgPositiveInt(name, colsValue, "cols")
	if err != nil {
		return 0, 0, err
	}
	return rows, cols, nil
}

func linalgPositiveInt(name string, value Value, label string) (int, error) {
	if !value.IsInt() {
		return 0, fmt.Errorf("%s: %s must be an integer", name, label)
	}
	n := value.Int()
	if n < 0 {
		return 0, fmt.Errorf("%s: %s must be non-negative", name, label)
	}
	return int(n), nil
}

func linalgIndex(name string, value Value, length int) (int, error) {
	if !value.IsInt() {
		return 0, fmt.Errorf("%s: index must be an integer", name)
	}
	i := int(value.Int())
	if i < 1 || i > length {
		return 0, fmt.Errorf("%s: index out of range", name)
	}
	return i - 1, nil
}

func linalgIndex2(name string, rowValue, colValue Value, rows, cols int) (int, int, error) {
	r, err := linalgIndex(name, rowValue, rows)
	if err != nil {
		return 0, 0, err
	}
	c, err := linalgIndex(name, colValue, cols)
	if err != nil {
		return 0, 0, err
	}
	return r, c, nil
}

func linalgNumber(name string, value Value) (float64, error) {
	if !value.IsNumber() {
		return 0, fmt.Errorf("%s: number expected, got %s", name, value.TypeName())
	}
	return toFloat(value), nil
}
