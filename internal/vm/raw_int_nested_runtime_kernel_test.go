package vm

import (
	"strings"
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

const rawIntNestedShiftedSource = `
func nestwave(level, width) {
	if level == 0 { return width + 2 }
	if width == 0 { return nestwave(level - 1, 2) }
	return nestwave(level - 1, nestwave(level, width - 1))
}
`

func TestRawIntNestedRuntimeKernelRecognizesShiftedNestedRecurrence(t *testing.T) {
	top := compileProto(t, rawIntNestedShiftedSource)
	fn := findTestProtoByName(top, "nestwave")
	if fn == nil {
		t.Fatal("nestwave proto not found")
	}
	kernel, ok := analyzeRawIntNestedKernel(fn)
	if !ok {
		t.Fatalf("nestwave should qualify for raw-int nested recurrence kernel:\n%s", dumpRawIntNestedTestBytecode(fn))
	}
	if kernel.selfName != "nestwave" || kernel.baseAdd != 2 || kernel.zeroArg != 2 || kernel.mStep != 1 || kernel.nStep != 1 {
		t.Fatalf("unexpected kernel: %#v", kernel)
	}
	got, ok := kernel.fold(runtime.IntValue(2), runtime.IntValue(6))
	if !ok || got != 764 {
		t.Fatalf("nestwave(2,6) kernel = %d/%v, want 764/true", got, ok)
	}
}

func dumpRawIntNestedTestBytecode(proto *FuncProto) string {
	var b strings.Builder
	for pc, inst := range proto.Code {
		b.WriteString(itoaRawIntNestedTest(int(DecodeOp(inst))))
		b.WriteString(" pc=")
		b.WriteString(itoaRawIntNestedTest(pc))
		b.WriteString(" A=")
		b.WriteString(itoaRawIntNestedTest(DecodeA(inst)))
		b.WriteString(" B=")
		b.WriteString(itoaRawIntNestedTest(DecodeB(inst)))
		b.WriteString(" C=")
		b.WriteString(itoaRawIntNestedTest(DecodeC(inst)))
		b.WriteString(" Bx=")
		b.WriteString(itoaRawIntNestedTest(DecodeBx(inst)))
		b.WriteString(" sBx=")
		b.WriteString(itoaRawIntNestedTest(DecodesBx(inst)))
		b.WriteByte('\n')
	}
	return b.String()
}

func itoaRawIntNestedTest(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
