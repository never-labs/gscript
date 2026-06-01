package leia

import llmopenai "github.com/never-labs/leia/llm/openai"

// WithOpenAICompatibleLLM installs an OpenAI-compatible chat-completions
// provider.
//
// Deprecated: use WithLLMProvider with openai.Provider from
// github.com/never-labs/leia/llm/openai.
func WithOpenAICompatibleLLM(endpoint, apiKey, model string) Option {
	return WithLLMProvider(llmopenai.Provider{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}
