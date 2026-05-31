package time

import "strings"

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
