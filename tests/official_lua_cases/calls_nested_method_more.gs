print("case:calls_nested_method_more")

a := {b: {c: {}}}
a.b.c.f1 = func(x) { return x + 1 }
a.b.c.f2 = func(self, x, y) { self[x] = y }
assert(a.b.c.f1(4) == 5)
a.b.c.f2(a.b.c, "k", 12)
assert(a.b.c.k == 12)

print("ok")
