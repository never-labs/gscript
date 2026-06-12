package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMConfigEnvSecretResolveAndRedact(t *testing.T) {
	t.Setenv("LEIA_TEST_CONFIG_TOKEN", "runtime-token-value")

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
spec := config.table({
    model: "mock-fast",
    api_key: config.secret("LEIA_TEST_CONFIG_TOKEN"),
})
resolved, err := config.resolve(spec)
display := config.display(spec)
redacted := config.redact(resolved)
safe_display := config.display(redacted)
llm_display := llm.config.display(spec)
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			got, err := vm.Get("display")
			if err != nil {
				t.Fatalf("Get display: %v", err)
			}
			if s := got.(string); strings.Contains(s, "runtime-token-value") || !strings.Contains(s, "<redacted:LEIA_TEST_CONFIG_TOKEN>") {
				t.Fatalf("display = %q, want redacted env reference without secret value", s)
			}
			got, err = vm.Get("safe_display")
			if err != nil {
				t.Fatalf("Get safe_display: %v", err)
			}
			if s := got.(string); strings.Contains(s, "runtime-token-value") || !strings.Contains(s, "api_key: <redacted>") {
				t.Fatalf("safe_display = %q, want redacted resolved table", s)
			}
			got, err = vm.Get("llm_display")
			if err != nil {
				t.Fatalf("Get llm_display: %v", err)
			}
			if s := got.(string); strings.Contains(s, "runtime-token-value") || !strings.Contains(s, "<redacted:LEIA_TEST_CONFIG_TOKEN>") {
				t.Fatalf("llm_display = %q, want llm.config alias to redact", s)
			}
		})
	}
}

func TestLLMConfigMissingKeyDiagnostic(t *testing.T) {
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
spec := config.table({api_key: config.secret("LEIA_TEST_CONFIG_MISSING")})
resolved, err := config.resolve(spec)
kind := err.kind
message := err.message
missing := err.missing
field := err.field
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]string{
				"kind":    "config",
				"missing": "LEIA_TEST_CONFIG_MISSING",
				"field":   "api_key",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %q", name, got, want)
				}
			}
			got, err := vm.Get("message")
			if err != nil {
				t.Fatalf("Get message: %v", err)
			}
			msg := got.(string)
			if !strings.Contains(msg, "missing required config key 'api_key'") || !strings.Contains(msg, "env 'LEIA_TEST_CONFIG_MISSING'") {
				t.Fatalf("message = %q, want missing key and env diagnostic", msg)
			}
		})
	}
}

func TestLLMConfigEnvHonorsEnvironmentCapability(t *testing.T) {
	t.Setenv("LEIA_TEST_CONFIG_BLOCKED", "must-not-read")

	vm := leia.New(leia.WithLibs(leia.LibLLM), leia.WithCapabilities(leia.CapSafe), leia.WithVM())
	if err := vm.Exec(`
config := llm.config
value, err := config.env("LEIA_TEST_CONFIG_BLOCKED", {required: true})
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
	if strings.Contains(got.(string), "must-not-read") || !strings.Contains(got.(string), "environment read access disabled") {
		t.Fatalf("message = %q, want capability error without secret", got)
	}
}
