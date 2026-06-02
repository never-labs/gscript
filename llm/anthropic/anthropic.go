package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/openai"
)

const (
	defaultAnthropicCompatibleEndpoint = "https://api.anthropic.com/v1/messages"
	defaultAnthropicVersion            = "2023-06-01"
)

// Provider adapts llm.turn to Anthropic's messages API
// wire format, including compatible gateways that expose the same protocol.
type Provider struct {
	Endpoint         string
	APIKey           string
	Model            string
	Client           *http.Client
	Headers          map[string]string
	AnthropicVersion string
	Timeout          time.Duration
	MaxAttempts      int
	RetryBackoff     time.Duration
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
	endpoint := anthropicMessagesEndpoint(p.Endpoint)
	model := req.Model
	if model == "" {
		model = p.Model
	}
	if model == "" {
		return llm.TurnResult{}, fmt.Errorf("anthropic-compatible llm model not configured")
	}
	body, err := json.Marshal(anthropicRequestFromTurn(req, model))
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
		if err := waitLLMRetry(ctx, p.RetryBackoff); err != nil {
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
		httpReq.Header.Set("x-api-key", p.APIKey)
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	version := p.AnthropicVersion
	if version == "" {
		version = defaultAnthropicVersion
	}
	httpReq.Header.Set("anthropic-version", version)
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
		retryable := llmRetryableStatus(resp.StatusCode)
		return llm.TurnResult{}, retryable, &Error{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(data)),
			Retryable:  retryable,
		}
	}
	if stream {
		res, err := decodeAnthropicStream(resp.Body, sink)
		return res, false, err
	}
	var out anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return llm.TurnResult{}, false, err
	}
	return anthropicTurnResult(out), false, nil
}

func decodeAnthropicStream(body io.Reader, sink llm.StreamSink) (llm.TurnResult, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	state := anthropicStreamState{}
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
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return llm.TurnResult{}, err
		}
		if err := state.apply(event, sink); err != nil {
			return llm.TurnResult{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return llm.TurnResult{}, err
	}
	return state.result(), nil
}

// Error reports a non-2xx response from an
// Anthropic-compatible messages endpoint.
type Error struct {
	StatusCode int
	Body       string
	Retryable  bool
}

func (e *Error) Error() string {
	if e == nil {
		return "anthropic-compatible llm error"
	}
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("anthropic-compatible llm status %d", e.StatusCode)
	}
	return fmt.Sprintf("anthropic-compatible llm status %d: %s", e.StatusCode, body)
}

func (e *Error) LLMProviderErrorKind() string {
	if e == nil {
		return llm.ProviderErrorProvider
	}
	return openai.HTTPStatusErrorKind(e.StatusCode, e.Retryable)
}

type anthropicRequest struct {
	Model         string             `json:"model"`
	System        string             `json:"system,omitempty"`
	Messages      []anthropicMessage `json:"messages"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    any                `json:"tool_choice,omitempty"`
	MaxTokens     int64              `json:"max_tokens,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Metadata      map[string]string  `json:"metadata,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

type anthropicResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Content      []struct {
		Type  string         `json:"type"`
		Text  string         `json:"text"`
		ID    string         `json:"id"`
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
	} `json:"content"`
	Usage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type  string         `json:"type"`
		Text  string         `json:"text"`
		ID    string         `json:"id"`
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Message struct {
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Usage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamState struct {
	text       strings.Builder
	reason     string
	usage      llm.TurnUsage
	toolBlocks map[int]*anthropicStreamToolBlock
}

type anthropicStreamToolBlock struct {
	id        string
	name      string
	inputJSON strings.Builder
	input     map[string]any
}

func anthropicMessagesEndpoint(endpoint string) string {
	if endpoint == "" {
		return defaultAnthropicCompatibleEndpoint
	}
	trimmed := strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(trimmed, "/v1/messages") || strings.HasSuffix(trimmed, "/messages") {
		return trimmed
	}
	return trimmed + "/v1/messages"
}

func anthropicRequestFromTurn(req llm.TurnRequest, model string) anthropicRequest {
	out := anthropicRequest{
		Model:         model,
		MaxTokens:     req.MaxTokens,
		Temperature:   cloneFloat64Ptr(req.Temperature),
		TopP:          cloneFloat64Ptr(req.TopP),
		StopSequences: append([]string(nil), req.Stop...),
		Metadata:      cloneStringMap(req.Metadata),
		Stream:        req.Stream,
	}
	if out.MaxTokens <= 0 {
		out.MaxTokens = 1024
	}
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if out.System == "" {
				out.System = msg.Text
			} else if msg.Text != "" {
				out.System += "\n" + msg.Text
			}
			continue
		}
		out.Messages = append(out.Messages, anthropicMessageFromLLM(msg))
	}
	if len(out.Messages) == 0 {
		out.Messages = append(out.Messages, anthropicMessage{Role: "user", Content: ""})
	}
	for _, tool := range req.Tools {
		out.Tools = append(out.Tools, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: openai.ToolSchema(tool),
		})
	}
	if req.ForceTool != "" {
		out.ToolChoice = anthropicToolChoice(req.ForceTool)
	}
	return out
}

func anthropicMessageFromLLM(msg llm.Message) anthropicMessage {
	role := msg.Role
	if role != "assistant" {
		role = "user"
	}
	if msg.ToolCall != nil {
		return anthropicMessage{Role: "assistant", Content: []anthropicContentBlock{anthropicToolUseFromLLM(*msg.ToolCall)}}
	}
	if msg.ToolUseID != "" {
		text := msg.Error
		if text == "" {
			text = fmt.Sprint(msg.Value)
		}
		return anthropicMessage{Role: "user", Content: []anthropicContentBlock{{
			Type:      "tool_result",
			ToolUseID: msg.ToolUseID,
			Content:   text,
		}}}
	}
	if msg.Text != "" || msg.Value == nil {
		return anthropicMessage{Role: role, Content: msg.Text}
	}
	data, err := json.Marshal(msg.Value)
	if err != nil {
		return anthropicMessage{Role: role, Content: fmt.Sprint(msg.Value)}
	}
	return anthropicMessage{Role: role, Content: string(data)}
}

func anthropicToolUseFromLLM(call llm.ToolCall) anthropicContentBlock {
	return anthropicContentBlock{
		Type:  "tool_use",
		ID:    call.ID,
		Name:  call.Tool,
		Input: cloneStringAnyMap(call.Args),
	}
}

func anthropicToolChoice(force string) any {
	switch force {
	case "any":
		return map[string]any{"type": "any"}
	case "auto":
		return map[string]any{"type": "auto"}
	case "none":
		return map[string]any{"type": "none"}
	default:
		return map[string]any{"type": "tool", "name": force}
	}
}

func anthropicTurnResult(resp anthropicResponse) llm.TurnResult {
	res := llm.TurnResult{
		Reason: resp.StopReason,
		Usage: llm.TurnUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
	}
	var text strings.Builder
	for _, block := range resp.Content {
		switch block.Type {
		case "tool_use":
			res.Calls = append(res.Calls, llm.ToolCall{
				ID:   block.ID,
				Tool: block.Name,
				Args: openai.NormalizeMap(block.Input),
			})
		case "text", "":
			if block.Text != "" {
				text.WriteString(block.Text)
			}
		}
	}
	res.Text = text.String()
	if len(res.Calls) > 0 {
		res.Status = "tool_calls"
	} else if resp.StopReason == "max_tokens" || resp.StopReason == "stop_sequence" {
		res.Status = "stop"
	} else {
		res.Status = "final_answer"
	}
	return res
}

func (s *anthropicStreamState) apply(event anthropicStreamEvent, sink llm.StreamSink) error {
	switch event.Type {
	case "message_start":
		s.applyUsage(event.Message.Usage.InputTokens, event.Message.Usage.OutputTokens)
	case "content_block_start":
		if event.ContentBlock.Type == "text" && event.ContentBlock.Text != "" {
			return s.appendText(event.ContentBlock.Text, sink)
		}
		if event.ContentBlock.Type == "tool_use" {
			block := s.toolBlock(event.Index)
			block.id = event.ContentBlock.ID
			block.name = event.ContentBlock.Name
			if len(event.ContentBlock.Input) > 0 {
				block.input = openai.NormalizeMap(event.ContentBlock.Input)
			}
		}
	case "content_block_delta":
		switch event.Delta.Type {
		case "text_delta":
			return s.appendText(event.Delta.Text, sink)
		case "input_json_delta":
			s.toolBlock(event.Index).inputJSON.WriteString(event.Delta.PartialJSON)
		}
	case "message_delta":
		if event.Delta.StopReason != "" {
			s.reason = event.Delta.StopReason
		}
		s.applyUsage(event.Usage.InputTokens, event.Usage.OutputTokens)
	case "message_stop":
		return nil
	}
	return nil
}

func (s *anthropicStreamState) appendText(text string, sink llm.StreamSink) error {
	if text == "" {
		return nil
	}
	s.text.WriteString(text)
	if sink == nil {
		return nil
	}
	return sink(llm.StreamEvent{Type: "token", Token: text, Text: text})
}

func (s *anthropicStreamState) applyUsage(input, output int64) {
	if input != 0 {
		s.usage.InputTokens = input
	}
	if output != 0 {
		s.usage.OutputTokens = output
	}
}

func (s *anthropicStreamState) toolBlock(index int) *anthropicStreamToolBlock {
	if s.toolBlocks == nil {
		s.toolBlocks = make(map[int]*anthropicStreamToolBlock)
	}
	block := s.toolBlocks[index]
	if block == nil {
		block = &anthropicStreamToolBlock{}
		s.toolBlocks[index] = block
	}
	return block
}

func (s *anthropicStreamState) result() llm.TurnResult {
	res := llm.TurnResult{
		Text:   s.text.String(),
		Reason: s.reason,
		Usage:  s.usage,
	}
	indices := make([]int, 0, len(s.toolBlocks))
	for index := range s.toolBlocks {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		block := s.toolBlocks[index]
		if block == nil || (block.id == "" && block.name == "" && block.inputJSON.Len() == 0 && len(block.input) == 0) {
			continue
		}
		args := block.input
		if args == nil && block.inputJSON.Len() > 0 {
			var decoded map[string]any
			if err := json.Unmarshal([]byte(block.inputJSON.String()), &decoded); err == nil {
				args = openai.NormalizeMap(decoded)
			}
		}
		res.Calls = append(res.Calls, llm.ToolCall{
			ID:   block.id,
			Tool: block.name,
			Args: args,
		})
	}
	if len(res.Calls) > 0 {
		res.Status = "tool_calls"
	} else if s.reason == "max_tokens" || s.reason == "stop_sequence" {
		res.Status = "stop"
	} else {
		res.Status = "final_answer"
	}
	return res
}

func llmRetryableStatus(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusConflict ||
		status == http.StatusTooManyRequests ||
		status >= 500
}

func waitLLMRetry(ctx context.Context, backoff time.Duration) error {
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

func cloneStringAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = cloneAny(v)
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
