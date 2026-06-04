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

func TestStdlibDataDialectsExecuteThroughStdlib(t *testing.T) {
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
			src := "name := \"Leia\"\n" +
				"csv_rows := csv`name,score\nAda,42\nBob,7\n`\n" +
				"csv_header_rows := dialect.eval(\"csv\", \"name;score\\nAda;42\\n\", {headers: true, sep: \";\"})\n" +
				"line_rows := lines`alpha\n\nbeta\n`\n" +
				"line_rows_keep_empty := dialect.eval(\"lines\", \"alpha\\n\\nbeta\\n\", {keep_empty: true})\n" +
				"words_rows := words`alpha beta gamma`\n" +
				"kv_rows := dialect.eval(\"kv\", \"name = Ada\\nscore = 42\\n\")\n" +
				"env_rows := dialect.eval(\"env\", \"TOKEN=\\\"abc 123\\\"\\nEMPTY=\\n\")\n" +
				"escaped_html := html_escape`<b>Ada & Bob</b>`\n" +
				"unescaped_html := dialect.eval(\"html_escape\", escaped_html, {mode: \"unescape\"})\n" +
				"urlquery_text := urlquery {q: \"hello world\", page: 2}\n" +
				"urlquery_rows := urlquery`q=hello+world&page=2&tag=a&tag=b`\n" +
				"template_text := template`Hello static`\n" +
				"template_cfg := template { text: \"Score {{.score}}\", data: {score: \"42\"} }\n" +
				"template_eval := dialect.eval(\"template\", \"Hi {{.name}}\", {data: {name: name}})\n\n" +
				"csv_row_2_name := csv_rows[2][1]\n" +
				"csv_header_name := csv_header_rows[1].name\n" +
				"csv_header_score := csv_header_rows[1].score\n" +
				"line_count := #line_rows\n" +
				"line_second := line_rows[2]\n" +
				"line_keep_empty_count := #line_rows_keep_empty\n" +
				"line_keep_empty_second := line_rows_keep_empty[2]\n" +
				"words_second := words_rows[2]\n" +
				"kv_name := kv_rows.name\n" +
				"kv_score := kv_rows.score\n" +
				"env_token := env_rows.TOKEN\n" +
				"env_empty := env_rows.EMPTY\n" +
				"urlquery_q := urlquery_rows.q\n" +
				"urlquery_page := urlquery_rows.page\n" +
				"urlquery_tag_2 := urlquery_rows.tag[2]\n"
			err := vm.Exec(src)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "csv_row_2_name", "Ada")
			assertGet(t, vm, "csv_header_name", "Ada")
			assertGet(t, vm, "csv_header_score", "42")
			assertGet(t, vm, "line_count", int64(2))
			assertGet(t, vm, "line_second", "beta")
			assertGet(t, vm, "line_keep_empty_count", int64(3))
			assertGet(t, vm, "line_keep_empty_second", "")
			assertGet(t, vm, "words_second", "beta")
			assertGet(t, vm, "kv_name", "Ada")
			assertGet(t, vm, "kv_score", "42")
			assertGet(t, vm, "env_token", "abc 123")
			assertGet(t, vm, "env_empty", "")
			assertGet(t, vm, "escaped_html", "&lt;b&gt;Ada &amp; Bob&lt;/b&gt;")
			assertGet(t, vm, "unescaped_html", "<b>Ada & Bob</b>")
			assertGet(t, vm, "urlquery_text", "page=2&q=hello+world")
			assertGet(t, vm, "urlquery_q", "hello world")
			assertGet(t, vm, "urlquery_page", "2")
			assertGet(t, vm, "urlquery_tag_2", "b")
			assertGet(t, vm, "template_text", "Hello static")
			assertGet(t, vm, "template_cfg", "Score 42")
			assertGet(t, vm, "template_eval", "Hi Leia")
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
