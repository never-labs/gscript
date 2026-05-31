package gscript_test

import (
	"path/filepath"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestHotInstanceReloadPreservesScalarStateAndReplacesFunction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
counter := 0
func inc() {
	counter += 1
	return counter
}
`)

	loader := gs.NewHotLoader()
	inst, err := loader.LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "inc", int64(1))
	assertCallResult(t, inst, "inc", int64(2))

	writeScript(t, path, `
counter := 0
func inc() {
	counter += 10
	return counter
}
`)
	if err := inst.Reload(); err != nil {
		t.Fatal(err)
	}
	if inst.Generation() != 2 {
		t.Fatalf("generation = %d, want 2", inst.Generation())
	}
	assertCallResult(t, inst, "inc", int64(12))
	assertCallResult(t, inst, "inc", int64(22))
}

func TestHotInstanceReloadIfChangedDoesNotReplayTopLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
loaded := 0
loaded += 1
func count_loaded() {
	return loaded
}
`)

	inst, err := gs.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "count_loaded", int64(1))

	result, err := inst.ReloadIfChanged()
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Fatal("ReloadIfChanged changed unchanged source")
	}
	assertCallResult(t, inst, "count_loaded", int64(1))

	writeScript(t, path, `
loaded := 0
loaded += 1
func count_loaded() {
	return loaded + 10
}
`)
	result, err = inst.ReloadIfChanged()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Generation != 2 {
		t.Fatalf("ReloadIfChanged result=%+v, want changed generation 2", result)
	}
	assertCallResult(t, inst, "count_loaded", int64(11))
}

func TestHotInstanceReloadPreservesTableIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
state := { total: 0 }
func add(v) {
	state.total += v
	return state.total
}
`)

	inst, err := gs.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "add", int64(3), int64(3))

	writeScript(t, path, `
state := { total: 100 }
func add(v) {
	state.total += v * 2
	return state.total
}
`)
	if err := inst.Reload(); err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "add", int64(4), int64(11))
}

func TestHotInstanceReloadMergesTableDefaultsAndMethods(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
api := {
	total: 0,
	add: func(v) {
		api.total += v
		return api.total
	}
}
func call_add(v) {
	return api.add(v)
}
func get_bonus() {
	return api.bonus
}
`)

	inst, err := gs.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "call_add", int64(3), int64(3))

	writeScript(t, path, `
api := {
	total: 100,
	bonus: 7,
	add: func(v) {
		api.total += v * 2
		return api.total
	}
}
func call_add(v) {
	return api.add(v)
}
func get_bonus() {
	return api.bonus
}
`)
	if err := inst.Reload(); err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "call_add", int64(4), int64(11))
	assertCallResult(t, inst, "get_bonus", int64(7))
}

func TestHotInstanceReloadAddsNewDefaultsWithoutResettingOldState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
counter := 0
func inc() {
	counter += 1
	return counter
}
`)

	inst, err := gs.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "inc", int64(1))

	writeScript(t, path, `
counter := 0
step := 5
func inc() {
	counter += step
	return counter
}
`)
	if err := inst.Reload(); err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "inc", int64(6))
}

func TestHotInstanceReloadSkipsUnchangedFunctionDeclarations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
seed := 0
make_counter := func() {
	n := 0
	return func() {
		n += 1
		return n
	}
}
counter_fn := make_counter()
func next_value() {
	return counter_fn()
}
`)

	inst, err := gs.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "next_value", int64(1))
	assertCallResult(t, inst, "next_value", int64(2))

	writeScript(t, path, `
seed := 99
make_counter := func() {
	n := 0
	return func() {
		n += 1
		return n
	}
}
counter_fn := make_counter()
func next_value() {
	return counter_fn()
}
extra := 42
`)
	if err := inst.Reload(); err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "next_value", int64(3))
}

func TestHotInstanceGenerationTracksAppliedInstanceOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `func answer() { return 1 }`)

	loader := gs.NewHotLoader()
	inst, err := loader.LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Generation() != 1 {
		t.Fatalf("instance generation = %d, want 1", inst.Generation())
	}

	writeScript(t, path, `func answer() { return 2 }`)
	if err := loader.Reload(path); err != nil {
		t.Fatal(err)
	}
	if inst.Handle().Generation() != 2 {
		t.Fatalf("handle generation = %d, want 2", inst.Handle().Generation())
	}
	if inst.Generation() != 1 {
		t.Fatalf("instance generation = %d, want still-applied 1", inst.Generation())
	}
}

func TestHotInstanceReloadDoesNotDeleteMissingFunctions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
counter := 0
func inc() {
	counter += 1
	return counter
}
`)

	inst, err := gs.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}

	writeScript(t, path, `
counter := 0
extra := 5
`)
	if err := inst.Reload(); err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "inc", int64(1))
	if val, err := inst.VM().Get("extra"); err != nil || val != int64(5) {
		t.Fatalf("extra = %v, %v; want 5, nil", val, err)
	}
}

func TestHotInstanceReloadWorksWithBytecodeVM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
counter := 1
func value() {
	counter += 1
	return counter
}
`)

	loader := gs.NewHotLoader(gs.WithHotLoaderVMOptions(gs.WithVM()))
	inst, err := loader.LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "value", int64(2))

	writeScript(t, path, `
counter := 1
func value() {
	counter += 3
	return counter
}
`)
	if err := inst.Reload(); err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "value", int64(5))
}

func TestHotInstanceCallDoesNotReplayTopLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logic.gs")
	writeScript(t, path, `
loaded := 0
loaded += 1
func count_loaded() {
	return loaded
}
`)

	inst, err := gs.NewHotLoader().LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "count_loaded", int64(1))
	assertCallResult(t, inst, "count_loaded", int64(1))
}
