package outputlimit

import (
	"strings"
	"testing"
)

func TestBuffersShareBudget(t *testing.T) {
	stdout, stderr := NewBuffers(5)
	if n, err := stdout.Write([]byte("abc")); err != nil || n != 3 {
		t.Fatalf("stdout.Write = (%d, %v), want (3, nil)", n, err)
	}
	if n, err := stderr.Write([]byte("de")); err != nil || n != 2 {
		t.Fatalf("stderr.Write = (%d, %v), want (2, nil)", n, err)
	}
	if n, err := stdout.Write([]byte("f")); err == nil || n != 0 {
		t.Fatalf("stdout.Write after limit = (%d, %v), want (0, error)", n, err)
	}
	if !stdout.Exceeded() || !stderr.Exceeded() {
		t.Fatalf("Exceeded did not propagate across shared budget")
	}
}

func TestBufferKeepsPartialWriteBeforeLimit(t *testing.T) {
	stdout, _ := NewBuffers(4)
	n, err := stdout.Write([]byte("abcdef"))
	if err == nil {
		t.Fatalf("Write returned nil error after exceeding limit")
	}
	if n != 4 {
		t.Fatalf("Write n = %d, want 4", n)
	}
	if got := stdout.String(); got != "abcd" {
		t.Fatalf("String = %q, want %q", got, "abcd")
	}
	if !strings.Contains(err.Error(), "host result byte limit exceeded (4)") {
		t.Fatalf("Write error = %q", err)
	}
}

func TestBufferUnlimitedWhenMaxIsZero(t *testing.T) {
	stdout, stderr := NewBuffers(0)
	if _, err := stdout.Write([]byte("abc")); err != nil {
		t.Fatalf("stdout.Write returned error: %v", err)
	}
	if _, err := stderr.Write([]byte("def")); err != nil {
		t.Fatalf("stderr.Write returned error: %v", err)
	}
	if stdout.Exceeded() || stderr.Exceeded() {
		t.Fatalf("unlimited buffers reported exceeded")
	}
}
