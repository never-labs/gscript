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

type genericAIPackageMatrix struct {
	SchemaVersion int                   `json:"schema_version"`
	MatrixID      string                `json:"matrix_id"`
	SourceRoot    string                `json:"source_root"`
	Packages      []genericAIPackageRow `json:"packages"`
}

type genericAIPackageRow struct {
	ID           string   `json:"id"`
	PackageName  string   `json:"package_name"`
	PackageDir   string   `json:"package_dir"`
	Capability   string   `json:"capability"`
	Capabilities []string `json:"capabilities"`
	BackendShape string   `json:"backend_shape"`
	ProviderFree bool     `json:"provider_free"`
	MainLeia     string   `json:"main_leia"`
	Manifest     string   `json:"manifest"`
	Contracts    []string `json:"contracts"`
	FixtureIndex string   `json:"fixture_index"`
}

func TestGenericAIPackageMatrixMapsTenGenericLivePackages(t *testing.T) {
	root := repoRoot(t)
	matrix := loadGenericAIPackageMatrix(t, root)
	if matrix.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", matrix.SchemaVersion)
	}
	if matrix.MatrixID != "generic-ai-live-package-matrix" {
		t.Fatalf("matrix_id = %q", matrix.MatrixID)
	}
	if matrix.SourceRoot != "examples/ai/finrobot_translation/live_packages" {
		t.Fatalf("source_root = %q", matrix.SourceRoot)
	}
	packagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	wantDirs := map[string]bool{}
	for _, dir := range genericLivePackageDirs(t, packagesRoot) {
		wantDirs[filepath.ToSlash(mustRel(t, root, dir))] = true
	}
	if len(wantDirs) == 0 {
		t.Fatal("generic live package dirs = 0")
	}
	if len(matrix.Packages) != len(wantDirs) {
		t.Fatalf("packages = %d, want %d generic live package dirs", len(matrix.Packages), len(wantDirs))
	}

	seen := map[string]bool{}
	for _, row := range matrix.Packages {
		row := row
		t.Run(row.ID, func(t *testing.T) {
			assertGenericAIPackageMatrixRow(t, root, row, wantDirs)
			if seen[row.ID] {
				t.Fatalf("duplicate package id %q", row.ID)
			}
			seen[row.ID] = true
		})
	}
	for dir := range wantDirs {
		if !seen[filepath.Base(filepath.FromSlash(dir))] {
			t.Fatalf("generic package dir %s missing from matrix", dir)
		}
	}
}

func TestGenericAIPackageMatrixIsProviderFreeAndCapabilityGeneric(t *testing.T) {
	root := repoRoot(t)
	matrix := loadGenericAIPackageMatrix(t, root)
	for _, row := range matrix.Packages {
		row := row
		t.Run(row.ID, func(t *testing.T) {
			if !row.ProviderFree {
				t.Fatalf("%s provider_free = false", row.ID)
			}
			assertGenericAIPackageMatrixCapability(t, row.ID, row.Capability)
			for _, capability := range row.Capabilities {
				assertGenericAIPackageMatrixCapability(t, row.ID, capability)
			}

			manifest := readJSONMap(t, filepath.Join(root, filepath.FromSlash(row.Manifest)))
			if !finrobotLivePackageBoolOrConst(manifest["provider_free"], true) {
				t.Fatalf("%s manifest provider_free = %#v, want true", row.ID, manifest["provider_free"])
			}
			if value, ok := manifest["live_network"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
				t.Fatalf("%s manifest live_network = %#v, want false", row.ID, value)
			}
			if value, ok := manifest["live_network_default"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
				t.Fatalf("%s manifest live_network_default = %#v, want false", row.ID, value)
			}

			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Manifest)))
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			manifestCapabilities, err := genericAIPackageMatrixManifestCapabilities(raw["capabilities"])
			if err != nil {
				t.Fatalf("%s capabilities: %v", row.Manifest, err)
			}
			sort.Strings(manifestCapabilities)
			matrixCapabilities := append([]string(nil), row.Capabilities...)
			sort.Strings(matrixCapabilities)
			if strings.Join(matrixCapabilities, "\n") != strings.Join(manifestCapabilities, "\n") {
				t.Fatalf("%s matrix capabilities do not match manifest\nmatrix=%#v\nmanifest=%#v", row.ID, matrixCapabilities, manifestCapabilities)
			}

			fixtureIndex := readJSONMap(t, filepath.Join(root, filepath.FromSlash(row.FixtureIndex)))
			if !finrobotLivePackageBoolOrConst(fixtureIndex["provider_free"], true) {
				t.Fatalf("%s fixture index provider_free = %#v, want true", row.ID, fixtureIndex["provider_free"])
			}
			if value, ok := fixtureIndex["live_network"]; ok && !finrobotLivePackageBoolOrConst(value, false) {
				t.Fatalf("%s fixture index live_network = %#v, want false", row.ID, value)
			}
		})
	}
}

func genericAIPackageMatrixManifestCapabilities(raw json.RawMessage) ([]string, error) {
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	capabilities := make([]string, 0, len(values))
	for _, value := range values {
		switch value := value.(type) {
		case string:
			capabilities = append(capabilities, value)
		case map[string]any:
			capability, _ := value["id"].(string)
			if capability == "" {
				capability, _ = value["capability_id"].(string)
			}
			if capability == "" {
				return nil, fmt.Errorf("capability object missing id or capability_id")
			}
			capabilities = append(capabilities, capability)
		default:
			return nil, fmt.Errorf("unsupported capability value %T", value)
		}
	}
	return capabilities, nil
}

func loadGenericAIPackageMatrix(t *testing.T, root string) genericAIPackageMatrix {
	t.Helper()
	var matrix genericAIPackageMatrix
	readJSONFile(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "generic_ai_package_matrix.json"), &matrix)
	return matrix
}

func assertGenericAIPackageMatrixRow(t *testing.T, root string, row genericAIPackageRow, wantDirs map[string]bool) {
	t.Helper()
	if row.ID == "" || row.PackageName == "" || row.PackageDir == "" || row.Capability == "" || row.BackendShape == "" {
		t.Fatalf("row has missing identity fields: %#v", row)
	}
	if !strings.HasPrefix(row.ID, "generic_") {
		t.Fatalf("id = %q, want generic_*", row.ID)
	}
	if !strings.HasPrefix(row.PackageName, "leia-generic-ai-") {
		t.Fatalf("%s package_name = %q, want leia-generic-ai-*", row.ID, row.PackageName)
	}
	if !wantDirs[row.PackageDir] {
		t.Fatalf("%s package_dir = %q, not a generic live package dir", row.ID, row.PackageDir)
	}
	if row.MainLeia != filepath.ToSlash(filepath.Join(row.PackageDir, "main.leia")) {
		t.Fatalf("%s main_leia = %q, want package main.leia", row.ID, row.MainLeia)
	}
	if row.Manifest != filepath.ToSlash(filepath.Join(row.PackageDir, "package.manifest.json")) {
		t.Fatalf("%s manifest = %q, want package.manifest.json", row.ID, row.Manifest)
	}
	if row.FixtureIndex != filepath.ToSlash(filepath.Join(row.PackageDir, "fixtures", "provider_free_fixture_index.json")) {
		t.Fatalf("%s fixture_index = %q, want provider_free fixture index", row.ID, row.FixtureIndex)
	}
	if len(row.Contracts) == 0 {
		t.Fatalf("%s contracts must not be empty", row.ID)
	}
	assertGenericAIPackageMatrixPath(t, root, row.PackageDir, true)
	assertGenericAIPackageMatrixPath(t, root, row.MainLeia, false)
	assertGenericAIPackageMatrixPath(t, root, row.Manifest, false)
	assertGenericAIPackageMatrixPath(t, root, row.FixtureIndex, false)
	for _, contract := range row.Contracts {
		if !strings.HasPrefix(contract, row.PackageDir+"/contracts/") {
			t.Fatalf("%s contract %q must be under package contracts/", row.ID, contract)
		}
		assertGenericAIPackageMatrixPath(t, root, contract, false)
	}
	if !genericLivePackageContains(row.Capabilities, row.Capability) {
		t.Fatalf("%s primary capability %q is not in capabilities %#v", row.ID, row.Capability, row.Capabilities)
	}
}

func assertGenericAIPackageMatrixPath(t *testing.T, root, rel string, wantDir bool) {
	t.Helper()
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") || strings.Contains(rel, "://") {
		t.Fatalf("invalid matrix path %q", rel)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("matrix path %q: %v", rel, err)
	}
	if info.IsDir() != wantDir {
		t.Fatalf("matrix path %q IsDir=%v, want %v", rel, info.IsDir(), wantDir)
	}
}

func assertGenericAIPackageMatrixCapability(t *testing.T, packageID, capability string) {
	t.Helper()
	if capability == "" || strings.Count(capability, ".") < 1 {
		t.Fatalf("%s capability %q is not namespaced", packageID, capability)
	}
	lower := strings.ToLower(capability)
	for _, prefix := range []string{"finrobot.", "finrobot_", "fr."} {
		if strings.HasPrefix(lower, prefix) {
			t.Fatalf("%s capability %q uses FinRobot-specific prefix %q", packageID, capability, prefix)
		}
	}
}
