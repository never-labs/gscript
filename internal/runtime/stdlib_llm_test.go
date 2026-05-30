package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gscript/gscript/internal/ast"
	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
)

type testLLMProvider struct {
	requests []LLMTurnRequest
	res      LLMTurnResult
	results  []LLMTurnResult
	err      error
}

func (p *testLLMProvider) Turn(_ context.Context, req LLMTurnRequest) (LLMTurnResult, error) {
	p.requests = append(p.requests, req)
	if p.err != nil {
		return LLMTurnResult{}, p.err
	}
	if len(p.results) > 0 {
		res := p.results[0]
		p.results = p.results[1:]
		return res, nil
	}
	return p.res, nil
}

type testLLMProviderKindError string

func (e testLLMProviderKindError) Error() string {
	return "typed provider failure: " + string(e)
}

func (e testLLMProviderKindError) LLMProviderErrorKind() string {
	return string(e)
}

func parseLLMTestProgram(t *testing.T, src string) *ast.Program {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return prog
}

func TestLLMRunAgentOutputValidationMissingField(t *testing.T) {
	provider := &testLLMProvider{res: LLMTurnResult{Status: "final_answer", Text: `{"name":"Ada"}`}}
	interp := New()
	interp.llmProvider = provider

	if err := interp.Exec(parseLLMTestProgram(t, `
result, err := llm.run_agent({
    model: "mock-json"
    messages: {llm.user("Extract the contact.")}
    output: {
        name: "Ada"
        email: "ada@example.com"
    }
})
err_kind := err.kind
err_message := err.message
`)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got := interp.GetGlobal("err_kind"); !got.IsString() || got.Str() != "validation" {
		t.Fatalf("err_kind = %v, want validation", got)
	}
	if got := interp.GetGlobal("err_message"); !got.IsString() || !strings.Contains(got.Str(), `missing field "email"`) {
		t.Fatalf("err_message = %v, want missing email field", got)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(provider.requests))
	}
	format, ok := provider.requests[0].ResponseFormat.(map[string]any)
	if !ok || format["type"] != "json_object" {
		t.Fatalf("response_format = %#v, want json_object", provider.requests[0].ResponseFormat)
	}
}

func TestLLMTurnProviderErrorKindReachesScript(t *testing.T) {
	provider := &testLLMProvider{err: testLLMProviderKindError(LLMProviderErrorRateLimit)}
	interp := New()
	interp.llmProvider = provider

	if err := interp.Exec(parseLLMTestProgram(t, `
result, err := llm.turn({
    model: "mock"
    messages: {llm.user("hello")}
})
err_kind := err.kind
err_message := err.message
`)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got := interp.GetGlobal("err_kind"); !got.IsString() || got.Str() != LLMProviderErrorRateLimit {
		t.Fatalf("err_kind = %v, want %s", got, LLMProviderErrorRateLimit)
	}
	if got := interp.GetGlobal("err_message"); !got.IsString() || !strings.Contains(got.Str(), "typed provider failure") {
		t.Fatalf("err_message = %v, want typed provider failure", got)
	}
}

func TestLLMReactProviderErrorKindReachesScript(t *testing.T) {
	provider := &testLLMProvider{err: context.DeadlineExceeded}
	interp := New()
	interp.llmProvider = provider

	if err := interp.Exec(parseLLMTestProgram(t, `
result, err := llm.react({
    model: "mock"
    messages: {llm.user("hello")}
    max_steps: 1
})
err_kind := err.kind
`)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got := interp.GetGlobal("err_kind"); !got.IsString() || got.Str() != LLMProviderErrorNetwork {
		t.Fatalf("err_kind = %v, want %s", got, LLMProviderErrorNetwork)
	}
}

func TestClassifyLLMProviderErrorRuntime(t *testing.T) {
	if got := ClassifyLLMProviderError(testLLMProviderKindError(LLMProviderErrorAuth)); got != LLMProviderErrorAuth {
		t.Fatalf("typed provider classification = %q, want %q", got, LLMProviderErrorAuth)
	}
	if got := ClassifyLLMProviderError(context.Canceled); got != LLMProviderErrorNetwork {
		t.Fatalf("context classification = %q, want %q", got, LLMProviderErrorNetwork)
	}
	if got := ClassifyLLMProviderError(errors.New("plain")); got != LLMProviderErrorProvider {
		t.Fatalf("plain classification = %q, want %q", got, LLMProviderErrorProvider)
	}
}

func TestLLMRunAgentOutputValidationTypeMismatch(t *testing.T) {
	provider := &testLLMProvider{res: LLMTurnResult{Status: "final_answer", Text: `{"name":"Ada","score":"high","ok":true,"meta":{}}`}}
	interp := New()
	interp.llmProvider = provider

	if err := interp.Exec(parseLLMTestProgram(t, `
result, err := llm.run_agent({
    model: "mock-json"
    messages: {llm.user("Classify the contact.")}
    output: {
        name: "Ada"
        score: 1
        ok: true
        meta: {source: "email"}
    }
})
err_kind := err.kind
err_message := err.message
`)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got := interp.GetGlobal("err_kind"); !got.IsString() || got.Str() != "validation" {
		t.Fatalf("err_kind = %v, want validation", got)
	}
	if got := interp.GetGlobal("err_message"); !got.IsString() || !strings.Contains(got.Str(), `field "score" has type string, want number`) {
		t.Fatalf("err_message = %v, want score type mismatch", got)
	}
}

func TestLLMRunAgentOutputValidationNestedMissingField(t *testing.T) {
	provider := &testLLMProvider{res: LLMTurnResult{Status: "final_answer", Text: `{"name":"Ada","profile":{"city":"London"}}`}}
	interp := New()
	interp.llmProvider = provider

	if err := interp.Exec(parseLLMTestProgram(t, `
result, err := llm.run_agent({
    model: "mock-json"
    messages: {llm.user("Extract the contact.")}
    output: {
        name: "Ada"
        profile: {
            email: "ada@example.com"
            city: "London"
        }
    }
})
err_kind := err.kind
err_message := err.message
`)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got := interp.GetGlobal("err_kind"); !got.IsString() || got.Str() != "validation" {
		t.Fatalf("err_kind = %v, want validation", got)
	}
	if got := interp.GetGlobal("err_message"); !got.IsString() || !strings.Contains(got.Str(), `missing field "profile.email"`) {
		t.Fatalf("err_message = %v, want missing nested email field", got)
	}
}

func TestLLMRunAgentOutputValidationArrayElementShape(t *testing.T) {
	provider := &testLLMProvider{res: LLMTurnResult{Status: "final_answer", Text: `{"items":[{"name":"Ada","score":1},{"name":"Grace","score":"high"}]}`}}
	interp := New()
	interp.llmProvider = provider

	if err := interp.Exec(parseLLMTestProgram(t, `
result, err := llm.run_agent({
    model: "mock-json"
    messages: {llm.user("Rank the contacts.")}
    output: {
        items: {
            {
                name: "Ada"
                score: 1
            }
        }
    }
})
err_kind := err.kind
err_message := err.message
`)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got := interp.GetGlobal("err_kind"); !got.IsString() || got.Str() != "validation" {
		t.Fatalf("err_kind = %v, want validation", got)
	}
	if got := interp.GetGlobal("err_message"); !got.IsString() || !strings.Contains(got.Str(), `field "items[2].score" has type string, want number`) {
		t.Fatalf("err_message = %v, want array element score type mismatch", got)
	}
}

func TestLLMRunAgentOutputRepairRetrySucceeds(t *testing.T) {
	provider := &testLLMProvider{results: []LLMTurnResult{
		{Status: "final_answer", Text: `{"name":"Ada"}`},
		{Status: "final_answer", Text: `{"name":"Ada","email":"ada@example.com"}`},
	}}
	interp := New()
	interp.llmProvider = provider

	if err := interp.Exec(parseLLMTestProgram(t, `
result, err := llm.run_agent({
    model: "mock-json"
    messages: {llm.user("Extract the contact.")}
    output: {
        name: "Ada"
        email: "ada@example.com"
    }
    output_retries: 1
})
email := result.value.email
err_is_nil := err == nil
`)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got := interp.GetGlobal("err_is_nil"); !got.Truthy() {
		t.Fatalf("err_is_nil = %v, want true", got)
	}
	if got := interp.GetGlobal("email"); !got.IsString() || got.Str() != "ada@example.com" {
		t.Fatalf("email = %v, want ada@example.com", got)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(provider.requests))
	}
	if len(provider.requests[1].Messages) != 2 {
		t.Fatalf("repair message count = %d, want 2", len(provider.requests[1].Messages))
	}
	repairPrompt := provider.requests[1].Messages[1].Text
	if !strings.Contains(repairPrompt, `missing field "email"`) || !strings.Contains(repairPrompt, `{"name":"Ada"}`) {
		t.Fatalf("repair prompt = %q, want validation error and previous response", repairPrompt)
	}
}

func TestLLMRunAgentOutputRepairConsumesTurnBudget(t *testing.T) {
	provider := &testLLMProvider{results: []LLMTurnResult{
		{Status: "final_answer", Text: `{"name":"Ada"}`},
		{Status: "final_answer", Text: `{"name":"Ada","email":"ada@example.com"}`},
	}}
	interp := New()
	interp.llmProvider = provider

	if err := interp.Exec(parseLLMTestProgram(t, `
result, err := llm.run_agent({
    model: "mock-json"
    messages: {llm.user("Extract the contact.")}
    output: {
        name: "Ada"
        email: "ada@example.com"
    }
    output_retries: 1
    budget: {turns: 1}
})
err_kind := err.kind
err_dimension := err.dimension
`)); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got := interp.GetGlobal("err_kind"); !got.IsString() || got.Str() != "budget" {
		t.Fatalf("err_kind = %v, want budget", got)
	}
	if got := interp.GetGlobal("err_dimension"); !got.IsString() || got.Str() != "turns" {
		t.Fatalf("err_dimension = %v, want turns", got)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("requests = %d, want 1 because repair is blocked by turn budget", len(provider.requests))
	}
}
