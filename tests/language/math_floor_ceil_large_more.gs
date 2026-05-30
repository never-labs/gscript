print("case:math_floor_ceil_large_more")

func eqT(a, b) {
  return a == b && math.type(a) == math.type(b)
}

assert(eqT(math.floor(3.4), 3))
assert(eqT(math.ceil(3.4), 4))
assert(eqT(math.floor(-3.4), -4))
assert(eqT(math.ceil(-3.4), -3))

ps := {31, 32, 40}
for _, p := range pairs(ps) {
  assert(math.floor(2 ** p) == 2 ** p)
  assert(math.floor(2 ** p + 0.5) == 2 ** p)
  assert(math.ceil(2 ** p) == 2 ** p)
  assert(math.ceil(2 ** p - 0.5) == 2 ** p)
}

print("ok")
