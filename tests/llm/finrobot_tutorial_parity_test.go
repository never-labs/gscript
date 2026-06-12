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

type finrobotTutorialParityLedger struct {
	SchemaVersion int `json:"schema_version"`
	ID            string
	Source        struct {
		TutorialRoots []string `json:"tutorial_roots"`
	} `json:"source"`
	Target struct {
		TranslationRoot             string `json:"translation_root"`
		ProviderFreeDefault         bool   `json:"provider_free_default"`
		LiveNetworkDefault          bool   `json:"live_network_default"`
		RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
	} `json:"target"`
	StatusValues []string `json:"status_values"`
	GlobalGates  []string `json:"global_gates"`
	Tutorials    []struct {
		ID                         string   `json:"id"`
		Level                      string   `json:"level"`
		SourcePath                 string   `json:"source_path"`
		SourceTitle                string   `json:"source_title"`
		Status                     string   `json:"status"`
		MappedExamples             []string `json:"mapped_examples"`
		MappedLivePackageSkeletons []string `json:"mapped_live_package_skeletons"`
		CoveredConcepts            []string `json:"covered_concepts"`
		Gaps                       []string `json:"gaps"`
		OptionalLiveGates          []struct {
			ID                          string   `json:"id"`
			Capabilities                []string `json:"capabilities"`
			LiveNetworkDefault          bool     `json:"live_network_default"`
			DefaultEnabled              bool     `json:"default_enabled"`
			RequiresCredentials         bool     `json:"requires_credentials"`
			CleanSkipWithoutCredentials bool     `json:"clean_skip_without_credentials"`
			Skeleton                    string   `json:"skeleton"`
		} `json:"optional_live_gates"`
	} `json:"tutorials"`
}

func TestFinRobotTutorialParityLedgerComplete(t *testing.T) {
	ledger := loadFinRobotTutorialParityLedger(t)
	if ledger.SchemaVersion != 1 || ledger.ID != "finrobot-tutorial-parity-ledger" {
		t.Fatalf("ledger header = schema %d id %q", ledger.SchemaVersion, ledger.ID)
	}
	if ledger.Target.TranslationRoot != "examples/ai/finrobot_translation" {
		t.Fatalf("translation root = %q", ledger.Target.TranslationRoot)
	}
	if !ledger.Target.ProviderFreeDefault || ledger.Target.LiveNetworkDefault || ledger.Target.RealDependencyImportDefault {
		t.Fatalf("target defaults must be provider-free with no live network/imports: %#v", ledger.Target)
	}
	if !reflect.DeepEqual(sortedCopy(ledger.Source.TutorialRoots), []string{"tutorials_advanced", "tutorials_beginner"}) {
		t.Fatalf("tutorial roots = %#v", ledger.Source.TutorialRoots)
	}

	wantSources := []string{
		"tutorials_advanced/agent_annual_report.ipynb",
		"tutorials_advanced/agent_fingpt_forecaster.ipynb",
		"tutorials_advanced/agent_openbb.ipynb",
		"tutorials_advanced/agent_trade_strategist.ipynb",
		"tutorials_advanced/lmm_agent_mplfinance.ipynb",
		"tutorials_advanced/lmm_agent_opt_smacross.ipynb",
		"tutorials_beginner/agent_annual_report.ipynb",
		"tutorials_beginner/agent_fingpt_forecaster.ipynb",
		"tutorials_beginner/agent_rag_earnings_call_sec_filings.ipynb",
		"tutorials_beginner/agent_rag_qa.ipynb",
		"tutorials_beginner/agent_rag_qa_up.ipynb",
		"tutorials_beginner/ollama function call.ipynb",
		"tutorials_beginner/ollama stock chart.ipynb",
	}
	gotSources := make([]string, 0, len(ledger.Tutorials))
	levels := map[string]int{}
	ids := map[string]bool{}
	statusValues := stringSet(ledger.StatusValues)
	for _, tutorial := range ledger.Tutorials {
		if ids[tutorial.ID] {
			t.Fatalf("duplicate tutorial id %q", tutorial.ID)
		}
		ids[tutorial.ID] = true
		gotSources = append(gotSources, tutorial.SourcePath)
		levels[tutorial.Level]++
		if tutorial.SourceTitle == "" {
			t.Fatalf("%s missing source title", tutorial.ID)
		}
		if !statusValues[tutorial.Status] {
			t.Fatalf("%s unknown status %q", tutorial.ID, tutorial.Status)
		}
	}
	sort.Strings(gotSources)
	if !reflect.DeepEqual(gotSources, wantSources) {
		t.Fatalf("tutorial sources mismatch\ngot  %#v\nwant %#v", gotSources, wantSources)
	}
	if levels["beginner"] != 7 || levels["advanced"] != 6 {
		t.Fatalf("level counts = %#v, want beginner=7 advanced=6", levels)
	}
}

func TestFinRobotTutorialParityLedgerMappingsAndGaps(t *testing.T) {
	root := repoRoot(t)
	ledger := loadFinRobotTutorialParityLedger(t)
	for _, tutorial := range ledger.Tutorials {
		if len(tutorial.MappedExamples) == 0 {
			t.Fatalf("%s has no mapped Leia examples", tutorial.ID)
		}
		if len(tutorial.CoveredConcepts) == 0 {
			t.Fatalf("%s has no covered concepts", tutorial.ID)
		}
		if len(tutorial.Gaps) == 0 {
			t.Fatalf("%s must record explicit parity gaps", tutorial.ID)
		}
		for _, path := range tutorial.MappedExamples {
			if !strings.HasPrefix(path, "examples/ai/finrobot_translation/") {
				t.Fatalf("%s mapped example outside translation root: %s", tutorial.ID, path)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				t.Fatalf("%s mapped example %s: %v", tutorial.ID, path, err)
			}
		}
		for _, path := range tutorial.MappedLivePackageSkeletons {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				t.Fatalf("%s live package skeleton %s: %v", tutorial.ID, path, err)
			}
		}
	}
}

func TestFinRobotTutorialParityLedgerNoLiveNetworkDefault(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "tutorial_parity", "ledger.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"live_network_default": true`) ||
		strings.Contains(string(data), `"default_enabled": true`) ||
		strings.Contains(string(data), `"real_dependency_import_default": true`) {
		t.Fatalf("ledger must not enable live network, optional gates, or real imports by default")
	}

	ledger := loadFinRobotTutorialParityLedger(t)
	optionalGateCount := 0
	for _, tutorial := range ledger.Tutorials {
		if len(tutorial.OptionalLiveGates) == 0 {
			t.Fatalf("%s must describe at least one optional live gate", tutorial.ID)
		}
		for _, gate := range tutorial.OptionalLiveGates {
			optionalGateCount++
			if gate.ID == "" || gate.Skeleton == "" || len(gate.Capabilities) == 0 {
				t.Fatalf("%s has incomplete optional gate: %#v", tutorial.ID, gate)
			}
			if gate.LiveNetworkDefault || gate.DefaultEnabled {
				t.Fatalf("%s gate %s must be disabled by default: %#v", tutorial.ID, gate.ID, gate)
			}
			if !gate.CleanSkipWithoutCredentials {
				t.Fatalf("%s gate %s must cleanly skip when optional credentials/services are absent", tutorial.ID, gate.ID)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gate.Skeleton))); err != nil {
				t.Fatalf("%s gate %s skeleton %s: %v", tutorial.ID, gate.ID, gate.Skeleton, err)
			}
		}
	}
	if optionalGateCount < len(ledger.Tutorials) {
		t.Fatalf("optional gate count = %d, want at least one per tutorial", optionalGateCount)
	}

	joinedGates := strings.ToLower(strings.Join(ledger.GlobalGates, " "))
	for _, want := range []string{"must not call live providers", "live_network_default=false", "default_enabled=false", "credentials"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("global gates missing %q: %s", want, joinedGates)
		}
	}
}

func loadFinRobotTutorialParityLedger(t *testing.T) finrobotTutorialParityLedger {
	t.Helper()
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "tutorial_parity", "ledger.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ledger finrobotTutorialParityLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("decode tutorial parity ledger: %v", err)
	}
	return ledger
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}
