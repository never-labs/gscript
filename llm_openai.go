package gscript

import llmopenai "github.com/never-labs/gscript/llm/openai"

// WithOpenAICompatibleLLM installs an OpenAI-compatible chat-completions
// provider.
//
// Deprecated: use WithLLMProvider with openai.Provider from
// github.com/never-labs/gscript/llm/openai.
func WithOpenAICompatibleLLM(endpoint, apiKey, model string) Option {
	return WithLLMProvider(llmopenai.Provider{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}
