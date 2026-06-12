package leia_test

import (
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestFinRobotDataNormalizationFixture(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibTable | leia.LibLLM),
			}, tc.opts...)...)
			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "data_normalization.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			if err := vm.Exec(`
probe_normalized_count := #normalized_rows
probe_missing_flag := normalized_rows[2].is_missing
probe_adjusted_value := normalized_rows[2].adjusted_value
probe_join_category := joined_rows[3].category
probe_group_missing_count := entity_groups[1].missing_count
probe_window_first_period := alpha_q2_window[1].period
probe_rolling_avg := rolling_two_periods[4].rolling_avg
probe_provenance_ok := validation.provenance_ok
`); err != nil {
				t.Fatalf("Exec probes: %v", err)
			}

			assertVMValue(t, vm, "data_normalization_summary", "fr_gap_008 rows=4 missing=1 groups=2 rolling=4")

			for name, want := range map[string]any{
				"probe_normalized_count":    int64(4),
				"probe_missing_flag":        true,
				"probe_adjusted_value":      float64(0),
				"probe_join_category":       "treatment",
				"probe_group_missing_count": int64(1),
				"probe_window_first_period": "2026-Q1",
				"probe_rolling_avg":         float64(9),
				"probe_provenance_ok":       true,
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func assertVMValue(t *testing.T, vm *leia.VM, name string, want any) {
	t.Helper()
	got, err := vm.Get(name)
	if err != nil {
		t.Fatalf("Get %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}
