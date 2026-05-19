print("case:events_eq_invalidate_more")

mt := {__eq: true}
a := {10}
b := {10}
setmetatable(a, mt)
setmetatable(b, mt)
mt.__eq = nil
assert(a != b)
mt.__eq = func(x, y) { return x[1] == y[1] }
assert(a == b)

print("ok")
