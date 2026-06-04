package bind

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	base64lib "github.com/never-labs/leia/internal/stdlib/lib/base64"
	hashlib "github.com/never-labs/leia/internal/stdlib/lib/hash"
	pathlib "github.com/never-labs/leia/internal/stdlib/lib/path"
	"github.com/never-labs/leia/internal/support"
)

// BuildDialect creates the "dialect" standard library table. Dialects are a
// small native dispatch layer used by tagged literals and tagged blocks:
// sh`...`, cmd`...`, glob`...`, json`...`, prompt`...`, quote { ... }, and
// small safe data transforms such as path`...`, url`...`, words`...`, kv`...`,
// env`...`, html_escape`...`, urlquery`...`, base64`...`, and hash`...`.
func BuildDialect(opts HostOptions, maxHostResult func() int64) *Table {
	t := markStdlibBoundModule(NewTable())
	root := func() string { return HostString(opts.FilesystemRoot) }

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "dialect." + name, Fn: fn}))
	}

	eval := func(tag string, body Value, options *Table) ([]Value, error) {
		failFast := options != nil && options.RawGetString("fail_fast").Truthy()
		switch tag {
		case "sh":
			return dialectShell(body.Str(), opts, failFast, maxHostResult)
		case "cmd":
			return dialectCommand(body.Str(), opts, failFast, maxHostResult)
		case "glob":
			return dialectGlob(body.Str(), root())
		case "re", "regexp":
			return dialectRegexp(body.Str(), failFast)
		case "json":
			return dialectJSON(body, options)
		case "csv":
			return dialectCSV(body.Str(), options)
		case "lines", "split":
			return dialectLines(body.Str(), options)
		case "words":
			return dialectWords(body.Str()), nil
		case "kv":
			return dialectKV(body.Str(), options, false)
		case "env":
			return dialectKV(body.Str(), options, true)
		case "template":
			return dialectTemplate(body, options, maxHostResult)
		case "path":
			return []Value{StringValue(pathlib.Clean(body.Str()))}, nil
		case "url":
			return dialectURL(body.Str())
		case "html_escape":
			return dialectHTMLEscape(body.Str(), options)
		case "urlquery":
			return dialectURLQuery(body, options)
		case "base64":
			return dialectBase64(body.Str(), options, maxHostResult)
		case "hash":
			return dialectHash(body.Str(), options)
		case "prompt":
			return []Value{dialectPrompt(body, options)}, nil
		case "quote":
			return []Value{dialectQuote(tag, body, options)}, nil
		default:
			return nil, fmt.Errorf("unknown dialect %q", tag)
		}
	}

	set("eval", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.eval' (tag and body expected)")
		}
		return eval(args[0].Str(), args[1], optionalTableArg(args, 2))
	})

	set("eval_block", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.eval_block' (tag and config expected)")
		}
		tag := args[0].Str()
		optsTbl := optionalTableArg(args, 2)
		switch tag {
		case "prompt":
			return []Value{dialectPrompt(args[1], optsTbl)}, nil
		case "json":
			return dialectJSON(args[1], optsTbl)
		case "template":
			return dialectTemplate(args[1], optsTbl, maxHostResult)
		case "quote":
			return []Value{dialectQuote(tag, args[1], optsTbl)}, nil
		default:
			return eval(tag, args[1], optsTbl)
		}
	})

	set("eval_raw", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() {
			return nil, fmt.Errorf("bad arguments to 'dialect.eval_raw' (tag and thunk expected)")
		}
		optsTbl := optionalTableArg(args, 2)
		return []Value{dialectQuote(args[0].Str(), args[1], optsTbl)}, nil
	})

	return t
}

func optionalTableArg(args []Value, idx int) *Table {
	if len(args) > idx && args[idx].IsTable() {
		return args[idx].Table()
	}
	return nil
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
	args := strings.Fields(src)
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

func processResultTable(ok bool, stdout, stderr string, code int) Value {
	result := NewTable()
	result.RawSetString("ok", BoolValue(ok))
	result.RawSetString("stdout", StringValue(stdout))
	result.RawSetString("stderr", StringValue(stderr))
	result.RawSetString("text", StringValue(stdout))
	result.RawSetString("code", IntValue(int64(code)))
	return TableValue(result)
}

func dialectGlob(pattern, root string) ([]Value, error) {
	resolved, err := resolveSandboxPath(root, pattern)
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

func dialectRegexp(pattern string, failFast bool) ([]Value, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		if failFast {
			return nil, fmt.Errorf("re dialect: %v", err)
		}
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{TableValue(makeReObject(re))}, nil
}

func dialectBase64(src string, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := "encode"
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	switch mode {
	case "encode", "":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), base64lib.EncodedLen(len(src))); err != nil {
			return nil, err
		}
		return []Value{StringValue(base64lib.Encode(src))}, nil
	case "decode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), base64lib.DecodedLen(len(src))); err != nil {
			return nil, err
		}
		decoded, err := base64lib.Decode(src)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	case "url_encode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), base64lib.URLEncodedLen(len(src))); err != nil {
			return nil, err
		}
		return []Value{StringValue(base64lib.URLEncode(src))}, nil
	case "url_decode":
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), base64lib.URLDecodedLen(len(src))); err != nil {
			return nil, err
		}
		decoded, err := base64lib.URLDecode(src)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	default:
		return nil, fmt.Errorf("base64 dialect: unknown mode %q", mode)
	}
}

func dialectHash(src string, opts *Table) ([]Value, error) {
	algo := "sha256"
	if opts != nil && opts.RawGetString("algo").IsString() {
		algo = opts.RawGetString("algo").Str()
	}
	switch strings.ToLower(algo) {
	case "md5":
		return []Value{StringValue(hashlib.MD5(src))}, nil
	case "sha1":
		return []Value{StringValue(hashlib.SHA1(src))}, nil
	case "sha256", "":
		return []Value{StringValue(hashlib.SHA256(src))}, nil
	case "sha512":
		return []Value{StringValue(hashlib.SHA512(src))}, nil
	case "crc32":
		return []Value{IntValue(int64(hashlib.CRC32(src)))}, nil
	default:
		return nil, fmt.Errorf("hash dialect: unknown algorithm %q", algo)
	}
}

func dialectPrompt(body Value, opts *Table) Value {
	out := NewTable()
	out.RawSetString("text", StringValue(body.String()))
	out.RawSetString("body", body)
	if opts != nil {
		out.RawSetString("options", TableValue(opts))
		if role := opts.RawGetString("role"); role.IsString() {
			out.RawSetString("role", role)
		}
	}
	return TableValue(out)
}

func dialectQuote(tag string, body Value, opts *Table) Value {
	out := NewTable()
	out.RawSetString("dialect", StringValue(tag))
	out.RawSetString("body", body)
	out.RawSetString("kind", StringValue(body.TypeName()))
	if opts != nil {
		out.RawSetString("options", TableValue(opts))
	}
	return TableValue(out)
}
