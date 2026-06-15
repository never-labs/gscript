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
`)
	assertFloat(t, interp.GetGlobal("total"), 10)
	assertFloat(t, interp.GetGlobal("mean"), 2.5)
	assertFloat(t, interp.GetGlobal("minimum"), 2)
	assertFloat(t, interp.GetGlobal("maximum"), 8)
	assertFloat(t, interp.GetGlobal("variance"), 1.25)
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
}

func TestStatsSystematicResample(t *testing.T) {
	interp := statsInterp(t, `
indices := stats.systematic_resample({0.1, 0.2, 0.7}, 0.5)
resampled, uniform, resample_idx := stats.resample({10, 20, 30}, {0.1, 0.2, 0.7}, 0.5)
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
}
