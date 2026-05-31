package runtime

import (
	"fmt"
	stdtime "github.com/never-labs/gscript/internal/stdlib/time"
	"os"
	"strings"
	"time"
)

// startTime is used by os.clock() to measure CPU time (approximated as wall time).
var startTime = time.Now()

func buildOSLib() *Table {
	return buildOSLibWithPolicy(true, true, nil, "", true)
}

// SetEnvironmentCapabilities controls script-side environment variable read and
// write access independently. It refreshes os in place so package.loaded.os
// observes the same policy.
func (interp *Interpreter) SetEnvironmentCapabilities(read, write bool) {
	interp.environmentRead = read
	interp.environmentWrite = write
	interp.refreshOSLib()
}

// SetEnvironmentAllowlist restricts script-side environment APIs to the named
// variables. A nil slice allows all environment variables; an empty non-nil
// slice allows none.
func (interp *Interpreter) SetEnvironmentAllowlist(names []string) {
	if names == nil {
		interp.allowedEnv = nil
	} else {
		allowed := make(map[string]bool, len(names))
		for _, name := range names {
			allowed[name] = true
		}
		interp.allowedEnv = allowed
	}
	interp.refreshOSLib()
}

func (interp *Interpreter) refreshOSLib() {
	if v, ok := interp.globals.Get("os"); ok && v.IsTable() {
		osLib := TableValue(buildOSLibWithPolicy(
			interp.environmentRead,
			interp.environmentWrite,
			interp.allowedEnv,
			interp.filesystemRoot,
			interp.filesystemWrite,
		))
		interp.globals.Define("os", osLib)
		interp.modules["os"] = osLib
		interp.markPackageLoaded("os", osLib)
	}
}

// buildOSLibWithEnvironment creates the "os" standard library table.
func buildOSLibWithEnvironment(envRead, envWrite bool, allowedEnv map[string]bool) *Table {
	return buildOSLibWithPolicy(envRead, envWrite, allowedEnv, "", true)
}

// buildOSLibWithPolicy creates the "os" standard library table.
func buildOSLibWithPolicy(envRead, envWrite bool, allowedEnv map[string]bool, fsRoot string, fsWrite bool) *Table {
	t := NewTable()
	envAllowed := func(name string) bool {
		return allowedEnv == nil || allowedEnv[name]
	}
	envDenied := func(name string) error {
		return fmt.Errorf("environment variable not allowed: %s", name)
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "os." + name,
			Fn:   fn,
		}))
	}
	setFastArg1 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "os." + name,
			Fn:       fn,
			FastArg1: fast,
		}))
	}
	setEnvRead := func(name string, fn func([]Value) ([]Value, error)) {
		set(name, func(args []Value) ([]Value, error) {
			if !envRead {
				return nil, fmt.Errorf("environment read access disabled")
			}
			return fn(args)
		})
	}
	setEnvWrite := func(name string, fn func([]Value) ([]Value, error)) {
		set(name, func(args []Value) ([]Value, error) {
			if !envWrite {
				return nil, fmt.Errorf("environment write access disabled")
			}
			return fn(args)
		})
	}
	setFSWrite := func(name string, fn func([]Value) ([]Value, error)) {
		set(name, func(args []Value) ([]Value, error) {
			if !fsWrite {
				return nil, fmt.Errorf("filesystem write access disabled")
			}
			return fn(args)
		})
	}
	resolveFSWritePath := func(path string) (string, error) {
		if !fsWrite {
			return "", fmt.Errorf("filesystem write access disabled")
		}
		return resolveSandboxPath(fsRoot, path)
	}

	// os.time() -> unix timestamp
	set("time", func(args []Value) ([]Value, error) {
		return []Value{IntValue(time.Now().Unix())}, nil
	})

	// os.clock() -> elapsed CPU time in seconds (approximated as wall time)
	set("clock", func(args []Value) ([]Value, error) {
		elapsed := time.Since(startTime).Seconds()
		return []Value{FloatValue(elapsed)}, nil
	})

	// os.date([format [, time]]) -> formatted date string
	set("date", func(args []Value) ([]Value, error) {
		format := "%c"
		if len(args) >= 1 && args[0].IsString() {
			format = args[0].Str()
		}
		var tm time.Time
		if len(args) >= 2 {
			tm = time.Unix(toInt(args[1]), 0)
		} else {
			tm = time.Now()
		}

		result := stdtime.LuaDateFormat(format, tm)
		return []Value{StringValue(result)}, nil
	})

	// os.exit([code])
	set("exit", func(args []Value) ([]Value, error) {
		code := 0
		if len(args) >= 1 && !args[0].IsNil() {
			if args[0].IsBool() {
				if !args[0].Bool() {
					code = 1
				}
			} else if args[0].IsNumber() {
				code = int(toInt(args[0]))
			} else {
				return nil, fmt.Errorf("bad argument #1 to 'os.exit' (number or boolean expected)")
			}
		}
		return nil, &ProcessExitError{Code: code}
	})

	// os.getenv(name) -> string or nil
	setEnvRead("getenv", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'os.getenv' (string expected)")
		}
		name := args[0].Str()
		if !envAllowed(name) {
			return []Value{NilValue()}, nil
		}
		val, ok := os.LookupEnv(name)
		if !ok {
			return []Value{NilValue()}, nil
		}
		return []Value{StringValue(val)}, nil
	})

	// os.remove(filename) -> true or nil, error
	setFSWrite("remove", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'os.remove' (string expected)")
		}
		path, err := resolveFSWritePath(args[0].Str())
		if err != nil {
			return nil, err
		}
		err = os.Remove(path)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// os.rename(old, new) -> true or nil, error
	setFSWrite("rename", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'os.rename' (string expected)")
		}
		oldPath, err := resolveFSWritePath(args[0].Str())
		if err != nil {
			return nil, err
		}
		newPath, err := resolveFSWritePath(args[1].Str())
		if err != nil {
			return nil, err
		}
		err = os.Rename(oldPath, newPath)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// os.tmpname() -> string
	setFSWrite("tmpname", func(args []Value) ([]Value, error) {
		dir := ""
		if fsRoot != "" {
			dir = fsRoot
		}
		f, err := os.CreateTemp(dir, "gscript_*")
		if err != nil {
			return nil, err
		}
		name := f.Name()
		f.Close()
		return []Value{StringValue(name)}, nil
	})

	// os.setenv(key, value) -- set environment variable
	setEnvWrite("setenv", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'os.setenv' (string expected)")
		}
		name := args[0].Str()
		if !envAllowed(name) {
			return nil, envDenied(name)
		}
		err := os.Setenv(name, args[1].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// os.unsetenv(key) -- unset environment variable
	setEnvWrite("unsetenv", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'os.unsetenv' (string expected)")
		}
		name := args[0].Str()
		if !envAllowed(name) {
			return nil, envDenied(name)
		}
		err := os.Unsetenv(name)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// os.environ() -- return table of environment variables
	setEnvRead("environ", func(args []Value) ([]Value, error) {
		tbl := NewTable()
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 && envAllowed(parts[0]) {
				tbl.RawSet(StringValue(parts[0]), StringValue(parts[1]))
			}
		}
		return []Value{TableValue(tbl)}, nil
	})

	// os.args() -- return table of os.Args (command line arguments)
	set("args", func(args []Value) ([]Value, error) {
		tbl := NewTable()
		for i, arg := range os.Args {
			tbl.RawSet(IntValue(int64(i+1)), StringValue(arg))
		}
		return []Value{TableValue(tbl)}, nil
	})

	// os.hostname() -- get hostname
	set("hostname", func(args []Value) ([]Value, error) {
		name, err := os.Hostname()
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(name)}, nil
	})

	// os.getpid() -- get process ID as int
	set("getpid", func(args []Value) ([]Value, error) {
		return []Value{IntValue(int64(os.Getpid()))}, nil
	})

	// os.expand(s) -- expand $VAR and ${VAR} in string using os.Expand
	osExpand := func(arg Value) (Value, error) {
		if !envRead {
			return NilValue(), fmt.Errorf("environment read access disabled")
		}
		if !arg.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'os.expand' (string expected)")
		}
		return StringValue(os.Expand(arg.Str(), func(name string) string {
			if !envAllowed(name) {
				return ""
			}
			return os.Getenv(name)
		})), nil
	}
	setFastArg1("expand", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'os.expand' (string expected)")
		}
		v, err := osExpand(args[0])
		return []Value{v}, err
	}, osExpand)

	return t
}
