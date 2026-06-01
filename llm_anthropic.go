package leia

import llmanthropic "github.com/never-labs/leia/llm/anthropic"

// WithAnthropicCompatibleLLM installs an Anthropic-compatible messages
// provider.
//
// Deprecated: use WithLLMProvider with anthropic.Provider from
// github.com/never-labs/leia/llm/anthropic.
func WithAnthropicCompatibleLLM(endpoint, apiKey, model string) Option {
	return WithLLMProvider(llmanthropic.Provider{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}
