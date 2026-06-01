package install

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/stdlibrt"
)

func TestInstallModulesRegistersMathFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("math"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered stdlibrt-owned math module: %v", got)
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
		t.Fatalf("InstallRuntimeStdlib registered stdlibrt-owned time module: %v", got)
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
		t.Fatalf("InstallRuntimeStdlib registered stdlibrt-owned process module: %v", got)
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
		t.Fatalf("InstallRuntimeStdlib registered stdlibrt-owned soa module: %v", got)
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

func TestInstallModulesRegistersSyncFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("sync"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered stdlibrt-owned sync module: %v", got)
	}

	Install(interp)
	syncLib := interp.GetGlobal("sync")
	if !syncLib.IsTable() {
		t.Fatalf("sync global is not a table: %v", syncLib)
	}
	if !syncLib.Table().RawGetString("__stdlibrt_module").Truthy() {
		t.Fatalf("sync global was not installed from stdlibrt")
	}
	for _, name := range []string{"waitgroup", "mutex", "rwmutex", "once", "group"} {
		if got := syncLib.Table().RawGetString(name); !got.IsFunction() {
			t.Fatalf("sync.%s is not a function: %v", name, got)
		}
	}
}

func TestInstallModulesRegistersStringFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("string"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered stdlibrt-owned string module: %v", got)
	}
	if got := interp.StringMeta(); got != nil {
		t.Fatalf("InstallRuntimeStdlib installed string metatable: %v", got)
	}

	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes, ModuleOptions{
		ScriptCaller: interp.CallFunction,
	})
	stringLib := interp.GetGlobal("string")
	if !stringLib.IsTable() {
		t.Fatalf("string global is not a table: %v", stringLib)
	}
	if got := interp.StringMeta(); got == nil {
		t.Fatal("string metatable was not bound after stdlibrt install")
	}
	upper := stringLib.Table().RawGetString("upper")
	if !upper.IsFunction() {
		t.Fatalf("string.upper is not a function: %v", upper)
	}
}

func TestInstallModulesRegistersTableFromStdlibrt(t *testing.T) {
	interp := runtime.NewCore()
	interp.InstallRuntimeStdlib()
	if got := interp.GetGlobal("table"); !got.IsNil() {
		t.Fatalf("InstallRuntimeStdlib registered stdlibrt-owned table module: %v", got)
	}

	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes, ModuleOptions{
		ScriptCaller: interp.CallFunction,
		Table: stdlibrt.TableOptions{
			Call: interp.CallFunction,
			Less: interp.ValueLessThan,
			Len:  interp.TableLen,
			Get:  interp.TableGet,
			Set:  interp.TableSet,
		},
	})
	tableLib := interp.GetGlobal("table")
	if !tableLib.IsTable() {
		t.Fatalf("table global is not a table: %v", tableLib)
	}
	for _, name := range []string{"sort", "insert", "remove", "unpack", "spread", "move", "concat", "map", "reduce"} {
		if got := tableLib.Table().RawGetString(name); !got.IsFunction() {
			t.Fatalf("table.%s is not a function: %v", name, got)
		}
	}
}
