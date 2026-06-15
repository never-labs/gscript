package tests_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestScientificNumericExamplesSourceContract(t *testing.T) {
	root := findRepoRoot(t)
	cases := []struct {
		rel     string
		summary string
		wantAPI []string
	}{
		{
			rel:     filepath.Join("examples", "scientific", "kalman_filter.leia"),
			summary: "ok kalman ",
			wantAPI: []string{"linalg.matrix", "linalg.row", "linalg.col", "linalg.sandwich", "linalg.chainmul", "linalg.solve_right", "linalg.scalar", "linalg.matmul", "linalg.trace", "linalg.at", "stats.rms", "math.near", "q {"},
		},
		{
			rel:     filepath.Join("examples", "scientific", "particle_filter.leia"),
			summary: "ok particle ",
			wantAPI: []string{"rand.seed", "rand.normal_vec", "linalg.add", "stats.uniform_weights", "stats.log_normal_pdf", "stats.normalize_log_weights", "stats.resample_if", "stats.mean", "stats.variance", "stats.std", "math.near", "q {"},
		},
		{
			rel:     filepath.Join("examples", "scientific", "inverted_pendulum.leia"),
			summary: "ok pendulum ",
			wantAPI: []string{"linalg.matrix", "control.lqr", "control.feedback", "ode.solve", "stats.mean", "stats.max", "math.near", "q {"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(root, tc.rel))
			if err != nil {
				t.Fatalf("read %s: %v", tc.rel, err)
			}
			text := string(src)
			if !strings.Contains(text, tc.summary) {
				t.Fatalf("%s missing stable summary prefix %q", tc.rel, tc.summary)
			}
			for _, api := range tc.wantAPI {
				if !strings.Contains(text, api) {
					t.Fatalf("%s missing expected general API usage %q", tc.rel, api)
				}
			}
			for _, forbidden := range []string{"native_kalman", "native_particle", "native_pendulum", "scientific_native", "example_native"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s uses forbidden example-specific native helper marker %q", tc.rel, forbidden)
				}
			}
		})
	}
}

func TestScientificNumericExamplesRun(t *testing.T) {
	root := findRepoRoot(t)
	cases := []struct {
		name    string
		rel     string
		summary string
		check   func(t *testing.T, values map[string]float64)
	}{
		{
			name:    "kalman",
			rel:     filepath.Join("examples", "scientific", "kalman_filter.leia"),
			summary: "ok kalman ",
			check: func(t *testing.T, values map[string]float64) {
				requireSummaryNear(t, values, "n", 5, 0)
				requireSummaryNear(t, values, "position", 5.0, 0.15)
				requireSummaryNear(t, values, "velocity", 1.0, 0.15)
				requireSummaryBelow(t, values, "rmse", 0.20)
				requireSummaryBetween(t, values, "trace", 0.0, 0.20)
			},
		},
		{
			name:    "particle",
			rel:     filepath.Join("examples", "scientific", "particle_filter.leia"),
			summary: "ok particle ",
			check: func(t *testing.T, values map[string]float64) {
				requireSummaryNear(t, values, "n", 64, 0)
				measurement := requireSummaryValue(t, values, "measurement")
				requireSummaryNear(t, values, "estimate", measurement, 0.15)
				requireSummaryBelow(t, values, "spread", 0.35)
			},
		},
		{
			name:    "pendulum",
			rel:     filepath.Join("examples", "scientific", "inverted_pendulum.leia"),
			summary: "ok pendulum ",
			check: func(t *testing.T, values map[string]float64) {
				requireSummaryNear(t, values, "steps", 240, 0)
				requireSummaryNear(t, values, "theta", 0.0, 0.04)
				requireSummaryNear(t, values, "omega", 0.0, 0.08)
				requireSummaryBelow(t, values, "peak_energy", 0.25)
			},
		},
	}
	modes := []struct {
		name  string
		flags []string
	}{
		{name: "default"},
		{name: "vm", flags: []string{"--vm"}},
	}
	for _, tc := range cases {
		tc := tc
		for _, mode := range modes {
			mode := mode
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				args := append([]string{"run", "./cmd/leia", "run"}, mode.flags...)
				args = append(args, tc.rel)
				out := runCommand(t, root, 20*time.Second, "go", args...)
				if !strings.Contains(out, tc.summary) {
					t.Fatalf("%s output = %q, want containing %q", tc.rel, out, tc.summary)
				}
				tc.check(t, parseScientificSummary(t, out))
			})
		}
	}
}

var scientificSummaryKV = regexp.MustCompile(`([a-z_]+)=([-+]?[0-9]+(?:\.[0-9]+)?)`)

func parseScientificSummary(t *testing.T, out string) map[string]float64 {
	t.Helper()
	values := map[string]float64{}
	for _, match := range scientificSummaryKV.FindAllStringSubmatch(out, -1) {
		value, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			t.Fatalf("parse summary value %q in output %q: %v", match[2], out, err)
		}
		values[match[1]] = value
	}
	if len(values) == 0 {
		t.Fatalf("no key=value summary fields found in output %q", out)
	}
	return values
}

func requireSummaryNear(t *testing.T, values map[string]float64, key string, want, tol float64) {
	t.Helper()
	got := requireSummaryValue(t, values, key)
	delta := got - want
	if delta < 0 {
		delta = -delta
	}
	if delta > tol {
		t.Fatalf("summary %s = %.12g, want %.12g +/- %.12g", key, got, want, tol)
	}
}

func requireSummaryBelow(t *testing.T, values map[string]float64, key string, limit float64) {
	t.Helper()
	got := requireSummaryValue(t, values, key)
	if got >= limit {
		t.Fatalf("summary %s = %.12g, want < %.12g", key, got, limit)
	}
}

func requireSummaryBetween(t *testing.T, values map[string]float64, key string, low, high float64) {
	t.Helper()
	got := requireSummaryValue(t, values, key)
	if got < low || got > high {
		t.Fatalf("summary %s = %.12g, want in [%.12g, %.12g]", key, got, low, high)
	}
}

func requireSummaryValue(t *testing.T, values map[string]float64, key string) float64 {
	t.Helper()
	got, ok := values[key]
	if !ok {
		t.Fatalf("summary missing %q in %#v", key, values)
	}
	return got
}
