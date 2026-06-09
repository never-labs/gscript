package q

import (
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qApplyIndexMode byte

const (
	qApplyIndexAt qApplyIndexMode = iota
	qApplyIndexDot
)

func (s *EvalState) evalApplyIndexForm(src string) (any, bool, error) {
	if strings.HasPrefix(src, ".[") && strings.HasSuffix(src, "]") {
		return s.evalDotApplyOrAmend(src)
	}
	if left, right, ok := splitTopLevelOperator(src, "@"); ok {
		out, err := s.evalApplyIndex(qApplyIndexAt, left, right)
		return out, true, err
	}
	if left, right, ok := splitTopLevelDotApply(src); ok {
		out, err := s.evalApplyIndex(qApplyIndexDot, left, right)
		return out, true, err
	}
	return nil, false, nil
}

func (s *EvalState) evalDotApplyOrAmend(src string) (any, bool, error) {
	inner := strings.TrimSpace(src[2 : len(src)-1])
	parts := splitTopLevelDelim(inner, ';')
	switch len(parts) {
	case 2:
		fn, err := s.eval(parts[0])
		if err != nil {
			return nil, true, err
		}
		args, err := s.evalDotApplyArgs(parts[1])
		if err != nil {
			return nil, true, err
		}
		out, err := s.applyCallable(fn, args)
		return out, true, err
	case 4:
		out, err := s.evalDotAmend(src)
		return out, true, err
	default:
		return nil, true, fmt.Errorf("dot apply expects .[fn;args] or .[dict;path;op;value]")
	}
}

func (s *EvalState) evalDotApplyArgs(src string) ([]any, error) {
	value, err := s.eval(src)
	if err != nil {
		return nil, err
	}
	return qApplyArgs(value), nil
}

func (s *EvalState) evalApplyIndex(mode qApplyIndexMode, leftExpr, rightExpr string) (any, error) {
	left, err := s.eval(leftExpr)
	if err != nil {
		return nil, err
	}
	right, err := s.eval(rightExpr)
	if err != nil {
		return nil, err
	}
	return s.applyOrIndexValue(mode, left, right)
}

func (s *EvalState) applyOrIndexValue(mode qApplyIndexMode, target, arg any) (any, error) {
	if isCallable(target) {
		if mode == qApplyIndexAt {
			return s.applyCallable(target, []any{arg})
		}
		return s.applyCallable(target, qApplyArgs(arg))
	}
	if mode == qApplyIndexDot {
		return dotIndexValue(target, arg)
	}
	return indexValue(target, arg)
}

func qApplyArgs(arg any) []any {
	if array, ok := arg.(data.Array); ok {
		items := array.Values()
		out := make([]any, len(items))
		copy(out, items)
		return out
	}
	return []any{arg}
}

func dotIndexValue(target any, path any) (any, error) {
	if matrix, ok := target.(data.Matrix); ok {
		indexes, scalar, err := indexInts(path)
		if err != nil {
			recordRuntimeKernelProbe("MatrixIndex", "matrix-dot/path-error", false, err)
			return nil, err
		}
		shape := fmt.Sprintf("matrix-dot/%dx%d/%d-indexes", qMatrixRows(matrix), qMatrixCols(matrix), len(indexes))
		switch {
		case scalar:
			row, ok := matrix.RowArray(indexes[0])
			if !ok {
				err := fmt.Errorf("index %d out of range", indexes[0])
				recordRuntimeKernelProbe("MatrixIndex", shape, true, err)
				return nil, err
			}
			out, handled, err := qTypedRuntimeResult("MatrixIndex", shape, row, true, nil)
			if err != nil {
				return nil, err
			}
			if handled {
				return out, nil
			}
			return row, nil
		case len(indexes) == 2:
			cell, ok := matrix.Cell(indexes[0], indexes[1])
			if !ok {
				err := fmt.Errorf("index %d %d out of range", indexes[0], indexes[1])
				recordRuntimeKernelProbe("MatrixIndex", shape, true, err)
				return nil, err
			}
			out, handled, err := qTypedRuntimeResult("MatrixIndex", shape, cell, true, nil)
			if err != nil {
				return nil, err
			}
			if handled {
				return out, nil
			}
			return cell, nil
		}
	}
	if array, ok := path.(data.Array); ok {
		current := target
		for _, item := range array.Values() {
			next, err := indexValue(current, item)
			if err != nil {
				return nil, err
			}
			current = next
		}
		return current, nil
	}
	return indexValue(target, path)
}

func qMatrixRows(matrix data.Matrix) int {
	shape := matrix.Shape()
	if len(shape) == 0 {
		return 0
	}
	return shape[0]
}

func qMatrixCols(matrix data.Matrix) int {
	shape := matrix.Shape()
	if len(shape) < 2 {
		return 0
	}
	return shape[1]
}

func splitTopLevelDotApply(src string) (string, string, bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '`':
			i = qSymbolLiteralEnd(src, i) - 1
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '.':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && qDotApplyBoundary(src, i) {
				left := strings.TrimSpace(src[:i])
				right := strings.TrimSpace(src[i+1:])
				return left, right, left != "" && right != ""
			}
		}
	}
	return "", "", false
}

func qDotApplyBoundary(src string, i int) bool {
	before := i > 0 && isSpace(src[i-1])
	after := i+1 < len(src) && isSpace(src[i+1])
	return before && after
}
