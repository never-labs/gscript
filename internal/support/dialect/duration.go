package dialect

import (
	"fmt"
	"math"
	"time"
)

type DurationParts struct {
	Text         string
	Nanoseconds  int64
	Seconds      float64
	Milliseconds float64
}

func ParseDuration(src string) (DurationParts, error) {
	d, err := time.ParseDuration(src)
	if err != nil {
		return DurationParts{}, &ParseError{Kind: "duration", Message: err.Error()}
	}
	return durationParts(d), nil
}

func EncodeDurationNanoseconds(ns int64) string {
	return time.Duration(ns).String()
}

func EncodeDurationSeconds(seconds float64) (string, error) {
	ns, err := DurationSecondsToNanoseconds(seconds)
	if err != nil {
		return "", err
	}
	return EncodeDurationNanoseconds(ns), nil
}

func EncodeDurationMilliseconds(milliseconds float64) (string, error) {
	ns, err := durationFloatToNanoseconds(milliseconds, float64(time.Millisecond), "milliseconds")
	if err != nil {
		return "", err
	}
	return EncodeDurationNanoseconds(ns), nil
}

func DurationSecondsToNanoseconds(seconds float64) (int64, error) {
	return durationFloatToNanoseconds(seconds, float64(time.Second), "seconds")
}

func durationParts(d time.Duration) DurationParts {
	ns := int64(d)
	return DurationParts{
		Text:         d.String(),
		Nanoseconds:  ns,
		Seconds:      float64(ns) / float64(time.Second),
		Milliseconds: float64(ns) / float64(time.Millisecond),
	}
}

func durationFloatToNanoseconds(value float64, scale float64, field string) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, &ParseError{Kind: "duration", Message: fmt.Sprintf("%s must be finite", field)}
	}
	ns := math.Round(value * scale)
	if ns > float64(math.MaxInt64) || ns < float64(math.MinInt64) {
		return 0, &ParseError{Kind: "duration", Message: fmt.Sprintf("%s duration overflows int64 nanoseconds", field)}
	}
	return int64(ns), nil
}
