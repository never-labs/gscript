package q

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func parseTemporalToken(text string) (string, string, bool) {
	switch strings.ToLower(text) {
	case "0nd":
		return "date", text, true
	case "0nm":
		return "month", text, true
	case "0nz":
		return "datetime", text, true
	case "0nn":
		return "timespan", text, true
	case "0nu":
		return "minute", text, true
	case "0nv":
		return "second", text, true
	case "0nt":
		return "time", text, true
	case "0np":
		return "timestamp", text, true
	}
	switch {
	case strings.Contains(text, "D") && strings.Count(strings.SplitN(text, "D", 2)[0], ".") == 0:
		if _, err := parseQTimespan(text); err == nil {
			return "timespan", text, true
		}
	case strings.Contains(text, "T"):
		if _, err := parseQTimestamp(text); err == nil {
			return "timestamp", text, true
		}
		if _, err := parseQDatetime(text); err == nil {
			return "datetime", text, true
		}
	case strings.Contains(text, "D"):
		if _, err := parseQTimestamp(text); err == nil {
			return "timestamp", text, true
		}
	case strings.Count(text, ".") == 1 && strings.Count(text, ":") == 0:
		if _, err := parseQMonth(text); err == nil {
			return "month", text, true
		}
	case strings.Count(text, "-") == 2 && strings.Count(text, ":") == 0:
		if _, err := parseQDate(text); err == nil {
			return "date", text, true
		}
	case strings.Count(text, ".") == 2 && strings.Count(text, ":") == 0:
		if _, err := parseQDate(text); err == nil {
			return "date", text, true
		}
	case strings.Count(text, ":") == 1:
		if _, err := parseQMinute(text); err == nil {
			return "minute", text, true
		}
	case strings.Count(text, ":") >= 2 && !strings.Contains(text, "."):
		if _, err := parseQSecond(text); err == nil {
			return "second", text, true
		}
	case strings.Count(text, ":") >= 2:
		if _, err := parseQTime(text); err == nil {
			return "time", text, true
		}
	}
	return "", "", false
}

func parseTypedNullTokenKind(text string) (string, bool) {
	switch strings.ToLower(text) {
	case "0nb":
		return "bool", true
	case "0nx":
		return "u8", true
	case "0nc":
		return "string", true
	case "0nh":
		return "i16", true
	case "0ni":
		return "i32", true
	case "0nj":
		return "i64", true
	case "0ne":
		return "f32", true
	case "0nf":
		return "f64", true
	case "0nm":
		return "month", true
	case "0nd":
		return "date", true
	case "0nz":
		return "datetime", true
	case "0nn":
		return "timespan", true
	case "0nu":
		return "minute", true
	case "0nv":
		return "second", true
	case "0nt":
		return "time", true
	case "0np":
		return "timestamp", true
	default:
		return "", false
	}
}

func parseQTemporal(kind, text string) (any, error) {
	return ParseTemporal(kind, text)
}

func parseTemporalAtomOrVector(text string) (any, bool, error) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil, false, nil
	}
	values := make([]any, len(fields))
	kinds := make([]data.Kind, len(fields))
	for i, field := range fields {
		fieldKind, fieldText, ok := parseTemporalToken(field)
		if !ok {
			return nil, false, nil
		}
		value, err := parseQTemporal(fieldKind, fieldText)
		if err != nil {
			return nil, true, err
		}
		values[i] = value
		kinds[i] = data.Kind(fieldKind)
	}
	if len(values) == 1 {
		return values[0], true, nil
	}
	kind, err := coerceTemporalVectorValues(values, kinds)
	if err != nil {
		return nil, true, err
	}
	column, err := data.NewColumnWithKind("q", kind, values)
	if err != nil {
		return nil, true, err
	}
	return column.Data, true, nil
}

func coerceTemporalVectorValues(values []any, explicit []data.Kind) (data.Kind, error) {
	target := data.Kind("")
	for i, value := range values {
		kind := data.Kind("")
		if i < len(explicit) {
			kind = explicit[i]
		}
		if kind == "" {
			kind = temporalKindOfValue(value)
		}
		if kind == "" || kind == data.KindNull {
			continue
		}
		if target == "" {
			target = kind
			continue
		}
		merged, ok := mergeTemporalKinds(target, kind)
		if !ok {
			return "", fmt.Errorf("mixed temporal vector kinds are not supported")
		}
		target = merged
	}
	if target == "" {
		return data.KindNull, nil
	}
	for i, value := range values {
		if data.IsNull(value) {
			continue
		}
		coerced, ok := coerceTemporalValue(target, value)
		if !ok {
			return "", fmt.Errorf("mixed temporal vector kinds are not supported")
		}
		values[i] = coerced
	}
	return target, nil
}

func temporalKindOfValue(v any) data.Kind {
	switch v.(type) {
	case data.Month:
		return data.KindMonth
	case data.Date:
		return data.KindDate
	case data.DateTime:
		return data.KindDateTime
	case data.Timespan:
		return data.KindTimespan
	case data.Minute:
		return data.KindMinute
	case data.Second:
		return data.KindSecond
	case data.Time:
		return data.KindTime
	case data.Timestamp:
		return data.KindTimestamp
	default:
		return ""
	}
}

func mergeTemporalKinds(left, right data.Kind) (data.Kind, bool) {
	if left == right {
		return left, true
	}
	if isTimeOfDayKind(left) && isTimeOfDayKind(right) {
		if left == data.KindTime || right == data.KindTime {
			return data.KindTime, true
		}
		if left == data.KindSecond || right == data.KindSecond {
			return data.KindSecond, true
		}
		return data.KindMinute, true
	}
	return "", false
}

func isTimeOfDayKind(kind data.Kind) bool {
	return kind == data.KindMinute || kind == data.KindSecond || kind == data.KindTime
}

func coerceTemporalValue(kind data.Kind, v any) (any, bool) {
	if temporalKindOfValue(v) == kind {
		return v, true
	}
	switch kind {
	case data.KindTime:
		switch x := v.(type) {
		case data.Minute:
			return data.TimeFromNanos(x.Minutes() * 60 * 1_000_000_000), true
		case data.Second:
			return data.TimeFromNanos(x.Seconds() * 1_000_000_000), true
		}
	case data.KindSecond:
		switch x := v.(type) {
		case data.Minute:
			return data.SecondFromSeconds(x.Minutes() * 60), true
		}
	}
	return nil, false
}

func ParseTemporal(kind, text string) (any, error) {
	switch kind {
	case "month":
		if strings.EqualFold(text, "0Nm") {
			return data.NullForKind(data.KindMonth), nil
		}
		return parseQMonth(text)
	case "date":
		if strings.EqualFold(text, "0Nd") {
			return data.NullForKind(data.KindDate), nil
		}
		tm, err := parseQDate(text)
		if err != nil {
			return nil, fmt.Errorf("invalid q date %q", text)
		}
		return data.DateFromDays(tm.Unix() / 86400), nil
	case "datetime":
		if strings.EqualFold(text, "0Nz") {
			return data.NullForKind(data.KindDateTime), nil
		}
		return parseQDatetime(text)
	case "timespan":
		if strings.EqualFold(text, "0Nn") {
			return data.NullForKind(data.KindTimespan), nil
		}
		return parseQTimespan(text)
	case "minute":
		if strings.EqualFold(text, "0Nu") {
			return data.NullForKind(data.KindMinute), nil
		}
		return parseQMinute(text)
	case "second":
		if strings.EqualFold(text, "0Nv") {
			return data.NullForKind(data.KindSecond), nil
		}
		return parseQSecond(text)
	case "time":
		if strings.EqualFold(text, "0Nt") {
			return data.NullForKind(data.KindTime), nil
		}
		return parseQTime(text)
	case "timestamp":
		if strings.EqualFold(text, "0Np") {
			return data.NullForKind(data.KindTimestamp), nil
		}
		return parseQTimestamp(text)
	default:
		return nil, fmt.Errorf("unsupported q temporal kind %q", kind)
	}
}

func typedNullTokenForKind(kind string) string {
	switch kind {
	case "month":
		return "0Nm"
	case "date":
		return "0Nd"
	case "datetime":
		return "0Nz"
	case "timespan":
		return "0Nn"
	case "minute":
		return "0Nu"
	case "second":
		return "0Nv"
	case "time":
		return "0Nt"
	case "timestamp":
		return "0Np"
	default:
		return "0N?"
	}
}

func parseQMonth(text string) (data.Month, error) {
	parts := strings.Split(text, ".")
	if len(parts) == 1 {
		parts = strings.Split(text, "-")
	}
	if len(parts) != 2 || len(parts[0]) != 4 || len(parts[1]) != 2 {
		return 0, fmt.Errorf("invalid q month %q", text)
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid q month %q", text)
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return 0, fmt.Errorf("invalid q month %q", text)
	}
	return data.MonthFromMonths(int64(year-1970)*12 + int64(month-1)), nil
}

func parseQDate(text string) (time.Time, error) {
	if tm, err := time.Parse("2006.01.02", text); err == nil {
		return tm, nil
	}
	return time.Parse("2006-01-02", text)
}

func parseQDatetime(text string) (data.DateTime, error) {
	parts := strings.SplitN(text, "T", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid q datetime %q", text)
	}
	date, err := parseQDate(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid q datetime %q", text)
	}
	tod, err := parseQTime(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid q datetime %q", text)
	}
	return data.DateTimeFromUnixNanos(date.UnixNano() + tod.Nanos()), nil
}

func parseQTimespan(text string) (data.Timespan, error) {
	parts := strings.SplitN(text, "D", 2)
	if len(parts) != 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid q timespan %q", text)
	}
	negative := strings.HasPrefix(parts[0], "-")
	dayText := strings.TrimPrefix(strings.TrimPrefix(parts[0], "-"), "+")
	if dayText == "" {
		return 0, fmt.Errorf("invalid q timespan %q", text)
	}
	days, err := strconv.ParseInt(dayText, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid q timespan %q", text)
	}
	tod, err := parseQTime(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid q timespan %q", text)
	}
	nanos := days*24*60*60*1_000_000_000 + tod.Nanos()
	if negative {
		nanos = -nanos
	}
	return data.TimespanFromNanos(nanos), nil
}

func parseQMinute(text string) (data.Minute, error) {
	tm, err := time.Parse("15:04", text)
	if err != nil {
		return 0, fmt.Errorf("invalid q minute %q", text)
	}
	minutes := int64(tm.Hour())*60 + int64(tm.Minute())
	return data.MinuteFromMinutes(minutes), nil
}

func parseQSecond(text string) (data.Second, error) {
	tm, err := time.Parse("15:04:05", text)
	if err != nil {
		return 0, fmt.Errorf("invalid q second %q", text)
	}
	seconds := int64(tm.Hour())*3600 + int64(tm.Minute())*60 + int64(tm.Second())
	return data.SecondFromSeconds(seconds), nil
}

func parseQTime(text string) (data.Time, error) {
	if dot := strings.LastIndexByte(text, '.'); dot >= 0 && len(text)-dot-1 > 9 {
		return 0, fmt.Errorf("invalid q time %q", text)
	}
	for _, layout := range []string{"15:04:05.999999999", "15:04:05.999999", "15:04:05.999", "15:04:05"} {
		tm, err := time.Parse(layout, text)
		if err == nil {
			nanos := int64(tm.Hour())*3600*1_000_000_000 + int64(tm.Minute())*60*1_000_000_000 + int64(tm.Second())*1_000_000_000 + int64(tm.Nanosecond())
			return data.TimeFromNanos(nanos), nil
		}
	}
	return 0, fmt.Errorf("invalid q time %q", text)
}

func parseQTimestamp(text string) (data.Timestamp, error) {
	if strings.Contains(text, "T") {
		tm, err := time.Parse(time.RFC3339Nano, text)
		if err == nil {
			return data.TimestampFromUnixNanos(tm.UnixNano()), nil
		}
		for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05"} {
			tm, err := time.Parse(layout, text)
			if err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), nil
			}
		}
	}
	parts := strings.SplitN(text, "D", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid q timestamp %q", text)
	}
	date, err := parseQDate(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid q timestamp %q", text)
	}
	tod, err := parseQTime(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid q timestamp %q", text)
	}
	return data.TimestampFromUnixNanos(date.UnixNano() + tod.Nanos()), nil
}

func FormatTemporal(v any) (string, bool) {
	switch x := v.(type) {
	case data.Month:
		months := x.Months()
		year := int64(1970) + floorDiv(months, 12)
		month := months - floorDiv(months, 12)*12 + 1
		return fmt.Sprintf("%04d.%02d", year, month), true
	case data.Date:
		return time.Unix(x.Days()*86400, 0).UTC().Format("2006-01-02"), true
	case data.DateTime:
		tm := time.Unix(0, x.UnixNanos()).UTC()
		return tm.Format("2006-01-02") + "T" + formatQTimeNanos(int64(tm.Hour())*3600*1_000_000_000+int64(tm.Minute())*60*1_000_000_000+int64(tm.Second())*1_000_000_000+int64(tm.Nanosecond())), true
	case data.Timespan:
		return formatQTimespanNanos(x.Nanos()), true
	case data.Minute:
		return fmt.Sprintf("%02d:%02d", x.Minutes()/60, x.Minutes()%60), true
	case data.Second:
		seconds := x.Seconds()
		return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60), true
	case data.Time:
		return formatQTimeNanos(x.Nanos()), true
	case data.Timestamp:
		tm := time.Unix(0, x.UnixNanos()).UTC()
		return tm.Format("2006-01-02") + "T" + formatQTimeNanos(int64(tm.Hour())*3600*1_000_000_000+int64(tm.Minute())*60*1_000_000_000+int64(tm.Second())*1_000_000_000+int64(tm.Nanosecond())) + "Z", true
	default:
		return "", false
	}
}

func floorDiv(left, right int64) int64 {
	q := left / right
	r := left % right
	if r != 0 && ((r < 0) != (right < 0)) {
		q--
	}
	return q
}

func formatQTimespanNanos(nanos int64) string {
	negative := nanos < 0
	magnitude := absNanos(nanos)
	dayNanos := uint64(24 * 60 * 60 * 1_000_000_000)
	days := magnitude / dayNanos
	magnitude -= days * dayNanos
	text := fmt.Sprintf("%dD%s", days, formatQTimeNanosUnsigned(magnitude))
	if negative {
		return "-" + text
	}
	return text
}

func absNanos(nanos int64) uint64 {
	if nanos < 0 {
		return uint64(-(nanos + 1)) + 1
	}
	return uint64(nanos)
}

func formatQTimeNanosUnsigned(nanos uint64) string {
	hour := nanos / (3600 * 1_000_000_000)
	nanos -= hour * 3600 * 1_000_000_000
	minute := nanos / (60 * 1_000_000_000)
	nanos -= minute * 60 * 1_000_000_000
	second := nanos / 1_000_000_000
	nanos -= second * 1_000_000_000
	if nanos == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	}
	frac := fmt.Sprintf("%09d", nanos)
	frac = strings.TrimRight(frac, "0")
	for len(frac) < 3 {
		frac += "0"
	}
	return fmt.Sprintf("%02d:%02d:%02d.%s", hour, minute, second, frac)
}

func formatQTimeNanos(nanos int64) string {
	if nanos < 0 {
		nanos = 0
	}
	hour := nanos / (3600 * 1_000_000_000)
	nanos -= hour * 3600 * 1_000_000_000
	minute := nanos / (60 * 1_000_000_000)
	nanos -= minute * 60 * 1_000_000_000
	second := nanos / 1_000_000_000
	nanos -= second * 1_000_000_000
	if nanos == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	}
	frac := fmt.Sprintf("%09d", nanos)
	frac = strings.TrimRight(frac, "0")
	for len(frac) < 3 {
		frac += "0"
	}
	return fmt.Sprintf("%02d:%02d:%02d.%s", hour, minute, second, frac)
}
