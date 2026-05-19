print("case:math_floor_ceil_large_more")

local function eqT(a, b)
  return a == b and math.type(a) == math.type(b)
end

assert(eqT(math.floor(3.4), 3))
assert(eqT(math.ceil(3.4), 4))
assert(eqT(math.floor(-3.4), -4))
assert(eqT(math.ceil(-3.4), -3))

for _, p in pairs{31, 32, 40} do
  assert(math.floor(2^p) == 2^p)
  assert(math.floor(2^p + 0.5) == 2^p)
  assert(math.ceil(2^p) == 2^p)
  assert(math.ceil(2^p - 0.5) == 2^p)
end

print("ok")
