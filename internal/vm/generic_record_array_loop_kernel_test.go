package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

const genericRecordArrayDriverLoopSource = `
func update(rows, n, dt) {
	for i := 1; i <= n; i++ {
		row := rows[i]
		row.x = row.x + row.v * dt
	}
}

func drive(passes) {
	for pass := 1; pass <= passes; pass++ {
		update(rows, n, dt)
	}
}

func build(n) {
	rows := {}
	for i := 1; i <= n; i++ {
		rows[i] = {x: i, v: i + 1}
	}
	return rows
}
`

func TestGenericRecordArrayLoopRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, genericRecordArrayDriverLoopSource)
	update := findTestProtoByName(top, "update")
	drive := findTestProtoByName(top, "drive")
	if update == nil || drive == nil {
		t.Fatal("missing update or drive proto")
	}
	globals := map[string]*FuncProto{"update": update}

	requireKernelInfo(t, DriverLoopKernelCatalog(), "generic_record_array_loop")
	requireKernelInfo(t, RecognizedDriverLoopKernels(drive, globals), "generic_record_array_loop")
	diag := requireKernelDiagnostic(t, DiagnoseDriverLoopKernels(drive, globals), "generic_record_array_loop")
	if !diag.Recognized || diag.Reason != kernelReasonDriverRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, kernelReasonDriverRecognized)
	}
	if diag.Kernel.Route != KernelRouteDriverLoop ||
		diag.Kernel.Arity != kernelUnknownDriverLoopArity ||
		diag.Kernel.Results != kernelUnknownDriverLoopResultCount {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Kernel)
	}

	diag = requireKernelDiagnostic(t, DiagnoseDriverLoopKernels(drive, nil), "generic_record_array_loop")
	if diag.Recognized || diag.Reason != kernelReasonMissingGlobalProtoMap {
		t.Fatalf("missing-global diagnostic = %+v, want %q", diag, kernelReasonMissingGlobalProtoMap)
	}
}

func TestGenericRecordArrayLoopRuntimeSpecializationRecordsHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, genericRecordArrayDriverLoopSource+`
n := 16
dt := 0.5
rows := build(n)
drive(128)
result := rows[1].x
`)
	expectGlobalFloat(t, globals, "result", 129)
	if got := runtimeStructuralKernelHitCount(stats, KernelRouteDriverLoop, "generic_record_array_loop"); got != 1 {
		t.Fatalf("generic_record_array_loop structural hit count = %d, want 1", got)
	}
}
