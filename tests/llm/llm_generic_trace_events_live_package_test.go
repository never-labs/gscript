package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericTraceEventsManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	DialectSymbols              []string `json:"dialect_symbols"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	SourceModules               []string `json:"source_modules"`
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
		TelemetrySinkRequired       bool   `json:"telemetry_sink_required"`
		CleanSkipWithoutSink        bool   `json:"clean_skip_without_sink"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints       map[string]string                   `json:"entrypoints"`
	Schemas           map[string]string                   `json:"schemas"`
	Fixtures          map[string]string                   `json:"fixtures"`
	Capabilities      []string                            `json:"capabilities"`
	EventContracts    []genericTraceEventsEventContract   `json:"event_contracts"`
	SequencePolicy    genericTraceEventsSequencePolicy    `json:"sequence_policy"`
	CorrelationPolicy genericTraceEventsCorrelationPolicy `json:"correlation_policy"`
	RedactionPolicy   struct {
		Fixture              string   `json:"fixture"`
		DefaultAction        string   `json:"default_action"`
		SecretFields         []string `json:"secret_fields"`
		AllowedPreviewFields []string `json:"allowed_preview_fields"`
		HashAlgorithm        string   `json:"hash_algorithm"`
	} `json:"redaction_policy"`
	ReplayContract struct {
		Mode           string   `json:"mode"`
		Fixture        string   `json:"fixture"`
		MatchingKeys   []string `json:"matching_keys"`
		MismatchPolicy string   `json:"mismatch_policy"`
		AllowLiveSink  bool     `json:"allow_live_sink"`
	} `json:"replay_contract"`
	NoBuiltInGuarantee struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
	TestGates []string `json:"test_gates"`
}

type genericTraceEventsEventContract struct {
	EventType             string   `json:"event_type"`
	Capability            string   `json:"capability"`
	RequiredPayloadFields []string `json:"required_payload_fields"`
	ProviderNamePolicy    string   `json:"provider_name_policy"`
}

type genericTraceEventsSequencePolicy struct {
	Marker             string   `json:"marker"`
	Clock              string   `json:"clock"`
	StartSequence      int      `json:"start_sequence"`
	StrictlyIncreasing bool     `json:"strictly_increasing"`
	DedupeKeyFields    []string `json:"dedupe_key_fields"`
	TerminalEventTypes []string `json:"terminal_event_types"`
}

type genericTraceEventsCorrelationPolicy struct {
	RequiredIDs             []string `json:"required_ids"`
	OptionalIDs             []string `json:"optional_ids"`
	Format                  string   `json:"format"`
	ProviderRequestIDPolicy string   `json:"provider_request_id_policy"`
}

type genericTraceEventsContract struct {
	SchemaVersion         int      `json:"schema_version"`
	ID                    string   `json:"id"`
	ProviderFree          bool     `json:"provider_free"`
	LiveNetwork           bool     `json:"live_network"`
	RealDependencyImports bool     `json:"real_dependency_imports"`
	Capability            string   `json:"capability"`
	EmitShape             string   `json:"emit_shape"`
	RequiredEventTypes    []string `json:"required_event_types"`
	RequiredEventFields   []string `json:"required_event_fields"`
	CorrelationIDFields   []string `json:"correlation_id_fields"`
	SequenceMarker        struct {
		Name               string `json:"name"`
		Clock              string `json:"clock"`
		StrictlyIncreasing bool   `json:"strictly_increasing"`
		StartSequence      int    `json:"start_sequence"`
	} `json:"sequence_marker"`
	RedactionContract struct {
		PolicyRef               string `json:"policy_ref"`
		DefaultAction           string `json:"default_action"`
		HashAlgorithm           string `json:"hash_algorithm"`
		ProviderRequestIDPolicy string `json:"provider_request_id_policy"`
		RawPromptPolicy         string `json:"raw_prompt_policy"`
		RawCompletionPolicy     string `json:"raw_completion_policy"`
	} `json:"redaction_contract"`
	ReplayContract struct {
		Mode           string   `json:"mode"`
		Fixture        string   `json:"fixture"`
		MatchKeys      []string `json:"match_keys"`
		MismatchPolicy string   `json:"mismatch_policy"`
		AllowLiveSink  bool     `json:"allow_live_sink"`
	} `json:"replay_contract"`
	TraceEnvelopeContract struct {
		Schema                      string   `json:"schema"`
		Fixture                     string   `json:"fixture"`
		RequiredFields              []string `json:"required_fields"`
		RequiredCorrelationIDFields []string `json:"required_correlation_id_fields"`
		CoveredSurfaces             []string `json:"covered_surfaces"`
		ProviderFree                bool     `json:"provider_free"`
		LiveNetwork                 bool     `json:"live_network"`
		LiveModel                   bool     `json:"live_model"`
		CredentialsRequired         bool     `json:"credentials_required"`
	} `json:"trace_envelope_contract"`
}

type genericTraceEventsSequenceFixture struct {
	SchemaVersion         int                              `json:"schema_version"`
	FixtureKey            string                           `json:"fixture_key"`
	ProviderFree          bool                             `json:"provider_free"`
	LiveNetwork           bool                             `json:"live_network"`
	RealDependencyImports bool                             `json:"real_dependency_imports"`
	TraceID               string                           `json:"trace_id"`
	RunID                 string                           `json:"run_id"`
	TurnID                string                           `json:"turn_id"`
	SequencePolicy        genericTraceEventsSequencePolicy `json:"sequence_policy"`
	RedactionPolicyRef    string                           `json:"redaction_policy_ref"`
	Events                []genericTraceEventFixture       `json:"events"`
	ReplayAssertions      struct {
		Mode               string   `json:"mode"`
		ExpectedEventCount int      `json:"expected_event_count"`
		MatchKeys          []string `json:"match_keys"`
		MismatchPolicy     string   `json:"mismatch_policy"`
		AllowLiveSink      bool     `json:"allow_live_sink"`
	} `json:"replay_assertions"`
}

type genericTraceEventFixture struct {
	SchemaVersion         int    `json:"schema_version"`
	FixtureKey            string `json:"fixture_key"`
	ProviderFree          bool   `json:"provider_free"`
	LiveNetwork           bool   `json:"live_network"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	TraceID               string `json:"trace_id"`
	RunID                 string `json:"run_id"`
	TurnID                string `json:"turn_id"`
	EventID               string `json:"event_id"`
	EventType             string `json:"event_type"`
	Capability            string `json:"capability"`
	Sequence              int    `json:"sequence"`
	SequenceMarker        struct {
		Name               string `json:"name"`
		Clock              string `json:"clock"`
		StrictlyIncreasing bool   `json:"strictly_increasing"`
	} `json:"sequence_marker"`
	TimestampMS int `json:"timestamp_ms"`
	Correlation struct {
		TraceID       string `json:"trace_id"`
		RunID         string `json:"run_id"`
		TurnID        string `json:"turn_id"`
		EventID       string `json:"event_id"`
		ParentEventID string `json:"parent_event_id"`
		ToolCallID    string `json:"tool_call_id"`
		ArtifactID    string `json:"artifact_id"`
		ApprovalID    string `json:"approval_id"`
		ReplayID      string `json:"replay_id"`
	} `json:"correlation"`
	Redaction struct {
		PolicyID       string   `json:"policy_id"`
		Status         string   `json:"status"`
		RedactedFields []string `json:"redacted_fields"`
		HashAlgorithm  string   `json:"hash_algorithm"`
	} `json:"redaction"`
	Payload map[string]any `json:"payload"`
	Replay  struct {
		FixtureKey    string   `json:"fixture_key"`
		Deterministic bool     `json:"deterministic"`
		LiveSink      bool     `json:"live_sink"`
		MatchKeys     []string `json:"match_keys"`
	} `json:"replay"`
}

type genericTraceEventsRedactionPolicy struct {
	SchemaVersion          int      `json:"schema_version"`
	PolicyID               string   `json:"policy_id"`
	ProviderFree           bool     `json:"provider_free"`
	LiveNetwork            bool     `json:"live_network"`
	DefaultAction          string   `json:"default_action"`
	HashAlgorithm          string   `json:"hash_algorithm"`
	SecretFields           []string `json:"secret_fields"`
	DropFields             []string `json:"drop_fields"`
	HashFields             []string `json:"hash_fields"`
	AllowedPreviewFields   []string `json:"allowed_preview_fields"`
	ProviderIdentityPolicy struct {
		ProviderName      string `json:"provider_name"`
		ProviderRequestID string `json:"provider_request_id"`
		ProviderEndpoint  string `json:"provider_endpoint"`
	} `json:"provider_identity_policy"`
}

func TestGenericTraceEventsLivePackageManifestAndContracts(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	manifest := loadGenericTraceEventsManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-trace-events-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-generic-ai-trace-events" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !reflect.DeepEqual(manifest.DialectSymbols, []string{"generic.ai.trace.events", "ai.trace.emit"}) {
		t.Fatalf("dialect symbols = %#v", manifest.DialectSymbols)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	for _, boundary := range []string{"provider SDK traces", "hosted telemetry", "storage sinks", "model-specific event adapters"} {
		if !strings.Contains(manifest.Credentials.Policy, boundary) {
			t.Fatalf("credential policy should name %q boundary: %q", boundary, manifest.Credentials.Policy)
		}
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.TelemetrySinkRequired ||
		!manifest.DefaultPolicy.CleanSkipWithoutSink ||
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_trace_events_fixture" {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}
	for _, key := range []string{"smoke", "trace_events_contract", "fixture_index", "trace_sequence_fixture", "redaction_policy_fixture", "trace_envelope_fixture"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericTraceEventsEntrypointPath(t, manifest.Entrypoints[key])
	}
	for _, key := range []string{"trace_event", "trace_sequence", "trace_envelope", "redaction_policy", "correlation_ids"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertGenericTraceEventsJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "trace_sequence", "trace_envelope", "redaction_policy"} {
		path := manifest.Fixtures[key]
		if path == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertGenericTraceEventsJSONFile(t, filepath.Join(base, path))
	}
	assertGenericTraceEventsJSONFile(t, filepath.Join(base, manifest.Entrypoints["trace_events_contract"]))

	for _, want := range []string{
		"generic.ai.trace.events",
		"ai.trace.emit",
		"ai.trace.turn_start",
		"ai.trace.stream",
		"ai.trace.tool",
		"ai.trace.artifact",
		"ai.trace.approval",
		"ai.trace.replay",
		"ai.trace.envelope",
		"ai.trace.turn_end",
		"ai.trace.tool_call",
		"ai.trace.tool_result",
		"ai.trace.workflow_step",
		"ai.trace.replay_record_matched",
		"ai.trace.agent_start",
		"ai.trace.agent_turn_tool_dispatch",
		"ai.trace.agent_done",
		"ai.trace.workflow_correlation",
		"ai.trace.agent_correlation",
		"ai.trace.redaction_policy",
		"ai.trace.correlation_ids",
		"ai.trace.sequence_marker",
		"ai.trace.fixture_replay",
	} {
		if !genericTraceEventsContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	contracts := map[string]genericTraceEventsEventContract{}
	for _, contract := range manifest.EventContracts {
		contracts[contract.EventType] = contract
		if contract.ProviderNamePolicy != "generic_or_redacted" {
			t.Fatalf("%s provider policy = %q", contract.EventType, contract.ProviderNamePolicy)
		}
		if !strings.HasPrefix(contract.Capability, "ai.trace.") {
			t.Fatalf("%s capability = %q", contract.EventType, contract.Capability)
		}
		if len(contract.RequiredPayloadFields) == 0 {
			t.Fatalf("%s has no payload contract", contract.EventType)
		}
	}
	for _, eventType := range []string{"turn_start", "stream", "tool", "artifact", "approval", "replay", "turn_end", "tool_call", "tool_result", "approval_replay_trace", "workflow_step", "replay_record_matched", "agent_start", "agent_turn_tool_dispatch", "agent_done"} {
		if contracts[eventType].EventType == "" {
			t.Fatalf("missing event contract %q", eventType)
		}
	}

	if manifest.SequencePolicy.Marker != "fixture_sequence" ||
		manifest.SequencePolicy.Clock != "fixture_monotonic_ms" ||
		manifest.SequencePolicy.StartSequence != 1 ||
		!manifest.SequencePolicy.StrictlyIncreasing ||
		!reflect.DeepEqual(manifest.SequencePolicy.DedupeKeyFields, []string{"trace_id", "event_id", "sequence"}) ||
		!reflect.DeepEqual(manifest.SequencePolicy.TerminalEventTypes, []string{"replay"}) {
		t.Fatalf("sequence policy incomplete: %#v", manifest.SequencePolicy)
	}
	for _, want := range []string{"trace_id", "run_id", "turn_id", "event_id"} {
		if !genericTraceEventsContains(manifest.CorrelationPolicy.RequiredIDs, want) {
			t.Fatalf("correlation required IDs missing %q: %#v", want, manifest.CorrelationPolicy)
		}
	}
	for _, want := range []string{"workflow_run_id", "workflow_step_id", "agent_run_id", "replay_session_id"} {
		if !genericTraceEventsContains(manifest.CorrelationPolicy.OptionalIDs, want) {
			t.Fatalf("correlation optional IDs missing %q: %#v", want, manifest.CorrelationPolicy)
		}
	}
	if manifest.CorrelationPolicy.ProviderRequestIDPolicy != "omitted_or_redacted" {
		t.Fatalf("provider request id policy = %q", manifest.CorrelationPolicy.ProviderRequestIDPolicy)
	}
	if manifest.RedactionPolicy.DefaultAction != "hash_or_drop" ||
		manifest.RedactionPolicy.HashAlgorithm != "sha256" ||
		!genericTraceEventsContains(manifest.RedactionPolicy.SecretFields, "provider_request_id") ||
		!genericTraceEventsContains(manifest.RedactionPolicy.SecretFields, "raw_prompt") ||
		!genericTraceEventsContains(manifest.RedactionPolicy.AllowedPreviewFields, "text_preview") {
		t.Fatalf("redaction policy incomplete: %#v", manifest.RedactionPolicy)
	}
	if manifest.ReplayContract.Mode != "deterministic_fixture_replay" ||
		manifest.ReplayContract.MismatchPolicy != "fail_closed" ||
		manifest.ReplayContract.AllowLiveSink ||
		!reflect.DeepEqual(manifest.ReplayContract.MatchingKeys, []string{"trace_id", "event_id", "sequence", "event_type"}) {
		t.Fatalf("replay contract incomplete: %#v", manifest.ReplayContract)
	}
	if !manifest.NoBuiltInGuarantee.Required {
		t.Fatal("generic trace events package must declare no built-in guarantee")
	}
	if !strings.Contains(manifest.NoBuiltInGuarantee.Statement, manifest.PackageName) || !strings.Contains(manifest.NoBuiltInGuarantee.Statement, "provider-free package boundary") {
		t.Fatalf("no built-in guarantee should name package boundary: %q", manifest.NoBuiltInGuarantee.Statement)
	}
}

func TestGenericTraceEventsFixtureReplaySequence(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	var fixture genericTraceEventsSequenceFixture
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "trace_sequence_ACME_fixture.json"), &fixture)

	if fixture.SchemaVersion != 1 || !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports {
		t.Fatalf("fixture header must be provider-free and offline: %#v", fixture)
	}
	if fixture.TraceID == "" || fixture.RunID == "" || fixture.TurnID == "" || fixture.RedactionPolicyRef != "fixtures/redaction_policy_fixture.json" {
		t.Fatalf("fixture correlation header incomplete: %#v", fixture)
	}
	if fixture.SequencePolicy.Marker != "fixture_sequence" || fixture.SequencePolicy.Clock != "fixture_monotonic_ms" || !fixture.SequencePolicy.StrictlyIncreasing || fixture.SequencePolicy.StartSequence != 1 {
		t.Fatalf("sequence policy incomplete: %#v", fixture.SequencePolicy)
	}
	wantOrder := []string{"turn_start", "stream", "tool", "artifact", "approval", "replay"}
	if len(fixture.Events) != len(wantOrder) {
		t.Fatalf("events = %d, want %d", len(fixture.Events), len(wantOrder))
	}
	seenIDs := map[string]bool{}
	for i, event := range fixture.Events {
		if event.EventType != wantOrder[i] {
			t.Fatalf("event order[%d] = %q, want %q", i, event.EventType, wantOrder[i])
		}
		wantSequence := i + 1
		if event.Sequence != wantSequence {
			t.Fatalf("%s sequence = %d, want %d", event.EventID, event.Sequence, wantSequence)
		}
		if seenIDs[event.EventID] {
			t.Fatalf("duplicate event id %q", event.EventID)
		}
		seenIDs[event.EventID] = true
		assertGenericTraceEventEnvelope(t, fixture, event)
		assertGenericTraceEventPayload(t, event)
	}
	if fixture.ReplayAssertions.Mode != "deterministic_fixture_replay" ||
		fixture.ReplayAssertions.ExpectedEventCount != len(fixture.Events) ||
		fixture.ReplayAssertions.MismatchPolicy != "fail_closed" ||
		fixture.ReplayAssertions.AllowLiveSink ||
		!reflect.DeepEqual(fixture.ReplayAssertions.MatchKeys, []string{"trace_id", "event_id", "sequence", "event_type"}) {
		t.Fatalf("replay assertions incomplete: %#v", fixture.ReplayAssertions)
	}
}

func TestGenericTraceEventsPackageLocalUnifiedEnvelope(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	manifest := loadGenericTraceEventsManifest(t, base)
	contract := loadGenericTraceEventsContract(t, base)
	var fixture genericTraceEnvelopeFixtureDoc
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "trace_envelope_fixture.json"), &fixture)

	if manifest.Fixtures["trace_envelope"] != contract.TraceEnvelopeContract.Fixture ||
		manifest.Schemas["trace_envelope"] != contract.TraceEnvelopeContract.Schema {
		t.Fatalf("trace envelope manifest/contract disconnected: fixture=%q/%q schema=%q/%q",
			manifest.Fixtures["trace_envelope"], contract.TraceEnvelopeContract.Fixture,
			manifest.Schemas["trace_envelope"], contract.TraceEnvelopeContract.Schema)
	}
	if fixture.SchemaVersion != 1 ||
		fixture.ID != "generic-ai-trace-envelope-fixture" ||
		fixture.PackageBoundaryID != "generic-ai-trace-events" ||
		!fixture.ProviderFree ||
		fixture.DomainSpecific ||
		fixture.LiveNetwork ||
		fixture.LiveModel ||
		fixture.CredentialsRequired {
		t.Fatalf("trace envelope fixture header is not generic/provider-free: %#v", fixture)
	}
	if !contract.TraceEnvelopeContract.ProviderFree ||
		contract.TraceEnvelopeContract.LiveNetwork ||
		contract.TraceEnvelopeContract.LiveModel ||
		contract.TraceEnvelopeContract.CredentialsRequired {
		t.Fatalf("trace envelope contract must stay provider-free/offline: %#v", contract.TraceEnvelopeContract)
	}
	assertGenericTraceEventsSameStrings(t, "trace envelope fields", fixture.TraceEnvelopeSchema.RequiredFields, contract.TraceEnvelopeContract.RequiredFields)
	assertGenericTraceEventsSameStrings(t, "trace envelope correlation ids", fixture.TraceEnvelopeSchema.CorrelationIDFields, contract.TraceEnvelopeContract.RequiredCorrelationIDFields)
	assertGenericTraceEventsSameStrings(t, "trace envelope surfaces", contract.TraceEnvelopeContract.CoveredSurfaces, []string{"turn", "tool", "approval", "workflow", "replay", "agent"})

	wantByEventType := map[string][]string{
		"turn_start":               {"turn_id", "workflow_run_id", "replay_session_id"},
		"turn_end":                 {"turn_id", "workflow_run_id", "replay_session_id", "parent_event_id"},
		"tool_call":                {"turn_id", "tool_call_id", "workflow_run_id", "workflow_step_id", "replay_session_id"},
		"tool_result":              {"turn_id", "tool_call_id", "workflow_run_id", "workflow_step_id", "replay_session_id", "parent_event_id"},
		"approval_replay_trace":    {"approval_id", "tool_call_id", "workflow_run_id", "workflow_step_id", "replay_session_id"},
		"workflow_step":            {"workflow_run_id", "workflow_step_id", "parent_event_id"},
		"replay_record_matched":    {"turn_id", "tool_call_id", "workflow_run_id", "replay_session_id", "parent_event_id"},
		"agent_start":              {"agent_run_id"},
		"agent_turn_tool_dispatch": {"agent_run_id", "turn_id", "tool_call_id", "parent_event_id"},
		"agent_done":               {"agent_run_id", "parent_event_id"},
	}
	seenTypes := map[string]bool{}
	seenIDs := map[string]bool{}
	lastSequence := 0
	lastTimestamp := int64(0)
	for _, event := range fixture.Events {
		if event.TraceID == "" || event.EventID == "" || event.EventType == "" || event.TimestampMS == 0 || event.Sequence == 0 || event.Status == "" {
			t.Fatalf("trace envelope event missing required fields: %#v", event)
		}
		if seenIDs[event.EventID] {
			t.Fatalf("duplicate trace envelope event id %q", event.EventID)
		}
		seenIDs[event.EventID] = true
		if event.Sequence <= lastSequence || event.TimestampMS < lastTimestamp {
			t.Fatalf("trace envelope is not stably ordered: %#v after seq=%d ts=%d", event, lastSequence, lastTimestamp)
		}
		lastSequence = event.Sequence
		lastTimestamp = event.TimestampMS
		wantCorrelation, ok := wantByEventType[event.EventType]
		if !ok {
			t.Fatalf("unexpected trace envelope event type %q", event.EventType)
		}
		for _, field := range wantCorrelation {
			if event.Correlation[field] == "" {
				t.Fatalf("%s missing correlation id %q: %#v", event.EventID, field, event.Correlation)
			}
		}
		seenTypes[event.EventType] = true
	}
	for eventType := range wantByEventType {
		if !seenTypes[eventType] {
			t.Fatalf("missing trace envelope event type %q; got %v", eventType, sortedGenericTraceKeys(seenTypes))
		}
	}
}

func TestGenericTraceEventsRedactionPolicy(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	var policy genericTraceEventsRedactionPolicy
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "redaction_policy_fixture.json"), &policy)

	if policy.SchemaVersion != 1 || policy.PolicyID != "generic-trace-redaction-v1" || !policy.ProviderFree || policy.LiveNetwork {
		t.Fatalf("redaction policy header incomplete: %#v", policy)
	}
	if policy.DefaultAction != "hash_or_drop" || policy.HashAlgorithm != "sha256" {
		t.Fatalf("redaction defaults = %#v", policy)
	}
	for _, field := range []string{"api_key", "authorization", "cookie", "provider_request_id", "raw_prompt", "raw_completion", "tool_input_raw", "tool_output_raw"} {
		if !genericTraceEventsContains(policy.SecretFields, field) {
			t.Fatalf("secret fields missing %q: %#v", field, policy.SecretFields)
		}
	}
	for _, field := range []string{"api_key", "authorization", "cookie", "provider_request_id"} {
		if !genericTraceEventsContains(policy.DropFields, field) {
			t.Fatalf("drop fields missing %q: %#v", field, policy.DropFields)
		}
	}
	for _, field := range []string{"raw_prompt", "raw_completion", "tool_input_raw", "tool_output_raw"} {
		if !genericTraceEventsContains(policy.HashFields, field) {
			t.Fatalf("hash fields missing %q: %#v", field, policy.HashFields)
		}
	}
	for _, field := range []string{"text_preview", "status", "artifact_kind", "decision", "model_alias", "tool_name"} {
		if !genericTraceEventsContains(policy.AllowedPreviewFields, field) {
			t.Fatalf("allowed preview fields missing %q: %#v", field, policy.AllowedPreviewFields)
		}
	}
	if policy.ProviderIdentityPolicy.ProviderName != "generic_or_redacted" ||
		policy.ProviderIdentityPolicy.ProviderRequestID != "drop" ||
		policy.ProviderIdentityPolicy.ProviderEndpoint != "drop" {
		t.Fatalf("provider identity redaction policy incomplete: %#v", policy.ProviderIdentityPolicy)
	}
}

func TestGenericTraceEventsEnvelopeSequenceCorrelationRedactionAreConnected(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	manifest := loadGenericTraceEventsManifest(t, base)
	contract := loadGenericTraceEventsContract(t, base)
	var sequence genericTraceEventsSequenceFixture
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "trace_sequence_ACME_fixture.json"), &sequence)
	var policy genericTraceEventsRedactionPolicy
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "redaction_policy_fixture.json"), &policy)

	if contract.SchemaVersion != 1 || contract.ID != "generic-ai-trace-events-contract" ||
		!contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports {
		t.Fatalf("contract header must stay provider-free and offline: %#v", contract)
	}
	if contract.Capability != "generic.ai.trace.events" ||
		!genericTraceEventsContains(manifest.Capabilities, contract.Capability) ||
		contract.EmitShape != "ai.trace.emit" ||
		!genericTraceEventsContains(manifest.DialectSymbols, contract.EmitShape) {
		t.Fatalf("contract capability/emit shape do not mirror manifest: %#v %#v", contract, manifest.DialectSymbols)
	}
	assertGenericTraceEventsSameStrings(t, "contract event types", contract.RequiredEventTypes, genericTraceEventsContractTypes(manifest.EventContracts))
	assertGenericTraceEventsSameStrings(t, "contract correlation ids", contract.CorrelationIDFields, append(append([]string{}, manifest.CorrelationPolicy.RequiredIDs...), manifest.CorrelationPolicy.OptionalIDs...))
	assertGenericTraceEventsSameStrings(t, "contract replay match keys", contract.ReplayContract.MatchKeys, manifest.ReplayContract.MatchingKeys)
	if contract.RedactionContract.PolicyRef != manifest.RedactionPolicy.Fixture ||
		contract.RedactionContract.PolicyRef != sequence.RedactionPolicyRef ||
		manifest.RedactionPolicy.Fixture != filepath.ToSlash(filepath.Join("fixtures", "redaction_policy_fixture.json")) {
		t.Fatalf("redaction policy references are disconnected: contract=%q manifest=%q sequence=%q", contract.RedactionContract.PolicyRef, manifest.RedactionPolicy.Fixture, sequence.RedactionPolicyRef)
	}
	if contract.RedactionContract.DefaultAction != manifest.RedactionPolicy.DefaultAction ||
		contract.RedactionContract.DefaultAction != policy.DefaultAction ||
		contract.RedactionContract.HashAlgorithm != manifest.RedactionPolicy.HashAlgorithm ||
		contract.RedactionContract.HashAlgorithm != policy.HashAlgorithm {
		t.Fatalf("redaction policy values are disconnected: contract=%#v manifest=%#v policy=%#v", contract.RedactionContract, manifest.RedactionPolicy, policy)
	}
	for _, field := range manifest.RedactionPolicy.SecretFields {
		if !genericTraceEventsContains(policy.SecretFields, field) {
			t.Fatalf("manifest redaction secret field %q is not declared by policy %#v", field, policy.SecretFields)
		}
	}
	if contract.RedactionContract.ProviderRequestIDPolicy != policy.ProviderIdentityPolicy.ProviderRequestID {
		t.Fatalf("provider request redaction mismatch: contract=%q policy=%q", contract.RedactionContract.ProviderRequestIDPolicy, policy.ProviderIdentityPolicy.ProviderRequestID)
	}
	if contract.SequenceMarker.Name != manifest.SequencePolicy.Marker ||
		contract.SequenceMarker.Name != sequence.SequencePolicy.Marker ||
		contract.SequenceMarker.Clock != manifest.SequencePolicy.Clock ||
		contract.SequenceMarker.Clock != sequence.SequencePolicy.Clock ||
		contract.SequenceMarker.StartSequence != manifest.SequencePolicy.StartSequence ||
		contract.SequenceMarker.StartSequence != sequence.SequencePolicy.StartSequence ||
		contract.SequenceMarker.StrictlyIncreasing != manifest.SequencePolicy.StrictlyIncreasing ||
		contract.SequenceMarker.StrictlyIncreasing != sequence.SequencePolicy.StrictlyIncreasing {
		t.Fatalf("sequence policy is disconnected: contract=%#v manifest=%#v sequence=%#v", contract.SequenceMarker, manifest.SequencePolicy, sequence.SequencePolicy)
	}
	if contract.ReplayContract.Fixture != manifest.ReplayContract.Fixture ||
		contract.ReplayContract.Mode != manifest.ReplayContract.Mode ||
		contract.ReplayContract.MismatchPolicy != manifest.ReplayContract.MismatchPolicy ||
		contract.ReplayContract.AllowLiveSink != manifest.ReplayContract.AllowLiveSink {
		t.Fatalf("replay policy is disconnected: contract=%#v manifest=%#v", contract.ReplayContract, manifest.ReplayContract)
	}

	requiredEnvelopeFields := []string{"schema_version", "fixture_key", "provider_free", "live_network", "real_dependency_imports", "trace_id", "run_id", "turn_id", "event_id", "event_type", "capability", "sequence", "sequence_marker", "timestamp_ms", "correlation", "redaction", "payload", "replay"}
	for _, field := range requiredEnvelopeFields {
		if !genericTraceEventsContains(contract.RequiredEventFields, field) {
			t.Fatalf("contract required envelope fields missing %q: %#v", field, contract.RequiredEventFields)
		}
	}

	assertGenericTraceEventsSequenceSemantics(t, manifest, sequence, policy)
	assertGenericTraceEventsPackageHasNoRawLeak(t, base)
}

func TestGenericTraceEventsMainLeiaSmoke(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	data, err := os.ReadFile(filepath.Join(base, "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, want := range []string{"generic.ai.trace.events", "ai.trace.emit", "fixture_sequence", "deterministic_fixture_replay", "hash_or_drop"} {
		if !strings.Contains(src, want) {
			t.Fatalf("main.leia missing %q", want)
		}
	}
	for _, forbidden := range []string{"openai", "anthropic", "gemini", "telemetry.export"} {
		if strings.Contains(strings.ToLower(src), strings.ToLower(forbidden)) {
			t.Fatalf("main.leia must stay provider-free and avoid forbidden dependency %q", forbidden)
		}
	}

	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString)}, tc.opts...)...)
			if err := vm.ExecFile(filepath.Join(base, "main.leia")); err != nil {
				t.Fatalf("ExecFile main.leia: %v", err)
			}
			valid, _ := vm.Get("generic_trace_events_contract_valid")
			if valid != true {
				t.Fatalf("generic_trace_events_contract_valid = %#v", valid)
			}
		})
	}
}

func assertGenericTraceEventEnvelope(t *testing.T, sequence genericTraceEventsSequenceFixture, event genericTraceEventFixture) {
	t.Helper()
	if event.SchemaVersion != 1 || !event.ProviderFree || event.LiveNetwork || event.RealDependencyImports {
		t.Fatalf("%s must be provider-free and offline: %#v", event.EventID, event)
	}
	if event.TraceID != sequence.TraceID || event.RunID != sequence.RunID || event.TurnID != sequence.TurnID {
		t.Fatalf("%s must match sequence correlation header: %#v", event.EventID, event)
	}
	if event.Correlation.TraceID != event.TraceID ||
		event.Correlation.RunID != event.RunID ||
		event.Correlation.TurnID != event.TurnID ||
		event.Correlation.EventID != event.EventID {
		t.Fatalf("%s correlation IDs do not mirror event envelope: %#v", event.EventID, event.Correlation)
	}
	if event.SequenceMarker.Name != "fixture_sequence" ||
		event.SequenceMarker.Clock != "fixture_monotonic_ms" ||
		!event.SequenceMarker.StrictlyIncreasing {
		t.Fatalf("%s sequence marker incomplete: %#v", event.EventID, event.SequenceMarker)
	}
	if event.Redaction.PolicyID != "generic-trace-redaction-v1" || event.Redaction.HashAlgorithm != "sha256" {
		t.Fatalf("%s redaction metadata incomplete: %#v", event.EventID, event.Redaction)
	}
	if event.Replay.FixtureKey != sequence.FixtureKey ||
		!event.Replay.Deterministic ||
		event.Replay.LiveSink ||
		!reflect.DeepEqual(event.Replay.MatchKeys, []string{"trace_id", "event_id", "sequence", "event_type"}) {
		t.Fatalf("%s replay metadata incomplete: %#v", event.EventID, event.Replay)
	}
	if !strings.HasPrefix(event.Capability, "ai.trace.") {
		t.Fatalf("%s capability = %q", event.EventID, event.Capability)
	}
	if _, ok := event.Payload["provider_request_id"]; ok {
		t.Fatalf("%s payload must not expose provider_request_id: %#v", event.EventID, event.Payload)
	}
}

func assertGenericTraceEventPayload(t *testing.T, event genericTraceEventFixture) {
	t.Helper()
	required := map[string][]string{
		"turn_start": {"model_alias", "message_count", "tools_declared", "provider_name"},
		"stream":     {"chunk_index", "delta_kind", "text_preview", "token_count"},
		"tool":       {"tool_call_id", "tool_name", "status", "input_digest", "output_digest"},
		"artifact":   {"artifact_id", "artifact_kind", "media_type", "content_hash"},
		"approval":   {"approval_id", "operation", "decision", "policy_id"},
		"replay":     {"replay_fixture_key", "matched_event_count", "mismatch_count", "deterministic"},
	}
	fields, ok := required[event.EventType]
	if !ok {
		t.Fatalf("unexpected event type %q", event.EventType)
	}
	for _, field := range fields {
		if _, ok := event.Payload[field]; !ok {
			t.Fatalf("%s payload missing %q: %#v", event.EventType, field, event.Payload)
		}
	}
	if provider, ok := event.Payload["provider_name"].(string); ok && provider != "generic" && provider != "redacted" {
		t.Fatalf("%s provider_name = %q", event.EventID, provider)
	}
	switch event.EventType {
	case "tool":
		if event.Correlation.ToolCallID == "" {
			t.Fatalf("%s missing tool_call_id correlation", event.EventID)
		}
	case "artifact":
		if event.Correlation.ArtifactID == "" {
			t.Fatalf("%s missing artifact_id correlation", event.EventID)
		}
	case "approval":
		if event.Correlation.ApprovalID == "" {
			t.Fatalf("%s missing approval_id correlation", event.EventID)
		}
	case "replay":
		if event.Correlation.ReplayID == "" || event.Payload["mismatch_count"].(float64) != 0 {
			t.Fatalf("%s replay payload/correlation incomplete: %#v %#v", event.EventID, event.Correlation, event.Payload)
		}
	}
}

func genericTraceEventsLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_trace_events")
}

func loadGenericTraceEventsManifest(t *testing.T, base string) genericTraceEventsManifest {
	t.Helper()
	var manifest genericTraceEventsManifest
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func loadGenericTraceEventsContract(t *testing.T, base string) genericTraceEventsContract {
	t.Helper()
	var contract genericTraceEventsContract
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "contracts", "trace_events_contract.json"), &contract)
	return contract
}

func decodeGenericTraceEventsJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func assertGenericTraceEventsJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeGenericTraceEventsJSONFile(t, path, &value)
}

func assertGenericTraceEventsEntrypointPath(t *testing.T, path string) {
	t.Helper()
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path {
		t.Fatalf("entrypoint must be a clean relative file path: %q", path)
	}
	switch filepath.Ext(path) {
	case ".json", ".leia":
	default:
		t.Fatalf("entrypoint must reference a JSON or Leia file path: %q", path)
	}
}

func assertGenericTraceEventsSequenceSemantics(t *testing.T, manifest genericTraceEventsManifest, sequence genericTraceEventsSequenceFixture, policy genericTraceEventsRedactionPolicy) {
	t.Helper()
	seenDedupeKeys := map[string]bool{}
	seenEvents := map[string]bool{}
	lastSequence := manifest.SequencePolicy.StartSequence - 1
	lastTimestamp := -1
	terminalTypes := map[string]bool{}
	for _, eventType := range manifest.SequencePolicy.TerminalEventTypes {
		terminalTypes[eventType] = true
	}
	for i, event := range sequence.Events {
		if event.Sequence != lastSequence+1 || event.TimestampMS < lastTimestamp {
			t.Fatalf("%s sequence/timestamp is not strictly explainable: seq=%d after %d ts=%d after %d", event.EventID, event.Sequence, lastSequence, event.TimestampMS, lastTimestamp)
		}
		lastSequence = event.Sequence
		lastTimestamp = event.TimestampMS
		dedupeKey := genericTraceEventsDedupeKey(event, manifest.SequencePolicy.DedupeKeyFields)
		if seenDedupeKeys[dedupeKey] {
			t.Fatalf("%s duplicate dedupe key %q", event.EventID, dedupeKey)
		}
		seenDedupeKeys[dedupeKey] = true
		if terminalTypes[event.EventType] && i != len(sequence.Events)-1 {
			t.Fatalf("%s terminal event type %q appears before final event", event.EventID, event.EventType)
		}
		if event.Correlation.ParentEventID != "" && !seenEvents[event.Correlation.ParentEventID] {
			t.Fatalf("%s parent_event_id %q does not reference an earlier event", event.EventID, event.Correlation.ParentEventID)
		}
		seenEvents[event.EventID] = true
		assertGenericTraceEventsRedactionExplainsPayload(t, event, policy)
	}
	if len(sequence.Events) == 0 || !terminalTypes[sequence.Events[len(sequence.Events)-1].EventType] {
		t.Fatalf("final event must be one of terminal event types %#v", manifest.SequencePolicy.TerminalEventTypes)
	}
}

func assertGenericTraceEventsRedactionExplainsPayload(t *testing.T, event genericTraceEventFixture, policy genericTraceEventsRedactionPolicy) {
	t.Helper()
	for _, field := range event.Redaction.RedactedFields {
		if !genericTraceEventsContains(policy.SecretFields, field) {
			t.Fatalf("%s redacted field %q is not declared secret by policy %#v", event.EventID, field, policy.SecretFields)
		}
	}
	for _, field := range policy.DropFields {
		if _, ok := event.Payload[field]; ok {
			t.Fatalf("%s payload exposes dropped field %q: %#v", event.EventID, field, event.Payload)
		}
	}
	for _, field := range policy.HashFields {
		if _, ok := event.Payload[field]; ok {
			t.Fatalf("%s payload exposes raw hash field %q: %#v", event.EventID, field, event.Payload)
		}
		if genericTraceEventsContains(event.Redaction.RedactedFields, field) {
			if !genericTraceEventsPayloadHasDigestForField(event.Payload, field, policy.HashAlgorithm) {
				t.Fatalf("%s redacts %q but lacks an explanatory %s digest: %#v", event.EventID, field, policy.HashAlgorithm, event.Payload)
			}
		}
	}
	if event.Redaction.Status == "clean" && len(event.Redaction.RedactedFields) != 0 {
		t.Fatalf("%s clean redaction status cannot name redacted fields: %#v", event.EventID, event.Redaction)
	}
	if event.Redaction.Status != "clean" && len(event.Redaction.RedactedFields) == 0 {
		t.Fatalf("%s non-clean redaction status must name redacted fields: %#v", event.EventID, event.Redaction)
	}
}

func genericTraceEventsPayloadHasDigestForField(payload map[string]any, field, algorithm string) bool {
	hashFields := []string{strings.TrimSuffix(field, "_raw") + "_hash"}
	switch field {
	case "tool_input_raw":
		hashFields = append(hashFields, "input_digest")
	case "tool_output_raw":
		hashFields = append(hashFields, "output_digest")
	}
	for _, hashField := range hashFields {
		value, ok := payload[hashField].(string)
		if ok && strings.HasPrefix(value, algorithm+":") {
			return true
		}
	}
	return false
}

func assertGenericTraceEventsPackageHasNoRawLeak(t *testing.T, base string) {
	t.Helper()
	for _, rel := range []string{
		"main.leia",
		"package.manifest.json",
		"contracts/trace_events_contract.json",
		"fixtures/provider_free_fixture_index.json",
		"fixtures/redaction_policy_fixture.json",
		"fixtures/trace_envelope_fixture.json",
		"fixtures/trace_sequence_ACME_fixture.json",
		"schemas/correlation_ids_v1.schema.json",
		"schemas/redaction_policy_v1.schema.json",
		"schemas/trace_event_v1.schema.json",
		"schemas/trace_envelope_v1.schema.json",
		"schemas/trace_sequence_v1.schema.json",
	} {
		data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		assertGenericTraceEventsTextHasNoRawLeak(t, rel, string(data))
	}
}

func assertGenericTraceEventsTextHasNoRawLeak(t *testing.T, path, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	for _, forbidden := range []string{
		"sk-",
		"bearer ",
		"api.openai.com",
		"api.anthropic.com",
		"generativelanguage.googleapis.com",
		"azure.com/openai",
		"localhost:",
		"127.0.0.1",
		"live_endpoint",
		"provider_endpoint_url",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("%s contains raw secret/live endpoint/provider leak marker %q", path, forbidden)
		}
	}
	for _, provider := range []string{"openai", "anthropic", "gemini", "azure-openai", "mistral", "cohere"} {
		if strings.Contains(lower, `"provider_name": "`+provider+`"`) ||
			strings.Contains(lower, `"provider": "`+provider+`"`) {
			t.Fatalf("%s contains raw provider identity %q", path, provider)
		}
	}
}

func assertGenericTraceEventsSameStrings(t *testing.T, label string, got, want []string) {
	t.Helper()
	got = append([]string{}, got...)
	want = append([]string{}, want...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}

func genericTraceEventsContractTypes(contracts []genericTraceEventsEventContract) []string {
	eventTypes := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		eventTypes = append(eventTypes, contract.EventType)
	}
	return eventTypes
}

func genericTraceEventsDedupeKey(event genericTraceEventFixture, fields []string) string {
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		switch field {
		case "trace_id":
			values = append(values, event.TraceID)
		case "event_id":
			values = append(values, event.EventID)
		case "sequence":
			values = append(values, strconv.Itoa(event.Sequence))
		case "event_type":
			values = append(values, event.EventType)
		default:
			values = append(values, "")
		}
	}
	return strings.Join(values, "\x00")
}

func genericTraceEventsContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
