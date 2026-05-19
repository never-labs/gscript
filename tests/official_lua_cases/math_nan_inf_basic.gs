print("case:math_nan_inf_basic")

isNaN := func(x) {
  return x != x
}

assert(isNaN(0 / 0))
assert(!isNaN(1 / 0))
assert((0 / 0) != (0 / 0))
assert(math.huge > 10e30)
assert(-math.huge < -10e30)
assert(math.type(0) == "integer" && math.type(0.0) == "float" && !math.type("10"))

x := math.pow(2.0, 53)
assert(x > x - 1.0 && x == x + 1.0)

print("ok")
