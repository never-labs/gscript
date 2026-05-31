//go:build !rl
// +build !rl

package install

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestInstallModulesRegistersRLFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("rl"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered stdlibrt-owned rl module: %v", got)
	}

	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes)
	rlLib := interp.GetGlobal("rl")
	if !rlLib.IsTable() {
		t.Fatalf("rl global is not a table: %v", rlLib)
	}
	if stub := rlLib.Table().RawGetString("_stub"); !stub.IsBool() || !stub.Bool() {
		t.Fatalf("rl._stub = %v, want true in default build", stub)
	}
}
