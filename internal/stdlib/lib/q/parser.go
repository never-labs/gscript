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
	tokenSemicolon
	tokenBang
	tokenLParen
	tokenRParen
	tokenLBracket
	tokenRBracket
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
	take, err := p.parseTakePrefix()
	if err != nil {
		return nil, err
	}

	kindTok := p.peek()
	var kind QueryKind
	switch strings.ToLower(kindTok.text) {
	case string(SelectQuery):
		kind = SelectQuery
	case string(ExecQuery):
		kind = ExecQuery
	case string(UpdateQuery):
		kind = UpdateQuery
	case string(DeleteQuery):
		kind = DeleteQuery
	case string(InsertQuery):
		kind = InsertQuery
	case string(UpsertQuery):
		kind = UpsertQuery
	default:
		return nil, p.errorf(kindTok, "expected select, exec, update, delete, insert, or upsert")
	}
	p.next()
	if kind == InsertQuery || kind == UpsertQuery {
		return p.parseInsertOrUpsert(kind, false, take)
	}

	var distinct bool
	if kind == SelectQuery || kind == ExecQuery {
		distinct = p.consumeKeyword("distinct")
	}

	var columns []Column
	if (kind == SelectQuery || kind == ExecQuery) && p.isKeyword("from") {
		columns = []Column{{Name: "*", Expr: AllColumns{}}}
	} else if (kind == SelectQuery || kind == ExecQuery) && p.isKeyword("by") {
		columns = nil
	} else if kind == DeleteQuery && !p.isKeyword("from") {
		columns, err = p.parseDeleteColumnsUntil("from")
		if err != nil {
			return nil, err
		}
	} else if kind != DeleteQuery || !p.isKeyword("from") {
		columns, err = p.parseColumns()
		if err != nil {
			return nil, err
		}
	}
	if distinct {
		for i := range columns {
			columns[i].Expr = Call{Func: "distinct", Arg: columns[i].Expr}
		}
	}

	var by []Column
	if (kind == SelectQuery || kind == ExecQuery || kind == UpdateQuery) && p.consumeKeyword("by") {
		by, err = p.parseColumnsUntil("from")
		if err != nil {
			return nil, err
		}
	}

	if !p.consumeKeyword("from") {
		return nil, p.errorf(p.peek(), "expected from")
	}
	fromTok := p.peek()
	var from string
	var fromExpr Expr
	if fromTok.kind == tokenIdent && !strings.EqualFold(fromTok.text, "flip") {
		from = fromTok.text
		p.next()
	} else {
		fromExpr, err = p.parseExpr(0)
		if err != nil {
			return nil, p.errorf(fromTok, "expected table name or table literal: %v", err)
		}
	}

	var joins []Join
	if kind == SelectQuery || kind == ExecQuery {
		for {
			join, ok, joinErr := p.parseOptionalJoin()
			if joinErr != nil {
				return nil, joinErr
			}
			if !ok {
				break
			}
			joins = append(joins, *join)
		}
	}

	var where Expr
	if p.consumeKeyword("where") {
		where, err = p.parseWhereExpr()
		if err != nil {
			return nil, err
		}
	}

	var orderBy []OrderTerm
	if (kind == SelectQuery || kind == ExecQuery) && p.consumeKeyword("order") {
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
	if p.consumeKeyword("take") {
		if take != nil {
			return nil, p.errorf(p.peek(), "take specified more than once")
		}
		if limit != nil {
			return nil, p.errorf(p.peek(), "limit and take cannot both be specified")
		}
		n, err := p.parseLimit()
		if err != nil {
			return nil, err
		}
		take = &n
	}

	if p.peek().kind != tokenEOF {
		return nil, p.errorf(p.peek(), "unexpected token %q", p.peek().text)
	}

	return &Query{
		Kind:     kind,
		Distinct: distinct,
		Columns:  columns,
		By:       by,
		From:     from,
		FromExpr: fromExpr,
		Join:     firstJoinPtr(joins),
		Joins:    joins,
		Where:    where,
		OrderBy:  orderBy,
		Limit:    limit,
		Take:     take,
	}, nil
}

func (p *parser) parseInsertOrUpsert(kind QueryKind, distinct bool, take *int) (*Query, error) {
	if distinct || take != nil {
		return nil, p.errorf(p.peek(), "q %s does not support distinct or take", kind)
	}
	if !p.consumeKeyword("into") {
		return nil, p.errorf(p.peek(), "expected into after %s", kind)
	}
	fromTok := p.peek()
	if fromTok.kind != tokenIdent || isClauseKeyword(fromTok.text) {
		return nil, p.errorf(fromTok, "expected target table name")
	}
	p.next()

	var columns []Column
	var err error
	if p.consume(tokenLParen) {
		columns, err = p.parseInsertColumnList()
		if err != nil {
			return nil, err
		}
		if !p.consume(tokenRParen) {
			return nil, p.errorf(p.peek(), "expected ) after insert column list")
		}
	}

	if !p.consumeKeyword("values") {
		return nil, p.errorf(p.peek(), "expected values after insert target")
	}
	if !p.consume(tokenLParen) {
		return nil, p.errorf(p.peek(), "expected ( before values")
	}
	values, err := p.parseValuesList()
	if err != nil {
		return nil, err
	}
	if !p.consume(tokenRParen) {
		return nil, p.errorf(p.peek(), "expected ) after values")
	}
	if len(columns) > 0 && len(columns) != len(values) {
		return nil, p.errorf(p.peek(), "insert column count does not match values count")
	}
	if p.peek().kind != tokenEOF {
		return nil, p.errorf(p.peek(), "unexpected token %q", p.peek().text)
	}
	return &Query{
		Kind:    kind,
		Columns: columns,
		Values:  values,
		From:    fromTok.text,
	}, nil
}

func (p *parser) parseJoin(kind string) (*Join, error) {
	var bounds *WindowBounds
	var err error
	if kind == "window" || kind == "window1" {
		bounds, err = p.parseOptionalWindowBounds()
		if err != nil {
			return nil, err
		}
	}
	rightTok := p.peek()
	if rightTok.kind != tokenIdent {
		return nil, p.errorf(rightTok, "expected joined table name")
	}
	p.next()
	if !p.consumeKeyword("on") {
		return nil, p.errorf(p.peek(), "expected on after join table")
	}
	keys, err := p.parseJoinKeys()
	if err != nil {
		return nil, err
	}
	return &Join{Kind: kind, Right: rightTok.text, Keys: keys, Window: bounds}, nil
}

func (p *parser) parseOptionalJoin() (*Join, bool, error) {
	var (
		join *Join
		err  error
	)
	switch {
	case p.consumeKeyword("inner"):
		if !p.consumeKeyword("join") {
			return nil, false, p.errorf(p.peek(), "expected join after inner")
		}
		join, err = p.parseJoin("inner")
	case p.consumeKeyword("left"):
		if !p.consumeKeyword("join") {
			return nil, false, p.errorf(p.peek(), "expected join after left")
		}
		join, err = p.parseJoin("left")
	case p.consumeKeyword("lj"):
		join, err = p.parseJoin("left")
	case p.consumeKeyword("join"):
		join, err = p.parseJoin("inner")
	case p.consumeKeyword("ij"):
		join, err = p.parseJoin("inner")
	case p.consumeKeyword("uj"):
		join, err = p.parseJoin("union")
	case p.consumeKeyword("pj"):
		join, err = p.parseJoin("plus")
	case p.consumeKeyword("wj"):
		join, err = p.parseJoin("window")
	case p.consumeKeyword("wj1"):
		join, err = p.parseJoin("window1")
	case p.consumeKeyword("aj"):
		join, err = p.parseJoin("asof")
	case p.consumeKeyword("aj0"):
		join, err = p.parseJoin("asof0")
	case p.consumeKeyword("ajf"):
		join, err = p.parseJoin("asof_fill")
	case p.consumeKeyword("ajf0"):
		join, err = p.parseJoin("asof_fill0")
	case p.consumeKeyword("asof"):
		if !p.consumeKeyword("join") {
			return nil, false, p.errorf(p.peek(), "expected join after asof")
		}
		join, err = p.parseJoin("asof")
	default:
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return join, true, nil
}

func firstJoinPtr(joins []Join) *Join {
	if len(joins) == 0 {
		return nil
	}
	join := joins[0]
	return &join
}

func (p *parser) parseOptionalWindowBounds() (*WindowBounds, error) {
	if !p.consume(tokenLBracket) {
		return nil, nil
	}
	low, err := p.parseWindowBound()
	if err != nil {
		return nil, err
	}
	p.consume(tokenComma)
	high, err := p.parseWindowBound()
	if err != nil {
		return nil, err
	}
	if !p.consume(tokenRBracket) {
		return nil, p.errorf(p.peek(), "expected ] after window bounds")
	}
	return &WindowBounds{Low: low, High: high}, nil
}

func (p *parser) parseWindowBound() (Expr, error) {
	sign := int64(1)
	if p.peek().kind == tokenOp && (p.peek().text == "-" || p.peek().text == "+") {
		if p.peek().text == "-" {
			sign = -1
		}
		p.next()
	}
	tok := p.peek()
	if tok.kind != tokenNumber {
		return nil, p.errorf(tok, "expected window bound literal")
	}
	p.next()
	var expr Expr
	if kind, text, ok := parseTemporalToken(tok.text); ok {
		expr = Temporal{Kind: kind, Text: text}
	} else {
		expr = Number{Text: tok.text}
	}
	if sign < 0 {
		return Binary{Op: "-", Left: Number{Text: "0"}, Right: expr}, nil
	}
	return expr, nil
}

func (p *parser) parseJoinKeys() ([]JoinKey, error) {
	var keys []JoinKey
	for {
		leftTok := p.peek()
		if leftTok.kind != tokenIdent || isClauseKeyword(leftTok.text) {
			return nil, p.errorf(leftTok, "expected join key column")
		}
		p.next()
		key := JoinKey{Left: leftTok.text, Right: leftTok.text}
		if p.peek().kind == tokenOp && p.peek().text == "=" {
			p.next()
			rightTok := p.peek()
			if rightTok.kind != tokenIdent || isClauseKeyword(rightTok.text) {
				return nil, p.errorf(rightTok, "expected right join key column")
			}
			p.next()
			key.Right = rightTok.text
		}
		keys = append(keys, key)
		if !p.consume(tokenComma) {
			break
		}
	}
	if len(keys) == 0 {
		return nil, p.errorf(p.peek(), "expected at least one join key")
	}
	return keys, nil
}

func (p *parser) parseTakePrefix() (*int, error) {
	if p.consumeKeyword("take") {
		n, err := p.parseLimit()
		if err != nil {
			return nil, err
		}
		return &n, nil
	}
	if p.peek().kind == tokenNumber && p.look(1).kind == tokenOp && p.look(1).text == "#" {
		tok := p.next()
		p.next()
		n, err := parseNonNegativeInt(tok)
		if err != nil {
			return nil, p.errorf(tok, "expected non-negative integer take")
		}
		return &n, nil
	}
	return nil, nil
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
	if p.peek().kind == tokenOp && p.peek().text == "*" {
		p.next()
		return Column{Name: "*", Expr: AllColumns{}}, nil
	}
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

func (p *parser) parseColumnsUntil(keyword string) ([]Column, error) {
	var columns []Column
	for {
		if p.peek().kind == tokenEOF || p.isKeyword(keyword) {
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
		return nil, p.errorf(p.peek(), "expected expression before %s", keyword)
	}
	return columns, nil
}

func (p *parser) parseDeleteColumnsUntil(keyword string) ([]Column, error) {
	var columns []Column
	for {
		if p.peek().kind == tokenEOF || p.isKeyword(keyword) {
			break
		}
		tok := p.peek()
		if tok.kind != tokenIdent || isClauseKeyword(tok.text) {
			return nil, p.errorf(tok, "expected delete column name")
		}
		if p.look(1).kind == tokenColon || p.look(1).kind == tokenOp {
			column, err := p.parseColumn()
			if err != nil {
				return nil, err
			}
			columns = append(columns, column)
		} else {
			p.next()
			columns = append(columns, Column{Name: tok.text, Expr: Ident{Name: tok.text}})
		}
		if p.consume(tokenComma) && (p.peek().kind == tokenEOF || p.isKeyword(keyword)) {
			return nil, p.errorf(p.peek(), "expected delete column name")
		}
	}
	if len(columns) == 0 {
		return nil, p.errorf(p.peek(), "expected column before %s", keyword)
	}
	return columns, nil
}

func (p *parser) parseInsertColumnList() ([]Column, error) {
	var columns []Column
	for {
		tok := p.peek()
		if tok.kind != tokenIdent || isClauseKeyword(tok.text) {
			return nil, p.errorf(tok, "expected insert column name")
		}
		p.next()
		columns = append(columns, Column{Name: tok.text, Expr: Ident{Name: tok.text}})
		if !p.consume(tokenComma) {
			break
		}
		if p.peek().kind == tokenRParen {
			return nil, p.errorf(p.peek(), "expected insert column name")
		}
	}
	if len(columns) == 0 {
		return nil, p.errorf(p.peek(), "expected insert column name")
	}
	return columns, nil
}

func (p *parser) parseValuesList() ([]Expr, error) {
	var values []Expr
	for {
		if p.peek().kind == tokenRParen || p.peek().kind == tokenEOF {
			break
		}
		value, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		if !p.consume(tokenComma) {
			break
		}
		if p.peek().kind == tokenRParen {
			return nil, p.errorf(p.peek(), "expected value expression")
		}
	}
	if len(values) == 0 {
		return nil, p.errorf(p.peek(), "expected value expression")
	}
	return values, nil
}

func (p *parser) parseWhereExpr() (Expr, error) {
	expr, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	for p.consume(tokenComma) || p.consume(tokenSemicolon) {
		if p.peek().kind == tokenEOF || p.isKeyword("order") || p.isKeyword("limit") || p.isKeyword("take") {
			return nil, p.errorf(p.peek(), "expected where expression")
		}
		next, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		expr = Binary{Op: "and", Left: expr, Right: next}
	}
	return expr, nil
}

func (p *parser) parseOrderBy() ([]OrderTerm, error) {
	var terms []OrderTerm
	for {
		if p.peek().kind == tokenEOF || p.isKeyword("limit") || p.isKeyword("take") {
			break
		}
		expr, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		name := columnName(expr)
		term := OrderTerm{Column: name, Expr: expr}
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
	n, err := parseNonNegativeInt(tok)
	if err != nil {
		return 0, p.errorf(tok, "expected non-negative integer limit")
	}
	return n, nil
}

func parseNonNegativeInt(tok token) (int, error) {
	n, err := strconv.Atoi(tok.text)
	if err != nil || n < 0 || strconv.Itoa(n) != tok.text {
		return 0, fmt.Errorf("invalid integer")
	}
	return n, nil
}

func (p *parser) parseExpr(minPrec int) (Expr, error) {
	left, err := p.parseVector()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		if tok.kind == tokenBang {
			if minPrec > 5 {
				break
			}
			p.next()
			right, err := p.parseExpr(6)
			if err != nil {
				return nil, err
			}
			left = DictExpr{Keys: left, Values: right}
			continue
		}
		if tok.kind == tokenIdent {
			op := strings.ToLower(tok.text)
			if qSQLDyadicFunction(op) {
				p.next()
				right, err := p.parseExpr(0)
				if err != nil {
					return nil, err
				}
				left = Call{Func: tok.text, Arg: Vector{Items: []Expr{left, right}}}
				continue
			}
			if op != "in" && op != "within" && op != "and" && op != "or" {
				break
			}
			prec := precedence(op)
			if prec < minPrec {
				break
			}
			p.next()
			right, err := p.parseExpr(prec + 1)
			if err != nil {
				return nil, err
			}
			left = Binary{Op: op, Left: left, Right: right}
			continue
		}
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

func (p *parser) parseVector() (Expr, error) {
	first, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	items := []Expr{first}
	for canStartVectorItem(p.peek()) {
		item, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if len(items) == 1 {
		return first, nil
	}
	return Vector{Items: items}, nil
}

func (p *parser) parsePostfix() (Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.consume(tokenLBracket) {
		index, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		if !p.consume(tokenRBracket) {
			return nil, p.errorf(p.peek(), "expected ]")
		}
		expr = IndexExpr{Expr: expr, Index: index}
	}
	return expr, nil
}

func (p *parser) parsePrimary() (Expr, error) {
	tok := p.peek()
	if tok.kind == tokenOp && tok.text == "?" {
		return p.parseConditionalCall()
	}
	if tok.kind == tokenOp && (tok.text == "-" || tok.text == "+") {
		p.next()
		expr, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		if tok.text == "-" {
			return Binary{Op: "-", Left: Number{Text: "0"}, Right: expr}, nil
		}
		return expr, nil
	}
	switch tok.kind {
	case tokenNumber:
		p.next()
		if kind, ok := parseTypedNullTokenKind(tok.text); ok {
			return TypedNull{Kind: kind}, nil
		}
		if kind, text, ok := parseTemporalToken(tok.text); ok {
			if strings.HasPrefix(strings.ToLower(text), "0n") {
				return TypedNull{Kind: kind}, nil
			}
			return Temporal{Kind: kind, Text: text}, nil
		}
		return Number{Text: tok.text}, nil
	case tokenString:
		p.next()
		return String{Value: tok.text}, nil
	case tokenSymbol:
		p.next()
		return Symbol{Name: tok.text}, nil
	case tokenIdent:
		p.next()
		if qPrefixFunction(tok.text) && p.canStartCallArg() {
			arg, err := p.parseExpr(11)
			if err != nil {
				return nil, err
			}
			return Call{Func: tok.text, Arg: arg}, nil
		}
		switch strings.ToLower(tok.text) {
		case "true":
			return Bool{Value: true}, nil
		case "false":
			return Bool{Value: false}, nil
		case "null", "nil":
			return Null{}, nil
		case "not":
			arg, err := p.parseExpr(11)
			if err != nil {
				return nil, err
			}
			return Call{Func: "not", Arg: arg}, nil
		}
		if strings.EqualFold(tok.text, "xbar") {
			arg, err := p.parseXbarArgs()
			if err != nil {
				return nil, err
			}
			return Call{Func: tok.text, Arg: arg}, nil
		}
		if strings.EqualFold(tok.text, "flip") {
			arg, err := p.parseFlipDictExpr()
			if err != nil {
				return nil, err
			}
			flip, err := flipFromDictExpr(arg)
			if err != nil {
				return nil, p.errorf(tok, "%s", err)
			}
			return flip, nil
		}
		return Ident{Name: tok.text}, nil
	case tokenLParen:
		return p.parseParenExpr()
	default:
		return nil, p.errorf(tok, "expected expression")
	}
}

func (p *parser) parseConditionalCall() (Expr, error) {
	p.next()
	if !p.consume(tokenLBracket) {
		return nil, p.errorf(p.peek(), "expected [ after ?")
	}
	cond, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if !p.consume(tokenSemicolon) {
		return nil, p.errorf(p.peek(), "expected ; after conditional condition")
	}
	thenExpr, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if !p.consume(tokenSemicolon) {
		return nil, p.errorf(p.peek(), "expected ; after conditional then expression")
	}
	elseExpr, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if !p.consume(tokenRBracket) {
		return nil, p.errorf(p.peek(), "expected ] after conditional")
	}
	return Call{Func: "?", Arg: Vector{Items: []Expr{cond, thenExpr, elseExpr}}}, nil
}

func (p *parser) parseXbarArgs() (Expr, error) {
	if !p.canStartCallArg() {
		return nil, p.errorf(p.peek(), "expected xbar interval")
	}
	interval, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if !p.canStartCallArg() {
		return nil, p.errorf(p.peek(), "expected xbar expression")
	}
	expr, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	return Vector{Items: []Expr{interval, expr}}, nil
}

func (p *parser) parseParenExpr() (Expr, error) {
	p.next()
	if p.consume(tokenLBracket) {
		var keys []Column
		var err error
		if !p.consume(tokenRBracket) {
			keys, err = p.parseTableLiteralColumns(tokenRBracket, "keyed table key")
			if err != nil {
				return nil, err
			}
			if !p.consume(tokenRBracket) {
				return nil, p.errorf(p.peek(), "expected ] in keyed table literal")
			}
		}
		columns, err := p.parseTableLiteralColumns(tokenRParen, "table column")
		if err != nil {
			return nil, err
		}
		if !p.consume(tokenRParen) {
			return nil, p.errorf(p.peek(), "expected ) after flip table literal")
		}
		if len(keys) == 0 && len(columns) == 0 {
			return nil, p.errorf(p.peek(), "expected at least one table literal column")
		}
		return Flip{Keys: keys, Columns: columns}, nil
	}
	expr, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.consume(tokenSemicolon) {
		items := []Expr{expr}
		for {
			if p.peek().kind == tokenRParen {
				break
			}
			item, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
			if !p.consume(tokenSemicolon) {
				break
			}
		}
		if !p.consume(tokenRParen) {
			return nil, p.errorf(p.peek(), "expected )")
		}
		return Vector{Items: items}, nil
	}
	if !p.consume(tokenRParen) {
		return nil, p.errorf(p.peek(), "expected )")
	}
	return expr, nil
}

func (p *parser) parseFlipDictExpr() (Expr, error) {
	keys, err := p.parseFlipDictKeys()
	if err != nil {
		return nil, err
	}
	names, err := symbolColumnNames(keys)
	if err != nil {
		return nil, p.errorf(p.peek(), "%s", err)
	}
	if !p.consume(tokenBang) {
		return nil, p.errorf(p.peek(), "expected ! after flip dictionary keys")
	}
	values, err := p.parseFlipDictValues(len(names))
	if err != nil {
		return nil, err
	}
	return DictExpr{Keys: keys, Values: values}, nil
}

func (p *parser) parseFlipDictKeys() (Expr, error) {
	first, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	items := []Expr{first}
	for p.peek().kind == tokenSymbol {
		item, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if len(items) == 1 {
		return first, nil
	}
	return Vector{Items: items}, nil
}

func (p *parser) parseFlipDictValues(columns int) (Expr, error) {
	if columns <= 1 {
		return p.parseExpr(6)
	}
	if !p.consume(tokenLParen) {
		return nil, p.errorf(p.peek(), "expected parenthesized value list for multi-column flip")
	}
	var items []Expr
	for {
		if p.peek().kind == tokenRParen || p.peek().kind == tokenEOF {
			break
		}
		item, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if !p.consume(tokenSemicolon) {
			break
		}
	}
	if !p.consume(tokenRParen) {
		return nil, p.errorf(p.peek(), "expected ) after flip dictionary values")
	}
	if len(items) != columns {
		return nil, p.errorf(p.peek(), "flip dictionary key/value count mismatch")
	}
	return Vector{Items: items}, nil
}

func flipFromDictExpr(expr Expr) (Flip, error) {
	dict, ok := expr.(DictExpr)
	if !ok {
		return Flip{}, fmt.Errorf("flip expects a dictionary expression")
	}
	names, err := symbolColumnNames(dict.Keys)
	if err != nil {
		return Flip{}, err
	}
	values, ok := dict.Values.(Vector)
	if !ok {
		values = Vector{Items: []Expr{dict.Values}}
	}
	if len(names) != len(values.Items) {
		return Flip{}, fmt.Errorf("flip dictionary key/value count mismatch")
	}
	columns := make([]Column, len(names))
	for i, name := range names {
		columns[i] = Column{Name: name, Expr: values.Items[i]}
	}
	return Flip{Columns: columns}, nil
}

func symbolColumnNames(expr Expr) ([]string, error) {
	switch x := expr.(type) {
	case Symbol:
		if x.Name == "" || strings.HasPrefix(x.Name, ":") {
			return nil, fmt.Errorf("flip dictionary keys must be column symbols")
		}
		return []string{x.Name}, nil
	case Vector:
		names := make([]string, len(x.Items))
		for i, item := range x.Items {
			sym, ok := item.(Symbol)
			if !ok || sym.Name == "" || strings.HasPrefix(sym.Name, ":") {
				return nil, fmt.Errorf("flip dictionary keys must be column symbols")
			}
			names[i] = sym.Name
		}
		return names, nil
	default:
		return nil, fmt.Errorf("flip dictionary keys must be column symbols")
	}
}

func (p *parser) parseTableLiteralColumns(end tokenKind, label string) ([]Column, error) {
	var columns []Column
	for p.peek().kind != tokenEOF && p.peek().kind != end {
		if p.consume(tokenSemicolon) {
			continue
		}
		if p.peek().kind != tokenIdent || p.look(1).kind != tokenColon {
			return nil, p.errorf(p.peek(), "expected %s assignment", label)
		}
		name := p.next().text
		p.next()
		expr, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		columns = append(columns, Column{Name: name, Expr: expr})
		if p.peek().kind == end {
			break
		}
		if !p.consume(tokenSemicolon) {
			return nil, p.errorf(p.peek(), "expected ; or closing delimiter after %s", label)
		}
	}
	return columns, nil
}

func columnName(expr Expr) string {
	switch x := expr.(type) {
	case Ident:
		return x.Name
	case Binary:
		left := columnName(x.Left)
		right := columnName(x.Right)
		if left == "" || right == "" {
			return ""
		}
		return left + x.Op + right
	case DictExpr:
		left := columnName(x.Keys)
		right := columnName(x.Values)
		if left == "" || right == "" {
			return ""
		}
		return left + "!" + right
	case Call:
		if x.Func == "?" {
			return "?"
		}
		arg := columnName(x.Arg)
		if arg == "" {
			return ""
		}
		return x.Func + " " + arg
	}
	return ""
}

func canStartPrimary(tok token) bool {
	return tok.kind == tokenIdent || tok.kind == tokenNumber || tok.kind == tokenString || tok.kind == tokenSymbol || tok.kind == tokenLParen || (tok.kind == tokenOp && (tok.text == "-" || tok.text == "+" || tok.text == "?"))
}

func canStartVectorItem(tok token) bool {
	if tok.kind == tokenIdent {
		switch strings.ToLower(tok.text) {
		case "true", "false", "null", "nil":
			return true
		default:
			return false
		}
	}
	return tok.kind == tokenNumber || tok.kind == tokenString || tok.kind == tokenSymbol
}

func (p *parser) canStartCallArg() bool {
	tok := p.peek()
	if tok.kind == tokenIdent {
		switch strings.ToLower(tok.text) {
		case "asc", "by", "delete", "desc", "distinct", "from", "insert", "into", "limit", "order", "select", "take", "update", "upsert", "values", "where":
			return false
		}
	}
	return canStartPrimary(tok)
}

func qPrefixFunction(name string) bool {
	switch strings.ToLower(name) {
	case "sum", "avg", "var", "dev", "med", "wavg", "count", "min", "max", "first", "last", "distinct",
		"prev", "next", "deltas", "fills", "ratios", "sums", "prds", "mins", "maxs", "avgs",
		"abs", "neg", "sqrt", "log", "exp", "reciprocal", "signum", "floor", "ceiling", "null", "rank":
		return true
	default:
		return false
	}
}

func qSQLDyadicFunction(name string) bool {
	switch strings.ToLower(name) {
	case "xbar", "xprev", "msum", "mavg", "mcount", "mmin", "mmax":
		return true
	default:
		return false
	}
}

func isClauseKeyword(name string) bool {
	switch strings.ToLower(name) {
	case "asc", "by", "delete", "desc", "distinct", "exec", "from", "insert", "into", "join", "limit", "order", "select", "take", "update", "upsert", "values", "where":
		return true
	default:
		return false
	}
}

func precedence(op string) int {
	switch op {
	case "or", "|":
		return 2
	case "and", "&":
		return 3
	case "in", "within":
		return 10
	case "*", "/", "%":
		return 30
	case "#":
		return 25
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
		case ';':
			tokens = append(tokens, token{kind: tokenSemicolon, text: ";", pos: pos})
			pos++
		case '!':
			tokens = append(tokens, token{kind: tokenBang, text: "!", pos: pos})
			pos++
		case '(':
			tokens = append(tokens, token{kind: tokenLParen, text: "(", pos: pos})
			pos++
		case ')':
			tokens = append(tokens, token{kind: tokenRParen, text: ")", pos: pos})
			pos++
		case '[':
			tokens = append(tokens, token{kind: tokenLBracket, text: "[", pos: pos})
			pos++
		case ']':
			tokens = append(tokens, token{kind: tokenRBracket, text: "]", pos: pos})
			pos++
		case '`':
			start := pos
			pos++
			if pos < len(src) && src[pos] == ':' {
				pos++
				for pos < len(src) && !isPathSymbolDelimiter(rune(src[pos])) {
					pos++
				}
				tokens = append(tokens, token{kind: tokenSymbol, text: src[start+1 : pos], pos: start})
				continue
			}
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
		case '+', '-', '*', '/', '%', '=', '<', '>', '#', '?', '&', '|':
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
				for pos < len(src) && isQLiteralPartAt(src, start, pos) {
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

func isQLiteralPart(r rune) bool {
	return unicode.IsDigit(r) || r == '.' || r == ':' || r == 'D' || r == 'N' || r == 'T' || r == 'W' || r == 'Z' || r == 'b' || r == 'c' || r == 'd' || r == 'e' || r == 'f' || r == 'h' || r == 'i' || r == 'j' || r == 'm' || r == 'n' || r == 'p' || r == 't' || r == 'u' || r == 'v' || r == 'w' || r == 'x' || r == 'z'
}

func isQLiteralPartAt(src string, start, pos int) bool {
	r := rune(src[pos])
	if isQLiteralPart(r) {
		return true
	}
	if r != '-' {
		return false
	}
	offset := pos - start
	if offset != 4 && offset != 7 {
		return false
	}
	for i := start; i < pos; i++ {
		if src[i] != '-' && !unicode.IsDigit(rune(src[i])) {
			return false
		}
	}
	return true
}

func isDelimiter(r rune) bool {
	return unicode.IsSpace(r) || r == '`' || r == ',' || r == ':' || r == ';' || r == '!' || r == '(' || r == ')' || r == '[' || r == ']' || r == '+' || r == '-' || r == '*' || r == '/' || r == '%' || r == '=' || r == '<' || r == '>' || r == '&' || r == '|'
}

func isPathSymbolDelimiter(r rune) bool {
	return unicode.IsSpace(r) || r == '`' || r == ',' || r == ';' || r == '!' || r == '(' || r == ')' || r == '[' || r == ']'
}
