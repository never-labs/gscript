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
mean := stats.mean({1, 2, 3, 4})
variance := stats["var"]({1, 2, 3, 4})
normalized := stats.normalize({1, 2, 3})
flat := stats.normalize({5, 5})
weighted := stats.weighted_mean({10, 20, 30}, {1, 2, 1})
cumsum := stats.cumsum({2, 3, 5})
`)
	assertFloat(t, interp.GetGlobal("mean"), 2.5)
	assertFloat(t, interp.GetGlobal("variance"), 1.25)
	assertTableFloat(t, interp.GetGlobal("normalized"), 2, 0)
	assertTableFloat(t, interp.GetGlobal("flat"), 1, 0)
	assertFloat(t, interp.GetGlobal("weighted"), 20)
	assertTableFloat(t, interp.GetGlobal("cumsum"), 3, 10)
}

func TestStatsSystematicResample(t *testing.T) {
	interp := statsInterp(t, `
indices := stats.systematic_resample({0.1, 0.2, 0.7}, 0.5)
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
}

func TestStatsErrors(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "stats", runtime.TableValue(BuildStats()))
	err := execSourceOnInterp(interp, `stats.mean({})`)
	if err == nil {
		t.Fatal("stats.mean({}) succeeded, want error")
	}
}
