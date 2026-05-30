print("case:math_order_ops")

assert(not (1 < 1) and (1 < 2) and not (2 < 1))
assert(not ("a" < "a") and ("a" < "b") and not ("b" < "a"))
assert((1 <= 1) and (1 <= 2) and not (2 <= 1))
assert(("a" <= "a") and ("a" <= "b") and not ("b" <= "a"))
assert(not (1 > 1) and not (1 > 2) and (2 > 1))
assert(not ("a" > "a") and not ("a" > "b") and ("b" > "a"))
assert((1 >= 1) and not (1 >= 2) and (2 >= 1))
assert(("a" >= "a") and not ("a" >= "b") and ("b" >= "a"))
assert(1.3 < 1.4 and 1.3 <= 1.4 and not (1.3 < 1.3) and 1.3 <= 1.3)

print("ok")
