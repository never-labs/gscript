print("case:constructs_precedence")

assert(2 ** 3 ** 2 == 2 ** (3 ** 2))
assert(2 ** 3 * 4 == (2 ** 3) * 4)
assert(2.0 ** -2 == 1 / 4)
assert(-2 ** 2 == -4 && (-2) ** 2 == 4)
assert(-3 % 5 == 2)
assert(2 * 1 + 3 / 3 == 3)
assert(1 + 2 .. 3 * 1 == "33")
assert(!((true || false) && nil))
assert(true || false && nil)

a, b := 1, nil
assert(-(1 || 2) == -1)
assert((1 && 2) + (-1.25 || -4) == 0.75)
x := ((b || a) + 1 == 2 && (10 || a) + 1 == 11)
assert(x)

print("ok")
