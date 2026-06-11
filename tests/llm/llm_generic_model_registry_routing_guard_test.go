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

func TestGenericModelRegistryRouteGuardPreservesFixtureReplayDescriptor(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var liveProviderCalls int
			var factoryCalls int
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibLLM),
				leia.WithLLMProvider(genericModelRegistryRoutingGuardProvider{calls: &liveProviderCalls}),
				leia.WithLLMProviderFactory(func(llm.ProviderConfig) (llm.Provider, error) {
					factoryCalls++
					return genericModelRegistryRoutingGuardProvider{calls: &liveProviderCalls}, nil
				}),
			}, tc.opts...)...)
			if err := vm.Exec(`
config := llm.config
models, models_err := config.aliases({
    default: "analyst",
    analyst: "fixture_analyst",
    reviewer: "fixture_reviewer",
    fixture_analyst: {
        provider: "fixture-replay",
        provider_model: "generic-fixture-analyst-v1",
        mode: "deterministic_fixture_replay",
    },
    fixture_reviewer: {
        provider: "fixture-replay",
        provider_model: "generic-fixture-reviewer-v1",
        mode: "deterministic_fixture_replay",
    },
    live_candidate: {
        provider: "future-live-provider",
        protocol: "openai-compatible",
        provider_model: "future-live-model",
    },
})

first, first_err := config.route(models, {
    model: "default",
    provider: "request-scoped-live-provider",
})
replayed, replay_err := config.route(models, {
    model: "live_candidate",
    provider: "second-request-live-provider",
    replay: first.replay,
})

route_ok := models_err == nil && first_err == nil && replay_err == nil
first_alias := first.alias
first_provider := first.provider
first_model := first.model
first_provider_model := first.provider_model
first_trace_reason := first.trace.reason
first_trace_replayed := first.trace.replayed
replayed_alias := replayed.alias
replayed_provider := replayed.provider
replayed_model := replayed.model
replayed_provider_model := replayed.provider_model
replayed_trace_reason := replayed.trace.reason
replayed_trace_replayed := replayed.trace.replayed
same_decision := first.replay.decision == replayed.replay.decision
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"route_ok":                true,
				"first_alias":             "default",
				"first_provider":          "fixture-replay",
				"first_model":             "generic-fixture-analyst-v1",
				"first_provider_model":    "generic-fixture-analyst-v1",
				"first_trace_reason":      "alias",
				"first_trace_replayed":    false,
				"replayed_alias":          "default",
				"replayed_provider":       "fixture-replay",
				"replayed_model":          "generic-fixture-analyst-v1",
				"replayed_provider_model": "generic-fixture-analyst-v1",
				"replayed_trace_reason":   "replay",
				"replayed_trace_replayed": true,
				"same_decision":           true,
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			if liveProviderCalls != 0 || factoryCalls != 0 {
				t.Fatalf("config.route called live provider/factory: provider=%d factory=%d", liveProviderCalls, factoryCalls)
			}
		})
	}
}
