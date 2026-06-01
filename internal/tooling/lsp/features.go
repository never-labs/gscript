package lsp

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/stdlib/catalog"
)

const (
	symbolKindClass    = 5
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

type sourceSymbol struct {
	Name      string
	Detail    string
	Kind      int
	Range     lspRange
	NameRange lspRange
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

func (s *Server) documentText(uri string) (string, bool) {
	if uri == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.docs[uri]
	return src, ok
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

var keywordHover = map[string]string{
	"agent":    "Declares or constructs an AI agent.",
	"break":    "Exits the nearest enclosing loop.",
	"chan":     "Creates or names a channel type in channel expressions.",
	"const":    "Declares read-only local bindings.",
	"continue": "Skips to the next iteration of the nearest enclosing loop.",
	"defer":    "Defers a call until the current function returns.",
	"else":     "Starts the fallback branch of an if statement.",
	"elseif":   "Starts an additional conditional branch of an if statement.",
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
