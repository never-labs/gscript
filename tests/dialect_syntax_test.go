package tests_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestProcessDialectSyntaxExecutesThroughStdlib(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibAll),
				leia.WithProcessExecution(true),
				leia.WithProcessShell(true),
			}, tc.opts...)...)
			src := "import \"json\"\n" +
				"import p \"path\"\n\n" +
				"name := \"leia\"\n" +
				"shell := $`printf hello-${name}`\n" +
				"cmd_out := cmd`printf cmd-ok`\n" +
				"matches := glob`dialect_syntax_test.go`\n" +
				"digits := re!`^[0-9]+$`\n" +
				"shell_text := shell.text\n" +
				"cmd_text := cmd_out.text\n" +
				"glob_first := matches[1]\n" +
				"match_ok := digits.match(\"123\")\n" +
				"joined := p.join(\"a\", \"b\")\n"
			err := vm.Exec(src)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "shell_text", "hello-leia")
			assertGet(t, vm, "cmd_text", "cmd-ok")
			assertGet(t, vm, "match_ok", true)
			assertStringContains(t, vm, "glob_first", "dialect_syntax_test.go")
			assertGet(t, vm, "joined", "a/b")
		})
	}
}

func TestPureDialectSyntaxExecutesThroughStdlib(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibAll),
				leia.WithProcessExecution(false),
				leia.WithProcessShell(false),
			}, tc.opts...)...)
			src := "name := \"leia\"\n" +
				"decoded := json`{\"x\": 7}`\n" +
				"encoded := json {x: 8, label: \"ok\"}\n" +
				"cleaned_path := path`a//b/../c`\n" +
				"parsed_url := url`https://example.com:8443/a/b?q=one&tag=two#frag`\n" +
				"base64_text := base64`Hello ${name}`\n" +
				"base64_decoded := dialect.eval(\"base64\", base64_text, {mode: \"decode\"})\n" +
				"base64_url := dialect.eval(\"base64\", \"a/b?\", {mode: \"url_encode\"})\n" +
				"base64_url_decoded := dialect.eval(\"base64\", base64_url, {mode: \"url_decode\"})\n" +
				"hash_sha256 := hash`leia`\n" +
				"hash_sha1 := dialect.eval(\"hash\", \"leia\", {algo: \"sha1\"})\n" +
				"msg := prompt`Hello ${name}`\n" +
				"prompt_cfg := prompt { role: \"system\", text: \"Hi\" }\n" +
				"quoted_cfg := quote { body: \"x := 1; x += 2\" }\n" +
				"quoted_raw := quote { x := 1; x += 2 }\n\n" +
				"decoded_x := decoded.x\n" +
				"encoded_text := encoded\n" +
				"url_host := parsed_url.host\n" +
				"url_port := parsed_url.port\n" +
				"url_query_q := parsed_url.query.q\n" +
				"url_query_tag := parsed_url.query.tag\n" +
				"prompt_text := msg.text\n" +
				"prompt_cfg_role := prompt_cfg.body.role\n" +
				"prompt_cfg_text := prompt_cfg.body.text\n" +
				"quote_cfg_kind := quoted_cfg.kind\n" +
				"quote_cfg_body := quoted_cfg.body.body\n" +
				"quote_raw_kind := quoted_raw.kind\n"
			err := vm.Exec(src)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "decoded_x", int64(7))
			assertGet(t, vm, "encoded_text", `{"label":"ok","x":8}`)
			assertGet(t, vm, "cleaned_path", "a/c")
			assertGet(t, vm, "url_host", "example.com")
			assertGet(t, vm, "url_port", "8443")
			assertGet(t, vm, "url_query_q", "one")
			assertGet(t, vm, "url_query_tag", "two")
			assertGet(t, vm, "base64_text", "SGVsbG8gbGVpYQ==")
			assertGet(t, vm, "base64_decoded", "Hello leia")
			assertGet(t, vm, "base64_url", "YS9iPw")
			assertGet(t, vm, "base64_url_decoded", "a/b?")
			assertGet(t, vm, "hash_sha256", "b0dea5555379c9e3384dd1e771de9d73db4ee7f9c24725bfe8b757b3768b015f")
			assertGet(t, vm, "hash_sha1", "3ea1ebad1aa28de8fc67188a456b9747bbcca81a")
			assertGet(t, vm, "prompt_text", "Hello leia")
			assertGet(t, vm, "prompt_cfg_role", "system")
			assertGet(t, vm, "prompt_cfg_text", "Hi")
			assertGet(t, vm, "quote_cfg_kind", "table")
			assertGet(t, vm, "quote_cfg_body", "x := 1; x += 2")
			assertGet(t, vm, "quote_raw_kind", "function")
		})
	}
}

func TestProcessDialectsRespectHostCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, blocked := range []struct {
				name string
				src  string
				want string
			}{
				{name: "shell", src: "out := $`printf blocked`", want: "process shell access disabled"},
				{name: "cmd", src: "out := cmd`printf blocked`", want: "process execution access disabled"},
			} {
				t.Run(blocked.name, func(t *testing.T) {
					vm := leia.New(append([]leia.Option{
						leia.WithLibs(leia.LibAll),
						leia.WithProcessExecution(false),
						leia.WithProcessShell(false),
					}, tc.opts...)...)
					err := vm.Exec(blocked.src)
					if err == nil || !strings.Contains(err.Error(), blocked.want) {
						t.Fatalf("Exec err = %v, want %q", err, blocked.want)
					}
				})
			}
		})
	}
}

func assertGet(t *testing.T, vm *leia.VM, name string, want any) {
	t.Helper()
	got, err := vm.Get(name)
	if err != nil {
		t.Fatalf("Get %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

func assertStringContains(t *testing.T, vm *leia.VM, name, want string) {
	t.Helper()
	got, err := vm.Get(name)
	if err != nil {
		t.Fatalf("Get %s: %v", name, err)
	}
	s, ok := got.(string)
	if !ok || !strings.Contains(s, want) {
		t.Fatalf("%s = %#v, want string containing %q", name, got, want)
	}
}
