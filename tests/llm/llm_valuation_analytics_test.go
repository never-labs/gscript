package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestFinRobotValuationAnalyticsDeterministicFixtures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		symbol string
		want   string
		opts   []leia.Option
	}{
		{
			name:   "valuation-analytics-interpreter",
			source: "valuation_analytics.leia",
			symbol: "valuation_analytics_summary",
			want:   "valuation_analytics methods=3 dcf=124.59 ev_ebitda=172.55 pe=188.15 target=151.69 upside=-0.2261",
		},
		{
			name:   "valuation-analytics-bytecode",
			source: "valuation_analytics.leia",
			symbol: "valuation_analytics_summary",
			want:   "valuation_analytics methods=3 dcf=124.59 ev_ebitda=172.55 pe=188.15 target=151.69 upside=-0.2261",
			opts:   []leia.Option{leia.WithVM()},
		},
		{
			name:   "sensitivity-math-interpreter",
			source: "sensitivity_math.leia",
			symbol: "sensitivity_math_summary",
			want:   "sensitivity_math matrix=3x3 base=124.59 best=166.33 worst=102.49 gross=0.9034 return=0.0285 tolerance=0.000000",
		},
		{
			name:   "sensitivity-math-bytecode",
			source: "sensitivity_math.leia",
			symbol: "sensitivity_math_summary",
			want:   "sensitivity_math matrix=3x3 base=124.59 best=166.33 worst=102.49 gross=0.9034 return=0.0285 tolerance=0.000000",
			opts:   []leia.Option{leia.WithVM()},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prints []string
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibMath | leia.LibMatrix | leia.LibString),
				leia.WithPrint(func(args ...any) {
					var parts []string
					for _, arg := range args {
						parts = append(parts, fmt.Sprint(arg))
					}
					prints = append(prints, strings.Join(parts, " "))
				}),
			}, tc.opts...)...)

			path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", tc.source)
			if err := vm.ExecFile(path); err != nil {
				t.Fatalf("ExecFile %s: %v", tc.source, err)
			}
			got, err := vm.Get(tc.symbol)
			if err != nil {
				t.Fatalf("Get %s: %v", tc.symbol, err)
			}
			if got != tc.want {
				t.Fatalf("%s = %#v, want %#v", tc.symbol, got, tc.want)
			}
			if len(prints) != 1 || prints[0] != tc.want {
				t.Fatalf("prints = %#v, want %q", prints, tc.want)
			}
		})
	}
}

func TestFinRobotValuationAnalyticsDocumentsGapCoverage(t *testing.T) {
	root := repoRoot(t)
	fixtures := map[string][]string{
		"valuation_analytics.leia": {
			"FR-GAP-015",
			"DCF",
			"EV/EBITDA",
			"P/E",
			"target price",
			"built_in_finance: false",
			"deterministic_fixture: true",
		},
		"sensitivity_math.leia": {
			"FR-GAP-009/015",
			"matrix.dense",
			"sensitivity matrix",
			"optimization_tolerance",
			"optimized_weights",
		},
	}

	for name, snippets := range fixtures {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "examples", "ai", "finrobot_translation", name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			source := string(data)
			for _, snippet := range snippets {
				if !strings.Contains(source, snippet) {
					t.Fatalf("%s missing gap evidence %q", name, snippet)
				}
			}
		})
	}
}
