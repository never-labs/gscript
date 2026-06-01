print("case:math_transcendentals_more")

func eq(a, b, limit) {
  if !limit {
    limit = 1e-11
  }
  return a == b || math.abs(a - b) <= limit
}

assert(eq(math.atan(1, 0), math.pi / 2))
assert(eq(math.sin(10), math.sin(10 % (2 * math.pi))))
assert(tonumber(" 1.3e-2 ") == 1.3e-2)
assert(tonumber(" -1.00000000000001 ") == -1.00000000000001)
assert(8388609 + -8388609 == 0)
assert(8388608 + -8388608 == 0)
assert(8388607 + -8388607 == 0)

print("ok")
