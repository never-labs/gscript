package q

// q error trap (@[f;x;e] / .[f;args;e]) and signal ('x).
//
// Disambiguation rule for the 3-argument bracket forms, mirroring q: the
// FIRST argument's runtime value decides the form.
//
//	@[first;second;third] with CALLABLE first -> trap: apply monadic first
//	    to second; on error apply third to the error TEXT when third is
//	    callable, else return third verbatim as the result.
//	@[first;second;third] with DATA first     -> amend-with-unary-op: apply
//	    unary third to the items of first selected by index second
//	    (q's @[v;i;f]).
//	.[first;second;third] follows the same rule: callable first traps a
//	    multi-argument apply (second is the argument list), data first is a
//	    deep amend applying unary third at index path second.
//
// Trap catches q-level evaluation errors only: every failure the evaluator
// returns as a Go error (the user-visible error-message taxonomy, including
// errors raised by the signal form). The handler receives err.Error() — the
// exact text an untrapped failure would print. Go panics are deliberately
// NOT recovered by trap: a residual panic inside a kernel stays a panic and
// surfaces to the host, never silently converted into a q error. Errors
// raised while evaluating f, x, or e themselves are NOT trapped (they happen
// before protected execution begins), matching q.
//
// Signal scope (deliberately minimal): an expression that BEGINS with '
// followed by a string literal ('"text"), a single symbol literal ('`name),
// or a bound name ('x) raises a q error. A bound name must hold a string or
// symbol; any other value reports "signal expects a string or symbol".
// Unbound or structured operands are NOT claimed, so their pre-existing
// parse errors are unchanged. q's general `'expr` signal and the leading
// apostrophe display convention are out of scope.
//
// Nesting: trap composes through ordinary Go error returns, so a trap inside
// a trapped function observes inner errors first and a signal crosses any
// number of un-handling frames to the nearest trap. Leia-level pcall around
// q.eval sits OUTSIDE all of this: an untrapped q error still surfaces to
// pcall unchanged.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// qSignalError is the error raised by the signal form. Its message is
// exactly the signaled text, so a trap handler receives what q would pass.
type qSignalError struct{ text string }

func (e qSignalError) Error() string { return e.text }

// evalSignalForm recognizes the minimal signal forms described above.
// handled=false leaves src on the existing evaluator cascade so statements
// the signal scope does not claim keep their pre-existing errors.
func (s *EvalState) evalSignalForm(src string) (bool, error) {
	if len(src) < 2 || src[0] != '\'' {
		return false, nil
	}
	rest := strings.TrimSpace(src[1:])
	if rest == "" {
		return false, nil
	}
	if len(rest) >= 2 && rest[0] == '"' && rest[len(rest)-1] == '"' {
		if text, err := strconv.Unquote(rest); err == nil {
			return true, qSignalError{text: text}
		}
		return false, nil
	}
	if rest[0] == '`' {
		if syms, err := parseSymbolList(rest); err == nil && len(syms) == 1 {
			return true, qSignalError{text: string(syms[0])}
		}
		return false, nil
	}
	if isQAssignmentName(rest) {
		value, ok := s.lookupName(rest)
		if !ok {
			return false, nil
		}
		switch x := value.(type) {
		case string:
			return true, qSignalError{text: x}
		case data.Symbol:
			return true, qSignalError{text: string(x)}
		default:
			return true, qSignalError{text: "signal expects a string or symbol"}
		}
	}
	return false, nil
}

// qTrapResult resolves a protected apply: no error passes the result
// through; on error a callable handler receives the error text, a data
// handler is itself the result.
func (s *EvalState) qTrapResult(out any, err error, handler any) (any, error) {
	if err == nil {
		return out, nil
	}
	if isCallable(handler) {
		return s.applyCallable1(handler, err.Error())
	}
	return handler, nil
}

// evalAtTripleForm is the string-route 3-argument @ form: trap when the
// first argument is callable, amend-with-unary-op when it is data. All three
// operands evaluate (unprotected) left to right before dispatch.
func (s *EvalState) evalAtTripleForm(parts []string) (any, error) {
	first, err := s.eval(parts[0])
	if err != nil {
		return nil, err
	}
	second, err := s.eval(parts[1])
	if err != nil {
		return nil, err
	}
	third, err := s.eval(parts[2])
	if err != nil {
		return nil, err
	}
	if isCallable(first) {
		out, err := s.applyCallable1(first, second)
		return s.qTrapResult(out, err, third)
	}
	return s.amendValueWithUnaryFunction(first, second, third)
}

// evalDotTripleForm is the string-route 3-argument . form: trap of a
// multi-argument apply when the first argument is callable, deep
// amend-with-unary-op when it is data.
func (s *EvalState) evalDotTripleForm(parts []string) (any, error) {
	first, err := s.eval(parts[0])
	if err != nil {
		return nil, err
	}
	if isCallable(first) {
		args, err := s.evalDotApplyArgs(parts[1])
		if err != nil {
			return nil, err
		}
		third, err := s.eval(parts[2])
		if err != nil {
			return nil, err
		}
		out, err := s.applyCallable(first, args)
		return s.qTrapResult(out, err, third)
	}
	pathValue, err := s.eval(parts[1])
	if err != nil {
		return nil, err
	}
	pathItems, err := amendPathItems(pathValue)
	if err != nil {
		return nil, err
	}
	third, err := s.eval(parts[2])
	if err != nil {
		return nil, err
	}
	return s.amendPathUnary(first, pathItems, third)
}

// amendValueWithUnaryFunction is q's @[v;i;f]: apply unary f to the items of
// v selected by i, copy-on-write like the dyadic amend forms.
func (s *EvalState) amendValueWithUnaryFunction(v any, key any, fn any) (any, error) {
	if !isCallable(fn) {
		return nil, fmt.Errorf("amend function is not callable")
	}
	switch x := v.(type) {
	case EvalDict:
		keys, _, err := lookupKeys(key)
		if err != nil {
			return nil, err
		}
		out := x
		for _, k := range keys {
			old, err := dictLookup(out, k)
			if err != nil {
				return nil, err
			}
			next, err := s.applyCallable1(fn, old)
			if err != nil {
				return nil, err
			}
			if out, err = dictAmend(out, k, next); err != nil {
				return nil, err
			}
		}
		return out, nil
	case data.Array:
		indexes, _, err := indexInts(key)
		if err != nil {
			return nil, err
		}
		values := make([]any, len(indexes))
		for i, index := range indexes {
			old, ok := x.At(index)
			if !ok {
				return nil, fmt.Errorf("amend index %d out of range", index)
			}
			next, err := s.applyCallable1(fn, old)
			if err != nil {
				return nil, err
			}
			values[i] = next
		}
		return vectorAmendIndexes(x, indexes, values)
	default:
		return nil, fmt.Errorf("amend expects a dictionary, vector, table, or keyed table")
	}
}

// amendPathUnary is q's .[d;path;f]: walk the index path and apply unary f
// at the leaf, rebuilding containers copy-on-write (mirrors amendPath).
func (s *EvalState) amendPathUnary(target any, path []any, fn any) (any, error) {
	if !isCallable(fn) {
		return nil, fmt.Errorf("amend function is not callable")
	}
	if len(path) == 0 {
		return s.applyCallable1(fn, target)
	}
	if len(path) == 1 {
		return s.amendValueWithUnaryFunction(target, path[0], fn)
	}
	child, err := indexValue(target, path[0])
	if err != nil {
		return nil, err
	}
	next, err := s.amendPathUnary(child, path[1:], fn)
	if err != nil {
		return nil, err
	}
	return setContainerChild(target, path[0], next)
}
