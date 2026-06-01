package time

import (
	"strings"
	gotime "time"
)

var strftimeReplacements = []struct {
	from string
	to   string
}{
	{"%Y", "2006"},
	{"%m", "01"},
	{"%d", "02"},
	{"%H", "15"},
	{"%M", "04"},
	{"%S", "05"},
	{"%A", "Monday"},
	{"%B", "January"},
	{"%Z", "MST"},
	{"%%", "%"},
}

func LayoutFromStrftime(layout string) string {
	if !strings.Contains(layout, "%") {
		return layout
	}

	result := layout
	for _, r := range strftimeReplacements {
		result = strings.ReplaceAll(result, r.from, r.to)
	}
	return result
}

func DurationFromSeconds(seconds float64) gotime.Duration {
	return gotime.Duration(seconds * float64(gotime.Second))
}

func UnixUTC(sec, nsec int64) gotime.Time {
	return gotime.Unix(sec, nsec).UTC()
}

func DateUTC(year int, month gotime.Month, day, hour, min, sec, nsec int) gotime.Time {
	return gotime.Date(year, month, day, hour, min, sec, nsec, gotime.UTC)
}

func FormatWithStrftimeLayout(t gotime.Time, layout string) string {
	return t.Format(LayoutFromStrftime(layout))
}

func ParseWithStrftimeLayout(value, layout string) (gotime.Time, error) {
	return gotime.Parse(LayoutFromStrftime(layout), value)
}

// LuaDateFormat formats t using the Lua os.date directive subset supported by
// the stdlib runtime binding layer.
func LuaDateFormat(format string, t gotime.Time) string {
	result := format
	replacements := map[string]string{
		"%Y": "2006",
		"%y": "06",
		"%m": "01",
		"%d": "02",
		"%H": "15",
		"%M": "04",
		"%S": "05",
		"%A": "Monday",
		"%a": "Mon",
		"%B": "January",
		"%b": "Jan",
		"%p": "PM",
		"%c": "Mon Jan  2 15:04:05 2006",
		"%X": "15:04:05",
		"%x": "01/02/06",
		"%%": "%",
	}
	for lua, goFmt := range replacements {
		if goFmt == "%" {
			continue
		}
		for {
			idx := findLuaFormatSpec(result, lua)
			if idx < 0 {
				break
			}
			result = result[:idx] + t.Format(goFmt) + result[idx+len(lua):]
		}
	}
	for {
		idx := findLuaFormatSpec(result, "%%")
		if idx < 0 {
			break
		}
		result = result[:idx] + "%" + result[idx+2:]
	}
	return result
}

func findLuaFormatSpec(s, spec string) int {
	for i := 0; i <= len(s)-len(spec); i++ {
		if s[i:i+len(spec)] == spec {
			return i
		}
	}
	return -1
}
