package vm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func TestCallSiteRuntimeSpecializationDiagnosticsRejectBenchmarkMetadataWithoutShape(t *testing.T) {
	proto, vm := compileSpectralKernelTestProgram(t, `
func fannkuch(n) { return n }
func sieve(n) { return n }
func product(left, right, size) { return left }
func advance(dt) { return dt }
`)
	defer vm.Close()
	if len(proto.Protos) != 4 {
		t.Fatalf("nested protos = %d, want 4", len(proto.Protos))
	}

	sources := []string{
		"benchmarks/suite/fannkuch.gs",
		"benchmarks/suite/sieve.gs",
		"benchmarks/suite/matmul.gs",
		"benchmarks/suite/nbody.gs",
	}
	for i, child := range proto.Protos {
		child.Source = sources[i]
		if infos := RecognizedCallSiteRuntimeSpecializations(child); len(infos) != 0 {
			t.Fatalf("metadata-only proto %q/%q recognized as %+v", child.Name, child.Source, infos)
		}
		for _, diag := range DiagnoseCallSiteRuntimeSpecializationProto(child) {
			if diag.Recognized {
				t.Fatalf("metadata-only proto %q/%q recognized by diagnostic %+v", child.Name, child.Source, diag)
			}
			if diag.Reason != runtimeSpecializationReasonShapeMismatch {
				t.Fatalf("metadata-only diagnostic reason = %q, want %q", diag.Reason, runtimeSpecializationReasonShapeMismatch)
			}
		}
	}
}

func TestPermutationFlipChecksumRecognizesCurrentBenchmarkShape(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "suite", "fannkuch.gs"))
	if err != nil {
		t.Fatal(err)
	}
	proto, vm := compileSpectralKernelTestProgram(t, string(src))
	defer vm.Close()
	child := findTestProtoByName(proto, "fannkuch")
	if child == nil {
		t.Fatal("missing fannkuch proto")
	}
	if !isPermutationFlipChecksumKernelProto(child) {
		t.Fatalf("permutation flip checksum recognizer rejected current structural shape: code=%d const=%d maxstack=%d", len(child.Code), len(child.Constants), child.MaxStack)
	}
	if !cachedRuntimeSpecializationRecognized(child, runtimeSpecializationPermutationFlipChecksum) {
		t.Fatal("permutation flip checksum rejected by runtime specialization cache")
	}
	if child.PermutationFlipChecksumKernel == nil || child.PermutationFlipChecksumKernel.spec == nil {
		t.Fatal("permutation flip checksum proto-local spec was not generated")
	}
}

func TestRuntimeSpecializationTieringPolicyCatalogCoversRuntimeSources(t *testing.T) {
	for _, tc := range []struct {
		name  string
		infos []RuntimeSpecializationInfo
	}{
		{name: "nested_int_recurrence", infos: CallSiteRuntimeSpecializationCatalog()},
		{name: "generic_record_array_loop", infos: DriverLoopRuntimeSpecializationCatalog()},
		{name: "record_pairwise_numeric_loop", infos: DriverLoopRuntimeSpecializationCatalog()},
	} {
		info := findRuntimeSpecializationInfo(t, tc.infos, tc.name)
		if !info.HasCapability(RuntimeSpecializationCapabilityStructuralTiering) {
			t.Fatalf("%s missing structural tiering capability: %+v", tc.name, info)
		}
		if !info.AllowsStructuralTiering(&FuncProto{}) {
			t.Fatalf("%s structural tiering policy rejected default proto: %+v", tc.name, info)
		}
	}

	info := findRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "record_pairwise_numeric")
	if !info.HasCapability(RuntimeSpecializationCapabilityStructuralTiering) {
		t.Fatalf("record_pairwise_numeric missing structural tiering capability: %+v", info)
	}
	if info.AllowsStructuralTiering(&FuncProto{}) {
		t.Fatal("record_pairwise_numeric structural tiering should require a float constant")
	}
	if !info.AllowsStructuralTiering(&FuncProto{Constants: []runtime.Value{runtime.FloatValue(1)}}) {
		t.Fatal("record_pairwise_numeric structural tiering should allow float-specialized protos")
	}
}

func requireRuntimeSpecializationInfo(t *testing.T, infos []RuntimeSpecializationInfo, name string) {
	t.Helper()
	if !hasRuntimeSpecializationInfo(infos, name) {
		t.Fatalf("kernel %q not found in %+v", name, infos)
	}
}

func rejectRuntimeSpecializationInfo(t *testing.T, infos []RuntimeSpecializationInfo, name string) {
	t.Helper()
	if hasRuntimeSpecializationInfo(infos, name) {
		t.Fatalf("kernel %q unexpectedly found in %+v", name, infos)
	}
}

func hasRuntimeSpecializationInfo(infos []RuntimeSpecializationInfo, name string) bool {
	for _, info := range infos {
		if info.Name == name {
			return true
		}
	}
	return false
}

func findRuntimeSpecializationInfo(t *testing.T, infos []RuntimeSpecializationInfo, name string) RuntimeSpecializationInfo {
	t.Helper()
	for _, info := range infos {
		if info.Name == name {
			return info
		}
	}
	t.Fatalf("kernel %q not found in %+v", name, infos)
	return RuntimeSpecializationInfo{}
}

func requireRuntimeSpecializationDiagnostic(t *testing.T, diagnostics []RuntimeSpecializationDiagnostic, name string) RuntimeSpecializationDiagnostic {
	t.Helper()
	for _, diag := range diagnostics {
		if diag.Specialization.Name == name {
			return diag
		}
	}
	t.Fatalf("diagnostic for %q not found in %+v", name, diagnostics)
	return RuntimeSpecializationDiagnostic{}
}
