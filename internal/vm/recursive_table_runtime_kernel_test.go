package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

const recursiveTableKernelSource = `
func makeTree(depth) {
	if depth == 0 {
		return {left: nil, right: nil}
	}
	return {left: makeTree(depth - 1), right: makeTree(depth - 1)}
}

func checkTree(node) {
	if node.left == nil {
		return 1
	}
	return 1 + checkTree(node.left) + checkTree(node.right)
}
`

func TestRecursiveTableWholeCallRunsWithoutTier2Promotion(t *testing.T) {
	top := compileProto(t, recursiveTableKernelSource+`
root := makeTree(4)
result := checkTree(root)
`)
	globals := runtime.NewInterpreterGlobals()
	v := New(globals)
	defer v.Close()
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("execute: %v", err)
	}
	root := v.GetGlobal("root")
	if !root.IsTable() {
		t.Fatalf("root=%v, want table", root)
	}
	if depth, key1, key2, ok := root.Table().LazyRecursiveTablePureInfo(); !ok || depth != 4 || key1 != "left" || key2 != "right" {
		t.Fatalf("root lazy info depth=%d key1=%q key2=%q ok=%v", depth, key1, key2, ok)
	}
	result := v.GetGlobal("result")
	if !result.IsInt() || result.Int() != 31 {
		t.Fatalf("result=%v, want int 31", result)
	}
}

func TestRecursiveTableRuntimeSpecializationRecognitionCacheAndDiagnostics(t *testing.T) {
	top := compileProto(t, recursiveTableKernelSource)
	builder := findTestProtoByName(top, "makeTree")
	fold := findTestProtoByName(top, "checkTree")
	if builder == nil || fold == nil {
		t.Fatalf("missing recursive table protos: builder=%v fold=%v", builder != nil, fold != nil)
	}

	requireRuntimeSpecializationInfo(t, WholeCallRuntimeSpecializationCatalog(), "lazy_recursive_table_builder")
	requireRuntimeSpecializationInfo(t, WholeCallRuntimeSpecializationCatalog(), "lazy_recursive_table_fold")
	requireRuntimeSpecializationInfo(t, RecognizedWholeCallRuntimeSpecializations(builder), "lazy_recursive_table_builder")
	requireRuntimeSpecializationInfo(t, RecognizedWholeCallRuntimeSpecializations(fold), "lazy_recursive_table_fold")
	if !cachedRuntimeSpecializationRecognized(builder, runtimeSpecializationLazyRecursiveTableBuilder) {
		t.Fatal("lazy recursive table builder rejected by runtime specialization cache")
	}
	if !cachedRuntimeSpecializationRecognized(fold, runtimeSpecializationLazyRecursiveTableFold) {
		t.Fatal("lazy recursive table fold rejected by runtime specialization cache")
	}

	builderDiag := requireRuntimeSpecializationDiagnostic(t, DiagnoseWholeCallRuntimeSpecializationProto(builder), "lazy_recursive_table_builder")
	if !builderDiag.Recognized || builderDiag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("builder diagnostic = %+v, want recognized %q", builderDiag, runtimeSpecializationReasonRecognized)
	}
	foldDiag := requireRuntimeSpecializationDiagnostic(t, DiagnoseWholeCallRuntimeSpecializationProto(fold), "lazy_recursive_table_fold")
	if !foldDiag.Recognized || foldDiag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("fold diagnostic = %+v, want recognized %q", foldDiag, runtimeSpecializationReasonRecognized)
	}
}

func TestRecursiveTableRuntimeSpecializationFallsBackWhenSelfGlobalChanges(t *testing.T) {
	top := compileProto(t, recursiveTableKernelSource)
	v := New(runtime.NewInterpreterGlobals())
	defer v.Close()
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("runtime error: %v", err)
	}

	builderFn := v.GetGlobal("makeTree")
	builderCl, ok := closureFromValue(builderFn)
	if !ok {
		t.Fatalf("makeTree global is not a VM closure: %s", builderFn.TypeName())
	}
	v.SetGlobal("makeTree", runtime.FunctionValue(&runtime.GoFunction{
		Name: "replacement_makeTree",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			return []runtime.Value{runtime.IntValue(41)}, nil
		},
	}))
	handled, _, err := v.tryRunValueRuntimeSpecialization(builderCl, []runtime.Value{runtime.IntValue(1)})
	if err != nil {
		t.Fatalf("builder runtime specialization returned error: %v", err)
	}
	if handled {
		t.Fatal("builder runtime specialization handled call after self global changed")
	}
	results, err := v.CallValue(builderFn, []runtime.Value{runtime.IntValue(1)})
	if err != nil {
		t.Fatalf("builder fallback call error: %v", err)
	}
	if len(results) != 1 || !results[0].IsTable() {
		t.Fatalf("builder fallback result = %+v, want table", results)
	}
	left := results[0].Table().RawGetString("left")
	if !left.IsInt() || left.Int() != 41 {
		t.Fatalf("builder fallback left child = %v, want 41", left)
	}

	foldFn := v.GetGlobal("checkTree")
	foldCl, ok := closureFromValue(foldFn)
	if !ok {
		t.Fatalf("checkTree global is not a VM closure: %s", foldFn.TypeName())
	}
	v.SetGlobal("checkTree", runtime.FunctionValue(&runtime.GoFunction{
		Name: "replacement_checkTree",
		Fn: func(args []runtime.Value) ([]runtime.Value, error) {
			return []runtime.Value{runtime.IntValue(10)}, nil
		},
	}))
	node := runtime.NewTable()
	node.RawSetString("left", runtime.TableValue(runtime.NewTable()))
	node.RawSetString("right", runtime.TableValue(runtime.NewTable()))
	handled, _, err = v.tryRunValueRuntimeSpecialization(foldCl, []runtime.Value{runtime.TableValue(node)})
	if err != nil {
		t.Fatalf("fold runtime specialization returned error: %v", err)
	}
	if handled {
		t.Fatal("fold runtime specialization handled call after self global changed")
	}
	results, err = v.CallValue(foldFn, []runtime.Value{runtime.TableValue(node)})
	if err != nil {
		t.Fatalf("fold fallback call error: %v", err)
	}
	if len(results) != 1 || !results[0].IsInt() || results[0].Int() != 21 {
		t.Fatalf("fold fallback result = %+v, want 21", results)
	}
}

func TestRecursiveTableRuntimeSpecializationRecordsHitsOnce(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, recursiveTableKernelSource+`
root := makeTree(4)
result := checkTree(root)
`)
	expectGlobalInt(t, globals, "result", 31)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteWholeCallValue, "lazy_recursive_table_builder"); got != 1 {
		t.Fatalf("builder structural hit count = %d, want 1", got)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteWholeCallValue, "lazy_recursive_table_fold"); got != 1 {
		t.Fatalf("fold structural hit count = %d, want 1", got)
	}
	if got := stats.Snapshot().RuntimeSpecialization.Total; got != 2 {
		t.Fatalf("total structural hit count = %d, want 2", got)
	}
}

func TestRecursiveTableNonRecursiveWholeCallEntryDoesNotTrigger(t *testing.T) {
	top := compileProto(t, recursiveTableKernelSource)
	v := New(runtime.NewInterpreterGlobals())
	defer v.Close()
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("runtime error: %v", err)
	}

	builderFn := v.GetGlobal("makeTree")
	builderCl, ok := closureFromValue(builderFn)
	if !ok {
		t.Fatalf("makeTree global is not a VM closure: %s", builderFn.TypeName())
	}
	handled, _, err := v.tryRunNonRecursiveTableValueRuntimeSpecialization(builderCl, []runtime.Value{runtime.IntValue(2)})
	if err != nil {
		t.Fatalf("non-recursive builder dispatch returned error: %v", err)
	}
	if handled {
		t.Fatal("non-recursive dispatch handled recursive table builder")
	}

	foldFn := v.GetGlobal("checkTree")
	foldCl, ok := closureFromValue(foldFn)
	if !ok {
		t.Fatalf("checkTree global is not a VM closure: %s", foldFn.TypeName())
	}
	builderCache := recursiveTableKernelForProto(builderCl.Proto)
	if builderCache.builder == nil {
		t.Fatal("missing builder cache")
	}
	root := runtime.FreshTableValue(runtime.NewLazyRecursiveTable(&builderCache.builder.ctor, 2))
	handled, _, err = v.tryRunNonRecursiveTableValueRuntimeSpecialization(foldCl, []runtime.Value{root})
	if err != nil {
		t.Fatalf("non-recursive fold dispatch returned error: %v", err)
	}
	if handled {
		t.Fatal("non-recursive dispatch handled recursive table fold")
	}
}

func TestRecursiveTableBuildFoldRegionStillHandlesCombinedCall(t *testing.T) {
	globals := compileAndRun(t, recursiveTableKernelSource+`
result := checkTree(makeTree(4))
`)
	expectGlobalInt(t, globals, "result", 31)
}
