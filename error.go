package leia

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/never-labs/leia/internal/runtime"
)

// ErrorKind identifies the phase of Leia execution that produced an error.
type ErrorKind string

const (
	ErrLex     ErrorKind = "lex"
	ErrParse   ErrorKind = "parse"
	ErrRuntime ErrorKind = "runtime"
	ErrScript  ErrorKind = "script" // error() called from Leia
)

// Error is a structured error from Leia execution.
type Error struct {
	Kind    ErrorKind
	Message string
	Line    int
	Col     int
	File    string
	// Err holds the underlying cause, when one is available.
	Err error
	// Value holds the original Leia error value when Kind == ErrScript.
	// It may be a string, table, or any Leia value converted to interface{}.
	Value interface{}
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Line > 0 {
		return fmt.Sprintf("[%s] %s:%d: %s", e.Kind, e.File, e.Line, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Kind, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *Error) Is(target error) bool {
	if e == nil {
		return target == nil
	}
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return t.Kind == "" || e.Kind == t.Kind
}

func wrapError(kind ErrorKind, err error) *Error {
	return newError(kind, err, "")
}

func newError(kind ErrorKind, err error, file string) *Error {
	if err == nil {
		return &Error{Kind: kind, File: file}
	}
	return &Error{Kind: kind, Message: err.Error(), File: file, Err: err}
}

// HostCallbackError is returned through execution APIs when a registered Go
// callback returns a non-nil error.
type HostCallbackError struct {
	Name string
	Err  error
}

func (e *HostCallbackError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == nil {
		if e.Name == "" {
			return "host function returned error"
		}
		return fmt.Sprintf("host function %s returned error", e.Name)
	}
	if e.Name == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("host function %s returned error: %v", e.Name, e.Err)
}

func (e *HostCallbackError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// HostCallbackPanicError is returned through execution APIs when a registered
// Go callback panics. The panic is recovered and exposed without crashing the
// host process.
type HostCallbackPanicError struct {
	Name  string
	Value interface{}
}

func (e *HostCallbackPanicError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Name == "" {
		return fmt.Sprintf("host function panicked: %v", e.Value)
	}
	return fmt.Sprintf("host function %s panicked: %v", e.Name, e.Value)
}

// ExitError reports a script-requested process exit. Embedders can catch this
// error and decide whether to terminate the host process, map it to an HTTP
// status, or treat it as an ordinary script result.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("script exit %d", e.Code)
}

func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// BudgetError reports exhaustion of a resource budget configured on a VM.
type BudgetError struct {
	Resource string
	Limit    int64
	Err      error
}

func (e *BudgetError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Limit > 0 {
		return fmt.Sprintf("%s budget exceeded (%d)", e.Resource, e.Limit)
	}
	return fmt.Sprintf("%s budget exceeded", e.Resource)
}

func (e *BudgetError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func runtimeError(err error, file string) *Error {
	if err == nil {
		return &Error{Kind: ErrRuntime, File: file}
	}

	out := &Error{Kind: ErrRuntime, Message: err.Error(), File: file, Err: err}

	var sourceErr *runtime.SourceError
	if errors.As(err, &sourceErr) {
		if sourceErr.SourceName != "" {
			out.File = sourceErr.SourceName
		}
		out.Line = sourceErr.Line
		out.Col = sourceErr.Column
		if sourceErr.Err != nil {
			out.Message = sourceErr.Err.Error()
		}
	}

	var luaErr *runtime.LuaError
	if errors.As(err, &luaErr) {
		out.Kind = ErrScript
		out.Message = luaErr.Error()
		if value, convErr := fromValueDefault(luaErr.Value); convErr == nil && value.IsValid() {
			out.Value = value.Interface()
		}
	}

	if budgetErr := budgetErrorFrom(err); budgetErr != nil {
		out.Err = budgetErr
	}
	if exitErr := exitErrorFrom(err); exitErr != nil {
		out.Err = exitErr
	}

	return out
}

var (
	stepBudgetErrorRE       = regexp.MustCompile(`execution step limit exceeded \(([0-9]+)\)`)
	nativeCallBudgetErrorRE = regexp.MustCompile(`native call limit exceeded \(([0-9]+)\)`)
	callDepthBudgetErrorRE  = regexp.MustCompile(`call depth limit exceeded \(([0-9]+)\)`)
	goroutineBudgetErrorRE  = regexp.MustCompile(`goroutine limit exceeded \(([0-9]+)\)`)
	channelCapBudgetErrorRE = regexp.MustCompile(`channel capacity limit exceeded \(([0-9]+)\)`)
	hostResultBudgetErrorRE = regexp.MustCompile(`host result byte limit exceeded \(([0-9]+)\)`)
	moduleBytesBudgetRE     = regexp.MustCompile(`module byte limit exceeded \(([0-9]+)\)`)
	moduleDepthBudgetRE     = regexp.MustCompile(`module depth limit exceeded \(([0-9]+)\)`)
)

func budgetErrorFrom(err error) *BudgetError {
	if err == nil {
		return nil
	}
	var budgetErr *BudgetError
	if errors.As(err, &budgetErr) {
		return budgetErr
	}
	msg := err.Error()
	matches := stepBudgetErrorRE.FindStringSubmatch(msg)
	if len(matches) == 2 {
		limit, _ := strconv.ParseInt(matches[1], 10, 64)
		return &BudgetError{Resource: "steps", Limit: limit, Err: err}
	}
	matches = nativeCallBudgetErrorRE.FindStringSubmatch(msg)
	if len(matches) == 2 {
		limit, _ := strconv.ParseInt(matches[1], 10, 64)
		return &BudgetError{Resource: "native_calls", Limit: limit, Err: err}
	}
	matches = callDepthBudgetErrorRE.FindStringSubmatch(msg)
	if len(matches) == 2 {
		limit, _ := strconv.ParseInt(matches[1], 10, 64)
		return &BudgetError{Resource: "call_depth", Limit: limit, Err: err}
	}
	matches = goroutineBudgetErrorRE.FindStringSubmatch(msg)
	if len(matches) == 2 {
		limit, _ := strconv.ParseInt(matches[1], 10, 64)
		return &BudgetError{Resource: "goroutines", Limit: limit, Err: err}
	}
	matches = channelCapBudgetErrorRE.FindStringSubmatch(msg)
	if len(matches) == 2 {
		limit, _ := strconv.ParseInt(matches[1], 10, 64)
		return &BudgetError{Resource: "channel_capacity", Limit: limit, Err: err}
	}
	matches = hostResultBudgetErrorRE.FindStringSubmatch(msg)
	if len(matches) == 2 {
		limit, _ := strconv.ParseInt(matches[1], 10, 64)
		return &BudgetError{Resource: "host_result_bytes", Limit: limit, Err: err}
	}
	matches = moduleBytesBudgetRE.FindStringSubmatch(msg)
	if len(matches) == 2 {
		limit, _ := strconv.ParseInt(matches[1], 10, 64)
		return &BudgetError{Resource: "module_bytes", Limit: limit, Err: err}
	}
	matches = moduleDepthBudgetRE.FindStringSubmatch(msg)
	if len(matches) == 2 {
		limit, _ := strconv.ParseInt(matches[1], 10, 64)
		return &BudgetError{Resource: "module_depth", Limit: limit, Err: err}
	}
	return nil
}

func exitErrorFrom(err error) *ExitError {
	if err == nil {
		return nil
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr
	}
	var runtimeExit *runtime.ProcessExitError
	if errors.As(err, &runtimeExit) {
		return &ExitError{Code: runtimeExit.Code, Err: err}
	}
	return nil
}
