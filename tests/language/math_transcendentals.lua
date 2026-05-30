print("case:math_transcendentals")

local function eq (a, b, limit)
  limit = limit or 1e-11
  return a == b or math.abs(a - b) <= limit
end

assert(eq(math.sin(-9.8) ^ 2 + math.cos(-9.8) ^ 2, 1))
assert(eq(math.tan(math.pi / 4), 1))
assert(eq(math.sin(math.pi / 2), 1) and eq(math.cos(math.pi / 2), 0))
assert(eq(math.atan(1), math.pi / 4))
assert(eq(math.acos(0), math.pi / 2))
assert(eq(math.asin(1), math.pi / 2))
assert(eq(math.deg(math.pi / 2), 90) and eq(math.rad(90), math.pi / 2))
assert(math.abs(-10.43) == 10.43)
assert(math.fmod(10, 3) == 1)
assert(eq(math.sqrt(10) ^ 2, 10))
assert(eq(math.log(2, 10), math.log(2) / math.log(10)))
assert(eq(math.log(2, 2), 1))
assert(eq(math.log(9, 3), 2))
assert(eq(math.exp(0), 1))

print("ok")
