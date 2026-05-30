package gscript

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOpenAICompatibleEndpoint = "https://api.openai.com/v1/chat/completions"

// OpenAICompatibleLLMProvider adapts llm.turn to the OpenAI Chat Completions
// wire format used by OpenAI and many local or third-party model gateways.
type OpenAICompatibleLLMProvider struct {
	Endpoint     string
	APIKey       string
	Model        string
	Client       *http.Client
	Headers      map[string]string
	Timeout      time.Duration
	MaxAttempts  int
	RetryBackoff time.Duration
}

// OpenAICompatibleLLMError reports a non-2xx response from an
// OpenAI-compatible chat-completions endpoint.
type OpenAICompatibleLLMError struct {
	StatusCode int
	Body       string
	Retryable  bool
}

func (e *OpenAICompatibleLLMError) Error() string {
	if e == nil {
		return "openai-compatible llm error"
	}
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("openai-compatible llm status %d", e.StatusCode)
	}
	return fmt.Sprintf("openai-compatible llm status %d: %s", e.StatusCode, body)
}

func WithOpenAICompatibleLLM(endpoint, apiKey, model string) Option {
	return WithLLMProvider(OpenAICompatibleLLMProvider{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}

func (p OpenAICompatibleLLMProvider) Turn(ctx context.Context, req LLMTurnRequest) (LLMTurnResult, error) {
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = defaultOpenAICompatibleEndpoint
	}
	model := req.Model
	if model == "" {
		model = p.Model
	}
	if model == "" {
		return LLMTurnResult{}, fmt.Errorf("openai-compatible llm model not configured")
	}
	body, err := json.Marshal(openAIChatRequestFromTurn(req, model))
	if err != nil {
		return LLMTurnResult{}, err
	}
	attempts := p.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		res, retry, err := p.turnOnce(ctx, endpoint, body)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !retry || attempt == attempts-1 {
			return LLMTurnResult{}, err
		}
		if err := waitOpenAIRetry(ctx, p.RetryBackoff); err != nil {
			return LLMTurnResult{}, err
		}
	}
	return LLMTurnResult{}, lastErr
}

func (p OpenAICompatibleLLMProvider) turnOnce(ctx context.Context, endpoint string, body []byte) (LLMTurnResult, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return LLMTurnResult{}, false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	for k, v := range p.Headers {
		httpReq.Header.Set(k, v)
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return LLMTurnResult{}, ctx.Err() == nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		retryable := openAIRetryableStatus(resp.StatusCode)
		return LLMTurnResult{}, retryable, &OpenAICompatibleLLMError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(data)),
			Retryable:  retryable,
		}
	}
	var out openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return LLMTurnResult{}, false, err
	}
	return openAITurnResult(out), false, nil
}

func openAIRetryableStatus(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusConflict ||
		status == http.StatusTooManyRequests ||
		status >= 500
}

func waitOpenAIRetry(ctx context.Context, backoff time.Duration) error {
	if backoff <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type openAIChatRequest struct {
	Model          string            `json:"model"`
	Messages       []openAIMessage   `json:"messages"`
	Tools          []openAITool      `json:"tools,omitempty"`
	ToolChoice     any               `json:"tool_choice,omitempty"`
	MaxTokens      int64             `json:"max_tokens,omitempty"`
	Temperature    *float64          `json:"temperature,omitempty"`
	TopP           *float64          `json:"top_p,omitempty"`
	ResponseFormat any               `json:"response_format,omitempty"`
	Stream         bool              `json:"stream,omitempty"`
	Stop           []string          `json:"stop,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatResponse struct {
	Choices []struct {
		FinishReason string        `json:"finish_reason"`
		Message      openAIMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

func openAIChatRequestFromTurn(req LLMTurnRequest, model string) openAIChatRequest {
	out := openAIChatRequest{
		Model:          model,
		Messages:       make([]openAIMessage, 0, len(req.Messages)),
		MaxTokens:      req.MaxTokens,
		Temperature:    cloneFloat64Ptr(req.Temperature),
		TopP:           cloneFloat64Ptr(req.TopP),
		ResponseFormat: cloneLLMAny(req.ResponseFormat),
		Stream:         req.Stream,
		Stop:           append([]string(nil), req.Stop...),
		Metadata:       cloneStringMap(req.Metadata),
	}
	for _, msg := range req.Messages {
		out.Messages = append(out.Messages, openAIMessageFromLLM(msg))
	}
	for _, tool := range req.Tools {
		out.Tools = append(out.Tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  openAIToolSchema(tool),
			},
		})
	}
	if req.ForceTool != "" {
		out.ToolChoice = openAIToolChoice(req.ForceTool)
	}
	return out
}

func openAIMessageFromLLM(msg LLMMessage) openAIMessage {
	out := openAIMessage{Role: msg.Role}
	if out.Role == "" {
		out.Role = "user"
	}
	if msg.ToolCall != nil {
		out.ToolCalls = []openAIToolCall{openAIToolCallFromLLM(*msg.ToolCall)}
		return out
	}
	if msg.ToolUseID != "" {
		out.ToolCallID = msg.ToolUseID
	}
	if msg.Error != "" {
		out.Content = msg.Error
	} else if msg.Text != "" {
		out.Content = msg.Text
	} else if msg.Value != nil {
		data, err := json.Marshal(msg.Value)
		if err == nil {
			out.Content = string(data)
		} else {
			out.Content = fmt.Sprint(msg.Value)
		}
	}
	return out
}

func openAIToolCallFromLLM(call LLMToolCall) openAIToolCall {
	data, err := json.Marshal(call.Args)
	if err != nil {
		data = []byte("{}")
	}
	return openAIToolCall{
		ID:   call.ID,
		Type: "function",
		Function: openAIToolFunction{
			Name:      call.Tool,
			Arguments: string(data),
		},
	}
}

func openAIToolSchema(tool LLMTool) any {
	if tool.Schema != nil {
		return cloneLLMAny(tool.Schema)
	}
	properties := map[string]any{}
	for _, param := range tool.Params {
		properties[param] = map[string]any{"type": "string"}
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   append([]string(nil), tool.Params...),
	}
}

func openAIToolChoice(force string) any {
	switch force {
	case "any", "auto", "none":
		return force
	default:
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": force,
			},
		}
	}
}

func openAITurnResult(resp openAIChatResponse) LLMTurnResult {
	res := LLMTurnResult{
		Usage: LLMTurnUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
	if len(resp.Choices) == 0 {
		res.Status = "stop"
		return res
	}
	choice := resp.Choices[0]
	res.Reason = choice.FinishReason
	res.Text = choice.Message.Content
	for _, call := range choice.Message.ToolCalls {
		res.Calls = append(res.Calls, llmToolCallFromOpenAI(call))
	}
	if len(res.Calls) > 0 {
		res.Status = "tool_calls"
	} else if choice.FinishReason == "length" || choice.FinishReason == "content_filter" {
		res.Status = "stop"
	} else {
		res.Status = "final_answer"
	}
	return res
}

func llmToolCallFromOpenAI(call openAIToolCall) LLMToolCall {
	args := map[string]any{}
	if call.Function.Arguments != "" {
		dec := json.NewDecoder(strings.NewReader(call.Function.Arguments))
		dec.UseNumber()
		if err := dec.Decode(&args); err == nil {
			for k, v := range args {
				args[k] = normalizeOpenAIJSON(v)
			}
		}
	}
	return LLMToolCall{
		ID:   call.ID,
		Tool: call.Function.Name,
		Args: args,
	}
}

func normalizeOpenAIJSON(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = normalizeOpenAIJSON(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = normalizeOpenAIJSON(v)
		}
		return out
	default:
		return x
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneFloat64Ptr(src *float64) *float64 {
	if src == nil {
		return nil
	}
	out := *src
	return &out
}
