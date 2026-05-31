package runtime

import "testing"

func TestStdlibInstallerModuleAliasAndPackageLoadedSemantics(t *testing.T) {
	interp := &Interpreter{
		globals: NewEnvironment(nil),
		modules: make(map[string]Value),
	}
	installer := newStdlibInstallContext(interp)
	module := NewTable()
	moduleVal := TableValue(module)
	aliasVal := StringValue("alias-value")

	installer.RegisterModule("demo", moduleVal)
	installer.RegisterAlias("demoAlias", aliasVal)
	installer.InstallPackage("/scripts")

	if got := interp.GetGlobal("demo"); got != moduleVal {
		t.Fatalf("global demo = %v, want registered module", got)
	}
	if got := interp.modules["demo"]; got != moduleVal {
		t.Fatalf("require cache demo = %v, want registered module", got)
	}
	pkg := interp.GetGlobal("package")
	if !pkg.IsTable() {
		t.Fatal("package global is not a table")
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		t.Fatal("package.loaded is not a table")
	}
	if got := loaded.Table().RawGetString("demo"); got != moduleVal {
		t.Fatalf("package.loaded.demo = %v, want registered module", got)
	}
	if got := interp.GetGlobal("demoAlias"); got != aliasVal {
		t.Fatalf("global demoAlias = %v, want alias value", got)
	}
	if got := loaded.Table().RawGetString("demoAlias"); !got.IsNil() {
		t.Fatalf("package.loaded.demoAlias = %v, want nil", got)
	}
	if _, ok := interp.modules["demoAlias"]; ok {
		t.Fatal("alias was inserted into require cache")
	}
	if got := pkg.Table().RawGetString("path"); !got.IsString() || got.Str() != "/scripts" {
		t.Fatalf("package.path = %v, want /scripts", got)
	}
}

func TestStdlibInstallerDefaultModuleIdentity(t *testing.T) {
	interp := NewCore()
	interp.InstallStdlib()
	for _, name := range []string{"string", "llm"} {
		global := interp.GetGlobal(name)
		if !global.IsTable() {
			t.Fatalf("%s global is not a table", name)
		}
		if got := interp.packageLoaded(name); got != global {
			t.Fatalf("package.loaded.%s does not match global", name)
		}
		if got := interp.modules[name]; got != global {
			t.Fatalf("require cache %s does not match global", name)
		}
	}
	if got := interp.packageLoaded("toolof"); !got.IsNil() {
		t.Fatalf("package.loaded.toolof = %v, want nil", got)
	}
	if _, ok := interp.modules["toolof"]; ok {
		t.Fatal("toolof alias was inserted into require cache")
	}
}

func TestRuntimeNewIsCoreOnly(t *testing.T) {
	interp := New()
	for _, name := range []string{"script", "string", "table", "json", "llm", "fs"} {
		if got := interp.GetGlobal(name); !got.IsNil() {
			t.Fatalf("%s global = %v, want nil before explicit stdlib install", name, got)
		}
		if _, ok := interp.modules[name]; ok {
			t.Fatalf("%s was inserted into require cache before explicit stdlib install", name)
		}
	}
}

func TestExplicitInstallStdlibKeepsLegacyMigratedHostIOTables(t *testing.T) {
	interp := NewCore()
	interp.InstallStdlib()
	for _, name := range []string{"fs", "http", "io", "net", "os"} {
		global := interp.GetGlobal(name)
		if !global.IsTable() {
			t.Fatalf("%s global is not a table", name)
		}
		if global.Table().RawGetString("__stdlibrt_module").Truthy() {
			t.Fatalf("%s global was installed from stdlibrt; explicit runtime InstallStdlib must keep legacy direct builders", name)
		}
		if got := interp.packageLoaded(name); got != global {
			t.Fatalf("package.loaded.%s does not match global", name)
		}
		if got := interp.modules[name]; got != global {
			t.Fatalf("require cache %s does not match global", name)
		}
	}
}

func TestInstallRuntimeStdlibOmitsMigratedHostIOTables(t *testing.T) {
	interp := NewCore()
	interp.InstallRuntimeStdlib()
	for _, name := range []string{"fs", "http", "io", "net", "os", "string", "table"} {
		if got := interp.GetGlobal(name); !got.IsNil() {
			t.Fatalf("%s global = %v, want nil before stdlibrt install", name, got)
		}
		if _, ok := interp.modules[name]; ok {
			t.Fatalf("%s was inserted into require cache before stdlibrt install", name)
		}
	}
}
