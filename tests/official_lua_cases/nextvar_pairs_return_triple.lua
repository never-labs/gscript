print("case:nextvar_pairs_return_triple")

local a = {}
local f, state, control = pairs(a)
assert(type(f) == "function" and state == a and control == nil)
assert(f(state, control) == nil)

local b = {x = 10}
f, state, control = pairs(b)
assert(type(f) == "function" and state == b and control == nil)
local k, v = f(state, control)
assert(k == "x" and v == 10)
assert(f(state, k) == nil)

print("ok")
