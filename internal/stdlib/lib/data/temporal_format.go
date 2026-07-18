package data

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// FormatTemporal formats data temporal scalars with the stable textual form
// used by Leia's data facade. The representation intentionally mirrors the
// q-compatible temporal spelling for existing persisted and displayed values,
// but it belongs to the data foundation so generic data code does not depend on
// the optional q extension.
func FormatTemporal(v any) (string, bool) {
	if n, kind, ok := TemporalUnderlying(v); ok {
		if token, ok := temporalInfinityToken(kind, n); ok {
			return token, true
		}
	}
	switch x := v.(type) {
	case Month:
		months := x.Months()
		year := int64(1970) + floorDiv(months, 12)
		month := months - floorDiv(months, 12)*12 + 1
		return fmt.Sprintf("%04d.%02d", year, month), true
	case Date:
		return time.Unix(x.Days()*86400, 0).UTC().Format("2006-01-02"), true
	case DateTime:
		tm := time.Unix(0, x.UnixNanos()).UTC()
		nanos := int64(tm.Hour())*3600*1_000_000_000 + int64(tm.Minute())*60*1_000_000_000 + int64(tm.Second())*1_000_000_000 + int64(tm.Nanosecond())
		return tm.Format("2006-01-02") + "T" + formatTimeNanos(nanos), true
	case Timespan:
		return formatTimespanNanos(x.Nanos()), true
	case Minute:
		minutes := x.Minutes()
		sign := ""
		if minutes < 0 {
			sign, minutes = "-", -minutes
		}
		return fmt.Sprintf("%s%02d:%02d", sign, minutes/60, minutes%60), true
	case Second:
		seconds := x.Seconds()
		sign := ""
		if seconds < 0 {
			sign, seconds = "-", -seconds
		}
		return fmt.Sprintf("%s%02d:%02d:%02d", sign, seconds/3600, seconds/60%60, seconds%60), true
	case Time:
		if nanos := x.Nanos(); nanos < 0 {
			return "-" + formatTimeNanosUnsigned(absNanos(nanos)), true
		}
		return formatTimeNanos(x.Nanos()), true
	case Timestamp:
		tm := time.Unix(0, x.UnixNanos()).UTC()
		nanos := int64(tm.Hour())*3600*1_000_000_000 + int64(tm.Minute())*60*1_000_000_000 + int64(tm.Second())*1_000_000_000 + int64(tm.Nanosecond())
		return tm.Format("2006-01-02") + "T" + formatTimeNanos(nanos) + "Z", true
	default:
		return "", false
	}
}

func temporalInfinityToken(kind Kind, n int64) (string, bool) {
	var suffix byte
	switch kind {
	case KindMonth:
		suffix = 'm'
	case KindDate:
		suffix = 'd'
	case KindDateTime:
		suffix = 'z'
	case KindTimespan:
		suffix = 'n'
	case KindMinute:
		suffix = 'u'
	case KindSecond:
		suffix = 'v'
	case KindTime:
		suffix = 't'
	case KindTimestamp:
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

func floorDiv(left, right int64) int64 {
	q := left / right
	r := left % right
	if r != 0 && ((r < 0) != (right < 0)) {
		q--
	}
	return q
}

func formatTimespanNanos(nanos int64) string {
	negative := nanos < 0
	magnitude := absNanos(nanos)
	dayNanos := uint64(24 * 60 * 60 * 1_000_000_000)
	days := magnitude / dayNanos
	magnitude -= days * dayNanos
	text := fmt.Sprintf("%dD%s", days, formatTimeNanosUnsigned(magnitude))
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

func formatTimeNanosUnsigned(nanos uint64) string {
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

func formatTimeNanos(nanos int64) string {
	if nanos < 0 {
		nanos = 0
	}
	return formatTimeNanosUnsigned(uint64(nanos))
}
