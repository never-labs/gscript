print("case:math_tonumber_decimal_edges")

assert(tonumber(3.4) == 3.4)
assert(tonumber(3) == 3)
assert(tonumber(1 / 0) == 1 / 0)
assert(tonumber("0") == 0)
assert(not tonumber(""))
assert(not tonumber("  "))
assert(not tonumber("-"))
assert(not tonumber("  -0x "))
assert(not tonumber{})

assert(
  tonumber"+0.01" == 1 / 100 and
  tonumber"+.01" == 0.01 and
  tonumber".01" == 0.01 and
  tonumber"-1." == -1 and
  tonumber"+1." == 1
)

assert(
  not tonumber"+ 0.01" and
  not tonumber"+.e1" and
  not tonumber"1e" and
  not tonumber"1.0e+" and
  not tonumber"."
)

assert(tonumber("-012") == -12)
assert(tonumber("-1.2e2") == -120)

print("ok")
