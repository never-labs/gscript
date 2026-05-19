print("case:bitwise_bit32_varargs")

assert(bit32.band() == bit32.bnot(0))
assert(bit32.btest() == true)
assert(bit32.bor() == 0)
assert(bit32.bxor() == 0)
assert(bit32.band() == bit32.band(4294967295))
assert(bit32.band(1, 2) == 0)
assert(bit32.bor(1, 2, 4, 8) == 15)
assert(bit32.bxor(1, 2, 4, 8) == 15)
assert(bit32.bxor(1, 2, 4, 8, 15) == 0)

print("ok")
