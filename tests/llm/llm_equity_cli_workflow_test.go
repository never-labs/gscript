package leia_test

import (
	"os"
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestFinRobotEquityCLIWorkflowStagesExample(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "equity_cli_workflow.leia"))
	if err != nil {
		t.Fatalf("ReadFile equity_cli_workflow.leia: %v", err)
	}
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM | leia.LibDialect)}, mode.opts...)...)
			if err := vm.Exec(string(src)); err != nil {
				t.Fatalf("Exec equity_cli_workflow.leia: %v", err)
			}
			for name, want := range map[string]any{
				"section_ok":  true,
				"chart_ok":    true,
				"source_ok":   true,
				"manifest_ok": true,
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
