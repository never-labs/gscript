package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericAIDialectStaysOutOfLeiaCoreBuiltins(t *testing.T) {
	root := repoRoot(t)

	for _, rel := range []string{
		"examples/ai/finrobot_translation/ai_dialect_index/README.md",
		"examples/ai/finrobot_translation/ai_dialect_index/PACKAGE_BOUNDARIES.md",
		"examples/ai/finrobot_translation/generic_ai_workflow_composition.leia",
		"examples/ai/finrobot_translation/generic_workflow_orchestration.leia",
	} {
		data := readGenericAINoBuiltinFile(t, filepath.Join(root, filepath.FromSlash(rel)))
		assertGenericAINoCoreBuiltinClaims(t, rel, data)
	}

	index := loadGenericAIDialectIndex(t, root)
	if index.Scope.ImportsLivePackages || index.Scope.DependsOnQRuntime || index.Scope.FinRobotSpecificSyntaxAssumption {
		t.Fatalf("generic AI dialect index leaked runtime/language coupling: %#v", index.Scope)
	}
	for _, entry := range index.Entries {
		if entry.MissingProductionPackageBoundary.Status != "missing" {
			t.Fatalf("%s must keep missing production boundary explicit: %#v", entry.CapabilityID, entry.MissingProductionPackageBoundary)
		}
		for _, surface := range entry.DialectSurface {
			assertGenericAIDialectSurfaceIsPackageLevel(t, entry.CapabilityID, surface)
		}
	}

	packagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	for _, packageDir := range genericLivePackageDirs(t, packagesRoot) {
		manifestPath := filepath.Join(packageDir, "package.manifest.json")
		manifest := readJSONMap(t, manifestPath)
		assertGenericAIPackageNoBuiltinBoundary(t, manifestPath, manifest)
	}
}

func assertGenericAIPackageNoBuiltinBoundary(t *testing.T, manifestPath string, manifest map[string]any) {
	t.Helper()
	packageName, _ := manifest["package_name"].(string)
	if packageName == "" || !strings.HasPrefix(packageName, "leia-generic-ai-") {
		t.Fatalf("%s package_name must stay in leia-generic-ai-* namespace: %#v", manifestPath, manifest["package_name"])
	}
	if !finrobotLivePackageBoolOrConst(manifest["provider_free"], true) {
		t.Fatalf("%s provider_free must be true: %#v", manifestPath, manifest["provider_free"])
	}
	if value, ok := manifest["live_network"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
		t.Fatalf("%s live_network must default false: %#v", manifestPath, value)
	}
	if value, ok := manifest["live_network_default"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
		t.Fatalf("%s live_network_default must default false: %#v", manifestPath, value)
	}
	if value, ok := manifest["real_dependency_import_default"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
		t.Fatalf("%s real_dependency_import_default must default false: %#v", manifestPath, value)
	}

	guarantee, ok := manifest["no_built_in_guarantee"]
	if !ok {
		t.Fatalf("%s missing no_built_in_guarantee", manifestPath)
	}
	statement := genericAINoBuiltinStatement(t, manifestPath, guarantee)
	lower := strings.ToLower(statement)
	for _, want := range []string{"leia core", "does not provide", "built-in", packageName, "package boundary"} {
		if !strings.Contains(lower, strings.ToLower(want)) {
			t.Fatalf("%s no-built-in statement missing %q: %q", manifestPath, want, statement)
		}
	}
	assertGenericAINoCoreBuiltinClaims(t, manifestPath, []byte(statement))
}

func genericAINoBuiltinStatement(t *testing.T, manifestPath string, value any) string {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		if !finrobotLivePackageBoolOrConst(value["required"], true) {
			t.Fatalf("%s no_built_in_guarantee.required = %#v", manifestPath, value["required"])
		}
		statement, _ := value["statement"].(string)
		if statement == "" {
			t.Fatalf("%s no_built_in_guarantee.statement missing", manifestPath)
		}
		return statement
	case bool:
		if !value {
			t.Fatalf("%s no_built_in_guarantee=false", manifestPath)
		}
		return "Leia core does not provide built-in generic AI behavior; this package boundary remains external."
	default:
		t.Fatalf("%s unsupported no_built_in_guarantee shape %#v", manifestPath, value)
		return ""
	}
}

func assertGenericAIDialectSurfaceIsPackageLevel(t *testing.T, owner, surface string) {
	t.Helper()
	if surface == "" {
		t.Fatalf("%s has empty dialect surface", owner)
	}
	lower := strings.ToLower(surface)
	for _, forbidden := range []string{"builtin", "built-in", "core builtin", "core built-in", "language primitive", "q/runtime"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("%s dialect surface %q looks like a core built-in claim", owner, surface)
		}
	}
}

func assertGenericAINoCoreBuiltinClaims(t *testing.T, rel string, data []byte) {
	t.Helper()
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{
		"llm is a leia core built-in",
		"provider is a leia core built-in",
		"model provider is built into leia core",
		"finance provider is built into leia core",
		"generic ai is implemented in q/runtime",
		"import q/runtime",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("%s contains forbidden core built-in claim %q", rel, forbidden)
		}
	}
}

func readGenericAINoBuiltinFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(path, ".json") {
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	return data
}
