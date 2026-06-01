package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFmtStdinLLMIndentation(t *testing.T) {
	src := `tool lookup(query) {
return "found:" .. query, nil
}
models {
default: "fast"
fast: {provider_model: "mock-fast"}
}
agent defaults {
model: "fast"
tools: [lookup]
budget: {turns: 2, calls: 4, tokens: 1000, time: 30s}
}
agent researcher(topic) {
system: "Use the tool."
user: topic
tools: [lookup]
} flow {
history := messages {
system: system
user: topic
}
result, err := turn {
messages: history
tools: tools
model: model
}
return result, err
}
answer := agent(q) {
user: q
}
budget { turns: 1 } {
direct, direct_err := turn {
messages: messages { user: "one-shot" }
}
_ = direct
_ = direct_err
}
`
	want := `tool lookup(query) {
    return "found:" .. query, nil
}
models {
    default: "fast"
    fast: {provider_model: "mock-fast"}
}
agent defaults {
    model: "fast"
    tools: [lookup]
    budget: {turns: 2, calls: 4, tokens: 1000, time: 30s}
}
agent researcher(topic) {
    system: "Use the tool."
    user: topic
    tools: [lookup]
} flow {
    history := messages {
        system: system
        user: topic
    }
    result, err := turn {
        messages: history
        tools: tools
        model: model
    }
    return result, err
}
answer := agent(q) {
    user: q
}
budget { turns: 1 } {
    direct, direct_err := turn {
        messages: messages { user: "one-shot" }
    }
    _ = direct
    _ = direct_err
}
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
	src := `agent sample() {
// keep this note

user: "hello"
} flow {
if true {
// nested
print("ok")
}
}
`
	want := `agent sample() {
    // keep this note

    user: "hello"
} flow {
    if true {
        // nested
        print("ok")
    }
}
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
//leia:requires docs.read
tool lookup(query) {
return "found:"..query,nil
}
models {
short: "x"
longer_key : {provider_model:"mock-fast"}
}
cfg := {short:1, longer_key : 2}
total:=1+  2
`
	want := `// lookup searches project docs.
//leia:requires docs.read
tool lookup(query) {
    return "found:"..query,nil
}
models {
    short: "x"
    longer_key : {provider_model:"mock-fast"}
}
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
	return []byte(`// lookup searches project docs.
//leia:requires docs.read
//leia:param query search query
tool lookup(query) {
    return "found:" .. query, nil
}

models {
    default: "fast"
    fast: {provider_model: "mock-fast"}
}

agent extractor(topic) {
    model: "fast"
    system: "Return JSON."
    user: topic
    output: {summary: "example"}
}

delegate := toolof(extractor, {
    name: "delegate"
    description: "Delegate extraction."
})

agent supervisor(topic) {
    model: "fast"
    tools: [extractor, delegate, lookup]
    user: topic
} flow {
    call := {id: "call_1", tool: "lookup", args: {query: topic}}
    msgs := messages {
        system: system
        user: topic
        msg.assistant_call(call)
        msg.tool_result("call_1", {summary: "docs"})
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
    return turn {
        messages: msgs
        tools: tools
        model: model
    }
}

answer, answer_err := supervisor("leia")
_ = answer
_ = answer_err
`)
}

func TestFmtLLMSyntaxCoverage(t *testing.T) {
	formatted, err := formatSource("llm.leia", llmToolchainCoverageSource())
	if err != nil {
		t.Fatalf("formatSource: %v", err)
	}
	for _, want := range []string{
		"tools: [extractor, delegate, lookup]",
		"msg.assistant_call(call)",
		"msg.tool_result(\"call_1\", {summary: \"docs\"})",
		"history.find(msgs, {role: \"tool\"})",
		"history.find_all(msgs, {role: \"user\"})",
		"llm.validate_output({summary: \"docs\"}, {summary: \"example\"})",
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
