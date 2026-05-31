package runtime

import "context"

const (
	StdlibLayerBase   = "base"
	StdlibLayerHost   = "host"
	StdlibLayerAI     = "ai"
	StdlibLayerData   = "data"
	StdlibLayerVendor = "vendor"
	StdlibLayerCompat = "compat"
)

// StdlibModuleInfo describes the public standard-library surface in a form
// suitable for CLI capability reports, test policy, and sandbox planning.
type StdlibModuleInfo struct {
	Name         string
	Layer        string
	Capabilities []string
	SafeDefault  bool
}

var stdlibModules = []StdlibModuleInfo{
	{Name: "array", Layer: StdlibLayerData, SafeDefault: true},
	{Name: "base64", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "binary", Layer: StdlibLayerData, SafeDefault: true},
	{Name: "bit32", Layer: StdlibLayerCompat, SafeDefault: true},
	{Name: "bits", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "bytes", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "chat", Layer: StdlibLayerAI, Capabilities: []string{"llm.turn"}},
	{Name: "color", Layer: StdlibLayerData, SafeDefault: true},
	{Name: "compress", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "container", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "context", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "crypto", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "csv", Layer: StdlibLayerData, SafeDefault: true},
	{Name: "debug", Layer: StdlibLayerHost, Capabilities: []string{"debug"}},
	{Name: "encoding", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "fs", Layer: StdlibLayerHost, Capabilities: []string{"fs.read", "fs.write"}},
	{Name: "hash", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "history", Layer: StdlibLayerAI},
	{Name: "http", Layer: StdlibLayerHost, Capabilities: []string{"net.listen"}},
	{Name: "io", Layer: StdlibLayerHost, Capabilities: []string{"io"}},
	{Name: "json", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "llm", Layer: StdlibLayerAI, Capabilities: []string{"llm.turn"}},
	{Name: "log", Layer: StdlibLayerHost, Capabilities: []string{"io.write"}},
	{Name: "loop", Layer: StdlibLayerAI, Capabilities: []string{"llm.turn"}},
	{Name: "math", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "matrix", Layer: StdlibLayerData, SafeDefault: true},
	{Name: "msg", Layer: StdlibLayerAI},
	{Name: "net", Layer: StdlibLayerHost, Capabilities: []string{"net.http"}},
	{Name: "os", Layer: StdlibLayerHost, Capabilities: []string{"env.read", "env.write"}},
	{Name: "path", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "process", Layer: StdlibLayerHost, Capabilities: []string{"process.exec", "process.shell"}},
	{Name: "rand", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "regexp", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "rl", Layer: StdlibLayerVendor},
	{Name: "script", Layer: StdlibLayerHost, Capabilities: []string{"script.eval", "module.load"}},
	{Name: "soa", Layer: StdlibLayerData, SafeDefault: true},
	{Name: "sort", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "string", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "sync", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "table", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "testkit", Layer: StdlibLayerHost, Capabilities: []string{"testkit"}},
	{Name: "time", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "url", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "utf8", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "uuid", Layer: StdlibLayerBase, SafeDefault: true},
	{Name: "vec", Layer: StdlibLayerData, SafeDefault: true},
}

var stdlibLayerOrder = []string{
	StdlibLayerBase,
	StdlibLayerHost,
	StdlibLayerAI,
	StdlibLayerData,
	StdlibLayerVendor,
	StdlibLayerCompat,
}

func StdlibModules() []StdlibModuleInfo {
	out := make([]StdlibModuleInfo, len(stdlibModules))
	for i, module := range stdlibModules {
		out[i] = module
		out[i].Capabilities = append([]string(nil), module.Capabilities...)
	}
	return out
}

func StdlibModule(name string) (StdlibModuleInfo, bool) {
	for _, module := range stdlibModules {
		if module.Name == name {
			module.Capabilities = append([]string(nil), module.Capabilities...)
			return module, true
		}
	}
	return StdlibModuleInfo{}, false
}

func StdlibModuleNames() []string {
	out := make([]string, len(stdlibModules))
	for i, module := range stdlibModules {
		out[i] = module.Name
	}
	return out
}

func StdlibLayers() []string {
	return append([]string(nil), stdlibLayerOrder...)
}

func StdlibModulesForLayer(layer string) []StdlibModuleInfo {
	var out []StdlibModuleInfo
	for _, module := range stdlibModules {
		if module.Layer != layer {
			continue
		}
		module.Capabilities = append([]string(nil), module.Capabilities...)
		out = append(out, module)
	}
	return out
}

// registerStdlib registers all standard library tables as globals.
// This is called from New() after registerBuiltins().
func (interp *Interpreter) registerStdlib() {
	// String library
	strLib := BuildStringLibWithCaller(interp.callFunction, func() int64 { return interp.maxHostResult })
	interp.globals.Define("string", TableValue(strLib))

	// Set up string metatable so "hello":upper() works
	interp.stringMeta = NewTable()
	interp.stringMeta.RawSet(StringValue("__index"), TableValue(strLib))

	// Table library (sort + higher-order functions need interp)
	tblLib := buildTableLib()
	buildTableProxyWithInterp(interp, tblLib)
	buildTableSortWithInterp(interp, tblLib)
	buildTableHigherOrderWithInterp(interp, tblLib)
	interp.globals.Define("table", TableValue(tblLib))

	// Math library
	interp.globals.Define("math", TableValue(buildMathLib()))

	// IO library
	interp.globals.Define("io", TableValue(buildIOLib(interp)))

	// OS library
	interp.globals.Define("os", TableValue(buildOSLib()))

	// HTTP server library
	interp.globals.Define("http", TableValue(httpLib(interp)))

	// Raylib game library (window, drawing, input, audio)
	interp.globals.Define("rl", TableValue(rlLib(interp)))

	// --- Encoding / Crypto ---
	interp.globals.Define("array", TableValue(buildArrayLib()))
	interp.globals.Define("json", TableValue(buildJSONLib()))
	interp.globals.Define("base64", TableValue(buildBase64Lib(interp)))
	interp.globals.Define("hash", TableValue(buildHashLib()))

	// --- File system & paths ---
	interp.globals.Define("fs", TableValue(buildFSLib(interp.filesystemRoot)))
	interp.globals.Define("path", TableValue(buildPathLib()))

	// --- Time & networking ---
	interp.globals.Define("time", TableValue(buildTimeLib()))
	interp.globals.Define("net", TableValue(buildNetLib(interp)))

	// --- System ---
	interp.globals.Define("process", TableValue(buildProcessLib(interp)))
	interp.globals.Define("script", TableValue(buildScriptLib(interp)))
	interp.globals.Define("sync", TableValue(BuildSyncLibWithCaller(interp.callFunction)))
	interp.globals.Define("debug", TableValue(buildDebugLib(interp)))
	interp.globals.Define("testkit", TableValue(buildTestkitLib(interp)))

	// --- Data formats ---
	interp.globals.Define("csv", TableValue(buildCSVLib(interp)))
	interp.globals.Define("url", TableValue(buildURLLib(interp)))

	// --- AI model integration ---
	llmLib := BuildLLMLib(interp.callFunction, func() LLMProvider {
		return interp.llmProvider
	}, func() LLMProviderFactory {
		return interp.llmProviderFactory
	}, func() int64 {
		return interp.maxHostResult
	}, func() context.Context {
		if interp.ctx == nil {
			return context.Background()
		}
		return interp.ctx
	}, func(event LLMTraceEvent) {
		if interp.llmTraceSink != nil {
			interp.llmTraceSink(event)
		}
	})
	interp.globals.Define("llm", TableValue(llmLib))
	interp.globals.Define("toolof", llmLib.RawGetString("toolof"))
	interp.globals.Define("msg", TableValue(BuildLLMMessageLib()))
	interp.globals.Define("history", TableValue(BuildLLMHistoryLib()))
	interp.globals.Define("chat", TableValue(BuildChatLib()))
	interp.globals.Define("loop", TableValue(BuildLLMLoopLib(interp.callFunction, func() LLMProvider {
		return interp.llmProvider
	}, func() int64 {
		return interp.maxHostResult
	}, func() context.Context {
		if interp.ctx == nil {
			return context.Background()
		}
		return interp.ctx
	}, func(event LLMTraceEvent) {
		if interp.llmTraceSink != nil {
			interp.llmTraceSink(event)
		}
	})))

	// --- Utilities ---
	interp.globals.Define("uuid", TableValue(buildUUIDLib()))
	interp.globals.Define("bytes", TableValue(buildBytesLib(interp)))
	interp.globals.Define("binary", TableValue(buildBinaryLib(interp)))

	// --- Game math ---
	interp.globals.Define("vec", TableValue(buildVecLib()))
	interp.globals.Define("color", TableValue(buildColorLib()))

	// --- Numeric matrix (R42 DenseMatrix Phase 1) ---
	interp.globals.Define("matrix", TableValue(buildMatrixLib()))
	interp.globals.Define("soa", TableValue(buildSoALib()))

	// --- Text processing ---
	interp.globals.Define("regexp", TableValue(buildRegexpLib()))
	interp.globals.Define("utf8", TableValue(buildUTF8Lib(interp)))

	// --- Low-level ---
	interp.globals.Define("bit32", TableValue(buildBit32Lib()))
	interp.globals.Define("bits", TableValue(buildBitsLib()))

	// --- Random number generation ---
	interp.globals.Define("rand", TableValue(buildRandLib()))

	// --- Sorting utilities ---
	interp.globals.Define("sort", TableValue(buildSortLib(interp)))

	// --- Encoding utilities ---
	interp.globals.Define("encoding", TableValue(buildEncodingLib(interp)))

	// --- Compression ---
	interp.globals.Define("compress", TableValue(buildCompressLib(interp)))

	// --- Cryptography ---
	interp.globals.Define("crypto", TableValue(buildCryptoLib(interp)))

	// --- Container data structures ---
	interp.globals.Define("container", TableValue(buildContainerLib(interp)))
	interp.globals.Define("context", TableValue(buildContextLib()))

	// --- Logging ---
	interp.globals.Define("log", TableValue(buildLogLib()))

	interp.registerPackageLib()
}

func (interp *Interpreter) registerPackageLib() {
	pkg := NewTable()
	loaded := NewTable()
	for _, module := range stdlibModules {
		name := module.Name
		if v, ok := interp.globals.Get(name); ok && v.IsTable() {
			interp.modules[name] = v
			loaded.RawSetString(name, v)
		}
	}
	pkg.RawSetString("loaded", TableValue(loaded))
	pkg.RawSetString("path", StringValue(interp.scriptDir))
	interp.globals.Define("package", TableValue(pkg))
}
