package install

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestInstallModulesRegistersMathFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("math"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered migrated math module: %v", got)
	}

	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes)
	mathLib := interp.GetGlobal("math")
	if !mathLib.IsTable() {
		t.Fatalf("math global is not a table: %v", mathLib)
	}
	floor := mathLib.Table().RawGetString("floor")
	if !floor.IsFunction() {
		t.Fatalf("math.floor is not a function: %v", floor)
	}
	gf := floor.GoFunction()
	if gf == nil || gf.FastArg1 == nil || gf.Fast1 == nil {
		t.Fatalf("math.floor missing fast paths after stdlibrt install: %#v", gf)
	}
}
