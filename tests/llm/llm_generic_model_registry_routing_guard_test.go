package leia_test

import (
	"context"
	"errors"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

type genericModelRegistryRoutingGuardProvider struct {
	calls *int
}

func (p genericModelRegistryRoutingGuardProvider) Turn(context.Context, llm.TurnRequest) (llm.TurnResult, error) {
	*p.calls++
	return llm.TurnResult{}, errors.New("generic model registry routing guard must not call a live provider")
}

func TestGenericModelRegistryRoutingGuardPrefersReplayDefaultOverRequestLiveProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var liveProviderCalls int
			var replayFactoryCalls int
			replayVM := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(genericModelRegistryRoutingGuardProvider{calls: &liveProviderCalls}),
				leia.WithLLMProviderFactory(func(llm.ProviderConfig) (llm.Provider, error) {
					replayFactoryCalls++
					return genericModelRegistryRoutingGuardProvider{calls: &liveProviderCalls}, nil
				}),
				leia.WithLLMReplay([]llm.Record{{
					Request: llm.TurnRequest{
						Model:    "mock-provider-free-default",
						Messages: []llm.Message{{Role: "user", Text: "route by replay/default"}},
					},
					Result: llm.TurnResult{
						Status: "final_answer",
						Text:   "offline default replay",
					},
				}}),
			}, tc.opts...)...)
			if err := replayVM.Exec(`
ok, register_err := llm.register_models({
    default: "offline_default",
    offline_default: {
        provider: "fixture-replay",
        provider_model: "mock-provider-free-default",
    },
    live_candidate: {
        provider: "future-live-provider",
        protocol: "openai-compatible",
        provider_model: "future-live-model",
    },
})
turn, turn_err := llm.turn({
    provider: "request-scoped-live-provider",
    messages: {llm.user("route by replay/default")},
})
turn_ok := register_err == nil && turn_err == nil
turn_text := turn.text
turn_status := turn.status
`); err != nil {
				t.Fatalf("replay Exec: %v", err)
			}
			for name, want := range map[string]any{
				"turn_ok":     true,
				"turn_text":   "offline default replay",
				"turn_status": "final_answer",
			} {
				got, err := replayVM.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			if liveProviderCalls != 0 || replayFactoryCalls != 0 {
				t.Fatalf("replay/default route called live provider/factory: provider=%d factory=%d", liveProviderCalls, replayFactoryCalls)
			}
		})
	}
}
