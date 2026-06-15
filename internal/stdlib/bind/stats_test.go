package bind

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func statsInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	return runWithLib(t, src, "stats", BuildStats())
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
}

func TestStatsSystematicResample(t *testing.T) {
	interp := statsInterp(t, `
indices := stats.systematic_resample({0.1, 0.2, 0.7}, 0.5)
resampled, uniform, resample_idx := stats.resample({10, 20, 30}, {0.1, 0.2, 0.7}, 0.5)
kept, kept_weights, kept_resampled, kept_ess, kept_idx := stats.resample_if({10, 20, 30}, {1, 1, 1}, 0.5, 0.5)
next, next_weights, did_resample, next_ess, next_idx := stats.resample_if({10, 20, 30}, {0.01, 0.01, 0.98}, 0.8, 0.5)
iw_keep, iw_keep_weights, iw_keep_resampled, iw_keep_ess, iw_keep_idx := stats.importance_update({10, 20, 30}, {1, 1, 1}, {0, 0, 0}, {min_ess_ratio: 0.5, offset: 0.5})
iw_next, iw_next_weights, iw_did_resample, iw_next_ess, iw_next_idx := stats.importance_update({10, 20, 30}, {0.2, 0.3, 0.5}, {-10, -10, 10}, {min_ess_ratio: 0.8, offset: 0.5})
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
	err = execSourceOnInterp(interp, `stats.importance_update({1}, {1}, {0}, {})`)
	if err == nil {
		t.Fatal("stats.importance_update missing min_ess_ratio succeeded, want error")
	}
}
