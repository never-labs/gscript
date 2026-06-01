package gscript

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModuleOptionsForScriptLoadsCollections(t *testing.T) {
	dir := t.TempDir()
	vendor := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(filepath.Join(vendor, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte("module example.com/app\ngs 0.1\ncollection vendor ./vendor\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendor, "pkg", "util.gs"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.gs")
	if err := os.WriteFile(mainPath, []byte(`u := require("vendor:pkg.util"); result := u.value`), 0644); err != nil {
		t.Fatal(err)
	}

	vm := New(append(ModuleOptionsForScript(mainPath), WithVM())...)
	if err := vm.ExecFile(mainPath); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("result = %#v, want 42", got)
	}
}
