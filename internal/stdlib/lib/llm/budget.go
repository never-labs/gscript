package llm

const (
	BudgetKey          = "budget"
	BudgetTokens       = "tokens"
	BudgetTurns        = "turns"
	BudgetCalls        = "calls"
	BudgetMoney        = "money"
	BudgetTime         = "time"
	OptionBudgetTokens = "budget_tokens"
	OptionBudgetTurns  = "budget_turns"
	OptionBudgetCalls  = "budget_calls"
	OptionBudgetMoney  = "budget_money"
	OptionBudgetTime   = "budget_time"
)

// BudgetLimits is the runtime-independent scalar shape for LLM loop budgets.
// Negative values mean the corresponding dimension is unlimited.
type BudgetLimits struct {
	MaxTokens      int64
	MaxTurns       int64
	MaxCalls       int64
	MaxMoney       float64
	MaxTimeSeconds float64
}

// DefaultBudgetLimits returns the unlimited budget sentinel values used by the
// runtime adapter before script-visible options are applied.
func DefaultBudgetLimits() BudgetLimits {
	return BudgetLimits{
		MaxTokens:      -1,
		MaxTurns:       -1,
		MaxCalls:       -1,
		MaxMoney:       -1,
		MaxTimeSeconds: -1,
	}
}

// NormalizeBudgetLimits clamps negative limits to the unlimited sentinel while
// preserving zero as an explicit limit.
func NormalizeBudgetLimits(limits BudgetLimits) BudgetLimits {
	if limits.MaxTokens < 0 {
		limits.MaxTokens = -1
	}
	if limits.MaxTurns < 0 {
		limits.MaxTurns = -1
	}
	if limits.MaxCalls < 0 {
		limits.MaxCalls = -1
	}
	if limits.MaxMoney < 0 {
		limits.MaxMoney = -1
	}
	if limits.MaxTimeSeconds < 0 {
		limits.MaxTimeSeconds = -1
	}
	return limits
}
