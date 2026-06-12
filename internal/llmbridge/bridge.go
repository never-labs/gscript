package llmbridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/llm"
	"github.com/never-labs/leia/llm/anthropic"
	"github.com/never-labs/leia/llm/openai"
)

type providerAdapter struct {
	provider llm.Provider
}

func ProviderAdapter(provider llm.Provider) runtime.LLMProvider {
	if provider == nil {
		return nil
	}
	return providerAdapter{provider: provider}
}

func (a providerAdapter) Turn(ctx context.Context, req runtime.LLMTurnRequest) (runtime.LLMTurnResult, error) {
	res, err := a.provider.Turn(ctx, PublicTurnRequest(req))
	return RuntimeTurnResult(res), err
}

func (a providerAdapter) StreamTurn(ctx context.Context, req runtime.LLMTurnRequest, sink runtime.LLMStreamSink) (runtime.LLMTurnResult, error) {
	streaming, ok := a.provider.(llm.StreamingProvider)
	if !ok {
		return a.Turn(ctx, req)
	}
	res, err := streaming.StreamTurn(ctx, PublicTurnRequest(req), func(event llm.StreamEvent) error {
		if sink == nil {
			return nil
		}
		return sink(RuntimeStreamEvent(event))
	})
	return RuntimeTurnResult(res), err
}

func (a providerAdapter) LastLLMReplayMatch() (runtime.LLMReplayMatch, bool) {
	replay, ok := a.provider.(llm.ReplayMatchProvider)
	if !ok {
		return runtime.LLMReplayMatch{}, false
	}
	match, ok := replay.LastReplayMatch()
	if !ok {
		return runtime.LLMReplayMatch{}, false
	}
	return RuntimeReplayMatch(match), true
}

func TraceAdapter(sink llm.TraceSink) runtime.LLMTraceSink {
	if sink == nil {
		return nil
	}
	return func(event runtime.LLMTraceEvent) {
		sink(PublicTraceEvent(event))
	}
}

func PublicProviderConfig(cfg runtime.LLMProviderConfig) llm.ProviderConfig {
	return llm.ProviderConfig{
		Name:          cfg.Name,
		Protocol:      cfg.Protocol,
		BaseURL:       cfg.BaseURL,
		APIKey:        cfg.APIKey,
		ProviderModel: cfg.ProviderModel,
		Provider:      cfg.Provider,
	}
}

func PublicTurnRequest(req runtime.LLMTurnRequest) llm.TurnRequest {
	out := llm.TurnRequest{
		Model:          req.Model,
		ForceTool:      req.ForceTool,
		MaxTokens:      req.MaxTokens,
		Temperature:    cloneFloat64Ptr(req.Temperature),
		TopP:           cloneFloat64Ptr(req.TopP),
		ResponseFormat: cloneLLMAny(req.ResponseFormat),
		Stream:         req.Stream,
		Stop:           append([]string(nil), req.Stop...),
		Metadata:       cloneStringMap(req.Metadata),
	}
	if len(req.Messages) > 0 {
		out.Messages = make([]llm.Message, len(req.Messages))
		for i := range req.Messages {
			out.Messages[i] = PublicMessage(req.Messages[i])
		}
	}
	if len(req.Tools) > 0 {
		out.Tools = make([]llm.Tool, len(req.Tools))
		for i := range req.Tools {
			out.Tools[i] = PublicTool(req.Tools[i])
		}
	}
	return out
}

func RuntimeTurnRequest(req llm.TurnRequest) runtime.LLMTurnRequest {
	out := runtime.LLMTurnRequest{
		Model:          req.Model,
		ForceTool:      req.ForceTool,
		MaxTokens:      req.MaxTokens,
		Temperature:    cloneFloat64Ptr(req.Temperature),
		TopP:           cloneFloat64Ptr(req.TopP),
		ResponseFormat: cloneLLMAny(req.ResponseFormat),
		Stream:         req.Stream,
		Stop:           append([]string(nil), req.Stop...),
		Metadata:       cloneStringMap(req.Metadata),
	}
	if len(req.Messages) > 0 {
		out.Messages = make([]runtime.LLMMessage, len(req.Messages))
		for i := range req.Messages {
			out.Messages[i] = RuntimeMessage(req.Messages[i])
		}
	}
	if len(req.Tools) > 0 {
		out.Tools = make([]runtime.LLMTool, len(req.Tools))
		for i := range req.Tools {
			out.Tools[i] = RuntimeTool(req.Tools[i])
		}
	}
	return out
}

func PublicMessage(msg runtime.LLMMessage) llm.Message {
	out := llm.Message{
		Role:      msg.Role,
		Text:      msg.Text,
		ToolUseID: msg.ToolUseID,
		Value:     cloneLLMAny(msg.Value),
		Error:     msg.Error,
	}
	if msg.ToolCall != nil {
		call := PublicToolCall(*msg.ToolCall)
		out.ToolCall = &call
	}
	return out
}

func RuntimeMessage(msg llm.Message) runtime.LLMMessage {
	out := runtime.LLMMessage{
		Role:      msg.Role,
		Text:      msg.Text,
		ToolUseID: msg.ToolUseID,
		Value:     cloneLLMAny(msg.Value),
		Error:     msg.Error,
	}
	if msg.ToolCall != nil {
		call := RuntimeToolCall(*msg.ToolCall)
		out.ToolCall = &call
	}
	return out
}

func PublicTool(tool runtime.LLMTool) llm.Tool {
	return llm.Tool{
		Name:        tool.Name,
		Description: tool.Description,
		Params:      append([]string(nil), tool.Params...),
		Requires:    append([]string(nil), tool.Requires...),
		Schema:      cloneLLMAny(tool.Schema),
	}
}

func RuntimeTool(tool llm.Tool) runtime.LLMTool {
	return runtime.LLMTool{
		Name:        tool.Name,
		Description: tool.Description,
		Params:      append([]string(nil), tool.Params...),
		Requires:    append([]string(nil), tool.Requires...),
		Schema:      cloneLLMAny(tool.Schema),
	}
}

func PublicToolCall(call runtime.LLMToolCall) llm.ToolCall {
	return llm.ToolCall{
		ID:   call.ID,
		Tool: call.Tool,
		Args: cloneLLMArgs(call.Args),
	}
}

func RuntimeToolCall(call llm.ToolCall) runtime.LLMToolCall {
	return runtime.LLMToolCall{
		ID:   call.ID,
		Tool: call.Tool,
		Args: cloneLLMArgs(call.Args),
	}
}

func cloneLLMArgs(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		out[k] = cloneLLMAny(v)
	}
	return out
}

func PublicTurnResult(res runtime.LLMTurnResult) llm.TurnResult {
	out := llm.TurnResult{
		Status: res.Status,
		Text:   res.Text,
		Reason: res.Reason,
		Usage:  PublicTurnUsage(res.Usage),
	}
	if len(res.Calls) > 0 {
		out.Calls = make([]llm.ToolCall, len(res.Calls))
		for i := range res.Calls {
			out.Calls[i] = PublicToolCall(res.Calls[i])
		}
	}
	return out
}

func RuntimeTurnResult(res llm.TurnResult) runtime.LLMTurnResult {
	out := runtime.LLMTurnResult{
		Status: res.Status,
		Text:   res.Text,
		Reason: res.Reason,
		Usage:  RuntimeTurnUsage(res.Usage),
	}
	if len(res.Calls) > 0 {
		out.Calls = make([]runtime.LLMToolCall, len(res.Calls))
		for i := range res.Calls {
			out.Calls[i] = RuntimeToolCall(res.Calls[i])
		}
	}
	return out
}

func PublicTurnUsage(usage runtime.LLMTurnUsage) llm.TurnUsage {
	return llm.TurnUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Cost:         usage.Cost,
		LatencyMS:    usage.LatencyMS,
	}
}

func PublicStreamEvent(event runtime.LLMStreamEvent) llm.StreamEvent {
	return llm.StreamEvent{
		Type:   event.Type,
		Token:  event.Token,
		Text:   event.Text,
		Status: event.Status,
		Reason: event.Reason,
		Usage:  PublicTurnUsage(event.Usage),
	}
}

func RuntimeStreamEvent(event llm.StreamEvent) runtime.LLMStreamEvent {
	return runtime.LLMStreamEvent{
		Type:   event.Type,
		Token:  event.Token,
		Text:   event.Text,
		Status: event.Status,
		Reason: event.Reason,
		Usage:  RuntimeTurnUsage(event.Usage),
	}
}

func RuntimeTurnUsage(usage llm.TurnUsage) runtime.LLMTurnUsage {
	return runtime.LLMTurnUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Cost:         usage.Cost,
		LatencyMS:    usage.LatencyMS,
	}
}

func RuntimeReplayMatch(match llm.ReplayMatch) runtime.LLMReplayMatch {
	return runtime.LLMReplayMatch{
		Turn:            match.Turn,
		ReplayKey:       match.ReplayKey,
		RequestHash:     match.RequestHash,
		ResponseHash:    match.ResponseHash,
		ReplayMode:      match.ReplayMode,
		ReplaySessionID: match.ReplaySessionID,
		ProviderFree:    match.ProviderFree,
	}
}

func PublicTraceEvent(event runtime.LLMTraceEvent) llm.TraceEvent {
	return llm.TraceEvent{
		TraceID:         event.TraceID,
		EventID:         event.EventID,
		ParentEventID:   event.ParentEventID,
		TurnID:          event.TurnID,
		ReplaySessionID: event.ReplaySessionID,
		Sequence:        event.Sequence,
		TimestampMS:     event.TimestampMS,
		Type:            event.Type,
		Model:           event.Model,
		Status:          event.Status,
		Tool:            event.Tool,
		CallID:          event.CallID,
		Token:           event.Token,
		ErrorKind:       event.ErrorKind,
		Message:         event.Message,
		Step:            event.Step,
		Attempt:         event.Attempt,
		MessageCount:    event.MessageCount,
		ToolCount:       event.ToolCount,
		Store:           event.Store,
		Usage:           PublicTurnUsage(event.Usage),
		ReplayKey:       event.ReplayKey,
		RequestHash:     event.RequestHash,
		ResponseHash:    event.ResponseHash,
		ReplayMode:      event.ReplayMode,
		ProviderFree:    event.ProviderFree,
	}
}

type recordingProvider struct {
	provider llm.Provider
	sink     llm.RecordSink
}

func ConfiguredProvider(provider llm.Provider, sink llm.RecordSink) llm.Provider {
	if provider == nil || sink == nil {
		return provider
	}
	return recordingProvider{provider: provider, sink: sink}
}

func ConfiguredProviderFactory(hostProvider llm.Provider, factory llm.ProviderFactory, sink llm.RecordSink) runtime.LLMProviderFactory {
	if factory == nil && hostProvider == nil {
		factory = DefaultProviderFactory
	}
	if factory == nil {
		return nil
	}
	return func(cfg runtime.LLMProviderConfig) (runtime.LLMProvider, error) {
		p, err := factory(PublicProviderConfig(cfg))
		if err != nil || p == nil || sink == nil {
			if p == nil {
				return nil, err
			}
			return providerAdapter{provider: p}, err
		}
		return providerAdapter{provider: recordingProvider{provider: p, sink: sink}}, nil
	}
}

func DefaultProviderFactory(cfg llm.ProviderConfig) (llm.Provider, error) {
	protocol := strings.ToLower(strings.ReplaceAll(cfg.Protocol, "_", "-"))
	switch protocol {
	case "openai", "openai-compatible", "openai-compat", "chat-completions":
		return openai.Provider{
			Endpoint: openai.ChatCompletionsEndpoint(cfg.BaseURL),
			APIKey:   cfg.APIKey,
			Model:    cfg.ProviderModel,
		}, nil
	case "anthropic", "anthropic-compatible", "anthropic-compat", "messages":
		return anthropic.Provider{
			Endpoint: cfg.BaseURL,
			APIKey:   cfg.APIKey,
			Model:    cfg.ProviderModel,
		}, nil
	default:
		if cfg.Protocol == "" {
			return nil, fmt.Errorf("llm provider protocol not configured for model %q", cfg.Name)
		}
		return nil, fmt.Errorf("unsupported llm provider protocol %q for model %q", cfg.Protocol, cfg.Name)
	}
}

func (p recordingProvider) Turn(ctx context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	res, err := p.provider.Turn(ctx, req)
	if p.sink != nil {
		record := llm.Record{Request: cloneLLMRequest(req), Result: cloneLLMResult(res)}
		if err != nil {
			record.Error = err.Error()
		}
		p.sink(record)
	}
	return res, err
}

func (p recordingProvider) StreamTurn(ctx context.Context, req llm.TurnRequest, sink llm.StreamSink) (llm.TurnResult, error) {
	streaming, ok := p.provider.(llm.StreamingProvider)
	if !ok {
		return p.Turn(ctx, req)
	}
	var events []llm.StreamEvent
	res, err := streaming.StreamTurn(ctx, req, func(event llm.StreamEvent) error {
		events = append(events, event)
		if sink == nil {
			return nil
		}
		return sink(event)
	})
	if p.sink != nil {
		record := llm.Record{
			Request:      cloneLLMRequest(req),
			Result:       cloneLLMResult(res),
			StreamEvents: append([]llm.StreamEvent(nil), events...),
		}
		if err != nil {
			record.Error = err.Error()
		}
		p.sink(record)
	}
	return res, err
}

func (p recordingProvider) LastReplayMatch() (llm.ReplayMatch, bool) {
	replay, ok := p.provider.(llm.ReplayMatchProvider)
	if !ok {
		return llm.ReplayMatch{}, false
	}
	return replay.LastReplayMatch()
}

func cloneLLMRequest(req llm.TurnRequest) llm.TurnRequest {
	out := req
	out.Messages = append([]llm.Message(nil), req.Messages...)
	for i := range out.Messages {
		if out.Messages[i].ToolCall != nil {
			call := cloneToolCall(*out.Messages[i].ToolCall)
			out.Messages[i].ToolCall = &call
		}
		out.Messages[i].Value = cloneLLMAny(out.Messages[i].Value)
	}
	out.Tools = append([]llm.Tool(nil), req.Tools...)
	for i := range out.Tools {
		out.Tools[i].Params = append([]string(nil), req.Tools[i].Params...)
		out.Tools[i].Requires = append([]string(nil), req.Tools[i].Requires...)
		out.Tools[i].Schema = cloneLLMAny(req.Tools[i].Schema)
	}
	if req.Temperature != nil {
		v := *req.Temperature
		out.Temperature = &v
	}
	if req.TopP != nil {
		v := *req.TopP
		out.TopP = &v
	}
	out.ResponseFormat = cloneLLMAny(req.ResponseFormat)
	out.Stop = append([]string(nil), req.Stop...)
	if req.Metadata != nil {
		out.Metadata = make(map[string]string, len(req.Metadata))
		for k, v := range req.Metadata {
			out.Metadata[k] = v
		}
	}
	return out
}

func cloneLLMResult(res llm.TurnResult) llm.TurnResult {
	out := res
	out.Calls = make([]llm.ToolCall, len(res.Calls))
	for i := range res.Calls {
		out.Calls[i] = cloneToolCall(res.Calls[i])
	}
	return out
}

func cloneToolCall(call llm.ToolCall) llm.ToolCall {
	out := call
	if call.Args != nil {
		out.Args = make(map[string]any, len(call.Args))
		for k, v := range call.Args {
			out.Args[k] = cloneLLMAny(v)
		}
	}
	return out
}

func cloneLLMAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = cloneLLMAny(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = cloneLLMAny(v)
		}
		return out
	default:
		return v
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
