package debug

import "github.com/never-labs/gscript/internal/support/debugstate"

// Frame describes one active runtime call in GScript-shaped terms.
type Frame = debugstate.Frame

// HookOptions describes the coarse-grained GScript debug hook filters.
type HookOptions = debugstate.HookOptions

func DefaultHookOptions() HookOptions {
	return debugstate.DefaultHookOptions()
}

func HookWants(opts HookOptions, eventType, kind string) bool {
	return debugstate.HookWants(opts, eventType, kind)
}

func FormatTraceback(message string, frames []Frame) string {
	return debugstate.FormatTraceback(message, frames)
}
