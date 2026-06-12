package bind

import (
	"context"
	"fmt"
)

func llmTurnWithOptionalStream(ctx context.Context, provider LLMProvider, req LLMTurnRequest, trace func(LLMTraceEvent), base LLMTraceEvent, call ScriptFunctionCaller, onStream Value) (LLMTurnResult, error) {
	if req.Stream {
		if streaming, ok := provider.(LLMStreamingProvider); ok {
			res, err := streaming.StreamTurn(ctx, req, func(event LLMStreamEvent) error {
				traceEvent := base
				traceEvent.Type = "turn_stream"
				traceEvent.Model = req.Model
				traceEvent.Token = event.Token
				traceEvent.Message = event.Text
				traceEvent.Status = event.Status
				traceEvent.Usage = event.Usage
				if trace != nil {
					trace(traceEvent)
				}
				if onStream.IsFunction() {
					if call == nil {
						return llmStreamCallbackError("llm.turn on_stream callback requires a function caller")
					}
					if _, err := call(onStream, []Value{TableValue(llmStreamEventTable(event))}); err != nil {
						return err
					}
				}
				return nil
			})
			if err == nil {
				llmTraceReplayMatch(provider, req, res, trace, base)
			}
			return res, err
		}
	}
	res, err := provider.Turn(ctx, req)
	if err == nil {
		llmTraceReplayMatch(provider, req, res, trace, base)
	}
	return res, err
}

func llmTraceReplayMatch(provider LLMProvider, req LLMTurnRequest, res LLMTurnResult, trace func(LLMTraceEvent), base LLMTraceEvent) {
	if trace == nil {
		return
	}
	replay, ok := provider.(LLMReplayMatchProvider)
	if !ok {
		return
	}
	match, ok := replay.LastLLMReplayMatch()
	if !ok {
		return
	}
	replayKey := match.ReplayKey
	if replayKey == "" && match.Turn >= 0 {
		replayKey = fmt.Sprintf("turn:%d", match.Turn)
	}
	replayMode := match.ReplayMode
	if replayMode == "" {
		replayMode = llmReplayModeFixture
	}
	requestHash := match.RequestHash
	if requestHash == "" {
		requestHash = llmTurnRequestHash(req)
	}
	responseHash := match.ResponseHash
	if responseHash == "" {
		responseHash = llmTurnResultHash(res)
	}
	event := base
	event.Type = "replay_record_matched"
	event.Model = req.Model
	event.Status = "matched"
	event.TurnID = replayKey
	event.ReplaySessionID = match.ReplaySessionID
	event.ReplayKey = replayKey
	event.RequestHash = requestHash
	event.ResponseHash = responseHash
	event.ReplayMode = replayMode
	event.ProviderFree = match.ProviderFree || replayMode == llmReplayModeFixture
	event.MessageCount = len(req.Messages)
	event.ToolCount = len(req.Tools)
	event.Usage = res.Usage
	trace(event)
}

func llmTurnResultHash(res LLMTurnResult) string {
	type replayCall struct {
		ID   string         `json:"id,omitempty"`
		Tool string         `json:"tool,omitempty"`
		Args map[string]any `json:"args,omitempty"`
	}
	payload := struct {
		Status string       `json:"status,omitempty"`
		Text   string       `json:"text,omitempty"`
		Calls  []replayCall `json:"calls,omitempty"`
		Reason string       `json:"reason,omitempty"`
		Usage  LLMTurnUsage `json:"usage,omitempty"`
	}{
		Status: res.Status,
		Text:   res.Text,
		Reason: res.Reason,
		Usage:  res.Usage,
	}
	for _, call := range res.Calls {
		payload.Calls = append(payload.Calls, replayCall{ID: call.ID, Tool: call.Tool, Args: call.Args})
	}
	return llmStableJSONHash(payload)
}

func llmStreamEventTable(event LLMStreamEvent) *Table {
	t := NewTable()
	t.RawSetString("type", StringValue(event.Type))
	t.RawSetString("token", StringValue(event.Token))
	t.RawSetString("text", StringValue(event.Text))
	t.RawSetString("status", StringValue(event.Status))
	t.RawSetString("reason", StringValue(event.Reason))
	usage := NewTable()
	usage.RawSetString("input_tokens", IntValue(event.Usage.InputTokens))
	usage.RawSetString("output_tokens", IntValue(event.Usage.OutputTokens))
	usage.RawSetString("cost", FloatValue(event.Usage.Cost))
	usage.RawSetString("latency_ms", IntValue(event.Usage.LatencyMS))
	t.RawSetString("usage", TableValue(usage))
	return t
}

func llmStreamCallbackError(message string) error {
	return &llmCallbackError{message: message}
}

type llmCallbackError struct {
	message string
}

func (e *llmCallbackError) Error() string {
	return e.message
}
