package gscript

import (
	"reflect"
	"sort"
	"testing"

	rt "github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/stdlib/catalog"
)

func TestStdlibModuleNameSetsStayInSync(t *testing.T) {
	catalogNames := sortedStringSet(catalog.ModuleNames())
	runtimeNames := runtimeStdlibModuleNames(t)
	allowedAllNames, disabledAllNames := stdlibPolicyNames(stdlibAllowedNames(LibAll))

	assertSameNames(t, "runtime stdlib registration", runtimeNames, "catalog", catalogNames)
	assertSameNames(t, "stdlibAllowedNames(LibAll)", allowedAllNames, "catalog", catalogNames)
	if len(disabledAllNames) > 0 {
		t.Fatalf("stdlibAllowedNames(LibAll) disables registered names: %v", disabledAllNames)
	}
}

func TestStdlibSafePolicyMatchesCatalog(t *testing.T) {
	var catalogSafeNames []string
	for _, module := range catalog.Modules() {
		if module.SafeDefault {
			catalogSafeNames = append(catalogSafeNames, module.Name)
		}
	}
	sort.Strings(catalogSafeNames)

	allowedSafeNames, _ := stdlibPolicyNames(stdlibAllowedNames(LibSafe))
	assertSameNames(t, "stdlibAllowedNames(LibSafe)", allowedSafeNames, "catalog SafeDefault", catalogSafeNames)
}

func runtimeStdlibModuleNames(t *testing.T) []string {
	t.Helper()

	pkg := rt.New().GetGlobal("package")
	if !pkg.IsTable() {
		t.Fatal("runtime package global is not a table")
	}
	loaded := pkg.Table().RawGetString("loaded")
	if !loaded.IsTable() {
		t.Fatal("runtime package.loaded is not a table")
	}

	var names []string
	for _, key := range loaded.Table().PairsKeysSnapshot() {
		if key.IsString() {
			names = append(names, key.Str())
		}
	}
	sort.Strings(names)
	return names
}

func stdlibPolicyNames(allowed map[string]bool) (enabled []string, disabled []string) {
	for name, ok := range allowed {
		if ok {
			enabled = append(enabled, name)
		} else {
			disabled = append(disabled, name)
		}
	}
	sort.Strings(enabled)
	sort.Strings(disabled)
	return enabled, disabled
}

func sortedStringSet(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}

func assertSameNames(t *testing.T, gotLabel string, got []string, wantLabel string, want []string) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		return
	}
	t.Fatalf("%s drifted from %s\nonly in %s: %v\nonly in %s: %v",
		gotLabel, wantLabel,
		gotLabel, stringSetDifference(got, want),
		wantLabel, stringSetDifference(want, got))
}

func stringSetDifference(a, b []string) []string {
	bSet := make(map[string]bool, len(b))
	for _, name := range b {
		bSet[name] = true
	}
	var diff []string
	for _, name := range a {
		if !bSet[name] {
			diff = append(diff, name)
		}
	}
	return diff
}
