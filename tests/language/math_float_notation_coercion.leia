print("case:math_float_notation_coercion")

assert(0e12 == 0 && 0.0 == 0 && 0. == 0 && 0.2e2 == 20 && 2.0E-1 == 0.2)

a := "2"
b := " 3e0 "
c := " 10  "
assert(a + b == 5 && -b == -3 && b + "2" == 5 && "10" - c == 0)
assert(type(a) == "string" && type(b) == "string" && type(c) == "string")
assert(a == "2" && b == " 3e0 " && c == " 10  " && -c == -"  10 ")
assert(c % a == 0 && math.pow(a, b) == 8)

a = 0
assert(a == -a && 0 == -0)

print("ok")
