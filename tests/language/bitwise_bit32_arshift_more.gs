print("case:bitwise_bit32_arshift_more")

assert(bit32.arshift(305419896, 0) == 305419896)
assert(bit32.arshift(305419896, 1) == 152709948)
assert(bit32.arshift(305419896, -1) == 610839792)
assert(bit32.arshift(-1, 1) == 4294967295)
assert(bit32.arshift(-1, 24) == 4294967295)
assert(bit32.arshift(-1, 32) == 4294967295)
assert(bit32.arshift(-1, -1) == bit32.band(-1 * 2, 4294967295))

print("ok")
