package leia

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/never-labs/leia/internal/modpkg"
)

func TestModuleOptionsForScriptLoadsCollections(t *testing.T) {
	dir := t.TempDir()
	vendor := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(filepath.Join(vendor, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leia.mod"), []byte("module example.com/app\nleia 0.1\ncollection vendor ./vendor\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendor, "pkg", "util.leia"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.leia")
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
	if err := os.WriteFile(filepath.Join(dir, "leia.mod"), []byte(`module example.com/app
leia 0.1
require example.com/lib v0.1.0
replace example.com/lib => ./local/lib
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localLib, "foo.leia"), []byte(`return { value: 77 }`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.leia")
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

func TestModuleOptionsForScriptLoadsDownloadedGitHubCache(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	t.Setenv("LEIA_CACHE", cache)
	if err := os.MkdirAll(filepath.Join(cache, "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg", "util.leia"), []byte(`return { value: 91 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leia.mod"), []byte(`module example.com/app
leia 0.1
require github.com/acme/toolkit v1.2.3
`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.leia")
	if err := os.WriteFile(mainPath, []byte(`u := require("github.com/acme/toolkit/pkg/util"); result := u.value`), 0644); err != nil {
		t.Fatal(err)
	}
	resolvedCache, err := modpkg.ModuleCacheDir("")
	if err != nil {
		t.Fatal(err)
	}
	if resolvedCache != cache {
		t.Fatalf("ModuleCacheDir = %q, want %q", resolvedCache, cache)
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
			if got != int64(91) {
				t.Fatalf("result = %#v, want 91", got)
			}
		})
	}
}

func TestModuleOptionsForScriptLoadsDownloadedGitHubSubdirCache(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	t.Setenv("LEIA_CACHE", cache)
	if err := os.MkdirAll(filepath.Join(cache, "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg", "util.leia"), []byte(`return { value: 92 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leia.mod"), []byte(`module example.com/app
leia 0.1
require github.com/acme/toolkit/pkg v1.2.3
`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.leia")
	if err := os.WriteFile(mainPath, []byte(`u := require("github.com/acme/toolkit/pkg/util"); result := u.value`), 0644); err != nil {
		t.Fatal(err)
	}

	vm := New(ModuleOptionsForScript(mainPath)...)
	if err := vm.ExecFile(mainPath); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(92) {
		t.Fatalf("result = %#v, want subdir cache module value", got)
	}
}

func TestModuleOptionsForScriptPrefersVendorOverCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEIA_CACHE", filepath.Join(dir, "cache"))
	if err := os.MkdirAll(filepath.Join(dir, "vendor", "github.com", "acme", "toolkit@v1.2.3", "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vendor", "github.com", "acme", "toolkit@v1.2.3", "pkg", "util.leia"), []byte(`return { value: 123 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cache", "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cache", "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg", "util.leia"), []byte(`return { value: 91 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leia.mod"), []byte(`module example.com/app
leia 0.1
require github.com/acme/toolkit v1.2.3
`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.leia")
	if err := os.WriteFile(mainPath, []byte(`u := require("github.com/acme/toolkit/pkg/util"); result := u.value`), 0644); err != nil {
		t.Fatal(err)
	}

	vm := New(ModuleOptionsForScript(mainPath)...)
	if err := vm.ExecFile(mainPath); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(123) {
		t.Fatalf("result = %#v, want vendored module value", got)
	}
}

func TestModuleOptionsForScriptModeVendorIgnoresCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEIA_CACHE", filepath.Join(dir, "cache"))
	if err := os.MkdirAll(filepath.Join(dir, "cache", "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cache", "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg", "util.leia"), []byte(`return { value: 91 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leia.mod"), []byte(`module example.com/app
leia 0.1
require github.com/acme/toolkit v1.2.3
`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.leia")
	if err := os.WriteFile(mainPath, []byte(`u := require("github.com/acme/toolkit/pkg/util"); result := u.value`), 0644); err != nil {
		t.Fatal(err)
	}

	for _, mode := range []ModuleMode{ModuleModeMod, ModuleModeReadonly} {
		t.Run(string(mode), func(t *testing.T) {
			vm := New(ModuleOptionsForScriptMode(mainPath, mode)...)
			if err := vm.ExecFile(mainPath); err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("result")
			if err != nil {
				t.Fatal(err)
			}
			if got != int64(91) {
				t.Fatalf("result = %#v, want cached module value", got)
			}
		})
	}

	vm := New(ModuleOptionsForScriptMode(mainPath, ModuleModeVendor)...)
	if err := vm.ExecFile(mainPath); err == nil {
		t.Fatal("vendor mode unexpectedly loaded cache-only module")
	}
}
