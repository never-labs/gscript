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
	ExternalEnvelopeProjections []genericTraceEventsExternalProjection `json:"external_envelope_projections"`
}

type genericTraceEventsExternalProjection struct {
	ID                    string   `json:"id"`
	SourcePackages        []string `json:"source_packages"`
	SourceFixtures        []string `json:"source_fixtures"`
	SourcePackage         string   `json:"source_package"`
	SourceSchema          string   `json:"source_schema"`
	SourceFixture         string   `json:"source_fixture"`
	SourceKind            string   `json:"source_kind"`
	TargetSchema          string   `json:"target_schema"`
	TargetFixture         string   `json:"target_fixture"`
	TargetEventType       string   `json:"target_event_type"`
	TargetEventTypes      []string `json:"target_event_types"`
	TargetCapability      string   `json:"target_capability"`
	RequiredSourcePaths   []string `json:"required_source_paths"`
	RequiredTargetPaths   []string `json:"required_target_paths"`
	NullPolicy            string   `json:"null_policy"`
	IdentityPolicy        string   `json:"identity_policy"`
	ProviderFree          bool     `json:"provider_free"`
	LiveNetwork           bool     `json:"live_network"`
	LiveModel             bool     `json:"live_model"`
	CredentialsRequired   bool     `json:"credentials_required"`
	RealDependencyImports bool     `json:"real_dependency_imports"`
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
		PolicyID            string   `json:"policy_id"`
		Status              string   `json:"status"`
		RedactedFields      []string `json:"redacted_fields"`
		HashAlgorithm       string   `json:"hash_algorithm"`
		SecretValuesPresent bool     `json:"secret_values_present"`
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

type genericTraceEventsApprovalProjectionFixture struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	ProjectionKind        string `json:"projection_kind"`
	ProviderFree          bool   `json:"provider_free"`
	DomainSpecific        bool   `json:"domain_specific"`
	LiveNetwork           bool   `json:"live_network"`
	LiveModel             bool   `json:"live_model"`
	CredentialsRequired   bool   `json:"credentials_required"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	Source                struct {
		PackageID      string   `json:"package_id"`
		PackageName    string   `json:"package_name"`
		Schema         string   `json:"schema"`
		Fixture        string   `json:"fixture"`
		Kind           string   `json:"kind"`
		RequiredFields []string `json:"required_fields"`
	} `json:"source"`
	Target struct {
		PackageID      string   `json:"package_id"`
		Schema         string   `json:"schema"`
		EventType      string   `json:"event_type"`
		Capability     string   `json:"capability"`
		RequiredFields []string `json:"required_fields"`
	} `json:"target"`
	FieldMappings []struct {
		Source     string `json:"source"`
		Target     string `json:"target"`
		NullPolicy string `json:"null_policy"`
	} `json:"field_mappings"`
	Fallbacks struct {
		RunID                 string `json:"run_id"`
		TurnID                string `json:"turn_id"`
		ApprovalIDWhenMissing string `json:"approval_id_when_missing"`
		EventIDTemplate       string `json:"event_id_template"`
		Sequence              int    `json:"sequence"`
		TimestampMS           int    `json:"timestamp_ms"`
	} `json:"fallbacks"`
	ProjectedEvent genericTraceEventFixture `json:"projected_event"`
}

type genericTraceEventsApprovalSourceFixture struct {
	SchemaVersion         int    `json:"schema_version"`
	Kind                  string `json:"kind"`
	TraceID               string `json:"trace_id"`
	FixtureKey            string `json:"fixture_key"`
	ProviderFree          bool   `json:"provider_free"`
	LiveNetwork           bool   `json:"live_network"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	Request               struct {
		RequestID  string         `json:"request_id"`
		Tool       string         `json:"tool"`
		Capability string         `json:"capability"`
		RiskLevel  string         `json:"risk_level"`
		ArgsShape  map[string]any `json:"args_shape"`
	} `json:"request"`
	Policy struct {
		Package                      string   `json:"package"`
		DefaultDecision              string   `json:"default_decision"`
		ExactCapabilityMatchRequired bool     `json:"exact_capability_match_required"`
		AllowedCapabilities          []string `json:"allowed_capabilities"`
	} `json:"policy"`
	Decision struct {
		Status           string  `json:"status"`
		Reason           string  `json:"reason"`
		ApprovalRequired bool    `json:"approval_required"`
		ApprovalID       *string `json:"approval_id"`
	} `json:"decision"`
	Result struct {
		Status              string `json:"status"`
		Executed            bool   `json:"executed"`
		SideEffects         bool   `json:"side_effects"`
		SecretValuesPresent bool   `json:"secret_values_present"`
	} `json:"result"`
	Replay struct {
		Mode                string `json:"mode"`
		Deterministic       bool   `json:"deterministic"`
		FixtureSHA256       string `json:"fixture_sha256"`
		CreatedFromProvider bool   `json:"created_from_provider"`
	} `json:"replay"`
}

type genericTraceEventsModelTurnReplayProjectionFixture struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	ProjectionKind        string `json:"projection_kind"`
	ProviderFree          bool   `json:"provider_free"`
	DomainSpecific        bool   `json:"domain_specific"`
	LiveNetwork           bool   `json:"live_network"`
	LiveModel             bool   `json:"live_model"`
	CredentialsRequired   bool   `json:"credentials_required"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	Sources               map[string]struct {
		PackageID      string `json:"package_id"`
		Fixture        string `json:"fixture"`
		Schema         string `json:"schema"`
		RecordSelector string `json:"record_selector"`
	} `json:"sources"`
	CorrelationPolicy struct {
		TraceIDSource               string   `json:"trace_id_source"`
		TurnIDSource                string   `json:"turn_id_source"`
		ReplaySessionIDSource       string   `json:"replay_session_id_source"`
		RequestHashSource           string   `json:"request_hash_source"`
		TurnRecordIDSource          string   `json:"turn_record_id_source"`
		RecordReplayIDSource        string   `json:"record_replay_id_source"`
		SourceIDsAreNotAssumedEqual []string `json:"source_ids_are_not_assumed_equal"`
	} `json:"correlation_policy"`
	FieldMappings []struct {
		Source string `json:"source"`
		Target string `json:"target"`
	} `json:"field_mappings"`
	ProjectedEvents []genericTraceEventsProjectedEnvelopeEvent `json:"projected_events"`
}

type genericTraceEventsToolApprovalResultCompositionFixture struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	CompositionKind       string `json:"composition_kind"`
	ProviderFree          bool   `json:"provider_free"`
	DomainSpecific        bool   `json:"domain_specific"`
	LiveNetwork           bool   `json:"live_network"`
	LiveModel             bool   `json:"live_model"`
	CredentialsRequired   bool   `json:"credentials_required"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	SourceRefs            map[string]struct {
		PackageID string `json:"package_id"`
		Fixture   string `json:"fixture"`
		Schema    string `json:"schema"`
		Role      string `json:"role"`
	} `json:"source_refs"`
	IdentityPolicy struct {
		CanonicalToolCallID      string   `json:"canonical_tool_call_id"`
		CanonicalToolName        string   `json:"canonical_tool_name"`
		CanonicalCapability      string   `json:"canonical_capability"`
		ApprovalIDWhenMissing    string   `json:"approval_id_when_missing"`
		SourceIDsAreNotAssumedEq []string `json:"source_ids_are_not_assumed_equal"`
		NormalizationPolicy      string   `json:"normalization_policy"`
	} `json:"identity_policy"`
	FieldMappings []struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Policy string `json:"policy"`
	} `json:"field_mappings"`
	SourceObservations map[string]map[string]any                  `json:"source_observations"`
	ProjectedEvents    []genericTraceEventsProjectedEnvelopeEvent `json:"projected_events"`
}

type genericTraceEventsProjectedEnvelopeEvent struct {
	TraceID     string            `json:"trace_id"`
	EventID     string            `json:"event_id"`
	EventType   string            `json:"event_type"`
	TimestampMS int64             `json:"timestamp_ms"`
	Sequence    int               `json:"sequence"`
	Status      string            `json:"status"`
	Correlation map[string]string `json:"correlation"`
	Payload     map[string]any    `json:"payload"`
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
	for _, key := range []string{"smoke", "trace_events_contract", "fixture_index", "trace_sequence_fixture", "redaction_policy_fixture", "trace_envelope_fixture", "approval_trace_projection_fixture", "tool_invocation_projection_fixture", "model_turn_replay_trace_projection_fixture", "tool_approval_result_composition_fixture"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericTraceEventsEntrypointPath(t, manifest.Entrypoints[key])
	}
	for _, key := range []string{"trace_event", "trace_sequence", "trace_envelope", "approval_trace_projection", "tool_invocation_projection", "model_turn_replay_trace_projection", "tool_approval_result_composition", "redaction_policy", "correlation_ids"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertGenericTraceEventsJSONFile(t, filepath.Join(base, path))
	}
	for _, key := range []string{"index", "trace_sequence", "trace_envelope", "approval_trace_projection", "tool_invocation_projection", "model_turn_replay_trace_projection", "tool_approval_result_composition", "redaction_policy"} {
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
		"ai.trace.approval_projection",
		"ai.trace.tool_invocation_projection",
		"ai.trace.model_turn_replay_projection",
		"ai.trace.tool_approval_result_composition",
		"ai.trace.external_envelope_projection",
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

func TestGenericTraceEventsApprovalTraceProjection(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	manifest := loadGenericTraceEventsManifest(t, base)
	contract := loadGenericTraceEventsContract(t, base)
	var projection genericTraceEventsApprovalProjectionFixture
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "approval_trace_projection_fixture.json"), &projection)

	sourcePath := filepath.Join(base, filepath.FromSlash(projection.Source.Fixture))
	var source genericTraceEventsApprovalSourceFixture
	decodeGenericTraceEventsJSONFile(t, sourcePath, &source)

	if manifest.Fixtures["approval_trace_projection"] != "fixtures/approval_trace_projection_fixture.json" ||
		manifest.Schemas["approval_trace_projection"] != "schemas/approval_trace_projection_v1.schema.json" {
		t.Fatalf("approval projection manifest entries missing: fixtures=%#v schemas=%#v", manifest.Fixtures, manifest.Schemas)
	}
	for _, want := range []string{"ai.trace.approval_projection", "ai.trace.external_envelope_projection"} {
		if !genericTraceEventsContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	projectionContract := genericTraceEventsFindExternalProjection(contract, "generic-approval-trace-to-ai-trace-approval")
	if projectionContract.ID == "" {
		t.Fatalf("missing approval external projection contract: %#v", contract.ExternalEnvelopeProjections)
	}
	if projectionContract.SourceFixture != projection.Source.Fixture ||
		projectionContract.SourceSchema != projection.Source.Schema ||
		projectionContract.TargetFixture != manifest.Fixtures["approval_trace_projection"] ||
		projectionContract.TargetSchema != projection.Target.Schema ||
		projectionContract.TargetEventType != projection.Target.EventType ||
		projectionContract.TargetCapability != projection.Target.Capability {
		t.Fatalf("approval projection manifest/contract/fixture disconnected: contract=%#v projection=%#v", projectionContract, projection)
	}
	if !projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork || projection.LiveModel ||
		projection.CredentialsRequired || projection.RealDependencyImports ||
		!projectionContract.ProviderFree || projectionContract.LiveNetwork || projectionContract.LiveModel ||
		projectionContract.CredentialsRequired || projectionContract.RealDependencyImports {
		t.Fatalf("approval projection must stay provider-free/offline: projection=%#v contract=%#v", projection, projectionContract)
	}
	if source.Kind != projection.Source.Kind || !source.ProviderFree || source.LiveNetwork || source.RealDependencyImports {
		t.Fatalf("approval source fixture is not compatible with projection source: source=%#v projection=%#v", source, projection.Source)
	}
	assertGenericTraceEventsSameStrings(t, "approval projection source paths", projectionContract.RequiredSourcePaths, []string{
		"trace_id",
		"fixture_key",
		"request.request_id",
		"request.tool",
		"request.capability",
		"request.risk_level",
		"decision.status",
		"decision.approval_id",
		"result.status",
		"result.secret_values_present",
		"replay.deterministic",
	})

	event := projection.ProjectedEvent
	if event.SchemaVersion != 1 ||
		!event.ProviderFree ||
		event.LiveNetwork ||
		event.RealDependencyImports ||
		event.TraceID != source.TraceID ||
		event.EventType != "approval" ||
		event.Capability != "ai.trace.approval" {
		t.Fatalf("projected trace event header invalid: event=%#v source=%#v", event, source)
	}
	if event.Correlation.TraceID != event.TraceID ||
		event.Correlation.ToolCallID != source.Request.RequestID ||
		event.Correlation.ApprovalID != projection.Fallbacks.ApprovalIDWhenMissing {
		t.Fatalf("projected trace event correlation invalid: event=%#v source=%#v projection=%#v", event.Correlation, source.Request, projection.Fallbacks)
	}
	if event.Redaction.SecretValuesPresent || source.Result.SecretValuesPresent ||
		event.Redaction.Status != "clean" ||
		len(event.Redaction.RedactedFields) != 0 {
		t.Fatalf("approval projection redaction is not clean/provider-free: event=%#v source=%#v", event.Redaction, source.Result)
	}
	if !event.Replay.Deterministic || event.Replay.LiveSink || !source.Replay.Deterministic || source.Replay.CreatedFromProvider {
		t.Fatalf("approval projection replay must be deterministic and offline: event=%#v source=%#v", event.Replay, source.Replay)
	}
	assertGenericTraceProjectionPayload(t, event.Payload, source, projection.Fallbacks.ApprovalIDWhenMissing)
}

func TestGenericTraceEventsToolInvocationProjection(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	manifest := loadGenericTraceEventsManifest(t, base)
	contract := loadGenericTraceEventsContract(t, base)
	var projection genericTraceEventsApprovalProjectionFixture
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "tool_invocation_projection_fixture.json"), &projection)

	sourcePath := filepath.Join(base, filepath.FromSlash(projection.Source.Fixture))
	source := loadGenericToolRegistryFixture(t, sourcePath)

	if manifest.Fixtures["tool_invocation_projection"] != "fixtures/tool_invocation_projection_fixture.json" ||
		manifest.Schemas["tool_invocation_projection"] != "schemas/tool_invocation_projection_v1.schema.json" {
		t.Fatalf("tool projection manifest entries missing: fixtures=%#v schemas=%#v", manifest.Fixtures, manifest.Schemas)
	}
	projectionContract := genericTraceEventsFindExternalProjection(contract, "generic-tool-invocation-to-ai-trace-tool")
	if projectionContract.ID == "" {
		t.Fatalf("missing tool invocation external projection contract: %#v", contract.ExternalEnvelopeProjections)
	}
	if projectionContract.SourceFixture != projection.Source.Fixture ||
		projectionContract.SourceSchema != projection.Source.Schema ||
		projectionContract.TargetFixture != manifest.Fixtures["tool_invocation_projection"] ||
		projectionContract.TargetSchema != projection.Target.Schema ||
		projectionContract.TargetEventType != projection.Target.EventType ||
		projectionContract.TargetCapability != projection.Target.Capability {
		t.Fatalf("tool projection manifest/contract/fixture disconnected: contract=%#v projection=%#v", projectionContract, projection)
	}
	if !projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork || projection.LiveModel ||
		projection.CredentialsRequired || projection.RealDependencyImports ||
		!projectionContract.ProviderFree || projectionContract.LiveNetwork || projectionContract.LiveModel ||
		projectionContract.CredentialsRequired || projectionContract.RealDependencyImports {
		t.Fatalf("tool projection must stay provider-free/offline: projection=%#v contract=%#v", projection, projectionContract)
	}
	assertGenericTraceEventsSameStrings(t, "tool projection source paths", projectionContract.RequiredSourcePaths, []string{
		"trace.trace_id",
		"trace.tool_name",
		"trace.caller_id",
		"trace.executor_id",
		"trace.capability_ids",
		"trace.events",
		"trace.schema_validation",
		"trace.approval.decision",
		"trace.result.status",
		"trace.result.content",
		"trace.provenance.fixture_key",
	})

	event := projection.ProjectedEvent
	if event.SchemaVersion != 1 ||
		!event.ProviderFree ||
		event.LiveNetwork ||
		event.RealDependencyImports ||
		event.TraceID != source.Trace.TraceID ||
		event.EventType != "tool" ||
		event.Capability != "ai.trace.tool" {
		t.Fatalf("projected tool trace event header invalid: event=%#v source=%#v", event, source.Trace)
	}
	if event.Correlation.TraceID != event.TraceID ||
		event.Correlation.ToolCallID == "" ||
		event.Payload["tool_call_id"] != event.Correlation.ToolCallID {
		t.Fatalf("projected tool trace correlation invalid: event=%#v payload=%#v", event.Correlation, event.Payload)
	}
	if event.Redaction.SecretValuesPresent ||
		event.Redaction.Status != "clean" ||
		len(event.Redaction.RedactedFields) != 0 ||
		!event.Replay.Deterministic ||
		event.Replay.LiveSink {
		t.Fatalf("tool projection redaction/replay invalid: redaction=%#v replay=%#v", event.Redaction, event.Replay)
	}
	assertGenericTraceToolProjectionPayload(t, event.Payload, source)
}

func TestGenericTraceEventsModelTurnReplayProjection(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	manifest := loadGenericTraceEventsManifest(t, base)
	contract := loadGenericTraceEventsContract(t, base)
	var projection genericTraceEventsModelTurnReplayProjectionFixture
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "model_turn_replay_trace_projection_fixture.json"), &projection)

	if manifest.Fixtures["model_turn_replay_trace_projection"] != "fixtures/model_turn_replay_trace_projection_fixture.json" ||
		manifest.Schemas["model_turn_replay_trace_projection"] != "schemas/model_turn_replay_trace_projection_v1.schema.json" {
		t.Fatalf("model turn replay projection manifest entries missing: fixtures=%#v schemas=%#v", manifest.Fixtures, manifest.Schemas)
	}
	projectionContract := genericTraceEventsFindExternalProjection(contract, "generic-model-turn-replay-to-trace-envelope")
	if projectionContract.ID == "" {
		t.Fatalf("missing model turn replay external projection contract: %#v", contract.ExternalEnvelopeProjections)
	}
	if projectionContract.TargetFixture != manifest.Fixtures["model_turn_replay_trace_projection"] ||
		projectionContract.TargetSchema != manifest.Schemas["model_turn_replay_trace_projection"] ||
		projectionContract.TargetCapability != "ai.trace.model_turn_replay_projection" {
		t.Fatalf("model turn replay projection manifest/contract disconnected: contract=%#v manifest=%#v", projectionContract, manifest.Fixtures)
	}
	if !projection.ProviderFree || projection.DomainSpecific || projection.LiveNetwork || projection.LiveModel ||
		projection.CredentialsRequired || projection.RealDependencyImports ||
		!projectionContract.ProviderFree || projectionContract.LiveNetwork || projectionContract.LiveModel ||
		projectionContract.CredentialsRequired || projectionContract.RealDependencyImports {
		t.Fatalf("model turn replay projection must stay provider-free/offline: projection=%#v contract=%#v", projection, projectionContract)
	}
	assertGenericTraceEventsSameStrings(t, "model turn replay target event types", projectionContract.TargetEventTypes, []string{"turn_start", "turn_end", "replay_record_matched"})
	assertGenericTraceEventsSameStrings(t, "model turn replay source packages", projectionContract.SourcePackages, []string{"generic-model-io-envelope", "generic-turn-runner", "generic-record-replay"})

	modelIO := decodeGenericTraceEventsMapFile(t, filepath.Join(base, filepath.FromSlash(projection.Sources["model_io"].Fixture)))
	turnExecution := decodeGenericTraceEventsMapFile(t, filepath.Join(base, filepath.FromSlash(projection.Sources["turn_execution"].Fixture)))
	turnReplay := decodeGenericTraceEventsMapFile(t, filepath.Join(base, filepath.FromSlash(projection.Sources["turn_replay_match"].Fixture)))
	recordReplay := decodeGenericTraceEventsMapFile(t, filepath.Join(base, filepath.FromSlash(projection.Sources["record_replay"].Fixture)))

	traceID := genericTraceEventsPathString(t, modelIO, "records.0.request.metadata.trace_id")
	turnID := genericTraceEventsPathString(t, modelIO, "records.0.request.turn_id")
	replaySessionID := genericTraceEventsPathString(t, modelIO, "replay.replay_session_id")
	responseStatus := genericTraceEventsPathString(t, modelIO, "records.0.response.status")
	turnRequestHash := genericTraceEventsPathString(t, turnExecution, "replay.request_hash")
	turnRecordID := genericTraceEventsPathString(t, turnExecution, "replay.record_id")
	turnReplayRecordID := genericTraceEventsPathString(t, turnReplay, "record_id")
	recordReplayRecordID := genericTraceEventsPathString(t, recordReplay, "records.0.record_id")
	recordReplayRequestHash := genericTraceEventsPathString(t, recordReplay, "records.0.request_hash")
	recordReplayResponseHash := genericTraceEventsPathString(t, recordReplay, "records.0.response_hash")

	if traceID == "" || turnID == "" || replaySessionID == "" || turnRequestHash == "" ||
		turnRecordID == "" || turnReplayRecordID == "" || recordReplayRecordID == "" {
		t.Fatalf("source fixture identity fields incomplete")
	}
	if turnRecordID != turnReplayRecordID {
		t.Fatalf("turn execution record_id %q != replay match record_id %q", turnRecordID, turnReplayRecordID)
	}
	if len(projection.CorrelationPolicy.SourceIDsAreNotAssumedEqual) == 0 ||
		!strings.Contains(projectionContract.IdentityPolicy, "no_cross_package_id_equality") {
		t.Fatalf("projection must explicitly avoid source ID equality assumptions: projection=%#v contract=%#v", projection.CorrelationPolicy, projectionContract.IdentityPolicy)
	}

	wantOrder := []string{"turn_start", "turn_end", "replay_record_matched"}
	seenIDs := map[string]bool{}
	for i, event := range projection.ProjectedEvents {
		if event.EventType != wantOrder[i] || event.TraceID != traceID || event.Sequence != i+1 || event.TimestampMS == 0 {
			t.Fatalf("projected event[%d] = %#v, want type %q trace %q seq %d", i, event, wantOrder[i], traceID, i+1)
		}
		if seenIDs[event.EventID] {
			t.Fatalf("duplicate projected event id %q", event.EventID)
		}
		seenIDs[event.EventID] = true
		if event.Correlation["turn_id"] != turnID || event.Correlation["replay_session_id"] != replaySessionID {
			t.Fatalf("%s correlation = %#v, want turn=%q replay=%q", event.EventID, event.Correlation, turnID, replaySessionID)
		}
	}
	turnStart := projection.ProjectedEvents[0]
	if turnStart.Payload["model_alias"] != genericTraceEventsPathString(t, modelIO, "records.0.request.model") ||
		turnStart.Payload["request_hash"] != turnRequestHash {
		t.Fatalf("turn_start payload does not explain model request/hash: %#v", turnStart.Payload)
	}
	turnEnd := projection.ProjectedEvents[1]
	if turnEnd.Payload["response_status"] != responseStatus ||
		turnEnd.Payload["turn_record_id"] != turnRecordID {
		t.Fatalf("turn_end payload does not explain response/turn record: %#v", turnEnd.Payload)
	}
	replayMatched := projection.ProjectedEvents[2]
	if replayMatched.Payload["turn_replay_record_id"] != turnReplayRecordID ||
		replayMatched.Payload["record_replay_record_id"] != recordReplayRecordID ||
		replayMatched.Payload["request_hash"] != turnRequestHash ||
		replayMatched.Payload["record_replay_request_hash"] != recordReplayRequestHash ||
		replayMatched.Payload["response_hash"] != recordReplayResponseHash ||
		replayMatched.Payload["deterministic"] != true {
		t.Fatalf("replay_record_matched payload does not explain replay identity: %#v", replayMatched.Payload)
	}
}

func TestGenericTraceEventsToolApprovalResultComposition(t *testing.T) {
	base := genericTraceEventsLivePackageDir(t)
	manifest := loadGenericTraceEventsManifest(t, base)
	contract := loadGenericTraceEventsContract(t, base)
	var composition genericTraceEventsToolApprovalResultCompositionFixture
	decodeGenericTraceEventsJSONFile(t, filepath.Join(base, "fixtures", "tool_approval_result_composition_fixture.json"), &composition)

	if manifest.Fixtures["tool_approval_result_composition"] != "fixtures/tool_approval_result_composition_fixture.json" ||
		manifest.Schemas["tool_approval_result_composition"] != "schemas/tool_approval_result_composition_v1.schema.json" {
		t.Fatalf("tool approval result composition manifest entries missing: fixtures=%#v schemas=%#v", manifest.Fixtures, manifest.Schemas)
	}
	projectionContract := genericTraceEventsFindExternalProjection(contract, "generic-tool-approval-result-composition")
	if projectionContract.ID == "" {
		t.Fatalf("missing tool approval result composition contract: %#v", contract.ExternalEnvelopeProjections)
	}
	if projectionContract.TargetFixture != manifest.Fixtures["tool_approval_result_composition"] ||
		projectionContract.TargetSchema != manifest.Schemas["tool_approval_result_composition"] ||
		projectionContract.TargetCapability != "ai.trace.tool_approval_result_composition" {
		t.Fatalf("tool approval result composition manifest/contract disconnected: contract=%#v manifest=%#v", projectionContract, manifest.Fixtures)
	}
	if !composition.ProviderFree || composition.DomainSpecific || composition.LiveNetwork || composition.LiveModel ||
		composition.CredentialsRequired || composition.RealDependencyImports ||
		!projectionContract.ProviderFree || projectionContract.LiveNetwork || projectionContract.LiveModel ||
		projectionContract.CredentialsRequired || projectionContract.RealDependencyImports {
		t.Fatalf("tool approval result composition must stay provider-free/offline: composition=%#v contract=%#v", composition, projectionContract)
	}
	assertGenericTraceEventsSameStrings(t, "tool approval result source packages", projectionContract.SourcePackages, []string{"generic-tool-registry-live-package", "generic-ai-approval-policy", "generic-tool-contracts-live-package"})
	assertGenericTraceEventsSameStrings(t, "tool approval result event types", projectionContract.TargetEventTypes, []string{"tool_call", "approval_replay_trace", "tool_result"})
	if composition.IdentityPolicy.CanonicalToolCallID == "" ||
		composition.IdentityPolicy.CanonicalToolName != "fixture.lookup" ||
		composition.IdentityPolicy.CanonicalCapability == "" ||
		composition.IdentityPolicy.ApprovalIDWhenMissing == "" ||
		len(composition.IdentityPolicy.SourceIDsAreNotAssumedEq) == 0 ||
		!strings.Contains(projectionContract.IdentityPolicy, "no_source_id_equality") {
		t.Fatalf("composition identity policy must use explicit canonical keys: identity=%#v contract=%q", composition.IdentityPolicy, projectionContract.IdentityPolicy)
	}
	for _, key := range []string{"tool_registry", "approval_policy", "tool_contracts"} {
		ref := composition.SourceRefs[key]
		if ref.PackageID == "" || ref.Fixture == "" || ref.Schema == "" || ref.Role == "" {
			t.Fatalf("composition source ref %q incomplete: %#v", key, ref)
		}
		assertGenericTraceEventsJSONFile(t, filepath.Join(base, filepath.FromSlash(ref.Fixture)))
	}
	registry := composition.SourceObservations["tool_registry"]
	approval := composition.SourceObservations["approval_policy"]
	toolContracts := composition.SourceObservations["tool_contracts"]
	if registry["approval_decision"] != "deny" || registry["clean_skip"] != true || registry["executed"] != false ||
		approval["decision"] != "denied" || approval["secret_values_present"] != false ||
		toolContracts["result_ok"] != false || toolContracts["result_error_kind"] != "validation" {
		t.Fatalf("source observations do not prove denied/validation path: registry=%#v approval=%#v tool_contracts=%#v", registry, approval, toolContracts)
	}

	wantOrder := []string{"tool_call", "approval_replay_trace", "tool_result"}
	seenIDs := map[string]bool{}
	for i, event := range composition.ProjectedEvents {
		if event.EventType != wantOrder[i] || event.Sequence != i+1 || event.TimestampMS == 0 || event.TraceID == "" || event.Status == "" {
			t.Fatalf("composition projected event[%d] = %#v, want type %q seq %d", i, event, wantOrder[i], i+1)
		}
		if seenIDs[event.EventID] {
			t.Fatalf("duplicate composition event id %q", event.EventID)
		}
		seenIDs[event.EventID] = true
		if event.Correlation["tool_call_id"] != composition.IdentityPolicy.CanonicalToolCallID {
			t.Fatalf("%s tool_call_id = %#v, want %q", event.EventID, event.Correlation, composition.IdentityPolicy.CanonicalToolCallID)
		}
	}
	toolCall := composition.ProjectedEvents[0]
	if toolCall.Payload["tool_call_id"] != composition.IdentityPolicy.CanonicalToolCallID ||
		toolCall.Payload["tool_name"] != composition.IdentityPolicy.CanonicalToolName ||
		toolCall.Payload["capability"] != composition.IdentityPolicy.CanonicalCapability ||
		toolCall.Payload["input_digest"] == "" {
		t.Fatalf("tool_call payload does not use canonical keys/digest: %#v", toolCall.Payload)
	}
	approvalEvent := composition.ProjectedEvents[1]
	if approvalEvent.Correlation["approval_id"] != composition.IdentityPolicy.ApprovalIDWhenMissing ||
		approvalEvent.Payload["decision"] != "denied" ||
		approvalEvent.Payload["source_approval_id"] != nil ||
		approvalEvent.Payload["secret_values_present"] != false {
		t.Fatalf("approval event payload/correlation invalid: correlation=%#v payload=%#v", approvalEvent.Correlation, approvalEvent.Payload)
	}
	resultEvent := composition.ProjectedEvents[2]
	if resultEvent.Payload["tool_name"] != composition.IdentityPolicy.CanonicalToolName ||
		resultEvent.Payload["result_ok"] != false ||
		resultEvent.Payload["result_error_kind"] != "validation" ||
		resultEvent.Payload["result_replay_key"] != toolContracts["result_replay_key"] ||
		resultEvent.Payload["output_digest"] == "" ||
		resultEvent.Payload["deterministic"] != true {
		t.Fatalf("tool_result payload invalid: %#v", resultEvent.Payload)
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
	manifest := loadGenericTraceEventsManifest(t, base)
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

	for _, result := range runFinRobotLivePackageSummarySmoke(
		t,
		filepath.Join(base, "main.leia"),
		"generic_trace_events_live_package_summary",
		"generic_trace_events_live_package",
		leia.LibString,
		"generic_trace_events_contract_valid",
		"generic_trace_events_live_package",
	) {
		if result.Globals["generic_trace_events_contract_valid"] != true {
			t.Fatalf("generic_trace_events_contract_valid = %#v", result.Globals["generic_trace_events_contract_valid"])
		}
		fields := result.Fields
		requireFinRobotSummaryFields(t, fields, "events", "envelope_events", "projections", "match_keys", "provider_free", "replay")
		if fields["events"] != "6" ||
			fields["envelope_events"] != "10" ||
			fields["projections"] != "4" ||
			fields["match_keys"] != strconv.Itoa(len(manifest.ReplayContract.MatchingKeys)) ||
			(fields["provider_free"] == "true") != manifest.ProviderFree ||
			fields["replay"] != manifest.ReplayContract.Mode {
			t.Fatalf("summary does not align with trace manifest: summary=%#v manifest=%#v", fields, manifest)
		}
		global, ok := result.Globals["generic_trace_events_live_package"].(map[string]any)
		if !ok {
			t.Fatalf("generic_trace_events_live_package = %T %#v, want map", result.Globals["generic_trace_events_live_package"], result.Globals["generic_trace_events_live_package"])
		}
		if global["provider_free"] != true ||
			global["live_network"] != false ||
			global["real_dependency_imports"] != false ||
			global["telemetry_sink_required"] != false ||
			global["clean_skip_without_sink"] != true ||
			global["default_mode"] != "fixture_replay" ||
			global["capability"] != "generic.ai.trace.events" ||
			global["emit_shape"] != "ai.trace.emit" {
			t.Fatalf("generic_trace_events_live_package boundary drifted: %#v", global)
		}
		sequenceMarker, ok := global["sequence_marker"].(map[string]any)
		if !ok ||
			sequenceMarker["name"] != "fixture_sequence" ||
			sequenceMarker["clock"] != "fixture_monotonic_ms" ||
			sequenceMarker["strictly_increasing"] != true {
			t.Fatalf("generic_trace_events_live_package sequence marker drifted: %#v", sequenceMarker)
		}
		redactionPolicy, ok := global["redaction_policy"].(map[string]any)
		if !ok ||
			redactionPolicy["default_action"] != "hash_or_drop" ||
			redactionPolicy["provider_request_id_policy"] != "drop" ||
			redactionPolicy["hash_algorithm"] != "sha256" {
			t.Fatalf("generic_trace_events_live_package redaction policy drifted: %#v", redactionPolicy)
		}
		replay, ok := global["replay"].(map[string]any)
		if !ok ||
			replay["mode"] != manifest.ReplayContract.Mode ||
			replay["fixture_key"] != "generic_trace_events:sequence:ACME:offline" ||
			replay["expected_event_count"] != int64(6) ||
			replay["expected_envelope_event_count"] != int64(10) ||
			replay["expected_projection_count"] != int64(4) ||
			replay["mismatch_policy"] != manifest.ReplayContract.MismatchPolicy ||
			replay["allow_live_sink"] != false {
			t.Fatalf("generic_trace_events_live_package replay contract drifted: %#v", replay)
		}
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

func decodeGenericTraceEventsMapFile(t *testing.T, path string) map[string]any {
	t.Helper()
	var out map[string]any
	decodeGenericTraceEventsJSONFile(t, path, &out)
	return out
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

func genericTraceEventsPathString(t *testing.T, value map[string]any, dottedPath string) string {
	t.Helper()
	current := any(value)
	for _, part := range strings.Split(dottedPath, ".") {
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[part]
			if !ok {
				t.Fatalf("path %q missing segment %q in %#v", dottedPath, part, node)
			}
			current = next
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node) {
				t.Fatalf("path %q has invalid index %q for %#v", dottedPath, part, node)
			}
			current = node[index]
		default:
			t.Fatalf("path %q cannot descend into %T %#v at %q", dottedPath, current, current, part)
		}
	}
	text, ok := current.(string)
	if !ok {
		t.Fatalf("path %q = %#v, want string", dottedPath, current)
	}
	return text
}

func assertGenericTraceEventsPackageHasNoRawLeak(t *testing.T, base string) {
	t.Helper()
	for _, rel := range []string{
		"main.leia",
		"package.manifest.json",
		"contracts/trace_events_contract.json",
		"fixtures/approval_trace_projection_fixture.json",
		"fixtures/model_turn_replay_trace_projection_fixture.json",
		"fixtures/provider_free_fixture_index.json",
		"fixtures/redaction_policy_fixture.json",
		"fixtures/tool_approval_result_composition_fixture.json",
		"fixtures/trace_envelope_fixture.json",
		"fixtures/tool_invocation_projection_fixture.json",
		"schemas/approval_trace_projection_v1.schema.json",
		"schemas/model_turn_replay_trace_projection_v1.schema.json",
		"schemas/tool_approval_result_composition_v1.schema.json",
		"schemas/tool_invocation_projection_v1.schema.json",
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

func genericTraceEventsFindExternalProjection(contract genericTraceEventsContract, id string) genericTraceEventsExternalProjection {
	for _, projection := range contract.ExternalEnvelopeProjections {
		if projection.ID == id {
			return projection
		}
	}
	return genericTraceEventsExternalProjection{}
}

func assertGenericTraceProjectionPayload(t *testing.T, payload map[string]any, source genericTraceEventsApprovalSourceFixture, fallbackApprovalID string) {
	t.Helper()
	wantStrings := map[string]string{
		"approval_id":        fallbackApprovalID,
		"operation":          source.Request.Tool,
		"decision":           source.Decision.Status,
		"policy_id":          source.Policy.Package,
		"capability":         source.Request.Capability,
		"risk_level":         source.Request.RiskLevel,
		"result_status":      source.Result.Status,
		"source_fixture_key": source.FixtureKey,
		"provider_name":      "generic",
	}
	for key, want := range wantStrings {
		if got, _ := payload[key].(string); got != want {
			t.Fatalf("projected payload[%s] = %#v, want %q; payload=%#v", key, payload[key], want, payload)
		}
	}
	if got, _ := payload["approval_required"].(bool); got != source.Decision.ApprovalRequired {
		t.Fatalf("projected payload approval_required = %#v, want %v; payload=%#v", payload["approval_required"], source.Decision.ApprovalRequired, payload)
	}
	if source.Decision.ApprovalID == nil {
		if value, ok := payload["source_approval_id"]; !ok || value != nil {
			t.Fatalf("projected payload source_approval_id = %#v, want null; payload=%#v", value, payload)
		}
	}
}

func assertGenericTraceToolProjectionPayload(t *testing.T, payload map[string]any, source genericToolRegistryFixture) {
	t.Helper()
	wantStrings := map[string]string{
		"tool_name":          source.Trace.ToolName,
		"caller_id":          source.Trace.CallerID,
		"executor_id":        source.Trace.ExecutorID,
		"approval_decision":  source.Trace.Approval.Decision,
		"status":             source.Trace.Result.Status,
		"text_preview":       source.Trace.Result.Content,
		"source_fixture_key": source.Trace.Provenance.FixtureKey,
		"provider_name":      "generic",
	}
	for key, want := range wantStrings {
		if got, _ := payload[key].(string); got != want {
			t.Fatalf("projected tool payload[%s] = %#v, want %q; payload=%#v", key, payload[key], want, payload)
		}
	}
	if payload["input_digest"] == "" || payload["output_digest"] == "" {
		t.Fatalf("projected tool payload must carry digests, not raw inputs: %#v", payload)
	}
	capabilities, ok := payload["capability_ids"].([]any)
	if !ok || len(capabilities) != len(source.Trace.CapabilityIDs) {
		t.Fatalf("projected tool payload capability_ids = %#v, want %d values", payload["capability_ids"], len(source.Trace.CapabilityIDs))
	}
	schemaValidation, ok := payload["schema_validation"].(map[string]any)
	if !ok ||
		schemaValidation["input_valid"] != source.Trace.SchemaValidation.InputValid ||
		schemaValidation["output_valid"] != source.Trace.SchemaValidation.OutputValid ||
		schemaValidation["additional_properties_rejected"] != source.Trace.SchemaValidation.AdditionalPropertiesRejected {
		t.Fatalf("projected tool schema validation = %#v, want source %#v", payload["schema_validation"], source.Trace.SchemaValidation)
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
