package ai

import "fmt"

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
	"max_steps",
	"max_tool_retries",
	"max_history_tokens",
	"force_tool",
	"stop",
	"metadata",
	"output",
	"output_repair",
	"output_retries",
	"budget",
	"budget_tokens",
	"budget_turns",
	"budget_calls",
	"budget_money",
	"budget_time",
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
