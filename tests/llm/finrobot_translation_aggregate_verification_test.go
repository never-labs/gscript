package leia_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type finrobotAggregateExample struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Runnable  bool   `json:"runnable"`
	Checkable bool   `json:"checkable"`
	Runner    string `json:"runner"`
	Requires  string `json:"requires"`
}

func TestFinRobotTranslationAggregateVerification(t *testing.T) {
	root := repoRoot(t)
	manifest := loadFinRobotEvaluationHarnessManifest(t, root)

	recordedSources := map[string]string{}
	for _, entry := range manifest.RecordsInventory {
		if entry.SourcePath == "" || entry.SourceRecordsPath == "" || entry.ReplayRecordsPath == "" {
			t.Fatalf("%s has incomplete records paths", entry.ID)
		}
		if entry.SourceRecordsPath != entry.ReplayRecordsPath {
			t.Fatalf("%s source/replay records diverge: %s vs %s", entry.ID, entry.SourceRecordsPath, entry.ReplayRecordsPath)
		}
		assertFileSHA256(t, filepath.Join(root, entry.SourceRecordsPath), entry.SourceRecordsSHA256)
		assertFileSHA256(t, filepath.Join(root, entry.ReplayRecordsPath), entry.ReplayRecordsSHA256)
		assertRecordFixtureShape(t, filepath.Join(root, entry.ReplayRecordsPath), entry.TurnCount, entry.Models)
		recordedSources[filepath.ToSlash(entry.SourcePath)] = filepath.ToSlash(entry.ReplayRecordsPath)
	}

	registryExamples := finrobotAggregateRegistryExamples(t, root)
	filesystemExamples := finrobotAggregateFilesystemExamples(t, root)
	if !reflect.DeepEqual(pathsOfFinRobotAggregateExamples(registryExamples), filesystemExamples) {
		t.Fatalf("FinRobot registry paths mismatch\ngot  %#v\nwant %#v", pathsOfFinRobotAggregateExamples(registryExamples), filesystemExamples)
	}

	seenRecordedSources := map[string]bool{}
	for _, example := range registryExamples {
		if !example.Runnable || !example.Checkable {
			t.Fatalf("%s registry state = runnable:%t checkable:%t, want provider-free runnable/checkable", example.ID, example.Runnable, example.Checkable)
		}
		if example.Requires != "" {
			t.Fatalf("%s has unexpected provider/manual requirement %q", example.ID, example.Requires)
		}
		wantRunner := "host-vm"
		if recordsPath, ok := recordedSources[example.Path]; ok {
			wantRunner = "llm-replay"
			seenRecordedSources[example.Path] = true
			if _, err := os.Stat(filepath.Join(root, recordsPath)); err != nil {
				t.Fatalf("%s registry points at missing replay records %s: %v", example.ID, recordsPath, err)
			}
		} else if finrobotAggregateHasEvaluateBlock(t, root, example.Path) {
			wantRunner = "evaluate"
		}
		if example.Runner != wantRunner {
			t.Fatalf("%s runner = %q, want %q", example.ID, example.Runner, wantRunner)
		}
	}
	for source := range recordedSources {
		if !seenRecordedSources[source] {
			t.Fatalf("records manifest source %s is not present in examples registry", source)
		}
	}

	report := finrobotAggregateExamplesCheck(t, root)
	if report.SchemaVersion != 1 || !report.OK || report.Runnable != len(registryExamples) || report.Skipped != 0 || report.Failed != 0 {
		t.Fatalf("aggregate examples check summary = schema:%d ok:%t runnable:%d skipped:%d failed:%d, want all %d runnable with no skips/failures",
			report.SchemaVersion, report.OK, report.Runnable, report.Skipped, report.Failed, len(registryExamples))
	}
	gotChecked := make([]string, 0, len(report.Results))
	for _, result := range report.Results {
		if result.Status != "ok" {
			t.Fatalf("%s status = %q, requires=%q error=%q", result.ID, result.Status, result.Requires, result.Error)
		}
		if result.Requires != "" {
			t.Fatalf("%s check result has unexpected skip/provider requirement %q", result.ID, result.Requires)
		}
		gotChecked = append(gotChecked, filepath.ToSlash(result.Path))
	}
	sort.Strings(gotChecked)
	if !reflect.DeepEqual(gotChecked, filesystemExamples) {
		t.Fatalf("checked examples mismatch\ngot  %#v\nwant %#v", gotChecked, filesystemExamples)
	}
}

func finrobotAggregateRegistryExamples(t *testing.T, root string) []finrobotAggregateExample {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "run", "./cmd/leia", "examples", "list", "--json")
	cmd.Dir = root
	cmd.Env = withoutLLMProviderEnv(os.Environ())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("examples list failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion int                        `json:"schema_version"`
		Examples      []finrobotAggregateExample `json:"examples"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode examples list: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("examples list schema_version = %d, want 1", payload.SchemaVersion)
	}
	var out []finrobotAggregateExample
	for _, example := range payload.Examples {
		if strings.HasPrefix(filepath.ToSlash(example.Path), "examples/ai/finrobot_translation/") {
			out = append(out, example)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	if len(out) == 0 {
		t.Fatal("examples registry has no FinRobot translation examples")
	}
	return out
}

func finrobotAggregateFilesystemExamples(t *testing.T, root string) []string {
	t.Helper()
	base := filepath.Join(root, "examples", "ai", "finrobot_translation")
	var paths []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "evaluation_harness" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".leia") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

func finrobotAggregateHasEvaluateBlock(t *testing.T, root, path string) bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "evaluate ") {
			return true
		}
	}
	return false
}

func pathsOfFinRobotAggregateExamples(examples []finrobotAggregateExample) []string {
	paths := make([]string, 0, len(examples))
	for _, example := range examples {
		paths = append(paths, filepath.ToSlash(example.Path))
	}
	sort.Strings(paths)
	return paths
}

func finrobotAggregateExamplesCheck(t *testing.T, root string) struct {
	SchemaVersion int  `json:"schema_version"`
	OK            bool `json:"ok"`
	Runnable      int  `json:"runnable"`
	Skipped       int  `json:"skipped"`
	Failed        int  `json:"failed"`
	Results       []struct {
		ID       string `json:"id"`
		Path     string `json:"path"`
		Status   string `json:"status"`
		Requires string `json:"requires"`
		Error    string `json:"error"`
	} `json:"results"`
} {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(
		"go", "run", "./cmd/leia",
		"examples", "check",
		"--json",
		"--jobs=4",
		"--timeout=30s",
		"examples/ai/finrobot_translation",
	)
	cmd.Dir = root
	cmd.Env = withoutLLMProviderEnv(os.Environ())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("examples check failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion int  `json:"schema_version"`
		OK            bool `json:"ok"`
		Runnable      int  `json:"runnable"`
		Skipped       int  `json:"skipped"`
		Failed        int  `json:"failed"`
		Results       []struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			Status   string `json:"status"`
			Requires string `json:"requires"`
			Error    string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode examples check: %v\n%s", err, stdout.String())
	}
	return payload
}
