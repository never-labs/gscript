print("case:bitwise_bit32_wrap_more")

assert(bit32.band(-1) == 4294967295)
assert(bit32.band((2 ** 33) - 1) == 4294967295)
assert(bit32.band(-(2 ** 33) - 1) == 4294967295)
assert(bit32.band((2 ** 33) + 1) == 1)
assert(bit32.band(-(2 ** 33) + 1) == 1)
assert(bit32.band(-(2 ** 40)) == 0)
assert(bit32.band(2 ** 40) == 0)
assert(bit32.band(-(2 ** 40) - 2) == 4294967294)
assert(bit32.band((2 ** 40) - 4) == 4294967292)

print("ok")
