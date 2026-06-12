package leia_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type finrobotUpstreamCoverageLedger struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	AuditKind     string `json:"audit_kind"`
	Source        struct {
		Repo         string `json:"repo"`
		SourceRoot   string `json:"source_root"`
		SourceCommit string `json:"source_commit"`
		SourceMode   string `json:"source_mode"`
	} `json:"source"`
	Target struct {
		BaselineBranch              string `json:"baseline_branch"`
		BaselineCommit              string `json:"baseline_commit"`
		TranslationRoot             string `json:"translation_root"`
		ProviderFreeDefault         bool   `json:"provider_free_default"`
		LiveNetworkDefault          bool   `json:"live_network_default"`
		RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
	} `json:"target"`
	LedgerPolicy struct {
		GenericAudit                  bool `json:"generic_audit"`
		NotTestHardcoded              bool `json:"not_test_hardcoded"`
		NoQRuntimeDependency          bool `json:"no_q_runtime_dependency"`
		ProviderFreeValidation        bool `json:"provider_free_validation"`
		MissingItemsRequireNextAction bool `json:"missing_items_require_next_action"`
	} `json:"ledger_policy"`
	StatusValues    []string `json:"status_values"`
	SourceInventory []string `json:"source_inventory"`
	Records         []struct {
		ID             string   `json:"id"`
		SourcePath     string   `json:"source_path"`
		CoverageStatus string   `json:"coverage_status"`
		TargetExamples []string `json:"target_examples"`
		TargetPackages []string `json:"target_packages"`
		ExcludedScope  []string `json:"excluded_scope"`
		SourceHash     string   `json:"source_hash"`
		FixtureHashes  []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"fixture_hashes"`
		NextAction string `json:"next_action"`
	} `json:"records"`
}

func TestFinRobotUpstreamCoverageLedgerIsGenericAndComplete(t *testing.T) {
	ledger := loadFinRobotUpstreamCoverageLedger(t)
	if ledger.SchemaVersion != 1 || ledger.ID != "finrobot-upstream-coverage-ledger" {
		t.Fatalf("ledger header = schema %d id %q", ledger.SchemaVersion, ledger.ID)
	}
	if ledger.AuditKind != "ai_dialect_migration_upstream_coverage" {
		t.Fatalf("audit kind = %q", ledger.AuditKind)
	}
	if ledger.Source.Repo != "FinRobot" || ledger.Source.SourceRoot != ".external/FinRobot" || ledger.Source.SourceCommit == "" {
		t.Fatalf("source metadata incomplete: %#v", ledger.Source)
	}
	if ledger.Target.BaselineBranch != "origin/codex/ai-dialect-polish" ||
		ledger.Target.TranslationRoot != "examples/ai/finrobot_translation" {
		t.Fatalf("target metadata = %#v", ledger.Target)
	}
	if !ledger.Target.ProviderFreeDefault || ledger.Target.LiveNetworkDefault || ledger.Target.RealDependencyImportDefault {
		t.Fatalf("target defaults must be provider-free and offline: %#v", ledger.Target)
	}
	if !ledger.LedgerPolicy.GenericAudit ||
		!ledger.LedgerPolicy.NotTestHardcoded ||
		!ledger.LedgerPolicy.NoQRuntimeDependency ||
		!ledger.LedgerPolicy.ProviderFreeValidation ||
		!ledger.LedgerPolicy.MissingItemsRequireNextAction {
		t.Fatalf("ledger policy is too weak: %#v", ledger.LedgerPolicy)
	}

	if len(ledger.SourceInventory) < 40 {
		t.Fatalf("source inventory has %d entries, want broad upstream module coverage", len(ledger.SourceInventory))
	}
	if len(ledger.Records) != len(ledger.SourceInventory) {
		t.Fatalf("records = %d, source inventory = %d", len(ledger.Records), len(ledger.SourceInventory))
	}

	inventory := append([]string{}, ledger.SourceInventory...)
	recordSources := make([]string, 0, len(ledger.Records))
	ids := map[string]bool{}
	statusValues := finrobotUpstreamStringSet(ledger.StatusValues)
	for _, record := range ledger.Records {
		if record.ID == "" {
			t.Fatal("record missing id")
		}
		if ids[record.ID] {
			t.Fatalf("duplicate record id %q", record.ID)
		}
		ids[record.ID] = true
		recordSources = append(recordSources, record.SourcePath)
		if !statusValues[record.CoverageStatus] {
			t.Fatalf("%s has unknown coverage status %q", record.ID, record.CoverageStatus)
		}
	}
	sort.Strings(inventory)
	sort.Strings(recordSources)
	if !reflect.DeepEqual(recordSources, inventory) {
		t.Fatalf("record sources do not match source inventory\ngot  %#v\nwant %#v", recordSources, inventory)
	}
}

func TestFinRobotUpstreamCoverageRecordsMapToCheckedInTargets(t *testing.T) {
	root := repoRoot(t)
	ledger := loadFinRobotUpstreamCoverageLedger(t)
	hashPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	bareHashPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)

	statusCounts := map[string]int{}
	for _, record := range ledger.Records {
		statusCounts[record.CoverageStatus]++
		if record.SourcePath == "" {
			t.Fatalf("%s missing source path", record.ID)
		}
		if !hashPattern.MatchString(record.SourceHash) {
			t.Fatalf("%s source hash = %q, want sha256:<64 hex>", record.ID, record.SourceHash)
		}
		if len(record.FixtureHashes) == 0 {
			t.Fatalf("%s missing fixture hashes", record.ID)
		}
		if record.NextAction == "" {
			t.Fatalf("%s missing next action", record.ID)
		}
		if record.CoverageStatus != "excluded_non_ai_surface" && len(record.TargetExamples)+len(record.TargetPackages) == 0 {
			t.Fatalf("%s has no target example or package", record.ID)
		}
		if record.CoverageStatus != "covered_example" && len(record.ExcludedScope) == 0 {
			t.Fatalf("%s must record excluded scope for non-complete coverage", record.ID)
		}
		for _, path := range append(append([]string{}, record.TargetExamples...), record.TargetPackages...) {
			assertFinRobotUpstreamCoverageTargetPath(t, root, ledger.Target.TranslationRoot, path)
		}
		for _, fixture := range record.FixtureHashes {
			if !strings.HasPrefix(fixture.Path, ledger.Target.TranslationRoot+"/") {
				t.Fatalf("%s fixture outside translation root: %s", record.ID, fixture.Path)
			}
			if !bareHashPattern.MatchString(fixture.SHA256) {
				t.Fatalf("%s fixture hash for %s = %q", record.ID, fixture.Path, fixture.SHA256)
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fixture.Path)))
			if err != nil {
				t.Fatalf("%s fixture %s: %v", record.ID, fixture.Path, err)
			}
			sum := sha256.Sum256(data)
			if got := hex.EncodeToString(sum[:]); got != fixture.SHA256 {
				t.Fatalf("%s fixture hash mismatch for %s: got %s want %s", record.ID, fixture.Path, got, fixture.SHA256)
			}
		}
	}

	for _, status := range []string{
		"covered_example",
		"covered_live_package_skeleton",
		"partial_fixture_contract",
		"mapped_optional_gate",
		"excluded_non_ai_surface",
	} {
		if statusCounts[status] == 0 {
			t.Fatalf("coverage ledger has no records with status %q", status)
		}
	}
	if statusCounts["covered_live_package_skeleton"] < 15 {
		t.Fatalf("live package coverage records = %d, want broad skeleton mapping", statusCounts["covered_live_package_skeleton"])
	}
}

func TestFinRobotUpstreamCoverageProviderFreeAndNoRuntimeDependency(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, "examples", "ai", "finrobot_translation", "demo_parity", "upstream_coverage")
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(base, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, blocked := range []string{
			`"live_network_default"` + `: true`,
			`"real_dependency_import_default"` + `: true`,
			`"provider_free_default"` + `: false`,
			"q/" + "runtime",
			"import autogen",
			"import yfinance",
			"import finnhub",
			"import openbb",
		} {
			if strings.Contains(strings.ToLower(text), strings.ToLower(blocked)) {
				t.Fatalf("%s contains blocked provider/runtime marker %q", path, blocked)
			}
		}
	}
}

func loadFinRobotUpstreamCoverageLedger(t *testing.T) finrobotUpstreamCoverageLedger {
	t.Helper()
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "demo_parity", "upstream_coverage", "ledger.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ledger finrobotUpstreamCoverageLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("decode upstream coverage ledger: %v", err)
	}
	return ledger
}

func assertFinRobotUpstreamCoverageTargetPath(t *testing.T, root, translationRoot, path string) {
	t.Helper()
	if !strings.HasPrefix(path, translationRoot+"/") {
		t.Fatalf("target path outside translation root: %s", path)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatalf("target path %s: %v", path, err)
	}
}

func finrobotUpstreamStringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}
