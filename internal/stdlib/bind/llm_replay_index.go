package bind

import (
	"fmt"
	"strings"
)

var llmReplayDefaultIdentityFields = []string{"operation", "capability", "replay_key", "request_hash"}

func registerLLMReplayHelpers(t *Table) {
	setLLMFunction(t, "llm", "replay_record", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replay_record' (record table expected)")
		}
		return []Value{TableValue(llmReplayRecordValue(args[0].Table())), NilValue()}, nil
	})
	setLLMFunction(t, "llm", "replayRecord", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replayRecord' (record table expected)")
		}
		return []Value{TableValue(llmReplayRecordValue(args[0].Table())), NilValue()}, nil
	})
	replayHTTPRecord := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replay_http_record' (HTTP replay record table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.replay_http_record' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmReplayHTTPRecordValue(args[0].Table(), opts)), NilValue()}, nil
	}
	setLLMFunction(t, "llm", "replay_http_record", replayHTTPRecord)
	setLLMFunction(t, "llm", "replayHttpRecord", replayHTTPRecord)
	setLLMFunction(t, "llm", "replay_api_record", replayHTTPRecord)
	setLLMFunction(t, "llm", "replayApiRecord", replayHTTPRecord)
	replayArtifactRecord := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replay_artifact_record' (artifact replay record table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.replay_artifact_record' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmReplayArtifactRecordValue(args[0].Table(), opts)), NilValue()}, nil
	}
	setLLMFunction(t, "llm", "replay_artifact_record", replayArtifactRecord)
	setLLMFunction(t, "llm", "replayArtifactRecord", replayArtifactRecord)
	setLLMFunction(t, "llm", "replay_fixture", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replay_fixture' (records or fixture table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.replay_fixture' (options table expected)")
			}
			opts = args[1].Table()
		}
		fixture, errValue := llmReplayFixtureValue(args[0].Table(), opts)
		if !errValue.IsNil() {
			return []Value{NilValue(), errValue}, nil
		}
		return []Value{TableValue(fixture), NilValue()}, nil
	})
	setLLMFunction(t, "llm", "replayFixture", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replayFixture' (records or fixture table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.replayFixture' (options table expected)")
			}
			opts = args[1].Table()
		}
		fixture, errValue := llmReplayFixtureValue(args[0].Table(), opts)
		if !errValue.IsNil() {
			return []Value{NilValue(), errValue}, nil
		}
		return []Value{TableValue(fixture), NilValue()}, nil
	})
	setLLMFunction(t, "llm", "replay_index", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replay_index' (records table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.replay_index' (options table expected)")
			}
			opts = args[1].Table()
		}
		session, errValue := newLLMReplayIndex(args[0].Table(), opts)
		if !errValue.IsNil() {
			return []Value{NilValue(), errValue}, nil
		}
		return []Value{TableValue(session), NilValue()}, nil
	})
	setLLMFunction(t, "llm", "fixture_index", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.fixture_index' (fixture index table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.fixture_index' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmFixtureIndexValue(args[0].Table(), opts))}, nil
	})
	setLLMFunction(t, "llm", "fixtureIndex", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.fixtureIndex' (fixture index table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.fixtureIndex' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmFixtureIndexValue(args[0].Table(), opts))}, nil
	})
	setLLMFunction(t, "llm", "validate_fixture_index", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.validate_fixture_index' (fixture index table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.validate_fixture_index' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmValidateFixtureIndexValue(args[0].Table(), opts))}, nil
	})
	setLLMFunction(t, "llm", "validateFixtureIndex", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.validateFixtureIndex' (fixture index table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.validateFixtureIndex' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmValidateFixtureIndexValue(args[0].Table(), opts))}, nil
	})
}

func llmFixtureIndexValue(src, opts *Table) *Table {
	out := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		out.RawSet(key, llmCloneValue(src.RawGet(key)))
	}
	out.RawSetString("__llm_fixture_index", BoolValue(true))
	out.RawSetString("kind", StringValue("fixture_index"))
	out.RawSetString("schema_version", IntValue(int64(llmReplayOptionInt(src, "schema_version", llmReplayOptionInt(opts, "schema_version", 1)))))
	out.RawSetString("version", StringValue(llmReplayOptionString(src, "version", llmReplayOptionString(opts, "version", "fixture_index.v1"))))
	fixtureID := llmReplayOptionString(src, "fixture_id", llmReplayOptionString(src, "id", llmReplayOptionString(opts, "fixture_id", "fixture-index")))
	out.RawSetString("fixture_id", StringValue(fixtureID))
	if out.RawGetString("id").IsNil() {
		out.RawSetString("id", StringValue(fixtureID))
	}
	out.RawSetString("provider_free", BoolValue(llmReplayOptionBool(src, "provider_free", llmReplayOptionBool(opts, "provider_free", true))))
	out.RawSetString("domain_specific", BoolValue(llmReplayOptionBool(src, "domain_specific", llmReplayOptionBool(opts, "domain_specific", false))))
	out.RawSetString("live_network", BoolValue(llmReplayOptionBool(src, "live_network", llmReplayOptionBool(opts, "live_network", false))))
	out.RawSetString("live_model", BoolValue(llmReplayOptionBool(src, "live_model", llmReplayOptionBool(opts, "live_model", false))))
	out.RawSetString("live_model_calls", BoolValue(llmReplayOptionBool(src, "live_model_calls", llmReplayOptionBool(opts, "live_model_calls", false))))
	out.RawSetString("real_dependency_imports", BoolValue(llmReplayOptionBool(src, "real_dependency_imports", llmReplayOptionBool(opts, "real_dependency_imports", false))))
	out.RawSetString("credentials_required", BoolValue(llmReplayOptionBool(src, "credentials_required", llmReplayOptionBool(opts, "credentials_required", false))))
	out.RawSetString("provider_credentials_required", BoolValue(llmReplayOptionBool(src, "provider_credentials_required", llmReplayOptionBool(opts, "provider_credentials_required", false))))
	out.RawSetString("secret_values_present", BoolValue(llmReplayOptionBool(src, "secret_values_present", llmReplayOptionBool(opts, "secret_values_present", false))))
	out.RawSetString("mode", StringValue(llmReplayOptionString(src, "mode", llmReplayOptionString(opts, "mode", llmReplayModeFixture))))
	out.RawSetString("strategy", StringValue(llmReplayOptionString(src, "strategy", llmReplayOptionString(opts, "strategy", "strict_ordered"))))
	fixtures := llmFixtureIndexFixtures(src, opts, out)
	out.RawSetString("fixtures", TableValue(fixtures))
	out.RawSetString("fixture_count", IntValue(int64(fixtures.Length())))
	out.RawSetString("matching", TableValue(llmFixtureIndexMatchingValue(src, opts)))
	out.RawSetString("deterministic_summary_order", llmReplayStringListValue([]string{"fixture_id", "strategy", "fixture_count", "provider_free", "live_network", "real_dependency_imports"}))
	out.RawSetString("summary", TableValue(llmFixtureIndexSummaryValue(out)))
	return out
}

func llmFixtureIndexMatchingValue(src, opts *Table) *Table {
	matching := NewTable()
	if existing := src.RawGetString("matching"); existing.IsTable() {
		llmCopyTable(matching, existing.Table(), true)
	}
	matching.RawSetString("scan_ahead", BoolValue(llmReplayOptionBool(matching, "scan_ahead", false)))
	matching.RawSetString("consume_on_match", BoolValue(llmReplayOptionBool(matching, "consume_on_match", llmReplayOptionBool(opts, "consume_on_match", true))))
	matching.RawSetString("consume_on_mismatch", BoolValue(llmReplayOptionBool(matching, "consume_on_mismatch", llmReplayOptionBool(opts, "consume_on_mismatch", false))))
	if matching.RawGetString("identity_fields").IsNil() {
		matching.RawSetString("identity_fields", llmReplayStringListValue(llmReplayIdentityFields(opts)))
	}
	matching.RawSetString("mismatch_finding_kind", StringValue(llmReplayOptionString(matching, "mismatch_finding_kind", "generic.ai.replay.mismatch")))
	matching.RawSetString("unconsumed_finding_kind", StringValue(llmReplayOptionString(matching, "unconsumed_finding_kind", "generic.ai.replay.unconsumed_record")))
	matching.RawSetString("exhausted_finding_kind", StringValue(llmReplayOptionString(matching, "exhausted_finding_kind", "generic.ai.replay.exhausted")))
	return matching
}

func llmFixtureIndexSummaryValue(index *Table) *Table {
	summary := NewTable()
	for _, field := range []string{"fixture_id", "strategy", "fixture_count", "provider_free", "domain_specific", "live_network", "live_model", "real_dependency_imports", "credentials_required"} {
		if value := index.RawGetString(field); !value.IsNil() {
			summary.RawSetString(field, llmCloneValue(value))
		}
	}
	return summary
}

func llmFixtureIndexFixtures(src, opts, index *Table) *Table {
	fixturesValue := src.RawGetString("fixtures")
	if !fixturesValue.IsTable() {
		return NewSequentialArrayTable(0)
	}
	srcFixtures := fixturesValue.Table()
	out := NewSequentialArrayTable(0)
	for _, item := range llmFixtureIndexEntryValues(srcFixtures) {
		if !item.IsTable() {
			continue
		}
		out.RawSet(IntValue(int64(out.Length()+1)), TableValue(llmFixtureIndexEntryValue(item.Table(), opts, index, out.Length()+1)))
	}
	return out
}

func llmFixtureIndexEntryValue(src, opts, index *Table, ordinal int) *Table {
	out := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		out.RawSet(key, llmCloneValue(src.RawGet(key)))
	}
	id := llmReplayOptionString(src, "id", llmReplayOptionString(src, "fixture_key", llmReplayOptionString(src, "key", fmt.Sprintf("fixture:%d", ordinal))))
	fixtureKey := llmReplayOptionString(src, "fixture_key", id)
	out.RawSetString("id", StringValue(id))
	out.RawSetString("fixture_key", StringValue(fixtureKey))
	if out.RawGetString("schema").IsNil() {
		if value := src.RawGetString("schema_path"); !value.IsNil() {
			out.RawSetString("schema", llmCloneValue(value))
		} else if value := src.RawGetString("schemas"); !value.IsNil() {
			out.RawSetString("schema", llmCloneValue(value))
		}
	}
	out.RawSetString("provider_free", BoolValue(llmReplayOptionBool(src, "provider_free", llmReplayOptionBool(index, "provider_free", true))))
	out.RawSetString("live_network", BoolValue(llmReplayOptionBool(src, "live_network", llmReplayOptionBool(index, "live_network", false))))
	out.RawSetString("live_model", BoolValue(llmReplayOptionBool(src, "live_model", llmReplayOptionBool(index, "live_model", false))))
	out.RawSetString("real_dependency_imports", BoolValue(llmReplayOptionBool(src, "real_dependency_imports", llmReplayOptionBool(index, "real_dependency_imports", false))))
	out.RawSetString("credentials_required", BoolValue(llmReplayOptionBool(src, "credentials_required", llmReplayOptionBool(index, "credentials_required", false))))
	out.RawSetString("provider_credentials_required", BoolValue(llmReplayOptionBool(src, "provider_credentials_required", llmReplayOptionBool(index, "provider_credentials_required", false))))
	metadata := NewTable()
	if existing := src.RawGetString("metadata"); existing.IsTable() {
		llmCopyTable(metadata, existing.Table(), true)
	}
	metadata.RawSetString("replay_ready", BoolValue(llmReplayOptionBool(metadata, "replay_ready", llmReplayOptionBool(opts, "replay_ready", true))))
	metadata.RawSetString("provider_free", BoolValue(llmReplayOptionBool(metadata, "provider_free", llmReplayOptionBool(out, "provider_free", true))))
	metadata.RawSetString("live_network", BoolValue(llmReplayOptionBool(metadata, "live_network", llmReplayOptionBool(out, "live_network", false))))
	metadata.RawSetString("real_dependency_imports", BoolValue(llmReplayOptionBool(metadata, "real_dependency_imports", llmReplayOptionBool(out, "real_dependency_imports", false))))
	metadata.RawSetString("credentials_required", BoolValue(llmReplayOptionBool(metadata, "credentials_required", llmReplayOptionBool(out, "credentials_required", false))))
	metadata.RawSetString("live_model_calls", BoolValue(llmReplayOptionBool(metadata, "live_model_calls", llmReplayOptionBool(index, "live_model_calls", false))))
	out.RawSetString("metadata", TableValue(metadata))
	return out
}

func llmValidateFixtureIndexValue(index, opts *Table) *Table {
	findings := NewSequentialArrayTable(0)
	if llmReplayOptionBool(opts, "require_provider_free", true) && !llmReplayOptionBool(index, "provider_free", false) {
		llmFixtureIndexFinding(findings, "provider_free", "fixture index must be provider-free", "")
	}
	if llmReplayOptionBool(opts, "require_offline", true) {
		if llmReplayOptionBool(index, "live_network", false) {
			llmFixtureIndexFinding(findings, "live_network", "fixture index must not require live network", "")
		}
		if llmReplayOptionBool(index, "real_dependency_imports", false) {
			llmFixtureIndexFinding(findings, "real_dependency_imports", "fixture index must not require real dependency imports", "")
		}
		if llmReplayOptionBool(index, "credentials_required", false) || llmReplayOptionBool(index, "provider_credentials_required", false) {
			llmFixtureIndexFinding(findings, "credentials_required", "fixture index must not require credentials", "")
		}
	}
	fixtures := index.RawGetString("fixtures").Table()
	if fixtures == nil || fixtures.Length() == 0 {
		if llmReplayOptionBool(opts, "require_fixtures", false) {
			llmFixtureIndexFinding(findings, "fixtures", "fixture index must contain fixtures", "")
		}
	} else {
		llmValidateFixtureIndexEntries(fixtures, opts, findings)
	}
	ok := findings.Length() == 0
	out := NewTable()
	out.RawSetString("kind", StringValue("fixture_index_validation"))
	out.RawSetString("schema_version", IntValue(1))
	out.RawSetString("version", StringValue("fixture_index_validation.v1"))
	out.RawSetString("ok", BoolValue(ok))
	if ok {
		out.RawSetString("status", StringValue("ok"))
		out.RawSetString("result_status", StringValue("ok"))
	} else {
		out.RawSetString("status", StringValue("failed"))
		out.RawSetString("result_status", StringValue("blocked"))
	}
	out.RawSetString("fixture_id", llmCloneValue(index.RawGetString("fixture_id")))
	out.RawSetString("fixture_count", IntValue(int64(llmFixtureIndexLength(fixtures))))
	out.RawSetString("finding_count", IntValue(int64(findings.Length())))
	out.RawSetString("findings", TableValue(findings))
	out.RawSetString("provider_free", BoolValue(llmReplayOptionBool(index, "provider_free", true)))
	out.RawSetString("live_network", BoolValue(llmReplayOptionBool(index, "live_network", false)))
	out.RawSetString("real_dependency_imports", BoolValue(llmReplayOptionBool(index, "real_dependency_imports", false)))
	return out
}

func llmValidateFixtureIndexEntries(fixtures, opts, findings *Table) {
	ordinal := 0
	for _, item := range llmFixtureIndexEntryValues(fixtures) {
		ordinal++
		fixtureName := fmt.Sprintf("fixture:%d", ordinal)
		if !item.IsTable() {
			llmFixtureIndexFinding(findings, "fixture", "fixture must be a table", fixtureName)
			continue
		}
		fixture := item.Table()
		if id := llmReplayOptionString(fixture, "fixture_key", llmReplayOptionString(fixture, "key", llmReplayOptionString(fixture, "id", ""))); id != "" {
			fixtureName = id
		} else {
			llmFixtureIndexFinding(findings, "fixture_key", "fixture must include id or fixture_key", fixtureName)
		}
		if path := fixture.RawGetString("path"); !path.IsNil() {
			llmValidateFixtureIndexReference(findings, "path", path, fixtureName)
		} else if llmReplayOptionBool(opts, "require_path", false) {
			llmFixtureIndexFinding(findings, "path", "fixture must include a path", fixtureName)
		}
		for _, field := range []string{"schema", "schema_path", "schemas"} {
			if value := fixture.RawGetString(field); !value.IsNil() {
				llmValidateFixtureIndexReference(findings, field, value, fixtureName)
			}
		}
		if llmReplayOptionBool(opts, "require_provider_free", true) && !llmReplayOptionBool(fixture, "provider_free", false) {
			llmFixtureIndexFinding(findings, "provider_free", "fixture must be provider-free", fixtureName)
		}
		if llmReplayOptionBool(opts, "require_offline", true) {
			if llmReplayOptionBool(fixture, "live_network", false) {
				llmFixtureIndexFinding(findings, "live_network", "fixture must not require live network", fixtureName)
			}
			if llmReplayOptionBool(fixture, "real_dependency_imports", false) {
				llmFixtureIndexFinding(findings, "real_dependency_imports", "fixture must not require real dependency imports", fixtureName)
			}
		}
		if llmReplayOptionBool(opts, "require_replay_ready", false) {
			metadata := fixture.RawGetString("metadata").Table()
			if metadata == nil || !llmReplayOptionBool(metadata, "replay_ready", false) {
				llmFixtureIndexFinding(findings, "replay_ready", "fixture metadata must be replay-ready", fixtureName)
			}
		}
		if metadata := fixture.RawGetString("metadata"); metadata.IsTable() {
			llmValidateFixtureIndexMetadata(metadata.Table(), findings, fixtureName)
		}
	}
}

func llmFixtureIndexEntryValues(fixtures *Table) []Value {
	if fixtures == nil {
		return nil
	}
	out := make([]Value, 0, fixtures.Length())
	for i := 1; i <= fixtures.Length(); i++ {
		out = append(out, fixtures.RawGet(IntValue(int64(i))))
	}
	if len(out) > 0 {
		return out
	}
	for _, key := range fixtures.PairsKeysSnapshot() {
		out = append(out, fixtures.RawGet(key))
	}
	return out
}

func llmValidateFixtureIndexMetadata(metadata, findings *Table, fixtureName string) {
	for _, check := range []struct {
		field string
		want  bool
	}{
		{"provider_free", true},
		{"live_network", false},
		{"real_dependency_imports", false},
		{"replay_ready", true},
	} {
		value := metadata.RawGetString(check.field)
		if value.IsNil() {
			continue
		}
		if !value.IsBool() || value.Bool() != check.want {
			llmFixtureIndexFinding(findings, check.field, fmt.Sprintf("fixture metadata %s must be %v", check.field, check.want), fixtureName)
		}
	}
}

func llmValidateFixtureIndexReference(findings *Table, field string, value Value, fixtureName string) {
	if value.IsString() {
		llmValidateFixtureIndexReferenceString(findings, field, value.Str(), fixtureName)
		return
	}
	if value.IsTable() {
		for i := 1; i <= value.Table().Length(); i++ {
			item := value.Table().RawGet(IntValue(int64(i)))
			if !item.IsString() {
				llmFixtureIndexFinding(findings, field, "fixture reference list entries must be strings", fixtureName)
				continue
			}
			llmValidateFixtureIndexReferenceString(findings, field, item.Str(), fixtureName)
		}
		return
	}
	llmFixtureIndexFinding(findings, field, "fixture reference must be a string or string list", fixtureName)
}

func llmValidateFixtureIndexReferenceString(findings *Table, field, ref, fixtureName string) {
	if ref == "" {
		llmFixtureIndexFinding(findings, field, "fixture reference must be non-empty", fixtureName)
		return
	}
	if strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "\\") || strings.Contains(ref, "../") || strings.Contains(ref, `..\`) || strings.Contains(ref, "://") {
		llmFixtureIndexFinding(findings, field, "fixture reference must be relative metadata, not an absolute path, parent traversal, or URL", fixtureName)
	}
}

func llmFixtureIndexFinding(findings *Table, kind, message, fixture string) {
	finding := NewTable()
	finding.RawSetString("kind", StringValue(kind))
	finding.RawSetString("message", StringValue(message))
	if fixture != "" {
		finding.RawSetString("fixture", StringValue(fixture))
	}
	findings.RawSet(IntValue(int64(findings.Length()+1)), TableValue(finding))
}

func llmFixtureIndexLength(fixtures *Table) int {
	if fixtures == nil {
		return 0
	}
	return len(llmFixtureIndexEntryValues(fixtures))
}

func llmReplayRecordValue(src *Table) *Table {
	out := NewTable()
	for _, key := range src.PairsKeysSnapshot() {
		out.RawSet(key, llmCloneValue(src.RawGet(key)))
	}
	out.RawSetString("mode", StringValue(llmReplayOptionString(src, "mode", llmReplayModeFixture)))
	out.RawSetString("operation", StringValue(llmReplayOptionString(src, "operation", "llm.turn")))
	out.RawSetString("capability", StringValue(llmReplayOptionString(src, "capability", "generic.ai.turn")))
	out.RawSetString("provider_free", BoolValue(llmReplayOptionBool(src, "provider_free", true)))
	out.RawSetString("live_network", BoolValue(llmReplayOptionBool(src, "live_network", false)))
	out.RawSetString("live_model", BoolValue(llmReplayOptionBool(src, "live_model", false)))
	out.RawSetString("created_from_provider", BoolValue(llmReplayOptionBool(src, "created_from_provider", false)))
	if out.RawGetString("replay_key").IsNil() {
		out.RawSetString("replay_key", StringValue(llmReplayRecordDefaultKey(src)))
	}
	if out.RawGetString("request_hash").IsNil() {
		out.RawSetString("request_hash", StringValue(llmReplayRecordRequestHash(out.RawGetString("request"))))
	}
	if out.RawGetString("response_hash").IsNil() {
		out.RawSetString("response_hash", StringValue(llmStableValueHash(out.RawGetString("response"))))
	}
	if out.RawGetString("replay").IsNil() {
		out.RawSetString("replay", llmReplayRecordTurnReplayValue(out))
	}
	return out
}

func llmReplayRecordRequestHash(request Value) string {
	if request.IsTable() {
		if req, err := llmRequestFromTable(request.Table()); err == nil {
			return llmTurnRequestHash(req)
		}
	}
	return llmStableValueHash(request)
}

func llmReplayRecordTurnReplayValue(record *Table) Value {
	replay := NewTable()
	replay.RawSetString("mode", StringValue(llmReplayOptionString(record, "mode", llmReplayModeFixture)))
	replay.RawSetString("replay_key", llmCloneValue(record.RawGetString("replay_key")))
	replay.RawSetString("request_hash", llmCloneValue(record.RawGetString("request_hash")))
	replay.RawSetString("response_hash", llmCloneValue(record.RawGetString("response_hash")))
	replay.RawSetString("provider_free", BoolValue(llmReplayOptionBool(record, "provider_free", true)))
	replay.RawSetString("live_network", BoolValue(llmReplayOptionBool(record, "live_network", false)))
	replay.RawSetString("created_from_provider", BoolValue(llmReplayOptionBool(record, "created_from_provider", false)))
	replay.RawSetString("response", llmCloneValue(record.RawGetString("response")))
	return TableValue(replay)
}

func llmReplayRecordDefaultKey(src *Table) string {
	if id := llmReplayOptionString(src, "record_id", ""); id != "" {
		return id
	}
	if id := llmReplayOptionString(src, "fixture_key", ""); id != "" {
		return id
	}
	return "record:1"
}

func llmReplayFixtureValue(input, opts *Table) (*Table, Value) {
	records := input
	if recordsValue := input.RawGetString("records"); recordsValue.IsTable() {
		records = recordsValue.Table()
	}
	normalized := NewSequentialArrayTable(0)
	for i := 1; i <= records.Length(); i++ {
		record := records.RawGet(IntValue(int64(i)))
		if record.IsNil() {
			continue
		}
		if !record.IsTable() {
			return nil, llmErrorValue("validation", fmt.Sprintf("llm.replay_fixture record #%d must be a table", i))
		}
		normalized.RawSet(IntValue(int64(normalized.Length()+1)), TableValue(llmReplayRecordValue(record.Table())))
	}
	indexOpts := llmReplayFixtureIndexOptions(input, opts)
	session, errValue := newLLMReplayIndex(normalized, indexOpts)
	if !errValue.IsNil() {
		return nil, errValue
	}
	fixture := NewTable()
	fixture.RawSetString("__llm_replay_fixture", BoolValue(true))
	fixture.RawSetString("fixture_id", StringValue(llmReplayOptionString(indexOpts, "fixture_id", "fixture")))
	fixture.RawSetString("mode", StringValue(llmReplayOptionString(input, "mode", llmReplayModeFixture)))
	fixture.RawSetString("strategy", StringValue(llmReplayOptionString(indexOpts, "strategy", "strict_ordered")))
	fixture.RawSetString("provider_free", BoolValue(llmReplayOptionBool(input, "provider_free", llmReplayOptionBool(opts, "provider_free", true))))
	fixture.RawSetString("live_network", BoolValue(llmReplayOptionBool(input, "live_network", llmReplayOptionBool(opts, "live_network", false))))
	fixture.RawSetString("live_model", BoolValue(llmReplayOptionBool(input, "live_model", llmReplayOptionBool(opts, "live_model", false))))
	fixture.RawSetString("loaded_records", IntValue(int64(normalized.Length())))
	fixture.RawSetString("records", TableValue(normalized))
	fixture.RawSetString("index", TableValue(session))
	fixture.RawSetString("identity_fields", session.RawGetString("identity_fields"))
	fixture.RawSetString("match", session.RawGetString("match"))
	fixture.RawSetString("summary", session.RawGetString("summary"))
	fixture.RawSetString("replay", session.RawGetString("replay"))
	return fixture, NilValue()
}

func llmReplayFixtureIndexOptions(input, opts *Table) *Table {
	out := NewTable()
	for _, src := range []*Table{input, opts} {
		if src == nil {
			continue
		}
		for _, field := range []string{"fixture_id", "strategy", "identity_fields", "consume_on_match", "consume_on_mismatch"} {
			if value := src.RawGetString(field); !value.IsNil() {
				out.RawSetString(field, llmCloneValue(value))
			}
		}
	}
	if out.RawGetString("fixture_id").IsNil() {
		out.RawSetString("fixture_id", StringValue("fixture"))
	}
	if out.RawGetString("strategy").IsNil() {
		out.RawSetString("strategy", StringValue("strict_ordered"))
	}
	return out
}

type llmReplayIndexState struct {
	fixtureID         string
	strategy          string
	identityFields    []string
	consumeOnMatch    bool
	consumeOnMismatch bool
	records           []Value
	next              int
	requests          int
	matched           int
	mismatches        int
	exhausted         int
	findings          []string
	matchedRecordIDs  []string
}

func newLLMReplayIndex(records, opts *Table) (*Table, Value) {
	state := &llmReplayIndexState{
		fixtureID:         llmReplayOptionString(opts, "fixture_id", "fixture"),
		strategy:          llmReplayOptionString(opts, "strategy", "strict_ordered"),
		identityFields:    llmReplayIdentityFields(opts),
		consumeOnMatch:    llmReplayOptionBool(opts, "consume_on_match", true),
		consumeOnMismatch: llmReplayOptionBool(opts, "consume_on_mismatch", false),
	}
	if state.strategy != "strict_ordered" {
		return nil, llmErrorValue("validation", "llm.replay_index only supports strict_ordered strategy")
	}
	for i := 1; i <= records.Length(); i++ {
		record := records.RawGet(IntValue(int64(i)))
		if !record.IsTable() {
			return nil, llmErrorValue("validation", fmt.Sprintf("llm.replay_index record #%d must be a table", i))
		}
		state.records = append(state.records, llmCloneValue(record))
	}
	session := NewTable()
	session.RawSetString("__llm_replay_index", BoolValue(true))
	session.RawSetString("fixture_id", StringValue(state.fixtureID))
	session.RawSetString("strategy", StringValue(state.strategy))
	session.RawSetString("loaded_records", IntValue(int64(len(state.records))))
	session.RawSetString("identity_fields", llmReplayStringListValue(state.identityFields))
	session.RawSetString("match", FunctionValue(&GoFunction{Name: "llm.replay_index.match", Fn: func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replay_index.match' (request table expected)")
		}
		return []Value{state.match(args[0].Table())}, nil
	}}))
	session.RawSetString("summary", FunctionValue(&GoFunction{Name: "llm.replay_index.summary", Fn: func(args []Value) ([]Value, error) {
		return []Value{state.summary()}, nil
	}}))
	session.RawSetString("replay", FunctionValue(&GoFunction{Name: "llm.replay_index.replay", Fn: func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.replay_index.replay' (request table expected)")
		}
		replayKey := ""
		if len(args) >= 2 {
			if !args[1].IsString() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.replay_index.replay' (replay_key string expected)")
			}
			replayKey = args[1].Str()
		}
		return state.replay(args[0].Table(), replayKey), nil
	}}))
	return session, NilValue()
}

func (s *llmReplayIndexState) replay(request *Table, replayKey string) []Value {
	identity, errValue := llmReplayRequestIdentityValue(request, replayKey, s.identityNeedsField("request_hash"))
	if !errValue.IsNil() {
		return []Value{NilValue(), errValue}
	}
	match := s.match(identity)
	matchTable := match.Table()
	if matchTable == nil || !matchTable.RawGetString("ok").Truthy() {
		return []Value{NilValue(), llmReplayMatchErrorValue(matchTable)}
	}
	record := matchTable.RawGetString("record")
	if !record.IsTable() {
		return []Value{NilValue(), llmErrorValue("validation", "llm.replay_fixture replay matched a non-table record")}
	}
	replay := record.Table().RawGetString("replay")
	if !replay.IsTable() {
		return []Value{llmReplayRecordTurnReplayValue(record.Table()), NilValue()}
	}
	return []Value{llmCloneValue(replay), NilValue()}
}

func llmReplayRequestIdentityValue(request *Table, replayKey string, needsRequestHash bool) (*Table, Value) {
	identity := NewTable()
	for _, key := range request.PairsKeysSnapshot() {
		identity.RawSet(key, llmCloneValue(request.RawGet(key)))
	}
	operation := llmReplayOptionString(request, "operation", "llm.turn")
	identity.RawSetString("operation", StringValue(operation))
	identity.RawSetString("capability", StringValue(llmReplayOptionString(request, "capability", "generic.ai.turn")))
	if replayKey == "" {
		replayKey = llmReplayOptionString(request, "replay_key", llmReplayOptionString(request, "record_id", ""))
	}
	identity.RawSetString("replay_key", StringValue(replayKey))
	if hash := llmReplayOptionString(request, "request_hash", ""); hash != "" {
		identity.RawSetString("request_hash", StringValue(hash))
	} else if needsRequestHash && operation == "llm.turn" {
		req, err := llmRequestFromTable(request)
		if err != nil {
			return nil, llmErrorValue("validation", err.Error())
		}
		identity.RawSetString("request_hash", StringValue(llmTurnRequestHash(req)))
	} else if needsRequestHash {
		identity.RawSetString("request_hash", StringValue(llmStableValueHash(TableValue(request))))
	}
	return identity, NilValue()
}

func (s *llmReplayIndexState) identityNeedsField(field string) bool {
	for _, identityField := range s.identityFields {
		if identityField == field {
			return true
		}
	}
	return false
}

func llmReplayMatchErrorValue(match *Table) Value {
	if match == nil {
		return llmErrorValue("validation", "llm.replay_fixture replay failed")
	}
	status := llmReplayOptionString(match, "status", "mismatch")
	message := llmReplayOptionString(match, "message", fmt.Sprintf("llm.replay_fixture replay %s", status))
	err := llmErrorValue("validation", message)
	if et := err.Table(); et != nil {
		et.RawSetString("status", StringValue(status))
		if finding := match.RawGetString("finding_kind"); !finding.IsNil() {
			et.RawSetString("finding_kind", llmCloneValue(finding))
		}
		if summary := match.RawGetString("summary"); !summary.IsNil() {
			et.RawSetString("summary", llmCloneValue(summary))
		}
	}
	return err
}

func (s *llmReplayIndexState) match(request *Table) Value {
	s.requests++
	result := NewTable()
	result.RawSetString("request", llmCloneValue(TableValue(request)))
	result.RawSetString("next_index", IntValue(int64(s.next)))
	if s.next >= len(s.records) {
		s.exhausted++
		s.findings = append(s.findings, "generic.ai.replay.exhausted")
		result.RawSetString("ok", BoolValue(false))
		result.RawSetString("status", StringValue("exhausted"))
		result.RawSetString("finding_kind", StringValue("generic.ai.replay.exhausted"))
		result.RawSetString("summary", s.summary())
		return TableValue(result)
	}
	record := s.records[s.next]
	recordTable := record.Table()
	result.RawSetString("record", llmCloneValue(record))
	if mismatch := s.identityMismatch(recordTable, request); mismatch != "" {
		s.mismatches++
		s.findings = append(s.findings, "generic.ai.replay.mismatch")
		if s.consumeOnMismatch {
			s.next++
		}
		result.RawSetString("ok", BoolValue(false))
		result.RawSetString("status", StringValue("mismatch"))
		result.RawSetString("finding_kind", StringValue("generic.ai.replay.mismatch"))
		result.RawSetString("message", StringValue(mismatch))
		result.RawSetString("summary", s.summary())
		return TableValue(result)
	}
	s.matched++
	if id := recordTable.RawGetString("record_id"); id.IsString() && id.Str() != "" {
		s.matchedRecordIDs = append(s.matchedRecordIDs, id.Str())
	}
	if s.consumeOnMatch {
		s.next++
	}
	result.RawSetString("ok", BoolValue(true))
	result.RawSetString("status", StringValue("matched"))
	result.RawSetString("summary", s.summary())
	return TableValue(result)
}

func (s *llmReplayIndexState) identityMismatch(record, request *Table) string {
	for _, field := range s.identityFields {
		expected := record.RawGetString(field)
		actual := request.RawGetString(field)
		if !llmReplayIdentityEqual(expected, actual) {
			return fmt.Sprintf("replay identity mismatch on %s: got %s want %s", field, llmReplayIdentityString(actual), llmReplayIdentityString(expected))
		}
	}
	return ""
}

func (s *llmReplayIndexState) summary() Value {
	out := NewTable()
	out.RawSetString("fixture_id", StringValue(s.fixtureID))
	out.RawSetString("strategy", StringValue(s.strategy))
	out.RawSetString("loaded_records", IntValue(int64(len(s.records))))
	out.RawSetString("requests", IntValue(int64(s.requests)))
	out.RawSetString("matched", IntValue(int64(s.matched)))
	out.RawSetString("mismatches", IntValue(int64(s.mismatches)))
	out.RawSetString("exhausted", IntValue(int64(s.exhausted)))
	unconsumed := len(s.records) - s.next
	if unconsumed < 0 {
		unconsumed = 0
	}
	out.RawSetString("unconsumed", IntValue(int64(unconsumed)))
	out.RawSetString("next_index", IntValue(int64(s.next)))
	out.RawSetString("finding_kinds", llmReplayFindingKindsValue(s.findings, unconsumed))
	out.RawSetString("matched_record_ids", llmReplayStringListValue(s.matchedRecordIDs))
	return TableValue(out)
}

func llmReplayIdentityFields(opts *Table) []string {
	if opts != nil {
		if fields := opts.RawGetString("identity_fields"); fields.IsTable() {
			out := make([]string, 0, fields.Table().Length())
			for i := 1; i <= fields.Table().Length(); i++ {
				field := fields.Table().RawGet(IntValue(int64(i))).Str()
				if field != "" {
					out = append(out, field)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return append([]string(nil), llmReplayDefaultIdentityFields...)
}

func llmReplayFindingKindsValue(findings []string, unconsumed int) Value {
	out := NewSequentialArrayTable(len(findings) + unconsumed)
	for i, finding := range findings {
		out.RawSet(IntValue(int64(i+1)), StringValue(finding))
	}
	for i := 0; i < unconsumed; i++ {
		out.RawSet(IntValue(int64(len(findings)+i+1)), StringValue("generic.ai.replay.unconsumed_record"))
	}
	return TableValue(out)
}

func llmReplayStringListValue(items []string) Value {
	out := NewSequentialArrayTable(len(items))
	for i, item := range items {
		out.RawSet(IntValue(int64(i+1)), StringValue(item))
	}
	return TableValue(out)
}

func llmReplayOptionString(opts *Table, key, fallback string) string {
	if opts == nil {
		return fallback
	}
	if value := opts.RawGetString(key); value.IsString() && value.Str() != "" {
		return value.Str()
	}
	return fallback
}

func llmReplayOptionBool(opts *Table, key string, fallback bool) bool {
	if opts == nil {
		return fallback
	}
	value := opts.RawGetString(key)
	if value.IsNil() {
		return fallback
	}
	return value.Truthy()
}

func llmReplayOptionInt(opts *Table, key string, fallback int) int {
	if opts == nil {
		return fallback
	}
	value := opts.RawGetString(key)
	if value.IsNil() {
		return fallback
	}
	if n := toInt(value); n != 0 {
		return int(n)
	}
	return fallback
}

func llmReplayIdentityEqual(a, b Value) bool {
	if a.IsString() || b.IsString() {
		return a.Str() == b.Str()
	}
	return a.Equal(b)
}

func llmReplayIdentityString(v Value) string {
	if v.IsNil() {
		return "<nil>"
	}
	if v.IsString() {
		return v.Str()
	}
	return v.String()
}
