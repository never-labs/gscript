package bind

import (
	"fmt"
	"math/rand"
	"time"

	stdrand "github.com/never-labs/leia/internal/stdlib/lib/rand"
)

// BuildRand creates the "rand" standard library table.
// Provides a dedicated random number generation library with seeded generators,
// distributions, shuffle, choice, and other utilities.
// Inspired by Odin's math/rand package.
func BuildRand() *Table {
	t := NewTable()

	// Local random source, seeded with current time by default
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "rand." + name,
			Fn:   fn,
		}))
	}
	requireNumber := func(fn string, idx int, v Value) error {
		if !v.IsNumber() {
			return fmt.Errorf("bad argument #%d to '%s' (number expected, got %s)", idx, fn, v.TypeName())
		}
		return nil
	}
	addNoise := func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad arguments to 'rand.add_noise' (values and distribution expected)")
		}
		valuesValue := args[0]
		weightsValue := NilValue()
		if statsIsSampleSetValue(args[0]) {
			samples, err := statsSampleSetFromValue("rand.add_noise", args[0])
			if err != nil {
				return nil, err
			}
			valuesValue = samples.values
			weightsValue = samples.weights
		}
		values, err := linalgVectorValue("rand.add_noise", valuesValue)
		if err != nil {
			return nil, err
		}
		dist, err := statsDistributionFromValue("rand.add_noise", args[1])
		if err != nil {
			return nil, err
		}
		drift := 0.0
		if len(args) >= 3 {
			if err := requireNumber("rand.add_noise", 3, args[2]); err != nil {
				return nil, err
			}
			drift = toFloat(args[2])
		}
		out := make([]float64, len(values))
		switch dist.name {
		case "normal":
			for i, value := range values {
				noise, err := stdrand.Normal(rng.NormFloat64, dist.mean, dist.stddev)
				if err != nil {
					return nil, fmt.Errorf("rand.add_noise: %s", err)
				}
				out[i] = value + drift + noise
			}
		default:
			return nil, fmt.Errorf("rand.add_noise: unsupported distribution %q", dist.name)
		}
		outValue := DenseArrayValue(NewDenseArrayF64Owned(out))
		if !weightsValue.IsNil() {
			return statsMakeSampleSet("rand.add_noise", outValue, weightsValue, nil)
		}
		return []Value{outValue}, nil
	}

	// rand.seed(n) - seed the random source
	set("seed", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'rand.seed' (number expected)")
		}
		if err := requireNumber("rand.seed", 1, args[0]); err != nil {
			return nil, err
		}
		seed := toInt(args[0])
		rng.Seed(seed)
		return nil, nil
	})

	// rand.int([min, max]) - random integer
	// No args: random int48 (fits in NaN-boxed integer)
	// One arg: [0, max)
	// Two args: [min, max]
	set("int", func(args []Value) ([]Value, error) {
		if len(args) == 0 {
			// Mask to 47 bits to guarantee NaN-boxed int (not float promotion).
			return []Value{IntValue(stdrand.MaskInt48(rng.Int63()))}, nil
		}
		if len(args) == 1 {
			if err := requireNumber("rand.int", 1, args[0]); err != nil {
				return nil, err
			}
			n, err := stdrand.IntBelow(rng.Int63n, toInt(args[0]))
			if err != nil {
				return nil, fmt.Errorf("bad argument #1 to 'rand.int' (positive number expected)")
			}
			return []Value{IntValue(n)}, nil
		}
		if err := requireNumber("rand.int", 1, args[0]); err != nil {
			return nil, err
		}
		if err := requireNumber("rand.int", 2, args[1]); err != nil {
			return nil, err
		}
		min := toInt(args[0])
		max := toInt(args[1])
		span, err := stdrand.InclusiveSpan(min, max)
		if err != nil {
			return nil, fmt.Errorf("bad argument to 'rand.int' (min > max)")
		}
		return []Value{IntValue(min + rng.Int63n(span))}, nil
	})

	// rand.float() - random float64 in [0.0, 1.0)
	set("float", func(args []Value) ([]Value, error) {
		return []Value{FloatValue(rng.Float64())}, nil
	})

	// rand.normal([mean, stddev]) - sample from normal distribution
	// Defaults: mean=0, stddev=1
	set("normal", func(args []Value) ([]Value, error) {
		mean := 0.0
		stddev := 1.0
		if len(args) >= 1 {
			if err := requireNumber("rand.normal", 1, args[0]); err != nil {
				return nil, err
			}
			mean = toFloat(args[0])
		}
		if len(args) >= 2 {
			if err := requireNumber("rand.normal", 2, args[1]); err != nil {
				return nil, err
			}
			stddev = toFloat(args[1])
			if stddev < 0 {
				return nil, fmt.Errorf("bad argument #2 to 'rand.normal' (non-negative stddev expected)")
			}
		}
		n, err := stdrand.Normal(rng.NormFloat64, mean, stddev)
		if err != nil {
			return nil, fmt.Errorf("bad argument #2 to 'rand.normal' (non-negative stddev expected)")
		}
		return []Value{FloatValue(n)}, nil
	})

	// rand.normal_vec(n[, mean, stddev]) - sample a dense normal vector
	set("normal_vec", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'rand.normal_vec' (size expected)")
		}
		n, err := linalgPositiveInt("rand.normal_vec", args[0], "size")
		if err != nil {
			return nil, err
		}
		mean := 0.0
		stddev := 1.0
		if len(args) >= 2 {
			if err := requireNumber("rand.normal_vec", 2, args[1]); err != nil {
				return nil, err
			}
			mean = toFloat(args[1])
		}
		if len(args) >= 3 {
			if err := requireNumber("rand.normal_vec", 3, args[2]); err != nil {
				return nil, err
			}
			stddev = toFloat(args[2])
		}
		if stddev < 0 {
			return nil, fmt.Errorf("bad argument #3 to 'rand.normal_vec' (non-negative stddev expected)")
		}
		out := make([]float64, n)
		for i := range out {
			v, err := stdrand.Normal(rng.NormFloat64, mean, stddev)
			if err != nil {
				return nil, fmt.Errorf("bad argument #3 to 'rand.normal_vec' (non-negative stddev expected)")
			}
			out[i] = v
		}
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
	})

	// rand.uniform_vec(n[, max] | n, min, max) - sample a dense uniform vector
	set("uniform_vec", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'rand.uniform_vec' (size expected)")
		}
		n, err := linalgPositiveInt("rand.uniform_vec", args[0], "size")
		if err != nil {
			return nil, err
		}
		min := 0.0
		max := 1.0
		if len(args) == 2 {
			if err := requireNumber("rand.uniform_vec", 2, args[1]); err != nil {
				return nil, err
			}
			max = toFloat(args[1])
		}
		if len(args) >= 3 {
			if err := requireNumber("rand.uniform_vec", 2, args[1]); err != nil {
				return nil, err
			}
			if err := requireNumber("rand.uniform_vec", 3, args[2]); err != nil {
				return nil, err
			}
			min = toFloat(args[1])
			max = toFloat(args[2])
		}
		if max < min {
			return nil, fmt.Errorf("bad arguments to 'rand.uniform_vec' (min must be <= max)")
		}
		out := make([]float64, n)
		width := max - min
		for i := range out {
			out[i] = min + width*rng.Float64()
		}
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
	})

	// rand.exp([rate]) - sample from exponential distribution
	// Default rate=1
	set("exp", func(args []Value) ([]Value, error) {
		rate := 1.0
		if len(args) >= 1 {
			if err := requireNumber("rand.exp", 1, args[0]); err != nil {
				return nil, err
			}
			rate = toFloat(args[0])
			if rate <= 0 {
				return nil, fmt.Errorf("bad argument #1 to 'rand.exp' (positive rate expected)")
			}
		}
		n, err := stdrand.Exponential(rng.ExpFloat64, rate)
		if err != nil {
			return nil, fmt.Errorf("bad argument #1 to 'rand.exp' (positive rate expected)")
		}
		return []Value{FloatValue(n)}, nil
	})

	// rand.exp_vec(n[, rate]) - sample a dense exponential vector
	set("exp_vec", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'rand.exp_vec' (size expected)")
		}
		n, err := linalgPositiveInt("rand.exp_vec", args[0], "size")
		if err != nil {
			return nil, err
		}
		rate := 1.0
		if len(args) >= 2 {
			if err := requireNumber("rand.exp_vec", 2, args[1]); err != nil {
				return nil, err
			}
			rate = toFloat(args[1])
			if rate <= 0 {
				return nil, fmt.Errorf("bad argument #2 to 'rand.exp_vec' (positive rate expected)")
			}
		}
		out := make([]float64, n)
		for i := range out {
			v, err := stdrand.Exponential(rng.ExpFloat64, rate)
			if err != nil {
				return nil, fmt.Errorf("bad argument #2 to 'rand.exp_vec' (positive rate expected)")
			}
			out[i] = v
		}
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
	})

	// rand.bool() - random boolean (50/50)
	set("bool", func(args []Value) ([]Value, error) {
		return []Value{BoolValue(rng.Intn(2) == 1)}, nil
	})

	// rand.choice(table) - pick a random element from an array-like table
	set("choice", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'rand.choice' (table expected)")
		}
		if !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'rand.choice' (table expected, got %s)", args[0].TypeName())
		}
		tbl := args[0].Table()
		length := tbl.Length()
		if length == 0 {
			return []Value{NilValue()}, nil
		}
		idx := rng.Intn(length) + 1 // 1-based indexing
		return []Value{tbl.RawGet(IntValue(int64(idx)))}, nil
	})

	// rand.shuffle(table) - shuffle an array-like table in place, returns the table
	set("shuffle", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'rand.shuffle' (table expected)")
		}
		if !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'rand.shuffle' (table expected, got %s)", args[0].TypeName())
		}
		tbl := args[0].Table()
		length := tbl.Length()
		// Fisher-Yates shuffle
		for i := length; i > 1; i-- {
			j := rng.Intn(i) + 1 // j in [1, i]
			vi := tbl.RawGet(IntValue(int64(i)))
			vj := tbl.RawGet(IntValue(int64(j)))
			tbl.RawSet(IntValue(int64(i)), vj)
			tbl.RawSet(IntValue(int64(j)), vi)
		}
		return []Value{args[0]}, nil
	})

	// rand.sample(table, n) - sample n unique elements from an array-like table
	// rand.sample(distribution[, n]) - sample scalar or dense vector from a distribution
	set("sample", func(args []Value) ([]Value, error) {
		if len(args) >= 1 && statsIsDistributionValue(args[0]) {
			dist, err := statsDistributionFromValue("rand.sample", args[0])
			if err != nil {
				return nil, err
			}
			switch dist.name {
			case "normal":
				if len(args) == 1 {
					v, err := stdrand.Normal(rng.NormFloat64, dist.mean, dist.stddev)
					if err != nil {
						return nil, fmt.Errorf("rand.sample: %s", err)
					}
					return []Value{FloatValue(v)}, nil
				}
				if err := requireNumber("rand.sample", 2, args[1]); err != nil {
					return nil, err
				}
				n := int(toInt(args[1]))
				if n < 0 {
					return nil, fmt.Errorf("bad argument #2 to 'rand.sample' (non-negative count expected)")
				}
				out := make([]float64, n)
				for i := range out {
					v, err := stdrand.Normal(rng.NormFloat64, dist.mean, dist.stddev)
					if err != nil {
						return nil, fmt.Errorf("rand.sample: %s", err)
					}
					out[i] = v
				}
				return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
			default:
				return nil, fmt.Errorf("rand.sample: unsupported distribution %q", dist.name)
			}
		}
		if len(args) < 2 {
			return nil, fmt.Errorf("bad arguments to 'rand.sample' (table and count expected)")
		}
		if !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'rand.sample' (table expected, got %s)", args[0].TypeName())
		}
		if err := requireNumber("rand.sample", 2, args[1]); err != nil {
			return nil, err
		}
		tbl := args[0].Table()
		n := int(toInt(args[1]))
		length := tbl.Length()
		n, err := stdrand.ClampSampleCount(n, length)
		if err != nil {
			return nil, fmt.Errorf("bad argument #2 to 'rand.sample' (non-negative count expected)")
		}
		// Copy indices
		indices := make([]int, length)
		for i := range indices {
			indices[i] = i + 1
		}
		// Partial Fisher-Yates
		result := NewTable()
		for i := 0; i < n; i++ {
			j := i + rng.Intn(length-i)
			indices[i], indices[j] = indices[j], indices[i]
			result.RawSet(IntValue(int64(i+1)), tbl.RawGet(IntValue(int64(indices[i]))))
		}
		return []Value{TableValue(result)}, nil
	})

	// rand.add_noise(values, distribution[, drift]) - add distribution noise and optional drift to a dense vector
	set("add_noise", addNoise)

	// rand.particle_filter(samples, observations, opts) - run a compact sequential Monte Carlo loop
	set("particle_filter", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad arguments to 'rand.particle_filter' (samples, observations, and options expected)")
		}
		if !args[2].IsTable() {
			return nil, fmt.Errorf("rand.particle_filter: options must be a table")
		}
		observations, err := linalgVectorValue("rand.particle_filter observations", args[1])
		if err != nil {
			return nil, err
		}
		if len(observations) == 0 {
			return nil, fmt.Errorf("rand.particle_filter: observations must not be empty")
		}
		opts := args[2].Table()
		processNoise, err := randParticleFilterOption(opts, "process_noise", "process")
		if err != nil {
			return nil, err
		}
		sensorNoise, err := randParticleFilterOption(opts, "sensor_noise", "observation_noise", "sensor", "observation")
		if err != nil {
			return nil, err
		}
		drift := FloatValue(0)
		if value := opts.RawGetString("drift"); !value.IsNil() {
			if err := requireNumber("rand.particle_filter", 0, value); err != nil {
				return nil, err
			}
			drift = value
		}
		ensemble := args[0]
		if !statsIsSampleSetValue(ensemble) {
			samples, err := statsSamples([]Value{ensemble})
			if err != nil {
				return nil, fmt.Errorf("rand.particle_filter: %w", err)
			}
			ensemble = samples[0]
		}
		keepTrajectory := opts.RawGetString("trajectory").Truthy() || opts.RawGetString("states").Truthy()
		estimates := make([]float64, 0, len(observations))
		trajectory := NewAppendArrayTable(len(observations))
		for i, observation := range observations {
			noisy, err := addNoise([]Value{ensemble, processNoise, drift})
			if err != nil {
				return nil, fmt.Errorf("rand.particle_filter: %w", err)
			}
			observed, err := statsObserve([]Value{noisy[0], sensorNoise, FloatValue(observation), TableValue(opts)})
			if err != nil {
				return nil, fmt.Errorf("rand.particle_filter: %w", err)
			}
			ensemble = observed[0]
			summary := ensemble.Table().RawGetString("summary")
			mean := summary.Table().RawGetString("mean")
			if !mean.IsNumber() {
				return nil, fmt.Errorf("rand.particle_filter: summary mean must be numeric")
			}
			estimates = append(estimates, mean.Number())
			if keepTrajectory {
				trajectory.RawSetInt(int64(i+1), ensemble)
			}
		}
		summary, err := statsDescribe([]Value{ensemble})
		if err != nil {
			return nil, fmt.Errorf("rand.particle_filter: %w", err)
		}
		out := NewTable()
		out.RawSetString("kind", StringValue("particle_filter_result"))
		out.RawSetString("ensemble", ensemble)
		out.RawSetString("samples", ensemble)
		out.RawSetString("final", ensemble)
		out.RawSetString("summary", summary[0])
		finalTable := ensemble.Table()
		out.RawSetString("ess", finalTable.RawGetString("ess"))
		out.RawSetString("resampled", finalTable.RawGetString("resampled"))
		out.RawSetString("indexes", finalTable.RawGetString("indexes"))
		out.RawSetString("estimates", DenseArrayValue(NewDenseArrayF64Owned(estimates)))
		if keepTrajectory {
			out.RawSetString("states", TableValue(trajectory))
			out.RawSetString("trajectory", TableValue(trajectory))
		}
		return []Value{TableValue(out)}, nil
	})

	// rand.uuid() - generate a random UUID v4 string
	set("uuid", func(args []Value) ([]Value, error) {
		return []Value{StringValue(stdrand.UUIDV4(func() byte { return byte(rng.Intn(256)) }))}, nil
	})

	// rand.bytes(n) - generate n random bytes as a string
	set("bytes", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'rand.bytes' (number expected)")
		}
		if err := requireNumber("rand.bytes", 1, args[0]); err != nil {
			return nil, err
		}
		n := int(toInt(args[0]))
		buf, err := stdrand.Bytes(n, func() byte { return byte(rng.Intn(256)) })
		if err != nil {
			return nil, fmt.Errorf("bad argument #1 to 'rand.bytes' (non-negative number expected)")
		}
		return []Value{StringValue(string(buf))}, nil
	})

	// rand.weighted(table, weights) - pick a random element using weighted probabilities
	// weights is a table of positive numbers corresponding to each element in table
	set("weighted", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad arguments to 'rand.weighted' (table and weights expected)")
		}
		if !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'rand.weighted' (table expected)")
		}
		if !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument #2 to 'rand.weighted' (table expected)")
		}
		items := args[0].Table()
		weights := args[1].Table()
		length := items.Length()
		if length == 0 {
			return []Value{NilValue()}, nil
		}

		weightValues := make([]float64, length)
		for i := 1; i <= length; i++ {
			w := weights.RawGet(IntValue(int64(i)))
			if w.IsNil() {
				return nil, fmt.Errorf("rand.weighted: weight at index %d is nil", i)
			}
			if !w.IsNumber() {
				return nil, fmt.Errorf("rand.weighted: weight at index %d is not a number", i)
			}
			weightValues[i-1] = toFloat(w)
		}
		total, err := stdrand.ValidateWeights(weightValues)
		if err != nil {
			return nil, fmt.Errorf("rand.weighted: %s", err)
		}

		idx := stdrand.WeightedIndex(weightValues, rng.Float64()*total)
		return []Value{items.RawGet(IntValue(int64(idx + 1)))}, nil
	})

	// rand.timeSeed() - seed with current time (convenience)
	set("timeSeed", func(args []Value) ([]Value, error) {
		seed := time.Now().UnixNano()
		rng.Seed(seed)
		// Mask seed to 47 bits to guarantee NaN-boxed int (not float promotion).
		return []Value{IntValue(stdrand.MaskInt48(seed))}, nil
	})

	return t
}

func randParticleFilterOption(opts *Table, names ...string) (Value, error) {
	for _, name := range names {
		if value := opts.RawGetString(name); !value.IsNil() {
			return value, nil
		}
	}
	return NilValue(), fmt.Errorf("rand.particle_filter: missing option %s", names[0])
}
