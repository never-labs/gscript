package leia_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestGenericLivePackageHashAndFixtureRegistryAudit(t *testing.T) {
	root := repoRoot(t)
	packagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	genericDirs := genericLivePackageDirs(t, packagesRoot)
	if len(genericDirs) == 0 {
		t.Fatal("no generic live packages found")
	}

	rootRefs := genericPackageHashAuditRootReferences(t, root)
	for _, packageDir := range genericDirs {
		packageDir := packageDir
		relDir := filepath.ToSlash(mustRel(t, root, packageDir))
		t.Run(filepath.Base(packageDir), func(t *testing.T) {
			manifestRel := filepath.ToSlash(filepath.Join(relDir, "package.manifest.json"))
			mainRel := filepath.ToSlash(filepath.Join(relDir, "main.leia"))
			if !rootRefs[manifestRel] {
				t.Fatalf("%s is not referenced by live_package_plan_manifest or ai_dialect_index/backend_plan", manifestRel)
			}
			if !rootRefs[mainRel] {
				t.Fatalf("%s is not referenced by live_package_plan_manifest or ai_dialect_index/backend_plan", mainRel)
			}

			manifestPath := filepath.Join(packageDir, "package.manifest.json")
			manifestHash := genericPackageAuditFileHash(t, manifestPath)
			if manifestHash == "" {
				t.Fatalf("%s has empty content hash", manifestRel)
			}

			manifest := readJSONMap(t, manifestPath)
			manifestRefs := genericPackageHashAuditManifestReferences(t, relDir, manifest)
			rootContractRefs := 0
			for _, contractRel := range genericPackageHashAuditContractFiles(t, root, packageDir) {
				if rootRefs[contractRel] {
					rootContractRefs++
				}
				if !rootRefs[contractRel] && !manifestRefs[contractRel] {
					t.Fatalf("%s contract is not referenced by live_package_plan_manifest, ai_dialect_index/backend_plan, or %s", contractRel, manifestRel)
				}
				if hash := genericPackageAuditFileHash(t, filepath.Join(root, filepath.FromSlash(contractRel))); hash == "" {
					t.Fatalf("%s has empty content hash", contractRel)
				}
			}
			if rootContractRefs == 0 {
				t.Fatalf("%s has no contract referenced by live_package_plan_manifest or ai_dialect_index/backend_plan", relDir)
			}

			fixtureIndexRel := filepath.ToSlash(filepath.Join(relDir, "fixtures", "provider_free_fixture_index.json"))
			if !manifestRefs[fixtureIndexRel] {
				t.Fatalf("%s fixture index is not referenced by %s", fixtureIndexRel, manifestRel)
			}
			if hash := genericPackageAuditFileHash(t, filepath.Join(root, filepath.FromSlash(fixtureIndexRel))); hash == "" {
				t.Fatalf("%s has empty content hash", fixtureIndexRel)
			}
			genericPackageHashAuditFixtureIndex(t, root, packageDir, fixtureIndexRel)
		})
	}
}

func genericPackageHashAuditRootReferences(t *testing.T, root string) map[string]bool {
	t.Helper()
	refs := map[string]bool{}
	for _, rel := range []string{
		"examples/ai/finrobot_translation/live_package_plan_manifest.json",
		"examples/ai/finrobot_translation/ai_dialect_index/index.json",
		"examples/ai/finrobot_translation/ai_dialect_index/backend_plan.json",
	} {
		var value any
		readJSONFile(t, filepath.Join(root, filepath.FromSlash(rel)), &value)
		genericPackageHashAuditCollectRepoRefs(value, refs)
	}
	return refs
}

func genericPackageHashAuditManifestReferences(t *testing.T, relDir string, manifest map[string]any) map[string]bool {
	t.Helper()
	refs := map[string]bool{}
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
			if strings.HasPrefix(value, "contracts/") ||
				strings.HasPrefix(value, "fixtures/") ||
				strings.HasPrefix(value, "schemas/") ||
				value == "main.leia" {
				refs[filepath.ToSlash(filepath.Join(relDir, filepath.FromSlash(value)))] = true
			}
		}
	}
	walk(manifest)
	return refs
}

func genericPackageHashAuditContractFiles(t *testing.T, root, packageDir string) []string {
	t.Helper()
	contractsRoot := filepath.Join(packageDir, "contracts")
	entries, err := os.ReadDir(contractsRoot)
	if err != nil {
		t.Fatalf("%s: %v", contractsRoot, err)
	}
	var refs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		refs = append(refs, filepath.ToSlash(mustRel(t, root, filepath.Join(contractsRoot, entry.Name()))))
	}
	sort.Strings(refs)
	if len(refs) == 0 {
		t.Fatalf("%s has no contract json files", contractsRoot)
	}
	return refs
}

func genericPackageHashAuditFixtureIndex(t *testing.T, root, packageDir, fixtureIndexRel string) {
	t.Helper()
	var index struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Fixtures     []struct {
			Key          string `json:"key"`
			FixtureKey   string `json:"fixture_key"`
			Capability   string `json:"capability"`
			Path         string `json:"path"`
			Schema       string `json:"schema"`
			ProviderFree bool   `json:"provider_free"`
			Metadata     struct {
				ReplayReady bool `json:"replay_ready"`
			} `json:"metadata"`
		} `json:"fixtures"`
	}
	readJSONFile(t, filepath.Join(root, filepath.FromSlash(fixtureIndexRel)), &index)
	if !index.ProviderFree || index.LiveNetwork {
		t.Fatalf("%s must be provider-free and offline: %#v", fixtureIndexRel, index)
	}
	if len(index.Fixtures) == 0 {
		t.Fatalf("%s has no fixtures", fixtureIndexRel)
	}
	seen := map[string]bool{}
	for _, fixture := range index.Fixtures {
		id := fixture.Key
		if id == "" {
			id = fixture.FixtureKey
		}
		if id == "" {
			id = fixture.Path
		}
		if id == "" || fixture.Path == "" || seen[id] {
			t.Fatalf("%s has invalid or duplicate fixture entry: %#v", fixtureIndexRel, fixture)
		}
		seen[id] = true
		fixtureRel := filepath.ToSlash(filepath.Join(filepath.ToSlash(mustRel(t, root, packageDir)), filepath.FromSlash(fixture.Path)))
		if hash := genericPackageAuditFileHash(t, filepath.Join(root, filepath.FromSlash(fixtureRel))); hash == "" {
			t.Fatalf("%s has empty content hash", fixtureRel)
		}
	}
}

func genericPackageAuditFileHash(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("%s points to a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func genericPackageHashAuditCollectRepoRefs(value any, refs map[string]bool) {
	switch value := value.(type) {
	case map[string]any:
		for _, child := range value {
			genericPackageHashAuditCollectRepoRefs(child, refs)
		}
	case []any:
		for _, child := range value {
			genericPackageHashAuditCollectRepoRefs(child, refs)
		}
	case string:
		if strings.HasPrefix(value, "examples/ai/finrobot_translation/live_packages/generic_") {
			refs[filepath.ToSlash(value)] = true
		}
	}
}
