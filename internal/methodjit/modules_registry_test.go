package methodjit

import (
	"strings"
	"testing"
)

func registryTestBuilder(phase Tier2OptimizerPhase, names ...string) ModuleBuilder {
	return func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule {
		modules := make([]Tier2OptimizerModule, 0, len(names))
		for _, name := range names {
			modules = append(modules, Tier2OptimizerModule{
				Name:  name,
				Phase: phase,
				Run: func(fn *Function, opts *Tier2PipelineOpts) (*Function, error) {
					return fn, nil
				},
			})
		}
		return modules
	}
}

func TestModuleRegistryBuildsIsolatedPlan(t *testing.T) {
	registry := NewModuleRegistry()
	if err := registry.RegisterModuleBuilder(Tier2PhaseNumeric, 20, registryTestBuilder(Tier2PhaseNumeric, "numeric")); err != nil {
		t.Fatalf("register numeric: %v", err)
	}
	if err := registry.RegisterModuleBuilder(Tier2PhaseEarlyCanonical, 10, registryTestBuilder(Tier2PhaseEarlyCanonical, "early")); err != nil {
		t.Fatalf("register early: %v", err)
	}

	plan := registry.BuildModulePlan(&Tier2OptimizerContext{})
	if got, want := plan.Phases, []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical, Tier2PhaseNumeric}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("phases=%v want %v", got, want)
	}
	if got, want := tier2ModuleNames(plan.Modules), []string{"early", "numeric"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("modules=%v want %v", got, want)
	}
}

func TestModuleRegistryRejectsNilBuilder(t *testing.T) {
	registry := NewModuleRegistry()
	err := registry.RegisterModuleBuilder(Tier2PhaseNumeric, 10, nil)
	if err == nil {
		t.Fatal("expected nil builder error")
	}
	if !strings.Contains(err.Error(), "nil builder") {
		t.Fatalf("error=%v want nil builder", err)
	}
}

func TestModuleRegistryRejectsDuplicateOrderAcrossPhases(t *testing.T) {
	registry := NewModuleRegistry()
	if err := registry.RegisterModuleBuilder(Tier2PhaseEarlyCanonical, 10, registryTestBuilder(Tier2PhaseEarlyCanonical, "early")); err != nil {
		t.Fatalf("register early: %v", err)
	}
	err := registry.RegisterModuleBuilder(Tier2PhaseNumeric, 10, registryTestBuilder(Tier2PhaseNumeric, "numeric"))
	if err == nil {
		t.Fatal("expected duplicate order error")
	}
	if !strings.Contains(err.Error(), "duplicate module builder order") {
		t.Fatalf("error=%v want duplicate order", err)
	}
}

func TestModuleRegistryGroupsSamePhaseBuilders(t *testing.T) {
	registry := NewModuleRegistry()
	if err := registry.RegisterModuleBuilder(Tier2PhaseNumeric, 20, registryTestBuilder(Tier2PhaseNumeric, "numeric-a")); err != nil {
		t.Fatalf("register numeric-a: %v", err)
	}
	if err := registry.RegisterModuleBuilder(Tier2PhaseEarlyCanonical, 10, registryTestBuilder(Tier2PhaseEarlyCanonical, "early")); err != nil {
		t.Fatalf("register early: %v", err)
	}
	if err := registry.RegisterModuleBuilder(Tier2PhaseNumeric, 20, registryTestBuilder(Tier2PhaseNumeric, "numeric-b")); err != nil {
		t.Fatalf("register numeric-b: %v", err)
	}

	plan := registry.BuildModulePlan(&Tier2OptimizerContext{})
	if got, want := plan.Phases, []Tier2OptimizerPhase{Tier2PhaseEarlyCanonical, Tier2PhaseNumeric}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("phases=%v want %v", got, want)
	}
	if len(plan.PhaseGroups) != 2 {
		t.Fatalf("phase group count=%d want 2: %#v", len(plan.PhaseGroups), plan.PhaseGroups)
	}
	if got := tier2ModuleNames(plan.PhaseGroups[1].Modules); len(got) != 2 || got[0] != "numeric-a" || got[1] != "numeric-b" {
		t.Fatalf("numeric group modules=%v want [numeric-a numeric-b]", got)
	}
	if got := tier2ModuleNames(plan.Modules); len(got) != 3 || got[0] != "early" || got[1] != "numeric-a" || got[2] != "numeric-b" {
		t.Fatalf("flat modules=%v want [early numeric-a numeric-b]", got)
	}
}

func TestModuleRegistryRejectsPhaseOrderChange(t *testing.T) {
	registry := NewModuleRegistry()
	if err := registry.RegisterModuleBuilder(Tier2PhaseNumeric, 20, registryTestBuilder(Tier2PhaseNumeric, "numeric-a")); err != nil {
		t.Fatalf("register numeric-a: %v", err)
	}
	err := registry.RegisterModuleBuilder(Tier2PhaseNumeric, 30, registryTestBuilder(Tier2PhaseNumeric, "numeric-b"))
	if err == nil {
		t.Fatal("expected phase order change error")
	}
	if !strings.Contains(err.Error(), "already registered with order") {
		t.Fatalf("error=%v want phase order conflict", err)
	}
}
