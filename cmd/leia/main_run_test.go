package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommandExecutesFileWithArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "args.leia")
	src := `assert(arg[0] == "` + path + `")
assert(arg[1] == "one")
assert(arg[2] == "two")
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runRunCommand([]string{"--vm", path, "one", "two"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runRunCommand code = %d, stderr = %q", code, stderr.String())
	}
}

func TestRunCommandExecutesDialectSyntaxFeatureFile(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "feature.txt"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEIA_CLI_DIALECT_TEST", "visible")

	globPattern := filepath.ToSlash(filepath.Join(nested, "*.txt"))
	path := filepath.Join(dir, "dialects.leia")
	src := `import "json"
import p "path"

name := "Leia"
decoded := json` + "`" + `{"name":"Ada","score":42}` + "`" + `
encoded := json {ok: true, name: name}
pattern := re!` + "`" + `^A[a-z]+$` + "`" + `
alias_pattern := regexp` + "`" + `^[0-9]+$` + "`" + `
host_env := env` + "`" + `LEIA_CLI_DIALECT_TEST` + "`" + `
shell := $` + "`" + `printf shell-${name}` + "`" + `
command := cmd` + "`" + `printf command` + "`" + `
matches := glob` + "`" + globPattern + "`" + `
prompt_cfg := prompt { role: "system", text: "Hi" }
quoted_raw := quote! { x := 1; x += 2 }

assert(decoded.name == "Ada")
assert(encoded == "{\"name\":\"Leia\",\"ok\":true}")
assert(pattern.match("Ada"))
assert(alias_pattern.match("123"))
assert(regexp.match("^Le", name))
assert(host_env == "visible")
assert(shell.text == "shell-Leia")
assert(command.text == "command")
assert(#matches == 1)
assert(p.match("nested/*.txt", p.join("nested", "feature.txt")))
assert(prompt_cfg.body.role == "system")
assert(prompt_cfg.body.text == "Hi")
assert(quoted_raw.kind == "function")
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runRunCommand([]string{"--vm", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runRunCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunCommandReportsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runRunCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runRunCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: leia run") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunCommandRejectsInvalidModuleMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runRunCommand([]string{"--mod=bad", "main.leia"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runRunCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid --mod") {
		t.Fatalf("stderr = %q, want invalid --mod", stderr.String())
	}
}

func TestRunCommandModuleModesWireCacheAndVendor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEIA_CACHE", filepath.Join(dir, "cache"))
	if err := os.MkdirAll(filepath.Join(dir, "cache", "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cache", "extract", "github.com", "acme", "toolkit@v1.2.3", "pkg", "util.leia"), []byte(`return { value: 91 }`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leia.mod"), []byte(`module example.com/app
leia 0.1
require github.com/acme/toolkit v1.2.3
`), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.leia")
	if err := os.WriteFile(path, []byte(`import "github.com/acme/toolkit/pkg/util" as u
assert(u.value == 91)
`), 0644); err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"mod", "readonly"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runRunCommand([]string{"--vm", "--mod=" + mode, path}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("runRunCommand code = %d, stderr = %q", code, stderr.String())
			}
		})
	}

	var stdout, stderr bytes.Buffer
	code := runRunCommand([]string{"--vm", "--mod=vendor", path}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("vendor mode unexpectedly loaded cache-only module")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "github.com/acme/toolkit") {
		t.Fatalf("stderr = %q, want missing module path", stderr.String())
	}
}

func TestEvalCommandExecutesSourceWithArgs(t *testing.T) {
	src := `assert(arg[0] == "<eval>")
assert(arg[1] == "one")
assert(arg[2] == "two")
`
	var stdout, stderr bytes.Buffer
	code := runEvalCommand([]string{"--vm", src, "one", "two"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvalCommand code = %d, stderr = %q", code, stderr.String())
	}
}

func TestRunPublicPathPredicateKeepsDiagnosticsInternal(t *testing.T) {
	if !canUsePublicRunPath(cliRunOptions{UseJIT: true}) {
		t.Fatal("plain JIT run should use public path")
	}
	diagnosticCases := []cliRunOptions{
		{ShowJITStats: true},
		{JIT: jitCLIOptions{TimelinePath: "timeline.jsonl"}},
		{JIT: jitCLIOptions{WarmDumpDir: "warm"}},
		{JIT: jitCLIOptions{ShowExitStats: true}},
		{JIT: jitCLIOptions{ShowExitStatsJSON: true}},
		{JIT: jitCLIOptions{ShowTier2PerfStats: true}},
		{JIT: jitCLIOptions{ShowTier2PerfStatsJSON: true}},
		{JIT: jitCLIOptions{ShowTier2SpecStateJSON: true}},
		{JIT: jitCLIOptions{ShowTier2SpecWorklistJSON: true}},
		{JIT: jitCLIOptions{ShowCoroutineStats: true}},
		{JIT: jitCLIOptions{ShowPathStats: true}},
		{JIT: jitCLIOptions{ShowPathStatsJSON: true}},
	}
	for _, opts := range diagnosticCases {
		if canUsePublicRunPath(opts) {
			t.Fatalf("diagnostic options should require internal path: %+v", opts)
		}
	}
}

func TestEvalCommandReportsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEvalCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runEvalCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: leia eval") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestDefaultCommandRejectsCPUProfileWithJIT(t *testing.T) {
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	profile := filepath.Join(t.TempDir(), "cpu.pprof")
	os.Args = []string{"leia", "-cpuprofile", profile, "-e", "print(1)"}
	flag.CommandLine = flag.NewFlagSet("leia", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	var stdout, stderr bytes.Buffer
	code := runDefaultCommand(&stdout, &stderr)
	if code != 1 {
		t.Fatalf("runDefaultCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "-cpuprofile is not supported while JIT is enabled") {
		t.Fatalf("stderr = %q, want JIT/cpuprofile rejection", stderr.String())
	}
}

func TestREPLCommandRejectsArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPLCommand([]string{"extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runREPLCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: leia repl") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}
