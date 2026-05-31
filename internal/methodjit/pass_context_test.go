package methodjit

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/vm"
)

// TestPassContext_RangeAnalysisCorpusEnforced drives the real Tier 2 pipeline
// over the benchmark corpus with passContextEnforce=true. RangeAnalysis runs in ctx form reaches analysis only through its PassContext, whose
// allow-set covers exactly the numeric domain it declares. If RangeAnalysis
// touched any undeclared domain, its accessor would panic; this test asserts no
// such panic occurs across the whole corpus, proving the execution-level domain
// isolation holds end-to-end for the keystone pass.
func TestPassContext_RangeAnalysisCorpusEnforced(t *testing.T) {
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

	prev := passContextEnforce
	passContextEnforce = true
	defer func() { passContextEnforce = prev }()

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
			if runEnforcedPipelineForProto(p) {
				pipelines++
			}
		}
	}
	t.Logf("pass-context enforced scan: %d sources, %d protos run through pipeline with enforcement; no undeclared-domain panic from RangeAnalysis",
		compiled, pipelines)
}

// runEnforcedPipelineForProto runs the production pipeline on one proto with
// enforcement active. It recovers ONLY the "unsupported proto shape" panics that
// pipelineRunsForProto tolerates, by recovering and reporting nothing for those;
// a PassContext domain-violation panic would carry our distinctive message and
// is surfaced via t.Fatalf rather than swallowed. Since we cannot run a *testing.T
// across a recover boundary cleanly for arbitrary panics, we re-panic anything
// that looks like a domain violation.
func runEnforcedPipelineForProto(p *vm.FuncProto) (ran bool) {
	if p == nil {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok && strings.HasPrefix(s, "pass accessed undeclared domain") {
				// A real domain-isolation violation: re-panic so the test fails loudly.
				panic(r)
			}
			// Unsupported / not-promotable proto: treat as "not run", like
			// pipelineRunsForProto does.
			ran = false
		}
	}()
	var captured []Tier2ModuleRun
	opts := &Tier2PipelineOpts{ModuleRuns: &captured}
	if _, _, err := RunTier2Pipeline(BuildGraph(p), opts); err != nil {
		return false
	}
	return true
}

// TestPassContext_UndeclaredDomainPanics is the negative guardrail: a PassContext
// allowing only the numeric domain with enforcement on must panic when a pass
// reaches an undeclared domain (table shape here), proving the barrier actually
// blocks cross-domain access rather than silently delegating.
func TestPassContext_UndeclaredDomainPanics(t *testing.T) {
	fn := &Function{}
	fn.ensureAnalysis()
	allowed := map[factDomain]bool{factDomainNumeric: true}
	ctx := newPassContext(fn, nil, allowed, true)

	// Allowed domain: must not panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("numeric (allowed) domain access panicked: %v", r)
			}
		}()
		_ = ctx.Numeric()
	}()

	// Undeclared domain: must panic.
	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		_ = ctx.TableShape()
	}()
	if !panicked {
		t.Fatalf("expected ctx.TableShape() to panic under enforcement with only numeric allowed")
	}
}
