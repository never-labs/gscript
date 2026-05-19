print("case:math_tointeger_edges_more")

assert(math.tointeger("34.0") == 34)
assert(not math.tointeger("34.3"))
assert(not math.tointeger({}))
assert(not math.tointeger(0/0))
assert(not math.tointeger(math.huge))
assert(not math.tointeger(-math.huge))
assert(math.floor(math.huge) == math.huge)
assert(math.ceil(math.huge) == math.huge)
assert(math.floor(-math.huge) == -math.huge)
assert(math.ceil(-math.huge) == -math.huge)

print("ok")
