package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type finrobotPackageManifestAuditLedger struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	AuditKind     string `json:"audit_kind"`
	Scope         struct {
		LivePackagesRoot            string `json:"live_packages_root"`
		PlanManifest                string `json:"plan_manifest"`
		ReadOnly                    bool   `json:"read_only"`
		GenericPackageBoundary      bool   `json:"generic_package_boundary"`
		FixtureIndexCapabilityScope string `json:"fixture_index_capability_scope"`
	} `json:"scope"`
	Rules   map[string]bool `json:"rules"`
	Waivers struct {
		NoBuiltInGuarantee []struct {
			PackageID              string `json:"package_id"`
			Reason                 string `json:"reason"`
			CoveredByPlanGuarantee bool   `json:"covered_by_plan_guarantee"`
		} `json:"no_built_in_guarantee"`
	} `json:"waivers"`
}

func TestFinRobotPackageManifestConsistencyAudit(t *testing.T) {
	root := repoRoot(t)
	ledger := loadFinRobotPackageManifestAuditLedger(t, root)
	plan := loadLivePackagePlanManifest(t, root)

	if ledger.SchemaVersion != 1 ||
		ledger.ID != "finrobot-package-manifest-consistency-audit" ||
		ledger.AuditKind != "package_manifest_consistency" {
		t.Fatalf("unexpected audit ledger header: %#v", ledger)
	}
	if !ledger.Scope.ReadOnly || !ledger.Scope.GenericPackageBoundary {
		t.Fatalf("audit scope must be a read-only generic package boundary: %#v", ledger.Scope)
	}
	if ledger.Scope.FixtureIndexCapabilityScope != "all_package_manifests" {
		t.Fatalf("fixture index capability scope must cover all package manifests: %#v", ledger.Scope)
	}
	for _, rule := range []string{
		"provider_free",
		"live_network_disabled",
		"manifest_entrypoints_exist",
		"referenced_artifacts_exist",
		"capability_prefixes_match_plan",
		"fixture_index_capabilities_match_manifest_and_plan",
		"no_built_in_guarantee",
		"registered_example_mapping",
	} {
		if !ledger.Rules[rule] {
			t.Fatalf("audit rule %q is not enabled", rule)
		}
	}
	if !plan.NoBuiltInGuarantee.Required || !strings.Contains(strings.ToLower(plan.NoBuiltInGuarantee.Statement), "does not provide built-in") {
		t.Fatalf("plan no-built-in guarantee is missing or weak: %#v", plan.NoBuiltInGuarantee)
	}

	livePackagesRoot := filepath.Join(root, filepath.FromSlash(ledger.Scope.LivePackagesRoot))
	planBySkeletonDir := map[string]struct {
		ID                 string
		PackageName        string
		Capabilities       []string
		NoBuiltInGuarantee bool
	}{}
	for _, pkg := range plan.Packages {
		planBySkeletonDir[filepath.ToSlash(pkg.SkeletonDirectory)] = struct {
			ID                 string
			PackageName        string
			Capabilities       []string
			NoBuiltInGuarantee bool
		}{
			ID:                 pkg.ID,
			PackageName:        pkg.PackageName,
			Capabilities:       pkg.Capabilities,
			NoBuiltInGuarantee: pkg.NoBuiltInGuarantee,
		}
	}

	registeredExamples := finrobotAuditRegisteredExamples(t, root, plan)
	manifestPaths := finrobotAuditManifestPaths(t, livePackagesRoot)
	if len(manifestPaths) != len(plan.Packages) {
		t.Fatalf("manifest count = %d, want one per planned package (%d)", len(manifestPaths), len(plan.Packages))
	}

	waivedNoBuiltIn := finrobotAuditNoBuiltInWaivers(t, ledger)
	for _, manifestPath := range manifestPaths {
		relDir := filepath.ToSlash(mustRel(t, root, filepath.Dir(manifestPath)))
		planned, ok := planBySkeletonDir[relDir]
		if !ok {
			t.Fatalf("manifest %s is not mapped by live_package_plan_manifest.json", manifestPath)
		}
		t.Run(planned.ID, func(t *testing.T) {
			manifest := readJSONMap(t, manifestPath)
			assertFinRobotAuditProviderFree(t, manifestPath, manifest)
			assertFinRobotAuditNetworkDisabled(t, manifestPath, manifest)
			assertFinRobotAuditFixtureReplayDefaults(t, manifestPath, manifest)
			assertFinRobotAuditNoBuiltIn(t, manifestPath, planned.ID, planned.NoBuiltInGuarantee, waivedNoBuiltIn, manifest)
			assertFinRobotAuditEntrypoints(t, root, manifestPath, filepath.Dir(manifestPath), manifest)
			assertFinRobotAuditReferencedArtifacts(t, root, manifestPath, filepath.Dir(manifestPath), manifest)
			assertFinRobotAuditCapabilities(t, manifestPath, planned.PackageName, planned.Capabilities, manifest)
			assertFinRobotAuditFixtureIndexCapabilities(t, root, manifestPath, planned.Capabilities, manifest)
			if registered := registeredExamples[relDir]; registered == "" {
				t.Fatalf("%s has no registered example mapping", relDir)
			} else if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(registered))); err != nil {
				t.Fatalf("%s registered example %q: %v", relDir, registered, err)
			}
		})
	}
}

func loadFinRobotPackageManifestAuditLedger(t *testing.T, root string) finrobotPackageManifestAuditLedger {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "package_manifest_audit", "ledger.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ledger finrobotPackageManifestAuditLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return ledger
}

func finrobotAuditManifestPaths(t *testing.T, livePackagesRoot string) []string {
	t.Helper()
	entries, err := os.ReadDir(livePackagesRoot)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(livePackagesRoot, entry.Name())
		candidates := []string{
			filepath.Join(dir, "package.manifest.json"),
			filepath.Join(dir, "manifest.json"),
		}
		found := ""
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				if found != "" {
					t.Fatalf("%s has multiple skeleton manifests", dir)
				}
				found = candidate
			}
		}
		if found == "" {
			t.Fatalf("%s has no package manifest", dir)
		}
		paths = append(paths, found)
	}
	sort.Strings(paths)
	return paths
}

func finrobotAuditRegisteredExamples(t *testing.T, root string, plan livePackagePlanManifest) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, skeleton := range plan.LivePackageSkeletons {
		if skeleton.RegisteredExample == nil || *skeleton.RegisteredExample == "" {
			t.Fatalf("%s missing registered_example mapping", skeleton.ID)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(*skeleton.RegisteredExample))); err != nil {
			t.Fatalf("%s registered_example %q: %v", skeleton.ID, *skeleton.RegisteredExample, err)
		}
		result[filepath.ToSlash(skeleton.Directory)] = *skeleton.RegisteredExample
	}
	return result
}

func finrobotAuditNoBuiltInWaivers(t *testing.T, ledger finrobotPackageManifestAuditLedger) map[string]string {
	t.Helper()
	waivers := map[string]string{}
	for _, waiver := range ledger.Waivers.NoBuiltInGuarantee {
		if waiver.PackageID == "" || waiver.Reason == "" || !waiver.CoveredByPlanGuarantee {
			t.Fatalf("invalid no-built-in waiver: %#v", waiver)
		}
		waivers[waiver.PackageID] = waiver.Reason
	}
	return waivers
}

func assertFinRobotAuditProviderFree(t *testing.T, path string, manifest map[string]any) {
	t.Helper()
	if !finrobotLivePackageBoolOrConst(manifest["provider_free"], true) {
		t.Fatalf("%s provider_free = %#v, want true", path, manifest["provider_free"])
	}
	assertFinRobotAuditRecursiveBool(t, path, manifest, "provider_free", true)
}

func assertFinRobotAuditNetworkDisabled(t *testing.T, path string, manifest map[string]any) {
	t.Helper()
	if value, ok := manifest["live_network"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
		t.Fatalf("%s live_network = %#v, want false", path, value)
	}
	if value, ok := manifest["live_network_default"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
		t.Fatalf("%s live_network_default = %#v, want false", path, value)
	}
	assertFinRobotAuditRecursiveBool(t, path, manifest, "live_network", false)
	assertFinRobotAuditRecursiveBool(t, path, manifest, "allow_network", false)
	assertFinRobotAuditRecursiveBool(t, path, manifest, "real_dependency_imports", false)
	assertFinRobotAuditRecursiveBool(t, path, manifest, "real_dependency_import_default", false)
}

func assertFinRobotAuditFixtureReplayDefaults(t *testing.T, path string, manifest map[string]any) {
	t.Helper()
	if !finrobotLivePackageBoolOrConst(manifest["live_network_default"], false) {
		t.Fatalf("%s live_network_default = %#v, want false", path, manifest["live_network_default"])
	}
	if !finrobotLivePackageBoolOrConst(manifest["real_dependency_import_default"], false) {
		t.Fatalf("%s real_dependency_import_default = %#v, want false", path, manifest["real_dependency_import_default"])
	}
	policy, ok := manifest["default_policy"].(map[string]any)
	if !ok {
		t.Fatalf("%s missing default_policy map", path)
	}
	if mode, _ := policy["mode"].(string); mode != "fixture_replay" {
		t.Fatalf("%s default_policy.mode = %q, want fixture_replay", path, mode)
	}
	if !finrobotLivePackageBoolOrConst(policy["live_network"], false) ||
		!finrobotLivePackageBoolOrConst(policy["provider_credentials_required"], false) ||
		!finrobotLivePackageBoolOrConst(policy["real_dependency_imports"], false) {
		t.Fatalf("%s default_policy must disable live network, credentials, and real imports: %#v", path, policy)
	}
}

func assertFinRobotAuditRecursiveBool(t *testing.T, path string, value any, key string, want bool) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for childKey, child := range value {
			if childKey == key && !finrobotLivePackageBoolOrConst(child, want) {
				t.Fatalf("%s %s = %#v, want %v", path, key, child, want)
			}
			assertFinRobotAuditRecursiveBool(t, path, child, key, want)
		}
	case []any:
		for _, child := range value {
			assertFinRobotAuditRecursiveBool(t, path, child, key, want)
		}
	}
}

func assertFinRobotAuditNoBuiltIn(t *testing.T, path, packageID string, planned bool, waivers map[string]string, manifest map[string]any) {
	t.Helper()
	if !planned {
		t.Fatalf("%s plan package %s does not carry no_built_in_guarantee", path, packageID)
	}
	value, ok := manifest["no_built_in_guarantee"]
	if !ok {
		if waivers[packageID] == "" {
			t.Fatalf("%s missing no_built_in_guarantee and has no audit waiver", path)
		}
		return
	}
	switch value := value.(type) {
	case bool:
		if !value {
			t.Fatalf("%s no_built_in_guarantee = false", path)
		}
	case map[string]any:
		if !finrobotLivePackageBoolOrConst(value["required"], true) {
			t.Fatalf("%s no_built_in_guarantee.required = %#v, want true", path, value["required"])
		}
		statement, _ := value["statement"].(string)
		lowerStatement := strings.ToLower(statement)
		if !strings.Contains(lowerStatement, "does not provide") || !strings.Contains(lowerStatement, "built-in") {
			t.Fatalf("%s no_built_in_guarantee statement is missing the package boundary: %q", path, statement)
		}
	default:
		t.Fatalf("%s no_built_in_guarantee has unsupported shape %#v", path, value)
	}
}

func assertFinRobotAuditEntrypoints(t *testing.T, root, manifestPath, packageDir string, manifest map[string]any) {
	t.Helper()
	entrypoints, ok := manifest["entrypoints"].(map[string]any)
	if !ok || len(entrypoints) == 0 {
		if _, err := os.Stat(filepath.Join(packageDir, "main.leia")); err != nil {
			t.Fatalf("%s missing entrypoints and conventional main.leia: %v", manifestPath, err)
		}
		return
	}
	for name, value := range entrypoints {
		rel, ok := value.(string)
		if !ok || rel == "" {
			t.Fatalf("%s entrypoint %s = %#v, want relative path", manifestPath, name, value)
		}
		assertFinRobotAuditExistingRelativePath(t, root, manifestPath, packageDir, rel)
	}
}

func assertFinRobotAuditReferencedArtifacts(t *testing.T, root, manifestPath, packageDir string, manifest map[string]any) {
	t.Helper()
	for _, rel := range finrobotAuditPathReferences(manifest) {
		assertFinRobotAuditExistingRelativePath(t, root, manifestPath, packageDir, rel)
	}
	contractDir := filepath.Join(packageDir, "contracts")
	_ = filepath.WalkDir(contractDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		contract := readJSONMap(t, path)
		for _, rel := range finrobotAuditPathReferences(contract) {
			assertFinRobotAuditExistingRelativePath(t, root, path, packageDir, rel)
		}
		return nil
	})
}

func finrobotAuditPathReferences(value any) []string {
	seen := map[string]bool{}
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for _, child := range value {
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		case string:
			if finrobotAuditIsPackageRelativePath(value) {
				seen[value] = true
			}
		}
	}
	walk(value)
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func finrobotAuditIsPackageRelativePath(value string) bool {
	if strings.HasPrefix(value, "../") || strings.HasPrefix(value, "/") || strings.Contains(value, "://") {
		return false
	}
	if strings.HasPrefix(value, "fixtures/") ||
		strings.HasPrefix(value, "schemas/") ||
		strings.HasPrefix(value, "contracts/") {
		return true
	}
	return strings.HasSuffix(value, ".leia") ||
		strings.HasSuffix(value, ".schema.json") ||
		strings.HasSuffix(value, ".manifest.json")
}

func assertFinRobotAuditExistingRelativePath(t *testing.T, root, contextPath, packageDir, rel string) {
	t.Helper()
	if strings.Contains(rel, "*") {
		return
	}
	rel = strings.SplitN(rel, "#", 2)[0]
	if strings.HasPrefix(rel, "examples/") {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s references missing repo artifact %q: %v", contextPath, rel, err)
		}
		return
	}
	if _, err := os.Stat(filepath.Join(packageDir, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("%s references missing package artifact %q: %v", contextPath, rel, err)
	}
}

func assertFinRobotAuditCapabilities(t *testing.T, manifestPath, packageName string, plannedCapabilities []string, manifest map[string]any) {
	t.Helper()
	prefixes := finrobotAuditCapabilityPrefixes(plannedCapabilities)
	for _, capability := range plannedCapabilities {
		assertFinRobotAuditCapabilityPrefix(t, manifestPath, capability, prefixes)
	}
	for _, capability := range finrobotAuditCollectCapabilities(manifest) {
		assertFinRobotAuditNamespacedCapability(t, manifestPath, capability)
	}
}

func assertFinRobotAuditFixtureIndexCapabilities(t *testing.T, root, manifestPath string, plannedCapabilities []string, manifest map[string]any) {
	t.Helper()
	fixtureIndex, ok := finrobotAuditFixtureIndexPath(manifest)
	if !ok {
		return
	}
	packageDir := filepath.Dir(manifestPath)
	indexPath := filepath.Join(packageDir, filepath.FromSlash(strings.SplitN(fixtureIndex, "#", 2)[0]))
	index := readJSONMap(t, indexPath)
	manifestCapabilities := finrobotAuditStringSet(finrobotAuditCollectCapabilities(manifest))
	plannedCapabilitySet := finrobotAuditStringSet(plannedCapabilities)
	assertFinRobotFixtureIndexOfflineFlags(t, filepath.ToSlash(indexPath), index)
	assertFinRobotFixtureIndexEntries(t, packageDir, indexPath, index)
	if len(finrobotAuditFixtureIndexEntries(index)) == 0 {
		t.Fatalf("%s has no fixture index entries", indexPath)
	}
	for _, capability := range finrobotAuditCollectCapabilityLists(index) {
		assertFinRobotAuditNamespacedCapability(t, indexPath, capability)
		if !manifestCapabilities[capability] {
			t.Fatalf("%s fixture index capability %q is not declared by %s", indexPath, capability, manifestPath)
		}
		if !plannedCapabilitySet[capability] {
			relIndex := filepath.ToSlash(mustRel(t, root, indexPath))
			t.Fatalf("%s fixture index capability %q is not declared by live_package_plan_manifest.json", relIndex, capability)
		}
	}
}

func finrobotAuditFixtureIndexPath(manifest map[string]any) (string, bool) {
	if fixtures, ok := manifest["fixtures"].(map[string]any); ok {
		if path, ok := fixtures["index"].(string); ok && path != "" {
			return path, true
		}
	}
	if artifacts, ok := manifest["artifacts"].(map[string]any); ok {
		if path, ok := artifacts["fixture_index"].(string); ok && path != "" {
			return path, true
		}
	}
	return "", false
}

func finrobotAuditFixtureIndexEntries(index map[string]any) []map[string]any {
	fixtures, ok := index["fixtures"]
	if !ok {
		return nil
	}
	var entries []map[string]any
	switch fixtures := fixtures.(type) {
	case []any:
		for _, raw := range fixtures {
			if entry, ok := raw.(map[string]any); ok {
				entries = append(entries, entry)
			}
		}
	case map[string]any:
		for _, raw := range fixtures {
			if entry, ok := raw.(map[string]any); ok {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

func finrobotAuditStringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func finrobotAuditCollectCapabilityLists(value any) []string {
	seen := map[string]bool{}
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "capabilities" || key == "common_capabilities" {
					if items, ok := child.([]any); ok {
						for _, item := range items {
							if capability, ok := item.(string); ok && strings.Contains(capability, ".") {
								seen[capability] = true
							}
						}
						continue
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(value)
	capabilities := make([]string, 0, len(seen))
	for capability := range seen {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	return capabilities
}

func finrobotAuditCapabilityPrefixes(capabilities []string) []string {
	prefixSet := map[string]bool{}
	for _, capability := range capabilities {
		prefixSet[finrobotAuditCapabilityPrefix(capability)] = true
	}
	prefixes := make([]string, 0, len(prefixSet))
	for prefix := range prefixSet {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	return prefixes
}

func finrobotAuditCapabilityPrefix(capability string) string {
	parts := strings.Split(capability, ".")
	if len(parts) < 3 {
		return capability
	}
	return strings.Join(parts[:2], ".") + "."
}

func assertFinRobotAuditCapabilityPrefix(t *testing.T, manifestPath, capability string, prefixes []string) {
	t.Helper()
	assertFinRobotAuditNamespacedCapability(t, manifestPath, capability)
	for _, prefix := range prefixes {
		if capability == prefix || strings.HasPrefix(capability, prefix) {
			return
		}
	}
	t.Fatalf("%s capability %q does not match planned prefixes %v", manifestPath, capability, prefixes)
}

func assertFinRobotAuditNamespacedCapability(t *testing.T, manifestPath, capability string) {
	t.Helper()
	if strings.Count(capability, ".") < 1 {
		t.Fatalf("%s capability %q is not namespaced", manifestPath, capability)
	}
}

func finrobotAuditCollectCapabilities(value any) []string {
	seen := map[string]bool{}
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "capability" {
					if capability, ok := child.(string); ok {
						seen[capability] = true
					}
					continue
				}
				if key == "capabilities" || key == "common_capabilities" {
					if items, ok := child.([]any); ok {
						for _, item := range items {
							if capability, ok := item.(string); ok && strings.Contains(capability, ".") {
								seen[capability] = true
							}
						}
						continue
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(value)
	capabilities := make([]string, 0, len(seen))
	for capability := range seen {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	return capabilities
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return value
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(rel, "..") {
		t.Fatalf("%s is outside %s", target, base)
	}
	return filepath.ToSlash(rel)
}

func TestFinRobotPackageManifestAuditSchemaIsValidJSON(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"examples/ai/finrobot_translation/package_manifest_audit/schema.json",
		"examples/ai/finrobot_translation/package_manifest_audit/ledger.json",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("%s is not valid JSON: %v", rel, err)
		}
	}
	if got := fmt.Sprintf("%T", loadFinRobotPackageManifestAuditLedger(t, root)); !strings.Contains(got, "finrobotPackageManifestAuditLedger") {
		t.Fatalf("ledger did not decode into audit type: %s", got)
	}
}
