print("case:db_upvalue_closure_flow")

a, b, c := 1, 2, 3

foo1 := func(x) {
  b = x
  return c
}

foo2 := func(x) {
  a = x
  return c + b
}

assert(foo1(10) == 3)
assert(foo2(5) == 13)
assert(a == 5 && b == 10 && c == 3)

make_counter := func(step) {
  total := a + b
  return func(delta) {
    total = total + step + delta
    return total
  }
}

c1 := make_counter(2)
c2 := make_counter(5)
assert(c1(1) == 18)
assert(c1(1) == 21)
assert(c2(0) == 20)
assert(c1(0) == 23)

print("ok")
