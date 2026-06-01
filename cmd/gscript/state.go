package main

import (
	"io"
	"os"
	"sort"

	gscript "github.com/never-labs/gscript"
	bytecodevm "github.com/never-labs/gscript/internal/vm"
)

// jitStatsReporter is implemented by the platform-specific JIT engine wrapper
// so the CLI can print tier statistics after execution.
type jitStatsReporter interface {
	PrintStats(w *os.File)
	PrintExitStats(w *os.File)
	PrintExitStatsJSON(w *os.File) error
	PrintTier2PerfStats(w *os.File)
	PrintTier2PerfStatsJSON(w *os.File) error
	PrintTier2SpeculationStateJSON(w *os.File) error
	PrintTier2SpeculationWorklistJSON(w *os.File) error
	Close() error
}

type jitCLIOptions struct {
	TimelinePath              string
	TimelineFormat            string
	WarmDumpDir               string
	WarmDumpProto             string
	ShowExitStats             bool
	ShowExitStatsJSON         bool
	ShowTier2PerfStats        bool
	ShowTier2PerfStatsJSON    bool
	ShowTier2SpecStateJSON    bool
	ShowTier2SpecWorklistJSON bool
	ShowCoroutineStats        bool
	ShowPathStats             bool
	ShowPathStatsJSON         bool
}

type jitWarmDumpController interface {
	EnableWarmDump(dir, protoName string) error
	WriteWarmDump(top *bytecodevm.FuncProto) error
}

type cliRunOptions struct {
	UseVM        bool
	UseJIT       bool
	ShowJITStats bool
	ModuleMode   gscript.ModuleMode
	JIT          jitCLIOptions
}

var cliStdin io.Reader = os.Stdin

// sortStrings is a tiny helper shared with platform files to keep them from
// each importing "sort".
func sortStrings(s []string) { sort.Strings(s) }
