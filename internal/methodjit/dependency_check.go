package methodjit

import (
	"fmt"
	"sort"
	"strings"
)

// ValidateDependencyOrder checks that all module dependencies are satisfied in
// the given optimizer plan. It iterates through phases and modules in order,
// tracking which facts have been provided so far. Requires facts must be
// available before a module runs. Updates facts must also be available; they
// document refreshes of an existing fact rather than first-time production.
// OptionalReads are validated facts and count as consumers, but deliberately do
// not create ordering edges.
//
// This is intended to be called at initialization or test time, not in the
// production hot path.
func ValidateDependencyOrder(plan Tier2OptimizerPlan) error {
	var issues []string
	modules := dependencyPlanModules(plan)
	provided := make(map[AnalysisFact]string) // fact -> module that first provided it
	providersByFact := make(map[AnalysisFact][]string)
	consumed := make(map[AnalysisFact]bool)
	moduleDeps := make(map[string][]string)
	moduleRefs := make(map[string]bool)

	// First pass: collect all facts that are provided and by which modules.
	for _, module := range modules {
		ref := dependencyModuleRef(module)
		moduleRefs[ref] = true
		for _, fact := range module.Provides {
			validateDeclaredAnalysisFact(ref, "provides", fact, &issues)
			if _, ok := provided[fact]; !ok {
				provided[fact] = ref
			}
			providersByFact[fact] = append(providersByFact[fact], ref)
		}
	}

	for _, module := range modules {
		ref := dependencyModuleRef(module)
		for _, fact := range module.Requires {
			consumed[fact] = true
			validateDeclaredAnalysisFact(ref, "requires", fact, &issues)
			if provider, ok := provided[fact]; ok && provider != ref {
				moduleDeps[ref] = append(moduleDeps[ref], provider)
			}
		}
		for _, fact := range module.Updates {
			consumed[fact] = true
			validateDeclaredAnalysisFact(ref, "updates", fact, &issues)
			if provider, ok := provided[fact]; ok && provider != ref {
				moduleDeps[ref] = append(moduleDeps[ref], provider)
			}
		}
		for _, fact := range module.OptionalReads {
			consumed[fact] = true
			validateDeclaredAnalysisFact(ref, "optional-reads", fact, &issues)
		}
	}

	for fact, providers := range providersByFact {
		if len(providers) <= 1 {
			continue
		}
		sort.Strings(providers)
		issues = append(issues, fmt.Sprintf("fact %s has multiple providers: %s", fact, strings.Join(providers, ", ")))
	}

	for _, module := range modules {
		ref := dependencyModuleRef(module)
		provides := factSet(module.Provides)
		requires := factSet(module.Requires)
		updates := factSet(module.Updates)
		for _, fact := range module.Requires {
			if provides[fact] {
				issues = append(issues, fmt.Sprintf("%s has self-dependency on fact %s", ref, fact))
			}
		}
		for _, fact := range module.Updates {
			if provides[fact] {
				issues = append(issues, fmt.Sprintf("%s both provides and updates fact %s", ref, fact))
			}
		}
		for _, fact := range module.OptionalReads {
			switch {
			case requires[fact]:
				issues = append(issues, fmt.Sprintf("%s declares fact %s as both required and optional", ref, fact))
			case provides[fact]:
				issues = append(issues, fmt.Sprintf("%s declares fact %s as both provided and optional", ref, fact))
			case updates[fact]:
				issues = append(issues, fmt.Sprintf("%s declares fact %s as both updated and optional", ref, fact))
			}
		}
	}

	// Second pass: verify that all required facts are available before each module runs
	available := make(map[AnalysisFact]bool)
	for _, group := range plan.phaseGroups() {
		for _, module := range group.Modules {
			ref := dependencyModuleRef(module)
			// Check that all required facts are available
			for _, required := range module.Requires {
				if !available[required] {
					provider, ok := provided[required]
					if !ok {
						issues = append(issues, fmt.Sprintf("%s requires fact %s which is never provided", ref, required))
						continue
					}
					issues = append(issues, fmt.Sprintf("%s requires fact %s which is provided by %s (but not yet available)", ref, required, provider))
				}
			}
			for _, updated := range module.Updates {
				if !available[updated] {
					provider, ok := provided[updated]
					if !ok {
						issues = append(issues, fmt.Sprintf("%s updates fact %s which is never provided", ref, updated))
						continue
					}
					issues = append(issues, fmt.Sprintf("%s updates fact %s which is provided by %s (but not yet available)", ref, updated, provider))
				}
			}

			// Mark this module's facts as available for subsequent modules
			for _, providedFact := range module.Provides {
				available[providedFact] = true
			}
		}
	}

	for fact := range provided {
		if consumed[fact] {
			continue
		}
		metadata, ok := lookupAnalysisFactMetadata(fact)
		if ok && len(metadata.Consumers) > 0 {
			continue
		}
		issues = append(issues, fmt.Sprintf("fact %s is provided by %s but has no declared consumers", fact, provided[fact]))
	}

	for _, cycle := range findModuleDependencyCycles(moduleDeps, moduleRefs) {
		issues = append(issues, fmt.Sprintf("module dependency cycle: %s", strings.Join(cycle, " -> ")))
	}

	if len(issues) == 0 {
		return nil
	}
	sort.Strings(issues)
	return fmt.Errorf("analysis fact contract violations:\n  %s", strings.Join(issues, "\n  "))
}

func dependencyPlanModules(plan Tier2OptimizerPlan) []Tier2OptimizerModule {
	if len(plan.Modules) > 0 {
		return plan.Modules
	}
	var modules []Tier2OptimizerModule
	for _, group := range plan.phaseGroups() {
		modules = append(modules, group.Modules...)
	}
	return modules
}

func validateDeclaredAnalysisFact(moduleRef, relation string, fact AnalysisFact, issues *[]string) {
	if _, ok := lookupAnalysisFactMetadata(fact); ok {
		return
	}
	*issues = append(*issues, fmt.Sprintf("%s %s unregistered fact %s", moduleRef, relation, fact))
}

func dependencyModuleRef(module Tier2OptimizerModule) string {
	return fmt.Sprintf("%s/%s", module.Phase, module.Name)
}

func factSet(facts []AnalysisFact) map[AnalysisFact]bool {
	out := make(map[AnalysisFact]bool, len(facts))
	for _, fact := range facts {
		out[fact] = true
	}
	return out
}

func findModuleDependencyCycles(deps map[string][]string, moduleRefs map[string]bool) [][]string {
	var cycles [][]string
	visited := make(map[string]bool, len(moduleRefs))
	inStack := make(map[string]bool, len(moduleRefs))
	var stack []string

	var visit func(string)
	visit = func(module string) {
		if inStack[module] {
			cycle := []string{module}
			for i := len(stack) - 1; i >= 0; i-- {
				cycle = append(cycle, stack[i])
				if stack[i] == module {
					break
				}
			}
			reverseStrings(cycle)
			cycles = append(cycles, cycle)
			return
		}
		if visited[module] {
			return
		}
		visited[module] = true
		inStack[module] = true
		stack = append(stack, module)

		next := append([]string(nil), deps[module]...)
		sort.Strings(next)
		for _, dep := range next {
			visit(dep)
		}

		stack = stack[:len(stack)-1]
		inStack[module] = false
	}

	refs := make([]string, 0, len(moduleRefs))
	for ref := range moduleRefs {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		visit(ref)
	}
	return cycles
}

func reverseStrings(values []string) {
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
}
