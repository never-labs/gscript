package bind

import "fmt"

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
	return session, NilValue()
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
