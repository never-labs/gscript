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
		"base64", "cmd", "cookie", "cookies", "csv", "duration", "env", "glob",
		"hash", "headers", "html_escape", "http_headers", "httpmsg", "ini", "json", "jsonl",
		"junit", "kv", "lines", "mdtable", "mime", "numbers", "nums", "path", "prompt",
		"quote", "re", "regexp", "semver", "sh", "shellwords", "split", "tap", "template", "tsv", "url",
		"urlpath", "urlquery", "words",
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
		missing_info := dialect.info("missing")
		all_info := dialect.list()
	`, "dialect", BuildDialect(HostOptions{}, nil))

	shInfo := interp.GetGlobal("sh_info").Table()
	if got := shInfo.RawGetString("category").Str(); got != "shell" {
		t.Fatalf("sh category = %q, want shell", got)
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
		roundtrip := dialect.eval("shellwords", encoded)
		bad, bad_err := dialect.eval("shellwords", "'unterminated")
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
