package methodjit

import (
	"reflect"
	"testing"

	"github.com/gscript/gscript/internal/vm"
)

func TestNewAnalysisResultInitializesMapsAndPreservesNilSentinels(t *testing.T) {
	a := NewAnalysisResult()

	assertAnalysisResultMapSentinels(t, a, "NewAnalysisResult")
}

func TestAnalysisResultInitializeZeroValuePreservesNilSentinels(t *testing.T) {
	var a AnalysisResult

	a.Initialize()

	assertAnalysisResultMapSentinels(t, &a, "Initialize")
}

func TestAnalysisResultInitializePreservesExplicitSentinelMaps(t *testing.T) {
	callee := &vm.FuncProto{Name: "callee"}
	a := &AnalysisResult{
		Globals: map[string]*vm.FuncProto{
			"callee": callee,
		},
		SuppressedSpecGuardKinds: map[int]map[string]bool{
			12: {"GuardCalleeProto": true},
		},
		Int48Safe: map[int]bool{7: true},
	}

	a.Initialize()

	if got := a.Globals["callee"]; got != callee {
		t.Fatalf("Initialize replaced or lost Globals entry: got %p want %p", got, callee)
	}
	if !a.SuppressedSpecGuardKinds[12]["GuardCalleeProto"] {
		t.Fatalf("Initialize replaced or lost SuppressedSpecGuardKinds entry")
	}
	if !a.Int48Safe[7] {
		t.Fatalf("Initialize replaced or lost ordinary analysis map entry")
	}
}

func TestAnalysisResultNumericFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	numeric := a.NumericFacts()

	numeric.SetInt48Safe(map[int]bool{11: true})
	numeric.SetIntModNonZeroDivisor(map[int]bool{12: true})
	numeric.SetIntModNoSignAdjust(map[int]bool{13: true})
	numeric.SetIntRanges(map[int]intRange{14: pointRange(5)})
	numeric.SetIntNonNegative(map[int]bool{15: true})
	numeric.RecordProfiledIntRange(16, intRange{min: 1, max: 3, known: true})
	numeric.RecordProfiledLenRange(17, intRange{min: 2, max: 4, known: true})

	if !a.Int48Safe[11] || !a.IntModNonZeroDivisor[12] || !a.IntModNoSignAdjust[13] || !a.IntNonNegative[15] {
		t.Fatalf("NumericFacts boolean mutators did not update compatibility fields")
	}
	if got, ok := a.IntRanges[14]; !ok || !got.known || got.min != 5 || got.max != 5 {
		t.Fatalf("NumericFacts.SetIntRanges did not update compatibility field: got %#v ok=%v", got, ok)
	}
	if got, ok := a.ProfiledIntRanges[16]; !ok || !got.known || got.min != 1 || got.max != 3 {
		t.Fatalf("NumericFacts.RecordProfiledIntRange did not update compatibility field: got %#v ok=%v", got, ok)
	}
	if got, ok := a.ProfiledLenRanges[17]; !ok || !got.known || got.min != 2 || got.max != 4 {
		t.Fatalf("NumericFacts.RecordProfiledLenRange did not update compatibility field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultNumericFactsAdoptsLegacyFields(t *testing.T) {
	a := &AnalysisResult{
		Int48Safe:            map[int]bool{21: true},
		IntModNonZeroDivisor: map[int]bool{22: true},
		IntModNoSignAdjust:   map[int]bool{23: true},
		IntRanges:            map[int]intRange{24: pointRange(6)},
		ProfiledIntRanges:    map[int]intRange{25: {min: 1, max: 9, known: true}},
		ProfiledLenRanges:    map[int]intRange{26: {min: 2, max: 8, known: true}},
		IntNonNegative:       map[int]bool{27: true},
	}

	numeric := a.NumericFacts()
	if !numeric.IsInt48Safe(21) || !numeric.IsIntModNonZeroDivisor(22) || !numeric.IsIntModNoSignAdjust(23) || !numeric.IsIntNonNegative(27) {
		t.Fatalf("NumericFacts did not adopt legacy boolean fields")
	}
	if got, ok := numeric.IntRange(24); !ok || !got.known || got.min != 6 || got.max != 6 {
		t.Fatalf("NumericFacts did not adopt legacy IntRanges: got %#v ok=%v", got, ok)
	}
	if got, ok := numeric.ProfiledIntRange(25); !ok || !got.known || got.min != 1 || got.max != 9 {
		t.Fatalf("NumericFacts did not adopt legacy ProfiledIntRanges: got %#v ok=%v", got, ok)
	}
	if got, ok := numeric.ProfiledLenRange(26); !ok || !got.known || got.min != 2 || got.max != 8 {
		t.Fatalf("NumericFacts did not adopt legacy ProfiledLenRanges: got %#v ok=%v", got, ok)
	}
}

func TestNumericFactsHelpersAreNilSafe(t *testing.T) {
	var numeric *NumericFacts
	if numeric.IsInt48Safe(1) || numeric.IsIntModNonZeroDivisor(1) || numeric.IsIntModNoSignAdjust(1) || numeric.IsIntNonNegative(1) {
		t.Fatalf("nil NumericFacts reported boolean facts")
	}
	if _, ok := numeric.IntRange(1); ok {
		t.Fatalf("nil NumericFacts reported IntRange")
	}
	if _, ok := numeric.ProfiledIntRange(1); ok {
		t.Fatalf("nil NumericFacts reported ProfiledIntRange")
	}
	if _, ok := numeric.ProfiledLenRange(1); ok {
		t.Fatalf("nil NumericFacts reported ProfiledLenRange")
	}
	numeric.SetComputedRanges(map[int]bool{1: true}, map[int]intRange{1: pointRange(1)}, map[int]bool{1: true})
	numeric.SetModuloFacts(map[int]bool{1: true}, map[int]bool{1: true})
	numeric.RecordProfiledIntRange(1, pointRange(1))
	numeric.RecordProfiledLenRange(1, pointRange(1))
}

func TestAnalysisResultNumericFactsRebindsAfterLegacyMutation(t *testing.T) {
	a := NewAnalysisResult()
	numeric := a.NumericFacts()
	a.IntRanges = map[int]intRange{31: pointRange(7)}
	a.Int48Safe = map[int]bool{32: true}

	numeric = a.NumericFacts()
	if got, ok := numeric.IntRange(31); !ok || !got.known || got.min != 7 || got.max != 7 {
		t.Fatalf("NumericFacts did not rebind legacy IntRanges: got %#v ok=%v", got, ok)
	}
	if !numeric.IsInt48Safe(32) {
		t.Fatalf("NumericFacts did not rebind legacy Int48Safe")
	}
}

func TestAnalysisResultCallFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	calls := a.CallFacts()

	desc := CallABIDescriptor{NumArgs: 2, NumRets: 1}
	calls.SetCallABIs(map[int]CallABIDescriptor{11: desc})
	if got, ok := a.CallABIs[11]; !ok || got.NumArgs != desc.NumArgs || got.NumRets != desc.NumRets {
		t.Fatalf("CallFacts.SetCallABIs did not update compatibility field: got %#v ok=%v", got, ok)
	}

	calls.SetRuntimeSpecializationConstCallFolds(map[int]RuntimeSpecializationConstCallFoldFact{12: {Result: 99}})
	if got, ok := a.RuntimeSpecializationConstCallFolds[12]; !ok || got.Result != 99 {
		t.Fatalf("CallFacts.SetRuntimeSpecializationConstCallFolds did not update compatibility field: got %#v ok=%v", got, ok)
	}

	calls.SetWholeCallNoResultRuntimeSpecializations(map[int]bool{13: true})
	if !a.WholeCallNoResultRuntimeSpecializations[13] {
		t.Fatalf("CallFacts.SetWholeCallNoResultRuntimeSpecializations did not update compatibility field")
	}

	calls.SetWholeCallNoResultRuntimeSpecializationBatches(map[int]WholeCallNoResultRuntimeSpecializationBatchFact{13: {ExitPC: 44}})
	if got, ok := a.WholeCallNoResultRuntimeSpecializationBatches[13]; !ok || got.ExitPC != 44 {
		t.Fatalf("CallFacts.SetWholeCallNoResultRuntimeSpecializationBatches did not update compatibility field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultKernelFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	kernels := a.KernelFacts()

	kernels.SetTableArrayUpperBoundSafe(map[int]bool{11: true})
	if !a.TableArrayUpperBoundSafe[11] {
		t.Fatalf("KernelFacts.SetTableArrayUpperBoundSafe did not update compatibility field")
	}

	kernels.SetTableArrayLowerBoundSafe(map[int]bool{12: true})
	if !a.TableArrayLowerBoundSafe[12] {
		t.Fatalf("KernelFacts.SetTableArrayLowerBoundSafe did not update compatibility field")
	}

	loopFact := LoopTableArrayFact{HeaderBlockID: 1, PreheaderBlockID: 2, AccessOp: OpTableArrayLoad}
	kernels.SetLoopTableArrayFacts(map[int]LoopTableArrayFact{13: loopFact})
	if got, ok := a.LoopTableArrayFacts[13]; !ok || got.HeaderBlockID != 1 || got.AccessOp != OpTableArrayLoad {
		t.Fatalf("KernelFacts.SetLoopTableArrayFacts did not update compatibility field: got %#v ok=%v", got, ok)
	}

	spec := RecordArrayLoopKernelSpec{ShapeID: 99, ScalarCount: 1}
	kernels.SetRecordArrayLoopKernels(map[int]RecordArrayLoopKernelSpec{14: spec})
	if got, ok := a.RecordArrayLoopKernels[14]; !ok || got.ShapeID != 99 || got.ScalarCount != 1 {
		t.Fatalf("KernelFacts.SetRecordArrayLoopKernels did not update compatibility field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultKernelFactsAdoptsLegacyFields(t *testing.T) {
	loopFact := LoopTableArrayFact{HeaderBlockID: 3, PreheaderBlockID: 4, AccessOp: OpTableArrayStore}
	spec := RecordArrayLoopKernelSpec{ShapeID: 77}
	a := &AnalysisResult{
		TableArrayUpperBoundSafe: map[int]bool{21: true},
		TableArrayLowerBoundSafe: map[int]bool{22: true},
		LoopTableArrayFacts:      map[int]LoopTableArrayFact{23: loopFact},
		RecordArrayLoopKernels:   map[int]RecordArrayLoopKernelSpec{24: spec},
	}

	kernels := a.KernelFacts()
	if !kernels.TableArrayUpperBoundIsSafe(21) {
		t.Fatalf("KernelFacts did not adopt legacy TableArrayUpperBoundSafe")
	}
	if !kernels.TableArrayLowerBoundIsSafe(22) {
		t.Fatalf("KernelFacts did not adopt legacy TableArrayLowerBoundSafe")
	}
	if got, ok := kernels.LoopTableArrayFact(23); !ok || got.HeaderBlockID != 3 || got.AccessOp != OpTableArrayStore {
		t.Fatalf("KernelFacts did not adopt legacy LoopTableArrayFacts: got %#v ok=%v", got, ok)
	}
	if got, ok := kernels.RecordArrayLoopKernel(24); !ok || got.ShapeID != 77 {
		t.Fatalf("KernelFacts did not adopt legacy RecordArrayLoopKernels: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultCallFactsAdoptsLegacyFields(t *testing.T) {
	desc := CallABIDescriptor{NumArgs: 3, NumRets: 1}
	a := &AnalysisResult{
		CallABIs:                                      map[int]CallABIDescriptor{21: desc},
		RuntimeSpecializationConstCallFolds:           map[int]RuntimeSpecializationConstCallFoldFact{22: {Result: 7}},
		WholeCallNoResultRuntimeSpecializations:       map[int]bool{23: true},
		WholeCallNoResultRuntimeSpecializationBatches: map[int]WholeCallNoResultRuntimeSpecializationBatchFact{23: {ExitPC: 8}},
	}

	calls := a.CallFacts()
	if got, ok := calls.CallABI(21); !ok || got.NumArgs != desc.NumArgs {
		t.Fatalf("CallFacts did not adopt legacy CallABIs: got %#v ok=%v", got, ok)
	}
	if got, ok := calls.RuntimeSpecializationConstCallFold(22); !ok || got.Result != 7 {
		t.Fatalf("CallFacts did not adopt legacy RuntimeSpecializationConstCallFolds: got %#v ok=%v", got, ok)
	}
	if !calls.WholeCallNoResultRuntimeSpecialization(23) {
		t.Fatalf("CallFacts did not adopt compatibility WholeCallNoResultRuntimeSpecializations")
	}
	if got, ok := calls.WholeCallNoResultRuntimeSpecializationBatch(23); !ok || got.ExitPC != 8 {
		t.Fatalf("CallFacts did not adopt compatibility WholeCallNoResultRuntimeSpecializationBatches: got %#v ok=%v", got, ok)
	}
}

func TestCallFactsReadHelpersAreNilSafe(t *testing.T) {
	var calls *CallFacts
	if calls.CallABICount() != 0 {
		t.Fatalf("nil CallFacts reported CallABI facts")
	}
	if calls.RuntimeSpecializationConstCallFoldCount() != 0 {
		t.Fatalf("nil CallFacts reported runtime specialization const call facts")
	}
	if calls.WholeCallNoResultRuntimeSpecializationBatchMap() != nil {
		t.Fatalf("nil CallFacts returned whole-call no-result batches")
	}

	visitedCallABI := false
	calls.ForEachCallABI(func(int, CallABIDescriptor) bool {
		visitedCallABI = true
		return true
	})
	if visitedCallABI {
		t.Fatalf("nil CallFacts visited CallABI facts")
	}

	visitedProtocolFold := false
	calls.ForEachRuntimeSpecializationConstCallFold(func(int, RuntimeSpecializationConstCallFoldFact) bool {
		visitedProtocolFold = true
		return true
	})
	if visitedProtocolFold {
		t.Fatalf("nil CallFacts visited runtime specialization const call facts")
	}
}

func TestAnalysisResultCallFactsRebindsAfterLegacyMutation(t *testing.T) {
	a := NewAnalysisResult()
	calls := a.CallFacts()
	a.CallABIs = map[int]CallABIDescriptor{31: {NumArgs: 4}}
	a.RuntimeSpecializationConstCallFolds = map[int]RuntimeSpecializationConstCallFoldFact{32: {Result: 12}}
	a.WholeCallNoResultRuntimeSpecializations = map[int]bool{33: true}
	a.WholeCallNoResultRuntimeSpecializationBatches = map[int]WholeCallNoResultRuntimeSpecializationBatchFact{34: {ExitPC: 55}}

	calls = a.CallFacts()
	if got, ok := calls.CallABI(31); !ok || got.NumArgs != 4 {
		t.Fatalf("CallFacts did not rebind legacy CallABIs: got %#v ok=%v", got, ok)
	}
	if got, ok := calls.RuntimeSpecializationConstCallFold(32); !ok || got.Result != 12 {
		t.Fatalf("CallFacts did not rebind legacy RuntimeSpecializationConstCallFolds: got %#v ok=%v", got, ok)
	}
	if !calls.WholeCallNoResultRuntimeSpecialization(33) {
		t.Fatalf("CallFacts did not rebind compatibility WholeCallNoResultRuntimeSpecializations")
	}
	if got, ok := calls.WholeCallNoResultRuntimeSpecializationBatch(34); !ok || got.ExitPC != 55 {
		t.Fatalf("CallFacts did not rebind compatibility WholeCallNoResultRuntimeSpecializationBatches: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultCallFactsPreservesDomainMapsWhenLegacyFieldsNil(t *testing.T) {
	a := &AnalysisResult{Call: NewCallFacts()}
	a.Call.CallABIs[41] = CallABIDescriptor{NumArgs: 5}
	a.Call.RuntimeSpecializationConstCallFolds[42] = RuntimeSpecializationConstCallFoldFact{Result: 13}
	a.Call.WholeCallNoResultRuntimeSpecializations[43] = true
	a.Call.WholeCallNoResultRuntimeSpecializationBatches[44] = WholeCallNoResultRuntimeSpecializationBatchFact{ExitPC: 66}

	calls := a.CallFacts()
	if got, ok := calls.CallABI(41); !ok || got.NumArgs != 5 {
		t.Fatalf("CallFacts lost domain CallABIs with nil legacy field: got %#v ok=%v", got, ok)
	}
	if got, ok := calls.RuntimeSpecializationConstCallFold(42); !ok || got.Result != 13 {
		t.Fatalf("CallFacts lost domain RuntimeSpecializationConstCallFolds with nil legacy field: got %#v ok=%v", got, ok)
	}
	if !calls.WholeCallNoResultRuntimeSpecialization(43) {
		t.Fatalf("CallFacts lost domain WholeCallNoResultRuntimeSpecializations with nil legacy field")
	}
	if got, ok := calls.WholeCallNoResultRuntimeSpecializationBatch(44); !ok || got.ExitPC != 66 {
		t.Fatalf("CallFacts lost domain WholeCallNoResultRuntimeSpecializationBatches with nil legacy field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultSpeculationFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	spec := a.SpeculationFacts()
	callee := &vm.FuncProto{Name: "callee"}

	spec.SetSpecDependencyProtos(map[*vm.FuncProto]bool{callee: true})
	if !a.SpecDependencyProtos[callee] {
		t.Fatalf("SpeculationFacts.SetSpecDependencyProtos did not update compatibility field")
	}

	spec.SetSuppressedSpecGuardPCs(map[int]bool{31: true})
	if !a.SuppressedSpecGuardPCs[31] {
		t.Fatalf("SpeculationFacts.SetSuppressedSpecGuardPCs did not update compatibility field")
	}

	spec.SetSuppressedSpecGuardKinds(map[int]map[string]bool{
		32: {"GuardType": true},
	})
	if !a.SuppressedSpecGuardKinds[32]["GuardType"] {
		t.Fatalf("SpeculationFacts.SetSuppressedSpecGuardKinds did not update compatibility field")
	}
}

func TestAnalysisResultSpeculationFactsAdoptsLegacyFields(t *testing.T) {
	callee := &vm.FuncProto{Name: "callee"}
	a := &AnalysisResult{
		SpecDependencyProtos:     map[*vm.FuncProto]bool{callee: true},
		SuppressedSpecGuardPCs:   map[int]bool{41: true},
		SuppressedSpecGuardKinds: map[int]map[string]bool{42: {"GuardCalleeProto": true}},
	}

	spec := a.SpeculationFacts()
	if !spec.SpecDependencyProtos[callee] {
		t.Fatalf("SpeculationFacts did not adopt legacy SpecDependencyProtos")
	}
	if !spec.SuppressedSpecGuardPCs[41] {
		t.Fatalf("SpeculationFacts did not adopt legacy SuppressedSpecGuardPCs")
	}
	if !spec.SuppressedSpecGuardKinds[42]["GuardCalleeProto"] {
		t.Fatalf("SpeculationFacts did not adopt legacy SuppressedSpecGuardKinds")
	}
}

func TestAnalysisResultSpeculationFactsPreservesSuppressedKindsNilSentinel(t *testing.T) {
	a := NewAnalysisResult()
	spec := a.SpeculationFacts()

	if spec.SuppressedSpecGuardKinds != nil || a.SuppressedSpecGuardKinds != nil {
		t.Fatalf("NewAnalysisResult initialized SuppressedSpecGuardKinds nil sentinel")
	}

	spec.SetSuppressedSpecGuardKinds(map[int]map[string]bool{})
	if specGuardKindSuppressed(&Function{Analysis: a}, 51, "GuardType") {
		t.Fatalf("empty non-nil SuppressedSpecGuardKinds should not fall back to SuppressedSpecGuardPCs")
	}

	a.SuppressedSpecGuardPCs = map[int]bool{51: true}
	a.SuppressedSpecGuardKinds = nil
	if !specGuardKindSuppressed(&Function{Analysis: a}, 51, "GuardType") {
		t.Fatalf("legacy nil SuppressedSpecGuardKinds sentinel should restore PC fallback")
	}
}

func TestAnalysisResultTableShapeFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	shapes := a.TableShapeFacts()

	cases := []FieldPolyShapeCase{{ShapeID: 11, FieldIdx: 2}}
	shapes.SetFieldPolyShapeFacts(map[int][]FieldPolyShapeCase{101: cases})
	if got, ok := a.FieldPolyShapeFacts[101]; !ok || len(got) != 1 || got[0].ShapeID != 11 {
		t.Fatalf("TableShapeFacts.SetFieldPolyShapeFacts did not update compatibility field: got %#v ok=%v", got, ok)
	}

	shapes.SetFieldPolyShapeReceivers(map[int]bool{102: true})
	if !a.FieldPolyShapeReceivers[102] {
		t.Fatalf("TableShapeFacts.SetFieldPolyShapeReceivers did not update compatibility field")
	}

	shapes.SetFieldPolyShapeCatalog(map[uint32]FixedShapeTableFact{12: {ShapeID: 12}})
	if got, ok := a.FieldPolyShapeCatalog[12]; !ok || got.ShapeID != 12 {
		t.Fatalf("TableShapeFacts.SetFieldPolyShapeCatalog did not update compatibility field: got %#v ok=%v", got, ok)
	}

	fusions := []FieldCallPolyLenFusion{{LenValueID: 103, ShapeID: 13, Len: 4}}
	shapes.SetFieldCallPolyLenFusions(map[int][]FieldCallPolyLenFusion{104: fusions})
	if got, ok := a.FieldCallPolyLenFusions[104]; !ok || len(got) != 1 || got[0].LenValueID != 103 {
		t.Fatalf("TableShapeFacts.SetFieldCallPolyLenFusions did not update compatibility field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultTableShapeFactsRecordHelpersBindLegacyMaps(t *testing.T) {
	a := NewAnalysisResult()
	shapes := a.TableShapeFacts()

	cases := []FieldPolyShapeCase{{
		ShapeID:  51,
		FieldIdx: 2,
		ReceiverFact: FixedShapeTableFact{
			ShapeID:    51,
			FieldNames: []string{"step"},
		},
	}}
	shapes.RecordFieldPolyShapeCases(501, cases)
	if got := a.FieldPolyShapeFacts[501]; len(got) != 1 || got[0].ShapeID != 51 {
		t.Fatalf("RecordFieldPolyShapeCases did not update compatibility field: got %#v", got)
	}
	if got, ok := a.FieldPolyShapeCatalog[51]; !ok || got.ShapeID != 51 || len(got.FieldNames) != 1 || got.FieldNames[0] != "step" {
		t.Fatalf("RecordFieldPolyShapeCases did not record catalog clone: got %#v ok=%v", got, ok)
	}
	cases[0].ReceiverFact.FieldNames[0] = "mutated"
	if got := a.FieldPolyShapeCatalog[51]; got.FieldNames[0] != "step" {
		t.Fatalf("catalog fact should be cloned, got %#v", got.FieldNames)
	}

	shapes.RecordFieldPolyShapeReceiver(502)
	if !a.FieldPolyShapeReceivers[502] {
		t.Fatalf("RecordFieldPolyShapeReceiver did not update compatibility field")
	}

	shapes.RecordFieldCallPolyLenFusions(503, []FieldCallPolyLenFusion{{LenValueID: 504, ShapeID: 51, Len: 7}})
	if got := a.FieldCallPolyLenFusions[503]; len(got) != 1 || got[0].Len != 7 {
		t.Fatalf("RecordFieldCallPolyLenFusions did not update compatibility field: got %#v", got)
	}

	shapes.DeleteFieldPolyShapeCases(501)
	if _, ok := a.FieldPolyShapeFacts[501]; ok {
		t.Fatalf("DeleteFieldPolyShapeCases did not update compatibility field")
	}
}

func TestAnalysisResultTableShapeFactsAdoptsLegacyFields(t *testing.T) {
	a := &AnalysisResult{
		FieldPolyShapeFacts:     map[int][]FieldPolyShapeCase{201: {{ShapeID: 21, FieldIdx: 1}}},
		FieldPolyShapeReceivers: map[int]bool{202: true},
		FieldPolyShapeCatalog:   map[uint32]FixedShapeTableFact{22: {ShapeID: 22}},
		FieldCallPolyLenFusions: map[int][]FieldCallPolyLenFusion{203: {{LenValueID: 204, ShapeID: 23}}},
	}

	shapes := a.TableShapeFacts()
	if got, ok := shapes.FieldPolyShapeCases(201); !ok || len(got) != 1 || got[0].ShapeID != 21 {
		t.Fatalf("TableShapeFacts did not adopt legacy FieldPolyShapeFacts: got %#v ok=%v", got, ok)
	}
	if !shapes.FieldPolyShapeReceiver(202) {
		t.Fatalf("TableShapeFacts did not adopt legacy FieldPolyShapeReceivers")
	}
	if got, ok := shapes.FieldPolyShapeCatalogFact(22); !ok || got.ShapeID != 22 {
		t.Fatalf("TableShapeFacts did not adopt legacy FieldPolyShapeCatalog: got %#v ok=%v", got, ok)
	}
	if got, ok := shapes.FieldCallPolyLenFusionCases(203); !ok || len(got) != 1 || got[0].LenValueID != 204 {
		t.Fatalf("TableShapeFacts did not adopt legacy FieldCallPolyLenFusions: got %#v ok=%v", got, ok)
	}
}

func TestTableShapeFactsReadHelpers(t *testing.T) {
	a := &AnalysisResult{
		FieldPolyShapeFacts: map[int][]FieldPolyShapeCase{
			301: {
				{ShapeID: 31, FieldIdx: 1},
				{ShapeID: 32, FieldIdx: 2},
			},
		},
		FieldPolyShapeCatalog: map[uint32]FixedShapeTableFact{
			33: {ShapeID: 33},
		},
	}
	shapes := a.TableShapeFacts()

	if !shapes.HasFieldPolyShapeCases(301) {
		t.Fatalf("HasFieldPolyShapeCases returned false for populated cases")
	}
	if shapes.HasFieldPolyShapeCases(302) {
		t.Fatalf("HasFieldPolyShapeCases returned true for missing cases")
	}

	visitedCases := 0
	shapes.ForEachFieldPolyShapeCase(func(id int, c FieldPolyShapeCase) bool {
		if id != 301 {
			t.Fatalf("visited unexpected field poly shape id: %d", id)
		}
		visitedCases++
		return visitedCases < 1
	})
	if visitedCases != 1 {
		t.Fatalf("ForEachFieldPolyShapeCase short-circuit visits=%d want 1", visitedCases)
	}

	visitedCatalog := 0
	shapes.ForEachFieldPolyShapeCatalogFact(func(shapeID uint32, fact FixedShapeTableFact) bool {
		if shapeID != 33 || fact.ShapeID != 33 {
			t.Fatalf("visited unexpected catalog fact: shapeID=%d fact=%#v", shapeID, fact)
		}
		visitedCatalog++
		return true
	})
	if visitedCatalog != 1 {
		t.Fatalf("ForEachFieldPolyShapeCatalogFact visits=%d want 1", visitedCatalog)
	}

	if functionTableShapeFacts(nil).HasFieldPolyShapeCases(301) {
		t.Fatalf("nil function table shape helper returned cases")
	}
}

func TestAnalysisResultTableShapeFactsRebindsAfterLegacyMutation(t *testing.T) {
	a := NewAnalysisResult()
	shapes := a.TableShapeFacts()

	a.FieldPolyShapeFacts = map[int][]FieldPolyShapeCase{301: {{ShapeID: 31}}}
	shapes = a.TableShapeFacts()
	if got, ok := shapes.FieldPolyShapeCases(301); !ok || len(got) != 1 || got[0].ShapeID != 31 {
		t.Fatalf("TableShapeFacts did not rebind from mutated legacy FieldPolyShapeFacts: got %#v ok=%v", got, ok)
	}

	shapes.SetFieldPolyShapeFacts(map[int][]FieldPolyShapeCase{302: {{ShapeID: 32}}})
	if _, ok := a.FieldPolyShapeFacts[301]; ok {
		t.Fatalf("TableShapeFacts.SetFieldPolyShapeFacts did not replace legacy compatibility field")
	}
	if got, ok := a.FieldPolyShapeFacts[302]; !ok || len(got) != 1 || got[0].ShapeID != 32 {
		t.Fatalf("TableShapeFacts.SetFieldPolyShapeFacts did not rebind legacy compatibility field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultTableShapeFactsPreservesEmptyMapSentinelBehavior(t *testing.T) {
	a := &AnalysisResult{
		FieldPolyShapeFacts: map[int][]FieldPolyShapeCase{401: {{ShapeID: 41}}},
	}
	shapes := a.TableShapeFacts()

	shapes.SetFieldPolyShapeFacts(map[int][]FieldPolyShapeCase{})
	if _, ok := shapes.FieldPolyShapeCases(401); ok {
		t.Fatalf("empty non-nil FieldPolyShapeFacts should not fall back to previous legacy entries")
	}
	if len(a.FieldPolyShapeFacts) != 0 {
		t.Fatalf("empty non-nil FieldPolyShapeFacts should remain bound to legacy field: got %#v", a.FieldPolyShapeFacts)
	}

	shapes.SetFieldCallPolyLenFusions(map[int][]FieldCallPolyLenFusion{})
	if got := len(a.FieldCallPolyLenFusions); got != 0 {
		t.Fatalf("empty non-nil FieldCallPolyLenFusions should remain bound to legacy field: len=%d", got)
	}
}

func TestSpecGuardKindSuppressedNilKindsFallsBackToPCs(t *testing.T) {
	fn := &Function{Analysis: &AnalysisResult{
		SuppressedSpecGuardPCs: map[int]bool{42: true},
	}}

	if !specGuardKindSuppressed(fn, 42, "GuardCalleeProto") {
		t.Fatalf("nil SuppressedSpecGuardKinds should fall back to SuppressedSpecGuardPCs")
	}

	fn.Analysis.SuppressedSpecGuardKinds = map[int]map[string]bool{}
	if specGuardKindSuppressed(fn, 42, "GuardCalleeProto") {
		t.Fatalf("empty non-nil SuppressedSpecGuardKinds should not fall back to SuppressedSpecGuardPCs")
	}
}

func assertAnalysisResultMapSentinels(t *testing.T, a *AnalysisResult, constructor string) {
	t.Helper()

	v := reflect.ValueOf(a).Elem()
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fieldValue := v.Field(i)
		if fieldValue.Kind() != reflect.Map {
			continue
		}

		name := typ.Field(i).Name
		if name == "Globals" || name == "SuppressedSpecGuardKinds" {
			if !fieldValue.IsNil() {
				t.Fatalf("%s initialized nil sentinel field %s", constructor, name)
			}
			continue
		}

		if fieldValue.IsNil() {
			t.Fatalf("%s left analysis map %s nil", constructor, name)
		}
	}
}
