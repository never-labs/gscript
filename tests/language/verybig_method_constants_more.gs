print("case:verybig_method_constants_more")

dummy := {}
for i := 1; i <= 300; i++ {
  dummy[i] = i
}

t := {foo: func(self, x) { return x + self.x }, x: 10}
t.t = t
assert(dummy[1] == 1 && dummy[256] == 256 && dummy[300] == 300)
assert(t:foo(1.5) == 11.5)
assert(t.t:foo(0.5) == 10.5)
assert((func() { return t.x + dummy[275] })() == 285)

print("ok")
