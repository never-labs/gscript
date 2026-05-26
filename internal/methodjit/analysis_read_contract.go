package methodjit

import (
	"fmt"
	"sort"
	"strings"
)

// analysis_read_contract.go observes the Tier 2 optimizer's *read* contract,
// symmetric to the write-side gate in analysis_write_contract.go. A module
// declares the facts it Requires/Provides/Updates; the union of those facts'
// owning domains is the set of domains the module is "allowed" to touch. The
// read observer (see analysis_fact_domain.go) records which domains a module
// actually accessed through the AnalysisResult accessors during an observed run.
// Any accessed domain not covered by a declared fact is an undeclared read.
//
// This is REPORT mode: it records and surfaces the landscape so the gaps can be
// closed deliberately. It never panics and never fails the build. Production
// cost is zero: the observer is nil unless the module runner installs it during
// an observed run (callback active), the same gating as the write-side diff.
//
// Limitation: the observer watches a single AnalysisResult (the one the pass
// mutates, fn.Analysis). If a pass internally builds a sub-Function with its own
// AnalysisResult (e.g. inlining), accesses on that sub-result are not observed.
// That is acceptable for report mode and noted here for honesty.

// allowedReadDomains returns the set of fact domains covered by a run's declared
// facts (Requires ∪ Provides ∪ Updates). A module that declares nothing has an
// empty allowed set, so every accessed domain is undeclared — exactly the
// interesting finding this observation is meant to surface.
func allowedReadDomains(run Tier2ModuleRun) map[factDomain]bool {
	allowed := make(map[factDomain]bool)
	add := func(facts []AnalysisFact) {
		for _, f := range facts {
			if d, ok := analysisFactDomain[f]; ok {
				allowed[d] = true
			}
		}
	}
	add(run.Requires)
	add(run.Provides)
	add(run.Updates)
	return allowed
}

// undeclaredReadDomains computes the sorted set of domains the run accessed but
// did not declare via any fact. It is pure and deterministic given the run's
// AccessedDomains and declared facts.
func undeclaredReadDomains(run Tier2ModuleRun) []string {
	if len(run.AccessedDomains) == 0 {
		return nil
	}
	allowed := allowedReadDomains(run)
	var out []string
	for _, d := range run.AccessedDomains {
		if !allowed[factDomain(d)] {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}

// ReadContractFinding records one module's undeclared read of a fact domain.
type ReadContractFinding struct {
	Phase  Tier2OptimizerPhase
	Module string
	Domain string
}

func (f ReadContractFinding) String() string {
	return fmt.Sprintf("%s/%s read domain %s without declaring a fact in that domain (Requires/Provides/Updates)",
		f.Phase, f.Module, f.Domain)
}

// AggregateReadContractReport summarizes undeclared reads across a whole
// pipeline execution. Findings are deduplicated per (module, domain) and the
// per-module domain map gives the landscape at a glance.
type AggregateReadContractReport struct {
	// Findings is the deduplicated, sorted list of undeclared reads.
	Findings []ReadContractFinding
	// ByModule maps a module name to the sorted set of domains it read
	// undeclared, across all observed runs of that module.
	ByModule map[string][]string
	// ModuleRunsWithUndeclaredReads counts module runs (not unique modules) that
	// accessed at least one undeclared domain.
	ModuleRunsWithUndeclaredReads int
	// ObservedRuns counts module runs whose access set was observed at all.
	ObservedRuns int
}

// CheckPipelineReadContract evaluates the read contract for every module run
// captured during a pipeline execution (e.g. via Tier2PipelineOpts.ModuleRuns).
// It is pure and deterministic given the runs.
func CheckPipelineReadContract(runs []Tier2ModuleRun) AggregateReadContractReport {
	agg := AggregateReadContractReport{ByModule: make(map[string][]string)}
	// dedupe per (module, domain) for findings, and per module for ByModule.
	seenFinding := make(map[string]bool)
	moduleDomains := make(map[string]map[string]bool)

	for _, run := range runs {
		if len(run.AccessedDomains) > 0 {
			agg.ObservedRuns++
		}
		undeclared := run.UndeclaredReadDomains
		if undeclared == nil {
			undeclared = undeclaredReadDomains(run)
		}
		if len(undeclared) > 0 {
			agg.ModuleRunsWithUndeclaredReads++
		}
		for _, d := range undeclared {
			key := run.ModuleName + "\x00" + d
			if !seenFinding[key] {
				seenFinding[key] = true
				agg.Findings = append(agg.Findings, ReadContractFinding{
					Phase:  run.Phase,
					Module: run.ModuleName,
					Domain: d,
				})
			}
			if moduleDomains[run.ModuleName] == nil {
				moduleDomains[run.ModuleName] = make(map[string]bool)
			}
			moduleDomains[run.ModuleName][d] = true
		}
	}

	for module, domains := range moduleDomains {
		list := make([]string, 0, len(domains))
		for d := range domains {
			list = append(list, d)
		}
		sort.Strings(list)
		agg.ByModule[module] = list
	}

	sort.Slice(agg.Findings, func(i, j int) bool {
		if agg.Findings[i].Module != agg.Findings[j].Module {
			return agg.Findings[i].Module < agg.Findings[j].Module
		}
		return agg.Findings[i].Domain < agg.Findings[j].Domain
	})
	return agg
}

// FormatReadContractFindings renders the per-module undeclared reads for logging.
func FormatReadContractFindings(report AggregateReadContractReport) string {
	if len(report.ByModule) == 0 {
		return "(none)"
	}
	modules := make([]string, 0, len(report.ByModule))
	for m := range report.ByModule {
		modules = append(modules, m)
	}
	sort.Strings(modules)
	var b strings.Builder
	for _, m := range modules {
		fmt.Fprintf(&b, "  %s: %s\n", m, strings.Join(report.ByModule[m], ", "))
	}
	return b.String()
}
