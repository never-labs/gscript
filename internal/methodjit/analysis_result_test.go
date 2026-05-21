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
