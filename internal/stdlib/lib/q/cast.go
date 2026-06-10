package q

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qCastTarget struct {
	kind       data.Kind
	sourceText string
}

func qSymbolCastTarget() qCastTarget {
	return qCastTarget{kind: data.KindSymbol, sourceText: "`"}
}

func qCastTargetFromDomain(domain any) (qCastTarget, bool) {
	switch x := domain.(type) {
	case qCastTarget:
		return x, true
	case data.Symbol:
		if kind, ok := qCastKindFromSymbol(x); ok {
			return qCastTarget{kind: kind, sourceText: "`" + string(x)}, true
		}
	case string:
		if kind, ok := qCastKindFromTypeText(x); ok {
			return qCastTarget{kind: kind, sourceText: strconv.Quote(x)}, true
		}
	case int16:
		if kind, ok := qCastKindFromTypeCode(int(x)); ok {
			return qCastTarget{kind: kind, sourceText: strconv.FormatInt(int64(x), 10)}, true
		}
	case int32:
		if kind, ok := qCastKindFromTypeCode(int(x)); ok {
			return qCastTarget{kind: kind, sourceText: strconv.FormatInt(int64(x), 10)}, true
		}
	case int64:
		if kind, ok := qCastKindFromTypeCode(int(x)); ok {
			return qCastTarget{kind: kind, sourceText: strconv.FormatInt(x, 10)}, true
		}
	}
	return qCastTarget{}, false
}

func castOrEnum(domain any, values any) (any, error) {
	if target, ok := qCastTargetFromDomain(domain); ok {
		return castValue(target, values)
	}
	return enumCast(domain, values)
}

func qCastKindFromSymbol(sym data.Symbol) (data.Kind, bool) {
	return qCastKindFromTypeText(string(sym))
}

func qCastKindFromTypeText(text string) (data.Kind, bool) {
	switch strings.ToLower(text) {
	case "b", "bool", "boolean":
		return data.KindBool, true
	case "x", "byte":
		return data.KindU8, true
	case "h", "short":
		return data.KindI16, true
	case "i", "int":
		return data.KindI32, true
	case "j", "long":
		return data.KindI64, true
	case "e", "real":
		return data.KindF32, true
	case "f", "float":
		return data.KindF64, true
	case "s", "symbol":
		return data.KindSymbol, true
	case "c", "char", "string":
		return data.KindString, true
	case "m", "month":
		return data.KindMonth, true
	case "d", "date":
		return data.KindDate, true
	case "z", "datetime":
		return data.KindDateTime, true
	case "n", "timespan":
		return data.KindTimespan, true
	case "u", "minute":
		return data.KindMinute, true
	case "v", "second":
		return data.KindSecond, true
	case "t", "time":
		return data.KindTime, true
	case "p", "timestamp":
		return data.KindTimestamp, true
	default:
		return "", false
	}
}

func qCastKindFromTypeCode(code int) (data.Kind, bool) {
	if code < 0 {
		code = -code
	}
	switch code {
	case 1:
		return data.KindBool, true
	case 4:
		return data.KindU8, true
	case 5:
		return data.KindI16, true
	case 6:
		return data.KindI32, true
	case 7:
		return data.KindI64, true
	case 8:
		return data.KindF32, true
	case 9:
		return data.KindF64, true
	case 10:
		return data.KindString, true
	case 11:
		return data.KindSymbol, true
	case 12:
		return data.KindTimestamp, true
	case 13:
		return data.KindMonth, true
	case 14:
		return data.KindDate, true
	case 15:
		return data.KindDateTime, true
	case 16:
		return data.KindTimespan, true
	case 17:
		return data.KindMinute, true
	case 18:
		return data.KindSecond, true
	case 19:
		return data.KindTime, true
	default:
		return "", false
	}
}

func castValue(target qCastTarget, values any) (any, error) {
	if array, ok := values.(data.Array); ok {
		if typed, handled, err := tryTypedQCast(target, array); handled || err != nil {
			if err != nil {
				return nil, fmt.Errorf("q cast %s: %w", target.sourceText, err)
			}
			return typed, nil
		}
		out := array.Values()
		for i, value := range out {
			normalized, err := castScalarValue(target.kind, value)
			if err != nil {
				return nil, fmt.Errorf("q cast %s value %d: %w", target.sourceText, i+1, err)
			}
			out[i] = normalized
		}
		column, err := data.NewColumnWithKind("_", target.kind, out)
		if err != nil {
			return nil, fmt.Errorf("q cast %s: %w", target.sourceText, err)
		}
		return column.Data, nil
	}
	normalized, err := castScalarValue(target.kind, values)
	if err != nil {
		return nil, fmt.Errorf("q cast %s: %w", target.sourceText, err)
	}
	return normalized, nil
}

func tryTypedQCast(target qCastTarget, array data.Array) (data.Array, bool, error) {
	return data.TryTypedCast(target.kind, array)
}

func evalQTypedCastPrimitive(target qCastTarget, value any) (any, bool, error) {
	shape := "q-cast/" + string(target.kind) + "/" + string(qRuntimeKernelOperandKind(value, nil))
	return evalQTypedRuntimeKernel(qTypedRuntimeKernel[any]{
		kernel:         "QCastPrimitive",
		shape:          shape,
		fallbackReason: RuntimeFallbackUnsupportedType,
		call: func() (any, bool, error) {
			return qTypedCastPrimitiveValue(target, value)
		},
	})
}

func qTypedCastPrimitiveValue(target qCastTarget, value any) (any, bool, error) {
	if array, ok := value.(data.Array); ok {
		if target.kind == data.KindString {
			out, handled, err := data.TryTypedStringCast(array)
			if err != nil || handled {
				return out, handled, err
			}
		}
		if out, handled, err := data.TryTypedCast(target.kind, array); err != nil || handled {
			return out, handled, err
		}
		if target.kind != data.KindSymbol && target.kind != data.KindString {
			return nil, false, nil
		}
		values := make([]any, array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return nil, true, fmt.Errorf("q cast %s row %d out of range", target.sourceText, i+1)
			}
			converted, err := castScalarValue(target.kind, item)
			if err != nil {
				return nil, true, err
			}
			values[i] = converted
		}
		column, err := data.NewColumnWithKind("_", target.kind, values)
		if err != nil {
			return nil, true, err
		}
		return column.Data, true, nil
	}
	switch target.kind {
	case data.KindSymbol, data.KindString, data.KindBool,
		data.KindI8, data.KindI16, data.KindI32, data.KindI64,
		data.KindU8, data.KindU16, data.KindU32, data.KindU64,
		data.KindF32, data.KindF64:
		out, err := castScalarValue(target.kind, value)
		return out, true, err
	default:
		return nil, false, nil
	}
}

func castScalarValue(kind data.Kind, value any) (any, error) {
	if data.IsNull(value) {
		return data.NullForKind(kind), nil
	}
	if temporalKind, ok := qTemporalCastKindName(kind); ok {
		if text, ok := value.(string); ok {
			return parseQTemporal(temporalKind, text)
		}
	}
	if text, ok := value.(string); ok {
		if parsed, handled, err := castScalarStringValue(kind, text); handled || err != nil {
			return parsed, err
		}
	}
	if kind == data.KindString {
		switch x := value.(type) {
		case data.Symbol:
			return string(x), nil
		default:
			return data.NormalizeValueForKind(kind, value)
		}
	}
	if kind == data.KindBool {
		switch x := value.(type) {
		case bool:
			return x, nil
		case int64:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		case int32:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		case int16:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		case int8:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		case uint8:
			if x == 0 || x == 1 {
				return x == 1, nil
			}
		}
	}
	if converted, ok, err := castScalarNumericIntegerValue(kind, value); ok || err != nil {
		return converted, err
	}
	return data.NormalizeValueForKind(kind, value)
}

func castScalarNumericIntegerValue(kind data.Kind, value any) (any, bool, error) {
	min, max, signed, ok := qCastIntegerBounds(kind)
	if !ok {
		return nil, false, nil
	}
	n, ok := qCastTruncatedInteger(value)
	if !ok {
		return nil, false, nil
	}
	if !signed && n < 0 {
		return nil, true, fmt.Errorf("value must be non-negative for %s", kind)
	}
	if n < min || n > max {
		return nil, true, fmt.Errorf("value %d out of range for %s", n, kind)
	}
	switch kind {
	case data.KindI8:
		return int8(n), true, nil
	case data.KindI16:
		return int16(n), true, nil
	case data.KindI32:
		return int32(n), true, nil
	case data.KindI64:
		return n, true, nil
	case data.KindU8:
		return uint8(n), true, nil
	case data.KindU16:
		return uint16(n), true, nil
	case data.KindU32:
		return uint32(n), true, nil
	case data.KindU64:
		return uint64(n), true, nil
	default:
		return nil, false, nil
	}
}

func qCastIntegerBounds(kind data.Kind) (min, max int64, signed, ok bool) {
	switch kind {
	case data.KindI8:
		return -128, 127, true, true
	case data.KindI16:
		return -32768, 32767, true, true
	case data.KindI32:
		return -2147483648, 2147483647, true, true
	case data.KindI64:
		return -9223372036854775808, 9223372036854775807, true, true
	case data.KindU8:
		return 0, 255, false, true
	case data.KindU16:
		return 0, 65535, false, true
	case data.KindU32:
		return 0, 4294967295, false, true
	case data.KindU64:
		return 0, 9223372036854775807, false, true
	default:
		return 0, 0, false, false
	}
}

func qCastTruncatedInteger(value any) (int64, bool) {
	switch x := value.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x <= 9223372036854775807 {
			return int64(x), true
		}
		return 0, false
	case float32:
		return qCastTruncatedFloatInteger(float64(x))
	case float64:
		return qCastTruncatedFloatInteger(x)
	default:
		return 0, false
	}
}

func qCastTruncatedFloatInteger(value float64) (int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < -9223372036854775808.0 || value >= 9223372036854775808.0 {
		return 0, false
	}
	return int64(value), true
}

func castScalarStringValue(kind data.Kind, text string) (any, bool, error) {
	switch kind {
	case data.KindSymbol:
		return data.Symbol(text), true, nil
	case data.KindI8:
		n, err := strconv.ParseInt(text, 10, 8)
		if err != nil {
			return nil, true, err
		}
		return int8(n), true, nil
	case data.KindI16:
		n, err := strconv.ParseInt(text, 10, 16)
		if err != nil {
			return nil, true, err
		}
		return int16(n), true, nil
	case data.KindI32:
		n, err := strconv.ParseInt(text, 10, 32)
		if err != nil {
			return nil, true, err
		}
		return int32(n), true, nil
	case data.KindI64:
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, true, err
		}
		return n, true, nil
	case data.KindU8:
		n, err := strconv.ParseUint(text, 10, 8)
		if err != nil {
			return nil, true, err
		}
		return uint8(n), true, nil
	case data.KindF32:
		n, err := strconv.ParseFloat(text, 32)
		if err != nil {
			return nil, true, err
		}
		return float32(n), true, nil
	case data.KindF64:
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, true, err
		}
		return n, true, nil
	default:
		return nil, false, nil
	}
}

func (s *EvalState) evalCastDomain(src string) (any, error) {
	switch strings.TrimSpace(src) {
	case "", "`":
		return qSymbolCastTarget(), nil
	}
	value, err := s.eval(src)
	if err == nil {
		return value, nil
	}
	if isBareQCastName(src) {
		return data.Symbol(src), nil
	}
	return nil, err
}

func isBareQCastName(src string) bool {
	if !isQAssignmentName(src) {
		return false
	}
	_, ok := qCastKindFromSymbol(data.Symbol(src))
	return ok
}

func qTemporalCastKindName(kind data.Kind) (string, bool) {
	switch kind {
	case data.KindMonth:
		return "month", true
	case data.KindDate:
		return "date", true
	case data.KindDateTime:
		return "datetime", true
	case data.KindTimespan:
		return "timespan", true
	case data.KindMinute:
		return "minute", true
	case data.KindSecond:
		return "second", true
	case data.KindTime:
		return "time", true
	case data.KindTimestamp:
		return "timestamp", true
	default:
		return "", false
	}
}

func enumCast(domain any, values any) (qEnumVector, error) {
	sym, ok := domain.(data.Symbol)
	if !ok {
		return qEnumVector{}, fmt.Errorf("q enum cast expects a symbol domain")
	}
	var symbols []data.Symbol
	switch x := values.(type) {
	case qEnumVector:
		if x.domain == sym {
			return x, nil
		}
		for _, value := range x.Values() {
			symbol, ok := value.(data.Symbol)
			if !ok {
				return qEnumVector{}, fmt.Errorf("q enum cast `%s expects symbol values", sym)
			}
			symbols = append(symbols, symbol)
		}
	case data.Symbol:
		symbols = []data.Symbol{x}
	case data.Array:
		if x.Kind() != data.KindSymbol {
			return qEnumVector{}, fmt.Errorf("q enum cast `%s expects symbol values", sym)
		}
		values := x.Values()
		symbols = make([]data.Symbol, len(values))
		for i, value := range values {
			symbol, ok := value.(data.Symbol)
			if !ok {
				return qEnumVector{}, fmt.Errorf("q enum cast `%s expects symbol values", sym)
			}
			symbols[i] = symbol
		}
	default:
		return qEnumVector{}, fmt.Errorf("q enum cast `%s expects symbol values", sym)
	}
	return qEnumVector{domain: sym, encoded: data.NewEncodedSymbols(symbols)}, nil
}
