package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestGenericLivePackageConsistency(t *testing.T) {
	root := repoRoot(t)
	packagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	genericDirs := genericLivePackageDirs(t, packagesRoot)
	if len(genericDirs) == 0 {
		t.Fatal("no generic live packages found")
	}

	registeredExamples := genericLivePackageRegisteredExamples(t, root)
	indexBoundaries := genericLivePackageIndexBoundaries(t, root)
	for _, packageDir := range genericDirs {
		packageDir := packageDir
		relDir := filepath.ToSlash(mustRel(t, root, packageDir))
		t.Run(filepath.Base(packageDir), func(t *testing.T) {
			assertGenericLivePackageLayout(t, packageDir)

			manifestPath := filepath.Join(packageDir, "package.manifest.json")
			manifest := readJSONMap(t, manifestPath)
			assertGenericLivePackageProviderFree(t, manifestPath, manifest)
			assertGenericLivePackageFixtureReplayDefaultPolicy(t, manifestPath, manifest)
			assertGenericLivePackageEntrypoints(t, root, manifestPath, packageDir, manifest)
			assertGenericLivePackageNoBuiltInGuarantee(t, manifestPath, manifest)
			assertGenericLivePackageProviderFreeFixtureIndex(t, root, packageDir)
			assertGenericLivePackageNoQRuntime(t, packageDir)

			registeredExample := registeredExamples[relDir]
			if registeredExample == "" {
				t.Fatalf("%s has no registered example in live_package_plan_manifest.json", relDir)
			}
			if registeredExample != filepath.ToSlash(filepath.Join(relDir, "main.leia")) {
				t.Fatalf("%s registered example = %q, want main.leia", relDir, registeredExample)
			}
			assertGenericLivePackageRepoFile(t, root, relDir, registeredExample)

			indexExamples := indexBoundaries[relDir]
			if len(indexExamples) == 0 {
				t.Fatalf("%s is not discoverable from ai_dialect_index production_package_boundary", relDir)
			}
			if !genericLivePackageContains(indexExamples, registeredExample) {
				t.Fatalf("%s registered example %q not listed by ai_dialect_index: %#v", relDir, registeredExample, indexExamples)
			}
		})
	}
}

func TestGenericLivePackageMatrixMatchesManifestAndDirectories(t *testing.T) {
	root := repoRoot(t)
	packagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	dirIDs := map[string]string{}
	for _, packageDir := range genericLivePackageDirs(t, packagesRoot) {
		relDir := filepath.ToSlash(mustRel(t, root, packageDir))
		dirIDs[filepath.Base(packageDir)] = relDir
	}

	manifestIDs := map[string]string{}
	manifest := loadLivePackagePlanManifest(t, root)
	for _, skeleton := range manifest.LivePackageSkeletons {
		if !strings.HasPrefix(skeleton.ID, "generic_") {
			continue
		}
		if skeleton.Status != "checked_in_registered_example" {
			t.Fatalf("%s status = %q", skeleton.ID, skeleton.Status)
		}
		if skeleton.RegisteredExample == nil || *skeleton.RegisteredExample == "" {
			t.Fatalf("%s missing registered_example", skeleton.ID)
		}
		manifestIDs[skeleton.ID] = filepath.ToSlash(skeleton.Directory)
		if _, ok := dirIDs[skeleton.ID]; !ok {
			t.Fatalf("%s exists in live_package_plan_manifest.json but has no live_packages/generic_* directory", skeleton.ID)
		}
	}

	matrix := loadGenericAIPackageMatrix(t, root)
	if matrix.SchemaVersion != 1 || matrix.MatrixID != "generic-ai-live-package-matrix" {
		t.Fatalf("unexpected generic package matrix header: %#v", matrix)
	}
	if matrix.SourceRoot != "examples/ai/finrobot_translation/live_packages" {
		t.Fatalf("matrix source_root = %q", matrix.SourceRoot)
	}
	matrixIDs := map[string]string{}
	for _, pkg := range matrix.Packages {
		if !strings.HasPrefix(pkg.ID, "generic_") {
			t.Fatalf("matrix package id must stay generic: %#v", pkg)
		}
		if !pkg.ProviderFree {
			t.Fatalf("%s provider_free = false", pkg.ID)
		}
		if pkg.PackageDir == "" || pkg.MainLeia == "" || pkg.Manifest == "" || pkg.FixtureIndex == "" || len(pkg.Contracts) == 0 {
			t.Fatalf("%s has incomplete matrix references: %#v", pkg.ID, pkg)
		}
		if pkg.PackageDir != manifestIDs[pkg.ID] {
			t.Fatalf("%s matrix package_dir = %q, manifest directory = %q", pkg.ID, pkg.PackageDir, manifestIDs[pkg.ID])
		}
		assertGenericLivePackageRepoFile(t, root, pkg.PackageDir, pkg.MainLeia)
		assertGenericLivePackageRepoFile(t, root, pkg.PackageDir, pkg.Manifest)
		assertGenericLivePackageRepoFile(t, root, pkg.PackageDir, pkg.FixtureIndex)
		for _, contract := range pkg.Contracts {
			assertGenericLivePackageRepoFile(t, root, pkg.PackageDir, contract)
		}
		packageManifest := readJSONMap(t, filepath.Join(root, filepath.FromSlash(pkg.Manifest)))
		if got, _ := packageManifest["package_name"].(string); got != pkg.PackageName {
			t.Fatalf("%s package_name mismatch: matrix %q manifest %q", pkg.ID, pkg.PackageName, got)
		}
		if got, _ := packageManifest["provider_free"].(bool); !got {
			t.Fatalf("%s package manifest provider_free = %#v", pkg.ID, packageManifest["provider_free"])
		}
		matrixIDs[pkg.ID] = pkg.PackageDir
	}

	assertSameStringMap(t, "generic live package directories vs live package manifest", dirIDs, manifestIDs)
	assertSameStringMap(t, "generic live package manifest vs matrix", manifestIDs, matrixIDs)
}

func genericLivePackageDirs(t *testing.T, packagesRoot string) []string {
	t.Helper()
	entries, err := os.ReadDir(packagesRoot)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "generic_") {
			dirs = append(dirs, filepath.Join(packagesRoot, entry.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs
}

func assertGenericLivePackageLayout(t *testing.T, packageDir string) {
	t.Helper()
	for _, rel := range []string{"main.leia", "package.manifest.json"} {
		path := filepath.Join(packageDir, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if info.IsDir() {
			t.Fatalf("%s must be a file", path)
		}
	}
	for _, rel := range []string{"contracts", "fixtures", "schemas"} {
		path := filepath.Join(packageDir, rel)
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(entries) == 0 {
			t.Fatalf("%s must not be empty", path)
		}
	}
}

func assertGenericLivePackageProviderFree(t *testing.T, manifestPath string, manifest map[string]any) {
	t.Helper()
	if !finrobotLivePackageBoolOrConst(manifest["provider_free"], true) {
		t.Fatalf("%s provider_free = %#v, want true", manifestPath, manifest["provider_free"])
	}
	if _, ok := manifest["live_network"]; !ok {
		if _, ok := manifest["live_network_default"]; !ok {
			t.Fatalf("%s missing live_network or live_network_default false declaration", manifestPath)
		}
	}
	if value, ok := manifest["live_network"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
		t.Fatalf("%s live_network = %#v, want false", manifestPath, value)
	}
	if value, ok := manifest["live_network_default"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
		t.Fatalf("%s live_network_default = %#v, want false", manifestPath, value)
	}
	assertFinRobotAuditRecursiveBool(t, manifestPath, manifest, "live_network", false)
	assertFinRobotAuditRecursiveBool(t, manifestPath, manifest, "allow_network", false)
	assertFinRobotAuditRecursiveBool(t, manifestPath, manifest, "real_dependency_imports", false)
	assertFinRobotAuditRecursiveBool(t, manifestPath, manifest, "real_dependency_import_default", false)
}

func assertGenericLivePackageFixtureReplayDefaultPolicy(t *testing.T, manifestPath string, manifest map[string]any) {
	t.Helper()
	policy, ok := manifest["default_policy"].(map[string]any)
	if !ok {
		t.Fatalf("%s missing default_policy map", manifestPath)
	}
	if mode, _ := policy["mode"].(string); mode != "fixture_replay" {
		t.Fatalf("%s default_policy.mode = %q, want fixture_replay", manifestPath, mode)
	}
	if value, ok := policy["provider_policy"]; ok {
		if got, _ := value.(string); got != "provider-free" {
			t.Fatalf("%s default_policy.provider_policy = %#v, want provider-free", manifestPath, value)
		}
	}
	if !finrobotLivePackageBoolOrConst(policy["live_network"], false) ||
		!finrobotLivePackageBoolOrConst(policy["provider_credentials_required"], false) ||
		!finrobotLivePackageBoolOrConst(policy["real_dependency_imports"], false) {
		t.Fatalf("%s default_policy must disable live network, credentials, and real imports: %#v", manifestPath, policy)
	}
	if value, ok := policy["live_model_calls"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
		t.Fatalf("%s default_policy.live_model_calls = %#v, want false", manifestPath, value)
	}
	hook, _ := policy["fixture_hook"].(string)
	if !strings.HasPrefix(hook, "recorded_generic_") || !strings.HasSuffix(hook, "_fixture") {
		t.Fatalf("%s default_policy.fixture_hook = %q, want recorded generic fixture hook", manifestPath, hook)
	}
	if !finrobotLivePackageBoolOrConst(policy["clean_skip_without_dependency"], true) &&
		!finrobotLivePackageBoolOrConst(policy["clean_skip_without_descriptor"], true) &&
		!finrobotLivePackageBoolOrConst(policy["clean_skip_without_sink"], true) {
		t.Fatalf("%s default_policy missing clean-skip guard: %#v", manifestPath, policy)
	}
}

func assertGenericLivePackageEntrypoints(t *testing.T, root, manifestPath, packageDir string, manifest map[string]any) {
	t.Helper()
	entrypoints, ok := manifest["entrypoints"].(map[string]any)
	if !ok || len(entrypoints) == 0 {
		t.Fatalf("%s missing entrypoints map", manifestPath)
	}
	for name, value := range entrypoints {
		rel, ok := value.(string)
		if !ok || rel == "" {
			t.Fatalf("%s entrypoint %s = %#v, want relative file path", manifestPath, name, value)
		}
		if filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") || strings.Contains(rel, "://") {
			t.Fatalf("%s entrypoint %s = %q, want package-relative file path", manifestPath, name, rel)
		}
		assertFinRobotAuditExistingRelativePath(t, root, manifestPath, packageDir, rel)
		info, err := os.Stat(filepath.Join(packageDir, filepath.FromSlash(strings.SplitN(rel, "#", 2)[0])))
		if err != nil {
			t.Fatalf("%s entrypoint %s %q: %v", manifestPath, name, rel, err)
		}
		if info.IsDir() {
			t.Fatalf("%s entrypoint %s = %q points to a directory", manifestPath, name, rel)
		}
	}
}

func assertGenericLivePackageNoBuiltInGuarantee(t *testing.T, manifestPath string, manifest map[string]any) {
	t.Helper()
	value, ok := manifest["no_built_in_guarantee"]
	if !ok {
		t.Fatalf("%s missing no_built_in_guarantee", manifestPath)
	}
	switch value := value.(type) {
	case bool:
		if !value {
			t.Fatalf("%s no_built_in_guarantee = false", manifestPath)
		}
	case map[string]any:
		if !finrobotLivePackageBoolOrConst(value["required"], true) {
			t.Fatalf("%s no_built_in_guarantee.required = %#v, want true", manifestPath, value["required"])
		}
		statement, _ := value["statement"].(string)
		lower := strings.ToLower(statement)
		if !strings.Contains(lower, "does not provide") || !strings.Contains(lower, "built-in") {
			t.Fatalf("%s no_built_in_guarantee statement is weak: %q", manifestPath, statement)
		}
	default:
		t.Fatalf("%s no_built_in_guarantee has unsupported shape %#v", manifestPath, value)
	}
}

func assertGenericLivePackageProviderFreeFixtureIndex(t *testing.T, root, packageDir string) {
	t.Helper()
	indexPath := filepath.Join(packageDir, "fixtures", "provider_free_fixture_index.json")
	index := readJSONMap(t, indexPath)
	if !finrobotLivePackageBoolOrConst(index["provider_free"], true) ||
		!finrobotLivePackageBoolOrConst(index["live_network"], false) ||
		!finrobotLivePackageBoolOrConst(index["real_dependency_imports"], false) {
		t.Fatalf("%s must declare provider-free offline defaults: %#v", indexPath, index)
	}
	fixtures, ok := index["fixtures"].([]any)
	if !ok || len(fixtures) == 0 {
		t.Fatalf("%s missing fixtures array", indexPath)
	}
	for i, value := range fixtures {
		fixture, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("%s fixtures[%d] = %#v, want object", indexPath, i, value)
		}
		path, _ := fixture["path"].(string)
		if path == "" || filepath.IsAbs(path) || strings.Contains(filepath.ToSlash(path), "../") || strings.Contains(path, "://") {
			t.Fatalf("%s fixtures[%d].path = %q, want package-relative fixture path", indexPath, i, path)
		}
		if !strings.HasPrefix(filepath.ToSlash(path), "fixtures/") {
			t.Fatalf("%s fixtures[%d].path = %q, want fixtures/... path", indexPath, i, path)
		}
		assertFinRobotAuditExistingRelativePath(t, root, indexPath, packageDir, path)
		info, err := os.Stat(filepath.Join(packageDir, filepath.FromSlash(strings.SplitN(path, "#", 2)[0])))
		if err != nil {
			t.Fatalf("%s fixtures[%d].path %q: %v", indexPath, i, path, err)
		}
		if info.IsDir() {
			t.Fatalf("%s fixtures[%d].path %q points to a directory", indexPath, i, path)
		}
		if filepath.Ext(strings.SplitN(path, "#", 2)[0]) != ".json" {
			t.Fatalf("%s fixtures[%d].path = %q, want JSON fixture", indexPath, i, path)
		}
		if !finrobotLivePackageBoolOrConst(fixture["provider_free"], true) ||
			!finrobotLivePackageBoolOrConst(fixture["live_network"], false) ||
			!finrobotLivePackageBoolOrConst(fixture["real_dependency_imports"], false) {
			t.Fatalf("%s fixtures[%d] must declare provider-free offline flags: %#v", indexPath, i, fixture)
		}
		metadata, ok := fixture["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("%s fixtures[%d] missing metadata map", indexPath, i)
		}
		if !finrobotLivePackageBoolOrConst(metadata["provider_free"], true) ||
			!finrobotLivePackageBoolOrConst(metadata["live_network"], false) ||
			!finrobotLivePackageBoolOrConst(metadata["real_dependency_imports"], false) {
			t.Fatalf("%s fixtures[%d].metadata must declare provider-free offline flags: %#v", indexPath, i, metadata)
		}
	}
}

func assertGenericLivePackageNoQRuntime(t *testing.T, packageDir string) {
	t.Helper()
	for _, rel := range []string{"main.leia", "package.manifest.json", "contracts", "fixtures", "schemas"} {
		path := filepath.Join(packageDir, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if info.IsDir() {
			err = filepath.WalkDir(path, func(child string, d os.DirEntry, err error) error {
				if err != nil || d == nil || d.IsDir() {
					return err
				}
				assertGenericLivePackageFileNoQRuntime(t, child)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if rel == "package.manifest.json" {
			assertGenericLivePackageManifestNoQRuntimeDependency(t, path)
			continue
		}
		assertGenericLivePackageFileNoQRuntime(t, path)
	}
}

func assertGenericLivePackageFileNoQRuntime(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	forbiddenRuntime := "q/" + "runtime"
	if strings.Contains(strings.ToLower(string(data)), forbiddenRuntime) {
		t.Fatalf("%s must not depend on the q runtime package", path)
	}
}

func assertGenericLivePackageManifestNoQRuntimeDependency(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var manifest any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var walk func(any, []string)
	walk = func(value any, keys []string) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				walk(child, append(keys, key))
			}
		case []any:
			for _, child := range value {
				walk(child, keys)
			}
		case string:
			lower := strings.ToLower(value)
			forbiddenRuntime := "q/" + "runtime"
			if !strings.Contains(lower, forbiddenRuntime) {
				return
			}
			keyPath := strings.Join(keys, ".")
			if strings.Contains(keyPath, "blocked_imports") || strings.Contains(lower, "no "+forbiddenRuntime+" dependency") {
				return
			}
			t.Fatalf("%s has q runtime dependency at %s: %q", path, keyPath, value)
		}
	}
	walk(manifest, nil)
}

func genericLivePackageRegisteredExamples(t *testing.T, root string) map[string]string {
	t.Helper()
	manifest := loadLivePackagePlanManifest(t, root)
	registered := map[string]string{}
	for _, skeleton := range manifest.LivePackageSkeletons {
		dir := filepath.ToSlash(skeleton.Directory)
		if !strings.Contains(dir, "/generic_") {
			continue
		}
		if skeleton.RegisteredExample == nil || *skeleton.RegisteredExample == "" {
			t.Fatalf("%s missing registered_example", skeleton.ID)
		}
		if len(skeleton.CoversPackageIDs) == 0 || !genericLivePackageContains(skeleton.CoversPackageIDs, skeleton.ID) {
			t.Fatalf("%s covers_package_ids = %#v, want to include its own id", skeleton.ID, skeleton.CoversPackageIDs)
		}
		assertGenericLivePackageRepoFile(t, root, dir, *skeleton.RegisteredExample)
		registered[dir] = filepath.ToSlash(*skeleton.RegisteredExample)
	}
	return registered
}

func genericLivePackageIndexBoundaries(t *testing.T, root string) map[string][]string {
	t.Helper()
	index := loadGenericAIDialectIndex(t, root)
	boundaries := map[string][]string{}
	for _, entry := range index.Entries {
		if entry.ProductionPackageBoundary == nil || entry.ProductionPackageBoundary.Status != "checked_in" {
			continue
		}
		boundary := *entry.ProductionPackageBoundary
		dir := filepath.ToSlash(boundary.Directory)
		if !strings.Contains(dir, "/generic_") {
			continue
		}
		if !boundary.ProviderFree || boundary.DomainSpecific {
			t.Fatalf("%s index boundary must be generic and provider-free: %#v", entry.Capability, boundary)
		}
		assertGenericLivePackageRepoFile(t, root, dir, boundary.RegisteredExample)
		boundaries[dir] = append(boundaries[dir], filepath.ToSlash(boundary.RegisteredExample))
	}
	for dir := range boundaries {
		sort.Strings(boundaries[dir])
	}
	return boundaries
}

func assertGenericLivePackageRepoFile(t *testing.T, root, ownerDir, rel string) {
	t.Helper()
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") {
		t.Fatalf("%s has invalid repo file reference %q", ownerDir, rel)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s references missing repo file %q: %v", ownerDir, rel, err)
	}
	if info.IsDir() {
		t.Fatalf("%s references directory %q, want file", ownerDir, rel)
	}
}

func genericLivePackageContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertSameStringMap(t *testing.T, label string, want, got map[string]string) {
	t.Helper()
	for key, wantValue := range want {
		if gotValue, ok := got[key]; !ok {
			t.Fatalf("%s missing %q; got keys %v", label, key, sortedStringMapKeys(got))
		} else if gotValue != wantValue {
			t.Fatalf("%s %q = %q, want %q", label, key, gotValue, wantValue)
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Fatalf("%s has unexpected %q; want keys %v", label, key, sortedStringMapKeys(want))
		}
	}
}

func sortedStringMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestGenericLivePackageConsistencyHelpersCompileWithoutRuntimeDeps(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "tests", "llm", "llm_generic_package_consistency_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	forbiddenRuntime := "q/" + "runtime"
	if strings.Contains(strings.ToLower(string(data)), forbiddenRuntime) {
		t.Fatal("generic package consistency test must not import or mention q runtime dependencies")
	}
	var decoded any
	if err := json.Unmarshal([]byte(`{"provider_free":true,"live_network":false}`), &decoded); err != nil {
		t.Fatalf("json helper dependency unavailable: %v", err)
	}
}
