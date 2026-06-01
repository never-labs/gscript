package vm

import (
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVMStdlibModuleIdentityAcrossGlobalPackageAndRequire(t *testing.T) {
	v := New(vmtest.NewInterpreterGlobals())
	defer v.Close()

	global := v.GetGlobal("json")
	if !global.IsTable() {
		t.Fatal("json global is not a table")
	}
	pkg := v.GetGlobal("package")
	if !pkg.IsTable() {
		t.Fatal("package global is not a table")
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		t.Fatal("package.loaded is not a table")
	}
	if got := loaded.Table().RawGetString("json"); got != global {
		t.Fatalf("package.loaded.json does not match json global")
	}
	results, err := v.callValue(v.GetGlobal("require"), []runtime.Value{runtime.StringValue("json")})
	if err != nil {
		t.Fatalf("require(json) failed: %v", err)
	}
	if len(results) != 1 || results[0] != global {
		t.Fatalf("require(json) = %v, want json global", results)
	}
	if got := loaded.Table().RawGetString("toolof"); !got.IsNil() {
		t.Fatalf("package.loaded.toolof = %v, want nil", got)
	}
}
