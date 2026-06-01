print("case:closure_tailcall_upvalue")

t := func() {
  c := func(a, b) {
    assert(a == "test" && b == "OK")
  }
  v := func(f, ... ) {
    c("test", f() != 1 && "FAILED" || "OK")
  }
  x := 1
  return v(func() { return x })
}

t()

print("ok")
