package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func TestTablePipelineChecksumRuntimeSpecialization(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	top := compileProto(t, `
MOD := 1000000007
func addmod(a, b, ...) { return (a + b) % MOD }
func build(n, ...) {
  t := {}
  for i := 1; i <= n; i++ {
    t[i] = i * 3 + 1
    if i % 3 == 0 { t["k" .. i] = i * 7 + 5 }
    if i % 10 == 0 { t[-i] = i * 11 + 9 }
  }
  return t
}
func scanp(t, ...) {
  sum := 0
  count := 0
  for k, v := range pairs(t) {
    if type(k) == "number" { sum = addmod(sum, k * 3 + v) } else { sum = addmod(sum, #k * 5 + v) }
    count = count + 1
  }
  return addmod(sum, count * 17)
}
func scann(t, ...) {
  sum := 0
  count := 0
  k := nil
  v := nil
  for {
    k, v = next(t, k)
    if k == nil { break }
    if type(k) == "number" { sum = addmod(sum, k * 13 + v) } else { sum = addmod(sum, #k * 19 + v) }
    count = count + 1
  }
  return addmod(sum, count * 23)
}
func scani(t, ...) {
  sum := 0
  count := 0
  for i, v := range ipairs(t) {
    sum = addmod(sum, i * 29 + v)
    count = count + 1
  }
  return addmod(sum, count * 31)
}
func mutate(n, reps, ...) {
  t := {}
  for i := 1; i <= n; i++ {
    t[i] = i
    rawset(t, "s" .. i, i + 1)
  }
  checksum := addmod(rawlen(t), #t)
  for r := 1; r <= reps; r++ {
    pos := (r % n) + 1
    table.insert(t, pos, r)
    removed := table.remove(t, pos + 1)
    hotKey := "hot" .. (r % 64)
    rawset(t, hotKey, removed)
    if r % 5 == 0 {
      rawset(t, n + 8, r)
      rawset(t, n + 8, nil)
    }
    checksum = addmod(checksum, rawget(t, hotKey) + rawlen(t) + #t)
  }
  return checksum
}
func alloc(n, reps, ...) {
  roots := {}
  checksum := 0
  for r := 1; r <= reps; r++ {
    batch := {}
    prev := nil
    for i := 1; i <= n; i++ {
      obj := {id: i + r, tag: "node", value: (i * r) % 997, left: prev, right: nil}
      if prev != nil { prev.right = obj }
      batch[i] = obj
      prev = obj
    }
    for i := 1; i <= n; i = i + 4 {
      obj := batch[i]
      checksum = addmod(checksum, obj.id + obj.value)
    }
    roots[(r % 32) + 1] = batch
  }
  return addmod(checksum, #roots * 37)
}
func drive(size, reps, allocN, allocReps, ...) {
  checksum := 0
  for r := 1; r <= reps; r++ {
    t := build(size)
    checksum = addmod(checksum, scanp(t))
    checksum = addmod(checksum, scann(t))
    checksum = addmod(checksum, scani(t))
  }
  checksum = addmod(checksum, mutate(size, reps * 6))
  checksum = addmod(checksum, alloc(allocN, allocReps))
  return checksum
}
result := drive(40, 3, 20, 4)
`)
	drive := findTestProtoByName(top, "drive")
	if drive == nil {
		t.Fatal("missing drive proto")
	}
	if !isTablePipelineChecksumProto(drive) {
		t.Fatalf("drive did not match table pipeline shape: code=%d const=%d diagnostics=%+v", len(drive.Code), len(drive.Constants), DiagnoseCallSiteRuntimeSpecializationProto(drive))
	}
	vm := New(runtime.NewInterpreterGlobals())
	defer vm.Close()
	if _, err := vm.Execute(top); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "table_pipeline_checksum"); got != 1 {
		t.Fatalf("table_pipeline_checksum hit count = %d, want 1", got)
	}
	if vm.GetGlobal("result").IsNil() {
		t.Fatal("missing result")
	}
}
