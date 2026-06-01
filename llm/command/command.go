package command

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/never-labs/leia/llm"
)

// Provider is a simple command-backed LLM provider. It is intended for local
// tooling and tests. The prompt is rendered from the request messages and
// passed as the final command argument.
type Provider struct {
	Command string
	Args    []string
	Model   string
}

func (p Provider) Turn(ctx context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	if p.Command == "" {
		return llm.TurnResult{}, fmt.Errorf("llm command not configured")
	}
	args := append([]string{}, p.Args...)
	if p.Model != "" && req.Model == "" {
		req.Model = p.Model
	}
	if req.Model != "" && !containsModelFlag(args) {
		args = append(args, "--model", req.Model)
	}
	args = append(args, renderPrompt(req))
	out, err := exec.CommandContext(ctx, p.Command, args...).CombinedOutput()
	if err != nil {
		return llm.TurnResult{}, fmt.Errorf("%s: %w: %s", p.Command, err, strings.TrimSpace(string(out)))
	}
	return llm.TurnResult{
		Status: "final_answer",
		Text:   strings.TrimRight(string(out), "\n"),
	}, nil
}

func containsModelFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--model" || strings.HasPrefix(arg, "--model=") {
			return true
		}
	}
	return false
}

func renderPrompt(req llm.TurnRequest) string {
	var b strings.Builder
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			b.WriteString("System: ")
		case "assistant":
			b.WriteString("Assistant: ")
		case "tool":
			b.WriteString("Tool: ")
		default:
			b.WriteString("User: ")
		}
		if msg.Text != "" {
			b.WriteString(msg.Text)
		} else if msg.Error != "" {
			b.WriteString(msg.Error)
		} else if msg.Value != nil {
			b.WriteString(fmt.Sprint(msg.Value))
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
