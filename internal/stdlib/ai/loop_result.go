package ai

const (
	ErrorKindBudget    = "budget"
	ErrorKindCancelled = "cancelled"
	ErrorKindDeadline  = "deadline"

	ReactStatusDone    = "done"
	ReactStatusStopped = "stopped"
	ReactStatusPending = "pending"
)

// ErrorShape describes public loop error tables without depending on runtime
// Value/Table adapters.
type ErrorShape struct {
	Kind      string
	Dimension string
	Message   string
	Limit     int64
	Used      int64
}

// ReactResultShape describes the scalar fields of loop.react-style results.
// Runtime owns embedding provider result values and history arrays.
type ReactResultShape struct {
	Status string
	Text   string
	Reason string
}

// BudgetExceededError normalizes budget failure fields shared by loop
// execution and future agent surfaces.
func BudgetExceededError(dimension string, limit, used int64) ErrorShape {
	return ErrorShape{
		Kind:      ErrorKindBudget,
		Dimension: dimension,
		Message:   "llm budget exceeded: " + dimension,
		Limit:     limit,
		Used:      used,
	}
}

// CancelledError normalizes cancellation/deadline failure fields.
func CancelledError(reason string) ErrorShape {
	if reason == "" {
		reason = ErrorKindCancelled
	}
	kind := ErrorKindCancelled
	if IsDeadlineReason(reason) {
		kind = ErrorKindDeadline
	}
	return ErrorShape{Kind: kind, Message: reason}
}

// IsDeadlineReason identifies cancellation messages that should be surfaced as
// deadline errors instead of generic cancellation.
func IsDeadlineReason(reason string) bool {
	return reason == "deadline exceeded" || reason == "context deadline exceeded"
}

// ReactResult normalizes the scalar result fields for built-in loops.
func ReactResult(status, text, reason string) ReactResultShape {
	return ReactResultShape{Status: status, Text: text, Reason: reason}
}
