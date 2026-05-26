package methodjit

import (
	"fmt"
	"sort"
	"strings"
)

// analysis_write_contract.go enforces the Tier 2 optimizer's *write* contract:
// a module may only mutate AnalysisResult fact domains it declared in Provides
// or Updates. This converts the declarative Provides/Updates lists from
// documentation into an enforced invariant, closing the "looks modular, shares
// global state" gap on the write side.
//
// The check piggybacks on the existing per-module fact diff: PhaseScope.finish
// already computes run.ChangedDomains (the AnalysisResult fact-map domains a
// module actually mutated) by snapshotting before/after each module. We compare
// that observed write set against the declared one. It runs only when a module
// run is being observed (diagnostics, tests), never on the production hot path.
//
// Read-side enforcement (Requires) cannot be done by diffing, since reads leave
// no trace; it is handled separately by routing reads through scoped views.

// analysisFactForDomainField maps an AnalysisResult fact-map field name to the
// AnalysisFact that models it. Keys are the field names used by
// snapshotAnalysisFactDomains (a domain-struct field may also appear prefixed
// as "Domain.Field"; canonicalDomainField strips the prefix before lookup).
//
// Fields absent from this map are not yet modeled by the fact-contract system.
// Writes to them are reported as Unmodeled rather than treated as violations,
// so the gate does not false-positive on facts the contract layer never named.
// Growing this map (and the AnalysisFact set) toward full coverage is itself a
// tracked goal of the modularization.
// Keys are the (now unexported) domain-struct map field names produced by
// snapshotAnalysisFactDomains via reflection, so they are lowerCamel. The
// String* facts model Function-level maps that are not part of AnalysisResult
// and never surface through the AnalysisResult snapshot; their keys are kept for
// completeness only.
var analysisFactForDomainField = map[string]AnalysisFact{
	"int48Safe":                      AnalysisFactInt48Safe,
	"intModNonZeroDivisor":           AnalysisFactIntModNonZeroDivisor,
	"intModNoSignAdjust":             AnalysisFactIntModNoSignAdjust,
	"intRanges":                      AnalysisFactIntRanges,
	"intNonNegative":                 AnalysisFactIntNonNegative,
	"tableArrayDataPtrs":             AnalysisFactTableArrayDataPtrs,
	"shapeFieldTypeElidedLoads":      AnalysisFactShapeFieldTypeElided,
	"recordArrayLoopSpecializations": AnalysisFactRecordArrayLoopSpecialization,
	"fixedShapeTables":               AnalysisFactFixedShapeTables,
	"fixedShapeEntryGuards":          AnalysisFactFixedShapeEntryGuards,
	"fieldPolyShapeFacts":            AnalysisFactFieldPolyShapeFacts,
	"fieldPolyShapeCatalog":          AnalysisFactFieldPolyShapeCatalog,
	"fixedTableConstructors":         AnalysisFactFixedTableConstructors,
	"specDependencyProtos":           AnalysisFactSpecDependencyProtos,
	"callABIs":                       AnalysisFactCallABIs,
	"guardedConstCallFolds":          AnalysisFactGuardedConstCallFolds,

	"callSiteNoResultRuntimeSpecializations":       AnalysisFactCallSiteNoResultRuntimeSpecializations,
	"callSiteNoResultRuntimeSpecializationBatches": AnalysisFactCallSiteNoResultRuntimeSpecializationBatches,

	"StringConstTables":    AnalysisFactStringConstTables,
	"StringFormatPatterns": AnalysisFactStringFormatPatterns,
	"StringSplitSubSpecs":  AnalysisFactStringSplitSubSpecs,
}

// canonicalDomainField reduces a snapshot domain key to its bare field name.
// Top-level compatibility fields appear as "Field"; domain-struct fields appear
// as "Domain.Field" and alias the same underlying map, so both must resolve to
// the same fact.
func canonicalDomainField(domain string) string {
	if i := strings.LastIndexByte(domain, '.'); i >= 0 {
		return domain[i+1:]
	}
	return domain
}

// WriteContractViolation records a module that mutated a modeled fact it did not
// declare in Provides or Updates.
type WriteContractViolation struct {
	Phase  Tier2OptimizerPhase
	Module string
	Fact   AnalysisFact
	Domain string // the raw snapshot domain key that surfaced the write
}

func (v WriteContractViolation) String() string {
	return fmt.Sprintf("%s/%s wrote fact %s (domain %s) without declaring it in Provides/Updates",
		v.Phase, v.Module, v.Fact, v.Domain)
}

// WriteContractReport summarizes one module run's observed writes against its
// declared write contract.
type WriteContractReport struct {
	// Violations are writes to modeled facts the module did not declare.
	Violations []WriteContractViolation
	// UnmodeledDomains are changed domains with no AnalysisFact mapping. They
	// are not failures; they mark contract-coverage gaps to be closed.
	UnmodeledDomains []string
}

// HasViolations reports whether any declared-contract violation was found.
func (r WriteContractReport) HasViolations() bool { return len(r.Violations) > 0 }

// CheckModuleWriteContract verifies that every modeled fact domain mutated by
// the run was declared in its Provides or Updates list. It is pure and
// deterministic given the run; ChangedDomains must already be populated (it is,
// after PhaseScope.finish).
func CheckModuleWriteContract(run Tier2ModuleRun) WriteContractReport {
	declared := make(map[AnalysisFact]bool, len(run.Provides)+len(run.Updates))
	for _, f := range run.Provides {
		declared[f] = true
	}
	for _, f := range run.Updates {
		declared[f] = true
	}

	var report WriteContractReport
	seenFact := make(map[AnalysisFact]bool)
	seenUnmodeled := make(map[string]bool)
	for _, domain := range run.ChangedDomains {
		field := canonicalDomainField(domain)
		fact, ok := analysisFactForDomainField[field]
		if !ok {
			if !seenUnmodeled[field] {
				seenUnmodeled[field] = true
				report.UnmodeledDomains = append(report.UnmodeledDomains, field)
			}
			continue
		}
		if seenFact[fact] {
			continue
		}
		seenFact[fact] = true
		if !declared[fact] {
			report.Violations = append(report.Violations, WriteContractViolation{
				Phase:  run.Phase,
				Module: run.ModuleName,
				Fact:   fact,
				Domain: domain,
			})
		}
	}
	sort.Strings(report.UnmodeledDomains)
	return report
}

// AggregateWriteContractReport combines per-run reports across a whole pipeline
// execution, deduplicating unmodeled domains and preserving every violation.
type AggregateWriteContractReport struct {
	Violations       []WriteContractViolation
	UnmodeledDomains []string
}

// CheckPipelineWriteContract evaluates the write contract for every module run
// captured during a pipeline execution (e.g. via Tier2PipelineOpts.ModuleRuns).
func CheckPipelineWriteContract(runs []Tier2ModuleRun) AggregateWriteContractReport {
	var agg AggregateWriteContractReport
	unmodeled := make(map[string]bool)
	for _, run := range runs {
		report := CheckModuleWriteContract(run)
		agg.Violations = append(agg.Violations, report.Violations...)
		for _, d := range report.UnmodeledDomains {
			unmodeled[d] = true
		}
	}
	for d := range unmodeled {
		agg.UnmodeledDomains = append(agg.UnmodeledDomains, d)
	}
	sort.Strings(agg.UnmodeledDomains)
	return agg
}

// FormatWriteContractViolations renders violations for test failure messages.
func FormatWriteContractViolations(violations []WriteContractViolation) string {
	if len(violations) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(violations))
	for _, v := range violations {
		lines = append(lines, v.String())
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n  ")
}
