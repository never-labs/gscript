package runtime

import (
	"fmt"
	stdlibstring "github.com/never-labs/gscript/internal/stdlib/string"
	"regexp"
	"strconv"
	"strings"
	"unsafe"
)

// native_string_pattern.go holds runtime-owned native Lua-pattern helpers for
// string.find/match/gmatch/gsub: pattern compilation/caching, the simple
// pattern matcher, the regex-backed program, balanced-pattern matching, and
// the gsub replacement expanders used by VM/JIT paths.

func StdStringFindIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringFindIdentity)
}

func StdStringMatchIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringMatchIdentity)
}

func StdStringGSubIdentityPtr() unsafe.Pointer {
	return unsafe.Pointer(&stdStringGSubIdentity)
}

// FastStringFindRet2 computes the first two string.find return values without
// allocating a result slice. It only handles cases where dropping captures is
// semantically safe for the caller's fixed result count.
func FastStringFindRet2(sv, pv, initv, plainv Value, nArgs, rawC int) (Value, Value, int, bool, error) {
	if nArgs != 2 && nArgs != 3 && nArgs != 4 {
		return NilValue(), NilValue(), 0, false, nil
	}
	if !sv.IsString() || !pv.IsString() {
		return NilValue(), NilValue(), 0, true, fmt.Errorf("bad argument to 'string.find' (string expected)")
	}
	s := sv.Str()
	pattern := pv.Str()
	init := 1
	if nArgs >= 3 {
		init = int(toInt(initv))
	}
	if init < 0 {
		init = len(s) + init + 1
	}
	if init < 1 {
		init = 1
	}
	if init > len(s)+1 {
		return NilValue(), NilValue(), 1, true, nil
	}
	searchStr := s[init-1:]
	if nArgs >= 4 && plainv.Truthy() {
		idx := strings.Index(searchStr, pattern)
		if idx < 0 {
			return NilValue(), NilValue(), 1, true, nil
		}
		start := idx + init
		end := start + len(pattern) - 1
		return IntValue(int64(start)), IntValue(int64(end)), 2, true, nil
	}

	if balanced, open, close := stdlibstring.ParseStandaloneBalancedPattern(pattern); balanced {
		loc := stdlibstring.FindBalancedRange(searchStr, open, close, 0)
		if loc == nil {
			return NilValue(), NilValue(), 1, true, nil
		}
		return IntValue(int64(loc[0] + init)), IntValue(int64(loc[1] + init - 1)), 2, true, nil
	}
	if simple, ok := cachedSimpleLuaPattern(pattern); ok {
		if simple.CaptureCount() > 0 && (rawC == 0 || rawC-1 > 2) {
			return NilValue(), NilValue(), 0, false, nil
		}
		m, ok := simple.FindNext(searchStr, 0)
		if !ok {
			return NilValue(), NilValue(), 1, true, nil
		}
		return IntValue(int64(m.Start + init)), IntValue(int64(m.End + init - 1)), 2, true, nil
	}
	prog, re, err := cachedLuaPatternRegexp(pattern)
	if err != nil {
		return NilValue(), NilValue(), 0, true, fmt.Errorf("invalid pattern: %s", err)
	}
	if len(prog.captureSlots) > 0 && (rawC == 0 || rawC-1 > 2) {
		return NilValue(), NilValue(), 0, false, nil
	}
	loc := prog.findSubmatchIndex(re, searchStr)
	if loc == nil {
		return NilValue(), NilValue(), 1, true, nil
	}
	return IntValue(int64(loc[0] + init)), IntValue(int64(loc[1] + init - 1)), 2, true, nil
}

// FastStringMatchRet2 computes string.match results without allocating a result
// slice when the caller's fixed result count cannot observe later captures.
func FastStringMatchRet2(sv, pv, initv Value, nArgs, rawC int) (Value, Value, int, bool, error) {
	if nArgs != 2 && nArgs != 3 {
		return NilValue(), NilValue(), 0, false, nil
	}
	if !sv.IsString() || !pv.IsString() {
		return NilValue(), NilValue(), 0, true, fmt.Errorf("bad argument to 'string.match' (string expected)")
	}
	s := sv.Str()
	pattern := pv.Str()
	init := 1
	if nArgs >= 3 {
		init = int(toInt(initv))
	}
	if init < 0 {
		init = len(s) + init + 1
	}
	if init < 1 {
		init = 1
	}
	if init > len(s)+1 {
		return NilValue(), NilValue(), 1, true, nil
	}
	searchStr := s[init-1:]

	if balanced, open, close := stdlibstring.ParseStandaloneBalancedPattern(pattern); balanced {
		loc := stdlibstring.FindBalancedRange(searchStr, open, close, 0)
		if loc == nil {
			return NilValue(), NilValue(), 1, true, nil
		}
		return StringValue(searchStr[loc[0]:loc[1]]), NilValue(), 1, true, nil
	}
	if simple, ok := cachedSimpleLuaPattern(pattern); ok {
		if simple.CaptureCount() > 2 && (rawC == 0 || rawC-1 > 2) {
			return NilValue(), NilValue(), 0, false, nil
		}
		m, ok := simple.FindNext(searchStr, 0)
		if !ok {
			return NilValue(), NilValue(), 1, true, nil
		}
		if m.NCapture == 0 {
			return StringValue(searchStr[m.Start:m.End]), NilValue(), 1, true, nil
		}
		r0 := StringValue(searchStr[m.Captures[0][0]:m.Captures[0][1]])
		if m.NCapture == 1 {
			return r0, NilValue(), 1, true, nil
		}
		return r0, StringValue(searchStr[m.Captures[1][0]:m.Captures[1][1]]), 2, true, nil
	}
	prog, re, err := cachedLuaPatternRegexp(pattern)
	if err != nil {
		return NilValue(), NilValue(), 0, true, fmt.Errorf("invalid pattern: %s", err)
	}
	if len(prog.captureSlots) > 2 && (rawC == 0 || rawC-1 > 2) {
		return NilValue(), NilValue(), 0, false, nil
	}
	loc := prog.findSubmatchIndex(re, searchStr)
	if loc == nil {
		return NilValue(), NilValue(), 1, true, nil
	}
	if len(prog.captureSlots) == 0 {
		return StringValue(searchStr[loc[0]:loc[1]]), NilValue(), 1, true, nil
	}
	var r0, r1 Value
	for i, slot := range prog.captureSlots {
		if i >= 2 {
			break
		}
		pos := slot * 2
		v := NilValue()
		if pos+1 < len(loc) && loc[pos] >= 0 {
			if prog.captureKinds[i] == luaPatternCapturePosition {
				v = IntValue(int64(loc[pos] + init))
			} else {
				v = StringValue(searchStr[loc[pos]:loc[pos+1]])
			}
		}
		if i == 0 {
			r0 = v
		} else {
			r1 = v
		}
	}
	if len(prog.captureSlots) == 1 {
		return r0, NilValue(), 1, true, nil
	}
	return r0, r1, 2, true, nil
}

func IsStdStringSubFunction(v Value) bool {
	gf := v.GoFunction()
	return gf != nil &&
		gf.NativeKind == NativeKindStdStringSub &&
		gf.NativeData == StdStringSubIdentityPtr() &&
		gf.FastArg2 != nil &&
		gf.FastArg3 != nil
}

type luaPatternCaptureKind uint8

const (
	luaPatternCaptureText luaPatternCaptureKind = iota
	luaPatternCapturePosition
)

type simpleLuaPatternCacheEntry struct {
	pattern *stdlibstring.SimplePattern
	ok      bool
}

func cachedSimpleLuaPattern(pattern string) (*stdlibstring.SimplePattern, bool) {
	if cached, ok := simpleLuaPatternCache.Load(pattern); ok {
		entry := cached.(simpleLuaPatternCacheEntry)
		return entry.pattern, entry.ok
	}
	compiled, ok := stdlibstring.CompileSimplePattern(pattern)
	entry := simpleLuaPatternCacheEntry{pattern: compiled, ok: ok}
	actual, _ := simpleLuaPatternCache.LoadOrStore(pattern, entry)
	entry = actual.(simpleLuaPatternCacheEntry)
	return entry.pattern, entry.ok
}

func simpleMatchValues(s string, m stdlibstring.SimplePatternMatch) []Value {
	if m.NCapture == 0 {
		return []Value{StringValue(s[m.Start:m.End])}
	}
	out := make([]Value, 0, m.NCapture)
	for i := 0; i < m.NCapture; i++ {
		out = append(out, StringValue(s[m.Captures[i][0]:m.Captures[i][1]]))
	}
	return out
}

type luaPatternFrontier struct {
	slot  int
	class string
}

type luaPatternProgram struct {
	regex        string
	captureKinds []luaPatternCaptureKind
	captureSlots []int
	frontiers    []luaPatternFrontier
}

// luaPatternToRegex converts a Lua-style pattern string to a Go regex string.
func luaPatternToRegex(pattern string) string {
	return compileLuaPattern(pattern).regex
}

func luaPatternToRegexWithCaptures(pattern string) (string, []luaPatternCaptureKind) {
	prog := compileLuaPattern(pattern)
	return prog.regex, prog.captureKinds
}

func cachedLuaPatternRegexp(pattern string) (luaPatternProgram, *regexp.Regexp, error) {
	if cached, ok := compiledLuaPatternCache.Load(pattern); ok {
		entry := cached.(compiledLuaPatternCacheEntry)
		return entry.prog, entry.re, nil
	}
	prog := compileLuaPattern(pattern)
	re, err := regexp.Compile(prog.regex)
	if err != nil {
		return luaPatternProgram{}, nil, err
	}
	entry := compiledLuaPatternCacheEntry{prog: prog, re: re}
	actual, _ := compiledLuaPatternCache.LoadOrStore(pattern, entry)
	entry = actual.(compiledLuaPatternCacheEntry)
	return entry.prog, entry.re, nil
}

func compileLuaPattern(pattern string) luaPatternProgram {
	var buf strings.Builder
	var captureKinds []luaPatternCaptureKind
	var captureSlots []int
	var frontiers []luaPatternFrontier
	captureSlot := 1
	i := 0
	n := len(pattern)

	// Handle anchors
	if n > 0 && pattern[0] == '^' {
		buf.WriteByte('^')
		i++
	}

	// Track whether the previous item was a matchable item (can have a quantifier)
	prevMatchable := false

	for i < n {
		c := pattern[i]
		switch c {
		case '%':
			i++
			if i >= n {
				buf.WriteByte('%')
				prevMatchable = false
				continue
			}
			next := pattern[i]
			switch next {
			case 'd':
				buf.WriteString("[0-9]")
			case 'D':
				buf.WriteString("[^0-9]")
			case 'x':
				buf.WriteString("[0-9A-Fa-f]")
			case 'X':
				buf.WriteString("[^0-9A-Fa-f]")
			case 'a':
				buf.WriteString("[a-zA-Z]")
			case 'A':
				buf.WriteString("[^a-zA-Z]")
			case 'l':
				buf.WriteString("[a-z]")
			case 'L':
				buf.WriteString("[^a-z]")
			case 'u':
				buf.WriteString("[A-Z]")
			case 'U':
				buf.WriteString("[^A-Z]")
			case 's':
				buf.WriteString("[\\t\\n\\r\\f\\v ]")
			case 'S':
				buf.WriteString("[^\\t\\n\\r\\f\\v ]")
			case 'w':
				buf.WriteString("[a-zA-Z0-9]")
			case 'W':
				buf.WriteString("[^a-zA-Z0-9]")
			case 'p':
				buf.WriteString("[!-/:-@\\[-`{-~]")
			case 'P':
				buf.WriteString("[^!-/:-@\\[-`{-~]")
			case 'c':
				buf.WriteString("[\\x00-\\x1f\\x7f]")
			case 'C':
				buf.WriteString("[^\\x00-\\x1f\\x7f]")
			case 'z':
				buf.WriteString("\\x00")
			case 'Z':
				buf.WriteString("[^\\x00]")
			case 'f':
				if i+1 < n && pattern[i+1] == '[' {
					start := i + 1
					j := start + 1
					if j < n && pattern[j] == '^' {
						j++
					}
					if j < n && pattern[j] == ']' && j+1 < n {
						j++
					}
					for j < n && pattern[j] != ']' {
						j++
					}
					if j < n {
						buf.WriteString("()")
						frontiers = append(frontiers, luaPatternFrontier{slot: captureSlot, class: pattern[start : j+1]})
						captureSlot++
						i = j + 1
						prevMatchable = false
						continue
					}
				}
				buf.WriteString("[")
			case 'b':
				buf.WriteString("[")
			default:
				// Escape the literal character
				buf.WriteString(regexp.QuoteMeta(string(next)))
			}
			i++
			prevMatchable = true
		case '[':
			start := i
			i++
			if i < n && pattern[i] == '^' {
				i++
			}
			if i < n && pattern[i] == ']' && i+1 < n {
				i++
			}
			for i < n && pattern[i] != ']' {
				i++
			}
			if i < n {
				i++
			}
			buf.WriteString(luaBracketClassToRegex(pattern[start:i]))
			prevMatchable = true
		case '(':
			if i+1 < n && pattern[i+1] == ')' {
				buf.WriteString("()")
				captureKinds = append(captureKinds, luaPatternCapturePosition)
				captureSlots = append(captureSlots, captureSlot)
				captureSlot++
				i += 2
			} else {
				buf.WriteByte('(')
				captureKinds = append(captureKinds, luaPatternCaptureText)
				captureSlots = append(captureSlots, captureSlot)
				captureSlot++
				i++
			}
			prevMatchable = false
		case ')':
			buf.WriteByte(')')
			i++
			prevMatchable = false // In Lua, groups cannot be quantified
		case '.':
			// Lua patterns match any byte for '.', including newlines.
			buf.WriteString("(?s:.)")
			i++
			prevMatchable = true
		case '*', '+', '?':
			buf.WriteByte(c)
			i++
			prevMatchable = false
		case '-':
			// In Lua, '-' is a non-greedy repetition modifier (like *? in regex)
			// but only when it follows a matchable item. Otherwise it's a literal '-'.
			if prevMatchable {
				buf.WriteString("*?")
				prevMatchable = false
			} else {
				buf.WriteString("\\-")
				prevMatchable = true
			}
			i++
		case '$':
			if i == n-1 {
				buf.WriteByte('$')
				prevMatchable = false
			} else {
				buf.WriteString(regexp.QuoteMeta("$"))
				prevMatchable = true
			}
			i++
		default:
			// Check if the char needs escaping for Go regex
			if isRegexMeta(c) {
				buf.WriteByte('\\')
			}
			buf.WriteByte(c)
			i++
			prevMatchable = true
		}
	}

	return luaPatternProgram{
		regex:        buf.String(),
		captureKinds: captureKinds,
		captureSlots: captureSlots,
		frontiers:    frontiers,
	}
}

func luaBracketClassToRegex(class string) string {
	if len(class) < 2 || class[0] != '[' || class[len(class)-1] != ']' {
		return "["
	}
	negated := len(class) > 2 && class[1] == '^'
	bodyStart := 1
	if negated {
		bodyStart = 2
	}
	body := class[bodyStart : len(class)-1]
	if body == "" || body[len(body)-1] == '%' {
		return "["
	}
	if kind, ok := luaStandaloneClassEscape(body); ok {
		return luaPatternClassRegex(kind, negated)
	}

	var buf strings.Builder
	buf.WriteByte('[')
	if negated {
		buf.WriteByte('^')
	}
	for i := 0; i < len(body); i++ {
		if body[i] == '%' && i+1 < len(body) {
			i++
			ch := body[i]
			if content, ok := luaPositiveClassContent(ch); ok {
				buf.WriteString(content)
			} else {
				buf.WriteString(regexp.QuoteMeta(string(ch)))
			}
			continue
		}
		buf.WriteByte(body[i])
	}
	buf.WriteByte(']')
	return buf.String()
}

func luaStandaloneClassEscape(body string) (byte, bool) {
	if len(body) == 2 && body[0] == '%' {
		return body[1], true
	}
	return 0, false
}

func luaPatternClassRegex(kind byte, outerNegated bool) string {
	if positive, ok := luaPositiveClassContent(kind); ok {
		if outerNegated {
			return "[^" + positive + "]"
		}
		return "[" + positive + "]"
	}
	if positive, ok := luaNegatedClassContent(kind); ok {
		if outerNegated {
			return "[" + positive + "]"
		}
		return "[^" + positive + "]"
	}
	quoted := regexp.QuoteMeta(string(kind))
	if outerNegated {
		return "[^" + quoted + "]"
	}
	return "[" + quoted + "]"
}

func (p luaPatternProgram) findSubmatchIndex(re *regexp.Regexp, s string) []int {
	if len(p.frontiers) == 0 {
		return re.FindStringSubmatchIndex(s)
	}
	for _, loc := range re.FindAllStringSubmatchIndex(s, -1) {
		if p.frontiersMatch(s, loc) {
			return loc
		}
	}
	return nil
}

func (p luaPatternProgram) findAllSubmatchIndex(re *regexp.Regexp, s string) [][]int {
	matches := re.FindAllStringSubmatchIndex(s, -1)
	if len(p.frontiers) == 0 || len(matches) == 0 {
		return matches
	}
	out := matches[:0]
	for _, loc := range matches {
		if p.frontiersMatch(s, loc) {
			out = append(out, loc)
		}
	}
	return out
}

func (p luaPatternProgram) frontiersMatch(s string, loc []int) bool {
	for _, frontier := range p.frontiers {
		posSlot := frontier.slot * 2
		if posSlot >= len(loc) || loc[posSlot] < 0 {
			return false
		}
		if !luaFrontierMatches(s, loc[posSlot], frontier.class) {
			return false
		}
	}
	return true
}

func luaFrontierMatches(s string, pos int, class string) bool {
	prev := byte(0)
	if pos > 0 {
		prev = s[pos-1]
	}
	next := byte(0)
	if pos < len(s) {
		next = s[pos]
	}
	return !luaBracketClassByteContains(class, prev) && luaBracketClassByteContains(class, next)
}

func luaBracketClassByteContains(class string, b byte) bool {
	if len(class) < 2 || class[0] != '[' || class[len(class)-1] != ']' {
		return false
	}
	negated := len(class) > 2 && class[1] == '^'
	bodyStart := 1
	if negated {
		bodyStart = 2
	}
	body := class[bodyStart : len(class)-1]
	matched := luaClassBodyByteContains(body, b)
	if negated {
		return !matched
	}
	return matched
}

func luaClassBodyByteContains(body string, b byte) bool {
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if ch == '%' && i+1 < len(body) {
			i++
			if luaClassEscapeByteContains(body[i], b) {
				return true
			}
			continue
		}
		if i+2 < len(body) && body[i+1] == '-' && body[i+2] != ']' {
			end := body[i+2]
			if ch <= b && b <= end {
				return true
			}
			i += 2
			continue
		}
		if ch == b {
			return true
		}
	}
	return false
}

func luaClassEscapeByteContains(kind byte, b byte) bool {
	switch kind {
	case 'd':
		return b >= '0' && b <= '9'
	case 'D':
		return !(b >= '0' && b <= '9')
	case 'x':
		return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')
	case 'X':
		return !luaClassEscapeByteContains('x', b)
	case 'a':
		return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
	case 'A':
		return !luaClassEscapeByteContains('a', b)
	case 'l':
		return b >= 'a' && b <= 'z'
	case 'L':
		return !luaClassEscapeByteContains('l', b)
	case 'u':
		return b >= 'A' && b <= 'Z'
	case 'U':
		return !luaClassEscapeByteContains('u', b)
	case 's':
		return b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v' || b == ' '
	case 'S':
		return !luaClassEscapeByteContains('s', b)
	case 'w':
		return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
	case 'W':
		return !luaClassEscapeByteContains('w', b)
	case 'p':
		return (b >= '!' && b <= '/') || (b >= ':' && b <= '@') || (b >= '[' && b <= '`') || (b >= '{' && b <= '~')
	case 'P':
		return !luaClassEscapeByteContains('p', b)
	case 'c':
		return b <= 0x1f || b == 0x7f
	case 'C':
		return !luaClassEscapeByteContains('c', b)
	case 'z':
		return b == 0
	case 'Z':
		return b != 0
	default:
		return b == kind
	}
}

func luaPositiveClassContent(kind byte) (string, bool) {
	switch kind {
	case 'd':
		return "0-9", true
	case 'x':
		return "0-9A-Fa-f", true
	case 'a':
		return "a-zA-Z", true
	case 'l':
		return "a-z", true
	case 'u':
		return "A-Z", true
	case 's':
		return "\\t\\n\\r\\f\\v ", true
	case 'w':
		return "a-zA-Z0-9", true
	case 'p':
		return "!-/:-@\\[-`{-~", true
	case 'c':
		return "\\x00-\\x1f\\x7f", true
	case 'z':
		return "\\x00", true
	default:
		return "", false
	}
}

func luaNegatedClassContent(kind byte) (string, bool) {
	switch kind {
	case 'D':
		return "0-9", true
	case 'X':
		return "0-9A-Fa-f", true
	case 'A':
		return "a-zA-Z", true
	case 'L':
		return "a-z", true
	case 'U':
		return "A-Z", true
	case 'S':
		return "\\t\\n\\r\\f\\v ", true
	case 'W':
		return "a-zA-Z0-9", true
	case 'P':
		return "!-/:-@\\[-`{-~", true
	case 'C':
		return "\\x00-\\x1f\\x7f", true
	case 'Z':
		return "\\x00", true
	default:
		return "", false
	}
}

func replaceBalancedPatternString(s string, open, close byte, repl string, maxRepl int, count *int) string {
	result, _ := replaceBalancedPattern(s, open, close, maxRepl, count, func(loc []int) (string, error) {
		return expandLuaReplacement(s, loc, luaPatternProgram{}, repl), nil
	})
	return result
}

func replaceBalancedPatternRaw(s string, open, close byte, repl string, maxRepl int, count *int) string {
	result, _ := replaceBalancedPattern(s, open, close, maxRepl, count, func(_ []int) (string, error) {
		return repl, nil
	})
	return result
}

func replaceBalancedPatternTable(s string, open, close byte, repl *Table, maxRepl int, count *int) (string, error) {
	return replaceBalancedPattern(s, open, close, maxRepl, count, func(loc []int) (string, error) {
		key := StringValue(s[loc[0]:loc[1]])
		val := repl.RawGet(key)
		if val.IsNil() || (val.IsBool() && !val.Bool()) {
			return s[loc[0]:loc[1]], nil
		}
		if val.IsString() || val.IsNumber() {
			return val.String(), nil
		}
		return "", fmt.Errorf("invalid replacement value (a %s)", val.TypeName())
	})
}

func replaceBalancedPatternFunction(s string, open, close byte, fn Value, caller ScriptFunctionCaller, maxRepl int, count *int) (string, error) {
	return replaceBalancedPattern(s, open, close, maxRepl, count, func(loc []int) (string, error) {
		return callLuaReplacementFunction(s, loc, luaPatternProgram{}, fn, caller)
	})
}

func replaceBalancedPattern(s string, open, close byte, maxRepl int, count *int, repl func([]int) (string, error)) (string, error) {
	var b strings.Builder
	last := 0
	next := 0
	for next < len(s) {
		if maxRepl >= 0 && *count >= maxRepl {
			break
		}
		loc := stdlibstring.FindBalancedRange(s, open, close, next)
		if loc == nil {
			break
		}
		replacement, err := repl(loc)
		if err != nil {
			return "", err
		}
		b.WriteString(s[last:loc[0]])
		b.WriteString(replacement)
		last = loc[1]
		next = loc[1]
		(*count)++
	}
	if *count == 0 {
		return s, nil
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

func replaceLuaPatternFunction(s string, re *regexp.Regexp, prog luaPatternProgram, fn Value, caller ScriptFunctionCaller, maxRepl int, count *int) (string, error) {
	matches := prog.findAllSubmatchIndex(re, s)
	if len(matches) == 0 {
		return s, nil
	}
	var b strings.Builder
	last := 0
	for _, loc := range matches {
		if maxRepl >= 0 && *count >= maxRepl {
			break
		}
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		replacement, err := callLuaReplacementFunction(s, loc, prog, fn, caller)
		if err != nil {
			return "", err
		}
		b.WriteString(s[last:start])
		b.WriteString(replacement)
		last = end
		(*count)++
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

func replaceLuaPatternString(s string, re *regexp.Regexp, prog luaPatternProgram, repl string, maxRepl int, count *int) string {
	matches := prog.findAllSubmatchIndex(re, s)
	if len(matches) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, loc := range matches {
		if maxRepl >= 0 && *count >= maxRepl {
			break
		}
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		b.WriteString(s[last:start])
		b.WriteString(expandLuaReplacement(s, loc, prog, repl))
		last = end
		(*count)++
	}
	b.WriteString(s[last:])
	return b.String()
}

func replaceLuaPatternTable(s string, re *regexp.Regexp, prog luaPatternProgram, repl *Table, maxRepl int, count *int) (string, error) {
	matches := prog.findAllSubmatchIndex(re, s)
	if len(matches) == 0 {
		return s, nil
	}
	var b strings.Builder
	last := 0
	for _, loc := range matches {
		if maxRepl >= 0 && *count >= maxRepl {
			break
		}
		start, end := loc[0], loc[1]
		if start < last {
			continue
		}
		key := luaReplacementTableKey(s, loc, prog)
		val := repl.RawGet(key)
		replacement := s[start:end]
		if !val.IsNil() && !(val.IsBool() && !val.Bool()) {
			if val.IsString() || val.IsNumber() {
				replacement = val.String()
			} else {
				return "", fmt.Errorf("invalid replacement value (a %s)", val.TypeName())
			}
		}
		b.WriteString(s[last:start])
		b.WriteString(replacement)
		last = end
		(*count)++
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

func replaceSimpleLuaPatternFunction(s string, pattern *stdlibstring.SimplePattern, fn Value, caller ScriptFunctionCaller, maxRepl int, count *int) (string, error) {
	if pattern == nil {
		return s, nil
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
			nextStart = stdlibstring.NextSearchStart(s, m.Start, m.End)
			continue
		}
		replacement, err := callSimpleReplacementFunction(s, m, fn, caller)
		if err != nil {
			return "", err
		}
		b.WriteString(s[last:m.Start])
		b.WriteString(replacement)
		last = m.End
		(*count)++
		nextStart = stdlibstring.NextSearchStart(s, m.Start, m.End)
	}
	if *count == 0 {
		return s, nil
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

func replaceSimpleLuaPatternString(s string, pattern *stdlibstring.SimplePattern, repl string, maxRepl int, count *int) string {
	return stdlibstring.ReplaceSimpleLuaPatternString(s, pattern, repl, maxRepl, count)
}

func replaceSimpleLuaPatternTable(s string, pattern *stdlibstring.SimplePattern, repl *Table, maxRepl int, count *int) (string, error) {
	if pattern == nil {
		return s, nil
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
			nextStart = stdlibstring.NextSearchStart(s, m.Start, m.End)
			continue
		}
		key := simpleReplacementTableKey(s, m)
		val := repl.RawGet(key)
		replacement := s[m.Start:m.End]
		if !val.IsNil() && !(val.IsBool() && !val.Bool()) {
			if val.IsString() || val.IsNumber() {
				replacement = val.String()
			} else {
				return "", fmt.Errorf("invalid replacement value (a %s)", val.TypeName())
			}
		}
		b.WriteString(s[last:m.Start])
		b.WriteString(replacement)
		last = m.End
		(*count)++
		nextStart = stdlibstring.NextSearchStart(s, m.Start, m.End)
	}
	if *count == 0 {
		return s, nil
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

func replaceSimpleLuaPatternRaw(s string, pattern *stdlibstring.SimplePattern, repl string, maxRepl int, count *int) string {
	return stdlibstring.ReplaceSimpleLuaPatternRaw(s, pattern, repl, maxRepl, count)
}

func callSimpleReplacementFunction(s string, m stdlibstring.SimplePatternMatch, fn Value, caller ScriptFunctionCaller) (string, error) {
	args := simpleReplacementFunctionArgsStack(s, m)
	results, err := callGScriptFunction(fn, args, caller)
	if err != nil {
		return "", err
	}
	replacement := s[m.Start:m.End]
	if len(results) == 0 {
		return replacement, nil
	}
	val := results[0]
	if val.IsNil() || (val.IsBool() && !val.Bool()) {
		return replacement, nil
	}
	if val.IsString() || val.IsNumber() {
		return val.String(), nil
	}
	return "", fmt.Errorf("invalid replacement value (a %s)", val.TypeName())
}

func simpleReplacementFunctionArgsStack(s string, m stdlibstring.SimplePatternMatch) []Value {
	var args [4]Value
	if m.NCapture == 0 {
		args[0] = StringValue(s[m.Start:m.End])
		return args[:1]
	}
	n := m.NCapture
	if n > len(args) {
		n = len(args)
	}
	for i := 0; i < n; i++ {
		args[i] = StringValue(s[m.Captures[i][0]:m.Captures[i][1]])
	}
	return args[:n]
}

func simpleReplacementFunctionArgs(s string, m stdlibstring.SimplePatternMatch) []Value {
	if m.NCapture == 0 {
		return []Value{StringValue(s[m.Start:m.End])}
	}
	args := make([]Value, 0, m.NCapture)
	for i := 0; i < m.NCapture; i++ {
		args = append(args, StringValue(s[m.Captures[i][0]:m.Captures[i][1]]))
	}
	return args
}

func simpleReplacementTableKey(s string, m stdlibstring.SimplePatternMatch) Value {
	if m.NCapture == 0 {
		return StringValue(s[m.Start:m.End])
	}
	return StringValue(s[m.Captures[0][0]:m.Captures[0][1]])
}

func callLuaReplacementFunction(s string, loc []int, prog luaPatternProgram, fn Value, caller ScriptFunctionCaller) (string, error) {
	results, err := callGScriptFunction(fn, luaReplacementFunctionArgs(s, loc, prog), caller)
	if err != nil {
		return "", err
	}
	replacement := s[loc[0]:loc[1]]
	if len(results) == 0 {
		return replacement, nil
	}
	val := results[0]
	if val.IsNil() || (val.IsBool() && !val.Bool()) {
		return replacement, nil
	}
	if val.IsString() || val.IsNumber() {
		return val.String(), nil
	}
	return "", fmt.Errorf("invalid replacement value (a %s)", val.TypeName())
}

func callGScriptFunction(fn Value, args []Value, caller ScriptFunctionCaller) ([]Value, error) {
	if caller != nil {
		return caller(fn, args)
	}
	if gf := fn.GoFunction(); gf != nil {
		return gf.Fn(args)
	}
	return nil, fmt.Errorf("attempt to call a %s value", fn.TypeName())
}

func luaReplacementFunctionArgs(s string, loc []int, prog luaPatternProgram) []Value {
	if len(prog.captureKinds) == 0 || len(prog.captureSlots) == 0 {
		return []Value{StringValue(s[loc[0]:loc[1]])}
	}
	args := make([]Value, 0, len(prog.captureSlots))
	for i, slot := range prog.captureSlots {
		pos := slot * 2
		if pos+1 >= len(loc) || loc[pos] < 0 {
			args = append(args, NilValue())
			continue
		}
		if prog.captureKinds[i] == luaPatternCapturePosition {
			args = append(args, IntValue(int64(loc[pos]+1)))
		} else {
			args = append(args, StringValue(s[loc[pos]:loc[pos+1]]))
		}
	}
	return args
}

func luaReplacementTableKey(s string, loc []int, prog luaPatternProgram) Value {
	if len(prog.captureKinds) == 0 || len(prog.captureSlots) == 0 {
		return StringValue(s[loc[0]:loc[1]])
	}
	slot := prog.captureSlots[0]
	pos := slot * 2
	if pos+1 >= len(loc) || loc[pos] < 0 {
		return StringValue("")
	}
	if prog.captureKinds[0] == luaPatternCapturePosition {
		return IntValue(int64(loc[pos] + 1))
	}
	return StringValue(s[loc[pos]:loc[pos+1]])
}

func validateLuaReplacementString(repl string, captureCount int) error {
	return stdlibstring.ValidateLuaReplacementString(repl, captureCount)
}

func expandLuaReplacement(s string, loc []int, prog luaPatternProgram, repl string) string {
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
			} else if idx == 1 && len(prog.captureSlots) == 0 {
				b.WriteString(s[loc[0]:loc[1]])
			} else if idx > 0 && idx <= len(prog.captureSlots) {
				slot := prog.captureSlots[idx-1]
				pos := slot * 2
				if prog.captureKinds[idx-1] == luaPatternCapturePosition && pos+1 < len(loc) && loc[pos] >= 0 {
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

// isRegexMeta returns true if the byte is a Go regex metacharacter that
// needs escaping in a literal context.
func isRegexMeta(c byte) bool {
	switch c {
	case '\\', '{', '}', '|':
		return true
	}
	return false
}
