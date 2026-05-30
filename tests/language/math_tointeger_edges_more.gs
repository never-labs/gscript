print("case:math_tointeger_edges_more")

assert(math.tointeger("34.0") == 34)
assert(!math.tointeger("34.3"))
assert(!math.tointeger({}))
assert(!math.tointeger(0 / 0))
assert(!math.tointeger(math.huge))
assert(!math.tointeger(-math.huge))
assert(math.floor(math.huge) == math.huge)
assert(math.ceil(math.huge) == math.huge)
assert(math.floor(-math.huge) == -math.huge)
assert(math.ceil(-math.huge) == -math.huge)

print("ok")
