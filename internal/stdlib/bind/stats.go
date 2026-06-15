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
	set("variance", statsVar)
	set("std", statsStd)
	set("describe", statsDescribe)
	set("describe_fields", statsDescribeFields)
	set("samples", statsSamples)
	set("update", statsUpdate)
	set("normalize", statsNormalize)
	set("zscore", statsNormalize)
	set("normal", statsNormal)
	set("pdf", statsPDF)
	set("logpdf", statsLogPDF)
	set("loglik", statsLogLik)
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
	set("resample_if", statsResampleIf)
	set("importance_update", statsImportanceUpdate)
	set("bayes_update", statsBayesUpdate)
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

func statsDescribe(args []Value) ([]Value, error) {
	if len(args) == 1 && statsIsSampleSetValue(args[0]) {
		samples := args[0].Table()
		return statsDescribe([]Value{samples.RawGetString("values"), samples.RawGetString("weights")})
	}
	values, err := statsVectorArg("stats.describe", args)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.describe: empty input")
	}
	if len(args) >= 2 {
		return statsDescribeWeighted(values, args[1])
	}
	min := values[0]
	max := values[0]
	sum := 0.0
	sumSquares := 0.0
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
		sumSquares += v * v
	}
	mean := sum / float64(len(values))
	variance := statsVarianceWithMean(values, mean)
	out := NewTable()
	out.RawSetString("count", IntValue(int64(len(values))))
	out.RawSetString("sum", FloatValue(sum))
	out.RawSetString("mean", FloatValue(mean))
	out.RawSetString("variance", FloatValue(variance))
	out.RawSetString("var", FloatValue(variance))
	out.RawSetString("std", FloatValue(math.Sqrt(variance)))
	out.RawSetString("min", FloatValue(min))
	out.RawSetString("max", FloatValue(max))
	out.RawSetString("rms", FloatValue(math.Sqrt(sumSquares/float64(len(values)))))
	return []Value{TableValue(out)}, nil
}

func statsSamples(args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("stats.samples: need values")
	}
	values, err := linalgVectorValue("stats.samples", args[0])
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("stats.samples: empty input")
	}
	var weightsValue Value
	if len(args) >= 2 {
		weights, total, err := statsWeights("stats.samples", []Value{args[1]})
		if err != nil {
			return nil, err
		}
		if len(values) != len(weights) {
			return nil, fmt.Errorf("stats.samples: values and weights length mismatch")
		}
		weightsValue = DenseArrayValue(NewDenseArrayF64Owned(statsNormalizeWeightsOf(weights, total)))
	} else {
		weights, err := statsUniformWeights([]Value{IntValue(int64(len(values)))})
		if err != nil {
			return nil, err
		}
		weightsValue = weights[0]
	}
	return statsMakeSampleSet("stats.samples", args[0], weightsValue, nil)
}

func statsUpdate(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("stats.update: need samples and log_likelihoods")
	}
	samples, err := statsSampleSetFromValue("stats.update", args[0])
	if err != nil {
		return nil, err
	}
	updateArgs := []Value{samples.values, samples.weights, args[1]}
	if len(args) >= 3 {
		updateArgs = append(updateArgs, args[2])
	}
	return statsBayesUpdate(updateArgs)
}

func statsDescribeWeighted(values []float64, weightsValue Value) ([]Value, error) {
	weights, total, err := statsWeights("stats.describe", []Value{weightsValue})
	if err != nil {
		return nil, err
	}
	if len(values) != len(weights) {
		return nil, fmt.Errorf("stats.describe: length mismatch")
	}
	minSet := false
	min := 0.0
	max := 0.0
	weightedSum := 0.0
	weightedSquares := 0.0
	for i, v := range values {
		w := weights[i]
		if w > 0 {
			if !minSet {
				min = v
				max = v
				minSet = true
			} else {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}
		}
		weightedSum += v * w
		weightedSquares += v * v * w
	}
	mean := weightedSum / total
	variance := 0.0
	for i, v := range values {
		d := v - mean
		variance += weights[i] * d * d
	}
	variance /= total
	out := NewTable()
	out.RawSetString("count", IntValue(int64(len(values))))
	out.RawSetString("weight_sum", FloatValue(total))
	out.RawSetString("sum", FloatValue(weightedSum))
	out.RawSetString("weighted_sum", FloatValue(weightedSum))
	out.RawSetString("mean", FloatValue(mean))
	out.RawSetString("variance", FloatValue(variance))
	out.RawSetString("var", FloatValue(variance))
	out.RawSetString("std", FloatValue(math.Sqrt(variance)))
	out.RawSetString("min", FloatValue(min))
	out.RawSetString("max", FloatValue(max))
	out.RawSetString("rms", FloatValue(math.Sqrt(weightedSquares/total)))
	return []Value{TableValue(out)}, nil
}

func statsDescribeFields(args []Value) ([]Value, error) {
	if len(args) < 1 || !args[0].IsTable() {
		return nil, fmt.Errorf("stats.describe_fields: need table")
	}
	weights := NilValue()
	if len(args) >= 2 {
		weights = args[1]
	}
	src := args[0].Table()
	out := NewTable()
	count := 0
	for _, key := range src.PairsKeysSnapshot() {
		if !key.IsString() {
			return nil, fmt.Errorf("stats.describe_fields: field key must be string, got %s", key.TypeName())
		}
		name := key.Str()
		value := src.RawGet(key)
		describeArgs := []Value{value}
		if !weights.IsNil() {
			describeArgs = append(describeArgs, weights)
		}
		desc, err := statsDescribe(describeArgs)
		if err != nil {
			return nil, fmt.Errorf("stats.describe_fields field %q: %w", name, err)
		}
		out.RawSetString(name, desc[0])
		count++
	}
	if count == 0 {
		return nil, fmt.Errorf("stats.describe_fields: empty table")
	}
	return []Value{TableValue(out)}, nil
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
	return statsNormalPDFValue("stats.normal_pdf", args[0], mean, stddev, false)
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
	return statsNormalPDFValue("stats.log_normal_pdf", args[0], mean, stddev, true)
}

func statsNormal(args []Value) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("stats.normal: need mean and sigma")
	}
	mean, err := linalgNumber("stats.normal", args[0])
	if err != nil {
		return nil, err
	}
	stddev, err := linalgNumber("stats.normal", args[1])
	if err != nil {
		return nil, err
	}
	if stddev <= 0 {
		return nil, fmt.Errorf("stats.normal: sigma must be positive")
	}
	dist := NewTable()
	dist.RawSetString("kind", StringValue("distribution"))
	dist.RawSetString("name", StringValue("normal"))
	dist.RawSetString("distribution", StringValue("normal"))
	dist.RawSetString("mean", FloatValue(mean))
	dist.RawSetString("sigma", FloatValue(stddev))
	dist.RawSetString("stddev", FloatValue(stddev))
	return []Value{TableValue(dist)}, nil
}

func statsPDF(args []Value) ([]Value, error) {
	return statsDistributionPDF("stats.pdf", args, false)
}

func statsLogPDF(args []Value) ([]Value, error) {
	return statsDistributionPDF("stats.logpdf", args, true)
}

func statsLogLik(args []Value) ([]Value, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("stats.loglik: need distribution, observed, and predicted")
	}
	dist, err := statsDistributionFromValue("stats.loglik", args[0])
	if err != nil {
		return nil, err
	}
	switch dist.name {
	case "normal":
		return statsNormalLogLik(args[1], args[2], dist.mean, dist.stddev)
	default:
		return nil, fmt.Errorf("stats.loglik: unsupported distribution %q", dist.name)
	}
}

func statsDistributionPDF(fn string, args []Value, logPDF bool) ([]Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("%s: need distribution and x", fn)
	}
	dist, err := statsDistributionFromValue(fn, args[0])
	if err != nil {
		return nil, err
	}
	switch dist.name {
	case "normal":
		return statsNormalPDFValue(fn, args[1], dist.mean, dist.stddev, logPDF)
	default:
		return nil, fmt.Errorf("%s: unsupported distribution %q", fn, dist.name)
	}
}

type statsDistribution struct {
	name   string
	mean   float64
	stddev float64
}

func statsDistributionFromValue(fn string, v Value) (statsDistribution, error) {
	if !v.IsTable() {
		return statsDistribution{}, fmt.Errorf("%s: distribution expected, got %s", fn, v.TypeName())
	}
	tbl := v.Table()
	if kind := tbl.RawGetString("kind"); !kind.IsString() || kind.Str() != "distribution" {
		return statsDistribution{}, fmt.Errorf("%s: distribution expected", fn)
	}
	nameValue := tbl.RawGetString("distribution")
	if nameValue.IsNil() {
		nameValue = tbl.RawGetString("name")
	}
	if !nameValue.IsString() {
		return statsDistribution{}, fmt.Errorf("%s: distribution name expected", fn)
	}
	switch name := nameValue.Str(); name {
	case "normal":
		mean, err := linalgNumber(fn, tbl.RawGetString("mean"))
		if err != nil {
			return statsDistribution{}, err
		}
		stddevValue := tbl.RawGetString("sigma")
		if stddevValue.IsNil() {
			stddevValue = tbl.RawGetString("stddev")
		}
		stddev, err := linalgNumber(fn, stddevValue)
		if err != nil {
			return statsDistribution{}, err
		}
		if stddev <= 0 {
			return statsDistribution{}, fmt.Errorf("%s: normal sigma must be positive", fn)
		}
		return statsDistribution{name: name, mean: mean, stddev: stddev}, nil
	default:
		return statsDistribution{}, fmt.Errorf("%s: unsupported distribution %q", fn, name)
	}
}

func statsIsDistributionValue(v Value) bool {
	return v.IsTable() && v.Table().RawGetString("kind").IsString() && v.Table().RawGetString("kind").Str() == "distribution"
}

func statsNormalPDFValue(fn string, xValue Value, mean, stddev float64, logPDF bool) ([]Value, error) {
	eval := statsNormalPDFEvaluator(mean, stddev, logPDF)
	if xValue.IsNumber() {
		return []Value{FloatValue(eval(toFloat(xValue)))}, nil
	}
	values, err := linalgVectorValue(fn, xValue)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = eval(v)
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func statsNormalLogLik(observedValue, predictedValue Value, mean, stddev float64) ([]Value, error) {
	observed, err := linalgPointwiseDecode("stats.loglik", observedValue)
	if err != nil {
		return nil, err
	}
	predicted, err := linalgPointwiseDecode("stats.loglik", predictedValue)
	if err != nil {
		return nil, err
	}
	if observed.kind == "matrix" || predicted.kind == "matrix" {
		return nil, fmt.Errorf("stats.loglik: matrix operands are not supported")
	}
	eval := statsNormalPDFEvaluator(mean, stddev, true)
	if observed.kind == "scalar" && predicted.kind == "scalar" {
		return []Value{FloatValue(eval(observed.scalar - predicted.scalar))}, nil
	}
	length := len(observed.data)
	if observed.kind == "scalar" {
		length = len(predicted.data)
	} else if predicted.kind != "scalar" && len(predicted.data) != length {
		return nil, fmt.Errorf("stats.loglik: vector length mismatch")
	}
	out := make([]float64, length)
	for i := range out {
		out[i] = eval(observed.valueAt(i) - predicted.valueAt(i))
	}
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out))}, nil
}

func statsNormalPDFEvaluator(mean, stddev float64, logPDF bool) func(float64) float64 {
	logNorm := -math.Log(stddev) - 0.5*math.Log(2*math.Pi)
	return func(x float64) float64 {
		z := (x - mean) / stddev
		logDensity := logNorm - 0.5*z*z
		if logPDF {
			return logDensity
		}
		return math.Exp(logDensity)
	}
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
	offset, err := statsResampleOffset("stats.resample", args, 2)
	if err != nil {
		return nil, err
	}
	out, uniform, indexes := statsSystematicResampleValues(values, weights, total, offset)
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(out)), DenseArrayValue(NewDenseArrayF64Owned(uniform)), TableValue(indexes)}, nil
}

func statsResampleIf(args []Value) ([]Value, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("stats.resample_if: need values, weights, and minimum ESS ratio")
	}
	values, err := linalgVectorValue("stats.resample_if", args[0])
	if err != nil {
		return nil, err
	}
	weights, total, err := statsWeights("stats.resample_if", []Value{args[1]})
	if err != nil {
		return nil, err
	}
	if len(values) != len(weights) {
		return nil, fmt.Errorf("stats.resample_if: values and weights length mismatch")
	}
	minRatio, err := linalgNumber("stats.resample_if", args[2])
	if err != nil {
		return nil, err
	}
	if minRatio < 0 || minRatio > 1 {
		return nil, fmt.Errorf("stats.resample_if: minimum ESS ratio must be in [0, 1]")
	}
	offset, err := statsResampleOffset("stats.resample_if", args, 3)
	if err != nil {
		return nil, err
	}
	n := len(weights)
	ess := statsEffectiveSampleSizeOf(weights, total)
	if ess >= float64(n)*minRatio {
		return []Value{
			args[0],
			DenseArrayValue(NewDenseArrayF64Owned(statsNormalizeWeightsOf(weights, total))),
			BoolValue(false),
			FloatValue(ess),
			TableValue(NewAppendArrayTable(0)),
		}, nil
	}
	out, uniform, indexes := statsSystematicResampleValues(values, weights, total, offset)
	return []Value{
		DenseArrayValue(NewDenseArrayF64Owned(out)),
		DenseArrayValue(NewDenseArrayF64Owned(uniform)),
		BoolValue(true),
		FloatValue(ess),
		TableValue(indexes),
	}, nil
}

func statsImportanceUpdate(args []Value) ([]Value, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("stats.importance_update: need values, weights, log_likelihoods, and options")
	}
	values, err := linalgVectorValue("stats.importance_update", args[0])
	if err != nil {
		return nil, err
	}
	weights, total, err := statsWeights("stats.importance_update", []Value{args[1]})
	if err != nil {
		return nil, err
	}
	logLikelihoods, err := linalgVectorValue("stats.importance_update", args[2])
	if err != nil {
		return nil, err
	}
	if len(values) == 0 || len(values) != len(weights) || len(values) != len(logLikelihoods) {
		return nil, fmt.Errorf("stats.importance_update: values, weights, and log_likelihoods length mismatch")
	}
	if !args[3].IsTable() {
		return nil, fmt.Errorf("stats.importance_update: options must be a table")
	}
	opts := args[3].Table()
	minRatioValue := opts.RawGetString("min_ess_ratio")
	if minRatioValue.IsNil() {
		minRatioValue = opts.RawGetString("min_ratio")
	}
	if minRatioValue.IsNil() {
		minRatioValue = opts.RawGetString("minEssRatio")
	}
	if minRatioValue.IsNil() {
		return nil, fmt.Errorf("stats.importance_update: options.min_ess_ratio is required")
	}
	minRatio, err := linalgNumber("stats.importance_update", minRatioValue)
	if err != nil {
		return nil, err
	}
	if minRatio < 0 || minRatio > 1 {
		return nil, fmt.Errorf("stats.importance_update: minimum ESS ratio must be in [0, 1]")
	}
	offset := 0.5
	if offsetValue := opts.RawGetString("offset"); !offsetValue.IsNil() {
		offset, err = linalgNumber("stats.importance_update", offsetValue)
		if err != nil {
			return nil, err
		}
		if offset < 0 || offset >= 1 {
			return nil, fmt.Errorf("stats.importance_update: offset must be in [0, 1)")
		}
	}
	logWeights := make([]float64, len(weights))
	logTotalWeight := math.Log(total)
	for i, w := range weights {
		if w == 0 {
			logWeights[i] = math.Inf(-1)
		} else {
			logWeights[i] = math.Log(w) - logTotalWeight
		}
		logWeights[i] += logLikelihoods[i]
	}
	normalized, err := statsNormalizeLogWeights([]Value{DenseArrayValue(NewDenseArrayF64Owned(logWeights))})
	if err != nil {
		return nil, err
	}
	return statsResampleIf([]Value{args[0], normalized[0], FloatValue(minRatio), FloatValue(offset)})
}

func statsBayesUpdate(args []Value) ([]Value, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("stats.bayes_update: need values, weights, and log_likelihoods")
	}
	opts, err := statsBayesUpdateOptions(args)
	if err != nil {
		return nil, err
	}
	updated, err := statsImportanceUpdate([]Value{args[0], args[1], args[2], TableValue(opts)})
	if err != nil {
		return nil, err
	}
	summary, err := statsDescribe([]Value{updated[0], updated[1]})
	if err != nil {
		return nil, err
	}
	out := NewTable()
	out.RawSetString("kind", StringValue("weighted_samples"))
	out.RawSetString("values", updated[0])
	out.RawSetString("weights", updated[1])
	out.RawSetString("resampled", updated[2])
	out.RawSetString("ess", updated[3])
	out.RawSetString("indexes", updated[4])
	out.RawSetString("summary", summary[0])
	return []Value{TableValue(out)}, nil
}

type statsSampleSet struct {
	values  Value
	weights Value
}

func statsMakeSampleSet(fn string, valuesValue, weightsValue Value, extra *Table) ([]Value, error) {
	summary, err := statsDescribe([]Value{valuesValue, weightsValue})
	if err != nil {
		return nil, err
	}
	out := NewTable()
	out.RawSetString("kind", StringValue("weighted_samples"))
	out.RawSetString("values", valuesValue)
	out.RawSetString("weights", weightsValue)
	out.RawSetString("summary", summary[0])
	if extra != nil {
		for _, key := range extra.PairsKeysSnapshot() {
			out.RawSet(key, extra.RawGet(key))
		}
	}
	return []Value{TableValue(out)}, nil
}

func statsSampleSetFromValue(fn string, value Value) (statsSampleSet, error) {
	if !statsIsSampleSetValue(value) {
		return statsSampleSet{}, fmt.Errorf("%s: weighted sample set expected", fn)
	}
	tbl := value.Table()
	values := tbl.RawGetString("values")
	weights := tbl.RawGetString("weights")
	if values.IsNil() || weights.IsNil() {
		return statsSampleSet{}, fmt.Errorf("%s: weighted sample set missing values or weights", fn)
	}
	return statsSampleSet{values: values, weights: weights}, nil
}

func statsIsSampleSetValue(value Value) bool {
	return value.IsTable() &&
		value.Table().RawGetString("kind").IsString() &&
		value.Table().RawGetString("kind").Str() == "weighted_samples"
}

func statsBayesUpdateOptions(args []Value) (*Table, error) {
	opts := NewTable()
	if len(args) >= 4 {
		if !args[3].IsTable() {
			return nil, fmt.Errorf("stats.bayes_update: options must be a table")
		}
		src := args[3].Table()
		for _, key := range src.PairsKeysSnapshot() {
			opts.RawSet(key, src.RawGet(key))
		}
	}
	if opts.RawGetString("min_ess_ratio").IsNil() &&
		opts.RawGetString("min_ratio").IsNil() &&
		opts.RawGetString("minEssRatio").IsNil() {
		opts.RawSetString("min_ess_ratio", FloatValue(0))
	}
	if opts.RawGetString("offset").IsNil() {
		opts.RawSetString("offset", FloatValue(0.5))
	}
	return opts, nil
}

func statsSystematicResampleValues(values, weights []float64, total, offset float64) ([]float64, []float64, *Table) {
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
	return out, uniform, indexes
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
	return []Value{DenseArrayValue(NewDenseArrayF64Owned(statsNormalizeWeightsOf(weights, total)))}, nil
}

func statsNormalizeWeightsOf(weights []float64, total float64) []float64 {
	out := make([]float64, len(weights))
	for i, w := range weights {
		out[i] = w / total
	}
	return out
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
	return []Value{FloatValue(statsEffectiveSampleSizeOf(weights, total))}, nil
}

func statsEffectiveSampleSizeOf(weights []float64, total float64) float64 {
	sumSq := 0.0
	for _, w := range weights {
		normalized := w / total
		sumSq += normalized * normalized
	}
	if sumSq == 0 {
		return 0
	}
	return 1.0 / sumSq
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

func statsResampleOffset(name string, args []Value, index int) (float64, error) {
	offset := 0.5
	if len(args) <= index {
		return offset, nil
	}
	value, err := linalgNumber(name, args[index])
	if err != nil {
		return 0, err
	}
	if value < 0 || value >= 1 {
		return 0, fmt.Errorf("%s: offset must be in [0, 1)", name)
	}
	return value, nil
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
