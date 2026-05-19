print("case:math_lib_basic")

assert(math.abs(-10) == 10)
assert(math.floor(3.7) == 3)
assert(math.ceil(3.1) == 4)
assert(math.max(1, 9, 3) == 9)
assert(math.min(1, 9, 3) == 1)
assert(math.sqrt(81) == 9)
assert(math.type(1) == "integer" && math.type(1.5) == "float")
assert(math.fmod(10, 3) == 1)
assert(math.sin(0) == 0)
assert(math.cos(0) == 1)

print("ok")
