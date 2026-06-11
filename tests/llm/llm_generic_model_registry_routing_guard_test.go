package leia_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
			var routeProviderCalls int
			var routeFactoryCalls int
			routeVM := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProvider(genericModelRegistryRoutingGuardProvider{calls: &routeProviderCalls}),
				leia.WithLLMProviderFactory(func(llm.ProviderConfig) (llm.Provider, error) {
					routeFactoryCalls++
					return genericModelRegistryRoutingGuardProvider{calls: &routeProviderCalls}, nil
				}),
			}, tc.opts...)...)

			if err := routeVM.Exec(`
config := llm.config
models, models_err := config.aliases({
    default: "offline_default",
    offline_default: {
        provider: "fixture-replay",
        provider_model: "mock-provider-free-default",
    },
    live_candidate: {
        provider: "future-live-provider",
        protocol: "openai-compatible",
        provider_model: "future-live-model",
        api_key: config.secret("FUTURE_LIVE_PROVIDER_API_KEY", {required: false}),
    },
})
ok, register_err := llm.register_models(models)

default_route, default_err := config.route(models, {provider: "request-scoped-live-provider"})
live_metadata_route, live_metadata_err := config.route(models, {
    model: "live_candidate",
    provider: "request-scoped-live-provider",
})
replayed_route, replayed_err := config.route(models, {
    model: "live_candidate",
    provider: "request-scoped-live-provider",
    replay: default_route.replay,
})

routing_guard_ok := models_err == nil && register_err == nil && default_err == nil && live_metadata_err == nil && replayed_err == nil
default_provider := default_route.provider
default_model := default_route.model
default_reason := default_route.trace.reason
default_replayed := default_route.trace.replayed
live_metadata_provider := live_metadata_route.provider
live_metadata_model := live_metadata_route.model
live_metadata_reason := live_metadata_route.trace.reason
replayed_provider := replayed_route.provider
replayed_model := replayed_route.model
replayed_reason := replayed_route.trace.reason
replayed_flag := replayed_route.trace.replayed
`); err != nil {
				t.Fatalf("route Exec: %v", err)
			}

			for name, want := range map[string]any{
				"routing_guard_ok":       true,
				"default_provider":       "fixture-replay",
				"default_model":          "mock-provider-free-default",
				"default_reason":         "alias",
				"default_replayed":       false,
				"live_metadata_provider": "future-live-provider",
				"live_metadata_model":    "future-live-model",
				"live_metadata_reason":   "alias",
				"replayed_provider":      "fixture-replay",
				"replayed_model":         "mock-provider-free-default",
				"replayed_reason":        "replay",
				"replayed_flag":          true,
			} {
				got, err := routeVM.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			if routeProviderCalls != 0 || routeFactoryCalls != 0 {
				t.Fatalf("routing guard called host live provider/factory: provider=%d factory=%d", routeProviderCalls, routeFactoryCalls)
			}

			var replayFactoryCalls int
			replayVM := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibLLM),
				leia.WithLLMProviderFactory(func(llm.ProviderConfig) (llm.Provider, error) {
					replayFactoryCalls++
					return genericModelRegistryRoutingGuardProvider{calls: new(int)}, nil
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
`); err != nil {
				t.Fatalf("replay Exec: %v", err)
			}
			for name, want := range map[string]any{
				"turn_ok":   true,
				"turn_text": "offline default replay",
			} {
				got, err := replayVM.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			if replayFactoryCalls != 0 {
				t.Fatalf("replay/default route called request-scoped live provider factory: %d", replayFactoryCalls)
			}
		})
	}
}

func TestGenericModelRegistryRoutingGuardTreatsVendorAndOptionalPackagesAsExternalMetadata(t *testing.T) {
	root := repoRoot(t)
	vendorPath := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "vendor_adapters", "manifest.json")
	optionalPath := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "optional_integrations", "package.manifest.json")
	optionalContractPath := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "optional_integrations", "contracts", "optional_integration_capability_gates.json")

	for _, path := range []string{vendorPath, optionalPath, optionalContractPath} {
		var value any
		decodeGenericModelRegistryRoutingGuardJSON(t, path, &value)
		assertGenericModelRegistryRoutingGuardNoLiveDefaults(t, path, value)
	}

	vendorManifest := loadVendorLivePackageManifest(t, filepath.Dir(vendorPath))
	if !vendorManifest.ProviderFree || vendorManifest.LiveNetwork || vendorManifest.RealDependencyImports {
		t.Fatalf("vendor adapters must stay external provider metadata: %#v", vendorManifest)
	}
	if vendorManifest.DefaultPolicy.Mode != "fixture_replay" ||
		vendorManifest.DefaultPolicy.LiveNetwork ||
		vendorManifest.DefaultPolicy.ProviderCredentialsRequired ||
		!vendorManifest.DefaultPolicy.CleanSkipWithoutCredentials {
		t.Fatalf("vendor adapters default policy enables live provider behavior: %#v", vendorManifest.DefaultPolicy)
	}
	for _, adapter := range vendorManifest.Adapters {
		if adapter.Credential.Required || adapter.Terms.LiveNetwork {
			t.Fatalf("%s vendor adapter crossed from metadata into live defaults: %#v", adapter.ID, adapter)
		}
	}

	optionalManifest := loadOptionalIntegrationsLiveManifest(t, filepath.Dir(optionalPath))
	if !optionalManifest.ProviderFree || optionalManifest.LiveNetworkDefault || optionalManifest.RealDependencyImportDefault {
		t.Fatalf("optional integrations must stay external provider metadata: %#v", optionalManifest)
	}
	if len(optionalManifest.Credentials.Required) != 0 ||
		len(optionalManifest.Credentials.Optional) != 0 ||
		len(optionalManifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("optional integrations should not declare default credentials: %#v", optionalManifest.Credentials)
	}
	if optionalManifest.DefaultPolicy.Mode != "fixture_replay" ||
		optionalManifest.DefaultPolicy.LiveNetwork ||
		optionalManifest.DefaultPolicy.ProviderCredentialsRequired ||
		optionalManifest.DefaultPolicy.RealDependencyImports ||
		!optionalManifest.DefaultPolicy.CleanSkipWithoutDependency {
		t.Fatalf("optional integrations default policy enables live provider behavior: %#v", optionalManifest.DefaultPolicy)
	}
}

func decodeGenericModelRegistryRoutingGuardJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertGenericModelRegistryRoutingGuardNoLiveDefaults(t *testing.T, path string, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "live_network", "live_network_default", "live_model_calls", "real_dependency_imports", "real_dependency_import_default", "provider_credentials_required", "dependency_imported":
				if child == true {
					t.Fatalf("%s declares default live/provider execution via %s=true", path, key)
				}
			}
			assertGenericModelRegistryRoutingGuardNoLiveDefaults(t, path, child)
		}
	case []any:
		for _, child := range value {
			assertGenericModelRegistryRoutingGuardNoLiveDefaults(t, path, child)
		}
	}
}
