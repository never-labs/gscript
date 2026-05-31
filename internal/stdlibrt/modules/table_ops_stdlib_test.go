package modules

import "testing"

func TestTableLengthAfterInsert(t *testing.T) {
	v := getGlobal(t, `
		t := {1, 2, 3}
		table.insert(t, 4)
		result := #t
	`, "result")
	if !v.IsInt() || v.Int() != 4 {
		t.Errorf("expected 4, got %v", v)
	}
}

func TestTableLengthAfterRemove(t *testing.T) {
	v := getGlobal(t, `
		t := {1, 2, 3, 4, 5}
		table.remove(t)
		result := #t
	`, "result")
	if !v.IsInt() || v.Int() != 4 {
		t.Errorf("expected 4, got %v", v)
	}
}

func TestTableConcatEmpty(t *testing.T) {
	v := getGlobal(t, `
		t := {}
		result := table.concat(t, ",")
	`, "result")
	if v.Str() != "" {
		t.Errorf("expected empty string, got %q", v.Str())
	}
}

func TestTableConcatSingleElement(t *testing.T) {
	v := getGlobal(t, `
		t := {"hello"}
		result := table.concat(t, ",")
	`, "result")
	if v.Str() != "hello" {
		t.Errorf("expected 'hello', got %q", v.Str())
	}
}

func TestTableSortStrings(t *testing.T) {
	interp := runProgram(t, `
		t := {"banana", "apple", "cherry"}
		table.sort(t)
	`)
	tbl := interp.GetGlobal("t").Table()
	if tbl.RawGet(IntValue(1)).Str() != "apple" {
		t.Errorf("expected t[1]='apple', got %v", tbl.RawGet(IntValue(1)))
	}
	if tbl.RawGet(IntValue(2)).Str() != "banana" {
		t.Errorf("expected t[2]='banana', got %v", tbl.RawGet(IntValue(2)))
	}
	if tbl.RawGet(IntValue(3)).Str() != "cherry" {
		t.Errorf("expected t[3]='cherry', got %v", tbl.RawGet(IntValue(3)))
	}
}

func TestTableAsStack(t *testing.T) {
	v := getGlobal(t, `
		stack := {}
		table.insert(stack, 10)
		table.insert(stack, 20)
		table.insert(stack, 30)
		top := table.remove(stack)
		result := top + #stack
	`, "result")
	if !v.IsInt() || v.Int() != 32 {
		t.Errorf("expected 32, got %v", v)
	}
}

func TestTableUnpackSubrange(t *testing.T) {
	interp := runProgram(t, `
		t := {10, 20, 30, 40, 50}
		a, b, c := table.unpack(t, 2, 4)
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	c := interp.GetGlobal("c")
	if a.Int() != 20 || b.Int() != 30 || c.Int() != 40 {
		t.Errorf("expected 20,30,40, got %v,%v,%v", a, b, c)
	}
}

func TestTableUnpackSparseRangeBoundary(t *testing.T) {
	interp := runProgram(t, `
		t := {[1000000000]: "tail"}
		ok, err := pcall(table.unpack, t, 1, 1000000000)
		result := !ok && string.find(err, "too many results", 1, true) != nil
	`)
	result := interp.GetGlobal("result")
	if !result.IsBool() || !result.Bool() {
		t.Fatalf("expected table.unpack to reject extreme sparse range, got %v", result)
	}
}
