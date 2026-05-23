package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func spectralSource(t *testing.T, file string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "suite", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return string(src)
}

func TestSpectralRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, spectralSource(t, "spectral_norm.gs"))
	multiplyAv := findTestProtoByName(top, "multiplyAv")
	multiplyAtv := findTestProtoByName(top, "multiplyAtv")
	multiplyAtAv := findTestProtoByName(top, "multiplyAtAv")
	if multiplyAv == nil || multiplyAtv == nil || multiplyAtAv == nil {
		t.Fatal("missing spectral protos")
	}

	for _, tc := range []struct {
		name  string
		proto *FuncProto
		id    int
		kind  spectralRuntimeSpecializationKind
	}{
		{name: "coefficient_matrix_vector", proto: multiplyAv, id: wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixVector, kind: spectralRuntimeSpecializationAv},
		{name: "coefficient_matrix_transpose_vector", proto: multiplyAtv, id: wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixTransposeVector, kind: spectralRuntimeSpecializationAtv},
		{name: "coefficient_matrix_ata_vector", proto: multiplyAtAv, id: wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixAtAVector, kind: spectralRuntimeSpecializationAtAv},
	} {
		requireRuntimeSpecializationInfo(t, WholeCallRuntimeSpecializationCatalog(), tc.name)
		requireRuntimeSpecializationInfo(t, RecognizedWholeCallRuntimeSpecializations(tc.proto), tc.name)
		if !cachedWholeCallNoResultRuntimeSpecializationRecognized(tc.proto, tc.id) {
			t.Fatalf("%s rejected by no-result runtime specialization cache", tc.name)
		}
		if tc.proto.SpectralRuntimeSpecialization == nil || tc.proto.SpectralRuntimeSpecialization.spec == nil {
			t.Fatalf("%s proto-local spec was not generated", tc.name)
		}
		if got := tc.proto.SpectralRuntimeSpecialization.spec.kind; got != tc.kind {
			t.Fatalf("%s kind = %d, want %d", tc.name, got, tc.kind)
		}
		diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseWholeCallRuntimeSpecializationProto(tc.proto), tc.name)
		if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
			t.Fatalf("%s diagnostic = %+v, want recognized %q", tc.name, diag, runtimeSpecializationReasonRecognized)
		}
	}
}

func TestDenseSpectralRuntimeSpecializationDiagnostics(t *testing.T) {
	top := compileProto(t, spectralSource(t, "spectral_norm_dense.gs"))
	multiplyAtAv := findTestProtoByName(top, "multiplyAtAv")
	if multiplyAtAv == nil {
		t.Fatal("missing dense multiplyAtAv proto")
	}

	requireRuntimeSpecializationInfo(t, WholeCallRuntimeSpecializationCatalog(), "dense_coefficient_matrix_ata_vector")
	requireRuntimeSpecializationInfo(t, RecognizedWholeCallRuntimeSpecializations(multiplyAtAv), "dense_coefficient_matrix_ata_vector")
	if !cachedWholeCallNoResultRuntimeSpecializationRecognized(multiplyAtAv, wholeCallNoResultRuntimeSpecializationSpectralDenseCoefficientMatrixAtAVector) {
		t.Fatal("dense_coefficient_matrix_ata_vector rejected by no-result runtime specialization cache")
	}
	if multiplyAtAv.SpectralRuntimeSpecialization == nil || multiplyAtAv.SpectralRuntimeSpecialization.spec == nil {
		t.Fatal("dense spectral proto-local spec was not generated")
	}
	if got := multiplyAtAv.SpectralRuntimeSpecialization.spec.kind; got != spectralRuntimeSpecializationDenseAtAv {
		t.Fatalf("dense spectral kind = %d, want %d", got, spectralRuntimeSpecializationDenseAtAv)
	}
}

func TestSpectralRuntimeSpecializationRecordsHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	src := spectralSource(t, "spectral_norm.gs")
	src = strings.Replace(src, "N := 2000", "N := 32", 1)
	globals := compileAndRun(t, src)
	if v := globals["result"]; !v.IsNumber() {
		t.Fatalf("result = %s (%v), want number", v.TypeName(), v)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteWholeCallNoResult, "coefficient_matrix_ata_vector"); got == 0 {
		t.Fatal("coefficient_matrix_ata_vector structural hit count = 0, want at least 1")
	}
}

func TestDenseSpectralRuntimeSpecializationRecordsHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	src := spectralSource(t, "spectral_norm_dense.gs")
	src = strings.Replace(src, "N := 1500", "N := 32", 1)
	src = strings.Replace(src, "WARM_N := 64", "WARM_N := 16", 1)
	globals := compileAndRun(t, src)
	if v := globals["result"]; !v.IsNumber() {
		t.Fatalf("result = %s (%v), want number", v.TypeName(), v)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteWholeCallNoResult, "dense_coefficient_matrix_ata_vector"); got == 0 {
		t.Fatal("dense_coefficient_matrix_ata_vector structural hit count = 0, want at least 1")
	}
}
