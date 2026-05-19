print("case:math_tonumber_overflow_more")

func eqT(a, b) { return a == b && math.type(a) == math.type(b) }
assert(eqT(tonumber(tostring(math.maxinteger)), math.maxinteger))
assert(eqT(tonumber(tostring(math.mininteger)), math.mininteger))
assert(eqT(tonumber("1" .. string.rep("0", 30)), 1e30))
assert(eqT(tonumber("-1" .. string.rep("0", 30)), -1e30))

print("ok")
