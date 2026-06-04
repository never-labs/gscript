package tests_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestShellCommandFilesystemDialectSyntaxExecutesThroughStdlib(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "beta.txt"), []byte("beta"), 0600); err != nil {
		t.Fatal(err)
	}
	globPattern := filepath.ToSlash(filepath.Join(root, "nested", "*.txt"))

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
			src := "import p \"path\"\n\n" +
				"name := \"leia\"\n" +
				"shell_dollar := $`printf dollar-${name}`\n" +
				"shell_sh := sh`printf sh-${name}`\n" +
				"shell_fail := $`printf shellerr 1>&2; exit 7`\n" +
				"shell_fail_explicit := dialect.eval(\"sh\", \"printf expliciterr 1>&2; exit 6\", {fail_fast: false})\n" +
				"cmd_out := cmd`printf command-${name}`\n" +
				"cmd_fail := cmd`/bin/sh -c false`\n" +
				"matches := glob`" + globPattern + "`\n" +
				"cleaned := path`./nested/../alpha.txt`\n" +
				"digits := re!`^[0-9]+$`\n" +
				"identifier := regexp!`^[A-Za-z_][A-Za-z0-9_]*$`\n" +
				"shell_dollar_text := shell_dollar.text\n" +
				"shell_dollar_ok := shell_dollar.ok\n" +
				"shell_sh_text := shell_sh.text\n" +
				"shell_sh_code := shell_sh.code\n" +
				"shell_fail_ok := shell_fail.ok\n" +
				"shell_fail_code := shell_fail.code\n" +
				"shell_fail_stderr := shell_fail.stderr\n" +
				"shell_fail_explicit_ok := shell_fail_explicit.ok\n" +
				"shell_fail_explicit_code := shell_fail_explicit.code\n" +
				"shell_fail_explicit_stderr := shell_fail_explicit.stderr\n" +
				"cmd_text := cmd_out.text\n" +
				"cmd_ok := cmd_out.ok\n" +
				"cmd_fail_ok := cmd_fail.ok\n" +
				"cmd_fail_code := cmd_fail.code\n" +
				"glob_count := #matches\n" +
				"glob_first := matches[1]\n" +
				"joined := p.join(\"nested\", \"beta.txt\")\n" +
				"path_match_ok := p.match(\"nested/*.txt\", joined)\n" +
				"match_ok := digits.match(\"123\")\n" +
				"identifier_ok := identifier.match(\"name_1\")\n"
			err := vm.Exec(src)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "shell_dollar_text", "dollar-leia")
			assertGet(t, vm, "shell_dollar_ok", true)
			assertGet(t, vm, "shell_sh_text", "sh-leia")
			assertGet(t, vm, "shell_sh_code", int64(0))
			assertGet(t, vm, "shell_fail_ok", false)
			assertGet(t, vm, "shell_fail_code", int64(7))
			assertGet(t, vm, "shell_fail_stderr", "shellerr")
			assertGet(t, vm, "shell_fail_explicit_ok", false)
			assertGet(t, vm, "shell_fail_explicit_code", int64(6))
			assertGet(t, vm, "shell_fail_explicit_stderr", "expliciterr")
			assertGet(t, vm, "cmd_text", "command-leia")
			assertGet(t, vm, "cmd_ok", true)
			assertGet(t, vm, "cmd_fail_ok", false)
			assertGet(t, vm, "cmd_fail_code", int64(1))
			assertGet(t, vm, "match_ok", true)
			assertGet(t, vm, "identifier_ok", true)
			assertGet(t, vm, "glob_count", int64(1))
			assertStringContains(t, vm, "glob_first", "beta.txt")
			assertGet(t, vm, "cleaned", "alpha.txt")
			assertGet(t, vm, "joined", "nested/beta.txt")
			assertGet(t, vm, "path_match_ok", true)
		})
	}
}

func TestFilesystemDialectGlobRespectsRootAndPathStaysPure(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "inside.txt"), []byte("inside"), 0600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibSafe),
				leia.WithFilesystemRoot(root),
				leia.WithFilesystemWrite(false),
			}, tc.opts...)...)
			src := "matches := glob`nested/*.txt`\n" +
				"outside, outside_err := dialect.eval(\"glob\", \"../*.txt\")\n" +
				"cleaned := path`nested/../nested/./inside.txt`\n" +
				"glob_count := #matches\n" +
				"glob_first := matches[1]\n" +
				"outside_is_nil := outside == nil\n" +
				"path_is_pure := cleaned == \"nested/inside.txt\"\n"
			if err := vm.Exec(src); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "glob_count", int64(1))
			assertStringContains(t, vm, "glob_first", "inside.txt")
			assertGet(t, vm, "outside_is_nil", true)
			assertStringContains(t, vm, "outside_err", "escapes root")
			assertGet(t, vm, "path_is_pure", true)
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
				"tsv_rows := tsv`name\tscore\nAda\t42\nBob\t7\n`\n" +
				"tsv_header_rows := dialect.eval(\"tsv\", \"name\\tscore\\nAda\\t42\\n\", {headers: true})\n" +
				"line_rows := lines`alpha\n\nbeta\n`\n" +
				"split_rows := split`left\nright\n`\n" +
				"line_rows_keep_empty := dialect.eval(\"lines\", \"alpha\\n\\nbeta\\n\", {keep_empty: true})\n" +
				"words_rows := words`alpha beta gamma`\n" +
				"num_rows := nums`1, 2; 3.5\n4e1`\n" +
				"number_rows := dialect.eval(\"numbers\", \"-2 0 2.25\")\n" +
				"num_matrix := dialect.eval(\"nums\", \"1,2,3\\n4,5,6\\n\", {matrix: true})\n" +
				"bad_nums, bad_nums_err := dialect.eval(\"nums\", \"1 nope 3\")\n" +
				"bad_num_matrix, bad_num_matrix_err := dialect.eval(\"nums\", \"1,2\\n3,4,5\\n\", {matrix: true})\n" +
				"kv_rows := dialect.eval(\"kv\", \"name = Ada\\nscore = 42\\n\")\n" +
				"env_rows := dialect.eval(\"env\", \"TOKEN=\\\"abc 123\\\"\\nEMPTY=\\n\")\n" +
				"escaped_html := html_escape`<b>Ada & Bob</b>`\n" +
				"unescaped_html := dialect.eval(\"html_escape\", escaped_html, {mode: \"unescape\"})\n" +
				"urlquery_component := dialect.eval(\"urlquery\", \"hello world&x\", {mode: \"escape\"})\n" +
				"urlquery_component_decoded := dialect.eval(\"urlquery\", urlquery_component, {mode: \"unescape\"})\n" +
				"urlquery_text := urlquery {q: \"hello world\", page: 2}\n" +
				"urlquery_rows := urlquery`q=hello+world&page=2&tag=a&tag=b`\n" +
				"mime_type := mime`text/html; charset=utf-8; boundary=\"abc def\"`\n" +
				"mime_encoded := mime {type: \"application/json\", params: {charset: \"utf-8\", version: 2}}\n" +
				"template_text := template`Hello static`\n" +
				"template_cfg := template { text: \"Score {{.score}}\", data: {score: \"42\"} }\n" +
				"template_eval := dialect.eval(\"template\", \"Hi {{.name}}\", {data: {name: name}})\n" +
				"bad_json, bad_json_err := dialect.eval(\"json\", \"{} []\")\n" +
				"jsonl_rows := jsonl`{\"name\":\"Ada\",\"score\":42}\n{\"name\":\"Bob\",\"score\":7}\n`\n" +
				"bad_jsonl, bad_jsonl_err := dialect.eval(\"jsonl\", \"{\\\"ok\\\":true}\\n\\n{\\\"ok\\\":false}\\n\")\n" +
				"jsonl_text := dialect.eval(\"jsonl\", jsonl_rows, {mode: \"encode\"})\n\n" +
				"csv_row_2_name := csv_rows[2][1]\n" +
				"csv_header_name := csv_header_rows[1].name\n" +
				"csv_header_score := csv_header_rows[1].score\n" +
				"tsv_row_2_name := tsv_rows[2][1]\n" +
				"tsv_header_name := tsv_header_rows[1].name\n" +
				"tsv_header_score := tsv_header_rows[1].score\n" +
				"line_count := #line_rows\n" +
				"line_second := line_rows[2]\n" +
				"split_first := split_rows[1]\n" +
				"split_second := split_rows[2]\n" +
				"line_keep_empty_count := #line_rows_keep_empty\n" +
				"line_keep_empty_second := line_rows_keep_empty[2]\n" +
				"words_second := words_rows[2]\n" +
				"num_count := #num_rows\n" +
				"num_first := num_rows[1]\n" +
				"num_third := num_rows[3]\n" +
				"num_fourth := num_rows[4]\n" +
				"number_first := number_rows[1]\n" +
				"number_third := number_rows[3]\n" +
				"num_matrix_rows := #num_matrix\n" +
				"num_matrix_cols := #num_matrix[1]\n" +
				"num_matrix_last := num_matrix[2][3]\n" +
				"bad_nums_is_nil := bad_nums == nil\n" +
				"bad_num_matrix_is_nil := bad_num_matrix == nil\n" +
				"kv_name := kv_rows.name\n" +
				"kv_score := kv_rows.score\n" +
				"env_token := env_rows.TOKEN\n" +
				"env_empty := env_rows.EMPTY\n" +
				"urlquery_q := urlquery_rows.q\n" +
				"urlquery_page := urlquery_rows.page\n" +
				"urlquery_tag_2 := urlquery_rows.tag[2]\n"
			src += "mime_type_value := mime_type.type\n" +
				"mime_charset := mime_type.params.charset\n" +
				"mime_boundary := mime_type.params.boundary\n" +
				"bad_json_is_nil := bad_json == nil\n" +
				"bad_jsonl_is_nil := bad_jsonl == nil\n" +
				"jsonl_first_name := jsonl_rows[1].name\n" +
				"jsonl_second_score := jsonl_rows[2].score\n"
			err := vm.Exec(src)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "csv_row_2_name", "Ada")
			assertGet(t, vm, "csv_header_name", "Ada")
			assertGet(t, vm, "csv_header_score", "42")
			assertGet(t, vm, "tsv_row_2_name", "Ada")
			assertGet(t, vm, "tsv_header_name", "Ada")
			assertGet(t, vm, "tsv_header_score", "42")
			assertGet(t, vm, "line_count", int64(2))
			assertGet(t, vm, "line_second", "beta")
			assertGet(t, vm, "split_first", "left")
			assertGet(t, vm, "split_second", "right")
			assertGet(t, vm, "line_keep_empty_count", int64(3))
			assertGet(t, vm, "line_keep_empty_second", "")
			assertGet(t, vm, "words_second", "beta")
			assertGet(t, vm, "num_count", int64(4))
			assertGet(t, vm, "num_first", int64(1))
			assertGet(t, vm, "num_third", 3.5)
			assertGet(t, vm, "num_fourth", 40.0)
			assertGet(t, vm, "number_first", int64(-2))
			assertGet(t, vm, "number_third", 2.25)
			assertGet(t, vm, "num_matrix_rows", int64(2))
			assertGet(t, vm, "num_matrix_cols", int64(3))
			assertGet(t, vm, "num_matrix_last", int64(6))
			assertGet(t, vm, "bad_nums_is_nil", true)
			assertStringContains(t, vm, "bad_nums_err", `invalid number "nope"`)
			assertGet(t, vm, "bad_num_matrix_is_nil", true)
			assertStringContains(t, vm, "bad_num_matrix_err", "matrix row 2 has 3 values, want 2")
			assertGet(t, vm, "kv_name", "Ada")
			assertGet(t, vm, "kv_score", "42")
			assertGet(t, vm, "env_token", "abc 123")
			assertGet(t, vm, "env_empty", "")
			assertGet(t, vm, "escaped_html", "&lt;b&gt;Ada &amp; Bob&lt;/b&gt;")
			assertGet(t, vm, "unescaped_html", "<b>Ada & Bob</b>")
			assertGet(t, vm, "urlquery_component", "hello+world%26x")
			assertGet(t, vm, "urlquery_component_decoded", "hello world&x")
			assertGet(t, vm, "urlquery_text", "page=2&q=hello+world")
			assertGet(t, vm, "urlquery_q", "hello world")
			assertGet(t, vm, "urlquery_page", "2")
			assertGet(t, vm, "urlquery_tag_2", "b")
			assertGet(t, vm, "mime_type_value", "text/html")
			assertGet(t, vm, "mime_charset", "utf-8")
			assertGet(t, vm, "mime_boundary", "abc def")
			assertGet(t, vm, "mime_encoded", "application/json; charset=utf-8; version=2")
			assertGet(t, vm, "template_text", "Hello static")
			assertGet(t, vm, "template_cfg", "Score 42")
			assertGet(t, vm, "template_eval", "Hi Leia")
			assertGet(t, vm, "bad_json_is_nil", true)
			assertStringContains(t, vm, "bad_json_err", "invalid JSON: trailing data")
			assertGet(t, vm, "bad_jsonl_is_nil", true)
			assertStringContains(t, vm, "bad_jsonl_err", "line 2: empty JSONL record")
			assertGet(t, vm, "jsonl_first_name", "Ada")
			assertGet(t, vm, "jsonl_second_score", int64(7))
			assertGet(t, vm, "jsonl_text", "{\"name\":\"Ada\",\"score\":42}\n{\"name\":\"Bob\",\"score\":7}\n")
		})
	}
}

func TestProcessDialectsRespectHostCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, blocked := range []struct {
				name string
				src  string
				want string
				opts []leia.Option
			}{
				{name: "shell", src: "out := $`printf blocked`", want: "process shell access disabled"},
				{name: "cmd", src: "out := cmd`printf blocked`", want: "process execution access disabled"},
				{name: "glob", src: "out := glob`*.leia`", want: "filesystem read access disabled", opts: []leia.Option{leia.WithFilesystemRead(false)}},
			} {
				t.Run(blocked.name, func(t *testing.T) {
					opts := append([]leia.Option{
						leia.WithLibs(leia.LibAll),
						leia.WithProcessExecution(false),
						leia.WithProcessShell(false),
					}, tc.opts...)
					opts = append(opts, blocked.opts...)
					vm := leia.New(opts...)
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
