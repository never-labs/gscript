package leia_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
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
	if genericCompositionString(t, toolPayload, "input_digest") != genericCompositionString(t, toolRequestReplay, "deterministic_tool_hash") ||
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
