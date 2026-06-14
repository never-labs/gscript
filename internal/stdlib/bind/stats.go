package bind

import (
	"fmt"
	"math"
)

// BuildStats creates the "stats" standard library table.
func BuildStats() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "stats." + name, Fn: fn}))
	}
	set("mean", statsMean)
	set("var", statsVar)
	set("normalize", statsNormalize)
	set("weighted_mean", statsWeightedMean)
	set("cumsum", statsCumsum)
	set("systematic_resample", statsSystematicResample)
	return t
}

func statsMean(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.mean", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.mean: empty input")
	}
	return []Value{FloatValue(statsMeanOf(values))}, nil
}

func statsVar(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.var", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.var: empty input")
	}
	mean := statsMeanOf(values)
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return []Value{FloatValue(sum / float64(len(values)))}, nil
}

func statsNormalize(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.normalize", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.normalize: empty input")
	}
	mean := statsMeanOf(values)
	variance := 0.0
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	stddev := math.Sqrt(variance / float64(len(values)))
	out := make([]float64, len(values))
	if stddev != 0 {
		for i, v := range values {
			out[i] = (v - mean) / stddev
		}
	}
	return []Value{TableValue(linalgVectorTable(out))}, nil
}

func statsWeightedMean(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("stats.weighted_mean: need values and weights")
	}
	values, err := linalgVectorValue("stats.weighted_mean", args[0])
	if err != nil {
		return nil, err
	}
	weights, err := linalgVectorValue("stats.weighted_mean", args[1])
	if err != nil {
		return nil, err
	}
	if len(values) == 0 || len(values) != len(weights) {
		return nil, fmt.Errorf("stats.weighted_mean: length mismatch")
	}
	weighted := 0.0
	total := 0.0
	for i, v := range values {
		weighted += v * weights[i]
		total += weights[i]
	}
	if total == 0 {
		return nil, fmt.Errorf("stats.weighted_mean: total weight is zero")
	}
	return []Value{FloatValue(weighted / total)}, nil
}

func statsCumsum(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.cumsum", args)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(values))
	sum := 0.0
	for i, v := range values {
		sum += v
		out[i] = sum
	}
	return []Value{TableValue(linalgVectorTable(out))}, nil
}

func statsSystematicResample(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("stats.systematic_resample: need weights")
	}
	weights, err := linalgVectorValue("stats.systematic_resample", args[0])
	if err != nil {
		return nil, err
	}
	if len(weights) == 0 {
		return nil, fmt.Errorf("stats.systematic_resample: empty weights")
	}
	offset := 0.5
	if len(args) >= 2 {
		offset, err = linalgNumber("stats.systematic_resample", args[1])
		if err != nil {
			return nil, err
		}
		if offset < 0 || offset >= 1 {
			return nil, fmt.Errorf("stats.systematic_resample: offset must be in [0, 1)")
		}
	}
	total := 0.0
	for _, w := range weights {
		if w < 0 {
			return nil, fmt.Errorf("stats.systematic_resample: weights must be non-negative")
		}
		total += w
	}
	if total == 0 {
		return nil, fmt.Errorf("stats.systematic_resample: total weight is zero")
	}
	n := len(weights)
	out := NewAppendArrayTable(n + 1)
	cumulative := weights[0] / total
	index := 0
	for i := 0; i < n; i++ {
		position := (offset + float64(i)) / float64(n)
		for position > cumulative && index < n-1 {
			index++
			cumulative += weights[index] / total
		}
		out.RawSetInt(int64(i+1), IntValue(int64(index+1)))
	}
	return []Value{TableValue(out)}, nil
}

func statsVectorArg(name string, args []Value) ([]float64, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("%s: need values", name)
	}
	return linalgVectorValue(name, args[0])
}

func statsMeanOf(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
