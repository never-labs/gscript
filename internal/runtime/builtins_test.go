package runtime

import "testing"

// ==================================================================
// Error handling tests
// ==================================================================

func TestPcall_success(t *testing.T) {
	interp := runProgram(t, `
		func add(a, b) {
			return a + b
		}
		ok, result := pcall(add, 1, 2)
	`)
	ok := interp.GetGlobal("ok")
	result := interp.GetGlobal("result")
	if !ok.Bool() {
		t.Errorf("pcall should return true on success, got %v", ok)
	}
	if !result.IsInt() || result.Int() != 3 {
		t.Errorf("expected 3, got %v", result)
	}
}

func TestPcall_error_string(t *testing.T) {
	interp := runProgram(t, `
		func fail() {
			error("something went wrong")
		}
		ok, msg := pcall(fail)
	`)
	ok := interp.GetGlobal("ok")
	msg := interp.GetGlobal("msg")
	if ok.Truthy() {
		t.Errorf("pcall should return false on error")
	}
	if msg.Str() != "something went wrong" {
		t.Errorf("expected 'something went wrong', got '%v'", msg)
	}
}

func TestPcall_error_value(t *testing.T) {
	interp := runProgram(t, `
		func fail() {
			error(42)
		}
		ok, val := pcall(fail)
	`)
	ok := interp.GetGlobal("ok")
	val := interp.GetGlobal("val")
	if ok.Truthy() {
		t.Errorf("pcall should return false on error")
	}
	if !val.IsInt() || val.Int() != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestPcall_runtime_error(t *testing.T) {
	interp := runProgram(t, `
		func fail() {
			x := nil
			return x.foo
		}
		ok, msg := pcall(fail)
	`)
	ok := interp.GetGlobal("ok")
	msg := interp.GetGlobal("msg")
	if ok.Truthy() {
		t.Errorf("pcall should return false on runtime error")
	}
	if !msg.IsString() || msg.Str() == "" {
		t.Errorf("expected error message, got %v", msg)
	}
}

func TestXpcall(t *testing.T) {
	interp := runProgram(t, `
		func fail() {
			error("oops")
		}
		func handler(err) {
			return "handled: " .. err
		}
		ok, msg := xpcall(fail, handler)
	`)
	ok := interp.GetGlobal("ok")
	msg := interp.GetGlobal("msg")
	if ok.Truthy() {
		t.Errorf("xpcall should return false on error")
	}
	if msg.Str() != "handled: oops" {
		t.Errorf("expected 'handled: oops', got '%v'", msg)
	}
}

func TestXpcall_success(t *testing.T) {
	interp := runProgram(t, `
		func good() {
			return 10, 20
		}
		func handler(err) {
			return "bad"
		}
		ok, a, b := xpcall(good, handler)
	`)
	ok := interp.GetGlobal("ok")
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	if !ok.Bool() {
		t.Errorf("xpcall should return true on success")
	}
	if a.Int() != 10 || b.Int() != 20 {
		t.Errorf("expected 10, 20 got %v, %v", a, b)
	}
}

func TestAssert_pass(t *testing.T) {
	interp := runProgram(t, `
		x := assert(42, "should not fail")
	`)
	x := interp.GetGlobal("x")
	if !x.IsInt() || x.Int() != 42 {
		t.Errorf("assert should return its first arg on success, got %v", x)
	}
}

func TestAssert_fail(t *testing.T) {
	interp := runProgram(t, `
		ok, msg := pcall(assert, false, "my message")
	`)
	ok := interp.GetGlobal("ok")
	msg := interp.GetGlobal("msg")
	if ok.Truthy() {
		t.Errorf("assert(false) should error")
	}
	if msg.Str() != "my message" {
		t.Errorf("expected 'my message', got '%v'", msg)
	}
}

func TestAssert_fail_default_msg(t *testing.T) {
	interp := runProgram(t, `
		ok, msg := pcall(assert, nil)
	`)
	ok := interp.GetGlobal("ok")
	msg := interp.GetGlobal("msg")
	if ok.Truthy() {
		t.Errorf("assert(nil) should error")
	}
	if msg.Str() != "assertion failed" {
		t.Errorf("expected 'assertion failed', got '%v'", msg)
	}
}

func TestErrorObject(t *testing.T) {
	interp := runProgram(t, `
		errTbl := {code: 404, msg: "not found"}
		func fail() {
			error(errTbl)
		}
		ok, val := pcall(fail)
	`)
	ok := interp.GetGlobal("ok")
	val := interp.GetGlobal("val")
	if ok.Truthy() {
		t.Errorf("pcall should return false")
	}
	if !val.IsTable() {
		t.Errorf("expected table error value, got %s", val.TypeName())
	}
	code := val.Table().RawGet(StringValue("code"))
	if code.Int() != 404 {
		t.Errorf("expected code 404, got %v", code)
	}
}

// ==================================================================
// Pairs/ipairs tests
// ==================================================================

func TestIpairs(t *testing.T) {
	interp := runProgram(t, `
		t := {10, 20, 30}
		sum := 0
		for i, v := range ipairs(t) {
			sum = sum + v
		}
	`)
	sum := interp.GetGlobal("sum")
	if sum.Int() != 60 {
		t.Errorf("expected sum=60, got %v", sum)
	}
}

func TestPairs(t *testing.T) {
	interp := runProgram(t, `
		t := {a: 1, b: 2, c: 3}
		sum := 0
		for k, v := range pairs(t) {
			sum = sum + v
		}
	`)
	sum := interp.GetGlobal("sum")
	if sum.Int() != 6 {
		t.Errorf("expected sum=6, got %v", sum)
	}
}

func TestSelect(t *testing.T) {
	interp := runProgram(t, `
		count := select("#", 10, 20, 30)
		first := select(1, 10, 20, 30)
		second := select(2, 10, 20, 30)
	`)
	if interp.GetGlobal("count").Int() != 3 {
		t.Errorf("expected count=3, got %v", interp.GetGlobal("count"))
	}
	if interp.GetGlobal("first").Int() != 10 {
		t.Errorf("expected first=10, got %v", interp.GetGlobal("first"))
	}
	if interp.GetGlobal("second").Int() != 20 {
		t.Errorf("expected second=20, got %v", interp.GetGlobal("second"))
	}
}

func TestUnpack(t *testing.T) {
	interp := runProgram(t, `
		a, b, c := unpack({10, 20, 30})
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

func TestNext(t *testing.T) {
	interp := runProgram(t, `
		t := {10, 20}
		k1, v1 := next(t, nil)
		k2, v2 := next(t, k1)
	`)
	k1 := interp.GetGlobal("k1")
	v1 := interp.GetGlobal("v1")
	if k1.Int() != 1 || v1.Int() != 10 {
		t.Errorf("expected k1=1,v1=10, got k1=%v,v1=%v", k1, v1)
	}
	k2 := interp.GetGlobal("k2")
	v2 := interp.GetGlobal("v2")
	if k2.Int() != 2 || v2.Int() != 20 {
		t.Errorf("expected k2=2,v2=20, got k2=%v,v2=%v", k2, v2)
	}
}

// ==================================================================
// Error nesting: pcall inside pcall
// ==================================================================

func TestPcall_nested(t *testing.T) {
	interp := runProgram(t, `
		func inner() {
			error("inner error")
		}
		func outer() {
			ok, msg := pcall(inner)
			return ok, msg
		}
		ok, msg := outer()
	`)
	ok := interp.GetGlobal("ok")
	msg := interp.GetGlobal("msg")
	if ok.Truthy() {
		t.Errorf("expected false from pcall(inner)")
	}
	if msg.Str() != "inner error" {
		t.Errorf("expected 'inner error', got %v", msg)
	}
}

func TestPcall_with_args(t *testing.T) {
	interp := runProgram(t, `
		func div(a, b) {
			if b == 0 {
				error("division by zero")
			}
			return a / b
		}
		ok1, r1 := pcall(div, 10, 2)
		ok2, r2 := pcall(div, 10, 0)
	`)
	if !interp.GetGlobal("ok1").Bool() {
		t.Errorf("pcall(div, 10, 2) should succeed")
	}
	if interp.GetGlobal("r1").Int() != 5 {
		t.Errorf("expected 5, got %v", interp.GetGlobal("r1"))
	}
	if interp.GetGlobal("ok2").Truthy() {
		t.Errorf("pcall(div, 10, 0) should fail")
	}
	if interp.GetGlobal("r2").Str() != "division by zero" {
		t.Errorf("expected 'division by zero', got %v", interp.GetGlobal("r2"))
	}
}
