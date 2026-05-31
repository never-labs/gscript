package ai

const (
	defaultPlanPrompt    = "Create a concise execution plan. Do not call tools."
	defaultReflectPrompt = "Review the answer. If it can be improved, return the improved final answer only."
	executionPlanPrefix  = "Execution plan:\n"
)

// SelectPlanModel applies the model fallback used by plan_execute.
func SelectPlanModel(planModel, model string) string {
	if planModel != "" {
		return planModel
	}
	return model
}

// SelectReflectModel applies the model fallback used by reflect loops.
func SelectReflectModel(reflectModel, model string) string {
	if reflectModel != "" {
		return reflectModel
	}
	return model
}

// PlanPrompt returns the system prompt for the planning turn.
func PlanPrompt() string {
	return defaultPlanPrompt
}

// ExecutionPlanMessage formats a plan for reinjection into the execution
// history. The boolean tells the runtime adapter whether a message should be
// appended at all.
func ExecutionPlanMessage(plan string) (string, bool) {
	if plan == "" {
		return "", false
	}
	return executionPlanPrefix + plan, true
}

// ReflectPrompt applies the default prompt used by the reflect loop.
func ReflectPrompt(prompt string) string {
	if prompt != "" {
		return prompt
	}
	return defaultReflectPrompt
}

// ReflectIterations applies the public default for the reflect loop.
func ReflectIterations(maxIters int64) int64 {
	if maxIters <= 0 {
		return 1
	}
	return maxIters
}
