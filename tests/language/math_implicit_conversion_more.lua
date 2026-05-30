print("case:math_implicit_conversion_more")

local a, b = "10", "20"
assert(a * b == 200 and a + b == 30 and a - b == -10 and a / b == 0.5 and -b == -20)
assert(a == "10" and b == "20")

print("ok")
