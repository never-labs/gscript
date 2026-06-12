package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericAIManifestNamingAndDialectFieldConvergence(t *testing.T) {
	root := repoRoot(t)
	matrix := loadGenericAIPackageMatrix(t, root)
	index := loadGenericAIDialectIndex(t, root)
	backendPlan := loadGenericAIDialectBackendPlan(t, root)
	if len(matrix.Packages) == 0 {
		t.Fatal("generic AI package matrix is empty")
	}
	for _, row := range matrix.Packages {
		row := row
		t.Run(row.ID, func(t *testing.T) {
			base := filepath.Join(root, filepath.FromSlash(row.PackageDir))
			var manifest map[string]json.RawMessage
			readJSONFile(t, filepath.Join(root, filepath.FromSlash(row.Manifest)), &manifest)

			var packageName string
			if err := json.Unmarshal(manifest["package_name"], &packageName); err != nil {
				t.Fatalf("package_name: %v", err)
			}
			if packageName != row.PackageName {
				t.Fatalf("package_name = %q, want %q", packageName, row.PackageName)
			}

			capabilities := genericManifestConvergenceCapabilities(t, manifest["capabilities"])
			if len(capabilities) == 0 {
				t.Fatal("capabilities must not be empty")
			}
			if !genericManifestConvergenceContains(capabilities, row.Capability) {
				t.Fatalf("capabilities %#v do not include matrix capability %q", capabilities, row.Capability)
			}

			var canonicalCapabilityID string
			if err := json.Unmarshal(manifest["canonical_capability_id"], &canonicalCapabilityID); err != nil {
				t.Fatalf("canonical_capability_id: %v", err)
			}
			if canonicalCapabilityID != row.Capability {
				t.Fatalf("canonical_capability_id = %q, want matrix capability %q", canonicalCapabilityID, row.Capability)
			}

			var entrypoints map[string]string
			if err := json.Unmarshal(manifest["entrypoints"], &entrypoints); err != nil {
				t.Fatalf("entrypoints must map names to file paths only: %v", err)
			}
			for name, rel := range entrypoints {
				if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "://") {
					t.Fatalf("entrypoint %q is not a relative file path: %q", name, rel)
				}
				if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel))); err != nil {
					t.Fatalf("entrypoint %q path %q: %v", name, rel, err)
				}
			}

			var guarantee struct {
				Required  bool   `json:"required"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(manifest["no_built_in_guarantee"], &guarantee); err != nil {
				t.Fatalf("no_built_in_guarantee: %v", err)
			}
			if !guarantee.Required || !strings.Contains(guarantee.Statement, packageName) {
				t.Fatalf("no_built_in_guarantee must be required and name %q: %#v", packageName, guarantee)
			}

			var dialectCapabilityID string
			if err := json.Unmarshal(manifest["dialect_capability_id"], &dialectCapabilityID); err != nil {
				t.Fatalf("dialect_capability_id: %v", err)
			}
			if dialectCapabilityID != row.Capability {
				t.Fatalf("dialect_capability_id = %q, want matrix capability %q", dialectCapabilityID, row.Capability)
			}
			if raw, ok := manifest["capability_id"]; ok {
				var got string
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("capability_id: %v", err)
				}
				if got != row.Capability {
					t.Fatalf("capability_id = %q, want matrix capability %q", got, row.Capability)
				}
			}
			var dialectBackendShape string
			if err := json.Unmarshal(manifest["dialect_backend_shape"], &dialectBackendShape); err != nil {
				t.Fatalf("dialect_backend_shape: %v", err)
			}
			if dialectBackendShape != row.BackendShape {
				t.Fatalf("dialect_backend_shape = %q, want matrix backend_shape %q", dialectBackendShape, row.BackendShape)
			}
			for _, oldKey := range []string{"backend_shape"} {
				if _, ok := manifest[oldKey]; ok {
					t.Fatalf("old dialect symbol field %q must use a dialect_* name", oldKey)
				}
			}

			var aliases []genericAIDialectAlias
			if err := json.Unmarshal(manifest["capability_aliases"], &aliases); err != nil {
				t.Fatalf("capability_aliases: %v", err)
			}
			expectedAliases := genericManifestConvergenceExpectedAliases(row.PackageDir, index, backendPlan)
			if len(expectedAliases) == 0 {
				t.Fatalf("no expected aliases from index/backend plan for %s", row.PackageDir)
			}
			for _, expected := range expectedAliases {
				if !genericManifestConvergenceCapabilityAliasContains(aliases, expected) {
					t.Fatalf("capability_aliases missing %#v in %#v", expected, aliases)
				}
			}
			for _, alias := range aliases {
				if alias.Alias == "" || alias.TargetCapabilityID == "" || alias.AliasKind == "" || alias.Scope == "" || alias.Status != "active" {
					t.Fatalf("incomplete capability alias: %#v", alias)
				}
				if alias.TargetCapabilityID != row.Capability {
					t.Fatalf("capability alias target = %q, want matrix capability %q", alias.TargetCapabilityID, row.Capability)
				}
			}
			var rawAliases []map[string]json.RawMessage
			if err := json.Unmarshal(manifest["capability_aliases"], &rawAliases); err != nil {
				t.Fatalf("capability_aliases raw: %v", err)
			}
			for _, alias := range rawAliases {
				for _, forbidden := range []string{"capability", "capabilities"} {
					if _, ok := alias[forbidden]; ok {
						t.Fatalf("capability_aliases must not use recursive audit key %q: %#v", forbidden, alias)
					}
				}
			}

			var dialectAliases []struct {
				ID     string `json:"id"`
				Target string `json:"target"`
				Scope  string `json:"scope"`
				Source string `json:"source"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(manifest["dialect_aliases"], &dialectAliases); err != nil {
				t.Fatalf("dialect_aliases: %v", err)
			}
			if !genericManifestConvergenceAliasContains(dialectAliases, row.BackendShape, row.Capability) {
				t.Fatalf("dialect_aliases must map backend shape %q to capability %q: %#v", row.BackendShape, row.Capability, dialectAliases)
			}
			for _, alias := range dialectAliases {
				if alias.ID == "" || alias.Target == "" || alias.Scope != "package" || alias.Source == "" || alias.Status != "active" {
					t.Fatalf("incomplete dialect alias: %#v", alias)
				}
				if alias.Target != row.Capability {
					t.Fatalf("dialect alias target = %q, want matrix capability %q", alias.Target, row.Capability)
				}
			}
		})
	}
}

func genericManifestConvergenceExpectedAliases(packageDir string, index genericAIDialectIndex, backendPlan genericAIDialectBackendPlan) []genericAIDialectAlias {
	var aliases []genericAIDialectAlias
	for _, entry := range index.Entries {
		if entry.ProductionPackageBoundary != nil && entry.ProductionPackageBoundary.Directory == packageDir {
			aliases = append(aliases, entry.CapabilityAliases...)
		}
	}
	for _, shape := range backendPlan.BackendShapes {
		if shape.PackageBoundary != nil && shape.PackageBoundary.Directory == packageDir {
			aliases = append(aliases, shape.CapabilityAliases...)
		}
	}
	return aliases
}

func genericManifestConvergenceCapabilityAliasContains(aliases []genericAIDialectAlias, want genericAIDialectAlias) bool {
	for _, alias := range aliases {
		if alias.Alias == want.Alias &&
			alias.TargetCapabilityID == want.TargetCapabilityID &&
			alias.AliasKind == want.AliasKind &&
			alias.Scope == want.Scope &&
			alias.Status == want.Status {
			return true
		}
	}
	return false
}

func genericManifestConvergenceAliasContains(aliases []struct {
	ID     string `json:"id"`
	Target string `json:"target"`
	Scope  string `json:"scope"`
	Source string `json:"source"`
	Status string `json:"status"`
}, id, target string) bool {
	for _, alias := range aliases {
		if alias.ID == id && alias.Target == target && alias.Status == "active" {
			return true
		}
	}
	return false
}

func genericManifestConvergenceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func genericManifestConvergenceCapabilities(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("capabilities must be an explicit array: %v", err)
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		switch value := value.(type) {
		case string:
			out = append(out, value)
		case map[string]any:
			for _, key := range []string{"capability_id", "id", "capability"} {
				if capability, ok := value[key].(string); ok && capability != "" {
					out = append(out, capability)
					break
				}
			}
		default:
			t.Fatalf("capability entry has unsupported shape: %#v", value)
		}
	}
	return out
}
