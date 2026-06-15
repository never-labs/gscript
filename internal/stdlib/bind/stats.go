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
	set("std", statsStd)
	set("normalize", statsNormalize)
	set("zscore", statsNormalize)
	set("normalize_weights", statsNormalizeWeights)
	set("effective_sample_size", statsEffectiveSampleSize)
	set("weighted_mean", statsWeightedMean)
	set("cumsum", statsCumsum)
	set("diff", statsDiff)
	set("fill", statsFill)
	set("gather", statsGather)
	set("rmse", statsRMSE)
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

func statsStd(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.std", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.std: empty input")
	}
	return []Value{FloatValue(math.Sqrt(statsVarianceOf(values)))}, nil
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
	stddev := math.Sqrt(statsVarianceWithMean(values, mean))
	out := make([]float64, len(values))
	if stddev != 0 {
		for i, v := range values {
			out[i] = (v - mean) / stddev
		}
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func statsDiff(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.diff", args)
	if err != nil {
		return nil, err
	}
	if len(values) < 2 {
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(nil))}, nil
	}
	out := make([]float64, len(values)-1)
	for i := 1; i < len(values); i++ {
		out[i-1] = values[i] - values[i-1]
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func statsFill(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("stats.fill: need size and value")
	}
	n, err := linalgPositiveInt("stats.fill", args[0], "size")
	if err != nil {
		return nil, err
	}
	value, err := linalgNumber("stats.fill", args[1])
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	for i := range out {
		out[i] = value
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func statsGather(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("stats.gather: need values and indexes")
	}
	values, err := linalgVectorValue("stats.gather", args[0])
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return []Value{DenseArrayValue(NewDenseArrayF64Owned(nil))}, nil
	}
	if !args[1].IsTable() && !args[1].IsDenseArray() {
		return nil, fmt.Errorf("stats.gather: indexes must be a numeric table or dense array")
	}
	indexes, err := linalgVectorValue("stats.gather", args[1])
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(indexes))
	for i, idxValue := range indexes {
		idx := int(idxValue)
		if idxValue != float64(idx) {
			return nil, fmt.Errorf("stats.gather: index %d must be an integer", i+1)
		}
		if idx < 1 || idx > len(values) {
			return nil, fmt.Errorf("stats.gather: index %d out of range", idx)
		}
		out[i] = values[idx-1]
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func statsRMSE(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("stats.rmse: need actual and predicted values")
	}
	actual, err := linalgVectorValue("stats.rmse", args[0])
	if err != nil {
		return nil, err
	}
	predicted, err := linalgVectorValue("stats.rmse", args[1])
	if err != nil {
		return nil, err
	}
	if len(actual) == 0 || len(actual) != len(predicted) {
		return nil, fmt.Errorf("stats.rmse: length mismatch")
	}
	sum := 0.0
	for i, v := range actual {
		d := v - predicted[i]
		sum += d * d
	}
	return []Value{FloatValue(math.Sqrt(sum / float64(len(actual))))}, nil
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

func statsNormalizeWeights(args []Value) ([]Value, error) {
	weights, total, err := statsWeights("stats.normalize_weights", args)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(weights))
	for i, w := range weights {
		out[i] = w / total
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func statsEffectiveSampleSize(args []Value) ([]Value, error) {
	weights, total, err := statsWeights("stats.effective_sample_size", args)
	if err != nil {
		return nil, err
	}
	sumSq := 0.0
	for _, w := range weights {
		normalized := w / total
		sumSq += normalized * normalized
	}
	if sumSq == 0 {
		return nil, fmt.Errorf("stats.effective_sample_size: total weight is zero")
	}
	return []Value{FloatValue(1.0 / sumSq)}, nil
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
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
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

func statsWeights(name string, args []Value) ([]float64, float64, error) {
	weights, err := statsVectorArg(name, args)
	if err != nil {
		return nil, 0, err
	}
	if len(weights) == 0 {
		return nil, 0, fmt.Errorf("%s: empty weights", name)
	}
	total := 0.0
	for _, w := range weights {
		if w < 0 {
			return nil, 0, fmt.Errorf("%s: weights must be non-negative", name)
		}
		total += w
	}
	if total == 0 {
		return nil, 0, fmt.Errorf("%s: total weight is zero", name)
	}
	return weights, total, nil
}

func statsMeanOf(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func statsVarianceOf(values []float64) float64 {
	return statsVarianceWithMean(values, statsMeanOf(values))
}

func statsVarianceWithMean(values []float64, mean float64) float64 {
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return sum / float64(len(values))
}
