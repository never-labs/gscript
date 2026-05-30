package gscript

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gscript/gscript/internal/runtime"
)

// LLMProvider is the Go embedding hook behind the llm standard library.
// Implementations can call a remote model API, a local model, or a test double.
type LLMProvider interface {
	Turn(context.Context, LLMTurnRequest) (LLMTurnResult, error)
}

type LLMMessage = runtime.LLMMessage
type LLMTool = runtime.LLMTool
type LLMToolCall = runtime.LLMToolCall
type LLMTurnRequest = runtime.LLMTurnRequest
type LLMTurnResult = runtime.LLMTurnResult
type LLMTurnUsage = runtime.LLMTurnUsage

// WithLLMProvider installs the provider used by llm.turn. A nil provider makes
// llm.turn return a provider error.
func WithLLMProvider(provider LLMProvider) Option {
	return func(o *vmOptions) { o.llmProvider = provider }
}

// WithLLMCommand installs a simple command-backed provider. It is intended for
// local tooling and tests, including wrappers such as glm_cc. The prompt is
// rendered from the request messages and passed as the final command argument.
func WithLLMCommand(command string, args ...string) Option {
	return WithLLMProvider(CommandLLMProvider{Command: command, Args: args})
}

type CommandLLMProvider struct {
	Command string
	Args    []string
	Model   string
}

func (p CommandLLMProvider) Turn(ctx context.Context, req LLMTurnRequest) (LLMTurnResult, error) {
	if p.Command == "" {
		return LLMTurnResult{}, fmt.Errorf("llm command not configured")
	}
	args := append([]string{}, p.Args...)
	if p.Model != "" && req.Model == "" {
		req.Model = p.Model
	}
	if req.Model != "" && !containsModelFlag(args) {
		args = append(args, "--model", req.Model)
	}
	args = append(args, renderLLMPrompt(req))
	out, err := exec.CommandContext(ctx, p.Command, args...).CombinedOutput()
	if err != nil {
		return LLMTurnResult{}, fmt.Errorf("%s: %w: %s", p.Command, err, strings.TrimSpace(string(out)))
	}
	return LLMTurnResult{
		Status: "final_answer",
		Text:   strings.TrimRight(string(out), "\n"),
	}, nil
}

type llmProviderAdapter struct {
	provider LLMProvider
}

func (a llmProviderAdapter) Turn(ctx context.Context, req runtime.LLMTurnRequest) (runtime.LLMTurnResult, error) {
	return a.provider.Turn(ctx, req)
}

func containsModelFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--model" || strings.HasPrefix(arg, "--model=") {
			return true
		}
	}
	return false
}

func renderLLMPrompt(req LLMTurnRequest) string {
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
