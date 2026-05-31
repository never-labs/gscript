package ai

// HistoryMessage is the runtime-independent shape used by conversation
// history helpers. Runtime adapters populate Fields from script table keys.
type HistoryMessage struct {
	Role      string
	Tool      string
	ToolUseID string
	Error     string
	Fields    map[string]string
}

// HistoryFilterValue carries the small amount of script value behavior that
// history matching needs without depending on runtime.Value.
type HistoryFilterValue struct {
	String string
	Truthy bool
}

// MatchHistoryMessage applies the public history.find/history.last filter
// rules to a message summary.
func MatchHistoryMessage(msg HistoryMessage, filters map[string]HistoryFilterValue) bool {
	for key, want := range filters {
		switch key {
		case "role":
			if msg.Role != want.String {
				return false
			}
		case "tool":
			if msg.Tool == "" && msg.Role == "tool" {
				return false
			}
			if msg.Tool != want.String {
				return false
			}
		case "tool_use_id", "id":
			if msg.ToolUseID != want.String {
				return false
			}
		case "has_error":
			if want.Truthy != (msg.Error != "") {
				return false
			}
		default:
			if msg.Fields[key] != want.String {
				return false
			}
		}
	}
	return true
}
