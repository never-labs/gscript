package dialect

import (
	"strings"
	"testing"
)

func TestParseDuration(t *testing.T) {
	got, err := ParseDuration("1h30m250ms")
	if err != nil {
		t.Fatalf("ParseDuration: %v", err)
	}
	if got.Text != "1h30m0.25s" {
		t.Fatalf("Text = %q, want canonical duration", got.Text)
	}
	if got.Nanoseconds != 5_400_250_000_000 {
		t.Fatalf("Nanoseconds = %d", got.Nanoseconds)
	}
	if got.Seconds != 5400.25 {
		t.Fatalf("Seconds = %v", got.Seconds)
	}
	if got.Milliseconds != 5_400_250 {
		t.Fatalf("Milliseconds = %v", got.Milliseconds)
	}
}

func TestEncodeDuration(t *testing.T) {
	if got := EncodeDurationNanoseconds(90_250_000_000); got != "1m30.25s" {
		t.Fatalf("EncodeDurationNanoseconds = %q", got)
	}
	got, err := EncodeDurationSeconds(1.5)
	if err != nil {
		t.Fatalf("EncodeDurationSeconds: %v", err)
	}
	if got != "1.5s" {
		t.Fatalf("EncodeDurationSeconds = %q", got)
	}
	got, err = EncodeDurationMilliseconds(250)
	if err != nil {
		t.Fatalf("EncodeDurationMilliseconds: %v", err)
	}
	if got != "250ms" {
		t.Fatalf("EncodeDurationMilliseconds = %q", got)
	}
}

func TestParseDurationError(t *testing.T) {
	_, err := ParseDuration("not-duration")
	if err == nil {
		t.Fatalf("ParseDuration returned nil error")
	}
	if !strings.Contains(err.Error(), "duration dialect:") {
		t.Fatalf("error = %q", err)
	}
}
