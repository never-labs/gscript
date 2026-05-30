package gscript_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	gs "github.com/gscript/gscript/gscript"
)

const defaultGLMAnthropicCompatibleBaseURL = "https://open.bigmodel.cn/api/anthropic"

type glmSmokeConfig struct {
	Endpoint string
	APIKey   string
	Model    string
}

func glmAnthropicCompatibleSmokeConfig(t *testing.T) glmSmokeConfig {
	t.Helper()
	if os.Getenv("GSCRIPT_LLM_INTEGRATION") == "" {
		t.Skip("set GSCRIPT_LLM_INTEGRATION=1 to run real GLM provider smoke")
	}
	endpoint := firstNonEmptyEnv("GSCRIPT_GLM_BASE_URL", "ANTHROPIC_BASE_URL")
	if endpoint == "" {
		endpoint = defaultGLMAnthropicCompatibleBaseURL
	}
	apiKey := firstNonEmptyEnv("GSCRIPT_GLM_API_KEY", "SENTINEL_GLM_API_KEY", "GLM_API_KEY", "ANTHROPIC_AUTH_TOKEN")
	model := firstNonEmptyEnv("GSCRIPT_GLM_MODEL", "GLM_MODEL", "ANTHROPIC_MODEL")
	if model == "" {
		model = "glm-5.1"
	}
	if apiKey == "" {
		t.Skip("set GSCRIPT_GLM_API_KEY, SENTINEL_GLM_API_KEY, or GLM_API_KEY; glm_cc uses SENTINEL_GLM_API_KEY/GLM_API_KEY")
	}
	return glmSmokeConfig{Endpoint: endpoint, APIKey: apiKey, Model: model}
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func assertLLMSmokeText(t *testing.T, got, want string) {
	t.Helper()
	normalized := strings.ToLower(strings.TrimSpace(got))
	normalized = strings.Trim(normalized, `"'`+"`")
	if !strings.Contains(normalized, want) {
		t.Fatalf("llm text = %q, want it to contain %q", got, want)
	}
}

// TestAnthropicCompatibleLLMIntegration is a gated real-provider smoke test.
// It intentionally uses generic Anthropic-compatible configuration names so
// local wrappers or vendor-specific credentials do not become GScript API.
func TestAnthropicCompatibleLLMIntegration(t *testing.T) {
	if os.Getenv("GSCRIPT_LLM_INTEGRATION") == "" {
		t.Skip("set GSCRIPT_LLM_INTEGRATION=1 to run real Anthropic-compatible provider smoke")
	}
	endpoint := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_BASE_URL")
	apiKey := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_API_KEY")
	model := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_MODEL")
	if endpoint == "" || apiKey == "" || model == "" {
		t.Skip("set GSCRIPT_ANTHROPIC_COMPAT_BASE_URL, GSCRIPT_ANTHROPIC_COMPAT_API_KEY, and GSCRIPT_ANTHROPIC_COMPAT_MODEL")
	}
	provider := gs.AnthropicCompatibleLLMProvider{
		Endpoint:     endpoint,
		APIKey:       apiKey,
		Model:        model,
		Timeout:      45 * time.Second,
		MaxAttempts:  2,
		RetryBackoff: 500 * time.Millisecond,
	}
	fmt.Printf("endpoint=%s\n", endpoint)
	fmt.Printf("model=%s\n", model)
	for _, tc := range []struct {
		name      string
		system    string
		user      string
		maxTokens int64
	}{
		{
			name:      "exact_text",
			system:    "You are a concise test assistant. Return plain text only.",
			user:      "Reply with exactly: gscript llm native ok",
			maxTokens: 32,
		},
		{
			name:      "model_identity",
			system:    "You are a concise test assistant. Return plain text only.",
			user:      "What model are you? Answer in one short sentence.",
			maxTokens: 64,
		},
		{
			name:      "json_answer",
			system:    "Return only valid compact JSON. Do not wrap it in Markdown.",
			user:      `Return {"language":"gscript","native_llm":true,"provider":"anthropic-compatible"}.`,
			maxTokens: 96,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			temperature := 0.0
			fmt.Printf("case=%s\n", tc.name)
			fmt.Printf("system=%q\n", tc.system)
			fmt.Printf("user=%q\n", tc.user)
			res, err := provider.Turn(context.Background(), gs.LLMTurnRequest{
				Messages: []gs.LLMMessage{
					{Role: "system", Text: tc.system},
					{Role: "user", Text: tc.user},
				},
				MaxTokens:   tc.maxTokens,
				Temperature: &temperature,
			})
			if err != nil {
				t.Fatalf("Turn failed: %v", err)
			}
			fmt.Printf("status=%s reason=%s input_tokens=%d output_tokens=%d\n",
				res.Status, res.Reason, res.Usage.InputTokens, res.Usage.OutputTokens)
			fmt.Printf("text=%q\n", res.Text)
			if res.Text == "" {
				t.Fatalf("empty response: %#v", res)
			}
		})
	}
}

// TestAINativeSyntaxAnthropicCompatibleLLMIntegration verifies that AI-native
// models/turn syntax can construct the same gated real-provider adapter. It is
// intentionally skipped unless all real-provider environment variables are set.
func TestAINativeSyntaxAnthropicCompatibleLLMIntegration(t *testing.T) {
	if os.Getenv("GSCRIPT_LLM_INTEGRATION") == "" {
		t.Skip("set GSCRIPT_LLM_INTEGRATION=1 to run AI-native real provider smoke")
	}
	endpoint := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_BASE_URL")
	apiKey := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_API_KEY")
	model := os.Getenv("GSCRIPT_ANTHROPIC_COMPAT_MODEL")
	if endpoint == "" || apiKey == "" || model == "" {
		t.Skip("set GSCRIPT_ANTHROPIC_COMPAT_BASE_URL, GSCRIPT_ANTHROPIC_COMPAT_API_KEY, and GSCRIPT_ANTHROPIC_COMPAT_MODEL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vm := gs.New(gs.WithLibs(gs.LibString | gs.LibOS | gs.LibLLM))
	if err := vm.ExecContext(ctx, `
models {
    default: "smoke"
    smoke: {
        protocol: "anthropic_compatible"
        base_url: os.getenv("GSCRIPT_ANTHROPIC_COMPAT_BASE_URL")
        api_key: os.getenv("GSCRIPT_ANTHROPIC_COMPAT_API_KEY")
        provider_model: os.getenv("GSCRIPT_ANTHROPIC_COMPAT_MODEL")
    }
}

result, err := turn {
    messages: messages {
        system: "You are a concise test assistant. Return plain text only."
        user: "Reply with exactly: gscript ai native provider ok"
    }
    max_tokens: 32
    temperature: 0
}
smoke_text := result.text
`); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	got, err := vm.Get("smoke_text")
	if err != nil {
		t.Fatalf("Get smoke_text: %v", err)
	}
	text, ok := got.(string)
	if !ok || text == "" {
		t.Fatalf("smoke_text = %#v, want non-empty string", got)
	}
	fmt.Printf("endpoint=%s\n", endpoint)
	fmt.Printf("model=%s\n", model)
	fmt.Printf("text=%q\n", text)
}

// TestGLMAnthropicCompatibleLLMIntegration is a gated smoke for the GLM setup
// used by the local glm_cc wrapper, without invoking the wrapper itself.
func TestGLMAnthropicCompatibleLLMIntegration(t *testing.T) {
	cfg := glmAnthropicCompatibleSmokeConfig(t)
	provider := gs.AnthropicCompatibleLLMProvider{
		Endpoint:     cfg.Endpoint,
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		Timeout:      45 * time.Second,
		MaxAttempts:  2,
		RetryBackoff: 500 * time.Millisecond,
	}

	temperature := 0.0
	prompt := "Reply with exactly: gscript glm smoke ok"
	fmt.Printf("endpoint=%s\n", cfg.Endpoint)
	fmt.Printf("model=%s\n", cfg.Model)
	fmt.Printf("user=%q\n", prompt)
	res, err := provider.Turn(context.Background(), gs.LLMTurnRequest{
		Messages: []gs.LLMMessage{
			{Role: "system", Text: "You are a concise smoke-test assistant. Return plain text only."},
			{Role: "user", Text: prompt},
		},
		MaxTokens:   32,
		Temperature: &temperature,
	})
	if err != nil {
		t.Fatalf("Turn failed: %v", err)
	}
	fmt.Printf("status=%s reason=%s input_tokens=%d output_tokens=%d\n",
		res.Status, res.Reason, res.Usage.InputTokens, res.Usage.OutputTokens)
	fmt.Printf("text=%q\n", res.Text)
	assertLLMSmokeText(t, res.Text, "gscript glm smoke ok")
}

// TestAINativeSyntaxGLMIntegration verifies a real multi-turn GLM flow through
// AI-native models/turn/agent syntax. It mirrors glm_cc's endpoint/key/model
// env convention but never shells out to it.
func TestAINativeSyntaxGLMIntegration(t *testing.T) {
	cfg := glmAnthropicCompatibleSmokeConfig(t)
	t.Setenv("GSCRIPT_GLM_BASE_URL", cfg.Endpoint)
	t.Setenv("GSCRIPT_GLM_API_KEY", cfg.APIKey)
	t.Setenv("GSCRIPT_GLM_MODEL", cfg.Model)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	vm := gs.New(gs.WithLibs(gs.LibString | gs.LibOS | gs.LibLLM))
	if err := vm.ExecContext(ctx, `
models {
    default: "glm-smoke"
    "glm-smoke": {
        protocol: "anthropic_compatible"
        base_url: os.getenv("GSCRIPT_GLM_BASE_URL")
        api_key: os.getenv("GSCRIPT_GLM_API_KEY")
        provider_model: os.getenv("GSCRIPT_GLM_MODEL")
    }
}

glm_memory_error := nil
glm_stored_text := ""
glm_recalled_text := ""
glm_history_len := 0
glm_project := ""
glm_owner := ""
glm_remembered := false
glm_source := ""
history := messages {
    system: "You are a deterministic memory smoke-test assistant. Follow exact reply instructions. Keep answers short."
    user: "Store this memory: project codename is ORCHID and owner is ADA. Reply exactly: MEMORY_STORED"
}

stored, stored_err := turn {
    messages: history
    max_tokens: 32
    temperature: 0
}
if stored_err != nil {
    glm_memory_error = stored_err.message
} else {
    history[#history + 1] = msg.assistant(stored.text)
    history[#history + 1] = msg.user("Using only the stored memory, reply exactly: project=ORCHID;owner=ADA")

    recalled, recalled_err := turn {
        messages: history
        max_tokens: 48
        temperature: 0
    }
    if recalled_err != nil {
        glm_memory_error = recalled_err.message
    } else {
        history[#history + 1] = msg.assistant(recalled.text)

        extractor := agent(summary) {
            model: "glm-smoke"
            system: "Return only compact JSON with exactly these keys: project, owner, remembered, meta. meta must be an object with source. Do not include Markdown."
            user: "Convert this memory recall into JSON. Use project=\"ORCHID\", owner=\"ADA\", remembered=true, meta.source=\"history\" when the recall says project=ORCHID;owner=ADA. Recall: " .. summary
            output: {
                project: "ORCHID"
                owner: "ADA"
                remembered: true
                meta: {
                    source: "history"
                }
            }
            max_tokens: 96
            temperature: 0
        }

        extracted, extract_err := extractor(recalled.text)
        if extract_err != nil {
            glm_memory_error = extract_err.message
        } else {
            glm_stored_text = stored.text
            glm_recalled_text = recalled.text
            glm_history_len = #history
            glm_project = extracted.value.project
            glm_owner = extracted.value.owner
            glm_remembered = extracted.value.remembered
            glm_source = extracted.value.meta.source
        }
    }
}
`); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	if got, err := vm.Get("glm_memory_error"); err == nil && got != nil {
		t.Fatalf("glm_memory_error = %#v", got)
	}
	stored, err := vm.Get("glm_stored_text")
	if err != nil {
		t.Fatalf("Get glm_stored_text: %v", err)
	}
	recalled, err := vm.Get("glm_recalled_text")
	if err != nil {
		t.Fatalf("Get glm_recalled_text: %v", err)
	}
	historyLen, err := vm.Get("glm_history_len")
	if err != nil {
		t.Fatalf("Get glm_history_len: %v", err)
	}
	project, err := vm.Get("glm_project")
	if err != nil {
		t.Fatalf("Get glm_project: %v", err)
	}
	owner, err := vm.Get("glm_owner")
	if err != nil {
		t.Fatalf("Get glm_owner: %v", err)
	}
	remembered, err := vm.Get("glm_remembered")
	if err != nil {
		t.Fatalf("Get glm_remembered: %v", err)
	}
	source, err := vm.Get("glm_source")
	if err != nil {
		t.Fatalf("Get glm_source: %v", err)
	}
	fmt.Printf("endpoint=%s\n", cfg.Endpoint)
	fmt.Printf("model=%s\n", cfg.Model)
	fmt.Printf("stored=%q\n", stored)
	fmt.Printf("recalled=%q\n", recalled)
	fmt.Printf("history_len=%#v project=%#v owner=%#v remembered=%#v source=%#v\n", historyLen, project, owner, remembered, source)
	assertLLMSmokeText(t, fmt.Sprint(stored), "memory_stored")
	assertLLMSmokeText(t, fmt.Sprint(recalled), "project=orchid;owner=ada")
	if historyLen != int64(5) || project != "ORCHID" || owner != "ADA" || remembered != true || source != "history" {
		t.Fatalf("structured memory result mismatch: history=%#v project=%#v owner=%#v remembered=%#v source=%#v", historyLen, project, owner, remembered, source)
	}
}
