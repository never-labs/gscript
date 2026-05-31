package gscript

import llmopenai "github.com/never-labs/gscript/llm/openai"

func WithOpenAICompatibleLLM(endpoint, apiKey, model string) Option {
	return WithLLMProvider(llmopenai.Provider{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
}
