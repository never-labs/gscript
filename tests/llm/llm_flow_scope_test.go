package leia_test

import (
	leia "github.com/never-labs/leia"
	"testing"
)

func TestLLMAgentFlowExplicitConfigLocalsAreShadowable(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibLLM)}, tc.opts...)...)
			err := vm.Exec(`
probe_config := {
    model: "cfg-model"
    system: "cfg-system"
    capabilities: {"cfg.cap"}
    response_format: {type: "json_object"}
}

probe := llm.agent("probe", func(q) {
    return {
        model: probe_config.model
        system: probe_config.system
        capabilities: probe_config.capabilities
        user: q
        response_format: probe_config.response_format
    }, nil
}, func(q) {
    observed := probe_config.model .. "|" .. probe_config.system .. "|" .. probe_config.capabilities[1]
    model := "local-model"
    system := "local-system"
    capabilities := {"local.cap"}
    return {
        observed: observed,
        shadowed: model .. "|" .. system .. "|" .. capabilities[1]
    }, nil
})

out, err := probe("hello")
observed := out.observed
shadowed := out.shadowed
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			observed, _ := vm.Get("observed")
			shadowed, _ := vm.Get("shadowed")
			if observed != "cfg-model|cfg-system|cfg.cap" {
				t.Fatalf("observed = %#v", observed)
			}
			if shadowed != "local-model|local-system|local.cap" {
				t.Fatalf("shadowed = %#v", shadowed)
			}
		})
	}
}

func TestLLMAgentFlowDoesNotInjectArbitraryMetaFields(t *testing.T) {
	for _, field := range []struct {
		name   string
		config string
	}{
		{name: "user", config: `user: q`},
		{name: "response_format", config: `response_format: {type: "json_object"}`},
		{name: "metadata", config: `metadata: {trace_id: "abc"}`},
	} {
		t.Run(field.name, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				opts []leia.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibLLM)}, tc.opts...)...)
					err := vm.Exec(`
` + field.name + ` := "outer-` + field.name + `"

probe := llm.agent("probe", func(q) {
    return {
        model: "cfg-model"
        ` + field.config + `
    }, nil
}, func(q) {
    return ` + field.name + `, nil
})

out, err := probe("hello")
got := out
`)
					if err != nil {
						t.Fatalf("Exec: %v", err)
					}
					got, err := vm.Get("got")
					if err != nil {
						t.Fatalf("Get got: %v", err)
					}
					if want := "outer-" + field.name; got != want {
						t.Fatalf("got = %#v, want %q", got, want)
					}
				})
			}
		})
	}
}
