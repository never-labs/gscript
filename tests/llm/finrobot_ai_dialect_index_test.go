package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type genericAIDialectIndex struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Baseline      string `json:"baseline_branch"`
	Scope         struct {
		TranslationRoot                  string `json:"translation_root"`
		IndexDirectory                   string `json:"index_directory"`
		ProviderFree                     bool   `json:"provider_free"`
		DomainSpecific                   bool   `json:"domain_specific"`
		ReadOnly                         bool   `json:"read_only"`
		ImportsLivePackages              bool   `json:"imports_live_packages"`
		DependsOnQRuntime                bool   `json:"depends_on_q_runtime"`
		FinRobotSpecificSyntaxAssumption bool   `json:"finrobot_specific_syntax_assumption"`
	} `json:"scope"`
	Rules                map[string]bool             `json:"rules"`
	RequiredCapabilities []string                    `json:"required_capabilities"`
	Entries              []genericAIDialectIndexItem `json:"entries"`
}

type genericAIDialectIndexItem struct {
	Capability                       string   `json:"capability"`
	CapabilityID                     string   `json:"capability_id"`
	ProviderFree                     bool     `json:"provider_free"`
	DomainSpecific                   bool     `json:"domain_specific"`
	FinRobotSpecificSyntaxAssumption bool     `json:"finrobot_specific_syntax_assumption"`
	DialectSurface                   []string `json:"dialect_surface"`
	Example                          string   `json:"example"`
	Test                             string   `json:"test"`
	Fixture                          string   `json:"fixture"`
	MissingProductionPackageBoundary struct {
		PackageID string `json:"package_id"`
		Boundary  string `json:"boundary"`
		Status    string `json:"status"`
		Reason    string `json:"reason"`
	} `json:"missing_production_package_boundary"`
}

func TestFinRobotGenericAIDialectPackageIndexAudit(t *testing.T) {
	root := repoRoot(t)
	index := loadGenericAIDialectIndex(t, root)

	if index.SchemaVersion != 1 ||
		index.ID != "generic-ai-dialect-package-index-audit" ||
		index.Baseline != "origin/codex/ai-dialect-polish" {
		t.Fatalf("unexpected index header: %#v", index)
	}
	if index.Scope.TranslationRoot != "examples/ai/finrobot_translation" ||
		index.Scope.IndexDirectory != "examples/ai/finrobot_translation/ai_dialect_index" ||
		!index.Scope.ProviderFree ||
		index.Scope.DomainSpecific ||
		!index.Scope.ReadOnly ||
		index.Scope.ImportsLivePackages ||
		index.Scope.DependsOnQRuntime ||
		index.Scope.FinRobotSpecificSyntaxAssumption {
		t.Fatalf("index scope is not provider-free and generic: %#v", index.Scope)
	}
	for _, rule := range []string{
		"reference_paths_exist",
		"fixtures_are_provider_free",
		"tests_are_index_driven",
		"no_q_runtime_dependency",
		"no_finrobot_specific_syntax_assumption",
		"missing_production_package_boundary_recorded",
	} {
		if !index.Rules[rule] {
			t.Fatalf("index rule %q is not enabled", rule)
		}
	}

	seenCapabilities := map[string]bool{}
	seenIDs := map[string]bool{}
	for _, entry := range index.Entries {
		entry := entry
		t.Run(entry.Capability, func(t *testing.T) {
			if entry.Capability == "" || entry.CapabilityID == "" {
				t.Fatalf("entry is missing capability identity: %#v", entry)
			}
			if seenIDs[entry.CapabilityID] {
				t.Fatalf("duplicate capability_id %q", entry.CapabilityID)
			}
			seenIDs[entry.CapabilityID] = true
			seenCapabilities[entry.Capability] = true
			if !entry.ProviderFree || entry.DomainSpecific || entry.FinRobotSpecificSyntaxAssumption {
				t.Fatalf("%s is not provider-free and generic: %#v", entry.Capability, entry)
			}
			if len(entry.DialectSurface) == 0 {
				t.Fatalf("%s has no dialect surface", entry.Capability)
			}
			assertGenericAIDialectReference(t, root, entry.Example, true)
			assertGenericAIDialectReference(t, root, entry.Test, true)
			assertGenericAIDialectReference(t, root, entry.Fixture, true)
			assertGenericAIDialectFixtureProviderFree(t, root, entry.Fixture)
			assertGenericAIDialectNoLivePackageReference(t, entry)
			assertGenericAIDialectNoFinRobotSyntaxAssumption(t, entry)
			if entry.MissingProductionPackageBoundary.PackageID == "" ||
				entry.MissingProductionPackageBoundary.Boundary == "" ||
				entry.MissingProductionPackageBoundary.Status != "missing" ||
				entry.MissingProductionPackageBoundary.Reason == "" {
				t.Fatalf("%s missing production package boundary is incomplete: %#v", entry.Capability, entry.MissingProductionPackageBoundary)
			}
		})
	}

	for _, capability := range index.RequiredCapabilities {
		if !seenCapabilities[capability] {
			t.Fatalf("required capability %q is not represented in entries; got %v", capability, sortedStringKeys(seenCapabilities))
		}
	}
}

func loadGenericAIDialectIndex(t *testing.T, root string) genericAIDialectIndex {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "ai_dialect_index", "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var index genericAIDialectIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return index
}

func assertGenericAIDialectReference(t *testing.T, root, rel string, scanForQRuntime bool) {
	t.Helper()
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") {
		t.Fatalf("invalid relative reference %q", rel)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("reference %q: %v", rel, err)
	}
	if info.IsDir() {
		t.Fatalf("reference %q points to a directory", rel)
	}
	if scanForQRuntime {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		forbiddenRuntime := "q/" + "runtime"
		if strings.Contains(strings.ToLower(string(data)), forbiddenRuntime) {
			t.Fatalf("reference %q must not depend on the q runtime package", rel)
		}
	}
}

func assertGenericAIDialectFixtureProviderFree(t *testing.T, root, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	lower := strings.ToLower(string(data))
	if strings.Contains(lower, `"provider_free": false`) ||
		strings.Contains(lower, `"live_network": true`) ||
		strings.Contains(lower, `"live_model": true`) {
		t.Fatalf("fixture %q is not provider-free", rel)
	}
}

func assertGenericAIDialectNoLivePackageReference(t *testing.T, entry genericAIDialectIndexItem) {
	t.Helper()
	for _, rel := range []string{entry.Example, entry.Test, entry.Fixture} {
		if strings.Contains(filepath.ToSlash(rel), "/live_packages/") {
			t.Fatalf("%s references live_packages path %q", entry.Capability, rel)
		}
	}
}

func assertGenericAIDialectNoFinRobotSyntaxAssumption(t *testing.T, entry genericAIDialectIndexItem) {
	t.Helper()
	values := []string{
		entry.Capability,
		entry.CapabilityID,
		strings.Join(entry.DialectSurface, " "),
		entry.MissingProductionPackageBoundary.PackageID,
		entry.MissingProductionPackageBoundary.Boundary,
		entry.MissingProductionPackageBoundary.Reason,
	}
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, forbidden := range []string{"finrobot.", "finrobot_", "autogen", "openbb", "fingpt"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s contains FinRobot-specific syntax assumption %q in %q", entry.Capability, forbidden, value)
			}
		}
	}
}

func sortedStringKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
