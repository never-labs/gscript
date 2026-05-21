package methodjit

import (
	"fmt"
	"sort"
)

// ModuleBuilder constructs a slice of Tier2OptimizerModules given the
// optimizer context. Builder functions are registered via
// RegisterModuleBuilder (typically from an init() in the domain-specific
// file) and collected by BuildModulePlan.
type ModuleBuilder func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule

// DefaultModuleRegistry holds the process-wide module builders registered by
// init functions. Tests that need isolation should create their own registry
// with NewModuleRegistry.
var DefaultModuleRegistry = NewModuleRegistry()

// ModuleRegistry stores module builders in insertion order.
type ModuleRegistry struct {
	builders []moduleBuilderEntry
}

type moduleBuilderEntry struct {
	phase   Tier2OptimizerPhase
	order   int // determines execution order among phases
	builder ModuleBuilder
}

// NewModuleRegistry creates an empty module registry.
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{}
}

// RegisterModuleBuilder adds a module builder for the given phase.
// The order parameter controls the position of this phase relative to
// other phases when BuildModulePlan constructs the final plan.
// Call this from an init() function in each domain-specific module file.
func RegisterModuleBuilder(phase Tier2OptimizerPhase, order int, builder ModuleBuilder) {
	if err := DefaultModuleRegistry.RegisterModuleBuilder(phase, order, builder); err != nil {
		panic(err)
	}
}

// RegisterModuleBuilder adds a module builder to this registry.
func (r *ModuleRegistry) RegisterModuleBuilder(phase Tier2OptimizerPhase, order int, builder ModuleBuilder) error {
	if r == nil {
		return fmt.Errorf("methodjit: register module builder on nil registry")
	}
	if builder == nil {
		return fmt.Errorf("methodjit: register module builder for phase %s with nil builder", phase)
	}
	for _, existing := range r.builders {
		if existing.phase == phase && existing.order != order {
			return fmt.Errorf("methodjit: phase %s already registered with order %d, cannot also use order %d", phase, existing.order, order)
		}
		if existing.phase != phase && existing.order == order {
			return fmt.Errorf("methodjit: duplicate module builder order %d for phases %s and %s", order, existing.phase, phase)
		}
	}
	r.builders = append(r.builders, moduleBuilderEntry{
		phase:   phase,
		order:   order,
		builder: builder,
	})
	return nil
}

// BuildModulePlan constructs the ordered module plan from all registered
// builders. It returns a Tier2OptimizerPlan with phases sorted by their
// declared order and the concatenated module lists from each builder.
func BuildModulePlan(ctx *Tier2OptimizerContext) Tier2OptimizerPlan {
	return DefaultModuleRegistry.BuildModulePlan(ctx)
}

// BuildModulePlan constructs the ordered module plan from this registry.
func (r *ModuleRegistry) BuildModulePlan(ctx *Tier2OptimizerContext) Tier2OptimizerPlan {
	if r == nil {
		return Tier2OptimizerPlan{}
	}
	// Sort builders by order to determine phase sequence.
	sorted := make([]moduleBuilderEntry, len(r.builders))
	copy(sorted, r.builders)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].order < sorted[j].order
	})

	phases := make([]Tier2OptimizerPhase, 0, len(sorted))
	var modules []Tier2OptimizerModule
	groupIndex := make(map[Tier2OptimizerPhase]int, len(sorted))
	var groups []Tier2OptimizerPhaseGroup
	for _, entry := range sorted {
		built := entry.builder(ctx)
		if idx, ok := groupIndex[entry.phase]; ok {
			groups[idx].Modules = append(groups[idx].Modules, built...)
		} else {
			groupIndex[entry.phase] = len(groups)
			phases = append(phases, entry.phase)
			groups = append(groups, Tier2OptimizerPhaseGroup{
				Phase:   entry.phase,
				Modules: built,
			})
		}
		modules = append(modules, built...)
	}
	return Tier2OptimizerPlan{
		Phases:      phases,
		Modules:     modules,
		PhaseGroups: groups,
	}
}
