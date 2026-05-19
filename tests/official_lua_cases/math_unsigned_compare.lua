print("case:math_unsigned_compare")

local maxint = math.maxinteger
local minint = math.mininteger

assert(math.ult(3, 4))
assert(not math.ult(4, 4))
assert(math.ult(-2, -1))
assert(math.ult(2, -1))
assert(not math.ult(-2, -2))
assert(math.ult(maxint, minint))
assert(not math.ult(minint, maxint))

print("ok")
