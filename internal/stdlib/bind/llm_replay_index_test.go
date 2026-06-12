package bind

import (
	"strings"
	"testing"
)

func TestLLMReplayIndexStrictOrderedMatchesAndSummarizes(t *testing.T) {
	interp := runLLMTestProgram(t, `
records := {
    {
        record_id: "rec-000"
        operation: "llm.complete"
        capability: "generic.ai.completion"
        replay_key: "turn:000"
        request_hash: "sha256:aaa"
        response_hash: "sha256:111"
        provider_free: true
    },
    {
        record_id: "rec-001"
        operation: "tool.call"
        capability: "generic.ai.tool.invoke"
        replay_key: "turn:001"
        request_hash: "sha256:bbb"
        response_hash: "sha256:222"
        provider_free: true
    },
    {
        record_id: "rec-002"
        operation: "llm.complete"
        capability: "generic.ai.completion"
        replay_key: "turn:002"
        request_hash: "sha256:ccc"
        response_hash: "sha256:333"
        provider_free: true
    },
}

index, err := llm.replay_index(records, {
    fixture_id: "fixture:v1"
    strategy: "strict_ordered"
    consume_on_match: true
    consume_on_mismatch: false
})

first := index.match({
    operation: "llm.complete"
    capability: "generic.ai.completion"
    replay_key: "turn:000"
    request_hash: "sha256:aaa"
})
second := index.match({
    operation: "tool.call"
    capability: "generic.ai.tool.invoke"
    replay_key: "turn:001"
    request_hash: "sha256:bbb"
})
summary := index.summary()

err_nil := err == nil
first_ok := first.ok
second_ok := second.ok
matched := summary.matched
unconsumed := summary.unconsumed
next_index := summary.next_index
finding_count := #summary.finding_kinds
first_id := summary.matched_record_ids[1]
second_id := summary.matched_record_ids[2]
`, nil)

	if got := interp.GetGlobal("err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("err_nil = %v, want true", got)
	}
	if got := interp.GetGlobal("first_ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("first_ok = %v, want true", got)
	}
	if got := interp.GetGlobal("second_ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("second_ok = %v, want true", got)
	}
	assertGlobalInt(t, interp, "matched", 2)
	assertGlobalInt(t, interp, "unconsumed", 1)
	assertGlobalInt(t, interp, "next_index", 2)
	assertGlobalInt(t, interp, "finding_count", 1)
	assertGlobalString(t, interp, "first_id", "rec-000")
	assertGlobalString(t, interp, "second_id", "rec-001")
}

func TestLLMReplayIndexStrictOrderedMismatchDoesNotConsumeByDefault(t *testing.T) {
	interp := runLLMTestProgram(t, `
records := {
    {
        record_id: "rec-000"
        operation: "llm.complete"
        capability: "generic.ai.completion"
        replay_key: "turn:000"
        request_hash: "sha256:aaa"
    },
    {
        record_id: "rec-001"
        operation: "tool.call"
        capability: "generic.ai.tool.invoke"
        replay_key: "turn:001"
        request_hash: "sha256:bbb"
    },
}
index, err := llm.replay_index(records, {})
ok := index.match({
    operation: "llm.complete"
    capability: "generic.ai.completion"
    replay_key: "turn:000"
    request_hash: "sha256:aaa"
})
bad := index.match({
    operation: "tool.call"
    capability: "generic.ai.tool.invoke"
    replay_key: "turn:001"
    request_hash: "sha256:wrong"
})
summary := index.summary()

ok_status := ok.status
bad_status := bad.status
bad_kind := bad.finding_kind
bad_message := bad.message
matched := summary.matched
mismatches := summary.mismatches
unconsumed := summary.unconsumed
next_index := summary.next_index
finding_count := #summary.finding_kinds
`, nil)

	assertGlobalString(t, interp, "ok_status", "matched")
	assertGlobalString(t, interp, "bad_status", "mismatch")
	assertGlobalString(t, interp, "bad_kind", "generic.ai.replay.mismatch")
	msg := interp.GetGlobal("bad_message")
	if !msg.IsString() || !strings.Contains(msg.Str(), "request_hash") {
		t.Fatalf("bad_message = %v, want request_hash mismatch", msg)
	}
	assertGlobalInt(t, interp, "matched", 1)
	assertGlobalInt(t, interp, "mismatches", 1)
	assertGlobalInt(t, interp, "unconsumed", 1)
	assertGlobalInt(t, interp, "next_index", 1)
	assertGlobalInt(t, interp, "finding_count", 2)
}

func TestLLMReplayRecordNormalizesHashesAndTurnReplayOptions(t *testing.T) {
	interp := runLLMTestProgram(t, `
record, err := llm.replay_record({
    record_id: "rec-1"
    replay_key: "turn:1"
    request: {
        model: "mock"
        messages: {llm.user("hello replay")}
        max_tokens: 16
    }
    response: {
        status: "final_answer"
        text: "fixture answer"
        calls: {}
        usage: {}
    }
})
err_nil := err == nil
mode := record.mode
operation := record.operation
capability := record.capability
provider_free := record.provider_free
live_network := record.live_network
replay_mode := record.replay.mode
replay_key := record.replay.replay_key
request_hash_present := record.request_hash != ""
response_hash_present := record.response_hash != ""
replay_response_text := record.replay.response.text
`, nil)

	if got := interp.GetGlobal("err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "mode", "fixture_replay")
	assertGlobalString(t, interp, "operation", "llm.turn")
	assertGlobalString(t, interp, "capability", "generic.ai.turn")
	assertGlobalString(t, interp, "replay_mode", "fixture_replay")
	assertGlobalString(t, interp, "replay_key", "turn:1")
	assertGlobalString(t, interp, "replay_response_text", "fixture answer")
	if got := interp.GetGlobal("provider_free"); !got.IsBool() || !got.Bool() {
		t.Fatalf("provider_free = %v, want true", got)
	}
	if got := interp.GetGlobal("live_network"); !got.IsBool() || got.Bool() {
		t.Fatalf("live_network = %v, want false", got)
	}
	if got := interp.GetGlobal("request_hash_present"); !got.IsBool() || !got.Bool() {
		t.Fatalf("request_hash_present = %v, want true", got)
	}
	if got := interp.GetGlobal("response_hash_present"); !got.IsBool() || !got.Bool() {
		t.Fatalf("response_hash_present = %v, want true", got)
	}
}

func TestLLMReplayFixtureWrapsReplayIndexSession(t *testing.T) {
	interp := runLLMTestProgram(t, `
record1, err1 := llm.replay_record({
    record_id: "rec-1"
    replay_key: "turn:1"
    request_hash: "hash:1"
    response: {text: "one"}
})
record2, err2 := llm.replay_record({
    record_id: "rec-2"
    replay_key: "turn:2"
    request_hash: "hash:2"
    response: {text: "two"}
})
fixture, err := llm.replay_fixture({record1, record2}, {
    fixture_id: "fixture:test"
    identity_fields: {"replay_key", "request_hash"}
})
first := fixture.match({replay_key: "turn:1" request_hash: "hash:1"})
bad := fixture.match({replay_key: "turn:2" request_hash: "wrong"})
summary := fixture.summary()

err_nil := err == nil
fixture_id := fixture.fixture_id
mode := fixture.mode
loaded := fixture.loaded_records
first_ok := first.ok
bad_status := bad.status
matched := summary.matched
mismatches := summary.mismatches
unconsumed := summary.unconsumed
first_id := summary.matched_record_ids[1]
`, nil)

	if got := interp.GetGlobal("err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "fixture_id", "fixture:test")
	assertGlobalString(t, interp, "mode", "fixture_replay")
	assertGlobalInt(t, interp, "loaded", 2)
	if got := interp.GetGlobal("first_ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("first_ok = %v, want true", got)
	}
	assertGlobalString(t, interp, "bad_status", "mismatch")
	assertGlobalInt(t, interp, "matched", 1)
	assertGlobalInt(t, interp, "mismatches", 1)
	assertGlobalInt(t, interp, "unconsumed", 1)
	assertGlobalString(t, interp, "first_id", "rec-1")
}

func assertGlobalInt(t *testing.T, interp interface{ GetGlobal(string) Value }, name string, want int64) {
	t.Helper()
	got := interp.GetGlobal(name)
	if !got.IsInt() || got.Int() != want {
		t.Fatalf("%s = %v, want %d", name, got, want)
	}
}
