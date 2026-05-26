package methodjit

import (
	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// Test-only builders that construct domain fact structs from pre-populated maps.
// The domain map fields are unexported to force accessor-only access in
// production; these helpers let tests seed a domain in one expression while
// still going through the package's setters.

type numericFactsSeed struct {
	Int48Safe            map[int]bool
	IntModNonZeroDivisor map[int]bool
	IntModNoSignAdjust   map[int]bool
	IntRanges            map[int]intRange
	ProfiledIntRanges    map[int]intRange
	ProfiledLenRanges    map[int]intRange
	IntNonNegative       map[int]bool
}

func newNumericFactsForTest(seed numericFactsSeed) *NumericFacts {
	n := &NumericFacts{}
	if seed.Int48Safe != nil {
		n.SetInt48Safe(seed.Int48Safe)
	}
	if seed.IntModNonZeroDivisor != nil {
		n.SetIntModNonZeroDivisor(seed.IntModNonZeroDivisor)
	}
	if seed.IntModNoSignAdjust != nil {
		n.SetIntModNoSignAdjust(seed.IntModNoSignAdjust)
	}
	if seed.IntRanges != nil {
		n.SetIntRanges(seed.IntRanges)
	}
	if seed.ProfiledIntRanges != nil {
		n.SetProfiledIntRanges(seed.ProfiledIntRanges)
	}
	if seed.ProfiledLenRanges != nil {
		n.SetProfiledLenRanges(seed.ProfiledLenRanges)
	}
	if seed.IntNonNegative != nil {
		n.SetIntNonNegative(seed.IntNonNegative)
	}
	return n
}

func newCallFactsForTest(callABIs map[int]CallABIDescriptor) *CallFacts {
	c := &CallFacts{}
	if callABIs != nil {
		c.SetCallABIs(callABIs)
	}
	return c
}

type speculationFactsSeed struct {
	SpecDependencyProtos     map[*vm.FuncProto]bool
	SuppressedSpecGuardPCs   map[int]bool
	SuppressedSpecGuardKinds map[int]map[string]bool
}

func newSpeculationFactsForTest(seed speculationFactsSeed) *SpeculationFacts {
	s := &SpeculationFacts{}
	if seed.SpecDependencyProtos != nil {
		s.SetSpecDependencyProtos(seed.SpecDependencyProtos)
	}
	if seed.SuppressedSpecGuardPCs != nil {
		s.SetSuppressedSpecGuardPCs(seed.SuppressedSpecGuardPCs)
	}
	if seed.SuppressedSpecGuardKinds != nil {
		s.SetSuppressedSpecGuardKinds(seed.SuppressedSpecGuardKinds)
	}
	return s
}

type tableShapeFactsSeed struct {
	FieldPolyShapeFacts     map[int][]FieldPolyShapeCase
	FieldPolyShapeReceivers map[int]bool
	FieldPolyShapeCatalog   map[uint32]FixedShapeTableFact
	FieldCallPolyLenFusions map[int][]FieldCallPolyLenFusion
	FixedShapeTables        map[int]FixedShapeTableFact
	FixedShapeArgFacts      map[int]FixedShapeTableFact
	FixedTableConstructors  map[int]FixedTableConstructorFact
	FixedShapeEntryGuards   map[int]FixedShapeTableFact
}

func newTableShapeFactsForTest(seed tableShapeFactsSeed) *TableShapeFacts {
	t := &TableShapeFacts{}
	if seed.FieldPolyShapeFacts != nil {
		t.SetFieldPolyShapeFacts(seed.FieldPolyShapeFacts)
	}
	if seed.FieldPolyShapeReceivers != nil {
		t.SetFieldPolyShapeReceivers(seed.FieldPolyShapeReceivers)
	}
	if seed.FieldPolyShapeCatalog != nil {
		t.SetFieldPolyShapeCatalog(seed.FieldPolyShapeCatalog)
	}
	if seed.FieldCallPolyLenFusions != nil {
		t.SetFieldCallPolyLenFusions(seed.FieldCallPolyLenFusions)
	}
	if seed.FixedShapeTables != nil {
		t.SetFixedShapeTables(seed.FixedShapeTables)
	}
	for idx, fact := range seed.FixedShapeArgFacts {
		t.RecordFixedShapeArgFact(idx, fact)
	}
	for idx, fact := range seed.FixedTableConstructors {
		t.RecordFixedTableConstructor(idx, fact)
	}
	for idx, fact := range seed.FixedShapeEntryGuards {
		t.RecordFixedShapeEntryGuard(idx, fact)
	}
	return t
}

type loopSpecializationFactsSeed struct {
	TableArrayUpperBoundSafe       map[int]bool
	TableArrayLowerBoundSafe       map[int]bool
	LoopTableArrayFacts            map[int]LoopTableArrayFact
	RecordArrayLoopSpecializations map[int]RecordArrayLoopSpecializationSpec
	TableArrayDataPtrs             map[int]TableArrayDataPtrFact
}

func newLoopSpecializationFactsForTest(seed loopSpecializationFactsSeed) *LoopSpecializationFacts {
	k := &LoopSpecializationFacts{}
	if seed.TableArrayUpperBoundSafe != nil {
		k.SetTableArrayUpperBoundSafe(seed.TableArrayUpperBoundSafe)
	}
	if seed.TableArrayLowerBoundSafe != nil {
		k.SetTableArrayLowerBoundSafe(seed.TableArrayLowerBoundSafe)
	}
	if seed.LoopTableArrayFacts != nil {
		k.SetLoopTableArrayFacts(seed.LoopTableArrayFacts)
	}
	if seed.RecordArrayLoopSpecializations != nil {
		k.SetRecordArrayLoopSpecializations(seed.RecordArrayLoopSpecializations)
	}
	if seed.TableArrayDataPtrs != nil {
		k.SetTableArrayDataPtrs(seed.TableArrayDataPtrs)
	}
	return k
}

type globalFactsSeed struct {
	Globals                 map[string]*vm.FuncProto
	NumericGlobalValues     map[string]runtime.Value
	GlobalArrayElementFacts map[string]FixedShapeTableFact
}

func newGlobalFactsForTest(seed globalFactsSeed) *GlobalFacts {
	g := &GlobalFacts{}
	if seed.Globals != nil {
		g.SetGlobals(seed.Globals)
	}
	if seed.NumericGlobalValues != nil {
		g.SetNumericGlobalValues(seed.NumericGlobalValues)
	}
	if seed.GlobalArrayElementFacts != nil {
		g.SetGlobalArrayElementFacts(seed.GlobalArrayElementFacts)
	}
	return g
}
