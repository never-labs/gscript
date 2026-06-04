package bind

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

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
			return dialectCommand(body.Str(), opts, dialectFailFast(options), maxHostResult)
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

func dialectCommand(src string, opts HostOptions, failFast bool, maxHostResult func() int64) ([]Value, error) {
	if !HostBool(opts.ProcessExecution, true) {
		return nil, fmt.Errorf("process execution access disabled")
	}
	args, parseErr := dialectlib.Shellwords(src)
	if parseErr != nil {
		return []Value{NilValue(), StringValue(parseErr.Error())}, nil
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("cmd dialect: empty command")
	}
	cmd := exec.Command(args[0], args[1:]...)
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
		return nil, fmt.Errorf("cmd dialect failed with exit code %d: %s", exitCode, strings.TrimSpace(stderr.String()))
	}
	return []Value{processResultTable(ok, stdout.String(), stderr.String(), exitCode)}, nil
}

func dialectShellwords(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if mode == "encode" || !body.IsString() {
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
	if mode != "" && mode != "decode" {
		return nil, fmt.Errorf("shellwords dialect: unknown mode %q", mode)
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
	resolved, err := resolveSandboxPath(HostString(opts.FilesystemRoot), pattern)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	matches, err := filepath.Glob(resolved)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := NewTable()
	for i, match := range matches {
		out.RawSet(IntValue(int64(i+1)), StringValue(match))
	}
	return []Value{TableValue(out)}, nil
}
