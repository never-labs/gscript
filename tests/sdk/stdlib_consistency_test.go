package gscript_test

import (
	"sort"
	"testing"

	gs "github.com/never-labs/gscript"
	"github.com/never-labs/gscript/internal/stdlib/catalog"
)

func TestStdlibCatalogMatchesPublicLibAll(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibAll))
	for _, name := range catalog.ModuleNames() {
		t.Run(name, func(t *testing.T) {
			if got := publicModuleType(t, vm, name); got != "table" {
				t.Fatalf("module %q type = %q, want table", name, got)
			}
		})
	}
}

func TestStdlibCatalogSafeDefaultMatchesPublicLibSafe(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	for _, module := range catalog.Modules() {
		t.Run(module.Name, func(t *testing.T) {
			got := publicModuleType(t, vm, module.Name)
			if module.SafeDefault {
				if got != "table" {
					t.Fatalf("safe module %q type = %q, want table", module.Name, got)
				}
				return
			}
			if got != "nil" {
				t.Fatalf("unsafe module %q type = %q, want nil", module.Name, got)
			}
		})
	}
}

func TestStdlibCatalogHasUniqueModuleNames(t *testing.T) {
	names := catalog.ModuleNames()
	if len(names) == 0 {
		t.Fatal("catalog contains no stdlib modules")
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	for i := 1; i < len(sorted); i++ {
		if sorted[i] == sorted[i-1] {
			t.Fatalf("catalog contains duplicate stdlib module %q", sorted[i])
		}
	}
}

func publicModuleType(t *testing.T, vm *gs.VM, name string) string {
	t.Helper()
	if err := vm.Exec(`result := type(package.loaded["` + name + `"])`); err != nil {
		t.Fatalf("type(package.loaded[%q]): %v", name, err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatalf("get result for %s: %v", name, err)
	}
	if got == nil {
		return "nil"
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("type(%s) result has Go type %T, want string", name, got)
	}
	return s
}
