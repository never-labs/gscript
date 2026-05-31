package ai

import (
	"strings"
)

const defaultStructuredOutputRepairPrompt = "Return only a JSON object that matches the requested output shape."

// StructuredOutputRepairPrompt formats the retry prompt used when an agent
// response fails structured output validation.
func StructuredOutputRepairPrompt(customPrompt, previousText, validationMessage, outputShape string) string {
	prompt := strings.TrimSpace(customPrompt)
	if prompt == "" {
		prompt = defaultStructuredOutputRepairPrompt
	}
	var b strings.Builder
	b.WriteString(prompt)
	if validationMessage != "" {
		b.WriteString("\nValidation error: ")
		b.WriteString(validationMessage)
	}
	if outputShape != "" {
		b.WriteString("\nOutput shape example: ")
		b.WriteString(outputShape)
	}
	b.WriteString("\nPrevious response:\n")
	b.WriteString(previousText)
	return b.String()
}
