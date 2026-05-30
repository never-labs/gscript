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
// (Requires ∪ Provides ∪ Updates ∪ OptionalReads) and reports undeclared reads.
//
// It ENFORCES by default: every optional hint read must be declared on the
// module contract. Set GSCRIPT_ENFORCE_READ_CONTRACT=0 to downgrade to
// report-only while intentionally introducing a new undeclared read.
//
// Note: a module that declares nothing has an empty allowed set, so every domain
// it reads is undeclared. Reads performed inside a pass's internally-built
// sub-Function (e.g. inlining) are not observed.
func TestReadContract_ProductionPipeline(t *testing.T) {
	enforce := os.Getenv("GSCRIPT_ENFORCE_READ_CONTRACT") != "0"
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

	unexpected := report.UnexpectedFindings()
	if len(unexpected) == 0 {
		t.Logf("read contract satisfied")
		return
	}
	lines := make([]string, 0, len(unexpected))
	for _, f := range unexpected {
		lines = append(lines, f.String())
	}
	sort.Strings(lines)
	msg := "read-contract violations (undeclared reads missing from module contract):\n  " +
		joinLines(lines)
	if enforce {
		t.Errorf("%d %s", len(unexpected), msg)
	} else {
		t.Logf("%d %s", len(unexpected), msg)
	}
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n  "
		}
		out += l
	}
	return out
}
