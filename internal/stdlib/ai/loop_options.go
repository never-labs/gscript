package ai

import "fmt"

const (
	OptionKeyMaxSteps = "max_steps"

	// DefaultSimpleMaxSteps is the public max_steps default for loop.simple.
	DefaultSimpleMaxSteps int64 = 1
	// DefaultReactMaxSteps is the internal safety default for react-style loops.
	DefaultReactMaxSteps int64 = 8
)

// LoopOptionKeys are the script-visible loop/turn options copied from an
// agent options table into the normalized request table.
var LoopOptionKeys = []string{
	"model",
	"tools",
	"max_tokens",
	"temperature",
	"top_p",
	"response_format",
	"stream",
	OptionKeyMaxSteps,
	"max_tool_retries",
	"max_history_tokens",
	"force_tool",
	"stop",
	"metadata",
	"output",
	"output_repair",
	"output_retries",
	BudgetKey,
	OptionBudgetTokens,
	OptionBudgetTurns,
	OptionBudgetCalls,
	OptionBudgetMoney,
	OptionBudgetTime,
	"ctx",
	"context",
	"cancel",
}

// LoopOptionInput is the runtime-independent subset needed to normalize loop
// options. Runtime adapters keep ownership of Value/Table conversion.
type LoopOptionInput struct {
	HasMessages         bool
	HasUser             bool
	HasMaxSteps         bool
	HasResponseFormat   bool
	HasStructuredOutput bool
	DefaultMaxSteps     int64
}

// LoopOptionPlan describes the normalization actions the runtime adapter must
// apply after reading script values.
type LoopOptionPlan struct {
	SynthesizeMessages    bool
	SetDefaultMaxSteps    bool
	DefaultMaxSteps       int64
	SetJSONResponseFormat bool
}

// ReactControls are the normalized loop controls used by built-in agent loops.
type ReactControls struct {
	MaxSteps         int
	MaxToolRetries   int
	MaxHistoryTokens int64
}

// NormalizeLoopOptions applies the public loop option defaults without
// depending on runtime Value/Table internals.
func NormalizeLoopOptions(input LoopOptionInput) (LoopOptionPlan, error) {
	if !input.HasMessages && !input.HasUser {
		return LoopOptionPlan{}, fmt.Errorf("loop requires messages or user")
	}
	plan := LoopOptionPlan{
		SynthesizeMessages: !input.HasMessages,
	}
	if input.DefaultMaxSteps > 0 && !input.HasMaxSteps {
		plan.SetDefaultMaxSteps = true
		plan.DefaultMaxSteps = input.DefaultMaxSteps
	}
	if !input.HasResponseFormat && input.HasStructuredOutput {
		plan.SetJSONResponseFormat = true
	}
	return plan, nil
}

// NormalizeReactControls applies runtime-independent defaults and clamps for
// the hot react loop. Value/Table decoding remains in the runtime adapter.
func NormalizeReactControls(maxSteps, maxToolRetries, maxHistoryTokens int64) ReactControls {
	if maxSteps <= 0 {
		maxSteps = DefaultReactMaxSteps
	}
	if maxToolRetries < 0 {
		maxToolRetries = 0
	}
	if maxHistoryTokens < 0 {
		maxHistoryTokens = 0
	}
	return ReactControls{
		MaxSteps:         int(maxSteps),
		MaxToolRetries:   int(maxToolRetries),
		MaxHistoryTokens: maxHistoryTokens,
	}
}
