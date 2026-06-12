package q

import (
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func parseQBoolLiteral(src string) (any, bool, error) {
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return nil, false, nil
	}
	compact := ""
	if len(fields) == 1 {
		compact = fields[0]
	} else {
		last := fields[len(fields)-1]
		if !strings.HasSuffix(last, "b") {
			return nil, false, nil
		}
		var b strings.Builder
		for i, field := range fields {
			if i == len(fields)-1 {
				field = strings.TrimSuffix(field, "b")
			}
			if field == "" {
				return nil, false, nil
			}
			b.WriteString(field)
		}
		compact = b.String() + "b"
	}
	if !strings.HasSuffix(compact, "b") || len(compact) <= 1 {
		return nil, false, nil
	}
	body := strings.TrimSuffix(compact, "b")
	out := make([]bool, len(body))
	for i, r := range body {
		switch r {
		case '0':
			out[i] = false
		case '1':
			out[i] = true
		default:
			return nil, false, nil
		}
	}
	if len(fields) == 1 && len(out) == 1 {
		return out[0], true, nil
	}
	return data.NewBool(out), true, nil
}

func qCutValue(left, right any) (any, error) {
	if starts, atom, err := qAtomCutStarts(left, right); atom {
		if err != nil {
			return nil, err
		}
		return data.Cut(starts, right)
	}
	indexes, err := qIntegerIndexes("cut", left)
	if err != nil {
		return nil, err
	}
	return data.Cut(indexes, right)
}

// qAtomCutStarts implements canonical atom cut: `n cut x` chunks x into
// n-sized pieces (2 cut 1 2 3 4 5 -> (1 2;3 4;,5)), equivalent to cutting at
// 0, n, 2n, ... Vector-left cut keeps its cut-at-indexes semantics.
func qAtomCutStarts(left, right any) (starts []int, atom bool, err error) {
	if _, isArray := left.(data.Array); isArray {
		return nil, false, nil
	}
	n, ok := integerValue(left)
	if !ok || int64(int(n)) != n {
		return nil, false, nil
	}
	if n <= 0 {
		return nil, true, fmt.Errorf("cut chunk size must be a positive integer")
	}
	length, ok := qSequenceLengthOf(right)
	if !ok {
		return nil, true, fmt.Errorf("cut expects a list or string")
	}
	starts = make([]int, 0, (length+int(n)-1)/int(n))
	for i := 0; i < length; i += int(n) {
		starts = append(starts, i)
	}
	return starts, true, nil
}

// qSequenceLengthOf reports the element count of list-like values.
func qSequenceLengthOf(v any) (int, bool) {
	switch x := v.(type) {
	case data.Array:
		return x.Len(), true
	case string:
		return len([]rune(x)), true
	case data.Frame:
		return x.Len(), true
	case data.KeyedFrame:
		return x.Frame().Len(), true
	case EvalDict:
		return len(x.Keys), true
	default:
		return 0, false
	}
}

func qSublistValue(left, right any) (any, error) {
	args, err := qIntegerIndexes("sublist", left)
	if err != nil {
		return nil, err
	}
	switch len(args) {
	case 1:
		return qSublistTake(args[0], right)
	case 2:
		return data.Sublist(args[0], args[1], right)
	default:
		return nil, fmt.Errorf("sublist expects count or start count")
	}
}

// qSublistTake is the canonical one-argument sublist: at most |n| items from
// the front (n>=0) or back (n<0) WITHOUT cycling — sublist never overtakes
// (10 sublist 1 2 -> 1 2), unlike take (#).
func qSublistTake(n int, v any) (any, error) {
	if length, ok := qSequenceLengthOf(v); ok {
		if n >= 0 {
			if n > length {
				n = length
			}
		} else if -n > length {
			n = -length
		}
	}
	return take(n, v)
}

// qSublistTakeCount is the row count qSublistTake would produce.
func qSublistTakeCount(n int, v any) int64 {
	if length, ok := qSequenceLengthOf(v); ok {
		if n < 0 {
			n = -n
		}
		if n > length {
			n = length
		}
		return int64(n)
	}
	return qTakeCount(n, v)
}

// qSublistTransformArgs converts q sublist arguments to the data sequence
// layer's clamping start/count form. The data layer's one-argument sublist
// step is take (used by `n#x` pipelines, which cycle); q's one-argument
// sublist clamps instead, so it must be encoded as start 0/count n. Negative
// one-argument sublist (take from the back) is not expressible; callers fall
// back to the generic route.
func qSublistTransformArgs(args []int) ([]int, bool) {
	switch len(args) {
	case 1:
		if args[0] < 0 {
			return nil, false
		}
		return []int{0, args[0]}, true
	case 2:
		return args, true
	default:
		return nil, false
	}
}

func qIntegerIndexes(name string, value any) ([]int, error) {
	if n, ok := integerValue(value); ok && int64(int(n)) == n {
		return []int{int(n)}, nil
	}
	array, ok := value.(data.Array)
	if !ok {
		return nil, fmt.Errorf("%s expects integer indexes", name)
	}
	out := make([]int, array.Len())
	for i := 0; i < array.Len(); i++ {
		value, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("%s index row %d out of range", name, i)
		}
		n, ok := integerValue(value)
		if !ok || int64(int(n)) != n {
			return nil, fmt.Errorf("%s expects integer indexes", name)
		}
		out[i] = int(n)
	}
	return out, nil
}

func qCrossValue(left, right any) (any, error) {
	return data.Cross(left, right), nil
}

func qListItems(value any) []any {
	return data.SequenceItems(value)
}

func qTrimValue(value any) (any, error) {
	return data.TrimStringValue(value)
}

func qLTrimValue(value any) (any, error) {
	return data.LTrimStringValue(value)
}

func qRTrimValue(value any) (any, error) {
	return data.RTrimStringValue(value)
}

func qSSValue(left, right any) (any, error) {
	haystack, err := qStringOperand("ss", left)
	if err != nil {
		return nil, err
	}
	needle, err := qStringOperand("ss", right)
	if err != nil {
		return nil, err
	}
	return data.StringSearch(haystack, needle), nil
}

func qSSRValue(args any) (any, error) {
	items, err := qArgsN("ssr", args, 3)
	if err != nil {
		return nil, err
	}
	source, err := qStringOperand("ssr", items[0])
	if err != nil {
		return nil, err
	}
	old, err := qStringOperand("ssr", items[1])
	if err != nil {
		return nil, err
	}
	repl, err := qStringOperand("ssr", items[2])
	if err != nil {
		return nil, err
	}
	return data.StringReplaceAll(source, old, repl), nil
}

func qSSRWithSourceValue(left, right any) (any, error) {
	items, err := qArgsN("ssr", right, 2)
	if err != nil {
		return nil, err
	}
	return qSSRValue(data.NewAny([]any{left, items[0], items[1]}))
}

func qSVValue(left, right any) (any, error) {
	sep, err := qStringOperand("sv", left)
	if err != nil {
		return nil, err
	}
	items := qListItems(right)
	return data.StringJoin(sep, items)
}

func qVSValue(left, right any) (any, error) {
	sep, err := qStringOperand("vs", left)
	if err != nil {
		return nil, err
	}
	text, err := qStringOperand("vs", right)
	if err != nil {
		return nil, err
	}
	return data.StringSplit(sep, text), nil
}

func qStringOperand(name string, value any) (string, error) {
	return data.StringScalar(name, value)
}

func qArgsN(name string, args any, n int) ([]any, error) {
	items := qListItems(args)
	if len(items) != n {
		return nil, fmt.Errorf("%s expects %d arguments, got %d", name, n, len(items))
	}
	return items, nil
}

func qFunctionCallArgs(src string) ([]string, bool) {
	open := strings.Index(src, "[")
	if open <= 0 || !strings.HasSuffix(src, "]") || !enclosed(src[open:], '[', ']') {
		return nil, false
	}
	name := strings.TrimSpace(src[:open])
	if name == "" {
		return nil, false
	}
	return splitTopLevelDelim(strings.TrimSpace(src[open+1:len(src)-1]), ';'), true
}

func (s *EvalState) evalListStringFunctionCall(src string) (any, bool, error) {
	args, ok := qFunctionCallArgs(src)
	if !ok {
		return nil, false, nil
	}
	name := strings.TrimSpace(src[:strings.Index(src, "[")])
	switch name {
	case "cut":
		if len(args) != 2 {
			return nil, true, fmt.Errorf("cut expects 2 arguments")
		}
		left, err := s.eval(args[0])
		if err != nil {
			return nil, true, err
		}
		right, err := s.eval(args[1])
		if err != nil {
			return nil, true, err
		}
		out, err := qCutValue(left, right)
		return out, true, err
	case "sublist":
		if len(args) != 2 {
			return nil, true, fmt.Errorf("sublist expects 2 arguments")
		}
		left, err := s.eval(args[0])
		if err != nil {
			return nil, true, err
		}
		right, err := s.eval(args[1])
		if err != nil {
			return nil, true, err
		}
		out, err := qSublistValue(left, right)
		return out, true, err
	case "ssr":
		values := make([]any, len(args))
		for i, arg := range args {
			value, err := s.eval(arg)
			if err != nil {
				return nil, true, err
			}
			values[i] = value
		}
		out, err := qSSRValue(data.NewAny(values))
		return out, true, err
	default:
		return nil, false, nil
	}
}
