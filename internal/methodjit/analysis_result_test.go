package methodjit

import (
	"reflect"
	"testing"

	"github.com/never-labs/gscript/internal/vm"
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
		Global: newGlobalFactsForTest(globalFactsSeed{
			Globals: map[string]*vm.FuncProto{
				"callee": callee,
			},
		}),
		Speculation: newSpeculationFactsForTest(speculationFactsSeed{
			SuppressedSpecGuardKinds: map[int]map[string]bool{
				12: {"GuardCalleeProto": true},
			},
		}),
		Numeric: newNumericFactsForTest(numericFactsSeed{
			Int48Safe: map[int]bool{7: true},
		}),
	}

	a.Initialize()

	if got, _ := a.GlobalFacts().GlobalProto("callee"); got != callee {
		t.Fatalf("Initialize replaced or lost Globals entry: got %p want %p", got, callee)
	}
	if !a.SpeculationFacts().SpecGuardKindSuppressed(12, "GuardCalleeProto") {
		t.Fatalf("Initialize replaced or lost SuppressedSpecGuardKinds entry")
	}
	if !a.NumericFacts().IsInt48Safe(7) {
		t.Fatalf("Initialize replaced or lost ordinary analysis map entry")
	}
}

func TestAnalysisResultNumericFactsMutatorsUpdateDomain(t *testing.T) {
	a := NewAnalysisResult()
	numeric := a.NumericFacts()

	numeric.SetInt48Safe(map[int]bool{11: true})
	numeric.SetIntModNonZeroDivisor(map[int]bool{12: true})
	numeric.SetIntModNoSignAdjust(map[int]bool{13: true})
	numeric.SetIntRanges(map[int]intRange{14: pointRange(5)})
	numeric.SetIntNonNegative(map[int]bool{15: true})
	numeric.RecordProfiledIntRange(16, intRange{min: 1, max: 3, known: true})
	numeric.RecordProfiledLenRange(17, intRange{min: 2, max: 4, known: true})

	if !numeric.IsInt48Safe(11) || !numeric.IsIntModNonZeroDivisor(12) || !numeric.IsIntModNoSignAdjust(13) || !numeric.IsIntNonNegative(15) {
		t.Fatalf("NumericFacts boolean mutators did not update domain fields")
	}
	if got, ok := numeric.IntRange(14); !ok || !got.known || got.min != 5 || got.max != 5 {
		t.Fatalf("NumericFacts.SetIntRanges did not update domain field: got %#v ok=%v", got, ok)
	}
	if got, ok := numeric.ProfiledIntRange(16); !ok || !got.known || got.min != 1 || got.max != 3 {
		t.Fatalf("NumericFacts.RecordProfiledIntRange did not update domain field: got %#v ok=%v", got, ok)
	}
	if got, ok := numeric.ProfiledLenRange(17); !ok || !got.known || got.min != 2 || got.max != 4 {
		t.Fatalf("NumericFacts.RecordProfiledLenRange did not update domain field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultNumericFactsPreservesPrePopulatedDomain(t *testing.T) {
	a := &AnalysisResult{
		Numeric: newNumericFactsForTest(numericFactsSeed{
			Int48Safe:            map[int]bool{21: true},
			IntModNonZeroDivisor: map[int]bool{22: true},
			IntModNoSignAdjust:   map[int]bool{23: true},
			IntRanges:            map[int]intRange{24: pointRange(6)},
			ProfiledIntRanges:    map[int]intRange{25: {min: 1, max: 9, known: true}},
			ProfiledLenRanges:    map[int]intRange{26: {min: 2, max: 8, known: true}},
			IntNonNegative:       map[int]bool{27: true},
		}),
	}

	numeric := a.NumericFacts()
	if !numeric.IsInt48Safe(21) || !numeric.IsIntModNonZeroDivisor(22) || !numeric.IsIntModNoSignAdjust(23) || !numeric.IsIntNonNegative(27) {
		t.Fatalf("NumericFacts did not preserve pre-populated boolean fields")
	}
	if got, ok := numeric.IntRange(24); !ok || !got.known || got.min != 6 || got.max != 6 {
		t.Fatalf("NumericFacts did not preserve pre-populated IntRanges: got %#v ok=%v", got, ok)
	}
	if got, ok := numeric.ProfiledIntRange(25); !ok || !got.known || got.min != 1 || got.max != 9 {
		t.Fatalf("NumericFacts did not preserve pre-populated ProfiledIntRanges: got %#v ok=%v", got, ok)
	}
	if got, ok := numeric.ProfiledLenRange(26); !ok || !got.known || got.min != 2 || got.max != 8 {
		t.Fatalf("NumericFacts did not preserve pre-populated ProfiledLenRanges: got %#v ok=%v", got, ok)
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

func TestAnalysisResultCallFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	calls := a.CallFacts()

	desc := CallABIDescriptor{NumArgs: 2, NumRets: 1}
	calls.SetCallABIs(map[int]CallABIDescriptor{11: desc})
	if got, ok := calls.CallABI(11); !ok || got.NumArgs != desc.NumArgs || got.NumRets != desc.NumRets {
		t.Fatalf("CallFacts.SetCallABIs did not update domain field: got %#v ok=%v", got, ok)
	}

	calls.SetGuardedConstCallFolds(map[int]GuardedConstCallFoldFact{12: {Result: 99}})
	if got, ok := calls.GuardedConstCallFold(12); !ok || got.Result != 99 {
		t.Fatalf("CallFacts.SetGuardedConstCallFolds did not update domain field: got %#v ok=%v", got, ok)
	}

	calls.SetCallSiteNoResultRuntimeSpecializations(map[int]bool{13: true})
	if !calls.CallSiteNoResultRuntimeSpecialization(13) {
		t.Fatalf("CallFacts.SetCallSiteNoResultRuntimeSpecializations did not update domain field")
	}

	calls.SetCallSiteNoResultRuntimeSpecializationBatches(map[int]CallSiteNoResultRuntimeSpecializationBatchFact{13: {ExitPC: 44}})
	if got, ok := calls.CallSiteNoResultRuntimeSpecializationBatch(13); !ok || got.ExitPC != 44 {
		t.Fatalf("CallFacts.SetCallSiteNoResultRuntimeSpecializationBatches did not update domain field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultLoopSpecializationFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	specializations := a.LoopSpecializationFacts()

	specializations.SetTableArrayUpperBoundSafe(map[int]bool{11: true})
	if !specializations.TableArrayUpperBoundIsSafe(11) {
		t.Fatalf("LoopSpecializationFacts.SetTableArrayUpperBoundSafe did not update domain field")
	}

	specializations.SetTableArrayLowerBoundSafe(map[int]bool{12: true})
	if !specializations.TableArrayLowerBoundIsSafe(12) {
		t.Fatalf("LoopSpecializationFacts.SetTableArrayLowerBoundSafe did not update domain field")
	}

	loopFact := LoopTableArrayFact{HeaderBlockID: 1, PreheaderBlockID: 2, AccessOp: OpTableArrayLoad}
	specializations.SetLoopTableArrayFacts(map[int]LoopTableArrayFact{13: loopFact})
	if got, ok := specializations.LoopTableArrayFact(13); !ok || got.HeaderBlockID != 1 || got.AccessOp != OpTableArrayLoad {
		t.Fatalf("LoopSpecializationFacts.SetLoopTableArrayFacts did not update domain field: got %#v ok=%v", got, ok)
	}

	spec := RecordArrayLoopSpecializationSpec{ShapeID: 99, ScalarCount: 1}
	specializations.SetRecordArrayLoopSpecializations(map[int]RecordArrayLoopSpecializationSpec{14: spec})
	if got, ok := specializations.RecordArrayLoopSpecialization(14); !ok || got.ShapeID != 99 || got.ScalarCount != 1 {
		t.Fatalf("LoopSpecializationFacts.SetRecordArrayLoopSpecializations did not update domain field: got %#v ok=%v", got, ok)
	}

	dataFact := TableArrayDataPtrFact{TableID: 5}
	specializations.SetTableArrayDataPtrs(map[int]TableArrayDataPtrFact{15: dataFact})
	if got, ok := specializations.TableArrayDataPtr(15); !ok || got.TableID != 5 {
		t.Fatalf("LoopSpecializationFacts.SetTableArrayDataPtrs did not update domain field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultLoopSpecializationFactsPreservesPrePopulatedDomain(t *testing.T) {
	loopFact := LoopTableArrayFact{HeaderBlockID: 3, PreheaderBlockID: 4, AccessOp: OpTableArrayStore}
	spec := RecordArrayLoopSpecializationSpec{ShapeID: 77}
	a := &AnalysisResult{
		LoopSpecialization: newLoopSpecializationFactsForTest(loopSpecializationFactsSeed{
			TableArrayUpperBoundSafe:       map[int]bool{21: true},
			TableArrayLowerBoundSafe:       map[int]bool{22: true},
			LoopTableArrayFacts:            map[int]LoopTableArrayFact{23: loopFact},
			RecordArrayLoopSpecializations: map[int]RecordArrayLoopSpecializationSpec{24: spec},
		}),
	}

	specializations := a.LoopSpecializationFacts()
	if !specializations.TableArrayUpperBoundIsSafe(21) {
		t.Fatalf("LoopSpecializationFacts did not preserve pre-populated TableArrayUpperBoundSafe")
	}
	if !specializations.TableArrayLowerBoundIsSafe(22) {
		t.Fatalf("LoopSpecializationFacts did not preserve pre-populated TableArrayLowerBoundSafe")
	}
	if got, ok := specializations.LoopTableArrayFact(23); !ok || got.HeaderBlockID != 3 || got.AccessOp != OpTableArrayStore {
		t.Fatalf("LoopSpecializationFacts did not preserve pre-populated LoopTableArrayFacts: got %#v ok=%v", got, ok)
	}
	if got, ok := specializations.RecordArrayLoopSpecialization(24); !ok || got.ShapeID != 77 {
		t.Fatalf("LoopSpecializationFacts did not preserve pre-populated RecordArrayLoopSpecializations: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultCallFactsPreservesPrePopulatedDomain(t *testing.T) {
	desc := CallABIDescriptor{NumArgs: 3, NumRets: 1}
	a := &AnalysisResult{
		Call: newCallFactsForTest(map[int]CallABIDescriptor{21: desc}),
	}

	calls := a.CallFacts()
	if got, ok := calls.CallABI(21); !ok || got.NumArgs != desc.NumArgs {
		t.Fatalf("CallFacts did not preserve pre-populated CallABIs: got %#v ok=%v", got, ok)
	}
}

func TestCallFactsReadHelpersAreNilSafe(t *testing.T) {
	var calls *CallFacts
	if calls.CallABICount() != 0 {
		t.Fatalf("nil CallFacts reported CallABI facts")
	}
	if calls.GuardedConstCallFoldCount() != 0 {
		t.Fatalf("nil CallFacts reported runtime specialization const call facts")
	}
	if calls.CallSiteNoResultRuntimeSpecializationBatchMap() != nil {
		t.Fatalf("nil CallFacts returned call-site no-result batches")
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
	calls.ForEachGuardedConstCallFold(func(int, GuardedConstCallFoldFact) bool {
		visitedProtocolFold = true
		return true
	})
	if visitedProtocolFold {
		t.Fatalf("nil CallFacts visited runtime specialization const call facts")
	}
}

func TestAnalysisResultCallFactsPreservesDomainMapsWhenCompatibilityFieldsNil(t *testing.T) {
	a := &AnalysisResult{Call: NewCallFacts()}
	a.CallFacts().SetCallABIs(map[int]CallABIDescriptor{41: {NumArgs: 5}})
	a.CallFacts().SetGuardedConstCallFolds(map[int]GuardedConstCallFoldFact{42: {Result: 13}})
	a.CallFacts().SetCallSiteNoResultRuntimeSpecializations(map[int]bool{43: true})
	a.CallFacts().SetCallSiteNoResultRuntimeSpecializationBatches(map[int]CallSiteNoResultRuntimeSpecializationBatchFact{44: {ExitPC: 66}})

	calls := a.CallFacts()
	if got, ok := calls.CallABI(41); !ok || got.NumArgs != 5 {
		t.Fatalf("CallFacts lost domain CallABIs with nil compatibility field: got %#v ok=%v", got, ok)
	}
	if got, ok := calls.GuardedConstCallFold(42); !ok || got.Result != 13 {
		t.Fatalf("CallFacts lost domain GuardedConstCallFolds with nil compatibility field: got %#v ok=%v", got, ok)
	}
	if !calls.CallSiteNoResultRuntimeSpecialization(43) {
		t.Fatalf("CallFacts lost domain CallSiteNoResultRuntimeSpecializations with nil compatibility field")
	}
	if got, ok := calls.CallSiteNoResultRuntimeSpecializationBatch(44); !ok || got.ExitPC != 66 {
		t.Fatalf("CallFacts lost domain CallSiteNoResultRuntimeSpecializationBatches with nil compatibility field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultSpeculationFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	spec := a.SpeculationFacts()
	callee := &vm.FuncProto{Name: "callee"}

	spec.SetSpecDependencyProtos(map[*vm.FuncProto]bool{callee: true})
	if !spec.SpecDependencyProto(callee) {
		t.Fatalf("SpeculationFacts.SetSpecDependencyProtos did not update domain field")
	}

	spec.SetSuppressedSpecGuardPCs(map[int]bool{31: true})
	if !spec.SuppressedSpecGuardPC(31) {
		t.Fatalf("SpeculationFacts.SetSuppressedSpecGuardPCs did not update domain field")
	}

	spec.SetSuppressedSpecGuardKinds(map[int]map[string]bool{
		32: {"GuardType": true},
	})
	if !spec.SpecGuardKindSuppressed(32, "GuardType") {
		t.Fatalf("SpeculationFacts.SetSuppressedSpecGuardKinds did not update domain field")
	}
}

func TestAnalysisResultSpeculationFactsPreservesPrePopulatedDomain(t *testing.T) {
	callee := &vm.FuncProto{Name: "callee"}
	a := &AnalysisResult{
		Speculation: newSpeculationFactsForTest(speculationFactsSeed{
			SpecDependencyProtos:     map[*vm.FuncProto]bool{callee: true},
			SuppressedSpecGuardPCs:   map[int]bool{41: true},
			SuppressedSpecGuardKinds: map[int]map[string]bool{42: {"GuardCalleeProto": true}},
		}),
	}

	spec := a.SpeculationFacts()
	if !spec.SpecDependencyProto(callee) {
		t.Fatalf("SpeculationFacts did not adopt compatibility SpecDependencyProtos")
	}
	if !spec.SuppressedSpecGuardPC(41) {
		t.Fatalf("SpeculationFacts did not adopt compatibility SuppressedSpecGuardPCs")
	}
	if !spec.SpecGuardKindSuppressed(42, "GuardCalleeProto") {
		t.Fatalf("SpeculationFacts did not adopt compatibility SuppressedSpecGuardKinds")
	}
}

func TestAnalysisResultSpeculationFactsPreservesSuppressedKindsNilSentinel(t *testing.T) {
	a := NewAnalysisResult()
	spec := a.SpeculationFacts()

	if spec.SuppressedSpecGuardKindsMap() != nil {
		t.Fatalf("NewAnalysisResult initialized SuppressedSpecGuardKinds nil sentinel")
	}

	spec.SetSuppressedSpecGuardKinds(map[int]map[string]bool{})
	if specGuardKindSuppressed(&Function{Analysis: a}, 51, "GuardType") {
		t.Fatalf("empty non-nil SuppressedSpecGuardKinds should not fall back to SuppressedSpecGuardPCs")
	}

	spec.SetSuppressedSpecGuardPCs(map[int]bool{51: true})
	spec.SetSuppressedSpecGuardKinds(nil)
	if !specGuardKindSuppressed(&Function{Analysis: a}, 51, "GuardType") {
		t.Fatalf("nil SuppressedSpecGuardKinds sentinel should restore PC fallback")
	}
}

func TestAnalysisResultTableShapeFactsMutatorsUpdateDomain(t *testing.T) {
	a := NewAnalysisResult()
	shapes := a.TableShapeFacts()

	cases := []FieldPolyShapeCase{{ShapeID: 11, FieldIdx: 2}}
	shapes.SetFieldPolyShapeFacts(map[int][]FieldPolyShapeCase{101: cases})
	if got, ok := shapes.FieldPolyShapeCases(101); !ok || len(got) != 1 || got[0].ShapeID != 11 {
		t.Fatalf("TableShapeFacts.SetFieldPolyShapeFacts did not update domain field: got %#v ok=%v", got, ok)
	}

	shapes.SetFieldPolyShapeReceivers(map[int]bool{102: true})
	if !shapes.FieldPolyShapeReceiver(102) {
		t.Fatalf("TableShapeFacts.SetFieldPolyShapeReceivers did not update domain field")
	}

	shapes.SetFieldPolyShapeCatalog(map[uint32]FixedShapeTableFact{12: {ShapeID: 12}})
	if got, ok := shapes.FieldPolyShapeCatalogFact(12); !ok || got.ShapeID != 12 {
		t.Fatalf("TableShapeFacts.SetFieldPolyShapeCatalog did not update domain field: got %#v ok=%v", got, ok)
	}

	fusions := []FieldCallPolyLenFusion{{LenValueID: 103, ShapeID: 13, Len: 4}}
	shapes.SetFieldCallPolyLenFusions(map[int][]FieldCallPolyLenFusion{104: fusions})
	if got, ok := shapes.FieldCallPolyLenFusionCases(104); !ok || len(got) != 1 || got[0].LenValueID != 103 {
		t.Fatalf("TableShapeFacts.SetFieldCallPolyLenFusions did not update domain field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultTableShapeFactsRecordHelpersUpdateDomain(t *testing.T) {
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
	if got, ok := shapes.FieldPolyShapeCases(501); !ok || len(got) != 1 || got[0].ShapeID != 51 {
		t.Fatalf("RecordFieldPolyShapeCases did not update domain field: got %#v", got)
	}
	if got, ok := shapes.FieldPolyShapeCatalogFact(51); !ok || got.ShapeID != 51 || len(got.FieldNames) != 1 || got.FieldNames[0] != "step" {
		t.Fatalf("RecordFieldPolyShapeCases did not record catalog clone: got %#v ok=%v", got, ok)
	}
	cases[0].ReceiverFact.FieldNames[0] = "mutated"
	if got, _ := shapes.FieldPolyShapeCatalogFact(51); got.FieldNames[0] != "step" {
		t.Fatalf("catalog fact should be cloned, got %#v", got.FieldNames)
	}

	shapes.RecordFieldPolyShapeReceiver(502)
	if !shapes.FieldPolyShapeReceiver(502) {
		t.Fatalf("RecordFieldPolyShapeReceiver did not update domain field")
	}

	shapes.RecordFieldCallPolyLenFusions(503, []FieldCallPolyLenFusion{{LenValueID: 504, ShapeID: 51, Len: 7}})
	if got, ok := shapes.FieldCallPolyLenFusionCases(503); !ok || len(got) != 1 || got[0].Len != 7 {
		t.Fatalf("RecordFieldCallPolyLenFusions did not update domain field: got %#v", got)
	}

	shapes.DeleteFieldPolyShapeCases(501)
	if _, ok := shapes.FieldPolyShapeCases(501); ok {
		t.Fatalf("DeleteFieldPolyShapeCases did not update domain field")
	}
}

func TestAnalysisResultTableShapeFactsPreservesPrePopulatedDomain(t *testing.T) {
	a := &AnalysisResult{
		TableShape: newTableShapeFactsForTest(tableShapeFactsSeed{
			FieldPolyShapeFacts:     map[int][]FieldPolyShapeCase{201: {{ShapeID: 21, FieldIdx: 1}}},
			FieldPolyShapeReceivers: map[int]bool{202: true},
			FieldPolyShapeCatalog:   map[uint32]FixedShapeTableFact{22: {ShapeID: 22}},
			FieldCallPolyLenFusions: map[int][]FieldCallPolyLenFusion{203: {{LenValueID: 204, ShapeID: 23}}},
		}),
	}

	shapes := a.TableShapeFacts()
	if got, ok := shapes.FieldPolyShapeCases(201); !ok || len(got) != 1 || got[0].ShapeID != 21 {
		t.Fatalf("TableShapeFacts did not preserve pre-populated FieldPolyShapeFacts: got %#v ok=%v", got, ok)
	}
	if !shapes.FieldPolyShapeReceiver(202) {
		t.Fatalf("TableShapeFacts did not preserve pre-populated FieldPolyShapeReceivers")
	}
	if got, ok := shapes.FieldPolyShapeCatalogFact(22); !ok || got.ShapeID != 22 {
		t.Fatalf("TableShapeFacts did not preserve pre-populated FieldPolyShapeCatalog: got %#v ok=%v", got, ok)
	}
	if got, ok := shapes.FieldCallPolyLenFusionCases(203); !ok || len(got) != 1 || got[0].LenValueID != 204 {
		t.Fatalf("TableShapeFacts did not preserve pre-populated FieldCallPolyLenFusions: got %#v ok=%v", got, ok)
	}
}

func TestTableShapeFactsReadHelpers(t *testing.T) {
	a := &AnalysisResult{
		TableShape: newTableShapeFactsForTest(tableShapeFactsSeed{
			FieldPolyShapeFacts: map[int][]FieldPolyShapeCase{
				301: {
					{ShapeID: 31, FieldIdx: 1},
					{ShapeID: 32, FieldIdx: 2},
				},
			},
			FieldPolyShapeCatalog: map[uint32]FixedShapeTableFact{
				33: {ShapeID: 33},
			},
		}),
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

func TestAnalysisResultTableShapeFactsPreservesEmptyMapSentinelBehavior(t *testing.T) {
	a := &AnalysisResult{
		TableShape: newTableShapeFactsForTest(tableShapeFactsSeed{
			FieldPolyShapeFacts: map[int][]FieldPolyShapeCase{401: {{ShapeID: 41}}},
		}),
	}
	shapes := a.TableShapeFacts()

	shapes.SetFieldPolyShapeFacts(map[int][]FieldPolyShapeCase{})
	if _, ok := shapes.FieldPolyShapeCases(401); ok {
		t.Fatalf("empty non-nil FieldPolyShapeFacts should not fall back to previous entries")
	}
	if got := shapes.FieldPolyShapeFactCount(); got != 0 {
		t.Fatalf("empty non-nil FieldPolyShapeFacts should be retained: count=%d", got)
	}

	shapes.SetFieldCallPolyLenFusions(map[int][]FieldCallPolyLenFusion{})
	if got, ok := shapes.FieldCallPolyLenFusionCases(0); ok || got != nil {
		t.Fatalf("empty non-nil FieldCallPolyLenFusions should be retained: got %#v ok=%v", got, ok)
	}
}

func TestSpecGuardKindSuppressedNilKindsFallsBackToPCs(t *testing.T) {
	fn := &Function{Analysis: &AnalysisResult{
		Speculation: newSpeculationFactsForTest(speculationFactsSeed{
			SuppressedSpecGuardPCs: map[int]bool{42: true},
		}),
	}}

	if !specGuardKindSuppressed(fn, 42, "GuardCalleeProto") {
		t.Fatalf("nil SuppressedSpecGuardKinds should fall back to SuppressedSpecGuardPCs")
	}

	fn.Analysis.SpeculationFacts().SetSuppressedSpecGuardKinds(map[int]map[string]bool{})
	if specGuardKindSuppressed(fn, 42, "GuardCalleeProto") {
		t.Fatalf("empty non-nil SuppressedSpecGuardKinds should not fall back to SuppressedSpecGuardPCs")
	}
}

func assertAnalysisResultMapSentinels(t *testing.T, a *AnalysisResult, constructor string) {
	t.Helper()

	// AnalysisResult is now a pure domain-struct container; its analysis maps
	// live on the domain structs. Walk each non-nil domain pointer and check the
	// map sentinels there.
	v := reflect.ValueOf(a).Elem()
	for i := 0; i < v.NumField(); i++ {
		domain := v.Field(i)
		if domain.Kind() != reflect.Pointer || domain.IsNil() {
			continue
		}
		assertDomainStructMapSentinels(t, domain.Elem(), constructor)
	}
}

func assertDomainStructMapSentinels(t *testing.T, v reflect.Value, constructor string) {
	t.Helper()
	if v.Kind() != reflect.Struct {
		return
	}
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fieldValue := v.Field(i)
		if fieldValue.Kind() != reflect.Map {
			continue
		}

		name := typ.Field(i).Name
		if name == "globals" || name == "suppressedSpecGuardKinds" {
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
