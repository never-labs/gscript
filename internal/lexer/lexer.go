package lexer

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

// Lexer tokenizes Leia source code.
type Lexer struct {
	source          string
	pos             int
	line            int
	col             int
	tokens          []Token
	pendingComments []Comment
	lastTokenLine   int
}

// New creates a new Lexer for the given source string.
func New(source string) *Lexer {
	return &Lexer{
		source: source,
		pos:    0,
		line:   1,
		col:    1,
	}
}

// peek returns the current character without advancing, or 0 if at end.
func (l *Lexer) peek() byte {
	if l.pos >= len(l.source) {
		return 0
	}
	return l.source[l.pos]
}

// peekAt returns the character at pos+offset, or 0 if out of bounds.
func (l *Lexer) peekAt(offset int) byte {
	idx := l.pos + offset
	if idx >= len(l.source) {
		return 0
	}
	return l.source[idx]
}

// advance moves forward one character and updates line/col tracking.
func (l *Lexer) advance() byte {
	if l.pos >= len(l.source) {
		return 0
	}
	ch := l.source[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

// skipWhitespace skips spaces, tabs, carriage returns, and newlines.
func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) skipWhitespaceAndComments() error {
	for {
		l.skipWhitespace()

		if l.peek() == '/' && l.peekAt(1) == '/' {
			comment := l.readLineComment()
			if comment.Line != l.lastTokenLine {
				l.pendingComments = append(l.pendingComments, comment)
			}
			continue
		}

		if l.peek() == '/' && l.peekAt(1) == '*' {
			l.pendingComments = nil
			if err := l.skipBlockComment(); err != nil {
				return err
			}
			continue
		}

		return nil
	}
}

// NextToken returns the next token from the source.
// After all tokens are consumed, it returns TOKEN_EOF.
func (l *Lexer) NextToken() Token {
	tok, err := l.nextTokenInternal()
	if err != nil {
		return Token{Type: TOKEN_ILLEGAL, Value: err.Error(), Line: l.line, Column: l.col}
	}
	return tok
}

// nextTokenInternal returns the next token or an error.
func (l *Lexer) nextTokenInternal() (Token, error) {
	if err := l.skipWhitespaceAndComments(); err != nil {
		return Token{}, err
	}

	if l.pos >= len(l.source) {
		return Token{Type: TOKEN_EOF, Value: "", Line: l.line, Column: l.col}, nil
	}

	ch := l.peek()
	startLine := l.line
	startCol := l.col

	// String literal
	if ch == '"' {
		tok, err := l.readString()
		if err != nil {
			return Token{}, err
		}
		return l.finishToken(tok), nil
	}
	if ch == '`' {
		tok, err := l.readRawString()
		if err != nil {
			return Token{}, err
		}
		return l.finishToken(tok), nil
	}

	// Number literal
	if isDigit(ch) {
		tok, err := l.readNumber()
		if err != nil {
			return Token{}, err
		}
		return l.finishToken(tok), nil
	}

	// Identifier or keyword
	if isLetter(ch) {
		tok, err := l.readIdentifier()
		if err != nil {
			return Token{}, err
		}
		return l.finishToken(tok), nil
	}

	// Operators and separators
	tok, err := l.readOperator(startLine, startCol)
	if err != nil {
		return Token{}, err
	}
	return l.finishToken(tok), nil
}

func (l *Lexer) finishToken(tok Token) Token {
	if tok.Type != TOKEN_EOF && len(l.pendingComments) > 0 {
		tok.LeadingComments = append([]Comment(nil), l.pendingComments...)
	}
	l.pendingComments = nil
	if tok.Type != TOKEN_EOF {
		l.lastTokenLine = tok.Line
	}
	return tok
}

// readLineComment reads from // to end of line.
func (l *Lexer) readLineComment() Comment {
	startLine := l.line
	startCol := l.col
	// consume the //
	l.advance()
	l.advance()
	start := l.pos
	for l.pos < len(l.source) && l.peek() != '\n' {
		l.advance()
	}
	return Comment{Text: l.source[start:l.pos], Line: startLine, Column: startCol}
}

// skipBlockComment skips from /* to */. Returns error if unterminated.
func (l *Lexer) skipBlockComment() error {
	startLine := l.line
	startCol := l.col
	// consume /*
	l.advance()
	l.advance()
	for l.pos < len(l.source) {
		if l.peek() == '*' && l.peekAt(1) == '/' {
			l.advance() // *
			l.advance() // /
			return nil
		}
		l.advance()
	}
	return fmt.Errorf("unterminated block comment starting at %d:%d", startLine, startCol)
}

// readString reads a double-quoted string literal with escape sequences.
func (l *Lexer) readString() (Token, error) {
	startLine := l.line
	startCol := l.col
	l.advance() // consume opening "

	var result []byte
	for l.pos < len(l.source) {
		ch := l.peek()
		if ch == '\n' {
			return Token{}, fmt.Errorf("unterminated string at %d:%d: newline in string literal", startLine, startCol)
		}
		if ch == '"' {
			l.advance() // consume closing "
			return Token{Type: TOKEN_STRING, Value: string(result), Line: startLine, Column: startCol}, nil
		}
		if ch == '\\' {
			l.advance() // consume backslash
			if l.pos >= len(l.source) {
				return Token{}, fmt.Errorf("unterminated string at %d:%d: unexpected end after escape", startLine, startCol)
			}
			esc := l.advance()
			switch esc {
			case 'a':
				result = append(result, '\a')
			case 'b':
				result = append(result, '\b')
			case 'f':
				result = append(result, '\f')
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case 'v':
				result = append(result, '\v')
			case '\\':
				result = append(result, '\\')
			case '"':
				result = append(result, '"')
			case 'x':
				b, err := l.readFixedHexEscape(2, startLine, startCol)
				if err != nil {
					return Token{}, err
				}
				result = append(result, byte(b))
			case 'u':
				r, err := l.readFixedHexEscape(4, startLine, startCol)
				if err != nil {
					return Token{}, err
				}
				if !utf8.ValidRune(rune(r)) {
					return Token{}, fmt.Errorf("invalid unicode escape at %d:%d", startLine, startCol)
				}
				result = append(result, string(rune(r))...)
			case 'U':
				r, err := l.readFixedHexEscape(8, startLine, startCol)
				if err != nil {
					return Token{}, err
				}
				if !utf8.ValidRune(rune(r)) {
					return Token{}, fmt.Errorf("invalid unicode escape at %d:%d", startLine, startCol)
				}
				result = append(result, string(rune(r))...)
			default:
				if isDigit(esc) {
					b, err := l.readDecimalByteEscape(esc, startLine, startCol)
					if err != nil {
						return Token{}, err
					}
					result = append(result, byte(b))
					continue
				}
				// Unrecognized escape: keep backslash and character
				result = append(result, '\\', esc)
			}
			continue
		}
		result = append(result, l.advance())
	}
	return Token{}, fmt.Errorf("unterminated string at %d:%d", startLine, startCol)
}

func (l *Lexer) readFixedHexEscape(width int, startLine, startCol int) (int64, error) {
	start := l.pos
	for i := 0; i < width; i++ {
		if !isHexDigit(l.peekAt(i)) {
			return 0, fmt.Errorf("invalid hex escape at %d:%d", startLine, startCol)
		}
	}
	for i := 0; i < width; i++ {
		l.advance()
	}
	v, err := strconv.ParseInt(l.source[start:l.pos], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid hex escape at %d:%d", startLine, startCol)
	}
	return v, nil
}

func (l *Lexer) readDecimalByteEscape(first byte, startLine, startCol int) (int64, error) {
	digits := []byte{first}
	for len(digits) < 3 && isDigit(l.peek()) {
		digits = append(digits, l.advance())
	}
	v, err := strconv.ParseInt(string(digits), 10, 16)
	if err != nil || v > 255 {
		return 0, fmt.Errorf("invalid decimal byte escape at %d:%d", startLine, startCol)
	}
	return v, nil
}

// readRawString reads a Go-style backtick string literal.
func (l *Lexer) readRawString() (Token, error) {
	startLine := l.line
	startCol := l.col
	l.advance() // consume opening `

	startPos := l.pos
	for l.pos < len(l.source) {
		if l.peek() == '`' {
			value := l.source[startPos:l.pos]
			l.advance() // consume closing `
			return Token{Type: TOKEN_STRING, Value: value, Line: startLine, Column: startCol}, nil
		}
		l.advance()
	}
	return Token{}, fmt.Errorf("unterminated raw string at %d:%d", startLine, startCol)
}

// readNumber reads a Go-style integer or floating-point number.
func (l *Lexer) readNumber() (Token, error) {
	startLine := l.line
	startCol := l.col
	startPos := l.pos

	if l.peek() == '0' && isIntBasePrefix(l.peekAt(1)) {
		l.advance() // consume 0
		l.advance() // consume base prefix
		for l.pos < len(l.source) && isBaseLiteralChar(l.peek()) {
			l.advance()
		}
		value := l.source[startPos:l.pos]
		return Token{Type: TOKEN_NUMBER, Value: value, Line: startLine, Column: startCol}, nil
	}

	// Read integer digits.
	for l.pos < len(l.source) && isDigitOrUnderscore(l.peek()) {
		l.advance()
	}

	// Check for decimal point — but not if followed by another dot (which is CONCAT "..")
	if l.pos < len(l.source) && l.peek() == '.' && l.peekAt(1) != '.' {
		l.advance() // consume .
		for l.pos < len(l.source) && isDigitOrUnderscore(l.peek()) {
			l.advance()
		}
	}

	// Check for exponent
	if l.pos < len(l.source) && (l.peek() == 'e' || l.peek() == 'E') {
		l.advance() // consume e/E
		if l.pos < len(l.source) && (l.peek() == '+' || l.peek() == '-') {
			l.advance() // consume sign
		}
		for l.pos < len(l.source) && isDigitOrUnderscore(l.peek()) {
			l.advance()
		}
	}

	value := l.source[startPos:l.pos]
	return Token{Type: TOKEN_NUMBER, Value: value, Line: startLine, Column: startCol}, nil
}

// readIdentifier reads an identifier or keyword.
func (l *Lexer) readIdentifier() (Token, error) {
	startLine := l.line
	startCol := l.col
	startPos := l.pos

	for l.pos < len(l.source) && isLetterOrDigit(l.peek()) {
		l.advance()
	}

	value := l.source[startPos:l.pos]
	tokType := LookupIdent(value)
	return Token{Type: tokType, Value: value, Line: startLine, Column: startCol}, nil
}

// readOperator reads operators and separator tokens.
func (l *Lexer) readOperator(startLine, startCol int) (Token, error) {
	ch := l.peek()

	makeToken := func(typ TokenType, val string) Token {
		return Token{Type: typ, Value: val, Line: startLine, Column: startCol}
	}

	switch ch {
	case '(':
		l.advance()
		return makeToken(TOKEN_LPAREN, "("), nil
	case ')':
		l.advance()
		return makeToken(TOKEN_RPAREN, ")"), nil
	case '{':
		l.advance()
		return makeToken(TOKEN_LBRACE, "{"), nil
	case '}':
		l.advance()
		return makeToken(TOKEN_RBRACE, "}"), nil
	case '[':
		l.advance()
		return makeToken(TOKEN_LBRACKET, "["), nil
	case ']':
		l.advance()
		return makeToken(TOKEN_RBRACKET, "]"), nil
	case ',':
		l.advance()
		return makeToken(TOKEN_COMMA, ","), nil
	case ';':
		l.advance()
		return makeToken(TOKEN_SEMICOLON, ";"), nil
	case '#':
		l.advance()
		return makeToken(TOKEN_LEN, "#"), nil
	case '%':
		l.advance()
		return makeToken(TOKEN_PERCENT, "%"), nil
	case '$':
		l.advance()
		return makeToken(TOKEN_DOLLAR, "$"), nil

	case ':':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_DECLARE, ":="), nil
		}
		return makeToken(TOKEN_COLON, ":"), nil

	case '=':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_EQ, "=="), nil
		}
		return makeToken(TOKEN_ASSIGN, "="), nil

	case '!':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_NEQ, "!="), nil
		}
		return makeToken(TOKEN_NOT, "!"), nil

	case '<':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_LE, "<="), nil
		}
		if l.peek() == '<' {
			l.advance()
			return makeToken(TOKEN_SHL, "<<"), nil
		}
		if l.peek() == '-' {
			l.advance()
			return makeToken(TOKEN_ARROW, "<-"), nil
		}
		return makeToken(TOKEN_LT, "<"), nil

	case '>':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_GE, ">="), nil
		}
		if l.peek() == '>' {
			l.advance()
			return makeToken(TOKEN_SHR, ">>"), nil
		}
		return makeToken(TOKEN_GT, ">"), nil

	case '+':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_PLUS_ASSIGN, "+="), nil
		}
		if l.peek() == '+' {
			l.advance()
			return makeToken(TOKEN_INC, "++"), nil
		}
		return makeToken(TOKEN_PLUS, "+"), nil

	case '-':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_MINUS_ASSIGN, "-="), nil
		}
		if l.peek() == '-' {
			l.advance()
			return makeToken(TOKEN_DEC, "--"), nil
		}
		return makeToken(TOKEN_MINUS, "-"), nil

	case '*':
		l.advance()
		if l.peek() == '*' {
			l.advance()
			return makeToken(TOKEN_POW, "**"), nil
		}
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_STAR_ASSIGN, "*="), nil
		}
		return makeToken(TOKEN_STAR, "*"), nil

	case '/':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return makeToken(TOKEN_SLASH_ASSIGN, "/="), nil
		}
		return makeToken(TOKEN_SLASH, "/"), nil

	case '&':
		l.advance()
		if l.peek() == '&' {
			l.advance()
			return makeToken(TOKEN_AND, "&&"), nil
		}
		if l.peek() == '^' {
			l.advance()
			return makeToken(TOKEN_BIT_AND_NOT, "&^"), nil
		}
		return makeToken(TOKEN_BIT_AND, "&"), nil

	case '|':
		l.advance()
		if l.peek() == '|' {
			l.advance()
			return makeToken(TOKEN_OR, "||"), nil
		}
		return makeToken(TOKEN_BIT_OR, "|"), nil

	case '^':
		l.advance()
		return makeToken(TOKEN_BIT_XOR, "^"), nil

	case '.':
		l.advance()
		if l.peek() == '.' {
			l.advance()
			if l.peek() == '.' {
				l.advance()
				return makeToken(TOKEN_ELLIPSIS, "..."), nil
			}
			return makeToken(TOKEN_CONCAT, ".."), nil
		}
		return makeToken(TOKEN_DOT, "."), nil

	default:
		l.advance()
		return Token{}, fmt.Errorf("unexpected character %q at %d:%d", string(ch), startLine, startCol)
	}
}

// Tokenize returns all tokens from the source, including a trailing EOF.
// Returns an error if any lexical error is encountered.
func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token
	for {
		tok, err := l.nextTokenInternal()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.Type == TOKEN_EOF {
			break
		}
	}
	return tokens, nil
}

// isDigit returns true for ASCII digits.
func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isDigitOrUnderscore(ch byte) bool {
	return isDigit(ch) || ch == '_'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isIntBasePrefix(ch byte) bool {
	return ch == 'x' || ch == 'X' || ch == 'b' || ch == 'B' || ch == 'o' || ch == 'O'
}

func isBaseLiteralChar(ch byte) bool {
	return isLetterOrDigit(ch) || ch == '_'
}

// isLetter returns true for ASCII letters and underscore.
func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

// isLetterOrDigit returns true for ASCII letters, digits, and underscore.
func isLetterOrDigit(ch byte) bool {
	return isLetter(ch) || isDigit(ch)
}
