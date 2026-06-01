print("case:calls_closure_parameter_capture_more")

func g(z) {
  func f(a, b, c, d) {
    return func(x, y) { return a + b + c + d + a + x + y + z }
  }
  return f(z, z + 1, z + 2, z + 3)
}

f := g(10)
assert(f(9, 16) == 10 + 11 + 12 + 13 + 10 + 9 + 16 + 10)

func maker(a, b, c) {
  return func(x) {
    return a + b + c + x
  }
}

h := maker(1, 2, 3, 4)
assert(h(10) == 16)
assert(!pcall(maker(5, 6), 7))

print("ok")
