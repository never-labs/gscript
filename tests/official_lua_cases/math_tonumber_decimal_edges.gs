print("case:math_tonumber_decimal_edges")

assert(tonumber(3.4) == 3.4)
assert(tonumber(3) == 3)
assert(tonumber(1 / 0) == 1 / 0)
assert(tonumber("0") == 0)
assert(!tonumber(""))
assert(!tonumber("  "))
assert(!tonumber("-"))
assert(!tonumber("  -0x "))
assert(!tonumber({}))

assert(
  tonumber("+0.01") == 1 / 100 &&
  tonumber("+.01") == 0.01 &&
  tonumber(".01") == 0.01 &&
  tonumber("-1.") == -1 &&
  tonumber("+1.") == 1
)

assert(
  !tonumber("+ 0.01") &&
  !tonumber("+.e1") &&
  !tonumber("1e") &&
  !tonumber("1.0e+") &&
  !tonumber(".")
)

assert(tonumber("-012") == -12)
assert(tonumber("-1.2e2") == -120)

print("ok")
