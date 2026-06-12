package q

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

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
	// Temporal accessor casts (`year$d, `mm$d, `hh$t, ...) share the symbol
	// domain syntax with enum casts; the operand disambiguates: accessors
	// only apply to temporal values, enum casts only to symbol values.
	if out, handled, err := qTemporalAccessorCast(domain, values); handled || err != nil {
		return out, err
	}
	// Pad ($ with an integer atom left and string values): canonical q pads
	// 5$"ab" to "ab   " and -5$"ab" to "   ab". This outranks the type-code
	// cast, which canonical q reserves for short atoms (5h$x) — the dialect
	// promotes all ints to long, so a long left with string values is pad.
	if out, handled, err := qPadCast(domain, values); handled || err != nil {
		return out, err
	}
	if target, ok := qCastTargetFromDomain(domain); ok {
		return castValue(target, values)
	}
	return enumCast(domain, values)
}

// qPadCast implements canonical int$string padding: n$s pads s with spaces to
// width |n| (right-pad for n>=0, left-pad for n<0) and truncates to the first
// |n| characters when s is longer. Applies to a string atom or a string list.
// Short atoms (int16) are excluded: canonical q reserves 5h$x for the
// type-code cast, and the dialect keeps its short-code string parsing there.
func qPadCast(domain, values any) (any, bool, error) {
	var width int64
	switch x := domain.(type) {
	case int32:
		width = int64(x)
	case int64:
		width = x
	default:
		return nil, false, nil
	}
	switch v := values.(type) {
	case string:
		return qPadString(width, v), true, nil
	case data.Array:
		if v.Kind() != data.KindString {
			return nil, false, nil
		}
		out := make([]string, v.Len())
		for i := 0; i < v.Len(); i++ {
			item, ok := v.At(i)
			if !ok {
				return nil, true, fmt.Errorf("q cast %d row %d out of range", width, i+1)
			}
			text := ""
			if !data.IsNull(item) {
				s, ok := item.(string)
				if !ok {
					return nil, false, nil
				}
				text = s
			}
			out[i] = qPadString(width, text)
		}
		return data.NewString(out), true, nil
	}
	return nil, false, nil
}

func qPadString(width int64, s string) string {
	w := width
	if w < 0 {
		w = -w
	}
	runes := []rune(s)
	if int64(len(runes)) >= w {
		return string(runes[:w])
	}
	pad := strings.Repeat(" ", int(w)-len(runes))
	if width < 0 {
		return pad + s
	}
	return s + pad
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
		if converted, handled, err := castTemporalScalarValue(kind, value); handled || err != nil {
			return converted, err
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

// --- temporal casts -----------------------------------------------------------

// castTemporalScalarValue converts a temporal scalar into another temporal
// kind for `kind$value casts: date/month/timestamp/datetime move along the
// calendar axis, minute/second/time/timespan along the time-of-day axis, and
// instants project onto either axis (date$timestamp, minute$timestamp, ...).
func castTemporalScalarValue(kind data.Kind, value any) (any, bool, error) {
	u, from, ok := data.TemporalUnderlying(value)
	if !ok {
		return nil, false, nil
	}
	if from == kind {
		return value, true, nil
	}
	// Infinities project onto the target kind keeping their sign.
	if u == math.MaxInt64 || u == -math.MaxInt64 {
		out, ok := data.NewTemporalValue(kind, u)
		if !ok {
			return nil, false, nil
		}
		return out, true, nil
	}
	switch kind {
	case data.KindMonth:
		if year, month, _, ok := qTemporalCivil(from, u); ok {
			return data.MonthFromMonths(int64(year-1970)*12 + int64(month-1)), true, nil
		}
	case data.KindDate:
		if from == data.KindMonth {
			year := 1970 + floorDiv(u, 12)
			month := u - floorDiv(u, 12)*12 + 1
			days := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC).Unix() / 86400
			return data.DateFromDays(days), true, nil
		}
		if days, ok := qTemporalDays(from, u); ok {
			return data.DateFromDays(days), true, nil
		}
	case data.KindTimestamp:
		switch from {
		case data.KindDate:
			return data.TimestampFromUnixNanos(u * temporalNanosPerDay), true, nil
		case data.KindDateTime:
			return data.TimestampFromUnixNanos(u), true, nil
		}
	case data.KindDateTime:
		switch from {
		case data.KindDate:
			return data.DateTimeFromUnixNanos(u * temporalNanosPerDay), true, nil
		case data.KindTimestamp:
			return data.DateTimeFromUnixNanos(u), true, nil
		}
	case data.KindMinute:
		if nanos, ok := qTemporalTimeOfDayNanos(from, u); ok {
			return data.MinuteFromMinutes(floorDiv(nanos, temporalNanosPerMinute)), true, nil
		}
	case data.KindSecond:
		if nanos, ok := qTemporalTimeOfDayNanos(from, u); ok {
			return data.SecondFromSeconds(floorDiv(nanos, temporalNanosPerSecond)), true, nil
		}
	case data.KindTime:
		if from == data.KindTimespan {
			return data.TimeFromNanos(floorMod(u, temporalNanosPerDay)), true, nil
		}
		if nanos, ok := qTemporalTimeOfDayNanos(from, u); ok {
			return data.TimeFromNanos(nanos), true, nil
		}
	case data.KindTimespan:
		switch from {
		case data.KindMinute:
			return data.TimespanFromNanos(u * temporalNanosPerMinute), true, nil
		case data.KindSecond:
			return data.TimespanFromNanos(u * temporalNanosPerSecond), true, nil
		case data.KindTime:
			return data.TimespanFromNanos(u), true, nil
		case data.KindTimestamp, data.KindDateTime:
			return data.TimespanFromNanos(floorMod(u, temporalNanosPerDay)), true, nil
		}
	}
	return nil, true, fmt.Errorf("cannot cast %s to %s", from, kind)
}

// qTemporalDays maps an instant kind onto whole days since 1970-01-01.
func qTemporalDays(kind data.Kind, u int64) (int64, bool) {
	switch kind {
	case data.KindDate:
		return u, true
	case data.KindTimestamp, data.KindDateTime:
		return floorDiv(u, temporalNanosPerDay), true
	default:
		return 0, false
	}
}

// qTemporalCivil returns the civil (year, month, day) of a calendar-bearing
// temporal value.
func qTemporalCivil(kind data.Kind, u int64) (int, int, int, bool) {
	if kind == data.KindMonth {
		year := 1970 + floorDiv(u, 12)
		month := u - floorDiv(u, 12)*12 + 1
		return int(year), int(month), 1, true
	}
	days, ok := qTemporalDays(kind, u)
	if !ok {
		return 0, 0, 0, false
	}
	tm := time.Unix(days*86400, 0).UTC()
	year, month, day := tm.Date()
	return year, int(month), day, true
}

// qTemporalTimeOfDayNanos maps a time-of-day or instant kind onto
// nanoseconds since midnight.
func qTemporalTimeOfDayNanos(kind data.Kind, u int64) (int64, bool) {
	switch kind {
	case data.KindMinute:
		return u * temporalNanosPerMinute, true
	case data.KindSecond:
		return u * temporalNanosPerSecond, true
	case data.KindTime:
		return u, true
	case data.KindTimestamp, data.KindDateTime:
		return floorMod(u, temporalNanosPerDay), true
	default:
		return 0, false
	}
}

func floorMod(left, right int64) int64 {
	return left - floorDiv(left, right)*right
}

// --- temporal accessor casts --------------------------------------------------

// qTemporalAccessorName recognizes the accessor cast domains. `mm is
// month-of-year on calendar kinds and minute-of-hour on time-of-day kinds;
// the others are single-axis.
func qTemporalAccessorName(domain any) (string, bool) {
	sym, ok := domain.(data.Symbol)
	if !ok {
		return "", false
	}
	switch string(sym) {
	case "year", "mm", "dd", "hh", "ss", "week":
		return string(sym), true
	default:
		return "", false
	}
}

// qTemporalAccessorCast applies `year$ `mm$ `dd$ `hh$ `ss$ `week$ to temporal
// operands. Non-temporal operands decline so symbol enum casts keep their
// behavior.
func qTemporalAccessorCast(domain any, values any) (any, bool, error) {
	name, ok := qTemporalAccessorName(domain)
	if !ok {
		return nil, false, nil
	}
	if array, ok := values.(data.Array); ok {
		if !data.IsTemporalKind(array.Kind()) {
			return nil, false, nil
		}
		out := make([]any, array.Len())
		for i := range out {
			item, ok := array.At(i)
			if !ok {
				return nil, true, fmt.Errorf("q cast `%s row %d out of range", name, i+1)
			}
			converted, err := qTemporalAccessorScalar(name, item)
			if err != nil {
				return nil, true, err
			}
			out[i] = converted
		}
		kind := data.KindI64
		if name == "week" {
			kind = data.KindDate
		}
		column, err := data.NewColumnWithKind("_", kind, out)
		if err != nil {
			return nil, true, fmt.Errorf("q cast `%s: %w", name, err)
		}
		return column.Data, true, nil
	}
	if !qTemporalAccessorOperand(values) {
		return nil, false, nil
	}
	out, err := qTemporalAccessorScalar(name, values)
	return out, true, err
}

func qTemporalAccessorOperand(value any) bool {
	if kind, ok := data.NullKind(value); ok {
		return data.IsTemporalKind(kind)
	}
	return temporalKindOfValue(value) != ""
}

func qTemporalAccessorScalar(name string, value any) (any, error) {
	if data.IsNull(value) {
		if name == "week" {
			return data.NullForKind(data.KindDate), nil
		}
		return data.NullForKind(data.KindI64), nil
	}
	u, kind, ok := data.TemporalUnderlying(value)
	if !ok {
		return nil, fmt.Errorf("q cast `%s expects a temporal value, got %T", name, value)
	}
	// Infinities stay infinite under accessors (year$0Wd -> 0W).
	if u == math.MaxInt64 || u == -math.MaxInt64 {
		if name == "week" {
			return data.DateFromDays(u), nil
		}
		return u, nil
	}
	switch name {
	case "year":
		if year, _, _, ok := qTemporalCivil(kind, u); ok {
			return int64(year), nil
		}
	case "mm":
		switch kind {
		case data.KindMonth, data.KindDate, data.KindTimestamp, data.KindDateTime:
			if _, month, _, ok := qTemporalCivil(kind, u); ok {
				return int64(month), nil
			}
		default:
			if nanos, ok := qTemporalTimeOfDayNanos(kind, u); ok {
				return floorMod(floorDiv(nanos, temporalNanosPerMinute), 60), nil
			}
		}
	case "dd":
		switch kind {
		case data.KindDate, data.KindTimestamp, data.KindDateTime:
			if _, _, day, ok := qTemporalCivil(kind, u); ok {
				return int64(day), nil
			}
		}
	case "hh":
		if nanos, ok := qTemporalTimeOfDayNanos(kind, u); ok {
			return floorDiv(nanos, 60*temporalNanosPerMinute), nil
		}
	case "ss":
		switch kind {
		case data.KindSecond, data.KindTime, data.KindTimestamp, data.KindDateTime:
			if nanos, ok := qTemporalTimeOfDayNanos(kind, u); ok {
				return floorMod(floorDiv(nanos, temporalNanosPerSecond), 60), nil
			}
		}
	case "week":
		if days, ok := qTemporalDays(kind, u); ok {
			return data.DateFromDays(days - floorMod(days+3, 7)), nil
		}
	}
	return nil, fmt.Errorf("q cast `%s does not apply to %s values", name, kind)
}
