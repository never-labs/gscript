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

func TestModuleOptionsForScriptLoadsLocalReplace(t *testing.T) {
	dir := t.TempDir()
	localLib := filepath.Join(dir, "local", "lib")
	if err := os.MkdirAll(localLib, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gscript.mod"), []byte(`module example.com/app
gs 0.1
require example.com/lib v0.1.0
replace example.com/lib => ./local/lib
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localLib, "foo.gs"), []byte(`return { value: 77 }`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.gs")
	if err := os.WriteFile(mainPath, []byte(`u := require("example.com/lib/foo"); result := u.value`), 0644); err != nil {
		t.Fatal(err)
	}

	for _, useVM := range []bool{false, true} {
		t.Run(map[bool]string{false: "tree", true: "bytecode"}[useVM], func(t *testing.T) {
			opts := ModuleOptionsForScript(mainPath)
			if useVM {
				opts = append(opts, WithVM())
			}
			vm := New(opts...)
			if err := vm.ExecFile(mainPath); err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("result")
			if err != nil {
				t.Fatal(err)
			}
			if got != int64(77) {
				t.Fatalf("result = %#v, want 77", got)
			}
		})
	}
}
