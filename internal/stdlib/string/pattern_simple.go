package stringlib

import "strings"

type simplePatternOpKind uint8

const (
	simplePatternLiteral simplePatternOpKind = iota
	simplePatternDigit
	simplePatternDigitPlus
	simplePatternCaptureStart
	simplePatternCaptureEnd
)

type simplePatternOp struct {
	kind simplePatternOpKind
	text string
}

type SimplePattern struct {
	ops          []simplePatternOp
	captureCount int
	firstLiteral string
	fast         simplePatternFast
}

func (p *SimplePattern) CaptureCount() int {
	if p == nil {
		return 0
	}
	return p.captureCount
}

type simplePatternFastKind uint8

const (
	simplePatternFastNone simplePatternFastKind = iota
	simplePatternFastTwoDigitRuns
)

type simplePatternFast struct {
	kind               simplePatternFastKind
	prefix             string
	middle             string
	suffix             string
	firstCapturePrefix bool
}

type SimplePatternMatch struct {
	Start    int
	End      int
	NCapture int
	Captures [4][2]int
}

func CompileSimplePattern(pattern string) (*SimplePattern, bool) {
	ops, captures, ok := compileSimplePatternOps(pattern)
	if !ok || len(ops) == 0 || captures > 4 {
		return nil, false
	}
	firstLiteral := ""
	for _, op := range ops {
		if op.kind == simplePatternLiteral && op.text != "" {
			firstLiteral = op.text
			break
		}
		if op.kind != simplePatternCaptureStart {
			break
		}
	}
	return &SimplePattern{ops: ops, captureCount: captures, firstLiteral: firstLiteral, fast: simplePatternFastForOps(ops, captures)}, true
}

func compileSimplePatternOps(pattern string) ([]simplePatternOp, int, bool) {
	ops := make([]simplePatternOp, 0, 8)
	captures := 0
	flushLiteral := func(start, end int) {
		if end > start {
			ops = append(ops, simplePatternOp{kind: simplePatternLiteral, text: pattern[start:end]})
		}
	}
	for i := 0; i < len(pattern); {
		litStart := i
		for i < len(pattern) && pattern[i] != '%' && pattern[i] != '(' && pattern[i] != ')' &&
			pattern[i] != '[' && pattern[i] != '.' && pattern[i] != '*' && pattern[i] != '?' &&
			pattern[i] != '-' && pattern[i] != '+' && pattern[i] != '^' && pattern[i] != '$' {
			i++
		}
		flushLiteral(litStart, i)
		if i >= len(pattern) {
			break
		}
		switch pattern[i] {
		case '%':
			if i+1 >= len(pattern) || pattern[i+1] != 'd' {
				return nil, 0, false
			}
			if i+2 < len(pattern) && pattern[i+2] == '+' {
				ops = append(ops, simplePatternOp{kind: simplePatternDigitPlus})
				i += 3
			} else {
				ops = append(ops, simplePatternOp{kind: simplePatternDigit})
				i += 2
			}
		case '(':
			end := strings.IndexByte(pattern[i+1:], ')')
			if end < 0 {
				return nil, 0, false
			}
			inner := pattern[i+1 : i+1+end]
			if inner == "" || strings.ContainsAny(inner, "()[]^$.*?-") {
				return nil, 0, false
			}
			innerOps, innerCaptures, ok := compileSimplePatternOps(inner)
			if !ok || innerCaptures != 0 {
				return nil, 0, false
			}
			ops = append(ops, simplePatternOp{kind: simplePatternCaptureStart})
			ops = append(ops, innerOps...)
			ops = append(ops, simplePatternOp{kind: simplePatternCaptureEnd})
			captures++
			i += end + 2
		default:
			return nil, 0, false
		}
	}
	return ops, captures, true
}

func (p *SimplePattern) FindNext(s string, start int) (SimplePatternMatch, bool) {
	if p.fast.kind == simplePatternFastTwoDigitRuns {
		return p.findNextTwoDigitRuns(s, start)
	}
	if start < 0 {
		start = 0
	}
	if start > len(s) {
		return SimplePatternMatch{}, false
	}
	for pos := start; pos <= len(s); pos++ {
		if p.firstLiteral != "" {
			idx := strings.Index(s[pos:], p.firstLiteral)
			if idx < 0 {
				return SimplePatternMatch{}, false
			}
			pos += idx
		}
		if m, ok := p.matchAt(s, pos); ok {
			return m, true
		}
	}
	return SimplePatternMatch{}, false
}

func simplePatternFastForOps(ops []simplePatternOp, captures int) simplePatternFast {
	if captures != 2 {
		return simplePatternFast{}
	}
	// Shape A: prefix(%d+)middle%d%dsuffix(%d+)
	if len(ops) == 11 &&
		ops[0].kind == simplePatternLiteral &&
		ops[1].kind == simplePatternCaptureStart &&
		ops[2].kind == simplePatternDigitPlus &&
		ops[3].kind == simplePatternCaptureEnd &&
		ops[4].kind == simplePatternLiteral &&
		ops[5].kind == simplePatternDigit &&
		ops[6].kind == simplePatternDigit &&
		ops[7].kind == simplePatternLiteral &&
		ops[8].kind == simplePatternCaptureStart &&
		ops[9].kind == simplePatternDigitPlus &&
		ops[10].kind == simplePatternCaptureEnd {
		return simplePatternFast{
			kind:   simplePatternFastTwoDigitRuns,
			prefix: ops[0].text,
			middle: ops[4].text,
			suffix: ops[7].text,
		}
	}
	// Shape B: (prefix%d+)middle%d%dsuffix(%d+)
	if len(ops) == 11 &&
		ops[0].kind == simplePatternCaptureStart &&
		ops[1].kind == simplePatternLiteral &&
		ops[2].kind == simplePatternDigitPlus &&
		ops[3].kind == simplePatternCaptureEnd &&
		ops[4].kind == simplePatternLiteral &&
		ops[5].kind == simplePatternDigit &&
		ops[6].kind == simplePatternDigit &&
		ops[7].kind == simplePatternLiteral &&
		ops[8].kind == simplePatternCaptureStart &&
		ops[9].kind == simplePatternDigitPlus &&
		ops[10].kind == simplePatternCaptureEnd {
		return simplePatternFast{
			kind:               simplePatternFastTwoDigitRuns,
			prefix:             ops[1].text,
			middle:             ops[4].text,
			suffix:             ops[7].text,
			firstCapturePrefix: true,
		}
	}
	return simplePatternFast{}
}

func (p *SimplePattern) findNextTwoDigitRuns(s string, start int) (SimplePatternMatch, bool) {
	if start < 0 {
		start = 0
	}
	if start > len(s) {
		return SimplePatternMatch{}, false
	}
	f := p.fast
	for search := start; search <= len(s); {
		idx := strings.Index(s[search:], f.prefix)
		if idx < 0 {
			return SimplePatternMatch{}, false
		}
		pos := search + idx
		digits1Start := pos + len(f.prefix)
		digits1End := ScanASCIIDigits(s, digits1Start)
		if digits1End == digits1Start || !HasStringAt(s, digits1End, f.middle) {
			search = pos + 1
			continue
		}
		tagStart := digits1End + len(f.middle)
		if tagStart+2 > len(s) || !IsASCIIDigit(s[tagStart]) || !IsASCIIDigit(s[tagStart+1]) {
			search = pos + 1
			continue
		}
		suffixStart := tagStart + 2
		if !HasStringAt(s, suffixStart, f.suffix) {
			search = pos + 1
			continue
		}
		digits2Start := suffixStart + len(f.suffix)
		digits2End := ScanASCIIDigits(s, digits2Start)
		if digits2End == digits2Start {
			search = pos + 1
			continue
		}
		cap0Start := digits1Start
		if f.firstCapturePrefix {
			cap0Start = pos
		}
		return SimplePatternMatch{
			Start:    pos,
			End:      digits2End,
			NCapture: 2,
			Captures: [4][2]int{
				{cap0Start, digits1End},
				{digits2Start, digits2End},
			},
		}, true
	}
	return SimplePatternMatch{}, false
}

func (p *SimplePattern) matchAt(s string, pos int) (SimplePatternMatch, bool) {
	m := SimplePatternMatch{Start: pos}
	capStack := [4]int{}
	for _, op := range p.ops {
		switch op.kind {
		case simplePatternLiteral:
			if !strings.HasPrefix(s[pos:], op.text) {
				return SimplePatternMatch{}, false
			}
			pos += len(op.text)
		case simplePatternDigit:
			if pos >= len(s) || !IsASCIIDigit(s[pos]) {
				return SimplePatternMatch{}, false
			}
			pos++
		case simplePatternDigitPlus:
			start := pos
			pos = ScanASCIIDigits(s, pos)
			if pos == start {
				return SimplePatternMatch{}, false
			}
		case simplePatternCaptureStart:
			if m.NCapture >= len(m.Captures) {
				return SimplePatternMatch{}, false
			}
			capStack[m.NCapture] = pos
		case simplePatternCaptureEnd:
			m.Captures[m.NCapture] = [2]int{capStack[m.NCapture], pos}
			m.NCapture++
		}
	}
	m.End = pos
	return m, true
}
