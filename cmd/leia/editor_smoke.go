package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/catalog"
)

type editorSmokePattern map[string]any

func runEditorSmokeCommand(args []string, outw, errw io.Writer) int {
	if len(args) != 0 {
		if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
			fmt.Fprintln(outw, "usage: leia editor smoke")
			return 0
		}
		fmt.Fprintln(errw, "usage: leia editor smoke")
		return 2
	}
	root, err := findCLIRepoRootFromCWD()
	if err != nil {
		fmt.Fprintf(errw, "leia editor smoke: %v\n", err)
		return 1
	}
	smoke := editorSmoke{root: root}
	if err := smoke.run(); err != nil {
		fmt.Fprintf(errw, "leia editor smoke: %v\n", err)
		return 1
	}
	fmt.Fprintln(outw, "leia editor smoke: ok")
	return 0
}

type editorSmoke struct {
	root string
}

func (s editorSmoke) run() error {
	for _, check := range []func() error{
		s.checkTextMate,
		s.checkVSCode,
		s.checkEmacs,
		s.checkTreeSitterAssets,
		s.checkPackagedEditorIntegrations,
		s.checkDownstreamDocs,
	} {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func (s editorSmoke) path(rel string) string {
	return filepath.Join(s.root, filepath.FromSlash(rel))
}

func (s editorSmoke) read(rel string) (string, error) {
	data, err := os.ReadFile(s.path(rel))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s editorSmoke) loadJSON(rel string) (map[string]any, error) {
	data, err := os.ReadFile(s.path(rel))
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", rel, err)
	}
	return value, nil
}

func (s editorSmoke) loadJSONAny(rel string) (any, error) {
	data, err := os.ReadFile(s.path(rel))
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", rel, err)
	}
	return value, nil
}

func (s editorSmoke) assertPath(rel string) error {
	if !fileExists(s.path(rel)) {
		return fmt.Errorf("missing path %s", rel)
	}
	return nil
}

func (s editorSmoke) checkTextMate() error {
	leia, err := s.loadJSON("tools/syntax/textmate/leia.tmLanguage.json")
	if err != nil {
		return err
	}
	leiaMod, err := s.loadJSON("tools/syntax/textmate/leia-mod.tmLanguage.json")
	if err != nil {
		return err
	}
	vscodeLeia, err := s.loadJSON("editors/vscode/syntaxes/leia.tmLanguage.json")
	if err != nil {
		return err
	}
	vscodeLeiaMod, err := s.loadJSON("editors/vscode/syntaxes/leia-mod.tmLanguage.json")
	if err != nil {
		return err
	}
	if _, err := s.loadJSON("editors/vscode/language-configuration.json"); err != nil {
		return err
	}
	source, err := s.read("tools/editor/smoke/fixtures/llm_surface.leia")
	if err != nil {
		return err
	}
	manifest, err := s.read("tools/editor/smoke/fixtures/leia.mod")
	if err != nil {
		return err
	}
	if leia["scopeName"] != "source.leia" {
		return errors.New("Leia TextMate grammar has the wrong scope")
	}
	if leiaMod["scopeName"] != "source.leia.mod" {
		return errors.New("leia.mod TextMate grammar has the wrong scope")
	}
	if !reflect.DeepEqual(vscodeLeia, leia) {
		return errors.New("VS Code Leia TextMate grammar drifted from tools/syntax/textmate/leia.tmLanguage.json")
	}
	if !reflect.DeepEqual(vscodeLeiaMod, leiaMod) {
		return errors.New("VS Code leia.mod TextMate grammar drifted from tools/syntax/textmate/leia-mod.tmLanguage.json")
	}

	for _, tc := range []struct {
		name   string
		sample string
	}{
		{"keyword.control.directive.leia", source},
		{"storage.type.function.leia", source},
		{"meta.import.leia", `import "go:net/http" as http`},
		{"meta.import.leia", `import "json"`},
		{"meta.import.leia", `import p "path"`},
		{"meta.import.group.leia", "import (\n  \"regexp\"\n  fs \"fs\"\n)"},
		{"meta.dialect.tagged-string.leia", "rows := csv`a,b\\n1,2\\n`"},
		{"meta.dialect.tagged-string.leia", "rows := csv!`a,b\\n1,2\\n`"},
		{"meta.dialect.tagged-string.leia", "rows := custom`domain source`"},
		{"meta.dialect.shell.tagged-string.leia", "out := $`printf ok`"},
		{"meta.dialect.shell.tagged-string.leia", "out := $!`printf ok`"},
		{"meta.dialect.tagged-block.leia", `prompt! { role: "system" }`},
		{"support.type.primitive.leia", source},
		{"support.type.primitive.leia", "ids := [3]i64{1, 2, 3}"},
		{"constant.numeric.duration.leia", "timeout := 30s"},
		{"keyword.operator.leia", source},
		{"entity.name.function.leia", source},
		{"support.module.leia", "joined := bytes.concat(parts)"},
		{"keyword.control.leia-mod", manifest},
		{"keyword.operator.arrow.leia-mod", manifest},
		{"constant.other.version.leia-mod", manifest},
		{"string.unquoted.path.leia-mod", manifest},
	} {
		if err := assertEditorPatternMatch(tc.name, leiaPatternGrammar(tc.name, leia, leiaMod), tc.sample); err != nil {
			return err
		}
	}
	for _, tc := range []struct {
		name   string
		sample string
	}{
		{"keyword.control.import.leia", `import "go:net/http" as http`},
		{"string.quoted.double.import.path.leia", `import "go:net/http" as http`},
		{"keyword.control.import.as.leia", `import "go:net/http" as http`},
		{"entity.name.namespace.import.leia", `import "go:net/http" as http`},
		{"entity.name.namespace.import.leia", `import p "path"`},
		{"entity.name.tag.dialect.leia", "rows := csv`a,b\\n1,2\\n`"},
		{"keyword.operator.raw.dialect.leia", "rows := csv!`a,b\\n1,2\\n`"},
		{"entity.name.tag.shell.leia", "out := $`printf ok`"},
		{"keyword.operator.raw.dialect.leia", "out := $!`printf ok`"},
	} {
		if err := assertEditorAnyPatternMatch(leia, tc.name, tc.sample); err != nil {
			return err
		}
	}
	if hasEditorPattern(leia, "keyword.control.ai.leia") {
		return errors.New("Leia TextMate grammar still exposes old AI dialect keyword scope")
	}
	for _, removed := range []string{
		"meta.dialect.q.tagged-string.leia",
		"entity.name.tag.dialect.q.leia",
		"keyword.control.qsql.q.leia",
		"support.function.q.leia",
		"constant.other.symbol.q.leia",
	} {
		if hasEditorPattern(leia, removed) {
			return fmt.Errorf("Leia TextMate grammar still exposes q-specific scope %s", removed)
		}
	}
	for _, unsupported := range []string{"i8", "i16", "u8", "u16", "u32", "u64"} {
		if err := assertEditorPatternNoMatch(leia, "support.type.primitive.leia", fmt.Sprintf("ids := [3]%s{1, 2, 3}", unsupported)); err != nil {
			return err
		}
	}
	operator, err := editorPatternByName(leia, "keyword.operator.leia")
	if err != nil {
		return err
	}
	if strings.Contains(fmt.Sprint(operator["match"]), "%=") {
		return errors.New("Leia TextMate grammar still exposes unsupported %= compound assignment")
	}
	return nil
}

func leiaPatternGrammar(name string, leia, leiaMod map[string]any) map[string]any {
	if strings.HasSuffix(name, ".leia-mod") {
		return leiaMod
	}
	return leia
}

func (s editorSmoke) checkVSCode() error {
	packageJSON, err := s.loadJSON("editors/vscode/package.json")
	if err != nil {
		return err
	}
	contributes, _ := packageJSON["contributes"].(map[string]any)
	languages := map[string]map[string]any{}
	for _, item := range anySlice(contributes["languages"]) {
		if m, ok := item.(map[string]any); ok {
			languages[stringValue(m["id"])] = m
		}
	}
	grammars := map[string]map[string]any{}
	for _, item := range anySlice(contributes["grammars"]) {
		if m, ok := item.(map[string]any); ok {
			grammars[stringValue(m["language"])] = m
		}
	}
	commands := map[string]bool{}
	for _, item := range anySlice(contributes["commands"]) {
		if m, ok := item.(map[string]any); ok {
			commands[stringValue(m["command"])] = true
		}
	}
	config := map[string]any{}
	if cfg, ok := contributes["configuration"].(map[string]any); ok {
		if props, ok := cfg["properties"].(map[string]any); ok {
			config = props
		}
	}
	for _, language := range []string{"leia", "leia-mod"} {
		if languages[language] == nil {
			return fmt.Errorf("VS Code package does not contribute %s", language)
		}
		grammar := grammars[language]
		if grammar == nil {
			return fmt.Errorf("VS Code package does not wire grammar for %s", language)
		}
		if err := s.assertPath("editors/vscode/" + strings.TrimPrefix(stringValue(grammar["path"]), "./")); err != nil {
			return err
		}
	}
	for _, command := range []string{"leia.runFile", "leia.testWorkspace", "leia.formatFile", "leia.lintWorkspace", "leia.checkWorkspace", "leia.previewSpec", "leia.restartLanguageServer", "leia.evaluate.case"} {
		if !commands[command] {
			return fmt.Errorf("VS Code package is missing command %s", command)
		}
	}
	if commands["leia.agent.run"] {
		return errors.New("VS Code package still exposes old agent run command")
	}
	for _, setting := range []string{"leia.languageServer.enabled", "leia.languageServer.executable"} {
		if _, ok := config[setting]; !ok {
			return fmt.Errorf("VS Code package is missing setting %s", setting)
		}
	}
	extension, err := s.read("editors/vscode/extension.js")
	if err != nil {
		return err
	}
	if err := assertJSArrayLiteral(extension, "semanticTokenTypes", []string{"keyword", "variable", "function", "method", "string", "number", "operator", "type", "parameter", "property", "namespace"}); err != nil {
		return err
	}
	if err := assertJSArrayLiteral(extension, "semanticTokenModifiers", []string{"declaration", "readonly", "defaultLibrary", "import", "dialect"}); err != nil {
		return err
	}
	for _, marker := range []string{"startLanguageServer(context)", "textDocument/definition", "textDocument/references", "textDocument/prepareRename", "textDocument/rename", "workspace/symbol", "textDocument/codeLens", "textDocument/inlayHint", "textDocument/documentLink", "textDocument/semanticTokens/full", "textDocument/publishDiagnostics", "restartLanguageServer", "failPending"} {
		if !strings.Contains(extension, marker) {
			return fmt.Errorf("VS Code extension missing LSP marker %s", marker)
		}
	}
	snippets, err := s.loadJSON("editors/vscode/snippets/leia.json")
	if err != nil {
		return err
	}
	for _, key := range []string{"function", "llm agent", "llm tool", "llm turn", "test", "go routine"} {
		if _, ok := snippets[key]; !ok {
			return fmt.Errorf("VS Code snippets missing %s", key)
		}
	}
	for _, key := range []string{"agent", "tool", "turn"} {
		if _, ok := snippets[key]; ok {
			return fmt.Errorf("VS Code snippets still expose old %s block snippet", key)
		}
	}
	return nil
}

func (s editorSmoke) checkEmacs() error {
	mode, err := s.read("editors/emacs/leia-mode.el")
	if err != nil {
		return err
	}
	modules, err := elispStringList(mode, "leia--modules")
	if err != nil {
		return err
	}
	expected := catalogModuleNames()
	if !slices.Equal(modules, expected) {
		return fmt.Errorf("Emacs leia--modules drifted from stdlib catalog: got %v, want %v", modules, expected)
	}
	for _, constName := range []string{"leia--keywords", "leia--declarations", "leia--builtins", "leia--primitive-types"} {
		values, err := elispStringList(mode, constName)
		if err != nil {
			return err
		}
		if len(values) == 0 {
			return fmt.Errorf("Emacs %s must not be empty", constName)
		}
	}
	for _, marker := range []string{"defcustom leia-lsp-command", "defun leia-eglot-setup", "(require 'eglot)", "eglot-server-programs", "leia-lsp-command", "add-to-list 'auto-mode-alist", "(provide 'leia-mode)"} {
		if !strings.Contains(mode, marker) {
			return fmt.Errorf("Emacs mode missing %s", marker)
		}
	}
	for _, oldKeyword := range []string{"agent", "tool", "evaluate", "models", "messages", "budget"} {
		for _, constName := range []string{"leia--keywords", "leia--declarations", "leia--contextual-keywords"} {
			values, err := elispStringList(mode, constName)
			if err != nil {
				return err
			}
			if slices.Contains(values, oldKeyword) {
				return fmt.Errorf("Emacs %s still exposes old AI dialect keyword %s", constName, oldKeyword)
			}
		}
	}
	return nil
}

func (s editorSmoke) checkTreeSitterAssets() error {
	packageJSON, err := s.loadJSON("tools/tree-sitter-leia/package.json")
	if err != nil {
		return err
	}
	config, err := s.loadJSON("tools/tree-sitter-leia/tree-sitter.json")
	if err != nil {
		return err
	}
	if _, err := s.loadJSONAny("tools/tree-sitter-leia/src/grammar.json"); err != nil {
		return err
	}
	nodeTypesValue, err := s.loadJSONAny("tools/tree-sitter-leia/src/node-types.json")
	if err != nil {
		return err
	}
	grammarJS, err := s.read("tools/tree-sitter-leia/grammar.js")
	if err != nil {
		return err
	}
	packageEntry := firstMap(anySlice(packageJSON["tree-sitter"]))
	if packageEntry["scope"] != "source.leia" || !stringSliceEqual(anyStringSlice(packageEntry["file-types"]), []string{"leia"}) || packageEntry["grammar"] != "leia" {
		return errors.New("tree-sitter package metadata has invalid scope, file types, or grammar name")
	}
	grammarEntry := firstMap(anySlice(config["grammars"]))
	if grammarEntry["name"] != "leia" || grammarEntry["scope"] != "source.leia" || !stringSliceEqual(anyStringSlice(grammarEntry["file-types"]), []string{"leia"}) {
		return errors.New("tree-sitter.json has invalid grammar name, scope, or file types")
	}
	namedTypes := map[string]bool{}
	for _, item := range anySlice(nodeTypesValue) {
		if m, ok := item.(map[string]any); ok && boolValue(m["named"]) {
			namedTypes[stringValue(m["type"])] = true
		}
	}
	for _, nodeType := range []string{"import_declaration", "import_group", "import_spec", "dense_literal", "tagged_string_expression", "tagged_block_expression"} {
		if !namedTypes[nodeType] {
			return fmt.Errorf("tree-sitter node-types missing %s", nodeType)
		}
	}
	for _, oldNodeType := range []string{"agent_declaration", "agent_defaults_declaration", "agent_literal", "budget_statement", "evaluate_block", "message_field", "messages_expression", "models_declaration", "tool_declaration", "turn_expression"} {
		if namedTypes[oldNodeType] {
			return fmt.Errorf("tree-sitter node-types still expose old AI dialect node %s", oldNodeType)
		}
	}
	query, err := s.read("tools/tree-sitter-leia/queries/highlights.scm")
	if err != nil {
		return err
	}
	for _, editorQuery := range []string{"editors/neovim/queries/leia/highlights.scm", "editors/helix/queries/leia/highlights.scm", "editors/zed/languages/leia/highlights.scm"} {
		contents, err := s.read(editorQuery)
		if err != nil {
			return err
		}
		if contents != query {
			return fmt.Errorf("%s drifted from tools/tree-sitter-leia/queries/highlights.scm", editorQuery)
		}
	}
	for _, marker := range []string{"@keyword.control", "@function.call", "@variable.parameter", "@tag.dialect", "@tag.shell", "@operator.raw.dialect", "@string.special.dialect", "@string.special.shell", "@keyword.control.import", "@keyword.control.import.as", "@string.special.import", "@namespace.import"} {
		if !strings.Contains(query, marker) {
			return fmt.Errorf("tree-sitter highlight query missing %s", marker)
		}
	}
	for _, marker := range []string{"@tag)", "@namespace)", "\"import\"\n] @keyword.function", "\"as\"\n] @keyword"} {
		if strings.Contains(query, marker) {
			return fmt.Errorf("tree-sitter highlight query still uses stale generic marker %s", marker)
		}
	}
	for _, marker := range []string{"(evaluate_block", "(agent_declaration", "(tool_declaration", "(message_field"} {
		if strings.Contains(query, marker) {
			return fmt.Errorf("tree-sitter highlight query still references old AI dialect marker %s", marker)
		}
	}
	for _, unsupported := range []string{"\"%=\"", "\"i8\"", "\"i16\"", "\"u8\"", "\"u16\"", "\"u32\"", "\"u64\""} {
		if strings.Contains(grammarJS, unsupported) || strings.Contains(query, unsupported) {
			return fmt.Errorf("tree-sitter editor assets still expose unsupported token %s", unsupported)
		}
	}
	var corpus strings.Builder
	matches, err := filepath.Glob(s.path("tools/tree-sitter-leia/corpus/*.txt"))
	if err != nil {
		return err
	}
	slices.Sort(matches)
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		corpus.Write(data)
		corpus.WriteByte('\n')
	}
	for _, marker := range []string{"import \"go:net/http\" as http", "import \"json\"", "import p \"path\"", "fs \"fs\"", "csv!`a,b\\n1,2\\n`", "$!`printf ok`", "prompt! { role:", "(tagged_string_expression", "(tagged_block_expression", "(shell_tag)", "(dialect_bang)"} {
		if !strings.Contains(corpus.String(), marker) {
			return fmt.Errorf("tree-sitter corpus missing import/dialect smoke marker %s", marker)
		}
	}
	return nil
}

func (s editorSmoke) checkPackagedEditorIntegrations() error {
	for _, path := range []string{"editors/neovim/README.md", "editors/neovim/queries/leia/highlights.scm", "editors/helix/languages.toml", "editors/helix/queries/leia/highlights.scm", "editors/zed/extension.toml", "editors/zed/languages/leia/config.toml", "editors/zed/languages/leia/highlights.scm"} {
		if err := s.assertPath(path); err != nil {
			return err
		}
	}
	neovim, err := s.read("editors/neovim/README.md")
	if err != nil {
		return err
	}
	for _, marker := range []string{"parser_config.leia", "tools/tree-sitter-leia", "queries/leia"} {
		if !strings.Contains(neovim, marker) {
			return fmt.Errorf("Neovim README missing %s", marker)
		}
	}
	helix, err := s.read("editors/helix/languages.toml")
	if err != nil {
		return err
	}
	for _, marker := range []string{`name = "leia"`, `language-servers = ["leia-lsp"]`, `source = { path = "../../tools/tree-sitter-leia" }`} {
		if !strings.Contains(helix, marker) {
			return fmt.Errorf("Helix config missing %s", marker)
		}
	}
	zed, err := s.read("editors/zed/extension.toml")
	if err != nil {
		return err
	}
	for _, marker := range []string{`id = "leia"`, "[grammars.leia]", "tools/tree-sitter-leia"} {
		if !strings.Contains(zed, marker) {
			return fmt.Errorf("Zed extension manifest missing %s", marker)
		}
	}
	return nil
}

func (s editorSmoke) checkDownstreamDocs() error {
	treeSitterReadme, err := s.read("tools/tree-sitter-leia/README.md")
	if err != nil {
		return err
	}
	emacsReadme, err := s.read("editors/emacs/README.md")
	if err != nil {
		return err
	}
	for _, marker := range []string{"## Downstream Editor Integration", "grammar name: `leia`", "scope: `source.leia`", "### Neovim", "parser_config.leia", "### Helix", "[[grammar]]", "### Zed", "[grammars.leia]", "### Emacs `treesit`", "treesit-language-source-alist"} {
		if !strings.Contains(treeSitterReadme, marker) {
			return fmt.Errorf("tree-sitter README missing downstream marker %s", marker)
		}
	}
	for _, marker := range []string{"## Tree-sitter", "tools/tree-sitter-leia", "treesit-language-source-alist", "source.leia"} {
		if !strings.Contains(emacsReadme, marker) {
			return fmt.Errorf("Emacs README missing tree-sitter marker %s", marker)
		}
	}
	return nil
}

func editorRepositoryPatterns(grammar map[string]any) []editorSmokePattern {
	var patterns []editorSmokePattern
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if _, hasName := v["name"]; hasName {
				if _, hasMatch := v["match"]; hasMatch {
					patterns = append(patterns, editorSmokePattern(v))
				} else if _, hasBegin := v["begin"]; hasBegin {
					patterns = append(patterns, editorSmokePattern(v))
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(grammar["repository"])
	walk(grammar["patterns"])
	return patterns
}

func editorPatternByName(grammar map[string]any, name string) (editorSmokePattern, error) {
	for _, pattern := range editorRepositoryPatterns(grammar) {
		if pattern["name"] == name {
			return pattern, nil
		}
	}
	return nil, fmt.Errorf("missing TextMate pattern %s", name)
}

func hasEditorPattern(grammar map[string]any, name string) bool {
	_, err := editorPatternByName(grammar, name)
	return err == nil
}

func assertEditorPatternMatch(name string, grammar map[string]any, sample string) error {
	pattern, err := editorPatternByName(grammar, name)
	if err != nil {
		return err
	}
	return matchEditorPattern(pattern, name, sample, true)
}

func assertEditorPatternNoMatch(grammar map[string]any, name, sample string) error {
	pattern, err := editorPatternByName(grammar, name)
	if err != nil {
		return err
	}
	err = matchEditorPattern(pattern, name, sample, true)
	if err == nil {
		return fmt.Errorf("pattern %s unexpectedly matched %q", name, sample)
	}
	if strings.Contains(err.Error(), "did not match") {
		return nil
	}
	return err
}

func assertEditorAnyPatternMatch(grammar map[string]any, name, sample string) error {
	for _, pattern := range editorRepositoryPatterns(grammar) {
		captureNames := map[string]bool{}
		for _, capturesKey := range []string{"captures", "beginCaptures", "endCaptures"} {
			if captures, ok := pattern[capturesKey].(map[string]any); ok {
				for _, capture := range captures {
					if m, ok := capture.(map[string]any); ok {
						captureNames[stringValue(m["name"])] = true
					}
				}
			}
		}
		if pattern["name"] != name && !captureNames[name] {
			continue
		}
		if err := matchEditorPattern(pattern, name, sample, false); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no pattern %s matched %q", name, sample)
}

func matchEditorPattern(pattern editorSmokePattern, name, sample string, strict bool) error {
	regex := stringValue(pattern["match"])
	if regex == "" {
		regex = stringValue(pattern["begin"])
	}
	if regex == "" {
		if strict {
			return fmt.Errorf("pattern %s has no regex", name)
		}
		return errors.New("pattern has no regex")
	}
	compiled, err := compileEditorTextMateRegex(regex)
	if err != nil {
		return fmt.Errorf("pattern %s does not compile: %w", name, err)
	}
	if !compiled.MatchString(sample) {
		return fmt.Errorf("pattern %s did not match %q", name, sample)
	}
	return nil
}

func compileEditorTextMateRegex(regex string) (*regexp.Regexp, error) {
	compiled, err := regexp.Compile("(?m)" + regex)
	if err == nil {
		return compiled, nil
	}
	// The editor smoke only needs search semantics. TextMate grammars use a
	// small Oniguruma subset that Go's RE2 does not support, notably positive
	// lookahead for call/module markers. Preserve the required following token
	// in the matched text for those forms.
	compat := strings.ReplaceAll(regex, `(?=\s*\()`, `\s*\(`)
	compat = strings.ReplaceAll(compat, `(?=\.)`, `\.`)
	if compat == regex {
		return nil, err
	}
	return regexp.Compile("(?m)" + compat)
}

func assertJSArrayLiteral(source, constName string, expected []string) error {
	re := regexp.MustCompile(`(?s)const\s+` + regexp.QuoteMeta(constName) + `\s*=\s*(\[[^\]]*\])`)
	match := re.FindStringSubmatch(source)
	if match == nil {
		return fmt.Errorf("VS Code extension missing %s array", constName)
	}
	var value []string
	if err := json.Unmarshal([]byte(match[1]), &value); err != nil {
		return fmt.Errorf("VS Code extension %s array is not JSON-compatible: %w", constName, err)
	}
	if !slices.Equal(value, expected) {
		return fmt.Errorf("VS Code extension %s = %v, want %v", constName, value, expected)
	}
	return nil
}

func elispStringList(source, constName string) ([]string, error) {
	re := regexp.MustCompile(`(?s)\(defconst\s+` + regexp.QuoteMeta(constName) + `\s+'\((.*?)\)\)`)
	match := re.FindStringSubmatch(source)
	if match == nil {
		return nil, fmt.Errorf("Emacs mode missing %s defconst list", constName)
	}
	itemRe := regexp.MustCompile(`"([^"]+)"`)
	matches := itemRe.FindAllStringSubmatch(match[1], -1)
	values := make([]string, 0, len(matches))
	for _, item := range matches {
		values = append(values, item[1])
	}
	return values, nil
}

func catalogModuleNames() []string {
	modules := catalog.Modules()
	names := make([]string, 0, len(modules))
	seen := map[string]bool{}
	for _, module := range modules {
		if seen[module.Name] {
			continue
		}
		seen[module.Name] = true
		names = append(names, module.Name)
	}
	return names
}

func anySlice(value any) []any {
	if out, ok := value.([]any); ok {
		return out
	}
	return nil
}

func anyStringSlice(value any) []string {
	var out []string
	for _, item := range anySlice(value) {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func firstMap(values []any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	if out, ok := values[0].(map[string]any); ok {
		return out
	}
	return map[string]any{}
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func boolValue(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

func stringSliceEqual(got, want []string) bool {
	return slices.Equal(got, want)
}
