package utf8x

import (
	"reflect"
	"testing"
)

func TestCodepointStartsAndContinuationByte(t *testing.T) {
	if got := CodepointStarts("A中文"); !reflect.DeepEqual(got, []int{0, 1, 4}) {
		t.Fatalf("CodepointStarts = %v, want [0 1 4]", got)
	}
	if !IsContinuationByte(0x80) || !IsContinuationByte(0xBF) {
		t.Fatalf("expected continuation byte bounds to be true")
	}
	if IsContinuationByte(0x7F) || IsContinuationByte(0xC0) {
		t.Fatalf("expected non-continuation bytes to be false")
	}
}

func TestValidateAndSanitize(t *testing.T) {
	valid := Validate("A\U0001F600")
	if !valid.Valid || valid.ByteCount != 5 || valid.RuneCount != 2 || valid.ErrorPos != 0 || valid.Error != "" {
		t.Fatalf("valid report = %#v", valid)
	}

	truncated := Validate("ok" + string([]byte{0xE2}))
	if truncated.Valid || truncated.ByteCount != 3 || truncated.RuneCount != 2 || truncated.ErrorPos != 3 || truncated.Error == "" {
		t.Fatalf("truncated report = %#v", truncated)
	}

	if got := Sanitize("a"+string([]byte{0x80})+"b", "\uFFFD"); got != "a\uFFFDb" {
		t.Fatalf("Sanitize replacement = %q", got)
	}
	if got := Sanitize(string([]byte{0xC0, 0x80}), "?"); got != "??" {
		t.Fatalf("Sanitize custom = %q", got)
	}
}

func TestReverseSubAndOffset(t *testing.T) {
	if got := Reverse("a中文"); got != "文中a" {
		t.Fatalf("Reverse = %q", got)
	}
	if got := Sub("中文测试", 2, 3); got != "文测" {
		t.Fatalf("Sub = %q", got)
	}
	if got := Sub("hello", -3, -1); got != "llo" {
		t.Fatalf("Sub negative = %q", got)
	}

	pos, ok := Offset("中文测试", 2, 1)
	if !ok || pos != 4 {
		t.Fatalf("Offset positive = %d, %v; want 4, true", pos, ok)
	}
	pos, ok = Offset("ABC", 0, 3)
	if !ok || pos != 3 {
		t.Fatalf("Offset n=0 = %d, %v; want 3, true", pos, ok)
	}
	_, ok = Offset("ABC", -4, 1)
	if ok {
		t.Fatalf("Offset out of range ok = true")
	}
}
