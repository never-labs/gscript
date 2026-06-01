package bind

import "github.com/never-labs/leia/internal/runtime"

type HostOptions struct {
	SkipHostIO            bool
	NetworkAllowed        func() bool
	FilesystemRoot        func() string
	FilesystemRead        func() bool
	FilesystemWrite       func() bool
	MaxFSReadBytes        func() int64
	MaxFSWriteBytes       func() int64
	EnvironmentRead       func() bool
	EnvironmentWrite      func() bool
	EnvironmentAllowed    func(string) bool
	ProcessExecution      func() bool
	ProcessShell          func() bool
	ResolveFilesystemPath func(string) (string, error)
	Args                  func() []string
	SetArgs               func(string, []string)
	ScriptDir             func() string
	MaxHostResult         func() int64
	Call                  runtime.ScriptFunctionCaller
}

func HostBool(fn func() bool, fallback bool) bool {
	if fn == nil {
		return fallback
	}
	return fn()
}

func HostString(fn func() string) string {
	if fn == nil {
		return ""
	}
	return fn()
}
