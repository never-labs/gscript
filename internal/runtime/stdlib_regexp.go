package runtime

import (
	"fmt"
	"regexp"

	regexplib "github.com/never-labs/gscript/internal/stdlib/base/regexp"
)

func cachedStdlibRegexp(pattern string) (*regexp.Regexp, error) {
	return regexplib.Compile(pattern)
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
		m, found := regexplib.FindCompiled(re, str.Str())
		if !found {
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
		loc, found := regexplib.FindSubmatchIndexCompiled(re, s)
		if !found {
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
		matches := regexplib.FindAllStringsCompiled(re, str.Str(), int(toInt(n)))
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
		allMatches := regexplib.FindAllSubmatchIndexCompiled(re, s, int(toInt(n)))
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
		result := regexplib.ReplaceFirstCompiled(re, args[0].Str(), args[1].Str())
		return []Value{StringValue(result)}, nil
	})

	// re.replaceAll(str, repl) → string
	reReplaceAll := func(str, repl Value) (Value, error) {
		return StringValue(regexplib.ReplaceAllStringCompiled(re, str.Str(), repl.Str())), nil
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
		parts := regexplib.SplitCompiled(re, str.Str(), int(toInt(n)))
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
		matched, err := regexplib.Match(args[0].Str(), args[1].Str())
		if err != nil {
			return nil, fmt.Errorf("regexp.match: %v", err)
		}
		return []Value{BoolValue(matched)}, nil
	})

	// regexp.find(pattern, str) → string or nil
	regexpFind := func(pattern, str Value) (Value, error) {
		m, found, err := regexplib.Find(pattern.Str(), str.Str())
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.find: %v", err)
		}
		if !found {
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
		matches, err := regexplib.FindAllStrings(pattern.Str(), str.Str(), n)
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.findAll: %v", err)
		}
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
		allMatches, err := regexplib.FindAllSubmatchIndex(pattern.Str(), s, n)
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.findAllSubmatch: %v", err)
		}
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
		result, err := regexplib.ReplaceFirst(args[0].Str(), args[1].Str(), args[2].Str())
		if err != nil {
			return nil, fmt.Errorf("regexp.replace: %v", err)
		}
		return []Value{StringValue(result)}, nil
	})

	// regexp.replaceAll(pattern, str, repl) → string
	regexpReplaceAll := func(pattern, str, repl Value) (Value, error) {
		out, err := regexplib.ReplaceAllString(pattern.Str(), str.Str(), repl.Str())
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.replaceAll: %v", err)
		}
		return StringValue(out), nil
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
		parts, err := regexplib.Split(pattern.Str(), str.Str(), n)
		if err != nil {
			return NilValue(), fmt.Errorf("regexp.split: %v", err)
		}
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
