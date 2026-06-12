package q

import (
	"fmt"
	"math"
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
	if kind, ok := temporalInfinityTokenKind(text); ok {
		return kind, text, true
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
		// Shape pre-check (4-char year, 2-char month) keeps the rejection
		// path allocation-free: plain float literals like "2.5" otherwise
		// build a fmt.Errorf inside parseQMonth on every warm eval.
		if qMonthTokenShape(text) {
			if _, err := parseQMonth(text); err == nil {
				return "month", text, true
			}
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

// temporalInfinityTokenKind recognizes the temporal infinity literals 0Wd 0Wm
// 0Wz 0Wn 0Wu 0Wv 0Wt 0Wp and their negative forms, returning the temporal
// kind name.
func temporalInfinityTokenKind(text string) (string, bool) {
	body := strings.TrimPrefix(text, "-")
	if len(body) != 3 || body[0] != '0' || (body[1] != 'W' && body[1] != 'w') {
		return "", false
	}
	switch body[2] {
	case 'd':
		return "date", true
	case 'm':
		return "month", true
	case 'z':
		return "datetime", true
	case 'n':
		return "timespan", true
	case 'u':
		return "minute", true
	case 'v':
		return "second", true
	case 't':
		return "time", true
	case 'p':
		return "timestamp", true
	default:
		return "", false
	}
}

// parseTemporalInfinity builds the infinity value for a recognized 0W*/-0W*
// temporal token. Temporal infinities are the extreme underlying encodings, so
// they sort greatest (least when negative) within their kind and wrap under
// arithmetic exactly like integer 0W does.
func parseTemporalInfinity(text string) (any, bool) {
	kindName, ok := temporalInfinityTokenKind(text)
	if !ok {
		return nil, false
	}
	n := int64(math.MaxInt64)
	if strings.HasPrefix(text, "-") {
		n = -n
	}
	value, ok := data.NewTemporalValue(data.Kind(kindName), n)
	if !ok {
		return nil, false
	}
	return value, true
}

// temporalInfinityToken formats a temporal infinity back to its literal
// token, mirroring typedNullTokenForKind.
func temporalInfinityToken(kind data.Kind, n int64) (string, bool) {
	var suffix byte
	switch kind {
	case data.KindMonth:
		suffix = 'm'
	case data.KindDate:
		suffix = 'd'
	case data.KindDateTime:
		suffix = 'z'
	case data.KindTimespan:
		suffix = 'n'
	case data.KindMinute:
		suffix = 'u'
	case data.KindSecond:
		suffix = 'v'
	case data.KindTime:
		suffix = 't'
	case data.KindTimestamp:
		suffix = 'p'
	default:
		return "", false
	}
	switch n {
	case math.MaxInt64:
		return "0W" + string(suffix), true
	case -math.MaxInt64:
		return "-0W" + string(suffix), true
	default:
		return "", false
	}
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
	if value, ok := parseTemporalInfinity(text); ok {
		return value, nil
	}
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
	text = normalizeQMonthText(text)
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

// qMonthTokenShape mirrors parseQMonth's structural requirement (a 4-char
// year part and 2-char month part around a single '.' or '-', optionally
// suffixed with 'm') so callers can reject non-month tokens without paying
// parseQMonth's error allocation.
func qMonthTokenShape(text string) bool {
	text = normalizeQMonthText(text)
	sep := strings.IndexByte(text, '.')
	if sep < 0 {
		sep = strings.IndexByte(text, '-')
	}
	return sep == 4 && len(text) == 7
}

func normalizeQMonthText(text string) string {
	if len(text) > 0 && text[len(text)-1] == 'm' {
		return text[:len(text)-1]
	}
	return text
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
	if n, kind, ok := data.TemporalUnderlying(v); ok {
		if token, ok := temporalInfinityToken(kind, n); ok {
			return token, true
		}
	}
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
		minutes := x.Minutes()
		sign := ""
		if minutes < 0 {
			sign, minutes = "-", -minutes
		}
		return fmt.Sprintf("%s%02d:%02d", sign, minutes/60, minutes%60), true
	case data.Second:
		seconds := x.Seconds()
		sign := ""
		if seconds < 0 {
			sign, seconds = "-", -seconds
		}
		return fmt.Sprintf("%s%02d:%02d:%02d", sign, seconds/3600, seconds/60%60, seconds%60), true
	case data.Time:
		if nanos := x.Nanos(); nanos < 0 {
			return "-" + formatQTimeNanosUnsigned(absNanos(nanos)), true
		}
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

// --- temporal dyadic arithmetic ----------------------------------------------
//
// Canonical kdb+/q temporal arithmetic on scalars. The rules implemented here
// are the ones the canonical harness pins:
//
//	temporal + int / int + temporal -> same kind (int scaled per kind:
//	    days for dates, months for months, minutes/seconds for those kinds,
//	    milliseconds for times, nanoseconds for timestamps/timespans, days
//	    for datetimes)
//	temporal - int                  -> same kind
//	temporal - temporal (same kind) -> span: date/month -> long, timestamp/
//	    timespan/datetime -> timespan, minute/second/time -> same kind
//	date + time-of-day / timespan   -> timestamp (and date - timespan)
//	timestamp ± timespan            -> timestamp
//	timestamp - date, date - timestamp -> timespan
//	time-of-day ± time-of-day       -> finer of the two kinds
//
// Anything else (floats, int - temporal, month/date sums, datetime mixes)
// stays unhandled so the caller's numeric-operand error is preserved.

const (
	temporalNanosPerDay    int64 = 24 * 60 * 60 * 1_000_000_000
	temporalNanosPerMinute int64 = 60 * 1_000_000_000
	temporalNanosPerSecond int64 = 1_000_000_000
)

// applyTemporalDyadic handles + and - when at least one operand is a temporal
// scalar. handled=false means the operand shapes are not temporal arithmetic
// and the caller should keep its existing dispatch.
func applyTemporalDyadic(op byte, left, right any) (any, bool, error) {
	if op != '+' && op != '-' {
		return nil, false, nil
	}
	lu, lk, lok := data.TemporalUnderlying(left)
	ru, rk, rok := data.TemporalUnderlying(right)
	switch {
	case lok && !rok:
		n, ok := temporalIntegerOperand(right)
		if !ok {
			return nil, false, nil
		}
		scale, ok := data.TemporalIntUnitScale(lk)
		if !ok {
			return nil, false, nil
		}
		if op == '-' {
			n = -n
		}
		value, ok := data.NewTemporalValue(lk, lu+n*scale)
		return value, ok, nil
	case !lok && rok:
		if op != '+' {
			return nil, false, nil
		}
		n, ok := temporalIntegerOperand(left)
		if !ok {
			return nil, false, nil
		}
		scale, ok := data.TemporalIntUnitScale(rk)
		if !ok {
			return nil, false, nil
		}
		value, ok := data.NewTemporalValue(rk, ru+n*scale)
		return value, ok, nil
	case lok && rok:
		return applyTemporalPairDyadic(op, lu, lk, ru, rk)
	default:
		return nil, false, nil
	}
}

func temporalIntegerOperand(v any) (int64, bool) {
	if b, ok := v.(bool); ok {
		if b {
			return 1, true
		}
		return 0, true
	}
	return integerValue(v)
}

func applyTemporalPairDyadic(op byte, lu int64, lk data.Kind, ru int64, rk data.Kind) (any, bool, error) {
	if lk == rk {
		switch op {
		case '-':
			span, ok := data.TemporalSpanKind(lk)
			if !ok {
				return nil, false, nil
			}
			if span == data.KindI64 {
				return lu - ru, true, nil
			}
			value, ok := data.NewTemporalValue(span, lu-ru)
			return value, ok, nil
		case '+':
			switch lk {
			case data.KindMinute, data.KindSecond, data.KindTime, data.KindTimespan:
				value, ok := data.NewTemporalValue(lk, lu+ru)
				return value, ok, nil
			}
			return nil, false, nil
		}
		return nil, false, nil
	}
	if isTimeOfDayKind(lk) && isTimeOfDayKind(rk) {
		merged, ok := mergeTemporalKinds(lk, rk)
		if !ok {
			return nil, false, nil
		}
		lm := temporalTimeOfDayInUnits(lu, lk, merged)
		rm := temporalTimeOfDayInUnits(ru, rk, merged)
		n := lm + rm
		if op == '-' {
			n = lm - rm
		}
		value, ok := data.NewTemporalValue(merged, n)
		return value, ok, nil
	}
	switch op {
	case '+':
		switch {
		case lk == data.KindDate && (isTimeOfDayKind(rk) || rk == data.KindTimespan):
			value, ok := data.NewTemporalValue(data.KindTimestamp, lu*temporalNanosPerDay+temporalNanos(ru, rk))
			return value, ok, nil
		case rk == data.KindDate && (isTimeOfDayKind(lk) || lk == data.KindTimespan):
			value, ok := data.NewTemporalValue(data.KindTimestamp, ru*temporalNanosPerDay+temporalNanos(lu, lk))
			return value, ok, nil
		case lk == data.KindTimestamp && rk == data.KindTimespan:
			value, ok := data.NewTemporalValue(data.KindTimestamp, lu+ru)
			return value, ok, nil
		case lk == data.KindTimespan && rk == data.KindTimestamp:
			value, ok := data.NewTemporalValue(data.KindTimestamp, lu+ru)
			return value, ok, nil
		}
	case '-':
		switch {
		case lk == data.KindTimestamp && rk == data.KindTimespan:
			value, ok := data.NewTemporalValue(data.KindTimestamp, lu-ru)
			return value, ok, nil
		case lk == data.KindDate && rk == data.KindTimespan:
			value, ok := data.NewTemporalValue(data.KindTimestamp, lu*temporalNanosPerDay-ru)
			return value, ok, nil
		case lk == data.KindTimestamp && rk == data.KindDate:
			value, ok := data.NewTemporalValue(data.KindTimespan, lu-ru*temporalNanosPerDay)
			return value, ok, nil
		case lk == data.KindDate && rk == data.KindTimestamp:
			value, ok := data.NewTemporalValue(data.KindTimespan, lu*temporalNanosPerDay-ru)
			return value, ok, nil
		}
	}
	return nil, false, nil
}

// temporalTimeOfDayInUnits converts a time-of-day underlying count from one
// time-of-day kind into another (only coarse -> fine conversions occur:
// minute -> second/time, second -> time).
func temporalTimeOfDayInUnits(n int64, from, to data.Kind) int64 {
	if from == to {
		return n
	}
	switch {
	case from == data.KindMinute && to == data.KindSecond:
		return n * 60
	case from == data.KindMinute && to == data.KindTime:
		return n * temporalNanosPerMinute
	case from == data.KindSecond && to == data.KindTime:
		return n * temporalNanosPerSecond
	default:
		return n
	}
}

// temporalNanos converts a time-of-day or timespan underlying count to
// nanoseconds.
func temporalNanos(n int64, kind data.Kind) int64 {
	switch kind {
	case data.KindMinute:
		return n * temporalNanosPerMinute
	case data.KindSecond:
		return n * temporalNanosPerSecond
	default: // time, timespan already store nanoseconds
		return n
	}
}

// temporalDyadicResultKind mirrors applyTemporalDyadic for null propagation:
// it returns the result kind of op over the two operand kinds when temporal
// arithmetic applies, so typed temporal nulls survive + and -.
func temporalDyadicResultKind(op byte, lk, rk data.Kind) (data.Kind, bool) {
	if op != '+' && op != '-' {
		return "", false
	}
	lt := data.IsTemporalKind(lk)
	rt := data.IsTemporalKind(rk)
	switch {
	case lt && !rt:
		if temporalIntLikeKind(rk) {
			return lk, true
		}
		return "", false
	case rt && !lt:
		if op == '+' && temporalIntLikeKind(lk) {
			return rk, true
		}
		return "", false
	case lt && rt:
		if lk == rk {
			if op == '-' {
				return data.TemporalSpanKind(lk)
			}
			switch lk {
			case data.KindMinute, data.KindSecond, data.KindTime, data.KindTimespan:
				return lk, true
			}
			return "", false
		}
		if isTimeOfDayKind(lk) && isTimeOfDayKind(rk) {
			return mergeTemporalKinds(lk, rk)
		}
		switch op {
		case '+':
			switch {
			case lk == data.KindDate && (isTimeOfDayKind(rk) || rk == data.KindTimespan),
				rk == data.KindDate && (isTimeOfDayKind(lk) || lk == data.KindTimespan),
				lk == data.KindTimestamp && rk == data.KindTimespan,
				lk == data.KindTimespan && rk == data.KindTimestamp:
				return data.KindTimestamp, true
			}
		case '-':
			switch {
			case lk == data.KindTimestamp && rk == data.KindTimespan,
				lk == data.KindDate && rk == data.KindTimespan:
				return data.KindTimestamp, true
			case lk == data.KindTimestamp && rk == data.KindDate,
				lk == data.KindDate && rk == data.KindTimestamp:
				return data.KindTimespan, true
			}
		}
		return "", false
	default:
		return "", false
	}
}

// temporalIntLikeKind reports kinds that act as plain integer steps in
// temporal arithmetic, including null/any so typed temporal nulls propagate
// against untyped nulls.
func temporalIntLikeKind(kind data.Kind) bool {
	switch kind {
	case data.KindNull, data.KindAny, data.KindBool:
		return true
	default:
		return qKindIsInteger(kind)
	}
}
