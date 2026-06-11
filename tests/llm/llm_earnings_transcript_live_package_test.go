package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type earningsTranscriptManifest struct {
	SchemaVersion               int                 `json:"schema_version"`
	ID                          string              `json:"id"`
	PackageName                 string              `json:"package_name"`
	ProviderFree                bool                `json:"provider_free"`
	LiveNetworkDefault          bool                `json:"live_network_default"`
	RealDependencyImportDefault bool                `json:"real_dependency_import_default"`
	SourceModules               []string            `json:"source_modules"`
	Entrypoints                 map[string]string   `json:"entrypoints"`
	Schemas                     map[string]string   `json:"schemas"`
	Fixtures                    map[string]string   `json:"fixtures"`
	NormalizerDomains           []string            `json:"normalizer_domains"`
	Capabilities                []string            `json:"capabilities"`
	BlockedImports              []string            `json:"blocked_imports"`
	FieldCanonicalization       map[string]string   `json:"field_canonicalization"`
	DateCorrectionPolicy        map[string]any      `json:"date_correction_policy"`
	ChunkingPolicy              map[string]any      `json:"chunking_policy"`
	DeterministicOrdering       map[string][]string `json:"deterministic_ordering"`
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
	ProviderGate struct {
		AllowNetwork         bool     `json:"allow_network"`
		RequiredCredentials  []string `json:"required_credentials"`
		OptionalCredentials  []string `json:"optional_credentials"`
		ProviderSDKsRequired bool     `json:"provider_sdks_required"`
		TestRule             string   `json:"test_rule"`
	} `json:"provider_gate"`
}

type earningsTranscriptSchema struct {
	SchemaVersion         int      `json:"schema_version"`
	ID                    string   `json:"id"`
	Domain                string   `json:"domain"`
	Required              []string `json:"required"`
	CanonicalFields       []string `json:"canonical_fields"`
	DeterministicOrder    []string `json:"deterministic_order"`
	MissingRequiredPolicy string   `json:"missing_required_policy"`
	MissingOptionalPolicy string   `json:"missing_optional_policy"`
	ProviderFree          bool     `json:"provider_free"`
}

func TestFinRobotEarningsTranscriptLivePackageManifest(t *testing.T) {
	base := earningsTranscriptLivePackageDir(t)
	manifest := loadEarningsTranscriptManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-earnings-transcript-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-earnings-transcript" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("earnings transcript skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}
	if manifest.ProviderGate.AllowNetwork || manifest.ProviderGate.ProviderSDKsRequired ||
		len(manifest.ProviderGate.RequiredCredentials) != 0 ||
		len(manifest.ProviderGate.OptionalCredentials) != 0 {
		t.Fatalf("provider gate must stay closed: %#v", manifest.ProviderGate)
	}

	wantDomains := []string{"speaker_cleaning", "date_correction", "quarter_year_lookup", "segment_provenance", "transcript_chunking", "http_clean_skip"}
	if strings.Join(manifest.NormalizerDomains, ",") != strings.Join(wantDomains, ",") {
		t.Fatalf("domains = %#v, want %#v", manifest.NormalizerDomains, wantDomains)
	}
	for _, key := range []string{"smoke", "contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertEarningsTranscriptJSONOrLeiaFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"request", "segment", "chunk", "skip"} {
		if manifest.Schemas[key] == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertEarningsTranscriptJSONFile(t, filepath.Join(base, manifest.Schemas[key]))
	}
	for _, key := range []string{"raw_transcript", "normalized_transcript", "chunks", "http_clean_skip"} {
		if manifest.Fixtures[key] == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertEarningsTranscriptJSONFile(t, filepath.Join(base, manifest.Fixtures[key]))
	}
	for _, want := range []string{
		"finance.earnings_transcript.speaker.clean",
		"finance.earnings_transcript.date.correct",
		"finance.earnings_transcript.period.lookup",
		"finance.earnings_transcript.segment.provenance",
		"finance.earnings_transcript.chunk",
		"finance.earnings_transcript.http_clean.skip",
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if manifest.FieldCanonicalization["case"] != "snake_case" ||
		manifest.DateCorrectionPolicy["warning_marker"] != "earnings_transcript_date_corrected" ||
		manifest.DateCorrectionPolicy["unknown_period_policy"] != "emit clean_skip with reason period_lookup_missing" ||
		manifest.ChunkingPolicy["preserve_segment_boundaries"] != true {
		t.Fatalf("canonicalization/date/chunking policy is too weak: fields=%#v date=%#v chunking=%#v", manifest.FieldCanonicalization, manifest.DateCorrectionPolicy, manifest.ChunkingPolicy)
	}
	for _, domain := range []string{"segments", "chunks", "skips"} {
		if len(manifest.DeterministicOrdering[domain]) == 0 {
			t.Fatalf("deterministic ordering missing for %q", domain)
		}
	}
}

func TestFinRobotEarningsTranscriptSchemasFixturesAndProvenance(t *testing.T) {
	base := earningsTranscriptLivePackageDir(t)
	manifest := loadEarningsTranscriptManifest(t, base)

	for key, schemaPath := range manifest.Schemas {
		var schema earningsTranscriptSchema
		decodeEarningsTranscriptJSONFile(t, filepath.Join(base, schemaPath), &schema)
		if schema.SchemaVersion != 1 || schema.ID == "" || !schema.ProviderFree {
			t.Fatalf("%s schema header incomplete: %#v", key, schema)
		}
		for _, want := range []string{"schema"} {
			if !contains(schema.Required, want) {
				t.Fatalf("%s schema required fields missing %q: %#v", key, want, schema.Required)
			}
		}
		if schema.MissingRequiredPolicy != "reject_record" || schema.MissingOptionalPolicy != "emit_null_with_missing_reason" {
			t.Fatalf("%s missing policies = %q/%q", key, schema.MissingRequiredPolicy, schema.MissingOptionalPolicy)
		}
		assertEarningsTranscriptSnakeCase(t, append(append([]string{}, schema.Required...), schema.CanonicalFields...))
	}

	var raw struct {
		ProviderFree      bool   `json:"provider_free"`
		RawEventDate      string `json:"raw_event_date"`
		CalendarEventDate string `json:"calendar_event_date"`
		FiscalYear        int    `json:"fiscal_year"`
		FiscalQuarter     string `json:"fiscal_quarter"`
		FiscalCalendar    struct {
			EarningsCallDate string `json:"earnings_call_date"`
		} `json:"fiscal_calendar"`
		RawLines []struct {
			Line    int    `json:"line"`
			Speaker string `json:"speaker"`
			Text    string `json:"text"`
		} `json:"raw_lines"`
	}
	decodeEarningsTranscriptJSONFile(t, filepath.Join(base, manifest.Fixtures["raw_transcript"]), &raw)
	if !raw.ProviderFree || raw.RawEventDate == raw.CalendarEventDate || raw.CalendarEventDate != raw.FiscalCalendar.EarningsCallDate {
		t.Fatalf("raw fixture must show corrected date lookup: %#v", raw)
	}
	if raw.FiscalYear != 2026 || raw.FiscalQuarter != "Q1" || len(raw.RawLines) < 4 {
		t.Fatalf("raw fixture period/lines incomplete: %#v", raw)
	}

	var normalized struct {
		ProviderFree       bool             `json:"provider_free"`
		Rows               []map[string]any `json:"rows"`
		DeterministicOrder []string         `json:"deterministic_order"`
	}
	decodeEarningsTranscriptJSONFile(t, filepath.Join(base, manifest.Fixtures["normalized_transcript"]), &normalized)
	if !normalized.ProviderFree || len(normalized.Rows) != len(raw.RawLines) {
		t.Fatalf("normalized fixture header/rows invalid: %#v", normalized)
	}
	for i, row := range normalized.Rows {
		for _, field := range []string{"segment_id", "symbol", "fiscal_year", "fiscal_quarter", "event_date", "speaker_raw", "speaker_clean", "source_id", "source_line_start", "source_line_end", "record_hash"} {
			if row[field] == nil || row[field] == "" {
				t.Fatalf("normalized row %d missing %s: %#v", i, field, row)
			}
		}
		if row["event_date"] != raw.CalendarEventDate || row["date_corrected"] != true || row["quarter_lookup_source"] != "fiscal_calendar_fixture" {
			t.Fatalf("normalized row %d did not carry date correction/quarter lookup: %#v", i, row)
		}
		if strings.Contains(earningsTranscriptStringKey(row["speaker_clean"]), ":") ||
			strings.Contains(earningsTranscriptStringKey(row["speaker_clean"]), " - ") ||
			strings.Contains(earningsTranscriptStringKey(row["speaker_clean"]), ", CFO") {
			t.Fatalf("speaker was not cleaned in row %d: %#v", i, row)
		}
		if row["source_line_start"].(float64) > row["source_line_end"].(float64) {
			t.Fatalf("row %d provenance line range invalid: %#v", i, row)
		}
	}
	if !earningsTranscriptRowsSorted(normalized.Rows, normalized.DeterministicOrder) {
		t.Fatalf("normalized rows are not sorted by %#v", normalized.DeterministicOrder)
	}
}

func TestFinRobotEarningsTranscriptChunkingAndCleanSkip(t *testing.T) {
	base := earningsTranscriptLivePackageDir(t)
	manifest := loadEarningsTranscriptManifest(t, base)

	var chunks struct {
		ProviderFree       bool             `json:"provider_free"`
		MaxTokenEstimate   float64          `json:"max_token_estimate"`
		Rows               []map[string]any `json:"rows"`
		DeterministicOrder []string         `json:"deterministic_order"`
	}
	decodeEarningsTranscriptJSONFile(t, filepath.Join(base, manifest.Fixtures["chunks"]), &chunks)
	if !chunks.ProviderFree || chunks.MaxTokenEstimate <= 0 || len(chunks.Rows) < 2 {
		t.Fatalf("chunk fixture invalid: %#v", chunks)
	}
	for i, row := range chunks.Rows {
		if row["preserve_segment_boundaries"] != true {
			t.Fatalf("chunk %d does not preserve segment boundaries: %#v", i, row)
		}
		if row["token_estimate"].(float64) > chunks.MaxTokenEstimate {
			t.Fatalf("chunk %d token estimate exceeds max: %#v", i, row)
		}
		if row["source_line_start"].(float64) > row["source_line_end"].(float64) {
			t.Fatalf("chunk %d provenance line range invalid: %#v", i, row)
		}
		if len(row["segment_ids"].([]any)) == 0 || len(row["provenance"].([]any)) == 0 {
			t.Fatalf("chunk %d missing segment provenance: %#v", i, row)
		}
	}
	if !earningsTranscriptRowsSorted(chunks.Rows, chunks.DeterministicOrder) {
		t.Fatalf("chunk rows are not sorted by %#v", chunks.DeterministicOrder)
	}

	var skip struct {
		ProviderFree                bool   `json:"provider_free"`
		CleanSkip                   bool   `json:"clean_skip"`
		Dependency                  string `json:"dependency"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		CapabilityRequired          string `json:"capability_required"`
		FallbackFixture             string `json:"fallback_fixture"`
	}
	decodeEarningsTranscriptJSONFile(t, filepath.Join(base, manifest.Fixtures["http_clean_skip"]), &skip)
	if !skip.ProviderFree || !skip.CleanSkip || skip.Dependency != "http_cleaner" ||
		skip.LiveNetwork || skip.ProviderCredentialsRequired ||
		skip.CapabilityRequired != "finance.earnings_transcript.http_clean.skip" ||
		skip.FallbackFixture == "" {
		t.Fatalf("HTTP clean skip fixture invalid: %#v", skip)
	}
}

func TestFinRobotEarningsTranscriptNoLiveProvidersAndSmoke(t *testing.T) {
	base := earningsTranscriptLivePackageDir(t)
	manifest := loadEarningsTranscriptManifest(t, base)

	var files []string
	files = append(files, filepath.Join(base, "package.manifest.json"))
	for _, rel := range manifest.Entrypoints {
		files = append(files, filepath.Join(base, rel))
	}
	for _, rel := range manifest.Schemas {
		files = append(files, filepath.Join(base, rel))
	}
	for _, rel := range manifest.Fixtures {
		files = append(files, filepath.Join(base, rel))
	}
	files = append(files, filepath.Join(base, "contracts", "earnings_transcript_contract.json"))

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, blocked := range manifest.BlockedImports {
			if strings.Contains(text, "import "+blocked) ||
				strings.Contains(text, `require("`+blocked+`"`) ||
				strings.Contains(text, "from "+blocked+" import") {
				t.Fatalf("%s appears to import blocked provider dependency %q", path, blocked)
			}
		}
		for _, networkMarker := range []string{"https://", "http://"} {
			if strings.Contains(text, networkMarker) {
				t.Fatalf("%s contains live network locator %q", path, networkMarker)
			}
		}
	}

	vm := leia.New()
	if err := vm.ExecFile(filepath.Join(base, "main.leia")); err != nil {
		t.Fatalf("ExecFile main.leia: %v", err)
	}
	got, err := vm.Get("earnings_transcript_live_package_summary")
	if err != nil {
		t.Fatalf("Get summary: %v", err)
	}
	if !strings.Contains(got.(string), "provider_free=true") || !strings.Contains(got.(string), "clean_skip=true") {
		t.Fatalf("summary = %#v", got)
	}
}

func earningsTranscriptLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "earnings_transcript")
}

func loadEarningsTranscriptManifest(t *testing.T, base string) earningsTranscriptManifest {
	t.Helper()
	var manifest earningsTranscriptManifest
	decodeEarningsTranscriptJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertEarningsTranscriptJSONOrLeiaFile(t *testing.T, path string) {
	t.Helper()
	if strings.HasSuffix(path, ".json") {
		assertEarningsTranscriptJSONFile(t, path)
		return
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertEarningsTranscriptJSONFile(t *testing.T, path string) {
	t.Helper()
	var v any
	decodeEarningsTranscriptJSONFile(t, path, &v)
}

func decodeEarningsTranscriptJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertEarningsTranscriptSnakeCase(t *testing.T, fields []string) {
	t.Helper()
	re := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for _, field := range fields {
		if !re.MatchString(field) {
			t.Fatalf("field %q is not snake_case", field)
		}
	}
}

func earningsTranscriptRowsSorted(rows []map[string]any, order []string) bool {
	if len(order) == 0 {
		return false
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		var parts []string
		for _, field := range order {
			parts = append(parts, earningsTranscriptStringKey(row[field]))
		}
		keys = append(keys, strings.Join(parts, "\x00"))
	}
	return sort.StringsAreSorted(keys)
}

func earningsTranscriptStringKey(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		data, _ := json.Marshal(value)
		return strings.TrimRight(strings.TrimRight(string(data), "0"), ".")
	default:
		return ""
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
