package install

import (
	"context"

	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/stdlibrt/modules"
)

// Install registers the standard library on interp.
func Install(interp *runtime.Interpreter) {
	if interp == nil {
		return
	}
	interp.InstallRuntimeStdlib()
	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes, ModuleOptions{
		ScriptCaller: interp.CallFunction,
		Host: modules.HostOptions{
			NetworkAllowed:        interp.NetworkAccessEnabled,
			FilesystemRoot:        interp.FilesystemRoot,
			FilesystemRead:        interp.FilesystemReadEnabled,
			FilesystemWrite:       interp.FilesystemWriteEnabled,
			MaxFSReadBytes:        interp.MaxFilesystemReadBytes,
			MaxFSWriteBytes:       interp.MaxFilesystemWriteBytes,
			EnvironmentRead:       interp.EnvironmentReadEnabled,
			EnvironmentWrite:      interp.EnvironmentWriteEnabled,
			EnvironmentAllowed:    interp.EnvironmentAllowed,
			ProcessExecution:      interp.ProcessExecutionEnabled,
			ProcessShell:          interp.ProcessShellEnabled,
			ResolveFilesystemPath: interp.ResolveFilesystemPath,
			Args:                  interp.Args,
			SetArgs:               interp.SetArgs,
			ScriptDir:             interp.ScriptDir,
			MaxHostResult:         interp.MaxHostResultBytes,
			Call:                  interp.CallFunction,
		},
	})
	InstallLLM(interp)
}

type ModuleOptions struct {
	ScriptCaller runtime.ScriptFunctionCaller
	Less         modules.ValueLessFunc
	Host         modules.HostOptions
}

// InstallModules registers stdlibrt-owned modules on a runtime-compatible
// installer. VM and tree-walker entry points use this to avoid separate module
// construction paths while the broader stdlib continues to migrate.
func InstallModules(installer runtime.StdlibInstaller, maxHostResult func() int64, options ...ModuleOptions) {
	if installer == nil {
		return
	}
	if maxHostResult == nil {
		maxHostResult = func() int64 { return 0 }
	}
	var opts ModuleOptions
	if len(options) > 0 {
		opts = options[0]
	}
	hostOpts := opts.Host
	if hostOpts.MaxHostResult == nil {
		hostOpts.MaxHostResult = maxHostResult
	}
	if hostOpts.Call == nil {
		hostOpts.Call = opts.ScriptCaller
	}
	installer.RegisterTable("array", modules.BuildArray())
	installer.RegisterTable("base64", modules.BuildBase64(maxHostResult))
	installer.RegisterTable("binary", modules.BuildBinary(maxHostResult))
	installer.RegisterTable("bits", modules.BuildBits())
	installer.RegisterTable("bit32", modules.BuildBit32())
	installer.RegisterTable("bytes", modules.BuildBytes(maxHostResult))
	installer.RegisterTable("color", modules.BuildColor())
	installer.RegisterTable("compress", modules.BuildCompress(maxHostResult))
	installer.RegisterTable("container", modules.BuildContainer())
	installer.RegisterTable("context", modules.BuildContext())
	installer.RegisterTable("crypto", modules.BuildCrypto(maxHostResult))
	installer.RegisterTable("csv", modules.BuildCSV(maxHostResult))
	installer.RegisterTable("encoding", modules.BuildEncoding(maxHostResult))
	if !hostOpts.SkipHostIO {
		installer.RegisterTable("fs", modules.BuildFSWithPolicy(hostOpts))
		installer.RegisterTable("http", modules.BuildHTTP(hostOpts))
		installer.RegisterTable("io", modules.BuildIO(hostOpts))
		installer.RegisterTable("net", modules.BuildNet(hostOpts))
		installer.RegisterTable("os", modules.BuildOSWithPolicy(hostOpts))
		installer.RegisterTable("process", modules.BuildProcessWithPolicy(hostOpts))
	}
	installer.RegisterTable("hash", modules.BuildHash())
	installer.RegisterTable("json", modules.BuildJSON())
	installer.RegisterTable("log", modules.BuildLog())
	installer.RegisterTable("matrix", modules.BuildMatrix())
	installer.RegisterTable("math", modules.BuildMath())
	installer.RegisterTable("path", modules.BuildPath())
	installer.RegisterTable("rand", modules.BuildRand())
	installer.RegisterTable("regexp", modules.BuildRegexp())
	installer.RegisterTable("sort", modules.BuildSortLibWithCallerAndLess(opts.ScriptCaller, opts.Less))
	installer.RegisterTable("time", modules.BuildTime())
	installer.RegisterTable("url", modules.BuildURL(maxHostResult))
	installer.RegisterTable("utf8", modules.BuildUTF8(maxHostResult))
	installer.RegisterTable("uuid", modules.BuildUUID())
	installer.RegisterTable("vec", modules.BuildVec())
}

func InstallLLM(interp *runtime.Interpreter) {
	if interp == nil {
		return
	}
	modules.InstallLLM(interpreterInstaller{interp: interp}, modules.LLMOptions{
		Call: interp.CallFunction,
		Provider: func() runtime.LLMProvider {
			return interp.LLMProvider()
		},
		ProviderFactory: func() runtime.LLMProviderFactory {
			return interp.LLMProviderFactory()
		},
		MaxHostResult: interp.MaxHostResultBytes,
		Context: func() context.Context {
			if ctx := interp.Context(); ctx != nil {
				return ctx
			}
			return context.Background()
		},
		Trace: func(event runtime.LLMTraceEvent) {
			if sink := interp.LLMTraceSink(); sink != nil {
				sink(event)
			}
		},
	})
}

type interpreterInstaller struct {
	interp *runtime.Interpreter
}

func (installer interpreterInstaller) RegisterModule(name string, module runtime.Value) {
	if installer.interp == nil {
		return
	}
	installer.interp.SetGlobal(name, module)
	installer.interp.SetModule(name, module)
}

func (installer interpreterInstaller) RegisterTable(name string, table *runtime.Table) {
	installer.RegisterModule(name, runtime.TableValue(table))
}

func (installer interpreterInstaller) RegisterAlias(name string, value runtime.Value) {
	if installer.interp == nil {
		return
	}
	installer.interp.SetGlobal(name, value)
}
