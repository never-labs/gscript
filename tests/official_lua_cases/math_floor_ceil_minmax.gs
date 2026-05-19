print("case:math_floor_ceil_minmax")

assert(math.floor(3.4) == 3)
assert(math.ceil(3.4) == 4)
assert(math.floor(-3.4) == -4)
assert(math.ceil(-3.4) == -3)
assert(math.max(3) == 3)
assert(math.max(3, 5, 9, 1) == 9)
assert(math.min(3) == 3)
assert(math.min(3, 5, 9, 1) == 1)
assert(math.min(3.2, 5.9, -9.2, 1.1) == -9.2)
assert(math.min(1.9, 1.7, 1.72) == 1.7)

print("ok")
