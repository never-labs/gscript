print("case:vararg_tail_missing_args")

f := func(a, b, c) {
  return c, b
}

g := func() {
  return f(1, 2)
}

a, b := g()
assert(a == nil && b == 2)

print("ok")
