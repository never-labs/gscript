package vm

import (
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

const rawIntNestedShiftedSource = `
func nestwave(level, width) {
	if level == 0 { return width + 2 }
	if width == 0 { return nestwave(level - 1, 2) }
	return nestwave(level - 1, nestwave(level, width - 1))
}
`

func TestRawIntNestedRuntimeSpecializationRecognizesShiftedNestedRecurrence(t *testing.T) {
	top := compileProto(t, rawIntNestedShiftedSource)
	fn := findTestProtoByName(top, "nestwave")
	if fn == nil {
		t.Fatal("nestwave proto not found")
	}
	specialization, ok := analyzeRawIntNestedSpecialization(fn)
	if !ok {
		t.Fatalf("nestwave should qualify for raw-int nested recurrence specialization:\n%s", dumpRawIntNestedTestBytecode(fn))
	}
	if specialization.selfName != "nestwave" || specialization.baseAdd != 2 || specialization.zeroArg != 2 || specialization.mStep != 1 || specialization.nStep != 1 {
		t.Fatalf("unexpected specialization: %#v", specialization)
	}
	got, ok := specialization.fold(runtime.IntValue(2), runtime.IntValue(6))
	if !ok || got != 764 {
		t.Fatalf("nestwave(2,6) specialization = %d/%v, want 764/true", got, ok)
	}
}

func TestRawIntNestedRuntimeSpecializationRecognitionCacheAndDiagnostics(t *testing.T) {
	top := compileProto(t, rawIntNestedShiftedSource)
	fn := findTestProtoByName(top, "nestwave")
	if fn == nil {
		t.Fatal("nestwave proto not found")
	}

	requireRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "nested_int_recurrence")
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(fn), "nested_int_recurrence")
	if !cachedRuntimeSpecializationRecognized(fn, runtimeSpecializationRawIntNested) {
		t.Fatal("raw nested int runtime specialization rejected by hot cache")
	}
	if fn.RuntimeSpecs.RuntimeSpecialization == nil || fn.RuntimeSpecs.RuntimeSpecialization.recognized == 0 {
		t.Fatal("runtime specialization cache was not populated")
	}
	if fn.RuntimeSpecs.RawIntNestedSpecialization == nil || fn.RuntimeSpecs.RawIntNestedSpecialization.plan == nil {
		t.Fatal("raw nested int specialization cache was not populated")
	}
	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseCallSiteRuntimeSpecializationProto(fn), "nested_int_recurrence")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
	if diag.Specialization.Route != RuntimeSpecializationRouteCallSiteValue || diag.Specialization.Arity != 2 || diag.Specialization.Results != 1 {
		t.Fatalf("unexpected diagnostic metadata: %+v", diag.Specialization)
	}

	mutated := *fn
	mutated.Code = nil
	rejectRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(&mutated), "nested_int_recurrence")
	diag = requireRuntimeSpecializationDiagnostic(t, DiagnoseCallSiteRuntimeSpecializationProto(&mutated), "nested_int_recurrence")
	if diag.Recognized || diag.Reason != runtimeSpecializationReasonShapeMismatch {
		t.Fatalf("mutated diagnostic = %+v, want shape mismatch", diag)
	}
}

func TestRawIntNestedRuntimeSpecializationRecordsHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, rawIntNestedShiftedSource+`
result := nestwave(2, 6)
`)
	expectGlobalInt(t, globals, "result", 764)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "nested_int_recurrence"); got != 1 {
		t.Fatalf("nested_int_recurrence structural hit count = %d, want 1", got)
	}
}

func TestRawIntNestedRuntimeSpecializationFallsBackWhenSelfGlobalChanges(t *testing.T) {
	top := compileProto(t, rawIntNestedShiftedSource)
	v := New(vmtest.NewInterpreterGlobals())
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("runtime error: %v", err)
	}
	fn := v.GetGlobal("nestwave")
	cl, ok := closureFromValue(fn)
	if !ok {
		t.Fatalf("nestwave global is not a VM closure: %s", fn.TypeName())
	}
	v.SetGlobal("nestwave", runtime.FunctionValue(&runtime.GoFunction{
		Name: "replacement",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			return []runtime.Value{runtime.IntValue(41)}, nil
		},
	}))

	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	handled, _, err := v.tryRunValueRuntimeSpecialization(cl, []runtime.Value{runtime.IntValue(1), runtime.IntValue(0)})
	if err != nil {
		t.Fatalf("runtime specialization returned error: %v", err)
	}
	if handled {
		t.Fatal("runtime specialization handled call after self global changed")
	}
	results, err := v.CallValue(fn, []runtime.Value{runtime.IntValue(1), runtime.IntValue(0)})
	if err != nil {
		t.Fatalf("fallback call error: %v", err)
	}
	if len(results) != 1 || !results[0].IsInt() || results[0].Int() != 41 {
		t.Fatalf("fallback result = %+v, want 41", results)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "nested_int_recurrence"); got != 0 {
		t.Fatalf("nested_int_recurrence structural hit count = %d, want 0", got)
	}
}

func runtimeRuntimeSpecializationHitCount(stats *runtime.RuntimePathStats, route RuntimeSpecializationRoute, name string) uint64 {
	for _, entry := range stats.Snapshot().RuntimeSpecialization.PerSpecialization {
		if entry.Route == string(route) && entry.Name == name {
			return entry.Count
		}
	}
	return 0
}

func dumpRawIntNestedTestBytecode(proto *FuncProto) string {
	var b strings.Builder
	for pc, inst := range proto.Code {
		b.WriteString(itoaRawIntNestedTest(int(DecodeOp(inst))))
		b.WriteString(" pc=")
		b.WriteString(itoaRawIntNestedTest(pc))
		b.WriteString(" A=")
		b.WriteString(itoaRawIntNestedTest(DecodeA(inst)))
		b.WriteString(" B=")
		b.WriteString(itoaRawIntNestedTest(DecodeB(inst)))
		b.WriteString(" C=")
		b.WriteString(itoaRawIntNestedTest(DecodeC(inst)))
		b.WriteString(" Bx=")
		b.WriteString(itoaRawIntNestedTest(DecodeBx(inst)))
		b.WriteString(" sBx=")
		b.WriteString(itoaRawIntNestedTest(DecodesBx(inst)))
		b.WriteByte('\n')
	}
	return b.String()
}

func itoaRawIntNestedTest(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
