package leia_test

import (
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestHotInstanceReloadFailureDoesNotAdvanceOrPolluteState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.leia")
	writeScript(t, path, `
counter := 0
func inc() {
	counter += 1
	return counter
}
`)

	inst, err := leia.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "inc", int64(1))

	writeScript(t, path, `func inc() {`)
	err = inst.Reload()
	if err == nil {
		t.Fatal("expected reload error")
	}
	if inst.Generation() != 1 {
		t.Fatalf("generation = %d, want 1", inst.Generation())
	}
	assertCallResult(t, inst, "inc", int64(2))
}

func TestHotInstanceReloadRuntimeFailureDeepRollsBackTableMutation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.leia")
	writeScript(t, path, `
state := { total: 1, nested: { value: 2 } }
func total() {
	return state.total + state.nested.value
}
`)

	inst, err := leia.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "total", int64(3))

	writeScript(t, path, `
state.total = 100
state.nested.value = 200
missing_function()
func total() {
	return state.total + state.nested.value
}
`)
	if err := inst.Reload(); err == nil {
		t.Fatal("expected reload error")
	}
	assertCallResult(t, inst, "total", int64(3))
}

func TestHotInstanceReloadRuntimeFailureRollsBackBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.leia")
	writeScript(t, path, `
counter := 0
func inc() {
	counter += 1
	return counter
}
`)

	inst, err := leia.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "inc", int64(1))

	writeScript(t, path, `
counter := 0
new_default := 9
missing_function()
func inc() {
	counter += 100
	return counter
}
`)
	err = inst.Reload()
	if err == nil {
		t.Fatal("expected reload error")
	}
	if inst.Generation() != 1 {
		t.Fatalf("generation = %d, want 1", inst.Generation())
	}
	assertCallResult(t, inst, "inc", int64(2))
	if val, err := inst.VM().Get("new_default"); err != nil || val != nil {
		t.Fatalf("new_default = %v, %v; want nil, nil", val, err)
	}
}
