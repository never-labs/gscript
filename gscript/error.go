package gscript

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/gscript/gscript/internal/runtime"
)

// ErrorKind identifies the phase of GScript execution that produced an error.
type ErrorKind string

const (
	ErrLex     ErrorKind = "lex"
	ErrParse   ErrorKind = "parse"
	ErrRuntime ErrorKind = "runtime"
	ErrScript  ErrorKind = "script" // error() called from GScript
)

// Error is a structured error from GScript execution.
type Error struct {
	Kind    ErrorKind
	Message string
	Line    int
	Col     int
	File    string
	// Err holds the underlying cause, when one is available.
	Err error
	// Value holds the original GScript error value when Kind == ErrScript.
	// It may be a string, table, or any GScript value converted to interface{}.
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

	return out
}

var stepBudgetErrorRE = regexp.MustCompile(`execution step limit exceeded \(([0-9]+)\)`)

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
	if len(matches) != 2 {
		return nil
	}
	limit, _ := strconv.ParseInt(matches[1], 10, 64)
	return &BudgetError{Resource: "steps", Limit: limit, Err: err}
}
