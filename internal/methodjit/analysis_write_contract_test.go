package methodjit

import "testing"

func TestCheckModuleWriteContract_DeclaredWriteIsClean(t *testing.T) {
	run := Tier2ModuleRun{
		Phase:          Tier2PhaseNumeric,
		ModuleName:     "RangeAnalysis",
		Provides:       []AnalysisFact{AnalysisFactInt48Safe, AnalysisFactIntRanges},
		ChangedDomains: []string{"int48Safe", "intRanges"},
	}
	report := CheckModuleWriteContract(run)
	if report.HasViolations() {
		t.Fatalf("expected no violations, got: %s", FormatWriteContractViolations(report.Violations))
	}
	if len(report.UnmodeledDomains) != 0 {
		t.Fatalf("expected no unmodeled domains, got %v", report.UnmodeledDomains)
	}
}

func TestCheckModuleWriteContract_UndeclaredWriteIsViolation(t *testing.T) {
	// The exact scenario from the design review: a module declares Int48Safe
	// but secretly also mutates IntModNoSignAdjust.
	run := Tier2ModuleRun{
		Phase:          Tier2PhaseNumeric,
		ModuleName:     "OverflowBoxing",
		Provides:       []AnalysisFact{AnalysisFactInt48Safe},
		ChangedDomains: []string{"int48Safe", "intModNoSignAdjust"},
	}
	report := CheckModuleWriteContract(run)
	if !report.HasViolations() {
		t.Fatalf("expected a violation for undeclared IntModNoSignAdjust write")
	}
	if len(report.Violations) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %s", len(report.Violations),
			FormatWriteContractViolations(report.Violations))
	}
	if got := report.Violations[0].Fact; got != AnalysisFactIntModNoSignAdjust {
		t.Fatalf("violation fact = %s, want %s", got, AnalysisFactIntModNoSignAdjust)
	}
}

func TestCheckModuleWriteContract_UpdatesCountAsDeclared(t *testing.T) {
	run := Tier2ModuleRun{
		Phase:          Tier2PhaseNumeric,
		ModuleName:     "RangeRefresh",
		Updates:        []AnalysisFact{AnalysisFactIntRanges},
		ChangedDomains: []string{"intRanges"},
	}
	if report := CheckModuleWriteContract(run); report.HasViolations() {
		t.Fatalf("Updates should satisfy the write contract, got: %s",
			FormatWriteContractViolations(report.Violations))
	}
}

func TestCheckModuleWriteContract_DomainStructPrefixResolves(t *testing.T) {
	// A domain-struct field surfaces as "Call.callABIs" and aliases the same
	// map as top-level "callABIs"; both must resolve to AnalysisFactCallABIs.
	run := Tier2ModuleRun{
		Phase:          Tier2PhaseCallLower,
		ModuleName:     "CallABI",
		Provides:       []AnalysisFact{AnalysisFactCallABIs},
		ChangedDomains: []string{"callABIs", "Call.callABIs"},
	}
	if report := CheckModuleWriteContract(run); report.HasViolations() {
		t.Fatalf("domain-struct prefix should resolve to the same fact, got: %s",
			FormatWriteContractViolations(report.Violations))
	}
}

func TestCheckModuleWriteContract_NewlyModeledDomainsAreViolationsWhenUndeclared(t *testing.T) {
	run := Tier2ModuleRun{
		Phase:          Tier2PhaseLoopPost,
		ModuleName:     "TableArrayStaticBounds",
		ChangedDomains: []string{"loopTableArrayFacts", "profiledIntRanges"},
	}
	report := CheckModuleWriteContract(run)
	if len(report.Violations) != 2 {
		t.Fatalf("expected modeled loop/profile writes to be violations, got %d: %s",
			len(report.Violations), FormatWriteContractViolations(report.Violations))
	}
	if len(report.UnmodeledDomains) != 0 {
		t.Fatalf("expected no unmodeled domains, got %v", report.UnmodeledDomains)
	}
}

func TestCheckModuleWriteContract_UnknownDomainIsReportedNotViolation(t *testing.T) {
	run := Tier2ModuleRun{
		Phase:          Tier2PhaseLoopPost,
		ModuleName:     "FuturePass",
		ChangedDomains: []string{"futureUnmodeledFact"},
	}
	report := CheckModuleWriteContract(run)
	if report.HasViolations() {
		t.Fatalf("unknown fields must not be violations, got: %s",
			FormatWriteContractViolations(report.Violations))
	}
	if len(report.UnmodeledDomains) != 1 || report.UnmodeledDomains[0] != "futureUnmodeledFact" {
		t.Fatalf("unmodeled domains = %v, want [futureUnmodeledFact]", report.UnmodeledDomains)
	}
}

func TestCheckPipelineWriteContract_AggregatesAndDedups(t *testing.T) {
	runs := []Tier2ModuleRun{
		{
			Phase:          Tier2PhaseNumeric,
			ModuleName:     "A",
			Provides:       []AnalysisFact{AnalysisFactInt48Safe},
			ChangedDomains: []string{"int48Safe", "intRanges"}, // IntRanges undeclared
		},
		{
			Phase:          Tier2PhaseLoopPost,
			ModuleName:     "B",
			ChangedDomains: []string{"futureUnmodeledFact", "futureUnmodeledFact"}, // unmodeled, duplicate
		},
	}
	agg := CheckPipelineWriteContract(runs)
	if len(agg.Violations) != 1 {
		t.Fatalf("expected 1 aggregate violation, got %d", len(agg.Violations))
	}
	if len(agg.UnmodeledDomains) != 1 || agg.UnmodeledDomains[0] != "futureUnmodeledFact" {
		t.Fatalf("unmodeled = %v, want [futureUnmodeledFact]", agg.UnmodeledDomains)
	}
}
