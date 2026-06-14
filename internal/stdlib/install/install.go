package install

import (
	"context"

	"github.com/never-labs/leia/internal/runtime"
	stdbind "github.com/never-labs/leia/internal/stdlib/bind"
)

// Install registers the standard library on interp.
func Install(interp *runtime.Interpreter) {
	InstallWithOptions(interp, ModuleOptions{})
}

// InstallWithOptions registers the standard library on interp with additional
// module options used by embedders.
func InstallWithOptions(interp *runtime.Interpreter, extra ModuleOptions) {
	if interp == nil {
		return
	}
	interp.InstallRuntimeStdlib()
	opts := ModuleOptions{
		ScriptCaller: interp.CallFunction,
		TaskLauncher: interp.LaunchFunction,
		Host: stdbind.HostOptions{
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
			Global:                interp.GetGlobal,
		},
		Table: stdbind.TableOptions{
			Call: interp.CallFunction,
			Less: interp.ValueLessThan,
			Len:  interp.TableLen,
			Get:  interp.TableGet,
			Set:  interp.TableSet,
		},
	}
	opts.Dialects = append(opts.Dialects, extra.Dialects...)
	InstallModules(interpreterInstaller{interp: interp}, interp.MaxHostResultBytes, opts)
	InstallDebugAndTestkit(interp)
	InstallLLM(interp)
}

type ModuleOptions struct {
	ScriptCaller runtime.ScriptFunctionCaller
	TaskLauncher stdbind.TaskLauncher
	Less         stdbind.Less
	Host         stdbind.HostOptions
	Table        stdbind.TableOptions
	Dialects     []stdbind.DialectSpec
	SkipTable    bool
}

// InstallModules registers stdlib-bound modules on a stdlib-compatible
// installer. VM and tree-walker entry points use this shared module plan so
// the public standard library has one registration path.
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
	installer.RegisterTable("array", stdbind.BuildArray())
	installer.RegisterTable("base64", stdbind.BuildBase64(maxHostResult))
	installer.RegisterTable("binary", stdbind.BuildBinary(maxHostResult))
	installer.RegisterTable("bits", stdbind.BuildBits())
	installer.RegisterTable("bit32", stdbind.BuildBit32())
	installer.RegisterTable("bytes", stdbind.BuildBytes(maxHostResult))
	installer.RegisterTable("color", stdbind.BuildColor())
	installer.RegisterTable("compress", stdbind.BuildCompress(maxHostResult))
	installer.RegisterTable("container", stdbind.BuildContainer())
	installer.RegisterTable("context", stdbind.BuildContext())
	installer.RegisterTable("control", stdbind.BuildControl())
	installer.RegisterTable("crypto", stdbind.BuildCrypto(maxHostResult))
	installer.RegisterTable("csv", stdbind.BuildCSV(maxHostResult))
	installer.RegisterTable("data", buildDataModule())
	installer.RegisterTable("db", stdbind.BuildDB(hostOpts))
	installer.RegisterTable("dialect", stdbind.BuildDialect(hostOpts, maxHostResult, opts.Dialects...))
	installer.RegisterTable("encoding", stdbind.BuildEncoding(maxHostResult))
	if !hostOpts.SkipHostIO {
		installer.RegisterTable("fs", stdbind.BuildFSWithPolicy(hostOpts))
		installer.RegisterTable("http", stdbind.BuildHTTP(hostOpts))
		installer.RegisterTable("io", stdbind.BuildIO(hostOpts))
		installer.RegisterTable("net", stdbind.BuildNet(hostOpts))
		installer.RegisterTable("os", stdbind.BuildOSWithPolicy(hostOpts))
		installer.RegisterTable("process", stdbind.BuildProcessWithPolicy(hostOpts))
		installer.RegisterTable("serve", stdbind.BuildServe(hostOpts))
	}
	installer.RegisterTable("hash", stdbind.BuildHash())
	installer.RegisterTable("json", stdbind.BuildJSON())
	installer.RegisterTable("linalg", stdbind.BuildLinalg())
	installer.RegisterTable("log", stdbind.BuildLog())
	installer.RegisterTable("matrix", stdbind.BuildMatrix())
	installer.RegisterTable("math", stdbind.BuildMath())
	installer.RegisterTable("ode", stdbind.BuildODE(opts.ScriptCaller))
	installer.RegisterTable("path", stdbind.BuildPath())
	installer.RegisterTable("q", stdbind.BuildQ())
	installer.RegisterTable("rand", stdbind.BuildRand())
	installer.RegisterTable("regexp", stdbind.BuildRegexp())
	installer.RegisterTable("sort", stdbind.BuildSortLibWithCallerAndLess(opts.ScriptCaller, opts.Less))
	installer.RegisterTable("soa", stdbind.BuildSOA())
	installer.RegisterTable("stats", stdbind.BuildStats())
	installer.RegisterTable("sync", stdbind.BuildSync(stdbind.ConcurrencyOptions{
		Call:   opts.ScriptCaller,
		Launch: opts.TaskLauncher,
	}))
	installer.RegisterTable("string", stdbind.BuildString(opts.ScriptCaller, maxHostResult))
	if !opts.SkipTable {
		installer.RegisterTable("table", stdbind.BuildTable(opts.Table))
	}
	installer.RegisterTable("time", stdbind.BuildTime())
	installer.RegisterTable("url", stdbind.BuildURL(maxHostResult))
	installer.RegisterTable("utf8", stdbind.BuildUTF8(maxHostResult))
	installer.RegisterTable("uuid", stdbind.BuildUUID())
	installer.RegisterTable("vec", stdbind.BuildVec())
}

func buildDataModule() *runtime.Table {
	return stdbind.BuildData()
}

func InstallLLM(interp *runtime.Interpreter) {
	if interp == nil {
		return
	}
	stdbind.InstallLLM(interpreterInstaller{interp: interp}, stdbind.LLMOptions{
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
		EnvironmentRead: interp.EnvironmentReadEnabled,
	})
}

func InstallDebugAndTestkit(interp *runtime.Interpreter) {
	if interp == nil {
		return
	}
	installer := interpreterInstaller{interp: interp}
	installer.RegisterTable("debug", stdbind.BuildDebug(interp))
	installer.RegisterTable("testkit", stdbind.BuildTestkit(stdbind.TestkitOptions{
		Runtime: interp,
		Call:    interp.CallFunction,
	}))
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
	installer.interp.MarkStdlibModule(name)
}

func (installer interpreterInstaller) RegisterTable(name string, table *runtime.Table) {
	if name == "string" && installer.interp != nil {
		installer.interp.SetStringLibrary(table)
	}
	installer.RegisterModule(name, runtime.TableValue(table))
}

func (installer interpreterInstaller) RegisterAlias(name string, value runtime.Value) {
	if installer.interp == nil {
		return
	}
	installer.interp.SetGlobal(name, value)
}
