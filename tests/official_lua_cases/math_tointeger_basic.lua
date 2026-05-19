print("case:math_tointeger_basic")

assert(math.tointeger(3) == 3)
assert(math.tointeger(3.0) == 3)
assert(math.tointeger(3.4) == nil)
assert(math.type(math.tointeger(3.0)) == "integer")
assert(math.tointeger("34.0") == 34)
assert(not math.tointeger("34.3"))
assert(not math.tointeger({}))

print("ok")
