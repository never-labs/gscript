package runtime

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/never-labs/leia/internal/ast"
)

// LuaError represents an error raised by the error() built-in or other
// Leia runtime mechanisms. It wraps a Value so that error objects are
// not limited to strings.
type LuaError struct {
	Value Value // the error value (can be any type)
}

func (e *LuaError) Error() string {
	return e.Value.String()
}

// SourceError attaches stable source coordinates to lexer, parser, and runtime
// errors without changing the underlying error value.
type SourceError struct {
	SourceName string
	Line       int
	Column     int
	Err        error
}

func (e *SourceError) Error() string {
	if e.SourceName != "" && e.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: %v", e.SourceName, e.Line, e.Column, e.Err)
	}
	if e.SourceName != "" {
		return fmt.Sprintf("%s: %v", e.SourceName, e.Err)
	}
	if e.Line > 0 {
		return fmt.Sprintf("%d:%d: %v", e.Line, e.Column, e.Err)
	}
	return e.Err.Error()
}

func (e *SourceError) Unwrap() error {
	return e.Err
}

func (interp *Interpreter) wrapRuntimeError(err error, pos ast.Pos) error {
	if err == nil || sourceErrorOf(err) != nil {
		return err
	}
	if interp.currentSourceName == "" && pos.Line == 0 {
		return err
	}
	return &SourceError{
		SourceName: interp.currentSourceName,
		Line:       pos.Line,
		Column:     pos.Column,
		Err:        err,
	}
}

func wrapCompileError(err error, sourceName string) error {
	if err == nil || sourceName == "" || sourceErrorOf(err) != nil {
		return err
	}
	line, col := extractDiagnosticPosition(err.Error())
	return &SourceError{
		SourceName: sourceName,
		Line:       line,
		Column:     col,
		Err:        err,
	}
}

func sourceErrorOf(err error) *SourceError {
	for err != nil {
		if se, ok := err.(*SourceError); ok {
			return se
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil
		}
		err = u.Unwrap()
	}
	return nil
}

var diagnosticPositionRE = regexp.MustCompile(`(?:at|starting at) ([0-9]+):([0-9]+)`)

func extractDiagnosticPosition(msg string) (int, int) {
	matches := diagnosticPositionRE.FindStringSubmatch(msg)
	if len(matches) != 3 {
		return 0, 0
	}
	line, _ := strconv.Atoi(matches[1])
	col, _ := strconv.Atoi(matches[2])
	return line, col
}
