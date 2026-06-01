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
	Description  string
	Capabilities []string
	SafeDefault  bool
}

var modules = []ModuleInfo{
	{Name: "array", Layer: LayerData, Description: "Dense typed arrays and conversion helpers for hot data loops.", SafeDefault: true},
	{Name: "base64", Layer: LayerBase, Description: "Base64 and URL-safe base64 encode/decode helpers.", SafeDefault: true},
	{Name: "binary", Layer: LayerData, Description: "Binary pack/unpack over Leia strings using declarative field formats.", SafeDefault: true},
	{Name: "bit32", Layer: LayerCompat, Description: "Lua-compatible 32-bit bit operations.", SafeDefault: true},
	{Name: "bits", Layer: LayerBase, Description: "Go math/bits-style integer bit counting and rotation helpers.", SafeDefault: true},
	{Name: "bytes", Layer: LayerBase, Description: "Byte-string transforms, buffers, hex helpers, and byte comparisons.", SafeDefault: true},
	{Name: "chat", Layer: LayerLLM, Description: "Lightweight chat and conversation helpers for AI-native scripts.", Capabilities: []string{"llm.turn"}},
	{Name: "color", Layer: LayerBase, Description: "RGBA colors, color-space conversion, interpolation, and common constants.", SafeDefault: true},
	{Name: "compress", Layer: LayerBase, Description: "Compression and decompression helpers over strings.", SafeDefault: true},
	{Name: "container", Layer: LayerBase, Description: "Sets, queues, stacks, deques, and heaps implemented in process.", SafeDefault: true},
	{Name: "context", Layer: LayerBase, Description: "Cancellation, timeout, and done-channel helpers.", SafeDefault: true},
	{Name: "crypto", Layer: LayerBase, Description: "Secure random data and high-level cryptographic primitives.", SafeDefault: true},
	{Name: "csv", Layer: LayerData, Description: "CSV parse and encode helpers backed by Go's CSV behavior.", SafeDefault: true},
	{Name: "debug", Layer: LayerHost, Description: "Runtime stack, globals, hook, and diagnostic helpers.", Capabilities: []string{"debug"}},
	{Name: "encoding", Layer: LayerBase, Description: "Text and byte encoding conversion helpers.", SafeDefault: true},
	{Name: "fs", Layer: LayerHost, Description: "Filesystem read, write, stat, directory, glob, and path-affecting operations.", Capabilities: []string{"fs.read", "fs.write"}},
	{Name: "hash", Layer: LayerBase, Description: "Hash digest helpers over strings and byte data.", SafeDefault: true},
	{Name: "history", Layer: LayerLLM, Description: "Conversation history search, append, and recall helpers."},
	{Name: "http", Layer: LayerHost, Description: "HTTP client/server helpers and request/response adaptation.", Capabilities: []string{"net.listen"}},
	{Name: "io", Layer: LayerHost, Description: "File handles, process stdio, and stream helpers.", Capabilities: []string{"io"}},
	{Name: "json", Layer: LayerBase, Description: "JSON encode/decode/validate helpers over Leia values.", SafeDefault: true},
	{Name: "llm", Layer: LayerLLM, Description: "Model turns, tools, validation, record/replay, and provider-backed AI calls.", Capabilities: []string{"llm.turn"}},
	{Name: "log", Layer: LayerHost, Description: "In-process log sink, levels, and log records.", Capabilities: []string{"io.write"}},
	{Name: "loop", Layer: LayerLLM, Description: "Reusable AI agent loop drivers such as react and plan/execute.", Capabilities: []string{"llm.turn"}},
	{Name: "math", Layer: LayerBase, Description: "Numeric functions, constants, and Lua-compatible math helpers.", SafeDefault: true},
	{Name: "matrix", Layer: LayerData, Description: "Dense matrix values and numeric matrix helpers.", SafeDefault: true},
	{Name: "msg", Layer: LayerLLM, Description: "Normalized LLM message constructors for system, user, assistant, and tool roles."},
	{Name: "net", Layer: LayerHost, Description: "Network helpers and HTTP-facing host integration.", Capabilities: []string{"net.http"}},
	{Name: "os", Layer: LayerHost, Description: "Environment, process metadata, temporary names, and Lua-style OS helpers.", Capabilities: []string{"env.read", "env.write"}},
	{Name: "path", Layer: LayerBase, Description: "Host filepath join, clean, split, match, rel, and separator helpers.", SafeDefault: true},
	{Name: "process", Layer: LayerHost, Description: "Subprocess execution, shell execution, lookup, args, and entry metadata.", Capabilities: []string{"process.exec", "process.shell"}},
	{Name: "rand", Layer: LayerBase, Description: "Pseudo-random values, ranges, bytes, and seedable random helpers.", SafeDefault: true},
	{Name: "regexp", Layer: LayerBase, Description: "Go RE2 regular expression compile, match, find, replace, and split helpers.", SafeDefault: true},
	{Name: "script", Layer: LayerHost, Description: "Script compilation, evaluation, loader, source, and entrypoint helpers.", Capabilities: []string{"script.eval", "module.load"}},
	{Name: "soa", Layer: LayerData, Description: "Structure-of-arrays records and column-oriented data processing.", SafeDefault: true},
	{Name: "sort", Layer: LayerBase, Description: "Sort helpers for arrays, numbers, tables, and callback-based ordering.", SafeDefault: true},
	{Name: "string", Layer: LayerBase, Description: "Lua-style byte-string helpers plus Go-style string utilities.", SafeDefault: true},
	{Name: "sync", Layer: LayerBase, Description: "In-process waitgroup, mutex, rwmutex, and once primitives.", SafeDefault: true},
	{Name: "table", Layer: LayerBase, Description: "Lua-compatible table helpers plus higher-order table utilities.", SafeDefault: true},
	{Name: "testkit", Layer: LayerHost, Description: "Conformance and diagnostic helpers intended for tests.", Capabilities: []string{"testkit"}},
	{Name: "time", Layer: LayerBase, Description: "Wall-clock, duration, sleep, parse, format, and timeout helpers.", SafeDefault: true},
	{Name: "url", Layer: LayerBase, Description: "URL parse, escape, query, and construction helpers.", SafeDefault: true},
	{Name: "utf8", Layer: LayerBase, Description: "UTF-8 validation, codepoint, length, and offset helpers.", SafeDefault: true},
	{Name: "uuid", Layer: LayerBase, Description: "UUID generation, parsing, validation, and metadata helpers.", SafeDefault: true},
	{Name: "vec", Layer: LayerData, Description: "Vector construction, arithmetic, geometry, and numeric helpers.", SafeDefault: true},
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
