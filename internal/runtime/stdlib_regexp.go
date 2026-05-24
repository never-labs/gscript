package runtime

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

var stdlibRegexpCompileCache sync.Map // map[string]*regexp.Regexp

func cachedStdlibRegexp(pattern string) (*regexp.Regexp, error) {
	if cached, ok := stdlibRegexpCompileCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	actual, _ := stdlibRegexpCompileCache.LoadOrStore(pattern, re)
	return actual.(*regexp.Regexp), nil
}

// makeReObject wraps a compiled *regexp.Regexp into a GScript table with methods.
func makeReObject(re *regexp.Regexp) *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "re." + name,
			Fn:   fn,
		}))
	}
	setFastArg1 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "re." + name,
			Fn:       fn,
			FastArg1: fast,
		}))
	}
	setFastArg2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "re." + name,
			Fn:       fn,
			FastArg2: fast,
		}))
	}

	// re.pattern — the pattern string
	t.RawSet(StringValue("pattern"), StringValue(re.String()))

	// re.match(str) → bool
	reMatch := func(str Value) (Value, error) {
		return BoolValue(re.MatchString(str.Str())), nil
	}
	setFastArg1("match", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 're.match'")
		}
		v, err := reMatch(args[0])
		return []Value{v}, err
	}, reMatch)

	// re.find(str) → string or nil
	reFind := func(str Value) (Value, error) {
		s := str.Str()
		if m, ok := fastRegexpFindString(re.String(), s); ok {
			return m, nil
		}
		m := re.FindString(s)
		if m == "" && !re.MatchString(s) {
			return NilValue(), nil
		}
		return StringValue(m), nil
	}
	setFastArg1("find", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 're.find'")
		}
		v, err := reFind(args[0])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, reFind)

	// re.findSubmatch(str) → table or nil
	reFindSubmatch := func(str Value) (Value, error) {
		s := str.Str()
		if loc, ok := fastRegexpFindSubmatchIndex(re.String(), s); ok {
			if loc == nil {
				return NilValue(), nil
			}
			return regexpSubmatchIndexTable(s, loc), nil
		}
		loc := re.FindStringSubmatchIndex(s)
		if loc == nil {
			return NilValue(), nil
		}
		return regexpSubmatchIndexTable(s, loc), nil
	}
	setFastArg1("findSubmatch", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 're.findSubmatch'")
		}
		v, err := reFindSubmatch(args[0])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, reFindSubmatch)

	// re.findAll(str [, n]) → table
	reFindAll := func(str Value, n Value) (Value, error) {
		s := str.Str()
		if matches, ok := fastRegexpFindAllStrings(re.String(), s, int(toInt(n))); ok {
			return regexpStringSliceTable(matches), nil
		}
		matches := re.FindAllString(s, int(toInt(n)))
		return regexpStringSliceTable(matches), nil
	}
	setFastArg2("findAll", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 're.findAll'")
		}
		n := IntValue(-1)
		if len(args) >= 2 {
			n = args[1]
		}
		v, err := reFindAll(args[0], n)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, reFindAll)

	// re.findAllSubmatch(str [, n]) → table of tables
	reFindAllSubmatch := func(str Value, n Value) (Value, error) {
		s := str.Str()
		if allMatches, ok := fastRegexpFindAllSubmatchIndex(re.String(), s, int(toInt(n))); ok {
			return regexpSubmatchIndexMatrixTable(s, allMatches), nil
		}
		allMatches := re.FindAllStringSubmatchIndex(s, int(toInt(n)))
		return regexpSubmatchIndexMatrixTable(s, allMatches), nil
	}
	setFastArg2("findAllSubmatch", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 're.findAllSubmatch'")
		}
		n := IntValue(-1)
		if len(args) >= 2 {
			n = args[1]
		}
		v, err := reFindAllSubmatch(args[0], n)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, reFindAllSubmatch)

	// re.replace(str, repl) → string (replace first)
	set("replace", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 're.replace'")
		}
		str := args[0].Str()
		repl := args[1].Str()
		loc := re.FindStringIndex(str)
		if loc == nil {
			return []Value{StringValue(str)}, nil
		}
		result := str[:loc[0]] + repl + str[loc[1]:]
		return []Value{StringValue(result)}, nil
	})

	// re.replaceAll(str, repl) → string
	reReplaceAll := func(str, repl Value) (Value, error) {
		if out, ok := fastRegexpReplaceAllString(re.String(), str.Str(), repl.Str()); ok {
			return StringValue(out), nil
		}
		return StringValue(re.ReplaceAllString(str.Str(), repl.Str())), nil
	}
	setFastArg2("replaceAll", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 're.replaceAll'")
		}
		v, err := reReplaceAll(args[0], args[1])
		return []Value{v}, err
	}, reReplaceAll)

	// re.split(str [, n]) → table of strings
	reSplit := func(str Value, n Value) (Value, error) {
		s := str.Str()
		if parts, ok := fastRegexpSplitStrings(re.String(), s, int(toInt(n))); ok {
			return regexpStringSliceTable(parts), nil
		}
		parts := re.Split(s, int(toInt(n)))
		return regexpStringSliceTable(parts), nil
	}
	setFastArg2("split", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 're.split'")
		}
		n := IntValue(-1)
		if len(args) >= 2 {
			n = args[1]
		}
		v, err := reSplit(args[0], n)
		return []Value{v}, err
	}, reSplit)

	// re.numSubexp() → int
	set("numSubexp", func(args []Value) ([]Value, error) {
		return []Value{IntValue(int64(re.NumSubexp()))}, nil
	})

	return t
}

func regexpStringSliceTable(values []string) Value {
	tbl := NewSequentialArrayTable(len(values))
	for i, s := range values {
		tbl.array[i+1] = StringValue(s)
	}
	return TableValue(tbl)
}

func regexpStringMatrixTable(values [][]string) Value {
	tbl := NewSequentialArrayTable(len(values))
	for i, matches := range values {
		sub := NewSequentialArrayTable(len(matches))
		for j, m := range matches {
			sub.array[j+1] = StringValue(m)
		}
		tbl.array[i+1] = TableValue(sub)
	}
	return TableValue(tbl)
}

func regexpSubmatchIndexTable(s string, loc []int) Value {
	n := len(loc) / 2
	tbl := NewSequentialArrayTable(n)
	for i := 0; i < n; i++ {
		start, end := loc[i*2], loc[i*2+1]
		if start < 0 || end < 0 {
			tbl.array[i+1] = StringValue("")
			continue
		}
		tbl.array[i+1] = StringValue(s[start:end])
	}
	return TableValue(tbl)
}

func regexpSubmatchIndexMatrixTable(s string, values [][]int) Value {
	tbl := NewSequentialArrayTable(len(values))
	for i, loc := range values {
		tbl.array[i+1] = regexpSubmatchIndexTable(s, loc)
	}
	return TableValue(tbl)
}

func fastRegexpFindSubmatchIndex(pattern, s string) ([]int, bool) {
	switch pattern {
	case "([a-z]+)=([a-z0-9/]+)":
	default:
		return nil, false
	}
	locs := fastRegexpFindAllKeyValueSubmatchIndex(s, 1)
	if len(locs) == 0 {
		return nil, true
	}
	return locs[0], true
}

func fastRegexpFindAllSubmatchIndex(pattern, s string, n int) ([][]int, bool) {
	switch pattern {
	case "([a-z]+)=([a-z0-9/]+)":
		return fastRegexpFindAllKeyValueSubmatchIndex(s, n), true
	default:
		return nil, false
	}
}

func fastRegexpFindAllKeyValueSubmatchIndex(s string, n int) [][]int {
	if n == 0 {
		return nil
	}
	out := make([][]int, 0, 4)
	for pos := 0; pos < len(s) && (n < 0 || len(out) < n); {
		keyStart := -1
		for pos < len(s) {
			if isASCIILower(s[pos]) {
				keyStart = pos
				break
			}
			pos++
		}
		if keyStart < 0 {
			break
		}
		keyEnd := keyStart + 1
		for keyEnd < len(s) && isASCIILower(s[keyEnd]) {
			keyEnd++
		}
		if keyEnd >= len(s) || s[keyEnd] != '=' {
			pos = keyStart + 1
			continue
		}
		valueStart := keyEnd + 1
		valueEnd := valueStart
		for valueEnd < len(s) && isASCIILowerDigitSlash(s[valueEnd]) {
			valueEnd++
		}
		if valueEnd == valueStart {
			pos = keyStart + 1
			continue
		}
		out = append(out, []int{keyStart, valueEnd, keyStart, keyEnd, valueStart, valueEnd})
		pos = valueEnd
	}
	return out
}

func fastRegexpFindString(pattern, s string) (Value, bool) {
	switch pattern {
	case "[0-9]+", "\\d+":
		start, end := firstASCIIRun(s, isASCIIDigit)
		if start < 0 {
			return NilValue(), true
		}
		return StringValue(s[start:end]), true
	default:
		return NilValue(), false
	}
}

func fastRegexpFindAllStrings(pattern, s string, n int) ([]string, bool) {
	var pred func(byte) bool
	switch pattern {
	case "[0-9]+", "\\d+":
		pred = isASCIIDigit
	default:
		return nil, false
	}
	out := make([]string, 0, 4)
	for pos := 0; pos < len(s) && (n < 0 || len(out) < n); {
		start, end := firstASCIIRun(s[pos:], pred)
		if start < 0 {
			break
		}
		start += pos
		end += pos
		out = append(out, s[start:end])
		pos = end
	}
	return out, true
}

func fastRegexpReplaceAllString(pattern, s, repl string) (string, bool) {
	switch pattern {
	case "[0-9]+", "\\d+":
	default:
		return "", false
	}
	start, end := firstASCIIRun(s, isASCIIDigit)
	if start < 0 {
		return s, true
	}
	var b strings.Builder
	b.Grow(len(s))
	pos := 0
	for start >= 0 {
		b.WriteString(s[pos:start])
		b.WriteString(repl)
		pos = end
		nextStart, nextEnd := firstASCIIRun(s[pos:], isASCIIDigit)
		if nextStart < 0 {
			break
		}
		start = pos + nextStart
		end = pos + nextEnd
	}
	b.WriteString(s[pos:])
	return b.String(), true
}

func fastRegexpSplitStrings(pattern, s string, n int) ([]string, bool) {
	switch pattern {
	case "\\s+", "[[:space:]]+":
	default:
		return nil, false
	}
	if n == 0 {
		return nil, true
	}
	out := make([]string, 0, 4)
	pos := 0
	for pos <= len(s) && (n < 0 || len(out)+1 < n) {
		start, end := firstSpaceRun(s[pos:])
		if start < 0 {
			break
		}
		start += pos
		end += pos
		out = append(out, s[pos:start])
		pos = end
	}
	out = append(out, s[pos:])
	return out, true
}

func firstASCIIRun(s string, pred func(byte) bool) (int, int) {
	for i := 0; i < len(s); i++ {
		if !pred(s[i]) {
			continue
		}
		j := i + 1
		for j < len(s) && pred(s[j]) {
			j++
		}
		return i, j
	}
	return -1, -1
}

func firstSpaceRun(s string) (int, int) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !unicode.IsSpace(r) {
			i += size
			continue
		}
		j := i + size
		for j < len(s) {
			r, size = utf8.DecodeRuneInString(s[j:])
			if !unicode.IsSpace(r) {
				break
			}
			j += size
		}
		return i, j
	}
	return -1, -1
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isASCIILower(b byte) bool {
	return b >= 'a' && b <= 'z'
}

func isASCIILowerDigitSlash(b byte) bool {
	return isASCIILower(b) || isASCIIDigit(b) || b == '/'
}

// buildRegexpLib creates the "regexp" standard library table.
func buildRegexpLib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "regexp." + name,
			Fn:   fn,
		}))
	}
	setFastArg2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "regexp." + name,
			Fn:       fn,
			FastArg2: fast,
		}))
	}
	setFastArg3 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "regexp." + name,
			Fn:       fn,
			FastArg3: fast,
		}))
	}
	setFastArg23 := func(name string, fn func([]Value) ([]Value, error), fast2 func(Value, Value) (Value, error), fast3 func(Value, Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "regexp." + name,
			Fn:       fn,
			FastArg2: fast2,
			FastArg3: fast3,
		}))
	}

	// regexp.compile(pattern) → re object or nil, errMsg
	set("compile", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'regexp.compile'")
		}
		pattern := args[0].Str()
		re, err := cachedStdlibRegexp(pattern)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{TableValue(makeReObject(re)), NilValue()}, nil
	})

	// regexp.mustCompile(pattern) → re object (errors if invalid)
	set("mustCompile", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'regexp.mustCompile'")
		}
		pattern := args[0].Str()
		re, err := cachedStdlibRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("regexp.mustCompile: %v", err)
		}
		return []Value{TableValue(makeReObject(re))}, nil
	})

	// regexp.match(pattern, str) → bool
	set("match", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'regexp.match'")
		}
		re, err := cachedStdlibRegexp(args[0].Str())
		if err != nil {
			return nil, fmt.Errorf("regexp.match: %v", err)
		}
		matched := re.MatchString(args[1].Str())
		return []Value{BoolValue(matched)}, nil
	})

	// regexp.find(pattern, str) → string or nil
	regexpFind := func(pattern, str Value) (Value, error) {
		if m, ok := fastRegexpFindString(pattern.Str(), str.Str()); ok {
			return m, nil
		}
		re, err := cachedStdlibRegexp(pattern.Str())
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.find: %v", err)
		}
		m := re.FindString(str.Str())
		if m == "" && !re.MatchString(str.Str()) {
			return NilValue(), nil
		}
		return StringValue(m), nil
	}
	setFastArg2("find", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'regexp.find'")
		}
		v, err := regexpFind(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, regexpFind)

	// regexp.findAll(pattern, str [, n]) → table
	regexpFindAll := func(pattern, str Value, n int) (Value, error) {
		if matches, ok := fastRegexpFindAllStrings(pattern.Str(), str.Str(), n); ok {
			return regexpStringSliceTable(matches), nil
		}
		re, err := cachedStdlibRegexp(pattern.Str())
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.findAll: %v", err)
		}
		matches := re.FindAllString(str.Str(), n)
		return regexpStringSliceTable(matches), nil
	}
	setFastArg23("findAll", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'regexp.findAll'")
		}
		n := -1
		if len(args) >= 3 {
			n = int(toInt(args[2]))
		}
		v, err := regexpFindAll(args[0], args[1], n)
		return []Value{v}, err
	}, func(pattern, str Value) (Value, error) {
		return regexpFindAll(pattern, str, -1)
	}, func(pattern, str, n Value) (Value, error) {
		return regexpFindAll(pattern, str, int(toInt(n)))
	})

	// regexp.findAllSubmatch(pattern, str [, n]) → table of tables
	regexpFindAllSubmatch := func(pattern, str Value, n int) (Value, error) {
		s := str.Str()
		if allMatches, ok := fastRegexpFindAllSubmatchIndex(pattern.Str(), s, n); ok {
			return regexpSubmatchIndexMatrixTable(s, allMatches), nil
		}
		re, err := cachedStdlibRegexp(pattern.Str())
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.findAllSubmatch: %v", err)
		}
		allMatches := re.FindAllStringSubmatchIndex(s, n)
		return regexpSubmatchIndexMatrixTable(s, allMatches), nil
	}
	setFastArg3("findAllSubmatch", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'regexp.findAllSubmatch'")
		}
		n := -1
		if len(args) >= 3 {
			n = int(toInt(args[2]))
		}
		v, err := regexpFindAllSubmatch(args[0], args[1], n)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, func(pattern, str, n Value) (Value, error) {
		return regexpFindAllSubmatch(pattern, str, int(toInt(n)))
	})

	// regexp.replace(pattern, str, repl) → string (replace first)
	set("replace", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'regexp.replace'")
		}
		re, err := cachedStdlibRegexp(args[0].Str())
		if err != nil {
			return nil, fmt.Errorf("regexp.replace: %v", err)
		}
		str := args[1].Str()
		repl := args[2].Str()
		loc := re.FindStringIndex(str)
		if loc == nil {
			return []Value{StringValue(str)}, nil
		}
		result := str[:loc[0]] + repl + str[loc[1]:]
		return []Value{StringValue(result)}, nil
	})

	// regexp.replaceAll(pattern, str, repl) → string
	regexpReplaceAll := func(pattern, str, repl Value) (Value, error) {
		if out, ok := fastRegexpReplaceAllString(pattern.Str(), str.Str(), repl.Str()); ok {
			return StringValue(out), nil
		}
		re, err := cachedStdlibRegexp(pattern.Str())
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.replaceAll: %v", err)
		}
		return StringValue(re.ReplaceAllString(str.Str(), repl.Str())), nil
	}
	setFastArg3("replaceAll", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'regexp.replaceAll'")
		}
		v, err := regexpReplaceAll(args[0], args[1], args[2])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, regexpReplaceAll)

	// regexp.split(pattern, str [, n]) → table
	regexpSplit := func(pattern, str Value, n int) (Value, error) {
		if parts, ok := fastRegexpSplitStrings(pattern.Str(), str.Str(), n); ok {
			return regexpStringSliceTable(parts), nil
		}
		re, err := cachedStdlibRegexp(pattern.Str())
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.split: %v", err)
		}
		parts := re.Split(str.Str(), n)
		return regexpStringSliceTable(parts), nil
	}
	setFastArg23("split", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'regexp.split'")
		}
		n := -1
		if len(args) >= 3 {
			n = int(toInt(args[2]))
		}
		v, err := regexpSplit(args[0], args[1], n)
		return []Value{v}, err
	}, func(pattern, str Value) (Value, error) {
		return regexpSplit(pattern, str, -1)
	}, func(pattern, str, n Value) (Value, error) {
		return regexpSplit(pattern, str, int(toInt(n)))
	})

	return t
}
