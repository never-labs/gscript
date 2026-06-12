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

			for _, key := range []string{"capability_id", "dialect_capability_id"} {
				if raw, ok := manifest[key]; ok {
					var got string
					if err := json.Unmarshal(raw, &got); err != nil {
						t.Fatalf("%s: %v", key, err)
					}
					if got != row.Capability {
						t.Fatalf("%s = %q, want matrix capability %q", key, got, row.Capability)
					}
				}
			}
			if raw, ok := manifest["dialect_backend_shape"]; ok {
				var got string
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("dialect_backend_shape: %v", err)
				}
				if got != row.BackendShape {
					t.Fatalf("dialect_backend_shape = %q, want matrix backend_shape %q", got, row.BackendShape)
				}
			}
			for _, oldKey := range []string{"backend_shape"} {
				if _, ok := manifest[oldKey]; ok {
					t.Fatalf("old dialect symbol field %q must use a dialect_* name", oldKey)
				}
			}
		})
	}
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
