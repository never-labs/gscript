package lsp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/stdlib/catalog"
)

const (
	symbolKindClass    = 5
	symbolKindEvent    = 24
	symbolKindFunction = 12
)

type hoverParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
	Range    lspRange      `json:"range,omitempty"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type documentSymbolParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type documentSymbol struct {
	Name           string   `json:"name"`
	Detail         string   `json:"detail,omitempty"`
	Kind           int      `json:"kind"`
	Range          lspRange `json:"range"`
	SelectionRange lspRange `json:"selectionRange"`
}

type workspaceSymbolParams struct {
	Query string `json:"query,omitempty"`
}

type workspaceSymbolInformation struct {
	Name      string   `json:"name"`
	Kind      int      `json:"kind"`
	Location  location `json:"location"`
	Container string   `json:"containerName,omitempty"`
}

type codeLensParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

type codeLens struct {
	Range   lspRange `json:"range"`
	Command command  `json:"command,omitempty"`
}

type inlayHintParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        lspRange               `json:"range,omitempty"`
}

type inlayHint struct {
	Position position `json:"position"`
	Label    string   `json:"label"`
	Kind     int      `json:"kind,omitempty"`
	Tooltip  string   `json:"tooltip,omitempty"`
}

type documentLinkParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type documentLink struct {
	Range   lspRange `json:"range"`
	Target  string   `json:"target,omitempty"`
	Tooltip string   `json:"tooltip,omitempty"`
}

type sourceSymbol struct {
	Name      string
	Detail    string
	Kind      int
	Range     lspRange
	NameRange lspRange
}

type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type referenceParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
	Context      referenceContext       `json:"context,omitempty"`
}

type referenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration,omitempty"`
}

type renameParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
	NewName      string                 `json:"newName"`
}

type prepareRenameResult struct {
	Range       lspRange `json:"range"`
	Placeholder string   `json:"placeholder"`
}

type location struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type workspaceEdit struct {
	Changes map[string][]textEdit `json:"changes"`
}

func (s *Server) hover(id *json.RawMessage, params json.RawMessage) error {
	var p hoverParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, nil, nil)
	}

	word, wordRange := wordAtPosition(src, p.Position)
	if word == "" {
		return s.respondMaybe(id, nil, nil)
	}
	if value := hoverText(src, word); value != "" {
		return s.respondMaybe(id, hoverResult{
			Contents: markupContent{Kind: "markdown", Value: value},
			Range:    wordRange,
		}, nil)
	}
	return s.respondMaybe(id, nil, nil)
}

func (s *Server) definition(id *json.RawMessage, params json.RawMessage) error {
	var p textDocumentPositionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, nil, nil)
	}
	word, _ := wordAtPosition(src, p.Position)
	if word == "" {
		return s.respondMaybe(id, nil, nil)
	}
	sym, ok := findSourceSymbol(src, word)
	if !ok {
		return s.respondMaybe(id, nil, nil)
	}
	return s.respondMaybe(id, location{URI: p.TextDocument.URI, Range: sym.NameRange}, nil)
}

func (s *Server) references(id *json.RawMessage, params json.RawMessage) error {
	var p referenceParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, []location{}, nil)
	}
	word, _ := wordAtPosition(src, p.Position)
	if word == "" {
		return s.respondMaybe(id, []location{}, nil)
	}
	refs := wordReferences(src, word)
	if !p.Context.IncludeDeclaration {
		if sym, ok := findSourceSymbol(src, word); ok {
			refs = filterReferenceRanges(refs, sym.NameRange)
		}
	}
	out := make([]location, 0, len(refs))
	for _, r := range refs {
		out = append(out, location{URI: p.TextDocument.URI, Range: r})
	}
	return s.respondMaybe(id, out, nil)
}

func (s *Server) prepareRename(id *json.RawMessage, params json.RawMessage) error {
	var p textDocumentPositionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, nil, nil)
	}
	word, wordRange := wordAtPosition(src, p.Position)
	if word == "" || !validIdentifierName(word) {
		return s.respondMaybe(id, nil, nil)
	}
	if len(wordReferences(src, word)) == 0 {
		return s.respondMaybe(id, nil, nil)
	}
	return s.respondMaybe(id, prepareRenameResult{Range: wordRange, Placeholder: word}, nil)
}

func (s *Server) rename(id *json.RawMessage, params json.RawMessage) error {
	var p renameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	if !validIdentifierName(p.NewName) {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: "newName must be a valid Leia identifier"})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, workspaceEdit{Changes: map[string][]textEdit{}}, nil)
	}
	word, _ := wordAtPosition(src, p.Position)
	if word == "" {
		return s.respondMaybe(id, workspaceEdit{Changes: map[string][]textEdit{}}, nil)
	}
	refs := wordReferences(src, word)
	edits := make([]textEdit, 0, len(refs))
	for _, r := range refs {
		edits = append(edits, textEdit{Range: r, NewText: p.NewName})
	}
	return s.respondMaybe(id, workspaceEdit{Changes: map[string][]textEdit{p.TextDocument.URI: edits}}, nil)
}

func (s *Server) documentSymbol(id *json.RawMessage, params json.RawMessage) error {
	var p documentSymbolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, []documentSymbol{}, nil)
	}
	syms := collectSourceSymbols(src)
	out := make([]documentSymbol, 0, len(syms))
	for _, sym := range syms {
		out = append(out, documentSymbol{
			Name:           sym.Name,
			Detail:         sym.Detail,
			Kind:           sym.Kind,
			Range:          sym.Range,
			SelectionRange: sym.NameRange,
		})
	}
	return s.respondMaybe(id, out, nil)
}

func (s *Server) workspaceSymbol(id *json.RawMessage, params json.RawMessage) error {
	var p workspaceSymbolParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
		}
	}
	query := strings.ToLower(strings.TrimSpace(p.Query))
	docs := s.documentSnapshot()
	uris := make([]string, 0, len(docs))
	for uri := range docs {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	var out []workspaceSymbolInformation
	for _, uri := range uris {
		for _, sym := range collectSourceSymbols(docs[uri]) {
			if query != "" && !strings.Contains(strings.ToLower(sym.Name), query) {
				continue
			}
			out = append(out, workspaceSymbolInformation{
				Name: sym.Name,
				Kind: sym.Kind,
				Location: location{
					URI:   uri,
					Range: sym.NameRange,
				},
			})
		}
	}
	return s.respondMaybe(id, out, nil)
}

func (s *Server) codeLens(id *json.RawMessage, params json.RawMessage) error {
	var p codeLensParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, []codeLens{}, nil)
	}
	return s.respondMaybe(id, collectCodeLens(p.TextDocument.URI, src), nil)
}

func (s *Server) inlayHint(id *json.RawMessage, params json.RawMessage) error {
	var p inlayHintParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, []inlayHint{}, nil)
	}
	return s.respondMaybe(id, collectInlayHints(src, p.Range), nil)
}

func (s *Server) documentLink(id *json.RawMessage, params json.RawMessage) error {
	var p documentLinkParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, []documentLink{}, nil)
	}
	return s.respondMaybe(id, collectDocumentLinks(p.TextDocument.URI, src), nil)
}

func (s *Server) documentText(uri string) (string, bool) {
	if uri == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.docs[uri]
	return src, ok
}

func collectDocumentLinks(uri, src string) []documentLink {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil
	}
	var out []documentLink
	for i, tok := range tokens {
		if tok.Type != lexer.TOKEN_STRING || !looksLikeLocalModulePath(tok.Value) {
			continue
		}
		switch {
		case tokenLooksLikeImportPath(tokens, i):
			if target := resolveLocalModuleURI(uri, tok.Value); target != "" {
				out = append(out, documentLink{Range: stringLiteralContentRange(tok), Target: target, Tooltip: "Open imported Leia module"})
			}
		case tokenLooksLikeRequireArg(tokens, i):
			if target := resolveLocalModuleURI(uri, tok.Value); target != "" {
				out = append(out, documentLink{Range: stringLiteralContentRange(tok), Target: target, Tooltip: "Open required Leia module"})
			}
		}
	}
	return out
}

func tokenLooksLikeImportPath(tokens []lexer.Token, index int) bool {
	return index > 0 && tokenText(tokens[index-1]) == "import"
}

func tokenLooksLikeRequireArg(tokens []lexer.Token, index int) bool {
	for i := index - 1; i >= 0 && tokens[i].Line == tokens[index].Line; i-- {
		if tokenText(tokens[i]) == "require" {
			return true
		}
	}
	return false
}

func looksLikeLocalModulePath(path string) bool {
	if path == "" || strings.HasPrefix(path, "go:") || strings.Contains(path, "://") {
		return false
	}
	return strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") || strings.Contains(path, "/") || strings.HasSuffix(path, ".leia")
}

func resolveLocalModuleURI(baseURI, modulePath string) string {
	target := modulePath
	if !filepath.IsAbs(target) {
		basePath := fileURIPath(baseURI)
		if basePath == "" {
			return ""
		}
		target = filepath.Join(filepath.Dir(basePath), modulePath)
	}
	if filepath.Ext(target) == "" {
		target += ".leia"
	}
	return pathToFileURI(filepath.Clean(target))
}

func fileURIPath(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" || parsed.Path == "" {
		return ""
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return ""
	}
	return filepath.FromSlash(path)
}

func pathToFileURI(path string) string {
	if path == "" {
		return ""
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	return u.String()
}

func collectCodeLens(uri, src string) []codeLens {
	syms := collectSourceSymbols(src)
	out := make([]codeLens, 0, len(syms))
	for _, sym := range syms {
		switch sym.Kind {
		case symbolKindEvent:
			out = append(out, codeLens{
				Range: sym.Range,
				Command: command{
					Title:     "Run evaluate case",
					Command:   "leia.evaluate.case",
					Arguments: []any{uri, sym.Name},
				},
			})
		case symbolKindClass:
			out = append(out, codeLens{
				Range: sym.Range,
				Command: command{
					Title:     "Run agent",
					Command:   "leia.agent.run",
					Arguments: []any{uri, sym.Name},
				},
			})
		}
	}
	return out
}

func collectInlayHints(src string, requested lspRange) []inlayHint {
	syms := collectSourceSymbols(src)
	out := make([]inlayHint, 0, len(syms))
	for _, sym := range syms {
		if !rangeIntersectsLine(requested, sym.Range.Start.Line) {
			continue
		}
		switch sym.Kind {
		case symbolKindEvent:
			out = append(out, inlayHint{
				Position: sym.Range.End,
				Label:    " eval",
				Kind:     3,
				Tooltip:  "Leia evaluate block discovered by `leia evaluate`.",
			})
		case symbolKindClass:
			out = append(out, inlayHint{
				Position: sym.Range.End,
				Label:    " agent",
				Kind:     3,
				Tooltip:  "Leia AI agent declaration.",
			})
		}
	}
	return out
}

func rangeIntersectsLine(r lspRange, line int) bool {
	if r.Start == (position{}) && r.End == (position{}) {
		return true
	}
	return line >= r.Start.Line && line <= r.End.Line
}

func hoverText(src, word string) string {
	if info, ok := keywordHover[word]; ok {
		return fmt.Sprintf("**%s**\n\n%s", word, info)
	}
	if module, ok := catalog.Module(word); ok {
		return fmt.Sprintf("**stdlib module `%s`**\n\n%s", module.Name, module.Description)
	}
	for _, sym := range collectSourceSymbols(src) {
		if sym.Name == word {
			return fmt.Sprintf("**%s**\n\n%s", sym.Name, sym.Detail)
		}
	}
	return ""
}

func findSourceSymbol(src, name string) (sourceSymbol, bool) {
	for _, sym := range collectSourceSymbols(src) {
		if sym.Name == name {
			return sym, true
		}
	}
	return sourceSymbol{}, false
}

func wordReferences(src, word string) []lspRange {
	if word == "" {
		return nil
	}
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil
	}
	var out []lspRange
	for _, tok := range tokens {
		if tok.Type == lexer.TOKEN_IDENT && tok.Value == word {
			out = append(out, tokenRange(tok))
			continue
		}
		if tok.Type == lexer.TOKEN_STRING && tok.Value == word && tokenLooksLikeEvaluateName(tokens, tok) {
			out = append(out, tokenRange(tok))
		}
	}
	return out
}

func tokenLooksLikeEvaluateName(tokens []lexer.Token, target lexer.Token) bool {
	for i, tok := range tokens {
		if tok.Line != target.Line || tok.Column != target.Column {
			continue
		}
		for j := i - 1; j >= 0 && tokens[j].Line == target.Line; j-- {
			if tokenText(tokens[j]) == "evaluate" {
				return true
			}
		}
		return false
	}
	return false
}

func filterReferenceRanges(refs []lspRange, exclude lspRange) []lspRange {
	out := refs[:0]
	for _, r := range refs {
		if sameRange(r, exclude) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func sameRange(a, b lspRange) bool {
	return a.Start == b.Start && a.End == b.End
}

func validIdentifierName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

var keywordHover = map[string]string{
	"agent":    "Declares or constructs an AI agent.",
	"break":    "Exits the nearest enclosing loop.",
	"chan":     "Creates or names a channel type in channel expressions.",
	"const":    "Declares read-only local bindings.",
	"continue": "Skips to the next iteration of the nearest enclosing loop.",
	"defer":    "Defers a call until the current function returns.",
	"else":     "Starts the fallback branch of an if statement.",
	"elseif":   "Starts an additional conditional branch of an if statement.",
	"evaluate": "Declares an agent regression case for `leia evaluate`.",
	"false":    "Boolean false literal.",
	"for":      "Starts a loop.",
	"func":     "Declares a named function or creates a function literal.",
	"go":       "Starts a concurrent call.",
	"goto":     "Jumps to a label in the current function.",
	"if":       "Starts a conditional statement.",
	"in":       "Separates range loop bindings from the iterator expression.",
	"nil":      "Nil literal.",
	"range":    "Iterates over values in a for loop.",
	"return":   "Returns values from the current function.",
	"tool":     "Declares an AI tool callable by agent workflows.",
	"true":     "Boolean true literal.",
	"var":      "Declares mutable local bindings.",
}

func collectSourceSymbols(src string) []sourceSymbol {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return collectSourceSymbolsFromTokens(src, tokens)
	}
	var out []sourceSymbol
	collectStmtSymbols(src, tokens, prog.Stmts, &out)
	return out
}

func collectStmtSymbols(src string, tokens []lexer.Token, stmts []ast.Stmt, out *[]sourceSymbol) {
	for _, stmt := range stmts {
		switch st := stmt.(type) {
		case *ast.FuncDeclStmt:
			*out = append(*out, declarationSymbol(src, tokens, st.P, "func", st.Name, functionDetail(st.Name, st.Params), symbolKindFunction))
			if st.Body != nil {
				collectStmtSymbols(src, tokens, st.Body.Stmts, out)
			}
		case *ast.ToolDeclStmt:
			*out = append(*out, declarationSymbol(src, tokens, st.P, "tool", st.Name, toolDetail(st), symbolKindFunction))
			if st.Body != nil {
				collectStmtSymbols(src, tokens, st.Body.Stmts, out)
			}
		case *ast.AgentDeclStmt:
			*out = append(*out, declarationSymbol(src, tokens, st.P, "agent", st.Name, agentDetail(st), symbolKindClass))
			if st.Flow != nil {
				collectStmtSymbols(src, tokens, st.Flow.Stmts, out)
			}
		case *ast.IfStmt:
			if st.Body != nil {
				collectStmtSymbols(src, tokens, st.Body.Stmts, out)
			}
			for _, elif := range st.ElseIfs {
				if elif.Body != nil {
					collectStmtSymbols(src, tokens, elif.Body.Stmts, out)
				}
			}
			if st.ElseBody != nil {
				collectStmtSymbols(src, tokens, st.ElseBody.Stmts, out)
			}
		case *ast.ForStmt:
			if st.Body != nil {
				collectStmtSymbols(src, tokens, st.Body.Stmts, out)
			}
		case *ast.ForNumStmt:
			if st.Body != nil {
				collectStmtSymbols(src, tokens, st.Body.Stmts, out)
			}
		case *ast.ForRangeStmt:
			if st.Body != nil {
				collectStmtSymbols(src, tokens, st.Body.Stmts, out)
			}
		case *ast.BudgetStmt:
			if st.Body != nil {
				collectStmtSymbols(src, tokens, st.Body.Stmts, out)
			}
		case *ast.EvaluateBlockStmt:
			*out = append(*out, evaluateBlockSymbol(src, tokens, st))
			if st.Body != nil {
				collectStmtSymbols(src, tokens, st.Body.Stmts, out)
			}
		}
	}
}

func collectSourceSymbolsFromTokens(src string, tokens []lexer.Token) []sourceSymbol {
	var out []sourceSymbol
	for i := 0; i+1 < len(tokens); i++ {
		tok := tokens[i]
		nameTok := tokens[i+1]
		if nameTok.Type != lexer.TOKEN_IDENT {
			continue
		}
		switch {
		case tok.Type == lexer.TOKEN_FUNC:
			out = append(out, declarationSymbol(src, tokens, tokenPos(tok), "func", nameTok.Value, fmt.Sprintf("func `%s`", nameTok.Value), symbolKindFunction))
		case tok.Type == lexer.TOKEN_IDENT && tok.Value == "tool":
			out = append(out, declarationSymbol(src, tokens, tokenPos(tok), "tool", nameTok.Value, fmt.Sprintf("tool `%s`", nameTok.Value), symbolKindFunction))
		case tok.Type == lexer.TOKEN_IDENT && tok.Value == "agent" && nameTok.Value != "defaults":
			out = append(out, declarationSymbol(src, tokens, tokenPos(tok), "agent", nameTok.Value, fmt.Sprintf("agent `%s`", nameTok.Value), symbolKindClass))
		}
	}
	return out
}

func evaluateBlockSymbol(src string, tokens []lexer.Token, st *ast.EvaluateBlockStmt) sourceSymbol {
	nameRange := findEvaluateNameRange(tokens, st.P, st.Name)
	return sourceSymbol{
		Name:      st.Name,
		Detail:    fmt.Sprintf("evaluate `%s`", st.Name),
		Kind:      symbolKindEvent,
		Range:     lineRange(src, nameRange.Start.Line),
		NameRange: nameRange,
	}
}

func declarationSymbol(src string, tokens []lexer.Token, pos ast.Pos, prefix, name, detail string, kind int) sourceSymbol {
	nameRange := findNameRange(tokens, pos, prefix, name)
	return sourceSymbol{
		Name:      name,
		Detail:    detail,
		Kind:      kind,
		Range:     lineRange(src, nameRange.Start.Line),
		NameRange: nameRange,
	}
}

func findNameRange(tokens []lexer.Token, pos ast.Pos, prefix, name string) lspRange {
	for i, tok := range tokens {
		if tok.Line != pos.Line || tok.Column != pos.Column || tokenText(tok) != prefix {
			continue
		}
		for j := i + 1; j < len(tokens); j++ {
			next := tokens[j]
			if next.Line != tok.Line && next.Type != lexer.TOKEN_IDENT {
				break
			}
			if next.Type == lexer.TOKEN_IDENT && next.Value == name {
				return tokenRange(next)
			}
		}
	}
	return lspRange{Start: positionFromOneBased(pos.Line, pos.Column), End: positionFromOneBased(pos.Line, pos.Column+len(name))}
}

func findEvaluateNameRange(tokens []lexer.Token, pos ast.Pos, name string) lspRange {
	for i, tok := range tokens {
		if tok.Line != pos.Line || tok.Column != pos.Column || tokenText(tok) != "evaluate" {
			continue
		}
		for j := i + 1; j < len(tokens); j++ {
			next := tokens[j]
			if next.Line != tok.Line {
				break
			}
			if next.Type == lexer.TOKEN_STRING && next.Value == name {
				return tokenRange(next)
			}
		}
	}
	return lspRange{Start: positionFromOneBased(pos.Line, pos.Column), End: positionFromOneBased(pos.Line, pos.Column+len(name))}
}

func functionDetail(name string, params []ast.FuncParam) string {
	return fmt.Sprintf("func `%s(%s)`", name, formatParams(params))
}

func toolDetail(st *ast.ToolDeclStmt) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("tool `%s(%s)`", st.Name, formatParams(st.Params)))
	if st.DocComment != "" {
		parts = append(parts, st.DocComment)
	}
	if len(st.Requires) > 0 {
		parts = append(parts, "requires: "+strings.Join(st.Requires, ", "))
	}
	return strings.Join(parts, "\n\n")
}

func agentDetail(st *ast.AgentDeclStmt) string {
	return fmt.Sprintf("agent `%s(%s)`", st.Name, formatParams(st.Params))
}

func formatParams(params []ast.FuncParam) string {
	out := make([]string, 0, len(params))
	for _, param := range params {
		if param.IsVarArg && param.Name != "..." {
			out = append(out, param.Name+"...")
			continue
		}
		out = append(out, param.Name)
	}
	return strings.Join(out, ", ")
}

func wordAtPosition(src string, pos position) (string, lspRange) {
	lines := strings.Split(src, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return "", lspRange{}
	}
	line := lines[pos.Line]
	if pos.Character < 0 {
		return "", lspRange{}
	}
	if pos.Character > len(line) {
		pos.Character = len(line)
	}
	start := pos.Character
	for start > 0 && isWordRune(rune(line[start-1])) {
		start--
	}
	end := pos.Character
	for end < len(line) && isWordRune(rune(line[end])) {
		end++
	}
	if start == end {
		return "", lspRange{}
	}
	return line[start:end], lspRange{
		Start: position{Line: pos.Line, Character: start},
		End:   position{Line: pos.Line, Character: end},
	}
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func lineRange(src string, line int) lspRange {
	lines := strings.Split(src, "\n")
	if line < 0 {
		line = 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}
	if line < 0 {
		line = 0
	}
	return lspRange{
		Start: position{Line: line, Character: 0},
		End:   position{Line: line, Character: len(lines[line])},
	}
}

func tokenRange(tok lexer.Token) lspRange {
	start := positionFromOneBased(tok.Line, tok.Column)
	return lspRange{
		Start: start,
		End:   position{Line: start.Line, Character: start.Character + len(tok.Value)},
	}
}

func stringLiteralContentRange(tok lexer.Token) lspRange {
	start := positionFromOneBased(tok.Line, tok.Column+1)
	return lspRange{
		Start: start,
		End:   position{Line: start.Line, Character: start.Character + len(tok.Value)},
	}
}

func tokenPos(tok lexer.Token) ast.Pos {
	return ast.Pos{Line: tok.Line, Column: tok.Column}
}

func tokenText(tok lexer.Token) string {
	if tok.Value != "" {
		return tok.Value
	}
	switch tok.Type {
	case lexer.TOKEN_FUNC:
		return "func"
	case lexer.TOKEN_CONST:
		return "const"
	case lexer.TOKEN_VAR:
		return "var"
	default:
		return ""
	}
}
