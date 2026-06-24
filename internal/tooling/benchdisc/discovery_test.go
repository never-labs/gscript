package benchdisc

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeSpec struct {
	group string
	name  string
}

func (f fakeSpec) ID() string { return f.group + "/" + f.name }

func writeBenchFile(t *testing.T, root, group, name string) {
	t.Helper()
	path := filepath.Join(root, "benchmarks", group, name+".leia")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("-- test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGroupChoicesIncludesOnlyDomainGroupNames(t *testing.T) {
	want := []string{"numeric", "recursion", "table", "calls", "string", "concurrency", "data", "app", "control", "precision"}
	if !reflect.DeepEqual(GroupChoices(DomainGroups), want) {
		t.Fatalf("GroupChoices = %#v, want %#v", GroupChoices(DomainGroups), want)
	}
}

func TestGroupChoicesUsesAllowedGroupsVerbatim(t *testing.T) {
	want := []string{"numeric", "data"}
	if !reflect.DeepEqual(GroupChoices(want), want) {
		t.Fatalf("GroupChoices = %#v, want %#v", GroupChoices(want), want)
	}
}

func TestDomainSpecsPrefersDefaultOrderThenSortedExtrasAndLuaJITRefs(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"z_extra", "matmul", "a_extra", "matmul_row"} {
		writeBenchFile(t, root, "numeric", name)
	}
	lua := filepath.Join(root, "benchmarks", "lua_ref", "numeric", "matmul.lua")
	if err := os.MkdirAll(filepath.Dir(lua), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lua, []byte("-- ref\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	specs, err := DomainSpecs(root, "numeric")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	if want := []string{"matmul", "a_extra", "matmul_row", "z_extra"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %#v, want %#v", names, want)
	}
	if got := specs[0].LuaJITRel(); got != "benchmarks/lua_ref/numeric/matmul.lua" {
		t.Fatalf("LuaJITRel = %q", got)
	}
	if specs[1].LuaJITRel() != "" {
		t.Fatalf("extra LuaJITRel = %q, want empty", specs[1].LuaJITRel())
	}
	if specs[2].Base != "matmul" {
		t.Fatalf("base = %q, want matmul", specs[2].Base)
	}
}

func TestSelectSpecsRejectsShortNameAliases(t *testing.T) {
	_, err := SelectSpecs([]fakeSpec{{"numeric", "sort"}, {"table", "sort"}}, []string{"sort"})
	if err == nil || !strings.Contains(err.Error(), "unknown benchmark selector: sort") {
		t.Fatalf("err = %v", err)
	}
}

func TestSelectSpecsRejectsUnknownDomainSelector(t *testing.T) {
	_, err := SelectSpecs([]fakeSpec{{"table", "events_metamethod"}}, []string{"missing_domain/events_metamethod"})
	if err == nil || !strings.Contains(err.Error(), "unknown benchmark selector") {
		t.Fatalf("err = %v", err)
	}
}

func TestBenchmarkIDFromSelectorAcceptsBenchmarkFilePaths(t *testing.T) {
	for _, selector := range []string{"benchmarks/numeric/matmul.leia", "numeric/matmul.leia"} {
		got, ok := BenchmarkIDFromSelector(selector, DomainGroups)
		if !ok || got != "numeric/matmul" {
			t.Fatalf("BenchmarkIDFromSelector(%q) = %q, %v", selector, got, ok)
		}
	}
}

func TestSpecSelectorsIncludesOnlyCanonicalDomainNames(t *testing.T) {
	got := SpecSelectors([]fakeSpec{{"numeric", "matmul"}, {"calls", "closure_accumulator"}})
	want := map[string]struct{}{"numeric/matmul": {}, "calls/closure_accumulator": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectors = %#v, want %#v", got, want)
	}
}

func TestSelectorMatchesSpecAcceptsOnlyDomainSelectors(t *testing.T) {
	spec := fakeSpec{"numeric", "matmul"}
	if !SelectorMatchesSpec("numeric/matmul", spec) {
		t.Fatal("numeric/matmul should match")
	}
	if !SelectorMatchesSpec("benchmarks/numeric/matmul.leia", spec) {
		t.Fatal("path selector should match")
	}
	for _, selector := range []string{"missing_domain/matmul", "matmul", "missing_domain/events_metamethod"} {
		if SelectorMatchesSpec(selector, spec) {
			t.Fatalf("%q should not match", selector)
		}
	}
}

func TestParseSelectorCountOverridesAcceptsModesAndDomainSelectors(t *testing.T) {
	overrides, err := ParseSelectorCountOverrides(
		[]string{"recursion/fib=4", "numeric/matmul=6", "vm/table/events_metamethod=8"},
		[]string{"vm", "default"},
		"--repeat",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode string
		id   string
		want int
	}{
		{"default", "recursion/fib", 4},
		{"default", "numeric/matmul", 6},
		{"vm", "table/events_metamethod", 8},
	} {
		got, ok := SelectorCountOverride(overrides, tc.mode, tc.id)
		if !ok || got != tc.want {
			t.Fatalf("override(%s,%s) = %d,%v want %d,true", tc.mode, tc.id, got, ok, tc.want)
		}
	}
	if _, ok := SelectorCountOverride(overrides, "default", "table/events_metamethod"); ok {
		t.Fatal("default table/events_metamethod should be absent")
	}
	if _, ok := SelectorCountOverride(overrides, "default", "missing_domain/matmul"); ok {
		t.Fatal("missing domain should be absent")
	}
}

func TestParseSelectorCountOverridesRejectsBadCounts(t *testing.T) {
	for _, value := range []string{"recursion/fib=0", "recursion/fib=nope", "fib=4"} {
		if _, err := ParseSelectorCountOverrides([]string{value}, []string{"vm"}, "--repeat"); err == nil {
			t.Fatalf("ParseSelectorCountOverrides(%q) succeeded", value)
		}
	}
}

func TestResolveScriptPathRejectsUnknownSuffixSelectors(t *testing.T) {
	root := t.TempDir()
	writeBenchFile(t, root, "calls", "closure_accumulator")
	want := filepath.Join(root, "benchmarks", "calls", "closure_accumulator.leia")
	for _, selector := range []string{"calls/closure_accumulator", "benchmarks/calls/closure_accumulator.leia"} {
		got, ok := ResolveScriptPath(root, selector, DomainGroups)
		if !ok || got != want {
			t.Fatalf("ResolveScriptPath(%q) = %q,%v want %q,true", selector, got, ok, want)
		}
	}
	for _, selector := range []string{"old_group/closure_accumulator", "calls/closure_accumulator_unknown", "closure_accumulator"} {
		if got, ok := ResolveScriptPath(root, selector, DomainGroups); ok {
			t.Fatalf("ResolveScriptPath(%q) = %q,true want false", selector, got)
		}
	}
}

func TestResolveScriptIdentityReturnsDomainGroupNameAndPath(t *testing.T) {
	root := t.TempDir()
	writeBenchFile(t, root, "table", "events_metamethod")
	got, ok := ResolveScriptIdentity(root, "table/events_metamethod", DomainGroups)
	if !ok {
		t.Fatal("ResolveScriptIdentity failed")
	}
	wantPath := filepath.Join(root, "benchmarks", "table", "events_metamethod.leia")
	if got.Group != "table" || got.Name != "events_metamethod" || got.Leia != wantPath {
		t.Fatalf("identity = %#v, want table/events_metamethod %s", got, wantPath)
	}
	if _, ok := ResolveScriptIdentity(root, "missing_domain/events_metamethod", DomainGroups); ok {
		t.Fatal("missing domain should not resolve")
	}
}

func TestGroupsForSelectorsIncludesDomainSelectorGroups(t *testing.T) {
	root := t.TempDir()
	writeBenchFile(t, root, "concurrency", "goroutine_sleep")
	writeBenchFile(t, root, "table", "events_metamethod")
	got, err := GroupsForSelectors(root, []string{"data"}, []string{"concurrency/goroutine_sleep", "table/events_metamethod"}, DomainGroups)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"data", "concurrency", "table"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
}

func TestGroupsForSelectorsIgnoresUnknownSelectors(t *testing.T) {
	root := t.TempDir()
	writeBenchFile(t, root, "concurrency", "goroutine_sleep")
	writeBenchFile(t, root, "table", "events_metamethod")
	got, err := GroupsForSelectors(root, []string{"data"}, []string{"missing_domain/goroutine_sleep", "unknown/events_metamethod"}, DomainGroups)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"data"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
}

func TestGroupsForSelectorsCanStartFromOnlySelectors(t *testing.T) {
	root := t.TempDir()
	writeBenchFile(t, root, "table", "events_metamethod")
	writeBenchFile(t, root, "data", "soa_dot")
	got, err := GroupsForSelectors(root, []string{}, []string{"table/events_metamethod", "data/soa_dot"}, DomainGroups)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"table", "data"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
}

func TestGroupsForSelectorsIgnoresSelectorsOutsideAllowedGroups(t *testing.T) {
	root := t.TempDir()
	writeBenchFile(t, root, "data", "soa_dot")
	writeBenchFile(t, root, "concurrency", "goroutine_sleep")
	got, err := GroupsForSelectors(root, []string{"data"}, []string{"missing_domain/goroutine_sleep"}, []string{"data"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"data"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
}

func TestGroupsForSelectionHandlesAllGroupsFlag(t *testing.T) {
	got, err := GroupsForSelection(t.TempDir(), []string{"data"}, []string{"table/events_metamethod"}, true, []string{"data", "table"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"data", "table"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
}
