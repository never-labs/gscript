package leia_test

import (
	"os"
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestFinRobotWebProductUISnapshotSmokeExample(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "web_product.leia"))
	if err != nil {
		t.Fatalf("ReadFile web_product.leia: %v", err)
	}
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString)}, mode.opts...)...)
			if err := vm.Exec(string(src)); err != nil {
				t.Fatalf("Exec web_product.leia: %v", err)
			}
			for name, want := range map[string]any{
				"route_parity_ok":            true,
				"auth_session_ok":            true,
				"background_log_ok":          true,
				"downloads_ok":               true,
				"crud_fixture_ok":            true,
				"snapshot_manifest_ok":       true,
				"accessibility_checklist_ok": true,
				"web_product_smoke_ok":       true,
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
