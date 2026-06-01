package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/stdlibrt/host"
	"github.com/never-labs/gscript/internal/support/outputlimit"
)

// BuildProcessWithPolicy creates the "process" standard library table.
func BuildProcessWithPolicy(opts host.Options) *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{
			Name: "process." + name,
			Fn:   fn,
		}))
	}

	set("run", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'process.run'")
		}
		if !host.Bool(opts.ProcessExecution, true) {
			return nil, fmt.Errorf("process execution access disabled")
		}

		var done *runtime.Channel
		var errFn Value
		argOffset := 0
		if d, e, ok := runtime.ScriptContextDoneAndErr(args[0]); ok {
			done = d
			errFn = e
			argOffset = 1
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument #2 to 'process.run'")
			}
		}

		var cmdArgs []string
		cmdVal := args[argOffset]
		if cmdVal.IsString() {
			cmdArgs = strings.Fields(cmdVal.Str())
		} else if cmdVal.IsTable() {
			tbl := cmdVal.Table()
			length := tbl.Length()
			for i := int64(1); i <= int64(length); i++ {
				cmdArgs = append(cmdArgs, tbl.RawGet(IntValue(i)).String())
			}
		} else {
			return nil, fmt.Errorf("bad argument #%d to 'process.run' (string or table expected)", argOffset+1)
		}
		if len(cmdArgs) == 0 {
			return nil, fmt.Errorf("process.run: empty command")
		}

		var stdinStr string
		var envVars []string
		var dir string
		var timeout time.Duration

		optsIndex := argOffset + 1
		if len(args) > optsIndex && args[optsIndex].IsTable() {
			runOpts := args[optsIndex].Table()
			if v := runOpts.RawGetString("stdin"); v.IsString() {
				stdinStr = v.Str()
			}
			if v := runOpts.RawGetString("dir"); v.IsString() {
				dir = v.Str()
				if host.String(opts.FilesystemRoot) != "" && opts.ResolveFilesystemPath != nil {
					resolved, err := opts.ResolveFilesystemPath(dir)
					if err != nil {
						return nil, err
					}
					dir = resolved
				}
			}
			if v := runOpts.RawGetString("timeout"); v.IsNumber() {
				timeout = time.Duration(toFloat(v) * float64(time.Second))
			}
			if v := runOpts.RawGetString("env"); v.IsTable() {
				if !host.Bool(opts.EnvironmentWrite, true) {
					return nil, fmt.Errorf("environment write access disabled")
				}
				envTbl := v.Table()
				k, val, ok := envTbl.Next(NilValue())
				for ok {
					name := k.String()
					if opts.EnvironmentAllowed != nil && !opts.EnvironmentAllowed(name) {
						return nil, fmt.Errorf("environment variable not allowed: %s", name)
					}
					envVars = append(envVars, name+"="+val.String())
					k, val, ok = envTbl.Next(k)
				}
			}
		}

		ctx := context.Background()
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, timeout)
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		defer cancel()

		stopWatch := make(chan struct{})
		if done != nil {
			defer close(stopWatch)
			go func() {
				_, _, stopped := done.RecvOrStop(stopWatch)
				if stopped {
					return
				}
				cancel()
			}()
		}

		cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
		stdout, stderr := outputlimit.NewBuffers(hostResultLimit(opts.MaxHostResult))
		cmd.Stdout = stdout
		cmd.Stderr = stderr
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
		if stdout.Exceeded() || stderr.Exceeded() {
			return nil, fmt.Errorf("host result byte limit exceeded (%d)", stdout.Limit())
		}
		exitCode := 0
		ok := true
		cancelled := false
		errVal := NilValue()
		if err != nil {
			ok = false
			if ctx.Err() != nil {
				cancelled = true
				if done != nil {
					errVal = runtime.ScriptContextErrValue(errFn)
				} else if ctx.Err() == context.DeadlineExceeded {
					errVal = StringValue("deadline exceeded")
				} else {
					errVal = StringValue(ctx.Err().Error())
				}
			} else {
				errVal = StringValue(err.Error())
			}
			if exitErr, isExit := err.(*exec.ExitError); isExit {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}

		result := NewTable()
		result.RawSetString("ok", BoolValue(ok))
		result.RawSetString("stdout", StringValue(stdout.String()))
		result.RawSetString("stderr", StringValue(stderr.String()))
		result.RawSetString("code", IntValue(int64(exitCode)))
		result.RawSetString("cancelled", BoolValue(cancelled))
		result.RawSetString("err", errVal)
		return []Value{TableValue(result)}, nil
	})

	set("exec", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'process.exec' (string expected)")
		}
		if !host.Bool(opts.ProcessExecution, true) {
			return nil, fmt.Errorf("process execution access disabled")
		}
		cmdArgs := make([]string, len(args))
		for i, a := range args {
			cmdArgs[i] = a.String()
		}
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		stdout, stderr := outputlimit.NewBuffers(hostResultLimit(opts.MaxHostResult))
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()
		if stdout.Exceeded() || stderr.Exceeded() {
			return nil, fmt.Errorf("host result byte limit exceeded (%d)", stdout.Limit())
		}
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(stdout.String())}, nil
	})

	set("shell", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'process.shell' (string expected)")
		}
		if !host.Bool(opts.ProcessShell, true) {
			return nil, fmt.Errorf("process shell access disabled")
		}
		cmd := exec.Command("/bin/sh", "-c", args[0].Str())
		stdout, stderr := outputlimit.NewBuffers(hostResultLimit(opts.MaxHostResult))
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		err := cmd.Run()
		if stdout.Exceeded() || stderr.Exceeded() {
			return nil, fmt.Errorf("host result byte limit exceeded (%d)", stdout.Limit())
		}
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
		result.RawSetString("ok", BoolValue(ok))
		result.RawSetString("stdout", StringValue(stdout.String()))
		result.RawSetString("stderr", StringValue(stderr.String()))
		result.RawSetString("code", IntValue(int64(exitCode)))
		return []Value{TableValue(result)}, nil
	})

	set("which", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'process.which' (string expected)")
		}
		if !host.Bool(opts.ProcessExecution, true) {
			return nil, fmt.Errorf("process execution access disabled")
		}
		path, err := exec.LookPath(args[0].Str())
		if err != nil {
			return []Value{NilValue()}, nil
		}
		return []Value{StringValue(path)}, nil
	})

	set("pid", func(args []Value) ([]Value, error) {
		return []Value{IntValue(int64(os.Getpid()))}, nil
	})

	set("env", func(args []Value) ([]Value, error) {
		if !host.Bool(opts.EnvironmentRead, true) {
			return nil, fmt.Errorf("environment read access disabled")
		}
		tbl := NewTable()
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 && (opts.EnvironmentAllowed == nil || opts.EnvironmentAllowed(parts[0])) {
				tbl.RawSetString(parts[0], StringValue(parts[1]))
			}
		}
		return []Value{TableValue(tbl)}, nil
	})

	set("args", func(args []Value) ([]Value, error) {
		tbl := NewTable()
		argv := os.Args
		if opts.Args != nil {
			if hostArgs := opts.Args(); len(hostArgs) > 0 {
				argv = hostArgs
			}
		}
		for i, arg := range argv {
			tbl.RawSet(IntValue(int64(i)), StringValue(arg))
		}
		return []Value{TableValue(tbl)}, nil
	})

	set("entry", func(args []Value) ([]Value, error) {
		tbl := NewTable()
		if opts.Args != nil {
			if argv := opts.Args(); len(argv) > 0 {
				tbl.RawSetString("file", StringValue(argv[0]))
			} else {
				tbl.RawSetString("file", NilValue())
			}
		} else {
			tbl.RawSetString("file", NilValue())
		}
		tbl.RawSetString("dir", StringValue(host.String(opts.ScriptDir)))
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

	set("setArgs", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'process.setArgs' (string expected)")
		}
		if opts.SetArgs == nil {
			return nil, fmt.Errorf("process.setArgs requires an interpreter-backed process library")
		}
		argv := make([]string, 0, len(args)-1)
		for _, arg := range args[1:] {
			argv = append(argv, arg.String())
		}
		opts.SetArgs(args[0].Str(), argv)
		return nil, nil
	})

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
		return nil, &runtime.ProcessExitError{Code: code}
	})

	return markStdlibrtModule(t)
}
