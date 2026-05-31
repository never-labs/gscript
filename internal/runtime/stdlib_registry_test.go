package runtime

import (
	"reflect"
	"sort"
	"testing"
)

func TestStdlibRegistryClassifiesEveryModule(t *testing.T) {
	allowedLayers := map[string]bool{}
	for _, layer := range StdlibLayers() {
		if allowedLayers[layer] {
			t.Fatalf("duplicate stdlib layer %q", layer)
		}
		allowedLayers[layer] = true
	}

	seen := map[string]bool{}
	for _, module := range StdlibModules() {
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

	names := StdlibModuleNames()
	sort.Strings(names)
	for _, name := range names {
		module, ok := StdlibModule(name)
		if !ok {
			t.Fatalf("StdlibModule(%q) not found", name)
		}
		if module.Name != name {
			t.Fatalf("StdlibModule(%q).Name = %q", name, module.Name)
		}
	}
	if _, ok := StdlibModule("missing"); ok {
		t.Fatal("StdlibModule(\"missing\") unexpectedly found")
	}
}

func TestStdlibRegistryLayerQueries(t *testing.T) {
	cases := map[string][]string{
		StdlibLayerAI:     {"chat", "history", "llm", "loop", "msg"},
		StdlibLayerCompat: {"bit32"},
		StdlibLayerData:   {"array", "binary", "color", "csv", "matrix", "soa", "vec"},
		StdlibLayerHost:   {"debug", "fs", "http", "io", "log", "net", "os", "process", "script", "testkit"},
		StdlibLayerVendor: {"rl"},
	}
	for layer, want := range cases {
		got := moduleNames(StdlibModulesForLayer(layer))
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("StdlibModulesForLayer(%q) = %#v, want %#v", layer, got, want)
		}
	}
	if got := StdlibModulesForLayer("missing"); len(got) != 0 {
		t.Fatalf("StdlibModulesForLayer(\"missing\") = %#v, want empty", moduleNames(got))
	}
}

func TestStdlibRegistryCopiesMetadata(t *testing.T) {
	modules := StdlibModules()
	modules[0].Name = "mutated"
	if len(modules[0].Capabilities) > 0 {
		modules[0].Capabilities[0] = "mutated"
	}
	if got := StdlibModuleNames()[0]; got == "mutated" {
		t.Fatal("StdlibModules exposed mutable module name")
	}

	llm, ok := StdlibModule("llm")
	if !ok {
		t.Fatal("StdlibModule(\"llm\") not found")
	}
	llm.Capabilities[0] = "mutated"
	llmAgain, _ := StdlibModule("llm")
	if llmAgain.Capabilities[0] == "mutated" {
		t.Fatal("StdlibModule exposed mutable capabilities")
	}

	layers := StdlibLayers()
	layers[0] = "mutated"
	if got := StdlibLayers()[0]; got == "mutated" {
		t.Fatal("StdlibLayers exposed mutable layer order")
	}
}

func moduleNames(modules []StdlibModuleInfo) []string {
	names := make([]string, len(modules))
	for i, module := range modules {
		names[i] = module.Name
	}
	return names
}
