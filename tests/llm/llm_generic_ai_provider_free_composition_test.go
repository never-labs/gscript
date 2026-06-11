package leia_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericAIProviderFreeContractComposition(t *testing.T) {
	toolFixture := loadGenericCompositionMap(t,
		filepath.Join(genericToolContractsLivePackageDir(t), "fixtures", "invoke_success_fixture.json"))
	turnRequest := loadGenericCompositionMap(t,
		filepath.Join(genericTurnRunnerPackageDir(t), "fixtures", "generic_turn_request_fixture.json"))
	turnResponse := loadGenericCompositionMap(t,
		filepath.Join(genericTurnRunnerPackageDir(t), "fixtures", "ai_turn_execute_fixture.json"))
	turnToolRequest := loadGenericCompositionMap(t,
		filepath.Join(genericTurnRunnerPackageDir(t), "fixtures", "tool_request_fixture.json"))
	traceSequence := loadGenericCompositionMap(t,
		filepath.Join(genericTraceEventsLivePackageDir(t), "fixtures", "trace_sequence_ACME_fixture.json"))

	assertGenericCompositionToolEnvelope(t, toolFixture)
	assertGenericCompositionTurnEnvelope(t, turnRequest, turnResponse, turnToolRequest)
	assertGenericCompositionTraceEnvelope(t, traceSequence, turnRequest, turnToolRequest, toolFixture)

	turnRequestHash := canonicalGenericCompositionHash(t, turnRequest, "request_id")
	toolRequestHash := canonicalGenericCompositionHash(t, genericCompositionObject(t, toolFixture, "request"))
	turnResponseHash := canonicalGenericCompositionHash(t, turnResponse)
	toolResultHash := canonicalGenericCompositionHash(t, genericCompositionObject(t, toolFixture, "result"))

	index := loadGenericReplayIndex(t, genericRecordReplayLivePackageDir(t))
	records := []genericReplayRecord{
		newGenericCompositionRecord(
			"composition-rec-000-turn",
			0,
			genericCompositionString(t, genericCompositionObject(t, turnRequest, "replay"), "fixture_key"),
			"llm.complete",
			"generic.ai.completion",
			"sha256:"+turnRequestHash,
			"sha256:"+turnResponseHash,
		),
		newGenericCompositionRecord(
			"composition-rec-001-tool",
			1,
			genericCompositionString(t, genericCompositionObject(t, genericCompositionObject(t, toolFixture, "result"), "replay"), "replay_key"),
			"tool.call",
			"generic.ai.tool.invoke",
			"sha256:"+toolRequestHash,
			"sha256:"+toolResultHash,
		),
	}
	requests := []genericReplayRequest{
		{
			ReplayKey:   records[0].ReplayKey,
			Operation:   records[0].Operation,
			Capability:  records[0].Capability,
			RequestHash: records[0].RequestHash,
		},
		{
			ReplayKey:   records[1].ReplayKey,
			Operation:   records[1].Operation,
			Capability:  records[1].Capability,
			RequestHash: records[1].RequestHash,
		},
	}

	summary, findings := runGenericStrictOrderedReplay(index, records, requests)
	if summary.Matched != 2 || summary.Mismatches != 0 || summary.Unconsumed != 0 || len(findings) != 0 {
		t.Fatalf("generic record/replay did not consume composed provider-free envelopes: summary=%#v findings=%#v", summary, findings)
	}
	if !reflect.DeepEqual(summary.MatchedRecordIDs, []string{"composition-rec-000-turn", "composition-rec-001-tool"}) {
		t.Fatalf("matched record order = %#v", summary.MatchedRecordIDs)
	}
}

func TestGenericAITurnCanonicalizationForComposition(t *testing.T) {
	turnRequest := loadGenericCompositionMap(t,
		filepath.Join(genericTurnRunnerPackageDir(t), "fixtures", "generic_turn_request_fixture.json"))

	baseline := canonicalGenericCompositionHash(t, turnRequest, "request_id")

	renamed := cloneGenericCompositionMap(t, turnRequest)
	renamed["request_id"] = "turn-request-renamed-by-composition"
	if got := canonicalGenericCompositionHash(t, renamed, "request_id"); got != baseline {
		t.Fatalf("request_id must be excluded from generic turn replay hash: got %s want %s", got, baseline)
	}

	reversedMessages := cloneGenericCompositionMap(t, turnRequest)
	messages := genericCompositionSlice(t, reversedMessages, "messages")
	reversedMessages["messages"] = []any{messages[1], messages[0]}
	if got := canonicalGenericCompositionHash(t, reversedMessages, "request_id"); got == baseline {
		t.Fatalf("message order must be preserved in generic turn replay hash")
	}

	withoutTools := cloneGenericCompositionMap(t, turnRequest)
	withoutTools["tools"] = []any{}
	if got := canonicalGenericCompositionHash(t, withoutTools, "request_id"); got == baseline {
		t.Fatalf("tool declarations must participate in generic turn replay hash")
	}

	changedFormat := cloneGenericCompositionMap(t, turnRequest)
	changedFormat["response_format"] = map[string]any{"type": "text"}
	if got := canonicalGenericCompositionHash(t, changedFormat, "request_id"); got == baseline {
		t.Fatalf("response_format must participate in generic turn replay hash")
	}
}

func TestGenericAgentLoopCompositionGuardExplainsProviderFreeChain(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
			}, tc.opts...)...)
			if err := vm.ExecFile(filepath.Join(repoRoot(t), filepath.FromSlash(genericAgentLoopCompositionExample))); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			if err := vm.Exec(`
func generic_composition_has_tag(tags, want) {
    for i := 1; i <= #tags; i++ {
        if tags[i] == want {
            return true
        }
    }
    return false
}

probe_model_tags_ok := generic_composition_has_tag(generic_model_contract.capability_tags, "generic.ai.model.contract") && generic_composition_has_tag(generic_model_contract.capability_tags, "generic.ai.provider_free")
probe_agent_tags_ok := generic_composition_has_tag(generic_agent_runner.capability_tags, "generic.ai.agent.runner") && generic_composition_has_tag(generic_agent_runner.capability_tags, "generic.ai.policy.fixture_replay")
probe_turn_tags_ok := generic_composition_has_tag(generic_turn_runner.capability_tags, "generic.ai.turn.runner") && generic_composition_has_tag(agent_loop_result.turns[1].capability_tags, "generic.ai.tool.request")
probe_tool_tags_ok := generic_composition_has_tag(generic_tool_contracts.capability_tags, "generic.ai.tool.contract") && generic_composition_has_tag(agent_loop_result.turns[2].calls[1].capability_tags, "local.response.compose")
probe_trace_tags_ok := generic_composition_has_tag(generic_trace_events.capability_tags, "generic.ai.trace.events") && generic_composition_has_tag(generic_trace_events.capability_tags, "generic.ai.redaction.no_secret")
probe_replay_tags_ok := generic_composition_has_tag(generic_model_contract.capability_tags, "generic.ai.replay.fixture") && generic_composition_has_tag(generic_turn_runner.capability_tags, "generic.ai.replay.fixture")

probe_provider_free_chain_ok := generic_model_contract.provider_free == true && generic_agent_runner.provider_free == true && generic_turn_runner.provider_free == true && generic_tool_contracts.provider_free == true && generic_trace_events.provider_free == true && agent_loop_result.provider_free == true
probe_live_network_chain_ok := generic_model_contract.live_network == false && generic_agent_runner.live_network == false && generic_turn_runner.live_network == false && generic_tool_contracts.live_network == false && generic_trace_events.live_network == false && agent_loop_result.live_network == false
probe_real_imports_chain_ok := generic_model_contract.real_dependency_imports == false && generic_agent_runner.real_dependency_imports == false && generic_turn_runner.real_dependency_imports == false && generic_tool_contracts.real_dependency_imports == false && generic_trace_events.real_dependency_imports == false && agent_loop_result.real_dependency_imports == false
probe_redaction_chain_ok := generic_model_contract.redaction.secret_values_present == false && generic_agent_runner.redaction.secret_values_present == false && agent_loop_result.redaction.raw_prompt_stored == false && agent_loop_result.redaction.raw_completion_stored == false && generic_trace_events.events[1].redaction.secret_values_present == false
probe_policy_chain_ok := generic_model_contract.policy.mode == "fixture_replay" && generic_agent_runner.policy.mode == generic_model_contract.policy.mode && generic_agent_runner.policy.provider_credentials_required == false && generic_agent_runner.policy.live_model_calls == false && agent_loop_result.policy.secret_values_allowed == false
probe_replay_chain_ok := generic_model_contract.replay.match_key == "deterministic_contract_hash" && generic_agent_runner.replay.match_key == generic_model_contract.replay.match_key && agent_loop_result.replay.provider_free == true && agent_loop_result.replay.real_dependency_imports == false

probe_trace_correlation_chain_ok := generic_model_contract.correlation_id == generic_agent_runner.correlation_id && generic_agent_runner.correlation_id == generic_turn_runner.correlation_id && generic_turn_runner.correlation_id == generic_tool_contracts.correlation_id && generic_tool_contracts.correlation_id == generic_trace_events.correlation_id && generic_trace_events.correlation_id == agent_loop_result.correlation_id
probe_turn_tool_trace_correlation_ok := agent_loop_result.turns[1].correlation_id == agent_loop_result.turns[1].calls[1].correlation_id && agent_loop_result.turns[2].correlation_id == agent_loop_result.turns[2].calls[1].correlation_id && agent_loop_result.turns[1].calls[1].correlation_id == generic_trace_events.events[4].correlation.correlation_id && agent_loop_result.turns[2].calls[1].correlation_id == generic_trace_events.events[7].correlation.correlation_id && generic_trace_events.events[8].correlation.correlation_id == agent_loop_result.correlation_id
probe_trace_id_chain_ok := generic_model_contract.trace_id == generic_agent_runner.trace_id && generic_agent_runner.trace_id == generic_turn_runner.trace_id && generic_trace_events.events[1].correlation.trace_id == generic_trace_events.trace_id && generic_trace_events.events[8].correlation.trace_id == agent_loop_result.trace_id
probe_explainable_chain := generic_model_contract.id .. ">" .. generic_agent_runner.model_contract .. ">" .. generic_agent_runner.turn_runner .. ">" .. generic_agent_runner.tool_contracts .. ">" .. generic_agent_runner.trace_events
probe_event_components := generic_trace_events.events[1].component .. ">" .. generic_trace_events.events[2].component .. ">" .. generic_trace_events.events[4].component .. ">" .. generic_trace_events.events[8].component
`); err != nil {
				t.Fatalf("Exec probes: %v", err)
			}

			for name, want := range map[string]any{
				"probe_model_tags_ok":                  true,
				"probe_agent_tags_ok":                  true,
				"probe_turn_tags_ok":                   true,
				"probe_tool_tags_ok":                   true,
				"probe_trace_tags_ok":                  true,
				"probe_replay_tags_ok":                 true,
				"probe_provider_free_chain_ok":         true,
				"probe_live_network_chain_ok":          true,
				"probe_real_imports_chain_ok":          true,
				"probe_redaction_chain_ok":             true,
				"probe_policy_chain_ok":                true,
				"probe_replay_chain_ok":                true,
				"probe_trace_correlation_chain_ok":     true,
				"probe_turn_tool_trace_correlation_ok": true,
				"probe_trace_id_chain_ok":              true,
				"probe_explainable_chain":              "generic_model_contract_v1>generic_model_contract_v1>generic_turn_runner_v1>generic_tool_contracts_v1>generic_trace_events_v1",
				"probe_event_components":               "generic_agent_runner>generic_turn_runner>generic_tool_contracts>generic_agent_runner",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func assertGenericCompositionToolEnvelope(t *testing.T, fixture map[string]any) {
	t.Helper()
	request := genericCompositionObject(t, fixture, "request")
	result := genericCompositionObject(t, fixture, "result")
	replay := genericCompositionObject(t, result, "replay")

	if genericCompositionString(t, request, "tool_name") == "" ||
		genericCompositionString(t, result, "tool_name") != genericCompositionString(t, request, "tool_name") {
		t.Fatalf("tool invocation/result envelope names do not compose: request=%#v result=%#v", request, result)
	}
	if genericCompositionBool(t, result, "ok") != true ||
		genericCompositionString(t, replay, "replay_key") != genericCompositionString(t, request, "replay_key") ||
		genericCompositionBool(t, replay, "provider_free") != true ||
		genericCompositionBool(t, replay, "live_network") {
		t.Fatalf("tool result replay envelope is not provider-free/reusable: %#v", result)
	}
	for _, want := range []string{"generic.ai.tool.contract", "ai.tool.invoke", "ai.tool.result.envelope", "ai.tool.artifact.refs"} {
		if !genericCompositionContainsString(genericCompositionStringSlice(t, result, "capability_tags"), want) {
			t.Fatalf("tool result capability tags missing %q: %#v", want, result["capability_tags"])
		}
	}
}

func assertGenericCompositionTurnEnvelope(t *testing.T, request, response, toolRequest map[string]any) {
	t.Helper()
	requestReplay := genericCompositionObject(t, request, "replay")
	responseReplay := genericCompositionObject(t, response, "replay")
	metadata := genericCompositionObject(t, request, "metadata")
	policy := genericCompositionObject(t, toolRequest, "policy")

	if genericCompositionString(t, request, "schema") != "single_turn_request_v1" ||
		genericCompositionString(t, response, "schema") != "execute_response_envelope_v1" ||
		genericCompositionString(t, response, "request_id") != genericCompositionString(t, request, "request_id") {
		t.Fatalf("turn request/response envelope mismatch: request=%#v response=%#v", request, response)
	}
	if !genericCompositionBool(t, metadata, "provider_free") ||
		genericCompositionBool(t, metadata, "live_network") ||
		genericCompositionString(t, genericCompositionObject(t, response, "usage"), "source") != "fixture" {
		t.Fatalf("turn envelope must stay provider-free fixture-only: request=%#v response=%#v", request, response)
	}
	if genericCompositionString(t, requestReplay, "match_key") != "deterministic_request_hash" ||
		genericCompositionString(t, responseReplay, "request_hash") != genericCompositionString(t, requestReplay, "request_hash") {
		t.Fatalf("turn replay envelope does not carry deterministic match key/hash: request=%#v response=%#v", requestReplay, responseReplay)
	}
	if genericCompositionString(t, toolRequest, "schema") != "tool_request_envelope_v1" ||
		genericCompositionString(t, toolRequest, "status") != "requested" ||
		genericCompositionString(t, policy, "execution_policy") != "request-only-envelope" ||
		!genericCompositionBool(t, policy, "provider_free") ||
		genericCompositionBool(t, policy, "live_execution") {
		t.Fatalf("turn runner tool request is not reusable as a provider-free envelope: %#v", toolRequest)
	}
}

func assertGenericCompositionTraceEnvelope(t *testing.T, traceSequence, turnRequest, turnToolRequest, toolFixture map[string]any) {
	t.Helper()
	turnTools := genericCompositionSlice(t, turnRequest, "tools")
	if len(turnTools) != 1 {
		t.Fatalf("turn request must declare exactly one composed tool: %#v", turnTools)
	}
	turnToolDeclaration, ok := turnTools[0].(map[string]any)
	if !ok {
		t.Fatalf("turn tool declaration is not an object: %#v", turnTools[0])
	}
	toolEvent := genericCompositionTraceEvent(t, traceSequence, "tool")
	turnStartEvent := genericCompositionTraceEvent(t, traceSequence, "turn_start")
	turnStartPayload := genericCompositionObject(t, turnStartEvent, "payload")
	toolPayload := genericCompositionObject(t, toolEvent, "payload")
	toolCorrelation := genericCompositionObject(t, toolEvent, "correlation")
	toolRequestReplay := genericCompositionObject(t, turnToolRequest, "replay")
	toolResult := genericCompositionObject(t, toolFixture, "result")
	toolResultReplay := genericCompositionObject(t, toolResult, "replay")
	expectedToolRequestDigest := "sha256:" + canonicalGenericCompositionHash(t, genericCompositionObject(t, toolFixture, "request"))

	if genericCompositionString(t, turnToolDeclaration, "name") != genericCompositionString(t, turnToolRequest, "name") ||
		genericCompositionString(t, turnToolRequest, "name") != genericCompositionString(t, genericCompositionObject(t, toolFixture, "request"), "tool_name") ||
		genericCompositionString(t, toolResult, "tool_name") != genericCompositionString(t, turnToolRequest, "name") {
		t.Fatalf("turn runner and tool contract names do not compose: declaration=%#v request=%#v fixture=%#v", turnToolDeclaration, turnToolRequest, toolFixture)
	}
	if genericCompositionString(t, toolCorrelation, "tool_call_id") != genericCompositionString(t, turnToolRequest, "tool_call_id") ||
		genericCompositionString(t, toolPayload, "tool_call_id") != genericCompositionString(t, turnToolRequest, "tool_call_id") ||
		genericCompositionString(t, toolPayload, "tool_name") != genericCompositionString(t, turnToolRequest, "name") {
		t.Fatalf("trace tool event does not correlate with turn runner tool request: event=%#v request=%#v", toolEvent, turnToolRequest)
	}
	if !reflect.DeepEqual(genericCompositionStringSlice(t, turnStartPayload, "tools_declared"), []string{genericCompositionString(t, turnToolDeclaration, "name")}) {
		t.Fatalf("trace turn_start tools do not mirror turn request declarations: payload=%#v declaration=%#v", turnStartPayload, turnToolDeclaration)
	}
	if genericCompositionString(t, toolRequestReplay, "deterministic_tool_hash") != expectedToolRequestDigest ||
		genericCompositionString(t, toolPayload, "input_digest") != expectedToolRequestDigest ||
		genericCompositionString(t, toolPayload, "output_digest") != "sha256:"+canonicalGenericCompositionHash(t, toolResult) ||
		genericCompositionString(t, toolPayload, "replay_key") != genericCompositionString(t, toolResultReplay, "replay_key") {
		t.Fatalf("trace tool event must carry reusable request/result replay digests: payload=%#v request=%#v result=%#v", toolPayload, turnToolRequest, toolResult)
	}
}

func genericCompositionTraceEvent(t *testing.T, traceSequence map[string]any, eventType string) map[string]any {
	t.Helper()
	for _, rawEvent := range genericCompositionSlice(t, traceSequence, "events") {
		event, ok := rawEvent.(map[string]any)
		if !ok {
			t.Fatalf("trace event is not an object: %#v", rawEvent)
		}
		if genericCompositionString(t, event, "event_type") == eventType {
			return event
		}
	}
	t.Fatalf("trace sequence missing event_type %q", eventType)
	return nil
}

func newGenericCompositionRecord(recordID string, sequence int, replayKey, operation, capability, requestHash, responseHash string) genericReplayRecord {
	record := genericReplayRecord{
		RecordID:     recordID,
		Sequence:     sequence,
		ReplayKey:    replayKey,
		Operation:    operation,
		Capability:   capability,
		RequestHash:  requestHash,
		ResponseHash: responseHash,
		ProviderFree: true,
		RecordedAt:   "2026-01-02T03:04:05Z",
	}
	record.Metadata.Fixture = true
	record.Metadata.Deterministic = true
	return record
}

func canonicalGenericCompositionHash(t *testing.T, value any, excludedKeys ...string) string {
	t.Helper()
	canonical := canonicalGenericCompositionValue(value, excludedKeys)
	data, err := json.Marshal(canonical)
	if err != nil {
		t.Fatalf("marshal canonical value: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func canonicalGenericCompositionValue(value any, excludedKeys []string) any {
	excluded := map[string]bool{}
	for _, key := range excludedKeys {
		excluded[key] = true
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			if excluded[key] {
				continue
			}
			out[key] = canonicalGenericCompositionValue(nested, excludedKeys)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, nested := range typed {
			out[i] = canonicalGenericCompositionValue(nested, excludedKeys)
		}
		return out
	default:
		return typed
	}
}

func loadGenericCompositionMap(t *testing.T, path string) map[string]any {
	t.Helper()
	var value map[string]any
	decodeGenericRecordReplayJSON(t, path, &value)
	return value
}

func cloneGenericCompositionMap(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal clone source: %v", err)
	}
	var clone map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatalf("unmarshal clone: %v", err)
	}
	return clone
}

func genericCompositionObject(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	object, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("%q is not an object in %#v", key, value)
	}
	return object
}

func genericCompositionSlice(t *testing.T, value map[string]any, key string) []any {
	t.Helper()
	slice, ok := value[key].([]any)
	if !ok {
		t.Fatalf("%q is not an array in %#v", key, value)
	}
	return slice
}

func genericCompositionString(t *testing.T, value map[string]any, key string) string {
	t.Helper()
	got, ok := value[key].(string)
	if !ok {
		t.Fatalf("%q is not a string in %#v", key, value)
	}
	return got
}

func genericCompositionBool(t *testing.T, value map[string]any, key string) bool {
	t.Helper()
	got, ok := value[key].(bool)
	if !ok {
		t.Fatalf("%q is not a bool in %#v", key, value)
	}
	return got
}

func genericCompositionStringSlice(t *testing.T, value map[string]any, key string) []string {
	t.Helper()
	raw := genericCompositionSlice(t, value, key)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("%q contains non-string item %#v", key, item)
		}
		out = append(out, text)
	}
	return out
}

func genericCompositionContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
