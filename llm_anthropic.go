package gscript

import llmanthropic "github.com/never-labs/gscript/llm/anthropic"

func WithAnthropicCompatibleLLM(endpoint, apiKey, model string) Option {
	return WithLLMProvider(llmanthropic.Provider{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}
