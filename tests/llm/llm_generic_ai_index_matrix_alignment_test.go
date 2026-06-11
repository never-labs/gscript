package leia_test

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestGenericAIDialectIndexProductionBoundariesAlignWithPackageMatrix(t *testing.T) {
	root := repoRoot(t)
	index := loadGenericAIDialectIndex(t, root)
	matrix := loadGenericAIPackageMatrix(t, root)

	matrixByDir := map[string]genericAIPackageRow{}
	matrixByPackage := map[string]genericAIPackageRow{}
	for _, row := range matrix.Packages {
		if row.PackageDir == "" || row.PackageName == "" {
			t.Fatalf("matrix row missing package identity: %#v", row)
		}
		matrixByDir[row.PackageDir] = row
		matrixByPackage[row.PackageName] = row
	}

	boundaryByDir := map[string]genericAIDialectIndexItem{}
	for _, entry := range index.Entries {
		if entry.ProductionPackageBoundary == nil {
			t.Fatalf("%s missing checked-in production package boundary", entry.CapabilityID)
		}
		boundary := *entry.ProductionPackageBoundary
		if boundary.Status != "checked_in" || !boundary.ProviderFree || boundary.DomainSpecific {
			t.Fatalf("%s boundary must stay checked-in/provider-free/generic: %#v", entry.CapabilityID, boundary)
		}
		if _, ok := matrixByDir[boundary.Directory]; !ok {
			t.Fatalf("%s boundary directory %q is not represented in generic_ai_package_matrix", entry.CapabilityID, boundary.Directory)
		}
		boundaryByDir[boundary.Directory] = entry
	}

	for _, row := range matrix.Packages {
		entry, ok := boundaryByDir[row.PackageDir]
		if !ok {
			t.Fatalf("%s matrix package has no ai_dialect_index production boundary", row.ID)
		}
		boundary := *entry.ProductionPackageBoundary
		if row.MainLeia != boundary.RegisteredExample {
			t.Fatalf("%s matrix main_leia = %q, index registered_example = %q", row.ID, row.MainLeia, boundary.RegisteredExample)
		}
		if row.Manifest != filepath.ToSlash(filepath.Join(row.PackageDir, "package.manifest.json")) {
			t.Fatalf("%s manifest path = %q", row.ID, row.Manifest)
		}
		manifest := readJSONMap(t, filepath.Join(root, filepath.FromSlash(row.Manifest)))
		packageName, _ := manifest["package_name"].(string)
		if packageName != row.PackageName {
			t.Fatalf("%s package_name drift: matrix %q manifest %q", row.ID, row.PackageName, packageName)
		}
		if !genericAIMatrixCapabilitiesCoverIndexSurface(row, entry) {
			t.Fatalf("%s capabilities do not cover index capability/surface: capability_id=%q surface=%#v capabilities=%#v",
				row.ID, entry.CapabilityID, entry.DialectSurface, row.Capabilities)
		}
	}

	if len(boundaryByDir) != len(matrix.Packages) {
		t.Fatalf("index/matrix package count drift: index dirs=%d matrix packages=%d index=%v matrix=%v",
			len(boundaryByDir), len(matrix.Packages), sortedGenericAIAlignmentKeys(boundaryByDir), sortedGenericAIAlignmentMatrixKeys(matrixByDir))
	}
	if len(matrixByPackage) != len(matrix.Packages) {
		t.Fatalf("duplicate package names in generic_ai_package_matrix")
	}
}

func genericAIMatrixCapabilitiesCoverIndexSurface(row genericAIPackageRow, entry genericAIDialectIndexItem) bool {
	haystack := strings.ToLower(row.Capability + "\n" + row.BackendShape + "\n" + strings.Join(row.Capabilities, "\n"))
	if capabilityTail := genericAIAlignmentTail(entry.CapabilityID); capabilityTail != "" && strings.Contains(haystack, capabilityTail) {
		return true
	}
	for _, surface := range entry.DialectSurface {
		for _, token := range strings.FieldsFunc(strings.ToLower(surface), func(r rune) bool {
			return r == '.' || r == '_' || r == '-' || r == ' '
		}) {
			if len(token) >= 4 && strings.Contains(haystack, token) {
				return true
			}
		}
	}
	return false
}

func genericAIAlignmentTail(value string) string {
	parts := strings.Split(strings.ToLower(value), ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func sortedGenericAIAlignmentKeys(values map[string]genericAIDialectIndexItem) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedGenericAIAlignmentMatrixKeys(values map[string]genericAIPackageRow) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
