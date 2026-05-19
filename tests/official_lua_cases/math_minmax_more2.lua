print("case:math_minmax_more2")

local function eqT(a, b) return a == b and math.type(a) == math.type(b) end
local maxint = math.maxinteger
local minint = math.mininteger
assert(eqT(math.max(3), 3))
assert(eqT(math.max(3, 5, 9, 1), 9))
assert(math.max(maxint, 10e60) == 10e60)
assert(eqT(math.max(minint, minint + 1), minint + 1))
assert(eqT(math.min(3), 3))
assert(eqT(math.min(3, 5, 9, 1), 1))
assert(math.min(3.2, 5.9, -9.2, 1.1) == -9.2)
assert(math.min(1.9, 1.7, 1.72) == 1.7)
assert(math.min(-10e60, minint) == -10e60)
assert(eqT(math.min(maxint, maxint - 1), maxint - 1))
assert(eqT(math.min(maxint - 2, maxint, maxint - 1), maxint - 2))

print("ok")
