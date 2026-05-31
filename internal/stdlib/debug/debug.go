package debug

import (
	"fmt"
	"strings"
)

// Frame describes one active runtime call in GScript-shaped terms.
type Frame struct {
	Name       string
	Kind       string
	SourceName string
	Line       int
	Column     int
}

// HookOptions describes the coarse-grained GScript debug hook filters.
type HookOptions struct {
	Call   bool
	Return bool
	Error  bool
	Emit   bool
	Script bool
	Native bool
}

func DefaultHookOptions() HookOptions {
	return HookOptions{
		Call:   true,
		Return: true,
		Error:  true,
		Emit:   true,
		Script: true,
		Native: true,
	}
}

func HookWants(opts HookOptions, eventType, kind string) bool {
	switch eventType {
	case "call":
		if !opts.Call {
			return false
		}
	case "return":
		if !opts.Return {
			return false
		}
	case "error":
		if !opts.Error {
			return false
		}
	case "emit":
		if !opts.Emit {
			return false
		}
	default:
		return false
	}
	switch kind {
	case "script":
		return opts.Script
	case "native":
		return opts.Native
	default:
		return true
	}
}

func FormatTraceback(message string, frames []Frame) string {
	var b strings.Builder
	if message != "" {
		b.WriteString(message)
		b.WriteByte('\n')
	}
	b.WriteString("stack traceback:")
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		b.WriteString("\n  ")
		b.WriteString(frame.Kind)
		b.WriteByte(' ')
		b.WriteString(frame.Name)
		if frame.SourceName != "" && frame.Line > 0 {
			b.WriteString(" @ ")
			b.WriteString(frame.SourceName)
			b.WriteString(":")
			b.WriteString(fmt.Sprintf("%d:%d", frame.Line, frame.Column))
		}
	}
	return b.String()
}
