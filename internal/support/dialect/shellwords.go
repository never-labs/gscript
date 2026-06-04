package dialect

import (
	"fmt"
	"strings"
	"unicode"
)

func Shellwords(src string) ([]string, error) {
	var out []string
	var b strings.Builder
	inSingle := false
	inDouble := false
	haveWord := false

	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == 0 {
			return nil, &ParseError{Kind: "shellwords", Message: "NUL byte is not allowed"}
		}
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
				continue
			}
			b.WriteByte(c)
			haveWord = true
		case inDouble:
			switch c {
			case '"':
				inDouble = false
			case '\\':
				i++
				if i >= len(src) {
					return nil, &ParseError{Kind: "shellwords", Message: "unfinished escape"}
				}
				if src[i] == 0 {
					return nil, &ParseError{Kind: "shellwords", Message: "NUL byte is not allowed"}
				}
				b.WriteByte(src[i])
				haveWord = true
			default:
				b.WriteByte(c)
				haveWord = true
			}
		default:
			switch {
			case unicode.IsSpace(rune(c)):
				if haveWord {
					out = append(out, b.String())
					b.Reset()
					haveWord = false
				}
			case c == '\'':
				inSingle = true
				haveWord = true
			case c == '"':
				inDouble = true
				haveWord = true
			case c == '\\':
				i++
				if i >= len(src) {
					return nil, &ParseError{Kind: "shellwords", Message: "unfinished escape"}
				}
				if src[i] == 0 {
					return nil, &ParseError{Kind: "shellwords", Message: "NUL byte is not allowed"}
				}
				b.WriteByte(src[i])
				haveWord = true
			default:
				b.WriteByte(c)
				haveWord = true
			}
		}
	}
	if inSingle {
		return nil, &ParseError{Kind: "shellwords", Message: "unterminated single quote"}
	}
	if inDouble {
		return nil, &ParseError{Kind: "shellwords", Message: "unterminated double quote"}
	}
	if haveWord {
		out = append(out, b.String())
	}
	return out, nil
}

func ShellwordsEncode(args []string) (string, error) {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "", fmt.Errorf("shellwords dialect: argument %d contains NUL byte", i+1)
		}
		parts[i] = ShellQuote(arg)
	}
	return strings.Join(parts, " "), nil
}

func ShellQuote(arg string) string {
	if arg != "" && isShellBareword(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

func isShellBareword(arg string) bool {
	for i := 0; i < len(arg); i++ {
		c := arg[i]
		if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') {
			continue
		}
		switch c {
		case '_', '@', '%', '+', '=', ':', ',', '.', '/', '-':
			continue
		default:
			return false
		}
	}
	return true
}
