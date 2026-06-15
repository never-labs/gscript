package bind

import (
	"fmt"
	"math"

	stddata "github.com/never-labs/leia/internal/stdlib/lib/data"
)

// BuildControl creates the "control" standard library table.
func BuildControl() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "control." + name, Fn: fn}))
	}

	set("saturate", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[1].IsNumber() {
			return nil, fmt.Errorf("control.saturate: need value and numeric limit")
		}
		out, err := controlSaturateValue("control.saturate", args[0], args[1].Number())
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	})

	set("clamp", func(args []Value) ([]Value, error) {
		if len(args) < 3 || !args[1].IsNumber() || !args[2].IsNumber() {
			return nil, fmt.Errorf("control.clamp: need value, lower, upper")
		}
		out, err := controlClampValue("control.clamp", args[0], args[1].Number(), args[2].Number())
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	})

	set("wrap_angle", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsNumber() {
			return nil, fmt.Errorf("control.wrap_angle: need numeric angle")
		}
		return []Value{FloatValue(math.Atan2(math.Sin(args[0].Number()), math.Cos(args[0].Number())))}, nil
	})

	set("feedback", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("control.feedback: need gain and state")
		}
		state, err := linalgVectorValue("control.feedback", args[1])
		if err != nil {
			return nil, err
		}
		var opts *Table
		if len(args) >= 3 && args[2].IsTable() {
			opts = args[2].Table()
		}
		if opts != nil {
			if ref := opts.RawGetString("reference"); !ref.IsNil() {
				reference, err := linalgVectorValue("control.feedback reference", ref)
				if err != nil {
					return nil, err
				}
				if len(reference) != len(state) {
					return nil, fmt.Errorf("control.feedback: reference and state length mismatch")
				}
				for i := range state {
					state[i] -= reference[i]
				}
			}
		}
		if rows, cols, gain, ok, err := linalgMatrixValue("control.feedback", args[0]); err != nil {
			return nil, err
		} else if ok {
			if cols != len(state) {
				return nil, fmt.Errorf("control.feedback: gain and state dimension mismatch")
			}
			u := stddata.LinalgF64Matvec(rows, cols, gain, state)
			for i := range u {
				u[i] = -u[i]
			}
			if err := controlApplyFeedbackOptions(opts, u); err != nil {
				return nil, err
			}
			return []Value{DenseArrayValue(NewDenseArrayF64Owned(u))}, nil
		}
		gain, err := linalgVectorValue("control.feedback", args[0])
		if err != nil {
			return nil, err
		}
		if len(gain) != len(state) {
			return nil, fmt.Errorf("control.feedback: gain and state length mismatch")
		}
		u := []float64{-stddata.LinalgF64VectorDot(gain, state)}
		if err := controlApplyFeedbackOptions(opts, u); err != nil {
			return nil, err
		}
		return []Value{FloatValue(u[0])}, nil
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
	return controlClamp(x, -limit, limit)
}

func controlClamp(x, lower, upper float64) float64 {
	if lower > upper {
		lower, upper = upper, lower
	}
	if x < lower {
		return lower
	}
	if x > upper {
		return upper
	}
	return x
}

func controlSaturateValue(name string, value Value, limit float64) (Value, error) {
	return controlMapValue(name, value, func(x float64) float64 { return controlSaturate(x, limit) })
}

func controlClampValue(name string, value Value, lower, upper float64) (Value, error) {
	return controlMapValue(name, value, func(x float64) float64 { return controlClamp(x, lower, upper) })
}

func controlMapValue(name string, value Value, fn func(float64) float64) (Value, error) {
	if value.IsNumber() {
		return FloatValue(fn(value.Number())), nil
	}
	if rows, cols, values, ok, err := linalgMatrixValue(name, value); err != nil {
		return NilValue(), err
	} else if ok {
		out := make([]float64, len(values))
		for i, v := range values {
			out[i] = fn(v)
		}
		return linalgMatrixDenseValue(rows, cols, out), nil
	}
	values, err := linalgVectorValue(name, value)
	if err != nil {
		return NilValue(), err
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = fn(v)
	}
	return DenseArrayValue(NewDenseArrayF64Owned(out)), nil
}

func controlApplyFeedbackOptions(opts *Table, values []float64) error {
	if opts == nil {
		return nil
	}
	if ff := opts.RawGetString("feedforward"); !ff.IsNil() {
		if ff.IsNumber() {
			for i := range values {
				values[i] += ff.Number()
			}
		} else {
			feedforward, err := linalgVectorValue("control.feedback feedforward", ff)
			if err != nil {
				return err
			}
			if len(feedforward) != len(values) {
				return fmt.Errorf("control.feedback: feedforward length mismatch")
			}
			for i := range values {
				values[i] += feedforward[i]
			}
		}
	}
	if lower := opts.RawGetString("lower"); lower.IsNumber() {
		upper := opts.RawGetString("upper")
		if !upper.IsNumber() {
			return fmt.Errorf("control.feedback: upper must be numeric when lower is set")
		}
		for i, v := range values {
			values[i] = controlClamp(v, lower.Number(), upper.Number())
		}
		return nil
	}
	if limit := opts.RawGetString("limit"); limit.IsNumber() {
		for i, v := range values {
			values[i] = controlSaturate(v, limit.Number())
		}
	}
	return nil
}

type controlDenseMatrix struct {
	rows int
	cols int
	data []float64
}

func controlMatrix(v Value, name string) (controlDenseMatrix, error) {
	rows, cols, values, ok, err := linalgMatrixValue(name, v)
	if err != nil {
		return controlDenseMatrix{}, err
	}
	if ok {
		return controlDenseMatrix{rows: rows, cols: cols, data: values}, nil
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
