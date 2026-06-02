package bind

import "context"

func llmTurnWithOptionalStream(ctx context.Context, provider LLMProvider, req LLMTurnRequest, trace func(LLMTraceEvent), base LLMTraceEvent, call ScriptFunctionCaller, onStream Value) (LLMTurnResult, error) {
	if req.Stream {
		if streaming, ok := provider.(LLMStreamingProvider); ok {
			return streaming.StreamTurn(ctx, req, func(event LLMStreamEvent) error {
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
		}
	}
	return provider.Turn(ctx, req)
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
