package tests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	checkJIT := os.Getenv("GSCRIPT_CONFORMANCE_CHECK_JIT") == "1" || os.Getenv("GSCRIPT_OFFICIAL_CHECK_JIT") == "1"

	cases := readManifestConformanceCases(t, root)

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			want := normalizeProcessOutput(runCommand(t, root, 10*time.Second, luaBin, testCase.LuaPath))
			vm := runCommandResult(root, 10*time.Second, gscriptBin, "-vm", testCase.GScriptPath)

			if !processOutputMatches(vm, want) {
				t.Fatalf("VM output mismatch for %s\nLua:\n%s\nGScript -vm:\n%s", testCase.Name, want, vm.diagnostic())
			}
			if checkJIT {
				jit := runCommandResult(root, 10*time.Second, gscriptBin, "-jit", testCase.GScriptPath)
				if !processOutputMatches(jit, want) {
					t.Fatalf("JIT output mismatch for %s\nLua:\n%s\nGScript -jit:\n%s", testCase.Name, want, jit.diagnostic())
				}
			}
		})
	}
}

type manifestConformanceCase struct {
	Name        string
	GScriptPath string
	LuaPath     string
}

func readManifestConformanceCases(t *testing.T, root string) []manifestConformanceCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "tests", "manifest.json"))
	if err != nil {
		t.Fatalf("read tests manifest: %v", err)
	}
	var manifest struct {
		Cases []struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			Domain string `json:"domain"`
			LuaRef string `json:"lua_ref"`
			Status string `json:"status"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode tests manifest: %v", err)
	}
	var cases []manifestConformanceCase
	seen := map[string]bool{}
	for _, entry := range manifest.Cases {
		if entry.Domain != "language" || entry.LuaRef == "" || entry.Status != "passing" {
			continue
		}
		name := filepath.Base(entry.ID)
		if seen[name] {
			t.Fatalf("tests manifest has duplicate language conformance case %q", name)
		}
		seen[name] = true
		gsPath := filepath.Join(root, filepath.FromSlash(entry.Path))
		luaPath := filepath.Join(root, filepath.FromSlash(entry.LuaRef))
		if _, err := os.Stat(gsPath); err != nil {
			t.Fatalf("manifest case %s missing GScript file %s: %v", entry.ID, entry.Path, err)
		}
		if _, err := os.Stat(luaPath); err != nil {
			t.Fatalf("manifest case %s missing Lua oracle %s: %v", entry.ID, entry.LuaRef, err)
		}
		cases = append(cases, manifestConformanceCase{
			Name:        name,
			GScriptPath: gsPath,
			LuaPath:     luaPath,
		})
	}
	if len(cases) == 0 {
		t.Fatal("tests manifest contains no passing language conformance cases with Lua oracle")
	}
	sort.Slice(cases, func(i, j int) bool {
		return cases[i].Name < cases[j].Name
	})
	return cases
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
