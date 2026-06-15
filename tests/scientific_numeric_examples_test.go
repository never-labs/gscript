package tests_test

import (
	"os"
	"path/filepath"
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
			wantAPI: []string{"linalg.matrix", "linalg.row", "linalg.col", "linalg.solve", "linalg.matmul", "linalg.trace", "linalg.get", "stats.rms", "q.eval"},
		},
		{
			rel:     filepath.Join("examples", "scientific", "particle_filter.leia"),
			summary: "ok particle ",
			wantAPI: []string{"rand.seed", "rand.normal_vec", "stats.normal_pdf", "stats.normalize_weights", "stats.effective_sample_size", "stats.resample", "stats.mean", "stats[\"var\"]", "stats.rmse", "q.eval"},
		},
		{
			rel:     filepath.Join("examples", "scientific", "inverted_pendulum.leia"),
			summary: "ok pendulum ",
			wantAPI: []string{"linalg.matrix", "control.lqr2", "control.feedback", "ode.rk4", "stats.mean", "stats.max", "q.eval"},
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
	}{
		{
			name:    "kalman",
			rel:     filepath.Join("examples", "scientific", "kalman_filter.leia"),
			summary: "ok kalman ",
		},
		{
			name:    "particle",
			rel:     filepath.Join("examples", "scientific", "particle_filter.leia"),
			summary: "ok particle ",
		},
		{
			name:    "pendulum",
			rel:     filepath.Join("examples", "scientific", "inverted_pendulum.leia"),
			summary: "ok pendulum ",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out := runCommand(t, root, 20*time.Second, "go", "run", "./cmd/leia", "run", tc.rel)
			if !strings.Contains(out, tc.summary) {
				t.Fatalf("%s output = %q, want containing %q", tc.rel, out, tc.summary)
			}
		})
	}
}
