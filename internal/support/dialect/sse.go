package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry int64
}

func ParseSSE(src string) ([]SSEEvent, error) {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var events []SSEEvent
	var cur SSEEvent
	var data []string
	seen := false
	flush := func() {
		if !seen && len(data) == 0 {
			return
		}
		cur.Data = strings.Join(data, "\n")
		events = append(events, cur)
		cur = SSEEvent{}
		data = nil
		seen = false
	}
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			field, value = line, ""
		} else if strings.HasPrefix(value, " ") {
			value = value[1:]
		}
		switch field {
		case "event":
			cur.Event = value
			seen = true
		case "data":
			data = append(data, value)
			seen = true
		case "id":
			cur.ID = value
			seen = true
		case "retry":
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("sse: invalid retry %q", value)
			}
			cur.Retry = n
			seen = true
		default:
			continue
		}
	}
	flush()
	return events, nil
}

func EncodeSSE(events []SSEEvent) string {
	var b strings.Builder
	for _, event := range events {
		if event.Event != "" {
			b.WriteString("event: ")
			b.WriteString(event.Event)
			b.WriteByte('\n')
		}
		if event.ID != "" {
			b.WriteString("id: ")
			b.WriteString(event.ID)
			b.WriteByte('\n')
		}
		if event.Retry > 0 {
			b.WriteString("retry: ")
			b.WriteString(strconv.FormatInt(event.Retry, 10))
			b.WriteByte('\n')
		}
		for _, line := range strings.Split(event.Data, "\n") {
			b.WriteString("data: ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}
