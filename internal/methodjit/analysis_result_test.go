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

func TestAnalysisResultCallFactsBindsCompatibilityFields(t *testing.T) {
	a := NewAnalysisResult()
	calls := a.CallFacts()

	desc := CallABIDescriptor{NumArgs: 2, NumRets: 1}
	calls.SetCallABIs(map[int]CallABIDescriptor{11: desc})
	if got, ok := a.CallABIs[11]; !ok || got.NumArgs != desc.NumArgs || got.NumRets != desc.NumRets {
		t.Fatalf("CallFacts.SetCallABIs did not update compatibility field: got %#v ok=%v", got, ok)
	}

	calls.SetProtocolConstCallFolds(map[int]ProtocolConstCallFoldFact{12: {Result: 99}})
	if got, ok := a.ProtocolConstCallFolds[12]; !ok || got.Result != 99 {
		t.Fatalf("CallFacts.SetProtocolConstCallFolds did not update compatibility field: got %#v ok=%v", got, ok)
	}

	calls.SetWholeCallNoResultKernels(map[int]bool{13: true})
	if !a.WholeCallNoResultKernels[13] {
		t.Fatalf("CallFacts.SetWholeCallNoResultKernels did not update compatibility field")
	}

	calls.SetWholeCallNoResultBatches(map[int]WholeCallNoResultBatchFact{13: {ExitPC: 44}})
	if got, ok := a.WholeCallNoResultBatches[13]; !ok || got.ExitPC != 44 {
		t.Fatalf("CallFacts.SetWholeCallNoResultBatches did not update compatibility field: got %#v ok=%v", got, ok)
	}
}

func TestAnalysisResultCallFactsAdoptsLegacyFields(t *testing.T) {
	desc := CallABIDescriptor{NumArgs: 3, NumRets: 1}
	a := &AnalysisResult{
		CallABIs:                 map[int]CallABIDescriptor{21: desc},
		ProtocolConstCallFolds:   map[int]ProtocolConstCallFoldFact{22: {Result: 7}},
		WholeCallNoResultKernels: map[int]bool{23: true},
		WholeCallNoResultBatches: map[int]WholeCallNoResultBatchFact{23: {ExitPC: 8}},
	}

	calls := a.CallFacts()
	if got, ok := calls.CallABI(21); !ok || got.NumArgs != desc.NumArgs {
		t.Fatalf("CallFacts did not adopt legacy CallABIs: got %#v ok=%v", got, ok)
	}
	if got, ok := calls.ProtocolConstCallFold(22); !ok || got.Result != 7 {
		t.Fatalf("CallFacts did not adopt legacy ProtocolConstCallFolds: got %#v ok=%v", got, ok)
	}
	if !calls.WholeCallNoResultKernel(23) {
		t.Fatalf("CallFacts did not adopt legacy WholeCallNoResultKernels")
	}
	if got, ok := calls.WholeCallNoResultBatch(23); !ok || got.ExitPC != 8 {
		t.Fatalf("CallFacts did not adopt legacy WholeCallNoResultBatches: got %#v ok=%v", got, ok)
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
