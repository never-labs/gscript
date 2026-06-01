// Package catalog describes the public standard-library surface without
// depending on runtime values or module constructors.
package catalog

const (
	LayerBase   = "base"
	LayerHost   = "host"
	LayerLLM    = "llm"
	LayerData   = "data"
	LayerVendor = "vendor"
	LayerCompat = "compat"
)

// ModuleInfo describes a standard-library module for CLI capability reports,
// test policy, and sandbox planning.
type ModuleInfo struct {
	Name         string
	Layer        string
	Capabilities []string
	SafeDefault  bool
}

var modules = []ModuleInfo{
	{Name: "array", Layer: LayerData, SafeDefault: true},
	{Name: "base64", Layer: LayerBase, SafeDefault: true},
	{Name: "binary", Layer: LayerData, SafeDefault: true},
	{Name: "bit32", Layer: LayerCompat, SafeDefault: true},
	{Name: "bits", Layer: LayerBase, SafeDefault: true},
	{Name: "bytes", Layer: LayerBase, SafeDefault: true},
	{Name: "chat", Layer: LayerLLM, Capabilities: []string{"llm.turn"}},
	{Name: "color", Layer: LayerBase, SafeDefault: true},
	{Name: "compress", Layer: LayerBase, SafeDefault: true},
	{Name: "container", Layer: LayerBase, SafeDefault: true},
	{Name: "context", Layer: LayerBase, SafeDefault: true},
	{Name: "crypto", Layer: LayerBase, SafeDefault: true},
	{Name: "csv", Layer: LayerData, SafeDefault: true},
	{Name: "debug", Layer: LayerHost, Capabilities: []string{"debug"}},
	{Name: "encoding", Layer: LayerBase, SafeDefault: true},
	{Name: "fs", Layer: LayerHost, Capabilities: []string{"fs.read", "fs.write"}},
	{Name: "hash", Layer: LayerBase, SafeDefault: true},
	{Name: "history", Layer: LayerLLM},
	{Name: "http", Layer: LayerHost, Capabilities: []string{"net.listen"}},
	{Name: "io", Layer: LayerHost, Capabilities: []string{"io"}},
	{Name: "json", Layer: LayerBase, SafeDefault: true},
	{Name: "llm", Layer: LayerLLM, Capabilities: []string{"llm.turn"}},
	{Name: "log", Layer: LayerHost, Capabilities: []string{"io.write"}},
	{Name: "loop", Layer: LayerLLM, Capabilities: []string{"llm.turn"}},
	{Name: "math", Layer: LayerBase, SafeDefault: true},
	{Name: "matrix", Layer: LayerData, SafeDefault: true},
	{Name: "msg", Layer: LayerLLM},
	{Name: "net", Layer: LayerHost, Capabilities: []string{"net.http"}},
	{Name: "os", Layer: LayerHost, Capabilities: []string{"env.read", "env.write"}},
	{Name: "path", Layer: LayerBase, SafeDefault: true},
	{Name: "process", Layer: LayerHost, Capabilities: []string{"process.exec", "process.shell"}},
	{Name: "rand", Layer: LayerBase, SafeDefault: true},
	{Name: "regexp", Layer: LayerBase, SafeDefault: true},
	{Name: "script", Layer: LayerHost, Capabilities: []string{"script.eval", "module.load"}},
	{Name: "soa", Layer: LayerData, SafeDefault: true},
	{Name: "sort", Layer: LayerBase, SafeDefault: true},
	{Name: "string", Layer: LayerBase, SafeDefault: true},
	{Name: "sync", Layer: LayerBase, SafeDefault: true},
	{Name: "table", Layer: LayerBase, SafeDefault: true},
	{Name: "testkit", Layer: LayerHost, Capabilities: []string{"testkit"}},
	{Name: "time", Layer: LayerBase, SafeDefault: true},
	{Name: "url", Layer: LayerBase, SafeDefault: true},
	{Name: "utf8", Layer: LayerBase, SafeDefault: true},
	{Name: "uuid", Layer: LayerBase, SafeDefault: true},
	{Name: "vec", Layer: LayerData, SafeDefault: true},
}

var layerOrder = []string{
	LayerBase,
	LayerHost,
	LayerLLM,
	LayerData,
	LayerVendor,
	LayerCompat,
}

func Modules() []ModuleInfo {
	out := make([]ModuleInfo, len(modules))
	for i, module := range modules {
		out[i] = module
		out[i].Capabilities = append([]string(nil), module.Capabilities...)
	}
	return out
}

func Module(name string) (ModuleInfo, bool) {
	for _, module := range modules {
		if module.Name == name {
			module.Capabilities = append([]string(nil), module.Capabilities...)
			return module, true
		}
	}
	return ModuleInfo{}, false
}

func ModuleNames() []string {
	out := make([]string, len(modules))
	for i, module := range modules {
		out[i] = module.Name
	}
	return out
}

func Layers() []string {
	return append([]string(nil), layerOrder...)
}

func ModulesForLayer(layer string) []ModuleInfo {
	var out []ModuleInfo
	for _, module := range modules {
		if module.Layer != layer {
			continue
		}
		module.Capabilities = append([]string(nil), module.Capabilities...)
		out = append(out, module)
	}
	return out
}
