package leia_test

import (
	"context"

	"github.com/never-labs/leia/llm"
)

type mockLLMProvider struct {
	last     llm.TurnRequest
	requests []llm.TurnRequest
	res      llm.TurnResult
	err      error
}

func (p *mockLLMProvider) Turn(_ context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	p.last = req
	p.requests = append(p.requests, req)
	if p.err != nil {
		return llm.TurnResult{}, p.err
	}
	if p.res.Status != "" || p.res.Text != "" || len(p.res.Calls) > 0 {
		return p.res, nil
	}
	return llm.TurnResult{Status: "final_answer", Text: "ok"}, nil
}
