print("case:math_tonumber_hex_more2")

assert(tonumber('+0x2') == 2)
assert(tonumber('-0xaA') == -170)
assert(tonumber('  0x2.5  ') == 0x25/16)
assert(tonumber('  -0x2.5  ') == -0x25/16)
assert(tonumber('  +0x0.51p+8  ') == 0x51)
assert(tonumber('0x3.0') == 3)
assert(tonumber('0x0.8') == 0.5)
assert(tonumber('0x4P-2') == 1)
assert(tonumber('0xa.aP4') == 0XAA)

print("ok")
