package methodjit

import "sort"

// ModuleBuilder constructs a slice of Tier2OptimizerModules given the
// optimizer context. Builder functions are registered via
// RegisterModuleBuilder (typically from an init() in the domain-specific
// file) and collected by BuildModulePlan.
type ModuleBuilder func(ctx *Tier2OptimizerContext) []Tier2OptimizerModule

// moduleBuilders holds all registered module builders in insertion order.
// Each domain file registers its builder via init().
var moduleBuilders []moduleBuilderEntry

type moduleBuilderEntry struct {
	phase   Tier2OptimizerPhase
	order   int // determines execution order among phases
	builder ModuleBuilder
}

// RegisterModuleBuilder adds a module builder for the given phase.
// The order parameter controls the position of this phase relative to
// other phases when BuildModulePlan constructs the final plan.
// Call this from an init() function in each domain-specific module file.
func RegisterModuleBuilder(phase Tier2OptimizerPhase, order int, builder ModuleBuilder) {
	moduleBuilders = append(moduleBuilders, moduleBuilderEntry{
		phase:   phase,
		order:   order,
		builder: builder,
	})
}

// BuildModulePlan constructs the ordered module plan from all registered
// builders. It returns a Tier2OptimizerPlan with phases sorted by their
// declared order and the concatenated module lists from each builder.
func BuildModulePlan(ctx *Tier2OptimizerContext) Tier2OptimizerPlan {
	// Sort builders by order to determine phase sequence.
	sorted := make([]moduleBuilderEntry, len(moduleBuilders))
	copy(sorted, moduleBuilders)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].order < sorted[j].order
	})

	phases := make([]Tier2OptimizerPhase, 0, len(sorted))
	var modules []Tier2OptimizerModule
	for _, entry := range sorted {
		phases = append(phases, entry.phase)
		modules = append(modules, entry.builder(ctx)...)
	}
	return Tier2OptimizerPlan{
		Phases:  phases,
		Modules: modules,
	}
}
