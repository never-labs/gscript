print("case:closure_shared_sibling_upvalues_more")

foo1, foo2, foo3 := nil, nil, nil
if true {
  a, b, c := 3, 5, 7
  foo1 = func() { return a + b }
  foo2 = func() {
    b = b + 1
    return b + a
  }
  if true {
    a := 10
    foo3 = func() { return a + b }
  }
  assert(c == 7)
}

assert(foo1() == 8)
assert(foo2() == 9)
assert(foo1() == 9)
assert(foo3() == 16)

print("ok")
