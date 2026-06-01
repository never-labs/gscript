package methodjit

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/never-labs/leia/internal/vm"
)

// TestWriteContract_ProductionPipeline drives the real Tier 2 pipeline over the
// benchmark corpus and checks every optimizer module run against its declared
// write contract (Provides ∪ Updates). This is the ground-truth gate that makes
// the declarative contract enforced rather than documentary.
//
// It ENFORCES by default: any module that mutates a modeled fact it did not
// declare in Provides/Updates fails the test. The current corpus is clean, so
// this locks the guarantee in as a regression guard. Set
// LEIA_ENFORCE_WRITE_CONTRACT=0 to downgrade to report-only while
// intentionally introducing a new fact whose contract is not yet wired.
func TestWriteContract_ProductionPipeline(t *testing.T) {
	enforce := os.Getenv("LEIA_ENFORCE_WRITE_CONTRACT") != "0"

	roots := []string{
		"../../benchmarks/numeric",
		"../../benchmarks/recursion",
		"../../benchmarks/table",
		"../../benchmarks/calls",
		"../../benchmarks/string",
		"../../benchmarks/concurrency",
		"../../benchmarks/data",
		"../../benchmarks/app",
		"../../benchmarks/control",
	}
	var files []string
	for _, root := range roots {
		matches, _ := filepath.Glob(filepath.Join(root, "*.leia"))
		files = append(files, matches...)
	}
	if len(files) == 0 {
		t.Skip("no benchmark sources found")
	}
	sort.Strings(files)

	var allRuns []Tier2ModuleRun
	compiled, pipelines := 0, 0
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Logf("skip %s: %v", file, err)
			continue
		}
		proto := compileTop(t, string(src))
		compiled++
		for _, p := range allProtos(proto) {
			runs := pipelineRunsForProto(p)
			if runs == nil {
				continue
			}
			pipelines++
			allRuns = append(allRuns, runs...)
		}
	}

	agg := CheckPipelineWriteContract(allRuns)
	t.Logf("write-contract scan: %d sources, %d protos compiled through pipeline, %d module runs observed",
		compiled, pipelines, len(allRuns))
	if len(agg.UnmodeledDomains) > 0 {
		t.Logf("unmodeled fact domains (contract-coverage gaps): %v", agg.UnmodeledDomains)
		if enforce {
			t.Errorf("%d unmodeled fact domains; add AnalysisFact coverage or set LEIA_ENFORCE_WRITE_CONTRACT=0 while wiring it", len(agg.UnmodeledDomains))
		}
	}
	if len(agg.Violations) > 0 {
		t.Logf("WRITE-CONTRACT VIOLATIONS (%d):\n  %s", len(agg.Violations),
			FormatWriteContractViolations(agg.Violations))
		if enforce {
			t.Errorf("%d undeclared fact writes; see log above", len(agg.Violations))
		}
	} else {
		t.Logf("no write-contract violations across observed module runs")
	}
}

// allProtos returns the top proto and all nested protos, depth-first.
func allProtos(top *vm.FuncProto) []*vm.FuncProto {
	if top == nil {
		return nil
	}
	out := []*vm.FuncProto{top}
	for _, p := range top.Protos {
		out = append(out, allProtos(p)...)
	}
	return out
}

// pipelineRunsForProto runs the production Tier 2 pipeline on one proto and
// returns the per-module run records, or nil if the proto cannot be compiled
// through the pipeline (not promotable / unsupported shape). Pipeline panics on
// unsupported protos are recovered and treated as "not observed".
func pipelineRunsForProto(p *vm.FuncProto) (runs []Tier2ModuleRun) {
	if p == nil {
		return nil
	}
	defer func() {
		if recover() != nil {
			runs = nil
		}
	}()
	var captured []Tier2ModuleRun
	opts := &Tier2PipelineOpts{ModuleRuns: &captured}
	if _, _, err := RunTier2Pipeline(BuildGraph(p), opts); err != nil {
		return nil
	}
	return captured
}
