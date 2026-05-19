print("case:math_nan_zero_more")

local mz = -0.0
local z = 0.0
assert(mz == z)
assert(1/mz < 0 and 0 < 1/z)
local inf = math.huge * 2 + 1
mz = -1/inf
z = 1/inf
assert(mz == z)
assert(1/mz < 0 and 0 < 1/z)
local NaN = inf - inf
assert(NaN ~= NaN)
assert(not (NaN < NaN))
assert(not (NaN <= NaN))
assert(not (NaN > NaN))
assert(not (NaN >= NaN))
assert(not (0 < NaN) and not (NaN < 0))
local NaN1 = 0/0
assert(NaN ~= NaN1 and not (NaN <= NaN1) and not (NaN1 <= NaN))

print("ok")
