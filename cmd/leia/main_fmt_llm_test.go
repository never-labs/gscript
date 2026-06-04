package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFmtStdinLLMStdlibIndentation(t *testing.T) {
	src := `lookup := llm.tool("lookup", func(query) {
return "found:" .. query, nil
}, {
description: "Lookup docs."
params: {"query"}
})

llm.register_models({
default: "fast"
fast: {provider_model: "mock-fast"}
})

answer := llm.agent("answer", func(q) {
return {user: q}, nil
}, {
model: "fast"
system: "Use the tool."
tools: {lookup}
})

result, err := llm.turn({
messages: {llm.user("one-shot")}
tools: {lookup}
model: "fast"
})
_ = answer
_ = result
_ = err
`
	want := `lookup := llm.tool("lookup", func(query) {
    return "found:" .. query, nil
}, {
    description: "Lookup docs."
    params: {"query"}
})

llm.register_models({
    default: "fast"
    fast: {provider_model: "mock-fast"}
})

answer := llm.agent("answer", func(q) {
    return {user: q}, nil
}, {
    model: "fast"
    system: "Use the tool."
    tools: {lookup}
})

result, err := llm.turn({
    messages: {llm.user("one-shot")}
    tools: {lookup}
    model: "fast"
})
_ = answer
_ = result
_ = err
`

	oldStdin := cliStdin
	cliStdin = strings.NewReader(src)
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "llm.leia"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtStdinPreservesCommentOnlyLines(t *testing.T) {
	src := `answer := llm.agent("sample", func(q) {
// keep this note

if true {
// nested
print(q)
}
return {user: q}, nil
})
`
	want := `answer := llm.agent("sample", func(q) {
    // keep this note

    if true {
        // nested
        print(q)
    }
    return {user: q}, nil
})
`

	oldStdin := cliStdin
	cliStdin = strings.NewReader(src)
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "comments.leia"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtPreservesIntraLineFormattingBoundary(t *testing.T) {
	src := `// lookup searches project docs.
lookup := llm.tool("lookup", func(query) {
return "found:"..query,nil
}, {params:{"query"}})
llm.register_models({
short: "x"
longer_key : {provider_model:"mock-fast"}
})
cfg := {short:1, longer_key : 2}
total:=1+  2
`
	want := `// lookup searches project docs.
lookup := llm.tool("lookup", func(query) {
    return "found:"..query,nil
}, {params:{"query"}})
llm.register_models({
    short: "x"
    longer_key : {provider_model:"mock-fast"}
})
cfg := {short:1, longer_key : 2}
total:=1+  2
`

	formatted, err := formatSource("boundary.leia", []byte(src))
	if err != nil {
		t.Fatalf("formatSource: %v", err)
	}
	if got := string(formatted); got != want {
		t.Fatalf("formatted = %q, want %q", got, want)
	}
}

func llmToolchainCoverageSource() []byte {
	return []byte(`import "json"
import p "path"
import (
    "regexp"
    fs "fs"
)

lookup := llm.tool("lookup", func(query) {
    return "found:" .. query, nil
}, {
    description: "Lookup docs."
    requires: {"docs.read"}
    params: {"query"}
})

llm.register_models({
    default: "fast"
    fast: {provider_model: "mock-fast"}
})

extractor := llm.agent("extractor", func(topic) {
    return {
        model: "fast"
        system: "Return JSON."
        user: topic
        output: {summary: "example"}
    }, nil
})

delegate := llm.toolof(extractor, {
    name: "delegate"
    description: "Delegate extraction."
})

supervisor := llm.agent("supervisor", func(topic) {
    call := {id: "call_1", tool: "lookup", args: {query: topic}}
    msgs := {
        llm.system("Return concise JSON."),
        llm.user(topic),
        msg.assistant_call(call),
        msg.tool_result("call_1", {summary: "docs"}),
    }
    tool_msg, tool_idx := history.find(msgs, {role: "tool"})
    assistant_msg, assistant_idx := history.last(msgs, {role: "assistant"})
    all_users := history.find_all(msgs, {role: "user"})
    history.append(msgs, msg.user("Summarize."))
    ok, ok_msg := llm.validate_output({summary: "docs"}, {summary: "example"})
    _ = tool_msg
    _ = tool_idx
    _ = assistant_msg
    _ = assistant_idx
    _ = all_users
    _ = ok
    _ = ok_msg
    return {
        messages: msgs
        tools: {lookup, delegate}
        model: "fast"
    }, nil
})

answer, answer_err := supervisor("leia")
direct, direct_err := llm.turn({
    messages: {llm.user("one-shot")}
    model: "fast"
})
shell := $` + "`printf leia`" + `
raw_csv := csv!` + "`a,b\\n1,2\\n`" + `
raw_prompt := prompt! { role: "system" }
_ = answer
_ = answer_err
_ = direct
_ = direct_err
_ = shell
_ = raw_csv
_ = raw_prompt
`)
}

func TestFmtLLMSyntaxCoverage(t *testing.T) {
	formatted, err := formatSource("llm.leia", llmToolchainCoverageSource())
	if err != nil {
		t.Fatalf("formatSource: %v", err)
	}
	for _, want := range []string{
		"tools: {lookup, delegate}",
		"msg.assistant_call(call)",
		"msg.tool_result(\"call_1\", {summary: \"docs\"})",
		"history.find(msgs, {role: \"tool\"})",
		"history.find_all(msgs, {role: \"user\"})",
		"llm.validate_output({summary: \"docs\"}, {summary: \"example\"})",
		"import p \"path\"",
		"shell := $`printf leia`",
		"raw_csv := csv!`a,b\\n1,2\\n`",
		"raw_prompt := prompt! { role: \"system\" }",
	} {
		if !strings.Contains(string(formatted), want) {
			t.Fatalf("formatted LLM source missing %q:\n%s", want, formatted)
		}
	}
	if strings.Contains(string(formatted), "}  \n") {
		t.Fatalf("formatted source still contains trailing spaces: %q", string(formatted))
	}
	if !strings.HasSuffix(string(formatted), "\n") {
		t.Fatalf("formatted source does not end with newline: %q", string(formatted))
	}
	formattedAgain, err := formatSource("llm.leia", formatted)
	if err != nil {
		t.Fatalf("format formatted source: %v", err)
	}
	if !bytes.Equal(formattedAgain, formatted) {
		t.Fatalf("LLM formatting is not idempotent:\nonce:\n%s\ntwice:\n%s", formatted, formattedAgain)
	}
}

func TestFmtStdinRejectsPathArguments(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "scratch.leia", "file.leia"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runFmtCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--stdin-file-name cannot be used with path arguments") {
		t.Fatalf("stderr = %q, want stdin/path diagnostic", stderr.String())
	}
}

func TestFmtRefusesSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.leia")
	original := []byte("func {\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "parse error") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed after parse failure: %q", string(got))
	}
}
