package leia_test

import (
	"path/filepath"
	"sort"
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

	boundaryByDir := map[string][]genericAIDialectIndexItem{}
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
		row := matrixByDir[boundary.Directory]
		assertGenericAIIndexAliasesResolveToMatrix(t, row, entry)
		boundaryByDir[boundary.Directory] = append(boundaryByDir[boundary.Directory], entry)
	}

	for _, row := range matrix.Packages {
		entries, ok := boundaryByDir[row.PackageDir]
		if !ok {
			t.Fatalf("%s matrix package has no ai_dialect_index production boundary", row.ID)
		}
		for _, entry := range entries {
			boundary := *entry.ProductionPackageBoundary
			if row.MainLeia != boundary.RegisteredExample {
				t.Fatalf("%s matrix main_leia = %q, index registered_example = %q", row.ID, row.MainLeia, boundary.RegisteredExample)
			}
			if entry.Example != row.MainLeia {
				t.Fatalf("%s index example = %q, want matrix main_leia %q", row.ID, entry.Example, row.MainLeia)
			}
			if entry.Fixture != row.FixtureIndex {
				t.Fatalf("%s index fixture = %q, want matrix fixture_index %q", row.ID, entry.Fixture, row.FixtureIndex)
			}
			assertGenericAIIndexAliasesResolveToMatrix(t, row, entry)
		}
		if row.Manifest != filepath.ToSlash(filepath.Join(row.PackageDir, "package.manifest.json")) {
			t.Fatalf("%s manifest path = %q", row.ID, row.Manifest)
		}
		manifest := readJSONMap(t, filepath.Join(root, filepath.FromSlash(row.Manifest)))
		packageName, _ := manifest["package_name"].(string)
		if packageName != row.PackageName {
			t.Fatalf("%s package_name drift: matrix %q manifest %q", row.ID, row.PackageName, packageName)
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

func TestGenericAIBackendPlanProductionBoundariesAlignWithPackageMatrix(t *testing.T) {
	root := repoRoot(t)
	plan := loadGenericAIDialectBackendPlan(t, root)
	matrix := loadGenericAIPackageMatrix(t, root)

	matrixByDir := map[string]genericAIPackageRow{}
	for _, row := range matrix.Packages {
		matrixByDir[row.PackageDir] = row
	}
	seenDirs := map[string]bool{}
	for _, shape := range plan.BackendShapes {
		shape := shape
		if shape.PackageBoundary == nil || shape.PackageBoundary.Status != "checked_in" {
			t.Fatalf("%s missing checked-in package boundary", shape.ShapeID)
		}
		boundary := *shape.PackageBoundary
		row, ok := matrixByDir[boundary.Directory]
		if !ok {
			t.Fatalf("%s package boundary directory %q is absent from generic package matrix", shape.ShapeID, boundary.Directory)
		}
		seenDirs[boundary.Directory] = true
		if shape.Example != row.MainLeia {
			t.Fatalf("%s backend example = %q, want matrix main_leia %q", shape.ShapeID, shape.Example, row.MainLeia)
		}
		if shape.Fixture != row.FixtureIndex {
			t.Fatalf("%s backend fixture = %q, want matrix fixture_index %q", shape.ShapeID, shape.Fixture, row.FixtureIndex)
		}
		if boundary.RegisteredExample != row.MainLeia {
			t.Fatalf("%s registered example = %q, want matrix main_leia %q", shape.ShapeID, boundary.RegisteredExample, row.MainLeia)
		}
		if row.BackendShape != "" && shape.ShapeID != row.BackendShape {
			t.Fatalf("%s shape_id = %q, want matrix backend_shape %q", row.ID, shape.ShapeID, row.BackendShape)
		}
		assertGenericAIBackendAliasesResolveToMatrix(t, row, shape)
	}
	if len(seenDirs) != len(matrix.Packages) {
		t.Fatalf("backend/matrix package count drift: backend dirs=%d matrix packages=%d backend=%v matrix=%v",
			len(seenDirs), len(matrix.Packages), sortedGenericAIAlignmentBoolKeys(seenDirs), sortedGenericAIAlignmentMatrixKeys(matrixByDir))
	}
}

func assertGenericAIIndexAliasesResolveToMatrix(t *testing.T, row genericAIPackageRow, entry genericAIDialectIndexItem) {
	t.Helper()
	if !genericAIMatrixCapabilityExists(row, entry.CanonicalCapabilityID) {
		t.Fatalf("%s index canonical_capability_id %q is not a matrix capability", row.ID, entry.CanonicalCapabilityID)
	}
	if !genericAIDialectAliasesContain(entry.CapabilityAliases, entry.Capability, entry.CanonicalCapabilityID) {
		t.Fatalf("%s missing scoped alias for index capability slug %q -> %q", row.ID, entry.Capability, entry.CanonicalCapabilityID)
	}
	if !genericAIDialectAliasesContain(entry.CapabilityAliases, entry.CapabilityID, entry.CanonicalCapabilityID) {
		t.Fatalf("%s missing scoped alias for index capability_id %q -> %q", row.ID, entry.CapabilityID, entry.CanonicalCapabilityID)
	}
	assertGenericAIDialectAliasesResolve(t, row, entry.CapabilityAliases)
}

func assertGenericAIBackendAliasesResolveToMatrix(t *testing.T, row genericAIPackageRow, shape genericAIDialectBackendShape) {
	t.Helper()
	if !genericAIMatrixCapabilityExists(row, shape.CanonicalCapabilityID) {
		t.Fatalf("%s backend canonical_capability_id %q is not a matrix capability", row.ID, shape.CanonicalCapabilityID)
	}
	if !genericAIDialectAliasesContain(shape.CapabilityAliases, shape.ShapeID, shape.CanonicalCapabilityID) {
		t.Fatalf("%s missing scoped alias for backend shape %q -> %q", row.ID, shape.ShapeID, shape.CanonicalCapabilityID)
	}
	for _, capability := range shape.Capabilities {
		if !genericAIDialectAliasesContain(shape.CapabilityAliases, capability, shape.CanonicalCapabilityID) {
			t.Fatalf("%s missing scoped alias for backend capability slug %q -> %q", row.ID, capability, shape.CanonicalCapabilityID)
		}
	}
	assertGenericAIDialectAliasesResolve(t, row, shape.CapabilityAliases)
}

func assertGenericAIDialectAliasesResolve(t *testing.T, row genericAIPackageRow, aliases []genericAIDialectAlias) {
	t.Helper()
	if len(aliases) == 0 {
		t.Fatalf("%s missing scoped capability aliases", row.ID)
	}
	for _, alias := range aliases {
		if alias.Alias == "" || alias.TargetCapabilityID == "" || alias.AliasKind == "" || alias.Scope == "" || alias.Status != "active" {
			t.Fatalf("%s incomplete capability alias: %#v", row.ID, alias)
		}
		if !genericAIMatrixCapabilityExists(row, alias.TargetCapabilityID) {
			t.Fatalf("%s alias target %q is not a matrix capability", row.ID, alias.TargetCapabilityID)
		}
	}
}

func genericAIMatrixCapabilityExists(row genericAIPackageRow, capability string) bool {
	if capability == "" {
		return false
	}
	if row.Capability == capability {
		return true
	}
	for _, candidate := range row.Capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}

func genericAIDialectAliasesContain(aliases []genericAIDialectAlias, alias, target string) bool {
	for _, candidate := range aliases {
		if candidate.Alias == alias && candidate.TargetCapabilityID == target && candidate.Status == "active" {
			return true
		}
	}
	return false
}

func sortedGenericAIAlignmentKeys(values map[string][]genericAIDialectIndexItem) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedGenericAIAlignmentBoolKeys(values map[string]bool) []string {
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
