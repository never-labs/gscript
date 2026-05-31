package catalog

import (
	"reflect"
	"sort"
	"testing"
)

func TestRegistryClassifiesEveryModule(t *testing.T) {
	allowedLayers := map[string]bool{}
	for _, layer := range Layers() {
		if allowedLayers[layer] {
			t.Fatalf("duplicate stdlib layer %q", layer)
		}
		allowedLayers[layer] = true
	}

	seen := map[string]bool{}
	for _, module := range Modules() {
		if module.Name == "" {
			t.Fatal("stdlib registry has module with empty name")
		}
		if seen[module.Name] {
			t.Fatalf("duplicate stdlib module %q", module.Name)
		}
		seen[module.Name] = true
		if !allowedLayers[module.Layer] {
			t.Fatalf("stdlib module %q has unknown layer %q", module.Name, module.Layer)
		}
	}

	names := ModuleNames()
	sort.Strings(names)
	for _, name := range names {
		module, ok := Module(name)
		if !ok {
			t.Fatalf("Module(%q) not found", name)
		}
		if module.Name != name {
			t.Fatalf("Module(%q).Name = %q", name, module.Name)
		}
	}
	if _, ok := Module("missing"); ok {
		t.Fatal("Module(\"missing\") unexpectedly found")
	}
}

func TestRegistryLayerQueries(t *testing.T) {
	cases := map[string][]string{
		LayerLLM:    {"chat", "history", "llm", "loop", "msg"},
		LayerCompat: {"bit32"},
		LayerData:   {"array", "binary", "csv", "matrix", "soa", "vec"},
		LayerHost:   {"debug", "fs", "http", "io", "log", "net", "os", "process", "script", "testkit"},
		LayerVendor: {"rl"},
	}
	for layer, want := range cases {
		got := moduleNames(ModulesForLayer(layer))
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ModulesForLayer(%q) = %#v, want %#v", layer, got, want)
		}
	}
	if got := ModulesForLayer("missing"); len(got) != 0 {
		t.Fatalf("ModulesForLayer(\"missing\") = %#v, want empty", moduleNames(got))
	}
}

func TestRegistryCopiesMetadata(t *testing.T) {
	modules := Modules()
	modules[0].Name = "mutated"
	if len(modules[0].Capabilities) > 0 {
		modules[0].Capabilities[0] = "mutated"
	}
	if got := ModuleNames()[0]; got == "mutated" {
		t.Fatal("Modules exposed mutable module name")
	}

	llm, ok := Module("llm")
	if !ok {
		t.Fatal("Module(\"llm\") not found")
	}
	llm.Capabilities[0] = "mutated"
	llmAgain, _ := Module("llm")
	if llmAgain.Capabilities[0] == "mutated" {
		t.Fatal("Module exposed mutable capabilities")
	}

	layers := Layers()
	layers[0] = "mutated"
	if got := Layers()[0]; got == "mutated" {
		t.Fatal("Layers exposed mutable layer order")
	}
}

func moduleNames(modules []ModuleInfo) []string {
	names := make([]string, len(modules))
	for i, module := range modules {
		names[i] = module.Name
	}
	return names
}
