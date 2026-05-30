print("case:events_compare_compat_more")

local t1, t2, c, d
t1 = {}
c = {}; setmetatable(c, t1)
d = {}
t1.__eq = function() return true end
t1.__lt = function() return true end
t1.__le = function() return false end
setmetatable(d, t1)
assert(c == d and c < d and not (d <= c))

t2 = {}
t2.__eq = t1.__eq
t2.__lt = t1.__lt
setmetatable(d, t2)
assert(c == d and c < d and not (d <= c))

print("ok")
