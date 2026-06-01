package install

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestInstallDebugAndTestkitFromStdlibInstall(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	for _, name := range []string{"debug", "testkit"} {
		if got := interp.GetGlobal(name); !got.IsNil() {
			t.Fatalf("InstallRuntimeStdlib registered stdlib-bound %s module: %v", name, got)
		}
	}

	InstallDebugAndTestkit(interp)

	for _, name := range []string{"debug", "testkit"} {
		global := interp.GetGlobal(name)
		if !global.IsTable() {
			t.Fatalf("%s global is not a table", name)
		}
	}
}

func TestPublicInstallRegistersDebugAndTestkit(t *testing.T) {
	interp := runtime.NewCore()
	Install(interp)

	for _, name := range []string{"debug", "testkit"} {
		global := interp.GetGlobal(name)
		if !global.IsTable() {
			t.Fatalf("%s global is not a table", name)
		}
	}
}
