package q

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenNumber
	tokenString
	tokenSymbol
	tokenComma
	tokenColon
	tokenOp
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

func Parse(src string) (*Query, error) {
	tokens, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := parser{tokens: tokens}
	return p.parseQuery()
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) parseQuery() (*Query, error) {
	kindTok := p.peek()
	var kind QueryKind
	switch strings.ToLower(kindTok.text) {
	case string(SelectQuery):
		kind = SelectQuery
	case string(ExecQuery):
		kind = ExecQuery
	default:
		return nil, p.errorf(kindTok, "expected select or exec")
	}
	p.next()

	columns, err := p.parseColumns()
	if err != nil {
		return nil, err
	}

	var by []Expr
	if p.consumeKeyword("by") {
		by, err = p.parseExprListUntil("from")
		if err != nil {
			return nil, err
		}
	}

	if !p.consumeKeyword("from") {
		return nil, p.errorf(p.peek(), "expected from")
	}
	fromTok := p.peek()
	if fromTok.kind != tokenIdent {
		return nil, p.errorf(fromTok, "expected table name")
	}
	p.next()

	var where Expr
	if p.consumeKeyword("where") {
		where, err = p.parseExpr(0)
		if err != nil {
			return nil, err
		}
	}

	var orderBy []OrderTerm
	if p.consumeKeyword("order") {
		if !p.consumeKeyword("by") {
			return nil, p.errorf(p.peek(), "expected by after order")
		}
		orderBy, err = p.parseOrderBy()
		if err != nil {
			return nil, err
		}
	}

	var limit *int
	if p.consumeKeyword("limit") {
		n, err := p.parseLimit()
		if err != nil {
			return nil, err
		}
		limit = &n
	}

	if p.peek().kind != tokenEOF {
		return nil, p.errorf(p.peek(), "unexpected token %q", p.peek().text)
	}

	return &Query{
		Kind:    kind,
		Columns: columns,
		By:      by,
		From:    fromTok.text,
		Where:   where,
		OrderBy: orderBy,
		Limit:   limit,
	}, nil
}

func (p *parser) parseColumns() ([]Column, error) {
	var columns []Column
	for {
		if p.peek().kind == tokenEOF || p.isKeyword("from") || p.isKeyword("by") {
			break
		}
		column, err := p.parseColumn()
		if err != nil {
			return nil, err
		}
		columns = append(columns, column)
		if !p.consume(tokenComma) {
			break
		}
	}
	if len(columns) == 0 {
		return nil, p.errorf(p.peek(), "expected projection")
	}
	return columns, nil
}

func (p *parser) parseColumn() (Column, error) {
	if p.peek().kind == tokenIdent && p.look(1).kind == tokenColon {
		name := p.next().text
		p.next()
		expr, err := p.parseExpr(0)
		return Column{Name: name, Expr: expr}, err
	}
	expr, err := p.parseExpr(0)
	if err != nil {
		return Column{}, err
	}
	return Column{Name: columnName(expr), Expr: expr}, nil
}

func (p *parser) parseExprListUntil(keyword string) ([]Expr, error) {
	var exprs []Expr
	for {
		if p.peek().kind == tokenEOF || p.isKeyword(keyword) {
			break
		}
		expr, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
		if !p.consume(tokenComma) {
			break
		}
	}
	if len(exprs) == 0 {
		return nil, p.errorf(p.peek(), "expected expression before %s", keyword)
	}
	return exprs, nil
}

func (p *parser) parseOrderBy() ([]OrderTerm, error) {
	var terms []OrderTerm
	for {
		if p.peek().kind == tokenEOF || p.isKeyword("limit") {
			break
		}
		tok := p.peek()
		if tok.kind != tokenIdent {
			return nil, p.errorf(tok, "expected order by column")
		}
		p.next()
		term := OrderTerm{Column: tok.text}
		if p.consumeKeyword("asc") {
			term.Desc = false
		} else if p.consumeKeyword("desc") {
			term.Desc = true
		}
		terms = append(terms, term)
		if !p.consume(tokenComma) {
			break
		}
	}
	if len(terms) == 0 {
		return nil, p.errorf(p.peek(), "expected order by column")
	}
	return terms, nil
}

func (p *parser) parseLimit() (int, error) {
	tok := p.peek()
	if tok.kind != tokenNumber {
		return 0, p.errorf(tok, "expected limit count")
	}
	p.next()
	n, err := strconv.Atoi(tok.text)
	if err != nil || n < 0 || strconv.Itoa(n) != tok.text {
		return 0, p.errorf(tok, "expected non-negative integer limit")
	}
	return n, nil
}

func (p *parser) parseExpr(minPrec int) (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		if tok.kind != tokenOp {
			break
		}
		prec := precedence(tok.text)
		if prec < minPrec {
			break
		}
		p.next()
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: tok.text, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parsePrimary() (Expr, error) {
	tok := p.peek()
	switch tok.kind {
	case tokenNumber:
		p.next()
		return Number{Text: tok.text}, nil
	case tokenString:
		p.next()
		return String{Value: tok.text}, nil
	case tokenSymbol:
		p.next()
		return Symbol{Name: tok.text}, nil
	case tokenIdent:
		p.next()
		switch strings.ToLower(tok.text) {
		case "true":
			return Bool{Value: true}, nil
		case "false":
			return Bool{Value: false}, nil
		case "null", "nil":
			return Null{}, nil
		}
		if p.canStartCallArg() {
			arg, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			return Call{Func: tok.text, Arg: arg}, nil
		}
		return Ident{Name: tok.text}, nil
	default:
		return nil, p.errorf(tok, "expected expression")
	}
}

func columnName(expr Expr) string {
	if ident, ok := expr.(Ident); ok {
		return ident.Name
	}
	return ""
}

func canStartPrimary(tok token) bool {
	return tok.kind == tokenIdent || tok.kind == tokenNumber || tok.kind == tokenString || tok.kind == tokenSymbol
}

func (p *parser) canStartCallArg() bool {
	tok := p.peek()
	if tok.kind == tokenIdent {
		switch strings.ToLower(tok.text) {
		case "asc", "by", "desc", "from", "limit", "order", "where":
			return false
		}
	}
	return canStartPrimary(tok)
}

func precedence(op string) int {
	switch op {
	case "*", "/":
		return 30
	case "+", "-":
		return 20
	case "=", "<", ">", "<=", ">=", "<>":
		return 10
	default:
		return -1
	}
}

func (p *parser) consume(kind tokenKind) bool {
	if p.peek().kind != kind {
		return false
	}
	p.next()
	return true
}

func (p *parser) consumeKeyword(keyword string) bool {
	if !p.isKeyword(keyword) {
		return false
	}
	p.next()
	return true
}

func (p *parser) isKeyword(keyword string) bool {
	tok := p.peek()
	return tok.kind == tokenIdent && strings.EqualFold(tok.text, keyword)
}

func (p *parser) peek() token {
	return p.look(0)
}

func (p *parser) look(offset int) token {
	idx := p.pos + offset
	if idx >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[idx]
}

func (p *parser) next() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) errorf(tok token, format string, args ...any) error {
	return fmt.Errorf("q parse error at %d: %s", tok.pos, fmt.Sprintf(format, args...))
}

func lex(src string) ([]token, error) {
	var tokens []token
	for pos := 0; pos < len(src); {
		r := rune(src[pos])
		if unicode.IsSpace(r) {
			pos++
			continue
		}
		switch r {
		case ',':
			tokens = append(tokens, token{kind: tokenComma, text: ",", pos: pos})
			pos++
		case ':':
			tokens = append(tokens, token{kind: tokenColon, text: ":", pos: pos})
			pos++
		case '`':
			start := pos
			pos++
			for pos < len(src) && !isDelimiter(rune(src[pos])) {
				pos++
			}
			if pos == start+1 {
				return nil, fmt.Errorf("q parse error at %d: empty symbol", start)
			}
			tokens = append(tokens, token{kind: tokenSymbol, text: src[start+1 : pos], pos: start})
		case '"':
			start := pos
			pos++
			for pos < len(src) {
				if src[pos] == '\\' {
					pos += 2
					continue
				}
				if src[pos] == '"' {
					pos++
					break
				}
				pos++
			}
			if pos > len(src) || src[pos-1] != '"' {
				return nil, fmt.Errorf("q parse error at %d: unterminated string", start)
			}
			value, err := strconv.Unquote(src[start:pos])
			if err != nil {
				return nil, fmt.Errorf("q parse error at %d: invalid string literal", start)
			}
			tokens = append(tokens, token{kind: tokenString, text: value, pos: start})
		case '+', '-', '*', '/', '=', '<', '>':
			start := pos
			pos++
			if pos < len(src) && (src[start] == '<' || src[start] == '>') && src[pos] == '=' {
				pos++
			}
			if pos < len(src) && src[start] == '<' && src[pos] == '>' {
				pos++
			}
			tokens = append(tokens, token{kind: tokenOp, text: src[start:pos], pos: start})
		default:
			if isIdentStart(r) {
				start := pos
				pos++
				for pos < len(src) && isIdentPart(rune(src[pos])) {
					pos++
				}
				tokens = append(tokens, token{kind: tokenIdent, text: src[start:pos], pos: start})
				continue
			}
			if unicode.IsDigit(r) {
				start := pos
				pos++
				for pos < len(src) && (unicode.IsDigit(rune(src[pos])) || src[pos] == '.') {
					pos++
				}
				tokens = append(tokens, token{kind: tokenNumber, text: src[start:pos], pos: start})
				continue
			}
			return nil, fmt.Errorf("q parse error at %d: unexpected character %q", pos, r)
		}
	}
	tokens = append(tokens, token{kind: tokenEOF, pos: len(src)})
	return tokens, nil
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r)
}

func isDelimiter(r rune) bool {
	return unicode.IsSpace(r) || r == ',' || r == ':' || r == '+' || r == '-' || r == '*' || r == '/' || r == '=' || r == '<' || r == '>'
}
