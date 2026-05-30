print("case:math_modf_inf_integer")

local function eqT (a, b)
  return a == b and math.type(a) == math.type(b)
end

local a, b = math.modf(-1 / 0)
assert(a == -1 / 0 and b == 0.0 and math.type(b) == "float")

a, b = math.modf(1 / 0)
assert(a == 1 / 0 and b == 0.0 and math.type(b) == "float")

a, b = math.modf(3)
assert(eqT(a, 3) and eqT(b, 0.0))

print("ok")
