package llm

import "testing"

func TestMatchHistoryMessage(t *testing.T) {
	msg := HistoryMessage{
		Role:      "assistant",
		Tool:      "lookup",
		ToolUseID: "call-1",
		Error:     "",
		Fields: map[string]string{
			"role":        "assistant",
			"custom_tag":  "search",
			"tool_use_id": "call-1",
		},
	}

	tests := []struct {
		name    string
		filters map[string]HistoryFilterValue
		want    bool
	}{
		{
			name: "role",
			filters: map[string]HistoryFilterValue{
				"role": {String: "assistant", Truthy: true},
			},
			want: true,
		},
		{
			name: "tool",
			filters: map[string]HistoryFilterValue{
				"tool": {String: "lookup", Truthy: true},
			},
			want: true,
		},
		{
			name: "id_alias",
			filters: map[string]HistoryFilterValue{
				"id": {String: "call-1", Truthy: true},
			},
			want: true,
		},
		{
			name: "custom_field",
			filters: map[string]HistoryFilterValue{
				"custom_tag": {String: "search", Truthy: true},
			},
			want: true,
		},
		{
			name: "has_error_false",
			filters: map[string]HistoryFilterValue{
				"has_error": {Truthy: false},
			},
			want: true,
		},
		{
			name: "mismatch",
			filters: map[string]HistoryFilterValue{
				"tool": {String: "fetch", Truthy: true},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchHistoryMessage(msg, tt.filters); got != tt.want {
				t.Fatalf("MatchHistoryMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchHistoryMessageToolResultWithoutToolName(t *testing.T) {
	msg := HistoryMessage{
		Role:      "tool",
		ToolUseID: "call-1",
		Fields: map[string]string{
			"role":        "tool",
			"tool_use_id": "call-1",
		},
	}

	if MatchHistoryMessage(msg, map[string]HistoryFilterValue{"tool": {String: "lookup", Truthy: true}}) {
		t.Fatalf("tool result without tool name should not match tool filter")
	}
	if !MatchHistoryMessage(msg, map[string]HistoryFilterValue{"tool_use_id": {String: "call-1", Truthy: true}}) {
		t.Fatalf("tool_use_id should match")
	}
}
