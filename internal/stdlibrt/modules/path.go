package modules

import (
	"fmt"
	"github.com/never-labs/gscript/internal/runtime"

	pathlib "github.com/never-labs/gscript/internal/stdlib/path"
)

// buildPathLib creates the "path" standard library table.
func BuildPath() *runtime.Table {
	t := runtime.NewTable()

	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name: "path." + name,
			Fn:   fn,
		}))
	}

	// Constants
	t.RawSet(runtime.StringValue("separator"), runtime.StringValue(pathlib.Separator()))
	t.RawSet(runtime.StringValue("listSeparator"), runtime.StringValue(pathlib.ListSeparator()))

	// path.join(...) -> string
	set("join", func(args []runtime.Value) ([]runtime.Value, error) {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, a.Str())
		}
		return []runtime.Value{runtime.StringValue(pathlib.Join(parts...))}, nil
	})

	// path.dir(p) -> string
	set("dir", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'path.dir' (string expected)")
		}
		return []runtime.Value{runtime.StringValue(pathlib.Dir(args[0].Str()))}, nil
	})

	// path.base(p) -> string
	set("base", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'path.base' (string expected)")
		}
		return []runtime.Value{runtime.StringValue(pathlib.Base(args[0].Str()))}, nil
	})

	// path.ext(p) -> string
	set("ext", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'path.ext' (string expected)")
		}
		return []runtime.Value{runtime.StringValue(pathlib.Ext(args[0].Str()))}, nil
	})

	// path.abs(p) -> string or nil, errMsg
	set("abs", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'path.abs' (string expected)")
		}
		abs, err := pathlib.Abs(args[0].Str())
		if err != nil {
			return []runtime.Value{runtime.NilValue(), runtime.StringValue(err.Error())}, nil
		}
		return []runtime.Value{runtime.StringValue(abs)}, nil
	})

	setIsAbs := func(name string) {
		set(name, func(args []runtime.Value) ([]runtime.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("bad argument #1 to 'path.%s' (string expected)", name)
			}
			return []runtime.Value{runtime.BoolValue(pathlib.IsAbs(args[0].Str()))}, nil
		})
	}

	// path.isAbs(p) -> bool
	setIsAbs("isAbs")

	// path.isabs(p) -> bool
	setIsAbs("isabs")

	// path.clean(p) -> string
	set("clean", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'path.clean' (string expected)")
		}
		return []runtime.Value{runtime.StringValue(pathlib.Clean(args[0].Str()))}, nil
	})

	// path.split(p) -> dir, file
	set("split", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'path.split' (string expected)")
		}
		dir, file := pathlib.Split(args[0].Str())
		return []runtime.Value{runtime.StringValue(dir), runtime.StringValue(file)}, nil
	})

	// path.match(pattern, name) -> bool, errMsg
	set("match", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'path.match' (pattern and name expected)")
		}
		matched, err := pathlib.Match(args[0].Str(), args[1].Str())
		if err != nil {
			return []runtime.Value{runtime.BoolValue(false), runtime.StringValue(err.Error())}, nil
		}
		return []runtime.Value{runtime.BoolValue(matched)}, nil
	})

	// path.rel(basepath, targpath) -> string or nil, errMsg
	set("rel", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'path.rel' (basepath and targpath expected)")
		}
		rel, err := pathlib.Rel(args[0].Str(), args[1].Str())
		if err != nil {
			return []runtime.Value{runtime.NilValue(), runtime.StringValue(err.Error())}, nil
		}
		return []runtime.Value{runtime.StringValue(rel)}, nil
	})

	return t
}
