print("case:math_modf_finite_edges")

local a, b = math.modf(3.5)
assert(a == 3.0 and b == 0.5)

a, b = math.modf(-2.5)
assert(a == -2.0 and b == -0.5)

a, b = math.modf(-3e23)
assert(a == -3e23 and b == 0.0)

a, b = math.modf(3e35)
assert(a == 3e35 and b == 0.0)

print("ok")
