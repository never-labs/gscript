package debug

import "github.com/never-labs/leia/internal/support"

// Frame describes one active runtime call in Leia-shaped terms.
type Frame = support.Frame

// HookOptions describes the coarse-grained Leia debug hook filters.
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
