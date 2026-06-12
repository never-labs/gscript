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

func assertGlobalInt(t *testing.T, interp interface{ GetGlobal(string) Value }, name string, want int64) {
	t.Helper()
	got := interp.GetGlobal(name)
	if !got.IsInt() || got.Int() != want {
		t.Fatalf("%s = %v, want %d", name, got, want)
	}
}
