print("case:bitwise_bit32_float_conversion")

assert(bit32.bor(3.0) == 3)
assert(bit32.bor(-4.0) == 4294967292)
assert(bit32.bor(4294967291.0) == 4294967291)
assert(bit32.bor(-4294967302.0) == 4294967290)
assert(bit32.bor(281474976710651.0) == 4294967291)
assert(bit32.bor(-281474976710662.0) == 4294967290)

print("ok")
