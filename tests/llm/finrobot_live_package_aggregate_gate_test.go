package leia_test

import (
	"encoding/json"
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
				assertFinRobotLivePackageAggregateJSON(t, path)
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
				t.Skip("package does not use provider-free fixture indexes")
			}
			pkgDir := filepath.Join(root, filepath.FromSlash(relDir))
			for _, indexPath := range indexPaths {
				index := readJSONMap(t, indexPath)
				assertFinRobotFixtureIndexOfflineFlags(t, filepath.ToSlash(indexPath), index)
				assertFinRobotFixtureIndexReferences(t, pkgDir, indexPath, index)
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

func assertFinRobotLivePackageAggregateJSON(t *testing.T, path string) {
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
	case "path", "paths", "schema", "schemas", "schema_path", "schema_paths", "record_schema", "records_path", "fixture_path", "fixture_paths", "index", "fixture_index":
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
	if got, ok := value.(bool); ok {
		return got == want
	}
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	got, ok := object["const"].(bool)
	return ok && got == want
}
