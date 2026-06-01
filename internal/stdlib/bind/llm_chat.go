package bind

import (
	"fmt"
	"strings"
)

func BuildChatLib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{Name: "chat." + name, Fn: fn}))
	}

	set("merge", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'chat.merge' (history, additions expected)")
		}
		left := llmMessageValuesFromTable(args[0].Table())
		right := llmMessageValuesFromTable(args[1].Table())
		out := make([]Value, 0, len(left)+len(right))
		out = append(out, left...)
		out = append(out, right...)
		return []Value{llmTableFromValues(out)}, nil
	})

	set("window", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument to 'chat.window' (history, max_tokens expected)")
		}
		maxTokens := toInt(args[1])
		if maxTokens < 0 {
			return nil, fmt.Errorf("bad argument #2 to 'chat.window' (non-negative max_tokens expected)")
		}
		out := chatWindow(args[0].Table(), maxTokens)
		return []Value{llmTableFromValues(out)}, nil
	})

	set("token_count", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'chat.token_count'")
		}
		return []Value{IntValue(chatTokenCountValue(args[0]))}, nil
	})

	set("summarize", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'chat.summarize' (history expected)")
		}
		opts := (*Table)(nil)
		if len(args) >= 2 && args[1].IsTable() {
			opts = args[1].Table()
		}
		return []Value{chatSummaryValue(args[0].Table(), opts)}, nil
	})

	return t
}

func chatWindow(history *Table, maxTokens int64) []Value {
	values := llmMessageValuesFromTable(history)
	if maxTokens == 0 || len(values) == 0 {
		return nil
	}
	var out []Value
	var total int64
	for i := len(values) - 1; i >= 0; i-- {
		cost := chatTokenCountValue(values[i])
		if len(out) > 0 && total+cost > maxTokens {
			break
		}
		if len(out) == 0 && cost > maxTokens {
			break
		}
		total += cost
		out = append(out, values[i])
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func chatTokenCountValue(v Value) int64 {
	if v.IsTable() {
		t := v.Table()
		var total int64
		if n := t.Length(); n > 0 {
			for i := 1; i <= n; i++ {
				total += chatTokenCountValue(t.RawGet(IntValue(int64(i))))
			}
			return total
		}
		total += chatTokenEstimate(t.RawGetString("role").Str())
		total += chatTokenEstimate(t.RawGetString("text").Str())
		total += chatTokenEstimate(t.RawGetString("error").Str())
		total += chatTokenEstimate(t.RawGetString("tool_use_id").Str())
		if call := t.RawGetString("tool_call"); call.IsTable() {
			total += chatTokenCountValue(call)
		}
		if value := t.RawGetString("value"); !value.IsNil() {
			total += chatTokenCountValue(value)
		}
		if args := t.RawGetString("args"); args.IsTable() {
			total += chatTokenCountValue(args)
		}
		return total
	}
	if v.IsString() {
		return chatTokenEstimate(v.Str())
	}
	if v.IsNil() {
		return 0
	}
	return chatTokenEstimate(v.String())
}

func chatTokenEstimate(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	words := int64(len(strings.Fields(s)))
	chars := int64((len(s) + 3) / 4)
	if chars > words {
		return chars
	}
	return words
}

func chatSummaryValue(history, opts *Table) Value {
	maxChars := int64(2048)
	if opts != nil {
		if v := toInt(opts.RawGetString("max_chars")); v > 0 {
			maxChars = v
		}
	}
	parts := make([]string, 0, history.Length())
	for _, msg := range llmMessageValuesFromTable(history) {
		if !msg.IsTable() {
			continue
		}
		t := msg.Table()
		role := t.RawGetString("role").Str()
		text := t.RawGetString("text").Str()
		if text == "" {
			text = t.RawGetString("error").Str()
		}
		if text == "" && !t.RawGetString("value").IsNil() {
			text = t.RawGetString("value").String()
		}
		if role == "" {
			role = "message"
		}
		if text != "" {
			parts = append(parts, role+": "+text)
		}
	}
	summary := strings.Join(parts, "\n")
	if int64(len(summary)) > maxChars {
		if maxChars <= 3 {
			summary = summary[:maxChars]
		} else {
			summary = summary[:maxChars-3] + "..."
		}
	}
	out := NewTable()
	out.RawSetString("role", StringValue("system"))
	out.RawSetString("text", StringValue(summary))
	out.RawSetString("tokens", IntValue(chatTokenEstimate(summary)))
	return TableValue(out)
}
