package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericTurnRunnerManifest struct {
	SchemaVersion               int               `json:"schema_version"`
	ID                          string            `json:"id"`
	PackageName                 string            `json:"package_name"`
	ProviderFree                bool              `json:"provider_free"`
	LiveNetworkDefault          bool              `json:"live_network_default"`
	RealDependencyImportDefault bool              `json:"real_dependency_import_default"`
	Entrypoints                 map[string]string `json:"entrypoints"`
	Schemas                     map[string]string `json:"schemas"`
	Fixtures                    map[string]string `json:"fixtures"`
	Capabilities                []string          `json:"capabilities"`
	BlockedImports              []string          `json:"blocked_imports"`
	DefaultPolicy               struct {
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
	RequestContract struct {
		DialectCapability   string   `json:"dialect_capability"`
		Schema              string   `json:"schema"`
		TurnModel           string   `json:"turn_model"`
		RequiredFields      []string `json:"required_fields"`
		MessageRoles        []string `json:"message_roles"`
		ResponseFormatModes []string `json:"response_format_modes"`
		ProviderFree        bool     `json:"provider_free"`
	} `json:"request_contract"`
	ExecuteContract struct {
		DialectCapability      string   `json:"dialect_capability"`
		Schema                 string   `json:"schema"`
		ResponseEnvelopeFields []string `json:"response_envelope_fields"`
		UsageFields            []string `json:"usage_fields"`
		ErrorFields            []string `json:"error_fields"`
		ProviderFree           bool     `json:"provider_free"`
	} `json:"execute_contract"`
	ToolRequestContract struct {
		Schema           string   `json:"schema"`
		RequiredFields   []string `json:"required_fields"`
		ArgumentEncoding string   `json:"argument_encoding"`
		ExecutionPolicy  string   `json:"execution_policy"`
		LiveExecution    bool     `json:"live_execution"`
	} `json:"tool_request_contract"`
	ReplayMatchContract struct {
		Schema                      string   `json:"schema"`
		MatchKey                    string   `json:"match_key"`
		Canonicalization            []string `json:"canonicalization"`
		MissBehavior                string   `json:"miss_behavior"`
		ProviderCredentialsRequired bool     `json:"provider_credentials_required"`
		LiveNetwork                 bool     `json:"live_network"`
	} `json:"replay_match_contract"`
	MemoryContextProjectionContract struct {
		DialectCapability          string `json:"dialect_capability"`
		SourcePackage              string `json:"source_package"`
		SourceFixture              string `json:"source_fixture"`
		TargetFixture              string `json:"target_fixture"`
		TargetField                string `json:"target_field"`
		ProjectionPolicy           string `json:"projection_policy"`
		RawMemoryTextAllowed       bool   `json:"raw_memory_text_allowed"`
		SecretValuesAllowed        bool   `json:"secret_values_allowed"`
		DeterministicOrderRequired bool   `json:"deterministic_order_required"`
	} `json:"memory_context_projection_contract"`
	NoBuiltInGuarantee struct {
		Required  bool   `json:"required"`
		Statement string `json:"statement"`
	} `json:"no_built_in_guarantee"`
}

func TestGenericTurnRunnerManifestContracts(t *testing.T) {
	base := genericTurnRunnerPackageDir(t)
	manifest := loadGenericTurnRunnerManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-generic-turn-runner-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-generic-ai-turn-runner" {
		t.Fatalf("package_name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency {
		t.Fatalf("default policy must stay fixture-only: %#v", manifest.DefaultPolicy)
	}
	if manifest.ProviderGate.AllowNetwork ||
		manifest.ProviderGate.ProviderSDKsRequired ||
		len(manifest.ProviderGate.RequiredCredentials) != 0 ||
		len(manifest.ProviderGate.OptionalCredentials) != 0 {
		t.Fatalf("provider gate must stay closed: %#v", manifest.ProviderGate)
	}
	for _, want := range []string{"generic.ai.turn.request", "ai.turn.execute", "generic.ai.turn.memory_context_projection"} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if !manifest.NoBuiltInGuarantee.Required ||
		!strings.Contains(manifest.NoBuiltInGuarantee.Statement, "generic AI provider execution") {
		t.Fatalf("no_built_in_guarantee inconsistent: %#v", manifest.NoBuiltInGuarantee)
	}
	if manifest.RequestContract.DialectCapability != "generic.ai.turn.request" ||
		manifest.RequestContract.Schema != "single_turn_request_v1" ||
		manifest.RequestContract.TurnModel != "single_turn" ||
		!manifest.RequestContract.ProviderFree {
		t.Fatalf("request contract incomplete: %#v", manifest.RequestContract)
	}
	for _, want := range []string{"request_id", "correlation_id", "model", "messages", "response_format", "replay"} {
		if !contains(manifest.RequestContract.RequiredFields, want) {
			t.Fatalf("request contract missing required field %q: %#v", want, manifest.RequestContract.RequiredFields)
		}
	}
	for _, want := range []string{"system", "user", "assistant", "tool"} {
		if !contains(manifest.RequestContract.MessageRoles, want) {
			t.Fatalf("request message roles missing %q: %#v", want, manifest.RequestContract.MessageRoles)
		}
	}
	for _, want := range []string{"text", "json_object", "json_schema"} {
		if !contains(manifest.RequestContract.ResponseFormatModes, want) {
			t.Fatalf("response_format modes missing %q: %#v", want, manifest.RequestContract.ResponseFormatModes)
		}
	}
	if manifest.ExecuteContract.DialectCapability != "ai.turn.execute" ||
		manifest.ExecuteContract.Schema != "execute_response_envelope_v1" ||
		!manifest.ExecuteContract.ProviderFree {
		t.Fatalf("execute contract incomplete: %#v", manifest.ExecuteContract)
	}
	for _, want := range []string{"status", "request_id", "correlation_id", "message", "response_format", "usage", "error", "tool_requests", "replay"} {
		if !contains(manifest.ExecuteContract.ResponseEnvelopeFields, want) {
			t.Fatalf("execute envelope missing %q: %#v", want, manifest.ExecuteContract.ResponseEnvelopeFields)
		}
	}
	for _, want := range []string{"input_tokens", "output_tokens", "total_tokens", "source"} {
		if !contains(manifest.ExecuteContract.UsageFields, want) {
			t.Fatalf("usage envelope missing %q: %#v", want, manifest.ExecuteContract.UsageFields)
		}
	}
	for _, want := range []string{"code", "message", "retryable", "provider_free", "live_network", "credentials_required", "no_secret", "request_hash", "correlation_id"} {
		if !contains(manifest.ExecuteContract.ErrorFields, want) {
			t.Fatalf("error envelope missing %q: %#v", want, manifest.ExecuteContract.ErrorFields)
		}
	}
	if manifest.ToolRequestContract.Schema != "tool_request_envelope_v1" ||
		manifest.ToolRequestContract.ArgumentEncoding != "json_object" ||
		manifest.ToolRequestContract.ExecutionPolicy != "request-only-envelope" ||
		manifest.ToolRequestContract.LiveExecution {
		t.Fatalf("tool request contract incomplete: %#v", manifest.ToolRequestContract)
	}
	if manifest.ReplayMatchContract.Schema != "replay_record_match_v1" ||
		manifest.ReplayMatchContract.MatchKey != "deterministic_request_hash" ||
		manifest.ReplayMatchContract.MissBehavior != "clean-skip" ||
		manifest.ReplayMatchContract.ProviderCredentialsRequired ||
		manifest.ReplayMatchContract.LiveNetwork {
		t.Fatalf("replay match contract incomplete: %#v", manifest.ReplayMatchContract)
	}
	for _, want := range []string{"request_id excluded from hash", "messages preserve order", "object keys sorted", "response_format included", "tool declarations included"} {
		if !contains(manifest.ReplayMatchContract.Canonicalization, want) {
			t.Fatalf("replay canonicalization missing %q: %#v", want, manifest.ReplayMatchContract.Canonicalization)
		}
	}
	if manifest.MemoryContextProjectionContract.DialectCapability != "generic.ai.turn.memory_context_projection" ||
		manifest.MemoryContextProjectionContract.SourcePackage != "generic_memory_store" ||
		manifest.MemoryContextProjectionContract.TargetField != "messages" ||
		manifest.MemoryContextProjectionContract.ProjectionPolicy != "context_ref_message" ||
		manifest.MemoryContextProjectionContract.RawMemoryTextAllowed ||
		manifest.MemoryContextProjectionContract.SecretValuesAllowed ||
		!manifest.MemoryContextProjectionContract.DeterministicOrderRequired {
		t.Fatalf("memory context projection contract incomplete: %#v", manifest.MemoryContextProjectionContract)
	}
}

func TestGenericTurnRunnerSchemasFixturesAndContract(t *testing.T) {
	base := genericTurnRunnerPackageDir(t)
	manifest := loadGenericTurnRunnerManifest(t, base)

	for _, key := range []string{"smoke", "contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericTurnRunnerJSONOrLeiaFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"turn_request", "execute_response", "tool_request", "replay_record", "error", "memory_context_turn_projection"} {
		if manifest.Schemas[key] == "" {
			t.Fatalf("missing schema %q", key)
		}
		var schema struct {
			SchemaVersion int      `json:"schema_version"`
			ID            string   `json:"id"`
			ProviderFree  bool     `json:"provider_free"`
			LiveNetwork   bool     `json:"live_network"`
			Required      []string `json:"required"`
		}
		decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, manifest.Schemas[key]), &schema)
		if schema.SchemaVersion != 1 || schema.ID == "" || !schema.ProviderFree || schema.LiveNetwork {
			t.Fatalf("%s schema header invalid: %#v", key, schema)
		}
		if len(schema.Required) == 0 {
			t.Fatalf("%s schema lacks required fields", key)
		}
	}
	for _, key := range []string{"index", "turn_request", "execute_response", "tool_request", "replay_record", "error", "memory_context_turn_projection"} {
		if manifest.Fixtures[key] == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertGenericTurnRunnerJSONFile(t, filepath.Join(base, manifest.Fixtures[key]))
	}

	var contract struct {
		ProviderFree                    bool           `json:"provider_free"`
		LiveNetwork                     bool           `json:"live_network"`
		MessageContract                 map[string]any `json:"message_contract"`
		ResponseFormatContract          map[string]any `json:"response_format_contract"`
		UsageContract                   map[string]any `json:"usage_contract"`
		ErrorContract                   map[string]any `json:"error_contract"`
		ToolRequestContract             map[string]any `json:"tool_request_contract"`
		ReplayRecordMatchContract       map[string]any `json:"replay_record_match_contract"`
		MemoryContextProjectionContract map[string]any `json:"memory_context_projection_contract"`
	}
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, manifest.Entrypoints["contract"]), &contract)
	if !contract.ProviderFree || contract.LiveNetwork {
		t.Fatalf("contract must stay provider-free/offline: %#v", contract)
	}
	for name, section := range map[string]map[string]any{
		"message_contract":                   contract.MessageContract,
		"response_format_contract":           contract.ResponseFormatContract,
		"usage_contract":                     contract.UsageContract,
		"error_contract":                     contract.ErrorContract,
		"tool_request_contract":              contract.ToolRequestContract,
		"replay_record_match_contract":       contract.ReplayRecordMatchContract,
		"memory_context_projection_contract": contract.MemoryContextProjectionContract,
	} {
		if section["provider_free"] != true {
			t.Fatalf("%s must declare provider_free: %#v", name, section)
		}
	}

	var fixtureIndex struct {
		ProviderFree bool             `json:"provider_free"`
		LiveNetwork  bool             `json:"live_network"`
		Fixtures     []map[string]any `json:"fixtures"`
	}
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]), &fixtureIndex)
	if !fixtureIndex.ProviderFree || fixtureIndex.LiveNetwork {
		t.Fatalf("fixture index must stay provider-free/offline: %#v", fixtureIndex)
	}
	wantSchemas := map[string]bool{
		"single_turn_request_v1":            false,
		"execute_response_envelope_v1":      false,
		"tool_request_envelope_v1":          false,
		"replay_record_match_v1":            false,
		"error_envelope_v1":                 false,
		"memory_context_turn_projection_v1": false,
	}
	for _, fixture := range fixtureIndex.Fixtures {
		schema, _ := fixture["schema"].(string)
		if _, ok := wantSchemas[schema]; !ok {
			continue
		}
		wantSchemas[schema] = true
		requireGenericTurnRunnerGuardEvidence(t, "fixture index "+schema, fixture, "corr-generic-turn-acme-summary-001", "sha256:40c43cc2c05232c0dc659cf6e266d046905b01b383cccfd952d9d9abcabd05ba")
		metadata, _ := fixture["metadata"].(map[string]any)
		requireGenericTurnRunnerGuardEvidence(t, "fixture index metadata "+schema, metadata, "corr-generic-turn-acme-summary-001", "sha256:40c43cc2c05232c0dc659cf6e266d046905b01b383cccfd952d9d9abcabd05ba")
		assertGenericTurnRunnerJSONFile(t, filepath.Join(base, fixture["path"].(string)))
	}
	for schema, seen := range wantSchemas {
		if !seen {
			t.Fatalf("fixture index does not cover core envelope schema %q: %#v", schema, fixtureIndex.Fixtures)
		}
	}
}

func TestGenericTurnRunnerEnvelopeFixtures(t *testing.T) {
	base := genericTurnRunnerPackageDir(t)

	var request struct {
		Schema         string           `json:"schema"`
		RequestID      string           `json:"request_id"`
		CorrelationID  string           `json:"correlation_id"`
		Model          string           `json:"model"`
		Messages       []map[string]any `json:"messages"`
		ResponseFormat map[string]any   `json:"response_format"`
		Tools          []map[string]any `json:"tools"`
		Metadata       map[string]any   `json:"metadata"`
		Replay         map[string]any   `json:"replay"`
	}
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, "fixtures", "generic_turn_request_fixture.json"), &request)
	if request.Schema != "single_turn_request_v1" || request.RequestID == "" || request.CorrelationID == "" || request.Model == "" || len(request.Messages) != 2 {
		t.Fatalf("request fixture incomplete: %#v", request)
	}
	if request.ResponseFormat["type"] != "json_schema" || len(request.Tools) != 1 || request.Replay["match_key"] != "deterministic_request_hash" {
		t.Fatalf("request response_format/tools/replay incomplete: %#v", request)
	}
	requestHash, _ := request.Replay["request_hash"].(string)
	requireGenericTurnRunnerStableHash(t, "request replay request_hash", requestHash)
	requireGenericTurnRunnerGuardEvidence(t, "request metadata", request.Metadata, request.CorrelationID, "")

	var response struct {
		Schema         string           `json:"schema"`
		RequestID      string           `json:"request_id"`
		CorrelationID  string           `json:"correlation_id"`
		Status         string           `json:"status"`
		Message        map[string]any   `json:"message"`
		ResponseFormat map[string]any   `json:"response_format"`
		Usage          map[string]any   `json:"usage"`
		Error          map[string]any   `json:"error"`
		ToolRequests   []map[string]any `json:"tool_requests"`
		Replay         map[string]any   `json:"replay"`
	}
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, "fixtures", "ai_turn_execute_fixture.json"), &response)
	if response.Schema != "execute_response_envelope_v1" || response.RequestID != request.RequestID || response.CorrelationID != request.CorrelationID || response.Status != "ok" {
		t.Fatalf("execute fixture header incomplete: %#v", response)
	}
	if response.Message["role"] != "assistant" || response.ResponseFormat["type"] != "json_schema" {
		t.Fatalf("execute message/response_format incomplete: %#v", response)
	}
	if response.Usage["source"] != "fixture" || response.Usage["total_tokens"].(float64) != response.Usage["input_tokens"].(float64)+response.Usage["output_tokens"].(float64) {
		t.Fatalf("usage envelope invalid: %#v", response.Usage)
	}
	if response.Error["provider_free"] != true || response.Error["retryable"] != false {
		t.Fatalf("error envelope invalid: %#v", response.Error)
	}
	if response.Replay["match_key"] != request.Replay["match_key"] || response.Replay["request_hash"] != request.Replay["request_hash"] {
		t.Fatalf("replay envelope does not match request: response=%#v request=%#v", response.Replay, request.Replay)
	}
	requireGenericTurnRunnerGuardEvidence(t, "execute replay", response.Replay, request.CorrelationID, requestHash)
	requireGenericTurnRunnerGuardEvidence(t, "execute error", response.Error, request.CorrelationID, requestHash)

	var tool struct {
		Schema        string         `json:"schema"`
		ToolCallID    string         `json:"tool_call_id"`
		CorrelationID string         `json:"correlation_id"`
		Name          string         `json:"name"`
		Arguments     map[string]any `json:"arguments"`
		Status        string         `json:"status"`
		Replay        map[string]any `json:"replay"`
		Policy        map[string]any `json:"policy"`
	}
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, "fixtures", "tool_request_fixture.json"), &tool)
	if tool.Schema != "tool_request_envelope_v1" || tool.ToolCallID == "" || tool.CorrelationID != request.CorrelationID || tool.Name != "fixture.lookup" ||
		tool.Arguments["ticker"] != "ACME" || tool.Arguments["horizon"] != "1d" {
		t.Fatalf("tool request fixture incomplete: %#v", tool)
	}
	if tool.Status != "requested" || tool.Policy["execution_policy"] != "request-only-envelope" || tool.Policy["live_execution"] != false || tool.Policy["provider_free"] != true {
		t.Fatalf("tool request policy invalid: %#v", tool)
	}
	requireGenericTurnRunnerGuardEvidence(t, "tool replay", tool.Replay, request.CorrelationID, requestHash)
	requireGenericTurnRunnerGuardEvidence(t, "tool policy", tool.Policy, "", "")

	var replay struct {
		Schema        string         `json:"schema"`
		RecordID      string         `json:"record_id"`
		CorrelationID string         `json:"correlation_id"`
		MatchKey      string         `json:"match_key"`
		RequestHash   string         `json:"request_hash"`
		Request       map[string]any `json:"request"`
		Response      map[string]any `json:"response"`
		MatchContract map[string]any `json:"match_contract"`
	}
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, "fixtures", "replay_record_match_fixture.json"), &replay)
	if replay.Schema != "replay_record_match_v1" || replay.RecordID == "" || replay.CorrelationID != request.CorrelationID || replay.MatchKey != "deterministic_request_hash" || replay.RequestHash != request.Replay["request_hash"] {
		t.Fatalf("replay fixture header invalid: %#v", replay)
	}
	if replay.MatchContract["miss_behavior"] != "clean-skip" || replay.MatchContract["provider_free"] != true || replay.MatchContract["live_network"] != false {
		t.Fatalf("replay match contract invalid: %#v", replay.MatchContract)
	}
	requireGenericTurnRunnerGuardEvidence(t, "replay match contract", replay.MatchContract, "", "")

	var errorEnvelope map[string]any
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, "fixtures", "error_envelope_fixture.json"), &errorEnvelope)
	if errorEnvelope["schema"] != "error_envelope_v1" || errorEnvelope["code"] != "replay_miss" || errorEnvelope["retryable"] != false {
		t.Fatalf("error fixture invalid: %#v", errorEnvelope)
	}
	requireGenericTurnRunnerGuardEvidence(t, "error fixture", errorEnvelope, request.CorrelationID, requestHash)
}

func TestGenericTurnRunnerMemoryContextProjection(t *testing.T) {
	base := genericTurnRunnerPackageDir(t)
	root := repoRoot(t)
	memory := loadGenericMemoryStoreFixture(t, filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "generic_memory_store", "fixtures", "generic_memory_store_fixture.json"))

	var request struct {
		RequestID     string           `json:"request_id"`
		CorrelationID string           `json:"correlation_id"`
		Messages      []map[string]any `json:"messages"`
		Replay        map[string]any   `json:"replay"`
	}
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, "fixtures", "generic_turn_request_fixture.json"), &request)

	var projection struct {
		SchemaVersion         int    `json:"schema_version"`
		FixtureKey            string `json:"fixture_key"`
		ProjectionKind        string `json:"projection_kind"`
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		CredentialsRequired   bool   `json:"credentials_required"`
		RawMemoryTextAllowed  bool   `json:"raw_memory_text_allowed"`
		SecretValuesAllowed   bool   `json:"secret_values_allowed"`
		SourceFixtureRefs     struct {
			MemoryStore string `json:"memory_store"`
			TurnRequest string `json:"turn_request"`
		} `json:"source_fixture_refs"`
		TargetRequest struct {
			RequestID     string `json:"request_id"`
			CorrelationID string `json:"correlation_id"`
			RequestHash   string `json:"request_hash"`
			TargetField   string `json:"target_field"`
		} `json:"target_request"`
		ProjectedMessages []struct {
			Role     string `json:"role"`
			Content  string `json:"content"`
			Metadata struct {
				MessageKind           string   `json:"message_kind"`
				SourceFixtureKey      string   `json:"source_fixture_key"`
				ContextWindowStrategy string   `json:"context_window_strategy"`
				IncludedMemoryIDs     []string `json:"included_memory_ids"`
				ContextRefs           []string `json:"context_refs"`
				RawMemoryTextStored   bool     `json:"raw_memory_text_stored"`
				SecretValuesPresent   bool     `json:"secret_values_present"`
				DeterministicOrder    bool     `json:"deterministic_order"`
			} `json:"metadata"`
		} `json:"projected_messages"`
		ContextRefMappings []struct {
			MemoryID           string `json:"memory_id"`
			Rank               int    `json:"rank"`
			ContextRef         string `json:"context_ref"`
			TargetMessageIndex int    `json:"target_message_index"`
			ProjectionPolicy   string `json:"projection_policy"`
		} `json:"context_ref_mappings"`
		ProjectionAssertions map[string]bool `json:"projection_assertions"`
	}
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, "fixtures", "memory_context_turn_projection_fixture.json"), &projection)
	if projection.SchemaVersion != 1 ||
		projection.FixtureKey != "generic_turn:memory_context_projection:offline:v1" ||
		projection.ProjectionKind != "memory_context_to_turn_request_projection" ||
		projection.SourceFixtureRefs.MemoryStore == "" ||
		projection.SourceFixtureRefs.TurnRequest == "" {
		t.Fatalf("projection header/source refs invalid: %#v", projection)
	}
	if !projection.ProviderFree || projection.LiveNetwork || projection.RealDependencyImports ||
		projection.CredentialsRequired || projection.RawMemoryTextAllowed || projection.SecretValuesAllowed {
		t.Fatalf("projection must stay provider-free/offline: %#v", projection)
	}
	requestHash, _ := request.Replay["request_hash"].(string)
	if projection.TargetRequest.RequestID != request.RequestID ||
		projection.TargetRequest.CorrelationID != request.CorrelationID ||
		projection.TargetRequest.RequestHash != requestHash ||
		projection.TargetRequest.TargetField != "messages" {
		t.Fatalf("projection target request does not resolve: projection=%#v request=%#v", projection.TargetRequest, request)
	}
	if len(projection.ProjectedMessages) != 1 {
		t.Fatalf("projected messages = %d, want 1", len(projection.ProjectedMessages))
	}
	message := projection.ProjectedMessages[0]
	if message.Role != "system" ||
		message.Metadata.MessageKind != "memory_context_refs" ||
		message.Metadata.SourceFixtureKey != memory.Provenance.FixtureKey ||
		message.Metadata.ContextWindowStrategy != memory.ContextWindow.Strategy ||
		message.Metadata.RawMemoryTextStored ||
		message.Metadata.SecretValuesPresent ||
		!message.Metadata.DeterministicOrder ||
		!sameStringSet(message.Metadata.IncludedMemoryIDs, memory.ContextWindow.IncludedMemoryIDs) {
		t.Fatalf("projected memory context message invalid: %#v", message)
	}
	for _, item := range memory.MemoryItems {
		if strings.Contains(message.Content, item.Text) {
			t.Fatalf("projected message copied raw memory text %q: %#v", item.MemoryID, message)
		}
	}

	retrievalByMemory := map[string]struct {
		Rank       int
		ContextRef string
	}{}
	for _, result := range memory.RetrievalResults {
		retrievalByMemory[result.MemoryID] = struct {
			Rank       int
			ContextRef string
		}{Rank: result.Rank, ContextRef: result.ContextRef}
	}
	lastRank := 0
	for _, mapping := range projection.ContextRefMappings {
		retrieval, ok := retrievalByMemory[mapping.MemoryID]
		if !ok ||
			mapping.Rank <= lastRank ||
			mapping.Rank != retrieval.Rank ||
			mapping.ContextRef != retrieval.ContextRef ||
			mapping.TargetMessageIndex != 0 ||
			mapping.ProjectionPolicy != "context_ref_only" {
			t.Fatalf("context ref mapping invalid: mapping=%#v retrieval=%#v", mapping, retrieval)
		}
		lastRank = mapping.Rank
	}
	for _, want := range []string{"projection_is_provider_free", "turn_request_identity_resolves", "context_refs_match_memory_retrieval_results", "included_memory_ids_match_context_window", "message_order_is_deterministic", "raw_memory_text_absent", "secret_values_absent"} {
		if !projection.ProjectionAssertions[want] {
			t.Fatalf("projection assertion missing %q: %#v", want, projection.ProjectionAssertions)
		}
	}
}

func TestGenericTurnRunnerSmokeAndNoProviderDependencies(t *testing.T) {
	base := genericTurnRunnerPackageDir(t)
	manifest := loadGenericTurnRunnerManifest(t, base)

	vm := leia.New(leia.WithLibs(leia.LibString|leia.LibLLM|leia.LibDialect), leia.WithVM())
	if err := vm.ExecFile(filepath.Join(base, "main.leia")); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}
	got, err := vm.Get("generic_turn_runner_live_package_summary")
	if err != nil {
		t.Fatalf("Get summary: %v", err)
	}
	want := "generic_turn_runner_live_package provider_free=true live_network=false capabilities=3 schemas=6 fixtures=6 replay_match=deterministic_request_hash memory_context_projection=true"
	if got != want {
		t.Fatalf("summary = %#v, want %#v", got, want)
	}

	var files []string
	for _, rel := range manifest.Entrypoints {
		files = append(files, filepath.Join(base, rel))
	}
	for _, rel := range manifest.Schemas {
		files = append(files, filepath.Join(base, rel))
	}
	for _, rel := range manifest.Fixtures {
		files = append(files, filepath.Join(base, rel))
	}
	files = append(files, filepath.Join(base, "package.manifest.json"))

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
				t.Fatalf("%s appears to import blocked dependency %q", path, blocked)
			}
		}
		if strings.Contains(text, "https://") || strings.Contains(text, "http://") {
			t.Fatalf("%s contains live network locator", path)
		}
	}
}

func genericTurnRunnerPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_turn_runner")
}

func loadGenericTurnRunnerManifest(t *testing.T, base string) genericTurnRunnerManifest {
	t.Helper()
	var manifest genericTurnRunnerManifest
	decodeGenericTurnRunnerJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertGenericTurnRunnerJSONOrLeiaFile(t *testing.T, path string) {
	t.Helper()
	if strings.HasSuffix(path, ".json") {
		assertGenericTurnRunnerJSONFile(t, path)
		return
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertGenericTurnRunnerJSONFile(t *testing.T, path string) {
	t.Helper()
	var v any
	decodeGenericTurnRunnerJSONFile(t, path, &v)
}

func decodeGenericTurnRunnerJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func requireGenericTurnRunnerGuardEvidence(t *testing.T, name string, got map[string]any, wantCorrelationID, wantRequestHash string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s missing guard evidence", name)
	}
	if got["provider_free"] != true || got["live_network"] != false || got["credentials_required"] != false || got["no_secret"] != true {
		t.Fatalf("%s must be provider-free/offline/no-secret: %#v", name, got)
	}
	if patterns, ok := got["secret_env_patterns"].([]any); ok && len(patterns) != 0 {
		t.Fatalf("%s must not declare secret env patterns: %#v", name, patterns)
	}
	if wantCorrelationID != "" && got["correlation_id"] != wantCorrelationID {
		t.Fatalf("%s correlation_id = %#v, want %#v", name, got["correlation_id"], wantCorrelationID)
	}
	if wantRequestHash != "" && got["request_hash"] != wantRequestHash {
		t.Fatalf("%s request_hash = %#v, want %#v", name, got["request_hash"], wantRequestHash)
	}
	if hash, _ := got["request_hash"].(string); hash != "" {
		requireGenericTurnRunnerStableHash(t, name+" request_hash", hash)
	}
}

func requireGenericTurnRunnerStableHash(t *testing.T, name, hash string) {
	t.Helper()
	if !strings.HasPrefix(hash, "sha256:") || len(hash) != len("sha256:")+64 {
		t.Fatalf("%s must be stable sha256 hash, got %q", name, hash)
	}
	for _, r := range strings.TrimPrefix(hash, "sha256:") {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("%s must use lowercase hex sha256, got %q", name, hash)
		}
	}
}
