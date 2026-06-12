package leia_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type finrobotEvaluationHarnessManifest struct {
	SchemaVersion    int    `json:"schema_version"`
	HarnessID        string `json:"harness_id"`
	FixtureVersion   string `json:"fixture_version"`
	RecordsInventory []struct {
		ID                  string   `json:"id"`
		FixtureVersion      string   `json:"fixture_version"`
		SourcePath          string   `json:"source_path"`
		SourceRecordsPath   string   `json:"source_records_path"`
		ReplayRecordsPath   string   `json:"replay_records_path"`
		SourceRecordsSHA256 string   `json:"source_records_sha256"`
		ReplayRecordsSHA256 string   `json:"replay_records_sha256"`
		TurnCount           int      `json:"turn_count"`
		Models              []string `json:"models"`
		GoldenReport        struct {
			Status           string   `json:"status"`
			EvaluateBlocks   int      `json:"evaluate_blocks"`
			CasesPassed      int      `json:"cases_passed"`
			CasesFailed      int      `json:"cases_failed"`
			Assertions       int      `json:"assertions"`
			LLMLoadedTurns   int      `json:"llm_loaded_turns"`
			LLMReplayedTurns int      `json:"llm_replayed_turns"`
			LLMTurns         int      `json:"llm_turns"`
			CaseNames        []string `json:"case_names"`
		} `json:"golden_report"`
	} `json:"records_inventory"`
}

func TestFinRobotEvaluationHarnessManifestInventoriesOfflineRecords(t *testing.T) {
	root := repoRoot(t)
	manifest := loadFinRobotEvaluationHarnessManifest(t, root)
	if manifest.SchemaVersion != 1 || manifest.HarnessID != "FR-GAP-023-finrobot-evaluation-harness" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.HarnessID)
	}
	if manifest.FixtureVersion != "finrobot-eval-fixtures-v1" {
		t.Fatalf("fixture_version = %q", manifest.FixtureVersion)
	}

	gotInventory := make([]string, 0, len(manifest.RecordsInventory))
	for _, entry := range manifest.RecordsInventory {
		if entry.FixtureVersion != manifest.FixtureVersion {
			t.Fatalf("%s fixture_version = %q, want %q", entry.ID, entry.FixtureVersion, manifest.FixtureVersion)
		}
		if entry.SourcePath == "" || entry.SourceRecordsPath == "" || entry.ReplayRecordsPath == "" {
			t.Fatalf("%s has incomplete paths: %#v", entry.ID, entry)
		}
		gotInventory = append(gotInventory, filepath.ToSlash(entry.SourceRecordsPath))
		assertFileSHA256(t, filepath.Join(root, entry.SourceRecordsPath), entry.SourceRecordsSHA256)
		assertFileSHA256(t, filepath.Join(root, entry.ReplayRecordsPath), entry.ReplayRecordsSHA256)
		assertRecordFixtureShape(t, filepath.Join(root, entry.ReplayRecordsPath), entry.TurnCount, entry.Models)
	}
	sort.Strings(gotInventory)

	var wantInventory []string
	err := filepath.WalkDir(filepath.Join(root, "examples", "ai", "finrobot_translation"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "evaluation_harness" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".records.json") {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			wantInventory = append(wantInventory, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(wantInventory)
	if !reflect.DeepEqual(gotInventory, wantInventory) {
		t.Fatalf("records inventory mismatch\ngot  %#v\nwant %#v", gotInventory, wantInventory)
	}
}

func TestFinRobotEvaluationHarnessRunsProviderFreeCIReports(t *testing.T) {
	root := repoRoot(t)
	manifest := loadFinRobotEvaluationHarnessManifest(t, root)
	for _, entry := range manifest.RecordsInventory {
		entry := entry
		t.Run(entry.ID, func(t *testing.T) {
			reportPath := filepath.Join(t.TempDir(), entry.ID+".report.json")
			cmd := exec.Command(
				"go", "run", "./cmd/leia",
				"evaluate",
				"--gate",
				"--report", reportPath,
				"--replay", entry.ReplayRecordsPath,
				entry.SourcePath,
			)
			cmd.Dir = root
			cmd.Env = withoutLLMProviderEnv(os.Environ())
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("evaluate failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
			}
			var report struct {
				Status  string `json:"status"`
				Summary struct {
					EvaluateBlocks int `json:"evaluate_blocks"`
					CasesPassed    int `json:"cases_passed"`
					CasesFailed    int `json:"cases_failed"`
					Assertions     int `json:"assertions"`
				} `json:"summary"`
				LLM *struct {
					Mode          string `json:"mode"`
					LoadedTurns   int    `json:"loaded_turns"`
					ReplayedTurns int    `json:"replayed_turns"`
					Turns         int    `json:"turns"`
				} `json:"llm"`
				Cases []struct {
					Name   string `json:"name"`
					Status string `json:"status"`
				} `json:"cases"`
				Findings []any `json:"findings"`
			}
			data, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatalf("decode report: %v\n%s", err, string(data))
			}
			if report.Status != entry.GoldenReport.Status || report.Summary.EvaluateBlocks != entry.GoldenReport.EvaluateBlocks ||
				report.Summary.CasesPassed != entry.GoldenReport.CasesPassed || report.Summary.CasesFailed != entry.GoldenReport.CasesFailed ||
				report.Summary.Assertions != entry.GoldenReport.Assertions {
				t.Fatalf("report summary = %#v, golden = %#v", report.Summary, entry.GoldenReport)
			}
			if report.LLM == nil || report.LLM.Mode != "replay" || report.LLM.LoadedTurns != entry.GoldenReport.LLMLoadedTurns ||
				report.LLM.ReplayedTurns != entry.GoldenReport.LLMReplayedTurns || report.LLM.Turns != entry.GoldenReport.LLMTurns {
				t.Fatalf("llm report = %#v, golden = %#v", report.LLM, entry.GoldenReport)
			}
			if len(report.Findings) != 0 {
				t.Fatalf("findings = %#v, want none", report.Findings)
			}
			var caseNames []string
			for _, c := range report.Cases {
				if c.Status != "passed" {
					t.Fatalf("case %q status = %q", c.Name, c.Status)
				}
				caseNames = append(caseNames, c.Name)
			}
			if !reflect.DeepEqual(caseNames, entry.GoldenReport.CaseNames) {
				t.Fatalf("case names = %#v, want %#v", caseNames, entry.GoldenReport.CaseNames)
			}
		})
	}
}

func loadFinRobotEvaluationHarnessManifest(t *testing.T, root string) finrobotEvaluationHarnessManifest {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "evaluation_harness", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest finrobotEvaluationHarnessManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return manifest
}

func assertFileSHA256(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("%s sha256 = %s, want %s", path, got, want)
	}
}

func assertRecordFixtureShape(t *testing.T, path string, wantTurns int, wantModels []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var records []struct {
		Request struct {
			Model string `json:"Model"`
		} `json:"Request"`
	}
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if len(records) != wantTurns {
		t.Fatalf("%s turns = %d, want %d", path, len(records), wantTurns)
	}
	seen := map[string]bool{}
	for _, record := range records {
		seen[record.Request.Model] = true
	}
	var gotModels []string
	for model := range seen {
		gotModels = append(gotModels, model)
	}
	sort.Strings(gotModels)
	if !reflect.DeepEqual(gotModels, wantModels) {
		t.Fatalf("%s models = %#v, want %#v", path, gotModels, wantModels)
	}
}

func withoutLLMProviderEnv(env []string) []string {
	filtered := env[:0]
	for _, item := range env {
		if strings.HasPrefix(item, "LEIA_LLM_") ||
			strings.HasPrefix(item, "OPENAI_") ||
			strings.HasPrefix(item, "ANTHROPIC_") {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}
