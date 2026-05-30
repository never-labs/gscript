print("case:calls_nested_method_more")

local a = {b = {c = {}}}
function a.b.c.f1(x) return x + 1 end
function a.b.c.f2(self, x, y) self[x] = y end
assert(a.b.c.f1(4) == 5)
a.b.c.f2(a.b.c, "k", 12)
assert(a.b.c.k == 12)

print("ok")
