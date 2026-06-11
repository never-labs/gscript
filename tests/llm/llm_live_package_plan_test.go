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

type livePackagePlanManifest struct {
	SchemaVersion               int    `json:"schema_version"`
	ID                          string `json:"id"`
	BasePackage                 string `json:"base_package"`
	SourceMode                  string `json:"source_mode"`
	TargetMode                  string `json:"target_mode"`
	LiveNetworkDefault          bool   `json:"live_network_default"`
	RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
	NoBuiltInGuarantee          struct {
		Required     bool     `json:"required"`
		Statement    string   `json:"statement"`
		CoreBoundary []string `json:"core_boundary"`
	} `json:"no_built_in_guarantee"`
	LivePackageSkeletons []struct {
		ID                string   `json:"id"`
		Directory         string   `json:"directory"`
		RegisteredExample *string  `json:"registered_example"`
		CoversPackageIDs  []string `json:"covers_package_ids"`
		Status            string   `json:"status"`
	} `json:"live_package_skeletons"`
	Packages []struct {
		ID                 string   `json:"id"`
		PackageName        string   `json:"package_name"`
		Status             string   `json:"status"`
		SkeletonDirectory  string   `json:"skeleton_directory"`
		PlannedDirectory   string   `json:"planned_directory"`
		Manifest           string   `json:"manifest"`
		Contract           string   `json:"contract"`
		MigrationSource    string   `json:"migration_source"`
		Capabilities       []string `json:"capabilities"`
		TestGates          []string `json:"test_gates"`
		NoBuiltInGuarantee bool     `json:"no_built_in_guarantee"`
	} `json:"packages"`
	GlobalTestGates []string `json:"global_test_gates"`
}

func TestFinRobotLivePackagePlanSkeletons(t *testing.T) {
	root := repoRoot(t)
	manifest := loadLivePackagePlanManifest(t, root)
	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-live-external-package-plan" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.BasePackage != "leia-finrobot-translation" ||
		manifest.SourceMode != "replay-slice" ||
		manifest.TargetMode != "live-external-packages" {
		t.Fatalf("manifest migration modes = %#v", manifest)
	}
	if manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("live network/import defaults must be disabled: %#v", manifest)
	}

	wantPackages := map[string]string{
		"vendor_adapters":       "leia-finrobot-vendor-adapters",
		"normalizers":           "leia-finrobot-normalizers",
		"valuation":             "leia-finrobot-valuation",
		"report_renderer":       "leia-finrobot-report-renderer",
		"web_product":           "leia-finrobot-web-product",
		"optional_integrations": "leia-finrobot-optional-integrations",
		"prompt_roles":          "leia-finrobot-prompt-roles",
		"news_catalyst":         "leia-finrobot-news-catalyst",
	}
	if len(manifest.Packages) != len(wantPackages) {
		t.Fatalf("packages = %d, want %d", len(manifest.Packages), len(wantPackages))
	}
	for _, pkg := range manifest.Packages {
		if wantPackages[pkg.ID] != pkg.PackageName {
			t.Fatalf("package %s name = %q", pkg.ID, pkg.PackageName)
		}
		for label, value := range map[string]string{
			"planned_directory": pkg.PlannedDirectory,
			"manifest":          pkg.Manifest,
			"contract":          pkg.Contract,
			"migration_source":  pkg.MigrationSource,
		} {
			if value == "" {
				t.Fatalf("%s missing %s", pkg.ID, label)
			}
		}
		if !strings.HasPrefix(pkg.PlannedDirectory, "packages/finrobot/") {
			t.Fatalf("%s planned_directory = %q", pkg.ID, pkg.PlannedDirectory)
		}
		if pkg.Status != "skeleton_contract_checked_in" {
			t.Fatalf("%s status = %q, want skeleton_contract_checked_in", pkg.ID, pkg.Status)
		}
		if pkg.SkeletonDirectory == "" {
			t.Fatalf("%s missing skeleton_directory", pkg.ID)
		}
		if _, err := os.Stat(filepath.Join(root, pkg.SkeletonDirectory)); err != nil {
			t.Fatalf("%s skeleton_directory %q: %v", pkg.ID, pkg.SkeletonDirectory, err)
		}
		if !strings.HasSuffix(pkg.Manifest, "package.manifest.json") {
			t.Fatalf("%s manifest = %q", pkg.ID, pkg.Manifest)
		}
		if !strings.Contains(pkg.Contract, "/contracts/") ||
			(!strings.HasSuffix(pkg.Contract, "_contract.json") && !strings.HasSuffix(pkg.Contract, "_gates.json")) {
			t.Fatalf("%s contract = %q", pkg.ID, pkg.Contract)
		}
		if _, err := os.Stat(filepath.Join(root, pkg.MigrationSource)); err != nil {
			t.Fatalf("%s migration source %q: %v", pkg.ID, pkg.MigrationSource, err)
		}
	}
	assertLivePackageSkeletonSummaryMatchesPackages(t, root, manifest)
}

func assertLivePackageSkeletonSummaryMatchesPackages(t *testing.T, root string, manifest livePackagePlanManifest) {
	t.Helper()
	skeletonDirs := map[string]bool{}
	for _, skeleton := range manifest.LivePackageSkeletons {
		if skeleton.ID == "" || skeleton.Directory == "" || skeleton.Status == "" || len(skeleton.CoversPackageIDs) == 0 {
			t.Fatalf("incomplete live_package_skeleton entry: %#v", skeleton)
		}
		if _, err := os.Stat(filepath.Join(root, skeleton.Directory)); err != nil {
			t.Fatalf("live package skeleton directory %s: %v", skeleton.Directory, err)
		}
		if skeleton.RegisteredExample != nil {
			if _, err := os.Stat(filepath.Join(root, *skeleton.RegisteredExample)); err != nil {
				t.Fatalf("live package registered example %s: %v", *skeleton.RegisteredExample, err)
			}
		}
		skeletonDirs[filepath.ToSlash(skeleton.Directory)] = true
	}
	packageDirs := map[string]bool{}
	for _, pkg := range manifest.Packages {
		packageDirs[filepath.ToSlash(pkg.SkeletonDirectory)] = true
	}
	if !reflect.DeepEqual(sortedKeys(skeletonDirs), sortedKeys(packageDirs)) {
		t.Fatalf("live_package_skeletons directories = %#v, want package skeleton dirs %#v", sortedKeys(skeletonDirs), sortedKeys(packageDirs))
	}
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestFinRobotLivePackagePlanCapabilitiesAndGates(t *testing.T) {
	manifest := loadLivePackagePlanManifest(t, repoRoot(t))
	wantCapabilityPrefix := map[string]string{
		"vendor_adapters":       "finance.vendor.",
		"normalizers":           "finance.normalize.",
		"valuation":             "finance.valuation.",
		"report_renderer":       "report.render.",
		"web_product":           "web.",
		"optional_integrations": "integration.optional.",
		"prompt_roles":          "prompt.role.",
		"news_catalyst":         "finance.",
	}
	for _, pkg := range manifest.Packages {
		if len(pkg.Capabilities) < 5 {
			t.Fatalf("%s capabilities = %d, want at least 5", pkg.ID, len(pkg.Capabilities))
		}
		prefix := wantCapabilityPrefix[pkg.ID]
		for _, capability := range pkg.Capabilities {
			if !strings.HasPrefix(capability, prefix) {
				t.Fatalf("%s capability %q does not use prefix %q", pkg.ID, capability, prefix)
			}
		}
		if len(pkg.TestGates) < 4 {
			t.Fatalf("%s test_gates = %d, want at least 4", pkg.ID, len(pkg.TestGates))
		}
		joinedGates := strings.ToLower(strings.Join(pkg.TestGates, " "))
		for _, want := range []string{"fixture", "contract"} {
			if !strings.Contains(joinedGates, want) {
				t.Fatalf("%s gates missing %q evidence: %s", pkg.ID, want, joinedGates)
			}
		}
		if strings.Contains(joinedGates, "real network") || strings.Contains(joinedGates, "live_network=true") {
			t.Fatalf("%s gates must not require real network: %s", pkg.ID, joinedGates)
		}
	}
	joinedGlobalGates := strings.ToLower(strings.Join(manifest.GlobalTestGates, " "))
	for _, want := range []string{"no test", "real network", "live_network", "credentials", "capabilities"} {
		if !strings.Contains(joinedGlobalGates, want) {
			t.Fatalf("global gates missing %q: %s", want, joinedGlobalGates)
		}
	}
}

func TestFinRobotLivePackagePlanNoBuiltInGuarantee(t *testing.T) {
	root := repoRoot(t)
	manifest := loadLivePackagePlanManifest(t, root)
	if !manifest.NoBuiltInGuarantee.Required {
		t.Fatal("top-level no_built_in_guarantee must be required")
	}
	statement := strings.ToLower(manifest.NoBuiltInGuarantee.Statement)
	for _, want := range []string{"does not provide built-in", "external packages", "capability gates"} {
		if !strings.Contains(statement, want) {
			t.Fatalf("no built-in guarantee statement missing %q: %q", want, statement)
		}
	}
	for _, pkg := range manifest.Packages {
		if !pkg.NoBuiltInGuarantee {
			t.Fatalf("%s must opt into the no built-in guarantee", pkg.ID)
		}
	}

	docData, err := os.ReadFile(filepath.Join(root, "examples", "ai", "finrobot_translation", "live_package_plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(docData)
	for _, want := range []string{
		"leia-finrobot-vendor-adapters",
		"leia-finrobot-normalizers",
		"leia-finrobot-valuation",
		"leia-finrobot-report-renderer",
		"leia-finrobot-web-product",
		"not Leia built-ins",
		"must not call real providers",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("live_package_plan.md missing %q", want)
		}
	}
}

func loadLivePackagePlanManifest(t *testing.T, root string) livePackagePlanManifest {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_package_plan_manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest livePackagePlanManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode live_package_plan_manifest.json: %v", err)
	}
	return manifest
}
