package gscript_test

import (
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestAINativeAgentTurnScenarioRecordReplay(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `
models {
    default: "chat"
    chat: {provider_model: "mock-chat"}
}

agent reviewer(topic) {
    model: "chat"
    system: "Review with two passes."
} flow {
    draft, draft_err := turn {
        messages: messages {
            system: system
            user: "Draft " .. topic
        }
        max_tokens: 32
    }
    if draft_err != nil {
        return nil, draft_err
    }

    final, final_err := turn {
        messages: messages {
            user: draft.text .. " / final"
        }
    }
    return {draft: draft.text, final: final.text}, final_err
}

out, err := reviewer("recording")
draft_text := out.draft
final_text := out.final
`
			provider := &mockLLMProvider{results: []gs.LLMTurnResult{
				{Status: "final_answer", Text: "draft pass", Usage: gs.LLMTurnUsage{InputTokens: 2, OutputTokens: 3}},
				{Status: "final_answer", Text: "final pass", Usage: gs.LLMTurnUsage{InputTokens: 4, OutputTokens: 5}},
			}}
			recorder := gs.NewLLMRecorder()
			recordOpts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMProvider(provider),
				gs.WithLLMRecorder(recorder.Record),
			}, tc.opts...)
			recordVM := gs.New(recordOpts...)

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

			replayOpts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibLLM),
				gs.WithLLMReplay(records),
			}, tc.opts...)
			replayVM := gs.New(replayOpts...)
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
