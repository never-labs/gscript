package runtime

import "context"

var stdlibModuleNames = []string{
	"base64",
	"array",
	"binary",
	"bit32",
	"bits",
	"bytes",
	"color",
	"compress",
	"container",
	"context",
	"crypto",
	"csv",
	"debug",
	"encoding",
	"fs",
	"hash",
	"http",
	"io",
	"json",
	"llm",
	"log",
	"math",
	"matrix",
	"net",
	"os",
	"path",
	"process",
	"rand",
	"regexp",
	"rl",
	"script",
	"soa",
	"sort",
	"string",
	"sync",
	"table",
	"testkit",
	"time",
	"url",
	"utf8",
	"uuid",
	"vec",
}

func StdlibModuleNames() []string {
	out := make([]string, len(stdlibModuleNames))
	copy(out, stdlibModuleNames)
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
	interp.globals.Define("llm", TableValue(BuildLLMLib(interp.callFunction, func() LLMProvider {
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
	for _, name := range stdlibModuleNames {
		if v, ok := interp.globals.Get(name); ok && v.IsTable() {
			interp.modules[name] = v
			loaded.RawSetString(name, v)
		}
	}
	pkg.RawSetString("loaded", TableValue(loaded))
	pkg.RawSetString("path", StringValue(interp.scriptDir))
	interp.globals.Define("package", TableValue(pkg))
}
