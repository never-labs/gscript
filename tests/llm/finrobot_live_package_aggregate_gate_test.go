package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestFinRobotLivePackageAggregateGate(t *testing.T) {
	root := repoRoot(t)
	plan := loadLivePackagePlanManifest(t, root)
	livePackagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	llmTestFiles := finrobotLivePackageLLMTestFiles(t, root)

	actualPackageDirs := finrobotLivePackageDirs(t, root, livePackagesRoot)
	plannedPackageDirs := finrobotLivePackagePlanSkeletonDirs(t, root, plan)
	if !reflect.DeepEqual(actualPackageDirs, plannedPackageDirs) {
		t.Fatalf("live package skeleton directories mismatch\ngot  %#v\nwant %#v", actualPackageDirs, plannedPackageDirs)
	}

	for _, relDir := range actualPackageDirs {
		t.Run(filepath.Base(relDir), func(t *testing.T) {
			pkgDir := filepath.Join(root, filepath.FromSlash(relDir))
			manifestCount, checkedJSONCount := 0, 0
			err := filepath.WalkDir(pkgDir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") || !finrobotLivePackageAggregateJSON(path) {
					return nil
				}
				checkedJSONCount++
				if strings.Contains(d.Name(), "manifest") {
					manifestCount++
				}
				decoded := assertFinRobotLivePackageAggregateJSON(t, path)
				if filepath.Base(path) == "package.manifest.json" {
					manifest, ok := decoded.(map[string]any)
					if !ok {
						t.Fatalf("%s root = %#v, want object", path, decoded)
					}
					assertFinRobotPackageManifestEntrypoints(t, pkgDir, path, manifest)
					assertFinRobotPackageSmokeDiscovery(t, root, relDir, pkgDir, path, manifest, llmTestFiles)
				}
				if finrobotLivePackageSchemaJSON(path) {
					schema, ok := decoded.(map[string]any)
					if !ok {
						t.Fatalf("%s root = %#v, want schema object", path, decoded)
					}
					assertFinRobotLivePackageSchemaRequiredOrExempt(t, path, schema)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if manifestCount == 0 {
				t.Fatalf("%s has no package manifest JSON", relDir)
			}
			if checkedJSONCount == 0 {
				t.Fatalf("%s has no manifest/schema/fixture JSON files", relDir)
			}
			indexPaths := finrobotPackageFixtureIndexPaths(t, root, relDir)
			if len(indexPaths) == 0 {
				t.Fatalf("%s has no provider-free fixture index", relDir)
			}
		})
	}
}

func TestFinRobotNonGenericProviderFreeFixtureIndexesStayOffline(t *testing.T) {
	root := repoRoot(t)
	livePackagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")

	for _, relDir := range finrobotNonGenericLivePackageDirs(t, root, livePackagesRoot) {
		t.Run(filepath.Base(relDir), func(t *testing.T) {
			indexPaths := finrobotPackageFixtureIndexPaths(t, root, relDir)
			if len(indexPaths) == 0 {
				t.Skip("package does not use provider-free fixture indexes")
			}
			for _, indexPath := range indexPaths {
				index := readJSONMap(t, indexPath)
				if !finrobotLivePackageBoolOrConst(index["provider_free"], true) {
					t.Fatalf("%s provider_free = %#v, want true", indexPath, index["provider_free"])
				}
				assertFinRobotFixtureIndexOfflineFlags(t, filepath.ToSlash(indexPath), index)
			}
		})
	}
}

func TestFinRobotLivePackageFixtureIndexesReferenceExistingJSON(t *testing.T) {
	root := repoRoot(t)
	livePackagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")

	for _, relDir := range finrobotLivePackageDirs(t, root, livePackagesRoot) {
		t.Run(filepath.Base(relDir), func(t *testing.T) {
			indexPaths := finrobotPackageFixtureIndexPaths(t, root, relDir)
			if len(indexPaths) == 0 {
				t.Fatalf("%s has no provider-free fixture index", relDir)
			}
			pkgDir := filepath.Join(root, filepath.FromSlash(relDir))
			for _, indexPath := range indexPaths {
				index := readJSONMap(t, indexPath)
				assertFinRobotFixtureIndexOfflineFlags(t, filepath.ToSlash(indexPath), index)
				assertFinRobotFixtureIndexEntries(t, pkgDir, indexPath, index)
			}
		})
	}
}

func finrobotPackageFixtureIndexPaths(t *testing.T, root, relDir string) []string {
	t.Helper()
	var paths []string
	for _, rel := range []string{
		"fixtures/provider_free_fixture_index.json",
		"fixtures/offline_replay_index.json",
	} {
		path := filepath.Join(root, filepath.FromSlash(relDir), filepath.FromSlash(rel))
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	return paths
}

func finrobotLivePackageDirs(t *testing.T, root, livePackagesRoot string) []string {
	t.Helper()
	entries, err := os.ReadDir(livePackagesRoot)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel, err := filepath.Rel(root, filepath.Join(livePackagesRoot, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, filepath.ToSlash(rel))
	}
	sort.Strings(dirs)
	return dirs
}

func finrobotNonGenericLivePackageDirs(t *testing.T, root, livePackagesRoot string) []string {
	t.Helper()
	dirs := finrobotLivePackageDirs(t, root, livePackagesRoot)
	filtered := dirs[:0]
	for _, dir := range dirs {
		if strings.HasPrefix(filepath.Base(dir), "generic_") {
			continue
		}
		filtered = append(filtered, dir)
	}
	return filtered
}

func finrobotLivePackagePlanSkeletonDirs(t *testing.T, root string, plan livePackagePlanManifest) []string {
	t.Helper()
	const wantPrefix = "examples/ai/finrobot_translation/live_packages/"
	seen := map[string]bool{}
	for _, pkg := range plan.Packages {
		if !strings.HasPrefix(pkg.SkeletonDirectory, wantPrefix) {
			t.Fatalf("%s skeleton_directory = %q, want under %s", pkg.ID, pkg.SkeletonDirectory, wantPrefix)
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(pkg.SkeletonDirectory)))
		if err != nil {
			t.Fatalf("%s skeleton_directory %q: %v", pkg.ID, pkg.SkeletonDirectory, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s skeleton_directory %q is not a directory", pkg.ID, pkg.SkeletonDirectory)
		}
		seen[filepath.ToSlash(pkg.SkeletonDirectory)] = true
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func finrobotLivePackageAggregateJSON(path string) bool {
	name := filepath.Base(path)
	slashPath := filepath.ToSlash(path)
	return strings.Contains(name, "manifest") ||
		strings.Contains(name, "schema") ||
		strings.Contains(slashPath, "/schemas/") ||
		strings.Contains(slashPath, "/fixtures/")
}

func assertFinRobotLivePackageAggregateJSON(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	assertFinRobotProviderFreeGate(t, filepath.ToSlash(path), decoded)
	return decoded
}

func finrobotLivePackageSchemaJSON(path string) bool {
	slashPath := filepath.ToSlash(path)
	return filepath.Base(path) == "package.schema.json" ||
		strings.Contains(slashPath, "/schemas/") ||
		strings.HasSuffix(filepath.Base(path), ".schema.json")
}

func assertFinRobotPackageManifestEntrypoints(t *testing.T, pkgDir, manifestPath string, manifest map[string]any) {
	t.Helper()
	rawEntrypoints, ok := manifest["entrypoints"]
	if !ok {
		t.Fatalf("%s missing entrypoints", manifestPath)
	}
	entrypoints, ok := rawEntrypoints.(map[string]any)
	if !ok || len(entrypoints) == 0 {
		t.Fatalf("%s entrypoints = %#v, want non-empty object", manifestPath, rawEntrypoints)
	}
	for key, raw := range entrypoints {
		ref, ok := raw.(string)
		if !ok || strings.TrimSpace(ref) == "" {
			t.Fatalf("%s entrypoints.%s = %#v, want non-empty file path", manifestPath, key, raw)
		}
		assertFinRobotManifestFileReference(t, pkgDir, manifestPath, "entrypoints."+key, ref)
	}
	for key, raw := range manifest {
		if key == "entrypoints" || key != "fixture_index" && !strings.HasSuffix(key, "_fixture_index") {
			continue
		}
		ref, ok := raw.(string)
		if !ok || strings.TrimSpace(ref) == "" {
			t.Fatalf("%s %s = %#v, want non-empty fixture index path", manifestPath, key, raw)
		}
		assertFinRobotManifestFileReference(t, pkgDir, manifestPath, key, ref)
	}
}

func assertFinRobotPackageSmokeDiscovery(t *testing.T, root, relDir, pkgDir, manifestPath string, manifest map[string]any, llmTestFiles []finrobotLLMTestFile) {
	t.Helper()
	entrypoints := manifest["entrypoints"].(map[string]any)
	smokeRef, ok := finrobotLivePackageSmokeEntrypoint(entrypoints)
	if !ok {
		t.Fatalf("%s entrypoints must include smoke or main .leia entrypoint", manifestPath)
	}
	assertFinRobotManifestFileReference(t, pkgDir, manifestPath, "entrypoints.smoke_or_main", smokeRef)

	testFiles := finrobotLivePackageSpecificTestFiles(root, relDir, manifest, llmTestFiles)
	if len(testFiles) == 0 {
		t.Fatalf("%s has no package-specific tests/llm test file", manifestPath)
	}
	for _, testFile := range testFiles {
		if strings.Contains(testFile.Text, "ExecFile") {
			return
		}
		if finrobotTestFileSmokeDiscoveryExempt(testFile.Text) {
			return
		}
	}
	if finrobotLivePackageSmokeDiscoveryExempt(manifest) {
		return
	}
	var paths []string
	for _, testFile := range testFiles {
		paths = append(paths, testFile.RelPath)
	}
	t.Fatalf("%s package-specific test files %v must contain ExecFile or manifest metadata must declare contract_only/metadata_only smoke discovery", manifestPath, paths)
}

func finrobotLivePackageSmokeEntrypoint(entrypoints map[string]any) (string, bool) {
	for _, key := range []string{"smoke", "main"} {
		ref, ok := entrypoints[key].(string)
		if !ok || strings.TrimSpace(ref) == "" {
			continue
		}
		if strings.HasSuffix(strings.TrimSpace(strings.Split(ref, "#")[0]), ".leia") {
			return ref, true
		}
	}
	return "", false
}

func finrobotLivePackageSmokeDiscoveryExempt(value any) bool {
	manifest, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if finrobotSmokeDiscoveryExemptMap(manifest) {
		return true
	}
	if smokeDiscovery, ok := manifest["smoke_discovery"].(map[string]any); ok {
		return finrobotSmokeDiscoveryExemptMap(smokeDiscovery)
	}
	return false
}

func finrobotSmokeDiscoveryExemptMap(value map[string]any) bool {
	return finrobotLivePackageBoolOrConst(value["contract_only"], true) ||
		finrobotLivePackageBoolOrConst(value["metadata_only"], true) ||
		value["mode"] == "metadata-only" ||
		value["policy"] == "metadata-only"
}

func finrobotTestFileSmokeDiscoveryExempt(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "contract_only") ||
		strings.Contains(text, "metadata_only") ||
		strings.Contains(text, "metadata-only")
}

type finrobotLLMTestFile struct {
	RelPath string
	Text    string
}

func finrobotLivePackageLLMTestFiles(t *testing.T, root string) []finrobotLLMTestFile {
	t.Helper()
	testRoot := filepath.Join(root, "tests", "llm")
	entries, err := os.ReadDir(testRoot)
	if err != nil {
		t.Fatal(err)
	}
	var files []finrobotLLMTestFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(testRoot, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, finrobotLLMTestFile{RelPath: filepath.ToSlash(rel), Text: string(data)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files
}

func finrobotLivePackageSpecificTestFiles(root, relDir string, manifest map[string]any, llmTestFiles []finrobotLLMTestFile) []finrobotLLMTestFile {
	packageName := filepath.Base(relDir)
	manifestTokens := []string{
		relDir,
		filepath.ToSlash(filepath.Join(root, filepath.FromSlash(relDir))),
		packageName,
		strings.ReplaceAll(packageName, "_", "-"),
		finrobotStringValue(manifest["id"]),
		finrobotStringValue(manifest["package_name"]),
	}
	var matches []finrobotLLMTestFile
	for _, testFile := range llmTestFiles {
		text := strings.ToLower(filepath.ToSlash(testFile.Text))
		relPath := strings.ToLower(testFile.RelPath)
		for _, token := range manifestTokens {
			token = strings.ToLower(strings.TrimSpace(token))
			if token == "" {
				continue
			}
			if strings.Contains(text, token) || strings.Contains(relPath, token) {
				matches = append(matches, testFile)
				break
			}
		}
	}
	return matches
}

func finrobotStringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func assertFinRobotManifestFileReference(t *testing.T, pkgDir, manifestPath, key, ref string) {
	t.Helper()
	refPath := strings.TrimSpace(strings.Split(ref, "#")[0])
	if refPath == "" || strings.Contains(refPath, "://") || strings.HasPrefix(refPath, "$") {
		t.Fatalf("%s %s = %q, want package-local file path", manifestPath, key, ref)
	}
	path := filepath.Clean(filepath.Join(pkgDir, filepath.FromSlash(refPath)))
	rel, err := filepath.Rel(pkgDir, path)
	if err != nil {
		t.Fatalf("%s %s = %q: %v", manifestPath, key, ref, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("%s %s = %q resolves outside package", manifestPath, key, ref)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s %s = %q: %v", manifestPath, key, ref, err)
	}
	if info.IsDir() {
		t.Fatalf("%s %s = %q resolves to directory", manifestPath, key, ref)
	}
}

func assertFinRobotLivePackageSchemaRequiredOrExempt(t *testing.T, path string, schema map[string]any) {
	t.Helper()
	if raw, ok := schema["required"]; ok {
		required, ok := raw.([]any)
		if !ok || len(required) == 0 {
			t.Fatalf("%s required = %#v, want non-empty array", path, raw)
		}
		for i, field := range required {
			if name, ok := field.(string); !ok || strings.TrimSpace(name) == "" {
				t.Fatalf("%s required[%d] = %#v, want non-empty string", path, i, field)
			}
		}
		return
	}
	if finrobotLivePackageBoolOrConst(schema["metadata_only"], true) {
		return
	}
	t.Fatalf("%s missing top-level required; set required or metadata_only=true for schema metadata registries", path)
}

func assertFinRobotProviderFreeGate(t *testing.T, path string, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "provider_free":
				if !finrobotLivePackageBoolOrConst(child, true) {
					t.Fatalf("%s provider_free = %#v, want true", path, child)
				}
			case "live_network", "live_network_default":
				if !finrobotLivePackageBoolOrConst(child, false) {
					t.Fatalf("%s %s = %#v, want false", path, key, child)
				}
			}
			assertFinRobotProviderFreeGate(t, path, child)
		}
	case []any:
		for _, child := range value {
			assertFinRobotProviderFreeGate(t, path, child)
		}
	}
}

func assertFinRobotFixtureIndexOfflineFlags(t *testing.T, path string, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "provider_free":
				if !finrobotLivePackageBoolOrConst(child, true) {
					t.Fatalf("%s provider_free = %#v, want true", path, child)
				}
			case "live_network", "live_network_default", "allow_network":
				if !finrobotLivePackageBoolOrConst(child, false) {
					t.Fatalf("%s %s = %#v, want false", path, key, child)
				}
			case "real_dependency_imports", "real_dependency_import_default", "provider_credentials_required", "requires_credentials":
				if !finrobotLivePackageBoolOrConst(child, false) {
					t.Fatalf("%s %s = %#v, want false", path, key, child)
				}
			}
			assertFinRobotFixtureIndexOfflineFlags(t, path, child)
		}
	case []any:
		for _, child := range value {
			assertFinRobotFixtureIndexOfflineFlags(t, path, child)
		}
	}
}

func assertFinRobotFixtureIndexReferences(t *testing.T, pkgDir, indexPath string, value any) {
	t.Helper()
	seen := map[string]bool{}
	assertFinRobotFixtureIndexReferencesValue(t, pkgDir, filepath.Dir(indexPath), filepath.ToSlash(indexPath), "", value, seen)
}

func assertFinRobotFixtureIndexEntries(t *testing.T, pkgDir, indexPath string, index map[string]any) {
	t.Helper()
	slashIndexPath := filepath.ToSlash(indexPath)
	seen := map[string]bool{}
	rootFlags := finrobotFixtureIndexRootFlags(t, slashIndexPath, index)
	assertFinRobotFixtureIndexMetadataConsistent(t, slashIndexPath, "metadata", index, rootFlags)

	fixtures, ok := index["fixtures"]
	if !ok {
		return
	}
	switch entries := fixtures.(type) {
	case []any:
		for i, rawEntry := range entries {
			entryPath := fmt.Sprintf("%s fixtures[%d]", slashIndexPath, i)
			assertFinRobotFixtureIndexEntry(t, pkgDir, indexPath, entryPath, rawEntry, rootFlags, seen)
		}
	case map[string]any:
		keys := make([]string, 0, len(entries))
		for key := range entries {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entryPath := fmt.Sprintf("%s fixtures.%s", slashIndexPath, key)
			assertFinRobotFixtureIndexEntry(t, pkgDir, indexPath, entryPath, entries[key], rootFlags, seen)
		}
	default:
		t.Fatalf("%s fixtures = %#v, want array or object", slashIndexPath, fixtures)
	}
}

func assertFinRobotFixtureIndexEntry(t *testing.T, pkgDir, indexPath, entryPath string, rawEntry any, rootFlags map[string]bool, seen map[string]bool) {
	t.Helper()
	entry, ok := rawEntry.(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", entryPath, rawEntry)
	}
	assertFinRobotFixtureIndexReferencesValue(t, pkgDir, filepath.Dir(indexPath), entryPath, "", entry, seen)
	entryFlags := finrobotFixtureIndexEntryFlags(t, entryPath, entry, rootFlags)
	assertFinRobotFixtureIndexMetadataConsistent(t, entryPath, "metadata", entry, entryFlags)
}

func finrobotFixtureIndexRootFlags(t *testing.T, path string, root map[string]any) map[string]bool {
	t.Helper()
	flags := map[string]bool{}
	for _, policy := range finrobotFixtureIndexMetadataPolicies() {
		got, ok := finrobotFixtureIndexBoolValue(root[policy.key])
		if !ok {
			t.Fatalf("%s %s = %#v, want bool or const bool", path, policy.key, root[policy.key])
		}
		if got != policy.want {
			t.Fatalf("%s %s = %#v, want %v", path, policy.key, root[policy.key], policy.want)
		}
		flags[policy.key] = got
	}
	return flags
}

func finrobotFixtureIndexEntryFlags(t *testing.T, path string, entry map[string]any, rootFlags map[string]bool) map[string]bool {
	t.Helper()
	flags := map[string]bool{}
	for _, policy := range finrobotFixtureIndexMetadataPolicies() {
		want := rootFlags[policy.key]
		got := want
		if raw, ok := entry[policy.key]; ok {
			var valid bool
			got, valid = finrobotFixtureIndexBoolValue(raw)
			if !valid {
				t.Fatalf("%s %s = %#v, want bool or const bool", path, policy.key, raw)
			}
			if got != want {
				t.Fatalf("%s %s = %#v contradicts root %s = %v", path, policy.key, raw, policy.key, want)
			}
		}
		flags[policy.key] = got
	}
	return flags
}

func assertFinRobotFixtureIndexMetadataConsistent(t *testing.T, path, metadataKey string, object map[string]any, expected map[string]bool) {
	t.Helper()
	rawMetadata, ok := object[metadataKey]
	if !ok {
		return
	}
	metadata, ok := rawMetadata.(map[string]any)
	if !ok {
		t.Fatalf("%s %s = %#v, want object", path, metadataKey, rawMetadata)
	}
	for _, policy := range finrobotFixtureIndexMetadataPolicies() {
		raw, ok := metadata[policy.key]
		if !ok {
			continue
		}
		got, valid := finrobotFixtureIndexBoolValue(raw)
		if !valid {
			t.Fatalf("%s %s.%s = %#v, want bool or const bool", path, metadataKey, policy.key, raw)
		}
		if got != expected[policy.key] {
			t.Fatalf("%s %s.%s = %#v contradicts entry/root %s = %v", path, metadataKey, policy.key, raw, policy.key, expected[policy.key])
		}
	}
}

type finrobotFixtureIndexMetadataPolicy struct {
	key  string
	want bool
}

func finrobotFixtureIndexMetadataPolicies() []finrobotFixtureIndexMetadataPolicy {
	return []finrobotFixtureIndexMetadataPolicy{
		{key: "provider_free", want: true},
		{key: "live_network", want: false},
		{key: "real_dependency_imports", want: false},
	}
}

func assertFinRobotFixtureIndexReferencesValue(t *testing.T, pkgDir, indexDir, indexPath, key string, value any, seen map[string]bool) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for childKey, child := range value {
			assertFinRobotFixtureIndexReferencesValue(t, pkgDir, indexDir, indexPath, childKey, child, seen)
		}
	case []any:
		for _, child := range value {
			assertFinRobotFixtureIndexReferencesValue(t, pkgDir, indexDir, indexPath, key, child, seen)
		}
	case string:
		if finrobotFixtureIndexPathReferenceKey(key) {
			assertFinRobotFixtureIndexJSONReference(t, pkgDir, indexDir, indexPath, key, value, seen)
		}
		if finrobotFixtureIndexHashReferenceKey(key) && !finrobotFixtureIndexHashReferenceValue(value) {
			t.Fatalf("%s %s hash reference = %q, want non-empty hash reference", indexPath, key, value)
		}
	}
}

func finrobotFixtureIndexPathReferenceKey(key string) bool {
	switch key {
	case "path", "paths", "schema", "schemas", "schema_path", "schema_paths", "record_schema", "records_path", "contract", "contracts", "contract_path", "contract_paths", "fixture_path", "fixture_paths", "index", "fixture_index":
		return true
	default:
		return false
	}
}

func finrobotFixtureIndexHashReferenceKey(key string) bool {
	key = strings.ToLower(key)
	if key == "hash_algorithm" || key == "hash_seed" || key == "deterministic_hash_seed" || key == "hash_input" {
		return false
	}
	return strings.Contains(key, "hash") || strings.Contains(key, "sha256")
}

func finrobotFixtureIndexHashReferenceValue(value string) bool {
	value = strings.TrimSpace(value)
	return value != ""
}

func assertFinRobotFixtureIndexJSONReference(t *testing.T, pkgDir, indexDir, indexPath, key, ref string, seen map[string]bool) {
	t.Helper()
	refPath := strings.TrimSpace(strings.Split(ref, "#")[0])
	if refPath == "" || strings.HasPrefix(refPath, "http://") || strings.HasPrefix(refPath, "https://") {
		return
	}
	if strings.Contains(refPath, "://") || strings.HasPrefix(refPath, "$") {
		return
	}
	if ext := filepath.Ext(refPath); ext != ".json" {
		if strings.Contains(key, "schema") {
			return
		}
		t.Fatalf("%s %s = %q, want JSON reference", indexPath, key, ref)
	}
	path, ok := finrobotResolveFixtureIndexReference(pkgDir, indexDir, refPath)
	if !ok {
		t.Fatalf("%s %s = %q does not resolve under package", indexPath, key, ref)
	}
	if seen[path] {
		return
	}
	seen[path] = true
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s %s = %q: %v", indexPath, key, ref, err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("%s %s = %q references non-decodable JSON %s: %v", indexPath, key, ref, path, err)
	}
}

func finrobotResolveFixtureIndexReference(pkgDir, indexDir, ref string) (string, bool) {
	if filepath.IsAbs(ref) {
		info, err := os.Stat(ref)
		return ref, err == nil && !info.IsDir()
	}
	for _, base := range []string{pkgDir, indexDir} {
		path := filepath.Clean(filepath.Join(base, filepath.FromSlash(ref)))
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func finrobotLivePackageBoolOrConst(value any, want bool) bool {
	got, ok := finrobotFixtureIndexBoolValue(value)
	return ok && got == want
}

func finrobotFixtureIndexBoolValue(value any) (bool, bool) {
	if got, ok := value.(bool); ok {
		return got, true
	}
	object, ok := value.(map[string]any)
	if !ok {
		return false, false
	}
	got, ok := object["const"].(bool)
	return got, ok
}
