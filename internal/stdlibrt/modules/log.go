package modules

import (
	"fmt"
	"strings"
	"time"
)

const (
	logLevelDebug = 0
	logLevelInfo  = 1
	logLevelWarn  = 2
	logLevelError = 3
	logLevelFatal = 4
)

// BuildLog creates the "log" standard library table.
// Provides structured logging with levels, formatting, and output capture.
func BuildLog() *Table {
	t := NewTable()

	currentLevel := logLevelInfo
	var logOutput []string
	prefix := ""
	showTimestamp := true
	showLevel := true

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "log." + name,
			Fn:   fn,
		}))
	}

	t.RawSet(StringValue("DEBUG"), IntValue(logLevelDebug))
	t.RawSet(StringValue("INFO"), IntValue(logLevelInfo))
	t.RawSet(StringValue("WARN"), IntValue(logLevelWarn))
	t.RawSet(StringValue("ERROR"), IntValue(logLevelError))
	t.RawSet(StringValue("FATAL"), IntValue(logLevelFatal))

	levelName := func(level int) string {
		switch level {
		case logLevelDebug:
			return "DEBUG"
		case logLevelInfo:
			return "INFO"
		case logLevelWarn:
			return "WARN"
		case logLevelError:
			return "ERROR"
		case logLevelFatal:
			return "FATAL"
		default:
			return "UNKNOWN"
		}
	}

	formatMsg := func(level int, args []Value) string {
		var parts []string
		if showTimestamp {
			parts = append(parts, time.Now().Format("2006-01-02 15:04:05"))
		}
		if showLevel {
			parts = append(parts, fmt.Sprintf("[%s]", levelName(level)))
		}
		if prefix != "" {
			parts = append(parts, prefix)
		}
		var msgParts []string
		for _, a := range args {
			msgParts = append(msgParts, a.String())
		}
		parts = append(parts, strings.Join(msgParts, " "))
		return strings.Join(parts, " ")
	}

	doLog := func(level int, args []Value) {
		if level < currentLevel {
			return
		}
		msg := formatMsg(level, args)
		logOutput = append(logOutput, msg)
		fmt.Println(msg)
	}

	set("setLevel", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'log.setLevel' (number expected)")
		}
		currentLevel = int(toInt(args[0]))
		return nil, nil
	})
	set("getLevel", func(args []Value) ([]Value, error) {
		return []Value{IntValue(int64(currentLevel))}, nil
	})
	set("setPrefix", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'log.setPrefix' (string expected)")
		}
		prefix = args[0].Str()
		return nil, nil
	})
	set("setTimestamp", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'log.setTimestamp' (boolean expected)")
		}
		showTimestamp = args[0].Truthy()
		return nil, nil
	})
	set("setShowLevel", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'log.setShowLevel' (boolean expected)")
		}
		showLevel = args[0].Truthy()
		return nil, nil
	})
	set("debug", func(args []Value) ([]Value, error) {
		doLog(logLevelDebug, args)
		return nil, nil
	})
	set("info", func(args []Value) ([]Value, error) {
		doLog(logLevelInfo, args)
		return nil, nil
	})
	set("warn", func(args []Value) ([]Value, error) {
		doLog(logLevelWarn, args)
		return nil, nil
	})
	set("error", func(args []Value) ([]Value, error) {
		doLog(logLevelError, args)
		return nil, nil
	})
	set("fatal", func(args []Value) ([]Value, error) {
		doLog(logLevelFatal, args)
		return nil, nil
	})
	set("history", func(args []Value) ([]Value, error) {
		result := NewTable()
		for i, msg := range logOutput {
			result.RawSet(IntValue(int64(i+1)), StringValue(msg))
		}
		return []Value{TableValue(result)}, nil
	})
	set("clear", func(args []Value) ([]Value, error) {
		logOutput = nil
		return nil, nil
	})
	set("count", func(args []Value) ([]Value, error) {
		return []Value{IntValue(int64(len(logOutput)))}, nil
	})
	set("format", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'log.format' (level expected)")
		}
		level := int(toInt(args[0]))
		msg := formatMsg(level, args[1:])
		return []Value{StringValue(msg)}, nil
	})

	return t
}
