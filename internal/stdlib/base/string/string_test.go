package stringlib

import "testing"

func TestStringTransforms(t *testing.T) {
	if got := Upper("héllo"); got != "HÉLLO" {
		t.Fatalf("Upper = %q", got)
	}
	if got := Lower("HÉLLO"); got != "héllo" {
		t.Fatalf("Lower = %q", got)
	}
	if got := Reverse("ab界"); got != "界ba" {
		t.Fatalf("Reverse = %q", got)
	}
	if got := RepeatJoin("ha", 3, "-"); got != "ha-ha-ha" {
		t.Fatalf("RepeatJoin = %q", got)
	}
}

func TestTrimAndPredicates(t *testing.T) {
	if got := TrimSpace("\t hi \n"); got != "hi" {
		t.Fatalf("TrimSpace = %q", got)
	}
	if got := Trim("..hi..", "."); got != "hi" {
		t.Fatalf("Trim = %q", got)
	}
	if !HasPrefix("gscript", "gs") || !HasSuffix("gscript", "script") || !Contains("gscript", "cri") {
		t.Fatal("string predicates failed")
	}
	if got := Count("banana", "an"); got != 2 {
		t.Fatalf("Count = %d", got)
	}
}

func TestReplaceTitlePadNumeric(t *testing.T) {
	if got := ReplaceAll("a-b-a", "a", "x"); got != "x-b-x" {
		t.Fatalf("ReplaceAll = %q", got)
	}
	if got := Title("hello world"); got != "Hello World" {
		t.Fatalf("Title = %q", got)
	}
	if got := PadLeft("go", 5, "0"); got != "000go" {
		t.Fatalf("PadLeft = %q", got)
	}
	if got := PadRight("go", 5, "ab"); got != "goaba" {
		t.Fatalf("PadRight = %q", got)
	}
	if !IsNumeric(" 3.14 ") || IsNumeric(" ") || IsNumeric("no") {
		t.Fatal("IsNumeric failed")
	}
}
