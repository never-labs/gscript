package gscript_test

import (
	gs "github.com/never-labs/gscript/gscript"
	"testing"
)

func TestAINativeAgentFlowImplicitConfigLocalsAreWhitelistedAndShadowable(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(append([]gs.Option{gs.WithLibs(gs.LibLLM)}, tc.opts...)...)
			err := vm.Exec(`
agent probe(q) {
    model: "cfg-model"
    system: "cfg-system"
    capabilities: ["cfg.cap"]
    user: q
    response_format: {type: "json_object"}
} flow {
    observed := model .. "|" .. system .. "|" .. capabilities[1]
    model := "local-model"
    system := "local-system"
    capabilities := ["local.cap"]
    return {
        observed: observed,
        shadowed: model .. "|" .. system .. "|" .. capabilities[1]
    }, nil
}

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

func TestAINativeAgentFlowDoesNotInjectArbitraryMetaFields(t *testing.T) {
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
				opts []gs.Option
			}{
				{name: "interpreter"},
				{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					vm := gs.New(append([]gs.Option{gs.WithLibs(gs.LibLLM)}, tc.opts...)...)
					err := vm.Exec(`
` + field.name + ` := "outer-` + field.name + `"

agent probe(q) {
    model: "cfg-model"
    ` + field.config + `
} flow {
    return ` + field.name + `, nil
}

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
