package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMAgentTurnScenarioRecordReplay(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `
llm.register_models({
    default: "chat"
    chat: {provider_model: "mock-chat"}
})

func reviewer_config(topic) {
    return {
        model: "chat"
        system: "Review with two passes."
    }
}

reviewer := llm.agent("reviewer", reviewer_config, func(topic) {
    cfg := reviewer_config(topic)
    draft, draft_err := llm.turn({
        model: cfg.model
        messages: [
            llm.system(cfg.system),
            llm.user("Draft " .. topic),
        ]
        max_tokens: 32
    })
    if draft_err != nil {
        return nil, draft_err
    }

    final, final_err := llm.turn({
        model: cfg.model
        messages: [
            llm.user(draft.text .. " / final"),
        ]
    })
    return {draft: draft.text, final: final.text}, final_err
}, {
    params: ["topic"]
    description: "Review with two passes."
})

out, err := reviewer("recording")
draft_text := out.draft
final_text := out.final
`
			provider := &mockLLMProvider{results: []llm.TurnResult{
				{Status: "final_answer", Text: "draft pass", Usage: llm.TurnUsage{InputTokens: 2, OutputTokens: 3}},
				{Status: "final_answer", Text: "final pass", Usage: llm.TurnUsage{InputTokens: 4, OutputTokens: 5}},
			}}
			recorder := llm.NewRecorder()
			recordOpts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(provider),
				leia.WithLLMRecorder(recorder.Record),
			}, tc.opts...)
			recordVM := leia.New(recordOpts...)

			if err := recordVM.Exec(source); err != nil {
				t.Fatalf("record Exec: %v", err)
			}
			records := recorder.Records()
			if len(records) != 2 {
				t.Fatalf("records = %#v, want 2", records)
			}
			if records[0].Request.Model != "mock-chat" || records[0].Request.MaxTokens != 32 {
				t.Fatalf("first recorded request = %#v", records[0].Request)
			}
			if len(records[0].Request.Messages) != 2 ||
				records[0].Request.Messages[0].Role != "system" ||
				records[0].Request.Messages[0].Text != "Review with two passes." ||
				records[0].Request.Messages[1].Text != "Draft recording" {
				t.Fatalf("first recorded messages = %#v", records[0].Request.Messages)
			}
			if records[1].Request.Model != "mock-chat" || len(records[1].Request.Messages) != 1 ||
				records[1].Request.Messages[0].Text != "draft pass / final" {
				t.Fatalf("second recorded request = %#v", records[1].Request)
			}

			replayOpts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMReplay(records),
			}, tc.opts...)
			replayVM := leia.New(replayOpts...)
			if err := replayVM.Exec(source); err != nil {
				t.Fatalf("replay Exec: %v", err)
			}
			for name, want := range map[string]any{
				"draft_text": "draft pass",
				"final_text": "final pass",
			} {
				got, err := replayVM.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}
