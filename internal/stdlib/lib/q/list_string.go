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
	indexes, err := qIntegerIndexes("cut", left)
	if err != nil {
		return nil, err
	}
	return data.Cut(indexes, right)
}

func qSublistValue(left, right any) (any, error) {
	args, err := qIntegerIndexes("sublist", left)
	if err != nil {
		return nil, err
	}
	switch len(args) {
	case 1:
		return take(args[0], right)
	case 2:
		return data.Sublist(args[0], args[1], right)
	default:
		return nil, fmt.Errorf("sublist expects count or start count")
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
	// Numeric left atom dispatches base decode: 2 sv 1 0 1 0 is 10 in
	// canonical q. String/symbol left keeps the join semantics.
	if base, ok := qNumericBase(left); ok {
		return qBaseDecode(base, right)
	}
	sep, err := qStringOperand("sv", left)
	if err != nil {
		return nil, err
	}
	items := qListItems(right)
	return data.StringJoin(sep, items)
}

func qVSValue(left, right any) (any, error) {
	// Numeric left atom dispatches base encode: 2 vs 10 is 1 0 1 0 and
	// 10 vs 123 is 1 2 3 in canonical q. String/symbol left keeps split.
	if base, ok := qNumericBase(left); ok {
		return qBaseEncode(base, right)
	}
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

// qNumericBase recognizes an integer atom radix for numeric vs/sv. Bools and
// bytes are excluded: 0b vs (bit decompose) and 0x0 vs (byte decompose) keep
// their canonical meanings unimplemented rather than mis-binding here.
func qNumericBase(v any) (int64, bool) {
	switch x := v.(type) {
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	default:
		return 0, false
	}
}

// qBaseEncode is numeric vs: digits of value in the given base, most
// significant first, as a long vector (2 vs 10 -> 1 0 1 0).
func qBaseEncode(base int64, value any) (any, error) {
	if base < 2 {
		return nil, fmt.Errorf("vs base must be at least 2")
	}
	n, ok := integerValue(value)
	if !ok || n < 0 {
		return nil, fmt.Errorf("vs expects a non-negative integer value for base encode")
	}
	var digits []int64
	for n > 0 {
		digits = append(digits, n%base)
		n /= base
	}
	if len(digits) == 0 {
		digits = []int64{0}
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return data.NewI64(digits), nil
}

// qBaseDecode is numeric sv: fold digits back into a long
// (2 sv 1 0 1 0 -> 10).
func qBaseDecode(base int64, value any) (any, error) {
	if base < 2 {
		return nil, fmt.Errorf("sv base must be at least 2")
	}
	array, ok := value.(data.Array)
	if !ok {
		n, ok := integerValue(value)
		if !ok {
			return nil, fmt.Errorf("sv expects integer digits for base decode")
		}
		return n, nil
	}
	var out int64
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return nil, fmt.Errorf("sv digit row %d out of range", i)
		}
		n, ok := integerValue(item)
		if !ok {
			return nil, fmt.Errorf("sv expects integer digits for base decode")
		}
		out = out*base + n
	}
	return out, nil
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
