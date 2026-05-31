package runtime

import (
	"context"
)

// InstallRuntimeStdlib registers runtime-owned standard-library tables as
// globals. stdlibrt/install calls this before installing modules that have
// migrated out of runtime, so this method must not register migrated modules.
func (interp *Interpreter) InstallRuntimeStdlib() {
	interp.installStdlib(false)
}

// InstallStdlib registers all legacy runtime standard-library tables as
// globals. Newer embedding entry points should prefer runtime.NewCore plus
// stdlibrt/install.Install so migrated modules are installed from stdlibrt.
func (interp *Interpreter) InstallStdlib() {
	interp.installStdlib(true)
}

func (interp *Interpreter) installStdlib(includeMigratedCompat bool) {
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

	if includeMigratedCompat {
		// Historical direct-runtime install path. Public embedding now installs
		// math from stdlibrt/modules through stdlibrt/install.
		std.RegisterTable("math", buildMathLib())
	}

	if includeMigratedCompat {
		// Historical direct-runtime install path. Public embedding now installs
		// these from stdlibrt/modules through stdlibrt/install.
		std.RegisterTable("io", buildIOLib(interp))
		std.RegisterTable("os", buildOSLib())
		std.RegisterTable("http", httpLib(interp))
	}

	// Raylib game library (window, drawing, input, audio)
	std.RegisterTable("rl", rlLib(interp))

	if includeMigratedCompat {
		// Historical direct-runtime install path. Public embedding now installs
		// json from stdlibrt/modules through stdlibrt/install.
		std.RegisterTable("json", buildJSONLib())
	}

	if includeMigratedCompat {
		std.RegisterTable("fs", buildFSLib(interp.filesystemRoot))
	}

	// --- Time & networking ---
	if includeMigratedCompat {
		// Historical direct-runtime install path. Public embedding now installs
		// time from stdlibrt/modules through stdlibrt/install.
		std.RegisterTable("time", buildTimeLib())
	}
	if includeMigratedCompat {
		std.RegisterTable("net", buildNetLib(interp))
	}

	// --- System ---
	std.RegisterTable("process", buildProcessLib(interp))
	std.RegisterTable("script", buildScriptLib(interp))
	std.RegisterTable("sync", BuildSyncLibWithCaller(interp.callFunction))
	std.RegisterTable("debug", buildDebugLib(interp))
	std.RegisterTable("testkit", buildTestkitLib(interp))

	if includeMigratedCompat {
		// Historical direct-runtime install path. Public embedding now installs
		// LLM bindings from stdlibrt/modules through stdlibrt/install.
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
	}

	if includeMigratedCompat {
		// Historical direct-runtime install path. Public embedding now installs
		// matrix from stdlibrt/modules through stdlibrt/install.
		std.RegisterTable("matrix", buildMatrixLib())
	}
	std.RegisterTable("soa", buildSoALib())

	if includeMigratedCompat {
		// Historical direct-runtime install path. Public embedding now installs
		// utf8 from stdlibrt/modules through stdlibrt/install.
		std.RegisterTable("utf8", buildUTF8Lib(interp))
	}

	std.InstallPackage(interp.scriptDir)
}
