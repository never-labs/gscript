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

type finrobotDemoParityLedger struct {
	SchemaVersion int `json:"schema_version"`
	ID            string
	Source        struct {
		SourceRoots []string `json:"source_roots"`
	} `json:"source"`
	Target struct {
		TranslationRoot             string `json:"translation_root"`
		ProviderFreeDefault         bool   `json:"provider_free_default"`
		LiveNetworkDefault          bool   `json:"live_network_default"`
		RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
	} `json:"target"`
	StatusValues    []string `json:"status_values"`
	GlobalGates     []string `json:"global_gates"`
	RecordsManifest string   `json:"records_manifest"`
	Sources         []struct {
		ID                     string   `json:"id"`
		Kind                   string   `json:"kind"`
		SourcePath             string   `json:"source_path"`
		SourceTitle            string   `json:"source_title"`
		Status                 string   `json:"status"`
		MappedExamples         []string `json:"mapped_examples"`
		MappedEvaluateExamples []string `json:"mapped_evaluate_examples"`
		MappedEvaluateRecords  []struct {
			ManifestRecordID  string   `json:"manifest_record_id"`
			SourcePath        string   `json:"source_path"`
			ReplayRecordsPath string   `json:"replay_records_path"`
			CoveredCases      []string `json:"covered_cases"`
		} `json:"mapped_evaluate_records"`
		CoveredConcepts   []string `json:"covered_concepts"`
		Gaps              []string `json:"gaps"`
		OptionalLiveGates []struct {
			ID                          string   `json:"id"`
			Capabilities                []string `json:"capabilities"`
			LiveNetworkDefault          bool     `json:"live_network_default"`
			DefaultEnabled              bool     `json:"default_enabled"`
			RequiresCredentials         bool     `json:"requires_credentials"`
			CleanSkipWithoutCredentials bool     `json:"clean_skip_without_credentials"`
			Skeleton                    string   `json:"skeleton"`
		} `json:"optional_live_gates"`
	} `json:"sources"`
}

func TestFinRobotDemoParityLedgerComplete(t *testing.T) {
	ledger := loadFinRobotDemoParityLedger(t)
	if ledger.SchemaVersion != 1 || ledger.ID != "finrobot-demo-regression-parity-ledger" {
		t.Fatalf("ledger header = schema %d id %q", ledger.SchemaVersion, ledger.ID)
	}
	if ledger.Target.TranslationRoot != "examples/ai/finrobot_translation" {
		t.Fatalf("translation root = %q", ledger.Target.TranslationRoot)
	}
	if !ledger.Target.ProviderFreeDefault || ledger.Target.LiveNetworkDefault || ledger.Target.RealDependencyImportDefault {
		t.Fatalf("target defaults must be provider-free with no live network/imports: %#v", ledger.Target)
	}
	if ledger.RecordsManifest != "examples/ai/finrobot_translation/evaluation_harness/manifest.json" {
		t.Fatalf("records manifest = %q", ledger.RecordsManifest)
	}

	wantSources := []string{
		"agent_builder_demo.py",
		"finrobot_equity/core/tests/test_generate_report.py",
		"finrobot_equity/core/tests/test_modules.py",
		"finrobot_equity/core/tests/test_retail_sentiment_client.py",
		"test_module.py",
	}
	gotSources := make([]string, 0, len(ledger.Sources))
	ids := map[string]bool{}
	kinds := map[string]int{}
	statusValues := finrobotDemoStringSet(ledger.StatusValues)
	for _, source := range ledger.Sources {
		if ids[source.ID] {
			t.Fatalf("duplicate source id %q", source.ID)
		}
		ids[source.ID] = true
		gotSources = append(gotSources, source.SourcePath)
		kinds[source.Kind]++
		if source.SourceTitle == "" {
			t.Fatalf("%s missing source title", source.ID)
		}
		if !statusValues[source.Status] {
			t.Fatalf("%s unknown status %q", source.ID, source.Status)
		}
	}
	sort.Strings(gotSources)
	if !reflect.DeepEqual(gotSources, wantSources) {
		t.Fatalf("demo parity sources mismatch\ngot  %#v\nwant %#v", gotSources, wantSources)
	}
	if kinds["demo"] != 1 || kinds["demo_regression"] != 1 || kinds["core_test"] != 3 {
		t.Fatalf("source kinds = %#v, want demo=1 demo_regression=1 core_test=3", kinds)
	}
}

func TestFinRobotDemoParityMappingsRecordsAndGaps(t *testing.T) {
	root := repoRoot(t)
	ledger := loadFinRobotDemoParityLedger(t)
	manifest := loadFinRobotEvaluationHarnessManifest(t, root)
	recordsByID := map[string]struct {
		SourcePath        string
		ReplayRecordsPath string
		CaseNames         []string
	}{}
	for _, entry := range manifest.RecordsInventory {
		recordsByID[entry.ID] = struct {
			SourcePath        string
			ReplayRecordsPath string
			CaseNames         []string
		}{
			SourcePath:        filepath.ToSlash(entry.SourcePath),
			ReplayRecordsPath: filepath.ToSlash(entry.ReplayRecordsPath),
			CaseNames:         entry.GoldenReport.CaseNames,
		}
	}

	mappedRecordCount := 0
	for _, source := range ledger.Sources {
		if len(source.MappedExamples) == 0 {
			t.Fatalf("%s has no mapped Leia examples", source.ID)
		}
		if len(source.CoveredConcepts) == 0 {
			t.Fatalf("%s has no covered concepts", source.ID)
		}
		if len(source.Gaps) == 0 {
			t.Fatalf("%s must record explicit parity gaps", source.ID)
		}
		for _, path := range source.MappedExamples {
			if !strings.HasPrefix(path, "examples/ai/finrobot_translation/") {
				t.Fatalf("%s mapped example outside translation root: %s", source.ID, path)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				t.Fatalf("%s mapped example %s: %v", source.ID, path, err)
			}
		}
		for _, path := range source.MappedEvaluateExamples {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				t.Fatalf("%s mapped evaluate example %s: %v", source.ID, path, err)
			}
			if !finrobotAggregateHasEvaluateBlock(t, root, path) {
				t.Fatalf("%s mapped evaluate example %s has no evaluate block", source.ID, path)
			}
		}
		for _, record := range source.MappedEvaluateRecords {
			mappedRecordCount++
			manifestRecord, ok := recordsByID[record.ManifestRecordID]
			if !ok {
				t.Fatalf("%s maps unknown manifest record %q", source.ID, record.ManifestRecordID)
			}
			if record.SourcePath != manifestRecord.SourcePath || record.ReplayRecordsPath != manifestRecord.ReplayRecordsPath {
				t.Fatalf("%s record %s path mismatch: ledger=(%s,%s) manifest=(%s,%s)",
					source.ID, record.ManifestRecordID, record.SourcePath, record.ReplayRecordsPath,
					manifestRecord.SourcePath, manifestRecord.ReplayRecordsPath)
			}
			if !reflect.DeepEqual(record.CoveredCases, manifestRecord.CaseNames) {
				t.Fatalf("%s record %s covered cases = %#v, want %#v",
					source.ID, record.ManifestRecordID, record.CoveredCases, manifestRecord.CaseNames)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(record.ReplayRecordsPath))); err != nil {
				t.Fatalf("%s replay records %s: %v", source.ID, record.ReplayRecordsPath, err)
			}
		}
	}
	if mappedRecordCount < 4 {
		t.Fatalf("mapped evaluate record count = %d, want at least 4", mappedRecordCount)
	}
}

func TestFinRobotDemoParityLedgerNoLiveDefaults(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "demo_parity", "ledger.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	if strings.Contains(raw, `"live_network_default": true`) ||
		strings.Contains(raw, `"default_enabled": true`) ||
		strings.Contains(raw, `"real_dependency_import_default": true`) {
		t.Fatalf("ledger must not enable live network, optional gates, or real imports by default")
	}

	ledger := loadFinRobotDemoParityLedger(t)
	optionalGateCount := 0
	for _, source := range ledger.Sources {
		if len(source.OptionalLiveGates) == 0 {
			t.Fatalf("%s must describe at least one optional live gate", source.ID)
		}
		for _, gate := range source.OptionalLiveGates {
			optionalGateCount++
			if gate.ID == "" || gate.Skeleton == "" || len(gate.Capabilities) == 0 {
				t.Fatalf("%s has incomplete optional gate: %#v", source.ID, gate)
			}
			if gate.LiveNetworkDefault || gate.DefaultEnabled {
				t.Fatalf("%s gate %s must be disabled by default: %#v", source.ID, gate.ID, gate)
			}
			if !gate.CleanSkipWithoutCredentials {
				t.Fatalf("%s gate %s must cleanly skip when optional credentials/services are absent", source.ID, gate.ID)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gate.Skeleton))); err != nil {
				t.Fatalf("%s gate %s skeleton %s: %v", source.ID, gate.ID, gate.Skeleton, err)
			}
		}
	}
	if optionalGateCount != len(ledger.Sources) {
		t.Fatalf("optional gate count = %d, want one per source", optionalGateCount)
	}

	joinedGates := strings.ToLower(strings.Join(ledger.GlobalGates, " "))
	for _, want := range []string{"must not call live providers", "live_network_default=false", "default_enabled=false", "credentials"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("global gates missing %q: %s", want, joinedGates)
		}
	}
}

func loadFinRobotDemoParityLedger(t *testing.T) finrobotDemoParityLedger {
	t.Helper()
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "demo_parity", "ledger.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ledger finrobotDemoParityLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("decode demo parity ledger: %v", err)
	}
	return ledger
}

func finrobotDemoStringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}
