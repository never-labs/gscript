package bind

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDialectTagsExposeInstalledHandlers(t *testing.T) {
	interp := runWithLib(t, `
		tags := dialect.tags()
	`, "dialect", BuildDialect(HostOptions{}, nil))

	got := stringSetFromArray(interp.GetGlobal("tags").Table())
	want := []string{
		"base32", "base64", "binary", "cidr", "cmd", "cookie", "cookies", "csv", "deflate", "duration", "emailaddr", "env", "glob",
		"gzip", "hash", "headers", "hex", "hostport", "html", "html_escape", "http_headers", "httpmsg", "ini", "ipaddr", "json", "jsonl", "jsonptr",
		"junit", "jwt", "kv", "lines", "logfmt", "mailaddr", "markdown", "md", "mdtable", "mime", "multipart", "numbers", "nums", "path", "pem", "prompt",
		"quote", "re", "regexp", "rfc3339", "semver", "sh", "shellwords", "split", "sql", "sse", "tap", "template", "timestamp", "tsv", "url",
		"urlpath", "urlquery", "uuid", "words", "xml", "yaml", "yml", "zlib",
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("dialect.tags missing %q; got %#v", name, got)
		}
	}
	for _, reserved := range []string{"agent", "command", "data-science", "db", "fs", "network", "shell", "test", "text", "web", "workflow"} {
		if got[reserved] {
			t.Fatalf("dialect.tags unexpectedly exposes reserved category %q", reserved)
		}
	}
	if gotList := stringSliceFromArray(interp.GetGlobal("tags").Table()); !reflect.DeepEqual(gotList, want) {
		t.Fatalf("dialect.tags = %#v, want %#v", gotList, want)
	}
}

func TestDialectUnknownTagListsInstalledHandlers(t *testing.T) {
	eval := BuildDialect(HostOptions{}, nil).RawGetString("eval").GoFunction()
	if eval == nil {
		t.Fatalf("dialect.eval is not a Go function")
	}
	_, err := eval.Fn([]Value{StringValue("workflow"), StringValue("")})
	if err == nil {
		t.Fatalf("unknown dialect returned nil error")
	}
	msg := err.Error()
	for _, want := range []string{`unknown dialect "workflow"`, "available:", "cmd", "json", "prompt", "urlquery"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("unknown dialect error = %q, want containing %q", msg, want)
		}
	}
}

func TestDialectInfoAndListExposeMetadata(t *testing.T) {
	interp := runWithLib(t, `
		tags := dialect.tags()
		sh_info := dialect.info("sh")
		glob_info := dialect.info("glob")
		json_info := dialect.info("json")
		jsonptr_info := dialect.info("jsonptr")
		logfmt_info := dialect.info("logfmt")
		junit_info := dialect.info("junit")
		sse_info := dialect.info("sse")
		multipart_info := dialect.info("multipart")
		missing_info := dialect.info("missing")
		all_info := dialect.list()
	`, "dialect", BuildDialect(HostOptions{}, nil))

	shInfo := interp.GetGlobal("sh_info").Table()
	if got := shInfo.RawGetString("category").Str(); got != "host" {
		t.Fatalf("sh category = %q, want host", got)
	}
	if !shInfo.RawGetString("builtin").Bool() || !shInfo.RawGetString("eval").Bool() || shInfo.RawGetString("block").Bool() {
		t.Fatalf("sh info flags = builtin:%v eval:%v block:%v, want builtin eval-only", shInfo.RawGetString("builtin"), shInfo.RawGetString("eval"), shInfo.RawGetString("block"))
	}
	if got := stringSliceFromArray(shInfo.RawGetString("capabilities").Table()); !reflect.DeepEqual(got, []string{"process.shell"}) {
		t.Fatalf("sh capabilities = %#v, want process.shell", got)
	}
	if got := stringSliceFromArray(interp.GetGlobal("glob_info").Table().RawGetString("capabilities").Table()); !reflect.DeepEqual(got, []string{"fs.read"}) {
		t.Fatalf("glob capabilities = %#v, want fs.read", got)
	}
	if got := interp.GetGlobal("json_info").Table().RawGetString("category").Str(); got != "text" {
		t.Fatalf("json category = %q, want text", got)
	}
	for name, global := range map[string]string{
		"jsonptr":   "jsonptr_info",
		"logfmt":    "logfmt_info",
		"junit":     "junit_info",
		"sse":       "sse_info",
		"multipart": "multipart_info",
	} {
		category := "text"
		if name == "sse" || name == "multipart" {
			category = "protocol"
		} else if name == "junit" {
			category = "compat"
		}
		if got := interp.GetGlobal(global).Table().RawGetString("category").Str(); got != category {
			t.Fatalf("%s category = %q, want %s", name, got, category)
		}
	}
	if !interp.GetGlobal("missing_info").IsNil() {
		t.Fatalf("missing dialect info = %v, want nil", interp.GetGlobal("missing_info"))
	}
	if got, want := interp.GetGlobal("all_info").Table().Length(), interp.GetGlobal("tags").Table().Length(); got != want {
		t.Fatalf("dialect.list length = %d, want tags length %d", got, want)
	}
	if first := interp.GetGlobal("all_info").Table().RawGetInt(1).Table(); !first.RawGetString("name").IsString() || !first.RawGetString("category").IsString() {
		t.Fatalf("dialect.list first entry missing name/category: %v", first)
	}
}

func TestBuiltinDialectInfoCategoriesAreExplicit(t *testing.T) {
	interp := runWithLib(t, `
		tags := dialect.tags()
		all_info := dialect.list()
	`, "dialect", BuildDialect(HostOptions{}, nil))

	tags := stringSliceFromArray(interp.GetGlobal("tags").Table())
	allInfo := interp.GetGlobal("all_info").Table()
	wantCategories := expectedBuiltinDialectCategories()
	if got, want := allInfo.Length(), len(tags); got != want {
		t.Fatalf("dialect.list length = %d, want %d", got, want)
	}
	for i, tag := range tags {
		info := allInfo.RawGetInt(int64(i + 1)).Table()
		if got := info.RawGetString("name").Str(); got != tag {
			t.Fatalf("dialect.list[%d].name = %q, want %q", i+1, got, tag)
		}
		if !info.RawGetString("builtin").Bool() {
			t.Fatalf("builtin dialect %q reports builtin=false", tag)
		}
		category := info.RawGetString("category").Str()
		wantCategory, ok := wantCategories[tag]
		if !ok {
			t.Fatalf("builtin dialect %q has no category gate entry; update expectedBuiltinDialectCategories", tag)
		}
		if category != wantCategory {
			t.Fatalf("builtin dialect %q category = %q; want %q", tag, category, wantCategory)
		}
		delete(wantCategories, tag)
	}
	if len(wantCategories) > 0 {
		t.Fatalf("category gate contains dialects not exposed by dialect.tags: %#v", wantCategories)
	}
}

func expectedBuiltinDialectCategories() map[string]string {
	return map[string]string{
		"sh": "host", "cmd": "host", "glob": "host",
		"shellwords": "text", "path": "text", "re": "text", "regexp": "text", "json": "text", "jsonptr": "text", "jsonl": "text", "csv": "text", "tsv": "text", "mdtable": "text", "markdown": "text", "md": "text", "lines": "text", "split": "text", "words": "text", "nums": "text", "numbers": "text", "kv": "text", "logfmt": "text", "env": "text", "ini": "text", "yaml": "text", "yml": "text", "semver": "text", "duration": "text", "timestamp": "text", "rfc3339": "text", "tap": "text", "xml": "text", "template": "text",
		"url": "protocol", "html_escape": "protocol", "html": "protocol", "urlquery": "protocol", "urlpath": "protocol", "mime": "protocol", "mailaddr": "protocol", "emailaddr": "protocol", "headers": "protocol", "http_headers": "protocol", "cookie": "protocol", "cookies": "protocol", "httpmsg": "protocol", "sse": "protocol", "multipart": "protocol", "jwt": "protocol", "ipaddr": "protocol", "cidr": "protocol", "hostport": "protocol",
		"base64": "data", "hash": "data", "hex": "data", "base32": "data", "uuid": "data", "gzip": "data", "zlib": "data", "deflate": "data", "binary": "data", "pem": "data",
		"sql":    "database",
		"prompt": "llm", "quote": "llm",
		"junit": "compat",
	}
}

func TestDialectRegistryRejectsDuplicateNames(t *testing.T) {
	registry := newDialectRegistry()
	handler := dialectHandler{eval: func(Value, *Table) ([]Value, error) {
		return []Value{NilValue()}, nil
	}}
	registry.register([]string{"json"}, handler)
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("duplicate dialect registration did not panic")
		}
		if got := fmt.Sprint(recovered); !strings.Contains(got, `duplicate dialect "json"`) {
			t.Fatalf("panic = %q, want duplicate dialect json", got)
		}
	}()
	registry.register([]string{"json"}, handler)
}

func TestDialectRegistryRejectsDuplicateNamesInSingleRegistration(t *testing.T) {
	registry := newDialectRegistry()
	handler := dialectHandler{eval: func(Value, *Table) ([]Value, error) {
		return []Value{NilValue()}, nil
	}}
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("duplicate dialect alias in same registration did not panic")
		}
		if got := fmt.Sprint(recovered); !strings.Contains(got, `duplicate dialect "json"`) {
			t.Fatalf("panic = %q, want duplicate dialect json", got)
		}
	}()
	registry.register([]string{"json", "json"}, handler)
}

func TestDialectRegisterScriptHandler(t *testing.T) {
	interp := New()
	installTestModule(interp, "dialect", TableValue(BuildDialect(HostOptions{Call: interp.CallFunction}, nil)))
	execOnInterp(t, interp, `
		dialect.register("wrap", func(body, opts) {
			prefix := "<"
			suffix := ">"
			if opts != nil && opts.prefix != nil { prefix = opts.prefix }
			if opts != nil && opts.suffix != nil { suffix = opts.suffix }
			return prefix .. body .. suffix
		}, {aliases: {"bracket"}, category: "text", capabilities: {"text.wrap"}})

		literal := wrap`+"`"+`ok`+"`"+`
		via_alias := bracket`+"`"+`ok`+"`"+`
		explicit := dialect.eval("wrap", "ok", {prefix: "[", suffix: "]"})
		tags := dialect.tags()
		info := dialect.info("wrap")
		alias_info := dialect.info("bracket")
	`)

	if got := interp.GetGlobal("literal").Str(); got != "<ok>" {
		t.Fatalf("literal = %q, want <ok>", got)
	}
	if got := interp.GetGlobal("via_alias").Str(); got != "<ok>" {
		t.Fatalf("via_alias = %q, want <ok>", got)
	}
	if got := interp.GetGlobal("explicit").Str(); got != "[ok]" {
		t.Fatalf("explicit = %q, want [ok]", got)
	}
	tags := stringSetFromArray(interp.GetGlobal("tags").Table())
	if !tags["wrap"] || !tags["bracket"] {
		t.Fatalf("registered tags missing from dialect.tags: %#v", tags)
	}
	info := interp.GetGlobal("info").Table()
	if got := info.RawGetString("category").Str(); got != "text" {
		t.Fatalf("registered category = %q, want text", got)
	}
	if info.RawGetString("builtin").Bool() {
		t.Fatalf("registered builtin flag = true, want false")
	}
	if got := stringSliceFromArray(info.RawGetString("aliases").Table()); !reflect.DeepEqual(got, []string{"bracket"}) {
		t.Fatalf("registered aliases = %#v, want bracket", got)
	}
	if got := stringSliceFromArray(info.RawGetString("capabilities").Table()); !reflect.DeepEqual(got, []string{"text.wrap"}) {
		t.Fatalf("registered capabilities = %#v, want text.wrap", got)
	}
	if got := stringSliceFromArray(interp.GetGlobal("alias_info").Table().RawGetString("aliases").Table()); !reflect.DeepEqual(got, []string{"wrap"}) {
		t.Fatalf("alias info aliases = %#v, want wrap", got)
	}
}

func TestDialectRegisterScriptBlockHandler(t *testing.T) {
	interp := New()
	installTestModule(interp, "dialect", TableValue(BuildDialect(HostOptions{Call: interp.CallFunction}, nil)))
	execOnInterp(t, interp, `
		dialect.register({
			name: "box",
			aliases: {"boxcfg"},
			eval: func(body, opts) {
				return {kind: "eval", body: body}
			},
			block: func(body, opts) {
				return {kind: "block", title: body.title, body: body}
			},
		})

		result := box {
			title: "Plan"
			steps: 3
		}
		raw := dialect.eval("box", "plain")
	`)

	result := interp.GetGlobal("result").Table()
	if got := result.RawGetString("kind").Str(); got != "block" {
		t.Fatalf("result.kind = %q, want block", got)
	}
	if got := result.RawGetString("title").Str(); got != "Plan" {
		t.Fatalf("result.title = %q, want Plan", got)
	}
	raw := interp.GetGlobal("raw").Table()
	if got := raw.RawGetString("kind").Str(); got != "eval" {
		t.Fatalf("raw.kind = %q, want eval", got)
	}
}

func TestDialectRegisterRejectsDuplicateBuiltin(t *testing.T) {
	interp := New()
	installTestModule(interp, "dialect", TableValue(BuildDialect(HostOptions{Call: interp.CallFunction}, nil)))
	err := execSourceOnInterp(interp, `
		dialect.register("json", func(body, opts) { return body })
	`)
	if err == nil || !strings.Contains(err.Error(), `duplicate dialect "json"`) {
		t.Fatalf("duplicate register err = %v, want duplicate json", err)
	}
}

func TestDialectShellwordsParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("shellwords", "printf 'hello world' a\\ b \"\"")
		to_encode := {}
		to_encode[1] = "printf"
		to_encode[2] = "%s\n"
		to_encode[3] = "hello world"
		to_encode[4] = "it's"
		to_encode[5] = ""
		encoded := dialect.eval("shellwords", to_encode, {mode: "encode"})
		formatted := dialect.eval("shellwords", to_encode, {mode: "format"})
		roundtrip := dialect.eval("shellwords", encoded)
		bad, bad_err := dialect.eval("shellwords", "'unterminated")
		bad_mode, bad_mode_err := dialect.eval("shellwords", "printf ok", {mode: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := stringSliceFromArray(interp.GetGlobal("parsed").Table())
	wantParsed := []string{"printf", "hello world", "a b", ""}
	if !reflect.DeepEqual(parsed, wantParsed) {
		t.Fatalf("parsed = %#v, want %#v", parsed, wantParsed)
	}
	wantEncoded := "printf '%s\n' 'hello world' 'it'\\''s' ''"
	if got := interp.GetGlobal("encoded").Str(); got != wantEncoded {
		t.Fatalf("encoded = %q, want %q", got, wantEncoded)
	}
	if got := interp.GetGlobal("formatted").Str(); got != wantEncoded {
		t.Fatalf("formatted = %q, want %q", got, wantEncoded)
	}
	roundtrip := stringSliceFromArray(interp.GetGlobal("roundtrip").Table())
	wantRoundtrip := []string{"printf", "%s\n", "hello world", "it's", ""}
	if !reflect.DeepEqual(roundtrip, wantRoundtrip) {
		t.Fatalf("roundtrip = %#v, want %#v", roundtrip, wantRoundtrip)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad shellwords returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err"); !got.IsString() || !strings.Contains(got.Str(), "unterminated single quote") {
		t.Fatalf("bad_err = %v, want unterminated single quote", got)
	}
	assertDialectModeError(t, interp.GetGlobal("bad_mode"), interp.GetGlobal("bad_mode_err"), "shellwords dialect: unknown mode")
}

func TestDialectCommandStructuredOptions(t *testing.T) {
	interp := runWithLib(t, `
		stdin_env := dialect.eval("cmd", {
			cmd: "sh",
			args: {"-c", "cat; printf :$LEIA_DIALECT_TEST"},
			stdin: "payload",
			env: {LEIA_DIALECT_TEST: "env-ok"},
		})
		argv := dialect.eval("cmd", {"printf", "argv-%s", "ok"})
		string_opts := dialect.eval("cmd", "sh -c 'cat; printf :$LEIA_DIALECT_TEST'", {
			stdin: "body",
			env: {LEIA_DIALECT_TEST: "from-options"},
		})
		option_override := dialect.eval("cmd", {
			cmd: "sh",
			args: {"-c", "cat; printf :$LEIA_DIALECT_TEST"},
			stdin: "body-a",
			env: {LEIA_DIALECT_TEST: "env-a"},
		}, {
			stdin: "body-b",
			env: {LEIA_DIALECT_TEST: "env-b"},
		})
		cwd_alias := dialect.eval("cmd", {
			cmd: "sh",
			args: {"-c", "basename \"$PWD\""},
			cwd: ".",
		})
		option_cwd := dialect.eval("cmd", {
			cmd: "sh",
			args: {"-c", "basename \"$PWD\""},
			cwd: "/",
		}, {cwd: "."})
		missing := dialect.eval("cmd", "definitely-not-a-leia-command")
		bad_words, bad_words_err := dialect.eval("cmd", "'unterminated")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	stdinEnv := interp.GetGlobal("stdin_env").Table()
	if !stdinEnv.RawGetString("ok").Bool() {
		t.Fatalf("stdin_env ok = false, stderr=%q", stdinEnv.RawGetString("stderr").Str())
	}
	if got := stdinEnv.RawGetString("text").Str(); got != "payload:env-ok" {
		t.Fatalf("stdin_env text = %q, want payload:env-ok", got)
	}
	argv := interp.GetGlobal("argv").Table()
	if !argv.RawGetString("ok").Bool() {
		t.Fatalf("argv ok = false, stderr=%q", argv.RawGetString("stderr").Str())
	}
	if got := argv.RawGetString("text").Str(); got != "argv-ok" {
		t.Fatalf("argv text = %q, want argv-ok", got)
	}
	stringOpts := interp.GetGlobal("string_opts").Table()
	if !stringOpts.RawGetString("ok").Bool() {
		t.Fatalf("string_opts ok = false, stderr=%q", stringOpts.RawGetString("stderr").Str())
	}
	if got := stringOpts.RawGetString("text").Str(); got != "body:from-options" {
		t.Fatalf("string_opts text = %q, want body:from-options", got)
	}
	optionOverride := interp.GetGlobal("option_override").Table()
	if !optionOverride.RawGetString("ok").Bool() {
		t.Fatalf("option_override ok = false, stderr=%q", optionOverride.RawGetString("stderr").Str())
	}
	if got := optionOverride.RawGetString("text").Str(); got != "body-b:env-b" {
		t.Fatalf("option_override text = %q, want body-b:env-b", got)
	}
	cwdAlias := interp.GetGlobal("cwd_alias").Table()
	if !cwdAlias.RawGetString("ok").Bool() {
		t.Fatalf("cwd_alias ok = false, stderr=%q", cwdAlias.RawGetString("stderr").Str())
	}
	if got := strings.TrimSpace(cwdAlias.RawGetString("text").Str()); got != "bind" {
		t.Fatalf("cwd_alias basename = %q, want bind", got)
	}
	optionCWD := interp.GetGlobal("option_cwd").Table()
	if !optionCWD.RawGetString("ok").Bool() {
		t.Fatalf("option_cwd ok = false, stderr=%q", optionCWD.RawGetString("stderr").Str())
	}
	if got := strings.TrimSpace(optionCWD.RawGetString("text").Str()); got != "bind" {
		t.Fatalf("option_cwd basename = %q, want bind", got)
	}
	missing := interp.GetGlobal("missing").Table()
	if missing.RawGetString("ok").Bool() {
		t.Fatalf("missing command ok = true, want false")
	}
	if got := missing.RawGetString("code").Int(); got != -1 {
		t.Fatalf("missing command code = %d, want -1", got)
	}
	if !interp.GetGlobal("bad_words").IsNil() {
		t.Fatalf("bad command words returned non-nil result")
	}
	if got := interp.GetGlobal("bad_words_err"); !got.IsString() || !strings.Contains(got.Str(), "unterminated single quote") {
		t.Fatalf("bad command words err = %v, want shellwords error", got)
	}
}

func TestDialectCommandHostPoliciesAndFailFast(t *testing.T) {
	eval := BuildDialect(HostOptions{
		EnvironmentWrite: func() bool { return false },
	}, nil).RawGetString("eval").GoFunction()
	if eval == nil {
		t.Fatalf("dialect.eval is not a Go function")
	}
	envBlocked, err := eval.Fn([]Value{
		StringValue("cmd"),
		TableValue(testTableFromMap(map[string]Value{
			"cmd":  StringValue("sh"),
			"args": TableValue(testArrayTableFromValues([]Value{StringValue("-c"), StringValue("printf $LEIA_BLOCKED")})),
			"env":  TableValue(testTableFromMap(map[string]Value{"LEIA_BLOCKED": StringValue("blocked")})),
		})),
	})
	if err != nil {
		t.Fatalf("env blocked returned Go error: %v", err)
	}
	if len(envBlocked) != 2 || !envBlocked[0].IsNil() || !strings.Contains(envBlocked[1].Str(), "environment write access disabled") {
		t.Fatalf("env blocked result = %#v, want nil + environment error", envBlocked)
	}

	_, err = BuildDialect(HostOptions{}, nil).RawGetString("eval").GoFunction().Fn([]Value{
		StringValue("cmd"),
		StringValue("sh -c 'printf cmderr 1>&2; exit 4'"),
		TableValue(testTableFromMap(map[string]Value{"fail_fast": BoolValue(true)})),
	})
	if err == nil || !strings.Contains(err.Error(), "cmd dialect failed with exit code 4: cmderr") {
		t.Fatalf("fail_fast err = %v, want cmd exit error", err)
	}
}

func TestDialectShellwordsArgumentErrors(t *testing.T) {
	eval := BuildDialect(HostOptions{}, nil).RawGetString("eval").GoFunction()
	for _, tc := range []struct {
		name string
		body Value
		want string
	}{
		{name: "non table", body: IntValue(7), want: "table or string expected"},
		{name: "nested table", body: TableValue(testArrayTableFromValues([]Value{TableValue(NewTable())})), want: "argument 1 must be scalar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := eval.Fn([]Value{StringValue("shellwords"), tc.body, TableValue(testTableFromMap(map[string]Value{"mode": StringValue("encode")}))})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("shellwords err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestDialectGlobEmptyAndInvalidPattern(t *testing.T) {
	interp := runWithLib(t, `
		empty := dialect.eval("glob", "definitely-no-leia-file-*")
		bad, bad_err := dialect.eval("glob", "[")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("empty").Table().Length(); got != 0 {
		t.Fatalf("empty glob length = %d, want 0", got)
	}
	if !interp.GetGlobal("bad").IsNil() || !strings.Contains(interp.GetGlobal("bad_err").Str(), "syntax error in pattern") {
		t.Fatalf("bad glob = %v err %v, want nil syntax error", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
}

func testTableFromMap(values map[string]Value) *Table {
	t := NewTable()
	for key, value := range values {
		t.RawSetString(key, value)
	}
	return t
}

func testArrayTableFromValues(values []Value) *Table {
	t := NewTable()
	for i, value := range values {
		t.RawSetInt(int64(i+1), value)
	}
	return t
}

func TestDialectBlockAndRawEvalHelpers(t *testing.T) {
	interp := runWithLib(t, `
		encoded := dialect.eval_block("json", {x: 1, label: "ok"})
		prompt_msg := dialect.eval_block("prompt", {role: "system", text: "hi"})
		raw := dialect.eval_raw("triage-plan", {step: "collect"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("encoded").Str(); got != `{"label":"ok","x":1}` {
		t.Fatalf("eval_block json = %q, want encoded object", got)
	}
	prompt := interp.GetGlobal("prompt_msg").Table()
	body := prompt.RawGetString("body").Table()
	if got := body.RawGetString("role").Str(); got != "system" {
		t.Fatalf("prompt body role = %q, want system", got)
	}
	if got := body.RawGetString("text").Str(); got != "hi" {
		t.Fatalf("prompt body text = %q, want hi", got)
	}
	raw := interp.GetGlobal("raw").Table()
	if got := raw.RawGetString("dialect").Str(); got != "triage-plan" {
		t.Fatalf("raw dialect = %q, want triage-plan", got)
	}
	if got := raw.RawGetString("kind").Str(); got != "table" {
		t.Fatalf("raw kind = %q, want table", got)
	}
	if got := raw.RawGetString("body").Table().RawGetString("step").Str(); got != "collect" {
		t.Fatalf("raw body step = %q, want collect", got)
	}
}

func TestDialectJUnitSummary(t *testing.T) {
	interp := runWithLib(t, `
		report := junit`+"`"+`<testsuites name="ci" tests="3" failures="1" errors="0" skipped="1" time="1.25">
  <testsuite name="unit" tests="2" failures="1" errors="0" skipped="0" time="0.75">
    <testcase classname="pkg.A" name="passes" time="0.10"/>
    <testcase classname="pkg.A" name="fails" time="0.20"><failure type="assert" message="want true">stack line</failure></testcase>
  </testsuite>
  <testsuite name="integration" tests="1" failures="0" errors="0" skipped="1" time="0.50">
    <testcase classname="pkg.B" name="skips"><skipped message="not configured"/></testcase>
  </testsuite>
</testsuites>`+"`"+`
		bad, bad_err := dialect.eval("junit", "<testsuite tests=\"nope\"/>")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	report := interp.GetGlobal("report").Table()
	if got := report.RawGetString("name").Str(); got != "ci" {
		t.Fatalf("report.name = %q, want ci", got)
	}
	if got := report.RawGetString("tests").Int(); got != 3 {
		t.Fatalf("report.tests = %d, want 3", got)
	}
	if got := report.RawGetString("passed").Int(); got != 1 {
		t.Fatalf("report.passed = %d, want 1", got)
	}
	if got := report.RawGetString("suites").Table().RawGetInt(1).Table().RawGetString("name").Str(); got != "unit" {
		t.Fatalf("first suite name = %q, want unit", got)
	}
	failed := report.RawGetString("cases").Table().RawGetInt(2).Table()
	if got := failed.RawGetString("status").Str(); got != "failed" {
		t.Fatalf("failed.status = %q, want failed", got)
	}
	if got := failed.RawGetString("message").Str(); got != "want true" {
		t.Fatalf("failed.message = %q, want want true", got)
	}
	if got := failed.RawGetString("text").Str(); got != "stack line" {
		t.Fatalf("failed.text = %q, want stack line", got)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad junit returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err").Str(); !strings.Contains(got, `junit dialect: testsuite 1: invalid tests attribute "nope"`) {
		t.Fatalf("bad_err = %q", got)
	}
}

func TestDialectXMLEscapeAndUnescape(t *testing.T) {
	interp := runWithLib(t, `
		escaped := xml`+"`"+`<node attr="a&b">Tom & 'Jerry'</node>`+"`"+`
		unescaped, unescape_err := dialect.eval("xml", escaped, {mode: "unescape"})
		decoded, decoded_err := dialect.eval("xml", "&lt;x&gt;&quot;q&quot; &apos;s&apos;&lt;/x&gt;", {mode: "decode"})
		encoded := dialect.eval("xml", "<x>&</x>", {mode: "encode"})
		bad, bad_err := dialect.eval("xml", "&#x;", {mode: "unescape"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got, want := interp.GetGlobal("escaped").Str(), `&lt;node attr=&#34;a&amp;b&#34;&gt;Tom &amp; &#39;Jerry&#39;&lt;/node&gt;`; got != want {
		t.Fatalf("escaped = %q, want %q", got, want)
	}
	if !interp.GetGlobal("unescape_err").IsNil() {
		t.Fatalf("unescape_err = %v, want nil", interp.GetGlobal("unescape_err"))
	}
	if got := interp.GetGlobal("unescaped").Str(); got != `<node attr="a&b">Tom & 'Jerry'</node>` {
		t.Fatalf("unescaped = %q", got)
	}
	if !interp.GetGlobal("decoded_err").IsNil() {
		t.Fatalf("decoded_err = %v, want nil", interp.GetGlobal("decoded_err"))
	}
	if got := interp.GetGlobal("decoded").Str(); got != `<x>"q" 's'</x>` {
		t.Fatalf("decoded = %q", got)
	}
	if got := interp.GetGlobal("encoded").Str(); got != "&lt;x&gt;&amp;&lt;/x&gt;" {
		t.Fatalf("encoded = %q, want escaped string form", got)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad xml returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err").Str(); !strings.Contains(got, "invalid XML numeric character reference") {
		t.Fatalf("bad_err = %q", got)
	}
}

func TestDialectLogfmtParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := logfmt`+"`"+`level=info msg="hello world" ok trace_id=abc`+"`"+`
		encoded := dialect.eval("logfmt", {msg: "hello world", level: "info", empty: ""}, {mode: "encode"})
		bad, bad_err := dialect.eval("logfmt", "level=info msg=\"oops")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("level").Str(); got != "info" {
		t.Fatalf("level = %q, want info", got)
	}
	if got := parsed.RawGetString("msg").Str(); got != "hello world" {
		t.Fatalf("msg = %q, want hello world", got)
	}
	if got := parsed.RawGetString("ok").Str(); got != "true" {
		t.Fatalf("ok flag = %q, want true", got)
	}
	pairs := parsed.RawGetString("pairs").Table()
	if got := pairs.RawGetInt(1).Table().RawGetString("key").Str(); got != "level" {
		t.Fatalf("first pair key = %q, want level", got)
	}
	if got := interp.GetGlobal("encoded").Str(); got != `empty="" level=info msg="hello world"` {
		t.Fatalf("encoded = %q", got)
	}
	if !interp.GetGlobal("bad").IsNil() || !interp.GetGlobal("bad_err").IsString() {
		t.Fatalf("bad logfmt = %v err %v, want nil error string", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
}

func TestDialectJSONPointerLookupAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		doc := json`+"`"+`{"events":[{"type":"token","data":"hello"},{"type":"done","data":{}}],"meta":{"trace/id":"agent-42"}}`+"`"+`
		first := dialect.eval("jsonptr", doc, {path: "/events/0/data"})
		trace := dialect.eval("jsonptr", doc, {path: "/meta/trace~1id"})
		encoded := dialect.eval("jsonptr", {"meta", "trace/id"}, {mode: "encode"})
		missing, missing_err := dialect.eval("jsonptr", doc, {path: "/events/2"})
		bad, bad_err := dialect.eval("jsonptr", doc, {path: "events/0"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("first").Str(); got != "hello" {
		t.Fatalf("first = %q, want hello", got)
	}
	if got := interp.GetGlobal("trace").Str(); got != "agent-42" {
		t.Fatalf("trace = %q, want agent-42", got)
	}
	if got := interp.GetGlobal("encoded").Str(); got != "/meta/trace~1id" {
		t.Fatalf("encoded = %q, want /meta/trace~1id", got)
	}
	if !interp.GetGlobal("missing").IsNil() || !interp.GetGlobal("missing_err").IsString() {
		t.Fatalf("missing = %v err %v, want nil error string", interp.GetGlobal("missing"), interp.GetGlobal("missing_err"))
	}
	if !interp.GetGlobal("bad").IsNil() || !interp.GetGlobal("bad_err").IsString() {
		t.Fatalf("bad = %v err %v, want nil error string", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
}

func TestDialectHexAndBase32RoundTrip(t *testing.T) {
	interp := runWithLib(t, `
		hexed := hex`+"`"+`go`+"`"+`
		hex_decoded, hex_err := dialect.eval("hex", hexed, {mode: "decode"})
		bad_hex, bad_hex_err := dialect.eval("hex", "xx", {mode: "decode"})
		bad_hex_mode, bad_hex_mode_err := dialect.eval("hex", "go", {mode: "bogus"})
		base32ed := base32`+"`"+`go`+"`"+`
		base32_decoded, base32_err := dialect.eval("base32", base32ed, {mode: "decode"})
		base32hexed := dialect.eval("base32", "go", {mode: "hex_encode"})
		base32hex_decoded, base32hex_err := dialect.eval("base32", base32hexed, {mode: "hex_decode"})
		bad_base32_mode, bad_base32_mode_err := dialect.eval("base32", "go", {mode: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("hexed").Str(); got != "676f" {
		t.Fatalf("hexed = %q, want 676f", got)
	}
	if !interp.GetGlobal("hex_err").IsNil() || interp.GetGlobal("hex_decoded").Str() != "go" {
		t.Fatalf("hex decode = %v err %v, want go nil", interp.GetGlobal("hex_decoded"), interp.GetGlobal("hex_err"))
	}
	if !interp.GetGlobal("bad_hex").IsNil() || !strings.Contains(interp.GetGlobal("bad_hex_err").Str(), "invalid byte") {
		t.Fatalf("bad hex = %v err %v, want nil invalid byte", interp.GetGlobal("bad_hex"), interp.GetGlobal("bad_hex_err"))
	}
	assertDialectModeError(t, interp.GetGlobal("bad_hex_mode"), interp.GetGlobal("bad_hex_mode_err"), "hex dialect: unknown mode")
	if got := interp.GetGlobal("base32ed").Str(); got != "M5XQ====" {
		t.Fatalf("base32ed = %q, want M5XQ====", got)
	}
	if !interp.GetGlobal("base32_err").IsNil() || interp.GetGlobal("base32_decoded").Str() != "go" {
		t.Fatalf("base32 decode = %v err %v, want go nil", interp.GetGlobal("base32_decoded"), interp.GetGlobal("base32_err"))
	}
	if got := interp.GetGlobal("base32hexed").Str(); got != "CTNG" {
		t.Fatalf("base32hexed = %q, want CTNG", got)
	}
	if !interp.GetGlobal("base32hex_err").IsNil() || interp.GetGlobal("base32hex_decoded").Str() != "go" {
		t.Fatalf("base32hex decode = %v err %v, want go nil", interp.GetGlobal("base32hex_decoded"), interp.GetGlobal("base32hex_err"))
	}
	assertDialectModeError(t, interp.GetGlobal("bad_base32_mode"), interp.GetGlobal("bad_base32_mode_err"), "base32 dialect: unknown mode")
}

func TestDialectUUIDParseAndValidate(t *testing.T) {
	interp := runWithLib(t, `
		parsed := uuid`+"`"+`550e8400-e29b-41d4-a716-446655440000`+"`"+`
		valid := dialect.eval("uuid", "550e8400-e29b-41d4-a716-446655440000", {mode: "is_valid"})
		invalid := dialect.eval("uuid", "not-a-uuid", {mode: "is_valid"})
		bad, bad_err := dialect.eval("uuid", "not-a-uuid")
		nil_uuid := dialect.eval("uuid", "", {mode: "nil"})
		bad_mode, bad_mode_err := dialect.eval("uuid", "", {mode: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("version").Int(); got != 4 {
		t.Fatalf("uuid version = %d, want 4", got)
	}
	if got := parsed.RawGetString("variant").Str(); got != "RFC4122" {
		t.Fatalf("uuid variant = %q, want RFC4122", got)
	}
	if got := parsed.RawGetString("bytes").Str(); got != "550e8400e29b41d4a716446655440000" {
		t.Fatalf("uuid bytes = %q", got)
	}
	if !interp.GetGlobal("valid").Bool() || interp.GetGlobal("invalid").Bool() {
		t.Fatalf("uuid valid flags = %v/%v, want true/false", interp.GetGlobal("valid"), interp.GetGlobal("invalid"))
	}
	if !interp.GetGlobal("bad").IsNil() || interp.GetGlobal("bad_err").Str() != "invalid UUID format" {
		t.Fatalf("bad uuid = %v err %v, want nil invalid UUID format", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
	if got := interp.GetGlobal("nil_uuid").Str(); got != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("nil uuid = %q", got)
	}
	assertDialectModeError(t, interp.GetGlobal("bad_mode"), interp.GetGlobal("bad_mode_err"), "uuid dialect: unknown mode")
}

func TestDialectCompressRoundTrip(t *testing.T) {
	interp := runWithLib(t, `
		data := "agent trace payload agent trace payload agent trace payload agent trace payload"
		gziped := gzip`+"`"+`agent trace payload agent trace payload agent trace payload`+"`"+`
		gzip_decoded, gzip_err := dialect.eval("gzip", gziped, {mode: "decode"})
		zlibed := dialect.eval("zlib", data, {level: 1})
		zlib_decoded, zlib_err := dialect.eval("zlib", zlibed, {mode: "decode"})
		deflated := dialect.eval("deflate", data)
		deflate_decoded, deflate_err := dialect.eval("deflate", deflated, {mode: "decode"})
		bad, bad_err := dialect.eval("gzip", "not gzip", {mode: "decode"})
		bad_mode, bad_mode_err := dialect.eval("gzip", data, {mode: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if !interp.GetGlobal("gzip_err").IsNil() || interp.GetGlobal("gzip_decoded").Str() != "agent trace payload agent trace payload agent trace payload" {
		t.Fatalf("gzip decoded = %v err %v", interp.GetGlobal("gzip_decoded"), interp.GetGlobal("gzip_err"))
	}
	if !interp.GetGlobal("zlib_err").IsNil() || interp.GetGlobal("zlib_decoded").Str() != interp.GetGlobal("data").Str() {
		t.Fatalf("zlib decoded = %v err %v", interp.GetGlobal("zlib_decoded"), interp.GetGlobal("zlib_err"))
	}
	if !interp.GetGlobal("deflate_err").IsNil() || interp.GetGlobal("deflate_decoded").Str() != interp.GetGlobal("data").Str() {
		t.Fatalf("deflate decoded = %v err %v", interp.GetGlobal("deflate_decoded"), interp.GetGlobal("deflate_err"))
	}
	if !interp.GetGlobal("bad").IsNil() || !interp.GetGlobal("bad_err").IsString() {
		t.Fatalf("bad gzip = %v err %v, want nil error string", interp.GetGlobal("bad"), interp.GetGlobal("bad_err"))
	}
	assertDialectModeError(t, interp.GetGlobal("bad_mode"), interp.GetGlobal("bad_mode_err"), "gzip dialect: unknown mode")
}

func TestDialectBase64HashUnknownOptionsReturnErrors(t *testing.T) {
	interp := runWithLib(t, `
		bad_base64, bad_base64_err := dialect.eval("base64", "go", {mode: "bogus"})
		bad_hash, bad_hash_err := dialect.eval("hash", "go", {algo: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	assertDialectModeError(t, interp.GetGlobal("bad_base64"), interp.GetGlobal("bad_base64_err"), "base64 dialect: unknown mode")
	if !interp.GetGlobal("bad_hash").IsNil() {
		t.Fatalf("bad hash value = %v, want nil", interp.GetGlobal("bad_hash"))
	}
	if got := interp.GetGlobal("bad_hash_err"); !got.IsString() || !strings.Contains(got.Str(), `hash dialect: unknown algorithm "bogus"`) {
		t.Fatalf("bad hash err = %v, want unknown algorithm", got)
	}
}

func TestDialectBinaryPackUnpackAndSize(t *testing.T) {
	interp := runWithLib(t, `
		packed := dialect.eval("binary", {258, "go"}, {mode: "pack", format: "be:u16 bytes:2"})
		hexed := dialect.eval("hex", packed)
		unpacked := dialect.eval("binary", packed, {mode: "unpack", format: "be:u16 bytes:2"})
		sized := dialect.eval("binary", "", {mode: "size", format: "be:u16 bytes:2"})
		mixed := dialect.eval("binary", {-1234, 16909060, 1.5, "OK"}, {mode: "pack", format: "le:i16 u32 f32 bytes:2"})
		mixed_hex := dialect.eval("hex", mixed)
		mixed_unpacked := dialect.eval("binary", mixed, {mode: "unpack", format: "le:i16 u32 f32 bytes:2"})
		mixed_size := dialect.eval("binary", "", {mode: "size", format: "le:i16 u32 f32 bytes:2"})
		var_size, var_err := dialect.eval("binary", "", {mode: "size", format: "string"})
		short, short_err := dialect.eval("binary", "x", {mode: "unpack", format: "u32"})
		bad_mode, bad_mode_err := dialect.eval("binary", "", {mode: "bogus", format: "u8"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("hexed").Str(); got != "0102676f" {
		t.Fatalf("binary hex = %q, want 0102676f", got)
	}
	unpacked := interp.GetGlobal("unpacked").Table()
	values := unpacked.RawGetString("values").Table()
	if got := values.RawGetInt(1).Int(); got != 258 {
		t.Fatalf("unpacked[1] = %d, want 258", got)
	}
	if got := values.RawGetInt(2).Str(); got != "go" {
		t.Fatalf("unpacked[2] = %q, want go", got)
	}
	if got := unpacked.RawGetString("next").Int(); got != 5 {
		t.Fatalf("next = %d, want 5", got)
	}
	if got := interp.GetGlobal("sized").Int(); got != 4 {
		t.Fatalf("size = %d, want 4", got)
	}
	if got := interp.GetGlobal("mixed_hex").Str(); got != "2efb040302010000c03f4f4b" {
		t.Fatalf("mixed binary hex = %q, want 2efb040302010000c03f4f4b", got)
	}
	mixed := interp.GetGlobal("mixed_unpacked").Table()
	mixedValues := mixed.RawGetString("values").Table()
	if got := mixedValues.RawGetInt(1).Int(); got != -1234 {
		t.Fatalf("mixed unpacked[1] = %d, want -1234", got)
	}
	if got := mixedValues.RawGetInt(2).Int(); got != 16909060 {
		t.Fatalf("mixed unpacked[2] = %d, want 16909060", got)
	}
	if got := mixedValues.RawGetInt(3).Number(); got != 1.5 {
		t.Fatalf("mixed unpacked[3] = %f, want 1.5", got)
	}
	if got := mixedValues.RawGetInt(4).Str(); got != "OK" {
		t.Fatalf("mixed unpacked[4] = %q, want OK", got)
	}
	if got := mixed.RawGetString("next").Int(); got != 13 {
		t.Fatalf("mixed next = %d, want 13", got)
	}
	if got := interp.GetGlobal("mixed_size").Int(); got != 12 {
		t.Fatalf("mixed size = %d, want 12", got)
	}
	if !interp.GetGlobal("var_size").IsNil() || !interp.GetGlobal("var_err").IsString() {
		t.Fatalf("variable size = %v err %v, want nil error string", interp.GetGlobal("var_size"), interp.GetGlobal("var_err"))
	}
	if !interp.GetGlobal("short").IsNil() || !strings.Contains(interp.GetGlobal("short_err").Str(), "binary dialect: data too short") {
		t.Fatalf("short unpack = %v err %v, want nil data too short", interp.GetGlobal("short"), interp.GetGlobal("short_err"))
	}
	assertDialectModeError(t, interp.GetGlobal("bad_mode"), interp.GetGlobal("bad_mode_err"), "binary dialect: unknown mode")

	for name, tc := range map[string]struct {
		source string
		want   string
	}{
		"format parse error": {
			source: `dialect.eval("binary", "", {mode: "size", format: "bytes:nope"})`,
			want:   `binary: invalid field size "nope"`,
		},
		"bad size": {
			source: `dialect.eval("binary", {"abc"}, {mode: "pack", format: "bytes:2"})`,
			want:   "binary dialect: bytes:2 got 3 bytes",
		},
	} {
		interp := New()
		installTestModule(interp, "dialect", TableValue(BuildDialect(HostOptions{}, nil)))
		err := execSourceOnInterp(interp, tc.source)
		if err == nil {
			t.Fatalf("%s returned nil error", name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s error = %q, want containing %q", name, err.Error(), tc.want)
		}
	}
}

func TestDialectINIDecodeAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		cfg := ini`+"`"+`
app = ledger
enabled: true

[database]
host = db.internal
port = 5432
`+"`"+`
		encoded := dialect.eval("ini", {app: "ledger", enabled: true, database: {host: "db.internal", port: 5432}}, {mode: "encode"})
		roundtrip := dialect.eval("ini", encoded)
		bad, bad_err := dialect.eval("ini", "[broken")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	cfg := interp.GetGlobal("cfg").Table()
	if got := cfg.RawGetString("app").Str(); got != "ledger" {
		t.Fatalf("ini app = %q, want ledger", got)
	}
	if got := cfg.RawGetString("enabled").Str(); got != "true" {
		t.Fatalf("ini enabled = %q, want true", got)
	}
	db := cfg.RawGetString("database").Table()
	if got := db.RawGetString("host").Str(); got != "db.internal" {
		t.Fatalf("ini database.host = %q, want db.internal", got)
	}
	if got := db.RawGetString("port").Str(); got != "5432" {
		t.Fatalf("ini database.port = %q, want 5432", got)
	}
	wantEncoded := "app=ledger\nenabled=true\n\n[database]\nhost=db.internal\nport=5432\n"
	if got := interp.GetGlobal("encoded").Str(); got != wantEncoded {
		t.Fatalf("encoded ini = %q, want %q", got, wantEncoded)
	}
	roundtripDB := interp.GetGlobal("roundtrip").Table().RawGetString("database").Table()
	if got := roundtripDB.RawGetString("port").Str(); got != "5432" {
		t.Fatalf("roundtrip database.port = %q, want 5432", got)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad ini result is not nil")
	}
	if got := interp.GetGlobal("bad_err").Str(); !strings.Contains(got, "ini dialect: line 1: malformed section header") {
		t.Fatalf("bad ini error = %q", got)
	}
}

func TestDialectSemVerDecodeAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := semver`+"`"+`1.2.3-rc.1+build.7`+"`"+`
		encoded := dialect.eval("semver", {major: 2, minor: 0, patch: 1, prerelease: {"beta", "2"}, build: {"ci", "0042"}}, {mode: "encode"})
		formatted := dialect.eval("semver", {major: 3, minor: 4, patch: 5, pre: "alpha.1", build_metadata: "sha.abcdef"}, {mode: "format"})
		roundtrip := dialect.eval("semver", encoded)
		bad, bad_err := dialect.eval("semver", "1.02.3")
		bad_table, bad_table_err := dialect.eval("semver", {major: 1, minor: 2, patch: 3, prerelease: {"01"}}, {mode: "encode"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("major").Int(); got != 1 {
		t.Fatalf("semver major = %d, want 1", got)
	}
	if got := parsed.RawGetString("minor").Int(); got != 2 {
		t.Fatalf("semver minor = %d, want 2", got)
	}
	if got := parsed.RawGetString("patch").Int(); got != 3 {
		t.Fatalf("semver patch = %d, want 3", got)
	}
	if got := parsed.RawGetString("prerelease").Table().RawGetInt(1).Str(); got != "rc" {
		t.Fatalf("semver prerelease[1] = %q, want rc", got)
	}
	if got := parsed.RawGetString("prerelease").Table().RawGetInt(2).Str(); got != "1" {
		t.Fatalf("semver prerelease[2] = %q, want 1", got)
	}
	if got := parsed.RawGetString("build").Table().RawGetInt(2).Str(); got != "7" {
		t.Fatalf("semver build[2] = %q, want 7", got)
	}
	if got := parsed.RawGetString("version").Str(); got != "1.2.3-rc.1+build.7" {
		t.Fatalf("semver version = %q", got)
	}
	if got := interp.GetGlobal("encoded").Str(); got != "2.0.1-beta.2+ci.0042" {
		t.Fatalf("encoded semver = %q", got)
	}
	if got := interp.GetGlobal("formatted").Str(); got != "3.4.5-alpha.1+sha.abcdef" {
		t.Fatalf("formatted semver = %q", got)
	}
	if got := interp.GetGlobal("roundtrip").Table().RawGetString("pre").Str(); got != "beta.2" {
		t.Fatalf("roundtrip pre = %q", got)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad semver returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err").Str(); !strings.Contains(got, "semver dialect: minor number has leading zero") {
		t.Fatalf("bad_err = %q", got)
	}
	if !interp.GetGlobal("bad_table").IsNil() {
		t.Fatalf("bad semver table returned non-nil result")
	}
	if got := interp.GetGlobal("bad_table_err").Str(); !strings.Contains(got, "semver dialect: prerelease numeric identifier has leading zero") {
		t.Fatalf("bad_table_err = %q", got)
	}
}

func TestDialectMarkdownTableDecodeAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		rows := mdtable`+"`"+`
| Name | Score | Note |
| --- | ---: | --- |
| Ada | 42 | uses \| safely |
| Bob | 7 |
`+"`"+`
		encoded := dialect.eval("mdtable", rows, {mode: "encode"})
		custom := dialect.eval("mdtable", {{name: "Ada", score: 42}}, {mode: "encode", headers: {"name", "score"}})
		bad, bad_err := dialect.eval("mdtable", "| a | b |\n| -- | --- |\n")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	rows := interp.GetGlobal("rows").Table()
	if got := rows.Length(); got != 2 {
		t.Fatalf("rows length = %d, want 2", got)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("Name").Str(); got != "Ada" {
		t.Fatalf("row 1 name = %q, want Ada", got)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("Note").Str(); got != "uses | safely" {
		t.Fatalf("row 1 note = %q, want escaped pipe decoded", got)
	}
	if got := rows.RawGetInt(2).Table().RawGetString("Note").Str(); got != "" {
		t.Fatalf("row 2 note = %q, want empty missing cell", got)
	}
	wantEncoded := "| Name | Score | Note |\n| --- | --- | --- |\n| Ada | 42 | uses \\| safely |\n| Bob | 7 |  |\n"
	if got := interp.GetGlobal("encoded").Str(); got != wantEncoded {
		t.Fatalf("encoded = %q, want %q", got, wantEncoded)
	}
	if got := interp.GetGlobal("custom").Str(); got != "| name | score |\n| --- | --- |\n| Ada | 42 |\n" {
		t.Fatalf("custom encoded = %q", got)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad mdtable returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err").Str(); !strings.Contains(got, "mdtable dialect: delimiter cells must contain at least three hyphens") {
		t.Fatalf("bad_err = %q", got)
	}
}

func TestDialectDurationParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := duration`+"`"+`1h30m250ms`+"`"+`
		encoded_seconds := dialect.eval("duration", 90.25, {mode: "encode"})
		encoded_millis := dialect.eval("duration", {milliseconds: 250}, {mode: "encode"})
		encoded_roundtrip := dialect.eval("duration", parsed, {mode: "encode"})
		bad, bad_err := dialect.eval("duration", "not-duration")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	parsed := interp.GetGlobal("parsed").Table()
	if got := parsed.RawGetString("text").Str(); got != "1h30m0.25s" {
		t.Fatalf("duration text = %q", got)
	}
	if got := parsed.RawGetString("nanoseconds").Int(); got != 5_400_250_000_000 {
		t.Fatalf("duration nanoseconds = %d", got)
	}
	if got := parsed.RawGetString("seconds").Number(); got != 5400.25 {
		t.Fatalf("duration seconds = %v", got)
	}
	if got := parsed.RawGetString("milliseconds").Number(); got != 5_400_250 {
		t.Fatalf("duration milliseconds = %v", got)
	}
	if got := interp.GetGlobal("encoded_seconds").Str(); got != "1m30.25s" {
		t.Fatalf("encoded seconds = %q", got)
	}
	if got := interp.GetGlobal("encoded_millis").Str(); got != "250ms" {
		t.Fatalf("encoded milliseconds = %q", got)
	}
	if got := interp.GetGlobal("encoded_roundtrip").Str(); got != "1h30m0.25s" {
		t.Fatalf("encoded roundtrip = %q", got)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad duration returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err").Str(); !strings.Contains(got, "duration dialect:") {
		t.Fatalf("bad duration error = %q", got)
	}
}

func TestDialectTAPParseAndEncode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := tap`+"`"+`TAP version 13
1..2
ok 1 - boot
not ok 2 - deploy # TODO flaky
# expected ready
`+"`"+`
		encoded := dialect.eval("tap", parsed, {mode: "encode"})
		bad, bad_err := dialect.eval("tap", "not tap")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	rows := interp.GetGlobal("parsed").Table()
	if got := rows.RawGetInt(1).Table().RawGetString("version").Int(); got != 13 {
		t.Fatalf("tap version = %d", got)
	}
	if got := rows.RawGetInt(2).Table().RawGetString("last").Int(); got != 2 {
		t.Fatalf("tap plan last = %d", got)
	}
	second := rows.RawGetInt(4).Table()
	if got := second.RawGetString("ok").Bool(); got {
		t.Fatalf("tap second ok = true, want false")
	}
	if got := second.RawGetString("directive").Str(); got != "TODO" {
		t.Fatalf("tap directive = %q", got)
	}
	if got := second.RawGetString("diagnostics").Table().RawGetInt(1).Str(); got != "expected ready" {
		t.Fatalf("tap diagnostic = %q", got)
	}
	wantEncoded := "TAP version 13\n1..2\nok 1 - boot\nnot ok 2 - deploy # TODO flaky\n# expected ready\n"
	if got := interp.GetGlobal("encoded").Str(); got != wantEncoded {
		t.Fatalf("encoded tap = %q, want %q", got, wantEncoded)
	}
	if !interp.GetGlobal("bad").IsNil() {
		t.Fatalf("bad tap returned non-nil result")
	}
	if got := interp.GetGlobal("bad_err").Str(); !strings.Contains(got, "tap dialect: line 1: unrecognized TAP line") {
		t.Fatalf("bad tap error = %q", got)
	}
}

func stringSetFromArray(tbl *Table) map[string]bool {
	out := make(map[string]bool)
	for i := 1; i <= tbl.Length(); i++ {
		out[tbl.RawGetInt(int64(i)).Str()] = true
	}
	return out
}

func stringSliceFromArray(tbl *Table) []string {
	out := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		out = append(out, tbl.RawGetInt(int64(i)).Str())
	}
	return out
}
