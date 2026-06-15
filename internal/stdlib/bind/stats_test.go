package bind

import (
	"math"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func statsInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	return runWithLib(t, src, "stats", BuildStats())
}

func statsLinalgInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := New()
	installTestModule(interp, "stats", TableValue(BuildStats()))
	installTestModule(interp, "linalg", TableValue(BuildLinalg()))
	execOnInterp(t, interp, src)
	return interp
}

func assertNear(t *testing.T, got Value, want, tol float64) {
	t.Helper()
	if !got.IsNumber() {
		t.Fatalf("got %s, want number near %.12f", got.TypeName(), want)
	}
	if math.Abs(got.Number()-want) > tol {
		t.Fatalf("got %.12f, want %.12f +/- %.12f", got.Number(), want, tol)
	}
}

func TestStatsAggregatesAndTransforms(t *testing.T) {
	interp := statsInterp(t, `
total := stats.sum({1, 2, 3, 4})
mean := stats.mean({1, 2, 3, 4})
minimum := stats.min({4, 2, 8})
maximum := stats.max({4, 2, 8})
variance := stats["var"]({1, 2, 3, 4})
variance_alias := stats.variance({1, 2, 3, 4})
stddev := stats.std({1, 2, 3, 4})
normalized := stats.normalize({1, 2, 3})
zscore := stats.zscore({1, 2, 3})
flat := stats.normalize({5, 5})
weighted := stats.weighted_mean({10, 20, 30}, {1, 2, 1})
weighted_var := stats.weighted_var({10, 20, 30}, {1, 2, 1})
weighted_std := stats.weighted_std({10, 20, 30}, {1, 2, 1})
normalized_weights := stats.normalize_weights({2, 3, 5})
uniform_weights := stats.uniform_weights(4)
logsum := stats.logsumexp({0, 0})
log_pdf0 := stats.log_normal_pdf(0, 0, 1)
log_pdfs := stats.log_normal_pdf({0, 1}, 0, 1)
normalized_log_weights := stats.normalize_log_weights({-1000, -1000})
ess_uniform := stats.effective_sample_size({0.25, 0.25, 0.25, 0.25})
ess_raw := stats.effective_sample_size({2, 2, 2, 2})
cumsum := stats.cumsum({2, 3, 5})
diff := stats.diff({2, 5, 11})
filled := stats.fill(4, 0.25)
gathered := stats.gather({10, 20, 30}, {3, 1, 3})
pdf0 := stats.normal_pdf(0, 0, 1)
pdfs := stats.normal_pdf({0, 1}, 0, 1)
rms := stats.rms({3, 4})
rmse := stats.rmse({1, 2, 3}, {1, 4, 3})
desc := stats.describe({1, 2, 3, 4})
weighted_desc := stats.describe({10, 20, 30}, {1, 2, 1})
weighted_sparse := stats.describe({-100, 1, 2, 100}, {0, 1, 1, 0})
fields := {}
fields.x = stats.cumsum({1, 1, 1})
fields.y = stats.cumsum({10, 10, 10})
field_desc := stats.describe_fields(fields)
weighted_fields := {}
weighted_fields.x = stats.cumsum({10, 10, 10})
weighted_fields.y = stats.cumsum({1, 1, 1})
weighted_field_desc := stats.describe_fields(weighted_fields, {1, 2, 1})
`)
	assertFloat(t, interp.GetGlobal("total"), 10)
	assertFloat(t, interp.GetGlobal("mean"), 2.5)
	assertFloat(t, interp.GetGlobal("minimum"), 2)
	assertFloat(t, interp.GetGlobal("maximum"), 8)
	assertFloat(t, interp.GetGlobal("variance"), 1.25)
	assertFloat(t, interp.GetGlobal("variance_alias"), 1.25)
	assertFloat(t, interp.GetGlobal("stddev"), 1.118033988749895)
	assertTableFloat(t, interp.GetGlobal("normalized"), 2, 0)
	assertTableFloat(t, interp.GetGlobal("zscore"), 3, 1.224744871391589)
	assertTableFloat(t, interp.GetGlobal("flat"), 1, 0)
	assertFloat(t, interp.GetGlobal("weighted"), 20)
	assertFloat(t, interp.GetGlobal("weighted_var"), 50)
	assertFloat(t, interp.GetGlobal("weighted_std"), 7.0710678118654755)
	assertTableFloat(t, interp.GetGlobal("normalized_weights"), 2, 0.3)
	assertTableFloat(t, interp.GetGlobal("uniform_weights"), 4, 0.25)
	assertFloat(t, interp.GetGlobal("logsum"), 0.6931471805599453)
	assertFloat(t, interp.GetGlobal("log_pdf0"), -0.9189385332046727)
	assertTableFloat(t, interp.GetGlobal("log_pdfs"), 2, -1.4189385332046727)
	assertTableFloat(t, interp.GetGlobal("normalized_log_weights"), 1, 0.5)
	assertFloat(t, interp.GetGlobal("ess_uniform"), 4)
	assertFloat(t, interp.GetGlobal("ess_raw"), 4)
	assertTableFloat(t, interp.GetGlobal("cumsum"), 3, 10)
	assertTableFloat(t, interp.GetGlobal("diff"), 2, 6)
	assertTableFloat(t, interp.GetGlobal("filled"), 4, 0.25)
	assertTableFloat(t, interp.GetGlobal("gathered"), 2, 10)
	assertFloat(t, interp.GetGlobal("pdf0"), 0.3989422804014327)
	assertTableFloat(t, interp.GetGlobal("pdfs"), 2, 0.24197072451914337)
	assertFloat(t, interp.GetGlobal("rms"), 3.5355339059327378)
	assertFloat(t, interp.GetGlobal("rmse"), 1.1547005383792515)
	desc := interp.GetGlobal("desc").Table()
	assertFloat(t, desc.RawGetString("count"), 4)
	assertFloat(t, desc.RawGetString("sum"), 10)
	assertFloat(t, desc.RawGetString("mean"), 2.5)
	assertFloat(t, desc.RawGetString("variance"), 1.25)
	assertFloat(t, desc.RawGetString("var"), 1.25)
	assertFloat(t, desc.RawGetString("std"), 1.118033988749895)
	assertFloat(t, desc.RawGetString("min"), 1)
	assertFloat(t, desc.RawGetString("max"), 4)
	assertFloat(t, desc.RawGetString("rms"), 2.7386127875258306)
	weightedDesc := interp.GetGlobal("weighted_desc").Table()
	assertFloat(t, weightedDesc.RawGetString("count"), 3)
	assertFloat(t, weightedDesc.RawGetString("weight_sum"), 4)
	assertFloat(t, weightedDesc.RawGetString("sum"), 80)
	assertFloat(t, weightedDesc.RawGetString("weighted_sum"), 80)
	assertFloat(t, weightedDesc.RawGetString("mean"), 20)
	assertFloat(t, weightedDesc.RawGetString("variance"), 50)
	assertFloat(t, weightedDesc.RawGetString("var"), 50)
	assertFloat(t, weightedDesc.RawGetString("std"), 7.0710678118654755)
	assertFloat(t, weightedDesc.RawGetString("min"), 10)
	assertFloat(t, weightedDesc.RawGetString("max"), 30)
	assertFloat(t, weightedDesc.RawGetString("rms"), 21.213203435596427)
	weightedSparse := interp.GetGlobal("weighted_sparse").Table()
	assertFloat(t, weightedSparse.RawGetString("mean"), 1.5)
	assertFloat(t, weightedSparse.RawGetString("min"), 1)
	assertFloat(t, weightedSparse.RawGetString("max"), 2)
	fieldDesc := interp.GetGlobal("field_desc").Table()
	assertFloat(t, fieldDesc.RawGetString("x").Table().RawGetString("mean"), 2)
	assertFloat(t, fieldDesc.RawGetString("y").Table().RawGetString("max"), 30)
	weightedFieldDesc := interp.GetGlobal("weighted_field_desc").Table()
	assertFloat(t, weightedFieldDesc.RawGetString("x").Table().RawGetString("mean"), 20)
	assertFloat(t, weightedFieldDesc.RawGetString("y").Table().RawGetString("variance"), 0.5)
}

func TestStatsDistributionFacadeNormalPDF(t *testing.T) {
	interp := statsInterp(t, `
dist := stats.normal(0, 1)
dist_kind := dist.kind
dist_name := dist.name
dist_mean := dist.mean
dist_sigma := dist.sigma
pdf0 := stats.pdf(dist, 0)
pdfs := stats.pdf(dist, {0, 1})
log_pdf0 := stats.logpdf(dist, 0)
log_pdfs := stats.logpdf(dist, {0, 1})
loglik_scalar := stats.loglik(dist, 2, 2)
loglik_vector := stats.loglik(dist, 2, {1, 2})
loglik_broadcast := stats.loglik(dist, {1, 2}, 2)
sample_loglik := stats.loglik(dist, 2, stats.samples({1, 2}))
observed_samples := stats.observe(stats.samples({1, 2}, {1, 1}), dist, 2, {min_ess_ratio: 0.0})
manual_samples := stats.update(stats.samples({1, 2}, {1, 1}), sample_loglik, {min_ess_ratio: 0.0})
observed_resample := stats.observe(stats.samples({10, 20, 30}, {0.2, 0.3, 0.5}), dist, 30, {min_ess_ratio: 0.8, offset: 0.5})
manual_resample := stats.update(stats.samples({10, 20, 30}, {0.2, 0.3, 0.5}), stats.loglik(dist, 30, stats.samples({10, 20, 30}, {0.2, 0.3, 0.5})), {min_ess_ratio: 0.8, offset: 0.5})
`)
	if got := interp.GetGlobal("dist_kind"); !got.IsString() || got.Str() != "distribution" {
		t.Fatalf("dist.kind = %v, want distribution", got)
	}
	if got := interp.GetGlobal("dist_name"); !got.IsString() || got.Str() != "normal" {
		t.Fatalf("dist.name = %v, want normal", got)
	}
	assertFloat(t, interp.GetGlobal("dist_mean"), 0)
	assertFloat(t, interp.GetGlobal("dist_sigma"), 1)
	assertFloat(t, interp.GetGlobal("pdf0"), 0.3989422804014327)
	assertTableFloat(t, interp.GetGlobal("pdfs"), 2, 0.24197072451914337)
	assertFloat(t, interp.GetGlobal("log_pdf0"), -0.9189385332046727)
	assertTableFloat(t, interp.GetGlobal("log_pdfs"), 2, -1.4189385332046727)
	assertFloat(t, interp.GetGlobal("loglik_scalar"), -0.9189385332046727)
	assertTableFloat(t, interp.GetGlobal("loglik_vector"), 1, -1.4189385332046727)
	assertTableFloat(t, interp.GetGlobal("loglik_broadcast"), 1, -1.4189385332046727)
	assertTableFloat(t, interp.GetGlobal("sample_loglik"), 1, -1.4189385332046727)
	observedSamples := interp.GetGlobal("observed_samples").Table()
	if got := observedSamples.RawGetString("kind"); !got.IsString() || got.Str() != "weighted_samples" {
		t.Fatalf("observed_samples.kind = %v, want weighted_samples", got)
	}
	assertTableFloat(t, observedSamples.RawGetString("weights"), 2, 0.6224593312018546)
	assertTableFloat(t, observedSamples.RawGetString("values"), 2, 2)
	assertFloat(t, observedSamples.RawGetString("summary").Table().RawGetString("mean"), 1.6224593312018545)
	assertTableFloat(t, interp.GetGlobal("manual_samples").Table().RawGetString("weights"), 2, 0.6224593312018546)
	assertTableFloat(t, interp.GetGlobal("manual_samples").Table().RawGetString("values"), 2, 2)
	assertFloat(t, interp.GetGlobal("manual_samples").Table().RawGetString("summary").Table().RawGetString("mean"), 1.6224593312018545)
	observedResample := interp.GetGlobal("observed_resample").Table()
	manualResample := interp.GetGlobal("manual_resample").Table()
	if got := observedResample.RawGetString("resampled"); !got.IsBool() || !got.Bool() {
		t.Fatalf("observed_resample.resampled = %v, want true", got)
	}
	if got := manualResample.RawGetString("resampled"); !got.IsBool() || !got.Bool() {
		t.Fatalf("manual_resample.resampled = %v, want true", got)
	}
	assertTableFloat(t, observedResample.RawGetString("values"), 1, 30)
	assertTableFloat(t, manualResample.RawGetString("values"), 1, 30)
	assertTableFloat(t, observedResample.RawGetString("weights"), 3, 1.0/3.0)
	assertTableFloat(t, manualResample.RawGetString("weights"), 3, 1.0/3.0)
	assertFloat(t, observedResample.RawGetString("summary").Table().RawGetString("mean"), 30)
	assertFloat(t, manualResample.RawGetString("summary").Table().RawGetString("mean"), 30)
	if got := observedResample.RawGetString("indexes").Table().Length(); got != manualResample.RawGetString("indexes").Table().Length() {
		t.Fatalf("resample index lengths differ: %d vs %d", got, manualResample.RawGetString("indexes").Table().Length())
	}
}

func TestStatsLinearGaussianStateSpace(t *testing.T) {
	interp := statsLinalgInterp(t, `
dt := 1.0
F := linalg.matrix({{1.0, dt}, {0.0, 1.0}})
H := linalg.row(1.0, 0.0)
Q := linalg.eye(2, 0.01)
state := stats.gaussian_state(linalg.vector(0.0, 1.0), linalg.eye(2))
measurements := {0.95, 2.05, 2.95, 4.10, 5.00}
innovations := {}
for i := 1; i <= #measurements; i++ {
    state = stats.linear_predict(state, F, Q)
    state = stats.linear_update(state, H, measurements[i], 0.04)
    innovations[i] = state.innovation[1]
}
position := linalg.at(state.x, 1)
velocity := linalg.at(state.x, 2)
trace := linalg.trace(state.P)
rmse := stats.rms(innovations)
row_state := stats.gaussian_state(linalg.row(1.0, 2.0), linalg.eye(2))
col_state := stats.gaussian_state(linalg.matrix({{1.0}, {2.0}}), linalg.eye(2))
handmade := {kind: "gaussian_state", x: linalg.row(0.0, 1.0), P: linalg.eye(2)}
handmade_next := stats.linear_predict(handmade, F, Q)
handmade_velocity := linalg.at(handmade_next.x, 2)
`)
	assertNear(t, interp.GetGlobal("position"), 5.0, 0.05)
	assertNear(t, interp.GetGlobal("velocity"), 1.0, 0.05)
	assertNear(t, interp.GetGlobal("handmade_velocity"), 1.0, 0.10)
	if got := interp.GetGlobal("trace").Number(); got <= 0 || got >= 0.08 {
		t.Fatalf("trace = %.12f, want in (0, 0.08)", got)
	}
	if got := interp.GetGlobal("rmse").Number(); got >= 0.20 {
		t.Fatalf("rmse = %.12f, want below 0.20", got)
	}
	assertTableFloat(t, interp.GetGlobal("row_state").Table().RawGetString("x"), 2, 2)
	assertTableFloat(t, interp.GetGlobal("col_state").Table().RawGetString("x"), 2, 2)
}

func TestStatsLinearGaussianRejectsShapeErrors(t *testing.T) {
	interp := statsLinalgInterp(t, ``)
	cases := []string{
		`stats.gaussian_state({1, 2}, linalg.matrix(1, 2, {1, 2}))`,
		`stats.linear_predict(stats.gaussian_state({1, 2}, linalg.eye(2)), linalg.matrix(1, 3, {1, 0, 0}), linalg.eye(1))`,
		`stats.linear_update(stats.gaussian_state({1, 2}, linalg.eye(2)), linalg.eye(2), {1, 2}, 0.1)`,
		`stats.linear_update(stats.gaussian_state({1, 2}, linalg.eye(2)), linalg.row(1, 0), {1, 2}, 0.1)`,
		`stats.linear_predict(stats.gaussian_state({1, 2}, linalg.eye(2)), linalg.eye(2), 0.01)`,
	}
	for _, src := range cases {
		if err := execSourceOnInterp(interp, src); err == nil {
			t.Fatalf("%s succeeded, want error", src)
		}
	}
}

func TestStatsSystematicResample(t *testing.T) {
	interp := statsInterp(t, `
indices := stats.systematic_resample({0.1, 0.2, 0.7}, 0.5)
resampled, uniform, resample_idx := stats.resample({10, 20, 30}, {0.1, 0.2, 0.7}, 0.5)
kept, kept_weights, kept_resampled, kept_ess, kept_idx := stats.resample_if({10, 20, 30}, {1, 1, 1}, 0.5, 0.5)
next, next_weights, did_resample, next_ess, next_idx := stats.resample_if({10, 20, 30}, {0.01, 0.01, 0.98}, 0.8, 0.5)
iw_keep, iw_keep_weights, iw_keep_resampled, iw_keep_ess, iw_keep_idx := stats.importance_update({10, 20, 30}, {1, 1, 1}, {0, 0, 0}, {min_ess_ratio: 0.5, offset: 0.5})
iw_next, iw_next_weights, iw_did_resample, iw_next_ess, iw_next_idx := stats.importance_update({10, 20, 30}, {0.2, 0.3, 0.5}, {-10, -10, 10}, {min_ess_ratio: 0.8, offset: 0.5})
bayes_keep := stats.bayes_update({10, 20, 30}, {1, 1, 1}, {0, 0, 0})
bayes_next := stats.bayes_update({10, 20, 30}, {0.2, 0.3, 0.5}, {-10, -10, 10}, {min_ess_ratio: 0.8, offset: 0.5})
samples := stats.samples({10, 20, 30}, {1, 2, 1})
sample_desc := stats.describe(samples)
sample_next := stats.update(samples, {-10, -10, 10}, {min_ess_ratio: 0.8, offset: 0.5})
`)
	indices := interp.GetGlobal("indices")
	if !indices.IsTable() {
		t.Fatalf("indices = %s, want table", indices.TypeName())
	}
	want := []int64{2, 3, 3}
	for i, w := range want {
		got := indices.Table().RawGetInt(int64(i + 1))
		if !got.IsInt() || got.Int() != w {
			t.Fatalf("indices[%d] = %v, want %d", i+1, got, w)
		}
	}
	assertTableFloat(t, interp.GetGlobal("resampled"), 1, 20)
	assertTableFloat(t, interp.GetGlobal("resampled"), 3, 30)
	assertTableFloat(t, interp.GetGlobal("uniform"), 2, 1.0/3.0)
	idx := interp.GetGlobal("resample_idx").Table()
	assertTableFloat(t, interp.GetGlobal("iw_keep"), 2, 20)
	assertTableFloat(t, interp.GetGlobal("iw_keep_weights"), 1, 1.0/3.0)
	if got := interp.GetGlobal("iw_keep_resampled"); !got.IsBool() || got.Bool() {
		t.Fatalf("iw_keep_resampled = %v, want false", got)
	}
	if got := interp.GetGlobal("iw_keep_idx").Table().Length(); got != 0 {
		t.Fatalf("iw_keep_idx length = %d, want 0", got)
	}
	assertTableFloat(t, interp.GetGlobal("iw_next"), 1, 30)
	assertTableFloat(t, interp.GetGlobal("iw_next_weights"), 1, 1.0/3.0)
	if got := interp.GetGlobal("iw_did_resample"); !got.IsBool() || !got.Bool() {
		t.Fatalf("iw_did_resample = %v, want true", got)
	}
	if got := interp.GetGlobal("iw_next_idx").Table().Length(); got != 3 {
		t.Fatalf("iw_next_idx length = %d, want 3", got)
	}
	bayesKeep := interp.GetGlobal("bayes_keep").Table()
	if got := bayesKeep.RawGetString("kind"); !got.IsString() || got.Str() != "weighted_samples" {
		t.Fatalf("bayes_keep.kind = %v, want weighted_samples", got)
	}
	assertTableFloat(t, bayesKeep.RawGetString("values"), 2, 20)
	assertTableFloat(t, bayesKeep.RawGetString("weights"), 1, 1.0/3.0)
	if got := bayesKeep.RawGetString("resampled"); !got.IsBool() || got.Bool() {
		t.Fatalf("bayes_keep.resampled = %v, want false", got)
	}
	assertFloat(t, bayesKeep.RawGetString("summary").Table().RawGetString("mean"), 20)
	bayesNext := interp.GetGlobal("bayes_next").Table()
	assertTableFloat(t, bayesNext.RawGetString("values"), 1, 30)
	assertTableFloat(t, bayesNext.RawGetString("weights"), 1, 1.0/3.0)
	if got := bayesNext.RawGetString("resampled"); !got.IsBool() || !got.Bool() {
		t.Fatalf("bayes_next.resampled = %v, want true", got)
	}
	if got := bayesNext.RawGetString("indexes").Table().Length(); got != 3 {
		t.Fatalf("bayes_next.indexes length = %d, want 3", got)
	}
	assertFloat(t, bayesNext.RawGetString("summary").Table().RawGetString("mean"), 30)
	samples := interp.GetGlobal("samples").Table()
	if got := samples.RawGetString("kind"); !got.IsString() || got.Str() != "weighted_samples" {
		t.Fatalf("samples.kind = %v, want weighted_samples", got)
	}
	assertTableFloat(t, samples.RawGetString("weights"), 2, 0.5)
	assertFloat(t, interp.GetGlobal("sample_desc").Table().RawGetString("mean"), 20)
	sampleNext := interp.GetGlobal("sample_next").Table()
	if got := sampleNext.RawGetString("kind"); !got.IsString() || got.Str() != "weighted_samples" {
		t.Fatalf("sample_next.kind = %v, want weighted_samples", got)
	}
	assertTableFloat(t, sampleNext.RawGetString("values"), 1, 30)
	assertFloat(t, sampleNext.RawGetString("summary").Table().RawGetString("mean"), 30)
	if got := idx.RawGetInt(1); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("resample_idx[1] = %v, want 2", got)
	}
	if interp.GetGlobal("kept_resampled").Truthy() {
		t.Fatalf("kept_resampled = %v, want false", interp.GetGlobal("kept_resampled"))
	}
	assertTableFloat(t, interp.GetGlobal("kept"), 2, 20)
	assertTableFloat(t, interp.GetGlobal("kept_weights"), 2, 1.0/3.0)
	assertFloat(t, interp.GetGlobal("kept_ess"), 3)
	if got := interp.GetGlobal("kept_idx").Table().Len(); got != 0 {
		t.Fatalf("kept_idx length = %d, want 0", got)
	}
	if !interp.GetGlobal("did_resample").Truthy() {
		t.Fatalf("did_resample = %v, want true", interp.GetGlobal("did_resample"))
	}
	assertTableFloat(t, interp.GetGlobal("next"), 1, 30)
	assertTableFloat(t, interp.GetGlobal("next_weights"), 2, 1.0/3.0)
	if got := interp.GetGlobal("next_ess").Number(); got >= 2 {
		t.Fatalf("next_ess = %v, want below 2", got)
	}
	nextIdx := interp.GetGlobal("next_idx").Table()
	if got := nextIdx.RawGetInt(1); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("next_idx[1] = %v, want 3", got)
	}
	if interp.GetGlobal("iw_keep_resampled").Truthy() {
		t.Fatalf("iw_keep_resampled = %v, want false", interp.GetGlobal("iw_keep_resampled"))
	}
	assertTableFloat(t, interp.GetGlobal("iw_keep"), 2, 20)
	assertTableFloat(t, interp.GetGlobal("iw_keep_weights"), 2, 1.0/3.0)
	assertFloat(t, interp.GetGlobal("iw_keep_ess"), 3)
	if got := interp.GetGlobal("iw_keep_idx").Table().Len(); got != 0 {
		t.Fatalf("iw_keep_idx length = %d, want 0", got)
	}
	if !interp.GetGlobal("iw_did_resample").Truthy() {
		t.Fatalf("iw_did_resample = %v, want true", interp.GetGlobal("iw_did_resample"))
	}
	assertTableFloat(t, interp.GetGlobal("iw_next"), 1, 30)
	assertTableFloat(t, interp.GetGlobal("iw_next_weights"), 3, 1.0/3.0)
	if got := interp.GetGlobal("iw_next_ess").Number(); got >= 2 {
		t.Fatalf("iw_next_ess = %v, want below 2", got)
	}
	if got := interp.GetGlobal("iw_next_idx").Table().RawGetInt(1); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("iw_next_idx[1] = %v, want 3", got)
	}
}

func TestStatsErrors(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "stats", runtime.TableValue(BuildStats()))
	err := execSourceOnInterp(interp, `stats.mean({})`)
	if err == nil {
		t.Fatal("stats.mean({}) succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.normalize_weights({0, 0})`)
	if err == nil {
		t.Fatal("stats.normalize_weights({0, 0}) succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.effective_sample_size({1, -1})`)
	if err == nil {
		t.Fatal("stats.effective_sample_size({1, -1}) succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.resample_if({1}, {1}, 1.5)`)
	if err == nil {
		t.Fatal("stats.resample_if invalid threshold succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.describe({})`)
	if err == nil {
		t.Fatal("stats.describe({}) succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.describe({1, 2}, {1})`)
	if err == nil {
		t.Fatal("stats.describe length mismatch succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.describe({1, 2}, {1, -1})`)
	if err == nil {
		t.Fatal("stats.describe negative weight succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.describe({1, 2}, {0, 0})`)
	if err == nil {
		t.Fatal("stats.describe zero total weight succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.describe_fields({})`)
	if err == nil {
		t.Fatal("stats.describe_fields empty table succeeded, want error")
	}
	err = execSourceOnInterp(interp, `fields := {}; fields.x = {"bad"}; stats.describe_fields(fields)`)
	if err == nil {
		t.Fatal("stats.describe_fields non-numeric field succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.importance_update({1}, {1}, {0}, {})`)
	if err == nil {
		t.Fatal("stats.importance_update missing min_ess_ratio succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.bayes_update({1}, {1})`)
	if err == nil {
		t.Fatal("stats.bayes_update missing log_likelihoods succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.bayes_update({1}, {1}, {0}, "bad")`)
	if err == nil {
		t.Fatal("stats.bayes_update bad options succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.observe()`)
	if err == nil {
		t.Fatal("stats.observe missing args succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.observe(stats.samples({1}), stats.normal(0, 1), 1, "bad")`)
	if err == nil {
		t.Fatal("stats.observe bad options succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.observe(stats.samples({1, 2}), stats.normal(0, 1), {1, 2, 3})`)
	if err == nil {
		t.Fatal("stats.observe length mismatch succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.observe(stats.samples({1, 2}), stats.normal(0, 1), {rows: 1, cols: 2, values: {1, 2}})`)
	if err == nil {
		t.Fatal("stats.observe matrix observed succeeded, want error")
	}
	err = execSourceOnInterp(interp, `stats.observe("bad-samples", "bad-dist", 1)`)
	if err == nil || !strings.Contains(err.Error(), "stats.loglik") {
		t.Fatalf("stats.observe invalid distribution/samples error = %v, want stats.loglik error", err)
	}
}
