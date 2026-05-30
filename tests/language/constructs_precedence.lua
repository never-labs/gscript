print("case:constructs_precedence")

assert(2^3^2 == 2^(3^2))
assert(2^3*4 == (2^3)*4)
assert(2.0^-2 == 1/4)
assert(-2^2 == -4 and (-2)^2 == 4)
assert(-3%5 == 2)
assert(2*1+3/3 == 3)
assert(1+2 .. 3*1 == "33")
assert(not ((true or false) and nil))
assert(true or false and nil)

local a, b = 1, nil
assert(-(1 or 2) == -1)
assert((1 and 2)+(-1.25 or -4) == 0.75)
local x = ((b or a)+1 == 2 and (10 or a)+1 == 11)
assert(x)

print("ok")
