print("case:bitwise_bit32_errors")

assert(!pcall(bit32.band, {}))
assert(!pcall(bit32.bnot, "a"))
assert(!pcall(bit32.lshift, 45))
assert(!pcall(bit32.lshift, 45, print))
assert(!pcall(bit32.rshift, 45, print))
assert(bit32.band(1, 3, 7) == 1)
assert(bit32.bnot(0) == 4294967295)

print("ok")
