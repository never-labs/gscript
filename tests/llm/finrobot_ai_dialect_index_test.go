package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type genericAIDialectIndex struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Baseline      string `json:"baseline_branch"`
	Scope         struct {
		TranslationRoot                  string `json:"translation_root"`
		IndexDirectory                   string `json:"index_directory"`
		ProviderFree                     bool   `json:"provider_free"`
		DomainSpecific                   bool   `json:"domain_specific"`
		ReadOnly                         bool   `json:"read_only"`
		ImportsLivePackages              bool   `json:"imports_live_packages"`
		DependsOnQRuntime                bool   `json:"depends_on_q_runtime"`
		FinRobotSpecificSyntaxAssumption bool   `json:"finrobot_specific_syntax_assumption"`
	} `json:"scope"`
	Rules                map[string]bool             `json:"rules"`
	RequiredCapabilities []string                    `json:"required_capabilities"`
	Entries              []genericAIDialectIndexItem `json:"entries"`
}

type genericAIDialectIndexItem struct {
	Capability                       string                         `json:"capability"`
	CapabilityID                     string                         `json:"capability_id"`
	ProviderFree                     bool                           `json:"provider_free"`
	DomainSpecific                   bool                           `json:"domain_specific"`
	FinRobotSpecificSyntaxAssumption bool                           `json:"finrobot_specific_syntax_assumption"`
	DialectSurface                   []string                       `json:"dialect_surface"`
	Example                          string                         `json:"example"`
	Test                             string                         `json:"test"`
	Fixture                          string                         `json:"fixture"`
	SmokeCoverage                    *genericAIDialectSmokeCoverage `json:"smoke_coverage"`
	MissingProductionPackageBoundary struct {
		PackageID string `json:"package_id"`
		Boundary  string `json:"boundary"`
		Status    string `json:"status"`
		Reason    string `json:"reason"`
	} `json:"missing_production_package_boundary"`
	ProductionPackageBoundary *struct {
		Status            string `json:"status"`
		PackageID         string `json:"package_id"`
		Directory         string `json:"directory"`
		RegisteredExample string `json:"registered_example"`
		ProviderFree      bool   `json:"provider_free"`
		DomainSpecific    bool   `json:"domain_specific"`
		Reason            string `json:"reason"`
	} `json:"production_package_boundary"`
}

type genericAIDialectBackendPlan struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Scope         struct {
		ProviderFree                     bool `json:"provider_free"`
		DomainSpecific                   bool `json:"domain_specific"`
		ReadOnly                         bool `json:"read_only"`
		DependsOnQRuntime                bool `json:"depends_on_q_runtime"`
		FinRobotSpecificSyntaxAssumption bool `json:"finrobot_specific_syntax_assumption"`
	} `json:"scope"`
	BackendShapes []genericAIDialectBackendShape `json:"backend_shapes"`
}

type genericAIDialectBackendShape struct {
	ShapeID         string                           `json:"shape_id"`
	Status          string                           `json:"status"`
	Capabilities    []string                         `json:"capabilities"`
	Contract        string                           `json:"contract"`
	Inputs          []string                         `json:"inputs"`
	Outputs         []string                         `json:"outputs"`
	Example         string                           `json:"example"`
	Test            string                           `json:"test"`
	Fixture         string                           `json:"fixture"`
	SmokeCoverage   *genericAIDialectSmokeCoverage   `json:"smoke_coverage"`
	PackageBoundary *genericAIDialectPackageBoundary `json:"package_boundary"`
}

type genericAIDialectSmokeCoverage struct {
	Test              string   `json:"test"`
	Command           string   `json:"command"`
	PackageManifest   string   `json:"package_manifest"`
	ManifestGateField string   `json:"manifest_gate_field"`
	GateTerms         []string `json:"gate_terms"`
}

type genericAIDialectPackageBoundary struct {
	Status            string `json:"status"`
	PackageID         string `json:"package_id"`
	Directory         string `json:"directory"`
	RegisteredExample string `json:"registered_example"`
	ContractPath      string `json:"contract_path"`
	ProviderFree      bool   `json:"provider_free"`
	DomainSpecific    bool   `json:"domain_specific"`
}

func TestFinRobotGenericAIDialectPackageIndexAudit(t *testing.T) {
	root := repoRoot(t)
	index := loadGenericAIDialectIndex(t, root)

	if index.SchemaVersion != 1 ||
		index.ID != "generic-ai-dialect-package-index-audit" ||
		index.Baseline != "origin/codex/ai-dialect-polish" {
		t.Fatalf("unexpected index header: %#v", index)
	}
	if index.Scope.TranslationRoot != "examples/ai/finrobot_translation" ||
		index.Scope.IndexDirectory != "examples/ai/finrobot_translation/ai_dialect_index" ||
		!index.Scope.ProviderFree ||
		index.Scope.DomainSpecific ||
		!index.Scope.ReadOnly ||
		index.Scope.ImportsLivePackages ||
		index.Scope.DependsOnQRuntime ||
		index.Scope.FinRobotSpecificSyntaxAssumption {
		t.Fatalf("index scope is not provider-free and generic: %#v", index.Scope)
	}
	for _, rule := range []string{
		"reference_paths_exist",
		"fixtures_are_provider_free",
		"tests_are_index_driven",
		"no_q_runtime_dependency",
		"no_finrobot_specific_syntax_assumption",
		"missing_production_package_boundary_recorded",
	} {
		if !index.Rules[rule] {
			t.Fatalf("index rule %q is not enabled", rule)
		}
	}

	seenCapabilities := map[string]bool{}
	seenIDs := map[string]bool{}
	for _, entry := range index.Entries {
		entry := entry
		t.Run(entry.Capability, func(t *testing.T) {
			if entry.Capability == "" || entry.CapabilityID == "" {
				t.Fatalf("entry is missing capability identity: %#v", entry)
			}
			if seenIDs[entry.CapabilityID] {
				t.Fatalf("duplicate capability_id %q", entry.CapabilityID)
			}
			seenIDs[entry.CapabilityID] = true
			seenCapabilities[entry.Capability] = true
			if !entry.ProviderFree || entry.DomainSpecific || entry.FinRobotSpecificSyntaxAssumption {
				t.Fatalf("%s is not provider-free and generic: %#v", entry.Capability, entry)
			}
			if len(entry.DialectSurface) == 0 {
				t.Fatalf("%s has no dialect surface", entry.Capability)
			}
			assertGenericAIDialectReference(t, root, entry.Example, true)
			assertGenericAIDialectReference(t, root, entry.Test, true)
			assertGenericAIDialectReference(t, root, entry.Fixture, true)
			assertGenericAIDialectFixtureProviderFree(t, root, entry.Fixture)
			assertGenericAIDialectNoLivePackageReference(t, entry)
			assertGenericAIDialectNoFinRobotSyntaxAssumption(t, entry)
			assertGenericAIDialectProductionBoundaryGap(t, entry)
			assertGenericAIDialectProductionBoundary(t, root, entry)
			if entry.ProductionPackageBoundary != nil && entry.ProductionPackageBoundary.Status == "checked_in" {
				assertGenericAIDialectPackageBoundaryTestPointer(t, entry.CapabilityID, entry.ProductionPackageBoundary.Directory, entry.Test)
			}
			if genericAIDialectRequiresSmokeCoverage(entry.CapabilityID) {
				if entry.ProductionPackageBoundary == nil {
					t.Fatalf("%s smoke coverage requires a production package boundary", entry.Capability)
				}
				assertGenericAIDialectSmokeCoverage(t, root, entry.CapabilityID, entry.Test, entry.ProductionPackageBoundary.Directory, entry.SmokeCoverage)
			}
		})
	}

	for _, capability := range index.RequiredCapabilities {
		if !seenCapabilities[capability] {
			t.Fatalf("required capability %q is not represented in entries; got %v", capability, sortedStringKeys(seenCapabilities))
		}
	}
	assertGenericAIDialectDocumentedBoundaryList(t, root, index)
}

func TestFinRobotGenericAIDialectBackendPlan(t *testing.T) {
	root := repoRoot(t)
	index := loadGenericAIDialectIndex(t, root)
	plan := loadGenericAIDialectBackendPlan(t, root)

	if plan.SchemaVersion != 1 || plan.ID != "generic-ai-dialect-backend-plan" {
		t.Fatalf("unexpected backend plan header: %#v", plan)
	}
	if !plan.Scope.ProviderFree ||
		plan.Scope.DomainSpecific ||
		!plan.Scope.ReadOnly ||
		plan.Scope.DependsOnQRuntime ||
		plan.Scope.FinRobotSpecificSyntaxAssumption {
		t.Fatalf("backend plan scope is not provider-free and generic: %#v", plan.Scope)
	}

	requiredCapabilities := map[string]bool{}
	for _, capability := range index.RequiredCapabilities {
		requiredCapabilities[capability] = false
	}
	indexedBoundaries := genericAIDialectIndexedProductionBoundaries(t, index)
	seenShapes := map[string]bool{}
	for _, shape := range plan.BackendShapes {
		shape := shape
		t.Run(shape.ShapeID, func(t *testing.T) {
			if shape.ShapeID == "" || seenShapes[shape.ShapeID] {
				t.Fatalf("invalid or duplicate backend shape id %q", shape.ShapeID)
			}
			seenShapes[shape.ShapeID] = true
			if shape.Status != "checked_in_package_boundary" {
				t.Fatalf("%s has unexpected status %q", shape.ShapeID, shape.Status)
			}
			if shape.Contract == "" || len(shape.Inputs) == 0 || len(shape.Outputs) == 0 || len(shape.Capabilities) == 0 {
				t.Fatalf("%s has incomplete contract surface: %#v", shape.ShapeID, shape)
			}
			for _, capability := range shape.Capabilities {
				if _, ok := requiredCapabilities[capability]; !ok {
					t.Fatalf("%s references unknown capability %q", shape.ShapeID, capability)
				}
				requiredCapabilities[capability] = true
			}
			assertGenericAIDialectReference(t, root, shape.Example, true)
			assertGenericAIDialectReference(t, root, shape.Test, true)
			assertGenericAIDialectReference(t, root, shape.Fixture, true)
			assertGenericAIDialectFixtureProviderFree(t, root, shape.Fixture)
			assertGenericAIDialectBackendShapeGeneric(t, shape)
			assertGenericAIDialectBackendBoundaryIndexed(t, shape, indexedBoundaries)
			assertGenericAIDialectBackendPackageBoundary(t, root, shape)
			if shape.PackageBoundary != nil {
				assertGenericAIDialectPackageBoundaryTestPointer(t, shape.ShapeID, shape.PackageBoundary.Directory, shape.Test)
			}
			if shape.PackageBoundary != nil && genericAIDialectRequiresSmokeCoveragePackage(shape.PackageBoundary.PackageID) {
				assertGenericAIDialectSmokeCoverage(t, root, shape.ShapeID, shape.Test, shape.PackageBoundary.Directory, shape.SmokeCoverage)
			}
		})
	}
	if len(seenShapes) < 8 {
		t.Fatalf("backend plan is too narrow: got %d shapes", len(seenShapes))
	}
	for capability, covered := range requiredCapabilities {
		if !covered {
			t.Fatalf("required capability %q is not covered by backend plan", capability)
		}
	}
}

func genericAIDialectRequiresSmokeCoverage(capabilityID string) bool {
	switch capabilityID {
	case "generic.ai.model.io.envelope", "generic.ai.tool.registry":
		return true
	default:
		return false
	}
}

func genericAIDialectRequiresSmokeCoveragePackage(packageID string) bool {
	switch packageID {
	case "generic-model-io-envelope", "generic-tool-registry":
		return true
	default:
		return false
	}
}

func assertGenericAIDialectSmokeCoverage(t *testing.T, root, owner, indexedTest, boundaryDirectory string, smoke *genericAIDialectSmokeCoverage) {
	t.Helper()
	if smoke == nil {
		t.Fatalf("%s is missing executable smoke coverage", owner)
	}
	if smoke.Test != indexedTest {
		t.Fatalf("%s smoke test %q does not match indexed test %q", owner, smoke.Test, indexedTest)
	}
	assertGenericAIDialectReference(t, root, smoke.Test, true)
	if smoke.Command == "" || !strings.HasPrefix(smoke.Command, "go test ./tests/llm -run ") {
		t.Fatalf("%s smoke command is not an executable llm test gate: %q", owner, smoke.Command)
	}
	testName := strings.TrimSpace(strings.TrimPrefix(smoke.Command, "go test ./tests/llm -run "))
	if testName == "" || strings.Contains(testName, " ") {
		t.Fatalf("%s smoke command has invalid test selector: %q", owner, smoke.Command)
	}
	testData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(smoke.Test)))
	if err != nil {
		t.Fatalf("%s: %v", smoke.Test, err)
	}
	if !strings.Contains(string(testData), "func "+testName+"(") {
		t.Fatalf("%s smoke command selects missing test %q in %s", owner, testName, smoke.Test)
	}
	assertGenericAIDialectReference(t, root, smoke.PackageManifest, false)
	if filepath.ToSlash(filepath.Dir(smoke.PackageManifest)) != boundaryDirectory {
		t.Fatalf("%s smoke package manifest %q is outside boundary %q", owner, smoke.PackageManifest, boundaryDirectory)
	}
	if smoke.ManifestGateField != "test_gates" || len(smoke.GateTerms) == 0 {
		t.Fatalf("%s smoke gate metadata is incomplete: %#v", owner, smoke)
	}
	var manifest struct {
		TestGates []string `json:"test_gates"`
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(smoke.PackageManifest)))
	if err != nil {
		t.Fatalf("%s: %v", smoke.PackageManifest, err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("%s: %v", smoke.PackageManifest, err)
	}
	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, "\n"))
	for _, term := range smoke.GateTerms {
		if !strings.Contains(joinedGates, strings.ToLower(term)) {
			t.Fatalf("%s smoke gates missing %q in %s: %s", owner, term, smoke.PackageManifest, joinedGates)
		}
	}
}

func assertGenericAIDialectPackageBoundaryTestPointer(t *testing.T, owner, boundaryDirectory, testPath string) {
	t.Helper()
	packageName := filepath.Base(filepath.FromSlash(boundaryDirectory))
	expected := "tests/llm/llm_" + packageName + "_live_package_test.go"
	if override := genericAIDialectPackageBoundaryTestPointerOverride(packageName); override != "" {
		expected = override
	}
	if testPath == expected {
		return
	}
	testFile := filepath.Base(filepath.FromSlash(testPath))
	if strings.HasPrefix(testFile, "llm_"+packageName+"_") && strings.HasSuffix(testFile, "_test.go") {
		return
	}
	t.Fatalf("%s package boundary %q test = %q, want package-level test %q", owner, boundaryDirectory, testPath, expected)
}

func genericAIDialectPackageBoundaryTestPointerOverride(packageName string) string {
	switch packageName {
	case "generic_model_io_envelope":
		return "tests/llm/llm_generic_model_io_envelope_test.go"
	case "generic_planning_graph":
		return "tests/llm/llm_generic_planning_graph_contract_test.go"
	default:
		return ""
	}
}

func genericAIDialectIndexedProductionBoundaries(t *testing.T, index genericAIDialectIndex) map[string]genericAIDialectIndexItem {
	t.Helper()
	indexed := map[string]genericAIDialectIndexItem{}
	for _, entry := range index.Entries {
		if entry.ProductionPackageBoundary == nil || entry.ProductionPackageBoundary.Status != "checked_in" {
			continue
		}
		packageID := entry.ProductionPackageBoundary.PackageID
		if packageID == "" {
			t.Fatalf("%s checked-in production boundary has no package id", entry.Capability)
		}
		if previous, ok := indexed[packageID]; ok {
			previousBoundary := previous.ProductionPackageBoundary
			if previousBoundary.Directory != entry.ProductionPackageBoundary.Directory ||
				previousBoundary.RegisteredExample != entry.ProductionPackageBoundary.RegisteredExample ||
				previousBoundary.ProviderFree != entry.ProductionPackageBoundary.ProviderFree ||
				previousBoundary.DomainSpecific != entry.ProductionPackageBoundary.DomainSpecific {
				t.Fatalf("package boundary %q is indexed with conflicting metadata: %#v vs %#v", packageID, *previousBoundary, *entry.ProductionPackageBoundary)
			}
			continue
		}
		indexed[packageID] = entry
	}
	return indexed
}

func assertGenericAIDialectDocumentedBoundaryList(t *testing.T, root string, index genericAIDialectIndex) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "examples", "ai", "finrobot_translation", "ai_dialect_index", "PACKAGE_BOUNDARIES.md"))
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	inPackageList := false
	for _, line := range strings.Split(string(data), "\n") {
		switch strings.TrimSpace(line) {
		case "<!-- ai-dialect-boundary-package-list:start -->":
			inPackageList = true
			continue
		case "<!-- ai-dialect-boundary-package-list:end -->":
			inPackageList = false
			continue
		}
		if !inPackageList || !strings.HasPrefix(line, "- ") {
			continue
		}
		parts := strings.Split(line, "`")
		if len(parts) != 3 {
			t.Fatalf("malformed documented package boundary line %q", line)
		}
		documented[parts[1]] = true
	}

	indexed := map[string]bool{}
	for _, entry := range index.Entries {
		if entry.ProductionPackageBoundary == nil || entry.ProductionPackageBoundary.Status != "checked_in" {
			continue
		}
		indexed[entry.ProductionPackageBoundary.Directory] = true
	}
	for directory := range indexed {
		if !documented[directory] {
			t.Fatalf("checked-in production boundary %q is missing from PACKAGE_BOUNDARIES.md", directory)
		}
	}
	for directory := range documented {
		if !indexed[directory] {
			t.Fatalf("PACKAGE_BOUNDARIES.md documents stale package boundary %q", directory)
		}
	}
}

func assertGenericAIDialectBackendBoundaryIndexed(t *testing.T, shape genericAIDialectBackendShape, indexed map[string]genericAIDialectIndexItem) {
	t.Helper()
	if shape.PackageBoundary == nil {
		t.Fatalf("%s has no package boundary to compare with index", shape.ShapeID)
	}
	boundary := *shape.PackageBoundary
	entry, ok := indexed[boundary.PackageID]
	if !ok {
		t.Fatalf("%s references package boundary %q absent from index production boundaries", shape.ShapeID, boundary.PackageID)
	}
	indexBoundary := entry.ProductionPackageBoundary
	if indexBoundary == nil {
		t.Fatalf("%s matched index entry %q without production boundary", shape.ShapeID, entry.Capability)
	}
	if boundary.Directory != indexBoundary.Directory ||
		boundary.RegisteredExample != indexBoundary.RegisteredExample ||
		boundary.ProviderFree != indexBoundary.ProviderFree ||
		boundary.DomainSpecific != indexBoundary.DomainSpecific {
		t.Fatalf("%s package boundary diverges from index entry %q: %#v vs %#v", shape.ShapeID, entry.Capability, boundary, *indexBoundary)
	}
}

func assertGenericAIDialectBackendPackageBoundary(t *testing.T, root string, shape genericAIDialectBackendShape) {
	t.Helper()
	if shape.PackageBoundary == nil {
		t.Fatalf("%s has no checked-in package boundary", shape.ShapeID)
	}
	boundary := *shape.PackageBoundary
	if boundary.Status != "checked_in" {
		t.Fatalf("%s package boundary is not checked_in: %#v", shape.ShapeID, boundary)
	}
	if boundary.PackageID == "" || boundary.Directory == "" || boundary.RegisteredExample == "" || boundary.ContractPath == "" {
		t.Fatalf("%s checked-in package boundary is incomplete: %#v", shape.ShapeID, boundary)
	}
	if !boundary.ProviderFree || boundary.DomainSpecific {
		t.Fatalf("%s package boundary is not generic/provider-free: %#v", shape.ShapeID, boundary)
	}
	assertGenericAIDialectPackageBoundaryGeneric(t, shape.ShapeID, boundary)

	manifest := filepath.ToSlash(filepath.Join(boundary.Directory, "package.manifest.json"))
	assertGenericAIDialectReference(t, root, manifest, false)
	assertGenericAIDialectReference(t, root, boundary.RegisteredExample, true)
	assertGenericAIDialectReference(t, root, boundary.ContractPath, true)
	assertGenericAIDialectPackageManifestProviderFree(t, root, manifest)
	assertGenericAIDialectRegisteredExampleProviderFree(t, root, boundary.RegisteredExample)
	assertGenericAIDialectContractProviderFree(t, root, boundary.ContractPath)

	dirInfo, err := os.Stat(filepath.Join(root, filepath.FromSlash(boundary.Directory)))
	if err != nil {
		t.Fatalf("%s package boundary directory %q: %v", shape.ShapeID, boundary.Directory, err)
	}
	if !dirInfo.IsDir() {
		t.Fatalf("%s package boundary directory %q is not a directory", shape.ShapeID, boundary.Directory)
	}
	if got, want := filepath.ToSlash(filepath.Dir(boundary.RegisteredExample)), boundary.Directory; got != want {
		t.Fatalf("%s registered example %q is outside package directory %q", shape.ShapeID, boundary.RegisteredExample, boundary.Directory)
	}
	if got, want := filepath.ToSlash(filepath.Dir(filepath.Dir(boundary.ContractPath))), boundary.Directory; got != want {
		t.Fatalf("%s contract path %q is outside package directory %q", shape.ShapeID, boundary.ContractPath, boundary.Directory)
	}
}

func assertGenericAIDialectContractProviderFree(t *testing.T, root, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var contract struct {
		ProviderFree                bool `json:"provider_free"`
		DomainSpecific              bool `json:"domain_specific"`
		LiveNetwork                 bool `json:"live_network"`
		LiveNetworkDefault          bool `json:"live_network_default"`
		RealDependencyImports       bool `json:"real_dependency_imports"`
		RealDependencyImportDefault bool `json:"real_dependency_import_default"`
		LiveModelCalls              bool `json:"live_model_calls"`
		RequiresCredentials         bool `json:"requires_credentials"`
		ProviderCredentialsRequired bool `json:"provider_credentials_required"`
		ProviderSDKsRequired        bool `json:"provider_sdks_required"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	if !contract.ProviderFree ||
		contract.DomainSpecific ||
		contract.LiveNetwork ||
		contract.LiveNetworkDefault ||
		contract.RealDependencyImports ||
		contract.RealDependencyImportDefault ||
		contract.LiveModelCalls ||
		contract.RequiresCredentials ||
		contract.ProviderCredentialsRequired ||
		contract.ProviderSDKsRequired {
		t.Fatalf("contract %q is not provider-free: %#v", rel, contract)
	}
	assertGenericAIDialectProviderFreeText(t, rel, string(data))
}

func assertGenericAIDialectRegisteredExampleProviderFree(t *testing.T, root, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	text := string(data)
	assertGenericAIDialectProviderFreeText(t, rel, text)
	compact := compactGenericAIDialectText(text)
	if !strings.Contains(compact, "provider_free:true") && !strings.Contains(compact, "provider_free=true") {
		t.Fatalf("registered example %q does not declare provider_free true", rel)
	}
	if !strings.Contains(compact, "live_network:false") && !strings.Contains(compact, "live_network=false") {
		t.Fatalf("registered example %q does not declare live_network false", rel)
	}
}

func assertGenericAIDialectProviderFreeText(t *testing.T, rel, text string) {
	t.Helper()
	compact := compactGenericAIDialectText(text)
	for _, forbidden := range []string{
		`"provider_free":false`,
		"provider_free:false",
		"provider_free=false",
		`"live_network":true`,
		"live_network:true",
		"live_network=true",
		`"live_network_default":true`,
		"live_network_default:true",
		"live_network_default=true",
		`"live_model":true`,
		"live_model:true",
		"live_model=true",
		`"live_model_calls":true`,
		"live_model_calls:true",
		"live_model_calls=true",
		`"real_dependency_imports":true`,
		"real_dependency_imports:true",
		"real_dependency_imports=true",
		`"real_dependency_import_default":true`,
		"real_dependency_import_default:true",
		"real_dependency_import_default=true",
		`"requires_credentials":true`,
		"requires_credentials:true",
		"requires_credentials=true",
		`"provider_credentials_required":true`,
		"provider_credentials_required:true",
		"provider_credentials_required=true",
		`"provider_sdks_required":true`,
		"provider_sdks_required:true",
		"provider_sdks_required=true",
	} {
		if strings.Contains(compact, forbidden) {
			t.Fatalf("%q contains provider-specific marker %q", rel, forbidden)
		}
	}
}

func compactGenericAIDialectText(text string) string {
	lower := strings.ToLower(text)
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
	return replacer.Replace(lower)
}

func assertGenericAIDialectProductionBoundary(t *testing.T, root string, entry genericAIDialectIndexItem) {
	t.Helper()
	if entry.ProductionPackageBoundary == nil {
		t.Fatalf("%s has no production package boundary status", entry.Capability)
	}
	boundary := *entry.ProductionPackageBoundary
	switch boundary.Status {
	case "checked_in":
		if boundary.PackageID == "" || boundary.Directory == "" || boundary.RegisteredExample == "" {
			t.Fatalf("%s checked-in production boundary is incomplete: %#v", entry.Capability, boundary)
		}
		if !boundary.ProviderFree || boundary.DomainSpecific {
			t.Fatalf("%s production boundary is not generic/provider-free: %#v", entry.Capability, boundary)
		}
		assertGenericAIDialectReference(t, root, filepath.ToSlash(filepath.Join(boundary.Directory, "package.manifest.json")), false)
		assertGenericAIDialectReference(t, root, boundary.RegisteredExample, true)
	case "pending":
		if boundary.PackageID == "" || boundary.Reason == "" {
			t.Fatalf("%s pending production boundary is incomplete: %#v", entry.Capability, boundary)
		}
	default:
		t.Fatalf("%s has unsupported production boundary status %q", entry.Capability, boundary.Status)
	}
}

func assertGenericAIDialectPackageManifestProviderFree(t *testing.T, root, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var manifest struct {
		ProviderFree                bool `json:"provider_free"`
		DomainSpecific              bool `json:"domain_specific"`
		LiveNetworkDefault          bool `json:"live_network_default"`
		RealDependencyImportDefault bool `json:"real_dependency_import_default"`
		LiveModelCalls              bool `json:"live_model_calls"`
		RealDependencyImports       bool `json:"real_dependency_imports"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	if !manifest.ProviderFree ||
		manifest.DomainSpecific ||
		manifest.LiveNetworkDefault ||
		manifest.RealDependencyImportDefault ||
		manifest.LiveModelCalls ||
		manifest.RealDependencyImports {
		t.Fatalf("package manifest %q is not provider-free: %#v", rel, manifest)
	}
}

func loadGenericAIDialectIndex(t *testing.T, root string) genericAIDialectIndex {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "ai_dialect_index", "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var index genericAIDialectIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return index
}

func loadGenericAIDialectBackendPlan(t *testing.T, root string) genericAIDialectBackendPlan {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "ai_dialect_index", "backend_plan.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan genericAIDialectBackendPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return plan
}

func assertGenericAIDialectReference(t *testing.T, root, rel string, scanForQRuntime bool) {
	t.Helper()
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") {
		t.Fatalf("invalid relative reference %q", rel)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("reference %q: %v", rel, err)
	}
	if info.IsDir() {
		t.Fatalf("reference %q points to a directory", rel)
	}
	if scanForQRuntime {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		text := strings.ToLower(string(data))
		if strings.HasPrefix(filepath.ToSlash(rel), "tests/") {
			for _, forbidden := range []string{
				`"github.com/never-labs/leia/internal/q`,
				`"github.com/never-labs/leia/internal/runtime`,
				`"github.com/never-labs/leia/runtime`,
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("reference %q must not import the q runtime package", rel)
				}
			}
			return
		}
		forbiddenRuntime := "q/" + "runtime"
		if strings.Contains(text, forbiddenRuntime) {
			t.Fatalf("reference %q must not depend on the q runtime package", rel)
		}
	}
}

func assertGenericAIDialectProductionBoundaryGap(t *testing.T, entry genericAIDialectIndexItem) {
	t.Helper()
	gap := entry.MissingProductionPackageBoundary
	if gap.PackageID == "" || gap.Boundary == "" || gap.Reason == "" {
		t.Fatalf("%s production package boundary gap is incomplete: %#v", entry.Capability, gap)
	}
	switch gap.Status {
	case "missing":
		if entry.ProductionPackageBoundary != nil && entry.ProductionPackageBoundary.Status == "checked_in" {
			t.Fatalf("%s has checked-in production boundary but unresolved missing gap: %#v", entry.Capability, gap)
		}
	case "resolved":
		if entry.ProductionPackageBoundary == nil || entry.ProductionPackageBoundary.Status != "checked_in" {
			t.Fatalf("%s resolved gap requires checked-in production boundary: gap=%#v boundary=%#v", entry.Capability, gap, entry.ProductionPackageBoundary)
		}
		lowerReason := strings.ToLower(gap.Reason)
		for _, stale := range []string{
			"currently only",
			"not extracted",
			"not promoted",
			"no standalone",
			"not packaged",
			"lack a package-level",
			"without a production package",
			"not a reusable production",
			"not a generic package boundary",
		} {
			if strings.Contains(lowerReason, stale) {
				t.Fatalf("%s resolved gap reason still reads unresolved (%q): %q", entry.Capability, stale, gap.Reason)
			}
		}
	default:
		t.Fatalf("%s production package boundary gap has invalid status %q", entry.Capability, gap.Status)
	}
}

func assertGenericAIDialectFixtureProviderFree(t *testing.T, root, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	lower := strings.ToLower(string(data))
	if strings.Contains(lower, `"provider_free": false`) ||
		strings.Contains(lower, `"live_network": true`) ||
		strings.Contains(lower, `"live_model": true`) {
		t.Fatalf("fixture %q is not provider-free", rel)
	}
}

func assertGenericAIDialectNoLivePackageReference(t *testing.T, entry genericAIDialectIndexItem) {
	t.Helper()
	for _, rel := range []string{entry.Example, entry.Test, entry.Fixture} {
		if strings.Contains(filepath.ToSlash(rel), "/live_packages/") {
			t.Fatalf("%s references live_packages path %q", entry.Capability, rel)
		}
	}
}

func assertGenericAIDialectNoFinRobotSyntaxAssumption(t *testing.T, entry genericAIDialectIndexItem) {
	t.Helper()
	values := []string{
		entry.Capability,
		entry.CapabilityID,
		strings.Join(entry.DialectSurface, " "),
		entry.MissingProductionPackageBoundary.PackageID,
		entry.MissingProductionPackageBoundary.Boundary,
		entry.MissingProductionPackageBoundary.Reason,
	}
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, forbidden := range []string{"finrobot.", "finrobot_", "autogen", "openbb", "fingpt"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s contains FinRobot-specific syntax assumption %q in %q", entry.Capability, forbidden, value)
			}
		}
	}
}

func assertGenericAIDialectBackendShapeGeneric(t *testing.T, shape genericAIDialectBackendShape) {
	t.Helper()
	values := []string{
		shape.ShapeID,
		shape.Status,
		shape.Contract,
		strings.Join(shape.Capabilities, " "),
		strings.Join(shape.Inputs, " "),
		strings.Join(shape.Outputs, " "),
	}
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, forbidden := range []string{"finrobot.", "finrobot_", "autogen", "openbb", "fingpt"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s contains FinRobot-specific backend assumption %q in %q", shape.ShapeID, forbidden, value)
			}
		}
	}
}

func assertGenericAIDialectPackageBoundaryGeneric(t *testing.T, shapeID string, boundary genericAIDialectPackageBoundary) {
	t.Helper()
	values := []string{
		shapeID,
		boundary.PackageID,
		filepath.Base(filepath.FromSlash(boundary.Directory)),
		filepath.Base(filepath.FromSlash(boundary.RegisteredExample)),
		filepath.Base(filepath.FromSlash(boundary.ContractPath)),
	}
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, forbidden := range []string{"finrobot.", "finrobot_", "autogen", "openbb", "fingpt"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s contains FinRobot-specific package boundary assumption %q in %q", shapeID, forbidden, value)
			}
		}
	}
	if !strings.HasPrefix(filepath.Base(filepath.FromSlash(boundary.Directory)), "generic_") {
		t.Fatalf("%s package boundary directory is not a generic live package: %q", shapeID, boundary.Directory)
	}
}

func sortedStringKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
