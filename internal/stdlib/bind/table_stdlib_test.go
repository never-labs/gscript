package bind

import "testing"

func TestTableInsert(t *testing.T) {
	interp := runProgram(t, `
		t := {1, 2, 3}
		table.insert(t, 4)
		table.insert(t, 2, 10)
	`)
	tbl := interp.GetGlobal("t").Table()
	if tbl.Length() != 5 {
		t.Errorf("expected length 5, got %d", tbl.Length())
	}
	if tbl.RawGet(IntValue(1)).Int() != 1 {
		t.Errorf("expected t[1]=1")
	}
	if tbl.RawGet(IntValue(2)).Int() != 10 {
		t.Errorf("expected t[2]=10, got %v", tbl.RawGet(IntValue(2)))
	}
	if tbl.RawGet(IntValue(3)).Int() != 2 {
		t.Errorf("expected t[3]=2, got %v", tbl.RawGet(IntValue(3)))
	}
	if tbl.RawGet(IntValue(5)).Int() != 4 {
		t.Errorf("expected t[5]=4, got %v", tbl.RawGet(IntValue(5)))
	}
}

func TestTableRemove(t *testing.T) {
	interp := runProgram(t, `
		t := {10, 20, 30, 40}
		removed := table.remove(t, 2)
	`)
	tbl := interp.GetGlobal("t").Table()
	removed := interp.GetGlobal("removed")
	if removed.Int() != 20 {
		t.Errorf("expected removed=20, got %v", removed)
	}
	if tbl.Length() != 3 {
		t.Errorf("expected length 3, got %d", tbl.Length())
	}
	if tbl.RawGet(IntValue(2)).Int() != 30 {
		t.Errorf("expected t[2]=30, got %v", tbl.RawGet(IntValue(2)))
	}
}

func TestTableRemoveLast(t *testing.T) {
	interp := runProgram(t, `
		t := {10, 20, 30}
		removed := table.remove(t)
	`)
	removed := interp.GetGlobal("removed")
	if removed.Int() != 30 {
		t.Errorf("expected removed=30, got %v", removed)
	}
	tbl := interp.GetGlobal("t").Table()
	if tbl.Length() != 2 {
		t.Errorf("expected length 2, got %d", tbl.Length())
	}
}

func TestTableConcat(t *testing.T) {
	interp := runProgram(t, `
		t := {"hello", "world", "foo"}
		a := table.concat(t, ", ")
		b := table.concat(t, "-", 1, 2)
	`)
	if interp.GetGlobal("a").Str() != "hello, world, foo" {
		t.Errorf("expected 'hello, world, foo', got '%s'", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "hello-world" {
		t.Errorf("expected 'hello-world', got '%s'", interp.GetGlobal("b").Str())
	}
}

func TestTableSort(t *testing.T) {
	interp := runProgram(t, `
		t := {3, 1, 4, 1, 5, 9}
		table.sort(t)
	`)
	tbl := interp.GetGlobal("t").Table()
	expected := []int64{1, 1, 3, 4, 5, 9}
	for i, exp := range expected {
		v := tbl.RawGet(IntValue(int64(i + 1)))
		if v.Int() != exp {
			t.Errorf("t[%d] = %v, expected %d", i+1, v, exp)
		}
	}
}

func TestTableSortCustom(t *testing.T) {
	interp := runProgram(t, `
		t := {3, 1, 4, 1, 5}
		table.sort(t, func(a, b) { return a > b })
	`)
	tbl := interp.GetGlobal("t").Table()
	expected := []int64{5, 4, 3, 1, 1}
	for i, exp := range expected {
		v := tbl.RawGet(IntValue(int64(i + 1)))
		if v.Int() != exp {
			t.Errorf("t[%d] = %v, expected %d", i+1, v, exp)
		}
	}
}

func TestTableUnpack(t *testing.T) {
	interp := runProgram(t, `
		a, b, c := table.unpack({10, 20, 30})
	`)
	if interp.GetGlobal("a").Int() != 10 {
		t.Errorf("expected a=10")
	}
	if interp.GetGlobal("b").Int() != 20 {
		t.Errorf("expected b=20")
	}
	if interp.GetGlobal("c").Int() != 30 {
		t.Errorf("expected c=30")
	}
}

func TestTablePack(t *testing.T) {
	interp := runProgram(t, `
		t := table.pack(10, 20, 30)
		n := t.n
	`)
	tbl := interp.GetGlobal("t").Table()
	if tbl.RawGet(IntValue(1)).Int() != 10 {
		t.Errorf("expected t[1]=10")
	}
	if interp.GetGlobal("n").Int() != 3 {
		t.Errorf("expected n=3, got %v", interp.GetGlobal("n"))
	}
}

func TestTableMove(t *testing.T) {
	interp := runProgram(t, `
		t := {1, 2, 3, 4, 5}
		table.move(t, 3, 5, 1)
	`)
	tbl := interp.GetGlobal("t").Table()
	if tbl.RawGet(IntValue(1)).Int() != 3 {
		t.Errorf("expected t[1]=3, got %v", tbl.RawGet(IntValue(1)))
	}
	if tbl.RawGet(IntValue(2)).Int() != 4 {
		t.Errorf("expected t[2]=4, got %v", tbl.RawGet(IntValue(2)))
	}
}
