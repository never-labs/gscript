package bind

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	pathlib "github.com/never-labs/leia/internal/stdlib/lib/path"
	"github.com/never-labs/leia/internal/support"
	dialectlib "github.com/never-labs/leia/internal/support/dialect"
)

func registerDialectShellFS(register dialectRegisterFunc, opts HostOptions, maxHostResult func() int64) {
	register([]string{"sh"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectShell(body.Str(), opts, dialectFailFast(options), maxHostResult)
		},
	})
	register([]string{"cmd"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectCommand(body, options, opts, dialectFailFast(options), maxHostResult)
		},
	})
	register([]string{"shellwords"}, dialectHandler{
		eval:  dialectShellwords,
		block: dialectShellwords,
	})
	register([]string{"glob"}, dialectHandler{
		eval: func(body Value, _ *Table) ([]Value, error) {
			return dialectGlob(body.Str(), opts)
		},
	})
	register([]string{"path"}, dialectHandler{
		eval: func(body Value, _ *Table) ([]Value, error) {
			return []Value{StringValue(pathlib.Clean(body.Str()))}, nil
		},
	})
}

func dialectShell(src string, opts HostOptions, failFast bool, maxHostResult func() int64) ([]Value, error) {
	if !HostBool(opts.ProcessShell, true) {
		return nil, fmt.Errorf("process shell access disabled")
	}
	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", src)
	stdout, stderr := support.NewOutputBuffers(hostResultLimit(maxHostResult))
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
	if failFast && !ok {
		return nil, fmt.Errorf("sh dialect failed with exit code %d: %s", exitCode, strings.TrimSpace(stderr.String()))
	}
	return []Value{processResultTable(ok, stdout.String(), stderr.String(), exitCode)}, nil
}

func dialectCommand(body Value, runOpts *Table, opts HostOptions, failFast bool, maxHostResult func() int64) ([]Value, error) {
	if !HostBool(opts.ProcessExecution, true) {
		return nil, fmt.Errorf("process execution access disabled")
	}
	args, stdin, dir, envVars, timeout, parseErr := dialectCommandSpec(body, runOpts, opts)
	if parseErr != nil {
		return []Value{NilValue(), StringValue(parseErr.Error())}, nil
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("cmd dialect: empty command")
	}
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	stdout, stderr := support.NewOutputBuffers(hostResultLimit(maxHostResult))
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
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
	if err != nil {
		ok = false
		if exitErr, isExit := err.(*exec.ExitError); isExit {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	if failFast && !ok {
		return nil, fmt.Errorf("cmd dialect failed with exit code %d: %s", exitCode, strings.TrimSpace(stderr.String()))
	}
	return []Value{processResultTable(ok, stdout.String(), stderr.String(), exitCode)}, nil
}

func dialectCommandSpec(body Value, opts *Table, hostOpts HostOptions) ([]string, string, string, []string, time.Duration, error) {
	var args []string
	var stdin string
	var dir string
	var envVars []string
	var timeout time.Duration

	if body.IsString() {
		parsed, err := dialectlib.Shellwords(body.Str())
		args = parsed
		if err != nil {
			return nil, "", "", nil, 0, err
		}
	} else if body.IsTable() {
		tbl := body.Table()
		args = dialectCommandArgsFromTable(tbl)
		if len(args) == 0 {
			if cmdName := firstStringField(tbl, "cmd", "command", "name", "exe", "program"); cmdName != "" {
				args = append(args, cmdName)
			}
			if argv := tbl.RawGetString("args"); argv.IsTable() {
				args = append(args, stringArrayFromSequentialTable(argv.Table())...)
			}
		}
		if v := tbl.RawGetString("stdin"); v.IsString() {
			stdin = v.Str()
		}
		if v := tbl.RawGetString("dir"); v.IsString() {
			dir = v.Str()
		} else if v := tbl.RawGetString("cwd"); v.IsString() {
			dir = v.Str()
		}
		if v := tbl.RawGetString("timeout"); v.IsNumber() {
			timeout = time.Duration(v.Number() * float64(time.Second))
		}
		if v := tbl.RawGetString("env"); v.IsTable() {
			env, err := dialectCommandEnv(v.Table(), hostOpts)
			if err != nil {
				return nil, "", "", nil, 0, err
			}
			envVars = env
		}
	} else {
		return nil, "", "", nil, 0, fmt.Errorf("cmd dialect: string or table expected")
	}

	if opts != nil {
		if v := opts.RawGetString("stdin"); v.IsString() {
			stdin = v.Str()
		}
		if v := opts.RawGetString("dir"); v.IsString() {
			dir = v.Str()
		} else if v := opts.RawGetString("cwd"); v.IsString() {
			dir = v.Str()
		}
		if v := opts.RawGetString("timeout"); v.IsNumber() {
			timeout = time.Duration(v.Number() * float64(time.Second))
		}
		if v := opts.RawGetString("env"); v.IsTable() {
			env, err := dialectCommandEnv(v.Table(), hostOpts)
			if err != nil {
				return nil, "", "", nil, 0, err
			}
			envVars = env
		}
	}

	if dir != "" && HostString(hostOpts.FilesystemRoot) != "" && hostOpts.ResolveFilesystemPath != nil {
		resolved, err := hostOpts.ResolveFilesystemPath(dir)
		if err != nil {
			return nil, "", "", nil, 0, err
		}
		dir = resolved
	}
	return args, stdin, dir, envVars, timeout, nil
}

func dialectCommandArgsFromTable(tbl *Table) []string {
	args := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		v := tbl.RawGetInt(int64(i))
		if v.IsNil() {
			break
		}
		args = append(args, v.String())
	}
	return args
}

func stringArrayFromSequentialTable(tbl *Table) []string {
	out := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		v := tbl.RawGetInt(int64(i))
		if v.IsNil() {
			break
		}
		out = append(out, v.String())
	}
	return out
}

func dialectCommandEnv(tbl *Table, opts HostOptions) ([]string, error) {
	if !HostBool(opts.EnvironmentWrite, true) {
		return nil, fmt.Errorf("environment write access disabled")
	}
	envVars := make([]string, 0)
	for key, val, ok := tbl.Next(NilValue()); ok; key, val, ok = tbl.Next(key) {
		name := key.String()
		if opts.EnvironmentAllowed != nil && !opts.EnvironmentAllowed(name) {
			return nil, fmt.Errorf("environment variable not allowed: %s", name)
		}
		envVars = append(envVars, name+"="+val.String())
	}
	return envVars, nil
}

func dialectShellwords(body Value, opts *Table) ([]Value, error) {
	mode := dialectMode(opts)
	if !dialectModeAllowed(mode, "", "parse", "decode", "encode", "format") {
		return dialectUnknownMode("shellwords", mode)
	}
	if mode == "encode" || mode == "format" || !body.IsString() {
		args, err := shellwordsArgs(body)
		if err != nil {
			return nil, err
		}
		encoded, err := dialectlib.ShellwordsEncode(args)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(encoded)}, nil
	}
	args, err := dialectlib.Shellwords(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{shellwordsTable(args)}, nil
}

func shellwordsTable(args []string) Value {
	out := NewAppendArrayTable(len(args))
	for i, arg := range args {
		out.RawSetInt(int64(i+1), StringValue(arg))
	}
	return TableValue(out)
}

func shellwordsArgs(body Value) ([]string, error) {
	if body.IsString() {
		return []string{body.Str()}, nil
	}
	if !body.IsTable() {
		return nil, fmt.Errorf("shellwords dialect: table or string expected")
	}
	tbl := body.Table()
	args := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		v := tbl.RawGetInt(int64(i))
		if v.IsNil() {
			return nil, fmt.Errorf("shellwords dialect: missing argument at index %d", i)
		}
		if v.IsTable() {
			return nil, fmt.Errorf("shellwords dialect: argument %d must be scalar", i)
		}
		args = append(args, v.String())
	}
	return args, nil
}

func processResultTable(ok bool, stdout, stderr string, code int) Value {
	result := NewTable()
	result.RawSetString("ok", BoolValue(ok))
	result.RawSetString("stdout", StringValue(stdout))
	result.RawSetString("stderr", StringValue(stderr))
	result.RawSetString("text", StringValue(stdout))
	result.RawSetString("code", IntValue(int64(code)))
	return TableValue(result)
}

func dialectGlob(pattern string, opts HostOptions) ([]Value, error) {
	if !HostBool(opts.FilesystemRead, true) {
		return nil, fmt.Errorf("filesystem read access disabled")
	}
	matches, err := expandHostGlobSpec(HostString(opts.FilesystemRoot), pattern)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := NewTable()
	for i, match := range matches {
		out.RawSet(IntValue(int64(i+1)), StringValue(match))
	}
	return []Value{TableValue(out)}, nil
}
