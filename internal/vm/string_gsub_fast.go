package vm

import (
	"strings"

	"github.com/gscript/gscript/internal/runtime"
)

type simpleGSubTwoCapturePattern struct {
	prefix string
	middle string
	tag    string
}

type concatTwoArgReplacementSpec struct {
	firstParam  int
	secondParam int
	separator   string
}

func (vm *VM) ExecuteStdStringGSubCall(absSlot, nArgs, rawC int) (bool, error) {
	if nArgs != 3 || absSlot+3 >= len(vm.regs) {
		return false, nil
	}
	sv, pv, replv := vm.regs[absSlot+1], vm.regs[absSlot+2], vm.regs[absSlot+3]
	if !sv.IsString() || !pv.IsString() {
		return true, nil
	}
	pat, ok := parseSimpleGSubTwoCapturePattern(pv.Str())
	if !ok {
		return false, nil
	}
	repl, ok := closureFromValue(replv)
	if !ok {
		return false, nil
	}
	spec, ok := concatTwoArgReplacementSpecForProto(repl.Proto)
	if !ok {
		return false, nil
	}
	out, count := replaceSimpleGSubTwoCapture(sv.Str(), pat, spec)
	if err := vm.recordFastNativeCall(vm.regs[absSlot].GoFunction()); err != nil {
		return true, err
	}
	outVal := runtime.StringValue(out)
	if err := vm.checkHostResultBudget(outVal); err != nil {
		return true, err
	}
	vm.storeStdSelectResults(absSlot, rawC, []runtime.Value{outVal, runtime.IntValue(int64(count))})
	return true, nil
}

func parseSimpleGSubTwoCapturePattern(pattern string) (simpleGSubTwoCapturePattern, bool) {
	open1 := strings.IndexByte(pattern, '(')
	if open1 <= 0 || !strings.HasPrefix(pattern[open1:], "(%d+)") {
		return simpleGSubTwoCapturePattern{}, false
	}
	after1 := open1 + len("(%d+)")
	open2Rel := strings.IndexByte(pattern[after1:], '(')
	if open2Rel < 0 {
		return simpleGSubTwoCapturePattern{}, false
	}
	open2 := after1 + open2Rel
	if !strings.HasSuffix(pattern[open2:], ")") {
		return simpleGSubTwoCapturePattern{}, false
	}
	inner2 := pattern[open2+1 : len(pattern)-1]
	tag := ""
	for len(inner2) >= 2 && strings.HasSuffix(inner2, "%d") {
		inner2 = inner2[:len(inner2)-2]
		tag += "%d"
	}
	if tag != "%d%d" || inner2 == "" {
		return simpleGSubTwoCapturePattern{}, false
	}
	return simpleGSubTwoCapturePattern{
		prefix: pattern[:open1],
		middle: pattern[after1:open2],
		tag:    inner2,
	}, true
}

func concatTwoArgReplacementSpecForProto(p *FuncProto) (concatTwoArgReplacementSpec, bool) {
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 5 || len(p.Constants) != 1 {
		return concatTwoArgReplacementSpec{}, false
	}
	code := p.Code
	if DecodeOp(code[0]) != OP_MOVE || DecodeOp(code[1]) != OP_LOADK ||
		DecodeOp(code[2]) != OP_MOVE || DecodeOp(code[3]) != OP_CONCAT ||
		DecodeOp(code[4]) != OP_RETURN {
		return concatTwoArgReplacementSpec{}, false
	}
	sep, ok := constStringAt(p, DecodeBx(code[1]))
	if !ok {
		return concatTwoArgReplacementSpec{}, false
	}
	if DecodeA(code[3]) != 2 || DecodeB(code[3]) != DecodeA(code[0]) || DecodeC(code[3]) != DecodeA(code[2]) ||
		DecodeA(code[4]) != 2 || DecodeB(code[4]) != 2 {
		return concatTwoArgReplacementSpec{}, false
	}
	first := DecodeB(code[0])
	second := DecodeB(code[2])
	if (first != 0 && first != 1) || (second != 0 && second != 1) || first == second {
		return concatTwoArgReplacementSpec{}, false
	}
	return concatTwoArgReplacementSpec{firstParam: first, secondParam: second, separator: sep}, true
}

func replaceSimpleGSubTwoCapture(s string, pat simpleGSubTwoCapturePattern, repl concatTwoArgReplacementSpec) (string, int) {
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	next := 0
	count := 0
	for next < len(s) {
		idx := strings.Index(s[next:], pat.prefix)
		if idx < 0 {
			break
		}
		start := next + idx
		cap1Start := start + len(pat.prefix)
		cap1End := scanDigits(s, cap1Start)
		if cap1End == cap1Start || !hasAt(s, cap1End, pat.middle) {
			next = start + 1
			continue
		}
		cap2Start := cap1End + len(pat.middle)
		if !hasAt(s, cap2Start, pat.tag) {
			next = start + 1
			continue
		}
		cap2Digits := cap2Start + len(pat.tag)
		if cap2Digits+2 > len(s) || !isDigit(s[cap2Digits]) || !isDigit(s[cap2Digits+1]) {
			next = start + 1
			continue
		}
		end := cap2Digits + 2
		caps := [2]string{s[cap1Start:cap1End], s[cap2Start:end]}
		b.WriteString(s[last:start])
		b.WriteString(caps[repl.firstParam])
		b.WriteString(repl.separator)
		b.WriteString(caps[repl.secondParam])
		last = end
		next = end
		count++
	}
	if count == 0 {
		return s, 0
	}
	b.WriteString(s[last:])
	return b.String(), count
}

func scanDigits(s string, pos int) int {
	for pos < len(s) && isDigit(s[pos]) {
		pos++
	}
	return pos
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func hasAt(s string, pos int, needle string) bool {
	return pos <= len(s) && len(needle) <= len(s)-pos && s[pos:pos+len(needle)] == needle
}
