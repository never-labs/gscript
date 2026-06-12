package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestFinRobotAPIReplayExampleCoversOfflineSubstrate(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prints []string
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibJSON | leia.LibLLM),
				leia.WithPrint(func(args ...any) {
					var parts []string
					for _, arg := range args {
						parts = append(parts, fmt.Sprint(arg))
					}
					prints = append(prints, strings.Join(parts, " "))
				}),
			}, tc.opts...)...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "api_replay.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			got, err := vm.Get("api_replay_summary")
			if err != nil {
				t.Fatalf("Get api_replay_summary: %v", err)
			}
			want := "api_replay rows=2 pages=2 first_attempt=2 auth_error=auth artifact=artifact-sec-acme-10k-2025-html redirect=mock://sec/acme/2025/10-k.html"
			if got != want {
				t.Fatalf("api_replay_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestFinRobotAPIReplayExampleDocumentsFRGAP006007Terms(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "api_replay.leia")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	source := string(data)
	for _, snippet := range []string{
		"auth headers",
		"typed JSON decode",
		"retry/pagination/rate-limit/error metadata",
		"download artifacts",
		"Authorization: \"Bearer ${FMP_API_KEY}\"",
		"secret_ref: \"env:FMP_API_KEY\"",
		"typed_as: \"MetricRow[]\"",
		"schema_mismatch",
		"Retry-After",
		"pagination:",
		"rate_limit:",
		"error: {kind: \"auth\"",
		"redirects:",
		"artifact:",
		"terms:",
		"live_network: false",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("api_replay.leia missing substrate evidence %q", snippet)
		}
	}
}
