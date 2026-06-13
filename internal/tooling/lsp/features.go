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
	symbolKindFunction = 12
)

var semanticTokenTypes = []string{
	"keyword",
	"variable",
	"function",
	"method",
	"string",
	"number",
	"operator",
	"type",
	"parameter",
	"property",
	"namespace",
}

var semanticTokenModifiers = []string{
	"declaration",
	"readonly",
	"defaultLibrary",
	"import",
	"dialect",
}

const (
	semanticKeyword = iota
	semanticVariable
	semanticFunction
	semanticMethod
	semanticString
	semanticNumber
	semanticOperator
	semanticType
	semanticParameter
	semanticProperty
	semanticNamespace
)

const (
	semanticDeclarationModifier = 1 << iota
	semanticReadonlyModifier
	semanticDefaultLibraryModifier
	semanticImportModifier
	semanticDialectModifier
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

type semanticTokensParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type semanticTokensResult struct {
	Data []int `json:"data"`
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

func (s *Server) semanticTokensFull(id *json.RawMessage, params json.RawMessage) error {
	var p semanticTokensParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.respondMaybe(id, nil, &responseError{Code: errCodeInvalidParams, Message: err.Error()})
	}
	src, ok := s.documentText(p.TextDocument.URI)
	if !ok {
		return s.respondMaybe(id, semanticTokensResult{Data: []int{}}, nil)
	}
	return s.respondMaybe(id, semanticTokensResult{Data: collectSemanticTokens(src)}, nil)
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

func collectSemanticTokens(src string) []int {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return []int{}
	}
	var data []int
	prevLine, prevChar := 0, 0
	for i, tok := range tokens {
		tokenType, modifiers, ok := semanticTokenKind(tokens, i)
		if !ok {
			continue
		}
		r, ok := semanticTokenRange(src, tok)
		if !ok {
			continue
		}
		if r.End.Character <= r.Start.Character {
			continue
		}
		deltaLine := r.Start.Line - prevLine
		deltaStart := r.Start.Character
		if deltaLine == 0 {
			deltaStart -= prevChar
		}
		length := r.End.Character - r.Start.Character
		data = append(data, deltaLine, deltaStart, length, tokenType, modifiers)
		prevLine = r.Start.Line
		prevChar = r.Start.Character
	}
	return data
}

func semanticTokenKind(tokens []lexer.Token, index int) (int, int, bool) {
	tok := tokens[index]
	switch tok.Type {
	case lexer.TOKEN_IDENT:
		switch {
		case tokenLooksLikeImportAlias(tokens, index):
			return semanticNamespace, semanticDeclarationModifier | semanticImportModifier, true
		case tokenLooksLikeImportKeyword(tokens, index):
			return semanticKeyword, semanticImportModifier, true
		case tokenLooksLikeImportAs(tokens, index):
			return semanticKeyword, semanticImportModifier, true
		case tokenLooksLikeContextualKeyword(tokens, index):
			return semanticKeyword, 0, true
		case tokenLooksLikeDialectTag(tokens, index):
			return semanticNamespace, semanticDialectModifier, true
		case tokenIsDeclarationName(tokens, index):
			return semanticFunction, semanticDeclarationModifier, true
		case tokenIsParameterDeclaration(tokens, index):
			return semanticParameter, semanticDeclarationModifier, true
		case tokenIsConstDeclarationName(tokens, index):
			return semanticVariable, semanticDeclarationModifier | semanticReadonlyModifier, true
		case tokenLooksLikeMethodCall(tokens, index):
			return semanticMethod, 0, true
		case tokenLooksLikeProperty(tokens, index):
			return semanticProperty, 0, true
		case tokenLooksLikeStdlibNamespace(tokens, index):
			return semanticNamespace, semanticDefaultLibraryModifier, true
		case tokenLooksLikeFunctionCall(tokens, index):
			return semanticFunction, 0, true
		case tok.Value == "i32" || tok.Value == "i64" || tok.Value == "f32" || tok.Value == "f64" || tok.Value == "bool":
			return semanticType, semanticDefaultLibraryModifier, true
		default:
			return semanticVariable, 0, true
		}
	case lexer.TOKEN_STRING:
		if tokenLooksLikeImportPath(tokens, index) {
			return semanticString, semanticImportModifier, true
		}
		if tokenLooksLikeDialectBody(tokens, index) {
			return semanticString, semanticDialectModifier, true
		}
		return semanticString, 0, true
	case lexer.TOKEN_NUMBER:
		return semanticNumber, 0, true
	case lexer.TOKEN_TRUE, lexer.TOKEN_FALSE, lexer.TOKEN_NIL:
		return semanticKeyword, 0, true
	case lexer.TOKEN_FUNC, lexer.TOKEN_RETURN, lexer.TOKEN_IF, lexer.TOKEN_ELSE, lexer.TOKEN_ELSEIF,
		lexer.TOKEN_FOR, lexer.TOKEN_RANGE, lexer.TOKEN_BREAK, lexer.TOKEN_CONTINUE, lexer.TOKEN_IN,
		lexer.TOKEN_VAR, lexer.TOKEN_GO, lexer.TOKEN_CHAN, lexer.TOKEN_DEFER, lexer.TOKEN_CONST, lexer.TOKEN_GOTO:
		return semanticKeyword, 0, true
	case lexer.TOKEN_ARROW, lexer.TOKEN_ASSIGN, lexer.TOKEN_DECLARE, lexer.TOKEN_PLUS_ASSIGN, lexer.TOKEN_MINUS_ASSIGN,
		lexer.TOKEN_STAR_ASSIGN, lexer.TOKEN_SLASH_ASSIGN, lexer.TOKEN_EQ, lexer.TOKEN_NEQ, lexer.TOKEN_LT, lexer.TOKEN_LE,
		lexer.TOKEN_GT, lexer.TOKEN_GE, lexer.TOKEN_PLUS, lexer.TOKEN_MINUS, lexer.TOKEN_STAR, lexer.TOKEN_SLASH,
		lexer.TOKEN_PERCENT, lexer.TOKEN_POW, lexer.TOKEN_AND, lexer.TOKEN_OR, lexer.TOKEN_NOT, lexer.TOKEN_BIT_AND,
		lexer.TOKEN_BIT_OR, lexer.TOKEN_BIT_XOR, lexer.TOKEN_BIT_AND_NOT, lexer.TOKEN_SHL, lexer.TOKEN_SHR,
		lexer.TOKEN_CONCAT, lexer.TOKEN_LEN, lexer.TOKEN_ELLIPSIS, lexer.TOKEN_INC, lexer.TOKEN_DEC:
		if tok.Type == lexer.TOKEN_NOT && tokenLooksLikeDialectBang(tokens, index) {
			return semanticOperator, semanticDialectModifier, true
		}
		return semanticOperator, 0, true
	case lexer.TOKEN_DOLLAR:
		if tokenLooksLikeShellDialectTag(tokens, index) {
			return semanticNamespace, semanticDialectModifier, true
		}
		return 0, 0, false
	default:
		return 0, 0, false
	}
}

func tokenIsDeclarationName(tokens []lexer.Token, index int) bool {
	if index == 0 || tokens[index].Type != lexer.TOKEN_IDENT {
		return false
	}
	prev := tokens[index-1]
	return prev.Type == lexer.TOKEN_FUNC
}

func tokenLooksLikeImportAlias(tokens []lexer.Token, index int) bool {
	if index > 0 && tokenText(tokens[index-1]) == "as" {
		return true
	}
	return tokens[index].Type == lexer.TOKEN_IDENT &&
		index+1 < len(tokens) && tokens[index+1].Type == lexer.TOKEN_STRING &&
		(index > 0 && tokenText(tokens[index-1]) == "import" || tokenIsInImportGroup(tokens, index))
}

func tokenLooksLikeImportKeyword(tokens []lexer.Token, index int) bool {
	return tokens[index].Type == lexer.TOKEN_IDENT && tokens[index].Value == "import" && tokenAtStartOfStmt(tokens, index)
}

func tokenLooksLikeImportAs(tokens []lexer.Token, index int) bool {
	return tokens[index].Type == lexer.TOKEN_IDENT && tokens[index].Value == "as" && index > 0 &&
		tokens[index-1].Type == lexer.TOKEN_STRING && index+1 < len(tokens) && tokens[index+1].Type == lexer.TOKEN_IDENT
}

func tokenLooksLikeDialectTag(tokens []lexer.Token, index int) bool {
	if tokens[index].Type != lexer.TOKEN_IDENT || !tokenCanStartDialectExpression(tokens, index) {
		return false
	}
	return index+1 < len(tokens) && (tokenIsRawString(tokens[index+1]) || tokens[index+1].Type == lexer.TOKEN_LBRACE ||
		(tokens[index+1].Type == lexer.TOKEN_NOT && index+2 < len(tokens) &&
			(tokenIsRawString(tokens[index+2]) || tokens[index+2].Type == lexer.TOKEN_LBRACE)))
}

func tokenCanStartDialectExpression(tokens []lexer.Token, index int) bool {
	if index == 0 {
		return true
	}
	prev := tokens[index-1]
	if prev.Line < tokens[index].Line {
		return true
	}
	switch prev.Type {
	case lexer.TOKEN_ASSIGN, lexer.TOKEN_DECLARE, lexer.TOKEN_COMMA, lexer.TOKEN_LPAREN, lexer.TOKEN_LBRACKET,
		lexer.TOKEN_LBRACE, lexer.TOKEN_COLON, lexer.TOKEN_RETURN, lexer.TOKEN_ARROW, lexer.TOKEN_PLUS,
		lexer.TOKEN_MINUS, lexer.TOKEN_STAR, lexer.TOKEN_SLASH, lexer.TOKEN_PERCENT, lexer.TOKEN_AND,
		lexer.TOKEN_OR, lexer.TOKEN_NOT:
		return true
	default:
		return false
	}
}

func tokenLooksLikeDialectBang(tokens []lexer.Token, index int) bool {
	if index == 0 || index+1 >= len(tokens) {
		return false
	}
	switch tokens[index-1].Type {
	case lexer.TOKEN_IDENT:
		return (tokenIsRawString(tokens[index+1]) || tokens[index+1].Type == lexer.TOKEN_LBRACE) &&
			tokenLooksLikeDialectTag(tokens, index-1)
	case lexer.TOKEN_DOLLAR:
		return tokenIsRawString(tokens[index+1]) && tokenLooksLikeShellDialectTag(tokens, index-1)
	default:
		return false
	}
}

func tokenLooksLikeDialectBody(tokens []lexer.Token, index int) bool {
	if !tokenIsRawString(tokens[index]) {
		return false
	}
	if index > 0 && tokens[index-1].Type == lexer.TOKEN_IDENT && tokenLooksLikeDialectTag(tokens, index-1) {
		return true
	}
	if index > 1 && tokens[index-1].Type == lexer.TOKEN_NOT && tokenLooksLikeDialectBang(tokens, index-1) {
		return true
	}
	return index > 0 && (tokenLooksLikeShellDialectTag(tokens, index-1) ||
		(tokens[index-1].Type == lexer.TOKEN_NOT && tokenLooksLikeDialectBang(tokens, index-1)))
}

func tokenLooksLikeShellDialectTag(tokens []lexer.Token, index int) bool {
	return tokens[index].Type == lexer.TOKEN_DOLLAR && index+1 < len(tokens) &&
		(tokenIsRawString(tokens[index+1]) ||
			(tokens[index+1].Type == lexer.TOKEN_NOT && index+2 < len(tokens) && tokenIsRawString(tokens[index+2])))
}

func tokenIsRawString(tok lexer.Token) bool {
	return tok.Type == lexer.TOKEN_STRING && tok.IsRawString
}

func tokenLooksLikeContextualKeyword(tokens []lexer.Token, index int) bool {
	tok := tokens[index]
	if tok.Type != lexer.TOKEN_IDENT {
		return false
	}
	switch tok.Value {
	case "import":
		return tokenAtStartOfStmt(tokens, index)
	case "select":
		return tokenAtStartOfStmt(tokens, index) && index+1 < len(tokens) && tokens[index+1].Type == lexer.TOKEN_LBRACE
	case "case", "default":
		return tokenInsideSelect(tokens, index)
	case "as":
		return index > 0 && tokens[index-1].Type == lexer.TOKEN_STRING && index+1 < len(tokens) && tokens[index+1].Type == lexer.TOKEN_IDENT
	default:
		return false
	}
}

func tokenAtStartOfStmt(tokens []lexer.Token, index int) bool {
	if index == 0 {
		return true
	}
	prev := tokens[index-1]
	return prev.Type == lexer.TOKEN_SEMICOLON || prev.Type == lexer.TOKEN_LBRACE || prev.Type == lexer.TOKEN_RBRACE || prev.Line < tokens[index].Line
}

func tokenInsideSelect(tokens []lexer.Token, index int) bool {
	for i := index - 1; i >= 0; i-- {
		if tokenText(tokens[i]) == "select" && i+1 < len(tokens) && tokens[i+1].Type == lexer.TOKEN_LBRACE {
			return true
		}
		if tokens[i].Type == lexer.TOKEN_RBRACE {
			return false
		}
	}
	return false
}

func tokenIsParameterDeclaration(tokens []lexer.Token, index int) bool {
	open := -1
	depth := 0
	for i := index - 1; i >= 0; i-- {
		switch tokens[i].Type {
		case lexer.TOKEN_RPAREN:
			depth++
		case lexer.TOKEN_LPAREN:
			if depth == 0 {
				open = i
				i = -1
			} else {
				depth--
			}
		}
	}
	if open <= 1 {
		return false
	}
	if tokenText(tokens[open-2]) != "func" {
		return false
	}
	for i := open + 1; i < len(tokens) && i < index; i++ {
		if tokens[i].Type == lexer.TOKEN_RPAREN {
			return false
		}
	}
	return true
}

func tokenIsConstDeclarationName(tokens []lexer.Token, index int) bool {
	return index > 0 && tokens[index].Type == lexer.TOKEN_IDENT && tokens[index-1].Type == lexer.TOKEN_CONST
}

func tokenLooksLikeMethodCall(tokens []lexer.Token, index int) bool {
	return index > 0 && index+1 < len(tokens) && tokens[index-1].Type == lexer.TOKEN_COLON && tokens[index+1].Type == lexer.TOKEN_LPAREN
}

func tokenLooksLikeProperty(tokens []lexer.Token, index int) bool {
	return (index > 0 && tokens[index-1].Type == lexer.TOKEN_DOT) || (index+1 < len(tokens) && tokens[index+1].Type == lexer.TOKEN_COLON)
}

func tokenLooksLikeStdlibNamespace(tokens []lexer.Token, index int) bool {
	if index+1 >= len(tokens) || tokens[index+1].Type != lexer.TOKEN_DOT {
		return false
	}
	_, ok := catalog.Module(tokens[index].Value)
	return ok
}

func tokenLooksLikeFunctionCall(tokens []lexer.Token, index int) bool {
	return index+1 < len(tokens) && tokens[index+1].Type == lexer.TOKEN_LPAREN
}

func semanticTokenRange(src string, tok lexer.Token) (lspRange, bool) {
	if tok.Type == lexer.TOKEN_STRING {
		return sourceStringLiteralContentRange(src, tok)
	}
	return tokenRange(tok), true
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
	if index > 0 && tokenText(tokens[index-1]) == "import" {
		return true
	}
	if index > 1 && tokens[index-1].Type == lexer.TOKEN_IDENT && tokenText(tokens[index-2]) == "import" {
		return true
	}
	if tokenIsInImportGroup(tokens, index) {
		return true
	}
	if index > 0 && tokenText(tokens[index-1]) == "as" {
		return true
	}
	return false
}

func tokenIsInImportGroup(tokens []lexer.Token, index int) bool {
	depth := 0
	for i := index - 1; i >= 0; i-- {
		switch tokens[i].Type {
		case lexer.TOKEN_RPAREN:
			depth++
		case lexer.TOKEN_LPAREN:
			if depth == 0 {
				return i > 0 && tokenText(tokens[i-1]) == "import"
			}
			depth--
		}
	}
	return false
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
	prog, ok := parseLSPProgram(src)
	if !ok {
		return []codeLens{}
	}
	var out []codeLens
	collectCodeLensFromStmts(uri, src, prog.Stmts, &out)
	return out
}

func collectInlayHints(src string, requested lspRange) []inlayHint {
	prog, ok := parseLSPProgram(src)
	if !ok {
		return []inlayHint{}
	}
	sigs := map[string][]ast.FuncParam{}
	collectFunctionSignatures(prog.Stmts, sigs)
	var out []inlayHint
	collectInlayHintsFromStmts(src, prog.Stmts, sigs, requested, &out)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Position.Line != out[j].Position.Line {
			return out[i].Position.Line < out[j].Position.Line
		}
		if out[i].Position.Character != out[j].Position.Character {
			return out[i].Position.Character < out[j].Position.Character
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func parseLSPProgram(src string) (*ast.Program, bool) {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return nil, false
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, false
	}
	return prog, true
}

func collectCodeLensFromStmts(uri, src string, stmts []ast.Stmt, out *[]codeLens) {
	for _, stmt := range stmts {
		switch st := stmt.(type) {
		case *ast.EvaluateStmt:
			title := "Run evaluate case"
			if strings.TrimSpace(st.Name) != "" {
				title = "Run evaluate: " + st.Name
			}
			*out = append(*out, codeLens{
				Range: lineRange(src, st.P.Line-1),
				Command: command{
					Title:     title,
					Command:   "leia.evaluate.case",
					Arguments: []any{uri, st.Name},
				},
			})
			if st.Body != nil {
				collectCodeLensFromStmts(uri, src, st.Body.Stmts, out)
			}
		case *ast.BlockStmt:
			collectCodeLensFromStmts(uri, src, st.Stmts, out)
		case *ast.FuncDeclStmt:
			if st.Body != nil {
				collectCodeLensFromStmts(uri, src, st.Body.Stmts, out)
			}
		case *ast.IfStmt:
			if st.Body != nil {
				collectCodeLensFromStmts(uri, src, st.Body.Stmts, out)
			}
			for _, elseif := range st.ElseIfs {
				if elseif.Body != nil {
					collectCodeLensFromStmts(uri, src, elseif.Body.Stmts, out)
				}
			}
			if st.ElseBody != nil {
				collectCodeLensFromStmts(uri, src, st.ElseBody.Stmts, out)
			}
		case *ast.ForStmt:
			if st.Body != nil {
				collectCodeLensFromStmts(uri, src, st.Body.Stmts, out)
			}
		case *ast.ForNumStmt:
			if st.Body != nil {
				collectCodeLensFromStmts(uri, src, st.Body.Stmts, out)
			}
		case *ast.ForRangeStmt:
			if st.Body != nil {
				collectCodeLensFromStmts(uri, src, st.Body.Stmts, out)
			}
		case *ast.SelectStmt:
			for _, clause := range st.Cases {
				if clause.Body != nil {
					collectCodeLensFromStmts(uri, src, clause.Body.Stmts, out)
				}
			}
			if st.Default != nil {
				collectCodeLensFromStmts(uri, src, st.Default.Stmts, out)
			}
		}
	}
}

func collectFunctionSignatures(stmts []ast.Stmt, out map[string][]ast.FuncParam) {
	for _, stmt := range stmts {
		switch st := stmt.(type) {
		case *ast.FuncDeclStmt:
			out[st.Name] = append([]ast.FuncParam(nil), st.Params...)
			if st.Body != nil {
				collectFunctionSignatures(st.Body.Stmts, out)
			}
		case *ast.BlockStmt:
			collectFunctionSignatures(st.Stmts, out)
		case *ast.IfStmt:
			if st.Body != nil {
				collectFunctionSignatures(st.Body.Stmts, out)
			}
			for _, elseif := range st.ElseIfs {
				if elseif.Body != nil {
					collectFunctionSignatures(elseif.Body.Stmts, out)
				}
			}
			if st.ElseBody != nil {
				collectFunctionSignatures(st.ElseBody.Stmts, out)
			}
		case *ast.ForStmt:
			if st.Body != nil {
				collectFunctionSignatures(st.Body.Stmts, out)
			}
		case *ast.ForNumStmt:
			if st.Body != nil {
				collectFunctionSignatures(st.Body.Stmts, out)
			}
		case *ast.ForRangeStmt:
			if st.Body != nil {
				collectFunctionSignatures(st.Body.Stmts, out)
			}
		case *ast.EvaluateStmt:
			if st.Body != nil {
				collectFunctionSignatures(st.Body.Stmts, out)
			}
		case *ast.SelectStmt:
			for _, clause := range st.Cases {
				if clause.Body != nil {
					collectFunctionSignatures(clause.Body.Stmts, out)
				}
			}
			if st.Default != nil {
				collectFunctionSignatures(st.Default.Stmts, out)
			}
		}
	}
}

func collectInlayHintsFromStmts(src string, stmts []ast.Stmt, sigs map[string][]ast.FuncParam, requested lspRange, out *[]inlayHint) {
	for _, stmt := range stmts {
		switch st := stmt.(type) {
		case *ast.DeclareStmt:
			collectStdlibRequireHint(st, requested, out)
			for _, value := range st.Values {
				collectInlayHintsFromExpr(value, sigs, requested, out)
			}
		case *ast.AssignStmt:
			for _, value := range st.Values {
				collectInlayHintsFromExpr(value, sigs, requested, out)
			}
		case *ast.CompoundAssignStmt:
			collectInlayHintsFromExpr(st.Target, sigs, requested, out)
			collectInlayHintsFromExpr(st.Value, sigs, requested, out)
		case *ast.IncDecStmt:
			collectInlayHintsFromExpr(st.Target, sigs, requested, out)
		case *ast.CallStmt:
			collectInlayHintsFromExpr(st.Call, sigs, requested, out)
		case *ast.GoStmt:
			collectInlayHintsFromExpr(st.Call, sigs, requested, out)
		case *ast.DeferStmt:
			collectInlayHintsFromExpr(st.Call, sigs, requested, out)
		case *ast.SendStmt:
			collectInlayHintsFromExpr(st.Channel, sigs, requested, out)
			collectInlayHintsFromExpr(st.Value, sigs, requested, out)
		case *ast.SelectStmt:
			for _, clause := range st.Cases {
				collectInlayHintsFromExpr(clause.Channel, sigs, requested, out)
				collectInlayHintsFromExpr(clause.SendValue, sigs, requested, out)
				if clause.Body != nil {
					collectInlayHintsFromStmts(src, clause.Body.Stmts, sigs, requested, out)
				}
			}
			if st.Default != nil {
				collectInlayHintsFromStmts(src, st.Default.Stmts, sigs, requested, out)
			}
		case *ast.IfStmt:
			collectInlayHintsFromExpr(st.Cond, sigs, requested, out)
			if st.Body != nil {
				collectInlayHintsFromStmts(src, st.Body.Stmts, sigs, requested, out)
			}
			for _, elseif := range st.ElseIfs {
				collectInlayHintsFromExpr(elseif.Cond, sigs, requested, out)
				if elseif.Body != nil {
					collectInlayHintsFromStmts(src, elseif.Body.Stmts, sigs, requested, out)
				}
			}
			if st.ElseBody != nil {
				collectInlayHintsFromStmts(src, st.ElseBody.Stmts, sigs, requested, out)
			}
		case *ast.ForStmt:
			collectInlayHintsFromExpr(st.Cond, sigs, requested, out)
			if st.Body != nil {
				collectInlayHintsFromStmts(src, st.Body.Stmts, sigs, requested, out)
			}
		case *ast.ForNumStmt:
			collectInlayHintsFromStmts(src, []ast.Stmt{st.Init, st.Post}, sigs, requested, out)
			collectInlayHintsFromExpr(st.Cond, sigs, requested, out)
			if st.Body != nil {
				collectInlayHintsFromStmts(src, st.Body.Stmts, sigs, requested, out)
			}
		case *ast.ForRangeStmt:
			collectInlayHintsFromExpr(st.Iter, sigs, requested, out)
			if st.Body != nil {
				collectInlayHintsFromStmts(src, st.Body.Stmts, sigs, requested, out)
			}
		case *ast.ReturnStmt:
			for _, value := range st.Values {
				collectInlayHintsFromExpr(value, sigs, requested, out)
			}
		case *ast.EvaluateStmt:
			if st.Body != nil {
				collectInlayHintsFromStmts(src, st.Body.Stmts, sigs, requested, out)
			}
		case *ast.FuncDeclStmt:
			if st.Body != nil {
				collectInlayHintsFromStmts(src, st.Body.Stmts, sigs, requested, out)
			}
		case *ast.BlockStmt:
			collectInlayHintsFromStmts(src, st.Stmts, sigs, requested, out)
		}
	}
}

func collectStdlibRequireHint(stmt *ast.DeclareStmt, requested lspRange, out *[]inlayHint) {
	if len(stmt.Names) != 1 || len(stmt.Values) != 1 {
		return
	}
	call, ok := stmt.Values[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return
	}
	fn, ok := call.Func.(*ast.IdentExpr)
	if !ok || fn.Name != "require" {
		return
	}
	arg, ok := call.Args[0].(*ast.StringLit)
	if !ok {
		return
	}
	module, ok := catalog.Module(arg.Value)
	if !ok {
		return
	}
	pos := positionFromOneBased(arg.P.Line, arg.P.Column+len(arg.Value)+2)
	if stmt.P.Line == fn.P.Line && stmt.P.Column < fn.P.Column {
		pos = positionFromOneBased(stmt.P.Line, stmt.P.Column+len(stmt.Names[0]))
	}
	if !positionInRequestedRange(pos, requested) {
		return
	}
	*out = append(*out, inlayHint{
		Position: pos,
		Label:    ": stdlib " + module.Layer,
		Kind:     1,
		Tooltip:  module.Description,
	})
}

func collectInlayHintsFromExpr(expr ast.Expr, sigs map[string][]ast.FuncParam, requested lspRange, out *[]inlayHint) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		collectInlayHintsFromExpr(e.Left, sigs, requested, out)
		collectInlayHintsFromExpr(e.Right, sigs, requested, out)
	case *ast.UnaryExpr:
		collectInlayHintsFromExpr(e.Operand, sigs, requested, out)
	case *ast.ParenExpr:
		collectInlayHintsFromExpr(e.Inner, sigs, requested, out)
	case *ast.IndexExpr:
		collectInlayHintsFromExpr(e.Table, sigs, requested, out)
		collectInlayHintsFromExpr(e.Index, sigs, requested, out)
	case *ast.FieldExpr:
		collectInlayHintsFromExpr(e.Table, sigs, requested, out)
	case *ast.CallExpr:
		collectCallParamHints(e, sigs, requested, out)
		collectInlayHintsFromExpr(e.Func, sigs, requested, out)
		for _, arg := range e.Args {
			collectInlayHintsFromExpr(arg, sigs, requested, out)
		}
	case *ast.MethodCallExpr:
		collectInlayHintsFromExpr(e.Object, sigs, requested, out)
		for _, arg := range e.Args {
			collectInlayHintsFromExpr(arg, sigs, requested, out)
		}
	case *ast.FuncLitExpr:
		if e.Body != nil {
			collectInlayHintsFromStmts("", e.Body.Stmts, sigs, requested, out)
		}
	case *ast.ListLitExpr:
		for _, value := range e.Values {
			collectInlayHintsFromExpr(value, sigs, requested, out)
		}
	case *ast.TableLitExpr:
		for _, field := range e.Fields {
			collectInlayHintsFromExpr(field.Key, sigs, requested, out)
			collectInlayHintsFromExpr(field.Value, sigs, requested, out)
		}
	case *ast.DenseLitExpr:
		for _, value := range e.Values {
			collectInlayHintsFromExpr(value, sigs, requested, out)
		}
	case *ast.RecvExpr:
		collectInlayHintsFromExpr(e.Channel, sigs, requested, out)
	case *ast.MakeChanExpr:
		collectInlayHintsFromExpr(e.Size, sigs, requested, out)
	case *ast.TaggedStringExpr:
		collectInlayHintsFromExpr(e.Body, sigs, requested, out)
	case *ast.TaggedBlockExpr:
		for _, field := range e.Config {
			collectInlayHintsFromExpr(field.Key, sigs, requested, out)
			collectInlayHintsFromExpr(field.Value, sigs, requested, out)
		}
		if e.Body != nil {
			collectInlayHintsFromStmts("", e.Body.Stmts, sigs, requested, out)
		}
	case *ast.InterpolatedStringExpr:
		for _, part := range e.Parts {
			collectInlayHintsFromExpr(part.Expr, sigs, requested, out)
		}
	}
}

func collectCallParamHints(call *ast.CallExpr, sigs map[string][]ast.FuncParam, requested lspRange, out *[]inlayHint) {
	ident, ok := call.Func.(*ast.IdentExpr)
	if !ok {
		return
	}
	params := sigs[ident.Name]
	for i := 0; i < len(call.Args) && i < len(params); i++ {
		param := params[i]
		if param.Name == "" || param.Name == "_" || param.Name == "..." || param.IsVarArg {
			continue
		}
		if argIdent, ok := call.Args[i].(*ast.IdentExpr); ok && argIdent.Name == param.Name {
			continue
		}
		pos := positionFromOneBased(call.Args[i].GetPos().Line, call.Args[i].GetPos().Column)
		if !positionInRequestedRange(pos, requested) {
			continue
		}
		*out = append(*out, inlayHint{
			Position: pos,
			Label:    param.Name + ":",
			Kind:     2,
			Tooltip:  "Parameter for " + ident.Name + "(" + formatParams(params) + ")",
		})
	}
}

func positionInRequestedRange(pos position, requested lspRange) bool {
	if requested == (lspRange{}) {
		return true
	}
	if comparePosition(pos, requested.Start) < 0 {
		return false
	}
	return comparePosition(pos, requested.End) <= 0
}

func comparePosition(a, b position) int {
	if a.Line < b.Line {
		return -1
	}
	if a.Line > b.Line {
		return 1
	}
	if a.Character < b.Character {
		return -1
	}
	if a.Character > b.Character {
		return 1
	}
	return 0
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
	}
	return out
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
	"as":       "Names an imported module alias.",
	"break":    "Exits the nearest enclosing loop.",
	"case":     "Starts a select case.",
	"chan":     "Creates or names a channel type in channel expressions.",
	"const":    "Declares read-only local bindings.",
	"continue": "Skips to the next iteration of the nearest enclosing loop.",
	"default":  "Starts the default select branch.",
	"defer":    "Defers a call until the current function returns.",
	"else":     "Starts the fallback branch of an if statement.",
	"elseif":   "Starts an additional conditional branch of an if statement.",
	"false":    "Boolean false literal.",
	"for":      "Starts a loop.",
	"func":     "Declares a named function or creates a function literal.",
	"go":       "Starts a concurrent call.",
	"goto":     "Jumps to a label in the current function.",
	"if":       "Starts a conditional statement.",
	"import":   "Imports a Leia module through `require`.",
	"in":       "Separates range loop bindings from the iterator expression.",
	"nil":      "Nil literal.",
	"range":    "Iterates over values in a for loop.",
	"return":   "Returns values from the current function.",
	"select":   "Waits on channel send or receive cases.",
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
		}
	}
	return out
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

func functionDetail(name string, params []ast.FuncParam) string {
	return fmt.Sprintf("func `%s(%s)`", name, formatParams(params))
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

func sourceStringLiteralContentRange(src string, tok lexer.Token) (lspRange, bool) {
	lines := strings.Split(src, "\n")
	lineIdx := tok.Line - 1
	colIdx := tok.Column - 1
	if lineIdx < 0 || lineIdx >= len(lines) || colIdx < 0 || colIdx >= len(lines[lineIdx]) {
		return lspRange{}, false
	}
	line := lines[lineIdx]
	quote := line[colIdx]
	if quote != '"' && quote != '`' {
		return stringLiteralContentRange(tok), true
	}
	delimLen := 1
	if quote == '`' && colIdx+2 < len(line) && line[colIdx+1] == '`' && line[colIdx+2] == '`' {
		delimLen = 3
	}
	for i := colIdx + delimLen; i+delimLen <= len(line); i++ {
		ch := line[i]
		if quote == '"' && ch == '\\' {
			i++
			continue
		}
		if ch == quote && strings.HasPrefix(line[i:], strings.Repeat(string(quote), delimLen)) {
			return lspRange{
				Start: position{Line: lineIdx, Character: colIdx + delimLen},
				End:   position{Line: lineIdx, Character: i},
			}, true
		}
	}
	// Multiline raw strings are valid Leia, but LSP semantic token entries are
	// single-line. Skip them until the server supports multiline token splitting.
	return lspRange{}, false
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
