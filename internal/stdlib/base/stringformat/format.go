package stringformat

type CompileStatus uint8

const (
	CompileOK CompileStatus = iota
	CompileNotSimple
	CompileErrEndsWithPercent
	CompileErrInvalid
)

type Token struct {
	Start int
	End   int
	Spec  string
	Verb  byte
}

type Part struct {
	Lit   string
	Spec  string
	Verb  byte
	Pad   byte
	Width int
	Prec  int
}

type Program struct {
	Parts     []Part
	ArgCount  int
	LitBytes  int
	SingleInt bool
}

func IsFlag(b byte) bool {
	switch b {
	case '-', '+', ' ', '#', '0':
		return true
	default:
		return false
	}
}

func ScanToken(format string, percent int) (Token, bool) {
	if percent < 0 || percent >= len(format) || format[percent] != '%' {
		return Token{}, false
	}
	i := percent + 1
	for i < len(format) && IsFlag(format[i]) {
		i++
	}
	for i < len(format) && isDigit(format[i]) {
		i++
	}
	if i < len(format) && format[i] == '.' {
		i++
		for i < len(format) && isDigit(format[i]) {
			i++
		}
	}
	if i >= len(format) {
		return Token{}, false
	}
	i++
	return Token{
		Start: percent,
		End:   i,
		Spec:  format[percent:i],
		Verb:  format[i-1],
	}, true
}

func CompileSimple(format string) (Program, CompileStatus) {
	parts := make([]Part, 0, 4)
	litStart := 0
	argCount := 0
	litBytes := 0
	for i := 0; i < len(format); {
		if format[i] != '%' {
			i++
			continue
		}
		if i+1 >= len(format) {
			return Program{}, CompileErrEndsWithPercent
		}
		if format[i+1] == '%' {
			return Program{}, CompileNotSimple
		}
		if i > litStart {
			lit := format[litStart:i]
			parts = append(parts, Part{Lit: lit})
			litBytes += len(lit)
		}

		start := i
		i++
		for i < len(format) && IsFlag(format[i]) {
			if format[i] != '0' {
				return Program{}, CompileNotSimple
			}
			i++
		}
		for i < len(format) && isDigit(format[i]) {
			i++
		}
		precisionStart := -1
		if i < len(format) && format[i] == '.' {
			precisionStart = i
			i++
			for i < len(format) && isDigit(format[i]) {
				i++
			}
		}
		if i >= len(format) {
			return Program{}, CompileErrInvalid
		}
		verb := format[i]
		i++
		switch verb {
		case 'd', 'i', 'u', 'x', 'X', 'o':
			if precisionStart >= 0 {
				return Program{}, CompileNotSimple
			}
			part, ok := compileSimpleIntegerPart(format[start:i], verb)
			if !ok {
				return Program{}, CompileNotSimple
			}
			parts = append(parts, part)
		case 'f':
			part, ok := compileSimpleFloatPart(format[start:i])
			if !ok {
				return Program{}, CompileNotSimple
			}
			parts = append(parts, part)
		case 's':
			if precisionStart >= 0 || i-start != 2 {
				return Program{}, CompileNotSimple
			}
			parts = append(parts, Part{Spec: "%s", Verb: verb})
		default:
			return Program{}, CompileNotSimple
		}
		argCount++
		litStart = i
	}
	if argCount == 0 {
		return Program{}, CompileNotSimple
	}
	if litStart < len(format) {
		lit := format[litStart:]
		parts = append(parts, Part{Lit: lit})
		litBytes += len(lit)
	}
	return Program{
		Parts:     parts,
		ArgCount:  argCount,
		LitBytes:  litBytes,
		SingleInt: argCount == 1 && hasSingleIntegerArg(parts),
	}, CompileOK
}

func compileSimpleIntegerPart(fmtSpec string, verb byte) (Part, bool) {
	if len(fmtSpec) < 2 || fmtSpec[0] != '%' || fmtSpec[len(fmtSpec)-1] != verb {
		return Part{}, false
	}
	pos := 1
	pad := byte(' ')
	if pos < len(fmtSpec)-1 && fmtSpec[pos] == '0' {
		pad = '0'
		pos++
	}
	width := 0
	for pos < len(fmtSpec)-1 && isDigit(fmtSpec[pos]) {
		width = width*10 + int(fmtSpec[pos]-'0')
		pos++
	}
	if pos != len(fmtSpec)-1 {
		return Part{}, false
	}
	return Part{Spec: fmtSpec, Verb: verb, Pad: pad, Width: width}, true
}

func compileSimpleFloatPart(fmtSpec string) (Part, bool) {
	if len(fmtSpec) < 3 || fmtSpec[0] != '%' || fmtSpec[len(fmtSpec)-1] != 'f' {
		return Part{}, false
	}
	pos := 1
	width := 0
	for pos < len(fmtSpec)-1 && isDigit(fmtSpec[pos]) {
		width = width*10 + int(fmtSpec[pos]-'0')
		pos++
	}
	prec := 6
	if pos < len(fmtSpec)-1 && fmtSpec[pos] == '.' {
		pos++
		prec = 0
		if pos >= len(fmtSpec)-1 || !isDigit(fmtSpec[pos]) {
			return Part{}, false
		}
		for pos < len(fmtSpec)-1 && isDigit(fmtSpec[pos]) {
			prec = prec*10 + int(fmtSpec[pos]-'0')
			pos++
		}
	}
	if pos != len(fmtSpec)-1 || prec > 9 {
		return Part{}, false
	}
	return Part{Spec: fmtSpec, Verb: 'f', Width: width, Prec: prec}, true
}

func hasSingleIntegerArg(parts []Part) bool {
	seen := false
	for _, part := range parts {
		if part.Verb == 0 {
			continue
		}
		switch part.Verb {
		case 'd', 'i', 'u', 'x', 'X', 'o':
			if seen {
				return false
			}
			seen = true
		default:
			return false
		}
	}
	return seen
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
