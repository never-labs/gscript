package debug

import "github.com/never-labs/gscript/internal/support"

// Frame describes one active runtime call in GScript-shaped terms.
type Frame = support.Frame

// HookOptions describes the coarse-grained GScript debug hook filters.
type HookOptions = support.HookOptions

func DefaultHookOptions() HookOptions {
	return support.DefaultHookOptions()
}

func HookWants(opts HookOptions, eventType, kind string) bool {
	return support.HookWants(opts, eventType, kind)
}

func FormatTraceback(message string, frames []Frame) string {
	return support.FormatTraceback(message, frames)
}
