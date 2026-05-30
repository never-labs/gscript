package tests_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLanguageConformanceTranslatedCases(t *testing.T) {
	root := findRepoRoot(t)
	luaBin := os.Getenv("LUA_BIN")
	if luaBin == "" {
		var err error
		luaBin, err = exec.LookPath("lua")
		if err != nil {
			t.Skip("lua not found; set LUA_BIN to run language conformance checks")
		}
	}

	gscriptBin := filepath.Join(t.TempDir(), "gscript")
	runCommand(t, root, 60*time.Second, "go", "build", "-o", gscriptBin, "./cmd/gscript")
	checkJIT := os.Getenv("GSCRIPT_OFFICIAL_CHECK_JIT") == "1"

	caseDir := filepath.Join(root, "tests", "language")
	luaCases, err := filepath.Glob(filepath.Join(caseDir, "*.lua"))
	if err != nil {
		t.Fatalf("glob language conformance cases: %v", err)
	}
	if len(luaCases) == 0 {
		t.Fatalf("no language conformance cases found in %s", caseDir)
	}

	for _, luaFile := range luaCases {
		name := strings.TrimSuffix(filepath.Base(luaFile), ".lua")
		gsFile := filepath.Join(caseDir, name+".gs")
		if _, err := os.Stat(gsFile); err != nil {
			t.Fatalf("missing GScript translation for %s: %v", luaFile, err)
		}
		t.Run(name, func(t *testing.T) {
			want := normalizeProcessOutput(runCommand(t, root, 10*time.Second, luaBin, luaFile))
			vm := runCommandResult(root, 10*time.Second, gscriptBin, "-vm", gsFile)

			if !processOutputMatches(vm, want) {
				t.Fatalf("VM output mismatch for %s\nLua:\n%s\nGScript -vm:\n%s", name, want, vm.diagnostic())
			}
			if checkJIT {
				jit := runCommandResult(root, 10*time.Second, gscriptBin, "-jit", gsFile)
				if !processOutputMatches(jit, want) {
					t.Fatalf("JIT output mismatch for %s\nLua:\n%s\nGScript -jit:\n%s", name, want, jit.diagnostic())
				}
			}
		})
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not find repo root containing go.mod")
		}
		wd = parent
	}
}

func runCommand(t *testing.T, dir string, timeout time.Duration, name string, args ...string) string {
	t.Helper()
	result := runCommandResult(dir, timeout, name, args...)
	if result.err != nil {
		if result.timedOut {
			t.Fatalf("%s timed out after %s", commandLine(name, args), timeout)
		}
		t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", commandLine(name, args), result.err, result.stdout, result.stderr)
	}
	return result.stdout
}

type commandResult struct {
	stdout   string
	stderr   string
	err      error
	timedOut bool
}

func runCommandResult(dir string, timeout time.Duration, name string, args ...string) commandResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		err:      err,
		timedOut: ctx.Err() == context.DeadlineExceeded,
	}
}

func commandLine(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func normalizeProcessOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimSpace(s)
}

func processOutputMatches(result commandResult, want string) bool {
	return result.err == nil && normalizeProcessOutput(result.stdout) == want
}

func (r commandResult) summary() string {
	if r.timedOut {
		return "timeout"
	}
	if r.err != nil {
		return fmt.Sprintf("error=%v stdout=%q stderr=%q", r.err, normalizeProcessOutput(r.stdout), normalizeProcessOutput(r.stderr))
	}
	return fmt.Sprintf("stdout=%q", normalizeProcessOutput(r.stdout))
}

func (r commandResult) diagnostic() string {
	if r.timedOut {
		return "timeout"
	}
	if r.err != nil {
		return fmt.Sprintf("error: %v\nstdout:\n%s\nstderr:\n%s", r.err, normalizeProcessOutput(r.stdout), normalizeProcessOutput(r.stderr))
	}
	return normalizeProcessOutput(r.stdout)
}
