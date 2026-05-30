print("case:math_log_angle_more")

func eq(a, b) {
  return math.abs(a - b) <= 1e-11
}

assert(eq(math.deg(math.pi / 2), 90))
assert(eq(math.rad(180), math.pi))
assert(eq(math.atan(1, 0), math.pi / 2))
assert(eq(math.atan(-1, 0), -math.pi / 2))
assert(eq(math.log(9, 3), 2))
assert(eq(math.log(2, 10), math.log(2) / math.log(10)))
assert(eq(math.pow(math.sin(-9.8), 2) + math.pow(math.cos(-9.8), 2), 1))

print("ok")
