package bind

import "testing"

func TestHTTPGoToLeia(t *testing.T) {
	v := goToLeia(nil)
	if !v.IsNil() {
		t.Errorf("expected nil, got %v", v)
	}

	v = goToLeia(true)
	if !v.IsBool() || !v.Bool() {
		t.Errorf("expected true, got %v", v)
	}

	v = goToLeia(3.14)
	if !v.IsFloat() || v.Number() != 3.14 {
		t.Errorf("expected 3.14, got %v", v)
	}

	v = goToLeia("hello")
	if !v.IsString() || v.Str() != "hello" {
		t.Errorf("expected 'hello', got %v", v)
	}

	v = goToLeia([]interface{}{"a", "b"})
	if !v.IsTable() {
		t.Errorf("expected table, got %v", v.TypeName())
	}
	tbl := v.Table()
	if tbl.Length() != 2 {
		t.Errorf("expected length 2, got %d", tbl.Length())
	}
	if tbl.RawGet(IntValue(1)).Str() != "a" {
		t.Errorf("expected 'a' at index 1, got %v", tbl.RawGet(IntValue(1)))
	}

	v = goToLeia(map[string]interface{}{"key": "val"})
	if !v.IsTable() {
		t.Errorf("expected table, got %v", v.TypeName())
	}
	tbl = v.Table()
	if tbl.RawGet(StringValue("key")).Str() != "val" {
		t.Errorf("expected 'val' for key 'key', got %v", tbl.RawGet(StringValue("key")))
	}
}

func TestHTTPLeiaToGo(t *testing.T) {
	if leiaToGo(NilValue()) != nil {
		t.Errorf("expected nil")
	}
	if leiaToGo(BoolValue(true)) != true {
		t.Errorf("expected true")
	}
	if leiaToGo(IntValue(42)) != int64(42) {
		t.Errorf("expected 42")
	}
	if leiaToGo(FloatValue(3.14)) != 3.14 {
		t.Errorf("expected 3.14")
	}
	if leiaToGo(StringValue("hello")) != "hello" {
		t.Errorf("expected 'hello'")
	}

	tbl := NewTable()
	tbl.RawSet(IntValue(1), StringValue("a"))
	tbl.RawSet(IntValue(2), StringValue("b"))
	result := leiaToGo(TableValue(tbl))
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Errorf("expected [a, b], got %v", arr)
	}

	tbl2 := NewTable()
	tbl2.RawSet(StringValue("key"), StringValue("val"))
	result2 := leiaToGo(TableValue(tbl2))
	m, ok := result2.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result2)
	}
	if m["key"] != "val" {
		t.Errorf("expected key=val, got %v", m)
	}
}
