package bind

import (
	"fmt"

	stdlibllm "github.com/never-labs/leia/internal/stdlib/lib/llm"
)

// BuildLLMHistoryLib creates the "history" stdlib table with helpers for
// searching and mutating conversation history values produced by agents and
// llm.react. The block-form `messages { }` constructor remains the preferred
// way to assemble *initial* history; these helpers cover the dynamic case where
// the user reads/appends to a history list at runtime.
func BuildLLMHistoryLib() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{Name: "history." + name, Fn: fn}))
	}

	iter := func(historyVal, optsVal Value, yield func(idx int, msg Value) bool) {
		if !historyVal.IsTable() {
			return
		}
		ht := historyVal.Table()
		var opts *Table
		if optsVal.IsTable() {
			opts = optsVal.Table()
		} else {
			opts = NewTable()
		}
		filters := llmHistoryFilters(opts)
		for i := 1; i <= ht.Length(); i++ {
			m := ht.RawGet(IntValue(int64(i)))
			if !m.IsTable() {
				continue
			}
			if !stdlibllm.MatchHistoryMessage(llmHistoryMessageSummary(m.Table()), filters) {
				continue
			}
			if !yield(i, m) {
				return
			}
		}
	}

	set("find", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'history.find' (history expected)")
		}
		optsVal := NilValue()
		if len(args) >= 2 {
			optsVal = args[1]
		}
		var foundMsg Value = NilValue()
		foundIdx := int64(-1)
		iter(args[0], optsVal, func(idx int, msg Value) bool {
			foundMsg = msg
			foundIdx = int64(idx)
			return false
		})
		return []Value{foundMsg, IntValue(foundIdx)}, nil
	})

	set("find_all", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'history.find_all' (history expected)")
		}
		optsVal := NilValue()
		if len(args) >= 2 {
			optsVal = args[1]
		}
		out := NewTable()
		iter(args[0], optsVal, func(idx int, msg Value) bool {
			out.RawSet(IntValue(int64(out.Length()+1)), msg)
			return true
		})
		return []Value{TableValue(out)}, nil
	})

	set("last", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'history.last' (history expected)")
		}
		optsVal := NilValue()
		if len(args) >= 2 {
			optsVal = args[1]
		}
		var foundMsg Value = NilValue()
		foundIdx := int64(-1)
		iter(args[0], optsVal, func(idx int, msg Value) bool {
			foundMsg = msg
			foundIdx = int64(idx)
			return true
		})
		return []Value{foundMsg, IntValue(foundIdx)}, nil
	})

	set("append", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument to 'history.append' (history, message expected)")
		}
		h := args[0].Table()
		h.RawSet(IntValue(int64(h.Length()+1)), args[1])
		return []Value{args[0]}, nil
	})

	return t
}

func llmHistoryMessageSummary(msg *Table) stdlibllm.HistoryMessage {
	if msg == nil {
		return stdlibllm.HistoryMessage{}
	}
	fields := make(map[string]string)
	for _, key := range msg.PairsKeysSnapshot() {
		if !key.IsString() {
			continue
		}
		fields[key.Str()] = msg.RawGet(key).Str()
	}
	tool := ""
	if call := msg.RawGetString("tool_call"); call.IsTable() {
		tool = call.Table().RawGetString("tool").Str()
	}
	return stdlibllm.HistoryMessage{
		Role:      msg.RawGetString("role").Str(),
		Tool:      tool,
		ToolUseID: msg.RawGetString("tool_use_id").Str(),
		Error:     msg.RawGetString("error").Str(),
		Fields:    fields,
	}
}

func llmHistoryFilters(opts *Table) map[string]stdlibllm.HistoryFilterValue {
	out := make(map[string]stdlibllm.HistoryFilterValue)
	if opts == nil {
		return out
	}
	for _, key := range opts.PairsKeysSnapshot() {
		if !key.IsString() {
			continue
		}
		want := opts.RawGet(key)
		out[key.Str()] = stdlibllm.HistoryFilterValue{
			String: want.Str(),
			Truthy: want.Truthy(),
		}
	}
	return out
}
