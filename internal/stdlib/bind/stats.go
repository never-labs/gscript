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
	set("sum", statsSum)
	set("mean", statsMean)
	set("min", statsMin)
	set("max", statsMax)
	set("var", statsVar)
	set("std", statsStd)
	set("normalize", statsNormalize)
	set("zscore", statsNormalize)
	set("normal_pdf", statsNormalPDF)
	set("log_normal_pdf", statsLogNormalPDF)
	set("normalize_weights", statsNormalizeWeights)
	set("normalize_log_weights", statsNormalizeLogWeights)
	set("effective_sample_size", statsEffectiveSampleSize)
	set("logsumexp", statsLogSumExp)
	set("weighted_mean", statsWeightedMean)
	set("weighted_var", statsWeightedVar)
	set("weighted_std", statsWeightedStd)
	set("cumsum", statsCumsum)
	set("diff", statsDiff)
	set("fill", statsFill)
	set("uniform_weights", statsUniformWeights)
	set("gather", statsGather)
	set("rms", statsRMS)
	set("rmse", statsRMSE)
	set("resample", statsResample)
	set("systematic_resample", statsSystematicResample)
	return t
}

func statsSum(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.sum", args)
	if err != nil {
		return nil, err
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return []Value{FloatValue(sum)}, nil
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

func statsMin(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.min", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.min: empty input")
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return []Value{FloatValue(min)}, nil
}

func statsMax(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.max", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.max: empty input")
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return []Value{FloatValue(max)}, nil
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

func statsNormalPDF(args []Value) ([]Value, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("stats.normal_pdf: need x, mean, stddev")
	}
	mean, err := linalgNumber("stats.normal_pdf", args[1])
	if err != nil {
		return nil, err
	}
	stddev, err := linalgNumber("stats.normal_pdf", args[2])
	if err != nil {
		return nil, err
	}
	if stddev <= 0 {
		return nil, fmt.Errorf("stats.normal_pdf: stddev must be positive")
	}
	eval := func(x float64) float64 {
		z := (x - mean) / stddev
		return math.Exp(-0.5*z*z) / (stddev * math.Sqrt(2*math.Pi))
	}
	if args[0].IsNumber() {
		return []Value{FloatValue(eval(toFloat(args[0])))}, nil
	}
	values, err := linalgVectorValue("stats.normal_pdf", args[0])
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = eval(v)
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func statsLogNormalPDF(args []Value) ([]Value, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("stats.log_normal_pdf: need x, mean, stddev")
	}
	mean, err := linalgNumber("stats.log_normal_pdf", args[1])
	if err != nil {
		return nil, err
	}
	stddev, err := linalgNumber("stats.log_normal_pdf", args[2])
	if err != nil {
		return nil, err
	}
	if stddev <= 0 {
		return nil, fmt.Errorf("stats.log_normal_pdf: stddev must be positive")
	}
	logNorm := -math.Log(stddev) - 0.5*math.Log(2*math.Pi)
	eval := func(x float64) float64 {
		z := (x - mean) / stddev
		return logNorm - 0.5*z*z
	}
	if args[0].IsNumber() {
		return []Value{FloatValue(eval(toFloat(args[0])))}, nil
	}
	values, err := linalgVectorValue("stats.log_normal_pdf", args[0])
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = eval(v)
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

func statsUniformWeights(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("stats.uniform_weights: need size")
	}
	n, err := linalgPositiveInt("stats.uniform_weights", args[0], "size")
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n > 0 {
		w := 1.0 / float64(n)
		for i := range out {
			out[i] = w
		}
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

func statsRMS(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.rms", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.rms: empty input")
	}
	sum := 0.0
	for _, v := range values {
		sum += v * v
	}
	return []Value{FloatValue(math.Sqrt(sum / float64(len(values))))}, nil
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

func statsResample(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("stats.resample: need values and weights")
	}
	values, err := linalgVectorValue("stats.resample", args[0])
	if err != nil {
		return nil, err
	}
	weights, total, err := statsWeights("stats.resample", []Value{args[1]})
	if err != nil {
		return nil, err
	}
	if len(values) != len(weights) {
		return nil, fmt.Errorf("stats.resample: values and weights length mismatch")
	}
	offset := 0.5
	if len(args) >= 3 {
		offset, err = linalgNumber("stats.resample", args[2])
		if err != nil {
			return nil, err
		}
		if offset < 0 || offset >= 1 {
			return nil, fmt.Errorf("stats.resample: offset must be in [0, 1)")
		}
	}
	n := len(weights)
	indexes := NewAppendArrayTable(n)
	out := make([]float64, n)
	cumulative := 0.0
	j := 0
	for i := 0; i < n; i++ {
		pos := (float64(i) + offset) / float64(n)
		for j < n-1 && pos > (cumulative+weights[j])/total {
			cumulative += weights[j]
			j++
		}
		indexes.RawSetInt(int64(i+1), IntValue(int64(j+1)))
		out[i] = values[j]
	}
	uniform := make([]float64, n)
	if n > 0 {
		w := 1.0 / float64(n)
		for i := range uniform {
			uniform[i] = w
		}
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out)), DenseArrayValue(NewDenseArrayF64Owned(uniform)), TableValue(indexes)}, nil
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

func statsWeightedVar(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("stats.weighted_var: need values and weights")
	}
	values, err := linalgVectorValue("stats.weighted_var", args[0])
	if err != nil {
		return nil, err
	}
	weights, total, err := statsWeights("stats.weighted_var", []Value{args[1]})
	if err != nil {
		return nil, err
	}
	if len(values) == 0 || len(values) != len(weights) {
		return nil, fmt.Errorf("stats.weighted_var: length mismatch")
	}
	mean := 0.0
	for i, v := range values {
		mean += v * weights[i]
	}
	mean /= total
	variance := 0.0
	for i, v := range values {
		d := v - mean
		variance += weights[i] * d * d
	}
	return []Value{FloatValue(variance / total)}, nil
}

func statsWeightedStd(args []Value) ([]Value, error) {
	variance, err := statsWeightedVar(args)
	if err != nil {
		return nil, err
	}
	return []Value{FloatValue(math.Sqrt(variance[0].Number()))}, nil
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

func statsNormalizeLogWeights(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.normalize_log_weights", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.normalize_log_weights: empty weights")
	}
	logTotal := statsLogSumExpOf(values)
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = math.Exp(v - logTotal)
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

func statsLogSumExp(args []Value) ([]Value, error) {
	values, err := statsVectorArg("stats.logsumexp", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.logsumexp: empty input")
	}
	return []Value{FloatValue(statsLogSumExpOf(values))}, nil
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

func statsLogSumExpOf(values []float64) float64 {
	maxValue := values[0]
	for _, v := range values[1:] {
		if v > maxValue {
			maxValue = v
		}
	}
	if math.IsInf(maxValue, -1) {
		return maxValue
	}
	sum := 0.0
	for _, v := range values {
		sum += math.Exp(v - maxValue)
	}
	return maxValue + math.Log(sum)
}
