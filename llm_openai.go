package gscript

import llmopenai "github.com/never-labs/gscript/llm/openai"

// OpenAICompatibleLLMProvider adapts llm.turn to the OpenAI Chat Completions
// wire format used by OpenAI and many local or third-party model gateways.
type OpenAICompatibleLLMProvider = llmopenai.Provider

// OpenAICompatibleLLMError reports a non-2xx response from an
// OpenAI-compatible chat-completions endpoint.
type OpenAICompatibleLLMError = llmopenai.Error

func WithOpenAICompatibleLLM(endpoint, apiKey, model string) Option {
	return WithLLMProvider(OpenAICompatibleLLMProvider{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}
