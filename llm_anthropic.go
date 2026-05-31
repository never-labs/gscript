package gscript

import llmanthropic "github.com/never-labs/gscript/llm/anthropic"

// AnthropicCompatibleLLMProvider adapts llm.turn to Anthropic's messages API
// wire format, including compatible gateways that expose the same protocol.
type AnthropicCompatibleLLMProvider = llmanthropic.Provider

// AnthropicCompatibleLLMError reports a non-2xx response from an
// Anthropic-compatible messages endpoint.
type AnthropicCompatibleLLMError = llmanthropic.Error

func WithAnthropicCompatibleLLM(endpoint, apiKey, model string) Option {
	return WithLLMProvider(AnthropicCompatibleLLMProvider{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}
