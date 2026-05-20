package methodjit

import (
	"fmt"
	"sort"
)

// ValidateDependencyOrder checks that all module dependencies are satisfied
// in the given optimizer plan. It iterates through phases and modules in order,
// tracking which facts have been provided so far. If a module Requires a fact
// that hasn't been provided yet, it returns an error with details.
//
// This is intended to be called at initialization or test time, not in the
// production hot path.
func ValidateDependencyOrder(plan Tier2OptimizerPlan) error {
	provided := make(map[string]string) // fact -> module that provided it
	providers := make(map[string][]string)

	// First pass: collect all facts that are provided and by which modules
	for _, module := range plan.Modules {
		for _, fact := range module.Provides {
			if existingProvider, ok := provided[fact]; ok {
				return fmt.Errorf("fact %s is provided by multiple modules: %s and %s", fact, existingProvider, module.Name)
			}
			provided[fact] = module.Name
			providers[module.Name] = append(providers[module.Name], fact)
		}
	}

	// Second pass: verify that all required facts are available before each module runs
	available := make(map[string]bool)
	for _, phase := range plan.Phases {
		for _, module := range plan.Modules {
			if module.Phase != phase {
				continue
			}

			// Check that all required facts are available
			for _, required := range module.Requires {
				if !available[required] {
					provider, ok := provided[required]
					if !ok {
						return fmt.Errorf("%s/%s requires fact %s which is never provided", phase, module.Name, required)
					}
					return fmt.Errorf("%s/%s requires fact %s which is provided by %s (but not yet available)", phase, module.Name, required, provider)
				}
			}

			// Mark this module's facts as available for subsequent modules
			for _, providedFact := range module.Provides {
				available[providedFact] = true
			}
		}
	}

	// Optional: warn about facts that are provided but never required
	// (these might still be used by codegen or other infrastructure)
	neverRequired := []string{}
	for fact := range provided {
		required := false
		for _, module := range plan.Modules {
			for _, req := range module.Requires {
				if req == fact {
					required = true
					break
				}
			}
			if required {
				break
			}
		}
		if !required {
			neverRequired = append(neverRequired, fact)
		}
	}
	if len(neverRequired) > 0 {
		sort.Strings(neverRequired)
		// This is informational, not an error
		return nil
	}

	return nil
}