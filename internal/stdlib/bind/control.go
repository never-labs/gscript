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
		return controlFeedbackValue(args)
	})

	set("policy", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("control.policy: need gain")
		}
		stateDim, err := controlGainDimension(args[0], "control.policy")
		if err != nil {
			return nil, err
		}
		policy := NewTable()
		policy.RawSetString("kind", StringValue("control_policy"))
		policy.RawSetString("gain", args[0])
		policy.RawSetString("state_dim", IntValue(int64(stateDim)))
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("control.policy: options must be a table")
			}
			opts := controlCopyTable(args[1].Table())
			if err := controlValidatePolicyOptions("control.policy", opts, stateDim); err != nil {
				return nil, err
			}
			policy.RawSetString("options", TableValue(opts))
		} else {
			policy.RawSetString("options", TableValue(NewTable()))
		}
		return []Value{TableValue(policy)}, nil
	})

	set("apply", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("control.apply: need policy and state")
		}
		var opts *Table
		if len(args) >= 3 {
			if !args[2].IsTable() {
				return nil, fmt.Errorf("control.apply: options must be a table")
			}
			opts = args[2].Table()
		}
		return controlApplyPolicyValue(args[0], args[1], opts)
	})

	set("lqr", controlLQR)

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

func controlLQR(args []Value) ([]Value, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("control.lqr: need A, B, Q, R")
	}
	A, err := controlMatrix(args[0], "control.lqr A")
	if err != nil {
		return nil, err
	}
	B, err := controlMatrix(args[1], "control.lqr B")
	if err != nil {
		return nil, err
	}
	Q, err := controlMatrix(args[2], "control.lqr Q")
	if err != nil {
		return nil, err
	}
	if A.rows != A.cols {
		return nil, fmt.Errorf("control.lqr: A must be square")
	}
	if B.rows != A.rows {
		return nil, fmt.Errorf("control.lqr: B row count must match A")
	}
	if Q.rows != A.rows || Q.cols != A.cols {
		return nil, fmt.Errorf("control.lqr: Q shape must match A")
	}
	if err := controlValidateSymmetric("control.lqr Q", Q, 1e-9); err != nil {
		return nil, err
	}
	if err := controlValidateNonNegativeDiagonal("control.lqr Q", Q); err != nil {
		return nil, err
	}
	R, err := controlRMatrix(args[3], B.cols)
	if err != nil {
		return nil, err
	}
	if err := controlValidateSymmetric("control.lqr R", R, 1e-9); err != nil {
		return nil, err
	}
	if err := controlValidatePositiveDiagonal("control.lqr R", R); err != nil {
		return nil, err
	}
	iterations := 20000
	step := 0.001
	tolerance := 1e-12
	if len(args) >= 5 && args[4].IsTable() {
		opts := args[4].Table()
		if v := opts.RawGetString("iterations"); v.IsNumber() {
			iterations = int(v.Int())
		}
		if v := opts.RawGetString("step"); v.IsNumber() {
			step = v.Number()
		}
		if v := opts.RawGetString("tolerance"); v.IsNumber() {
			tolerance = v.Number()
		}
	}
	if iterations <= 0 {
		return nil, fmt.Errorf("control.lqr: iterations must be positive")
	}
	if step <= 0 {
		return nil, fmt.Errorf("control.lqr: step must be positive")
	}
	if tolerance < 0 {
		return nil, fmt.Errorf("control.lqr: tolerance must be non-negative")
	}
	P := append([]float64(nil), Q.data...)
	for i := 0; i < iterations; i++ {
		dP, _, err := controlRiccati(A, B, P, Q.data, R)
		if err != nil {
			return nil, err
		}
		maxDelta := 0.0
		for j := range P {
			delta := step * dP[j]
			if math.IsNaN(delta) || math.IsInf(delta, 0) {
				return nil, fmt.Errorf("control.lqr: Riccati iteration diverged")
			}
			P[j] += delta
			if math.Abs(delta) > maxDelta {
				maxDelta = math.Abs(delta)
			}
		}
		if maxDelta < tolerance {
			break
		}
	}
	_, K, err := controlRiccati(A, B, P, Q.data, R)
	if err != nil {
		return nil, err
	}
	return []Value{linalgMatrixDenseValue(B.cols, A.rows, K)}, nil
}

func controlRMatrix(value Value, inputs int) (controlDenseMatrix, error) {
	if value.IsNumber() {
		r := value.Number()
		data := make([]float64, inputs*inputs)
		for i := 0; i < inputs; i++ {
			data[i*inputs+i] = r
		}
		return controlDenseMatrix{rows: inputs, cols: inputs, data: data}, nil
	}
	R, err := controlMatrix(value, "control.lqr R")
	if err != nil {
		return controlDenseMatrix{}, err
	}
	if R.rows != inputs || R.cols != inputs {
		return controlDenseMatrix{}, fmt.Errorf("control.lqr: R must be %dx%d", inputs, inputs)
	}
	return R, nil
}

type controlPolicyValue struct {
	gain     Value
	options  *Table
	stateDim int
}

func controlFeedbackValue(args []Value) ([]Value, error) {
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
		if rows == 1 {
			return []Value{FloatValue(u[0])}, nil
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
}

func controlApplyPolicyValue(policyValue, stateValue Value, overrides *Table) ([]Value, error) {
	policy, err := controlPolicyFromValue(policyValue, "control.apply")
	if err != nil {
		return nil, err
	}
	opts := controlCopyTable(policy.options)
	if overrides != nil {
		for _, key := range overrides.PairsKeysSnapshot() {
			opts.RawSet(key, overrides.RawGet(key))
		}
	}
	if err := controlValidatePolicyOptions("control.apply", opts, policy.stateDim); err != nil {
		return nil, err
	}
	state, err := controlPolicyStateValue("control.apply", stateValue, opts, policy.stateDim)
	if err != nil {
		return nil, err
	}
	return controlFeedbackValue([]Value{policy.gain, state, TableValue(opts)})
}

func controlPolicyFromValue(value Value, name string) (controlPolicyValue, error) {
	if !value.IsTable() {
		return controlPolicyValue{}, fmt.Errorf("%s: policy expected, got %s", name, value.TypeName())
	}
	t := value.Table()
	kind := t.RawGetString("kind")
	if !kind.IsString() || kind.Str() != "control_policy" {
		return controlPolicyValue{}, fmt.Errorf("%s: policy expected", name)
	}
	gain := t.RawGetString("gain")
	if gain.IsNil() {
		return controlPolicyValue{}, fmt.Errorf("%s: policy missing gain", name)
	}
	if _, err := controlGainDimension(gain, name); err != nil {
		return controlPolicyValue{}, err
	}
	stateDimValue := t.RawGetString("state_dim")
	if !stateDimValue.IsInt() || stateDimValue.Int() <= 0 {
		return controlPolicyValue{}, fmt.Errorf("%s: policy missing state dimension", name)
	}
	stateDim := int(stateDimValue.Int())
	options := t.RawGetString("options")
	if options.IsNil() {
		return controlPolicyValue{gain: gain, options: NewTable(), stateDim: stateDim}, nil
	}
	if !options.IsTable() {
		return controlPolicyValue{}, fmt.Errorf("%s: policy options must be a table", name)
	}
	return controlPolicyValue{gain: gain, options: options.Table(), stateDim: stateDim}, nil
}

func controlGainDimension(value Value, name string) (int, error) {
	if rows, cols, _, ok, err := linalgMatrixValue(name, value); err != nil {
		return 0, err
	} else if ok {
		if rows <= 0 || cols <= 0 {
			return 0, fmt.Errorf("%s: gain must not be empty", name)
		}
		return cols, nil
	}
	gain, err := linalgVectorValue(name, value)
	if err != nil {
		return 0, err
	}
	if len(gain) == 0 {
		return 0, fmt.Errorf("%s: gain must not be empty", name)
	}
	return len(gain), nil
}

func controlCopyTable(src *Table) *Table {
	dst := NewTable()
	if src == nil {
		return dst
	}
	for _, key := range src.PairsKeysSnapshot() {
		dst.RawSet(key, src.RawGet(key))
	}
	return dst
}

func controlValidatePolicyOptions(name string, opts *Table, stateDim int) error {
	meta, err := stateMetaFromOptions(name, opts, stateDim, "gain dimension")
	if err != nil {
		return err
	}
	return controlStorePolicyStateMeta(opts, meta)
}

func controlPolicyStateValue(name string, value Value, opts *Table, stateDim int) (Value, error) {
	meta, err := stateMetaFromOptions(name, opts, stateDim, "gain dimension")
	if err != nil {
		return NilValue(), err
	}
	if len(meta.names) == 0 {
		return value, nil
	}
	state, err := stateVectorFromValue(name, value, meta.names)
	if err != nil {
		return NilValue(), err
	}
	wrapStateIndexes(state, meta.wrap)
	return DenseArrayValue(NewDenseArrayF64Owned(state)), nil
}

func controlStorePolicyStateMeta(opts *Table, meta stateMeta) error {
	if opts == nil || meta.sourceKey == "" {
		return nil
	}
	source := opts.RawGetString(meta.sourceKey)
	if source.IsTable() {
		if opts.RawGetString("state_names").IsNil() && opts.RawGetString("stateNames").IsNil() {
			if names := source.Table().RawGetString("state_names"); !names.IsNil() {
				opts.RawSetString("state_names", names)
			} else if names := source.Table().RawGetString("names"); !names.IsNil() {
				opts.RawSetString("state_names", names)
			}
		}
		if opts.RawGetString("wrap_angles").IsNil() && opts.RawGetString("wrap").IsNil() {
			if wrap := source.Table().RawGetString("wrap_angles"); !wrap.IsNil() {
				opts.RawSetString("wrap_angles", wrap)
			} else if wrap := source.Table().RawGetString("wrap"); !wrap.IsNil() {
				opts.RawSetString("wrap_angles", wrap)
			}
		}
	}
	return nil
}

func controlValidateSymmetric(name string, matrix controlDenseMatrix, tolerance float64) error {
	if matrix.rows != matrix.cols {
		return fmt.Errorf("%s must be square", name)
	}
	for r := 0; r < matrix.rows; r++ {
		for c := r + 1; c < matrix.cols; c++ {
			if math.Abs(matrix.data[r*matrix.cols+c]-matrix.data[c*matrix.cols+r]) > tolerance {
				return fmt.Errorf("%s must be symmetric", name)
			}
		}
	}
	return nil
}

func controlValidateNonNegativeDiagonal(name string, matrix controlDenseMatrix) error {
	for i := 0; i < matrix.rows; i++ {
		if matrix.data[i*matrix.cols+i] < 0 {
			return fmt.Errorf("%s diagonal must be non-negative", name)
		}
	}
	return nil
}

func controlValidatePositiveDiagonal(name string, matrix controlDenseMatrix) error {
	for i := 0; i < matrix.rows; i++ {
		if matrix.data[i*matrix.cols+i] <= 0 {
			return fmt.Errorf("%s diagonal must be positive", name)
		}
	}
	return nil
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

func controlRiccati(A, B controlDenseMatrix, P, Q []float64, R controlDenseMatrix) ([]float64, []float64, error) {
	n := A.rows
	m := B.cols
	at := stddata.LinalgF64Transpose(A.rows, A.cols, A.data)
	atp := stddata.LinalgF64Matmul(n, n, n, at, P)
	pa := stddata.LinalgF64Matmul(n, n, n, P, A.data)
	bt := stddata.LinalgF64Transpose(B.rows, B.cols, B.data)
	btp := stddata.LinalgF64Matmul(m, n, n, bt, P)
	k, err := linalgSolveDense(m, n, R.data, btp)
	if err != nil {
		return nil, nil, err
	}
	pb := stddata.LinalgF64Matmul(n, n, m, P, B.data)
	term := stddata.LinalgF64Matmul(n, m, n, pb, k)
	dP := make([]float64, n*n)
	for i := range dP {
		dP[i] = atp[i] + pa[i] - term[i] + Q[i]
	}
	return dP, k, nil
}
