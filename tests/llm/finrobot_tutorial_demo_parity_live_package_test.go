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

type tutorialDemoParityLiveManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	SourceExamples              []string `json:"source_examples"`
	CoveredSourceRoots          []string `json:"covered_source_roots"`
	CoveredSourceFiles          []string `json:"covered_source_files"`
	Credentials                 struct {
		Required          []string `json:"required"`
		Optional          []string `json:"optional"`
		SecretEnvPatterns []string `json:"secret_env_patterns"`
		Policy            string   `json:"policy"`
	} `json:"credentials"`
	DefaultPolicy struct {
		Mode                        string `json:"mode"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints        map[string]string `json:"entrypoints"`
	Schemas            map[string]string `json:"schemas"`
	LedgerLinks        []ledgerLink      `json:"ledger_links"`
	RegisteredExamples []struct {
		ID       string `json:"id"`
		Path     string `json:"path"`
		Contract string `json:"contract"`
	} `json:"registered_examples"`
	ReplayRecordLinks  []replayRecordLink `json:"replay_record_links"`
	DemoSmokeContracts []struct {
		ID                    string   `json:"id"`
		SourcePath            string   `json:"source_path"`
		MappedExamples        []string `json:"mapped_examples"`
		RequiresLiveNetwork   bool     `json:"requires_live_network"`
		ImportsRealDependency bool     `json:"imports_real_dependency"`
		ExpectedStatus        string   `json:"expected_status"`
	} `json:"demo_smoke_contracts"`
	TestGates []string `json:"test_gates"`
}

type ledgerLink struct {
	ID              string   `json:"id"`
	Path            string   `json:"path"`
	ExpectedEntries int      `json:"expected_entries"`
	RequiredRoots   []string `json:"required_roots"`
	RequiredSources []string `json:"required_sources"`
}

type replayRecordLink struct {
	ManifestRecordID  string `json:"manifest_record_id"`
	SourcePath        string `json:"source_path"`
	ReplayRecordsPath string `json:"replay_records_path"`
}

func TestFinRobotTutorialDemoParityLivePackageManifestAndLedgerCompleteness(t *testing.T) {
	root := repoRoot(t)
	base := tutorialDemoParityLivePackageDir(t)
	manifest := loadTutorialDemoParityLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-tutorial-demo-parity-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-tutorial-demo-parity" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	if !reflect.DeepEqual(sortedCopy(manifest.CoveredSourceRoots), []string{"tutorials_advanced", "tutorials_beginner"}) {
		t.Fatalf("covered roots = %#v", manifest.CoveredSourceRoots)
	}
	if !reflect.DeepEqual(sortedCopy(manifest.CoveredSourceFiles), []string{"agent_builder_demo.py", "test_module.py"}) {
		t.Fatalf("covered source files = %#v", manifest.CoveredSourceFiles)
	}

	for _, source := range manifest.SourceExamples {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(source))); err != nil {
			t.Fatalf("source example %q: %v", source, err)
		}
	}
	for _, key := range []string{"contract", "optional_gates", "fixture_index", "replay_records", "conversion_checklist", "smoke"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
	}
	for _, key := range []string{"contract", "optional_gate", "fixture_index", "conversion_checklist"} {
		assertTutorialDemoJSONFile(t, filepath.Join(base, manifest.Schemas[key]))
	}

	linksByID := map[string]ledgerLink{}
	for _, link := range manifest.LedgerLinks {
		linksByID[link.ID] = link
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(link.Path))); err != nil {
			t.Fatalf("ledger link %s path %s: %v", link.ID, link.Path, err)
		}
	}
	tutorialLink := linksByID["tutorial_parity"]
	demoLink := linksByID["demo_parity"]
	tutorialLedger := loadFinRobotTutorialParityLedger(t)
	demoLedger := loadFinRobotDemoParityLedger(t)
	if tutorialLink.ExpectedEntries != len(tutorialLedger.Tutorials) || tutorialLink.ExpectedEntries != 13 {
		t.Fatalf("tutorial parity entries = link:%d ledger:%d", tutorialLink.ExpectedEntries, len(tutorialLedger.Tutorials))
	}
	if demoLink.ExpectedEntries != len(demoLedger.Sources) || demoLink.ExpectedEntries != 5 {
		t.Fatalf("demo parity entries = link:%d ledger:%d", demoLink.ExpectedEntries, len(demoLedger.Sources))
	}
	if !reflect.DeepEqual(sortedCopy(tutorialLedger.Source.TutorialRoots), sortedCopy(tutorialLink.RequiredRoots)) {
		t.Fatalf("tutorial roots = ledger:%#v link:%#v", tutorialLedger.Source.TutorialRoots, tutorialLink.RequiredRoots)
	}
	demoSources := map[string]bool{}
	for _, source := range demoLedger.Sources {
		demoSources[source.SourcePath] = true
	}
	for _, want := range demoLink.RequiredSources {
		if !demoSources[want] {
			t.Fatalf("demo ledger missing required source %q", want)
		}
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"tutorial parity ledger", "demo parity ledger", "registered", "replay record", "optional gates", "conversion checklist"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotTutorialDemoParityRegisteredExamplesAndReplayLinks(t *testing.T) {
	root := repoRoot(t)
	base := tutorialDemoParityLivePackageDir(t)
	manifest := loadTutorialDemoParityLiveManifest(t, base)
	if len(manifest.RegisteredExamples) != 1 {
		t.Fatalf("registered examples = %d, want 1", len(manifest.RegisteredExamples))
	}
	for _, example := range manifest.RegisteredExamples {
		if example.ID == "" || example.Contract == "" {
			t.Fatalf("registered example metadata incomplete: %#v", example)
		}
		if !strings.HasPrefix(example.Path, "examples/ai/finrobot_translation/live_packages/tutorial_demo_parity/") {
			t.Fatalf("registered example outside package: %s", example.Path)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(example.Path))); err != nil {
			t.Fatalf("registered example %s: %v", example.Path, err)
		}
	}

	manifestRecords := loadFinRobotEvaluationHarnessManifest(t, root)
	recordsByID := map[string]replayRecordLink{}
	for _, record := range manifestRecords.RecordsInventory {
		recordsByID[record.ID] = replayRecordLink{
			ManifestRecordID:  record.ID,
			SourcePath:        filepath.ToSlash(record.SourcePath),
			ReplayRecordsPath: filepath.ToSlash(record.ReplayRecordsPath),
		}
	}
	if len(manifest.ReplayRecordLinks) < 4 {
		t.Fatalf("replay record links = %d, want at least 4", len(manifest.ReplayRecordLinks))
	}
	for _, link := range manifest.ReplayRecordLinks {
		record, ok := recordsByID[link.ManifestRecordID]
		if !ok {
			t.Fatalf("unknown replay manifest record %q", link.ManifestRecordID)
		}
		if link.SourcePath != record.SourcePath || link.ReplayRecordsPath != record.ReplayRecordsPath {
			t.Fatalf("record %s mismatch: link=(%s,%s) manifest=(%s,%s)",
				link.ManifestRecordID, link.SourcePath, link.ReplayRecordsPath, record.SourcePath, record.ReplayRecordsPath)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(link.ReplayRecordsPath))); err != nil {
			t.Fatalf("replay records %s: %v", link.ReplayRecordsPath, err)
		}
	}

	var replayFixture struct {
		ProviderFree    bool               `json:"provider_free"`
		RecordsManifest string             `json:"records_manifest"`
		Records         []replayRecordLink `json:"records"`
	}
	decodeTutorialDemoJSONFile(t, filepath.Join(base, "fixtures", "provider_free_replay_records.json"), &replayFixture)
	if !replayFixture.ProviderFree || replayFixture.RecordsManifest != "examples/ai/finrobot_translation/evaluation_harness/manifest.json" {
		t.Fatalf("replay fixture header = %#v", replayFixture)
	}
	if len(replayFixture.Records) != len(manifest.ReplayRecordLinks) {
		t.Fatalf("replay fixture records = %d, manifest links = %d", len(replayFixture.Records), len(manifest.ReplayRecordLinks))
	}
}

func TestFinRobotTutorialDemoParityOptionalGatesAndSmoke(t *testing.T) {
	base := tutorialDemoParityLivePackageDir(t)
	var gates struct {
		ProviderFree               bool `json:"provider_free"`
		LiveNetwork                bool `json:"live_network"`
		RealDependencyImports      bool `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool `json:"clean_skip_without_dependency"`
		Gates                      []struct {
			ID                          string   `json:"id"`
			Scope                       string   `json:"scope"`
			Capabilities                []string `json:"capabilities"`
			LiveNetworkDefault          bool     `json:"live_network_default"`
			DefaultEnabled              bool     `json:"default_enabled"`
			RequiresCredentials         bool     `json:"requires_credentials"`
			DependencyImported          bool     `json:"dependency_imported"`
			CleanSkipWithoutCredentials bool     `json:"clean_skip_without_credentials"`
			StatusWithoutCredentials    string   `json:"status_without_credentials"`
			FixtureKey                  string   `json:"fixture_key"`
		} `json:"gates"`
	}
	decodeTutorialDemoJSONFile(t, filepath.Join(base, "contracts", "optional_live_gates.json"), &gates)
	if !gates.ProviderFree || gates.LiveNetwork || gates.RealDependencyImports || !gates.CleanSkipWithoutDependency {
		t.Fatalf("gate contract header must stay provider-free and clean-skip safe: %#v", gates)
	}
	if len(gates.Gates) != 3 {
		t.Fatalf("gates = %d, want 3", len(gates.Gates))
	}
	ids := map[string]bool{}
	for _, gate := range gates.Gates {
		if gate.ID == "" || gate.Scope == "" || gate.FixtureKey == "" || len(gate.Capabilities) == 0 {
			t.Fatalf("gate metadata incomplete: %#v", gate)
		}
		if ids[gate.ID] {
			t.Fatalf("duplicate gate id %q", gate.ID)
		}
		ids[gate.ID] = true
		if gate.LiveNetworkDefault || gate.DefaultEnabled || gate.DependencyImported || !gate.CleanSkipWithoutCredentials || gate.StatusWithoutCredentials != "skipped" {
			t.Fatalf("gate must be disabled, no-import, and clean-skip safe: %#v", gate)
		}
	}

	var fixtureIndex struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeTutorialDemoJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &fixtureIndex)
	if !fixtureIndex.ProviderFree || fixtureIndex.LiveNetwork || fixtureIndex.RealDependencyImports || len(fixtureIndex.Fixtures) != 3 {
		t.Fatalf("fixture index header/count = %#v", fixtureIndex)
	}
	for _, fixture := range fixtureIndex.Fixtures {
		if fixture.FixtureKey == "" || fixture.Metadata["replay_ready"] != true {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
	}

	var checklist struct {
		ProviderFree                            bool `json:"provider_free"`
		RequiredBeforeLivePackageImplementation bool `json:"required_before_live_package_implementation"`
		Items                                   []struct {
			ID       string `json:"id"`
			Required bool   `json:"required"`
		} `json:"items"`
	}
	decodeTutorialDemoJSONFile(t, filepath.Join(base, "fixtures", "notebook_to_leia_conversion_checklist.json"), &checklist)
	if !checklist.ProviderFree || !checklist.RequiredBeforeLivePackageImplementation || len(checklist.Items) < 6 {
		t.Fatalf("conversion checklist incomplete: %#v", checklist)
	}
	for _, item := range checklist.Items {
		if item.ID == "" || !item.Required {
			t.Fatalf("conversion checklist item must be required: %#v", item)
		}
	}

	data, err := os.ReadFile(filepath.Join(base, "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}
	for _, provider := range []string{"autogen", "backtrader", "yfinance", "openbb", "mplfinance", "fingpt"} {
		pattern := `(?m)^\s*` + regexp.QuoteMeta(provider) + `\s*[.(]`
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia must not call provider SDK symbol matching %q", pattern)
		}
	}
}

func TestFinRobotTutorialDemoParityExecutableSkeleton(t *testing.T) {
	path := filepath.Join(tutorialDemoParityLivePackageDir(t), "main.leia")
	want := "tutorial_demo_parity_live_package tutorials=13 demos=5 records=4 gates=3 clean_skip=3 live_network=false imports=false"

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
			got, err := vm.Get("tutorial_demo_parity_summary")
			if err != nil {
				t.Fatalf("Get tutorial_demo_parity_summary: %v", err)
			}
			if got != want {
				t.Fatalf("tutorial_demo_parity_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func tutorialDemoParityLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "tutorial_demo_parity")
}

func loadTutorialDemoParityLiveManifest(t *testing.T, base string) tutorialDemoParityLiveManifest {
	t.Helper()
	var manifest tutorialDemoParityLiveManifest
	decodeTutorialDemoJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertTutorialDemoJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeTutorialDemoJSONFile(t, path, &value)
}

func decodeTutorialDemoJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func sortedTutorialDemoKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
