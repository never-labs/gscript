print("case:math_float_notation_coercion")

assert(0e12 == 0 and .0 == 0 and 0. == 0 and .2e2 == 20 and 2.E-1 == 0.2)

local a, b, c = "2", " 3e0 ", " 10  "
assert(a + b == 5 and -b == -3 and b + "2" == 5 and "10" - c == 0)
assert(type(a) == "string" and type(b) == "string" and type(c) == "string")
assert(a == "2" and b == " 3e0 " and c == " 10  " and -c == -"  10 ")
assert(c % a == 0 and a ^ b == 08)

a = 0
assert(a == -a and 0 == -0)

print("ok")
