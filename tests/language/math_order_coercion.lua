print("case:math_order_coercion")

assert(1 < 1.1); assert(not (1 < 0.9))
assert(1 <= 1.1); assert(not (1 <= 0.9))
assert(-1 < -0.9); assert(not (-1 < -1.1))
assert(-1 <= -0.9); assert(not (-1 <= -1.1))

assert("2" + 1 == 3)
assert("2 " + 1 == 3)
assert(" -2 " + 1 == -1)

print("ok")
