package modules

type HostOptions struct {
	SkipHostIO         bool
	NetworkAllowed     func() bool
	FilesystemRoot     func() string
	FilesystemRead     func() bool
	FilesystemWrite    func() bool
	MaxFSReadBytes     func() int64
	MaxFSWriteBytes    func() int64
	EnvironmentRead    func() bool
	EnvironmentWrite   func() bool
	EnvironmentAllowed func(string) bool
	MaxHostResult      func() int64
	Call               ScriptFunctionCaller
}

func hostBool(fn func() bool, fallback bool) bool {
	if fn == nil {
		return fallback
	}
	return fn()
}

func hostString(fn func() string) string {
	if fn == nil {
		return ""
	}
	return fn()
}

func markStdlibrtModule(t *Table) *Table {
	if t != nil {
		t.RawSetString("__stdlibrt_module", BoolValue(true))
	}
	return t
}

func IsStdlibrtModule(t *Table) bool {
	return t != nil && t.RawGetString("__stdlibrt_module").Truthy()
}
