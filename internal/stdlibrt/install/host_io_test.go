package install

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestInstallUsesStdlibrtHostIOModules(t *testing.T) {
	interp := runtime.NewCore()
	Install(interp)

	for _, name := range []string{"fs", "http", "io", "net", "os"} {
		global := interp.GetGlobal(name)
		if !global.IsTable() {
			t.Fatalf("%s global is not a table", name)
		}
		if !global.Table().RawGetString("__stdlibrt_module").Truthy() {
			t.Fatalf("%s global is not marked as a stdlibrt module", name)
		}
		pkg := interp.GetGlobal("package")
		if !pkg.IsTable() {
			t.Fatal("package global is not a table")
		}
		loaded := pkg.Table().RawGetString("loaded")
		if !loaded.IsTable() {
			t.Fatal("package.loaded is not a table")
		}
		if got := loaded.Table().RawGetString(name); got != global {
			t.Fatalf("package.loaded.%s does not match global", name)
		}
	}
}
