package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func TestLLMSchemaEdgeRejectsMalformedSchemaInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "validate output schema must be table",
			src: `
ok, msg := llm.validate_output({name: "Ada"}, "not a schema table")
got := tostring(ok) .. "|" .. msg
`,
			want: "false|schema must be a table example or JSON schema",
		},
		{
			name: "output schema requires schema spec",
			src:  `format := llm.output_schema("contact", nil)`,
			want: "bad argument to 'llm.output_schema' (schema spec expected)",
		},
		{
			name: "schema info requires schema spec",
			src:  `info := llm.schema_info()`,
			want: "bad argument #1 to 'llm.schema_info' (schema spec expected)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name string
				opts []leia.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
					err := vm.Exec(tc.src)
					if strings.HasPrefix(tc.src, "\n") {
						if err != nil {
							t.Fatalf("Exec: %v", err)
						}
						got, err := vm.Get("got")
						if err != nil {
							t.Fatalf("Get got: %v", err)
						}
						if !strings.Contains(got.(string), tc.want) {
							t.Fatalf("got = %#v, want substring %q", got, tc.want)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("Exec error = %v, want substring %q", err, tc.want)
					}
				})
			}
		})
	}
}

func TestLLMUnknownEvidenceShapeFallsBackToUserMessage(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			provider := &mockLLMProvider{res: llm.TurnResult{Status: "final_answer", Text: "ok"}}
			vm := leia.New(llmScenarioOptions(provider, mode.opts...)...)
			if err := vm.Exec(`
result, err := llm.turn({
    model: "mock"
    messages: [llm.user("base question")]
    evidence: {unknown_shape: {nested: "value"}}
})
err_is_nil := err == nil
text := result.text
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("provider requests = %d, want 1", len(provider.requests))
			}
			messages := provider.requests[0].Messages
			if len(messages) != 2 ||
				messages[0].Role != "user" || messages[0].Text != "base question" ||
				messages[1].Role != "user" || messages[1].Text == "" {
				t.Fatalf("messages = %#v", messages)
			}
			for name, want := range map[string]any{
				"err_is_nil": true,
				"text":       "ok",
			} {
				got, err := vm.Get(name)
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
