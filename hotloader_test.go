package gscript_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript"
)

func TestHotLoaderReloadSwapsProgram(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	if err := os.WriteFile(path, []byte(`func answer() { return 1 }`), 0644); err != nil {
		t.Fatal(err)
	}

	loader := gs.NewHotLoader()
	handle, err := loader.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if handle.Generation() != 1 {
		t.Fatalf("generation = %d, want 1", handle.Generation())
	}

	vm := gs.New()
	got, err := handle.Call(vm, "answer")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != int64(1) {
		t.Fatalf("answer = %v, want [1]", got)
	}

	if err := os.WriteFile(path, []byte(`func answer() { return 2 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := loader.Reload(path); err != nil {
		t.Fatal(err)
	}
	if handle.Generation() != 2 {
		t.Fatalf("generation = %d, want 2", handle.Generation())
	}

	vm = gs.New()
	got, err = handle.Call(vm, "answer")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != int64(2) {
		t.Fatalf("answer = %v, want [2]", got)
	}
}

func TestHotLoaderReloadFailureKeepsPreviousProgram(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	if err := os.WriteFile(path, []byte(`func answer() { return 7 }`), 0644); err != nil {
		t.Fatal(err)
	}

	loader := gs.NewHotLoader()
	handle, err := loader.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(`func answer() {`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := loader.Reload(path); err == nil {
		t.Fatal("expected reload error")
	}
	if handle.Generation() != 1 {
		t.Fatalf("generation = %d, want 1", handle.Generation())
	}

	vm := gs.New()
	got, err := handle.Call(vm, "answer")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != int64(7) {
		t.Fatalf("answer = %v, want [7]", got)
	}
}

func TestHotLoaderReloadIfChangedSkipsSameSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	if err := os.WriteFile(path, []byte(`func answer() { return 7 }`), 0644); err != nil {
		t.Fatal(err)
	}

	loader := gs.NewHotLoader()
	handle, err := loader.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	result, err := loader.ReloadIfChanged(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Fatal("ReloadIfChanged changed unchanged source")
	}
	if result.Generation != 1 || handle.Generation() != 1 {
		t.Fatalf("generation result=%d handle=%d, want 1", result.Generation, handle.Generation())
	}

	if err := os.WriteFile(path, []byte(`func answer() { return 8 }`), 0644); err != nil {
		t.Fatal(err)
	}
	result, err = loader.ReloadIfChanged(path)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Generation != 2 || handle.Generation() != 2 {
		t.Fatalf("ReloadIfChanged result=%+v handle generation=%d, want changed generation 2", result, handle.Generation())
	}
}

func writeScript(t *testing.T, path, src string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(src)), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertCallResult(t *testing.T, inst *gs.HotInstance, name string, argsAndWant ...interface{}) {
	t.Helper()
	if len(argsAndWant) == 0 {
		t.Fatal("missing expected result")
	}
	want := argsAndWant[len(argsAndWant)-1]
	args := argsAndWant[:len(argsAndWant)-1]
	got, err := inst.Call(name, args...)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s(%v) = %v, want [%v]", name, args, got, want)
	}
}
