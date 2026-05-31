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

func TestInstallModulesRegistersTimeFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("time"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered migrated time module: %v", got)
	}

	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes)
	timeLib := interp.GetGlobal("time")
	if !timeLib.IsTable() {
		t.Fatalf("time global is not a table: %v", timeLib)
	}
	for _, name := range []string{"sleep", "after", "now", "since", "unix"} {
		if got := timeLib.Table().RawGetString(name); !got.IsFunction() {
			t.Fatalf("time.%s is not a function: %v", name, got)
		}
	}
}

func TestInstallModulesRegistersProcessFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("process"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered migrated process module: %v", got)
	}

	Install(interp)
	processLib := interp.GetGlobal("process")
	if !processLib.IsTable() {
		t.Fatalf("process global is not a table: %v", processLib)
	}
	if !processLib.Table().RawGetString("__stdlibrt_module").Truthy() {
		t.Fatalf("process global was not installed from stdlibrt")
	}
	if run := processLib.Table().RawGetString("run"); !run.IsFunction() {
		t.Fatalf("process.run is not a function: %v", run)
	}
}

func TestInstallModulesRegistersSOAFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("soa"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered migrated soa module: %v", got)
	}

	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes)
	soaLib := interp.GetGlobal("soa")
	if !soaLib.IsTable() {
		t.Fatalf("soa global is not a table: %v", soaLib)
	}
	lenFn := soaLib.Table().RawGetString("len").GoFunction()
	if lenFn == nil || lenFn.FastArg1 == nil {
		t.Fatalf("soa.len missing fast path after stdlibrt install: %#v", lenFn)
	}
}
