package leia_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type earningsLivePackagePlanManifest struct {
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

func TestFinRobotLivePackagePlanIncludesEarningsTranscript(t *testing.T) {
	root := repoRoot(t)
	manifest := loadEarningsLivePackagePlanManifest(t, root)
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
	if !manifest.NoBuiltInGuarantee.Required {
		t.Fatal("top-level no_built_in_guarantee must be required")
	}
	statement := strings.ToLower(manifest.NoBuiltInGuarantee.Statement)
	for _, want := range []string{"does not provide built-in", "external packages", "capability gates"} {
		if !strings.Contains(statement, want) {
			t.Fatalf("no built-in guarantee statement missing %q: %q", want, statement)
		}
	}

	if len(manifest.Packages) != 1 {
		t.Fatalf("packages = %d, want 1", len(manifest.Packages))
	}
	pkg := manifest.Packages[0]
	if pkg.ID != "earnings_transcript" || pkg.PackageName != "leia-finrobot-earnings-transcript" {
		t.Fatalf("package = %#v", pkg)
	}
	if pkg.Status != "skeleton_contract_checked_in" || !pkg.NoBuiltInGuarantee {
		t.Fatalf("package status/no-built-in = %#v", pkg)
	}
	for label, value := range map[string]string{
		"skeleton_directory": pkg.SkeletonDirectory,
		"planned_directory":  pkg.PlannedDirectory,
		"manifest":           pkg.Manifest,
		"contract":           pkg.Contract,
		"migration_source":   pkg.MigrationSource,
	} {
		if value == "" {
			t.Fatalf("%s missing", label)
		}
	}
	if !strings.HasPrefix(pkg.PlannedDirectory, "packages/finrobot/") {
		t.Fatalf("planned_directory = %q", pkg.PlannedDirectory)
	}
	if _, err := os.Stat(filepath.Join(root, pkg.SkeletonDirectory)); err != nil {
		t.Fatalf("skeleton_directory %q: %v", pkg.SkeletonDirectory, err)
	}
	if _, err := os.Stat(filepath.Join(root, pkg.MigrationSource)); err != nil {
		t.Fatalf("migration source %q: %v", pkg.MigrationSource, err)
	}
	for _, want := range []string{
		"finance.earnings_transcript.speaker.clean",
		"finance.earnings_transcript.date.correct",
		"finance.earnings_transcript.period.lookup",
		"finance.earnings_transcript.segment.provenance",
		"finance.earnings_transcript.chunk",
		"finance.earnings_transcript.http_clean.skip",
	} {
		if !contains(pkg.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, pkg.Capabilities)
		}
	}
	joinedGates := strings.ToLower(strings.Join(pkg.TestGates, " "))
	for _, want := range []string{"fixture", "contract", "clean skip"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("package gates missing %q evidence: %s", want, joinedGates)
		}
	}

	assertEarningsLivePackageSkeletonSummaryMatchesPackages(t, root, manifest)
}

func assertEarningsLivePackageSkeletonSummaryMatchesPackages(t *testing.T, root string, manifest earningsLivePackagePlanManifest) {
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

func loadEarningsLivePackagePlanManifest(t *testing.T, root string) earningsLivePackagePlanManifest {
	t.Helper()
	var manifest earningsLivePackagePlanManifest
	decodeEarningsTranscriptJSONFile(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_package_plan_manifest.json"), &manifest)
	return manifest
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
