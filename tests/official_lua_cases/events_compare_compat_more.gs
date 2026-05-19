print("case:events_compare_compat_more")

t1 := {}
c := {}; setmetatable(c, t1)
d := {}
t1.__eq = func() { return true }
t1.__lt = func() { return true }
t1.__le = func() { return false }
setmetatable(d, t1)
assert(c == d && c < d && !(d <= c))

t2 := {}
t2.__eq = t1.__eq
t2.__lt = t1.__lt
setmetatable(d, t2)
assert(c == d && c < d && !(d <= c))

print("ok")
