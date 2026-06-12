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

func TestLLMReplayFixtureBuildsTurnReplayFromRequest(t *testing.T) {
	interp := runLLMTestProgram(t, `
record, record_err := llm.replay_record({
    record_id: "rec-direct"
    replay_key: "turn:direct"
    request: {
        model: "mock"
        messages: {llm.user("direct replay")}
    }
    response: {
        status: "final_answer"
        text: "direct fixture"
        calls: {}
        usage: {}
    }
})
fixture, fixture_err := llm.replay_fixture({record}, {
    fixture_id: "fixture:direct"
})
replay, err := fixture.replay({
    model: "mock"
    messages: {llm.user("direct replay")}
}, "turn:direct")
summary := fixture.summary()

record_err_nil := record_err == nil
fixture_err_nil := fixture_err == nil
err_nil := err == nil
mode := replay.mode
replay_key := replay.replay_key
response_text := replay.response.text
matched := summary.matched
unconsumed := summary.unconsumed
`, nil)

	if got := interp.GetGlobal("record_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("record_err_nil = %v, want true", got)
	}
	if got := interp.GetGlobal("fixture_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("fixture_err_nil = %v, want true", got)
	}
	if got := interp.GetGlobal("err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "mode", "fixture_replay")
	assertGlobalString(t, interp, "replay_key", "turn:direct")
	assertGlobalString(t, interp, "response_text", "direct fixture")
	assertGlobalInt(t, interp, "matched", 1)
	assertGlobalInt(t, interp, "unconsumed", 0)
}

func TestLLMReplayFixtureReplayReportsMismatchAndExhaustion(t *testing.T) {
	interp := runLLMTestProgram(t, `
record, record_err := llm.replay_record({
    record_id: "rec-direct"
    replay_key: "turn:direct"
    request: {
        model: "mock"
        messages: {llm.user("direct replay")}
    }
    response: {status: "final_answer" text: "direct fixture" calls: {} usage: {}}
})
fixture, fixture_err := llm.replay_fixture({record}, {
    fixture_id: "fixture:direct"
})
bad_replay, bad_err := fixture.replay({
    model: "mock"
    messages: {llm.user("wrong replay")}
}, "turn:direct")
ok_replay, ok_err := fixture.replay({
    model: "mock"
    messages: {llm.user("direct replay")}
}, "turn:direct")
exhausted_replay, exhausted_err := fixture.replay({
    model: "mock"
    messages: {llm.user("direct replay")}
}, "turn:direct")
summary := fixture.summary()

bad_replay_nil := bad_replay == nil
bad_kind := bad_err.kind
bad_status := bad_err.status
bad_finding := bad_err.finding_kind
ok_err_nil := ok_err == nil
exhausted_replay_nil := exhausted_replay == nil
exhausted_status := exhausted_err.status
exhausted_finding := exhausted_err.finding_kind
matched := summary.matched
mismatches := summary.mismatches
exhausted := summary.exhausted
`, nil)

	if got := interp.GetGlobal("bad_replay_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("bad_replay_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "bad_kind", "validation")
	assertGlobalString(t, interp, "bad_status", "mismatch")
	assertGlobalString(t, interp, "bad_finding", "generic.ai.replay.mismatch")
	if got := interp.GetGlobal("ok_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok_err_nil = %v, want true", got)
	}
	if got := interp.GetGlobal("exhausted_replay_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("exhausted_replay_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "exhausted_status", "exhausted")
	assertGlobalString(t, interp, "exhausted_finding", "generic.ai.replay.exhausted")
	assertGlobalInt(t, interp, "matched", 1)
	assertGlobalInt(t, interp, "mismatches", 1)
	assertGlobalInt(t, interp, "exhausted", 1)
}

func TestLLMReplayFixtureReplayKeepsStrictOrder(t *testing.T) {
	interp := runLLMTestProgram(t, `
first, err1 := llm.replay_record({
    record_id: "rec-1"
    replay_key: "turn:1"
    request: {model: "mock" messages: {llm.user("first")}}
    response: {status: "final_answer" text: "first" calls: {} usage: {}}
})
second, err2 := llm.replay_record({
    record_id: "rec-2"
    replay_key: "turn:2"
    request: {model: "mock" messages: {llm.user("second")}}
    response: {status: "final_answer" text: "second" calls: {} usage: {}}
})
fixture, fixture_err := llm.replay_fixture({first, second}, {
    fixture_id: "fixture:strict"
})
replay, err := fixture.replay({
    model: "mock"
    messages: {llm.user("second")}
}, "turn:2")
first_replay, first_err := fixture.replay({
    model: "mock"
    messages: {llm.user("first")}
}, "turn:1")
summary := fixture.summary()

replay_nil := replay == nil
err_kind := err.kind
err_status := err.status
err_finding := err.finding_kind
first_err_nil := first_err == nil
first_text := first_replay.response.text
matched := summary.matched
mismatches := summary.mismatches
unconsumed := summary.unconsumed
next_index := summary.next_index
`, nil)

	if got := interp.GetGlobal("replay_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("replay_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "err_kind", "validation")
	assertGlobalString(t, interp, "err_status", "mismatch")
	assertGlobalString(t, interp, "err_finding", "generic.ai.replay.mismatch")
	if got := interp.GetGlobal("first_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("first_err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "first_text", "first")
	assertGlobalInt(t, interp, "matched", 1)
	assertGlobalInt(t, interp, "mismatches", 1)
	assertGlobalInt(t, interp, "unconsumed", 1)
	assertGlobalInt(t, interp, "next_index", 1)
}

func TestLLMReplayFixtureReplaySynthesizesOldRecordAndReturnsClone(t *testing.T) {
	interp := runLLMTestProgram(t, `
fixture, fixture_err := llm.replay_fixture({
    {
        record_id: "legacy-rec"
        operation: "llm.turn"
        capability: "generic.ai.turn"
        replay_key: "turn:legacy"
        request: {model: "mock" messages: {llm.user("legacy")}}
        response: {status: "final_answer" text: "legacy fixture" calls: {} usage: {}}
    },
}, {
    fixture_id: "fixture:legacy"
})
replay, err := fixture.replay({
    model: "mock"
    messages: {llm.user("legacy")}
}, "turn:legacy")
replay.response.text = "mutated"
stored_text := fixture.records[1].replay.response.text
returned_text := replay.response.text
err_nil := err == nil
mode := replay.mode
`, nil)

	if got := interp.GetGlobal("err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "mode", "fixture_replay")
	assertGlobalString(t, interp, "returned_text", "mutated")
	assertGlobalString(t, interp, "stored_text", "legacy fixture")
}

func TestLLMReplayFixtureReplayRejectsUnnormalizedTurnRequest(t *testing.T) {
	interp := runLLMTestProgram(t, `
record, record_err := llm.replay_record({
    record_id: "rec"
    replay_key: "turn:missing"
    request_hash: "hash:missing"
    response: {status: "final_answer" text: "ok" calls: {} usage: {}}
})
fixture, fixture_err := llm.replay_fixture({record}, {
    fixture_id: "fixture:missing"
})
replay, err := fixture.replay({model: "mock"}, "turn:missing")
replay_nil := replay == nil
err_kind := err.kind
err_message := err.message
`, nil)

	if got := interp.GetGlobal("replay_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("replay_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "err_kind", "validation")
	msg := interp.GetGlobal("err_message")
	if !msg.IsString() || !strings.Contains(msg.Str(), "llm.turn requires messages") {
		t.Fatalf("err_message = %v, want llm.turn requires messages", msg)
	}
}

func TestLLMReplayFixtureReplayCustomIdentityFields(t *testing.T) {
	interp := runLLMTestProgram(t, `
record_key_only, err1 := llm.replay_record({
    record_id: "rec-key"
    replay_key: "turn:shared"
    request: {model: "mock" messages: {llm.user("record request")}}
    response: {status: "final_answer" text: "key only fixture" calls: {} usage: {}}
})
fixture_key_only, fixture_err1 := llm.replay_fixture({record_key_only}, {
    fixture_id: "fixture:key-only"
    identity_fields: {"replay_key"}
})
key_replay, key_err := fixture_key_only.replay({
    replay_key: "turn:shared"
}, "turn:shared")

record_tenant, err2 := llm.replay_record({
    record_id: "rec-tenant"
    tenant_id: "alpha"
    replay_key: "turn:tenant"
    request: {model: "mock" messages: {llm.user("tenant request")}}
    response: {status: "final_answer" text: "tenant fixture" calls: {} usage: {}}
})
fixture_tenant, fixture_err2 := llm.replay_fixture({record_tenant}, {
    fixture_id: "fixture:tenant"
    identity_fields: {"tenant_id", "replay_key"}
})
bad_replay, bad_err := fixture_tenant.replay({
    tenant_id: "beta"
    model: "mock"
    messages: {llm.user("tenant request")}
}, "turn:tenant")
good_replay, good_err := fixture_tenant.replay({
    tenant_id: "alpha"
    model: "mock"
    messages: {llm.user("tenant request")}
}, "turn:tenant")

setup_ok := err1 == nil && fixture_err1 == nil && err2 == nil && fixture_err2 == nil
key_err_nil := key_err == nil
key_text := key_replay.response.text
bad_replay_nil := bad_replay == nil
bad_status := bad_err.status
bad_message := bad_err.message
good_err_nil := good_err == nil
good_text := good_replay.response.text
`, nil)

	if got := interp.GetGlobal("setup_ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("setup_ok = %v, want true", got)
	}
	if got := interp.GetGlobal("key_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("key_err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "key_text", "key only fixture")
	if got := interp.GetGlobal("bad_replay_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("bad_replay_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "bad_status", "mismatch")
	msg := interp.GetGlobal("bad_message")
	if !msg.IsString() || !strings.Contains(msg.Str(), "tenant_id") {
		t.Fatalf("bad_message = %v, want tenant_id mismatch", msg)
	}
	if got := interp.GetGlobal("good_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("good_err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "good_text", "tenant fixture")
}

func TestLLMReplayFixtureReplayCustomIdentityDoesNotRequireTurnRequest(t *testing.T) {
	interp := runLLMTestProgram(t, `
record_key_only, err1 := llm.replay_record({
    record_id: "rec-key"
    replay_key: "turn:shared"
    request_hash: "hash:any"
    response: {status: "final_answer" text: "key only fixture" calls: {} usage: {}}
})
fixture_key_only, fixture_err1 := llm.replay_fixture({record_key_only}, {
    fixture_id: "fixture:key-only"
    identity_fields: {"replay_key"}
})
key_replay, key_err := fixture_key_only.replay({replay_key: "turn:shared"})

record_tool, err2 := llm.replay_record({
    record_id: "tool-rec"
    operation: "tool.call"
    capability: "generic.ai.tool.invoke"
    replay_key: "tool:1"
    request_hash: "hash:any"
    response: {status: "ok" text: "tool fixture"}
})
fixture_tool, fixture_err2 := llm.replay_fixture({record_tool}, {
    fixture_id: "fixture:tool"
    identity_fields: {"operation", "capability", "replay_key"}
})
tool_replay, tool_err := fixture_tool.replay({
    operation: "tool.call"
    capability: "generic.ai.tool.invoke"
    replay_key: "tool:1"
})

setup_ok := err1 == nil && fixture_err1 == nil && err2 == nil && fixture_err2 == nil
key_err_nil := key_err == nil
key_text := key_replay.response.text
tool_err_nil := tool_err == nil
tool_text := tool_replay.response.text
`, nil)

	if got := interp.GetGlobal("setup_ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("setup_ok = %v, want true", got)
	}
	if got := interp.GetGlobal("key_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("key_err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "key_text", "key only fixture")
	if got := interp.GetGlobal("tool_err_nil"); !got.IsBool() || !got.Bool() {
		t.Fatalf("tool_err_nil = %v, want true", got)
	}
	assertGlobalString(t, interp, "tool_text", "tool fixture")
}

func TestLLMReplayFixtureBadArguments(t *testing.T) {
	interp := runLLMTestProgram(t, `
missing_ok, missing_err := pcall(llm.replay_fixture)
bad_records_ok, bad_records_err := pcall(llm.replay_fixture, "records")
bad_opts_ok, bad_opts_err := pcall(llm.replay_fixture, {}, "opts")
record, record_err := llm.replay_record({
    record_id: "rec"
    replay_key: "turn:1"
    request_hash: "hash:1"
    response: {status: "final_answer" text: "ok" calls: {} usage: {}}
})
fixture, fixture_err := llm.replay_fixture({record}, {})
bad_request_ok, bad_request_err := pcall(fixture.replay, "request", "turn:1")
bad_key_ok, bad_key_err := pcall(fixture.replay, {request_hash: "hash:1"}, 1)

setup_ok := record_err == nil && fixture_err == nil
`, nil)

	if got := interp.GetGlobal("setup_ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("setup_ok = %v, want true", got)
	}
	for _, name := range []string{"missing_ok", "bad_records_ok", "bad_opts_ok", "bad_request_ok", "bad_key_ok"} {
		if got := interp.GetGlobal(name); !got.IsBool() || got.Bool() {
			t.Fatalf("%s = %v, want false", name, got)
		}
	}
	for _, name := range []string{"missing_err", "bad_records_err", "bad_opts_err", "bad_request_err", "bad_key_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want error string", name, got)
		}
	}
}

func assertGlobalInt(t *testing.T, interp interface{ GetGlobal(string) Value }, name string, want int64) {
	t.Helper()
	got := interp.GetGlobal(name)
	if !got.IsInt() || got.Int() != want {
		t.Fatalf("%s = %v, want %d", name, got, want)
	}
}
