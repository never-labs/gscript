package bind

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

const qSymbolVectorMarker = "__q_symbol_vector"
const qKeyedFrameMarker = "__q_keyed_frame"
const qDictKeysMarker = "__q_dict_keys"

func dataArrayHasNull(array data.Array) bool {
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok || data.IsNull(item) {
			return true
		}
	}
	return false
}

func qDataFrameFromSoA(s *SoA) (data.Frame, error) {
	if s == nil {
		return data.Frame{}, fmt.Errorf("soa is nil")
	}
	cols := make([]data.Column, 0, len(s.ColumnNames()))
	for _, name := range s.ColumnNames() {
		col, ok := s.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("soa column %q not found", name)
		}
		array, err := qDataArrayFromDense(col)
		if err != nil {
			return data.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, data.Column{Name: data.Symbol(name), Data: array})
	}
	return data.NewFrame(cols...)
}

func qDataFrameFromSoABorrowed(s *SoA) (data.Frame, error) {
	if s == nil {
		return data.Frame{}, fmt.Errorf("soa is nil")
	}
	names := s.ColumnNames()
	cols := make([]data.Column, 0, len(names))
	for _, name := range names {
		col, ok := s.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("soa column %q not found", name)
		}
		array, err := qDataArrayFromDenseBorrowed(col)
		if err != nil {
			return data.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, data.Column{Name: data.Symbol(name), Data: array})
	}
	return data.NewFrame(cols...)
}

func qDataArrayFromDenseBorrowed(col *DenseArray) (data.Array, error) {
	if col == nil {
		return nil, fmt.Errorf("dense array is nil")
	}
	switch col.DType() {
	case DenseArrayI64:
		xs, ok := col.I64()
		if !ok {
			return nil, fmt.Errorf("i64 dense array storage unavailable")
		}
		return data.NewI64Borrowed(xs), nil
	case DenseArrayF64:
		xs, ok := col.F64()
		if !ok {
			return nil, fmt.Errorf("f64 dense array storage unavailable")
		}
		return data.NewF64Borrowed(xs), nil
	case DenseArrayBool:
		xs, ok := col.Bool()
		if !ok {
			return nil, fmt.Errorf("bool dense array storage unavailable")
		}
		return data.NewBoolBorrowed(xs), nil
	case DenseArrayString:
		xs, ok := col.StringValues()
		if !ok {
			return nil, fmt.Errorf("string dense array storage unavailable")
		}
		return data.NewStringBorrowed(xs), nil
	default:
		return nil, fmt.Errorf("unsupported dense array dtype %s", col.DType())
	}
}

func qDataArrayFromDense(col *DenseArray) (data.Array, error) {
	switch col.DType() {
	case DenseArrayF64:
		xs := make([]float64, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Number()
		}
		return data.NewF64(xs), nil
	case DenseArrayI64:
		xs := make([]int64, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Int()
		}
		return data.NewI64(xs), nil
	case DenseArrayBool:
		xs := make([]bool, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Bool()
		}
		return data.NewBool(xs), nil
	case DenseArrayString:
		xs := make([]string, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Str()
		}
		return data.NewString(xs), nil
	default:
		return nil, fmt.Errorf("unsupported dense array type %s", col.DType())
	}
}

func qPlainStringDictionaryKeyOrder(tbl *Table) ([]data.Symbol, bool) {
	if tbl == nil || tbl.Length() != 0 {
		return nil, false
	}
	keys := make([]data.Symbol, 0)
	tbl.ForEachPlainRaw(func(key, _ Value) bool {
		if key.IsString() {
			keys = append(keys, data.Symbol(key.Str()))
		}
		return true
	})
	if len(keys) == 0 {
		return nil, false
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys, true
}

func qLooksLikeFrame(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind, ok := qNativeFrameRuntimeKind(tbl); ok {
		return kind == NativePayloadDataFrame
	}
	return isDataFrameTable(tbl) || qLooksLikeScriptDataFrameFacade(tbl)
}

func qLooksLikeScriptDataFrameFacade(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind := tbl.RawGetString("kind"); kind.IsString() && kind.Str() == "data_frame" {
		return true
	}
	if typ := tbl.RawGetString("type"); typ.IsString() && typ.Str() == "data_frame" {
		return true
	}
	return tbl.RawGetString("columns").IsTable() && tbl.RawGetString("column_names").IsTable()
}

func qIsKeyedFrameTable(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind, ok := qNativeFrameRuntimeKind(tbl); ok {
		return kind == NativePayloadKeyedFrame
	}
	return tbl.RawGetString(qKeyedFrameMarker).Truthy()
}

func qNativeFrameRuntimeKind(tbl *Table) (NativePayloadKind, bool) {
	if tbl == nil {
		return NativePayloadNone, false
	}
	if _, info, ok := qTypedNativeFramePayload(tbl); ok {
		return info.Kind, true
	}
	return qLegacyNativeFramePayloadKind(tbl)
}

func qNativeFrameRuntimeKindMatches(tbl *Table, want NativePayloadKind) bool {
	kind, ok := qNativeFrameRuntimeKind(tbl)
	return ok && kind == want
}

func qTypedNativeFramePayload(tbl *Table) (any, NativePayloadInfo, bool) {
	if tbl == nil {
		return nil, NativePayloadInfo{}, false
	}
	return tbl.NativeFramePayload()
}

func qLegacyNativeFramePayloadKind(tbl *Table) (NativePayloadKind, bool) {
	if tbl == nil {
		return NativePayloadNone, false
	}
	if _, hasInfo := tbl.NativePayloadInfo(); hasInfo {
		return NativePayloadNone, false
	}
	switch tbl.NativePayload().(type) {
	case data.Frame:
		return NativePayloadDataFrame, true
	case data.KeyedFrame:
		return NativePayloadKeyedFrame, true
	default:
		return NativePayloadNone, false
	}
}

func qParseTemporalAny(kind data.Kind, v any) (any, bool) {
	switch kind {
	case data.KindMonth:
		switch x := v.(type) {
		case data.Month:
			return x, true
		case int64:
			return data.MonthFromMonths(x), true
		case string:
			for _, layout := range []string{"2006.01", "2006-01"} {
				if tm, err := time.Parse(layout, x); err == nil {
					return data.MonthFromMonths(int64((tm.Year()-1970)*12 + int(tm.Month()) - 1)), true
				}
			}
		}
	case data.KindDate:
		switch x := v.(type) {
		case data.Date:
			return x, true
		case int64:
			return data.DateFromDays(x), true
		case string:
			for _, layout := range []string{"2006-01-02", "2006.01.02"} {
				if tm, err := time.Parse(layout, x); err == nil {
					return data.DateFromDays(tm.Unix() / 86400), true
				}
			}
		}
	case data.KindDateTime:
		switch x := v.(type) {
		case data.DateTime:
			return x, true
		case int64:
			return data.DateTimeFromUnixNanos(x), true
		case string:
			for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006.01.02T15:04:05.999999999", "2006.01.02T15:04:05"} {
				if tm, err := time.Parse(layout, x); err == nil {
					return data.DateTimeFromUnixNanos(tm.UnixNano()), true
				}
			}
		}
	case data.KindTimespan:
		switch x := v.(type) {
		case data.Timespan:
			return x, true
		case int64:
			return data.TimespanFromNanos(x), true
		case string:
			if nanos, ok := dataParseTimespanNanos(x); ok {
				return data.TimespanFromNanos(nanos), true
			}
		}
	case data.KindMinute:
		switch x := v.(type) {
		case data.Minute:
			return x, true
		case int64:
			return data.MinuteFromMinutes(x), true
		case string:
			if nanos, ok := dataParseTimeOfDayNanos(x); ok && nanos%(60*1_000_000_000) == 0 {
				return data.MinuteFromMinutes(nanos / (60 * 1_000_000_000)), true
			}
		}
	case data.KindSecond:
		switch x := v.(type) {
		case data.Second:
			return x, true
		case int64:
			return data.SecondFromSeconds(x), true
		case string:
			if nanos, ok := dataParseTimeOfDayNanos(x); ok && nanos%1_000_000_000 == 0 {
				return data.SecondFromSeconds(nanos / 1_000_000_000), true
			}
		}
	case data.KindTime:
		switch x := v.(type) {
		case data.Time:
			return x, true
		case int64:
			return data.TimeFromNanos(x), true
		case string:
			if nanos, ok := dataParseTimeOfDayNanos(x); ok {
				return data.TimeFromNanos(nanos), true
			}
		}
	case data.KindTimestamp:
		switch x := v.(type) {
		case data.Timestamp:
			return x, true
		case int64:
			return data.TimestampFromUnixNanos(x), true
		case string:
			for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006.01.02D15:04:05.999999999", "2006.01.02D15:04:05"} {
				if tm, err := time.Parse(layout, x); err == nil {
					return data.TimestampFromUnixNanos(tm.UnixNano()), true
				}
			}
		}
	}
	return nil, false
}

func dataParseTimeOfDayNanos(s string) (int64, bool) {
	for _, layout := range []string{"15:04:05.999999999", "15:04:05.999999", "15:04:05.999", "15:04:05", "15:04"} {
		if tm, err := time.Parse(layout, s); err == nil {
			return int64(tm.Hour())*3600*1_000_000_000 + int64(tm.Minute())*60*1_000_000_000 + int64(tm.Second())*1_000_000_000 + int64(tm.Nanosecond()), true
		}
	}
	return 0, false
}

func dataParseTimespanNanos(s string) (int64, bool) {
	sign := int64(1)
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = strings.TrimPrefix(s, "-")
	} else {
		s = strings.TrimPrefix(s, "+")
	}
	days := int64(0)
	if parts := strings.SplitN(s, "D", 2); len(parts) == 2 {
		n, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, false
		}
		days = n
		s = parts[1]
	}
	nanos, ok := dataParseTimeOfDayNanos(s)
	if !ok {
		return 0, false
	}
	return sign * (days*24*60*60*1_000_000_000 + nanos), true
}
