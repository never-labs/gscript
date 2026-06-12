package q

// Temporal arithmetic coverage.
//
//  1. temporal ± int (and int + temporal) keeps the kind, stepping in the
//     kind's q unit (days, months, minutes, seconds, milliseconds for time,
//     nanoseconds for timestamp/timespan)
//  2. same-kind differences produce the span type: date/month -> long,
//     timestamp/timespan -> timespan, minute/second/time -> same kind
//  3. mixed-kind promotions: date+time-of-day/timespan -> timestamp,
//     timestamp ± timespan -> timestamp, timestamp-date -> timespan,
//     minute/second/time merge to the finer kind
//  4. typed temporal nulls propagate (0Nd+1 -> 0Nd) and temporal infinities
//     (0Wd, -0Wp, ...) parse, sort extreme, and survive accessor casts
//  5. accessor casts `year `mm `dd `hh `ss `week and temporal kind casts
//     (`month$d, `minute$t, `date$p, ...)
//  6. dense temporal vectors stay typed columns through the dyadic kernels
//
// Statements that are compiler-representable also run through
// assertBothRoutes so the compiled route and string evaluator agree.

import (
	"math"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestTemporalScalarArithmetic(t *testing.T) {
	// date ± int (days), int + date commutes.
	assertEvalValue(t, "2024.01.31+1", data.DateFromDays(19754))
	assertEvalValue(t, "1+2024.01.31", data.DateFromDays(19754))
	assertEvalValue(t, "2024.01.01-1", data.DateFromDays(19722))
	// month ± int (months).
	assertEvalValue(t, "2024.05m+1", data.MonthFromMonths(653))
	assertEvalValue(t, "2024.05m-1", data.MonthFromMonths(651))
	// minute/second ± int in their own resolution; no 24h wrap (q shows 24:01).
	assertEvalValue(t, "12:30+30", data.MinuteFromMinutes(780))
	assertEvalValue(t, "12:30-31", data.MinuteFromMinutes(719))
	assertEvalValue(t, "23:59+2", data.MinuteFromMinutes(1441))
	assertEvalValue(t, "12:30:00+30", data.SecondFromSeconds(45030))
	// time ± int steps in milliseconds (q time resolution).
	assertEvalValue(t, "12:00:00.000+1", data.TimeFromNanos(43200001000000))
	// timestamp/timespan ± int step in nanoseconds.
	assertEvalValue(t, "2024.01.01D00:00:00+1000000000", data.TimestampFromUnixNanos(1704067201000000000))
	assertEvalValue(t, "0D00:00:01+500000000", data.TimespanFromNanos(1500000000))
}

func TestTemporalDifferences(t *testing.T) {
	// date-date and month-month -> long counts.
	assertEvalValue(t, "2024.01.02-2024.01.01", int64(1))
	assertEvalValue(t, "2024.03.01-2024.02.28", int64(2))
	assertEvalValue(t, "2024.06m-2024.04m", int64(2))
	// timestamp-timestamp -> timespan.
	assertEvalValue(t, "2024.01.02D00:00:00-2024.01.01D00:00:00", data.TimespanFromNanos(86400000000000))
	// time-of-day same-kind differences keep the kind.
	assertEvalValue(t, "13:00-12:15", data.MinuteFromMinutes(45))
	assertEvalValue(t, "12:30:30-12:30:00", data.SecondFromSeconds(30))
	assertEvalValue(t, "12:00:00.500-12:00:00.250", data.TimeFromNanos(250000000))
	// negative time-of-day results render signed.
	assertEvalValue(t, "string 12:00-12:30", "-00:30")
}

func TestTemporalMixedKindPromotions(t *testing.T) {
	// date + time-of-day / timespan -> timestamp (either order).
	assertEvalValue(t, "2024.01.01+12:00:00.000", data.TimestampFromUnixNanos(1704110400000000000))
	assertEvalValue(t, "12:00:00.000+2024.01.01", data.TimestampFromUnixNanos(1704110400000000000))
	assertEvalValue(t, "2024.01.01+12:00", data.TimestampFromUnixNanos(1704110400000000000))
	assertEvalValue(t, "2024.01.01+0D06:00:00", data.TimestampFromUnixNanos(1704088800000000000))
	// timestamp ± timespan -> timestamp.
	assertEvalValue(t, "2024.01.01D12:00:00+0D00:30:00", data.TimestampFromUnixNanos(1704112200000000000))
	assertEvalValue(t, "2024.01.01D12:00:00-0D00:30:00", data.TimestampFromUnixNanos(1704108600000000000))
	// timestamp - date -> timespan (and the reverse).
	assertEvalValue(t, "2024.01.01D12:00:00-2024.01.01", data.TimespanFromNanos(43200000000000))
	assertEvalValue(t, "2024.01.01-2024.01.01D12:00:00", data.TimespanFromNanos(-43200000000000))
	// time-of-day mixes promote to the finer kind.
	assertEvalValue(t, "12:00+00:00:30", data.SecondFromSeconds(43230))
	assertEvalValue(t, "0D00:30:00+0D00:15:00", data.TimespanFromNanos(2700000000000))
	// int - temporal stays an error (skipped: canonical semantics unverified).
	assertEvalErrorContains(t, "1-2024.01.01", "numeric operands")
	// floats do not slide into temporal arithmetic.
	assertEvalErrorContains(t, "2024.01.01+1.5", "numeric operands")
}

func TestTemporalNullAndInfinityArithmetic(t *testing.T) {
	// typed temporal nulls propagate through + and -.
	assertEvalValue(t, "0Nd+1", data.NullForKind(data.KindDate))
	assertEvalValue(t, "1+0Nu", data.NullForKind(data.KindMinute))
	assertEvalValue(t, "0Np-1", data.NullForKind(data.KindTimestamp))
	assertEvalValue(t, "2024.01.01+0N", data.NullForKind(data.KindDate))
	assertEvalValue(t, "2024.01.02-0Nd", data.NullForKind(data.KindI64))
	// temporal infinities parse as the extreme encodings.
	assertEvalValue(t, "0Wd", data.DateFromDays(math.MaxInt64))
	assertEvalValue(t, "-0Wd", data.DateFromDays(-math.MaxInt64))
	assertEvalValue(t, "0Wp", data.TimestampFromUnixNanos(math.MaxInt64))
	assertEvalValue(t, "0Wu", data.MinuteFromMinutes(math.MaxInt64))
	assertEvalValue(t, "0Wt", data.TimeFromNanos(math.MaxInt64))
	assertEvalValue(t, "0Wn", data.TimespanFromNanos(math.MaxInt64))
	assertEvalValue(t, "0Wv", data.SecondFromSeconds(math.MaxInt64))
	assertEvalValue(t, "0Wm", data.MonthFromMonths(math.MaxInt64))
	assertEvalValue(t, "0Wz", data.DateTimeFromUnixNanos(math.MaxInt64))
	// infinities sort greatest / least within their kind.
	assertEvalValue(t, "0Wd>2024.01.01", true)
	assertEvalValue(t, "-0Wd<2024.01.01", true)
	assertEvalValue(t, "0Wd=0Wd", true)
	// infinity literals round-trip through string.
	assertEvalValue(t, "string 0Wd", "0Wd")
	assertEvalValue(t, "string -0Wp", "-0Wp")
}

func TestTemporalAccessorCasts(t *testing.T) {
	assertEvalValue(t, "`year$2024.06.12", int64(2024))
	assertEvalValue(t, "`mm$2024.06.12", int64(6))
	assertEvalValue(t, "`dd$2024.06.12", int64(12))
	assertEvalValue(t, "`week$2024.06.12", data.DateFromDays(19884))
	assertEvalValue(t, "`year$2024.05m", int64(2024))
	assertEvalValue(t, "`mm$2024.05m", int64(5))
	assertEvalValue(t, "`hh$12:34:56", int64(12))
	assertEvalValue(t, "`mm$12:34:56", int64(34))
	assertEvalValue(t, "`ss$12:34:56", int64(56))
	assertEvalValue(t, "`hh$2024.06.12D10:30:00", int64(10))
	// accessor casts propagate nulls and infinities.
	assertEvalValue(t, "`year$0Nd", data.NullForKind(data.KindI64))
	assertEvalValue(t, "`year$0Wd", int64(math.MaxInt64))
	// accessor casts apply elementwise over temporal vectors.
	assertEvalArray(t, "`year$2024.06.10 2024.06.12", data.KindI64, []any{int64(2024), int64(2024)})
	// temporal kind conversions.
	assertEvalValue(t, "`month$2024.06.12", data.MonthFromMonths(653))
	assertEvalValue(t, "`date$2024.06.12D10:00:00", data.DateFromDays(19886))
	assertEvalValue(t, "`date$2024.05m", data.DateFromDays(19844))
	assertEvalValue(t, "`minute$12:34:56", data.MinuteFromMinutes(754))
	assertEvalValue(t, "`second$12:34", data.SecondFromSeconds(45240))
	assertEvalValue(t, "`time$2024.01.01D12:30:15", data.TimeFromNanos(45015000000000))
	assertEvalValue(t, "`timestamp$2024.01.01", data.TimestampFromUnixNanos(1704067200000000000))
	// accessors do not hijack symbol enum casts.
	assertEvalValue(t, "(`year$`a`b)[0]", data.Symbol("a"))
}

func TestTemporalVectorArithmeticStaysTyped(t *testing.T) {
	// date vector ± int stays a date column.
	assertEvalArray(t, "2024.01.01 2024.01.02+1", data.KindDate,
		[]any{data.DateFromDays(19724), data.DateFromDays(19725)})
	assertEvalArray(t, "1+2024.01.01 2024.01.02", data.KindDate,
		[]any{data.DateFromDays(19724), data.DateFromDays(19725)})
	assertEvalArray(t, "2024.01.05 2024.01.06-2", data.KindDate,
		[]any{data.DateFromDays(19725), data.DateFromDays(19726)})
	// date vector + int vector.
	assertEvalArray(t, "2024.01.01 2024.01.01+0 1", data.KindDate,
		[]any{data.DateFromDays(19723), data.DateFromDays(19724)})
	// date vector - date scalar -> long day counts.
	assertEvalArray(t, "(2024.01.02 2024.01.03)-2024.01.01", data.KindI64,
		[]any{int64(1), int64(2)})
	// date vector - date vector -> long day counts.
	assertEvalArray(t, "2024.01.02 2024.01.03-2024.01.01 2024.01.01", data.KindI64,
		[]any{int64(1), int64(2)})
	// time-of-day vectors keep their kind.
	assertEvalArray(t, "09:00 09:30+30", data.KindMinute,
		[]any{data.MinuteFromMinutes(570), data.MinuteFromMinutes(600)})
	// timestamp vector - timestamp scalar -> timespan column.
	assertEvalArray(t, "(2024.01.01D00:00:01 2024.01.01D00:00:02)-2024.01.01D00:00:00", data.KindTimespan,
		[]any{data.TimespanFromNanos(1000000000), data.TimespanFromNanos(2000000000)})
	// null-bearing temporal vectors propagate nulls and keep the kind.
	assertEvalArray(t, "0Nd 2024.01.01+1", data.KindDate,
		[]any{data.NullValue, data.DateFromDays(19724)})
}

func TestTemporalArithmeticBothRoutes(t *testing.T) {
	assertBothRoutes(t, "d:2024.01.31; d+1")
	assertBothRoutes(t, "2024.01.02-2024.01.01")
	assertBothRoutes(t, "v:2024.01.01 2024.01.02; v+1")
	assertBothRoutes(t, "12:30+30")
	assertBothRoutes(t, "`year$2024.06.12")
	assertBothRoutes(t, "`week$2024.06.12")
	assertBothRoutes(t, "0Nd+1")
	assertBothRoutes(t, "0Wd")
	assertBothRoutes(t, "2024.01.01D12:00:00+0D00:30:00")
}
