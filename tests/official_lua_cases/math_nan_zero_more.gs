print("case:math_nan_zero_more")

mz := -0.0
z := 0.0
assert(mz == z)
assert(1/mz < 0 && 0 < 1/z)
inf := math.huge * 2 + 1
mz = -1/inf
z = 1/inf
assert(mz == z)
assert(1/mz < 0 && 0 < 1/z)
NaN := inf - inf
assert(NaN != NaN)
assert(!(NaN < NaN))
assert(!(NaN <= NaN))
assert(!(NaN > NaN))
assert(!(NaN >= NaN))
assert(!(0 < NaN) && !(NaN < 0))
NaN1 := 0/0
assert(NaN != NaN1 && !(NaN <= NaN1) && !(NaN1 <= NaN))

print("ok")
