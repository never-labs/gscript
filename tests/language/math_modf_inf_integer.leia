print("case:math_modf_inf_integer")

eqT := func(a, b) {
  return a == b && math.type(a) == math.type(b)
}

a, b := math.modf(-1 / 0)
assert(a == -1 / 0 && b == 0.0 && math.type(b) == "float")

a, b = math.modf(1 / 0)
assert(a == 1 / 0 && b == 0.0 && math.type(b) == "float")

a, b = math.modf(3)
assert(eqT(a, 3) && eqT(b, 0.0))

print("ok")
