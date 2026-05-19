print("case:calls_fixed_arity_nested_adjust_more")

func triple() {
  return 10, 20, 30
}

func id(x) {
  return x
}

func join3(a, b, c) {
  return a, b, c
}

a, b, c := join3(id(triple()), id((triple())), 99)
assert(a == 10 && b == 10 && c == 99)

func F(n) {
  if n == 0 { return 1 }
  return n - M(F(n - 1))
}

func M(n) {
  if n == 0 { return 0 }
  return n - F(M(n - 1))
}

assert(F(8) == 5)
assert(M(8) == 5)
assert(F(16) == 10)

print("ok")
