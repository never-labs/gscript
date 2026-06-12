package leia_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericRecordReplayManifest struct {
	SchemaVersion               int    `json:"schema_version"`
	ID                          string `json:"id"`
	PackageName                 string `json:"package_name"`
	ProviderFree                bool   `json:"provider_free"`
	DomainSpecific              bool   `json:"domain_specific"`
	LiveNetworkDefault          bool   `json:"live_network_default"`
	RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
	Credentials                 struct {
		Required          []string `json:"required"`
		Optional          []string `json:"optional"`
		SecretEnvPatterns []string `json:"secret_env_patterns"`
		Policy            string   `json:"policy"`
	} `json:"credentials"`
	DefaultPolicy struct {
		Mode                  string `json:"mode"`
		Matching              string `json:"matching"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		FixtureHook           string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints        map[string]string `json:"entrypoints"`
	Schemas            map[string]string `json:"schemas"`
	Fixtures           map[string]string `json:"fixtures"`
	Capabilities       []string          `json:"capabilities"`
	DialectSurface     []string          `json:"dialect_surface"`
	TestGates          []string          `json:"test_gates"`
	NoBuiltInGuarantee struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
}

type genericReplayRecordFixture struct {
	SchemaVersion         int                   `json:"schema_version"`
	FixtureID             string                `json:"fixture_id"`
	ProviderFree          bool                  `json:"provider_free"`
	DomainSpecific        bool                  `json:"domain_specific"`
	LiveNetwork           bool                  `json:"live_network"`
	RealDependencyImports bool                  `json:"real_dependency_imports"`
	Records               []genericReplayRecord `json:"records"`
}

type genericReplayRecord struct {
	RecordID     string `json:"record_id"`
	Sequence     int    `json:"sequence"`
	ReplayKey    string `json:"replay_key"`
	Operation    string `json:"operation"`
	Capability   string `json:"capability"`
	RequestHash  string `json:"request_hash"`
	ResponseHash string `json:"response_hash"`
	ProviderFree bool   `json:"provider_free"`
	RecordedAt   string `json:"recorded_at"`
	Metadata     struct {
		Fixture               bool `json:"fixture"`
		Deterministic         bool `json:"deterministic"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
	} `json:"metadata"`
}

type genericReplayRequestFixture struct {
	SchemaVersion  int                    `json:"schema_version"`
	FixtureID      string                 `json:"fixture_id"`
	ProviderFree   bool                   `json:"provider_free"`
	DomainSpecific bool                   `json:"domain_specific"`
	Requests       []genericReplayRequest `json:"requests"`
}

type genericReplayRequest struct {
	ReplayKey   string `json:"replay_key"`
	Operation   string `json:"operation"`
	Capability  string `json:"capability"`
	RequestHash string `json:"request_hash"`
}

type genericReplayIndex struct {
	SchemaVersion   int               `json:"schema_version"`
	FixtureID       string            `json:"fixture_id"`
	ProviderFree    bool              `json:"provider_free"`
	DomainSpecific  bool              `json:"domain_specific"`
	Strategy        string            `json:"strategy"`
	RecordSchema    string            `json:"record_schema"`
	RecordsPath     string            `json:"records_path"`
	RequestFixtures map[string]string `json:"request_fixtures"`
	Matching        struct {
		ScanAhead             bool     `json:"scan_ahead"`
		ConsumeOnMatch        bool     `json:"consume_on_match"`
		ConsumeOnMismatch     bool     `json:"consume_on_mismatch"`
		IdentityFields        []string `json:"identity_fields"`
		MismatchFindingKind   string   `json:"mismatch_finding_kind"`
		UnconsumedFindingKind string   `json:"unconsumed_finding_kind"`
		ExhaustedFindingKind  string   `json:"exhausted_finding_kind"`
	} `json:"matching"`
	Fixtures []struct {
		ID                    string         `json:"id"`
		Path                  string         `json:"path"`
		Schema                string         `json:"schema"`
		Records               int            `json:"records"`
		Metadata              map[string]any `json:"metadata"`
		ProviderFree          bool           `json:"provider_free"`
		LiveNetwork           bool           `json:"live_network"`
		RealDependencyImports bool           `json:"real_dependency_imports"`
	} `json:"fixtures"`
	ExpectedSummaries         map[string]genericReplaySummary `json:"expected_summaries"`
	DeterministicSummaryOrder []string                        `json:"deterministic_summary_order"`
	LiveNetwork               bool                            `json:"live_network"`
	RealDependencyImports     bool                            `json:"real_dependency_imports"`
}

type genericReplaySummary struct {
	FixtureID        string   `json:"fixture_id"`
	Strategy         string   `json:"strategy"`
	LoadedRecords    int      `json:"loaded_records"`
	Requests         int      `json:"requests"`
	Matched          int      `json:"matched"`
	Mismatches       int      `json:"mismatches"`
	Unconsumed       int      `json:"unconsumed"`
	Exhausted        int      `json:"exhausted"`
	NextIndex        int      `json:"next_index"`
	FindingKinds     []string `json:"finding_kinds"`
	MatchedRecordIDs []string `json:"matched_record_ids"`
}

type genericReplayFinding struct {
	Kind     string                `json:"kind"`
	Cursor   int                   `json:"cursor"`
	RecordID string                `json:"record_id,omitempty"`
	Expected genericReplayIdentity `json:"expected,omitempty"`
	Actual   genericReplayIdentity `json:"actual,omitempty"`
	Message  string                `json:"message"`
}

type genericReplayIdentity struct {
	Operation   string `json:"operation"`
	Capability  string `json:"capability"`
	ReplayKey   string `json:"replay_key"`
	RequestHash string `json:"request_hash"`
}

func TestGenericRecordReplayLivePackageManifest(t *testing.T) {
	base := genericRecordReplayLivePackageDir(t)
	manifest := loadGenericRecordReplayManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-record-replay-live-package" ||
		manifest.PackageName != "leia-generic-ai-record-replay" {
		t.Fatalf("unexpected manifest header: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific ||
		manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("manifest is not provider-free/generic: %#v", manifest)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 ||
		len(manifest.Credentials.SecretEnvPatterns) != 0 ||
		!strings.Contains(manifest.Credentials.Policy, "provider-specific packages") {
		t.Fatalf("credential policy should stay empty and provider-free: %#v", manifest.Credentials)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.Matching != "strict_ordered" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_ai_replay_fixture" {
		t.Fatalf("default policy = %#v", manifest.DefaultPolicy)
	}
	for _, key := range []string{
		"smoke",
		"record_replay_contract",
		"strict_ordered_matching_contract",
		"fixture_index",
		"ordered_records_fixture",
		"matching_requests_fixture",
		"mismatch_requests_fixture",
	} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericRecordReplayPath(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"record", "replay_index", "match_finding", "matching_summary"} {
		if manifest.Schemas[key] == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertGenericRecordReplayPath(t, filepath.Join(base, manifest.Schemas[key]))
	}
	for _, key := range []string{"index", "ordered_records", "matching_requests", "mismatch_requests"} {
		if manifest.Fixtures[key] == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertGenericRecordReplayPath(t, filepath.Join(base, manifest.Fixtures[key]))
	}
	for _, want := range []string{
		"generic.ai.record.schema",
		"generic.ai.replay.index",
		"generic.ai.replay.match.strict_ordered",
		"generic.ai.replay.finding.mismatch",
		"generic.ai.replay.finding.unconsumed",
		"generic.ai.replay.summary.deterministic",
	} {
		if !containsGenericRecordReplay(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	for _, want := range []string{"generic.ai.record.replay", "generic.ai.record", "generic.ai.replay"} {
		if !containsGenericRecordReplay(manifest.DialectSurface, want) {
			t.Fatalf("dialect surface missing %q: %#v", want, manifest.DialectSurface)
		}
	}
	if !manifest.NoBuiltInGuarantee.Required || !strings.Contains(manifest.NoBuiltInGuarantee.Statement, "does not provide") {
		t.Fatalf("generic record replay package must declare no built-in guarantee: %#v", manifest.NoBuiltInGuarantee)
	}
	gates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"record schema", "strict ordered", "mismatch", "unconsumed", "fixture index", "deterministic summary"} {
		if !strings.Contains(gates, want) {
			t.Fatalf("test gates missing %q: %s", want, gates)
		}
	}
}

func TestGenericRecordReplaySmokeExecutes(t *testing.T) {
	base := genericRecordReplayLivePackageDir(t)
	mainPath := filepath.Join(base, "main.leia")
	sourceBytes, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, forbidden := range []string{"import q", "q/runtime", "$`", "$!`", "openai", "anthropic", "provider_sdk"} {
		if strings.Contains(strings.ToLower(source), forbidden) {
			t.Fatalf("main.leia must stay provider-free; found %q", forbidden)
		}
	}

	want := "generic_record_replay_live_package strategy=strict_ordered records=3 partial_matched=2 partial_unconsumed=1 mismatch_matched=1 mismatches=1 mismatch_unconsumed=2 next_index=1 findings=3 provider_free=true domain_specific=false live_network=false imports=false"
	for _, result := range runFinRobotLivePackageSummarySmoke(t, mainPath, "generic_record_replay_live_package_summary", "generic_record_replay_live_package", leia.LibAll) {
		if result.Summary != want {
			t.Fatalf("summary = %#v, want %#v", result.Summary, want)
		}
		fields := result.Fields
		requireFinRobotSummaryFields(t, fields, "strategy", "records", "partial_matched", "partial_unconsumed", "mismatch_matched", "mismatches", "mismatch_unconsumed", "next_index", "findings", "provider_free", "domain_specific", "live_network", "imports")
		if fields["strategy"] != "strict_ordered" ||
			fields["records"] != "3" ||
			fields["partial_matched"] != "2" ||
			fields["partial_unconsumed"] != "1" ||
			fields["mismatch_matched"] != "1" ||
			fields["mismatches"] != "1" ||
			fields["mismatch_unconsumed"] != "2" ||
			fields["next_index"] != "1" ||
			fields["findings"] != "3" ||
			fields["provider_free"] != "true" ||
			fields["domain_specific"] != "false" ||
			fields["live_network"] != "false" ||
			fields["imports"] != "false" {
			t.Fatalf("summary fields = %#v", fields)
		}
	}
}

func TestGenericRecordReplaySchemaAndFixtureIndex(t *testing.T) {
	base := genericRecordReplayLivePackageDir(t)

	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "record_v1.schema.json"), []string{"record_id", "sequence", "replay_key", "operation", "capability", "request_hash", "response_hash", "request", "response", "provider_free", "recorded_at"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "record_v1.schema.json"), []string{"properties", "request"}, []string{"canonical_json", "redactions"})
	assertDocumentPipelineNestedSchemaRequired(t, filepath.Join(base, "schemas", "record_v1.schema.json"), []string{"properties", "response"}, []string{"canonical_json", "status"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "replay_index_v1.schema.json"), []string{"schema_version", "fixture_id", "provider_free", "domain_specific", "strategy", "record_schema", "records_path", "request_fixtures", "expected_summaries"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "match_finding_v1.schema.json"), []string{"kind", "cursor", "message", "provider_free"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "matching_summary_v1.schema.json"), []string{"fixture_id", "strategy", "loaded_records", "requests", "matched", "mismatches", "unconsumed", "exhausted", "next_index", "finding_kinds", "matched_record_ids"})

	var recordSchema struct {
		AdditionalProperties bool                       `json:"additionalProperties"`
		Required             []string                   `json:"required"`
		Properties           map[string]json.RawMessage `json:"properties"`
	}
	decodeGenericRecordReplayJSON(t, filepath.Join(base, "schemas", "record_v1.schema.json"), &recordSchema)
	if recordSchema.AdditionalProperties {
		t.Fatal("record schema must forbid undeclared top-level provider fields")
	}
	for _, want := range []string{
		"record_id",
		"sequence",
		"replay_key",
		"operation",
		"capability",
		"request_hash",
		"response_hash",
		"request",
		"response",
		"provider_free",
		"recorded_at",
	} {
		if !containsGenericRecordReplay(recordSchema.Required, want) {
			t.Fatalf("record schema missing required field %q: %#v", want, recordSchema.Required)
		}
	}
	for _, forbidden := range []string{"provider", "provider_sdk", "api_key", "credential", "network_client", "live_endpoint"} {
		if _, ok := recordSchema.Properties[forbidden]; ok {
			t.Fatalf("record schema must not declare provider-specific field %q", forbidden)
		}
	}

	index := loadGenericReplayIndex(t, base)
	if index.SchemaVersion != 1 || index.FixtureID != "generic-ai-record-replay-fixture-index" ||
		!index.ProviderFree || index.DomainSpecific || index.Strategy != "strict_ordered" ||
		index.RecordSchema != "schemas/record_v1.schema.json" ||
		index.RecordsPath != "fixtures/ordered_records_fixture.json#/records" ||
		index.LiveNetwork || index.RealDependencyImports {
		t.Fatalf("unexpected replay index header: %#v", index)
	}
	for _, key := range []string{"strict_ordered_partial", "strict_ordered_mismatch"} {
		if index.RequestFixtures[key] == "" {
			t.Fatalf("request fixture %q missing: %#v", key, index.RequestFixtures)
		}
	}
	if index.Matching.ScanAhead || !index.Matching.ConsumeOnMatch || index.Matching.ConsumeOnMismatch ||
		index.Matching.MismatchFindingKind != "generic.ai.replay.mismatch" ||
		index.Matching.UnconsumedFindingKind != "generic.ai.replay.unconsumed_record" ||
		index.Matching.ExhaustedFindingKind != "generic.ai.replay.exhausted" {
		t.Fatalf("strict ordered matching policy = %#v", index.Matching)
	}
	wantIdentity := []string{"operation", "capability", "replay_key", "request_hash"}
	if !reflect.DeepEqual(index.Matching.IdentityFields, wantIdentity) {
		t.Fatalf("identity fields = %#v, want %#v", index.Matching.IdentityFields, wantIdentity)
	}
	wantOrder := []string{"fixture_id", "strategy", "loaded_records", "requests", "matched", "mismatches", "unconsumed", "exhausted", "next_index", "finding_kinds", "matched_record_ids"}
	if !reflect.DeepEqual(index.DeterministicSummaryOrder, wantOrder) {
		t.Fatalf("deterministic summary order = %#v", index.DeterministicSummaryOrder)
	}
	wantFixtures := map[string]struct {
		path    string
		schema  string
		records int
	}{
		"ordered_records": {
			path:    "fixtures/ordered_records_fixture.json",
			schema:  "schemas/record_v1.schema.json",
			records: 3,
		},
		"matching_requests": {
			path:    "fixtures/matching_requests_fixture.json",
			records: 2,
		},
		"mismatch_requests": {
			path:    "fixtures/mismatch_requests_fixture.json",
			records: 2,
		},
	}
	if len(index.Fixtures) != len(wantFixtures) {
		t.Fatalf("fixture index entries = %d, want %d", len(index.Fixtures), len(wantFixtures))
	}
	for _, fixture := range index.Fixtures {
		expected, ok := wantFixtures[fixture.ID]
		if !ok {
			t.Fatalf("unexpected fixture index entry %q", fixture.ID)
		}
		if fixture.Path != expected.path || fixture.Schema != expected.schema || fixture.Records != expected.records ||
			!fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports ||
			fixture.Metadata["replay_ready"] != true ||
			fixture.Metadata["provider_free"] != true ||
			fixture.Metadata["live_network"] != false ||
			fixture.Metadata["real_dependency_imports"] != false {
			t.Fatalf("fixture index entry drifted: %#v", fixture)
		}
		assertGenericRecordReplayPath(t, filepath.Join(base, fixture.Path))
		if fixture.Schema != "" {
			assertGenericRecordReplayPath(t, filepath.Join(base, fixture.Schema))
		}
		delete(wantFixtures, fixture.ID)
	}
	if len(wantFixtures) != 0 {
		t.Fatalf("fixture index missing entries: %#v", wantFixtures)
	}
}

func TestGenericRecordReplayCanonicalHashContract(t *testing.T) {
	base := genericRecordReplayLivePackageDir(t)

	var contract struct {
		DeterministicSummary struct {
			HashAlgorithm string `json:"hash_algorithm"`
			StableJSON    bool   `json:"stable_json"`
		} `json:"deterministic_summary"`
	}
	decodeGenericRecordReplayJSON(t, filepath.Join(base, "contracts", "record_replay_contract.json"), &contract)
	if contract.DeterministicSummary.HashAlgorithm != "sha256" || !contract.DeterministicSummary.StableJSON {
		t.Fatalf("canonical hash contract must stay sha256 stable JSON: %#v", contract.DeterministicSummary)
	}

	request := map[string]any{
		"messages": []any{
			map[string]any{"role": "system", "content": "Use fixture data only."},
			map[string]any{"role": "user", "content": "Summarize the row."},
		},
		"tools": []any{
			map[string]any{
				"name": "fixture.lookup",
				"schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"symbol": map[string]any{"type": "string"}},
				},
			},
		},
	}
	reordered := map[string]any{
		"tools": request["tools"],
		"messages": []any{
			map[string]any{"content": "Use fixture data only.", "role": "system"},
			map[string]any{"content": "Summarize the row.", "role": "user"},
		},
	}
	reversedMessages := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "Summarize the row."},
			map[string]any{"role": "system", "content": "Use fixture data only."},
		},
		"tools": request["tools"],
	}

	baseline := genericRecordReplayCanonicalHash(t, request)
	if got := genericRecordReplayCanonicalHash(t, reordered); got != baseline {
		t.Fatalf("object key order must not affect canonical request hash: got %s want %s", got, baseline)
	}
	if got := genericRecordReplayCanonicalHash(t, reversedMessages); got == baseline {
		t.Fatalf("array order must participate in canonical request hash")
	}
}

func TestGenericRecordReplayStrictOrderedMatching(t *testing.T) {
	base := genericRecordReplayLivePackageDir(t)
	index := loadGenericReplayIndex(t, base)
	records := loadGenericReplayRecords(t, base)
	requests := loadGenericReplayRequests(t, base, "matching_requests_fixture.json")

	summary, findings := runGenericStrictOrderedReplay(index, records, requests)
	if !reflect.DeepEqual(summary, index.ExpectedSummaries["strict_ordered_partial"]) {
		t.Fatalf("partial summary = %#v, want %#v; findings=%#v", summary, index.ExpectedSummaries["strict_ordered_partial"], findings)
	}
	if len(findings) != 1 || findings[0].Kind != "generic.ai.replay.unconsumed_record" ||
		findings[0].Cursor != 2 || findings[0].RecordID != "rec-002-final-summary" {
		t.Fatalf("unconsumed finding = %#v", findings)
	}
}

func TestGenericRecordReplayMismatchDoesNotScanAhead(t *testing.T) {
	base := genericRecordReplayLivePackageDir(t)
	index := loadGenericReplayIndex(t, base)
	records := loadGenericReplayRecords(t, base)
	requests := loadGenericReplayRequests(t, base, "mismatch_requests_fixture.json")

	summary, findings := runGenericStrictOrderedReplay(index, records, requests)
	if !reflect.DeepEqual(summary, index.ExpectedSummaries["strict_ordered_mismatch"]) {
		t.Fatalf("mismatch summary = %#v, want %#v; findings=%#v", summary, index.ExpectedSummaries["strict_ordered_mismatch"], findings)
	}
	if len(findings) != 3 {
		t.Fatalf("findings = %#v, want mismatch plus two unconsumed records", findings)
	}
	mismatch := findings[0]
	if mismatch.Kind != "generic.ai.replay.mismatch" || mismatch.Cursor != 1 ||
		mismatch.RecordID != "rec-001-tool-lookup" ||
		mismatch.Expected.ReplayKey != "turn:001:tool.lookup" ||
		mismatch.Actual.ReplayKey != "turn:002:llm.complete" ||
		mismatch.Expected.RequestHash != "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" ||
		mismatch.Actual.RequestHash != "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc" {
		t.Fatalf("mismatch finding does not explain strict ordered failure: %#v", mismatch)
	}
	if findings[1].RecordID != "rec-001-tool-lookup" || findings[2].RecordID != "rec-002-final-summary" {
		t.Fatalf("strict ordered matcher scanned ahead or consumed incorrectly: %#v", findings)
	}
}

func TestGenericRecordReplayDeterministicSummary(t *testing.T) {
	base := genericRecordReplayLivePackageDir(t)
	index := loadGenericReplayIndex(t, base)
	records := loadGenericReplayRecords(t, base)
	requests := loadGenericReplayRequests(t, base, "mismatch_requests_fixture.json")

	first, firstFindings := runGenericStrictOrderedReplay(index, records, requests)
	second, secondFindings := runGenericStrictOrderedReplay(index, records, requests)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(firstFindings, secondFindings) {
		t.Fatalf("strict ordered replay is not deterministic:\nfirst=%#v %#v\nsecond=%#v %#v", first, firstFindings, second, secondFindings)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("summary JSON changed between runs:\n%s\n%s", firstJSON, secondJSON)
	}
}

func runGenericStrictOrderedReplay(index genericReplayIndex, records []genericReplayRecord, requests []genericReplayRequest) (genericReplaySummary, []genericReplayFinding) {
	cursor := 0
	matched := 0
	mismatches := 0
	exhausted := 0
	matchedIDs := []string{}
	findings := []genericReplayFinding{}

	for _, request := range requests {
		if cursor >= len(records) {
			exhausted++
			findings = append(findings, genericReplayFinding{
				Kind:    index.Matching.ExhaustedFindingKind,
				Cursor:  cursor,
				Actual:  identityFromGenericReplayRequest(request),
				Message: "request had no remaining replay record",
			})
			continue
		}
		record := records[cursor]
		if genericReplayIdentityMatches(record, request) {
			matched++
			matchedIDs = append(matchedIDs, record.RecordID)
			cursor++
			continue
		}
		mismatches++
		findings = append(findings, genericReplayFinding{
			Kind:     index.Matching.MismatchFindingKind,
			Cursor:   cursor,
			RecordID: record.RecordID,
			Expected: identityFromGenericReplayRecord(record),
			Actual:   identityFromGenericReplayRequest(request),
			Message:  "strict ordered replay mismatch at current cursor",
		})
	}

	for i := cursor; i < len(records); i++ {
		findings = append(findings, genericReplayFinding{
			Kind:     index.Matching.UnconsumedFindingKind,
			Cursor:   i,
			RecordID: records[i].RecordID,
			Expected: identityFromGenericReplayRecord(records[i]),
			Message:  "replay record was not consumed",
		})
	}
	findingKinds := make([]string, 0, len(findings))
	for _, finding := range findings {
		findingKinds = append(findingKinds, finding.Kind)
	}
	return genericReplaySummary{
		FixtureID:        index.FixtureID,
		Strategy:         index.Strategy,
		LoadedRecords:    len(records),
		Requests:         len(requests),
		Matched:          matched,
		Mismatches:       mismatches,
		Unconsumed:       len(records) - cursor,
		Exhausted:        exhausted,
		NextIndex:        cursor,
		FindingKinds:     findingKinds,
		MatchedRecordIDs: matchedIDs,
	}, findings
}

func genericReplayIdentityMatches(record genericReplayRecord, request genericReplayRequest) bool {
	return record.Operation == request.Operation &&
		record.Capability == request.Capability &&
		record.ReplayKey == request.ReplayKey &&
		record.RequestHash == request.RequestHash
}

func identityFromGenericReplayRecord(record genericReplayRecord) genericReplayIdentity {
	return genericReplayIdentity{
		Operation:   record.Operation,
		Capability:  record.Capability,
		ReplayKey:   record.ReplayKey,
		RequestHash: record.RequestHash,
	}
}

func identityFromGenericReplayRequest(request genericReplayRequest) genericReplayIdentity {
	return genericReplayIdentity{
		Operation:   request.Operation,
		Capability:  request.Capability,
		ReplayKey:   request.ReplayKey,
		RequestHash: request.RequestHash,
	}
}

func loadGenericRecordReplayManifest(t *testing.T, base string) genericRecordReplayManifest {
	t.Helper()
	var manifest genericRecordReplayManifest
	decodeGenericRecordReplayJSON(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func loadGenericReplayIndex(t *testing.T, base string) genericReplayIndex {
	t.Helper()
	var index genericReplayIndex
	decodeGenericRecordReplayJSON(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	return index
}

func loadGenericReplayRecords(t *testing.T, base string) []genericReplayRecord {
	t.Helper()
	var fixture genericReplayRecordFixture
	decodeGenericRecordReplayJSON(t, filepath.Join(base, "fixtures", "ordered_records_fixture.json"), &fixture)
	if fixture.SchemaVersion != 1 || !fixture.ProviderFree || fixture.DomainSpecific ||
		fixture.LiveNetwork || fixture.RealDependencyImports || len(fixture.Records) != 3 {
		t.Fatalf("invalid record fixture header/count: %#v", fixture)
	}
	for i, record := range fixture.Records {
		if record.Sequence != i || record.RecordID == "" || record.ReplayKey == "" ||
			record.Operation == "" || record.Capability == "" ||
			!strings.HasPrefix(record.Capability, "generic.ai.") ||
			!strings.HasPrefix(record.RequestHash, "sha256:") ||
			!strings.HasPrefix(record.ResponseHash, "sha256:") ||
			!record.ProviderFree || record.RecordedAt == "" ||
			!record.Metadata.Fixture || !record.Metadata.Deterministic ||
			record.Metadata.LiveNetwork || record.Metadata.RealDependencyImports {
			t.Fatalf("invalid record[%d]: %#v", i, record)
		}
	}
	return fixture.Records
}

func loadGenericReplayRequests(t *testing.T, base, name string) []genericReplayRequest {
	t.Helper()
	var fixture genericReplayRequestFixture
	decodeGenericRecordReplayJSON(t, filepath.Join(base, "fixtures", name), &fixture)
	if fixture.SchemaVersion != 1 || !fixture.ProviderFree || fixture.DomainSpecific || len(fixture.Requests) == 0 {
		t.Fatalf("invalid request fixture %s: %#v", name, fixture)
	}
	for i, request := range fixture.Requests {
		if request.ReplayKey == "" || request.Operation == "" || request.Capability == "" ||
			!strings.HasPrefix(request.Capability, "generic.ai.") ||
			!strings.HasPrefix(request.RequestHash, "sha256:") {
			t.Fatalf("invalid request[%d] in %s: %#v", i, name, request)
		}
	}
	return fixture.Requests
}

func decodeGenericRecordReplayJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func genericRecordReplayCanonicalHash(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal canonical value: %v", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum)
}

func assertGenericRecordReplayPath(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if strings.HasSuffix(path, ".json") {
		var decoded any
		decodeGenericRecordReplayJSON(t, path, &decoded)
	}
}

func genericRecordReplayLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_record_replay")
}

func containsGenericRecordReplay(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
