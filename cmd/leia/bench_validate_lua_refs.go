package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runBenchValidateLuaRefsCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench validate-lua-refs", flag.ContinueOnError)
	fs.SetOutput(errw)
	luaBin := fs.String("lua-bin", defaultLuaJITBinary(), "LuaJIT binary")
	timeout := fs.Int("timeout", 60, "per-reference timeout in seconds")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(errw, "leia bench validate-lua-refs: --timeout must be positive")
		return 2
	}
	root, err := findCLIRepoRootFromCWD()
	if err != nil {
		fmt.Fprintf(errw, "leia bench validate-lua-refs: %v\n", err)
		return 1
	}
	refs, err := benchLuaRefs(root)
	if err != nil {
		fmt.Fprintf(errw, "leia bench validate-lua-refs: %v\n", err)
		return 1
	}
	failures := make([]string, 0)
	for _, ref := range refs {
		status, message := benchValidateLuaRef(root, *luaBin, ref, time.Duration(*timeout)*time.Second)
		if status == "missing_lua" {
			fmt.Fprintln(errw, message)
			return 2
		}
		fmt.Fprintln(outw, message)
		if status != "ok" {
			failures = append(failures, ref+"="+status)
		}
	}
	if len(failures) > 0 {
		fmt.Fprintf(errw, "Lua reference validation failed: %s\n", strings.Join(failures, ", "))
		return 1
	}
	fmt.Fprintf(outw, "Validated %d Lua references.\n", len(refs))
	return 0
}

func defaultLuaJITBinary() string {
	if path, ok := lookupExecutable("luajit"); ok {
		return path
	}
	return "luajit"
}

func lookupExecutable(name string) (string, bool) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return path, true
		}
	}
	return "", false
}

func benchLuaRefs(root string) ([]string, error) {
	refs := make([]string, 0)
	for _, group := range benchProfileGroups {
		pattern := filepath.Join(root, "benchmarks", "lua_ref", group, "*.lua")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			refs = append(refs, group+"/"+strings.TrimSuffix(filepath.Base(path), ".lua"))
		}
	}
	return refs, nil
}

func benchValidateLuaRef(root, luaBin, name string, timeout time.Duration) (string, string) {
	if !strings.Contains(name, "/") {
		return "missing", name + ": expected domain/name"
	}
	parts := strings.SplitN(name, "/", 2)
	luaFile := filepath.Join(root, "benchmarks", "lua_ref", parts[0], parts[1]+".lua")
	if info, err := os.Stat(luaFile); err != nil || info.IsDir() {
		return "missing", fmt.Sprintf("%s: missing %s", name, relativePath(root, luaFile))
	}
	cmd := benchExecCommand(luaBin, luaFile)
	cmd.Dir = root
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := benchProfileRunCommand(cmd, timeout); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "timeout", fmt.Sprintf("%s: timeout after %ds", name, int(timeout.Seconds()))
		}
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "no such file") {
			return "missing_lua", "Lua binary not found: " + luaBin
		}
		exitCode := 1
		type exitCoder interface{ ExitCode() int }
		var ec exitCoder
		if errors.As(err, &ec) {
			exitCode = ec.ExitCode()
		}
		return "error", fmt.Sprintf("%s: exit %d\n%s", name, exitCode, lastNonEmptyLines(output.String(), 6))
	}
	if benchProfileParseTime(output.String()) == nil {
		return "no_time", fmt.Sprintf("%s: no parseable Time: ...s line\n%s", name, lastNonEmptyLines(output.String(), 6))
	}
	return "ok", name + ": ok"
}

func relativePath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

func lastNonEmptyLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	out := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return strings.Join(out, "\n")
}
