print("case:math_modulo_lua_semantics")

func eqT(a, b) {
  return a == b && math.type(a) == math.type(b)
}

assert(eqT(-4 % 3, 2))
assert(eqT(4 % -3, -2))
assert(eqT(-4.0 % 3, 2.0))
assert(eqT(4 % -3.0, -2.0))
assert(eqT(4 % -5, -1))
assert(eqT(4 % -5.0, -1.0))
assert(eqT(4 % 5, 4))
assert(eqT(4 % 5.0, 4.0))
assert(eqT(-4 % -5, -4))
assert(eqT(-4 % -5.0, -4.0))
assert(eqT(-4 % 5, 1))
assert(eqT(-4 % 5.0, 1.0))
assert(eqT(4.25 % 4, 0.25))
assert(eqT(10.0 % 2, 0.0))
assert(eqT(-10.0 % 2, 0.0))
assert(eqT(-10.0 % -2, 0.0))
assert(math.pi - math.pi % 1 == 3)
assert(math.pi - math.pi % 0.001 == 3.141)

for i := -10; i <= 10; i++ {
  for j := -10; j <= 10; j++ {
    if j != 0 {
      assert((i + 0.0) % j == i % j)
    }
  }
}

print("ok")
