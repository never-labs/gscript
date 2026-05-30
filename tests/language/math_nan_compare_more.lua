print("case:math_nan_compare_more")

local NaN = 0 / 0
assert(not (NaN < 0))
assert(not (NaN > math.mininteger))
assert(not (NaN <= -9))
assert(not (NaN <= math.maxinteger))
assert(not (NaN < math.maxinteger))
assert(not (math.mininteger <= NaN))
assert(not (math.mininteger < NaN))
assert(not (4 <= NaN))
assert(not (4 < NaN))

print("ok")
