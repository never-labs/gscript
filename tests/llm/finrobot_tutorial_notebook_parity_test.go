package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type finrobotTutorialNotebookParityFixtureIndex struct {
	SchemaVersion         int                                     `json:"schema_version"`
	ID                    string                                  `json:"id"`
	ProviderFree          bool                                    `json:"provider_free"`
	LiveNetwork           bool                                    `json:"live_network"`
	CredentialsRequired   bool                                    `json:"credentials_required"`
	ModelRequired         bool                                    `json:"model_required"`
	RealDependencyImports bool                                    `json:"real_dependency_imports"`
	RegisteredExamples    []finrobotTutorialNotebookParityExample `json:"registered_examples"`
}

type finrobotTutorialNotebookParityExample struct {
	ID              string   `json:"id"`
	Path            string   `json:"path"`
	SourceNotebooks []string `json:"source_notebooks"`
	FixtureKey      string   `json:"fixture_key"`
	ExpectedSummary string   `json:"expected_summary"`
	Checkable       bool     `json:"checkable"`
	OptionalGate    struct {
		ID                      string `json:"id"`
		DefaultEnabled          bool   `json:"default_enabled"`
		LiveNetworkDefault      bool   `json:"live_network_default"`
		DependencyImported      bool   `json:"dependency_imported"`
		CleanSkipWithoutService bool   `json:"clean_skip_without_service"`
		StatusWithoutService    string `json:"status_without_service"`
	} `json:"optional_gate"`
}

func TestFinRobotTutorialNotebookParityFirstBatchRegistry(t *testing.T) {
	root := repoRoot(t)
	index := loadFinRobotTutorialNotebookParityFixtureIndex(t)

	if index.SchemaVersion != 1 || index.ID != "finrobot-tutorial-notebook-parity-first-batch-fixtures" {
		t.Fatalf("fixture index header = schema %d id %q", index.SchemaVersion, index.ID)
	}
	if !index.ProviderFree || index.LiveNetwork || index.CredentialsRequired || index.ModelRequired || index.RealDependencyImports {
		t.Fatalf("fixture index must require no provider, network, credentials, model, or real imports: %#v", index)
	}
	if len(index.RegisteredExamples) != 3 {
		t.Fatalf("registered examples = %d, want 3", len(index.RegisteredExamples))
	}

	wantIDs := []string{"annual_report", "ollama_function_call_optional_gate", "rag_earnings_sec"}
	var gotIDs []string
	seenSources := map[string]bool{}
	for _, example := range index.RegisteredExamples {
		gotIDs = append(gotIDs, example.ID)
		if !example.Checkable {
			t.Fatalf("%s must be checkable", example.ID)
		}
		if !strings.HasPrefix(example.Path, "examples/ai/finrobot_translation/tutorials/notebook_parity/") {
			t.Fatalf("%s path outside tutorial notebook parity root: %s", example.ID, example.Path)
		}
		if !strings.HasPrefix(example.FixtureKey, "tutorial:notebook:") || !strings.Contains(example.FixtureKey, ":provider_free:") {
			t.Fatalf("%s fixture key is not provider-free tutorial notebook key: %s", example.ID, example.FixtureKey)
		}
		if example.ExpectedSummary == "" {
			t.Fatalf("%s missing expected summary", example.ID)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(example.Path))); err != nil {
			t.Fatalf("%s registered example %s: %v", example.ID, example.Path, err)
		}
		if len(example.SourceNotebooks) == 0 {
			t.Fatalf("%s missing source notebooks", example.ID)
		}
		for _, source := range example.SourceNotebooks {
			if !strings.HasPrefix(source, "tutorials_beginner/") && !strings.HasPrefix(source, "tutorials_advanced/") {
				t.Fatalf("%s source notebook outside tutorial roots: %s", example.ID, source)
			}
			seenSources[source] = true
		}
	}
	sort.Strings(gotIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("registered ids = %#v, want %#v", gotIDs, wantIDs)
	}
	for _, want := range []string{
		"tutorials_beginner/agent_annual_report.ipynb",
		"tutorials_advanced/agent_annual_report.ipynb",
		"tutorials_beginner/agent_rag_earnings_call_sec_filings.ipynb",
		"tutorials_beginner/ollama function call.ipynb",
	} {
		if !seenSources[want] {
			t.Fatalf("missing notebook source %q", want)
		}
	}
}

func TestFinRobotTutorialNotebookParityNoLiveProviderOrModelRequirement(t *testing.T) {
	root := repoRoot(t)
	index := loadFinRobotTutorialNotebookParityFixtureIndex(t)

	blockedLoaders := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*import\s+`),
		regexp.MustCompile(`(?m)^\s*use\s+`),
		regexp.MustCompile(`(?m)^\s*load\s*\(`),
		regexp.MustCompile(`(?m)^\s*require\s*\(`),
	}
	blockedProviderCalls := []string{
		"autogen", "openai", "anthropic", "ollama", "yfinance", "finnhub", "fmp", "openbb", "sec_api",
	}

	for _, example := range index.RegisteredExamples {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(example.Path)))
		if err != nil {
			t.Fatalf("%s: %v", example.ID, err)
		}
		source := string(data)
		for _, loader := range blockedLoaders {
			if loader.FindString(source) != "" {
				t.Fatalf("%s contains live dependency loader matching %q", example.ID, loader.String())
			}
		}
		for _, provider := range blockedProviderCalls {
			pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(provider) + `\s*[.(]`)
			if pattern.FindString(source) != "" {
				t.Fatalf("%s must not call provider SDK symbol matching %q", example.ID, pattern.String())
			}
		}
		for _, requiredLiteral := range []string{
			"provider_free: true",
			"live_network: false",
			"credentials_required: false",
			"model_required: false",
			"real_dependency_imports: false",
		} {
			if !strings.Contains(source, requiredLiteral) {
				t.Fatalf("%s missing literal %q", example.ID, requiredLiteral)
			}
		}
	}

	ollama := findFinRobotTutorialNotebookParityExample(t, index, "ollama_function_call_optional_gate")
	if ollama.OptionalGate.ID != "ollama-local-function-call" ||
		ollama.OptionalGate.DefaultEnabled ||
		ollama.OptionalGate.LiveNetworkDefault ||
		ollama.OptionalGate.DependencyImported ||
		!ollama.OptionalGate.CleanSkipWithoutService ||
		ollama.OptionalGate.StatusWithoutService != "skipped" {
		t.Fatalf("ollama optional gate must be disabled and clean-skip safe: %#v", ollama.OptionalGate)
	}
}

func TestFinRobotTutorialNotebookParityExamplesExecute(t *testing.T) {
	index := loadFinRobotTutorialNotebookParityFixtureIndex(t)

	for _, example := range index.RegisteredExamples {
		example := example
		t.Run(example.ID, func(t *testing.T) {
			path := filepath.Join(repoRoot(t), filepath.FromSlash(example.Path))
			for _, tc := range []struct {
				name string
				opts []leia.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var prints []string
					vm := leia.New(append([]leia.Option{
						leia.WithLibs(leia.LibString),
						leia.WithPrint(func(args ...any) {
							var parts []string
							for _, arg := range args {
								parts = append(parts, fmt.Sprint(arg))
							}
							prints = append(prints, strings.Join(parts, " "))
						}),
					}, tc.opts...)...)

					if err := vm.ExecFile(path); err != nil {
						t.Fatalf("ExecFile: %v", err)
					}
					if len(prints) != 1 || prints[0] != example.ExpectedSummary {
						t.Fatalf("prints = %#v, want %q", prints, example.ExpectedSummary)
					}
				})
			}
		})
	}
}

func loadFinRobotTutorialNotebookParityFixtureIndex(t *testing.T) finrobotTutorialNotebookParityFixtureIndex {
	t.Helper()
	path := filepath.Join(
		repoRoot(t),
		"examples", "ai", "finrobot_translation", "tutorials", "notebook_parity", "fixtures", "provider_free_fixture_index.json",
	)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var index finrobotTutorialNotebookParityFixtureIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("decode tutorial notebook parity fixture index: %v", err)
	}
	return index
}

func findFinRobotTutorialNotebookParityExample(t *testing.T, index finrobotTutorialNotebookParityFixtureIndex, id string) finrobotTutorialNotebookParityExample {
	t.Helper()
	for _, example := range index.RegisteredExamples {
		if example.ID == id {
			return example
		}
	}
	t.Fatalf("missing registered example %q", id)
	return finrobotTutorialNotebookParityExample{}
}
