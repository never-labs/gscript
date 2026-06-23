package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorSmokeCommand(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	code := runEditorCommand([]string{"smoke"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEditorCommand smoke code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "leia editor smoke: ok") {
		t.Fatalf("stdout = %q, want ok marker", stdout.String())
	}
}

func TestEditorCheckScriptUnknownArgument(t *testing.T) {
	proc := runEditorCheckScriptForTest(t, "--bad-flag")
	if proc.ExitCode() != 2 {
		t.Fatalf("editor_check.sh --bad-flag exit = %d, output:\n%s", proc.ExitCode(), proc.Output)
	}
	if !strings.Contains(proc.Output, "unknown argument: --bad-flag") || !strings.Contains(proc.Output, "Usage: scripts/editor_check.sh") {
		t.Fatalf("unexpected output:\n%s", proc.Output)
	}
}

func TestEditorCheckScriptRequiresTreeSitter(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	localCLI := filepath.Join(root, "tools", "tree-sitter-leia", "node_modules", ".bin", "tree-sitter")
	if _, err := os.Stat(localCLI); err == nil {
		t.Skip("local tree-sitter CLI is installed")
	}
	proc := runEditorCheckScriptForTest(t, "--require-tree-sitter")
	if proc.ExitCode() != 1 {
		t.Fatalf("editor_check.sh --require-tree-sitter exit = %d, output:\n%s", proc.ExitCode(), proc.Output)
	}
	if !strings.Contains(proc.Output, "tree-sitter CLI is required") || !strings.Contains(proc.Output, "npm --prefix tools/tree-sitter-leia ci") {
		t.Fatalf("unexpected output:\n%s", proc.Output)
	}
}

func TestEditorCheckScriptAcceptsTreeSitterOnPath(t *testing.T) {
	proc := runEditorCheckScriptWithStubForTest(t, editorCheckTreeSitterStub(t), "--require-tree-sitter")
	if proc.ExitCode() != 0 {
		t.Fatalf("editor_check.sh --require-tree-sitter exit = %d, output:\n%s", proc.ExitCode(), proc.Output)
	}
	if !strings.Contains(proc.Output, "editor_check.sh: ok") {
		t.Fatalf("unexpected output:\n%s", proc.Output)
	}
}

type editorCheckProcess struct {
	Output string
	State  *os.ProcessState
}

func (p editorCheckProcess) ExitCode() int {
	if p.State == nil {
		return -1
	}
	return p.State.ExitCode()
}

func runEditorCheckScriptForTest(t *testing.T, args ...string) editorCheckProcess {
	t.Helper()
	return runEditorCheckScriptWithStubForTest(t, "", args...)
}

func runEditorCheckScriptWithStubForTest(t *testing.T, stub string, args ...string) editorCheckProcess {
	t.Helper()
	root := repoRootForBoundaryTest(t)
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"go", "node"} {
		target, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s is required to exercise editor_check.sh", tool)
		}
		if err := os.Symlink(target, filepath.Join(binDir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	if stub != "" {
		path := filepath.Join(binDir, "tree-sitter")
		if err := os.WriteFile(path, []byte(stub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", append([]string{filepath.Join(root, "scripts", "editor_check.sh")}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	out, _ := cmd.CombinedOutput()
	return editorCheckProcess{Output: string(out), State: cmd.ProcessState}
}

func editorCheckTreeSitterStub(t *testing.T) string {
	t.Helper()
	return "#!/usr/bin/env sh\n[ \"$1\" = test ] || exit 64\nexit 0\n"
}
