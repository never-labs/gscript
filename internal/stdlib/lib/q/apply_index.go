package q

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qApplyIndexMode byte

const (
	qApplyIndexAt qApplyIndexMode = iota
	qApplyIndexDot
)

type qScalarApplyIndexPlan struct {
	mode    qApplyIndexMode
	target  string
	index   int
	indexes []int
	scalar  bool
}

func (s *EvalState) evalApplyIndexForm(src string) (any, bool, error) {
	if out, ok, err := s.tryEvalScalarApplyIndexFastPath(src); ok || err != nil {
		return out, ok, err
	}
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
	src = strings.TrimSpace(src)
	if src == "()" {
		return nil, nil
	}
	if enclosed(src, '(', ')') {
		inner := strings.TrimSpace(src[1 : len(src)-1])
		parts := splitTopLevelDelim(inner, ';')
		if len(parts) > 1 {
			args := make([]any, len(parts))
			for i, part := range parts {
				value, err := s.eval(part)
				if err != nil {
					return nil, err
				}
				args[i] = value
			}
			return args, nil
		}
	}
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
		return array.Values()
	}
	return []any{arg}
}

func (s *EvalState) tryEvalScalarApplyIndexFastPath(src string) (any, bool, error) {
	plan, ok := s.scalarApplyIndexPlan(src)
	if !ok {
		return nil, false, nil
	}
	target, ok := s.lookupName(plan.target)
	if !ok || isCallable(target) {
		return nil, false, nil
	}
	return scalarApplyIndexPlanValue(plan, target)
}

func (s *EvalState) scalarApplyIndexPlan(src string) (qScalarApplyIndexPlan, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return qScalarApplyIndexPlan{}, false
	}
	if s.applyIndexCache != nil {
		if plan, ok := s.applyIndexCache[src]; ok {
			return plan, true
		}
	}
	plan, ok := buildScalarApplyIndexPlan(src)
	if !ok {
		return qScalarApplyIndexPlan{}, false
	}
	if s.applyIndexCache == nil {
		s.applyIndexCache = make(map[string]qScalarApplyIndexPlan, 16)
	} else if len(s.applyIndexCache) >= 128 {
		s.applyIndexCache = make(map[string]qScalarApplyIndexPlan, 16)
	}
	s.applyIndexCache[src] = plan
	return plan, true
}

func buildScalarApplyIndexPlan(src string) (qScalarApplyIndexPlan, bool) {
	if left, right, ok := splitTopLevelOperator(src, "@"); ok {
		return scalarApplyIndexPlanFromParts(qApplyIndexAt, left, right)
	}
	if left, right, ok := splitTopLevelDotApply(src); ok {
		return scalarApplyIndexPlanFromParts(qApplyIndexDot, left, right)
	}
	return qScalarApplyIndexPlan{}, false
}

func scalarApplyIndexPlanFromParts(mode qApplyIndexMode, left, right string) (qScalarApplyIndexPlan, bool) {
	target := strings.TrimSpace(left)
	if !isQAssignmentName(target) {
		return qScalarApplyIndexPlan{}, false
	}
	indexes, scalar, ok := parseIndexLiteralPath(strings.TrimSpace(right))
	if !ok {
		return qScalarApplyIndexPlan{}, false
	}
	index := 0
	if len(indexes) > 0 {
		index = indexes[0]
	}
	return qScalarApplyIndexPlan{mode: mode, target: target, index: index, indexes: indexes, scalar: scalar}, true
}

func parseScalarIndexLiteral(src string) (int, bool) {
	if src == "" || src[0] == '-' || strings.ContainsAny(src, " \t\r\n.") {
		return 0, false
	}
	n, err := strconv.ParseInt(src, 10, 0)
	if err != nil || n < 0 {
		return 0, false
	}
	return int(n), true
}

func parseIndexLiteralPath(src string) ([]int, bool, bool) {
	if index, ok := parseScalarIndexLiteral(src); ok {
		return []int{index}, true, true
	}
	if src == "" || strings.ContainsAny(src, ".;()[]{}") {
		return nil, false, false
	}
	fields := strings.Fields(src)
	if len(fields) <= 1 {
		return nil, false, false
	}
	indexes := make([]int, len(fields))
	for i, field := range fields {
		if field == "" || field[0] == '-' {
			return nil, false, false
		}
		n, err := strconv.ParseInt(field, 10, 0)
		if err != nil || n < 0 {
			return nil, false, false
		}
		indexes[i] = int(n)
	}
	return indexes, false, true
}

func scalarApplyIndexPlanValue(plan qScalarApplyIndexPlan, target any) (any, bool, error) {
	if plan.scalar || len(plan.indexes) == 1 {
		return scalarIndexValue(plan.mode, target, plan.index)
	}
	if plan.mode == qApplyIndexDot {
		if matrix, ok := target.(data.Matrix); ok && len(plan.indexes) == 2 {
			shape := qMatrixIndexShape(matrix, len(plan.indexes))
			cell, handled, err := data.TryMatrixCellIndex(matrix, plan.indexes[0], plan.indexes[1])
			if err != nil || !handled {
				if err == nil {
					err = fmt.Errorf("index %d %d out of range", plan.indexes[0], plan.indexes[1])
				}
				recordRuntimeKernelProbe("MatrixIndex", shape, true, err)
				return nil, true, err
			}
			out, handled, err := qTypedRuntimeResult("MatrixIndex", shape, cell, true, nil)
			if err != nil {
				return nil, true, err
			}
			if handled {
				return out, true, nil
			}
			return cell, true, nil
		}
	}
	return nil, false, nil
}

func scalarIndexValue(mode qApplyIndexMode, target any, row int) (any, bool, error) {
	switch x := target.(type) {
	case data.Matrix:
		return nil, false, nil
	case data.Array:
		shape := qScalarApplyIndexShape(mode, string(x.Kind()))
		recordRuntimeKernelExecution("ArrayScalarIndex", shape, "attempt", "attempt")
		value, ok := x.At(row)
		if !ok {
			err := fmt.Errorf("index %d out of range", row)
			recordRuntimeKernelExecution("ArrayScalarIndex", shape, "error", "runtime_error")
			return nil, true, err
		}
		recordRuntimeKernelExecution("ArrayScalarIndex", shape, "hit", "typed_scalar_index")
		return value, true, nil
	case string:
		shape := qScalarApplyIndexShape(mode, string(data.KindString))
		recordRuntimeKernelExecution("StringScalarIndex", shape, "attempt", "attempt")
		runes := []rune(x)
		if row < 0 || row >= len(runes) {
			err := fmt.Errorf("index %d out of range", row)
			recordRuntimeKernelExecution("StringScalarIndex", shape, "error", "runtime_error")
			return nil, true, err
		}
		recordRuntimeKernelExecution("StringScalarIndex", shape, "hit", "typed_scalar_index")
		return string(runes[row]), true, nil
	default:
		return nil, false, nil
	}
}

func qScalarApplyIndexShape(mode qApplyIndexMode, kind string) string {
	if mode == qApplyIndexDot {
		switch kind {
		case string(data.KindI64):
			return "scalar-index/dot/i64"
		case string(data.KindString):
			return "scalar-index/dot/string"
		default:
			return "scalar-index/dot/" + kind
		}
	}
	switch kind {
	case string(data.KindI64):
		return "scalar-index/at/i64"
	case string(data.KindString):
		return "scalar-index/at/string"
	default:
		return "scalar-index/at/" + kind
	}
}

func dotIndexValue(target any, path any) (any, error) {
	if matrix, ok := target.(data.Matrix); ok {
		indexes, scalar, err := indexInts(path)
		if err != nil {
			recordRuntimeKernelProbe("MatrixIndex", "matrix-dot/path-error", false, err)
			return nil, err
		}
		shape := qMatrixIndexShape(matrix, len(indexes))
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
			cell, handled, err := data.TryMatrixCellIndex(matrix, indexes[0], indexes[1])
			if err != nil || !handled {
				if err == nil {
					err = fmt.Errorf("index %d %d out of range", indexes[0], indexes[1])
				}
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

func qMatrixIndexShape(matrix data.Matrix, indexes int) string {
	return fmt.Sprintf("matrix-dot/%dx%d/%d-indexes", qMatrixRows(matrix), qMatrixCols(matrix), indexes)
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
