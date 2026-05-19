print("case:math_nan_inf_basic")

local function isNaN (x)
  return x ~= x
end

assert(isNaN(0 / 0))
assert(not isNaN(1 / 0))
assert((0 / 0) ~= (0 / 0))
assert(math.huge > 10e30)
assert(-math.huge < -10e30)
assert(math.type(0) == "integer" and math.type(0.0) == "float" and not math.type("10"))

local x = 2.0 ^ 53
assert(x > x - 1.0 and x == x + 1.0)

print("ok")
