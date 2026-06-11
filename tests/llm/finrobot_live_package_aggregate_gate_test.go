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
	plannedPackageDirs = append(plannedPackageDirs, "examples/ai/finrobot_translation/live_packages/news_catalyst")
	sort.Strings(plannedPackageDirs)
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
