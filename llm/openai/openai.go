package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/never-labs/leia/llm"
)

const defaultOpenAICompatibleEndpoint = "https://api.openai.com/v1/chat/completions"

// Provider adapts llm.turn to the OpenAI Chat Completions
// wire format used by OpenAI and many local or third-party model gateways.
type Provider struct {
	Endpoint     string
	APIKey       string
	Model        string
	Client       *http.Client
	Headers      map[string]string
	Timeout      time.Duration
	MaxAttempts  int
	RetryBackoff time.Duration
}

// Error reports a non-2xx response from an
// OpenAI-compatible chat-completions endpoint.
type Error struct {
	StatusCode int
	Body       string
	Retryable  bool
}

func (e *Error) Error() string {
	if e == nil {
		return "openai-compatible llm error"
	}
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("openai-compatible llm status %d", e.StatusCode)
	}
	return fmt.Sprintf("openai-compatible llm status %d: %s", e.StatusCode, body)
}

func (e *Error) LLMProviderErrorKind() string {
	if e == nil {
		return llm.ProviderErrorProvider
	}
	return HTTPStatusErrorKind(e.StatusCode, e.Retryable)
}

func (p Provider) Turn(ctx context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	return p.turn(ctx, req, nil)
}

func (p Provider) StreamTurn(ctx context.Context, req llm.TurnRequest, sink llm.StreamSink) (llm.TurnResult, error) {
	req.Stream = true
	return p.turn(ctx, req, sink)
}

func (p Provider) turn(ctx context.Context, req llm.TurnRequest, sink llm.StreamSink) (llm.TurnResult, error) {
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}
	endpoint := ChatCompletionsEndpoint(p.Endpoint)
	model := req.Model
	if model == "" {
		model = p.Model
	}
	if model == "" {
		return llm.TurnResult{}, fmt.Errorf("openai-compatible llm model not configured")
	}
	body, err := json.Marshal(openAIChatRequestFromTurn(req, model))
	if err != nil {
		return llm.TurnResult{}, err
	}
	attempts := p.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		res, retry, err := p.turnOnce(ctx, endpoint, body, req.Stream, sink)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !retry || attempt == attempts-1 {
			return llm.TurnResult{}, err
		}
		if err := waitOpenAIRetry(ctx, p.RetryBackoff); err != nil {
			return llm.TurnResult{}, err
		}
	}
	return llm.TurnResult{}, lastErr
}

func (p Provider) turnOnce(ctx context.Context, endpoint string, body []byte, stream bool, sink llm.StreamSink) (llm.TurnResult, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return llm.TurnResult{}, false, err
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
		return llm.TurnResult{}, ctx.Err() == nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		retryable := openAIRetryableStatus(resp.StatusCode)
		return llm.TurnResult{}, retryable, &Error{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(data)),
			Retryable:  retryable,
		}
	}
	if stream {
		res, err := decodeOpenAIStream(resp.Body, sink)
		return res, false, err
	}
	var out openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return llm.TurnResult{}, false, err
	}
	return openAITurnResult(out), false, nil
}

func decodeOpenAIStream(body io.Reader, sink llm.StreamSink) (llm.TurnResult, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	state := openAIStreamState{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk openAIChatStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return llm.TurnResult{}, err
		}
		if err := state.apply(chunk, sink); err != nil {
			return llm.TurnResult{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return llm.TurnResult{}, err
	}
	return state.result(), nil
}

func openAIRetryableStatus(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusConflict ||
		status == http.StatusTooManyRequests ||
		status >= 500
}

func HTTPStatusErrorKind(status int, retryable bool) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return llm.ProviderErrorAuth
	case http.StatusTooManyRequests:
		return llm.ProviderErrorRateLimit
	}
	if retryable {
		return llm.ProviderErrorNetwork
	}
	if status >= 400 && status < 500 {
		return llm.ProviderErrorRequest
	}
	return llm.ProviderErrorProvider
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

func ChatCompletionsEndpoint(endpoint string) string {
	if endpoint == "" {
		return defaultOpenAICompatibleEndpoint
	}
	trimmed := strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(trimmed, "/v1/chat/completions") || strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed + "/chat/completions"
	}
	return trimmed + "/v1/chat/completions"
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

type openAIChatStreamResponse struct {
	Choices []struct {
		FinishReason string             `json:"finish_reason"`
		Delta        openAIMessageDelta `json:"delta"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

type openAIMessageDelta struct {
	Role      string                `json:"role,omitempty"`
	Content   string                `json:"content,omitempty"`
	ToolCalls []openAIToolCallDelta `json:"tool_calls,omitempty"`
}

type openAIToolCallDelta struct {
	Index    int                `json:"index"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function openAIToolFunction `json:"function,omitempty"`
}

type openAIStreamState struct {
	text         strings.Builder
	finishReason string
	usage        llm.TurnUsage
	toolCalls    []openAIToolCall
}

func (s *openAIStreamState) apply(chunk openAIChatStreamResponse, sink llm.StreamSink) error {
	if chunk.Usage.PromptTokens != 0 || chunk.Usage.CompletionTokens != 0 {
		s.usage.InputTokens = chunk.Usage.PromptTokens
		s.usage.OutputTokens = chunk.Usage.CompletionTokens
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	choice := chunk.Choices[0]
	if choice.FinishReason != "" {
		s.finishReason = choice.FinishReason
	}
	if choice.Delta.Content != "" {
		s.text.WriteString(choice.Delta.Content)
		if sink != nil {
			if err := sink(llm.StreamEvent{Type: "token", Token: choice.Delta.Content, Text: choice.Delta.Content}); err != nil {
				return err
			}
		}
	}
	for _, delta := range choice.Delta.ToolCalls {
		s.applyToolCallDelta(delta)
	}
	return nil
}

func (s *openAIStreamState) applyToolCallDelta(delta openAIToolCallDelta) {
	for len(s.toolCalls) <= delta.Index {
		s.toolCalls = append(s.toolCalls, openAIToolCall{Type: "function"})
	}
	call := &s.toolCalls[delta.Index]
	if delta.ID != "" {
		call.ID = delta.ID
	}
	if delta.Type != "" {
		call.Type = delta.Type
	}
	if delta.Function.Name != "" {
		call.Function.Name += delta.Function.Name
	}
	if delta.Function.Arguments != "" {
		call.Function.Arguments += delta.Function.Arguments
	}
}

func (s openAIStreamState) result() llm.TurnResult {
	res := llm.TurnResult{
		Text:   s.text.String(),
		Reason: s.finishReason,
		Usage:  s.usage,
	}
	for _, call := range s.toolCalls {
		if call.ID == "" && call.Function.Name == "" && call.Function.Arguments == "" {
			continue
		}
		res.Calls = append(res.Calls, llmToolCallFromOpenAI(call))
	}
	if len(res.Calls) > 0 {
		res.Status = "tool_calls"
	} else if s.finishReason == "length" || s.finishReason == "content_filter" {
		res.Status = "stop"
	} else {
		res.Status = "final_answer"
	}
	return res
}

func openAIChatRequestFromTurn(req llm.TurnRequest, model string) openAIChatRequest {
	out := openAIChatRequest{
		Model:          model,
		Messages:       make([]openAIMessage, 0, len(req.Messages)),
		MaxTokens:      req.MaxTokens,
		Temperature:    cloneFloat64Ptr(req.Temperature),
		TopP:           cloneFloat64Ptr(req.TopP),
		ResponseFormat: cloneAny(req.ResponseFormat),
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
				Parameters:  ToolSchema(tool),
			},
		})
	}
	if req.ForceTool != "" {
		out.ToolChoice = openAIToolChoice(req.ForceTool)
	}
	return out
}

func openAIMessageFromLLM(msg llm.Message) openAIMessage {
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

func openAIToolCallFromLLM(call llm.ToolCall) openAIToolCall {
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

func ToolSchema(tool llm.Tool) any {
	if tool.Schema != nil {
		return cloneAny(tool.Schema)
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

func openAITurnResult(resp openAIChatResponse) llm.TurnResult {
	res := llm.TurnResult{
		Usage: llm.TurnUsage{
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

func llmToolCallFromOpenAI(call openAIToolCall) llm.ToolCall {
	args := map[string]any{}
	if call.Function.Arguments != "" {
		dec := json.NewDecoder(strings.NewReader(call.Function.Arguments))
		dec.UseNumber()
		if err := dec.Decode(&args); err == nil {
			for k, v := range args {
				args[k] = NormalizeJSON(v)
			}
		}
	}
	return llm.ToolCall{
		ID:   call.ID,
		Tool: call.Function.Name,
		Args: args,
	}
}

func NormalizeJSON(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case float64:
		if x == float64(int64(x)) {
			return int64(x)
		}
		return x
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = NormalizeJSON(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = NormalizeJSON(v)
		}
		return out
	default:
		return x
	}
}

func NormalizeMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = NormalizeJSON(v)
	}
	return out
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

func cloneAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = cloneAny(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = cloneAny(v)
		}
		return out
	default:
		return x
	}
}
