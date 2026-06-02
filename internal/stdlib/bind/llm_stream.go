package bind

import "context"

func llmTurnWithOptionalStream(ctx context.Context, provider LLMProvider, req LLMTurnRequest, trace func(LLMTraceEvent), base LLMTraceEvent) (LLMTurnResult, error) {
	if req.Stream {
		if streaming, ok := provider.(LLMStreamingProvider); ok {
			return streaming.StreamTurn(ctx, req, func(event LLMStreamEvent) error {
				if trace == nil {
					return nil
				}
				traceEvent := base
				traceEvent.Type = "turn_stream"
				traceEvent.Model = req.Model
				traceEvent.Token = event.Token
				traceEvent.Message = event.Text
				traceEvent.Status = event.Status
				traceEvent.Usage = event.Usage
				trace(traceEvent)
				return nil
			})
		}
	}
	return provider.Turn(ctx, req)
}
