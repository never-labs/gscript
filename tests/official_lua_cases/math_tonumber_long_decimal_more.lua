print("case:math_tonumber_long_decimal_more")

local function eqT (a,b) return a == b and math.type(a) == math.type(b) end

assert(eqT(tonumber("1"..string.rep("0", 30)), 1e30))
assert(eqT(tonumber("-1"..string.rep("0", 30)), -1e30))
assert(not tonumber("e1"))
assert(not tonumber("e  1"))
assert(not tonumber(" 3.4.5 "))
assert(tonumber("1111111111") - tonumber("1111111110") ==
       tonumber"  +0.001e+3 \n\t")

print("ok")
