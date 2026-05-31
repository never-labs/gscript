package runtime

import (
	"context"
)

// InstallStdlib registers all standard-library tables as globals.
func (interp *Interpreter) InstallStdlib() {
	std := newStdlibInstallContext(interp)

	// String library
	strLib := BuildStringLibWithCaller(interp.callFunction, func() int64 { return interp.maxHostResult })
	std.RegisterTable("string", strLib)

	// Set up string metatable so "hello":upper() works
	interp.stringMeta = NewTable()
	interp.stringMeta.RawSet(StringValue("__index"), TableValue(strLib))

	// Table library (sort + higher-order functions need interp)
	tblLib := buildTableLib()
	buildTableProxyWithInterp(interp, tblLib)
	buildTableSortWithInterp(interp, tblLib)
	buildTableHigherOrderWithInterp(interp, tblLib)
	std.RegisterTable("table", tblLib)

	// Math library
	std.RegisterTable("math", buildMathLib())

	// IO library
	std.RegisterTable("io", buildIOLib(interp))

	// OS library
	std.RegisterTable("os", buildOSLib())

	// HTTP server library
	std.RegisterTable("http", httpLib(interp))

	// Raylib game library (window, drawing, input, audio)
	std.RegisterTable("rl", rlLib(interp))

	// --- Encoding / Crypto ---
	std.RegisterTable("array", buildArrayLib())
	std.RegisterTable("json", buildJSONLib())
	std.RegisterTable("base64", buildBase64Lib(interp))
	std.RegisterTable("hash", buildHashLib())

	// --- File system & paths ---
	std.RegisterTable("fs", buildFSLib(interp.filesystemRoot))
	std.RegisterTable("path", buildPathLib())

	// --- Time & networking ---
	std.RegisterTable("time", buildTimeLib())
	std.RegisterTable("net", buildNetLib(interp))

	// --- System ---
	std.RegisterTable("process", buildProcessLib(interp))
	std.RegisterTable("script", buildScriptLib(interp))
	std.RegisterTable("sync", BuildSyncLibWithCaller(interp.callFunction))
	std.RegisterTable("debug", buildDebugLib(interp))
	std.RegisterTable("testkit", buildTestkitLib(interp))

	// --- Data formats ---
	std.RegisterTable("csv", buildCSVLib(interp))
	std.RegisterTable("url", buildURLLib(interp))

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
	std.RegisterTable("llm", llmLib)
	std.RegisterAlias("toolof", llmLib.RawGetString("toolof"))
	std.RegisterTable("msg", BuildLLMMessageLib())
	std.RegisterTable("history", BuildLLMHistoryLib())
	std.RegisterTable("chat", BuildChatLib())
	std.RegisterTable("loop", BuildLLMLoopLib(interp.callFunction, func() LLMProvider {
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
	}))

	// --- Utilities ---
	std.RegisterTable("uuid", buildUUIDLib())
	std.RegisterTable("bytes", buildBytesLib(interp))
	std.RegisterTable("binary", buildBinaryLib(interp))

	// --- Game math ---
	std.RegisterTable("vec", buildVecLib())
	std.RegisterTable("color", buildColorLib())

	// --- Numeric matrix (R42 DenseMatrix Phase 1) ---
	std.RegisterTable("matrix", buildMatrixLib())
	std.RegisterTable("soa", buildSoALib())

	// --- Text processing ---
	std.RegisterTable("regexp", buildRegexpLib())
	std.RegisterTable("utf8", buildUTF8Lib(interp))

	// --- Low-level ---
	std.RegisterTable("bit32", buildBit32Lib())
	std.RegisterTable("bits", buildBitsLib())

	// --- Random number generation ---
	std.RegisterTable("rand", buildRandLib())

	// --- Sorting utilities ---
	std.RegisterTable("sort", buildSortLib(interp))

	// --- Encoding utilities ---
	std.RegisterTable("encoding", buildEncodingLib(interp))

	// --- Compression ---
	std.RegisterTable("compress", buildCompressLib(interp))

	// --- Cryptography ---
	std.RegisterTable("crypto", buildCryptoLib(interp))

	// --- Container data structures ---
	std.RegisterTable("container", buildContainerLib(interp))
	std.RegisterTable("context", buildContextLib())

	// --- Logging ---
	std.RegisterTable("log", buildLogLib())

	std.InstallPackage(interp.scriptDir)
}
