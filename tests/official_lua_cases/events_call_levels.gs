print("case:events_call_levels")

i := nil
tt := {}
tt.__call = func(t, ...) {
  i = i + 1
  if t.f {
    return t.f(...)
  } else {
    return {...}
  }
}

a := setmetatable({}, tt)
b := {}
b.f = a
setmetatable(b, tt)
c := {}
c.f = b
setmetatable(c, tt)

i = 0
x := c(3, 4, 5)
assert(i == 3)

print("ok")
