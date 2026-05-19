print("case:nextvar_pairs_return_triple")

a := {}
f, state, control := pairs(a)
assert(type(f) == "function" && state == a && control == nil)
assert(f(state, control) == nil)

b := {x: 10}
f, state, control = pairs(b)
assert(type(f) == "function" && state == b && control == nil)
k, v := f(state, control)
assert(k == "x" && v == 10)
assert(f(state, k) == nil)

print("ok")
