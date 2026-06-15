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
				"cmd_quoted := cmd`printf 'quoted command'`\n" +
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
				"cmd_quoted_text := cmd_quoted.text\n" +
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
			assertGet(t, vm, "cmd_quoted_text", "quoted command")
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

func TestTaggedDialectStringsRejectQuotedStringLiterals(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, src := range []string{
				`msg := prompt"hello"`,
				`msg := prompt!"hello"`,
				`shell := $"printf hello"`,
				`shell := $!"printf hello"`,
			} {
				t.Run(src, func(t *testing.T) {
					vm := leia.New(append([]leia.Option{
						leia.WithLibs(leia.LibAll),
						leia.WithProcessExecution(false),
						leia.WithProcessShell(false),
					}, tc.opts...)...)
					err := vm.Exec(src)
					if err == nil {
						t.Fatalf("Exec(%q) succeeded, want parse error", src)
					}
				})
			}
		})
	}
}

func TestUserRegisteredDialectSyntaxExecutesThroughStdlib(t *testing.T) {
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
			src := `
				dialect.register("wrap", func(body, opts) {
					prefix := "<"
					suffix := ">"
					if opts != nil && opts.prefix != nil { prefix = opts.prefix }
					if opts != nil && opts.suffix != nil { suffix = opts.suffix }
					return prefix .. body .. suffix
				}, {aliases: {"bracket"}})
				dialect.register({
					name: "record",
					eval: func(body, opts) {
						return {kind: "eval", body: body}
					},
					block: func(body, opts) {
						return {kind: "block", name: body.name, count: body.count}
					},
				})

					name := "leia"
					literal := wrap` + "`" + `hello-${name}` + "`" + `
					fenced_literal := wrap` + "```" + `hello-${name}
raw` + "```" + `
					alias_literal := bracket` + "`" + `ok` + "`" + `
					explicit := dialect.eval("wrap", "plain", {prefix: "[", suffix: "]"})
				block := record {
					name: "jobs"
					count: 3
				}
				eval_record := dialect.eval("record", "plain")
				block_kind := block.kind
				block_name := block.name
				block_count := block.count
				eval_kind := eval_record.kind
				eval_body := eval_record.body
			`
			if err := vm.Exec(src); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "literal", "<hello-leia>")
			assertGet(t, vm, "fenced_literal", "<hello-leia\nraw>")
			assertGet(t, vm, "alias_literal", "<ok>")
			assertGet(t, vm, "explicit", "[plain]")
			assertGet(t, vm, "block_kind", "block")
			assertGet(t, vm, "block_name", "jobs")
			assertGet(t, vm, "block_count", int64(3))
			assertGet(t, vm, "eval_kind", "eval")
			assertGet(t, vm, "eval_body", "plain")
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
				"parsed_ip := ipaddr`10.2.3.4`\n" +
				"parsed_net := cidr`10.2.0.0/16`\n" +
				"parsed_hostport := hostport`127.0.0.1:8080`\n" +
				"parsed_log := logfmt`level=info msg=\"hello leia\" ok`\n" +
				"base64_text := base64`Hello ${name}`\n" +
				"base64_decoded := dialect.eval(\"base64\", base64_text, {mode: \"decode\"})\n" +
				"raw_line_rows := lines`one\\nnot split`\n" +
				"base64_url := dialect.eval(\"base64\", \"a/b?\", {mode: \"url_encode\"})\n" +
				"base64_url_decoded := dialect.eval(\"base64\", base64_url, {mode: \"url_decode\"})\n" +
				"hex_text := hex`go`\n" +
				"hex_decoded := dialect.eval(\"hex\", hex_text, {mode: \"decode\"})\n" +
				"base32_text := base32`go`\n" +
				"base32_decoded := dialect.eval(\"base32\", base32_text, {mode: \"decode\"})\n" +
				"uuid_parsed := uuid`550e8400-e29b-41d4-a716-446655440000`\n" +
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
				"ip_private := parsed_ip.private\n" +
				"net_bits := parsed_net.bits\n" +
				"hp_port := parsed_hostport.port\n" +
				"log_msg := parsed_log.msg\n" +
				"log_ok := parsed_log.ok\n" +
				"uuid_version := uuid_parsed.version\n" +
				"uuid_variant := uuid_parsed.variant\n" +
				"prompt_text := msg.text\n" +
				"raw_line_count := #raw_line_rows\n" +
				"raw_line_first := raw_line_rows[1]\n" +
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
			assertGet(t, vm, "ip_private", true)
			assertGet(t, vm, "net_bits", int64(16))
			assertGet(t, vm, "hp_port", "8080")
			assertGet(t, vm, "log_msg", "hello leia")
			assertGet(t, vm, "log_ok", "true")
			assertGet(t, vm, "base64_text", "SGVsbG8gbGVpYQ==")
			assertGet(t, vm, "base64_decoded", "Hello leia")
			assertGet(t, vm, "raw_line_count", int64(1))
			assertGet(t, vm, "raw_line_first", `one\nnot split`)
			assertGet(t, vm, "base64_url", "YS9iPw")
			assertGet(t, vm, "base64_url_decoded", "a/b?")
			assertGet(t, vm, "hex_text", "676f")
			assertGet(t, vm, "hex_decoded", "go")
			assertGet(t, vm, "base32_text", "M5XQ====")
			assertGet(t, vm, "base32_decoded", "go")
			assertGet(t, vm, "uuid_version", int64(4))
			assertGet(t, vm, "uuid_variant", "RFC4122")
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
				"csv_text := dialect.eval(\"csv\", {{\"name\", \"score\"}, {\"Ada\", 42}, {\"Bob\", 7}}, {mode: \"encode\"})\n" +
				"csv_header_text := dialect.eval(\"csv\", {{name: \"Ada\", score: 42}, {name: \"Bob\", score: 7}}, {mode: \"encode\", headers: {\"name\", \"score\"}})\n" +
				"tsv_rows := tsv`name\tscore\nAda\t42\nBob\t7\n`\n" +
				"tsv_header_rows := dialect.eval(\"tsv\", \"name\\tscore\\nAda\\t42\\n\", {headers: true})\n" +
				"tsv_text := dialect.eval(\"tsv\", {{\"name\", \"score\"}, {\"Ada\", 42}}, {mode: \"encode\"})\n" +
				"mdtable_rows := mdtable`| Name | Score | Note |\n| --- | ---: | --- |\n| Ada | 42 | uses \\| safely |\n| Bob | 7 |\n`\n" +
				"mdtable_text := dialect.eval(\"mdtable\", mdtable_rows, {mode: \"encode\"})\n" +
				"line_rows := lines`alpha\n\nbeta\n`\n" +
				"split_rows := split`left\nright\n`\n" +
				"line_rows_keep_empty := dialect.eval(\"lines\", \"alpha\\n\\nbeta\\n\", {keep_empty: true})\n" +
				"words_rows := words`alpha beta gamma`\n" +
				"shellwords_rows := shellwords`printf 'hello world' a\\ b \"\"`\n" +
				"shellwords_to_encode := {}\n" +
				"shellwords_to_encode[1] = \"printf\"\n" +
				"shellwords_to_encode[2] = \"%s\\n\"\n" +
				"shellwords_to_encode[3] = \"hello world\"\n" +
				"shellwords_to_encode[4] = \"it's\"\n" +
				"shellwords_to_encode[5] = \"\"\n" +
				"shellwords_text := dialect.eval(\"shellwords\", shellwords_to_encode, {mode: \"encode\"})\n" +
				"shellwords_roundtrip := dialect.eval(\"shellwords\", shellwords_text)\n" +
				"num_rows := nums`1, 2; 3.5\n4e1`\n" +
				"number_rows := dialect.eval(\"numbers\", \"-2 0 2.25\")\n" +
				"num_matrix := dialect.eval(\"nums\", \"1,2,3\\n4,5,6\\n\", {matrix: true})\n" +
				"bad_nums, bad_nums_err := dialect.eval(\"nums\", \"1 nope 3\")\n" +
				"bad_num_matrix, bad_num_matrix_err := dialect.eval(\"nums\", \"1,2\\n3,4,5\\n\", {matrix: true})\n" +
				"kv_rows := dialect.eval(\"kv\", \"name = Ada\\nscore = 42\\n\")\n" +
				"env_rows := dialect.eval(\"env\", \"TOKEN=\\\"abc 123\\\"\\nEMPTY=\\n\")\n" +
				"ini_cfg := ini`app = ledger\n[database]\nhost = db.internal\nport = 5432\n`\n" +
				"ini_database := {host: \"db.internal\", port: 5432}\n" +
				"ini_text := dialect.eval(\"ini\", {app: \"ledger\", enabled: true, database: ini_database}, {mode: \"encode\"})\n" +
				"ini_roundtrip := dialect.eval(\"ini\", ini_text)\n" +
				"duration_parsed := duration`1h30m250ms`\n" +
				"duration_seconds_encoded := dialect.eval(\"duration\", 90.25, {mode: \"encode\"})\n" +
				"duration_millis_encoded := dialect.eval(\"duration\", {milliseconds: 250}, {mode: \"encode\"})\n" +
				"duration_roundtrip := dialect.eval(\"duration\", duration_parsed, {mode: \"encode\"})\n" +
				"tap_rows := tap`TAP version 13\n1..2\nok 1 - boot\nnot ok 2 - deploy # TODO flaky\n# expected ready\n`\n" +
				"tap_text := dialect.eval(\"tap\", tap_rows, {mode: \"encode\"})\n" +
				"escaped_html := html_escape`<b>Ada & Bob</b>`\n" +
				"unescaped_html := dialect.eval(\"html_escape\", escaped_html, {mode: \"unescape\"})\n" +
				"urlquery_component := dialect.eval(\"urlquery\", \"hello world&x\", {mode: \"escape\"})\n" +
				"urlquery_component_decoded := dialect.eval(\"urlquery\", urlquery_component, {mode: \"unescape\"})\n" +
				"urlpath_text := urlpath`a b/米`\n" +
				"urlpath_decoded := dialect.eval(\"urlpath\", urlpath_text, {mode: \"unescape\"})\n" +
				"urlquery_text := urlquery {q: \"hello world\", page: 2}\n" +
				"urlquery_rows := urlquery`q=hello+world&page=2&tag=a&tag=b`\n" +
				"mime_type := mime`text/html; charset=utf-8; boundary=\"abc def\"`\n" +
				"mime_encoded := mime {type: \"application/json\", params: {charset: \"utf-8\", version: 2}}\n" +
				"header_rows := dialect.eval(\"headers\", \"content-type: text/plain\\r\\nset-cookie: a=1\\r\\nset-cookie: b=2\\r\\n\")\n" +
				"header_to_encode := {}\n" +
				"header_to_encode[\"x-trace\"] = \"abc\"\n" +
				"header_encoded := dialect.eval(\"http_headers\", header_to_encode, {mode: \"encode\"})\n" +
				"http_request := httpmsg`POST /submit HTTP/1.1\nHost: example.test\nContent-Type: text/plain\n\nhello`\n" +
				"http_response := dialect.eval(\"httpmsg\", \"HTTP/1.1 201 Created\\r\\ncontent-length: 2\\r\\n\\r\\nok\")\n" +
				"http_headers := {}\n" +
				"http_headers.host = \"example.test\"\n" +
				"http_encoded := dialect.eval(\"httpmsg\", {method: \"GET\", target: \"/health\", headers: http_headers}, {mode: \"encode\"})\n" +
				"cookie_rows := cookie`session=abc123; tag=a; tag=b`\n" +
				"cookie_encoded := dialect.eval(\"cookies\", {session: \"abc123\", tag: {\"a\", \"b\"}}, {mode: \"encode\"})\n" +
				"template_text := template`Hello static`\n" +
				"template_cfg := template { text: \"Score {{.score}}\", data: {score: \"42\"} }\n" +
				"template_eval := dialect.eval(\"template\", \"Hi {{.name}}\", {data: {name: name}})\n" +
				"xml_escaped := xml`<node attr=\"a&b\">Tom & 'Jerry'</node>`\n" +
				"xml_unescaped, xml_unescape_err := dialect.eval(\"xml\", xml_escaped, {mode: \"unescape\"})\n" +
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
				"mdtable_first_name := mdtable_rows[1].Name\n" +
				"mdtable_first_note := mdtable_rows[1].Note\n" +
				"mdtable_second_note := mdtable_rows[2].Note\n" +
				"line_count := #line_rows\n" +
				"line_second := line_rows[2]\n" +
				"split_first := split_rows[1]\n" +
				"split_second := split_rows[2]\n" +
				"line_keep_empty_count := #line_rows_keep_empty\n" +
				"line_keep_empty_second := line_rows_keep_empty[2]\n" +
				"words_second := words_rows[2]\n" +
				"shellwords_count := #shellwords_rows\n" +
				"shellwords_second := shellwords_rows[2]\n" +
				"shellwords_third := shellwords_rows[3]\n" +
				"shellwords_empty := shellwords_rows[4]\n" +
				"shellwords_roundtrip_fourth := shellwords_roundtrip[4]\n" +
				"shellwords_roundtrip_fifth := shellwords_roundtrip[5]\n" +
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
				"ini_app := ini_cfg.app\n" +
				"ini_host := ini_cfg.database.host\n" +
				"ini_port := ini_cfg.database.port\n" +
				"ini_roundtrip_port := ini_roundtrip.database.port\n" +
				"duration_text := duration_parsed.text\n" +
				"duration_seconds := duration_parsed.seconds\n" +
				"duration_milliseconds := duration_parsed.milliseconds\n" +
				"duration_nanoseconds := duration_parsed.nanoseconds\n" +
				"tap_plan_last := tap_rows[2].last\n" +
				"tap_second_ok := tap_rows[4].ok\n" +
				"tap_second_directive := tap_rows[4].directive\n" +
				"tap_second_diagnostic := tap_rows[4].diagnostics[1]\n" +
				"urlquery_q := urlquery_rows.q\n" +
				"urlquery_page := urlquery_rows.page\n" +
				"urlquery_tag_2 := urlquery_rows.tag[2]\n"
			src += "mime_type_value := mime_type.type\n" +
				"mime_charset := mime_type.params.charset\n" +
				"mime_boundary := mime_type.params.boundary\n" +
				"header_content_type := header_rows[\"Content-Type\"]\n" +
				"header_second_cookie := header_rows[\"Set-Cookie\"][2]\n" +
				"http_request_method := http_request.method\n" +
				"http_request_body := http_request.body\n" +
				"http_response_status := http_response.status\n" +
				"http_response_body := http_response.body\n" +
				"cookie_session := cookie_rows.session\n" +
				"cookie_second_tag := cookie_rows.tag[2]\n" +
				"xml_unescape_ok := xml_unescape_err == nil\n" +
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
			assertGet(t, vm, "csv_text", "name,score\nAda,42\nBob,7\n")
			assertGet(t, vm, "csv_header_text", "name,score\nAda,42\nBob,7\n")
			assertGet(t, vm, "tsv_row_2_name", "Ada")
			assertGet(t, vm, "tsv_header_name", "Ada")
			assertGet(t, vm, "tsv_header_score", "42")
			assertGet(t, vm, "tsv_text", "name\tscore\nAda\t42\n")
			assertGet(t, vm, "mdtable_first_name", "Ada")
			assertGet(t, vm, "mdtable_first_note", "uses | safely")
			assertGet(t, vm, "mdtable_second_note", "")
			assertGet(t, vm, "mdtable_text", "| Name | Score | Note |\n| --- | --- | --- |\n| Ada | 42 | uses \\| safely |\n| Bob | 7 |  |\n")
			assertGet(t, vm, "line_count", int64(2))
			assertGet(t, vm, "line_second", "beta")
			assertGet(t, vm, "split_first", "left")
			assertGet(t, vm, "split_second", "right")
			assertGet(t, vm, "line_keep_empty_count", int64(3))
			assertGet(t, vm, "line_keep_empty_second", "")
			assertGet(t, vm, "words_second", "beta")
			assertGet(t, vm, "shellwords_count", int64(4))
			assertGet(t, vm, "shellwords_second", "hello world")
			assertGet(t, vm, "shellwords_third", "a b")
			assertGet(t, vm, "shellwords_empty", "")
			assertGet(t, vm, "shellwords_roundtrip_fourth", "it's")
			assertGet(t, vm, "shellwords_roundtrip_fifth", "")
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
			assertGet(t, vm, "ini_app", "ledger")
			assertGet(t, vm, "ini_host", "db.internal")
			assertGet(t, vm, "ini_port", "5432")
			assertGet(t, vm, "ini_text", "app=ledger\nenabled=true\n\n[database]\nhost=db.internal\nport=5432\n")
			assertGet(t, vm, "ini_roundtrip_port", "5432")
			assertGet(t, vm, "duration_text", "1h30m0.25s")
			assertGet(t, vm, "duration_seconds", 5400.25)
			assertGet(t, vm, "duration_milliseconds", 5400250.0)
			assertGet(t, vm, "duration_nanoseconds", int64(5_400_250_000_000))
			assertGet(t, vm, "duration_seconds_encoded", "1m30.25s")
			assertGet(t, vm, "duration_millis_encoded", "250ms")
			assertGet(t, vm, "duration_roundtrip", "1h30m0.25s")
			assertGet(t, vm, "tap_plan_last", int64(2))
			assertGet(t, vm, "tap_second_ok", false)
			assertGet(t, vm, "tap_second_directive", "TODO")
			assertGet(t, vm, "tap_second_diagnostic", "expected ready")
			assertGet(t, vm, "tap_text", "TAP version 13\n1..2\nok 1 - boot\nnot ok 2 - deploy # TODO flaky\n# expected ready\n")
			assertGet(t, vm, "escaped_html", "&lt;b&gt;Ada &amp; Bob&lt;/b&gt;")
			assertGet(t, vm, "unescaped_html", "<b>Ada & Bob</b>")
			assertGet(t, vm, "urlquery_component", "hello+world%26x")
			assertGet(t, vm, "urlquery_component_decoded", "hello world&x")
			assertGet(t, vm, "urlpath_text", "a%20b%2F%E7%B1%B3")
			assertGet(t, vm, "urlpath_decoded", "a b/米")
			assertGet(t, vm, "urlquery_text", "page=2&q=hello+world")
			assertGet(t, vm, "urlquery_q", "hello world")
			assertGet(t, vm, "urlquery_page", "2")
			assertGet(t, vm, "urlquery_tag_2", "b")
			assertGet(t, vm, "mime_type_value", "text/html")
			assertGet(t, vm, "mime_charset", "utf-8")
			assertGet(t, vm, "mime_boundary", "abc def")
			assertGet(t, vm, "mime_encoded", "application/json; charset=utf-8; version=2")
			assertGet(t, vm, "header_content_type", "text/plain")
			assertGet(t, vm, "header_second_cookie", "b=2")
			assertGet(t, vm, "header_encoded", "X-Trace: abc\r\n")
			assertGet(t, vm, "http_request_method", "POST")
			assertGet(t, vm, "http_request_body", "hello")
			assertGet(t, vm, "http_response_status", int64(201))
			assertGet(t, vm, "http_response_body", "ok")
			assertGet(t, vm, "http_encoded", "GET /health HTTP/1.1\r\nHost: example.test\r\n\r\n")
			assertGet(t, vm, "cookie_session", "abc123")
			assertGet(t, vm, "cookie_second_tag", "b")
			assertGet(t, vm, "cookie_encoded", "session=abc123; tag=a; tag=b")
			assertGet(t, vm, "xml_escaped", "&lt;node attr=&#34;a&amp;b&#34;&gt;Tom &amp; &#39;Jerry&#39;&lt;/node&gt;")
			assertGet(t, vm, "xml_unescape_ok", true)
			assertGet(t, vm, "xml_unescaped", `<node attr="a&b">Tom & 'Jerry'</node>`)
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

func TestQSymbolicDialectMilestone1ExecutesThroughStdlib(t *testing.T) {
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
			}, tc.opts...)...)
			src := "v := q`1 2 3`\n" +
				"plus := q`2+1 2 3`\n" +
				"over := q`+/1 2 3`\n" +
				"scan := q`+\\1 2 3`\n" +
				"dict := dialect.eval(\"q\", \"`a`b!10 20\")\n" +
				"syms := dialect.eval(\"q\", \"`AAPL`MSFT`NVDA\")\n" +
				"spread := dialect.eval(\"q\", \"100 101.5 103 - 99.5 100 101\")\n" +
				"idx := dialect.eval(\"q\", \"where 100 101.5 103>100\")\n" +
				"first_two := dialect.eval(\"q\", \"2#10 20 30\")\n" +
				"book := dialect.eval(\"q\", \"`bid`ask!(99.5 100;100.5 101)\")\n" +
				"trades := dialect.eval(\"q\", \"flip `sym`side`price`size!(`AAPL`MSFT`AAPL;`buy`sell`buy;100.5 200 101;10 15 20)\")\n" +
				"fenced_trades := q```flip `sym`price`size!(`AAPL`MSFT`AAPL;100.5 200 101;10 15 20)```\n" +
				"v1 := v[1]\n" +
				"v_sum := array.sum(v)\n" +
				"plus3 := plus[3]\n" +
				"scan3 := scan[3]\n" +
				"dict_a := dict.a\n" +
				"dict_b := dict.b\n" +
				"sym2 := syms[2]\n" +
				"spread3 := spread[3]\n" +
				"idx1 := idx[1]\n" +
				"first_two2 := first_two[2]\n" +
				"book_ask2 := book.ask[2]\n" +
				"trade2_sym := trades[2].sym\n" +
				"trade3_price := trades[3].price\n" +
				"fenced_sym2 := fenced_trades[2].sym\n" +
				"fenced_price3 := fenced_trades[3].price\n"
			if err := vm.Exec(src); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "v1", int64(1))
			assertGet(t, vm, "v_sum", int64(6))
			assertGet(t, vm, "plus3", int64(5))
			assertGet(t, vm, "over", int64(6))
			assertGet(t, vm, "scan3", int64(6))
			assertGet(t, vm, "dict_a", int64(10))
			assertGet(t, vm, "dict_b", int64(20))
			assertGet(t, vm, "sym2", "MSFT")
			assertGet(t, vm, "spread3", float64(2))
			assertGet(t, vm, "idx1", int64(1))
			assertGet(t, vm, "first_two2", int64(20))
			assertGet(t, vm, "book_ask2", float64(101))
			assertGet(t, vm, "trade2_sym", "MSFT")
			assertGet(t, vm, "trade3_price", float64(101))
			assertGet(t, vm, "fenced_sym2", "MSFT")
			assertGet(t, vm, "fenced_price3", float64(101))
		})
	}
}

func TestKVEnvDialectsEncodeThroughStdlib(t *testing.T) {
	t.Setenv("LEIA_DIALECT_SYNTAX_ENV_ALLOWED", "syntax-visible")
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
				leia.WithEnvironmentRead(true),
				leia.WithEnvironmentAllowlist("LEIA_DIALECT_SYNTAX_ENV_ALLOWED", "LEIA_DIALECT_SYNTAX_ENV_MISSING"),
			}, tc.opts...)...)
			err := vm.Exec(`
				kv_text := dialect.eval("kv", {score: 42, name: "Ada"}, {mode: "encode"})
				kv_roundtrip := dialect.eval("kv", kv_text)
				env_text := dialect.eval("env", {TOKEN: "abc 123", EMPTY: ""}, {mode: "encode"})
				env_roundtrip := dialect.eval("env", env_text)
				host_env := env` + "`" + `LEIA_DIALECT_SYNTAX_ENV_ALLOWED` + "`" + `
				host_env_missing := env` + "`" + `LEIA_DIALECT_SYNTAX_ENV_MISSING` + "`" + `
				host_env_missing_fast_ok, host_env_missing_fast_err := pcall(func() {
					return env!` + "`" + `LEIA_DIALECT_SYNTAX_ENV_MISSING` + "`" + `
				})

				kv_name := kv_roundtrip.name
				kv_score := kv_roundtrip.score
				env_token := env_roundtrip.TOKEN
				env_empty := env_roundtrip.EMPTY
			`)
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "kv_text", "name=Ada\nscore=42\n")
			assertGet(t, vm, "kv_name", "Ada")
			assertGet(t, vm, "kv_score", "42")
			assertGet(t, vm, "env_text", "EMPTY=\nTOKEN=abc 123\n")
			assertGet(t, vm, "env_token", "abc 123")
			assertGet(t, vm, "env_empty", "")
			assertGet(t, vm, "host_env", "syntax-visible")
			assertGet(t, vm, "host_env_missing", nil)
			assertGet(t, vm, "host_env_missing_fast_ok", false)
			assertStringContains(t, vm, "host_env_missing_fast_err", "environment variable not set")
		})
	}
}

func TestReportAndRouteDialectsExecuteThroughStdlib(t *testing.T) {
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
			src := `
				report := junit` + "`" + `<testsuites name="ci" tests="3" failures="1" errors="0" skipped="1" time="1.25">
  <testsuite name="unit" tests="2" failures="1" errors="0" skipped="0" time="0.75">
    <testcase classname="pkg.A" name="passes" time="0.10"/>
    <testcase classname="pkg.A" name="fails" time="0.20"><failure type="assert" message="want true">stack line</failure></testcase>
  </testsuite>
  <testsuite name="integration" tests="1" failures="0" errors="0" skipped="1" time="0.50">
    <testcase classname="pkg.B" name="skips"><skipped message="not configured"/></testcase>
  </testsuite>
</testsuites>` + "`" + `
				bad_report, bad_report_err := dialect.eval("junit", "<testsuite tests=\"nope\"/>")
				route := dialect.eval("urlpath", "/v1/users/ada/files/docs/readme.md", {template: "/v1/users/{id}/files/{*rest}", mode: "match_template"})
				built := dialect.eval("urlpath", {id: "ada", rest: "docs/read me.md"}, {template: "/v1/users/{id}/files/{*rest}", mode: "encode_template"})

				report_name := report.name
				report_tests := report.tests
				report_passed := report.passed
				first_suite := report.suites[1].name
				failed_status := report.cases[2].status
				failed_message := report.cases[2].message
				failed_text := report.cases[2].text
				skipped_status := report.cases[3].status
				bad_report_is_nil := bad_report == nil
				route_ok := route.matched
				route_id := route.params.id
				route_rest := route.params.rest
			`
			if err := vm.Exec(src); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "report_name", "ci")
			assertGet(t, vm, "report_tests", int64(3))
			assertGet(t, vm, "report_passed", int64(1))
			assertGet(t, vm, "first_suite", "unit")
			assertGet(t, vm, "failed_status", "failed")
			assertGet(t, vm, "failed_message", "want true")
			assertGet(t, vm, "failed_text", "stack line")
			assertGet(t, vm, "skipped_status", "skipped")
			assertGet(t, vm, "bad_report_is_nil", true)
			assertStringContains(t, vm, "bad_report_err", `junit dialect: testsuite 1: invalid tests attribute "nope"`)
			assertGet(t, vm, "route_ok", true)
			assertGet(t, vm, "route_id", "ada")
			assertGet(t, vm, "route_rest", "docs/readme.md")
			assertGet(t, vm, "built", "/v1/users/ada/files/docs/read%20me.md")
		})
	}
}

func TestStdlibMarkdownDialectCodeBlocksExecuteThroughStdlib(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibAll)}, tc.opts...)...)
			src := "doc := markdown`# Notes\n\n~~~leia\nprint(\"ok\")\n~~~\n`\n" +
				"code_count := #doc.code_blocks\n" +
				"code_info := doc.code_blocks[1].info\n" +
				"code_text := doc.code_blocks[1].text\n"
			if err := vm.Exec(src); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "code_count", int64(1))
			assertGet(t, vm, "code_info", "leia")
			assertGet(t, vm, "code_text", `print("ok")`)
		})
	}
}

func TestStdlibSemVerDialectExecutesThroughStdlib(t *testing.T) {
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
			}, tc.opts...)...)
			err := vm.Exec(
				"release := semver`1.2.3-rc.1+build.7`\n" +
					"encoded := dialect.eval(\"semver\", {major: 2, minor: 0, patch: 1, prerelease: {\"beta\", \"2\"}, build: {\"ci\", \"0042\"}}, {mode: \"encode\"})\n" +
					"formatted := dialect.eval(\"semver\", {major: 3, minor: 4, patch: 5, pre: \"alpha.1\", build_metadata: \"sha.abcdef\"}, {mode: \"format\"})\n" +
					"roundtrip := dialect.eval(\"semver\", encoded)\n" +
					"bad, bad_err := dialect.eval(\"semver\", \"1.02.3\")\n" +
					"major := release.major\n" +
					"pre_2 := release.prerelease[2]\n" +
					"build_2 := release.build[2]\n" +
					"roundtrip_pre := roundtrip.pre\n" +
					"bad_is_nil := bad == nil\n")
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "major", int64(1))
			assertGet(t, vm, "pre_2", "1")
			assertGet(t, vm, "build_2", "7")
			assertGet(t, vm, "encoded", "2.0.1-beta.2+ci.0042")
			assertGet(t, vm, "formatted", "3.4.5-alpha.1+sha.abcdef")
			assertGet(t, vm, "roundtrip_pre", "beta.2")
			assertGet(t, vm, "bad_is_nil", true)
			assertStringContains(t, vm, "bad_err", "semver dialect: minor number has leading zero")
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

func TestShellShortcutBangFailsFast(t *testing.T) {
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
			if err := vm.Exec("ok := $!`printf shell-ok`\nok_text := ok.text"); err != nil {
				t.Fatalf("Exec success case: %v", err)
			}
			assertGet(t, vm, "ok_text", "shell-ok")

			err := vm.Exec("failed := $!`printf fastfailerr 1>&2; exit 9`")
			if err == nil || !strings.Contains(err.Error(), "sh dialect failed with exit code 9: fastfailerr") {
				t.Fatalf("Exec err = %v, want fail-fast shell error", err)
			}
		})
	}
}

func TestCommandDialectBangFailsFast(t *testing.T) {
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
			if err := vm.Exec("ok := cmd!`printf command-ok`\nok_text := ok.text"); err != nil {
				t.Fatalf("Exec success case: %v", err)
			}
			assertGet(t, vm, "ok_text", "command-ok")

			err := vm.Exec("failed := cmd!`sh -c 'printf cmdfailerr 1>&2; exit 8'`")
			if err == nil || !strings.Contains(err.Error(), "cmd dialect failed with exit code 8: cmdfailerr") {
				t.Fatalf("Exec err = %v, want fail-fast cmd error", err)
			}
		})
	}
}

func TestStringInterpolationFormsExecuteThroughInterpreterAndBytecode(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(tc.opts...)
			if err := vm.Exec("name := \"Ada\"\nscore := 42\nquoted := \"user=${name};score=${score};next=${score + 1}\"\nsingle := 'user=${name}'\nraw := `user=${name}`"); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "quoted", "user=Ada;score=42;next=43")
			assertGet(t, vm, "single", "user=${name}")
			assertGet(t, vm, "raw", "user=${name}")
		})
	}
}

func TestQTaggedInterpolationEncodesNumericRuntimeLists(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibAll)}, tc.opts...)...)
			if err := vm.Exec(`
a := [1,2,3,4,5,6,7,8,6]
x := q` + "`sum ${a}`" + `
dense := array.f64(1.5, 2.5, 3.0)
dense_sum := q` + "`sum ${dense}`" + `
vec := linalg.vector(2, 4, 6)
vec_sum := q` + "`sum ${vec}`" + `
name := "abc"
n := q` + "`count ${name}`" + `
flag := true
choice := q` + "`$[${flag};10;20]`" + `
matrix_value := matrix.dense(2, 2)
matrix_ok, matrix_err := pcall(func() { return q` + "`sum ${matrix_value}`" + ` })
linalg_matrix := linalg.matrix(2, 2, {1, 2, 3, 4})
linalg_matrix_ok, linalg_matrix_err := pcall(func() { return q` + "`sum ${linalg_matrix}`" + ` })
frame_value := data.frame({x: data.i64({1, 2})})
frame_ok, frame_err := pcall(func() { return q` + "`sum ${frame_value}`" + ` })
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "x", int64(42))
			assertGet(t, vm, "dense_sum", float64(7))
			assertGet(t, vm, "vec_sum", int64(12))
			assertGet(t, vm, "n", int64(3))
			assertGet(t, vm, "choice", int64(10))
			assertGet(t, vm, "matrix_ok", false)
			assertStringContains(t, vm, "matrix_err", "q interpolation does not support matrix.dense values")
			assertGet(t, vm, "linalg_matrix_ok", false)
			assertStringContains(t, vm, "linalg_matrix_err", "q interpolation does not support matrix.dense values")
			assertGet(t, vm, "frame_ok", false)
			assertStringContains(t, vm, "frame_err", "q interpolation does not support frame values")
		})
	}
}

func TestQRawSourceBlockExecutesThroughDialect(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibAll)}, tc.opts...)...)
			if err := vm.Exec("info := dialect.info(\"q\")\n" +
				"has_block := info.block\n" +
				"sum_inline := q {+/1 2 3}\n" +
				"sum_multi := q {\n" +
				"sum 4 5 6\n" +
				"}\n" +
				"symbol_count := q {\n" +
				"count `AAPL`MSFT`NVDA\n" +
				"}\n" +
				"choice := q {$[1b;10;20]}\n" +
				"bad_ok, bad_err := pcall(func() { return q {not-a-q-token} })\n"); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "has_block", true)
			assertGet(t, vm, "sum_inline", int64(6))
			assertGet(t, vm, "sum_multi", int64(15))
			assertGet(t, vm, "symbol_count", int64(3))
			assertGet(t, vm, "choice", int64(10))
			assertGet(t, vm, "bad_ok", false)
			assertStringContains(t, vm, "bad_err", "q dialect:")
		})
	}
}

func TestQIdentifierControlFlowDoesNotBecomeRawBlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibAll)}, tc.opts...)...)
			if err := vm.Exec(`
q := true
x := 0
if q {
    x = 1
}
n := 0
for q {
    n++
    q = false
}
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			assertGet(t, vm, "x", int64(1))
			assertGet(t, vm, "n", int64(1))
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
