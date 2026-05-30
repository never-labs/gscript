print("case:code_arithmetic_constants_more")

local k0 = 0
local k3 = 3
local border = 65535

assert(0.0 == 0)
assert(k0 == 0)
assert(3 ^ -1 == 1 / 3)
assert((1 + 1) ^ (5 + 5) == 1024)
assert((-2) ^ (5 - 2) == -8)
assert(-k3 % 5 == 2)
assert(-((2.0 ^ 8 + -(-1)) % 8) / 2 * 4 - 3 == -5.0)
assert(border == 65535 and -border == -65535)
assert(border + 1 == 65536 and -(border + 1) == -65536)

print("ok")
