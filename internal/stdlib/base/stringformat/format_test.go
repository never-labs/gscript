package stringformat

import "testing"

func TestScanToken(t *testing.T) {
	tok, ok := ScanToken("a%05.2fb", 1)
	if !ok {
		t.Fatal("ScanToken rejected valid token")
	}
	if tok.Start != 1 || tok.End != 7 || tok.Spec != "%05.2f" || tok.Verb != 'f' {
		t.Fatalf("ScanToken = %#v", tok)
	}
	if _, ok := ScanToken("%", 0); ok {
		t.Fatal("ScanToken accepted trailing percent")
	}
}

func TestCompileSimpleInteger(t *testing.T) {
	prog, status := CompileSimple("id=%05d")
	if status != CompileOK {
		t.Fatalf("CompileSimple status = %v", status)
	}
	if prog.ArgCount != 1 || prog.LitBytes != 3 || !prog.SingleInt {
		t.Fatalf("CompileSimple program = %#v", prog)
	}
	if len(prog.Parts) != 2 {
		t.Fatalf("parts len = %d", len(prog.Parts))
	}
	part := prog.Parts[1]
	if part.Spec != "%05d" || part.Verb != 'd' || part.Pad != '0' || part.Width != 5 {
		t.Fatalf("integer part = %#v", part)
	}
}

func TestCompileSimpleStatus(t *testing.T) {
	tests := []struct {
		format string
		want   CompileStatus
	}{
		{"plain", CompileNotSimple},
		{"%%", CompileNotSimple},
		{"%", CompileErrEndsWithPercent},
		{"%0.", CompileErrInvalid},
		{"%-5d", CompileNotSimple},
		{"%.2d", CompileNotSimple},
		{"%.10f", CompileNotSimple},
	}
	for _, tt := range tests {
		if _, got := CompileSimple(tt.format); got != tt.want {
			t.Fatalf("CompileSimple(%q) status = %v, want %v", tt.format, got, tt.want)
		}
	}
}
