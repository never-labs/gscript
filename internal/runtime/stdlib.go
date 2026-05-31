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

	// Table library (sort + higher-order functions need interp)
	tblLib := buildTableLib()
	buildTableProxyWithInterp(interp, tblLib)
	buildTableSortWithInterp(interp, tblLib)
	buildTableHigherOrderWithInterp(interp, tblLib)
	std.RegisterTable("table", tblLib)

	if includeMigratedCompat {
		interp.installLegacyMigratedStdlib(std)
	}

	// --- System ---
	std.RegisterTable("script", buildScriptLib(interp))
	std.RegisterTable("debug", buildDebugLib(interp))
	std.RegisterTable("testkit", buildTestkitLib(interp))

	std.InstallPackage(interp.scriptDir)
}

// installLegacyMigratedStdlib is the compatibility path for runtime.New and
// direct runtime.Interpreter.InstallStdlib callers. Public embedding goes
// through stdlibrt/install, so new migrated modules should not be added here.
func (interp *Interpreter) installLegacyMigratedStdlib(std StdlibInstaller) {
	strLib := BuildStringLibWithCaller(interp.callFunction, func() int64 { return interp.maxHostResult })
	std.RegisterTable("string", strLib)
	interp.SetStringLibrary(strLib)

	std.RegisterTable("math", buildMathLib())
	std.RegisterTable("rl", rlLib(interp))
	std.RegisterTable("io", buildIOLib(interp))
	std.RegisterTable("os", buildOSLibWithPolicy(
		interp.environmentRead,
		interp.environmentWrite,
		interp.allowedEnv,
		interp.filesystemRoot,
		interp.filesystemWrite,
	))
	std.RegisterTable("http", httpLib(interp))
	std.RegisterTable("json", buildJSONLib())
	std.RegisterTable("fs", buildFSLibWithCapabilities(
		interp.filesystemRoot,
		interp.filesystemRead,
		interp.filesystemWrite,
		interp.maxFSReadBytes,
		interp.maxFSWriteBytes,
	))
	std.RegisterTable("time", buildTimeLib())
	std.RegisterTable("net", buildNetLib(interp))
	std.RegisterTable("process", buildProcessLib(interp))
	std.RegisterTable("soa", buildSoALib())
	std.RegisterTable("sync", BuildSyncLibWithCaller(interp.callFunction))

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

	std.RegisterTable("matrix", buildMatrixLib())
	std.RegisterTable("utf8", buildUTF8Lib(interp))
}
