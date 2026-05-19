package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ProcessExitError is returned by process.exit so hosts can choose whether
// to terminate the OS process, report the status, or catch it in tests.
type ProcessExitError struct {
	Code int
}

func (e *ProcessExitError) Error() string {
	return fmt.Sprintf("process exit %d", e.Code)
}

// buildProcessLib creates the "process" standard library table.
func buildProcessLib(interps ...*Interpreter) *Table {
	t := NewTable()
	var interp *Interpreter
	if len(interps) > 0 {
		interp = interps[0]
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "process." + name,
			Fn:   fn,
		}))
	}

	// process.run(cmd [, opts]) -- run command, return {ok, stdout, stderr, code}
	// opts: {stdin=str, env={}, dir=str, timeout=seconds}
	// cmd can be a string (split by spaces) or a table of args
	set("run", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'process.run'")
		}

		var cmdArgs []string
		if args[0].IsString() {
			cmdArgs = strings.Fields(args[0].Str())
		} else if args[0].IsTable() {
			tbl := args[0].Table()
			length := tbl.Length()
			for i := int64(1); i <= int64(length); i++ {
				cmdArgs = append(cmdArgs, tbl.RawGet(IntValue(i)).String())
			}
		} else {
			return nil, fmt.Errorf("bad argument #1 to 'process.run' (string or table expected)")
		}

		if len(cmdArgs) == 0 {
			return nil, fmt.Errorf("process.run: empty command")
		}

		var stdinStr string
		var envVars []string
		var dir string
		var timeout time.Duration

		if len(args) >= 2 && args[1].IsTable() {
			opts := args[1].Table()
			if v := opts.RawGet(StringValue("stdin")); v.IsString() {
				stdinStr = v.Str()
			}
			if v := opts.RawGet(StringValue("dir")); v.IsString() {
				dir = v.Str()
			}
			if v := opts.RawGet(StringValue("timeout")); v.IsNumber() {
				timeout = time.Duration(toFloat(v) * float64(time.Second))
			}
			if v := opts.RawGet(StringValue("env")); v.IsTable() {
				envTbl := v.Table()
				k, val, ok := envTbl.Next(NilValue())
				for ok {
					envVars = append(envVars, k.String()+"="+val.String())
					k, val, ok = envTbl.Next(k)
				}
			}
		}

		var ctx context.Context
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			defer cancel()
		} else {
			ctx = context.Background()
		}

		cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if stdinStr != "" {
			cmd.Stdin = strings.NewReader(stdinStr)
		}
		if dir != "" {
			cmd.Dir = dir
		}
		if len(envVars) > 0 {
			cmd.Env = append(os.Environ(), envVars...)
		}

		err := cmd.Run()
		exitCode := 0
		ok := true
		if err != nil {
			ok = false
			if exitErr, isExit := err.(*exec.ExitError); isExit {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}

		result := NewTable()
		result.RawSet(StringValue("ok"), BoolValue(ok))
		result.RawSet(StringValue("stdout"), StringValue(stdout.String()))
		result.RawSet(StringValue("stderr"), StringValue(stderr.String()))
		result.RawSet(StringValue("code"), IntValue(int64(exitCode)))

		return []Value{TableValue(result)}, nil
	})

	// process.exec(cmd, ...) -- run command with args, return stdout string or nil,err
	set("exec", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'process.exec' (string expected)")
		}
		cmdArgs := make([]string, len(args))
		for i, a := range args {
			cmdArgs[i] = a.String()
		}
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		out, err := cmd.Output()
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(string(out))}, nil
	})

	// process.shell(cmd) -- run via shell (/bin/sh -c), return {ok, stdout, stderr, code}
	set("shell", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'process.shell' (string expected)")
		}
		cmd := exec.Command("/bin/sh", "-c", args[0].Str())
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		exitCode := 0
		ok := true
		if err != nil {
			ok = false
			if exitErr, isExit := err.(*exec.ExitError); isExit {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}

		result := NewTable()
		result.RawSet(StringValue("ok"), BoolValue(ok))
		result.RawSet(StringValue("stdout"), StringValue(stdout.String()))
		result.RawSet(StringValue("stderr"), StringValue(stderr.String()))
		result.RawSet(StringValue("code"), IntValue(int64(exitCode)))

		return []Value{TableValue(result)}, nil
	})

	// process.which(name) -- find executable in PATH, return path or nil
	set("which", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'process.which' (string expected)")
		}
		path, err := exec.LookPath(args[0].Str())
		if err != nil {
			return []Value{NilValue()}, nil
		}
		return []Value{StringValue(path)}, nil
	})

	// process.pid() -- current process ID
	set("pid", func(args []Value) ([]Value, error) {
		return []Value{IntValue(int64(os.Getpid()))}, nil
	})

	// process.env() -- return table of all environment variables
	set("env", func(args []Value) ([]Value, error) {
		tbl := NewTable()
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				tbl.RawSet(StringValue(parts[0]), StringValue(parts[1]))
			}
		}
		return []Value{TableValue(tbl)}, nil
	})

	// process.args() -- return the current script entrypoint arguments.
	set("args", func(args []Value) ([]Value, error) {
		tbl := NewTable()
		if interp != nil && len(interp.args) > 0 {
			tbl.RawSet(IntValue(0), StringValue(interp.args[0]))
			for i, arg := range interp.args[1:] {
				tbl.RawSet(IntValue(int64(i+1)), StringValue(arg))
			}
			return []Value{TableValue(tbl)}, nil
		}
		for i, arg := range os.Args {
			tbl.RawSet(IntValue(int64(i)), StringValue(arg))
		}
		return []Value{TableValue(tbl)}, nil
	})

	// process.entry() -- return {file, dir, args} for the current script.
	set("entry", func(args []Value) ([]Value, error) {
		tbl := NewTable()
		if interp != nil && len(interp.args) > 0 {
			tbl.RawSetString("file", StringValue(interp.args[0]))
		} else {
			tbl.RawSetString("file", NilValue())
		}
		if interp != nil {
			tbl.RawSetString("dir", StringValue(interp.ScriptDir()))
		} else {
			tbl.RawSetString("dir", StringValue(""))
		}
		argsFn := t.RawGetString("args").GoFunction()
		argVals, err := argsFn.Fn(nil)
		if err != nil {
			return nil, err
		}
		if len(argVals) > 0 {
			tbl.RawSetString("args", argVals[0])
		}
		return []Value{TableValue(tbl)}, nil
	})

	// process.setArgs(script, args...) -- update script arguments for embedders/tests.
	set("setArgs", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'process.setArgs' (string expected)")
		}
		if interp == nil {
			return nil, fmt.Errorf("process.setArgs requires an interpreter-backed process library")
		}
		argv := make([]string, 0, len(args)-1)
		for _, arg := range args[1:] {
			argv = append(argv, arg.String())
		}
		interp.SetArgs(args[0].Str(), argv)
		return nil, nil
	})

	// process.exit([code]) -- signal host-controlled process termination.
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
				return nil, fmt.Errorf("bad argument #1 to 'process.exit' (number or boolean expected)")
			}
		}
		return nil, &ProcessExitError{Code: code}
	})

	return t
}
