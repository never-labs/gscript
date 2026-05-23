//go:build darwin && arm64

package methodjit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gscript/gscript/internal/vm"
)

const rawIntNestedTieringSource = `
func nestwave(level, width) {
	if level == 0 { return width + 2 }
	if width == 0 { return nestwave(level - 1, 2) }
	return nestwave(level - 1, nestwave(level, width - 1))
}
`

const genericRecordArrayLoopTieringSource = `
func update(rows, n, dt) {
	for i := 1; i <= n; i++ {
		row := rows[i]
		row.x = row.x + row.v * dt
	}
}

func build(n) {
	rows := {}
	for i := 1; i <= n; i++ {
		rows[i] = {x: i, v: i + 1}
	}
	return rows
}

n := 16
dt := 0.5
rows := build(n)
for pass := 1; pass <= 128; pass++ {
	update(rows, n, dt)
}
`

func TestWholeCallRuntimeSpecializationTieringUsesVMCapability(t *testing.T) {
	top := compileProto(t, rawIntNestedTieringSource)
	fn := findProtoByName(top, "nestwave")
	if fn == nil {
		t.Fatal("nestwave proto not found")
	}
	info, ok := recognizedWholeCallRuntimeSpecializationForTiering(fn)
	if !ok {
		t.Fatal("nested_int_recurrence should expose a whole-call structural tiering kernel")
	}
	if info.Name != "nested_int_recurrence" {
		t.Fatalf("whole-call runtime specialization=%q, want nested_int_recurrence", info.Name)
	}
	if !info.HasCapability(vm.RuntimeSpecializationCapabilityStructuralTiering) {
		t.Fatalf("nested_int_recurrence missing structural tiering capability: %+v", info)
	}
	tm := NewTieringManager()
	decision := tm.policy.Decide(fn, analyzeFuncProfile(fn), PromotionPolicyState{Manager: tm})
	if decision.Action != TieringActionRuntimeSpecialization {
		t.Fatalf("tiering action=%s, want %s", decision.Action, TieringActionRuntimeSpecialization)
	}
	if decision.RuntimeSpecialization.specialization != "nested_int_recurrence" {
		t.Fatalf("decision runtime specialization=%q, want nested_int_recurrence", decision.RuntimeSpecialization.specialization)
	}
}

func TestDriverLoopRuntimeSpecializationTieringUsesVMCapability(t *testing.T) {
	top := compileProto(t, genericRecordArrayLoopTieringSource)
	tm := NewTieringManager()
	info, ok := tm.driverLoopRuntimeSpecializationForTiering(top)
	if !ok {
		t.Fatal("generic record-array loop should expose a driver-loop structural runtime specialization")
	}
	if info.Name != "generic_record_array_loop" {
		t.Fatalf("driver loop runtime specialization=%q, want generic_record_array_loop", info.Name)
	}
	if !info.HasCapability(vm.RuntimeSpecializationCapabilityStructuralTiering) {
		t.Fatalf("generic_record_array_loop missing structural tiering capability: %+v", info)
	}
	decision := tm.policy.Decide(top, analyzeFuncProfile(top), PromotionPolicyState{Manager: tm})
	if decision.Action != TieringActionRuntimeSpecialization {
		t.Fatalf("tiering action=%s, want %s", decision.Action, TieringActionRuntimeSpecialization)
	}
	if decision.RuntimeSpecialization.specialization != "generic_record_array_loop" {
		t.Fatalf("decision runtime specialization=%q, want generic_record_array_loop", decision.RuntimeSpecialization.specialization)
	}
}

func TestDriverLoopRuntimeSpecializationTieringKeepsDensePairwiseLoopOnVMRoute(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "suite", "nbody_dense.gs"))
	if err != nil {
		t.Fatalf("read nbody_dense benchmark: %v", err)
	}
	top := compileProto(t, string(src))
	tm := NewTieringManager()
	info, ok := tm.driverLoopRuntimeSpecializationForTiering(top)
	if !ok {
		t.Fatal("nbody_dense main should expose a driver-loop structural runtime specialization")
	}
	if info.Name != "record_pairwise_numeric_loop" {
		t.Fatalf("driver loop runtime specialization=%q, want record_pairwise_numeric_loop", info.Name)
	}
	if !info.HasCapability(vm.RuntimeSpecializationCapabilityStructuralTiering) {
		t.Fatalf("record_pairwise_numeric_loop missing structural tiering capability: %+v", info)
	}
	decision := tm.policy.Decide(top, analyzeFuncProfile(top), PromotionPolicyState{Manager: tm})
	if decision.Action != TieringActionRuntimeSpecialization {
		t.Fatalf("tiering action=%s, want %s", decision.Action, TieringActionRuntimeSpecialization)
	}
	if decision.RuntimeSpecialization.specialization != "record_pairwise_numeric_loop" {
		t.Fatalf("decision runtime specialization=%q, want record_pairwise_numeric_loop", decision.RuntimeSpecialization.specialization)
	}
}
