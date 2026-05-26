package methodjit

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestReadContract_ProductionPipeline drives the real Tier 2 pipeline over the
// benchmark corpus and observes, per optimizer module run, which AnalysisResult
// fact domains the module accessed through the domain accessors. It compares the
// accessed set against the domains covered by the module's declared facts
// (Requires ∪ Provides ∪ Updates) and reports undeclared reads.
//
// This is REPORT mode, mirroring the write-contract test but deliberately NOT
// enforcing: we log the landscape of undeclared reads so we can decide where to
// add Requires before turning on enforcement. It never t.Errors.
//
// Note: a module that declares nothing has an empty allowed set, so every domain
// it reads is undeclared — those are the interesting findings. Reads performed
// inside a pass's internally-built sub-Function (e.g. inlining) are not observed.
func TestReadContract_ProductionPipeline(t *testing.T) {
	roots := []string{
		"../../benchmarks/suite",
		"../../benchmarks/extended",
		"../../benchmarks/variants",
	}
	var files []string
	for _, root := range roots {
		matches, _ := filepath.Glob(filepath.Join(root, "*.gs"))
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

	report := CheckPipelineReadContract(allRuns)
	t.Logf("read-contract scan: %d sources, %d protos compiled through pipeline, %d module runs observed (%d with an observed access set)",
		compiled, pipelines, len(allRuns), report.ObservedRuns)
	t.Logf("module runs that accessed an UNDECLARED domain: %d", report.ModuleRunsWithUndeclaredReads)

	if len(report.Findings) == 0 {
		t.Logf("no undeclared read-domain accesses across observed module runs")
		return
	}

	t.Logf("undeclared read domains by module (%d distinct module/domain pairs):\n%s",
		len(report.Findings), FormatReadContractFindings(report))

	// REPORT MODE: do not fail. Surface the landscape only.
}
