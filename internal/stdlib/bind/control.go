package bind

import (
	"fmt"
	"math"
)

// BuildControl creates the "control" standard library table.
func BuildControl() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "control." + name, Fn: fn}))
	}

	set("saturate", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsNumber() || !args[1].IsNumber() {
			return nil, fmt.Errorf("control.saturate: need numeric value and limit")
		}
		return []Value{FloatValue(controlSaturate(args[0].Number(), args[1].Number()))}, nil
	})

	set("wrap_angle", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsNumber() {
			return nil, fmt.Errorf("control.wrap_angle: need numeric angle")
		}
		return []Value{FloatValue(math.Atan2(math.Sin(args[0].Number()), math.Cos(args[0].Number())))}, nil
	})

	set("lqr2", func(args []Value) ([]Value, error) {
		if len(args) < 4 {
			return nil, fmt.Errorf("control.lqr2: need A, B, Q, R")
		}
		A, err := controlMatrix(args[0], "control.lqr2 A")
		if err != nil {
			return nil, err
		}
		B, err := controlMatrix(args[1], "control.lqr2 B")
		if err != nil {
			return nil, err
		}
		Q, err := controlMatrix(args[2], "control.lqr2 Q")
		if err != nil {
			return nil, err
		}
		if A.rows != 2 || A.cols != 2 || B.rows != 2 || B.cols != 1 || Q.rows != 2 || Q.cols != 2 {
			return nil, fmt.Errorf("control.lqr2: expected A 2x2, B 2x1, Q 2x2")
		}
		if !args[3].IsNumber() {
			return nil, fmt.Errorf("control.lqr2: R must be numeric")
		}
		R := args[3].Number()
		if R == 0 {
			return nil, fmt.Errorf("control.lqr2: R must be non-zero")
		}
		iterations := 20000
		step := 0.001
		if len(args) >= 5 && args[4].IsTable() {
			opts := args[4].Table()
			if v := opts.RawGetString("iterations"); v.IsNumber() {
				iterations = int(v.Int())
			}
			if v := opts.RawGetString("step"); v.IsNumber() {
				step = v.Number()
			}
		}
		P := append([]float64(nil), Q.data...)
		for i := 0; i < iterations; i++ {
			dP := controlRiccati2(A.data, B.data, P, Q.data, R)
			maxDelta := 0.0
			for j := range P {
				delta := step * dP[j]
				P[j] += delta
				if math.Abs(delta) > maxDelta {
					maxDelta = math.Abs(delta)
				}
			}
			if maxDelta < 1e-12 {
				break
			}
		}
		k0 := (B.data[0]*P[0] + B.data[1]*P[2]) / R
		k1 := (B.data[0]*P[1] + B.data[1]*P[3]) / R
		return []Value{DenseArrayValue(NewDenseArrayF64Owned([]float64{k0, k1}))}, nil
	})

	return t
}

func controlSaturate(x, limit float64) float64 {
	if limit < 0 {
		limit = -limit
	}
	if x > limit {
		return limit
	}
	if x < -limit {
		return -limit
	}
	return x
}

type controlDenseMatrix struct {
	rows int
	cols int
	data []float64
}

func controlMatrix(v Value, name string) (controlDenseMatrix, error) {
	if v.IsTable() {
		t := v.Table()
		if backing, rows, stride, ok := t.DenseMatrixBacking(); ok {
			data := make([]float64, rows*stride)
			copy(data, backing[:rows*stride])
			return controlDenseMatrix{rows: rows, cols: stride, data: data}, nil
		}
		rowsValue := t.RawGetString("rows")
		colsValue := t.RawGetString("cols")
		valuesValue := t.RawGetString("values")
		if !rowsValue.IsNil() || !colsValue.IsNil() || !valuesValue.IsNil() {
			rows, cols, values, ok, err := linalgMatrixValue(name, v)
			if err != nil {
				return controlDenseMatrix{}, err
			}
			if ok {
				return controlDenseMatrix{rows: rows, cols: cols, data: values}, nil
			}
		}
		if t.Length() > 0 {
			rows := t.Length()
			first := t.RawGetInt(1)
			if first.IsTable() {
				cols := first.Table().Length()
				data := make([]float64, rows*cols)
				for i := 0; i < rows; i++ {
					row := t.RawGetInt(int64(i + 1))
					if !row.IsTable() || row.Table().Length() != cols {
						return controlDenseMatrix{}, fmt.Errorf("%s rows must have consistent length", name)
					}
					for j := 0; j < cols; j++ {
						x := row.Table().RawGetInt(int64(j + 1))
						if !x.IsNumber() {
							return controlDenseMatrix{}, fmt.Errorf("%s[%d][%d] must be numeric", name, i+1, j+1)
						}
						data[i*cols+j] = x.Number()
					}
				}
				return controlDenseMatrix{rows: rows, cols: cols, data: data}, nil
			}
		}
	}
	return controlDenseMatrix{}, fmt.Errorf("%s must be a dense matrix or nested numeric table", name)
}

func controlRiccati2(A, B, P, Q []float64, R float64) []float64 {
	// dP = A'P + PA - PBR^-1B'P + Q, all 2x2 except B 2x1.
	atp := []float64{
		A[0]*P[0] + A[2]*P[2],
		A[0]*P[1] + A[2]*P[3],
		A[1]*P[0] + A[3]*P[2],
		A[1]*P[1] + A[3]*P[3],
	}
	pa := []float64{
		P[0]*A[0] + P[1]*A[2],
		P[0]*A[1] + P[1]*A[3],
		P[2]*A[0] + P[3]*A[2],
		P[2]*A[1] + P[3]*A[3],
	}
	pb0 := P[0]*B[0] + P[1]*B[1]
	pb1 := P[2]*B[0] + P[3]*B[1]
	return []float64{
		atp[0] + pa[0] - pb0*pb0/R + Q[0],
		atp[1] + pa[1] - pb0*pb1/R + Q[1],
		atp[2] + pa[2] - pb1*pb0/R + Q[2],
		atp[3] + pa[3] - pb1*pb1/R + Q[3],
	}
}
