print("case:math_float_int_order_edges")

assert(1 < 1.1)
assert(not (1 < 0.9))
assert(1 <= 1.1)
assert(not (1 <= 0.9))
assert(-1 < -0.9)
assert(not (-1 < -1.1))
assert(1 <= 1.1)
assert(not (-1 <= -1.1))
assert(-1 < -0.9)
assert(not (-1 < -1.1))
assert(-1 <= -0.9)
assert(not (-1 <= -1.1))

assert(9007199254740992.0 == 9007199254740992)
assert(9007199254740992.0 - 1.0 ~= 9007199254740992)

print("ok")
