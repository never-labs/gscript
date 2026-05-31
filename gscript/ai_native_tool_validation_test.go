package gscript_test

import (
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestAINativeSyntaxValidationAllowsStaticToolCapsCoverage(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
//gscript:requires docs.read, net.client
tool lookup(query) {
    return query, nil
}

//gscript:requires none
tool local_only(query) {
    return query, nil
}

agent defaults {
    tools: [lookup]
}

agent inherited(q) {
    capabilities: ["docs.read", "net.client"]
    user: q
}

agent override(q) {
    tools: [local_only]
    capabilities: []
    user: q
}

agent answer(q) {
    tools: [lookup]
    capabilities: ["docs.read", "net.client"]
    user: q
}

func f(caps) {
    _ = turn {
        tools: [lookup]
        caps: caps
        user: "hello"
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsDynamicDefaultsToolCapsRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
//gscript:requires net.client
tool lookup(query) {
    return query, nil
}

default_tools := [lookup]
default_caps := []

agent defaults {
    tools: default_tools
    capabilities: default_caps
}

agent answer(q) {
    user: q
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsDynamicToolRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
func f(tools) {
    _ = turn {
        tools: [tools[0]]
        user: "hello"
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestAINativeSyntaxValidationAllowsScopedToolRefs(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithLibs(gs.LibString | gs.LibLLM)}, mode.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`
func make_agent(prefix) {
    //gscript:requires none
    tool local_lookup(query) {
        return prefix .. query, nil
    }
    return agent(q) {
        tools: [local_lookup]
        user: q
    }
}
`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}
