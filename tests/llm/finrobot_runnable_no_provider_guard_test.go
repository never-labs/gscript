package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var finrobotProviderBoundSurface = regexp.MustCompile(`(?m)\bllm\.(turn|agent|react|register_models|dispatch|tool)\b|^\s*evaluate\s+"`)

func TestFinRobotRunnableExamplesDeclareNoProviderGuard(t *testing.T) {
	root := repoRoot(t)
	examples := finrobotAggregateRegistryExamples(t, root)
	recordedSources := finrobotNoProviderRecordedSources(t, root)

	var missing []string
	for _, example := range examples {
		if !strings.HasPrefix(example.Path, "examples/ai/finrobot_translation/") {
			continue
		}
		if !example.Runnable || !example.Checkable || example.Requires != "" {
			missing = append(missing, example.Path+" is not provider-free checkable in examples metadata")
			continue
		}

		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(example.Path)))
		if err != nil {
			t.Fatalf("read %s: %v", example.Path, err)
		}
		source := string(data)
		if !finrobotProviderBoundSurface.MatchString(source) {
			continue
		}

		if _, ok := recordedSources[filepath.ToSlash(example.Path)]; ok {
			if example.Runner != "llm-replay" {
				missing = append(missing, example.Path+" has replay records but examples runner is "+example.Runner)
			}
			continue
		}
		if finrobotSourceHasNoProviderGuard(source) {
			continue
		}
		missing = append(missing, example.Path+" uses LLM/evaluate surface without replay records or an inline no-provider guard")
	}

	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf("FinRobot runnable examples missing explicit no-provider guard:\n%s", strings.Join(missing, "\n"))
	}
}

func finrobotNoProviderRecordedSources(t *testing.T, root string) map[string]string {
	t.Helper()
	path := filepath.Join(root, "examples", "ai", "finrobot_translation", "evaluation_harness", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read evaluation harness manifest: %v", err)
	}
	var manifest struct {
		RecordsInventory []struct {
			SourcePath        string `json:"source_path"`
			ReplayRecordsPath string `json:"replay_records_path"`
		} `json:"records_inventory"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode evaluation harness manifest: %v", err)
	}
	recorded := make(map[string]string, len(manifest.RecordsInventory))
	for _, record := range manifest.RecordsInventory {
		if record.SourcePath == "" || record.ReplayRecordsPath == "" {
			t.Fatalf("incomplete records inventory entry: %#v", record)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(record.ReplayRecordsPath))); err != nil {
			t.Fatalf("%s replay records missing: %v", record.SourcePath, err)
		}
		recorded[filepath.ToSlash(record.SourcePath)] = filepath.ToSlash(record.ReplayRecordsPath)
	}
	if len(recorded) == 0 {
		t.Fatal("evaluation harness records inventory is empty")
	}
	return recorded
}

func finrobotSourceHasNoProviderGuard(source string) bool {
	normalized := strings.ToLower(source)
	for _, guard := range []string{
		"provider_free",
		"provider-free",
		"provider free",
		"no provider",
		"without provider",
		"offline",
		"fixture",
		"replay",
		"live_network: false",
		"live_network=false",
		"live_model: false",
		"model_calls: false",
	} {
		if strings.Contains(normalized, guard) {
			return true
		}
	}
	return false
}
