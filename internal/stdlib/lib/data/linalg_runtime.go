package data

import (
	"fmt"
	"math"
)

func LinalgVectorKernelShape(op string, n int) string {
	return fmt.Sprintf("linalg/vector/%s/f64/%d", op, n)
}

func LinalgMatrixKernelShape(op string, rows, cols int) string {
	return fmt.Sprintf("linalg/matrix/%s/f64/%dx%d", op, rows, cols)
}

func RecordLinalgVectorKernel(kernel, op string, n int) {
	RecordRuntimeKernelProbe(kernel, LinalgVectorKernelShape(op, n), true, nil)
}

func RecordLinalgMatrixKernel(kernel, op string, rows, cols int) {
	RecordRuntimeKernelProbe(kernel, LinalgMatrixKernelShape(op, rows, cols), true, nil)
}

func LinalgF64VectorScale(values []float64, scalar float64) []float64 {
	RecordLinalgVectorKernel("LinalgVectorScale", "scale", len(values))
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = v * scalar
	}
	return out
}

func LinalgF64MatrixScale(rows, cols int, values []float64, scalar float64) []float64 {
	RecordLinalgMatrixKernel("LinalgMatrixScale", "scale", rows, cols)
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = v * scalar
	}
	return out
}

func LinalgF64VectorDot(a, b []float64) float64 {
	RecordLinalgVectorKernel("LinalgVectorDot", "dot", len(a))
	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

func LinalgF64VectorNorm(values []float64) float64 {
	RecordLinalgVectorKernel("LinalgVectorNorm", "norm", len(values))
	sum := 0.0
	for _, v := range values {
		sum += v * v
	}
	return math.Sqrt(sum)
}

func LinalgF64Matvec(rows, cols int, matrix, vector []float64) []float64 {
	RecordLinalgVectorKernel("LinalgMatrixVector", fmt.Sprintf("matvec/%dx%d", rows, cols), rows)
	out := make([]float64, rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			out[r] += matrix[r*cols+c] * vector[c]
		}
	}
	return out
}

func LinalgF64Matmul(ar, ac, bc int, a, b []float64) []float64 {
	RecordLinalgMatrixKernel("LinalgMatrixMatmul", fmt.Sprintf("matmul/%dx%d", ac, ac), ar, bc)
	out := make([]float64, ar*bc)
	for r := 0; r < ar; r++ {
		for c := 0; c < bc; c++ {
			for k := 0; k < ac; k++ {
				out[r*bc+c] += a[r*ac+k] * b[k*bc+c]
			}
		}
	}
	return out
}

func LinalgF64Transpose(rows, cols int, values []float64) []float64 {
	RecordLinalgMatrixKernel("LinalgMatrixTranspose", "transpose", cols, rows)
	out := make([]float64, len(values))
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			out[c*rows+r] = values[r*cols+c]
		}
	}
	return out
}

func LinalgF64BinaryVector(kernel, op string, a, b []float64, fn func(float64, float64) float64) []float64 {
	RecordLinalgVectorKernel(kernel, op, len(a))
	out := make([]float64, len(a))
	for i := range a {
		out[i] = fn(a[i], b[i])
	}
	return out
}

func LinalgF64BinaryMatrix(kernel, op string, rows, cols int, a, b []float64, fn func(float64, float64) float64) []float64 {
	RecordLinalgMatrixKernel(kernel, op, rows, cols)
	out := make([]float64, len(a))
	for i := range a {
		out[i] = fn(a[i], b[i])
	}
	return out
}

func LinalgF64Solve2(a, b []float64) []float64 {
	RecordLinalgVectorKernel("LinalgMatrixSolve2", "solve2", 2)
	det := a[0]*a[3] - a[1]*a[2]
	return []float64{
		(b[0]*a[3] - a[1]*b[1]) / det,
		(a[0]*b[1] - b[0]*a[2]) / det,
	}
}
