package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMModelAliasRouteTraceAndReplayDecision(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{leia.WithLibs(leia.LibLLM)}, tc.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`
config := llm.config
models, models_err := config.aliases({
    default: "analyst",
    analyst: "cheap_analyst",
    cheap_analyst: {
        provider: "local-replay",
        provider_model: "mock-finrobot-analyst",
    },
    strict_reviewer: {
        provider: "audit-provider",
        model: "mock-finrobot-reviewer",
    },
})

first, first_err := config.route(models, {model: "analyst"})
replayed, replay_err := config.route(models, {
    model: "strict_reviewer",
    replay: first.replay,
})

first_alias := first.alias
first_provider := first.provider
first_model := first.model
first_provider_model := first.provider_model
first_trace_type := first.trace.type
first_trace_reason := first.trace.reason
first_trace_replayed := first.trace.replayed
first_trace_path_1 := first.trace.path[1]
first_trace_path_2 := first.trace.path[2]
first_trace_path_3 := first.trace.path[3]
first_replay_decision := first.replay.decision

replayed_alias := replayed.alias
replayed_provider := replayed.provider
replayed_model := replayed.model
replayed_trace_reason := replayed.trace.reason
replayed_trace_replayed := replayed.trace.replayed
replayed_decision := replayed.replay.decision
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"first_alias":             "analyst",
				"first_provider":          "local-replay",
				"first_model":             "mock-finrobot-analyst",
				"first_provider_model":    "mock-finrobot-analyst",
				"first_trace_type":        "model_route",
				"first_trace_reason":      "alias",
				"first_trace_replayed":    false,
				"first_trace_path_1":      "analyst",
				"first_trace_path_2":      "cheap_analyst",
				"first_trace_path_3":      "mock-finrobot-analyst",
				"replayed_alias":          "analyst",
				"replayed_provider":       "local-replay",
				"replayed_model":          "mock-finrobot-analyst",
				"replayed_trace_reason":   "replay",
				"replayed_trace_replayed": true,
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			firstDecision, err := vm.Get("first_replay_decision")
			if err != nil {
				t.Fatalf("Get first_replay_decision: %v", err)
			}
			replayedDecision, err := vm.Get("replayed_decision")
			if err != nil {
				t.Fatalf("Get replayed_decision: %v", err)
			}
			if firstDecision != replayedDecision {
				t.Fatalf("replayed decision = %#v, want first decision %#v", replayedDecision, firstDecision)
			}
		})
	}
}

func TestLLMModelAliasCycleDiagnostic(t *testing.T) {
	vm := leia.New(leia.WithLibs(leia.LibLLM))
	if err := vm.Exec(`
models, err := llm.config.aliases({
    a: "b",
    b: "a",
})
kind := err.kind
message := err.message
`); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	got, err := vm.Get("kind")
	if err != nil {
		t.Fatalf("Get kind: %v", err)
	}
	if got != "config" {
		t.Fatalf("kind = %#v, want config", got)
	}
	got, err = vm.Get("message")
	if err != nil {
		t.Fatalf("Get message: %v", err)
	}
	if !strings.Contains(got.(string), "llm model alias cycle") {
		t.Fatalf("message = %q, want model alias cycle diagnostic", got)
	}
}
