package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/gscript/gscript/internal/ast"
	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
)

type testLLMProvider struct {
	requests []LLMTurnRequest
	res      LLMTurnResult
}

func (p *testLLMProvider) Turn(_ context.Context, req LLMTurnRequest) (LLMTurnResult, error) {
	p.requests = append(p.requests, req)
	return p.res, nil
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
