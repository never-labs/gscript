package leia_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type finrobotTutorialParityMatrix struct {
	SchemaVersion   int    `json:"schema_version"`
	ID              string `json:"id"`
	SourceInventory struct {
		Repo          string   `json:"repo"`
		SourceCommit  string   `json:"source_commit"`
		TutorialRoots []string `json:"tutorial_roots"`
		Notebooks     []string `json:"notebooks"`
	} `json:"source_inventory"`
	Defaults struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		CredentialsRequired   bool `json:"credentials_required"`
		ModelRequired         bool `json:"model_required"`
		RealDependencyImports bool `json:"real_dependency_imports"`
	} `json:"defaults"`
	Rows []finrobotTutorialParityMatrixRow `json:"rows"`
}

type finrobotTutorialParityMatrixRow struct {
	ID     string `json:"id"`
	Level  string `json:"level"`
	Source struct {
		Module string `json:"module"`
		SHA256 string `json:"sha256"`
	} `json:"source"`
	ProviderFree          bool   `json:"provider_free"`
	LiveNetwork           bool   `json:"live_network"`
	CredentialsRequired   bool   `json:"credentials_required"`
	ModelRequired         bool   `json:"model_required"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	RunnableExample       string `json:"runnable_example"`
	ExpectedSummary       string `json:"expected_summary"`
	OptionalLiveGate      struct {
		ID                          string `json:"id"`
		DefaultEnabled              bool   `json:"default_enabled"`
		LiveNetworkDefault          bool   `json:"live_network_default"`
		RequiresCredentials         bool   `json:"requires_credentials"`
		CleanSkipWithoutCredentials bool   `json:"clean_skip_without_credentials"`
		SkipReason                  string `json:"skip_reason"`
	} `json:"optional_live_gate"`
	SkipReason        string   `json:"skip_reason"`
	ExpectedArtifacts []string `json:"expected_artifacts"`
}

func TestFinRobotTutorialParityMatrixRegistersAllSourceNotebooks(t *testing.T) {
	matrix := loadFinRobotTutorialParityMatrix(t)
	if matrix.SchemaVersion != 1 || matrix.ID != "finrobot-tutorial-parity-matrix" {
		t.Fatalf("matrix header = schema %d id %q", matrix.SchemaVersion, matrix.ID)
	}
	if matrix.SourceInventory.Repo != "FinRobot" || matrix.SourceInventory.SourceCommit == "" {
		t.Fatalf("source inventory = %#v", matrix.SourceInventory)
	}
	if !reflect.DeepEqual(sortedStrings(matrix.SourceInventory.TutorialRoots), []string{"tutorials_advanced", "tutorials_beginner"}) {
		t.Fatalf("tutorial roots = %#v", matrix.SourceInventory.TutorialRoots)
	}
	if !matrix.Defaults.ProviderFree || matrix.Defaults.LiveNetwork || matrix.Defaults.CredentialsRequired ||
		matrix.Defaults.ModelRequired || matrix.Defaults.RealDependencyImports {
		t.Fatalf("defaults must be provider-free and offline: %#v", matrix.Defaults)
	}

	wantInventory := []string{
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
	if got := sortedStrings(matrix.SourceInventory.Notebooks); !reflect.DeepEqual(got, wantInventory) {
		t.Fatalf("source inventory changed; every added/removed tutorial must be registered\ngot  %#v\nwant %#v", got, wantInventory)
	}

	rowSources := make([]string, 0, len(matrix.Rows))
	rowIDs := map[string]bool{}
	levels := map[string]int{}
	for _, row := range matrix.Rows {
		if rowIDs[row.ID] {
			t.Fatalf("duplicate row id %q", row.ID)
		}
		rowIDs[row.ID] = true
		rowSources = append(rowSources, row.Source.Module)
		levels[row.Level]++
	}
	if got := sortedStrings(rowSources); !reflect.DeepEqual(got, wantInventory) {
		t.Fatalf("matrix rows do not exactly match source inventory\ngot  %#v\nwant %#v", got, wantInventory)
	}
	if levels["beginner"] != 7 || levels["advanced"] != 6 {
		t.Fatalf("level counts = %#v, want beginner=7 advanced=6", levels)
	}

	assertExternalFinRobotInventoryIfPresent(t, matrix)
}

func TestFinRobotTutorialParityMatrixProviderFreeRowsAreOffline(t *testing.T) {
	root := repoRoot(t)
	matrix := loadFinRobotTutorialParityMatrix(t)
	shaPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

	for _, row := range matrix.Rows {
		if row.ID == "" || row.Source.Module == "" || row.RunnableExample == "" {
			t.Fatalf("incomplete matrix row: %#v", row)
		}
		if !strings.HasPrefix(row.Source.Module, "tutorials_beginner/") && !strings.HasPrefix(row.Source.Module, "tutorials_advanced/") {
			t.Fatalf("%s source outside tutorial roots: %s", row.ID, row.Source.Module)
		}
		if !shaPattern.MatchString(row.Source.SHA256) {
			t.Fatalf("%s source hash = %q", row.ID, row.Source.SHA256)
		}
		if !row.ProviderFree || row.LiveNetwork || row.CredentialsRequired || row.ModelRequired || row.RealDependencyImports {
			t.Fatalf("%s must be provider-free with no network, credentials, model, or real imports: %#v", row.ID, row)
		}
		if row.OptionalLiveGate.ID == "" || row.OptionalLiveGate.SkipReason == "" {
			t.Fatalf("%s missing optional live gate details: %#v", row.ID, row.OptionalLiveGate)
		}
		if row.OptionalLiveGate.DefaultEnabled || row.OptionalLiveGate.LiveNetworkDefault {
			t.Fatalf("%s optional live gate must be disabled by default: %#v", row.ID, row.OptionalLiveGate)
		}
		if !row.OptionalLiveGate.CleanSkipWithoutCredentials {
			t.Fatalf("%s optional live gate must cleanly skip when services or credentials are absent", row.ID)
		}
		if row.SkipReason == "" {
			t.Fatalf("%s missing provider-free skip reason", row.ID)
		}
		if len(row.ExpectedArtifacts) == 0 {
			t.Fatalf("%s missing expected artifacts", row.ID)
		}
		examplePath := filepath.Join(root, filepath.FromSlash(row.RunnableExample))
		if !strings.HasPrefix(row.RunnableExample, "examples/ai/finrobot_translation/tutorial_parity/runnable/") {
			t.Fatalf("%s runnable example outside tutorial parity root: %s", row.ID, row.RunnableExample)
		}
		source, err := os.ReadFile(examplePath)
		if err != nil {
			t.Fatalf("%s runnable example %s: %v", row.ID, row.RunnableExample, err)
		}
		assertProviderFreeRunnableSource(t, row.ID, string(source))
	}
}

func TestFinRobotTutorialParityMatrixRunnableExamplesExecute(t *testing.T) {
	matrix := loadFinRobotTutorialParityMatrix(t)
	for _, row := range matrix.Rows {
		row := row
		t.Run(row.ID, func(t *testing.T) {
			path := filepath.Join(repoRoot(t), filepath.FromSlash(row.RunnableExample))
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
					if len(prints) != 1 || prints[0] != row.ExpectedSummary {
						t.Fatalf("prints = %#v, want %q", prints, row.ExpectedSummary)
					}
				})
			}
		})
	}
}

func assertProviderFreeRunnableSource(t *testing.T, rowID, source string) {
	t.Helper()
	blockedLoaders := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*import\s+`),
		regexp.MustCompile(`(?m)^\s*use\s+`),
		regexp.MustCompile(`(?m)^\s*load\s*\(`),
		regexp.MustCompile(`(?m)^\s*require\s*\(`),
	}
	for _, loader := range blockedLoaders {
		if loader.FindString(source) != "" {
			t.Fatalf("%s runnable example contains live dependency loader matching %q", rowID, loader.String())
		}
	}
	for _, literal := range []string{
		"provider_free: true",
		"live_network: false",
		"credentials_required: false",
	} {
		if !strings.Contains(source, literal) {
			t.Fatalf("%s runnable example missing literal %q", rowID, literal)
		}
	}
	blockedProviderCalls := []string{
		"autogen", "openai", "anthropic", "ollama", "yfinance", "finnhub", "fmp", "openbb", "sec_api",
		"http.get", "http.post", "fetch", "curl",
	}
	for _, provider := range blockedProviderCalls {
		pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(provider) + `\s*[.(]`)
		if pattern.FindString(source) != "" {
			t.Fatalf("%s runnable example must not call provider or network symbol matching %q", rowID, pattern.String())
		}
	}
}

func assertExternalFinRobotInventoryIfPresent(t *testing.T, matrix finrobotTutorialParityMatrix) {
	t.Helper()
	root := repoRoot(t)
	externalRoot := filepath.Join(root, ".external", "FinRobot")
	if info, err := os.Stat(externalRoot); err != nil || !info.IsDir() {
		return
	}

	var actual []string
	for _, tutorialRoot := range matrix.SourceInventory.TutorialRoots {
		dir := filepath.Join(externalRoot, filepath.FromSlash(tutorialRoot))
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && path != dir {
				return filepath.SkipDir
			}
			if d.Type().IsRegular() && strings.HasSuffix(d.Name(), ".ipynb") {
				rel, err := filepath.Rel(externalRoot, path)
				if err != nil {
					return err
				}
				actual = append(actual, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan external FinRobot tutorials: %v", err)
		}
	}
	if got, want := sortedStrings(actual), sortedStrings(matrix.SourceInventory.Notebooks); !reflect.DeepEqual(got, want) {
		t.Fatalf("external FinRobot tutorials changed; update matrix registrations\ngot  %#v\nwant %#v", got, want)
	}

	rowsBySource := map[string]finrobotTutorialParityMatrixRow{}
	for _, row := range matrix.Rows {
		rowsBySource[row.Source.Module] = row
	}
	for _, source := range matrix.SourceInventory.Notebooks {
		data, err := os.ReadFile(filepath.Join(externalRoot, filepath.FromSlash(source)))
		if err != nil {
			t.Fatalf("read external tutorial %s: %v", source, err)
		}
		sum := sha256.Sum256(data)
		got := "sha256:" + hex.EncodeToString(sum[:])
		if want := rowsBySource[source].Source.SHA256; got != want {
			t.Fatalf("%s hash = %s, want %s", source, got, want)
		}
	}
}

func loadFinRobotTutorialParityMatrix(t *testing.T) finrobotTutorialParityMatrix {
	t.Helper()
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "tutorial_parity", "matrix.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var matrix finrobotTutorialParityMatrix
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode tutorial parity matrix: %v", err)
	}
	return matrix
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
