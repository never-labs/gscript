package vm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCachedWholeCallKernelRecognizedUsesHotCache(t *testing.T) {
	proto := &FuncProto{
		WholeCallKernel: &wholeCallKernelProtoCache{
			fingerprint: wholeCallKernelFingerprint{codeLen: 123},
			recognized:  uint64(1) << uint(wholeCallKernelRecordWalkFold),
		},
	}
	if !cachedWholeCallKernelRecognized(proto, wholeCallKernelRecordWalkFold) {
		t.Fatal("cached hot dispatch guard recomputed structure instead of using cached bits")
	}
	if cachedWholeCallKernelRecognized(proto, wholeCallKernelIntGridAggregate) {
		t.Fatal("cached hot dispatch guard reported an uncached kernel bit")
	}
}

func TestWholeCallKernelDiagnosticsRejectBenchmarkMetadataWithoutShape(t *testing.T) {
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
		if infos := RecognizedWholeCallKernels(child); len(infos) != 0 {
			t.Fatalf("metadata-only proto %q/%q recognized as %+v", child.Name, child.Source, infos)
		}
		for _, diag := range DiagnoseWholeCallKernelProto(child) {
			if diag.Recognized {
				t.Fatalf("metadata-only proto %q/%q recognized by diagnostic %+v", child.Name, child.Source, diag)
			}
			if diag.Reason != kernelReasonShapeMismatch {
				t.Fatalf("metadata-only diagnostic reason = %q, want %q", diag.Reason, kernelReasonShapeMismatch)
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
		t.Fatalf("permutation flip checksum recognizer rejected current benchmark shape: code=%d const=%d maxstack=%d", len(child.Code), len(child.Constants), child.MaxStack)
	}
	if !mayHaveWholeCallValueKernelCandidate(child, 1, false) {
		t.Fatal("permutation flip checksum rejected by value-kernel candidate gate")
	}
	if !cachedWholeCallKernelRecognized(child, wholeCallKernelPermutationFlipChecksum) {
		t.Fatal("permutation flip checksum rejected by cached kernel bits")
	}
}

func requireKernelInfo(t *testing.T, infos []KernelInfo, name string) {
	t.Helper()
	if !hasKernelInfo(infos, name) {
		t.Fatalf("kernel %q not found in %+v", name, infos)
	}
}

func rejectKernelInfo(t *testing.T, infos []KernelInfo, name string) {
	t.Helper()
	if hasKernelInfo(infos, name) {
		t.Fatalf("kernel %q unexpectedly found in %+v", name, infos)
	}
}

func hasKernelInfo(infos []KernelInfo, name string) bool {
	for _, info := range infos {
		if info.Name == name {
			return true
		}
	}
	return false
}

func requireKernelDiagnostic(t *testing.T, diagnostics []KernelDiagnostic, name string) KernelDiagnostic {
	t.Helper()
	for _, diag := range diagnostics {
		if diag.Kernel.Name == name {
			return diag
		}
	}
	t.Fatalf("diagnostic for %q not found in %+v", name, diagnostics)
	return KernelDiagnostic{}
}
