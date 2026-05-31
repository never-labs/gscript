package ai

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"

	StatusFinalAnswer = "final_answer"
	StatusToolCalls   = "tool_calls"
)

// TextMessage is the runtime-independent shape for simple role/text messages.
type TextMessage struct {
	Role string
	Text string
}

// AssistantCallMessage is the runtime-independent shape for a message that
// carries a provider tool call request.
type AssistantCallMessage struct {
	Role        string
	ToolCallKey string
}

// ToolResultMessage is the runtime-independent shape for a tool result or
// error message.
type ToolResultMessage struct {
	Role         string
	ToolUseIDKey string
	ValueKey     string
	ErrorKey     string
}

// NewTextMessage normalizes simple message construction shared by msg.*,
// llm.* helpers, and agent loop internals.
func NewTextMessage(role, text string) TextMessage {
	return TextMessage{Role: role, Text: text}
}

// NewAssistantCallMessage normalizes assistant tool-call message keys shared by
// msg.* and llm.* helpers.
func NewAssistantCallMessage() AssistantCallMessage {
	return AssistantCallMessage{Role: RoleAssistant, ToolCallKey: "tool_call"}
}

// NewToolResultMessage normalizes tool result/error message keys shared by
// msg.* and llm.* helpers.
func NewToolResultMessage() ToolResultMessage {
	return ToolResultMessage{
		Role:         RoleTool,
		ToolUseIDKey: "tool_use_id",
		ValueKey:     "value",
		ErrorKey:     "error",
	}
}

// DefaultTurnStatus derives the public turn status when a provider omits it.
func DefaultTurnStatus(status string, toolCallCount int) string {
	if status != "" {
		return status
	}
	if toolCallCount > 0 {
		return StatusToolCalls
	}
	return StatusFinalAnswer
}
