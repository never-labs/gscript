//go:build darwin && arm64

package methodjit

import (
	"encoding/json"
	"errors"
	"github.com/never-labs/gscript/internal/testutil/vmtest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/vm"
)

func TestWarmDump_ProductionRunArtifacts(t *testing.T) {
	src := `
func sum(n) {
    s := 0
    for i := 1; i <= n; i++ {
        s = s + i
    }
    return s
}

a := sum(10)
b := sum(20)
`
	top := compileProto(t, src)
	globals := vmtest.NewInterpreterGlobals()
	v := vm.New(globals)
	tm := NewTieringManager()
	v.SetMethodJIT(tm)

	outDir := t.TempDir()
	if err := tm.EnableWarmDump(outDir, "sum"); err != nil {
		t.Fatalf("EnableWarmDump: %v", err)
	}
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := tm.WriteWarmDump(top); err != nil {
		t.Fatalf("WriteWarmDump: %v", err)
	}

	sumProto := findProtoByName(top, "sum")
	if sumProto == nil {
		t.Fatal("sum proto not found")
	}
	if sumProto.EnteredTier2 == 0 {
		t.Fatal("sum did not enter Tier 2; test did not exercise warm production dump")
	}

	required := []string{
		"manifest.json",
		"jit-symbols.txt",
		"sum.status.json",
		"sum.feedback.txt",
		"sum.ir.before.txt",
		"sum.ir.after.txt",
		"sum.regalloc.txt",
		"sum.pipeline.txt",
		"sum.contracts.txt",
		"sum.loops.txt",
		"sum.bin",
		"sum.asm.txt",
		"sum.sourcemap.json",
		"sum.pcmap.json",
		"pcmap.json",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("expected warm dump file %s: %v", name, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest warmDumpManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if len(manifest.Protos) != 1 {
		t.Fatalf("manifest protos = %d, want 1", len(manifest.Protos))
	}
	got := manifest.Protos[0]
	if got.Name != "sum" || got.Status != "entered" || !got.Compiled || !got.Entered || got.Failed {
		t.Fatalf("unexpected status: %+v", got)
	}
	if got.Feedback.Slots == 0 || got.Files["feedback"] == "" {
		t.Fatalf("feedback summary missing: %+v", got.Feedback)
	}
	if got.InsnCount == 0 || got.CodeBytes == 0 {
		t.Fatalf("missing code stats: insns=%d bytes=%d", got.InsnCount, got.CodeBytes)
	}
	if len(got.LoopDiagnostics) == 0 || got.Files["loops"] == "" {
		t.Fatalf("loop diagnostics missing: %+v", got.LoopDiagnostics)
	}
	if len(got.PipelineStages) == 0 || got.Files["pipeline"] == "" {
		t.Fatalf("pipeline summary missing: %+v", got.PipelineStages)
	}
	if len(got.ModuleContracts) == 0 || got.Files["module_contracts"] == "" {
		t.Fatalf("module contracts missing: %+v files=%+v", got.ModuleContracts, got.Files)
	}
	pipelineText, err := os.ReadFile(filepath.Join(outDir, got.Files["pipeline"]))
	if err != nil {
		t.Fatalf("read pipeline summary: %v", err)
	}
	if !strings.Contains(string(pipelineText), "RunTier2Pipeline") || !strings.Contains(string(pipelineText), "RegAlloc") {
		t.Fatalf("pipeline summary missing expected stages:\n%s", string(pipelineText))
	}
	contractsText, err := os.ReadFile(filepath.Join(outDir, got.Files["module_contracts"]))
	if err != nil {
		t.Fatalf("read module contracts: %v", err)
	}
	if !strings.Contains(string(contractsText), "requires:") || !strings.Contains(string(contractsText), "provides:") {
		t.Fatalf("module contracts missing expected fields:\n%s", string(contractsText))
	}
	if got.CodeStart == "" || got.CodeEnd == "" || got.Files["sourcemap"] == "" || got.Files["pcmap"] == "" {
		t.Fatalf("PC/source map metadata missing: %+v", got)
	}

	pcMapData, err := os.ReadFile(filepath.Join(outDir, "pcmap.json"))
	if err != nil {
		t.Fatalf("read PC map: %v", err)
	}
	var pcMap warmDumpPCMap
	if err := json.Unmarshal(pcMapData, &pcMap); err != nil {
		t.Fatalf("unmarshal PC map: %v", err)
	}
	if pcMap.Version != 1 || len(pcMap.Functions) != 1 {
		t.Fatalf("unexpected PC map header: %+v", pcMap)
	}
	fn := pcMap.Functions[0]
	if fn.Name != "sum" || fn.CodeBase == "" || fn.CodeEnd == "" || fn.CodeBytes == 0 || len(fn.Ranges) == 0 {
		t.Fatalf("unexpected PC map function: %+v", fn)
	}
	foundIRRange := false
	for _, r := range fn.Ranges {
		if r.PCStart == "" || r.PCEnd == "" {
			t.Fatalf("PC range missing absolute address: %+v", r)
		}
		if r.CodeStart >= 0 && r.CodeEnd > r.CodeStart && r.InstrID > 0 && r.IROp != "" {
			foundIRRange = true
		}
	}
	if !foundIRRange {
		t.Fatalf("PC map has no usable IR ranges: %+v", fn.Ranges)
	}

	symbols, err := os.ReadFile(filepath.Join(outDir, "jit-symbols.txt"))
	if err != nil {
		t.Fatalf("read JIT symbols: %v", err)
	}
	if len(symbols) == 0 ||
		!strings.Contains(string(symbols), "gscript_jit::sum") ||
		!strings.Contains(string(symbols), "proto=sum") ||
		!strings.Contains(string(symbols), "ir=") ||
		!strings.Contains(string(symbols), "bcop=") {
		t.Fatalf("JIT symbols missing expected metadata:\n%s", string(symbols))
	}
}

func TestWarmDump_FailedCompileKeepsPipelineScope(t *testing.T) {
	proto := &vm.FuncProto{Name: "failed", NumParams: 1, MaxStack: 2}
	tm := NewTieringManager()
	outDir := t.TempDir()
	if err := tm.EnableWarmDump(outDir, "failed"); err != nil {
		t.Fatalf("EnableWarmDump: %v", err)
	}
	compileErr := errors.New("synthetic compile failure")
	trace := &Tier2Trace{
		IRBefore: "before",
		PipelineStages: []PipelineStageTiming{
			newNestedPipelineStageTiming("RunTier2Pipeline/numeric/FailingModule", 12, compileErr),
		},
		ModuleRuns: []Tier2ModuleRun{
			{
				Phase:      Tier2PhaseNumeric,
				ModuleName: "FailingModule",
				StageName:  "RunTier2Pipeline/numeric/FailingModule",
				Outcome:    "skip",
				ReasonPass: "FailingPass",
				Reason:     "missing input fact",
				Requires:   analysisFacts(AnalysisFactIntRanges),
				Provides:   analysisFacts(AnalysisFactInt48Safe),
				ChangedDomains: []string{
					"Int48Safe",
				},
				ActualFactDiff: []AnalysisFactDomainDiff{
					{
						Domain:      "Int48Safe",
						BeforeCount: 0,
						AfterCount:  1,
						BeforeHash:  0,
						AfterHash:   0x1234,
					},
				},
			},
		},
	}
	tm.recordWarmDumpCompile(proto, trace, nil, compileErr)
	if err := tm.WriteWarmDump(proto); err != nil {
		t.Fatalf("WriteWarmDump: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest warmDumpManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if len(manifest.Protos) != 1 {
		t.Fatalf("manifest protos = %d, want 1", len(manifest.Protos))
	}
	got := manifest.Protos[0]
	if got.Status != "failed" || !got.Failed || got.Compiled {
		t.Fatalf("unexpected failed status: %+v", got)
	}
	if !strings.Contains(got.FailureReason, compileErr.Error()) {
		t.Fatalf("failure reason = %q, want %q", got.FailureReason, compileErr.Error())
	}
	if len(got.PipelineStages) != 1 || got.PipelineStages[0].Name != "RunTier2Pipeline/numeric/FailingModule" || got.PipelineStages[0].Error != compileErr.Error() {
		t.Fatalf("pipeline stages missing failed module: %+v", got.PipelineStages)
	}
	if len(got.ModuleContracts) != 1 || got.ModuleContracts[0].ModuleName != "FailingModule" ||
		len(got.ModuleContracts[0].Requires) != 1 || got.ModuleContracts[0].Requires[0] != AnalysisFactIntRanges {
		t.Fatalf("module contracts missing failed module: %+v", got.ModuleContracts)
	}
	if len(got.ModuleReasons) != 1 || got.ModuleReasons[0].ModuleName != "FailingModule" ||
		got.ModuleReasons[0].Outcome != "skip" || got.ModuleReasons[0].Reason != "missing input fact" ||
		got.Files["module_reasons"] == "" {
		t.Fatalf("module reasons missing failed module: reasons=%+v files=%+v", got.ModuleReasons, got.Files)
	}
	if len(got.ModuleFactDiffs) != 1 || got.ModuleFactDiffs[0].ModuleName != "FailingModule" ||
		len(got.ModuleFactDiffs[0].ActualFactDiff) != 1 ||
		got.ModuleFactDiffs[0].ActualFactDiff[0].Domain != "Int48Safe" ||
		got.Files["module_fact_diffs"] == "" {
		t.Fatalf("module fact diffs missing failed module: diffs=%+v files=%+v", got.ModuleFactDiffs, got.Files)
	}
	pipelineText, err := os.ReadFile(filepath.Join(outDir, got.Files["pipeline"]))
	if err != nil {
		t.Fatalf("read pipeline summary: %v", err)
	}
	if !strings.Contains(string(pipelineText), "FailingModule") || !strings.Contains(string(pipelineText), "error=") {
		t.Fatalf("pipeline summary missing failed module:\n%s", string(pipelineText))
	}
	reasonsText, err := os.ReadFile(filepath.Join(outDir, got.Files["module_reasons"]))
	if err != nil {
		t.Fatalf("read module reasons: %v", err)
	}
	if !strings.Contains(string(reasonsText), "FailingModule: skip") || !strings.Contains(string(reasonsText), "missing input fact") {
		t.Fatalf("module reasons missing details:\n%s", string(reasonsText))
	}
	factDiffText, err := os.ReadFile(filepath.Join(outDir, got.Files["module_fact_diffs"]))
	if err != nil {
		t.Fatalf("read module fact diffs: %v", err)
	}
	if !strings.Contains(string(factDiffText), "FailingModule: Int48Safe") || !strings.Contains(string(factDiffText), "count 0->1") {
		t.Fatalf("module fact diffs missing details:\n%s", string(factDiffText))
	}
}
