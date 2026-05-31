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

// InstallStdlib registers the standard-library tables that are still owned by
// runtime. Newer embedding entry points should prefer runtime.NewCore plus
// stdlibrt/install.Install so external modules are installed from stdlibrt.
func (interp *Interpreter) InstallStdlib() {
	interp.installStdlib(true)
}

func (interp *Interpreter) installStdlib(includeMigratedCompat bool) {
	std := newStdlibInstallContext(interp)

	if includeMigratedCompat {
		interp.installRuntimeOwnedStdlib(std)
	}

	// --- System ---
	std.RegisterTable("script", buildScriptLib(interp))

	std.InstallPackage(interp.scriptDir)
}

// installRuntimeOwnedStdlib registers the few stdlib surfaces that have not yet
// moved behind stdlibrt/modules. Public embedding goes through stdlibrt/install,
// so newly extracted modules should not be added here.
func (interp *Interpreter) installRuntimeOwnedStdlib(std StdlibInstaller) {
	tblLib := buildTableLib()
	buildTableProxyWithInterp(interp, tblLib)
	buildTableSortWithInterp(interp, tblLib)
	buildTableHigherOrderWithInterp(interp, tblLib)
	std.RegisterTable("table", tblLib)

	strLib := BuildStringLibWithCaller(interp.callFunction, func() int64 { return interp.maxHostResult })
	std.RegisterTable("string", strLib)
	interp.SetStringLibrary(strLib)

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
