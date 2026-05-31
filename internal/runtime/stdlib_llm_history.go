package runtime

import "fmt"

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

	matches := func(msg *Table, opts *Table) bool {
		if msg == nil {
			return false
		}
		for _, key := range opts.PairsKeysSnapshot() {
			if !key.IsString() {
				continue
			}
			k := key.Str()
			want := opts.RawGet(key)
			switch k {
			case "role":
				if msg.RawGetString("role").Str() != want.Str() {
					return false
				}
			case "tool":
				name := ""
				if call := msg.RawGetString("tool_call"); call.IsTable() {
					name = call.Table().RawGetString("tool").Str()
				}
				if name == "" && msg.RawGetString("role").Str() == "tool" {
					// tool result messages don't carry the tool name directly;
					// match via tool_use_id prefix or skip.
					return false
				}
				if name != want.Str() {
					return false
				}
			case "tool_use_id", "id":
				if msg.RawGetString("tool_use_id").Str() != want.Str() {
					return false
				}
			case "has_error":
				if want.Truthy() != (msg.RawGetString("error").Str() != "") {
					return false
				}
			default:
				if msg.RawGetString(k).Str() != want.Str() {
					return false
				}
			}
		}
		return true
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
		for i := 1; i <= ht.Length(); i++ {
			m := ht.RawGet(IntValue(int64(i)))
			if !m.IsTable() {
				continue
			}
			if !matches(m.Table(), opts) {
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
