package methodjit

import (
	"strings"
	"testing"
)

func TestValidateDependencyOrderRejectsMultipleProviders(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical},
		Modules: []Tier2OptimizerModule{
			dependencyCheckTestModule("Provider1", Tier2PhaseEarlyCanonical, nil, []AnalysisFact{"SharedFact"}, nil),
			dependencyCheckTestModule("Provider2", Tier2PhaseEarlyCanonical, nil, []AnalysisFact{"SharedFact"}, nil),
			dependencyCheckTestModule("Consumer", Tier2PhaseEarlyCanonical, []AnalysisFact{"SharedFact"}, nil, nil),
		},
	}

	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected multiple provider error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple providers") || !strings.Contains(err.Error(), "SharedFact") {
		t.Fatalf("error should report multiple providers for SharedFact, got: %v", err)
	}
}

func TestValidateDependencyOrderRejectsUnusedFact(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical},
		Modules: []Tier2OptimizerModule{
			dependencyCheckTestModule("Provider", Tier2PhaseEarlyCanonical, nil, []AnalysisFact{"UnusedFact"}, nil),
		},
	}

	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected unused fact error, got nil")
	}
	if !strings.Contains(err.Error(), "UnusedFact") || !strings.Contains(err.Error(), "no declared consumers") {
		t.Fatalf("error should report unused fact, got: %v", err)
	}
}

func TestValidateDependencyOrderTreatsOptionalReadAsConsumerWithoutHardDependency(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical},
		Modules: []Tier2OptimizerModule{
			{
				Name:          "OptionalReader",
				Phase:         Tier2PhaseEarlyCanonical,
				OptionalReads: analysisFacts(AnalysisFactGlobals),
				Run:           func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
		},
	}

	if err := ValidateDependencyOrder(plan); err != nil {
		t.Fatalf("optional reads should not require a provider or ordering edge, got: %v", err)
	}
}

func TestValidateDependencyOrderCountsOptionalReadAsFactConsumer(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical},
		Modules: []Tier2OptimizerModule{
			{
				Name:     "Provider",
				Phase:    Tier2PhaseEarlyCanonical,
				Provides: analysisFacts(AnalysisFactGlobals),
				Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
			{
				Name:          "OptionalReader",
				Phase:         Tier2PhaseEarlyCanonical,
				OptionalReads: analysisFacts(AnalysisFactGlobals),
				Run:           func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
		},
	}

	if err := ValidateDependencyOrder(plan); err != nil {
		t.Fatalf("optional reads should count as consumers without requiring order, got: %v", err)
	}
}

func TestValidateDependencyOrderRejectsUnregisteredOptionalRead(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical},
		Modules: []Tier2OptimizerModule{
			{
				Name:          "OptionalReader",
				Phase:         Tier2PhaseEarlyCanonical,
				OptionalReads: []AnalysisFact{"UnknownOptionalFact"},
				Run:           func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
			},
		},
	}

	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected unregistered optional read error, got nil")
	}
	if !strings.Contains(err.Error(), "optional-reads unregistered fact UnknownOptionalFact") {
		t.Fatalf("error should report unregistered optional read, got: %v", err)
	}
}

func TestValidateDependencyOrderRejectsSelfDependency(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical},
		Modules: []Tier2OptimizerModule{
			dependencyCheckTestModule("Self", Tier2PhaseEarlyCanonical, []AnalysisFact{"SelfFact"}, []AnalysisFact{"SelfFact"}, nil),
		},
	}

	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected self-dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "self-dependency") || !strings.Contains(err.Error(), "SelfFact") {
		t.Fatalf("error should report self-dependency, got: %v", err)
	}
}

func TestValidateDependencyOrderRejectsSimpleCycle(t *testing.T) {
	plan := Tier2OptimizerPlan{
		Phases: []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical},
		Modules: []Tier2OptimizerModule{
			dependencyCheckTestModule("ProviderA", Tier2PhaseEarlyCanonical, []AnalysisFact{"FactB"}, []AnalysisFact{"FactA"}, nil),
			dependencyCheckTestModule("ProviderB", Tier2PhaseEarlyCanonical, []AnalysisFact{"FactA"}, []AnalysisFact{"FactB"}, nil),
		},
	}

	err := ValidateDependencyOrder(plan)
	if err == nil {
		t.Fatal("expected dependency cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "module dependency cycle") || !strings.Contains(err.Error(), "ProviderA") || !strings.Contains(err.Error(), "ProviderB") {
		t.Fatalf("error should report simple cycle, got: %v", err)
	}
}

func dependencyCheckTestModule(name string, phase Tier2OptimizerPhase, requires, provides, updates []AnalysisFact) Tier2OptimizerModule {
	return Tier2OptimizerModule{
		Name:     name,
		Phase:    phase,
		Requires: requires,
		Provides: provides,
		Updates:  updates,
		Run:      func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) { return fn, nil },
	}
}
