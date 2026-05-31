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

// NewTextMessage normalizes simple message construction shared by msg.*,
// llm.* helpers, and agent loop internals.
func NewTextMessage(role, text string) TextMessage {
	return TextMessage{Role: role, Text: text}
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
