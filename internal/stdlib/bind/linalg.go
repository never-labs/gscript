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
	set("row", linalgRow)
	set("col", linalgCol)
	set("matrix", linalgMatrix)
	set("zeros", linalgZeros)
	set("eye", linalgEye)
	set("diag", linalgDiag)
	set("at", linalgAt)
	set("get", linalgGet)
	set("set", linalgSet)
	set("add", linalgAdd)
	set("sub", linalgSub)
	set("mul", linalgMul)
	set("div", linalgDiv)
	set("scale", linalgScale)
	set("affine", linalgAffine)
	set("dot", linalgDot)
	set("matvec", linalgMatvec)
	set("matmul", linalgMatmul)
	set("chainmul", linalgChainmul)
	set("sandwich", linalgSandwich)
	set("transpose", linalgTranspose)
	set("trace", linalgTrace)
	set("scalar", linalgScalar)
	set("norm", linalgNorm)
	set("solve", linalgSolve)
	set("solve_right", linalgSolveRight)
	set("rsolve", linalgSolveRight)
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

func linalgRow(args []Value) ([]Value, error) {
	values, err := linalgVectorArgs("linalg.row", args)
	if err != nil {
		return nil, err
	}
	stddata.RecordLinalgMatrixKernel("LinalgMatrixRow", "row", 1, len(values))
	return []Value{linalgMatrixDenseValue(1, len(values), values)}, nil
}

func linalgCol(args []Value) ([]Value, error) {
	values, err := linalgVectorArgs("linalg.col", args)
	if err != nil {
		return nil, err
	}
	stddata.RecordLinalgMatrixKernel("LinalgMatrixCol", "col", len(values), 1)
	return []Value{linalgMatrixDenseValue(len(values), 1, values)}, nil
}

func linalgMatrix(args []Value) ([]Value, error) {
	if len(args) == 1 {
		rows, cols, values, err := linalgNestedMatrix("linalg.matrix", args[0])
		if err != nil {
			return nil, err
		}
		stddata.RecordLinalgMatrixKernel("LinalgMatrixConstruct", "construct", rows, cols)
		return []Value{linalgMatrixDenseValue(rows, cols, values)}, nil
	}
	if len(args) < 3 {
		return nil, fmt.Errorf("linalg.matrix: need nested rows or rows, cols, values")
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

func linalgAt(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.at: need value and index")
	}
	if rows, cols, values, ok, err := linalgMatrixValue("linalg.at", args[0]); err != nil {
		return nil, err
	} else if ok {
		if len(args) >= 3 {
			r, c, err := linalgIndex2("linalg.at", args[1], args[2], rows, cols)
			if err != nil {
				return nil, err
			}
			return []Value{FloatValue(values[r*cols+c])}, nil
		}
		if cols == 1 {
			i, err := linalgIndex("linalg.at", args[1], rows)
			if err != nil {
				return nil, err
			}
			return []Value{FloatValue(values[i*cols])}, nil
		}
		if rows == 1 {
			i, err := linalgIndex("linalg.at", args[1], cols)
			if err != nil {
				return nil, err
			}
			return []Value{FloatValue(values[i])}, nil
		}
		return nil, fmt.Errorf("linalg.at: matrix access needs row and col unless matrix is a row or column vector")
	}
	values, err := linalgVectorValue("linalg.at", args[0])
	if err != nil {
		return nil, err
	}
	i, err := linalgIndex("linalg.at", args[1], len(values))
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
	return linalgPointwise(args, "linalg.add", "add", func(a, b float64) float64 { return a + b })
}
func linalgSub(args []Value) ([]Value, error) {
	return linalgPointwise(args, "linalg.sub", "sub", func(a, b float64) float64 { return a - b })
}
func linalgMul(args []Value) ([]Value, error) {
	return linalgPointwise(args, "linalg.mul", "mul", func(a, b float64) float64 { return a * b })
}
func linalgDiv(args []Value) ([]Value, error) {
	return linalgPointwise(args, "linalg.div", "div", func(a, b float64) float64 { return a / b })
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

func linalgAffine(args []Value) ([]Value, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("linalg.affine: need base, delta, baseScale, deltaScale")
	}
	base, err := linalgPointwiseDecode("linalg.affine", args[0])
	if err != nil {
		return nil, err
	}
	delta, err := linalgPointwiseDecode("linalg.affine", args[1])
	if err != nil {
		return nil, err
	}
	baseScale, err := linalgNumber("linalg.affine", args[2])
	if err != nil {
		return nil, err
	}
	deltaScale, err := linalgNumber("linalg.affine", args[3])
	if err != nil {
		return nil, err
	}
	return linalgAffineOperands(base, delta, baseScale, deltaScale)
}

func linalgAffineOperands(base, delta linalgPointwiseOperand, baseScale, deltaScale float64) ([]Value, error) {
	switch {
	case base.kind == "matrix" || delta.kind == "matrix":
		if base.kind == "vector" || delta.kind == "vector" {
			return nil, fmt.Errorf("linalg.affine: cannot mix matrix and vector operands")
		}
		rows, cols := base.rows, base.cols
		if delta.kind == "matrix" {
			if base.kind == "matrix" && (base.rows != delta.rows || base.cols != delta.cols) {
				return nil, fmt.Errorf("linalg.affine: matrix shape mismatch")
			}
			rows, cols = delta.rows, delta.cols
		}
		out := make([]float64, rows*cols)
		for i := range out {
			out[i] = baseScale*base.valueAt(i) + deltaScale*delta.valueAt(i)
		}
		stddata.RecordLinalgMatrixKernel("LinalgMatrixAffine", "affine", rows, cols)
		return []Value{linalgMatrixDenseValue(rows, cols, out)}, nil
	case base.kind == "vector" || delta.kind == "vector":
		length := len(base.data)
		if delta.kind == "vector" {
			if base.kind == "vector" && len(base.data) != len(delta.data) {
				return nil, fmt.Errorf("linalg.affine: vector length mismatch")
			}
			length = len(delta.data)
		}
		out := make([]float64, length)
		for i := range out {
			out[i] = baseScale*base.valueAt(i) + deltaScale*delta.valueAt(i)
		}
		stddata.RecordLinalgVectorKernel("LinalgVectorAffine", "affine", length)
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
	default:
		return []Value{FloatValue(baseScale*base.scalar + deltaScale*delta.scalar)}, nil
	}
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

func linalgChainmul(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.chainmul: need at least two matrices")
	}
	rows, cols, values, ok, err := linalgMatrixValue("linalg.chainmul", args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.chainmul: argument 1 must be a matrix")
	}
	for i := 1; i < len(args); i++ {
		br, bc, b, ok, err := linalgMatrixValue("linalg.chainmul", args[i])
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("linalg.chainmul: argument %d must be a matrix", i+1)
		}
		if cols != br {
			return nil, fmt.Errorf("linalg.chainmul: dimension mismatch at argument %d", i+1)
		}
		values = stddata.LinalgF64Matmul(rows, cols, bc, values, b)
		cols = bc
	}
	return []Value{linalgMatrixDenseValue(rows, cols, values)}, nil
}

func linalgSandwich(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.sandwich: need transform and matrix")
	}
	ar, ac, a, ok, err := linalgMatrixValue("linalg.sandwich", args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.sandwich: argument 1 must be a matrix")
	}
	pr, pc, p, ok, err := linalgMatrixValue("linalg.sandwich", args[1])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.sandwich: argument 2 must be a matrix")
	}
	if ac != pr || pc != ac {
		return nil, fmt.Errorf("linalg.sandwich: dimension mismatch")
	}
	ap := stddata.LinalgF64Matmul(ar, ac, pc, a, p)
	at := stddata.LinalgF64Transpose(ar, ac, a)
	out := stddata.LinalgF64Matmul(ar, pc, ar, ap, at)
	return []Value{linalgMatrixDenseValue(ar, ar, out)}, nil
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

func linalgTrace(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("linalg.trace: need matrix")
	}
	rows, cols, values, ok, err := linalgMatrixValue("linalg.trace", args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.trace: argument 1 must be a matrix")
	}
	n := rows
	if cols < n {
		n = cols
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += values[i*cols+i]
	}
	stddata.RecordLinalgMatrixKernel("LinalgMatrixTrace", "trace", rows, cols)
	return []Value{FloatValue(sum)}, nil
}

func linalgScalar(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("linalg.scalar: need value")
	}
	if args[0].IsNumber() {
		return []Value{FloatValue(args[0].Number())}, nil
	}
	if rows, cols, values, ok, err := linalgMatrixValue("linalg.scalar", args[0]); err != nil {
		return nil, err
	} else if ok {
		if rows != 1 || cols != 1 {
			return nil, fmt.Errorf("linalg.scalar: matrix must be 1x1")
		}
		return []Value{FloatValue(values[0])}, nil
	}
	values, err := linalgVectorValue("linalg.scalar", args[0])
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("linalg.scalar: vector length must be 1")
	}
	return []Value{FloatValue(values[0])}, nil
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

func linalgSolve(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.solve: need matrix and right-hand side")
	}
	n, cols, a, ok, err := linalgMatrixValue("linalg.solve", args[0])
	if err != nil {
		return nil, err
	}
	if !ok || n != cols {
		return nil, fmt.Errorf("linalg.solve: argument 1 must be a square matrix")
	}
	if br, bc, b, ok, err := linalgMatrixValue("linalg.solve", args[1]); err != nil {
		return nil, err
	} else if ok {
		if br != n {
			return nil, fmt.Errorf("linalg.solve: right-hand side row mismatch")
		}
		out, err := linalgSolveDense(n, bc, a, b)
		if err != nil {
			return nil, err
		}
		stddata.RecordLinalgMatrixKernel("LinalgMatrixSolve", "solve", n, bc)
		return []Value{linalgMatrixDenseValue(n, bc, out)}, nil
	}
	b, err := linalgVectorValue("linalg.solve", args[1])
	if err != nil {
		return nil, err
	}
	if len(b) != n {
		return nil, fmt.Errorf("linalg.solve: right-hand side length mismatch")
	}
	out, err := linalgSolveDense(n, 1, a, b)
	if err != nil {
		return nil, err
	}
	stddata.RecordLinalgVectorKernel("LinalgMatrixSolve", "solve", n)
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func linalgSolveRight(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("linalg.solve_right: need right-hand side and matrix")
	}
	br, bc, b, ok, err := linalgMatrixValue("linalg.solve_right", args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("linalg.solve_right: argument 1 must be a matrix")
	}
	ar, ac, a, ok, err := linalgMatrixValue("linalg.solve_right", args[1])
	if err != nil {
		return nil, err
	}
	if !ok || ar != ac {
		return nil, fmt.Errorf("linalg.solve_right: argument 2 must be a square matrix")
	}
	if bc != ar {
		return nil, fmt.Errorf("linalg.solve_right: dimension mismatch")
	}
	at := stddata.LinalgF64Transpose(ar, ac, a)
	bt := stddata.LinalgF64Transpose(br, bc, b)
	xt, err := linalgSolveDense(ar, br, at, bt)
	if err != nil {
		return nil, err
	}
	out := stddata.LinalgF64Transpose(ar, br, xt)
	stddata.RecordLinalgMatrixKernel("LinalgMatrixSolveRight", "solve_right", br, bc)
	return []Value{linalgMatrixDenseValue(br, bc, out)}, nil
}

type linalgPointwiseOperand struct {
	kind       string
	rows, cols int
	data       []float64
	scalar     float64
}

func linalgPointwise(args []Value, name, opName string, op func(float64, float64) float64) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("%s: need two values", name)
	}
	operands := make([]linalgPointwiseOperand, len(args))
	resultKind := "scalar"
	rows, cols, length := 0, 0, 0
	for i, arg := range args {
		operand, err := linalgPointwiseDecode(name, arg)
		if err != nil {
			return nil, err
		}
		operands[i] = operand
		switch operand.kind {
		case "matrix":
			if resultKind == "vector" {
				return nil, fmt.Errorf("%s: cannot mix matrix and vector operands", name)
			}
			if resultKind == "matrix" && (operand.rows != rows || operand.cols != cols) {
				return nil, fmt.Errorf("%s: matrix shape mismatch", name)
			}
			resultKind = "matrix"
			rows, cols = operand.rows, operand.cols
		case "vector":
			if resultKind == "matrix" {
				return nil, fmt.Errorf("%s: cannot mix matrix and vector operands", name)
			}
			if resultKind == "vector" && len(operand.data) != length {
				return nil, fmt.Errorf("%s: vector length mismatch", name)
			}
			resultKind = "vector"
			length = len(operand.data)
		}
	}
	switch resultKind {
	case "matrix":
		out := make([]float64, rows*cols)
		for i := range out {
			out[i] = operands[0].valueAt(i)
		}
		for _, operand := range operands[1:] {
			for i := range out {
				out[i] = op(out[i], operand.valueAt(i))
			}
		}
		stddata.RecordLinalgMatrixKernel("LinalgMatrixPointwise", opName, rows, cols)
		return []Value{linalgMatrixDenseValue(rows, cols, out)}, nil
	case "vector":
		out := make([]float64, length)
		for i := range out {
			out[i] = operands[0].valueAt(i)
		}
		for _, operand := range operands[1:] {
			for i := range out {
				out[i] = op(out[i], operand.valueAt(i))
			}
		}
		stddata.RecordLinalgVectorKernel("LinalgVectorPointwise", opName, length)
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
	default:
		out := operands[0].scalar
		for _, operand := range operands[1:] {
			out = op(out, operand.scalar)
		}
		return []Value{FloatValue(out)}, nil
	}
}

func linalgPointwiseDecode(name string, value Value) (linalgPointwiseOperand, error) {
	if value.IsNumber() {
		return linalgPointwiseOperand{kind: "scalar", scalar: value.Number()}, nil
	}
	if rows, cols, values, ok, err := linalgMatrixValue(name, value); err != nil {
		return linalgPointwiseOperand{}, err
	} else if ok {
		return linalgPointwiseOperand{kind: "matrix", rows: rows, cols: cols, data: values}, nil
	}
	values, err := linalgVectorValue(name, value)
	if err != nil {
		return linalgPointwiseOperand{}, err
	}
	return linalgPointwiseOperand{kind: "vector", data: values}, nil
}

func (operand linalgPointwiseOperand) valueAt(index int) float64 {
	if operand.kind == "scalar" {
		return operand.scalar
	}
	return operand.data[index]
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
		if rows, cols, values, err := linalgNestedMatrix(name, value); err == nil {
			return rows, cols, values, true, nil
		}
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

func linalgNestedMatrix(name string, value Value) (int, int, []float64, error) {
	if !value.IsTable() {
		return 0, 0, nil, fmt.Errorf("%s: expected nested numeric table", name)
	}
	t := value.Table()
	rows := t.Length()
	if rows == 0 {
		return 0, 0, nil, fmt.Errorf("%s: nested matrix must have at least one row", name)
	}
	first := t.RawGetInt(1)
	if !first.IsTable() {
		return 0, 0, nil, fmt.Errorf("%s: expected nested numeric table", name)
	}
	cols := first.Table().Length()
	if cols == 0 {
		return 0, 0, nil, fmt.Errorf("%s: nested matrix rows must not be empty", name)
	}
	out := make([]float64, rows*cols)
	for r := 0; r < rows; r++ {
		row := t.RawGetInt(int64(r + 1))
		if !row.IsTable() || row.Table().Length() != cols {
			return 0, 0, nil, fmt.Errorf("%s: nested matrix rows must have consistent length", name)
		}
		for c := 0; c < cols; c++ {
			x, err := linalgNumber(name, row.Table().RawGetInt(int64(c+1)))
			if err != nil {
				return 0, 0, nil, fmt.Errorf("%s[%d][%d]: %w", name, r+1, c+1, err)
			}
			out[r*cols+c] = x
		}
	}
	return rows, cols, out, nil
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

func linalgSolveDense(n, rhsCols int, a, b []float64) ([]float64, error) {
	m := append([]float64(nil), a...)
	x := append([]float64(nil), b...)
	for k := 0; k < n; k++ {
		pivot := k
		pivotAbs := absFloat(m[k*n+k])
		for r := k + 1; r < n; r++ {
			if v := absFloat(m[r*n+k]); v > pivotAbs {
				pivot = r
				pivotAbs = v
			}
		}
		if pivotAbs == 0 {
			return nil, fmt.Errorf("linalg.solve: singular matrix")
		}
		if pivot != k {
			for c := k; c < n; c++ {
				m[k*n+c], m[pivot*n+c] = m[pivot*n+c], m[k*n+c]
			}
			for c := 0; c < rhsCols; c++ {
				x[k*rhsCols+c], x[pivot*rhsCols+c] = x[pivot*rhsCols+c], x[k*rhsCols+c]
			}
		}
		for r := k + 1; r < n; r++ {
			factor := m[r*n+k] / m[k*n+k]
			m[r*n+k] = 0
			for c := k + 1; c < n; c++ {
				m[r*n+c] -= factor * m[k*n+c]
			}
			for c := 0; c < rhsCols; c++ {
				x[r*rhsCols+c] -= factor * x[k*rhsCols+c]
			}
		}
	}
	for k := n - 1; k >= 0; k-- {
		pivot := m[k*n+k]
		if pivot == 0 {
			return nil, fmt.Errorf("linalg.solve: singular matrix")
		}
		for c := 0; c < rhsCols; c++ {
			sum := x[k*rhsCols+c]
			for j := k + 1; j < n; j++ {
				sum -= m[k*n+j] * x[j*rhsCols+c]
			}
			x[k*rhsCols+c] = sum / pivot
		}
	}
	return x, nil
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
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
