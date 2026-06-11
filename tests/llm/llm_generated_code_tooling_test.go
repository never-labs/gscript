package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestFinRobotGeneratedCodeToolingExampleCoversFRGAP017ReplayEnvelope(t *testing.T) {
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
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithPrint(func(args ...any) {
					var parts []string
					for _, arg := range args {
						parts = append(parts, fmt.Sprint(arg))
					}
					prints = append(prints, strings.Join(parts, " "))
				}),
			}, tc.opts...)...)

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "generated_code_tooling.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}

			wantSummary := "generated_code_tooling read=generated-code.read write=recorded exec=python image=image/png denied=3 live_python=false live_notebook=false"
			gotSummary, err := vm.Get("generated_code_tooling_summary")
			if err != nil {
				t.Fatalf("Get generated_code_tooling_summary: %v", err)
			}
			if gotSummary != wantSummary {
				t.Fatalf("generated_code_tooling_summary = %#v, want %#v", gotSummary, wantSummary)
			}
			if len(prints) != 1 || prints[0] != wantSummary {
				t.Fatalf("prints = %#v, want %q", prints, wantSummary)
			}

			for name, want := range map[string]interface{}{
				"generated_code_tooling_read_capability":      "generated-code.read",
				"generated_code_tooling_write_status":         "recorded",
				"generated_code_tooling_execute_runtime":      "python",
				"generated_code_tooling_execute_stdout":       "signals: PDD=0.18 BABA=0.11",
				"generated_code_tooling_image_media_type":     "image/png",
				"generated_code_tooling_denied_count":         int64(3),
				"generated_code_tooling_default_denied_class": "generated-code",
				"generated_code_tooling_first_denied_command": "rm -rf /tmp/finrobot",
				"generated_code_tooling_first_denied_reason":  "shell command requires explicit generated-code approval",
				"generated_code_tooling_live_python":          false,
				"generated_code_tooling_live_notebook":        false,
				"generated_code_tooling_policy_ok":            true,
				"generated_code_tooling_policy_errors_nil":    true,
				"generated_code_tooling_replay_errors_nil":    true,
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

func TestFinRobotGeneratedCodeToolingExampleDocumentsDeniedCommandCases(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "generated_code_tooling.leia")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	source := string(data)
	for _, snippet := range []string{
		"FR-GAP-017 evidence",
		"generated_code_tooling.replay_envelope",
		"generated-code.read",
		"generated-code.write",
		"generated-code.execute",
		"generated-code.display-image",
		"live_python: false",
		"live_notebook: false",
		"live_shell: false",
		"rm -rf /tmp/finrobot",
		"pip install unknown-package",
		"jupyter nbconvert --execute strategy.ipynb",
		"llm.approval_trace",
		"status: \"denied\"",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("generated_code_tooling.leia missing FR-GAP-017 evidence %q", snippet)
		}
	}
}
