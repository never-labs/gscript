package leia_test

import (
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestEnvironmentCapabilities(t *testing.T) {
	t.Setenv("LEIA_PUBLIC_ENV_CAP_TEST", "visible")

	tests := []struct {
		name    string
		opts    []leia.Option
		src     string
		wantErr string
	}{
		{
			name:    "environment disabled blocks getenv",
			opts:    []leia.Option{leia.WithEnvironment(false)},
			src:     `value := os.getenv("LEIA_PUBLIC_ENV_CAP_TEST")`,
			wantErr: "environment read access disabled",
		},
		{
			name:    "read disabled blocks expand",
			opts:    []leia.Option{leia.WithEnvironmentRead(false)},
			src:     `value := os.expand("$LEIA_PUBLIC_ENV_CAP_TEST")`,
			wantErr: "environment read access disabled",
		},
		{
			name:    "write disabled blocks setenv",
			opts:    []leia.Option{leia.WithEnvironmentWrite(false)},
			src:     `ok := os.setenv("LEIA_PUBLIC_ENV_WRITE_TEST", "blocked")`,
			wantErr: "environment write access disabled",
		},
		{
			name: "read only still reads",
			opts: []leia.Option{leia.WithEnvironmentWrite(false)},
			src:  `value := os.getenv("LEIA_PUBLIC_ENV_CAP_TEST")`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(tc.opts...)
			err := vm.Exec(tc.src)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Exec error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("value")
			if err != nil {
				t.Fatal(err)
			}
			if got != "visible" {
				t.Fatalf("value = %v, want visible", got)
			}
		})
	}
}

func TestWithProcessShellFalseBlocksShell(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibProcess),
				leia.WithProcessShell(false),
			}, tc.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`result := process.shell("echo blocked")`)
			if err == nil || !strings.Contains(err.Error(), "process shell access disabled") {
				t.Fatalf("process.shell err = %v, want process shell access disabled", err)
			}
		})
	}
}

func TestWithProcessExecutionFalseBlocksRunExecAndWhich(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibProcess),
				leia.WithProcessExecution(false),
			}, tc.opts...)
			vm := leia.New(opts...)
			for _, src := range []string{
				`result := process.run("echo blocked")`,
				`result := process.exec("echo", "blocked")`,
				`result := process.which("echo")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "process execution access disabled") {
					t.Fatalf("%s err = %v, want process execution access disabled", src, err)
				}
			}
		})
	}
}

func TestWithFilesystemRootConfinesProcessRunDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibProcess),
				leia.WithFilesystemRoot(root),
			}, tc.opts...)
			vm := leia.New(opts...)
			src := fmt.Sprintf(`result := process.run(["pwd"], {dir: %q})`, outside)
			err := vm.Exec(src)
			if err == nil || !strings.Contains(err.Error(), "filesystem access denied") {
				t.Fatalf("process.run dir escape err = %v, want filesystem access denied", err)
			}
		})
	}
}

func TestProcessRunEnvFollowsEnvironmentPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name+"/write-disabled", func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibProcess),
				leia.WithEnvironmentWrite(false),
			}, tc.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`result := process.run(["pwd"], {env: {LEIA_PROCESS_ENV_POLICY_TEST: "blocked"}})`)
			if err == nil || !strings.Contains(err.Error(), "environment write access disabled") {
				t.Fatalf("process.run env err = %v, want environment write access disabled", err)
			}
		})

		t.Run(tc.name+"/allowlist", func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibProcess),
				leia.WithEnvironmentAllowlist("LEIA_PROCESS_ENV_ALLOWED"),
			}, tc.opts...)
			vm := leia.New(opts...)
			err := vm.Exec(`result := process.run(["pwd"], {env: {LEIA_PROCESS_ENV_BLOCKED: "blocked"}})`)
			if err == nil || !strings.Contains(err.Error(), "environment variable not allowed: LEIA_PROCESS_ENV_BLOCKED") {
				t.Fatalf("process.run env allowlist err = %v, want environment variable not allowed", err)
			}
		})
	}
}

func TestWithNetworkAccessFalseBlocksNetAndHTTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibNet | leia.LibHTTP),
				leia.WithNetworkAccess(false),
			}, tc.opts...)
			vm := leia.New(opts...)
			for _, src := range []string{
				`resp := net.get("http://127.0.0.1:1")`,
				`resp := net.request({url: "http://127.0.0.1:1"})`,
				`resp := http.get("http://127.0.0.1:1")`,
				`server := http.listen("127.0.0.1:0", func(req, res) {}, {background: true})`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "network access disabled") {
					t.Fatalf("%s err = %v, want network access disabled", src, err)
				}
			}
		})
	}
}

func TestWithDebugAccessFalseBlocksDebugAPIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibDebug),
				leia.WithDebugAccess(false),
			}, tc.opts...)
			vm := leia.New(opts...)
			for _, src := range []string{
				`stack := debug.stack()`,
				`globals := debug.globals()`,
				`raw := debug.goStack()`,
				`debug.setHook(func(event) {})`,
				`debug.emit("blocked")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "debug access disabled") {
					t.Fatalf("%s err = %v, want debug access disabled", src, err)
				}
			}
		})
	}
}

func TestWithTestkitAccessFalseBlocksTestkitAPIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibTestkit),
				leia.WithTestkitAccess(false),
			}, tc.opts...)
			vm := leia.New(opts...)
			for _, src := range []string{
				`stats := testkit.memory()`,
				`info := testkit.value(42)`,
				`kind := testkit.typeOf(42)`,
				`result := testkit.protect(func() { return 1 })`,
				`same := testkit.sameFunction(print, print)`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "testkit access disabled") {
					t.Fatalf("%s err = %v, want testkit access disabled", src, err)
				}
			}
		})
	}
}

func TestEnvironmentAllowlist(t *testing.T) {
	t.Setenv("LEIA_PUBLIC_ENV_ALLOWED", "visible")
	t.Setenv("LEIA_PUBLIC_ENV_BLOCKED", "secret")

	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{leia.WithEnvironmentAllowlist("LEIA_PUBLIC_ENV_ALLOWED")}, tc.opts...)
			vm := leia.New(opts...)
			if err := vm.Exec(`
				allowed := os.getenv("LEIA_PUBLIC_ENV_ALLOWED")
				blocked := os.getenv("LEIA_PUBLIC_ENV_BLOCKED")
				expanded := os.expand("$LEIA_PUBLIC_ENV_ALLOWED:$LEIA_PUBLIC_ENV_BLOCKED")
				all := os.environ()
				procEnv := process.env()
			`); err != nil {
				t.Fatal(err)
			}
			for name, want := range map[string]interface{}{
				"allowed":  "visible",
				"blocked":  nil,
				"expanded": "visible:",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("%s = %v, want %v", name, got, want)
				}
			}
			for _, tableName := range []string{"all", "procEnv"} {
				got, err := vm.Get(tableName)
				if err != nil {
					t.Fatal(err)
				}
				env, ok := got.(map[string]interface{})
				if !ok {
					t.Fatalf("%s = %T, want map", tableName, got)
				}
				if env["LEIA_PUBLIC_ENV_ALLOWED"] != "visible" {
					t.Fatalf("%s allowed = %v, want visible", tableName, env["LEIA_PUBLIC_ENV_ALLOWED"])
				}
				if _, ok := env["LEIA_PUBLIC_ENV_BLOCKED"]; ok {
					t.Fatalf("%s exposed blocked environment variable", tableName)
				}
			}
			err := vm.Exec(`os.setenv("LEIA_PUBLIC_ENV_BLOCKED", "changed")`)
			if err == nil || !strings.Contains(err.Error(), "environment variable not allowed") {
				t.Fatalf("setenv blocked err = %v, want environment variable not allowed", err)
			}
		})
	}
}
