package runtime

import "testing"

func TestStringSplitProjectSub(t *testing.T) {
	v, err := StringSplitProjectSub(StringValue("svc=api7|status=500|bytes=1234"), StringValue("|"), 2, 8, 0, false)
	if err != nil {
		t.Fatalf("StringSplitProjectSub: %v", err)
	}
	if !v.IsString() || v.Str() != "500" {
		t.Fatalf("substring = %v, want 500", v)
	}
}

func TestStringSplitProjectSubToNumber(t *testing.T) {
	v, err := StringSplitProjectSubToNumber(StringValue("svc=api7|status=500|bytes=1234"), StringValue("|"), 3, 7, 0, false)
	if err != nil {
		t.Fatalf("StringSplitProjectSubToNumber: %v", err)
	}
	if !v.IsInt() || v.Int() != 1234 {
		t.Fatalf("number = %v, want int 1234", v)
	}
}
