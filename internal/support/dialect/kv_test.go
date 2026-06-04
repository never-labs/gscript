package dialect

import "testing"

func TestEncodeKVSortsKeys(t *testing.T) {
	got, err := EncodeKV(map[string]string{
		"b": "2",
		"a": "1",
	}, KVOptions{})
	if err != nil {
		t.Fatalf("EncodeKV error: %v", err)
	}
	if want := "a=1\nb=2\n"; got != want {
		t.Fatalf("EncodeKV = %q, want %q", got, want)
	}
}

func TestEncodeKVHonorsSeparator(t *testing.T) {
	got, err := EncodeKV(map[string]string{
		"b": "2",
		"a": "1",
	}, KVOptions{Sep: ":"})
	if err != nil {
		t.Fatalf("EncodeKV error: %v", err)
	}
	if want := "a:1\nb:2\n"; got != want {
		t.Fatalf("EncodeKV = %q, want %q", got, want)
	}
	roundtrip, err := KV(got, KVOptions{Sep: ":", Trim: true})
	if err != nil {
		t.Fatalf("KV roundtrip error: %v", err)
	}
	if roundtrip["a"] != "1" || roundtrip["b"] != "2" {
		t.Fatalf("roundtrip = %#v", roundtrip)
	}
}

func TestEncodeKVRejectsUnsafeKeysAndValues(t *testing.T) {
	for _, values := range []map[string]string{
		{"bad=key": "1"},
		{"bad\nkey": "1"},
		{"ok": "bad\nvalue"},
	} {
		if _, err := EncodeKV(values, KVOptions{}); err == nil {
			t.Fatalf("EncodeKV(%#v) returned nil error", values)
		}
	}
}
