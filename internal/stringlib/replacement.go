package stringlib

import (
	"fmt"
	"strconv"
	"strings"
)

func ValidateLuaReplacementString(repl string, captureCount int) error {
	for i := 0; i < len(repl); i++ {
		if repl[i] != '%' {
			continue
		}
		if i+1 >= len(repl) {
			return fmt.Errorf("invalid use of '%%' in replacement string")
		}
		i++
		ch := repl[i]
		if ch == '%' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			idx := int(ch - '0')
			if idx == 0 {
				continue
			}
			if captureCount == 0 {
				if idx == 1 {
					continue
				}
			} else if idx <= captureCount {
				continue
			}
			return fmt.Errorf("invalid capture index %%%c", ch)
		}
		return fmt.Errorf("invalid use of '%%' in replacement string")
	}
	return nil
}

func ExpandSimpleLuaReplacement(s string, m SimplePatternMatch, repl string) string {
	var b strings.Builder
	for i := 0; i < len(repl); i++ {
		if repl[i] != '%' || i+1 >= len(repl) {
			b.WriteByte(repl[i])
			continue
		}
		i++
		ch := repl[i]
		if ch == '%' {
			b.WriteByte('%')
			continue
		}
		if ch >= '0' && ch <= '9' {
			idx := int(ch - '0')
			if idx == 0 {
				b.WriteString(s[m.Start:m.End])
			} else if idx == 1 && m.NCapture == 0 {
				b.WriteString(s[m.Start:m.End])
			} else if idx > 0 && idx <= m.NCapture {
				b.WriteString(s[m.Captures[idx-1][0]:m.Captures[idx-1][1]])
			}
			continue
		}
		b.WriteByte('%')
		b.WriteByte(ch)
	}
	return b.String()
}

func ReplaceSimpleLuaPatternString(s string, pattern *SimplePattern, repl string, maxRepl int, count *int) string {
	if pattern == nil {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	nextStart := 0
	for maxRepl < 0 || *count < maxRepl {
		m, ok := pattern.FindNext(s, nextStart)
		if !ok {
			break
		}
		if m.Start < last {
			nextStart = NextSearchStart(s, m.Start, m.End)
			continue
		}
		b.WriteString(s[last:m.Start])
		b.WriteString(ExpandSimpleLuaReplacement(s, m, repl))
		last = m.End
		(*count)++
		nextStart = NextSearchStart(s, m.Start, m.End)
	}
	if *count == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

func ReplaceSimpleLuaPatternRaw(s string, pattern *SimplePattern, repl string, maxRepl int, count *int) string {
	if pattern == nil {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	nextStart := 0
	for maxRepl < 0 || *count < maxRepl {
		m, ok := pattern.FindNext(s, nextStart)
		if !ok {
			break
		}
		if m.Start < last {
			nextStart = NextSearchStart(s, m.Start, m.End)
			continue
		}
		b.WriteString(s[last:m.Start])
		b.WriteString(repl)
		last = m.End
		(*count)++
		nextStart = NextSearchStart(s, m.Start, m.End)
	}
	if *count == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

type LuaPatternCaptureKind uint8

const (
	LuaPatternCaptureText LuaPatternCaptureKind = iota
	LuaPatternCapturePosition
)

type LuaPatternProgram struct {
	CaptureKinds []LuaPatternCaptureKind
	CaptureSlots []int
}

func ExpandLuaReplacement(s string, loc []int, prog LuaPatternProgram, repl string) string {
	var b strings.Builder
	for i := 0; i < len(repl); i++ {
		if repl[i] != '%' || i+1 >= len(repl) {
			b.WriteByte(repl[i])
			continue
		}
		i++
		ch := repl[i]
		if ch == '%' {
			b.WriteByte('%')
			continue
		}
		if ch >= '0' && ch <= '9' {
			idx := int(ch - '0')
			if idx == 0 {
				b.WriteString(s[loc[0]:loc[1]])
			} else if idx == 1 && len(prog.CaptureSlots) == 0 {
				b.WriteString(s[loc[0]:loc[1]])
			} else if idx > 0 && idx <= len(prog.CaptureSlots) {
				slot := prog.CaptureSlots[idx-1]
				pos := slot * 2
				if prog.CaptureKinds[idx-1] == LuaPatternCapturePosition && pos+1 < len(loc) && loc[pos] >= 0 {
					b.WriteString(strconv.Itoa(loc[pos] + 1))
				} else if pos+1 < len(loc) && loc[pos] >= 0 {
					b.WriteString(s[loc[pos]:loc[pos+1]])
				}
			}
			continue
		}
		b.WriteByte('%')
		b.WriteByte(ch)
	}
	return b.String()
}
