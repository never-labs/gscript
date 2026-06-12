package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type finrobotGapManifest struct {
	SchemaVersion            int    `json:"schema_version"`
	ID                       string `json:"id"`
	PackageHygieneExceptions []struct {
		ID              string   `json:"id"`
		PackageID       string   `json:"package_id"`
		Source          string   `json:"source"`
		ExceptionKind   string   `json:"exception_kind"`
		Status          string   `json:"status"`
		Reason          string   `json:"reason"`
		Guardrails      []string `json:"guardrails"`
		NotClosedReason string   `json:"not_closed_reason"`
	} `json:"package_hygiene_exceptions"`
	OpenProductionGaps []struct {
		ID      string `json:"id"`
		Area    string `json:"area"`
		Status  string `json:"status"`
		Summary string `json:"summary"`
	} `json:"open_production_gaps"`
	ClosedOrNonGapGuards []struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Kind    string `json:"kind"`
		Summary string `json:"summary"`
	} `json:"closed_or_non_gap_guards"`
	DocumentedIn []string `json:"documented_in"`
	Verification []string `json:"verification"`
}

func TestFinRobotStatusDocsMatchCurrentInventory(t *testing.T) {
	root := repoRoot(t)
	manifest := loadLivePackagePlanManifest(t, root)
	examples := finrobotAggregateRegistryExamples(t, root)

	liveSkeletons := 0
	genericSkeletons := 0
	providerFreeFixtureIndexes := 0
	for _, skeleton := range manifest.LivePackageSkeletons {
		if skeleton.Status == "checked_in_registered_example" {
			liveSkeletons++
		}
		if strings.HasPrefix(skeleton.ID, "generic_") {
			genericSkeletons++
		}
	}
	fixtureIndexMatches, err := filepath.Glob(filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "*", "fixtures", "provider_free_fixture_index.json"))
	if err != nil {
		t.Fatal(err)
	}
	providerFreeFixtureIndexes = len(fixtureIndexMatches)
	if liveSkeletons == 0 || genericSkeletons == 0 || genericSkeletons > liveSkeletons {
		t.Fatalf("live skeleton counts = %d generic=%d, want non-empty coherent counts", liveSkeletons, genericSkeletons)
	}
	files := finrobotTranslationFileCount(t, root)

	docs := map[string][]string{
		"examples/ai/finrobot_translation/README.md": {
			fmt.Sprintf("%d runnable", len(examples)),
			fmt.Sprintf("%d directories", liveSkeletons),
			"generic_*",
			"FinRobot-specific language features",
		},
		"examples/ai/finrobot_translation/live_package_plan.md": {
			fmt.Sprintf("%d runnable", len(examples)),
			fmt.Sprintf("%d", liveSkeletons),
			"checked-in live-package",
			"generic_*",
			"reusable generic AI",
		},
		"examples/ai/finrobot_translation/COVERAGE.md": {
			fmt.Sprintf("%d files", files),
			fmt.Sprintf("%d\n  provider-free live-package skeleton directories", liveSkeletons),
			fmt.Sprintf("%d live-package skeletons use", providerFreeFixtureIndexes),
			fmt.Sprintf("`%d` registered generic AI live-package examples", genericSkeletons),
			"gap_manifest.json",
			"FR-HYGIENE-001",
			"tracked_exception",
		},
		"examples/ai/finrobot_translation/VERIFICATION.md": {
			fmt.Sprintf("%d files", files),
			fmt.Sprintf("`%d` registered live-package examples", liveSkeletons),
			fmt.Sprintf("`%d` registered generic AI live-package examples", genericSkeletons),
			fmt.Sprintf("`%d` provider-free fixture indexes", providerFreeFixtureIndexes),
			"gap_manifest.json",
			"tracked_exception",
		},
		"examples/ai/finrobot_translation/GAPS.md": {
			fmt.Sprintf("%d registered examples", len(examples)),
			fmt.Sprintf("%d checked-in live-package skeleton directories", liveSkeletons),
			"no open generic AI dialect gap",
			"gap_manifest.json",
			"FR-HYGIENE-001",
			"tracked exception",
		},
	}
	for rel, expected := range docs {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, want := range expected {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing current inventory phrase %q", rel, want)
			}
		}
		for _, stale := range []string{
			"65 runnable",
			"66 runnable",
			"22 checked-in",
			"22 directories",
			"20 registered `live_packages/generic_*` examples",
			"42 checked-in",
			"451 files",
			"597 files",
			"630 files",
			"633 files",
			"635 files",
			"685 files",
			"735 files",
			"`88` examples",
			"`93` examples",
			"93 runnable",
			"`94` examples",
			"94 runnable",
			"`95` examples",
			"95 runnable",
			"`96` examples",
			"96 runnable",
			"47 checked-in",
			"47 directories",
			"48 checked-in",
			"48 directories",
			"49 checked-in",
			"49 directories",
			"50 checked-in",
			"50 directories",
			"25 registered `live_packages/generic_*` examples",
			"26 registered `live_packages/generic_*` examples",
			"27 registered `live_packages/generic_*` examples",
			"28 registered `live_packages/generic_*` examples",
			"32 live-package skeletons use",
			"755 files",
			"770 files",
			"785 files",
			"FR-HYGIENE-002",
			"html_ui_snapshots` and\n`vendor_adapters` are `tracked_exception`",
		} {
			if strings.Contains(text, stale) {
				t.Fatalf("%s still contains stale inventory phrase %q", rel, stale)
			}
		}
	}
}

func TestFinRobotGapManifestTracksPackageHygieneExceptions(t *testing.T) {
	root := repoRoot(t)
	manifest := loadFinRobotGapManifest(t, root)
	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-ai-dialect-gap-manifest" {
		t.Fatalf("gap manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	ledger := loadFinRobotPackageManifestAuditLedger(t, root)
	waivedPackages := map[string]bool{}
	for _, waiver := range ledger.Waivers.NoBuiltInGuarantee {
		waivedPackages[waiver.PackageID] = true
	}
	if len(manifest.PackageHygieneExceptions) != len(waivedPackages) {
		t.Fatalf("package_hygiene_exceptions = %d, want waiver count %d", len(manifest.PackageHygieneExceptions), len(waivedPackages))
	}
	wantPackages := map[string]string{
		"FR-HYGIENE-001": "html_ui_snapshots",
	}
	for _, exception := range manifest.PackageHygieneExceptions {
		if wantPackages[exception.ID] != exception.PackageID {
			t.Fatalf("unexpected package hygiene exception: %#v", exception)
		}
		if !waivedPackages[exception.PackageID] {
			t.Fatalf("%s exception package %q is not backed by package_manifest_audit waiver", exception.ID, exception.PackageID)
		}
		if exception.Status != "tracked_exception" {
			t.Fatalf("%s status = %q, want tracked_exception", exception.ID, exception.Status)
		}
		if strings.EqualFold(exception.Status, "closed") {
			t.Fatalf("%s must not be marked closed", exception.ID)
		}
		if exception.Source != "package_manifest_audit/ledger.json" ||
			exception.ExceptionKind == "" ||
			exception.Reason == "" ||
			exception.NotClosedReason == "" ||
			len(exception.Guardrails) < 4 {
			t.Fatalf("%s has incomplete hygiene tracking: %#v", exception.ID, exception)
		}
	}
	if len(manifest.OpenProductionGaps) == 0 {
		t.Fatal("gap_manifest.json must keep real production gaps visible")
	}
	for _, gap := range manifest.OpenProductionGaps {
		if gap.Status != "open" {
			t.Fatalf("%s production gap status = %q, want open", gap.ID, gap.Status)
		}
		if gap.Area == "" || gap.Summary == "" {
			t.Fatalf("%s has incomplete production gap tracking: %#v", gap.ID, gap)
		}
	}
	allowedGuardStatus := map[string]bool{"closed": true, "non_gap": true}
	for _, guard := range manifest.ClosedOrNonGapGuards {
		if !allowedGuardStatus[guard.Status] {
			t.Fatalf("%s guard status = %q", guard.ID, guard.Status)
		}
	}
	for _, rel := range []string{
		"examples/ai/finrobot_translation/GAPS.md",
		"examples/ai/finrobot_translation/COVERAGE.md",
		"examples/ai/finrobot_translation/VERIFICATION.md",
	} {
		text := readFinRobotStatusDoc(t, root, rel)
		for _, want := range []string{"gap_manifest.json", "tracked_exception"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing structured gap manifest phrase %q", rel, want)
			}
		}
	}
}

func finrobotTranslationFileCount(t *testing.T, root string) int {
	t.Helper()
	count := 0
	base := filepath.Join(root, "examples", "ai", "finrobot_translation")
	if err := filepath.WalkDir(base, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return count
}

func loadFinRobotGapManifest(t *testing.T, root string) finrobotGapManifest {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "gap_manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest finrobotGapManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode gap_manifest.json: %v", err)
	}
	return manifest
}

func readFinRobotStatusDoc(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
